package preview

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"apex-build/internal/bundler"
	"apex-build/pkg/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNormalizePreviewPath(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "", want: ""},
		{in: "   ", want: ""},
		{in: "/index.html", want: "index.html"},
		{in: "./index.html", want: "index.html"},
		{in: "\\src\\App.tsx", want: "src/App.tsx"},
		{in: ".", want: ""},
	}

	for _, tc := range tests {
		if got := normalizePreviewPath(tc.in); got != tc.want {
			t.Fatalf("normalizePreviewPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPreviewLookupVariationsIncludesNormalizedAndHtmlFallbacks(t *testing.T) {
	got := previewLookupVariations("/dashboard")
	want := []string{
		"dashboard",
		"dashboard.html",
		"dashboard/index.html",
		"public/dashboard",
		"src/dashboard",
	}

	if len(got) != len(want) {
		t.Fatalf("previewLookupVariations length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("previewLookupVariations[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestShouldProxyToBackendPathSupportsFullStackPrefixes(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/api/hello", want: true},
		{path: "/graphql", want: true},
		{path: "/trpc/posts.list", want: true},
		{path: "/socket.io/?EIO=4&transport=polling", want: true},
		{path: "/ws", want: true},
		{path: "/dashboard", want: false},
		{path: "/assets/app.js", want: false},
	}

	for _, tc := range tests {
		if got := shouldProxyToBackendPath(tc.path); got != tc.want {
			t.Fatalf("shouldProxyToBackendPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestCreateFileHandlerServesIndexHTMLForBundledRootRequests(t *testing.T) {
	ps := &PreviewServer{}
	session := &PreviewSession{
		ProjectID:  7,
		IsBundled:  true,
		StartedAt:  time.Now(),
		LastAccess: time.Now(),
		FileCache: map[string]*CachedFile{
			"index.html": {
				Content:     "<!doctype html><title>Bundled App</title><div id=\"root\"></div>",
				ContentType: "text/html; charset=utf-8",
			},
			"src/main.tsx": {
				Content:     "import React from \"react\";",
				ContentType: "application/typescript; charset=utf-8",
			},
		},
	}
	handler := ps.createFileHandler(session, &PreviewConfig{EntryPoint: "src/main.tsx"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("expected html content type, got %q", got)
	}
	if !strings.Contains(rec.Body.String(), "Bundled App") {
		t.Fatalf("expected bundled index html, got %q", rec.Body.String())
	}
}

func TestCreateFileHandlerFallsBackToIndexHTMLForBundledSPARoutes(t *testing.T) {
	ps := &PreviewServer{}
	session := &PreviewSession{
		ProjectID:  8,
		IsBundled:  true,
		StartedAt:  time.Now(),
		LastAccess: time.Now(),
		FileCache: map[string]*CachedFile{
			"index.html": {
				Content:     "<!doctype html><title>SPA Shell</title><div id=\"root\"></div>",
				ContentType: "text/html; charset=utf-8",
			},
		},
	}
	handler := ps.createFileHandler(session, &PreviewConfig{EntryPoint: "src/main.tsx"})

	req := httptest.NewRequest(http.MethodGet, "/clients/active", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SPA Shell") {
		t.Fatalf("expected spa shell fallback, got %q", rec.Body.String())
	}
}

func TestFrameworkRuntimePreludeIncludesReactCDNs(t *testing.T) {
	ps := &PreviewServer{}
	session := &PreviewSession{
		BundleConfig: &bundler.BundleConfig{Framework: "react"},
	}

	prelude := ps.frameworkRuntimePrelude(session)
	if !strings.Contains(prelude, "react.development.js") {
		t.Fatalf("expected react runtime prelude, got %q", prelude)
	}
	if !strings.Contains(prelude, "react-dom.development.js") {
		t.Fatalf("expected react-dom runtime prelude, got %q", prelude)
	}
}

func TestGenerateBundleHTMLUsesBundleConfigTitle(t *testing.T) {
	ps := &PreviewServer{}
	session := &PreviewSession{
		BundleConfig: &bundler.BundleConfig{
			Framework: "react",
			Title:     "PulseBoard",
		},
		BundleResult: &bundler.BundleResult{
			OutputJS: []byte("console.log('ready')"),
		},
	}

	html := ps.generateBundleHTML(session, &PreviewConfig{ProjectID: 42, EntryPoint: "src/main.tsx"})
	if !strings.Contains(html, "<title>PulseBoard Preview</title>") {
		t.Fatalf("expected project-specific preview title, got %q", html)
	}
}

func TestPreviewWebSocketOriginAllowsLocalAPIProxyHost(t *testing.T) {
	ps := NewPreviewServer(nil)
	req := httptest.NewRequest(http.MethodGet, "/__apex_ws", nil)
	req.Header.Set("Origin", "http://127.0.0.1:8080")

	if !ps.upgrader.CheckOrigin(req) {
		t.Fatal("expected preview websocket origin check to allow http://127.0.0.1:8080")
	}
}

func TestPreviewWebSocketOriginAllowsSandboxedProxyOrigin(t *testing.T) {
	ps := NewPreviewServer(nil)
	req := httptest.NewRequest(http.MethodGet, "/__apex_ws", nil)
	req.Header.Set("Origin", "null")

	if !ps.upgrader.CheckOrigin(req) {
		t.Fatal("expected preview websocket origin check to allow null origin from sandboxed preview iframes")
	}
}

func TestStartPreviewSkipsOccupiedPortAndReturnsReachableURL(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve occupied port: %v", err)
	}
	defer occupied.Close()
	basePort := occupied.Addr().(*net.TCPAddr).Port

	db := newPreviewTestDB(t, 101)
	ps := NewPreviewServer(db)
	ps.basePort = basePort

	status, err := ps.StartPreview(context.Background(), &PreviewConfig{
		ProjectID:  101,
		EntryPoint: "index.html",
		Framework:  "vanilla",
	})
	if err != nil {
		t.Fatalf("start preview: %v", err)
	}
	defer ps.StopPreview(context.Background(), 101)
	if status.Port == basePort {
		t.Fatalf("preview reused occupied port %d", basePort)
	}
	if !strings.HasPrefix(status.URL, "http://127.0.0.1:") {
		t.Fatalf("preview URL = %q, want loopback URL", status.URL)
	}

	resp, err := http.Get(status.URL) //nolint:gosec // Local preview test endpoint.
	if err != nil {
		t.Fatalf("GET preview URL: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preview status = %d body=%s", resp.StatusCode, string(body))
	}
	if !strings.Contains(string(body), "Preview booted") {
		t.Fatalf("preview body = %q", string(body))
	}
}

func TestGetPreviewStatusRemovesUnreachableSession(t *testing.T) {
	db := newPreviewTestDB(t, 102)
	ps := NewPreviewServer(db)

	status, err := ps.StartPreview(context.Background(), &PreviewConfig{
		ProjectID:  102,
		EntryPoint: "index.html",
		Framework:  "vanilla",
	})
	if err != nil {
		t.Fatalf("start preview: %v", err)
	}
	session := ps.sessions[102]
	if session == nil || session.server == nil {
		t.Fatal("expected preview session")
	}
	_ = session.server.Close()
	waitForPreviewPortClosed(t, status.Port)

	next := ps.GetPreviewStatus(102)
	if next.Active {
		t.Fatalf("unreachable preview still reported active: %+v", next)
	}
	if _, exists := ps.sessions[102]; exists {
		t.Fatal("expected unreachable preview session to be removed")
	}
}

func newPreviewTestDB(t *testing.T, projectID uint) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.File{}); err != nil {
		t.Fatalf("migrate files: %v", err)
	}
	if err := db.Create(&models.File{
		ProjectID: projectID,
		Path:      "index.html",
		Name:      "index.html",
		Type:      "file",
		Content:   "<!doctype html><html><body><main>Preview booted</main></body></html>",
	}).Error; err != nil {
		t.Fatalf("create file: %v", err)
	}
	return db
}

func waitForPreviewPortClosed(t *testing.T, port int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 50*time.Millisecond)
		if err != nil {
			return
		}
		_ = conn.Close()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("preview port %d remained reachable", port)
}
