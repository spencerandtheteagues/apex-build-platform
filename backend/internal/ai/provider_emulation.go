package ai

import (
	"context"
	"os"
	"strings"
)

type providerAliasClient struct {
	alias AIProvider
	base  AIClient
}

func (p *providerAliasClient) Generate(ctx context.Context, req *AIRequest) (*AIResponse, error) {
	resp, err := p.base.Generate(ctx, req)
	if resp != nil {
		resp.Provider = p.alias
		if resp.Metadata == nil {
			resp.Metadata = map[string]interface{}{}
		}
		resp.Metadata["provider_slot"] = string(p.alias)
		resp.Metadata["actual_provider"] = string(ProviderOllama)
	}
	return resp, err
}

func (p *providerAliasClient) GetCapabilities() []AICapability  { return p.base.GetCapabilities() }
func (p *providerAliasClient) Health(ctx context.Context) error { return p.base.Health(ctx) }
func (p *providerAliasClient) GetProvider() AIProvider          { return p.alias }
func (p *providerAliasClient) GetUsage() *ProviderUsage {
	usage := p.base.GetUsage()
	if usage == nil {
		return nil
	}
	cloned := *usage
	cloned.Provider = p.alias
	return &cloned
}

type forceModelClient struct {
	base  AIClient
	model string
}

func (f *forceModelClient) Generate(ctx context.Context, req *AIRequest) (*AIResponse, error) {
	if req == nil || strings.TrimSpace(f.model) == "" {
		return f.base.Generate(ctx, req)
	}
	reqCopy := *req
	reqCopy.Model = strings.TrimSpace(f.model)
	resp, err := f.base.Generate(ctx, &reqCopy)
	if resp != nil {
		if resp.Metadata == nil {
			resp.Metadata = map[string]interface{}{}
		}
		if _, exists := resp.Metadata["model"]; !exists {
			resp.Metadata["model"] = reqCopy.Model
		}
	}
	return resp, err
}

func (f *forceModelClient) GetCapabilities() []AICapability  { return f.base.GetCapabilities() }
func (f *forceModelClient) GetProvider() AIProvider          { return f.base.GetProvider() }
func (f *forceModelClient) Health(ctx context.Context) error { return f.base.Health(ctx) }
func (f *forceModelClient) GetUsage() *ProviderUsage         { return f.base.GetUsage() }

func newAliasedOllamaProviderClient(alias AIProvider, baseURL, apiKey, model string) AIClient {
	var client AIClient = NewOllamaClient(baseURL, apiKey)
	model = strings.TrimSpace(model)
	if model == "" {
		model = firstEnv("OLLAMA_MODEL_DEFAULT", "OLLAMA_MODEL_BALANCED", "OLLAMA_MODEL_FAST")
	}
	if model == "" {
		model = "deepseek-r1:14b"
	}
	if model != "" {
		client = &forceModelClient{
			base:  client,
			model: model,
		}
	}
	return &providerAliasClient{
		alias: alias,
		base:  client,
	}
}

func providerOllamaEmulationConfig(provider AIProvider) (string, string) {
	switch provider {
	case ProviderClaude:
		return firstEnv("CLAUDE_OLLAMA_URL", "CLAUDE_LOCAL_OLLAMA_URL"), firstEnv("CLAUDE_OLLAMA_MODEL", "CLAUDE_LOCAL_OLLAMA_MODEL")
	case ProviderGPT4:
		return firstEnv("OPENAI_OLLAMA_URL", "GPT4_OLLAMA_URL", "OPENAI_LOCAL_OLLAMA_URL"), firstEnv("OPENAI_OLLAMA_MODEL", "GPT4_OLLAMA_MODEL", "OPENAI_LOCAL_OLLAMA_MODEL")
	case ProviderGemini:
		return firstEnv("GEMINI_OLLAMA_URL", "GEMINI_LOCAL_OLLAMA_URL"), firstEnv("GEMINI_OLLAMA_MODEL", "GEMINI_LOCAL_OLLAMA_MODEL")
	case ProviderGrok:
		return firstEnv("GROK_OLLAMA_URL", "GROK_LOCAL_OLLAMA_URL"), firstEnv("GROK_OLLAMA_MODEL", "GROK_LOCAL_OLLAMA_MODEL")
	case ProviderDeepSeek:
		return firstEnv("DEEPSEEK_OLLAMA_URL", "OLLAMA_URL"), firstEnv("DEEPSEEK_OLLAMA_MODEL", "deepseek-v3.2")
	case ProviderGLM:
		return firstEnv("GLM_OLLAMA_URL", "OLLAMA_URL"), firstEnv("GLM_OLLAMA_MODEL", "glm-5.1")
	default:
		return "", ""
	}
}

func configuredOllamaEmulations() map[AIProvider]struct {
	URL   string
	Model string
} {
	out := map[AIProvider]struct {
		URL   string
		Model string
	}{}
	for _, provider := range []AIProvider{ProviderClaude, ProviderGPT4, ProviderGemini, ProviderGrok, ProviderDeepSeek, ProviderGLM} {
		url, model := providerOllamaEmulationConfig(provider)
		if strings.TrimSpace(url) == "" {
			continue
		}
		out[provider] = struct {
			URL   string
			Model string
		}{
			URL:   strings.TrimSpace(url),
			Model: strings.TrimSpace(model),
		}
	}
	return out
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}
