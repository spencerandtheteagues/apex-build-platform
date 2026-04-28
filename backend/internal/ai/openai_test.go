package ai

import (
	"bytes"
	"context"
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
		Model:       "gpt-5.4-pro",
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
