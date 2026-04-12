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
