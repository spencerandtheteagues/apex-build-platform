package ai

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGrokHealthClassifiesDisabledKeyWithoutLeakingProviderBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization header = %q", got)
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"The API key xai-...secret is disabled and cannot be used to perform requests."}`))
	}))
	defer server.Close()

	client := NewGrokClient("test-key")
	client.baseURL = server.URL

	err := client.Health(context.Background())
	if err == nil {
		t.Fatal("expected disabled-key health error")
	}
	errText := err.Error()
	if !strings.Contains(errText, "key is disabled") {
		t.Fatalf("expected disabled-key message, got %q", errText)
	}
	if strings.Contains(errText, "xai-") || strings.Contains(errText, "secret") {
		t.Fatalf("error leaked provider body/key material: %q", errText)
	}
	if got := classifyProviderError(err); got != "auth_error" {
		t.Fatalf("classifyProviderError = %q, want auth_error", got)
	}
}

func TestClassifyProviderErrorTreatsMissingModelAsProviderConfigError(t *testing.T) {
	t.Parallel()

	err := errors.New(`{"error":{"message":"The requested model 'gpt-5.4-codex' does not exist.","code":"model_not_found"}}`)
	if got := classifyProviderError(err); got != "auth_error" {
		t.Fatalf("classifyProviderError = %q, want auth_error", got)
	}
}
