package agents

import (
	"context"
	"strings"
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
	if len(build.SnapshotFiles) != 1 {
		t.Fatalf("expected restored snapshot files to live on the build, got %d", len(build.SnapshotFiles))
	}
	if len(build.Tasks) != 0 {
		t.Fatalf("expected no synthetic snapshot file tasks, got %d", len(build.Tasks))
	}
}

func TestPersistBuildSnapshotRestoresActivityTimeline(t *testing.T) {
	db := openBuildTestDB(t)
	persistManager := newTestIterationManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
	})
	persistManager.db = db
	completedAt := time.Now().UTC()

	build := &Build{
		ID:          "restore-activity-build",
		UserID:      1,
		Status:      BuildCompleted,
		Mode:        ModeFull,
		PowerMode:   PowerBalanced,
		Description: "Build a telemetry-aware preview flow",
		Agents: map[string]*Agent{
			"agent-1": {
				ID:       "agent-1",
				Role:     RoleArchitect,
				Provider: ai.ProviderClaude,
				Model:    "claude-sonnet",
				Status:   StatusCompleted,
				CurrentTask: &Task{
					ID:          "task-1",
					Type:        TaskPlan,
					Description: "Plan the preview workspace",
				},
			},
		},
		Tasks: []*Task{
			{
				ID:            "task-1",
				Type:          TaskPlan,
				Description:   "Plan the preview workspace",
				AssignedTo:    "agent-1",
				Status:        TaskCompleted,
				MaxRetries:    2,
				RetryCount:    1,
				CreatedAt:     time.Now().Add(-90 * time.Second).UTC(),
				CompletedAt:   &completedAt,
				RetryStrategy: RetryWithFix,
			},
		},
		Checkpoints: []*Checkpoint{
			{
				ID:          "checkpoint-1",
				BuildID:     "restore-activity-build",
				Number:      1,
				Name:        "Plan Ready",
				Description: "Initial plan completed",
				Progress:    35,
				Restorable:  true,
				CreatedAt:   time.Now().Add(-45 * time.Second).UTC(),
			},
		},
		ActivityTimeline: []BuildActivityEntry{
			{
				ID:        "activity-1",
				AgentID:   "agent-1",
				AgentRole: string(RoleArchitect),
				Provider:  string(ai.ProviderClaude),
				Model:     "claude-sonnet",
				Type:      "thinking",
				EventType: "agent:thinking",
				TaskID:    "task-1",
				TaskType:  string(TaskPlan),
				Content:   "Planning preview handoff",
				Timestamp: time.Now().Add(-30 * time.Second).UTC(),
			},
		},
		SnapshotState: BuildSnapshotState{
			CurrentPhase:       "completed",
			QualityGateStatus:  "passed",
			QualityGateStage:   "validation",
			AvailableProviders: []string{string(ai.ProviderClaude), string(ai.ProviderGPT4)},
		},
		Progress:    100,
		CreatedAt:   time.Now().Add(-2 * time.Minute).UTC(),
		UpdatedAt:   time.Now().Add(-time.Minute).UTC(),
		CompletedAt: &completedAt,
	}
	required := true
	build.SnapshotState.QualityGateRequired = &required

	persistManager.persistBuildSnapshot(build, []GeneratedFile{})

	var snapshot models.CompletedBuild
	if err := db.Where("build_id = ?", build.ID).First(&snapshot).Error; err != nil {
		t.Fatalf("fetch snapshot: %v", err)
	}
	if snapshot.ActivityJSON == "" {
		t.Fatalf("expected activity_json to be persisted")
	}
	if snapshot.StateJSON == "" {
		t.Fatalf("expected state_json to be persisted")
	}
	if snapshot.AgentsJSON == "" || snapshot.TasksJSON == "" || snapshot.CheckpointsJSON == "" {
		t.Fatalf("expected compact snapshot state to be persisted")
	}

	restoreManager := newTestIterationManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
	})
	restoreManager.db = db

	restoredBuild, restored, err := restoreManager.restoreBuildSessionFromSnapshot(&snapshot)
	if err != nil {
		t.Fatalf("restoreBuildSessionFromSnapshot returned error: %v", err)
	}
	if !restored {
		t.Fatalf("expected restored=true")
	}
	if len(restoredBuild.ActivityTimeline) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(restoredBuild.ActivityTimeline))
	}
	if restoredBuild.SnapshotState.CurrentPhase != "completed" {
		t.Fatalf("expected restored snapshot phase completed, got %s", restoredBuild.SnapshotState.CurrentPhase)
	}
	if restoredBuild.SnapshotState.QualityGateRequired == nil || !*restoredBuild.SnapshotState.QualityGateRequired {
		t.Fatalf("expected restored quality gate requirement to be true, got %+v", restoredBuild.SnapshotState.QualityGateRequired)
	}
	if restoredBuild.SnapshotState.QualityGateStatus != "passed" {
		t.Fatalf("expected restored quality gate status passed, got %s", restoredBuild.SnapshotState.QualityGateStatus)
	}
	if len(restoredBuild.SnapshotState.AvailableProviders) != 2 {
		t.Fatalf("expected restored available providers, got %+v", restoredBuild.SnapshotState.AvailableProviders)
	}
	if restoredBuild.ActivityTimeline[0].EventType != "agent:thinking" {
		t.Fatalf("expected persisted event type, got %s", restoredBuild.ActivityTimeline[0].EventType)
	}
	if restoredBuild.ActivityTimeline[0].Content != "Planning preview handoff" {
		t.Fatalf("expected persisted content, got %s", restoredBuild.ActivityTimeline[0].Content)
	}
	if len(restoredBuild.Tasks) != 1 || restoredBuild.Tasks[0].Description != "Plan the preview workspace" {
		t.Fatalf("expected restored task metadata, got %+v", restoredBuild.Tasks)
	}
	if len(restoredBuild.Checkpoints) != 1 || restoredBuild.Checkpoints[0].Restorable {
		t.Fatalf("expected historical checkpoint metadata with restorable=false, got %+v", restoredBuild.Checkpoints)
	}
	restoredAgent := restoredBuild.Agents["agent-1"]
	if restoredAgent == nil {
		t.Fatalf("expected restored architect agent")
	}
	if restoredAgent.CurrentTask == nil || restoredAgent.CurrentTask.Description != "Plan the preview workspace" {
		t.Fatalf("expected restored agent current task, got %+v", restoredAgent.CurrentTask)
	}
}

func TestBroadcastCapturesBuildActivityTimeline(t *testing.T) {
	manager := newTestIterationManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
	})

	now := time.Now().UTC()
	build := &Build{
		ID:          "capture-activity-build",
		UserID:      1,
		Status:      BuildInProgress,
		Mode:        ModeFull,
		PowerMode:   PowerBalanced,
		Description: "Build a restore-safe timeline",
		Agents: map[string]*Agent{
			"agent-1": {
				ID:       "agent-1",
				Role:     RoleFrontend,
				Provider: ai.ProviderGPT4,
				Model:    "gpt-4.1",
			},
		},
		Tasks: []*Task{
			{
				ID:     "task-1",
				Type:   TaskGenerateUI,
				Status: TaskInProgress,
			},
		},
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-30 * time.Second),
	}
	manager.builds[build.ID] = build

	manager.broadcast(build.ID, &WSMessage{
		Type:      "agent:generating",
		BuildID:   build.ID,
		AgentID:   "agent-1",
		Timestamp: now,
		Data: map[string]any{
			"task_id":    "task-1",
			"agent_role": string(RoleFrontend),
			"provider":   string(ai.ProviderGPT4),
			"model":      "gpt-4.1",
			"content":    "Frontend agent is generating code with gpt4...",
		},
	})
	manager.broadcast(build.ID, &WSMessage{
		Type:      "build:phase",
		BuildID:   build.ID,
		Timestamp: now.Add(time.Second),
		Data: map[string]any{
			"phase_key":             "provider_check",
			"available_providers":   []string{string(ai.ProviderClaude), string(ai.ProviderGPT4)},
			"quality_gate_required": true,
		},
	})

	build.mu.RLock()
	defer build.mu.RUnlock()
	if len(build.ActivityTimeline) != 1 {
		t.Fatalf("expected 1 captured activity entry, got %d", len(build.ActivityTimeline))
	}
	entry := build.ActivityTimeline[0]
	if entry.Type != "action" {
		t.Fatalf("expected action activity type, got %s", entry.Type)
	}
	if entry.TaskID != "task-1" {
		t.Fatalf("expected captured task id, got %s", entry.TaskID)
	}
	if entry.TaskType != string(TaskGenerateUI) {
		t.Fatalf("expected inferred task type %s, got %s", TaskGenerateUI, entry.TaskType)
	}
	if entry.Provider != string(ai.ProviderGPT4) {
		t.Fatalf("expected provider gpt4, got %s", entry.Provider)
	}
	if build.SnapshotState.CurrentPhase != "provider_check" {
		t.Fatalf("expected captured current phase provider_check, got %s", build.SnapshotState.CurrentPhase)
	}
	if build.SnapshotState.QualityGateRequired == nil || !*build.SnapshotState.QualityGateRequired {
		t.Fatalf("expected captured quality gate requirement, got %+v", build.SnapshotState.QualityGateRequired)
	}
	if len(build.SnapshotState.AvailableProviders) != 2 {
		t.Fatalf("expected captured available providers, got %+v", build.SnapshotState.AvailableProviders)
	}
}

func TestRollbackToCheckpointRejectsHistoricalCheckpoint(t *testing.T) {
	manager := newTestIterationManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
	})

	build := &Build{
		ID:        "historical-checkpoint-build",
		UserID:    1,
		Status:    BuildCompleted,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		Agents:    make(map[string]*Agent),
		Tasks:     make([]*Task, 0),
		Checkpoints: []*Checkpoint{
			{
				ID:          "checkpoint-1",
				BuildID:     "historical-checkpoint-build",
				Number:      1,
				Name:        "Plan Ready",
				Description: "Imported from snapshot",
				Progress:    35,
				Restorable:  false,
				CreatedAt:   time.Now().Add(-time.Minute).UTC(),
			},
		},
		CreatedAt: time.Now().Add(-2 * time.Minute).UTC(),
		UpdatedAt: time.Now().Add(-time.Minute).UTC(),
	}
	manager.builds[build.ID] = build

	err := manager.RollbackToCheckpoint(build.ID, "checkpoint-1")
	if err == nil || !strings.Contains(err.Error(), "historical only") {
		t.Fatalf("expected historical checkpoint rollback to be rejected, got %v", err)
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
