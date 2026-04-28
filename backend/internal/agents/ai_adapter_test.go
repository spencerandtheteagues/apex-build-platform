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

func TestSelectModelForPowerModeUsesProviderOwnedMaxModels(t *testing.T) {
	t.Setenv("OLLAMA_API_KEY", "")

	tests := []struct {
		name     string
		provider ai.AIProvider
		mode     PowerMode
		want     string
	}{
		{name: "claude max uses opus", provider: ai.ProviderClaude, mode: PowerMax, want: "claude-opus-4-7"},
		{name: "openai max uses chatgpt", provider: ai.ProviderGPT4, mode: PowerMax, want: "gpt-5.4-pro"},
		{name: "openai fast owns gpt 4o mini", provider: ai.ProviderGPT4, mode: PowerFast, want: "gpt-4o-mini"},
		{name: "gemini max uses pro before preview", provider: ai.ProviderGemini, mode: PowerMax, want: "gemini-3.1-pro"},
		{name: "grok max uses 4.20", provider: ai.ProviderGrok, mode: PowerMax, want: "grok-4.20-0309-reasoning"},
		{name: "ollama fast uses kimi", provider: ai.ProviderOllama, mode: PowerFast, want: "kimi-k2.6"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := selectModelForPowerMode(tt.provider, tt.mode); got != tt.want {
				t.Fatalf("selectModelForPowerMode(%s, %s) = %q, want %q", tt.provider, tt.mode, got, tt.want)
			}
		})
	}
}

func TestSelectModelForPowerModeUsesKimiCloudWhenManagedOllamaEnabled(t *testing.T) {
	t.Setenv("OLLAMA_API_KEY", "managed-key-present")

	if got := selectModelForPowerMode(ai.ProviderOllama, PowerMax); got != "kimi-k2.6:cloud" {
		t.Fatalf("selectModelForPowerMode(ollama, max) = %q, want kimi-k2.6:cloud", got)
	}
	if got := selectModelForPowerMode(ai.ProviderOllama, PowerFast); got != "kimi-k2.6:cloud" {
		t.Fatalf("selectModelForPowerMode(ollama, fast) = %q, want kimi-k2.6:cloud", got)
	}
}

func TestNormalizeModelForProviderRejectsCrossProviderModel(t *testing.T) {
	t.Parallel()

	if got := normalizeModelForProvider(ai.ProviderClaude, "gpt-4o-mini", PowerMax); got != "claude-opus-4-7" {
		t.Fatalf("Claude model normalization = %q, want Claude Opus 4.7", got)
	}
	if got := normalizeModelForProvider(ai.ProviderGPT4, "claude-opus-4-6", PowerFast); got != "gpt-4o-mini" {
		t.Fatalf("OpenAI model normalization = %q, want GPT-4o Mini", got)
	}
	if got := normalizeModelForProvider(ai.ProviderGrok, "gemini-2.5-flash", PowerMax); got != "grok-4.20-0309-reasoning" {
		t.Fatalf("Grok model normalization = %q, want Grok 4.20", got)
	}
	if got := normalizeModelForProvider(ai.ProviderGemini, "gemini-3.1-pro-preview", PowerMax); got != "gemini-3.1-pro-preview" {
		t.Fatalf("Gemini preview fallback should remain valid, got %q", got)
	}
	if got := normalizeModelForProvider(ai.ProviderOllama, "glm-5.1", PowerFast); got != "glm-5.1" {
		t.Fatalf("Ollama model normalization = %q, want GLM route to remain valid", got)
	}
	if got := normalizeModelForProvider(ai.ProviderOllama, "deepseek-v4-flash", PowerFast); got != "deepseek-v4-flash" {
		t.Fatalf("Ollama model normalization = %q, want DeepSeek route to remain valid", got)
	}
}

func TestSelectBuildModelForProviderLocked_IgnoresManagedOllamaOverride(t *testing.T) {
	t.Setenv("OLLAMA_API_KEY", "managed-key-present")

	build := &Build{
		ProviderMode: "platform",
		PowerMode:    PowerFast,
		ProviderModelOverrides: map[string]string{
			"ollama": "glm-5.1",
		},
	}

	if got := selectBuildModelForProviderLocked(build, ai.ProviderOllama); got != "kimi-k2.6:cloud" {
		t.Fatalf("managed ollama model = %q, want kimi-k2.6:cloud", got)
	}
}

func TestNormalizeExecutionModelForProvider_IgnoresManagedOllamaEnvOverride(t *testing.T) {
	t.Setenv("OLLAMA_API_KEY", "managed-key-present")
	t.Setenv("OLLAMA_MODEL_DEFAULT", "glm-5.1")

	if got := normalizeExecutionModelForProvider(ai.ProviderOllama, "glm-5.1", PowerFast, true); got != "kimi-k2.6:cloud" {
		t.Fatalf("managed ollama execution model = %q, want kimi-k2.6:cloud", got)
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

func TestPartitionPlatformProvidersByHealth_PrefersHealthyProviders(t *testing.T) {
	t.Parallel()

	healthy, degraded := partitionPlatformProvidersByHealth(map[ai.AIProvider]bool{
		ai.ProviderClaude: true,
		ai.ProviderGPT4:   true,
		ai.ProviderGrok:   false,
		ai.ProviderGemini: false,
	})

	if len(healthy) != 2 || healthy[0] != ai.ProviderClaude || healthy[1] != ai.ProviderGPT4 {
		t.Fatalf("healthy providers = %+v, want [claude gpt4]", healthy)
	}
	if len(degraded) != 2 || degraded[0] != ai.ProviderGemini || degraded[1] != ai.ProviderGrok {
		t.Fatalf("degraded providers = %+v, want [gemini grok]", degraded)
	}
}

func TestPartitionPlatformProvidersByHealth_FallsBackToDegradedWhenNeeded(t *testing.T) {
	t.Parallel()

	healthy, degraded := partitionPlatformProvidersByHealth(map[ai.AIProvider]bool{
		ai.ProviderClaude: false,
		ai.ProviderGPT4:   false,
	})

	if len(healthy) != 0 {
		t.Fatalf("healthy providers = %+v, want none", healthy)
	}
	if len(degraded) != 2 || degraded[0] != ai.ProviderClaude || degraded[1] != ai.ProviderGPT4 {
		t.Fatalf("degraded providers = %+v, want [claude gpt4]", degraded)
	}
}

func TestPartitionPlatformProvidersByHealth_PrefersManagedOllamaFirst(t *testing.T) {
	t.Parallel()

	healthy, degraded := partitionPlatformProvidersByHealth(map[ai.AIProvider]bool{
		ai.ProviderOllama: true,
		ai.ProviderClaude: true,
		ai.ProviderGPT4:   true,
		ai.ProviderGemini: false,
	})

	if len(healthy) != 3 || healthy[0] != ai.ProviderOllama {
		t.Fatalf("healthy providers = %+v, want ollama first", healthy)
	}
	if len(degraded) != 1 || degraded[0] != ai.ProviderGemini {
		t.Fatalf("degraded providers = %+v, want [gemini]", degraded)
	}
}
