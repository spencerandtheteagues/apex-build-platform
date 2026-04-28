package agents

import (
	"context"
	"sync"
	"testing"
	"time"

	"apex-build/internal/ai"
)

type deadlineProbeRouter struct {
	mu          sync.Mutex
	deadline    time.Time
	hasDeadline bool
}

func (r *deadlineProbeRouter) Generate(ctx context.Context, _ ai.AIProvider, _ string, _ GenerateOptions) (*ai.AIResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deadline, r.hasDeadline = ctx.Deadline()
	return &ai.AIResponse{
		Content: `{"reply":"On it","apply_changes":false}`,
	}, nil
}

func (r *deadlineProbeRouter) GetAvailableProviders() []ai.AIProvider {
	return []ai.AIProvider{ai.ProviderOllama}
}
func (r *deadlineProbeRouter) GetAvailableProvidersForUser(_ uint) []ai.AIProvider {
	return []ai.AIProvider{ai.ProviderOllama}
}
func (r *deadlineProbeRouter) HasConfiguredProviders() bool { return true }

type blockingInterventionRouter struct {
	startedOnce sync.Once
	started     chan struct{}
	release     chan struct{}
}

func (r *blockingInterventionRouter) Generate(ctx context.Context, _ ai.AIProvider, _ string, _ GenerateOptions) (*ai.AIResponse, error) {
	r.startedOnce.Do(func() {
		close(r.started)
	})
	select {
	case <-r.release:
		return &ai.AIResponse{
			Content: `{"reply":"Continuing the build","apply_changes":false}`,
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *blockingInterventionRouter) GetAvailableProviders() []ai.AIProvider {
	return []ai.AIProvider{ai.ProviderOllama}
}
func (r *blockingInterventionRouter) GetAvailableProvidersForUser(_ uint) []ai.AIProvider {
	return []ai.AIProvider{ai.ProviderOllama}
}
func (r *blockingInterventionRouter) HasConfiguredProviders() bool { return true }

func TestProcessUserMessageUsesProviderAwareTimeout(t *testing.T) {
	t.Parallel()

	router := &deadlineProbeRouter{}
	manager := newTestIterationManager(router)

	build := &Build{
		ID:          "intervention-timeout-build",
		UserID:      1,
		Status:      BuildInProgress,
		Mode:        ModeFull,
		PowerMode:   PowerBalanced,
		Description: "Build a CRM app",
		Agents:      make(map[string]*Agent),
		Tasks:       []*Task{},
		MaxRequests: 5,
		CreatedAt:   time.Now().Add(-time.Minute).UTC(),
		UpdatedAt:   time.Now().Add(-time.Second).UTC(),
		Interaction: BuildInteractionState{},
	}
	lead := &Agent{
		ID:        "lead-ollama",
		Role:      RoleLead,
		Provider:  ai.ProviderOllama,
		Model:     "deepseek-r1:14b",
		Status:    StatusWorking,
		BuildID:   build.ID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	build.Agents[lead.ID] = lead
	manager.builds[build.ID] = build
	manager.agents[lead.ID] = lead

	start := time.Now()
	manager.processUserMessage(lead, "Tighten the dashboard spacing")

	router.mu.Lock()
	deadline := router.deadline
	hasDeadline := router.hasDeadline
	router.mu.Unlock()

	if !hasDeadline {
		t.Fatal("expected processUserMessage to set a deadline")
	}

	got := deadline.Sub(start)
	want := defaultGenerateTimeout(ai.ProviderOllama, PowerBalanced)
	if got < want-5*time.Second || got > want+5*time.Second {
		t.Fatalf("expected intervention timeout near %v, got %v", want, got)
	}
}

func TestProcessUserMessageEmitsHeartbeatWhileProviderPending(t *testing.T) {
	originalInterval := agentActivityHeartbeatInterval
	agentActivityHeartbeatInterval = 10 * time.Millisecond
	defer func() {
		agentActivityHeartbeatInterval = originalInterval
	}()

	router := &blockingInterventionRouter{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	manager := newTestIterationManager(router)

	originalUpdatedAt := time.Now().Add(-time.Minute).UTC()
	build := &Build{
		ID:          "intervention-heartbeat-build",
		UserID:      1,
		Status:      BuildInProgress,
		Mode:        ModeFull,
		PowerMode:   PowerMax,
		Description: "Build a contractor SaaS app",
		Agents:      make(map[string]*Agent),
		Tasks:       []*Task{},
		MaxRequests: 5,
		CreatedAt:   time.Now().Add(-time.Minute).UTC(),
		UpdatedAt:   originalUpdatedAt,
		Interaction: BuildInteractionState{},
	}
	lead := &Agent{
		ID:        "lead-ollama-heartbeat",
		Role:      RoleLead,
		Provider:  ai.ProviderOllama,
		Model:     "kimi-k2.6",
		Status:    StatusWorking,
		BuildID:   build.ID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	build.Agents[lead.ID] = lead
	manager.builds[build.ID] = build
	manager.agents[lead.ID] = lead

	done := make(chan struct{})
	go func() {
		manager.processUserMessage(lead, "finish planning and continue the build")
		close(done)
	}()

	select {
	case <-router.started:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected provider call to start")
	}

	deadline := time.After(750 * time.Millisecond)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			close(router.release)
			t.Fatal("expected intervention heartbeat while provider call was pending")
		case <-ticker.C:
			build.mu.RLock()
			updated := build.UpdatedAt
			var found bool
			for _, entry := range build.ActivityTimeline {
				if entry.EventType == string(WSAgentWorking) && entry.Provider == string(ai.ProviderOllama) && entry.Model == "kimi-k2.6" {
					found = true
					break
				}
			}
			build.mu.RUnlock()
			if found {
				if !updated.After(originalUpdatedAt) {
					close(router.release)
					t.Fatal("expected heartbeat to refresh build updated_at")
				}
				close(router.release)
				select {
				case <-done:
				case <-time.After(250 * time.Millisecond):
					t.Fatal("expected intervention handler to finish after releasing provider call")
				}
				return
			}
		}
	}
}
