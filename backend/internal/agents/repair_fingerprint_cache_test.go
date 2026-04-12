package agents

import (
	"strings"
	"testing"
	"time"

	"apex-build/internal/ai"
)

func TestRepairFingerprintCacheLookupFindsRecentSuccessfulStrategy(t *testing.T) {
	am := &AgentManager{}
	build := repairFingerprintCacheTestBuild()
	agent := &Agent{Provider: ai.ProviderGPT4, Role: RoleFrontend}
	task := repairFingerprintCacheTestTask()

	insight := am.recentFailureFingerprintInsight(build, agent, task, "build_failure")
	entry := am.repairFingerprintCacheLookup(build, agent, task, "build_failure", "standard_retry", insight)

	if entry.SuccessfulRecoveries != 1 {
		t.Fatalf("expected one successful recovery, got %+v", entry)
	}
	if entry.SuggestedRetry != "fix_and_retry" {
		t.Fatalf("expected fix_and_retry cache suggestion, got %+v", entry)
	}
	if !containsString(entry.RecentStrategies, "targeted_symbol_repair") {
		t.Fatalf("expected recent successful strategy, got %+v", entry.RecentStrategies)
	}
	if !containsString(entry.RecentPatchClasses, "symbol_patch") {
		t.Fatalf("expected recent patch class, got %+v", entry.RecentPatchClasses)
	}
}

func TestDetermineRetryStrategyWithHistoryUsesSuccessfulRepairFingerprintCache(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubAIRouter{
			providers: []ai.AIProvider{ai.ProviderGPT4, ai.ProviderGemini},
		},
	}
	build := repairFingerprintCacheTestBuild()
	agent := &Agent{Provider: ai.ProviderGPT4, Role: RoleFrontend}
	task := repairFingerprintCacheTestTask()

	got := am.determineRetryStrategyWithHistory(build, agent, "unexpected generated component regression", task)
	if got != "fix_and_retry" {
		t.Fatalf("determineRetryStrategyWithHistory() = %q, want fix_and_retry", got)
	}
}

func TestBuildTaskPromptIncludesRepairFingerprintCacheContext(t *testing.T) {
	am := &AgentManager{}
	build := repairFingerprintCacheTestBuild()
	agent := &Agent{Provider: ai.ProviderGPT4, Role: RoleFrontend}
	task := repairFingerprintCacheTestTask()
	task.Error = "unexpected generated component regression"

	prompt := am.buildTaskPrompt(task, build, agent)
	if !strings.Contains(prompt, "<repair_fingerprint_cache>") {
		t.Fatalf("expected repair fingerprint cache prompt context, got %q", prompt)
	}
	if !strings.Contains(prompt, "recent_successful_strategies: targeted_symbol_repair") {
		t.Fatalf("expected successful strategy in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "recent_successful_patch_classes: symbol_patch") {
		t.Fatalf("expected patch class in prompt, got %q", prompt)
	}
}

func repairFingerprintCacheTestBuild() *Build {
	return &Build{
		ID:           "repair-fingerprint-cache-build",
		Description:  "Build a dashboard app",
		ProviderMode: "platform",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
				FailureFingerprints: []FailureFingerprint{
					{
						Provider:            ai.ProviderGPT4,
						TaskShape:           TaskShapeFrontendPatch,
						FailureClass:        "build_failure",
						FilesInvolved:       []string{"src/App.tsx"},
						RepairPathChosen:    []string{"task_execution", "fix_and_retry"},
						RepairSucceeded:     false,
						TokenCostToRecovery: 900,
						CreatedAt:           time.Now().Add(-2 * time.Minute).UTC(),
					},
					{
						Provider:            ai.ProviderGPT4,
						TaskShape:           TaskShapeFrontendPatch,
						FailureClass:        "build_failure",
						FilesInvolved:       []string{"src/App.tsx"},
						RepairPathChosen:    []string{"task_execution", "fix_and_retry"},
						RepairStrategy:      "targeted_symbol_repair",
						PatchClass:          "symbol_patch",
						RepairSucceeded:     true,
						TokenCostToRecovery: 1200,
						CreatedAt:           time.Now().Add(-time.Minute).UTC(),
					},
				},
			},
		},
	}
}

func repairFingerprintCacheTestTask() *Task {
	return &Task{
		Type:        TaskGenerateUI,
		Description: "Patch the frontend dashboard",
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				TaskShape: TaskShapeFrontendPatch,
			},
			"target_files": []string{"src/App.tsx"},
		},
	}
}
