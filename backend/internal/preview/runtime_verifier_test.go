package preview

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type stubRuntimeVisionVerifier struct {
	result *VisionRepairResult
}

func (s *stubRuntimeVisionVerifier) AnalyzeScreenshot(ctx context.Context, imageData []byte, description string) *VisionRepairResult {
	return s.result
}

type stubRuntimeCanaryTester struct {
	available bool
	result    *CanaryResult
}

func (s *stubRuntimeCanaryTester) Available() bool {
	return s.available
}

func (s *stubRuntimeCanaryTester) RunCanaryInteractions(ctx context.Context, pageURL string) *CanaryResult {
	return s.result
}

// ── Unit tests (no npm/vite required) ────────────────────────────────────────

func TestParseScriptSrcs(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<body>
  <script type="module" src="/src/main.tsx"></script>
  <script src="/vendor.js"></script>
</body>
</html>`
	srcs := parseScriptSrcs(html)
	if len(srcs) != 2 {
		t.Fatalf("expected 2 script srcs, got %d: %v", len(srcs), srcs)
	}
	if srcs[0] != "/src/main.tsx" {
		t.Errorf("expected /src/main.tsx, got %q", srcs[0])
	}
	if srcs[1] != "/vendor.js" {
		t.Errorf("expected /vendor.js, got %q", srcs[1])
	}
}

func TestParseCSSLinks(t *testing.T) {
	html := `<link rel="stylesheet" href="/assets/index.css">
<link href="/assets/vendor.css" rel="stylesheet">`
	links := parseCSSLinks(html)
	if len(links) != 2 {
		t.Fatalf("expected 2 css links, got %d: %v", len(links), links)
	}
}

func TestCheckMountPoint_Found(t *testing.T) {
	html := `<html><body><div id="root"></div></body></html>`
	c := (&RuntimeVerifier{}).checkMountPoint(html)
	if !c.Passed {
		t.Errorf("expected mount point found, got: %s", c.Detail)
	}
}

func TestCheckMountPoint_Missing(t *testing.T) {
	html := `<html><body><div class="container"></div></body></html>`
	c := (&RuntimeVerifier{}).checkMountPoint(html)
	if c.Passed {
		t.Error("expected mount point missing")
	}
}

func TestCheckMountPoint_AppId(t *testing.T) {
	html := `<html><body><div id="app"></div></body></html>`
	c := (&RuntimeVerifier{}).checkMountPoint(html)
	if !c.Passed {
		t.Errorf("expected id=app to be recognized as mount point")
	}
}

func TestViteErrorRe_DetectsTransformError(t *testing.T) {
	errorBody := `[@vite/client] Transform failed with 1 error:\n  SyntaxError: Unexpected token`
	if !viteErrorRe.MatchString(errorBody) {
		t.Error("expected viteErrorRe to match transform error body")
	}
}

func TestViteErrorRe_DoesNotMatchCleanJS(t *testing.T) {
	cleanJS := `import React from 'react'
export default function App() { return React.createElement('div', null, 'Hello') }
`
	if viteErrorRe.MatchString(cleanJS) && len(strings.TrimSpace(cleanJS)) < 500 {
		t.Error("expected viteErrorRe NOT to flag clean JS as error (or it's long enough to not trigger)")
	}
}

// ── HTTP mock-server tests (fast, no npm) ─────────────────────────────────────

// mockViteServer creates a test HTTP server that mimics a working Vite dev server.
func mockViteServer(t *testing.T, opts ...func(mux *http.ServeMux)) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// Default handlers
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Test App</title></head>
<body>
  <div id="root"></div>
  <script type="module" src="/@vite/client"></script>
  <script type="module" src="/src/main.tsx"></script>
</body>
</html>`)
	})

	mux.HandleFunc("/@vite/client", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, `// Vite HMR client\nconsole.log('[vite] connected')`)
	})

	mux.HandleFunc("/src/main.tsx", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, `import React from 'react'; import ReactDOM from 'react-dom/client';
ReactDOM.createRoot(document.getElementById('root')).render(React.createElement('div', null, 'Hello'));`)
	})

	for _, opt := range opts {
		opt(mux)
	}

	return newBrowserTestServer(t, mux)
}

func TestCheckRootPage_OK(t *testing.T) {
	srv := mockViteServer(t)
	defer srv.Close()

	rv := &RuntimeVerifier{}
	client := &http.Client{}
	body, c := rv.checkRootPage(context.Background(), client, srv.URL)
	if !c.Passed {
		t.Errorf("expected root page check to pass: %s", c.Detail)
	}
	if !strings.Contains(body, "root") {
		t.Error("expected body to contain root div")
	}
}

func TestCheckRootPage_RetriesTransient404(t *testing.T) {
	var calls int32
	srv := newBrowserTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			http.Error(w, "warming up", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html><html><body><div id="root"></div><script type="module" src="/src/main.tsx"></script></body></html>`)
	}))
	defer srv.Close()

	rv := &RuntimeVerifier{rootTimeout: time.Second}
	body, c := rv.checkRootPage(context.Background(), &http.Client{}, srv.URL)
	if !c.Passed {
		t.Fatalf("expected transient 404 to recover, got: %s", c.Detail)
	}
	if got := atomic.LoadInt32(&calls); got < 3 {
		t.Fatalf("expected retry attempts, got %d", got)
	}
	if !strings.Contains(body, `id="root"`) {
		t.Fatalf("expected recovered HTML body, got %q", body)
	}
}

func TestCheckRootPage_BlankBody(t *testing.T) {
	srv := newBrowserTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body></body></html>")
	}))
	defer srv.Close()

	rv := &RuntimeVerifier{}
	_, c := rv.checkRootPage(context.Background(), &http.Client{}, srv.URL)
	if c.Passed {
		t.Error("expected blank body to fail root page check")
	}
}

func TestCheckRootPage_Non200(t *testing.T) {
	srv := newBrowserTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	rv := &RuntimeVerifier{rootTimeout: time.Millisecond}
	_, c := rv.checkRootPage(context.Background(), &http.Client{}, srv.URL)
	if c.Passed {
		t.Error("expected 404 to fail root page check")
	}
	if !strings.Contains(c.Detail, "not found") {
		t.Errorf("expected 404 body snippet in detail, got %q", c.Detail)
	}
}

func TestCheckEntryModule_OK(t *testing.T) {
	srv := mockViteServer(t)
	defer srv.Close()

	rv := &RuntimeVerifier{}
	c := rv.checkEntryModule(context.Background(), &http.Client{}, srv.URL, "/src/main.tsx")
	if !c.Passed {
		t.Errorf("expected entry module check to pass: %s", c.Detail)
	}
}

func TestCheckEntryModule_TransformError(t *testing.T) {
	srv := newBrowserTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/src/main.tsx" {
			w.WriteHeader(500)
			fmt.Fprint(w, "Transform failed\nSyntaxError: Unexpected token at line 5")
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	rv := &RuntimeVerifier{}
	c := rv.checkEntryModule(context.Background(), &http.Client{}, srv.URL, "/src/main.tsx")
	if c.Passed {
		t.Error("expected transform error to fail entry module check")
	}
}

func TestCheckEntryModule_Missing(t *testing.T) {
	srv := newBrowserTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	rv := &RuntimeVerifier{}
	c := rv.checkEntryModule(context.Background(), &http.Client{}, srv.URL, "/src/main.tsx")
	if c.Passed {
		t.Error("expected 404 to fail entry module check")
	}
	if !strings.Contains(c.Detail, "404") {
		t.Errorf("expected 404 in detail, got: %s", c.Detail)
	}
}

func TestCheckViteClient_OK(t *testing.T) {
	srv := mockViteServer(t)
	defer srv.Close()

	rv := &RuntimeVerifier{}
	c := rv.checkViteClient(context.Background(), &http.Client{}, srv.URL)
	if !c.Passed {
		t.Errorf("expected vite client check to pass: %s", c.Detail)
	}
}

func TestCheckViteClient_Missing(t *testing.T) {
	srv := newBrowserTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	rv := &RuntimeVerifier{}
	c := rv.checkViteClient(context.Background(), &http.Client{}, srv.URL)
	if c.Passed {
		t.Error("expected missing vite client to fail")
	}
}

// TestWaitForTCPPort verifies the port-waiting helper works for a real listener.
func TestWaitForTCPPort_Ready(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "operation not permitted") {
			t.Skipf("local listener unavailable in this environment: %v", err)
		}
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	stop := make(chan struct{})
	defer close(stop)

	if !waitForTCPPort(port, 2*time.Second, stop) {
		t.Error("expected waitForTCPPort to return true for an open port")
	}
}

func TestWaitForTCPPort_Timeout(t *testing.T) {
	stop := make(chan struct{})
	defer close(stop)
	// Use a port that is definitely not open
	if waitForTCPPort(1, 200*time.Millisecond, stop) {
		t.Error("expected waitForTCPPort to return false for a closed port")
	}
}

func TestTruncateLog(t *testing.T) {
	long := strings.Repeat("x", 2000)
	result := truncateLog(long, 100)
	if len(result) > 103 { // 100 bytes + "…" (3 UTF-8 bytes)
		t.Errorf("expected truncated log <= 103 bytes, got %d", len(result))
	}
	if !strings.HasPrefix(result, "…") {
		t.Error("expected truncated log to start with …")
	}
}

// ── isViteProject helper ──────────────────────────────────────────────────────

func TestIsViteProject_ViaConfig(t *testing.T) {
	fm := map[string]string{"vite.config.ts": "import { defineConfig } from 'vite'"}
	if !isViteProject(fm) {
		t.Error("expected vite.config.ts to be recognized")
	}
}

func TestIsViteProject_ViaPackageJSON(t *testing.T) {
	fm := map[string]string{
		"package.json": `{"name":"x","devDependencies":{"vite":"^5.0.0"}}`,
	}
	if !isViteProject(fm) {
		t.Error("expected vite in package.json to be recognized")
	}
}

func TestIsViteProject_NotVite(t *testing.T) {
	fm := map[string]string{
		"package.json": `{"name":"x","dependencies":{"webpack":"^5"}}`,
	}
	if isViteProject(fm) {
		t.Error("expected webpack-only project NOT to be recognized as Vite")
	}
}

func TestRuntimeVerifierFailsWhenBrowserProofEnabledButChromeUnavailable(t *testing.T) {
	rv := &RuntimeVerifier{browser: &BrowserVerifier{chromePath: ""}}
	result := rv.VerifyViteApp(context.Background(), []VerifiableFile{
		{Path: "package.json", Content: `{"name":"x","scripts":{"dev":"vite"}}`},
	})
	if result.Passed {
		t.Fatal("expected runtime verifier to fail when browser proof is enabled but Chrome is unavailable")
	}
	if result.FailureKind != "browser_unavailable" {
		t.Fatalf("expected failure kind browser_unavailable, got %q", result.FailureKind)
	}
}

func TestRuntimeVerifierDefaultTimeouts(t *testing.T) {
	t.Parallel()

	httpOnly := &RuntimeVerifier{}
	if got := httpOnly.runtimeTotalTimeout(); got != 150*time.Second {
		t.Fatalf("expected default HTTP-only total timeout to be 150s, got %s", got)
	}
	if got := httpOnly.runtimeInstallTimeout(httpOnly.runtimeTotalTimeout()); got != 90*time.Second {
		t.Fatalf("expected default HTTP-only install timeout to be 90s, got %s", got)
	}
	if got := httpOnly.runtimeServerReadyTimeout(httpOnly.runtimeTotalTimeout(), httpOnly.runtimeInstallTimeout(httpOnly.runtimeTotalTimeout())); got != 45*time.Second {
		t.Fatalf("expected default HTTP-only server-ready timeout to be 45s, got %s", got)
	}

	withBrowser := &RuntimeVerifier{browser: &BrowserVerifier{chromePath: "/usr/bin/chromium-browser"}}
	if got := withBrowser.runtimeTotalTimeout(); got != 180*time.Second {
		t.Fatalf("expected browser total timeout to be 180s, got %s", got)
	}
	if got := withBrowser.runtimeInstallTimeout(withBrowser.runtimeTotalTimeout()); got != 120*time.Second {
		t.Fatalf("expected browser install timeout to be 120s, got %s", got)
	}
	if got := withBrowser.runtimeServerReadyTimeout(withBrowser.runtimeTotalTimeout(), withBrowser.runtimeInstallTimeout(withBrowser.runtimeTotalTimeout())); got != 60*time.Second {
		t.Fatalf("expected browser server-ready timeout to be 60s, got %s", got)
	}
}

func TestRuntimeVerifierCustomTimeouts(t *testing.T) {
	t.Parallel()

	rv := &RuntimeVerifier{
		totalTimeout:   42 * time.Second,
		installTimeout: 17 * time.Second,
		readyTimeout:   13 * time.Second,
	}
	if got := rv.runtimeTotalTimeout(); got != 42*time.Second {
		t.Fatalf("expected custom total timeout to be preserved, got %s", got)
	}
	if got := rv.runtimeInstallTimeout(rv.runtimeTotalTimeout()); got != 17*time.Second {
		t.Fatalf("expected custom install timeout to be preserved, got %s", got)
	}
	if got := rv.runtimeServerReadyTimeout(rv.runtimeTotalTimeout(), rv.runtimeInstallTimeout(rv.runtimeTotalTimeout())); got != 13*time.Second {
		t.Fatalf("expected custom server-ready timeout to be preserved, got %s", got)
	}
	if got := formatRuntimeTimeout(rv.runtimeInstallTimeout(rv.runtimeTotalTimeout())); got != "17s" {
		t.Fatalf("expected formatted install timeout 17s, got %q", got)
	}
}

func TestRuntimeVerifierServerReadyTimeoutEnvOverride(t *testing.T) {
	t.Setenv("APEX_PREVIEW_SERVER_READY_TIMEOUT_SECONDS", "75")

	rv := &RuntimeVerifier{}
	if got := rv.runtimeServerReadyTimeout(150*time.Second, 90*time.Second); got != 75*time.Second {
		t.Fatalf("expected env server-ready timeout 75s, got %s", got)
	}
}

func TestRunNpmInstallIncludesDevDependenciesInProductionEnv(t *testing.T) {
	t.Setenv("NODE_ENV", "production")
	t.Setenv("NPM_CONFIG_PRODUCTION", "true")

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}

	argsFile := filepath.Join(root, "args.txt")
	nodeEnvFile := filepath.Join(root, "node-env.txt")
	productionFile := filepath.Join(root, "production.txt")
	npmPath := filepath.Join(root, "npm")
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" > %q
printf '%%s\n' "$NODE_ENV" > %q
printf '%%s\n' "$NPM_CONFIG_PRODUCTION" > %q
exit 0
`, argsFile, nodeEnvFile, productionFile)
	if err := os.WriteFile(npmPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := (&RuntimeVerifier{}).runNpmInstall(context.Background(), appDir, npmPath)
	if err != nil {
		t.Fatalf("runNpmInstall failed: %v\n%s", err, out)
	}

	argsRaw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.TrimSpace(string(argsRaw))
	if !strings.Contains(args, "install --include=dev") {
		t.Fatalf("expected npm install to include dev dependencies, got args %q", args)
	}

	nodeEnvRaw, _ := os.ReadFile(nodeEnvFile)
	if got := strings.TrimSpace(string(nodeEnvRaw)); got != "development" {
		t.Fatalf("expected NODE_ENV=development for preview install, got %q", got)
	}
	productionRaw, _ := os.ReadFile(productionFile)
	if got := strings.TrimSpace(string(productionRaw)); got != "false" {
		t.Fatalf("expected NPM_CONFIG_PRODUCTION=false for preview install, got %q", got)
	}
}

func TestRunNpmInstallCIIncludesDevDependencies(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "package-lock.json"), []byte(`{"lockfileVersion":3}`), 0o644); err != nil {
		t.Fatal(err)
	}

	argsFile := filepath.Join(root, "args.txt")
	npmPath := filepath.Join(root, "npm")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" > %q\nexit 0\n", argsFile)
	if err := os.WriteFile(npmPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := (&RuntimeVerifier{}).runNpmInstall(context.Background(), appDir, npmPath)
	if err != nil {
		t.Fatalf("runNpmInstall failed: %v\n%s", err, out)
	}
	argsRaw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if args := strings.TrimSpace(string(argsRaw)); !strings.Contains(args, "ci --include=dev") {
		t.Fatalf("expected npm ci to include dev dependencies, got args %q", args)
	}
}

func TestViteBinaryRequiresLocalProjectInstall(t *testing.T) {
	root := t.TempDir()
	rv := &RuntimeVerifier{}

	if got, err := rv.viteBinary(root); err == nil || got != "" {
		t.Fatalf("expected missing local vite binary to fail, got path=%q err=%v", got, err)
	}

	local := filepath.Join(root, "node_modules", ".bin", "vite")
	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := rv.viteBinary(root)
	if err != nil {
		t.Fatalf("expected local vite binary to be accepted: %v", err)
	}
	if got != local {
		t.Fatalf("expected local vite path %q, got %q", local, got)
	}
}

func TestApplyAdvisoryBrowserSignalsAddsVisionAndCanaryMetadata(t *testing.T) {
	rv := &RuntimeVerifier{
		visionVerifier: &stubRuntimeVisionVerifier{
			result: &VisionRepairResult{
				Summary:     "UI loads but the CTA hierarchy is weak",
				RepairHints: []string{"Increase contrast on the primary CTA"},
			},
		},
		canary: &stubRuntimeCanaryTester{
			available: true,
			result: &CanaryResult{
				Clicked:                        3,
				VisibleControls:                5,
				PostInteractionVisibleControls: 4,
				PostInteractionChecked:         true,
				PostInteractionHealthy:         true,
				Errors:                         []string{"TypeError: Cannot read properties of undefined (reading 'map')"},
				RepairHints:                    []string{"Fix the click handler that assumes async data is already loaded"},
			},
		},
	}
	result := &RuntimeVerificationResult{}
	br := &BrowserPageLoadResult{
		Passed:         true,
		ScreenshotData: []byte{0x89, 0x50, 0x4e, 0x47},
	}

	rv.applyAdvisoryBrowserSignals(context.Background(), result, "http://127.0.0.1:5173", br)

	if len(result.ScreenshotData) == 0 {
		t.Fatal("expected screenshot data to be preserved on the runtime result")
	}
	if result.CanaryClickCount != 3 {
		t.Fatalf("expected canary click count 3, got %d", result.CanaryClickCount)
	}
	if len(result.CanaryErrors) != 1 {
		t.Fatalf("expected 1 canary error, got %v", result.CanaryErrors)
	}
	if result.CanaryVisibleControls != 5 {
		t.Fatalf("expected canary visible control count 5, got %d", result.CanaryVisibleControls)
	}
	if result.CanaryPostInteractionVisible != 4 {
		t.Fatalf("expected post-interaction visible control count 4, got %d", result.CanaryPostInteractionVisible)
	}
	if !result.CanaryPostInteractionChecked || !result.CanaryPostInteractionHealthy {
		t.Fatalf("expected canary post-interaction health metadata to be preserved, got %+v", result)
	}
	if !containsStringWithPrefix(result.RepairHints, "visual:") {
		t.Fatalf("expected visual repair hint prefix, got %v", result.RepairHints)
	}
	if !containsStringWithPrefix(result.RepairHints, "interaction:") {
		t.Fatalf("expected interaction repair hint prefix, got %v", result.RepairHints)
	}
	if !containsStringWithPrefix(result.CanaryErrors, "interaction:") {
		t.Fatalf("expected interaction canary error prefix, got %v", result.CanaryErrors)
	}
	if len(result.Checks) != 2 {
		t.Fatalf("expected 2 advisory checks (vision + canary), got %#v", result.Checks)
	}
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(needle) {
			return true
		}
	}
	return false
}

func containsStringWithPrefix(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), strings.ToLower(strings.TrimSpace(prefix))) {
			return true
		}
	}
	return false
}

func TestSanitizeRuntimeVerifyPath(t *testing.T) {
	tests := []struct {
		path string
		ok   bool
		want string
	}{
		{path: "src/main.tsx", ok: true, want: filepath.Clean("src/main.tsx")},
		{path: "index.html", ok: true, want: "index.html"},
		{path: "/etc/passwd", ok: false},
		{path: "../escape.js", ok: false},
		{path: "", ok: false},
	}

	for _, tc := range tests {
		got, ok := sanitizeRuntimeVerifyPath(tc.path)
		if ok != tc.ok {
			t.Fatalf("path %q: expected ok=%v, got %v", tc.path, tc.ok, ok)
		}
		if tc.ok && got != tc.want {
			t.Fatalf("path %q: expected %q, got %q", tc.path, tc.want, got)
		}
	}
}

func TestPrepareWorkDirRejectsEscapingPaths(t *testing.T) {
	rv := NewRuntimeVerifier()
	dir, cleanup, err := rv.prepareWorkDir([]VerifiableFile{
		{Path: "/etc/passwd", Content: "nope"},
		{Path: "src/main.tsx", Content: "console.log('ok')"},
	})
	defer cleanup()
	if err != nil {
		t.Fatalf("expected prepareWorkDir to ignore unsafe path and succeed, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "src", "main.tsx")); statErr != nil {
		t.Fatalf("expected safe file to exist: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "etc", "passwd")); !os.IsNotExist(statErr) {
		t.Fatalf("expected unsafe absolute path to be ignored inside temp dir, got %v", statErr)
	}
}
