package agents

import (
	"context"
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

func TestSelectBuildScaffoldNewStacks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stack  TechStack
		wantID string
	}{
		{"react+express", TechStack{Frontend: "React", Backend: "Express"}, "fullstack/react-vite-express-ts"},
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
