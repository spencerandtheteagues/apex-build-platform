// Package autonomous - Output Validator
// Validates generated output and provides self-correction feedback
package autonomous

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Validator handles output validation and self-correction
type Validator struct {
	ai      AIProvider
	workDir string
}

// NewValidator creates a new validator
func NewValidator(ai AIProvider) *Validator {
	return &Validator{ai: ai}
}

// SetWorkDir sets the working directory for build verification
func (v *Validator) SetWorkDir(dir string) {
	v.workDir = dir
}

// verifyBuild runs actual build commands to verify the project compiles
func (v *Validator) verifyBuild(ctx context.Context) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	// Detect project type and run appropriate verification
	projectType := v.detectProjectType()

	switch projectType {
	case "node":
		issues = append(issues, v.verifyNodeProject(ctx)...)
	case "go":
		issues = append(issues, v.verifyGoProject(ctx)...)
	case "python":
		issues = append(issues, v.verifyPythonProject(ctx)...)
	case "rust":
		issues = append(issues, v.verifyRustProject(ctx)...)
	case "java":
		issues = append(issues, v.verifyJavaProject(ctx)...)
	default:
		log.Printf("Validator: Unknown project type, skipping build verification")
	}

	// Run security scan regardless of project type
	securityIssues := v.runSecurityScan(ctx)
	issues = append(issues, securityIssues...)

	return issues
}

// detectProjectType identifies the project type from files present
func (v *Validator) detectProjectType() string {
	if _, err := os.Stat(filepath.Join(v.workDir, "package.json")); err == nil {
		return "node"
	}
	if _, err := os.Stat(filepath.Join(v.workDir, "go.mod")); err == nil {
		return "go"
	}
	if _, err := os.Stat(filepath.Join(v.workDir, "requirements.txt")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(v.workDir, "pyproject.toml")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(v.workDir, "Cargo.toml")); err == nil {
		return "rust"
	}
	if _, err := os.Stat(filepath.Join(v.workDir, "pom.xml")); err == nil {
		return "java"
	}
	if _, err := os.Stat(filepath.Join(v.workDir, "build.gradle")); err == nil {
		return "java"
	}
	return "unknown"
}

// verifyNodeProject runs Node.js/TypeScript build verification
func (v *Validator) verifyNodeProject(ctx context.Context) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	// Check if node_modules exists (deps installed)
	nodeModulesPath := filepath.Join(v.workDir, "node_modules")
	if _, err := os.Stat(nodeModulesPath); os.IsNotExist(err) {
		// Try installing deps first
		installCmd := exec.CommandContext(ctx, "npm", "install", "--prefer-offline")
		installCmd.Dir = v.workDir
		if output, err := installCmd.CombinedOutput(); err != nil {
			issues = append(issues, ValidationIssue{
				Type:        IssueDependencyMissing,
				Severity:    SeverityCritical,
				File:        "package.json",
				Description: fmt.Sprintf("npm install failed: %s", truncateOutput(string(output), 500)),
			})
			return issues
		}
	}

	// Run TypeScript type check if tsconfig.json exists
	if _, err := os.Stat(filepath.Join(v.workDir, "tsconfig.json")); err == nil {
		tscCmd := exec.CommandContext(ctx, "npx", "tsc", "--noEmit")
		tscCmd.Dir = v.workDir
		if output, err := tscCmd.CombinedOutput(); err != nil {
			parsed := v.parseTypeScriptErrors(string(output))
			issues = append(issues, parsed...)
		}
	}

	// Run ESLint if config exists
	eslintConfigs := []string{".eslintrc.js", ".eslintrc.json", ".eslintrc", "eslint.config.js", "eslint.config.mjs"}
	for _, config := range eslintConfigs {
		if _, err := os.Stat(filepath.Join(v.workDir, config)); err == nil {
			lintCmd := exec.CommandContext(ctx, "npx", "eslint", ".", "--format=json", "--max-warnings=0")
			lintCmd.Dir = v.workDir
			if output, _ := lintCmd.CombinedOutput(); len(output) > 0 {
				lintIssues := v.parseESLintOutput(string(output))
				issues = append(issues, lintIssues...)
			}
			break
		}
	}

	// Run build
	buildCmd := exec.CommandContext(ctx, "npm", "run", "build")
	buildCmd.Dir = v.workDir
	if output, err := buildCmd.CombinedOutput(); err != nil {
		issues = append(issues, ValidationIssue{
			Type:        IssueSyntaxError,
			Severity:    SeverityCritical,
			File:        "package.json",
			Description: fmt.Sprintf("Build failed: %s", truncateOutput(string(output), 500)),
		})
	}

	return issues
}

// verifyGoProject runs Go build verification
func (v *Validator) verifyGoProject(ctx context.Context) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	// Download dependencies
	modCmd := exec.CommandContext(ctx, "go", "mod", "download")
	modCmd.Dir = v.workDir
	if output, err := modCmd.CombinedOutput(); err != nil {
		issues = append(issues, ValidationIssue{
			Type:        IssueDependencyMissing,
			Severity:    SeverityCritical,
			File:        "go.mod",
			Description: fmt.Sprintf("go mod download failed: %s", truncateOutput(string(output), 500)),
		})
		return issues
	}

	// Run go vet
	vetCmd := exec.CommandContext(ctx, "go", "vet", "./...")
	vetCmd.Dir = v.workDir
	if output, err := vetCmd.CombinedOutput(); err != nil {
		vetIssues := v.parseGoVetOutput(string(output))
		issues = append(issues, vetIssues...)
	}

	// Run go build
	buildCmd := exec.CommandContext(ctx, "go", "build", "./...")
	buildCmd.Dir = v.workDir
	if output, err := buildCmd.CombinedOutput(); err != nil {
		buildIssues := v.parseGoBuildErrors(string(output))
		issues = append(issues, buildIssues...)
	}

	// Run tests
	testCmd := exec.CommandContext(ctx, "go", "test", "./...", "-short")
	testCmd.Dir = v.workDir
	if output, err := testCmd.CombinedOutput(); err != nil {
		issues = append(issues, ValidationIssue{
			Type:        IssueTestFailure,
			Severity:    SeverityError,
			Description: fmt.Sprintf("Go tests failed: %s", truncateOutput(string(output), 500)),
		})
	}

	return issues
}

// verifyPythonProject runs Python build verification
func (v *Validator) verifyPythonProject(ctx context.Context) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	// Check for virtual environment
	venvPath := filepath.Join(v.workDir, "venv")
	if _, err := os.Stat(venvPath); os.IsNotExist(err) {
		// Try creating venv
		venvCmd := exec.CommandContext(ctx, "python3", "-m", "venv", "venv")
		venvCmd.Dir = v.workDir
		venvCmd.Run() // Ignore errors - might already have system Python
	}

	// Install dependencies
	if _, err := os.Stat(filepath.Join(v.workDir, "requirements.txt")); err == nil {
		pipCmd := exec.CommandContext(ctx, "pip", "install", "-r", "requirements.txt", "-q")
		pipCmd.Dir = v.workDir
		if output, err := pipCmd.CombinedOutput(); err != nil {
			issues = append(issues, ValidationIssue{
				Type:        IssueDependencyMissing,
				Severity:    SeverityError,
				File:        "requirements.txt",
				Description: fmt.Sprintf("pip install failed: %s", truncateOutput(string(output), 300)),
			})
		}
	}

	// Run flake8 if available
	flakeCmd := exec.CommandContext(ctx, "python3", "-m", "flake8", "--max-line-length=120", "--format=%(path)s:%(row)d:%(col)d: %(code)s %(text)s")
	flakeCmd.Dir = v.workDir
	if output, err := flakeCmd.CombinedOutput(); err != nil {
		flakeIssues := v.parseFlake8Output(string(output))
		issues = append(issues, flakeIssues...)
	}

	// Run mypy for type checking if available
	mypyCmd := exec.CommandContext(ctx, "python3", "-m", "mypy", ".", "--ignore-missing-imports")
	mypyCmd.Dir = v.workDir
	if output, err := mypyCmd.CombinedOutput(); err != nil {
		mypyIssues := v.parseMypyOutput(string(output))
		issues = append(issues, mypyIssues...)
	}

	// Run pytest
	pytestCmd := exec.CommandContext(ctx, "python3", "-m", "pytest", "--tb=short", "-q")
	pytestCmd.Dir = v.workDir
	if output, err := pytestCmd.CombinedOutput(); err != nil {
		issues = append(issues, ValidationIssue{
			Type:        IssueTestFailure,
			Severity:    SeverityError,
			Description: fmt.Sprintf("pytest failed: %s", truncateOutput(string(output), 500)),
		})
	}

	return issues
}

// verifyRustProject runs Rust build verification
func (v *Validator) verifyRustProject(ctx context.Context) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	// Run cargo check (faster than full build)
	checkCmd := exec.CommandContext(ctx, "cargo", "check", "--message-format=short")
	checkCmd.Dir = v.workDir
	if output, err := checkCmd.CombinedOutput(); err != nil {
		rustIssues := v.parseCargoOutput(string(output))
		issues = append(issues, rustIssues...)
	}

	// Run clippy for linting
	clippyCmd := exec.CommandContext(ctx, "cargo", "clippy", "--", "-D", "warnings")
	clippyCmd.Dir = v.workDir
	if output, err := clippyCmd.CombinedOutput(); err != nil {
		clippyIssues := v.parseCargoOutput(string(output))
		for i := range clippyIssues {
			clippyIssues[i].Type = IssueBestPractice
			clippyIssues[i].Severity = SeverityWarning
		}
		issues = append(issues, clippyIssues...)
	}

	// Run tests
	testCmd := exec.CommandContext(ctx, "cargo", "test", "--", "--test-threads=1")
	testCmd.Dir = v.workDir
	if output, err := testCmd.CombinedOutput(); err != nil {
		issues = append(issues, ValidationIssue{
			Type:        IssueTestFailure,
			Severity:    SeverityError,
			Description: fmt.Sprintf("cargo test failed: %s", truncateOutput(string(output), 500)),
		})
	}

	return issues
}

// verifyJavaProject runs Java build verification
func (v *Validator) verifyJavaProject(ctx context.Context) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	// Check for Maven or Gradle
	if _, err := os.Stat(filepath.Join(v.workDir, "pom.xml")); err == nil {
		// Maven project
		compileCmd := exec.CommandContext(ctx, "mvn", "compile", "-q")
		compileCmd.Dir = v.workDir
		if output, err := compileCmd.CombinedOutput(); err != nil {
			issues = append(issues, ValidationIssue{
				Type:        IssueSyntaxError,
				Severity:    SeverityCritical,
				File:        "pom.xml",
				Description: fmt.Sprintf("Maven compile failed: %s", truncateOutput(string(output), 500)),
			})
		}

		testCmd := exec.CommandContext(ctx, "mvn", "test", "-q")
		testCmd.Dir = v.workDir
		if output, err := testCmd.CombinedOutput(); err != nil {
			issues = append(issues, ValidationIssue{
				Type:        IssueTestFailure,
				Severity:    SeverityError,
				Description: fmt.Sprintf("Maven tests failed: %s", truncateOutput(string(output), 500)),
			})
		}
	} else if _, err := os.Stat(filepath.Join(v.workDir, "build.gradle")); err == nil {
		// Gradle project
		compileCmd := exec.CommandContext(ctx, "gradle", "compileJava", "-q")
		compileCmd.Dir = v.workDir
		if output, err := compileCmd.CombinedOutput(); err != nil {
			issues = append(issues, ValidationIssue{
				Type:        IssueSyntaxError,
				Severity:    SeverityCritical,
				File:        "build.gradle",
				Description: fmt.Sprintf("Gradle compile failed: %s", truncateOutput(string(output), 500)),
			})
		}

		testCmd := exec.CommandContext(ctx, "gradle", "test", "-q")
		testCmd.Dir = v.workDir
		if output, err := testCmd.CombinedOutput(); err != nil {
			issues = append(issues, ValidationIssue{
				Type:        IssueTestFailure,
				Severity:    SeverityError,
				Description: fmt.Sprintf("Gradle tests failed: %s", truncateOutput(string(output), 500)),
			})
		}
	}

	return issues
}

// runSecurityScan performs basic security vulnerability scanning
func (v *Validator) runSecurityScan(ctx context.Context) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	// Scan for common security issues in code
	securityPatterns := []struct {
		pattern     string
		severity    Severity
		description string
		suggestion  string
	}{
		{`(?i)password\s*=\s*["'][^"']+["']`, SeverityCritical, "Hardcoded password detected", "Use environment variables or secrets manager"},
		{`(?i)api[_-]?key\s*=\s*["'][^"']+["']`, SeverityCritical, "Hardcoded API key detected", "Use environment variables"},
		{`(?i)secret\s*=\s*["'][^"']+["']`, SeverityCritical, "Hardcoded secret detected", "Use environment variables or secrets manager"},
		{`(?i)eval\s*\(`, SeverityError, "Dangerous eval() usage detected", "Avoid eval() for security"},
		{`(?i)exec\s*\(`, SeverityWarning, "exec() usage detected - potential command injection", "Sanitize inputs or use safer alternatives"},
		{`(?i)innerHTML\s*=`, SeverityWarning, "innerHTML usage detected - potential XSS", "Use textContent or sanitize HTML"},
		{`(?i)dangerouslySetInnerHTML`, SeverityWarning, "React dangerouslySetInnerHTML detected", "Sanitize HTML before rendering"},
		{`(?i)SELECT\s+.*\s+FROM\s+.*\s+WHERE.*\+`, SeverityError, "Potential SQL injection (string concatenation in query)", "Use parameterized queries"},
	}

	err := filepath.Walk(v.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip node_modules, vendor, etc.
		if info.IsDir() {
			baseName := filepath.Base(path)
			if baseName == "node_modules" || baseName == "vendor" || baseName == ".git" || baseName == "target" || baseName == "dist" || baseName == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only scan code files
		ext := strings.ToLower(filepath.Ext(path))
		codeExts := map[string]bool{".js": true, ".ts": true, ".jsx": true, ".tsx": true, ".py": true, ".go": true, ".rs": true, ".java": true, ".rb": true, ".php": true}
		if !codeExts[ext] {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		contentStr := string(content)
		relPath, _ := filepath.Rel(v.workDir, path)

		for _, sp := range securityPatterns {
			re, err := regexp.Compile(sp.pattern)
			if err != nil {
				continue
			}

			matches := re.FindAllStringIndex(contentStr, -1)
			for _, match := range matches {
				lineNum := strings.Count(contentStr[:match[0]], "\n") + 1
				issues = append(issues, ValidationIssue{
					Type:        IssueSecurityRisk,
					Severity:    sp.severity,
					File:        relPath,
					Line:        lineNum,
					Description: sp.description,
					Suggestion:  sp.suggestion,
				})
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("Validator: Security scan walk error: %v", err)
	}

	return issues
}

// Error parsing helpers

func (v *Validator) parseTypeScriptErrors(output string) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	lines := strings.Split(output, "\n")

	// TypeScript error format: src/file.ts(10,5): error TS2322: message
	re := regexp.MustCompile(`^(.+?)\((\d+),(\d+)\):\s*(error|warning)\s+TS(\d+):\s*(.+)$`)

	for _, line := range lines {
		matches := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) >= 7 {
			lineNum, _ := strconv.Atoi(matches[2])
			severity := SeverityError
			if matches[4] == "warning" {
				severity = SeverityWarning
			}
			issues = append(issues, ValidationIssue{
				Type:        IssueTypeError,
				Severity:    severity,
				File:        matches[1],
				Line:        lineNum,
				Description: fmt.Sprintf("TS%s: %s", matches[5], matches[6]),
			})
		}
	}

	return issues
}

func (v *Validator) parseGoBuildErrors(output string) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	lines := strings.Split(output, "\n")

	// Go error format: file.go:10:5: error message
	re := regexp.MustCompile(`^(.+?):(\d+):(\d+):\s*(.+)$`)

	for _, line := range lines {
		matches := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) >= 5 {
			lineNum, _ := strconv.Atoi(matches[2])
			issues = append(issues, ValidationIssue{
				Type:        IssueSyntaxError,
				Severity:    SeverityError,
				File:        matches[1],
				Line:        lineNum,
				Description: matches[4],
			})
		}
	}

	return issues
}

func (v *Validator) parseGoVetOutput(output string) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	lines := strings.Split(output, "\n")

	re := regexp.MustCompile(`^(.+?):(\d+):(\d+):\s*(.+)$`)

	for _, line := range lines {
		matches := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) >= 5 {
			lineNum, _ := strconv.Atoi(matches[2])
			issues = append(issues, ValidationIssue{
				Type:        IssueBestPractice,
				Severity:    SeverityWarning,
				File:        matches[1],
				Line:        lineNum,
				Description: fmt.Sprintf("go vet: %s", matches[4]),
			})
		}
	}

	return issues
}

func (v *Validator) parseFlake8Output(output string) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	lines := strings.Split(output, "\n")

	// Flake8 format: path:line:col: code text
	re := regexp.MustCompile(`^(.+?):(\d+):(\d+):\s*([A-Z]\d+)\s+(.+)$`)

	for _, line := range lines {
		matches := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) >= 6 {
			lineNum, _ := strconv.Atoi(matches[2])
			issues = append(issues, ValidationIssue{
				Type:        IssueBestPractice,
				Severity:    SeverityWarning,
				File:        matches[1],
				Line:        lineNum,
				Description: fmt.Sprintf("%s: %s", matches[4], matches[5]),
			})
		}
	}

	return issues
}

func (v *Validator) parseMypyOutput(output string) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	lines := strings.Split(output, "\n")

	// mypy format: file.py:10: error: message
	re := regexp.MustCompile(`^(.+?):(\d+):\s*(error|warning|note):\s*(.+)$`)

	for _, line := range lines {
		matches := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) >= 5 {
			lineNum, _ := strconv.Atoi(matches[2])
			severity := SeverityWarning
			if matches[3] == "error" {
				severity = SeverityError
			}
			issues = append(issues, ValidationIssue{
				Type:        IssueTypeError,
				Severity:    severity,
				File:        matches[1],
				Line:        lineNum,
				Description: fmt.Sprintf("mypy: %s", matches[4]),
			})
		}
	}

	return issues
}

func (v *Validator) parseCargoOutput(output string) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	lines := strings.Split(output, "\n")

	// Rust error format: error[E0001]: message
	//                   --> src/main.rs:10:5
	currentError := ""
	re := regexp.MustCompile(`^\s*-->\s*(.+?):(\d+):(\d+)$`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "error") || strings.HasPrefix(trimmed, "warning") {
			currentError = trimmed
		} else if matches := re.FindStringSubmatch(trimmed); len(matches) >= 4 && currentError != "" {
			lineNum, _ := strconv.Atoi(matches[2])
			severity := SeverityError
			if strings.HasPrefix(currentError, "warning") {
				severity = SeverityWarning
			}
			issues = append(issues, ValidationIssue{
				Type:        IssueSyntaxError,
				Severity:    severity,
				File:        matches[1],
				Line:        lineNum,
				Description: currentError,
			})
			currentError = ""
		}
	}

	return issues
}

func (v *Validator) parseESLintOutput(output string) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	// Try to parse as JSON
	var eslintResults []struct {
		FilePath string `json:"filePath"`
		Messages []struct {
			Line     int    `json:"line"`
			Column   int    `json:"column"`
			Severity int    `json:"severity"`
			Message  string `json:"message"`
			RuleID   string `json:"ruleId"`
		} `json:"messages"`
	}

	if err := json.Unmarshal([]byte(output), &eslintResults); err == nil {
		for _, result := range eslintResults {
			for _, msg := range result.Messages {
				severity := SeverityWarning
				if msg.Severity == 2 {
					severity = SeverityError
				}
				issues = append(issues, ValidationIssue{
					Type:        IssueBestPractice,
					Severity:    severity,
					File:        result.FilePath,
					Line:        msg.Line,
					Description: fmt.Sprintf("[%s] %s", msg.RuleID, msg.Message),
				})
			}
		}
	}

	return issues
}

func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}

// ValidationResult contains the validation outcome
type ValidationResult struct {
	Valid       bool              `json:"valid"`
	Score       int               `json:"score"`        // 0-100
	Summary     string            `json:"summary"`      // Overall summary
	Issues      []ValidationIssue `json:"issues"`       // Specific issues found
	Suggestions []string          `json:"suggestions"`  // Improvement suggestions
	Fixes       []SuggestedFix    `json:"fixes"`        // Specific fixes to apply
}

// ValidationIssue represents a specific problem found
type ValidationIssue struct {
	Type        IssueType `json:"type"`
	Severity    Severity  `json:"severity"`
	File        string    `json:"file,omitempty"`
	Line        int       `json:"line,omitempty"`
	Description string    `json:"description"`
	Suggestion  string    `json:"suggestion,omitempty"`
}

// IssueType categorizes validation issues
type IssueType string

const (
	IssueSyntaxError      IssueType = "syntax_error"
	IssueMissingImport    IssueType = "missing_import"
	IssueUndefinedVar     IssueType = "undefined_variable"
	IssueTypeError        IssueType = "type_error"
	IssueMissingFile      IssueType = "missing_file"
	IssueIncomplete       IssueType = "incomplete_implementation"
	IssuePlaceholder      IssueType = "placeholder_code"
	IssueSecurityRisk     IssueType = "security_risk"
	IssueBestPractice     IssueType = "best_practice"
	IssueTestFailure      IssueType = "test_failure"
	IssueDependencyMissing IssueType = "dependency_missing"
)

// Severity levels for issues
type Severity string

const (
	SeverityCritical Severity = "critical" // Blocks execution
	SeverityError    Severity = "error"    // Must fix
	SeverityWarning  Severity = "warning"  // Should fix
	SeverityInfo     Severity = "info"     // Nice to fix
)

// SuggestedFix contains a specific fix to apply
type SuggestedFix struct {
	File       string `json:"file"`
	Type       string `json:"type"` // replace, insert, delete
	Search     string `json:"search,omitempty"`
	Replace    string `json:"replace,omitempty"`
	Line       int    `json:"line,omitempty"`
	Content    string `json:"content,omitempty"`
	Reason     string `json:"reason"`
}

// ValidateOutput validates the generated output
func (v *Validator) ValidateOutput(ctx context.Context, plan *ExecutionPlan, artifacts []Artifact) (*ValidationResult, error) {
	log.Println("Validator: Starting output validation")

	result := &ValidationResult{
		Valid:       true,
		Score:       100,
		Issues:      make([]ValidationIssue, 0),
		Suggestions: make([]string, 0),
		Fixes:       make([]SuggestedFix, 0),
	}

	// Step 1: Validate that required files exist
	fileIssues := v.validateRequiredFiles(plan, artifacts)
	result.Issues = append(result.Issues, fileIssues...)

	// Step 2: Validate file contents (syntax, placeholders, completeness)
	for _, artifact := range artifacts {
		if artifact.Type == "file" && artifact.Content != "" {
			contentIssues := v.validateFileContent(artifact.Path, artifact.Content)
			result.Issues = append(result.Issues, contentIssues...)
		}
	}

	// Step 3: Use AI for deeper code analysis
	if len(artifacts) > 0 {
		aiIssues, err := v.aiValidation(ctx, artifacts)
		if err != nil {
			log.Printf("Validator: AI validation failed: %v", err)
			// Continue with basic validation
		} else {
			result.Issues = append(result.Issues, aiIssues...)
		}
	}

	// Step 4: Run actual build verification if work directory exists
	if v.workDir != "" {
		buildIssues := v.verifyBuild(ctx)
		result.Issues = append(result.Issues, buildIssues...)
	}

	// Calculate score and determine validity
	result.Score = v.calculateScore(result.Issues)
	result.Valid = result.Score >= 60 && !v.hasCriticalIssues(result.Issues)
	result.Summary = v.generateSummary(result)

	// Generate suggestions for improvement
	result.Suggestions = v.generateSuggestions(result.Issues)

	log.Printf("Validator: Validation complete - Score: %d, Valid: %v, Issues: %d",
		result.Score, result.Valid, len(result.Issues))

	return result, nil
}

// validateRequiredFiles checks that necessary files were created
func (v *Validator) validateRequiredFiles(plan *ExecutionPlan, artifacts []Artifact) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	if plan == nil {
		return issues
	}

	// Build a set of created files
	createdFiles := make(map[string]bool)
	for _, artifact := range artifacts {
		if artifact.Type == "file" {
			createdFiles[artifact.Path] = true
			createdFiles[artifact.Name] = true
		}
	}

	// Check each step that should produce files
	for _, step := range plan.Steps {
		if step.Status != StepCompleted {
			continue
		}

		expectedFiles := v.getExpectedFiles(step)
		for _, file := range expectedFiles {
			if !createdFiles[file] && !createdFiles[filepath.Base(file)] {
				issues = append(issues, ValidationIssue{
					Type:        IssueMissingFile,
					Severity:    SeverityError,
					File:        file,
					Description: fmt.Sprintf("Expected file %s was not created", file),
					Suggestion:  "Re-run the generation step or manually create the file",
				})
			}
		}
	}

	return issues
}

// getExpectedFiles returns files expected from a step
func (v *Validator) getExpectedFiles(step *PlanStep) []string {
	files := make([]string, 0)

	if step.Output == nil {
		return files
	}

	// Check output for file list
	if outputFiles, ok := step.Output["files_created"].([]string); ok {
		return outputFiles
	}

	// Check output for single file
	if filePath, ok := step.Output["path"].(string); ok {
		files = append(files, filePath)
	}

	return files
}

// validateFileContent checks file contents for issues
func (v *Validator) validateFileContent(filePath string, content string) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	// Check for placeholder patterns
	placeholderPatterns := []string{
		`TODO:?`,
		`FIXME:?`,
		`XXX:?`,
		`PLACEHOLDER`,
		`// \.\.\.$`,
		`# \.\.\.$`,
		`\{\s*\.\.\.\s*\}`,
		`pass\s*#\s*TODO`,
		`throw new Error\(['"]Not implemented['"]\)`,
		`raise NotImplementedError`,
		`undefined // implement`,
		`null // TODO`,
		`demo data`,
		`example data`,
		`mock data`,
		`sample data`,
		`Lorem ipsum`,
	}

	for _, pattern := range placeholderPatterns {
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			continue
		}

		matches := re.FindAllStringIndex(content, -1)
		for _, match := range matches {
			// Find line number
			lineNum := strings.Count(content[:match[0]], "\n") + 1
			issues = append(issues, ValidationIssue{
				Type:        IssuePlaceholder,
				Severity:    SeverityError,
				File:        filePath,
				Line:        lineNum,
				Description: "Placeholder code detected",
				Suggestion:  "Replace with complete implementation",
			})
		}
	}

	// Check for empty function bodies
	emptyFuncPatterns := []string{
		`function\s+\w+\([^)]*\)\s*\{\s*\}`,
		`const\s+\w+\s*=\s*\([^)]*\)\s*=>\s*\{\s*\}`,
		`def\s+\w+\([^)]*\):\s*pass\s*$`,
		`func\s+\w+\([^)]*\)\s*\{\s*\}`,
	}

	for _, pattern := range emptyFuncPatterns {
		re, err := regexp.Compile("(?m)" + pattern)
		if err != nil {
			continue
		}

		matches := re.FindAllStringIndex(content, -1)
		for _, match := range matches {
			lineNum := strings.Count(content[:match[0]], "\n") + 1
			issues = append(issues, ValidationIssue{
				Type:        IssueIncomplete,
				Severity:    SeverityWarning,
				File:        filePath,
				Line:        lineNum,
				Description: "Empty function body detected",
				Suggestion:  "Implement function logic",
			})
		}
	}

	// Check for common syntax issues based on file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".ts", ".tsx", ".js", ".jsx":
		issues = append(issues, v.validateJavaScriptSyntax(filePath, content)...)
	case ".py":
		issues = append(issues, v.validatePythonSyntax(filePath, content)...)
	case ".go":
		issues = append(issues, v.validateGoSyntax(filePath, content)...)
	}

	return issues
}

// validateJavaScriptSyntax checks JavaScript/TypeScript for common issues
func (v *Validator) validateJavaScriptSyntax(filePath string, content string) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	// Check for unbalanced brackets
	if strings.Count(content, "{") != strings.Count(content, "}") {
		issues = append(issues, ValidationIssue{
			Type:        IssueSyntaxError,
			Severity:    SeverityCritical,
			File:        filePath,
			Description: "Unbalanced curly braces",
			Suggestion:  "Check for missing { or }",
		})
	}

	// Check for common import issues
	if strings.Contains(content, "import ") {
		// Check for imports from undefined packages
		importPattern := regexp.MustCompile(`import\s+(?:[\w\s{},*]+\s+from\s+)?['"]([^'"]+)['"]`)
		matches := importPattern.FindAllStringSubmatch(content, -1)

		for _, match := range matches {
			if len(match) > 1 {
				pkg := match[1]
				// Check for relative imports that might not exist
				if strings.HasPrefix(pkg, ".") {
					// Relative import - could validate file exists
				}
			}
		}
	}

	// Check for console.log in production code
	if strings.Contains(content, "console.log(") {
		issues = append(issues, ValidationIssue{
			Type:        IssueBestPractice,
			Severity:    SeverityInfo,
			File:        filePath,
			Description: "console.log found in code",
			Suggestion:  "Consider using proper logging or removing debug statements",
		})
	}

	return issues
}

// validatePythonSyntax checks Python for common issues
func (v *Validator) validatePythonSyntax(filePath string, content string) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	// Check for inconsistent indentation
	lines := strings.Split(content, "\n")
	var lastIndent int
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Count leading spaces
		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		// Check for mixed tabs and spaces
		if strings.Contains(line[:indent], "\t") && strings.Contains(line[:indent], " ") {
			issues = append(issues, ValidationIssue{
				Type:        IssueSyntaxError,
				Severity:    SeverityError,
				File:        filePath,
				Line:        i + 1,
				Description: "Mixed tabs and spaces in indentation",
				Suggestion:  "Use consistent indentation (spaces recommended)",
			})
		}

		lastIndent = indent
	}
	_ = lastIndent

	return issues
}

// validateGoSyntax checks Go for common issues
func (v *Validator) validateGoSyntax(filePath string, content string) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	// Check for unbalanced brackets
	if strings.Count(content, "{") != strings.Count(content, "}") {
		issues = append(issues, ValidationIssue{
			Type:        IssueSyntaxError,
			Severity:    SeverityCritical,
			File:        filePath,
			Description: "Unbalanced curly braces",
			Suggestion:  "Check for missing { or }",
		})
	}

	// Check for missing error handling
	errPattern := regexp.MustCompile(`\w+,\s*_\s*:?=\s*\w+\(`)
	if errPattern.MatchString(content) {
		issues = append(issues, ValidationIssue{
			Type:        IssueBestPractice,
			Severity:    SeverityWarning,
			File:        filePath,
			Description: "Error return value ignored (using _)",
			Suggestion:  "Handle errors properly instead of ignoring them",
		})
	}

	return issues
}

// aiValidation uses AI for deeper code analysis
func (v *Validator) aiValidation(ctx context.Context, artifacts []Artifact) ([]ValidationIssue, error) {
	issues := make([]ValidationIssue, 0)

	// Prepare code samples for AI analysis
	var codeContent strings.Builder
	for _, artifact := range artifacts {
		if artifact.Type == "file" && len(artifact.Content) > 0 && len(artifact.Content) < 5000 {
			codeContent.WriteString(fmt.Sprintf("\n\n=== %s ===\n%s", artifact.Path, artifact.Content))
		}
	}

	if codeContent.Len() == 0 {
		return issues, nil
	}

	prompt := fmt.Sprintf(`Analyze this code for issues. Output ONLY valid JSON:
%s

Output format:
{
  "issues": [
    {"type": "syntax_error|missing_import|type_error|incomplete_implementation|security_risk", "severity": "critical|error|warning", "file": "path", "description": "issue description", "suggestion": "how to fix"}
  ]
}

Focus on:
1. Missing imports or dependencies
2. Incomplete implementations
3. Security vulnerabilities
4. Type errors
5. Logic errors`, codeContent.String())

	response, err := v.ai.Analyze(ctx, codeContent.String(), prompt, AIOptions{
		MaxTokens:   2000,
		Temperature: 0.2,
		SystemPrompt: "You are a code reviewer. Identify issues and output valid JSON only.",
	})
	if err != nil {
		return issues, err
	}

	// Parse AI response
	var aiResult struct {
		Issues []struct {
			Type        string `json:"type"`
			Severity    string `json:"severity"`
			File        string `json:"file"`
			Description string `json:"description"`
			Suggestion  string `json:"suggestion"`
		} `json:"issues"`
	}

	// Try to extract JSON from response
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
		response = strings.TrimSuffix(response, "```")
	}

	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start != -1 && end != -1 && end > start {
		response = response[start : end+1]
	}

	if err := json.Unmarshal([]byte(response), &aiResult); err != nil {
		log.Printf("Validator: Failed to parse AI response: %v", err)
		return issues, nil
	}

	// Convert to ValidationIssue
	for _, issue := range aiResult.Issues {
		severity := SeverityWarning
		switch strings.ToLower(issue.Severity) {
		case "critical":
			severity = SeverityCritical
		case "error":
			severity = SeverityError
		case "info":
			severity = SeverityInfo
		}

		issueType := IssueBestPractice
		switch strings.ToLower(issue.Type) {
		case "syntax_error":
			issueType = IssueSyntaxError
		case "missing_import":
			issueType = IssueMissingImport
		case "type_error":
			issueType = IssueTypeError
		case "incomplete_implementation":
			issueType = IssueIncomplete
		case "security_risk":
			issueType = IssueSecurityRisk
		}

		issues = append(issues, ValidationIssue{
			Type:        issueType,
			Severity:    severity,
			File:        issue.File,
			Description: issue.Description,
			Suggestion:  issue.Suggestion,
		})
	}

	return issues, nil
}

// calculateScore computes an overall quality score
func (v *Validator) calculateScore(issues []ValidationIssue) int {
	score := 100

	for _, issue := range issues {
		switch issue.Severity {
		case SeverityCritical:
			score -= 25
		case SeverityError:
			score -= 15
		case SeverityWarning:
			score -= 5
		case SeverityInfo:
			score -= 1
		}
	}

	if score < 0 {
		score = 0
	}

	return score
}

// hasCriticalIssues checks if there are blocking issues
func (v *Validator) hasCriticalIssues(issues []ValidationIssue) bool {
	for _, issue := range issues {
		if issue.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

// generateSummary creates a human-readable summary
func (v *Validator) generateSummary(result *ValidationResult) string {
	if result.Valid {
		if result.Score >= 90 {
			return "Excellent! Code quality is high with minimal issues."
		} else if result.Score >= 75 {
			return "Good quality code with some minor issues to address."
		} else {
			return "Code is acceptable but has several issues that should be fixed."
		}
	}

	criticalCount := 0
	errorCount := 0
	for _, issue := range result.Issues {
		if issue.Severity == SeverityCritical {
			criticalCount++
		} else if issue.Severity == SeverityError {
			errorCount++
		}
	}

	if criticalCount > 0 {
		return fmt.Sprintf("Validation failed: %d critical issues and %d errors must be fixed.", criticalCount, errorCount)
	}

	return fmt.Sprintf("Validation failed: %d errors need to be addressed.", errorCount)
}

// generateSuggestions creates improvement suggestions
func (v *Validator) generateSuggestions(issues []ValidationIssue) []string {
	suggestions := make([]string, 0)
	suggestionSet := make(map[string]bool)

	for _, issue := range issues {
		if issue.Suggestion != "" && !suggestionSet[issue.Suggestion] {
			suggestions = append(suggestions, issue.Suggestion)
			suggestionSet[issue.Suggestion] = true
		}
	}

	return suggestions
}

// AnalyzeError uses AI to analyze an error and suggest fixes
func (v *Validator) AnalyzeError(ctx context.Context, errorMsg string, step *PlanStep) string {
	prompt := fmt.Sprintf(`Analyze this error and suggest a fix.

Error: %s

Step that failed:
- Name: %s
- Type: %s
- Input: %v

Provide a brief analysis and suggested fix (1-2 sentences).`, errorMsg, step.Name, step.ActionType, step.Input)

	response, err := v.ai.Analyze(ctx, errorMsg, prompt, AIOptions{
		MaxTokens:    500,
		Temperature:  0.3,
		SystemPrompt: "You are a debugging expert. Provide brief, actionable error analysis.",
	})
	if err != nil {
		return ""
	}

	return response
}

// ValidateFile validates a single file's content
func (v *Validator) ValidateFile(ctx context.Context, filePath string, content string) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:       true,
		Score:       100,
		Issues:      make([]ValidationIssue, 0),
		Suggestions: make([]string, 0),
	}

	// Run content validation
	issues := v.validateFileContent(filePath, content)
	result.Issues = append(result.Issues, issues...)

	// Calculate score
	result.Score = v.calculateScore(result.Issues)
	result.Valid = result.Score >= 70 && !v.hasCriticalIssues(result.Issues)
	result.Summary = v.generateSummary(result)
	result.Suggestions = v.generateSuggestions(result.Issues)

	return result, nil
}

// SuggestFixes generates specific code fixes for issues
func (v *Validator) SuggestFixes(ctx context.Context, issues []ValidationIssue, fileContents map[string]string) ([]SuggestedFix, error) {
	fixes := make([]SuggestedFix, 0)

	// Group issues by file
	issuesByFile := make(map[string][]ValidationIssue)
	for _, issue := range issues {
		if issue.File != "" {
			issuesByFile[issue.File] = append(issuesByFile[issue.File], issue)
		}
	}

	// Generate fixes for each file
	for filePath, fileIssues := range issuesByFile {
		content, ok := fileContents[filePath]
		if !ok {
			continue
		}

		// Build prompt for AI to generate fixes
		var issueList strings.Builder
		for _, issue := range fileIssues {
			issueList.WriteString(fmt.Sprintf("- Line %d: %s (%s)\n", issue.Line, issue.Description, issue.Type))
		}

		prompt := fmt.Sprintf(`Generate specific code fixes for these issues.

File: %s
Content:
%s

Issues:
%s

Output ONLY valid JSON:
{
  "fixes": [
    {"type": "replace", "search": "original code", "replace": "fixed code", "reason": "why"}
  ]
}`, filePath, content, issueList.String())

		response, err := v.ai.Generate(ctx, prompt, AIOptions{
			MaxTokens:    2000,
			Temperature:  0.2,
			SystemPrompt: "You are a code fixer. Generate specific code replacements.",
		})
		if err != nil {
			log.Printf("Validator: Failed to generate fixes for %s: %v", filePath, err)
			continue
		}

		// Parse response
		var fixResult struct {
			Fixes []struct {
				Type    string `json:"type"`
				Search  string `json:"search"`
				Replace string `json:"replace"`
				Reason  string `json:"reason"`
			} `json:"fixes"`
		}

		// Extract JSON
		response = strings.TrimSpace(response)
		start := strings.Index(response, "{")
		end := strings.LastIndex(response, "}")
		if start != -1 && end != -1 && end > start {
			response = response[start : end+1]
		}

		if err := json.Unmarshal([]byte(response), &fixResult); err != nil {
			continue
		}

		for _, fix := range fixResult.Fixes {
			fixes = append(fixes, SuggestedFix{
				File:    filePath,
				Type:    fix.Type,
				Search:  fix.Search,
				Replace: fix.Replace,
				Reason:  fix.Reason,
			})
		}
	}

	return fixes, nil
}

// ValidateProjectStructure checks overall project structure
func (v *Validator) ValidateProjectStructure(workDir string) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:       true,
		Score:       100,
		Issues:      make([]ValidationIssue, 0),
		Suggestions: make([]string, 0),
	}

	// Check for essential files
	essentialFiles := []string{
		"package.json",
		"tsconfig.json",
	}

	for _, file := range essentialFiles {
		fullPath := filepath.Join(workDir, file)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			result.Issues = append(result.Issues, ValidationIssue{
				Type:        IssueMissingFile,
				Severity:    SeverityWarning,
				File:        file,
				Description: fmt.Sprintf("Essential file %s is missing", file),
				Suggestion:  "Create the file with appropriate configuration",
			})
		}
	}

	// Check for common directories
	commonDirs := []string{
		"src",
		"public",
	}

	for _, dir := range commonDirs {
		fullPath := filepath.Join(workDir, dir)
		if stat, err := os.Stat(fullPath); os.IsNotExist(err) || !stat.IsDir() {
			result.Issues = append(result.Issues, ValidationIssue{
				Type:        IssueMissingFile,
				Severity:    SeverityInfo,
				File:        dir,
				Description: fmt.Sprintf("Common directory %s is missing", dir),
				Suggestion:  "Consider adding this directory for better organization",
			})
		}
	}

	result.Score = v.calculateScore(result.Issues)
	result.Valid = result.Score >= 60
	result.Summary = v.generateSummary(result)
	result.Suggestions = v.generateSuggestions(result.Issues)

	return result, nil
}
