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
		{name: "cloud balanced", provider: ai.ProviderClaude, mode: PowerBalanced, want: 3 * time.Minute},
		{name: "cloud max", provider: ai.ProviderClaude, mode: PowerMax, want: 5 * time.Minute},
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

func TestAIRouterAdapterConfiguredProvidersDoNotDependOnHealthProbe(t *testing.T) {
	router := ai.NewAIRouter("", "sk-test-openai", "")
	adapter := NewAIRouterAdapter(router, nil)

	if !adapter.HasConfiguredProviders() {
		t.Fatal("expected configured provider before health probe settles")
	}
	providers := adapter.GetAvailableProviders()
	if len(providers) == 0 {
		t.Fatal("expected startup availability to include configured providers")
	}
	if providers[0] != ai.ProviderGPT4 {
		t.Fatalf("available providers = %v, want gpt4 first", providers)
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
		{name: "openai max uses chatgpt codex 5.4", provider: ai.ProviderGPT4, mode: PowerMax, want: "gpt-5.4-codex"},
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

func TestNormalizeProviderModelOverrideDowngradesUnavailableOpenAIMaxAlias(t *testing.T) {
	t.Parallel()

	if got := normalizeProviderModelOverride(ai.ProviderGPT4, "gpt-codex-5.5"); got != "gpt-5.4-codex" {
		t.Fatalf("stale codex 5.5 override = %q, want gpt-5.4-codex", got)
	}
	if got := normalizeModelForProvider(ai.ProviderGPT4, "gpt-5.5", PowerMax); got != "gpt-5.4-codex" {
		t.Fatalf("stale gpt 5.5 model = %q, want gpt-5.4-codex", got)
	}
	if got := normalizeProviderModelOverride(ai.ProviderGPT4, "gpt-5.4-pro"); got != "gpt-5.4-codex" {
		t.Fatalf("stale gpt 5.4 pro override = %q, want gpt-5.4-codex", got)
	}
}

func TestNormalizeProviderModelOverrideUpgradesClaudeOpusMaxAlias(t *testing.T) {
	t.Parallel()

	if got := normalizeProviderModelOverride(ai.ProviderClaude, "claude-opus-4-6"); got != "claude-opus-4-7" {
		t.Fatalf("stale claude opus override = %q, want claude-opus-4-7", got)
	}
	if got := normalizeModelForProvider(ai.ProviderClaude, "claude-opus-4-6", PowerMax); got != "claude-opus-4-7" {
		t.Fatalf("stale claude opus model = %q, want claude-opus-4-7", got)
	}
}

func TestNormalizeExecutionModelForProvider_DowngradesFlagshipOverridesInBalanced(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider ai.AIProvider
		model    string
		want     string
	}{
		{name: "claude opus", provider: ai.ProviderClaude, model: "claude-opus-4-7", want: "claude-sonnet-4-6"},
		{name: "openai codex", provider: ai.ProviderGPT4, model: "gpt-5.4-codex", want: "gpt-4.1"},
		{name: "gemini pro", provider: ai.ProviderGemini, model: "gemini-3.1-pro-preview", want: "gemini-2.5-pro"},
		{name: "grok reasoning", provider: ai.ProviderGrok, model: "grok-4.20-0309-reasoning", want: "grok-3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeExecutionModelForProvider(tt.provider, tt.model, PowerBalanced, true); got != tt.want {
				t.Fatalf("balanced %s execution model = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestCreateBuildNormalizesUnavailableOpenAIProviderModelOverride(t *testing.T) {
	manager := &AgentManager{
		builds: make(map[string]*Build),
	}

	build, err := manager.CreateBuild(1, "owner", &BuildRequest{
		Description: "Build a production-ready project management application",
		PowerMode:   PowerMax,
		ProviderModelOverrides: map[string]string{
			"gpt4":   "gpt-codex-5.5",
			"grok":   "grok-4.20-0309-reasoning",
			"gemini": "auto",
		},
	})
	if err != nil {
		t.Fatalf("CreateBuild returned error: %v", err)
	}
	if got := build.ProviderModelOverrides["gpt4"]; got != "gpt-5.4-codex" {
		t.Fatalf("stored OpenAI override = %q, want gpt-5.4-codex", got)
	}
	if _, exists := build.ProviderModelOverrides["gemini"]; exists {
		t.Fatalf("auto override should not be persisted, got %+v", build.ProviderModelOverrides)
	}
}

func TestCreateBuildDowngradesFlagshipOverridesInBalanced(t *testing.T) {
	manager := &AgentManager{
		builds: make(map[string]*Build),
	}

	build, err := manager.CreateBuild(1, "owner", &BuildRequest{
		Description: "Build a production-ready operations dashboard",
		PowerMode:   PowerBalanced,
		ProviderModelOverrides: map[string]string{
			"gpt4":   "gpt-5.4-codex",
			"claude": "claude-opus-4-7",
			"gemini": "gemini-3.1-pro-preview",
			"grok":   "grok-4.20-0309-reasoning",
		},
	})
	if err != nil {
		t.Fatalf("CreateBuild returned error: %v", err)
	}
	want := map[string]string{
		"gpt4":   "gpt-4.1",
		"claude": "claude-sonnet-4-6",
		"gemini": "gemini-2.5-pro",
		"grok":   "grok-3",
	}
	for provider, wantModel := range want {
		if got := build.ProviderModelOverrides[provider]; got != wantModel {
			t.Fatalf("balanced stored override %s = %q, want %q; overrides=%+v", provider, got, wantModel, build.ProviderModelOverrides)
		}
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

func TestSelectBuildModelForProviderLocked_RespectsManagedOllamaOverride(t *testing.T) {
	t.Setenv("OLLAMA_API_KEY", "managed-key-present")

	build := &Build{
		ProviderMode: "platform",
		PowerMode:    PowerFast,
		ProviderModelOverrides: map[string]string{
			"ollama": "glm-5.1",
		},
	}

	if got := selectBuildModelForProviderLocked(build, ai.ProviderOllama); got != "glm-5.1:cloud" {
		t.Fatalf("managed ollama model = %q, want glm-5.1:cloud", got)
	}
}

func TestNormalizeExecutionModelForProvider_QualifiesManagedOllamaCloudOverrides(t *testing.T) {
	t.Setenv("OLLAMA_API_KEY", "managed-key-present")
	t.Setenv("OLLAMA_MODEL_DEFAULT", "glm-5.1")

	if got := normalizeExecutionModelForProvider(ai.ProviderOllama, "glm-5.1", PowerFast, true); got != "glm-5.1:cloud" {
		t.Fatalf("managed ollama execution model = %q, want glm-5.1:cloud", got)
	}
	if got := normalizeExecutionModelForProvider(ai.ProviderOllama, "deepseek-v4", PowerFast, true); got != "deepseek-v4-flash:cloud" {
		t.Fatalf("managed deepseek execution model = %q, want deepseek-v4-flash:cloud", got)
	}
	if got := normalizeExecutionModelForProvider(ai.ProviderOllama, "qwen-3.6-27b", PowerBalanced, true); got != "qwen3.5:cloud" {
		t.Fatalf("managed qwen execution model = %q, want qwen3.5:cloud", got)
	}
	if got := normalizeExecutionModelForProvider(ai.ProviderOllama, "gemini-3-flash-preview:cloud", PowerFast, true); got != "gemini-3-flash-preview:cloud" {
		t.Fatalf("managed ollama-hosted gemini execution model = %q, want gemini-3-flash-preview:cloud", got)
	}
	if got := normalizeExecutionModelForProvider(ai.ProviderOllama, "auto", PowerFast, true); got != "glm-5.1:cloud" {
		t.Fatalf("managed auto execution model = %q, want env default glm-5.1:cloud", got)
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

func TestPartitionPlatformProvidersByHealth_PrefersFlagshipCloudBeforeManagedOllama(t *testing.T) {
	t.Parallel()

	healthy, degraded := partitionPlatformProvidersByHealth(map[ai.AIProvider]bool{
		ai.ProviderOllama: true,
		ai.ProviderClaude: true,
		ai.ProviderGPT4:   true,
		ai.ProviderGemini: false,
	})

	if len(healthy) != 3 || healthy[0] != ai.ProviderClaude || healthy[1] != ai.ProviderGPT4 || healthy[2] != ai.ProviderOllama {
		t.Fatalf("healthy providers = %+v, want [claude gpt4 ollama]", healthy)
	}
	if len(degraded) != 1 || degraded[0] != ai.ProviderGemini {
		t.Fatalf("degraded providers = %+v, want [gemini]", degraded)
	}
}
