package agents

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"apex-build/internal/ai"
	"apex-build/pkg/models"
)

const (
	maxHistoricalBuildSamples = 8
	maxHistoricalBuildScan    = 24
)

type historicalBuildScope struct {
	Name             string
	ProjectID        uint
	NormalizedName   string
	StackFingerprint string
}

type historicalBuildLearningResult struct {
	Summary            *BuildLearningSummary
	ProviderScorecards []ProviderScorecard
}

func historicalBuildLearningEnabled() bool {
	return envBool("APEX_BUILD_LEARNING_MEMORY", true)
}

func (am *AgentManager) refreshHistoricalBuildLearning(build *Build, req *BuildRequest) {
	if am == nil || am.db == nil || build == nil || !historicalBuildLearningEnabled() {
		return
	}

	result := am.deriveHistoricalBuildLearningResult(build, req)

	build.mu.Lock()
	defer build.mu.Unlock()

	state := ensureBuildOrchestrationStateLocked(build)
	if state == nil {
		return
	}
	updated := false
	if result.Summary != nil {
		state.HistoricalLearning = result.Summary
		updated = true
		log.Printf("[build_learning] {\"build_id\":%q,\"scope\":%q,\"observed_builds\":%d}", build.ID, result.Summary.Scope, result.Summary.ObservedBuilds)
	}
	if state.Flags.EnableProviderScorecards && len(result.ProviderScorecards) > 0 {
		state.ProviderScorecards = mergeHistoricalProviderScorecards(state.ProviderScorecards, result.ProviderScorecards, build.ProviderMode)
		updated = true
	}
	if updated {
		refreshDerivedSnapshotStateLocked(build, &build.SnapshotState)
	}
}

func (am *AgentManager) deriveHistoricalBuildLearning(build *Build, req *BuildRequest) *BuildLearningSummary {
	return am.deriveHistoricalBuildLearningResult(build, req).Summary
}

func (am *AgentManager) deriveHistoricalBuildLearningResult(build *Build, req *BuildRequest) historicalBuildLearningResult {
	if am == nil || am.db == nil || build == nil {
		return historicalBuildLearningResult{}
	}

	build.mu.RLock()
	buildID := strings.TrimSpace(build.ID)
	userID := build.UserID
	scope := resolveHistoricalBuildScopeLocked(build, req)
	build.mu.RUnlock()

	if userID == 0 || scope.Name == "" {
		return historicalBuildLearningResult{}
	}

	var snapshots []models.CompletedBuild
	query := am.db.Where("user_id = ?", userID).Order("updated_at DESC").Limit(maxHistoricalBuildScan)
	if scope.ProjectID != 0 {
		query = query.Where("project_id = ?", scope.ProjectID)
	}
	if scope.NormalizedName != "" {
		query = query.Where("LOWER(TRIM(project_name)) = ?", scope.NormalizedName)
	}
	if err := query.Find(&snapshots).Error; err != nil {
		log.Printf("build learning query failed for build %s: %v", buildID, err)
		return historicalBuildLearningResult{}
	}

	matches := make([]models.CompletedBuild, 0, maxHistoricalBuildSamples)
	for _, snapshot := range snapshots {
		if strings.TrimSpace(snapshot.BuildID) == "" || strings.TrimSpace(snapshot.BuildID) == buildID {
			continue
		}
		if !historicalSnapshotMatchesScope(snapshot, scope) {
			continue
		}
		matches = append(matches, snapshot)
		if len(matches) >= maxHistoricalBuildSamples {
			break
		}
	}
	if len(matches) == 0 {
		return historicalBuildLearningResult{}
	}

	return historicalBuildLearningResult{
		Summary:            summarizeHistoricalBuildLearning(scope.Name, matches),
		ProviderScorecards: summarizeHistoricalProviderScorecards(matches, build.ProviderMode),
	}
}

func resolveHistoricalBuildScopeLocked(build *Build, req *BuildRequest) historicalBuildScope {
	if build == nil {
		return historicalBuildScope{}
	}
	if build.ProjectID != nil && *build.ProjectID != 0 {
		return historicalBuildScope{
			Name:      "same_project",
			ProjectID: *build.ProjectID,
		}
	}

	if req != nil {
		if normalizedName := normalizeLearningName(req.ProjectName); normalizedName != "" {
			return historicalBuildScope{
				Name:           "same_project_name",
				NormalizedName: normalizedName,
			}
		}
		if fp := stackFingerprintFromTechStack(req.TechStack); fp != "" {
			return historicalBuildScope{
				Name:             "same_stack",
				StackFingerprint: fp,
			}
		}
	}

	if fp := stackFingerprintFromTechStack(build.TechStack); fp != "" {
		return historicalBuildScope{
			Name:             "same_stack",
			StackFingerprint: fp,
		}
	}

	return historicalBuildScope{}
}

func historicalSnapshotMatchesScope(snapshot models.CompletedBuild, scope historicalBuildScope) bool {
	switch {
	case scope.ProjectID != 0:
		return snapshot.ProjectID != nil && *snapshot.ProjectID == scope.ProjectID
	case scope.NormalizedName != "":
		return normalizeLearningName(snapshot.ProjectName) == scope.NormalizedName
	case scope.StackFingerprint != "":
		return stackFingerprintFromRawTechStack(snapshot.TechStack) == scope.StackFingerprint
	default:
		return false
	}
}

func stackFingerprintFromRawTechStack(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	var stack TechStack
	if err := json.Unmarshal([]byte(raw), &stack); err != nil {
		return ""
	}
	return stackFingerprintFromTechStack(&stack)
}

func stackFingerprintFromTechStack(stack *TechStack) string {
	if stack == nil {
		return ""
	}
	normalizedExtras := make([]string, 0, len(stack.Extras))
	for _, extra := range stack.Extras {
		if trimmed := strings.ToLower(strings.TrimSpace(extra)); trimmed != "" {
			normalizedExtras = append(normalizedExtras, trimmed)
		}
	}
	sort.Strings(normalizedExtras)

	parts := []string{
		strings.ToLower(strings.TrimSpace(stack.Frontend)),
		strings.ToLower(strings.TrimSpace(stack.Backend)),
		strings.ToLower(strings.TrimSpace(stack.Database)),
		strings.ToLower(strings.TrimSpace(stack.Styling)),
		strings.Join(normalizedExtras, ","),
	}
	if strings.TrimSpace(strings.Join(parts, "")) == "" {
		return ""
	}
	return strings.Join(parts, "|")
}

func normalizeLearningName(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func summarizeHistoricalBuildLearning(scope string, snapshots []models.CompletedBuild) *BuildLearningSummary {
	if len(snapshots) == 0 {
		return nil
	}

	failureCounts := make(map[string]int)
	repairPathCounts := make(map[string]int)
	repairStrategyStats := make(map[string]*repairStrategyWinRate)
	semanticRepairHintCounts := make(map[string]int)
	warningCounts := make(map[string]int)
	hotspotCounts := make(map[string]int)
	cleanPassCounts := make(map[string]int)
	sourceIDs := make([]string, 0, len(snapshots))

	for _, snapshot := range snapshots {
		if trimmedID := strings.TrimSpace(snapshot.BuildID); trimmedID != "" {
			sourceIDs = append(sourceIDs, trimmedID)
		}

		state := parseBuildSnapshotState(snapshot.StateJSON)
		orch := state.Orchestration
		if orch == nil {
			if class := normalizeFailureClass(snapshot.Error); class != "" && class != "build_failure" {
				failureCounts[class]++
			}
			continue
		}

		for _, fp := range orch.FailureFingerprints {
			if class := normalizeFailureIdentifier(fp.FailureClass); class != "" {
				failureCounts[class]++
			}
			if fp.RepairSucceeded && len(fp.RepairPathChosen) > 0 {
				repairPathCounts[strings.Join(fp.RepairPathChosen, " -> ")]++
			}
			if strategyKey := repairStrategyLearningKey(fp); strategyKey != "" {
				stats := repairStrategyStats[strategyKey]
				if stats == nil {
					stats = &repairStrategyWinRate{Key: strategyKey}
					repairStrategyStats[strategyKey] = stats
				}
				stats.Attempts++
				if fp.RepairSucceeded {
					stats.Successes++
				}
			}
			if hint := semanticRepairCacheHintFromFingerprint(fp); hint != "" {
				semanticRepairHintCounts[hint]++
			}
			for _, path := range fp.FilesInvolved {
				if trimmed := strings.TrimSpace(path); trimmed != "" {
					hotspotCounts[trimmed]++
				}
			}
		}

		for _, report := range latestVerificationReports(orch.VerificationReports) {
			if report.Status == VerificationPassed && len(report.Warnings) == 0 {
				signal := strings.TrimSpace(report.Phase)
				if surface := strings.TrimSpace(string(report.Surface)); surface != "" {
					signal = fmt.Sprintf("%s/%s", signal, surface)
				}
				if signal != "" {
					cleanPassCounts[signal+" clean"]++
				}
			}
			for _, warning := range report.Warnings {
				if trimmed := strings.TrimSpace(warning); trimmed != "" {
					warningCounts[trimmed]++
				}
			}
		}

		if summary := orch.ReliabilitySummary; summary != nil {
			for _, class := range summary.RecurringFailureClass {
				if normalized := normalizeFailureIdentifier(class); normalized != "" {
					failureCounts[normalized]++
				}
			}
		}
	}

	recurringFailures := topCountedStrings(failureCounts, 5)
	frequentWarnings := topCountedStrings(warningCounts, 5)
	recommendedAvoidance := deriveHistoricalAvoidanceTips(recurringFailures, frequentWarnings)

	return &BuildLearningSummary{
		Scope:                   scope,
		ObservedBuilds:          len(snapshots),
		SourceBuildIDs:          limitStrings(sourceIDs, 6),
		RecurringFailureClasses: recurringFailures,
		SuccessfulRepairPaths:   topCountedStrings(repairPathCounts, 4),
		RepairStrategyWinRates:  topRepairStrategyWinRates(repairStrategyStats, 4),
		SemanticRepairHints:     topCountedStrings(semanticRepairHintCounts, 4),
		FrequentWarnings:        frequentWarnings,
		HotspotFiles:            topCountedStrings(hotspotCounts, 5),
		RecommendedAvoidance:    recommendedAvoidance,
		CleanPassSignals:        topCountedStrings(cleanPassCounts, 4),
		GeneratedAt:             time.Now().UTC(),
	}
}

type repairStrategyWinRate struct {
	Key       string
	Attempts  int
	Successes int
}

func repairStrategyLearningKey(fingerprint FailureFingerprint) string {
	strategy := strings.TrimSpace(fingerprint.RepairStrategy)
	if strategy == "" {
		return ""
	}
	patchClass := normalizeRepairMemoryIdentifier(fingerprint.PatchClass)
	if patchClass == "" {
		return strategy
	}
	return strategy + "/" + patchClass
}

func topRepairStrategyWinRates(stats map[string]*repairStrategyWinRate, limit int) []string {
	if len(stats) == 0 {
		return nil
	}
	ordered := make([]*repairStrategyWinRate, 0, len(stats))
	for _, item := range stats {
		if item == nil || strings.TrimSpace(item.Key) == "" || item.Attempts <= 0 {
			continue
		}
		ordered = append(ordered, item)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		leftRate := float64(ordered[i].Successes) / float64(ordered[i].Attempts)
		rightRate := float64(ordered[j].Successes) / float64(ordered[j].Attempts)
		if leftRate != rightRate {
			return leftRate > rightRate
		}
		if ordered[i].Attempts != ordered[j].Attempts {
			return ordered[i].Attempts > ordered[j].Attempts
		}
		return ordered[i].Key < ordered[j].Key
	})
	out := make([]string, 0, min(len(ordered), limit))
	for _, item := range ordered {
		out = append(out, fmt.Sprintf("%s: %d/%d success", item.Key, item.Successes, item.Attempts))
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func summarizeHistoricalProviderScorecards(snapshots []models.CompletedBuild, providerMode string) []ProviderScorecard {
	if len(snapshots) == 0 {
		return nil
	}

	scorecardsByKey := map[string]*ProviderScorecard{}
	for _, snapshot := range snapshots {
		state := parseBuildSnapshotState(snapshot.StateJSON)
		if state.Orchestration == nil {
			continue
		}
		for _, scorecard := range state.Orchestration.ProviderScorecards {
			if scorecard.Provider == "" || scorecard.TaskShape == "" || !scorecardHasObservedSamples(scorecard) {
				continue
			}
			key := providerScorecardKey(scorecard.Provider, scorecard.TaskShape)
			merged := scorecardsByKey[key]
			if merged == nil {
				scorecard.HostedEligible = !hostedProviderMode(providerMode) || scorecard.Provider != ai.ProviderOllama
				scorecard = normalizeProviderScorecardRates(scorecard)
				scorecardsByKey[key] = &scorecard
				continue
			}
			mergeProviderScorecardSample(merged, scorecard)
		}
	}
	if len(scorecardsByKey) == 0 {
		return nil
	}

	scorecards := make([]ProviderScorecard, 0, len(scorecardsByKey))
	for _, scorecard := range scorecardsByKey {
		scorecards = append(scorecards, normalizeProviderScorecardRates(*scorecard))
	}
	sort.SliceStable(scorecards, func(i, j int) bool {
		if scorecards[i].TaskShape != scorecards[j].TaskShape {
			return scorecards[i].TaskShape < scorecards[j].TaskShape
		}
		return scorecards[i].Provider < scorecards[j].Provider
	})
	return scorecards
}

func mergeHistoricalProviderScorecards(base []ProviderScorecard, historical []ProviderScorecard, providerMode string) []ProviderScorecard {
	if len(historical) == 0 {
		return base
	}
	if len(base) == 0 {
		base = defaultProviderScorecards(providerMode)
	}
	merged := append([]ProviderScorecard(nil), base...)
	for _, historicalScorecard := range historical {
		if historicalScorecard.Provider == "" || historicalScorecard.TaskShape == "" {
			continue
		}
		idx := -1
		for i := range merged {
			if merged[i].Provider == historicalScorecard.Provider && merged[i].TaskShape == historicalScorecard.TaskShape {
				idx = i
				break
			}
		}
		if idx == -1 {
			historicalScorecard.HostedEligible = !hostedProviderMode(providerMode) || historicalScorecard.Provider != ai.ProviderOllama
			merged = append(merged, normalizeProviderScorecardRates(historicalScorecard))
			continue
		}
		mergeProviderScorecardSample(&merged[idx], historicalScorecard)
	}
	return merged
}

func providerScorecardKey(provider ai.AIProvider, shape TaskShape) string {
	return string(provider) + "|" + string(shape)
}

func mergeProviderScorecardSample(target *ProviderScorecard, sample ProviderScorecard) {
	if target == nil || sample.Provider == "" || sample.TaskShape == "" {
		return
	}
	target.AverageAcceptedTokens = mergeAveragedMetric(target.AverageAcceptedTokens, target.TokenSampleCount, sample.AverageAcceptedTokens, sample.TokenSampleCount)
	target.AverageCostPerSuccess = mergeAveragedMetric(target.AverageCostPerSuccess, target.CostSampleCount, sample.AverageCostPerSuccess, sample.CostSampleCount)
	target.AverageLatencySeconds = mergeAveragedMetric(target.AverageLatencySeconds, target.LatencySampleCount, sample.AverageLatencySeconds, sample.LatencySampleCount)

	target.SampleCount += sample.SampleCount
	target.SuccessCount += sample.SuccessCount
	target.FirstPassSampleCount += sample.FirstPassSampleCount
	target.FirstPassSuccessCount += sample.FirstPassSuccessCount
	target.RepairAttemptCount += sample.RepairAttemptCount
	target.RepairSuccessCount += sample.RepairSuccessCount
	target.TruncationEventCount += sample.TruncationEventCount
	target.FailureEventCount += sample.FailureEventCount
	target.PromotionAttemptCount += sample.PromotionAttemptCount
	target.PromotionSuccessCount += sample.PromotionSuccessCount
	target.TokenSampleCount += sample.TokenSampleCount
	target.CostSampleCount += sample.CostSampleCount
	target.LatencySampleCount += sample.LatencySampleCount

	*target = normalizeProviderScorecardRates(*target)
}

func mergeAveragedMetric(current float64, currentCount int, sample float64, sampleCount int) float64 {
	if sample <= 0 || sampleCount <= 0 {
		return current
	}
	if current <= 0 || currentCount <= 0 {
		return sample
	}
	return ((current * float64(currentCount)) + (sample * float64(sampleCount))) / float64(currentCount+sampleCount)
}

func normalizeProviderScorecardRates(scorecard ProviderScorecard) ProviderScorecard {
	scorecard.CompilePassRate = safeRatio(scorecard.SuccessCount, scorecard.SampleCount, scorecard.CompilePassRate)
	scorecard.FirstPassVerificationRate = safeRatio(scorecard.FirstPassSuccessCount, scorecard.FirstPassSampleCount, scorecard.FirstPassVerificationRate)
	scorecard.RepairSuccessRate = safeRatio(scorecard.RepairSuccessCount, scorecard.RepairAttemptCount, scorecard.RepairSuccessRate)
	scorecard.TruncationRate = safeRatio(scorecard.TruncationEventCount, scorecard.SampleCount, scorecard.TruncationRate)
	scorecard.FailureClassRecurrence = safeRatio(scorecard.FailureEventCount, scorecard.SampleCount, scorecard.FailureClassRecurrence)
	scorecard.PromotionRate = safeRatio(scorecard.PromotionSuccessCount, scorecard.PromotionAttemptCount, scorecard.PromotionRate)
	return scorecard
}

func topCountedStrings(counts map[string]int, limit int) []string {
	if len(counts) == 0 {
		return nil
	}
	type counted struct {
		Value string
		Count int
	}
	ordered := make([]counted, 0, len(counts))
	for value, count := range counts {
		if strings.TrimSpace(value) == "" || count <= 0 {
			continue
		}
		ordered = append(ordered, counted{Value: value, Count: count})
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Count != ordered[j].Count {
			return ordered[i].Count > ordered[j].Count
		}
		return ordered[i].Value < ordered[j].Value
	})
	values := make([]string, 0, min(len(ordered), limit))
	for _, item := range ordered {
		values = append(values, item.Value)
		if limit > 0 && len(values) >= limit {
			break
		}
	}
	return values
}

func deriveHistoricalAvoidanceTips(failureClasses, warnings []string) []string {
	tips := make([]string, 0, 8)
	for _, failureClass := range failureClasses {
		switch {
		case strings.Contains(failureClass, "contract"):
			tips = append(tips, "Lock API routes, env vars, and runtime commands before generation starts.")
		case strings.Contains(failureClass, "preview"):
			tips = append(tips, "Keep the preview entrypoint, ports, and boot path deterministic before adding surface polish.")
		case strings.Contains(failureClass, "compile"), strings.Contains(failureClass, "verification"):
			tips = append(tips, "Validate imports, dependency manifests, and build commands before broad file rewrites.")
		case strings.Contains(failureClass, "interaction"):
			tips = append(tips, "Re-check primary controls after UI edits so first-click flows stay functional.")
		case strings.Contains(failureClass, "visual"):
			tips = append(tips, "Run screenshot-level layout checks after styling changes to catch contrast and overflow regressions.")
		case strings.Contains(failureClass, "auth"):
			tips = append(tips, "Define auth callback/session strategy and required secrets before wiring protected routes.")
		case strings.Contains(failureClass, "database"):
			tips = append(tips, "Stabilize schema and connection configuration before generating dependent backend handlers.")
		}
	}
	for _, warning := range warnings {
		lower := strings.ToLower(strings.TrimSpace(warning))
		switch {
		case strings.Contains(lower, "api base url"):
			tips = append(tips, "Set an explicit API base URL and CORS contract instead of relying on inferred defaults.")
		case strings.Contains(lower, "runtime/build commands"):
			tips = append(tips, "Set install, build, preview, and start commands explicitly in the build/runtime contract.")
		case strings.Contains(lower, "interaction:"):
			tips = append(tips, "Treat canary interaction warnings as repair inputs before promoting the preview.")
		case strings.Contains(lower, "visual:"):
			tips = append(tips, "Treat advisory visual warnings as part of launch polish, not as optional cleanup.")
		}
	}
	return limitStrings(dedupeStrings(tips), 6)
}

func buildLearningPromptContext(summary *BuildLearningSummary) string {
	if summary == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<historical_build_learning>\n")
	sb.WriteString(fmt.Sprintf("scope: %s\n", strings.TrimSpace(summary.Scope)))
	sb.WriteString(fmt.Sprintf("observed_builds: %d\n", summary.ObservedBuilds))
	if len(summary.SourceBuildIDs) > 0 {
		sb.WriteString("source_build_ids:\n")
		for _, id := range summary.SourceBuildIDs {
			sb.WriteString("- " + strings.TrimSpace(id) + "\n")
		}
	}
	if len(summary.RecurringFailureClasses) > 0 {
		sb.WriteString("recurring_failure_classes:\n")
		for _, class := range summary.RecurringFailureClasses {
			sb.WriteString("- " + strings.TrimSpace(class) + "\n")
		}
	}
	if len(summary.SuccessfulRepairPaths) > 0 {
		sb.WriteString("successful_repair_paths:\n")
		for _, path := range summary.SuccessfulRepairPaths {
			sb.WriteString("- " + strings.TrimSpace(path) + "\n")
		}
	}
	if len(summary.RepairStrategyWinRates) > 0 {
		sb.WriteString("repair_strategy_win_rates:\n")
		for _, strategy := range summary.RepairStrategyWinRates {
			sb.WriteString("- " + strings.TrimSpace(strategy) + "\n")
		}
	}
	if len(summary.SemanticRepairHints) > 0 {
		sb.WriteString("semantic_repair_hints:\n")
		for _, hint := range summary.SemanticRepairHints {
			sb.WriteString("- " + strings.TrimSpace(hint) + "\n")
		}
	}
	if len(summary.FrequentWarnings) > 0 {
		sb.WriteString("frequent_warnings:\n")
		for _, warning := range summary.FrequentWarnings {
			sb.WriteString("- " + strings.TrimSpace(warning) + "\n")
		}
	}
	if len(summary.HotspotFiles) > 0 {
		sb.WriteString("hotspot_files:\n")
		for _, path := range summary.HotspotFiles {
			sb.WriteString("- " + strings.TrimSpace(path) + "\n")
		}
	}
	if len(summary.CleanPassSignals) > 0 {
		sb.WriteString("clean_pass_signals:\n")
		for _, signal := range summary.CleanPassSignals {
			sb.WriteString("- " + strings.TrimSpace(signal) + "\n")
		}
	}
	if len(summary.RecommendedAvoidance) > 0 {
		sb.WriteString("recommended_avoidance:\n")
		for _, tip := range summary.RecommendedAvoidance {
			sb.WriteString("- " + strings.TrimSpace(tip) + "\n")
		}
	}
	sb.WriteString("Use these lessons to avoid repeating known failure classes and to preserve prior clean passes.\n")
	sb.WriteString("</historical_build_learning>\n")
	return sb.String()
}
