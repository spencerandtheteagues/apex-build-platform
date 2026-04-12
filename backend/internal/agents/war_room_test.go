package agents

import "testing"

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

func containsCritiqueIssue(values []buildSpecCritiqueIssue, code string) bool {
	for _, value := range values {
		if value.Code == code {
			return true
		}
	}
	return false
}
