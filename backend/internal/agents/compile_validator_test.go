package agents

import (
	"context"
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
		{name: "fast", mode: PowerFast, want: 2},
		{name: "balanced", mode: PowerBalanced, want: 2},
		{name: "max", mode: PowerMax, want: 3},
		{name: "unknown defaults to fast behavior", mode: PowerMode("unknown"), want: 2},
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

func TestCompileValidationBudgetByPowerMode(t *testing.T) {
	t.Setenv("APEX_COMPILE_VALIDATION_BUDGET_SECONDS", "")
	t.Setenv("APEX_COMPILE_VALIDATION_BUDGET_FAST_SECONDS", "")
	t.Setenv("APEX_COMPILE_VALIDATION_BUDGET_BALANCED_SECONDS", "")
	t.Setenv("APEX_COMPILE_VALIDATION_BUDGET_MAX_SECONDS", "")

	tests := []struct {
		name string
		mode PowerMode
		want time.Duration
	}{
		{name: "fast", mode: PowerFast, want: 8 * time.Minute},
		{name: "balanced", mode: PowerBalanced, want: 10 * time.Minute},
		{name: "max", mode: PowerMax, want: 12 * time.Minute},
		{name: "unknown defaults to fast budget", mode: PowerMode("unknown"), want: 8 * time.Minute},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := compileValidationBudget(tt.mode); got != tt.want {
				t.Fatalf("compileValidationBudget(%q) = %s, want %s", tt.mode, got, tt.want)
			}
		})
	}
}

func TestCompileValidationBudgetHonorsOverrideAndFloor(t *testing.T) {
	t.Setenv("APEX_COMPILE_VALIDATION_BUDGET_SECONDS", "")
	t.Setenv("APEX_COMPILE_VALIDATION_BUDGET_FAST_SECONDS", "30")
	if got := compileValidationBudget(PowerFast); got != cvMinBudget {
		t.Fatalf("expected budget floor %s, got %s", cvMinBudget, got)
	}

	t.Setenv("APEX_COMPILE_VALIDATION_BUDGET_SECONDS", "180")
	if got := compileValidationBudget(PowerMax); got != 3*time.Minute {
		t.Fatalf("expected global override to win, got %s", got)
	}
}

func TestCVRunCommandHonorsParentDeadline(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	started := time.Now()
	_, err := cvRunCommand(ctx, t.TempDir(), time.Minute, "sh", "-c", "sleep 1")
	if err == nil {
		t.Fatal("expected parent context deadline to stop command")
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("expected command to stop near parent deadline, elapsed %s", elapsed)
	}
}

func TestCVRepairPowerModePreservesMaxBuildTier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode PowerMode
		want PowerMode
	}{
		{name: "max stays max", mode: PowerMax, want: PowerMax},
		{name: "balanced stays balanced", mode: PowerBalanced, want: PowerBalanced},
		{name: "fast stays fast", mode: PowerFast, want: PowerFast},
		{name: "empty defaults fast", mode: "", want: PowerFast},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := cvRepairPowerMode(tt.mode); got != tt.want {
				t.Fatalf("cvRepairPowerMode(%q) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestCVPackageManifestSanityIssues(t *testing.T) {
	t.Parallel()

	t.Run("invalid_json", func(t *testing.T) {
		t.Parallel()

		issues := cvPackageManifestSanityIssues([]GeneratedFile{{Path: "package.json", Content: `{"dependencies":`}})
		if len(issues) != 1 || !strings.Contains(issues[0].Message, "invalid JSON") {
			t.Fatalf("expected invalid JSON issue, got %+v", issues)
		}
	})

	t.Run("invalid_dependency_name", func(t *testing.T) {
		t.Parallel()

		issues := cvPackageManifestSanityIssues([]GeneratedFile{{Path: "package.json", Content: `{"dependencies":{"Bad Package":"1.0.0"}}`}})
		if len(issues) != 1 || !strings.Contains(issues[0].Message, "invalid dependency name") {
			t.Fatalf("expected invalid dependency issue, got %+v", issues)
		}
	})

	t.Run("valid_manifest", func(t *testing.T) {
		t.Parallel()

		issues := cvPackageManifestSanityIssues([]GeneratedFile{{Path: "package.json", Content: `{"dependencies":{"@vitejs/plugin-react":"latest","react":"latest"}}`}})
		if len(issues) != 0 {
			t.Fatalf("expected no manifest issues, got %+v", issues)
		}
	})
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

func TestCVSelectInlineRepairProviderPrefersGeminiInPlatformModeWithOllama(t *testing.T) {
	t.Parallel()

	am := &AgentManager{
		aiRouter: &stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderOllama, ai.ProviderGPT4, ai.ProviderGrok, ai.ProviderGemini},
			hasConfiguredProvider: true,
		},
	}
	build := &Build{
		ID:           "compile-repair-platform-routing",
		ProviderMode: "platform",
		PowerMode:    PowerBalanced,
	}

	if got := am.cvSelectInlineRepairProvider(build); got != ai.ProviderGemini {
		t.Fatalf("cvSelectInlineRepairProvider() = %s, want platform repair provider %s", got, ai.ProviderGemini)
	}
}

func TestCVSelectInlineRepairProviderPreservesBYOKOllamaPrimary(t *testing.T) {
	t.Parallel()

	am := &AgentManager{
		aiRouter: &stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderOllama, ai.ProviderGPT4, ai.ProviderGrok},
			hasConfiguredProvider: true,
		},
	}
	build := &Build{
		ID:           "compile-repair-byok-routing",
		ProviderMode: "byok",
		PowerMode:    PowerBalanced,
	}

	if got := am.cvSelectInlineRepairProvider(build); got != ai.ProviderOllama {
		t.Fatalf("cvSelectInlineRepairProvider() = %s, want BYOK provider %s", got, ai.ProviderOllama)
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

func TestCVRecordHydraRepairAttemptFingerprintCapturesFailedStrategy(t *testing.T) {
	t.Parallel()

	build := &Build{
		ID: "hydra-failed-attempt-fingerprint",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	cvRecordHydraRepairAttemptFingerprint(
		build,
		ai.ProviderGPT4,
		cvRepairStrategy{Name: "targeted_node_rewrite"},
		[]ParsedBuildError{{
			File:    "src/App.tsx",
			Code:    "TS2339",
			Message: "Property 'total' does not exist on type 'InvoiceSummary'.",
		}},
		&TaskOutput{Files: []GeneratedFile{{Path: "src/App.tsx", Content: "export default function App(){return null}"}}},
		nil,
		false,
	)

	state := build.SnapshotState.Orchestration
	if state == nil || len(state.FailureFingerprints) != 1 {
		t.Fatalf("expected failed hydra attempt fingerprint, got %+v", state)
	}
	fp := state.FailureFingerprints[0]
	if fp.RepairSucceeded {
		t.Fatalf("expected failed repair attempt, got %+v", fp)
	}
	if fp.RepairStrategy != "targeted_node_rewrite" {
		t.Fatalf("expected hydra strategy metadata, got %+v", fp)
	}
	if fp.PatchClass != "symbol_patch" {
		t.Fatalf("expected semantic patch class from compile error, got %+v", fp)
	}
	if !containsString(fp.RepairPathChosen, "hydra_repair") || !containsString(fp.RepairPathChosen, "targeted_node_rewrite") {
		t.Fatalf("expected hydra repair path, got %+v", fp.RepairPathChosen)
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
