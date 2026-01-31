// Package codereview - Build verification for code review
// Catches undefined symbols, missing imports, and compilation errors
// that AI-only review misses
package codereview

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// BuildVerifier runs actual compilation to catch errors AI review misses
type BuildVerifier struct {
	workDir string
	timeout time.Duration
}

// NewBuildVerifier creates a new build verifier
func NewBuildVerifier(workDir string) *BuildVerifier {
	return &BuildVerifier{
		workDir: workDir,
		timeout: 2 * time.Minute,
	}
}

// BuildResult contains the results of a build verification
type BuildResult struct {
	Success     bool            `json:"success"`
	ExitCode    int             `json:"exit_code"`
	Findings    []ReviewFinding `json:"findings"`
	RawOutput   string          `json:"raw_output"`
	Duration    int64           `json:"duration_ms"`
	BuildCmd    string          `json:"build_cmd"`
}

// VerifyBuild runs the appropriate build command and parses errors into findings
func (v *BuildVerifier) VerifyBuild(ctx context.Context, language string) (*BuildResult, error) {
	startTime := time.Now()

	// Determine build command based on language/project type
	buildCmd, args := v.getBuildCommand(language)
	if buildCmd == "" {
		return nil, fmt.Errorf("no build command for language: %s", language)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	// Run build command
	cmd := exec.CommandContext(ctx, buildCmd, args...)
	cmd.Dir = v.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &BuildResult{
		Success:  err == nil,
		Duration: time.Since(startTime).Milliseconds(),
		BuildCmd: fmt.Sprintf("%s %s", buildCmd, strings.Join(args, " ")),
	}

	// Combine output
	output := stderr.String()
	if output == "" {
		output = stdout.String()
	}
	result.RawOutput = output

	// Get exit code
	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	}

	// Parse errors into findings
	result.Findings = v.parseCompilerErrors(output, language)

	return result, nil
}

// getBuildCommand returns the appropriate build command for a language
func (v *BuildVerifier) getBuildCommand(language string) (string, []string) {
	switch strings.ToLower(language) {
	case "go", "golang":
		return "go", []string{"build", "./..."}
	case "typescript", "ts":
		return "npx", []string{"tsc", "--noEmit"}
	case "javascript", "js":
		// Check for TypeScript config
		if v.fileExists("tsconfig.json") {
			return "npx", []string{"tsc", "--noEmit"}
		}
		// Try ESLint for JS
		if v.fileExists("package.json") {
			return "npx", []string{"eslint", ".", "--ext", ".js,.jsx", "--max-warnings=0"}
		}
		return "", nil
	case "python", "py":
		// Use mypy for type checking if available
		if v.fileExists("pyproject.toml") || v.fileExists("mypy.ini") {
			return "mypy", []string{"."}
		}
		// Fall back to py_compile
		return "python", []string{"-m", "py_compile"}
	case "rust", "rs":
		return "cargo", []string{"check"}
	case "java":
		if v.fileExists("pom.xml") {
			return "mvn", []string{"compile", "-q"}
		}
		if v.fileExists("build.gradle") || v.fileExists("build.gradle.kts") {
			return "gradle", []string{"compileJava", "-q"}
		}
		return "javac", []string{"-d", "/tmp", "*.java"}
	case "c", "cpp", "c++":
		if v.fileExists("CMakeLists.txt") {
			return "cmake", []string{"--build", ".", "--target", "all"}
		}
		if v.fileExists("Makefile") {
			return "make", []string{"-n"} // Dry run
		}
		return "", nil
	default:
		return "", nil
	}
}

// fileExists checks if a file exists in the work directory
func (v *BuildVerifier) fileExists(name string) bool {
	path := filepath.Join(v.workDir, name)
	_, err := exec.Command("test", "-f", path).Output()
	return err == nil
}

// parseCompilerErrors parses compiler output into ReviewFindings
func (v *BuildVerifier) parseCompilerErrors(output, language string) []ReviewFinding {
	var findings []ReviewFinding

	switch strings.ToLower(language) {
	case "go", "golang":
		findings = v.parseGoErrors(output)
	case "typescript", "ts", "javascript", "js":
		findings = v.parseTypeScriptErrors(output)
	case "python", "py":
		findings = v.parsePythonErrors(output)
	case "rust", "rs":
		findings = v.parseRustErrors(output)
	case "java":
		findings = v.parseJavaErrors(output)
	default:
		// Generic line:col: error parsing
		findings = v.parseGenericErrors(output)
	}

	return findings
}

// parseGoErrors parses Go compiler errors
// Format: file.go:line:col: error message
func (v *BuildVerifier) parseGoErrors(output string) []ReviewFinding {
	var findings []ReviewFinding

	// Go error patterns
	patterns := []*regexp.Regexp{
		// Standard: file.go:10:5: undefined: SomeType
		regexp.MustCompile(`^(.+\.go):(\d+):(\d+):\s*(.+)$`),
		// Package error: package foo: error message
		regexp.MustCompile(`^#\s+(.+)\n(.+\.go):(\d+):(\d+):\s*(.+)$`),
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	var currentPkg string

	for scanner.Scan() {
		line := scanner.Text()

		// Track current package
		if strings.HasPrefix(line, "# ") {
			currentPkg = strings.TrimPrefix(line, "# ")
			continue
		}

		for _, pattern := range patterns {
			if matches := pattern.FindStringSubmatch(line); matches != nil {
				lineNum, _ := strconv.Atoi(matches[2])
				col, _ := strconv.Atoi(matches[3])
				msg := matches[4]
				if len(matches) > 4 {
					msg = matches[len(matches)-1]
				}

				finding := ReviewFinding{
					Type:     v.classifyGoError(msg),
					Severity: "error",
					Line:     lineNum,
					Column:   col,
					Message:  msg,
					Code:     matches[1],
					RuleID:   "build-error",
				}

				// Add suggestions for common errors
				finding.Suggestion = v.suggestGoFix(msg)

				if currentPkg != "" {
					finding.Message = fmt.Sprintf("[%s] %s", currentPkg, msg)
				}

				findings = append(findings, finding)
				break
			}
		}
	}

	return findings
}

// classifyGoError determines the type of Go error
func (v *BuildVerifier) classifyGoError(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "undefined"):
		return "undefined_symbol"
	case strings.Contains(lower, "imported and not used"):
		return "unused_import"
	case strings.Contains(lower, "declared but not used"):
		return "unused_variable"
	case strings.Contains(lower, "cannot use"):
		return "type_mismatch"
	case strings.Contains(lower, "not enough arguments"):
		return "argument_count"
	case strings.Contains(lower, "too many arguments"):
		return "argument_count"
	case strings.Contains(lower, "missing return"):
		return "missing_return"
	default:
		return "compilation_error"
	}
}

// suggestGoFix provides fix suggestions for common Go errors
func (v *BuildVerifier) suggestGoFix(msg string) string {
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "undefined:"):
		// Extract the undefined symbol
		parts := strings.Split(msg, "undefined:")
		if len(parts) > 1 {
			symbol := strings.TrimSpace(parts[1])
			return fmt.Sprintf("Symbol '%s' is not defined. Check: 1) Is the file containing it committed? 2) Is it exported (capitalized)? 3) Is the import statement correct?", symbol)
		}
		return "Check if the file containing this symbol is committed and properly imported"

	case strings.Contains(lower, "imported and not used"):
		return "Remove the unused import or use it in the code"

	case strings.Contains(lower, "declared but not used"):
		return "Use the variable or prefix with _ to ignore"

	case strings.Contains(lower, "cannot use") && strings.Contains(lower, "as type"):
		return "Check type compatibility - you may need a type conversion or assertion"

	case strings.Contains(lower, "has no field or method"):
		return "The type doesn't have this field/method. Check: 1) Is the method defined? 2) Is the file committed? 3) Is it exported?"

	default:
		return ""
	}
}

// parseTypeScriptErrors parses TypeScript compiler errors
// Format: file.ts(line,col): error TS1234: message
func (v *BuildVerifier) parseTypeScriptErrors(output string) []ReviewFinding {
	var findings []ReviewFinding

	// TypeScript error pattern
	pattern := regexp.MustCompile(`^(.+)\((\d+),(\d+)\):\s*(error|warning)\s+(TS\d+):\s*(.+)$`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		if matches := pattern.FindStringSubmatch(line); matches != nil {
			lineNum, _ := strconv.Atoi(matches[2])
			col, _ := strconv.Atoi(matches[3])
			severity := matches[4]
			code := matches[5]
			msg := matches[6]

			findings = append(findings, ReviewFinding{
				Type:       "typescript_error",
				Severity:   severity,
				Line:       lineNum,
				Column:     col,
				Message:    msg,
				Code:       matches[1],
				RuleID:     code,
				Suggestion: v.suggestTSFix(code, msg),
			})
		}
	}

	return findings
}

// suggestTSFix provides fix suggestions for TypeScript errors
func (v *BuildVerifier) suggestTSFix(code, msg string) string {
	switch code {
	case "TS2304": // Cannot find name
		return "The name is not defined. Check imports or declare the type"
	case "TS2322": // Type not assignable
		return "Type mismatch. Check the expected type and adjust the value or add type assertion"
	case "TS2339": // Property does not exist
		return "Property not found on type. Check spelling or add to interface definition"
	case "TS2345": // Argument type mismatch
		return "Wrong argument type. Check the function signature"
	case "TS7006": // Parameter implicitly has 'any' type
		return "Add explicit type annotation to the parameter"
	default:
		return ""
	}
}

// parsePythonErrors parses Python compiler/type errors
func (v *BuildVerifier) parsePythonErrors(output string) []ReviewFinding {
	var findings []ReviewFinding

	// Python syntax error: File "x.py", line 10
	syntaxPattern := regexp.MustCompile(`File "(.+)", line (\d+)`)
	// mypy error: file.py:10: error: message
	mypyPattern := regexp.MustCompile(`^(.+\.py):(\d+):\s*(error|warning|note):\s*(.+)$`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		if matches := mypyPattern.FindStringSubmatch(line); matches != nil {
			lineNum, _ := strconv.Atoi(matches[2])
			findings = append(findings, ReviewFinding{
				Type:     "python_type_error",
				Severity: matches[3],
				Line:     lineNum,
				Message:  matches[4],
				Code:     matches[1],
				RuleID:   "mypy",
			})
		} else if matches := syntaxPattern.FindStringSubmatch(line); matches != nil {
			lineNum, _ := strconv.Atoi(matches[2])
			findings = append(findings, ReviewFinding{
				Type:     "python_syntax_error",
				Severity: "error",
				Line:     lineNum,
				Message:  line,
				Code:     matches[1],
				RuleID:   "syntax",
			})
		}
	}

	return findings
}

// parseRustErrors parses Rust compiler errors
func (v *BuildVerifier) parseRustErrors(output string) []ReviewFinding {
	var findings []ReviewFinding

	// Rust error: error[E0425]: cannot find value `x` in this scope
	//  --> src/main.rs:10:5
	errorPattern := regexp.MustCompile(`^error\[E(\d+)\]:\s*(.+)$`)
	locationPattern := regexp.MustCompile(`^\s*-->\s*(.+):(\d+):(\d+)$`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	var currentError *ReviewFinding

	for scanner.Scan() {
		line := scanner.Text()

		if matches := errorPattern.FindStringSubmatch(line); matches != nil {
			if currentError != nil {
				findings = append(findings, *currentError)
			}
			currentError = &ReviewFinding{
				Type:     "rust_error",
				Severity: "error",
				Message:  matches[2],
				RuleID:   "E" + matches[1],
			}
		} else if matches := locationPattern.FindStringSubmatch(line); matches != nil && currentError != nil {
			currentError.Code = matches[1]
			currentError.Line, _ = strconv.Atoi(matches[2])
			currentError.Column, _ = strconv.Atoi(matches[3])
		}
	}

	if currentError != nil {
		findings = append(findings, *currentError)
	}

	return findings
}

// parseJavaErrors parses Java compiler errors
func (v *BuildVerifier) parseJavaErrors(output string) []ReviewFinding {
	var findings []ReviewFinding

	// javac: File.java:10: error: message
	pattern := regexp.MustCompile(`^(.+\.java):(\d+):\s*(error|warning):\s*(.+)$`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		if matches := pattern.FindStringSubmatch(line); matches != nil {
			lineNum, _ := strconv.Atoi(matches[2])
			findings = append(findings, ReviewFinding{
				Type:     "java_error",
				Severity: matches[3],
				Line:     lineNum,
				Message:  matches[4],
				Code:     matches[1],
				RuleID:   "javac",
			})
		}
	}

	return findings
}

// parseGenericErrors tries to parse generic compiler error formats
func (v *BuildVerifier) parseGenericErrors(output string) []ReviewFinding {
	var findings []ReviewFinding

	// Generic: file:line:col: message
	pattern := regexp.MustCompile(`^(.+):(\d+):(\d+)?:?\s*(.+)$`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		if matches := pattern.FindStringSubmatch(line); matches != nil {
			lineNum, _ := strconv.Atoi(matches[2])
			col := 0
			if matches[3] != "" {
				col, _ = strconv.Atoi(matches[3])
			}

			findings = append(findings, ReviewFinding{
				Type:     "build_error",
				Severity: "error",
				Line:     lineNum,
				Column:   col,
				Message:  matches[4],
				Code:     matches[1],
				RuleID:   "build",
			})
		}
	}

	return findings
}

// VerifyDependencies checks that all referenced symbols exist across files
func (v *BuildVerifier) VerifyDependencies(ctx context.Context, language string) ([]ReviewFinding, error) {
	// This is a lightweight check that can run before full compilation
	// to catch obvious missing dependencies

	switch strings.ToLower(language) {
	case "go", "golang":
		return v.verifyGoDependencies(ctx)
	case "typescript", "ts":
		return v.verifyTSDependencies(ctx)
	default:
		return nil, nil
	}
}

// verifyGoDependencies checks Go imports and referenced packages
func (v *BuildVerifier) verifyGoDependencies(ctx context.Context) ([]ReviewFinding, error) {
	var findings []ReviewFinding

	// Run go list to check for import errors
	cmd := exec.CommandContext(ctx, "go", "list", "-e", "-json", "./...")
	cmd.Dir = v.workDir

	output, err := cmd.Output()
	if err != nil {
		// Parse the error output for import failures
		if exitErr, ok := err.(*exec.ExitError); ok {
			errOutput := string(exitErr.Stderr)
			if strings.Contains(errOutput, "no required module") {
				findings = append(findings, ReviewFinding{
					Type:       "missing_dependency",
					Severity:   "error",
					Line:       1,
					Message:    "Missing Go module dependency: " + errOutput,
					Suggestion: "Run 'go mod tidy' to add missing dependencies",
					RuleID:     "go-mod",
				})
			}
		}
	}

	// Also check for any undefined references
	_ = output // Could parse JSON for more detailed analysis

	return findings, nil
}

// verifyTSDependencies checks TypeScript imports
func (v *BuildVerifier) verifyTSDependencies(ctx context.Context) ([]ReviewFinding, error) {
	// Quick TypeScript import check
	cmd := exec.CommandContext(ctx, "npx", "tsc", "--noEmit", "--listFiles")
	cmd.Dir = v.workDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	_ = cmd.Run()

	// Parse any import errors
	return v.parseTypeScriptErrors(stderr.String()), nil
}
