package agents

import (
	"context"
	"testing"

	"apex-build/internal/ai"
)

type waterfallProbeRouter struct {
	lastProvider ai.AIProvider
	lastOpts     GenerateOptions
}

func (r *waterfallProbeRouter) Generate(_ context.Context, provider ai.AIProvider, _ string, opts GenerateOptions) (*ai.AIResponse, error) {
	r.lastProvider = provider
	r.lastOpts = opts
	return &ai.AIResponse{
		Content: "// File: src/App.tsx\n```typescript\nexport default function App() { return <main>ok</main>; }\n```",
		Usage:   &ai.Usage{},
	}, nil
}

func (r *waterfallProbeRouter) GetAvailableProviders() []ai.AIProvider {
	return []ai.AIProvider{ai.ProviderGPT4}
}

func (r *waterfallProbeRouter) GetAvailableProvidersForUser(_ uint) []ai.AIProvider {
	return []ai.AIProvider{ai.ProviderGPT4}
}

func (r *waterfallProbeRouter) HasConfiguredProviders() bool { return true }

func TestPlanRoutingWaterfallCapsToBuildPower(t *testing.T) {
	t.Parallel()

	build := &Build{PowerMode: PowerBalanced}
	task := &Task{
		Type:       TaskGenerateAPI,
		RetryCount: 2,
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				TaskShape:     TaskShapeIntegration,
				RiskLevel:     RiskCritical,
				OwnedFiles:    []string{"src/App.tsx"},
				ReadableFiles: []string{"backend/internal/server.go"},
			},
		},
	}

	decision := planRoutingWaterfall(build, task, ai.ProviderGPT4)
	if decision.Stage != routingWaterfallStageExpensive {
		t.Fatalf("expected expensive stage, got %+v", decision)
	}
	if decision.PowerMode != PowerBalanced {
		t.Fatalf("expected power mode capped to build mode, got %+v", decision)
	}
	if decision.Model == "" {
		t.Fatalf("expected model selection, got %+v", decision)
	}
}

func TestGenerateTaskOutputWithProviderUsesRoutingWaterfallWhenEnabled(t *testing.T) {
	t.Parallel()

	router := &waterfallProbeRouter{}
	am := &AgentManager{
		aiRouter: router,
	}

	flags := defaultBuildOrchestrationFlags()
	flags.EnableRoutingWaterfall = true
	build := &Build{
		ID:           "build-waterfall-enabled",
		UserID:       11,
		PowerMode:    PowerBalanced,
		ProviderMode: "platform",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{Flags: flags},
		},
	}
	agent := &Agent{
		ID:       "agent-waterfall-enabled",
		Role:     RoleBackend,
		Provider: ai.ProviderGPT4,
		BuildID:  build.ID,
	}
	task := &Task{
		ID:          "task-waterfall-enabled",
		Type:        TaskGenerateAPI,
		Description: "Generate route",
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				TaskShape:     TaskShapeIntegration,
				RiskLevel:     RiskCritical,
				OwnedFiles:    []string{"src/App.tsx"},
				ReadableFiles: []string{"backend/internal/server.go"},
			},
		},
	}

	candidate, err := am.generateTaskOutputWithProvider(context.Background(), build, agent, task, "prompt", "system", ai.ProviderGPT4, 1200, 0.1)
	if err != nil {
		t.Fatalf("generateTaskOutputWithProvider returned error: %v", err)
	}
	if candidate == nil || candidate.Output == nil {
		t.Fatalf("expected candidate output")
	}
	if candidate.WaterfallStage != routingWaterfallStageExpensive {
		t.Fatalf("expected expensive stage from waterfall, got %+v", candidate)
	}
	if router.lastOpts.ModelOverride == "" {
		t.Fatalf("expected model override to be set by waterfall")
	}
	if router.lastOpts.PowerMode != PowerBalanced {
		t.Fatalf("expected capped power mode in router opts, got %+v", router.lastOpts)
	}
	if got := taskOutputMetricString(candidate.Output, "routing_waterfall_stage"); got != routingWaterfallStageExpensive {
		t.Fatalf("expected routing waterfall stage metric, got %q", got)
	}
}

func TestGenerateTaskOutputWithProviderFallsBackWhenWaterfallDisabled(t *testing.T) {
	t.Parallel()

	router := &waterfallProbeRouter{}
	am := &AgentManager{
		aiRouter: router,
	}

	flags := defaultBuildOrchestrationFlags()
	flags.EnableRoutingWaterfall = false
	build := &Build{
		ID:           "build-waterfall-disabled",
		UserID:       12,
		PowerMode:    PowerMax,
		ProviderMode: "platform",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{Flags: flags},
		},
	}
	agent := &Agent{
		ID:       "agent-waterfall-disabled",
		Role:     RoleFrontend,
		Provider: ai.ProviderGPT4,
		BuildID:  build.ID,
	}
	task := &Task{
		ID:          "task-waterfall-disabled",
		Type:        TaskGenerateUI,
		Description: "Generate UI",
	}

	candidate, err := am.generateTaskOutputWithProvider(context.Background(), build, agent, task, "prompt", "system", ai.ProviderGPT4, 1200, 0.1)
	if err != nil {
		t.Fatalf("generateTaskOutputWithProvider returned error: %v", err)
	}
	if candidate == nil {
		t.Fatalf("expected candidate output")
	}
	if candidate.WaterfallStage != "static_fallback" {
		t.Fatalf("expected fallback stage, got %+v", candidate)
	}
	if router.lastOpts.PowerMode != PowerMax {
		t.Fatalf("expected build power mode in fallback path, got %+v", router.lastOpts)
	}
}
