// runtime_verifier.go — Runtime boot verification for generated Vite/React apps.
// Proves that the generated app can actually serve and load, not just pass static checks.
//
// Flow:
//  1. Write generated files to a temp directory.
//  2. Run npm install (bounded timeout).
//  3. Start the Vite dev server on an ephemeral port.
//  4. Run deterministic HTTP checks: root page, entry module, Vite client, CSS assets.
//  5. Kill the server and remove the temp dir.
//
// The check is intentionally HTTP-only (no headless browser) to keep it safe for
// untrusted generated code and deployable without Chrome on the server. It detects:
//   - preview server boot failure
//   - missing/404 JS or CSS assets
//   - Vite transform errors in the entry module (HTTP 500 + error body)
//   - missing DOM mount point (<div id="root"> / <div id="app">)
//   - blank/empty HTML body
//   - broken client-side entry route (entry script not served)
package preview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// RuntimeVerifier performs actual dev-server boot, HTTP-level checks, and
// (when a BrowserVerifier is wired in) headless browser page-load proof
// against generated Vite/React applications.
type runtimeVisionVerifier interface {
	AnalyzeScreenshot(ctx context.Context, imageData []byte, description string) *VisionRepairResult
}

type runtimeCanaryTester interface {
	Available() bool
	RunCanaryInteractions(ctx context.Context, pageURL string) *CanaryResult
}

type RuntimeVerifier struct {
	browser        *BrowserVerifier      // nil = browser proof disabled
	visionVerifier runtimeVisionVerifier // nil = screenshot vision review disabled
	canary         runtimeCanaryTester   // nil = interaction canary disabled
	totalTimeout   time.Duration
	installTimeout time.Duration
	readyTimeout   time.Duration
}

// NewRuntimeVerifier creates a RuntimeVerifier with HTTP checks only.
func NewRuntimeVerifier() *RuntimeVerifier { return &RuntimeVerifier{} }

// NewRuntimeVerifierWithBrowser creates a RuntimeVerifier that adds headless
// Chrome page-load proof after HTTP checks pass. Chrome is auto-detected; if
// unavailable, verification fails honestly instead of silently downgrading.
func NewRuntimeVerifierWithBrowser() *RuntimeVerifier {
	return &RuntimeVerifier{
		browser:        NewBrowserVerifier(),
		visionVerifier: NewVisionVerifierFromEnv(),
		canary:         NewCanaryTester(),
	}
}

// RuntimeVerificationResult is returned by VerifyViteApp.
type RuntimeVerificationResult struct {
	Passed                       bool
	Checks                       []CheckResult
	FailureKind                  string
	RepairHints                  []string
	Details                      string
	Duration                     time.Duration
	Skipped                      bool   // true when prerequisites (npm/node) are absent
	ServerLogs                   string // truncated Vite stderr for debugging
	ScreenshotData               []byte
	CanaryErrors                 []string
	CanaryClickCount             int
	CanaryVisibleControls        int
	CanaryPostInteractionVisible int
	CanaryPostInteractionChecked bool
	CanaryPostInteractionHealthy bool
	VisionSeverity               string // "critical", "advisory", "clean", or "" when vision skipped
}

// ── Public entry point ────────────────────────────────────────────────────────

// VerifyViteApp proves the generated app boots and serves correctly.
// Total bounded timeout is 150 s (180 s when headless browser proof is enabled).
// Returns Skipped=true when npm/vite are absent.
func (rv *RuntimeVerifier) VerifyViteApp(ctx context.Context, files []VerifiableFile) *RuntimeVerificationResult {
	start := time.Now()
	totalTimeout := rv.runtimeTotalTimeout()
	bootCtx, bootCancel := context.WithTimeout(ctx, totalTimeout)
	defer bootCancel()

	if rv.browser != nil && !rv.browser.Available() {
		return rv.rtFail("browser_unavailable",
			"runtime preview verification requires Chrome, but no Chrome/Chromium binary was found on PATH",
			"Install Chrome/Chromium in the backend runtime or disable APEX_PREVIEW_RUNTIME_VERIFY until browser proof is available.",
			start,
		)
	}

	npmPath, err := exec.LookPath("npm")
	if err != nil {
		return rv.rtFail("boot_failed", "runtime verification requires npm, but npm was not found on PATH",
			"Install npm in the backend runtime or disable APEX_PREVIEW_RUNTIME_VERIFY until the preview runtime dependencies are available.",
			start)
	}

	// ── 1. Write files to a temp directory ──────────────────────────────
	dir, cleanup, writeErr := rv.prepareWorkDir(files)
	defer cleanup()
	if writeErr != nil {
		return rv.rtFail("boot_failed", "failed to create temp workspace: "+writeErr.Error(),
			"Ensure all generated files have valid paths.", start)
	}

	// ── 2. npm install ───────────────────────────────────────────────────
	installTimeout := rv.runtimeInstallTimeout(totalTimeout)
	installCtx, installCancel := context.WithTimeout(bootCtx, installTimeout)
	defer installCancel()

	installOut, installErr := rv.runNpmInstall(installCtx, dir, npmPath)
	if installErr != nil {
		if errors.Is(installCtx.Err(), context.DeadlineExceeded) {
			return rv.rtFail("boot_failed",
				fmt.Sprintf("runtime verification timed out during dependency install after %s", formatRuntimeTimeout(installTimeout)),
				"Reduce dependency weight or ensure the generated app has a minimal, installable package.json before preview verification runs.",
				start,
			)
		}
		detail := fmt.Sprintf("npm install failed: %v", installErr)
		if trimmed := strings.TrimSpace(truncateLog(string(installOut), 800)); trimmed != "" {
			detail = fmt.Sprintf("%s — %s", detail, trimmed)
		}
		return rv.rtFail("boot_failed", detail,
			"Fix dependency errors in package.json — check for invalid version ranges or missing packages.",
			start,
		)
	}

	// ── 3. Allocate port ────────────────────────────────────────────────
	port, portErr := rv.freePort()
	if portErr != nil {
		return rv.rtFail("boot_failed", "could not allocate ephemeral port: "+portErr.Error(), "", start)
	}

	// ── 4. Start Vite dev server ─────────────────────────────────────────
	viteBin := rv.viteBinary(dir)
	viteCmd, viteLogs, viteErr := rv.startVite(bootCtx, dir, viteBin, port)
	if viteErr != nil {
		return rv.rtFail("boot_failed",
			fmt.Sprintf("failed to start Vite process: %v", viteErr),
			"Check vite.config.ts/js for configuration errors. Ensure vite is listed in package.json dependencies.",
			start,
		)
	}
	viteExited := false
	viteExitCh := make(chan error, 1)
	go func() {
		viteExitCh <- viteCmd.Wait()
	}()
	defer func() {
		if !viteExited && viteCmd.Process != nil {
			viteCmd.Process.Kill() //nolint:errcheck
		}
		if !viteExited {
			<-viteExitCh //nolint:errcheck
		}
	}()

	// ── 5. Wait for port ready ───────────────────────────────────────────
	stop := make(chan struct{})
	defer close(stop)
	readyTimeout := rv.runtimeServerReadyTimeout(totalTimeout, installTimeout)
	if deadline, ok := bootCtx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < readyTimeout {
			readyTimeout = remaining
		}
	}
	if ready, exited, exitErr := waitForTCPPortOrExit(port, readyTimeout, stop, viteExitCh); !ready {
		serverLogs := truncateLog(viteLogs.String(), 1500)
		if exited {
			viteExited = true
			detail := "Vite dev server exited before becoming ready"
			if exitErr != nil {
				detail = fmt.Sprintf("%s: %v", detail, exitErr)
			}
			if trimmed := strings.TrimSpace(serverLogs); trimmed != "" {
				detail = fmt.Sprintf("%s — %s", detail, trimmed)
			}
			return rv.rtFailWithLogs("boot_failed",
				detail,
				"Fix the first Vite startup error in the server logs, usually an unresolved import, invalid config, or TypeScript dependency gap.",
				start, serverLogs,
			)
		}
		return rv.rtFailWithLogs("boot_failed",
			fmt.Sprintf("Vite dev server did not become ready on port %d within %s", port, formatRuntimeTimeout(readyTimeout)),
			"Check vite.config.ts for syntax errors. Ensure all imports resolve. Common cause: missing or misconfigured entry point.",
			start, serverLogs,
		)
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	httpClient := &http.Client{Timeout: 5 * time.Second}

	// ── 6. HTTP checks ───────────────────────────────────────────────────
	result := &RuntimeVerificationResult{Passed: true}

	// 6a. Root page
	htmlBody, rootCheck := rv.checkRootPage(bootCtx, httpClient, baseURL)
	result.Checks = append(result.Checks, rootCheck)
	if !rootCheck.Passed {
		logs := truncateLog(viteLogs.String(), 1500)
		return rv.rtFailWithLogs("blank_screen", rootCheck.Detail,
			"Ensure index.html has a non-empty <body> with the app mount point. Fix any errors reported by the Vite dev server.",
			start, logs)
	}

	// 6b. Mount point present in HTML
	mountCheck := rv.checkMountPoint(htmlBody)
	result.Checks = append(result.Checks, mountCheck)
	if !mountCheck.Passed {
		return rv.rtFail("missing_entrypoint",
			"Root page HTML has no app mount point (<div id=\"root\">, <div id=\"app\">, etc.).",
			"Add a <div id=\"root\"></div> inside the <body> of index.html as the React/Vue mount target.",
			start)
	}

	// 6c. Vite HMR client accessible
	viteClientCheck := rv.checkViteClient(bootCtx, httpClient, baseURL)
	result.Checks = append(result.Checks, viteClientCheck)
	if !viteClientCheck.Passed {
		return rv.rtFail("boot_failed", viteClientCheck.Detail,
			"Vite HMR client is not accessible. The Vite dev server may have started but is misconfigured.",
			start)
	}

	// 6d. Entry module served without transform errors
	entryScripts := parseScriptSrcs(htmlBody)
	if len(entryScripts) == 0 {
		// Fallback: check common entry paths
		entryScripts = []string{"/src/main.tsx", "/src/main.ts", "/src/main.jsx", "/src/main.js",
			"/src/index.tsx", "/src/index.ts"}
	}
	for _, src := range entryScripts {
		if src == "/@vite/client" || strings.HasPrefix(src, "http") {
			continue
		}
		entryCheck := rv.checkEntryModule(bootCtx, httpClient, baseURL, src)
		result.Checks = append(result.Checks, entryCheck)
		if !entryCheck.Passed {
			logs := truncateLog(viteLogs.String(), 1500)
			return rv.rtFailWithLogs("boot_failed", entryCheck.Detail,
				fmt.Sprintf("Fix the error in %s. Common causes: TypeScript syntax errors, missing imports, undefined exports.", src),
				start, logs)
		}
	}

	// 6e. CSS assets accessible
	cssLinks := parseCSSLinks(htmlBody)
	for _, href := range cssLinks {
		if strings.HasPrefix(href, "http") {
			continue
		}
		cssCheck := rv.checkAsset(bootCtx, httpClient, baseURL, href, "css")
		result.Checks = append(result.Checks, cssCheck)
		if !cssCheck.Passed {
			return rv.rtFail("boot_failed", cssCheck.Detail,
				fmt.Sprintf("Ensure %s exists and is a valid CSS file.", href),
				start)
		}
	}

	// ── 7. Browser page-load proof (headless Chrome) ────────────────────────
	if rv.browser != nil {
		br := rv.browser.VerifyPageLoad(bootCtx, baseURL)
		if br.Skipped {
			return rv.rtFail("browser_unavailable",
				"browser page-load proof skipped because Chrome became unavailable during verification",
				"Ensure Chrome/Chromium is installed and stable in the backend runtime before enabling APEX_PREVIEW_RUNTIME_VERIFY.",
				start,
			)
		}
		if !br.Passed {
			serverLogs := truncateLog(viteLogs.String(), 1500)
			hint := ""
			if len(br.RepairHints) > 0 {
				hint = br.RepairHints[0]
			}
			failed := rv.rtFailWithLogs(br.FailureKind, br.Details, hint, start, serverLogs)
			failed.ScreenshotData = br.ScreenshotData
			failed.RepairHints = appendUniqueStrings(failed.RepairHints, br.RepairHints...)
			return failed
		} else {
			detail := fmt.Sprintf("mount rendered in browser (children: %d)", br.MountChildCount)
			if len(br.JSErrors) > 0 {
				detail += fmt.Sprintf("; %d non-fatal JS error(s)", len(br.JSErrors))
			}
			result.Checks = append(result.Checks, check("browser_page_load", true, detail))
			rv.applyAdvisoryBrowserSignals(bootCtx, result, baseURL, br)
		}
	}

	result.Duration = time.Since(start)
	result.ServerLogs = truncateLog(viteLogs.String(), 500)
	log.Printf("[runtime_verifier] Vite boot checks passed in %s", result.Duration.Round(time.Millisecond))
	return result
}

func (rv *RuntimeVerifier) applyAdvisoryBrowserSignals(ctx context.Context, result *RuntimeVerificationResult, baseURL string, br *BrowserPageLoadResult) {
	if result == nil || br == nil || !br.Passed {
		return
	}

	if len(br.ScreenshotData) > 0 {
		result.ScreenshotData = br.ScreenshotData
	}

	if rv.visionVerifier != nil && len(br.ScreenshotData) > 0 {
		visionCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		vision := rv.visionVerifier.AnalyzeScreenshot(visionCtx, br.ScreenshotData, "A generated app preview that already mounted successfully in the browser.")
		cancel()
		if vision != nil {
			result.RepairHints = appendUniqueStrings(result.RepairHints, prefixAll("visual:", vision.RepairHints)...)
			result.VisionSeverity = vision.Severity
			severity := firstNonEmptyString(vision.Severity, "advisory")
			detail := firstNonEmptyString(
				vision.Summary,
				fmt.Sprintf("%d visual issue(s) detected", len(vision.Issues)),
				"visual review completed",
			)
			if len(vision.Issues) > 0 || vision.Summary != "" {
				result.Checks = append(result.Checks, check("vision_review:"+severity, true, detail))
			}
		}
	}

	if rv.canary != nil && rv.canary.Available() {
		canaryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		canary := rv.canary.RunCanaryInteractions(canaryCtx, baseURL)
		cancel()
		if canary != nil && !canary.Skipped {
			result.CanaryClickCount = canary.Clicked
			result.CanaryVisibleControls = canary.VisibleControls
			result.CanaryPostInteractionVisible = canary.PostInteractionVisibleControls
			result.CanaryPostInteractionChecked = canary.PostInteractionChecked
			result.CanaryPostInteractionHealthy = canary.PostInteractionHealthy
			result.CanaryErrors = appendUniqueStrings(result.CanaryErrors, prefixAll("interaction:", canary.Errors)...)
			result.RepairHints = appendUniqueStrings(result.RepairHints, prefixAll("interaction:", canary.RepairHints)...)

			detail := fmt.Sprintf("clicked %d of %d visible interactive control(s)", canary.Clicked, canary.VisibleControls)
			if canary.PostInteractionChecked {
				if canary.PostInteractionHealthy {
					detail += fmt.Sprintf("; preview remained rendered after settle (%d control(s) still visible)", canary.PostInteractionVisibleControls)
				} else {
					detail += "; preview failed the post-click settle check"
				}
			}
			if len(canary.Errors) > 0 {
				detail += fmt.Sprintf("; advisory errors: %s", summarizeIssues(canary.Errors, 2))
			}
			result.Checks = append(result.Checks, check("canary_interactions", true, detail))
		}
	}
}

// ── Private helpers ────────────────────────────────────────────────────────────

func (rv *RuntimeVerifier) prepareWorkDir(files []VerifiableFile) (dir string, cleanup func(), err error) {
	dir, err = os.MkdirTemp("", "apex-rt-verify-*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup = func() { os.RemoveAll(dir) }

	for _, f := range files {
		cleanPath, ok := sanitizeRuntimeVerifyPath(f.Path)
		if !ok {
			continue
		}
		dest := filepath.Join(dir, cleanPath)
		rel, relErr := filepath.Rel(dir, dest)
		if relErr != nil || strings.HasPrefix(rel, "..") || rel == "." {
			return dir, cleanup, fmt.Errorf("unsafe destination path derived from %q", f.Path)
		}
		if err2 := os.MkdirAll(filepath.Dir(dest), 0o755); err2 != nil {
			return dir, cleanup, fmt.Errorf("mkdir %s: %w", filepath.Dir(dest), err2)
		}
		if writeErr := os.WriteFile(dest, []byte(f.Content), 0o644); writeErr != nil {
			return dir, cleanup, fmt.Errorf("write %s: %w", f.Path, writeErr)
		}
	}
	return dir, cleanup, nil
}

func (rv *RuntimeVerifier) runNpmInstall(ctx context.Context, dir, npmPath string) ([]byte, error) {
	args := []string{"install", "--prefer-offline", "--no-audit", "--no-fund", "--loglevel=error"}
	if _, err := os.Stat(filepath.Join(dir, "package-lock.json")); err == nil {
		args = []string{"ci", "--prefer-offline", "--no-audit", "--no-fund", "--loglevel=error"}
	}
	cmd := exec.CommandContext(ctx, npmPath, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func (rv *RuntimeVerifier) runtimeTotalTimeout() time.Duration {
	if rv.totalTimeout > 0 {
		return rv.totalTimeout
	}
	if rv.browser != nil && rv.browser.Available() {
		return 180 * time.Second
	}
	return 150 * time.Second
}

func (rv *RuntimeVerifier) runtimeInstallTimeout(total time.Duration) time.Duration {
	if rv.installTimeout > 0 {
		return rv.installTimeout
	}
	if total >= 180*time.Second {
		return 120 * time.Second
	}
	if total >= 150*time.Second {
		return 90 * time.Second
	}
	if total <= 30*time.Second {
		return total
	}
	return total - 30*time.Second
}

func (rv *RuntimeVerifier) runtimeServerReadyTimeout(total, install time.Duration) time.Duration {
	if rv.readyTimeout > 0 {
		return rv.readyTimeout
	}
	if seconds := strings.TrimSpace(os.Getenv("APEX_PREVIEW_SERVER_READY_TIMEOUT_SECONDS")); seconds != "" {
		if parsed, err := strconv.Atoi(seconds); err == nil && parsed > 0 {
			return time.Duration(parsed) * time.Second
		}
	}

	target := 45 * time.Second
	if total >= 180*time.Second {
		target = 60 * time.Second
	}

	remaining := total - install
	if remaining <= 0 {
		return 5 * time.Second
	}
	if remaining < target {
		if remaining < 5*time.Second {
			return 5 * time.Second
		}
		return remaining
	}
	return target
}

func formatRuntimeTimeout(timeout time.Duration) string {
	if timeout%time.Second == 0 {
		return fmt.Sprintf("%ds", int(timeout/time.Second))
	}
	return timeout.String()
}

func appendUniqueStrings(existing []string, values ...string) []string {
	if len(values) == 0 {
		return existing
	}
	seen := make(map[string]struct{}, len(existing))
	out := make([]string, 0, len(existing)+len(values))
	for _, item := range existing {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	for _, item := range values {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func compactNonEmptyStrings(values []string) []string {
	return appendUniqueStrings(nil, values...)
}

func prefixAll(prefix string, values []string) []string {
	if prefix == "" || len(values) == 0 {
		return compactNonEmptyStrings(values)
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, strings.TrimSpace(prefix+" "+trimmed))
	}
	return compactNonEmptyStrings(out)
}

func summarizeIssues(values []string, max int) string {
	values = compactNonEmptyStrings(values)
	if len(values) == 0 {
		return ""
	}
	if max <= 0 || len(values) <= max {
		return strings.Join(values, "; ")
	}
	return strings.Join(values[:max], "; ")
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (rv *RuntimeVerifier) viteBinary(dir string) string {
	local := filepath.Join(dir, "node_modules", ".bin", "vite")
	if _, err := os.Stat(local); err == nil {
		return local
	}
	// Fallback to global npx (will use cached vite if available)
	if p, err := exec.LookPath("npx"); err == nil {
		return p
	}
	return "npx"
}

func (rv *RuntimeVerifier) startVite(ctx context.Context, dir, viteBin string, port int) (*exec.Cmd, *bytes.Buffer, error) {
	var args []string
	if strings.HasSuffix(viteBin, "npx") || viteBin == "npx" {
		args = []string{"vite", "--port", strconv.Itoa(port), "--host", "127.0.0.1", "--logLevel", "error"}
	} else {
		args = []string{"--port", strconv.Itoa(port), "--host", "127.0.0.1", "--logLevel", "error"}
	}

	cmd := exec.CommandContext(ctx, viteBin, args...)
	cmd.Dir = dir
	// Restrict env to avoid leaking secrets
	cmd.Env = []string{
		"HOME=" + os.Getenv("HOME"),
		"PATH=" + os.Getenv("PATH"),
		"NODE_ENV=development",
		"FORCE_COLOR=0",
	}

	var logs bytes.Buffer
	cmd.Stderr = &logs
	cmd.Stdout = &logs

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return cmd, &logs, nil
}

func (rv *RuntimeVerifier) freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port, nil
}

// waitForTCPPort polls until the given port accepts TCP connections or timeout.
func waitForTCPPort(port int, timeout time.Duration, stop <-chan struct{}) bool {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		select {
		case <-stop:
			return false
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", addr, 150*time.Millisecond)
			if err == nil {
				conn.Close()
				return true
			}
		}
	}
	return false
}

func waitForTCPPortOrExit(port int, timeout time.Duration, stop <-chan struct{}, exitCh <-chan error) (ready bool, exited bool, err error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		select {
		case <-stop:
			return false, false, nil
		case err := <-exitCh:
			return false, true, err
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", addr, 150*time.Millisecond)
			if err == nil {
				conn.Close()
				return true, false, nil
			}
		}
	}
	return false, false, nil
}

// ── HTTP check helpers ────────────────────────────────────────────────────────

func (rv *RuntimeVerifier) checkRootPage(ctx context.Context, client *http.Client, base string) (body string, c CheckResult) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/", nil)
	if err != nil {
		return "", check("root_page_200", false, "build request: "+err.Error())
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", check("root_page_200", false, "GET /: "+err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", check("root_page_200", false, fmt.Sprintf("GET / returned HTTP %d", resp.StatusCode))
	}

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	bodyStr := string(raw)
	lower := strings.ToLower(bodyStr)

	if len(strings.TrimSpace(bodyStr)) < 50 {
		return bodyStr, check("root_page_200", false,
			fmt.Sprintf("root page body too small (%d bytes) — blank screen", len(strings.TrimSpace(bodyStr))))
	}
	if !strings.Contains(lower, "<html") && !strings.Contains(lower, "<!doctype") {
		return bodyStr, check("root_page_200", false, "root page response is not valid HTML")
	}

	return bodyStr, check("root_page_200", true, fmt.Sprintf("HTTP 200, %d bytes", len(bodyStr)))
}

func (rv *RuntimeVerifier) checkMountPoint(html string) CheckResult {
	lower := strings.ToLower(html)
	mountPoints := []string{
		`id="root"`, `id='root'`,
		`id="app"`, `id='app'`,
		`id="__next"`, `id='__next'`,
		`id="app-root"`,
	}
	for _, mp := range mountPoints {
		if strings.Contains(lower, mp) {
			return check("mount_point_present", true, mp)
		}
	}
	return check("mount_point_present", false, "no <div id=\"root\"> or <div id=\"app\"> found in HTML")
}

func (rv *RuntimeVerifier) checkViteClient(ctx context.Context, client *http.Client, base string) CheckResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/@vite/client", nil)
	if err != nil {
		return check("vite_client_accessible", false, "build request: "+err.Error())
	}
	resp, err := client.Do(req)
	if err != nil {
		return check("vite_client_accessible", false, "GET /@vite/client: "+err.Error())
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return check("vite_client_accessible", false, fmt.Sprintf("/@vite/client returned HTTP %d", resp.StatusCode))
	}
	return check("vite_client_accessible", true, "")
}

// viteErrorRe matches error markers that Vite injects into failed module responses.
var viteErrorRe = regexp.MustCompile(`(?i)(transform failed|internal server error|\[vite\]|SyntaxError:|Cannot find module|Module not found|ENOENT|Failed to resolve)`)

func (rv *RuntimeVerifier) checkEntryModule(ctx context.Context, client *http.Client, base, src string) CheckResult {
	name := "entry_module:" + src
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+src, nil)
	if err != nil {
		return check(name, false, "build request: "+err.Error())
	}
	resp, err := client.Do(req)
	if err != nil {
		return check(name, false, "GET "+src+": "+err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return check(name, false, fmt.Sprintf("%s: 404 Not Found — file missing or import path wrong", src))
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		errDetail := strings.TrimSpace(string(raw))
		if len(errDetail) > 200 {
			errDetail = errDetail[:200] + "…"
		}
		return check(name, false,
			fmt.Sprintf("%s: HTTP %d — %s", src, resp.StatusCode, errDetail))
	}

	// Check for Vite error markers in 200 responses (Vite sometimes returns 200 with error content)
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	content := string(raw)
	if viteErrorRe.MatchString(content) && len(strings.TrimSpace(content)) < 500 {
		// Short response with error markers = transform failed
		detail := strings.TrimSpace(content)
		if len(detail) > 200 {
			detail = detail[:200] + "…"
		}
		return check(name, false, fmt.Sprintf("%s: Vite transform error — %s", src, detail))
	}
	if len(strings.TrimSpace(content)) < 20 {
		return check(name, false, fmt.Sprintf("%s: entry module response is empty", src))
	}

	return check(name, true, fmt.Sprintf("HTTP %d, %d bytes", resp.StatusCode, len(content)))
}

func (rv *RuntimeVerifier) checkAsset(ctx context.Context, client *http.Client, base, href, kind string) CheckResult {
	name := kind + "_asset:" + href
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+href, nil)
	if err != nil {
		return check(name, false, err.Error())
	}
	resp, err := client.Do(req)
	if err != nil {
		return check(name, false, "GET "+href+": "+err.Error())
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return check(name, false, fmt.Sprintf("%s: HTTP %d", href, resp.StatusCode))
	}
	return check(name, true, "")
}

// ── HTML parsing helpers ──────────────────────────────────────────────────────

// scriptSrcRe matches <script ... src="..."> or <script ... src='...'>
var scriptSrcRe = regexp.MustCompile(`(?i)<script[^>]+\bsrc=["']([^"']+)["']`)

func parseScriptSrcs(html string) []string {
	var srcs []string
	for _, m := range scriptSrcRe.FindAllStringSubmatch(html, -1) {
		if len(m) >= 2 {
			src := strings.TrimSpace(m[1])
			if src != "" && !strings.HasPrefix(src, "http") {
				srcs = append(srcs, src)
			}
		}
	}
	return srcs
}

// cssLinkRe matches <link rel="stylesheet" href="..."> in any attribute order
var cssLinkRe = regexp.MustCompile(`(?i)<link[^>]+\brel=["']stylesheet["'][^>]+\bhref=["']([^"']+)["']|<link[^>]+\bhref=["']([^"']+)["'][^>]+\brel=["']stylesheet["']`)

func parseCSSLinks(html string) []string {
	var links []string
	for _, m := range cssLinkRe.FindAllStringSubmatch(html, -1) {
		href := m[1]
		if href == "" {
			href = m[2]
		}
		href = strings.TrimSpace(href)
		if href != "" && !strings.HasPrefix(href, "http") {
			links = append(links, href)
		}
	}
	return links
}

// ── Result constructors ───────────────────────────────────────────────────────

func (rv *RuntimeVerifier) rtFail(kind, details, hint string, start time.Time) *RuntimeVerificationResult {
	return rv.rtFailWithLogs(kind, details, hint, start, "")
}

func (rv *RuntimeVerifier) rtFailWithLogs(kind, details, hint string, start time.Time, logs string) *RuntimeVerificationResult {
	r := &RuntimeVerificationResult{
		Passed:      false,
		FailureKind: kind,
		Details:     details,
		Duration:    time.Since(start),
		ServerLogs:  logs,
	}
	if hint != "" {
		r.RepairHints = []string{hint}
	}
	return r
}

func truncateLog(s string, maxBytes int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxBytes {
		return s
	}
	return "…" + s[len(s)-maxBytes:]
}

func sanitizeRuntimeVerifyPath(raw string) (string, bool) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(raw)))
	if clean == "." || clean == "" {
		return "", false
	}
	if filepath.IsAbs(clean) {
		return "", false
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false
	}
	return clean, true
}
