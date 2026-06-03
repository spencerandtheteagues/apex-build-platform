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
	"time"
)

const (
	// openRouterBaseURL is the OpenRouter API endpoint (OpenAI-compatible).
	openRouterBaseURL = "https://openrouter.ai/api/v1"

	// dispatcherModel is the primary LLM used to make routing decisions.
	// ChatGPT 5.5 — strong reasoning, good at structured JSON output.
	dispatcherModel = "openai/gpt-5.5"

	// dispatcherModelFallback is used when the primary dispatcher fails.
	// DeepSeek V4 Pro via OpenRouter (cheaper, still excellent at routing decisions).
	dispatcherModelFallback = "deepseek/deepseek-v4-pro"

	// dispatcherTimeout is the hard deadline for a routing decision.
	// If the dispatcher exceeds this, we fall back to catalog selection instantly.
	dispatcherTimeout = 3 * time.Second
)

// DispatchResult is the structured response from the LLM dispatcher.
type DispatchResult struct {
	ModelID string `json:"model"`
	Tier    string `json:"tier"`
	Reason  string `json:"reason"`
}

// AutoSelector picks the best OpenRouter model for a given request.
// It tries three strategies in order:
//  1. LLM dispatcher (gpt-5.5 → deepseek-v4-pro fallback) for smart context-aware selection
//  2. Ollama cloud DeepSeek fallback if OpenRouter dispatcher is unavailable
//  3. Algorithmic catalog scoring (instant, zero cost, always works)
type AutoSelector struct {
	apiKey     string
	httpClient *http.Client
	catalog    map[string]CatalogModel
	ollamaFB   *OllamaClient // Ollama cloud DeepSeek fallback for dispatcher
}

// NewAutoSelector creates an AutoSelector.
// ollamaFallback may be nil — catalog selection is always available as last resort.
func NewAutoSelector(apiKey string, ollamaFallback *OllamaClient) *AutoSelector {
	return &AutoSelector{
		apiKey:     apiKey,
		httpClient: &http.Client{},
		catalog:    CatalogByID(),
		ollamaFB:   ollamaFallback,
	}
}

// Select returns the best OpenRouter model ID for the given request.
// Never returns an empty string — always falls back to catalog selection.
func (s *AutoSelector) Select(ctx context.Context, req *AIRequest, freeOnly bool) string {
	// For free-only requests (free tier users), skip the paid dispatcher entirely
	// and go straight to catalog-based free model selection.
	if freeOnly {
		return s.catalogSelect(req, true)
	}

	// Try LLM dispatcher with a tight deadline.
	dispCtx, cancel := context.WithTimeout(ctx, dispatcherTimeout)
	defer cancel()

	model, err := s.dispatcherSelect(dispCtx, req, dispatcherModel)
	if err == nil && model != "" {
		log.Printf("[openrouter] dispatcher selected: %s", model)
		return model
	}
	log.Printf("[openrouter] primary dispatcher failed (%v), trying fallback", err)

	// Fallback dispatcher: deepseek-v4-pro via OpenRouter.
	model, err = s.dispatcherSelect(dispCtx, req, dispatcherModelFallback)
	if err == nil && model != "" {
		log.Printf("[openrouter] fallback dispatcher selected: %s", model)
		return model
	}
	log.Printf("[openrouter] fallback dispatcher failed (%v), trying Ollama DeepSeek", err)

	// Ollama cloud fallback: use DeepSeek via subscription Ollama API.
	// This only kicks in if the OpenRouter API itself is unreachable.
	if s.ollamaFB != nil {
		model = s.ollamaDispatcherSelect(ctx, req)
		if model != "" {
			log.Printf("[openrouter] ollama dispatcher selected: %s", model)
			return model
		}
	}

	// Final fallback: pure algorithmic selection from catalog.
	model = s.catalogSelect(req, false)
	log.Printf("[openrouter] catalog selected: %s", model)
	return model
}

// dispatcherSelect calls an LLM (via OpenRouter) to pick the best model.
func (s *AutoSelector) dispatcherSelect(ctx context.Context, req *AIRequest, routerModel string) (string, error) {
	systemPrompt := `You are a model router for Apex Build, an AI-powered app builder.
Your only job: given a task description, pick the best OpenRouter model for it.
Rules:
- Prioritize quality first, then minimize cost.
- Never pick any anthropic/* model.
- Return ONLY valid JSON: {"model": "<openrouter-model-id>", "tier": "<elite|pro|balanced|fast|free>", "reason": "<10 words max>"}`

	userPrompt := buildDispatcherPrompt(req)

	payload := map[string]interface{}{
		"model": routerModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":  120,
		"temperature": 0.1,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", openRouterBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", "https://apex-build.dev")
	httpReq.Header.Set("X-Title", "Apex Build")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("dispatcher HTTP %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}

	var orResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &orResp); err != nil {
		return "", err
	}
	if len(orResp.Choices) == 0 {
		return "", fmt.Errorf("dispatcher returned no choices")
	}

	content := strings.TrimSpace(orResp.Choices[0].Message.Content)
	var result DispatchResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		// Try to extract JSON from the content if wrapped in markdown
		if start := strings.Index(content, "{"); start >= 0 {
			if end := strings.LastIndex(content, "}"); end > start {
				if err2 := json.Unmarshal([]byte(content[start:end+1]), &result); err2 != nil {
					return "", fmt.Errorf("dispatcher response not valid JSON: %s", content)
				}
			}
		} else {
			return "", fmt.Errorf("dispatcher response not valid JSON: %s", content)
		}
	}

	model := strings.TrimSpace(result.ModelID)
	if model == "" || isAnthropicModel(model) {
		return "", fmt.Errorf("dispatcher returned invalid model: %q", model)
	}
	return model, nil
}

// ollamaDispatcherSelect uses the Ollama cloud client with DeepSeek to make a routing decision.
// This is the final LLM-backed fallback before pure catalog selection.
func (s *AutoSelector) ollamaDispatcherSelect(ctx context.Context, req *AIRequest) string {
	if s.ollamaFB == nil {
		return ""
	}

	dispatchReq := &AIRequest{
		Provider:    ProviderOllama,
		Model:       "deepseek-v4-pro", // Ollama cloud DeepSeek model name
		Capability:  CapabilityArchitecture,
		PowerMode:   "fast",
		MaxTokens:   150,
		Temperature: 0.1,
		Prompt: `You are a model router. Given the task below, output ONLY valid JSON: {"model": "<openrouter-model-id>", "tier": "...", "reason": "..."}
Never pick anthropic/* models. Prioritize quality then cost.

Task: ` + buildDispatcherPrompt(req),
	}

	dispCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	resp, err := s.ollamaFB.Generate(dispCtx, dispatchReq)
	if err != nil || resp == nil {
		return ""
	}

	content := strings.TrimSpace(resp.Content)
	var result DispatchResult
	if start := strings.Index(content, "{"); start >= 0 {
		if end := strings.LastIndex(content, "}"); end > start {
			if json.Unmarshal([]byte(content[start:end+1]), &result) == nil {
				model := strings.TrimSpace(result.ModelID)
				if model != "" && !isAnthropicModel(model) {
					return model
				}
			}
		}
	}
	return ""
}

// catalogSelect picks the best model from the curated catalog algorithmically.
// When freeOnly=true, only free models are considered.
func (s *AutoSelector) catalogSelect(req *AIRequest, freeOnly bool) string {
	var best CatalogModel
	bestScore := -1.0

	for _, m := range OpenRouterCatalog() {
		if freeOnly && !m.IsFree {
			continue
		}
		if isAnthropicModel(m.ID) {
			continue
		}
		score := catalogScore(m, req.Capability)
		if score > bestScore {
			bestScore = score
			best = m
		}
	}

	if best.ID != "" {
		return best.ID
	}
	// Absolute last resort
	if freeOnly {
		return "meta-llama/llama-3.3-70b-instruct:free"
	}
	return "openai/gpt-5.5"
}

// buildDispatcherPrompt constructs the user message for the dispatcher LLM.
func buildDispatcherPrompt(req *AIRequest) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Capability: %s\n", req.Capability))
	sb.WriteString(fmt.Sprintf("Power mode: %s\n", req.PowerMode))
	if req.Language != "" {
		sb.WriteString(fmt.Sprintf("Language: %s\n", req.Language))
	}
	// Truncate prompt to avoid burning tokens on the dispatcher
	prompt := req.Prompt
	if len(prompt) > 400 {
		prompt = prompt[:400] + "..."
	}
	sb.WriteString(fmt.Sprintf("Task preview: %s\n", prompt))
	sb.WriteString("\nPick the single best OpenRouter model. Return JSON only.")
	return sb.String()
}
