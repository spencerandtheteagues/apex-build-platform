// pipeline_logger.go — Real-time high-output structured pipeline telemetry.
//
// Every AI call, task execution, compile validation, preview gate decision,
// repair attempt, and build lifecycle event emits a JSON line tagged
// [APEX_PIPELINE] to stdout.  Filter with:
//
//	render logs <svc> | grep '\[APEX_PIPELINE\]'
//
// Enabled by default. Set APEX_PIPELINE_TELEMETRY=false to disable.
package agents

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Pipeline log category tags.
const (
	plAICall  = "AI_CALL"
	plTask    = "TASK"
	plBuild   = "BUILD"
	plCompile = "COMPILE"
	plPreview = "PREVIEW"
	plRepair  = "REPAIR"
	plLearn   = "LEARN"
	plGate    = "GATE"
	plAgent   = "AGENT"
	plStage   = "STAGE"
	plError   = "ERROR"
)

var (
	plRegistry sync.Map // buildID → *PipelineLogger
	plEnabled  bool
)

func init() {
	v := os.Getenv("APEX_PIPELINE_TELEMETRY")
	plEnabled = v != "false" && v != "0" && v != "off"
}

// PipelineLogger is a per-build structured event emitter.
type PipelineLogger struct {
	buildID string
	seq     atomic.Int64
}

type plEvent struct {
	TS      int64          `json:"ts_us"`
	Seq     int64          `json:"seq"`
	BuildID string         `json:"build_id"`
	Cat     string         `json:"cat"`
	Event   string         `json:"event"`
	Data    map[string]any `json:"data,omitempty"`
}

// pLog returns (or creates) the PipelineLogger for a given build ID.
// Returns nil when telemetry is disabled — all methods are nil-safe.
func pLog(buildID string) *PipelineLogger {
	if !plEnabled || buildID == "" {
		return nil
	}
	v, _ := plRegistry.LoadOrStore(buildID, &PipelineLogger{buildID: buildID})
	return v.(*PipelineLogger)
}

// pLogEvict removes a build's logger from the registry (call at build terminal state).
func pLogEvict(buildID string) {
	plRegistry.Delete(buildID)
}

func (pl *PipelineLogger) emit(cat, event string, data map[string]any) {
	if pl == nil {
		return
	}
	ev := plEvent{
		TS:      time.Now().UnixMicro(),
		Seq:     pl.seq.Add(1),
		BuildID: pl.buildID,
		Cat:     cat,
		Event:   event,
		Data:    data,
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	log.Printf("[APEX_PIPELINE] %s", b)
}

// ── Build lifecycle ────────────────────────────────────────────────────────

func (pl *PipelineLogger) BuildStart(mode, powerMode, plan, description string) {
	pl.emit(plBuild, "start", map[string]any{
		"mode":       mode,
		"power_mode": powerMode,
		"plan":       plan,
		"desc_len":   len(description),
	})
}

func (pl *PipelineLogger) BuildStage(stage, prev string) {
	pl.emit(plStage, "transition", map[string]any{
		"from": prev,
		"to":   stage,
	})
}

func (pl *PipelineLogger) BuildEnd(status, failureCategory, failureClass string, durationMS int64, compileAttempts, compileRepairAttempts, previewAttempts, readinessAttempts int) {
	pl.emit(plBuild, "end", map[string]any{
		"status":             status,
		"failure_category":   failureCategory,
		"failure_class":      failureClass,
		"duration_ms":        durationMS,
		"compile_attempts":   compileAttempts,
		"compile_repairs":    compileRepairAttempts,
		"preview_attempts":   previewAttempts,
		"readiness_attempts": readinessAttempts,
		"first_pass_success": compileRepairAttempts == 0 && previewAttempts == 0 && readinessAttempts == 0 && status == "completed",
	})
	pLogEvict(pl.buildID)
}

// ── Agent lifecycle ────────────────────────────────────────────────────────

func (pl *PipelineLogger) AgentAssigned(agentID, role, provider, model string) {
	pl.emit(plAgent, "assigned", map[string]any{
		"agent_id": agentID,
		"role":     role,
		"provider": provider,
		"model":    model,
	})
}

func (pl *PipelineLogger) AgentProviderSwitch(agentID, role, fromProvider, toProvider, reason string) {
	pl.emit(plAgent, "provider_switch", map[string]any{
		"agent_id":      agentID,
		"role":          role,
		"from_provider": fromProvider,
		"to_provider":   toProvider,
		"reason":        reason,
	})
}

// ── Task lifecycle ─────────────────────────────────────────────────────────

func (pl *PipelineLogger) TaskStart(taskID, taskType, agentID, role, provider, model string, retryCount int) {
	pl.emit(plTask, "start", map[string]any{
		"task_id":     taskID,
		"task_type":   taskType,
		"agent_id":    agentID,
		"role":        role,
		"provider":    provider,
		"model":       model,
		"retry_count": retryCount,
	})
}

// TaskDone returns a closure to call when the task finishes.
// Usage: done := pl.TaskStart(...); defer done(success, errMsg, fileCount)
func (pl *PipelineLogger) TaskDone(taskID, taskType, agentID, role, provider string) func(success bool, errMsg string, fileCount int) {
	start := time.Now()
	return func(success bool, errMsg string, fileCount int) {
		if len(errMsg) > 300 {
			errMsg = errMsg[:300] + "…"
		}
		pl.emit(plTask, "done", map[string]any{
			"task_id":     taskID,
			"task_type":   taskType,
			"agent_id":    agentID,
			"role":        role,
			"provider":    provider,
			"success":     success,
			"error":       errMsg,
			"file_count":  fileCount,
			"duration_ms": time.Since(start).Milliseconds(),
		})
	}
}

func (pl *PipelineLogger) TaskRetry(taskID, taskType, strategy, reason string, attempt int) {
	pl.emit(plTask, "retry", map[string]any{
		"task_id":   taskID,
		"task_type": taskType,
		"strategy":  strategy,
		"reason":    reason,
		"attempt":   attempt,
	})
}

// ── AI call ────────────────────────────────────────────────────────────────

// AICallStart logs an AI request and returns a completion closure.
// Usage:
//
//	done := pl.AICallStart(provider, model, capability, promptLen, maxTokens)
//	resp, err := router.Generate(...)
//	done(resp, err)
func (pl *PipelineLogger) AICallStart(provider, model, capability string, promptLen, maxTokens int) func(outputLen, inputTok, outputTok int, costUSD float64, err error) {
	start := time.Now()
	if pl != nil {
		pl.emit(plAICall, "start", map[string]any{
			"provider":   provider,
			"model":      model,
			"capability": capability,
			"prompt_len": promptLen,
			"max_tokens": maxTokens,
		})
	}
	return func(outputLen, inputTok, outputTok int, costUSD float64, err error) {
		status := "ok"
		errMsg := ""
		if err != nil {
			status = "error"
			errMsg = err.Error()
			if len(errMsg) > 300 {
				errMsg = errMsg[:300] + "…"
			}
		}
		if pl != nil {
			pl.emit(plAICall, "done", map[string]any{
				"provider":    provider,
				"model":       model,
				"capability":  capability,
				"status":      status,
				"error":       errMsg,
				"output_len":  outputLen,
				"input_tok":   inputTok,
				"output_tok":  outputTok,
				"cost_usd":    costUSD,
				"duration_ms": time.Since(start).Milliseconds(),
			})
		}
	}
}

// ── Compile validation ─────────────────────────────────────────────────────

func (pl *PipelineLogger) CompileCheck(tool string, errorCount int, errors []string) {
	sample := errors
	if len(sample) > 5 {
		sample = sample[:5]
	}
	pl.emit(plCompile, "check", map[string]any{
		"tool":        tool,
		"error_count": errorCount,
		"errors":      sample,
		"passed":      errorCount == 0,
	})
}

func (pl *PipelineLogger) CompileRepairStart(strategy string, attempt int, fileCount int) {
	pl.emit(plRepair, "compile_start", map[string]any{
		"strategy":   strategy,
		"attempt":    attempt,
		"file_count": fileCount,
	})
}

func (pl *PipelineLogger) CompileRepairDone(strategy string, attempt int, passed bool, remainingErrors int) {
	pl.emit(plRepair, "compile_done", map[string]any{
		"strategy":         strategy,
		"attempt":          attempt,
		"passed":           passed,
		"remaining_errors": remainingErrors,
	})
}

func (pl *PipelineLogger) HydraStart(strategies []string, fileCount int) {
	pl.emit(plRepair, "hydra_start", map[string]any{
		"strategies": strategies,
		"file_count": fileCount,
	})
}

func (pl *PipelineLogger) HydraResult(strategy string, passed bool, errorCount int) {
	pl.emit(plRepair, "hydra_result", map[string]any{
		"strategy":    strategy,
		"passed":      passed,
		"error_count": errorCount,
	})
}

func (pl *PipelineLogger) HydraDone(winnerStrategy string, found bool) {
	pl.emit(plRepair, "hydra_done", map[string]any{
		"winner_strategy": winnerStrategy,
		"winner_found":    found,
	})
}

// ── Preview gate ───────────────────────────────────────────────────────────

func (pl *PipelineLogger) PreviewGateStart(fileCount int, fullStack bool) {
	pl.emit(plPreview, "gate_start", map[string]any{
		"file_count": fileCount,
		"full_stack": fullStack,
	})
}

func (pl *PipelineLogger) PreviewGateDone(passed bool, failureKind, visionSeverity string, canaryClicked, canaryErrors int, durationMS int64) {
	pl.emit(plPreview, "gate_done", map[string]any{
		"passed":          passed,
		"failure_kind":    failureKind,
		"vision_severity": visionSeverity,
		"canary_clicked":  canaryClicked,
		"canary_errors":   canaryErrors,
		"duration_ms":     durationMS,
	})
}

func (pl *PipelineLogger) VisionRepairStart(warnings []string) {
	pl.emit(plRepair, "vision_start", map[string]any{
		"warning_count": len(warnings),
		"warnings":      warnings,
	})
}

func (pl *PipelineLogger) VisionRepairDone(passed bool, attempt int) {
	pl.emit(plRepair, "vision_done", map[string]any{
		"passed":  passed,
		"attempt": attempt,
	})
}

// ── Build learning ─────────────────────────────────────────────────────────

func (pl *PipelineLogger) LearnLookup(scope string, snapshotCount int, hasInsights bool) {
	pl.emit(plLearn, "lookup", map[string]any{
		"scope":          scope,
		"snapshot_count": snapshotCount,
		"has_insights":   hasInsights,
	})
}

// ── Readiness recovery ─────────────────────────────────────────────────────

func (pl *PipelineLogger) ReadinessRecoveryStart(attempt int, errorSummary string) {
	if len(errorSummary) > 300 {
		errorSummary = errorSummary[:300] + "…"
	}
	pl.emit(plRepair, "readiness_start", map[string]any{
		"attempt":       attempt,
		"error_summary": errorSummary,
	})
}

func (pl *PipelineLogger) ReadinessRecoveryDone(attempt int, launched bool) {
	pl.emit(plRepair, "readiness_done", map[string]any{
		"attempt":  attempt,
		"launched": launched,
	})
}

// ── Gate decisions ─────────────────────────────────────────────────────────

func (pl *PipelineLogger) Gate(name, result, reason string) {
	pl.emit(plGate, name, map[string]any{
		"result": result,
		"reason": reason,
	})
}

// ── Errors ─────────────────────────────────────────────────────────────────

func (pl *PipelineLogger) Error(component, event, msg string) {
	if len(msg) > 500 {
		msg = msg[:500] + "…"
	}
	pl.emit(plError, event, map[string]any{
		"component": component,
		"message":   msg,
	})
}
