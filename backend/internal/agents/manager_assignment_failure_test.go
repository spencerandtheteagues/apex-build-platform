package agents

import (
	"errors"
	"testing"
	"time"
)

func TestEnqueueRecoveryTaskNoSolverRestoresOriginalFailure(t *testing.T) {
	failedTask := &Task{
		ID:          "review-root",
		Type:        TaskReview,
		Description: "Final review",
		Status:      TaskFailed,
		Input:       map[string]any{},
	}
	build := &Build{
		ID:          "recovery-no-solver",
		Description: "Build a crypto tracker",
		Tasks:       []*Task{failedTask},
		Agents:      map[string]*Agent{},
	}
	manager := &AgentManager{
		builds:        map[string]*Build{build.ID: build},
		subscribers:   map[string][]chan *WSMessage{},
		taskQueue:     make(chan *Task, 4),
		resultQueue:   make(chan *TaskResult, 4),
		buildMonitors: make(map[string]struct{}),
	}

	manager.enqueueRecoveryTask(build.ID, failedTask, errors.New("review failed"))

	if failedTask.Status != TaskFailed {
		t.Fatalf("expected failed task to remain failed, got %s", failedTask.Status)
	}
	if _, ok := failedTask.Input["superseded_by_recovery"]; ok {
		t.Fatalf("expected superseded_by_recovery to be cleared after rollback")
	}
	if _, ok := failedTask.Input["recovery_queued"]; ok {
		t.Fatalf("expected recovery_queued to be cleared after rollback")
	}
	if len(build.Tasks) != 2 {
		t.Fatalf("expected recovery task to remain recorded, got %d tasks", len(build.Tasks))
	}
	recoveryTask := build.Tasks[1]
	if recoveryTask.Status != TaskFailed {
		t.Fatalf("expected recovery task to fail instead of staying pending, got %s", recoveryTask.Status)
	}
	if recoveryTask.Error == "" {
		t.Fatal("expected recovery task assignment failure to be recorded")
	}
	if recoveryTask.CompletedAt == nil {
		t.Fatal("expected recovery task completion time to be set")
	}
}

func TestSchedulePostFixValidationAssignmentFailureFailsTasks(t *testing.T) {
	build := &Build{
		ID:          "post-fix-validation-assignment-failure",
		Description: "Build a crypto tracker",
		Agents: map[string]*Agent{
			"testing-1": {ID: "testing-1", BuildID: "post-fix-validation-assignment-failure", Role: RoleTesting},
			"review-1":  {ID: "review-1", BuildID: "post-fix-validation-assignment-failure", Role: RoleReviewer},
		},
	}
	sourceTask := &Task{
		ID:     "fix-root",
		Type:   TaskFix,
		Status: TaskCompleted,
		Input: map[string]any{
			"action": "fix_review_issues",
		},
	}
	manager := &AgentManager{
		builds:      map[string]*Build{build.ID: build},
		subscribers: map[string][]chan *WSMessage{},
		// Intentionally leave manager.agents empty so AssignTask fails with agent not found.
		agents: make(map[string]*Agent),
	}

	manager.schedulePostFixValidation(build, sourceTask)

	if len(build.Tasks) != 2 {
		t.Fatalf("expected 2 follow-up validation tasks, got %d", len(build.Tasks))
	}
	for _, task := range build.Tasks {
		if task.Status != TaskFailed {
			t.Fatalf("expected validation task %s to fail instead of remaining pending, got %s", task.ID, task.Status)
		}
		if task.Error == "" {
			t.Fatalf("expected validation task %s to record assignment error", task.ID)
		}
	}
}

func TestAssignPhaseAgentsAssignmentFailureFailsTask(t *testing.T) {
	frontendAgent := &Agent{ID: "frontend-1", BuildID: "phase-assign-failure", Role: RoleFrontend}
	build := &Build{
		ID:          "phase-assign-failure",
		Description: "Build a crypto tracker",
		Agents: map[string]*Agent{
			frontendAgent.ID: frontendAgent,
		},
	}
	manager := &AgentManager{
		builds:      map[string]*Build{build.ID: build},
		subscribers: map[string][]chan *WSMessage{},
		agents:      make(map[string]*Agent),
	}

	taskIDs := manager.assignPhaseAgents(build, []agentPriority{{agent: frontendAgent, priority: 1}}, build.Description)

	if len(taskIDs) != 1 {
		t.Fatalf("expected one phase task id, got %d", len(taskIDs))
	}
	if len(build.Tasks) != 1 {
		t.Fatalf("expected one phase task to be created, got %d", len(build.Tasks))
	}
	if build.Tasks[0].Status != TaskFailed {
		t.Fatalf("expected phase task to fail fast on assignment error, got %s", build.Tasks[0].Status)
	}
	if build.Tasks[0].Error == "" {
		t.Fatal("expected phase task assignment failure to be captured")
	}
}

func TestUpdateBuildSnapshotStateLockedDoesNotRegressLatePhase(t *testing.T) {
	build := &Build{
		ID: "phase-regression-guard",
		SnapshotState: BuildSnapshotState{
			CurrentPhase:      "validation",
			QualityGateStage:  "validation",
			QualityGateStatus: "running",
		},
		UpdatedAt: time.Now(),
	}

	updated := updateBuildSnapshotStateLocked(build, &WSMessage{
		Type: WSBuildProgress,
		Data: map[string]any{
			"phase":               "integration",
			"quality_gate_stage":  "testing",
			"quality_gate_active": true,
		},
	})
	if !updated {
		t.Fatal("expected snapshot update to apply quality gate changes")
	}
	if build.SnapshotState.CurrentPhase != "validation" {
		t.Fatalf("expected late build phase to remain validation, got %q", build.SnapshotState.CurrentPhase)
	}
	if build.SnapshotState.QualityGateStage != "testing" {
		t.Fatalf("expected quality gate stage to update within the late-phase cluster, got %q", build.SnapshotState.QualityGateStage)
	}
}
