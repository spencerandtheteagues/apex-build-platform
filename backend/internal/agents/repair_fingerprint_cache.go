package agents

import (
	"fmt"
	"strings"
)

const maxRepairFingerprintCacheMatches = 3

type repairFingerprintCacheEntry struct {
	TaskShape             TaskShape
	FailureClass          string
	SameFailureCount      int
	SameProviderFailures  int
	CrossProviderFailures int
	SuccessfulRecoveries  int
	SuggestedRetry        string
	RecentStrategies      []string
	RecentPatchClasses    []string
	RecentFiles           []string
}

func (am *AgentManager) repairFingerprintCacheLookup(build *Build, agent *Agent, task *Task, failureClass string, base string, insight failureFingerprintInsight) repairFingerprintCacheEntry {
	entry := repairFingerprintCacheEntry{
		TaskShape:             insight.TaskShape,
		FailureClass:          normalizeFailureIdentifier(firstNonEmptyString(failureClass, insight.FailureClass)),
		SameFailureCount:      insight.SameFailureCount,
		SameProviderFailures:  insight.SameProviderFailures,
		CrossProviderFailures: insight.CrossProviderFailures,
		SuccessfulRecoveries:  insight.SuccessfulRecoveries,
	}
	if build == nil || task == nil || entry.FailureClass == "" {
		return entry
	}

	files := taskFingerprintFiles(task, nil)
	matches := recentSuccessfulRepairFingerprints(build, entry.FailureClass, files, maxRepairFingerprintCacheMatches)
	if len(matches) == 0 {
		return entry
	}

	strategies := make([]string, 0, len(matches))
	patchClasses := make([]string, 0, len(matches))
	recentFiles := make([]string, 0, len(matches)*2)
	successfulSolverPath := false
	successfulNarrowPatch := false
	matched := false
	for _, match := range matches {
		if entry.TaskShape != "" && match.TaskShape != "" && match.TaskShape != entry.TaskShape {
			continue
		}
		matched = true
		if strategy := strings.TrimSpace(match.RepairStrategy); strategy != "" {
			strategies = append(strategies, strategy)
		}
		if patchClass := normalizeRepairMemoryIdentifier(match.PatchClass); patchClass != "" {
			patchClasses = append(patchClasses, patchClass)
			if patchClass != "whole_file_rewrite" && patchClass != "multi_class_patch" {
				successfulNarrowPatch = true
			}
		}
		recentFiles = append(recentFiles, match.FilesInvolved...)
		if containsString(match.RepairPathChosen, "repair_work_order") || containsString(match.RepairPathChosen, string(RoutingModeDiagnosisRepair)) {
			successfulSolverPath = true
		}
	}
	if !matched {
		return entry
	}
	entry.RecentStrategies = limitStrings(dedupeStrings(strategies), maxRepairFingerprintCacheMatches)
	entry.RecentPatchClasses = limitStrings(dedupeStrings(patchClasses), maxRepairFingerprintCacheMatches)
	entry.RecentFiles = limitStrings(normalizeRepairMemoryFiles(recentFiles), 6)
	entry.SuggestedRetry = repairFingerprintCacheSuggestedRetry(base, insight, successfulNarrowPatch, successfulSolverPath)
	return entry
}

func repairFingerprintCacheSuggestedRetry(base string, insight failureFingerprintInsight, successfulNarrowPatch bool, successfulSolverPath bool) string {
	base = strings.TrimSpace(base)
	switch base {
	case "", "non_retriable", "abort", "backoff", "reduce_context":
		return ""
	}
	if insight.SuccessfulRecoveries == 0 {
		return ""
	}
	if successfulSolverPath && insight.FixPathFailures >= 2 && insight.SameFailureCount >= 3 {
		return "spawn_solver"
	}
	if successfulNarrowPatch && base == "standard_retry" {
		return "fix_and_retry"
	}
	return ""
}

func repairFingerprintCachePromptContext(entry repairFingerprintCacheEntry) string {
	if entry.FailureClass == "" || entry.SuccessfulRecoveries == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<repair_fingerprint_cache>\n")
	sb.WriteString(fmt.Sprintf("failure_class: %s\n", entry.FailureClass))
	if entry.TaskShape != "" {
		sb.WriteString(fmt.Sprintf("task_shape: %s\n", entry.TaskShape))
	}
	if entry.SameFailureCount > 0 {
		sb.WriteString(fmt.Sprintf("same_failure_count: %d\n", entry.SameFailureCount))
	}
	if entry.SameProviderFailures > 0 {
		sb.WriteString(fmt.Sprintf("same_provider_failures: %d\n", entry.SameProviderFailures))
	}
	if entry.CrossProviderFailures > 0 {
		sb.WriteString(fmt.Sprintf("cross_provider_failures: %d\n", entry.CrossProviderFailures))
	}
	sb.WriteString(fmt.Sprintf("successful_recoveries: %d\n", entry.SuccessfulRecoveries))
	if entry.SuggestedRetry != "" {
		sb.WriteString(fmt.Sprintf("suggested_retry: %s\n", entry.SuggestedRetry))
	}
	if len(entry.RecentStrategies) > 0 {
		sb.WriteString("recent_successful_strategies: " + strings.Join(entry.RecentStrategies, ",") + "\n")
	}
	if len(entry.RecentPatchClasses) > 0 {
		sb.WriteString("recent_successful_patch_classes: " + strings.Join(entry.RecentPatchClasses, ",") + "\n")
	}
	if len(entry.RecentFiles) > 0 {
		sb.WriteString("recent_files: " + strings.Join(entry.RecentFiles, ",") + "\n")
	}
	sb.WriteString("Treat this as a cache hit for diagnosis only; inspect the current files and fix the current failure instead of replaying stale patches.\n")
	sb.WriteString("</repair_fingerprint_cache>\n")
	return sb.String()
}

func repairFingerprintFailureClassForTask(task *Task) string {
	if task == nil {
		return ""
	}
	if task.Error != "" {
		return normalizeFailureClass(task.Error)
	}
	if task.Input == nil {
		return ""
	}
	for _, key := range []string{"failure_class", "failure_error", "build_error", "previous_errors"} {
		raw, ok := task.Input[key]
		if !ok || raw == nil {
			continue
		}
		text := strings.TrimSpace(fmt.Sprintf("%v", raw))
		if text == "" {
			continue
		}
		if !strings.ContainsAny(text, " \t\r\n") {
			return normalizeFailureIdentifier(text)
		}
		return normalizeFailureClass(text)
	}
	return ""
}

func repairFingerprintRetryStrategyForTask(task *Task) string {
	if task == nil {
		return ""
	}
	if task.RetryStrategy != "" {
		return strings.TrimSpace(string(task.RetryStrategy))
	}
	if task.Input == nil {
		return ""
	}
	if raw, ok := task.Input["retry_strategy"]; ok && raw != nil {
		return strings.TrimSpace(fmt.Sprintf("%v", raw))
	}
	return ""
}
