package ai

import (
	"context"
	"time"
)

// AIProvider represents the available AI providers
type AIProvider string

const (
	ProviderClaude      AIProvider = "claude"
	ProviderGPT4        AIProvider = "gpt4"
	ProviderGemini      AIProvider = "gemini"
	ProviderGrok        AIProvider = "grok"
	ProviderOllama      AIProvider = "ollama"       // local Ollama (BYOK)
	ProviderOllamaCloud AIProvider = "ollama_cloud" // Ollama Cloud hosted models (Pro+)
)

// AICapability represents different AI use cases
type AICapability string

const (
	CapabilityCodeGeneration        AICapability = "code_generation"
	CapabilityNaturalLanguageToCode AICapability = "natural_language_to_code"
	CapabilityCodeReview            AICapability = "code_review"
	CapabilityCodeCompletion        AICapability = "code_completion"
	CapabilityDebugging             AICapability = "debugging"
	CapabilityExplanation           AICapability = "explanation"
	CapabilityRefactoring           AICapability = "refactoring"
	CapabilityTesting               AICapability = "testing"
	CapabilityDocumentation         AICapability = "documentation"
	CapabilityArchitecture          AICapability = "architecture"
)

// AIRequest represents a request to an AI provider
type AIRequest struct {
	ID                 string                 `json:"id"`
	Provider           AIProvider             `json:"provider"`
	Model              string                 `json:"model,omitempty"` // Explicit model override (e.g. "grok-3", "claude-sonnet-4-6")
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
	Provider     AIProvider `json:"provider"`
	RequestCount int64      `json:"request_count"`
	TotalTokens  int64      `json:"total_tokens"`
	TotalCost    float64    `json:"total_cost"`
	AvgLatency   float64    `json:"avg_latency"`
	ErrorCount   int64      `json:"error_count"`
	LastUsed     time.Time  `json:"last_used"`
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

	// BYOK emergency fallback settings
	// Controls when BYOK providers can fallback to platform providers
	EnableBYOKEmergencyFallback bool `json:"enable_byok_emergency_fallback"`

	// Retry attempts before using emergency fallback
	MaxRetryAttempts map[AIProvider]int `json:"max_retry_attempts"`
}

// DefaultRouterConfig returns the optimal routing configuration
func DefaultRouterConfig() *RouterConfig {
	return &RouterConfig{
		DefaultProviders: map[AICapability]AIProvider{
			// Ollama (kimi-k2.6:cloud) is the primary orchestrator for all capabilities.
			// Cloud providers are fallback only.
			CapabilityCodeGeneration:        ProviderOllama,
			CapabilityNaturalLanguageToCode: ProviderOllama,
			CapabilityCodeReview:            ProviderOllama,
			CapabilityCodeCompletion:        ProviderOllama,
			CapabilityDebugging:             ProviderOllama,
			CapabilityExplanation:           ProviderOllama,
			CapabilityRefactoring:           ProviderOllama,
			CapabilityTesting:               ProviderOllama,
			CapabilityDocumentation:         ProviderOllama,
			CapabilityArchitecture:          ProviderOllama,
		},
		FallbackOrder: map[AIProvider][]AIProvider{
			// Ollama (kimi-k2.6:cloud) is the primary orchestrator.
			// If Ollama fails, fall back to Claude, then GPT4, then Gemini, then Grok.
			ProviderOllama: {ProviderClaude, ProviderGPT4, ProviderGemini, ProviderGrok},
			ProviderClaude: {ProviderOllama, ProviderGPT4, ProviderGemini, ProviderGrok},
			ProviderGPT4:   {ProviderOllama, ProviderClaude, ProviderGemini, ProviderGrok},
			ProviderGemini: {ProviderOllama, ProviderClaude, ProviderGPT4, ProviderGrok},
			ProviderGrok:   {ProviderOllama, ProviderGPT4, ProviderClaude, ProviderGemini},
		},
		LoadBalancing: map[AIProvider]float64{
			// Ollama (kimi-k2.6:cloud) is the primary orchestrator — highest weight.
			ProviderOllama:      0.50,
			ProviderOllamaCloud: 0.40,
			ProviderClaude:      0.15,
			ProviderGPT4:        0.15,
			ProviderGrok:        0.10,
			ProviderGemini:      0.10,
		},
		RateLimits: map[AIProvider]int{
			ProviderClaude:      100,  // requests per minute
			ProviderGPT4:        80,   // requests per minute
			ProviderGemini:      120,  // requests per minute
			ProviderGrok:        100,  // requests per minute
			ProviderOllama:      1000, // Local — no real limit
			ProviderOllamaCloud: 60,   // Ollama Cloud — API rate limit
		},
		CostThresholds: map[AIProvider]float64{
			ProviderClaude:      0.10, // max cost per request
			ProviderGPT4:        0.15, // max cost per request
			ProviderGemini:      0.08, // max cost per request
			ProviderGrok:        0.05, // max cost per request
			ProviderOllama:      0.00, // Free — runs locally
			ProviderOllamaCloud: 0.00, // Flat-rate subscription — no per-request cost
		},
		// Enable emergency fallback for BYOK scenarios to prevent build failures
		EnableBYOKEmergencyFallback: true,
		// Retry local models multiple times before falling back to cloud
		MaxRetryAttempts: map[AIProvider]int{
			ProviderClaude:      2, // Cloud providers get fewer retries
			ProviderGPT4:        2, // Cloud providers get fewer retries
			ProviderGemini:      2, // Cloud providers get fewer retries
			ProviderGrok:        2, // Cloud providers get fewer retries
			ProviderOllama:      5, // Local model gets more retries (network/startup issues)
			ProviderOllamaCloud: 3, // Ollama Cloud — moderate retries
		},
	}
}
