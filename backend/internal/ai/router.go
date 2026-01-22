package ai

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AIRouter intelligently routes AI requests to the optimal provider
type AIRouter struct {
	clients     map[AIProvider]AIClient
	config      *RouterConfig
	rateLimits  map[AIProvider]*rateLimiter
	mu          sync.RWMutex
	healthCheck map[AIProvider]bool
}

// rateLimiter tracks rate limiting for each provider
type rateLimiter struct {
	tokens    int
	maxTokens int
	lastRefill time.Time
	mu        sync.Mutex
}

// NewAIRouter creates a new AI router with multiple providers
func NewAIRouter(claudeKey, openAIKey, geminiKey string) *AIRouter {
	clients := make(map[AIProvider]AIClient)

	// Initialize all AI clients
	if claudeKey != "" {
		clients[ProviderClaude] = NewClaudeClient(claudeKey)
	}
	if openAIKey != "" {
		clients[ProviderGPT4] = NewOpenAIClient(openAIKey)
	}
	if geminiKey != "" {
		clients[ProviderGemini] = NewGeminiClient(geminiKey)
	}

	config := DefaultRouterConfig()

	// Initialize rate limiters
	rateLimits := make(map[AIProvider]*rateLimiter)
	for provider, limit := range config.RateLimits {
		rateLimits[provider] = &rateLimiter{
			tokens:     limit,
			maxTokens:  limit,
			lastRefill: time.Now(),
		}
	}

	router := &AIRouter{
		clients:     clients,
		config:      config,
		rateLimits:  rateLimits,
		healthCheck: make(map[AIProvider]bool),
	}

	// Start health monitoring
	go router.monitorHealth()

	return router
}

// Generate routes an AI request to the optimal provider
func (r *AIRouter) Generate(ctx context.Context, req *AIRequest) (*AIResponse, error) {
	// Set request ID if not provided
	if req.ID == "" {
		req.ID = uuid.New().String()
	}

	// Set default temperature if not provided
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}

	// Select the best provider for this request
	provider, err := r.selectProvider(req)
	if err != nil {
		return nil, fmt.Errorf("failed to select provider: %w", err)
	}

	// Get the client for the selected provider
	client, exists := r.clients[provider]
	if !exists {
		return nil, fmt.Errorf("client not available for provider: %s", provider)
	}

	// Check rate limiting
	if !r.checkRateLimit(provider) {
		// Try fallback providers
		fallbacks := r.config.FallbackOrder[provider]
		for _, fallbackProvider := range fallbacks {
			if r.checkRateLimit(fallbackProvider) {
				if fallbackClient, exists := r.clients[fallbackProvider]; exists {
					log.Printf("Rate limited for %s, using fallback %s", provider, fallbackProvider)
					return fallbackClient.Generate(ctx, req)
				}
			}
		}
		return nil, fmt.Errorf("rate limit exceeded for all available providers")
	}

	// Attempt to generate with primary provider
	response, err := client.Generate(ctx, req)
	if err != nil {
		log.Printf("Error from provider %s: %v", provider, err)

		// Try fallback providers
		fallbacks := r.config.FallbackOrder[provider]
		for _, fallbackProvider := range fallbacks {
			if fallbackClient, exists := r.clients[fallbackProvider]; exists {
				if r.checkRateLimit(fallbackProvider) && r.isHealthy(fallbackProvider) {
					log.Printf("Falling back to provider %s", fallbackProvider)
					fallbackResponse, fallbackErr := fallbackClient.Generate(ctx, req)
					if fallbackErr == nil {
						return fallbackResponse, nil
					}
					log.Printf("Fallback provider %s also failed: %v", fallbackProvider, fallbackErr)
				}
			}
		}

		return nil, fmt.Errorf("all providers failed, last error: %w", err)
	}

	return response, nil
}

// selectProvider chooses the optimal provider for a request
func (r *AIRouter) selectProvider(req *AIRequest) (AIProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Start with the default provider for this capability
	defaultProvider, exists := r.config.DefaultProviders[req.Capability]
	if !exists {
		// If no default, use load balancing
		return r.selectByLoadBalancing()
	}

	// Check if default provider is healthy and available
	if r.isHealthy(defaultProvider) && r.isAvailable(defaultProvider) {
		return defaultProvider, nil
	}

	// If default provider is not available, try fallbacks
	fallbacks := r.config.FallbackOrder[defaultProvider]
	for _, provider := range fallbacks {
		if r.isHealthy(provider) && r.isAvailable(provider) {
			return provider, nil
		}
	}

	// If no fallbacks work, use load balancing among healthy providers
	return r.selectByLoadBalancing()
}

// selectByLoadBalancing selects provider based on load balancing weights
func (r *AIRouter) selectByLoadBalancing() (AIProvider, error) {
	healthyProviders := []AIProvider{}
	totalWeight := 0.0

	// Collect healthy providers and their weights
	for provider, weight := range r.config.LoadBalancing {
		if r.isHealthy(provider) && r.isAvailable(provider) {
			healthyProviders = append(healthyProviders, provider)
			totalWeight += weight
		}
	}

	if len(healthyProviders) == 0 {
		return "", fmt.Errorf("no healthy providers available")
	}

	// Select provider based on weighted random selection
	randomValue := rand.Float64() * totalWeight
	currentWeight := 0.0

	for _, provider := range healthyProviders {
		currentWeight += r.config.LoadBalancing[provider]
		if randomValue <= currentWeight {
			return provider, nil
		}
	}

	// Fallback to first healthy provider
	return healthyProviders[0], nil
}

// checkRateLimit checks if a provider is within rate limits
func (r *AIRouter) checkRateLimit(provider AIProvider) bool {
	limiter, exists := r.rateLimits[provider]
	if !exists {
		return true
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	// Refill tokens based on time passed
	now := time.Now()
	timePassed := now.Sub(limiter.lastRefill)
	tokensToAdd := int(timePassed.Minutes()) * limiter.maxTokens

	if tokensToAdd > 0 {
		limiter.tokens = min(limiter.maxTokens, limiter.tokens+tokensToAdd)
		limiter.lastRefill = now
	}

	// Check if we have tokens available
	if limiter.tokens > 0 {
		limiter.tokens--
		return true
	}

	return false
}

// isHealthy checks if a provider is healthy
func (r *AIRouter) isHealthy(provider AIProvider) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	health, exists := r.healthCheck[provider]
	return exists && health
}

// isAvailable checks if a provider client is available
func (r *AIRouter) isAvailable(provider AIProvider) bool {
	_, exists := r.clients[provider]
	return exists
}

// monitorHealth continuously monitors provider health
func (r *AIRouter) monitorHealth() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	// Initial health check
	r.performHealthChecks()

	for range ticker.C {
		r.performHealthChecks()
	}
}

// performHealthChecks checks health of all providers
func (r *AIRouter) performHealthChecks() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for provider, client := range r.clients {
		go func(p AIProvider, c AIClient) {
			healthy := true

			if err := c.Health(ctx); err != nil {
				log.Printf("Health check failed for provider %s: %v", p, err)
				healthy = false
			}

			r.mu.Lock()
			r.healthCheck[p] = healthy
			r.mu.Unlock()
		}(provider, client)
	}
}

// GetProviderUsage returns usage statistics for all providers
func (r *AIRouter) GetProviderUsage() map[AIProvider]*ProviderUsage {
	usage := make(map[AIProvider]*ProviderUsage)

	for provider, client := range r.clients {
		usage[provider] = client.GetUsage()
	}

	return usage
}

// GetTotalUsage returns aggregated usage statistics
func (r *AIRouter) GetTotalUsage() *TotalUsage {
	totalUsage := &TotalUsage{
		Providers: make(map[AIProvider]*ProviderUsage),
	}

	for provider, client := range r.clients {
		usage := client.GetUsage()
		totalUsage.Providers[provider] = usage
		totalUsage.TotalRequests += usage.RequestCount
		totalUsage.TotalTokens += usage.TotalTokens
		totalUsage.TotalCost += usage.TotalCost
		totalUsage.TotalErrors += usage.ErrorCount
	}

	if totalUsage.TotalRequests > 0 {
		// Calculate average latency across all providers
		totalLatency := 0.0
		for _, usage := range totalUsage.Providers {
			totalLatency += usage.AvgLatency * float64(usage.RequestCount)
		}
		totalUsage.AvgLatency = totalLatency / float64(totalUsage.TotalRequests)
	}

	return totalUsage
}

// TotalUsage represents aggregated usage across all providers
type TotalUsage struct {
	Providers     map[AIProvider]*ProviderUsage `json:"providers"`
	TotalRequests int64                         `json:"total_requests"`
	TotalTokens   int64                         `json:"total_tokens"`
	TotalCost     float64                       `json:"total_cost"`
	TotalErrors   int64                         `json:"total_errors"`
	AvgLatency    float64                       `json:"avg_latency"`
}

// UpdateConfig updates the router configuration
func (r *AIRouter) UpdateConfig(config *RouterConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.config = config

	// Update rate limiters
	for provider, limit := range config.RateLimits {
		if limiter, exists := r.rateLimits[provider]; exists {
			limiter.mu.Lock()
			limiter.maxTokens = limit
			if limiter.tokens > limit {
				limiter.tokens = limit
			}
			limiter.mu.Unlock()
		} else {
			r.rateLimits[provider] = &rateLimiter{
				tokens:     limit,
				maxTokens:  limit,
				lastRefill: time.Now(),
			}
		}
	}
}

// GetHealthStatus returns current health status of all providers
func (r *AIRouter) GetHealthStatus() map[AIProvider]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := make(map[AIProvider]bool)
	for provider := range r.clients {
		status[provider] = r.healthCheck[provider]
	}

	return status
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}