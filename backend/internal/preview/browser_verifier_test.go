package preview

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── filterBrowserNoise ────────────────────────────────────────────────────────

func TestFilterBrowserNoise_RemovesResizeObserver(t *testing.T) {
	in := []string{
		"ResizeObserver loop limit exceeded",
		"ResizeObserver loop completed with undelivered notifications.",
	}
	out := filterBrowserNoise(in)
	if len(out) != 0 {
		t.Errorf("expected ResizeObserver noise filtered, got: %v", out)
	}
}

func TestFilterBrowserNoise_RemovesViteNoise(t *testing.T) {
	in := []string{
		"[vite] hot updated: /src/App.tsx",
		"@vite/client: connected",
	}
	out := filterBrowserNoise(in)
	if len(out) != 0 {
		t.Errorf("expected vite noise filtered, got: %v", out)
	}
}

func TestFilterBrowserNoise_RemovesBlockedExternalHost(t *testing.T) {
	in := []string{
		"net::ERR_BLOCKED_BY_CLIENT at https://fonts.googleapis.com/css2",
		"GET https://cdn.example.com/lib.js net::ERR_NAME_NOT_RESOLVED",
	}
	out := filterBrowserNoise(in)
	if len(out) != 0 {
		t.Errorf("expected blocked-host errors filtered, got: %v", out)
	}
}

func TestFilterBrowserNoise_RemovesFavicon(t *testing.T) {
	in := []string{"GET /favicon.ico 404 (Not Found)"}
	out := filterBrowserNoise(in)
	if len(out) != 0 {
		t.Errorf("expected favicon error filtered, got: %v", out)
	}
}

func TestFilterBrowserNoise_KeepsRealAppErrors(t *testing.T) {
	in := []string{
		"Uncaught ReferenceError: foo is not defined",
		"TypeError: Cannot read properties of undefined (reading 'map')",
		"Uncaught Error: Minified React error #130",
	}
	out := filterBrowserNoise(in)
	if len(out) != 3 {
		t.Errorf("expected 3 real errors preserved, got %d: %v", len(out), out)
	}
}

func TestFilterBrowserNoise_EmptyInput(t *testing.T) {
	if got := filterBrowserNoise(nil); len(got) != 0 {
		t.Errorf("expected nil→empty, got %v", got)
	}
	if got := filterBrowserNoise([]string{}); len(got) != 0 {
		t.Errorf("expected []→empty, got %v", got)
	}
}

func TestLooksLikeAppLevelNotFound(t *testing.T) {
	t.Parallel()

	if !looksLikeAppLevelNotFound("Apex FieldOps AI Dashboard Page Not Found Sorry, that page does not exist. Go to Dashboard") {
		t.Fatal("expected app-level not-found copy to fail browser verification")
	}
	if looksLikeAppLevelNotFound("Dashboard Job Pipeline New Job Crew Management Settings Launch Estimate Swarm") {
		t.Fatal("expected real app copy to pass not-found heuristic")
	}
}

func TestLooksLikeShellOnlyPreview(t *testing.T) {
	t.Parallel()

	shell := "Apex FieldOps AI Dashboard Job Pipeline New Job Crew Management Settings Bootstrapped by Apex.Build"
	if !looksLikeShellOnlyPreview(shell, len(shell)) {
		t.Fatal("expected sidebar-only shell to fail browser verification")
	}
	realApp := "Apex FieldOps AI Dashboard Job Pipeline New Job Crew Management Settings Open Jobs 7 Pending Estimate Value $48,000 Launch Estimate Swarm"
	if looksLikeShellOnlyPreview(realApp, len(realApp)) {
		t.Fatal("expected real dashboard content to pass shell-only heuristic")
	}
}

// ── BrowserVerifier.Available / skipped ──────────────────────────────────────

func TestBrowserVerifier_Skipped_WhenChromePathEmpty(t *testing.T) {
	bv := &BrowserVerifier{chromePath: ""}
	if bv.Available() {
		t.Error("expected Available() == false when chromePath is empty")
	}
	result := bv.VerifyPageLoad(context.Background(), "http://127.0.0.1:9999/")
	if !result.Skipped {
		t.Error("expected Skipped=true when Chrome is not available")
	}
	// Skipped = undetermined (not a failure). Caller is responsible for
	// treating Skipped=true as a non-blocking outcome; Passed may be false.
}

// ── findChrome ────────────────────────────────────────────────────────────────

func TestFindChrome_ReturnsStringOrEmpty(t *testing.T) {
	// findChrome either returns a valid path or "". Both are acceptable.
	result := findChrome()
	// If a path is returned it must be non-empty and not contain newlines.
	if result != "" {
		if strings.Contains(result, "\n") {
			t.Errorf("findChrome returned path with newline: %q", result)
		}
	}
}

func TestFindChromePrefersConfiguredPath(t *testing.T) {
	tmpDir := t.TempDir()
	fakeChrome := filepath.Join(tmpDir, "chromium-browser")
	if err := os.WriteFile(fakeChrome, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake chrome: %v", err)
	}

	t.Setenv("APEX_CHROME_PATH", fakeChrome)
	t.Setenv("CHROME_BIN", "")

	if got := findChrome(); got != fakeChrome {
		t.Fatalf("expected configured chrome path %q, got %q", fakeChrome, got)
	}
}

func TestSmokeTestChromeRejectsEmptyPath(t *testing.T) {
	if err := SmokeTestChrome(context.Background(), ""); err == nil {
		t.Fatal("expected empty chrome path to fail smoke test")
	}
}

func TestSmokeTestChromeFailsWhenBinaryCannotLaunch(t *testing.T) {
	tmpDir := t.TempDir()
	fakeChrome := filepath.Join(tmpDir, "chromium-browser")
	if err := os.WriteFile(fakeChrome, []byte("#!/bin/sh\necho missing-lib >&2\nexit 127\n"), 0o755); err != nil {
		t.Fatalf("write fake chrome: %v", err)
	}

	err := SmokeTestChrome(context.Background(), fakeChrome)
	if err == nil {
		t.Fatal("expected fake failing chrome to fail smoke test")
	}
	if !strings.Contains(err.Error(), "missing-lib") {
		t.Fatalf("expected smoke failure to include stderr, got %v", err)
	}
}

// ── clampMax ─────────────────────────────────────────────────────────────────

func TestClampMax(t *testing.T) {
	cases := []struct{ v, max, want int }{
		{5, 3, 3},
		{2, 3, 2},
		{3, 3, 3},
		{0, 3, 0},
	}
	for _, c := range cases {
		if got := clampMax(c.v, c.max); got != c.want {
			t.Errorf("clampMax(%d,%d) = %d, want %d", c.v, c.max, got, c.want)
		}
	}
}

// ── BrowserVerifier.VerifyPageLoad — mock HTTP server ────────────────────────
// These tests use a real Chrome instance if available, against an httptest
// server that mimics the Vite dev server.  Skipped when Chrome is absent.

func newBrowserTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	defer func() {
		if recovered := recover(); recovered != nil {
			if strings.Contains(fmt.Sprint(recovered), "failed to listen on a port") {
				t.Skipf("local listener unavailable in this environment: %v", recovered)
			}
			panic(recovered)
		}
	}()

	return httptest.NewServer(handler)
}

// mockAppServer creates a minimal HTML page that mounts a React-like div.
// renderedContent controls whether #root has children (simulates JS execution).
func mockAppServer(t *testing.T, renderedContent string) *httptest.Server {
	t.Helper()
	return newBrowserTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Test App</title></head>
<body>
  <div id="root">%s</div>
  <script>
    // Simulate React mount: immediately populate #root
    document.addEventListener('DOMContentLoaded', function() {
      var root = document.getElementById('root');
      if (root && root.childElementCount === 0 && root.textContent.trim() === '') {
        root.innerHTML = '<div class="app"><h1>Hello World</h1></div>';
      }
    });
  </script>
</body>
</html>`, renderedContent)
	}))
}

func TestBrowserVerifier_Skipped_WhenNotAvailable(t *testing.T) {
	bv := &BrowserVerifier{chromePath: ""}
	srv := mockAppServer(t, "")
	defer srv.Close()

	result := bv.VerifyPageLoad(context.Background(), srv.URL)
	if !result.Skipped {
		t.Error("expected Skipped=true when Chrome unavailable")
	}
}

func TestBrowserVerifier_PassesWhenAppRendered(t *testing.T) {
	bv := NewBrowserVerifier()
	if !bv.Available() {
		t.Skip("Chrome not available — skipping browser integration test")
	}

	// Serve a page with pre-rendered content in #root (no JS needed)
	srv := newBrowserTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>App</title></head>
<body>
  <div id="root"><div class="app"><h1>Counter: 0</h1><button>+</button></div></div>
</body>
</html>`)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result := bv.VerifyPageLoad(ctx, srv.URL)
	if result.Skipped {
		t.Skip("Chrome detected at start but unavailable at check — skip")
	}
	if !result.Passed {
		t.Errorf("expected pass for pre-rendered app, got: kind=%s detail=%s jsErrs=%v",
			result.FailureKind, result.Details, result.JSErrors)
	}
	if !result.MountRendered {
		t.Error("expected MountRendered=true")
	}
}

func TestBrowserVerifier_FailsOnBlankMount(t *testing.T) {
	bv := NewBrowserVerifier()
	if !bv.Available() {
		t.Skip("Chrome not available")
	}

	// Serve a page with empty #root and no JS to populate it
	srv := newBrowserTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Blank</title></head>
<body>
  <div id="root"></div>
</body>
</html>`)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result := bv.VerifyPageLoad(ctx, srv.URL)
	if result.Skipped {
		t.Skip("Chrome not available at check time")
	}
	if result.Passed {
		t.Errorf("expected failure for blank mount, got: passed=true detail=%s", result.Details)
	}
	if result.FailureKind != "blank_screen" && result.FailureKind != "js_runtime_error" {
		t.Errorf("expected blank_screen or js_runtime_error, got: %s", result.FailureKind)
	}
}

func TestBrowserVerifier_PassesWhenJSPopulatesMount(t *testing.T) {
	bv := NewBrowserVerifier()
	if !bv.Available() {
		t.Skip("Chrome not available")
	}

	// Simulate React: JS runs synchronously and populates #root
	srv := newBrowserTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>JS App</title></head>
<body>
  <div id="root"></div>
  <script>
    (function() {
      var root = document.getElementById('root');
      root.innerHTML = '<div><h1>Hello from JS</h1><p>App is running.</p></div>';
    })();
  </script>
</body>
</html>`)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result := bv.VerifyPageLoad(ctx, srv.URL)
	if result.Skipped {
		t.Skip("Chrome not available at check time")
	}
	if !result.Passed {
		t.Errorf("expected pass when JS populates mount, got: kind=%s detail=%s jsErrs=%v",
			result.FailureKind, result.Details, result.JSErrors)
	}
}

func TestBrowserVerifier_PollsForDelayedMount(t *testing.T) {
	bv := NewBrowserVerifier()
	if !bv.Available() {
		t.Skip("Chrome not available")
	}

	srv := newBrowserTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Delayed App</title></head>
<body>
  <div id="root"></div>
  <script>
    setTimeout(function() {
      var root = document.getElementById('root');
      root.innerHTML = '<main><h1>Loaded after route chunk</h1><button>Open dashboard</button></main>';
    }, 1500);
  </script>
</body>
</html>`)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result := bv.VerifyPageLoad(ctx, srv.URL)
	if result.Skipped {
		t.Skip("Chrome not available at check time")
	}
	if !result.Passed {
		t.Errorf("expected pass for delayed mount, got: kind=%s detail=%s jsErrs=%v",
			result.FailureKind, result.Details, result.JSErrors)
	}
}

func TestBrowserVerifier_DetectsUncaughtException(t *testing.T) {
	bv := NewBrowserVerifier()
	if !bv.Available() {
		t.Skip("Chrome not available")
	}

	// Page that throws but also leaves mount empty → should fail as js_runtime_error
	srv := newBrowserTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Broken App</title></head>
<body>
  <div id="root"></div>
  <script>
    throw new Error("Fatal: Cannot read module exports");
  </script>
</body>
</html>`)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result := bv.VerifyPageLoad(ctx, srv.URL)
	if result.Skipped {
		t.Skip("Chrome not available at check time")
	}
	if result.Passed {
		t.Errorf("expected failure for uncaught exception with blank mount, got passed=true")
	}
	if result.FailureKind != "js_runtime_error" && result.FailureKind != "blank_screen" {
		t.Errorf("expected js_runtime_error or blank_screen, got: %s", result.FailureKind)
	}
}

// ── Integration test with a real Vite project ────────────────────────────────
// See browser_verifier_integration_test.go (//go:build integration)
