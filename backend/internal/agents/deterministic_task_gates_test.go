package agents

import (
	"context"
	"strings"
	"testing"

	"apex-build/internal/ai"
)

func TestProviderAssistedTaskVerificationSkipsCritiqueOnDeterministicFailure(t *testing.T) {
	t.Parallel()

	router := &taskRoutingRouter{
		stubAIRouter: stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderGPT4, ai.ProviderGrok},
			hasConfiguredProvider: true,
		},
		verifyContent: `{"summary":"ok","warnings":[],"blockers":[],"confidence":0.9}`,
	}
	am := &AgentManager{
		aiRouter: router,
		ctx:      context.Background(),
	}

	flags := defaultBuildOrchestrationFlags()
	flags.EnableDeterministicTaskGates = true
	build := &Build{
		ID:           "build-deterministic-skip-critique",
		UserID:       77,
		ProviderMode: "platform",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{Flags: flags},
		},
	}

	task := &Task{
		ID:          "task-deterministic-skip-critique",
		Type:        TaskDeploy,
		Description: "Update deployment config",
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				ID:          "wo-deterministic-skip",
				TaskShape:   TaskShapeIntegration,
				RoutingMode: RoutingModeSingleWithVerifier,
				RiskLevel:   RiskHigh,
			},
		},
	}
	candidate := &taskGenerationCandidate{
		Provider:     ai.ProviderGPT4,
		VerifyPassed: true,
		Output: &TaskOutput{
			Files: []GeneratedFile{
				{
					Path:     "render.yaml",
					Language: "yaml",
					Content:  "services:\n  - name: api\n    envVars: [",
				},
			},
		},
	}

	report := am.providerAssistedTaskVerification(build, task, candidate)
	if report == nil {
		t.Fatal("expected deterministic verification report")
	}
	if report.Status != VerificationBlocked {
		t.Fatalf("expected deterministic failure to block, got %+v", report)
	}
	if report.DeterministicStatus != verificationReasonDeterministicFailed {
		t.Fatalf("expected deterministic_failed status, got %+v", report)
	}
	if report.ProviderCritiqueStatus != verificationReasonProviderCritiqueSkip {
		t.Fatalf("expected provider critique skip status, got %+v", report)
	}
	if !containsString(report.ChecksRun, verificationReasonDeterministicFailed) {
		t.Fatalf("expected deterministic failure reason in checks, got %+v", report.ChecksRun)
	}
	if strings.TrimSpace(router.lastPrompt) != "" {
		t.Fatalf("expected provider critique prompt to be skipped, got %q", router.lastPrompt)
	}
}

func TestProviderAssistedTaskVerificationRunsCritiqueWhenDeterministicPasses(t *testing.T) {
	t.Parallel()

	router := &taskRoutingRouter{
		stubAIRouter: stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderGPT4, ai.ProviderGrok},
			hasConfiguredProvider: true,
		},
		verifyContent: `{"summary":"looks good","warnings":[],"blockers":[],"confidence":0.82}`,
	}
	am := &AgentManager{
		aiRouter: router,
		ctx:      context.Background(),
	}

	flags := defaultBuildOrchestrationFlags()
	flags.EnableDeterministicTaskGates = true
	build := &Build{
		ID:           "build-deterministic-pass-critique",
		UserID:       78,
		ProviderMode: "platform",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{Flags: flags},
		},
	}

	task := &Task{
		ID:          "task-deterministic-pass-critique",
		Type:        TaskGenerateAPI,
		Description: "Generate backend route",
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				ID:          "wo-deterministic-pass",
				TaskShape:   TaskShapeBackendPatch,
				RoutingMode: RoutingModeSingleWithVerifier,
				RiskLevel:   RiskMedium,
			},
		},
	}
	candidate := &taskGenerationCandidate{
		Provider:     ai.ProviderGPT4,
		VerifyPassed: true,
		Output: &TaskOutput{
			Files: []GeneratedFile{
				{
					Path:     "backend/api/health.ts",
					Language: "typescript",
					Content:  "export const health = () => ({ ok: true })",
				},
			},
		},
	}

	report := am.providerAssistedTaskVerification(build, task, candidate)
	if report == nil {
		t.Fatal("expected provider verification report")
	}
	if report.DeterministicStatus != verificationReasonDeterministicPassed {
		t.Fatalf("expected deterministic_passed status, got %+v", report)
	}
	if report.ProviderCritiqueStatus != verificationReasonProviderCritiqueNeed {
		t.Fatalf("expected provider critique needed status, got %+v", report)
	}
	if !containsString(report.ChecksRun, verificationReasonProviderCritiqueNeed) {
		t.Fatalf("expected provider critique reason in checks, got %+v", report.ChecksRun)
	}
	if !strings.Contains(router.lastPrompt, "Review this AI-generated task result") {
		t.Fatalf("expected provider critique prompt, got %q", router.lastPrompt)
	}
}

func TestProviderAssistedTaskVerificationSkipsRecursiveCritiqueForReviewTasks(t *testing.T) {
	t.Parallel()

	router := &taskRoutingRouter{
		stubAIRouter: stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderGPT4, ai.ProviderGemini},
			hasConfiguredProvider: true,
		},
		verifyContent: `{"summary":"blocked","warnings":[],"blockers":["Missing error handling for individual API fetches in AppShell.tsx."],"confidence":0.92}`,
	}
	am := &AgentManager{
		aiRouter: router,
		ctx:      context.Background(),
	}
	build := &Build{
		ID:           "build-review-critique-skip",
		UserID:       79,
		ProviderMode: "platform",
	}
	task := &Task{
		ID:          "task-review-critique-skip",
		Type:        TaskReview,
		Description: "Review generated project files",
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				ID:          "wo-review-critique-skip",
				TaskShape:   TaskShapeVerification,
				RoutingMode: RoutingModeSingleWithVerifier,
				ContractSlice: WorkOrderContractSlice{
					Surface: SurfaceFrontend,
				},
			},
		},
	}
	candidate := &taskGenerationCandidate{
		Provider:     ai.ProviderGemini,
		VerifyPassed: true,
		Output: &TaskOutput{
			Messages: []string{"Review complete: no critical issues found."},
		},
	}

	report := am.providerAssistedTaskVerification(build, task, candidate)
	if report == nil {
		t.Fatal("expected skipped provider verification report")
	}
	if report.Status != VerificationPassed {
		t.Fatalf("expected skipped review verifier to pass, got %+v", report)
	}
	if report.ProviderCritiqueStatus != verificationReasonProviderCritiqueSkip {
		t.Fatalf("expected provider critique skip status, got %+v", report)
	}
	if !containsString(report.ChecksRun, "review_task_recursive_critique_skipped") {
		t.Fatalf("expected review skip reason in checks, got %+v", report.ChecksRun)
	}
	if strings.TrimSpace(router.lastPrompt) != "" {
		t.Fatalf("expected no recursive critique provider call, got prompt %q", router.lastPrompt)
	}
}

func TestProviderAssistedTaskVerificationStillCritiquesReviewFileArtifacts(t *testing.T) {
	t.Parallel()

	router := &taskRoutingRouter{
		stubAIRouter: stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderGPT4, ai.ProviderGemini},
			hasConfiguredProvider: true,
		},
		verifyContent: `{"summary":"looks good","warnings":[],"blockers":[],"confidence":0.9}`,
	}
	am := &AgentManager{
		aiRouter: router,
		ctx:      context.Background(),
	}
	build := &Build{
		ID:           "build-review-artifact-critique",
		UserID:       80,
		ProviderMode: "platform",
	}
	task := &Task{
		ID:          "task-review-artifact-critique",
		Type:        TaskReview,
		Description: "Review generated project files",
	}
	candidate := &taskGenerationCandidate{
		Provider:     ai.ProviderGemini,
		VerifyPassed: true,
		Output: &TaskOutput{
			Files: []GeneratedFile{
				{
					Path:    "review-notes.ts",
					Content: "export const reviewed = true;\n",
				},
			},
		},
	}

	report := am.providerAssistedTaskVerification(build, task, candidate)
	if report == nil {
		t.Fatal("expected provider verification report")
	}
	if report.ProviderCritiqueStatus != verificationReasonProviderCritiqueNeed {
		t.Fatalf("expected provider critique to run for file artifacts, got %+v", report)
	}
	if !strings.Contains(router.lastPrompt, "Review this AI-generated task result") {
		t.Fatalf("expected recursive critique provider call for file artifact, got %q", router.lastPrompt)
	}
}
