package agents

import (
	"context"
	"testing"
	"time"

	"apex-build/internal/ai"
	"apex-build/pkg/models"
)

func newTestIterationManager(router AIRouter) *AgentManager {
	return &AgentManager{
		agents:      make(map[string]*Agent),
		builds:      make(map[string]*Build),
		taskQueue:   make(chan *Task, 8),
		resultQueue: make(chan *TaskResult, 8),
		subscribers: make(map[string][]chan *WSMessage),
		aiRouter:    router,
		ctx:         context.Background(),
		cancel:      func() {},
	}
}

func TestRestoreBuildSessionFromSnapshotRehydratesFiles(t *testing.T) {
	db := openBuildTestDB(t)
	manager := newTestIterationManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
	})
	manager.db = db

	snapshot := &models.CompletedBuild{
		BuildID:     "restore-files-build",
		UserID:      1,
		Description: "Build a project tracker",
		Status:      "completed",
		Mode:        "full",
		PowerMode:   "balanced",
		Progress:    100,
		FilesJSON:   `[{"path":"src/App.tsx","content":"export default function App(){return null}","language":"typescript","size":43,"is_new":true}]`,
		CreatedAt:   time.Now().Add(-2 * time.Minute).UTC(),
		UpdatedAt:   time.Now().Add(-1 * time.Minute).UTC(),
	}

	build, restored, err := manager.restoreBuildSessionFromSnapshot(snapshot)
	if err != nil {
		t.Fatalf("restoreBuildSessionFromSnapshot returned error: %v", err)
	}
	if !restored {
		t.Fatalf("expected restored=true")
	}

	files := manager.collectGeneratedFiles(build)
	if len(files) != 1 {
		t.Fatalf("expected 1 restored file, got %d", len(files))
	}
	if files[0].Path != "src/App.tsx" {
		t.Fatalf("expected restored file path src/App.tsx, got %s", files[0].Path)
	}
	if len(build.Tasks) == 0 || build.Tasks[0].Output == nil || len(build.Tasks[0].Output.Files) != 1 {
		t.Fatalf("expected restored snapshot files to be rehydrated into task output")
	}
}

func TestProcessUserMessageCountsBuildBudgetRequests(t *testing.T) {
	router := &stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
		generateResult: &ai.AIResponse{
			Content: `{"reply":"On it","apply_changes":false}`,
		},
	}
	manager := newTestIterationManager(router)

	build := &Build{
		ID:          "budget-count-build",
		UserID:      1,
		Status:      BuildInProgress,
		Mode:        ModeFull,
		PowerMode:   PowerBalanced,
		Description: "Build a CRM app",
		Agents:      make(map[string]*Agent),
		Tasks: []*Task{
			{
				ID:         "done-task",
				Type:       TaskGenerateUI,
				Status:     TaskCompleted,
				Output:     &TaskOutput{},
				Input:      map[string]any{},
				CreatedAt:  time.Now().UTC(),
				MaxRetries: 1,
			},
		},
		MaxRequests: 5,
		CreatedAt:   time.Now().Add(-time.Minute).UTC(),
		UpdatedAt:   time.Now().Add(-time.Second).UTC(),
		Interaction: BuildInteractionState{},
	}
	lead := &Agent{
		ID:        "lead-1",
		Role:      RoleLead,
		Provider:  ai.ProviderClaude,
		Model:     "claude-sonnet",
		Status:    StatusWorking,
		BuildID:   build.ID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	build.Agents[lead.ID] = lead
	manager.builds[build.ID] = build
	manager.agents[lead.ID] = lead

	manager.processUserMessage(lead, "Tighten the dashboard spacing")

	if got := build.RequestsUsed; got != 1 {
		t.Fatalf("expected RequestsUsed=1 after intervention, got %d", got)
	}
	if got := router.generateCalls.Load(); got != 1 {
		t.Fatalf("expected 1 AI call, got %d", got)
	}
}

func TestProcessUserMessageStopsAtBuildRequestBudget(t *testing.T) {
	router := &stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
		generateResult: &ai.AIResponse{
			Content: `{"reply":"On it","apply_changes":false}`,
		},
	}
	manager := newTestIterationManager(router)

	build := &Build{
		ID:           "budget-limit-build",
		UserID:       1,
		Status:       BuildInProgress,
		Mode:         ModeFull,
		PowerMode:    PowerBalanced,
		Description:  "Build a CRM app",
		Agents:       make(map[string]*Agent),
		Tasks:        []*Task{},
		MaxRequests:  1,
		RequestsUsed: 1,
		CreatedAt:    time.Now().Add(-time.Minute).UTC(),
		UpdatedAt:    time.Now().Add(-time.Second).UTC(),
		Interaction:  BuildInteractionState{},
	}
	lead := &Agent{
		ID:        "lead-2",
		Role:      RoleLead,
		Provider:  ai.ProviderClaude,
		Model:     "claude-sonnet",
		Status:    StatusWorking,
		BuildID:   build.ID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	build.Agents[lead.ID] = lead
	manager.builds[build.ID] = build
	manager.agents[lead.ID] = lead

	manager.processUserMessage(lead, "Add export to CSV")

	if got := router.generateCalls.Load(); got != 0 {
		t.Fatalf("expected no AI call after budget exhaustion, got %d", got)
	}
	if got := build.RequestsUsed; got != 1 {
		t.Fatalf("expected RequestsUsed to remain capped at 1, got %d", got)
	}
}

func TestQueuedUserRevisionDispatchesBeforeFinalCompletion(t *testing.T) {
	router := &stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
		generateResult: &ai.AIResponse{
			Content: `{"reply":"Queued for the next pass.","apply_changes":true,"requires_user_response":false}`,
		},
	}
	manager := newTestIterationManager(router)

	build := &Build{
		ID:                     "queued-revision-build",
		UserID:                 1,
		Status:                 BuildInProgress,
		Mode:                   ModeFull,
		PowerMode:              PowerBalanced,
		Description:            "Build a CRM app",
		Agents:                 make(map[string]*Agent),
		Tasks:                  []*Task{},
		MaxRequests:            6,
		PhasedPipelineComplete: true,
		CreatedAt:              time.Now().Add(-time.Minute).UTC(),
		UpdatedAt:              time.Now().Add(-time.Second).UTC(),
		Interaction:            BuildInteractionState{},
	}

	codeTask := &Task{
		ID:          "code-task",
		Type:        TaskGenerateUI,
		Status:      TaskCompleted,
		Output:      &TaskOutput{Files: []GeneratedFile{{Path: "src/App.tsx", Content: "export default function App(){return null}", Language: "typescript", Size: 43, IsNew: true}}},
		Input:       map[string]any{},
		CreatedAt:   time.Now().Add(-30 * time.Second).UTC(),
		CompletedAt: func() *time.Time { now := time.Now().Add(-20 * time.Second).UTC(); return &now }(),
		MaxRetries:  1,
	}
	build.Tasks = append(build.Tasks, codeTask)

	lead := &Agent{
		ID:        "lead-3",
		Role:      RoleLead,
		Provider:  ai.ProviderClaude,
		Model:     "claude-sonnet",
		Status:    StatusWorking,
		BuildID:   build.ID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	solver := &Agent{
		ID:        "solver-1",
		Role:      RoleSolver,
		Provider:  ai.ProviderClaude,
		Model:     "claude-sonnet",
		Status:    StatusIdle,
		BuildID:   build.ID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	build.Agents[lead.ID] = lead
	build.Agents[solver.ID] = solver
	manager.builds[build.ID] = build
	manager.agents[lead.ID] = lead
	manager.agents[solver.ID] = solver

	manager.processUserMessage(lead, "Make the dashboard cards denser and add CSV export")

	if len(build.Interaction.PendingRevisions) != 1 {
		t.Fatalf("expected 1 queued revision, got %d", len(build.Interaction.PendingRevisions))
	}

	manager.checkBuildCompletion(build)

	if len(build.Interaction.PendingRevisions) != 0 {
		t.Fatalf("expected queued revisions to dispatch before completion")
	}

	foundRevisionTask := false
	for _, task := range build.Tasks {
		if task == nil || task.Type != TaskFix {
			continue
		}
		if action, _ := task.Input["action"].(string); action == "user_change_request" {
			foundRevisionTask = true
			break
		}
	}
	if !foundRevisionTask {
		t.Fatalf("expected queued revision to create a follow-up fix task")
	}
	if build.Status != BuildInProgress {
		t.Fatalf("expected build to stay active for follow-up revision, got %s", build.Status)
	}
}
