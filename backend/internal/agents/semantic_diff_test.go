package agents

import (
	"sort"
	"testing"
)

// helpers

func makeFiles(paths ...string) []GeneratedFile {
	out := make([]GeneratedFile, 0, len(paths))
	for _, p := range paths {
		out = append(out, GeneratedFile{Path: p, Content: "// placeholder"})
	}
	return out
}

// ─── isSupportedImportFileType ───────────────────────────────────────────────

func TestIsSupportedImportFileType(t *testing.T) {
	trueCases := []string{"src/app.ts", "src/app.tsx", "src/app.js", "src/app.jsx", "src/app.mjs"}
	for _, p := range trueCases {
		if !isSupportedImportFileType(p) {
			t.Errorf("expected isSupportedImportFileType(%q) = true", p)
		}
	}
	falseCases := []string{"src/styles.css", "src/logo.png", "README.md", "backend/main.go"}
	for _, p := range falseCases {
		if isSupportedImportFileType(p) {
			t.Errorf("expected isSupportedImportFileType(%q) = false", p)
		}
	}
}

// ─── resolveImportSpecifier ───────────────────────────────────────────────────

func TestResolveImportSpecifierRelative(t *testing.T) {
	pathIndex := map[string]struct{}{
		"src/components/Button.tsx": {},
	}
	got := resolveImportSpecifier("src/pages/Home.tsx", "./components/Button", pathIndex)
	// filepath.Join("src/pages", "./components/Button") → "src/pages/components/Button"
	// which doesn't match — let's use a sibling scenario
	pathIndex2 := map[string]struct{}{
		"src/components/Button.tsx": {},
	}
	got = resolveImportSpecifier("src/components/Card.tsx", "./Button", pathIndex2)
	if got != "src/components/Button.tsx" {
		t.Errorf("expected src/components/Button.tsx, got %q", got)
	}
}

func TestResolveImportSpecifierAlias(t *testing.T) {
	pathIndex := map[string]struct{}{
		"src/components/Button.tsx": {},
	}
	got := resolveImportSpecifier("src/pages/Home.tsx", "@/components/Button", pathIndex)
	if got != "src/components/Button.tsx" {
		t.Errorf("expected src/components/Button.tsx, got %q", got)
	}
}

func TestResolveImportSpecifierNodeModulesIgnored(t *testing.T) {
	pathIndex := map[string]struct{}{
		"src/components/Button.tsx": {},
	}
	got := resolveImportSpecifier("src/App.tsx", "react", pathIndex)
	if got != "" {
		t.Errorf("expected empty string for node_modules import, got %q", got)
	}
}

func TestResolveImportSpecifierIndexFallback(t *testing.T) {
	pathIndex := map[string]struct{}{
		"src/components/index.ts": {},
	}
	got := resolveImportSpecifier("src/App.tsx", "@/components", pathIndex)
	if got != "src/components/index.ts" {
		t.Errorf("expected src/components/index.ts, got %q", got)
	}
}

func TestResolveImportSpecifierNoMatch(t *testing.T) {
	pathIndex := map[string]struct{}{}
	got := resolveImportSpecifier("src/App.tsx", "./missing", pathIndex)
	if got != "" {
		t.Errorf("expected empty string for unresolved import, got %q", got)
	}
}

// ─── buildFileImportGraph ─────────────────────────────────────────────────────

func TestBuildFileImportGraphBasic(t *testing.T) {
	files := []GeneratedFile{
		{
			Path:    "src/App.tsx",
			Content: `import Button from "./components/Button";`,
		},
		{Path: "src/components/Button.tsx", Content: `export default function Button() {}`},
	}
	graph := buildFileImportGraph(files)
	deps, ok := graph["src/App.tsx"]
	if !ok || len(deps) == 0 {
		t.Fatalf("expected App.tsx to have import edges, got %v", graph)
	}
	if deps[0] != "src/components/Button.tsx" {
		t.Errorf("expected src/components/Button.tsx, got %q", deps[0])
	}
}

func TestBuildFileImportGraphIgnoresNodeModules(t *testing.T) {
	files := []GeneratedFile{
		{
			Path:    "src/App.tsx",
			Content: `import React from "react"; import { cn } from "clsx";`,
		},
	}
	graph := buildFileImportGraph(files)
	if len(graph["src/App.tsx"]) != 0 {
		t.Errorf("expected no edges for node_modules imports, got %v", graph["src/App.tsx"])
	}
}

func TestBuildFileImportGraphSkipsNonJSFiles(t *testing.T) {
	files := []GeneratedFile{
		{Path: "src/styles.css", Content: `@import "./base.css";`},
		{Path: "src/base.css", Content: ``},
	}
	graph := buildFileImportGraph(files)
	if len(graph) != 0 {
		t.Errorf("expected empty graph for CSS files, got %v", graph)
	}
}

func TestBuildFileImportGraphMultipleImports(t *testing.T) {
	files := []GeneratedFile{
		{
			Path: "src/App.tsx",
			Content: `import Button from "./components/Button";
import { useStore } from "@/store/app";`,
		},
		{Path: "src/components/Button.tsx", Content: ""},
		{Path: "src/store/app.ts", Content: ""},
	}
	graph := buildFileImportGraph(files)
	deps := graph["src/App.tsx"]
	sort.Strings(deps)
	if len(deps) != 2 {
		t.Errorf("expected 2 deps, got %v", deps)
	}
}

// ─── reverseFileImportGraph ───────────────────────────────────────────────────

func TestReverseFileImportGraph(t *testing.T) {
	graph := FileImportGraph{
		"src/App.tsx": {"src/components/Button.tsx"},
		"src/pages/Home.tsx": {"src/components/Button.tsx"},
	}
	rev := reverseFileImportGraph(graph)
	importers := rev["src/components/Button.tsx"]
	sort.Strings(importers)
	if len(importers) != 2 {
		t.Errorf("expected 2 importers of Button.tsx, got %v", importers)
	}
	if importers[0] != "src/App.tsx" || importers[1] != "src/pages/Home.tsx" {
		t.Errorf("unexpected importers: %v", importers)
	}
}

func TestReverseFileImportGraphEmpty(t *testing.T) {
	rev := reverseFileImportGraph(nil)
	if len(rev) != 0 {
		t.Errorf("expected empty reverse graph, got %v", rev)
	}
}

// ─── resolveAffectedFiles ─────────────────────────────────────────────────────

func TestResolveAffectedFilesDirectOnly(t *testing.T) {
	rev := FileImportGraph{
		"src/components/Button.tsx": {},
	}
	got := resolveAffectedFiles([]string{"src/components/Button.tsx"}, rev)
	if len(got) != 1 || got[0] != "src/components/Button.tsx" {
		t.Errorf("expected only direct file, got %v", got)
	}
}

func TestResolveAffectedFilesTransitive(t *testing.T) {
	// Button ← App ← main
	rev := FileImportGraph{
		"src/components/Button.tsx": {"src/App.tsx"},
		"src/App.tsx":               {"src/main.tsx"},
	}
	got := resolveAffectedFiles([]string{"src/components/Button.tsx"}, rev)
	sort.Strings(got)
	want := []string{"src/App.tsx", "src/components/Button.tsx", "src/main.tsx"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("want[%d]=%q, got[%d]=%q", i, want[i], i, got[i])
		}
	}
}

func TestResolveAffectedFilesEmptyChanges(t *testing.T) {
	rev := FileImportGraph{"a.ts": {"b.ts"}}
	got := resolveAffectedFiles(nil, rev)
	if got != nil {
		t.Errorf("expected nil for empty changedPaths, got %v", got)
	}
}

func TestResolveAffectedFilesEmptyGraph(t *testing.T) {
	got := resolveAffectedFiles([]string{"src/App.tsx"}, nil)
	if got != nil {
		t.Errorf("expected nil for empty graph, got %v", got)
	}
}

func TestResolveAffectedFilesNoCycles(t *testing.T) {
	// Mutual import: A ↔ B — BFS must not loop forever.
	rev := FileImportGraph{
		"src/A.tsx": {"src/B.tsx"},
		"src/B.tsx": {"src/A.tsx"},
	}
	got := resolveAffectedFiles([]string{"src/A.tsx"}, rev)
	sort.Strings(got)
	want := []string{"src/A.tsx", "src/B.tsx"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("expected %v, got %v", want, got)
	}
}

// ─── extractMentionedPaths ────────────────────────────────────────────────────

func TestExtractMentionedPathsMatchesByBasename(t *testing.T) {
	files := makeFiles("src/components/Button.tsx", "src/App.tsx", "src/styles.css")
	got := extractMentionedPaths("please fix the button component in Button.tsx", files)
	if len(got) != 1 || got[0] != "src/components/Button.tsx" {
		t.Errorf("expected Button.tsx match, got %v", got)
	}
}

func TestExtractMentionedPathsCaseInsensitive(t *testing.T) {
	files := makeFiles("src/components/Header.tsx")
	got := extractMentionedPaths("update header.tsx to add a logo", files)
	if len(got) != 1 || got[0] != "src/components/Header.tsx" {
		t.Errorf("expected Header.tsx match, got %v", got)
	}
}

func TestExtractMentionedPathsNoMatch(t *testing.T) {
	files := makeFiles("src/App.tsx", "src/styles.css")
	got := extractMentionedPaths("make the sidebar wider", files)
	if len(got) != 0 {
		t.Errorf("expected no matches, got %v", got)
	}
}

func TestExtractMentionedPathsDeduplicated(t *testing.T) {
	files := makeFiles("src/components/Button.tsx")
	// Mention the same base name twice.
	got := extractMentionedPaths("fix Button.tsx — Button.tsx is broken", files)
	if len(got) != 1 {
		t.Errorf("expected 1 deduplicated result, got %v", got)
	}
}

// ─── semanticDiffRoutingEnabled ──────────────────────────────────────────────

func TestSemanticDiffRoutingEnabledDefault(t *testing.T) {
	t.Setenv("APEX_SEMANTIC_DIFF_ROUTING", "")
	if !semanticDiffRoutingEnabled() {
		t.Error("expected semanticDiffRoutingEnabled=true when env var is unset")
	}
}

func TestSemanticDiffRoutingDisabledFalsy(t *testing.T) {
	for _, val := range []string{"false", "0", "no", "off"} {
		t.Setenv("APEX_SEMANTIC_DIFF_ROUTING", val)
		if semanticDiffRoutingEnabled() {
			t.Errorf("expected semanticDiffRoutingEnabled=false for %q", val)
		}
	}
}

// ─── computeSemanticDiffHint ──────────────────────────────────────────────────

func TestComputeSemanticDiffHintFlagDisabled(t *testing.T) {
	t.Setenv("APEX_SEMANTIC_DIFF_ROUTING", "false")
	files := makeNFiles(10)
	hint := computeSemanticDiffHint(&Build{ID: "b1"}, files, "fix Button.tsx")
	if !hint.Uncertainty {
		t.Error("expected Uncertainty=true when flag is disabled")
	}
}

func TestComputeSemanticDiffHintTooFewFiles(t *testing.T) {
	t.Setenv("APEX_SEMANTIC_DIFF_ROUTING", "true")
	files := makeFiles("src/App.tsx", "src/main.tsx") // 2 < minFilesForGraphTrust
	hint := computeSemanticDiffHint(&Build{ID: "b1"}, files, "fix app.tsx")
	if !hint.Uncertainty {
		t.Error("expected Uncertainty=true when fewer than minFilesForGraphTrust files")
	}
}

func TestComputeSemanticDiffHintNoMentionedFiles(t *testing.T) {
	t.Setenv("APEX_SEMANTIC_DIFF_ROUTING", "true")
	files := makeNFilesWithImports()
	hint := computeSemanticDiffHint(&Build{ID: "b1"}, files, "make the sidebar wider")
	if !hint.Uncertainty {
		t.Error("expected Uncertainty=true when no files are mentioned in the request")
	}
}

func TestComputeSemanticDiffHintTargetedScope(t *testing.T) {
	t.Setenv("APEX_SEMANTIC_DIFF_ROUTING", "true")
	files := makeNFilesWithImports()
	// "Button.tsx" appears in the request and is mentioned in the file list.
	hint := computeSemanticDiffHint(&Build{ID: "b1"}, files, "fix the bug in Button.tsx")
	if hint.Uncertainty {
		t.Error("expected Uncertainty=false for targeted request")
	}
	if len(hint.AffectedFiles) == 0 {
		t.Error("expected non-empty AffectedFiles")
	}
}

func TestComputeSemanticDiffHintNilBuild(t *testing.T) {
	t.Setenv("APEX_SEMANTIC_DIFF_ROUTING", "true")
	files := makeNFilesWithImports()
	// Should not panic.
	hint := computeSemanticDiffHint(nil, files, "fix Button.tsx")
	_ = hint
}

// ─── test fixture helpers ─────────────────────────────────────────────────────

// makeNFiles returns n placeholder files with unique paths.
func makeNFiles(n int) []GeneratedFile {
	out := make([]GeneratedFile, n)
	for i := 0; i < n; i++ {
		out[i] = GeneratedFile{
			Path:    "src/generated_file_" + itoa(i) + ".tsx",
			Content: "// placeholder",
		}
	}
	return out
}

// makeNFilesWithImports returns a realistic set of files (>= minFilesForGraphTrust)
// with one that imports Button.tsx, so the graph is non-trivial.
func makeNFilesWithImports() []GeneratedFile {
	return []GeneratedFile{
		{Path: "src/App.tsx", Content: `import Button from "./components/Button";`},
		{Path: "src/components/Button.tsx", Content: `export default function Button() {}`},
		{Path: "src/main.tsx", Content: `import App from "./App";`},
		{Path: "src/pages/Home.tsx", Content: `import App from "../App";`},
		{Path: "src/pages/About.tsx", Content: `import Button from "../components/Button";`},
		{Path: "src/styles.css", Content: `body { margin: 0; }`},
		{Path: "src/utils/cn.ts", Content: `export function cn() {}`},
	}
}

// itoa is a minimal int-to-string helper for test fixtures.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := []byte{}
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}
