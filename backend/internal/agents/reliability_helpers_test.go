package agents

import (
	"testing"
	"time"
)

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
