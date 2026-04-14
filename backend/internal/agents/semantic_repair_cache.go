package agents

import (
	"strings"
)

const maxSemanticRepairCacheHints = 3

func semanticRepairCachePromptContext(build *Build, errors []ParsedBuildError) string {
	patchClass := semanticRepairPatchClassForErrors(errors)
	if patchClass == "" || build == nil {
		return ""
	}

	var hints []string
	if build.SnapshotState.Orchestration != nil && build.SnapshotState.Orchestration.HistoricalLearning != nil {
		hints = append(hints, filterSemanticRepairHintsByPatchClass(build.SnapshotState.Orchestration.HistoricalLearning.SemanticRepairHints, patchClass)...)
	}
	for _, fp := range recentSuccessfulRepairFingerprints(build, "compile_failure", parsedBuildErrorFiles(errors), maxSemanticRepairCacheHints) {
		if hint := semanticRepairCacheHintFromFingerprint(fp); hint != "" && strings.Contains(hint, "patch="+patchClass) {
			hints = append(hints, hint)
		}
	}
	hints = limitStrings(dedupeStrings(hints), maxSemanticRepairCacheHints)
	if len(hints) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<semantic_repair_cache>\n")
	sb.WriteString("current_patch_class: " + patchClass + "\n")
	sb.WriteString("matching_repair_hints:\n")
	for _, hint := range hints {
		sb.WriteString("- " + strings.TrimSpace(hint) + "\n")
	}
	sb.WriteString("Use these as narrow technical priors only; do not replay old patches without validating the current files and errors.\n")
	sb.WriteString("</semantic_repair_cache>\n")
	return sb.String()
}

func semanticRepairPatchClassForErrors(errors []ParsedBuildError) string {
	for _, parsed := range errors {
		code := strings.ToUpper(strings.TrimSpace(parsed.Code))
		message := strings.ToLower(strings.TrimSpace(parsed.Message))
		switch {
		case code == "TS2305" || code == "TS2306" || code == "TS1192" || code == "TS2613" || code == "TS2614" ||
			strings.Contains(message, "has no exported member") ||
			strings.Contains(message, "has no default export") ||
			strings.Contains(message, "is not a module"):
			return "import_export_mismatch"
		case code == "TS2307" || strings.Contains(message, "cannot find module"):
			if missing := semanticRepairMissingModuleName(parsed.Message); strings.HasPrefix(missing, ".") || strings.HasPrefix(missing, "/") {
				return "missing_file"
			}
			return "dependency_manifest"
		case code == "TS7016" || strings.Contains(message, "could not find a declaration file for module"):
			if missing := semanticRepairMissingModuleName(parsed.Message); strings.HasPrefix(missing, ".") || strings.HasPrefix(missing, "/") {
				return "missing_file"
			}
			return "dependency_manifest"
		case code == "TS2322" || code == "TS2339" || code == "TS2345" || code == "TS2554" || code == "TS2741" ||
			strings.Contains(message, "is not assignable to type") ||
			strings.Contains(message, "property ") && strings.Contains(message, " does not exist on type") ||
			strings.Contains(message, "is missing in type") ||
			strings.Contains(message, "expected ") && strings.Contains(message, " arguments"):
			return "symbol_patch"
		case code == "TS2304" || strings.Contains(message, "cannot find name"):
			return "symbol_patch"
		}
	}
	return ""
}

func semanticRepairMissingModuleName(message string) string {
	trimmed := strings.TrimSpace(message)
	for _, quote := range []string{"'", "\""} {
		start := strings.Index(trimmed, quote)
		if start < 0 {
			continue
		}
		rest := trimmed[start+1:]
		end := strings.Index(rest, quote)
		if end < 0 {
			continue
		}
		return strings.TrimSpace(rest[:end])
	}
	return ""
}

func filterSemanticRepairHintsByPatchClass(hints []string, patchClass string) []string {
	if len(hints) == 0 || strings.TrimSpace(patchClass) == "" {
		return nil
	}
	needle := "patch=" + strings.TrimSpace(patchClass)
	out := make([]string, 0, len(hints))
	for _, hint := range hints {
		if strings.Contains(strings.TrimSpace(hint), needle) {
			out = append(out, strings.TrimSpace(hint))
		}
	}
	return out
}

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
