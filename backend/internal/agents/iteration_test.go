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
		agents:        make(map[string]*Agent),
		builds:        make(map[string]*Build),
		taskQueue:     make(chan *Task, 8),
		resultQueue:   make(chan *TaskResult, 8),
		subscribers:   make(map[string][]chan *WSMessage),
		buildMonitors: make(map[string]struct{}),
		aiRouter:      router,
		ctx:           context.Background(),
		cancel:        func() {},
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
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

func TestRecoverStaleBuildsOnStartupPreservesActiveSnapshots(t *testing.T) {
	db := openBuildTestDB(t)
	now := time.Now().UTC()
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "stale-active-build",
		UserID:      1,
		Description: "Interrupted build should stay resumable",
		Status:      "in_progress",
		Mode:        "full",
		PowerMode:   "balanced",
		Progress:    92,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("create active snapshot: %v", err)
	}

	manager := newTestIterationManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
	})
	manager.db = db

	recovered, err := manager.RecoverStaleBuildsOnStartup()
	if err != nil {
		t.Fatalf("RecoverStaleBuildsOnStartup returned error: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("expected 1 resumable stale build, got %d", recovered)
	}

	var snapshot models.CompletedBuild
	if err := db.Where("build_id = ?", "stale-active-build").First(&snapshot).Error; err != nil {
		t.Fatalf("fetch snapshot: %v", err)
	}
	if snapshot.Status != "in_progress" {
		t.Fatalf("expected snapshot status to remain in_progress, got %s", snapshot.Status)
	}
	if strings.TrimSpace(snapshot.Error) != "" {
		t.Fatalf("expected snapshot error to remain empty, got %q", snapshot.Error)
	}
}

func TestNormalizeRestoredBuildStatusTreatsInterruptedFailuresAsResumable(t *testing.T) {
	snapshot := &models.CompletedBuild{
		BuildID:     "interrupted-build",
		UserID:      1,
		Description: "Interrupted by restart",
		Status:      "failed",
		Error:       interruptedBuildRecoveryError,
		StateJSON:   `{"current_phase":"review"}`,
	}

	if got := normalizeRestoredBuildStatus(snapshot); got != BuildReviewing {
		t.Fatalf("normalizeRestoredBuildStatus = %s, want %s", got, BuildReviewing)
	}
}

func TestNormalizeRestoredBuildStatusTreatsLegacyBuildingAsResumable(t *testing.T) {
	snapshot := &models.CompletedBuild{
		BuildID: "legacy-building-build",
		Status:  "building",
	}

	if got := normalizeRestoredBuildStatus(snapshot); got != BuildInProgress {
		t.Fatalf("normalizeRestoredBuildStatus = %s, want %s", got, BuildInProgress)
	}
}

func TestMarkBuildTerminalSuccessSnapshotSetsPassedQualityGateState(t *testing.T) {
	manager := newTestIterationManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
	})

	build := &Build{
		ID:               "terminal-success-snapshot",
		UserID:           1,
		Status:           BuildCompleted,
		Mode:             ModeFull,
		PowerMode:        PowerBalanced,
		SubscriptionPlan: "builder",
		ProviderMode:     "platform",
		Description:      "Build a preview-first CRM app",
		Plan: &BuildPlan{
			ID:           "plan-terminal-success",
			BuildID:      "terminal-success-snapshot",
			AppType:      "web",
			DeliveryMode: "frontend_preview_only",
			TechStack:    TechStack{Frontend: "React", Styling: "Tailwind"},
		},
		Interaction: BuildInteractionState{
			PendingQuestion: "Are you happy with this frontend preview?",
			WaitingForUser:  true,
		},
		CreatedAt: time.Now().Add(-time.Minute).UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	ensureBuildOrchestrationStateLocked(build)

	manager.markBuildTerminalSuccessSnapshot(build, "complete")

	build.mu.RLock()
	defer build.mu.RUnlock()

	if build.SnapshotState.CurrentPhase != "completed" {
		t.Fatalf("expected current phase completed, got %q", build.SnapshotState.CurrentPhase)
	}
	if build.SnapshotState.QualityGateRequired == nil || !*build.SnapshotState.QualityGateRequired {
		t.Fatalf("expected quality gate required=true, got %+v", build.SnapshotState.QualityGateRequired)
	}
	if build.SnapshotState.QualityGateStatus != "passed" {
		t.Fatalf("expected quality gate status passed, got %q", build.SnapshotState.QualityGateStatus)
	}
	if build.SnapshotState.QualityGateStage != "complete" {
		t.Fatalf("expected quality gate stage complete, got %q", build.SnapshotState.QualityGateStage)
	}
	foundUserReplyBlocker := false
	for _, blocker := range build.SnapshotState.Blockers {
		if blocker.ID == "pending-user-reply" {
			foundUserReplyBlocker = true
			break
		}
	}
	if !foundUserReplyBlocker {
		t.Fatalf("expected derived blockers to include the waiting user reply, got %+v", build.SnapshotState.Blockers)
	}
}

func TestPersistBuildSnapshotDoesNotOverwriteNewerTerminalSnapshot(t *testing.T) {
	db := openBuildTestDB(t)
	manager := newTestIterationManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
	})
	manager.db = db

	now := time.Now().UTC()
	completedAt := now.Add(-10 * time.Second)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "snapshot-order-build",
		UserID:      1,
		Description: "Newest successful snapshot",
		Status:      string(BuildCompleted),
		Mode:        string(ModeFull),
		PowerMode:   string(PowerFast),
		Progress:    100,
		Error:       "",
		CreatedAt:   now.Add(-2 * time.Minute),
		UpdatedAt:   now,
		CompletedAt: &completedAt,
	}).Error; err != nil {
		t.Fatalf("create completed snapshot: %v", err)
	}

	build := &Build{
		ID:          "snapshot-order-build",
		UserID:      1,
		Status:      BuildFailed,
		Mode:        ModeFull,
		PowerMode:   PowerFast,
		Description: "Older failed write should not win",
		Progress:    93,
		Error:       "stale failure",
		Agents:      map[string]*Agent{},
		Tasks:       []*Task{},
		Checkpoints: []*Checkpoint{},
		CreatedAt:   now.Add(-3 * time.Minute),
		UpdatedAt:   now.Add(-time.Minute),
	}

	manager.persistBuildSnapshot(build, nil)

	var snapshot models.CompletedBuild
	if err := db.Where("build_id = ?", build.ID).First(&snapshot).Error; err != nil {
		t.Fatalf("fetch snapshot: %v", err)
	}
	if snapshot.Status != string(BuildCompleted) {
		t.Fatalf("expected newer completed snapshot to survive, got %s", snapshot.Status)
	}
	if snapshot.Progress != 100 {
		t.Fatalf("expected newer progress 100 to survive, got %d", snapshot.Progress)
	}
	if strings.TrimSpace(snapshot.Error) != "" {
		t.Fatalf("expected completed snapshot error to remain empty, got %q", snapshot.Error)
	}
}

func TestPersistBuildSnapshotDoesNotDowngradeTerminalSuccessWhenFailureUpdatedAtIsLater(t *testing.T) {
	db := openBuildTestDB(t)
	manager := newTestIterationManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
	})
	manager.db = db

	now := time.Now().UTC()
	completedAt := now.Add(-10 * time.Second)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "snapshot-success-lock-build",
		UserID:      1,
		Description: "Terminal success should not be downgraded",
		Status:      string(BuildCompleted),
		Mode:        string(ModeFull),
		PowerMode:   string(PowerBalanced),
		Progress:    100,
		Error:       "",
		CreatedAt:   now.Add(-2 * time.Minute),
		UpdatedAt:   now,
		CompletedAt: &completedAt,
	}).Error; err != nil {
		t.Fatalf("create completed snapshot: %v", err)
	}

	build := &Build{
		ID:          "snapshot-success-lock-build",
		UserID:      1,
		Status:      BuildFailed,
		Mode:        ModeFull,
		PowerMode:   PowerBalanced,
		Description: "Later failed write should lose to terminal success",
		Progress:    97,
		Error:       "preview verification failed after repair attempt",
		Agents:      map[string]*Agent{},
		Tasks:       []*Task{},
		Checkpoints: []*Checkpoint{},
		CreatedAt:   now.Add(-3 * time.Minute),
		UpdatedAt:   now.Add(15 * time.Second),
	}

	manager.persistBuildSnapshot(build, nil)

	var snapshot models.CompletedBuild
	if err := db.Where("build_id = ?", build.ID).First(&snapshot).Error; err != nil {
		t.Fatalf("fetch snapshot: %v", err)
	}
	if snapshot.Status != string(BuildCompleted) {
		t.Fatalf("expected terminal success to survive, got %s", snapshot.Status)
	}
	if snapshot.Progress != 100 {
		t.Fatalf("expected progress 100 to survive, got %d", snapshot.Progress)
	}
	if strings.TrimSpace(snapshot.Error) != "" {
		t.Fatalf("expected terminal success error to remain empty, got %q", snapshot.Error)
	}
	if snapshot.CompletedAt == nil {
		t.Fatalf("expected completed_at to remain populated")
	}
}

func TestPersistBuildSnapshotAllowsTerminalSuccessToOverwriteStaleFailureEvenWhenSuccessUpdatedAtIsEarlier(t *testing.T) {
	db := openBuildTestDB(t)
	manager := newTestIterationManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
	})
	manager.db = db

	now := time.Now().UTC()
	failureCompletedAt := now.Add(-5 * time.Second)
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "snapshot-self-heal-build",
		UserID:      1,
		Description: "Late stale failure should yield to live completion",
		Status:      string(BuildFailed),
		Mode:        string(ModeFast),
		PowerMode:   string(PowerMax),
		Progress:    97,
		Error:       "preview verification failed after repair attempt",
		CreatedAt:   now.Add(-2 * time.Minute),
		UpdatedAt:   now.Add(15 * time.Second),
		CompletedAt: &failureCompletedAt,
	}).Error; err != nil {
		t.Fatalf("create stale failure snapshot: %v", err)
	}

	successCompletedAt := now.Add(10 * time.Second)
	build := &Build{
		ID:          "snapshot-self-heal-build",
		UserID:      1,
		Status:      BuildCompleted,
		Mode:        ModeFast,
		PowerMode:   PowerMax,
		Description: "Live terminal completion should win",
		Progress:    100,
		Error:       "",
		Agents:      map[string]*Agent{},
		Tasks:       []*Task{},
		Checkpoints: []*Checkpoint{},
		CreatedAt:   now.Add(-3 * time.Minute),
		UpdatedAt:   now,
		CompletedAt: &successCompletedAt,
	}

	manager.persistBuildSnapshot(build, nil)

	var snapshot models.CompletedBuild
	if err := db.Where("build_id = ?", build.ID).First(&snapshot).Error; err != nil {
		t.Fatalf("fetch snapshot: %v", err)
	}
	if snapshot.Status != string(BuildCompleted) {
		t.Fatalf("expected live success to overwrite stale failure, got %s", snapshot.Status)
	}
	if snapshot.Progress != 100 {
		t.Fatalf("expected progress 100 after overwrite, got %d", snapshot.Progress)
	}
	if strings.TrimSpace(snapshot.Error) != "" {
		t.Fatalf("expected stale failure error to clear, got %q", snapshot.Error)
	}
	if snapshot.CompletedAt == nil {
		t.Fatalf("expected completed_at to be restored")
	}
}

func TestRestoreBuildSessionFromSnapshotRequeuesInterruptedTasks(t *testing.T) {
	db := openBuildTestDB(t)
	manager := newTestIterationManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
	})
	manager.db = db

	now := time.Now().Add(-2 * time.Minute).UTC()
	snapshot := &models.CompletedBuild{
		BuildID:     "restore-requeue-build",
		UserID:      1,
		Description: "Resume a build after restart",
		Status:      "failed",
		Error:       interruptedBuildRecoveryError,
		Mode:        "full",
		PowerMode:   "balanced",
		Progress:    92,
		StateJSON:   `{"current_phase":"review"}`,
		AgentsJSON: `[{
			"id":"lead-1",
			"role":"lead",
			"provider":"claude",
			"model":"claude-sonnet-4-6",
			"status":"working",
			"build_id":"restore-requeue-build",
			"current_task":{"id":"task-review","type":"review","description":"Run the final review"},
			"progress":92,
			"created_at":"2026-03-22T00:00:00Z",
			"updated_at":"2026-03-22T00:00:00Z"
		},{
			"id":"reviewer-1",
			"role":"reviewer",
			"provider":"claude",
			"model":"claude-sonnet-4-6",
			"status":"working",
			"build_id":"restore-requeue-build",
			"current_task":{"id":"task-review","type":"review","description":"Run the final review"},
			"progress":92,
			"created_at":"2026-03-22T00:00:00Z",
			"updated_at":"2026-03-22T00:00:00Z"
		}]`,
		TasksJSON: `[{
			"id":"task-review",
			"type":"review",
			"description":"Run the final review",
			"priority":90,
			"assigned_to":"reviewer-1",
			"status":"in_progress",
			"created_at":"2026-03-22T00:00:00Z",
			"started_at":"2026-03-22T00:00:00Z",
			"max_retries":2
		}]`,
		CreatedAt: now,
		UpdatedAt: now.Add(time.Minute),
	}

	build, restored, err := manager.restoreBuildSessionFromSnapshot(snapshot)
	if err != nil {
		t.Fatalf("restoreBuildSessionFromSnapshot returned error: %v", err)
	}
	if !restored {
		t.Fatalf("expected restored=true")
	}
	if build.Status != BuildReviewing {
		t.Fatalf("expected interrupted failed build to restore into reviewing, got %s", build.Status)
	}
	if manager.agents["reviewer-1"] == nil {
		t.Fatalf("expected restored reviewer agent to be registered")
	}

	select {
	case resumed := <-manager.taskQueue:
		if resumed == nil {
			t.Fatalf("expected requeued task, got nil")
		}
		if resumed.ID != "task-review" {
			t.Fatalf("expected task-review to be requeued, got %s", resumed.ID)
		}
		if resumed.AssignedTo != "reviewer-1" {
			t.Fatalf("expected requeued task assigned to reviewer-1, got %s", resumed.AssignedTo)
		}
	default:
		t.Fatalf("expected restored task to be requeued")
	}
}

func TestRestoreBuildSessionFromSnapshotPreservesRuntimeAndTaskState(t *testing.T) {
	db := openBuildTestDB(t)
	persistManager := newTestIterationManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4},
		userProviders: []ai.AIProvider{ai.ProviderGPT4},
	})
	persistManager.db = db

	now := time.Now().UTC()
	build := &Build{
		ID:                          "restore-runtime-build",
		UserID:                      42,
		Status:                      BuildReviewing,
		Mode:                        ModeFull,
		PowerMode:                   PowerMax,
		SubscriptionPlan:            "team",
		ProviderMode:                "byok",
		RequirePreviewReady:         true,
		Description:                 "Resume the orchestration state faithfully",
		TechStack:                   &TechStack{Frontend: "React", Backend: "Go", Database: "PostgreSQL", Extras: []string{"Redis"}},
		Plan:                        &BuildPlan{ID: "plan-1", BuildID: "restore-runtime-build", AppType: "fullstack", TechStack: TechStack{Frontend: "React", Backend: "Go"}},
		Progress:                    88,
		MaxAgents:                   6,
		MaxRetries:                  4,
		MaxRequests:                 17,
		MaxTokensPerRequest:         4096,
		RequestsUsed:                5,
		ReadinessRecoveryAttempts:   2,
		PreviewVerificationAttempts: 1,
		PhasedPipelineComplete:      true,
		DiffMode:                    true,
		RoleAssignments:             map[string]string{"architect": "claude", "frontend": "gpt4"},
		Tasks: []*Task{
			{
				ID:            "task-fix",
				Type:          TaskFix,
				Description:   "Patch the failing preview verification",
				Priority:      90,
				AssignedTo:    "solver-1",
				Status:        TaskInProgress,
				Input:         map[string]any{"action": "fix_tests", "path": "src/App.tsx"},
				Output:        &TaskOutput{Messages: []string{"retrying preview"}, Suggestions: []string{"install missing package"}},
				CreatedAt:     now.Add(-2 * time.Minute),
				StartedAt:     ptrTime(now.Add(-90 * time.Second)),
				Error:         "preview failed",
				RetryCount:    2,
				MaxRetries:    5,
				ErrorHistory:  []ErrorAttempt{{AttemptNumber: 1, Error: "missing dependency", Timestamp: now.Add(-80 * time.Second), Analysis: "add test dependency"}},
				RetryStrategy: RetryWithFix,
			},
		},
		CreatedAt: now.Add(-4 * time.Minute),
		UpdatedAt: now.Add(-30 * time.Second),
	}

	persistManager.persistBuildSnapshot(build, []GeneratedFile{{Path: "src/App.tsx", Content: "export default function App(){return null}", Language: "typescript", Size: 42, IsNew: true}})

	var snapshot models.CompletedBuild
	if err := db.Where("build_id = ?", build.ID).First(&snapshot).Error; err != nil {
		t.Fatalf("fetch snapshot: %v", err)
	}

	restoreManager := newTestIterationManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4},
		userProviders: []ai.AIProvider{ai.ProviderGPT4},
	})
	restoreManager.db = db

	restoredBuild, restored, err := restoreManager.restoreBuildSessionFromSnapshot(&snapshot)
	if err != nil {
		t.Fatalf("restoreBuildSessionFromSnapshot returned error: %v", err)
	}
	if !restored {
		t.Fatalf("expected restored=true")
	}
	if restoredBuild.ProviderMode != "byok" {
		t.Fatalf("expected provider mode byok, got %s", restoredBuild.ProviderMode)
	}
	if restoredBuild.SubscriptionPlan != "team" {
		t.Fatalf("expected subscription plan team, got %s", restoredBuild.SubscriptionPlan)
	}
	if !restoredBuild.RequirePreviewReady {
		t.Fatalf("expected require_preview_ready to persist")
	}
	if restoredBuild.RequestsUsed != 5 {
		t.Fatalf("expected RequestsUsed=5, got %d", restoredBuild.RequestsUsed)
	}
	if restoredBuild.ReadinessRecoveryAttempts != 2 {
		t.Fatalf("expected ReadinessRecoveryAttempts=2, got %d", restoredBuild.ReadinessRecoveryAttempts)
	}
	if restoredBuild.PreviewVerificationAttempts != 1 {
		t.Fatalf("expected PreviewVerificationAttempts=1, got %d", restoredBuild.PreviewVerificationAttempts)
	}
	if restoredBuild.MaxRequests != 17 || restoredBuild.MaxTokensPerRequest != 4096 {
		t.Fatalf("expected restored max request limits, got max_requests=%d max_tokens=%d", restoredBuild.MaxRequests, restoredBuild.MaxTokensPerRequest)
	}
	if !restoredBuild.DiffMode || !restoredBuild.PhasedPipelineComplete {
		t.Fatalf("expected diff/phased flags to persist, got diff=%v phased=%v", restoredBuild.DiffMode, restoredBuild.PhasedPipelineComplete)
	}
	if restoredBuild.TechStack == nil || restoredBuild.TechStack.Backend != "Go" {
		t.Fatalf("expected tech stack to restore, got %+v", restoredBuild.TechStack)
	}
	if restoredBuild.Plan == nil || restoredBuild.Plan.AppType != "fullstack" {
		t.Fatalf("expected plan to restore, got %+v", restoredBuild.Plan)
	}
	if restoredBuild.RoleAssignments["frontend"] != "gpt4" {
		t.Fatalf("expected role assignments to restore, got %+v", restoredBuild.RoleAssignments)
	}
	if len(restoredBuild.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(restoredBuild.Tasks))
	}
	restoredTask := restoredBuild.Tasks[0]
	if restoredTask.Input["action"] != "fix_tests" {
		t.Fatalf("expected task input to restore, got %+v", restoredTask.Input)
	}
	if restoredTask.Output == nil || len(restoredTask.Output.Messages) != 1 || restoredTask.Output.Messages[0] != "retrying preview" {
		t.Fatalf("expected task output to restore, got %+v", restoredTask.Output)
	}
	if len(restoredTask.ErrorHistory) != 1 || restoredTask.ErrorHistory[0].Error != "missing dependency" {
		t.Fatalf("expected error history to restore, got %+v", restoredTask.ErrorHistory)
	}
}

func TestResumeBuildRequeuesPausedRestoredTasks(t *testing.T) {
	manager := newTestIterationManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
	})

	snapshot := &models.CompletedBuild{
		BuildID:     "restore-paused-build",
		UserID:      1,
		Description: "Resume a paused restored build",
		Status:      "in_progress",
		Mode:        "full",
		PowerMode:   "balanced",
		Progress:    71,
		InteractionJSON: `{
			"paused": true,
			"pause_reason": "Waiting for user"
		}`,
		AgentsJSON: `[{
			"id":"solver-1",
			"role":"solver",
			"provider":"claude",
			"model":"claude-sonnet-4-6",
			"status":"working",
			"build_id":"restore-paused-build",
			"current_task":{"id":"task-fix","type":"fix","description":"Repair the preview build"},
			"progress":71,
			"created_at":"2026-03-22T00:00:00Z",
			"updated_at":"2026-03-22T00:00:00Z"
		}]`,
		TasksJSON: `[{
			"id":"task-fix",
			"type":"fix",
			"description":"Repair the preview build",
			"priority":70,
			"assigned_to":"solver-1",
			"status":"pending",
			"created_at":"2026-03-22T00:00:00Z",
			"max_retries":3
		}]`,
		CreatedAt: time.Now().Add(-2 * time.Minute).UTC(),
		UpdatedAt: time.Now().Add(-time.Minute).UTC(),
	}

	build, restored, err := manager.restoreBuildSessionFromSnapshotWithOptions(snapshot, restoreBuildSessionOptions{resumeExecution: false})
	if err != nil {
		t.Fatalf("restoreBuildSessionFromSnapshotWithOptions returned error: %v", err)
	}
	if !restored {
		t.Fatalf("expected restored=true")
	}
	select {
	case task := <-manager.taskQueue:
		t.Fatalf("expected paused restore not to queue tasks, got %+v", task)
	default:
	}

	interaction, err := manager.ResumeBuild(build.ID, "continue")
	if err != nil {
		t.Fatalf("ResumeBuild returned error: %v", err)
	}
	if interaction.Paused {
		t.Fatalf("expected paused flag to clear on resume")
	}

	select {
	case resumed := <-manager.taskQueue:
		if resumed == nil || resumed.ID != "task-fix" {
			t.Fatalf("expected task-fix to be requeued, got %+v", resumed)
		}
	default:
		t.Fatalf("expected resumed task to be queued")
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

func TestBroadcastCapturesGlassBoxActivityEvents(t *testing.T) {
	manager := newTestIterationManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude},
		userProviders: []ai.AIProvider{ai.ProviderClaude},
	})

	build := &Build{
		ID:        "capture-glassbox-build",
		UserID:    1,
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		Agents:    map[string]*Agent{},
		Tasks: []*Task{
			{
				ID:     "task-1",
				Type:   TaskGenerateAPI,
				Status: TaskInProgress,
			},
		},
		CreatedAt: time.Now().Add(-time.Minute),
		UpdatedAt: time.Now(),
	}
	manager.builds[build.ID] = build

	manager.broadcast(build.ID, &WSMessage{
		Type:      WSGlassWorkOrderCompiled,
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"agent_role":       "planner",
			"provider":         "orchestrator",
			"work_order_count": 2,
			"content":          "2 work order(s) compiled for execution.",
		},
	})
	manager.broadcast(build.ID, &WSMessage{
		Type:      WSGlassDeterministicGateFailed,
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"agent_role": "verifier",
			"provider":   "deterministic_gate",
			"task_id":    "task-1",
			"task_type":  string(TaskGenerateAPI),
			"content":    "Deterministic gate failed for backend generation.",
		},
	})
	manager.broadcast(build.ID, &WSMessage{
		Type:      WSGlassHydraWinnerSelected,
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"agent_role": "repair",
			"provider":   string(ai.ProviderClaude),
			"strategy":   "strict_ast_syntax_repair",
		},
	})

	build.mu.RLock()
	defer build.mu.RUnlock()
	if len(build.ActivityTimeline) != 3 {
		t.Fatalf("expected 3 captured glass-box entries, got %d", len(build.ActivityTimeline))
	}
	if build.ActivityTimeline[0].EventType != string(WSGlassWorkOrderCompiled) || build.ActivityTimeline[0].Type != "action" {
		t.Fatalf("expected work-order action event, got %+v", build.ActivityTimeline[0])
	}
	if build.ActivityTimeline[1].EventType != string(WSGlassDeterministicGateFailed) || build.ActivityTimeline[1].Type != "error" {
		t.Fatalf("expected deterministic gate error event, got %+v", build.ActivityTimeline[1])
	}
	if build.ActivityTimeline[1].TaskType != string(TaskGenerateAPI) {
		t.Fatalf("expected task type on deterministic event, got %s", build.ActivityTimeline[1].TaskType)
	}
	if build.ActivityTimeline[2].EventType != string(WSGlassHydraWinnerSelected) || build.ActivityTimeline[2].Type != "output" {
		t.Fatalf("expected hydra winner output event, got %+v", build.ActivityTimeline[2])
	}
	if build.ActivityTimeline[2].Content == "" {
		t.Fatalf("expected hydra winner content")
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

func TestSendMessageAffirmingFrontendApprovalQueuesBackendContinuation(t *testing.T) {
	for _, mode := range []PowerMode{PowerBalanced, PowerMax} {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			router := &stubPreflight{
				configured:    true,
				allProviders:  []ai.AIProvider{ai.ProviderClaude},
				userProviders: []ai.AIProvider{ai.ProviderClaude},
			}
			manager := newTestIterationManager(router)

			build := &Build{
				ID:               "frontend-approval-build-" + string(mode),
				UserID:           7,
				Status:           BuildCompleted,
				Mode:             ModeFull,
				PowerMode:        mode,
				SubscriptionPlan: "builder",
				Description:      "Build a full-stack CRM with a polished dashboard, auth, projects, and reporting.",
				Agents:           make(map[string]*Agent),
				Tasks:            []*Task{},
				Plan: &BuildPlan{
					AppType:      "fullstack",
					DeliveryMode: "frontend_preview_only",
					TechStack:    TechStack{Frontend: "React", Backend: "Express", Database: "PostgreSQL", Styling: "Tailwind"},
				},
				CreatedAt: time.Now().Add(-time.Minute).UTC(),
				UpdatedAt: time.Now().Add(-time.Second).UTC(),
				Interaction: BuildInteractionState{
					PendingQuestion: frontendApprovalPrompt,
					WaitingForUser:  true,
				},
			}
			lead := &Agent{
				ID:        "lead-approval-" + string(mode),
				Role:      RoleLead,
				Provider:  ai.ProviderClaude,
				Model:     "claude-sonnet",
				Status:    StatusWorking,
				BuildID:   build.ID,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}
			solver := &Agent{
				ID:        "solver-approval-" + string(mode),
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

			if err := manager.SendMessageWithClientToken(build.ID, "Yes, continue with the backend now.", "token-1"); err != nil {
				t.Fatalf("SendMessageWithClientToken returned error: %v", err)
			}

			if got := router.generateCalls.Load(); got != 0 {
				t.Fatalf("expected deterministic approval path to skip AI intervention, got %d calls", got)
			}
			if build.Status != BuildInProgress {
				t.Fatalf("expected build to re-open for backend continuation, got %s", build.Status)
			}
			if build.Plan == nil || build.Plan.DeliveryMode != "full_stack_preview" {
				t.Fatalf("expected plan delivery mode to promote back to full_stack_preview, got %+v", build.Plan)
			}
			if build.Interaction.WaitingForUser || strings.TrimSpace(build.Interaction.PendingQuestion) != "" {
				t.Fatalf("expected pending approval question to clear, got %+v", build.Interaction)
			}

			var continuationTask *Task
			select {
			case continuationTask = <-manager.taskQueue:
			default:
				t.Fatal("expected backend continuation task to be queued")
			}
			if continuationTask == nil || continuationTask.Type != TaskFix {
				t.Fatalf("expected follow-up fix task, got %+v", continuationTask)
			}
			if action, _ := continuationTask.Input["action"].(string); action != "user_change_request" {
				t.Fatalf("expected user_change_request action, got %+v", continuationTask.Input["action"])
			}
			if request, _ := continuationTask.Input["user_request"].(string); !strings.Contains(strings.ToLower(request), "backend") {
				t.Fatalf("expected backend continuation prompt, got %q", request)
			}
		})
	}
}
