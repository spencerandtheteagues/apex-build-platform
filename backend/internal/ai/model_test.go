package ai

import "testing"

func TestActualProviderUsesMetadataActualProvider(t *testing.T) {
	resp := &AIResponse{
		Provider: ProviderClaude,
		Metadata: map[string]interface{}{
			"actual_provider": string(ProviderOllama),
		},
	}

	if got := ActualProvider(resp, ProviderClaude); got != ProviderOllama {
		t.Fatalf("ActualProvider() = %q, want %q", got, ProviderOllama)
	}
}

func TestActualProviderFallsBackToResponseProvider(t *testing.T) {
	resp := &AIResponse{Provider: ProviderGPT4}

	if got := ActualProvider(resp, ProviderClaude); got != ProviderGPT4 {
		t.Fatalf("ActualProvider() = %q, want %q", got, ProviderGPT4)
	}
}

func TestActualProviderRejectsUnknownMetadataProvider(t *testing.T) {
	resp := &AIResponse{
		Provider: ProviderGemini,
		Metadata: map[string]interface{}{
			"actual_provider": "unknown",
		},
	}

	if got := ActualProvider(resp, ProviderClaude); got != ProviderGemini {
		t.Fatalf("ActualProvider() = %q, want %q", got, ProviderGemini)
	}
}
