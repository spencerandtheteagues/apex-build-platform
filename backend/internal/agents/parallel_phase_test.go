package agents

import "testing"

func makeAgents(roles ...AgentRole) []agentPriority {
	out := make([]agentPriority, len(roles))
	for i, r := range roles {
		out[i] = agentPriority{agent: &Agent{Role: r}, priority: i + 1}
	}
	return out
}

func TestBuildExecutionPhasesParallelStructure(t *testing.T) {
	arch := makeAgents(RoleArchitect)
	frontend := makeAgents(RoleFrontend)
	db := makeAgents(RoleDatabase)
	backend := makeAgents(RoleBackend)
	tests := makeAgents(RoleTesting)
	review := makeAgents(RoleReviewer)

	phases := buildExecutionPhasesParallel(arch, frontend, db, backend, tests, review)

	if len(phases) != 4 {
		t.Fatalf("expected 4 phases, got %d: %v", len(phases), phases)
	}

	keys := []string{"architecture", "parallel_core", "integration", "review"}
	for i, want := range keys {
		if phases[i].key != want {
			t.Errorf("phases[%d].key = %q, want %q", i, phases[i].key, want)
		}
	}
}

func TestBuildExecutionPhasesParallelCoreContainsAllThreeSurfaces(t *testing.T) {
	frontend := makeAgents(RoleFrontend)
	db := makeAgents(RoleDatabase)
	backend := makeAgents(RoleBackend)

	phases := buildExecutionPhasesParallel(nil, frontend, db, backend, nil, nil)

	var corePhase *executionPhase
	for i := range phases {
		if phases[i].key == "parallel_core" {
			corePhase = &phases[i]
			break
		}
	}
	if corePhase == nil {
		t.Fatal("parallel_core phase not found")
	}

	// Should contain frontend + db + backend agents (3 total)
	if len(corePhase.agents) != 3 {
		t.Fatalf("expected 3 agents in parallel_core, got %d", len(corePhase.agents))
	}
	roles := map[AgentRole]bool{}
	for _, ap := range corePhase.agents {
		roles[ap.agent.Role] = true
	}
	for _, r := range []AgentRole{RoleFrontend, RoleDatabase, RoleBackend} {
		if !roles[r] {
			t.Errorf("parallel_core missing role %q", r)
		}
	}
}

func TestBuildExecutionPhasesParallelNeverHasFrontendUIKey(t *testing.T) {
	phases := buildExecutionPhasesParallel(
		makeAgents(RoleArchitect),
		makeAgents(RoleFrontend),
		makeAgents(RoleDatabase),
		makeAgents(RoleBackend),
		makeAgents(RoleTesting),
		makeAgents(RoleReviewer),
	)
	for _, p := range phases {
		if p.key == "frontend_ui" {
			t.Errorf("parallel mode must not produce a frontend_ui phase (approval gate would never fire)")
		}
	}
}

func TestBuildExecutionPhasesParallelQualityStages(t *testing.T) {
	phases := buildExecutionPhasesParallel(
		makeAgents(RoleArchitect),
		makeAgents(RoleFrontend),
		makeAgents(RoleDatabase),
		makeAgents(RoleBackend),
		makeAgents(RoleTesting),
		makeAgents(RoleReviewer),
	)

	stageByKey := map[string]string{}
	for _, p := range phases {
		stageByKey[p.key] = p.qualityStage
	}

	if stageByKey["integration"] != "testing" {
		t.Errorf("integration phase qualityStage = %q, want %q", stageByKey["integration"], "testing")
	}
	if stageByKey["review"] != "review" {
		t.Errorf("review phase qualityStage = %q, want %q", stageByKey["review"], "review")
	}
	if stageByKey["parallel_core"] != "" {
		t.Errorf("parallel_core phase should have no qualityStage, got %q", stageByKey["parallel_core"])
	}
}

func TestBuildExecutionPhasesParallelEmptyAgentGroups(t *testing.T) {
	phases := buildExecutionPhasesParallel(nil, nil, nil, nil, nil, nil)
	if len(phases) != 4 {
		t.Fatalf("expected 4 phases even with empty agent groups, got %d", len(phases))
	}
	for _, p := range phases {
		if p.key == "parallel_core" && len(p.agents) != 0 {
			t.Errorf("parallel_core should have 0 agents with all-nil inputs, got %d", len(p.agents))
		}
	}
}

func TestBuildPhaseProgressWindowParallelCore(t *testing.T) {
	lo, hi, ok := buildPhaseProgressWindow("parallel_core", BuildInProgress)
	if !ok {
		t.Fatal("buildPhaseProgressWindow should recognise parallel_core")
	}
	if lo != 20 || hi != 79 {
		t.Errorf("parallel_core progress window = [%d, %d], want [20, 79]", lo, hi)
	}
}

func TestBuildExecutionPhasesParallelFrontendOrderedFirst(t *testing.T) {
	// Frontend agents must appear before db and backend in parallel_core so that
	// if a worker has capacity constraints, UI work has priority.
	frontend := makeAgents(RoleFrontend)
	db := makeAgents(RoleDatabase)
	backend := makeAgents(RoleBackend)

	phases := buildExecutionPhasesParallel(nil, frontend, db, backend, nil, nil)
	var corePhase *executionPhase
	for i := range phases {
		if phases[i].key == "parallel_core" {
			corePhase = &phases[i]
			break
		}
	}
	if corePhase == nil || len(corePhase.agents) == 0 {
		t.Fatal("parallel_core phase not found or empty")
	}
	if corePhase.agents[0].agent.Role != RoleFrontend {
		t.Errorf("first agent in parallel_core should be RoleFrontend, got %q", corePhase.agents[0].agent.Role)
	}
}
