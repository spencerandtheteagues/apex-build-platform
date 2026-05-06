package agents

import (
	"strings"
	"testing"

	"apex-build/internal/mobile"
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

func TestCompilePrecomputedValidatedBuildSpecExpandsSecurityAndPerformanceHeuristics(t *testing.T) {
	t.Parallel()

	prompt := "Build a multi-tenant AI support workspace with role-based admin controls, Stripe subscriptions, realtime activity feed, analytics dashboards, and assistant chat."
	spec := compilePrecomputedValidatedBuildSpec(&BuildRequest{
		Prompt:              prompt,
		RequirePreviewReady: true,
	}, &IntentBrief{
		AppType:           "fullstack",
		NormalizedRequest: prompt,
		RequiredCapabilities: []CapabilityRequirement{
			CapabilityAuth,
			CapabilityDatabase,
			CapabilityBilling,
			CapabilityRealtime,
			CapabilityExternalAPI,
		},
	})

	if spec == nil {
		t.Fatal("expected precomputed validated build spec")
	}

	wantSecurityCodes := []string{
		"role_boundary_enforcement",
		"billing_webhook_verification",
		"tenant_isolation",
		"ai_prompt_boundary",
	}
	for _, code := range wantSecurityCodes {
		if !hasBuildSpecAdvisoryCode(spec.SecurityAdvisories, code) {
			t.Fatalf("expected security advisory %q, got %+v", code, spec.SecurityAdvisories)
		}
	}

	wantPerformanceCodes := []string{
		"progressive_dashboard_loading",
		"feed_windowing",
		"upstream_latency_budget",
	}
	for _, code := range wantPerformanceCodes {
		if !hasBuildSpecAdvisoryCode(spec.PerformanceAdvisories, code) {
			t.Fatalf("expected performance advisory %q, got %+v", code, spec.PerformanceAdvisories)
		}
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

func TestValidatedBuildSpecCarriesMobileTargetMetadata(t *testing.T) {
	t.Parallel()

	prompt := "Build an Android and iOS field-service mobile app with camera uploads, offline drafts, login, and push reminders."
	intent := compileIntentBriefFromRequest(&BuildRequest{Prompt: prompt}, "platform")
	spec := compilePrecomputedValidatedBuildSpec(&BuildRequest{Prompt: prompt}, intent)
	if spec == nil {
		t.Fatal("expected precomputed spec")
	}
	if spec.TargetPlatform != mobile.TargetPlatformMobileExpo {
		t.Fatalf("target platform = %q, want mobile_expo", spec.TargetPlatform)
	}
	if spec.MobileFramework != mobile.MobileFrameworkExpoReactNative {
		t.Fatalf("mobile framework = %q, want Expo React Native", spec.MobileFramework)
	}
	if !hasValidatedMobilePlatform(spec.MobilePlatforms, mobile.MobilePlatformAndroid) || !hasValidatedMobilePlatform(spec.MobilePlatforms, mobile.MobilePlatformIOS) {
		t.Fatalf("expected Android and iOS platforms, got %+v", spec.MobilePlatforms)
	}
	if !hasValidatedMobileCapability(spec.MobileCapabilities, mobile.CapabilityCamera) || !hasValidatedMobileCapability(spec.MobileCapabilities, mobile.CapabilityOfflineMode) {
		t.Fatalf("expected camera and offline capabilities, got %+v", spec.MobileCapabilities)
	}

	plan := &BuildPlan{
		AppType:            "fullstack",
		DeliveryMode:       "mobile_source_only",
		TargetPlatform:     mobile.TargetPlatformMobileExpo,
		MobilePlatforms:    []mobile.MobilePlatform{mobile.MobilePlatformAndroid},
		MobileFramework:    mobile.MobileFrameworkExpoReactNative,
		MobileReleaseLevel: mobile.ReleaseSourceOnly,
		MobileCapabilities: []mobile.MobileCapability{mobile.CapabilityFileUploads},
	}
	contract := compileBuildContractFromPlan("build-mobile-validated", intent, plan)
	locked := finalizeValidatedBuildSpec("build-mobile-validated", spec, plan, contract)
	if locked == nil || !locked.Locked {
		t.Fatalf("expected locked validated spec, got %+v", locked)
	}
	if !hasValidatedMobilePlatform(locked.MobilePlatforms, mobile.MobilePlatformAndroid) {
		t.Fatalf("expected locked mobile platform metadata, got %+v", locked.MobilePlatforms)
	}
	if !hasValidatedMobileCapability(locked.MobileCapabilities, mobile.CapabilityFileUploads) {
		t.Fatalf("expected locked mobile capability override, got %+v", locked.MobileCapabilities)
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

func hasValidatedMobilePlatform(platforms []mobile.MobilePlatform, want mobile.MobilePlatform) bool {
	for _, platform := range platforms {
		if platform == want {
			return true
		}
	}
	return false
}

func hasValidatedMobileCapability(capabilities []mobile.MobileCapability, want mobile.MobileCapability) bool {
	for _, capability := range capabilities {
		if capability == want {
			return true
		}
	}
	return false
}

func hasBuildSpecAdvisoryCode(values []BuildSpecAdvisory, code string) bool {
	for _, advisory := range values {
		if advisory.Code == code {
			return true
		}
	}
	return false
}
