package architecture

type pocketRule struct {
	ID          string
	Name        string
	Layer       string
	Type        string
	Description string
	Directory   string
	RiskLevel   string
	RiskScore   int
	Tags        []string
	Keywords    []string
}

type contractRule struct {
	ID             string
	ContractType   string
	Producer       string
	Consumers      []string
	SchemaLocation string
	TestLocations  []string
	Keywords       []string
}

type databaseRule struct {
	ID       string
	Keywords []string
}

type structureRule struct {
	ID       string
	Keywords []string
}

func defaultPocketRules() []pocketRule {
	return []pocketRule{
		{
			ID: "web.builder", Name: "Build Workspace UI", Layer: "presentation", Type: "frontend",
			Description: "Build creation, status, orchestration visibility, provider controls, and recovery UX.",
			Directory:   "frontend/src/components/builder", RiskLevel: "high", RiskScore: 76,
			Tags:     []string{"build-status", "websocket-consumer", "user-flow"},
			Keywords: []string{"frontend/src/components/builder", "AppBuilder", "BuildScreen", "build workspace", "build status"},
		},
		{
			ID: "web.ide", Name: "IDE Workspace UI", Layer: "presentation", Type: "frontend",
			Description: "Editor, right-side operational panels, project shell, and IDE-adjacent workflows.",
			Directory:   "frontend/src/components/ide", RiskLevel: "medium", RiskScore: 58,
			Tags:     []string{"ide", "editor", "workspace"},
			Keywords: []string{"frontend/src/components/ide", "IDELayout", "right panel", "editor workspace"},
		},
		{
			ID: "web.preview", Name: "Preview Runtime UI", Layer: "presentation", Type: "frontend",
			Description: "Preview runtime pane, console/network panels, toolbar, and browser-local preview state.",
			Directory:   "frontend/src/components/preview", RiskLevel: "high", RiskScore: 73,
			Tags:     []string{"preview", "runtime", "canary"},
			Keywords: []string{"frontend/src/components/preview", "LivePreview", "PreviewRuntimePane", "preview runtime"},
		},
		{
			ID: "web.api_client", Name: "Frontend API Client", Layer: "client", Type: "api_client",
			Description: "Typed browser API service and request/response client contracts.",
			Directory:   "frontend/src/services", RiskLevel: "high", RiskScore: 72,
			Tags:     []string{"api-contract", "auth", "transport"},
			Keywords: []string{"frontend/src/services", "api.ts", "ApiService", "axios", "client contract"},
		},
		{
			ID: "web.admin", Name: "Admin Operations UI", Layer: "presentation", Type: "frontend",
			Description: "Admin-only operational controls, user management, and platform diagnostics.",
			Directory:   "frontend/src/components/admin", RiskLevel: "high", RiskScore: 70,
			Tags:     []string{"admin", "ops"},
			Keywords: []string{"frontend/src/components/admin", "AdminDashboard", "admin dashboard"},
		},
		{
			ID: "api.core", Name: "Core API Server", Layer: "api", Type: "backend",
			Description: "Authentication, project/file APIs, admin APIs, and platform route setup.",
			Directory:   "backend/internal/api", RiskLevel: "critical", RiskScore: 88,
			Tags:     []string{"api", "auth", "admin"},
			Keywords: []string{"backend/internal/api", "api.Server", "AdminGet", "AuthMiddleware", "project endpoints"},
		},
		{
			ID: "api.handlers", Name: "Feature API Handlers", Layer: "api", Type: "backend",
			Description: "Preview, spend, budget, deployment, secrets, import/export, and auxiliary handlers.",
			Directory:   "backend/internal/handlers", RiskLevel: "high", RiskScore: 77,
			Tags:     []string{"api", "feature-handlers"},
			Keywords: []string{"backend/internal/handlers", "RegisterRoutes", "PreviewHandler", "SpendHandler", "BudgetHandler"},
		},
		{
			ID: "ai.orchestration", Name: "Build Orchestration Engine", Layer: "ai-orchestration", Type: "backend",
			Description: "Agent manager, tasks, build lifecycle, recovery loops, quality gates, snapshots, and websocket events.",
			Directory:   "backend/internal/agents", RiskLevel: "critical", RiskScore: 96,
			Tags:     []string{"agents", "build-lifecycle", "quality-gates", "websocket"},
			Keywords: []string{"backend/internal/agents", "AgentManager", "BuildSnapshotState", "BuildStatus", "TaskOutput", "quality gate", "orchestration"},
		},
		{
			ID: "ai.providers", Name: "Provider Router and BYOK", Layer: "ai-provider", Type: "backend",
			Description: "Provider clients, model routing, BYOK validation, pricing metadata, and usage normalization.",
			Directory:   "backend/internal/ai", RiskLevel: "critical", RiskScore: 91,
			Tags:     []string{"provider-routing", "byok", "cost"},
			Keywords: []string{"backend/internal/ai", "AIRouter", "BYOKManager", "ProviderOllama", "ProviderGPT4", "model router"},
		},
		{
			ID: "runtime.preview", Name: "Preview Backend Runtime", Layer: "runtime", Type: "backend",
			Description: "Preview process/container runtime, canary verifier, bundler bridge, and proxy behavior.",
			Directory:   "backend/internal/preview", RiskLevel: "critical", RiskScore: 93,
			Tags:     []string{"preview", "runtime", "canary", "sandbox"},
			Keywords: []string{"backend/internal/preview", "PreviewHandler", "VerifyFiles", "canary", "container preview", "sandbox"},
		},
		{
			ID: "data.persistence", Name: "Persistence and Migrations", Layer: "data", Type: "database",
			Description: "Database connection, Redis, seed data, migrations, completed build snapshots, and persistence contracts.",
			Directory:   "backend/internal/db", RiskLevel: "critical", RiskScore: 90,
			Tags:     []string{"postgres", "redis", "migrations", "snapshots"},
			Keywords: []string{"backend/internal/db", "backend/migrations", "CompletedBuild", "Redis", "PostgreSQL", "snapshot"},
		},
		{
			ID: "data.migrations", Name: "SQL Migration Files", Layer: "data", Type: "database",
			Description: "Schema migrations and durable database structure changes.",
			Directory:   "backend/migrations", RiskLevel: "critical", RiskScore: 89,
			Tags:     []string{"migrations", "schema"},
			Keywords: []string{"backend/migrations", "migration", "CREATE TABLE", "ALTER TABLE"},
		},
		{
			ID: "security.auth", Name: "Auth and Security Boundary", Layer: "security", Type: "backend",
			Description: "JWT, cookies, OAuth, password handling, CORS, security headers, and request protection.",
			Directory:   "backend/internal/auth", RiskLevel: "critical", RiskScore: 94,
			Tags:     []string{"auth", "security", "tenant-boundary"},
			Keywords: []string{"backend/internal/auth", "JWT", "AuthMiddleware", "CSRF", "security boundary", "tenant"},
		},
		{
			ID: "deployment.runtime", Name: "Deployment and Hosting", Layer: "deployment", Type: "backend",
			Description: "External deployment, native hosting, deploy logs, Git export/import, and always-on behavior.",
			Directory:   "backend/internal/deploy", RiskLevel: "high", RiskScore: 78,
			Tags:     []string{"deploy", "hosting", "render"},
			Keywords: []string{"backend/internal/deploy", "backend/internal/hosting", "render.yaml", "deployment", "hosting"},
		},
		{
			ID: "billing.spend", Name: "Billing, Budget, and Spend", Layer: "business", Type: "backend",
			Description: "Stripe billing, credits, budget caps, spend tracking, and unit-economics controls.",
			Directory:   "backend/internal/billing", RiskLevel: "critical", RiskScore: 92,
			Tags:     []string{"billing", "spend", "budget"},
			Keywords: []string{"backend/internal/billing", "backend/internal/spend", "backend/internal/budget", "Stripe", "budget cap", "spend"},
		},
		{
			ID: "tests.e2e", Name: "End-to-End Tests", Layer: "verification", Type: "test",
			Description: "Browser-level smoke and regression coverage for generated app and platform flows.",
			Directory:   "tests/e2e", RiskLevel: "medium", RiskScore: 52,
			Tags:     []string{"playwright", "smoke"},
			Keywords: []string{"tests/e2e", "Playwright", "smoke test", "browser flow"},
		},
		{
			ID: "docs.architecture_intelligence", Name: "Architecture Intelligence Reference", Layer: "knowledge", Type: "docs",
			Description: "Imported schema, example map, diagnostics, and implementation plan for architecture intelligence.",
			Directory:   "docs/architecture-intelligence", RiskLevel: "medium", RiskScore: 55,
			Tags:     []string{"architecture-map", "agent-reference"},
			Keywords: []string{"docs/architecture-intelligence", "Architecture Intelligence Map", "AGENT_REFERENCE"},
		},
	}
}

func defaultContractRules() []contractRule {
	return []contractRule{
		{
			ID: "contract.build.lifecycle", ContractType: "build_state",
			Producer: "ai.orchestration", Consumers: []string{"web.builder", "web.api_client"},
			SchemaLocation: "backend/internal/agents/types.go",
			TestLocations:  []string{"backend/internal/agents/*_test.go", "frontend/src/components/builder/*.test.tsx"},
			Keywords:       []string{"BuildStatus", "BuildSnapshotState", "BuildResponse", "build lifecycle", "build:progress", "build:completed"},
		},
		{
			ID: "contract.preview.runtime", ContractType: "preview_runtime",
			Producer: "runtime.preview", Consumers: []string{"web.preview", "web.builder"},
			SchemaLocation: "backend/internal/handlers/preview*.go",
			TestLocations:  []string{"backend/internal/handlers/*preview*_test.go", "frontend/src/components/preview/*.test.tsx"},
			Keywords:       []string{"preview runtime", "PreviewStatus", "StartPreview", "preview proxy", "preview:ready"},
		},
		{
			ID: "contract.provider.routing", ContractType: "ai_provider",
			Producer: "ai.providers", Consumers: []string{"ai.orchestration", "billing.spend"},
			SchemaLocation: "backend/internal/agents/ai_adapter.go",
			TestLocations:  []string{"backend/internal/agents/*routing*_test.go", "backend/internal/ai/*_test.go"},
			Keywords:       []string{"provider routing", "ModelOverride", "BYOK", "ProviderOllama", "ProviderGPT4", "PowerMode"},
		},
		{
			ID: "contract.billing.spend", ContractType: "billing",
			Producer: "billing.spend", Consumers: []string{"api.handlers", "web.api_client"},
			SchemaLocation: "backend/internal/spend",
			TestLocations:  []string{"backend/internal/spend/*_test.go", "frontend/src/components/spend/*.test.tsx"},
			Keywords:       []string{"spend", "budget", "credits", "Stripe", "usage_events", "RecordSpendInput"},
		},
		{
			ID: "contract.auth.session", ContractType: "auth_policy",
			Producer: "security.auth", Consumers: []string{"api.core", "web.api_client"},
			SchemaLocation: "backend/internal/auth",
			TestLocations:  []string{"backend/internal/auth/*_test.go", "frontend/src/services/api.test.ts"},
			Keywords:       []string{"AuthMiddleware", "JWT", "refresh token", "CSRF", "cookie session"},
		},
		{
			ID: "contract.architecture.map", ContractType: "architecture_intelligence",
			Producer: "docs.architecture_intelligence", Consumers: []string{"web.admin", "api.core", "ai.orchestration"},
			SchemaLocation: "docs/architecture-intelligence/architecture-map.schema.json",
			TestLocations:  []string{"backend/internal/architecture/*_test.go"},
			Keywords:       []string{"architecture map", "blast radius", "reference telemetry", "Architecture Intelligence"},
		},
	}
}

func defaultDatabaseRules() []databaseRule {
	return []databaseRule{
		{ID: "db.completed_build_snapshots", Keywords: []string{"completed_builds", "CompletedBuild", "completed build snapshot", "snapshot"}},
		{ID: "db.build_interactions", Keywords: []string{"build_interactions", "BuildInteractionState", "permission_requests", "approval"}},
		{ID: "db.users_auth", Keywords: []string{"users", "User", "auth", "refresh token", "jwt"}},
		{ID: "db.projects_files", Keywords: []string{"projects", "files", "models.File", "Project", "project files"}},
		{ID: "db.usage_spend", Keywords: []string{"usage_events", "spend", "credits", "billing", "budget"}},
	}
}

func defaultStructureRules() []structureRule {
	return []structureRule{
		{ID: "BuildRequest", Keywords: []string{"BuildRequest"}},
		{ID: "BuildResponse", Keywords: []string{"BuildResponse"}},
		{ID: "BuildStatus", Keywords: []string{"BuildStatus", "BuildCompleted", "BuildFailed", "BuildReviewing"}},
		{ID: "BuildSnapshotState", Keywords: []string{"BuildSnapshotState", "snapshot_state"}},
		{ID: "BuildOrchestrationState", Keywords: []string{"BuildOrchestrationState", "orchestration state"}},
		{ID: "TaskOutput", Keywords: []string{"TaskOutput", "GeneratedFile", "StructuredPatchBundle"}},
		{ID: "VerificationReport", Keywords: []string{"VerificationReport", "quality gate", "verification report"}},
		{ID: "BuildInteractionState", Keywords: []string{"BuildInteractionState", "permission request", "approval event"}},
	}
}
