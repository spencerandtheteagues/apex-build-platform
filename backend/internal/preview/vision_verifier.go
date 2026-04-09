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
  "summary": "short visual assessment",
  "issues": ["specific user-visible problems"],
  "repair_hints": ["implementation-ready fixes for the frontend agent"]
}

Focus on visual launch blockers and polish gaps:
- blank or nearly blank screen
- unreadable text or dark-on-dark / light-on-light contrast
- broken layout hierarchy or overflow
- missing spacing, missing navigation, unstyled form controls
- obvious Tailwind/CSS failures

If the screen looks acceptable, return empty arrays for issues and repair_hints.`, strings.TrimSpace(description)))

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
	result.Issues = compactNonEmptyStrings(result.Issues)
	result.RepairHints = compactNonEmptyStrings(result.RepairHints)
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
