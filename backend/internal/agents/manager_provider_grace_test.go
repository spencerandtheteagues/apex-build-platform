package agents

import (
	"context"
	"testing"
	"time"

	"apex-build/internal/ai"
)

type stubAIRouter struct {
	providers             []ai.AIProvider
	userProviders         []ai.AIProvider
	hasConfiguredProvider bool
}

func (s *stubAIRouter) Generate(context.Context, ai.AIProvider, string, GenerateOptions) (*ai.AIResponse, error) {
	return nil, nil
}

func (s *stubAIRouter) GetAvailableProviders() []ai.AIProvider {
	return append([]ai.AIProvider(nil), s.providers...)
}

func (s *stubAIRouter) GetAvailableProvidersForUser(userID uint) []ai.AIProvider {
	_ = userID
	if s.userProviders != nil {
		return append([]ai.AIProvider(nil), s.userProviders...)
	}
	return s.GetAvailableProviders()
}

func (s *stubAIRouter) HasConfiguredProviders() bool {
	return s.hasConfiguredProvider
}

func TestGetAvailableProvidersWithGracePeriod_FailsFastWhenNoneConfigured(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubAIRouter{
			providers:             nil,
			hasConfiguredProvider: false,
		},
	}

	start := time.Now()
	got := am.getAvailableProvidersWithGracePeriod()
	elapsed := time.Since(start)

	if len(got) != 0 {
		t.Fatalf("expected no providers, got %v", got)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("expected fast return without grace-period sleep, took %s", elapsed)
	}
}

func TestGetCurrentlyAvailableProvidersForBuild_UsesBYOKProviders(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubAIRouter{
			providers:     nil,
			userProviders: []ai.AIProvider{ai.ProviderOllama},
		},
	}

	build := &Build{
		UserID:              42,
		ProviderMode:        "byok",
		RequirePreviewReady: true,
	}

	got := am.getCurrentlyAvailableProvidersForBuild(build)
	if len(got) != 1 || got[0] != ai.ProviderOllama {
		t.Fatalf("expected BYOK user providers [ollama], got %v", got)
	}
}

func TestGetCurrentlyAvailableProvidersForBuild_RetainsRecentSuccessfulOllama(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4},
			hasConfiguredProvider: true,
		},
	}

	completedAt := time.Now().Add(-30 * time.Second)
	build := &Build{
		ID:           "sticky-ollama-build",
		ProviderMode: "platform",
		Agents: map[string]*Agent{
			"lead-1": {
				ID:       "lead-1",
				Role:     RoleLead,
				Provider: ai.ProviderOllama,
			},
		},
		Tasks: []*Task{
			{
				ID:          "plan-1",
				Type:        TaskPlan,
				Status:      TaskCompleted,
				AssignedTo:  "lead-1",
				CompletedAt: &completedAt,
			},
		},
	}

	got := am.getCurrentlyAvailableProvidersForBuild(build)
	if len(got) != 3 || got[0] != ai.ProviderOllama {
		t.Fatalf("expected retained providers [ollama claude gpt4], got %v", got)
	}
}

func TestGetCurrentlyAvailableProvidersForBuild_DoesNotRetainFallbackProvider(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4},
			hasConfiguredProvider: true,
		},
	}

	completedAt := time.Now().Add(-30 * time.Second)
	build := &Build{
		ID:           "fallback-success-build",
		ProviderMode: "platform",
		Agents: map[string]*Agent{
			"lead-1": {
				ID:       "lead-1",
				Role:     RoleLead,
				Provider: ai.ProviderOllama,
			},
		},
		Tasks: []*Task{
			{
				ID:          "plan-1",
				Type:        TaskPlan,
				Status:      TaskCompleted,
				AssignedTo:  "lead-1",
				CompletedAt: &completedAt,
				Output: &TaskOutput{
					Metrics: map[string]any{
						"selected_provider": "claude",
						"model":             "claude-haiku-4-5-20251001",
					},
				},
			},
		},
	}

	got := am.getCurrentlyAvailableProvidersForBuild(build)
	if len(got) != 2 || got[0] != ai.ProviderClaude || got[1] != ai.ProviderGPT4 {
		t.Fatalf("expected no ollama retention after claude fallback, got %v", got)
	}
}
