// Package codereview provides AI-powered code review functionality
package codereview

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"apex-build/internal/ai"
)

// ReviewFinding represents a single code review finding
type ReviewFinding struct {
	Type       string `json:"type"`        // bug, security, performance, style, best_practice
	Severity   string `json:"severity"`    // error, warning, info, hint
	Line       int    `json:"line"`        // Starting line number (1-indexed)
	EndLine    int    `json:"end_line"`    // Ending line number (optional)
	Column     int    `json:"column"`      // Starting column (optional)
	EndColumn  int    `json:"end_column"`  // Ending column (optional)
	Message    string `json:"message"`     // Human-readable description
	Suggestion string `json:"suggestion"`  // Suggested fix or improvement
	Code       string `json:"code"`        // Code snippet if relevant
	RuleID     string `json:"rule_id"`     // Identifier for the rule that triggered this
}

// ReviewRequest represents a code review request
type ReviewRequest struct {
	Code       string            `json:"code" binding:"required"`
	Language   string            `json:"language" binding:"required"`
	FileName   string            `json:"file_name"`
	Context    string            `json:"context"`     // Surrounding code or project context
	Focus      []string          `json:"focus"`       // Specific areas to focus on: security, performance, etc.
	Severity   string            `json:"severity"`    // Minimum severity to report: error, warning, info, hint
	MaxResults int               `json:"max_results"` // Maximum number of findings
	Options    map[string]string `json:"options"`     // Additional options
}

// ReviewResponse represents the code review results
type ReviewResponse struct {
	Findings    []ReviewFinding   `json:"findings"`
	Summary     string            `json:"summary"`
	Score       int               `json:"score"`        // 0-100 quality score
	Metrics     ReviewMetrics     `json:"metrics"`
	Suggestions []string          `json:"suggestions"`  // General improvement suggestions
	ReviewedAt  time.Time         `json:"reviewed_at"`
	Duration    int64             `json:"duration_ms"`  // Review duration in milliseconds
}

// ReviewMetrics provides quantitative analysis
type ReviewMetrics struct {
	TotalLines     int `json:"total_lines"`
	CodeLines      int `json:"code_lines"`
	CommentLines   int `json:"comment_lines"`
	BlankLines     int `json:"blank_lines"`
	Complexity     int `json:"complexity"`      // Cyclomatic complexity estimate
	SecurityIssues int `json:"security_issues"`
	BugRisks       int `json:"bug_risks"`
	StyleIssues    int `json:"style_issues"`
}

// CodeReviewService provides AI-powered code review
type CodeReviewService struct {
	aiRouter      *ai.AIRouter
	buildVerifier *BuildVerifier
	projectDir    string
}

// NewCodeReviewService creates a new code review service
func NewCodeReviewService(aiRouter *ai.AIRouter) *CodeReviewService {
	return &CodeReviewService{
		aiRouter: aiRouter,
	}
}

// NewCodeReviewServiceWithBuild creates a service with build verification enabled
func NewCodeReviewServiceWithBuild(aiRouter *ai.AIRouter, projectDir string) *CodeReviewService {
	return &CodeReviewService{
		aiRouter:      aiRouter,
		buildVerifier: NewBuildVerifier(projectDir),
		projectDir:    projectDir,
	}
}

// Review performs an AI-powered code review
func (s *CodeReviewService) Review(ctx context.Context, req ReviewRequest) (*ReviewResponse, error) {
	startTime := time.Now()

	// Calculate basic metrics
	metrics := s.calculateMetrics(req.Code)

	// Determine focus areas
	focusAreas := req.Focus
	if len(focusAreas) == 0 {
		focusAreas = []string{"bugs", "security", "performance", "best_practices", "readability"}
	}

	// Build the review prompt
	prompt := s.buildReviewPrompt(req, focusAreas)

	// Get AI review
	aiResponse, err := s.aiRouter.Generate(ctx, &ai.AIRequest{
		Capability: ai.CapabilityCodeReview,
		Prompt:     prompt,
		Code:       req.Code,
		Language:   req.Language,
		Context: map[string]interface{}{
			"file_name": req.FileName,
			"focus":     focusAreas,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("AI review failed: %w", err)
	}

	// Parse AI response into findings
	findings, summary, score := s.parseReviewResponse(aiResponse.Content, req)

	// Filter by severity if specified
	if req.Severity != "" {
		findings = s.filterBySeverity(findings, req.Severity)
	}

	// Limit results if specified
	if req.MaxResults > 0 && len(findings) > req.MaxResults {
		findings = findings[:req.MaxResults]
	}

	// Update metrics with findings count
	metrics.SecurityIssues = s.countBySeverityType(findings, "security")
	metrics.BugRisks = s.countBySeverityType(findings, "bug")
	metrics.StyleIssues = s.countBySeverityType(findings, "style")

	// Generate improvement suggestions
	suggestions := s.generateSuggestions(findings, metrics)

	duration := time.Since(startTime).Milliseconds()

	return &ReviewResponse{
		Findings:    findings,
		Summary:     summary,
		Score:       score,
		Metrics:     metrics,
		Suggestions: suggestions,
		ReviewedAt:  time.Now(),
		Duration:    duration,
	}, nil
}

// buildReviewPrompt creates the AI prompt for code review
func (s *CodeReviewService) buildReviewPrompt(req ReviewRequest, focusAreas []string) string {
	var sb strings.Builder

	sb.WriteString("You are an expert code reviewer. Analyze the following ")
	sb.WriteString(req.Language)
	sb.WriteString(" code and provide a detailed review.\n\n")

	if req.FileName != "" {
		sb.WriteString("File: ")
		sb.WriteString(req.FileName)
		sb.WriteString("\n\n")
	}

	if req.Context != "" {
		sb.WriteString("Context: ")
		sb.WriteString(req.Context)
		sb.WriteString("\n\n")
	}

	sb.WriteString("Focus areas: ")
	sb.WriteString(strings.Join(focusAreas, ", "))
	sb.WriteString("\n\n")

	sb.WriteString(`Respond with a JSON object containing:
{
  "findings": [
    {
      "type": "bug|security|performance|style|best_practice",
      "severity": "error|warning|info|hint",
      "line": <line number>,
      "end_line": <optional end line>,
      "message": "<description of the issue>",
      "suggestion": "<how to fix it>",
      "rule_id": "<short identifier>"
    }
  ],
  "summary": "<2-3 sentence summary of code quality>",
  "score": <0-100 quality score>
}

Be thorough but prioritize the most important issues. Include line numbers where issues occur.
Only output valid JSON, no additional text.

Code to review:
`)

	return sb.String()
}

// parseReviewResponse parses the AI response into structured findings
func (s *CodeReviewService) parseReviewResponse(content string, req ReviewRequest) ([]ReviewFinding, string, int) {
	var findings []ReviewFinding
	var summary string
	score := 70 // Default score

	// Try to parse as JSON
	type AIResponse struct {
		Findings []ReviewFinding `json:"findings"`
		Summary  string          `json:"summary"`
		Score    int             `json:"score"`
	}

	// Clean the response - remove markdown code blocks if present
	cleanContent := content
	if strings.HasPrefix(cleanContent, "```json") {
		cleanContent = strings.TrimPrefix(cleanContent, "```json")
		cleanContent = strings.TrimSuffix(cleanContent, "```")
	}
	if strings.HasPrefix(cleanContent, "```") {
		cleanContent = strings.TrimPrefix(cleanContent, "```")
		cleanContent = strings.TrimSuffix(cleanContent, "```")
	}
	cleanContent = strings.TrimSpace(cleanContent)

	var parsed AIResponse
	if err := json.Unmarshal([]byte(cleanContent), &parsed); err != nil {
		// If JSON parsing fails, try to extract findings manually
		findings = s.extractFindingsFromText(content, req)
		summary = "Code review completed. Some issues were identified."
	} else {
		findings = parsed.Findings
		summary = parsed.Summary
		if parsed.Score > 0 {
			score = parsed.Score
		}
	}

	// Ensure all findings have valid data
	for i := range findings {
		if findings[i].Type == "" {
			findings[i].Type = "style"
		}
		if findings[i].Severity == "" {
			findings[i].Severity = "info"
		}
		if findings[i].RuleID == "" {
			findings[i].RuleID = fmt.Sprintf("%s-%d", findings[i].Type, i+1)
		}
	}

	return findings, summary, score
}

// extractFindingsFromText attempts to extract findings from unstructured text
func (s *CodeReviewService) extractFindingsFromText(content string, req ReviewRequest) []ReviewFinding {
	var findings []ReviewFinding

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for patterns like "Line X:" or "Error:" etc.
		finding := ReviewFinding{
			Type:     "style",
			Severity: "info",
			Message:  line,
		}

		if strings.Contains(strings.ToLower(line), "error") || strings.Contains(strings.ToLower(line), "bug") {
			finding.Severity = "error"
			finding.Type = "bug"
		} else if strings.Contains(strings.ToLower(line), "warning") {
			finding.Severity = "warning"
		} else if strings.Contains(strings.ToLower(line), "security") {
			finding.Type = "security"
			finding.Severity = "error"
		} else if strings.Contains(strings.ToLower(line), "performance") {
			finding.Type = "performance"
			finding.Severity = "warning"
		}

		if len(findings) < 20 { // Limit extraction
			findings = append(findings, finding)
		}
	}

	return findings
}

// calculateMetrics calculates basic code metrics
func (s *CodeReviewService) calculateMetrics(code string) ReviewMetrics {
	lines := strings.Split(code, "\n")
	metrics := ReviewMetrics{
		TotalLines: len(lines),
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			metrics.BlankLines++
		} else if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			metrics.CommentLines++
		} else {
			metrics.CodeLines++
		}
	}

	// Estimate complexity based on control flow keywords
	code_lower := strings.ToLower(code)
	metrics.Complexity = strings.Count(code_lower, "if ") +
		strings.Count(code_lower, "for ") +
		strings.Count(code_lower, "while ") +
		strings.Count(code_lower, "switch ") +
		strings.Count(code_lower, "case ") +
		strings.Count(code_lower, "catch ") +
		strings.Count(code_lower, "&&") +
		strings.Count(code_lower, "||") + 1

	return metrics
}

// filterBySeverity filters findings by minimum severity level
func (s *CodeReviewService) filterBySeverity(findings []ReviewFinding, minSeverity string) []ReviewFinding {
	severityOrder := map[string]int{
		"hint":    1,
		"info":    2,
		"warning": 3,
		"error":   4,
	}

	minLevel, ok := severityOrder[minSeverity]
	if !ok {
		return findings
	}

	var filtered []ReviewFinding
	for _, f := range findings {
		if level, ok := severityOrder[f.Severity]; ok && level >= minLevel {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// countBySeverityType counts findings of a specific type
func (s *CodeReviewService) countBySeverityType(findings []ReviewFinding, findingType string) int {
	count := 0
	for _, f := range findings {
		if f.Type == findingType {
			count++
		}
	}
	return count
}

// generateSuggestions creates general improvement suggestions based on findings
func (s *CodeReviewService) generateSuggestions(findings []ReviewFinding, metrics ReviewMetrics) []string {
	var suggestions []string

	// Suggest based on metrics
	if metrics.CommentLines == 0 && metrics.CodeLines > 20 {
		suggestions = append(suggestions, "Consider adding comments to document complex logic")
	}

	if metrics.Complexity > 15 {
		suggestions = append(suggestions, "High cyclomatic complexity detected. Consider breaking down into smaller functions")
	}

	// Suggest based on findings
	securityCount := 0
	performanceCount := 0
	for _, f := range findings {
		if f.Type == "security" {
			securityCount++
		}
		if f.Type == "performance" {
			performanceCount++
		}
	}

	if securityCount > 0 {
		suggestions = append(suggestions, fmt.Sprintf("Address %d security issue(s) before deployment", securityCount))
	}

	if performanceCount > 2 {
		suggestions = append(suggestions, "Multiple performance issues detected. Consider profiling the code")
	}

	if len(findings) > 10 {
		suggestions = append(suggestions, "Many issues found. Consider incremental refactoring")
	}

	return suggestions
}

// QuickReview performs a fast, lightweight review
func (s *CodeReviewService) QuickReview(ctx context.Context, code, language string) (*ReviewResponse, error) {
	return s.Review(ctx, ReviewRequest{
		Code:       code,
		Language:   language,
		Focus:      []string{"bugs", "security"},
		MaxResults: 5,
	})
}

// SecurityReview performs a security-focused review
func (s *CodeReviewService) SecurityReview(ctx context.Context, code, language string) (*ReviewResponse, error) {
	return s.Review(ctx, ReviewRequest{
		Code:     code,
		Language: language,
		Focus:    []string{"security"},
		Severity: "warning",
	})
}

// MultiPassReview performs a two-pass review for deeper analysis
// Pass 1: Fast model — syntax, security, obvious bugs
// Pass 2: Deep model — architecture, logic, performance, fix suggestions
func (s *CodeReviewService) MultiPassReview(ctx context.Context, req ReviewRequest) (*ReviewResponse, error) {
	startTime := time.Now()

	// Pass 1: Quick scan (syntax + security)
	pass1Req := ReviewRequest{
		Code:       req.Code,
		Language:   req.Language,
		FileName:   req.FileName,
		Context:    req.Context,
		Focus:      []string{"bugs", "security", "syntax"},
		MaxResults: 20,
	}
	pass1, err := s.Review(ctx, pass1Req)
	if err != nil {
		return nil, fmt.Errorf("pass 1 failed: %w", err)
	}

	// Pass 2: Deep analysis (architecture + performance + fix suggestions)
	pass2Req := ReviewRequest{
		Code:     req.Code,
		Language: req.Language,
		FileName: req.FileName,
		Context:  req.Context,
		Focus:    []string{"performance", "best_practices", "architecture", "readability"},
	}
	pass2, err := s.Review(ctx, pass2Req)
	if err != nil {
		// Pass 2 failure is non-fatal, return pass 1 results
		return pass1, nil
	}

	// Merge results
	merged := &ReviewResponse{
		Findings:    append(pass1.Findings, pass2.Findings...),
		Summary:     pass1.Summary + " " + pass2.Summary,
		Score:       (pass1.Score + pass2.Score) / 2,
		Metrics:     pass1.Metrics,
		Suggestions: append(pass1.Suggestions, pass2.Suggestions...),
		ReviewedAt:  time.Now(),
		Duration:    time.Since(startTime).Milliseconds(),
	}

	// Add Apex-specific static analysis for Apex files
	if IsApexFile(req.FileName) || strings.EqualFold(req.Language, "apex") {
		engine := NewApexRuleEngine()
		apexFindings := engine.Analyze(req.Code, req.FileName)
		merged.Findings = append(merged.Findings, apexFindings...)
	}

	// Deduplicate findings by line + message
	merged.Findings = s.deduplicateFindings(merged.Findings)

	// Apply filters
	if req.Severity != "" {
		merged.Findings = s.filterBySeverity(merged.Findings, req.Severity)
	}
	if req.MaxResults > 0 && len(merged.Findings) > req.MaxResults {
		merged.Findings = merged.Findings[:req.MaxResults]
	}

	// Recalculate metrics
	merged.Metrics.SecurityIssues = s.countBySeverityType(merged.Findings, "security")
	merged.Metrics.BugRisks = s.countBySeverityType(merged.Findings, "bug")
	merged.Metrics.StyleIssues = s.countBySeverityType(merged.Findings, "style")

	return merged, nil
}

// ReviewWithFixes performs a review and generates concrete fix diffs
func (s *CodeReviewService) ReviewWithFixes(ctx context.Context, req ReviewRequest) (*ReviewResponse, []FixSuggestion, error) {
	// First do the multi-pass review
	review, err := s.MultiPassReview(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	// Only generate fixes for error/warning findings
	var criticalFindings []ReviewFinding
	for _, f := range review.Findings {
		if f.Severity == "error" || f.Severity == "warning" {
			criticalFindings = append(criticalFindings, f)
		}
	}

	if len(criticalFindings) == 0 {
		return review, nil, nil
	}

	// Ask AI for concrete fixes
	fixes, err := s.generateFixes(ctx, req.Code, req.Language, criticalFindings)
	if err != nil {
		// Non-fatal — return review without fixes
		return review, nil, nil
	}

	return review, fixes, nil
}

// FixSuggestion represents a concrete code fix with search/replace
type FixSuggestion struct {
	FindingIndex int    `json:"finding_index"`
	File         string `json:"file"`
	Search       string `json:"search"`
	Replace      string `json:"replace"`
	Description  string `json:"description"`
}

func (s *CodeReviewService) generateFixes(ctx context.Context, code, language string, findings []ReviewFinding) ([]FixSuggestion, error) {
	// Build findings summary for AI
	var findingsList strings.Builder
	for i, f := range findings {
		findingsList.WriteString(fmt.Sprintf("%d. [Line %d] %s: %s\n", i+1, f.Line, f.Severity, f.Message))
	}

	prompt := fmt.Sprintf(`Given this %s code and the following issues, generate concrete fixes.

Issues:
%s

For each fix, respond with JSON array:
[
  {
    "finding_index": <1-based index>,
    "search": "<exact text to find in the code>",
    "replace": "<replacement text>",
    "description": "<what this fix does>"
  }
]

The search text must match the code exactly. Only output valid JSON.

Code:
%s`, language, findingsList.String(), code)

	aiResponse, err := s.aiRouter.Generate(ctx, &ai.AIRequest{
		Capability: ai.CapabilityCodeReview,
		Prompt:     prompt,
		Language:   language,
	})
	if err != nil {
		return nil, err
	}

	// Parse fixes
	clean := aiResponse.Content
	if strings.HasPrefix(clean, "```json") {
		clean = strings.TrimPrefix(clean, "```json")
		clean = strings.TrimSuffix(strings.TrimSpace(clean), "```")
	}
	if strings.HasPrefix(clean, "```") {
		clean = strings.TrimPrefix(clean, "```")
		clean = strings.TrimSuffix(strings.TrimSpace(clean), "```")
	}
	clean = strings.TrimSpace(clean)

	var fixes []FixSuggestion
	if err := json.Unmarshal([]byte(clean), &fixes); err != nil {
		return nil, fmt.Errorf("failed to parse fix suggestions: %w", err)
	}

	return fixes, nil
}

// deduplicateFindings removes duplicate findings by line + message
func (s *CodeReviewService) deduplicateFindings(findings []ReviewFinding) []ReviewFinding {
	seen := make(map[string]bool)
	var unique []ReviewFinding

	for _, f := range findings {
		key := fmt.Sprintf("%d:%s", f.Line, f.Message)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, f)
		}
	}

	return unique
}

// ComprehensiveReview performs BOTH AI review AND build verification
// This catches issues that pure AI review misses (undefined symbols, type errors)
func (s *CodeReviewService) ComprehensiveReview(ctx context.Context, req ReviewRequest) (*ReviewResponse, error) {
	startTime := time.Now()

	// PHASE 1: Build verification (catches real compilation errors)
	var buildFindings []ReviewFinding
	var buildSuccess bool = true

	if s.buildVerifier != nil {
		buildResult, err := s.buildVerifier.VerifyBuild(ctx, req.Language)
		if err == nil {
			buildFindings = buildResult.Findings
			buildSuccess = buildResult.Success

			// If build fails, prioritize build errors
			if !buildSuccess && len(buildFindings) > 0 {
				// Still do AI review but mark it as secondary
				for i := range buildFindings {
					buildFindings[i].Type = "build_" + buildFindings[i].Type
				}
			}
		}
	}

	// PHASE 2: AI-powered semantic review (catches logic, security, style issues)
	aiReview, err := s.MultiPassReview(ctx, req)
	if err != nil {
		// If AI review fails but build verification worked, return build results
		if len(buildFindings) > 0 {
			return &ReviewResponse{
				Findings:   buildFindings,
				Summary:    "Build verification completed. AI review unavailable.",
				Score:      buildSuccess ? 70 : 30,
				Metrics:    s.calculateMetrics(req.Code),
				ReviewedAt: time.Now(),
				Duration:   time.Since(startTime).Milliseconds(),
			}, nil
		}
		return nil, err
	}

	// PHASE 3: Merge findings (build errors first, then AI findings)
	allFindings := append(buildFindings, aiReview.Findings...)

	// Deduplicate
	allFindings = s.deduplicateFindings(allFindings)

	// Sort by severity (errors first)
	allFindings = s.sortBySeverity(allFindings)

	// Adjust score based on build success
	score := aiReview.Score
	if !buildSuccess {
		// Heavily penalize failed builds
		score = min(score, 40)
	}

	// Update summary
	summary := aiReview.Summary
	if !buildSuccess {
		summary = fmt.Sprintf("BUILD FAILED: %d compilation error(s). %s", len(buildFindings), summary)
	}

	return &ReviewResponse{
		Findings:    allFindings,
		Summary:     summary,
		Score:       score,
		Metrics:     aiReview.Metrics,
		Suggestions: append([]string{"Fix all build errors before deployment"}, aiReview.Suggestions...),
		ReviewedAt:  time.Now(),
		Duration:    time.Since(startTime).Milliseconds(),
	}, nil
}

// sortBySeverity sorts findings with errors first, then warnings, then info
func (s *CodeReviewService) sortBySeverity(findings []ReviewFinding) []ReviewFinding {
	severityOrder := map[string]int{
		"error":   0,
		"warning": 1,
		"info":    2,
		"hint":    3,
	}

	// Simple bubble sort (findings lists are typically small)
	for i := 0; i < len(findings); i++ {
		for j := i + 1; j < len(findings); j++ {
			iOrder := severityOrder[findings[i].Severity]
			jOrder := severityOrder[findings[j].Severity]
			if jOrder < iOrder {
				findings[i], findings[j] = findings[j], findings[i]
			}
		}
	}

	return findings
}

// PreCommitReview runs a fast review suitable for pre-commit hooks
// Focuses on build verification and critical issues only
func (s *CodeReviewService) PreCommitReview(ctx context.Context, language, projectDir string) (*BuildResult, error) {
	verifier := NewBuildVerifier(projectDir)
	return verifier.VerifyBuild(ctx, language)
}
