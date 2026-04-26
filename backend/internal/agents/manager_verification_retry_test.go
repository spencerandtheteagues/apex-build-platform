package agents

import (
	"context"
	"testing"
	"time"

	"apex-build/internal/ai"
)

func TestProcessResultVerificationFailureUsesReduceContextStrategyForInitialTruncation(t *testing.T) {
	t.Parallel()

	am, _, _, task := newVerificationRetryTestFixture(nil)

	am.processResult(&TaskResult{
		TaskID:  task.ID,
		AgentID: task.AssignedTo,
		Success: true,
		Output:  truncatedOpeningHoursOutput(),
	})

	if task.Status != TaskPending {
		t.Fatalf("expected task pending after verification retry, got %s", task.Status)
	}
	if task.RetryCount != 1 {
		t.Fatalf("expected retry count 1, got %d", task.RetryCount)
	}
	if task.RetryStrategy != RetryStrategy("reduce_context") {
		t.Fatalf("expected reduce_context retry strategy, got %s", task.RetryStrategy)
	}
	if got := task.Input["retry_strategy"]; got != "reduce_context" {
		t.Fatalf("expected retry_strategy input reduce_context, got %#v", got)
	}
	if _, ok := task.Input["previous_errors"]; !ok {
		t.Fatalf("expected previous_errors to be captured, got %+v", task.Input)
	}
	if len(am.taskQueue) != 1 {
		t.Fatalf("expected retried task to be requeued, queue len=%d", len(am.taskQueue))
	}
}

func TestProcessResultVerificationFailureEscalatesTruncationToSwitchProvider(t *testing.T) {
	t.Parallel()

	history := []FailureFingerprint{
		{
			Provider:            ai.ProviderGPT4,
			TaskShape:           TaskShapeFrontendPatch,
			FailureClass:        "truncation",
			RepairPathChosen:    []string{"task_execution", "reduce_context"},
			RepairSucceeded:     false,
			TokenCostToRecovery: 900,
			CreatedAt:           time.Now().Add(-time.Minute).UTC(),
		},
	}
	am, build, _, task := newVerificationRetryTestFixture(history)

	am.processResult(&TaskResult{
		TaskID:  task.ID,
		AgentID: task.AssignedTo,
		Success: true,
		Output:  truncatedOpeningHoursOutput(),
	})

	if task.Status != TaskPending {
		t.Fatalf("expected task pending after verification retry, got %s", task.Status)
	}
	if task.RetryCount != 1 {
		t.Fatalf("expected retry count 1, got %d", task.RetryCount)
	}
	if task.RetryStrategy != RetryStrategy("switch_provider") {
		t.Fatalf("expected switch_provider retry strategy, got %s", task.RetryStrategy)
	}
	if got := task.Input["retry_strategy"]; got != "switch_provider" {
		t.Fatalf("expected retry_strategy input switch_provider, got %#v", got)
	}
	if len(am.taskQueue) != 1 {
		t.Fatalf("expected retried task to be requeued, queue len=%d", len(am.taskQueue))
	}
	if build.SnapshotState.Orchestration == nil || len(build.SnapshotState.Orchestration.FailureFingerprints) != 2 {
		t.Fatalf("expected current verification failure to be recorded, got %+v", build.SnapshotState.Orchestration)
	}
}

func newVerificationRetryTestFixture(history []FailureFingerprint) (*AgentManager, *Build, *Agent, *Task) {
	task := &Task{
		ID:          "task-verify-truncation",
		Type:        TaskGenerateUI,
		Description: "Generate the landing page",
		Status:      TaskInProgress,
		MaxRetries:  2,
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				ID:        "wo-verify-truncation",
				Role:      RoleFrontend,
				TaskShape: TaskShapeFrontendPatch,
			},
		},
		CreatedAt: time.Now(),
	}
	build := &Build{
		ID:           "build-verify-truncation",
		Status:       BuildInProgress,
		Mode:         ModeFast,
		PowerMode:    PowerFast,
		ProviderMode: "platform",
		Tasks:        []*Task{task},
		Agents:       map[string]*Agent{},
		Plan: &BuildPlan{
			AppType:      "web",
			DeliveryMode: "frontend_preview_only",
			TechStack: TechStack{
				Frontend: "React",
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags:               defaultBuildOrchestrationFlags(),
				FailureFingerprints: append([]FailureFingerprint(nil), history...),
			},
		},
	}
	agent := &Agent{
		ID:          "agent-verify-truncation",
		Role:        RoleFrontend,
		Provider:    ai.ProviderGPT4,
		Model:       selectModelForPowerMode(ai.ProviderGPT4, PowerFast),
		BuildID:     build.ID,
		Status:      StatusWorking,
		CurrentTask: task,
	}
	task.AssignedTo = agent.ID
	now := time.Now()
	task.StartedAt = &now
	build.Agents[agent.ID] = agent

	am := &AgentManager{
		ctx: context.Background(),
		aiRouter: &stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderGPT4, ai.ProviderClaude},
			hasConfiguredProvider: true,
		},
		agents:      map[string]*Agent{agent.ID: agent},
		builds:      map[string]*Build{build.ID: build},
		taskQueue:   make(chan *Task, 2),
		resultQueue: make(chan *TaskResult, 1),
		subscribers: map[string][]chan *WSMessage{},
	}

	return am, build, agent, task
}

func truncatedOpeningHoursOutput() *TaskOutput {
	return &TaskOutput{
		Files: []GeneratedFile{
			{
				Path:     "src/components/OpeningHours.tsx",
				Language: "typescript",
				Content:  "export default function OpeningHours() {\n  return (\n    <section className=\"hours\">\n      <h2>Opening Hours</h2>\n",
			},
		},
	}
}
