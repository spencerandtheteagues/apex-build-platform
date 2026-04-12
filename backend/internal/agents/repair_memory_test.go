package agents

import (
	"strings"
	"testing"
	"time"

	"apex-build/internal/ai"
)

func TestAppendFailureFingerprintAnnotatesRepairMemoryFields(t *testing.T) {
	build := &Build{
		ID: "repair-memory-build",
		Plan: &BuildPlan{
			TechStack: TechStack{Frontend: "React", Backend: "Express", Database: "Postgres"},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	appendFailureFingerprint(build, FailureFingerprint{
		TaskShape:        TaskShapeRepair,
		Provider:         ai.ProviderClaude,
		FailureClass:     "compile_failure",
		FilesInvolved:    []string{"src/App.tsx", "src/App.tsx", "src/Button.tsx"},
		RepairPathChosen: []string{"compile_validator", "hydra_repair", "hydra_repair"},
		RepairStrategy:   "strict_ast_syntax_repair",
		PatchClass:       "import-export mismatch",
		RepairSucceeded:  true,
		CreatedAt:        time.Now().UTC(),
	})

	state := build.SnapshotState.Orchestration
	if state == nil || len(state.FailureFingerprints) != 1 {
		t.Fatalf("expected one fingerprint, got %+v", state)
	}
	fp := state.FailureFingerprints[0]
	if fp.ID == "" || fp.BuildID != build.ID || fp.StackCombination != "React|Express|Postgres" {
		t.Fatalf("expected fingerprint identity and stack defaults, got %+v", fp)
	}
	if fp.FailureClass != "compile_failure" {
		t.Fatalf("expected normalized failure class, got %+v", fp)
	}
	if fp.PatchClass != "import_export_mismatch" {
		t.Fatalf("expected normalized patch class, got %+v", fp)
	}
	if len(fp.FilesInvolved) != 2 || fp.FilesInvolved[0] != "src/App.tsx" || fp.FilesInvolved[1] != "src/Button.tsx" {
		t.Fatalf("expected normalized files, got %+v", fp.FilesInvolved)
	}
	if len(fp.RepairPathChosen) != 2 {
		t.Fatalf("expected deduped repair path, got %+v", fp.RepairPathChosen)
	}
	if fp.FingerprintKey == "" || !strings.Contains(fp.FingerprintKey, "repair") {
		t.Fatalf("expected fingerprint key, got %+v", fp)
	}
}

func TestRepairPatchClassFromBundleClassifiesNarrowRepair(t *testing.T) {
	bundle := &PatchBundle{
		Operations: []PatchOperation{
			{Type: PatchReplaceSymbol, Path: "src/App.tsx", Content: "import Button from './Button'\nexport function App(){ return <Button /> }\n"},
		},
	}

	if got := repairPatchClassFromBundle(bundle); got != "import_export_mismatch" {
		t.Fatalf("repairPatchClassFromBundle = %q, want import_export_mismatch", got)
	}
}

func TestRepairMemoryPromptContextReturnsRecentSuccessfulMatch(t *testing.T) {
	build := &Build{
		ID: "repair-memory-context-build",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
				FailureFingerprints: []FailureFingerprint{
					{
						ID:               "old-failure",
						BuildID:          "repair-memory-context-build",
						TaskShape:        TaskShapeRepair,
						FailureClass:     "compile_failure",
						FilesInvolved:    []string{"src/Other.tsx"},
						RepairStrategy:   "targeted_node_rewrite",
						PatchClass:       "symbol_patch",
						RepairSucceeded:  true,
						RepairPathChosen: []string{"compile_validator", "hydra_repair"},
						CreatedAt:        time.Now().Add(-time.Minute).UTC(),
					},
					{
						ID:               "recent-success",
						BuildID:          "repair-memory-context-build",
						TaskShape:        TaskShapeRepair,
						FailureClass:     "compile_failure",
						FilesInvolved:    []string{"src/App.tsx"},
						RepairStrategy:   "strict_ast_syntax_repair",
						PatchClass:       "import_export_mismatch",
						RepairSucceeded:  true,
						RepairPathChosen: []string{"compile_validator", "hydra_repair", "strict_ast_syntax_repair"},
						CreatedAt:        time.Now().UTC(),
					},
				},
			},
		},
	}

	context := repairMemoryPromptContextForBuild(build, "compile_failure", []string{"src/App.tsx"})
	if !strings.Contains(context, "<repair_memory>") {
		t.Fatalf("expected repair memory context, got %q", context)
	}
	if !strings.Contains(context, "strict_ast_syntax_repair") {
		t.Fatalf("expected recent matching strategy in context, got %q", context)
	}
	if strings.Contains(context, "targeted_node_rewrite") {
		t.Fatalf("expected non-overlapping historical repair to be filtered, got %q", context)
	}
}
