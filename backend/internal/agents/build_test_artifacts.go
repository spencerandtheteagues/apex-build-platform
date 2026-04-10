// build_test_artifacts.go — Structured test artifact summary emission.
//
// Emits a [test_artifacts] JSON log line at build completion alongside the
// [quality_telemetry] line from build_telemetry.go. Captures what tests were
// generated, which frameworks are in use, and what coverage classes are present.
// Machine-parseable by any log aggregator.
package agents

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TestArtifactSummary describes the set of test files produced by a build.
// All boolean fields reflect detected usage patterns — no compile/run required.
type TestArtifactSummary struct {
	BuildID  string `json:"build_id"`
	UserID   uint   `json:"user_id,omitempty"`

	// Counts.
	TestFileCount     int `json:"test_file_count"`
	ComponentTestCount int `json:"component_test_count,omitempty"`
	SmokeTestCount    int `json:"smoke_test_count,omitempty"`

	// Framework detection — inferred from imports in generated files.
	FrameworkVitest         bool `json:"framework_vitest,omitempty"`
	FrameworkTestingLibrary bool `json:"framework_testing_library,omitempty"`
	FrameworkPlaywright     bool `json:"framework_playwright,omitempty"`

	// Coverage class signals.
	HasSmokeTest      bool `json:"has_smoke_test,omitempty"`      // any e2e/*.spec.* present
	HasComponentTests bool `json:"has_component_tests,omitempty"` // any __tests__/*.test.tsx
	HasContractTest   bool `json:"has_contract_test,omitempty"`   // API contract drift checks

	// Delivery metadata.
	SkippedForPreviewOnly bool `json:"skipped_for_preview_only,omitempty"`
	SkippedByFlag         bool `json:"skipped_by_flag,omitempty"`

	GeneratedAt time.Time `json:"generated_at"`
}

// emitTestArtifactSummary derives and emits a [test_artifacts] log line.
// Safe to call with a nil build or empty allFiles.
func emitTestArtifactSummary(build *Build, allFiles []GeneratedFile, now time.Time) {
	s := deriveTestArtifactSummary(build, allFiles, now)
	if s == nil {
		return
	}
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	log.Printf("[test_artifacts] %s", string(data))
}

func deriveTestArtifactSummary(build *Build, allFiles []GeneratedFile, now time.Time) *TestArtifactSummary {
	if build == nil {
		return nil
	}

	build.mu.RLock()
	buildID := build.ID
	userID := build.UserID
	isPreviewOnly := buildUsesFrontendPreviewOnlyDeliveryLocked(build)
	build.mu.RUnlock()

	s := &TestArtifactSummary{
		BuildID:     buildID,
		UserID:      userID,
		GeneratedAt: now.UTC(),
	}

	if !testGenerationEnabled() {
		s.SkippedByFlag = true
		return s
	}
	if isPreviewOnly {
		s.SkippedForPreviewOnly = true
		return s
	}

	for _, f := range allFiles {
		if !isTestFile(f.Path) {
			continue
		}
		s.TestFileCount++
		content := f.Content
		lower := strings.ToLower(filepath.ToSlash(f.Path))

		// Framework detection from imports.
		if containsTestFrameworkImport(content, "vitest") {
			s.FrameworkVitest = true
		}
		if containsTestFrameworkImport(content, "@testing-library") {
			s.FrameworkTestingLibrary = true
		}
		if containsTestFrameworkImport(content, "@playwright/test") ||
			isPlaywrightSpecPath(lower) {
			s.FrameworkPlaywright = true
		}

		// Coverage class detection.
		if isPlaywrightSpecPath(lower) {
			s.HasSmokeTest = true
			s.SmokeTestCount++
		}
		if isComponentTestPath(lower) {
			s.HasComponentTests = true
			s.ComponentTestCount++
		}
		if isContractTestContent(content) {
			s.HasContractTest = true
		}
	}

	return s
}

// testGenerationEnabled returns true unless APEX_TEST_GENERATION is explicitly
// set to a falsy value. Default is enabled.
func testGenerationEnabled() bool {
	val := strings.TrimSpace(strings.ToLower(os.Getenv("APEX_TEST_GENERATION")))
	if val == "" {
		return true
	}
	return val == "1" || val == "true" || val == "yes" || val == "on"
}

// buildUsesFrontendPreviewOnlyDeliveryLocked returns true when the build's
// delivery mode is frontend_preview_only. Requires build.mu.RLock to be held.
func buildUsesFrontendPreviewOnlyDeliveryLocked(build *Build) bool {
	if build == nil {
		return false
	}
	if build.Plan != nil {
		if mode := strings.TrimSpace(strings.ToLower(build.Plan.DeliveryMode)); mode == "frontend_preview_only" {
			return true
		}
	}
	if build.SnapshotState.RestoreContext != nil && build.SnapshotState.RestoreContext.Plan != nil {
		if mode := strings.TrimSpace(strings.ToLower(build.SnapshotState.RestoreContext.Plan.DeliveryMode)); mode == "frontend_preview_only" {
			return true
		}
	}
	if orch := build.SnapshotState.Orchestration; orch != nil && orch.BuildContract != nil {
		if mode := strings.TrimSpace(strings.ToLower(orch.BuildContract.DeliveryMode)); mode == "frontend_preview_only" {
			return true
		}
	}
	return false
}

func containsTestFrameworkImport(content, framework string) bool {
	return strings.Contains(content, `"`+framework) ||
		strings.Contains(content, `'`+framework)
}

func isPlaywrightSpecPath(lower string) bool {
	return (strings.Contains(lower, "e2e/") || strings.Contains(lower, "/e2e/")) &&
		(strings.Contains(lower, ".spec.") || strings.Contains(lower, ".test."))
}

func isComponentTestPath(lower string) bool {
	return (strings.Contains(lower, "__tests__/") || strings.Contains(lower, "/__tests__/")) &&
		(strings.Contains(lower, ".test.tsx") || strings.Contains(lower, ".test.ts") ||
			strings.Contains(lower, ".test.jsx") || strings.Contains(lower, ".test.js"))
}

func isContractTestContent(content string) bool {
	lower := strings.ToLower(content)
	return (strings.Contains(lower, "/api/") || strings.Contains(lower, "fetch(")) &&
		(strings.Contains(lower, "expect") || strings.Contains(lower, "assert")) &&
		(strings.Contains(lower, "route") || strings.Contains(lower, "endpoint") || strings.Contains(lower, "contract"))
}
