package agents

import (
	"testing"

	"apex-build/internal/ai"
)

func TestAssignProvidersToRoles_UsesPreferredProvidersWhenAvailable(t *testing.T) {
	am := &AgentManager{}
	providers := []ai.AIProvider{
		ai.ProviderClaude,
		ai.ProviderGPT4,
		ai.ProviderGemini,
	}
	roles := []AgentRole{
		RoleArchitect,
		RolePlanner,
		RoleReviewer,
		RoleFrontend,
		RoleBackend,
		RoleDatabase,
		RoleTesting,
	}

	assignments := am.assignProvidersToRoles(providers, roles)

	if got := assignments[RoleArchitect]; got != ai.ProviderClaude {
		t.Fatalf("architect provider = %s, want %s", got, ai.ProviderClaude)
	}
	if got := assignments[RolePlanner]; got != ai.ProviderClaude {
		t.Fatalf("planner provider = %s, want %s", got, ai.ProviderClaude)
	}
	if got := assignments[RoleReviewer]; got != ai.ProviderClaude {
		t.Fatalf("reviewer provider = %s, want %s", got, ai.ProviderClaude)
	}
	if got := assignments[RoleFrontend]; got != ai.ProviderGPT4 {
		t.Fatalf("frontend provider = %s, want %s", got, ai.ProviderGPT4)
	}
	if got := assignments[RoleBackend]; got != ai.ProviderGPT4 {
		t.Fatalf("backend provider = %s, want %s", got, ai.ProviderGPT4)
	}
	if got := assignments[RoleDatabase]; got != ai.ProviderGPT4 {
		t.Fatalf("database provider = %s, want %s", got, ai.ProviderGPT4)
	}
	if got := assignments[RoleTesting]; got != ai.ProviderGemini {
		t.Fatalf("testing provider = %s, want %s", got, ai.ProviderGemini)
	}
}

func TestAssignProvidersToRoles_FallsBackWhenPreferredProvidersUnavailable(t *testing.T) {
	am := &AgentManager{}
	providers := []ai.AIProvider{
		ai.ProviderClaude,
		ai.ProviderGemini,
	}
	roles := []AgentRole{
		RoleArchitect,
		RoleFrontend,
		RoleTesting,
	}

	assignments := am.assignProvidersToRoles(providers, roles)

	if got := assignments[RoleArchitect]; got != ai.ProviderClaude {
		t.Fatalf("architect provider = %s, want %s", got, ai.ProviderClaude)
	}
	// GPT is unavailable, so coding should gracefully fall back to Claude.
	if got := assignments[RoleFrontend]; got != ai.ProviderClaude {
		t.Fatalf("frontend provider = %s, want %s", got, ai.ProviderClaude)
	}
	if got := assignments[RoleTesting]; got != ai.ProviderGemini {
		t.Fatalf("testing provider = %s, want %s", got, ai.ProviderGemini)
	}
}

func TestAssignProvidersToRolesForBuild_UsesLiveScorecards(t *testing.T) {
	am := &AgentManager{}
	build := &Build{
		ID:           "build-live-scorecards",
		ProviderMode: "platform",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
				ProviderScorecards: []ProviderScorecard{
					{Provider: ai.ProviderGPT4, TaskShape: TaskShapeFrontendPatch, CompilePassRate: 0.40, FirstPassVerificationRate: 0.35, RepairSuccessRate: 0.50, PromotionRate: 0.42, FailureClassRecurrence: 0.40, TruncationRate: 0.12, AverageCostPerSuccess: 0.15},
					{Provider: ai.ProviderGemini, TaskShape: TaskShapeFrontendPatch, CompilePassRate: 0.98, FirstPassVerificationRate: 0.97, RepairSuccessRate: 0.92, PromotionRate: 0.95, FailureClassRecurrence: 0.05, TruncationRate: 0.01, AverageCostPerSuccess: 0.05},
				},
			},
		},
	}

	assignments := am.assignProvidersToRolesForBuild(build, []ai.AIProvider{
		ai.ProviderClaude,
		ai.ProviderGPT4,
		ai.ProviderGemini,
	}, []AgentRole{
		RoleFrontend,
		RoleTesting,
	})

	if got := assignments[RoleFrontend]; got != ai.ProviderGemini {
		t.Fatalf("frontend provider = %s, want live-scorecard provider %s", got, ai.ProviderGemini)
	}
	if got := assignments[RoleTesting]; got != ai.ProviderGemini {
		t.Fatalf("testing provider = %s, want %s", got, ai.ProviderGemini)
	}
}

func TestGetNextFallbackProviderForTask_UsesLiveScorecards(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderGPT4, ai.ProviderClaude, ai.ProviderGemini},
			hasConfiguredProvider: true,
		},
	}
	build := &Build{
		ID:           "build-fallback-scorecards",
		ProviderMode: "platform",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
				ProviderScorecards: []ProviderScorecard{
					{Provider: ai.ProviderClaude, TaskShape: TaskShapeFrontendPatch, CompilePassRate: 0.70, FirstPassVerificationRate: 0.68, RepairSuccessRate: 0.72, PromotionRate: 0.70, FailureClassRecurrence: 0.20, TruncationRate: 0.04, AverageCostPerSuccess: 0.10},
					{Provider: ai.ProviderGemini, TaskShape: TaskShapeFrontendPatch, CompilePassRate: 0.96, FirstPassVerificationRate: 0.95, RepairSuccessRate: 0.90, PromotionRate: 0.94, FailureClassRecurrence: 0.06, TruncationRate: 0.01, AverageCostPerSuccess: 0.05},
				},
			},
		},
	}
	task := &Task{
		ID:   "task-fallback",
		Type: TaskGenerateUI,
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				ID:        "wo-front",
				Role:      RoleFrontend,
				TaskShape: TaskShapeFrontendPatch,
			},
		},
	}

	got := am.getNextFallbackProviderForTask(build, task, RoleFrontend, ai.ProviderGPT4)
	if got != ai.ProviderGemini {
		t.Fatalf("fallback provider = %s, want live-scorecard provider %s", got, ai.ProviderGemini)
	}
}
