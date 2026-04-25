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
