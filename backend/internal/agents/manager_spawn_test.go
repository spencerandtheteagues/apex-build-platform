package agents

import (
	"testing"
	"time"
)

func TestInitialAgentRolesForBuildUsesWorkOrders(t *testing.T) {
	build := &Build{
		Plan: &BuildPlan{
			WorkOrders: []BuildWorkOrder{
				{Role: RoleArchitect},
				{Role: RoleFrontend},
				{Role: RoleTesting},
				{Role: RoleReviewer},
				{Role: RoleSolver},
			},
		},
	}

	roles := initialAgentRolesForBuild(build)
	expected := []AgentRole{RoleArchitect, RoleFrontend, RoleTesting, RoleReviewer}
	if len(roles) != len(expected) {
		t.Fatalf("expected %d roles, got %d (%v)", len(expected), len(roles), roles)
	}
	for i, role := range expected {
		if roles[i] != role {
			t.Fatalf("expected role %d to be %s, got %s", i, role, roles[i])
		}
	}
}

func TestInitialAgentRolesForBuildFallsBackToLegacyRolesWithoutPlan(t *testing.T) {
	roles := initialAgentRolesForBuild(&Build{})
	expected := []AgentRole{
		RoleArchitect,
		RoleDatabase,
		RoleBackend,
		RoleFrontend,
		RoleTesting,
		RoleReviewer,
	}
	if len(roles) != len(expected) {
		t.Fatalf("expected %d roles, got %d (%v)", len(expected), len(roles), roles)
	}
	for i, role := range expected {
		if roles[i] != role {
			t.Fatalf("expected role %d to be %s, got %s", i, role, roles[i])
		}
	}
}

func TestFilterAgentRolesPreservesOrder(t *testing.T) {
	roles := []AgentRole{RoleArchitect, RoleDatabase, RoleBackend, RoleFrontend, RoleTesting, RoleReviewer}
	filtered := filterAgentRoles(roles, map[AgentRole]bool{
		RoleArchitect: true,
		RoleFrontend:  true,
		RoleReviewer:  true,
	})
	expected := []AgentRole{RoleArchitect, RoleFrontend, RoleReviewer}
	if len(filtered) != len(expected) {
		t.Fatalf("expected %d roles, got %d (%v)", len(expected), len(filtered), filtered)
	}
	for i, role := range expected {
		if filtered[i] != role {
			t.Fatalf("expected role %d to be %s, got %s", i, role, filtered[i])
		}
	}
}

func TestResumeBuildExecutionRequeuesPendingRecoveryTasksAndRefreshesTimestamp(t *testing.T) {
	oldUpdatedAt := time.Now().Add(-10 * time.Minute).UTC()
	build := &Build{
		ID:        "build-1",
		Status:    BuildReviewing,
		UpdatedAt: oldUpdatedAt,
		Agents: map[string]*Agent{
			"testing-1": {ID: "testing-1", BuildID: "build-1", Role: RoleTesting},
		},
		Tasks: []*Task{
			{
				ID:          "task-1",
				Type:        TaskTest,
				Description: "Regression test after automated fixes",
				Status:      TaskPending,
				Input: map[string]any{
					"action": "regression_test",
				},
			},
		},
	}
	am := &AgentManager{
		builds: map[string]*Build{
			build.ID: build,
		},
		agents: map[string]*Agent{
			"testing-1": build.Agents["testing-1"],
		},
		taskQueue: make(chan *Task, 1),
	}

	am.resumeBuildExecution(build, false)

	task := build.Tasks[0]
	if task.Status != TaskInProgress {
		t.Fatalf("expected task to be requeued as in-progress, got %s", task.Status)
	}
	if task.AssignedTo != "testing-1" {
		t.Fatalf("expected task to be assigned to testing-1, got %s", task.AssignedTo)
	}
	if !build.UpdatedAt.After(oldUpdatedAt) {
		t.Fatalf("expected build updated_at to advance after requeue, old=%s new=%s", oldUpdatedAt, build.UpdatedAt)
	}

	select {
	case queued := <-am.taskQueue:
		if queued.ID != task.ID {
			t.Fatalf("expected queued task %s, got %s", task.ID, queued.ID)
		}
	default:
		t.Fatalf("expected task to be pushed back onto the queue")
	}
}
