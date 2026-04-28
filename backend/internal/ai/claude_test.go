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
