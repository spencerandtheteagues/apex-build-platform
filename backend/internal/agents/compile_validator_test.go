package agents

import (
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
