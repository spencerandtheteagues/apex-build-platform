package agents

import (
	"strings"
	"testing"
)

func TestValidateFinalBuildReadiness(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}

	t.Run("valid_react_output", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "moneyflow",
  "scripts": {
    "dev": "vite",
    "build": "vite build"
  },
  "dependencies": {
    "react": "^18.3.0",
    "react-dom": "^18.3.0"
  }
}`,
			},
			{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
			{Path: "src/main.tsx", Content: "import React from 'react';"},
			{Path: "src/App.tsx", Content: "export const App = () => <div>ok</div>;"},
		}

		errs := am.validateFinalBuildReadiness(nil, files)
		if len(errs) != 0 {
			t.Fatalf("expected no readiness errors, got %v", errs)
		}
	})

	t.Run("incomplete_frontend_output", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "moneyflow",
  "dependencies": {
    "uuid": "^9.0.1"
  }
}`,
			},
			{Path: "src/tests/MoneyFlowApp.test.tsx", Content: "describe('x', () => {})"},
			{Path: "src/utils/validation.ts", Content: "export const x = 1"},
		}

		errs := am.validateFinalBuildReadiness(nil, files)
		if len(errs) == 0 {
			t.Fatalf("expected readiness errors for incomplete frontend output")
		}
		if !containsError(errs, "HTML entry point") {
			t.Fatalf("expected missing HTML entry point error, got %v", errs)
		}
		if !containsError(errs, "missing an entry source file") {
			t.Fatalf("expected missing frontend entry source error, got %v", errs)
		}
	})

	t.Run("no_files", func(t *testing.T) {
		t.Parallel()

		errs := am.validateFinalBuildReadiness(nil, nil)
		if len(errs) == 0 {
			t.Fatalf("expected readiness error for empty output")
		}
		if !containsError(errs, "No files were generated") {
			t.Fatalf("unexpected readiness errors: %v", errs)
		}
	})
}

func containsError(errors []string, want string) bool {
	for _, err := range errors {
		if strings.Contains(err, want) {
			return true
		}
	}
	return false
}
