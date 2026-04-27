// preview_gate.go — Preview readiness verification gate for the agent manager.
// Runs after code validation passes and before a build is declared complete.
// Ensures that generated output would produce a loadable interactive preview.
package agents

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// VerifiableFile is the minimal file representation passed to the preview verifier.
// It mirrors GeneratedFile without importing the preview package.
type VerifiableFile struct {
	Path    string
	Content string
}

// PreviewVerificationResult is the result of a preview readiness check.
type PreviewVerificationResult struct {
	Passed                       bool
	FailureKind                  string   // e.g. "missing_entrypoint", "blank_screen", "corrupt_content"
	RepairHints                  []string // Actionable directives for the repair agent
	Details                      string   // Human-readable failure description
	ScreenshotBase64             string
	CanaryErrors                 []string
	CanaryClickCount             int
	CanaryVisibleControls        int    // number of visible interactive controls detected on first load
	CanaryPostInteractionChecked bool   // true when the canary completed the post-click settle check
	CanaryPostInteractionHealthy bool   // false when the preview blanked after basic interactions
	VisionSeverity               string // "critical", "advisory", "clean", or "" when vision skipped
}

// BuildPreviewVerifier is the interface the agent manager uses for preview verification.
// Implemented in the preview package; wired via SetPreviewVerifier in main.go.
type BuildPreviewVerifier interface {
	VerifyBuildFiles(ctx context.Context, files []VerifiableFile, isFullStack bool) *PreviewVerificationResult
}

func includePreviewVerificationFile(path string) bool {
	lower := strings.ToLower(sanitizeFilePath(path))
	switch {
	case lower == "":
		return false
	case strings.HasPrefix(lower, "e2e/"), strings.Contains(lower, "/e2e/"):
		return false
	case strings.HasPrefix(lower, "tests/"), strings.Contains(lower, "/__tests__/"), strings.HasPrefix(lower, "__tests__/"):
		return false
	case strings.Contains(lower, ".test."), strings.Contains(lower, ".spec."):
		return false
	default:
		return true
	}
}

// SetPreviewVerifier wires a BuildPreviewVerifier into the agent manager.
func (am *AgentManager) SetPreviewVerifier(v BuildPreviewVerifier) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.previewVerifier = v
}

// runPreviewVerificationGate verifies that the generated files would produce a
// loadable preview. Called from runBuildFinalization after code validation passes.
//
// Returns true when the caller should return early (a repair task was queued).
// Returns false in all other cases; status/buildError may have been updated to
// BuildFailed if the gate failed and no repair was available.
func (am *AgentManager) runPreviewVerificationGate(
	build *Build,
	allFiles []GeneratedFile,
	status *BuildStatus,
	buildError *string,
	now time.Time,
) bool {
	if am.previewVerifier == nil || *status != BuildCompleted {
		return false // gate not configured or build already failed — skip
	}

	isFS := buildHasRuntimeIntegrationSurface(build)

	vFiles := make([]VerifiableFile, 0, len(allFiles))
	for _, f := range allFiles {
		if includePreviewVerificationFile(f.Path) {
			vFiles = append(vFiles, VerifiableFile{Path: f.Path, Content: f.Content})
		}
	}

	ctx, cancel := context.WithTimeout(am.ctx, 30*time.Second)
	defer cancel()

	pLog(build.ID).PreviewGateStart(len(vFiles), isFS)
	gateStart := time.Now()

	checksRun := []string{"preview_entrypoint", "preview_content", "preview_structure"}
	result := am.previewVerifier.VerifyBuildFiles(ctx, vFiles, isFS)

	// Emit gate telemetry for all outcomes.
	{
		passed := result == nil || result.Passed
		failureKind := ""
		visionSev := ""
		canaryClicked, canaryErrors := 0, 0
		if result != nil {
			if !result.Passed {
				failureKind = result.FailureKind
			}
			visionSev = result.VisionSeverity
			canaryClicked = result.CanaryClickCount
			canaryErrors = len(result.CanaryErrors)
		}
		pLog(build.ID).PreviewGateDone(passed, failureKind, visionSev, canaryClicked, canaryErrors, time.Since(gateStart).Milliseconds())
	}

	if result == nil || result.Passed {
		passedWarnings := []string(nil)
		canaryClicked := 0
		canaryErrors := 0
		visionReviewed := false
		if result != nil {
			passedWarnings = appendUniquePreviewWarnings(result.RepairHints, result.CanaryErrors)
			canaryClicked = result.CanaryClickCount
			canaryErrors = len(result.CanaryErrors)
			for _, w := range passedWarnings {
				if strings.HasPrefix(strings.TrimSpace(w), "visual:") {
					visionReviewed = true
					break
				}
			}
		}
		appendVerificationReport(build, VerificationReport{
			ID:               uuid.New().String(),
			BuildID:          build.ID,
			Phase:            "preview_verification",
			Surface:          SurfaceGlobal,
			Status:           VerificationPassed,
			Deterministic:    true,
			ChecksRun:        checksRun,
			Warnings:         passedWarnings,
			CanaryClickCount: canaryClicked,
			CanaryErrorCount: canaryErrors,
			VisionReviewed:   visionReviewed,
			GeneratedAt:      now.UTC(),
		})

		frontendFiles := frontendFilePathsFromFiles(allFiles)

		// Point 3: if this is a second-pass result after a repair attempt, record
		// the outcome — repair succeeded since the gate just passed.
		build.mu.RLock()
		prevAttempts := build.PreviewVerificationAttempts
		build.mu.RUnlock()
		if prevAttempts >= 1 {
			recordPreviewRepairOutcome(build, true, frontendFiles)
		}

		// Vision repair gate: when vision found critical visual issues and the
		// feature flag is enabled, launch a targeted repair pass before the build
		// is declared complete. This is best-effort — never a hard blocker.
		// Guarded by PreviewVerificationAttempts so we only attempt once.
		if result != nil && result.VisionSeverity == "critical" &&
			result.ScreenshotBase64 != "" &&
			os.Getenv("APEX_VISION_REPAIR") != "false" {
			if prevAttempts < 1 {
				criticalHints := filterVisualCriticalHints(result.RepairHints)
				if len(criticalHints) > 0 {
					visionFailResult := &PreviewVerificationResult{
						Passed:           false,
						FailureKind:      "visual_critical",
						Details:          "Vision analysis detected critical visual issues: " + summarizeHints(criticalHints, 2),
						RepairHints:      criticalHints,
						ScreenshotBase64: result.ScreenshotBase64,
						CanaryClickCount: result.CanaryClickCount,
						CanaryErrors:     result.CanaryErrors,
					}
					if am.launchPreviewRepairTask(build, allFiles, visionFailResult, now) {
						log.Printf("Build %s: vision critical repair task launched (%d hints)", build.ID, len(criticalHints))
						// Point 2: record the repair launch as a failure fingerprint.
						recordPreviewRepairLaunch(build, "visual_layout", frontendFiles)
						return true
					}
				}
			}
		}

		// Interaction repair gate: when the canary signals an interaction-dead
		// state (no visible controls on a UI app, or the preview blanked after
		// basic clicks), attempt a targeted repair before declaring success.
		// Guarded by PreviewVerificationAttempts; best-effort, never a hard blocker.
		if result != nil && interactionCriticalSignal(result) &&
			os.Getenv("APEX_INTERACTION_REPAIR") != "false" {
			if prevAttempts < 1 {
				hints := buildInteractionRepairHints(result)
				if len(hints) > 0 {
					interactionFailResult := &PreviewVerificationResult{
						Passed:                       false,
						FailureKind:                  "interaction_dead",
						Details:                      buildInteractionFailureDetail(result),
						RepairHints:                  hints,
						CanaryErrors:                 result.CanaryErrors,
						CanaryClickCount:             result.CanaryClickCount,
						CanaryVisibleControls:        result.CanaryVisibleControls,
						CanaryPostInteractionChecked: result.CanaryPostInteractionChecked,
						CanaryPostInteractionHealthy: result.CanaryPostInteractionHealthy,
					}
					if am.launchPreviewRepairTask(build, allFiles, interactionFailResult, now) {
						log.Printf("Build %s: interaction repair task launched (controls=%d, postHealthy=%t, canaryErrs=%d)",
							build.ID, result.CanaryVisibleControls, result.CanaryPostInteractionHealthy, len(result.CanaryErrors))
						// Point 2: record the repair launch as a failure fingerprint.
						recordPreviewRepairLaunch(build, "interaction_canary", frontendFiles)
						return true
					}
				}
			}
		}

		return false // gate passed — caller continues normally
	}

	log.Printf("Build %s: preview verification failed (%s): %s", build.ID, result.FailureKind, result.Details)

	// Record as a verification report so the frontend can surface it.
	failureChecks := append([]string(nil), checksRun...)
	failureChecks = append(failureChecks, fmt.Sprintf("failure_class:%s", previewFailureClass(result.FailureKind)))
	appendVerificationReport(build, VerificationReport{
		ID:            uuid.New().String(),
		BuildID:       build.ID,
		Phase:         "preview_verification",
		Surface:       SurfaceGlobal,
		Status:        VerificationFailed,
		Deterministic: true,
		ChecksRun:     failureChecks,
		Errors:        []string{result.Details},
		Blockers:      []string{fmt.Sprintf("preview_verification_failed:%s", result.FailureKind)},
		GeneratedAt:   now.UTC(),
	})

	// Check whether we've already attempted repair.
	build.mu.RLock()
	attempts := build.PreviewVerificationAttempts
	build.mu.RUnlock()

	if attempts >= 1 {
		// Already tried once. Terminate with failure.
		// Point 3: record repair outcome — failed second pass.
		recordPreviewRepairOutcome(build, false, frontendFilePathsFromFiles(allFiles))
		errMsg := fmt.Sprintf("Preview verification failed after repair attempt (%s): %s", result.FailureKind, result.Details)
		*status = BuildFailed
		*buildError = errMsg
		build.mu.Lock()
		build.Status = BuildFailed
		if build.Progress > 99 {
			build.Progress = 99
		}
		build.Error = errMsg
		build.CompletedAt = &now
		build.UpdatedAt = now
		build.mu.Unlock()
		return false
	}

	// ── Attempt 1: Deterministic in-line repair ─────────────────────────
	if am.applyPreviewDeterministicRepair(build, allFiles, result, now) {
		return true // repair applied, caller should return so finalization restarts
	}

	// ── Attempt 2: AI-guided repair task ────────────────────────────────
	if am.launchPreviewRepairTask(build, allFiles, result, now) {
		return true // repair task queued, caller should return
	}

	// No repair available — fail the build.
	errMsg := fmt.Sprintf("Preview verification failed (%s): %s", result.FailureKind, result.Details)
	*status = BuildFailed
	*buildError = errMsg
	build.mu.Lock()
	build.Status = BuildFailed
	if build.Progress > 99 {
		build.Progress = 99
	}
	build.Error = errMsg
	build.CompletedAt = &now
	build.UpdatedAt = now
	build.mu.Unlock()
	return false
}

// applyPreviewDeterministicRepair applies in-memory fixes for common preview
// failures that don't require AI (e.g. stripping markdown fences).
// Returns true if a repair was applied and finalization should restart.
func (am *AgentManager) applyPreviewDeterministicRepair(
	build *Build,
	allFiles []GeneratedFile,
	result *PreviewVerificationResult,
	now time.Time,
) bool {
	if result == nil {
		return false
	}

	switch result.FailureKind {
	case "missing_entrypoint":
		return am.applyPreviewMissingFrontendShellRepair(build, result, now)
	case "corrupt_content":
		return am.applyPreviewFenceStripRepair(build, allFiles, result, now)
	case "js_runtime_error":
		return am.applyPreviewRouterContextRepair(build, result, now)
	}
	return false
}

func (am *AgentManager) applyPreviewMissingFrontendShellRepair(
	build *Build,
	result *PreviewVerificationResult,
	now time.Time,
) bool {
	if build == nil || result == nil || !strings.EqualFold(strings.TrimSpace(result.FailureKind), "missing_entrypoint") {
		return false
	}

	readinessErrors := []string{
		"No recognized frontend entry point found (index.html, src/main.tsx, src/index.tsx, etc.).",
	}
	bundle, summary := am.applyDeterministicMissingFrontendShellRepair(build, readinessErrors)
	if bundle == nil {
		return false
	}
	bundle.BuildID = build.ID
	if strings.TrimSpace(bundle.Justification) == "" && strings.TrimSpace(summary) != "" {
		bundle.Justification = "missing_frontend_shell_repair: " + summary
	}
	if !am.applyPatchBundleToBuild(build, bundle) {
		return false
	}
	if previewPatchBundleRecordingEnabled(build) {
		appendPatchBundle(build, *bundle)
	}

	build.mu.Lock()
	build.PreviewVerificationAttempts++
	build.Status = BuildTesting
	build.CompletedAt = nil
	build.UpdatedAt = now
	build.Progress = 95
	build.Error = fmt.Sprintf("Preview verification: synthesized a frontend shell after missing-entrypoint failure. Re-checking. (%s)", result.Details)
	build.mu.Unlock()

	log.Printf("Build %s: preview missing-entrypoint repair applied (%s), re-checking", build.ID, summary)
	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   build.ID,
		Timestamp: now,
		Data: map[string]any{
			"phase":   "preview_verification",
			"status":  string(BuildTesting),
			"message": "Preview verification: synthesized a previewable frontend shell after missing entrypoint detection. Re-checking preview readiness.",
		},
	})
	am.checkBuildCompletion(build)
	return true
}

// applyPreviewFenceStripRepair removes unmatched markdown code fences from
// frontend entry files (the most common preview corruption artifact).
func (am *AgentManager) applyPreviewFenceStripRepair(
	build *Build,
	allFiles []GeneratedFile,
	result *PreviewVerificationResult,
	now time.Time,
) bool {
	beforeFiles := am.collectGeneratedFiles(build)
	if len(beforeFiles) == 0 && len(allFiles) > 0 {
		beforeFiles = append([]GeneratedFile(nil), allFiles...)
	}
	frontendEntries := []string{
		"index.html", "public/index.html",
		"src/main.tsx", "src/main.ts", "src/main.jsx", "src/main.js",
		"src/index.tsx", "src/index.ts",
	}

	repaired := false
	for _, entry := range frontendEntries {
		build.mu.Lock()
		for _, t := range build.Tasks {
			if t == nil || t.Output == nil {
				continue
			}
			for j := range t.Output.Files {
				if t.Output.Files[j].Path != entry {
					continue
				}
				content := t.Output.Files[j].Content
				fenceCount := strings.Count(content, "```")
				if fenceCount == 0 || fenceCount%2 == 0 {
					continue
				}
				t.Output.Files[j].Content = strings.ReplaceAll(content, "```", "")
				repaired = true
			}
		}
		build.mu.Unlock()
	}

	if !repaired {
		return false
	}
	am.recordPreviewDeterministicRepairPatchBundle(
		build,
		beforeFiles,
		am.collectGeneratedFiles(build),
		"Preview deterministic repair: stripped markdown fence artifacts from entry files",
	)

	build.mu.Lock()
	build.PreviewVerificationAttempts++
	build.Status = BuildTesting
	build.CompletedAt = nil
	build.UpdatedAt = now
	build.Progress = 95
	build.Error = fmt.Sprintf("Preview verification: stripped markdown fence artifacts. Re-checking. (%s)", result.Details)
	build.mu.Unlock()

	log.Printf("Build %s: preview fence-strip repair applied, re-checking", build.ID)
	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   build.ID,
		Timestamp: now,
		Data: map[string]any{
			"phase":   "preview_verification",
			"status":  string(BuildTesting),
			"message": "Preview verification: stripped markdown artifacts from entry files. Re-checking preview readiness.",
		},
	})
	am.checkBuildCompletion(build)
	return true
}

var reactRouterNamedImportPattern = regexp.MustCompile(`(?m)^import\s*\{([^}]*)\}\s*from\s*["']react-router-dom["'];?\s*$`)

func previewFailureLooksLikeMissingRouterContext(result *PreviewVerificationResult) bool {
	if result == nil || !strings.EqualFold(strings.TrimSpace(result.FailureKind), "js_runtime_error") {
		return false
	}
	haystack := strings.ToLower(strings.TrimSpace(result.Details + "\n" + strings.Join(result.RepairHints, "\n")))
	return strings.Contains(haystack, "linkwithref") ||
		strings.Contains(haystack, "basename") ||
		strings.Contains(haystack, "react-router-dom") ||
		strings.Contains(haystack, "browserrouter") ||
		strings.Contains(haystack, "router context")
}

func (am *AgentManager) applyPreviewRouterContextRepair(
	build *Build,
	result *PreviewVerificationResult,
	now time.Time,
) bool {
	if build == nil || !previewFailureLooksLikeMissingRouterContext(result) {
		return false
	}

	beforeFiles := am.collectGeneratedFiles(build)
	entryCandidates := map[string]bool{
		"src/main.tsx": true,
		"src/main.ts":  true,
		"src/main.jsx": true,
		"src/main.js":  true,
	}

	repaired := false
	packagePatched := false

	build.mu.Lock()
	for _, task := range build.Tasks {
		if task == nil || task.Output == nil {
			continue
		}
		for i := range task.Output.Files {
			path := strings.TrimSpace(task.Output.Files[i].Path)
			switch {
			case entryCandidates[path]:
				updated, changed := wrapPreviewEntryWithBrowserRouter(task.Output.Files[i].Content)
				if !changed {
					continue
				}
				task.Output.Files[i].Content = updated
				repaired = true
			case path == "package.json":
				updated, added := patchManifestDependenciesJSON(task.Output.Files[i].Content, []string{"react-router-dom"})
				if len(added) == 0 {
					continue
				}
				task.Output.Files[i].Content = updated
				packagePatched = true
			}
		}
	}
	build.mu.Unlock()

	if !repaired {
		return false
	}
	am.recordPreviewDeterministicRepairPatchBundle(
		build,
		beforeFiles,
		am.collectGeneratedFiles(build),
		"Preview deterministic repair: wrapped preview entry with BrowserRouter",
	)

	build.mu.Lock()
	build.PreviewVerificationAttempts++
	build.Status = BuildTesting
	build.CompletedAt = nil
	build.UpdatedAt = now
	build.Progress = 95
	build.Error = fmt.Sprintf("Preview verification: wrapped the app entry with BrowserRouter after router-context runtime failure. Re-checking. (%s)", result.Details)
	build.mu.Unlock()

	log.Printf("Build %s: preview router-context repair applied (package patched=%t), re-checking", build.ID, packagePatched)
	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   build.ID,
		Timestamp: now,
		Data: map[string]any{
			"phase":   "preview_verification",
			"status":  string(BuildTesting),
			"message": "Preview verification: wrapped the entry in BrowserRouter after a router-context runtime failure. Re-checking preview readiness.",
		},
	})
	am.checkBuildCompletion(build)
	return true
}

func (am *AgentManager) recordPreviewDeterministicRepairPatchBundle(build *Build, beforeFiles, afterFiles []GeneratedFile, justification string) {
	if build == nil || !previewPatchBundleRecordingEnabled(build) {
		return
	}
	bundle := buildPatchBundleFromFileDiff(build.ID, justification, beforeFiles, afterFiles)
	if bundle == nil {
		return
	}
	appendPatchBundle(build, *bundle)
}

func previewPatchBundleRecordingEnabled(build *Build) bool {
	if build == nil {
		return false
	}
	build.mu.RLock()
	orchestration := build.SnapshotState.Orchestration
	if orchestration != nil {
		enabled := orchestration.Flags.EnablePatchBundles
		build.mu.RUnlock()
		return enabled
	}
	build.mu.RUnlock()
	return defaultBuildOrchestrationFlags().EnablePatchBundles
}

func wrapPreviewEntryWithBrowserRouter(content string) (string, bool) {
	content = strings.TrimSpace(content)
	if content == "" || strings.Contains(content, "BrowserRouter") {
		return content, false
	}

	appPatterns := []string{"<App />", "<App/>"}
	replacement := "<BrowserRouter>\n      <App />\n    </BrowserRouter>"
	replaced := false
	for _, pattern := range appPatterns {
		if strings.Contains(content, pattern) {
			content = strings.Replace(content, pattern, replacement, 1)
			replaced = true
			break
		}
	}
	if !replaced {
		return content, false
	}

	if reactRouterNamedImportPattern.MatchString(content) {
		content = reactRouterNamedImportPattern.ReplaceAllStringFunc(content, func(match string) string {
			submatches := reactRouterNamedImportPattern.FindStringSubmatch(match)
			if len(submatches) < 2 {
				return match
			}
			specs := strings.TrimSpace(submatches[1])
			if strings.Contains(specs, "BrowserRouter") {
				return match
			}
			if specs == "" {
				return `import { BrowserRouter } from "react-router-dom";`
			}
			return fmt.Sprintf(`import { BrowserRouter, %s } from "react-router-dom";`, specs)
		})
		return content, true
	}

	importLine := `import { BrowserRouter } from "react-router-dom";`
	if idx := strings.Index(content, "\nimport App"); idx != -1 {
		content = content[:idx+1] + importLine + "\n" + content[idx+1:]
		return content, true
	}
	if idx := strings.Index(content, "\n"); idx != -1 {
		content = content[:idx+1] + importLine + "\n" + content[idx+1:]
		return content, true
	}
	return importLine + "\n" + content, true
}

// launchPreviewRepairTask enqueues an AI-guided recovery task targeting the
// specific files and failure identified by the verifier.
func (am *AgentManager) launchPreviewRepairTask(
	build *Build,
	allFiles []GeneratedFile,
	result *PreviewVerificationResult,
	now time.Time,
) bool {
	if !am.canCreateAutomatedFixTask(build, "fix_preview_verification") {
		return false
	}

	build.mu.Lock()
	build.PreviewVerificationAttempts++
	build.Status = BuildReviewing
	build.CompletedAt = nil
	build.UpdatedAt = now
	build.Progress = 95
	build.Error = fmt.Sprintf("Preview verification failed: %s", result.Details)
	progress := build.Progress
	build.mu.Unlock()

	hints := result.RepairHints
	if len(result.CanaryErrors) > 0 {
		hints = append([]string{fmt.Sprintf("canary_interaction: %s", strings.Join(result.CanaryErrors, "; "))}, hints...)
	}
	if len(hints) == 0 {
		hints = []string{fmt.Sprintf("Fix the %s issue so the preview loads correctly.", result.FailureKind)}
	}

	failedTask := &Task{
		ID:          "preview_verification_check",
		Type:        TaskReview,
		Description: "Preview verification",
		Status:      TaskFailed,
		Input:       buildPreviewRepairTaskInput(build, result, hints),
	}

	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   build.ID,
		Timestamp: now,
		Data: map[string]any{
			"phase":                 "preview_verification",
			"status":                string(BuildReviewing),
			"progress":              progress,
			"message":               fmt.Sprintf("Preview verification failed (%s). Launching targeted repair.", result.FailureKind),
			"quality_gate_required": true,
			"quality_gate_active":   true,
			"quality_gate_stage":    "preview_verification",
			"verification_errors":   []string{result.Details},
		},
	})

	am.enqueueRecoveryTask(build.ID, failedTask, fmt.Errorf("preview verification failed (%s): %s", result.FailureKind, result.Details))
	log.Printf("Build %s: launched preview repair task for %s", build.ID, result.FailureKind)

	_ = allFiles // available for future context selection
	return true
}

func previewHintsContainVisionAdvice(hints []string) bool {
	for _, hint := range hints {
		normalized := strings.ToLower(strings.TrimSpace(hint))
		if strings.HasPrefix(normalized, "vision:") || strings.HasPrefix(normalized, "visual:") {
			return true
		}
	}
	return false
}

func previewHintsContainNoVisibleControls(hints []string) bool {
	for _, hint := range hints {
		normalized := strings.ToLower(strings.TrimSpace(hint))
		if normalized == "" {
			continue
		}
		if strings.Contains(normalized, "no visible buttons") ||
			strings.Contains(normalized, "zero visible interactive") ||
			strings.Contains(normalized, "no visible interactive") ||
			strings.Contains(normalized, "exposes no visible") {
			return true
		}
	}
	return false
}

func buildPreviewRepairTaskInput(build *Build, result *PreviewVerificationResult, hints []string) map[string]any {
	input := map[string]any{
		"failure_kind":    result.FailureKind,
		"failure_details": result.Details,
		"repair_hints":    hints,
		"action":          "fix_preview_verification",
	}
	if result != nil && result.ScreenshotBase64 != "" && previewHintsContainVisionAdvice(hints) {
		input["screenshot_base64"] = result.ScreenshotBase64
	}
	// Inject repair memory so the agent can learn from previous repair strategies
	// on the same failure class.
	if build != nil {
		failureClass := normalizeFailureIdentifier(previewFailureClass(result.FailureKind))
		if failureClass == "" {
			failureClass = "preview_verification"
		}
		if memCtx := repairMemoryPromptContextForBuild(build, failureClass, nil); memCtx != "" {
			input["repair_memory_context"] = memCtx
		}
	}
	return input
}

func previewFailureClass(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "blank_screen", "missing_entrypoint", "corrupt_content", "invalid_html", "invalid_package_json":
		return "frontend_shell"
	case "js_runtime_error", "browser_load_failed":
		return "runtime"
	case "boot_failed":
		return "preview_boot"
	case "browser_unavailable":
		return "infrastructure"
	case "backend_missing", "backend_no_listen", "backend_no_routes":
		return "backend_contract"
	default:
		return "unknown"
	}
}

// filterVisualCriticalHints returns hints that describe critical visual issues
// (blank screen, invisible text, zero CSS) deserving an automated repair attempt.
func filterVisualCriticalHints(hints []string) []string {
	var out []string
	for _, hint := range hints {
		lower := strings.ToLower(strings.TrimSpace(hint))
		// Strip "visual:" prefix before matching.
		lower = strings.TrimPrefix(lower, "visual:")
		lower = strings.TrimSpace(lower)
		if isVisualCriticalHintText(lower) {
			out = append(out, hint)
		}
	}
	return out
}

// isVisualCriticalHintText returns true for hint text describing a visually
// broken state that impairs usability (blank screen, invisible text, zero CSS).
func isVisualCriticalHintText(lower string) bool {
	criticalPhrases := []string{
		"blank screen", "white screen", "empty screen",
		"blank page", "white page",
		"invisible text", "unreadable", "no visible text",
		"dark-on-dark", "light-on-light", "zero contrast",
		"no styling", "no css", "missing css", "unstyled",
		"browser defaults", "no tailwind",
		"nothing rendered", "nothing visible", "no content visible",
		"completely blank", "completely empty", "completely white",
	}
	for _, phrase := range criticalPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// summarizeHints returns the first n hints joined with "; ".
func summarizeHints(hints []string, n int) string {
	if len(hints) == 0 {
		return ""
	}
	if n <= 0 || n >= len(hints) {
		return strings.Join(hints, "; ")
	}
	return strings.Join(hints[:n], "; ")
}

func appendUniquePreviewWarnings(groups ...[]string) []string {
	var flattened []string
	for _, group := range groups {
		flattened = append(flattened, group...)
	}
	seen := make(map[string]struct{}, len(flattened))
	out := make([]string, 0, len(flattened))
	for _, item := range flattened {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

// ── Interaction repair gate helpers ──────────────────────────────────────────

// interactionCriticalSignal returns true when the canary result indicates the
// preview loaded but is interaction-dead or crashes after basic user input.
// Only triggers on UI builds where controls are expected.
func interactionCriticalSignal(result *PreviewVerificationResult) bool {
	if result == nil {
		return false
	}
	// Preview blanked or crashed after basic interactions.
	if result.CanaryClickCount > 0 && result.CanaryPostInteractionChecked && !result.CanaryPostInteractionHealthy {
		return true
	}
	// App mounted but has zero interactive controls. Treat this as blocking only
	// when the runtime canary emitted its explicit no-controls hint; generic
	// advisory interaction errors can be reported on otherwise-passing previews.
	if result.CanaryVisibleControls == 0 && previewHintsContainNoVisibleControls(result.RepairHints) {
		return true
	}
	return false
}

func buildInteractionFailureDetail(result *PreviewVerificationResult) string {
	if result == nil {
		return "interaction canary detected a non-interactive preview"
	}
	if result.CanaryClickCount > 0 && result.CanaryPostInteractionChecked && !result.CanaryPostInteractionHealthy {
		return fmt.Sprintf("preview blanked or crashed after %d interaction(s) — post-click settle check failed", result.CanaryClickCount)
	}
	if result.CanaryVisibleControls == 0 && previewHintsContainNoVisibleControls(result.RepairHints) {
		return "preview mounted but exposes zero visible interactive controls on first load"
	}
	return fmt.Sprintf("interaction canary errors: %s", strings.Join(result.CanaryErrors, "; "))
}

func buildInteractionRepairHints(result *PreviewVerificationResult) []string {
	if result == nil {
		return nil
	}
	var hints []string
	if result.CanaryClickCount > 0 && result.CanaryPostInteractionChecked && !result.CanaryPostInteractionHealthy {
		hints = append(hints,
			"The preview blanked or crashed after basic interactions. Ensure the app does not unmount or throw unhandled errors when buttons, links, or form controls are clicked.",
			"Wrap the root app in an error boundary and ensure routing does not render a blank screen on navigation.",
		)
	}
	if result.CanaryVisibleControls == 0 && previewHintsContainNoVisibleControls(result.RepairHints) {
		hints = append(hints,
			"The preview loaded but has no visible buttons, links, menus, or form controls on the first screen. Add at least one interactive element (CTA button, nav link, or form) to the initial view.",
		)
	}
	for _, e := range result.CanaryErrors {
		if t := strings.TrimSpace(e); t != "" {
			hints = append(hints, "canary: "+t)
		}
	}
	out := hints[:0]
	for _, h := range hints {
		if strings.TrimSpace(h) != "" {
			out = append(out, h)
		}
	}
	return out
}
