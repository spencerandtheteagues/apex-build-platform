package agents

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	build.POST("/:id/pause", h.PauseBuild)
	build.POST("/:id/resume", h.ResumeBuild)
	build.POST("/:id/cancel", h.CancelBuild)
	build.GET("/:id/checkpoints", h.GetCheckpoints)
	build.GET("/:id/agents", h.GetAgents)
	build.GET("/:id/tasks", h.GetTasks)
	build.GET("/:id/proposed-edits", h.GetProposedEdits)
	build.POST("/:id/approve-edits", h.ApproveEdits)
	build.POST("/:id/reject-edits", h.RejectEdits)
	build.POST("/:id/approve-all", h.ApproveAllEdits)
	build.POST("/:id/reject-all", h.RejectAllEdits)
	build.GET("/:id", h.GetBuildDetails)
	build.GET("/:id/status", h.GetBuildStatus)
	rg := r.Group("/api/v1", auth)
	rg.GET("/builds", h.ListBuilds)
	rg.GET("/builds/:buildId", h.GetCompletedBuild)
	rg.GET("/builds/:buildId/download", h.DownloadCompletedBuild)
	rg.DELETE("/builds/:buildId", h.DeleteBuild)
	return r
}

func openBuildTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.UserAPIKey{}, &models.CompletedBuild{}, &proposedEditRow{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

func TestBuildPlatformIssueFromErrorClassifiesRedisAllowlistAsConfiguration(t *testing.T) {
	issue := buildPlatformIssueFromError(errors.New("redis ping failed: AUTH failed: Client IP address is not in the allowlist."))
	if issue == nil {
		t.Fatal("expected platform issue classification")
	}
	if issue.Service != "redis_cache" {
		t.Fatalf("expected redis_cache service, got %s", issue.Service)
	}
	if issue.IssueType != "platform_configuration" {
		t.Fatalf("expected platform_configuration, got %s", issue.IssueType)
	}
	if issue.Retryable {
		t.Fatal("expected allowlist misconfiguration to be non-retryable until fixed")
	}
	if !strings.Contains(strings.ToLower(issue.Summary), "internal render key value connection string") {
		t.Fatalf("expected actionable remediation summary, got %q", issue.Summary)
	}
}

func TestGetBuildSessionForUserRestoresSnapshotWhenLiveBuildIsGone(t *testing.T) {
	db := openBuildTestDB(t)
	snapshot := &models.CompletedBuild{
		BuildID:     "snapshot-session-restore",
		UserID:      1,
		Status:      string(BuildFailed),
		Description: "Restore a failed build session",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
		Error:       "contract blocked",
	}
	if err := db.Create(snapshot).Error; err != nil {
		t.Fatalf("create snapshot: %v", err)
	}

	am := &AgentManager{
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		ctx:         context.Background(),
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
		db: db,
	}

	build, restored, err := am.getBuildSessionForUser(snapshot.BuildID, 1, false)
	if err != nil {
		t.Fatalf("getBuildSessionForUser returned error: %v", err)
	}
	if !restored {
		t.Fatalf("expected snapshot-backed session to be restored")
	}
	if build == nil || build.ID != snapshot.BuildID {
		t.Fatalf("expected restored build %s, got %+v", snapshot.BuildID, build)
	}
}

func TestGetBuildSnapshotFallsBackToBuildIDLookupAndEnforcesOwnership(t *testing.T) {
	db := openBuildTestDB(t)
	snapshot := &models.CompletedBuild{
		BuildID:     "snapshot-fallback-ownership",
		UserID:      7,
		Status:      string(BuildPlanning),
		Description: "Snapshot ownership fallback",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := db.Create(snapshot).Error; err != nil {
		t.Fatalf("create snapshot: %v", err)
	}

	handler := NewBuildHandler(&AgentManager{db: db}, nil)

	found, err := handler.getBuildSnapshot(7, snapshot.BuildID)
	if err != nil {
		t.Fatalf("expected owner lookup to succeed: %v", err)
	}
	if found == nil || found.BuildID != snapshot.BuildID {
		t.Fatalf("expected snapshot %s, got %+v", snapshot.BuildID, found)
	}

	if _, err := handler.getBuildSnapshot(8, snapshot.BuildID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected foreign-user lookup to return not found, got %v", err)
	}
}

func TestGetBuildSnapshotFallsBackToUnscopedLookup(t *testing.T) {
	db := openBuildTestDB(t)
	snapshot := &models.CompletedBuild{
		BuildID:     "snapshot-unscoped-fallback",
		UserID:      7,
		Status:      string(BuildPlanning),
		Description: "Snapshot unscoped ownership fallback",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := db.Create(snapshot).Error; err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if err := db.Delete(snapshot).Error; err != nil {
		t.Fatalf("soft delete snapshot: %v", err)
	}

	handler := NewBuildHandler(&AgentManager{db: db}, nil)

	found, err := handler.getBuildSnapshot(7, snapshot.BuildID)
	if err != nil {
		t.Fatalf("expected unscoped fallback lookup to succeed: %v", err)
	}
	if found == nil || found.BuildID != snapshot.BuildID {
		t.Fatalf("expected snapshot %s, got %+v", snapshot.BuildID, found)
	}
}

func TestGetBuildStatusServesActiveSnapshotReadOnlyWithoutRestoringSession(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "active-status-restore",
		UserID:      1,
		Description: "Continue building a static marketing site",
		Status:      string(BuildReviewing),
		Mode:        string(ModeFast),
		PowerMode:   string(PowerFast),
		Progress:    92,
		FilesJSON:   "[]",
		AgentsJSON: `[{
			"id":"lead-1",
			"role":"lead",
			"provider":"claude",
			"status":"working",
			"progress":80
		}]`,
		TasksJSON: `[{
			"id":"task-review",
			"type":"review",
			"description":"Review the generated frontend",
			"assigned_to":"lead-1",
			"status":"in_progress"
		}]`,
		StateJSON: `{
			"current_phase":"review",
			"quality_gate_required":true
		}`,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("create active snapshot: %v", err)
	}

	am := &AgentManager{
		db:          db,
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		ctx:         context.Background(),
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
		taskQueue:   make(chan *Task, 8),
		resultQueue: make(chan *TaskResult, 8),
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/build/active-status-restore/status", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["live"] != false {
		t.Fatalf("expected active snapshot read to stay non-live, got %v", body["live"])
	}
	if body["restored_from_snapshot"] != true {
		t.Fatalf("expected restored_from_snapshot=true, got %v", body["restored_from_snapshot"])
	}
	if body["status"] != string(BuildReviewing) {
		t.Fatalf("expected reviewing status, got %v", body["status"])
	}
	if _, exists := am.builds["active-status-restore"]; exists {
		t.Fatal("expected read-only status request not to restore build session into memory")
	}
}

func TestGetBuildStatusFallsBackToSnapshotWhenLiveLookupTimesOut(t *testing.T) {
	db := openBuildTestDB(t)
	now := time.Now().UTC()
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "active-status-timeout-fallback",
		UserID:      1,
		Description: "Serve snapshot when live lookup is blocked",
		Status:      string(BuildInProgress),
		Mode:        string(ModeFull),
		PowerMode:   string(PowerBalanced),
		Progress:    59,
		FilesJSON:   "[]",
		AgentsJSON: `[{
			"id":"backend-1",
			"role":"backend",
			"provider":"gpt4",
			"status":"working",
			"progress":59
		}]`,
		TasksJSON: `[{
			"id":"task-api",
			"type":"generate_api",
			"description":"Implement backend routes",
			"assigned_to":"backend-1",
			"status":"in_progress"
		}]`,
		StateJSON: `{
			"current_phase":"backend_and_data",
			"quality_gate_required":true
		}`,
		CreatedAt: now.Add(-2 * time.Minute),
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create active snapshot: %v", err)
	}

	am := &AgentManager{
		db:          db,
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		ctx:         context.Background(),
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}
	am.builds["active-status-timeout-fallback"] = &Build{
		ID:          "active-status-timeout-fallback",
		UserID:      1,
		Description: "Live build that is temporarily blocked",
		Status:      BuildInProgress,
		Mode:        ModeFull,
		PowerMode:   PowerBalanced,
		Progress:    59,
	}

	previousTimeout := readableBuildLookupTimeout
	readableBuildLookupTimeout = 50 * time.Millisecond
	defer func() { readableBuildLookupTimeout = previousTimeout }()

	am.mu.Lock()
	defer am.mu.Unlock()

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/build/active-status-timeout-fallback/status", nil)
		testRouter(am).ServeHTTP(w, req)
		done <- w
	}()

	select {
	case w := <-done:
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if body["live"] != false {
			t.Fatalf("expected timed-out live lookup to fall back to snapshot, got live=%v", body["live"])
		}
		if body["restored_from_snapshot"] != true {
			t.Fatalf("expected snapshot fallback to mark restored_from_snapshot=true, got %v", body["restored_from_snapshot"])
		}
		if body["progress"] != float64(59) {
			t.Fatalf("expected snapshot progress 59, got %v", body["progress"])
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected build status fallback to return promptly when live lookup blocks")
	}
}

func TestGetBuildStatusFallsBackToSnapshotWhenLiveReadStalls(t *testing.T) {
	db := openBuildTestDB(t)
	now := time.Now().UTC()
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "active-status-read-stall",
		UserID:      1,
		Description: "Serve snapshot when live status read blocks",
		Status:      string(BuildInProgress),
		Mode:        string(ModeFull),
		PowerMode:   string(PowerBalanced),
		Progress:    99,
		FilesJSON:   "[]",
		AgentsJSON:  `[{"id":"backend-1","role":"backend","provider":"gpt4","status":"working","progress":99}]`,
		TasksJSON:   `[{"id":"task-api","type":"generate_api","description":"Implement backend routes","assigned_to":"backend-1","status":"in_progress"}]`,
		StateJSON:   `{"current_phase":"backend_services","quality_gate_required":true}`,
		CreatedAt:   now.Add(-4 * time.Minute),
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("create active snapshot: %v", err)
	}

	am := &AgentManager{
		db:          db,
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		ctx:         context.Background(),
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}
	build := &Build{
		ID:          "active-status-read-stall",
		UserID:      1,
		Description: "Live build with blocked status read",
		Status:      BuildInProgress,
		Mode:        ModeFull,
		PowerMode:   PowerBalanced,
		Progress:    99,
	}
	am.builds[build.ID] = build

	previousTimeout := readableBuildStateTimeout
	readableBuildStateTimeout = 50 * time.Millisecond
	defer func() { readableBuildStateTimeout = previousTimeout }()

	build.mu.Lock()
	defer build.mu.Unlock()

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/build/active-status-read-stall/status", nil)
		testRouter(am).ServeHTTP(w, req)
		done <- w
	}()

	select {
	case w := <-done:
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if body["live"] != false {
			t.Fatalf("expected stalled live read to fall back to snapshot, got live=%v", body["live"])
		}
		if body["progress"] != float64(99) {
			t.Fatalf("expected snapshot progress 99, got %v", body["progress"])
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected build status fallback to return promptly when live state read blocks")
	}
}

func TestGetBuildDetailsServesActiveSnapshotReadOnlyWithoutRestoringSession(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "active-details-restore",
		UserID:      1,
		Description: "Continue building a full-stack dashboard",
		Status:      string(BuildInProgress),
		Mode:        string(ModeFull),
		PowerMode:   string(PowerBalanced),
		Progress:    44,
		FilesJSON:   "[]",
		AgentsJSON: `[{
			"id":"frontend-1",
			"role":"frontend",
			"provider":"gpt4",
			"status":"working",
			"progress":55
		}]`,
		TasksJSON: `[{
			"id":"task-ui",
			"type":"generate_ui",
			"description":"Build the dashboard shell",
			"assigned_to":"frontend-1",
			"status":"in_progress"
		}]`,
		StateJSON: `{
			"current_phase":"frontend_ui",
			"quality_gate_required":true
		}`,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("create active snapshot: %v", err)
	}

	am := &AgentManager{
		db:          db,
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		ctx:         context.Background(),
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
		taskQueue:   make(chan *Task, 8),
		resultQueue: make(chan *TaskResult, 8),
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/build/active-details-restore", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["live"] != false {
		t.Fatalf("expected details endpoint to serve snapshot as non-live, got %v", body["live"])
	}
	if body["restored_from_snapshot"] != true {
		t.Fatalf("expected restored_from_snapshot=true, got %v", body["restored_from_snapshot"])
	}
	if body["status"] != string(BuildInProgress) {
		t.Fatalf("expected in_progress status, got %v", body["status"])
	}
	if _, exists := am.builds["active-details-restore"]; exists {
		t.Fatal("expected read-only details request not to restore build session into memory")
	}
}

func TestGetBuildDetailsFallsBackToSnapshotWhenLiveReadStalls(t *testing.T) {
	db := openBuildTestDB(t)
	now := time.Now().UTC()
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "active-details-read-stall",
		UserID:      1,
		Description: "Serve snapshot when live details read blocks",
		Status:      string(BuildInProgress),
		Mode:        string(ModeFull),
		PowerMode:   string(PowerBalanced),
		Progress:    99,
		FilesJSON:   `[{"path":"src/App.tsx","content":"export default function App(){return null}","language":"typescript"}]`,
		AgentsJSON:  `[{"id":"backend-1","role":"backend","provider":"gpt4","status":"working","progress":99}]`,
		TasksJSON:   `[{"id":"task-api","type":"generate_api","description":"Implement backend routes","assigned_to":"backend-1","status":"in_progress"}]`,
		StateJSON:   `{"current_phase":"backend_services","quality_gate_required":true}`,
		CreatedAt:   now.Add(-4 * time.Minute),
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("create active snapshot: %v", err)
	}

	am := &AgentManager{
		db:          db,
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		ctx:         context.Background(),
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}
	build := &Build{
		ID:          "active-details-read-stall",
		UserID:      1,
		Description: "Live build with blocked details read",
		Status:      BuildInProgress,
		Mode:        ModeFull,
		PowerMode:   PowerBalanced,
		Progress:    99,
	}
	am.builds[build.ID] = build

	previousTimeout := readableBuildStateTimeout
	readableBuildStateTimeout = 50 * time.Millisecond
	defer func() { readableBuildStateTimeout = previousTimeout }()

	build.mu.Lock()
	defer build.mu.Unlock()

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/build/active-details-read-stall", nil)
		testRouter(am).ServeHTTP(w, req)
		done <- w
	}()

	select {
	case w := <-done:
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if body["live"] != false {
			t.Fatalf("expected stalled live details read to fall back to snapshot, got live=%v", body["live"])
		}
		files, ok := body["files"].([]any)
		if !ok || len(files) != 1 {
			t.Fatalf("expected snapshot files in fallback response, got %T %#v", body["files"], body["files"])
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected build details fallback to return promptly when live state read blocks")
	}
}

func TestGetBuildStatusKeepsFreshLeasedActiveSnapshotReadOnly(t *testing.T) {
	db := openBuildTestDB(t)
	now := time.Now().UTC()
	stateJSON := fmt.Sprintf(`{
		"current_phase":"testing",
		"restore_context":{
			"active_owner_instance_id":"owner-instance",
			"active_owner_heartbeat_at":%q
		}
	}`, now.Format(time.RFC3339Nano))
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "active-status-fresh-lease",
		UserID:      1,
		Description: "Fresh owner lease should stay read only",
		Status:      string(BuildTesting),
		Mode:        string(ModeFull),
		PowerMode:   string(PowerBalanced),
		Progress:    79,
		FilesJSON:   "[]",
		AgentsJSON: `[{
			"id":"testing-1",
			"role":"testing",
			"provider":"gpt4",
			"status":"working",
			"progress":79
		}]`,
		TasksJSON: `[{
			"id":"task-test",
			"type":"test",
			"description":"Run integration proof",
			"assigned_to":"testing-1",
			"status":"in_progress"
		}]`,
		StateJSON: stateJSON,
		CreatedAt: now.Add(-3 * time.Minute),
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create active snapshot: %v", err)
	}

	am := &AgentManager{
		db:          db,
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		resultQueue: make(chan *TaskResult, 1),
		ctx:         context.Background(),
		instanceID:  "reader-instance",
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/build/active-status-fresh-lease/status", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["live"] != false {
		t.Fatalf("expected fresh leased snapshot read to stay non-live, got %v", body["live"])
	}
	if _, exists := am.builds["active-status-fresh-lease"]; exists {
		t.Fatal("expected fresh leased snapshot not to materialize a live build")
	}
}

func TestGetBuildStatusRestoresStaleLeasedActiveSnapshot(t *testing.T) {
	db := openBuildTestDB(t)
	staleHeartbeat := time.Now().UTC().Add(-2 * activeBuildLeaseStaleAfter())
	stateJSON := fmt.Sprintf(`{
		"current_phase":"testing",
		"restore_context":{
			"subscription_plan":"team",
			"provider_mode":"platform",
			"active_owner_instance_id":"owner-instance",
			"active_owner_heartbeat_at":%q
		}
	}`, staleHeartbeat.Format(time.RFC3339Nano))
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "active-status-stale-lease",
		UserID:      1,
		Description: "Stale owner lease should be resumable",
		Status:      string(BuildTesting),
		Mode:        string(ModeFull),
		PowerMode:   string(PowerBalanced),
		Progress:    79,
		FilesJSON:   "[]",
		AgentsJSON: `[{
			"id":"lead-1",
			"role":"lead",
			"provider":"claude",
			"status":"working",
			"build_id":"active-status-stale-lease",
			"progress":79
		}]`,
		TasksJSON: `[{
			"id":"task-test",
			"type":"test",
			"description":"Run integration proof",
			"assigned_to":"lead-1",
			"status":"in_progress",
			"created_at":"2026-03-29T01:00:00Z",
			"started_at":"2026-03-29T01:00:00Z"
		}]`,
		StateJSON: stateJSON,
		CreatedAt: time.Now().UTC().Add(-10 * time.Minute),
		UpdatedAt: time.Now().UTC().Add(-10 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("create active snapshot: %v", err)
	}

	am := &AgentManager{
		db:          db,
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		resultQueue: make(chan *TaskResult, 1),
		ctx:         context.Background(),
		instanceID:  "reader-instance",
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/build/active-status-stale-lease/status", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["live"] != true {
		t.Fatalf("expected stale leased snapshot to restore as live, got %v", body["live"])
	}
	if body["restored_from_snapshot"] != true {
		t.Fatalf("expected restored_from_snapshot=true, got %v", body["restored_from_snapshot"])
	}
	build := am.builds["active-status-stale-lease"]
	if build == nil {
		t.Fatal("expected stale leased snapshot to materialize a live build")
	}
	build.mu.RLock()
	defer build.mu.RUnlock()
	if build.SnapshotState.RestoreContext == nil || build.SnapshotState.RestoreContext.ActiveOwnerInstanceID != "reader-instance" {
		t.Fatalf("expected restored build lease owner to switch to reader-instance, got %+v", build.SnapshotState.RestoreContext)
	}
}

func TestGetBuildStatusRestoresFreshLeaseSnapshotWhenTaskTimedOut(t *testing.T) {
	db := openBuildTestDB(t)
	freshHeartbeat := time.Now().UTC()
	stateJSON := fmt.Sprintf(`{
		"current_phase":"backend_services",
		"restore_context":{
			"subscription_plan":"team",
			"provider_mode":"platform",
			"active_owner_instance_id":"owner-instance",
			"active_owner_heartbeat_at":%q
		}
	}`, freshHeartbeat.Format(time.RFC3339Nano))
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "active-status-fresh-lease-stale-task",
		UserID:      1,
		Description: "Fresh lease should still restore if the active task has timed out",
		Status:      string(BuildInProgress),
		Mode:        string(ModeFull),
		PowerMode:   string(PowerBalanced),
		Progress:    59,
		FilesJSON:   "[]",
		AgentsJSON: `[{
			"id":"backend-1",
			"role":"backend",
			"provider":"gpt4",
			"status":"working",
			"build_id":"active-status-fresh-lease-stale-task",
			"progress":59
		}]`,
		TasksJSON: `[{
			"id":"task-api",
			"type":"generate_api",
			"description":"Implement backend API",
			"assigned_to":"backend-1",
			"status":"in_progress",
			"created_at":"2026-03-30T07:00:00Z",
			"started_at":"2026-03-30T07:00:00Z"
		}]`,
		StateJSON: stateJSON,
		CreatedAt: time.Now().UTC().Add(-20 * time.Minute),
		UpdatedAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("create active snapshot: %v", err)
	}

	am := &AgentManager{
		db:          db,
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		resultQueue: make(chan *TaskResult, 1),
		ctx:         context.Background(),
		instanceID:  "reader-instance",
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/build/active-status-fresh-lease-stale-task/status", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["live"] != true {
		t.Fatalf("expected timed-out task snapshot to restore as live, got %v", body["live"])
	}
	if body["restored_from_snapshot"] != true {
		t.Fatalf("expected restored_from_snapshot=true, got %v", body["restored_from_snapshot"])
	}
	if _, exists := am.builds["active-status-fresh-lease-stale-task"]; !exists {
		t.Fatal("expected timed-out task snapshot to materialize a live build")
	}
}

func TestClaimActiveSnapshotTakeoverRetriesAfterLeaseHeartbeatRace(t *testing.T) {
	db := openBuildTestDB(t)
	olderHeartbeat := time.Now().UTC().Add(-30 * time.Second)
	newerHeartbeat := time.Now().UTC()

	staleTaskJSON := `[{
		"id":"task-api",
		"type":"generate_api",
		"description":"Implement backend API",
		"assigned_to":"backend-1",
		"status":"in_progress",
		"created_at":"2026-03-30T07:00:00Z",
		"started_at":"2026-03-30T07:00:00Z"
	}]`

	oldStateJSON := fmt.Sprintf(`{
		"current_phase":"backend_services",
		"restore_context":{
			"subscription_plan":"team",
			"provider_mode":"platform",
			"active_owner_instance_id":"owner-instance",
			"active_owner_heartbeat_at":%q
		}
	}`, olderHeartbeat.Format(time.RFC3339Nano))
	newStateJSON := fmt.Sprintf(`{
		"current_phase":"backend_services",
		"restore_context":{
			"subscription_plan":"team",
			"provider_mode":"platform",
			"active_owner_instance_id":"owner-instance",
			"active_owner_heartbeat_at":%q
		}
	}`, newerHeartbeat.Format(time.RFC3339Nano))

	row := &models.CompletedBuild{
		BuildID:     "active-status-claim-race",
		UserID:      1,
		Description: "Concurrent heartbeat refresh should not block takeover",
		Status:      string(BuildInProgress),
		Mode:        string(ModeFull),
		PowerMode:   string(PowerBalanced),
		Progress:    59,
		AgentsJSON: `[{
			"id":"backend-1",
			"role":"backend",
			"provider":"gpt4",
			"status":"working",
			"build_id":"active-status-claim-race",
			"progress":59
		}]`,
		TasksJSON: staleTaskJSON,
		StateJSON: newStateJSON,
		CreatedAt: time.Now().UTC().Add(-20 * time.Minute),
		UpdatedAt: time.Now().UTC(),
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("create active snapshot: %v", err)
	}

	am := &AgentManager{
		db:          db,
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		resultQueue: make(chan *TaskResult, 1),
		ctx:         context.Background(),
		instanceID:  "reader-instance",
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}

	staleSnapshot := *row
	staleSnapshot.StateJSON = oldStateJSON

	claimed, ok, err := am.claimActiveSnapshotTakeover(&staleSnapshot)
	if err != nil {
		t.Fatalf("claimActiveSnapshotTakeover returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected takeover to succeed after refreshing snapshot state")
	}
	state := parseBuildSnapshotState(claimed.StateJSON)
	if state.RestoreContext == nil || state.RestoreContext.ActiveOwnerInstanceID != "reader-instance" {
		t.Fatalf("expected claimed snapshot owner to switch to reader-instance, got %+v", state.RestoreContext)
	}
}

func TestGetBuildStatusRestoresClaimedSnapshotOnlyOnce(t *testing.T) {
	db := openBuildTestDB(t)
	staleHeartbeat := time.Now().UTC().Add(-2 * time.Minute)
	stateJSON := fmt.Sprintf(`{
		"current_phase":"review",
		"restore_context":{
			"subscription_plan":"team",
			"provider_mode":"platform",
			"active_owner_instance_id":"owner-instance",
			"active_owner_heartbeat_at":%q
		}
	}`, staleHeartbeat.Format(time.RFC3339Nano))
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "active-status-restore-once",
		UserID:      1,
		Description: "Restore a claimed snapshot exactly once on status read",
		Status:      string(BuildReviewing),
		Mode:        string(ModeFast),
		PowerMode:   string(PowerFast),
		Progress:    92,
		FilesJSON:   "[]",
		AgentsJSON: `[{
			"id":"lead-1",
			"role":"lead",
			"provider":"claude",
			"status":"working",
			"build_id":"active-status-restore-once",
			"progress":80
		}]`,
		TasksJSON: `[{
			"id":"task-review",
			"type":"review",
			"description":"Review the generated frontend",
			"assigned_to":"lead-1",
			"status":"in_progress",
			"created_at":"2026-03-29T01:00:00Z",
			"started_at":"2026-03-29T01:00:00Z"
		}]`,
		StateJSON: stateJSON,
		CreatedAt: time.Now().UTC().Add(-10 * time.Minute),
		UpdatedAt: time.Now().UTC().Add(-10 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("create active snapshot: %v", err)
	}

	am := &AgentManager{
		db:          db,
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		taskQueue:   make(chan *Task, 8),
		resultQueue: make(chan *TaskResult, 1),
		ctx:         context.Background(),
		instanceID:  "reader-instance",
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/build/active-status-restore-once/status", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["live"] != true {
		t.Fatalf("expected restored claimed snapshot to be live, got %v", body["live"])
	}
	if body["restored_from_snapshot"] != true {
		t.Fatalf("expected restored_from_snapshot=true, got %v", body["restored_from_snapshot"])
	}

	select {
	case resumed := <-am.taskQueue:
		if resumed == nil || resumed.ID != "task-review" {
			t.Fatalf("expected restored task-review to be requeued once, got %+v", resumed)
		}
	default:
		t.Fatalf("expected restored active snapshot to resume execution")
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/build/active-status-restore-once/status", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected second poll to return 200, got %d: %s", w.Code, w.Body.String())
	}

	select {
	case duplicate := <-am.taskQueue:
		t.Fatalf("expected no duplicate resume task on repeated read, got %+v", duplicate)
	default:
	}
}

func TestGetBuildStatusNormalizesLiveProgressWithinPhaseWindow(t *testing.T) {
	am := &AgentManager{
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
	}

	build := &Build{
		ID:          "live-progress-status",
		UserID:      1,
		Status:      BuildInProgress,
		Description: "Live build should not overstate architecture progress",
		Progress:    99,
		Agents:      map[string]*Agent{},
		Tasks:       []*Task{},
		SnapshotState: BuildSnapshotState{
			CurrentPhase: "architecture",
		},
	}
	am.builds[build.ID] = build

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/build/live-progress-status/status", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got := int(body["progress"].(float64)); got != 19 {
		t.Fatalf("expected architecture progress to be capped at 19, got %d", got)
	}
}

func TestGetBuildStatusSelfHealsStaleLiveTask(t *testing.T) {
	am := &AgentManager{
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		resultQueue: make(chan *TaskResult, 1),
		ctx:         context.Background(),
	}

	startedAt := time.Now().Add(-10 * time.Minute)
	build := &Build{
		ID:          "live-stale-status-recovery",
		UserID:      1,
		Status:      BuildTesting,
		Mode:        ModeFull,
		PowerMode:   PowerBalanced,
		Description: "Status polling should recover stale test tasks",
		UpdatedAt:   time.Now().Add(-10 * time.Minute),
		Agents: map[string]*Agent{
			"testing-1": {
				ID:       "testing-1",
				BuildID:  "live-stale-status-recovery",
				Role:     RoleTesting,
				Provider: ai.ProviderGemini,
				Status:   StatusWorking,
			},
		},
		Tasks: []*Task{
			{
				ID:          "task-stale-test",
				Type:        TaskTest,
				Description: "Verify integration",
				Status:      TaskInProgress,
				AssignedTo:  "testing-1",
				StartedAt:   &startedAt,
				CreatedAt:   startedAt,
			},
		},
	}
	am.builds[build.ID] = build
	am.agents["testing-1"] = build.Agents["testing-1"]

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/build/live-stale-status-recovery/status", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	select {
	case result := <-am.resultQueue:
		if result == nil || result.TaskID != "task-stale-test" || result.Success {
			t.Fatalf("unexpected recovery result: %+v", result)
		}
	default:
		t.Fatal("expected status read to enqueue stale task recovery")
	}

	if got := taskInputInt(build.Tasks[0].Input, "stale_recovery_attempt"); got != 0 {
		t.Fatalf("expected stale recovery attempt marker 0, got %d", got)
	}
}

func TestGetBuildDetailsNormalizesLiveProgressWithinPhaseWindow(t *testing.T) {
	am := &AgentManager{
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
	}

	build := &Build{
		ID:          "live-progress-details",
		UserID:      1,
		Status:      BuildReviewing,
		Description: "Live build details should not overstate review progress",
		Progress:    99,
		Agents:      map[string]*Agent{},
		Tasks:       []*Task{},
		SnapshotState: BuildSnapshotState{
			CurrentPhase: "review",
		},
	}
	am.builds[build.ID] = build

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/build/live-progress-details", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got := int(body["progress"].(float64)); got != 98 {
		t.Fatalf("expected review progress to be capped at 98, got %d", got)
	}
}

func TestNormalizeBuildMessageProgressCapsActiveBuildUpdates(t *testing.T) {
	msg := &WSMessage{
		Type: WSBuildProgress,
		Data: map[string]any{
			"progress": 99,
		},
	}

	normalizeBuildMessageProgress(msg, BuildSnapshotState{CurrentPhase: "frontend_ui"}, BuildInProgress)

	data := msg.Data.(map[string]any)
	if got := data["progress"]; got != 44 {
		t.Fatalf("expected frontend_ui progress to be capped at 44, got %v", got)
	}
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

func TestPreflightReturnsSemanticStateForStaticRequest(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}

	body := bytes.NewBufferString(`{
		"description":"Build a polished static marketing site for an AI operations studio. Frontend only. No backend. No database. No auth. No billing. No realtime.",
		"provider_mode":"platform"
	}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/preflight", body)
	req.Header.Set("Content-Type", "application/json")
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var bodyJSON map[string]any
	json.Unmarshal(w.Body.Bytes(), &bodyJSON)
	if got := bodyJSON["classification"]; got != string(BuildClassificationStaticReady) {
		t.Fatalf("expected static classification, got %v", got)
	}
	if got := bodyJSON["upgrade_required"]; got != false {
		t.Fatalf("expected upgrade_required=false, got %v", got)
	}
	policy, ok := bodyJSON["policy"].(map[string]any)
	if !ok {
		t.Fatalf("expected policy object, got %T", bodyJSON["policy"])
	}
	if got := policy["classification"]; got != string(BuildClassificationStaticReady) {
		t.Fatalf("expected policy classification alias, got %v", got)
	}
	capabilityState, ok := bodyJSON["capability_detector"].(map[string]any)
	if !ok {
		t.Fatalf("expected capability_detector object, got %T", bodyJSON["capability_detector"])
	}
	if got, exists := capabilityState["requires_backend_runtime"]; exists && got != false {
		t.Fatalf("expected requires_backend_runtime to be false or omitted, got %v", got)
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

func TestBuildSnapshotStateResponseFieldsIncludesSemanticAliases(t *testing.T) {
	state := BuildSnapshotState{
		CapabilityState: &BuildCapabilityState{
			RequiresBackendRuntime: false,
		},
		PolicyState: &BuildPolicyState{
			PlanType:           "free",
			Classification:     BuildClassificationStaticReady,
			UpgradeRequired:    false,
			StaticFrontendOnly: true,
		},
	}

	fields := buildSnapshotStateResponseFields(state, "completed")
	if got := fields["classification"]; got != BuildClassificationStaticReady {
		t.Fatalf("expected classification alias, got %v", got)
	}
	if _, ok := fields["policy"]; !ok {
		t.Fatalf("expected policy alias in response fields")
	}
	if _, ok := fields["capability_detector"]; !ok {
		t.Fatalf("expected capability_detector alias in response fields")
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

func TestStartBuildFallsBackToFrontendPreviewForFreeFullStackRequests(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.User{
		ID:               1,
		Username:         "free-user",
		Email:            "free@example.com",
		PasswordHash:     "hashed",
		SubscriptionType: "free",
		CreditBalance:    10,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	am := &AgentManager{
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		ctx:         context.Background(),
		db:          db,
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}

	body, _ := json.Marshal(map[string]any{
		"description": "Build a SaaS app with login, Stripe billing, and a Postgres database.",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var response BuildResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if strings.TrimSpace(response.BuildID) == "" {
		t.Fatalf("expected build ID in response, got %+v", response)
	}
	build, err := am.GetBuild(response.BuildID)
	if err != nil {
		t.Fatalf("expected created build in manager: %v", err)
	}
	build.mu.RLock()
	defer build.mu.RUnlock()
	if !build.RequirePreviewReady {
		t.Fatalf("expected frontend/full-stack free fallback builds to require preview readiness")
	}
	if build.SnapshotState.PolicyState == nil {
		t.Fatalf("expected policy state on build")
	}
	if !build.SnapshotState.PolicyState.StaticFrontendOnly {
		t.Fatalf("expected free full-stack request to enter static frontend fallback, got %+v", build.SnapshotState.PolicyState)
	}
	if build.SnapshotState.PolicyState.RequiredPlan != "builder" {
		t.Fatalf("expected required plan builder, got %+v", build.SnapshotState.PolicyState)
	}
}

func TestStartBuildRejectsHostedOllamaRoleAssignments(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.User{
		ID:               1,
		Username:         "builder-user",
		Email:            "builder@example.com",
		PasswordHash:     "hashed",
		SubscriptionType: "builder",
		CreditBalance:    25,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	am := &AgentManager{
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		ctx:         context.Background(),
		db:          db,
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4, ai.ProviderOllama},
			userProviders: []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4, ai.ProviderOllama},
		},
	}

	body, _ := json.Marshal(map[string]any{
		"description":   "Build a full stack analytics dashboard with auth and a database.",
		"provider_mode": "platform",
		"role_assignments": map[string]string{
			"coder": "ollama",
		},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Ollama is local/BYOK-only") {
		t.Fatalf("expected hosted ollama rejection, got %s", w.Body.String())
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

func TestSendMessageTargetsSpecificAgentWithoutPlannerRoundTrip(t *testing.T) {
	routerStub := &stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
		generateResult: &ai.AIResponse{
			Content: `{"reply":"planner reply should not be used for direct agent messaging","apply_changes":false}`,
		},
	}

	am := &AgentManager{
		ctx:         context.Background(),
		cancel:      func() {},
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		aiRouter:    routerStub,
	}

	build := &Build{
		ID:          "live-build",
		UserID:      1,
		Status:      BuildInProgress,
		Description: "Live build with direct agent control",
		Agents: map[string]*Agent{
			"lead-1": {
				ID:       "lead-1",
				BuildID:  "live-build",
				Role:     RoleLead,
				Provider: ai.ProviderClaude,
				Model:    "claude-sonnet-4-6",
			},
			"frontend-1": {
				ID:       "frontend-1",
				BuildID:  "live-build",
				Role:     RoleFrontend,
				Provider: ai.ProviderClaude,
				Model:    "claude-sonnet-4-6",
			},
		},
	}
	am.builds[build.ID] = build

	body, _ := json.Marshal(map[string]string{
		"content":         "Tighten the header spacing and keep the controls visible.",
		"target_mode":     "agent",
		"target_agent_id": "frontend-1",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/live-build/message", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response["message"] != "Message sent to selected agent" {
		t.Fatalf("expected selected-agent response, got %v", response["message"])
	}
	if got := routerStub.generateCalls.Load(); got != 0 {
		t.Fatalf("expected planner AI not to run for direct agent message, got %d calls", got)
	}

	build.mu.RLock()
	defer build.mu.RUnlock()
	if len(build.Interaction.Messages) != 1 {
		t.Fatalf("expected one interaction message, got %d", len(build.Interaction.Messages))
	}
	msg := build.Interaction.Messages[0]
	if msg.Kind != ConversationKindDirective {
		t.Fatalf("expected direct message to be stored as directive, got %s", msg.Kind)
	}
	if msg.TargetMode != BuildMessageTargetAgent || msg.TargetAgentID != "frontend-1" {
		t.Fatalf("expected agent target metadata, got %+v", msg)
	}
}

func TestGetBuildDetailsIncludesSnapshotState(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "activity-build",
		UserID:      1,
		Description: "Build a preview-first dashboard",
		Status:      "completed",
		Mode:        "full",
		PowerMode:   "balanced",
		Progress:    100,
		FilesJSON:   "[]",
		AgentsJSON: `[{
			"id":"agent-1",
			"role":"architect",
			"provider":"claude",
			"status":"completed",
			"progress":100,
			"current_task":{"id":"task-1","type":"plan","description":"Plan the preview handoff"}
		}]`,
		TasksJSON: `[{
			"id":"task-1",
			"type":"plan",
			"description":"Plan the preview handoff",
			"assigned_to":"agent-1",
			"status":"completed"
		}]`,
		CheckpointsJSON: `[{
			"id":"checkpoint-1",
			"build_id":"activity-build",
			"number":1,
			"name":"Plan Ready",
			"description":"Initial plan completed",
			"progress":35,
			"restorable":false,
			"created_at":"2026-03-12T11:58:00Z"
		}]`,
		ActivityJSON: `[{
			"id":"activity-1",
			"agent_id":"agent-1",
			"agent_role":"architect",
			"provider":"claude",
			"type":"thinking",
			"event_type":"agent:thinking",
			"content":"Planning preview handoff",
			"timestamp":"2026-03-12T12:00:00Z"
		}]`,
		StateJSON: `{
			"current_phase":"completed",
			"quality_gate_required":true,
			"quality_gate_status":"passed",
			"quality_gate_stage":"validation",
			"available_providers":["claude","gpt4"]
		}`,
	}).Error; err != nil {
		t.Fatalf("create completed build: %v", err)
	}

	am := &AgentManager{
		db:          db,
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/build/activity-build", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	timeline, ok := body["activity_timeline"].([]any)
	if !ok || len(timeline) != 1 {
		t.Fatalf("expected 1 activity timeline entry, got %v", body["activity_timeline"])
	}
	entry, ok := timeline[0].(map[string]any)
	if !ok {
		t.Fatalf("expected activity entry object, got %T", timeline[0])
	}
	if entry["agent_role"] != "architect" {
		t.Fatalf("expected architect role, got %v", entry["agent_role"])
	}
	if entry["content"] != "Planning preview handoff" {
		t.Fatalf("expected persisted activity content, got %v", entry["content"])
	}
	agents, ok := body["agents"].([]any)
	if !ok || len(agents) != 1 {
		t.Fatalf("expected 1 persisted agent, got %v", body["agents"])
	}
	tasks, ok := body["tasks"].([]any)
	if !ok || len(tasks) != 1 {
		t.Fatalf("expected 1 persisted task, got %v", body["tasks"])
	}
	checkpoints, ok := body["checkpoints"].([]any)
	if !ok || len(checkpoints) != 1 {
		t.Fatalf("expected 1 persisted checkpoint, got %v", body["checkpoints"])
	}
	checkpoint, ok := checkpoints[0].(map[string]any)
	if !ok {
		t.Fatalf("expected checkpoint object, got %T", checkpoints[0])
	}
	if checkpoint["restorable"] != false {
		t.Fatalf("expected historical checkpoint restorable=false, got %v", checkpoint["restorable"])
	}
	if body["current_phase"] != "completed" {
		t.Fatalf("expected persisted current_phase=completed, got %v", body["current_phase"])
	}
	if body["quality_gate_required"] != true {
		t.Fatalf("expected persisted quality_gate_required=true, got %v", body["quality_gate_required"])
	}
	if body["quality_gate_passed"] != true {
		t.Fatalf("expected persisted quality_gate_passed=true, got %v", body["quality_gate_passed"])
	}
	if body["quality_gate_stage"] != "validation" {
		t.Fatalf("expected persisted quality_gate_stage=validation, got %v", body["quality_gate_stage"])
	}
	providers, ok := body["available_providers"].([]any)
	if !ok || len(providers) != 2 {
		t.Fatalf("expected persisted available providers, got %v", body["available_providers"])
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/builds/activity-build", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected completed build 200, got %d: %s", w.Code, w.Body.String())
	}

	body = map[string]any{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal completed build response: %v", err)
	}
	if body["current_phase"] != "completed" {
		t.Fatalf("expected completed build current_phase=completed, got %v", body["current_phase"])
	}
	if body["quality_gate_passed"] != true {
		t.Fatalf("expected completed build quality_gate_passed=true, got %v", body["quality_gate_passed"])
	}
}

func TestCompletedBuildEndpointsPresentCompletedTerminalSnapshot(t *testing.T) {
	db := openBuildTestDB(t)
	completedAt := time.Now().UTC()
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "terminal-presented-build",
		UserID:      1,
		Description: "Completed build should present terminal success",
		Status:      string(BuildFailed),
		Mode:        string(ModeFull),
		PowerMode:   string(PowerFast),
		Progress:    93,
		Error:       "",
		FilesJSON:   "[]",
		StateJSON:   `{"current_phase":"completed","quality_gate_required":true,"quality_gate_status":"passed","quality_gate_stage":"validation"}`,
		CreatedAt:   completedAt.Add(-time.Minute),
		UpdatedAt:   completedAt,
		CompletedAt: &completedAt,
	}).Error; err != nil {
		t.Fatalf("create completed build snapshot: %v", err)
	}

	am := &AgentManager{db: db}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/build/terminal-presented-build/status", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected build status 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal build status response: %v", err)
	}
	if body["status"] != "completed" {
		t.Fatalf("expected normalized status completed, got %v", body["status"])
	}
	if body["progress"] != float64(100) {
		t.Fatalf("expected normalized progress 100, got %v", body["progress"])
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/builds/terminal-presented-build", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected completed build 200, got %d: %s", w.Code, w.Body.String())
	}

	body = map[string]any{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal completed build response: %v", err)
	}
	if body["status"] != "completed" {
		t.Fatalf("expected completed build status=completed, got %v", body["status"])
	}
	if body["progress"] != float64(100) {
		t.Fatalf("expected completed build progress=100, got %v", body["progress"])
	}
	if body["resumable"] != false {
		t.Fatalf("expected completed build resumable=false, got %v", body["resumable"])
	}
}

func TestSnapshotReadEndpointsFallbackToPersistedState(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "snapshot-read-build",
		UserID:      1,
		Description: "Build a snapshot-readable dashboard",
		Status:      "completed",
		Mode:        "full",
		PowerMode:   "balanced",
		Progress:    100,
		FilesJSON:   "[]",
		AgentsJSON: `[{
			"id":"agent-1",
			"role":"architect",
			"provider":"claude",
			"status":"completed",
			"progress":100
		}]`,
		TasksJSON: `[{
			"id":"task-1",
			"type":"plan",
			"description":"Plan the restore flow",
			"status":"completed"
		}]`,
		CheckpointsJSON: `[{
			"id":"checkpoint-1",
			"build_id":"snapshot-read-build",
			"number":1,
			"name":"Plan Ready",
			"description":"Initial plan completed",
			"progress":35,
			"restorable":false,
			"created_at":"2026-03-12T11:58:00Z"
		}]`,
	}).Error; err != nil {
		t.Fatalf("create completed build: %v", err)
	}

	am := &AgentManager{
		db:          db,
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
	}

	tests := []struct {
		path      string
		countKey  string
		wantCount float64
	}{
		{path: "/api/v1/build/snapshot-read-build/checkpoints", countKey: "count", wantCount: 1},
		{path: "/api/v1/build/snapshot-read-build/agents", countKey: "count", wantCount: 1},
		{path: "/api/v1/build/snapshot-read-build/tasks", countKey: "total", wantCount: 1},
	}

	for _, tc := range tests {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", tc.path, nil)
		testRouter(am).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d: %s", tc.path, w.Code, w.Body.String())
		}

		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: unmarshal response: %v", tc.path, err)
		}
		if body["live"] != false {
			t.Fatalf("%s: expected live=false, got %v", tc.path, body["live"])
		}
		if body[tc.countKey] != tc.wantCount {
			t.Fatalf("%s: expected %s=%v, got %v", tc.path, tc.countKey, tc.wantCount, body[tc.countKey])
		}
	}
}

func TestApproveAllEditsRestoresAwaitingReviewSnapshot(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "review-build",
		UserID:      1,
		Description: "Review and approve the generated homepage",
		Status:      "awaiting_review",
		Mode:        "full",
		PowerMode:   "balanced",
		Progress:    82,
		FilesJSON:   "[]",
		AgentsJSON: `[{
			"id":"lead-1",
			"role":"lead",
			"provider":"claude",
			"status":"working",
			"build_id":"review-build",
			"progress":82
		}]`,
	}).Error; err != nil {
		t.Fatalf("create review snapshot: %v", err)
	}

	editStore := NewProposedEditStoreWithDB(db)
	editStore.AddProposedEdits("review-build", []*ProposedEdit{
		{
			AgentID:         "lead-1",
			AgentRole:       string(RoleLead),
			TaskID:          "task-1",
			FilePath:        "src/App.tsx",
			OriginalContent: "",
			ProposedContent: "export default function App(){return <main>Approved</main>}\n",
			Language:        "typescript",
		},
	})

	am := &AgentManager{
		db: db,
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		editStore:   editStore,
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/build/review-build/proposed-edits", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected proposed edits 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal proposed edits response: %v", err)
	}
	if body["count"] != float64(1) {
		t.Fatalf("expected one proposed edit, got %v", body["count"])
	}
	if body["live"] != false {
		t.Fatalf("expected proposed edits response to come from snapshot, got %v", body["live"])
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/build/review-build/approve-all", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected approve-all 200, got %d: %s", w.Code, w.Body.String())
	}

	body = map[string]any{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal approve-all response: %v", err)
	}
	if body["restored_session"] != true {
		t.Fatalf("expected restored_session=true, got %v", body["restored_session"])
	}

	build, err := am.GetBuild("review-build")
	if err != nil {
		t.Fatalf("expected restored live build session: %v", err)
	}
	if build.Status != BuildInProgress {
		t.Fatalf("expected restored build to resume in_progress, got %s", build.Status)
	}
	files := am.collectGeneratedFiles(build)
	if len(files) != 1 || files[0].Path != "src/App.tsx" {
		t.Fatalf("expected approved edits to become generated files, got %+v", files)
	}
	if pending := am.editStore.GetPendingEdits("review-build"); len(pending) != 0 {
		t.Fatalf("expected pending edits to be cleared, got %d", len(pending))
	}
}

func TestPauseAndResumeRestoreActiveSnapshot(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "paused-build",
		UserID:      1,
		Description: "Pause and resume a restored snapshot",
		Status:      "in_progress",
		Mode:        "full",
		PowerMode:   "balanced",
		Progress:    58,
		FilesJSON:   "[]",
	}).Error; err != nil {
		t.Fatalf("create active snapshot: %v", err)
	}

	am := &AgentManager{
		db: db,
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		editStore:   NewProposedEditStoreWithDB(db),
	}

	body, _ := json.Marshal(map[string]string{"reason": "Need to inspect the current output"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/paused-build/pause", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected pause 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal pause response: %v", err)
	}
	if response["restored_session"] != true {
		t.Fatalf("expected restored_session=true, got %v", response["restored_session"])
	}
	interaction, ok := response["interaction"].(map[string]any)
	if !ok || interaction["paused"] != true {
		t.Fatalf("expected paused interaction response, got %v", response["interaction"])
	}

	build, err := am.GetBuild("paused-build")
	if err != nil {
		t.Fatalf("expected restored build: %v", err)
	}
	if !build.Interaction.Paused {
		t.Fatalf("expected restored build to be paused")
	}

	body, _ = json.Marshal(map[string]string{"reason": "Continue building"})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/build/paused-build/resume", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected resume 200, got %d: %s", w.Code, w.Body.String())
	}

	response = map[string]any{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal resume response: %v", err)
	}
	if response["restored_session"] != false {
		t.Fatalf("expected resume to use existing live session, got %v", response["restored_session"])
	}
	if build.Interaction.Paused {
		t.Fatalf("expected resumed build to clear paused state")
	}
}

func TestCancelRestoresActiveSnapshotAndPersistsTerminalState(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "cancel-build",
		UserID:      1,
		Description: "Cancel a restored snapshot",
		Status:      "reviewing",
		Mode:        "full",
		PowerMode:   "balanced",
		Progress:    91,
		FilesJSON:   "[]",
	}).Error; err != nil {
		t.Fatalf("create review snapshot: %v", err)
	}

	am := &AgentManager{
		db: db,
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		editStore:   NewProposedEditStoreWithDB(db),
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/cancel-build/cancel", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected cancel 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal cancel response: %v", err)
	}
	if response["restored_session"] != true {
		t.Fatalf("expected restored_session=true, got %v", response["restored_session"])
	}

	build, err := am.GetBuild("cancel-build")
	if err != nil {
		t.Fatalf("expected restored build after cancel: %v", err)
	}
	if build.Status != BuildCancelled {
		t.Fatalf("expected cancelled build status, got %s", build.Status)
	}

	var snapshot models.CompletedBuild
	if err := db.Where("build_id = ?", "cancel-build").First(&snapshot).Error; err != nil {
		t.Fatalf("fetch cancelled snapshot: %v", err)
	}
	if snapshot.Status != string(BuildCancelled) {
		t.Fatalf("expected persisted snapshot status cancelled, got %s", snapshot.Status)
	}
	if snapshot.Error != "cancelled by user" {
		t.Fatalf("expected persisted cancel error, got %s", snapshot.Error)
	}
}

func TestDeleteBuildRemovesTerminalSnapshotFromHistory(t *testing.T) {
	db := openBuildTestDB(t)
	snapshot := &models.CompletedBuild{
		BuildID:     "delete-build",
		UserID:      1,
		Description: "Remove this failed build from history",
		Status:      string(BuildFailed),
		Mode:        "full",
		PowerMode:   "balanced",
		Progress:    88,
		FilesJSON:   "[]",
		Error:       "preview failed",
	}
	if err := db.Create(snapshot).Error; err != nil {
		t.Fatalf("create failed snapshot: %v", err)
	}

	am := &AgentManager{
		db: db,
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		editStore:   NewProposedEditStoreWithDB(db),
	}

	am.builds[snapshot.BuildID] = &Build{
		ID:     snapshot.BuildID,
		Status: BuildFailed,
		Agents: map[string]*Agent{
			"lead-delete": {
				ID:      "lead-delete",
				BuildID: snapshot.BuildID,
			},
		},
	}
	am.agents["lead-delete"] = &Agent{
		ID:      "lead-delete",
		BuildID: snapshot.BuildID,
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/builds/delete-build", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected delete 200, got %d: %s", w.Code, w.Body.String())
	}

	var count int64
	if err := db.Model(&models.CompletedBuild{}).Where("build_id = ?", "delete-build").Count(&count).Error; err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected deleted snapshot to disappear from history, found %d rows", count)
	}

	if _, err := am.GetBuild("delete-build"); err == nil {
		t.Fatalf("expected deleted build to be forgotten from memory")
	}
}

func TestDeleteBuildRejectsActiveSnapshot(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "active-build-delete",
		UserID:      1,
		Description: "Still running",
		Status:      string(BuildReviewing),
		Mode:        "full",
		PowerMode:   "balanced",
		Progress:    94,
		FilesJSON:   "[]",
	}).Error; err != nil {
		t.Fatalf("create active snapshot: %v", err)
	}

	am := &AgentManager{
		db: db,
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		editStore:   NewProposedEditStoreWithDB(db),
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/builds/active-build-delete", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected active delete 409, got %d: %s", w.Code, w.Body.String())
	}

	var count int64
	if err := db.Model(&models.CompletedBuild{}).Where("build_id = ?", "active-build-delete").Count(&count).Error; err != nil {
		t.Fatalf("count active snapshots: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected active snapshot to remain, found %d rows", count)
	}
}

func TestGetBuildDetailsMarksRestoredTerminalBuildAsNotLive(t *testing.T) {
	db := openBuildTestDB(t)
	snapshot := &models.CompletedBuild{
		BuildID:     "restored-failed-build",
		UserID:      1,
		Description: "Failed build restored into memory",
		Status:      string(BuildFailed),
		Mode:        "full",
		PowerMode:   "balanced",
		Progress:    88,
		FilesJSON:   "[]",
		Error:       "Build timed out after 40m0s",
		AgentsJSON: `[{
			"id":"lead-1",
			"role":"lead",
			"provider":"claude",
			"status":"working",
			"progress":55
		}]`,
		TasksJSON: `[{
			"id":"task-1",
			"type":"fix",
			"description":"Resume recovery",
			"assigned_to":"lead-1",
			"status":"in_progress"
		}]`,
	}
	if err := db.Create(snapshot).Error; err != nil {
		t.Fatalf("create failed snapshot: %v", err)
	}

	am := &AgentManager{
		ctx:         context.Background(),
		cancel:      func() {},
		db:          db,
		aiRouter:    &stubPreflight{configured: true, allProviders: []ai.AIProvider{ai.ProviderClaude}, userProviders: []ai.AIProvider{ai.ProviderClaude}},
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		editStore:   NewProposedEditStoreWithDB(db),
	}

	if _, _, err := am.restoreBuildSessionFromSnapshot(snapshot); err != nil {
		t.Fatalf("restore snapshot into memory: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/build/restored-failed-build", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response["live"] != false {
		t.Fatalf("expected live=false for restored failed build, got %v", response["live"])
	}
}

func TestRestartFailedBuildRestoresSnapshotAndQueuesRevision(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "failed-restart-build",
		UserID:      1,
		Description: "Failed build that should restart cleanly",
		Status:      "failed",
		Mode:        "full",
		PowerMode:   "balanced",
		Progress:    92,
		FilesJSON:   "[]",
		Error:       "Preview validation failed",
	}).Error; err != nil {
		t.Fatalf("create failed snapshot: %v", err)
	}

	routerStub := &stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
	}

	am := &AgentManager{
		ctx:         context.Background(),
		cancel:      func() {},
		db:          db,
		aiRouter:    routerStub,
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		editStore:   NewProposedEditStoreWithDB(db),
		taskQueue:   make(chan *Task, 2),
	}

	body, _ := json.Marshal(map[string]string{
		"command": "restart_failed",
		"content": "Retry the failed build, keep the working files, and fix preview validation.",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/failed-restart-build/message", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal restart response: %v", err)
	}
	if response["restored_session"] != true {
		t.Fatalf("expected restored_session=true, got %v", response["restored_session"])
	}
	if response["message"] != "Failed build restart requested" {
		t.Fatalf("expected restart response message, got %v", response["message"])
	}
	if got := routerStub.generateCalls.Load(); got != 0 {
		t.Fatalf("expected restart path to avoid planner AI, got %d calls", got)
	}

	build, err := am.GetBuild("failed-restart-build")
	if err != nil {
		t.Fatalf("expected restored build: %v", err)
	}

	build.mu.RLock()
	defer build.mu.RUnlock()
	if build.Status != BuildInProgress {
		t.Fatalf("expected restarted build to move to in_progress, got %s", build.Status)
	}
	if build.Error != "" {
		t.Fatalf("expected restart to clear build error, got %q", build.Error)
	}
	if len(build.Tasks) == 0 {
		t.Fatalf("expected restart to enqueue a follow-up task")
	}
	lastTask := build.Tasks[len(build.Tasks)-1]
	if lastTask.Type != TaskFix {
		t.Fatalf("expected restart task type %s, got %s", TaskFix, lastTask.Type)
	}
	if !strings.Contains(lastTask.Description, "Retry the failed build") {
		t.Fatalf("expected restart task description to include user request, got %q", lastTask.Description)
	}
}

func TestClassifyBuildMessageErrorTreatsRestartAvailabilityAsConflict(t *testing.T) {
	if got := classifyBuildMessageError(fmt.Errorf("restart is not available for completed or cancelled builds")); got != http.StatusConflict {
		t.Fatalf("expected restart availability error to map to 409, got %d", got)
	}
}

func TestSendMessageReturnsConflictForDirectMessageToTerminalBuild(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "completed-direct-message-build",
		UserID:      1,
		Description: "Completed build should reject direct agent messaging",
		Status:      "completed",
		Mode:        "full",
		PowerMode:   "balanced",
		Progress:    100,
		FilesJSON:   "[]",
	}).Error; err != nil {
		t.Fatalf("create completed snapshot: %v", err)
	}

	am := &AgentManager{
		ctx:         context.Background(),
		cancel:      func() {},
		db:          db,
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}

	body, _ := json.Marshal(map[string]string{
		"content":         "Tighten the layout",
		"target_mode":     "agent",
		"target_agent_id": "frontend-1",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/completed-direct-message-build/message", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "direct agent messages require an active build") {
		t.Fatalf("expected direct-message conflict details, got %s", w.Body.String())
	}
}

func TestPauseBuildRejectsTerminalSnapshotWithoutRestoringSession(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "completed-build",
		UserID:      1,
		Description: "Completed build should stay terminal",
		Status:      "completed",
		Mode:        "full",
		PowerMode:   "balanced",
		Progress:    100,
		FilesJSON:   "[]",
	}).Error; err != nil {
		t.Fatalf("create completed snapshot: %v", err)
	}

	am := &AgentManager{
		db: db,
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		editStore:   NewProposedEditStoreWithDB(db),
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/completed-build/pause", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected pause 400, got %d: %s", w.Code, w.Body.String())
	}

	if _, err := am.GetBuild("completed-build"); err == nil {
		t.Fatalf("expected terminal snapshot not to restore into a live build session")
	}
}

func TestPauseBuildRestoredSnapshotDoesNotAutoResumeWork(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "restored-active-build",
		UserID:      1,
		Description: "Pause should not resume restored work first",
		Status:      "in_progress",
		Mode:        "full",
		PowerMode:   "balanced",
		Progress:    72,
		AgentsJSON: `[{
			"id":"solver-1",
			"role":"solver",
			"provider":"claude",
			"model":"claude-sonnet-4-6",
			"status":"working",
			"build_id":"restored-active-build",
			"current_task":{"id":"task-fix","type":"fix","description":"Repair the preview build"},
			"progress":72,
			"created_at":"2026-03-22T00:00:00Z",
			"updated_at":"2026-03-22T00:00:00Z"
		}]`,
		TasksJSON: `[{
			"id":"task-fix",
			"type":"fix",
			"description":"Repair the preview build",
			"priority":70,
			"assigned_to":"solver-1",
			"status":"pending",
			"created_at":"2026-03-22T00:00:00Z",
			"max_retries":3
		}]`,
		FilesJSON: "[]",
	}).Error; err != nil {
		t.Fatalf("create active snapshot: %v", err)
	}

	am := newTestIterationManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
	})
	am.db = db
	am.editStore = NewProposedEditStoreWithDB(db)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/restored-active-build/pause", bytes.NewReader([]byte(`{"reason":"hold"}`)))
	req.Header.Set("Content-Type", "application/json")
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected pause 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"restored_session":true`) {
		t.Fatalf("expected restored_session=true, got %s", w.Body.String())
	}

	select {
	case task := <-am.taskQueue:
		t.Fatalf("expected pause route not to auto-resume restored work, got queued task %+v", task)
	default:
	}

	build, err := am.GetBuild("restored-active-build")
	if err != nil {
		t.Fatalf("expected restored live build after pause: %v", err)
	}
	if !build.Interaction.Paused {
		t.Fatalf("expected build to be paused after control route")
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

func TestDownloadCompletedBuildSkipsSuspiciousPaths(t *testing.T) {
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
		{Path: "../secrets.txt", Content: "should-not-export"},
		{Path: "/tmp/absolute.txt", Content: "also-should-not-export"},
	})
	if err != nil {
		t.Fatalf("marshal files: %v", err)
	}
	techStackJSON, err := json.Marshal(TechStack{Backend: "Node.js"})
	if err != nil {
		t.Fatalf("marshal tech stack: %v", err)
	}
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "suspicious-path-build",
		UserID:      1,
		Status:      "completed",
		ProjectName: "demo",
		TechStack:   string(techStackJSON),
		FilesCount:  6,
		FilesJSON:   string(filesJSON),
	}).Error; err != nil {
		t.Fatalf("create completed build snapshot: %v", err)
	}

	am := &AgentManager{db: db}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/builds/suspicious-path-build/download", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	reader, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatalf("open zip response: %v", err)
	}

	names := make(map[string]bool, len(reader.File))
	for _, file := range reader.File {
		names[file.Name] = true
	}

	if !names["server/package.json"] || !names["server/src/index.js"] {
		t.Fatalf("expected safe files to remain in archive, got %v", names)
	}
	if names["../secrets.txt"] || names["/tmp/absolute.txt"] || names["tmp/absolute.txt"] {
		t.Fatalf("expected suspicious paths to be skipped, got %v", names)
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
		"description": "Build me a marketing website with React and a pricing page",
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
		"description": "Build me a marketing website with React and a pricing page",
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

func TestListBuildsReturnsPlatformIssueWhenDatabaseUnavailable(t *testing.T) {
	db := openBuildTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	am := &AgentManager{db: db}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/builds", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response["platform_issue"] != true {
		t.Fatalf("expected platform_issue=true, got %v", response["platform_issue"])
	}
	if response["platform_service"] != "primary_database" {
		t.Fatalf("expected primary_database service, got %v", response["platform_service"])
	}
	if response["retryable"] != true {
		t.Fatalf("expected retryable=true, got %v", response["retryable"])
	}
}

func TestGetCompletedBuildReturnsPlatformIssueWhenDatabaseUnavailable(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "completed-build-db-outage",
		UserID:      1,
		Description: "Completed build during maintenance",
		Status:      string(BuildCompleted),
		Mode:        string(ModeFull),
		PowerMode:   string(PowerBalanced),
		FilesJSON:   "[]",
	}).Error; err != nil {
		t.Fatalf("create build: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	am := &AgentManager{db: db}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/builds/completed-build-db-outage", nil)
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response["platform_issue"] != true {
		t.Fatalf("expected platform_issue=true, got %v", response["platform_issue"])
	}
	if response["platform_service"] != "primary_database" {
		t.Fatalf("expected primary_database service, got %v", response["platform_service"])
	}
}

func TestSendMessageReturnsPlatformIssueWhenSnapshotLookupFails(t *testing.T) {
	db := openBuildTestDB(t)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "message-db-outage",
		UserID:      1,
		Description: "Snapshot exists but database drops during restart",
		Status:      string(BuildFailed),
		Mode:        string(ModeFull),
		PowerMode:   string(PowerBalanced),
		FilesJSON:   "[]",
	}).Error; err != nil {
		t.Fatalf("create build: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	am := &AgentManager{
		db:          db,
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
	}

	body, _ := json.Marshal(map[string]string{
		"content": "Restart from the last healthy checkpoint.",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/message-db-outage/message", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response["platform_issue"] != true {
		t.Fatalf("expected platform_issue=true, got %v", response["platform_issue"])
	}
	if response["platform_service"] != "primary_database" {
		t.Fatalf("expected primary_database service, got %v", response["platform_service"])
	}
}
