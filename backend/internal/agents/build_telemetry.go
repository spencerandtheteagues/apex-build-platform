// build_telemetry.go — Structured build quality metrics emission.
//
// Emits a single JSON log line tagged [quality_telemetry] at the end of every
// build's terminal state (completed, failed, or cancelled). These lines are
// designed to be machine-parseable by any log aggregator (Render log drain,
// Datadog, Loki, etc.) for dashboarding the Phase 3 success metrics:
//   - first-pass success rate
//   - preview gate pass rate
//   - canary interaction coverage
//   - compile and repair attempt counts
//   - time-to-complete
package agents

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"
)

// BuildQualityMetrics captures per-build quality signals emitted at terminal state.
// All fields are omitempty so absent signals don't inflate log volume.
type BuildQualityMetrics struct {
	BuildID          string `json:"build_id"`
	UserID           uint   `json:"user_id"`
	Status           string `json:"status"`
	Mode             string `json:"mode,omitempty"`
	PowerMode        string `json:"power_mode,omitempty"`
	SubscriptionPlan string `json:"subscription_plan,omitempty"`
	DeliveryMode     string `json:"delivery_mode,omitempty"`
	CanaryCohort     string `json:"canary_cohort,omitempty"`

	// Repair attempt counts — each non-zero signals a first-pass failure.
	FirstPassSuccess     bool `json:"first_pass_success"`
	CompileRepairCount   int  `json:"compile_repair_count,omitempty"`
	ReadinessRepairCount int  `json:"readiness_repair_count,omitempty"`
	PreviewRepairCount   int  `json:"preview_repair_count,omitempty"`

	// Preview gate and canary signals.
	PreviewGatePassed       *bool `json:"preview_gate_passed,omitempty"` // nil = skipped
	PreviewGateSkipped      bool  `json:"preview_gate_skipped,omitempty"`
	CanaryClicked           int   `json:"canary_clicked,omitempty"`
	CanaryErrorCount        int   `json:"canary_error_count,omitempty"`
	VisionReviewed          bool  `json:"vision_reviewed,omitempty"`
	VisualWarningCount      int   `json:"visual_warning_count,omitempty"`
	InteractionWarningCount int   `json:"interaction_warning_count,omitempty"`

	// Latest verification state by phase/surface stream.
	VerificationStatusByPhase map[string]string `json:"verification_status_by_phase,omitempty"`
	AdvisoryClasses           []string          `json:"advisory_classes,omitempty"`
	RecurringFailureClasses   []string          `json:"recurring_failure_classes,omitempty"`

	// Failure classification (empty on success).
	FailureCategory string `json:"failure_category,omitempty"`
	FailureClass    string `json:"failure_class,omitempty"`

	// Volume and duration.
	FileCount     int   `json:"file_count"`
	TestFileCount int   `json:"test_file_count,omitempty"`
	DurationMS    int64 `json:"duration_ms,omitempty"`

	GeneratedAt time.Time `json:"generated_at"`
}

// emitBuildQualityTelemetry derives and emits a [quality_telemetry] log line.
// Safe to call with a nil build. Respects APEX_BUILD_QUALITY_TELEMETRY=false to disable.
func emitBuildQualityTelemetry(build *Build, allFiles []GeneratedFile, now time.Time) {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APEX_BUILD_QUALITY_TELEMETRY")), "false") {
		return
	}
	m := deriveBuildQualityMetrics(build, allFiles, now)
	if m == nil {
		return
	}
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	log.Printf("[quality_telemetry] %s", string(data))
}

func countTestFiles(files []GeneratedFile) int {
	n := 0
	for _, f := range files {
		if isTestFile(f.Path) {
			n++
		}
	}
	return n
}

func deriveBuildQualityMetrics(build *Build, allFiles []GeneratedFile, now time.Time) *BuildQualityMetrics {
	if build == nil {
		return nil
	}
	build.mu.RLock()
	defer build.mu.RUnlock()

	m := &BuildQualityMetrics{
		BuildID:              build.ID,
		UserID:               build.UserID,
		Status:               string(build.Status),
		Mode:                 string(build.Mode),
		PowerMode:            string(build.PowerMode),
		SubscriptionPlan:     strings.TrimSpace(build.SubscriptionPlan),
		DeliveryMode:         buildCurrentDeliveryMode(build),
		CanaryCohort:         buildCanaryCohort(build),
		CompileRepairCount:   build.CompileValidationRepairs,
		ReadinessRepairCount: build.ReadinessRecoveryAttempts,
		PreviewRepairCount:   build.PreviewVerificationAttempts,
		FileCount:            len(allFiles),
		TestFileCount:        countTestFiles(allFiles),
		GeneratedAt:          now.UTC(),
	}

	// Duration: prefer CompletedAt if set, fall back to now.
	if !build.CreatedAt.IsZero() {
		end := now
		if build.CompletedAt != nil {
			end = *build.CompletedAt
		}
		m.DurationMS = end.Sub(build.CreatedAt).Milliseconds()
	}

	// First-pass: completed with zero repair attempts.
	m.FirstPassSuccess = build.Status == BuildCompleted &&
		build.CompileValidationRepairs == 0 &&
		build.PreviewVerificationAttempts == 0 &&
		build.ReadinessRecoveryAttempts == 0

	// Failure taxonomy — use current category/class if set, fall back to last.
	if ft := build.SnapshotState.FailureTaxonomy; ft != nil {
		m.FailureCategory = string(ft.CurrentCategory)
		if m.FailureCategory == "" {
			m.FailureCategory = string(ft.LastCategory)
		}
		m.FailureClass = strings.TrimSpace(ft.CurrentClass)
		if m.FailureClass == "" {
			m.FailureClass = strings.TrimSpace(ft.LastClass)
		}
	}

	// Preview gate and canary from VerificationReports.
	previewGateFound := false
	if orch := build.SnapshotState.Orchestration; orch != nil {
		latestReports := latestVerificationReports(orch.VerificationReports)
		m.VerificationStatusByPhase = verificationStatusByPhase(latestReports)
		m.AdvisoryClasses, _ = deriveReliabilityAdvisories(latestReports)
		m.RecurringFailureClasses = deriveRecurringFailureClasses(orch.FailureFingerprints)
		for _, report := range orch.VerificationReports {
			if strings.TrimSpace(report.Phase) != "preview_verification" {
				for _, warning := range report.Warnings {
					switch {
					case strings.HasPrefix(strings.TrimSpace(warning), "visual:"):
						m.VisualWarningCount++
					case strings.HasPrefix(strings.TrimSpace(warning), "interaction:"):
						m.InteractionWarningCount++
					}
				}
				continue
			}
			previewGateFound = true
			switch report.Status {
			case VerificationPassed:
				passed := true
				m.PreviewGatePassed = &passed
			case VerificationFailed, VerificationBlocked:
				if m.PreviewGatePassed == nil {
					passed := false
					m.PreviewGatePassed = &passed
				}
			}
			// Accumulate canary signals across all preview reports (repair retries may add more).
			m.CanaryClicked += report.CanaryClickCount
			m.CanaryErrorCount += report.CanaryErrorCount
			for _, warning := range report.Warnings {
				switch {
				case strings.HasPrefix(strings.TrimSpace(warning), "visual:"):
					m.VisualWarningCount++
				case strings.HasPrefix(strings.TrimSpace(warning), "interaction:"):
					m.InteractionWarningCount++
				}
			}
			if report.VisionReviewed {
				m.VisionReviewed = true
			}
		}
	}
	if !previewGateFound {
		m.PreviewGateSkipped = true
	}

	return m
}

func buildCanaryCohort(build *Build) string {
	if build == nil {
		return ""
	}

	plan := buildSubscriptionPlan(build)
	deliveryMode := buildCurrentDeliveryMode(build)
	switch {
	case plan == "free" && build.Mode == ModeFast && deliveryMode == "frontend_preview_only":
		return "free_fast_frontend_preview"
	case isPaidBuildPlan(plan) && build.PowerMode == PowerBalanced && deliveryMode != "frontend_preview_only":
		return "paid_balanced_full_stack"
	case isPaidBuildPlan(plan) && build.PowerMode == PowerMax && deliveryMode != "frontend_preview_only":
		return "paid_max_full_stack"
	default:
		return ""
	}
}

func verificationStatusByPhase(reports []VerificationReport) map[string]string {
	if len(reports) == 0 {
		return nil
	}
	out := make(map[string]string, len(reports))
	for _, report := range reports {
		key := verificationPhaseKey(report)
		if key == "" {
			continue
		}
		out[key] = string(report.Status)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func verificationPhaseKey(report VerificationReport) string {
	phase := strings.TrimSpace(report.Phase)
	if phase == "" {
		return ""
	}
	surface := strings.TrimSpace(string(report.Surface))
	if surface == "" || surface == string(SurfaceGlobal) {
		return phase
	}
	return phase + ":" + surface
}
