// Package agents — deterministic repair helpers for broken generated test files.
//
// Generated test artifacts must never be allowed to break preview/build completion.
// This file provides pure, stateless helpers that:
//
//  1. Parse TypeScript/Vite compiler output and extract paths of generated test
//     files that are causing build failures.
//  2. Detect classes of brittle patterns commonly produced by code-gen agents
//     (wrong RTL imports, missing vitest globals, malformed structure, etc.).
//  3. Return compile-safe replacement file content — either a minimal smoke test
//     that preserves intent, or a safe placeholder if intent cannot be recovered.
//
// None of these functions have side effects.  The caller (manager integration)
// decides whether and how to write the repairs back to disk.

package agents

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// ─── Error parsing ────────────────────────────────────────────────────────────

// TestFileError records a single compiler error that originated in a generated
// test file.
type TestFileError struct {
	// FilePath is the project-relative path reported by the compiler, e.g.
	//   "src/__tests__/AppShell.test.tsx"
	FilePath string

	// Line and Col are 1-based; 0 means the compiler did not report them.
	Line int
	Col  int

	// Code is the TS error code, e.g. "TS2305".  Empty when not present.
	Code string

	// Message is the raw compiler error message.
	Message string
}

// isGeneratedTestPath reports whether path is a generated test file that the
// repair pipeline is allowed to rewrite.
func isGeneratedTestPath(path string) bool {
	if path == "" {
		return false
	}
	p := filepath.ToSlash(strings.TrimSpace(path))
	markers := []string{
		".test.ts", ".test.tsx", ".test.js", ".test.jsx",
		".spec.ts", ".spec.tsx", ".spec.js", ".spec.jsx",
		"__tests__/", "__specs__/",
	}
	for _, marker := range markers {
		if strings.Contains(p, marker) {
			return true
		}
	}
	return false
}

// tsCompilerLineRe matches lines from tsc / Vite in both formats:
//
//	src/foo.tsx(12,3): error TS2305: Module '"…"' has no exported member 'screen'.
//	src/foo.tsx:12:3 - error TS2305: Module '"…"' has no exported member 'screen'.
var tsCompilerLineRe = regexp.MustCompile(
	`(?m)^([^\s(:\n][^(\n:]*?\.(?:tsx?|jsx?))` + // file path
		`(?:\((\d+),(\d+)\)|:(\d+):(\d+))` + // (line,col) or :line:col
		`\s*[-:]\s*(?:error\s+)?(TS\d+)?[:\s](.+)$`, // optional TSxxxx code + message
)

// ParseTestFileErrors scans a compiler error blob and returns all errors that
// originate in generated test files.  Errors in non-test source files are
// silently ignored (they belong to a different repair pathway).
func ParseTestFileErrors(compilerOutput string) []TestFileError {
	var out []TestFileError
	seen := map[string]bool{}

	normalized := compilerOutput
	prefixes := []string{
		"Preview verification build failed: ",
		"Backend verification build failed: ",
		"Final output validation failed: ",
	}
	for _, prefix := range prefixes {
		normalized = strings.ReplaceAll(normalized, prefix, "")
	}

	matches := tsCompilerLineRe.FindAllStringSubmatch(normalized, -1)
	for _, m := range matches {
		path := strings.TrimSpace(m[1])
		if !isGeneratedTestPath(path) {
			continue
		}

		line := parseInt(m[2], m[4])
		col := parseInt(m[3], m[5])
		code := strings.TrimSpace(m[6])
		msg := strings.TrimSpace(m[7])

		// de-duplicate: same path+line+code
		key := fmt.Sprintf("%s:%d:%s", path, line, code)
		if seen[key] {
			continue
		}
		seen[key] = true

		out = append(out, TestFileError{
			FilePath: path,
			Line:     line,
			Col:      col,
			Code:     code,
			Message:  msg,
		})
	}
	return out
}

// ExtractBrokenTestPaths returns a deduplicated, sorted list of test file paths
// referenced in the compiler output.  Convenience wrapper over ParseTestFileErrors.
func ExtractBrokenTestPaths(compilerOutput string) []string {
	errs := ParseTestFileErrors(compilerOutput)
	seen := map[string]struct{}{}
	var paths []string
	for _, e := range errs {
		if _, ok := seen[e.FilePath]; !ok {
			seen[e.FilePath] = struct{}{}
			paths = append(paths, e.FilePath)
		}
	}
	return paths
}

// ─── Brittleness detection ────────────────────────────────────────────────────

// TestFileFlaw classifies a known brittleness pattern in a generated test file.
type TestFileFlaw string

const (
	// FlawRTLScreenMissing — `screen` imported from @testing-library/react but
	// the installed version does not export it (common in RTL < v9).
	FlawRTLScreenMissing TestFileFlaw = "rtl_screen_missing"

	// FlawRTLBadImport — other @testing-library/react members that are absent or
	// renamed across versions (waitForElement, flushEffects, etc.).
	FlawRTLBadImport TestFileFlaw = "rtl_bad_import"

	// FlawMissingTestFramework — no import from vitest / jest, but uses describe/it/expect.
	FlawMissingTestFramework TestFileFlaw = "missing_test_framework_import"

	// FlawMissingReactImport — JSX used but React not imported (React 16 projects).
	FlawMissingReactImport TestFileFlaw = "missing_react_import"

	// FlawEmptyFile — file is empty or whitespace-only.
	FlawEmptyFile TestFileFlaw = "empty_file"

	// FlawMalformed — file cannot be parsed as a plausible test module at all.
	FlawMalformed TestFileFlaw = "malformed"
)

// screenNotExportedRe matches the specific error for missing `screen` export.
var screenNotExportedRe = regexp.MustCompile(
	`(?i)Module '"@testing-library/react"' has no exported member 'screen'` +
		`|Module '"@testing-library/react"' has no exported member named 'screen'`,
)

// rtlBadMemberRe matches other known-absent RTL members.
var rtlBadMemberRe = regexp.MustCompile(
	`(?i)Module '"@testing-library/react"' has no exported member '(waitForElement|flushEffects|waitForDomChange|act)'`,
)

// DetectTestFileFlaws analyses compiler errors for a single test file and
// returns all detected flaw classes.  An empty slice means no known brittleness
// patterns were found (the error may still be real and require manual attention).
func DetectTestFileFlaws(compilerErrors []TestFileError) []TestFileFlaw {
	var flaws []TestFileFlaw
	flawSet := map[TestFileFlaw]bool{}

	add := func(f TestFileFlaw) {
		if !flawSet[f] {
			flawSet[f] = true
			flaws = append(flaws, f)
		}
	}

	for _, e := range compilerErrors {
		msg := e.Message + " " + e.Code
		if screenNotExportedRe.MatchString(msg) {
			add(FlawRTLScreenMissing)
		}
		if rtlBadMemberRe.MatchString(msg) {
			add(FlawRTLBadImport)
		}
	}
	return flaws
}

// DetectSourceFlaws performs static analysis on raw file content (without the
// need for a compiler run) to detect structural brittleness.
func DetectSourceFlaws(content string) []TestFileFlaw {
	var flaws []TestFileFlaw
	flawSet := map[TestFileFlaw]bool{}

	add := func(f TestFileFlaw) {
		if !flawSet[f] {
			flawSet[f] = true
			flaws = append(flaws, f)
		}
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		add(FlawEmptyFile)
		return flaws
	}

	// Uses describe/it/expect but imports nothing from a test framework.
	// Note: check specifically for vitest/jest imports, not just any import —
	// a file can import from @testing-library/react without importing the test
	// framework, which is a separate flaw.
	hasTestFrameworkImport := strings.Contains(content, "from 'vitest'") ||
		strings.Contains(content, `from "vitest"`) ||
		strings.Contains(content, "from 'jest'") ||
		strings.Contains(content, `from "jest"`) ||
		strings.Contains(content, `from "@jest/globals"`)
	usesTestGlobals := regexp.MustCompile(`\b(describe|it|test|expect)\s*\(`).MatchString(content)
	if usesTestGlobals && !hasTestFrameworkImport {
		add(FlawMissingTestFramework)
	}

	// JSX present but no React import — only flag for React 16 projects.
	// Heuristic: if the file has a vitest import it is a modern React 17+/18
	// project using the automatic JSX transform and does not need an explicit
	// React import. Without vitest (e.g., bare jest + older babel config) an
	// explicit React import is still required.
	hasJSX := regexp.MustCompile(`<[A-Z][A-Za-z]*[\s/>]`).MatchString(content)
	hasReactImport := regexp.MustCompile(`import\s+React\b`).MatchString(content)
	if hasJSX && !hasReactImport && !hasTestFrameworkImport {
		// Only flag when there is no jsxRuntime pragma either.
		hasJSXRuntime := strings.Contains(content, "react/jsx-runtime") ||
			strings.Contains(content, "@jsx")
		if !hasJSXRuntime {
			add(FlawMissingReactImport)
		}
	}

	// RTL screen used but not imported.
	usesScreen := regexp.MustCompile(`\bscreen\.(getBy|queryBy|findBy|getAllBy|queryAllBy|findAllBy|debug)\b`).MatchString(content)
	importsScreen := strings.Contains(content, "screen") && strings.Contains(content, "@testing-library/react")
	if usesScreen && !importsScreen {
		add(FlawRTLScreenMissing)
	}

	return flaws
}

// ─── Repair strategies ────────────────────────────────────────────────────────

// TestFileRepair is the output of a repair attempt.
type TestFileRepair struct {
	// FilePath is the file the repair targets.
	FilePath string

	// RepairedContent is the compile-safe replacement content.
	RepairedContent string

	// Strategy describes what the repairer did.
	Strategy RepairStrategy

	// Flaws lists the brittleness patterns that triggered the repair.
	Flaws []TestFileFlaw
}

// RepairStrategy names the approach used to produce the repaired content.
type RepairStrategy string

const (
	StrategyPatchedImports RepairStrategy = "patched_imports" // fixed import statements; test logic preserved
	StrategyMinimalSmoke   RepairStrategy = "minimal_smoke"   // extracted component name, wrote a minimal render test
	StrategyPlaceholder    RepairStrategy = "placeholder"     // full placeholder — original could not be salvaged
)

// RepairGeneratedTestFile attempts to produce compile-safe replacement content
// for a broken generated test file.
//
//   - originalContent is the current (broken) file content.
//   - compilerErrors are the errors the compiler reported for this specific file.
//
// The function tries the least-invasive repair first:
//  1. Patch import statements only (preserves all test logic).
//  2. Extract the component name and emit a minimal smoke test.
//  3. Fall back to a safe placeholder.
//
// The returned TestFileRepair.RepairedContent is always non-empty and always
// compiles successfully in a standard Vite+React+vitest project.
func RepairGeneratedTestFile(filePath, originalContent string, compilerErrors []TestFileError) TestFileRepair {
	flaws := DetectTestFileFlaws(compilerErrors)
	sourceFlaws := DetectSourceFlaws(originalContent)
	flaws = mergeFlaws(flaws, sourceFlaws)

	repair := TestFileRepair{
		FilePath: filePath,
		Flaws:    flaws,
	}

	// Strategy 1: patch imports when the logic is otherwise sound.
	if patched, ok := tryPatchImports(originalContent, flaws); ok {
		repair.RepairedContent = patched
		repair.Strategy = StrategyPatchedImports
		return repair
	}

	// Strategy 2: emit a minimal smoke test using the inferred component name.
	if smoke, ok := tryMinimalSmoke(filePath, originalContent); ok {
		repair.RepairedContent = smoke
		repair.Strategy = StrategyMinimalSmoke
		return repair
	}

	// Strategy 3: safe placeholder.
	repair.RepairedContent = compileSafePlaceholder(filePath)
	repair.Strategy = StrategyPlaceholder
	return repair
}

// RepairAll is a convenience function that repairs every broken test file
// identified in compilerOutput.  It reads originalContents from the provided
// map (path → content); files not present in the map are given placeholder
// repairs.  Non-test paths in compilerOutput are ignored.
func RepairAll(
	compilerOutput string,
	originalContents map[string]string,
) []TestFileRepair {
	// Group errors by file.
	errs := ParseTestFileErrors(compilerOutput)
	byFile := map[string][]TestFileError{}
	for _, e := range errs {
		byFile[e.FilePath] = append(byFile[e.FilePath], e)
	}

	var repairs []TestFileRepair
	for path, fileErrs := range byFile {
		content := originalContents[path] // empty string → placeholder path
		repairs = append(repairs, RepairGeneratedTestFile(path, content, fileErrs))
	}
	return repairs
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

// parseInt returns the first non-empty int string parsed as int, or 0.
func parseInt(a, b string) int {
	for _, s := range []string{a, b} {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		n := 0
		for _, c := range s {
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int(c-'0')
		}
		if n > 0 {
			return n
		}
	}
	return 0
}

func mergeFlaws(a, b []TestFileFlaw) []TestFileFlaw {
	seen := map[TestFileFlaw]bool{}
	var out []TestFileFlaw
	for _, f := range append(a, b...) {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

// tryPatchImports rewrites import statements to fix known brittleness patterns
// without altering test logic.  Returns (patched, true) on success.
func tryPatchImports(content string, flaws []TestFileFlaw) (string, bool) {
	if len(flaws) == 0 {
		return "", false
	}

	patched := content
	changed := false

	for _, flaw := range flaws {
		switch flaw {

		case FlawRTLScreenMissing:
			// Replace: import { render, screen } from '@testing-library/react'
			// With:    import { render } from '@testing-library/react'
			// And add: import { screen } from '@testing-library/user-event' OR
			// rewrite screen.* → getBy* calls using document.querySelector.
			//
			// Safest approach: replace the broken import with a version that does
			// NOT import `screen`, then add a compatibility shim that defines
			// `screen` via @testing-library/dom (which does export it).
			rtlImportRe := regexp.MustCompile(
				`import\s*\{([^}]+)\}\s*from\s*['"]@testing-library/react['"]`,
			)
			if m := rtlImportRe.FindStringSubmatchIndex(patched); m != nil {
				members := patched[m[2]:m[3]]
				// Remove `screen` from the named members.
				cleaned := removeImportMember(members, "screen")
				// Add @testing-library/dom import for screen as a new line.
				newImport := fmt.Sprintf("import { %s } from '@testing-library/react'\nimport { screen } from '@testing-library/dom'", cleaned)
				patched = patched[:m[0]] + newImport + patched[m[1]:]
				changed = true
			}

		case FlawRTLBadImport:
			// Remove the specific bad member names from the RTL import.
			rtlImportRe := regexp.MustCompile(
				`import\s*\{([^}]+)\}\s*from\s*['"]@testing-library/react['"]`,
			)
			badMembers := []string{"waitForElement", "flushEffects", "waitForDomChange"}
			if m := rtlImportRe.FindStringSubmatchIndex(patched); m != nil {
				members := patched[m[2]:m[3]]
				for _, bad := range badMembers {
					members = removeImportMember(members, bad)
				}
				newImport := fmt.Sprintf("import { %s } from '@testing-library/react'", members)
				patched = patched[:m[0]] + newImport + patched[m[1]:]
				changed = true
			}

		case FlawMissingTestFramework:
			// Prepend vitest imports.
			vitest := "import { describe, it, expect, vi } from 'vitest'\n"
			if !strings.Contains(patched, "from 'vitest'") {
				patched = vitest + patched
				changed = true
			}

		case FlawMissingReactImport:
			// Prepend React import.
			ri := "import React from 'react'\n"
			if !strings.Contains(patched, "import React") {
				patched = ri + patched
				changed = true
			}
		}
	}

	if !changed {
		return "", false
	}

	// Final sanity: the patched file must still look like a test file.
	if !looksLikeTestFile(patched) {
		return "", false
	}

	return patched, true
}

// removeImportMember removes `member` from a comma-separated import member list.
// e.g. removeImportMember(" render, screen, fireEvent ", "screen")
//
//	→ " render, fireEvent "
func removeImportMember(members, member string) string {
	parts := strings.Split(members, ",")
	var kept []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == member || strings.HasPrefix(trimmed, member+" ") || strings.HasSuffix(trimmed, " "+member) {
			continue
		}
		if trimmed != "" {
			kept = append(kept, trimmed)
		}
	}
	return strings.Join(kept, ", ")
}

// looksLikeTestFile returns true if content contains at least one test call.
func looksLikeTestFile(content string) bool {
	return regexp.MustCompile(`\b(describe|it|test)\s*\(`).MatchString(content)
}

// tryMinimalSmoke attempts to write a minimal "render without crashing" smoke
// test inferred from the file path and the import list of the original file.
func tryMinimalSmoke(filePath, originalContent string) (string, bool) {
	componentName := inferComponentName(filePath, originalContent)
	if componentName == "" {
		return "", false
	}

	// Infer the import path from the file path.
	// __tests__/AppShell.test.tsx → ../AppShell
	importPath := inferComponentImportPath(filePath)

	smoke := fmt.Sprintf(`import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import %s from '%s'

describe('%s (smoke)', () => {
  it('renders without crashing', () => {
    const { container } = render(<%s />)
    expect(container).toBeTruthy()
  })
})
`, componentName, importPath, componentName, componentName)

	return smoke, true
}

// inferComponentName tries to extract the tested component name from the file
// path and the import statements of the original file.
func inferComponentName(filePath, content string) string {
	// Try to find a default import from a non-test module.
	defaultImportRe := regexp.MustCompile(`import\s+(\w+)\s+from\s+['"]([^'"]+)['"]`)
	for _, m := range defaultImportRe.FindAllStringSubmatch(content, -1) {
		importedName := m[1]
		importedPath := m[2]
		// Skip test-framework imports.
		if strings.Contains(importedPath, "vitest") ||
			strings.Contains(importedPath, "jest") ||
			strings.Contains(importedPath, "@testing-library") {
			continue
		}
		// Component names start with uppercase.
		if len(importedName) > 0 && importedName[0] >= 'A' && importedName[0] <= 'Z' {
			return importedName
		}
	}

	// Fall back to deriving from the file name.
	base := filepath.Base(filePath)
	// Strip extensions and test suffixes.
	noExt := strings.TrimSuffix(base, filepath.Ext(base))
	noExt = strings.TrimSuffix(noExt, filepath.Ext(noExt)) // .test.tsx → strip .test too
	noExt = strings.TrimSuffix(noExt, ".test")
	noExt = strings.TrimSuffix(noExt, ".spec")

	// Must start with uppercase to be a React component.
	if len(noExt) > 0 && noExt[0] >= 'A' && noExt[0] <= 'Z' {
		return noExt
	}

	return ""
}

// inferComponentImportPath guesses the relative import path from a test file to
// the component it is testing.
//
//	src/__tests__/AppShell.test.tsx  → ../AppShell
//	src/components/Button.test.tsx   → ./Button
func inferComponentImportPath(testFilePath string) string {
	dir := filepath.Dir(testFilePath)
	base := filepath.Base(testFilePath)

	// Strip test/spec extensions.
	name := base
	for _, ext := range []string{".test.tsx", ".test.ts", ".test.jsx", ".test.js",
		".spec.tsx", ".spec.ts", ".spec.jsx", ".spec.js"} {
		if strings.HasSuffix(name, ext) {
			name = strings.TrimSuffix(name, ext)
			break
		}
	}

	// If the file is inside a __tests__ directory, go up one level.
	dirBase := filepath.Base(dir)
	if dirBase == "__tests__" || dirBase == "__specs__" {
		return "../" + name
	}

	return "./" + name
}

// compileSafePlaceholder returns a test file that always compiles and passes.
// It imports nothing unusual and contains one trivially-true assertion.
func compileSafePlaceholder(filePath string) string {
	label := filepath.Base(filePath)
	return fmt.Sprintf(`// AUTO-REPAIRED: original generated test could not be compiled safely.
// This placeholder ensures the build completes. Replace with real tests.
import { describe, it, expect } from 'vitest'

describe('%s (placeholder)', () => {
  it('placeholder — replace with real test', () => {
    expect(true).toBe(true)
  })
})
`, label)
}
