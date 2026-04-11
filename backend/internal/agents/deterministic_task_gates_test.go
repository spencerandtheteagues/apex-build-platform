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
