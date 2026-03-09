package autonomous

import "testing"

func TestResumeTaskRestoresPausedFromState(t *testing.T) {
	agent := &AutonomousAgent{
		tasks:       make(map[string]*AutonomousTask),
		subscribers: make(map[string][]chan *WSUpdate),
	}

	task := &AutonomousTask{
		ID:    "task-1",
		State: StateValidating,
	}
	agent.tasks[task.ID] = task

	if err := agent.PauseTask(task.ID); err != nil {
		t.Fatalf("PauseTask returned error: %v", err)
	}
	if task.State != StatePaused {
		t.Fatalf("expected paused state, got %s", task.State)
	}
	if task.PausedFrom != StateValidating {
		t.Fatalf("expected paused-from state to be validating, got %s", task.PausedFrom)
	}

	if err := agent.ResumeTask(task.ID); err != nil {
		t.Fatalf("ResumeTask returned error: %v", err)
	}
	if task.State != StateValidating {
		t.Fatalf("expected resume to restore validating, got %s", task.State)
	}
	if task.PausedFrom != "" {
		t.Fatalf("expected paused-from state to clear after resume, got %s", task.PausedFrom)
	}
}
