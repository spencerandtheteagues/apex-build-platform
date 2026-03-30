package agents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"apex-build/internal/agents/autonomous"
	"apex-build/internal/ai"

	"github.com/google/uuid"
)

type buildScaffold struct {
	ID          string
	AppType     string
	Description string
	Required    []PlannedFile
	Ownership   map[AgentRole][]string
	EnvVars     []BuildEnvVar
	Acceptance  []BuildAcceptanceCheck
	APIContract *BuildAPIContract
}

type plannerRouterAdapter struct {
	router          AIRouter
	provider        ai.AIProvider
	userID          uint
	powerMode       PowerMode
	usePlatformKeys bool
}

func (a *plannerRouterAdapter) Generate(ctx context.Context, prompt string, opts autonomous.AIOptions) (string, error) {
	resp, err := a.router.Generate(ctx, a.provider, prompt, GenerateOptions{
		UserID:          a.userID,
		MaxTokens:       opts.MaxTokens,
		Temperature:     opts.Temperature,
		SystemPrompt:    opts.SystemPrompt,
		PowerMode:       a.powerMode,
		UsePlatformKeys: a.usePlatformKeys,
	})
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("empty response from planning provider")
	}
	return resp.Content, nil
}

func (a *plannerRouterAdapter) Analyze(ctx context.Context, content string, instruction string, opts autonomous.AIOptions) (string, error) {
	prompt := fmt.Sprintf("Content to analyze:\n%s\n\nInstruction: %s", content, instruction)
	return a.Generate(ctx, prompt, opts)
}

func createBuildPlanFromPlanningBundle(buildID string, description string, requested *TechStack, bundle *autonomous.PlanningBundle) *BuildPlan {
	appType := resolveBuildAppType(description, requested, bundle)
	stack := resolveBuildTechStack(description, requested, appType, bundle)
	scaffold := selectBuildScaffold(appType, stack)

	features := convertPlannedFeatures(bundle)
	models := convertPlannedModels(bundle)
	if appType == "web" && strings.TrimSpace(stack.Database) == "" {
		models = nil
	}
	files := mergePlannedFiles(scaffold.Required, planDerivedFiles(scaffold, bundle)...)
	contract := cloneAPIContract(scaffold.APIContract)
	envVars := append([]BuildEnvVar(nil), scaffold.EnvVars...)
	preflight := convertPreflightChecks(bundle)
	acceptance := append([]BuildAcceptanceCheck(nil), scaffold.Acceptance...)
	acceptance = append(acceptance, deriveAcceptanceChecks(appType, stack)...)
	ownership := buildOwnershipMap(scaffold)
	workOrders := buildWorkOrders(appType, stack, scaffold, ownership, acceptance)
	scaffoldFiles := scaffoldBootstrapFiles(scaffold, description, stack)
	estimatedTime := 45 * time.Minute
	createdAt := time.Now()
	if bundle != nil && bundle.Plan != nil {
		estimatedTime = bundle.Plan.EstimatedTime
		if !bundle.Plan.CreatedAt.IsZero() {
			createdAt = bundle.Plan.CreatedAt
		}
	}
	plan := &BuildPlan{
		ID:            uuid.New().String(),
		BuildID:       buildID,
		AppType:       appType,
		DeliveryMode:  defaultDeliveryModeForAppType(appType),
		TechStack:     stack,
		Features:      features,
		DataModels:    models,
		APIEndpoints:  apiEndpointsFromContract(contract),
		Files:         files,
		ScaffoldFiles: scaffoldFiles,
		ScaffoldID:    scaffold.ID,
		Source:        "autonomous_planner_v1",
		Ownership:     ownership,
		EnvVars:       envVars,
		Acceptance:    dedupeAcceptanceChecks(acceptance),
		WorkOrders:    workOrders,
		APIContract:   contract,
		Preflight:     preflight,
		EstimatedTime: estimatedTime,
		CreatedAt:     createdAt,
	}
	plan.SpecHash = hashBuildPlan(plan)

	// Validate that the plan has a non-empty file manifest before returning.
	// An empty manifest means the planner returned a malformed bundle; letting
	// the guarantee engine attempt execution would burn retries on a plan that
	// cannot produce any output.
	if len(plan.Files) == 0 {
		log.Printf("[build_spec] WARNING: build %s plan has empty file manifest (appType=%s scaffold=%s) — plan may be unusable", buildID, appType, scaffold.ID)
	}

	return plan
}

func applyBuildAssurancePolicyToPlan(build *Build, plan *BuildPlan) *BuildPlan {
	if build == nil || plan == nil || !buildRequiresStaticFrontendFallback(build) {
		return plan
	}

	staticStack := plan.TechStack
	if strings.TrimSpace(staticStack.Frontend) == "" {
		staticStack.Frontend = "React"
	}
	if strings.TrimSpace(staticStack.Styling) == "" && strings.TrimSpace(staticStack.Frontend) != "" {
		staticStack.Styling = "Tailwind"
	}
	staticStack.Backend = ""
	staticStack.Database = ""

	scaffold := selectBuildScaffold("web", staticStack)
	ownership := buildOwnershipMap(scaffold)
	acceptance := append([]BuildAcceptanceCheck(nil), scaffold.Acceptance...)
	acceptance = append(acceptance, deriveAcceptanceChecks("web", staticStack)...)
	acceptance = append(acceptance,
		BuildAcceptanceCheck{
			ID:          "frontend-preview-guarantee",
			Description: "Frontend must compile into a usable interactive preview that matches the requested product surface within frontend-only scope",
			Owner:       RoleFrontend,
			Required:    true,
		},
		BuildAcceptanceCheck{
			ID:          "tier-honesty-contract",
			Description: "Architecture must record any deferred backend/data/runtime contract honestly so a paid follow-up can wire it behind the shipped UI without redesigning the app",
			Owner:       RoleArchitect,
			Required:    true,
		},
	)

	plan.AppType = "web"
	plan.DeliveryMode = "frontend_preview_only"
	plan.TechStack = staticStack
	plan.Files = mergePlannedFiles(scaffold.Required, filterFrontendFallbackPlannedFiles(plan.Files)...)
	plan.ScaffoldID = scaffold.ID
	plan.ScaffoldFiles = scaffoldBootstrapFiles(scaffold, build.Description, staticStack)
	plan.Ownership = ownership
	plan.EnvVars = filterFrontendFallbackEnvVars(plan.EnvVars)
	plan.Acceptance = dedupeAcceptanceChecks(acceptance)
	plan.WorkOrders = buildWorkOrders("web", staticStack, scaffold, ownership, plan.Acceptance)
	plan.APIContract = nil
	plan.APIEndpoints = nil
	plan.DataModels = nil
	plan.SpecHash = hashBuildPlan(plan)

	return plan
}

func defaultDeliveryModeForAppType(appType string) string {
	switch strings.TrimSpace(strings.ToLower(appType)) {
	case "fullstack":
		return "full_stack_preview"
	case "api":
		return "api_runtime"
	default:
		return "frontend_preview"
	}
}

func filterFrontendFallbackPlannedFiles(files []PlannedFile) []PlannedFile {
	if len(files) == 0 {
		return nil
	}

	out := make([]PlannedFile, 0, len(files))
	for _, file := range files {
		path := strings.TrimSpace(strings.ToLower(file.Path))
		switch {
		case path == "":
			continue
		case file.Type == "backend" || file.Type == "database":
			continue
		case strings.HasPrefix(path, "server/"),
			strings.HasPrefix(path, "api/"),
			strings.HasPrefix(path, "routers/"),
			strings.HasPrefix(path, "models/"),
			strings.HasPrefix(path, "services/"),
			strings.HasPrefix(path, "middleware/"),
			strings.HasPrefix(path, "handlers/"),
			strings.HasPrefix(path, "internal/"),
			strings.HasPrefix(path, "pkg/"),
			strings.HasPrefix(path, "cmd/"),
			strings.HasPrefix(path, "migrations/"),
			strings.HasPrefix(path, "db/"),
			strings.HasPrefix(path, "prisma/"),
			strings.HasSuffix(path, "schema.sql"),
			path == "go.mod",
			path == "requirements.txt",
			path == "main.go",
			path == "main.py":
			continue
		default:
			out = append(out, file)
		}
	}
	return out
}

func filterFrontendFallbackEnvVars(envVars []BuildEnvVar) []BuildEnvVar {
	if len(envVars) == 0 {
		return nil
	}

	out := make([]BuildEnvVar, 0, len(envVars))
	for _, env := range envVars {
		name := strings.TrimSpace(strings.ToUpper(env.Name))
		switch {
		case name == "":
			continue
		case strings.HasPrefix(name, "VITE_"):
			out = append(out, env)
		case strings.Contains(name, "FRONTEND"):
			out = append(out, env)
		}
	}
	return out
}

func summarizeBuildPlan(plan *BuildPlan) string {
	if plan == nil {
		return ""
	}
	parts := []string{
		fmt.Sprintf("Frozen build spec %s using scaffold %s for a %s app.", plan.SpecHash, plan.ScaffoldID, plan.AppType),
	}
	if len(plan.Files) > 0 {
		parts = append(parts, fmt.Sprintf("Required manifest includes %d files.", len(plan.Files)))
	}
	if len(plan.ScaffoldFiles) > 0 {
		parts = append(parts, fmt.Sprintf("%d deterministic scaffold files were preloaded.", len(plan.ScaffoldFiles)))
	}
	if len(plan.WorkOrders) > 0 {
		parts = append(parts, fmt.Sprintf("%d role work orders were generated.", len(plan.WorkOrders)))
	}
	if len(plan.Acceptance) > 0 {
		parts = append(parts, fmt.Sprintf("%d acceptance checks are active.", len(plan.Acceptance)))
	}
	return strings.Join(parts, " ")
}

func getBuildWorkOrder(plan *BuildPlan, role AgentRole) *BuildWorkOrder {
	if plan == nil {
		return nil
	}
	for i := range plan.WorkOrders {
		if plan.WorkOrders[i].Role == role {
			order := plan.WorkOrders[i]
			return &order
		}
	}
	return nil
}

func buildSpecPromptContext(plan *BuildPlan, workOrder *BuildWorkOrder) string {
	if plan == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n<build_spec>\n")
	b.WriteString(fmt.Sprintf("spec_hash: %s\n", plan.SpecHash))
	b.WriteString(fmt.Sprintf("scaffold_id: %s\n", plan.ScaffoldID))
	b.WriteString(fmt.Sprintf("app_type: %s\n", plan.AppType))
	b.WriteString(fmt.Sprintf("tech_stack: frontend=%s backend=%s database=%s styling=%s\n",
		valueOrNone(plan.TechStack.Frontend),
		valueOrNone(plan.TechStack.Backend),
		valueOrNone(plan.TechStack.Database),
		valueOrNone(plan.TechStack.Styling),
	))
	if plan.APIContract != nil {
		b.WriteString("<api_contract>\n")
		if plan.APIContract.FrontendPort > 0 {
			b.WriteString(fmt.Sprintf("frontend_port: %d\n", plan.APIContract.FrontendPort))
		}
		if plan.APIContract.BackendPort > 0 {
			b.WriteString(fmt.Sprintf("backend_port: %d\n", plan.APIContract.BackendPort))
		}
		if plan.APIContract.APIBaseURL != "" {
			b.WriteString(fmt.Sprintf("api_base_url: %s\n", plan.APIContract.APIBaseURL))
		}
		if len(plan.APIContract.CORSOrigins) > 0 {
			b.WriteString(fmt.Sprintf("cors_origins: %s\n", strings.Join(plan.APIContract.CORSOrigins, ", ")))
		}
		if len(plan.APIContract.Endpoints) > 0 {
			b.WriteString("endpoints:\n")
			for _, endpoint := range plan.APIContract.Endpoints {
				b.WriteString(fmt.Sprintf("- %s %s — %s\n", endpoint.Method, endpoint.Path, endpoint.Description))
			}
		}
		b.WriteString("</api_contract>\n")
	}
	if len(plan.Files) > 0 {
		b.WriteString("required_file_manifest:\n")
		for _, file := range plan.Files {
			b.WriteString(fmt.Sprintf("- %s (%s) — %s\n", file.Path, file.Type, file.Description))
		}
	}
	if len(plan.ScaffoldFiles) > 0 {
		b.WriteString(fmt.Sprintf("preloaded_scaffold_files: %d deterministic starter files are already available in the repo state\n", len(plan.ScaffoldFiles)))
	}
	if len(plan.EnvVars) > 0 {
		b.WriteString("env_vars:\n")
		for _, env := range plan.EnvVars {
			b.WriteString(fmt.Sprintf("- %s (required=%t) — %s\n", env.Name, env.Required, strings.TrimSpace(env.Purpose)))
		}
	}
	if len(plan.Acceptance) > 0 {
		b.WriteString("acceptance_checks:\n")
		for _, check := range plan.Acceptance {
			b.WriteString(fmt.Sprintf("- [%s] %s\n", check.Owner, check.Description))
		}
	}
	b.WriteString("</build_spec>\n")

	if workOrder != nil {
		b.WriteString("\n<work_order>\n")
		b.WriteString(fmt.Sprintf("role: %s\n", workOrder.Role))
		b.WriteString(fmt.Sprintf("summary: %s\n", workOrder.Summary))
		if len(workOrder.OwnedFiles) > 0 {
			b.WriteString("owned_files:\n")
			for _, path := range workOrder.OwnedFiles {
				b.WriteString(fmt.Sprintf("- %s\n", path))
			}
		}
		if len(workOrder.RequiredFiles) > 0 {
			b.WriteString("required_scaffold_files:\n")
			for _, path := range workOrder.RequiredFiles {
				b.WriteString(fmt.Sprintf("- %s\n", path))
			}
		}
		if len(workOrder.ForbiddenFiles) > 0 {
			b.WriteString("forbidden_files:\n")
			for _, path := range workOrder.ForbiddenFiles {
				b.WriteString(fmt.Sprintf("- %s\n", path))
			}
		}
		if len(workOrder.AcceptanceChecks) > 0 {
			b.WriteString("role_acceptance_checks:\n")
			for _, check := range workOrder.AcceptanceChecks {
				b.WriteString(fmt.Sprintf("- %s\n", check))
			}
		}
		if len(workOrder.RequiredOutputs) > 0 {
			b.WriteString("required_outputs:\n")
			for _, out := range workOrder.RequiredOutputs {
				b.WriteString(fmt.Sprintf("- %s\n", out))
			}
		}
		b.WriteString("</work_order>\n")
	}

	return b.String()
}

func workOrderArtifactPromptContext(workOrder *WorkOrder) string {
	if workOrder == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n<work_order_artifact>\n")
	b.WriteString(fmt.Sprintf("id: %s\n", workOrder.ID))
	b.WriteString(fmt.Sprintf("role: %s\n", workOrder.Role))
	b.WriteString(fmt.Sprintf("category: %s\n", workOrder.Category))
	b.WriteString(fmt.Sprintf("task_shape: %s\n", workOrder.TaskShape))
	if summary := strings.TrimSpace(workOrder.Summary); summary != "" {
		b.WriteString(fmt.Sprintf("summary: %s\n", summary))
	}
	b.WriteString(fmt.Sprintf("risk_level: %s\n", workOrder.RiskLevel))
	b.WriteString(fmt.Sprintf("routing_mode: %s\n", workOrder.RoutingMode))
	if workOrder.PreferredProvider != "" {
		b.WriteString(fmt.Sprintf("preferred_provider: %s\n", workOrder.PreferredProvider))
	}
	if workOrder.MaxContextBudget > 0 {
		b.WriteString(fmt.Sprintf("max_context_budget: %d\n", workOrder.MaxContextBudget))
	}
	if len(workOrder.OwnedFiles) > 0 {
		b.WriteString("owned_files:\n")
		for _, path := range workOrder.OwnedFiles {
			b.WriteString(fmt.Sprintf("- %s\n", path))
		}
	}
	if len(workOrder.RequiredFiles) > 0 {
		b.WriteString("required_files:\n")
		for _, path := range workOrder.RequiredFiles {
			b.WriteString(fmt.Sprintf("- %s\n", path))
		}
	}
	if len(workOrder.ReadableFiles) > 0 {
		b.WriteString("readable_files:\n")
		for _, path := range workOrder.ReadableFiles {
			b.WriteString(fmt.Sprintf("- %s\n", path))
		}
	}
	if len(workOrder.ForbiddenFiles) > 0 {
		b.WriteString("forbidden_files:\n")
		for _, path := range workOrder.ForbiddenFiles {
			b.WriteString(fmt.Sprintf("- %s\n", path))
		}
	}
	if len(workOrder.RequiredOutputs) > 0 {
		b.WriteString("required_outputs:\n")
		for _, output := range workOrder.RequiredOutputs {
			b.WriteString(fmt.Sprintf("- %s\n", output))
		}
	}
	if len(workOrder.RequiredSymbols) > 0 {
		b.WriteString("required_exports:\n")
		for _, symbol := range workOrder.RequiredSymbols {
			b.WriteString(fmt.Sprintf("- %s\n", symbol))
		}
	}
	if len(workOrder.SurfaceLocalChecks) > 0 {
		b.WriteString("surface_local_acceptance_checks:\n")
		for _, check := range workOrder.SurfaceLocalChecks {
			b.WriteString(fmt.Sprintf("- %s\n", check))
		}
	}
	b.WriteString(fmt.Sprintf("contract_surface: %s\n", workOrder.ContractSlice.Surface))
	if len(workOrder.ContractSlice.OwnedChecks) > 0 {
		b.WriteString("contract_owned_checks:\n")
		for _, check := range workOrder.ContractSlice.OwnedChecks {
			b.WriteString(fmt.Sprintf("- %s\n", check))
		}
	}
	if len(workOrder.ContractSlice.RelevantRoutes) > 0 {
		b.WriteString("contract_relevant_routes:\n")
		for _, route := range workOrder.ContractSlice.RelevantRoutes {
			b.WriteString(fmt.Sprintf("- %s\n", route))
		}
	}
	if len(workOrder.ContractSlice.RelevantEnvVars) > 0 {
		b.WriteString("contract_relevant_env_vars:\n")
		for _, env := range workOrder.ContractSlice.RelevantEnvVars {
			b.WriteString(fmt.Sprintf("- %s\n", env))
		}
	}
	if len(workOrder.ContractSlice.RelevantModels) > 0 {
		b.WriteString("contract_relevant_models:\n")
		for _, model := range workOrder.ContractSlice.RelevantModels {
			b.WriteString(fmt.Sprintf("- %s\n", model))
		}
	}
	if len(workOrder.ContractSlice.TruthTags) > 0 {
		b.WriteString("contract_truth_tags:\n")
		for _, tag := range workOrder.ContractSlice.TruthTags {
			b.WriteString(fmt.Sprintf("- %s\n", tag))
		}
	}
	b.WriteString("</work_order_artifact>\n")
	return b.String()
}

func coordinationProtocolPrompt(workOrder *BuildWorkOrder) string {
	if workOrder == nil {
		return ""
	}
	return `
<coordination_protocol>
- You are operating under a frozen BuildSpec and WorkOrder. Do not improvise a new project layout.
- Before any code or prose, emit a <task_start_ack> tag whose body is VALID JSON with keys:
  {"summary":"...","owned_files":["..."],"dependencies":["..."],"acceptance_checks":["..."],"blockers":["..."]}
- After all code/prose, emit a <task_completion_report> tag whose body is VALID JSON with keys:
  {"summary":"...","created_files":["..."],"modified_files":["..."],"completed_checks":["..."],"remaining_risks":["..."],"blockers":["..."]}
- Treat preloaded scaffold files as the baseline. Modify them in place instead of inventing a different repo layout.
- Only create or modify files inside your owned file patterns unless the WorkOrder explicitly lists shared root files.
- If a needed change belongs to another role, record it in blockers or remaining_risks instead of silently taking over their area.
- Fast and balanced builds must adhere to the scaffold exactly. Missing mandatory scaffold files is a task failure.
</coordination_protocol>
`
}

func resolveBuildTechStack(description string, requested *TechStack, appType string, bundle *autonomous.PlanningBundle) TechStack {
	var recommended *autonomous.TechStack
	if bundle != nil && bundle.Analysis != nil {
		recommended = bundle.Analysis.TechStack
	}

	stack := TechStack{}
	if recommended != nil {
		stack.Frontend = canonicalFrontendName(recommended.Frontend)
		stack.Backend = canonicalBackendName(recommended.Backend)
		stack.Database = canonicalDatabaseName(recommended.Database)
		stack.Styling = canonicalStylingName(recommended.Styling)
		stack.Extras = dedupeStrings(recommended.Extras)
	}

	if requested != nil {
		if strings.TrimSpace(requested.Frontend) != "" {
			stack.Frontend = canonicalFrontendName(requested.Frontend)
		}
		if strings.TrimSpace(requested.Backend) != "" {
			stack.Backend = canonicalBackendName(requested.Backend)
		}
		if strings.TrimSpace(requested.Database) != "" {
			stack.Database = canonicalDatabaseName(requested.Database)
		}
		if strings.TrimSpace(requested.Styling) != "" {
			stack.Styling = canonicalStylingName(requested.Styling)
		}
		stack.Extras = dedupeStrings(append(stack.Extras, requested.Extras...))
	}

	if appType == "web" && explicitStaticWebIntent(description) {
		// Explicit static/frontend-only intent must win over remembered or UI-
		// selected backend/database defaults. Otherwise the planner can leak
		// server-side work orders into a free static build even when the user
		// clearly said "no backend" / "no database".
		stack.Backend = ""
		stack.Database = ""
	}

	// Only apply cascade defaults when BOTH frontend and backend are empty.
	// The fallback must respect explicit frontend-only/static intent instead of
	// silently growing a backend contract.
	if stack.Frontend == "" && stack.Backend == "" {
		switch appType {
		case "web":
			stack.Frontend = "React"
		case "api":
			stack.Backend = "Express"
		default:
			stack.Frontend = "React"
			stack.Backend = "Express"
			stack.Database = "PostgreSQL"
		}
	}
	// Only default styling when there IS a frontend
	if stack.Styling == "" && stack.Frontend != "" {
		stack.Styling = "Tailwind"
	}
	// Only default database when there IS a backend AND user/planner didn't explicitly leave it blank
	// (the planner recommending a backend without a database is intentional for simple APIs)
	return stack
}

func resolveBuildAppType(description string, requested *TechStack, bundle *autonomous.PlanningBundle) string {
	if explicitStaticWebIntent(description) {
		return "web"
	}

	if bundle != nil && bundle.Analysis != nil {
		appType := strings.TrimSpace(strings.ToLower(bundle.Analysis.AppType))
		switch appType {
		case "web", "api", "cli", "fullstack":
			return appType
		case "frontend", "spa", "landing", "static", "dashboard":
			return "web"
		case "backend", "server", "microservice", "service", "rest", "graphql":
			return "api"
		case "full-stack", "full_stack", "webapp", "web-app", "saas":
			return "fullstack"
		}
	}

	if requested != nil {
		hasFrontend := strings.TrimSpace(requested.Frontend) != ""
		hasBackend := strings.TrimSpace(requested.Backend) != ""
		switch {
		case hasFrontend && hasBackend:
			return "fullstack"
		case hasBackend:
			return "api"
		case hasFrontend:
			return "web"
		}
	}

	switch inferIntentAppType(description, requested) {
	case "api":
		return "api"
	case "fullstack":
		return "fullstack"
	}
	return "fullstack"
}

func explicitStaticWebIntent(description string) bool {
	normalized := normalizeDetectionText(description)
	if normalized == "" {
		return false
	}

	if containsAnyAffirmedTerm(normalized, []string{
		normalizeDetectionText("frontend only"),
		normalizeDetectionText("static"),
		normalizeDetectionText("landing page"),
		normalizeDetectionText("marketing site"),
		normalizeDetectionText("marketing website"),
		normalizeDetectionText("brochure site"),
		normalizeDetectionText("single page"),
	}) {
		return true
	}

	for _, marker := range []string{" no backend ", " without backend ", " no database ", " without database "} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func convertPlannedFeatures(bundle *autonomous.PlanningBundle) []Feature {
	if bundle == nil || bundle.Analysis == nil {
		return nil
	}
	out := make([]Feature, 0, len(bundle.Analysis.Features))
	for _, feature := range bundle.Analysis.Features {
		out = append(out, Feature{
			ID:           uuid.New().String(),
			Name:         strings.TrimSpace(feature.Name),
			Description:  strings.TrimSpace(feature.Description),
			Priority:     priorityStringToInt(feature.Priority),
			Dependencies: dedupeStrings(feature.Dependencies),
		})
	}
	return out
}

func convertPlannedModels(bundle *autonomous.PlanningBundle) []DataModel {
	if bundle == nil || bundle.Analysis == nil {
		return nil
	}
	out := make([]DataModel, 0, len(bundle.Analysis.DataModels))
	for _, model := range bundle.Analysis.DataModels {
		fieldNames := make([]string, 0, len(model.Fields))
		for name := range model.Fields {
			fieldNames = append(fieldNames, name)
		}
		sort.Strings(fieldNames)
		fields := make([]ModelField, 0, len(fieldNames))
		for _, name := range fieldNames {
			fields = append(fields, ModelField{
				Name:     name,
				Type:     strings.TrimSpace(model.Fields[name]),
				Required: true,
			})
		}
		out = append(out, DataModel{
			Name:        strings.TrimSpace(model.Name),
			Description: "",
			Fields:      normalizeModelFields(fields),
		})
	}
	return out
}

func normalizeDataModels(models []DataModel) []DataModel {
	if len(models) == 0 {
		return nil
	}

	out := make([]DataModel, 0, len(models))
	for _, model := range models {
		name := strings.TrimSpace(model.Name)
		if name == "" {
			continue
		}

		out = append(out, DataModel{
			Name:        name,
			Description: strings.TrimSpace(model.Description),
			Fields:      normalizeModelFields(model.Fields),
			Relations:   append([]Relation(nil), model.Relations...),
		})
	}

	decorateForeignKeyReferences(out)
	return out
}

func decorateForeignKeyReferences(models []DataModel) {
	if len(models) == 0 {
		return
	}

	modelNames := make([]string, 0, len(models))
	for _, model := range models {
		if strings.TrimSpace(model.Name) != "" {
			modelNames = append(modelNames, strings.TrimSpace(model.Name))
		}
	}

	for mi := range models {
		model := &models[mi]
		relationTargets := make(map[string]string, len(model.Relations))
		for _, relation := range model.Relations {
			field := strings.TrimSpace(relation.Field)
			target := strings.TrimSpace(relation.Target)
			if field != "" && target != "" {
				relationTargets[strings.ToLower(field)] = target
			}
		}

		for fi := range model.Fields {
			field := &model.Fields[fi]
			if !strings.Contains(strings.ToLower(field.Type), "foreign key") ||
				strings.Contains(strings.ToLower(field.Type), "references") {
				continue
			}

			target := resolveForeignKeyTargetModel(field.Name, relationTargets[strings.ToLower(field.Name)], modelNames)
			if target == "" {
				continue
			}
			field.Type = strings.TrimSpace(field.Type) + " references " + target + "(id)"
		}
	}
}

func resolveForeignKeyTargetModel(fieldName string, relationTarget string, modelNames []string) string {
	if target := canonicalDataModelName(relationTarget, modelNames); target != "" {
		return target
	}
	if target := inferForeignKeyTargetModel(fieldName, modelNames); target != "" {
		return target
	}
	if target := inferForeignKeyTargetModel(relationTarget, modelNames); target != "" {
		return target
	}
	return ""
}

func inferForeignKeyTargetModel(fieldName string, modelNames []string) string {
	field := strings.TrimSpace(strings.ToLower(fieldName))
	if field == "" {
		return ""
	}

	candidates := make([]string, 0, 3)
	if strings.HasSuffix(field, "_id") {
		base := strings.TrimSuffix(field, "_id")
		if base != "" {
			candidates = append(candidates, base)
			if strings.HasSuffix(base, "s") {
				candidates = append(candidates, strings.TrimSuffix(base, "s"))
			} else {
				candidates = append(candidates, base+"s")
			}
		}
	}
	if strings.HasSuffix(field, "id") && !strings.HasSuffix(field, "_id") {
		base := strings.TrimSuffix(field, "id")
		base = strings.TrimSuffix(base, "_")
		if base != "" {
			candidates = append(candidates, base)
		}
	}

	for _, candidate := range dedupeStrings(candidates) {
		if target := canonicalDataModelName(candidate, modelNames); target != "" {
			return target
		}
		if looksLikeIdentityRoleCandidate(candidate) {
			if target := preferredIdentityModelName(modelNames); target != "" {
				return target
			}
		}
	}
	if looksLikeActorReferenceField(field) {
		if target := preferredIdentityModelName(modelNames); target != "" {
			return target
		}
	}
	return ""
}

func looksLikeActorReferenceField(field string) bool {
	field = strings.TrimSpace(strings.ToLower(field))
	if field == "" {
		return false
	}

	switch field {
	case "assigned_to", "assignee", "assignee_id", "owner", "owner_id", "manager", "manager_id", "creator_id", "author_id", "reviewer_id":
		return true
	}

	for _, suffix := range []string{
		"_by",
		"by",
	} {
		if strings.HasSuffix(field, suffix) {
			base := strings.TrimSuffix(field, suffix)
			base = strings.TrimSuffix(base, "_")
			switch base {
			case "created", "updated", "deleted", "recorded", "approved", "submitted", "requested", "modified", "assigned":
				return true
			}
		}
	}

	return false
}

func looksLikeIdentityRoleCandidate(candidate string) bool {
	switch strings.TrimSpace(strings.ToLower(candidate)) {
	case "user", "member", "agent", "admin", "profile", "manager", "assignee", "owner", "creator", "author", "reviewer", "editor", "approver", "operator":
		return true
	default:
		return false
	}
}

func preferredIdentityModelName(modelNames []string) string {
	for _, preferred := range []string{"User", "Member", "Agent", "Admin", "Profile"} {
		if target := canonicalDataModelName(preferred, modelNames); target != "" {
			return target
		}
	}
	return ""
}

func canonicalDataModelName(candidate string, modelNames []string) string {
	candidateNorm := normalizeDataModelIdentifier(candidate)
	if candidateNorm == "" {
		return ""
	}
	for _, modelName := range modelNames {
		if normalizeDataModelIdentifier(modelName) == candidateNorm {
			return modelName
		}
	}
	return ""
}

func normalizeDataModelIdentifier(name string) string {
	trimmed := strings.TrimSpace(strings.ToLower(name))
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizeModelFields(fields []ModelField) []ModelField {
	if len(fields) == 0 {
		return nil
	}

	out := make([]ModelField, 0, len(fields))
	for _, field := range fields {
		name := strings.TrimSpace(field.Name)
		fieldType := strings.TrimSpace(field.Type)
		if name == "" || fieldType == "" {
			continue
		}

		normalized := field
		normalized.Name = name
		normalizedType, qualifiers := normalizeModelFieldTypeDescriptor(fieldType)
		normalized.Type = normalizedType
		if qualifiers.requiredSet {
			normalized.Required = qualifiers.required
		}
		if qualifiers.unique {
			normalized.Unique = true
		}
		if isNullableModelFieldType(normalized.Type) {
			normalized.Required = false
		}
		if isCanonicalPrimaryKeyField(name) {
			normalized.Required = true
			normalized.Unique = true
		}
		out = append(out, normalized)
	}

	return out
}

type modelFieldQualifierFlags struct {
	requiredSet bool
	required    bool
	unique      bool
}

func normalizeModelFieldTypeDescriptor(fieldType string) (string, modelFieldQualifierFlags) {
	trimmed := strings.TrimSpace(fieldType)
	if trimmed == "" {
		return "", modelFieldQualifierFlags{}
	}

	flags := modelFieldQualifierFlags{}
	tokens := strings.Fields(trimmed)
	baseTokens := make([]string, 0, len(tokens))

	for i := 0; i < len(tokens); i++ {
		token := strings.TrimSpace(tokens[i])
		lower := strings.ToLower(strings.Trim(token, ","))
		switch lower {
		case "|", "&", ",":
			continue
		case "unique":
			flags.unique = true
			continue
		case "required":
			flags.requiredSet = true
			flags.required = true
			continue
		case "optional", "nullable":
			flags.requiredSet = true
			flags.required = false
			continue
		case "null":
			flags.requiredSet = true
			flags.required = false
			continue
		case "index", "indexed":
			continue
		case "not":
			if i+1 < len(tokens) && strings.EqualFold(strings.Trim(tokens[i+1], ","), "null") {
				flags.requiredSet = true
				flags.required = true
				i++
				continue
			}
		}
		baseTokens = append(baseTokens, token)
	}

	normalized := strings.TrimSpace(strings.Join(baseTokens, " "))
	if normalized == "" {
		normalized = trimmed
	}

	return normalized, flags
}

func isCanonicalPrimaryKeyField(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), "id")
}

func isNullableModelFieldType(fieldType string) bool {
	lower := strings.ToLower(strings.TrimSpace(fieldType))
	return strings.Contains(lower, "?") ||
		strings.Contains(lower, "null") ||
		strings.Contains(lower, "nullable") ||
		strings.Contains(lower, "*")
}

func convertPreflightChecks(bundle *autonomous.PlanningBundle) []BuildPreflightCheck {
	if bundle == nil || bundle.Analysis == nil {
		return nil
	}
	out := make([]BuildPreflightCheck, 0, len(bundle.Analysis.PreflightChecks))
	for _, check := range bundle.Analysis.PreflightChecks {
		out = append(out, BuildPreflightCheck{
			Name:        strings.TrimSpace(check.Name),
			Description: strings.TrimSpace(check.Description),
			Command:     strings.TrimSpace(check.Command),
			Required:    check.Required,
		})
	}
	return out
}

func planDerivedFiles(scaffold buildScaffold, bundle *autonomous.PlanningBundle) []PlannedFile {
	files := make([]PlannedFile, 0, 4)
	seen := make(map[string]bool)
	for _, file := range scaffold.Required {
		seen[file.Path] = true
	}

	if bundle == nil || bundle.Plan == nil {
		return files
	}

	for _, step := range bundle.Plan.Steps {
		if step == nil {
			continue
		}
		switch step.ActionType {
		case autonomous.ActionAIGenerate:
			if kind, _ := step.Input["type"].(string); kind != "" {
				switch kind {
				case "backend":
					routesPath := "src/routes/index.ts"
					if scaffold.ID == "fullstack/react-vite-express-ts" {
						routesPath = "server/routes/api.ts"
					}
					if !seen[routesPath] {
						files = append(files, PlannedFile{Path: routesPath, Type: "backend", Description: "API routes generated from build spec"})
						seen[routesPath] = true
					}
				case "frontend":
					if !seen["src/components/AppShell.tsx"] {
						files = append(files, PlannedFile{Path: "src/components/AppShell.tsx", Type: "frontend", Description: "Application shell for primary user flows"})
						seen["src/components/AppShell.tsx"] = true
					}
				case "data_models":
					if scaffold.AppType == "web" {
						continue
					}
					if !seen["migrations/001_initial.sql"] {
						files = append(files, PlannedFile{Path: "migrations/001_initial.sql", Type: "database", Description: "Initial schema derived from planned data models"})
						seen["migrations/001_initial.sql"] = true
					}
				}
			}
		}
	}

	return files
}

func mergePlannedFiles(base []PlannedFile, extras ...PlannedFile) []PlannedFile {
	out := make([]PlannedFile, 0, len(base)+len(extras))
	seen := make(map[string]bool)
	add := func(file PlannedFile) {
		path := strings.TrimSpace(file.Path)
		if path == "" || seen[path] {
			return
		}
		file.Path = path
		out = append(out, file)
		seen[path] = true
	}
	for _, file := range base {
		add(file)
	}
	for _, file := range extras {
		add(file)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out
}

func buildOwnershipMap(scaffold buildScaffold) []BuildOwnership {
	pathRoles := make(map[string][]AgentRole)
	for role, paths := range scaffold.Ownership {
		for _, path := range paths {
			path = strings.TrimSpace(path)
			if path == "" {
				continue
			}
			pathRoles[path] = append(pathRoles[path], role)
		}
	}

	paths := make([]string, 0, len(pathRoles))
	for path := range pathRoles {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	out := make([]BuildOwnership, 0, len(paths))
	for _, path := range paths {
		role := selectOwnershipRoleForPath(path, pathRoles[path])
		out = append(out, BuildOwnership{
			Path:    path,
			Role:    role,
			Purpose: fmt.Sprintf("%s owns %s within scaffold %s", role, path, scaffold.ID),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Role == out[j].Role {
			return out[i].Path < out[j].Path
		}
		return out[i].Role < out[j].Role
	})
	return out
}

func selectOwnershipRoleForPath(path string, roles []AgentRole) AgentRole {
	if len(roles) == 0 {
		return ""
	}
	if len(roles) == 1 {
		return roles[0]
	}

	seen := make(map[AgentRole]bool)
	uniqueRoles := make([]AgentRole, 0, len(roles))
	for _, role := range roles {
		if !seen[role] {
			seen[role] = true
			uniqueRoles = append(uniqueRoles, role)
		}
	}
	if len(uniqueRoles) == 1 {
		return uniqueRoles[0]
	}

	switch path {
	case "package.json", "tsconfig.json", ".env.example", "go.mod", "requirements.txt":
		if containsAgentRole(uniqueRoles, RoleBackend) {
			return RoleBackend
		}
	case "vite.config.ts", "tailwind.config.js", "postcss.config.js", "next.config.js", "index.html":
		if containsAgentRole(uniqueRoles, RoleFrontend) {
			return RoleFrontend
		}
	}

	preferredOrder := []AgentRole{
		RoleArchitect,
		RoleDatabase,
		RoleBackend,
		RoleFrontend,
		RoleTesting,
		RoleReviewer,
		RoleSolver,
	}
	for _, preferred := range preferredOrder {
		if containsAgentRole(uniqueRoles, preferred) {
			return preferred
		}
	}

	sort.Slice(uniqueRoles, func(i, j int) bool { return uniqueRoles[i] < uniqueRoles[j] })
	return uniqueRoles[0]
}

func containsAgentRole(roles []AgentRole, target AgentRole) bool {
	for _, role := range roles {
		if role == target {
			return true
		}
	}
	return false
}

func buildWorkOrders(appType string, stack TechStack, scaffold buildScaffold, ownership []BuildOwnership, acceptance []BuildAcceptanceCheck) []BuildWorkOrder {
	roles := []AgentRole{RoleArchitect, RoleFrontend, RoleDatabase, RoleBackend, RoleTesting, RoleReviewer, RoleSolver}
	out := make([]BuildWorkOrder, 0, len(roles))

	for _, role := range roles {
		owned := make([]string, 0)
		ownedSet := make(map[string]bool)
		forbidden := make([]string, 0)
		checks := make([]string, 0)
		requiredFiles := requiredScaffoldFilesForRole(role, scaffold.Required)
		for _, item := range ownership {
			if item.Role == role {
				owned = append(owned, item.Path)
				ownedSet[item.Path] = true
			}
		}
		for _, path := range requiredFiles {
			if sharedScaffoldFileAllowedForRole(path, role) && !ownedSet[path] {
				owned = append(owned, path)
				ownedSet[path] = true
			}
		}
		for _, item := range ownership {
			if item.Role != role {
				if item.Path == "**" {
					continue
				}
				if ownedSet[item.Path] {
					continue
				}
				forbidden = append(forbidden, item.Path)
			}
		}
		for _, check := range acceptance {
			if check.Owner == role {
				checks = append(checks, check.Description)
			}
		}
		order := BuildWorkOrder{
			Role:             role,
			Summary:          summarizeWorkOrder(role, appType, stack),
			OwnedFiles:       dedupeStrings(owned),
			RequiredFiles:    requiredFiles,
			ForbiddenFiles:   dedupeStrings(forbidden),
			AcceptanceChecks: dedupeStrings(checks),
			RequiredOutputs:  requiredOutputsForRole(role),
		}
		if role == RoleArchitect && len(order.OwnedFiles) == 0 {
			order.OwnedFiles = []string{"docs/**", "ARCHITECTURE.md"}
		}
		if role == RoleTesting &&
			appType == "web" &&
			strings.TrimSpace(stack.Backend) == "" &&
			strings.TrimSpace(stack.Database) == "" &&
			len(order.AcceptanceChecks) == 0 &&
			len(order.RequiredFiles) == 0 {
			// Static frontend builds already go through deterministic readiness and
			// preview validation. Spawning a dedicated testing specialist here adds
			// cost and another failure surface without increasing contract truth.
			continue
		}
		if role == RoleBackend && strings.TrimSpace(stack.Backend) == "" && len(order.AcceptanceChecks) == 0 {
			continue
		}
		if role == RoleDatabase && strings.TrimSpace(stack.Database) == "" && len(order.OwnedFiles) == 0 && len(order.RequiredFiles) == 0 && len(order.AcceptanceChecks) == 0 {
			continue
		}
		out = append(out, order)
	}

	return out
}

func sharedScaffoldFileAllowedForRole(path string, role AgentRole) bool {
	switch path {
	case "package.json", "tsconfig.json":
		return role == RoleFrontend || role == RoleBackend
	default:
		return false
	}
}

func requiredScaffoldFilesForRole(role AgentRole, required []PlannedFile) []string {
	if len(required) == 0 {
		return nil
	}
	out := make([]string, 0, len(required))
	for _, file := range required {
		switch role {
		case RoleArchitect:
			if file.Path == "README.md" || file.Path == "ARCHITECTURE.md" || strings.HasPrefix(file.Path, "docs/") {
				out = append(out, file.Path)
			}
		case RoleFrontend:
			if file.Type == "frontend" || file.Path == "package.json" || file.Path == "tsconfig.json" || file.Path == "vite.config.ts" || file.Path == "tailwind.config.js" || file.Path == "postcss.config.js" {
				out = append(out, file.Path)
			}
		case RoleBackend:
			if file.Type == "backend" || file.Path == ".env.example" || file.Path == "package.json" || file.Path == "tsconfig.json" || file.Path == "go.mod" {
				out = append(out, file.Path)
			}
		case RoleDatabase:
			if file.Type == "database" || strings.Contains(file.Path, "schema") || strings.HasPrefix(file.Path, "migrations/") {
				out = append(out, file.Path)
			}
		case RoleTesting:
			if strings.Contains(file.Path, "test") || strings.Contains(file.Path, "spec") {
				out = append(out, file.Path)
			}
		}
	}
	return dedupeStrings(out)
}

func deriveAcceptanceChecks(appType string, stack TechStack) []BuildAcceptanceCheck {
	checks := []BuildAcceptanceCheck{
		{
			ID:          "review-no-placeholders",
			Description: "Reviewer must reject placeholder text, TODOs, or demo-only responses",
			Owner:       RoleReviewer,
			Required:    true,
		},
		{
			ID:          "solver-fixes-verification",
			Description: "Solver must keep fixes within the frozen scaffold and restore build verification",
			Owner:       RoleSolver,
			Required:    true,
		},
	}

	if stack.Frontend != "" {
		checks = append(checks, BuildAcceptanceCheck{
			ID:          "frontend-entry",
			Description: "Frontend must deliver the mandatory scaffold entry files, respect the frozen UI contract, and connect to the agreed API base URL",
			Owner:       RoleFrontend,
			Required:    true,
		})
		checks = append(checks, BuildAcceptanceCheck{
			ID:          "frontend-preview",
			Description: "Frontend must compile and serve cleanly in the interactive preview path",
			Owner:       RoleFrontend,
			Required:    true,
		})
	}
	if stack.Backend != "" {
		checks = append(checks, BuildAcceptanceCheck{
			ID:          "backend-health",
			Description: "Backend must expose a health route and preserve the agreed port, CORS, and API contract promised to the frontend",
			Owner:       RoleBackend,
			Required:    true,
		})
	}
	if stack.Database != "" {
		checks = append(checks, BuildAcceptanceCheck{
			ID:          "database-schema",
			Description: "Database schema and seed/migration outputs must match the planned data models",
			Owner:       RoleDatabase,
			Required:    true,
		})
	}
	if appType == "fullstack" {
		checks = append(checks, BuildAcceptanceCheck{
			ID:          "architecture-contract-freeze",
			Description: "Architect must freeze the screen map, API contract, data expectations, and env contract before frontend and backend implementation begins",
			Owner:       RoleArchitect,
			Required:    true,
		})
		checks = append(checks, BuildAcceptanceCheck{
			ID:          "testing-vertical-slice",
			Description: "Testing must verify the main vertical slice across frontend and backend boundaries, including preview readiness",
			Owner:       RoleTesting,
			Required:    true,
		})
	}
	return checks
}

func selectBuildScaffold(appType string, stack TechStack) buildScaffold {
	frontend := strings.ToLower(canonicalFrontendName(stack.Frontend))
	backend := strings.ToLower(canonicalBackendName(stack.Backend))

	switch {
	case frontend == "react" && (backend == "express" || backend == "node"):
		return buildScaffold{
			ID:          "fullstack/react-vite-express-ts",
			AppType:     "fullstack",
			Description: "Single-repo React + Vite frontend with Express TypeScript backend",
			Required: []PlannedFile{
				{Path: ".env.example", Type: "config", Description: "Environment variable template"},
				{Path: "README.md", Type: "docs", Description: "Run instructions and project overview"},
				{Path: "index.html", Type: "frontend", Description: "Vite HTML entry point"},
				{Path: "package.json", Type: "config", Description: "Single-repo dependency manifest and scripts"},
				{Path: "postcss.config.js", Type: "config", Description: "PostCSS config for Tailwind"},
				{Path: "server/index.ts", Type: "backend", Description: "Express entry point"},
				{Path: "server/routes/api.ts", Type: "backend", Description: "Primary API routes"},
				{Path: "src/App.tsx", Type: "frontend", Description: "Root React application"},
				{Path: "src/index.css", Type: "frontend", Description: "Global styles"},
				{Path: "src/main.tsx", Type: "frontend", Description: "React entry point"},
				{Path: "tailwind.config.js", Type: "config", Description: "Tailwind configuration"},
				{Path: "tsconfig.json", Type: "config", Description: "Shared TypeScript config"},
				{Path: "vite.config.ts", Type: "frontend", Description: "Vite config with proxy/API wiring"},
			},
			Ownership: map[AgentRole][]string{
				RoleArchitect: {"README.md", "ARCHITECTURE.md", "docs/**"},
				RoleFrontend:  {"package.json", "tsconfig.json", "vite.config.ts", "tailwind.config.js", "postcss.config.js", "index.html", "src/**", "public/**"},
				RoleBackend:   {"package.json", "tsconfig.json", ".env.example", "server/**"},
				RoleDatabase:  {"migrations/**", "db/**", "prisma/**", "schema.sql", "server/db/**", "server/migrate.ts", "server/seed.ts"},
				RoleTesting:   {"tests/**", "**/*.test.ts", "**/*.test.tsx", "**/*.spec.ts", "**/*.spec.tsx"},
				RoleReviewer:  {"**"},
				RoleSolver:    {"**"},
			},
			EnvVars: []BuildEnvVar{
				{Name: "PORT", Example: "3001", Purpose: "Backend listen port", Required: true},
				{Name: "VITE_API_URL", Example: "http://localhost:3001", Purpose: "Frontend API base URL", Required: false},
				{Name: "DATABASE_URL", Example: "postgresql://postgres:postgres@localhost:5432/app", Purpose: "Backend database connection", Required: backend != ""},
			},
			Acceptance: []BuildAcceptanceCheck{
				{ID: "fullstack-root-manifest", Description: "Scaffold root manifest must include runnable dev/build scripts for frontend and backend", Owner: RoleBackend, Required: true},
				{ID: "fullstack-ui-shell", Description: "Frontend must render a usable shell from src/main.tsx and src/App.tsx", Owner: RoleFrontend, Required: true},
			},
			APIContract: &BuildAPIContract{
				FrontendPort: 5173,
				BackendPort:  3001,
				APIBaseURL:   "http://localhost:3001",
				CORSOrigins:  []string{"http://localhost:5173", "http://localhost:3000"},
				Endpoints: []APIEndpoint{
					{Method: "GET", Path: "/api/health", Description: "Health check", Output: "{ status: \"ok\" }"},
				},
			},
		}
	case frontend == "react" && backend == "go":
		return buildScaffold{
			ID:          "fullstack/react-vite-go",
			AppType:     "fullstack",
			Description: "Single-repo React + Vite frontend with Go net/http backend",
			Required: []PlannedFile{
				{Path: ".env.example", Type: "config", Description: "Environment variable template"},
				{Path: "README.md", Type: "docs", Description: "Run instructions and project overview"},
				{Path: "go.mod", Type: "config", Description: "Go module definition"},
				{Path: "index.html", Type: "frontend", Description: "Vite HTML entry point"},
				{Path: "main.go", Type: "backend", Description: "Go HTTP entry point"},
				{Path: "package.json", Type: "config", Description: "Frontend dependency manifest"},
				{Path: "postcss.config.js", Type: "config", Description: "PostCSS config for Tailwind"},
				{Path: "src/App.tsx", Type: "frontend", Description: "Root React application"},
				{Path: "src/index.css", Type: "frontend", Description: "Global styles"},
				{Path: "src/main.tsx", Type: "frontend", Description: "React entry point"},
				{Path: "tailwind.config.js", Type: "config", Description: "Tailwind configuration"},
				{Path: "tsconfig.json", Type: "config", Description: "TypeScript config"},
				{Path: "vite.config.ts", Type: "frontend", Description: "Vite config with API proxy"},
			},
			Ownership: map[AgentRole][]string{
				RoleArchitect: {"README.md", "ARCHITECTURE.md", "docs/**"},
				RoleFrontend:  {"package.json", "tsconfig.json", "vite.config.ts", "tailwind.config.js", "postcss.config.js", "index.html", "src/**", "public/**"},
				RoleBackend:   {"go.mod", "main.go", "cmd/**", "internal/**", "pkg/**", "handlers/**", "middleware/**", ".env.example"},
				RoleDatabase:  {"migrations/**", "db/**", "internal/db/**", "schema.sql"},
				RoleTesting:   {"tests/**", "**/*.test.ts", "**/*.test.tsx", "**/*_test.go"},
				RoleReviewer:  {"**"},
				RoleSolver:    {"**"},
			},
			EnvVars: []BuildEnvVar{
				{Name: "PORT", Example: "8080", Purpose: "Backend listen port", Required: true},
				{Name: "VITE_API_URL", Example: "http://localhost:8080", Purpose: "Frontend API base URL", Required: false},
				{Name: "DATABASE_URL", Example: "postgresql://postgres:postgres@localhost:5432/app", Purpose: "Backend database connection", Required: stack.Database != ""},
			},
			Acceptance: []BuildAcceptanceCheck{
				{ID: "fullstack-go-backend", Description: "Go backend must compile and expose a health route on PORT", Owner: RoleBackend, Required: true},
				{ID: "fullstack-react-frontend", Description: "Frontend must render a usable shell from src/main.tsx and src/App.tsx", Owner: RoleFrontend, Required: true},
			},
			APIContract: &BuildAPIContract{
				FrontendPort: 5173,
				BackendPort:  8080,
				APIBaseURL:   "http://localhost:8080",
				CORSOrigins:  []string{"http://localhost:5173", "http://localhost:3000"},
				Endpoints: []APIEndpoint{
					{Method: "GET", Path: "/health", Description: "Health check", Output: "{ status: \"ok\" }"},
				},
			},
		}
	case frontend == "react" && (backend == "python" || backend == "fastapi"):
		return buildScaffold{
			ID:          "fullstack/react-vite-fastapi",
			AppType:     "fullstack",
			Description: "Single-repo React + Vite frontend with Python FastAPI backend",
			Required: []PlannedFile{
				{Path: ".env.example", Type: "config", Description: "Environment variable template"},
				{Path: "README.md", Type: "docs", Description: "Run instructions and project overview"},
				{Path: "index.html", Type: "frontend", Description: "Vite HTML entry point"},
				{Path: "main.py", Type: "backend", Description: "FastAPI entry point"},
				{Path: "package.json", Type: "config", Description: "Frontend dependency manifest"},
				{Path: "postcss.config.js", Type: "config", Description: "PostCSS config for Tailwind"},
				{Path: "requirements.txt", Type: "config", Description: "Python dependencies"},
				{Path: "src/App.tsx", Type: "frontend", Description: "Root React application"},
				{Path: "src/index.css", Type: "frontend", Description: "Global styles"},
				{Path: "src/main.tsx", Type: "frontend", Description: "React entry point"},
				{Path: "tailwind.config.js", Type: "config", Description: "Tailwind configuration"},
				{Path: "tsconfig.json", Type: "config", Description: "TypeScript config"},
				{Path: "vite.config.ts", Type: "frontend", Description: "Vite config with API proxy"},
			},
			Ownership: map[AgentRole][]string{
				RoleArchitect: {"README.md", "ARCHITECTURE.md", "docs/**"},
				RoleFrontend:  {"package.json", "tsconfig.json", "vite.config.ts", "tailwind.config.js", "postcss.config.js", "index.html", "src/**", "public/**"},
				RoleBackend:   {"requirements.txt", "main.py", "routers/**", "models/**", "services/**", "middleware/**", ".env.example"},
				RoleDatabase:  {"migrations/**", "db/**", "alembic/**", "schema.sql"},
				RoleTesting:   {"tests/**", "**/*.test.ts", "**/*.test.tsx", "**/*_test.py", "**/*.spec.ts"},
				RoleReviewer:  {"**"},
				RoleSolver:    {"**"},
			},
			EnvVars: []BuildEnvVar{
				{Name: "PORT", Example: "8000", Purpose: "Backend listen port", Required: true},
				{Name: "VITE_API_URL", Example: "http://localhost:8000", Purpose: "Frontend API base URL", Required: false},
				{Name: "DATABASE_URL", Example: "postgresql://postgres:postgres@localhost:5432/app", Purpose: "Backend database connection", Required: stack.Database != ""},
			},
			Acceptance: []BuildAcceptanceCheck{
				{ID: "fullstack-fastapi-backend", Description: "FastAPI backend must start and expose a health route on PORT", Owner: RoleBackend, Required: true},
				{ID: "fullstack-react-frontend", Description: "Frontend must render a usable shell from src/main.tsx and src/App.tsx", Owner: RoleFrontend, Required: true},
			},
			APIContract: &BuildAPIContract{
				FrontendPort: 5173,
				BackendPort:  8000,
				APIBaseURL:   "http://localhost:8000",
				CORSOrigins:  []string{"http://localhost:5173", "http://localhost:3000"},
				Endpoints: []APIEndpoint{
					{Method: "GET", Path: "/health", Description: "Health check", Output: "{ status: \"ok\" }"},
				},
			},
		}
	case frontend == "next" || frontend == "next.js" || frontend == "nextjs":
		nextBackend := backend
		nextScaffoldID := "frontend/nextjs-app"
		nextAppType := "web"
		if nextBackend != "" {
			nextScaffoldID = "fullstack/nextjs-api"
			nextAppType = "fullstack"
		}
		required := []PlannedFile{
			{Path: "README.md", Type: "docs", Description: "Run instructions and project overview"},
			{Path: "app/layout.tsx", Type: "frontend", Description: "Root layout component"},
			{Path: "app/page.tsx", Type: "frontend", Description: "Home page"},
			{Path: "next.config.js", Type: "config", Description: "Next.js configuration"},
			{Path: "package.json", Type: "config", Description: "Dependency manifest"},
			{Path: "tailwind.config.js", Type: "config", Description: "Tailwind configuration"},
			{Path: "tsconfig.json", Type: "config", Description: "TypeScript config"},
		}
		if nextBackend != "" {
			required = append(required,
				PlannedFile{Path: "app/api/health/route.ts", Type: "backend", Description: "Health check API route"},
			)
		}
		acceptance := []BuildAcceptanceCheck{
			{ID: "nextjs-app-shell", Description: "Next.js app must have a working layout.tsx and page.tsx", Owner: RoleFrontend, Required: true},
		}
		var apiContract *BuildAPIContract
		if nextBackend != "" {
			acceptance = append(acceptance,
				BuildAcceptanceCheck{ID: "nextjs-api-health", Description: "Next.js API routes must expose a working health endpoint from app/api/health/route.ts", Owner: RoleBackend, Required: true},
				BuildAcceptanceCheck{ID: "nextjs-fullstack-integration", Description: "Testing must verify Next.js pages and app/api routes agree on the shared /api contract", Owner: RoleTesting, Required: true},
			)
			apiContract = &BuildAPIContract{
				FrontendPort: 3000,
				BackendPort:  3000,
				APIBaseURL:   "/api",
				CORSOrigins:  []string{"http://localhost:3000"},
				Endpoints: []APIEndpoint{
					{Method: "GET", Path: "/api/health", Description: "Health check", Output: "{ status: \"ok\" }"},
				},
			}
		}
		ownership := map[AgentRole][]string{
			RoleArchitect: {"README.md", "ARCHITECTURE.md", "docs/**"},
			RoleFrontend:  {"package.json", "tsconfig.json", "next.config.js", "tailwind.config.js", "postcss.config.js", "app/**", "components/**", "lib/**", "public/**", "styles/**"},
			RoleTesting:   {"tests/**", "**/*.test.ts", "**/*.test.tsx", "**/*.spec.ts"},
			RoleReviewer:  {"**"},
			RoleSolver:    {"**"},
		}
		envVars := []BuildEnvVar(nil)
		if nextBackend != "" {
			ownership[RoleBackend] = []string{"package.json", "app/api/**", "lib/db/**", ".env.example"}
			ownership[RoleDatabase] = []string{"migrations/**", "db/**", "prisma/**", "lib/db/**", "schema.sql"}
			envVars = append(envVars, BuildEnvVar{
				Name:     "DATABASE_URL",
				Example:  "postgresql://postgres:postgres@localhost:5432/app",
				Purpose:  "Database connection",
				Required: stack.Database != "",
			})
		}
		return buildScaffold{
			ID:          nextScaffoldID,
			AppType:     nextAppType,
			Description: "Next.js App Router scaffold",
			Required:    required,
			Ownership:   ownership,
			EnvVars:     envVars,
			Acceptance:  acceptance,
			APIContract: apiContract,
		}
	case frontend == "react":
		return buildScaffold{
			ID:          "frontend/react-vite-spa",
			AppType:     "web",
			Description: "React + Vite single-page app scaffold",
			Required: []PlannedFile{
				{Path: "index.html", Type: "frontend", Description: "Vite HTML entry point"},
				{Path: "package.json", Type: "config", Description: "Frontend dependency manifest"},
				{Path: "postcss.config.js", Type: "config", Description: "PostCSS config for Tailwind"},
				{Path: "README.md", Type: "docs", Description: "Run instructions and project overview"},
				{Path: "src/App.tsx", Type: "frontend", Description: "Root React application"},
				{Path: "src/index.css", Type: "frontend", Description: "Global styles"},
				{Path: "src/main.tsx", Type: "frontend", Description: "React entry point"},
				{Path: "tailwind.config.js", Type: "config", Description: "Tailwind configuration"},
				{Path: "tsconfig.json", Type: "config", Description: "TypeScript config"},
				{Path: "vite.config.ts", Type: "frontend", Description: "Vite config"},
			},
			Ownership: map[AgentRole][]string{
				RoleArchitect: {"README.md", "ARCHITECTURE.md", "docs/**"},
				RoleFrontend:  {"package.json", "tsconfig.json", "vite.config.ts", "tailwind.config.js", "postcss.config.js", "index.html", "src/**", "public/**"},
				RoleTesting:   {"tests/**", "**/*.test.ts", "**/*.test.tsx", "**/*.spec.ts", "**/*.spec.tsx"},
				RoleReviewer:  {"**"},
				RoleSolver:    {"**"},
			},
			Acceptance: []BuildAcceptanceCheck{
				{ID: "spa-entry", Description: "Frontend must include Vite entry files and a complete app shell", Owner: RoleFrontend, Required: true},
			},
		}
	case backend == "python" || backend == "fastapi":
		return buildScaffold{
			ID:          "api/python-fastapi",
			AppType:     "api",
			Description: "Python FastAPI API scaffold",
			Required: []PlannedFile{
				{Path: ".env.example", Type: "config", Description: "Environment variable template"},
				{Path: "main.py", Type: "backend", Description: "FastAPI entry point"},
				{Path: "README.md", Type: "docs", Description: "Run instructions and project overview"},
				{Path: "requirements.txt", Type: "config", Description: "Python dependencies"},
			},
			Ownership: map[AgentRole][]string{
				RoleArchitect: {"README.md", "ARCHITECTURE.md", "docs/**"},
				RoleBackend:   {"requirements.txt", "main.py", "routers/**", "models/**", "services/**", "middleware/**", ".env.example"},
				RoleDatabase:  {"migrations/**", "db/**", "alembic/**", "schema.sql"},
				RoleTesting:   {"tests/**", "**/*_test.py", "**/*.test.py"},
				RoleReviewer:  {"**"},
				RoleSolver:    {"**"},
			},
			EnvVars: []BuildEnvVar{
				{Name: "PORT", Example: "8000", Purpose: "API listen port", Required: true},
				{Name: "DATABASE_URL", Example: "postgresql://postgres:postgres@localhost:5432/app", Purpose: "Database connection", Required: stack.Database != ""},
			},
			Acceptance: []BuildAcceptanceCheck{
				{ID: "fastapi-entry", Description: "FastAPI must start and expose a health route", Owner: RoleBackend, Required: true},
			},
			APIContract: &BuildAPIContract{
				BackendPort: 8000,
				APIBaseURL:  "http://localhost:8000",
				Endpoints: []APIEndpoint{
					{Method: "GET", Path: "/health", Description: "Health check", Output: "{ status: \"ok\" }"},
				},
			},
		}
	case backend == "go":
		return buildScaffold{
			ID:          "api/go-http",
			AppType:     "api",
			Description: "Go HTTP API scaffold",
			Required: []PlannedFile{
				{Path: "go.mod", Type: "config", Description: "Go module definition"},
				{Path: "main.go", Type: "backend", Description: "Go HTTP entry point"},
				{Path: ".env.example", Type: "config", Description: "Environment variable template"},
				{Path: "README.md", Type: "docs", Description: "Run instructions and project overview"},
			},
			Ownership: map[AgentRole][]string{
				RoleArchitect: {"README.md", "ARCHITECTURE.md", "docs/**"},
				RoleBackend:   {"go.mod", "main.go", "cmd/**", "internal/**", "pkg/**", ".env.example"},
				RoleDatabase:  {"migrations/**", "db/**", "internal/db/**", "schema.sql"},
				RoleTesting:   {"**/*_test.go"},
				RoleReviewer:  {"**"},
				RoleSolver:    {"**"},
			},
			EnvVars: []BuildEnvVar{
				{Name: "PORT", Example: "8080", Purpose: "API listen port", Required: true},
				{Name: "DATABASE_URL", Example: "postgresql://postgres:postgres@localhost:5432/app", Purpose: "Database connection", Required: stack.Database != ""},
			},
			Acceptance: []BuildAcceptanceCheck{
				{ID: "go-entry", Description: "Backend must compile with go build ./... and expose a health route", Owner: RoleBackend, Required: true},
			},
			APIContract: &BuildAPIContract{
				BackendPort: 8080,
				APIBaseURL:  "http://localhost:8080",
				Endpoints: []APIEndpoint{
					{Method: "GET", Path: "/health", Description: "Health check", Output: "{ status: \"ok\" }"},
				},
			},
		}
	default:
		return buildScaffold{
			ID:          "api/express-typescript",
			AppType:     "api",
			Description: "Express + TypeScript API scaffold",
			Required: []PlannedFile{
				{Path: ".env.example", Type: "config", Description: "Environment variable template"},
				{Path: "package.json", Type: "config", Description: "API dependency manifest"},
				{Path: "README.md", Type: "docs", Description: "Run instructions and project overview"},
				{Path: "src/server.ts", Type: "backend", Description: "Express entry point"},
				{Path: "src/routes/index.ts", Type: "backend", Description: "Primary API routes"},
				{Path: "tsconfig.json", Type: "config", Description: "TypeScript config"},
			},
			Ownership: map[AgentRole][]string{
				RoleArchitect: {"README.md", "ARCHITECTURE.md", "docs/**"},
				RoleBackend:   {"package.json", "tsconfig.json", ".env.example", "src/**"},
				RoleDatabase:  {"migrations/**", "db/**", "src/db/**", "schema.sql"},
				RoleTesting:   {"tests/**", "**/*.test.ts", "**/*.spec.ts"},
				RoleReviewer:  {"**"},
				RoleSolver:    {"**"},
			},
			EnvVars: []BuildEnvVar{
				{Name: "PORT", Example: "3001", Purpose: "API listen port", Required: true},
				{Name: "DATABASE_URL", Example: "postgresql://postgres:postgres@localhost:5432/app", Purpose: "Database connection", Required: stack.Database != ""},
			},
			Acceptance: []BuildAcceptanceCheck{
				{ID: "express-entry", Description: "Backend must include package.json, tsconfig.json, and a runnable Express entrypoint", Owner: RoleBackend, Required: true},
			},
			APIContract: &BuildAPIContract{
				BackendPort: 3001,
				APIBaseURL:  "http://localhost:3001",
				Endpoints: []APIEndpoint{
					{Method: "GET", Path: "/api/health", Description: "Health check", Output: "{ status: \"ok\" }"},
				},
			},
		}
	}
}

func scaffoldBootstrapFiles(scaffold buildScaffold, description string, stack TechStack) []GeneratedFile {
	displayName, packageName := deriveScaffoldNames(description, scaffold.AppType)
	filesByPath := make(map[string]GeneratedFile)
	add := func(path string, content string) {
		path = strings.TrimSpace(path)
		content = strings.TrimSpace(content)
		if path == "" || content == "" {
			return
		}
		filesByPath[path] = GeneratedFile{
			Path:     path,
			Content:  content + "\n",
			Language: scaffoldFileLanguage(path),
			Size:     int64(len(content) + 1),
			IsNew:    true,
		}
	}

	switch scaffold.ID {
	case "fullstack/react-vite-express-ts":
		add(".env.example", "PORT=3001\nVITE_API_URL=http://localhost:3001\nDATABASE_URL=postgresql://postgres:postgres@localhost:5432/app")
		add("README.md", fmt.Sprintf("# %s\n\nAPEX.BUILD bootstrapped this project with a deterministic React + Vite + Express scaffold.\n\n## Run\n\n1. Install dependencies with `npm install`\n2. Start the frontend and backend with `npm run dev`\n3. Open `http://localhost:5173`\n\n## Environment\n\nCopy `.env.example` and provide real values before production use.\n", displayName))
		add("package.json", fmt.Sprintf(`{
  "name": "%s",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "concurrently \"npm run dev:client\" \"npm run dev:server\"",
    "dev:client": "vite",
    "dev:server": "tsx watch server/index.ts",
    "build": "npm run build:client && npm run build:server",
    "build:client": "vite build",
    "build:server": "tsc -p tsconfig.json",
    "start": "node dist/server/index.js"
  },
  "dependencies": {
    "cors": "^2.8.5",
    "express": "^4.21.2",
    "react": "^18.3.1",
    "react-dom": "^18.3.1"
  },
  "devDependencies": {
    "@types/cors": "^2.8.17",
    "@types/express": "^5.0.1",
    "@types/node": "^22.13.10",
    "@types/react": "^18.3.12",
    "@types/react-dom": "^18.3.1",
    "@vitejs/plugin-react": "^4.3.4",
    "autoprefixer": "^10.4.20",
    "concurrently": "^9.1.2",
    "postcss": "^8.5.3",
    "tailwindcss": "^3.4.17",
    "tsx": "^4.19.3",
    "typescript": "^5.8.2",
    "vite": "^6.2.1"
  }
}`, packageName))
		add("tsconfig.json", `{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "allowJs": false,
    "skipLibCheck": true,
    "esModuleInterop": true,
    "allowSyntheticDefaultImports": true,
    "strict": true,
    "forceConsistentCasingInFileNames": true,
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": false,
    "jsx": "react-jsx",
    "outDir": "dist"
  },
  "include": ["src", "server"]
}`)
		add("vite.config.ts", `import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    host: "0.0.0.0",
    port: 5173,
    proxy: {
      "/api": {
        target: process.env.VITE_API_URL || process.env.VITE_API_BASE_URL || "http://localhost:3001",
        changeOrigin: true
      }
    }
  }
});`)
		add("tailwind.config.js", `/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {}
  },
  plugins: []
};`)
		add("postcss.config.js", `export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {}
  }
};`)
		add("index.html", fmt.Sprintf(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>%s</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>`, displayName))
		add("src/main.tsx", `import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);`)
		add("src/vite-env.d.ts", `/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly [key: string]: string | boolean | undefined
  readonly VITE_API_URL?: string
  readonly VITE_API_BASE_URL?: string
  readonly VITE_WS_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}`)
		add("src/App.tsx", fmt.Sprintf(`import { useEffect, useState } from "react";

type HealthState = {
  status: string;
  app?: string;
};

export default function App() {
  const [health, setHealth] = useState<HealthState | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    fetch("/api/health", { signal: controller.signal })
      .then(async (response) => {
        if (!response.ok) {
          throw new Error("Health check failed");
        }
        return response.json();
      })
      .then((data: HealthState) => setHealth(data))
      .catch((err: Error) => setError(err.message));
    return () => controller.abort();
  }, []);

  return (
    <main className="app-shell">
      <section className="hero">
        <p className="eyebrow">Bootstrapped by APEX.BUILD</p>
        <h1>%s</h1>
        <p className="lede">
          The deterministic scaffold is live. Replace this shell with the product-specific experience.
        </p>
      </section>

      <section className="status-card">
        <h2>Runtime Check</h2>
        {health ? (
          <p>Backend status: {health.status}</p>
        ) : error ? (
          <p>Backend status: {error}</p>
        ) : (
          <p>Checking backend connection...</p>
        )}
      </section>
    </main>
  );
}`, displayName))
		add("src/index.css", `@tailwind base;
@tailwind components;
@tailwind utilities;

:root {
  color: #f3f4f6;
  background: radial-gradient(circle at top, #1f2937, #020617 60%);
  font-family: "Inter", ui-sans-serif, system-ui, sans-serif;
}

body {
  margin: 0;
  min-height: 100vh;
}

#root {
  min-height: 100vh;
}

.app-shell {
  min-height: 100vh;
  padding: 4rem 1.5rem;
  display: grid;
  gap: 1.5rem;
  align-content: center;
  max-width: 72rem;
  margin: 0 auto;
}

.hero, .status-card {
  background: rgba(15, 23, 42, 0.78);
  border: 1px solid rgba(148, 163, 184, 0.24);
  border-radius: 1.25rem;
  padding: 1.5rem;
  backdrop-filter: blur(12px);
}

.eyebrow {
  text-transform: uppercase;
  letter-spacing: 0.18em;
  font-size: 0.75rem;
  color: #93c5fd;
  margin: 0 0 0.75rem;
}

.hero h1 {
  font-size: clamp(2.5rem, 6vw, 4.5rem);
  margin: 0;
}

.lede {
  color: #cbd5e1;
  max-width: 48rem;
}`)
		add("server/index.ts", fmt.Sprintf(`import cors from "cors";
import express from "express";
import apiRouter from "./routes/api";

const app = express();
const port = Number(process.env.PORT || 3001);

app.use(
  cors({
    origin: ["http://localhost:5173", "http://localhost:3000"],
    credentials: true
  })
);
app.use(express.json());
app.use("/api", apiRouter);

app.listen(port, "0.0.0.0", () => {
  console.log("%s backend listening on port " + port);
});`, packageName))
		add("server/routes/api.ts", fmt.Sprintf(`import { Router } from "express";

const router = Router();

router.get("/health", (_req, res) => {
  res.json({ status: "ok", app: "%s" });
});

export default router;`, displayName))
	case "frontend/react-vite-spa":
		add("README.md", fmt.Sprintf("# %s\n\nAPEX.BUILD bootstrapped this project with a deterministic React + Vite scaffold.\n\n## Run\n\n1. Install dependencies with `npm install`\n2. Start the app with `npm run dev`\n3. Open `http://localhost:5173`\n", displayName))
		add("package.json", fmt.Sprintf(`{
  "name": "%s",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1"
  },
  "devDependencies": {
    "@types/react": "^18.3.12",
    "@types/react-dom": "^18.3.1",
    "@vitejs/plugin-react": "^4.3.4",
    "autoprefixer": "^10.4.20",
    "postcss": "^8.5.3",
    "tailwindcss": "^3.4.17",
    "typescript": "^5.8.2",
    "vite": "^6.2.1"
  }
}`, packageName))
		add("tsconfig.json", `{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "strict": true,
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "jsx": "react-jsx",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true
  },
  "include": ["src"]
}`)
		add("vite.config.ts", `import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    host: "0.0.0.0",
    port: 5173
  }
});`)
		add("tailwind.config.js", `/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: { extend: {} },
  plugins: []
};`)
		add("postcss.config.js", `export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {}
  }
};`)
		add("index.html", fmt.Sprintf(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>%s</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>`, displayName))
		add("src/main.tsx", `import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);`)
		add("src/vite-env.d.ts", `/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly [key: string]: string | boolean | undefined
  readonly VITE_API_URL?: string
  readonly VITE_API_BASE_URL?: string
  readonly VITE_WS_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}`)
		add("src/App.tsx", fmt.Sprintf(`export default function App() {
  return (
    <main className="app-shell">
      <p className="eyebrow">Bootstrapped by APEX.BUILD</p>
      <h1>%s</h1>
      <p className="lede">
        The deterministic scaffold is live. Replace this shell with the real experience.
      </p>
    </main>
  );
}`, displayName))
		add("src/index.css", `@tailwind base;
@tailwind components;
@tailwind utilities;

body {
  margin: 0;
  min-height: 100vh;
  background: linear-gradient(180deg, #0f172a, #020617);
  color: #f8fafc;
  font-family: "Inter", ui-sans-serif, system-ui, sans-serif;
}

.app-shell {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 3rem 1.5rem;
  text-align: center;
}

.eyebrow {
  text-transform: uppercase;
  letter-spacing: 0.2em;
  color: #38bdf8;
}

.lede {
  max-width: 40rem;
  color: #cbd5e1;
}`)
	case "api/go-http":
		add(".env.example", "PORT=8080\nDATABASE_URL=postgresql://postgres:postgres@localhost:5432/app")
		add("README.md", fmt.Sprintf("# %s\n\nAPEX.BUILD bootstrapped this Go API scaffold.\n\n## Run\n\n1. Copy `.env.example`\n2. Run `go run .`\n3. Open `http://localhost:8080/health`\n", displayName))
		add("go.mod", fmt.Sprintf("module %s\n\ngo 1.26.0\n", strings.ReplaceAll(packageName, "-", "")))
		add("main.go", fmt.Sprintf(`package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"app":    "%s",
		})
	})

	log.Printf("%s API listening on :%%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}`, displayName, displayName))
	case "fullstack/react-vite-go":
		add(".env.example", "PORT=8080\nVITE_API_URL=http://localhost:8080\nDATABASE_URL=postgresql://postgres:postgres@localhost:5432/app")
		add("README.md", fmt.Sprintf("# %s\n\nAPEX.BUILD bootstrapped this React + Vite + Go scaffold.\n\n## Run\n\n1. Install frontend deps: `npm install`\n2. Start Go backend: `go run .`\n3. Start frontend: `npm run dev`\n4. Open `http://localhost:5173`\n", displayName))
		add("go.mod", fmt.Sprintf("module %s\n\ngo 1.26.0\n", strings.ReplaceAll(packageName, "-", "")))
		add("main.go", fmt.Sprintf(`package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "app": "%s"})
	})

	log.Printf("%s API listening on :%%s", port)
	if err := http.ListenAndServe(":"+port, corsMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}`, displayName, displayName))
		add("package.json", fmt.Sprintf(`{
  "name": "%s",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": "^19.1.0",
    "react-dom": "^19.1.0"
  },
  "devDependencies": {
    "@types/react": "^19.1.2",
    "@types/react-dom": "^19.1.2",
    "@vitejs/plugin-react": "^4.4.1",
    "autoprefixer": "^10.4.21",
    "postcss": "^8.5.3",
    "tailwindcss": "^3.4.17",
    "typescript": "^5.8.2",
    "vite": "^6.3.2"
  }
}`, packageName))
		add("tsconfig.json", `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "jsx": "react-jsx",
    "strict": true,
    "noEmit": true,
    "resolveJsonModule": true,
    "esModuleInterop": true
  },
  "include": ["src"]
}`)
		add("vite.config.ts", `import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": { target: "http://localhost:8080", changeOrigin: true },
      "/health": { target: "http://localhost:8080", changeOrigin: true },
    },
  },
});`)
		add("tailwind.config.js", `/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: { extend: {} },
  plugins: [],
};`)
		add("postcss.config.js", `export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
};`)
		add("index.html", fmt.Sprintf(`<!doctype html>
<html lang="en">
  <head><meta charset="UTF-8" /><meta name="viewport" content="width=device-width, initial-scale=1.0" /><title>%s</title></head>
  <body><div id="root"></div><script type="module" src="/src/main.tsx"></script></body>
</html>`, displayName))
		add("src/main.tsx", `import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode><App /></React.StrictMode>
);`)
		add("src/vite-env.d.ts", `/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly [key: string]: string | boolean | undefined
  readonly VITE_API_URL?: string
  readonly VITE_API_BASE_URL?: string
  readonly VITE_WS_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}`)
		add("src/App.tsx", fmt.Sprintf(`import { useEffect, useState } from "react";

export default function App() {
  const [status, setStatus] = useState<string>("loading...");

  useEffect(() => {
    fetch("/health")
      .then((r) => r.json())
      .then((d) => setStatus(d.status))
      .catch(() => setStatus("offline"));
  }, []);

  return (
    <div className="min-h-screen bg-slate-900 text-white flex items-center justify-center">
      <div className="text-center space-y-4">
        <h1 className="text-4xl font-bold">%s</h1>
        <p className="text-slate-400">Go API status: <span className="text-sky-400 font-mono">{status}</span></p>
      </div>
    </div>
  );
}`, displayName))
		add("src/index.css", `@tailwind base;
@tailwind components;
@tailwind utilities;`)

	case "fullstack/react-vite-fastapi":
		add(".env.example", "PORT=8000\nVITE_API_URL=http://localhost:8000\nDATABASE_URL=postgresql://postgres:postgres@localhost:5432/app")
		add("README.md", fmt.Sprintf("# %s\n\nAPEX.BUILD bootstrapped this React + Vite + FastAPI scaffold.\n\n## Run\n\n1. Install frontend deps: `npm install`\n2. Install Python deps: `pip install -r requirements.txt`\n3. Start backend: `uvicorn main:app --port 8000 --reload`\n4. Start frontend: `npm run dev`\n5. Open `http://localhost:5173`\n", displayName))
		add("requirements.txt", "fastapi>=0.115.0\nuvicorn[standard]>=0.34.0\npython-dotenv>=1.1.0\n")
		add("main.py", fmt.Sprintf(`import os
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

app = FastAPI(title="%s")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["http://localhost:5173", "http://localhost:3000"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

@app.get("/health")
def health():
    return {"status": "ok", "app": "%s"}

if __name__ == "__main__":
    import uvicorn
    port = int(os.environ.get("PORT", 8000))
    uvicorn.run("main:app", host="0.0.0.0", port=port, reload=True)
`, displayName, displayName))
		add("package.json", fmt.Sprintf(`{
  "name": "%s",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": "^19.1.0",
    "react-dom": "^19.1.0"
  },
  "devDependencies": {
    "@types/react": "^19.1.2",
    "@types/react-dom": "^19.1.2",
    "@vitejs/plugin-react": "^4.4.1",
    "autoprefixer": "^10.4.21",
    "postcss": "^8.5.3",
    "tailwindcss": "^3.4.17",
    "typescript": "^5.8.2",
    "vite": "^6.3.2"
  }
}`, packageName))
		add("tsconfig.json", `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "jsx": "react-jsx",
    "strict": true,
    "noEmit": true,
    "resolveJsonModule": true,
    "esModuleInterop": true
  },
  "include": ["src"]
}`)
		add("vite.config.ts", `import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": { target: "http://localhost:8000", changeOrigin: true },
      "/health": { target: "http://localhost:8000", changeOrigin: true },
    },
  },
});`)
		add("tailwind.config.js", `/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: { extend: {} },
  plugins: [],
};`)
		add("postcss.config.js", `export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
};`)
		add("index.html", fmt.Sprintf(`<!doctype html>
<html lang="en">
  <head><meta charset="UTF-8" /><meta name="viewport" content="width=device-width, initial-scale=1.0" /><title>%s</title></head>
  <body><div id="root"></div><script type="module" src="/src/main.tsx"></script></body>
</html>`, displayName))
		add("src/main.tsx", `import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode><App /></React.StrictMode>
);`)
		add("src/vite-env.d.ts", `/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly [key: string]: string | boolean | undefined
  readonly VITE_API_URL?: string
  readonly VITE_API_BASE_URL?: string
  readonly VITE_WS_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}`)
		add("src/App.tsx", fmt.Sprintf(`import { useEffect, useState } from "react";

export default function App() {
  const [status, setStatus] = useState<string>("loading...");

  useEffect(() => {
    fetch("/health")
      .then((r) => r.json())
      .then((d) => setStatus(d.status))
      .catch(() => setStatus("offline"));
  }, []);

  return (
    <div className="min-h-screen bg-slate-900 text-white flex items-center justify-center">
      <div className="text-center space-y-4">
        <h1 className="text-4xl font-bold">%s</h1>
        <p className="text-slate-400">FastAPI status: <span className="text-sky-400 font-mono">{status}</span></p>
      </div>
    </div>
  );
}`, displayName))
		add("src/index.css", `@tailwind base;
@tailwind components;
@tailwind utilities;`)

	case "api/python-fastapi":
		add(".env.example", "PORT=8000\nDATABASE_URL=postgresql://postgres:postgres@localhost:5432/app")
		add("README.md", fmt.Sprintf("# %s\n\nAPEX.BUILD bootstrapped this Python FastAPI scaffold.\n\n## Run\n\n1. `pip install -r requirements.txt`\n2. `uvicorn main:app --port 8000 --reload`\n3. Open `http://localhost:8000/health`\n", displayName))
		add("requirements.txt", "fastapi>=0.115.0\nuvicorn[standard]>=0.34.0\npython-dotenv>=1.1.0\n")
		add("main.py", fmt.Sprintf(`import os
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

app = FastAPI(title="%s")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

@app.get("/health")
def health():
    return {"status": "ok", "app": "%s"}

if __name__ == "__main__":
    import uvicorn
    port = int(os.environ.get("PORT", 8000))
    uvicorn.run("main:app", host="0.0.0.0", port=port, reload=True)
`, displayName, displayName))

	case "frontend/nextjs-app", "fullstack/nextjs-api":
		add("README.md", fmt.Sprintf("# %s\n\nAPEX.BUILD bootstrapped this Next.js App Router scaffold.\n\n## Run\n\n1. `npm install`\n2. `npm run dev`\n3. Open `http://localhost:3000`\n", displayName))
		add("package.json", fmt.Sprintf(`{
  "name": "%s",
  "private": true,
  "version": "0.1.0",
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "start": "next start"
  },
  "dependencies": {
    "next": "^15.3.2",
    "react": "^19.1.0",
    "react-dom": "^19.1.0"
  },
  "devDependencies": {
    "@types/react": "^19.1.2",
    "@types/react-dom": "^19.1.2",
    "autoprefixer": "^10.4.21",
    "postcss": "^8.5.3",
    "tailwindcss": "^3.4.17",
    "typescript": "^5.8.2"
  }
}`, packageName))
		add("tsconfig.json", `{
  "compilerOptions": {
    "target": "ES2017",
    "lib": ["dom", "dom.iterable", "esnext"],
    "jsx": "preserve",
    "module": "esnext",
    "moduleResolution": "bundler",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "strict": true,
    "noEmit": true,
    "esModuleInterop": true,
    "incremental": true,
    "plugins": [{ "name": "next" }],
    "paths": { "@/*": ["./*"] }
  },
  "include": ["next-env.d.ts", "**/*.ts", "**/*.tsx", ".next/types/**/*.ts"],
  "exclude": ["node_modules"]
}`)
		add("next.config.js", `/** @type {import('next').NextConfig} */
const nextConfig = {};
module.exports = nextConfig;`)
		add("tailwind.config.js", `/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./app/**/*.{js,ts,jsx,tsx}", "./components/**/*.{js,ts,jsx,tsx}"],
  theme: { extend: {} },
  plugins: [],
};`)
		add("app/layout.tsx", fmt.Sprintf(`import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "%s",
  description: "Built with APEX.BUILD",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}`, displayName))
		add("app/page.tsx", fmt.Sprintf(`export default function Home() {
  return (
    <main className="min-h-screen bg-slate-900 text-white flex items-center justify-center">
      <div className="text-center space-y-4">
        <h1 className="text-4xl font-bold">%s</h1>
        <p className="text-slate-400">Next.js App Router scaffold — ready to build.</p>
      </div>
    </main>
  );
}`, displayName))
		add("app/globals.css", `@tailwind base;
@tailwind components;
@tailwind utilities;`)
		if scaffold.ID == "fullstack/nextjs-api" {
			add("app/api/health/route.ts", fmt.Sprintf(`import { NextResponse } from "next/server";

export async function GET() {
  return NextResponse.json({ status: "ok", app: "%s" });
}`, displayName))
		}

	case "api/express-typescript":
		add(".env.example", "PORT=3001\nDATABASE_URL=postgresql://postgres:postgres@localhost:5432/app")
		add("README.md", fmt.Sprintf("# %s\n\nAPEX.BUILD bootstrapped this Express + TypeScript API scaffold.\n\n## Run\n\n1. Install dependencies with `npm install`\n2. Start the API with `npm run dev`\n3. Open `http://localhost:3001/api/health`\n", displayName))
		add("package.json", fmt.Sprintf(`{
  "name": "%s",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "tsx watch src/server.ts",
    "build": "tsc -p tsconfig.json",
    "start": "node dist/src/server.js"
  },
  "dependencies": {
    "cors": "^2.8.5",
    "express": "^4.21.2"
  },
  "devDependencies": {
    "@types/cors": "^2.8.17",
    "@types/express": "^5.0.1",
    "@types/node": "^22.13.10",
    "tsx": "^4.19.3",
    "typescript": "^5.8.2"
  }
}`, packageName))
		add("tsconfig.json", `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "esModuleInterop": true,
    "strict": true,
    "outDir": "dist",
    "resolveJsonModule": true,
    "noEmit": false
  },
  "include": ["src"]
}`)
		add("src/server.ts", fmt.Sprintf(`import cors from "cors";
import express from "express";
import routes from "./routes/index";

const app = express();
const port = Number(process.env.PORT || 3001);

app.use(cors());
app.use(express.json());
app.use("/api", routes);

app.listen(port, "0.0.0.0", () => {
  console.log("%s API listening on port " + port);
});`, displayName))
		add("src/routes/index.ts", fmt.Sprintf(`import { Router } from "express";

const router = Router();

router.get("/health", (_req, res) => {
  res.json({ status: "ok", app: "%s" });
});

export default router;`, displayName))
	}

	out := make([]GeneratedFile, 0, len(filesByPath))
	for _, planned := range scaffold.Required {
		if file, ok := filesByPath[planned.Path]; ok {
			out = append(out, file)
		}
	}
	return out
}

func deriveScaffoldNames(description string, appType string) (string, string) {
	tokens := strings.Fields(description)
	words := make([]string, 0, 4)
	for _, token := range tokens {
		token = strings.Trim(token, " \t\r\n.,:;!?'\"`()[]{}")
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		words = append(words, token)
		if len(words) == 4 {
			break
		}
	}

	displayName := strings.Join(words, " ")
	if displayName == "" {
		switch appType {
		case "api":
			displayName = "Apex API"
		case "web":
			displayName = "Apex App"
		default:
			displayName = "Apex Build App"
		}
	}

	packageParts := make([]string, 0, len(words))
	for _, word := range words {
		word = strings.ToLower(word)
		word = strings.Trim(word, "-_")
		word = strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z':
				return r
			case r >= '0' && r <= '9':
				return r
			default:
				return '-'
			}
		}, word)
		word = strings.Trim(word, "-")
		if word == "" {
			continue
		}
		packageParts = append(packageParts, word)
	}

	packageName := strings.Join(packageParts, "-")
	if packageName == "" {
		packageName = "apex-build-app"
	}
	return displayName, packageName
}

func scaffoldFileLanguage(path string) string {
	switch {
	case strings.HasSuffix(path, ".ts"), strings.HasSuffix(path, ".tsx"):
		return "typescript"
	case strings.HasSuffix(path, ".js"), strings.HasSuffix(path, ".cjs"), strings.HasSuffix(path, ".mjs"):
		return "javascript"
	case strings.HasSuffix(path, ".json"):
		return "json"
	case strings.HasSuffix(path, ".css"):
		return "css"
	case strings.HasSuffix(path, ".html"):
		return "html"
	case strings.HasSuffix(path, ".go"):
		return "go"
	case strings.HasSuffix(path, ".md"):
		return "markdown"
	default:
		return "text"
	}
}

func currentOwnedFilesPrompt(files []GeneratedFile, workOrder *BuildWorkOrder, maxChars int) string {
	if workOrder == nil || len(files) == 0 || maxChars <= 0 {
		return ""
	}

	var b strings.Builder
	remaining := maxChars
	count := 0
	for _, file := range files {
		if !pathAllowedByWorkOrder(file.Path, workOrder) {
			continue
		}
		block := fmt.Sprintf("// File: %s\n```%s\n%s\n```\n\n", file.Path, file.Language, strings.TrimSpace(file.Content))
		if len(block) > remaining {
			break
		}
		b.WriteString(block)
		remaining -= len(block)
		count++
	}
	if count == 0 {
		return ""
	}
	return fmt.Sprintf("\n<current_owned_files>\nThese files already exist in the repo for your WorkOrder. Modify them in place unless a required scaffold file is still missing.\n%s</current_owned_files>\n", b.String())
}

func priorityStringToInt(priority string) int {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "high":
		return 100
	case "medium":
		return 60
	case "low":
		return 20
	default:
		return 50
	}
}

func hashBuildPlan(plan *BuildPlan) string {
	if plan == nil {
		return ""
	}
	payload := map[string]any{
		"app_type":       plan.AppType,
		"tech_stack":     plan.TechStack,
		"features":       plan.Features,
		"data_models":    plan.DataModels,
		"files":          plan.Files,
		"scaffold_files": plan.ScaffoldFiles,
		"scaffold_id":    plan.ScaffoldID,
		"env_vars":       plan.EnvVars,
		"acceptance":     plan.Acceptance,
		"api":            plan.APIContract,
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

func apiEndpointsFromContract(contract *BuildAPIContract) []APIEndpoint {
	if contract == nil {
		return nil
	}
	return append([]APIEndpoint(nil), contract.Endpoints...)
}

func dedupeAcceptanceChecks(in []BuildAcceptanceCheck) []BuildAcceptanceCheck {
	if len(in) == 0 {
		return nil
	}
	out := make([]BuildAcceptanceCheck, 0, len(in))
	seen := make(map[string]bool)
	for _, item := range in {
		key := string(item.Owner) + ":" + item.Description
		if seen[key] || strings.TrimSpace(item.Description) == "" {
			continue
		}
		seen[key] = true
		if item.ID == "" {
			item.ID = uuid.New().String()
		}
		out = append(out, item)
	}
	return out
}

func dedupeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func valueOrNone(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "none"
	}
	return value
}

func requiredOutputsForRole(role AgentRole) []string {
	switch role {
	case RoleArchitect:
		return []string{
			"Architecture blueprint aligned to the scaffold",
			"Frozen UI, API, data, and env contract for downstream specialists",
			"Ownership clarifications for specialists",
		}
	case RoleFrontend:
		return []string{
			"Complete frontend files within owned paths",
			"First usable UI shell aligned to the frozen contract",
			"Preview-ready frontend experience that matches the requested product surface",
			"Frontend API calls restricted to the frozen API contract or the actual implemented backend routes",
			"No backend logic in UI files",
		}
	case RoleBackend:
		return []string{
			"Runnable backend entrypoint and routes",
			"Backend implementation that satisfies the frozen UI/API contract",
			"Every API path promised by the contract and used by the frontend implemented in backend-owned files",
			"Preview-compatible runtime behavior for the full-stack vertical slice",
			"No frontend JSX/UI ownership drift",
		}
	case RoleDatabase:
		return []string{
			"Schema or migration files matching planned models",
			"Persistence design that supports the frozen product flows",
		}
	case RoleTesting:
		return []string{
			"Executable test or verification artifacts for the main slice",
			"Explicit frontend/backend route, CORS, and contract alignment verification for full-stack builds",
		}
	case RoleReviewer:
		return []string{"Concrete findings or explicit no-findings review"}
	case RoleSolver:
		return []string{"Minimal repair patch that restores verification and preview readiness"}
	default:
		return nil
	}
}

func summarizeWorkOrder(role AgentRole, appType string, stack TechStack) string {
	switch role {
	case RoleArchitect:
		return fmt.Sprintf("Lock the %s scaffold, screen map, and backend/data contract before specialists write runtime code.", appType)
	case RoleFrontend:
		return fmt.Sprintf("Build the first usable %s frontend and UI shell on the frozen scaffold, using the agreed contract without touching backend-owned files.", valueOrNone(stack.Frontend))
	case RoleBackend:
		return fmt.Sprintf("Implement the %s backend behind the already-shaped UI and frozen contract without drifting from the scaffold.", valueOrNone(stack.Backend))
	case RoleDatabase:
		return fmt.Sprintf("Add schema, migrations, and persistence wiring for %s behind the frozen product flows and API contract.", valueOrNone(stack.Database))
	case RoleTesting:
		return "Verify the main vertical slice after the UI and backend are wired together, explicitly catch route/CORS/API drift, then catch build/runtime regressions before review."
	case RoleReviewer:
		return "Review the generated app against the frozen scaffold, ownership map, and acceptance checklist."
	case RoleSolver:
		return "Repair verification failures without broad rewrites or provider/model churn."
	default:
		return "Work within the frozen build contract."
	}
}

func canonicalFrontendName(value string) string {
	normalized := normalizeDetectionText(value)
	switch {
	case normalized == "":
		return ""
	case containsAnyAffirmedTerm(normalized, []string{
		normalizeDetectionText("none"),
		normalizeDetectionText("no frontend"),
		normalizeDetectionText("frontend only"),
	}):
		return ""
	case containsAnyAffirmedTerm(normalized, []string{
		normalizeDetectionText("next.js"),
		normalizeDetectionText("next js"),
		normalizeDetectionText("nextjs"),
		normalizeDetectionText("next"),
	}):
		return "Next.js"
	case containsAnyAffirmedTerm(normalized, []string{
		normalizeDetectionText("react"),
	}):
		return "React"
	case containsAnyAffirmedTerm(normalized, []string{
		normalizeDetectionText("vue"),
	}):
		return "Vue"
	default:
		return strings.TrimSpace(value)
	}
}

func canonicalBackendName(value string) string {
	normalized := normalizeDetectionText(value)
	switch {
	case normalized == "":
		return ""
	case containsAnyAffirmedTerm(normalized, []string{
		normalizeDetectionText("none"),
		normalizeDetectionText("no backend"),
		normalizeDetectionText("frontend only"),
	}):
		return ""
	case containsAnyAffirmedTerm(normalized, []string{
		normalizeDetectionText("express"),
		normalizeDetectionText("express.js"),
		normalizeDetectionText("node"),
		normalizeDetectionText("node.js"),
		normalizeDetectionText("node js"),
	}):
		return "Express"
	case containsAnyAffirmedTerm(normalized, []string{
		normalizeDetectionText("go"),
		normalizeDetectionText("golang"),
	}):
		return "Go"
	case containsAnyAffirmedTerm(normalized, []string{
		normalizeDetectionText("fastapi"),
		normalizeDetectionText("fast api"),
		normalizeDetectionText("fast-api"),
	}):
		return "FastAPI"
	case containsAnyAffirmedTerm(normalized, []string{
		normalizeDetectionText("python"),
		normalizeDetectionText("django"),
		normalizeDetectionText("flask"),
	}):
		return "Python"
	default:
		return strings.TrimSpace(value)
	}
}

func canonicalDatabaseName(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "none", "no database", "no db", "n/a", "na":
		return ""
	case "postgres", "postgresql":
		return "PostgreSQL"
	case "mongo", "mongodb":
		return "MongoDB"
	case "sqlite":
		return "SQLite"
	default:
		return strings.TrimSpace(value)
	}
}

func canonicalStylingName(value string) string {
	normalized := normalizeDetectionText(value)
	switch {
	case normalized == "":
		return ""
	case containsAnyAffirmedTerm(normalized, []string{
		normalizeDetectionText("none"),
		normalizeDetectionText("unstyled"),
	}):
		return ""
	case containsAnyAffirmedTerm(normalized, []string{
		normalizeDetectionText("tailwind"),
		normalizeDetectionText("tailwind css"),
		normalizeDetectionText("tailwindcss"),
	}):
		return "Tailwind"
	case containsAnyAffirmedTerm(normalized, []string{
		normalizeDetectionText("css modules"),
	}):
		return "CSS Modules"
	default:
		return strings.TrimSpace(value)
	}
}

func cloneAPIContract(in *BuildAPIContract) *BuildAPIContract {
	if in == nil {
		return nil
	}
	out := *in
	out.CORSOrigins = append([]string(nil), in.CORSOrigins...)
	out.Endpoints = append([]APIEndpoint(nil), in.Endpoints...)
	return &out
}
