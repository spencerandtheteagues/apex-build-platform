package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestClaudeGenerateNormalizesLegacyOpusOverride(t *testing.T) {
	t.Parallel()

	client := NewClaudeClient("test-key")
	client.baseURL = "https://claude.test/v1/messages"
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed reading request body: %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed unmarshalling request body: %v", err)
		}
		if got := payload["model"]; got != "claude-opus-4-7" {
			t.Fatalf("Claude request model = %v, want claude-opus-4-7", got)
		}
		if _, exists := payload["temperature"]; exists {
			t.Fatalf("Claude Opus 4.7 request should omit deprecated temperature, payload=%+v", payload)
		}
		response := []byte(`{"id":"msg_test","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"output_tokens":1}}`)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(response)),
			Header:     make(http.Header),
		}, nil
	})}

	resp, err := client.Generate(context.Background(), &AIRequest{
		ID:         "legacy-claude",
		Provider:   ProviderClaude,
		Model:      "claude-opus-4-6",
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

func TestClaudeGenerateKeepsTemperatureForSonnet(t *testing.T) {
	t.Parallel()

	client := NewClaudeClient("test-key")
	client.baseURL = "https://claude.test/v1/messages"
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed reading request body: %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed unmarshalling request body: %v", err)
		}
		if got := payload["model"]; got != "claude-sonnet-4-6" {
			t.Fatalf("Claude request model = %v, want claude-sonnet-4-6", got)
		}
		if got := payload["temperature"]; got == nil {
			t.Fatalf("Claude Sonnet request should include temperature, payload=%+v", payload)
		}
		response := []byte(`{"id":"msg_test","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"output_tokens":1}}`)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(response)),
			Header:     make(http.Header),
		}, nil
	})}

	_, err := client.Generate(context.Background(), &AIRequest{
		ID:          "sonnet-temperature",
		Provider:    ProviderClaude,
		Model:       "claude-sonnet-4-6",
		Capability:  CapabilityCodeGeneration,
		Prompt:      "Build a dashboard",
		Temperature: 0.2,
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
}
