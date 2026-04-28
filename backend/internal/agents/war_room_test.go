package agents

import (
	"context"
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
	lastProvider ai.AIProvider
	lastOpt      GenerateOptions
}

func (r *warRoomProbeRouter) Generate(_ context.Context, provider ai.AIProvider, _ string, opts GenerateOptions) (*ai.AIResponse, error) {
	r.lastProvider = provider
	r.lastOpt = opts
	return &ai.AIResponse{Content: `[]`}, nil
}

func TestEffectiveWarRoomCritiquePowerModeFollowsSelectedModeWithFastOverride(t *testing.T) {
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
		{name: "legacy fast mode overrides max", build: &Build{Mode: ModeFast, PowerMode: PowerMax}, want: PowerFast},
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
	if !router.lastOpt.UsePlatformKeys {
		t.Fatalf("expected platform-key routing to be preserved")
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
