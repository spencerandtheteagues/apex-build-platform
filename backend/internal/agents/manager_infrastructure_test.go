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
}
