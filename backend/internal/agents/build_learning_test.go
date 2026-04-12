package agents

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"apex-build/internal/ai"
	"apex-build/pkg/models"
)

func mustMarshalBuildState(t *testing.T, state BuildSnapshotState) string {
	t.Helper()
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal build state: %v", err)
	}
	return string(data)
}

func TestCreateBuildLoadsHistoricalLearningFromCompletedBuilds(t *testing.T) {
	db := openBuildTestDB(t)
	techStack := &TechStack{
		Frontend: "React",
		Backend:  "Express",
		Database: "PostgreSQL",
		Styling:  "Tailwind",
	}
	stateJSON := mustMarshalBuildState(t, BuildSnapshotState{
		Orchestration: &BuildOrchestrationState{
			FailureFingerprints: []FailureFingerprint{
				{
					ID:               "fp-1",
					BuildID:          "prev-build-1",
					TaskShape:        TaskShapeRepair,
					FailureClass:     "preview_verification",
					FilesInvolved:    []string{"src/App.tsx", "src/components/Hero.tsx"},
					RepairPathChosen: []string{"semantic_diff", "targeted_retry"},
					RepairStrategy:   "semantic_diff",
					PatchClass:       "import_export_mismatch",
					RepairSucceeded:  true,
					CreatedAt:        time.Now().UTC(),
				},
				{
					ID:            "fp-2",
					BuildID:       "prev-build-1",
					TaskShape:     TaskShapeVerification,
					FailureClass:  "interaction_canary",
					FilesInvolved: []string{"src/App.tsx"},
					CreatedAt:     time.Now().UTC(),
				},
			},
			VerificationReports: []VerificationReport{
				{
					ID:          "report-1",
					BuildID:     "prev-build-1",
					Phase:       "preview_verification",
					Surface:     SurfaceFrontend,
					Status:      VerificationPassed,
					Warnings:    []string{"interaction: CTA button stopped responding after nav click"},
					GeneratedAt: time.Now().UTC(),
				},
				{
					ID:          "report-2",
					BuildID:     "prev-build-1",
					Phase:       "runtime_verification",
					Surface:     SurfaceIntegration,
					Status:      VerificationPassed,
					GeneratedAt: time.Now().UTC(),
				},
			},
			ProviderScorecards: []ProviderScorecard{
				{
					Provider:                  ai.ProviderGPT4,
					TaskShape:                 TaskShapeFrontendPatch,
					CompilePassRate:           1,
					FirstPassVerificationRate: 1,
					RepairSuccessRate:         1,
					TruncationRate:            0,
					PromotionRate:             1,
					HostedEligible:            true,
					SampleCount:               4,
					SuccessCount:              4,
					FirstPassSampleCount:      4,
					FirstPassSuccessCount:     4,
					RepairAttemptCount:        2,
					RepairSuccessCount:        2,
					PromotionAttemptCount:     3,
					PromotionSuccessCount:     3,
					TokenSampleCount:          2,
					AverageAcceptedTokens:     5400,
					CostSampleCount:           2,
					AverageCostPerSuccess:     0.07,
					LatencySampleCount:        2,
					AverageLatencySeconds:     4.2,
				},
			},
		},
	})
	techJSON, err := json.Marshal(techStack)
	if err != nil {
		t.Fatalf("marshal tech stack: %v", err)
	}
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "prev-build-1",
		UserID:      1,
		ProjectName: "Acme CRM",
		Status:      string(BuildCompleted),
		TechStack:   string(techJSON),
		StateJSON:   stateJSON,
		CreatedAt:   time.Now().UTC().Add(-2 * time.Hour),
		UpdatedAt:   time.Now().UTC().Add(-1 * time.Hour),
	}).Error; err != nil {
		t.Fatalf("create completed build: %v", err)
	}

	am := NewAgentManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
	}, db)

	build, err := am.CreateBuild(1, "pro", &BuildRequest{
		Description: "Build a full-stack CRM with sales dashboards",
		ProjectName: "Acme CRM",
		TechStack:   techStack,
	})
	if err != nil {
		t.Fatalf("CreateBuild returned error: %v", err)
	}

	learning := build.SnapshotState.Orchestration.HistoricalLearning
	if learning == nil {
		t.Fatal("expected historical learning to be attached to the build")
	}
	if learning.Scope != "same_project_name" {
		t.Fatalf("expected same_project_name scope, got %q", learning.Scope)
	}
	if learning.ObservedBuilds != 1 {
		t.Fatalf("expected 1 observed build, got %d", learning.ObservedBuilds)
	}
	if !containsString(learning.RecurringFailureClasses, "preview_verification") {
		t.Fatalf("expected preview_verification in recurring failures, got %+v", learning.RecurringFailureClasses)
	}
	if !containsString(learning.SuccessfulRepairPaths, "semantic_diff -> targeted_retry") {
		t.Fatalf("expected successful repair path to be captured, got %+v", learning.SuccessfulRepairPaths)
	}
	if !containsString(learning.RepairStrategyWinRates, "semantic_diff/import_export_mismatch: 1/1 success") {
		t.Fatalf("expected repair strategy win rate to be captured, got %+v", learning.RepairStrategyWinRates)
	}
	if len(learning.SemanticRepairHints) == 0 || !strings.Contains(learning.SemanticRepairHints[0], "patch=import_export_mismatch") {
		t.Fatalf("expected semantic repair hint to be captured, got %+v", learning.SemanticRepairHints)
	}
	if !containsString(learning.HotspotFiles, "src/App.tsx") {
		t.Fatalf("expected hotspot file to be captured, got %+v", learning.HotspotFiles)
	}
	if !containsString(learning.FrequentWarnings, "interaction: CTA button stopped responding after nav click") {
		t.Fatalf("expected warning to be captured, got %+v", learning.FrequentWarnings)
	}
	if !containsString(learning.CleanPassSignals, "runtime_verification/integration clean") {
		t.Fatalf("expected clean pass signal to be captured, got %+v", learning.CleanPassSignals)
	}
	var imported *ProviderScorecard
	for i := range build.SnapshotState.Orchestration.ProviderScorecards {
		scorecard := &build.SnapshotState.Orchestration.ProviderScorecards[i]
		if scorecard.Provider == ai.ProviderGPT4 && scorecard.TaskShape == TaskShapeFrontendPatch {
			imported = scorecard
			break
		}
	}
	if imported == nil {
		t.Fatal("expected historical GPT4 frontend scorecard to be imported")
	}
	if imported.SampleCount < 4 || imported.SuccessCount < 4 {
		t.Fatalf("expected historical observed scorecard counts, got %+v", imported)
	}
	if !hasSufficientLiveScorecards(build.SnapshotState.Orchestration.ProviderScorecards) {
		t.Fatalf("expected historical scorecards to activate live scorecard routing")
	}
}

func TestAppendFailureFingerprintRefreshesReliabilitySummary(t *testing.T) {
	build := &Build{
		ID:          "refresh-learning-build",
		Description: "Test reliability refresh",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{},
		},
	}

	appendFailureFingerprint(build, FailureFingerprint{
		ID:           "fp-a",
		BuildID:      build.ID,
		TaskShape:    TaskShapeRepair,
		FailureClass: "preview_verification",
		CreatedAt:    time.Now().UTC(),
	})
	appendFailureFingerprint(build, FailureFingerprint{
		ID:           "fp-b",
		BuildID:      build.ID,
		TaskShape:    TaskShapeRepair,
		FailureClass: "preview_verification",
		CreatedAt:    time.Now().UTC(),
	})

	summary := build.SnapshotState.Orchestration.ReliabilitySummary
	if summary == nil {
		t.Fatal("expected reliability summary to be recomputed")
	}
	if !containsString(summary.RecurringFailureClass, "preview_verification") {
		t.Fatalf("expected recurring failure class to be updated, got %+v", summary.RecurringFailureClass)
	}
}

func TestBuildTaskPromptIncludesHistoricalLearningContext(t *testing.T) {
	am := &AgentManager{}
	build := &Build{
		ID:          "prompt-learning-build",
		Description: "Repair the generated CRM dashboard",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				HistoricalLearning: &BuildLearningSummary{
					Scope:                   "same_stack",
					ObservedBuilds:          2,
					RecurringFailureClasses: []string{"preview_verification"},
					RepairStrategyWinRates:  []string{"semantic_diff/import_export_mismatch: 1/1 success"},
					SemanticRepairHints:     []string{"failure=compile_failure patch=import_export_mismatch strategy=semantic_diff files=src/App.tsx"},
					RecommendedAvoidance:    []string{"Keep the preview entrypoint, ports, and boot path deterministic before adding surface polish."},
				},
			},
		},
	}
	task := &Task{
		ID:          "repair-task",
		Type:        TaskFix,
		Description: "Fix the preview verification issues",
		Input:       map[string]any{},
	}
	agent := &Agent{
		ID:       "solver-1",
		Role:     RoleSolver,
		Provider: ai.ProviderClaude,
	}

	prompt := am.buildTaskPrompt(task, build, agent)
	if !strings.Contains(prompt, "<historical_build_learning>") {
		t.Fatalf("expected historical build learning block in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "preview_verification") {
		t.Fatalf("expected recurring failure class in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "repair_strategy_win_rates") || !strings.Contains(prompt, "semantic_diff/import_export_mismatch") {
		t.Fatalf("expected repair strategy win rates in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "semantic_repair_hints") || !strings.Contains(prompt, "patch=import_export_mismatch") {
		t.Fatalf("expected semantic repair hints in prompt, got %q", prompt)
	}
}
