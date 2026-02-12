package ai

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// MaxPromptLength prevents excessive API costs and OOM
	MaxPromptLength = 100000
	// MaxCodeLength limits code input size
	MaxCodeLength = 50000
)

// AIRouter intelligently routes AI requests to the optimal provider
type AIRouter struct {
	clients     map[AIProvider]AIClient
	config      *RouterConfig
	rateLimits  map[AIProvider]*rateLimiter
	mu          sync.RWMutex
	healthCheck map[AIProvider]bool
}

// GetDefaultProvider returns the configured default provider for a capability.
// Falls back to Claude if no explicit mapping exists.
func (r *AIRouter) GetDefaultProvider(capability AICapability) AIProvider {
	if r == nil || r.config == nil {
		return ProviderClaude
	}
	if r.config.DefaultProviders != nil {
		if provider, ok := r.config.DefaultProviders[capability]; ok {
			return provider
		}
	}
	return ProviderClaude
}

// rateLimiter tracks rate limiting for each provider
type rateLimiter struct {
	tokens     int
	maxTokens  int
	lastRefill time.Time
	mu         sync.Mutex
}

// NewAIRouter creates a new AI router with multiple providers
func NewAIRouter(claudeKey, openAIKey, geminiKey string, extraKeys ...string) *AIRouter {
	clients := make(map[AIProvider]AIClient)
	claudeKey = normalizeAPIKey(claudeKey)
	openAIKey = normalizeAPIKey(openAIKey)
	geminiKey = normalizeAPIKey(geminiKey)

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
	// Grok key is the first extra key if provided
	grokKey := ""
	if len(extraKeys) > 0 {
		grokKey = normalizeAPIKey(extraKeys[0])
	}
	if grokKey != "" {
		clients[ProviderGrok] = NewGrokClient(grokKey)
	}

	// Ollama URL is the second extra key if provided
	ollamaURL := ""
	if len(extraKeys) > 1 {
		ollamaURL = extraKeys[1]
	}
	if ollamaURL != "" {
		clients[ProviderOllama] = NewOllamaClient(ollamaURL)
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
	// Validate input lengths to prevent abuse and excessive costs
	if len(req.Prompt) > MaxPromptLength {
		return nil, fmt.Errorf("prompt exceeds maximum length of %d characters", MaxPromptLength)
	}
	if len(req.Code) > MaxCodeLength {
		return nil, fmt.Errorf("code exceeds maximum length of %d characters", MaxCodeLength)
	}

	// Set request ID if not provided
	if req.ID == "" {
		req.ID = uuid.New().String()
	}

	// Set creation time
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now()
	}

	// Set default temperature if not provided (with bounds)
	if req.Temperature == 0 {
		req.Temperature = 0.7
	} else if req.Temperature < 0 {
		req.Temperature = 0
	} else if req.Temperature > 2.0 {
		req.Temperature = 2.0
	}

	// Set reasonable max tokens default
	if req.MaxTokens <= 0 {
		req.MaxTokens = 2000
	} else if req.MaxTokens > 8000 {
		req.MaxTokens = 8000 // Cap to prevent excessive costs
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
	primaryReq := r.requestForProvider(req, provider)

	// Check rate limiting
	if !r.checkRateLimit(provider) {
		// Try fallback providers
		fallbacks := r.config.FallbackOrder[provider]
		for _, fallbackProvider := range fallbacks {
			if r.checkRateLimit(fallbackProvider) {
				if fallbackClient, exists := r.clients[fallbackProvider]; exists {
					log.Printf("Rate limited for %s, using fallback %s", provider, fallbackProvider)
					fallbackReq := r.requestForProvider(primaryReq, fallbackProvider)
					return fallbackClient.Generate(ctx, fallbackReq)
				}
			}
		}
		return nil, fmt.Errorf("rate limit exceeded for all available providers")
	}

	// Attempt to generate with primary provider
	response, err := client.Generate(ctx, primaryReq)
	if err != nil {
		errStr := err.Error()
		log.Printf("Error from provider %s: %v", provider, err)

		// Mark provider as temporarily unhealthy for quota/rate limit errors
		if strings.Contains(errStr, "RATE_LIMIT") || strings.Contains(errStr, "QUOTA_EXCEEDED") {
			r.mu.Lock()
			r.healthCheck[provider] = false
			r.mu.Unlock()
			log.Printf("Marked provider %s as temporarily unhealthy due to quota/rate limit", provider)
		}

		// Collect all errors for better reporting
		failedProviders := []string{fmt.Sprintf("%s: %s", provider, errStr)}

		// Try fallback providers
		fallbacks := r.config.FallbackOrder[provider]
		for _, fallbackProvider := range fallbacks {
			if fallbackClient, exists := r.clients[fallbackProvider]; exists {
				if r.checkRateLimit(fallbackProvider) && r.isHealthyOrUnknown(fallbackProvider) {
					log.Printf("Falling back to provider %s", fallbackProvider)
					fallbackReq := r.requestForProvider(primaryReq, fallbackProvider)
					fallbackResponse, fallbackErr := fallbackClient.Generate(ctx, fallbackReq)
					if fallbackErr == nil {
						return fallbackResponse, nil
					}
					fallbackErrStr := fallbackErr.Error()
					log.Printf("Fallback provider %s also failed: %v", fallbackProvider, fallbackErr)
					failedProviders = append(failedProviders, fmt.Sprintf("%s: %s", fallbackProvider, fallbackErrStr))

					// Mark fallback as unhealthy too if quota/rate limited
					if strings.Contains(fallbackErrStr, "RATE_LIMIT") || strings.Contains(fallbackErrStr, "QUOTA_EXCEEDED") {
						r.mu.Lock()
						r.healthCheck[fallbackProvider] = false
						r.mu.Unlock()
					}
				} else {
					failedProviders = append(failedProviders, fmt.Sprintf("%s: unavailable or rate-limited", fallbackProvider))
				}
			}
		}

		return nil, fmt.Errorf("ALL_PROVIDERS_FAILED: %s", strings.Join(failedProviders, "; "))
	}

	return response, nil
}

// requestForProvider prepares a provider-specific request copy.
// If provider changes, model override is cleared to avoid cross-provider model mismatches.
func (r *AIRouter) requestForProvider(req *AIRequest, provider AIProvider) *AIRequest {
	if req == nil {
		return nil
	}

	reqCopy := *req
	if req.Provider != "" && req.Provider != provider {
		reqCopy.Model = ""
	}
	reqCopy.Provider = provider
	return &reqCopy
}

// selectProvider chooses the optimal provider for a request
func (r *AIRouter) selectProvider(req *AIRequest) (AIProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Respect explicit provider requests when possible (with fallbacks)
	if req.Provider != "" {
		requested := req.Provider
		if r.isAvailable(requested) && r.isHealthyOrUnknown(requested) {
			return requested, nil
		}
		if fallbacks, ok := r.config.FallbackOrder[requested]; ok {
			for _, provider := range fallbacks {
				if r.isAvailable(provider) && r.isHealthyOrUnknown(provider) {
					return provider, nil
				}
			}
		}
		// Last resort: try requested provider if it's configured at all
		if r.isAvailable(requested) {
			return requested, nil
		}
	}

	// Start with the default provider for this capability
	defaultProvider, exists := r.config.DefaultProviders[req.Capability]
	if !exists {
		// If no default, use load balancing
		return r.selectByLoadBalancing()
	}

	// Check if default provider is healthy and available
	if r.isHealthyOrUnknown(defaultProvider) && r.isAvailable(defaultProvider) {
		return defaultProvider, nil
	}

	// If default provider is not available, try fallbacks
	fallbacks := r.config.FallbackOrder[defaultProvider]
	for _, provider := range fallbacks {
		if r.isHealthyOrUnknown(provider) && r.isAvailable(provider) {
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
		if r.isHealthyOrUnknown(provider) && r.isAvailable(provider) {
			healthyProviders = append(healthyProviders, provider)
			totalWeight += weight
		}
	}

	if len(healthyProviders) == 0 {
		return "", fmt.Errorf("no healthy providers available")
	}

	// Select provider based on weighted random selection using crypto/rand
	randomValue := cryptoRandFloat64() * totalWeight
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

// cryptoRandFloat64 generates a cryptographically secure random float64 between 0 and 1
func cryptoRandFloat64() float64 {
	// Generate a random 64-bit integer
	max := big.NewInt(1 << 53) // Use 53 bits for float64 precision
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		// Fallback to 0.5 on error (extremely unlikely)
		return 0.5
	}
	return float64(n.Int64()) / float64(1<<53)
}

// checkRateLimit checks if a provider is within rate limits
func (r *AIRouter) checkRateLimit(provider AIProvider) bool {
	limiter, exists := r.rateLimits[provider]
	if !exists {
		return true
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	// Refill tokens based on time passed (per-second granularity)
	now := time.Now()
	timePassed := now.Sub(limiter.lastRefill)

	// Calculate tokens to add based on time passed (tokens per second = maxTokens / 60)
	tokensPerSecond := float64(limiter.maxTokens) / 60.0
	tokensToAdd := int(timePassed.Seconds() * tokensPerSecond)

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

// isHealthyOrUnknown treats missing health as healthy to avoid blocking requests during startup.
func (r *AIRouter) isHealthyOrUnknown(provider AIProvider) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	health, exists := r.healthCheck[provider]
	if !exists {
		return true
	}
	return health
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel() // Ensure cancel is called when function returns

	var wg sync.WaitGroup

	for provider, client := range r.clients {
		wg.Add(1)
		go func(p AIProvider, c AIClient) {
			defer wg.Done()
			healthy := true

			// Use a per-provider timeout to prevent one slow provider from blocking others
			providerCtx, providerCancel := context.WithTimeout(ctx, 10*time.Second)
			defer providerCancel()

			if err := c.Health(providerCtx); err != nil {
				log.Printf("Health check failed for provider %s: %v", p, err)
				healthy = false
			} else {
				log.Printf("Health check passed for provider %s", p)
			}

			r.mu.Lock()
			r.healthCheck[p] = healthy
			r.mu.Unlock()
		}(provider, client)
	}

	wg.Wait()
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
