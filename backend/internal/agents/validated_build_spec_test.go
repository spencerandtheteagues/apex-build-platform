package agents

import (
	"strings"
	"testing"
)

func TestCompilePrecomputedValidatedBuildSpecIncludesAdvisories(t *testing.T) {
	t.Parallel()

	spec := compilePrecomputedValidatedBuildSpec(&BuildRequest{
		Prompt:              "Build a full-stack CRM dashboard with auth, file upload, search, and Stripe billing.",
		RequirePreviewReady: true,
	}, &IntentBrief{
		AppType:           "fullstack",
		NormalizedRequest: "Build a full-stack CRM dashboard with auth, file upload, search, and Stripe billing.",
		RequiredCapabilities: []CapabilityRequirement{
			CapabilityAuth,
			CapabilityFileUpload,
			CapabilitySearch,
			CapabilityBilling,
		},
	})

	if spec == nil {
		t.Fatal("expected precomputed validated build spec")
	}
	if spec.Locked {
		t.Fatalf("expected precomputed spec to be advisory until plan lock")
	}
	if len(spec.SecurityAdvisories) == 0 {
		t.Fatalf("expected security advisories, got %+v", spec)
	}
	if len(spec.PerformanceAdvisories) == 0 {
		t.Fatalf("expected performance advisories, got %+v", spec)
	}
}

func TestFinalizeValidatedBuildSpecLocksPlanDetails(t *testing.T) {
	t.Parallel()

	plan := &BuildPlan{
		BuildID:      "build-validated-spec",
		AppType:      "fullstack",
		DeliveryMode: "full_stack_preview",
		Features: []Feature{
			{Name: "Client dashboard", Description: "See client records and status chips"},
		},
		Components: []UIComponent{
			{Name: "Dashboard", State: []string{"filters", "selectedClient"}},
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
			Endpoints: []APIEndpoint{
				{Method: "GET", Path: "/api/clients"},
			},
		},
		AcceptanceBySurface: []SurfaceAcceptanceContract{
			{Surface: SurfaceFrontend, Required: true},
			{Surface: SurfaceBackend, Required: true},
		},
	}

	spec := finalizeValidatedBuildSpec("build-validated-spec", compilePrecomputedValidatedBuildSpec(&BuildRequest{
		Prompt: "Build a client dashboard",
	}, &IntentBrief{AppType: "fullstack", NormalizedRequest: "Build a client dashboard"}), plan, contract)
	if spec == nil {
		t.Fatal("expected finalized spec")
	}
	if !spec.Locked {
		t.Fatalf("expected finalized validated spec to lock before generation")
	}
	if len(spec.RoutePlan) == 0 || len(spec.APIPaths) == 0 {
		t.Fatalf("expected finalized spec to include routes and api paths, got %+v", spec)
	}
}

func TestBuildTaskPromptIncludesValidatedBuildSpecContext(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		Description: "Build a customer portal",
		Plan: &BuildPlan{
			SpecHash: "spec-1",
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				ValidatedBuildSpec: &ValidatedBuildSpec{
					AppType:          "web",
					DeliveryMode:     "frontend_preview",
					PrimaryUserFlows: []string{"land in the portal and view customer status"},
					Locked:           true,
				},
			},
		},
	}
	task := &Task{Type: TaskGenerateUI, Description: "Build the portal shell"}
	agent := &Agent{Role: RoleFrontend}

	prompt := am.buildTaskPrompt(task, build, agent)
	if !strings.Contains(prompt, "<validated_build_spec>") {
		t.Fatalf("expected validated build spec context in task prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "This spec is locked for generation") {
		t.Fatalf("expected lock language in task prompt, got %q", prompt)
	}
}
