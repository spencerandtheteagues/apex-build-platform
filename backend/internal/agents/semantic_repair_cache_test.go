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

func TestSemanticRepairPatchClassForErrorsClassifiesNarrowCompileFailures(t *testing.T) {
	tests := []struct {
		name   string
		errors []ParsedBuildError
		want   string
	}{
		{
			name: "export mismatch",
			errors: []ParsedBuildError{{
				Code:    "TS2305",
				Message: `Module '"./api"' has no exported member 'client'.`,
			}},
			want: "import_export_mismatch",
		},
		{
			name: "local missing module",
			errors: []ParsedBuildError{{
				Code:    "TS2307",
				Message: `Cannot find module './Widget' or its corresponding type declarations.`,
			}},
			want: "missing_file",
		},
		{
			name: "package missing module",
			errors: []ParsedBuildError{{
				Code:    "TS2307",
				Message: `Cannot find module 'zod' or its corresponding type declarations.`,
			}},
			want: "dependency_manifest",
		},
		{
			name: "type mismatch",
			errors: []ParsedBuildError{{
				Code:    "TS2322",
				Message: `Type 'string' is not assignable to type 'number'.`,
			}},
			want: "symbol_patch",
		},
		{
			name: "property access mismatch",
			errors: []ParsedBuildError{{
				Code:    "TS2339",
				Message: `Property 'total' does not exist on type 'InvoiceSummary'.`,
			}},
			want: "symbol_patch",
		},
		{
			name: "argument type mismatch",
			errors: []ParsedBuildError{{
				Code:    "TS2345",
				Message: `Argument of type 'string' is not assignable to parameter of type 'number'.`,
			}},
			want: "symbol_patch",
		},
		{
			name: "missing required prop",
			errors: []ParsedBuildError{{
				Code:    "TS2741",
				Message: `Property 'onSave' is missing in type '{ title: string; }' but required in type 'EditorProps'.`,
			}},
			want: "symbol_patch",
		},
		{
			name: "wrong argument count",
			errors: []ParsedBuildError{{
				Code:    "TS2554",
				Message: `Expected 2 arguments, but got 1.`,
			}},
			want: "symbol_patch",
		},
		{
			name: "default export mismatch",
			errors: []ParsedBuildError{{
				Code:    "TS1192",
				Message: `Module '"/src/api"' has no default export.`,
			}},
			want: "import_export_mismatch",
		},
		{
			name: "not a module",
			errors: []ParsedBuildError{{
				Code:    "TS2306",
				Message: `File '/src/config.ts' is not a module.`,
			}},
			want: "import_export_mismatch",
		},
		{
			name: "package type declaration missing",
			errors: []ParsedBuildError{{
				Code:    "TS7016",
				Message: `Could not find a declaration file for module 'date-fns'.`,
			}},
			want: "dependency_manifest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := semanticRepairPatchClassForErrors(tt.errors); got != tt.want {
				t.Fatalf("semanticRepairPatchClassForErrors = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSemanticRepairCachePromptContextFiltersByCurrentPatchClass(t *testing.T) {
	build := &Build{
		ID: "semantic-cache-context-build",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				HistoricalLearning: &BuildLearningSummary{
					SemanticRepairHints: []string{
						"failure=compile_failure patch=import_export_mismatch strategy=strict_ast_syntax_repair files=src/App.tsx",
						"failure=compile_failure patch=dependency_manifest strategy=manifest_repair files=package.json",
					},
				},
			},
		},
	}

	context := semanticRepairCachePromptContext(build, []ParsedBuildError{{
		Code:    "TS2305",
		File:    "src/App.tsx",
		Message: `Module '"./api"' has no exported member 'client'.`,
	}})
	if !strings.Contains(context, "<semantic_repair_cache>") {
		t.Fatalf("expected semantic repair cache context, got %q", context)
	}
	if !strings.Contains(context, "patch=import_export_mismatch") {
		t.Fatalf("expected matching import/export hint, got %q", context)
	}
	if strings.Contains(context, "dependency_manifest") {
		t.Fatalf("expected unrelated semantic hint to be filtered, got %q", context)
	}
}
