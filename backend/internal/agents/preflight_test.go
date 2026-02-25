package agents

import (
	"context"
	"testing"

	"apex-build/internal/ai"
)

// stubPreflight implements AIRouter for preflight test scenarios.
type stubPreflight struct {
	configured     bool
	allProviders   []ai.AIProvider
	userProviders  []ai.AIProvider
	generateResult *ai.AIResponse
	generateErr    error
}

func (s *stubPreflight) Generate(_ context.Context, _ ai.AIProvider, _ string, _ GenerateOptions) (*ai.AIResponse, error) {
	return s.generateResult, s.generateErr
}
func (s *stubPreflight) GetAvailableProviders() []ai.AIProvider   { return s.allProviders }
func (s *stubPreflight) GetAvailableProvidersForUser(_ uint) []ai.AIProvider { return s.userProviders }
func (s *stubPreflight) HasConfiguredProviders() bool             { return s.configured }

func TestDetermineRetryStrategyNonRetriable(t *testing.T) {
	am := &AgentManager{}

	tests := []struct {
		name     string
		errMsg   string
		expected string
	}{
		{"insufficient credits", "INSUFFICIENT_CREDITS from provider", "non_retriable"},
		{"invalid api key", "invalid api key provided", "non_retriable"},
		{"quota exhausted", "quota exhausted for this project", "non_retriable"},
		{"rate limit is retriable", "429 rate limit exceeded", "backoff"},
		{"timeout is retriable", "connection timeout after 30s", "switch_provider"},
		{"context length is retriable", "context length exceeded", "reduce_context"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := am.determineRetryStrategy(tc.errMsg, &Task{})
			if result != tc.expected {
				t.Fatalf("determineRetryStrategy(%q) = %q, want %q", tc.errMsg, result, tc.expected)
			}
		})
	}
}

func TestStrategyToDecisionNonRetriableMapsToAbort(t *testing.T) {
	am := &AgentManager{}

	// non_retriable must map to abort, NOT spawn_solver.
	// Spawning a solver for an auth/billing failure wastes API calls.
	decision := am.strategyToDecision("non_retriable")
	if decision != decisionAbort {
		t.Fatalf("strategyToDecision(non_retriable) = %q, want %q", decision, decisionAbort)
	}
}

func TestBuildErrorPopulatedOnFailure(t *testing.T) {
	am := &AgentManager{
		builds:      make(map[string]*Build),
		subscribers: make(map[string][]chan *WSMessage),
	}

	build := &Build{
		ID:                    "test-error-surfacing",
		UserID:                1,
		Status:                BuildInProgress,
		PhasedPipelineComplete: true,
		Tasks: []*Task{
			{ID: "t1", Status: TaskCompleted},
			{ID: "t2", Status: TaskFailed, Error: "API key is invalid for provider openai"},
		},
	}
	am.builds[build.ID] = build

	am.checkBuildCompletion(build)

	build.mu.RLock()
	defer build.mu.RUnlock()

	if build.Status != BuildFailed {
		t.Fatalf("expected BuildFailed, got %s", build.Status)
	}
	if build.Error == "" {
		t.Fatal("expected build.Error to be populated from failed task, got empty string")
	}
	if build.Error != "API key is invalid for provider openai" {
		t.Fatalf("expected task error to be surfaced, got: %s", build.Error)
	}
}

func TestBuildErrorNotOverwrittenWhenAlreadySet(t *testing.T) {
	am := &AgentManager{
		builds:      make(map[string]*Build),
		subscribers: make(map[string][]chan *WSMessage),
	}

	build := &Build{
		ID:                    "test-existing-error",
		UserID:                1,
		Status:                BuildInProgress,
		Error:                 "Validation failed: missing entry point",
		PhasedPipelineComplete: true,
		Tasks: []*Task{
			{ID: "t1", Status: TaskFailed, Error: "something else"},
		},
	}
	am.builds[build.ID] = build

	am.checkBuildCompletion(build)

	build.mu.RLock()
	defer build.mu.RUnlock()

	if build.Error != "Validation failed: missing entry point" {
		t.Fatalf("expected original error preserved, got: %s", build.Error)
	}
}
