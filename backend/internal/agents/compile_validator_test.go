package agents

import (
	"strings"
	"testing"
	"time"
)

func TestMaxCompileAttemptsByPowerMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode PowerMode
		want int
	}{
		{name: "fast", mode: PowerFast, want: 1},
		{name: "balanced", mode: PowerBalanced, want: 2},
		{name: "max", mode: PowerMax, want: 3},
		{name: "unknown defaults to fast behavior", mode: PowerMode("unknown"), want: 1},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := maxCompileAttempts(tt.mode); got != tt.want {
				t.Fatalf("maxCompileAttempts(%q) = %d, want %d", tt.mode, got, tt.want)
			}
		})
	}
}

func TestRunCompileValidationLoopSkipsWhenAIRouterNil(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "compile-validator-nil-router",
		PowerMode: PowerBalanced,
	}
	files := []GeneratedFile{
		{
			Path: "package.json",
			Content: `{
  "name": "compile-validator-nil-router",
  "private": true,
  "scripts": {
    "build": "vite build"
  }
}`,
		},
		{
			Path:    "src/main.tsx",
			Content: `console.log("preview");`,
		},
	}

	result := am.runCompileValidationLoop(build, &files, time.Now())
	if result.Passed {
		t.Fatalf("expected compile loop to skip when aiRouter is nil")
	}
	if result.SkipReason != "ai router not configured" {
		t.Fatalf("expected skip reason for nil aiRouter, got %q", result.SkipReason)
	}
	if build.CompileValidationPassed {
		t.Fatalf("expected compile validation passed flag to remain false")
	}
	if build.CompileValidationAttempts != 0 {
		t.Fatalf("expected compile validation attempts to remain 0, got %d", build.CompileValidationAttempts)
	}
}

func TestCVRunInlineRepairAppliesDeterministicReactPropMismatchRepairBeforeAI(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "compile-inline-react-prop-mismatch",
		Status:    BuildInProgress,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path: "src/components/Button.tsx",
				Content: `export interface ButtonProps {
  label: string
}

export function Button({ label }: ButtonProps) {
  return <button className="rounded-lg bg-indigo-600 px-4 py-2 text-white">{label}</button>
}
`,
				IsNew: true,
			},
			{
				Path: "src/components/ClientCard.tsx",
				Content: `import { Button } from "./Button"

export function ClientCard({ onSelect }: { onSelect?: () => void }) {
  return (
    <div className="rounded-2xl border border-slate-700 p-4">
      <Button className="w-full justify-center" onClick={onSelect}>
        View Details
      </Button>
    </div>
  )
}
`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	allFiles := am.collectGeneratedFiles(build)
	repaired := am.cvRunInlineRepair(nil, build, []ParsedBuildError{
		{
			File:    "src/components/ClientCard.tsx",
			Line:    5,
			Column:  15,
			Code:    "TS2322",
			Message: "Type '{ children: string; className: string; onClick: (() => void) | undefined; }' is not assignable to type 'IntrinsicAttributes & ButtonProps'.",
			Source:  "tsc",
		},
	}, &allFiles, "")
	if !repaired {
		t.Fatal("expected deterministic inline repair to apply before AI fallback")
	}

	byPath := map[string]GeneratedFile{}
	for _, file := range allFiles {
		byPath[file.Path] = file
	}
	button, ok := byPath["src/components/Button.tsx"]
	if !ok {
		t.Fatalf("expected Button.tsx to remain available after repair, got %+v", allFiles)
	}
	if !strings.Contains(button.Content, "extends React.ButtonHTMLAttributes<HTMLButtonElement>") {
		t.Fatalf("expected ButtonProps to extend button HTML attributes, got %q", button.Content)
	}
	if !strings.Contains(button.Content, "<button {...buttonProps}") {
		t.Fatalf("expected Button to spread passthrough props onto root button, got %q", button.Content)
	}
}

func TestCVBuildRepairPromptUsesContextDiet(t *testing.T) {
	t.Parallel()

	prompt := cvBuildRepairPrompt([]ParsedBuildError{
		{
			File:    "src/App.tsx",
			Line:    12,
			Column:  8,
			Code:    "TS2322",
			Message: "Type mismatch",
			Source:  "tsc",
		},
	}, []GeneratedFile{
		{
			Path: "src/App.tsx",
			Content: `import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"

export interface AppProps {
  title: string
}

export function App({ title }: AppProps) {
  const brokenValue: string = 42 as unknown as string
  return (
    <Card>
      <Button>{title}</Button>
    </Card>
  )
}
`,
		},
	})

	if strings.Contains(prompt, "**Full file content**") {
		t.Fatalf("expected context-diet prompt, got full file dump: %q", prompt)
	}
	if !strings.Contains(prompt, "Pruned file context") {
		t.Fatalf("expected pruned context header, got %q", prompt)
	}
	if !strings.Contains(prompt, "Public signatures:") {
		t.Fatalf("expected public signatures section, got %q", prompt)
	}
	if !strings.Contains(prompt, "Focused source windows:") {
		t.Fatalf("expected focused source windows section, got %q", prompt)
	}
}

func TestCVHydraStrategiesEnabledForBalancedAndMax(t *testing.T) {
	t.Parallel()

	if got := len(cvHydraStrategies(PowerFast)); got != 0 {
		t.Fatalf("expected fast mode to skip hydra, got %d strategies", got)
	}
	if got := len(cvHydraStrategies(PowerBalanced)); got != 3 {
		t.Fatalf("expected balanced mode hydra strategies, got %d", got)
	}
	if got := len(cvHydraStrategies(PowerMax)); got != 3 {
		t.Fatalf("expected max mode hydra strategies, got %d", got)
	}
}
