package preview

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"apex-build/internal/bundler"
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
