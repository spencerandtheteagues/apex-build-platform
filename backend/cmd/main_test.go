package main

import (
	"strings"
	"testing"
	"time"

	"apex-build/internal/payments"
)

func TestPreviewRuntimeVerificationEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		environment string
		setting     string
		chromePath  string
		want        bool
	}{
		{name: "explicit true wins", environment: "development", setting: "true", chromePath: "", want: true},
		{name: "explicit false wins", environment: "production", setting: "false", chromePath: "/usr/bin/chromium-browser", want: false},
		{name: "production defaults on when chrome available", environment: "production", setting: "", chromePath: "/usr/bin/chromium-browser", want: true},
		{name: "production stays off without chrome", environment: "production", setting: "", chromePath: "", want: false},
		{name: "development default stays off", environment: "development", setting: "", chromePath: "/usr/bin/chromium-browser", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := previewRuntimeVerificationEnabled(tc.environment, tc.setting, tc.chromePath); got != tc.want {
				t.Fatalf("previewRuntimeVerificationEnabled(%q, %q, %q) = %v, want %v", tc.environment, tc.setting, tc.chromePath, got, tc.want)
			}
		})
	}
}

func TestFormatConfiguredPlansForLogUsesPaymentPlanTruth(t *testing.T) {
	got := formatConfiguredPlansForLog(payments.GetAllPlans())

	for _, want := range []string{
		"Free ($0/mo)",
		"Builder ($24/mo)",
		"Pro ($59/mo)",
		"Team ($149/mo)",
		"Enterprise (contact sales)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("plan log missing %q in %q", want, got)
		}
	}
	for _, stale := range []string{"Builder ($19/mo)", "Pro ($49/mo)", "Pro ($79/mo)", "Team ($99/mo)"} {
		if strings.Contains(got, stale) {
			t.Fatalf("plan log contains stale price %q in %q", stale, got)
		}
	}
}

func TestExecutionLaunchReadinessDetailsFlagsMissingRequiredSandbox(t *testing.T) {
	t.Setenv("E2B_API_KEY", "")
	t.Setenv("APEX_EXECUTION_DOCKER_HOST", "")
	t.Setenv("APEX_EXECUTION_DOCKER_SOCKET", "")
	t.Setenv("EXECUTION_DOCKER_SOCKET", "")
	t.Setenv("APEX_EXECUTION_DOCKER_CONTEXT", "")
	t.Setenv("APEX_PREVIEW_DOCKER_HOST", "")
	t.Setenv("APEX_PREVIEW_DOCKER_CONTEXT", "")
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_CONTEXT", "")

	details := executionLaunchReadinessDetails(map[string]interface{}{
		"execution_enabled":    false,
		"container_required":   true,
		"container_available":  false,
		"e2b_available":        false,
		"sandbox_v2_available": false,
	}, true)

	if details["launch_ready"] != false {
		t.Fatalf("launch_ready = %v, want false", details["launch_ready"])
	}
	if !stringSliceContains(details["issues"], "required_execution_sandbox_unavailable") {
		t.Fatalf("issues = %#v, want required_execution_sandbox_unavailable", details["issues"])
	}
	if !stringSliceContains(details["missing_env"], "E2B_API_KEY") ||
		!stringSliceContains(details["missing_env"], "APEX_EXECUTION_DOCKER_HOST") {
		t.Fatalf("missing_env = %#v, want E2B_API_KEY and APEX_EXECUTION_DOCKER_HOST", details["missing_env"])
	}
}

func TestExecutionLaunchReadinessDetailsAllowsManagedE2B(t *testing.T) {
	t.Setenv("E2B_API_KEY", "configured")

	details := executionLaunchReadinessDetails(map[string]interface{}{
		"execution_enabled":    true,
		"container_required":   false,
		"container_available":  true,
		"e2b_available":        true,
		"sandbox_v2_available": false,
	}, true)

	if details["launch_ready"] != true {
		t.Fatalf("launch_ready = %v, want true", details["launch_ready"])
	}
	if _, ok := details["issues"]; ok {
		t.Fatalf("issues = %#v, want none", details["issues"])
	}
}

func TestPreviewLaunchReadinessDetailsFlagsSandboxFallback(t *testing.T) {
	t.Setenv("APEX_PREVIEW_BACKEND_RUNTIME", "container")
	t.Setenv("APEX_PREVIEW_DOCKER_HOST", "")
	t.Setenv("APEX_PREVIEW_DOCKER_SOCKET", "")
	t.Setenv("APEX_PREVIEW_DOCKER_CONTEXT", "")
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_CONTEXT", "")

	details := previewLaunchReadinessDetails(map[string]interface{}{
		"sandbox_required": true,
		"sandbox_ready":    true,
		"sandbox_degraded": true,
		"server_runner": map[string]interface{}{
			"available": true,
			"runtime":   "host",
		},
	}, true)

	if details["launch_ready"] != false {
		t.Fatalf("launch_ready = %v, want false", details["launch_ready"])
	}
	if !stringSliceContains(details["issues"], "preview_sandbox_fallback") ||
		!stringSliceContains(details["issues"], "preview_backend_container_runtime_unavailable") {
		t.Fatalf("issues = %#v, want fallback and backend runtime issues", details["issues"])
	}
	if !stringSliceContains(details["missing_env"], "APEX_PREVIEW_DOCKER_HOST") {
		t.Fatalf("missing_env = %#v, want APEX_PREVIEW_DOCKER_HOST", details["missing_env"])
	}
	serverRunner, ok := details["server_runner"].(map[string]interface{})
	if !ok || serverRunner["reason"] == "" {
		t.Fatalf("server_runner = %#v, want fallback reason", details["server_runner"])
	}
}

func TestPreviewLaunchReadinessDetailsAllowsContainerRuntime(t *testing.T) {
	t.Setenv("APEX_PREVIEW_BACKEND_RUNTIME", "container")
	t.Setenv("APEX_PREVIEW_DOCKER_HOST", "ssh://apex-preview-runner")

	details := previewLaunchReadinessDetails(map[string]interface{}{
		"sandbox_required": true,
		"sandbox_ready":    true,
		"sandbox_degraded": false,
		"server_runner": map[string]interface{}{
			"available": true,
			"runtime":   "container",
		},
	}, true)

	if details["launch_ready"] != true {
		t.Fatalf("launch_ready = %v, want true", details["launch_ready"])
	}
	if _, ok := details["issues"]; ok {
		t.Fatalf("issues = %#v, want none", details["issues"])
	}
}

func stringSliceContains(value any, want string) bool {
	switch items := value.(type) {
	case []string:
		for _, item := range items {
			if item == want {
				return true
			}
		}
	case []any:
		for _, item := range items {
			if item == want {
				return true
			}
		}
	}
	return false
}

func TestAdminPromotionGuardRequiresStrongShortLivedTokenInProduction(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	validToken := strings.Repeat("a", adminPromotionTokenMinLength)
	validDigest := adminPromotionTokenDigest(validToken)

	tests := []struct {
		name      string
		env       string
		token     string
		expiresAt string
		digest    string // ADMIN_PROMOTE_TOKEN_DIGEST env value
		wantErr   bool
	}{
		{name: "development allows unguarded local bootstrap", env: "development", wantErr: false},
		{name: "production rejects missing token", env: "production", expiresAt: now.Add(time.Hour).Format(time.RFC3339), wantErr: true},
		{name: "production rejects short token", env: "production", token: "short", expiresAt: now.Add(time.Hour).Format(time.RFC3339), wantErr: true},
		{name: "production rejects missing digest", env: "production", token: validToken, expiresAt: now.Add(time.Hour).Format(time.RFC3339), digest: "", wantErr: true},
		{name: "production rejects digest mismatch", env: "production", token: validToken, expiresAt: now.Add(time.Hour).Format(time.RFC3339), digest: strings.Repeat("0", 64), wantErr: true},
		{name: "production rejects missing expiry", env: "production", token: validToken, digest: validDigest, wantErr: true},
		{name: "production rejects expired expiry", env: "production", token: validToken, digest: validDigest, expiresAt: now.Add(-time.Minute).Format(time.RFC3339), wantErr: true},
		{name: "production rejects long-lived expiry", env: "production", token: validToken, digest: validDigest, expiresAt: now.Add(25 * time.Hour).Format(time.RFC3339), wantErr: true},
		{name: "production accepts guarded short-lived bootstrap", env: "production", token: validToken, digest: validDigest, expiresAt: now.Add(time.Hour).Format(time.RFC3339), wantErr: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ADMIN_PROMOTE_TOKEN_DIGEST", tc.digest)
			err := validateAdminPromotionGuard(tc.env, tc.token, tc.expiresAt, now)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateAdminPromotionGuard() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestParseAdminPromotionEmailsNormalizesAndDeduplicates(t *testing.T) {
	got := parseAdminPromotionEmails(" Admin@Example.com, admin@example.com, owner@example.com ,, ")
	want := []string{"admin@example.com", "owner@example.com"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
