package agents

import "testing"

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
