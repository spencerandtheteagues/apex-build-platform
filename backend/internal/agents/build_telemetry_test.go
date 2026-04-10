package agents

import (
	"testing"
	"time"
)

func TestBuildCanaryCohort(t *testing.T) {
	tests := []struct {
		name  string
		build *Build
		want  string
	}{
		{
			name: "free fast frontend preview",
			build: &Build{
				Mode:             ModeFast,
				PowerMode:        PowerFast,
				SubscriptionPlan: "free",
				Plan:             &BuildPlan{DeliveryMode: "frontend_preview_only"},
			},
			want: "free_fast_frontend_preview",
		},
		{
			name: "paid balanced full stack",
			build: &Build{
				Mode:             ModeFull,
				PowerMode:        PowerBalanced,
				SubscriptionPlan: "builder",
				Plan:             &BuildPlan{DeliveryMode: "full_stack"},
			},
			want: "paid_balanced_full_stack",
		},
		{
			name: "paid max full stack",
			build: &Build{
				Mode:             ModeFull,
				PowerMode:        PowerMax,
				SubscriptionPlan: "builder",
				Plan:             &BuildPlan{DeliveryMode: "full_stack"},
			},
			want: "paid_max_full_stack",
		},
		{
			name: "unclassified shape",
			build: &Build{
				Mode:             ModeFast,
				PowerMode:        PowerBalanced,
				SubscriptionPlan: "builder",
				Plan:             &BuildPlan{DeliveryMode: "frontend_preview_only"},
			},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildCanaryCohort(tc.build); got != tc.want {
				t.Fatalf("buildCanaryCohort() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDeriveBuildQualityMetricsIncludesReliabilitySummaryFields(t *testing.T) {
	now := time.Now().UTC()
	build := &Build{
		ID:               "build-telemetry",
		UserID:           42,
		Status:           BuildCompleted,
		Mode:             ModeFast,
		PowerMode:        PowerFast,
		SubscriptionPlan: "free",
		Plan:             &BuildPlan{DeliveryMode: "frontend_preview_only"},
		CreatedAt:        now.Add(-2 * time.Minute),
		CompletedAt:      telemetryPtrTime(now.Add(-5 * time.Second)),
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				VerificationReports: []VerificationReport{
					{
						ID:               "preview-pass",
						Phase:            "preview_verification",
						Surface:          SurfaceGlobal,
						Status:           VerificationPassed,
						Warnings:         []string{"visual: improve hero contrast", "interaction: first click opens stale state"},
						CanaryClickCount: 3,
						CanaryErrorCount: 1,
						VisionReviewed:   true,
					},
					{
						ID:      "frontend-local",
						Phase:   "surface_local_verification",
						Surface: SurfaceFrontend,
						Status:  VerificationFailed,
						Errors:  []string{"missing frontend entry"},
					},
				},
				FailureFingerprints: []FailureFingerprint{
					{ID: "fp-1", FailureClass: "interaction_canary", TaskShape: TaskShapeVerification, CreatedAt: now.Add(-time.Minute)},
					{ID: "fp-2", FailureClass: "interaction_canary", TaskShape: TaskShapeVerification, CreatedAt: now},
				},
			},
		},
	}

	metrics := deriveBuildQualityMetrics(build, []GeneratedFile{
		{Path: "src/App.tsx"},
		{Path: "src/App.test.tsx"},
	}, now)
	if metrics == nil {
		t.Fatal("expected metrics")
	}
	if metrics.DeliveryMode != "frontend_preview_only" {
		t.Fatalf("expected delivery mode frontend_preview_only, got %q", metrics.DeliveryMode)
	}
	if metrics.CanaryCohort != "free_fast_frontend_preview" {
		t.Fatalf("expected free canary cohort, got %q", metrics.CanaryCohort)
	}
	if metrics.PreviewGatePassed == nil || !*metrics.PreviewGatePassed {
		t.Fatalf("expected preview gate to pass, got %+v", metrics.PreviewGatePassed)
	}
	if metrics.CanaryClicked != 3 || metrics.CanaryErrorCount != 1 {
		t.Fatalf("unexpected canary counts: %+v", metrics)
	}
	if !metrics.VisionReviewed {
		t.Fatalf("expected vision reviewed")
	}
	if metrics.VisualWarningCount != 1 || metrics.InteractionWarningCount != 1 {
		t.Fatalf("unexpected warning counts: %+v", metrics)
	}
	if got := metrics.VerificationStatusByPhase["preview_verification"]; got != string(VerificationPassed) {
		t.Fatalf("expected preview_verification=passed, got %q", got)
	}
	if got := metrics.VerificationStatusByPhase["surface_local_verification:frontend"]; got != string(VerificationFailed) {
		t.Fatalf("expected frontend local verification=failed, got %q", got)
	}
	if len(metrics.AdvisoryClasses) != 2 || !containsString(metrics.AdvisoryClasses, "visual_layout") || !containsString(metrics.AdvisoryClasses, "interaction_canary") {
		t.Fatalf("unexpected advisory classes: %+v", metrics.AdvisoryClasses)
	}
	if len(metrics.RecurringFailureClasses) != 1 || metrics.RecurringFailureClasses[0] != "interaction_canary" {
		t.Fatalf("unexpected recurring classes: %+v", metrics.RecurringFailureClasses)
	}
	if metrics.TestFileCount != 1 {
		t.Fatalf("expected one test file, got %d", metrics.TestFileCount)
	}
}

func TestVerificationStatusByPhaseUsesSurfaceForNonGlobalReports(t *testing.T) {
	got := verificationStatusByPhase([]VerificationReport{
		{Phase: "preview_verification", Surface: SurfaceGlobal, Status: VerificationPassed},
		{Phase: "surface_local_verification", Surface: SurfaceFrontend, Status: VerificationFailed},
		{Phase: "surface_local_verification", Surface: SurfaceBackend, Status: VerificationBlocked},
	})

	if len(got) != 3 {
		t.Fatalf("expected 3 verification status keys, got %+v", got)
	}
	if got["preview_verification"] != string(VerificationPassed) {
		t.Fatalf("expected preview phase status, got %+v", got)
	}
	if got["surface_local_verification:frontend"] != string(VerificationFailed) {
		t.Fatalf("expected frontend phase status, got %+v", got)
	}
	if got["surface_local_verification:backend"] != string(VerificationBlocked) {
		t.Fatalf("expected backend phase status, got %+v", got)
	}
}

func telemetryPtrTime(value time.Time) *time.Time {
	return &value
}
