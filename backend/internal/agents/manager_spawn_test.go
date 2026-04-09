package agents

import (
	"os"
	"testing"
	"time"
)

func TestInitialAgentRolesForBuildUsesWorkOrders(t *testing.T) {
	build := &Build{
		Plan: &BuildPlan{
			WorkOrders: []BuildWorkOrder{
				{Role: RoleArchitect},
				{Role: RoleFrontend},
				{Role: RoleTesting},
				{Role: RoleReviewer},
				{Role: RoleSolver},
			},
		},
	}

	roles := initialAgentRolesForBuild(build)
	expected := []AgentRole{RoleArchitect, RoleFrontend, RoleTesting, RoleReviewer}
	if len(roles) != len(expected) {
		t.Fatalf("expected %d roles, got %d (%v)", len(expected), len(roles), roles)
	}
	for i, role := range expected {
		if roles[i] != role {
			t.Fatalf("expected role %d to be %s, got %s", i, role, roles[i])
		}
	}
}

func TestInitialAgentRolesForBuildFallsBackToLegacyRolesWithoutPlan(t *testing.T) {
	roles := initialAgentRolesForBuild(&Build{})
	expected := []AgentRole{
		RoleArchitect,
		RoleFrontend,
		RoleDatabase,
		RoleBackend,
		RoleTesting,
		RoleReviewer,
	}
	if len(roles) != len(expected) {
		t.Fatalf("expected %d roles, got %d (%v)", len(expected), len(roles), roles)
	}
	for i, role := range expected {
		if roles[i] != role {
			t.Fatalf("expected role %d to be %s, got %s", i, role, roles[i])
		}
	}
}

func TestBuildExecutionPhasesPrefersFrontendBeforeBackendAndData(t *testing.T) {
	t.Setenv("APEX_PARALLEL_MID_PHASE", "false")

	phases := buildExecutionPhases(
		[]agentPriority{{agent: &Agent{Role: RoleArchitect}}},
		[]agentPriority{{agent: &Agent{Role: RoleFrontend}}},
		[]agentPriority{{agent: &Agent{Role: RoleDatabase}}},
		[]agentPriority{{agent: &Agent{Role: RoleBackend}}},
		[]agentPriority{{agent: &Agent{Role: RoleTesting}}},
		[]agentPriority{{agent: &Agent{Role: RoleReviewer}}},
	)

	expectedKeys := []string{"architecture", "frontend_ui", "data_foundation", "backend_services", "integration", "review"}
	if len(phases) != len(expectedKeys) {
		t.Fatalf("expected %d phases, got %d", len(expectedKeys), len(phases))
	}
	for i, key := range expectedKeys {
		if phases[i].key != key {
			t.Fatalf("expected phase %d key %q, got %q", i, key, phases[i].key)
		}
	}
	if phases[1].startMessage == "" || phases[3].completionMessage == "" {
		t.Fatalf("expected user-facing phase messages to be populated, got %+v", phases)
	}
	if phases[0].startMessage == "" || phases[0].completionMessage == "" {
		t.Fatalf("expected architecture phase to narrate contract freeze, got %+v", phases[0])
	}
}

func TestBuildExecutionPhasesParallelMidPhaseEnabledMergesCoreAgents(t *testing.T) {
	t.Setenv("APEX_PARALLEL_MID_PHASE", "true")

	frontend := agentPriority{agent: &Agent{Role: RoleFrontend}}
	database := agentPriority{agent: &Agent{Role: RoleDatabase}}
	backend := agentPriority{agent: &Agent{Role: RoleBackend}}

	phases := buildExecutionPhases(
		[]agentPriority{{agent: &Agent{Role: RoleArchitect}}},
		[]agentPriority{frontend},
		[]agentPriority{database},
		[]agentPriority{backend},
		[]agentPriority{{agent: &Agent{Role: RoleTesting}}},
		[]agentPriority{{agent: &Agent{Role: RoleReviewer}}},
	)

	expectedKeys := []string{"architecture", "parallel_core", "integration", "review"}
	if len(phases) != len(expectedKeys) {
		t.Fatalf("expected %d phases, got %d", len(expectedKeys), len(phases))
	}
	for i, key := range expectedKeys {
		if phases[i].key != key {
			t.Fatalf("expected phase %d key %q, got %q", i, key, phases[i].key)
		}
	}
	if len(phases[1].agents) != 3 {
		t.Fatalf("expected parallel core to merge frontend/database/backend agents, got %+v", phases[1].agents)
	}
	if phases[1].startMessage == "" || phases[1].completionMessage == "" {
		t.Fatalf("expected parallel_core user-facing messages, got %+v", phases[1])
	}
}

func TestBuildPhaseProgressWindowIncludesParallelCore(t *testing.T) {
	min, max, ok := buildPhaseProgressWindow("parallel_core", BuildInProgress)
	if !ok {
		t.Fatal("expected progress window for parallel_core")
	}
	if min != 20 || max != 79 {
		t.Fatalf("expected parallel_core window 20-79, got %d-%d", min, max)
	}
}

func TestSetBuildPhaseSnapshotPersistsCurrentPhaseForRestores(t *testing.T) {
	build := &Build{
		Status: BuildPlanning,
	}
	phase := executionPhase{
		key:          "frontend_ui",
		status:       BuildInProgress,
		qualityStage: "testing",
	}
	now := time.Now().UTC()

	setBuildPhaseSnapshot(build, phase, now)

	if build.Status != BuildInProgress {
		t.Fatalf("expected build status %s, got %s", BuildInProgress, build.Status)
	}
	if build.SnapshotState.CurrentPhase != "frontend_ui" {
		t.Fatalf("expected current phase frontend_ui, got %q", build.SnapshotState.CurrentPhase)
	}
	if build.SnapshotState.QualityGateStage != "testing" {
		t.Fatalf("expected quality gate stage testing, got %q", build.SnapshotState.QualityGateStage)
	}
	if build.SnapshotState.QualityGateStatus != "running" {
		t.Fatalf("expected quality gate status running, got %q", build.SnapshotState.QualityGateStatus)
	}
	if build.SnapshotState.QualityGateRequired == nil || !*build.SnapshotState.QualityGateRequired {
		t.Fatalf("expected quality gate to be required, got %+v", build.SnapshotState.QualityGateRequired)
	}
	if !build.UpdatedAt.Equal(now) {
		t.Fatalf("expected updated_at %s, got %s", now, build.UpdatedAt)
	}
}

func TestBuildTimeoutForBuildGivesFullstackBuildsMoreHeadroomByDefault(t *testing.T) {
	original, hadOriginal := os.LookupEnv("BUILD_TIMEOUT_FULL_SECONDS")
	if err := os.Unsetenv("BUILD_TIMEOUT_FULL_SECONDS"); err != nil {
		t.Fatalf("failed to unset build timeout env: %v", err)
	}
	t.Cleanup(func() {
		if hadOriginal {
			_ = os.Setenv("BUILD_TIMEOUT_FULL_SECONDS", original)
		} else {
			_ = os.Unsetenv("BUILD_TIMEOUT_FULL_SECONDS")
		}
	})

	am := &AgentManager{}
	build := &Build{
		Mode: ModeFull,
		Plan: &BuildPlan{
			AppType: "fullstack",
		},
	}

	timeout := am.buildTimeoutForBuild(build)
	if timeout < 30*time.Minute {
		t.Fatalf("expected fullstack full build timeout to be at least 30m, got %s", timeout)
	}
}

func TestBuildTimeoutForBuildHonorsExplicitEnvOverride(t *testing.T) {
	t.Setenv("BUILD_TIMEOUT_FULL_SECONDS", "600")

	am := &AgentManager{}
	build := &Build{
		Mode: ModeFull,
		Plan: &BuildPlan{
			AppType: "fullstack",
		},
	}

	timeout := am.buildTimeoutForBuild(build)
	if timeout != 10*time.Minute {
		t.Fatalf("expected explicit timeout override to win, got %s", timeout)
	}
}

func TestFilterAgentRolesPreservesOrder(t *testing.T) {
	roles := []AgentRole{RoleArchitect, RoleDatabase, RoleBackend, RoleFrontend, RoleTesting, RoleReviewer}
	filtered := filterAgentRoles(roles, map[AgentRole]bool{
		RoleArchitect: true,
		RoleFrontend:  true,
		RoleReviewer:  true,
	})
	expected := []AgentRole{RoleArchitect, RoleFrontend, RoleReviewer}
	if len(filtered) != len(expected) {
		t.Fatalf("expected %d roles, got %d (%v)", len(expected), len(filtered), filtered)
	}
	for i, role := range expected {
		if filtered[i] != role {
			t.Fatalf("expected role %d to be %s, got %s", i, role, filtered[i])
		}
	}
}

func TestResumeBuildExecutionRequeuesPendingRecoveryTasksAndRefreshesTimestamp(t *testing.T) {
	oldUpdatedAt := time.Now().Add(-10 * time.Minute).UTC()
	build := &Build{
		ID:        "build-1",
		Status:    BuildReviewing,
		UpdatedAt: oldUpdatedAt,
		Agents: map[string]*Agent{
			"testing-1": {ID: "testing-1", BuildID: "build-1", Role: RoleTesting},
		},
		Tasks: []*Task{
			{
				ID:          "task-1",
				Type:        TaskTest,
				Description: "Regression test after automated fixes",
				Status:      TaskPending,
				Input: map[string]any{
					"action": "regression_test",
				},
			},
		},
	}
	am := &AgentManager{
		builds: map[string]*Build{
			build.ID: build,
		},
		agents: map[string]*Agent{
			"testing-1": build.Agents["testing-1"],
		},
		taskQueue: make(chan *Task, 1),
	}

	am.resumeBuildExecution(build, false)

	task := build.Tasks[0]
	if task.Status != TaskInProgress {
		t.Fatalf("expected task to be requeued as in-progress, got %s", task.Status)
	}
	if task.AssignedTo != "testing-1" {
		t.Fatalf("expected task to be assigned to testing-1, got %s", task.AssignedTo)
	}
	if !build.UpdatedAt.After(oldUpdatedAt) {
		t.Fatalf("expected build updated_at to advance after requeue, old=%s new=%s", oldUpdatedAt, build.UpdatedAt)
	}

	select {
	case queued := <-am.taskQueue:
		if queued.ID != task.ID {
			t.Fatalf("expected queued task %s, got %s", task.ID, queued.ID)
		}
	default:
		t.Fatalf("expected task to be pushed back onto the queue")
	}
}

func TestFailBuildOnPhaseAbortMarksBuildFailed(t *testing.T) {
	build := &Build{
		ID:        "build-phase-abort",
		Status:    BuildTesting,
		Mode:      ModeFull,
		PowerMode: PowerFast,
		Progress:  78,
		Agents:    map[string]*Agent{},
		Tasks: []*Task{
			{ID: "task-testing", Type: TaskTest, Status: TaskInProgress},
			{ID: "task-review", Type: TaskReview, Status: TaskPending},
		},
	}
	am := &AgentManager{
		builds:      map[string]*Build{build.ID: build},
		subscribers: map[string][]chan *WSMessage{},
	}

	am.failBuildOnPhaseAbort(build, "Testing", BuildTesting, []string{"task-testing", "task-review"})

	if build.Status != BuildFailed {
		t.Fatalf("expected build to be failed, got %s", build.Status)
	}
	if build.CompletedAt == nil {
		t.Fatal("expected completed_at to be set")
	}
	if build.Tasks[0].Status != TaskCancelled || build.Tasks[1].Status != TaskCancelled {
		t.Fatalf("expected aborted phase tasks to be cancelled, got %+v", build.Tasks)
	}
	if build.Error == "" || build.Error == "cancelled by user" {
		t.Fatalf("expected explicit phase-abort error, got %q", build.Error)
	}
}

func TestRecoverStalledPhasedExecutionStartsNextPhaseAfterFrontend(t *testing.T) {
	oldUpdatedAt := time.Now().Add(-2 * time.Minute).UTC()
	build := &Build{
		ID:          "build-stalled-frontend-phase",
		Description: "Build a multi-tenant agency operations platform",
		Status:      BuildInProgress,
		UpdatedAt:   oldUpdatedAt,
		Agents: map[string]*Agent{
			"architect-1": {ID: "architect-1", BuildID: "build-stalled-frontend-phase", Role: RoleArchitect, Status: StatusCompleted},
			"frontend-1":  {ID: "frontend-1", BuildID: "build-stalled-frontend-phase", Role: RoleFrontend, Status: StatusCompleted},
			"database-1":  {ID: "database-1", BuildID: "build-stalled-frontend-phase", Role: RoleDatabase, Status: StatusIdle},
			"backend-1":   {ID: "backend-1", BuildID: "build-stalled-frontend-phase", Role: RoleBackend, Status: StatusIdle},
			"testing-1":   {ID: "testing-1", BuildID: "build-stalled-frontend-phase", Role: RoleTesting, Status: StatusIdle},
			"review-1":    {ID: "review-1", BuildID: "build-stalled-frontend-phase", Role: RoleReviewer, Status: StatusIdle},
		},
		Tasks: []*Task{
			{
				ID:          "task-architecture",
				Type:        TaskArchitecture,
				Description: "Lock the contract",
				AssignedTo:  "architect-1",
				Status:      TaskCompleted,
				CreatedAt:   oldUpdatedAt.Add(-2 * time.Minute),
				CompletedAt: ptrTimeSpawn(oldUpdatedAt.Add(-90 * time.Second)),
			},
			{
				ID:          "task-frontend",
				Type:        TaskGenerateUI,
				Description: "Build the frontend UI",
				AssignedTo:  "frontend-1",
				Status:      TaskCompleted,
				CreatedAt:   oldUpdatedAt.Add(-90 * time.Second),
				CompletedAt: ptrTimeSpawn(oldUpdatedAt.Add(-60 * time.Second)),
			},
		},
		SnapshotState: BuildSnapshotState{
			CurrentPhase: "frontend_ui",
		},
	}

	am := &AgentManager{
		builds:      map[string]*Build{build.ID: build},
		agents:      map[string]*Agent{},
		taskQueue:   make(chan *Task, 4),
		resultQueue: make(chan *TaskResult, 1),
		subscribers: map[string][]chan *WSMessage{},
	}
	for id, agent := range build.Agents {
		am.agents[id] = agent
	}

	if ok := am.recoverStalledPhasedExecution(build); !ok {
		t.Fatal("expected stalled phased execution to recover")
	}
	if build.SnapshotState.CurrentPhase != "data_foundation" {
		t.Fatalf("expected build to move to data_foundation, got %q", build.SnapshotState.CurrentPhase)
	}
	if build.Status != BuildInProgress {
		t.Fatalf("expected build to remain in progress, got %s", build.Status)
	}
	if !build.UpdatedAt.After(oldUpdatedAt) {
		t.Fatalf("expected updated_at to advance, old=%s new=%s", oldUpdatedAt, build.UpdatedAt)
	}
	if len(build.Tasks) != 3 {
		t.Fatalf("expected recovery to append the data foundation task, got %d tasks", len(build.Tasks))
	}

	nextTask := build.Tasks[len(build.Tasks)-1]
	if nextTask.Type != TaskGenerateSchema {
		t.Fatalf("expected next task type %s, got %s", TaskGenerateSchema, nextTask.Type)
	}
	if nextTask.AssignedTo != "database-1" {
		t.Fatalf("expected next phase task to be assigned to database-1, got %q", nextTask.AssignedTo)
	}
	if nextTask.Status != TaskInProgress {
		t.Fatalf("expected next phase task to be in progress, got %s", nextTask.Status)
	}
	if build.Agents["database-1"].Status != StatusWorking {
		t.Fatalf("expected database agent to be working, got %s", build.Agents["database-1"].Status)
	}

	select {
	case queued := <-am.taskQueue:
		if queued.ID != nextTask.ID {
			t.Fatalf("expected queued task %s, got %s", nextTask.ID, queued.ID)
		}
	default:
		t.Fatal("expected recovered phase task to be queued")
	}
}

func ptrTimeSpawn(t time.Time) *time.Time {
	return &t
}
