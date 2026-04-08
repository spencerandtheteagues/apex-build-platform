// compile_validator.go — Synchronous build-compile-repair loop for the agent manager.
//
// After code generation and before structural validation, this loop:
//   1. Materialises all generated files to a temporary workspace.
//   2. Runs `npm install` once (node_modules is reused across repair attempts).
//   3. Runs `npx tsc --noEmit` to surface TypeScript type errors.
//   4. Runs `npm run build` (vite) to surface bundler errors.
//   5. For each error batch, calls the AI inline (synchronously, no task queue)
//      with structured per-file error context and a 8-line source window.
//   6. Applies the AI's file patches back to build.Tasks and re-materialises.
//   7. Loops up to maxCompileAttempts times.
//
// Design principles:
//   - Never fails a build — if npm is unavailable or installs fail for host/network
//     reasons, the loop logs and returns silently. Structural validation will still
//     catch missing-dependency errors through its own repair ladder.
//   - node_modules is never re-installed between repair attempts (huge perf win).
//   - Errors are parsed into structured ParsedBuildError objects (file+line+col+code)
//     so the repair AI gets precise context rather than a wall of text.
//   - The AI call is synchronous and cheap (PowerFast, 4096 tokens max).
//   - Sets build.CompileValidationPassed = true on success so the downstream
//     shouldRunPreviewReadinessVerification() can skip the redundant build run.

package agents

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ─── Types ────────────────────────────────────────────────────────────────────

// ParsedBuildError is a structured compile error extracted from tsc or vite output.
type ParsedBuildError struct {
	File    string // relative path, e.g. "src/App.tsx"
	Line    int    // 1-based line number (0 if unknown)
	Column  int    // 1-based column (0 if unknown)
	Code    string // TypeScript error code, e.g. "TS2345" (empty for vite errors)
	Message string // full human-readable error message
	Source  string // "tsc" | "vite" | "install"
}

// compileLoopResult summarises the outcome of runCompileValidationLoop.
type compileLoopResult struct {
	Passed      bool
	SkipReason  string             // non-empty when the loop was skipped entirely
	Attempts    int                // repair attempts made
	FinalErrors []ParsedBuildError // errors still present after all attempts
}

// ─── Constants ────────────────────────────────────────────────────────────────

const (
	cvTscTimeout       = 40 * time.Second
	cvBuildTimeout     = 90 * time.Second
	cvInstallTimeout   = 2 * time.Minute
	cvRepairTokens     = 4096
	cvMaxErrorsPerFile = 3 // cap errors per file in the repair prompt
	cvMaxFilesInPrompt = 5 // cap distinct files in the repair prompt
	cvContextLines     = 8 // source lines to show around each error location
)

const compileRepairSystemPrompt = `You are a TypeScript/React build repair expert.
You will receive structured build errors with file paths, line numbers, and source context.
Output ONLY file patches — no prose, no explanations, no markdown outside code blocks.
Every patch must be the COMPLETE final file content after your fix.
`

// maxCompileAttempts returns how many compile-repair cycles to allow based on power mode.
func maxCompileAttempts(mode PowerMode) int {
	switch mode {
	case PowerMax:
		return 3
	case PowerBalanced:
		return 2
	default: // PowerFast and unknown
		return 1 // one shot — validate but don't loop
	}
}

// ─── Main Entry Point ─────────────────────────────────────────────────────────

// runCompileValidationLoop runs the compile-repair loop synchronously.
// It updates *allFiles in place when repairs succeed and sets
// build.CompileValidationPassed when the build toolchain validates cleanly.
// It never returns an error — failures are logged and the loop exits gracefully
// so the downstream structural validation can surface them via its own ladder.
func (am *AgentManager) runCompileValidationLoop(build *Build, allFiles *[]GeneratedFile, now time.Time) compileLoopResult {
	result := compileLoopResult{}
	if build == nil || allFiles == nil {
		result.SkipReason = "missing build context"
		return result
	}

	// Guard: must have frontend files and package.json to attempt compilation.
	if !cvHasFrontendBuildableFiles(*allFiles) {
		result.SkipReason = "no frontend buildable files"
		return result
	}

	// Guard: inline repair requires a configured AI router. Production managers
	// always have one; unit tests and recovery helpers sometimes do not.
	if am == nil || am.aiRouter == nil {
		result.SkipReason = "ai router not configured"
		log.Printf("[compile_validator] build %s: ai router not configured — skipping compile validation", build.ID)
		return result
	}

	// Guard: npm must be available on the host.
	if _, err := exec.LookPath("npm"); err != nil {
		result.SkipReason = "npm not available on host"
		log.Printf("[compile_validator] build %s: npm not found — skipping compile validation", build.ID)
		return result
	}

	// Broadcast that we're entering compile validation.
	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   build.ID,
		Timestamp: now,
		Data: map[string]any{
			"phase":    "compile_validation",
			"status":   string(BuildReviewing),
			"progress": 92,
			"message":  "Compile-validating generated code…",
		},
	})

	// Create persistent workspace — node_modules will be reused across repair attempts.
	tmpDir, err := os.MkdirTemp("", "apex-compile-validate-*")
	if err != nil {
		result.SkipReason = fmt.Sprintf("failed to create workspace: %v", err)
		log.Printf("[compile_validator] build %s: %s", build.ID, result.SkipReason)
		return result
	}
	defer os.RemoveAll(tmpDir)

	// Materialise all source files.
	if err := cvMaterializeFiles(*allFiles, tmpDir); err != nil {
		result.SkipReason = fmt.Sprintf("failed to materialise workspace: %v", err)
		log.Printf("[compile_validator] build %s: %s", build.ID, result.SkipReason)
		return result
	}

	// Run npm install once. Failures for environmental reasons (network, node-gyp, etc.)
	// cause a graceful skip rather than a build failure.
	ctx := am.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	installOut, installErr := cvRunCommand(ctx, tmpDir, cvInstallTimeout,
		"npm", "install", "--legacy-peer-deps", "--prefer-offline", "--no-audit", "--no-fund")
	if installErr != nil {
		skip, summary := classifyNodeInstallFailure(installOut, installErr)
		if skip {
			result.SkipReason = fmt.Sprintf("npm install skipped (env/host issue): %s", summary)
			log.Printf("[compile_validator] build %s: %s", build.ID, result.SkipReason)
			return result
		}
		// Non-skippable install failure (e.g. hallucinated package name). Surface via
		// structural validation's repair ladder rather than blocking here.
		result.SkipReason = fmt.Sprintf("npm install failed: %s", summary)
		log.Printf("[compile_validator] build %s: %s", build.ID, result.SkipReason)
		return result
	}

	maxAttempts := maxCompileAttempts(build.PowerMode)

	for attempt := 0; attempt < maxAttempts; attempt++ {
		result.Attempts = attempt + 1

		// Broadcast per-attempt progress.
		attemptMsg := "Compile-validating generated code…"
		if attempt > 0 {
			attemptMsg = fmt.Sprintf("Re-compiling after repair (attempt %d/%d)…", attempt+1, maxAttempts)
		}
		am.broadcast(build.ID, &WSMessage{
			Type:      WSBuildProgress,
			BuildID:   build.ID,
			Timestamp: time.Now(),
			Data: map[string]any{
				"phase":    "compile_validation",
				"status":   string(BuildReviewing),
				"progress": 92 + attempt,
				"message":  attemptMsg,
			},
		})

		// ── TypeScript type check ─────────────────────────────────────────────
		tscErrors := cvRunTscCheck(ctx, tmpDir)
		if len(tscErrors) > 0 {
			log.Printf("[compile_validator] build %s attempt %d: tsc found %d error(s)", build.ID, attempt+1, len(tscErrors))
			repaired := am.cvRunInlineRepair(ctx, build, tscErrors, allFiles, tmpDir)
			if repaired {
				// Rematerialise updated source files (not node_modules) and continue.
				if err := cvRematerializeSourceFiles(*allFiles, tmpDir); err != nil {
					log.Printf("[compile_validator] build %s: rematerialise failed: %v", build.ID, err)
				}
				continue
			}
			// Couldn't repair — record errors and fall through to structural validation.
			result.FinalErrors = tscErrors
			am.cvBroadcastResult(build, false, tscErrors)
			return result
		}

		// ── Vite / bundler build ──────────────────────────────────────────────
		viteErrors := cvRunViteBuild(ctx, tmpDir)
		if len(viteErrors) == 0 {
			// Build succeeded cleanly.
			result.Passed = true
			build.mu.Lock()
			build.CompileValidationPassed = true
			build.CompileValidationAttempts = result.Attempts
			build.mu.Unlock()
			log.Printf("[compile_validator] build %s: compile validation passed after %d attempt(s)", build.ID, result.Attempts)
			am.cvBroadcastResult(build, true, nil)
			return result
		}

		log.Printf("[compile_validator] build %s attempt %d: vite build found %d error(s)", build.ID, attempt+1, len(viteErrors))
		repaired := am.cvRunInlineRepair(ctx, build, viteErrors, allFiles, tmpDir)
		if repaired {
			if err := cvRematerializeSourceFiles(*allFiles, tmpDir); err != nil {
				log.Printf("[compile_validator] build %s: rematerialise failed: %v", build.ID, err)
			}
			continue
		}
		result.FinalErrors = viteErrors
		am.cvBroadcastResult(build, false, viteErrors)
		return result
	}

	// Exhausted attempts without passing.
	am.cvBroadcastResult(build, false, result.FinalErrors)
	return result
}

// ─── Compile Checkers ─────────────────────────────────────────────────────────

// cvRunTscCheck runs `npx tsc --noEmit` and returns structured errors.
// Returns nil/empty on success or if tsc is not available.
func cvRunTscCheck(ctx context.Context, workDir string) []ParsedBuildError {
	// Only run tsc if there's a tsconfig.json present.
	if _, err := os.Stat(filepath.Join(workDir, "tsconfig.json")); err != nil {
		return nil
	}

	out, err := cvRunCommand(ctx, workDir, cvTscTimeout,
		"npx", "--yes", "tsc", "--noEmit", "--pretty", "false")
	if err == nil {
		return nil // exit 0 — no errors
	}
	if out == "" {
		return nil // no output to parse
	}
	return parseTscOutput(out)
}

// cvRunViteBuild runs `npm run build` and returns structured errors.
// Returns nil/empty on success.
func cvRunViteBuild(ctx context.Context, workDir string) []ParsedBuildError {
	out, err := cvRunCommand(ctx, workDir, cvBuildTimeout, "npm", "run", "build")
	if err == nil {
		return nil
	}
	// Use existing host-environment skip classifier to avoid misidentifying
	// esbuild/rollup binary mismatches as code errors.
	skip, _ := classifyNodeBuildFailure(out, err)
	if skip {
		return nil
	}
	return parseViteOutput(out)
}

// ─── Error Parsers ────────────────────────────────────────────────────────────

// tscErrorRe matches TypeScript compiler output lines like:
//
//	src/App.tsx(15,3): error TS2345: Argument of type 'string' is not assignable to...
var tscErrorRe = regexp.MustCompile(`^(.+?)\((\d+),(\d+)\): error (TS\d+): (.+)$`)

func parseTscOutput(output string) []ParsedBuildError {
	var errors []ParsedBuildError
	seen := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		m := tscErrorRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key := m[1] + ":" + m[2] + ":" + m[3] + ":" + m[4]
		if seen[key] {
			continue
		}
		seen[key] = true
		lineNum, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		errors = append(errors, ParsedBuildError{
			File:    filepath.ToSlash(m[1]),
			Line:    lineNum,
			Column:  col,
			Code:    m[4],
			Message: strings.TrimSpace(m[5]),
			Source:  "tsc",
		})
		if len(errors) >= 30 { // cap to avoid overwhelming the repair prompt
			break
		}
	}
	return errors
}

// viteErrorFileRe matches Vite error locations like:
//
//	/tmp/apex-compile-validate-123/src/App.tsx:25:5:
var viteErrorFileRe = regexp.MustCompile(`(?:^|[\s(])([./\w-]+\.(?:tsx?|jsx?|css|json))\s*:(\d+)(?::(\d+))?`)

// viteErrorMsgRe matches standalone error messages in Vite output.
var viteErrorMsgRe = regexp.MustCompile(`(?i)(?:error|failed|cannot find|unexpected token|is not|does not).*`)

func parseViteOutput(output string) []ParsedBuildError {
	var errors []ParsedBuildError

	// First, try to extract structured file:line entries from Vite JSON-ish output.
	lines := strings.Split(output, "\n")
	var currentFile string
	var currentLine, currentCol int

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		// Check for file location references.
		if m := viteErrorFileRe.FindStringSubmatch(line); m != nil {
			path := filepath.ToSlash(m[1])
			// Strip any leading tmpDir prefix (absolute paths from vite output).
			if idx := strings.LastIndex(path, "src/"); idx != -1 {
				path = path[idx:]
			} else if idx := strings.LastIndex(path, "app/"); idx != -1 {
				path = path[idx:]
			}
			lineNum, _ := strconv.Atoi(m[2])
			col, _ := strconv.Atoi(m[3])
			currentFile = path
			currentLine = lineNum
			currentCol = col
		}

		// Capture error message lines.
		if previewBuildOutputHasActionableFailure(line) && currentFile != "" {
			errors = append(errors, ParsedBuildError{
				File:    currentFile,
				Line:    currentLine,
				Column:  currentCol,
				Message: line,
				Source:  "vite",
			})
			currentFile = ""
			currentLine = 0
			currentCol = 0
			if len(errors) >= 20 {
				break
			}
		}
	}

	// If structured parsing yielded nothing, fall back to summary extraction.
	if len(errors) == 0 {
		summary := summarizePreviewBuildFailure(output)
		if summary != "" && summary != "command failed with no output" {
			errors = append(errors, ParsedBuildError{
				Message: summary,
				Source:  "vite",
			})
		}
	}

	return errors
}

// ─── Inline AI Repair ─────────────────────────────────────────────────────────

// cvRunInlineRepair calls the AI synchronously with structured error context and
// applies any generated file patches back to build.Tasks. Returns true if at least
// one file was updated.
func (am *AgentManager) cvRunInlineRepair(
	ctx context.Context,
	build *Build,
	errors []ParsedBuildError,
	allFiles *[]GeneratedFile,
	tmpDir string,
) bool {
	if len(errors) == 0 {
		return false
	}

	// Select provider — prefer a fast model.
	if am.aiRouter == nil {
		log.Printf("[compile_validator] build %s: aiRouter not available for inline repair", build.ID)
		return false
	}
	providers := am.aiRouter.GetAvailableProvidersForUser(build.UserID)
	if len(providers) == 0 {
		providers = am.aiRouter.GetAvailableProviders()
	}
	if len(providers) == 0 || !am.aiRouter.HasConfiguredProviders() {
		log.Printf("[compile_validator] build %s: no AI provider available for inline repair", build.ID)
		return false
	}
	provider := providers[0]

	prompt := cvBuildRepairPrompt(errors, *allFiles)

	repairCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resp, err := am.aiRouter.Generate(repairCtx, provider, prompt, GenerateOptions{
		UserID:          build.UserID,
		MaxTokens:       cvRepairTokens,
		Temperature:     0.1,
		SystemPrompt:    compileRepairSystemPrompt,
		PowerMode:       PowerFast,
		UsePlatformKeys: build.ProviderMode != "byok",
	})
	if err != nil {
		log.Printf("[compile_validator] build %s: AI repair call failed: %v", build.ID, err)
		return false
	}
	if resp == nil || strings.TrimSpace(resp.Content) == "" {
		return false
	}

	// Parse the response for file patches.
	output := am.parseTaskOutput(TaskFix, resp.Content)
	if output == nil {
		return false
	}

	applied := false

	// Apply patch bundle if present.
	if output.StructuredPatchBundle != nil && len(output.StructuredPatchBundle.Operations) > 0 {
		if am.applyPatchBundleToBuild(build, output.StructuredPatchBundle) {
			applied = true
		}
	}

	// Apply any plain file outputs.
	if len(output.Files) > 0 {
		for _, f := range output.Files {
			if strings.TrimSpace(f.Path) == "" || strings.TrimSpace(f.Content) == "" {
				continue
			}
			if am.patchGeneratedFileContent(build, f.Path, f.Content) {
				applied = true
			} else if am.createGeneratedFile(build, f.Path, f.Content) {
				applied = true
			}
		}
	}

	if applied {
		*allFiles = am.collectGeneratedFiles(build)
	}
	return applied
}

// cvBuildRepairPrompt assembles the repair prompt with structured error context
// and per-file source windows around each error location.
func cvBuildRepairPrompt(errors []ParsedBuildError, allFiles []GeneratedFile) string {
	// Build a file content index for quick lookup.
	fileIndex := make(map[string]string, len(allFiles))
	for _, f := range allFiles {
		if strings.TrimSpace(f.Path) != "" {
			fileIndex[filepath.ToSlash(f.Path)] = f.Content
		}
	}

	// Group errors by file, capping per-file and total file counts.
	type fileErrors struct {
		path   string
		errors []ParsedBuildError
	}
	seenFiles := map[string]int{} // path → index in groups
	var groups []fileErrors

	for _, e := range errors {
		if len(groups) >= cvMaxFilesInPrompt {
			break
		}
		key := e.File
		if key == "" {
			key = "_unknown"
		}
		idx, ok := seenFiles[key]
		if !ok {
			groups = append(groups, fileErrors{path: key})
			idx = len(groups) - 1
			seenFiles[key] = idx
		}
		if len(groups[idx].errors) < cvMaxErrorsPerFile {
			groups[idx].errors = append(groups[idx].errors, e)
		}
	}

	var sb strings.Builder

	sb.WriteString(patchFirstTaskOutputFormatPrompt())
	sb.WriteString("\n\n## Build Errors\n\n")
	sb.WriteString(fmt.Sprintf("Source: %s\n\n", errors[0].Source))

	for gi, g := range groups {
		sb.WriteString(fmt.Sprintf("### File %d: %s\n\n", gi+1, g.path))

		content := fileIndex[g.path]
		fileLines := strings.Split(content, "\n")

		for _, e := range g.errors {
			if e.Code != "" {
				sb.WriteString(fmt.Sprintf("**Error %s** (line %d, col %d): %s\n", e.Code, e.Line, e.Column, e.Message))
			} else {
				sb.WriteString(fmt.Sprintf("**Error** (line %d): %s\n", e.Line, e.Message))
			}

			// Show source context window around the error line.
			if e.Line > 0 && len(fileLines) > 0 {
				startLine := e.Line - 1 - cvContextLines
				if startLine < 0 {
					startLine = 0
				}
				endLine := e.Line - 1 + cvContextLines
				if endLine >= len(fileLines) {
					endLine = len(fileLines) - 1
				}
				sb.WriteString("\n```typescript\n")
				for i := startLine; i <= endLine; i++ {
					prefix := "  "
					if i == e.Line-1 {
						prefix = "→ " // mark the error line
					}
					sb.WriteString(fmt.Sprintf("%s%4d | %s\n", prefix, i+1, fileLines[i]))
				}
				sb.WriteString("```\n\n")
			}
		}

		// Include the full file content for context (truncated to 200 lines).
		if content != "" {
			truncated := fileLines
			suffix := ""
			if len(truncated) > 200 {
				truncated = truncated[:200]
				suffix = "\n// ... (truncated)"
			}
			sb.WriteString(fmt.Sprintf("**Full file content** (`%s`):\n```typescript\n%s%s\n```\n\n",
				g.path, strings.Join(truncated, "\n"), suffix))
		}
	}

	sb.WriteString("\n## Instructions\n\n")
	sb.WriteString("Fix ALL listed errors. Output complete corrected file contents using the patch format above.\n")
	sb.WriteString("Do not change any files not mentioned above. Do not add comments about what you changed.\n")

	return sb.String()
}

// ─── Workspace Helpers ────────────────────────────────────────────────────────

// cvMaterializeFiles writes all generated files to tmpDir.
func cvMaterializeFiles(files []GeneratedFile, tmpDir string) error {
	for _, f := range files {
		path := strings.TrimSpace(f.Path)
		if path == "" || f.Content == "" {
			continue
		}
		path = filepath.ToSlash(strings.TrimPrefix(path, "./"))
		path = strings.TrimPrefix(path, "/")
		if strings.Contains(path, "..") {
			continue
		}
		// Skip binary-like files that shouldn't be materialised.
		if strings.HasPrefix(path, "node_modules/") {
			continue
		}
		full := filepath.Join(tmpDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", path, err)
		}
		if err := os.WriteFile(full, []byte(f.Content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

// cvRematerializeSourceFiles rewrites source files in tmpDir after an AI repair.
// node_modules is deliberately not touched — we reuse the existing install.
func cvRematerializeSourceFiles(files []GeneratedFile, tmpDir string) error {
	for _, f := range files {
		path := strings.TrimSpace(f.Path)
		if path == "" || f.Content == "" {
			continue
		}
		path = filepath.ToSlash(strings.TrimPrefix(path, "./"))
		path = strings.TrimPrefix(path, "/")
		if strings.Contains(path, "..") || strings.HasPrefix(path, "node_modules/") {
			continue
		}
		full := filepath.Join(tmpDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			continue
		}
		// Silently ignore write errors for rematerialisation — a failed rewrite
		// just means that file won't be updated before the next compile check.
		_ = os.WriteFile(full, []byte(f.Content), 0644)
	}
	return nil
}

// cvRunCommand executes a command in workDir with a timeout, returning combined stdout+stderr.
func cvRunCommand(ctx context.Context, workDir string, timeout time.Duration, name string, args ...string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, name, args...)
	configurePreviewCheckCommand(cmd)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "CI=1", "FORCE_COLOR=0")

	out, err := cmd.CombinedOutput()
	outStr := string(out)

	if cmdCtx.Err() == context.DeadlineExceeded {
		return outStr, fmt.Errorf("%s timed out after %s", name, timeout)
	}
	return outStr, err
}

// ─── Guard ────────────────────────────────────────────────────────────────────

// cvHasFrontendBuildableFiles returns true when the file set includes a package.json
// and at least one TypeScript or JavaScript source file — the minimum needed to
// run tsc/vite.
func cvHasFrontendBuildableFiles(files []GeneratedFile) bool {
	hasPkg := false
	hasSrc := false
	for _, f := range files {
		p := strings.ToLower(strings.TrimSpace(f.Path))
		if p == "package.json" || strings.HasSuffix(p, "/package.json") {
			hasPkg = true
		}
		if strings.HasSuffix(p, ".ts") || strings.HasSuffix(p, ".tsx") ||
			strings.HasSuffix(p, ".js") || strings.HasSuffix(p, ".jsx") {
			hasSrc = true
		}
		if hasPkg && hasSrc {
			return true
		}
	}
	return false
}

// ─── Progress Broadcasting ────────────────────────────────────────────────────

func (am *AgentManager) cvBroadcastResult(build *Build, passed bool, errors []ParsedBuildError) {
	msg := "Compile validation passed — generated code builds cleanly."
	if !passed {
		if len(errors) > 0 {
			msg = fmt.Sprintf("Compile validation found %d error(s); repair attempted.", len(errors))
		} else {
			msg = "Compile validation exhausted repair attempts."
		}
	}
	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"phase":                     "compile_validation",
			"status":                    string(BuildReviewing),
			"progress":                  95,
			"message":                   msg,
			"compile_validation_passed": passed,
			"compile_validation_errors": len(errors),
		},
	})
}
