package agents

import (
	"context"
	"sync"
	"testing"

	"apex-build/internal/ai"
)

func TestCompileDraftBuildSpecKeepsSpecUnlocked(t *testing.T) {
	t.Parallel()

	plan := &BuildPlan{
		AppType:      "fullstack",
		DeliveryMode: "full_stack_preview",
		Features: []Feature{
			{Name: "Dashboard", Description: "View account status"},
		},
		Files: []PlannedFile{
			{Path: "src/pages/Dashboard.tsx"},
		},
	}
	contract := &BuildContract{
		AppType:      "fullstack",
		DeliveryMode: "full_stack_preview",
		RoutePageMap: []ContractRoute{
			{Path: "/", File: "src/pages/Dashboard.tsx", Surface: SurfaceFrontend},
		},
		APIContract: &BuildAPIContract{
			Endpoints: []APIEndpoint{{Method: "GET", Path: "/api/dashboard"}},
		},
		AcceptanceBySurface: []SurfaceAcceptanceContract{
			{Surface: SurfaceFrontend, Required: true},
			{Surface: SurfaceBackend, Required: true},
		},
	}

	draft := compileDraftBuildSpec("build-draft-1", nil, plan, contract)
	if draft == nil || draft.Spec == nil {
		t.Fatal("expected draft spec")
	}
	if draft.Spec.Locked {
		t.Fatalf("expected draft spec to remain unlocked, got %+v", draft.Spec)
	}
	if draft.Spec.Source != "war_room_draft_v1" {
		t.Fatalf("expected draft source war_room_draft_v1, got %q", draft.Spec.Source)
	}
}

func TestCritiqueDraftBuildSpecFlagsRouteAndAPIGaps(t *testing.T) {
	t.Parallel()

	spec := &ValidatedBuildSpec{
		AppType:            "fullstack",
		DeliveryMode:       "full_stack_preview",
		AcceptanceSurfaces: []string{string(SurfaceFrontend), string(SurfaceBackend)},
	}
	issues := critiqueDraftBuildSpec(spec, nil)
	if len(issues) == 0 {
		t.Fatal("expected critique issues for missing route/api plan")
	}
	if !containsCritiqueIssue(issues, "war_room_api_plan_gap") {
		t.Fatalf("expected api plan gap issue, got %+v", issues)
	}
	if !containsCritiqueIssue(issues, "war_room_route_plan_gap") {
		t.Fatalf("expected route plan gap issue, got %+v", issues)
	}
}

func TestCompileWarRoomValidatedBuildSpecLocksWithCritiqueAdvisories(t *testing.T) {
	t.Parallel()

	plan := &BuildPlan{
		AppType:      "fullstack",
		DeliveryMode: "full_stack_preview",
	}
	contract := &BuildContract{
		AppType:      "fullstack",
		DeliveryMode: "full_stack_preview",
		AcceptanceBySurface: []SurfaceAcceptanceContract{
			{Surface: SurfaceFrontend, Required: true},
			{Surface: SurfaceBackend, Required: true},
		},
		AuthContract: &ContractAuthStrategy{
			Required: true,
		},
	}

	spec := compileWarRoomValidatedBuildSpec("build-war-room-1", &ValidatedBuildSpec{
		SecurityAdvisories: []BuildSpecAdvisory{},
	}, plan, contract)
	if spec == nil {
		t.Fatal("expected validated spec")
	}
	if !spec.Locked {
		t.Fatalf("expected war room validated spec to be locked, got %+v", spec)
	}
	if spec.Source != "war_room_validated_v1" {
		t.Fatalf("expected war room source tag, got %q", spec.Source)
	}
	if !hasBuildSpecAdvisoryCode(spec.SecurityAdvisories, "war_room_api_plan_gap") {
		t.Fatalf("expected API gap advisory in security advisories, got %+v", spec.SecurityAdvisories)
	}
	if !hasBuildSpecAdvisoryCode(spec.PerformanceAdvisories, "war_room_route_plan_gap") {
		t.Fatalf("expected route gap advisory in performance advisories, got %+v", spec.PerformanceAdvisories)
	}
}

type warRoomProbeRouter struct {
	stubAIRouter
	mu           sync.Mutex
	lastProvider ai.AIProvider
	lastOpt      GenerateOptions
	calls        []warRoomProbeCall
}

type warRoomProbeCall struct {
	provider ai.AIProvider
	opts     GenerateOptions
}

func (r *warRoomProbeRouter) Generate(_ context.Context, provider ai.AIProvider, _ string, opts GenerateOptions) (*ai.AIResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastProvider = provider
	r.lastOpt = opts
	r.calls = append(r.calls, warRoomProbeCall{provider: provider, opts: opts})
	return &ai.AIResponse{Content: `[]`}, nil
}

func (r *warRoomProbeRouter) snapshotCalls() []warRoomProbeCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]warRoomProbeCall(nil), r.calls...)
}

func TestEffectiveWarRoomCritiquePowerModeFollowsSelectedPowerMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		build *Build
		want  PowerMode
	}{
		{name: "nil defaults cheap", build: nil, want: PowerFast},
		{name: "unset defaults cheap", build: &Build{}, want: PowerFast},
		{name: "balanced follows selection", build: &Build{PowerMode: PowerBalanced}, want: PowerBalanced},
		{name: "max follows selection", build: &Build{PowerMode: PowerMax}, want: PowerMax},
		{name: "max power is not downgraded by fast build mode", build: &Build{Mode: ModeFast, PowerMode: PowerMax}, want: PowerMax},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := effectiveWarRoomCritiquePowerMode(tt.build); got != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}

func TestRunSingleDebateRoundUsesSelectedPowerModeAndTokenCap(t *testing.T) {
	router := &warRoomProbeRouter{}

	runSingleDebateRound(
		context.Background(),
		router,
		"build-war-room-power",
		42,
		true,
		PowerMax,
		0,
		"AppType: fullstack",
		ai.ProviderClaude,
		"security",
	)

	if router.lastProvider != ai.ProviderClaude {
		t.Fatalf("expected claude provider, got %s", router.lastProvider)
	}
	if router.lastOpt.PowerMode != PowerMax {
		t.Fatalf("expected max power mode, got %s", router.lastOpt.PowerMode)
	}
	if router.lastOpt.BuildID != "build-war-room-power" {
		t.Fatalf("expected build id to be preserved, got %q", router.lastOpt.BuildID)
	}
	if router.lastOpt.MaxTokens != warRoomLLMDebateMaxTokensForPowerMode(PowerMax) {
		t.Fatalf("expected max-mode token cap, got %d", router.lastOpt.MaxTokens)
	}
	if router.lastOpt.ModelOverride != selectModelForPowerMode(ai.ProviderClaude, PowerMax) {
		t.Fatalf("expected max-mode Claude flagship model override, got %q", router.lastOpt.ModelOverride)
	}
	if !router.lastOpt.UsePlatformKeys {
		t.Fatalf("expected platform-key routing to be preserved")
	}
}

func TestWarRoomLLMDebateRunsForBalancedAndMaxOnly(t *testing.T) {
	spec := &ValidatedBuildSpec{AppType: "fullstack", DeliveryMode: "full_stack_preview"}
	contract := &BuildContract{AppType: "fullstack", DeliveryMode: "full_stack_preview"}

	for _, tt := range []struct {
		name      string
		powerMode PowerMode
		wantCalls int
	}{
		{name: "fast skips paid LLM debate", powerMode: PowerFast, wantCalls: 0},
		{name: "balanced runs War Room debate", powerMode: PowerBalanced, wantCalls: 2},
		{name: "max runs War Room debate", powerMode: PowerMax, wantCalls: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			router := &warRoomProbeRouter{}
			am := &AgentManager{aiRouter: router, ctx: context.Background()}

			am.enrichWarRoomSpecWithLLMDebate("build-war-room-mode", 42, true, tt.powerMode, spec, contract)

			if calls := router.snapshotCalls(); len(calls) != tt.wantCalls {
				t.Fatalf("expected %d War Room LLM calls for %s, got %+v", tt.wantCalls, tt.powerMode, calls)
			}
		})
	}
}

func TestWarRoomMaxUsesOnlyFlagshipModelOverrides(t *testing.T) {
	router := &warRoomProbeRouter{}
	am := &AgentManager{aiRouter: router, ctx: context.Background()}

	am.runWarRoomLLMDebate(
		"build-war-room-max-flagship",
		42,
		true,
		PowerMax,
		&ValidatedBuildSpec{AppType: "fullstack", DeliveryMode: "full_stack_preview"},
		&BuildContract{AppType: "fullstack", DeliveryMode: "full_stack_preview"},
	)

	calls := router.snapshotCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 War Room calls, got %+v", calls)
	}
	wantModels := map[ai.AIProvider]string{
		ai.ProviderClaude: selectModelForPowerMode(ai.ProviderClaude, PowerMax),
		ai.ProviderGPT4:   selectModelForPowerMode(ai.ProviderGPT4, PowerMax),
	}
	for _, call := range calls {
		want := wantModels[call.provider]
		if want == "" {
			t.Fatalf("unexpected War Room provider in max mode: %+v", call)
		}
		if call.opts.PowerMode != PowerMax {
			t.Fatalf("expected max power mode for %+v", call)
		}
		if call.opts.ModelOverride != want {
			t.Fatalf("expected flagship model %q for %s, got %q", want, call.provider, call.opts.ModelOverride)
		}
	}
}

func containsCritiqueIssue(values []buildSpecCritiqueIssue, code string) bool {
	for _, value := range values {
		if value.Code == code {
			return true
		}
	}
	return false
}
