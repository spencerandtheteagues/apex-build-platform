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

	// Check if package.json exists
	packageJSONPath := filepath.Join(v.workDir, "package.json")
	if _, err := os.Stat(packageJSONPath); os.IsNotExist(err) {
		return issues // Not a Node.js project, skip
	}

	// Check if node_modules exists (deps installed)
	nodeModulesPath := filepath.Join(v.workDir, "node_modules")
	if _, err := os.Stat(nodeModulesPath); os.IsNotExist(err) {
		// Try installing deps first
		installCmd := exec.CommandContext(ctx, "npm", "install", "--prefer-offline")
		installCmd.Dir = v.workDir
		if output, err := installCmd.CombinedOutput(); err != nil {
			issues = append(issues, ValidationIssue{
				Type:        IssueSyntaxError,
				Severity:    SeverityCritical,
				File:        "package.json",
				Description: fmt.Sprintf("npm install failed: %s", string(output)),
			})
			return issues
		}
	}

	// Run TypeScript type check
	tscCmd := exec.CommandContext(ctx, "npx", "tsc", "--noEmit")
	tscCmd.Dir = v.workDir
	if output, err := tscCmd.CombinedOutput(); err != nil {
		issues = append(issues, ValidationIssue{
			Type:        IssueSyntaxError,
			Severity:    SeverityWarning,
			File:        "tsconfig.json",
			Description: fmt.Sprintf("TypeScript type check found errors: %s", truncateOutput(string(output), 500)),
		})
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
