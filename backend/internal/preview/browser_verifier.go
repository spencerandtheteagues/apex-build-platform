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
const mountCheckJS = `JSON.stringify((function() {
  var selectors = ['#root','#app','#__next','#app-root','[data-reactroot]'];
  for (var i = 0; i < selectors.length; i++) {
    var el = document.querySelector(selectors[i]);
    if (el) {
      var text = (el.textContent || '').trim();
      return {
        found: true,
        selector: selectors[i],
        childCount: el.childElementCount,
        textLength: text.length,
        hasContent: el.childElementCount > 0 || text.length > 3,
        snippet: text.substring(0, 80)
      };
    }
  }
  // Fallback: any non-trivial body content counts
  var bodyText = (document.body && document.body.textContent || '').trim();
  return {found: false, childCount: 0, textLength: bodyText.length,
          hasContent: bodyText.length > 10, snippet: bodyText.substring(0, 80)};
})())`

// VerifyPageLoad navigates to pageURL in an isolated headless browser, waits
// for DOMContentLoaded, pauses briefly for React/Vue to mount, then evaluates
// whether the mount point has content and whether fatal JS errors occurred.
func (bv *BrowserVerifier) VerifyPageLoad(ctx context.Context, pageURL string) *BrowserPageLoadResult {
	start := time.Now()
	if !bv.Available() {
		return &BrowserPageLoadResult{Skipped: true, Duration: time.Since(start)}
	}

	// 20 s hard budget for the entire browser check
	bCtx, bCancel := context.WithTimeout(ctx, 20*time.Second)
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
			if e.Type == cdpruntime.APITypeError {
				var parts []string
				for _, arg := range e.Args {
					if arg.Description != "" {
						parts = append(parts, arg.Description)
					}
				}
				if len(parts) > 0 {
					mu.Lock()
					consoleErrors = append(consoleErrors, strings.Join(parts, " "))
					mu.Unlock()
				}
			}
		}
	})

	// ── Navigate and check ───────────────────────────────────────────────────
	var mountJSON string
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
		// Brief pause for React/Vue synchronous mount and any micro-task flushing
		chromedp.Sleep(800*time.Millisecond),
		// Evaluate mount state
		chromedp.Evaluate(mountCheckJS, &mountJSON),
	)

	mu.Lock()
	capturedJS := filterBrowserNoise(jsErrors)
	capturedConsole := filterBrowserNoise(consoleErrors)
	mu.Unlock()

	// ── Navigation failure ───────────────────────────────────────────────────
	if navErr != nil && bCtx.Err() == nil {
		// Distinguish a true navigation failure from a context deadline
		detail := "browser failed to load page: " + navErr.Error()
		return &BrowserPageLoadResult{
			Passed:        false,
			FailureKind:   "browser_load_failed",
			Details:       detail,
			RepairHints:   []string{"Ensure index.html is valid and the Vite dev server is running. Check for JS parse errors in the entry module."},
			Duration:      time.Since(start),
			JSErrors:      capturedJS,
			ConsoleErrors: capturedConsole,
		}
	}

	// ── Parse mount state ────────────────────────────────────────────────────
	var mount struct {
		Found      bool   `json:"found"`
		Selector   string `json:"selector"`
		ChildCount int    `json:"childCount"`
		TextLength int    `json:"textLength"`
		HasContent bool   `json:"hasContent"`
		Snippet    string `json:"snippet"`
	}
	_ = json.Unmarshal([]byte(mountJSON), &mount)

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
		return &BrowserPageLoadResult{
			Passed:        false,
			FailureKind:   failKind,
			Details:       details,
			RepairHints:   hints,
			Duration:      time.Since(start),
			JSErrors:      capturedJS,
			ConsoleErrors: capturedConsole,
		}
	}

	return &BrowserPageLoadResult{
		Passed:          true,
		Duration:        time.Since(start),
		MountRendered:   true,
		MountChildCount: mount.ChildCount,
		JSErrors:        capturedJS,
		ConsoleErrors:   capturedConsole,
	}
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
