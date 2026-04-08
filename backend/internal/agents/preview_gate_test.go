package agents

import (
	"context"
	"strings"
	"testing"
	"time"
)

type stubPreviewVerifier struct {
	result *PreviewVerificationResult
}

func (s *stubPreviewVerifier) VerifyBuildFiles(ctx context.Context, files []VerifiableFile, isFullStack bool) *PreviewVerificationResult {
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
