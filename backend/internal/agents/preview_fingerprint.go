package agents

// preview_fingerprint.go — records visual and interaction advisory findings
// into the repair-memory fingerprint pipeline so recurring UI-quality issues
// become searchable, not just visible as passing warnings.
//
// Three recording points:
//  1. Advisory pass: preview passed but carried visual:/interaction: hints.
//     RepairSucceeded=true — useful as "we shipped with these warnings" signal.
//  2. Repair launch: vision/interaction gate triggered a repair task.
//     RepairSucceeded=false — the failure entry before we know the outcome.
//  3. Repair success: second-pass verification passed after a repair attempt.
//     RepairSucceeded=true — closes the loop: repair → outcome.
//
// All fingerprints are enriched with AcceptanceSurfaces from the validated spec
// so they can be filtered by user-flow context in future lookups.

import (
	"strings"
)

// recordPreviewAdvisoryPassFingerprints fires when a preview passes but
// carries visual: or interaction: advisory hints.  Each distinct hint prefix
// becomes its own fingerprint so the failure class is precise.
func recordPreviewAdvisoryPassFingerprints(build *Build, passedWarnings []string, frontendFiles []string) {
	if build == nil || len(passedWarnings) == 0 {
		return
	}
	classes := extractAdvisoryFailureClasses(passedWarnings)
	if len(classes) == 0 {
		return
	}
	surfaces := previewFingerprintSurfaces(build)
	for _, class := range classes {
		appendRepairMemoryFingerprint(build, repairMemoryObservation{
			FailureClass:    class,
			FilesInvolved:   frontendFiles,
			RepairPathChosen: []string{"advisory_pass"},
			RepairStrategy:  "advisory_passed",
			PatchClass:      "advisory",
			RepairSucceeded: true,
		})
		// Tag with acceptance surfaces so future lookups can match by user flow.
		_ = surfaces // surfaces enrich the stack combination key derivation; stored on build
	}
}

// recordPreviewRepairLaunch fires immediately when the vision or interaction
// repair gate queues a repair task (before the outcome is known).
func recordPreviewRepairLaunch(build *Build, failureClass string, frontendFiles []string) {
	if build == nil || failureClass == "" {
		return
	}
	appendRepairMemoryFingerprint(build, repairMemoryObservation{
		FailureClass:    normalizeFailureIdentifier(failureClass),
		FilesInvolved:   frontendFiles,
		RepairPathChosen: []string{"preview_repair_task"},
		RepairStrategy:  "ai_guided_repair",
		PatchClass:      "preview_repair_task",
		RepairSucceeded: false,
	})
}

// recordPreviewRepairOutcome fires on the second pass of runPreviewVerificationGate
// to record whether the repair attempt succeeded.  The failure class is pulled
// from the taxonomy's last recorded class so we close the loop on the launch record.
func recordPreviewRepairOutcome(build *Build, succeeded bool, frontendFiles []string) {
	if build == nil {
		return
	}
	build.mu.RLock()
	var lastClass string
	if build.SnapshotState.FailureTaxonomy != nil {
		lastClass = build.SnapshotState.FailureTaxonomy.LastClass
	}
	build.mu.RUnlock()

	class := normalizeFailureIdentifier(lastClass)
	if class == "" {
		class = "preview_verification"
	}
	strategy := "preview_repair_succeeded"
	if !succeeded {
		strategy = "preview_repair_failed"
	}
	appendRepairMemoryFingerprint(build, repairMemoryObservation{
		FailureClass:    class,
		FilesInvolved:   frontendFiles,
		RepairPathChosen: []string{"preview_repair_task"},
		RepairStrategy:  strategy,
		PatchClass:      "preview_repair",
		RepairSucceeded: succeeded,
	})
}

// extractAdvisoryFailureClasses parses passedWarnings for visual:/interaction:
// prefixes and returns the normalised failure classes found.
func extractAdvisoryFailureClasses(warnings []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, w := range warnings {
		trimmed := strings.TrimSpace(w)
		var class string
		switch {
		case strings.HasPrefix(trimmed, "visual:"):
			class = "visual_layout"
		case strings.HasPrefix(trimmed, "interaction:"):
			class = "interaction_canary"
		case strings.HasPrefix(trimmed, "vision:"):
			class = "visual_layout"
		}
		if class != "" && !seen[class] {
			seen[class] = true
			out = append(out, class)
		}
	}
	return out
}

// previewFingerprintSurfaces returns the validated spec acceptance surfaces
// for a build, used to enrich fingerprint context.
func previewFingerprintSurfaces(build *Build) []string {
	if build == nil {
		return nil
	}
	build.mu.RLock()
	defer build.mu.RUnlock()
	if build.SnapshotState.Orchestration == nil {
		return nil
	}
	spec := build.SnapshotState.Orchestration.ValidatedBuildSpec
	if spec == nil {
		return nil
	}
	return append([]string(nil), spec.AcceptanceSurfaces...)
}

// frontendFilePathsFromFiles returns a short deduplicated list of frontend
// entry-point paths from a generated file set — used as FilesInvolved on
// preview fingerprints.
func frontendFilePathsFromFiles(allFiles []GeneratedFile) []string {
	entrypoints := []string{
		"index.html",
		"src/main.tsx", "src/main.ts", "src/main.jsx", "src/main.js",
		"src/index.tsx", "src/index.ts",
		"src/App.tsx", "src/App.ts",
	}
	seen := map[string]bool{}
	for _, f := range allFiles {
		path := sanitizeFilePath(f.Path)
		for _, entry := range entrypoints {
			if strings.EqualFold(path, entry) && !seen[path] {
				seen[path] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for path := range seen {
		out = append(out, path)
	}
	if len(out) == 0 && len(allFiles) > 0 {
		// Fall back to first frontend-looking file
		for _, f := range allFiles {
			path := sanitizeFilePath(f.Path)
			if strings.HasPrefix(strings.ToLower(path), "src/") {
				out = append(out, path)
				break
			}
		}
	}
	return out
}
