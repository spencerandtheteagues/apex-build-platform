package core

import (
	"context"
	"testing"
	"time"
)

// --- FSM basic transitions ---

func TestNewAgentFSM_StartsIdle(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "build-1"})
	if got := fsm.CurrentState(); got != StateIdle {
		t.Fatalf("initial state = %s, want %s", got, StateIdle)
	}
}

func TestTransition_ValidPath_IdleToCompleted(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "b1", TotalSteps: 3})

	steps := []AgentEvent{
		EventStart,
		EventInitialized,
		EventPlanReady,
		EventStepComplete,
		EventStepComplete,
		EventStepComplete,
		EventAllStepsComplete,
		EventValidationPass,
	}
	for _, ev := range steps {
		if err := fsm.Transition(ev); err != nil {
			t.Fatalf("Transition(%s): %v", ev, err)
		}
	}
	if got := fsm.CurrentState(); got != StateCompleted {
		t.Fatalf("final state = %s, want completed", got)
	}
}

func TestTransition_InvalidEvent_ReturnsError(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "b2"})
	// Can't go from Idle directly to Executing
	err := fsm.Transition(EventAllStepsComplete)
	if err == nil {
		t.Fatal("expected error for invalid transition idle→all_steps_complete, got nil")
	}
}

func TestTransition_RetryCountIncrements(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "b3", MaxRetries: 5})
	_ = fsm.Transition(EventStart)
	_ = fsm.Transition(EventInitialized)
	_ = fsm.Transition(EventPlanReady)
	_ = fsm.Transition(EventAllStepsComplete)
	_ = fsm.Transition(EventValidationFail)

	if got := fsm.RetryCount(); got != 1 {
		t.Fatalf("retry count = %d, want 1", got)
	}
}

func TestTransition_ValidationFail_ExhaustRetries_EscalatesToRollback(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "b4", MaxRetries: 2})
	_ = fsm.Transition(EventStart)
	_ = fsm.Transition(EventInitialized)
	_ = fsm.Transition(EventPlanReady)
	_ = fsm.Transition(EventAllStepsComplete)

	// First failure moves to retrying
	_ = fsm.Transition(EventValidationFail)
	if fsm.CurrentState() != StateRetrying {
		t.Fatalf("after 1st fail, want retrying, got %s", fsm.CurrentState())
	}

	// From retrying, re-validate
	_ = fsm.Transition(EventStepComplete)

	// Second failure exhausts budget (maxRetries=2)
	_ = fsm.Transition(EventValidationFail)
	if got := fsm.CurrentState(); got != StateRollingBack {
		t.Fatalf("after exhausting retries, want rolling_back, got %s", got)
	}
}

func TestTransition_PauseAndResume(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "b5"})
	_ = fsm.Transition(EventStart)
	_ = fsm.Transition(EventInitialized)
	_ = fsm.Transition(EventPlanReady)

	if err := fsm.Transition(EventPause); err != nil {
		t.Fatalf("pause from executing: %v", err)
	}
	if fsm.CurrentState() != StatePaused {
		t.Fatalf("want paused, got %s", fsm.CurrentState())
	}

	if err := fsm.Transition(EventResume); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if fsm.CurrentState() != StateExecuting {
		t.Fatalf("want executing after resume, got %s", fsm.CurrentState())
	}
}

func TestTransition_Cancel(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "b6"})
	_ = fsm.Transition(EventStart)
	_ = fsm.Transition(EventInitialized)
	_ = fsm.Transition(EventPlanReady)

	if err := fsm.Transition(EventCancel); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if got := fsm.CurrentState(); got != StateCancelled {
		t.Fatalf("want cancelled, got %s", got)
	}
}

func TestIsTerminal(t *testing.T) {
	cases := []struct {
		state    AgentState
		terminal bool
	}{
		{StateCompleted, true},
		{StateFailed, true},
		{StateCancelled, true},
		{StateIdle, false},
		{StateExecuting, false},
		{StateRetrying, false},
	}

	for _, tc := range cases {
		fsm := &AgentFSM{state: tc.state}
		if got := fsm.IsTerminal(); got != tc.terminal {
			t.Errorf("IsTerminal(%s) = %v, want %v", tc.state, got, tc.terminal)
		}
	}
}

func TestProgress_ReturnsCorrectFraction(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "p1", TotalSteps: 4})
	_ = fsm.Transition(EventStart)
	_ = fsm.Transition(EventInitialized)
	_ = fsm.Transition(EventPlanReady)
	_ = fsm.Transition(EventStepComplete) // step 1 of 4

	got := fsm.Progress()
	if got < 0.24 || got > 0.26 {
		t.Fatalf("progress after 1/4 steps = %f, want ~0.25", got)
	}
}

func TestProgress_CapsAtOne(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "p2", TotalSteps: 2})
	fsm.stepIndex = 10 // force beyond total
	if got := fsm.Progress(); got != 1.0 {
		t.Fatalf("progress beyond total steps = %f, want 1.0", got)
	}
}

func TestProgress_ZeroTotalSteps(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "p3", TotalSteps: 0})
	if got := fsm.Progress(); got != 0 {
		t.Fatalf("progress with no total steps = %f, want 0", got)
	}
}

// --- Checkpoint ---

func TestCreateCheckpoint_InMemory(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "cp1"})
	_ = fsm.Transition(EventStart)
	_ = fsm.Transition(EventInitialized)

	id, err := fsm.CreateCheckpoint(context.Background(), "after init", `{"phase":"init"}`)
	if err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty checkpoint ID")
	}

	cps := fsm.ListCheckpoints()
	if len(cps) != 1 {
		t.Fatalf("expected 1 checkpoint, got %d", len(cps))
	}
	if cps[0].ID != id {
		t.Fatalf("checkpoint ID mismatch")
	}
}

func TestRollbackTo_RestoresStateAndStep(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "cp2", TotalSteps: 5})
	_ = fsm.Transition(EventStart)
	_ = fsm.Transition(EventInitialized)
	_ = fsm.Transition(EventPlanReady)

	cpID, _ := fsm.CreateCheckpoint(context.Background(), "pre-execution", "")

	_ = fsm.Transition(EventStepComplete)
	_ = fsm.Transition(EventStepComplete)
	_ = fsm.Transition(EventStepComplete)

	if fsm.StepIndex() != 3 {
		t.Fatalf("step index before rollback = %d, want 3", fsm.StepIndex())
	}

	cp, err := fsm.RollbackTo(context.Background(), cpID)
	if err != nil {
		t.Fatalf("RollbackTo: %v", err)
	}
	if cp.ID != cpID {
		t.Fatal("wrong checkpoint returned")
	}
	if fsm.CurrentState() != StateExecuting {
		t.Fatalf("state after rollback = %s, want executing", fsm.CurrentState())
	}
	if fsm.StepIndex() != 0 {
		t.Fatalf("step after rollback = %d, want 0", fsm.StepIndex())
	}
	if fsm.RetryCount() != 0 {
		t.Fatalf("retry count after rollback = %d, want 0", fsm.RetryCount())
	}
}

func TestRollbackTo_UnknownCheckpoint_ReturnsError(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "cp3"})
	_, err := fsm.RollbackTo(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for unknown checkpoint, got nil")
	}
}

func TestRollbackTo_NonRestorableCheckpoint_Rejected(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "cp4"})
	_ = fsm.Transition(EventStart)

	cpID, _ := fsm.CreateCheckpoint(context.Background(), "locked", "")

	// Mark the checkpoint as non-restorable
	fsm.mu.Lock()
	for _, cp := range fsm.checkpoints {
		if cp.ID == cpID {
			cp.CanRestore = false
		}
	}
	fsm.mu.Unlock()

	_, err := fsm.RollbackTo(context.Background(), cpID)
	if err == nil {
		t.Fatal("expected error for non-restorable checkpoint, got nil")
	}
}

// --- Subscription ---

func TestSubscribe_ReceivesTransitions(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "sub1"})
	ch := fsm.Subscribe(16)

	_ = fsm.Transition(EventStart)
	_ = fsm.Transition(EventInitialized)

	var received []StateTransition
	timeout := time.After(100 * time.Millisecond)
loop:
	for {
		select {
		case t := <-ch:
			received = append(received, t)
			if len(received) >= 2 {
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	if len(received) < 2 {
		t.Fatalf("expected ≥2 transitions, got %d", len(received))
	}
	if received[0].Event != EventStart {
		t.Errorf("first event = %s, want %s", received[0].Event, EventStart)
	}
	if received[1].Event != EventInitialized {
		t.Errorf("second event = %s, want %s", received[1].Event, EventInitialized)
	}
}

func TestUnsubscribe_StopsDelivery(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "sub2"})
	ch := fsm.Subscribe(16)
	fsm.Unsubscribe(ch)

	_ = fsm.Transition(EventStart)

	// Channel should be closed, no new events
	select {
	case _, open := <-ch:
		if open {
			t.Fatal("expected channel to be closed after unsubscribe")
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatal("channel not closed after unsubscribe")
	}
}

// --- History ---

func TestHistory_RecordsAllTransitions(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "hist1"})
	_ = fsm.Transition(EventStart)
	_ = fsm.Transition(EventInitialized)
	_ = fsm.Transition(EventPlanReady)

	h := fsm.History()
	if len(h) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(h))
	}
}

// --- Snapshot ---

func TestSnapshot_ProducesValidJSON(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "snap1", TotalSteps: 5})
	_ = fsm.Transition(EventStart)

	snap, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap) == 0 {
		t.Fatal("expected non-empty snapshot JSON")
	}
}

// --- TransitionWithMeta ---

func TestTransitionWithMeta_StoresFatalErrorMsg(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "meta1"})
	_ = fsm.Transition(EventStart)
	_ = fsm.Transition(EventInitialized)
	_ = fsm.Transition(EventPlanReady)

	if err := fsm.TransitionWithMeta(EventFatalError, "disk full"); err != nil {
		t.Fatalf("TransitionWithMeta: %v", err)
	}
	if fsm.CurrentState() != StateRollingBack {
		t.Fatalf("want rolling_back after fatal error, got %s", fsm.CurrentState())
	}

	h := fsm.History()
	last := h[len(h)-1]
	if last.ErrorMessage != "disk full" {
		t.Fatalf("error message = %q, want %q", last.ErrorMessage, "disk full")
	}
}

// --- ElapsedMs ---

func TestElapsedMs_IncreaseOverTime(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "elapsed1"})
	time.Sleep(5 * time.Millisecond)
	if got := fsm.ElapsedMs(); got <= 0 {
		t.Fatalf("ElapsedMs should be >0 after sleep, got %d", got)
	}
}

// --- DefaultRetries ---

func TestNewAgentFSM_DefaultMaxRetries(t *testing.T) {
	fsm := NewAgentFSM(AgentFSMConfig{BuildID: "def1", MaxRetries: 0})
	if fsm.maxRetries != 3 {
		t.Fatalf("default max retries = %d, want 3", fsm.maxRetries)
	}
}
