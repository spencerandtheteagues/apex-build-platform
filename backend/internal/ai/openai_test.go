package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestOpenAIGenerateDoesNotSilentlyDowngradeExplicitGPT5Model(t *testing.T) {
	t.Parallel()

	requestCount := 0
	client := NewOpenAIClient("test-key")
	client.baseURL = "https://openai.test/v1/chat/completions"
	client.responsesURL = "https://openai.test/v1/responses"
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requestCount++
		if strings.Contains(r.URL.Path, "/chat/completions") {
			t.Fatalf("unexpected chat completions fallback for explicit GPT-5 model")
		}
		body := []byte(`{"error":{"message":"responses temporarily unavailable"}}`)
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}

	_, err := client.Generate(context.Background(), &AIRequest{
		ID:          "explicit-gpt5",
		Provider:    ProviderGPT4,
		Model:       "gpt-5.4",
		Capability:  CapabilityCodeGeneration,
		Prompt:      "Build a product page",
		Temperature: 0.3,
	})
	if err == nil {
		t.Fatal("expected Generate to return an error")
	}
	if !strings.Contains(err.Error(), "SERVICE_ERROR") {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
}

func TestOpenAIGenerateNormalizesUnavailableCodexOverride(t *testing.T) {
	t.Parallel()

	client := NewOpenAIClient("test-key")
	client.baseURL = "https://openai.test/v1/chat/completions"
	client.responsesURL = "https://openai.test/v1/responses"
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.URL.Path, "/responses") {
			t.Fatalf("expected responses endpoint for GPT-5 tier, got %s", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed reading request body: %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed unmarshalling request body: %v", err)
		}
		if got := payload["model"]; got != "gpt-5.4" {
			t.Fatalf("OpenAI request model = %v, want gpt-5.4", got)
		}
		response := []byte(`{"id":"resp_test","model":"gpt-5.4","output_text":"ok","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(response)),
			Header:     make(http.Header),
		}, nil
	})}

	resp, err := client.Generate(context.Background(), &AIRequest{
		ID:         "legacy-openai",
		Provider:   ProviderGPT4,
		Model:      "gpt-codex-5.5",
		Capability: CapabilityCodeGeneration,
		Prompt:     "Build a dashboard",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("response content = %q, want ok", resp.Content)
	}
}
