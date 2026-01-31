// Package codereview - Apex/Salesforce-specific code review rules
// Detects governor limit violations, security issues, and Salesforce best practices
package codereview

import (
	"fmt"
	"regexp"
	"strings"
)

// ApexRuleEngine provides static analysis for Apex/Visualforce/LWC code
type ApexRuleEngine struct{}

// NewApexRuleEngine creates a new Apex rule engine
func NewApexRuleEngine() *ApexRuleEngine {
	return &ApexRuleEngine{}
}

// apexRule defines a static analysis rule
type apexRule struct {
	ID          string
	Type        string // governor_limit, security, best_practice, performance
	Severity    string // error, warning, info
	Pattern     *regexp.Regexp
	Message     string
	Suggestion  string
	LoopContext bool // Only applies inside loops
}

// Analyze runs all Apex-specific rules against the given code
func (e *ApexRuleEngine) Analyze(code, fileName string) []ReviewFinding {
	var findings []ReviewFinding

	lines := strings.Split(code, "\n")
	loopDepth := 0
	inTestMethod := false

	rules := e.getRules()

	for lineNum, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		// Track loop context
		if isLoopStart(lower) {
			loopDepth++
		}
		if isBlockEnd(trimmed) && loopDepth > 0 {
			loopDepth--
		}

		// Track test method context
		if strings.Contains(lower, "@istest") || strings.Contains(lower, "testmethod") {
			inTestMethod = true
		}
		if inTestMethod && isBlockEnd(trimmed) {
			inTestMethod = false
		}

		// Run pattern-based rules
		for _, rule := range rules {
			if rule.LoopContext && loopDepth == 0 {
				continue
			}
			if rule.Pattern.MatchString(trimmed) {
				findings = append(findings, ReviewFinding{
					Type:       rule.Type,
					Severity:   rule.Severity,
					Line:       lineNum + 1,
					Message:    rule.Message,
					Suggestion: rule.Suggestion,
					Code:       trimmed,
					RuleID:     rule.ID,
				})
			}
		}

		// Context-aware rules
		findings = append(findings, e.checkContextRules(trimmed, lower, lineNum+1, loopDepth, inTestMethod)...)
	}

	// Whole-file rules
	findings = append(findings, e.checkWholeFileRules(code, lines, fileName)...)

	return findings
}

func (e *ApexRuleEngine) getRules() []apexRule {
	return []apexRule{
		// ==================== Governor Limit Violations ====================
		{
			ID:          "apex-soql-in-loop",
			Type:        "governor_limit",
			Severity:    "error",
			Pattern:     regexp.MustCompile(`(?i)\[\s*SELECT\s+`),
			Message:     "SOQL query inside a loop — governor limit violation (max 100 SOQL queries per transaction)",
			Suggestion:  "Move the query before the loop and use a Map/Set for lookups",
			LoopContext: true,
		},
		{
			ID:          "apex-dml-in-loop",
			Type:        "governor_limit",
			Severity:    "error",
			Pattern:     regexp.MustCompile(`(?i)\b(insert|update|delete|upsert|undelete|merge)\s+`),
			Message:     "DML operation inside a loop — governor limit violation (max 150 DML statements per transaction)",
			Suggestion:  "Collect records in a List and perform DML after the loop",
			LoopContext: true,
		},
		{
			ID:          "apex-sosl-in-loop",
			Type:        "governor_limit",
			Severity:    "error",
			Pattern:     regexp.MustCompile(`(?i)\bFIND\s+`),
			Message:     "SOSL search inside a loop — governor limit violation (max 20 SOSL queries per transaction)",
			Suggestion:  "Move the SOSL query before the loop",
			LoopContext: true,
		},
		{
			ID:          "apex-future-in-loop",
			Type:        "governor_limit",
			Severity:    "error",
			Pattern:     regexp.MustCompile(`(?i)@future`),
			Message:     "@future method invocation inside a loop — governor limit violation",
			Suggestion:  "Collect IDs/data and call the @future method once after the loop",
			LoopContext: true,
		},
		{
			ID:          "apex-sendEmail-in-loop",
			Type:        "governor_limit",
			Severity:    "error",
			Pattern:     regexp.MustCompile(`(?i)Messaging\.sendEmail`),
			Message:     "Email send inside a loop — governor limit violation (max 10 sendEmail calls per transaction)",
			Suggestion:  "Collect emails in a list and send once after the loop",
			LoopContext: true,
		},

		// ==================== Security Rules ====================
		{
			ID:       "apex-soql-injection",
			Type:     "security",
			Severity: "error",
			Pattern:  regexp.MustCompile(`(?i)Database\.(query|getQueryLocator)\s*\(\s*['"].*\+`),
			Message:  "Potential SOQL injection — dynamic query with string concatenation",
			Suggestion: "Use bind variables (:variable) or String.escapeSingleQuotes()",
		},
		{
			ID:       "apex-soql-injection-var",
			Type:     "security",
			Severity: "error",
			Pattern:  regexp.MustCompile(`(?i)\'\s*\+\s*\w+\s*\+\s*\'.*(?:SELECT|FROM|WHERE)`),
			Message:  "Potential SOQL injection — user input concatenated into query string",
			Suggestion: "Use bind variables (:variable) instead of string concatenation",
		},
		{
			ID:       "apex-no-crud-check",
			Type:     "security",
			Severity: "warning",
			Pattern:  regexp.MustCompile(`(?i)(?:insert|update|delete)\s+\w+;`),
			Message:  "DML without CRUD/FLS check — may violate sharing rules",
			Suggestion: "Check Schema.sObjectType.ObjectName.isCreateable()/isUpdateable()/isDeletable() before DML",
		},
		{
			ID:       "apex-hardcoded-id",
			Type:     "security",
			Severity: "warning",
			Pattern:  regexp.MustCompile(`(?i)['"]\s*[a-zA-Z0-9]{15,18}\s*['"]`),
			Message:  "Hardcoded Salesforce record ID detected",
			Suggestion: "Use Custom Settings, Custom Metadata Types, or queries to get IDs dynamically",
		},
		{
			ID:       "apex-without-sharing",
			Type:     "security",
			Severity: "warning",
			Pattern:  regexp.MustCompile(`(?i)without\s+sharing`),
			Message:  "Class declared 'without sharing' — bypasses all sharing rules",
			Suggestion: "Use 'with sharing' unless there is a documented reason to bypass sharing rules",
		},

		// ==================== Best Practices ====================
		{
			ID:       "apex-hardcoded-string",
			Type:     "best_practice",
			Severity: "info",
			Pattern:  regexp.MustCompile(`(?i)System\.debug\s*\(\s*['"]`),
			Message:  "System.debug with hardcoded string — consider using a logging framework",
			Suggestion: "Use a custom Logger class with log levels for better production debugging",
		},
		{
			ID:       "apex-select-star",
			Type:     "performance",
			Severity: "warning",
			Pattern:  regexp.MustCompile(`(?i)SELECT\s+\*\s+FROM`),
			Message:  "SELECT * in SOQL is not supported — list specific fields",
			Suggestion: "Explicitly list the fields you need in the SELECT clause",
		},
		{
			ID:       "apex-no-limit",
			Type:     "performance",
			Severity: "info",
			Pattern:  regexp.MustCompile(`(?i)\[\s*SELECT\s+(?:(?!LIMIT).)*\]\s*;`),
			Message:  "SOQL query without LIMIT clause — could return excessive records",
			Suggestion: "Add a LIMIT clause to prevent hitting the 50,000 row limit",
		},
	}
}

// checkContextRules applies rules that need context awareness
func (e *ApexRuleEngine) checkContextRules(trimmed, lower string, lineNum, loopDepth int, inTest bool) []ReviewFinding {
	var findings []ReviewFinding

	// Callout in loop
	if loopDepth > 0 {
		if strings.Contains(lower, "http.send") || strings.Contains(lower, "httprequest") {
			findings = append(findings, ReviewFinding{
				Type:       "governor_limit",
				Severity:   "error",
				Line:       lineNum,
				Message:    "HTTP callout inside a loop — governor limit violation (max 100 callouts per transaction)",
				Suggestion: "Collect data and batch callouts, or use Queueable for async processing",
				Code:       trimmed,
				RuleID:     "apex-callout-in-loop",
			})
		}
	}

	// Test assertions
	if inTest {
		if strings.Contains(lower, "insert ") || strings.Contains(lower, "update ") {
			// Check if there are any assertions nearby — this is a heuristic
			if !strings.Contains(lower, "assert") {
				// This would need multi-line context for accuracy, so just flag the pattern
			}
		}
	}

	// Trigger with no handler pattern
	if strings.Contains(lower, "trigger ") && strings.Contains(lower, " on ") {
		if strings.Contains(lower, "insert") || strings.Contains(lower, "update") || strings.Contains(lower, "delete") {
			// Check if the trigger has logic directly (not delegating to a handler)
			if !strings.Contains(lower, "handler") && !strings.Contains(lower, "service") && !strings.Contains(lower, "helper") {
				findings = append(findings, ReviewFinding{
					Type:       "best_practice",
					Severity:   "warning",
					Line:       lineNum,
					Message:    "Trigger contains logic directly — use a trigger handler pattern",
					Suggestion: "Delegate trigger logic to a handler class (e.g., AccountTriggerHandler) for better testability and maintainability",
					Code:       trimmed,
					RuleID:     "apex-trigger-handler",
				})
			}
		}
	}

	return findings
}

// checkWholeFileRules runs rules that need full-file context
func (e *ApexRuleEngine) checkWholeFileRules(code string, lines []string, fileName string) []ReviewFinding {
	var findings []ReviewFinding
	lower := strings.ToLower(code)

	// Check for test class coverage patterns
	if strings.Contains(lower, "@istest") || strings.Contains(fileName, "test") || strings.Contains(fileName, "Test") {
		// Check for SeeAllData=true
		if strings.Contains(lower, "seealldata=true") {
			findings = append(findings, ReviewFinding{
				Type:       "best_practice",
				Severity:   "warning",
				Line:       findLineContaining(lines, "SeeAllData"),
				Message:    "@isTest(SeeAllData=true) detected — tests should create their own data",
				Suggestion: "Remove SeeAllData=true and use test data factory methods instead",
				RuleID:     "apex-seealldata",
			})
		}

		// Check for assertions
		assertCount := strings.Count(lower, "system.assert") +
			strings.Count(lower, "system.assertequals") +
			strings.Count(lower, "system.assertnotequals") +
			strings.Count(lower, "assert.are")
		testMethodCount := strings.Count(lower, "@istest") + strings.Count(lower, "testmethod")
		if testMethodCount > 0 && assertCount == 0 {
			findings = append(findings, ReviewFinding{
				Type:       "best_practice",
				Severity:   "error",
				Line:       1,
				Message:    "Test class has no assertions — tests must verify expected behavior",
				Suggestion: "Add System.assertEquals() or System.assert() calls to validate outcomes",
				RuleID:     "apex-no-assertions",
			})
		}
	}

	// Check for bulkification in triggers
	if strings.Contains(lower, "trigger ") && strings.Contains(lower, " on ") {
		if strings.Contains(lower, "trigger.new[0]") || strings.Contains(lower, "trigger.old[0]") {
			findings = append(findings, ReviewFinding{
				Type:       "governor_limit",
				Severity:   "error",
				Line:       findLineContaining(lines, "Trigger.new[0]"),
				Message:    "Non-bulkified trigger — accessing single record via index. Must handle bulk operations (up to 200 records)",
				Suggestion: "Iterate over Trigger.new/Trigger.old to handle all records in the batch",
				RuleID:     "apex-non-bulk-trigger",
			})
		}
	}

	// Check for missing sharing declaration
	classPattern := regexp.MustCompile(`(?i)(?:public|global)\s+class\s+\w+`)
	sharingPattern := regexp.MustCompile(`(?i)(?:with|without)\s+sharing`)
	if classPattern.MatchString(code) && !sharingPattern.MatchString(code) {
		findings = append(findings, ReviewFinding{
			Type:       "security",
			Severity:   "warning",
			Line:       1,
			Message:    "Class has no sharing declaration — defaults to 'without sharing' which may expose data",
			Suggestion: "Explicitly declare 'with sharing' or 'without sharing' on all Apex classes",
			RuleID:     "apex-no-sharing-declaration",
		})
	}

	return findings
}

// IsApexFile returns true if the file is an Apex/Salesforce file
func IsApexFile(fileName string) bool {
	lower := strings.ToLower(fileName)
	return strings.HasSuffix(lower, ".cls") ||
		strings.HasSuffix(lower, ".trigger") ||
		strings.HasSuffix(lower, ".apex") ||
		strings.HasSuffix(lower, ".soql")
}

// Helper to detect loop starts
func isLoopStart(lower string) bool {
	return strings.HasPrefix(lower, "for ") || strings.HasPrefix(lower, "for(") ||
		strings.HasPrefix(lower, "while ") || strings.HasPrefix(lower, "while(") ||
		strings.HasPrefix(lower, "do ") || strings.HasPrefix(lower, "do{")
}

// Helper to detect block ends
func isBlockEnd(trimmed string) bool {
	return trimmed == "}" || strings.HasPrefix(trimmed, "}")
}

// Find the line number containing a substring (case-insensitive)
func findLineContaining(lines []string, substr string) int {
	lower := strings.ToLower(substr)
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), lower) {
			return i + 1
		}
	}
	return 1
}

// FormatApexFindings formats findings into a human-readable report
func FormatApexFindings(findings []ReviewFinding) string {
	if len(findings) == 0 {
		return "No Apex-specific issues found."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d Apex-specific issue(s):\n\n", len(findings)))

	for i, f := range findings {
		icon := "ℹ️"
		switch f.Severity {
		case "error":
			icon = "🔴"
		case "warning":
			icon = "🟡"
		}
		sb.WriteString(fmt.Sprintf("%d. %s [%s] Line %d: %s\n", i+1, icon, f.RuleID, f.Line, f.Message))
		if f.Suggestion != "" {
			sb.WriteString(fmt.Sprintf("   Fix: %s\n", f.Suggestion))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
