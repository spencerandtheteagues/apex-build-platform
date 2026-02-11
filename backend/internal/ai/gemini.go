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

// GeminiClient implements the Google Gemini API client
type GeminiClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	usage      *ProviderUsage
	usageMu    sync.RWMutex // Protects usage statistics
}

// Gemini API request/response structures
type geminiRequest struct {
	Contents         []geminiContent    `json:"contents"`
	SafetySettings   []geminySafety     `json:"safetySettings,omitempty"`
	GenerationConfig *geminiGenConfig   `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string         `json:"role,omitempty"`
	Parts []geminiPart   `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminySafety struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type geminiGenConfig struct {
	Temperature     float32 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	TopP            float32 `json:"topP,omitempty"`
	TopK            int     `json:"topK,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason   string `json:"finishReason"`
		Index          int    `json:"index"`
		SafetyRatings  []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

// NewGeminiClient creates a new Gemini API client
func NewGeminiClient(apiKey string) *GeminiClient {
	return &GeminiClient{
		apiKey:  apiKey,
		baseURL: "https://generativelanguage.googleapis.com/v1beta/models",
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		usage: &ProviderUsage{
			Provider: ProviderGemini,
			LastUsed: time.Now(),
		},
	}
}

// Generate implements the AIClient interface for Gemini
func (g *GeminiClient) Generate(ctx context.Context, req *AIRequest) (*AIResponse, error) {
	startTime := time.Now()

	// Build the request content
	systemPrompt := g.buildSystemPrompt(req.Capability, req.Language)
	userPrompt := g.buildUserPrompt(req)

	// Create Gemini API request
	geminiReq := &geminiRequest{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{
					{Text: systemPrompt + "\n\n" + userPrompt},
				},
			},
		},
		SafetySettings: []geminySafety{
			{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_MEDIUM_AND_ABOVE"},
			{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_MEDIUM_AND_ABOVE"},
			{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_MEDIUM_AND_ABOVE"},
			{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_MEDIUM_AND_ABOVE"},
		},
		GenerationConfig: &geminiGenConfig{
			Temperature:     req.Temperature,
			MaxOutputTokens: g.getMaxTokens(req),
			TopP:           0.8,
			TopK:           40,
		},
	}

	// Select appropriate model - respect explicit override
	model := g.getModelForCapability(req.Capability)
	if req.Model != "" {
		model = req.Model
	}
	url := fmt.Sprintf("%s/%s:generateContent?key=%s", g.baseURL, model, g.apiKey)

	// Make API request
	resp, err := g.makeRequest(ctx, url, geminiReq)
	if err != nil {
		g.incrementErrorCount()
		return &AIResponse{
			ID:        req.ID,
			Provider:  ProviderGemini,
			Error:     err.Error(),
			Duration:  time.Since(startTime),
			CreatedAt: time.Now(),
		}, err
	}

	// Calculate cost based on Gemini pricing
	cost := g.calculateCost(resp.UsageMetadata.PromptTokenCount, resp.UsageMetadata.CandidatesTokenCount, model)

	// Update usage statistics
	g.updateUsage(resp.UsageMetadata.TotalTokenCount, cost, time.Since(startTime))

	// Extract response content
	content := ""
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		content = resp.Candidates[0].Content.Parts[0].Text
	}

	return &AIResponse{
		ID:       req.ID,
		Provider: ProviderGemini,
		Content:  content,
		Usage: &Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
			Cost:             cost,
		},
		Duration:  time.Since(startTime),
		CreatedAt: time.Now(),
	}, nil
}

// buildSystemPrompt creates capability-specific system prompts for Gemini
func (g *GeminiClient) buildSystemPrompt(capability AICapability, language string) string {
	basePrompt := `You are an expert coding assistant for APEX.BUILD, a professional cloud development platform.

CRITICAL REQUIREMENTS - ALWAYS FOLLOW:
1. NEVER output demo code, mock data, placeholder content, or TODO comments
2. ALWAYS produce complete, production-ready, fully functional code
3. If external resources are needed (API keys, database credentials, third-party services), either:
   a) Ask the user to provide them before proceeding, OR
   b) Build everything possible and clearly mark where the user must add their credentials
4. Include all necessary imports, error handling, and edge cases
5. Follow industry best practices and security standards
6. Write real implementations, not stubs or examples

When you need information from the user, explicitly ask for it.
When you can build functionality without external dependencies, build it completely.`

	switch capability {
	case CapabilityCodeCompletion:
		return fmt.Sprintf("%s\n\nProvide complete, production-ready code completions for %s. No partial implementations.", basePrompt, language)

	case CapabilityExplanation:
		return fmt.Sprintf("%s\n\nExplain code with real, working examples in %s. No placeholder or demo code in examples.", basePrompt, language)

	case CapabilityDebugging:
		return fmt.Sprintf("%s\n\nIdentify and fix bugs with complete, working solutions in %s. Provide the full fixed code.", basePrompt, language)

	case CapabilityCodeGeneration:
		return fmt.Sprintf("%s\n\nGenerate complete, production-ready %s code. Every function must be fully implemented.", basePrompt, language)

	default:
		return fmt.Sprintf("%s\n\nAssist with %s development. All code must be production-ready.", basePrompt, language)
	}
}

// buildUserPrompt constructs the user prompt
func (g *GeminiClient) buildUserPrompt(req *AIRequest) string {
	prompt := req.Prompt

	if req.Code != "" {
		prompt += fmt.Sprintf("\n\nCode:\n```%s\n%s\n```", req.Language, req.Code)
	}

	// Add context for better understanding
	if req.Context != nil {
		if currentLine, ok := req.Context["current_line"].(int); ok {
			prompt += fmt.Sprintf("\n\nCursor is at line: %d", currentLine)
		}
		if surrounding, ok := req.Context["surrounding_code"].(string); ok {
			prompt += fmt.Sprintf("\n\nSurrounding code context:\n%s", surrounding)
		}
	}

	return prompt
}

// makeRequest sends HTTP request to Gemini API
func (g *GeminiClient) makeRequest(ctx context.Context, url string, req *geminiRequest) (*geminiResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

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
		// Parse specific error types for better error messages
		switch resp.StatusCode {
		case 429:
			return nil, fmt.Errorf("RATE_LIMIT: Gemini API rate limit exceeded. Please wait before retrying")
		case 403:
			// Check if it's a quota issue
			if bytes.Contains(body, []byte("quota")) || bytes.Contains(body, []byte("QUOTA")) {
				return nil, fmt.Errorf("QUOTA_EXCEEDED: Gemini API quota exhausted. Consider adding billing or using another provider")
			}
			return nil, fmt.Errorf("FORBIDDEN: Gemini API access denied - check API key permissions")
		case 401:
			return nil, fmt.Errorf("UNAUTHORIZED: Invalid Gemini API key")
		case 500, 502, 503, 504:
			return nil, fmt.Errorf("SERVICE_ERROR: Gemini service temporarily unavailable (status %d)", resp.StatusCode)
		default:
			return nil, fmt.Errorf("API_ERROR: Gemini request failed with status %d: %s", resp.StatusCode, string(body))
		}
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("Gemini API error: %s", geminiResp.Error.Message)
	}

	return &geminiResp, nil
}

// getModelForCapability selects the best Gemini model for the capability
func (g *GeminiClient) getModelForCapability(capability AICapability) string {
	switch capability {
	case CapabilityCodeCompletion:
		return "gemini-2.0-flash-exp"  // Fast for real-time completions
	case CapabilityExplanation:
		return "gemini-1.5-pro"        // Better for detailed explanations
	default:
		return "gemini-2.0-flash-exp"  // Default to flash model
	}
}

// GetCapabilities returns capabilities Gemini excels at
func (g *GeminiClient) GetCapabilities() []AICapability {
	return []AICapability{
		CapabilityCodeCompletion,
		CapabilityExplanation,
		CapabilityDebugging,
		CapabilityCodeGeneration,
		CapabilityDocumentation,
	}
}

// GetProvider returns the provider identifier
func (g *GeminiClient) GetProvider() AIProvider {
	return ProviderGemini
}

// Health checks if Gemini API is accessible
func (g *GeminiClient) Health(ctx context.Context) error {
	testReq := &geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: "Hello"},
				},
			},
		},
		GenerationConfig: &geminiGenConfig{
			MaxOutputTokens: 5,
		},
	}

	url := fmt.Sprintf("%s/gemini-2.0-flash:generateContent?key=%s", g.baseURL, g.apiKey)
	_, err := g.makeRequest(ctx, url, testReq)
	return err
}

// GetUsage returns current usage statistics (thread-safe copy)
func (g *GeminiClient) GetUsage() *ProviderUsage {
	g.usageMu.RLock()
	defer g.usageMu.RUnlock()

	// Return a copy to prevent data races
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

// getMaxTokens determines appropriate max tokens based on request
func (g *GeminiClient) getMaxTokens(req *AIRequest) int {
	if req.MaxTokens > 0 {
		return req.MaxTokens
	}

	switch req.Capability {
	case CapabilityCodeCompletion:
		return 200
	case CapabilityExplanation:
		return 1000
	case CapabilityCodeGeneration:
		return 2000
	default:
		return 800
	}
}

// calculateCost estimates cost based on Gemini pricing
func (g *GeminiClient) calculateCost(inputTokens, outputTokens int, model string) float64 {
	// Gemini pricing (as of 2024)
	var inputCostPer1K, outputCostPer1K float64

	switch model {
	case "gemini-1.5-pro":
		inputCostPer1K = 0.00125   // $0.00125 per 1K input tokens
		outputCostPer1K = 0.00375  // $0.00375 per 1K output tokens
	case "gemini-2.0-flash-exp":
		inputCostPer1K = 0.000075  // $0.000075 per 1K input tokens
		outputCostPer1K = 0.0003   // $0.0003 per 1K output tokens
	default:
		inputCostPer1K = 0.000075
		outputCostPer1K = 0.0003
	}

	inputCost := float64(inputTokens) / 1000.0 * inputCostPer1K
	outputCost := float64(outputTokens) / 1000.0 * outputCostPer1K

	return inputCost + outputCost
}

// updateUsage updates internal usage statistics (thread-safe)
func (g *GeminiClient) updateUsage(totalTokens int, cost float64, duration time.Duration) {
	g.usageMu.Lock()
	defer g.usageMu.Unlock()

	g.usage.RequestCount++
	g.usage.TotalTokens += int64(totalTokens)
	g.usage.TotalCost += cost
	g.usage.AvgLatency = (g.usage.AvgLatency*float64(g.usage.RequestCount-1) + duration.Seconds()) / float64(g.usage.RequestCount)
	g.usage.LastUsed = time.Now()
}

// incrementErrorCount safely increments the error count
func (g *GeminiClient) incrementErrorCount() {
	g.usageMu.Lock()
	defer g.usageMu.Unlock()
	g.usage.ErrorCount++
}