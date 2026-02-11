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

// ClaudeClient implements the Claude/Anthropic API client
type ClaudeClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	usage      *ProviderUsage
	usageMu    sync.RWMutex // Protects usage statistics
}

// Claude API request/response structures
type claudeRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int            `json:"max_tokens"`
	Messages    []claudeMessage `json:"messages"`
	Temperature float32        `json:"temperature,omitempty"`
	System      string         `json:"system,omitempty"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	ID      string `json:"id"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// NewClaudeClient creates a new Claude API client
func NewClaudeClient(apiKey string) *ClaudeClient {
	return &ClaudeClient{
		apiKey:  apiKey,
		baseURL: "https://api.anthropic.com/v1/messages",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		usage: &ProviderUsage{
			Provider:  ProviderClaude,
			LastUsed:  time.Now(),
		},
	}
}

// Generate implements the AIClient interface for Claude
func (c *ClaudeClient) Generate(ctx context.Context, req *AIRequest) (*AIResponse, error) {
	startTime := time.Now()

	// Build system prompt based on capability
	systemPrompt := c.buildSystemPrompt(req.Capability, req.Language)

	// Build user prompt
	userPrompt := c.buildUserPrompt(req)

	// Select model - respect explicit override or use flagship
	model := "claude-opus-4-5-20251101"
	if req.Model != "" {
		model = req.Model
	}

	// Create Claude API request
	claudeReq := &claudeRequest{
		Model:     model,
		MaxTokens: c.getMaxTokens(req),
		Messages: []claudeMessage{
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Temperature: req.Temperature,
		System:      systemPrompt,
	}

	// Make API request
	resp, err := c.makeRequest(ctx, claudeReq)
	if err != nil {
		c.incrementErrorCount()
		return &AIResponse{
			ID:        req.ID,
			Provider:  ProviderClaude,
			Error:     err.Error(),
			Duration:  time.Since(startTime),
			CreatedAt: time.Now(),
		}, err
	}

	// Calculate cost (approximate based on Claude pricing)
	cost := c.calculateCost(resp.Usage.InputTokens, resp.Usage.OutputTokens)

	// Update usage statistics
	c.updateUsage(resp.Usage.InputTokens+resp.Usage.OutputTokens, cost, time.Since(startTime))

	// Extract response content
	content := ""
	if len(resp.Content) > 0 && resp.Content[0].Type == "text" {
		content = resp.Content[0].Text
	}

	return &AIResponse{
		ID:       req.ID,
		Provider: ProviderClaude,
		Content:  content,
		Usage: &Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
			Cost:             cost,
		},
		Duration:  time.Since(startTime),
		CreatedAt: time.Now(),
	}, nil
}

// buildSystemPrompt creates capability-specific system prompts
func (c *ClaudeClient) buildSystemPrompt(capability AICapability, language string) string {
	basePrompt := `You are an expert software developer for APEX.BUILD, a professional cloud development platform.

CRITICAL REQUIREMENTS - ALWAYS FOLLOW:
1. NEVER output demo code, mock data, placeholder content, or TODO comments
2. ALWAYS produce complete, production-ready, fully functional code
3. If external resources are needed (API keys, database credentials, third-party services), either:
   a) Ask the user to provide them before proceeding, OR
   b) Build everything possible and clearly mark where the user must add their credentials
4. Include all necessary imports, error handling, and edge cases
5. Follow industry best practices and security standards
6. Write real implementations, not stubs or examples

When you need information from the user (API keys, credentials, specific requirements), explicitly ask for it.
When you can build functionality without external dependencies, build it completely.`

	switch capability {
	case CapabilityCodeReview:
		return fmt.Sprintf("%s\n\nFocus on thorough code analysis, identifying bugs, security issues, performance problems, and suggesting concrete improvements with real code fixes.", basePrompt)

	case CapabilityDebugging:
		return fmt.Sprintf("%s\n\nYou are debugging code. Identify the root cause of issues, explain why they occur, and provide complete, working fixes - never partial solutions.", basePrompt)

	case CapabilityDocumentation:
		return fmt.Sprintf("%s\n\nGenerate comprehensive, clear documentation with real code examples that actually work. No placeholder examples.", basePrompt)

	case CapabilityRefactoring:
		return fmt.Sprintf("%s\n\nAnalyze code for refactoring opportunities. Provide complete refactored code following best practices for %s - not suggestions, but actual refactored implementations.", basePrompt, language)

	default:
		return fmt.Sprintf("%s\n\nAssist with %s development tasks. Every piece of code you output must be complete and production-ready.", basePrompt, language)
	}
}

// buildUserPrompt constructs the user prompt based on the request
func (c *ClaudeClient) buildUserPrompt(req *AIRequest) string {
	prompt := req.Prompt

	if req.Code != "" {
		prompt += fmt.Sprintf("\n\nCode to analyze:\n```%s\n%s\n```", req.Language, req.Code)
	}

	if req.Context != nil {
		if projectInfo, ok := req.Context["project_info"].(string); ok {
			prompt += fmt.Sprintf("\n\nProject context: %s", projectInfo)
		}
	}

	return prompt
}

// makeRequest sends HTTP request to Claude API
func (c *ClaudeClient) makeRequest(ctx context.Context, req *claudeRequest) (*claudeResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Parse specific error types for better error messages
		switch resp.StatusCode {
		case 429:
			return nil, fmt.Errorf("RATE_LIMIT: Claude API rate limit exceeded. Please wait before retrying")
		case 403:
			return nil, fmt.Errorf("FORBIDDEN: Claude API access denied - check API key permissions")
		case 401:
			return nil, fmt.Errorf("UNAUTHORIZED: Invalid Claude API key")
		case 402:
			return nil, fmt.Errorf("QUOTA_EXCEEDED: Claude API quota exhausted. Add credits or use another provider")
		case 500, 502, 503, 504, 529:
			return nil, fmt.Errorf("SERVICE_ERROR: Claude service temporarily unavailable (status %d)", resp.StatusCode)
		default:
			return nil, fmt.Errorf("API_ERROR: Claude request failed with status %d: %s", resp.StatusCode, string(body))
		}
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if claudeResp.Error != nil {
		return nil, fmt.Errorf("Claude API error: %s", claudeResp.Error.Message)
	}

	return &claudeResp, nil
}

// GetCapabilities returns capabilities Claude excels at
func (c *ClaudeClient) GetCapabilities() []AICapability {
	return []AICapability{
		CapabilityCodeReview,
		CapabilityDebugging,
		CapabilityDocumentation,
		CapabilityRefactoring,
		CapabilityCodeGeneration,
		CapabilityExplanation,
	}
}

// GetProvider returns the provider identifier
func (c *ClaudeClient) GetProvider() AIProvider {
	return ProviderClaude
}

// Health checks if Claude API is accessible
func (c *ClaudeClient) Health(ctx context.Context) error {
	testReq := &claudeRequest{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 5,
		Messages: []claudeMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
	}

	_, err := c.makeRequest(ctx, testReq)
	return err
}

// getMaxTokens determines appropriate max tokens based on request
func (c *ClaudeClient) getMaxTokens(req *AIRequest) int {
	if req.MaxTokens > 0 {
		return req.MaxTokens
	}

	switch req.Capability {
	case CapabilityCodeCompletion:
		return 500
	case CapabilityCodeGeneration:
		return 2000
	case CapabilityCodeReview:
		return 1500
	case CapabilityDocumentation:
		return 3000
	default:
		return 1000
	}
}

// calculateCost estimates cost based on Claude pricing
func (c *ClaudeClient) calculateCost(inputTokens, outputTokens int) float64 {
	// Claude 3.5 Sonnet pricing (as of 2024)
	inputCostPer1K := 0.003   // $0.003 per 1K input tokens
	outputCostPer1K := 0.015  // $0.015 per 1K output tokens

	inputCost := float64(inputTokens) / 1000.0 * inputCostPer1K
	outputCost := float64(outputTokens) / 1000.0 * outputCostPer1K

	return inputCost + outputCost
}

// updateUsage updates internal usage statistics (thread-safe)
func (c *ClaudeClient) updateUsage(totalTokens int, cost float64, duration time.Duration) {
	c.usageMu.Lock()
	defer c.usageMu.Unlock()

	c.usage.RequestCount++
	c.usage.TotalTokens += int64(totalTokens)
	c.usage.TotalCost += cost
	c.usage.AvgLatency = (c.usage.AvgLatency*float64(c.usage.RequestCount-1) + duration.Seconds()) / float64(c.usage.RequestCount)
	c.usage.LastUsed = time.Now()
}

// incrementErrorCount safely increments the error count
func (c *ClaudeClient) incrementErrorCount() {
	c.usageMu.Lock()
	defer c.usageMu.Unlock()
	c.usage.ErrorCount++
}

// GetUsage returns current usage statistics (thread-safe copy)
func (c *ClaudeClient) GetUsage() *ProviderUsage {
	c.usageMu.RLock()
	defer c.usageMu.RUnlock()

	// Return a copy to prevent data races
	return &ProviderUsage{
		Provider:     c.usage.Provider,
		RequestCount: c.usage.RequestCount,
		TotalTokens:  c.usage.TotalTokens,
		TotalCost:    c.usage.TotalCost,
		AvgLatency:   c.usage.AvgLatency,
		ErrorCount:   c.usage.ErrorCount,
		LastUsed:     c.usage.LastUsed,
	}
}