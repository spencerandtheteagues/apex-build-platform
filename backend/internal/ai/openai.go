package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// OpenAIClient implements the OpenAI GPT-4 API client
type OpenAIClient struct {
	apiKey       string
	baseURL      string
	responsesURL string
	httpClient   *http.Client
	usage        *ProviderUsage
	usageMu      sync.RWMutex // Protects usage statistics
}

// OpenAI API request/response structures
type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float32         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream"`
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

type openAIResponsesRequest struct {
	Model           string                 `json:"model"`
	Input           []openAIResponsesInput `json:"input"`
	MaxOutputTokens int                    `json:"max_output_tokens,omitempty"`
	Temperature     float32                `json:"temperature,omitempty"`
}

type openAIResponsesInput struct {
	Role    string                          `json:"role"`
	Content []openAIResponsesInputTextBlock `json:"content"`
}

type openAIResponsesInputTextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type openAIResponsesAPIResponse struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Output []struct {
		Type    string `json:"type"`
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	OutputText string `json:"output_text,omitempty"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// NewOpenAIClient creates a new OpenAI API client
func NewOpenAIClient(apiKey string) *OpenAIClient {
	apiKey = normalizeAPIKey(apiKey)
	return &OpenAIClient{
		apiKey:       apiKey,
		baseURL:      "https://api.openai.com/v1/chat/completions",
		responsesURL: "https://api.openai.com/v1/responses",
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
	model := o.getModelForRequest(req)

	// Build messages with system and user prompts
	messages := o.buildMessages(req)

	// Create OpenAI API request
	openAIReq := &openAIRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   o.getMaxTokens(req),
		Temperature: req.Temperature,
		Stream:      false,
	}

	var (
		content        string
		inputTokens    int
		outputTokens   int
		totalTokens    int
		effectiveModel string
		err            error
	)

	if o.shouldUseResponsesAPI(model) {
		var responsesResp *openAIResponsesAPIResponse
		responsesResp, err = o.makeResponsesRequest(ctx, model, messages, openAIReq.MaxTokens, openAIReq.Temperature)
		if err == nil {
			content = o.extractResponsesText(responsesResp)
			inputTokens = responsesResp.Usage.InputTokens
			outputTokens = responsesResp.Usage.OutputTokens
			totalTokens = responsesResp.Usage.TotalTokens
			effectiveModel = responsesResp.Model
		} else {
			// Fallback keeps builds progressing when newer models are temporarily unavailable.
			openAIReq.Model = "gpt-4o-mini"
			var chatResp *openAIResponse
			chatResp, err = o.makeRequest(ctx, openAIReq)
			if err == nil {
				if len(chatResp.Choices) > 0 {
					content = chatResp.Choices[0].Message.Content
				}
				inputTokens = chatResp.Usage.PromptTokens
				outputTokens = chatResp.Usage.CompletionTokens
				totalTokens = chatResp.Usage.TotalTokens
				effectiveModel = chatResp.Model
			}
		}
	} else {
		var chatResp *openAIResponse
		chatResp, err = o.makeRequest(ctx, openAIReq)
		if err == nil {
			if len(chatResp.Choices) > 0 {
				content = chatResp.Choices[0].Message.Content
			}
			inputTokens = chatResp.Usage.PromptTokens
			outputTokens = chatResp.Usage.CompletionTokens
			totalTokens = chatResp.Usage.TotalTokens
			effectiveModel = chatResp.Model
		}
	}

	if err != nil {
		o.incrementErrorCount()
		return &AIResponse{
			ID:        req.ID,
			Provider:  ProviderGPT4,
			Error:     err.Error(),
			Duration:  time.Since(startTime),
			CreatedAt: time.Now(),
		}, err
	}

	// Calculate cost based on OpenAI pricing
	if effectiveModel == "" {
		effectiveModel = openAIReq.Model
	}
	cost := o.calculateCost(inputTokens, outputTokens, effectiveModel)

	// Update usage statistics
	o.updateUsage(totalTokens, cost, time.Since(startTime))

	return &AIResponse{
		ID:       req.ID,
		Provider: ProviderGPT4,
		Content:  content,
		Metadata: map[string]interface{}{
			"model": effectiveModel,
		},
		Usage: &Usage{
			PromptTokens:     inputTokens,
			CompletionTokens: outputTokens,
			TotalTokens:      totalTokens,
			Cost:             cost,
		},
		Duration:  time.Since(startTime),
		CreatedAt: time.Now(),
	}, nil
}

func (o *OpenAIClient) shouldUseResponsesAPI(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(model, "gpt-5")
}

func (o *OpenAIClient) extractResponsesText(resp *openAIResponsesAPIResponse) string {
	if resp == nil {
		return ""
	}
	if strings.TrimSpace(resp.OutputText) != "" {
		return resp.OutputText
	}
	var out strings.Builder
	for _, item := range resp.Output {
		for _, block := range item.Content {
			if strings.TrimSpace(block.Text) == "" {
				continue
			}
			if out.Len() > 0 {
				out.WriteString("\n")
			}
			out.WriteString(block.Text)
		}
	}
	return out.String()
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
	case CapabilityCodeGeneration:
		return fmt.Sprintf("%s\n\nExcel at generating production-ready code. Every function must be complete and working. No placeholder implementations for %s.", basePrompt, language)

	case CapabilityTesting:
		return fmt.Sprintf("%s\n\nGenerate comprehensive, executable test suites with real assertions. Tests must actually run and verify functionality for %s.", basePrompt, language)

	case CapabilityRefactoring:
		return fmt.Sprintf("%s\n\nProvide complete refactored code, not suggestions. Output the entire refactored implementation following modern %s patterns.", basePrompt, language)

	case CapabilityCodeCompletion:
		return fmt.Sprintf("%s\n\nProvide intelligent, complete code that can be used immediately. Follow idiomatic %s patterns.", basePrompt, language)

	default:
		return fmt.Sprintf("%s\n\nAssist with %s development. Every output must be production-ready and immediately usable.", basePrompt, language)
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
		// Parse specific error types for better error messages
		switch resp.StatusCode {
		case 429:
			return nil, fmt.Errorf("RATE_LIMIT: OpenAI API rate limit exceeded. Please wait before retrying")
		case 403:
			return nil, fmt.Errorf("FORBIDDEN: OpenAI API access denied - check API key permissions")
		case 401:
			return nil, fmt.Errorf("UNAUTHORIZED: Invalid OpenAI API key")
		case 402:
			return nil, fmt.Errorf("QUOTA_EXCEEDED: OpenAI API quota exhausted. Add credits or use another provider")
		case 500, 502, 503, 504:
			return nil, fmt.Errorf("SERVICE_ERROR: OpenAI service temporarily unavailable (status %d)", resp.StatusCode)
		default:
			return nil, fmt.Errorf("API_ERROR: OpenAI request failed with status %d: %s", resp.StatusCode, string(body))
		}
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

func (o *OpenAIClient) makeResponsesRequest(
	ctx context.Context,
	model string,
	messages []openAIMessage,
	maxOutputTokens int,
	temperature float32,
) (*openAIResponsesAPIResponse, error) {
	input := make([]openAIResponsesInput, 0, len(messages))
	for _, msg := range messages {
		input = append(input, openAIResponsesInput{
			Role: msg.Role,
			Content: []openAIResponsesInputTextBlock{
				{
					Type: "input_text",
					Text: msg.Content,
				},
			},
		})
	}

	req := &openAIResponsesRequest{
		Model:           model,
		Input:           input,
		MaxOutputTokens: maxOutputTokens,
		Temperature:     temperature,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal responses request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.responsesURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create responses request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.apiKey))

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make responses request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read responses response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case 429:
			return nil, fmt.Errorf("RATE_LIMIT: OpenAI API rate limit exceeded. Please wait before retrying")
		case 403:
			return nil, fmt.Errorf("FORBIDDEN: OpenAI API access denied - check API key permissions")
		case 401:
			return nil, fmt.Errorf("UNAUTHORIZED: Invalid OpenAI API key")
		case 402:
			return nil, fmt.Errorf("QUOTA_EXCEEDED: OpenAI API quota exhausted. Add credits or use another provider")
		case 500, 502, 503, 504:
			return nil, fmt.Errorf("SERVICE_ERROR: OpenAI service temporarily unavailable (status %d)", resp.StatusCode)
		default:
			return nil, fmt.Errorf("API_ERROR: OpenAI responses request failed with status %d: %s", resp.StatusCode, string(body))
		}
	}

	var openAIResp openAIResponsesAPIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal responses response: %w", err)
	}
	if openAIResp.Error != nil {
		return nil, fmt.Errorf("OpenAI API error: %s", openAIResp.Error.Message)
	}
	return &openAIResp, nil
}

// getModelForRequest selects model respecting explicit override
func (o *OpenAIClient) getModelForRequest(req *AIRequest) string {
	if req.Model != "" {
		return req.Model
	}
	return o.getModelForCapability(req.Capability)
}

// getModelForCapability selects the best OpenAI model for the capability
// This is the fallback when no explicit model is set via power mode
func (o *OpenAIClient) getModelForCapability(capability AICapability) string {
	switch capability {
	case CapabilityCodeGeneration, CapabilityRefactoring, CapabilityTesting:
		return "gpt-4o-mini" // Default to fast/cheap model; power mode overrides for premium
	case CapabilityCodeCompletion:
		return "gpt-4o-mini" // Fast completions
	default:
		return "gpt-4o-mini" // Default to cheapest
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
		Model: "gpt-4o-mini", // Use cheapest model for health checks
		Messages: []openAIMessage{
			{Role: "user", Content: "Hi"},
		},
		MaxTokens: 3,
	}

	_, err := o.makeRequest(ctx, testReq)
	return err
}

// GetUsage returns current usage statistics (thread-safe copy)
func (o *OpenAIClient) GetUsage() *ProviderUsage {
	o.usageMu.RLock()
	defer o.usageMu.RUnlock()

	// Return a copy to prevent data races
	return &ProviderUsage{
		Provider:     o.usage.Provider,
		RequestCount: o.usage.RequestCount,
		TotalTokens:  o.usage.TotalTokens,
		TotalCost:    o.usage.TotalCost,
		AvgLatency:   o.usage.AvgLatency,
		ErrorCount:   o.usage.ErrorCount,
		LastUsed:     o.usage.LastUsed,
	}
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
	// OpenAI pricing (as of 2024-2025)
	var inputCostPer1K, outputCostPer1K float64

	switch model {
	case "gpt-5.2-codex":
		inputCostPer1K = 0.008
		outputCostPer1K = 0.024
	case "gpt-5":
		inputCostPer1K = 0.005
		outputCostPer1K = 0.015
	case "gpt-4o":
		inputCostPer1K = 0.0025
		outputCostPer1K = 0.01
	case "gpt-4o-mini":
		inputCostPer1K = 0.00015
		outputCostPer1K = 0.0006
	default:
		inputCostPer1K = 0.00015 // Default to cheapest pricing
		outputCostPer1K = 0.0006
	}

	inputCost := float64(inputTokens) / 1000.0 * inputCostPer1K
	outputCost := float64(outputTokens) / 1000.0 * outputCostPer1K

	return inputCost + outputCost
}

// updateUsage updates internal usage statistics (thread-safe)
func (o *OpenAIClient) updateUsage(totalTokens int, cost float64, duration time.Duration) {
	o.usageMu.Lock()
	defer o.usageMu.Unlock()

	o.usage.RequestCount++
	o.usage.TotalTokens += int64(totalTokens)
	o.usage.TotalCost += cost
	o.usage.AvgLatency = (o.usage.AvgLatency*float64(o.usage.RequestCount-1) + duration.Seconds()) / float64(o.usage.RequestCount)
	o.usage.LastUsed = time.Now()
}

// incrementErrorCount safely increments the error count
func (o *OpenAIClient) incrementErrorCount() {
	o.usageMu.Lock()
	defer o.usageMu.Unlock()
	o.usage.ErrorCount++
}
