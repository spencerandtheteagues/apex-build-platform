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
