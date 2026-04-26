package agents

import (
	"context"
	"testing"
	"time"

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

type routingDeadlineProbeRouter struct {
	lastDeadlineWindow time.Duration
}

func (r *routingDeadlineProbeRouter) Generate(ctx context.Context, _ ai.AIProvider, _ string, _ GenerateOptions) (*ai.AIResponse, error) {
	if deadline, ok := ctx.Deadline(); ok {
		r.lastDeadlineWindow = time.Until(deadline)
	}
	return &ai.AIResponse{
		Content: "// File: src/App.tsx\n```typescript\nexport default function App() { return <main>ok</main>; }\n```",
		Usage:   &ai.Usage{},
	}, nil
}

func (r *routingDeadlineProbeRouter) GetAvailableProviders() []ai.AIProvider {
	return []ai.AIProvider{ai.ProviderOllama, ai.ProviderClaude}
}

func (r *routingDeadlineProbeRouter) GetAvailableProvidersForUser(_ uint) []ai.AIProvider {
	return r.GetAvailableProviders()
}

func (r *routingDeadlineProbeRouter) HasConfiguredProviders() bool { return true }

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

func TestPlanRoutingWaterfallLocksMaxBuildToMaxModels(t *testing.T) {
	t.Parallel()

	build := &Build{PowerMode: PowerMax}
	task := &Task{
		Type: TaskReview,
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				TaskShape: TaskShapeVerification,
				RiskLevel: RiskLow,
			},
		},
	}

	decision := planRoutingWaterfall(build, task, ai.ProviderGrok)
	if decision.PowerMode != PowerMax {
		t.Fatalf("expected max power mode to remain locked, got %+v", decision)
	}
	if decision.Model != "grok-4.20-0309-reasoning" {
		t.Fatalf("expected grok max model, got %+v", decision)
	}
	if decision.Reason != "locked_to_max_power" {
		t.Fatalf("expected max-lock reason, got %+v", decision)
	}
}

func TestGenerateTaskOutputWithProviderUsesRoutingWaterfallWhenEnabled(t *testing.T) {
	t.Parallel()

	router := &waterfallProbeRouter{}
	am := &AgentManager{
		aiRouter: router,
		builds:   map[string]*Build{},
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
		Agents: map[string]*Agent{},
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
	build.Agents[agent.ID] = agent
	am.builds[build.ID] = build

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
	build.mu.RLock()
	defer build.mu.RUnlock()
	if len(build.ActivityTimeline) != 1 {
		t.Fatalf("expected provider route telemetry entry, got %d", len(build.ActivityTimeline))
	}
	if entry := build.ActivityTimeline[0]; entry.EventType != string(WSGlassProviderRouteSelected) || entry.TaskID != task.ID || entry.Provider != string(ai.ProviderGPT4) {
		t.Fatalf("unexpected provider route telemetry entry: %+v", entry)
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

func TestGenerateTaskOutputWithProviderHonorsManualProviderModelOverride(t *testing.T) {
	t.Parallel()

	router := &waterfallProbeRouter{}
	am := &AgentManager{
		aiRouter: router,
		builds:   map[string]*Build{},
	}

	flags := defaultBuildOrchestrationFlags()
	flags.EnableRoutingWaterfall = true
	build := &Build{
		ID:                     "build-waterfall-manual-override",
		UserID:                 13,
		PowerMode:              PowerMax,
		ProviderMode:           "platform",
		ProviderModelOverrides: map[string]string{"gpt4": "gpt-4.1"},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{Flags: flags},
		},
		Agents: map[string]*Agent{},
	}
	agent := &Agent{
		ID:       "agent-waterfall-manual-override",
		Role:     RoleBackend,
		Provider: ai.ProviderGPT4,
		BuildID:  build.ID,
	}
	task := &Task{
		ID:          "task-waterfall-manual-override",
		Type:        TaskGenerateAPI,
		Description: "Generate route",
	}
	build.Agents[agent.ID] = agent
	am.builds[build.ID] = build

	candidate, err := am.generateTaskOutputWithProvider(context.Background(), build, agent, task, "prompt", "system", ai.ProviderGPT4, 1200, 0.1)
	if err != nil {
		t.Fatalf("generateTaskOutputWithProvider returned error: %v", err)
	}
	if candidate == nil || candidate.Output == nil {
		t.Fatalf("expected candidate output")
	}
	if router.lastOpts.ModelOverride != "gpt-4.1" {
		t.Fatalf("expected manual provider model override to win, got %+v", router.lastOpts)
	}
	if candidate.WaterfallReason != "provider_model_override" {
		t.Fatalf("expected manual override reason, got %+v", candidate)
	}
}

func TestGenerateTaskOutputWithProviderUsesPerProviderAttemptTimeout(t *testing.T) {
	t.Parallel()

	router := &routingDeadlineProbeRouter{}
	am := &AgentManager{
		aiRouter: router,
		builds:   map[string]*Build{},
	}

	build := &Build{
		ID:           "build-provider-attempt-timeout",
		UserID:       21,
		PowerMode:    PowerMax,
		ProviderMode: "platform",
		Agents:       map[string]*Agent{},
	}
	agent := &Agent{
		ID:       "agent-provider-attempt-timeout",
		Role:     RoleBackend,
		Provider: ai.ProviderOllama,
		BuildID:  build.ID,
		Model:    "kimi-k2.6:cloud",
	}
	task := &Task{
		ID:          "task-provider-attempt-timeout",
		Type:        TaskGenerateAPI,
		Description: "Generate API",
	}
	build.Agents[agent.ID] = agent
	am.builds[build.ID] = build

	outerCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if _, err := am.generateTaskOutputWithProvider(outerCtx, build, agent, task, "prompt", "system", ai.ProviderOllama, 1200, 0.1); err != nil {
		t.Fatalf("generateTaskOutputWithProvider returned error: %v", err)
	}
	if router.lastDeadlineWindow <= 0 {
		t.Fatal("expected provider attempt deadline to be applied")
	}
	if router.lastDeadlineWindow > 7*time.Minute {
		t.Fatalf("provider attempt deadline = %v, want <= 7m so fallback providers can still run", router.lastDeadlineWindow)
	}
}
