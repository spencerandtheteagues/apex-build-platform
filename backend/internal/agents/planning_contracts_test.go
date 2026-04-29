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

type planningHardTimeoutProbeRouter struct {
	mu    sync.Mutex
	calls []ai.AIProvider
}

func (r *planningHardTimeoutProbeRouter) Generate(ctx context.Context, provider ai.AIProvider, _ string, _ GenerateOptions) (*ai.AIResponse, error) {
	r.mu.Lock()
	r.calls = append(r.calls, provider)
	r.mu.Unlock()

	if provider == ai.ProviderClaude {
		time.Sleep(50 * time.Millisecond)
		return &ai.AIResponse{Provider: provider, Content: `{"late":true}`, Usage: &ai.Usage{}}, nil
	}
	return &ai.AIResponse{Provider: provider, Content: `{"ok":true}`, Usage: &ai.Usage{}}, nil
}

func (r *planningHardTimeoutProbeRouter) GetAvailableProviders() []ai.AIProvider {
	return []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4}
}

func (r *planningHardTimeoutProbeRouter) GetAvailableProvidersForUser(_ uint) []ai.AIProvider {
	return r.GetAvailableProviders()
}

func (r *planningHardTimeoutProbeRouter) HasConfiguredProviders() bool { return true }

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

func TestPlannerRouterAdapterHardTimesOutProviderThatIgnoresContext(t *testing.T) {
	t.Setenv("APEX_PLANNING_PROVIDER_TIMEOUT_MS", "5")

	router := &planningHardTimeoutProbeRouter{}
	adapter := &plannerRouterAdapter{
		router:          router,
		provider:        ai.ProviderClaude,
		providers:       []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4},
		userID:          1,
		powerMode:       PowerBalanced,
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
	if adapter.lastProvider != ai.ProviderGPT4 {
		t.Fatalf("last provider = %s, want gpt4", adapter.lastProvider)
	}

	router.mu.Lock()
	defer router.mu.Unlock()
	if len(router.calls) < 2 || router.calls[0] != ai.ProviderClaude || router.calls[1] != ai.ProviderGPT4 {
		t.Fatalf("provider calls = %+v, want claude then gpt4", router.calls)
	}
}

func TestPlanningProviderAttemptTimeoutUsesShortManagedCloudCaps(t *testing.T) {
	t.Setenv("APEX_PLANNING_PROVIDER_TIMEOUT_MS", "")
	t.Setenv("APEX_PLANNING_PROVIDER_TIMEOUT_SECONDS", "")

	if got := planningProviderAttemptTimeout(ai.ProviderOllama, PowerMax, true); got != 45*time.Second {
		t.Fatalf("managed ollama max planning timeout = %s, want 45s", got)
	}
	if got := planningProviderAttemptTimeout(ai.ProviderOllama, PowerBalanced, false); got != 120*time.Second {
		t.Fatalf("BYOK/local ollama balanced planning timeout = %s, want 120s", got)
	}
	if got := planningProviderAttemptTimeout(ai.ProviderClaude, PowerBalanced, true); got != 55*time.Second {
		t.Fatalf("balanced cloud fallback planning timeout = %s, want 55s", got)
	}
	t.Setenv("APEX_PLANNING_OLLAMA_TIMEOUT_SECONDS", "240")
	if got := planningProviderAttemptTimeout(ai.ProviderOllama, PowerBalanced, false); got != 4*time.Minute {
		t.Fatalf("ollama planning timeout override = %s, want 4m", got)
	}
}

type configuredPlanningProviderRouter struct {
	stubAIRouter
	configured []ai.AIProvider
}

func (r *configuredPlanningProviderRouter) GetConfiguredProviders() []ai.AIProvider {
	return append([]ai.AIProvider(nil), r.configured...)
}

func TestPlanningProviderOrderIncludesConfiguredPlatformFallbacks(t *testing.T) {
	am := &AgentManager{
		aiRouter: &configuredPlanningProviderRouter{
			stubAIRouter: stubAIRouter{
				providers:             []ai.AIProvider{ai.ProviderOllama},
				hasConfiguredProvider: true,
			},
			configured: []ai.AIProvider{ai.ProviderOllama, ai.ProviderClaude, ai.ProviderGPT4, ai.ProviderGemini},
		},
	}
	build := &Build{
		ID:           "planning-fallbacks",
		ProviderMode: "platform",
		PowerMode:    PowerMax,
	}
	task := &Task{ID: "plan", Type: TaskPlan}

	got := am.planningProviderOrder(build, task, ai.ProviderOllama)
	want := []ai.AIProvider{ai.ProviderOllama, ai.ProviderClaude, ai.ProviderGPT4, ai.ProviderGemini}
	if len(got) < len(want) {
		t.Fatalf("planning providers = %v, want at least %v", got, want)
	}
	for i, provider := range want {
		if got[i] != provider {
			t.Fatalf("planning providers = %v, want prefix %v", got, want)
		}
	}
}

func TestPlanningProviderOrderPrefersOllamaForBalancedPlatformPlanning(t *testing.T) {
	am := &AgentManager{
		aiRouter: &configuredPlanningProviderRouter{
			stubAIRouter: stubAIRouter{
				providers:             []ai.AIProvider{ai.ProviderClaude, ai.ProviderOllama, ai.ProviderGPT4},
				hasConfiguredProvider: true,
			},
			configured: []ai.AIProvider{ai.ProviderClaude, ai.ProviderOllama, ai.ProviderGPT4},
		},
	}
	build := &Build{
		ID:           "planning-balanced-ollama",
		ProviderMode: "platform",
		PowerMode:    PowerBalanced,
	}
	task := &Task{ID: "plan", Type: TaskPlan}

	got := am.planningProviderOrder(build, task, ai.ProviderClaude)
	wantPrefix := []ai.AIProvider{ai.ProviderOllama, ai.ProviderClaude, ai.ProviderGPT4}
	if len(got) < len(wantPrefix) {
		t.Fatalf("planning providers = %v, want at least %v", got, wantPrefix)
	}
	for i, provider := range wantPrefix {
		if got[i] != provider {
			t.Fatalf("planning providers = %v, want prefix %v", got, wantPrefix)
		}
	}
}

func TestPlanningRouteCandidatesExpandBalancedOllamaCloudModelFallbacks(t *testing.T) {
	t.Setenv("APEX_BALANCED_OLLAMA_PLANNING_MODELS", "")

	got := planningRouteCandidates([]ai.AIProvider{ai.ProviderOllama, ai.ProviderClaude}, PowerBalanced, true)
	want := []planningRouteCandidate{
		{provider: ai.ProviderOllama, model: "kimi-k2.6:cloud"},
		{provider: ai.ProviderOllama, model: "glm-5.1:cloud"},
		{provider: ai.ProviderOllama, model: "deepseek-v4-pro:cloud"},
		{provider: ai.ProviderOllama, model: "deepseek-v4-flash:cloud"},
		{provider: ai.ProviderOllama, model: "qwen3.5:cloud"},
		{provider: ai.ProviderClaude},
	}
	if len(got) != len(want) {
		t.Fatalf("planning routes = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("planning routes = %+v, want %+v", got, want)
		}
	}
}
