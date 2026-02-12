package agents

import (
	"testing"

	"apex-build/internal/ai"
)

func TestAssignProvidersToRoles_UsesPreferredProvidersWhenAvailable(t *testing.T) {
	am := &AgentManager{}
	providers := []ai.AIProvider{
		ai.ProviderClaude,
		ai.ProviderGPT4,
		ai.ProviderGemini,
	}
	roles := []AgentRole{
		RoleArchitect,
		RolePlanner,
		RoleReviewer,
		RoleFrontend,
		RoleBackend,
		RoleDatabase,
		RoleTesting,
	}

	assignments := am.assignProvidersToRoles(providers, roles)

	if got := assignments[RoleArchitect]; got != ai.ProviderClaude {
		t.Fatalf("architect provider = %s, want %s", got, ai.ProviderClaude)
	}
	if got := assignments[RolePlanner]; got != ai.ProviderClaude {
		t.Fatalf("planner provider = %s, want %s", got, ai.ProviderClaude)
	}
	if got := assignments[RoleReviewer]; got != ai.ProviderClaude {
		t.Fatalf("reviewer provider = %s, want %s", got, ai.ProviderClaude)
	}
	if got := assignments[RoleFrontend]; got != ai.ProviderGPT4 {
		t.Fatalf("frontend provider = %s, want %s", got, ai.ProviderGPT4)
	}
	if got := assignments[RoleBackend]; got != ai.ProviderGPT4 {
		t.Fatalf("backend provider = %s, want %s", got, ai.ProviderGPT4)
	}
	if got := assignments[RoleDatabase]; got != ai.ProviderGPT4 {
		t.Fatalf("database provider = %s, want %s", got, ai.ProviderGPT4)
	}
	if got := assignments[RoleTesting]; got != ai.ProviderGemini {
		t.Fatalf("testing provider = %s, want %s", got, ai.ProviderGemini)
	}
}

func TestAssignProvidersToRoles_FallsBackWhenPreferredProvidersUnavailable(t *testing.T) {
	am := &AgentManager{}
	providers := []ai.AIProvider{
		ai.ProviderClaude,
		ai.ProviderGemini,
	}
	roles := []AgentRole{
		RoleArchitect,
		RoleFrontend,
		RoleTesting,
	}

	assignments := am.assignProvidersToRoles(providers, roles)

	if got := assignments[RoleArchitect]; got != ai.ProviderClaude {
		t.Fatalf("architect provider = %s, want %s", got, ai.ProviderClaude)
	}
	// GPT is unavailable, so coding should gracefully fall back to Claude.
	if got := assignments[RoleFrontend]; got != ai.ProviderClaude {
		t.Fatalf("frontend provider = %s, want %s", got, ai.ProviderClaude)
	}
	if got := assignments[RoleTesting]; got != ai.ProviderGemini {
		t.Fatalf("testing provider = %s, want %s", got, ai.ProviderGemini)
	}
}
