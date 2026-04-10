package agents

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"apex-build/internal/ai"
)

type visionSpecAnalyzer interface {
	AnalyzeImage(ctx context.Context, imageData []byte, prompt string) (string, error)
}

type ComponentSpec struct {
	AppType     string   `json:"app_type"`
	Layout      string   `json:"layout"`
	Components  []string `json:"components"`
	ColorScheme []string `json:"color_scheme"`
	Description string   `json:"description"`
	Confidence  float64  `json:"confidence"`
	Raw         string   `json:"raw,omitempty"`
}

type VisionIntakeProcessor struct {
	analyzer visionSpecAnalyzer
}

func NewVisionIntakeProcessor(analyzer visionSpecAnalyzer) *VisionIntakeProcessor {
	if analyzer == nil {
		return nil
	}
	return &VisionIntakeProcessor{analyzer: analyzer}
}

func NewVisionIntakeProcessorFromEnv() *VisionIntakeProcessor {
	// APEX_CLAUDE_VISION_KEY takes precedence; fall back to the shared ANTHROPIC_API_KEY
	// so vision intake works immediately without a dedicated env var.
	apiKey := strings.TrimSpace(os.Getenv("APEX_CLAUDE_VISION_KEY"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	}
	if apiKey == "" {
		return nil
	}
	return NewVisionIntakeProcessor(ai.NewClaudeClient(apiKey))
}

// multiModalInputEnabled returns true unless APEX_MULTI_MODAL_INPUT is
// explicitly set to a falsy value. Default is enabled.
func multiModalInputEnabled() bool {
	val := strings.TrimSpace(strings.ToLower(os.Getenv("APEX_MULTI_MODAL_INPUT")))
	if val == "" {
		return true
	}
	return val == "1" || val == "true" || val == "yes" || val == "on"
}

func (p *VisionIntakeProcessor) ExtractSpec(ctx context.Context, imageData []byte) *ComponentSpec {
	if p == nil || p.analyzer == nil || len(imageData) == 0 {
		return nil
	}

	prompt := `Analyze this uploaded app wireframe, mockup, screenshot, or reference UI.

Return ONLY JSON with this exact shape:
{
  "app_type": "short product category",
  "layout": "concise structural layout description",
  "components": ["key visible UI elements"],
  "color_scheme": ["dominant visual colors or style notes"],
  "description": "a concise implementation-ready visual brief for the planner/frontend agent",
  "confidence": 0.0
}

Focus on actionable product-design intent:
- app type and overall structure
- core UI regions, navigation, data surfaces, and inputs
- visual hierarchy and design language
- obvious interaction affordances

If unsure, keep fields concise and lower confidence.`

	raw, err := p.analyzer.AnalyzeImage(ctx, imageData, prompt)
	if err != nil {
		log.Printf("[vision_intake] image analysis skipped: %v", err)
		return nil
	}
	return parseVisionComponentSpec(raw)
}

func parseVisionComponentSpec(raw string) *ComponentSpec {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	var parsed ComponentSpec
	if json.Unmarshal([]byte(trimmed), &parsed) == nil {
		parsed.Raw = trimmed
		normalizeComponentSpec(&parsed)
		if parsed.Description == "" && parsed.AppType == "" && len(parsed.Components) == 0 {
			return nil
		}
		return &parsed
	}

	if object := extractVisionJSONObject(trimmed); object != "" {
		if json.Unmarshal([]byte(object), &parsed) == nil {
			parsed.Raw = object
			normalizeComponentSpec(&parsed)
			if parsed.Description == "" && parsed.AppType == "" && len(parsed.Components) == 0 {
				return nil
			}
			return &parsed
		}
	}

	return &ComponentSpec{
		Description: trimmed,
		Raw:         trimmed,
	}
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

func compactNonEmptyVisionStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeComponentSpec(spec *ComponentSpec) {
	if spec == nil {
		return
	}
	spec.AppType = strings.TrimSpace(spec.AppType)
	spec.Layout = strings.TrimSpace(spec.Layout)
	spec.Description = strings.TrimSpace(spec.Description)
	spec.Components = compactNonEmptyVisionStrings(spec.Components)
	spec.ColorScheme = compactNonEmptyVisionStrings(spec.ColorScheme)
	if spec.Confidence < 0 {
		spec.Confidence = 0
	}
	if spec.Confidence > 1 {
		spec.Confidence = 1
	}
}

func decodeVisionIntakeImage(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("wireframe image is empty")
	}
	if comma := strings.Index(trimmed, ","); comma >= 0 && strings.Contains(trimmed[:comma], "base64") {
		trimmed = trimmed[comma+1:]
	}
	decoded, err := base64.StdEncoding.DecodeString(trimmed)
	if err == nil {
		return decoded, nil
	}
	decoded, err = base64.RawStdEncoding.DecodeString(trimmed)
	if err == nil {
		return decoded, nil
	}
	return nil, err
}

func cloneBuildRequestForCreation(req *BuildRequest) *BuildRequest {
	if req == nil {
		return nil
	}
	cloned := *req
	cloned.TechStack = cloneTechStack(req.TechStack)
	cloned.RoleAssignments = cloneStringMap(req.RoleAssignments)
	return &cloned
}

func (am *AgentManager) prepareBuildRequestForCreation(req *BuildRequest) *BuildRequest {
	cloned := cloneBuildRequestForCreation(req)
	if cloned == nil {
		return nil
	}
	if am == nil || am.visionIntake == nil || strings.TrimSpace(cloned.WireframeImage) == "" {
		return cloned
	}
	if !multiModalInputEnabled() {
		log.Printf("[vision_intake] skipped: APEX_MULTI_MODAL_INPUT=false")
		return cloned
	}

	imageData, err := decodeVisionIntakeImage(cloned.WireframeImage)
	if err != nil {
		log.Printf("[vision_intake] could not decode wireframe image: %v", err)
		return cloned
	}

	baseCtx := context.Background()
	if am.ctx != nil {
		baseCtx = am.ctx
	}
	ctx, cancel := context.WithTimeout(baseCtx, 30*time.Second)
	start := time.Now()
	spec := am.visionIntake.ExtractSpec(ctx, imageData)
	elapsed := time.Since(start)
	cancel()

	if spec == nil {
		log.Printf("[vision_intake] {\"outcome\":\"no_spec\",\"image_bytes\":%d,\"elapsed_ms\":%d}", len(imageData), elapsed.Milliseconds())
		return cloned
	}

	cloned.WireframeDescription = spec.Description
	augmented := strings.TrimSpace(strings.Join([]string{
		buildVisionPromptPrefix(spec),
		firstNonEmptyString(cloned.Prompt, cloned.Description),
	}, "\n\n"))
	if augmented != "" {
		cloned.Prompt = augmented
	}

	log.Printf("[vision_intake] {\"outcome\":\"ok\",\"app_type\":%q,\"components\":%d,\"confidence\":%.2f,\"image_bytes\":%d,\"elapsed_ms\":%d}",
		spec.AppType, len(spec.Components), spec.Confidence, len(imageData), elapsed.Milliseconds())

	return cloned
}

func buildVisionPromptPrefix(spec *ComponentSpec) string {
	if spec == nil {
		return ""
	}
	lines := []string{"Visual reference extracted from uploaded wireframe:"}
	if spec.AppType != "" {
		lines = append(lines, "- App type: "+spec.AppType)
	}
	if spec.Layout != "" {
		lines = append(lines, "- Layout: "+spec.Layout)
	}
	if len(spec.Components) > 0 {
		lines = append(lines, "- Components: "+strings.Join(spec.Components, ", "))
	}
	if len(spec.ColorScheme) > 0 {
		lines = append(lines, "- Visual style: "+strings.Join(spec.ColorScheme, ", "))
	}
	if spec.Description != "" {
		lines = append(lines, "- Visual brief: "+spec.Description)
	}
	if spec.Confidence > 0 {
		lines = append(lines, fmt.Sprintf("- Vision confidence: %.2f", spec.Confidence))
	}
	lines = append(lines, "Treat this as concrete design direction unless the user's text explicitly overrides it.")
	return strings.Join(lines, "\n")
}
