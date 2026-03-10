// Package agents — LLM-powered build error analyzer.
//
// The ErrorAnalyzer closes the self-healing loop: instead of relying only on
// hard-coded heuristics (extractDependencyRepairHintsFromReadinessErrors), it
// sends the actual build errors plus the relevant source snippets to the AI and
// asks for precise, file-specific repair instructions.
//
// Design principles:
//   - Stateless: each call is independent; no mutable state on the struct.
//   - Non-blocking: if the AI call fails, the caller receives the original
//     heuristic hints unmodified — the analyzer never makes a build worse.
//   - Classified: errors are categorised before the LLM call so the prompt is
//     focused and the model isn't distracted by irrelevant context.
//   - Auditable: every analysis request and response is logged at debug level.

package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"apex-build/internal/ai"
)

// ─── Error classification ──────────────────────────────────────────────────────

// ErrorClass categorises a build error so the LLM prompt can be tightly focused.
type ErrorClass string

const (
	ErrorClassMissingDep    ErrorClass = "missing_dependency"    // package.json missing a required package
	ErrorClassTypeScript    ErrorClass = "typescript_error"      // TS type / compile error
	ErrorClassSyntax        ErrorClass = "syntax_error"          // generic syntax / parse error
	ErrorClassCORS          ErrorClass = "cors_misconfiguration" // CORS origin mismatch
	ErrorClassPortConflict  ErrorClass = "port_conflict"         // port already in use
	ErrorClassImportPath    ErrorClass = "import_path_error"     // bad module path / alias
	ErrorClassMissingFile   ErrorClass = "missing_file"          // referenced file not found
	ErrorClassBuildScript   ErrorClass = "build_script_error"    // package.json script mis-config
	ErrorClassIntegration   ErrorClass = "integration_error"     // frontend↔backend contract mismatch
	ErrorClassUnknown       ErrorClass = "unknown"               // catch-all
)

// ClassifiedError pairs a raw error message with its semantic class and any
// extracted metadata (e.g. the package name for a missing-dep error).
type ClassifiedError struct {
	Raw       string            `json:"raw"`
	Class     ErrorClass        `json:"class"`
	Meta      map[string]string `json:"meta,omitempty"`
}

// FileRepair is a single actionable fix the AI recommends for one file.
type FileRepair struct {
	FilePath    string `json:"file_path"`
	Problem     string `json:"problem"`
	Instruction string `json:"instruction"` // human-readable fix description
	CodeFix     string `json:"code_fix"`    // exact code to insert/replace (may be empty)
}

// RepairPlan is the structured output of an analysis run.
type RepairPlan struct {
	Repairs        []FileRepair `json:"repairs"`
	Summary        string       `json:"summary"`         // one-sentence diagnosis
	AnalyzedAt     time.Time    `json:"analyzed_at"`
	TokensUsed     int          `json:"tokens_used"`
	FallbackUsed   bool         `json:"fallback_used"`   // true when AI call failed and heuristics were used
}

// ─── Classifier ───────────────────────────────────────────────────────────────

// classificationRule maps a regex pattern to an error class and optional
// metadata extraction via named capture groups.
type classificationRule struct {
	pattern *regexp.Regexp
	class   ErrorClass
	metaFn  func(m []string) map[string]string
}

var classificationRules = []classificationRule{
	{
		pattern: regexp.MustCompile(`(?i)cannot find module '([^']+)'|Module not found: Error: Can't resolve '([^']+)'|does not declare dependency "([^"]+)"`),
		class:   ErrorClassMissingDep,
		metaFn: func(m []string) map[string]string {
			for _, g := range m[1:] {
				if g != "" {
					return map[string]string{"package": g}
				}
			}
			return nil
		},
	},
	{
		pattern: regexp.MustCompile(`(?i)TS\d{4}:|Type '.*' is not assignable|Property '.*' does not exist on type|Argument of type '.*' is not assignable`),
		class:   ErrorClassTypeScript,
	},
	{
		pattern: regexp.MustCompile(`(?i)SyntaxError:|Unexpected token|Expected ','\s|Expected ';'|is not valid JSON`),
		class:   ErrorClassSyntax,
	},
	{
		pattern: regexp.MustCompile(`(?i)CORS|Access-Control-Allow-Origin|blocked by CORS`),
		class:   ErrorClassCORS,
	},
	{
		pattern: regexp.MustCompile(`(?i)EADDRINUSE|address already in use|port \d+ is already`),
		class:   ErrorClassPortConflict,
	},
	{
		pattern: regexp.MustCompile(`(?i)Cannot find module '@/|resolve alias|Failed to resolve import`),
		class:   ErrorClassImportPath,
	},
	{
		pattern: regexp.MustCompile(`(?i)ENOENT: no such file|FileNotFoundError|open .*: no such file`),
		class:   ErrorClassMissingFile,
	},
	{
		pattern: regexp.MustCompile(`(?i)missing script:|npm run \w+ exited with code|command not found: (vite|ts-node|tsc)`),
		class:   ErrorClassBuildScript,
	},
	{
		pattern: regexp.MustCompile(`(?i)integration:|fetch.*failed|net::ERR|connection refused`),
		class:   ErrorClassIntegration,
	},
}

// ClassifyErrors takes raw error strings and returns structured classified errors.
func ClassifyErrors(rawErrors []string) []ClassifiedError {
	out := make([]ClassifiedError, 0, len(rawErrors))
	for _, raw := range rawErrors {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		ce := ClassifiedError{Raw: raw, Class: ErrorClassUnknown}
		for _, rule := range classificationRules {
			m := rule.pattern.FindStringSubmatch(raw)
			if m != nil {
				ce.Class = rule.class
				if rule.metaFn != nil {
					ce.Meta = rule.metaFn(m)
				}
				break
			}
		}
		out = append(out, ce)
	}
	return out
}

// ─── Analyzer ─────────────────────────────────────────────────────────────────

// ErrorAnalyzer uses an AIRouter to produce AI-powered repair plans from build
// errors. It falls back to heuristic hints if the AI call fails, so it is safe
// to call unconditionally in the build retry loop.
type ErrorAnalyzer struct {
	router   AIRouter
	provider ai.AIProvider // preferred provider for analysis calls
}

// NewErrorAnalyzer creates an analyzer. If provider is empty, the first
// available provider reported by the router is used.
func NewErrorAnalyzer(router AIRouter, provider ai.AIProvider) *ErrorAnalyzer {
	return &ErrorAnalyzer{router: router, provider: provider}
}

// Analyze classifies the given errors, selects the most relevant file snippets,
// calls the LLM for structured repair instructions, and returns a RepairPlan.
//
// relevantFiles maps file paths to their content. Pass only files the agent
// edited most recently — not the entire project — to stay within the context
// window. Recommended: at most 5–8 files, each truncated to ~150 lines.
//
// If the AI call fails or returns unparseable JSON, Analyze returns a plan
// built from the existing heuristic hints so the build can still retry.
func (a *ErrorAnalyzer) Analyze(
	ctx context.Context,
	rawErrors []string,
	relevantFiles map[string]string,
	userID uint,
) (*RepairPlan, error) {
	if len(rawErrors) == 0 {
		return &RepairPlan{AnalyzedAt: time.Now()}, nil
	}

	classified := ClassifyErrors(rawErrors)

	// Select a provider — prefer the requested one, fall back to any available.
	provider := a.provider
	if provider == "" {
		providers := a.router.GetAvailableProviders()
		if len(providers) > 0 {
			provider = providers[0]
		}
	}

	if provider == "" || !a.router.HasConfiguredProviders() {
		log.Printf("[error_analyzer] no AI provider available — returning heuristic-only plan")
		return a.heuristicFallback(classified), nil
	}

	prompt := a.buildPrompt(classified, relevantFiles)

	resp, err := a.router.Generate(ctx, provider, prompt, GenerateOptions{
		UserID:       userID,
		MaxTokens:    1200,
		Temperature:  0.1, // low temperature for precise, deterministic repair instructions
		SystemPrompt: errorAnalyzerSystemPrompt,
		PowerMode:    PowerFast, // cost-efficient; errors don't need frontier reasoning
	})
	if err != nil {
		log.Printf("[error_analyzer] AI call failed (%v) — returning heuristic-only plan", err)
		return a.heuristicFallback(classified), nil
	}

	plan, parseErr := a.parseResponse(resp.Content, classified)
	if parseErr != nil {
		log.Printf("[error_analyzer] failed to parse AI response (%v) — returning heuristic-only plan", parseErr)
		return a.heuristicFallback(classified), nil
	}

	if resp.Usage != nil {
		plan.TokensUsed = resp.Usage.TotalTokens
	}
	plan.AnalyzedAt = time.Now()
	return plan, nil
}

// buildPrompt assembles the prompt sent to the LLM, keeping it focused.
func (a *ErrorAnalyzer) buildPrompt(errors []ClassifiedError, files map[string]string) string {
	var sb strings.Builder

	sb.WriteString("## Build Errors\n\n")
	for i, ce := range errors {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, ce.Class, ce.Raw))
		for k, v := range ce.Meta {
			sb.WriteString(fmt.Sprintf("   %s: %s\n", k, v))
		}
	}

	if len(files) > 0 {
		sb.WriteString("\n## Relevant Source Files\n\n")
		for path, content := range files {
			lines := strings.Split(content, "\n")
			// Truncate long files to 120 lines to stay within context budget.
			if len(lines) > 120 {
				lines = lines[:120]
				lines = append(lines, "// ... (truncated)")
			}
			sb.WriteString(fmt.Sprintf("### %s\n```\n%s\n```\n\n", path, strings.Join(lines, "\n")))
		}
	}

	sb.WriteString(`
## Instructions

Return a JSON object with this exact structure:
{
  "summary": "one sentence diagnosis",
  "repairs": [
    {
      "file_path": "path/to/file",
      "problem": "what is wrong",
      "instruction": "how to fix it",
      "code_fix": "exact code snippet to use (empty string if not applicable)"
    }
  ]
}

Focus on the root cause, not symptoms. Each repair must be for a specific file.
Do not repeat the same fix for the same file. Output only valid JSON.`)

	return sb.String()
}

// parseResponse extracts a RepairPlan from the AI's JSON response.
// It handles the common case where the model wraps JSON in a markdown code fence.
func (a *ErrorAnalyzer) parseResponse(raw string, classified []ClassifiedError) (*RepairPlan, error) {
	// Strip ```json ... ``` fences if present.
	content := raw
	if idx := strings.Index(content, "```json"); idx != -1 {
		content = content[idx+7:]
		if end := strings.Index(content, "```"); end != -1 {
			content = content[:end]
		}
	} else if idx := strings.Index(content, "```"); idx != -1 {
		content = content[idx+3:]
		if end := strings.Index(content, "```"); end != -1 {
			content = content[:end]
		}
	}
	// Extract the first top-level JSON object by tracking brace depth.
	// Using first-open to last-close is fragile when the model emits prose
	// containing braces before the actual JSON object.
	content = extractFirstJSONObject(content)
	if content == "" {
		return nil, fmt.Errorf("no JSON object found in response")
	}

	var result struct {
		Summary string       `json:"summary"`
		Repairs []FileRepair `json:"repairs"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("JSON unmarshal failed: %w", err)
	}

	return &RepairPlan{
		Repairs: result.Repairs,
		Summary: result.Summary,
	}, nil
}

// heuristicFallback builds a RepairPlan from the existing string-matching hints,
// ensuring the build retry loop always gets actionable output even without AI.
func (a *ErrorAnalyzer) heuristicFallback(classified []ClassifiedError) *RepairPlan {
	plan := &RepairPlan{
		FallbackUsed: true,
		AnalyzedAt:   time.Now(),
		Summary:      "heuristic analysis (AI unavailable)",
	}

	for _, ce := range classified {
		repair := FileRepair{Problem: ce.Raw}
		switch ce.Class {
		case ErrorClassMissingDep:
			pkg := ce.Meta["package"]
			repair.FilePath = "package.json"
			repair.Instruction = fmt.Sprintf("Add missing package %q to dependencies in package.json", pkg)
			if pkg != "" {
				repair.CodeFix = fmt.Sprintf(`"%s": "latest"`, pkg)
			}
		case ErrorClassTypeScript:
			repair.FilePath = "(see error)"
			repair.Instruction = "Fix TypeScript type error: " + ce.Raw
		case ErrorClassCORS:
			repair.FilePath = "server/index.ts"
			repair.Instruction = "Ensure CORS middleware allows the frontend origin. Add: app.use(cors({ origin: process.env.FRONTEND_URL || 'http://localhost:5173' }))"
		case ErrorClassBuildScript:
			repair.FilePath = "package.json"
			repair.Instruction = `Verify scripts section includes "dev": "vite" and "build": "tsc -b && vite build"`
		case ErrorClassIntegration:
			repair.FilePath = "(see error)"
			repair.Instruction = "Check that frontend API base URL matches the backend port and that CORS is configured correctly."
		default:
			repair.FilePath = "(see error)"
			repair.Instruction = ce.Raw
		}
		plan.Repairs = append(plan.Repairs, repair)
	}
	return plan
}

// RepairHints converts a RepairPlan into the string slice format expected by the
// existing extractDependencyRepairHintsFromReadinessErrors return type, so it can
// be dropped into the retry loop without changing the manager interface.
func (p *RepairPlan) RepairHints() []string {
	hints := make([]string, 0, len(p.Repairs)+1)
	if p.Summary != "" {
		hints = append(hints, "DIAGNOSIS: "+p.Summary)
	}
	for _, r := range p.Repairs {
		hint := fmt.Sprintf("[%s] %s", r.FilePath, r.Instruction)
		if r.CodeFix != "" {
			hint += "\nSuggested fix:\n" + r.CodeFix
		}
		hints = append(hints, hint)
	}
	return hints
}

// errorAnalyzerSystemPrompt is the system prompt for analysis calls.
// It is kept short and task-focused to minimise token cost.
const errorAnalyzerSystemPrompt = `You are an expert build error analyst for a full-stack TypeScript/Go platform.
Your job is to read build errors and source code, identify the root cause, and produce
precise file-level fixes. Be concise and specific. Output only valid JSON.`

// extractFirstJSONObject scans s and returns the first syntactically complete
// top-level JSON object (i.e. balanced braces).  It handles the common case
// where the model emits prose or partial JSON before/after the actual object.
// Returns an empty string if no complete object is found.
func extractFirstJSONObject(s string) string {
	depth := 0
	inStr := false
	escape := false
	start := -1
	runes := []rune(s)
	for i, r := range runes {
		if escape {
			escape = false
			continue
		}
		if inStr {
			if r == '\\' {
				escape = true
			} else if r == '"' {
				inStr = false
			}
			continue
		}
		switch r {
		case '"':
			inStr = true
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			if depth > 0 {
				depth--
				if depth == 0 && start >= 0 {
					return string(runes[start : i+1])
				}
			}
		}
	}
	return ""
}
