package agents

import (
	"strings"
	"testing"
	"time"
)

func makeBuildWithDeliveryMode(t *testing.T, mode string) *Build {
	t.Helper()
	build := &Build{
		ID:     "ta-test-" + t.Name(),
		UserID: 42,
		Plan:   &BuildPlan{DeliveryMode: mode},
	}
	return build
}

func TestDeriveTestArtifactSummaryCountsTestFiles(t *testing.T) {
	build := makeBuildWithDeliveryMode(t, "full_stack")
	allFiles := []GeneratedFile{
		{Path: "src/App.tsx", Content: "export default function App() {}"},
		{Path: "src/__tests__/App.test.tsx", Content: `import { describe, it, expect } from "vitest"; import { render } from "@testing-library/react";`},
		{Path: "src/__tests__/Button.test.tsx", Content: `import { describe, it } from "vitest";`},
		{Path: "e2e/smoke.spec.ts", Content: `import { test } from "@playwright/test";`},
	}

	s := deriveTestArtifactSummary(build, allFiles, time.Now())
	if s == nil {
		t.Fatal("expected non-nil summary")
	}
	if s.TestFileCount != 3 {
		t.Errorf("TestFileCount = %d, want 3", s.TestFileCount)
	}
}

func TestDeriveTestArtifactSummaryFrameworkDetection(t *testing.T) {
	build := makeBuildWithDeliveryMode(t, "full_stack")
	allFiles := []GeneratedFile{
		{Path: "src/__tests__/App.test.tsx", Content: `import { describe, it } from "vitest"; import { render } from "@testing-library/react";`},
		{Path: "e2e/smoke.spec.ts", Content: `import { test, expect } from "@playwright/test";`},
	}

	s := deriveTestArtifactSummary(build, allFiles, time.Now())
	if s == nil {
		t.Fatal("expected non-nil summary")
	}
	if !s.FrameworkVitest {
		t.Error("expected FrameworkVitest=true")
	}
	if !s.FrameworkTestingLibrary {
		t.Error("expected FrameworkTestingLibrary=true")
	}
	if !s.FrameworkPlaywright {
		t.Error("expected FrameworkPlaywright=true")
	}
}

func TestDeriveTestArtifactSummarySmokeVsComponentClassification(t *testing.T) {
	build := makeBuildWithDeliveryMode(t, "full_stack")
	allFiles := []GeneratedFile{
		{Path: "src/__tests__/App.test.tsx", Content: `import { it } from "vitest";`},
		{Path: "src/__tests__/Button.test.tsx", Content: `import { it } from "vitest";`},
		{Path: "e2e/main.spec.ts", Content: `import { test } from "@playwright/test";`},
	}

	s := deriveTestArtifactSummary(build, allFiles, time.Now())
	if s == nil {
		t.Fatal("expected non-nil summary")
	}
	if s.ComponentTestCount != 2 {
		t.Errorf("ComponentTestCount = %d, want 2", s.ComponentTestCount)
	}
	if s.SmokeTestCount != 1 {
		t.Errorf("SmokeTestCount = %d, want 1", s.SmokeTestCount)
	}
	if !s.HasSmokeTest {
		t.Error("expected HasSmokeTest=true")
	}
	if !s.HasComponentTests {
		t.Error("expected HasComponentTests=true")
	}
}

func TestDeriveTestArtifactSummaryContractTestDetection(t *testing.T) {
	build := makeBuildWithDeliveryMode(t, "full_stack")
	allFiles := []GeneratedFile{
		{Path: "src/__tests__/api.test.ts", Content: `
import { describe, it, expect } from "vitest";
it("frontend routes match backend contract", async () => {
  const resp = await fetch("/api/users");
  expect(resp.ok).toBe(true); // endpoint contract check
});`},
	}

	s := deriveTestArtifactSummary(build, allFiles, time.Now())
	if s == nil || !s.HasContractTest {
		t.Errorf("expected HasContractTest=true, got %+v", s)
	}
}

func TestDeriveTestArtifactSummarySkipsPreviewOnlyBuilds(t *testing.T) {
	build := makeBuildWithDeliveryMode(t, "frontend_preview_only")
	allFiles := []GeneratedFile{
		{Path: "src/__tests__/App.test.tsx", Content: `import { it } from "vitest";`},
	}

	s := deriveTestArtifactSummary(build, allFiles, time.Now())
	if s == nil {
		t.Fatal("expected non-nil summary even for preview-only")
	}
	if !s.SkippedForPreviewOnly {
		t.Error("expected SkippedForPreviewOnly=true for frontend_preview_only builds")
	}
	if s.TestFileCount != 0 {
		t.Errorf("expected TestFileCount=0 when skipped, got %d", s.TestFileCount)
	}
}

func TestDeriveTestArtifactSummarySkipsWhenFlagDisabled(t *testing.T) {
	t.Setenv("APEX_TEST_GENERATION", "false")
	build := makeBuildWithDeliveryMode(t, "full_stack")
	allFiles := []GeneratedFile{
		{Path: "src/__tests__/App.test.tsx", Content: `import { it } from "vitest";`},
	}

	s := deriveTestArtifactSummary(build, allFiles, time.Now())
	if s == nil {
		t.Fatal("expected non-nil summary even when flag disabled")
	}
	if !s.SkippedByFlag {
		t.Error("expected SkippedByFlag=true when APEX_TEST_GENERATION=false")
	}
	if s.TestFileCount != 0 {
		t.Errorf("expected TestFileCount=0 when flag disabled, got %d", s.TestFileCount)
	}
}

func TestTestGenerationEnabledDefault(t *testing.T) {
	t.Setenv("APEX_TEST_GENERATION", "")
	if !testGenerationEnabled() {
		t.Error("expected testGenerationEnabled=true when env var is unset")
	}
}

func TestTestGenerationEnabledFalsy(t *testing.T) {
	for _, val := range []string{"false", "0", "no", "off"} {
		t.Setenv("APEX_TEST_GENERATION", val)
		if testGenerationEnabled() {
			t.Errorf("expected testGenerationEnabled=false for %q", val)
		}
	}
}

func TestBuildExecutionPhasesForBuildSkipsTestsWhenFlagOff(t *testing.T) {
	t.Setenv("APEX_TEST_GENERATION", "false")
	manager := &AgentManager{
		builds:      make(map[string]*Build),
		subscribers: make(map[string][]chan *WSMessage),
	}

	testingAgent := &Agent{ID: "agent-test", Role: RoleTesting, BuildID: "b1"}
	build := &Build{
		ID:     "b1",
		Agents: map[string]*Agent{"agent-test": testingAgent},
		Plan:   &BuildPlan{DeliveryMode: "full_stack"},
	}

	_, phases := manager.buildExecutionPhasesForBuild(build)
	for _, p := range phases {
		if p.key == "integration" && len(p.agents) > 0 {
			t.Errorf("expected testing phase to be empty when APEX_TEST_GENERATION=false, got %d agents", len(p.agents))
		}
	}
}

func TestBuildExecutionPhasesForBuildSkipsTestsForPreviewOnly(t *testing.T) {
	t.Setenv("APEX_TEST_GENERATION", "true")
	manager := &AgentManager{
		builds:      make(map[string]*Build),
		subscribers: make(map[string][]chan *WSMessage),
	}

	testingAgent := &Agent{ID: "agent-test", Role: RoleTesting, BuildID: "b2"}
	build := &Build{
		ID:     "b2",
		Agents: map[string]*Agent{"agent-test": testingAgent},
		Plan:   &BuildPlan{DeliveryMode: "frontend_preview_only"},
	}

	_, phases := manager.buildExecutionPhasesForBuild(build)
	for _, p := range phases {
		if p.key == "integration" && len(p.agents) > 0 {
			t.Errorf("expected testing phase to be empty for frontend_preview_only, got %d agents", len(p.agents))
		}
	}
}

func TestBuildExecutionPhasesForBuildIncludesTestsWhenEnabled(t *testing.T) {
	t.Setenv("APEX_TEST_GENERATION", "true")
	manager := &AgentManager{
		builds:      make(map[string]*Build),
		subscribers: make(map[string][]chan *WSMessage),
	}

	testingAgent := &Agent{ID: "agent-test", Role: RoleTesting, BuildID: "b3"}
	build := &Build{
		ID:     "b3",
		Agents: map[string]*Agent{"agent-test": testingAgent},
		Plan:   &BuildPlan{DeliveryMode: "full_stack"},
	}

	_, phases := manager.buildExecutionPhasesForBuild(build)
	found := false
	for _, p := range phases {
		if p.key == "integration" && len(p.agents) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected testing agent in integration phase when flag is enabled and delivery mode is full_stack")
	}
}

func TestCountTestFiles(t *testing.T) {
	files := []GeneratedFile{
		{Path: "src/App.tsx"},
		{Path: "src/__tests__/App.test.tsx"},
		{Path: "e2e/smoke.spec.ts"},
		{Path: "src/utils.ts"},
		{Path: "tests/api.test.go"},
	}
	if got := countTestFiles(files); got != 3 {
		t.Errorf("countTestFiles = %d, want 3", got)
	}
}

func TestIsPlaywrightSpecPath(t *testing.T) {
	trueCases := []string{"e2e/smoke.spec.ts", "src/e2e/app.spec.tsx", "e2e/main.test.ts"}
	for _, p := range trueCases {
		if !isPlaywrightSpecPath(p) {
			t.Errorf("expected isPlaywrightSpecPath(%q) = true", p)
		}
	}
	falseCases := []string{"src/__tests__/App.test.tsx", "tests/unit.test.ts"}
	for _, p := range falseCases {
		if isPlaywrightSpecPath(p) {
			t.Errorf("expected isPlaywrightSpecPath(%q) = false", p)
		}
	}
}

func TestIsComponentTestPath(t *testing.T) {
	trueCases := []string{"src/__tests__/App.test.tsx", "components/__tests__/Button.test.ts"}
	for _, p := range trueCases {
		if !isComponentTestPath(p) {
			t.Errorf("expected isComponentTestPath(%q) = true", p)
		}
	}
}

func TestDeriveTestArtifactSummaryNilBuild(t *testing.T) {
	s := deriveTestArtifactSummary(nil, nil, time.Now())
	if s != nil {
		t.Errorf("expected nil summary for nil build, got %+v", s)
	}
}

func TestDeriveTestArtifactSummaryNoTestFiles(t *testing.T) {
	build := makeBuildWithDeliveryMode(t, "full_stack")
	allFiles := []GeneratedFile{
		{Path: "src/App.tsx", Content: "export default function App() {}"},
		{Path: "src/main.tsx", Content: "import React from 'react';"},
	}

	s := deriveTestArtifactSummary(build, allFiles, time.Now())
	if s == nil {
		t.Fatal("expected non-nil summary")
	}
	if s.TestFileCount != 0 {
		t.Errorf("TestFileCount = %d, want 0", s.TestFileCount)
	}
	if s.HasSmokeTest || s.HasComponentTests || s.HasContractTest {
		t.Errorf("expected all coverage flags false with no test files: %+v", s)
	}
}

func TestContainsTestFrameworkImport(t *testing.T) {
	content := `import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";`

	if !containsTestFrameworkImport(content, "vitest") {
		t.Error("expected vitest to be detected")
	}
	if !containsTestFrameworkImport(content, "@testing-library") {
		t.Error("expected @testing-library to be detected")
	}
	if containsTestFrameworkImport(content, "@playwright/test") {
		t.Error("expected @playwright/test NOT to be detected")
	}
}

func TestBuildUsesFrontendPreviewOnlyDeliveryLocked(t *testing.T) {
	cases := []struct {
		mode string
		want bool
	}{
		{"frontend_preview_only", true},
		{"full_stack", false},
		{"", false},
		{"FRONTEND_PREVIEW_ONLY", true}, // case insensitive
	}
	for _, tc := range cases {
		build := makeBuildWithDeliveryMode(t, tc.mode)
		got := buildUsesFrontendPreviewOnlyDeliveryLocked(build)
		if got != tc.want {
			t.Errorf("mode=%q: got %v, want %v", tc.mode, got, tc.want)
		}
	}
}

func TestDeriveTestArtifactSummaryBuildIDAndUserID(t *testing.T) {
	build := &Build{
		ID:     "build-xyz",
		UserID: 99,
		Plan:   &BuildPlan{DeliveryMode: "full_stack"},
	}
	s := deriveTestArtifactSummary(build, nil, time.Now())
	if s == nil {
		t.Fatal("expected non-nil summary")
	}
	if s.BuildID != "build-xyz" {
		t.Errorf("BuildID = %q, want %q", s.BuildID, "build-xyz")
	}
	if s.UserID != 99 {
		t.Errorf("UserID = %d, want 99", s.UserID)
	}
}

func TestEmitTestArtifactSummaryDoesNotPanic(t *testing.T) {
	// Just verify it does not panic on edge cases
	emitTestArtifactSummary(nil, nil, time.Now())

	build := makeBuildWithDeliveryMode(t, "full_stack")
	files := []GeneratedFile{
		{Path: "src/__tests__/App.test.tsx", Content: strings.Repeat("x", 1000)},
	}
	emitTestArtifactSummary(build, files, time.Now())
}
