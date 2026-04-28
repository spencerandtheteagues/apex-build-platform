package ai

import "testing"

func TestParseOllamaCredentialAcceptsCloudKeyWithBaseURL(t *testing.T) {
	t.Setenv("OLLAMA_BASE_URL", "")

	baseURL, apiKey := parseOllamaCredential("sk-test / OLLAMA_BASE_URL:https://ollama.com")
	if baseURL != "https://ollama.com" {
		t.Fatalf("baseURL = %q, want https://ollama.com", baseURL)
	}
	if apiKey != "sk-test" {
		t.Fatalf("apiKey = %q, want sk-test", apiKey)
	}
}

func TestParseOllamaCredentialKeepsURLOnlyLocalMode(t *testing.T) {
	t.Setenv("OLLAMA_BASE_URL", "https://ollama.com")

	baseURL, apiKey := parseOllamaCredential("http://localhost:11434")
	if baseURL != "http://localhost:11434" {
		t.Fatalf("baseURL = %q, want http://localhost:11434", baseURL)
	}
	if apiKey != "" {
		t.Fatalf("apiKey = %q, want empty local key", apiKey)
	}
}
