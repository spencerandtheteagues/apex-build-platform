package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// CanaryResult captures advisory interaction coverage for a preview page.
// It never turns a passing preview into a hard failure by itself.
type CanaryResult struct {
	Clicked     int
	Errors      []string
	RepairHints []string
	Duration    time.Duration
	Skipped     bool
}

// CanaryTester exercises a small set of interactive controls in headless Chrome
// to catch "loads but unusable" regressions before users do.
type CanaryTester struct {
	chromePath string
}

func NewCanaryTester() *CanaryTester {
	return &CanaryTester{chromePath: findChrome()}
}

func (ct *CanaryTester) Available() bool {
	return ct != nil && ct.chromePath != ""
}

const canaryHarnessJS = `JSON.stringify((function() {
  var clickErrors = [];
  var clicked = 0;
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

  var runtimeErrors = [];
  if (Array.isArray(window.__apexRTErrors)) {
    runtimeErrors = runtimeErrors.concat(window.__apexRTErrors.map(String));
  }
  if (Array.isArray(window.__apexRTUnhandled)) {
    runtimeErrors = runtimeErrors.concat(window.__apexRTUnhandled.map(String));
  }

  return {
    clicked: clicked,
    errors: clickErrors.concat(runtimeErrors)
  };
})())`

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

	var harnessJSON string
	err := chromedp.Run(tabCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(earlyInjection).Do(ctx)
			return err
		}),
		chromedp.Navigate(pageURL),
		chromedp.Sleep(1100*time.Millisecond),
		chromedp.Evaluate(canaryHarnessJS, &harnessJSON),
	)
	if err != nil {
		return &CanaryResult{
			Skipped:     true,
			Duration:    time.Since(start),
			Errors:      []string{fmt.Sprintf("canary navigation failed: %v", err)},
			RepairHints: []string{"Review the first interactive route after mount and ensure the preview remains stable after initial user interactions."},
		}
	}

	var payload struct {
		Clicked int      `json:"clicked"`
		Errors  []string `json:"errors"`
	}
	_ = json.Unmarshal([]byte(harnessJSON), &payload)
	payload.Errors = filterBrowserNoise(payload.Errors)

	result := &CanaryResult{
		Clicked:  payload.Clicked,
		Errors:   payload.Errors,
		Duration: time.Since(start),
	}
	if len(payload.Errors) > 0 {
		result.RepairHints = []string{
			"Fix the runtime error(s) triggered by basic interactions such as button clicks, links, or form controls.",
			fmt.Sprintf("Stabilize the UI after mount so at least the first %d interactive control(s) can be exercised without JS exceptions.", payload.Clicked),
		}
	}
	if payload.Clicked == 0 {
		result.RepairHints = append(result.RepairHints,
			"The preview contains no usable interactive controls. Ensure the first screen exposes at least one obvious primary action or navigational control.",
		)
	}
	for i := range result.RepairHints {
		result.RepairHints[i] = strings.TrimSpace(result.RepairHints[i])
	}
	return result
}
