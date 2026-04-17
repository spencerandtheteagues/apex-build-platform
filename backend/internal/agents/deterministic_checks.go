package agents

import (
	"fmt"
	"strings"
)

const (
	verificationReasonDeterministicFailed  = "deterministic_failed"
	verificationReasonDeterministicPassed  = "deterministic_passed"
	verificationReasonProviderCritiqueNeed = "provider_critique_needed"
	verificationReasonProviderCritiqueSkip = "provider_critique_skipped"
)

type deterministicVerificationResult struct {
	Ran                    bool
	Checks                 []string
	Warnings               []string
	Errors                 []string
	DeterministicStatus    string
	ProviderCritiqueStatus string
}

func (am *AgentManager) evaluateDeterministicVerification(build *Build, task *Task, candidate *taskGenerationCandidate) deterministicVerificationResult {
	result := deterministicVerificationResult{
		DeterministicStatus:    verificationReasonDeterministicPassed,
		ProviderCritiqueStatus: verificationReasonProviderCritiqueNeed,
	}
	if am == nil || build == nil || task == nil || candidate == nil {
		return result
	}

	if !candidate.VerifyPassed && len(candidate.VerifyErrors) > 0 {
		result.Ran = true
		result.Checks = append(result.Checks, "deterministic:verify_generated_code")
		result.Errors = append(result.Errors, candidate.VerifyErrors...)
		result.DeterministicStatus = verificationReasonDeterministicFailed
		result.ProviderCritiqueStatus = verificationReasonProviderCritiqueSkip
		return result
	}

	// ── Cheap in-process checks (no subprocess) ───────────────────────────
	// Run these before the expensive surface workspace checks so obvious
	// failures are caught immediately without materializing a temp directory.
	cheap := runCheapInProcessChecks(task, candidate)
	result.Ran = result.Ran || cheap.Ran
	result.Checks = append(result.Checks, cheap.Checks...)
	result.Warnings = append(result.Warnings, cheap.Warnings...)
	result.Errors = append(result.Errors, cheap.Errors...)
	if len(cheap.Errors) > 0 {
		result.DeterministicStatus = verificationReasonDeterministicFailed
		result.ProviderCritiqueStatus = verificationReasonProviderCritiqueSkip
		return result
	}

	surface := am.runSurfaceDeterministicChecks(build, task, candidate)
	result.Ran = result.Ran || surface.Ran
	result.Checks = append(result.Checks, surface.Checks...)
	result.Warnings = append(result.Warnings, surface.Warnings...)
	result.Errors = append(result.Errors, surface.Errors...)
	if len(surface.Errors) > 0 {
		result.DeterministicStatus = verificationReasonDeterministicFailed
		result.ProviderCritiqueStatus = verificationReasonProviderCritiqueSkip
		return result
	}
	return result
}

// runCheapInProcessChecks performs fast, zero-subprocess validation on generated
// files before any workspace materialization. It catches four classes of defects:
//
//  1. Empty code files — an AI that returned empty content for a code file
//  2. Stub/placeholder output — AI left TODO markers or unimplemented sentinels
//  3. Truncated output — severely unbalanced braces indicating a cut-off response
//  4. Required-file presence — frontend tasks must produce at least one .tsx/.ts file;
//     backend tasks must produce at least one .go or server .ts file
func runCheapInProcessChecks(task *Task, candidate *taskGenerationCandidate) surfaceDeterministicResult {
	result := surfaceDeterministicResult{}
	if candidate == nil || candidate.Output == nil || len(candidate.Output.Files) == 0 {
		return result
	}
	files := candidate.Output.Files

	result.Ran = true

	// ── 1. Empty code files ───────────────────────────────────────────────
	result.Checks = append(result.Checks, "cheap:empty_files")
	codeExts := map[string]bool{
		".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".py": true, ".rb": true, ".java": true, ".rs": true, ".swift": true,
	}
	for _, f := range files {
		if !codeExts[fileExt(f.Path)] {
			continue
		}
		if strings.TrimSpace(f.Content) == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("empty code file: %s", f.Path))
		}
	}

	// ── 2. Stub / placeholder detection ──────────────────────────────────
	result.Checks = append(result.Checks, "cheap:stub_detection")
	stubSignals := []string{
		"// TODO: implement",
		"// TODO: add implementation",
		"throw new Error('not implemented')",
		"raise NotImplementedError",
		"pass  # TODO",
		"__YOUR_CODE_HERE__",
		"YOUR_API_KEY_HERE",
		"your-api-key",
		"PLACEHOLDER_VALUE",
		"/* implement me */",
		"// implement me",
		"unimplemented!()",
		"todo!()",
	}
	for _, f := range files {
		if !codeExts[fileExt(f.Path)] {
			continue
		}
		lower := strings.ToLower(f.Content)
		for _, sig := range stubSignals {
			if strings.Contains(lower, strings.ToLower(sig)) {
				result.Errors = append(result.Errors, fmt.Sprintf("stub placeholder found in %s: %q", f.Path, sig))
				break
			}
		}
	}

	// ── 3. Truncation detection (unbalanced braces) ───────────────────────
	result.Checks = append(result.Checks, "cheap:truncation_check")
	braceExts := map[string]bool{".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true}
	for _, f := range files {
		if !braceExts[fileExt(f.Path)] {
			continue
		}
		open := strings.Count(f.Content, "{")
		close := strings.Count(f.Content, "}")
		// Allow up to 3 unmatched opens (template literals, object spreads, etc.)
		// but flag large imbalances as likely truncation.
		if open-close > 5 {
			result.Errors = append(result.Errors,
				fmt.Sprintf("likely truncated output in %s: %d unclosed braces (open=%d close=%d)", f.Path, open-close, open, close))
		}
	}

	// ── 4. Required-file presence by task shape ───────────────────────────
	if task != nil {
		triage := triageTaskForWaterfall(task)
		result.Checks = append(result.Checks, "cheap:required_files")
		switch triage.TaskShape {
		case TaskShapeFrontendPatch:
			if !anyFileMatchesExt(files, ".tsx", ".ts", ".jsx", ".js", ".vue", ".svelte") {
				result.Warnings = append(result.Warnings, "frontend patch produced no recognizable UI files (.tsx/.ts/.jsx/.js/.vue/.svelte)")
			}
		case TaskShapeBackendPatch:
			if !anyFileMatchesExt(files, ".go", ".py", ".rb", ".java", ".rs") &&
				!anyFileContainsPath(files, "server", "api", "routes", "handler", "controller") {
				result.Warnings = append(result.Warnings, "backend patch produced no recognizable server-side files")
			}
		case TaskShapeSchema:
			if !anyFileMatchesExt(files, ".sql", ".prisma", ".graphql") &&
				!anyFileContainsPath(files, "migration", "schema", "migrate") {
				result.Warnings = append(result.Warnings, "schema task produced no recognizable schema files (.sql/.prisma/.graphql)")
			}
		}
	}

	return result
}

// fileExt returns the lowercase extension of a slash-separated path.
func fileExt(path string) string {
	base := path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		base = path[idx+1:]
	}
	if idx := strings.LastIndex(base, "."); idx >= 0 {
		return strings.ToLower(base[idx:])
	}
	return ""
}

// anyFileMatchesExt returns true if any file in the list has one of the given extensions.
func anyFileMatchesExt(files []GeneratedFile, exts ...string) bool {
	extSet := make(map[string]bool, len(exts))
	for _, e := range exts {
		extSet[e] = true
	}
	for _, f := range files {
		if extSet[fileExt(f.Path)] {
			return true
		}
	}
	return false
}

// anyFileContainsPath returns true if any file path contains one of the given substrings.
func anyFileContainsPath(files []GeneratedFile, substrs ...string) bool {
	for _, f := range files {
		lower := strings.ToLower(f.Path)
		for _, s := range substrs {
			if strings.Contains(lower, s) {
				return true
			}
		}
	}
	return false
}
