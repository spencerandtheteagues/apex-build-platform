package agents

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"apex-build/internal/ai"

	"github.com/google/uuid"
)

const maxRepairMemoryPromptMatches = 3

type repairMemoryObservation struct {
	TaskShape           TaskShape
	Provider            ai.AIProvider
	Model               string
	FailureClass        string
	FilesInvolved       []string
	RepairPathChosen    []string
	RepairStrategy      string
	PatchClass          string
	RepairSucceeded     bool
	TokenCostToRecovery int
}

func appendRepairMemoryFingerprint(build *Build, observation repairMemoryObservation) {
	if build == nil {
		return
	}
	appendFailureFingerprint(build, FailureFingerprint{
		ID:                  uuid.New().String(),
		BuildID:             build.ID,
		StackCombination:    stackCombinationFromBuild(build),
		TaskShape:           observation.TaskShape,
		Provider:            observation.Provider,
		Model:               strings.TrimSpace(observation.Model),
		FailureClass:        normalizeFailureIdentifier(observation.FailureClass),
		FilesInvolved:       normalizeRepairMemoryFiles(observation.FilesInvolved),
		RepairPathChosen:    dedupeStrings(observation.RepairPathChosen),
		RepairStrategy:      strings.TrimSpace(observation.RepairStrategy),
		PatchClass:          normalizeRepairMemoryIdentifier(observation.PatchClass),
		RepairSucceeded:     observation.RepairSucceeded,
		TokenCostToRecovery: observation.TokenCostToRecovery,
		CreatedAt:           time.Now().UTC(),
	})
}

func prepareFailureFingerprint(build *Build, fingerprint FailureFingerprint) FailureFingerprint {
	if strings.TrimSpace(fingerprint.ID) == "" {
		fingerprint.ID = uuid.New().String()
	}
	if build != nil && strings.TrimSpace(fingerprint.BuildID) == "" {
		fingerprint.BuildID = build.ID
	}
	if build != nil && strings.TrimSpace(fingerprint.StackCombination) == "" {
		fingerprint.StackCombination = stackCombinationFromBuild(build)
	}
	fingerprint.FailureClass = normalizeFailureIdentifier(fingerprint.FailureClass)
	fingerprint.FilesInvolved = normalizeRepairMemoryFiles(fingerprint.FilesInvolved)
	fingerprint.RepairPathChosen = dedupeStrings(fingerprint.RepairPathChosen)
	fingerprint.RepairStrategy = strings.TrimSpace(fingerprint.RepairStrategy)
	fingerprint.PatchClass = normalizeRepairMemoryIdentifier(fingerprint.PatchClass)
	if fingerprint.CreatedAt.IsZero() {
		fingerprint.CreatedAt = time.Now().UTC()
	}
	if strings.TrimSpace(fingerprint.FingerprintKey) == "" {
		fingerprint.FingerprintKey = repairMemoryFingerprintKey(fingerprint)
	}
	return fingerprint
}

func repairMemoryFingerprintKey(fingerprint FailureFingerprint) string {
	parts := []string{
		strings.TrimSpace(fingerprint.StackCombination),
		string(fingerprint.TaskShape),
		normalizeFailureIdentifier(fingerprint.FailureClass),
		normalizeRepairMemoryIdentifier(fingerprint.PatchClass),
		strings.Join(normalizeRepairMemoryFiles(fingerprint.FilesInvolved), ","),
	}
	return strings.Trim(strings.Join(parts, "|"), "|")
}

func normalizeRepairMemoryIdentifier(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return ""
	}
	replacer := strings.NewReplacer("-", "_", ":", "_", "/", "_", " ", "_", "\t", "_", "\r", "_", "\n", "_")
	trimmed = replacer.Replace(trimmed)
	for strings.Contains(trimmed, "__") {
		trimmed = strings.ReplaceAll(trimmed, "__", "_")
	}
	return strings.Trim(trimmed, "_")
}

func repairPatchClassFromBundle(bundle *PatchBundle) string {
	if bundle == nil {
		return ""
	}
	if bundle.WholeFileRewrite {
		return "whole_file_rewrite"
	}

	classes := map[string]bool{}
	for _, op := range bundle.Operations {
		if class := repairPatchClassFromOperation(op); class != "" {
			classes[class] = true
		}
	}
	switch {
	case classes["dependency_manifest"]:
		return "dependency_manifest"
	case classes["import_export_mismatch"]:
		return "import_export_mismatch"
	case classes["route_registration"]:
		return "route_registration"
	case classes["schema"]:
		return "schema"
	case classes["env"]:
		return "env"
	case classes["missing_file"]:
		return "missing_file"
	case len(classes) > 1:
		return "multi_class_patch"
	case len(classes) == 1:
		for class := range classes {
			return class
		}
	case len(bundle.Operations) > 1:
		return "multi_file_patch"
	}
	return "targeted_patch"
}

func repairPatchClassFromOperation(op PatchOperation) string {
	path := strings.ToLower(filepath.ToSlash(strings.TrimSpace(op.Path)))
	content := strings.ToLower(strings.TrimSpace(op.Content))
	switch {
	case op.Type == PatchPatchDependency || path == "package.json" || strings.HasSuffix(path, "/package.json"):
		return "dependency_manifest"
	case op.Type == PatchPatchRouteRegistration:
		return "route_registration"
	case op.Type == PatchPatchSchemaEntity:
		return "schema"
	case op.Type == PatchPatchEnvVar:
		return "env"
	case op.Type == PatchCreateFile:
		return "missing_file"
	case strings.Contains(content, "export ") || strings.Contains(content, "import "):
		return "import_export_mismatch"
	case op.Type == PatchReplaceFunction || op.Type == PatchReplaceSymbol || op.Type == PatchInsertAfterSymbol || op.Type == PatchRenameSymbol:
		return "symbol_patch"
	case op.Type == PatchPatchJSONKey:
		return "json_manifest"
	default:
		return ""
	}
}

func repairMemoryFilesFromPatchBundle(bundle *PatchBundle) []string {
	if bundle == nil {
		return nil
	}
	files := make([]string, 0, len(bundle.Operations))
	for _, op := range bundle.Operations {
		if trimmed := strings.TrimSpace(op.Path); trimmed != "" {
			files = append(files, trimmed)
		}
	}
	return normalizeRepairMemoryFiles(files)
}

func normalizeRepairMemoryFiles(files []string) []string {
	if len(files) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(files))
	for _, file := range files {
		trimmed := filepath.ToSlash(strings.TrimSpace(file))
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	sort.Strings(out)
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

func parsedBuildErrorFiles(errors []ParsedBuildError) []string {
	files := make([]string, 0, len(errors))
	for _, parsed := range errors {
		if trimmed := strings.TrimSpace(parsed.File); trimmed != "" {
			files = append(files, trimmed)
		}
	}
	return normalizeRepairMemoryFiles(files)
}

func repairMemoryPromptContextForBuild(build *Build, failureClass string, files []string) string {
	matches := recentSuccessfulRepairFingerprints(build, failureClass, files, maxRepairMemoryPromptMatches)
	if len(matches) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<repair_memory>\n")
	sb.WriteString(fmt.Sprintf("matched_failure_class: %s\n", normalizeFailureIdentifier(failureClass)))
	sb.WriteString("successful_repairs:\n")
	for _, fp := range matches {
		sb.WriteString(fmt.Sprintf("- strategy=%s patch_class=%s", strings.TrimSpace(fp.RepairStrategy), strings.TrimSpace(fp.PatchClass)))
		if len(fp.FilesInvolved) > 0 {
			sb.WriteString(" files=" + strings.Join(fp.FilesInvolved, ","))
		}
		if len(fp.RepairPathChosen) > 0 {
			sb.WriteString(" path=" + strings.Join(fp.RepairPathChosen, " -> "))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("Use these as repair priors only; still produce the smallest truthful patch and rely on deterministic validation.\n")
	sb.WriteString("</repair_memory>\n")
	return sb.String()
}

func recentSuccessfulRepairFingerprints(build *Build, failureClass string, files []string, limit int) []FailureFingerprint {
	if build == nil || limit == 0 {
		return nil
	}
	normalizedClass := normalizeFailureIdentifier(failureClass)
	if normalizedClass == "" {
		return nil
	}
	targetFiles := normalizeRepairMemoryFiles(files)

	build.mu.RLock()
	orchestration := cloneBuildOrchestrationState(build.SnapshotState.Orchestration)
	build.mu.RUnlock()
	if orchestration == nil || len(orchestration.FailureFingerprints) == 0 {
		return nil
	}

	matches := make([]FailureFingerprint, 0, maxRepairMemoryPromptMatches)
	for i := len(orchestration.FailureFingerprints) - 1; i >= 0; i-- {
		fp := prepareFailureFingerprint(build, orchestration.FailureFingerprints[i])
		if !fp.RepairSucceeded || normalizeFailureIdentifier(fp.FailureClass) != normalizedClass {
			continue
		}
		if strings.TrimSpace(fp.RepairStrategy) == "" && strings.TrimSpace(fp.PatchClass) == "" {
			continue
		}
		if len(targetFiles) > 0 && len(fp.FilesInvolved) > 0 && !repairMemoryFilesOverlap(targetFiles, fp.FilesInvolved) {
			continue
		}
		matches = append(matches, fp)
		if limit > 0 && len(matches) >= limit {
			break
		}
	}
	return matches
}

func repairMemoryFilesOverlap(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return true
	}
	seen := map[string]bool{}
	for _, file := range normalizeRepairMemoryFiles(a) {
		seen[file] = true
	}
	for _, file := range normalizeRepairMemoryFiles(b) {
		if seen[file] {
			return true
		}
	}
	return false
}
