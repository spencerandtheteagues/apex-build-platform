package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
	tests := []struct {
		model string
		want  string
	}{
		{model: "kimi-k2.6", want: "kimi-k2.6:cloud"},
		{model: "glm-5.1", want: "glm-5.1:cloud"},
		{model: "deepseek-v4", want: "deepseek-v4-flash:cloud"},
		{model: "deepseek-v4-pro", want: "deepseek-v4-pro:cloud"},
		{model: "qwen-3.6-27b", want: "qwen3.5:cloud"},
	}
	for _, tt := range tests {
		req := &AIRequest{
			Capability: CapabilityCodeGeneration,
			Model:      tt.model,
		}
		if got := client.getModel(req); got != tt.want {
			t.Fatalf("getModel(%q) = %q, want %q", tt.model, got, tt.want)
		}
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

// TestOllamaCloudClientCapsConcurrentInFlightRequests verifies that the
// Ollama Cloud concurrency semaphore caps in-flight Generate calls at the
// configured limit. Beyond the cap, additional callers must queue rather
// than hit the provider.
func TestOllamaCloudClientCapsConcurrentInFlightRequests(t *testing.T) {
	const maxConcurrent = 3
	t.Setenv("OLLAMA_CLOUD_MAX_CONCURRENT", "3")

	var (
		inFlight     atomic.Int32
		peakObserved atomic.Int32
		release      = make(chan struct{})
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			peak := peakObserved.Load()
			if current <= peak || peakObserved.CompareAndSwap(peak, current) {
				break
			}
		}
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","model":"kimi-k2.6:cloud","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer srv.Close()

	client := NewOllamaCloudClient(srv.URL, "fake-key")
	if client.concurrent == nil {
		t.Fatal("expected cloud client to allocate concurrency semaphore")
	}
	if cap(client.concurrent) != maxConcurrent {
		t.Fatalf("expected semaphore cap %d, got %d", maxConcurrent, cap(client.concurrent))
	}

	const total = 6
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = client.Generate(ctx, &AIRequest{ID: "test", Prompt: "hi", MaxTokens: 16})
		}()
	}

	// Give all goroutines a chance to enter the semaphore.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && inFlight.Load() < int32(maxConcurrent) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := inFlight.Load(); got != int32(maxConcurrent) {
		t.Fatalf("expected exactly %d concurrent in-flight, got %d", maxConcurrent, got)
	}

	close(release)
	wg.Wait()

	if peak := peakObserved.Load(); peak > int32(maxConcurrent) {
		t.Fatalf("expected peak in-flight to never exceed %d, got %d", maxConcurrent, peak)
	}
}

// TestOllamaCloudClientHonoursOllamaCloudMaxConcurrentZero verifies that
// setting OLLAMA_CLOUD_MAX_CONCURRENT=0 disables the gate entirely, so
// local benchmarks and tests aren't artificially throttled.
func TestOllamaCloudClientHonoursOllamaCloudMaxConcurrentZero(t *testing.T) {
	t.Setenv("OLLAMA_CLOUD_MAX_CONCURRENT", "0")
	client := NewOllamaCloudClient("https://ollama.com/v1", "fake")
	if client.concurrent != nil {
		t.Fatal("expected zero limit to disable concurrency semaphore (nil channel)")
	}
}

// TestNewOllamaClientLeavesConcurrencyUnbounded confirms that the local-only
// constructor does not allocate a semaphore. Local Ollama runs against
// localhost and has no provider-side concurrency cap.
func TestNewOllamaClientLeavesConcurrencyUnbounded(t *testing.T) {
	t.Parallel()
	client := NewOllamaClient("http://localhost:11434", "")
	if client.concurrent != nil {
		t.Fatal("expected NewOllamaClient (local) to leave concurrency semaphore nil")
	}
}
