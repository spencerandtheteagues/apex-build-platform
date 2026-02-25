// Package core — validator.go
//
// Build output validation for the 100% Success Guarantee Engine.
// Validates generated code, scans for placeholders, runs smoke tests,
// and determines whether a build passes or needs retry/rollback.
//
// PUBLIC CONTRACT FOR CODEX 5.3 INTEGRATION:
//
//	BuildValidator — instantiate with NewBuildValidator(cfg).
//	Validate(ctx, artifacts) → ValidationResult.
//	The guarantee engine calls this after every execution step.
package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// --- Validation Result ---

// ValidationVerdict is the outcome of a validation run.
type ValidationVerdict string

const (
	VerdictPass     ValidationVerdict = "pass"
	VerdictSoftFail ValidationVerdict = "soft_fail" // retriable
	VerdictHardFail ValidationVerdict = "hard_fail" // needs rollback
)

// ValidationResult contains the full outcome of validating a build.
type ValidationResult struct {
	Verdict       ValidationVerdict  `json:"verdict"`
	Score         float64            `json:"score"`          // 0.0–1.0
	Checks        []ValidationCheck  `json:"checks"`
	Placeholders  []PlaceholderHit   `json:"placeholders"`
	SmokeTestPass bool               `json:"smoke_test_pass"`
	ErrorSummary  string             `json:"error_summary,omitempty"`
	Duration      time.Duration      `json:"duration"`
	Timestamp     time.Time          `json:"timestamp"`
}

// ValidationCheck is a single pass/fail check.
type ValidationCheck struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "error", "warning", "info"
}

// PlaceholderHit represents a detected placeholder or TODO in generated code.
type PlaceholderHit struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Match    string `json:"match"`
	Pattern  string `json:"pattern"`
}

// --- Build Artifact ---

// BuildArtifact represents a single file produced by the agent.
type BuildArtifact struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Language string `json:"language"`
	IsNew    bool   `json:"is_new"`
}

// --- Validator Config ---

// ValidatorConfig tunes validation behavior.
type ValidatorConfig struct {
	// PlaceholderPatterns to scan for. Defaults provided if empty.
	PlaceholderPatterns []string

	// MinimumScore for a pass verdict (0.0–1.0). Default: 0.8
	MinimumPassScore float64

	// RunSmokeTest enables post-build smoke test execution.
	RunSmokeTest bool

	// SmokeTestCommand to execute (e.g., "npm test", "go test ./...").
	SmokeTestCommand string

	// SmokeTestTimeout duration.
	SmokeTestTimeout time.Duration

	// MaxPlaceholders before hard-failing.
	MaxPlaceholders int

	// StrictMode fails on any warning.
	StrictMode bool
}

// DefaultValidatorConfig returns sensible defaults.
func DefaultValidatorConfig() ValidatorConfig {
	return ValidatorConfig{
		PlaceholderPatterns: defaultPlaceholderPatterns(),
		MinimumPassScore:    0.8,
		RunSmokeTest:        true,
		SmokeTestTimeout:    30 * time.Second,
		MaxPlaceholders:     0, // zero tolerance
		StrictMode:          false,
	}
}

// --- Placeholder Patterns ---

func defaultPlaceholderPatterns() []string {
	return []string{
		`(?i)\bTODO\b`,
		`(?i)\bFIXME\b`,
		`(?i)\bHACK\b`,
		`(?i)\bXXX\b`,
		`(?i)placeholder`,
		`(?i)not\s+implemented`,
		`(?i)implement\s+this`,
		`(?i)add\s+your\s+(code|logic|implementation)\s+here`,
		`(?i)replace\s+this`,
		`(?i)your[-_]?(api[-_]?key|secret|token|password)`,
		`(?i)example\.com`,
		`(?i)lorem\s+ipsum`,
		`\.\.\.\s*$`, // trailing "..."
		`pass\s*$`,   // bare "pass" at end of line (Python no-op)
		`(?i)stub`,
		`(?i)mock\s+data`,
		`(?i)dummy`,
	}
}

// --- Build Validator ---

// BuildValidator validates agent build output.
type BuildValidator struct {
	config             ValidatorConfig
	compiledPatterns   []*regexp.Regexp
	smokeTestRunner    SmokeTestRunner
}

// SmokeTestRunner is an interface for executing smoke tests.
// Codex 5.3 provides the real implementation via SandboxManager.
type SmokeTestRunner interface {
	RunSmokeTest(ctx context.Context, command string, timeout time.Duration) (output string, exitCode int, err error)
}

// NewBuildValidator creates a validator with the given config.
func NewBuildValidator(cfg ValidatorConfig, runner SmokeTestRunner) (*BuildValidator, error) {
	compiled := make([]*regexp.Regexp, 0, len(cfg.PlaceholderPatterns))
	for _, pattern := range cfg.PlaceholderPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid placeholder pattern %q: %w", pattern, err)
		}
		compiled = append(compiled, re)
	}

	return &BuildValidator{
		config:           cfg,
		compiledPatterns: compiled,
		smokeTestRunner:  runner,
	}, nil
}

// Validate runs all validation checks against the build artifacts.
func (v *BuildValidator) Validate(ctx context.Context, artifacts []BuildArtifact) *ValidationResult {
	start := time.Now()
	result := &ValidationResult{
		Timestamp: start,
		Checks:    make([]ValidationCheck, 0, 10),
	}

	totalWeight := 0.0
	passWeight := 0.0

	// 1. File presence check
	fileCheck := v.checkFilesExist(artifacts)
	result.Checks = append(result.Checks, fileCheck)
	totalWeight += 1.0
	if fileCheck.Passed {
		passWeight += 1.0
	}

	// 2. Empty file check
	emptyCheck := v.checkNoEmptyFiles(artifacts)
	result.Checks = append(result.Checks, emptyCheck)
	totalWeight += 1.0
	if emptyCheck.Passed {
		passWeight += 1.0
	}

	// 3. Syntax sanity check (basic bracket/paren matching)
	syntaxCheck := v.checkSyntaxSanity(artifacts)
	result.Checks = append(result.Checks, syntaxCheck)
	totalWeight += 2.0
	if syntaxCheck.Passed {
		passWeight += 2.0
	}

	// 4. Placeholder scan (heavily weighted)
	placeholders := v.scanPlaceholders(artifacts)
	result.Placeholders = placeholders
	placeholderCheck := ValidationCheck{
		Name:     "placeholder_scan",
		Passed:   len(placeholders) <= v.config.MaxPlaceholders,
		Severity: "error",
	}
	if placeholderCheck.Passed {
		placeholderCheck.Message = "No prohibited placeholders found"
	} else {
		placeholderCheck.Message = fmt.Sprintf("Found %d placeholder(s) in generated code", len(placeholders))
	}
	result.Checks = append(result.Checks, placeholderCheck)
	totalWeight += 3.0
	if placeholderCheck.Passed {
		passWeight += 3.0
	}

	// 5. Import/dependency check
	importCheck := v.checkImports(artifacts)
	result.Checks = append(result.Checks, importCheck)
	totalWeight += 1.0
	if importCheck.Passed {
		passWeight += 1.0
	}

	// 6. Smoke test (if configured and runner available)
	if v.config.RunSmokeTest && v.smokeTestRunner != nil && v.config.SmokeTestCommand != "" {
		smokeCheck, passed := v.runSmokeTest(ctx)
		result.Checks = append(result.Checks, smokeCheck)
		result.SmokeTestPass = passed
		totalWeight += 3.0
		if passed {
			passWeight += 3.0
		}
	} else {
		result.SmokeTestPass = true // skip = pass
	}

	// Calculate score
	if totalWeight > 0 {
		result.Score = passWeight / totalWeight
	}

	// Determine verdict
	result.Duration = time.Since(start)
	result.Verdict = v.determineVerdict(result)

	// Build error summary
	if result.Verdict != VerdictPass {
		var failures []string
		for _, check := range result.Checks {
			if !check.Passed {
				failures = append(failures, check.Message)
			}
		}
		result.ErrorSummary = strings.Join(failures, "; ")
	}

	return result
}

// --- Individual Checks ---

func (v *BuildValidator) checkFilesExist(artifacts []BuildArtifact) ValidationCheck {
	if len(artifacts) == 0 {
		return ValidationCheck{
			Name:     "files_exist",
			Passed:   false,
			Message:  "No build artifacts produced",
			Severity: "error",
		}
	}
	return ValidationCheck{
		Name:     "files_exist",
		Passed:   true,
		Message:  fmt.Sprintf("%d artifact(s) produced", len(artifacts)),
		Severity: "info",
	}
}

func (v *BuildValidator) checkNoEmptyFiles(artifacts []BuildArtifact) ValidationCheck {
	var empty []string
	for _, a := range artifacts {
		trimmed := strings.TrimSpace(a.Content)
		if len(trimmed) == 0 {
			empty = append(empty, a.Path)
		}
	}
	if len(empty) > 0 {
		return ValidationCheck{
			Name:     "no_empty_files",
			Passed:   false,
			Message:  fmt.Sprintf("Empty files: %s", strings.Join(empty, ", ")),
			Severity: "error",
		}
	}
	return ValidationCheck{
		Name:     "no_empty_files",
		Passed:   true,
		Message:  "All files have content",
		Severity: "info",
	}
}

func (v *BuildValidator) checkSyntaxSanity(artifacts []BuildArtifact) ValidationCheck {
	var issues []string
	for _, a := range artifacts {
		if err := checkBracketBalance(a.Content, a.Path); err != nil {
			issues = append(issues, err.Error())
		}
	}
	if len(issues) > 0 {
		return ValidationCheck{
			Name:     "syntax_sanity",
			Passed:   false,
			Message:  strings.Join(issues, "; "),
			Severity: "error",
		}
	}
	return ValidationCheck{
		Name:     "syntax_sanity",
		Passed:   true,
		Message:  "Bracket/paren balance OK",
		Severity: "info",
	}
}

func checkBracketBalance(content, path string) error {
	stack := []rune{}
	pairs := map[rune]rune{')': '(', ']': '[', '}': '{'}

	for i, ch := range content {
		switch ch {
		case '(', '[', '{':
			stack = append(stack, ch)
		case ')', ']', '}':
			if len(stack) == 0 {
				return fmt.Errorf("%s: unmatched '%c' at position %d", path, ch, i)
			}
			expected := pairs[ch]
			got := stack[len(stack)-1]
			if got != expected {
				return fmt.Errorf("%s: mismatched bracket at position %d (expected '%c', got '%c')", path, i, expected, got)
			}
			stack = stack[:len(stack)-1]
		}
	}

	if len(stack) > 0 {
		return fmt.Errorf("%s: %d unclosed bracket(s)", path, len(stack))
	}
	return nil
}

func (v *BuildValidator) scanPlaceholders(artifacts []BuildArtifact) []PlaceholderHit {
	var hits []PlaceholderHit
	for _, a := range artifacts {
		lines := strings.Split(a.Content, "\n")
		for lineNum, line := range lines {
			for _, re := range v.compiledPatterns {
				locs := re.FindAllStringIndex(line, -1)
				for _, loc := range locs {
					// Skip if it's inside a comment that's clearly a docs/explanation
					// (simple heuristic: allow "TODO" in lines that are purely comments
					//  only when in non-strict mode)
					if !v.config.StrictMode && isDocComment(line) {
						continue
					}
					hits = append(hits, PlaceholderHit{
						FilePath: a.Path,
						Line:     lineNum + 1,
						Column:   loc[0] + 1,
						Match:    line[loc[0]:loc[1]],
						Pattern:  re.String(),
					})
				}
			}
		}
	}
	return hits
}

func isDocComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "//") ||
		strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(trimmed, "/*") ||
		strings.HasPrefix(trimmed, "*") ||
		strings.HasPrefix(trimmed, "\"\"\"") ||
		strings.HasPrefix(trimmed, "'''")
}

func (v *BuildValidator) checkImports(artifacts []BuildArtifact) ValidationCheck {
	// Heuristic: check that files importing modules don't have
	// obvious broken import paths (e.g., empty strings, "TODO")
	var issues []string
	importPatterns := []*regexp.Regexp{
		regexp.MustCompile(`import\s+["']([^"']+)["']`),                   // JS/TS
		regexp.MustCompile(`from\s+["']([^"']+)["']`),                     // JS/TS
		regexp.MustCompile(`import\s+\(\s*\n\s*"([^"]+)"`),               // Go
		regexp.MustCompile(`(?m)^\s*import\s+(\S+)`),                      // Python
		regexp.MustCompile(`require\s*\(\s*["']([^"']+)["']\s*\)`),       // CommonJS
	}

	for _, a := range artifacts {
		for _, re := range importPatterns {
			matches := re.FindAllStringSubmatch(a.Content, -1)
			for _, m := range matches {
				if len(m) > 1 {
					imp := m[1]
					if strings.Contains(strings.ToLower(imp), "todo") ||
						strings.Contains(strings.ToLower(imp), "placeholder") ||
						imp == "" {
						issues = append(issues, fmt.Sprintf("%s: suspicious import %q", a.Path, imp))
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		return ValidationCheck{
			Name:     "import_check",
			Passed:   false,
			Message:  strings.Join(issues, "; "),
			Severity: "warning",
		}
	}
	return ValidationCheck{
		Name:     "import_check",
		Passed:   true,
		Message:  "Import paths look valid",
		Severity: "info",
	}
}

func (v *BuildValidator) runSmokeTest(ctx context.Context) (ValidationCheck, bool) {
	testCtx, cancel := context.WithTimeout(ctx, v.config.SmokeTestTimeout)
	defer cancel()

	output, exitCode, err := v.smokeTestRunner.RunSmokeTest(testCtx, v.config.SmokeTestCommand, v.config.SmokeTestTimeout)
	if err != nil {
		return ValidationCheck{
			Name:     "smoke_test",
			Passed:   false,
			Message:  fmt.Sprintf("Smoke test error: %v", err),
			Severity: "error",
		}, false
	}

	if exitCode != 0 {
		// Truncate output for the check message
		msg := output
		if len(msg) > 500 {
			msg = msg[:500] + "..."
		}
		return ValidationCheck{
			Name:     "smoke_test",
			Passed:   false,
			Message:  fmt.Sprintf("Smoke test failed (exit %d): %s", exitCode, msg),
			Severity: "error",
		}, false
	}

	return ValidationCheck{
		Name:     "smoke_test",
		Passed:   true,
		Message:  "Smoke test passed",
		Severity: "info",
	}, true
}

// --- Verdict Logic ---

func (v *BuildValidator) determineVerdict(result *ValidationResult) ValidationVerdict {
	// Hard fail conditions
	for _, check := range result.Checks {
		if !check.Passed && check.Severity == "error" {
			if check.Name == "placeholder_scan" && len(result.Placeholders) > 5 {
				return VerdictHardFail // too many placeholders = agent is confused
			}
			if check.Name == "smoke_test" {
				return VerdictSoftFail // smoke test failure is retriable
			}
		}
	}

	// Score-based
	if result.Score >= v.config.MinimumPassScore {
		return VerdictPass
	}

	if result.Score >= 0.5 {
		return VerdictSoftFail
	}

	return VerdictHardFail
}
