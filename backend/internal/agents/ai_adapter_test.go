package agents

import (
	"testing"
	"time"

	"apex-build/internal/ai"
)

func TestDefaultGenerateTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider ai.AIProvider
		mode     PowerMode
		want     time.Duration
	}{
		{name: "cloud fast", provider: ai.ProviderClaude, mode: PowerFast, want: 2 * time.Minute},
		{name: "cloud balanced", provider: ai.ProviderClaude, mode: PowerBalanced, want: 150 * time.Second},
		{name: "cloud max", provider: ai.ProviderClaude, mode: PowerMax, want: 3 * time.Minute},
		{name: "ollama fast", provider: ai.ProviderOllama, mode: PowerFast, want: 4 * time.Minute},
		{name: "ollama balanced", provider: ai.ProviderOllama, mode: PowerBalanced, want: 5 * time.Minute},
		{name: "ollama max", provider: ai.ProviderOllama, mode: PowerMax, want: 6 * time.Minute},
		{name: "empty mode defaults to fast", provider: ai.ProviderGemini, mode: "", want: 2 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := defaultGenerateTimeout(tt.provider, tt.mode); got != tt.want {
				t.Fatalf("defaultGenerateTimeout(%q, %q) = %v, want %v", tt.provider, tt.mode, got, tt.want)
			}
		})
	}
}
