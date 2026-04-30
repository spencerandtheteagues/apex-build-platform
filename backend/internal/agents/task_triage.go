package agents

import "strings"

// RepairComplexity classifies how hard a repair is expected to be.
type RepairComplexity string

const (
	RepairSimple  RepairComplexity = "simple"  // Single-file, clear error pattern
	RepairMedium  RepairComplexity = "medium"  // Multi-file or ambiguous error
	RepairComplex RepairComplexity = "complex" // Cross-surface or repeated failure
)

// TaskTriageResult is the enriched output of triageTaskForWaterfall.
type TaskTriageResult struct {
	TaskShape         TaskShape
	RiskLevel         TaskRiskLevel
	Scope             string
	LocalRepair       bool
	CrossSurface      bool
	RepairComplexity  RepairComplexity
	ShouldUseDualCand bool // recommend dual-candidate routing
	ShouldUseVerifier bool // recommend single-with-verifier routing
	RoutingSuggestion ProviderRoutingMode
}

// triageTaskForWaterfall derives routing metadata from a task.
// It goes beyond the compiled work-order by inspecting description text,
// file counts, and retry depth to produce actionable routing hints.
func triageTaskForWaterfall(task *Task) TaskTriageResult {
	compiled := compileTaskWorkOrder(task)
	result := TaskTriageResult{
		TaskShape: compiled.TaskShape,
		RiskLevel: compiled.RiskLevel,
		Scope:     compiled.Scope,
	}
	if result.TaskShape == "" {
		result.TaskShape = TaskShapeRepair
	}
	if result.RiskLevel == "" {
		result.RiskLevel = RiskMedium
	}
	result.CrossSurface = result.Scope == "cross_surface"
	result.LocalRepair = result.Scope == "local" && (result.TaskShape == TaskShapeRepair ||
		result.TaskShape == TaskShapeFrontendPatch ||
		result.TaskShape == TaskShapeBackendPatch)

	// ── Enrich risk level from description signals ────────────────────────
	if task != nil {
		inputSnapshot := cloneTaskInputForSnapshot(task)
		inputDesc, _ := inputSnapshot["description"].(string)
		inputPrompt, _ := inputSnapshot["prompt"].(string)
		desc := strings.ToLower(task.Description + " " + inputDesc + " " + inputPrompt)
		if containsAny(desc, "migration", "migrate", "schema change", "drop table", "alter table") {
			result.RiskLevel = escalateRisk(result.RiskLevel, RiskHigh)
			result.TaskShape = TaskShapeSchema
		}
		if containsAny(desc, "auth", "authentication", "authorization", "jwt", "session", "password", "token", "csrf", "cors", "security") {
			result.RiskLevel = escalateRisk(result.RiskLevel, RiskHigh)
		}
		if containsAny(desc, "deploy", "production", "release", "publish", "infrastructure") {
			result.RiskLevel = escalateRisk(result.RiskLevel, RiskHigh)
		}
		if containsAny(desc, "database", "data loss", "irreversible", "delete all", "truncate") {
			result.RiskLevel = escalateRisk(result.RiskLevel, RiskCritical)
		}

		// Cross-surface signals in text even when no explicit file list
		if result.Scope == "unknown" && containsAny(desc, "frontend and backend", "api and ui", "full stack", "end-to-end") {
			result.CrossSurface = true
			result.Scope = "cross_surface"
		}
	}

	// ── Escalate risk on retries ──────────────────────────────────────────
	if task != nil && task.RetryCount >= 2 && result.RiskLevel != RiskCritical {
		result.RiskLevel = escalateRisk(result.RiskLevel, RiskHigh)
	}

	// ── File count heuristics ─────────────────────────────────────────────
	ownedCount := len(compiled.OwnedFiles)
	if ownedCount > 10 {
		result.RiskLevel = escalateRisk(result.RiskLevel, RiskHigh)
		result.CrossSurface = true
	}

	// ── Repair complexity ─────────────────────────────────────────────────
	switch {
	case result.RiskLevel == RiskCritical || result.CrossSurface || (task != nil && task.RetryCount >= 3):
		result.RepairComplexity = RepairComplex
	case result.RiskLevel == RiskHigh || ownedCount > 3 || (task != nil && task.RetryCount >= 1):
		result.RepairComplexity = RepairMedium
	default:
		result.RepairComplexity = RepairSimple
	}

	// ── Routing suggestion ────────────────────────────────────────────────
	switch result.RepairComplexity {
	case RepairComplex:
		result.ShouldUseDualCand = true
		result.RoutingSuggestion = RoutingModeDualCandidate
	case RepairMedium:
		result.ShouldUseVerifier = true
		result.RoutingSuggestion = RoutingModeSingleWithVerifier
	default:
		result.RoutingSuggestion = RoutingModeSingleProvider
	}

	// Verification tasks always want a verifier
	if result.TaskShape == TaskShapeVerification {
		result.ShouldUseVerifier = true
		if result.RoutingSuggestion == RoutingModeSingleProvider {
			result.RoutingSuggestion = RoutingModeSingleWithVerifier
		}
	}

	return result
}

// escalateRisk returns the higher of two risk levels.
func escalateRisk(current, floor TaskRiskLevel) TaskRiskLevel {
	order := map[TaskRiskLevel]int{RiskLow: 0, RiskMedium: 1, RiskHigh: 2, RiskCritical: 3}
	if order[floor] > order[current] {
		return floor
	}
	return current
}

// containsAny returns true if s contains any of the given substrings.
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
