package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// OllamaClient implements the Ollama AI API client (local or cloud).
// Ollama uses an OpenAI-compatible chat completions endpoint.
// https://ollama.com/blog/openai-compatibility
type OllamaClient struct {
	baseURL    string
	apiKey     string
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

// NewOllamaClient creates a new Ollama API client.
// apiKey is the Ollama Pro cloud API key; leave empty for local installs.
// Also supports embedded API key in URL: http://apikey@host (if apiKey param is empty).
// baseURL may include or omit the /v1 suffix — normalized away internally.
func NewOllamaClient(baseURL, apiKey string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	// Extract API key from embedded auth in URL as fallback (e.g., http://apikey@host/v1)
	if apiKey == "" && strings.Contains(baseURL, "@") {
		if parsed, err := url.Parse(baseURL); err == nil {
			if parsed.User != nil {
				apiKey = parsed.User.Username()
				parsed.User = nil
				baseURL = parsed.String()
			}
		}
	}
	// Strip trailing /v1 so makeRequest can always append /v1/<path> without doubling it.
	baseURL = strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/v1")
	return &OllamaClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 900 * time.Second,
		},
		usage: &ProviderUsage{
			Provider: ProviderOllama,
			LastUsed: time.Now(),
		},
	}
}

// NewOllamaCloudClient creates an Ollama client configured for Ollama Cloud (e.g., kimi-k2.6:cloud)
func NewOllamaCloudClient(baseURL, apiKey string) *OllamaClient {
	if baseURL == "" {
		baseURL = "https://ollama.com/v1"
	}
	return &OllamaClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Minute, // 15-minute timeout for large cloud models
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

	// Check if this is an Ollama Cloud client (has apiKey set)
	isCloud := o.apiKey != ""

	// DeepSeek and GLM provider emulation: route to their specific models on Ollama Cloud
	if req.Provider == ProviderDeepSeek {
		if isCloud {
			return "deepseek-v3.2"
		}
		return "deepseek-r1:14b"
	}
	if req.Provider == ProviderGLM {
		if isCloud {
			return "glm-5.1"
		}
		return "glm-4.5"
	}

	// Default models based on capability
	// For Ollama Cloud, use kimi-k2.6:cloud as the default
	// For local Ollama, use deepseek-r1:14b
	switch req.Capability {
	case CapabilityCodeGeneration, CapabilityRefactoring, CapabilityArchitecture:
		if isCloud {
			return "kimi-k2.6:cloud" // Best for complex code tasks (cloud)
		}
		return "deepseek-r1:14b" // Best for complex code tasks (local)
	case CapabilityCodeCompletion:
		if isCloud {
			return "kimi-k2.6:cloud" // Fast completions via cloud
		}
		return "deepseek-r1:14b" // Fast for completions
	case CapabilityCodeReview, CapabilityDebugging:
		if isCloud {
			return "kimi-k2.6:cloud" // Good at analysis (cloud)
		}
		return "deepseek-r1:14b" // Good at analysis (local)
	default:
		if isCloud {
			return "kimi-k2.6:cloud" // General purpose fallback (cloud)
		}
		return "deepseek-r1:14b" // General purpose fallback
	}
}

// makeRequest sends HTTP request to Ollama API
func (o *OllamaClient) makeRequest(ctx context.Context, req *ollamaRequest) (*ollamaResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := o.baseURL + "/v1/chat/completions"

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("ngrok-skip-browser-warning", "true")
	if o.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

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

// Health checks if Ollama server is accessible.
// Uses /v1/models (OpenAI-compatible) which works for both local and cloud.
func (o *OllamaClient) Health(ctx context.Context) error {
	// For Ollama Cloud (OpenAI-compatible), try OpenAI-compatible endpoints first.
	endpoints := []string{"/v1/models", "/v1/health", "/api/tags"}

	for _, path := range endpoints {
		url := o.baseURL + path
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}
		if o.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+o.apiKey)
		}
		req.Header.Set("ngrok-skip-browser-warning", "true")

		resp, err := o.httpClient.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()

		// 200 = healthy, 401 = auth failed (bad key — treat as unhealthy so router falls back)
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("Ollama auth failed (401): API key is invalid or missing")
		}
	}

	return fmt.Errorf("Ollama server not reachable at %s (tried %v)", o.baseURL, endpoints)
}

// openAIModelsResponse is the response from GET /v1/models
type openAIModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// GetAvailableModels fetches available models using the OpenAI-compatible /v1/models endpoint.
func (o *OllamaClient) GetAvailableModels(ctx context.Context) ([]string, error) {
	url := o.baseURL + "/v1/models"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("ngrok-skip-browser-warning", "true")
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch models: status %d", resp.StatusCode)
	}

	var modelsResp openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	models := make([]string, len(modelsResp.Data))
	for i, m := range modelsResp.Data {
		models[i] = m.ID
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

// SetAPIKey configures the API key for Ollama Cloud BYOK usage
func (o *OllamaClient) SetAPIKey(key string) {
	o.usageMu.Lock()
	defer o.usageMu.Unlock()
	o.apiKey = key
}

// GetAPIKey returns the configured API key
func (o *OllamaClient) GetAPIKey() string {
	o.usageMu.RLock()
	defer o.usageMu.RUnlock()
	return o.apiKey
}
