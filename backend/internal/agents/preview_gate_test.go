package agents

import (
	"context"
	"strings"
	"testing"
	"time"
)

type stubPreviewVerifier struct {
	result *PreviewVerificationResult
	files  []VerifiableFile
}

func (s *stubPreviewVerifier) VerifyBuildFiles(ctx context.Context, files []VerifiableFile, isFullStack bool) *PreviewVerificationResult {
	s.files = append([]VerifiableFile(nil), files...)
	return s.result
}

func TestApplyPreviewFenceStripRepairResetsProgressAndAttempts(t *testing.T) {
	manager := &AgentManager{
		ctx:         context.Background(),
		builds:      make(map[string]*Build),
		subscribers: make(map[string][]chan *WSMessage),
	}

	now := time.Now().UTC()
	build := &Build{
		ID:        "preview-fence-strip",
		Status:    BuildCompleted,
		Progress:  100,
		UpdatedAt: now,
		Tasks: []*Task{
			{
				ID: "gen-1",
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{Path: "src/main.tsx", Content: "```tsx\ncreateRoot(document.getElementById('root')).render(<App />)\n"},
					},
				},
			},
		},
	}

	ok := manager.applyPreviewFenceStripRepair(build, nil, &PreviewVerificationResult{
		FailureKind: "corrupt_content",
		Details:     "entry file contains markdown fence",
	}, now)
	if !ok {
		t.Fatal("expected fence-strip repair to apply")
	}
	if build.Status != BuildTesting {
		t.Fatalf("expected build status testing, got %s", build.Status)
	}
	if build.Progress != 95 {
		t.Fatalf("expected build progress 95 after preview repair, got %d", build.Progress)
	}
	if build.PreviewVerificationAttempts != 1 {
		t.Fatalf("expected preview verification attempts=1, got %d", build.PreviewVerificationAttempts)
	}
	got := build.Tasks[0].Output.Files[0].Content
	if got == "" || got == "```tsx\ncreateRoot(document.getElementById('root')).render(<App />)\n" {
		t.Fatalf("expected markdown fences to be stripped, got %q", got)
	}
}

func TestRunPreviewVerificationGateTerminalFailureDropsProgressBelowCompletion(t *testing.T) {
	manager := &AgentManager{
		ctx: context.Background(),
		previewVerifier: &stubPreviewVerifier{
			result: &PreviewVerificationResult{
				Passed:      false,
				FailureKind: "missing_entrypoint",
				Details:     "No preview entrypoint found",
			},
		},
	}

	now := time.Now().UTC()
	build := &Build{
		ID:                          "preview-terminal-failure",
		Status:                      BuildCompleted,
		Progress:                    100,
		PreviewVerificationAttempts: 1,
		Tasks:                       []*Task{},
	}
	status := BuildCompleted
	buildError := ""

	if manager.runPreviewVerificationGate(build, nil, &status, &buildError, now) {
		t.Fatal("expected terminal preview failure to return false")
	}
	if status != BuildFailed {
		t.Fatalf("expected status build failed, got %s", status)
	}
	if build.Status != BuildFailed {
		t.Fatalf("expected build status failed, got %s", build.Status)
	}
	if build.Progress != 99 {
		t.Fatalf("expected terminal preview failure progress 99, got %d", build.Progress)
	}
	if buildError == "" {
		t.Fatal("expected build error to be populated")
	}
}

func TestRunPreviewVerificationGateSkipsGeneratedTestArtifacts(t *testing.T) {
	verifier := &stubPreviewVerifier{
		result: &PreviewVerificationResult{Passed: true},
	}
	manager := &AgentManager{
		ctx:             context.Background(),
		previewVerifier: verifier,
	}

	now := time.Now().UTC()
	build := &Build{
		ID:       "preview-skip-tests",
		Status:   BuildCompleted,
		Progress: 100,
	}
	status := BuildCompleted
	buildError := ""
	allFiles := []GeneratedFile{
		{Path: "src/App.tsx", Content: "export default function App() { return <div>Hello</div>; }"},
		{Path: "src/__tests__/App.test.tsx", Content: "test('works', () => {})"},
		{Path: "e2e/smoke.spec.ts", Content: "test('smoke', async () => {})"},
		{Path: "tests/integration/auth.spec.ts", Content: "test('auth', () => {})"},
	}

	if manager.runPreviewVerificationGate(build, allFiles, &status, &buildError, now) {
		t.Fatal("expected passing preview verification to continue normally")
	}
	if len(verifier.files) != 1 {
		t.Fatalf("expected only runtime-relevant files to reach preview verifier, got %+v", verifier.files)
	}
	if verifier.files[0].Path != "src/App.tsx" {
		t.Fatalf("expected src/App.tsx to remain after filtering, got %+v", verifier.files)
	}
}

func TestApplyPreviewRouterContextRepairWrapsEntryAndAddsDependency(t *testing.T) {
	manager := &AgentManager{
		ctx:         context.Background(),
		builds:      make(map[string]*Build),
		subscribers: make(map[string][]chan *WSMessage),
	}

	now := time.Now().UTC()
	build := &Build{
		ID:        "preview-router-context",
		Status:    BuildCompleted,
		Progress:  100,
		UpdatedAt: now,
		Tasks: []*Task{
			{
				ID: "gen-entry",
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "src/main.tsx",
							Content: `import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);`,
						},
						{
							Path: "package.json",
							Content: `{
  "name": "router-preview-test",
  "private": true,
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1"
  }
}`,
						},
					},
				},
			},
		},
	}

	ok := manager.applyPreviewRouterContextRepair(build, &PreviewVerificationResult{
		FailureKind: "js_runtime_error",
		Details:     "JS runtime error prevented app render: TypeError: Cannot destructure property 'basename' of 'React.useContext(...)' as it is null. at LinkWithRef",
	}, now)
	if !ok {
		t.Fatal("expected router-context repair to apply")
	}
	if build.Status != BuildTesting {
		t.Fatalf("expected build status testing, got %s", build.Status)
	}
	if build.Progress != 95 {
		t.Fatalf("expected build progress 95 after preview repair, got %d", build.Progress)
	}
	if build.PreviewVerificationAttempts != 1 {
		t.Fatalf("expected preview verification attempts=1, got %d", build.PreviewVerificationAttempts)
	}

	entry := build.Tasks[0].Output.Files[0].Content
	if !strings.Contains(entry, `import { BrowserRouter } from "react-router-dom";`) {
		t.Fatalf("expected BrowserRouter import to be added, got %q", entry)
	}
	if !strings.Contains(entry, "<BrowserRouter>") || !strings.Contains(entry, "</BrowserRouter>") {
		t.Fatalf("expected App render to be wrapped with BrowserRouter, got %q", entry)
	}

	manifest := build.Tasks[0].Output.Files[1].Content
	if !strings.Contains(manifest, `"react-router-dom"`) {
		t.Fatalf("expected react-router-dom dependency to be added, got %q", manifest)
	}
}

func TestRunPreviewVerificationGatePassingReportSupersedesEarlierFailure(t *testing.T) {
	manager := &AgentManager{
		ctx: context.Background(),
		previewVerifier: &stubPreviewVerifier{
			result: &PreviewVerificationResult{Passed: true},
		},
	}

	now := time.Now().UTC()
	build := &Build{
		ID:       "preview-pass-reconciles-history",
		Status:   BuildCompleted,
		Progress: 100,
		Plan: &BuildPlan{
			ID:           "plan-preview-pass",
			BuildID:      "preview-pass-reconciles-history",
			AppType:      "web",
			DeliveryMode: "frontend_preview_only",
			TechStack:    TechStack{Frontend: "React", Styling: "Tailwind"},
		},
	}
	orchestration := ensureBuildOrchestrationStateLocked(build)
	orchestration.BuildContract = &BuildContract{
		BuildID:      build.ID,
		AppType:      "web",
		DeliveryMode: "frontend_preview_only",
		TruthBySurface: map[string][]TruthTag{
			string(SurfaceGlobal):   {},
			string(SurfaceFrontend): {},
		},
	}
	appendVerificationReport(build, VerificationReport{
		ID:            "preview-failed",
		BuildID:       build.ID,
		Phase:         "preview_verification",
		Surface:       SurfaceGlobal,
		Status:        VerificationFailed,
		Deterministic: true,
		Blockers:      []string{"preview_verification_failed:blank_screen"},
		GeneratedAt:   now.Add(-time.Minute),
	})

	status := BuildCompleted
	buildError := ""
	if manager.runPreviewVerificationGate(build, nil, &status, &buildError, now) {
		t.Fatal("expected passing preview verification to continue without repair")
	}
	if status != BuildCompleted {
		t.Fatalf("expected completed status to remain unchanged, got %s", status)
	}
	for _, blocker := range build.SnapshotState.Blockers {
		if blocker.Type == "verification_blocker" {
			t.Fatalf("expected stale preview blocker to be cleared after a passing verification report, got %+v", blocker)
		}
	}
	reports := build.SnapshotState.Orchestration.VerificationReports
	if len(reports) < 2 {
		t.Fatalf("expected preview verification history to include pass after fail, got %+v", reports)
	}
	if reports[len(reports)-1].Status != VerificationPassed {
		t.Fatalf("expected latest preview verification report to pass, got %+v", reports[len(reports)-1])
	}
}

func TestRunPreviewVerificationGatePassingReportPreservesAdvisoryWarnings(t *testing.T) {
	manager := &AgentManager{
		ctx: context.Background(),
		previewVerifier: &stubPreviewVerifier{
			result: &PreviewVerificationResult{
				Passed:       true,
				RepairHints:  []string{"visual: increase contrast on the primary CTA"},
				CanaryErrors: []string{"interaction: TypeError: Cannot read properties of undefined"},
			},
		},
	}

	now := time.Now().UTC()
	build := &Build{
		ID:       "preview-pass-advisories",
		Status:   BuildCompleted,
		Progress: 100,
	}
	status := BuildCompleted
	buildError := ""

	if manager.runPreviewVerificationGate(build, nil, &status, &buildError, now) {
		t.Fatal("expected passing preview verification not to early-return for repair")
	}
	if len(build.SnapshotState.Orchestration.VerificationReports) == 0 {
		t.Fatal("expected a verification report to be recorded")
	}
	report := build.SnapshotState.Orchestration.VerificationReports[len(build.SnapshotState.Orchestration.VerificationReports)-1]
	if report.Status != VerificationPassed {
		t.Fatalf("expected passed verification report, got %+v", report)
	}
	if len(report.Warnings) != 2 {
		t.Fatalf("expected advisory warnings to be retained, got %+v", report.Warnings)
	}
}

func TestBuildPreviewRepairTaskInputIncludesScreenshotForVisionHints(t *testing.T) {
	result := &PreviewVerificationResult{
		FailureKind:      "blank_screen",
		Details:          "preview mounted but appears visually broken",
		ScreenshotBase64: "ZmFrZS1zY3JlZW5zaG90",
	}
	hints := []string{
		"visual: add stronger contrast between the hero copy and the background",
		"visual: style the primary CTA so it is clearly visible above the fold",
	}

	input := buildPreviewRepairTaskInput(result, hints)
	if got, _ := input["screenshot_base64"].(string); got != result.ScreenshotBase64 {
		t.Fatalf("expected screenshot_base64 to be included, got %v", input["screenshot_base64"])
	}
	if gotHints, _ := input["repair_hints"].([]string); len(gotHints) != len(hints) {
		t.Fatalf("expected repair hints to be preserved, got %#v", input["repair_hints"])
	}
}

func TestBuildPreviewRepairTaskInputOmitsScreenshotWithoutVisionHints(t *testing.T) {
	result := &PreviewVerificationResult{
		FailureKind:      "blank_screen",
		Details:          "preview mounted but appears visually broken",
		ScreenshotBase64: "ZmFrZS1zY3JlZW5zaG90",
	}
	hints := []string{"Fix the missing app shell so the preview can render."}

	input := buildPreviewRepairTaskInput(result, hints)
	if _, exists := input["screenshot_base64"]; exists {
		t.Fatalf("expected screenshot_base64 to be omitted without vision hints: %#v", input)
	}
}

func TestPreviewFailureClass(t *testing.T) {
	cases := map[string]string{
		"blank_screen":        "frontend_shell",
		"js_runtime_error":    "runtime",
		"boot_failed":         "preview_boot",
		"browser_unavailable": "infrastructure",
		"backend_no_routes":   "backend_contract",
		"something_else":      "unknown",
	}
	for kind, want := range cases {
		if got := previewFailureClass(kind); got != want {
			t.Fatalf("previewFailureClass(%q) = %q, want %q", kind, got, want)
		}
	}
}
