// Package autonomous - Self-Healing Debug Agent
// Diagnoses build/runtime errors, generates fixes, and re-verifies
package autonomous

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DebugAgent provides autonomous error diagnosis and self-healing
type DebugAgent struct {
	ai       AIProvider
	workDir  string
	verifier *BuildVerifier
}

// NewDebugAgent creates a new debug agent
func NewDebugAgent(ai AIProvider, workDir string) *DebugAgent {
	return &DebugAgent{
		ai:       ai,
		workDir:  workDir,
		verifier: NewBuildVerifier(ai, workDir),
	}
}

// DiagnosisResult contains the outcome of an error diagnosis
type DiagnosisResult struct {
	ErrorType     string           `json:"error_type"`    // syntax, type, runtime, dependency, config, security
	RootCause     string           `json:"root_cause"`
	AffectedFiles []string         `json:"affected_files"`
	Fixes         []DiagnosisFix   `json:"fixes"`
	Confidence    float64          `json:"confidence"` // 0.0-1.0
	CauseChain    []CauseLink      `json:"cause_chain,omitempty"`
	Language      string           `json:"language,omitempty"`
	StackTrace    *ParsedStackTrace `json:"stack_trace,omitempty"`
}

// CauseLink represents a link in the causal chain
type CauseLink struct {
	Level       int    `json:"level"` // 0 = immediate, higher = deeper
	Description string `json:"description"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
}

// ParsedStackTrace contains parsed stack trace information
type ParsedStackTrace struct {
	Language string       `json:"language"`
	Frames   []StackFrame `json:"frames"`
	Summary  string       `json:"summary"`
}

// StackFrame represents a single frame in a stack trace
type StackFrame struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Context  string `json:"context,omitempty"`
}

// DiagnosisFix represents a specific code fix
type DiagnosisFix struct {
	File        string `json:"file"`
	Line        int    `json:"line,omitempty"`
	Description string `json:"description"`
	Search      string `json:"search"`
	Replace     string `json:"replace"`
}

// HealingResult contains the outcome of a self-healing cycle
type HealingResult struct {
	Success     bool              `json:"success"`
	Iterations  int               `json:"iterations"`
	MaxIter     int               `json:"max_iterations"`
	Diagnoses   []DiagnosisResult `json:"diagnoses"`
	FixesApplied int              `json:"fixes_applied"`
	FinalScore  int               `json:"final_score"`
	Duration    int64             `json:"duration_ms"`
	Summary     string            `json:"summary"`
}

// Heal runs the self-healing loop: error → diagnose → fix → verify → repeat
func (d *DebugAgent) Heal(ctx context.Context, errorOutput string, maxIterations int) (*HealingResult, error) {
	startTime := time.Now()

	if maxIterations <= 0 {
		maxIterations = 5
	}

	result := &HealingResult{
		MaxIter: maxIterations,
	}

	currentError := errorOutput

	for i := 0; i < maxIterations; i++ {
		result.Iterations = i + 1

		// Step 1: Diagnose the error
		diagnosis, err := d.Diagnose(ctx, currentError)
		if err != nil {
			log.Printf("Debug agent: diagnosis failed on iteration %d: %v", i+1, err)
			break
		}
		result.Diagnoses = append(result.Diagnoses, *diagnosis)

		if len(diagnosis.Fixes) == 0 {
			log.Printf("Debug agent: no fixes generated on iteration %d", i+1)
			break
		}

		// Step 2: Apply fixes
		appliedCount := 0
		for _, fix := range diagnosis.Fixes {
			pf := parsedFix{
				file:    fix.File,
				search:  fix.Search,
				replace: fix.Replace,
			}
			if d.verifier.applyFix(pf) {
				appliedCount++
			}
		}
		result.FixesApplied += appliedCount

		if appliedCount == 0 {
			log.Printf("Debug agent: no fixes could be applied on iteration %d", i+1)
			break
		}

		// Step 3: Re-verify
		verifyResult, err := d.verifier.Verify(ctx, 0) // No retries in verify — the debug agent IS the retry loop
		if err != nil {
			break
		}

		result.FinalScore = verifyResult.Score

		if verifyResult.Passed {
			result.Success = true
			break
		}

		// Collect new errors for next iteration
		currentError = d.collectErrors(verifyResult)
		if currentError == "" {
			break
		}
	}

	result.Duration = time.Since(startTime).Milliseconds()
	result.Summary = d.buildHealingSummary(result)

	return result, nil
}

// Diagnose analyzes an error and produces a diagnosis with fixes
func (d *DebugAgent) Diagnose(ctx context.Context, errorOutput string) (*DiagnosisResult, error) {
	// First, try pattern-based diagnosis for common errors (fast path)
	patternDiagnosis := d.patternBasedDiagnosis(errorOutput)
	if patternDiagnosis != nil && patternDiagnosis.Confidence >= 0.8 {
		log.Printf("Debug agent: Pattern-based diagnosis successful (confidence: %.2f)", patternDiagnosis.Confidence)
		return patternDiagnosis, nil
	}

	// Parse stack trace if present
	stackTrace := d.parseStackTrace(errorOutput)

	// Build enhanced prompt with parsed information
	prompt := d.buildDiagnosisPrompt(errorOutput, stackTrace, patternDiagnosis)

	response, err := d.ai.Generate(ctx, prompt, AIOptions{MaxTokens: 3000})
	if err != nil {
		// Fall back to pattern diagnosis if AI fails
		if patternDiagnosis != nil {
			return patternDiagnosis, nil
		}
		return nil, fmt.Errorf("AI diagnosis failed: %w", err)
	}

	// Parse the response
	clean := cleanJSONResponse(response)

	var diagnosis DiagnosisResult
	if err := json.Unmarshal([]byte(clean), &diagnosis); err != nil {
		// Fall back to pattern diagnosis
		if patternDiagnosis != nil {
			return patternDiagnosis, nil
		}
		return &DiagnosisResult{
			ErrorType: "unknown",
			RootCause: "Could not parse AI diagnosis",
		}, nil
	}

	// Merge pattern-based findings with AI findings
	if patternDiagnosis != nil {
		diagnosis = d.mergeDiagnoses(patternDiagnosis, &diagnosis)
	}

	// Attach parsed stack trace
	diagnosis.StackTrace = stackTrace

	return &diagnosis, nil
}

// patternBasedDiagnosis uses regex patterns for common errors (fast, no AI needed)
func (d *DebugAgent) patternBasedDiagnosis(errorOutput string) *DiagnosisResult {
	patterns := []struct {
		regex      string
		errorType  string
		language   string
		rootCause  func(matches []string) string
		fixBuilder func(matches []string) []DiagnosisFix
		confidence float64
	}{
		// Go undefined errors
		{
			regex:     `(\S+\.go):(\d+):(\d+): undefined: (\w+)`,
			errorType: "type",
			language:  "go",
			rootCause: func(m []string) string {
				return fmt.Sprintf("Undefined identifier '%s' in %s", m[4], m[1])
			},
			fixBuilder: func(m []string) []DiagnosisFix {
				return []DiagnosisFix{{
					File:        m[1],
					Line:        parseInt(m[2]),
					Description: fmt.Sprintf("Add import or definition for '%s'", m[4]),
				}}
			},
			confidence: 0.9,
		},
		// Go import cycle
		{
			regex:     `import cycle not allowed`,
			errorType: "dependency",
			language:  "go",
			rootCause: func(m []string) string { return "Circular import dependency detected" },
			confidence: 0.95,
		},
		// TypeScript cannot find module
		{
			regex:     `Cannot find module '([^']+)'`,
			errorType: "dependency",
			language:  "typescript",
			rootCause: func(m []string) string {
				return fmt.Sprintf("Missing module '%s'", m[1])
			},
			fixBuilder: func(m []string) []DiagnosisFix {
				return []DiagnosisFix{{
					Description: fmt.Sprintf("Install missing package: npm install %s", m[1]),
				}}
			},
			confidence: 0.9,
		},
		// TypeScript type error
		{
			regex:     `(.+\.tsx?)\((\d+),(\d+)\): error TS(\d+): (.+)`,
			errorType: "type",
			language:  "typescript",
			rootCause: func(m []string) string {
				return fmt.Sprintf("TypeScript error TS%s: %s", m[4], m[5])
			},
			fixBuilder: func(m []string) []DiagnosisFix {
				return []DiagnosisFix{{
					File:        m[1],
					Line:        parseInt(m[2]),
					Description: fmt.Sprintf("Fix type error: %s", m[5]),
				}}
			},
			confidence: 0.85,
		},
		// Python ModuleNotFoundError
		{
			regex:     `ModuleNotFoundError: No module named '([^']+)'`,
			errorType: "dependency",
			language:  "python",
			rootCause: func(m []string) string {
				return fmt.Sprintf("Missing Python module '%s'", m[1])
			},
			fixBuilder: func(m []string) []DiagnosisFix {
				return []DiagnosisFix{{
					Description: fmt.Sprintf("Install missing package: pip install %s", m[1]),
				}}
			},
			confidence: 0.9,
		},
		// Python SyntaxError
		{
			regex:     `File "([^"]+)", line (\d+)\s+SyntaxError: (.+)`,
			errorType: "syntax",
			language:  "python",
			rootCause: func(m []string) string {
				return fmt.Sprintf("Python syntax error: %s", m[3])
			},
			fixBuilder: func(m []string) []DiagnosisFix {
				return []DiagnosisFix{{
					File:        m[1],
					Line:        parseInt(m[2]),
					Description: fmt.Sprintf("Fix syntax error: %s", m[3]),
				}}
			},
			confidence: 0.9,
		},
		// Rust error
		{
			regex:     `error\[E(\d+)\]: (.+)\s+--> (.+):(\d+):(\d+)`,
			errorType: "syntax",
			language:  "rust",
			rootCause: func(m []string) string {
				return fmt.Sprintf("Rust error E%s: %s", m[1], m[2])
			},
			fixBuilder: func(m []string) []DiagnosisFix {
				return []DiagnosisFix{{
					File:        m[3],
					Line:        parseInt(m[4]),
					Description: fmt.Sprintf("Fix: %s", m[2]),
				}}
			},
			confidence: 0.85,
		},
		// NPM peer dependency
		{
			regex:     `npm WARN.*peer dep missing: ([^,]+)`,
			errorType: "dependency",
			language:  "node",
			rootCause: func(m []string) string {
				return fmt.Sprintf("Missing peer dependency: %s", m[1])
			},
			confidence: 0.7,
		},
		// React hook rules violation
		{
			regex:     `React Hook "(\w+)" is called (conditionally|in a loop)`,
			errorType: "runtime",
			language:  "react",
			rootCause: func(m []string) string {
				return fmt.Sprintf("React Hooks rule violation: %s called %s", m[1], m[2])
			},
			confidence: 0.9,
		},
		// Null pointer / undefined
		{
			regex:     `(?i)(cannot read propert|undefined is not|null is not|TypeError:.*undefined|TypeError:.*null)`,
			errorType: "runtime",
			rootCause: func(m []string) string { return "Null or undefined reference error" },
			confidence: 0.75,
		},
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p.regex)
		matches := re.FindStringSubmatch(errorOutput)
		if matches != nil {
			diagnosis := &DiagnosisResult{
				ErrorType:  p.errorType,
				Language:   p.language,
				RootCause:  p.rootCause(matches),
				Confidence: p.confidence,
			}
			if p.fixBuilder != nil {
				diagnosis.Fixes = p.fixBuilder(matches)
			}

			// Extract affected files from error output
			diagnosis.AffectedFiles = d.extractAffectedFiles(errorOutput)

			return diagnosis
		}
	}

	return nil
}

// parseStackTrace extracts and parses stack trace from error output
func (d *DebugAgent) parseStackTrace(errorOutput string) *ParsedStackTrace {
	// Try different stack trace formats
	parsers := []func(string) *ParsedStackTrace{
		d.parseJavaScriptStackTrace,
		d.parseGoStackTrace,
		d.parsePythonStackTrace,
		d.parseRustStackTrace,
	}

	for _, parser := range parsers {
		if trace := parser(errorOutput); trace != nil && len(trace.Frames) > 0 {
			return trace
		}
	}

	return nil
}

func (d *DebugAgent) parseJavaScriptStackTrace(output string) *ParsedStackTrace {
	// JS format: at functionName (file.js:10:5)
	re := regexp.MustCompile(`\s+at\s+(\S+)\s+\(([^:]+):(\d+):(\d+)\)`)
	matches := re.FindAllStringSubmatch(output, -1)

	if len(matches) == 0 {
		return nil
	}

	frames := make([]StackFrame, 0)
	for _, m := range matches {
		frames = append(frames, StackFrame{
			Function: m[1],
			File:     m[2],
			Line:     parseInt(m[3]),
		})
	}

	return &ParsedStackTrace{
		Language: "javascript",
		Frames:   frames,
		Summary:  fmt.Sprintf("JavaScript error in %s at %s:%d", frames[0].Function, frames[0].File, frames[0].Line),
	}
}

func (d *DebugAgent) parseGoStackTrace(output string) *ParsedStackTrace {
	// Go format: goroutine N [status]:
	//            package.function()
	//                /path/file.go:10 +0xNN
	funcRe := regexp.MustCompile(`^([a-zA-Z0-9_./]+\.[a-zA-Z0-9_]+)\(\)$`)
	fileRe := regexp.MustCompile(`^\s+(.+\.go):(\d+)`)

	lines := strings.Split(output, "\n")
	frames := make([]StackFrame, 0)

	var currentFunc string
	for _, line := range lines {
		if m := funcRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			currentFunc = m[1]
		} else if m := fileRe.FindStringSubmatch(line); m != nil && currentFunc != "" {
			frames = append(frames, StackFrame{
				Function: currentFunc,
				File:     m[1],
				Line:     parseInt(m[2]),
			})
			currentFunc = ""
		}
	}

	if len(frames) == 0 {
		return nil
	}

	return &ParsedStackTrace{
		Language: "go",
		Frames:   frames,
		Summary:  fmt.Sprintf("Go panic in %s at %s:%d", frames[0].Function, frames[0].File, frames[0].Line),
	}
}

func (d *DebugAgent) parsePythonStackTrace(output string) *ParsedStackTrace {
	// Python format:   File "filename.py", line 10, in function_name
	re := regexp.MustCompile(`File "([^"]+)", line (\d+), in (\S+)`)
	matches := re.FindAllStringSubmatch(output, -1)

	if len(matches) == 0 {
		return nil
	}

	frames := make([]StackFrame, 0)
	for _, m := range matches {
		frames = append(frames, StackFrame{
			Function: m[3],
			File:     m[1],
			Line:     parseInt(m[2]),
		})
	}

	// Python traces are bottom-up, reverse to get top-down
	for i, j := 0, len(frames)-1; i < j; i, j = i+1, j-1 {
		frames[i], frames[j] = frames[j], frames[i]
	}

	return &ParsedStackTrace{
		Language: "python",
		Frames:   frames,
		Summary:  fmt.Sprintf("Python error in %s at %s:%d", frames[0].Function, frames[0].File, frames[0].Line),
	}
}

func (d *DebugAgent) parseRustStackTrace(output string) *ParsedStackTrace {
	// Rust backtrace format:   N: function_name
	//                             at /path/file.rs:10
	funcRe := regexp.MustCompile(`^\s*\d+:\s+(.+)$`)
	fileRe := regexp.MustCompile(`^\s+at\s+(.+):(\d+)$`)

	lines := strings.Split(output, "\n")
	frames := make([]StackFrame, 0)

	var currentFunc string
	for _, line := range lines {
		if m := funcRe.FindStringSubmatch(line); m != nil {
			currentFunc = m[1]
		} else if m := fileRe.FindStringSubmatch(line); m != nil && currentFunc != "" {
			frames = append(frames, StackFrame{
				Function: currentFunc,
				File:     m[1],
				Line:     parseInt(m[2]),
			})
			currentFunc = ""
		}
	}

	if len(frames) == 0 {
		return nil
	}

	return &ParsedStackTrace{
		Language: "rust",
		Frames:   frames,
		Summary:  fmt.Sprintf("Rust panic in %s at %s:%d", frames[0].Function, frames[0].File, frames[0].Line),
	}
}

// buildDiagnosisPrompt creates an enhanced prompt with parsed information
func (d *DebugAgent) buildDiagnosisPrompt(errorOutput string, stackTrace *ParsedStackTrace, patternDiagnosis *DiagnosisResult) string {
	var sb strings.Builder

	sb.WriteString(`Analyze this build/runtime error and provide a diagnosis with fixes.

Error output:
`)
	sb.WriteString(errorOutput)

	if stackTrace != nil {
		sb.WriteString(fmt.Sprintf(`

Parsed Stack Trace (%s):
Summary: %s
Top frames:
`, stackTrace.Language, stackTrace.Summary))
		for i, frame := range stackTrace.Frames {
			if i >= 5 {
				break
			}
			sb.WriteString(fmt.Sprintf("  %d. %s in %s:%d\n", i+1, frame.Function, frame.File, frame.Line))
		}
	}

	if patternDiagnosis != nil {
		sb.WriteString(fmt.Sprintf(`

Pattern Analysis suggests:
- Error Type: %s
- Root Cause: %s
- Confidence: %.0f%%
`, patternDiagnosis.ErrorType, patternDiagnosis.RootCause, patternDiagnosis.Confidence*100))
	}

	sb.WriteString(`

Respond with valid JSON only:
{
  "error_type": "syntax|type|runtime|dependency|config|security",
  "root_cause": "<one-sentence explanation>",
  "affected_files": ["file1.ts", "file2.ts"],
  "cause_chain": [
    {"level": 0, "description": "immediate cause", "file": "file.ts", "line": 10},
    {"level": 1, "description": "underlying cause"}
  ],
  "fixes": [
    {
      "file": "<filepath>",
      "line": <line number or 0>,
      "description": "<what this fix does>",
      "search": "<exact text to find in the file>",
      "replace": "<replacement text>"
    }
  ],
  "confidence": 0.0-1.0
}

Be precise with the search/replace patterns. The search text must match exactly.
Provide a cause chain showing the sequence of issues leading to the error.`)

	return sb.String()
}

// mergeDiagnoses combines pattern-based and AI diagnoses
func (d *DebugAgent) mergeDiagnoses(pattern, ai *DiagnosisResult) DiagnosisResult {
	result := *ai

	// Use pattern's higher-confidence findings
	if pattern.Confidence > ai.Confidence {
		result.ErrorType = pattern.ErrorType
		result.RootCause = pattern.RootCause
		result.Language = pattern.Language
	}

	// Merge affected files
	fileSet := make(map[string]bool)
	for _, f := range pattern.AffectedFiles {
		fileSet[f] = true
	}
	for _, f := range ai.AffectedFiles {
		fileSet[f] = true
	}
	result.AffectedFiles = make([]string, 0, len(fileSet))
	for f := range fileSet {
		result.AffectedFiles = append(result.AffectedFiles, f)
	}

	// Combine fixes, deduplicating
	fixSet := make(map[string]bool)
	combinedFixes := make([]DiagnosisFix, 0)
	for _, fix := range pattern.Fixes {
		key := fmt.Sprintf("%s:%d", fix.File, fix.Line)
		if !fixSet[key] {
			fixSet[key] = true
			combinedFixes = append(combinedFixes, fix)
		}
	}
	for _, fix := range ai.Fixes {
		key := fmt.Sprintf("%s:%d", fix.File, fix.Line)
		if !fixSet[key] {
			fixSet[key] = true
			combinedFixes = append(combinedFixes, fix)
		}
	}
	result.Fixes = combinedFixes

	return result
}

// extractAffectedFiles extracts file paths from error output
func (d *DebugAgent) extractAffectedFiles(errorOutput string) []string {
	// Match common file path patterns
	patterns := []string{
		`([a-zA-Z0-9_./\\-]+\.(?:go|ts|tsx|js|jsx|py|rs|java|rb|php|c|cpp|h))`,
	}

	fileSet := make(map[string]bool)
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(errorOutput, -1)
		for _, m := range matches {
			// Skip common false positives
			file := m[1]
			if strings.Contains(file, "node_modules") || strings.Contains(file, "vendor") {
				continue
			}
			fileSet[file] = true
		}
	}

	files := make([]string, 0, len(fileSet))
	for f := range fileSet {
		files = append(files, f)
	}
	return files
}

func parseInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// DiagnoseFromSteps analyzes failures from verification steps
func (d *DebugAgent) DiagnoseFromSteps(ctx context.Context, steps []VerifyStep) (*DiagnosisResult, error) {
	errorOutput := d.collectErrorsFromSteps(steps)
	if errorOutput == "" {
		return &DiagnosisResult{
			ErrorType: "none",
			RootCause: "No errors found in verification steps",
		}, nil
	}
	return d.Diagnose(ctx, errorOutput)
}

// collectErrors extracts error messages from a verification result
func (d *DebugAgent) collectErrors(result *VerificationResult) string {
	var errors []string
	for _, step := range result.Steps {
		if step.Status == "failed" && step.Output != "" {
			errors = append(errors, fmt.Sprintf("[%s] %s", step.Name, step.Output))
		}
	}
	return strings.Join(errors, "\n\n")
}

func (d *DebugAgent) collectErrorsFromSteps(steps []VerifyStep) string {
	var errors []string
	for _, step := range steps {
		if step.Status == "failed" && step.Output != "" {
			errors = append(errors, fmt.Sprintf("[%s] %s", step.Name, step.Output))
		}
	}
	return strings.Join(errors, "\n\n")
}

func (d *DebugAgent) buildHealingSummary(result *HealingResult) string {
	if result.Success {
		return fmt.Sprintf("Self-healing succeeded after %d iteration(s). Applied %d fix(es). Final score: %d/100.",
			result.Iterations, result.FixesApplied, result.FinalScore)
	}

	reasons := []string{}
	for _, d := range result.Diagnoses {
		if d.RootCause != "" {
			reasons = append(reasons, d.RootCause)
		}
	}

	summary := fmt.Sprintf("Self-healing failed after %d iteration(s). Applied %d fix(es). Final score: %d/100.",
		result.Iterations, result.FixesApplied, result.FinalScore)
	if len(reasons) > 0 {
		summary += " Root causes: " + strings.Join(reasons, "; ")
	}
	return summary
}

// cleanJSONResponse strips markdown fences and whitespace from AI JSON responses
func cleanJSONResponse(content string) string {
	clean := strings.TrimSpace(content)
	if strings.HasPrefix(clean, "```json") {
		clean = strings.TrimPrefix(clean, "```json")
		clean = strings.TrimSuffix(clean, "```")
	}
	if strings.HasPrefix(clean, "```") {
		clean = strings.TrimPrefix(clean, "```")
		clean = strings.TrimSuffix(clean, "```")
	}
	return strings.TrimSpace(clean)
}
