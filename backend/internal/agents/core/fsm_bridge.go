// Package core — fsm_bridge.go
//
// Bridges AgentFSM state transitions to the WebSocket hub for real-time
// frontend updates. This is the integration layer between the FSM engine
// and the existing agents.WSHub broadcast system.
//
// Usage:
//
//	bridge := core.NewFSMBridge(fsm, broadcastFn)
//	defer bridge.Stop()
//
// The bridge subscribes to FSM events and converts them to WSMessage-compatible
// data payloads that the existing WSHub.Broadcast can send to connected clients.
package core

import (
	"log"
	"time"
)

// BroadcastFunc is the signature for sending messages to WebSocket clients.
// This matches the pattern: func(buildID string, msgType string, data map[string]any)
// The agents package wires this to WSHub.Broadcast.
type BroadcastFunc func(buildID string, msgType string, data map[string]any)

// FSMBridge forwards FSM state transitions to a broadcast function.
type FSMBridge struct {
	fsm       *AgentFSM
	broadcast BroadcastFunc
	sub       chan StateTransition
	done      chan struct{}
}

// NewFSMBridge creates and starts a bridge that forwards FSM events.
func NewFSMBridge(fsm *AgentFSM, broadcastFn BroadcastFunc) *FSMBridge {
	b := &FSMBridge{
		fsm:       fsm,
		broadcast: broadcastFn,
		sub:       fsm.Subscribe(128),
		done:      make(chan struct{}),
	}
	go b.run()
	return b
}

// Stop unsubscribes from the FSM and shuts down the bridge goroutine.
func (b *FSMBridge) Stop() {
	b.fsm.Unsubscribe(b.sub)
	close(b.done)
}

func (b *FSMBridge) run() {
	for {
		select {
		case <-b.done:
			return
		case trans, ok := <-b.sub:
			if !ok {
				return
			}
			b.forward(trans)
		}
	}
}

func (b *FSMBridge) forward(t StateTransition) {
	// Determine the WS message type based on the event
	msgType := mapEventToWSType(t.Event)

	data := map[string]any{
		"transition_id": t.ID,
		"from_state":    string(t.FromState),
		"to_state":      string(t.ToState),
		"event":         string(t.Event),
		"retry_count":   t.RetryCount,
		"step_id":       t.StepID,
		"duration_ms":   t.DurationMs,
		"timestamp":     t.Timestamp.Format(time.RFC3339Nano),
	}

	if t.ErrorMessage != "" {
		data["error"] = t.ErrorMessage
	}
	if t.CheckpointID != "" {
		data["checkpoint_id"] = t.CheckpointID
	}
	if t.Metadata != "" {
		data["metadata"] = t.Metadata
	}

	// Add FSM progress snapshot
	data["progress"] = b.fsm.Progress()
	data["fsm_state"] = string(b.fsm.CurrentState())
	data["elapsed_ms"] = b.fsm.ElapsedMs()

	b.broadcast(t.BuildID, msgType, data)

	log.Printf("[FSMBridge %s] forwarded %s (%s -> %s)", t.BuildID, t.Event, t.FromState, t.ToState)
}

// mapEventToWSType converts FSM events to WebSocket message types that the
// frontend can handle. These extend the existing WS message type namespace.
func mapEventToWSType(event AgentEvent) string {
	switch event {
	case EventStart:
		return "build:fsm:started"
	case EventInitialized:
		return "build:fsm:initialized"
	case EventPlanReady:
		return "build:fsm:plan_ready"
	case EventStepComplete:
		return "build:fsm:step_complete"
	case EventAllStepsComplete:
		return "build:fsm:all_steps_complete"
	case EventValidationPass:
		return "build:fsm:validation_pass"
	case EventValidationFail:
		return "build:fsm:validation_fail"
	case EventRetryExhausted:
		return "build:fsm:retry_exhausted"
	case EventRollbackComplete:
		return "build:fsm:rollback_complete"
	case EventRollbackFailed:
		return "build:fsm:rollback_failed"
	case EventPause:
		return "build:fsm:paused"
	case EventResume:
		return "build:fsm:resumed"
	case EventCancel:
		return "build:fsm:cancelled"
	case EventFatalError:
		return "build:fsm:fatal_error"
	default:
		// Handles synthetic events like "checkpoint_created" and "rollback"
		return "build:fsm:" + string(event)
	}
}
