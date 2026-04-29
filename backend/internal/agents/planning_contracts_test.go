package agents

import (
	"context"
	"sync"
	"testing"
	"time"

	"apex-build/internal/agents/autonomous"
	"apex-build/internal/ai"
)

type planningFallbackProbeRouter struct {
	mu    sync.Mutex
	calls []ai.AIProvider
}

func (r *planningFallbackProbeRouter) Generate(ctx context.Context, provider ai.AIProvider, _ string, _ GenerateOptions) (*ai.AIResponse, error) {
	r.mu.Lock()
	r.calls = append(r.calls, provider)
	r.mu.Unlock()

	if provider == ai.ProviderOllama {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &ai.AIResponse{
		Provider: provider,
		Content:  `{"ok":true}`,
		Usage:    &ai.Usage{},
	}, nil
}

func (r *planningFallbackProbeRouter) GetAvailableProviders() []ai.AIProvider {
	return []ai.AIProvider{ai.ProviderOllama, ai.ProviderClaude}
}

func (r *planningFallbackProbeRouter) GetAvailableProvidersForUser(_ uint) []ai.AIProvider {
	return r.GetAvailableProviders()
}

func (r *planningFallbackProbeRouter) HasConfiguredProviders() bool { return true }

func TestPlannerRouterAdapterFallsBackWhenPrimaryPlanningProviderTimesOut(t *testing.T) {
	t.Setenv("APEX_PLANNING_PROVIDER_TIMEOUT_MS", "5")

	router := &planningFallbackProbeRouter{}
	adapter := &plannerRouterAdapter{
		router:          router,
		provider:        ai.ProviderOllama,
		providers:       []ai.AIProvider{ai.ProviderOllama, ai.ProviderClaude},
		userID:          1,
		powerMode:       PowerMax,
		usePlatformKeys: true,
	}

	content, err := adapter.Generate(context.Background(), "plan this app", autonomous.AIOptions{
		MaxTokens:    4000,
		Temperature:  0.2,
		SystemPrompt: "Return JSON.",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if content != `{"ok":true}` {
		t.Fatalf("content = %q, want fallback content", content)
	}
	if adapter.lastProvider != ai.ProviderClaude {
		t.Fatalf("last provider = %s, want claude", adapter.lastProvider)
	}

	router.mu.Lock()
	defer router.mu.Unlock()
	if len(router.calls) != 2 || router.calls[0] != ai.ProviderOllama || router.calls[1] != ai.ProviderClaude {
		t.Fatalf("provider calls = %+v, want [ollama claude]", router.calls)
	}
}

func TestPlanningProviderAttemptTimeoutUsesShortManagedCloudCaps(t *testing.T) {
	t.Setenv("APEX_PLANNING_PROVIDER_TIMEOUT_MS", "")
	t.Setenv("APEX_PLANNING_PROVIDER_TIMEOUT_SECONDS", "")

	if got := planningProviderAttemptTimeout(ai.ProviderOllama, PowerMax, true); got != 90*time.Second {
		t.Fatalf("managed ollama max planning timeout = %s, want 90s", got)
	}
	if got := planningProviderAttemptTimeout(ai.ProviderOllama, PowerBalanced, false); got != 120*time.Second {
		t.Fatalf("BYOK/local ollama balanced planning timeout = %s, want 120s", got)
	}
	t.Setenv("APEX_PLANNING_OLLAMA_TIMEOUT_SECONDS", "240")
	if got := planningProviderAttemptTimeout(ai.ProviderOllama, PowerBalanced, false); got != 4*time.Minute {
		t.Fatalf("ollama planning timeout override = %s, want 4m", got)
	}
}
