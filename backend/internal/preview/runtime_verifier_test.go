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
	"testing"
	"time"
)

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

	return httptest.NewServer(mux)
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

func TestCheckRootPage_BlankBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	rv := &RuntimeVerifier{}
	_, c := rv.checkRootPage(context.Background(), &http.Client{}, srv.URL)
	if c.Passed {
		t.Error("expected 404 to fail root page check")
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
