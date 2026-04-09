package agents

import (
	"sort"
	"strings"
	"time"
)

func refreshDerivedReliabilitySummaryLocked(build *Build, state *BuildSnapshotState, orchestration *BuildOrchestrationState) {
	if build == nil || state == nil || orchestration == nil {
		return
	}
	orchestration.ReliabilitySummary = deriveBuildReliabilitySummary(build, state, orchestration)
}

func deriveBuildReliabilitySummary(build *Build, state *BuildSnapshotState, orchestration *BuildOrchestrationState) *BuildReliabilitySummary {
	if build == nil || state == nil || orchestration == nil {
		return nil
	}

	latestReports := latestVerificationReports(orchestration.VerificationReports)
	advisoryClasses, issues := deriveReliabilityAdvisories(latestReports)
	recurring := deriveRecurringFailureClasses(orchestration.FailureFingerprints)

	status := "clean"
	currentCategory := BuildFailureCategory("")
	currentClass := ""
	if state.FailureTaxonomy != nil {
		currentCategory = state.FailureTaxonomy.CurrentCategory
		currentClass = strings.TrimSpace(state.FailureTaxonomy.CurrentClass)
	}

	hasBlockedReport := false
	hasFailedReport := false
	for _, report := range latestReports {
		switch report.Status {
		case VerificationBlocked:
			hasBlockedReport = true
		case VerificationFailed:
			hasFailedReport = true
		}
	}

	switch {
	case hasBlockedReport || build.Status == BuildFailed:
		status = "blocked"
	case currentClass != "" || currentCategory != "" || hasFailedReport:
		status = "degraded"
	case len(advisoryClasses) > 0:
		status = "advisory"
	}

	recommendedFocus := deriveReliabilityRecommendedFocus(status, currentCategory, currentClass, advisoryClasses, recurring)

	acceptanceSurfaces := []string(nil)
	primaryUserFlows := []string(nil)
	if orchestration.ValidatedBuildSpec != nil {
		acceptanceSurfaces = append([]string(nil), orchestration.ValidatedBuildSpec.AcceptanceSurfaces...)
		primaryUserFlows = append([]string(nil), orchestration.ValidatedBuildSpec.PrimaryUserFlows...)
	}
	if len(acceptanceSurfaces) == 0 && orchestration.BuildContract != nil {
		acceptanceSurfaces = deriveValidatedAcceptanceSurfaces(orchestration.BuildContract, orchestration.BuildContract.DeliveryMode)
	}

	return &BuildReliabilitySummary{
		Status:                 status,
		CurrentFailureCategory: currentCategory,
		CurrentFailureClass:    currentClass,
		AdvisoryClasses:        advisoryClasses,
		RecurringFailureClass:  recurring,
		TopIssues:              limitStrings(issues, 6),
		RecommendedFocus:       recommendedFocus,
		AcceptanceSurfaces:     acceptanceSurfaces,
		PrimaryUserFlows:       primaryUserFlows,
		GeneratedAt:            time.Now().UTC(),
	}
}

func deriveReliabilityAdvisories(reports []VerificationReport) ([]string, []string) {
	if len(reports) == 0 {
		return nil, nil
	}
	classes := make([]string, 0, 4)
	issues := make([]string, 0, 8)
	for _, report := range reports {
		if report.Status != VerificationPassed {
			continue
		}
		for _, warning := range report.Warnings {
			trimmed := strings.TrimSpace(warning)
			switch {
			case strings.HasPrefix(trimmed, "visual:"):
				classes = append(classes, "visual_layout")
				issues = append(issues, trimmed)
			case strings.HasPrefix(trimmed, "interaction:"):
				classes = append(classes, "interaction_canary")
				issues = append(issues, trimmed)
			}
		}
	}
	return dedupeStrings(classes), dedupeStrings(issues)
}

func deriveRecurringFailureClasses(fingerprints []FailureFingerprint) []string {
	if len(fingerprints) == 0 {
		return nil
	}
	counts := make(map[string]int, len(fingerprints))
	for _, fp := range fingerprints {
		class := strings.TrimSpace(fp.FailureClass)
		if class == "" || class == "build_failure" {
			continue
		}
		counts[class]++
	}
	if len(counts) == 0 {
		return nil
	}
	type countedClass struct {
		class string
		count int
	}
	ordered := make([]countedClass, 0, len(counts))
	for class, count := range counts {
		if count > 1 {
			ordered = append(ordered, countedClass{class: class, count: count})
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].count != ordered[j].count {
			return ordered[i].count > ordered[j].count
		}
		return ordered[i].class < ordered[j].class
	})
	out := make([]string, 0, len(ordered))
	for _, item := range ordered {
		out = append(out, item.class)
		if len(out) >= 4 {
			break
		}
	}
	return out
}

func deriveReliabilityRecommendedFocus(status string, category BuildFailureCategory, class string, advisoryClasses, recurring []string) []string {
	reasons := make([]string, 0, 4)
	switch status {
	case "blocked":
		reasons = append(reasons, "clear the blocking verification/runtime issue before promotion")
	case "degraded":
		switch category {
		case FailureCategoryCompile:
			reasons = append(reasons, "expand deterministic compile repair coverage for the current failure class")
		case FailureCategoryContract:
			reasons = append(reasons, "tighten spec and contract normalization before generation starts")
		case FailureCategoryPreviewBoot:
			reasons = append(reasons, "stabilize preview boot/runtime setup before additional feature work")
		case FailureCategoryRuntime:
			reasons = append(reasons, "repair backend/runtime integration before calling the build production-ready")
		default:
			reasons = append(reasons, "investigate the current degradation path before broader rollout")
		}
	case "advisory":
		if containsString(advisoryClasses, "visual_layout") {
			reasons = append(reasons, "polish visual hierarchy and screenshot-level quality before promotion")
		}
		if containsString(advisoryClasses, "interaction_canary") {
			reasons = append(reasons, "stabilize first-click interaction paths in preview")
		}
	default:
		reasons = append(reasons, "continue running canaries on this build shape")
	}

	if class != "" && status != "clean" {
		reasons = append(reasons, "watch the current failure class: "+class)
	}
	if len(recurring) > 0 {
		reasons = append(reasons, "reduce recurring issues in: "+strings.Join(recurring, ", "))
	}
	return dedupeStrings(reasons)
}

func limitStrings(values []string, limit int) []string {
	if len(values) == 0 {
		return nil
	}
	if limit <= 0 || len(values) <= limit {
		return append([]string(nil), values...)
	}
	return append([]string(nil), values[:limit]...)
}
