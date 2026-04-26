package agents

import (
	"context"
	"strings"
	"testing"
	"time"

	"apex-build/internal/ai"
)

func TestProcessResultCancelledTaskReleasesAgentAndFinalizesBuild(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID:          "task-cancelled-recovery",
		Type:        TaskFix,
		Description: "Recover final validation failure",
		Status:      TaskCancelled,
		AssignedTo:  "agent-solver-cancelled",
		Input: map[string]any{
			"action": "solve_build_failure",
		},
		CreatedAt: time.Now().UTC(),
	}
	agent := &Agent{
		ID:          "agent-solver-cancelled",
		Role:        RoleSolver,
		Provider:    ai.ProviderOllama,
		Model:       "kimi-k2.6:cloud",
		Status:      StatusWorking,
		BuildID:     "build-cancelled-recovery",
		CurrentTask: task,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	build := &Build{
		ID:                        "build-cancelled-recovery",
		Status:                    BuildReviewing,
		Mode:                      ModeFast,
		PowerMode:                 PowerFast,
		ProviderMode:              "platform",
		PhasedPipelineComplete:    true,
		ReadinessRecoveryAttempts: maxAutomatedRecoveryAttempts(PowerFast),
		Tasks:                     []*Task{task},
		Agents:                    map[string]*Agent{agent.ID: agent},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	am := &AgentManager{
		ctx: context.Background(),
		aiRouter: &stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderOllama},
			hasConfiguredProvider: true,
		},
		agents:      map[string]*Agent{agent.ID: agent},
		builds:      map[string]*Build{build.ID: build},
		taskQueue:   make(chan *Task, 1),
		resultQueue: make(chan *TaskResult, 1),
		subscribers: map[string][]chan *WSMessage{},
	}

	am.processResult(&TaskResult{
		TaskID:  task.ID,
		AgentID: agent.ID,
		Success: true,
		Output: &TaskOutput{
			Messages: []string{"ignored after cancellation"},
		},
	})

	if agent.CurrentTask != nil {
		t.Fatalf("expected cancelled task ownership to be cleared, got %+v", agent.CurrentTask)
	}
	if agent.Status != StatusIdle {
		t.Fatalf("expected solver agent to return to idle after cancelled task result, got %s", agent.Status)
	}
	if build.Status != BuildFailed {
		t.Fatalf("expected build finalization to resume after cancelled task result, got %s", build.Status)
	}
	if build.CompletedAt == nil {
		t.Fatal("expected build to become terminal after cancelled task result")
	}
	if strings.TrimSpace(build.Error) == "" {
		t.Fatal("expected build failure reason to be recorded after resumed finalization")
	}
}
