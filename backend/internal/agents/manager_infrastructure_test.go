package agents

import (
	"context"
	"testing"
	"time"
)

func TestEnsureWorkerInfrastructureInitializesQueuesAndContext(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	am.ensureWorkerInfrastructure()

	if am.ctx == nil || am.cancel == nil {
		t.Fatalf("expected worker infrastructure to initialize context and cancel func, got ctx=%v cancel=%v", am.ctx, am.cancel)
	}
	if am.taskQueue == nil {
		t.Fatal("expected task queue to be initialized")
	}
	if cap(am.taskQueue) != agentTaskQueueBuffer {
		t.Fatalf("task queue capacity = %d, want %d", cap(am.taskQueue), agentTaskQueueBuffer)
	}
	if am.resultQueue == nil {
		t.Fatal("expected result queue to be initialized")
	}
	if cap(am.resultQueue) != agentResultQueueBuffer {
		t.Fatalf("result queue capacity = %d, want %d", cap(am.resultQueue), agentResultQueueBuffer)
	}
	if am.subscribers == nil {
		t.Fatal("expected subscribers map to be initialized")
	}
	if am.buildMonitors == nil {
		t.Fatal("expected buildMonitors map to be initialized")
	}
	if !am.taskDispatcherRunning {
		t.Fatal("expected task dispatcher to be running")
	}
	if !am.resultProcessorRunning {
		t.Fatal("expected result processor to be running")
	}
}

func TestEnsureWorkerInfrastructureRestartsWorkersAfterCancelledContext(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	am.ensureWorkerInfrastructure()

	originalCtx := am.ctx
	if originalCtx == nil {
		t.Fatal("expected original worker context to be initialized")
	}

	am.cancel()
	am.ensureWorkerInfrastructure()

	if am.ctx == nil || am.cancel == nil {
		t.Fatalf("expected worker infrastructure to recreate context after cancellation, got ctx=%v cancel=%v", am.ctx, am.cancel)
	}
	if am.ctx == originalCtx {
		t.Fatal("expected worker infrastructure to replace cancelled context")
	}
	if am.taskQueue == nil {
		t.Fatal("expected task queue to remain initialized after restart")
	}
	if am.resultQueue == nil {
		t.Fatal("expected result queue to remain initialized after restart")
	}
	if !am.taskDispatcherRunning {
		t.Fatal("expected task dispatcher to be restarted")
	}
	if !am.resultProcessorRunning {
		t.Fatal("expected result processor to be restarted")
	}
}

func TestEnqueueTaskQueueFailsFastWhenFull(t *testing.T) {
	t.Parallel()

	am := &AgentManager{taskQueue: make(chan *Task, 1)}
	am.taskQueue <- &Task{ID: "already-queued"}
	task := &Task{ID: "overflow", Status: TaskPending}

	if am.enqueueTaskQueue(task) {
		t.Fatal("expected enqueue to fail when task queue is full")
	}
	if task.Status != TaskFailed {
		t.Fatalf("task status = %s, want failed", task.Status)
	}
	if task.Error != "task queue saturated" {
		t.Fatalf("task error = %q, want queue saturation", task.Error)
	}
}

func TestEnqueueTaskResultFallsBackWhenResultQueueBlocked(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	now := time.Now().UTC()
	task := &Task{
		ID:         "review-completed-output",
		Type:       TaskReview,
		Status:     TaskInProgress,
		AssignedTo: "reviewer-1",
		StartedAt:  &now,
		CreatedAt:  now,
	}
	agent := &Agent{
		ID:          "reviewer-1",
		Role:        RoleReviewer,
		Status:      StatusWorking,
		BuildID:     "build-result-fallback",
		CurrentTask: task,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	build := &Build{
		ID:        "build-result-fallback",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		Agents:    map[string]*Agent{agent.ID: agent},
		Tasks:     []*Task{task},
		CreatedAt: now,
		UpdatedAt: now,
	}

	am := &AgentManager{
		ctx:                    ctx,
		cancel:                 cancel,
		agents:                 map[string]*Agent{agent.ID: agent},
		builds:                 map[string]*Build{build.ID: build},
		subscribers:            map[string][]chan *WSMessage{},
		taskQueue:              make(chan *Task, 1),
		resultQueue:            make(chan *TaskResult),
		taskDispatcherRunning:  true,
		resultProcessorRunning: true,
	}

	am.enqueueTaskResult(&TaskResult{
		TaskID:  task.ID,
		AgentID: agent.ID,
		Success: true,
		Output:  &TaskOutput{Messages: []string{"review passed"}},
	})

	deadline := time.After(4 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline:
			t.Fatalf("task status = %s agent status = %s; expected blocked result queue fallback to complete task", task.Status, agent.Status)
		case <-tick.C:
			if task.Status == TaskCompleted && agent.Status == StatusCompleted {
				if task.Output == nil || len(task.Output.Messages) != 1 || task.Output.Messages[0] != "review passed" {
					t.Fatalf("unexpected task output after fallback: %+v", task.Output)
				}
				return
			}
		}
	}
}
