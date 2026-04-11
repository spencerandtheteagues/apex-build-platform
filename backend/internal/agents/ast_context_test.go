package agents

import (
	"strings"
	"testing"
)

func TestBuildPrunedSymbolContextTargetsSymbolBodies(t *testing.T) {
	t.Parallel()

	source := `import { Card } from "./Card"

export interface ViewModel {
  title: string
}

export function renderCard(model: ViewModel) {
  return <Card>{model.title}</Card>
}

function helperLabel(input: string) {
  return input.trim()
}
`

	ctx, err := BuildPrunedSymbolContext("src/App.tsx", source, []string{"renderCard"}, []int{8}, PrunedSymbolContextOptions{ContextLines: 3})
	if err != nil {
		if strings.Contains(err.Error(), "without cgo") {
			t.Skip("tree-sitter unavailable in !cgo build")
		}
		t.Fatalf("BuildPrunedSymbolContext returned error: %v", err)
	}
	if !ctx.ParseSucceeded {
		t.Fatalf("expected parse success, got %+v", ctx)
	}
	if len(ctx.TargetSymbols) == 0 {
		t.Fatalf("expected target symbol bodies, got %+v", ctx)
	}
	foundTarget := false
	for _, symbol := range ctx.TargetSymbols {
		if symbol.Name == "renderCard" {
			foundTarget = true
			if !strings.Contains(symbol.Body, "return <Card>") {
				t.Fatalf("expected full target body, got %q", symbol.Body)
			}
		}
	}
	if !foundTarget {
		t.Fatalf("expected renderCard target in %+v", ctx.TargetSymbols)
	}
	if len(ctx.CollapsedSignatures) == 0 {
		t.Fatalf("expected non-target signatures, got %+v", ctx)
	}
}

func TestBuildPrunedSymbolContextSupportsJSTSVariants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path    string
		content string
	}{
		{path: "src/a.ts", content: "export function alpha() { return 1 }"},
		{path: "src/b.tsx", content: "export const Beta = () => <div/>"},
		{path: "src/c.js", content: "export function gamma() { return 2 }"},
		{path: "src/d.jsx", content: "export const Delta = () => <main />"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			ctx, err := BuildPrunedSymbolContext(tc.path, tc.content, nil, nil, PrunedSymbolContextOptions{})
			if err != nil {
				if strings.Contains(err.Error(), "without cgo") {
					t.Skip("tree-sitter unavailable in !cgo build")
				}
				t.Fatalf("BuildPrunedSymbolContext(%s) error: %v", tc.path, err)
			}
			if !ctx.ParseSucceeded {
				t.Fatalf("expected parse success for %s", tc.path)
			}
		})
	}
}

func TestCVBuildRepairPromptUsesASTPruningWhenEnabled(t *testing.T) {
	t.Parallel()

	_, err := BuildPrunedSymbolContext("src/App.tsx", "export function renderCard(){ return 1 }", nil, nil, PrunedSymbolContextOptions{})
	if err != nil && strings.Contains(err.Error(), "without cgo") {
		t.Skip("tree-sitter unavailable in !cgo build")
	}

	prompt := cvBuildRepairPrompt([]ParsedBuildError{
		{
			File:    "src/App.tsx",
			Line:    3,
			Column:  12,
			Code:    "TS2304",
			Message: "Cannot find name 'renderCard'.",
			Source:  "tsc",
		},
	}, []GeneratedFile{
		{
			Path: "src/App.tsx",
			Content: `import { Card } from "./Card"

export function renderCard() {
  return <Card>ok</Card>
}

function helper() {
  return "helper"
}
`,
		},
	}, "", true)

	if !strings.Contains(prompt, "Target symbol bodies:") {
		t.Fatalf("expected AST target symbol section, got %q", prompt)
	}
	if !strings.Contains(prompt, "renderCard") {
		t.Fatalf("expected targeted symbol in prompt, got %q", prompt)
	}
}
