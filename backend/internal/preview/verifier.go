// Package preview - Preview Verifier
// Deterministic checks that confirm generated output would produce a loadable
// interactive preview before a build is declared complete.
package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// VerifiableFile is a minimal file representation used for verification.
// Mirrors agents.GeneratedFile without creating a circular import.
type VerifiableFile struct {
	Path    string
	Content string
}

// VerificationResult holds the outcome of a preview verification run.
type VerificationResult struct {
	Passed      bool
	Checks      []CheckResult
	FailureKind string   // "missing_entrypoint", "blank_screen", "corrupt_content", "invalid_html", "invalid_package_json", "backend_missing", "backend_no_listen", "backend_no_routes"
	RepairHints []string // Actionable repair directives for the agent
	Details     string   // Human-readable failure description
	Duration    time.Duration
}

// CheckResult records the outcome of a single named check.
type CheckResult struct {
	Name   string
	Passed bool
	Detail string
}

// Verifier runs deterministic preview readiness checks against a set of
// generated files. Static checks are always performed. When runtimeVerifier
// is non-nil, Vite/React projects additionally get a live dev-server boot
// and HTTP asset check (the runtime proof layer).
type Verifier struct {
	// serverRunner is retained for future full-stack process-boot checks.
	// Currently unused; full-stack verification is static-only.
	serverRunner    *ServerRunner
	runtimeVerifier *RuntimeVerifier // nil = runtime check disabled
}

// NewVerifier returns a Verifier. serverRunner may be nil.
// Runtime verification is disabled; use NewVerifierWithRuntime to enable it.
func NewVerifier(serverRunner *ServerRunner) *Verifier {
	return &Verifier{serverRunner: serverRunner}
}

// NewVerifierWithRuntime returns a Verifier with runtime boot verification
// enabled for Vite/React projects. Includes headless Chrome browser proof
// when Chrome is available (adds ~30-120 s to finalization).
func NewVerifierWithRuntime(serverRunner *ServerRunner) *Verifier {
	return &Verifier{
		serverRunner:    serverRunner,
		runtimeVerifier: NewRuntimeVerifierWithBrowser(),
	}
}

// VerifyFiles runs all applicable checks against the provided file set.
// isFullStack enables additional backend-surface checks.
func (v *Verifier) VerifyFiles(ctx context.Context, files []VerifiableFile, isFullStack bool) *VerificationResult {
	start := time.Now()
	res := &VerificationResult{Passed: true}

	fileMap := buildFileMap(files)

	// ── 1. Entrypoint discovery ─────────────────────────────────────────
	htmlEntry := findHTMLEntrypoint(fileMap)
	jsEntry := findJSEntrypoint(fileMap)

	if htmlEntry == "" && jsEntry == "" {
		return res.fail("missing_entrypoint",
			"No recognized frontend entry point found (index.html, src/main.tsx, src/index.tsx, etc.).",
			"Generate the missing entry point file for this frontend application.",
			check("find_entrypoint", false, "no index.html or src/main.{tsx,ts,jsx,js} found"),
		)
	}
	res.addCheck(check("find_entrypoint", true, fmt.Sprintf("found: %s", coalesce(htmlEntry, jsEntry))))

	// ── 2. Entrypoint non-blank ─────────────────────────────────────────
	entryContent := fileMap[coalesce(htmlEntry, jsEntry)]
	if len(strings.TrimSpace(entryContent)) < 50 {
		return res.fail("blank_screen",
			fmt.Sprintf("Entry point %q is effectively empty (%d bytes).", coalesce(htmlEntry, jsEntry), len(strings.TrimSpace(entryContent))),
			fmt.Sprintf("Complete the empty entry point file %q with the application code.", coalesce(htmlEntry, jsEntry)),
			check("entrypoint_non_blank", false, "content < 50 bytes"),
		)
	}
	res.addCheck(check("entrypoint_non_blank", true, ""))

	// ── 3. No markdown code-fence artifacts ─────────────────────────────
	// HTML files must never contain backtick fences (any count is corrupt).
	// JS/TS files: only flag unmatched (odd-count) fences.
	if htmlEntry != "" {
		if strings.Contains(fileMap[htmlEntry], "```") {
			return res.fail("corrupt_content",
				fmt.Sprintf("%q contains markdown code fences (```) — the file must be pure HTML, not markdown.", htmlEntry),
				fmt.Sprintf("Remove all markdown code fences (```) from %q and ensure it is valid HTML.", htmlEntry),
				check("no_markdown_fences", false, fmt.Sprintf("%s: contains ``` artifacts", htmlEntry)),
			)
		}
	}
	if jsEntry != "" {
		content := fileMap[jsEntry]
		if count := strings.Count(content, "```"); count%2 != 0 {
			return res.fail("corrupt_content",
				fmt.Sprintf("%q contains unmatched markdown code fences (``` count: %d).", jsEntry, count),
				fmt.Sprintf("Remove all markdown code fences (```) from %q — the file must contain only valid source code.", jsEntry),
				check("no_markdown_fences", false, fmt.Sprintf("%s: %d unmatched fences", jsEntry, count)),
			)
		}
	}
	res.addCheck(check("no_markdown_fences", true, ""))

	// ── 4. HTML structure check ─────────────────────────────────────────
	if htmlEntry != "" {
		html := fileMap[htmlEntry]
		lower := strings.ToLower(html)
		if !strings.Contains(lower, "<html") && !strings.Contains(lower, "<!doctype") {
			return res.fail("invalid_html",
				fmt.Sprintf("%q is missing a valid <html> or <!DOCTYPE> declaration.", htmlEntry),
				fmt.Sprintf("Fix %q: add a proper <!DOCTYPE html><html> wrapper with <head> and <body> content.", htmlEntry),
				check("html_structure", false, "missing <html> or <!doctype>"),
			)
		}
		bodyStart := strings.Index(lower, "<body")
		bodyEnd := strings.Index(lower, "</body>")
		if bodyStart < 0 || bodyEnd < 0 || bodyEnd-bodyStart < 20 {
			return res.fail("blank_screen",
				fmt.Sprintf("%q has an empty or missing <body> element.", htmlEntry),
				fmt.Sprintf("Add content inside the <body> of %q — either a root <div id='root'> or direct HTML content.", htmlEntry),
				check("html_body_content", false, "empty <body>"),
			)
		}
		res.addCheck(check("html_structure", true, ""))
		res.addCheck(check("html_body_content", true, ""))
	}

	// ── 5. JS/TS entrypoint render/mount call ───────────────────────────
	if jsEntry != "" {
		jsContent := fileMap[jsEntry]
		if !hasRenderCall(jsContent) {
			return res.fail("missing_entrypoint",
				fmt.Sprintf("%q does not contain a recognized render or mount call.", jsEntry),
				fmt.Sprintf("Add a ReactDOM.createRoot/render, createApp, or mount call to %q to attach the app to the DOM.", jsEntry),
				check("render_call_present", false, "no createRoot/render/mount/createApp found"),
			)
		}
		res.addCheck(check("render_call_present", true, ""))
	}

	// ── 6. Root component existence (SPA) ───────────────────────────────
	if jsEntry != "" {
		rootComponent := findRootComponent(fileMap)
		if rootComponent == "" {
			return res.fail("missing_entrypoint",
				"No root App component found (App.tsx, App.jsx, App.vue, App.svelte).",
				"Create the root App component (e.g. src/App.tsx) and export it as the default export.",
				check("root_component_exists", false, "no App.{tsx,jsx,vue,svelte} found"),
			)
		}
		appContent := fileMap[rootComponent]
		if len(strings.TrimSpace(appContent)) < 30 {
			return res.fail("blank_screen",
				fmt.Sprintf("Root component %q is effectively empty.", rootComponent),
				fmt.Sprintf("Complete %q — it must export a valid React/Vue component with renderable JSX/template content.", rootComponent),
				check("root_component_non_blank", false, "component < 30 bytes"),
			)
		}
		res.addCheck(check("root_component_exists", true, rootComponent))
	}

	// ── 7. package.json sanity (if present) ─────────────────────────────
	if pkgContent, ok := fileMap["package.json"]; ok {
		if err := checkPackageJSON(pkgContent); err != nil {
			return res.fail("invalid_package_json",
				fmt.Sprintf("package.json is malformed or missing required fields: %v", err),
				fmt.Sprintf("Fix package.json: %v. Ensure it has a valid 'scripts' section with a 'dev' or 'start' command.", err),
				check("package_json_valid", false, err.Error()),
			)
		}
		res.addCheck(check("package_json_valid", true, ""))
	}

	// ── 8. Bundler entry resolution ─────────────────────────────────────
	if bundlerConfig, bundlerEntry := detectBundlerEntry(fileMap); bundlerConfig != "" {
		if bundlerEntry != "" {
			if _, exists := fileMap[bundlerEntry]; !exists {
				return res.fail("missing_entrypoint",
					fmt.Sprintf("Bundler config %q references entry %q which does not exist in generated files.", bundlerConfig, bundlerEntry),
					fmt.Sprintf("Create the file %q referenced as the bundler entry point in %q.", bundlerEntry, bundlerConfig),
					check("bundler_entry_resolves", false, fmt.Sprintf("%s -> %s (missing)", bundlerConfig, bundlerEntry)),
				)
			}
			res.addCheck(check("bundler_entry_resolves", true, fmt.Sprintf("%s -> %s", bundlerConfig, bundlerEntry)))
		}
	}

	// ── 10. Full-stack backend checks ─── (moved up, before HTTP boot) ──
	if isFullStack {
		if checkResult := v.verifyBackendStatic(fileMap); checkResult != nil {
			return checkResult
		}
	}

	// ── 9. HTTP boot check (vanilla HTML only, no bundler, not full-stack) ──
	isVanilla := htmlEntry != "" && !hasBundlerConfig(fileMap) && jsEntry == "" && !isFullStack
	if isVanilla && ctx.Err() == nil {
		if bootErr := v.httpBootCheck(ctx, files, htmlEntry); bootErr != nil {
			return res.fail("boot_failed",
				fmt.Sprintf("Preview HTTP boot check failed: %v", bootErr),
				"Ensure index.html references only files that exist in the project, uses relative paths, and has no broken <script> or <link> tags.",
				check("http_boot", false, bootErr.Error()),
			)
		}
		res.addCheck(check("http_boot", true, "FileServer 200 OK"))
	}

	// ── Runtime boot proof (Vite/React) ─────────────────────────────────
	// Only runs when a RuntimeVerifier is wired and the project is Vite-based.
	// Proves the dev server boots, all assets serve, and the mount point exists.
	if v.runtimeVerifier != nil && isViteProject(fileMap) && ctx.Err() == nil {
		rr := v.runtimeVerifier.VerifyViteApp(ctx, files)
		if rr != nil && !rr.Skipped {
			if rr.Passed {
				res.addCheck(check("vite_runtime_boot", true, fmt.Sprintf("dev server booted, HTTP checks passed in %s", rr.Duration.Round(time.Millisecond))))
			} else {
				hints := rr.RepairHints
				if len(hints) == 0 {
					hints = []string{"Fix the Vite/React app so the dev server boots and serves all assets correctly."}
				}
				return res.fail(rr.FailureKind, rr.Details, hints[0],
					check("vite_runtime_boot", false, rr.Details))
			}
		}
	}

	res.Duration = time.Since(start)
	return res
}

// verifyBackendStatic performs static checks on the backend entry file.
func (v *Verifier) verifyBackendStatic(fileMap map[string]string) *VerificationResult {
	beEntry := findBackendEntry(fileMap)
	if beEntry == "" {
		res := &VerificationResult{Passed: false}
		return res.fail("backend_missing",
			"No backend server entry file found (server.js, app.js, main.go, main.py, index.js with framework usage).",
			"Create the backend server entry file (e.g. server.js for Express, main.go for Go, main.py for FastAPI).",
			check("backend_entry_exists", false, "no server entry found"),
		)
	}

	content := fileMap[beEntry]
	if len(strings.TrimSpace(content)) < 50 {
		res := &VerificationResult{Passed: false}
		return res.fail("blank_screen",
			fmt.Sprintf("Backend entry %q is effectively empty.", beEntry),
			fmt.Sprintf("Complete %q — it must contain the full server setup and listen/bind call.", beEntry),
			check("backend_entry_non_blank", false, "< 50 bytes"),
		)
	}

	if !hasListenCall(content, beEntry) {
		res := &VerificationResult{Passed: false}
		return res.fail("backend_no_listen",
			fmt.Sprintf("Backend entry %q has no server start/listen call.", beEntry),
			fmt.Sprintf("Add a server.listen(), ListenAndServe(), or uvicorn.run() call to %q.", beEntry),
			check("backend_has_listen", false, "no listen/serve call found"),
		)
	}

	if !backendHasRouteDefinitions(fileMap, beEntry) {
		res := &VerificationResult{Passed: false}
		return res.fail("backend_no_routes",
			fmt.Sprintf("Backend entry %q defines no routes.", beEntry),
			fmt.Sprintf("Define or mount at least one GET or POST route handler in %q or an imported backend route module.", beEntry),
			check("backend_has_routes", false, "no route definitions found"),
		)
	}

	return nil // all backend static checks passed
}

// httpBootCheck writes files to a temp dir, serves them with a stdlib FileServer,
// and verifies the root path returns a 200 with HTML content.
// Used only for vanilla HTML projects (no bundler).
func (v *Verifier) httpBootCheck(ctx context.Context, files []VerifiableFile, htmlEntry string) error {
	dir, err := os.MkdirTemp("", "apex-preview-verify-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	// Write files to temp dir
	for _, f := range files {
		if strings.TrimSpace(f.Path) == "" || strings.TrimSpace(f.Content) == "" {
			continue
		}
		dest := filepath.Join(dir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			continue
		}
		if err := os.WriteFile(dest, []byte(f.Content), 0o644); err != nil {
			continue
		}
	}

	// Find a free port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("bind port: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(dir)))
	srv := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port), Handler: mux}

	// Start server in goroutine
	go srv.ListenAndServe() //nolint:errcheck

	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(shutCtx) //nolint:errcheck
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Determine request path from entry (index.html at root → "/")
	reqPath := "/"
	if htmlEntry != "index.html" && htmlEntry != "public/index.html" {
		reqPath = "/" + strings.TrimPrefix(htmlEntry, "/")
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d%s", port, reqPath), nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", reqPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	bodyStr := strings.ToLower(strings.TrimSpace(string(body)))
	if len(bodyStr) < 100 {
		return fmt.Errorf("response body too small (%d bytes) — likely a blank screen", len(bodyStr))
	}
	if !strings.Contains(bodyStr, "<html") && !strings.Contains(bodyStr, "<!doctype") {
		return fmt.Errorf("response body does not contain valid HTML")
	}

	return nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func buildFileMap(files []VerifiableFile) map[string]string {
	m := make(map[string]string, len(files))
	for _, f := range files {
		if f.Path != "" {
			m[f.Path] = f.Content
		}
	}
	return m
}

func findHTMLEntrypoint(fileMap map[string]string) string {
	candidates := []string{
		"index.html",
		"public/index.html",
		"src/index.html",
		"static/index.html",
		"dist/index.html",
	}
	for _, c := range candidates {
		if _, ok := fileMap[c]; ok {
			return c
		}
	}
	return ""
}

func findJSEntrypoint(fileMap map[string]string) string {
	candidates := []string{
		"src/main.tsx", "src/main.ts", "src/main.jsx", "src/main.js",
		"src/index.tsx", "src/index.ts", "src/index.jsx", "src/index.js",
		"pages/index.tsx", "pages/index.jsx",
		"app/page.tsx", "app/page.jsx",
		"index.tsx", "index.ts",
	}
	for _, c := range candidates {
		if _, ok := fileMap[c]; ok {
			return c
		}
	}
	return ""
}

func findRootComponent(fileMap map[string]string) string {
	candidates := []string{
		"src/App.tsx", "src/App.jsx", "src/App.vue", "src/App.svelte",
		"src/app.tsx", "src/app.jsx",
		"App.tsx", "App.jsx",
	}
	for _, c := range candidates {
		if _, ok := fileMap[c]; ok {
			return c
		}
	}
	return ""
}

func findBackendEntry(fileMap map[string]string) string {
	candidates := []string{
		"server.js", "server.ts",
		"server/index.js", "server/index.ts",
		"server/main.js", "server/main.ts",
		"app.js", "app.ts",
		"index.js", "index.ts",
		"src/server.js", "src/server.ts",
		"src/app.js", "src/app.ts",
		"src/index.js", "src/index.ts",
		"backend/index.js", "backend/index.ts",
		"backend/server.js", "backend/server.ts",
		"api/index.js", "api/index.ts",
		"api/server.js", "api/server.ts",
		"apps/api/src/index.js", "apps/api/src/index.ts",
		"apps/api/src/server.js", "apps/api/src/server.ts",
		"main.go",
		"main.py", "app.py",
		"server.py",
	}
	for _, c := range candidates {
		content, ok := fileMap[c]
		if !ok {
			continue
		}
		// Confirm it looks like a backend file (has a framework/listen marker)
		if hasListenCall(content, c) || hasServerFrameworkImport(content, c) {
			return c
		}
	}
	return ""
}

func hasRenderCall(content string) bool {
	patterns := []string{
		"createRoot(", "ReactDOM.render(", "render(<",
		"createApp(", "mount(", ".mount(\"",
		"hydrate(", "hydrateRoot(",
	}
	for _, p := range patterns {
		if strings.Contains(content, p) {
			return true
		}
	}
	return false
}

func hasListenCall(content, path string) bool {
	lower := strings.ToLower(content)
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return strings.Contains(lower, "listenandserve") || strings.Contains(lower, "listen(")
	case ".py":
		return strings.Contains(lower, "uvicorn") || strings.Contains(lower, "app.run(") ||
			strings.Contains(lower, "serve(") || strings.Contains(lower, "listen(")
	default: // .js, .ts
		return strings.Contains(lower, ".listen(") || strings.Contains(lower, "server.listen(") ||
			strings.Contains(lower, "app.listen(") || strings.Contains(lower, "listen(port")
	}
}

func hasRouteDefinition(content, path string) bool {
	lower := strings.ToLower(content)
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return strings.Contains(lower, "handlefunc") || strings.Contains(lower, ".get(") ||
			strings.Contains(lower, ".post(") || strings.Contains(lower, "router.")
	case ".py":
		return strings.Contains(lower, "@app.") || strings.Contains(lower, "@router.") ||
			strings.Contains(lower, "add_route") || strings.Contains(lower, "route(")
	default: // .js, .ts
		return strings.Contains(lower, ".get(") || strings.Contains(lower, ".post(") ||
			strings.Contains(lower, "router.") || strings.Contains(lower, "app.get") ||
			strings.Contains(lower, "app.post")
	}
}

func backendHasRouteDefinitions(fileMap map[string]string, entryPath string) bool {
	if content, ok := fileMap[entryPath]; ok && hasRouteDefinition(content, entryPath) {
		return true
	}
	for path, content := range fileMap {
		if path == entryPath || !isBackendRouteCandidate(path) {
			continue
		}
		if hasRouteDefinition(content, path) {
			return true
		}
	}
	return false
}

func isBackendRouteCandidate(path string) bool {
	clean := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(path, "\\", "/")))
	if clean == "" {
		return false
	}
	if strings.Contains(clean, "/__tests__/") || strings.Contains(clean, "/tests/") || strings.Contains(clean, "/test/") {
		return false
	}
	base := filepath.Base(clean)
	if strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") || strings.HasSuffix(base, "_test.go") {
		return false
	}
	ext := filepath.Ext(clean)
	switch ext {
	case ".js", ".jsx", ".ts", ".tsx", ".go", ".py":
	default:
		return false
	}

	backendPrefixes := []string{
		"server/",
		"backend/",
		"api/",
		"src/server/",
		"src/api/",
		"apps/api/",
	}
	for _, prefix := range backendPrefixes {
		if strings.HasPrefix(clean, prefix) {
			return true
		}
	}

	switch clean {
	case "server.js", "server.ts", "app.js", "app.ts", "index.js", "index.ts", "main.go", "main.py", "app.py", "server.py":
		return true
	}
	return false
}

func hasServerFrameworkImport(content, path string) bool {
	lower := strings.ToLower(content)
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return strings.Contains(lower, "net/http") || strings.Contains(lower, "gin") ||
			strings.Contains(lower, "fiber") || strings.Contains(lower, "echo")
	case ".py":
		return strings.Contains(lower, "fastapi") || strings.Contains(lower, "flask") ||
			strings.Contains(lower, "django") || strings.Contains(lower, "starlette")
	default:
		return strings.Contains(lower, "express") || strings.Contains(lower, "fastify") ||
			strings.Contains(lower, "hapi") || strings.Contains(lower, "koa")
	}
}

func hasBundlerConfig(fileMap map[string]string) bool {
	bundlerConfigs := []string{
		"vite.config.ts", "vite.config.js", "vite.config.mjs",
		"webpack.config.js", "webpack.config.ts",
		"rollup.config.js", "rollup.config.ts",
		"parcel.config.json",
		"next.config.js", "next.config.ts", "next.config.mjs",
		"nuxt.config.ts", "nuxt.config.js",
	}
	for _, c := range bundlerConfigs {
		if _, ok := fileMap[c]; ok {
			return true
		}
	}
	return false
}

// detectBundlerEntry returns the config file name and the entry point it references,
// or empty strings if no bundler config is present or no entry is parseable.
func detectBundlerEntry(fileMap map[string]string) (configFile, entryPath string) {
	if content, ok := fileMap["vite.config.ts"]; ok {
		return "vite.config.ts", extractViteEntry(content)
	}
	if content, ok := fileMap["vite.config.js"]; ok {
		return "vite.config.js", extractViteEntry(content)
	}
	return "", ""
}

func extractViteEntry(content string) string {
	// Look for input: 'path' or input: "path" patterns
	patterns := []string{"index.html", "src/main.tsx", "src/main.ts", "src/main.jsx", "src/main.js"}
	for _, p := range patterns {
		if strings.Contains(content, p) {
			return p
		}
	}
	return ""
}

func checkPackageJSON(content string) error {
	var pkg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Must have a name
	if _, ok := pkg["name"]; !ok {
		return fmt.Errorf("missing required field 'name'")
	}

	// Must have scripts with at least one runnable command
	if raw, ok := pkg["scripts"]; ok {
		var scripts map[string]string
		if err := json.Unmarshal(raw, &scripts); err != nil {
			return fmt.Errorf("'scripts' is not a valid object")
		}
		if len(scripts) == 0 {
			return fmt.Errorf("'scripts' section is empty — add a 'dev' or 'start' command")
		}
	} else {
		return fmt.Errorf("missing 'scripts' section")
	}

	return nil
}

func isViteProject(fileMap map[string]string) bool {
	viteConfigs := []string{"vite.config.ts", "vite.config.js", "vite.config.mjs"}
	for _, c := range viteConfigs {
		if _, ok := fileMap[c]; ok {
			return true
		}
	}
	if pkgContent, ok := fileMap["package.json"]; ok {
		return strings.Contains(pkgContent, `"vite"`) || strings.Contains(pkgContent, `"@vitejs`)
	}
	return false
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func check(name string, passed bool, detail string) CheckResult {
	return CheckResult{Name: name, Passed: passed, Detail: detail}
}

// fail marks the result as failed and returns it.
func (r *VerificationResult) fail(kind, details, repairHint string, checks ...CheckResult) *VerificationResult {
	r.Passed = false
	r.FailureKind = kind
	r.Details = details
	if repairHint != "" {
		r.RepairHints = append(r.RepairHints, repairHint)
	}
	for _, c := range checks {
		r.addCheck(c)
	}
	return r
}

func (r *VerificationResult) addCheck(c CheckResult) {
	r.Checks = append(r.Checks, c)
}
