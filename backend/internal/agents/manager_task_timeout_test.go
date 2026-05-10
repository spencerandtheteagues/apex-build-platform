package agents

import (
	"context"
	"testing"
	"time"

	"apex-build/internal/ai"
)

func TestTaskExecutionTimeoutForTask_ExtendsReviewTasks(t *testing.T) {
	t.Parallel()

	manager := newTestIterationManager(nil)
	build := &Build{
		ID:        "review-timeout-build",
		Status:    BuildReviewing,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
	}
	agent := &Agent{
		ID:       "reviewer-1",
		BuildID:  build.ID,
		Role:     RoleReviewer,
		Provider: ai.ProviderGPT4,
	}
	task := &Task{
		ID:          "review-task",
		Type:        TaskReview,
		Description: "Final review",
		Status:      TaskPending,
	}

	if got := manager.taskExecutionTimeoutForTask(build, task, agent); got != 12*time.Minute {
		t.Fatalf("review timeout = %v, want 12m", got)
	}
}

func TestTaskExecutionTimeoutForTask_DoesNotExtendPreviewRequiredReviewTasks(t *testing.T) {
	t.Parallel()

	manager := newTestIterationManager(nil)
	build := &Build{
		ID:                  "preview-review-timeout-build",
		Status:              BuildReviewing,
		Mode:                ModeFull,
		PowerMode:           PowerBalanced,
		RequirePreviewReady: true,
	}
	agent := &Agent{
		ID:       "reviewer-1",
		BuildID:  build.ID,
		Role:     RoleReviewer,
		Provider: ai.ProviderGPT4,
	}
	task := &Task{
		ID:          "review-task",
		Type:        TaskReview,
		Description: "Final review",
		Status:      TaskPending,
	}

	if got := manager.taskExecutionTimeoutForTask(build, task, agent); got >= 12*time.Minute {
		t.Fatalf("preview-required review timeout = %v, want below extended 12m floor", got)
	}
}

func TestExecuteTaskBypassesLLMReviewForPreviewRequiredBuild(t *testing.T) {
	t.Parallel()

	manager := newTestIterationManager(nil)
	build := &Build{
		ID:                  "preview-review-bypass-build",
		Status:              BuildReviewing,
		Mode:                ModeFull,
		PowerMode:           PowerBalanced,
		RequirePreviewReady: true,
		Agents:              map[string]*Agent{},
	}
	task := &Task{
		ID:          "review-task",
		Type:        TaskReview,
		Description: "Final review",
		AssignedTo:  "reviewer-1",
		Status:      TaskInProgress,
		MaxRetries:  2,
		Input: map[string]any{
			"agent_role": string(RoleReviewer),
		},
	}
	agent := &Agent{
		ID:          task.AssignedTo,
		BuildID:     build.ID,
		Role:        RoleReviewer,
		Provider:    ai.ProviderGPT4,
		Status:      StatusWorking,
		CurrentTask: task,
	}
	build.Agents[agent.ID] = agent
	build.Tasks = []*Task{task}
	manager.builds[build.ID] = build
	manager.agents[agent.ID] = agent

	manager.executeTask(task)

	select {
	case result := <-manager.resultQueue:
		if !result.Success {
			t.Fatalf("expected deterministic review success, got %+v", result)
		}
		if result.Output == nil || taskOutputMetricString(result.Output, "review_mode") != "preview_required_gate" {
			t.Fatalf("expected deterministic preview review output, got %+v", result.Output)
		}
	case <-time.After(time.Second):
		t.Fatal("expected deterministic review result without provider call")
	}

	if build.RequestsUsed != 0 {
		t.Fatalf("deterministic review should not consume AI request budget, got %d", build.RequestsUsed)
	}
}

func TestTaskExecutionTimeoutForTask_ExtendsReviewStageRecoveryTasks(t *testing.T) {
	t.Parallel()

	manager := newTestIterationManager(nil)
	build := &Build{
		ID:        "review-recovery-build",
		Status:    BuildReviewing,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotState: BuildSnapshotState{
			CurrentPhase: "validation",
		},
	}
	agent := &Agent{
		ID:       "solver-1",
		BuildID:  build.ID,
		Role:     RoleSolver,
		Provider: ai.ProviderClaude,
	}
	task := &Task{
		ID:          "solver-recovery",
		Type:        TaskFix,
		Description: "Recover final validation failure",
		Status:      TaskPending,
		Input: map[string]any{
			"action": "solve_build_failure",
		},
	}

	if got := manager.taskExecutionTimeoutForTask(build, task, agent); got != 12*time.Minute {
		t.Fatalf("review-stage recovery timeout = %v, want 12m", got)
	}
}

func TestRecoverStaleInProgressTasks_DoesNotPrematurelyRecoverReviewStageFix(t *testing.T) {
	t.Parallel()

	manager := &AgentManager{
		ctx:         context.Background(),
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		resultQueue: make(chan *TaskResult, 1),
	}

	startedAt := time.Now().Add(-9 * time.Minute).UTC()
	build := &Build{
		ID:        "review-stale-guard-build",
		Status:    BuildReviewing,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		UpdatedAt: time.Now().Add(-9 * time.Minute).UTC(),
		Agents:    make(map[string]*Agent),
		Tasks: []*Task{
			{
				ID:          "review-fix",
				Type:        TaskFix,
				Description: "Recover final validation",
				AssignedTo:  "solver-1",
				Status:      TaskInProgress,
				StartedAt:   &startedAt,
				MaxRetries:  3,
				Input: map[string]any{
					"action": "solve_build_failure",
				},
			},
		},
	}
	agent := &Agent{
		ID:       "solver-1",
		BuildID:  build.ID,
		Role:     RoleSolver,
		Provider: ai.ProviderClaude,
		Status:   StatusWorking,
	}
	build.Agents[agent.ID] = agent
	manager.builds[build.ID] = build
	manager.agents[agent.ID] = agent

	if recovered := manager.recoverStaleInProgressTasks(build, 9*time.Minute); recovered {
		t.Fatal("expected review-stage recovery task to remain in progress before the extended timeout")
	}

	select {
	case result := <-manager.resultQueue:
		t.Fatalf("expected no synthetic timeout result, got %+v", result)
	default:
	}
}

func TestRecoverStaleInProgressTasksCancelsRegisteredPlanningExecution(t *testing.T) {
	manager := &AgentManager{
		ctx:         context.Background(),
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		resultQueue: make(chan *TaskResult, 1),
	}

	execCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startedAt := time.Now().Add(-3 * time.Minute).UTC()
	task := &Task{
		ID:          "plan-stale-cancel",
		Type:        TaskPlan,
		Description: "Create build plan",
		AssignedTo:  "lead-1",
		Status:      TaskInProgress,
		StartedAt:   &startedAt,
		MaxRetries:  2,
	}
	build := &Build{
		ID:        "planning-stale-cancel-build",
		Status:    BuildPlanning,
		Mode:      ModeFast,
		PowerMode: PowerFast,
		UpdatedAt: startedAt,
		Agents:    make(map[string]*Agent),
		Tasks:     []*Task{task},
	}
	agent := &Agent{
		ID:       task.AssignedTo,
		BuildID:  build.ID,
		Role:     RoleLead,
		Provider: ai.ProviderClaude,
		Status:   StatusWorking,
	}
	build.Agents[agent.ID] = agent
	manager.builds[build.ID] = build
	manager.agents[agent.ID] = agent
	manager.registerTaskExecutionCancel(task.ID, cancel)

	if recovered := manager.recoverStaleInProgressTasks(build, 3*time.Minute); !recovered {
		t.Fatal("expected stale planning task recovery")
	}

	select {
	case <-execCtx.Done():
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected registered planning execution context to be cancelled")
	}
}

func TestBuildTimeoutForBuild_FullMaxStartsWithExtendedBudget(t *testing.T) {
	t.Parallel()

	manager := newTestIterationManager(nil)
	build := &Build{
		ID:        "full-max-timeout-build",
		Mode:      ModeFull,
		PowerMode: PowerMax,
	}

	if got := manager.buildTimeoutForBuild(build); got != 60*time.Minute {
		t.Fatalf("build timeout = %v, want 60m", got)
	}
}

func TestBuildStallTimeoutForBuild_RemainsBelowOverallBuildTimeout(t *testing.T) {
	t.Parallel()

	manager := newTestIterationManager(nil)
	build := &Build{
		ID:        "full-balanced-stall-build",
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
	}

	stallTimeout := manager.buildStallTimeoutForBuild(build)
	buildTimeout := manager.buildTimeoutForBuild(build)
	if stallTimeout != 15*time.Minute {
		t.Fatalf("stall timeout = %v, want 15m", stallTimeout)
	}
	if stallTimeout >= buildTimeout {
		t.Fatalf("stall timeout %v must remain below build timeout %v", stallTimeout, buildTimeout)
	}
}
