// Package guarantee implements the APEX.BUILD 100% Success Guarantee Engine.
//
// This orchestrates the full validation → retry → rollback loop:
//  1. Agent executes a build step
//  2. Engine validates the output (placeholder scan, syntax, smoke test)
//  3. On soft fail: retry with corrective instructions (up to MaxRetries)
//  4. On hard fail: rollback to last checkpoint
//  5. On pass: advance to next step or complete
//
// PUBLIC CONTRACT FOR CODEX 5.3 INTEGRATION:
//
//	GuaranteeEngine — instantiate with NewGuaranteeEngine(fsm, validator, cfg).
//	ExecuteWithGuarantee(ctx, stepFn) → runs a step with full retry/rollback.
//	The SandboxManager hooks into this via the SmokeTestRunner interface.
package guarantee

import (
	"context"
	"fmt"
	"log"
	"time"

	"apex-build/internal/agents/core"
)

// --- Engine Config ---

// EngineConfig controls guarantee behavior.
type EngineConfig struct {
	MaxRetries         int           // max retries per step (default: 3)
	RetryBackoff       time.Duration // backoff between retries (default: 2s)
	CheckpointEveryN   int           // create checkpoint every N steps (default: 1)
	FailFastOnHardFail bool          // immediately rollback on hard fail (default: true)
	EnableSmokeTest    bool          // run smoke tests (default: true)
	SmokeTestCommand   string        // command to run for smoke test
}

// DefaultEngineConfig returns production defaults.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		MaxRetries:         3,
		RetryBackoff:       2 * time.Second,
		CheckpointEveryN:   1,
		FailFastOnHardFail: true,
		EnableSmokeTest:    true,
	}
}

// --- Step Function ---

// StepFunc is a function that executes one build step and returns artifacts.
// On retry, it receives the previous ValidationResult so it can adjust.
type StepFunc func(ctx context.Context, retryContext *RetryContext) ([]core.BuildArtifact, error)

// RetryContext provides information to a StepFunc during retries.
type RetryContext struct {
	AttemptNumber    int                    `json:"attempt_number"`
	PreviousResult   *core.ValidationResult `json:"previous_result,omitempty"`
	CorrectionHints  []string               `json:"correction_hints,omitempty"`
	IsRetry          bool                   `json:"is_retry"`
}

// --- Guarantee Engine ---

// GuaranteeEngine orchestrates the validation → retry → rollback lifecycle.
type GuaranteeEngine struct {
	fsm       *core.AgentFSM
	validator *core.BuildValidator
	config    EngineConfig
	stepsRun  int

	// Event log for the build
	events []GuaranteeEvent
}

// GuaranteeEvent records what happened during guarantee execution.
type GuaranteeEvent struct {
	Type       string                `json:"type"` // "execute", "validate", "retry", "rollback", "pass", "fail"
	StepIndex  int                   `json:"step_index"`
	Attempt    int                   `json:"attempt"`
	Verdict    core.ValidationVerdict `json:"verdict,omitempty"`
	Score      float64               `json:"score,omitempty"`
	Message    string                `json:"message"`
	Timestamp  time.Time             `json:"timestamp"`
	DurationMs int64                 `json:"duration_ms"`
}

// StepResult is the outcome of executing a step through the guarantee engine.
type StepResult struct {
	Success      bool                   `json:"success"`
	Artifacts    []core.BuildArtifact   `json:"artifacts"`
	Validation   *core.ValidationResult `json:"validation"`
	Attempts     int                    `json:"attempts"`
	RolledBack   bool                   `json:"rolled_back"`
	CheckpointID string                 `json:"checkpoint_id,omitempty"`
	Error        string                 `json:"error,omitempty"`
	DurationMs   int64                  `json:"duration_ms"`
}

// NewGuaranteeEngine creates a new engine bound to an FSM and validator.
func NewGuaranteeEngine(fsm *core.AgentFSM, validator *core.BuildValidator, cfg EngineConfig) *GuaranteeEngine {
	return &GuaranteeEngine{
		fsm:       fsm,
		validator: validator,
		config:    cfg,
		events:    make([]GuaranteeEvent, 0, 32),
	}
}

// ExecuteWithGuarantee runs a build step with full validation, retry, and rollback.
// This is the core method that delivers the "100% success guarantee."
func (e *GuaranteeEngine) ExecuteWithGuarantee(ctx context.Context, stepName string, stepFn StepFunc) (*StepResult, error) {
	start := time.Now()
	result := &StepResult{}

	// 1. Create pre-step checkpoint
	var checkpointID string
	if e.stepsRun%e.config.CheckpointEveryN == 0 {
		snapshot, _ := e.fsm.Snapshot()
		cpID, err := e.fsm.CreateCheckpoint(ctx, fmt.Sprintf("pre-%s", stepName), snapshot)
		if err != nil {
			log.Printf("[Guarantee] Warning: failed to create checkpoint: %v", err)
		} else {
			checkpointID = cpID
			result.CheckpointID = cpID
		}
	}

	// 2. Execute with retry loop
	var lastValidation *core.ValidationResult
	var lastArtifacts []core.BuildArtifact

	for attempt := 0; attempt <= e.config.MaxRetries; attempt++ {
		result.Attempts = attempt + 1

		// Build retry context
		retryCtx := &RetryContext{
			AttemptNumber: attempt,
			IsRetry:       attempt > 0,
		}
		if lastValidation != nil {
			retryCtx.PreviousResult = lastValidation
			retryCtx.CorrectionHints = buildCorrectionHints(lastValidation)
		}

		// Log execution event
		e.logEvent("execute", stepName, attempt, "", 0)

		// Transition FSM to executing (or retrying)
		if attempt == 0 {
			// First attempt — FSM should already be in executing state
		} else {
			// Retry attempt
			log.Printf("[Guarantee] Retry %d/%d for step %q", attempt, e.config.MaxRetries, stepName)
			if e.config.RetryBackoff > 0 {
				time.Sleep(e.config.RetryBackoff)
			}
		}

		// Execute the step
		artifacts, err := stepFn(ctx, retryCtx)
		if err != nil {
			e.logEvent("execute_error", fmt.Sprintf("%s: %v", stepName, err), attempt, "", 0)

			if attempt >= e.config.MaxRetries {
				// Exhausted retries — rollback
				return e.handleRollback(ctx, result, checkpointID, start,
					fmt.Sprintf("step %q failed after %d attempts: %v", stepName, attempt+1, err))
			}
			continue
		}

		lastArtifacts = artifacts

		// 3. Validate the output
		validation := e.validator.Validate(ctx, artifacts)
		lastValidation = validation

		e.logEvent("validate", stepName, attempt,
			fmt.Sprintf("verdict=%s score=%.2f", validation.Verdict, validation.Score),
			validation.Duration.Milliseconds())

		// 4. Handle verdict
		switch validation.Verdict {
		case core.VerdictPass:
			// Success
			result.Success = true
			result.Artifacts = artifacts
			result.Validation = validation
			result.DurationMs = time.Since(start).Milliseconds()

			e.logEvent("pass", stepName, attempt,
				fmt.Sprintf("score=%.2f", validation.Score),
				result.DurationMs)

			e.stepsRun++

			// Advance FSM
			if err := e.fsm.Transition(core.EventStepComplete); err != nil {
				log.Printf("[Guarantee] FSM transition warning: %v", err)
			}

			return result, nil

		case core.VerdictSoftFail:
			// Retriable — continue the loop
			e.logEvent("soft_fail", stepName, attempt,
				fmt.Sprintf("score=%.2f errors=%s", validation.Score, validation.ErrorSummary),
				validation.Duration.Milliseconds())

			if attempt >= e.config.MaxRetries {
				// Final attempt also failed — rollback
				return e.handleRollback(ctx, result, checkpointID, start,
					fmt.Sprintf("step %q soft-failed after %d attempts: %s", stepName, attempt+1, validation.ErrorSummary))
			}
			// Continue to next retry iteration

		case core.VerdictHardFail:
			// Non-retriable — immediate rollback
			e.logEvent("hard_fail", stepName, attempt,
				fmt.Sprintf("score=%.2f errors=%s", validation.Score, validation.ErrorSummary),
				validation.Duration.Milliseconds())

			if e.config.FailFastOnHardFail {
				return e.handleRollback(ctx, result, checkpointID, start,
					fmt.Sprintf("step %q hard-failed: %s", stepName, validation.ErrorSummary))
			}
			// In non-strict mode, treat as soft fail
			if attempt >= e.config.MaxRetries {
				return e.handleRollback(ctx, result, checkpointID, start,
					fmt.Sprintf("step %q failed: %s", stepName, validation.ErrorSummary))
			}
		}
	}

	// Should not reach here, but handle gracefully
	result.Success = false
	result.Artifacts = lastArtifacts
	result.Validation = lastValidation
	result.DurationMs = time.Since(start).Milliseconds()
	result.Error = "guarantee loop exited unexpectedly"
	return result, nil
}

// handleRollback performs rollback and returns the failed result.
func (e *GuaranteeEngine) handleRollback(ctx context.Context, result *StepResult, checkpointID string, start time.Time, reason string) (*StepResult, error) {
	result.Success = false
	result.Error = reason
	result.DurationMs = time.Since(start).Milliseconds()

	if checkpointID != "" {
		e.logEvent("rollback", reason, result.Attempts-1, checkpointID, 0)

		cp, err := e.fsm.RollbackTo(ctx, checkpointID)
		if err != nil {
			log.Printf("[Guarantee] Rollback failed: %v", err)
			result.Error = fmt.Sprintf("%s (rollback also failed: %v)", reason, err)
			// Transition to failed state
			_ = e.fsm.TransitionWithMeta(core.EventRollbackFailed, reason)
		} else {
			result.RolledBack = true
			log.Printf("[Guarantee] Rolled back to checkpoint %s (state=%s step=%d)",
				cp.ID, cp.State, cp.StepIndex)
			_ = e.fsm.Transition(core.EventRollbackComplete)
		}
	} else {
		// No checkpoint available
		_ = e.fsm.TransitionWithMeta(core.EventFatalError, reason)
	}

	return result, fmt.Errorf("guarantee failed: %s", reason)
}

// --- Correction Hints ---

// buildCorrectionHints generates hints for the AI based on validation failures.
func buildCorrectionHints(validation *core.ValidationResult) []string {
	var hints []string

	for _, check := range validation.Checks {
		if check.Passed {
			continue
		}

		switch check.Name {
		case "placeholder_scan":
			hints = append(hints, "CRITICAL: Remove ALL placeholder text, TODO comments, and stub implementations. Every function must have real, working code.")
		case "no_empty_files":
			hints = append(hints, "Some files are empty. Ensure every file has complete, functional content.")
		case "syntax_sanity":
			hints = append(hints, "There are bracket/parenthesis mismatches. Check all opening and closing delimiters.")
		case "smoke_test":
			hints = append(hints, "The smoke test failed. Ensure the code compiles/runs without errors. Check imports and dependencies.")
		case "import_check":
			hints = append(hints, "Some import paths look invalid. Use real package names, not placeholders.")
		}
	}

	// Add specific placeholder hints
	if len(validation.Placeholders) > 0 {
		seen := make(map[string]bool)
		for _, p := range validation.Placeholders {
			key := fmt.Sprintf("%s:%d", p.FilePath, p.Line)
			if !seen[key] {
				hints = append(hints, fmt.Sprintf("Replace placeholder at %s line %d: %q", p.FilePath, p.Line, p.Match))
				seen[key] = true
			}
			if len(hints) > 10 {
				hints = append(hints, fmt.Sprintf("...and %d more placeholder(s)", len(validation.Placeholders)-10))
				break
			}
		}
	}

	return hints
}

// --- Event Logging ---

func (e *GuaranteeEngine) logEvent(eventType, message string, attempt int, detail string, durationMs int64) {
	event := GuaranteeEvent{
		Type:       eventType,
		StepIndex:  e.fsm.StepIndex(),
		Attempt:    attempt,
		Message:    message,
		Timestamp:  time.Now(),
		DurationMs: durationMs,
	}
	e.events = append(e.events, event)
}

// Events returns the full event log.
func (e *GuaranteeEngine) Events() []GuaranteeEvent {
	return e.events
}

// StepsCompleted returns how many steps have passed validation.
func (e *GuaranteeEngine) StepsCompleted() int {
	return e.stepsRun
}
