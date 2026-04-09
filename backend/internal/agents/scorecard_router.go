package agents

import "apex-build/internal/ai"

const minLiveScorecardSamples = 3

func scorecardHasObservedSamples(scorecard ProviderScorecard) bool {
	return scorecard.SampleCount >= minLiveScorecardSamples ||
		scorecard.SuccessCount >= minLiveScorecardSamples ||
		scorecard.FirstPassSampleCount >= minLiveScorecardSamples ||
		scorecard.RepairAttemptCount >= minLiveScorecardSamples ||
		scorecard.PromotionAttemptCount >= minLiveScorecardSamples ||
		scorecard.TokenSampleCount >= minLiveScorecardSamples ||
		scorecard.CostSampleCount >= minLiveScorecardSamples ||
		scorecard.LatencySampleCount >= minLiveScorecardSamples
}

func observedScorecards(scorecards []ProviderScorecard) []ProviderScorecard {
	if len(scorecards) == 0 {
		return nil
	}
	filtered := make([]ProviderScorecard, 0, len(scorecards))
	for _, scorecard := range scorecards {
		if scorecardHasObservedSamples(scorecard) {
			filtered = append(filtered, scorecard)
		}
	}
	return filtered
}

func hasSufficientLiveScorecards(scorecards []ProviderScorecard) bool {
	return len(observedScorecards(scorecards)) > 0
}

func buildCostSensitivity(build *Build) CostSensitivity {
	if build == nil {
		return CostSensitivityMedium
	}
	build.mu.RLock()
	defer build.mu.RUnlock()
	if orchestration := build.SnapshotState.Orchestration; orchestration != nil && orchestration.IntentBrief != nil && orchestration.IntentBrief.CostSensitivity != "" {
		return orchestration.IntentBrief.CostSensitivity
	}
	switch build.PowerMode {
	case PowerMax:
		return CostSensitivityLow
	case PowerFast:
		return CostSensitivityHigh
	default:
		return CostSensitivityMedium
	}
}

func selectProviderByScorecard(build *Build, role AgentRole, shape TaskShape, available []ai.AIProvider, scorecards []ProviderScorecard) ai.AIProvider {
	if len(available) == 0 {
		return ""
	}
	if shape == "" {
		shape = taskShapeForRole(role)
	}
	if shape == "" {
		return ""
	}
	live := observedScorecards(scorecards)
	if len(live) == 0 {
		return ""
	}
	availableSet := make(map[ai.AIProvider]bool, len(available))
	for _, provider := range available {
		availableSet[provider] = true
	}
	for _, candidate := range rankedProvidersForTaskShapeWithCost(shape, live, buildCostSensitivity(build)) {
		if availableSet[candidate] {
			return candidate
		}
	}
	return ""
}

func reliabilityPreferredProviders(build *Build, role AgentRole) []ai.AIProvider {
	if build == nil {
		return nil
	}
	build.mu.RLock()
	var summary *BuildReliabilitySummary
	if build.SnapshotState.Orchestration != nil && build.SnapshotState.Orchestration.ReliabilitySummary != nil {
		copied := *build.SnapshotState.Orchestration.ReliabilitySummary
		copied.AdvisoryClasses = append([]string(nil), copied.AdvisoryClasses...)
		copied.RecurringFailureClass = append([]string(nil), copied.RecurringFailureClass...)
		summary = &copied
	}
	build.mu.RUnlock()
	if summary == nil {
		return nil
	}

	compileRecurring := summary.CurrentFailureClass == "compile_failure" || containsString(summary.RecurringFailureClass, "compile_failure")
	visualRecurring := containsString(summary.AdvisoryClasses, "visual_layout") || containsString(summary.RecurringFailureClass, "visual_layout")
	interactionRecurring := containsString(summary.AdvisoryClasses, "interaction_canary") || containsString(summary.RecurringFailureClass, "interaction_canary")
	contractRecurring := summary.CurrentFailureClass == "contract_violation" || summary.CurrentFailureClass == "coordination_violation" ||
		containsString(summary.RecurringFailureClass, "contract_violation") || containsString(summary.RecurringFailureClass, "coordination_violation")

	switch {
	case (visualRecurring || interactionRecurring) && (role == RoleFrontend || role == RoleReviewer || role == RoleTesting || role == RoleSolver):
		return []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4, ai.ProviderGemini, ai.ProviderGrok}
	case compileRecurring && (role == RoleFrontend || role == RoleBackend || role == RoleDatabase || role == RoleSolver):
		return []ai.AIProvider{ai.ProviderGPT4, ai.ProviderClaude, ai.ProviderGemini, ai.ProviderGrok}
	case contractRecurring && (role == RolePlanner || role == RoleArchitect || role == RoleReviewer):
		return []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4, ai.ProviderGemini, ai.ProviderGrok}
	default:
		return nil
	}
}

func validatedSpecPreferredProviders(build *Build, role AgentRole) []ai.AIProvider {
	if build == nil {
		return nil
	}
	build.mu.RLock()
	var spec *ValidatedBuildSpec
	if build.SnapshotState.Orchestration != nil && build.SnapshotState.Orchestration.ValidatedBuildSpec != nil {
		copied := *build.SnapshotState.Orchestration.ValidatedBuildSpec
		copied.SecurityAdvisories = append([]BuildSpecAdvisory(nil), copied.SecurityAdvisories...)
		copied.PerformanceAdvisories = append([]BuildSpecAdvisory(nil), copied.PerformanceAdvisories...)
		spec = &copied
	}
	build.mu.RUnlock()
	if spec == nil {
		return nil
	}

	frontendPerf := false
	integrationPerf := false
	backendSecurity := false
	for _, advisory := range spec.PerformanceAdvisories {
		switch advisory.Surface {
		case SurfaceFrontend, SurfaceGlobal:
			frontendPerf = true
		case SurfaceIntegration:
			integrationPerf = true
		}
	}
	for _, advisory := range spec.SecurityAdvisories {
		switch advisory.Surface {
		case SurfaceBackend, SurfaceData, SurfaceIntegration, SurfaceGlobal:
			backendSecurity = true
		}
	}

	switch {
	case frontendPerf && (role == RoleFrontend || role == RoleTesting || role == RoleReviewer):
		return []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4, ai.ProviderGemini, ai.ProviderGrok}
	case integrationPerf && (role == RoleBackend || role == RoleSolver || role == RoleReviewer):
		return []ai.AIProvider{ai.ProviderGPT4, ai.ProviderClaude, ai.ProviderGemini, ai.ProviderGrok}
	case backendSecurity && (role == RoleReviewer || role == RoleArchitect || role == RolePlanner):
		return []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4, ai.ProviderGemini, ai.ProviderGrok}
	default:
		return nil
	}
}
