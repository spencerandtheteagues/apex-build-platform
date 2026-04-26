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
