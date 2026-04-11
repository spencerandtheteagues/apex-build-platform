package agents

func astContextDietEnabledForBuild(build *Build) bool {
	if build != nil && build.SnapshotState.Orchestration != nil {
		return build.SnapshotState.Orchestration.Flags.EnableASTContextDiet
	}
	return envBool("APEX_AST_CONTEXT_DIET", false)
}

func routingWaterfallEnabledForBuild(build *Build) bool {
	if build != nil && build.SnapshotState.Orchestration != nil {
		return build.SnapshotState.Orchestration.Flags.EnableRoutingWaterfall
	}
	return envBool("APEX_ROUTING_WATERFALL", false)
}

func deterministicTaskGatesEnabledForBuild(build *Build) bool {
	if build != nil && build.SnapshotState.Orchestration != nil {
		return build.SnapshotState.Orchestration.Flags.EnableDeterministicTaskGates
	}
	return envBool("APEX_DETERMINISTIC_TASK_GATES", false)
}
