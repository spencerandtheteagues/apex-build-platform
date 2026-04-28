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

func TestGeminiThinkingBudgetScalesByPowerMode(t *testing.T) {
	tests := []struct {
		name      string
		powerMode string
		want      int
	}{
		{name: "fast", powerMode: "fast", want: 256},
		{name: "balanced", powerMode: "balanced", want: 1024},
		{name: "max", powerMode: "max", want: 4096},
		{name: "unset defaults balanced", powerMode: "", want: 1024},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := geminiThinkingBudgetForModel("gemini-2.5-pro", tt.powerMode, 1200)
			if got != tt.want {
				t.Fatalf("budget = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGeminiThinkingBudgetPromptLengthAndEnvClamp(t *testing.T) {
	t.Setenv("GEMINI_THINKING_BUDGET_MAX", "7000")
	t.Setenv("GEMINI_THINKING_BUDGET_CEILING", "8192")

	if got := geminiThinkingBudgetForModel("gemini-2.5-pro", "max", 110000); got != 8192 {
		t.Fatalf("long max budget = %d, want ceiling 8192", got)
	}

	t.Setenv("GEMINI_THINKING_BUDGET_FLOOR", "512")
	if got := geminiThinkingBudgetForModel("gemini-2.5-pro", "fast", 1200); got != 512 {
		t.Fatalf("fast budget with floor = %d, want 512", got)
	}
}

func TestGeminiThinkingBudgetOnlyAppliesToThinkingModel(t *testing.T) {
	if got := geminiThinkingBudgetForModel("gemini-3-flash-preview", "max", 110000); got != 0 {
		t.Fatalf("flash budget = %d, want 0", got)
	}
}
