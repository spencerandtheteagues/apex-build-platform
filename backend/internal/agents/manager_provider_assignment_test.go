package agents

import (
	"errors"
	"testing"
	"time"

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
					{Provider: ai.ProviderGPT4, TaskShape: TaskShapeFrontendPatch, SampleCount: 4, FirstPassSampleCount: 4, CompilePassRate: 0.40, FirstPassVerificationRate: 0.35, RepairSuccessRate: 0.50, PromotionRate: 0.42, FailureClassRecurrence: 0.40, TruncationRate: 0.12, AverageCostPerSuccess: 0.15},
					{Provider: ai.ProviderGemini, TaskShape: TaskShapeFrontendPatch, SampleCount: 4, FirstPassSampleCount: 4, CompilePassRate: 0.98, FirstPassVerificationRate: 0.97, RepairSuccessRate: 0.92, PromotionRate: 0.95, FailureClassRecurrence: 0.05, TruncationRate: 0.01, AverageCostPerSuccess: 0.05},
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

func TestAssignProvidersToRolesForBuild_IgnoresLowSampleScorecards(t *testing.T) {
	am := &AgentManager{}
	build := &Build{
		ID:           "build-low-sample-scorecards",
		ProviderMode: "platform",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
				ProviderScorecards: []ProviderScorecard{
					{Provider: ai.ProviderGemini, TaskShape: TaskShapeFrontendPatch, SampleCount: 2, FirstPassSampleCount: 2, CompilePassRate: 0.99, FirstPassVerificationRate: 0.98, RepairSuccessRate: 0.95, PromotionRate: 0.94, FailureClassRecurrence: 0.01, TruncationRate: 0.01, AverageCostPerSuccess: 0.02},
				},
			},
		},
	}

	assignments := am.assignProvidersToRolesForBuild(build, []ai.AIProvider{
		ai.ProviderClaude,
		ai.ProviderGPT4,
		ai.ProviderGemini,
	}, []AgentRole{RoleFrontend, RoleTesting})

	if got := assignments[RoleFrontend]; got != ai.ProviderGPT4 {
		t.Fatalf("frontend provider = %s, want baseline policy provider %s when samples are insufficient", got, ai.ProviderGPT4)
	}
	if got := assignments[RoleTesting]; got != ai.ProviderGemini {
		t.Fatalf("testing provider = %s, want baseline policy provider %s", got, ai.ProviderGemini)
	}
}

func TestAssignProvidersToRolesForBuild_UsesReliabilityBiasWhenScorecardsAreInsufficient(t *testing.T) {
	am := &AgentManager{}
	build := &Build{
		ID:           "build-reliability-bias",
		ProviderMode: "platform",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
				ReliabilitySummary: &BuildReliabilitySummary{
					Status:                "advisory",
					AdvisoryClasses:       []string{"visual_layout"},
					RecurringFailureClass: []string{"visual_layout"},
				},
				ProviderScorecards: []ProviderScorecard{
					{Provider: ai.ProviderGemini, TaskShape: TaskShapeFrontendPatch, SampleCount: 1, FirstPassSampleCount: 1, CompilePassRate: 0.99, FirstPassVerificationRate: 0.98},
				},
			},
		},
	}

	assignments := am.assignProvidersToRolesForBuild(build, []ai.AIProvider{
		ai.ProviderClaude,
		ai.ProviderGPT4,
		ai.ProviderGemini,
	}, []AgentRole{RoleFrontend, RoleReviewer})

	reliabilityPreferred := reliabilityPreferredProviders(build, RoleFrontend)
	if len(reliabilityPreferred) == 0 || reliabilityPreferred[0] != ai.ProviderClaude {
		t.Fatalf("frontend reliabilityPreferredProviders = %+v, want leading %s", reliabilityPreferred, ai.ProviderClaude)
	}

	if got := assignments[RoleFrontend]; got != ai.ProviderClaude {
		t.Fatalf("frontend provider = %s, want reliability-bias provider %s", got, ai.ProviderClaude)
	}
	if got := assignments[RoleReviewer]; got != ai.ProviderClaude {
		t.Fatalf("reviewer provider = %s, want reliability-bias provider %s", got, ai.ProviderClaude)
	}
}

func TestAssignProvidersToRolesForBuild_CompileReliabilityBiasesTowardGPT(t *testing.T) {
	am := &AgentManager{}
	build := &Build{
		ID:           "build-compile-reliability-bias",
		ProviderMode: "platform",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
				ReliabilitySummary: &BuildReliabilitySummary{
					Status:                "degraded",
					CurrentFailureClass:   "compile_failure",
					RecurringFailureClass: []string{"compile_failure"},
				},
			},
		},
	}

	assignments := am.assignProvidersToRolesForBuild(build, []ai.AIProvider{
		ai.ProviderClaude,
		ai.ProviderGPT4,
		ai.ProviderGemini,
	}, []AgentRole{RoleFrontend, RoleBackend, RoleSolver})

	if got := assignments[RoleFrontend]; got != ai.ProviderGPT4 {
		t.Fatalf("frontend provider = %s, want compile-bias provider %s", got, ai.ProviderGPT4)
	}
	if got := assignments[RoleBackend]; got != ai.ProviderGPT4 {
		t.Fatalf("backend provider = %s, want compile-bias provider %s", got, ai.ProviderGPT4)
	}
	if got := assignments[RoleSolver]; got != ai.ProviderGPT4 {
		t.Fatalf("solver provider = %s, want compile-bias provider %s", got, ai.ProviderGPT4)
	}
}

func TestAssignProvidersToRolesForBuild_UsesValidatedSpecPerformanceBiasWhenReliabilityAbsent(t *testing.T) {
	am := &AgentManager{}
	build := &Build{
		ID:           "build-validated-spec-performance-bias",
		ProviderMode: "platform",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
				ValidatedBuildSpec: &ValidatedBuildSpec{
					PerformanceAdvisories: []BuildSpecAdvisory{
						{
							Code:    "progressive_dashboard_loading",
							Surface: SurfaceFrontend,
							Summary: "Dashboard-style apps should reveal value before every widget finishes loading.",
						},
					},
				},
			},
		},
	}

	assignments := am.assignProvidersToRolesForBuild(build, []ai.AIProvider{
		ai.ProviderClaude,
		ai.ProviderGPT4,
		ai.ProviderGemini,
	}, []AgentRole{RoleFrontend, RoleTesting})

	specPreferred := validatedSpecPreferredProviders(build, RoleFrontend)
	if len(specPreferred) == 0 || specPreferred[0] != ai.ProviderClaude {
		t.Fatalf("frontend validatedSpecPreferredProviders = %+v, want leading %s", specPreferred, ai.ProviderClaude)
	}
	if got := assignments[RoleFrontend]; got != ai.ProviderClaude {
		t.Fatalf("frontend provider = %s, want validated-spec performance provider %s", got, ai.ProviderClaude)
	}
	if got := assignments[RoleTesting]; got != ai.ProviderClaude {
		t.Fatalf("testing provider = %s, want validated-spec performance provider %s", got, ai.ProviderClaude)
	}
}

func TestAssignProvidersToRolesForBuild_IgnoresIrrelevantValidatedSpecBiasForFrontend(t *testing.T) {
	am := &AgentManager{}
	build := &Build{
		ID:           "build-validated-spec-irrelvant-bias",
		ProviderMode: "platform",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
				ValidatedBuildSpec: &ValidatedBuildSpec{
					SecurityAdvisories: []BuildSpecAdvisory{
						{
							Code:    "tenant_isolation",
							Surface: SurfaceBackend,
							Summary: "Multi-tenant data models need explicit tenant isolation at query and mutation boundaries.",
						},
					},
				},
			},
		},
	}

	assignments := am.assignProvidersToRolesForBuild(build, []ai.AIProvider{
		ai.ProviderClaude,
		ai.ProviderGPT4,
		ai.ProviderGemini,
	}, []AgentRole{RoleFrontend, RoleTesting})

	if got := assignments[RoleFrontend]; got != ai.ProviderGPT4 {
		t.Fatalf("frontend provider = %s, want baseline provider %s when validated-spec bias is irrelevant", got, ai.ProviderGPT4)
	}
	if got := assignments[RoleTesting]; got != ai.ProviderGemini {
		t.Fatalf("testing provider = %s, want baseline provider %s when validated-spec bias is irrelevant", got, ai.ProviderGemini)
	}
}

func TestSelectProviderByScorecardUsesPowerModeCostSensitivity(t *testing.T) {
	scorecards := []ProviderScorecard{
		{Provider: ai.ProviderClaude, TaskShape: TaskShapeFrontendPatch, SampleCount: 5, FirstPassSampleCount: 5, CompilePassRate: 0.96, FirstPassVerificationRate: 0.95, RepairSuccessRate: 0.92, PromotionRate: 0.94, FailureClassRecurrence: 0.03, TruncationRate: 0.01, AverageCostPerSuccess: 0.45},
		{Provider: ai.ProviderGemini, TaskShape: TaskShapeFrontendPatch, SampleCount: 5, FirstPassSampleCount: 5, CompilePassRate: 0.88, FirstPassVerificationRate: 0.87, RepairSuccessRate: 0.84, PromotionRate: 0.85, FailureClassRecurrence: 0.05, TruncationRate: 0.02, AverageCostPerSuccess: 0.03},
	}
	available := []ai.AIProvider{ai.ProviderClaude, ai.ProviderGemini}

	fastBuild := &Build{PowerMode: PowerFast}
	if got := selectProviderByScorecard(fastBuild, RoleFrontend, TaskShapeFrontendPatch, available, scorecards); got != ai.ProviderGemini {
		t.Fatalf("fast mode provider = %s, want cheaper provider %s", got, ai.ProviderGemini)
	}

	maxBuild := &Build{PowerMode: PowerMax}
	if got := selectProviderByScorecard(maxBuild, RoleFrontend, TaskShapeFrontendPatch, available, scorecards); got != ai.ProviderClaude {
		t.Fatalf("max mode provider = %s, want higher-quality provider %s", got, ai.ProviderClaude)
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

func TestRankedFallbackProvidersForTaskUsesScorecardsAndCooldown(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderGPT4, ai.ProviderClaude, ai.ProviderGemini},
			hasConfiguredProvider: true,
		},
		providerCooldowns: make(map[string]map[ai.AIProvider]time.Time),
	}
	build := &Build{
		ID:           "build-ranked-fallback",
		ProviderMode: "platform",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
				ProviderScorecards: []ProviderScorecard{
					{Provider: ai.ProviderClaude, TaskShape: TaskShapeFrontendPatch, SampleCount: 5, FirstPassSampleCount: 5, CompilePassRate: 0.70, FirstPassVerificationRate: 0.68, RepairSuccessRate: 0.72, PromotionRate: 0.70, FailureClassRecurrence: 0.20, TruncationRate: 0.04, AverageCostPerSuccess: 0.10},
					{Provider: ai.ProviderGemini, TaskShape: TaskShapeFrontendPatch, SampleCount: 5, FirstPassSampleCount: 5, CompilePassRate: 0.96, FirstPassVerificationRate: 0.95, RepairSuccessRate: 0.90, PromotionRate: 0.94, FailureClassRecurrence: 0.06, TruncationRate: 0.01, AverageCostPerSuccess: 0.05},
				},
			},
		},
	}
	task := &Task{
		ID:   "task-ranked-fallback",
		Type: TaskGenerateUI,
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				ID:        "wo-front",
				Role:      RoleFrontend,
				TaskShape: TaskShapeFrontendPatch,
			},
		},
	}
	tried := map[ai.AIProvider]bool{ai.ProviderGPT4: true}

	got := am.rankedFallbackProvidersForTask(build, task, RoleFrontend, tried)
	if len(got) == 0 || got[0] != ai.ProviderGemini {
		t.Fatalf("fallback order = %+v, want Gemini first by scorecard", got)
	}

	am.markProviderTemporaryFailure(build.ID, ai.ProviderGemini)
	got = am.rankedFallbackProvidersForTask(build, task, RoleFrontend, tried)
	if len(got) == 0 || got[0] == ai.ProviderGemini {
		t.Fatalf("fallback order = %+v, expected cooled-down Gemini to be skipped", got)
	}
}

func TestPlanningRetryRotatesProviderAfterProviderFailure(t *testing.T) {
	am := newTestIterationManager(&stubAIRouter{
		providers:             []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4},
		hasConfiguredProvider: true,
	})
	build := &Build{
		ID:           "build-plan-rotation",
		Status:       BuildPlanning,
		ProviderMode: "platform",
		PowerMode:    PowerBalanced,
		MaxRetries:   2,
		Agents:       map[string]*Agent{},
	}
	lead := &Agent{
		ID:       "lead-1",
		BuildID:  build.ID,
		Role:     RoleLead,
		Provider: ai.ProviderClaude,
		Model:    "claude-opus-4-7",
		Status:   StatusWorking,
	}
	task := &Task{
		ID:         "plan-1",
		Type:       TaskPlan,
		Status:     TaskInProgress,
		MaxRetries: 2,
		Input:      map[string]any{"description": "Build an app"},
	}
	lead.CurrentTask = task
	build.Agents[lead.ID] = lead
	build.Tasks = []*Task{task}
	am.agents[lead.ID] = lead
	am.builds[build.ID] = build

	am.handleTaskFailure(lead, task, &TaskResult{
		TaskID:  task.ID,
		AgentID: lead.ID,
		Error:   errors.New("rate limit exceeded"),
	})

	if lead.Provider != ai.ProviderGPT4 {
		t.Fatalf("lead provider = %s, want provider rotation to GPT4", lead.Provider)
	}
	if taskInputStringValue(task.Input, "retry_provider_rotation") != "claude -> gpt4" {
		t.Fatalf("missing retry provider rotation marker, got %+v", task.Input)
	}
	select {
	case queued := <-am.taskQueue:
		if queued.ID != task.ID {
			t.Fatalf("queued task = %s, want %s", queued.ID, task.ID)
		}
	default:
		t.Fatal("expected planning task to be requeued")
	}
}

func TestAssignProvidersToRolesForBuild_UsesSpecialistRoutingInPlatformModeWithOllama(t *testing.T) {
	am := &AgentManager{}
	build := &Build{
		ID:           "build-platform-specialists",
		ProviderMode: "platform",
		PowerMode:    PowerBalanced,
	}

	roles := []AgentRole{RoleArchitect, RoleFrontend, RoleBackend, RoleTesting, RoleReviewer, RoleSolver}
	assignments := am.assignProvidersToRolesForBuild(build, []ai.AIProvider{
		ai.ProviderClaude,
		ai.ProviderGPT4,
		ai.ProviderGemini,
		ai.ProviderGrok,
		ai.ProviderOllama,
	}, roles)

	want := map[AgentRole]ai.AIProvider{
		RoleArchitect: ai.ProviderClaude,
		RoleFrontend:  ai.ProviderGPT4,
		RoleBackend:   ai.ProviderGPT4,
		RoleTesting:   ai.ProviderGemini,
		RoleReviewer:  ai.ProviderGrok,
		RoleSolver:    ai.ProviderGemini,
	}
	for role, wantProvider := range want {
		if got := assignments[role]; got != wantProvider {
			t.Fatalf("%s provider = %s, want %s", role, got, wantProvider)
		}
	}
}

func TestAssignProvidersToRolesForBuild_ForcesOllamaForBYOKWhenAvailable(t *testing.T) {
	am := &AgentManager{}
	build := &Build{
		ID:           "build-byok-ollama-primary",
		ProviderMode: "byok",
		PowerMode:    PowerBalanced,
	}

	roles := []AgentRole{RoleArchitect, RoleFrontend, RoleBackend, RoleTesting, RoleReviewer}
	assignments := am.assignProvidersToRolesForBuild(build, []ai.AIProvider{
		ai.ProviderClaude,
		ai.ProviderGPT4,
		ai.ProviderGemini,
		ai.ProviderOllama,
	}, roles)

	for _, role := range roles {
		if got := assignments[role]; got != ai.ProviderOllama {
			t.Fatalf("%s provider = %s, want %s", role, got, ai.ProviderOllama)
		}
	}
}

func TestGetNextFallbackProviderForTask_PrefersTaskPreferredProvider(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4, ai.ProviderGemini},
			hasConfiguredProvider: true,
		},
	}
	build := &Build{
		ID:           "build-fallback-preferred",
		ProviderMode: "platform",
	}
	task := &Task{
		ID:   "task-fallback-preferred",
		Type: TaskGenerateUI,
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				ID:                "wo-front-preferred",
				Role:              RoleFrontend,
				TaskShape:         TaskShapeFrontendPatch,
				PreferredProvider: ai.ProviderClaude,
			},
		},
	}

	got := am.getNextFallbackProviderForTask(build, task, RoleFrontend, ai.ProviderGPT4)
	if got != ai.ProviderClaude {
		t.Fatalf("fallback provider = %s, want task preferred provider %s", got, ai.ProviderClaude)
	}
}

func TestAssignTaskSwitchesAgentProviderToTaskPreferredProvider(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4, ai.ProviderGemini},
			hasConfiguredProvider: true,
		},
		agents:      map[string]*Agent{},
		builds:      map[string]*Build{},
		taskQueue:   make(chan *Task, 1),
		resultQueue: make(chan *TaskResult, 1),
		subscribers: map[string][]chan *WSMessage{},
	}

	build := &Build{
		ID:           "build-assign-preferred",
		Status:       BuildInProgress,
		ProviderMode: "platform",
		Agents:       map[string]*Agent{},
	}
	agent := &Agent{
		ID:       "agent-front-preferred",
		Role:     RoleFrontend,
		Provider: ai.ProviderGPT4,
		Model:    selectModelForPowerMode(ai.ProviderGPT4, PowerBalanced),
		BuildID:  build.ID,
		Status:   StatusIdle,
	}
	task := &Task{
		ID:         "task-front-preferred",
		Type:       TaskGenerateUI,
		Status:     TaskPending,
		MaxRetries: 2,
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				ID:                "wo-front-preferred",
				Role:              RoleFrontend,
				TaskShape:         TaskShapeFrontendPatch,
				PreferredProvider: ai.ProviderClaude,
			},
		},
		CreatedAt: time.Now(),
	}
	build.Agents[agent.ID] = agent
	am.agents[agent.ID] = agent
	am.builds[build.ID] = build

	if err := am.AssignTask(agent.ID, task); err != nil {
		t.Fatalf("AssignTask returned error: %v", err)
	}
	if agent.Provider != ai.ProviderClaude {
		t.Fatalf("agent provider = %s, want %s", agent.Provider, ai.ProviderClaude)
	}
	if agent.Model == "" {
		t.Fatalf("expected provider switch to update model")
	}
}
