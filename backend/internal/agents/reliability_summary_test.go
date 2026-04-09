package agents

import (
	"strings"
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
	if len(summary.RecurringFailureClass) == 0 || summary.RecurringFailureClass[0] != "visual_layout" {
		t.Fatalf("expected recurring visual class, got %+v", summary.RecurringFailureClass)
	}
}

func TestRefreshDerivedReliabilitySummaryLockedMarksDegradedCurrentFailure(t *testing.T) {
	build := &Build{
		ID:               "reliability-degraded-build",
		Status:           BuildReviewing,
		SubscriptionPlan: "builder",
		ProviderMode:     "platform",
		Description:      "Build a full stack workspace",
		UpdatedAt:        time.Now().UTC(),
		SnapshotState: BuildSnapshotState{
			FailureTaxonomy: &BuildFailureTaxonomy{
				CurrentCategory: FailureCategoryCompile,
				CurrentClass:    "compile_failure",
				CurrentPhase:    "compile_validation",
			},
		},
	}
	orchestration := ensureBuildOrchestrationStateLocked(build)
	orchestration.VerificationReports = []VerificationReport{
		{
			ID:          "compile-fail",
			BuildID:     build.ID,
			Phase:       "compile_validation",
			Surface:     SurfaceFrontend,
			Status:      VerificationFailed,
			Errors:      []string{"tsc failed"},
			GeneratedAt: time.Now().UTC(),
		},
	}

	refreshDerivedSnapshotStateLocked(build, &build.SnapshotState)

	summary := build.SnapshotState.Orchestration.ReliabilitySummary
	if summary == nil {
		t.Fatal("expected reliability summary")
	}
	if summary.Status != "degraded" {
		t.Fatalf("expected degraded status, got %+v", summary)
	}
	if summary.CurrentFailureCategory != FailureCategoryCompile || summary.CurrentFailureClass != "compile_failure" {
		t.Fatalf("expected compile failure details, got %+v", summary)
	}
	if len(summary.RecommendedFocus) == 0 {
		t.Fatalf("expected recommended focus, got %+v", summary)
	}
}

func TestBuildTaskPromptIncludesReliabilitySummaryContext(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		Description: "Build a preview-first workspace",
		Plan: &BuildPlan{
			SpecHash: "spec-reliability",
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				ReliabilitySummary: &BuildReliabilitySummary{
					Status:                "degraded",
					CurrentFailureClass:   "compile_failure",
					AcceptanceSurfaces:    []string{"frontend"},
					PrimaryUserFlows:      []string{"land in the product shell and reach an interactive preview on first pass"},
					RecurringFailureClass: []string{"compile_failure"},
					RecommendedFocus:      []string{"expand deterministic compile repair coverage for the current failure class"},
				},
			},
		},
	}
	task := &Task{Type: TaskFix, Description: "Repair the current preview blocker"}
	agent := &Agent{Role: RoleSolver}

	prompt := am.buildTaskPrompt(task, build, agent)
	if !strings.Contains(prompt, "<reliability_summary>") {
		t.Fatalf("expected reliability summary context in task prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "compile_failure") {
		t.Fatalf("expected failure class in task prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "Preserve the acceptance surfaces") {
		t.Fatalf("expected acceptance-surface preservation guidance in task prompt, got %q", prompt)
	}
}
