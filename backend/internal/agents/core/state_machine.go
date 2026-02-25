// Package core provides the Agent Finite State Machine (FSM) engine for APEX.BUILD.
//
// The FSM governs agent lifecycle: from initialization through planning, execution,
// validation, and completion — with full rollback support via Postgres-backed checkpoints.
//
// PUBLIC CONTRACT FOR CODEX 5.3 INTEGRATION:
//
//	AgentFSM interface — call NewAgentFSM() to instantiate.
//	Transition(event) to advance state.
//	CurrentState() to inspect.
//	CreateCheckpoint() / RollbackTo(checkpointID) for recovery.
//	Subscribe(chan StateTransition) for real-time WebSocket bridging.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// --- State & Event Enums ---

// AgentState represents the discrete states of the agent FSM.
type AgentState string

const (
	StateIdle         AgentState = "idle"
	StateInitializing AgentState = "initializing"
	StatePlanning     AgentState = "planning"
	StateExecuting    AgentState = "executing"
	StateValidating   AgentState = "validating"
	StateRetrying     AgentState = "retrying"
	StateRollingBack  AgentState = "rolling_back"
	StatePaused       AgentState = "paused"
	StateCompleted    AgentState = "completed"
	StateFailed       AgentState = "failed"
	StateCancelled    AgentState = "cancelled"
)

// AgentEvent represents events that trigger state transitions.
type AgentEvent string

const (
	EventStart            AgentEvent = "start"
	EventInitialized      AgentEvent = "initialized"
	EventPlanReady        AgentEvent = "plan_ready"
	EventStepComplete     AgentEvent = "step_complete"
	EventAllStepsComplete AgentEvent = "all_steps_complete"
	EventValidationPass   AgentEvent = "validation_pass"
	EventValidationFail   AgentEvent = "validation_fail"
	EventRetryExhausted   AgentEvent = "retry_exhausted"
	EventRollbackComplete AgentEvent = "rollback_complete"
	EventRollbackFailed   AgentEvent = "rollback_failed"
	EventPause            AgentEvent = "pause"
	EventResume           AgentEvent = "resume"
	EventCancel           AgentEvent = "cancel"
	EventFatalError       AgentEvent = "fatal_error"
)

// --- Transition Table ---

// transition defines a valid (from, event) → to mapping.
type transition struct {
	From  AgentState
	Event AgentEvent
	To    AgentState
}

// validTransitions is the canonical FSM transition table.
var validTransitions = []transition{
	// Startup path
	{StateIdle, EventStart, StateInitializing},
	{StateInitializing, EventInitialized, StatePlanning},
	{StatePlanning, EventPlanReady, StateExecuting},

	// Execution → validation
	{StateExecuting, EventStepComplete, StateExecuting},     // intermediate steps
	{StateExecuting, EventAllStepsComplete, StateValidating}, // final step done

	// Validation outcomes
	{StateValidating, EventValidationPass, StateCompleted},
	{StateValidating, EventValidationFail, StateRetrying},

	// Retry loop
	{StateRetrying, EventStepComplete, StateValidating},
	{StateRetrying, EventRetryExhausted, StateRollingBack},

	// Rollback outcomes
	{StateRollingBack, EventRollbackComplete, StateFailed},
	{StateRollingBack, EventRollbackFailed, StateFailed},

	// Pause/resume
	{StateExecuting, EventPause, StatePaused},
	{StateValidating, EventPause, StatePaused},
	{StatePaused, EventResume, StateExecuting},

	// Cancel — allowed from many states
	{StateInitializing, EventCancel, StateCancelled},
	{StatePlanning, EventCancel, StateCancelled},
	{StateExecuting, EventCancel, StateCancelled},
	{StateValidating, EventCancel, StateCancelled},
	{StateRetrying, EventCancel, StateCancelled},
	{StatePaused, EventCancel, StateCancelled},

	// Fatal error — allowed from non-terminal states
	{StateInitializing, EventFatalError, StateFailed},
	{StatePlanning, EventFatalError, StateFailed},
	{StateExecuting, EventFatalError, StateRollingBack},
	{StateValidating, EventFatalError, StateRollingBack},
	{StateRetrying, EventFatalError, StateRollingBack},
}

// --- State Transition Record ---

// StateTransition is emitted on every state change. Subscribe to receive these
// for WebSocket bridging or audit logging.
type StateTransition struct {
	ID            string     `json:"id"`
	BuildID       string     `json:"build_id"`
	FromState     AgentState `json:"from_state"`
	ToState       AgentState `json:"to_state"`
	Event         AgentEvent `json:"event"`
	Timestamp     time.Time  `json:"timestamp"`
	RetryCount    int        `json:"retry_count"`
	ErrorMessage  string     `json:"error_message,omitempty"`
	CheckpointID  string     `json:"checkpoint_id,omitempty"`
	StepID        string     `json:"step_id,omitempty"`
	DurationMs    int64      `json:"duration_ms"`
	Metadata      string     `json:"metadata,omitempty"` // JSON blob
}

// --- Checkpoint ---

// Checkpoint represents a Postgres-backed recovery point.
type Checkpoint struct {
	ID          string     `json:"id"`
	BuildID     string     `json:"build_id"`
	State       AgentState `json:"state"`
	StepIndex   int        `json:"step_index"`
	CreatedAt   time.Time  `json:"created_at"`
	Description string     `json:"description"`
	SnapshotJSON string    `json:"snapshot_json"` // serialized agent context
	CanRestore  bool       `json:"can_restore"`
}

// --- CheckpointStore Interface ---
// Codex 5.3 implements this with real Postgres; we define the contract.

// CheckpointStore abstracts persistence for checkpoints.
type CheckpointStore interface {
	SaveCheckpoint(ctx context.Context, cp *Checkpoint) error
	GetCheckpoint(ctx context.Context, id string) (*Checkpoint, error)
	ListCheckpoints(ctx context.Context, buildID string) ([]*Checkpoint, error)
	DeleteCheckpoint(ctx context.Context, id string) error
}

// --- Agent FSM ---

// AgentFSM is the core finite state machine governing an agent build.
type AgentFSM struct {
	mu sync.RWMutex

	BuildID     string
	state       AgentState
	retryCount  int
	maxRetries  int
	stepIndex   int
	totalSteps  int
	startTime   time.Time
	lastTransAt time.Time
	errorMsg    string

	// Checkpoint management
	checkpoints     []*Checkpoint
	checkpointStore CheckpointStore // nil = in-memory only

	// Event subscribers (for WebSocket bridging)
	subscribers []chan StateTransition

	// Transition history for audit
	history []StateTransition
}

// AgentFSMConfig provides initialization parameters.
type AgentFSMConfig struct {
	BuildID         string
	MaxRetries      int
	TotalSteps      int
	CheckpointStore CheckpointStore // optional — nil uses in-memory
}

// NewAgentFSM creates and returns a new FSM in the Idle state.
// This is the primary public constructor — Codex 5.3 calls this.
func NewAgentFSM(cfg AgentFSMConfig) *AgentFSM {
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &AgentFSM{
		BuildID:         cfg.BuildID,
		state:           StateIdle,
		maxRetries:      maxRetries,
		totalSteps:      cfg.TotalSteps,
		startTime:       time.Now(),
		lastTransAt:     time.Now(),
		checkpointStore: cfg.CheckpointStore,
		history:         make([]StateTransition, 0, 64),
	}
}

// CurrentState returns the current FSM state (thread-safe).
func (fsm *AgentFSM) CurrentState() AgentState {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	return fsm.state
}

// RetryCount returns how many retries have been attempted.
func (fsm *AgentFSM) RetryCount() int {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	return fsm.retryCount
}

// StepIndex returns the current step index (0-based).
func (fsm *AgentFSM) StepIndex() int {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	return fsm.stepIndex
}

// IsTerminal returns true if the FSM is in a terminal state.
func (fsm *AgentFSM) IsTerminal() bool {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	return fsm.state == StateCompleted || fsm.state == StateFailed || fsm.state == StateCancelled
}

// ElapsedMs returns milliseconds since the FSM was created.
func (fsm *AgentFSM) ElapsedMs() int64 {
	return time.Since(fsm.startTime).Milliseconds()
}

// --- Core Transition ---

// Transition attempts to move the FSM from its current state via the given event.
// Returns an error if the transition is invalid.
// On success, emits a StateTransition to all subscribers.
func (fsm *AgentFSM) Transition(event AgentEvent) error {
	return fsm.TransitionWithMeta(event, "")
}

// TransitionWithMeta is like Transition but attaches metadata JSON to the record.
func (fsm *AgentFSM) TransitionWithMeta(event AgentEvent, metadata string) error {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()

	fromState := fsm.state

	// Find valid transition
	var targetState AgentState
	found := false
	for _, t := range validTransitions {
		if t.From == fromState && t.Event == event {
			targetState = t.To
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("invalid transition: state=%s event=%s", fromState, event)
	}

	now := time.Now()
	duration := now.Sub(fsm.lastTransAt).Milliseconds()

	// Apply state-specific side effects
	switch event {
	case EventStepComplete:
		fsm.stepIndex++
	case EventValidationFail:
		fsm.retryCount++
		if fsm.retryCount > fsm.maxRetries {
			// Override: auto-escalate to rollback
			targetState = StateRollingBack
			event = EventRetryExhausted
		}
	case EventFatalError:
		fsm.errorMsg = metadata
	}

	// Record transition
	record := StateTransition{
		ID:           uuid.New().String(),
		BuildID:      fsm.BuildID,
		FromState:    fromState,
		ToState:      targetState,
		Event:        event,
		Timestamp:    now,
		RetryCount:   fsm.retryCount,
		ErrorMessage: fsm.errorMsg,
		StepID:       fmt.Sprintf("step-%d", fsm.stepIndex),
		DurationMs:   duration,
		Metadata:     metadata,
	}

	fsm.state = targetState
	fsm.lastTransAt = now
	fsm.history = append(fsm.history, record)

	// Notify subscribers (non-blocking)
	for _, ch := range fsm.subscribers {
		select {
		case ch <- record:
		default:
			// Drop if subscriber is slow — they can replay from history
		}
	}

	log.Printf("[AgentFSM %s] %s --%s--> %s (step=%d retry=%d elapsed=%dms)",
		fsm.BuildID, fromState, event, targetState, fsm.stepIndex, fsm.retryCount, duration)

	return nil
}

// --- Checkpoint Operations ---

// CreateCheckpoint saves the current FSM state as a checkpoint.
// Returns the checkpoint ID for later rollback.
func (fsm *AgentFSM) CreateCheckpoint(ctx context.Context, description string, snapshotJSON string) (string, error) {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()

	cp := &Checkpoint{
		ID:           uuid.New().String(),
		BuildID:      fsm.BuildID,
		State:        fsm.state,
		StepIndex:    fsm.stepIndex,
		CreatedAt:    time.Now(),
		Description:  description,
		SnapshotJSON: snapshotJSON,
		CanRestore:   true,
	}

	// Persist if store is available
	if fsm.checkpointStore != nil {
		if err := fsm.checkpointStore.SaveCheckpoint(ctx, cp); err != nil {
			return "", fmt.Errorf("failed to persist checkpoint: %w", err)
		}
	}

	fsm.checkpoints = append(fsm.checkpoints, cp)

	log.Printf("[AgentFSM %s] Checkpoint created: %s (%s) at step %d",
		fsm.BuildID, cp.ID, description, cp.StepIndex)

	// Emit checkpoint event to subscribers
	record := StateTransition{
		ID:           uuid.New().String(),
		BuildID:      fsm.BuildID,
		FromState:    fsm.state,
		ToState:      fsm.state,
		Event:        "checkpoint_created",
		Timestamp:    time.Now(),
		CheckpointID: cp.ID,
		StepID:       fmt.Sprintf("step-%d", fsm.stepIndex),
	}
	for _, ch := range fsm.subscribers {
		select {
		case ch <- record:
		default:
		}
	}

	return cp.ID, nil
}

// RollbackTo restores the FSM to a previous checkpoint.
// This sets the state and step index back but does NOT undo file changes
// (that's the SandboxManager's job on the Codex 5.3 side).
func (fsm *AgentFSM) RollbackTo(ctx context.Context, checkpointID string) (*Checkpoint, error) {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()

	// Find the checkpoint
	var target *Checkpoint
	for _, cp := range fsm.checkpoints {
		if cp.ID == checkpointID {
			target = cp
			break
		}
	}

	// Try the persistent store if not in memory
	if target == nil && fsm.checkpointStore != nil {
		var err error
		target, err = fsm.checkpointStore.GetCheckpoint(ctx, checkpointID)
		if err != nil {
			return nil, fmt.Errorf("checkpoint not found in store: %w", err)
		}
	}

	if target == nil {
		return nil, fmt.Errorf("checkpoint %s not found", checkpointID)
	}

	if !target.CanRestore {
		return nil, fmt.Errorf("checkpoint %s is marked as non-restorable", checkpointID)
	}

	prevState := fsm.state
	fsm.state = target.State
	fsm.stepIndex = target.StepIndex
	fsm.retryCount = 0 // reset retries on rollback
	fsm.errorMsg = ""

	log.Printf("[AgentFSM %s] Rolled back from %s to checkpoint %s (state=%s step=%d)",
		fsm.BuildID, prevState, checkpointID, target.State, target.StepIndex)

	// Emit rollback event
	record := StateTransition{
		ID:           uuid.New().String(),
		BuildID:      fsm.BuildID,
		FromState:    prevState,
		ToState:      target.State,
		Event:        "rollback",
		Timestamp:    time.Now(),
		CheckpointID: checkpointID,
		StepID:       fmt.Sprintf("step-%d", target.StepIndex),
	}
	for _, ch := range fsm.subscribers {
		select {
		case ch <- record:
		default:
		}
	}

	return target, nil
}

// ListCheckpoints returns all checkpoints for this build.
func (fsm *AgentFSM) ListCheckpoints() []*Checkpoint {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	result := make([]*Checkpoint, len(fsm.checkpoints))
	copy(result, fsm.checkpoints)
	return result
}

// --- Subscription ---

// Subscribe returns a channel that receives StateTransition records.
// Buffer size controls how many transitions can queue before dropping.
func (fsm *AgentFSM) Subscribe(bufferSize int) chan StateTransition {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	ch := make(chan StateTransition, bufferSize)
	fsm.mu.Lock()
	fsm.subscribers = append(fsm.subscribers, ch)
	fsm.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel.
func (fsm *AgentFSM) Unsubscribe(ch chan StateTransition) {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()
	for i, sub := range fsm.subscribers {
		if sub == ch {
			fsm.subscribers = append(fsm.subscribers[:i], fsm.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// --- History / Serialization ---

// History returns a copy of all state transitions.
func (fsm *AgentFSM) History() []StateTransition {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	result := make([]StateTransition, len(fsm.history))
	copy(result, fsm.history)
	return result
}

// Snapshot returns a JSON-serializable snapshot of the FSM state.
func (fsm *AgentFSM) Snapshot() (string, error) {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	snap := map[string]interface{}{
		"build_id":    fsm.BuildID,
		"state":       fsm.state,
		"step_index":  fsm.stepIndex,
		"total_steps": fsm.totalSteps,
		"retry_count": fsm.retryCount,
		"max_retries": fsm.maxRetries,
		"elapsed_ms":  time.Since(fsm.startTime).Milliseconds(),
		"error":       fsm.errorMsg,
		"checkpoints": len(fsm.checkpoints),
		"transitions": len(fsm.history),
	}

	data, err := json.Marshal(snap)
	if err != nil {
		return "", fmt.Errorf("failed to serialize FSM snapshot: %w", err)
	}
	return string(data), nil
}

// Progress returns a float 0.0–1.0 representing completion.
func (fsm *AgentFSM) Progress() float64 {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	if fsm.totalSteps <= 0 {
		return 0
	}
	p := float64(fsm.stepIndex) / float64(fsm.totalSteps)
	if p > 1.0 {
		p = 1.0
	}
	return p
}
