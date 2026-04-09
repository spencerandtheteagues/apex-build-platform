package agents

import (
	"strings"
	"time"
)

type BuildFailureCategory string

const (
	FailureCategoryPlanning     BuildFailureCategory = "planning"
	FailureCategoryGeneration   BuildFailureCategory = "generation"
	FailureCategoryCompile      BuildFailureCategory = "compile"
	FailureCategoryPreviewBoot  BuildFailureCategory = "preview_boot"
	FailureCategoryVisual       BuildFailureCategory = "visual"
	FailureCategoryInteraction  BuildFailureCategory = "interaction"
	FailureCategoryContract     BuildFailureCategory = "contract"
	FailureCategoryRuntime      BuildFailureCategory = "runtime"
	FailureCategoryDeployment   BuildFailureCategory = "deployment"
	FailureCategoryVerification BuildFailureCategory = "verification"
	FailureCategoryBudget       BuildFailureCategory = "budget"
	FailureCategoryUnknown      BuildFailureCategory = "unknown"
)

type BuildFailureRecord struct {
	Category    BuildFailureCategory `json:"category,omitempty"`
	Class       string               `json:"class,omitempty"`
	Phase       string               `json:"phase,omitempty"`
	Surface     ContractSurface      `json:"surface,omitempty"`
	MessageType string               `json:"message_type,omitempty"`
	RecordedAt  time.Time            `json:"recorded_at"`
}

type BuildFailureTaxonomy struct {
	CurrentCategory BuildFailureCategory `json:"current_category,omitempty"`
	CurrentClass    string               `json:"current_class,omitempty"`
	CurrentPhase    string               `json:"current_phase,omitempty"`
	CurrentSurface  ContractSurface      `json:"current_surface,omitempty"`
	LastCategory    BuildFailureCategory `json:"last_category,omitempty"`
	LastClass       string               `json:"last_class,omitempty"`
	LastPhase       string               `json:"last_phase,omitempty"`
	LastSurface     ContractSurface      `json:"last_surface,omitempty"`
	LastFailureAt   *time.Time           `json:"last_failure_at,omitempty"`
	Counts          map[string]int       `json:"counts,omitempty"`
	Recent          []BuildFailureRecord `json:"recent,omitempty"`
}

const (
	maxBuildFailureRecords    = 16
	buildFailureDedupInterval = 10 * time.Second
	verificationReportMsgType = "verification_report"
)

func ensureBuildFailureTaxonomy(state *BuildSnapshotState) *BuildFailureTaxonomy {
	if state == nil {
		return nil
	}
	if state.FailureTaxonomy == nil {
		state.FailureTaxonomy = &BuildFailureTaxonomy{}
	}
	if state.FailureTaxonomy.Counts == nil {
		state.FailureTaxonomy.Counts = map[string]int{}
	}
	return state.FailureTaxonomy
}

func clearCurrentBuildFailureTaxonomy(state *BuildSnapshotState) {
	if state == nil || state.FailureTaxonomy == nil {
		return
	}
	state.FailureTaxonomy.CurrentCategory = ""
	state.FailureTaxonomy.CurrentClass = ""
	state.FailureTaxonomy.CurrentPhase = ""
	state.FailureTaxonomy.CurrentSurface = ""
}

func recordBuildFailureTaxonomy(state *BuildSnapshotState, record BuildFailureRecord) {
	if state == nil {
		return
	}
	record.Class = normalizeFailureIdentifier(record.Class)
	record.Phase = strings.TrimSpace(record.Phase)
	record.MessageType = strings.TrimSpace(record.MessageType)
	record.Surface = normalizeFailureSurface(record.Surface)
	if record.RecordedAt.IsZero() {
		record.RecordedAt = time.Now().UTC()
	}
	if record.Class == "" {
		record.Class = "build_failure"
	}
	if record.Category == "" {
		record.Category = inferFailureCategory(record.Class, record.Phase, record.Surface, "")
	}
	if record.Category == "" {
		record.Category = FailureCategoryUnknown
	}

	taxonomy := ensureBuildFailureTaxonomy(state)
	if taxonomy == nil {
		return
	}

	isDuplicate := false
	if n := len(taxonomy.Recent); n > 0 {
		last := taxonomy.Recent[n-1]
		if last.Category == record.Category &&
			last.Class == record.Class &&
			last.Phase == record.Phase &&
			last.Surface == record.Surface &&
			record.RecordedAt.Sub(last.RecordedAt) <= buildFailureDedupInterval {
			isDuplicate = true
		}
	}

	taxonomy.CurrentCategory = record.Category
	taxonomy.CurrentClass = record.Class
	taxonomy.CurrentPhase = record.Phase
	taxonomy.CurrentSurface = record.Surface
	taxonomy.LastCategory = record.Category
	taxonomy.LastClass = record.Class
	taxonomy.LastPhase = record.Phase
	taxonomy.LastSurface = record.Surface
	recordedAt := record.RecordedAt
	taxonomy.LastFailureAt = &recordedAt

	if isDuplicate {
		return
	}

	taxonomy.Counts[string(record.Category)]++
	taxonomy.Recent = append(taxonomy.Recent, record)
	if len(taxonomy.Recent) > maxBuildFailureRecords {
		taxonomy.Recent = append([]BuildFailureRecord(nil), taxonomy.Recent[len(taxonomy.Recent)-maxBuildFailureRecords:]...)
	}
}

func applyBuildMessageFailureTaxonomy(state *BuildSnapshotState, msg *WSMessage, data map[string]any) {
	if state == nil || msg == nil {
		return
	}
	switch msg.Type {
	case WSBuildCompleted, WSBuildFSMValidationPass:
		clearCurrentBuildFailureTaxonomy(state)
		return
	}
	record, ok := inferBuildFailureRecord(msg, data, state.CurrentPhase)
	if !ok {
		return
	}
	recordBuildFailureTaxonomy(state, record)
}

func applyVerificationReportFailureTaxonomyLocked(state *BuildSnapshotState, report VerificationReport) {
	if state == nil {
		return
	}
	switch report.Status {
	case VerificationPassed:
		clearCurrentBuildFailureTaxonomy(state)
		return
	case VerificationFailed, VerificationBlocked:
	default:
		return
	}
	record, ok := inferVerificationFailureRecord(report)
	if !ok {
		return
	}
	recordBuildFailureTaxonomy(state, record)
}

func inferBuildFailureRecord(msg *WSMessage, data map[string]any, currentPhase string) (BuildFailureRecord, bool) {
	if msg == nil {
		return BuildFailureRecord{}, false
	}
	switch msg.Type {
	case WSBuildError, WSBuildFSMValidationFail, WSBuildFSMRetryExhausted, WSBuildFSMRollbackDone, WSBuildFSMRollbackFail, WSBuildFSMFatalError:
	default:
		return BuildFailureRecord{}, false
	}

	phase := firstBuildActivityString(
		buildActivityString(data["phase_key"]),
		buildActivityString(data["phase"]),
		buildActivityString(data["quality_gate_stage"]),
		currentPhase,
	)
	details := buildFailureDetails(msg, data)
	class := extractExplicitFailureClassFromActivity(data)
	if class == "" {
		class = inferFailureClassFromActivity(phase, details)
	}
	surface := inferFailureSurface(phase, class, normalizeFailureSurfaceName(buildActivityString(data["surface"])), details)
	category := inferFailureCategory(class, phase, surface, details)
	return BuildFailureRecord{
		Category:    category,
		Class:       class,
		Phase:       phase,
		Surface:     surface,
		MessageType: string(msg.Type),
		RecordedAt:  time.Now().UTC(),
	}, true
}

func inferVerificationFailureRecord(report VerificationReport) (BuildFailureRecord, bool) {
	phase := strings.TrimSpace(report.Phase)
	if phase == "" {
		phase = "verification"
	}
	details := firstNonEmptyString(
		strings.Join(report.Errors, "; "),
		strings.Join(report.Blockers, "; "),
		strings.Join(report.Warnings, "; "),
	)
	class := extractFailureClassToken(report.ChecksRun)
	if class == "" {
		for _, blocker := range report.Blockers {
			if suffix, ok := strings.CutPrefix(strings.TrimSpace(blocker), "preview_verification_failed:"); ok {
				class = previewFailureClass(suffix)
				break
			}
		}
	}
	if class == "" {
		switch {
		case looksLikeInteractionIssue(details):
			class = "interaction_canary"
		case looksLikeVisualIssue(details):
			class = "visual_layout"
		case strings.Contains(strings.ToLower(phase), "preview"):
			class = "preview_verification"
		default:
			class = "verification_failure"
		}
	}
	surface := normalizeFailureSurface(report.Surface)
	category := inferFailureCategory(class, phase, surface, details)
	return BuildFailureRecord{
		Category:    category,
		Class:       class,
		Phase:       phase,
		Surface:     surface,
		MessageType: verificationReportMsgType + ":" + string(report.Status),
		RecordedAt:  time.Now().UTC(),
	}, true
}

func buildFailureDetails(msg *WSMessage, data map[string]any) string {
	return firstNonEmptyString(
		buildActivityString(data["error"]),
		buildActivityString(data["details"]),
		buildActivityString(data["message"]),
		buildActivityString(data["summary"]),
		strings.Join(buildActivityStringSlice(data["errors"]), "; "),
		buildActivityString(data["content"]),
	)
}

func extractExplicitFailureClassFromActivity(data map[string]any) string {
	if len(data) == 0 {
		return ""
	}
	if class := normalizeFailureIdentifier(buildActivityString(data["failure_class"])); class != "" {
		return class
	}
	if token := extractFailureClassToken(buildActivityStringSlice(data["checks_run"])); token != "" {
		return token
	}
	if token := extractFailureClassToken(buildActivityStringSlice(data["checks"])); token != "" {
		return token
	}
	if kind := buildActivityString(data["failure_kind"]); kind != "" {
		return normalizeFailureIdentifier(previewFailureClass(kind))
	}
	return ""
}

func extractFailureClassToken(values []string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		switch {
		case strings.HasPrefix(trimmed, "failure_class:"):
			return normalizeFailureIdentifier(strings.TrimPrefix(trimmed, "failure_class:"))
		case strings.HasPrefix(trimmed, "failure_class="):
			return normalizeFailureIdentifier(strings.TrimPrefix(trimmed, "failure_class="))
		}
	}
	return ""
}

func inferFailureClassFromActivity(phase string, details string) string {
	lowerPhase := strings.ToLower(strings.TrimSpace(phase))
	lowerDetails := strings.ToLower(strings.TrimSpace(details))
	switch {
	case strings.Contains(lowerPhase, "planning"), strings.Contains(lowerPhase, "architecture"):
		if strings.Contains(lowerDetails, "timeout") || strings.Contains(lowerDetails, "deadline") {
			return "timeout"
		}
		return "build_failure"
	case strings.Contains(lowerPhase, "preview"):
		if looksLikeInteractionIssue(details) {
			return "interaction_canary"
		}
		if looksLikeVisualIssue(details) {
			return "visual_layout"
		}
		if strings.Contains(lowerDetails, "boot") {
			return "preview_boot"
		}
		if strings.Contains(lowerDetails, "runtime") {
			return "runtime"
		}
		return "preview_verification"
	case looksLikeCompileIssue(details):
		return "compile_failure"
	default:
		return normalizeFailureClass(details)
	}
}

func inferFailureCategory(class string, phase string, surface ContractSurface, details string) BuildFailureCategory {
	lowerPhase := strings.ToLower(strings.TrimSpace(phase))
	switch normalizeFailureIdentifier(class) {
	case "contract_violation", "coordination_violation", "contract_verification_blocked", "backend_contract":
		return FailureCategoryContract
	case "verification_failure", "final_validation_failure":
		return FailureCategoryVerification
	case "preview_verification", "preview_boot", "frontend_shell", "browser_unavailable", "infrastructure":
		return FailureCategoryPreviewBoot
	case "visual_layout", "visual":
		return FailureCategoryVisual
	case "interaction_canary", "interaction":
		return FailureCategoryInteraction
	case "compile_failure":
		return FailureCategoryCompile
	case "runtime":
		if surface == SurfaceFrontend {
			return FailureCategoryPreviewBoot
		}
		return FailureCategoryRuntime
	case "truncation":
		return FailureCategoryGeneration
	case "timeout":
		switch {
		case strings.Contains(lowerPhase, "planning"), strings.Contains(lowerPhase, "architecture"):
			return FailureCategoryPlanning
		case strings.Contains(lowerPhase, "deploy"):
			return FailureCategoryDeployment
		case strings.Contains(lowerPhase, "preview"):
			return FailureCategoryPreviewBoot
		case strings.Contains(lowerPhase, "validation"), strings.Contains(lowerPhase, "review"), strings.Contains(lowerPhase, "testing"):
			return FailureCategoryVerification
		case surface == SurfaceBackend || surface == SurfaceIntegration:
			return FailureCategoryRuntime
		default:
			return FailureCategoryGeneration
		}
	case "budget":
		return FailureCategoryBudget
	case "build_failure":
		switch {
		case strings.Contains(lowerPhase, "planning"), strings.Contains(lowerPhase, "architecture"):
			return FailureCategoryPlanning
		case strings.Contains(lowerPhase, "deploy"):
			return FailureCategoryDeployment
		case strings.Contains(lowerPhase, "preview"):
			return FailureCategoryPreviewBoot
		case looksLikeCompileIssue(details):
			return FailureCategoryCompile
		case surface == SurfaceBackend || surface == SurfaceIntegration:
			return FailureCategoryRuntime
		default:
			return FailureCategoryGeneration
		}
	}

	switch {
	case strings.Contains(lowerPhase, "planning"), strings.Contains(lowerPhase, "architecture"):
		return FailureCategoryPlanning
	case strings.Contains(lowerPhase, "deploy"):
		return FailureCategoryDeployment
	case strings.Contains(lowerPhase, "preview"):
		if looksLikeInteractionIssue(details) {
			return FailureCategoryInteraction
		}
		if looksLikeVisualIssue(details) {
			return FailureCategoryVisual
		}
		return FailureCategoryPreviewBoot
	case looksLikeCompileIssue(details):
		return FailureCategoryCompile
	case looksLikeInteractionIssue(details):
		return FailureCategoryInteraction
	case looksLikeVisualIssue(details):
		return FailureCategoryVisual
	case surface == SurfaceBackend || surface == SurfaceIntegration:
		return FailureCategoryRuntime
	case surface == SurfaceDeployment:
		return FailureCategoryDeployment
	default:
		return FailureCategoryUnknown
	}
}

func inferFailureSurface(phase string, class string, explicit ContractSurface, details string) ContractSurface {
	if explicit != "" && explicit != SurfaceGlobal {
		return explicit
	}
	lowerPhase := strings.ToLower(strings.TrimSpace(phase))
	switch normalizeFailureIdentifier(class) {
	case "frontend_shell", "preview_verification", "preview_boot", "visual_layout", "visual", "interaction_canary", "interaction":
		return SurfaceFrontend
	case "runtime", "backend_contract":
		if strings.Contains(lowerPhase, "preview") {
			return SurfaceFrontend
		}
		return SurfaceBackend
	}
	switch {
	case strings.Contains(lowerPhase, "frontend"), strings.Contains(lowerPhase, "preview"), looksLikeVisualIssue(details), looksLikeInteractionIssue(details):
		return SurfaceFrontend
	case strings.Contains(lowerPhase, "backend"), strings.Contains(lowerPhase, "database"), strings.Contains(lowerPhase, "data"):
		return SurfaceBackend
	case strings.Contains(lowerPhase, "integration"), strings.Contains(lowerPhase, "testing"), strings.Contains(lowerPhase, "review"):
		return SurfaceIntegration
	case strings.Contains(lowerPhase, "deploy"):
		return SurfaceDeployment
	default:
		return SurfaceGlobal
	}
}

func normalizeFailureIdentifier(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return ""
	}
	if strings.ContainsAny(trimmed, " \t\r\n") {
		return normalizeFailureClass(trimmed)
	}
	replacer := strings.NewReplacer("-", "_", ":", "_", "/", "_")
	trimmed = replacer.Replace(trimmed)
	for strings.Contains(trimmed, "__") {
		trimmed = strings.ReplaceAll(trimmed, "__", "_")
	}
	return strings.Trim(trimmed, "_")
}

func normalizeFailureSurface(surface ContractSurface) ContractSurface {
	return normalizeFailureSurfaceName(string(surface))
}

func normalizeFailureSurfaceName(raw string) ContractSurface {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(SurfaceFrontend):
		return SurfaceFrontend
	case string(SurfaceBackend):
		return SurfaceBackend
	case string(SurfaceData):
		return SurfaceData
	case string(SurfaceIntegration):
		return SurfaceIntegration
	case string(SurfaceDeployment):
		return SurfaceDeployment
	case string(SurfaceGlobal):
		return SurfaceGlobal
	default:
		return SurfaceGlobal
	}
}

func looksLikeCompileIssue(details string) bool {
	lower := strings.ToLower(strings.TrimSpace(details))
	return strings.Contains(lower, "typescript") ||
		strings.Contains(lower, "ts") && strings.Contains(lower, "error") ||
		strings.Contains(lower, "compile") ||
		strings.Contains(lower, "build failed") ||
		strings.Contains(lower, "vite") ||
		strings.Contains(lower, "syntax")
}

func looksLikeVisualIssue(details string) bool {
	lower := strings.ToLower(strings.TrimSpace(details))
	return strings.Contains(lower, "contrast") ||
		strings.Contains(lower, "overflow") ||
		strings.Contains(lower, "overlap") ||
		strings.Contains(lower, "unstyled") ||
		strings.Contains(lower, "layout") ||
		strings.Contains(lower, "screenshot")
}

func looksLikeInteractionIssue(details string) bool {
	lower := strings.ToLower(strings.TrimSpace(details))
	return strings.Contains(lower, "canary") ||
		strings.Contains(lower, "click") ||
		strings.Contains(lower, "interaction")
}
