package agents

import (
	"strings"
	"testing"
	"time"
)

func TestBuildRepairCommitFlowSkipsAutoSafeBundles(t *testing.T) {
	t.Parallel()

	plan := buildRepairCommitFlow(&PatchBundle{
		ID:          "bundle-safe",
		MergePolicy: RepairPatchMergeAutoSafe,
	})
	if plan != nil {
		t.Fatalf("expected nil repair commit flow for auto-safe bundle, got %+v", plan)
	}
}

func TestBuildRepairCommitFlowGeneratesBranchAndCommitTitle(t *testing.T) {
	t.Parallel()

	bundle := &PatchBundle{
		ID:             "bundle-987654321",
		MergePolicy:    RepairPatchMergeReviewRequired,
		ReviewRequired: true,
		Justification:  "Compile validator Hydra winner (targeted_node_rewrite)",
		CreatedAt:      time.Date(2026, time.April, 12, 14, 15, 0, 0, time.UTC),
	}

	plan := buildRepairCommitFlow(bundle)
	if plan == nil {
		t.Fatal("expected repair commit flow for review-required bundle")
	}
	if !strings.HasPrefix(plan.ReviewBranch, "ai-repair/20260412-") {
		t.Fatalf("expected deterministic review branch prefix, got %q", plan.ReviewBranch)
	}
	if !strings.Contains(plan.ReviewBranch, "bundle-987") {
		t.Fatalf("expected review branch to include bundle id fragment, got %q", plan.ReviewBranch)
	}
	if !strings.Contains(plan.SuggestedCommitTitle, "AI repair:") {
		t.Fatalf("expected suggested commit title to include prefix, got %q", plan.SuggestedCommitTitle)
	}
	if !strings.Contains(plan.SuggestedCommitTitle, "Compile validator Hydra winner") {
		t.Fatalf("expected suggested commit title to include justification, got %q", plan.SuggestedCommitTitle)
	}
}
