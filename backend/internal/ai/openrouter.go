package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	openRouterChatURL   = openRouterBaseURL + "/chat/completions"
	openRouterModelsURL = openRouterBaseURL + "/models"

	// AutoModelSentinel is the model value that triggers auto-selection.
	AutoModelSentinel = "auto"
)

// OpenRouterClient implements AIClient for OpenRouter.
// It routes to any of the 300+ models OpenRouter exposes via OpenAI-compatible API.
// Claude/Anthropic models are explicitly blocked — those use the direct Anthropic client.
type OpenRouterClient struct {
	apiKey     string
	httpClient *http.Client
	selector   *AutoSelector
	usage      *ProviderUsage
	usageMu    sync.RWMutex
}

// openRouterRequest mirrors the OpenAI chat completions request shape.
type openRouterRequest struct {
	Model       string              `json:"model"`
	Messages    []openRouterMessage `json:"messages"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float32             `json:"temperature,omitempty"`
	Stream      bool                `json:"stream"`
}

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    interface{} `json:"code"`
	} `json:"error,omitempty"`
}

// NewOpenRouterClient creates an OpenRouter client.
// ollamaFallback is the Ollama cloud client used as dispatcher fallback; may be nil.
func NewOpenRouterClient(apiKey string, ollamaFallback *OllamaClient) *OpenRouterClient {
	apiKey = normalizeAPIKey(apiKey)
	return &OpenRouterClient{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Minute},
		selector:   NewAutoSelector(apiKey, ollamaFallback),
		usage: &ProviderUsage{
			Provider: ProviderOpenRouter,
			LastUsed: time.Now(),
		},
	}
}

// Generate sends a request to OpenRouter. When req.Model is "auto" or empty,
// the AutoSelector picks the best available model. Claude/Anthropic models are
// always rejected and will return an error — use the Claude client instead.
func (c *OpenRouterClient) Generate(ctx context.Context, req *AIRequest) (*AIResponse, error) {
	start := time.Now()

	model := strings.TrimSpace(req.Model)
	if model == "" || strings.EqualFold(model, AutoModelSentinel) {
		freeOnly := isFreeOnlyRequest(req)
		model = c.selector.Select(ctx, req, freeOnly)
	}

	// Hard guard: never route to Anthropic via OpenRouter
	if isAnthropicModel(model) {
		return nil, fmt.Errorf("openrouter: model %q is an Anthropic model; use the Claude client instead", model)
	}

	messages := buildOpenRouterMessages(req)
	payload := openRouterRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("openrouter: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", openRouterChatURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openrouter: create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", "https://apex-build.dev")
	httpReq.Header.Set("X-Title", "Apex Build")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openrouter: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("openrouter: read response: %w", err)
	}

	var orResp openRouterResponse
	if err := json.Unmarshal(respBody, &orResp); err != nil {
		return nil, fmt.Errorf("openrouter: parse response: %w", err)
	}

	if orResp.Error != nil && orResp.Error.Message != "" {
		errMsg := orResp.Error.Message
		// Classify for the router's health tracking
		if strings.Contains(strings.ToLower(errMsg), "invalid") && strings.Contains(strings.ToLower(errMsg), "key") {
			return nil, fmt.Errorf("unauthorized: %s", redactSecrets(errMsg, ""))
		}
		if strings.Contains(strings.ToLower(errMsg), "quota") || strings.Contains(strings.ToLower(errMsg), "credits") {
			return nil, fmt.Errorf("quota_exceeded: %s", redactSecrets(errMsg, ""))
		}
		return nil, fmt.Errorf("service_error: openrouter: %s", redactSecrets(errMsg, ""))
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, fmt.Errorf("unauthorized: openrouter HTTP %d", resp.StatusCode)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("service_error: openrouter HTTP %d: %s", resp.StatusCode, redactSecrets(string(respBody), ""))
	}

	if len(orResp.Choices) == 0 {
		return nil, fmt.Errorf("openrouter: empty response for model %s", model)
	}

	content := orResp.Choices[0].Message.Content
	duration := time.Since(start)

	// Calculate cost from catalog if available
	cost := estimateOpenRouterCost(model, orResp.Usage.PromptTokens, orResp.Usage.CompletionTokens)

	c.updateUsage(orResp.Usage.PromptTokens, orResp.Usage.CompletionTokens, cost)

	log.Printf("[openrouter] model=%s prompt=%d completion=%d cost=$%.6f duration=%v",
		model, orResp.Usage.PromptTokens, orResp.Usage.CompletionTokens, cost, duration)

	// Use the model actually returned by OpenRouter (may differ from requested if they alias it)
	actualModel := orResp.Model
	if actualModel == "" {
		actualModel = model
	}

	return &AIResponse{
		ID:       req.ID,
		Provider: ProviderOpenRouter,
		Content:  content,
		Usage: &Usage{
			PromptTokens:     orResp.Usage.PromptTokens,
			CompletionTokens: orResp.Usage.CompletionTokens,
			TotalTokens:      orResp.Usage.TotalTokens,
			Cost:             cost,
		},
		Metadata: map[string]interface{}{
			"model":           actualModel,
			"requested_model": model,
			"provider":        "openrouter",
		},
		Duration:  duration,
		CreatedAt: time.Now(),
	}, nil
}

// GetCapabilities returns all capabilities — OpenRouter models can handle any task.
func (c *OpenRouterClient) GetCapabilities() []AICapability {
	return []AICapability{
		CapabilityCodeGeneration,
		CapabilityNaturalLanguageToCode,
		CapabilityCodeReview,
		CapabilityCodeCompletion,
		CapabilityDebugging,
		CapabilityExplanation,
		CapabilityRefactoring,
		CapabilityTesting,
		CapabilityDocumentation,
		CapabilityArchitecture,
	}
}

// GetProvider returns the provider identifier.
func (c *OpenRouterClient) GetProvider() AIProvider { return ProviderOpenRouter }

// Health checks OpenRouter availability by hitting the models endpoint.
func (c *OpenRouterClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", openRouterModelsURL+"?limit=1", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("HTTP-Referer", "https://apex-build.dev")
	req.Header.Set("X-Title", "Apex Build")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("service_error: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	switch resp.StatusCode {
	case 200:
		return nil
	case 401, 403:
		return fmt.Errorf("unauthorized: openrouter health check returned %d", resp.StatusCode)
	default:
		return fmt.Errorf("service_error: openrouter health check returned %d", resp.StatusCode)
	}
}

// GetUsage returns usage statistics.
func (c *OpenRouterClient) GetUsage() *ProviderUsage {
	c.usageMu.RLock()
	defer c.usageMu.RUnlock()
	copied := *c.usage
	return &copied
}

// FetchLiveModels fetches the full model list from OpenRouter's API.
// Returns enriched models merged with catalog quality scores.
func (c *OpenRouterClient) FetchLiveModels(ctx context.Context) ([]LiveModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", openRouterModelsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("HTTP-Referer", "https://apex-build.dev")
	req.Header.Set("X-Title", "Apex Build")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openrouter: fetch models: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("openrouter: read models: %w", err)
	}

	var result struct {
		Data []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			ContextLength int    `json:"context_length"`
			Pricing       struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
			Architecture struct {
				Modality string `json:"modality"`
			} `json:"architecture"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("openrouter: parse models: %w", err)
	}

	catalog := CatalogByID()
	var models []LiveModelInfo
	for _, m := range result.Data {
		// Exclude Anthropic models from the list surfaced to frontend routing
		if isAnthropicModel(m.ID) {
			continue
		}
		inPer1M := parsePrice(m.Pricing.Prompt) * 1_000_000
		outPer1M := parsePrice(m.Pricing.Completion) * 1_000_000
		isFree := inPer1M == 0 && outPer1M == 0

		info := LiveModelInfo{
			ID:            m.ID,
			Name:          m.Name,
			ContextWindow: m.ContextLength,
			InputPer1M:    inPer1M,
			OutputPer1M:   outPer1M,
			IsFree:        isFree,
			Multimodal:    strings.Contains(m.Architecture.Modality, "image"),
		}
		// Merge catalog quality scores if available
		if cat, ok := catalog[m.ID]; ok {
			info.QualityCode = cat.QualityCode
			info.QualityReason = cat.QualityReason
			info.SpeedRating = cat.SpeedRating
			info.Tier = cat.Tier
			info.Tags = cat.Tags
		}
		models = append(models, info)
	}
	return models, nil
}

// LiveModelInfo is the API response shape for the frontend model picker.
type LiveModelInfo struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	ContextWindow int      `json:"context_window"`
	InputPer1M    float64  `json:"input_per_1m"`
	OutputPer1M   float64  `json:"output_per_1m"`
	IsFree        bool     `json:"is_free"`
	Multimodal    bool     `json:"multimodal"`
	QualityCode   float64  `json:"quality_code,omitempty"`
	QualityReason float64  `json:"quality_reasoning,omitempty"`
	SpeedRating   float64  `json:"speed_rating,omitempty"`
	Tier          string   `json:"tier,omitempty"`
	Tags          []string `json:"tags,omitempty"`
}

func (c *OpenRouterClient) updateUsage(promptTokens, completionTokens int, cost float64) {
	c.usageMu.Lock()
	defer c.usageMu.Unlock()
	c.usage.RequestCount++
	c.usage.TotalTokens += int64(promptTokens + completionTokens)
	c.usage.TotalCost += cost
	c.usage.LastUsed = time.Now()
}

// isFreeOnlyRequest returns true when the request must be served by a free model.
// Extend this to check subscription tier from req.Context when billing is wired.
func isFreeOnlyRequest(req *AIRequest) bool {
	if req == nil {
		return false
	}
	if req.Context != nil {
		if tier, ok := req.Context["subscription_tier"].(string); ok {
			return strings.EqualFold(tier, "free")
		}
	}
	return false
}

// buildOpenRouterMessages converts an AIRequest into the OpenRouter messages array.
func buildOpenRouterMessages(req *AIRequest) []openRouterMessage {
	system := buildSystemPrompt(req)
	user := req.Prompt
	if req.Code != "" {
		user = fmt.Sprintf("%s\n\n```%s\n%s\n```", user, req.Language, req.Code)
	}
	msgs := []openRouterMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}
	return msgs
}

// buildSystemPrompt builds the system prompt for OpenRouter requests.
// Mirrors the pattern used by the existing OpenAI/Gemini clients.
func buildSystemPrompt(req *AIRequest) string {
	base := "You are an expert software engineer and AI coding assistant for Apex Build, a full-stack app builder. Produce production-quality, complete, working code. Never truncate output."
	switch req.Capability {
	case CapabilityCodeGeneration, CapabilityNaturalLanguageToCode:
		return base + " Generate complete, working code files. Include all imports and dependencies."
	case CapabilityDebugging:
		return base + " Identify and fix bugs. Explain the root cause briefly."
	case CapabilityArchitecture:
		return base + " Design robust, scalable architecture. Provide clear structure and rationale."
	case CapabilityTesting:
		return base + " Write comprehensive tests with edge cases. Use the project's existing test framework."
	case CapabilityCodeReview:
		return base + " Review code for correctness, security, performance, and maintainability."
	case CapabilityRefactoring:
		return base + " Refactor for clarity and maintainability without changing behavior."
	default:
		return base
	}
}

// estimateOpenRouterCost calculates actual cost from catalog pricing + token counts.
func estimateOpenRouterCost(modelID string, promptTokens, completionTokens int) float64 {
	catalog := CatalogByID()
	m, ok := catalog[modelID]
	if !ok || m.IsFree {
		return 0
	}
	return (m.InputPer1M * float64(promptTokens) / 1_000_000) +
		(m.OutputPer1M * float64(completionTokens) / 1_000_000)
}

// parsePrice converts an OpenRouter pricing string (per token) to float64.
func parsePrice(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0
	}
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}
