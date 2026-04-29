package ai

import (
	"context"
	"testing"
	"time"
)

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

	router := &AIRouter{
		clients: map[AIProvider]AIClient{
			ProviderGPT4: &routerStubClient{
				generate: func(ctx context.Context, req *AIRequest) (*AIResponse, error) {
					primaryCalls++
					<-ctx.Done()
					return nil, ctx.Err()
				},
			},
			ProviderClaude: &routerStubClient{
				generate: func(ctx context.Context, req *AIRequest) (*AIResponse, error) {
					fallbackCalls++
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

	ctx, cancel := context.WithTimeout(context.Background(), 160*time.Millisecond)
	defer cancel()

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
