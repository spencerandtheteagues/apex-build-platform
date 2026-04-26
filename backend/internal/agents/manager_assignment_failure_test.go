package agents

import (
	"errors"
	"strings"
	"testing"
)

func TestEnqueueRecoveryTask_DoesNotLeavePendingTaskWithoutSolver(t *testing.T) {
	t.Parallel()

	manager := newTestIterationManager(nil)
	build := &Build{
		ID:          "recovery-no-solver-build",
		Status:      BuildReviewing,
		Description: "Recover a failed review",
		MaxRetries:  3,
		Agents:      map[string]*Agent{},
	}
	failedTask := &Task{
		ID:          "review-failed",
		Type:        TaskReview,
		Description: "Final review",
		Status:      TaskFailed,
		Input:       map[string]any{},
	}
	build.Tasks = []*Task{failedTask}
	manager.builds[build.ID] = build

	if queued := manager.enqueueRecoveryTask(build.ID, failedTask, errors.New("review timed out")); queued {
		t.Fatal("expected recovery task to remain unqueued without a solver")
	}
	if len(build.Tasks) != 1 {
		t.Fatalf("expected only the original failed task to remain, got %d tasks", len(build.Tasks))
	}
	if failedTask.Status != TaskFailed {
		t.Fatalf("expected original task to stay failed, got %s", failedTask.Status)
	}
	if _, ok := failedTask.Input["recovery_queued"]; ok {
		t.Fatalf("expected recovery_queued flag to remain unset, got %+v", failedTask.Input["recovery_queued"])
	}
	if _, ok := failedTask.Input["superseded_by_recovery"]; ok {
		t.Fatalf("expected no superseding recovery task, got %+v", failedTask.Input["superseded_by_recovery"])
	}
}

func TestEnqueueRecoveryTask_RollsBackWhenAssignmentFails(t *testing.T) {
	t.Parallel()

	manager := newTestIterationManager(nil)
	build := &Build{
		ID:          "recovery-assign-fail-build",
		Status:      BuildReviewing,
		Description: "Recover a failed review",
		MaxRetries:  3,
		Agents: map[string]*Agent{
			"solver-1": {
				ID:      "solver-1",
				BuildID: "recovery-assign-fail-build",
				Role:    RoleSolver,
				Status:  StatusIdle,
			},
		},
	}
	failedTask := &Task{
		ID:          "review-failed",
		Type:        TaskReview,
		Description: "Final review",
		Status:      TaskFailed,
		Input:       map[string]any{},
	}
	build.Tasks = []*Task{failedTask}
	manager.builds[build.ID] = build

	if queued := manager.enqueueRecoveryTask(build.ID, failedTask, errors.New("review timed out")); queued {
		t.Fatal("expected recovery task assignment failure to roll back queued recovery")
	}
	if len(build.Tasks) != 1 {
		t.Fatalf("expected rollback to remove recovery task, got %d tasks", len(build.Tasks))
	}
	if failedTask.Status != TaskFailed {
		t.Fatalf("expected original task to be restored to failed, got %s", failedTask.Status)
	}
	if !strings.Contains(failedTask.Error, "review timed out") {
		t.Fatalf("expected original task error to retain failure cause, got %q", failedTask.Error)
	}
	if _, ok := failedTask.Input["recovery_queued"]; ok {
		t.Fatalf("expected rollback to clear recovery_queued flag, got %+v", failedTask.Input["recovery_queued"])
	}
	if _, ok := failedTask.Input["superseded_by_recovery"]; ok {
		t.Fatalf("expected rollback to clear superseded_by_recovery, got %+v", failedTask.Input["superseded_by_recovery"])
	}
}

func TestHandleTestCompletion_NoFixAgentFailsRecoveryTask(t *testing.T) {
	t.Parallel()

	manager := newTestIterationManager(nil)
	build := &Build{
		ID:          "test-fix-no-agent-build",
		Status:      BuildTesting,
		Description: "Exercise test repair",
		MaxRetries:  2,
		Agents:      map[string]*Agent{},
	}
	sourceTask := &Task{
		ID:     "test-task",
		Type:   TaskTest,
		Status: TaskCompleted,
	}
	build.Tasks = []*Task{sourceTask}
	manager.builds[build.ID] = build

	manager.handleTestCompletion(build, sourceTask, &TaskOutput{
		Messages: []string{"FAIL: expected 1 got 0"},
	})

	if len(build.Tasks) != 2 {
		t.Fatalf("expected a failed repair task to be recorded, got %d tasks", len(build.Tasks))
	}
	fixTask := build.Tasks[1]
	if fixTask.Type != TaskFix {
		t.Fatalf("expected follow-up task type %s, got %s", TaskFix, fixTask.Type)
	}
	if fixTask.Status != TaskFailed {
		t.Fatalf("expected follow-up task to fail fast without an assignee, got %s", fixTask.Status)
	}
	if got := taskInputStringValue(fixTask.Input, "action"); got != "fix_tests" {
		t.Fatalf("expected test-fix action, got %q", got)
	}
	if !strings.Contains(fixTask.Error, "no available agent") {
		t.Fatalf("expected missing-agent error, got %q", fixTask.Error)
	}
}

func TestHandleReviewCompletion_NoFixAgentFailsRecoveryTask(t *testing.T) {
	t.Parallel()

	manager := newTestIterationManager(nil)
	build := &Build{
		ID:          "review-fix-no-agent-build",
		Status:      BuildReviewing,
		Description: "Exercise review repair",
		MaxRetries:  2,
		Agents:      map[string]*Agent{},
	}
	sourceTask := &Task{
		ID:     "review-task",
		Type:   TaskReview,
		Status: TaskCompleted,
	}
	build.Tasks = []*Task{sourceTask}
	manager.builds[build.ID] = build

	manager.handleReviewCompletion(build, sourceTask, &TaskOutput{
		Messages: []string{"critical security vulnerability in auth middleware"},
	})

	if len(build.Tasks) != 2 {
		t.Fatalf("expected a failed review-repair task to be recorded, got %d tasks", len(build.Tasks))
	}
	fixTask := build.Tasks[1]
	if fixTask.Type != TaskFix {
		t.Fatalf("expected follow-up task type %s, got %s", TaskFix, fixTask.Type)
	}
	if fixTask.Status != TaskFailed {
		t.Fatalf("expected follow-up task to fail fast without an assignee, got %s", fixTask.Status)
	}
	if got := taskInputStringValue(fixTask.Input, "action"); got != "fix_review_issues" {
		t.Fatalf("expected review-fix action, got %q", got)
	}
	if !strings.Contains(fixTask.Error, "no available agent") {
		t.Fatalf("expected missing-agent error, got %q", fixTask.Error)
	}
}

func TestSchedulePostFixValidation_AssignmentFailureFailsValidationTasks(t *testing.T) {
	t.Parallel()

	manager := newTestIterationManager(nil)
	build := &Build{
		ID:          "post-fix-validation-assign-fail-build",
		Status:      BuildReviewing,
		Description: "Exercise post-fix validation",
		MaxRetries:  2,
		Agents: map[string]*Agent{
			"tester-1": {
				ID:      "tester-1",
				BuildID: "post-fix-validation-assign-fail-build",
				Role:    RoleTesting,
				Status:  StatusIdle,
			},
			"reviewer-1": {
				ID:      "reviewer-1",
				BuildID: "post-fix-validation-assign-fail-build",
				Role:    RoleReviewer,
				Status:  StatusIdle,
			},
		},
	}
	sourceTask := &Task{
		ID:     "fix-task",
		Type:   TaskFix,
		Status: TaskCompleted,
		Input: map[string]any{
			"action": "solve_build_failure",
		},
	}
	build.Tasks = []*Task{sourceTask}
	manager.builds[build.ID] = build

	manager.schedulePostFixValidation(build, sourceTask)

	if len(build.Tasks) != 2 {
		t.Fatalf("expected only the regression validation task, got %d tasks", len(build.Tasks))
	}
	task := build.Tasks[1]
	if task.Type != TaskTest {
		t.Fatalf("expected regression validation task, got %s", task.Type)
	}
	if task.Status != TaskFailed {
		t.Fatalf("expected validation task %s to fail fast on assignment error, got %s", task.ID, task.Status)
	}
	if !strings.Contains(task.Error, "could not be assigned") {
		t.Fatalf("expected assignment error on %s, got %q", task.ID, task.Error)
	}
}

func TestHandleTestCompletion_SchedulesPostFixReviewAfterPassingRegression(t *testing.T) {
	t.Parallel()

	manager := newTestIterationManager(nil)
	manager.taskQueue = make(chan *Task, 8)
	build := &Build{
		ID:          "post-fix-regression-pass-build",
		Status:      BuildReviewing,
		Description: "Exercise serialized post-fix validation",
		MaxRetries:  2,
		Agents: map[string]*Agent{
			"reviewer-1": {
				ID:      "reviewer-1",
				BuildID: "post-fix-regression-pass-build",
				Role:    RoleReviewer,
				Status:  StatusIdle,
				Model:   "kimi-k2.6:cloud",
			},
		},
	}
	sourceTask := &Task{
		ID:     "regression-task",
		Type:   TaskTest,
		Status: TaskCompleted,
		Input: map[string]any{
			"action":                   "regression_test",
			"schedule_post_fix_review": true,
			"fix_context_action":       "solve_build_failure",
		},
	}
	build.Tasks = []*Task{sourceTask}
	manager.builds[build.ID] = build
	manager.agents["reviewer-1"] = build.Agents["reviewer-1"]

	manager.handleTestCompletion(build, sourceTask, &TaskOutput{
		Messages: []string{"All regression checks passed"},
	})

	if len(build.Tasks) != 2 {
		t.Fatalf("expected a follow-up review task, got %d tasks", len(build.Tasks))
	}
	reviewTask := build.Tasks[1]
	if reviewTask.Type != TaskReview {
		t.Fatalf("expected follow-up task type %s, got %s", TaskReview, reviewTask.Type)
	}
	if got := taskInputStringValue(reviewTask.Input, "action"); got != "post_fix_review" {
		t.Fatalf("expected post-fix review action, got %q", got)
	}
	if reviewTask.Status != TaskInProgress {
		t.Fatalf("expected review task to be assigned immediately, got %s", reviewTask.Status)
	}
}

func TestResumeBuildExecution_FailsUnresumablePendingTask(t *testing.T) {
	t.Parallel()

	manager := newTestIterationManager(nil)
	build := &Build{
		ID:          "resume-unassigned-build",
		Status:      BuildReviewing,
		Description: "Resume a pending fix task",
		Agents:      map[string]*Agent{},
		Tasks: []*Task{
			{
				ID:     "pending-fix",
				Type:   TaskFix,
				Status: TaskPending,
				Input: map[string]any{
					"action": "solve_build_failure",
				},
			},
		},
	}
	manager.builds[build.ID] = build

	manager.resumeBuildExecution(build, false)

	task := build.Tasks[0]
	if task.Status != TaskFailed {
		t.Fatalf("expected unresumable pending task to fail fast, got %s", task.Status)
	}
	if !strings.Contains(task.Error, "no resumable assignee") {
		t.Fatalf("expected no-assignee resume error, got %q", task.Error)
	}
}
