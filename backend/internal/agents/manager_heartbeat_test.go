package agents

import (
	"context"
	"testing"
	"time"

	"apex-build/internal/ai"
)

func TestAgentActivityHeartbeatAppendsTimelineUpdates(t *testing.T) {
	originalInterval := agentActivityHeartbeatInterval
	agentActivityHeartbeatInterval = 10 * time.Millisecond
	defer func() {
		agentActivityHeartbeatInterval = originalInterval
	}()

	am := &AgentManager{
		builds:      map[string]*Build{},
		subscribers: map[string][]chan *WSMessage{},
	}

	build := &Build{
		ID:     "build-heartbeat",
		Status: BuildInProgress,
		Agents: map[string]*Agent{},
	}
	agent := &Agent{
		ID:       "agent-heartbeat",
		Role:     RoleBackend,
		Provider: ai.ProviderGrok,
		Model:    "grok-4.20-0309-reasoning",
		BuildID:  build.ID,
		Status:   StatusWorking,
	}
	task := &Task{
		ID:          "task-heartbeat",
		Type:        TaskGenerateAPI,
		Description: "Generate API contract",
	}
	build.Agents[agent.ID] = agent
	am.builds[build.ID] = build

	ctx, cancel := context.WithCancel(context.Background())
	stopHeartbeat := am.startAgentActivityHeartbeat(ctx, build.ID, agent, task, "agent:generating", "generation", agent.Provider, agent.Model)

	time.Sleep(35 * time.Millisecond)
	stopHeartbeat()
	cancel()
	time.Sleep(5 * time.Millisecond)

	build.mu.RLock()
	defer build.mu.RUnlock()

	if len(build.ActivityTimeline) == 0 {
		t.Fatal("expected heartbeat activity to be appended")
	}
	entry := build.ActivityTimeline[len(build.ActivityTimeline)-1]
	if entry.EventType != "agent:generating" {
		t.Fatalf("event type = %q, want agent:generating", entry.EventType)
	}
	if entry.Provider != string(ai.ProviderGrok) {
		t.Fatalf("provider = %q, want %q", entry.Provider, ai.ProviderGrok)
	}
	if entry.Model != "grok-4.20-0309-reasoning" {
		t.Fatalf("model = %q, want grok-4.20-0309-reasoning", entry.Model)
	}
	if entry.TaskID != task.ID {
		t.Fatalf("task id = %q, want %q", entry.TaskID, task.ID)
	}
	if entry.Content == "" {
		t.Fatal("expected heartbeat content")
	}
}
