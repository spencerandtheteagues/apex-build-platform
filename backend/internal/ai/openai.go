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

// OpenAIClient implements the OpenAI GPT-4 API client
type OpenAIClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	usage      *ProviderUsage
}

// OpenAI API request/response structures
type openAIRequest struct {
	Model       string            `json:"model"`
	Messages    []openAIMessage   `json:"messages"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Temperature float32           `json:"temperature,omitempty"`
	Stream      bool              `json:"stream"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
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

// NewOpenAIClient creates a new OpenAI API client
func NewOpenAIClient(apiKey string) *OpenAIClient {
	return &OpenAIClient{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1/chat/completions",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		usage: &ProviderUsage{
			Provider: ProviderGPT4,
			LastUsed: time.Now(),
		},
	}
}

// Generate implements the AIClient interface for OpenAI GPT-4
func (o *OpenAIClient) Generate(ctx context.Context, req *AIRequest) (*AIResponse, error) {
	startTime := time.Now()

	// Build messages with system and user prompts
	messages := o.buildMessages(req)

	// Create OpenAI API request
	openAIReq := &openAIRequest{
		Model:       o.getModelForCapability(req.Capability),
		Messages:    messages,
		MaxTokens:   o.getMaxTokens(req),
		Temperature: req.Temperature,
		Stream:      false,
	}

	// Make API request
	resp, err := o.makeRequest(ctx, openAIReq)
	if err != nil {
		o.usage.ErrorCount++
		return &AIResponse{
			ID:        req.ID,
			Provider:  ProviderGPT4,
			Error:     err.Error(),
			Duration:  time.Since(startTime),
			CreatedAt: time.Now(),
		}, err
	}

	// Calculate cost based on OpenAI pricing
	cost := o.calculateCost(resp.Usage.PromptTokens, resp.Usage.CompletionTokens, openAIReq.Model)

	// Update usage statistics
	o.updateUsage(resp.Usage.TotalTokens, cost, time.Since(startTime))

	// Extract response content
	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	return &AIResponse{
		ID:       req.ID,
		Provider: ProviderGPT4,
		Content:  content,
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

// buildMessages creates the message array for OpenAI API
func (o *OpenAIClient) buildMessages(req *AIRequest) []openAIMessage {
	messages := []openAIMessage{}

	// Add system message based on capability
	systemPrompt := o.buildSystemPrompt(req.Capability, req.Language)
	messages = append(messages, openAIMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	// Add user message
	userPrompt := o.buildUserPrompt(req)
	messages = append(messages, openAIMessage{
		Role:    "user",
		Content: userPrompt,
	})

	return messages
}

// buildSystemPrompt creates capability-specific system prompts for GPT-4
func (o *OpenAIClient) buildSystemPrompt(capability AICapability, language string) string {
	basePrompt := "You are an expert software developer assistant for APEX.BUILD, a next-generation cloud development platform."

	switch capability {
	case CapabilityCodeGeneration:
		return fmt.Sprintf("%s You excel at generating high-quality, production-ready code. Focus on clean, efficient, well-documented code that follows best practices for %s.", basePrompt, language)

	case CapabilityTesting:
		return fmt.Sprintf("%s Generate comprehensive test suites including unit tests, integration tests, and edge cases. Write tests that are maintainable and provide good coverage for %s.", basePrompt, language)

	case CapabilityRefactoring:
		return fmt.Sprintf("%s Analyze code and suggest refactoring improvements. Focus on code organization, performance optimization, and modern %s patterns.", basePrompt, language)

	case CapabilityCodeCompletion:
		return fmt.Sprintf("%s Provide intelligent code completions and suggestions. Consider context, best practices, and idiomatic %s patterns.", basePrompt, language)

	default:
		return fmt.Sprintf("%s Assist with %s development tasks with focus on practical, working solutions.", basePrompt, language)
	}
}

// buildUserPrompt constructs the user prompt
func (o *OpenAIClient) buildUserPrompt(req *AIRequest) string {
	prompt := req.Prompt

	if req.Code != "" {
		prompt += fmt.Sprintf("\n\nHere's the code:\n```%s\n%s\n```", req.Language, req.Code)
	}

	// Add context if available
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

// makeRequest sends HTTP request to OpenAI API
func (o *OpenAIClient) makeRequest(ctx context.Context, req *openAIRequest) (*openAIResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.apiKey))

	resp, err := o.httpClient.Do(httpReq)
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

	var openAIResp openAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if openAIResp.Error != nil {
		return nil, fmt.Errorf("OpenAI API error: %s", openAIResp.Error.Message)
	}

	return &openAIResp, nil
}

// getModelForCapability selects the best OpenAI model for the capability
func (o *OpenAIClient) getModelForCapability(capability AICapability) string {
	switch capability {
	case CapabilityCodeGeneration, CapabilityRefactoring, CapabilityTesting:
		return "gpt-4-turbo"  // Best for complex code tasks
	case CapabilityCodeCompletion:
		return "gpt-4"        // Fast enough for completions
	default:
		return "gpt-4"        // Default to GPT-4
	}
}

// GetCapabilities returns capabilities GPT-4 excels at
func (o *OpenAIClient) GetCapabilities() []AICapability {
	return []AICapability{
		CapabilityCodeGeneration,
		CapabilityTesting,
		CapabilityRefactoring,
		CapabilityCodeCompletion,
		CapabilityDebugging,
		CapabilityDocumentation,
	}
}

// GetProvider returns the provider identifier
func (o *OpenAIClient) GetProvider() AIProvider {
	return ProviderGPT4
}

// Health checks if OpenAI API is accessible
func (o *OpenAIClient) Health(ctx context.Context) error {
	testReq := &openAIRequest{
		Model: "gpt-4",
		Messages: []openAIMessage{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens: 5,
	}

	_, err := o.makeRequest(ctx, testReq)
	return err
}

// GetUsage returns current usage statistics
func (o *OpenAIClient) GetUsage() *ProviderUsage {
	return o.usage
}

// getMaxTokens determines appropriate max tokens based on request
func (o *OpenAIClient) getMaxTokens(req *AIRequest) int {
	if req.MaxTokens > 0 {
		return req.MaxTokens
	}

	switch req.Capability {
	case CapabilityCodeCompletion:
		return 300
	case CapabilityCodeGeneration:
		return 2500
	case CapabilityTesting:
		return 3000
	case CapabilityRefactoring:
		return 2000
	default:
		return 1500
	}
}

// calculateCost estimates cost based on OpenAI pricing
func (o *OpenAIClient) calculateCost(inputTokens, outputTokens int, model string) float64 {
	// OpenAI pricing (as of 2024)
	var inputCostPer1K, outputCostPer1K float64

	switch model {
	case "gpt-4-turbo":
		inputCostPer1K = 0.01   // $0.01 per 1K input tokens
		outputCostPer1K = 0.03  // $0.03 per 1K output tokens
	case "gpt-4":
		inputCostPer1K = 0.03   // $0.03 per 1K input tokens
		outputCostPer1K = 0.06  // $0.06 per 1K output tokens
	default:
		inputCostPer1K = 0.01
		outputCostPer1K = 0.03
	}

	inputCost := float64(inputTokens) / 1000.0 * inputCostPer1K
	outputCost := float64(outputTokens) / 1000.0 * outputCostPer1K

	return inputCost + outputCost
}

// updateUsage updates internal usage statistics
func (o *OpenAIClient) updateUsage(totalTokens int, cost float64, duration time.Duration) {
	o.usage.RequestCount++
	o.usage.TotalTokens += int64(totalTokens)
	o.usage.TotalCost += cost
	o.usage.AvgLatency = (o.usage.AvgLatency*float64(o.usage.RequestCount-1) + duration.Seconds()) / float64(o.usage.RequestCount)
	o.usage.LastUsed = time.Now()
}