package agents

import "testing"

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
