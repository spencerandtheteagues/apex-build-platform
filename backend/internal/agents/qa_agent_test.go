package agents

import (
	"context"
	"testing"
	"time"

	"apex-build/internal/agents/autonomous"
)

func TestDerivedTestContractIncludesFrontendAndBackendCoverage(t *testing.T) {
	t.Parallel()

	contract := derivedTestContract(&BuildPlan{
		AppType: "fullstack",
		TechStack: TechStack{
			Frontend: "React",
			Backend:  "Express",
		},
	})
	if contract == nil {
		t.Fatal("expected test contract")
	}
	for _, framework := range []string{"vitest", "@testing-library/react", "@testing-library/user-event", "@playwright/test"} {
		if !containsString(contract.Frameworks, framework) {
			t.Fatalf("expected framework %q in %+v", framework, contract.Frameworks)
		}
	}
	for _, path := range []string{"src/__tests__/", "tests/", "e2e/"} {
		if !containsString(contract.OwnedTestPaths, path) {
			t.Fatalf("expected owned test path %q in %+v", path, contract.OwnedTestPaths)
		}
	}
	if !contract.RequireFrontendSmoke {
		t.Fatalf("expected frontend smoke requirement, got %+v", contract)
	}
	if !contract.RequireBackendContractTest {
		t.Fatalf("expected backend contract requirement, got %+v", contract)
	}
	if !contract.ExcludeFromPreviewProof {
		t.Fatalf("expected preview-proof exclusion, got %+v", contract)
	}
}

func TestAssignPhaseAgentsInjectsTestingContractInputs(t *testing.T) {
	t.Parallel()

	plan := createBuildPlanFromPlanningBundle("build-qa", "Build RevenuePilot", nil, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "fullstack",
			TechStack: &autonomous.TechStack{
				Frontend: "React",
				Backend:  "Node",
				Database: "PostgreSQL",
				Styling:  "Tailwind",
			},
		},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-qa",
			EstimatedTime: time.Hour,
			CreatedAt:     time.Now(),
		},
	})
	build := &Build{
		ID:           "build-qa",
		Description:  "Build RevenuePilot",
		Status:       BuildInProgress,
		Plan:         plan,
		ProviderMode: "platform",
		Tasks:        []*Task{},
		Agents:       map[string]*Agent{},
	}
	intent := &IntentBrief{AppType: plan.AppType}
	contract := compileBuildContractFromPlan(build.ID, intent, plan)
	build.SnapshotState.Orchestration = &BuildOrchestrationState{
		Flags:              defaultBuildOrchestrationFlags(),
		BuildContract:      contract,
		WorkOrders:         compileWorkOrdersFromPlan(build.ID, contract, plan, defaultProviderScorecards(build.ProviderMode)),
		ProviderScorecards: defaultProviderScorecards(build.ProviderMode),
	}
	agent := &Agent{ID: "test-1", BuildID: build.ID, Role: RoleTesting}
	build.Agents[agent.ID] = agent

	am := &AgentManager{
		agents:      map[string]*Agent{agent.ID: agent},
		builds:      map[string]*Build{build.ID: build},
		taskQueue:   make(chan *Task, 1),
		subscribers: map[string][]chan *WSMessage{},
		ctx:         context.Background(),
	}

	taskIDs := am.assignPhaseAgents(build, []agentPriority{{agent: agent, priority: 55}}, build.Description)
	if len(taskIDs) != 1 {
		t.Fatalf("expected one task id, got %d", len(taskIDs))
	}
	task := build.Tasks[0]
	frameworks, ok := task.Input["test_frameworks"].([]string)
	if !ok || len(frameworks) == 0 {
		t.Fatalf("expected test_frameworks in task input, got %+v", task.Input["test_frameworks"])
	}
	if !containsString(frameworks, "@playwright/test") {
		t.Fatalf("expected playwright in test frameworks, got %+v", frameworks)
	}
	paths, ok := task.Input["owned_test_paths"].([]string)
	if !ok || !containsString(paths, "e2e/") {
		t.Fatalf("expected owned_test_paths with e2e/, got %+v", task.Input["owned_test_paths"])
	}
	testContract, ok := task.Input["test_contract"].(*QAWorkOrder)
	if !ok || testContract == nil {
		t.Fatalf("expected typed test_contract, got %+v", task.Input["test_contract"])
	}
	if !testContract.RequireFrontendSmoke || !testContract.RequireBackendContractTest {
		t.Fatalf("expected full-stack testing requirements, got %+v", testContract)
	}
}
