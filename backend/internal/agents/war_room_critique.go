package agents

import (
	"strings"
)

type buildSpecCritiqueCategory string

const (
	buildSpecCritiqueSecurity    buildSpecCritiqueCategory = "security"
	buildSpecCritiquePerformance buildSpecCritiqueCategory = "performance"
)

type buildSpecCritiqueIssue struct {
	Code           string
	Category       buildSpecCritiqueCategory
	Surface        ContractSurface
	Summary        string
	Recommendation string
}

func critiqueDraftBuildSpec(spec *ValidatedBuildSpec, contract *BuildContract) []buildSpecCritiqueIssue {
	if spec == nil {
		return nil
	}

	issues := make([]buildSpecCritiqueIssue, 0, 6)
	add := func(code string, category buildSpecCritiqueCategory, surface ContractSurface, summary, recommendation string) {
		issues = append(issues, buildSpecCritiqueIssue{
			Code:           code,
			Category:       category,
			Surface:        surface,
			Summary:        summary,
			Recommendation: recommendation,
		})
	}

	deliveryMode := strings.TrimSpace(strings.ToLower(spec.DeliveryMode))
	appType := strings.TrimSpace(strings.ToLower(spec.AppType))

	if (deliveryMode == "full_stack_preview" || deliveryMode == "api_runtime" || appType == "fullstack") && len(spec.APIPaths) == 0 {
		add(
			"war_room_api_plan_gap",
			buildSpecCritiqueSecurity,
			SurfaceIntegration,
			"Validated spec has backend/runtime expectations but no API path plan.",
			"Freeze the expected API routes before generation so frontend/backend integration remains truthful.",
		)
	}

	if (appType == "fullstack" || appType == "web") && len(spec.RoutePlan) == 0 {
		add(
			"war_room_route_plan_gap",
			buildSpecCritiquePerformance,
			SurfaceFrontend,
			"Validated spec is missing a concrete route plan for preview-visible surfaces.",
			"Add at least one concrete route/component target so generation does not improvise navigation.",
		)
	}

	if len(spec.PrimaryUserFlows) == 0 {
		add(
			"war_room_user_flow_gap",
			buildSpecCritiquePerformance,
			SurfaceGlobal,
			"Validated spec is missing explicit primary user flows.",
			"Define at least one critical flow to anchor implementation and verification tasks.",
		)
	}

	if contract != nil && contract.AuthContract != nil && contract.AuthContract.Required && !buildSpecHasAdvisoryCode(spec.SecurityAdvisories, "auth_session_hardening") {
		add(
			"war_room_auth_boundary_gap",
			buildSpecCritiqueSecurity,
			SurfaceBackend,
			"Auth contract is required but the validated spec does not call out auth session hardening.",
			"Require callback/session/token strategy guardrails in the validated spec before coding tasks start.",
		)
	}

	if containsString(spec.AcceptanceSurfaces, string(SurfaceBackend)) && len(spec.APIPaths) == 0 {
		add(
			"war_room_backend_acceptance_without_api",
			buildSpecCritiqueSecurity,
			SurfaceBackend,
			"Backend acceptance is required but no backend API surface is listed in the validated spec.",
			"Add backend API paths or explicitly scope backend acceptance to non-HTTP runtime checks.",
		)
	}

	return dedupeBuildSpecCritiqueIssues(issues)
}

func applyWarRoomCritiqueAdvisories(spec *ValidatedBuildSpec, issues []buildSpecCritiqueIssue) {
	if spec == nil || len(issues) == 0 {
		return
	}

	for _, issue := range issues {
		advisory := BuildSpecAdvisory{
			Code:           issue.Code,
			Severity:       BuildSpecSeverityWarning,
			Surface:        issue.Surface,
			Summary:        issue.Summary,
			Recommendation: issue.Recommendation,
		}

		switch issue.Category {
		case buildSpecCritiquePerformance:
			spec.PerformanceAdvisories = append(spec.PerformanceAdvisories, advisory)
		default:
			spec.SecurityAdvisories = append(spec.SecurityAdvisories, advisory)
		}
	}
}

func dedupeBuildSpecCritiqueIssues(values []buildSpecCritiqueIssue) []buildSpecCritiqueIssue {
	if len(values) == 0 {
		return nil
	}
	out := make([]buildSpecCritiqueIssue, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, issue := range values {
		key := strings.TrimSpace(issue.Code) + "|" + strings.TrimSpace(string(issue.Surface))
		if key == "|" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, issue)
	}
	return out
}

func buildSpecHasAdvisoryCode(values []BuildSpecAdvisory, code string) bool {
	for _, value := range values {
		if value.Code == code {
			return true
		}
	}
	return false
}
