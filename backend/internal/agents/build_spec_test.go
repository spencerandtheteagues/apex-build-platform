package agents

import (
	"context"
	"strings"
	"testing"
	"time"

	"apex-build/internal/agents/autonomous"
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
				{Name: "Transcript", Fields: map[string]string{"id": "uuid", "title": "string"}},
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
		ID:          "build-1",
		Description: "Build TranscriptVault",
		Status:      BuildInProgress,
		MaxRetries:  2,
		Plan:        plan,
		Tasks:       []*Task{},
		Agents:      map[string]*Agent{},
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
