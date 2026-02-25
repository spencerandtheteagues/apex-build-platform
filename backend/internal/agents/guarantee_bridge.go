// Package agents — guarantee_bridge.go
//
// Integrates the core.AgentFSM + guarantee.GuaranteeEngine into the existing
// BuildOrchestrator pipeline. This file provides:
//
//  1. BuildFSMContext — per-build FSM + GuaranteeEngine instance
//  2. Integration functions to hook FSM events into WSHub broadcasting
//  3. Guarantee-wrapped task execution for the orchestration pipeline
package agents

import (
	"context"
	"fmt"
	"log"
	"time"

	"apex-build/internal/agents/core"
	"apex-build/internal/agents/guarantee"
)

// BuildFSMContext holds the FSM, validator, guarantee engine, and bridge for a single build.
type BuildFSMContext struct {
	FSM       *core.AgentFSM
	Validator *core.BuildValidator
	Engine    *guarantee.GuaranteeEngine
	Bridge    *core.FSMBridge
}

// NewBuildFSMContext creates the full FSM + guarantee stack for a build.
// The broadcastFn bridges FSM events to the WebSocket hub.
func NewBuildFSMContext(buildID string, totalSteps int, broadcastFn core.BroadcastFunc) (*BuildFSMContext, error) {
	// Create FSM
	fsm := core.NewAgentFSM(core.AgentFSMConfig{
		BuildID:    buildID,
		MaxRetries: 3,
		TotalSteps: totalSteps,
	})

	// Create validator (no smoke test runner in base config — Codex 5.3 provides that)
	validator, err := core.NewBuildValidator(core.ValidatorConfig{
		MinimumPassScore: 0.8,
		RunSmokeTest:     false, // enabled when SmokeTestRunner is available
		SmokeTestTimeout: 30 * time.Second,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create validator: %w", err)
	}

	// Create guarantee engine
	engine := guarantee.NewGuaranteeEngine(fsm, validator, guarantee.DefaultEngineConfig())

	// Start FSM→WebSocket bridge
	bridge := core.NewFSMBridge(fsm, broadcastFn)

	return &BuildFSMContext{
		FSM:       fsm,
		Validator: validator,
		Engine:    engine,
		Bridge:    bridge,
	}, nil
}

// Stop tears down the bridge goroutine.
func (ctx *BuildFSMContext) Stop() {
	if ctx.Bridge != nil {
		ctx.Bridge.Stop()
	}
}

// --- WSHub Integration ---

// MakeBroadcastFunc creates a BroadcastFunc that wraps the existing WSHub.Broadcast.
// This converts the generic (buildID, msgType, data) signature into WSMessage structs.
func MakeBroadcastFunc(hub *WSHub) core.BroadcastFunc {
	return func(buildID string, msgType string, data map[string]any) {
		hub.Broadcast(buildID, &WSMessage{
			Type:      WSMessageType(msgType),
			BuildID:   buildID,
			Timestamp: time.Now(),
			Data:      data,
		})
	}
}

// --- Guarantee-wrapped task execution ---

// ExecuteTaskWithGuarantee wraps a task's execution through the guarantee engine.
// It converts between the existing Task/TaskOutput types and the core.BuildArtifact types.
func ExecuteTaskWithGuarantee(
	ctx context.Context,
	fsmCtx *BuildFSMContext,
	task *Task,
	executeFn func(ctx context.Context, task *Task) (*TaskOutput, error),
) (*TaskOutput, error) {
	stepName := fmt.Sprintf("%s:%s", task.Type, task.ID)

	result, err := fsmCtx.Engine.ExecuteWithGuarantee(ctx, stepName, func(ctx context.Context, retryCtx *guarantee.RetryContext) ([]core.BuildArtifact, error) {
		// Pass correction hints to the task input if this is a retry
		if retryCtx.IsRetry && len(retryCtx.CorrectionHints) > 0 {
			if task.Input == nil {
				task.Input = make(map[string]any)
			}
			task.Input["correction_hints"] = retryCtx.CorrectionHints
			task.Input["attempt_number"] = retryCtx.AttemptNumber
		}

		// Execute the actual task
		output, err := executeFn(ctx, task)
		if err != nil {
			return nil, err
		}

		// Convert TaskOutput.Files to BuildArtifacts
		artifacts := make([]core.BuildArtifact, 0, len(output.Files))
		for _, f := range output.Files {
			artifacts = append(artifacts, core.BuildArtifact{
				Path:     f.Path,
				Content:  f.Content,
				Language: f.Language,
				IsNew:    f.IsNew,
			})
		}

		return artifacts, nil
	})

	if err != nil {
		return nil, err
	}

	// Convert back to TaskOutput
	output := &TaskOutput{
		Files: make([]GeneratedFile, 0, len(result.Artifacts)),
		Metrics: map[string]any{
			"guarantee_attempts":  result.Attempts,
			"guarantee_score":     result.Validation.Score,
			"guarantee_verdict":   string(result.Validation.Verdict),
			"guarantee_rolled_back": result.RolledBack,
			"guarantee_duration_ms": result.DurationMs,
		},
	}

	for _, a := range result.Artifacts {
		output.Files = append(output.Files, GeneratedFile{
			Path:     a.Path,
			Content:  a.Content,
			Language: a.Language,
			Size:     int64(len(a.Content)),
			IsNew:    a.IsNew,
		})
	}

	log.Printf("[GuaranteeBridge] Task %s completed: attempts=%d score=%.2f verdict=%s",
		task.ID, result.Attempts, result.Validation.Score, result.Validation.Verdict)

	return output, nil
}
