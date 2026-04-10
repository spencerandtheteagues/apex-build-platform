package agents

import (
	"context"
	"testing"
	"time"

	"apex-build/internal/ai"
)

func TestRepairTaskEmitsPatchBundleAndVerificationReport(t *testing.T) {
	t.Parallel()

	// 1. Setup - failed task
	failedTask := &Task{
		ID:          "task-failed-1",
		Type:        TaskGenerateUI,
		Description: "Generate a broken dashboard",
		Status:      TaskFailed,
		CreatedAt:   time.Now(),
	}

	// 2. Setup - repair task
	repairTask := &Task{
		ID:          "task-repair-1",
		Type:        TaskFix,
		Description: "Repair the broken dashboard",
		Status:      TaskPending,
		MaxRetries:  2,
		Input: map[string]any{
			"action":         "solve_build_failure",
			"failed_task_id": failedTask.ID,
		},
		CreatedAt: time.Now(),
	}

	build := &Build{
		ID:     "build-repair-1",
		Status: BuildInProgress,
		Tasks:  []*Task{failedTask, repairTask},
		Agents: map[string]*Agent{},
		SnapshotFiles: []GeneratedFile{
			{
				Path:     "src/Dashboard.tsx",
				Content:  "export default function Dashboard(){ return <div>broken</div> }\n",
				Language: "typescript",
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
				WorkOrders: []WorkOrder{
					{
						ID:            "wo-repair",
						BuildID:       "build-repair-1",
						Role:          RoleSolver,
						Category:      WorkOrderRepair,
						TaskShape:     TaskShapeRepair,
						OwnedFiles:    []string{"src/Dashboard.tsx"},
						ContractSlice: WorkOrderContractSlice{Surface: SurfaceFrontend},
					},
				},
			},
		},
	}

	agent := &Agent{
		ID:       "agent-solver-1",
		Role:     RoleSolver,
		Provider: ai.ProviderGPT4,
		BuildID:  build.ID,
		Status:   StatusIdle,
	}
	build.Agents[agent.ID] = agent

	am := &AgentManager{
		agents:      map[string]*Agent{agent.ID: agent},
		builds:      map[string]*Build{build.ID: build},
		taskQueue:   make(chan *Task, 2),
		resultQueue: make(chan *TaskResult, 1),
		subscribers: map[string][]chan *WSMessage{},
		ctx:         context.Background(),
	}

	// 3. Assign task (triggers hydration of baseline)
	if err := am.AssignTask(agent.ID, repairTask); err != nil {
		t.Fatalf("AssignTask failed: %v", err)
	}

	// 4. Process successful result
	am.processResult(&TaskResult{
		TaskID:  repairTask.ID,
		AgentID: agent.ID,
		Success: true,
		Output: &TaskOutput{
			Files: []GeneratedFile{
				{
					Path:     "src/Dashboard.tsx",
					Content:  "export default function Dashboard(){ return <main>fixed</main> }\n",
					Language: "typescript",
				},
			},
		},
	})

	// 5. Verification
	state := build.SnapshotState.Orchestration
	if state == nil {
		t.Fatalf("expected orchestration state")
	}

	// 5.1 PatchBundle check
	if len(state.PatchBundles) == 0 {
		t.Fatalf("expected patch bundle for repair task")
	}
	bundle := state.PatchBundles[len(state.PatchBundles)-1]
	found := false
	for _, op := range bundle.Operations {
		if op.Path == "src/Dashboard.tsx" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected patch operation for fixed file, got %+v", bundle.Operations)
	}

	// 5.2 VerificationReport check
	if len(state.VerificationReports) == 0 {
		t.Fatalf("expected verification report for repair task")
	}
	var taskReport *VerificationReport
	for i := len(state.VerificationReports) - 1; i >= 0; i-- {
		if state.VerificationReports[i].Phase == "task_local_verification" {
			taskReport = &state.VerificationReports[i]
			break
		}
	}
	if taskReport == nil {
		t.Fatalf("expected task-local verification report, got %v", state.VerificationReports)
	}
	if taskReport.Status != VerificationPassed {
		t.Fatalf("expected passed task-local verification report, got %+v", taskReport)
	}

	// 5.3 Repair Path check
	if len(state.FailureFingerprints) == 0 {
		t.Fatalf("expected failure fingerprint recording the repair outcome")
	}
	var repairFp *FailureFingerprint
	for i := len(state.FailureFingerprints) - 1; i >= 0; i-- {
		if state.FailureFingerprints[i].TaskShape == TaskShapeRepair {
			repairFp = &state.FailureFingerprints[i]
			break
		}
	}
	if repairFp == nil {
		t.Fatalf("expected repair task fingerprint, got %v", state.FailureFingerprints)
	}
	if !containsStringLocal(repairFp.RepairPathChosen, "solve_build_failure") {
		t.Fatalf("expected repair path to include solve_build_failure, got %v", repairFp.RepairPathChosen)
	}
	if !repairFp.RepairSucceeded {
		t.Fatalf("expected repair_success to be true")
	}

	// 5.4 ReliabilitySummary check
	if state.ReliabilitySummary == nil {
		t.Fatalf("expected reliability summary")
	}
	if !containsStringLocal(state.ReliabilitySummary.ActiveRepairPath, "solve_build_failure") {
		t.Fatalf("expected reliability summary active repair path to include solve_build_failure, got %v", state.ReliabilitySummary.ActiveRepairPath)
	}
}

func containsStringLocal(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
