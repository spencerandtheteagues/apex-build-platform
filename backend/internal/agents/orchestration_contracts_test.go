package agents

import (
	"strings"
	"testing"

	"apex-build/internal/ai"
)

func TestCompileIntentBriefFromRequestDetectsCapabilities(t *testing.T) {
	req := &BuildRequest{
		Description:         "Build a fullstack app with login, Stripe subscriptions, file uploads, OpenAI integration, and background jobs.",
		Mode:                ModeFull,
		PowerMode:           PowerFast,
		RequirePreviewReady: true,
	}

	brief := compileIntentBriefFromRequest(req, "platform")
	if brief == nil {
		t.Fatal("expected intent brief")
	}
	if brief.AppType != "fullstack" {
		t.Fatalf("app type = %q, want fullstack", brief.AppType)
	}
	if brief.CostSensitivity != CostSensitivityHigh {
		t.Fatalf("cost sensitivity = %q, want %q", brief.CostSensitivity, CostSensitivityHigh)
	}
	if !capabilityRequired(brief, CapabilityAuth) {
		t.Fatalf("expected auth capability in %+v", brief.RequiredCapabilities)
	}
	if !capabilityRequired(brief, CapabilityBilling) {
		t.Fatalf("expected billing capability in %+v", brief.RequiredCapabilities)
	}
	if !capabilityRequired(brief, CapabilityFileUpload) {
		t.Fatalf("expected file upload capability in %+v", brief.RequiredCapabilities)
	}
	if !capabilityRequired(brief, CapabilityJobs) {
		t.Fatalf("expected jobs capability in %+v", brief.RequiredCapabilities)
	}
	if !capabilityRequired(brief, CapabilityExternalAPI) {
		t.Fatalf("expected external api capability in %+v", brief.RequiredCapabilities)
	}
}

func TestCompileBuildContractFromPlanSeedsTruthAndVerification(t *testing.T) {
	plan := &BuildPlan{
		ID:      "plan-1",
		BuildID: "build-1",
		AppType: "fullstack",
		TechStack: TechStack{
			Frontend: "react",
			Backend:  "node",
			Database: "postgres",
		},
		Files: []PlannedFile{
			{Path: "src/App.tsx", Type: "frontend"},
			{Path: "src/pages/index.tsx", Type: "frontend"},
		},
		EnvVars: []BuildEnvVar{
			{Name: "OPENAI_API_KEY", Required: true},
		},
		Acceptance: []BuildAcceptanceCheck{
			{ID: "frontend", Owner: RoleFrontend, Description: "frontend renders"},
			{ID: "backend", Owner: RoleBackend, Description: "backend responds"},
			{ID: "integration", Owner: RoleTesting, Description: "frontend and backend align"},
		},
		APIContract: &BuildAPIContract{
			BackendPort: 3001,
			Endpoints: []APIEndpoint{
				{Method: "GET", Path: "/api/health", Description: "health"},
			},
		},
	}

	intent := &IntentBrief{
		AppType:              "fullstack",
		RequiredCapabilities: []CapabilityRequirement{CapabilityAuth, CapabilityExternalAPI},
	}

	contract := compileBuildContractFromPlan("build-1", intent, plan)
	if contract == nil {
		t.Fatal("expected contract")
	}
	if len(contract.VerificationGates) == 0 {
		t.Fatal("expected verification gates")
	}
	if tags := contract.TruthBySurface[string(SurfaceDeployment)]; len(tags) == 0 {
		t.Fatalf("expected deployment truth tags, got %+v", contract.TruthBySurface)
	}
	if !containsTruthTag(contract.TruthBySurface[string(SurfaceDeployment)], TruthNeedsSecrets) {
		t.Fatalf("expected deployment truth tags to include needs_secrets, got %+v", contract.TruthBySurface[string(SurfaceDeployment)])
	}
	if !containsTruthTag(contract.TruthBySurface[string(SurfaceIntegration)], TruthNeedsExternalAPI) {
		t.Fatalf("expected integration truth tags to include needs_external_api, got %+v", contract.TruthBySurface[string(SurfaceIntegration)])
	}
}

func TestVerifyAndNormalizeBuildContractBlocksMissingAuthAndSchema(t *testing.T) {
	intent := &IntentBrief{
		AppType:              "fullstack",
		RequiredCapabilities: []CapabilityRequirement{CapabilityAuth, CapabilityDatabase},
	}

	contract := &BuildContract{
		ID:                  "contract-1",
		BuildID:             "build-1",
		AppType:             "fullstack",
		RoutePageMap:        []ContractRoute{{Path: "/", Surface: SurfaceFrontend}},
		APIContract:         &BuildAPIContract{BackendPort: 3001, Endpoints: []APIEndpoint{{Method: "GET", Path: "/api/health"}}},
		RuntimeContract:     deriveRuntimeContractFromAppType("fullstack"),
		AcceptanceBySurface: deriveAcceptanceBySurfaceFromAppType("fullstack"),
		VerificationGates:   deriveVerificationGatesFromAppType("fullstack"),
	}

	verified, report := verifyAndNormalizeBuildContract(intent, contract)
	if verified == nil {
		t.Fatal("expected corrected contract")
	}
	if report.Status != VerificationBlocked {
		t.Fatalf("verification status = %q, want blocked", report.Status)
	}
	joined := strings.Join(report.Blockers, " | ")
	if !strings.Contains(joined, "auth capability requested without an auth contract") {
		t.Fatalf("expected auth blocker, got %v", report.Blockers)
	}
	if !strings.Contains(joined, "storage/database capability requested without schema entities") {
		t.Fatalf("expected schema blocker, got %v", report.Blockers)
	}
}

func TestGetAvailableProvidersWithGracePeriodForBuildFiltersOllamaForPlatform(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderOllama, ai.ProviderClaude, ai.ProviderGPT4},
			hasConfiguredProvider: true,
		},
	}

	build := &Build{
		UserID:       42,
		ProviderMode: "platform",
	}

	got := am.getAvailableProvidersWithGracePeriodForBuild(build)
	if len(got) != 2 {
		t.Fatalf("expected hosted providers only, got %v", got)
	}
	for _, provider := range got {
		if provider == ai.ProviderOllama {
			t.Fatalf("expected ollama to be filtered from platform build providers, got %v", got)
		}
	}
}

func TestRecordProviderTaskOutcomeUpdatesLiveScorecard(t *testing.T) {
	build := &Build{
		ID:           "build-scorecard-1",
		ProviderMode: "platform",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags:              defaultBuildOrchestrationFlags(),
				ProviderScorecards: defaultProviderScorecards("platform"),
			},
		},
	}

	recordProviderTaskOutcome(build, providerTaskOutcome{
		Provider:             ai.ProviderGPT4,
		TaskShape:            TaskShapeFrontendPatch,
		Success:              true,
		FirstPass:            true,
		VerificationObserved: true,
		VerificationPassed:   true,
		PromotionObserved:    true,
		PromotionSucceeded:   true,
		Truncated:            true,
		TotalTokens:          9000,
		Cost:                 0.18,
		LatencySeconds:       9.5,
	})

	state := build.SnapshotState.Orchestration
	if state == nil {
		t.Fatal("expected orchestration state")
	}
	var scorecard *ProviderScorecard
	for i := range state.ProviderScorecards {
		if state.ProviderScorecards[i].Provider == ai.ProviderGPT4 && state.ProviderScorecards[i].TaskShape == TaskShapeFrontendPatch {
			scorecard = &state.ProviderScorecards[i]
			break
		}
	}
	if scorecard == nil {
		t.Fatal("expected updated GPT4 frontend patch scorecard")
	}
	if scorecard.SampleCount == 0 || scorecard.SuccessCount == 0 {
		t.Fatalf("expected live scorecard counts, got %+v", scorecard)
	}
	if scorecard.FirstPassSampleCount == 0 || scorecard.FirstPassSuccessCount == 0 {
		t.Fatalf("expected first-pass verification counts, got %+v", scorecard)
	}
	if scorecard.TruncationEventCount == 0 || scorecard.TruncationRate <= 0 {
		t.Fatalf("expected truncation tracking, got %+v", scorecard)
	}
	if scorecard.TokenSampleCount == 0 || scorecard.AverageAcceptedTokens <= 7600 {
		t.Fatalf("expected accepted token average to move from prior, got %+v", scorecard)
	}
	if scorecard.CostSampleCount == 0 || scorecard.AverageCostPerSuccess <= 0.12 {
		t.Fatalf("expected cost average to move from prior, got %+v", scorecard)
	}
	if scorecard.LatencySampleCount == 0 || scorecard.AverageLatencySeconds <= 7.2 {
		t.Fatalf("expected latency average to move from prior, got %+v", scorecard)
	}
}

func containsTruthTag(tags []TruthTag, want TruthTag) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}
