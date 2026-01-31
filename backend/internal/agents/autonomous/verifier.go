// Package autonomous - Build Verification Agent
// Pre-delivery pipeline: install → lint → build → test → AI review → retry
package autonomous

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// BuildVerifier runs a multi-step build verification pipeline on generated code
type BuildVerifier struct {
	ai      AIProvider
	workDir string
}

// NewBuildVerifier creates a new build verifier
func NewBuildVerifier(ai AIProvider, workDir string) *BuildVerifier {
	return &BuildVerifier{ai: ai, workDir: workDir}
}

// VerificationResult contains the outcome of the full verification pipeline
type VerificationResult struct {
	Passed        bool               `json:"passed"`
	Score         int                `json:"score"`       // 0-100 quality score
	Steps         []VerifyStep       `json:"steps"`
	Summary       string             `json:"summary"`
	Duration      int64              `json:"duration_ms"`
	Retries       int                `json:"retries"`
	FixesApplied  []AppliedFix       `json:"fixes_applied,omitempty"`
	SecurityIssues []SecurityFinding `json:"security_issues,omitempty"`
	DependencyAudit *DependencyAudit `json:"dependency_audit,omitempty"`
	CoverageReport *CoverageReport  `json:"coverage_report,omitempty"`
}

// SecurityFinding represents a security vulnerability found
type SecurityFinding struct {
	Severity    string `json:"severity"` // critical, high, medium, low
	Type        string `json:"type"`     // vulnerability, exposure, misconfiguration
	Package     string `json:"package,omitempty"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description"`
	Remediation string `json:"remediation,omitempty"`
	CVE         string `json:"cve,omitempty"`
}

// DependencyAudit contains results of dependency vulnerability scanning
type DependencyAudit struct {
	TotalDependencies int               `json:"total_dependencies"`
	Vulnerabilities   int               `json:"vulnerabilities"`
	OutdatedCount     int               `json:"outdated_count"`
	Findings          []SecurityFinding `json:"findings,omitempty"`
}

// CoverageReport contains test coverage information
type CoverageReport struct {
	LineCoverage   float64 `json:"line_coverage"`
	BranchCoverage float64 `json:"branch_coverage,omitempty"`
	FunctionsCovered int   `json:"functions_covered"`
	FunctionsTotal   int   `json:"functions_total"`
}

// VerifyStep represents one step in the verification pipeline
type VerifyStep struct {
	Name     string `json:"name"`
	Status   string `json:"status"` // passed, failed, skipped
	Output   string `json:"output,omitempty"`
	Duration int64  `json:"duration_ms"`
	Score    int    `json:"score"` // Points awarded for this step
}

// AppliedFix represents a fix that was automatically applied during retry
type AppliedFix struct {
	File        string `json:"file"`
	Description string `json:"description"`
	Iteration   int    `json:"iteration"`
}

// Verify runs the complete verification pipeline with retry logic
func (bv *BuildVerifier) Verify(ctx context.Context, maxRetries int) (*VerificationResult, error) {
	startTime := time.Now()

	if maxRetries <= 0 {
		maxRetries = 3
	}

	result := &VerificationResult{}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		result.Retries = attempt

		steps := bv.runPipeline(ctx)
		result.Steps = steps
		result.Score = bv.calculateScore(steps)

		// If score is passing (60+), we're done
		if result.Score >= 60 {
			result.Passed = true
			break
		}

		// If we have retries left, try to fix errors
		if attempt < maxRetries {
			fixes := bv.attemptFixes(ctx, steps)
			result.FixesApplied = append(result.FixesApplied, fixes...)

			if len(fixes) == 0 {
				// No fixes could be generated, stop retrying
				break
			}
		}
	}

	result.Duration = time.Since(startTime).Milliseconds()
	result.Summary = bv.buildSummary(result)

	return result, nil
}

// runPipeline executes each verification step
func (bv *BuildVerifier) runPipeline(ctx context.Context) []VerifyStep {
	var steps []VerifyStep

	// Detect project type
	projectType := bv.detectProjectType()

	switch projectType {
	case "node":
		steps = append(steps, bv.stepInstallDeps(ctx, "npm", "install", "--prefer-offline"))
		steps = append(steps, bv.stepSecurityAudit(ctx, projectType))
		steps = append(steps, bv.stepLint(ctx, projectType))
		steps = append(steps, bv.stepTypeCheck(ctx))
		steps = append(steps, bv.stepBuild(ctx, "npm", "run", "build"))
		steps = append(steps, bv.stepTest(ctx, "npm", "test", "--", "--passWithNoTests", "--watchAll=false"))
	case "go":
		steps = append(steps, bv.stepInstallDeps(ctx, "go", "mod", "download"))
		steps = append(steps, bv.stepSecurityAudit(ctx, projectType))
		steps = append(steps, bv.stepLint(ctx, projectType))
		steps = append(steps, bv.stepBuild(ctx, "go", "build", "./..."))
		steps = append(steps, bv.stepTest(ctx, "go", "test", "./...", "-v"))
	case "python":
		steps = append(steps, bv.stepInstallDeps(ctx, "pip", "install", "-r", "requirements.txt"))
		steps = append(steps, bv.stepSecurityAudit(ctx, projectType))
		steps = append(steps, bv.stepLint(ctx, projectType))
		steps = append(steps, bv.stepTypeCheck(ctx))
		steps = append(steps, bv.stepTest(ctx, "python", "-m", "pytest", "--tb=short"))
	case "rust":
		steps = append(steps, bv.stepInstallDeps(ctx, "cargo", "fetch"))
		steps = append(steps, bv.stepSecurityAudit(ctx, projectType))
		steps = append(steps, bv.stepLint(ctx, projectType))
		steps = append(steps, bv.stepBuild(ctx, "cargo", "build"))
		steps = append(steps, bv.stepTest(ctx, "cargo", "test"))
	case "java-maven":
		steps = append(steps, bv.stepInstallDeps(ctx, "mvn", "dependency:resolve", "-q"))
		steps = append(steps, bv.stepSecurityAudit(ctx, projectType))
		steps = append(steps, bv.stepBuild(ctx, "mvn", "compile", "-q"))
		steps = append(steps, bv.stepTest(ctx, "mvn", "test", "-q"))
	case "java-gradle":
		steps = append(steps, bv.stepInstallDeps(ctx, "gradle", "dependencies", "-q"))
		steps = append(steps, bv.stepSecurityAudit(ctx, projectType))
		steps = append(steps, bv.stepBuild(ctx, "gradle", "compileJava", "-q"))
		steps = append(steps, bv.stepTest(ctx, "gradle", "test", "-q"))
	case "dotnet":
		steps = append(steps, bv.stepInstallDeps(ctx, "dotnet", "restore"))
		steps = append(steps, bv.stepSecurityAudit(ctx, projectType))
		steps = append(steps, bv.stepBuild(ctx, "dotnet", "build", "--no-restore"))
		steps = append(steps, bv.stepTest(ctx, "dotnet", "test", "--no-build"))
	default:
		steps = append(steps, VerifyStep{
			Name:   "detect",
			Status: "skipped",
			Output: "Unknown project type — skipping build verification",
		})
	}

	return steps
}

// stepSecurityAudit runs security vulnerability scanning
func (bv *BuildVerifier) stepSecurityAudit(ctx context.Context, projectType string) VerifyStep {
	start := time.Now()

	switch projectType {
	case "node":
		// npm audit
		cmd := exec.CommandContext(ctx, "npm", "audit", "--json")
		cmd.Dir = bv.workDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			// npm audit returns non-zero for vulnerabilities found
			return VerifyStep{
				Name:     "security",
				Status:   "warning",
				Output:   truncateOutput(string(output), 1000),
				Duration: time.Since(start).Milliseconds(),
				Score:    10, // Partial score for running but finding issues
			}
		}
		return VerifyStep{
			Name:     "security",
			Status:   "passed",
			Output:   "No vulnerabilities found",
			Duration: time.Since(start).Milliseconds(),
			Score:    10,
		}

	case "go":
		// govulncheck if available
		cmd := exec.CommandContext(ctx, "govulncheck", "./...")
		cmd.Dir = bv.workDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			// Check if govulncheck is not installed
			if strings.Contains(string(output), "not found") || strings.Contains(string(output), "executable file not found") {
				return VerifyStep{
					Name:     "security",
					Status:   "skipped",
					Output:   "govulncheck not installed - run: go install golang.org/x/vuln/cmd/govulncheck@latest",
					Duration: time.Since(start).Milliseconds(),
				}
			}
			return VerifyStep{
				Name:     "security",
				Status:   "warning",
				Output:   truncateOutput(string(output), 1000),
				Duration: time.Since(start).Milliseconds(),
				Score:    10,
			}
		}
		return VerifyStep{
			Name:     "security",
			Status:   "passed",
			Output:   "No vulnerabilities found",
			Duration: time.Since(start).Milliseconds(),
			Score:    10,
		}

	case "python":
		// pip-audit or safety
		cmd := exec.CommandContext(ctx, "pip-audit", "--format=json")
		cmd.Dir = bv.workDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			if strings.Contains(string(output), "not found") {
				// Try safety as fallback
				cmd2 := exec.CommandContext(ctx, "safety", "check", "--json")
				cmd2.Dir = bv.workDir
				output, err = cmd2.CombinedOutput()
				if err != nil && strings.Contains(string(output), "not found") {
					return VerifyStep{
						Name:     "security",
						Status:   "skipped",
						Output:   "pip-audit/safety not installed - run: pip install pip-audit",
						Duration: time.Since(start).Milliseconds(),
					}
				}
			}
			return VerifyStep{
				Name:     "security",
				Status:   "warning",
				Output:   truncateOutput(string(output), 1000),
				Duration: time.Since(start).Milliseconds(),
				Score:    10,
			}
		}
		return VerifyStep{
			Name:     "security",
			Status:   "passed",
			Duration: time.Since(start).Milliseconds(),
			Score:    10,
		}

	case "rust":
		// cargo audit
		cmd := exec.CommandContext(ctx, "cargo", "audit")
		cmd.Dir = bv.workDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			if strings.Contains(string(output), "not found") {
				return VerifyStep{
					Name:     "security",
					Status:   "skipped",
					Output:   "cargo-audit not installed - run: cargo install cargo-audit",
					Duration: time.Since(start).Milliseconds(),
				}
			}
			return VerifyStep{
				Name:     "security",
				Status:   "warning",
				Output:   truncateOutput(string(output), 1000),
				Duration: time.Since(start).Milliseconds(),
				Score:    10,
			}
		}
		return VerifyStep{
			Name:     "security",
			Status:   "passed",
			Duration: time.Since(start).Milliseconds(),
			Score:    10,
		}

	default:
		return VerifyStep{
			Name:     "security",
			Status:   "skipped",
			Output:   "No security scanner available for this project type",
			Duration: time.Since(start).Milliseconds(),
		}
	}
}

func (bv *BuildVerifier) stepInstallDeps(ctx context.Context, command string, args ...string) VerifyStep {
	return bv.runStep(ctx, "install", command, args...)
}

func (bv *BuildVerifier) stepTypeCheck(ctx context.Context) VerifyStep {
	// Check for TypeScript
	if _, err := os.Stat(filepath.Join(bv.workDir, "tsconfig.json")); err == nil {
		return bv.runStep(ctx, "typecheck", "npx", "tsc", "--noEmit")
	}

	// Check for Python type hints with mypy
	projectType := bv.detectProjectType()
	if projectType == "python" {
		// Check if mypy is available
		cmd := exec.CommandContext(ctx, "python", "-m", "mypy", "--version")
		if err := cmd.Run(); err == nil {
			return bv.runStep(ctx, "typecheck", "python", "-m", "mypy", ".", "--ignore-missing-imports")
		}
		return VerifyStep{Name: "typecheck", Status: "skipped", Output: "mypy not installed - run: pip install mypy"}
	}

	return VerifyStep{Name: "typecheck", Status: "skipped", Output: "No type checking available for this project type"}
}

func (bv *BuildVerifier) stepLint(ctx context.Context, projectType string) VerifyStep {
	switch projectType {
	case "node":
		// Check if eslint config exists
		for _, name := range []string{".eslintrc.js", ".eslintrc.json", ".eslintrc", "eslint.config.js", "eslint.config.mjs"} {
			if _, err := os.Stat(filepath.Join(bv.workDir, name)); err == nil {
				return bv.runStep(ctx, "lint", "npx", "eslint", ".", "--max-warnings=0")
			}
		}
		return VerifyStep{Name: "lint", Status: "skipped", Output: "No ESLint config found"}
	case "go":
		// Try golangci-lint first, fall back to go vet
		cmd := exec.CommandContext(ctx, "golangci-lint", "run", "--timeout=2m")
		cmd.Dir = bv.workDir
		if output, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(output), "not found") {
			status := "passed"
			if err != nil {
				status = "failed"
			}
			return VerifyStep{
				Name:     "lint",
				Status:   status,
				Output:   truncateOutput(string(output), 2000),
				Duration: 0,
			}
		}
		return bv.runStep(ctx, "lint", "go", "vet", "./...")
	case "python":
		// Try ruff first (faster), fall back to flake8
		cmd := exec.CommandContext(ctx, "ruff", "check", ".")
		cmd.Dir = bv.workDir
		if output, err := cmd.CombinedOutput(); !strings.Contains(string(output), "not found") {
			status := "passed"
			if err != nil {
				status = "failed"
			}
			return VerifyStep{
				Name:     "lint",
				Status:   status,
				Output:   truncateOutput(string(output), 2000),
				Duration: 0,
			}
		}
		return bv.runStep(ctx, "lint", "python", "-m", "flake8", "--max-line-length=120")
	case "rust":
		return bv.runStep(ctx, "lint", "cargo", "clippy", "--", "-D", "warnings")
	case "java-maven":
		// checkstyle via maven if configured
		return VerifyStep{Name: "lint", Status: "skipped", Output: "Configure checkstyle plugin for Java linting"}
	case "java-gradle":
		return VerifyStep{Name: "lint", Status: "skipped", Output: "Configure checkstyle plugin for Java linting"}
	case "dotnet":
		return bv.runStep(ctx, "lint", "dotnet", "format", "--verify-no-changes")
	}
	return VerifyStep{Name: "lint", Status: "skipped"}
}

func (bv *BuildVerifier) stepBuild(ctx context.Context, command string, args ...string) VerifyStep {
	return bv.runStep(ctx, "build", command, args...)
}

func (bv *BuildVerifier) stepTest(ctx context.Context, command string, args ...string) VerifyStep {
	return bv.runStep(ctx, "test", command, args...)
}

// runStep executes a single command and returns its result
func (bv *BuildVerifier) runStep(ctx context.Context, name, command string, args ...string) VerifyStep {
	start := time.Now()

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = bv.workDir
	cmd.Env = append(os.Environ(), "CI=true", "NODE_ENV=test")

	output, err := cmd.CombinedOutput()
	duration := time.Since(start).Milliseconds()

	status := "passed"
	if err != nil {
		status = "failed"
	}

	return VerifyStep{
		Name:     name,
		Status:   status,
		Output:   truncateOutput(string(output), 2000),
		Duration: duration,
	}
}

// calculateScore awards points based on step results
func (bv *BuildVerifier) calculateScore(steps []VerifyStep) int {
	if len(steps) == 0 {
		return 0
	}

	// Scoring weights
	weights := map[string]int{
		"install":   10,
		"lint":      15,
		"typecheck": 15,
		"build":     35,
		"test":      25,
	}

	score := 0
	totalWeight := 0

	for _, step := range steps {
		weight, ok := weights[step.Name]
		if !ok {
			weight = 10
		}

		if step.Status == "skipped" {
			// Redistribute skipped weight proportionally
			continue
		}

		totalWeight += weight
		if step.Status == "passed" {
			score += weight
		}
	}

	if totalWeight == 0 {
		return 0
	}

	// Normalize to 0-100
	return (score * 100) / totalWeight
}

// attemptFixes uses AI to fix errors from failed steps
func (bv *BuildVerifier) attemptFixes(ctx context.Context, steps []VerifyStep) []AppliedFix {
	var fixes []AppliedFix

	for _, step := range steps {
		if step.Status != "failed" || step.Output == "" {
			continue
		}

		// Ask AI to diagnose and fix
		prompt := fmt.Sprintf(
			"The following build step '%s' failed with this output:\n\n%s\n\n"+
				"Analyze the errors and provide specific fixes. For each fix, output:\n"+
				"FILE: <filepath>\n"+
				"SEARCH: <exact text to find>\n"+
				"REPLACE: <replacement text>\n"+
				"---\n"+
				"Only output fixes, no explanations.",
			step.Name, step.Output,
		)

		response, err := bv.ai.Generate(ctx, prompt, AIOptions{MaxTokens: 2000})
		if err != nil {
			log.Printf("Build verifier: AI fix generation failed for step %s: %v", step.Name, err)
			continue
		}

		// Parse and apply fixes
		parsedFixes := bv.parseFixes(response)
		for _, fix := range parsedFixes {
			if bv.applyFix(fix) {
				fixes = append(fixes, AppliedFix{
					File:        fix.file,
					Description: fmt.Sprintf("Fixed %s error in %s", step.Name, fix.file),
				})
			}
		}
	}

	return fixes
}

type parsedFix struct {
	file    string
	search  string
	replace string
}

func (bv *BuildVerifier) parseFixes(response string) []parsedFix {
	var fixes []parsedFix

	blocks := strings.Split(response, "---")
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		fix := parsedFix{}
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "FILE: ") {
				fix.file = strings.TrimPrefix(line, "FILE: ")
			} else if strings.HasPrefix(line, "SEARCH: ") {
				fix.search = strings.TrimPrefix(line, "SEARCH: ")
			} else if strings.HasPrefix(line, "REPLACE: ") {
				fix.replace = strings.TrimPrefix(line, "REPLACE: ")
			}
		}

		if fix.file != "" && fix.search != "" {
			fixes = append(fixes, fix)
		}
	}

	return fixes
}

func (bv *BuildVerifier) applyFix(fix parsedFix) bool {
	filePath := filepath.Join(bv.workDir, fix.file)

	content, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	original := string(content)
	if !strings.Contains(original, fix.search) {
		return false
	}

	updated := strings.Replace(original, fix.search, fix.replace, 1)
	if err := os.WriteFile(filePath, []byte(updated), 0644); err != nil {
		return false
	}

	return true
}

func (bv *BuildVerifier) detectProjectType() string {
	if _, err := os.Stat(filepath.Join(bv.workDir, "package.json")); err == nil {
		return "node"
	}
	if _, err := os.Stat(filepath.Join(bv.workDir, "go.mod")); err == nil {
		return "go"
	}
	if _, err := os.Stat(filepath.Join(bv.workDir, "requirements.txt")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(bv.workDir, "pyproject.toml")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(bv.workDir, "setup.py")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(bv.workDir, "Cargo.toml")); err == nil {
		return "rust"
	}
	if _, err := os.Stat(filepath.Join(bv.workDir, "pom.xml")); err == nil {
		return "java-maven"
	}
	if _, err := os.Stat(filepath.Join(bv.workDir, "build.gradle")); err == nil {
		return "java-gradle"
	}
	if _, err := os.Stat(filepath.Join(bv.workDir, "build.gradle.kts")); err == nil {
		return "java-gradle"
	}
	// Check for .NET projects
	matches, _ := filepath.Glob(filepath.Join(bv.workDir, "*.csproj"))
	if len(matches) > 0 {
		return "dotnet"
	}
	matches, _ = filepath.Glob(filepath.Join(bv.workDir, "*.fsproj"))
	if len(matches) > 0 {
		return "dotnet"
	}
	return "unknown"
}

func (bv *BuildVerifier) buildSummary(result *VerificationResult) string {
	passed := 0
	failed := 0
	skipped := 0
	for _, s := range result.Steps {
		switch s.Status {
		case "passed":
			passed++
		case "failed":
			failed++
		case "skipped":
			skipped++
		}
	}

	status := "PASSED"
	if !result.Passed {
		status = "FAILED"
	}

	summary := fmt.Sprintf("Build verification %s (score: %d/100). %d passed, %d failed, %d skipped.",
		status, result.Score, passed, failed, skipped)

	if result.Retries > 0 {
		summary += fmt.Sprintf(" %d retry attempt(s).", result.Retries)
	}
	if len(result.FixesApplied) > 0 {
		summary += fmt.Sprintf(" %d auto-fix(es) applied.", len(result.FixesApplied))
	}

	return summary
}
