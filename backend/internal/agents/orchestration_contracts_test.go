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

func TestCompileIntentBriefFromRequestHonorsNegatedBackendRequirements(t *testing.T) {
	req := &BuildRequest{
		Description: "Build a simple static marketing website. Frontend only. No backend. No database. No auth. No billing. No realtime.",
		Mode:        ModeFull,
		PowerMode:   PowerFast,
	}

	brief := compileIntentBriefFromRequest(req, "platform")
	if brief == nil {
		t.Fatal("expected intent brief")
	}
	if brief.AppType != "web" {
		t.Fatalf("app type = %q, want web", brief.AppType)
	}
	if capabilityRequired(brief, CapabilityAPI) || capabilityRequired(brief, CapabilityAuth) || capabilityRequired(brief, CapabilityDatabase) || capabilityRequired(brief, CapabilityBilling) || capabilityRequired(brief, CapabilityRealtime) {
		t.Fatalf("expected no paid/backend capabilities, got %+v", brief.RequiredCapabilities)
	}
}

func TestCompileIntentBriefFromRequestDoesNotTreatCleanFileStructureAsUploadStorage(t *testing.T) {
	req := &BuildRequest{
		Description: "Build a polished frontend-only client dashboard called PulseBoard using React 18, Vite, and Tailwind CSS with a responsive dark modern UI that works well in the preview pane, a dashboard home with KPI cards, trend widgets, an activity feed, and a highlighted primary action, a clients page with searchable cards, filters, empty states, and detail panels, a projects page with kanban-style status columns and clear progress visuals, a settings page with profile, notifications, and theme sections, realistic seed content in the UI so the preview feels complete immediately, strong loading, empty, and error states, reusable components and a clean file structure, and no backend, no database, and no fake API requirements in this free-tier preview pass.",
		Mode:        ModeFull,
		PowerMode:   PowerFast,
	}

	brief := compileIntentBriefFromRequest(req, "platform")
	if brief == nil {
		t.Fatal("expected intent brief")
	}
	if brief.AppType != "web" {
		t.Fatalf("app type = %q, want web", brief.AppType)
	}
	if capabilityRequired(brief, CapabilityStorage) {
		t.Fatalf("did not expect storage capability for clean file structure prompt, got %+v", brief.RequiredCapabilities)
	}
	if capabilityRequired(brief, CapabilityFileUpload) {
		t.Fatalf("did not expect file_upload capability for clean file structure prompt, got %+v", brief.RequiredCapabilities)
	}
	if capabilityRequired(brief, CapabilityAPI) || capabilityRequired(brief, CapabilityDatabase) {
		t.Fatalf("did not expect backend/database capabilities for frontend-only canary prompt, got %+v", brief.RequiredCapabilities)
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
		DataModels: []DataModel{
			{
				Name: "Project",
				Fields: []ModelField{
					{Name: "id", Type: "uuid", Required: true, Unique: false},
					{Name: "name", Type: "string", Required: true},
				},
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
	if len(contract.DBSchemaContract) != 1 || len(contract.DBSchemaContract[0].Fields) == 0 || !contract.DBSchemaContract[0].Fields[0].Unique {
		t.Fatalf("expected compileBuildContractFromPlan to normalize canonical id field uniqueness, got %+v", contract.DBSchemaContract)
	}
}

func TestCompileBuildContractFromPlanNormalizesUniqueTypeQualifiers(t *testing.T) {
	t.Parallel()

	plan := &BuildPlan{
		ID:      "plan-qualifiers",
		BuildID: "build-qualifiers",
		AppType: "fullstack",
		TechStack: TechStack{
			Frontend: "react",
			Backend:  "node",
			Database: "postgres",
		},
		DataModels: []DataModel{
			{
				Name: "Tenant",
				Fields: []ModelField{
					{Name: "slug", Type: "string unique"},
				},
			},
			{
				Name: "User",
				Fields: []ModelField{
					{Name: "email", Type: "string unique"},
				},
			},
		},
	}

	contract := compileBuildContractFromPlan("build-qualifiers", &IntentBrief{AppType: "fullstack"}, plan)
	if contract == nil {
		t.Fatal("expected contract")
	}
	if len(contract.DBSchemaContract) != 2 {
		t.Fatalf("expected normalized schema models, got %+v", contract.DBSchemaContract)
	}

	tenantField := contract.DBSchemaContract[0].Fields[0]
	userField := contract.DBSchemaContract[1].Fields[0]
	if tenantField.Type != "string" || !tenantField.Unique {
		t.Fatalf("expected tenant slug field to normalize unique qualifier, got %+v", tenantField)
	}
	if userField.Type != "string" || !userField.Unique {
		t.Fatalf("expected user email field to normalize unique qualifier, got %+v", userField)
	}
}

func TestCompileBuildContractFromPlanInfersForeignKeyReferences(t *testing.T) {
	t.Parallel()

	plan := &BuildPlan{
		ID:      "plan-fk-refs",
		BuildID: "build-fk-refs",
		AppType: "fullstack",
		TechStack: TechStack{
			Frontend: "react",
			Backend:  "node",
			Database: "postgres",
		},
		DataModels: []DataModel{
			{
				Name: "Tenant",
				Fields: []ModelField{
					{Name: "id", Type: "uuid primary key"},
				},
			},
			{
				Name: "User",
				Fields: []ModelField{
					{Name: "tenant_id", Type: "uuid foreign key"},
				},
			},
		},
	}

	contract := compileBuildContractFromPlan("build-fk-refs", &IntentBrief{AppType: "fullstack"}, plan)
	if contract == nil {
		t.Fatal("expected contract")
	}
	if len(contract.DBSchemaContract) != 2 {
		t.Fatalf("expected normalized schema models, got %+v", contract.DBSchemaContract)
	}

	userField := contract.DBSchemaContract[1].Fields[0]
	if userField.Type != "uuid foreign key references Tenant(id)" {
		t.Fatalf("expected tenant_id foreign key reference to be inferred, got %+v", userField)
	}
}

func TestCompileBuildContractFromPlanInfersActorForeignKeyReferences(t *testing.T) {
	t.Parallel()

	plan := &BuildPlan{
		ID:      "plan-actor-fk-refs",
		BuildID: "build-actor-fk-refs",
		AppType: "fullstack",
		TechStack: TechStack{
			Frontend: "react",
			Backend:  "node",
			Database: "postgres",
		},
		DataModels: []DataModel{
			{
				Name: "User",
				Fields: []ModelField{
					{Name: "id", Type: "uuid primary key"},
				},
			},
			{
				Name: "Task",
				Fields: []ModelField{
					{Name: "created_by", Type: "uuid foreign key"},
					{Name: "assigned_to", Type: "uuid foreign key"},
				},
			},
		},
	}

	contract := compileBuildContractFromPlan("build-actor-fk-refs", &IntentBrief{AppType: "fullstack"}, plan)
	if contract == nil {
		t.Fatal("expected contract")
	}
	if len(contract.DBSchemaContract) != 2 {
		t.Fatalf("expected normalized schema models, got %+v", contract.DBSchemaContract)
	}

	taskFields := contract.DBSchemaContract[1].Fields
	if taskFields[0].Type != "uuid foreign key references User(id)" {
		t.Fatalf("expected created_by foreign key reference to be inferred, got %+v", taskFields[0])
	}
	if taskFields[1].Type != "uuid foreign key references User(id)" {
		t.Fatalf("expected assigned_to foreign key reference to be inferred, got %+v", taskFields[1])
	}
}

func TestCompileBuildContractFromPlanNormalizesActorRelationTargetsToIdentityModel(t *testing.T) {
	t.Parallel()

	plan := &BuildPlan{
		ID:      "plan-actor-relation-targets",
		BuildID: "build-actor-relation-targets",
		AppType: "fullstack",
		TechStack: TechStack{
			Frontend: "react",
			Backend:  "node",
			Database: "postgres",
		},
		DataModels: []DataModel{
			{
				Name: "User",
				Fields: []ModelField{
					{Name: "id", Type: "uuid primary key"},
				},
			},
			{
				Name: "Project",
				Fields: []ModelField{
					{Name: "manager_id", Type: "uuid foreign key"},
				},
				Relations: []Relation{
					{Field: "manager_id", Target: "Manager"},
				},
			},
		},
	}

	contract := compileBuildContractFromPlan("build-actor-relation-targets", &IntentBrief{AppType: "fullstack"}, plan)
	if contract == nil {
		t.Fatal("expected contract")
	}
	if len(contract.DBSchemaContract) != 2 {
		t.Fatalf("expected normalized schema models, got %+v", contract.DBSchemaContract)
	}

	managerField := contract.DBSchemaContract[1].Fields[0]
	if managerField.Type != "uuid foreign key references User(id)" {
		t.Fatalf("expected manager_id foreign key relation target to normalize to User(id), got %+v", managerField)
	}
}

func TestCompileBuildContractFromPlanSeedsAuthEndpointsFromIntent(t *testing.T) {
	t.Parallel()

	plan := &BuildPlan{
		ID:      "plan-auth-seed",
		BuildID: "build-auth-seed",
		AppType: "fullstack",
		TechStack: TechStack{
			Frontend: "React",
			Backend:  "Express",
			Database: "SQLite",
		},
		EnvVars: []BuildEnvVar{
			{Name: "JWT_SECRET", Required: true},
		},
	}

	intent := &IntentBrief{
		AppType:              "fullstack",
		NormalizedRequest:    "Build a CRM where users can create an account, log in, and manage clients from a dashboard.",
		RequiredCapabilities: []CapabilityRequirement{CapabilityAPI, CapabilityAuth},
	}

	contract := compileBuildContractFromPlan("build-auth-seed", intent, plan)
	if contract == nil {
		t.Fatal("expected contract")
	}
	if contract.APIContract == nil {
		t.Fatal("expected API contract to be seeded")
	}
	if contract.APIContract.BackendPort != 3001 {
		t.Fatalf("expected backend port 3001, got %+v", contract.APIContract)
	}
	if contract.APIContract.APIBaseURL != "/api" {
		t.Fatalf("expected API base URL /api, got %+v", contract.APIContract)
	}
	if contract.AuthContract == nil || !contract.AuthContract.Required || contract.AuthContract.TokenStrategy != "token" {
		t.Fatalf("expected auth contract with token strategy, got %+v", contract.AuthContract)
	}

	endpoints := make(map[string]bool)
	for _, endpoint := range contract.APIContract.Endpoints {
		endpoints[strings.ToUpper(strings.TrimSpace(endpoint.Method))+" "+strings.TrimSpace(endpoint.Path)] = true
	}
	for _, key := range []string{
		"GET /api/health",
		"POST /api/auth/login",
		"GET /api/auth/me",
		"POST /api/auth/register",
	} {
		if !endpoints[key] {
			t.Fatalf("expected seeded API endpoint %q, got %+v", key, contract.APIContract.Endpoints)
		}
	}

	backendDeps := make(map[string]bool)
	for _, resource := range contract.BackendResourceMap {
		for _, dep := range resource.DependsOn {
			backendDeps[dep] = true
		}
	}
	for _, dep := range []string{"POST /api/auth/login", "GET /api/auth/me", "POST /api/auth/register"} {
		if !backendDeps[dep] {
			t.Fatalf("expected backend resource dependency %q, got %+v", dep, contract.BackendResourceMap)
		}
	}
}

func TestVerifyAndNormalizeBuildContractBlocksMissingAuthAndWarnsOnMissingSchema(t *testing.T) {
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
	warnings := strings.Join(report.Warnings, " | ")
	if !strings.Contains(warnings, "database capability detected but no schema entities were pre-planned") {
		t.Fatalf("expected schema warning, got blockers=%v warnings=%v", report.Blockers, report.Warnings)
	}
}

func TestVerifyAndNormalizeBuildContractAcceptsNextFullstackScaffoldContract(t *testing.T) {
	t.Parallel()

	scaffold := selectBuildScaffold("fullstack", TechStack{Frontend: "Next.js", Backend: "Express"})
	plan := &BuildPlan{
		ID:          "plan-next-fullstack",
		BuildID:     "build-next-fullstack",
		AppType:     scaffold.AppType,
		TechStack:   TechStack{Frontend: "Next.js", Backend: "Express"},
		Files:       append([]PlannedFile(nil), scaffold.Required...),
		Acceptance:  append([]BuildAcceptanceCheck(nil), scaffold.Acceptance...),
		APIContract: cloneAPIContract(scaffold.APIContract),
	}
	intent := &IntentBrief{
		AppType:              "fullstack",
		RequiredCapabilities: []CapabilityRequirement{CapabilityAPI},
	}

	contract := compileBuildContractFromPlan("build-next-fullstack", intent, plan)
	if contract == nil {
		t.Fatal("expected next fullstack contract")
	}

	verified, report := verifyAndNormalizeBuildContract(intent, contract)
	if verified == nil {
		t.Fatal("expected corrected next fullstack contract")
	}
	if report.Status == VerificationBlocked {
		t.Fatalf("expected next fullstack scaffold contract to verify, got blockers %v", report.Blockers)
	}
	if verified.APIContract == nil || len(verified.APIContract.Endpoints) == 0 {
		t.Fatalf("expected verified API contract, got %+v", verified.APIContract)
	}
	if !hasSurface(verified.AcceptanceBySurface, SurfaceDeployment) {
		t.Fatalf("expected deployment acceptance surface, got %+v", verified.AcceptanceBySurface)
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

func TestAppendPatchBundleUpdatesTruthBySurface(t *testing.T) {
	build := &Build{
		ID: "build-truth-patch",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
				BuildContract: &BuildContract{
					ID:      "contract-truth",
					BuildID: "build-truth-patch",
					TruthBySurface: map[string][]TruthTag{
						string(SurfaceFrontend): {TruthScaffolded, TruthBlocked},
					},
				},
			},
		},
	}

	appendPatchBundle(build, PatchBundle{
		ID:      "bundle-truth",
		BuildID: build.ID,
		Operations: []PatchOperation{
			{Type: PatchReplaceFunction, Path: "src/App.tsx", Content: "export default function App(){ return <main>ok</main> }\n"},
		},
	})

	state := build.SnapshotState.Orchestration
	if state == nil || state.BuildContract == nil {
		t.Fatal("expected orchestration build contract")
	}
	tags := state.BuildContract.TruthBySurface[string(SurfaceFrontend)]
	if containsTruthTag(tags, TruthScaffolded) || containsTruthTag(tags, TruthBlocked) {
		t.Fatalf("expected patch promotion to clear scaffolded/blocked tags, got %+v", tags)
	}
	if !containsTruthTag(tags, TruthPartiallyWired) {
		t.Fatalf("expected patch promotion to mark frontend partially wired, got %+v", tags)
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
