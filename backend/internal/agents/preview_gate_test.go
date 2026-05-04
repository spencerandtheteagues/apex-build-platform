package agents

import (
	"context"
	"strings"
	"testing"
	"time"
)

type stubPreviewVerifier struct {
	result   *PreviewVerificationResult
	files    []VerifiableFile
	onVerify func([]VerifiableFile)
}

func (s *stubPreviewVerifier) VerifyBuildFiles(ctx context.Context, files []VerifiableFile, isFullStack bool) *PreviewVerificationResult {
	s.files = append([]VerifiableFile(nil), files...)
	if s.onVerify != nil {
		s.onVerify(s.files)
	}
	return s.result
}

func TestPreviewVerificationGateTimeoutScalesByPowerMode(t *testing.T) {
	t.Setenv("APEX_PREVIEW_GATE_TIMEOUT_SECONDS", "")

	if got := previewVerificationGateTimeout(PowerFast); got != 90*time.Second {
		t.Fatalf("fast timeout = %s, want 90s", got)
	}
	if got := previewVerificationGateTimeout(PowerBalanced); got != 3*time.Minute {
		t.Fatalf("balanced timeout = %s, want 3m", got)
	}
	if got := previewVerificationGateTimeout(PowerMax); got != 4*time.Minute {
		t.Fatalf("max timeout = %s, want 4m", got)
	}

	t.Setenv("APEX_PREVIEW_GATE_TIMEOUT_SECONDS", "45")
	if got := previewVerificationGateTimeout(PowerMax); got != 45*time.Second {
		t.Fatalf("env override timeout = %s, want 45s", got)
	}
}

func TestRunPreviewVerificationGateBroadcastsPreviewPhaseBeforeVerifier(t *testing.T) {
	var phaseDuringVerifier string
	var stageDuringVerifier string
	var build *Build
	manager := &AgentManager{
		ctx:         context.Background(),
		builds:      map[string]*Build{},
		subscribers: map[string][]chan *WSMessage{},
		previewVerifier: &stubPreviewVerifier{
			result: &PreviewVerificationResult{Passed: true},
			onVerify: func(_ []VerifiableFile) {
				build.mu.RLock()
				phaseDuringVerifier = build.SnapshotState.CurrentPhase
				stageDuringVerifier = build.SnapshotState.QualityGateStage
				build.mu.RUnlock()
			},
		},
	}

	now := time.Now().UTC()
	build = &Build{
		ID:        "preview-phase-telemetry",
		Status:    BuildReviewing,
		Progress:  99,
		UpdatedAt: now,
		Tasks:     []*Task{},
		Agents:    map[string]*Agent{},
	}
	manager.builds[build.ID] = build

	updates := make(chan *WSMessage, 4)
	manager.Subscribe(build.ID, updates)

	status := BuildCompleted
	buildError := ""
	if manager.runPreviewVerificationGate(build, []GeneratedFile{
		{Path: "index.html", Content: `<div id="root"></div><script type="module" src="/src/main.tsx"></script>`},
		{Path: "src/main.tsx", Content: `console.log("preview")`},
	}, &status, &buildError, now) {
		t.Fatal("expected passing preview gate to continue finalization")
	}

	if phaseDuringVerifier != "preview_verification" {
		t.Fatalf("expected preview phase before verifier started, got %q", phaseDuringVerifier)
	}
	if stageDuringVerifier != "preview_verification" {
		t.Fatalf("expected preview quality stage before verifier started, got %q", stageDuringVerifier)
	}

	select {
	case msg := <-updates:
		data, _ := msg.Data.(map[string]any)
		if got := buildActivityString(data["phase_key"]); got != "preview_verification" {
			t.Fatalf("expected preview phase broadcast, got data=%+v", data)
		}
		if got := buildActivityString(data["quality_gate_stage"]); got != "preview_verification" {
			t.Fatalf("expected preview quality stage broadcast, got data=%+v", data)
		}
	default:
		t.Fatal("expected preview phase progress broadcast")
	}
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
				ID:   "gen-1",
				Type: TaskGenerateUI,
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
	state := build.SnapshotState.Orchestration
	if state == nil || len(state.PatchBundles) == 0 {
		t.Fatalf("expected preview repair patch bundle, got %+v", state)
	}
	bundle := state.PatchBundles[len(state.PatchBundles)-1]
	if !strings.Contains(bundle.Justification, "stripped markdown fence") {
		t.Fatalf("expected fence-strip patch justification, got %+v", bundle)
	}
	if len(bundle.Operations) != 1 || bundle.Operations[0].Path != "src/main.tsx" {
		t.Fatalf("expected patch operation for repaired entry file, got %+v", bundle.Operations)
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

func TestRunPreviewVerificationGateFailsFastWhenPreviewRepairCannotQueue(t *testing.T) {
	manager := &AgentManager{
		ctx:         context.Background(),
		agents:      map[string]*Agent{},
		builds:      map[string]*Build{},
		subscribers: map[string][]chan *WSMessage{},
		previewVerifier: &stubPreviewVerifier{
			result: &PreviewVerificationResult{
				Passed:      false,
				FailureKind: "backend_missing",
				Details:     "Backend runtime missing and no solver is available",
			},
		},
	}

	now := time.Now().UTC()
	build := &Build{
		ID:        "preview-repair-no-solver",
		Status:    BuildReviewing,
		Progress:  99,
		UpdatedAt: now,
		Agents:    map[string]*Agent{},
		Tasks:     []*Task{},
	}
	manager.builds[build.ID] = build

	status := BuildCompleted
	buildError := ""
	if manager.runPreviewVerificationGate(build, nil, &status, &buildError, now) {
		t.Fatal("expected preview gate to fail fast when repair cannot be queued")
	}
	if status != BuildFailed {
		t.Fatalf("expected status build failed, got %s", status)
	}
	if build.Status != BuildFailed {
		t.Fatalf("expected build status failed, got %s", build.Status)
	}
	if build.Progress != 99 {
		t.Fatalf("expected failed preview to remain at 99 rather than fake 95 recovery, got %d", build.Progress)
	}
	if build.PreviewVerificationAttempts != 0 {
		t.Fatalf("expected no preview attempt to be counted when repair did not queue, got %d", build.PreviewVerificationAttempts)
	}
	if strings.TrimSpace(buildError) == "" || !strings.Contains(buildError, "Preview verification failed") {
		t.Fatalf("expected preview failure error, got %q", buildError)
	}
	if len(build.Tasks) != 0 {
		t.Fatalf("expected no unassigned recovery task to remain, got %+v", build.Tasks)
	}
}

func TestRunPreviewVerificationGateMarksRecoveryOnlyAfterPreviewRepairQueues(t *testing.T) {
	taskQueue := make(chan *Task, 1)
	solver := &Agent{
		ID:       "solver-preview-repair",
		Role:     RoleSolver,
		Status:   StatusIdle,
		BuildID:  "preview-repair-queues",
		Provider: "gpt4",
	}
	manager := &AgentManager{
		ctx:         context.Background(),
		agents:      map[string]*Agent{solver.ID: solver},
		builds:      map[string]*Build{},
		subscribers: map[string][]chan *WSMessage{},
		taskQueue:   taskQueue,
		previewVerifier: &stubPreviewVerifier{
			result: &PreviewVerificationResult{
				Passed:      false,
				FailureKind: "interaction_dead",
				Details:     "Preview loaded but exposes no interactive controls",
				RepairHints: []string{"Add visible buttons, links, menus, or toggles on first load."},
			},
		},
	}

	now := time.Now().UTC()
	build := &Build{
		ID:        solver.BuildID,
		Status:    BuildReviewing,
		Progress:  99,
		UpdatedAt: now,
		Agents:    map[string]*Agent{solver.ID: solver},
		Tasks:     []*Task{},
	}
	manager.builds[build.ID] = build

	status := BuildCompleted
	buildError := ""
	if !manager.runPreviewVerificationGate(build, nil, &status, &buildError, now) {
		t.Fatal("expected preview gate to pause finalization after queuing repair")
	}
	if build.Status != BuildReviewing {
		t.Fatalf("expected build status reviewing, got %s", build.Status)
	}
	if build.Progress != 95 {
		t.Fatalf("expected build progress 95 after actual repair queue, got %d", build.Progress)
	}
	if build.PreviewVerificationAttempts != 1 {
		t.Fatalf("expected preview attempts=1 after actual repair queue, got %d", build.PreviewVerificationAttempts)
	}
	if len(build.Tasks) != 1 {
		t.Fatalf("expected one queued repair task, got %+v", build.Tasks)
	}
	repairTask := build.Tasks[0]
	if repairTask.Type != TaskFix || repairTask.Status != TaskInProgress {
		t.Fatalf("expected in-progress fix task, got %+v", repairTask)
	}
	if action := taskInputStringValue(repairTask.Input, "action"); action != "fix_preview_verification" {
		t.Fatalf("expected preview repair action, got %q", action)
	}
	select {
	case queued := <-taskQueue:
		if queued.ID != repairTask.ID {
			t.Fatalf("expected queued task %s, got %s", repairTask.ID, queued.ID)
		}
	default:
		t.Fatal("expected repair task to be enqueued for execution")
	}
}

func TestRunPreviewVerificationGateUsesDeterministicShellFallbackBeforeSolverForBlankScreen(t *testing.T) {
	manager := &AgentManager{
		ctx:         context.Background(),
		agents:      map[string]*Agent{},
		builds:      map[string]*Build{},
		subscribers: map[string][]chan *WSMessage{},
		previewVerifier: &stubPreviewVerifier{
			result: &PreviewVerificationResult{
				Passed:      false,
				FailureKind: "blank_screen",
				Details:     "Preview rendered a blank screen after mounting the root.",
				RepairHints: []string{"Install a working React app shell at the root route."},
			},
		},
	}

	now := time.Now().UTC()
	build := &Build{
		ID:          "preview-blank-shell-fallback",
		Description: "Build a complete contractor field operations SaaS dashboard",
		Status:      BuildReviewing,
		Progress:    99,
		UpdatedAt:   now,
		TechStack:   &TechStack{Frontend: "React", Styling: "Tailwind"},
		Agents:      map[string]*Agent{},
		Tasks: []*Task{
			{
				ID:     "frontend",
				Type:   TaskGenerateUI,
				Status: TaskInProgress,
				Output: &TaskOutput{Files: []GeneratedFile{
					{Path: "package.json", Content: `{"scripts":{"dev":"vite","build":"vite build"},"dependencies":{"@vitejs/plugin-react":"latest","vite":"latest","typescript":"latest","react":"latest","react-dom":"latest"}}`},
					{Path: "index.html", Content: `<div id="root"></div><script type="module" src="/src/main.tsx"></script>`},
					{Path: "src/main.tsx", Content: `import React from "react"; import { createRoot } from "react-dom/client"; import App from "./App"; createRoot(document.getElementById("root")!).render(<App />);`},
					{Path: "src/App.tsx", Content: `export default function App(){ return null; }`},
				}},
			},
		},
	}
	build.SnapshotState.Orchestration = &BuildOrchestrationState{
		Flags: BuildOrchestrationFlags{EnablePatchBundles: true},
	}
	manager.builds[build.ID] = build

	status := BuildCompleted
	buildError := ""
	if !manager.runPreviewVerificationGate(build, manager.collectGeneratedFiles(build), &status, &buildError, now) {
		t.Fatal("expected preview gate to apply deterministic shell fallback and pause finalization")
	}
	if build.Status != BuildTesting {
		t.Fatalf("expected build status testing after deterministic fallback, got %s", build.Status)
	}
	if build.Progress != 95 {
		t.Fatalf("expected build progress 95 after deterministic fallback, got %d", build.Progress)
	}
	if build.PreviewVerificationAttempts != 1 {
		t.Fatalf("expected preview attempts=1 after deterministic fallback, got %d", build.PreviewVerificationAttempts)
	}
	if len(build.Tasks) != 1 {
		t.Fatalf("expected deterministic fallback not to queue solver tasks, got %+v", build.Tasks)
	}

	filesByPath := map[string]string{}
	for _, file := range manager.collectGeneratedFiles(build) {
		filesByPath[file.Path] = file.Content
	}
	app := filesByPath["src/App.tsx"]
	lowerApp := strings.ToLower(app)
	if strings.Contains(app, "return null") ||
		strings.Contains(lowerApp, "apex recovered preview") ||
		!strings.Contains(lowerApp, "contractor") ||
		!strings.Contains(lowerApp, "add live item") {
		t.Fatalf("expected blank App.tsx to be replaced by prompt-adaptive contractor app shell, got %q", app)
	}
	if strings.TrimSpace(filesByPath["src/index.css"]) == "" || strings.TrimSpace(filesByPath["vite.config.ts"]) == "" {
		t.Fatalf("expected fallback to install canonical preview scaffold files, got paths: %+v", filesByPath)
	}
	state := build.SnapshotState.Orchestration
	if state == nil || len(state.PatchBundles) == 0 {
		t.Fatalf("expected preview fallback patch bundle, got %+v", state)
	}
	bundle := state.PatchBundles[len(state.PatchBundles)-1]
	if !strings.Contains(bundle.Justification, "preview_fallback_repair") {
		t.Fatalf("expected preview fallback patch justification, got %+v", bundle)
	}
}

func TestRunPreviewVerificationGateFailsAfterFallbackStillReportsShellFailure(t *testing.T) {
	manager := &AgentManager{
		ctx:         context.Background(),
		agents:      map[string]*Agent{},
		builds:      map[string]*Build{},
		subscribers: map[string][]chan *WSMessage{},
		previewVerifier: &stubPreviewVerifier{
			result: &PreviewVerificationResult{
				Passed:      false,
				FailureKind: "blank_screen",
				Details:     "Preview still reported a blank screen after fallback.",
			},
		},
	}

	now := time.Now().UTC()
	build := &Build{
		ID:                          "preview-fallback-second-pass",
		Description:                 "Build a reliable React SaaS app",
		Status:                      BuildTesting,
		Progress:                    99,
		UpdatedAt:                   now,
		TechStack:                   &TechStack{Frontend: "React", Styling: "Tailwind"},
		PreviewVerificationAttempts: 1,
		Agents:                      map[string]*Agent{},
		Tasks: []*Task{
			{
				ID:     "frontend",
				Type:   TaskGenerateUI,
				Status: TaskCompleted,
				Output: &TaskOutput{Files: []GeneratedFile{
					{Path: "package.json", Content: `{"private":true,"scripts":{"dev":"vite","build":"vite build","preview":"vite preview"},"dependencies":{"@vitejs/plugin-react":"latest","vite":"latest","typescript":"latest","react":"latest","react-dom":"latest"}}`},
					{Path: "index.html", Content: `<div id="root"></div><script type="module" src="/src/main.tsx"></script>`},
					{Path: "src/main.tsx", Content: `import React from "react"; import { createRoot } from "react-dom/client"; import App from "./App"; createRoot(document.getElementById("root")!).render(<App />);`},
					{Path: "src/App.tsx", Content: `export default function App(){ return <main>APEX recovered preview</main>; }`},
					{Path: "src/index.css", Content: `@tailwind base; @tailwind components; @tailwind utilities;`},
					{Path: "vite.config.ts", Content: `import { defineConfig } from "vite"; import react from "@vitejs/plugin-react"; export default defineConfig({ plugins: [react()] });`},
				}},
			},
		},
	}
	build.SnapshotState.Orchestration = &BuildOrchestrationState{
		PatchBundles: []PatchBundle{
			{
				ID:            "preview-fallback",
				BuildID:       build.ID,
				Justification: "preview_fallback_repair: replaced unrepaired frontend output with validated React/Vite preview baseline",
				CreatedAt:     now,
			},
		},
	}
	manager.builds[build.ID] = build

	status := BuildCompleted
	buildError := "previous preview error"
	if manager.runPreviewVerificationGate(build, manager.collectGeneratedFiles(build), &status, &buildError, now) {
		t.Fatal("expected fallback second pass to fail terminally, not queue another repair")
	}
	if status != BuildFailed {
		t.Fatalf("expected terminal failure for recovered preview shell, got %s", status)
	}
	if build.Status != BuildFailed {
		t.Fatalf("expected build to be failed after recovered preview shell, got %s", build.Status)
	}
	if !strings.Contains(buildError, "Preview verification failed after repair attempt") {
		t.Fatalf("expected terminal preview failure, got %q", buildError)
	}
	reports := build.SnapshotState.Orchestration.VerificationReports
	if len(reports) < 1 {
		t.Fatalf("expected failed preview report, got %+v", reports)
	}
	latest := reports[len(reports)-1]
	if latest.Status != VerificationFailed {
		t.Fatalf("expected latest preview report to fail, got %+v", latest)
	}
	if !containsString(latest.Blockers, "preview_verification_failed:blank_screen") {
		t.Fatalf("expected blank-screen preview blocker, got %+v", latest.Blockers)
	}
	if build.SnapshotState.FailureTaxonomy == nil || build.SnapshotState.FailureTaxonomy.CurrentClass == "" {
		t.Fatalf("expected terminal failure taxonomy, got %+v", build.SnapshotState.FailureTaxonomy)
	}
}

func TestRunPreviewVerificationGateFailsPassedFallbackShell(t *testing.T) {
	manager := &AgentManager{
		ctx: context.Background(),
		previewVerifier: &stubPreviewVerifier{
			result: &PreviewVerificationResult{Passed: true},
		},
	}

	now := time.Now().UTC()
	build := &Build{
		ID:                          "preview-fallback-passed",
		Description:                 "Build a reliable React SaaS app",
		Status:                      BuildTesting,
		Progress:                    99,
		UpdatedAt:                   now,
		TechStack:                   &TechStack{Frontend: "React", Styling: "Tailwind"},
		PreviewVerificationAttempts: 1,
		Agents:                      map[string]*Agent{},
		Tasks: []*Task{
			{
				ID:     "frontend",
				Type:   TaskGenerateUI,
				Status: TaskCompleted,
				Output: &TaskOutput{Files: []GeneratedFile{
					{Path: "package.json", Content: `{"private":true,"scripts":{"dev":"vite","build":"vite build","preview":"vite preview"},"dependencies":{"vite":"latest","react":"latest","react-dom":"latest"}}`},
					{Path: "index.html", Content: `<div id="root"></div><script type="module" src="/src/main.tsx"></script>`},
					{Path: "src/main.tsx", Content: `import React from "react"; import { createRoot } from "react-dom/client"; import App from "./App"; createRoot(document.getElementById("root")!).render(<App />);`},
					{Path: "src/App.tsx", Content: `export default function App(){ return <main>APEX recovered preview</main>; }`},
					{Path: "src/index.css", Content: `@tailwind base; @tailwind components; @tailwind utilities;`},
					{Path: "vite.config.ts", Content: `import { defineConfig } from "vite"; export default defineConfig({});`},
				}},
			},
		},
	}
	build.SnapshotState.Orchestration = &BuildOrchestrationState{
		PatchBundles: []PatchBundle{
			{
				ID:            "preview-fallback",
				BuildID:       build.ID,
				Justification: "preview_fallback_repair: replaced unrepaired frontend output with validated React/Vite preview baseline",
				CreatedAt:     now,
			},
		},
	}
	manager.builds = map[string]*Build{build.ID: build}

	status := BuildCompleted
	buildError := ""
	if manager.runPreviewVerificationGate(build, manager.collectGeneratedFiles(build), &status, &buildError, now) {
		t.Fatal("expected recovered shell to fail terminally, not queue repair")
	}
	if status != BuildFailed || build.Status != BuildFailed {
		t.Fatalf("expected recovered shell to fail, status=%s build=%s", status, build.Status)
	}
	if !strings.Contains(buildError, "deterministic recovery shell") {
		t.Fatalf("expected recovered-shell error, got %q", buildError)
	}
	reports := build.SnapshotState.Orchestration.VerificationReports
	if len(reports) == 0 || reports[len(reports)-1].Status != VerificationFailed {
		t.Fatalf("expected failed preview report, got %+v", reports)
	}
}

func TestRunPreviewVerificationGateTerminalFailureCancelsPreviewRecoveryTasks(t *testing.T) {
	agent := &Agent{
		ID:     "solver-1",
		Role:   RoleSolver,
		Status: StatusWorking,
	}
	recoveryTask := &Task{
		ID:         "preview-repair",
		Type:       TaskFix,
		Status:     TaskInProgress,
		AssignedTo: agent.ID,
		Input: map[string]any{
			"action": "fix_preview_verification",
		},
	}
	agent.CurrentTask = recoveryTask

	manager := &AgentManager{
		ctx: context.Background(),
		previewVerifier: &stubPreviewVerifier{
			result: &PreviewVerificationResult{
				Passed:      false,
				FailureKind: "js_runtime_error",
				Details:     "preview still throws after repair",
			},
		},
	}

	now := time.Now().UTC()
	build := &Build{
		ID:                          "preview-terminal-cancel-recovery",
		Status:                      BuildCompleted,
		Progress:                    100,
		PreviewVerificationAttempts: 1,
		Agents: map[string]*Agent{
			agent.ID: agent,
		},
		Tasks: []*Task{recoveryTask},
	}
	status := BuildCompleted
	buildError := ""

	if manager.runPreviewVerificationGate(build, nil, &status, &buildError, now) {
		t.Fatal("expected terminal preview failure to return false")
	}
	if recoveryTask.Status != TaskCancelled {
		t.Fatalf("expected pending preview recovery to be cancelled, got %s", recoveryTask.Status)
	}
	if agent.CurrentTask != nil {
		t.Fatalf("expected cancelled preview recovery to be released from agent, got %+v", agent.CurrentTask)
	}
	if agent.Status != StatusIdle {
		t.Fatalf("expected agent to return to idle, got %s", agent.Status)
	}
}

func TestRunBuildFinalizationDoesNotMarkCompletedBeforePreviewPasses(t *testing.T) {
	now := time.Now().UTC()
	build := &Build{
		ID:                      "finalize-preview-order",
		UserID:                  1,
		Status:                  BuildReviewing,
		Mode:                    ModeFull,
		PowerMode:               PowerBalanced,
		Description:             "Build a previewable React app",
		Progress:                97,
		PhasedPipelineComplete:  true,
		CompileValidationPassed: true,
		Agents:                  map[string]*Agent{},
		Tasks: []*Task{
			{
				ID:     "frontend",
				Type:   TaskGenerateUI,
				Status: TaskCompleted,
				Output: &TaskOutput{Files: []GeneratedFile{
					{Path: "package.json", Content: `{"scripts":{"dev":"vite","build":"vite build"},"dependencies":{"@vitejs/plugin-react":"latest","vite":"latest","typescript":"latest","react":"latest","react-dom":"latest"}}`},
					{Path: "index.html", Content: `<div id="root"></div><script type="module" src="/src/main.tsx"></script>`},
					{Path: "src/main.tsx", Content: `import React from "react"; import { createRoot } from "react-dom/client"; import App from "./App"; createRoot(document.getElementById("root")!).render(<App />);`},
					{Path: "src/App.tsx", Content: `export default function App(){ return <main>Ready</main>; }`},
					{Path: "README.md", Content: `# Ready`},
				}},
			},
		},
		Checkpoints: []*Checkpoint{},
		CreatedAt:   now.Add(-time.Minute),
		UpdatedAt:   now,
	}

	manager := &AgentManager{
		ctx:         context.Background(),
		builds:      map[string]*Build{build.ID: build},
		subscribers: map[string][]chan *WSMessage{},
		previewVerifier: &stubPreviewVerifier{
			result: &PreviewVerificationResult{Passed: true},
			onVerify: func([]VerifiableFile) {
				build.mu.RLock()
				status := build.Status
				progress := build.Progress
				completedAt := build.CompletedAt
				build.mu.RUnlock()
				if status == BuildCompleted || progress == 100 || completedAt != nil {
					t.Fatalf("preview verifier observed premature completion: status=%s progress=%d completedAt=%v", status, progress, completedAt)
				}
			},
		},
	}

	manager.runBuildFinalization(build, manager.buildCompletionSnapshot(build))

	build.mu.RLock()
	defer build.mu.RUnlock()
	if build.Status != BuildCompleted {
		t.Fatalf("expected build to complete after preview pass, got %s", build.Status)
	}
	if build.Progress != 100 {
		t.Fatalf("expected progress 100 after preview pass, got %d", build.Progress)
	}
	if build.CompletedAt == nil {
		t.Fatal("expected completed_at after preview pass")
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
				ID:   "gen-entry",
				Type: TaskGenerateUI,
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
	if !strings.Contains(entry, "basename={window.location.pathname.match") || !strings.Contains(entry, "</BrowserRouter>") {
		t.Fatalf("expected App render to be wrapped with preview-safe BrowserRouter, got %q", entry)
	}

	manifest := build.Tasks[0].Output.Files[1].Content
	if !strings.Contains(manifest, `"react-router-dom"`) {
		t.Fatalf("expected react-router-dom dependency to be added, got %q", manifest)
	}
	state := build.SnapshotState.Orchestration
	if state == nil || len(state.PatchBundles) == 0 {
		t.Fatalf("expected preview repair patch bundle, got %+v", state)
	}
	bundle := state.PatchBundles[len(state.PatchBundles)-1]
	if !strings.Contains(bundle.Justification, "BrowserRouter") {
		t.Fatalf("expected router-context patch justification, got %+v", bundle)
	}
	foundEntry := false
	foundManifest := false
	for _, op := range bundle.Operations {
		if op.Path == "src/main.tsx" {
			foundEntry = true
		}
		if op.Path == "package.json" {
			foundManifest = true
		}
	}
	if !foundEntry || !foundManifest {
		t.Fatalf("expected patch operations for entry and package manifest, got %+v", bundle.Operations)
	}
}

func TestApplyPreviewRouterContextRepairAddsProxyBasenameToAliasRouter(t *testing.T) {
	manager := &AgentManager{
		ctx:         context.Background(),
		builds:      make(map[string]*Build),
		subscribers: make(map[string][]chan *WSMessage),
	}

	now := time.Now().UTC()
	build := &Build{
		ID:        "preview-router-alias-basename",
		Status:    BuildCompleted,
		Progress:  100,
		UpdatedAt: now,
		Tasks: []*Task{
			{
				ID:   "gen-app",
				Type: TaskGenerateUI,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "src/App.tsx",
							Content: `import { BrowserRouter as Router, Routes, Route } from "react-router-dom";

export default function App() {
  return <Router><Routes><Route path="/" element={<main>Dashboard</main>} /></Routes></Router>;
}`,
						},
						{
							Path: "package.json",
							Content: `{
  "dependencies": {
    "react": "^18.3.1",
    "react-router-dom": "^6.26.2"
  }
}`,
						},
					},
				},
			},
		},
	}

	ok := manager.applyPreviewRouterContextRepair(build, &PreviewVerificationResult{
		FailureKind: "app_route_not_found",
		Details:     `No routes matched location "/api/v1/preview/proxy/89/"`,
		RepairHints: []string{"If using react-router-dom BrowserRouter behind the Apex preview proxy, set BrowserRouter basename from window.location.pathname before '/preview/proxy/{projectID}'."},
	}, now)
	if !ok {
		t.Fatal("expected router-context repair to apply")
	}

	app := build.Tasks[0].Output.Files[0].Content
	if !strings.Contains(app, "<Router basename={window.location.pathname.match") {
		t.Fatalf("expected aliased BrowserRouter to receive preview basename, got %q", app)
	}
	if strings.Contains(app, "<Router><Routes>") {
		t.Fatalf("expected Router opening tag to be rewritten, got %q", app)
	}
	if build.PreviewVerificationAttempts != 1 {
		t.Fatalf("expected preview verification attempts=1, got %d", build.PreviewVerificationAttempts)
	}
	if !strings.Contains(build.Error, "BrowserRouter basename") {
		t.Fatalf("expected basename repair message, got %q", build.Error)
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
	if len(build.SnapshotState.Orchestration.FailureFingerprints) != 2 {
		t.Fatalf("expected advisory fingerprints to be recorded, got %+v", build.SnapshotState.Orchestration.FailureFingerprints)
	}
	fingerprints := build.SnapshotState.Orchestration.FailureFingerprints
	if fingerprints[0].FailureClass != "visual_layout" || !fingerprints[0].RepairSucceeded {
		t.Fatalf("expected visual advisory fingerprint, got %+v", fingerprints[0])
	}
	if fingerprints[1].FailureClass != "interaction_canary" || !fingerprints[1].RepairSucceeded {
		t.Fatalf("expected interaction advisory fingerprint, got %+v", fingerprints[1])
	}
	if build.SnapshotState.FailureTaxonomy != nil && build.SnapshotState.FailureTaxonomy.CurrentClass != "" {
		t.Fatalf("expected passed advisory verification not to set current failure taxonomy, got %+v", build.SnapshotState.FailureTaxonomy)
	}
}

func TestInteractionCriticalSignalRequiresProvenCriticalCanarySignal(t *testing.T) {
	advisoryOnly := &PreviewVerificationResult{
		Passed:                true,
		CanaryErrors:          []string{"interaction: TypeError: Cannot read properties of undefined"},
		CanaryVisibleControls: 0,
	}
	if interactionCriticalSignal(advisoryOnly) {
		t.Fatalf("expected advisory-only canary warning not to block preview completion")
	}

	postClickBlank := &PreviewVerificationResult{
		Passed:                       true,
		CanaryClickCount:             2,
		CanaryVisibleControls:        3,
		CanaryPostInteractionChecked: true,
		CanaryPostInteractionHealthy: false,
	}
	if !interactionCriticalSignal(postClickBlank) {
		t.Fatalf("expected a checked post-interaction blank preview to trigger repair")
	}

	noControls := &PreviewVerificationResult{
		Passed:                true,
		CanaryVisibleControls: 0,
		RepairHints: []string{
			"interaction: The preview mounted but exposes no visible buttons, links, menus, or toggles on first load.",
		},
	}
	if !interactionCriticalSignal(noControls) {
		t.Fatalf("expected explicit no-visible-controls canary hint to trigger repair")
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

	input := buildPreviewRepairTaskInput(nil, result, hints)
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

	input := buildPreviewRepairTaskInput(nil, result, hints)
	if _, exists := input["screenshot_base64"]; exists {
		t.Fatalf("expected screenshot_base64 to be omitted without vision hints: %#v", input)
	}
}

func TestFilterVisualCriticalHints(t *testing.T) {
	criticalHints := []string{
		"visual: Fix Tailwind CSS — blank screen detected",
		"visual: Address dark-on-dark contrast failure",
		"visual: App shows no styling, browser defaults only",
		"visual: completely blank screen — no content visible",
	}
	advisoryHints := []string{
		"visual: Increase spacing between cards",
		"visual: Navigation bar could be more prominent",
		"interaction: button click throws JS error",
		"Fix TypeScript error in App.tsx",
	}
	all := append(criticalHints, advisoryHints...)

	got := filterVisualCriticalHints(all)
	if len(got) != len(criticalHints) {
		t.Errorf("filterVisualCriticalHints returned %d hints, want %d\ngot: %v", len(got), len(criticalHints), got)
	}
	for _, h := range got {
		lower := strings.ToLower(h)
		lower = strings.TrimPrefix(lower, "visual:")
		if !isVisualCriticalHintText(strings.TrimSpace(lower)) {
			t.Errorf("unexpected non-critical hint in result: %q", h)
		}
	}
}

func TestFilterVisualCriticalHintsEmpty(t *testing.T) {
	advisoryOnly := []string{
		"visual: spacing could be improved",
		"interaction: click error",
	}
	got := filterVisualCriticalHints(advisoryOnly)
	if len(got) != 0 {
		t.Errorf("expected empty result for advisory-only hints, got %v", got)
	}
}

func TestSummarizeHints(t *testing.T) {
	hints := []string{"A", "B", "C"}
	if got := summarizeHints(hints, 2); got != "A; B" {
		t.Errorf("summarizeHints(2) = %q, want %q", got, "A; B")
	}
	if got := summarizeHints(hints, 0); got != "A; B; C" {
		t.Errorf("summarizeHints(0) = %q, want %q", got, "A; B; C")
	}
	if got := summarizeHints(nil, 2); got != "" {
		t.Errorf("summarizeHints(nil) = %q, want empty", got)
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
