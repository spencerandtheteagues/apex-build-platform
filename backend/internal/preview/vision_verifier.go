package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"apex-build/internal/ai"
)

type screenshotAnalyzer interface {
	AnalyzeImage(ctx context.Context, imageData []byte, prompt string) (string, error)
}

// VisionRepairResult is a structured, advisory UI review from a screenshot.
type VisionRepairResult struct {
	Summary     string   `json:"summary"`
	// Severity classifies the overall visual quality:
	//   "critical" — app is visually broken (blank screen, invisible text, zero CSS)
	//   "advisory" — usable but has polish gaps
	//   "clean"    — looks good, no actionable issues
	Severity    string   `json:"severity,omitempty"`
	Issues      []string `json:"issues"`
	RepairHints []string `json:"repair_hints"`
	Raw         string   `json:"raw,omitempty"`
}

// VisionVerifier uses an image-capable Claude model to review generated preview screenshots.
// It is optional and should never block builds when unavailable.
type VisionVerifier struct {
	analyzer screenshotAnalyzer
}

func NewVisionVerifier(analyzer screenshotAnalyzer) *VisionVerifier {
	if analyzer == nil {
		return nil
	}
	return &VisionVerifier{analyzer: analyzer}
}

func NewVisionVerifierFromEnv() *VisionVerifier {
	apiKey := strings.TrimSpace(os.Getenv("APEX_CLAUDE_VISION_KEY"))
	if apiKey == "" {
		// Fall back to the platform Anthropic key so vision review works without
		// a separate dedicated key. APEX_CLAUDE_VISION_KEY takes precedence when set.
		apiKey = strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	}
	if apiKey == "" {
		return nil
	}
	return NewVisionVerifier(ai.NewClaudeClient(apiKey))
}

func (vv *VisionVerifier) AnalyzeScreenshot(ctx context.Context, imageData []byte, description string) *VisionRepairResult {
	if vv == nil || vv.analyzer == nil || len(imageData) == 0 {
		return nil
	}

	prompt := strings.TrimSpace(fmt.Sprintf(`Analyze this screenshot of an in-browser generated app preview.

Context: %s

Return ONLY JSON with this shape:
{
  "summary": "one sentence visual assessment",
  "severity": "critical|advisory|clean",
  "issues": ["specific user-visible problems"],
  "repair_hints": ["implementation-ready fixes for the frontend agent"]
}

Severity rules:
- "critical": app is visually broken in a way that impairs usability — blank or white screen, invisible/unreadable text due to contrast failure, zero visible CSS styling, no content rendered
- "advisory": app is usable but has quality gaps — layout overflow, missing spacing, minor hierarchy issues, unstyled controls, could be improved
- "clean": looks good, no actionable visual issues

Focus on:
- blank or nearly blank screen
- unreadable text (dark-on-dark, light-on-light contrast failures)
- completely unstyled content (raw browser defaults, no Tailwind/CSS applied)
- broken layout hierarchy or severe overflow
- missing navigation or unstyled form controls

If the screen looks acceptable, set severity to "clean" and return empty arrays.`, strings.TrimSpace(description)))

	raw, err := vv.analyzer.AnalyzeImage(ctx, imageData, prompt)
	if err != nil {
		log.Printf("[vision_verifier] screenshot analysis skipped: %v", err)
		return nil
	}
	return parseVisionRepairResult(raw)
}

func parseVisionRepairResult(raw string) *VisionRepairResult {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	var parsed VisionRepairResult
	if json.Unmarshal([]byte(trimmed), &parsed) == nil {
		parsed.Raw = trimmed
		normalizeVisionRepairResult(&parsed)
		if parsed.Summary == "" && len(parsed.Issues) == 0 && len(parsed.RepairHints) == 0 {
			return nil
		}
		return &parsed
	}

	if object := extractVisionJSONObject(trimmed); object != "" {
		if json.Unmarshal([]byte(object), &parsed) == nil {
			parsed.Raw = object
			normalizeVisionRepairResult(&parsed)
			if parsed.Summary == "" && len(parsed.Issues) == 0 && len(parsed.RepairHints) == 0 {
				return nil
			}
			return &parsed
		}
	}

	fallback := &VisionRepairResult{
		Summary: trimmed,
		Raw:     trimmed,
	}
	normalizeVisionRepairResult(fallback)
	return fallback
}

func normalizeVisionRepairResult(result *VisionRepairResult) {
	if result == nil {
		return
	}
	result.Summary = strings.TrimSpace(result.Summary)
	result.Severity = normalizeVisionSeverity(result.Severity, result.Issues)
	result.Issues = compactNonEmptyStrings(result.Issues)
	result.RepairHints = compactNonEmptyStrings(result.RepairHints)
}

// normalizeVisionSeverity validates and canonicalizes the severity field.
// When Claude doesn't return a severity (or returns an unrecognized value),
// it is inferred from the issue text using keyword matching.
func normalizeVisionSeverity(raw string, issues []string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "critical":
		return "critical"
	case "advisory":
		return "advisory"
	case "clean":
		return "clean"
	}
	// Infer from issues when not set.
	if len(issues) == 0 {
		return "clean"
	}
	combined := strings.ToLower(strings.Join(issues, " "))
	if isVisionCriticalText(combined) {
		return "critical"
	}
	return "advisory"
}

// isVisionCriticalText returns true when the text describes a visually broken
// state that impairs usability (blank screen, invisible text, zero CSS).
func isVisionCriticalText(lower string) bool {
	criticalPhrases := []string{
		"blank screen", "white screen", "empty screen",
		"blank page", "white page",
		"invisible text", "unreadable", "no visible text",
		"dark-on-dark", "light-on-light", "zero contrast",
		"no styling", "no css", "missing css", "unstyled",
		"browser defaults", "no tailwind",
		"nothing rendered", "nothing visible", "no content",
		"completely blank", "completely empty", "completely white",
	}
	for _, phrase := range criticalPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// IsCritical reports whether this vision result represents a visually broken UI.
func (r *VisionRepairResult) IsCritical() bool {
	if r == nil {
		return false
	}
	return r.Severity == "critical"
}

func extractVisionJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(raw); i++ {
		ch := raw[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[start : i+1]
			}
		}
	}
	return ""
}
