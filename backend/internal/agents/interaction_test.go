package agents

import (
	"strings"
	"testing"
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

	context := buildInteractionPromptContext(build)
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
