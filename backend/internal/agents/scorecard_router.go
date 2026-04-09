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
