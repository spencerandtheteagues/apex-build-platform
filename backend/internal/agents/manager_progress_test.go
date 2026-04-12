package agents

import "testing"

func TestUpdateBuildProgressCapsArchitecturePhaseProgress(t *testing.T) {
	manager := &AgentManager{
		builds:      make(map[string]*Build),
		subscribers: make(map[string][]chan *WSMessage),
	}

	build := &Build{
		ID:     "phase-progress-architecture",
		Status: BuildInProgress,
		Tasks: []*Task{
			{ID: "task-1", Status: TaskCompleted},
			{ID: "task-2", Status: TaskInProgress},
		},
		Agents: map[string]*Agent{
			"lead":     {ID: "lead", Role: RoleLead, Status: StatusWorking},
			"frontend": {ID: "frontend", Role: RoleFrontend, Status: StatusCompleted},
			"backend":  {ID: "backend", Role: RoleBackend, Status: StatusWorking},
		},
		SnapshotState: BuildSnapshotState{
			CurrentPhase: "architecture",
		},
	}

	manager.updateBuildProgress(build)

	if build.Progress < 10 || build.Progress > 19 {
		t.Fatalf("expected architecture progress in 10-19 range, got %d", build.Progress)
	}
}

func TestUpdateBuildProgressKeepsReviewPhaseBelowCompletion(t *testing.T) {
	manager := &AgentManager{
		builds:      make(map[string]*Build),
		subscribers: make(map[string][]chan *WSMessage),
	}

	build := &Build{
		ID:     "phase-progress-review",
		Status: BuildReviewing,
		Tasks: []*Task{
			{ID: "task-1", Status: TaskCompleted},
			{ID: "task-2", Status: TaskCompleted},
			{ID: "task-3", Status: TaskCompleted},
			{ID: "task-4", Status: TaskCompleted},
		},
		Agents: map[string]*Agent{
			"lead":     {ID: "lead", Role: RoleLead, Status: StatusWorking},
			"frontend": {ID: "frontend", Role: RoleFrontend, Status: StatusCompleted},
			"backend":  {ID: "backend", Role: RoleBackend, Status: StatusCompleted},
			"testing":  {ID: "testing", Role: RoleTesting, Status: StatusCompleted},
		},
		SnapshotState: BuildSnapshotState{
			CurrentPhase: "review",
		},
	}

	manager.updateBuildProgress(build)

	if build.Progress < 90 || build.Progress > 98 {
		t.Fatalf("expected review progress in 90-98 range, got %d", build.Progress)
	}
}

func TestShouldPersistBuildSnapshotMessageIncludesRetryLearningEvents(t *testing.T) {
	for _, msgType := range []WSMessageType{
		"agent:retrying",
		"agent:verification_failed",
		"agent:coordination_failed",
		WSAgentError,
	} {
		t.Run(string(msgType), func(t *testing.T) {
			if !shouldPersistBuildSnapshotMessage(msgType) {
				t.Fatalf("expected %s to trigger snapshot persistence", msgType)
			}
		})
	}
}
