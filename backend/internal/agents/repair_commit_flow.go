package agents

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var repairBranchSanitizer = regexp.MustCompile(`[^a-z0-9-]+`)

type repairCommitFlowPlan struct {
	ReviewBranch         string
	SuggestedCommitTitle string
}

func buildRepairCommitFlow(bundle *PatchBundle) *repairCommitFlowPlan {
	if bundle == nil {
		return nil
	}
	if !(bundle.ReviewRequired || bundle.MergePolicy == RepairPatchMergeReviewRequired) {
		return nil
	}

	reviewBranch := strings.TrimSpace(bundle.ReviewBranch)
	if reviewBranch == "" {
		reviewBranch = deriveRepairReviewBranch(bundle)
	}

	commitTitle := strings.TrimSpace(bundle.SuggestedCommit)
	if commitTitle == "" {
		commitTitle = deriveRepairCommitTitle(bundle)
	}

	return &repairCommitFlowPlan{
		ReviewBranch:         reviewBranch,
		SuggestedCommitTitle: commitTitle,
	}
}

func deriveRepairReviewBranch(bundle *PatchBundle) string {
	if bundle == nil {
		return ""
	}

	datePart := time.Now().UTC().Format("20060102")
	if !bundle.CreatedAt.IsZero() {
		datePart = bundle.CreatedAt.UTC().Format("20060102")
	}

	seed := strings.TrimSpace(bundle.WorkOrderID)
	if seed == "" {
		seed = strings.TrimSpace(bundle.Justification)
	}
	if seed == "" {
		seed = strings.TrimSpace(bundle.ID)
	}

	slug := sanitizeRepairBranchPart(seed, 28)
	if slug == "" {
		slug = "hydra-repair"
	}

	idPart := sanitizeRepairBranchPart(bundle.ID, 10)
	if idPart != "" {
		return fmt.Sprintf("ai-repair/%s-%s-%s", datePart, slug, idPart)
	}
	return fmt.Sprintf("ai-repair/%s-%s", datePart, slug)
}

func deriveRepairCommitTitle(bundle *PatchBundle) string {
	if bundle == nil {
		return "AI repair: review-required patch bundle"
	}

	summary := strings.TrimSpace(bundle.Justification)
	if summary == "" {
		switch {
		case len(bundle.RiskReasons) > 0:
			summary = "review-required patch bundle (" + strings.Join(bundle.RiskReasons, ", ") + ")"
		default:
			summary = "review-required patch bundle"
		}
	}

	return truncate("AI repair: "+summary, 96)
}

func sanitizeRepairBranchPart(value string, maxLen int) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return ""
	}

	normalized = strings.ReplaceAll(normalized, "_", "-")
	normalized = strings.ReplaceAll(normalized, "/", "-")
	normalized = repairBranchSanitizer.ReplaceAllString(normalized, "-")
	normalized = strings.Trim(normalized, "-")
	if normalized == "" {
		return ""
	}
	if maxLen > 0 && len(normalized) > maxLen {
		normalized = strings.TrimRight(normalized[:maxLen], "-")
	}
	return normalized
}
