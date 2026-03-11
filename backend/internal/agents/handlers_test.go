package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"apex-build/internal/ai"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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
	build.POST("/:id/message", h.SendMessage)
	rg := r.Group("/api/v1", auth)
	rg.GET("/builds/:buildId/download", h.DownloadCompletedBuild)
	return r
}

func openBuildTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.UserAPIKey{}, &models.CompletedBuild{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
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
		{"whitespace bypass attempt", "                    "}, // 20 spaces would pass len>=10 without trim
		{"padded short", "   ab   "},
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

func TestStartBuildValidatesMalformedRequestBeforeCreditCheck(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.User{
		ID:            1,
		Username:      "creditless-user",
		Email:         "creditless@example.com",
		PasswordHash:  "hashed",
		CreditBalance: 0,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	am := &AgentManager{
		db: db,
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}

	body, _ := json.Marshal(map[string]string{"description": "   "})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 before credit check, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSendMessageRestoresSnapshotBackedBuildSession(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "snapshot-build",
		UserID:      1,
		Description: "Build a kanban app with comments and drag and drop",
		Status:      "completed",
		Mode:        "full",
		PowerMode:   "balanced",
		Progress:    100,
		FilesCount:  1,
		FilesJSON:   "[]",
		CompletedAt: nil,
	}).Error; err != nil {
		t.Fatalf("create completed build: %v", err)
	}

	ctx := context.Background()
	am := &AgentManager{
		ctx:         ctx,
		cancel:      func() {},
		db:          db,
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
			generateResult: &ai.AIResponse{
				Content: `{"reply":"I captured the change request.","apply_changes":false}`,
			},
		},
	}

	body, _ := json.Marshal(map[string]string{"content": "Add a bulk edit toolbar"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/snapshot-build/message", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["live"] != true {
		t.Fatalf("expected live=true, got %v", resp["live"])
	}
	if resp["restored_session"] != true {
		t.Fatalf("expected restored_session=true, got %v", resp["restored_session"])
	}

	restored, err := am.GetBuild("snapshot-build")
	if err != nil {
		t.Fatalf("expected restored build in manager: %v", err)
	}
	restored.mu.RLock()
	defer restored.mu.RUnlock()
	if len(restored.Agents) == 0 {
		t.Fatalf("expected restored build to have a lead agent")
	}
	if len(restored.Interaction.Messages) == 0 {
		t.Fatalf("expected restored build interaction to contain the user message")
	}
}

func TestDownloadCompletedBuildRejectsFailedSnapshot(t *testing.T) {
	db := openBuildTestDB(t)
	filesJSON, err := json.Marshal([]GeneratedFile{
		{Path: "server/package.json", Content: `{"name":"api","scripts":{"build":"node -e \"console.log('ok')\""}}`},
	})
	if err != nil {
		t.Fatalf("marshal files: %v", err)
	}
	if err := db.Create(&models.CompletedBuild{
		BuildID:    "failed-build",
		UserID:     1,
		Status:     "failed",
		FilesCount: 1,
		FilesJSON:  string(filesJSON),
	}).Error; err != nil {
		t.Fatalf("create completed build snapshot: %v", err)
	}

	am := &AgentManager{db: db}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/builds/failed-build/download", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "build is not exportable") {
		t.Fatalf("expected exportable error, got %s", w.Body.String())
	}
}

func TestDownloadCompletedBuildRejectsInvalidCompletedSnapshot(t *testing.T) {
	db := openBuildTestDB(t)
	filesJSON, err := json.Marshal([]GeneratedFile{
		{
			Path: "package.json",
			Content: `{
  "name": "demo",
  "private": true,
  "scripts": { "build": "node -e \"console.log('ok')\"" }
}`,
		},
		{
			Path: "server/package.json",
			Content: `{
  "name": "api",
  "private": true,
  "scripts": { "dev": "tsx src/index.ts" }
}`,
		},
		{Path: "server/src/index.ts", Content: "console.log('broken')"},
	})
	if err != nil {
		t.Fatalf("marshal files: %v", err)
	}
	techStackJSON, err := json.Marshal(TechStack{Backend: "Node.js"})
	if err != nil {
		t.Fatalf("marshal tech stack: %v", err)
	}
	if err := db.Create(&models.CompletedBuild{
		BuildID:    "invalid-completed-build",
		UserID:     1,
		Status:     "completed",
		TechStack:  string(techStackJSON),
		FilesCount: 2,
		FilesJSON:  string(filesJSON),
	}).Error; err != nil {
		t.Fatalf("create completed build snapshot: %v", err)
	}

	am := &AgentManager{db: db}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/builds/invalid-completed-build/download", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "failed final validation") {
		t.Fatalf("expected validation failure, got %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "missing a build script") {
		t.Fatalf("expected missing build script hint, got %s", w.Body.String())
	}
}

func TestDownloadCompletedBuildStreamsValidSnapshot(t *testing.T) {
	db := openBuildTestDB(t)
	filesJSON, err := json.Marshal([]GeneratedFile{
		{
			Path: "server/package.json",
			Content: `{
  "name": "api",
  "private": true,
  "scripts": { "build": "node -e \"console.log('ok')\"" }
}`,
		},
		{Path: "server/src/index.js", Content: "console.log('ok')"},
		{Path: "README.md", Content: "# Demo\n\nRun instructions."},
		{Path: ".env.example", Content: "PORT=3001\n"},
	})
	if err != nil {
		t.Fatalf("marshal files: %v", err)
	}
	techStackJSON, err := json.Marshal(TechStack{Backend: "Node.js"})
	if err != nil {
		t.Fatalf("marshal tech stack: %v", err)
	}
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "valid-completed-build",
		UserID:      1,
		Status:      "completed",
		ProjectName: "demo",
		TechStack:   string(techStackJSON),
		FilesCount:  2,
		FilesJSON:   string(filesJSON),
	}).Error; err != nil {
		t.Fatalf("create completed build snapshot: %v", err)
	}

	am := &AgentManager{db: db}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/builds/valid-completed-build/download", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/zip") {
		t.Fatalf("expected zip content type, got %q", got)
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
