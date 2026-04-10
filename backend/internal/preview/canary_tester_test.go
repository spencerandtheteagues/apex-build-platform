package preview

import (
	"strings"
	"testing"
	"time"
)

func TestCanaryProbesEnabled(t *testing.T) {
	t.Setenv("APEX_CANARY_PROBES", "")
	if !canaryProbesEnabled() {
		t.Fatal("expected canary probes to default on")
	}

	for _, value := range []string{"false", "0", "no", "off"} {
		t.Setenv("APEX_CANARY_PROBES", value)
		if canaryProbesEnabled() {
			t.Fatalf("expected canary probes disabled for %q", value)
		}
	}

	for _, value := range []string{"true", "1", "yes", "on", "garbage"} {
		t.Setenv("APEX_CANARY_PROBES", value)
		if !canaryProbesEnabled() {
			t.Fatalf("expected canary probes enabled for %q", value)
		}
	}
}

func TestSubtractStringMultiset(t *testing.T) {
	after := []string{"boom", "boom", "after-only", "shared"}
	before := []string{"boom", "shared"}

	got := subtractStringMultiset(after, before)
	if len(got) != 2 {
		t.Fatalf("expected 2 residual values, got %v", got)
	}
	if got[0] != "boom" || got[1] != "after-only" {
		t.Fatalf("unexpected residual values: %v", got)
	}
}

func TestDeriveCanaryResultTracksPostInteractionFailure(t *testing.T) {
	result := deriveCanaryResult(
		canaryInteractionPayload{
			Clicked:         2,
			VisibleControls: 3,
			Errors:          []string{"handler exploded"},
		},
		canaryMountPayload{
			Selector:    "#root",
			VisibleText: 0,
			HasContent:  false,
		},
		0,
		nil,
		[]string{"handler exploded", "new runtime error"},
		1200*time.Millisecond,
	)

	if !result.PostInteractionChecked {
		t.Fatal("expected settle check metadata")
	}
	if result.PostInteractionHealthy {
		t.Fatalf("expected unhealthy post-interaction result, got %+v", result)
	}
	if result.VisibleControls != 3 || result.PostInteractionVisibleControls != 0 {
		t.Fatalf("unexpected control counts: %+v", result)
	}
	if len(result.Errors) != 3 {
		t.Fatalf("expected 3 unique canary errors, got %v", result.Errors)
	}
	if !containsExactString(result.Errors, "post-click settle check failed: #root stopped rendering after interactions") {
		t.Fatalf("expected settle failure error, got %v", result.Errors)
	}
	if !containsSubstring(result.RepairHints, "Keep the first rendered screen stable after the initial click path") {
		t.Fatalf("expected settle repair hint, got %v", result.RepairHints)
	}
}

func TestDeriveCanaryResultIgnoresBaselineRuntimeNoise(t *testing.T) {
	result := deriveCanaryResult(
		canaryInteractionPayload{
			Clicked:         1,
			VisibleControls: 1,
		},
		canaryMountPayload{
			Selector:    "#root",
			VisibleText: 64,
			HasContent:  true,
		},
		1,
		[]string{"ResizeObserver loop limit exceeded", "shared runtime error"},
		[]string{"ResizeObserver loop limit exceeded", "shared runtime error"},
		500*time.Millisecond,
	)

	if len(result.Errors) != 0 {
		t.Fatalf("expected baseline-only runtime errors to be ignored, got %v", result.Errors)
	}
	if !result.PostInteractionHealthy {
		t.Fatalf("expected healthy post-interaction result, got %+v", result)
	}
}

func TestDeriveCanaryResultHintsOnZeroVisibleControls(t *testing.T) {
	result := deriveCanaryResult(
		canaryInteractionPayload{},
		canaryMountPayload{
			Selector:    "#root",
			VisibleText: 40,
			HasContent:  true,
		},
		0,
		nil,
		nil,
		250*time.Millisecond,
	)

	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors for a stable but control-less preview, got %v", result.Errors)
	}
	if !containsSubstring(result.RepairHints, "exposes no visible buttons, links, menus, or toggles") {
		t.Fatalf("expected zero-control hint, got %v", result.RepairHints)
	}
}

func containsExactString(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}

func containsSubstring(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(strings.TrimSpace(value), want) {
			return true
		}
	}
	return false
}
