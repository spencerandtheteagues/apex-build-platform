package agents

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

type stubVisionSpecAnalyzer struct {
	raw string
	err error
}

func (s *stubVisionSpecAnalyzer) AnalyzeImage(ctx context.Context, imageData []byte, prompt string) (string, error) {
	return s.raw, s.err
}

func TestDecodeVisionIntakeImageSupportsDataURL(t *testing.T) {
	want := []byte{0x89, 0x50, 0x4e, 0x47}
	got, err := decodeVisionIntakeImage("data:image/png;base64," + base64.StdEncoding.EncodeToString(want))
	if err != nil {
		t.Fatalf("expected data URL decode to succeed, got %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestParseVisionComponentSpecExtractsEmbeddedJSONObject(t *testing.T) {
	raw := "Here is the analysis:\n{\"app_type\":\"dashboard\",\"layout\":\"sidebar + content\",\"components\":[\"sidebar\",\"cards\"],\"color_scheme\":[\"slate\",\"indigo\"],\"description\":\"A metrics dashboard with prominent KPI cards.\",\"confidence\":0.92}"
	spec := parseVisionComponentSpec(raw)
	if spec == nil {
		t.Fatal("expected parsed component spec")
	}
	if spec.AppType != "dashboard" {
		t.Fatalf("expected app type dashboard, got %q", spec.AppType)
	}
	if spec.Layout != "sidebar + content" {
		t.Fatalf("expected layout to be preserved, got %q", spec.Layout)
	}
	if spec.Description == "" || spec.Confidence != 0.92 {
		t.Fatalf("unexpected parsed spec: %+v", spec)
	}
}

func TestCreateBuildUsesPromptAndVisionAugmentation(t *testing.T) {
	t.Setenv("APEX_MULTI_MODAL_INPUT", "true")
	am := &AgentManager{
		builds:        make(map[string]*Build),
		agents:        make(map[string]*Agent),
		subscribers:   make(map[string][]chan *WSMessage),
		buildMonitors: make(map[string]struct{}),
		ctx:           context.Background(),
		visionIntake: NewVisionIntakeProcessor(&stubVisionSpecAnalyzer{
			raw: `{"app_type":"dashboard","layout":"sidebar + workspace","components":["sidebar","kanban board"],"color_scheme":["slate","emerald"],"description":"A workspace dashboard with a left nav and kanban board.","confidence":0.88}`,
		}),
	}

	originalPrompt := "Build a project planning dashboard"
	req := &BuildRequest{
		Description:    "Short build request",
		Prompt:         originalPrompt,
		WireframeImage: base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4e, 0x47}),
		Mode:           ModeFull,
		PowerMode:      PowerBalanced,
	}

	build, err := am.CreateBuild(42, "pro", req)
	if err != nil {
		t.Fatalf("expected build creation to succeed, got %v", err)
	}
	if !strings.Contains(build.Description, "Visual reference extracted from uploaded wireframe:") {
		t.Fatalf("expected augmented build description, got %q", build.Description)
	}
	if !strings.Contains(build.Description, originalPrompt) {
		t.Fatalf("expected original prompt to remain in build description, got %q", build.Description)
	}
	if !strings.Contains(build.Description, "kanban board") {
		t.Fatalf("expected visual components to be merged into build description, got %q", build.Description)
	}
	if req.Prompt != originalPrompt {
		t.Fatalf("expected original request prompt to remain unchanged, got %q", req.Prompt)
	}
}

func TestMultiModalInputFlagDisablesVisionAugmentation(t *testing.T) {
	t.Setenv("APEX_MULTI_MODAL_INPUT", "false")
	am := &AgentManager{
		builds:        make(map[string]*Build),
		agents:        make(map[string]*Agent),
		subscribers:   make(map[string][]chan *WSMessage),
		buildMonitors: make(map[string]struct{}),
		ctx:           context.Background(),
		visionIntake: NewVisionIntakeProcessor(&stubVisionSpecAnalyzer{
			raw: `{"app_type":"dashboard","layout":"left nav","components":["sidebar"],"description":"Dashboard.","confidence":0.9}`,
		}),
	}

	originalPrompt := "Build a dashboard"
	req := &BuildRequest{
		Description:    originalPrompt,
		Prompt:         originalPrompt,
		WireframeImage: base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4e, 0x47}),
		Mode:           ModeFull,
		PowerMode:      PowerFast,
	}

	build, err := am.CreateBuild(42, "pro", req)
	if err != nil {
		t.Fatalf("expected build creation to succeed, got %v", err)
	}
	if strings.Contains(build.Description, "Visual reference extracted from uploaded wireframe:") {
		t.Fatalf("expected NO vision augmentation when APEX_MULTI_MODAL_INPUT=false, got %q", build.Description)
	}
}

func TestMultiModalInputEnabledByDefault(t *testing.T) {
	t.Setenv("APEX_MULTI_MODAL_INPUT", "")
	if !multiModalInputEnabled() {
		t.Error("expected multiModalInputEnabled=true when env var is unset")
	}
}

func TestMultiModalInputFlagFalsyValues(t *testing.T) {
	for _, val := range []string{"false", "0", "no", "off"} {
		t.Setenv("APEX_MULTI_MODAL_INPUT", val)
		if multiModalInputEnabled() {
			t.Errorf("expected multiModalInputEnabled=false for %q", val)
		}
	}
}

func TestNewVisionIntakeProcessorFromEnvFallsBackToAnthropicKey(t *testing.T) {
	t.Setenv("APEX_CLAUDE_VISION_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-key")
	// Should not return nil when ANTHROPIC_API_KEY is set
	p := NewVisionIntakeProcessorFromEnv()
	if p == nil {
		t.Error("expected non-nil VisionIntakeProcessor when ANTHROPIC_API_KEY is set")
	}
}

func TestNewVisionIntakeProcessorFromEnvPrefersApexKey(t *testing.T) {
	t.Setenv("APEX_CLAUDE_VISION_KEY", "sk-apex-key")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-fallback")
	// Should work regardless of which key is used
	p := NewVisionIntakeProcessorFromEnv()
	if p == nil {
		t.Error("expected non-nil VisionIntakeProcessor when APEX_CLAUDE_VISION_KEY is set")
	}
}

func TestNewVisionIntakeProcessorFromEnvNilWhenNoKeys(t *testing.T) {
	t.Setenv("APEX_CLAUDE_VISION_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	p := NewVisionIntakeProcessorFromEnv()
	if p != nil {
		t.Error("expected nil VisionIntakeProcessor when no API keys are set")
	}
}

func TestBuildVisionPromptPrefixIncludesAllFields(t *testing.T) {
	spec := &ComponentSpec{
		AppType:     "e-commerce",
		Layout:      "grid with sidebar",
		Components:  []string{"product cards", "cart icon", "search bar"},
		ColorScheme: []string{"indigo", "white"},
		Description: "A shopping app with a product grid and persistent cart.",
		Confidence:  0.85,
	}
	prefix := buildVisionPromptPrefix(spec)
	for _, want := range []string{
		"e-commerce", "grid with sidebar", "product cards", "cart icon",
		"indigo", "A shopping app", "0.85",
	} {
		if !strings.Contains(prefix, want) {
			t.Errorf("expected prefix to contain %q, got:\n%s", want, prefix)
		}
	}
}

func TestPrepareBuildRequestForCreationPassesThroughWithoutImage(t *testing.T) {
	t.Setenv("APEX_MULTI_MODAL_INPUT", "true")
	am := &AgentManager{
		visionIntake: NewVisionIntakeProcessor(&stubVisionSpecAnalyzer{
			raw: `{"app_type":"todo","description":"Todo app.","confidence":0.9}`,
		}),
	}
	req := &BuildRequest{
		Description: "Build a todo app",
		Prompt:      "Build a todo app",
	}
	result := am.prepareBuildRequestForCreation(req)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if strings.Contains(result.Prompt, "Visual reference") {
		t.Error("expected no vision augmentation when WireframeImage is empty")
	}
}

func TestPrepareBuildRequestForCreationClonesRequest(t *testing.T) {
	t.Setenv("APEX_MULTI_MODAL_INPUT", "true")
	am := &AgentManager{}
	original := &BuildRequest{
		Description: "Original description",
		Prompt:      "Original prompt",
	}
	result := am.prepareBuildRequestForCreation(original)
	result.Prompt = "Modified"
	if original.Prompt != "Original prompt" {
		t.Error("expected prepareBuildRequestForCreation to clone the request, not modify original")
	}
}

func TestExtractVisionJSONObjectFromEmbeddedText(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{`{"key":"value"}`, `{"key":"value"}`},
		{`prefix {"key":"value"} suffix`, `{"key":"value"}`},
		{`no json here`, ""},
		{`{"nested":{"a":1}}`, `{"nested":{"a":1}}`},
	}
	for _, tc := range cases {
		got := extractVisionJSONObject(tc.input)
		if got != tc.want {
			t.Errorf("extractVisionJSONObject(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestNormalizeComponentSpecClampsConfidence(t *testing.T) {
	spec := &ComponentSpec{Confidence: 1.5}
	normalizeComponentSpec(spec)
	if spec.Confidence != 1.0 {
		t.Errorf("expected confidence clamped to 1.0, got %f", spec.Confidence)
	}
	spec2 := &ComponentSpec{Confidence: -0.3}
	normalizeComponentSpec(spec2)
	if spec2.Confidence != 0.0 {
		t.Errorf("expected confidence clamped to 0.0, got %f", spec2.Confidence)
	}
}

func TestParseVisionComponentSpecFallsBackToRawDescription(t *testing.T) {
	raw := "This looks like a task management app with a kanban layout."
	spec := parseVisionComponentSpec(raw)
	if spec == nil {
		t.Fatal("expected fallback spec for non-JSON input")
	}
	if spec.Description != raw {
		t.Errorf("expected raw description fallback, got %q", spec.Description)
	}
}

// Ensure the stub implements the interface correctly.
var _ visionSpecAnalyzer = (*stubVisionSpecAnalyzer)(nil)

func TestStubVisionSpecAnalyzerReturnsRaw(t *testing.T) {
	stub := &stubVisionSpecAnalyzer{raw: "hello"}
	got, err := stub.AnalyzeImage(context.Background(), []byte("img"), "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}

func TestStubVisionSpecAnalyzerReturnsError(t *testing.T) {
	stub := &stubVisionSpecAnalyzer{err: fmt.Errorf("api down")}
	_, err := stub.AnalyzeImage(context.Background(), nil, "")
	if err == nil || err.Error() != "api down" {
		t.Errorf("expected api down error, got %v", err)
	}
}
