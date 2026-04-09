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
	if am.resultQueue == nil {
		t.Fatal("expected result queue to be initialized")
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
