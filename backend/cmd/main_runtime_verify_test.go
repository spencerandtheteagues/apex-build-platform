package main

import "testing"

func TestResolvePreviewRuntimeVerify(t *testing.T) {
	resetEnv := func(t *testing.T) {
		t.Helper()
		t.Setenv("GO_ENV", "")
		t.Setenv("APEX_ENV", "")
		t.Setenv("ENVIRONMENT", "")
		t.Setenv("ENV", "")
		t.Setenv("APEX_PREVIEW_RUNTIME_VERIFY", "")
	}

	t.Run("explicit enable wins outside production", func(t *testing.T) {
		resetEnv(t)
		t.Setenv("GO_ENV", "development")
		t.Setenv("APEX_PREVIEW_RUNTIME_VERIFY", "true")

		enabled, mode := resolvePreviewRuntimeVerify("/usr/bin/chromium")
		if !enabled || mode != "explicit_enabled" {
			t.Fatalf("resolvePreviewRuntimeVerify() = (%v, %q), want (true, %q)", enabled, mode, "explicit_enabled")
		}
	})

	t.Run("explicit disable wins in production", func(t *testing.T) {
		resetEnv(t)
		t.Setenv("GO_ENV", "production")
		t.Setenv("APEX_PREVIEW_RUNTIME_VERIFY", "false")

		enabled, mode := resolvePreviewRuntimeVerify("/usr/bin/chromium")
		if enabled || mode != "explicit_disabled" {
			t.Fatalf("resolvePreviewRuntimeVerify() = (%v, %q), want (false, %q)", enabled, mode, "explicit_disabled")
		}
	})

	t.Run("production defaults on when chrome is available", func(t *testing.T) {
		resetEnv(t)
		t.Setenv("GO_ENV", "production")

		enabled, mode := resolvePreviewRuntimeVerify("/usr/bin/chromium")
		if !enabled || mode != "production_default" {
			t.Fatalf("resolvePreviewRuntimeVerify() = (%v, %q), want (true, %q)", enabled, mode, "production_default")
		}
	})

	t.Run("production without chrome stays on so preview gate fails closed", func(t *testing.T) {
		resetEnv(t)
		t.Setenv("GO_ENV", "production")

		enabled, mode := resolvePreviewRuntimeVerify("")
		if !enabled || mode != "production_missing_browser" {
			t.Fatalf("resolvePreviewRuntimeVerify() = (%v, %q), want (true, %q)", enabled, mode, "production_missing_browser")
		}
	})

	t.Run("non production defaults off", func(t *testing.T) {
		resetEnv(t)
		t.Setenv("GO_ENV", "development")

		enabled, mode := resolvePreviewRuntimeVerify("/usr/bin/chromium")
		if enabled || mode != "non_production_default" {
			t.Fatalf("resolvePreviewRuntimeVerify() = (%v, %q), want (false, %q)", enabled, mode, "non_production_default")
		}
	})
}

func TestDefaultProductionTrustedProxiesIncludesCloudflareAndPrivateRanges(t *testing.T) {
	proxies := defaultProductionTrustedProxies()
	want := []string{"173.245.48.0/20", "2606:4700::/32", "10.0.0.0/8", "127.0.0.1"}
	for _, expected := range want {
		found := false
		for _, proxy := range proxies {
			if proxy == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected default trusted proxies to include %s", expected)
		}
	}
}

func TestSplitCSVTrimsEmptyValues(t *testing.T) {
	got := splitCSV(" 127.0.0.1, , 10.0.0.0/8 ")
	want := []string{"127.0.0.1", "10.0.0.0/8"}
	if len(got) != len(want) {
		t.Fatalf("splitCSV length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitCSV[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
