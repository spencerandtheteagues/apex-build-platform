// Package codereview provides AI-powered code review functionality
package codereview

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"apex-build/internal/ai/router"
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
	aiRouter *router.AIRouter
}

// NewCodeReviewService creates a new code review service
func NewCodeReviewService(aiRouter *router.AIRouter) *CodeReviewService {
	return &CodeReviewService{
		aiRouter: aiRouter,
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
	aiResponse, err := s.aiRouter.Route(ctx, &router.Request{
		Capability: "code_review",
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
