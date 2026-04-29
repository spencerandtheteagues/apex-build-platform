package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// CanaryResult captures advisory interaction coverage for a preview page.
// It never turns a passing preview into a hard failure by itself.
type CanaryResult struct {
	Clicked                        int
	VisibleControls                int
	PostInteractionVisibleControls int
	PostInteractionChecked         bool
	PostInteractionHealthy         bool
	Errors                         []string
	RepairHints                    []string
	Duration                       time.Duration
	Skipped                        bool
}

// CanaryTester exercises a small set of interactive controls in headless Chrome
// to catch "loads but unusable" regressions before users do.
type CanaryTester struct {
	chromePath string
	enabled    bool
}

func NewCanaryTester() *CanaryTester {
	return &CanaryTester{
		chromePath: findChrome(),
		enabled:    canaryProbesEnabled(),
	}
}

func (ct *CanaryTester) Available() bool {
	return ct != nil && ct.enabled && ct.chromePath != ""
}

// CanaryProbesEnabled reports whether interaction probes are enabled.
// The default is on; APEX_CANARY_PROBES=false disables the feature explicitly.
func CanaryProbesEnabled() bool {
	return canaryProbesEnabled()
}

func canaryProbesEnabled() bool {
	switch strings.TrimSpace(strings.ToLower(os.Getenv("APEX_CANARY_PROBES"))) {
	case "", "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

const canaryHarnessJS = `JSON.stringify((function() {
  var clickErrors = [];
  var clicked = 0;
  var visibleControls = 0;
  var nodes = Array.prototype.slice.call(document.querySelectorAll(
    'button,[role=button],a[href],select,summary,input[type="checkbox"],input[type="radio"]'
  ));
  var blocker = function(event) {
    event.preventDefault();
  };

  for (var i = 0; i < nodes.length && i < 40; i++) {
    var el = nodes[i];
    if (!el || el.disabled || el.getAttribute('aria-disabled') === 'true') continue;
    var rect = el.getBoundingClientRect();
    if (!rect || rect.width < 2 || rect.height < 2) continue;
    visibleControls++;
    try {
      document.addEventListener('click', blocker, true);
      el.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, view: window }));
      clicked++;
    } catch (err) {
      clickErrors.push(String((err && err.message) || err));
    } finally {
      document.removeEventListener('click', blocker, true);
    }
  }

  return {
    clicked: clicked,
    visibleControls: visibleControls,
    errors: clickErrors
  };
})())`

const canaryRuntimeErrorsJS = `JSON.stringify((function() {
  var runtimeErrors = [];
  if (Array.isArray(window.__apexRTErrors)) {
    runtimeErrors = runtimeErrors.concat(window.__apexRTErrors.map(String));
  }
  if (Array.isArray(window.__apexRTUnhandled)) {
    runtimeErrors = runtimeErrors.concat(window.__apexRTUnhandled.map(String));
  }
  return { errors: runtimeErrors };
})())`

const canaryVisibleControlsJS = `(function() {
  var visibleControls = 0;
  var nodes = Array.prototype.slice.call(document.querySelectorAll(
    'button,[role=button],a[href],select,summary,input[type="checkbox"],input[type="radio"]'
  ));
  for (var i = 0; i < nodes.length; i++) {
    var el = nodes[i];
    if (!el || el.disabled || el.getAttribute('aria-disabled') === 'true') continue;
    var rect = el.getBoundingClientRect();
    if (!rect || rect.width < 2 || rect.height < 2) continue;
    visibleControls++;
  }
  return visibleControls;
})()`

type canaryInteractionPayload struct {
	Clicked         int      `json:"clicked"`
	VisibleControls int      `json:"visibleControls"`
	Errors          []string `json:"errors"`
}

type canaryRuntimeErrorPayload struct {
	Errors []string `json:"errors"`
}

type canaryMountPayload struct {
	Found       bool   `json:"found"`
	Selector    string `json:"selector"`
	ChildCount  int    `json:"childCount"`
	TextLength  int    `json:"textLength"`
	VisibleText int    `json:"visibleText"`
	HasContent  bool   `json:"hasContent"`
	Snippet     string `json:"snippet"`
}

type canaryProbeTelemetry struct {
	PageURL                        string `json:"page_url,omitempty"`
	Skipped                        bool   `json:"skipped"`
	Clicked                        int    `json:"clicked"`
	VisibleControls                int    `json:"visible_controls"`
	PostInteractionVisibleControls int    `json:"post_interaction_visible_controls"`
	PostInteractionChecked         bool   `json:"post_interaction_checked"`
	PostInteractionHealthy         bool   `json:"post_interaction_healthy"`
	ErrorCount                     int    `json:"error_count"`
	DurationMS                     int64  `json:"duration_ms"`
}

func (ct *CanaryTester) RunCanaryInteractions(ctx context.Context, pageURL string) *CanaryResult {
	start := time.Now()
	if !ct.Available() {
		return &CanaryResult{Skipped: true, Duration: time.Since(start)}
	}

	runCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(ct.chromePath),
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
		chromedp.Flag("proxy-server", "direct://"),
		chromedp.Flag("proxy-bypass-list", "<-loopback>"),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(runCtx, allocOpts...)
	defer allocCancel()

	tabCtx, tabCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(func(string, ...any) {}))
	defer tabCancel()

	var (
		baselineErrorsJSON string
		harnessJSON        string
		settledMountJSON   string
		settledVisible     int
		postErrorsJSON     string
	)
	err := chromedp.Run(tabCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(earlyInjection).Do(ctx)
			return err
		}),
		chromedp.Navigate(pageURL),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return pollBrowserMountContent(ctx, &settledMountJSON)
		}),
		chromedp.Evaluate(canaryRuntimeErrorsJS, &baselineErrorsJSON),
		chromedp.Evaluate(canaryHarnessJS, &harnessJSON),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return pollBrowserMountContent(ctx, &settledMountJSON)
		}),
		chromedp.Evaluate(canaryVisibleControlsJS, &settledVisible),
		chromedp.Evaluate(canaryRuntimeErrorsJS, &postErrorsJSON),
	)
	if err != nil {
		result := &CanaryResult{
			Duration:    time.Since(start),
			Errors:      []string{fmt.Sprintf("canary navigation failed: %v", err)},
			RepairHints: []string{"Review the first interactive route after mount and ensure the preview remains stable after initial user interactions."},
		}
		emitCanaryProbeTelemetry(pageURL, result)
		return result
	}

	var interaction canaryInteractionPayload
	var baselineErrors canaryRuntimeErrorPayload
	var postErrors canaryRuntimeErrorPayload
	var mount canaryMountPayload
	if err := json.Unmarshal([]byte(harnessJSON), &interaction); err != nil {
		log.Printf("[canary] build probe at %s: failed to parse harness JSON: %v", pageURL, err)
		return &CanaryResult{Skipped: true, Duration: time.Since(start)}
	}
	if err := json.Unmarshal([]byte(settledMountJSON), &mount); err != nil {
		log.Printf("[canary] build probe at %s: failed to parse mount JSON: %v", pageURL, err)
		return &CanaryResult{Skipped: true, Duration: time.Since(start)}
	}
	_ = json.Unmarshal([]byte(baselineErrorsJSON), &baselineErrors)
	_ = json.Unmarshal([]byte(postErrorsJSON), &postErrors)

	result := deriveCanaryResult(interaction, mount, settledVisible, baselineErrors.Errors, postErrors.Errors, time.Since(start))
	emitCanaryProbeTelemetry(pageURL, result)
	return result
}

func deriveCanaryResult(
	interaction canaryInteractionPayload,
	mount canaryMountPayload,
	postInteractionVisibleControls int,
	baselineErrors []string,
	postInteractionErrors []string,
	duration time.Duration,
) *CanaryResult {
	newRuntimeErrors := subtractStringMultiset(filterBrowserNoise(postInteractionErrors), filterBrowserNoise(baselineErrors))
	errors := appendUniqueStrings(nil, filterBrowserNoise(interaction.Errors)...)
	errors = appendUniqueStrings(errors, newRuntimeErrors...)

	result := &CanaryResult{
		Clicked:                        interaction.Clicked,
		VisibleControls:                interaction.VisibleControls,
		PostInteractionVisibleControls: postInteractionVisibleControls,
		PostInteractionChecked:         true,
		PostInteractionHealthy:         mount.HasContent,
		Errors:                         errors,
		Duration:                       duration,
	}

	if !mount.HasContent {
		errorText := "post-click settle check failed: preview became visually empty after interactions"
		if mount.Selector != "" {
			errorText = fmt.Sprintf("post-click settle check failed: %s stopped rendering after interactions", mount.Selector)
		}
		result.Errors = appendUniqueStrings(result.Errors, errorText)
	}

	if len(result.Errors) > 0 {
		result.RepairHints = append(result.RepairHints,
			"Fix the runtime error(s) triggered by basic interactions such as button clicks, links, or form controls.",
			fmt.Sprintf("Stabilize the UI after mount so the first %d interactive control(s) can be exercised without JS exceptions or blanking the preview.", maxInt(interaction.Clicked, 1)),
		)
	}
	if interaction.VisibleControls == 0 {
		result.RepairHints = append(result.RepairHints,
			"The preview mounted but exposes no visible buttons, links, menus, or toggles on first load. Ensure the first screen contains at least one obvious CTA or navigational control.",
		)
	}
	if interaction.VisibleControls > 0 && !mount.HasContent {
		result.RepairHints = append(result.RepairHints,
			"Keep the first rendered screen stable after the initial click path. The preview should remain visibly mounted after basic CTA or navigation interactions settle.",
		)
	}
	result.Errors = compactNonEmptyStrings(result.Errors)
	result.RepairHints = compactNonEmptyStrings(result.RepairHints)
	return result
}

func subtractStringMultiset(after []string, before []string) []string {
	counts := make(map[string]int, len(before))
	for _, item := range before {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		counts[trimmed]++
	}

	var out []string
	for _, item := range after {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if counts[trimmed] > 0 {
			counts[trimmed]--
			continue
		}
		out = append(out, trimmed)
	}
	return compactNonEmptyStrings(out)
}

func emitCanaryProbeTelemetry(pageURL string, result *CanaryResult) {
	if result == nil {
		return
	}
	payload := canaryProbeTelemetry{
		PageURL:                        strings.TrimSpace(pageURL),
		Skipped:                        result.Skipped,
		Clicked:                        result.Clicked,
		VisibleControls:                result.VisibleControls,
		PostInteractionVisibleControls: result.PostInteractionVisibleControls,
		PostInteractionChecked:         result.PostInteractionChecked,
		PostInteractionHealthy:         result.PostInteractionHealthy,
		ErrorCount:                     len(result.Errors),
		DurationMS:                     result.Duration.Milliseconds(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	log.Printf("[canary_probe] %s", string(data))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
