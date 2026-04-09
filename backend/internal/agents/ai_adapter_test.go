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

func TestMapProviderToCapabilityPrefersExplicitRoleHint(t *testing.T) {
	t.Parallel()

	adapter := &AIRouterAdapter{}

	tests := []struct {
		name string
		opts GenerateOptions
		want ai.AICapability
	}{
		{name: "planner", opts: GenerateOptions{RoleHint: string(RolePlanner)}, want: ai.CapabilityArchitecture},
		{name: "architect", opts: GenerateOptions{RoleHint: string(RoleArchitect)}, want: ai.CapabilityArchitecture},
		{name: "reviewer", opts: GenerateOptions{RoleHint: string(RoleReviewer)}, want: ai.CapabilityCodeReview},
		{name: "testing", opts: GenerateOptions{RoleHint: string(RoleTesting)}, want: ai.CapabilityTesting},
		{name: "solver", opts: GenerateOptions{RoleHint: string(RoleSolver)}, want: ai.CapabilityDebugging},
		{name: "frontend", opts: GenerateOptions{RoleHint: string(RoleFrontend)}, want: ai.CapabilityCodeGeneration},
		{name: "backend", opts: GenerateOptions{RoleHint: string(RoleBackend)}, want: ai.CapabilityCodeGeneration},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := adapter.mapProviderToCapability(ai.ProviderClaude, tt.opts); got != tt.want {
				t.Fatalf("mapProviderToCapability(..., %+v) = %s, want %s", tt.opts, got, tt.want)
			}
		})
	}
}

func TestMapProviderToCapabilityFallsBackToPromptHints(t *testing.T) {
	t.Parallel()

	adapter := &AIRouterAdapter{}

	if got := adapter.mapProviderToCapability(ai.ProviderClaude, GenerateOptions{SystemPrompt: "You are a senior software architect."}); got != ai.CapabilityArchitecture {
		t.Fatalf("expected architecture capability from prompt fallback, got %s", got)
	}
	if got := adapter.mapProviderToCapability(ai.ProviderClaude, GenerateOptions{SystemPrompt: "You are a strict code review assistant."}); got != ai.CapabilityCodeReview {
		t.Fatalf("expected code review capability from review prompt, got %s", got)
	}
}
