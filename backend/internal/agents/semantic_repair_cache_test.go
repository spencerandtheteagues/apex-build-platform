package agents

import (
	"strings"
	"testing"
)

func TestSemanticRepairCacheHintFromFingerprintKeepsNarrowCompileClasses(t *testing.T) {
	hint := semanticRepairCacheHintFromFingerprint(FailureFingerprint{
		TaskShape:       TaskShapeRepair,
		FailureClass:    "compile_failure",
		RepairStrategy:  "strict_ast_syntax_repair",
		PatchClass:      "import_export_mismatch",
		FilesInvolved:   []string{"src/App.tsx", "src/Button.tsx"},
		RepairSucceeded: true,
	})

	for _, want := range []string{"failure=compile_failure", "patch=import_export_mismatch", "strategy=strict_ast_syntax_repair", "files=src/App.tsx,src/Button.tsx"} {
		if !strings.Contains(hint, want) {
			t.Fatalf("expected semantic repair hint to contain %q, got %q", want, hint)
		}
	}
}

func TestSemanticRepairCacheHintFromFingerprintRejectsBroadRewrite(t *testing.T) {
	hint := semanticRepairCacheHintFromFingerprint(FailureFingerprint{
		TaskShape:       TaskShapeRepair,
		FailureClass:    "preview_verification",
		RepairStrategy:  "targeted_node_rewrite",
		PatchClass:      "whole_file_rewrite",
		FilesInvolved:   []string{"src/App.tsx"},
		RepairSucceeded: true,
	})

	if hint != "" {
		t.Fatalf("expected broad rewrite to be omitted from semantic repair cache, got %q", hint)
	}
}
