package agents

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"apex-build/internal/ai"

	"github.com/gin-gonic/gin"
)

// testRouter sets up a gin router with auth middleware stub and build handler.
func testRouter(am *AgentManager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewBuildHandler(am, nil)

	// Stub auth middleware: inject user_id=1
	auth := func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Next()
	}

	build := r.Group("/api/v1/build", auth)
	build.POST("/preflight", h.PreflightCheck)
	build.POST("/start", h.StartBuild)
	return r
}

func TestPreflightReturnsReadyWithProviders(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/preflight", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["ready"] != true {
		t.Fatalf("expected ready=true, got %v", body["ready"])
	}
	providers := body["providers_available"].(float64)
	if providers < 1 {
		t.Fatalf("expected at least 1 provider, got %v", providers)
	}
}

func TestPreflightReturnsNoRouterWhenNil(t *testing.T) {
	am := &AgentManager{aiRouter: nil}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/preflight", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["error_code"] != "NO_ROUTER" {
		t.Fatalf("expected error_code=NO_ROUTER, got %v", body["error_code"])
	}
}

func TestPreflightReturnsNoProviderWhenNoneConfigured(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubPreflight{configured: false},
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/preflight", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["error_code"] != "NO_PROVIDER" {
		t.Fatalf("expected error_code=NO_PROVIDER, got %v", body["error_code"])
	}
}

func TestPreflightReturnsInsufficientCreditsWhenUserHasNoProviders(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{}, // user has no providers
		},
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/preflight", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["error_code"] != "INSUFFICIENT_CREDITS" {
		t.Fatalf("expected error_code=INSUFFICIENT_CREDITS, got %v", body["error_code"])
	}
}

func TestPreflightReturnsAllProvidersDownWhenNoneAvailable(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{}, // all providers down
			userProviders: []ai.AIProvider{},
		},
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/preflight", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["error_code"] != "ALL_PROVIDERS_DOWN" {
		t.Fatalf("expected error_code=ALL_PROVIDERS_DOWN, got %v", body["error_code"])
	}
}

func TestStartBuildRejectsShortDescription(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}

	tests := []struct {
		name        string
		description string
	}{
		{"empty", ""},
		{"too short", "hi"},
		{"whitespace only", "         "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"description": tc.description})
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/build/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			testRouter(am).ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestStartBuildRejectsWhenNoProviders(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{}, // no providers for user
		},
	}

	body, _ := json.Marshal(map[string]string{
		"description": "Build me a todo app with React and Express backend",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d: %s", w.Code, w.Body.String())
	}

	var respBody map[string]any
	json.Unmarshal(w.Body.Bytes(), &respBody)
	if respBody["error_code"] != "INSUFFICIENT_CREDITS" {
		t.Fatalf("expected error_code=INSUFFICIENT_CREDITS, got %v", respBody["error_code"])
	}
}

func TestStartBuildRejectsWhenNoProviderConfigured(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubPreflight{
			configured: false,
		},
	}

	body, _ := json.Marshal(map[string]string{
		"description": "Build me a todo app with React and Express backend",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}

	var respBody map[string]any
	json.Unmarshal(w.Body.Bytes(), &respBody)
	if respBody["error_code"] != "NO_PROVIDER" {
		t.Fatalf("expected error_code=NO_PROVIDER, got %v", respBody["error_code"])
	}
}
