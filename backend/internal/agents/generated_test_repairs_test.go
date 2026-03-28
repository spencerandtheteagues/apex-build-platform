package agents

import (
	"strings"
	"testing"
)

// ─── isGeneratedTestPath ──────────────────────────────────────────────────────

func TestIsGeneratedTestPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"src/__tests__/AppShell.test.tsx", true},
		{"src/__tests__/Button.spec.ts", true},
		{"src/components/Button.test.tsx", true},
		{"src/components/Button.spec.jsx", true},
		{"src/utils/helpers.test.js", true},
		{"__specs__/App.test.ts", true},

		// Non-test files must NOT match.
		{"src/components/Button.tsx", false},
		{"src/App.tsx", false},
		{"src/index.ts", false},
		{"backend/main.go", false},
		{"", false},
	}
	for _, tc := range cases {
		got := isGeneratedTestPath(tc.path)
		if got != tc.want {
			t.Errorf("isGeneratedTestPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// ─── ParseTestFileErrors ──────────────────────────────────────────────────────

const exactErrorFromIssue = `
src/__tests__/AppShell.test.tsx(2,18): error TS2305: Module '"@testing-library/react"' has no exported member 'screen'.
`

func TestParseTestFileErrors_RTLScreenMissing(t *testing.T) {
	errs := ParseTestFileErrors(exactErrorFromIssue)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %+v", len(errs), errs)
	}
	e := errs[0]
	if e.FilePath != "src/__tests__/AppShell.test.tsx" {
		t.Errorf("FilePath = %q, want src/__tests__/AppShell.test.tsx", e.FilePath)
	}
	if e.Line != 2 {
		t.Errorf("Line = %d, want 2", e.Line)
	}
	if e.Col != 18 {
		t.Errorf("Col = %d, want 18", e.Col)
	}
	if e.Code != "TS2305" {
		t.Errorf("Code = %q, want TS2305", e.Code)
	}
	if !strings.Contains(e.Message, "screen") {
		t.Errorf("Message should mention 'screen', got %q", e.Message)
	}
}

func TestParseTestFileErrors_ColonFormat(t *testing.T) {
	// Some Vite versions emit path:line:col format.
	input := "src/__tests__/Foo.test.ts:5:10 - error TS2304: Cannot find name 'describe'.\n"
	errs := ParseTestFileErrors(input)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Line != 5 || errs[0].Col != 10 {
		t.Errorf("Line/Col = %d/%d, want 5/10", errs[0].Line, errs[0].Col)
	}
}

func TestParseTestFileErrors_IgnoresNonTestFiles(t *testing.T) {
	// Errors in source files should be ignored.
	input := `
src/components/Button.tsx(10,5): error TS2345: Argument of type 'string' is not assignable.
src/__tests__/Button.test.tsx(3,1): error TS2305: Module '"@testing-library/react"' has no exported member 'screen'.
`
	errs := ParseTestFileErrors(input)
	if len(errs) != 1 {
		t.Fatalf("should only return test-file error; got %d errors", len(errs))
	}
	if errs[0].FilePath != "src/__tests__/Button.test.tsx" {
		t.Errorf("unexpected path %q", errs[0].FilePath)
	}
}

func TestParseTestFileErrors_Deduplication(t *testing.T) {
	input := `
src/__tests__/App.test.tsx(2,18): error TS2305: Module '"@testing-library/react"' has no exported member 'screen'.
src/__tests__/App.test.tsx(2,18): error TS2305: Module '"@testing-library/react"' has no exported member 'screen'.
`
	errs := ParseTestFileErrors(input)
	if len(errs) != 1 {
		t.Errorf("expected 1 deduplicated error, got %d", len(errs))
	}
}

func TestParseTestFileErrors_MultipleFiles(t *testing.T) {
	input := `
src/__tests__/App.test.tsx(2,18): error TS2305: Module '"@testing-library/react"' has no exported member 'screen'.
src/__tests__/Login.test.tsx(4,1): error TS2304: Cannot find name 'describe'.
`
	errs := ParseTestFileErrors(input)
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errs))
	}
}

func TestExtractBrokenTestPaths(t *testing.T) {
	input := `
src/__tests__/App.test.tsx(2,18): error TS2305: Module '"@testing-library/react"' has no exported member 'screen'.
src/__tests__/App.test.tsx(5,3): error TS2305: Module '"@testing-library/react"' has no exported member 'screen'.
src/__tests__/Login.test.tsx(4,1): error TS2304: Cannot find name 'describe'.
src/components/Header.tsx(10,5): error TS2345: Argument of type 'string' is not assignable.
`
	paths := ExtractBrokenTestPaths(input)
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d: %v", len(paths), paths)
	}
}

// ─── DetectTestFileFlaws ──────────────────────────────────────────────────────

func TestDetectTestFileFlaws_RTLScreen(t *testing.T) {
	errs := ParseTestFileErrors(exactErrorFromIssue)
	flaws := DetectTestFileFlaws(errs)
	if !containsFlaw(flaws, FlawRTLScreenMissing) {
		t.Errorf("expected FlawRTLScreenMissing, got %v", flaws)
	}
}

func TestDetectTestFileFlaws_NoFlaws(t *testing.T) {
	errs := []TestFileError{{
		FilePath: "src/__tests__/Foo.test.tsx",
		Code:     "TS2345",
		Message:  "Argument of type 'string' is not assignable to parameter of type 'number'.",
	}}
	flaws := DetectTestFileFlaws(errs)
	if len(flaws) != 0 {
		t.Errorf("expected no known flaws, got %v", flaws)
	}
}

// ─── DetectSourceFlaws ────────────────────────────────────────────────────────

func TestDetectSourceFlaws_EmptyFile(t *testing.T) {
	flaws := DetectSourceFlaws("   \n  ")
	if !containsFlaw(flaws, FlawEmptyFile) {
		t.Errorf("expected FlawEmptyFile, got %v", flaws)
	}
}

func TestDetectSourceFlaws_MissingTestFrameworkImport(t *testing.T) {
	content := `
describe('something', () => {
  it('works', () => {
    expect(true).toBe(true)
  })
})
`
	flaws := DetectSourceFlaws(content)
	if !containsFlaw(flaws, FlawMissingTestFramework) {
		t.Errorf("expected FlawMissingTestFramework, got %v", flaws)
	}
}

func TestDetectSourceFlaws_MissingReactImport(t *testing.T) {
	content := `
import { render } from '@testing-library/react'

describe('Button', () => {
  it('renders', () => {
    render(<Button label="hi" />)
  })
})
`
	flaws := DetectSourceFlaws(content)
	if !containsFlaw(flaws, FlawMissingReactImport) {
		t.Errorf("expected FlawMissingReactImport, got %v", flaws)
	}
}

func TestDetectSourceFlaws_CleanFile(t *testing.T) {
	content := `
import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import App from '../App'

describe('App', () => {
  it('renders', () => {
    const { container } = render(<App />)
    expect(container).toBeTruthy()
  })
})
`
	flaws := DetectSourceFlaws(content)
	if len(flaws) != 0 {
		t.Errorf("expected no flaws for clean file, got %v", flaws)
	}
}

// ─── RepairGeneratedTestFile — patch imports strategy ─────────────────────────

func TestRepairGeneratedTestFile_PatchesRTLScreen(t *testing.T) {
	// The exact content that triggered the live canary failure.
	brokenContent := `import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import AppShell from '../AppShell'

describe('AppShell', () => {
  it('renders without crashing', () => {
    render(<AppShell />)
    expect(screen.getByRole('main')).toBeTruthy()
  })
})
`
	errs := ParseTestFileErrors(exactErrorFromIssue)
	repair := RepairGeneratedTestFile("src/__tests__/AppShell.test.tsx", brokenContent, errs)

	if repair.Strategy != StrategyPatchedImports {
		t.Errorf("expected StrategyPatchedImports, got %s", repair.Strategy)
	}
	// The repaired file must NOT import screen from @testing-library/react.
	if strings.Contains(repair.RepairedContent, `{ render, screen } from '@testing-library/react'`) {
		t.Error("repaired content still imports screen from @testing-library/react")
	}
	// Must import screen from @testing-library/dom instead.
	if !strings.Contains(repair.RepairedContent, `@testing-library/dom`) {
		t.Error("repaired content should import screen from @testing-library/dom")
	}
	// Test logic must be preserved.
	if !strings.Contains(repair.RepairedContent, "renders without crashing") {
		t.Error("repaired content should preserve the test description")
	}
}

func TestRepairGeneratedTestFile_PatchesMissingVitestImport(t *testing.T) {
	brokenContent := `import { render } from '@testing-library/react'
import Button from '../Button'

describe('Button', () => {
  it('renders', () => {
    render(<Button />)
  })
})
`
	repair := RepairGeneratedTestFile("src/__tests__/Button.test.tsx", brokenContent, nil)

	if repair.Strategy != StrategyPatchedImports {
		t.Errorf("expected StrategyPatchedImports for missing vitest import, got %s", repair.Strategy)
	}
	if !strings.Contains(repair.RepairedContent, "from 'vitest'") {
		t.Error("patched content should add vitest imports")
	}
}

// ─── RepairGeneratedTestFile — minimal smoke strategy ─────────────────────────

func TestRepairGeneratedTestFile_MinimalSmoke_WhenPatchNotPossible(t *testing.T) {
	// A file so broken that import patching cannot fix it, but component name is
	// clearly inferrable from the file path.
	brokenContent := `TOTALLY BROKEN CONTENT @@#$%`
	repair := RepairGeneratedTestFile("src/__tests__/Dashboard.test.tsx", brokenContent, nil)

	if repair.Strategy != StrategyMinimalSmoke && repair.Strategy != StrategyPlaceholder {
		t.Errorf("expected MinimalSmoke or Placeholder, got %s", repair.Strategy)
	}
	if repair.Strategy == StrategyMinimalSmoke {
		if !strings.Contains(repair.RepairedContent, "Dashboard") {
			t.Error("minimal smoke should reference the Dashboard component")
		}
		if !strings.Contains(repair.RepairedContent, "renders without crashing") {
			t.Error("minimal smoke should have a 'renders without crashing' test")
		}
	}
}

func TestRepairGeneratedTestFile_MinimalSmoke_ExtractsComponentFromImport(t *testing.T) {
	// Has a recognisable default import.
	brokenContent := `import Sidebar from '../Sidebar'
// everything else is broken
@@#$%
`
	repair := RepairGeneratedTestFile("src/__tests__/Sidebar.test.tsx", brokenContent, nil)

	if repair.Strategy == StrategyMinimalSmoke {
		if !strings.Contains(repair.RepairedContent, "Sidebar") {
			t.Error("smoke test should reference Sidebar")
		}
	}
}

// ─── RepairGeneratedTestFile — placeholder fallback ───────────────────────────

func TestRepairGeneratedTestFile_Placeholder_WhenNoComponentInferable(t *testing.T) {
	// A file with a lowercase name — not a component.
	brokenContent := `GARBAGE`
	repair := RepairGeneratedTestFile("src/__tests__/utils.test.ts", brokenContent, nil)

	// utils.test.ts has a lowercase component name, so smoke fails → placeholder.
	if repair.Strategy != StrategyPlaceholder {
		// Acceptable if smoke also worked for some other reason.
		if repair.Strategy != StrategyMinimalSmoke {
			t.Errorf("expected Placeholder (or MinimalSmoke), got %s", repair.Strategy)
		}
	}
	if !strings.Contains(repair.RepairedContent, "describe") {
		t.Error("placeholder should contain a describe block")
	}
	if !strings.Contains(repair.RepairedContent, "expect(true).toBe(true)") {
		t.Error("placeholder should contain a trivially-passing assertion")
	}
}

func TestRepairGeneratedTestFile_PlaceholderAlwaysCompiles(t *testing.T) {
	placeholder := compileSafePlaceholder("src/__tests__/SomeComponent.test.tsx")
	// Must contain valid TS constructs.
	if !strings.Contains(placeholder, "import { describe, it, expect } from 'vitest'") {
		t.Error("placeholder must import from vitest")
	}
	if !strings.Contains(placeholder, "expect(true).toBe(true)") {
		t.Error("placeholder must have a passing assertion")
	}
}

// ─── RepairAll ────────────────────────────────────────────────────────────────

func TestRepairAll_RepairsMultipleFiles(t *testing.T) {
	input := `
src/__tests__/App.test.tsx(2,18): error TS2305: Module '"@testing-library/react"' has no exported member 'screen'.
src/__tests__/Login.test.tsx(4,1): error TS2304: Cannot find name 'describe'.
`
	contents := map[string]string{
		"src/__tests__/App.test.tsx": `import { render, screen } from '@testing-library/react'
import App from '../App'
describe('App', () => { it('x', () => { screen.debug() }) })`,
		"src/__tests__/Login.test.tsx": `import { render } from '@testing-library/react'
import Login from '../Login'
describe('Login', () => { it('x', () => { render(<Login />) }) })`,
	}
	repairs := RepairAll(input, contents)
	if len(repairs) != 2 {
		t.Fatalf("expected 2 repairs, got %d", len(repairs))
	}
	for _, r := range repairs {
		if r.RepairedContent == "" {
			t.Errorf("repair for %q has empty content", r.FilePath)
		}
	}
}

func TestRepairAll_IgnoresNonTestFiles(t *testing.T) {
	input := `
src/components/Button.tsx(10,5): error TS2345: Argument.
src/__tests__/Button.test.tsx(3,1): error TS2305: Module '"@testing-library/react"' has no exported member 'screen'.
`
	repairs := RepairAll(input, map[string]string{})
	// Only the test file error should produce a repair.
	if len(repairs) != 1 {
		t.Errorf("expected 1 repair (only test file), got %d", len(repairs))
	}
	if repairs[0].FilePath != "src/__tests__/Button.test.tsx" {
		t.Errorf("wrong file repaired: %q", repairs[0].FilePath)
	}
}

func TestRepairAll_MissingContentProducesPlaceholder(t *testing.T) {
	input := `src/__tests__/Ghost.test.tsx(1,1): error TS2305: Module '"@testing-library/react"' has no exported member 'screen'.`
	repairs := RepairAll(input, map[string]string{}) // no content provided
	if len(repairs) != 1 {
		t.Fatalf("expected 1 repair, got %d", len(repairs))
	}
	r := repairs[0]
	if r.RepairedContent == "" {
		t.Error("repair content must not be empty even when original is missing")
	}
	// With no content there's nothing to patch — should be placeholder or smoke.
	if r.Strategy != StrategyPlaceholder && r.Strategy != StrategyMinimalSmoke {
		t.Errorf("expected Placeholder or MinimalSmoke for missing content, got %s", r.Strategy)
	}
}

// ─── inferComponentName / inferComponentImportPath ───────────────────────────

func TestInferComponentName_FromFilePath(t *testing.T) {
	cases := []struct {
		path    string
		content string
		want    string
	}{
		{"src/__tests__/AppShell.test.tsx", "", "AppShell"},
		{"src/components/Button.test.tsx", "", "Button"},
		{"src/__tests__/utils.test.ts", "", ""},     // lowercase → no component
		{"src/__tests__/index.test.ts", "", ""},     // lowercase
	}
	for _, tc := range cases {
		got := inferComponentName(tc.path, tc.content)
		if got != tc.want {
			t.Errorf("inferComponentName(%q, _) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestInferComponentName_PrefersImportOverFileName(t *testing.T) {
	content := `import MyWidget from '../MyWidget'`
	got := inferComponentName("src/__tests__/something.test.tsx", content)
	if got != "MyWidget" {
		t.Errorf("expected MyWidget from import, got %q", got)
	}
}

func TestInferComponentImportPath(t *testing.T) {
	cases := []struct {
		testPath string
		want     string
	}{
		{"src/__tests__/AppShell.test.tsx", "../AppShell"},
		{"src/components/Button.test.tsx", "./Button"},
		{"src/__specs__/Login.spec.tsx", "../Login"},
	}
	for _, tc := range cases {
		got := inferComponentImportPath(tc.testPath)
		if got != tc.want {
			t.Errorf("inferComponentImportPath(%q) = %q, want %q", tc.testPath, got, tc.want)
		}
	}
}

// ─── removeImportMember ───────────────────────────────────────────────────────

func TestRemoveImportMember(t *testing.T) {
	cases := []struct {
		members string
		remove  string
		want    string
	}{
		{"render, screen, fireEvent", "screen", "render, fireEvent"},
		{"screen", "screen", ""},
		{"render, fireEvent", "screen", "render, fireEvent"}, // not present
		{"render, screen", "screen", "render"},
		{"screen, render", "screen", "render"},
	}
	for _, tc := range cases {
		got := removeImportMember(tc.members, tc.remove)
		if got != tc.want {
			t.Errorf("removeImportMember(%q, %q) = %q, want %q", tc.members, tc.remove, got, tc.want)
		}
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func containsFlaw(flaws []TestFileFlaw, target TestFileFlaw) bool {
	for _, f := range flaws {
		if f == target {
			return true
		}
	}
	return false
}
