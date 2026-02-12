package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// GrokClient implements the xAI Grok API client
// API docs: https://docs.x.ai/docs/api-reference
// Compatible with OpenAI chat completions format
type GrokClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	usage      *ProviderUsage
	usageMu    sync.RWMutex
}

// Grok API uses OpenAI-compatible request/response format
type grokRequest struct {
	Model       string        `json:"model"`
	Messages    []grokMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float32       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream"`
}

type grokMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type grokResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
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
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// NewGrokClient creates a new xAI Grok API client
func NewGrokClient(apiKey string) *GrokClient {
	return &GrokClient{
		apiKey:  apiKey,
		baseURL: "https://api.x.ai/v1/chat/completions",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		usage: &ProviderUsage{
			Provider: ProviderGrok,
			LastUsed: time.Now(),
		},
	}
}

// Generate implements the AIClient interface for Grok
func (g *GrokClient) Generate(ctx context.Context, req *AIRequest) (*AIResponse, error) {
	startTime := time.Now()

	messages := g.buildMessages(req)

	model := g.getModel(req)

	grokReq := &grokRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   g.getMaxTokens(req),
		Temperature: req.Temperature,
		Stream:      false,
	}

	resp, err := g.makeRequest(ctx, grokReq)
	if err != nil {
		g.incrementErrorCount()
		return &AIResponse{
			ID:        req.ID,
			Provider:  ProviderGrok,
			Error:     err.Error(),
			Duration:  time.Since(startTime),
			CreatedAt: time.Now(),
		}, err
	}

	cost := g.calculateCost(resp.Usage.PromptTokens, resp.Usage.CompletionTokens, model)
	g.updateUsage(resp.Usage.TotalTokens, cost, time.Since(startTime))

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	return &AIResponse{
		ID:       req.ID,
		Provider: ProviderGrok,
		Content:  content,
		Metadata: map[string]interface{}{
			"model": resp.Model,
		},
		Usage: &Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
			Cost:             cost,
		},
		Duration:  time.Since(startTime),
		CreatedAt: time.Now(),
	}, nil
}

// buildMessages creates the message array for Grok API
func (g *GrokClient) buildMessages(req *AIRequest) []grokMessage {
	messages := []grokMessage{}

	systemPrompt := g.buildSystemPrompt(req.Capability, req.Language)
	messages = append(messages, grokMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	userPrompt := g.buildUserPrompt(req)
	messages = append(messages, grokMessage{
		Role:    "user",
		Content: userPrompt,
	})

	return messages
}

// buildSystemPrompt creates capability-specific system prompts
func (g *GrokClient) buildSystemPrompt(capability AICapability, language string) string {
	basePrompt := `You are an expert software developer for APEX.BUILD, a professional cloud development platform.

CRITICAL REQUIREMENTS - ALWAYS FOLLOW:
1. NEVER output demo code, mock data, placeholder content, or TODO comments
2. ALWAYS produce complete, production-ready, fully functional code
3. If external resources are needed (API keys, database credentials, third-party services), either:
   a) Ask the user to provide them before proceeding, OR
   b) Build everything possible and clearly mark where the user must add their credentials
4. Include all necessary imports, error handling, and edge cases
5. Follow industry best practices and security standards
6. Write real implementations, not stubs or examples`

	switch capability {
	case CapabilityCodeGeneration:
		return fmt.Sprintf("%s\n\nGenerate production-ready %s code. Every function must be complete and working.", basePrompt, language)
	case CapabilityCodeReview:
		return fmt.Sprintf("%s\n\nPerform thorough code review. Identify bugs, security issues, and performance problems with concrete fixes.", basePrompt)
	case CapabilityDebugging:
		return fmt.Sprintf("%s\n\nDebug the code. Identify root causes and provide complete, working fixes.", basePrompt)
	case CapabilityTesting:
		return fmt.Sprintf("%s\n\nGenerate comprehensive, executable test suites for %s.", basePrompt, language)
	case CapabilityRefactoring:
		return fmt.Sprintf("%s\n\nRefactor the code following modern %s best practices. Output complete refactored code.", basePrompt, language)
	default:
		return fmt.Sprintf("%s\n\nAssist with %s development. Every output must be production-ready.", basePrompt, language)
	}
}

// buildUserPrompt constructs the user prompt
func (g *GrokClient) buildUserPrompt(req *AIRequest) string {
	prompt := req.Prompt

	if req.Code != "" {
		prompt += fmt.Sprintf("\n\nCode:\n```%s\n%s\n```", req.Language, req.Code)
	}

	if req.Context != nil {
		if fileContext, ok := req.Context["file_context"].(string); ok {
			prompt += fmt.Sprintf("\n\nFile context: %s", fileContext)
		}
		if projectStructure, ok := req.Context["project_structure"].(string); ok {
			prompt += fmt.Sprintf("\n\nProject structure: %s", projectStructure)
		}
	}

	return prompt
}

// getModel selects the appropriate Grok model
func (g *GrokClient) getModel(req *AIRequest) string {
	// Respect explicit model override
	if req.Model != "" {
		return req.Model
	}

	switch req.Capability {
	case CapabilityCodeGeneration, CapabilityRefactoring, CapabilityArchitecture:
		return "grok-4" // Flagship for complex tasks
	case CapabilityCodeCompletion:
		return "grok-4-fast" // Fast for completions
	case CapabilityCodeReview, CapabilityDebugging:
		return "grok-4" // Flagship for analysis
	default:
		return "grok-4-fast" // Default to fast model
	}
}

// makeRequest sends HTTP request to xAI API
func (g *GrokClient) makeRequest(ctx context.Context, req *grokRequest) (*grokResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", g.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.apiKey))

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case 429:
			return nil, fmt.Errorf("RATE_LIMIT: Grok API rate limit exceeded")
		case 403:
			return nil, fmt.Errorf("FORBIDDEN: Grok API access denied - check API key permissions")
		case 401:
			return nil, fmt.Errorf("UNAUTHORIZED: Invalid Grok API key")
		case 402:
			return nil, fmt.Errorf("QUOTA_EXCEEDED: Grok API quota exhausted - add credits at console.x.ai")
		case 500, 502, 503, 504:
			return nil, fmt.Errorf("SERVICE_ERROR: Grok service temporarily unavailable (status %d)", resp.StatusCode)
		default:
			return nil, fmt.Errorf("API_ERROR: Grok request failed with status %d: %s", resp.StatusCode, string(body))
		}
	}

	var grokResp grokResponse
	if err := json.Unmarshal(body, &grokResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if grokResp.Error != nil {
		return nil, fmt.Errorf("Grok API error: %s", grokResp.Error.Message)
	}

	return &grokResp, nil
}

// GetCapabilities returns capabilities Grok excels at
func (g *GrokClient) GetCapabilities() []AICapability {
	return []AICapability{
		CapabilityCodeGeneration,
		CapabilityCodeReview,
		CapabilityDebugging,
		CapabilityRefactoring,
		CapabilityTesting,
		CapabilityDocumentation,
		CapabilityCodeCompletion,
	}
}

// GetProvider returns the provider identifier
func (g *GrokClient) GetProvider() AIProvider {
	return ProviderGrok
}

// Health checks if Grok API is accessible
func (g *GrokClient) Health(ctx context.Context) error {
	testReq := &grokRequest{
		Model: "grok-4-fast",
		Messages: []grokMessage{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens: 5,
	}

	_, err := g.makeRequest(ctx, testReq)
	return err
}

// GetUsage returns current usage statistics
func (g *GrokClient) GetUsage() *ProviderUsage {
	g.usageMu.RLock()
	defer g.usageMu.RUnlock()

	return &ProviderUsage{
		Provider:     g.usage.Provider,
		RequestCount: g.usage.RequestCount,
		TotalTokens:  g.usage.TotalTokens,
		TotalCost:    g.usage.TotalCost,
		AvgLatency:   g.usage.AvgLatency,
		ErrorCount:   g.usage.ErrorCount,
		LastUsed:     g.usage.LastUsed,
	}
}

// getMaxTokens determines appropriate max tokens
func (g *GrokClient) getMaxTokens(req *AIRequest) int {
	if req.MaxTokens > 0 {
		return req.MaxTokens
	}

	switch req.Capability {
	case CapabilityCodeCompletion:
		return 500
	case CapabilityCodeGeneration:
		return 4000
	case CapabilityTesting:
		return 3000
	case CapabilityRefactoring:
		return 3000
	case CapabilityCodeReview:
		return 2000
	default:
		return 2000
	}
}

// calculateCost estimates cost based on xAI Grok pricing (Jan 2026)
func (g *GrokClient) calculateCost(inputTokens, outputTokens int, model string) float64 {
	var inputCostPer1M, outputCostPer1M float64

	switch model {
	case "grok-4":
		inputCostPer1M = 2.00   // $2.00 per 1M input tokens
		outputCostPer1M = 10.00 // $10.00 per 1M output tokens
	case "grok-4-fast":
		inputCostPer1M = 0.20  // $0.20 per 1M input tokens
		outputCostPer1M = 0.50 // $0.50 per 1M output tokens
	case "grok-3-mini":
		inputCostPer1M = 0.30  // $0.30 per 1M input tokens
		outputCostPer1M = 0.50 // $0.50 per 1M output tokens
	default:
		inputCostPer1M = 0.20 // Default to grok-4-fast pricing
		outputCostPer1M = 0.50
	}

	inputCost := float64(inputTokens) / 1_000_000.0 * inputCostPer1M
	outputCost := float64(outputTokens) / 1_000_000.0 * outputCostPer1M

	return inputCost + outputCost
}

// updateUsage updates internal usage statistics
func (g *GrokClient) updateUsage(totalTokens int, cost float64, duration time.Duration) {
	g.usageMu.Lock()
	defer g.usageMu.Unlock()

	g.usage.RequestCount++
	g.usage.TotalTokens += int64(totalTokens)
	g.usage.TotalCost += cost
	g.usage.AvgLatency = (g.usage.AvgLatency*float64(g.usage.RequestCount-1) + duration.Seconds()) / float64(g.usage.RequestCount)
	g.usage.LastUsed = time.Now()
}

// incrementErrorCount safely increments the error count
func (g *GrokClient) incrementErrorCount() {
	g.usageMu.Lock()
	defer g.usageMu.Unlock()
	g.usage.ErrorCount++
}
