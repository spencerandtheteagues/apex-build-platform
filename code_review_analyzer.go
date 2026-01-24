package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// CodeReviewAnalyzer performs comprehensive code quality analysis
type CodeReviewAnalyzer struct {
	BaseDir     string
	Results     []ReviewResult
	FileSet     *token.FileSet
	TotalFiles  int
	TotalLines  int
	IssueCount  int
}

type ReviewResult struct {
	File         string
	Line         int
	Column       int
	Severity     string
	Category     string
	Issue        string
	Description  string
	Suggestion   string
}

type QualityMetrics struct {
	CyclomaticComplexity int
	CodeDuplication      int
	TestCoverage         float64
	DocumentationRatio   float64
	SecurityIssues       int
	PerformanceIssues    int
}

// NewCodeReviewAnalyzer creates a new code review analyzer
func NewCodeReviewAnalyzer(baseDir string) *CodeReviewAnalyzer {
	return &CodeReviewAnalyzer{
		BaseDir: baseDir,
		Results: make([]ReviewResult, 0),
		FileSet: token.NewFileSet(),
	}
}

// RunFullReview executes comprehensive code review
func (cra *CodeReviewAnalyzer) RunFullReview() {
	fmt.Println("🔍 APEX.BUILD Code Review Analysis Starting...")

	cra.AnalyzeGoCode()
	cra.AnalyzeTypeScriptCode()
	cra.AnalyzeSecurityPatterns()
	cra.AnalyzePerformancePatterns()
	cra.AnalyzeDependencies()
	cra.AnalyzeDocumentation()
	cra.AnalyzeTestCoverage()
	cra.CheckBestPractices()

	cra.GenerateReviewReport()
}

// AnalyzeGoCode analyzes Go backend code
func (cra *CodeReviewAnalyzer) AnalyzeGoCode() {
	err := filepath.Walk(filepath.Join(cra.BaseDir, "backend"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		cra.TotalFiles++
		cra.analyzeGoFile(path)
		return nil
	})

	if err != nil {
		cra.Results = append(cra.Results, ReviewResult{
			File:        "backend/",
			Severity:    "ERROR",
			Category:    "Analysis",
			Issue:       "Failed to analyze Go code",
			Description: err.Error(),
		})
	}
}

func (cra *CodeReviewAnalyzer) analyzeGoFile(filename string) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return
	}

	cra.TotalLines += len(strings.Split(string(src), "\n"))

	// Parse Go file
	file, err := parser.ParseFile(cra.FileSet, filename, src, parser.ParseComments)
	if err != nil {
		cra.Results = append(cra.Results, ReviewResult{
			File:        filename,
			Severity:    "ERROR",
			Category:    "Syntax",
			Issue:       "Parse error",
			Description: err.Error(),
			Suggestion:  "Fix syntax errors",
		})
		return
	}

	// Check for common Go issues
	cra.checkGoComplexity(file, filename)
	cra.checkGoSecurity(file, filename)
	cra.checkGoErrorHandling(file, filename)
	cra.checkGoNaming(file, filename)
	cra.checkGoDocumentation(file, filename)
}

func (cra *CodeReviewAnalyzer) checkGoComplexity(file *ast.File, filename string) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			if fn.Body != nil {
				complexity := cra.calculateCyclomaticComplexity(fn.Body)
				if complexity > 10 {
					pos := cra.FileSet.Position(fn.Pos())
					cra.Results = append(cra.Results, ReviewResult{
						File:        filename,
						Line:        pos.Line,
						Column:      pos.Column,
						Severity:    "HIGH",
						Category:    "Complexity",
						Issue:       fmt.Sprintf("High cyclomatic complexity: %d", complexity),
						Description: fmt.Sprintf("Function %s has complexity of %d (threshold: 10)", fn.Name.Name, complexity),
						Suggestion:  "Break down function into smaller, more focused functions",
					})
					cra.IssueCount++
				}
			}
		}
		return true
	})
}

func (cra *CodeReviewAnalyzer) calculateCyclomaticComplexity(block *ast.BlockStmt) int {
	complexity := 1 // Base complexity

	ast.Inspect(block, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.RangeStmt, *ast.ForStmt, *ast.TypeSwitchStmt, *ast.SwitchStmt:
			complexity++
		case *ast.CaseClause:
			complexity++
		}
		return true
	})

	return complexity
}

func (cra *CodeReviewAnalyzer) checkGoSecurity(file *ast.File, filename string) {
	// Check for potential security issues
	securityPatterns := []struct {
		pattern     string
		severity    string
		issue       string
		description string
		suggestion  string
	}{
		{
			pattern:     `sql\.Query\(.*\+.*\)`,
			severity:    "CRITICAL",
			issue:       "Potential SQL injection",
			description: "SQL query uses string concatenation which may allow injection",
			suggestion:  "Use parameterized queries with prepared statements",
		},
		{
			pattern:     `exec\.Command\(.*\+.*\)`,
			severity:    "HIGH",
			issue:       "Command injection risk",
			description: "Command execution uses string concatenation",
			suggestion:  "Validate and sanitize command inputs",
		},
		{
			pattern:     `http\.Get\(.*\+.*\)`,
			severity:    "MEDIUM",
			issue:       "URL injection risk",
			description: "HTTP request URL uses string concatenation",
			suggestion:  "Validate and encode URL components",
		},
	}

	content, _ := os.ReadFile(filename)
	lines := strings.Split(string(content), "\n")

	for i, line := range lines {
		for _, pattern := range securityPatterns {
			matched, _ := regexp.MatchString(pattern.pattern, line)
			if matched {
				cra.Results = append(cra.Results, ReviewResult{
					File:        filename,
					Line:        i + 1,
					Severity:    pattern.severity,
					Category:    "Security",
					Issue:       pattern.issue,
					Description: pattern.description,
					Suggestion:  pattern.suggestion,
				})
				cra.IssueCount++
			}
		}
	}
}

func (cra *CodeReviewAnalyzer) checkGoErrorHandling(file *ast.File, filename string) {
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			// Check for ignored errors
			if ident, ok := call.Fun.(*ast.Ident); ok {
				// Look for function calls that return errors but are not handled
				if strings.Contains(ident.Name, "Error") || strings.HasSuffix(ident.Name, "Err") {
					pos := cra.FileSet.Position(call.Pos())
					cra.Results = append(cra.Results, ReviewResult{
						File:        filename,
						Line:        pos.Line,
						Column:      pos.Column,
						Severity:    "MEDIUM",
						Category:    "Error Handling",
						Issue:       "Potential ignored error",
						Description: "Function call may return error that should be handled",
						Suggestion:  "Add proper error handling and logging",
					})
					cra.IssueCount++
				}
			}
		}
		return true
	})
}

func (cra *CodeReviewAnalyzer) checkGoNaming(file *ast.File, filename string) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			if node.Name.IsExported() && !isCapitalized(node.Name.Name) {
				pos := cra.FileSet.Position(node.Pos())
				cra.Results = append(cra.Results, ReviewResult{
					File:        filename,
					Line:        pos.Line,
					Column:      pos.Column,
					Severity:    "LOW",
					Category:    "Naming",
					Issue:       "Exported function should be capitalized",
					Description: fmt.Sprintf("Function %s is exported but not capitalized", node.Name.Name),
					Suggestion:  "Use proper Go naming conventions for exported functions",
				})
				cra.IssueCount++
			}
		case *ast.GenDecl:
			if node.Tok == token.VAR || node.Tok == token.CONST {
				for _, spec := range node.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range valueSpec.Names {
							if name.IsExported() && !isCapitalized(name.Name) {
								pos := cra.FileSet.Position(name.Pos())
								cra.Results = append(cra.Results, ReviewResult{
									File:        filename,
									Line:        pos.Line,
									Column:      pos.Column,
									Severity:    "LOW",
									Category:    "Naming",
									Issue:       "Exported variable should be capitalized",
									Description: fmt.Sprintf("Variable %s is exported but not capitalized", name.Name),
									Suggestion:  "Use proper Go naming conventions for exported variables",
								})
								cra.IssueCount++
							}
						}
					}
				}
			}
		}
		return true
	})
}

func (cra *CodeReviewAnalyzer) checkGoDocumentation(file *ast.File, filename string) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			if node.Name.IsExported() && node.Doc == nil {
				pos := cra.FileSet.Position(node.Pos())
				cra.Results = append(cra.Results, ReviewResult{
					File:        filename,
					Line:        pos.Line,
					Column:      pos.Column,
					Severity:    "MEDIUM",
					Category:    "Documentation",
					Issue:       "Missing documentation for exported function",
					Description: fmt.Sprintf("Function %s is exported but lacks documentation", node.Name.Name),
					Suggestion:  "Add documentation comment starting with function name",
				})
				cra.IssueCount++
			}
		case *ast.GenDecl:
			if node.Tok == token.TYPE {
				for _, spec := range node.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if typeSpec.Name.IsExported() && node.Doc == nil {
							pos := cra.FileSet.Position(typeSpec.Pos())
							cra.Results = append(cra.Results, ReviewResult{
								File:        filename,
								Line:        pos.Line,
								Column:      pos.Column,
								Severity:    "MEDIUM",
								Category:    "Documentation",
								Issue:       "Missing documentation for exported type",
								Description: fmt.Sprintf("Type %s is exported but lacks documentation", typeSpec.Name.Name),
								Suggestion:  "Add documentation comment starting with type name",
							})
							cra.IssueCount++
						}
					}
				}
			}
		}
		return true
	})
}

// AnalyzeTypeScriptCode analyzes frontend TypeScript code
func (cra *CodeReviewAnalyzer) AnalyzeTypeScriptCode() {
	frontendPath := filepath.Join(cra.BaseDir, "frontend", "src")
	if _, err := os.Stat(frontendPath); os.IsNotExist(err) {
		return
	}

	err := filepath.Walk(frontendPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx") {
			cra.TotalFiles++
			cra.analyzeTypeScriptFile(path)
		}
		return nil
	})

	if err != nil {
		cra.Results = append(cra.Results, ReviewResult{
			File:        "frontend/",
			Severity:    "ERROR",
			Category:    "Analysis",
			Issue:       "Failed to analyze TypeScript code",
			Description: err.Error(),
		})
	}
}

func (cra *CodeReviewAnalyzer) analyzeTypeScriptFile(filename string) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")
	cra.TotalLines += len(lines)

	// Check TypeScript-specific issues
	for i, line := range lines {
		// Check for 'any' type usage
		if strings.Contains(line, ": any") || strings.Contains(line, "as any") {
			cra.Results = append(cra.Results, ReviewResult{
				File:        filename,
				Line:        i + 1,
				Severity:    "MEDIUM",
				Category:    "Type Safety",
				Issue:       "Usage of 'any' type",
				Description: "Using 'any' type defeats TypeScript's type checking",
				Suggestion:  "Use specific types or interfaces instead of 'any'",
			})
			cra.IssueCount++
		}

		// Check for console.log in production code
		if strings.Contains(line, "console.log") && !strings.Contains(filename, "test") {
			cra.Results = append(cra.Results, ReviewResult{
				File:        filename,
				Line:        i + 1,
				Severity:    "LOW",
				Category:    "Code Quality",
				Issue:       "Console.log in production code",
				Description: "Console.log statements should not be in production code",
				Suggestion:  "Remove console.log or use proper logging library",
			})
			cra.IssueCount++
		}

		// Check for TODO/FIXME comments
		if strings.Contains(strings.ToUpper(line), "TODO") || strings.Contains(strings.ToUpper(line), "FIXME") {
			cra.Results = append(cra.Results, ReviewResult{
				File:        filename,
				Line:        i + 1,
				Severity:    "LOW",
				Category:    "Technical Debt",
				Issue:       "Unresolved TODO/FIXME comment",
				Description: "Code contains unresolved TODO or FIXME comment",
				Suggestion:  "Resolve the TODO/FIXME or create a proper issue",
			})
		}
	}
}

// AnalyzeSecurityPatterns analyzes for security vulnerabilities
func (cra *CodeReviewAnalyzer) AnalyzeSecurityPatterns() {
	securityChecks := []struct {
		pattern     string
		severity    string
		issue       string
		description string
	}{
		{`password.*=.*["'].*["']`, "CRITICAL", "Hardcoded password", "Password appears to be hardcoded in source"},
		{`api[_-]?key.*=.*["'].*["']`, "CRITICAL", "Hardcoded API key", "API key appears to be hardcoded in source"},
		{`secret.*=.*["'].*["']`, "HIGH", "Hardcoded secret", "Secret value appears to be hardcoded in source"},
		{`eval\(`, "HIGH", "Use of eval()", "eval() usage can lead to code injection vulnerabilities"},
		{`innerHTML\s*=`, "MEDIUM", "Use of innerHTML", "innerHTML usage may lead to XSS vulnerabilities"},
		{`document\.write\(`, "MEDIUM", "Use of document.write", "document.write() can be dangerous and is deprecated"},
	}

	err := filepath.Walk(cra.BaseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip binary files and directories
		if info.IsDir() || strings.Contains(path, "node_modules") || strings.Contains(path, ".git") {
			return nil
		}

		// Check relevant file types
		ext := filepath.Ext(path)
		if ext == ".go" || ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx" {
			cra.checkFileForSecurityPatterns(path, securityChecks)
		}

		return nil
	})

	if err != nil {
		cra.Results = append(cra.Results, ReviewResult{
			File:        cra.BaseDir,
			Severity:    "ERROR",
			Category:    "Security Analysis",
			Issue:       "Failed to analyze security patterns",
			Description: err.Error(),
		})
	}
}

func (cra *CodeReviewAnalyzer) checkFileForSecurityPatterns(filename string, patterns []struct {
	pattern     string
	severity    string
	issue       string
	description string
}) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")

	for i, line := range lines {
		for _, pattern := range patterns {
			matched, _ := regexp.MatchString(pattern.pattern, strings.ToLower(line))
			if matched {
				cra.Results = append(cra.Results, ReviewResult{
					File:        filename,
					Line:        i + 1,
					Severity:    pattern.severity,
					Category:    "Security",
					Issue:       pattern.issue,
					Description: pattern.description,
					Suggestion:  "Move sensitive values to environment variables or secure configuration",
				})
				cra.IssueCount++
			}
		}
	}
}

// AnalyzePerformancePatterns analyzes for performance issues
func (cra *CodeReviewAnalyzer) AnalyzePerformancePatterns() {
	performanceChecks := []struct {
		pattern     string
		severity    string
		issue       string
		description string
		suggestion  string
	}{
		{
			pattern:     `for.*range.*{[\s\S]*append\(`,
			severity:    "MEDIUM",
			issue:       "Inefficient append in loop",
			description: "Using append in a loop without pre-allocation can cause performance issues",
			suggestion:  "Pre-allocate slice with make() if size is known",
		},
		{
			pattern:     `fmt\.Sprintf.*\+`,
			severity:    "LOW",
			issue:       "String concatenation with Sprintf",
			description: "Using fmt.Sprintf with string concatenation is inefficient",
			suggestion:  "Use strings.Builder or direct Sprintf formatting",
		},
		{
			pattern:     `regexp\.MustCompile\(.*\)`,
			severity:    "MEDIUM",
			issue:       "Regex compilation in loop/function",
			description: "Regular expression compilation should be done once, not repeatedly",
			suggestion:  "Move regex compilation to package level or use sync.Once",
		},
	}

	err := filepath.Walk(filepath.Join(cra.BaseDir, "backend"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.HasSuffix(path, ".go") {
			cra.checkFileForPerformancePatterns(path, performanceChecks)
		}
		return nil
	})

	if err != nil {
		cra.Results = append(cra.Results, ReviewResult{
			File:        "backend/",
			Severity:    "ERROR",
			Category:    "Performance Analysis",
			Issue:       "Failed to analyze performance patterns",
			Description: err.Error(),
		})
	}
}

func (cra *CodeReviewAnalyzer) checkFileForPerformancePatterns(filename string, patterns []struct {
	pattern     string
	severity    string
	issue       string
	description string
	suggestion  string
}) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return
	}

	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern.pattern)
		if err != nil {
			continue
		}

		matches := re.FindAllStringIndex(string(content), -1)
		for _, match := range matches {
			// Calculate line number
			lineNum := 1 + strings.Count(string(content[:match[0]]), "\n")

			cra.Results = append(cra.Results, ReviewResult{
				File:        filename,
				Line:        lineNum,
				Severity:    pattern.severity,
				Category:    "Performance",
				Issue:       pattern.issue,
				Description: pattern.description,
				Suggestion:  pattern.suggestion,
			})
			cra.IssueCount++
		}
	}
}

// AnalyzeDependencies checks for dependency security and updates
func (cra *CodeReviewAnalyzer) AnalyzeDependencies() {
	// Check Go dependencies
	goModPath := filepath.Join(cra.BaseDir, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		cra.analyzeGoMod(goModPath)
	}

	// Check Node.js dependencies
	packageJsonPath := filepath.Join(cra.BaseDir, "frontend", "package.json")
	if _, err := os.Stat(packageJsonPath); err == nil {
		cra.analyzePackageJson(packageJsonPath)
	}
}

func (cra *CodeReviewAnalyzer) analyzeGoMod(filename string) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		// Check for old Go versions
		if strings.Contains(line, "go ") && (strings.Contains(line, "1.1") || strings.Contains(line, "1.2")) {
			cra.Results = append(cra.Results, ReviewResult{
				File:        filename,
				Line:        i + 1,
				Severity:    "HIGH",
				Category:    "Dependencies",
				Issue:       "Outdated Go version",
				Description: "Using outdated Go version with known security issues",
				Suggestion:  "Upgrade to latest stable Go version",
			})
			cra.IssueCount++
		}

		// Check for known vulnerable packages (simplified check)
		vulnerablePackages := []string{
			"github.com/dgrijalva/jwt-go", // Known vulnerability, use github.com/golang-jwt/jwt instead
		}

		for _, pkg := range vulnerablePackages {
			if strings.Contains(line, pkg) {
				cra.Results = append(cra.Results, ReviewResult{
					File:        filename,
					Line:        i + 1,
					Severity:    "HIGH",
					Category:    "Dependencies",
					Issue:       "Vulnerable dependency",
					Description: fmt.Sprintf("Package %s has known security vulnerabilities", pkg),
					Suggestion:  "Update to secure alternative package",
				})
				cra.IssueCount++
			}
		}
	}
}

func (cra *CodeReviewAnalyzer) analyzePackageJson(filename string) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return
	}

	// Simple check for common vulnerable packages
	vulnerablePackages := []string{
		"lodash", // Often has vulnerabilities in older versions
		"jquery", // Often has XSS vulnerabilities
		"moment", // Deprecated, should use date-fns or dayjs
	}

	for _, pkg := range vulnerablePackages {
		if strings.Contains(string(content), `"`+pkg+`"`) {
			cra.Results = append(cra.Results, ReviewResult{
				File:        filename,
				Severity:    "MEDIUM",
				Category:    "Dependencies",
				Issue:       "Potentially vulnerable dependency",
				Description: fmt.Sprintf("Package %s may have security issues", pkg),
				Suggestion:  "Review and update to latest version or secure alternative",
			})
			cra.IssueCount++
		}
	}
}

// AnalyzeDocumentation checks documentation coverage
func (cra *CodeReviewAnalyzer) AnalyzeDocumentation() {
	// Check for README files
	requiredDocs := []string{"README.md", "API.md", "CONTRIBUTING.md", "SECURITY.md"}

	for _, doc := range requiredDocs {
		docPath := filepath.Join(cra.BaseDir, doc)
		if _, err := os.Stat(docPath); os.IsNotExist(err) {
			cra.Results = append(cra.Results, ReviewResult{
				File:        cra.BaseDir,
				Severity:    "MEDIUM",
				Category:    "Documentation",
				Issue:       fmt.Sprintf("Missing %s", doc),
				Description: fmt.Sprintf("Project lacks %s documentation", doc),
				Suggestion:  fmt.Sprintf("Create %s with project information", doc),
			})
			cra.IssueCount++
		}
	}
}

// AnalyzeTestCoverage checks test coverage
func (cra *CodeReviewAnalyzer) AnalyzeTestCoverage() {
	// Count test files vs source files
	testFiles := 0
	sourceFiles := 0

	err := filepath.Walk(cra.BaseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.Contains(path, "node_modules") || strings.Contains(path, ".git") {
			return nil
		}

		if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".test.ts") ||
		   strings.HasSuffix(path, ".test.tsx") || strings.HasSuffix(path, ".spec.ts") {
			testFiles++
		} else if strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx") {
			sourceFiles++
		}

		return nil
	})

	if err == nil {
		testCoverage := float64(testFiles) / float64(sourceFiles) * 100

		if testCoverage < 50 {
			cra.Results = append(cra.Results, ReviewResult{
				File:        cra.BaseDir,
				Severity:    "HIGH",
				Category:    "Test Coverage",
				Issue:       "Low test coverage",
				Description: fmt.Sprintf("Test coverage is %.1f%% (target: 80%%)", testCoverage),
				Suggestion:  "Add more unit tests and integration tests",
			})
			cra.IssueCount++
		}
	}
}

// CheckBestPractices checks for general best practices
func (cra *CodeReviewAnalyzer) CheckBestPractices() {
	// Check for .env files in repository
	envFiles := []string{".env", ".env.local", ".env.development", ".env.production"}

	for _, envFile := range envFiles {
		envPath := filepath.Join(cra.BaseDir, envFile)
		if _, err := os.Stat(envPath); err == nil {
			cra.Results = append(cra.Results, ReviewResult{
				File:        envPath,
				Severity:    "HIGH",
				Category:    "Security",
				Issue:       "Environment file in repository",
				Description: "Environment files should not be committed to repository",
				Suggestion:  "Add to .gitignore and use .env.example template instead",
			})
			cra.IssueCount++
		}
	}

	// Check for proper .gitignore
	gitignorePath := filepath.Join(cra.BaseDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		cra.Results = append(cra.Results, ReviewResult{
			File:        cra.BaseDir,
			Severity:    "MEDIUM",
			Category:    "Best Practices",
			Issue:       "Missing .gitignore",
			Description: "Project lacks .gitignore file",
			Suggestion:  "Add .gitignore to exclude build artifacts, dependencies, and sensitive files",
		})
		cra.IssueCount++
	}
}

// GenerateReviewReport generates comprehensive review report
func (cra *CodeReviewAnalyzer) GenerateReviewReport() {
	fmt.Println("\n🔍 APEX.BUILD CODE REVIEW REPORT")
	fmt.Println("==================================")

	// Sort results by severity
	sort.Slice(cra.Results, func(i, j int) bool {
		severityOrder := map[string]int{
			"CRITICAL": 4,
			"HIGH":     3,
			"MEDIUM":   2,
			"LOW":      1,
			"INFO":     0,
		}
		return severityOrder[cra.Results[i].Severity] > severityOrder[cra.Results[j].Severity]
	})

	// Count by severity and category
	severityCounts := make(map[string]int)
	categoryCounts := make(map[string]int)

	for _, result := range cra.Results {
		severityCounts[result.Severity]++
		categoryCounts[result.Category]++
	}

	// Display summary
	fmt.Printf("\n📊 REVIEW SUMMARY\n")
	fmt.Printf("================\n")
	fmt.Printf("Total Files Analyzed: %d\n", cra.TotalFiles)
	fmt.Printf("Total Lines of Code: %d\n", cra.TotalLines)
	fmt.Printf("Total Issues Found: %d\n", cra.IssueCount)

	fmt.Printf("\n🚨 ISSUES BY SEVERITY\n")
	fmt.Printf("====================\n")
	fmt.Printf("Critical: %d\n", severityCounts["CRITICAL"])
	fmt.Printf("High:     %d\n", severityCounts["HIGH"])
	fmt.Printf("Medium:   %d\n", severityCounts["MEDIUM"])
	fmt.Printf("Low:      %d\n", severityCounts["LOW"])

	fmt.Printf("\n📋 ISSUES BY CATEGORY\n")
	fmt.Printf("====================\n")
	for category, count := range categoryCounts {
		fmt.Printf("%-15s: %d\n", category, count)
	}

	// Display detailed issues
	fmt.Printf("\n🔍 DETAILED FINDINGS\n")
	fmt.Printf("===================\n")

	currentSeverity := ""
	for _, result := range cra.Results {
		if result.Severity != currentSeverity {
			currentSeverity = result.Severity
			fmt.Printf("\n--- %s SEVERITY ISSUES ---\n", currentSeverity)
		}

		fmt.Printf("\n[%s] %s\n", result.Category, result.Issue)
		fmt.Printf("  File: %s", result.File)
		if result.Line > 0 {
			fmt.Printf(":%d", result.Line)
		}
		if result.Column > 0 {
			fmt.Printf(":%d", result.Column)
		}
		fmt.Printf("\n")
		fmt.Printf("  Description: %s\n", result.Description)
		if result.Suggestion != "" {
			fmt.Printf("  Suggestion: %s\n", result.Suggestion)
		}
	}

	// Code quality score
	qualityScore := cra.calculateQualityScore()
	fmt.Printf("\n📊 CODE QUALITY SCORE\n")
	fmt.Printf("====================\n")
	fmt.Printf("Overall Score: %.1f/100\n", qualityScore)

	if qualityScore >= 90 {
		fmt.Printf("✅ EXCELLENT: Code quality is excellent\n")
	} else if qualityScore >= 80 {
		fmt.Printf("✅ GOOD: Code quality is good with minor issues\n")
	} else if qualityScore >= 70 {
		fmt.Printf("⚠️  FAIR: Code quality needs improvement\n")
	} else if qualityScore >= 60 {
		fmt.Printf("⚠️  POOR: Code quality has significant issues\n")
	} else {
		fmt.Printf("🚨 CRITICAL: Code quality requires immediate attention\n")
	}

	// Recommendations
	fmt.Printf("\n🎯 TOP RECOMMENDATIONS\n")
	fmt.Printf("======================\n")
	cra.generateRecommendations()
}

func (cra *CodeReviewAnalyzer) calculateQualityScore() float64 {
	if cra.TotalFiles == 0 {
		return 100.0
	}

	// Base score
	score := 100.0

	// Deduct points for issues
	score -= float64(severityCounts["CRITICAL"]) * 20.0
	score -= float64(severityCounts["HIGH"]) * 10.0
	score -= float64(severityCounts["MEDIUM"]) * 5.0
	score -= float64(severityCounts["LOW"]) * 2.0

	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}

	return score
}

var severityCounts = make(map[string]int)

func (cra *CodeReviewAnalyzer) generateRecommendations() {
	recommendations := []string{
		"1. Address all CRITICAL and HIGH severity security issues immediately",
		"2. Implement comprehensive unit tests to increase coverage above 80%",
		"3. Add proper error handling and logging throughout the application",
		"4. Document all exported functions and types",
		"5. Review and update dependencies for security vulnerabilities",
		"6. Implement proper input validation and sanitization",
		"7. Add security headers and CSRF protection",
		"8. Use environment variables for all configuration",
		"9. Implement proper logging and monitoring",
		"10. Add integration tests for critical user flows",
	}

	for _, rec := range recommendations {
		fmt.Printf("  %s\n", rec)
	}
}

// Helper functions
func isCapitalized(name string) bool {
	return len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
}

func main() {
	analyzer := NewCodeReviewAnalyzer("/Users/spencerteague/apex-build")
	analyzer.RunFullReview()
}