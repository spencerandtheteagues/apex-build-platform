package agents

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"apex-build/internal/agents/autonomous"
	"apex-build/internal/ai"
)

func TestCreateBuildPlanFromPlanningBundle(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	bundle := &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "fullstack",
			Features: []autonomous.Feature{
				{Name: "Transcript import", Description: "Ingest and process transcripts", Priority: "high"},
			},
			DataModels: []autonomous.DataModel{
				{Name: "Transcript", Fields: map[string]string{"id": "uuid", "title": "string", "completedAt": "datetime | null"}},
			},
			TechStack: &autonomous.TechStack{
				Frontend: "React",
				Backend:  "Node",
				Database: "PostgreSQL",
				Styling:  "Tailwind",
			},
			PreflightChecks: []autonomous.PreflightCheck{
				{Name: "docker", Description: "Docker should be installed", Required: false},
			},
		},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-1",
			EstimatedTime: 90 * time.Minute,
			CreatedAt:     now,
			Steps: []*autonomous.PlanStep{
				{
					ID:         "step-1",
					Name:       "Create Data Models",
					ActionType: autonomous.ActionAIGenerate,
					Input:      map[string]interface{}{"type": "data_models"},
				},
				{
					ID:         "step-2",
					Name:       "Create Backend",
					ActionType: autonomous.ActionAIGenerate,
					Input:      map[string]interface{}{"type": "backend"},
				},
				{
					ID:         "step-3",
					Name:       "Create Frontend",
					ActionType: autonomous.ActionAIGenerate,
					Input:      map[string]interface{}{"type": "frontend"},
				},
			},
		},
	}

	plan := createBuildPlanFromPlanningBundle("build-1", "Build TranscriptVault", nil, bundle)
	if plan == nil {
		t.Fatal("expected build plan")
	}
	if plan.SpecHash == "" {
		t.Fatal("expected non-empty spec hash")
	}
	if plan.ScaffoldID != "fullstack/react-vite-express-ts" {
		t.Fatalf("unexpected scaffold: %s", plan.ScaffoldID)
	}
	if plan.APIContract == nil || plan.APIContract.BackendPort != 3001 {
		t.Fatalf("expected default fullstack api contract, got %+v", plan.APIContract)
	}
	if len(plan.WorkOrders) == 0 {
		t.Fatal("expected work orders")
	}
	if len(plan.DataModels) != 1 {
		t.Fatalf("expected one normalized data model, got %+v", plan.DataModels)
	}
	var idField *ModelField
	var nullableField *ModelField
	for i := range plan.DataModels[0].Fields {
		switch plan.DataModels[0].Fields[i].Name {
		case "id":
			idField = &plan.DataModels[0].Fields[i]
		case "completedAt":
			nullableField = &plan.DataModels[0].Fields[i]
		}
	}
	if idField == nil || !idField.Unique {
		t.Fatalf("expected canonical id field to be marked unique, got %+v", plan.DataModels[0].Fields)
	}
	if nullableField == nil || nullableField.Required {
		t.Fatalf("expected nullable completedAt field to be non-required, got %+v", plan.DataModels[0].Fields)
	}
	if wo := getBuildWorkOrder(plan, RoleFrontend); wo == nil || len(wo.OwnedFiles) == 0 {
		t.Fatalf("expected frontend work order, got %+v", wo)
	} else if !pathAllowedByWorkOrder("package.json", wo) {
		t.Fatalf("expected shared root manifest to be allowed for frontend work order, got %+v", wo)
	}
	if wo := getBuildWorkOrder(plan, RoleArchitect); wo == nil {
		t.Fatalf("expected architect work order, got %+v", wo)
	} else {
		if !strings.Contains(strings.ToLower(wo.Summary), "contract") {
			t.Fatalf("expected architect summary to mention contract freeze, got %q", wo.Summary)
		}
		if !strings.Contains(strings.ToLower(strings.Join(wo.RequiredOutputs, " ")), "frozen ui, api, data, and env contract") {
			t.Fatalf("expected architect outputs to include frozen contract guidance, got %+v", wo.RequiredOutputs)
		}
	}
	if len(plan.ScaffoldFiles) == 0 {
		t.Fatal("expected deterministic scaffold files")
	}

	filePaths := make([]string, 0, len(plan.Files))
	for _, file := range plan.Files {
		filePaths = append(filePaths, file.Path)
	}
	joined := strings.Join(filePaths, ",")
	for _, required := range []string{"package.json", "src/main.tsx", "server/index.ts", "migrations/001_initial.sql"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("expected planned files to include %s; got %s", required, joined)
		}
	}

	scaffoldPaths := make([]string, 0, len(plan.ScaffoldFiles))
	for _, file := range plan.ScaffoldFiles {
		scaffoldPaths = append(scaffoldPaths, file.Path)
	}
	scaffoldJoined := strings.Join(scaffoldPaths, ",")
	for _, required := range []string{"package.json", "server/index.ts", "src/main.tsx", "README.md"} {
		if !strings.Contains(scaffoldJoined, required) {
			t.Fatalf("expected scaffold files to include %s; got %s", required, scaffoldJoined)
		}
	}

	var packageOwners []AgentRole
	for _, ownership := range plan.Ownership {
		if ownership.Path == "package.json" {
			packageOwners = append(packageOwners, ownership.Role)
		}
	}
	if len(packageOwners) != 1 || packageOwners[0] != RoleBackend {
		t.Fatalf("expected package.json to have a single backend owner, got %+v", packageOwners)
	}
}

func TestCreateBuildPlanFromPlanningBundleDoesNotInventDatabaseSurfaceForResponseModels(t *testing.T) {
	t.Parallel()

	description := "Build a minimal full-stack uptime monitor with React and Express. The backend must expose GET /api/health and GET /api/metrics returning JSON response models. No database, no auth, no external APIs."
	plan := createBuildPlanFromPlanningBundle("build-no-db-response-models", description, &TechStack{
		Frontend: "React",
		Backend:  "Express",
		Styling:  "Tailwind",
	}, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "fullstack",
			TechStack: &autonomous.TechStack{
				Frontend: "React",
				Backend:  "Node",
				Styling:  "Tailwind",
			},
			DataModels: []autonomous.DataModel{
				{Name: "HealthResponse", Fields: map[string]string{"status": "string", "serverTime": "string", "uptimeSeconds": "number"}},
				{Name: "MetricsResponse", Fields: map[string]string{"requestCount": "number", "uptimeSeconds": "number"}},
			},
		},
	})

	if plan == nil {
		t.Fatal("expected build plan")
	}
	if len(plan.DataModels) != 0 {
		t.Fatalf("expected API response models not to become persistent data models without database intent, got %+v", plan.DataModels)
	}
	if wo := getBuildWorkOrder(plan, RoleDatabase); wo != nil {
		t.Fatalf("expected no database work order for explicit no-database build, got %+v", wo)
	}
	for _, env := range plan.EnvVars {
		if strings.EqualFold(strings.TrimSpace(env.Name), "DATABASE_URL") && env.Required {
			t.Fatalf("expected DATABASE_URL not to be required for no-database full-stack build, got %+v", plan.EnvVars)
		}
	}
	for _, file := range plan.ScaffoldFiles {
		if file.Path == ".env.example" && strings.Contains(file.Content, "DATABASE_URL") {
			t.Fatalf("expected deterministic .env.example to omit DATABASE_URL for no-database build, got %q", file.Content)
		}
	}
}

func TestNormalizeModelFieldsPromotesTypeQualifiersToFlags(t *testing.T) {
	t.Parallel()

	fields := normalizeModelFields([]ModelField{
		{Name: "slug", Type: "string unique"},
		{Name: "email", Type: "TEXT NOT NULL UNIQUE"},
		{Name: "completedAt", Type: "datetime | null", Required: true},
	})

	if len(fields) != 3 {
		t.Fatalf("expected 3 normalized fields, got %+v", fields)
	}

	if fields[0].Type != "string" || !fields[0].Unique {
		t.Fatalf("expected slug field to normalize unique qualifier, got %+v", fields[0])
	}
	if fields[1].Type != "TEXT" || !fields[1].Unique || !fields[1].Required {
		t.Fatalf("expected SQL qualifiers to normalize into flags, got %+v", fields[1])
	}
	if fields[2].Type != "datetime" || fields[2].Required {
		t.Fatalf("expected nullable qualifier to clear required flag, got %+v", fields[2])
	}
}

func TestCreateBuildPlanFromPlanningBundleHonorsStaticFrontendIntent(t *testing.T) {
	t.Parallel()

	description := "Build a polished static marketing site for an AI operations studio with a hero section, services grid, testimonials, FAQ, and pricing. Frontend only. No backend. No database. No auth. No billing. No realtime."
	plan := createBuildPlanFromPlanningBundle("build-static-1", description, nil, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-static-1",
			EstimatedTime: 20 * time.Minute,
			CreatedAt:     time.Now().UTC(),
		},
	})

	if plan == nil {
		t.Fatal("expected build plan")
	}
	if plan.AppType != "web" {
		t.Fatalf("expected web app type, got %q", plan.AppType)
	}
	if plan.TechStack.Frontend != "React" {
		t.Fatalf("expected React frontend fallback, got %+v", plan.TechStack)
	}
	if plan.TechStack.Backend != "" || plan.TechStack.Database != "" {
		t.Fatalf("expected frontend-only fallback stack, got %+v", plan.TechStack)
	}
	if plan.ScaffoldID != "frontend/react-vite-spa" {
		t.Fatalf("expected frontend scaffold, got %q", plan.ScaffoldID)
	}
	if plan.APIContract != nil {
		t.Fatalf("expected no api contract for static web plan, got %+v", plan.APIContract)
	}

	intent := &IntentBrief{AppType: "web"}
	contract := compileBuildContractFromPlan(plan.BuildID, intent, plan)
	if contract == nil {
		t.Fatal("expected build contract")
	}
	verified, report := verifyAndNormalizeBuildContract(intent, contract)
	if verified == nil {
		t.Fatal("expected verified build contract")
	}
	if report.Status == VerificationBlocked {
		t.Fatalf("expected static web contract to verify, got blockers %v", report.Blockers)
	}
}

func TestCreateBuildPlanFromPlanningBundleIgnoresContradictoryPlannerBackendForStaticIntent(t *testing.T) {
	t.Parallel()

	description := "Build a polished static marketing site for an AI operations studio with a hero section, services grid, testimonials, FAQ, and pricing. Frontend only. No backend. No database. No auth. No billing. No realtime."
	plan := createBuildPlanFromPlanningBundle("build-static-2", description, nil, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "web",
			TechStack: &autonomous.TechStack{
				Frontend: "React",
				Backend:  "Node",
				Database: "PostgreSQL",
				Styling:  "Tailwind",
			},
		},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-static-2",
			EstimatedTime: 20 * time.Minute,
			CreatedAt:     time.Now().UTC(),
		},
	})

	if plan == nil {
		t.Fatal("expected build plan")
	}
	if plan.TechStack.Backend != "" || plan.TechStack.Database != "" {
		t.Fatalf("expected static intent to strip contradictory planner backend/database, got %+v", plan.TechStack)
	}
	if plan.ScaffoldID != "frontend/react-vite-spa" {
		t.Fatalf("expected frontend scaffold, got %q", plan.ScaffoldID)
	}
	if plan.APIContract != nil {
		t.Fatalf("expected no api contract for contradictory static plan, got %+v", plan.APIContract)
	}
	if len(plan.DataModels) != 0 {
		t.Fatalf("expected no planner data models for static intent without a database, got %+v", plan.DataModels)
	}
}

func TestApplyBuildAssurancePolicyToPlanDowngradesFreeFullStackToFrontendPreview(t *testing.T) {
	t.Parallel()

	build := &Build{
		ID:               "free-build",
		UserID:           11,
		SubscriptionPlan: "free",
		Description:      "Build a full-stack CRM with auth, database-backed clients, projects, and reporting dashboards",
		SnapshotState: BuildSnapshotState{
			PolicyState: &BuildPolicyState{
				PlanType:           "free",
				Classification:     BuildClassificationUpgradeRequired,
				UpgradeRequired:    true,
				StaticFrontendOnly: true,
				RequiredPlan:       "builder",
			},
		},
	}

	plan := createBuildPlanFromPlanningBundle("build-free-fallback", build.Description, nil, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "fullstack",
			TechStack: &autonomous.TechStack{
				Frontend: "React",
				Backend:  "Node",
				Database: "PostgreSQL",
				Styling:  "Tailwind",
			},
		},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-free-fallback",
			EstimatedTime: 25 * time.Minute,
			CreatedAt:     time.Now().UTC(),
			Steps: []*autonomous.PlanStep{
				{ID: "frontend", ActionType: autonomous.ActionAIGenerate, Input: map[string]any{"type": "frontend"}},
				{ID: "backend", ActionType: autonomous.ActionAIGenerate, Input: map[string]any{"type": "backend"}},
				{ID: "data", ActionType: autonomous.ActionAIGenerate, Input: map[string]any{"type": "data_models"}},
			},
		},
	})

	plan = applyBuildAssurancePolicyToPlan(build, plan)
	if plan == nil {
		t.Fatal("expected plan")
	}
	if plan.AppType != "web" {
		t.Fatalf("expected web app type after fallback, got %q", plan.AppType)
	}
	if plan.DeliveryMode != "frontend_preview_only" {
		t.Fatalf("expected frontend preview delivery mode, got %q", plan.DeliveryMode)
	}
	if plan.TechStack.Backend != "" || plan.TechStack.Database != "" {
		t.Fatalf("expected backend/database to be stripped, got %+v", plan.TechStack)
	}
	if plan.APIContract != nil {
		t.Fatalf("expected no API contract in frontend-only fallback, got %+v", plan.APIContract)
	}
	if plan.ScaffoldID != "frontend/react-vite-spa" {
		t.Fatalf("expected frontend scaffold after fallback, got %q", plan.ScaffoldID)
	}
	if wo := getBuildWorkOrder(plan, RoleBackend); wo != nil {
		t.Fatalf("expected backend work order to be removed, got %+v", wo)
	}
	if wo := getBuildWorkOrder(plan, RoleDatabase); wo != nil {
		t.Fatalf("expected database work order to be removed, got %+v", wo)
	}

	joinedFiles := make([]string, 0, len(plan.Files))
	for _, file := range plan.Files {
		joinedFiles = append(joinedFiles, file.Path)
	}
	if strings.Contains(strings.Join(joinedFiles, ","), "server/index.ts") || strings.Contains(strings.Join(joinedFiles, ","), "migrations/001_initial.sql") {
		t.Fatalf("expected backend/database files to be removed, got %+v", joinedFiles)
	}

	intent := &IntentBrief{
		AppType:               "fullstack",
		RequiredCapabilities:  []CapabilityRequirement{CapabilityAuth, CapabilityDatabase, CapabilityAPI},
		AcceptanceSummarySeed: []string{"interactive preview serves the app"},
	}
	contract := compileBuildContractFromPlan(build.ID, intent, plan)
	if contract == nil {
		t.Fatal("expected build contract")
	}
	verified, report := verifyAndNormalizeBuildContract(intent, contract)
	if verified == nil {
		t.Fatal("expected verified contract")
	}
	if report.Status == VerificationBlocked {
		t.Fatalf("expected frontend preview fallback contract to verify, got blockers %v", report.Blockers)
	}
}

func TestApplyBuildAssurancePolicyToPlanRunsPaidFullStackWithoutFrontendApprovalByDefault(t *testing.T) {
	t.Parallel()

	for _, mode := range []PowerMode{PowerBalanced, PowerMax} {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			build := &Build{
				ID:               "paid-build-" + string(mode),
				UserID:           22,
				SubscriptionPlan: "builder",
				Description:      "Build a full-stack CRM with auth, database-backed clients, projects, and reporting dashboards",
				PowerMode:        mode,
			}

			plan := createBuildPlanFromPlanningBundle("build-paid-frontstage-"+string(mode), build.Description, nil, &autonomous.PlanningBundle{
				Analysis: &autonomous.RequirementAnalysis{
					AppType: "fullstack",
					TechStack: &autonomous.TechStack{
						Frontend: "React",
						Backend:  "Node",
						Database: "PostgreSQL",
						Styling:  "Tailwind",
					},
				},
				Plan: &autonomous.ExecutionPlan{
					ID:            "plan-paid-frontstage-" + string(mode),
					EstimatedTime: 25 * time.Minute,
					CreatedAt:     time.Now().UTC(),
				},
			})

			plan = applyBuildAssurancePolicyToPlan(build, plan)
			if plan == nil {
				t.Fatal("expected plan")
			}
			if plan.DeliveryMode != "full_stack_preview" {
				t.Fatalf("expected paid full-stack build to continue through backend runtime by default, got %q", plan.DeliveryMode)
			}
			if plan.TechStack.Backend == "" || plan.TechStack.Database == "" {
				t.Fatalf("expected paid build to retain backend/database contract, got %+v", plan.TechStack)
			}
			if plan.APIContract == nil {
				t.Fatalf("expected API contract to remain available for full-stack execution")
			}
			if wo := getBuildWorkOrder(plan, RoleBackend); wo == nil {
				t.Fatalf("expected backend work order to run by default, got %+v", plan.WorkOrders)
			}
			if wo := getBuildWorkOrder(plan, RoleDatabase); wo == nil {
				t.Fatalf("expected database work order to run by default, got %+v", plan.WorkOrders)
			}
			if wo := getBuildWorkOrder(plan, RoleFrontend); wo == nil {
				t.Fatalf("expected frontend work order to remain active, got %+v", plan.WorkOrders)
			}
		})
	}
}

func TestApplyBuildAssurancePolicyToPlanCanStagePaidFullStackBehindFrontendApproval(t *testing.T) {
	t.Setenv("APEX_FRONTEND_APPROVAL_CHECKPOINT", "true")

	build := &Build{
		ID:               "paid-build-frontstage-opt-in",
		UserID:           22,
		SubscriptionPlan: "builder",
		Description:      "Build a full-stack CRM with auth, database-backed clients, projects, and reporting dashboards",
		PowerMode:        PowerBalanced,
	}

	plan := createBuildPlanFromPlanningBundle("build-paid-frontstage-opt-in", build.Description, nil, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "fullstack",
			TechStack: &autonomous.TechStack{
				Frontend: "React",
				Backend:  "Node",
				Database: "PostgreSQL",
				Styling:  "Tailwind",
			},
		},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-paid-frontstage-opt-in",
			EstimatedTime: 25 * time.Minute,
			CreatedAt:     time.Now().UTC(),
		},
	})

	plan = applyBuildAssurancePolicyToPlan(build, plan)
	if plan == nil {
		t.Fatal("expected plan")
	}
	if plan.DeliveryMode != "frontend_preview_only" {
		t.Fatalf("expected opt-in frontend approval checkpoint, got %q", plan.DeliveryMode)
	}
	if wo := getBuildWorkOrder(plan, RoleBackend); wo != nil {
		t.Fatalf("expected backend work order to be deferred until frontend approval, got %+v", wo)
	}
	if wo := getBuildWorkOrder(plan, RoleFrontend); wo == nil {
		t.Fatalf("expected frontend work order to remain active, got %+v", plan.WorkOrders)
	}
}

func TestCreateBuildPlanFromPlanningBundlePrefersStaticIntentOverPlannerFullstackAppType(t *testing.T) {
	t.Parallel()

	description := "Build a polished static marketing site for an AI operations studio with a hero section, services grid, testimonials, FAQ, and pricing. Frontend only. No backend. No database. No auth. No billing. No realtime."
	plan := createBuildPlanFromPlanningBundle("build-static-override", description, nil, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "fullstack",
			TechStack: &autonomous.TechStack{
				Frontend: "React",
				Backend:  "none",
				Database: "none",
				Styling:  "Tailwind",
			},
		},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-static-override",
			EstimatedTime: 20 * time.Minute,
			CreatedAt:     time.Now().UTC(),
		},
	})

	if plan == nil {
		t.Fatal("expected build plan")
	}
	if plan.AppType != "web" {
		t.Fatalf("expected explicit static intent to override planner fullstack app type, got %q", plan.AppType)
	}
	if plan.ScaffoldID != "frontend/react-vite-spa" {
		t.Fatalf("expected frontend scaffold, got %q", plan.ScaffoldID)
	}
	if wo := getBuildWorkOrder(plan, RoleDatabase); wo != nil {
		t.Fatalf("expected static override plan to omit database work order, got %+v", wo)
	}
}

func TestCreateBuildPlanFromPlanningBundlePrefersInMemoryPreviewIntentOverFullStackWording(t *testing.T) {
	t.Parallel()

	description := "Build a complete production-ready full-stack SaaS web app called Apex FieldOps AI using React, TypeScript, Tailwind, and shadcn/ui. All data stored in memory. No database, no external APIs, no real API keys needed. Include 7 jobs, job pipeline, estimate builder, crew management, settings, and simulated AI panels."
	plan := createBuildPlanFromPlanningBundle("build-in-memory-preview", description, nil, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "fullstack",
			TechStack: &autonomous.TechStack{
				Frontend: "React",
				Backend:  "Node.js",
				Database: "PostgreSQL",
				Styling:  "Tailwind",
			},
		},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-in-memory-preview",
			EstimatedTime: 20 * time.Minute,
			CreatedAt:     time.Now().UTC(),
		},
	})

	if plan == nil {
		t.Fatal("expected build plan")
	}
	if plan.AppType != "web" {
		t.Fatalf("expected explicit in-memory preview intent to override planner fullstack app type, got %q", plan.AppType)
	}
	if plan.TechStack.Backend != "" || plan.TechStack.Database != "" {
		t.Fatalf("expected backend/database to be stripped for in-memory preview, got %+v", plan.TechStack)
	}
	if plan.APIContract != nil || len(plan.APIEndpoints) != 0 {
		t.Fatalf("expected no API contract/endpoints for in-memory preview, got api=%+v endpoints=%+v", plan.APIContract, plan.APIEndpoints)
	}
	if plan.ScaffoldID != "frontend/react-vite-spa" {
		t.Fatalf("expected frontend scaffold, got %q", plan.ScaffoldID)
	}
	if plan.TemplateID != "" {
		t.Fatalf("expected runtime-free preview prompt not to inherit runtime template %q", plan.TemplateID)
	}
	for _, check := range plan.Acceptance {
		if strings.HasPrefix(check.ID, "ai-saas-") || strings.HasPrefix(check.ID, "dashboard-") {
			t.Fatalf("did not expect runtime template acceptance checks for in-memory FieldOps build, got %+v", check)
		}
	}
	if wo := getBuildWorkOrder(plan, RoleBackend); wo != nil {
		t.Fatalf("expected in-memory preview plan to omit backend work order, got %+v", wo)
	}
}

func TestCreateBuildPlanFromPlanningBundleAppliesPrimaryAndSecondaryTemplates(t *testing.T) {
	t.Parallel()

	description := `Build an AI SaaS prompt optimizer with BYOK, provider routing, token usage tracking,
generation history, and a model selector. Also include a public landing page and marketing site with
waitlist signup, lead capture, pricing, testimonials, and a demo request CTA.`

	plan := createBuildPlanFromPlanningBundle("build-layered-templates", description, &TechStack{
		Frontend: "React",
		Backend:  "Node.js",
		Database: "PostgreSQL",
		Styling:  "Tailwind",
	}, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "web",
			TechStack: &autonomous.TechStack{
				Frontend: "React",
				Backend:  "Node.js",
				Database: "PostgreSQL",
				Styling:  "Tailwind",
			},
		},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-layered-templates",
			EstimatedTime: 45 * time.Minute,
			CreatedAt:     time.Now().UTC(),
		},
	})

	if plan == nil {
		t.Fatal("expected build plan")
	}
	if plan.TemplateID != "ai-saas" {
		t.Fatalf("expected primary ai-saas template, got %q from detected templates %+v", plan.TemplateID, templateIDs(DetectAppTemplates(description, 0)))
	}
	if !slices.Contains(plan.SecondaryTemplateIDs, "landing-page") {
		t.Fatalf("expected landing-page secondary template, got %+v", plan.SecondaryTemplateIDs)
	}
	if plan.AppType != "fullstack" {
		t.Fatalf("expected runtime template to promote app type to fullstack, got %q", plan.AppType)
	}
	if !hasAcceptanceCheckPrefix(plan.Acceptance, "ai-saas-auth") {
		t.Fatalf("expected AI SaaS acceptance checks, got %+v", plan.Acceptance)
	}
	if !hasAcceptanceCheckPrefix(plan.Acceptance, "landing-conversion-goal") {
		t.Fatalf("expected landing-page acceptance checks, got %+v", plan.Acceptance)
	}
}

func TestCreateBuildPlanFromPlanningBundleStaticIntentOverridesRequestedBackendAndDatabase(t *testing.T) {
	t.Parallel()

	description := "Build a polished static marketing site for an AI operations studio using Next.js. Frontend only. No backend. No database. No auth. No billing. No realtime."
	plan := createBuildPlanFromPlanningBundle("build-static-requested-stack", description, &TechStack{
		Frontend: "Next.js",
		Backend:  "Node.js",
		Database: "PostgreSQL",
		Styling:  "Tailwind",
	}, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "fullstack",
			TechStack: &autonomous.TechStack{
				Frontend: "Next.js",
				Backend:  "Node.js",
				Database: "PostgreSQL",
				Styling:  "Tailwind",
			},
		},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-static-requested-stack",
			EstimatedTime: 20 * time.Minute,
			CreatedAt:     time.Now().UTC(),
		},
	})

	if plan == nil {
		t.Fatal("expected build plan")
	}
	if plan.AppType != "web" {
		t.Fatalf("expected static intent to keep app type web, got %q", plan.AppType)
	}
	if plan.TechStack.Frontend != "Next.js" {
		t.Fatalf("expected frontend selection to remain Next.js, got %+v", plan.TechStack)
	}
	if plan.TechStack.Backend != "" || plan.TechStack.Database != "" {
		t.Fatalf("expected static intent to strip requested backend/database, got %+v", plan.TechStack)
	}
	if plan.ScaffoldID != "frontend/nextjs-app" {
		t.Fatalf("expected frontend nextjs scaffold, got %q", plan.ScaffoldID)
	}
	if wo := getBuildWorkOrder(plan, RoleBackend); wo != nil {
		t.Fatalf("expected static requested-stack plan to omit backend work order, got %+v", wo)
	}
	if wo := getBuildWorkOrder(plan, RoleDatabase); wo != nil {
		t.Fatalf("expected static requested-stack plan to omit database work order, got %+v", wo)
	}
}

func TestSelectBuildScaffoldNextjsWebOmitsBackendAndDatabaseRoles(t *testing.T) {
	t.Parallel()

	scaffold := selectBuildScaffold("web", TechStack{
		Frontend: "Next.js",
		Backend:  "",
		Database: "",
		Styling:  "Tailwind",
	})

	if scaffold.ID != "frontend/nextjs-app" {
		t.Fatalf("expected frontend nextjs scaffold, got %q", scaffold.ID)
	}
	if _, exists := scaffold.Ownership[RoleBackend]; exists {
		t.Fatalf("expected web-only nextjs scaffold to omit backend ownership, got %+v", scaffold.Ownership[RoleBackend])
	}
	if _, exists := scaffold.Ownership[RoleDatabase]; exists {
		t.Fatalf("expected web-only nextjs scaffold to omit database ownership, got %+v", scaffold.Ownership[RoleDatabase])
	}
	if len(scaffold.EnvVars) != 0 {
		t.Fatalf("expected web-only nextjs scaffold to omit backend env vars, got %+v", scaffold.EnvVars)
	}
}

func TestCreateBuildPlanFromPlanningBundleSkipsDatabaseLaneForStaticIntent(t *testing.T) {
	t.Parallel()

	description := "Build a polished static marketing site for an AI operations studio with a hero section, services grid, testimonials, FAQ, and pricing. Frontend only. No backend. No database. No auth. No billing. No realtime."
	plan := createBuildPlanFromPlanningBundle("build-static-3", description, nil, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "web",
			TechStack: &autonomous.TechStack{
				Frontend: "React",
				Styling:  "Tailwind",
			},
		},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-static-3",
			EstimatedTime: 20 * time.Minute,
			CreatedAt:     time.Now().UTC(),
			Steps: []*autonomous.PlanStep{
				{
					ID:         "step-1",
					Name:       "Create Data Models",
					ActionType: autonomous.ActionAIGenerate,
					Input:      map[string]interface{}{"type": "data_models"},
				},
			},
		},
	})

	if plan == nil {
		t.Fatal("expected build plan")
	}
	for _, file := range plan.Files {
		if file.Path == "migrations/001_initial.sql" {
			t.Fatalf("expected static plan to skip migration output, got %+v", plan.Files)
		}
	}
	if wo := getBuildWorkOrder(plan, RoleDatabase); wo != nil {
		t.Fatalf("expected static plan to omit database work order, got %+v", wo)
	}
	if wo := getBuildWorkOrder(plan, RoleBackend); wo != nil {
		t.Fatalf("expected static plan to omit backend work order, got %+v", wo)
	}
	if wo := getBuildWorkOrder(plan, RoleTesting); wo != nil {
		t.Fatalf("expected static plan to omit dedicated testing work order, got %+v", wo)
	}
}

func TestCreateBuildPlanFromPlanningBundleStaticNextjsOmitsDedicatedTestingRole(t *testing.T) {
	t.Parallel()

	description := "Build a polished static marketing site for an AI operations studio using Next.js. Frontend only. No backend. No database. No auth. No billing. No realtime."
	plan := createBuildPlanFromPlanningBundle("build-static-nextjs-testing", description, &TechStack{
		Frontend: "Next.js",
		Styling:  "Tailwind",
	}, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "web",
			TechStack: &autonomous.TechStack{
				Frontend: "Next.js",
				Styling:  "Tailwind",
			},
		},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-static-nextjs-testing",
			EstimatedTime: 20 * time.Minute,
			CreatedAt:     time.Now().UTC(),
		},
	})

	if plan == nil {
		t.Fatal("expected build plan")
	}
	if wo := getBuildWorkOrder(plan, RoleTesting); wo != nil {
		t.Fatalf("expected static nextjs plan to omit dedicated testing work order, got %+v", wo)
	}
}

func TestAssignPhaseAgentsUsesFrozenWorkOrder(t *testing.T) {
	t.Parallel()

	plan := createBuildPlanFromPlanningBundle("build-1", "Build TranscriptVault", nil, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "fullstack",
			TechStack: &autonomous.TechStack{
				Frontend: "React",
				Backend:  "Node",
				Database: "PostgreSQL",
				Styling:  "Tailwind",
			},
		},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-1",
			EstimatedTime: time.Hour,
			CreatedAt:     time.Now(),
		},
	})

	build := &Build{
		ID:           "build-1",
		Description:  "Build TranscriptVault",
		Status:       BuildInProgress,
		MaxRetries:   2,
		Plan:         plan,
		ProviderMode: "platform",
		Tasks:        []*Task{},
		Agents:       map[string]*Agent{},
	}
	intent := &IntentBrief{AppType: plan.AppType}
	contract := compileBuildContractFromPlan(build.ID, intent, plan)
	build.SnapshotState.Orchestration = &BuildOrchestrationState{
		Flags:              defaultBuildOrchestrationFlags(),
		BuildContract:      contract,
		WorkOrders:         compileWorkOrdersFromPlan(build.ID, contract, plan, defaultProviderScorecards(build.ProviderMode)),
		ProviderScorecards: defaultProviderScorecards(build.ProviderMode),
	}
	agent := &Agent{ID: "front-1", BuildID: build.ID, Role: RoleFrontend}
	build.Agents[agent.ID] = agent

	am := &AgentManager{
		agents:      map[string]*Agent{agent.ID: agent},
		builds:      map[string]*Build{build.ID: build},
		taskQueue:   make(chan *Task, 1),
		subscribers: map[string][]chan *WSMessage{},
		ctx:         context.Background(),
	}

	taskIDs := am.assignPhaseAgents(build, []agentPriority{{agent: agent, priority: 60}}, build.Description)
	if len(taskIDs) != 1 {
		t.Fatalf("expected one task id, got %d", len(taskIDs))
	}
	if len(build.Tasks) != 1 {
		t.Fatalf("expected one task, got %d", len(build.Tasks))
	}

	task := build.Tasks[0]
	if task.Description == "" || task.Description == build.Description {
		t.Fatalf("expected work-order-specific description, got %q", task.Description)
	}
	if got, _ := task.Input["build_spec_hash"].(string); got != plan.SpecHash {
		t.Fatalf("expected spec hash %s, got %s", plan.SpecHash, got)
	}
	if requireCheckins, _ := task.Input["require_checkins"].(bool); !requireCheckins {
		t.Fatal("expected require_checkins=true")
	}
	if workOrder := taskWorkOrderFromInput(task); workOrder == nil || workOrder.Role != RoleFrontend {
		t.Fatalf("expected frontend work order, got %+v", workOrder)
	}
	if artifact := taskArtifactWorkOrderFromInput(task); artifact == nil || artifact.Role != RoleFrontend {
		t.Fatalf("expected frontend work order artifact, got %+v", artifact)
	} else if artifact.MaxContextBudget == 0 || artifact.Summary == "" {
		t.Fatalf("expected hydrated artifact metadata, got %+v", artifact)
	}
	if readable, ok := task.Input["readable_files"].([]string); !ok || len(readable) == 0 {
		t.Fatalf("expected readable_files to be hydrated from artifact, got %+v", task.Input["readable_files"])
	}
}

func TestApplyReliabilityWorkOrderBiasAddsFrontendAndRecurringChecks(t *testing.T) {
	t.Parallel()

	plan := &BuildPlan{
		WorkOrders: []BuildWorkOrder{
			{Role: RoleFrontend, AcceptanceChecks: []string{"Render the main dashboard route"}},
			{Role: RoleTesting, AcceptanceChecks: []string{"Verify preview boot"}},
			{Role: RoleBackend, AcceptanceChecks: []string{"Serve the API contract"}},
		},
	}
	spec := &ValidatedBuildSpec{
		DeliveryMode:       "full_stack_preview",
		AcceptanceSurfaces: []string{"frontend", "backend"},
		PrimaryUserFlows: []string{
			"land in the product shell and reach an interactive preview on first pass",
			"open the dashboard and inspect KPI cards",
		},
	}
	summary := &BuildReliabilitySummary{
		Status:                "degraded",
		CurrentFailureClass:   "compile_failure",
		AdvisoryClasses:       []string{"visual_layout", "interaction_canary"},
		RecurringFailureClass: []string{"compile_failure", "visual_layout"},
		RecommendedFocus:      []string{"expand deterministic compile repair coverage for the current failure class"},
	}

	applyReliabilityWorkOrderBias(plan, spec, summary)

	frontend := getBuildWorkOrder(plan, RoleFrontend)
	if frontend == nil {
		t.Fatal("expected frontend work order")
	}
	if !containsString(frontend.AcceptanceChecks, "Deliver a preview-visible frontend shell first and keep the accepted frontend surfaces truthful before backend/runtime follow-up.") {
		t.Fatalf("expected frontend preview-first check, got %+v", frontend.AcceptanceChecks)
	}
	if !containsString(frontend.AcceptanceChecks, "Guard against compile regressions in owned files: keep imports, exports, and types runnable without unresolved symbols.") {
		t.Fatalf("expected frontend compile guard check, got %+v", frontend.AcceptanceChecks)
	}
	if !containsString(frontend.AcceptanceChecks, "Check preview-critical screens for blank states, contrast issues, layout overlap, and unstyled surfaces before completion.") {
		t.Fatalf("expected frontend visual reliability check, got %+v", frontend.AcceptanceChecks)
	}
	if !containsString(frontend.AcceptanceChecks, "Verify first-click interactions on primary CTAs, buttons, links, and menus so preview canary checks stay clean.") {
		t.Fatalf("expected frontend interaction reliability check, got %+v", frontend.AcceptanceChecks)
	}

	testing := getBuildWorkOrder(plan, RoleTesting)
	if testing == nil {
		t.Fatal("expected testing work order")
	}
	if !containsString(testing.AcceptanceChecks, "Reliability focus: expand deterministic compile repair coverage for the current failure class") {
		t.Fatalf("expected testing reliability focus, got %+v", testing.AcceptanceChecks)
	}

	backend := getBuildWorkOrder(plan, RoleBackend)
	if backend == nil {
		t.Fatal("expected backend work order")
	}
	if !containsString(backend.AcceptanceChecks, "Implement runtime work without regressing the already-approved frontend preview shell or route contract.") {
		t.Fatalf("expected backend preview-preservation check, got %+v", backend.AcceptanceChecks)
	}
}

func TestAssignPhaseAgentsIncludesRoleScopedValidatedAdvisories(t *testing.T) {
	t.Parallel()

	plan := createBuildPlanFromPlanningBundle("build-validated-advisories", "Build a multi-tenant analytics dashboard", nil, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "fullstack",
			TechStack: &autonomous.TechStack{
				Frontend: "React",
				Backend:  "Node",
				Database: "PostgreSQL",
				Styling:  "Tailwind",
			},
		},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-validated-advisories",
			EstimatedTime: time.Hour,
			CreatedAt:     time.Now(),
		},
	})

	build := &Build{
		ID:           "build-validated-advisories",
		Description:  "Build a multi-tenant analytics dashboard",
		Status:       BuildPlanning,
		Plan:         plan,
		ProviderMode: "platform",
		Tasks:        []*Task{},
		Agents:       map[string]*Agent{},
	}
	intent := &IntentBrief{AppType: plan.AppType}
	contract := compileBuildContractFromPlan(build.ID, intent, plan)
	build.SnapshotState.Orchestration = &BuildOrchestrationState{
		Flags:              defaultBuildOrchestrationFlags(),
		BuildContract:      contract,
		WorkOrders:         compileWorkOrdersFromPlan(build.ID, contract, plan, defaultProviderScorecards(build.ProviderMode)),
		ProviderScorecards: defaultProviderScorecards(build.ProviderMode),
		ValidatedBuildSpec: &ValidatedBuildSpec{
			PerformanceAdvisories: []BuildSpecAdvisory{
				{
					Code:    "progressive_dashboard_loading",
					Surface: SurfaceFrontend,
					Summary: "Dashboard-style apps should reveal value before every widget finishes loading.",
				},
			},
			SecurityAdvisories: []BuildSpecAdvisory{
				{
					Code:    "tenant_isolation",
					Surface: SurfaceBackend,
					Summary: "Multi-tenant data models need explicit tenant isolation at query and mutation boundaries.",
				},
			},
		},
	}

	frontendAgent := &Agent{ID: "front-validated", BuildID: build.ID, Role: RoleFrontend}
	backendAgent := &Agent{ID: "back-validated", BuildID: build.ID, Role: RoleBackend}
	build.Agents[frontendAgent.ID] = frontendAgent
	build.Agents[backendAgent.ID] = backendAgent

	am := &AgentManager{
		agents:      map[string]*Agent{frontendAgent.ID: frontendAgent, backendAgent.ID: backendAgent},
		builds:      map[string]*Build{build.ID: build},
		taskQueue:   make(chan *Task, 2),
		subscribers: map[string][]chan *WSMessage{},
		ctx:         context.Background(),
	}

	taskIDs := am.assignPhaseAgents(build, []agentPriority{
		{agent: frontendAgent, priority: 60},
		{agent: backendAgent, priority: 60},
	}, build.Description)
	if len(taskIDs) != 2 {
		t.Fatalf("expected two task ids, got %d", len(taskIDs))
	}
	if len(build.Tasks) != 2 {
		t.Fatalf("expected two tasks, got %d", len(build.Tasks))
	}

	var frontendTask, backendTask *Task
	for _, task := range build.Tasks {
		if role, _ := task.Input["agent_role"].(string); role == string(RoleFrontend) {
			frontendTask = task
		}
		if role, _ := task.Input["agent_role"].(string); role == string(RoleBackend) {
			backendTask = task
		}
	}
	if frontendTask == nil || backendTask == nil {
		t.Fatalf("expected both frontend and backend tasks, got %+v", build.Tasks)
	}

	frontendPerformance, ok := frontendTask.Input["validated_performance_advisories"].([]BuildSpecAdvisory)
	if !ok || len(frontendPerformance) != 1 || frontendPerformance[0].Code != "progressive_dashboard_loading" {
		t.Fatalf("expected scoped frontend performance advisory, got %+v", frontendTask.Input["validated_performance_advisories"])
	}
	if _, exists := frontendTask.Input["validated_security_advisories"]; exists {
		t.Fatalf("did not expect backend security advisories on frontend task, got %+v", frontendTask.Input["validated_security_advisories"])
	}

	backendSecurity, ok := backendTask.Input["validated_security_advisories"].([]BuildSpecAdvisory)
	if !ok || len(backendSecurity) != 1 || backendSecurity[0].Code != "tenant_isolation" {
		t.Fatalf("expected scoped backend security advisory, got %+v", backendTask.Input["validated_security_advisories"])
	}
	if _, exists := backendTask.Input["validated_performance_advisories"]; exists {
		t.Fatalf("did not expect frontend performance advisories on backend task, got %+v", backendTask.Input["validated_performance_advisories"])
	}
}

func TestApplyReliabilityWorkOrderBiasIncludesValidatedSpecAdvisories(t *testing.T) {
	t.Parallel()

	plan := &BuildPlan{
		WorkOrders: []BuildWorkOrder{
			{Role: RoleFrontend, AcceptanceChecks: []string{"Render the main dashboard route"}},
			{Role: RoleBackend, AcceptanceChecks: []string{"Serve the API contract"}},
			{Role: RoleReviewer, AcceptanceChecks: []string{"Review for production readiness"}},
		},
	}
	spec := &ValidatedBuildSpec{
		DeliveryMode:       "full_stack_preview",
		AcceptanceSurfaces: []string{"frontend", "backend", "integration"},
		SecurityAdvisories: []BuildSpecAdvisory{
			{
				Code:           "tenant_isolation",
				Surface:        SurfaceBackend,
				Summary:        "Multi-tenant data models need explicit tenant isolation at query and mutation boundaries.",
				Recommendation: "Freeze tenant/workspace ownership fields in the schema and require every backend read/write path to scope by tenant.",
			},
			{
				Code:           "billing_webhook_verification",
				Surface:        SurfaceIntegration,
				Summary:        "Payment-capable apps need webhook verification and idempotent entitlement updates.",
				Recommendation: "Specify the webhook source of truth up front and never grant access from client redirects alone.",
			},
		},
		PerformanceAdvisories: []BuildSpecAdvisory{
			{
				Code:           "progressive_dashboard_loading",
				Surface:        SurfaceFrontend,
				Summary:        "Dashboard-style apps should reveal value before every widget finishes loading.",
				Recommendation: "Prioritize hero KPIs, stagger secondary widgets, and avoid blocking first paint on full analytics hydration.",
			},
		},
	}

	applyReliabilityWorkOrderBias(plan, spec, nil)

	frontend := getBuildWorkOrder(plan, RoleFrontend)
	if frontend == nil {
		t.Fatal("expected frontend work order")
	}
	if !containsString(frontend.AcceptanceChecks, "Honor validated performance advisory (progressive_dashboard_loading): Dashboard-style apps should reveal value before every widget finishes loading. Guardrail: Prioritize hero KPIs, stagger secondary widgets, and avoid blocking first paint on full analytics hydration.") {
		t.Fatalf("expected frontend performance advisory check, got %+v", frontend.AcceptanceChecks)
	}
	if containsString(frontend.AcceptanceChecks, "Honor validated security advisory (tenant_isolation): Multi-tenant data models need explicit tenant isolation at query and mutation boundaries. Guardrail: Freeze tenant/workspace ownership fields in the schema and require every backend read/write path to scope by tenant.") {
		t.Fatalf("did not expect backend security advisory on frontend work order, got %+v", frontend.AcceptanceChecks)
	}

	backend := getBuildWorkOrder(plan, RoleBackend)
	if backend == nil {
		t.Fatal("expected backend work order")
	}
	if !containsString(backend.AcceptanceChecks, "Honor validated security advisory (tenant_isolation): Multi-tenant data models need explicit tenant isolation at query and mutation boundaries. Guardrail: Freeze tenant/workspace ownership fields in the schema and require every backend read/write path to scope by tenant.") {
		t.Fatalf("expected backend security advisory check, got %+v", backend.AcceptanceChecks)
	}

	reviewer := getBuildWorkOrder(plan, RoleReviewer)
	if reviewer == nil {
		t.Fatal("expected reviewer work order")
	}
	if !containsString(reviewer.AcceptanceChecks, "Honor validated security advisory (billing_webhook_verification): Payment-capable apps need webhook verification and idempotent entitlement updates. Guardrail: Specify the webhook source of truth up front and never grant access from client redirects alone.") {
		t.Fatalf("expected reviewer integration security advisory check, got %+v", reviewer.AcceptanceChecks)
	}
	if !containsString(reviewer.AcceptanceChecks, "Honor validated performance advisory (progressive_dashboard_loading): Dashboard-style apps should reveal value before every widget finishes loading. Guardrail: Prioritize hero KPIs, stagger secondary widgets, and avoid blocking first paint on full analytics hydration.") {
		t.Fatalf("expected reviewer performance advisory check, got %+v", reviewer.AcceptanceChecks)
	}
}

func TestAssignTaskHydratesArtifactWorkOrderForAdHocTask(t *testing.T) {
	t.Parallel()

	plan := createBuildPlanFromPlanningBundle("build-1", "Build TranscriptVault", nil, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "fullstack",
			TechStack: &autonomous.TechStack{
				Frontend: "React",
				Backend:  "Node",
				Database: "PostgreSQL",
				Styling:  "Tailwind",
			},
		},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-1",
			EstimatedTime: time.Hour,
			CreatedAt:     time.Now(),
		},
	})

	build := &Build{
		ID:           "build-1",
		Description:  "Build TranscriptVault",
		Status:       BuildInProgress,
		Plan:         plan,
		ProviderMode: "platform",
		Tasks:        []*Task{},
		Agents:       map[string]*Agent{},
	}
	intent := &IntentBrief{AppType: plan.AppType}
	contract := compileBuildContractFromPlan(build.ID, intent, plan)
	build.SnapshotState.Orchestration = &BuildOrchestrationState{
		Flags:              defaultBuildOrchestrationFlags(),
		BuildContract:      contract,
		WorkOrders:         compileWorkOrdersFromPlan(build.ID, contract, plan, defaultProviderScorecards(build.ProviderMode)),
		ProviderScorecards: defaultProviderScorecards(build.ProviderMode),
	}
	agent := &Agent{ID: "review-1", BuildID: build.ID, Role: RoleReviewer}
	build.Agents[agent.ID] = agent

	am := &AgentManager{
		agents:      map[string]*Agent{agent.ID: agent},
		builds:      map[string]*Build{build.ID: build},
		taskQueue:   make(chan *Task, 1),
		subscribers: map[string][]chan *WSMessage{},
		ctx:         context.Background(),
	}

	task := &Task{
		ID:          "review-task-1",
		Type:        TaskReview,
		Description: "Review targeted patch",
		Status:      TaskPending,
		Input: map[string]any{
			"action": "post_fix_review",
		},
		CreatedAt: time.Now(),
	}

	if err := am.AssignTask(agent.ID, task); err != nil {
		t.Fatalf("AssignTask returned error: %v", err)
	}

	artifact := taskArtifactWorkOrderFromInput(task)
	if artifact == nil || artifact.Role != RoleReviewer {
		t.Fatalf("expected reviewer artifact work order, got %+v", artifact)
	}
	if task.Input["routing_mode"] == "" || task.Input["risk_level"] == "" {
		t.Fatalf("expected task contract metadata to be hydrated, got %+v", task.Input)
	}
	if task.Input["contract_slice"] == nil {
		t.Fatalf("expected contract_slice to be hydrated, got %+v", task.Input)
	}
}

func TestAssignTaskCapturesPatchBaselineWithinWorkOrderScope(t *testing.T) {
	t.Parallel()

	plan := createBuildPlanFromPlanningBundle("build-1", "Build TranscriptVault", nil, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "fullstack",
			TechStack: &autonomous.TechStack{
				Frontend: "React",
				Backend:  "Node",
				Database: "PostgreSQL",
				Styling:  "Tailwind",
			},
		},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-1",
			EstimatedTime: time.Hour,
			CreatedAt:     time.Now(),
		},
	})

	build := &Build{
		ID:           "build-1",
		Description:  "Build TranscriptVault",
		Status:       BuildInProgress,
		Plan:         plan,
		ProviderMode: "platform",
		Tasks: []*Task{
			{
				ID:     "seed-files",
				Type:   TaskGenerateFile,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{Path: "src/App.tsx", Content: "export default function App(){ return <div>seed</div> }\n", Language: "typescript"},
						{Path: "package.json", Content: "{\"name\":\"seed-app\"}\n", Language: "json"},
						{Path: "server/index.ts", Content: "export const server = true;\n", Language: "typescript"},
					},
				},
			},
		},
		Agents: map[string]*Agent{},
	}
	intent := &IntentBrief{AppType: plan.AppType}
	contract := compileBuildContractFromPlan(build.ID, intent, plan)
	build.SnapshotState.Orchestration = &BuildOrchestrationState{
		Flags:              defaultBuildOrchestrationFlags(),
		BuildContract:      contract,
		WorkOrders:         compileWorkOrdersFromPlan(build.ID, contract, plan, defaultProviderScorecards(build.ProviderMode)),
		ProviderScorecards: defaultProviderScorecards(build.ProviderMode),
	}
	agent := &Agent{ID: "front-1", BuildID: build.ID, Role: RoleFrontend}
	build.Agents[agent.ID] = agent

	am := &AgentManager{
		agents:      map[string]*Agent{agent.ID: agent},
		builds:      map[string]*Build{build.ID: build},
		taskQueue:   make(chan *Task, 1),
		subscribers: map[string][]chan *WSMessage{},
		ctx:         context.Background(),
	}

	task := &Task{
		ID:          "frontend-task-1",
		Type:        TaskGenerateUI,
		Description: "Update frontend shell",
		Status:      TaskPending,
		Input:       map[string]any{},
		CreatedAt:   time.Now(),
	}

	if err := am.AssignTask(agent.ID, task); err != nil {
		t.Fatalf("AssignTask returned error: %v", err)
	}

	baseline := taskPatchBaselineFromInput(task)
	if len(baseline) == 0 {
		t.Fatalf("expected patch baseline files, got %+v", task.Input["patch_baseline_files"])
	}
	paths := make([]string, 0, len(baseline))
	for _, file := range baseline {
		paths = append(paths, file.Path)
	}
	joined := strings.Join(paths, ",")
	if !strings.Contains(joined, "src/App.tsx") {
		t.Fatalf("expected frontend baseline to include src/App.tsx, got %s", joined)
	}
	if !strings.Contains(joined, "package.json") {
		t.Fatalf("expected frontend baseline to include package.json, got %s", joined)
	}
	if strings.Contains(joined, "server/index.ts") {
		t.Fatalf("expected frontend baseline to exclude backend-owned files, got %s", joined)
	}
}

func TestParseTaskOutputExtractsCoordinationBlocks(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	resp := "<task_start_ack>{\"summary\":\"working frontend shell\",\"owned_files\":[\"src/**\"],\"dependencies\":[\"api contract\"],\"acceptance_checks\":[\"render app\"],\"blockers\":[]}</task_start_ack>\n" +
		"// File: src/App.tsx\n" +
		"```typescript\n" +
		"export default function App() { return <div>ok</div>; }\n" +
		"```\n" +
		"<task_completion_report>{\"summary\":\"frontend shell completed\",\"created_files\":[\"src/App.tsx\"],\"modified_files\":[],\"completed_checks\":[\"render app\"],\"remaining_risks\":[],\"blockers\":[]}</task_completion_report>"

	out := am.parseTaskOutput(TaskGenerateUI, resp)
	if out.StartAck == nil || out.StartAck.Summary != "working frontend shell" {
		t.Fatalf("expected parsed start ack, got %+v", out.StartAck)
	}
	if out.Completion == nil || out.Completion.Summary != "frontend shell completed" {
		t.Fatalf("expected parsed completion report, got %+v", out.Completion)
	}
	if len(out.Files) != 1 || out.Files[0].Path != "src/App.tsx" {
		t.Fatalf("expected parsed file, got %+v", out.Files)
	}
}

func TestExtractTaskCheckinsHandlesBareTagsWithoutClosingTags(t *testing.T) {
	t.Parallel()

	resp := "<task_start_ack>{\"summary\":\"working frontend shell\",\"owned_files\":[\"src/**\"],\"dependencies\":[\"api contract\"],\"acceptance_checks\":[\"render app\"],\"blockers\":[]}\n" +
		"```json\n" +
		"{\"patch_bundle\":{\"operations\":[{\"type\":\"replace_function\",\"path\":\"src/App.tsx\",\"content\":\"export default function App(){ return <div>ok</div>; }\"}]}}\n" +
		"```\n" +
		"<task_completion_report>{\"summary\":\"frontend shell completed\",\"created_files\":[],\"modified_files\":[\"src/App.tsx\"],\"completed_checks\":[\"render app\"],\"remaining_risks\":[],\"blockers\":[]}"

	clean, startAck, completion := extractTaskCheckins(resp)
	if startAck == nil || startAck.Summary != "working frontend shell" {
		t.Fatalf("expected parsed start ack, got %+v", startAck)
	}
	if completion == nil || completion.Summary != "frontend shell completed" {
		t.Fatalf("expected parsed completion report, got %+v", completion)
	}
	if strings.Contains(clean, "<task_start_ack>") || strings.Contains(clean, "<task_completion_report>") {
		t.Fatalf("expected checkin tags to be removed, got %q", clean)
	}
	if !strings.Contains(clean, "\"patch_bundle\"") {
		t.Fatalf("expected patch bundle body to remain after stripping checkins, got %q", clean)
	}
}

func TestParseTaskOutputExtractsStructuredPatchBundleWithBareCheckins(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	resp := "<task_start_ack>{\"summary\":\"repairing frontend shell\",\"owned_files\":[\"src/App.tsx\",\"src/index.css\"],\"dependencies\":[\"design system\"],\"acceptance_checks\":[\"replace scaffold\"],\"blockers\":[]}\n" +
		"```json\n" +
		"{\n" +
		"  \"patch_bundle\": {\n" +
		"    \"operations\": [\n" +
		"      {\"type\":\"replace_function\",\"path\":\"src/App.tsx\",\"content\":\"// File: src/App.tsx\\nexport default function App(){ return <div>real app</div>; }\"},\n" +
		"      {\"type\":\"replace_function\",\"path\":\"src/index.css\",\"content\":\"// File: src/index.css\\nbody { margin: 0; }\"}\n" +
		"    ]\n" +
		"  }\n" +
		"}\n" +
		"```\n" +
		"<task_completion_report>{\"summary\":\"frontend repair completed\",\"created_files\":[],\"modified_files\":[\"src/App.tsx\",\"src/index.css\"],\"completed_checks\":[\"replace scaffold\"],\"remaining_risks\":[],\"blockers\":[]}"

	out := am.parseTaskOutput(TaskGenerateUI, resp)
	if out.StartAck == nil || out.StartAck.Summary != "repairing frontend shell" {
		t.Fatalf("expected parsed start ack, got %+v", out.StartAck)
	}
	if out.Completion == nil || out.Completion.Summary != "frontend repair completed" {
		t.Fatalf("expected parsed completion report, got %+v", out.Completion)
	}
	if out.StructuredPatchBundle == nil {
		t.Fatalf("expected structured patch bundle, got %+v", out)
	}
	if len(out.StructuredPatchBundle.Operations) != 2 {
		t.Fatalf("expected 2 patch operations, got %+v", out.StructuredPatchBundle.Operations)
	}
	if len(out.Files) != 0 {
		t.Fatalf("expected parseTaskOutput to avoid generated artifact fallback, got %+v", out.Files)
	}
}

func TestBuildTaskPromptUsesPatchFirstFormatForRepairTasks(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	prompt := am.buildTaskPrompt(
		&Task{
			Type:        TaskFix,
			Description: "Repair frontend build failure",
			Input: map[string]any{
				"work_order_artifact": WorkOrder{
					ID:          "wo-repair-prompt",
					Role:        RoleSolver,
					Category:    WorkOrderRepair,
					TaskShape:   TaskShapeRepair,
					RoutingMode: RoutingModeDiagnosisRepair,
				},
			},
		},
		&Build{
			Description: "Fix the failing preview build",
			PowerMode:   PowerBalanced,
		},
		&Agent{Role: RoleSolver},
	)

	if !strings.Contains(prompt, "PATCH-FIRST OUTPUT FORMAT - CRITICAL") {
		t.Fatalf("expected repair prompt to advertise patch-first output, got %q", prompt)
	}
	if !strings.Contains(prompt, "\"patch_bundle\"") {
		t.Fatalf("expected repair prompt to include patch bundle schema, got %q", prompt)
	}
}

func TestValidateTaskCoordinationOutputRejectsOutOfScopeFiles(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	task := &Task{
		ID: "task-1",
		Input: map[string]any{
			"require_checkins": true,
			"work_order": &BuildWorkOrder{
				Role:           RoleFrontend,
				OwnedFiles:     []string{"src/**"},
				ForbiddenFiles: []string{"server/**"},
			},
		},
	}
	output := &TaskOutput{
		StartAck:   &TaskStartAck{Summary: "working", OwnedFiles: []string{"src/**"}},
		Completion: &TaskCompletionReport{Summary: "done", CreatedFiles: []string{"server/index.ts"}},
		Files: []GeneratedFile{
			{Path: "server/index.ts", Content: "export {}", Language: "typescript"},
		},
	}

	errs := am.validateTaskCoordinationOutput(task, output)
	if len(errs) == 0 {
		t.Fatal("expected work-order validation errors")
	}
	if !strings.Contains(strings.Join(errs, "\n"), "outside work order ownership") {
		t.Fatalf("unexpected coordination errors: %v", errs)
	}
}

func TestPathAllowedByWorkOrderSpecificOwnedPathOverridesBroadForbiddenPattern(t *testing.T) {
	t.Parallel()

	workOrder := &BuildWorkOrder{
		Role: RoleDatabase,
		OwnedFiles: []string{
			"server/db/**",
			"server/migrate.ts",
			"server/seed.ts",
		},
		ForbiddenFiles: []string{
			"server/**",
			"src/**",
		},
	}

	for _, path := range []string{"server/migrate.ts", "server/seed.ts", "server/db/index.ts"} {
		if !pathAllowedByWorkOrder(path, workOrder) {
			t.Fatalf("expected %s to be allowed by specific database ownership override", path)
		}
	}
	if pathAllowedByWorkOrder("server/routes/api.ts", workOrder) {
		t.Fatal("did not expect unrelated server path to bypass broad forbidden pattern")
	}
}

func TestBootstrapBuildScaffoldCreatesSyntheticTask(t *testing.T) {
	t.Parallel()

	plan := createBuildPlanFromPlanningBundle("build-1", "Build TranscriptVault", nil, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "fullstack",
			TechStack: &autonomous.TechStack{
				Frontend: "React",
				Backend:  "Node",
				Database: "PostgreSQL",
				Styling:  "Tailwind",
			},
		},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-1",
			EstimatedTime: time.Hour,
			CreatedAt:     time.Now(),
		},
	})

	build := &Build{
		ID:          "build-1",
		Description: "Build TranscriptVault",
		Status:      BuildPlanning,
		Plan:        plan,
		Tasks:       []*Task{},
		Agents:      map[string]*Agent{},
	}
	am := &AgentManager{
		builds:      map[string]*Build{build.ID: build},
		taskQueue:   make(chan *Task, 1),
		resultQueue: make(chan *TaskResult, 1),
		subscribers: map[string][]chan *WSMessage{},
		ctx:         context.Background(),
	}

	count := am.bootstrapBuildScaffold(build)
	if count == 0 {
		t.Fatal("expected scaffold bootstrap files to be created")
	}
	if len(build.Tasks) != 1 {
		t.Fatalf("expected one synthetic scaffold task, got %d", len(build.Tasks))
	}
	task := build.Tasks[0]
	if task.Description != "Bootstrap deterministic scaffold" {
		t.Fatalf("unexpected task description: %s", task.Description)
	}
	files := am.collectGeneratedFiles(build)
	if len(files) < count {
		t.Fatalf("expected collectGeneratedFiles to include scaffold files, got %d want >= %d", len(files), count)
	}
}

func TestCurrentOwnedFilesPromptIncludesSharedOwnedFile(t *testing.T) {
	t.Parallel()

	workOrder := &BuildWorkOrder{
		Role:          RoleFrontend,
		OwnedFiles:    []string{"package.json", "src/**"},
		RequiredFiles: []string{"package.json", "src/App.tsx"},
	}

	context := currentOwnedFilesPrompt([]GeneratedFile{
		{Path: "package.json", Language: "json", Content: `{"name":"demo"}`},
		{Path: "src/App.tsx", Language: "typescript", Content: "export default function App(){return null}"},
		{Path: "server/index.ts", Language: "typescript", Content: "export {}"},
	}, workOrder, 4000)

	if !strings.Contains(context, "// File: package.json") {
		t.Fatalf("expected shared package.json to appear in current owned files context: %s", context)
	}
	if strings.Contains(context, "// File: server/index.ts") {
		t.Fatalf("did not expect backend file in frontend owned context: %s", context)
	}
}

func TestTaskWorkOrderFromInputUsesArtifactFallback(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID: "task-1",
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				ID:                 "wo-1",
				BuildID:            "build-1",
				Role:               RoleBackend,
				Summary:            "Implement backend API",
				OwnedFiles:         []string{"server/**"},
				RequiredFiles:      []string{"server/index.ts"},
				ForbiddenFiles:     []string{"src/**"},
				SurfaceLocalChecks: []string{"server compiles"},
				RequiredOutputs:    []string{"health endpoint"},
			},
		},
	}

	workOrder := taskWorkOrderFromInput(task)
	if workOrder == nil {
		t.Fatal("expected legacy work order derived from artifact")
	}
	if workOrder.Role != RoleBackend || workOrder.Summary != "Implement backend API" {
		t.Fatalf("unexpected derived work order: %+v", workOrder)
	}
	if len(workOrder.RequiredFiles) != 1 || workOrder.RequiredFiles[0] != "server/index.ts" {
		t.Fatalf("expected required files to be preserved, got %+v", workOrder.RequiredFiles)
	}
}

func TestWorkOrderArtifactPromptContextIncludesContractSlice(t *testing.T) {
	t.Parallel()

	context := workOrderArtifactPromptContext(&WorkOrder{
		ID:                 "wo-1",
		BuildID:            "build-1",
		Role:               RoleFrontend,
		Category:           WorkOrderFrontend,
		TaskShape:          TaskShapeFrontendPatch,
		Summary:            "Implement dashboard shell",
		OwnedFiles:         []string{"src/**"},
		RequiredFiles:      []string{"src/main.tsx"},
		ReadableFiles:      []string{"package.json", "src/main.tsx"},
		ForbiddenFiles:     []string{"server/**"},
		RequiredOutputs:    []string{"DashboardPage"},
		RequiredSymbols:    []string{"DashboardPage"},
		SurfaceLocalChecks: []string{"dashboard route renders"},
		MaxContextBudget:   8000,
		RiskLevel:          RiskMedium,
		RoutingMode:        RoutingModeSingleProvider,
		PreferredProvider:  ai.ProviderGPT4,
		ContractSlice: WorkOrderContractSlice{
			Surface:         SurfaceFrontend,
			OwnedChecks:     []string{"dashboard route renders"},
			RelevantRoutes:  []string{"GET /api/dashboard"},
			RelevantEnvVars: []string{"VITE_API_URL"},
			RelevantModels:  []string{"DashboardCard"},
			TruthTags:       []TruthTag{TruthScaffolded},
		},
	})

	for _, want := range []string{
		"<work_order_artifact>",
		"summary: Implement dashboard shell",
		"readable_files:",
		"contract_relevant_routes:",
		"contract_truth_tags:",
	} {
		if !strings.Contains(context, want) {
			t.Fatalf("expected %q in artifact context, got %s", want, context)
		}
	}
}

func TestPathMatchesOwnedPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path    string
		pattern string
		want    bool
	}{
		// Wildcard all
		{"anything.go", "**", true},
		// Extension match: **/*.ext
		{"src/App.test.ts", "**/*.test.ts", true},
		{"tests/deep/nested/thing.test.ts", "**/*.test.ts", true},
		{"src/App.tsx", "**/*.test.ts", false},
		// Suffix match: **/*_test.go (the bug that was fixed)
		{"handlers/user_test.go", "**/*_test.go", true},
		{"internal/db/db_test.go", "**/*_test.go", true},
		{"main.go", "**/*_test.go", false},
		// Directory prefix: dir/**
		{"src/App.tsx", "src/**", true},
		{"src/components/Layout.tsx", "src/**", true},
		{"server/index.ts", "src/**", false},
		{"src", "src/**", true},
		// Nested glob: dir/**/*.ext
		{"src/components/App.tsx", "src/**/*.tsx", true},
		{"src/App.tsx", "src/**/*.tsx", true},
		{"server/index.ts", "src/**/*.tsx", false},
		// Exact match
		{"package.json", "package.json", true},
		{"tsconfig.json", "package.json", false},
		// Top-level extension: *.ts
		{"index.ts", "*.ts", true},
		{"src/index.ts", "*.ts", false},
		// Empty/edge cases
		{"file.go", "", false},
	}

	for _, tt := range tests {
		got := pathMatchesOwnedPattern(tt.path, tt.pattern)
		if got != tt.want {
			t.Errorf("pathMatchesOwnedPattern(%q, %q) = %v, want %v", tt.path, tt.pattern, got, tt.want)
		}
	}
}

func TestResolveBuildAppType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"web", "web"},
		{"api", "api"},
		{"cli", "cli"},
		{"fullstack", "fullstack"},
		{"frontend", "web"},
		{"spa", "web"},
		{"dashboard", "web"},
		{"backend", "api"},
		{"server", "api"},
		{"microservice", "api"},
		{"webapp", "fullstack"},
		{"saas", "fullstack"},
		{"unknown_thing", "fullstack"}, // default
	}

	for _, tt := range tests {
		bundle := &autonomous.PlanningBundle{
			Analysis: &autonomous.RequirementAnalysis{AppType: tt.input},
		}
		got := resolveBuildAppType("Build a product", nil, bundle)
		if got != tt.want {
			t.Errorf("resolveBuildAppType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveBuildAppTypePreservesBackendWithoutDatabase(t *testing.T) {
	t.Parallel()

	description := "Build a full-stack uptime monitor with a React frontend and Express backend API. No database, no auth, no external APIs."
	requested := &TechStack{Frontend: "React + Vite + TypeScript", Backend: "Node.js + Express", Styling: "Tailwind CSS"}
	bundle := &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{AppType: "web"},
	}

	if explicitStaticWebIntent(description) {
		t.Fatal("no database must not imply frontend-only/static intent")
	}
	if got := resolveBuildAppType(description, requested, bundle); got != "fullstack" {
		t.Fatalf("resolveBuildAppType() = %q, want fullstack", got)
	}

	stack := resolveBuildTechStack(description, requested, "fullstack", bundle)
	if stack.Backend != "Express" {
		t.Fatalf("backend = %q, want Express", stack.Backend)
	}
	if stack.Database != "" {
		t.Fatalf("database = %q, want empty for explicit no-database backend app", stack.Database)
	}
}

func TestSelectBuildScaffoldNewStacks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stack  TechStack
		wantID string
	}{
		{"react+express", TechStack{Frontend: "React", Backend: "Express"}, "fullstack/react-vite-express-ts"},
		{"react18 frontend preview", TechStack{Frontend: "React 18", Styling: "Tailwind CSS"}, "frontend/react-vite-spa"},
		{"react18+nodejs fullstack", TechStack{Frontend: "React 18", Backend: "Node.js + Express", Styling: "Tailwind CSS"}, "fullstack/react-vite-express-ts"},
		{"react+go", TechStack{Frontend: "React", Backend: "Go"}, "fullstack/react-vite-go"},
		{"react+python", TechStack{Frontend: "React", Backend: "Python"}, "fullstack/react-vite-fastapi"},
		{"react+fastapi", TechStack{Frontend: "React", Backend: "FastAPI"}, "fullstack/react-vite-fastapi"},
		{"nextjs standalone", TechStack{Frontend: "Next.js"}, "frontend/nextjs-app"},
		{"nextjs+api", TechStack{Frontend: "Next.js", Backend: "Express"}, "fullstack/nextjs-api"},
		{"python api", TechStack{Backend: "Python"}, "api/python-fastapi"},
		{"fastapi api", TechStack{Backend: "FastAPI"}, "api/python-fastapi"},
		{"go api", TechStack{Backend: "Go"}, "api/go-http"},
		{"react spa", TechStack{Frontend: "React"}, "frontend/react-vite-spa"},
		{"default fallback", TechStack{Backend: "Express"}, "api/express-typescript"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaffold := selectBuildScaffold("fullstack", tt.stack)
			if scaffold.ID != tt.wantID {
				t.Errorf("selectBuildScaffold(%q) = %q, want %q", tt.name, scaffold.ID, tt.wantID)
			}
		})
	}
}

func TestCreateBuildPlanFromPlanningBundlePulseBoardUsesFrontendScaffoldAndFrontendOwnership(t *testing.T) {
	t.Parallel()

	description := "Build a polished frontend-only client dashboard called PulseBoard using React 18, Vite, and Tailwind CSS with a responsive dark modern UI that works well in the preview pane, a dashboard home with KPI cards, trend widgets, an activity feed, and a highlighted primary action, a clients page with searchable cards, filters, empty states, and detail panels, a projects page with kanban-style status columns and clear progress visuals, a settings page with profile, notifications, and theme sections, realistic seed content in the UI so the preview feels complete immediately, strong loading, empty, and error states, reusable components and a clean file structure, and no backend, no database, and no fake API requirements in this free-tier preview pass."
	plan := createBuildPlanFromPlanningBundle("build-pulseboard", description, nil, &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			AppType: "web",
			TechStack: &autonomous.TechStack{
				Frontend: "React 18",
				Backend:  "none",
				Database: "none",
				Styling:  "Tailwind CSS",
			},
		},
		Plan: &autonomous.ExecutionPlan{
			ID:            "plan-pulseboard",
			EstimatedTime: 20 * time.Minute,
			CreatedAt:     time.Now().UTC(),
		},
	})

	if plan == nil {
		t.Fatal("expected build plan")
	}
	if plan.TechStack.Frontend != "React" {
		t.Fatalf("expected React frontend normalization, got %+v", plan.TechStack)
	}
	foundComponentsOwnership := false
	for _, ownership := range plan.Ownership {
		if ownership.Role == RoleFrontend && ownership.Path == "components.json" {
			foundComponentsOwnership = true
			break
		}
	}
	if !foundComponentsOwnership {
		t.Fatalf("expected frontend ownership to include components.json, got %+v", plan.Ownership)
	}
	if plan.TechStack.Styling != "Tailwind" {
		t.Fatalf("expected Tailwind styling normalization, got %+v", plan.TechStack)
	}
	if plan.ScaffoldID != "frontend/react-vite-spa" {
		t.Fatalf("expected frontend scaffold, got %q", plan.ScaffoldID)
	}
	if wo := getBuildWorkOrder(plan, RoleFrontend); wo == nil {
		t.Fatal("expected frontend work order")
	} else {
		if !pathAllowedByWorkOrder("src/App.tsx", wo) || !pathAllowedByWorkOrder("index.html", wo) || !pathAllowedByWorkOrder("vite.config.ts", wo) {
			t.Fatalf("expected frontend work order to own Vite app shell files, got %+v", wo)
		}
		for _, forbidden := range wo.ForbiddenFiles {
			if forbidden == "src/**" || forbidden == "index.html" || forbidden == "vite.config.ts" {
				t.Fatalf("did not expect frontend shell files to be forbidden, got %+v", wo.ForbiddenFiles)
			}
		}
	}
	if wo := getBuildWorkOrder(plan, RoleBackend); wo != nil {
		t.Fatalf("expected no backend work order for PulseBoard frontend-only plan, got %+v", wo)
	}
}

func TestSelectBuildScaffoldNextFullstackSeedsAPIContractAndAcceptance(t *testing.T) {
	t.Parallel()

	scaffold := selectBuildScaffold("fullstack", TechStack{Frontend: "Next.js", Backend: "Express"})
	if scaffold.ID != "fullstack/nextjs-api" {
		t.Fatalf("unexpected scaffold id: %s", scaffold.ID)
	}
	if scaffold.APIContract == nil {
		t.Fatal("expected next fullstack scaffold to include an API contract")
	}
	if scaffold.APIContract.BackendPort != 3000 {
		t.Fatalf("expected next fullstack backend port 3000, got %+v", scaffold.APIContract)
	}
	if len(scaffold.APIContract.Endpoints) == 0 || scaffold.APIContract.Endpoints[0].Path != "/api/health" {
		t.Fatalf("expected next fullstack health endpoint, got %+v", scaffold.APIContract)
	}
	joined := make([]string, 0, len(scaffold.Acceptance))
	for _, check := range scaffold.Acceptance {
		joined = append(joined, string(check.Owner)+":"+check.Description)
	}
	acceptance := strings.Join(joined, " | ")
	if !strings.Contains(acceptance, string(RoleBackend)) {
		t.Fatalf("expected backend acceptance coverage, got %s", acceptance)
	}
	if !strings.Contains(acceptance, string(RoleTesting)) {
		t.Fatalf("expected integration acceptance coverage, got %s", acceptance)
	}
}

func TestFullstackExpressScaffoldAssignsDatabaseRuntimeHelpersToDatabaseRole(t *testing.T) {
	t.Parallel()

	scaffold := selectBuildScaffold("fullstack", TechStack{Frontend: "React", Backend: "Express", Database: "PostgreSQL"})
	if scaffold.ID != "fullstack/react-vite-express-ts" {
		t.Fatalf("unexpected scaffold id: %s", scaffold.ID)
	}

	databaseOwned := scaffold.Ownership[RoleDatabase]
	if !slices.Contains(databaseOwned, "server/migrate.ts") {
		t.Fatalf("expected database role to own server/migrate.ts, got %+v", databaseOwned)
	}
	if !slices.Contains(databaseOwned, "server/seed.ts") {
		t.Fatalf("expected database role to own server/seed.ts, got %+v", databaseOwned)
	}
}

func TestResolveBuildTechStackDoesNotForceBackend(t *testing.T) {
	t.Parallel()

	// When planner says frontend=React with no backend, the system should
	// NOT force Backend=Express (old behavior)
	bundle := &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			TechStack: &autonomous.TechStack{
				Frontend: "React",
			},
		},
	}
	stack := resolveBuildTechStack("Build a marketing site", nil, "web", bundle)
	if stack.Backend != "" {
		t.Errorf("expected empty backend for frontend-only project, got %q", stack.Backend)
	}
	if stack.Database != "" {
		t.Errorf("expected empty database for frontend-only project, got %q", stack.Database)
	}
}

func TestResolveBuildTechStackNormalizesPlannerNoneLiterals(t *testing.T) {
	t.Parallel()

	bundle := &autonomous.PlanningBundle{
		Analysis: &autonomous.RequirementAnalysis{
			TechStack: &autonomous.TechStack{
				Frontend: "React",
				Backend:  "none",
				Database: "none",
				Styling:  "Tailwind",
			},
		},
	}

	stack := resolveBuildTechStack("Build a polished static landing page. Frontend only. No backend. No database.", nil, "web", bundle)
	if stack.Frontend != "React" {
		t.Fatalf("expected frontend to remain React, got %q", stack.Frontend)
	}
	if stack.Backend != "" {
		t.Fatalf("expected planner backend 'none' to normalize to empty, got %q", stack.Backend)
	}
	if stack.Database != "" {
		t.Fatalf("expected planner database 'none' to normalize to empty, got %q", stack.Database)
	}
}

func TestSelectBuildScaffoldReactVariantsIncludeShadcnRequiredFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		appType string
		stack   TechStack
	}{
		{name: "spa", appType: "web", stack: TechStack{Frontend: "React"}},
		{name: "express", appType: "fullstack", stack: TechStack{Frontend: "React", Backend: "Express"}},
		{name: "go", appType: "fullstack", stack: TechStack{Frontend: "React", Backend: "Go"}},
		{name: "fastapi", appType: "fullstack", stack: TechStack{Frontend: "React", Backend: "FastAPI"}},
	}

	requiredPaths := []string{
		"components.json",
		"src/lib/utils.ts",
		"src/components/ui/button.tsx",
		"src/components/ui/card.tsx",
		"src/components/ui/input.tsx",
		"src/components/ui/badge.tsx",
		"src/components/ui/dialog.tsx",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaffold := selectBuildScaffold(tt.appType, tt.stack)
			required := make([]string, 0, len(scaffold.Required))
			for _, file := range scaffold.Required {
				required = append(required, file.Path)
			}
			for _, path := range requiredPaths {
				if !slices.Contains(required, path) {
					t.Fatalf("expected scaffold %s to require %s, got %+v", scaffold.ID, path, required)
				}
			}
		})
	}
}

func TestScaffoldBootstrapFilesReactVariantsWireShadcnBaseline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		appType           string
		stack             TechStack
		expectedScaffold  string
		expectedBuildLine string
	}{
		{
			name:              "spa",
			appType:           "web",
			stack:             TechStack{Frontend: "React"},
			expectedScaffold:  "frontend/react-vite-spa",
			expectedBuildLine: `"build": "tsc -b && vite build"`,
		},
		{
			name:              "express",
			appType:           "fullstack",
			stack:             TechStack{Frontend: "React", Backend: "Express"},
			expectedScaffold:  "fullstack/react-vite-express-ts",
			expectedBuildLine: `"build:server": "tsc -p tsconfig.json"`,
		},
		{
			name:              "go",
			appType:           "fullstack",
			stack:             TechStack{Frontend: "React", Backend: "Go"},
			expectedScaffold:  "fullstack/react-vite-go",
			expectedBuildLine: `"build": "tsc -b && vite build"`,
		},
		{
			name:              "fastapi",
			appType:           "fullstack",
			stack:             TechStack{Frontend: "React", Backend: "FastAPI"},
			expectedScaffold:  "fullstack/react-vite-fastapi",
			expectedBuildLine: `"build": "tsc -b && vite build"`,
		},
	}

	fileMap := func(files []GeneratedFile) map[string]string {
		out := make(map[string]string, len(files))
		for _, file := range files {
			out[file.Path] = file.Content
		}
		return out
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaffold := selectBuildScaffold(tt.appType, tt.stack)
			if scaffold.ID != tt.expectedScaffold {
				t.Fatalf("expected scaffold %s, got %s", tt.expectedScaffold, scaffold.ID)
			}

			files := fileMap(scaffoldBootstrapFiles(scaffold, "Build a reliable launch-ready dashboard", tt.stack))
			for _, required := range []string{
				"components.json",
				"src/lib/utils.ts",
				"src/components/ui/button.tsx",
				"src/components/ui/dialog.tsx",
				"package.json",
				"tsconfig.json",
				"vite.config.ts",
				"tailwind.config.js",
				"src/index.css",
			} {
				if strings.TrimSpace(files[required]) == "" {
					t.Fatalf("expected generated scaffold file %s, got files %+v", required, keys(files))
				}
			}

			packageJSON := files["package.json"]
			for _, snippet := range []string{
				`"@radix-ui/react-dialog"`,
				`"@radix-ui/react-slot"`,
				`"class-variance-authority"`,
				`"clsx"`,
				`"tailwind-merge"`,
				`"tailwindcss-animate"`,
				tt.expectedBuildLine,
			} {
				if !strings.Contains(packageJSON, snippet) {
					t.Fatalf("expected package.json for %s to contain %q, got %s", scaffold.ID, snippet, packageJSON)
				}
			}

			if !strings.Contains(files["components.json"], `"iconLibrary": "lucide"`) {
				t.Fatalf("expected components.json to include lucide icon library, got %s", files["components.json"])
			}
			if !strings.Contains(files["src/lib/utils.ts"], "twMerge") {
				t.Fatalf("expected src/lib/utils.ts to include twMerge helper, got %s", files["src/lib/utils.ts"])
			}
			if !strings.Contains(files["src/components/ui/dialog.tsx"], "@radix-ui/react-dialog") {
				t.Fatalf("expected dialog primitive to reference radix dialog, got %s", files["src/components/ui/dialog.tsx"])
			}
			if !strings.Contains(files["tsconfig.json"], `"@/*": ["./src/*"]`) {
				t.Fatalf("expected tsconfig alias wiring, got %s", files["tsconfig.json"])
			}
			if !strings.Contains(files["vite.config.ts"], `import path from "path";`) ||
				!strings.Contains(files["vite.config.ts"], `alias: {`) ||
				!strings.Contains(files["vite.config.ts"], `"@"`) {
				t.Fatalf("expected vite alias wiring, got %s", files["vite.config.ts"])
			}
			if !strings.Contains(files["tailwind.config.js"], `tailwindcss-animate`) {
				t.Fatalf("expected tailwind animate plugin, got %s", files["tailwind.config.js"])
			}
			if !strings.Contains(files["src/index.css"], "--background: 222 47% 7%;") {
				t.Fatalf("expected shadcn theme tokens in src/index.css, got %s", files["src/index.css"])
			}
			if scaffold.ID == "frontend/react-vite-spa" && !strings.Contains(files["src/App.tsx"], `className="hero"`) {
				t.Fatalf("expected frontend SPA scaffold to render through the hero shell, got %s", files["src/App.tsx"])
			}
		})
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	slices.Sort(out)
	return out
}

func hasAcceptanceCheckPrefix(checks []BuildAcceptanceCheck, prefix string) bool {
	for _, check := range checks {
		if strings.HasPrefix(check.ID, prefix) || strings.HasPrefix(check.Description, prefix) {
			return true
		}
	}
	return false
}
