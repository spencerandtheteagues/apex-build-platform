package agents

import (
	"testing"
	"time"

	"apex-build/internal/ai"
)

func TestSetProviderModelOverrideUpdatesProviderAgents(t *testing.T) {
	now := time.Now().UTC()
	agent := &Agent{
		ID:        "agent-grok",
		Provider:  ai.ProviderGrok,
		Model:     selectModelForPowerMode(ai.ProviderGrok, PowerMax),
		BuildID:   "build-provider-model",
		CreatedAt: now,
		UpdatedAt: now,
	}
	build := &Build{
		ID:        "build-provider-model",
		Status:    BuildInProgress,
		PowerMode: PowerMax,
		Agents: map[string]*Agent{
			agent.ID: agent,
		},
	}
	manager := &AgentManager{
		builds:      map[string]*Build{build.ID: build},
		subscribers: map[string][]chan *WSMessage{},
	}

	if err := manager.SetProviderModelOverride(build.ID, ai.ProviderGrok, "grok-3"); err != nil {
		t.Fatalf("SetProviderModelOverride returned error: %v", err)
	}
	if got := build.ProviderModelOverrides["grok"]; got != "grok-3" {
		t.Fatalf("expected persisted grok override, got %q", got)
	}
	if agent.Model != "grok-3" {
		t.Fatalf("expected live grok agent to update model, got %q", agent.Model)
	}

	if err := manager.SetProviderModelOverride(build.ID, ai.ProviderGrok, "auto"); err != nil {
		t.Fatalf("SetProviderModelOverride(auto) returned error: %v", err)
	}
	if _, exists := build.ProviderModelOverrides["grok"]; exists {
		t.Fatalf("expected auto to clear grok override, got %+v", build.ProviderModelOverrides)
	}
	if agent.Model != "grok-4.20-0309-reasoning" {
		t.Fatalf("expected grok model to reset to power-mode default, got %q", agent.Model)
	}
}
