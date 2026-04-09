package agents

import (
	"context"
	"encoding/base64"
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
