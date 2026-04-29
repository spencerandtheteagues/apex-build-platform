package ai

import (
	"strings"
	"testing"
)

func TestOllamaBuildMessagesSeparatesSystemAndUserPrompts(t *testing.T) {
	t.Parallel()

	client := NewOllamaClient("", "")
	req := &AIRequest{
		Capability: CapabilityCodeGeneration,
		Language:   "typescript",
		Prompt:     "<task>Build a dashboard.</task>",
	}

	messages := client.buildMessages(req)
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != "system" {
		t.Fatalf("expected first message role=system, got %q", messages[0].Role)
	}
	if !strings.Contains(messages[0].Content, "You are an expert software developer for APEX.BUILD") {
		t.Fatalf("expected base system prompt in system message, got %q", messages[0].Content)
	}
	if !strings.Contains(messages[0].Content, "production-ready typescript code") {
		t.Fatalf("expected capability/language system prompt in system message, got %q", messages[0].Content)
	}
	if !strings.Contains(messages[1].Content, req.Prompt) {
		t.Fatalf("expected user prompt in user message, got %q", messages[1].Content)
	}
}

func TestOllamaCloudNormalizesKimiAlias(t *testing.T) {
	t.Parallel()

	client := NewOllamaClient("https://ollama.com", "cloud-key")
	req := &AIRequest{
		Capability: CapabilityCodeGeneration,
		Model:      "kimi-k2.6",
	}

	if got := client.getModel(req); got != "kimi-k2.6:cloud" {
		t.Fatalf("getModel() = %q, want kimi-k2.6:cloud", got)
	}
}

func TestOllamaBaseURLNormalizesV1Suffix(t *testing.T) {
	t.Parallel()

	client := NewOllamaCloudClient("https://ollama.com/v1", "cloud-key")
	if got := client.baseURL; got != "https://ollama.com" {
		t.Fatalf("baseURL = %q, want https://ollama.com", got)
	}
}

func TestOllamaCloudDisablesReasoningByDefault(t *testing.T) {
	t.Parallel()

	cloudClient := NewOllamaClient("https://ollama.com", "cloud-key")
	if got := cloudClient.reasoningEffort("kimi-k2.6"); got != "none" {
		t.Fatalf("cloud reasoningEffort() = %q, want none", got)
	}

	localClient := NewOllamaClient("http://localhost:11434", "")
	if got := localClient.reasoningEffort("deepseek-r1:14b"); got != "" {
		t.Fatalf("local reasoningEffort() = %q, want empty", got)
	}
}
