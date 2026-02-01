package ai

import (
	"context"
	"time"
)

// AIProvider represents the available AI providers
type AIProvider string

const (
	ProviderClaude   AIProvider = "claude"
	ProviderGPT4     AIProvider = "gpt4"
	ProviderGemini   AIProvider = "gemini"
	ProviderGrok     AIProvider = "grok"
	ProviderOllama   AIProvider = "ollama"
)

// AICapability represents different AI use cases
type AICapability string

const (
	CapabilityCodeGeneration       AICapability = "code_generation"
	CapabilityNaturalLanguageToCode AICapability = "natural_language_to_code"
	CapabilityCodeReview          AICapability = "code_review"
	CapabilityCodeCompletion      AICapability = "code_completion"
	CapabilityDebugging           AICapability = "debugging"
	CapabilityExplanation         AICapability = "explanation"
	CapabilityRefactoring         AICapability = "refactoring"
	CapabilityTesting             AICapability = "testing"
	CapabilityDocumentation       AICapability = "documentation"
	CapabilityArchitecture        AICapability = "architecture"
)

// AIRequest represents a request to an AI provider
type AIRequest struct {
	ID                 string                 `json:"id"`
	Provider           AIProvider             `json:"provider"`
	Model              string                 `json:"model,omitempty"`     // Explicit model override (e.g. "grok-4-fast", "claude-sonnet-4-20250514")
	Capability         AICapability           `json:"capability"`
	Prompt             string                 `json:"prompt"`
	Code               string                 `json:"code,omitempty"`
	Language           string                 `json:"language,omitempty"`
	Context            map[string]interface{} `json:"context,omitempty"`
	MaxTokens          int                    `json:"max_tokens,omitempty"`
	Temperature        float32                `json:"temperature,omitempty"`
	UserID             string                 `json:"user_id"`
	ProjectID          string                 `json:"project_id,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
	MaxResponseTime    time.Duration          `json:"max_response_time,omitempty"`
	QualityRequirement float64                `json:"quality_requirement,omitempty"`
	MaxCost            float64                `json:"max_cost,omitempty"`
}

// GetCacheKey generates a cache key for the request
func (r *AIRequest) GetCacheKey() string {
	return r.ID + "_" + string(r.Provider) + "_" + string(r.Capability)
}

// AIResponse represents a response from an AI provider
type AIResponse struct {
	ID             string                 `json:"id"`
	Provider       AIProvider             `json:"provider"`
	Content        string                 `json:"content"`
	Usage          *Usage                 `json:"usage,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Error          string                 `json:"error,omitempty"`
	Duration       time.Duration          `json:"duration"`
	CreatedAt      time.Time              `json:"created_at"`
	Quality        float64                `json:"quality,omitempty"`
	GenerationTime time.Duration          `json:"generation_time,omitempty"`
}

// Cost returns the cost of the response based on usage
func (r *AIResponse) Cost() float64 {
	if r.Usage != nil {
		return r.Usage.Cost
	}
	return 0.0
}

// Usage represents token/cost usage for an AI request
type Usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Cost             float64 `json:"cost"`
}

// AIClient interface that all AI providers must implement
type AIClient interface {
	// Generate generates content based on the request
	Generate(ctx context.Context, req *AIRequest) (*AIResponse, error)

	// GetCapabilities returns the capabilities this provider supports
	GetCapabilities() []AICapability

	// GetProvider returns the provider identifier
	GetProvider() AIProvider

	// Health checks if the provider is healthy
	Health(ctx context.Context) error

	// GetUsage returns usage statistics
	GetUsage() *ProviderUsage
}

// ProviderUsage tracks usage statistics for a provider
type ProviderUsage struct {
	Provider      AIProvider `json:"provider"`
	RequestCount  int64      `json:"request_count"`
	TotalTokens   int64      `json:"total_tokens"`
	TotalCost     float64    `json:"total_cost"`
	AvgLatency    float64    `json:"avg_latency"`
	ErrorCount    int64      `json:"error_count"`
	LastUsed      time.Time  `json:"last_used"`
}

// RouterConfig configures how requests are routed to providers
type RouterConfig struct {
	// Default provider preferences for each capability
	DefaultProviders map[AICapability]AIProvider `json:"default_providers"`

	// Fallback order when primary provider fails
	FallbackOrder map[AIProvider][]AIProvider `json:"fallback_order"`

	// Load balancing weights (0.0 to 1.0)
	LoadBalancing map[AIProvider]float64 `json:"load_balancing"`

	// Rate limits per provider (requests per minute)
	RateLimits map[AIProvider]int `json:"rate_limits"`

	// Cost thresholds for switching providers
	CostThresholds map[AIProvider]float64 `json:"cost_thresholds"`
}

// DefaultRouterConfig returns the optimal routing configuration
// NOTE: Default providers are intentionally empty - the router will use the first
// available provider dynamically. This prevents failures when Ollama isn't configured.
// If Ollama IS configured (via OLLAMA_BASE_URL or BYOK), it will be prioritized.
func DefaultRouterConfig() *RouterConfig {
	return &RouterConfig{
		DefaultProviders: map[AICapability]AIProvider{
			// Empty map = router uses first available provider with fallback chain
			// Priority order: Ollama (if configured) > Claude > GPT4 > Gemini > Grok
		},
		FallbackOrder: map[AIProvider][]AIProvider{
			ProviderClaude: {ProviderGPT4, ProviderGrok, ProviderOllama, ProviderGemini},
			ProviderGPT4:   {ProviderClaude, ProviderGrok, ProviderOllama, ProviderGemini},
			ProviderGemini: {ProviderGrok, ProviderOllama, ProviderGPT4, ProviderClaude},
			ProviderGrok:   {ProviderOllama, ProviderGPT4, ProviderClaude, ProviderGemini},
			ProviderOllama: {}, // No fallbacks for local/free model to prevent accidental costs
		},
		LoadBalancing: map[AIProvider]float64{
			ProviderClaude: 0.25,
			ProviderGPT4:   0.25,
			ProviderGrok:   0.20,
			ProviderGemini: 0.15,
			ProviderOllama: 0.15,
		},
		RateLimits: map[AIProvider]int{
			ProviderClaude: 100,  // requests per minute
			ProviderGPT4:   80,   // requests per minute
			ProviderGemini: 120,  // requests per minute
			ProviderGrok:   100,  // requests per minute
			ProviderOllama: 1000, // Local — no real limit
		},
		CostThresholds: map[AIProvider]float64{
			ProviderClaude: 0.10,  // max cost per request
			ProviderGPT4:   0.15,  // max cost per request
			ProviderGemini: 0.08,  // max cost per request
			ProviderGrok:   0.05,  // max cost per request
			ProviderOllama: 0.00,  // Free — runs locally
		},
	}
}

// BYOKRouterConfig returns a strict BYOK configuration with NO fallbacks.
// When BYOK is active, users should ONLY use their configured providers.
// If a requested provider isn't available, the request fails instead of falling back.
func BYOKRouterConfig(configuredProviders []AIProvider) *RouterConfig {
	// Create empty fallback chains - NO fallbacks in BYOK mode
	fallbackOrder := make(map[AIProvider][]AIProvider)
	loadBalancing := make(map[AIProvider]float64)
	rateLimits := make(map[AIProvider]int)
	costThresholds := make(map[AIProvider]float64)

	// Only configure the providers the user has set up
	for _, provider := range configuredProviders {
		fallbackOrder[provider] = []AIProvider{} // NO fallbacks
		loadBalancing[provider] = 1.0 / float64(len(configuredProviders)) // Equal weight
		rateLimits[provider] = 1000 // High limit for user's own keys
		costThresholds[provider] = 1.0 // User pays their own costs
	}

	return &RouterConfig{
		DefaultProviders: map[AICapability]AIProvider{
			// Empty - let the router pick from configured providers
		},
		FallbackOrder:   fallbackOrder,
		LoadBalancing:   loadBalancing,
		RateLimits:      rateLimits,
		CostThresholds:  costThresholds,
	}
}