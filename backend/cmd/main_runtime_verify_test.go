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

	t.Run("production without chrome stays off", func(t *testing.T) {
		resetEnv(t)
		t.Setenv("GO_ENV", "production")

		enabled, mode := resolvePreviewRuntimeVerify("")
		if enabled || mode != "production_no_chrome" {
			t.Fatalf("resolvePreviewRuntimeVerify() = (%v, %q), want (false, %q)", enabled, mode, "production_no_chrome")
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
