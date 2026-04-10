package agents

import (
	"testing"
	"time"
)

func TestRefreshDerivedReliabilitySummaryLockedCapturesPassedPreviewAdvisories(t *testing.T) {
	build := &Build{
		ID:               "reliability-advisory-build",
		Status:           BuildCompleted,
		SubscriptionPlan: "builder",
		ProviderMode:     "platform",
		Description:      "Build a polished dashboard preview",
		UpdatedAt:        time.Now().UTC(),
	}
	orchestration := ensureBuildOrchestrationStateLocked(build)
	orchestration.ValidatedBuildSpec = &ValidatedBuildSpec{
		AcceptanceSurfaces: []string{"frontend"},
		PrimaryUserFlows:   []string{"land in the dashboard and review metrics"},
	}
	orchestration.VerificationReports = []VerificationReport{
		{
			ID:          "preview-pass",
			BuildID:     build.ID,
			Phase:       "preview_verification",
			Surface:     SurfaceFrontend,
			Status:      VerificationPassed,
			Warnings:    []string{"visual: improve hero contrast", "interaction: first click opens stale empty state"},
			GeneratedAt: time.Now().UTC(),
		},
	}
	orchestration.FailureFingerprints = []FailureFingerprint{
		{BuildID: build.ID, TaskShape: TaskShapeVerification, FailureClass: "visual_layout", RepairSucceeded: true, CreatedAt: time.Now().Add(-time.Minute).UTC()},
		{BuildID: build.ID, TaskShape: TaskShapeVerification, FailureClass: "visual_layout", RepairSucceeded: true, CreatedAt: time.Now().UTC()},
	}

	refreshDerivedSnapshotStateLocked(build, &build.SnapshotState)

	summary := build.SnapshotState.Orchestration.ReliabilitySummary
	if summary == nil {
		t.Fatal("expected reliability summary")
	}
	if summary.Status != "advisory" {
		t.Fatalf("expected advisory status, got %+v", summary)
	}
	if len(summary.AdvisoryClasses) != 2 {
		t.Fatalf("expected visual and interaction advisory classes, got %+v", summary.AdvisoryClasses)
	}
	if len(summary.AcceptanceSurfaces) != 1 || summary.AcceptanceSurfaces[0] != "frontend" {
		t.Fatalf("expected acceptance surfaces from validated spec, got %+v", summary.AcceptanceSurfaces)
	}
	if len(summary.PrimaryUserFlows) != 1 {
		t.Fatalf("expected user flows from validated spec, got %+v", summary.PrimaryUserFlows)
	}
}

func TestDeriveBuildReliabilitySummaryCapturesActiveRepairPath(t *testing.T) {
	build := &Build{ID: "repair-summary-build"}
	orchestration := &BuildOrchestrationState{
		FailureFingerprints: []FailureFingerprint{
			{
				BuildID:          "repair-summary-build",
				RepairPathChosen: []string{"old_strategy"},
				CreatedAt:        time.Now().Add(-time.Hour).UTC(),
			},
			{
				BuildID:          "repair-summary-build",
				RepairPathChosen: []string{"solve_build_failure", "switch_provider"},
				CreatedAt:        time.Now().UTC(),
			},
		},
	}
	state := &BuildSnapshotState{}

	summary := deriveBuildReliabilitySummary(build, state, orchestration)
	if summary == nil {
		t.Fatal("expected reliability summary")
	}
	if len(summary.ActiveRepairPath) != 2 || summary.ActiveRepairPath[0] != "solve_build_failure" || summary.ActiveRepairPath[1] != "switch_provider" {
		t.Fatalf("expected latest repair path to be captured, got %v", summary.ActiveRepairPath)
	}
}

func TestDeriveBuildReliabilitySummarySkipsPromotionFallbackForActiveRepairPath(t *testing.T) {
	build := &Build{ID: "repair-summary-promotion-build"}
	orchestration := &BuildOrchestrationState{
		FailureFingerprints: []FailureFingerprint{
			{
				BuildID:          build.ID,
				TaskShape:        TaskShapeRepair,
				RepairPathChosen: []string{"solve_build_failure"},
				CreatedAt:        time.Now().Add(-time.Minute).UTC(),
			},
			{
				BuildID:          build.ID,
				TaskShape:        TaskShapePromotion,
				RepairPathChosen: []string{"final_readiness"},
				CreatedAt:        time.Now().UTC(),
			},
		},
	}

	summary := deriveBuildReliabilitySummary(build, &BuildSnapshotState{}, orchestration)
	if summary == nil {
		t.Fatal("expected reliability summary")
	}
	if len(summary.ActiveRepairPath) != 1 || summary.ActiveRepairPath[0] != "solve_build_failure" {
		t.Fatalf("expected repair path to ignore promotion fallback, got %v", summary.ActiveRepairPath)
	}
}
