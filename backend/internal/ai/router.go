package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
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

// ProviderHealthDetail holds detailed health info for a single provider.
type ProviderHealthDetail struct {
	// Status is one of: "ok", "no_credits", "auth_error", "timeout", "error", "unknown"
	Status string `json:"status"`
	// Balance is nil when unknown, 0 when known-depleted, positive when known.
	// Populated only when we can infer it (e.g. a depleted-credit error implies 0).
	Balance *float64 `json:"balance"`
}

// AIRouter intelligently routes AI requests to the optimal provider
type AIRouter struct {
	clients      map[AIProvider]AIClient
	config       *RouterConfig
	rateLimits   map[AIProvider]*rateLimiter
	mu           sync.RWMutex
	healthCheck  map[AIProvider]bool
	healthStatus map[AIProvider]string // "ok", "no_credits", "auth_error", "timeout", "error", "unknown"
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

	for provider, emulation := range configuredOllamaEmulations() {
		if _, exists := clients[provider]; exists {
			continue
		}
		clients[provider] = newAliasedOllamaProviderClient(provider, emulation.URL, emulation.Model)
		log.Printf("Local provider emulation enabled: slot=%s -> ollama(%s, model=%s)", provider, emulation.URL, emulation.Model)
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
		clients:      clients,
		config:       config,
		rateLimits:   rateLimits,
		healthCheck:  make(map[AIProvider]bool),
		healthStatus: make(map[AIProvider]string),
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

	// Set reasonable max tokens default.
	// Code-generation tasks can request up to 32k tokens; keep the hard ceiling
	// generous enough that large file outputs are never silently truncated.
	if req.MaxTokens <= 0 {
		req.MaxTokens = 4000
	} else if req.MaxTokens > 32000 {
		req.MaxTokens = 32000
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
					return r.generateWithAttemptBudget(ctx, fallbackProvider, fallbackClient, fallbackReq, len(fallbacks))
				}
			}
		}
		return nil, fmt.Errorf("rate limit exceeded for all available providers")
	}

	// Attempt to generate with primary provider.
	// Retry up to 2 extra times on transient network errors (important for
	// Ollama-only mode where there is no fallback provider).
	const maxRetries = 2
	var response *AIResponse
	var genErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 3 * time.Second
			log.Printf("Retrying provider %s after transient error (attempt %d/%d, backoff %v): %v",
				provider, attempt, maxRetries, backoff, genErr)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
		response, genErr = r.generateWithAttemptBudget(ctx, provider, client, primaryReq, 1+len(r.config.FallbackOrder[provider]))
		if genErr == nil || !isTransientError(genErr) {
			break
		}
	}
	if genErr != nil {
		errStr := genErr.Error()
		log.Printf("Error from provider %s (after retries): %v", provider, genErr)

		// Mark provider unhealthy for any definitive billing/quota/auth failure
		// so subsequent selectProvider calls skip it immediately.
		if errClass := classifyProviderError(genErr); errClass == "no_credits" || errClass == "auth_error" {
			r.mu.Lock()
			r.healthCheck[provider] = false
			r.healthStatus[provider] = errClass
			r.mu.Unlock()
			log.Printf("Marked provider %s as unhealthy (%s) after generate error", provider, errClass)
		}

		// Collect all errors for better reporting
		failedProviders := []string{fmt.Sprintf("%s: %s", provider, errStr)}

		// Try fallback providers
		fallbacks := r.config.FallbackOrder[provider]
		for idx, fallbackProvider := range fallbacks {
			if fallbackClient, exists := r.clients[fallbackProvider]; exists {
				if r.checkRateLimit(fallbackProvider) && r.isHealthyOrUnknown(fallbackProvider) {
					log.Printf("Falling back to provider %s", fallbackProvider)
					fallbackReq := r.requestForProvider(primaryReq, fallbackProvider)
					fallbackResponse, fallbackErr := r.generateWithAttemptBudget(ctx, fallbackProvider, fallbackClient, fallbackReq, len(fallbacks)-idx)
					if fallbackErr == nil {
						return fallbackResponse, nil
					}
					fallbackErrStr := fallbackErr.Error()
					log.Printf("Fallback provider %s also failed: %v", fallbackProvider, fallbackErr)
					failedProviders = append(failedProviders, fmt.Sprintf("%s: %s", fallbackProvider, fallbackErrStr))

					// Mark fallback unhealthy for definitive billing/quota/auth failures
					if fbClass := classifyProviderError(fallbackErr); fbClass == "no_credits" || fbClass == "auth_error" {
						r.mu.Lock()
						r.healthCheck[fallbackProvider] = false
						r.healthStatus[fallbackProvider] = fbClass
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

// isTransientError returns true for errors that are safe to retry: network
// resets, EOF, connection refused, and context-deadline-exceeded (but NOT
// context-cancelled, which means the caller explicitly gave up).
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	// Never retry a caller-initiated cancellation.
	if errors.Is(err, context.Canceled) {
		return false
	}
	// Deadline exceeded on the parent context also shouldn't be retried.
	if errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "eof") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "i/o timeout")
}

func providerAttemptBaseTimeout(provider AIProvider) time.Duration {
	switch provider {
	case ProviderGemini:
		return 70 * time.Second
	case ProviderOllama:
		return 3 * time.Minute
	default:
		return 90 * time.Second
	}
}

func providerFallbackReserve(provider AIProvider) time.Duration {
	if provider == ProviderOllama {
		return 45 * time.Second
	}
	return 20 * time.Second
}

func minDuration(a, b time.Duration) time.Duration {
	if a <= 0 {
		return b
	}
	if b <= 0 || a < b {
		return a
	}
	return b
}

func providerAttemptSafetyMargin(remaining time.Duration) time.Duration {
	if remaining <= 0 {
		return 0
	}
	margin := remaining / 10
	if margin <= 0 {
		return time.Millisecond
	}
	if margin > 250*time.Millisecond {
		return 250 * time.Millisecond
	}
	return margin
}

func (r *AIRouter) generateWithAttemptBudget(
	ctx context.Context,
	provider AIProvider,
	client AIClient,
	req *AIRequest,
	attemptsRemaining int,
) (*AIResponse, error) {
	if client == nil {
		return nil, fmt.Errorf("client not available for provider: %s", provider)
	}
	if attemptsRemaining < 1 {
		attemptsRemaining = 1
	}

	attemptCtx := ctx
	cancel := func() {}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		safetyMargin := providerAttemptSafetyMargin(remaining)
		if remaining <= safetyMargin {
			return nil, context.DeadlineExceeded
		}

		budget := remaining - safetyMargin
		if attemptsRemaining > 1 {
			reserve := time.Duration(attemptsRemaining-1) * providerFallbackReserve(provider)
			maxReserve := budget / 2
			if reserve > maxReserve {
				reserve = maxReserve
			}
			budget -= reserve
		}
		budget = minDuration(providerAttemptBaseTimeout(provider), budget)
		if budget <= 0 {
			return nil, context.DeadlineExceeded
		}
		attemptCtx, cancel = context.WithTimeout(ctx, budget)
		defer cancel()
	} else {
		attemptCtx, cancel = context.WithTimeout(ctx, providerAttemptBaseTimeout(provider))
		defer cancel()
	}

	return client.Generate(attemptCtx, req)
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
		if r.isAvailable(requested) {
			knownBad := false
			if status, ok := r.healthStatus[requested]; ok {
				knownBad = status == "no_credits" || status == "auth_error"
			}
			if !knownBad {
				return requested, nil
			}
		}
		if fallbacks, ok := r.config.FallbackOrder[requested]; ok {
			for _, provider := range fallbacks {
				if r.isAvailable(provider) && r.isHealthyOrUnknown(provider) {
					return provider, nil
				}
			}
		}
		// Requested provider is definitively unhealthy and no fallback worked —
		// let load balancing pick a healthy alternative rather than forcing a
		// provider that will immediately fail.
		return r.selectByLoadBalancing()
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

// classifyProviderError returns a short status string describing a health-check error.
func classifyProviderError(err error) string {
	if err == nil {
		return "ok"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "credit balance") ||
		strings.Contains(msg, "insufficient_funds") ||
		strings.Contains(msg, "billing") ||
		strings.Contains(msg, "quota_exceeded") ||
		strings.Contains(msg, "out of credits") ||
		strings.Contains(msg, "exceeded your") ||
		strings.Contains(msg, "you have exceeded") ||
		strings.Contains(msg, "rate_limit_exceeded") ||
		strings.Contains(msg, "resource_exhausted"):
		return "no_credits"
	case strings.Contains(msg, "invalid api key") ||
		strings.Contains(msg, "api key not found") ||
		strings.Contains(msg, "api_key") ||
		strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "authentication") ||
		strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, " 401") ||
		strings.Contains(msg, " 403"):
		return "auth_error"
	case strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "context deadline"):
		return "timeout"
	default:
		return "error"
	}
}

// performHealthChecks checks health of all providers
func (r *AIRouter) performHealthChecks() {
	if r == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel() // Ensure cancel is called when function returns

	var wg sync.WaitGroup

	for provider, client := range r.clients {
		wg.Add(1)
		go func(p AIProvider, c AIClient) {
			defer wg.Done()

			// Use a per-provider timeout to prevent one slow provider from blocking others
			providerCtx, providerCancel := context.WithTimeout(ctx, 10*time.Second)
			defer providerCancel()

			err := c.Health(providerCtx)
			status := classifyProviderError(err)
			healthy := (status == "ok")

			if err != nil {
				log.Printf("Health check failed for provider %s [%s]: %v", p, status, err)
			} else {
				log.Printf("Health check passed for provider %s", p)
			}

			r.mu.Lock()
			if r.healthCheck == nil {
				r.healthCheck = make(map[AIProvider]bool)
			}
			if r.healthStatus == nil {
				r.healthStatus = make(map[AIProvider]string)
			}
			r.healthCheck[p] = healthy
			r.healthStatus[p] = status
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

// GetHealthStatus returns current health status of all providers (bool map, kept for compat).
func (r *AIRouter) GetHealthStatus() map[AIProvider]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := make(map[AIProvider]bool)
	for provider := range r.clients {
		status[provider] = r.healthCheck[provider]
	}

	return status
}

// zeroBalance is a helper to return a pointer to 0.0.
func zeroBalance() *float64 { v := 0.0; return &v }

// GetDetailedHealthStatus returns rich status info for each provider.
// Status values: "ok", "no_credits", "auth_error", "timeout", "error", "unknown"
// Balance is nil when unknown, 0.0 when known-depleted.
// NOTE: Balance reflects only what can be inferred from health-check errors; it does
// not query the provider billing APIs in real time.
func (r *AIRouter) GetDetailedHealthStatus() map[string]*ProviderHealthDetail {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*ProviderHealthDetail, len(r.clients))
	for provider := range r.clients {
		status, ok := r.healthStatus[provider]
		if !ok || status == "" {
			status = "unknown"
		}
		var balance *float64
		if status == "no_credits" {
			balance = zeroBalance() // known to be depleted
		}
		result[string(provider)] = &ProviderHealthDetail{
			Status:  status,
			Balance: balance,
		}
	}
	return result
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
