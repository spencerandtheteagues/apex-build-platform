package agents

import (
	"strings"
	"testing"
	"time"
)

func TestRecordPermissionRequestLockedConsumesAllowOnceRule(t *testing.T) {
	build := &Build{
		Interaction: BuildInteractionState{
			PermissionRules: []BuildPermissionRule{
				{
					Scope:    PermissionScopeProgram,
					Target:   "docker",
					Decision: PermissionDecisionAllow,
					Mode:     PermissionModeOnce,
				},
			},
		},
	}

	request, created := recordPermissionRequestLocked(build, leadMessagePermissionRequest{
		Scope:  "program",
		Target: "docker",
		Reason: "Need Docker to run the app",
	}, nil)

	if created {
		t.Fatalf("expected allow-once rule to bypass request creation, got request %+v", request)
	}
	if len(build.Interaction.PermissionRules) != 0 {
		t.Fatalf("expected allow-once rule to be consumed, got %d rules", len(build.Interaction.PermissionRules))
	}
	if len(build.Interaction.SteeringNotes) == 0 || !strings.Contains(build.Interaction.SteeringNotes[0], "approved") {
		t.Fatalf("expected approval steering note, got %#v", build.Interaction.SteeringNotes)
	}
}

func TestBuildInteractionPromptContextIncludesDeniedPermissions(t *testing.T) {
	build := &Build{
		Agents: map[string]*Agent{
			"lead-1": {ID: "lead-1", Role: RoleLead},
		},
		Interaction: BuildInteractionState{
			PermissionRules: []BuildPermissionRule{
				{
					Scope:    PermissionScopeProgram,
					Target:   "docker",
					Decision: PermissionDecisionAllow,
					Mode:     PermissionModeBuild,
				},
				{
					Scope:    PermissionScopeNetwork,
					Target:   "localhost",
					Decision: PermissionDecisionDeny,
					Mode:     PermissionModeBuild,
				},
			},
		},
	}

	context := buildInteractionPromptContext(build, build.Agents["lead-1"])
	if !strings.Contains(context, "<local_permissions>") {
		t.Fatalf("expected granted permissions in prompt context, got %q", context)
	}
	if !strings.Contains(context, "<restricted_permissions>") {
		t.Fatalf("expected denied permissions in prompt context, got %q", context)
	}
	if !strings.Contains(context, "localhost") {
		t.Fatalf("expected denied target in prompt context, got %q", context)
	}
}

func TestBuildInteractionPromptContextFiltersTargetedMessagesByAgent(t *testing.T) {
	build := &Build{
		Agents: map[string]*Agent{
			"lead-1":     {ID: "lead-1", Role: RoleLead},
			"frontend-1": {ID: "frontend-1", Role: RoleFrontend},
			"backend-1":  {ID: "backend-1", Role: RoleBackend},
		},
		Interaction: BuildInteractionState{
			Messages: []BuildConversationMessage{
				{
					ID:        "global-1",
					Role:      ConversationRoleUser,
					Kind:      ConversationKindMessage,
					Content:   "Keep the UI dense.",
					Timestamp: time.Now().UTC(),
				},
				{
					ID:              "frontend-1",
					Role:            ConversationRoleLead,
					Kind:            ConversationKindDirective,
					Content:         "Tighten the dashboard spacing and card layout.",
					AgentID:         "lead-1",
					AgentRole:       string(RoleLead),
					TargetMode:      BuildMessageTargetRole,
					TargetAgentRole: string(RoleFrontend),
					Timestamp:       time.Now().UTC(),
				},
				{
					ID:              "backend-1",
					Role:            ConversationRoleUser,
					Kind:            ConversationKindDirective,
					Content:         "Refactor the API pagination behavior.",
					TargetMode:      BuildMessageTargetAgent,
					TargetAgentID:   "backend-1",
					TargetAgentRole: string(RoleBackend),
					Timestamp:       time.Now().UTC(),
				},
			},
		},
	}

	frontendContext := buildInteractionPromptContext(build, build.Agents["frontend-1"])
	if !strings.Contains(frontendContext, "Keep the UI dense.") {
		t.Fatalf("expected global message in frontend context, got %q", frontendContext)
	}
	if !strings.Contains(frontendContext, "dashboard spacing") {
		t.Fatalf("expected frontend directive in frontend context, got %q", frontendContext)
	}
	if strings.Contains(frontendContext, "pagination") {
		t.Fatalf("did not expect backend-only directive in frontend context, got %q", frontendContext)
	}

	backendContext := buildInteractionPromptContext(build, build.Agents["backend-1"])
	if strings.Contains(backendContext, "dashboard spacing") {
		t.Fatalf("did not expect frontend-only directive in backend context, got %q", backendContext)
	}
	if !strings.Contains(backendContext, "pagination") {
		t.Fatalf("expected backend directive in backend context, got %q", backendContext)
	}

	leadContext := buildInteractionPromptContext(build, build.Agents["lead-1"])
	if !strings.Contains(leadContext, "dashboard spacing") || !strings.Contains(leadContext, "pagination") {
		t.Fatalf("expected lead to see all targeted directives, got %q", leadContext)
	}
}
