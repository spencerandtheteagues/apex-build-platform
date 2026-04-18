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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestGeminiGenerateFallsBackFrom31ProTo31ProPreview(t *testing.T) {
	t.Parallel()

	seenPaths := make([]string, 0, 2)
	client := NewGeminiClient("test-key")
	client.baseURL = "https://gemini.test/v1beta/models"
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seenPaths = append(seenPaths, r.URL.Path)
		if strings.Contains(r.URL.Path, "gemini-3.1-pro:generateContent") {
			body, _ := json.Marshal(map[string]interface{}{
				"error": map[string]interface{}{
					"code":    http.StatusNotFound,
					"message": "models/gemini-3.1-pro is not found for API version v1beta",
					"status":  "NOT_FOUND",
				},
			})
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}

		if !strings.Contains(r.URL.Path, "gemini-3.1-pro-preview:generateContent") {
			t.Fatalf("unexpected Gemini model path %q", r.URL.Path)
		}

		body, _ := json.Marshal(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"role": "model",
						"parts": []map[string]string{
							{"text": "preview response"},
						},
					},
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     11,
				"candidatesTokenCount": 7,
				"totalTokenCount":      18,
			},
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}

	resp, err := client.Generate(context.Background(), &AIRequest{
		ID:          "gemini-fallback",
		Provider:    ProviderGemini,
		Model:       "gemini-3.1-pro",
		Capability:  CapabilityCodeGeneration,
		Prompt:      "Build a reliable app",
		Temperature: 0.7,
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if resp.Content != "preview response" {
		t.Fatalf("content = %q, want preview response", resp.Content)
	}
	if got := resp.Metadata["model"]; got != "gemini-3.1-pro-preview" {
		t.Fatalf("metadata model = %v, want gemini-3.1-pro-preview", got)
	}
	if len(seenPaths) != 2 {
		t.Fatalf("Gemini calls = %d, want 2 paths: %#v", len(seenPaths), seenPaths)
	}
}
