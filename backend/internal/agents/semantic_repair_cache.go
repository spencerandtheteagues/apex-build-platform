package agents

import (
	"strings"
)

func semanticRepairCacheHintFromFingerprint(fingerprint FailureFingerprint) string {
	if !fingerprint.RepairSucceeded {
		return ""
	}
	failureClass := normalizeFailureIdentifier(fingerprint.FailureClass)
	patchClass := normalizeRepairMemoryIdentifier(fingerprint.PatchClass)
	if !semanticRepairCacheEligible(failureClass, patchClass) {
		return ""
	}

	parts := []string{}
	if failureClass != "" {
		parts = append(parts, "failure="+failureClass)
	}
	if patchClass != "" {
		parts = append(parts, "patch="+patchClass)
	}
	if strategy := strings.TrimSpace(fingerprint.RepairStrategy); strategy != "" {
		parts = append(parts, "strategy="+strategy)
	}
	if files := normalizeRepairMemoryFiles(fingerprint.FilesInvolved); len(files) > 0 {
		if len(files) > 3 {
			files = files[:3]
		}
		parts = append(parts, "files="+strings.Join(files, ","))
	}
	return strings.Join(parts, " ")
}

func semanticRepairCacheEligible(failureClass, patchClass string) bool {
	switch patchClass {
	case "import_export_mismatch", "dependency_manifest", "symbol_patch", "json_manifest", "missing_file":
		return true
	}
	switch failureClass {
	case "compile_failure", "build_failure", "verification_failure":
		return patchClass != "" && patchClass != "whole_file_rewrite" && patchClass != "multi_class_patch"
	default:
		return false
	}
}
