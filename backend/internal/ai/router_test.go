package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// healthStubClient is a configurable AIClient for exercising health-check classification
// and the detail surfaced by GetDetailedHealthStatus.
type healthStubClient struct {
	provider AIProvider
	healthFn func(context.Context) error
}

func (s *healthStubClient) Generate(ctx context.Context, req *AIRequest) (*AIResponse, error) {
	return &AIResponse{Provider: s.provider, Content: "ok"}, nil
}
func (s *healthStubClient) GetCapabilities() []AICapability {
	return []AICapability{CapabilityCodeGeneration}
}
func (s *healthStubClient) GetProvider() AIProvider { return s.provider }
func (s *healthStubClient) Health(ctx context.Context) error {
	if s.healthFn == nil {
		return nil
	}
	return s.healthFn(ctx)
}
func (s *healthStubClient) GetUsage() *ProviderUsage {
	return &ProviderUsage{Provider: s.provider, LastUsed: time.Now()}
}

func TestClassifyProviderErrorMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil is ok", nil, "ok"},
		{"gemini rate limit prefix is transient error", errors.New("RATE_LIMIT: Gemini API rate limit exceeded - please wait before retrying (status=429)"), "error"},
		{"gemini quota prefix is no_credits", errors.New("QUOTA_EXCEEDED: Gemini API quota exhausted - add billing or use another provider (status=403)"), "no_credits"},
		{"gemini unauthorized prefix is auth_error", errors.New("UNAUTHORIZED: invalid Gemini API key (status=401)"), "auth_error"},
		{"gemini forbidden prefix is auth_error", errors.New("FORBIDDEN: Gemini API access denied - check API key permissions (status=403)"), "auth_error"},
		{"gemini service error prefix is transient error", errors.New("SERVICE_ERROR: Gemini service temporarily unavailable (status=503)"), "error"},
		{"grok disabled key forbidden is auth_error", errors.New("FORBIDDEN: Grok API key is disabled - enable or regenerate the key in the xAI console (status 403)"), "auth_error"},
		{"auth prefix wins over quota mention in body", errors.New("FORBIDDEN: Gemini API access denied (status=403: your quota and billing look fine but key lacks permission)"), "auth_error"},
		{"timeout heuristic", errors.New("context deadline exceeded"), "timeout"},
		{"raw model_not_found is auth_error", errors.New(`{"error":{"message":"model does not exist","code":"model_not_found"}}`), "auth_error"},
		{"unknown body is generic error", errors.New("API_ERROR: Grok request failed (status=520: cloudflare hiccup)"), "error"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := classifyProviderError(tc.err); got != tc.want {
				t.Fatalf("classifyProviderError(%q) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestPerformHealthChecksSurfacesRedactedDetail(t *testing.T) {
	t.Parallel()

	router := &AIRouter{
		clients: map[AIProvider]AIClient{
			ProviderGemini: &healthStubClient{
				provider: ProviderGemini,
				healthFn: func(context.Context) error {
					// Simulate a transport error that leaked the key in the URL query string.
					return errors.New("failed to make request: Get \"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-lite:generateContent?key=AIzaSyEXAMPLEexamplekey1234567890abcd\": dial tcp: timeout")
				},
			},
			ProviderClaude: &healthStubClient{provider: ProviderClaude},
		},
	}

	router.performHealthChecks()

	detail := router.GetDetailedHealthStatus()

	gemini := detail[string(ProviderGemini)]
	if gemini == nil {
		t.Fatal("missing gemini detail")
	}
	if gemini.Status != "error" && gemini.Status != "timeout" {
		t.Fatalf("gemini status = %q, want error/timeout", gemini.Status)
	}
	if gemini.Detail == "" {
		t.Fatal("expected a non-empty diagnostic detail for the failing provider")
	}
	if strings.Contains(gemini.Detail, "AIza") || strings.Contains(gemini.Detail, "key=AIza") {
		t.Fatalf("health detail leaked api key: %q", gemini.Detail)
	}

	claude := detail[string(ProviderClaude)]
	if claude == nil || claude.Status != "ok" {
		t.Fatalf("claude status = %+v, want ok", claude)
	}
	if claude.Detail != "" {
		t.Fatalf("healthy provider should have empty detail, got %q", claude.Detail)
	}
}

type routerStubClient struct {
	generate func(context.Context, *AIRequest) (*AIResponse, error)
}

func (s *routerStubClient) Generate(ctx context.Context, req *AIRequest) (*AIResponse, error) {
	if s.generate == nil {
		return &AIResponse{Provider: req.Provider, Content: "ok"}, nil
	}
	return s.generate(ctx, req)
}

func (s *routerStubClient) GetCapabilities() []AICapability {
	return []AICapability{CapabilityCodeGeneration}
}

func (s *routerStubClient) GetProvider() AIProvider      { return ProviderGPT4 }
func (s *routerStubClient) Health(context.Context) error { return nil }
func (s *routerStubClient) GetUsage() *ProviderUsage {
	return &ProviderUsage{Provider: ProviderGPT4, LastUsed: time.Now()}
}

type fakeProviderRateLimitStore struct {
	allow bool
	calls int
}

func (f *fakeProviderRateLimitStore) Allow(ctx context.Context, provider AIProvider, limit int, window time.Duration) (bool, error) {
	f.calls++
	return f.allow, nil
}

func (f *fakeProviderRateLimitStore) Close() error { return nil }

func TestSelectProviderHonorsExplicitProviderDespiteTransientHealth(t *testing.T) {
	t.Parallel()

	router := &AIRouter{
		clients: map[AIProvider]AIClient{
			ProviderGrok:   &routerStubClient{},
			ProviderClaude: &routerStubClient{},
		},
		config: DefaultRouterConfig(),
		healthStatus: map[AIProvider]string{
			ProviderGrok:   "timeout",
			ProviderClaude: "ok",
		},
		healthCheck: map[AIProvider]bool{
			ProviderGrok:   false,
			ProviderClaude: true,
		},
	}

	provider, err := router.selectProvider(&AIRequest{Provider: ProviderGrok})
	if err != nil {
		t.Fatalf("selectProvider returned error: %v", err)
	}
	if provider != ProviderGrok {
		t.Fatalf("provider = %s, want %s", provider, ProviderGrok)
	}
}

func TestGeneratePreservesTimeForFallbackProvider(t *testing.T) {
	t.Parallel()

	primaryCalls := 0
	fallbackCalls := 0
	primaryHadReservedBudget := false

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	parentDeadline, _ := ctx.Deadline()

	router := &AIRouter{
		clients: map[AIProvider]AIClient{
			ProviderGPT4: &routerStubClient{
				generate: func(ctx context.Context, req *AIRequest) (*AIResponse, error) {
					primaryCalls++
					if deadline, ok := ctx.Deadline(); ok && deadline.Before(parentDeadline.Add(-100*time.Millisecond)) {
						primaryHadReservedBudget = true
					}
					return nil, context.DeadlineExceeded
				},
			},
			ProviderClaude: &routerStubClient{
				generate: func(ctx context.Context, req *AIRequest) (*AIResponse, error) {
					fallbackCalls++
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					return &AIResponse{
						Provider: ProviderClaude,
						Content:  "fallback ok",
					}, nil
				},
			},
		},
		config: DefaultRouterConfig(),
		healthStatus: map[AIProvider]string{
			ProviderGPT4:   "ok",
			ProviderClaude: "ok",
		},
		healthCheck: map[AIProvider]bool{
			ProviderGPT4:   true,
			ProviderClaude: true,
		},
	}

	resp, err := router.Generate(ctx, &AIRequest{
		ID:         "fallback-budget",
		Provider:   ProviderGPT4,
		Capability: CapabilityCodeGeneration,
		Prompt:     "Build a dashboard",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if resp == nil || resp.Provider != ProviderClaude || resp.Content != "fallback ok" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if primaryCalls != 1 {
		t.Fatalf("primary calls = %d, want 1", primaryCalls)
	}
	if fallbackCalls != 1 {
		t.Fatalf("fallback calls = %d, want 1", fallbackCalls)
	}
	if !primaryHadReservedBudget {
		t.Fatal("primary provider attempt did not reserve parent deadline time for fallback")
	}
}

func TestGenerateCanDisableInternalFallback(t *testing.T) {
	t.Parallel()

	primaryCalls := 0
	fallbackCalls := 0

	router := &AIRouter{
		clients: map[AIProvider]AIClient{
			ProviderGPT4: &routerStubClient{
				generate: func(ctx context.Context, req *AIRequest) (*AIResponse, error) {
					primaryCalls++
					return nil, context.DeadlineExceeded
				},
			},
			ProviderClaude: &routerStubClient{
				generate: func(ctx context.Context, req *AIRequest) (*AIResponse, error) {
					fallbackCalls++
					return &AIResponse{Provider: ProviderClaude, Content: "fallback should not run"}, nil
				},
			},
		},
		config: DefaultRouterConfig(),
		healthStatus: map[AIProvider]string{
			ProviderGPT4:   "ok",
			ProviderClaude: "ok",
		},
		healthCheck: map[AIProvider]bool{
			ProviderGPT4:   true,
			ProviderClaude: true,
		},
	}

	_, err := router.Generate(context.Background(), &AIRequest{
		ID:              "no-internal-fallback",
		Provider:        ProviderGPT4,
		Capability:      CapabilityCodeGeneration,
		Prompt:          "Build a dashboard",
		DisableFallback: true,
	})
	if err == nil {
		t.Fatal("expected primary provider error")
	}
	if primaryCalls != 1 {
		t.Fatalf("primary calls = %d, want 1", primaryCalls)
	}
	if fallbackCalls != 0 {
		t.Fatalf("fallback calls = %d, want 0 when fallback is disabled", fallbackCalls)
	}
}

func TestGenerateWithDisabledFallbackRejectsUnavailableExplicitProvider(t *testing.T) {
	t.Parallel()

	fallbackCalls := 0

	router := &AIRouter{
		clients: map[AIProvider]AIClient{
			ProviderGPT4: &routerStubClient{
				generate: func(ctx context.Context, req *AIRequest) (*AIResponse, error) {
					fallbackCalls++
					return &AIResponse{Provider: req.Provider, Content: "fallback should not run"}, nil
				},
			},
		},
		config: DefaultRouterConfig(),
		healthStatus: map[AIProvider]string{
			ProviderGPT4: "ok",
		},
		healthCheck: map[AIProvider]bool{
			ProviderGPT4: true,
		},
	}

	_, err := router.Generate(context.Background(), &AIRequest{
		ID:              "no-fallback-unavailable-explicit-provider",
		Provider:        ProviderOpenRouter,
		Capability:      CapabilityCodeGeneration,
		Prompt:          "Build a dashboard",
		DisableFallback: true,
	})
	if err == nil {
		t.Fatal("expected unavailable explicit provider error")
	}
	if !strings.Contains(err.Error(), "provider openrouter unavailable") {
		t.Fatalf("expected unavailable openrouter error, got %v", err)
	}
	if fallbackCalls != 0 {
		t.Fatalf("fallback calls = %d, want 0 when fallback is disabled", fallbackCalls)
	}
}

func TestGenerateWithDisabledFallbackRejectsUnhealthyExplicitProvider(t *testing.T) {
	t.Parallel()

	fallbackCalls := 0
	openRouterCalls := 0

	router := &AIRouter{
		clients: map[AIProvider]AIClient{
			ProviderOpenRouter: &routerStubClient{
				generate: func(ctx context.Context, req *AIRequest) (*AIResponse, error) {
					openRouterCalls++
					return &AIResponse{Provider: req.Provider, Content: "unhealthy primary should not run"}, nil
				},
			},
			ProviderGPT4: &routerStubClient{
				generate: func(ctx context.Context, req *AIRequest) (*AIResponse, error) {
					fallbackCalls++
					return &AIResponse{Provider: req.Provider, Content: "fallback should not run"}, nil
				},
			},
		},
		config: DefaultRouterConfig(),
		healthStatus: map[AIProvider]string{
			ProviderOpenRouter: "auth_error",
			ProviderGPT4:       "ok",
		},
		healthCheck: map[AIProvider]bool{
			ProviderOpenRouter: false,
			ProviderGPT4:       true,
		},
	}

	_, err := router.Generate(context.Background(), &AIRequest{
		ID:              "no-fallback-unhealthy-explicit-provider",
		Provider:        ProviderOpenRouter,
		Capability:      CapabilityCodeGeneration,
		Prompt:          "Build a dashboard",
		DisableFallback: true,
	})
	if err == nil {
		t.Fatal("expected unhealthy explicit provider error")
	}
	if !strings.Contains(err.Error(), "provider openrouter unhealthy") {
		t.Fatalf("expected unhealthy openrouter error, got %v", err)
	}
	if openRouterCalls != 0 {
		t.Fatalf("openrouter calls = %d, want 0 when provider is definitively unhealthy", openRouterCalls)
	}
	if fallbackCalls != 0 {
		t.Fatalf("fallback calls = %d, want 0 when fallback is disabled", fallbackCalls)
	}
}

func TestGenerateWithDisabledFallbackAllowsOpenRouterFreeModelDespiteNoCreditsHealth(t *testing.T) {
	t.Parallel()

	openRouterCalls := 0
	fallbackCalls := 0

	router := &AIRouter{
		clients: map[AIProvider]AIClient{
			ProviderOpenRouter: &routerStubClient{
				generate: func(ctx context.Context, req *AIRequest) (*AIResponse, error) {
					openRouterCalls++
					return &AIResponse{Provider: req.Provider, Content: "free openrouter ok"}, nil
				},
			},
			ProviderGPT4: &routerStubClient{
				generate: func(ctx context.Context, req *AIRequest) (*AIResponse, error) {
					fallbackCalls++
					return &AIResponse{Provider: req.Provider, Content: "fallback should not run"}, nil
				},
			},
		},
		config: DefaultRouterConfig(),
		healthStatus: map[AIProvider]string{
			ProviderOpenRouter: "no_credits",
			ProviderGPT4:       "ok",
		},
		healthCheck: map[AIProvider]bool{
			ProviderOpenRouter: false,
			ProviderGPT4:       true,
		},
	}

	resp, err := router.Generate(context.Background(), &AIRequest{
		ID:              "no-fallback-openrouter-free-despite-no-credits-health",
		Provider:        ProviderOpenRouter,
		Model:           "moonshotai/kimi-k2.6:free",
		Capability:      CapabilityCodeGeneration,
		Prompt:          "Build a dashboard",
		DisableFallback: true,
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if resp == nil || resp.Provider != ProviderOpenRouter {
		t.Fatalf("response = %+v, want openrouter", resp)
	}
	if openRouterCalls != 1 {
		t.Fatalf("openrouter calls = %d, want 1", openRouterCalls)
	}
	if fallbackCalls != 0 {
		t.Fatalf("fallback calls = %d, want 0 when fallback is disabled", fallbackCalls)
	}
}

func TestGenerateReroutesWhenDefaultProviderExceedsCostThreshold(t *testing.T) {
	t.Parallel()

	gptCalls := 0
	geminiCalls := 0
	config := DefaultRouterConfig()
	config.CostThresholds[ProviderGPT4] = 0.000001
	config.CostThresholds[ProviderGemini] = 10

	router := &AIRouter{
		clients: map[AIProvider]AIClient{
			ProviderGPT4: &routerStubClient{
				generate: func(ctx context.Context, req *AIRequest) (*AIResponse, error) {
					gptCalls++
					return &AIResponse{Provider: req.Provider, Content: "expensive primary"}, nil
				},
			},
			ProviderGemini: &routerStubClient{
				generate: func(ctx context.Context, req *AIRequest) (*AIResponse, error) {
					geminiCalls++
					return &AIResponse{Provider: req.Provider, Content: "cost-safe fallback"}, nil
				},
			},
		},
		config: config,
		healthStatus: map[AIProvider]string{
			ProviderGPT4:   "ok",
			ProviderGemini: "ok",
		},
		healthCheck: map[AIProvider]bool{
			ProviderGPT4:   true,
			ProviderGemini: true,
		},
	}

	resp, err := router.Generate(context.Background(), &AIRequest{
		ID:         "cost-threshold-reroute",
		Capability: CapabilityCodeGeneration,
		Prompt:     "Build a dashboard",
		MaxTokens:  32000,
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if resp.Provider != ProviderGemini {
		t.Fatalf("provider = %s, want %s", resp.Provider, ProviderGemini)
	}
	if gptCalls != 0 {
		t.Fatalf("gpt calls = %d, want 0 when cost threshold forces pre-call reroute", gptCalls)
	}
	if geminiCalls != 1 {
		t.Fatalf("gemini calls = %d, want 1", geminiCalls)
	}
}

func TestGenerateBlocksExplicitProviderOverCostThresholdWhenFallbackDisabled(t *testing.T) {
	t.Parallel()

	gptCalls := 0
	config := DefaultRouterConfig()
	config.CostThresholds[ProviderGPT4] = 0.000001

	router := &AIRouter{
		clients: map[AIProvider]AIClient{
			ProviderGPT4: &routerStubClient{
				generate: func(ctx context.Context, req *AIRequest) (*AIResponse, error) {
					gptCalls++
					return &AIResponse{Provider: req.Provider, Content: "should not run"}, nil
				},
			},
		},
		config: config,
		healthStatus: map[AIProvider]string{
			ProviderGPT4: "ok",
		},
		healthCheck: map[AIProvider]bool{
			ProviderGPT4: true,
		},
	}

	_, err := router.Generate(context.Background(), &AIRequest{
		ID:              "cost-threshold-disable-fallback",
		Provider:        ProviderGPT4,
		Capability:      CapabilityCodeGeneration,
		Prompt:          "Build a dashboard",
		MaxTokens:       32000,
		DisableFallback: true,
	})
	if err == nil {
		t.Fatal("expected cost threshold error")
	}
	if gptCalls != 0 {
		t.Fatalf("gpt calls = %d, want 0 when cost threshold blocks pre-call", gptCalls)
	}
}

func TestCheckRateLimitUsesSharedProviderStoreWhenConfigured(t *testing.T) {
	t.Parallel()

	store := &fakeProviderRateLimitStore{allow: false}
	router := &AIRouter{
		config: DefaultRouterConfig(),
		rateLimits: map[AIProvider]*rateLimiter{
			ProviderGPT4: {
				tokens:     100,
				maxTokens:  100,
				lastRefill: time.Now(),
			},
		},
		sharedRates: store,
	}

	if router.checkRateLimit(ProviderGPT4) {
		t.Fatal("expected shared provider rate limiter to deny request")
	}
	if store.calls != 1 {
		t.Fatalf("shared store calls = %d, want 1", store.calls)
	}
	if router.rateLimits[ProviderGPT4].tokens != 100 {
		t.Fatalf("local tokens changed despite shared limiter decision: %d", router.rateLimits[ProviderGPT4].tokens)
	}
}
