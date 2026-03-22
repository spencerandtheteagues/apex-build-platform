package ai

import (
	"context"
	"testing"
	"time"
)

type stubProviderClient struct {
	provider AIProvider
	lastReq  *AIRequest
}

func (s *stubProviderClient) Generate(_ context.Context, req *AIRequest) (*AIResponse, error) {
	if req != nil {
		copyReq := *req
		s.lastReq = &copyReq
	}
	return &AIResponse{
		Provider: ProviderOllama,
		Content:  "ok",
		Metadata: map[string]interface{}{},
	}, nil
}

func (s *stubProviderClient) GetCapabilities() []AICapability {
	return []AICapability{CapabilityCodeGeneration}
}
func (s *stubProviderClient) GetProvider() AIProvider      { return s.provider }
func (s *stubProviderClient) Health(context.Context) error { return nil }
func (s *stubProviderClient) GetUsage() *ProviderUsage {
	return &ProviderUsage{Provider: s.provider, LastUsed: time.Now()}
}

func TestProviderAliasClientRewritesProviderIdentity(t *testing.T) {
	base := &stubProviderClient{provider: ProviderOllama}
	client := &providerAliasClient{
		alias: ProviderClaude,
		base:  base,
	}

	resp, err := client.Generate(context.Background(), &AIRequest{Prompt: "hi"})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if resp.Provider != ProviderClaude {
		t.Fatalf("expected aliased provider claude, got %s", resp.Provider)
	}
	if resp.Metadata["actual_provider"] != string(ProviderOllama) {
		t.Fatalf("expected actual_provider=ollama, got %#v", resp.Metadata["actual_provider"])
	}
	if client.GetProvider() != ProviderClaude {
		t.Fatalf("expected GetProvider to return alias claude, got %s", client.GetProvider())
	}
}

func TestNewAliasedOllamaProviderClientAppliesDefaultModel(t *testing.T) {
	base := &stubProviderClient{provider: ProviderOllama}
	client := &providerAliasClient{
		alias: ProviderGPT4,
		base: &forceModelClient{
			base:  base,
			model: "qwen3-coder:30b",
		},
	}

	_, err := client.Generate(context.Background(), &AIRequest{Prompt: "build app"})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if base.lastReq == nil {
		t.Fatal("expected underlying client to receive request")
	}
	if base.lastReq.Model != "qwen3-coder:30b" {
		t.Fatalf("expected default model qwen3-coder:30b, got %q", base.lastReq.Model)
	}
}

func TestNewAIRouterAddsOllamaEmulatedProviderSlots(t *testing.T) {
	t.Setenv("CLAUDE_OLLAMA_URL", "http://127.0.0.1:11434")
	t.Setenv("CLAUDE_OLLAMA_MODEL", "qwen3-coder:30b")
	t.Setenv("OPENAI_OLLAMA_URL", "http://127.0.0.1:11435")
	t.Setenv("OPENAI_OLLAMA_MODEL", "qwen3-coder:30b")

	router := NewAIRouter("", "", "")
	t.Cleanup(func() {
		if router != nil {
			router.healthCheck = nil
		}
	})

	if _, ok := router.clients[ProviderClaude]; !ok {
		t.Fatal("expected claude slot to be emulated with ollama")
	}
	if _, ok := router.clients[ProviderGPT4]; !ok {
		t.Fatal("expected gpt4 slot to be emulated with ollama")
	}
	if got := router.clients[ProviderClaude].GetProvider(); got != ProviderClaude {
		t.Fatalf("expected emulated claude slot to report claude, got %s", got)
	}
	if got := router.clients[ProviderGPT4].GetProvider(); got != ProviderGPT4 {
		t.Fatalf("expected emulated gpt4 slot to report gpt4, got %s", got)
	}
}
