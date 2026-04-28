package agents

import (
	"testing"

	"apex-build/internal/ai"
)

func TestActualProviderForAIResponseUsesRouterMetadata(t *testing.T) {
	resp := &ai.AIResponse{
		Provider: ai.ProviderClaude,
		Metadata: map[string]interface{}{
			"actual_provider": string(ai.ProviderOllama),
			"provider_slot":   string(ai.ProviderClaude),
		},
	}

	if got := actualProviderForAIResponse(resp, ai.ProviderClaude); got != ai.ProviderOllama {
		t.Fatalf("actualProviderForAIResponse() = %q, want %q", got, ai.ProviderOllama)
	}
}

func TestActualProviderForAIResponseFallsBackToResponseProvider(t *testing.T) {
	resp := &ai.AIResponse{Provider: ai.ProviderGPT4}

	if got := actualProviderForAIResponse(resp, ai.ProviderClaude); got != ai.ProviderGPT4 {
		t.Fatalf("actualProviderForAIResponse() = %q, want %q", got, ai.ProviderGPT4)
	}
}

func TestActualProviderForAIResponseFallsBackWhenMetadataInvalid(t *testing.T) {
	resp := &ai.AIResponse{
		Provider: ai.ProviderGemini,
		Metadata: map[string]interface{}{
			"actual_provider": "unknown-provider",
		},
	}

	if got := actualProviderForAIResponse(resp, ai.ProviderClaude); got != ai.ProviderGemini {
		t.Fatalf("actualProviderForAIResponse() = %q, want %q", got, ai.ProviderGemini)
	}
}
