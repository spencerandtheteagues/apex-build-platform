package ai

// CatalogModel describes an OpenRouter model with quality and cost metadata.
// Quality scores (0.0–1.0) are curated estimates based on provider benchmarks,
// community evals, and observed performance — not automated measurements.
type CatalogModel struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Org             string             `json:"org"`
	ContextWindow   int                `json:"context_window"`
	InputPer1M      float64            `json:"input_per_1m"`  // USD per 1M input tokens
	OutputPer1M     float64            `json:"output_per_1m"` // USD per 1M output tokens
	IsFree          bool               `json:"is_free"`
	QualityCode     float64            `json:"quality_code"`      // 0.0–1.0
	QualityReason   float64            `json:"quality_reasoning"` // 0.0–1.0
	SpeedRating     float64            `json:"speed_rating"`      // 0.0–1.0 (higher=faster)
	Tags            []string           `json:"tags,omitempty"`
	Tier            string             `json:"tier"` // "elite","pro","balanced","fast","free"
	Capabilities    []AICapability     `json:"capabilities,omitempty"`
	SupportsTools   bool               `json:"supports_tools"`
	Multimodal      bool               `json:"multimodal"`
}

// isAnthropicModel returns true for any model that must route through direct Anthropic API.
// These are explicitly blocked from OpenRouter routing.
func isAnthropicModel(id string) bool {
	return len(id) >= 10 && (id[:10] == "anthropic/" || id[:11] == "~anthropic/")
}

// OpenRouterCatalog returns the curated model catalog.
// Free models are tagged with IsFree=true and have Tier="free".
// All anthropic/* models are excluded — Claude always routes direct.
func OpenRouterCatalog() []CatalogModel {
	all := []CatalogModel{
		// ── ELITE TIER ──────────────────────────────────────────────────────────
		{
			ID: "openai/gpt-5.5", Name: "GPT-5.5", Org: "openai",
			ContextWindow: 1050000, InputPer1M: 5.00, OutputPer1M: 30.00,
			QualityCode: 0.96, QualityReason: 0.95, SpeedRating: 0.74,
			Tier: "elite", SupportsTools: true,
			Tags: []string{"flagship", "coding", "reasoning"},
		},
		{
			ID: "openai/gpt-5.5-pro", Name: "GPT-5.5 Pro", Org: "openai",
			ContextWindow: 1050000, InputPer1M: 30.00, OutputPer1M: 180.00,
			QualityCode: 0.97, QualityReason: 0.97, SpeedRating: 0.62,
			Tier: "elite", SupportsTools: true,
			Tags: []string{"flagship", "premium", "coding", "reasoning"},
		},
		{
			ID: "openai/o3-pro", Name: "o3 Pro", Org: "openai",
			ContextWindow: 200000, InputPer1M: 20.00, OutputPer1M: 80.00,
			QualityCode: 0.91, QualityReason: 0.99, SpeedRating: 0.45,
			Tier: "elite", SupportsTools: true,
			Tags: []string{"reasoning", "math", "hard-problems"},
		},
		{
			ID: "openai/gpt-5.4", Name: "GPT-5.4", Org: "openai",
			ContextWindow: 1050000, InputPer1M: 2.50, OutputPer1M: 15.00,
			QualityCode: 0.94, QualityReason: 0.93, SpeedRating: 0.78,
			Tier: "elite", SupportsTools: true,
			Tags: []string{"flagship", "coding"},
		},
		{
			ID: "openai/gpt-5.4-pro", Name: "GPT-5.4 Pro", Org: "openai",
			ContextWindow: 1050000, InputPer1M: 30.00, OutputPer1M: 180.00,
			QualityCode: 0.95, QualityReason: 0.94, SpeedRating: 0.63,
			Tier: "elite", SupportsTools: true,
			Tags: []string{"premium"},
		},
		{
			ID: "google/gemini-2.5-pro", Name: "Gemini 2.5 Pro", Org: "google",
			ContextWindow: 1048576, InputPer1M: 1.25, OutputPer1M: 10.00,
			QualityCode: 0.93, QualityReason: 0.93, SpeedRating: 0.76,
			Tier: "elite", SupportsTools: true,
			Tags: []string{"flagship", "long-context", "coding"},
		},
		{
			ID: "google/gemini-2.5-pro-preview", Name: "Gemini 2.5 Pro Preview", Org: "google",
			ContextWindow: 1048576, InputPer1M: 1.25, OutputPer1M: 10.00,
			QualityCode: 0.92, QualityReason: 0.92, SpeedRating: 0.76,
			Tier: "elite", SupportsTools: true,
			Tags: []string{"flagship", "long-context"},
		},

		// ── PRO TIER ────────────────────────────────────────────────────────────
		{
			ID: "openai/gpt-5.3-codex", Name: "GPT-5.3 Codex", Org: "openai",
			ContextWindow: 400000, InputPer1M: 1.75, OutputPer1M: 14.00,
			QualityCode: 0.94, QualityReason: 0.90, SpeedRating: 0.80,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"coding", "codex"},
		},
		{
			ID: "openai/gpt-5.2-codex", Name: "GPT-5.2 Codex", Org: "openai",
			ContextWindow: 400000, InputPer1M: 1.75, OutputPer1M: 14.00,
			QualityCode: 0.93, QualityReason: 0.89, SpeedRating: 0.81,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"coding", "codex"},
		},
		{
			ID: "openai/gpt-5.1-codex", Name: "GPT-5.1 Codex", Org: "openai",
			ContextWindow: 400000, InputPer1M: 1.25, OutputPer1M: 10.00,
			QualityCode: 0.92, QualityReason: 0.88, SpeedRating: 0.82,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"coding", "codex"},
		},
		{
			ID: "openai/gpt-5.1", Name: "GPT-5.1", Org: "openai",
			ContextWindow: 400000, InputPer1M: 1.25, OutputPer1M: 10.00,
			QualityCode: 0.91, QualityReason: 0.89, SpeedRating: 0.82,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"coding"},
		},
		{
			ID: "openai/gpt-5", Name: "GPT-5", Org: "openai",
			ContextWindow: 400000, InputPer1M: 1.25, OutputPer1M: 10.00,
			QualityCode: 0.90, QualityReason: 0.88, SpeedRating: 0.83,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"flagship"},
		},
		{
			ID: "openai/o3", Name: "o3", Org: "openai",
			ContextWindow: 200000, InputPer1M: 2.00, OutputPer1M: 8.00,
			QualityCode: 0.90, QualityReason: 0.98, SpeedRating: 0.55,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"reasoning", "math"},
		},
		{
			ID: "openai/o4-mini", Name: "o4-mini", Org: "openai",
			ContextWindow: 200000, InputPer1M: 1.10, OutputPer1M: 4.40,
			QualityCode: 0.87, QualityReason: 0.94, SpeedRating: 0.75,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"reasoning", "fast-reasoning"},
		},
		{
			ID: "openai/o4-mini-high", Name: "o4-mini High", Org: "openai",
			ContextWindow: 200000, InputPer1M: 1.10, OutputPer1M: 4.40,
			QualityCode: 0.88, QualityReason: 0.95, SpeedRating: 0.70,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"reasoning"},
		},
		{
			ID: "openai/gpt-4.1", Name: "GPT-4.1", Org: "openai",
			ContextWindow: 1047576, InputPer1M: 2.00, OutputPer1M: 8.00,
			QualityCode: 0.88, QualityReason: 0.86, SpeedRating: 0.85,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"long-context"},
		},
		{
			ID: "deepseek/deepseek-v4-pro", Name: "DeepSeek V4 Pro", Org: "deepseek",
			ContextWindow: 1048576, InputPer1M: 0.435, OutputPer1M: 0.87,
			QualityCode: 0.91, QualityReason: 0.90, SpeedRating: 0.78,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"coding", "cost-effective"},
		},
		{
			ID: "deepseek/deepseek-r1-0528", Name: "DeepSeek R1 (0528)", Org: "deepseek",
			ContextWindow: 163840, InputPer1M: 0.50, OutputPer1M: 2.15,
			QualityCode: 0.89, QualityReason: 0.96, SpeedRating: 0.65,
			Tier: "pro", SupportsTools: false,
			Tags: []string{"reasoning", "thinking"},
		},
		{
			ID: "deepseek/deepseek-r1", Name: "DeepSeek R1", Org: "deepseek",
			ContextWindow: 163840, InputPer1M: 0.70, OutputPer1M: 2.50,
			QualityCode: 0.88, QualityReason: 0.95, SpeedRating: 0.65,
			Tier: "pro", SupportsTools: false,
			Tags: []string{"reasoning", "thinking"},
		},
		{
			ID: "x-ai/grok-4.20", Name: "Grok 4.20", Org: "x-ai",
			ContextWindow: 2000000, InputPer1M: 1.25, OutputPer1M: 2.50,
			QualityCode: 0.88, QualityReason: 0.89, SpeedRating: 0.79,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"long-context", "coding"},
		},
		{
			ID: "x-ai/grok-4.20-multi-agent", Name: "Grok 4.20 Multi-Agent", Org: "x-ai",
			ContextWindow: 2000000, InputPer1M: 2.00, OutputPer1M: 6.00,
			QualityCode: 0.89, QualityReason: 0.90, SpeedRating: 0.76,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"agentic", "long-context"},
		},
		{
			ID: "x-ai/grok-build-0.1", Name: "Grok Build 0.1", Org: "x-ai",
			ContextWindow: 256000, InputPer1M: 1.00, OutputPer1M: 2.00,
			QualityCode: 0.90, QualityReason: 0.88, SpeedRating: 0.82,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"coding", "build"},
		},
		{
			ID: "google/gemini-3.5-flash", Name: "Gemini 3.5 Flash", Org: "google",
			ContextWindow: 1048576, InputPer1M: 1.50, OutputPer1M: 9.00,
			QualityCode: 0.88, QualityReason: 0.88, SpeedRating: 0.91,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"fast", "long-context"},
		},
		{
			ID: "google/gemini-3.1-pro-preview", Name: "Gemini 3.1 Pro Preview", Org: "google",
			ContextWindow: 1048576, InputPer1M: 2.00, OutputPer1M: 12.00,
			QualityCode: 0.90, QualityReason: 0.90, SpeedRating: 0.74,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"flagship", "long-context"},
		},
		{
			ID: "mistralai/mistral-medium-3-5", Name: "Mistral Medium 3.5", Org: "mistralai",
			ContextWindow: 262144, InputPer1M: 1.50, OutputPer1M: 7.50,
			QualityCode: 0.86, QualityReason: 0.85, SpeedRating: 0.83,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"coding"},
		},
		{
			ID: "qwen/qwen3-235b-a22b", Name: "Qwen3 235B A22B", Org: "qwen",
			ContextWindow: 131072, InputPer1M: 0.455, OutputPer1M: 1.82,
			QualityCode: 0.88, QualityReason: 0.90, SpeedRating: 0.72,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"large", "reasoning"},
		},
		{
			ID: "qwen/qwen3-coder-plus", Name: "Qwen3 Coder Plus", Org: "qwen",
			ContextWindow: 1000000, InputPer1M: 0.65, OutputPer1M: 3.25,
			QualityCode: 0.89, QualityReason: 0.85, SpeedRating: 0.80,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"coding", "long-context"},
		},
		{
			ID: "moonshotai/kimi-k2.6", Name: "Kimi K2.6", Org: "moonshotai",
			ContextWindow: 262144, InputPer1M: 0.684, OutputPer1M: 3.42,
			QualityCode: 0.86, QualityReason: 0.87, SpeedRating: 0.80,
			Tier: "pro", SupportsTools: true,
			Tags: []string{"coding", "agentic"},
		},

		// ── BALANCED TIER ───────────────────────────────────────────────────────
		{
			ID: "openai/gpt-5-mini", Name: "GPT-5 Mini", Org: "openai",
			ContextWindow: 400000, InputPer1M: 0.25, OutputPer1M: 2.00,
			QualityCode: 0.85, QualityReason: 0.83, SpeedRating: 0.89,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"fast", "cost-effective"},
		},
		{
			ID: "openai/gpt-5.1-codex-mini", Name: "GPT-5.1 Codex Mini", Org: "openai",
			ContextWindow: 400000, InputPer1M: 0.25, OutputPer1M: 2.00,
			QualityCode: 0.86, QualityReason: 0.83, SpeedRating: 0.90,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"coding", "fast"},
		},
		{
			ID: "openai/gpt-4.1-mini", Name: "GPT-4.1 Mini", Org: "openai",
			ContextWindow: 1047576, InputPer1M: 0.40, OutputPer1M: 1.60,
			QualityCode: 0.83, QualityReason: 0.81, SpeedRating: 0.91,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"fast", "long-context"},
		},
		{
			ID: "google/gemini-2.5-flash", Name: "Gemini 2.5 Flash", Org: "google",
			ContextWindow: 1048576, InputPer1M: 0.30, OutputPer1M: 2.50,
			QualityCode: 0.85, QualityReason: 0.86, SpeedRating: 0.92,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"fast", "long-context", "cost-effective"},
		},
		{
			ID: "google/gemini-2.5-flash-lite", Name: "Gemini 2.5 Flash Lite", Org: "google",
			ContextWindow: 1048576, InputPer1M: 0.10, OutputPer1M: 0.40,
			QualityCode: 0.80, QualityReason: 0.79, SpeedRating: 0.96,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"fast", "cheap", "long-context"},
		},
		{
			ID: "deepseek/deepseek-v3.2", Name: "DeepSeek V3.2", Org: "deepseek",
			ContextWindow: 131072, InputPer1M: 0.2288, OutputPer1M: 0.3432,
			QualityCode: 0.88, QualityReason: 0.86, SpeedRating: 0.82,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"coding", "cost-effective"},
		},
		{
			ID: "deepseek/deepseek-v4-flash", Name: "DeepSeek V4 Flash", Org: "deepseek",
			ContextWindow: 1048576, InputPer1M: 0.0983, OutputPer1M: 0.1966,
			QualityCode: 0.85, QualityReason: 0.83, SpeedRating: 0.91,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"coding", "fast", "cheap", "long-context"},
		},
		{
			ID: "deepseek/deepseek-chat-v3.1", Name: "DeepSeek Chat V3.1", Org: "deepseek",
			ContextWindow: 163840, InputPer1M: 0.21, OutputPer1M: 0.79,
			QualityCode: 0.87, QualityReason: 0.85, SpeedRating: 0.83,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"coding", "chat"},
		},
		{
			ID: "qwen/qwen3-coder", Name: "Qwen3 Coder", Org: "qwen",
			ContextWindow: 1048576, InputPer1M: 0.22, OutputPer1M: 1.80,
			QualityCode: 0.87, QualityReason: 0.85, SpeedRating: 0.83,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"coding", "long-context"},
		},
		{
			ID: "qwen/qwen3-max", Name: "Qwen3 Max", Org: "qwen",
			ContextWindow: 262144, InputPer1M: 0.78, OutputPer1M: 3.90,
			QualityCode: 0.87, QualityReason: 0.88, SpeedRating: 0.78,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"reasoning"},
		},
		{
			ID: "meta-llama/llama-4-maverick", Name: "Llama 4 Maverick", Org: "meta-llama",
			ContextWindow: 1048576, InputPer1M: 0.15, OutputPer1M: 0.60,
			QualityCode: 0.85, QualityReason: 0.84, SpeedRating: 0.87,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"open", "cost-effective", "long-context"},
		},
		{
			ID: "meta-llama/llama-4-scout", Name: "Llama 4 Scout", Org: "meta-llama",
			ContextWindow: 10000000, InputPer1M: 0.08, OutputPer1M: 0.30,
			QualityCode: 0.82, QualityReason: 0.80, SpeedRating: 0.90,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"open", "massive-context"},
		},
		{
			ID: "mistralai/devstral-2512", Name: "Devstral 2512", Org: "mistralai",
			ContextWindow: 262144, InputPer1M: 0.40, OutputPer1M: 2.00,
			QualityCode: 0.86, QualityReason: 0.83, SpeedRating: 0.84,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"coding", "agentic"},
		},
		{
			ID: "mistralai/codestral-2508", Name: "Codestral 2508", Org: "mistralai",
			ContextWindow: 256000, InputPer1M: 0.30, OutputPer1M: 0.90,
			QualityCode: 0.85, QualityReason: 0.82, SpeedRating: 0.86,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"coding"},
		},
		{
			ID: "mistralai/mistral-large-2512", Name: "Mistral Large 2512", Org: "mistralai",
			ContextWindow: 262144, InputPer1M: 0.50, OutputPer1M: 1.50,
			QualityCode: 0.84, QualityReason: 0.84, SpeedRating: 0.85,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"reasoning"},
		},
		{
			ID: "amazon/nova-premier-v1", Name: "Nova Premier V1", Org: "amazon",
			ContextWindow: 1000000, InputPer1M: 2.50, OutputPer1M: 12.50,
			QualityCode: 0.86, QualityReason: 0.85, SpeedRating: 0.80,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"flagship", "long-context"},
		},
		{
			ID: "amazon/nova-pro-v1", Name: "Nova Pro V1", Org: "amazon",
			ContextWindow: 300000, InputPer1M: 0.80, OutputPer1M: 3.20,
			QualityCode: 0.83, QualityReason: 0.82, SpeedRating: 0.84,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"coding"},
		},

		// ── FAST TIER ───────────────────────────────────────────────────────────
		{
			ID: "openai/gpt-5-nano", Name: "GPT-5 Nano", Org: "openai",
			ContextWindow: 400000, InputPer1M: 0.05, OutputPer1M: 0.40,
			QualityCode: 0.78, QualityReason: 0.76, SpeedRating: 0.97,
			Tier: "fast", SupportsTools: true,
			Tags: []string{"fast", "cheap"},
		},
		{
			ID: "openai/gpt-4.1-nano", Name: "GPT-4.1 Nano", Org: "openai",
			ContextWindow: 1047576, InputPer1M: 0.10, OutputPer1M: 0.40,
			QualityCode: 0.76, QualityReason: 0.74, SpeedRating: 0.97,
			Tier: "fast", SupportsTools: true,
			Tags: []string{"fast", "cheap", "long-context"},
		},
		{
			ID: "qwen/qwen3-coder-flash", Name: "Qwen3 Coder Flash", Org: "qwen",
			ContextWindow: 1000000, InputPer1M: 0.195, OutputPer1M: 0.975,
			QualityCode: 0.82, QualityReason: 0.80, SpeedRating: 0.91,
			Tier: "fast", SupportsTools: true,
			Tags: []string{"coding", "fast", "long-context"},
		},
		{
			ID: "google/gemini-3.1-flash-lite", Name: "Gemini 3.1 Flash Lite", Org: "google",
			ContextWindow: 1048576, InputPer1M: 0.25, OutputPer1M: 1.50,
			QualityCode: 0.80, QualityReason: 0.79, SpeedRating: 0.95,
			Tier: "fast", SupportsTools: true,
			Tags: []string{"fast", "cheap", "long-context"},
		},
		{
			ID: "mistralai/mistral-small-3.2-24b-instruct", Name: "Mistral Small 3.2", Org: "mistralai",
			ContextWindow: 128000, InputPer1M: 0.075, OutputPer1M: 0.20,
			QualityCode: 0.77, QualityReason: 0.75, SpeedRating: 0.93,
			Tier: "fast", SupportsTools: true,
			Tags: []string{"fast", "cheap"},
		},
		{
			ID: "meta-llama/llama-3.3-70b-instruct", Name: "Llama 3.3 70B", Org: "meta-llama",
			ContextWindow: 131072, InputPer1M: 0.10, OutputPer1M: 0.32,
			QualityCode: 0.80, QualityReason: 0.79, SpeedRating: 0.88,
			Tier: "fast", SupportsTools: true,
			Tags: []string{"open", "fast"},
		},
		{
			ID: "amazon/nova-lite-v1", Name: "Nova Lite V1", Org: "amazon",
			ContextWindow: 300000, InputPer1M: 0.06, OutputPer1M: 0.24,
			QualityCode: 0.74, QualityReason: 0.72, SpeedRating: 0.94,
			Tier: "fast", SupportsTools: true,
			Tags: []string{"fast", "cheap"},
		},
		{
			ID: "amazon/nova-micro-v1", Name: "Nova Micro V1", Org: "amazon",
			ContextWindow: 128000, InputPer1M: 0.035, OutputPer1M: 0.14,
			QualityCode: 0.70, QualityReason: 0.68, SpeedRating: 0.97,
			Tier: "fast", SupportsTools: true,
			Tags: []string{"fast", "ultra-cheap"},
		},
		{
			ID: "microsoft/phi-4", Name: "Phi-4", Org: "microsoft",
			ContextWindow: 16384, InputPer1M: 0.065, OutputPer1M: 0.14,
			QualityCode: 0.75, QualityReason: 0.76, SpeedRating: 0.93,
			Tier: "fast", SupportsTools: false,
			Tags: []string{"small", "fast", "cheap"},
		},
		{
			ID: "openai/gpt-4o-mini", Name: "GPT-4o Mini", Org: "openai",
			ContextWindow: 128000, InputPer1M: 0.15, OutputPer1M: 0.60,
			QualityCode: 0.79, QualityReason: 0.77, SpeedRating: 0.93,
			Tier: "fast", SupportsTools: true,
			Tags: []string{"fast", "cost-effective"},
		},
		{
			ID: "openai/gpt-4o", Name: "GPT-4o", Org: "openai",
			ContextWindow: 128000, InputPer1M: 2.50, OutputPer1M: 10.00,
			QualityCode: 0.86, QualityReason: 0.85, SpeedRating: 0.85,
			Tier: "balanced", SupportsTools: true,
			Tags: []string{"multimodal", "stable"},
		},

		// ── FREE TIER ───────────────────────────────────────────────────────────
		{
			ID: "moonshotai/kimi-k2.6:free", Name: "Kimi K2.6 (Free)", Org: "moonshotai",
			ContextWindow: 262144, IsFree: true,
			QualityCode: 0.86, QualityReason: 0.87, SpeedRating: 0.70,
			Tier: "free", SupportsTools: true,
			Tags: []string{"free", "coding", "agentic"},
		},
		{
			ID: "openai/gpt-oss-120b:free", Name: "GPT OSS 120B (Free)", Org: "openai",
			ContextWindow: 131072, IsFree: true,
			QualityCode: 0.83, QualityReason: 0.82, SpeedRating: 0.72,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free", "coding"},
		},
		{
			ID: "openai/gpt-oss-20b:free", Name: "GPT OSS 20B (Free)", Org: "openai",
			ContextWindow: 131072, IsFree: true,
			QualityCode: 0.76, QualityReason: 0.74, SpeedRating: 0.85,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free", "fast"},
		},
		{
			ID: "qwen/qwen3-coder:free", Name: "Qwen3 Coder (Free)", Org: "qwen",
			ContextWindow: 1048576, IsFree: true,
			QualityCode: 0.82, QualityReason: 0.80, SpeedRating: 0.74,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free", "coding", "long-context"},
		},
		{
			ID: "nvidia/nemotron-3-super-120b-a12b:free", Name: "Nemotron 3 Super 120B (Free)", Org: "nvidia",
			ContextWindow: 1000000, IsFree: true,
			QualityCode: 0.82, QualityReason: 0.81, SpeedRating: 0.70,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free", "long-context"},
		},
		{
			ID: "meta-llama/llama-3.3-70b-instruct:free", Name: "Llama 3.3 70B (Free)", Org: "meta-llama",
			ContextWindow: 131072, IsFree: true,
			QualityCode: 0.80, QualityReason: 0.79, SpeedRating: 0.75,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free", "coding", "open"},
		},
		{
			ID: "nousresearch/hermes-3-llama-3.1-405b:free", Name: "Hermes 3 405B (Free)", Org: "nousresearch",
			ContextWindow: 131072, IsFree: true,
			QualityCode: 0.79, QualityReason: 0.78, SpeedRating: 0.62,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free"},
		},
		{
			ID: "qwen/qwen3-next-80b-a3b-instruct:free", Name: "Qwen3 Next 80B (Free)", Org: "qwen",
			ContextWindow: 262144, IsFree: true,
			QualityCode: 0.81, QualityReason: 0.80, SpeedRating: 0.72,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free", "coding"},
		},
		{
			ID: "google/gemma-4-31b-it:free", Name: "Gemma 4 31B (Free)", Org: "google",
			ContextWindow: 262144, IsFree: true,
			QualityCode: 0.78, QualityReason: 0.77, SpeedRating: 0.78,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free"},
		},
		{
			ID: "google/gemma-4-26b-a4b-it:free", Name: "Gemma 4 26B MoE (Free)", Org: "google",
			ContextWindow: 262144, IsFree: true,
			QualityCode: 0.77, QualityReason: 0.76, SpeedRating: 0.82,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free"},
		},
		{
			ID: "z-ai/glm-4.5-air:free", Name: "GLM-4.5 Air (Free)", Org: "z-ai",
			ContextWindow: 131072, IsFree: true,
			QualityCode: 0.77, QualityReason: 0.76, SpeedRating: 0.80,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free"},
		},
		{
			ID: "nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free", Name: "Nemotron Nano Omni 30B (Free)", Org: "nvidia",
			ContextWindow: 256000, IsFree: true,
			QualityCode: 0.76, QualityReason: 0.78, SpeedRating: 0.80,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free", "reasoning"},
		},
		{
			ID: "nvidia/nemotron-3-nano-30b-a3b:free", Name: "Nemotron Nano 30B (Free)", Org: "nvidia",
			ContextWindow: 256000, IsFree: true,
			QualityCode: 0.75, QualityReason: 0.73, SpeedRating: 0.82,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free"},
		},
		{
			ID: "poolside/laguna-m.1:free", Name: "Laguna M.1 (Free)", Org: "poolside",
			ContextWindow: 262144, IsFree: true,
			QualityCode: 0.80, QualityReason: 0.79, SpeedRating: 0.76,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free", "coding"},
		},
		{
			ID: "poolside/laguna-xs.2:free", Name: "Laguna XS.2 (Free)", Org: "poolside",
			ContextWindow: 262144, IsFree: true,
			QualityCode: 0.74, QualityReason: 0.72, SpeedRating: 0.88,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free", "coding", "fast"},
		},
		{
			ID: "meta-llama/llama-3.2-3b-instruct:free", Name: "Llama 3.2 3B (Free)", Org: "meta-llama",
			ContextWindow: 131072, IsFree: true,
			QualityCode: 0.62, QualityReason: 0.60, SpeedRating: 0.97,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free", "tiny", "fast"},
		},
		{
			ID: "cognitivecomputations/dolphin-mistral-24b-venice-edition:free", Name: "Dolphin Mistral 24B (Free)", Org: "cognitivecomputations",
			ContextWindow: 32768, IsFree: true,
			QualityCode: 0.72, QualityReason: 0.70, SpeedRating: 0.84,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free", "uncensored"},
		},
		{
			ID: "liquid/lfm-2.5-1.2b-instruct:free", Name: "LFM 2.5 1.2B (Free)", Org: "liquid",
			ContextWindow: 32768, IsFree: true,
			QualityCode: 0.55, QualityReason: 0.53, SpeedRating: 0.99,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free", "tiny", "ultra-fast"},
		},
		{
			ID: "openrouter/free", Name: "OpenRouter Free", Org: "openrouter",
			ContextWindow: 200000, IsFree: true,
			QualityCode: 0.75, QualityReason: 0.73, SpeedRating: 0.80,
			Tier: "free", SupportsTools: false,
			Tags: []string{"free"},
		},
	}
	return all
}

// CatalogByID returns a map of model ID → CatalogModel for O(1) lookups.
func CatalogByID() map[string]CatalogModel {
	m := make(map[string]CatalogModel)
	for _, model := range OpenRouterCatalog() {
		m[model.ID] = model
	}
	return m
}

// FreeCatalogModels returns only the free models from the catalog.
func FreeCatalogModels() []CatalogModel {
	var result []CatalogModel
	for _, m := range OpenRouterCatalog() {
		if m.IsFree {
			result = append(result, m)
		}
	}
	return result
}

// BestFreeModel returns the highest-quality free model for a given capability.
func BestFreeModel(cap AICapability) CatalogModel {
	var best CatalogModel
	for _, m := range OpenRouterCatalog() {
		if !m.IsFree {
			continue
		}
		score := catalogScore(m, cap)
		if best.ID == "" || score > catalogScore(best, cap) {
			best = m
		}
	}
	// Default fallback
	if best.ID == "" {
		return CatalogModel{ID: "meta-llama/llama-3.3-70b-instruct:free", Name: "Llama 3.3 70B (Free)", IsFree: true}
	}
	return best
}

// catalogScore computes a quality-priority-over-cost score for a model + capability.
// Higher is better. quality² / (1 + log2(cost_per_call + 1))
func catalogScore(m CatalogModel, cap AICapability) float64 {
	quality := m.QualityCode
	if isReasoningCapability(cap) {
		quality = m.QualityReason
	}
	// Avoid divide-by-zero; free models get full quality score since cost=0
	if m.IsFree {
		return quality * quality
	}
	// Estimated cost per typical call: 2000 input tokens + 1500 output tokens
	costPerCall := (m.InputPer1M * 2000 / 1_000_000) + (m.OutputPer1M * 1500 / 1_000_000)
	import_log2 := 1.0
	if costPerCall > 0 {
		import_log2 = 1.0 + log2Approx(costPerCall+1)
	}
	return (quality * quality) / import_log2
}

func isReasoningCapability(cap AICapability) bool {
	return cap == CapabilityArchitecture || cap == CapabilityDebugging || cap == CapabilityRefactoring
}

// log2Approx is a fast log2 approximation avoiding math import in this file.
func log2Approx(x float64) float64 {
	if x <= 1 {
		return 0
	}
	// ln(x)/ln(2) approximation using iterative halving
	result := 0.0
	for x > 2 {
		x /= 2
		result++
	}
	result += x - 1 // linear approximation of log2 in [1,2]
	return result
}
