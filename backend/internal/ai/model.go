package ai

import (
	"fmt"
	"strings"
)

// GetModelUsed returns the model identifier used for a request.
// It prefers response metadata, then request model, then provider name.
func GetModelUsed(resp *AIResponse, req *AIRequest) string {
	if resp != nil && resp.Metadata != nil {
		if v, ok := resp.Metadata["model"]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	if req != nil && req.Model != "" {
		return req.Model
	}
	if resp != nil && resp.Provider != "" {
		return string(resp.Provider)
	}
	return "unknown"
}

// ActualProvider returns the underlying provider that served a response.
// Emulated provider slots may expose resp.Provider as the visible slot while
// metadata.actual_provider carries the real backend, for example Ollama Cloud.
func ActualProvider(resp *AIResponse, fallback ...AIProvider) AIProvider {
	if resp != nil && resp.Metadata != nil {
		if provider := ParseProvider(fmt.Sprintf("%v", resp.Metadata["actual_provider"])); provider != "" {
			return provider
		}
	}
	if resp != nil && resp.Provider != "" {
		fallback = append([]AIProvider{resp.Provider}, fallback...)
	}
	for _, provider := range fallback {
		if parsed := ParseProvider(string(provider)); parsed != "" {
			return parsed
		}
	}
	return ""
}

// ParseProvider normalizes a string into a supported provider identifier.
func ParseProvider(value string) AIProvider {
	provider := AIProvider(strings.TrimSpace(value))
	switch provider {
	case ProviderClaude, ProviderGPT4, ProviderGemini, ProviderGrok, ProviderOllama, ProviderDeepSeek, ProviderGLM:
		return provider
	default:
		return ""
	}
}
