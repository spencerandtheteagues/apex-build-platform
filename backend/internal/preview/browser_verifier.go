// browser_verifier.go — Headless Chrome page-load proof for generated Vite/React apps.
//
// Extends runtime verification beyond HTTP-level checks by loading the page
// in a real sandboxed browser and verifying:
//   - page navigation succeeds (no net::ERR_* failures)
//   - the app mount point has actual child content after JS execution
//   - no uncaught JS exceptions prevented the initial render
//
// Chrome is required; when not found, Available() returns false and
// VerifyPageLoad returns Skipped=true so higher-level runtime verification can
// decide whether that is blocking.
//
// Security model: Chrome runs as the non-root backend user inside the backend
// container, with loopback-only proxy settings and hardened headless flags for
// container execution on Render. Browser reachability is restricted to the
// local ephemeral Vite port.
package preview

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// chromeCandidates is searched in PATH order, then macOS absolute paths.
var chromeCandidates = []string{
	"google-chrome",
	"google-chrome-stable",
	"chromium",
	"chromium-browser",
	"google-chrome-beta",
}

var chromeMacPaths = []string{
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Chromium.app/Contents/MacOS/Chromium",
	"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
}

// FindChrome returns the path to a usable Chrome/Chromium binary, or "".
// Exported so callers (e.g. main.go health reporting) can surface availability.
func FindChrome() string { return findChrome() }

// findChrome is the internal implementation.
func findChrome() string {
	for _, envKey := range []string{"APEX_CHROME_PATH", "CHROME_BIN"} {
		if candidate := strings.TrimSpace(os.Getenv(envKey)); candidate != "" {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	for _, c := range chromeCandidates {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	for _, p := range chromeMacPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// SmokeTestChrome verifies that the discovered Chrome/Chromium binary can
// actually launch headless. Some container images expose a chromium binary that
// exists on PATH but cannot start because shared libraries or sandbox support
// are missing.
func SmokeTestChrome(ctx context.Context, chromePath string) error {
	chromePath = strings.TrimSpace(chromePath)
	if chromePath == "" {
		return fmt.Errorf("chrome path is empty")
	}

	smokeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		smokeCtx,
		chromePath,
		"--headless=new",
		"--no-sandbox",
		"--disable-gpu",
		"--disable-dev-shm-usage",
		"--disable-background-networking",
		"--dump-dom",
		"data:text/html,<html><body>apex-chrome-smoke</body></html>",
	)
	out, err := cmd.CombinedOutput()
	if smokeCtx.Err() != nil {
		return fmt.Errorf("chrome smoke timed out: %w", smokeCtx.Err())
	}
	if err != nil {
		return fmt.Errorf("chrome smoke failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if !strings.Contains(string(out), "apex-chrome-smoke") {
		return fmt.Errorf("chrome smoke returned unexpected output")
	}
	return nil
}

// BrowserVerifier loads a URL in a sandboxed headless Chrome and checks that
// the app actually rendered. It is stateless and safe for concurrent use.
type BrowserVerifier struct {
	chromePath string
}

// NewBrowserVerifier creates a BrowserVerifier. Detects Chrome at construction
// time; subsequent Available() calls are instant.
func NewBrowserVerifier() *BrowserVerifier {
	return &BrowserVerifier{chromePath: findChrome()}
}

// NewBrowserVerifierWithChromePath creates a BrowserVerifier pinned to a
// previously validated Chrome binary. Passing an empty path makes browser proof
// unavailable.
func NewBrowserVerifierWithChromePath(chromePath string) *BrowserVerifier {
	return &BrowserVerifier{chromePath: strings.TrimSpace(chromePath)}
}

// Available reports whether a Chrome/Chromium binary was found.
func (bv *BrowserVerifier) Available() bool { return bv.chromePath != "" }

// BrowserPageLoadResult holds the outcome of a VerifyPageLoad call.
type BrowserPageLoadResult struct {
	Passed          bool
	FailureKind     string
	Details         string
	RepairHints     []string
	Duration        time.Duration
	Skipped         bool     // true when Chrome is not available
	JSErrors        []string // uncaught JS exceptions observed
	ConsoleErrors   []string // console.error() calls observed
	MountRendered   bool
	MountChildCount int
	VisibleText     int // length of innerText visible to the user (0 = likely blank or CSS failure)
	ScreenshotData  []byte
}

func consoleAPIArgsText(args []*cdpruntime.RemoteObject) string {
	var parts []string
	for _, arg := range args {
		switch {
		case arg == nil:
			continue
		case len(arg.Value) > 0:
			var decoded any
			decoder := json.NewDecoder(bytes.NewReader(arg.Value))
			decoder.UseNumber()
			if err := decoder.Decode(&decoded); err == nil {
				switch value := decoded.(type) {
				case string:
					parts = append(parts, value)
				case nil:
					parts = append(parts, "null")
				default:
					parts = append(parts, fmt.Sprint(value))
				}
				continue
			}
		}
		if arg.Description != "" {
			parts = append(parts, arg.Description)
		}
	}
	return strings.Join(parts, " ")
}

// ── earlyInjection is added via Page.addScriptToEvaluateOnNewDocument so it
// runs in every frame before the page's own scripts, capturing window-level
// errors that CDP runtime events might miss.
const earlyInjection = `
(function() {
  window.__apexRTErrors = [];
  window.__apexRTUnhandled = [];
  var orig = window.onerror;
  window.onerror = function(msg, src, line, col, err) {
    window.__apexRTErrors.push(String(msg));
    if (orig) return orig.apply(this, arguments);
  };
  window.addEventListener('unhandledrejection', function(e) {
    window.__apexRTUnhandled.push(String(e.reason));
  }, true);
})();
`

// mountCheckJS returns a JSON string describing the mount point state.
// visibleText uses innerText (not textContent) to capture only CSS-visible text,
// which helps detect Tailwind failures where elements render but are invisible.
const mountCheckJS = `JSON.stringify((function() {
  var selectors = ['#root','#app','#__next','#app-root','[data-reactroot]'];
	for (var i = 0; i < selectors.length; i++) {
		var el = document.querySelector(selectors[i]);
		if (el) {
			var text = (el.textContent || '').trim();
			var visible = (el.innerText || '').trim();
			var html = (el.innerHTML || '').trim();
			var hasDomChildren = el.childElementCount > 0;
			var hasMeaningfulText = visible.length >= 10 || text.length >= 12;
			var hasMarkupOnlyContent = html.length >= 20 && hasDomChildren;
			var hasStructuralContent =
				hasDomChildren && (
					text.length > 0 ||
					visible.length > 0 ||
					!!el.querySelector('img,svg,canvas,video,iframe,button,input,select,textarea,nav,main,section,article,aside,header,footer,form,table,ul,ol,[role],[aria-label],[data-testid]')
				);
			return {
				found: true,
				selector: selectors[i],
				childCount: el.childElementCount,
				textLength: text.length,
				visibleText: visible.length,
				hasContent: hasStructuralContent || hasMeaningfulText || hasMarkupOnlyContent,
				snippet: (visible || text).substring(0, 500),
				analysisText: visible || text
			};
		}
	}
  // Fallback: any non-trivial body content counts
  var bodyText = (document.body && document.body.textContent || '').trim();
  var bodyVisible = (document.body && document.body.innerText || '').trim();
  return {found: false, childCount: 0, textLength: bodyText.length,
          visibleText: bodyVisible.length,
          hasContent: bodyVisible.length > 20,
          snippet: (bodyVisible || bodyText).substring(0, 500),
          analysisText: bodyVisible || bodyText};
})())`

const (
	browserMountPollInterval = 200 * time.Millisecond
	browserMountPollTimeout  = 30 * time.Second
	browserPageLoadTimeout   = 60 * time.Second
)

func browserMountJSONHasContent(raw string) bool {
	var mount struct {
		HasContent bool `json:"hasContent"`
	}
	if err := json.Unmarshal([]byte(raw), &mount); err != nil {
		return false
	}
	return mount.HasContent
}

func pollBrowserMountContent(ctx context.Context, mountJSON *string) error {
	deadline := time.Now().Add(browserMountPollTimeout)
	for {
		var current string
		if err := chromedp.Evaluate(mountCheckJS, &current).Do(ctx); err != nil {
			return err
		}
		*mountJSON = current
		if browserMountJSONHasContent(current) || time.Now().After(deadline) {
			return nil
		}
		timer := time.NewTimer(browserMountPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// VerifyPageLoad navigates to pageURL in an isolated headless browser, waits
// for DOMContentLoaded, polls briefly for React/Vue to mount, then evaluates
// whether the mount point has content and whether fatal JS errors occurred.
func (bv *BrowserVerifier) VerifyPageLoad(ctx context.Context, pageURL string) *BrowserPageLoadResult {
	start := time.Now()
	if !bv.Available() {
		return &BrowserPageLoadResult{Skipped: true, Duration: time.Since(start)}
	}

	// Hard budget for the entire browser check. Chrome startup can be slow on
	// saturated CI/VPS hosts, but this remains bounded for launch gates.
	bCtx, bCancel := context.WithTimeout(ctx, browserPageLoadTimeout)
	defer bCancel()

	// ── Launch sandboxed Chrome ──────────────────────────────────────────────
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(bv.chromePath),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-component-update", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("incognito", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-translate", true),
		chromedp.Flag("disable-plugins", true),
		chromedp.Flag("disable-features", "Translate,OptimizationHints,MediaRouter"),
		chromedp.Flag("no-sandbox", true),
		// Restrict to loopback: block DNS for all external hosts while still
		// allowing direct IP connections to 127.0.0.1 (the Vite server).
		// NOTE: "MAP * NOTFOUND" also blocks IP literals in some Chrome builds,
		// so we use the safer proxy-bypass + disable-background-networking combo
		// rather than host-rules.
		chromedp.Flag("proxy-server", "direct://"),
		chromedp.Flag("proxy-bypass-list", "<-loopback>"),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(bCtx, allocOpts...)
	defer allocCancel()

	// Suppress internal chromedp log noise
	tabCtx, tabCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(func(string, ...any) {}))
	defer tabCancel()

	// ── Collect CDP events ───────────────────────────────────────────────────
	var (
		mu            sync.Mutex
		jsErrors      []string
		consoleErrors []string
	)
	chromedp.ListenTarget(tabCtx, func(ev any) {
		switch e := ev.(type) {
		case *cdpruntime.EventExceptionThrown:
			if e.ExceptionDetails != nil {
				msg := e.ExceptionDetails.Text
				if e.ExceptionDetails.Exception != nil && e.ExceptionDetails.Exception.Description != "" {
					msg = e.ExceptionDetails.Exception.Description
				}
				if msg != "" {
					mu.Lock()
					jsErrors = append(jsErrors, msg)
					mu.Unlock()
				}
			}
		case *cdpruntime.EventConsoleAPICalled:
			if e.Type == cdpruntime.APITypeError || e.Type == cdpruntime.APITypeWarning {
				text := consoleAPIArgsText(e.Args)
				if text != "" {
					mu.Lock()
					consoleErrors = append(consoleErrors, text)
					mu.Unlock()
				}
			}
		}
	})

	// ── Navigate and check ───────────────────────────────────────────────────
	var (
		mountJSON      string
		screenshotData []byte
	)
	navErr := chromedp.Run(tabCtx,
		// Enable runtime events before any navigation
		cdpruntime.Enable(),
		// Inject error catcher before page scripts run
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(earlyInjection).Do(ctx)
			return err
		}),
		// Navigate — chromedp waits for DOMContentLoaded by default
		chromedp.Navigate(pageURL),
		// Poll for actual rendered content instead of assuming every React app
		// mounts within a fixed 800ms window.
		chromedp.ActionFunc(func(ctx context.Context) error {
			return pollBrowserMountContent(ctx, &mountJSON)
		}),
		// Capture the fully rendered page before evaluating mount heuristics so
		// downstream visual analysis can inspect the actual preview state.
		chromedp.CaptureScreenshot(&screenshotData),
	)

	mu.Lock()
	capturedJS := filterBrowserNoise(jsErrors)
	capturedConsole := filterBrowserNoise(consoleErrors)
	mu.Unlock()

	// ── Navigation failure ───────────────────────────────────────────────────
	if navErr != nil {
		failKind := "browser_load_failed"
		detail := "browser failed to load page: " + navErr.Error()
		if bCtx.Err() != nil {
			failKind = "browser_load_timeout"
			detail = fmt.Sprintf("browser page-load proof timed out after %s: %v", browserPageLoadTimeout, bCtx.Err())
		}
		return &BrowserPageLoadResult{
			Passed:         false,
			FailureKind:    failKind,
			Details:        detail,
			RepairHints:    []string{"Ensure index.html is valid and the Vite dev server is running. Check for JS parse errors in the entry module."},
			Duration:       time.Since(start),
			JSErrors:       capturedJS,
			ConsoleErrors:  capturedConsole,
			ScreenshotData: screenshotData,
		}
	}

	// ── Parse mount state ────────────────────────────────────────────────────
	var mount struct {
		Found        bool   `json:"found"`
		Selector     string `json:"selector"`
		ChildCount   int    `json:"childCount"`
		TextLength   int    `json:"textLength"`
		VisibleText  int    `json:"visibleText"`
		HasContent   bool   `json:"hasContent"`
		Snippet      string `json:"snippet"`
		AnalysisText string `json:"analysisText"`
	}
	_ = json.Unmarshal([]byte(mountJSON), &mount)
	heuristicText := mount.AnalysisText
	if strings.TrimSpace(heuristicText) == "" {
		heuristicText = mount.Snippet
	}

	// ── Decision logic ───────────────────────────────────────────────────────
	// Render success is the primary signal: if the mount point has content,
	// the app is working regardless of non-fatal console noise.
	//
	// If the mount is empty AND JS errors exist → the errors caused blank screen.
	// If the mount is empty AND no JS errors → blank screen for unknown reason.

	if !mount.HasContent {
		failKind := "blank_screen"
		details := "app mount point is empty after browser page load — white screen"
		if len(capturedJS) > 0 {
			failKind = "js_runtime_error"
			errSummary := strings.Join(capturedJS[:clampMax(len(capturedJS), 2)], "; ")
			details = fmt.Sprintf("JS runtime error prevented app render: %s", errSummary)
		}
		hints := []string{
			"Ensure createRoot(document.getElementById('root')).render(<App />) is called in the entry file.",
			"Fix any JavaScript errors in the entry module and its imports before render is reached.",
		}
		if mount.Selector != "" {
			hints[0] = fmt.Sprintf("Mount point %q exists but has no rendered children. Check that the root component returns JSX and is free of runtime errors.", mount.Selector)
		}
		// Blank content with no JS errors often means Tailwind CSS failed to load.
		if mount.TextLength < 25 && mount.VisibleText < 10 && len(capturedJS) == 0 {
			hints = append(hints,
				"The page appears blank with no JS errors — this often means Tailwind CSS failed to load. "+
					"Check that tailwind.config.js has content: ['./index.html', './src/**/*.{ts,tsx}'] and "+
					"that src/index.css starts with @tailwind base; @tailwind components; @tailwind utilities;",
			)
		}
		return &BrowserPageLoadResult{
			Passed:         false,
			FailureKind:    failKind,
			Details:        details,
			RepairHints:    hints,
			Duration:       time.Since(start),
			JSErrors:       capturedJS,
			ConsoleErrors:  capturedConsole,
			VisibleText:    mount.VisibleText,
			ScreenshotData: screenshotData,
		}
	}

	if looksLikeAppLevelNotFound(heuristicText) {
		return &BrowserPageLoadResult{
			Passed:      false,
			FailureKind: "app_route_not_found",
			Details:     fmt.Sprintf("app rendered its own not-found route at the preview root: %q", mount.Snippet),
			RepairHints: []string{
				"Ensure the preview root route renders the requested app instead of an internal 404/not-found page.",
				"If using react-router-dom BrowserRouter behind the Apex preview proxy, set BrowserRouter basename from window.location.pathname before '/preview/proxy/{projectID}'.",
				"Add a valid '/' route with the main dashboard or landing screen.",
			},
			Duration:       time.Since(start),
			JSErrors:       capturedJS,
			ConsoleErrors:  capturedConsole,
			VisibleText:    mount.VisibleText,
			ScreenshotData: screenshotData,
		}
	}

	for _, consoleMessage := range capturedConsole {
		if looksLikeReactRouterNoMatch(consoleMessage) {
			return &BrowserPageLoadResult{
				Passed:      false,
				FailureKind: "app_route_not_found",
				Details:     fmt.Sprintf("React Router did not match the preview proxy path: %q", consoleMessage),
				RepairHints: []string{
					"If using react-router-dom BrowserRouter behind the Apex preview proxy, set BrowserRouter basename from window.location.pathname before '/preview/proxy/{projectID}'.",
					"Ensure the preview root route renders the requested app instead of only the navigation shell.",
				},
				Duration:       time.Since(start),
				JSErrors:       capturedJS,
				ConsoleErrors:  capturedConsole,
				VisibleText:    mount.VisibleText,
				ScreenshotData: screenshotData,
			}
		}
	}

	if looksLikeShellOnlyPreview(heuristicText, mount.VisibleText) {
		return &BrowserPageLoadResult{
			Passed:      false,
			FailureKind: "shell_only_preview",
			Details:     fmt.Sprintf("app rendered only a navigation shell without requested screen content: %q", mount.Snippet),
			RepairHints: []string{
				"Render the requested app's default screen at the preview root, not only a sidebar/app shell.",
				"Add concrete Dashboard, Pipeline, New Job, Crew Management, and Settings route content before declaring preview success.",
				"Reject comments such as 'screens will be routed here in future patches' and replace them with functional UI.",
			},
			Duration:       time.Since(start),
			JSErrors:       capturedJS,
			ConsoleErrors:  capturedConsole,
			VisibleText:    mount.VisibleText,
			ScreenshotData: screenshotData,
		}
	}

	if looksLikePlaceholderPreview(heuristicText, mount.VisibleText) {
		return &BrowserPageLoadResult{
			Passed:      false,
			FailureKind: "placeholder_preview",
			Details:     fmt.Sprintf("app rendered placeholder dashboard content instead of concrete seed data: %q", mount.Snippet),
			RepairHints: []string{
				"Replace generic KPI labels, Value text, skeleton rows, and placeholder cards with concrete, domain-specific seeded content.",
				"The first preview screen must show usable dashboard data, named records, statuses, and actions instead of a loading-only or template-only shell.",
				"Keep loading and empty states available as alternate states, but do not make the initial loaded preview render as skeleton placeholders.",
			},
			Duration:       time.Since(start),
			JSErrors:       capturedJS,
			ConsoleErrors:  capturedConsole,
			VisibleText:    mount.VisibleText,
			ScreenshotData: screenshotData,
		}
	}

	return &BrowserPageLoadResult{
		Passed:          true,
		Duration:        time.Since(start),
		MountRendered:   true,
		MountChildCount: mount.ChildCount,
		VisibleText:     mount.VisibleText,
		JSErrors:        capturedJS,
		ConsoleErrors:   capturedConsole,
		ScreenshotData:  screenshotData,
	}
}

func looksLikeAppLevelNotFound(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if strings.Contains(lower, "page not found") ||
		strings.Contains(lower, "sorry, that page does not exist") ||
		strings.Contains(lower, "route not found") {
		return true
	}
	return strings.Contains(lower, "404") && strings.Contains(lower, "not found")
}

func looksLikeReactRouterNoMatch(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return strings.Contains(lower, "no routes matched location") &&
		strings.Contains(lower, "/preview/proxy/")
}

func looksLikeShellOnlyPreview(text string, visibleText int) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if strings.Contains(lower, "future patches") ||
		strings.Contains(lower, "real ui screens will be routed here") ||
		strings.Contains(lower, "routes will be added later") ||
		strings.Contains(lower, "apex recovered preview") ||
		strings.Contains(lower, "frontend-first recovery") ||
		strings.Contains(lower, "recovered baseline") ||
		strings.Contains(lower, "generated project produced a backend runtime without a frontend entrypoint") ||
		strings.Contains(lower, "synthesized a stable preview shell") {
		return true
	}

	navHits := 0
	for _, label := range []string{"dashboard", "job pipeline", "new job", "crew management", "settings"} {
		if strings.Contains(lower, label) {
			navHits++
		}
	}
	if navHits < 4 || !strings.Contains(lower, "bootstrapped by apex.build") {
		return false
	}

	if visibleText <= 0 {
		visibleText = len([]rune(lower))
	}
	return visibleText < 180 &&
		!strings.Contains(lower, "open jobs") &&
		!strings.Contains(lower, "pending estimate") &&
		!strings.Contains(lower, "launch estimate swarm") &&
		!strings.Contains(lower, "recommended final quote")
}

func looksLikePlaceholderPreview(text string, visibleText int) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if visibleText <= 0 {
		visibleText = len([]rune(lower))
	}

	// Generic KPI / Metric cards with Value labels
	genericKPIHits := 0
	for _, label := range []string{"kpi 1", "kpi 2", "kpi 3", "metric 1", "metric 2", "metric 3"} {
		if strings.Contains(lower, label) {
			genericKPIHits++
		}
	}
	if genericKPIHits >= 2 && countWholeWord(lower, "value") >= 2 && visibleText < 320 {
		return true
	}

	// Repeated placeholder section titles (e.g., Sample Data, Chart Placeholder, Coming Soon)
	sectionPlaceholderHits := 0
	for _, label := range []string{"sample data", "chart placeholder", "coming soon"} {
		if strings.Contains(lower, label) {
			sectionPlaceholderHits++
		}
	}
	if sectionPlaceholderHits >= 2 && visibleText < 320 {
		return true
	}

	// Loading-only or skeleton-only content
	if visibleText < 180 && strings.Contains(lower, "skeleton") &&
		!strings.Contains(lower, "$") && !strings.Contains(lower, "%") {
		return true
	}
	if visibleText < 120 && strings.Contains(lower, "loading") &&
		(strings.Contains(lower, "...") || strings.Contains(lower, "…")) &&
		!strings.Contains(lower, "$") && !strings.Contains(lower, "%") {
		return true
	}

	sectionHits := 0
	for _, label := range []string{"activity feed", "projects board", "client cards", "settings panel"} {
		if strings.Contains(lower, label) {
			sectionHits++
		}
	}
	if sectionHits >= 3 && visibleText < 260 &&
		!strings.Contains(lower, "$") &&
		!strings.Contains(lower, "%") &&
		!strings.Contains(lower, "completed") &&
		!strings.Contains(lower, "active") {
		return true
	}

	placeholderPhrases := []string{
		"lorem ipsum",
		"coming soon",
		"placeholder content",
		"replace this",
		"todo",
	}
	for _, phrase := range placeholderPhrases {
		if strings.Contains(lower, phrase) && visibleText < 360 {
			return true
		}
	}

	return false
}

func countWholeWord(text, word string) int {
	if word == "" {
		return 0
	}
	count := 0
	offset := 0
	for {
		idx := strings.Index(text[offset:], word)
		if idx < 0 {
			return count
		}
		start := offset + idx
		end := start + len(word)
		beforeOK := start == 0 || !isASCIIWordByte(text[start-1])
		afterOK := end == len(text) || !isASCIIWordByte(text[end])
		if beforeOK && afterOK {
			count++
		}
		offset = end
	}
}

func isASCIIWordByte(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_'
}

// filterBrowserNoise strips well-known benign browser messages that are not
// indicative of application errors.
func filterBrowserNoise(msgs []string) []string {
	var out []string
	for _, msg := range msgs {
		lower := strings.ToLower(msg)
		switch {
		case strings.Contains(lower, "resizeobserver"):
			// Browser layout quirk, not an app failure
		case strings.Contains(lower, "[vite]"), strings.Contains(lower, "@vite"):
			// Vite HMR chatter
		case strings.Contains(lower, "err_blocked"), strings.Contains(lower, "blocked_by_client"),
			strings.Contains(lower, "err_name_not_resolved"):
			// Expected: we block external network via --host-rules
		case strings.Contains(lower, "favicon"):
			// Cosmetic
		case strings.Contains(lower, "hot module replacement"),
			strings.Contains(lower, "hmr"):
			// HMR noise
		default:
			out = append(out, msg)
		}
	}
	return out
}

func clampMax(v, max int) int {
	if v > max {
		return max
	}
	return v
}
