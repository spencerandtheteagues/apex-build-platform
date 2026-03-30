package agents

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"apex-build/internal/ai"
)

type consensusRetryRouter struct {
	providers []ai.AIProvider
}

func (s *consensusRetryRouter) Generate(context.Context, ai.AIProvider, string, GenerateOptions) (*ai.AIResponse, error) {
	return &ai.AIResponse{
		Content: "VOTE: switch_provider\nRATIONALE: transient timeout, retry on another provider",
	}, nil
}

func (s *consensusRetryRouter) GetAvailableProviders() []ai.AIProvider {
	return append([]ai.AIProvider(nil), s.providers...)
}

func (s *consensusRetryRouter) GetAvailableProvidersForUser(userID uint) []ai.AIProvider {
	_ = userID
	return s.GetAvailableProviders()
}

func (s *consensusRetryRouter) HasConfiguredProviders() bool {
	return len(s.providers) > 0
}

func TestIsNonRetriableAIErrorMessageAuthQuotaBilling(t *testing.T) {
	am := &AgentManager{}

	tests := []struct {
		name   string
		errMsg string
		want   bool
	}{
		{name: "insufficient credits", errMsg: "INSUFFICIENT_CREDITS from provider", want: true},
		{name: "invalid api key", errMsg: "OpenAI: invalid api key provided", want: true},
		{name: "authentication failed", errMsg: "Anthropic authentication failed", want: true},
		{name: "quota exhausted", errMsg: "quota exhausted for this project", want: true},
		{name: "billing", errMsg: "billing hard limit reached", want: true},
		{name: "rate limit", errMsg: "429 rate limit exceeded", want: false},
		{name: "chat endpoint mismatch", errMsg: "This is not a chat model and not supported in the v1/chat/completions endpoint", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := am.isNonRetriableAIErrorMessage(tc.errMsg); got != tc.want {
				t.Fatalf("isNonRetriableAIErrorMessage(%q) = %v, want %v", tc.errMsg, got, tc.want)
			}
		})
	}
}

func TestLatestFailedTaskErrorLockedPrefersLatestTaskErrorThenHistory(t *testing.T) {
	now := time.Now()
	build := &Build{
		Tasks: []*Task{
			{ID: "1", Status: TaskCompleted},
			{
				ID:     "2",
				Status: TaskFailed,
				ErrorHistory: []ErrorAttempt{
					{AttemptNumber: 1, Error: "older parse error", Timestamp: now},
					{AttemptNumber: 2, Error: "newest parse error", Timestamp: now},
				},
			},
			{
				ID:     "3",
				Status: TaskFailed,
				Error:  "terminal insufficient credits",
			},
		},
	}

	if got := latestFailedTaskErrorLocked(build); got != "terminal insufficient credits" {
		t.Fatalf("latestFailedTaskErrorLocked() = %q, want terminal task error", got)
	}

	build.Tasks[2].Error = ""
	if got := latestFailedTaskErrorLocked(build); got != "newest parse error" {
		t.Fatalf("latestFailedTaskErrorLocked() fallback = %q, want newest parse error", got)
	}
}

func TestFailBuildOnStallMarksFailedAndCancelsActiveTasks(t *testing.T) {
	am := &AgentManager{
		builds:      make(map[string]*Build),
		subscribers: make(map[string][]chan *WSMessage),
	}

	build := &Build{
		ID:        "build-stall-test",
		UserID:    1,
		Status:    BuildReviewing,
		Progress:  82,
		CreatedAt: time.Now().Add(-5 * time.Minute),
		UpdatedAt: time.Now().Add(-4 * time.Minute),
		Tasks: []*Task{
			{
				ID:     "t1",
				Type:   TaskFix,
				Status: TaskPending,
				Input:  map[string]any{"action": "fix_review_issues"},
			},
			{
				ID:     "t2",
				Type:   TaskReview,
				Status: TaskInProgress,
				Input:  map[string]any{"action": "post_fix_review"},
			},
			{
				ID:     "t3",
				Type:   TaskGenerateFile,
				Status: TaskCompleted,
			},
		},
	}
	am.builds[build.ID] = build

	am.failBuildOnStall(build.ID, BuildReviewing, 3*time.Minute, 1, 1, 2)

	if build.Status != BuildFailed {
		t.Fatalf("build status = %s, want failed", build.Status)
	}
	if build.CompletedAt == nil {
		t.Fatalf("expected CompletedAt to be set")
	}
	if build.Tasks[0].Status != TaskCancelled || build.Tasks[1].Status != TaskCancelled {
		t.Fatalf("expected active tasks to be cancelled, got %s and %s", build.Tasks[0].Status, build.Tasks[1].Status)
	}
	if build.Tasks[2].Status != TaskCompleted {
		t.Fatalf("expected completed task to remain completed, got %s", build.Tasks[2].Status)
	}
	if build.Error == "" {
		t.Fatalf("expected explicit stall error to be populated")
	}
	if len(build.Checkpoints) == 0 {
		t.Fatalf("expected a checkpoint to be created for stalled build")
	}
}

func TestRelatedPhaseTaskIDsIncludesRecoveryAndValidationDescendants(t *testing.T) {
	roots := map[string]struct{}{"phase-task": {}}
	tasks := []*Task{
		{
			ID:     "phase-task",
			Status: TaskCancelled,
			Input: map[string]any{
				"superseded_by_recovery": "recovery-task",
			},
		},
		{
			ID:     "recovery-task",
			Status: TaskCompleted,
			Input: map[string]any{
				"failed_task_id": "phase-task",
			},
		},
		{
			ID:     "validation-task",
			Status: TaskPending,
			Input: map[string]any{
				"trigger_task": "recovery-task",
			},
		},
	}

	related := relatedPhaseTaskIDs(tasks, roots)

	for _, taskID := range []string{"phase-task", "recovery-task", "validation-task"} {
		if _, ok := related[taskID]; !ok {
			t.Fatalf("expected related phase task set to include %s", taskID)
		}
	}
}

func TestWaitForPhaseCompletionWaitsForRecoveryLineage(t *testing.T) {
	manager := &AgentManager{
		ctx:         context.Background(),
		builds:      make(map[string]*Build),
		subscribers: make(map[string][]chan *WSMessage),
	}

	build := &Build{
		ID:     "phase-lineage-wait",
		Status: BuildInProgress,
		Tasks: []*Task{
			{
				ID:     "phase-task",
				Status: TaskCancelled,
				Input: map[string]any{
					"superseded_by_recovery": "recovery-task",
				},
			},
			{
				ID:     "recovery-task",
				Status: TaskInProgress,
				Input: map[string]any{
					"failed_task_id": "phase-task",
				},
			},
		},
	}

	go func() {
		time.Sleep(700 * time.Millisecond)
		build.mu.Lock()
		build.Tasks[1].Status = TaskCompleted
		now := time.Now()
		build.Tasks[1].CompletedAt = &now
		build.UpdatedAt = now
		build.mu.Unlock()
	}()

	if ok := manager.waitForPhaseCompletion(build, []string{"phase-task"}); !ok {
		t.Fatal("expected phase completion to wait for recovery lineage and then succeed")
	}
}

func TestWaitForPhaseCompletionFailsOnUnresolvedLineageFailure(t *testing.T) {
	manager := &AgentManager{
		ctx:         context.Background(),
		builds:      make(map[string]*Build),
		subscribers: make(map[string][]chan *WSMessage),
	}

	build := &Build{
		ID:     "phase-lineage-fail",
		Status: BuildInProgress,
		Tasks: []*Task{
			{
				ID:     "phase-task",
				Status: TaskFailed,
			},
		},
	}

	start := time.Now()
	if ok := manager.waitForPhaseCompletion(build, []string{"phase-task"}); ok {
		t.Fatal("expected phase completion to abort on unresolved failed phase task")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("expected unresolved failure to abort quickly, took %v", time.Since(start))
	}
}

func TestHandleReviewCompletionLinksFixTaskToTriggerTask(t *testing.T) {
	manager := &AgentManager{}
	sourceTask := &Task{ID: "review-root", Type: TaskReview, Status: TaskCompleted}
	build := &Build{
		ID:          "review-link-build",
		Description: "Review lineage should keep follow-on fix tasks in phase scope",
		Tasks:       []*Task{sourceTask},
	}

	manager.handleReviewCompletion(build, sourceTask, &TaskOutput{
		Messages: []string{"Critical security vulnerability found in auth middleware"},
	})

	if len(build.Tasks) != 2 {
		t.Fatalf("expected follow-on fix task to be appended, got %d tasks", len(build.Tasks))
	}
	fixTask := build.Tasks[1]
	if fixTask == nil || fixTask.Type != TaskFix {
		t.Fatalf("expected appended task to be a fix task, got %+v", fixTask)
	}
	if got := taskInputStringValue(fixTask.Input, "trigger_task"); got != sourceTask.ID {
		t.Fatalf("expected fix task trigger_task=%q, got %q", sourceTask.ID, got)
	}
}

func TestHandleTestCompletionLinksFixTaskToTriggerTask(t *testing.T) {
	manager := &AgentManager{}
	sourceTask := &Task{ID: "test-root", Type: TaskTest, Status: TaskCompleted}
	build := &Build{
		ID:          "test-link-build",
		Description: "Test lineage should keep follow-on fix tasks in phase scope",
		Tasks:       []*Task{sourceTask},
	}

	manager.handleTestCompletion(build, sourceTask, &TaskOutput{
		Messages: []string{"test failed: expected 200 got 500"},
	})

	if len(build.Tasks) != 2 {
		t.Fatalf("expected follow-on fix task to be appended, got %d tasks", len(build.Tasks))
	}
	fixTask := build.Tasks[1]
	if fixTask == nil || fixTask.Type != TaskFix {
		t.Fatalf("expected appended task to be a fix task, got %+v", fixTask)
	}
	if got := taskInputStringValue(fixTask.Input, "trigger_task"); got != sourceTask.ID {
		t.Fatalf("expected fix task trigger_task=%q, got %q", sourceTask.ID, got)
	}
}

func TestWaitForPhaseCompletionRecoversStaleInProgressTaskWithoutMonitor(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cancelled := make(chan struct{}, 1)
	manager := &AgentManager{
		ctx:         ctx,
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		resultQueue: make(chan *TaskResult, 1),
		subscribers: make(map[string][]chan *WSMessage),
		taskCancels: map[string]context.CancelFunc{
			"phase-task": func() {
				select {
				case cancelled <- struct{}{}:
				default:
				}
			},
		},
	}

	startedAt := time.Now().Add(-6 * time.Minute).UTC()
	build := &Build{
		ID:        "phase-stale-recovery",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		UpdatedAt: time.Now().Add(-5 * time.Minute).UTC(),
		Agents:    make(map[string]*Agent),
		Tasks: []*Task{
			{
				ID:          "phase-task",
				Type:        TaskGenerateAPI,
				Description: "Implement backend services",
				AssignedTo:  "backend-1",
				Status:      TaskInProgress,
				StartedAt:   &startedAt,
				MaxRetries:  3,
				Input:       map[string]any{},
			},
		},
	}
	agent := &Agent{
		ID:       "backend-1",
		BuildID:  build.ID,
		Role:     RoleBackend,
		Provider: ai.ProviderGPT4,
		Status:   StatusWorking,
	}
	build.Agents[agent.ID] = agent
	manager.builds[build.ID] = build
	manager.agents[agent.ID] = agent

	done := make(chan bool, 1)
	go func() {
		done <- manager.waitForPhaseCompletion(build, []string{"phase-task"})
	}()

	select {
	case result := <-manager.resultQueue:
		if result.TaskID != "phase-task" {
			t.Fatalf("unexpected recovered task id %q", result.TaskID)
		}
		if result.Error == nil || !strings.Contains(result.Error.Error(), "timeout") {
			t.Fatalf("expected timeout recovery error, got %+v", result.Error)
		}
		build.mu.Lock()
		build.Tasks[0].Status = TaskCompleted
		now := time.Now()
		build.Tasks[0].CompletedAt = &now
		build.UpdatedAt = now
		build.mu.Unlock()
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("expected phase waiter to trigger stale-task recovery")
	}

	select {
	case ok := <-done:
		if !ok {
			t.Fatal("expected recovered phase to complete successfully")
		}
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("expected phase waiter to exit after recovery")
	}

	select {
	case <-cancelled:
	default:
		t.Fatal("expected stale task cancel func to be invoked")
	}
}

func TestWaitForPhaseCompletionSurvivesRecoverableProviderTimeoutRetry(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	task := &Task{
		ID:          "phase-timeout-task",
		Type:        TaskGenerateAPI,
		Description: "Implement backend services",
		Status:      TaskInProgress,
		AssignedTo:  "backend-1",
		MaxRetries:  3,
		Input:       map[string]any{},
		CreatedAt:   time.Now(),
	}
	build := &Build{
		ID:        "phase-timeout-build",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		UpdatedAt: time.Now(),
		Agents:    make(map[string]*Agent),
		Tasks:     []*Task{task},
	}
	agent := &Agent{
		ID:          "backend-1",
		BuildID:     build.ID,
		Role:        RoleBackend,
		Provider:    ai.ProviderGPT4,
		Status:      StatusWorking,
		CurrentTask: task,
	}
	build.Agents[agent.ID] = agent

	manager := &AgentManager{
		ctx:         ctx,
		builds:      map[string]*Build{build.ID: build},
		agents:      map[string]*Agent{agent.ID: agent},
		aiRouter:    &consensusRetryRouter{providers: []ai.AIProvider{ai.ProviderGPT4, ai.ProviderClaude}},
		taskQueue:   make(chan *Task, 1),
		subscribers: map[string][]chan *WSMessage{},
	}

	done := make(chan bool, 1)
	go func() {
		done <- manager.waitForPhaseCompletion(build, []string{task.ID})
	}()

	manager.processResult(&TaskResult{
		TaskID:  task.ID,
		AgentID: agent.ID,
		Attempt: 0,
		Success: false,
		Error:   errors.New("context deadline exceeded"),
	})

	if task.Status != TaskPending {
		t.Fatalf("expected timed-out task to be requeued pending, got %s", task.Status)
	}
	if task.RetryCount != 1 {
		t.Fatalf("expected retry count to advance to 1, got %d", task.RetryCount)
	}
	if task.RetryStrategy != RetryStrategy("switch_provider") {
		t.Fatalf("expected switch_provider retry strategy, got %s", task.RetryStrategy)
	}

	select {
	case queued := <-manager.taskQueue:
		if queued.ID != task.ID {
			t.Fatalf("expected retried task %s, got %s", task.ID, queued.ID)
		}
		build.mu.Lock()
		task.Status = TaskCompleted
		now := time.Now()
		task.CompletedAt = &now
		build.UpdatedAt = now
		build.mu.Unlock()
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("expected timed-out phase task to be requeued")
	}

	select {
	case ok := <-done:
		if !ok {
			t.Fatal("expected phase waiter to tolerate retry and finish cleanly")
		}
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("expected phase waiter to complete after retry handoff")
	}
}

func TestRecoverStaleInProgressTasksQueuesSyntheticTimeoutFailure(t *testing.T) {
	t.Parallel()

	cancelled := make(chan struct{}, 1)
	manager := &AgentManager{
		ctx:         context.Background(),
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		resultQueue: make(chan *TaskResult, 2),
		subscribers: make(map[string][]chan *WSMessage),
		taskCancels: map[string]context.CancelFunc{
			"stale-task": func() {
				select {
				case cancelled <- struct{}{}:
				default:
				}
			},
		},
	}

	startedAt := time.Now().Add(-7 * time.Minute).UTC()
	build := &Build{
		ID:        "stale-build",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		UpdatedAt: time.Now().Add(-6 * time.Minute).UTC(),
		Agents:    make(map[string]*Agent),
		Tasks: []*Task{
			{
				ID:          "stale-task",
				Type:        TaskGenerateUI,
				Description: "Build dashboard",
				AssignedTo:  "frontend-1",
				Status:      TaskInProgress,
				StartedAt:   &startedAt,
				MaxRetries:  4,
				Input:       map[string]any{},
			},
		},
	}
	agent := &Agent{
		ID:       "frontend-1",
		BuildID:  build.ID,
		Role:     RoleFrontend,
		Provider: ai.ProviderGPT4,
		Status:   StatusWorking,
	}
	build.Agents[agent.ID] = agent
	manager.builds[build.ID] = build
	manager.agents[agent.ID] = agent

	if recovered := manager.recoverStaleInProgressTasks(build, 6*time.Minute); !recovered {
		t.Fatal("expected stale in-progress task to be recovered")
	}

	result := <-manager.resultQueue
	if result.TaskID != "stale-task" || result.Attempt != 0 {
		t.Fatalf("unexpected synthetic result: %+v", result)
	}
	if result.Error == nil || !strings.Contains(result.Error.Error(), "timeout") {
		t.Fatalf("expected timeout synthetic error, got %+v", result.Error)
	}
	if got := taskInputInt(build.Tasks[0].Input, "stale_recovery_attempt"); got != 0 {
		t.Fatalf("expected stale_recovery_attempt marker 0, got %d", got)
	}
	select {
	case <-cancelled:
	default:
		t.Fatal("expected stale task cancel func to be invoked")
	}
}

func TestProcessResultDropsStaleTaskAttemptResult(t *testing.T) {
	t.Parallel()

	manager := &AgentManager{
		ctx:         context.Background(),
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
	}

	now := time.Now().UTC()
	task := &Task{
		ID:         "retry-task",
		Type:       TaskGenerateUI,
		Status:     TaskInProgress,
		RetryCount: 1,
		StartedAt:  &now,
	}
	agent := &Agent{
		ID:          "frontend-1",
		BuildID:     "retry-build",
		Role:        RoleFrontend,
		Provider:    ai.ProviderGPT4,
		Status:      StatusWorking,
		CurrentTask: task,
	}
	build := &Build{
		ID:        "retry-build",
		Status:    BuildInProgress,
		UpdatedAt: now,
	}
	manager.agents[agent.ID] = agent
	manager.builds[build.ID] = build

	manager.processResult(&TaskResult{
		TaskID:  task.ID,
		AgentID: agent.ID,
		Attempt: 0,
		Success: true,
		Output:  &TaskOutput{Messages: []string{"late success"}},
	})

	if task.Status != TaskInProgress {
		t.Fatalf("expected stale attempt result to leave task in progress, got %s", task.Status)
	}
	if task.Output != nil {
		t.Fatal("expected stale attempt result to be ignored")
	}
}
