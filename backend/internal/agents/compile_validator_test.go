package agents

import (
	"strings"
	"testing"
	"time"

	"apex-build/internal/ai"
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

func TestCVBroadcastResultRecordsCompileFailureFingerprint(t *testing.T) {
	am := &AgentManager{
		builds:      make(map[string]*Build),
		subscribers: make(map[string][]chan *WSMessage),
	}
	build := &Build{
		ID: "compile-failure-fingerprint",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}
	am.builds[build.ID] = build

	am.cvBroadcastResult(build, false, []ParsedBuildError{
		{
			File:    "src/App.tsx",
			Line:    4,
			Column:  12,
			Code:    "TS2304",
			Message: "Cannot find name 'MissingThing'.",
			Source:  "tsc",
		},
	})

	state := build.SnapshotState.Orchestration
	if state == nil || len(state.FailureFingerprints) != 1 {
		t.Fatalf("expected compile failure fingerprint, got %+v", state)
	}
	fp := state.FailureFingerprints[0]
	if fp.FailureClass != "compile_failure" || fp.TaskShape != TaskShapeVerification || fp.RepairSucceeded {
		t.Fatalf("expected terminal compile verification fingerprint, got %+v", fp)
	}
	if !containsString(fp.FilesInvolved, "src/App.tsx") {
		t.Fatalf("expected compile error file to be captured, got %+v", fp.FilesInvolved)
	}
	if !containsString(fp.RepairPathChosen, "compile_validator") {
		t.Fatalf("expected compile validator repair path, got %+v", fp.RepairPathChosen)
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
	}, "", false)

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

func TestCVBuildRepairPromptIncludesReliabilitySummary(t *testing.T) {
	t.Parallel()

	prompt := cvBuildRepairPrompt([]ParsedBuildError{
		{
			File:    "src/App.tsx",
			Line:    8,
			Column:  4,
			Code:    "TS2304",
			Message: "Cannot find name 'BrokenThing'.",
			Source:  "tsc",
		},
	}, []GeneratedFile{
		{
			Path: "src/App.tsx",
			Content: `export function App() {
  return <main>{BrokenThing}</main>
}
`,
		},
	}, reliabilitySummaryPromptContext(&BuildReliabilitySummary{
		Status:                "degraded",
		CurrentFailureClass:   "compile_failure",
		AcceptanceSurfaces:    []string{"frontend"},
		PrimaryUserFlows:      []string{"land in the product shell"},
		RecurringFailureClass: []string{"compile_failure"},
		RecommendedFocus:      []string{"expand deterministic compile repair coverage for the current failure class"},
	}), false)

	if !strings.Contains(prompt, "<reliability_summary>") {
		t.Fatalf("expected reliability summary in repair prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "compile_failure") {
		t.Fatalf("expected recurring failure class in repair prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "acceptance_surfaces:") {
		t.Fatalf("expected acceptance surfaces in repair prompt, got %q", prompt)
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

func TestClassifyRepairPatchBundleMarksSmallPatchAutoMergeSafe(t *testing.T) {
	t.Parallel()

	classification := classifyRepairPatchBundle(&PatchBundle{
		Operations: []PatchOperation{
			{
				Type:    PatchReplaceFunction,
				Path:    "src/components/Card.tsx",
				Content: "export function Card(){ return <div /> }",
			},
		},
	})

	if classification.MergePolicy != RepairPatchMergeAutoSafe {
		t.Fatalf("expected auto-merge-safe classification, got %q", classification.MergePolicy)
	}
	if classification.ReviewRequired {
		t.Fatalf("expected review_required=false for small local patch, got %+v", classification)
	}
	if len(classification.Reasons) > 0 {
		t.Fatalf("expected no risk reasons for small local patch, got %+v", classification.Reasons)
	}
}

func TestClassifyRepairPatchBundleMarksRiskyPatchReviewRequired(t *testing.T) {
	t.Parallel()

	classification := classifyRepairPatchBundle(&PatchBundle{
		Operations: []PatchOperation{
			{
				Type:    PatchPatchEnvVar,
				Path:    ".env.production",
				Content: "API_KEY=secret",
			},
		},
	})

	if classification.MergePolicy != RepairPatchMergeReviewRequired {
		t.Fatalf("expected review-required classification, got %q", classification.MergePolicy)
	}
	if !classification.ReviewRequired {
		t.Fatalf("expected review_required=true for env changes, got %+v", classification)
	}
	if !strings.Contains(strings.Join(classification.Reasons, ","), "env_changes_require_review") {
		t.Fatalf("expected env review reason, got %+v", classification.Reasons)
	}
}

func TestCVHydraWinnerPatchBundleAnnotatesMetadataAndMetrics(t *testing.T) {
	t.Parallel()

	output := &TaskOutput{
		StructuredPatchBundle: &PatchBundle{
			Operations: []PatchOperation{
				{
					Type:    PatchCreateFile,
					Path:    "src/newFile.ts",
					Content: "export const ready = true",
				},
			},
		},
	}

	candidate := cvRepairCandidate{
		Strategy: cvRepairStrategy{
			Name: "strict_ast_syntax_repair",
		},
		Provider: ai.ProviderGPT4,
		Output:   output,
	}

	bundle := cvHydraWinnerPatchBundle(&Build{ID: "build-hydra-1"}, candidate, nil)
	if bundle == nil {
		t.Fatal("expected hydra winner patch bundle")
	}
	if bundle.BuildID != "build-hydra-1" {
		t.Fatalf("expected build id to be backfilled, got %q", bundle.BuildID)
	}
	if bundle.Provider != ai.ProviderGPT4 {
		t.Fatalf("expected provider metadata to be attached, got %q", bundle.Provider)
	}
	if bundle.MergePolicy != RepairPatchMergeAutoSafe {
		t.Fatalf("expected auto-safe merge policy for simple create-file patch, got %q", bundle.MergePolicy)
	}
	if strings.TrimSpace(bundle.Justification) == "" || !strings.Contains(bundle.Justification, "strict_ast_syntax_repair") {
		t.Fatalf("expected hydra strategy in justification, got %q", bundle.Justification)
	}

	if output.Metrics == nil {
		t.Fatal("expected output metrics to be initialized")
	}
	if got, ok := output.Metrics["repair_merge_policy"].(string); !ok || got != string(RepairPatchMergeAutoSafe) {
		t.Fatalf("expected repair_merge_policy metric, got %+v", output.Metrics["repair_merge_policy"])
	}
	if got, ok := output.Metrics["hydra_winner_provider"].(string); !ok || got != string(ai.ProviderGPT4) {
		t.Fatalf("expected hydra_winner_provider metric, got %+v", output.Metrics["hydra_winner_provider"])
	}
}

func TestCVHydraWinnerPatchBundleAddsReviewCommitFlowForRiskyPatch(t *testing.T) {
	t.Parallel()

	output := &TaskOutput{
		StructuredPatchBundle: &PatchBundle{
			ID:            "bundle-risky-1",
			CreatedAt:     time.Date(2026, time.April, 12, 16, 30, 0, 0, time.UTC),
			Justification: "Compile validator Hydra winner (targeted_node_rewrite)",
			Operations: []PatchOperation{
				{
					Type:    PatchPatchEnvVar,
					Path:    ".env.production",
					Content: "API_KEY=secret",
				},
			},
		},
	}

	candidate := cvRepairCandidate{
		Strategy: cvRepairStrategy{
			Name: "targeted_node_rewrite",
		},
		Provider: ai.ProviderClaude,
		Output:   output,
	}

	bundle := cvHydraWinnerPatchBundle(&Build{ID: "build-hydra-risky"}, candidate, nil)
	if bundle == nil {
		t.Fatal("expected hydra winner patch bundle")
	}
	if !bundle.ReviewRequired || bundle.MergePolicy != RepairPatchMergeReviewRequired {
		t.Fatalf("expected risky patch to require review, got %+v", bundle)
	}
	if strings.TrimSpace(bundle.ReviewBranch) == "" {
		t.Fatalf("expected review branch annotation, got %+v", bundle)
	}
	if strings.TrimSpace(bundle.SuggestedCommit) == "" {
		t.Fatalf("expected suggested commit title annotation, got %+v", bundle)
	}

	if got, ok := output.Metrics["repair_review_branch"].(string); !ok || strings.TrimSpace(got) == "" {
		t.Fatalf("expected repair_review_branch metric, got %+v", output.Metrics["repair_review_branch"])
	}
	if got, ok := output.Metrics["repair_suggested_commit_title"].(string); !ok || strings.TrimSpace(got) == "" {
		t.Fatalf("expected repair_suggested_commit_title metric, got %+v", output.Metrics["repair_suggested_commit_title"])
	}
}
