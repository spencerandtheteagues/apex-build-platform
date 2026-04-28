package agents

import (
	"strings"
	"time"
)

// DraftBuildSpec represents the pre-generation spec snapshot before it is
// locked for downstream coding tasks.
type DraftBuildSpec struct {
	Spec      *ValidatedBuildSpec
	CreatedAt time.Time
}

func compileDraftBuildSpec(buildID string, existing *ValidatedBuildSpec, plan *BuildPlan, contract *BuildContract) *DraftBuildSpec {
	spec := finalizeValidatedBuildSpec(buildID, existing, plan, contract)
	if spec == nil {
		return nil
	}

	spec.Locked = false
	spec.Source = "war_room_draft_v1"
	spec.BuildID = buildID
	if spec.CreatedAt.IsZero() {
		spec.CreatedAt = time.Now().UTC()
	}

	return &DraftBuildSpec{
		Spec:      spec,
		CreatedAt: time.Now().UTC(),
	}
}

func compileWarRoomValidatedBuildSpec(buildID string, existing *ValidatedBuildSpec, plan *BuildPlan, contract *BuildContract) *ValidatedBuildSpec {
	draft := compileDraftBuildSpec(buildID, existing, plan, contract)
	if draft == nil || draft.Spec == nil {
		return nil
	}

	issues := critiqueDraftBuildSpec(draft.Spec, contract)
	applyWarRoomCritiqueAdvisories(draft.Spec, issues)

	draft.Spec.Locked = true
	draft.Spec.Source = "war_room_validated_v1"
	draft.Spec.BuildID = buildID
	draft.Spec.SecurityAdvisories = dedupeBuildSpecAdvisories(draft.Spec.SecurityAdvisories)
	draft.Spec.PerformanceAdvisories = dedupeBuildSpecAdvisories(draft.Spec.PerformanceAdvisories)
	if len(draft.Spec.AcceptanceSurfaces) == 0 {
		draft.Spec.AcceptanceSurfaces = deriveValidatedAcceptanceSurfaces(contract, strings.TrimSpace(draft.Spec.DeliveryMode))
	}
	return draft.Spec
}

// enrichWarRoomSpecWithLLMDebate runs the two-provider LLM debate against the
// already-locked spec and applies any new advisories it produces. It is called
// outside the build lock after the static critique has already been applied.
// If the router is nil or the debate produces no results it is a no-op.
func (am *AgentManager) enrichWarRoomSpecWithLLMDebate(buildID string, userID uint, usesPlatformKeys bool, powerMode PowerMode, spec *ValidatedBuildSpec, contract *BuildContract) {
	if am == nil || spec == nil || am.aiRouter == nil {
		return
	}
	if !shouldRunWarRoomLLMDebate(powerMode) {
		return
	}
	issues := am.runWarRoomLLMDebate(buildID, userID, usesPlatformKeys, powerMode, spec, contract)
	if len(issues) == 0 {
		return
	}
	applyWarRoomCritiqueAdvisories(spec, issues)
	spec.SecurityAdvisories = dedupeBuildSpecAdvisories(spec.SecurityAdvisories)
	spec.PerformanceAdvisories = dedupeBuildSpecAdvisories(spec.PerformanceAdvisories)
}

func countWarRoomBuildSpecAdvisories(spec *ValidatedBuildSpec) int {
	if spec == nil {
		return 0
	}
	count := 0
	countAdvisories := func(values []BuildSpecAdvisory) {
		for _, value := range values {
			if strings.HasPrefix(strings.TrimSpace(value.Code), "war_room_") {
				count++
			}
		}
	}
	countAdvisories(spec.SecurityAdvisories)
	countAdvisories(spec.PerformanceAdvisories)
	return count
}
