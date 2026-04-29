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

func TestOllamaCloudKeepsReasoningForPlanningAndPadsTokenBudget(t *testing.T) {
	t.Parallel()

	cloudClient := NewOllamaClient("https://ollama.com", "cloud-key")
	balancedPlan := &AIRequest{
		Capability: CapabilityArchitecture,
		Model:      "kimi-k2.6",
		MaxTokens:  4000,
		PowerMode:  "balanced",
	}
	effort := cloudClient.reasoningEffort(balancedPlan, "kimi-k2.6:cloud")
	if effort != "medium" {
		t.Fatalf("balanced planning reasoningEffort() = %q, want medium", effort)
	}
	if got := cloudClient.thinkEnabled(balancedPlan, effort); got == nil || !*got {
		t.Fatalf("balanced planning thinkEnabled() = %v, want true pointer", got)
	}
	budget := cloudClient.reasoningTokenBudget(balancedPlan, effort)
	if budget != 4096 {
		t.Fatalf("balanced planning reasoning budget = %d, want 4096", budget)
	}
	if got := cloudClient.getMaxTokens(balancedPlan, budget); got != 8096 {
		t.Fatalf("balanced planning max_tokens = %d, want visible+reasoning budget 8096", got)
	}

	maxPlan := &AIRequest{
		Capability: CapabilityArchitecture,
		Model:      "kimi-k2.6",
		MaxTokens:  8000,
		PowerMode:  "max",
	}
	effort = cloudClient.reasoningEffort(maxPlan, "kimi-k2.6:cloud")
	if effort != "high" {
		t.Fatalf("max planning reasoningEffort() = %q, want high", effort)
	}
	budget = cloudClient.reasoningTokenBudget(maxPlan, effort)
	if budget != 8192 {
		t.Fatalf("max planning reasoning budget = %d, want 8192", budget)
	}

	fastCompletion := &AIRequest{
		Capability: CapabilityCodeCompletion,
		MaxTokens:  500,
		PowerMode:  "fast",
	}
	effort = cloudClient.reasoningEffort(fastCompletion, "kimi-k2.6:cloud")
	if effort != "none" {
		t.Fatalf("fast completion reasoningEffort() = %q, want none", effort)
	}
	if got := cloudClient.thinkEnabled(fastCompletion, effort); got == nil || *got {
		t.Fatalf("fast completion thinkEnabled() = %v, want false pointer", got)
	}

	localClient := NewOllamaClient("http://localhost:11434", "")
	localReq := &AIRequest{Capability: CapabilityArchitecture, PowerMode: "max"}
	if got := localClient.reasoningEffort(localReq, "deepseek-r1:14b"); got != "" {
		t.Fatalf("local reasoningEffort() = %q, want empty", got)
	}
	if got := localClient.thinkEnabled(localReq, ""); got != nil {
		t.Fatalf("local thinkEnabled() = %v, want nil", got)
	}
}

func TestOllamaExtractsSeparateReasoningOutput(t *testing.T) {
	t.Parallel()

	var resp ollamaResponse
	resp.Choices = append(resp.Choices, struct {
		Index   int `json:"index"`
		Message struct {
			Role             string `json:"role"`
			Content          string `json:"content"`
			Reasoning        string `json:"reasoning,omitempty"`
			Thinking         string `json:"thinking,omitempty"`
			ReasoningContent string `json:"reasoning_content,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	}{FinishReason: "length"})
	resp.Choices[0].Message.Reasoning = "planning the implementation"

	content, reasoning, finishReason := extractOllamaChoice(&resp)
	if content != "" {
		t.Fatalf("content = %q, want empty", content)
	}
	if reasoning != "planning the implementation" {
		t.Fatalf("reasoning = %q, want separate reasoning content", reasoning)
	}
	if finishReason != "length" {
		t.Fatalf("finishReason = %q, want length", finishReason)
	}
}

func TestOllamaReasoningBudgetErrorIsClassifiedAsTruncation(t *testing.T) {
	t.Parallel()

	err := ollamaReasoningBudgetError("length", 128, 64)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"OLLAMA_REASONING_BUDGET_EXHAUSTED", "truncated", "completion_tokens=64"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q missing %q", msg, want)
		}
	}
}
