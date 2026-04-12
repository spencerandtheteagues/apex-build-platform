package agents

import (
	"path/filepath"
	"strings"
)

type repairPatchClassification struct {
	MergePolicy    RepairPatchMergePolicy
	ReviewRequired bool
	Reasons        []string
}

func classifyRepairPatchBundle(bundle *PatchBundle) repairPatchClassification {
	if bundle == nil || len(bundle.Operations) == 0 {
		return repairPatchClassification{
			MergePolicy:    RepairPatchMergeReviewRequired,
			ReviewRequired: true,
			Reasons:        []string{"empty_patch_bundle"},
		}
	}

	reasons := make([]string, 0, 6)
	uniquePaths := make(map[string]struct{}, len(bundle.Operations))
	totalContentBytes := 0

	for _, op := range bundle.Operations {
		path := strings.TrimSpace(filepath.ToSlash(op.Path))
		if path != "" {
			uniquePaths[path] = struct{}{}
		}
		totalContentBytes += len(strings.TrimSpace(op.Content))

		switch op.Type {
		case PatchPatchEnvVar:
			reasons = append(reasons, "env_changes_require_review")
		case PatchPatchSchemaEntity:
			reasons = append(reasons, "schema_changes_require_review")
		case PatchPatchDependency:
			reasons = append(reasons, "dependency_changes_require_review")
		}

		if patchPathTouchesRiskySurface(path) {
			reasons = append(reasons, "risky_surface_requires_review")
		}
	}

	if bundle.WholeFileRewrite {
		reasons = append(reasons, "whole_file_rewrite_requires_review")
	}
	if len(bundle.Operations) > 4 {
		reasons = append(reasons, "large_operation_count")
	}
	if len(uniquePaths) > 2 {
		reasons = append(reasons, "multi_file_patch")
	}
	if totalContentBytes > 6000 {
		reasons = append(reasons, "large_patch_payload")
	}

	reasons = dedupeNonEmptyStrings(reasons)
	if len(reasons) == 0 {
		return repairPatchClassification{
			MergePolicy:    RepairPatchMergeAutoSafe,
			ReviewRequired: false,
		}
	}

	return repairPatchClassification{
		MergePolicy:    RepairPatchMergeReviewRequired,
		ReviewRequired: true,
		Reasons:        reasons,
	}
}

func patchPathTouchesRiskySurface(path string) bool {
	if path == "" {
		return false
	}
	normalized := strings.ToLower(filepath.ToSlash(path))
	if strings.HasSuffix(normalized, ".env") || strings.Contains(normalized, ".env.") {
		return true
	}

	keywords := []string{
		"/auth", "auth/", "jwt", "oauth",
		"/billing", "billing/", "stripe",
		"/deploy", "deploy/", "render.yaml", "docker", "kubernetes",
		"/config", "config/",
		"/migration", "migration/", "schema", "prisma", "/db/",
	}
	for _, keyword := range keywords {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}
	return false
}

func dedupeNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
