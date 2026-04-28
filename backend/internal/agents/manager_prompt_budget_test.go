package agents

import (
	"strings"
	"testing"

	"apex-build/internal/ai"
)

func TestBuildTaskPromptBudgetsLargeRepairContextBelowRouterLimit(t *testing.T) {
	t.Parallel()

	largeSource := "export const value = `" + strings.Repeat("x", 160000) + "`;\n"
	am := &AgentManager{}
	build := &Build{
		ID:          "large-repair-context",
		Description: strings.Repeat("Build a polished enterprise field service app. ", 1200),
		PowerMode:   PowerBalanced,
		Tasks: []*Task{
			{
				ID:     "frontend-output",
				Type:   TaskGenerateUI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{Path: "src/App.tsx", Language: "tsx", Content: largeSource, Size: int64(len(largeSource))},
						{Path: "src/components/Dashboard.tsx", Language: "tsx", Content: largeSource, Size: int64(len(largeSource))},
					},
				},
			},
		},
	}
	task := &Task{
		ID:          "repair-task",
		Type:        TaskFix,
		Description: "Fix critical issues found during code review",
		Input: map[string]any{
			"previous_errors": strings.Repeat("Preview verification failed: unreadable text and missing entrypoint. ", 1000),
			"retry_strategy":  "fix_and_retry",
		},
	}
	agent := &Agent{Role: RoleSolver, Provider: ai.ProviderGPT4}

	prompt := am.buildTaskPrompt(task, build, agent)
	if len(prompt) > taskPromptBudget(build, agent, task) {
		t.Fatalf("prompt length %d exceeded budget %d", len(prompt), taskPromptBudget(build, agent, task))
	}
	if len(prompt) >= ai.MaxPromptLength {
		t.Fatalf("prompt length %d should remain below router limit %d", len(prompt), ai.MaxPromptLength)
	}
	if !strings.Contains(prompt, "PATCH-FIRST OUTPUT FORMAT - CRITICAL") {
		t.Fatalf("compacted repair prompt must preserve patch-first instructions")
	}
	if !strings.Contains(prompt, "context compacted") {
		t.Fatalf("expected prompt to mark compaction for oversized context")
	}
}

func TestClampAIRouterPromptPreservesHeadAndTail(t *testing.T) {
	t.Parallel()

	prompt := "HEAD-" + strings.Repeat("x", ai.MaxPromptLength+50000) + "-TAIL"
	got := clampAIRouterPrompt(prompt, ai.MaxPromptLength-1024)
	if len(got) > ai.MaxPromptLength-1024 {
		t.Fatalf("clamped prompt length %d exceeded max", len(got))
	}
	if !strings.HasPrefix(got, "HEAD-") {
		t.Fatalf("expected clamped prompt to preserve head")
	}
	if !strings.HasSuffix(got, "-TAIL") {
		t.Fatalf("expected clamped prompt to preserve tail")
	}
	if !strings.Contains(got, "prompt compacted") {
		t.Fatalf("expected clamped prompt to include compaction marker")
	}
}
