package agents

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type BuildSpecSignalSeverity string

const (
	BuildSpecSeverityInfo    BuildSpecSignalSeverity = "info"
	BuildSpecSeverityWarning BuildSpecSignalSeverity = "warning"
)

type BuildSpecAdvisory struct {
	Code           string                  `json:"code"`
	Severity       BuildSpecSignalSeverity `json:"severity"`
	Surface        ContractSurface         `json:"surface"`
	Summary        string                  `json:"summary"`
	Recommendation string                  `json:"recommendation,omitempty"`
}

type ValidatedBuildSpec struct {
	ID                    string              `json:"id"`
	BuildID               string              `json:"build_id,omitempty"`
	Source                string              `json:"source"`
	NormalizedRequest     string              `json:"normalized_request,omitempty"`
	AppType               string              `json:"app_type,omitempty"`
	DeliveryMode          string              `json:"delivery_mode,omitempty"`
	PrimaryUserFlows      []string            `json:"primary_user_flows,omitempty"`
	RoutePlan             []string            `json:"route_plan,omitempty"`
	StateDomains          []string            `json:"state_domains,omitempty"`
	APIPaths              []string            `json:"api_paths,omitempty"`
	SecurityAdvisories    []BuildSpecAdvisory `json:"security_advisories,omitempty"`
	PerformanceAdvisories []BuildSpecAdvisory `json:"performance_advisories,omitempty"`
	AcceptanceSurfaces    []string            `json:"acceptance_surfaces,omitempty"`
	Locked                bool                `json:"locked"`
	CreatedAt             time.Time           `json:"created_at"`
}

func compilePrecomputedValidatedBuildSpec(req *BuildRequest, intent *IntentBrief) *ValidatedBuildSpec {
	if req == nil && intent == nil {
		return nil
	}

	normalized := ""
	appType := ""
	if intent != nil {
		normalized = strings.TrimSpace(intent.NormalizedRequest)
		appType = strings.TrimSpace(intent.AppType)
	}
	if normalized == "" && req != nil {
		normalized = normalizeCompactText(strings.Join(compactNonEmptyIntentParts([]string{
			strings.TrimSpace(req.WireframeDescription),
			firstNonEmpty(req.Prompt, req.Description),
		}), "\n\n"))
	}
	if appType == "" && req != nil {
		appType = inferIntentAppType(normalized, req.TechStack)
	}
	if normalized == "" && appType == "" {
		return nil
	}

	capabilities := detectRequiredCapabilities(normalized, nil)
	if intent != nil && len(intent.RequiredCapabilities) > 0 {
		capabilities = dedupeCapabilities(append(capabilities, intent.RequiredCapabilities...))
	}
	spec := &ValidatedBuildSpec{
		ID:                    uuid.New().String(),
		Source:                "precompute_request_v1",
		NormalizedRequest:     normalized,
		AppType:               appType,
		DeliveryMode:          defaultDeliveryModeForAppType(appType),
		PrimaryUserFlows:      deriveValidatedUserFlows(normalized, capabilities),
		StateDomains:          deriveValidatedStateDomains(capabilities, normalized),
		SecurityAdvisories:    deriveValidatedSecurityAdvisories(capabilities, normalized),
		PerformanceAdvisories: deriveValidatedPerformanceAdvisories(appType, capabilities, normalized),
		CreatedAt:             time.Now().UTC(),
	}
	spec.AcceptanceSurfaces = deriveValidatedAcceptanceSurfaces(nil, spec.DeliveryMode)
	return spec
}

func finalizeValidatedBuildSpec(buildID string, existing *ValidatedBuildSpec, plan *BuildPlan, contract *BuildContract) *ValidatedBuildSpec {
	if existing == nil && plan == nil && contract == nil {
		return nil
	}

	spec := &ValidatedBuildSpec{
		ID:        uuid.New().String(),
		BuildID:   buildID,
		Source:    "validated_plan_v1",
		Locked:    true,
		CreatedAt: time.Now().UTC(),
	}
	if existing != nil {
		*spec = *existing
		spec.BuildID = buildID
		spec.Source = "validated_plan_v1"
		spec.Locked = true
		if spec.CreatedAt.IsZero() {
			spec.CreatedAt = time.Now().UTC()
		}
	}

	if plan != nil {
		if strings.TrimSpace(plan.AppType) != "" {
			spec.AppType = strings.TrimSpace(plan.AppType)
		}
		if strings.TrimSpace(plan.DeliveryMode) != "" {
			spec.DeliveryMode = strings.TrimSpace(plan.DeliveryMode)
		}
		spec.PrimaryUserFlows = dedupeStrings(append(spec.PrimaryUserFlows, derivePlanUserFlows(plan)...))
		spec.StateDomains = dedupeStrings(append(spec.StateDomains, derivePlanStateDomains(plan)...))
		spec.RoutePlan = dedupeStrings(append(spec.RoutePlan, derivePlanRoutePlan(plan)...))
	}

	if contract != nil {
		if strings.TrimSpace(contract.AppType) != "" {
			spec.AppType = strings.TrimSpace(contract.AppType)
		}
		if strings.TrimSpace(contract.DeliveryMode) != "" {
			spec.DeliveryMode = strings.TrimSpace(contract.DeliveryMode)
		}
		spec.RoutePlan = dedupeStrings(append(spec.RoutePlan, deriveContractRoutePlan(contract)...))
		spec.APIPaths = dedupeStrings(append(spec.APIPaths, deriveContractAPIPaths(contract)...))
		spec.AcceptanceSurfaces = deriveValidatedAcceptanceSurfaces(contract, spec.DeliveryMode)
	}

	spec.SecurityAdvisories = dedupeBuildSpecAdvisories(spec.SecurityAdvisories)
	spec.PerformanceAdvisories = dedupeBuildSpecAdvisories(spec.PerformanceAdvisories)
	if len(spec.AcceptanceSurfaces) == 0 {
		spec.AcceptanceSurfaces = deriveValidatedAcceptanceSurfaces(contract, spec.DeliveryMode)
	}
	return spec
}

func deriveValidatedUserFlows(normalized string, capabilities []CapabilityRequirement) []string {
	flows := []string{"land in the product shell and reach an interactive preview on first pass"}
	if capabilityRequired(&IntentBrief{RequiredCapabilities: capabilities}, CapabilityAuth) {
		flows = append(flows, "authenticate and restore a truthful signed-in application state")
	}
	if capabilityRequired(&IntentBrief{RequiredCapabilities: capabilities}, CapabilityFileUpload) {
		flows = append(flows, "select a file, validate it client-side, and show upload lifecycle feedback")
	}
	if strings.Contains(strings.ToLower(normalized), "dashboard") {
		flows = append(flows, "see a populated dashboard with empty, loading, and success states")
	}
	return dedupeStrings(flows)
}

func deriveValidatedStateDomains(capabilities []CapabilityRequirement, normalized string) []string {
	domains := []string{"routing", "ui_feedback", "network_state"}
	if capabilityRequired(&IntentBrief{RequiredCapabilities: capabilities}, CapabilityAuth) {
		domains = append(domains, "auth_session")
	}
	if capabilityRequired(&IntentBrief{RequiredCapabilities: capabilities}, CapabilityDatabase) {
		domains = append(domains, "resource_collection")
	}
	if capabilityRequired(&IntentBrief{RequiredCapabilities: capabilities}, CapabilityFileUpload) {
		domains = append(domains, "upload_queue")
	}
	if strings.Contains(strings.ToLower(normalized), "search") {
		domains = append(domains, "search_query")
	}
	return dedupeStrings(domains)
}

func deriveValidatedSecurityAdvisories(capabilities []CapabilityRequirement, normalized string) []BuildSpecAdvisory {
	advisories := make([]BuildSpecAdvisory, 0, 6)
	add := func(code string, surface ContractSurface, summary, recommendation string) {
		advisories = append(advisories, BuildSpecAdvisory{
			Code:           code,
			Severity:       BuildSpecSeverityWarning,
			Surface:        surface,
			Summary:        summary,
			Recommendation: recommendation,
		})
	}

	intent := &IntentBrief{RequiredCapabilities: capabilities}
	normalizedLower := strings.ToLower(normalized)
	if capabilityRequired(intent, CapabilityAuth) {
		add("auth_session_hardening", SurfaceBackend, "Auth-capable builds must define secure session boundaries before runtime code lands.", "Freeze the auth contract, include logout/session refresh routes, and prefer httpOnly cookie or clearly scoped bearer token handling.")
	}
	if capabilityRequired(intent, CapabilityAuth) && (strings.Contains(normalizedLower, "role") || strings.Contains(normalizedLower, "rbac") || strings.Contains(normalizedLower, "admin")) {
		add("role_boundary_enforcement", SurfaceBackend, "Role-based surfaces need explicit authorization boundaries for every privileged action.", "Lock admin/manager/member permissions in the contract and require server-side enforcement for every write path.")
	}
	if capabilityRequired(intent, CapabilityFileUpload) {
		add("upload_validation", SurfaceBackend, "Upload flows need strict MIME, size, and storage validation.", "Require client + server validation, sanitize filenames, and avoid trusting browser-provided content type.")
	}
	if capabilityRequired(intent, CapabilityExternalAPI) {
		add("external_api_guardrails", SurfaceIntegration, "External API integrations need timeout, retry, and secret-isolation rules.", "Keep provider secrets server-side, define error budgets, and specify degraded states in the UI contract.")
	}
	if capabilityRequired(intent, CapabilityBilling) || strings.Contains(normalizedLower, "payment") {
		add("billing_truthfulness", SurfaceBackend, "Billing-related flows must be truthful about entitlement and payment status.", "Ensure the UI never implies successful purchase or upgraded capability until backend verification confirms it.")
	}
	if capabilityRequired(intent, CapabilityBilling) || strings.Contains(normalizedLower, "stripe") || strings.Contains(normalizedLower, "subscription") || strings.Contains(normalizedLower, "checkout") {
		add("billing_webhook_verification", SurfaceIntegration, "Payment-capable apps need webhook verification and idempotent entitlement updates.", "Specify the webhook source of truth up front and never grant access from client redirects alone.")
	}
	if capabilityRequired(intent, CapabilityDatabase) && (strings.Contains(normalizedLower, "tenant") || strings.Contains(normalizedLower, "workspace") || strings.Contains(normalizedLower, "organization") || strings.Contains(normalizedLower, "multi-tenant")) {
		add("tenant_isolation", SurfaceBackend, "Multi-tenant data models need explicit tenant isolation at query and mutation boundaries.", "Freeze tenant/workspace ownership fields in the schema and require every backend read/write path to scope by tenant.")
	}
	if strings.Contains(normalizedLower, "ai") || strings.Contains(normalizedLower, "llm") || strings.Contains(normalizedLower, "assistant") || strings.Contains(normalizedLower, "chat") {
		add("ai_prompt_boundary", SurfaceIntegration, "AI-assisted features need prompt-injection and data-exfiltration boundaries.", "Keep system prompts and secrets server-side, sanitize retrieved context, and define what user content is allowed to reach model calls.")
	}
	return dedupeBuildSpecAdvisories(advisories)
}

func deriveValidatedPerformanceAdvisories(appType string, capabilities []CapabilityRequirement, normalized string) []BuildSpecAdvisory {
	advisories := make([]BuildSpecAdvisory, 0, 6)
	add := func(code string, surface ContractSurface, summary, recommendation string) {
		advisories = append(advisories, BuildSpecAdvisory{
			Code:           code,
			Severity:       BuildSpecSeverityInfo,
			Surface:        surface,
			Summary:        summary,
			Recommendation: recommendation,
		})
	}

	intent := &IntentBrief{RequiredCapabilities: capabilities}
	normalizedLower := strings.ToLower(normalized)
	switch strings.TrimSpace(strings.ToLower(appType)) {
	case "fullstack", "web":
		add("preview_first_render", SurfaceFrontend, "First preview should render useful UI immediately without waiting on backend completion.", "Ship optimistic shell state first, then hydrate data surfaces behind truthful loading states.")
	}
	if capabilityRequired(intent, CapabilitySearch) {
		add("search_debounce", SurfaceFrontend, "Search-heavy experiences need debounced queries and empty/loading states.", "Debounce client queries and avoid firing backend requests on every keystroke.")
	}
	if capabilityRequired(intent, CapabilityRealtime) {
		add("realtime_backpressure", SurfaceIntegration, "Realtime surfaces need backpressure-aware state updates.", "Batch or throttle feed updates so the first preview remains smooth under event bursts.")
	}
	if strings.Contains(normalizedLower, "dashboard") || strings.Contains(normalizedLower, "table") {
		add("list_scaling", SurfaceFrontend, "Data-dense surfaces should avoid heavy first paint and jitter.", "Use pagination or list virtualization patterns once collection sizes grow beyond simple card grids.")
	}
	if strings.Contains(normalizedLower, "dashboard") || strings.Contains(normalizedLower, "analytics") || strings.Contains(normalizedLower, "chart") {
		add("progressive_dashboard_loading", SurfaceFrontend, "Dashboard-style apps should reveal value before every widget finishes loading.", "Prioritize hero KPIs, stagger secondary widgets, and avoid blocking first paint on full analytics hydration.")
	}
	if strings.Contains(normalizedLower, "feed") || strings.Contains(normalizedLower, "activity") || capabilityRequired(intent, CapabilityRealtime) {
		add("feed_windowing", SurfaceFrontend, "Feed-oriented surfaces need bounded rendering to keep preview smooth.", "Window long activity lists and append new events in batches instead of rerendering the full feed on every update.")
	}
	if capabilityRequired(intent, CapabilityExternalAPI) || strings.Contains(normalizedLower, "ai") || strings.Contains(normalizedLower, "assistant") {
		add("upstream_latency_budget", SurfaceIntegration, "Apps that depend on remote providers need explicit latency budgets and graceful fallbacks.", "Cache stable responses, parallelize independent calls, and keep the first render useful when upstream providers are slow.")
	}
	return dedupeBuildSpecAdvisories(advisories)
}

func derivePlanUserFlows(plan *BuildPlan) []string {
	if plan == nil {
		return nil
	}
	flows := make([]string, 0, len(plan.Features))
	for _, feature := range plan.Features {
		name := strings.TrimSpace(feature.Name)
		desc := strings.TrimSpace(feature.Description)
		switch {
		case name != "" && desc != "":
			flows = append(flows, fmt.Sprintf("%s — %s", name, desc))
		case name != "":
			flows = append(flows, name)
		}
	}
	return dedupeStrings(flows)
}

func derivePlanStateDomains(plan *BuildPlan) []string {
	if plan == nil {
		return nil
	}
	domains := make([]string, 0, len(plan.Components)+len(plan.DataModels))
	for _, component := range plan.Components {
		for _, state := range component.State {
			if trimmed := strings.TrimSpace(state); trimmed != "" {
				domains = append(domains, trimmed)
			}
		}
	}
	for _, model := range plan.DataModels {
		if name := strings.TrimSpace(model.Name); name != "" {
			domains = append(domains, strings.ToLower(name))
		}
	}
	return dedupeStrings(domains)
}

func derivePlanRoutePlan(plan *BuildPlan) []string {
	if plan == nil {
		return nil
	}
	routes := make([]string, 0, len(plan.Files))
	for _, file := range plan.Files {
		path := filepath.ToSlash(strings.TrimSpace(file.Path))
		if strings.HasPrefix(path, "src/pages/") || strings.HasPrefix(path, "app/") {
			routes = append(routes, path)
		}
	}
	sort.Strings(routes)
	return dedupeStrings(routes)
}

func deriveContractRoutePlan(contract *BuildContract) []string {
	if contract == nil {
		return nil
	}
	routes := make([]string, 0, len(contract.RoutePageMap))
	for _, route := range contract.RoutePageMap {
		parts := compactNonEmptyIntentParts([]string{strings.TrimSpace(route.Path), strings.TrimSpace(route.File)})
		if len(parts) > 0 {
			routes = append(routes, strings.Join(parts, " -> "))
		}
	}
	return dedupeStrings(routes)
}

func deriveContractAPIPaths(contract *BuildContract) []string {
	if contract == nil || contract.APIContract == nil {
		return nil
	}
	paths := make([]string, 0, len(contract.APIContract.Endpoints))
	for _, endpoint := range contract.APIContract.Endpoints {
		method := strings.TrimSpace(endpoint.Method)
		path := strings.TrimSpace(endpoint.Path)
		if method == "" && path == "" {
			continue
		}
		paths = append(paths, strings.TrimSpace(strings.Join(compactNonEmptyIntentParts([]string{method, path}), " ")))
	}
	return dedupeStrings(paths)
}

func deriveValidatedAcceptanceSurfaces(contract *BuildContract, deliveryMode string) []string {
	set := map[string]struct{}{
		string(SurfaceFrontend): {},
	}
	if contract != nil {
		for _, surface := range contract.AcceptanceBySurface {
			if key := strings.TrimSpace(string(surface.Surface)); key != "" {
				set[key] = struct{}{}
			}
		}
	}
	switch strings.TrimSpace(strings.ToLower(deliveryMode)) {
	case "full_stack_preview", "api_runtime":
		set[string(SurfaceBackend)] = struct{}{}
	case "frontend_preview_only", "frontend_preview":
		// frontend-only mode intentionally omits backend surface from acceptance.
	}

	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func dedupeBuildSpecAdvisories(values []BuildSpecAdvisory) []BuildSpecAdvisory {
	if len(values) == 0 {
		return nil
	}
	out := make([]BuildSpecAdvisory, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, advisory := range values {
		key := strings.TrimSpace(advisory.Code) + "|" + strings.TrimSpace(string(advisory.Surface)) + "|" + strings.TrimSpace(advisory.Summary)
		if key == "||" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, advisory)
	}
	return out
}

func validatedBuildSpecPromptContext(spec *ValidatedBuildSpec) string {
	if spec == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n<validated_build_spec>\n")
	if req := strings.TrimSpace(spec.NormalizedRequest); req != "" {
		sb.WriteString("Normalized request: " + req + "\n")
	}
	if appType := strings.TrimSpace(spec.AppType); appType != "" {
		sb.WriteString("App type: " + appType + "\n")
	}
	if delivery := strings.TrimSpace(spec.DeliveryMode); delivery != "" {
		sb.WriteString("Delivery mode: " + delivery + "\n")
	}
	if len(spec.PrimaryUserFlows) > 0 {
		sb.WriteString("Primary user flows:\n")
		for _, flow := range spec.PrimaryUserFlows {
			sb.WriteString("- " + flow + "\n")
		}
	}
	if len(spec.RoutePlan) > 0 {
		sb.WriteString("Route plan:\n")
		for _, route := range spec.RoutePlan {
			sb.WriteString("- " + route + "\n")
		}
	}
	if len(spec.APIPaths) > 0 {
		sb.WriteString("API paths:\n")
		for _, apiPath := range spec.APIPaths {
			sb.WriteString("- " + apiPath + "\n")
		}
	}
	if len(spec.StateDomains) > 0 {
		sb.WriteString("State domains: " + strings.Join(spec.StateDomains, ", ") + "\n")
	}
	if len(spec.SecurityAdvisories) > 0 {
		sb.WriteString("Security advisories:\n")
		for _, advisory := range spec.SecurityAdvisories {
			sb.WriteString(fmt.Sprintf("- [%s] %s — %s\n", advisory.Surface, advisory.Summary, advisory.Recommendation))
		}
	}
	if len(spec.PerformanceAdvisories) > 0 {
		sb.WriteString("Performance advisories:\n")
		for _, advisory := range spec.PerformanceAdvisories {
			sb.WriteString(fmt.Sprintf("- [%s] %s — %s\n", advisory.Surface, advisory.Summary, advisory.Recommendation))
		}
	}
	if len(spec.AcceptanceSurfaces) > 0 {
		sb.WriteString("Acceptance surfaces: " + strings.Join(spec.AcceptanceSurfaces, ", ") + "\n")
	}
	if spec.Locked {
		sb.WriteString("This spec is locked for generation. Do not redesign it mid-task.\n")
	}
	sb.WriteString("</validated_build_spec>\n")
	return sb.String()
}
