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

// OllamaClient implements the Ollama local AI API client
// Ollama uses an OpenAI-compatible chat completions endpoint
// https://ollama.com/blog/openai-compatibility
type OllamaClient struct {
	baseURL    string
	httpClient *http.Client
	usage      *ProviderUsage
	usageMu    sync.RWMutex
}

// Ollama uses OpenAI-compatible request/response format
type ollamaRequest struct {
	Model       string          `json:"model"`
	Messages    []ollamaMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float32         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaResponse struct {
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

// ollamaModelsResponse represents the response from /api/tags
type ollamaModelsResponse struct {
	Models []struct {
		Name       string `json:"name"`
		ModifiedAt string `json:"modified_at"`
		Size       int64  `json:"size"`
	} `json:"models"`
}

// NewOllamaClient creates a new Ollama API client
func NewOllamaClient(baseURL string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 900 * time.Second, // 15-minute timeout for local inference (large models can be slow)
		},
		usage: &ProviderUsage{
			Provider: ProviderOllama,
			LastUsed: time.Now(),
		},
	}
}

// Generate implements the AIClient interface for Ollama
func (o *OllamaClient) Generate(ctx context.Context, req *AIRequest) (*AIResponse, error) {
	startTime := time.Now()

	messages := o.buildMessages(req)
	model := o.getModel(req)

	ollamaReq := &ollamaRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   o.getMaxTokens(req),
		Temperature: req.Temperature,
		Stream:      false,
	}

	resp, err := o.makeRequest(ctx, ollamaReq)
	if err != nil {
		o.incrementErrorCount()
		return &AIResponse{
			ID:        req.ID,
			Provider:  ProviderOllama,
			Error:     err.Error(),
			Duration:  time.Since(startTime),
			CreatedAt: time.Now(),
		}, err
	}

	// Ollama runs locally — cost is always $0
	cost := 0.0
	o.updateUsage(resp.Usage.TotalTokens, cost, time.Since(startTime))

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	return &AIResponse{
		ID:       req.ID,
		Provider: ProviderOllama,
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

// buildMessages creates the message array for Ollama API
func (o *OllamaClient) buildMessages(req *AIRequest) []ollamaMessage {
	messages := []ollamaMessage{}

	systemPrompt := o.buildSystemPrompt(req.Capability, req.Language)
	messages = append(messages, ollamaMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	userPrompt := o.buildUserPrompt(req)
	messages = append(messages, ollamaMessage{
		Role:    "user",
		Content: userPrompt,
	})

	return messages
}

// buildSystemPrompt creates capability-specific system prompts
func (o *OllamaClient) buildSystemPrompt(capability AICapability, language string) string {
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
func (o *OllamaClient) buildUserPrompt(req *AIRequest) string {
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

// getModel selects the appropriate Ollama model
func (o *OllamaClient) getModel(req *AIRequest) string {
	// Respect explicit model override
	if req.Model != "" {
		return req.Model
	}

	// Default models based on capability
	// Users should have these installed via `ollama pull <model>`
	switch req.Capability {
	case CapabilityCodeGeneration, CapabilityRefactoring, CapabilityArchitecture:
		return "deepseek-r1:14b" // Best for complex code tasks (local)
	case CapabilityCodeCompletion:
		return "deepseek-r1:14b" // Fast for completions
	case CapabilityCodeReview, CapabilityDebugging:
		return "deepseek-r1:14b" // Good at analysis (local)
	default:
		return "deepseek-r1:14b" // General purpose fallback
	}
}

// makeRequest sends HTTP request to Ollama API
func (o *OllamaClient) makeRequest(ctx context.Context, req *ollamaRequest) (*ollamaResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Ollama's OpenAI-compatible endpoint
	url := o.baseURL + "/v1/chat/completions"

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	// Bypass ngrok browser warning for free tunnels
	httpReq.Header.Set("ngrok-skip-browser-warning", "true")
	// No Authorization header — Ollama runs locally without auth

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ollama server at %s: %w", o.baseURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case 404:
			return nil, fmt.Errorf("MODEL_NOT_FOUND: Model '%s' not installed. Run: ollama pull %s", req.Model, req.Model)
		case 500, 502, 503, 504:
			return nil, fmt.Errorf("SERVICE_ERROR: Ollama server error (status %d). Is Ollama running?", resp.StatusCode)
		default:
			return nil, fmt.Errorf("API_ERROR: Ollama request failed with status %d: %s", resp.StatusCode, string(body))
		}
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if ollamaResp.Error != nil {
		return nil, fmt.Errorf("Ollama API error: %s", ollamaResp.Error.Message)
	}

	return &ollamaResp, nil
}

// GetCapabilities returns capabilities Ollama supports
func (o *OllamaClient) GetCapabilities() []AICapability {
	return []AICapability{
		CapabilityCodeGeneration,
		CapabilityCodeReview,
		CapabilityDebugging,
		CapabilityRefactoring,
		CapabilityTesting,
		CapabilityDocumentation,
		CapabilityCodeCompletion,
		CapabilityExplanation,
	}
}

// GetProvider returns the provider identifier
func (o *OllamaClient) GetProvider() AIProvider {
	return ProviderOllama
}

// Health checks if Ollama server is accessible
func (o *OllamaClient) Health(ctx context.Context) error {
	url := o.baseURL + "/api/tags"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	// Bypass ngrok browser warning for free tunnels
	req.Header.Set("ngrok-skip-browser-warning", "true")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Ollama server not reachable at %s: %w", o.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// GetAvailableModels fetches the list of installed models from Ollama
func (o *OllamaClient) GetAvailableModels(ctx context.Context) ([]string, error) {
	url := o.baseURL + "/api/tags"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Bypass ngrok browser warning for free tunnels
	req.Header.Set("ngrok-skip-browser-warning", "true")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch models: status %d", resp.StatusCode)
	}

	var modelsResp ollamaModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	models := make([]string, len(modelsResp.Models))
	for i, m := range modelsResp.Models {
		models[i] = m.Name
	}

	return models, nil
}

// GetUsage returns current usage statistics
func (o *OllamaClient) GetUsage() *ProviderUsage {
	o.usageMu.RLock()
	defer o.usageMu.RUnlock()

	return &ProviderUsage{
		Provider:     o.usage.Provider,
		RequestCount: o.usage.RequestCount,
		TotalTokens:  o.usage.TotalTokens,
		TotalCost:    o.usage.TotalCost, // Always 0 for Ollama
		AvgLatency:   o.usage.AvgLatency,
		ErrorCount:   o.usage.ErrorCount,
		LastUsed:     o.usage.LastUsed,
	}
}

// getMaxTokens determines appropriate max tokens
func (o *OllamaClient) getMaxTokens(req *AIRequest) int {
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

// updateUsage updates internal usage statistics
func (o *OllamaClient) updateUsage(totalTokens int, cost float64, duration time.Duration) {
	o.usageMu.Lock()
	defer o.usageMu.Unlock()

	o.usage.RequestCount++
	o.usage.TotalTokens += int64(totalTokens)
	o.usage.TotalCost += cost // Will always be 0 for Ollama

	// Safe incremental average calculation (prevents division by zero)
	if o.usage.RequestCount > 1 {
		o.usage.AvgLatency = (o.usage.AvgLatency*float64(o.usage.RequestCount-1) + duration.Seconds()) / float64(o.usage.RequestCount)
	} else {
		o.usage.AvgLatency = duration.Seconds()
	}
	o.usage.LastUsed = time.Now()
}

// incrementErrorCount safely increments the error count
func (o *OllamaClient) incrementErrorCount() {
	o.usageMu.Lock()
	defer o.usageMu.Unlock()
	o.usage.ErrorCount++
}
