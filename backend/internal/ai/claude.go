package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ClaudeClient implements the Claude/Anthropic API client
type ClaudeClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	usage      *ProviderUsage
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

	// Create Claude API request
	claudeReq := &claudeRequest{
		Model:     "claude-sonnet-4-20250514",  // Latest Claude model
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
		c.usage.ErrorCount++
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
	basePrompt := "You are an expert software developer and code assistant for APEX.BUILD, a cloud development platform."

	switch capability {
	case CapabilityCodeReview:
		return fmt.Sprintf("%s Focus on thorough code analysis, identifying bugs, security issues, performance problems, and suggesting improvements. Provide detailed explanations for your findings.", basePrompt)

	case CapabilityDebugging:
		return fmt.Sprintf("%s You are debugging code. Identify the root cause of issues, explain why they occur, and provide specific fixes. Be methodical and thorough in your analysis.", basePrompt)

	case CapabilityDocumentation:
		return fmt.Sprintf("%s Generate comprehensive, clear documentation that explains code functionality, usage, and examples. Make it accessible to both beginners and experts.", basePrompt)

	case CapabilityRefactoring:
		return fmt.Sprintf("%s Analyze code for refactoring opportunities. Focus on improving readability, maintainability, performance, and following best practices for %s.", basePrompt, language)

	default:
		return fmt.Sprintf("%s Assist with %s development tasks. Provide high-quality, production-ready code and explanations.", basePrompt, language)
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
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
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
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 10,
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

// GetUsage returns current usage statistics
func (c *ClaudeClient) GetUsage() *ProviderUsage {
	return c.usage
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

// updateUsage updates internal usage statistics
func (c *ClaudeClient) updateUsage(totalTokens int, cost float64, duration time.Duration) {
	c.usage.RequestCount++
	c.usage.TotalTokens += int64(totalTokens)
	c.usage.TotalCost += cost
	c.usage.AvgLatency = (c.usage.AvgLatency*float64(c.usage.RequestCount-1) + duration.Seconds()) / float64(c.usage.RequestCount)
	c.usage.LastUsed = time.Now()
}