package agents

import (
	"fmt"
	"strings"
	"time"
)

func buildSubscriptionPlan(build *Build) string {
	if build == nil {
		return "free"
	}
	if plan := strings.TrimSpace(strings.ToLower(build.SubscriptionPlan)); plan != "" {
		return plan
	}
	if build.SnapshotState.PolicyState != nil {
		if plan := strings.TrimSpace(strings.ToLower(build.SnapshotState.PolicyState.PlanType)); plan != "" {
			return plan
		}
	}
	return "free"
}

func buildRequiresPaidFeatures(build *Build) (bool, string) {
	if build == nil {
		return false, ""
	}
	return buildSubscriptionRequirement(&BuildRequest{
		Description: build.Description,
		Prompt:      build.Description,
		TechStack:   build.TechStack,
	})
}

func buildCapabilityState(build *Build) *BuildCapabilityState {
	if build == nil {
		return nil
	}

	var required []CapabilityRequirement
	requiresPaidFeatures, _ := buildRequiresPaidFeatures(build)
	appType := ""
	if orchestration := build.SnapshotState.Orchestration; orchestration != nil && orchestration.IntentBrief != nil && len(orchestration.IntentBrief.RequiredCapabilities) > 0 {
		required = orchestration.IntentBrief.RequiredCapabilities
		appType = strings.TrimSpace(strings.ToLower(orchestration.IntentBrief.AppType))
	} else {
		required = detectRequiredCapabilities(build.Description, build.TechStack)
	}
	if appType == "" {
		appType = strings.TrimSpace(strings.ToLower(inferIntentAppType(build.Description, build.TechStack)))
	}

	filtered := make([]CapabilityRequirement, 0, len(required))
	for _, capability := range required {
		if capability == CapabilityAPI && !requiresPaidFeatures && appType == "web" {
			continue
		}
		filtered = append(filtered, capability)
	}
	required = filtered

	state := &BuildCapabilityState{
		RequiredCapabilities: capabilityStrings(required),
	}
	for _, capability := range required {
		switch capability {
		case CapabilityAuth:
			state.RequiresAuth = true
			state.RequiresBackendRuntime = true
		case CapabilityDatabase:
			state.RequiresDatabase = true
			state.RequiresBackendRuntime = true
		case CapabilityStorage, CapabilityFileUpload:
			state.RequiresStorage = true
			state.RequiresBackendRuntime = true
		case CapabilityExternalAPI:
			state.RequiresExternalAPI = true
		case CapabilityBilling:
			state.RequiresBilling = true
			state.RequiresExternalAPI = true
			state.RequiresBackendRuntime = true
		case CapabilityRealtime:
			state.RequiresRealtime = true
			state.RequiresBackendRuntime = true
		case CapabilityJobs:
			state.RequiresJobs = true
			state.RequiresBackendRuntime = true
		case CapabilityAPI:
			if requiresPaidFeatures || (build.TechStack != nil && strings.TrimSpace(build.TechStack.Backend) != "") {
				state.RequiresBackendRuntime = true
			}
		}
	}

	description := " " + strings.ToLower(normalizeCompactText(build.Description)) + " "
	if strings.Contains(description, " publish ") || strings.Contains(description, " deploy ") || strings.Contains(description, " production ") {
		state.RequiresPublish = true
	}
	if build.RequirePreviewReady && build.TechStack != nil && strings.TrimSpace(build.TechStack.Backend) != "" {
		state.RequiresBackendRuntime = true
	}
	if build.ProviderMode == "byok" || strings.Contains(description, " byok ") || strings.Contains(description, " bring your own key ") || strings.Contains(description, " own api key ") {
		state.RequiresBYOK = true
	}

	return state
}

func buildPolicyState(build *Build, capabilityState *BuildCapabilityState) *BuildPolicyState {
	planType := buildSubscriptionPlan(build)
	requiresPaidFeatures, upgradeReason := buildRequiresPaidFeatures(build)

	classification := BuildClassificationStaticReady
	requiredPlan := ""
	if requiresPaidFeatures {
		if isPaidBuildPlan(planType) {
			classification = BuildClassificationFullStackCandidate
		} else {
			classification = BuildClassificationUpgradeRequired
			requiredPlan = "builder"
		}
	}

	return &BuildPolicyState{
		PlanType:           planType,
		Classification:     classification,
		UpgradeRequired:    classification == BuildClassificationUpgradeRequired,
		UpgradeReason:      upgradeReason,
		RequiredPlan:       requiredPlan,
		StaticFrontendOnly: classification == BuildClassificationStaticReady,
		FullStackEligible:  isPaidBuildPlan(planType),
		PublishEnabled:     isPaidBuildPlan(planType),
		BYOKEnabled:        isPaidBuildPlan(planType),
	}
}

func blockerCategoryForPermissionScope(scope BuildPermissionScope) BuildBlockerCategory {
	switch scope {
	case PermissionScopeNetwork, PermissionScopeService:
		return BlockerCategoryExternalAccess
	case PermissionScopeFilesystem, PermissionScopeProgram:
		return BlockerCategoryEnvironment
	default:
		return BlockerCategoryApprovals
	}
}

func deriveBuildBlockers(build *Build, policy *BuildPolicyState) []BuildBlocker {
	if build == nil {
		return nil
	}

	now := build.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	blockers := make([]BuildBlocker, 0)
	seen := map[string]bool{}
	appendBlocker := func(blocker BuildBlocker) {
		if blocker.ID == "" || seen[blocker.ID] {
			return
		}
		seen[blocker.ID] = true
		blockers = append(blockers, blocker)
	}

	if policy != nil && policy.UpgradeRequired {
		appendBlocker(BuildBlocker{
			ID:                     "plan-upgrade-required",
			Title:                  "Upgrade required for full-stack work",
			Type:                   "plan_upgrade_required",
			Category:               BlockerCategoryPlanTier,
			Severity:               BlockerSeverityBlocking,
			WhoMustAct:             "user",
			Summary:                fmt.Sprintf("This request needs %s, which is locked on the %s plan.", firstNonEmptyString(policy.UpgradeReason, "paid capabilities"), firstNonEmptyString(policy.PlanType, "free")),
			UnblocksWith:           "Upgrade to Builder or higher, or continue in honest static-only mode.",
			PartialProgressAllowed: true,
			PlanTierRelated:        true,
		})
	}

	interaction := build.Interaction
	if strings.TrimSpace(interaction.PendingQuestion) != "" {
		appendBlocker(BuildBlocker{
			ID:                     "pending-user-reply",
			Title:                  "Build is waiting for user input",
			Type:                   "user_reply_required",
			Category:               BlockerCategoryApprovals,
			Severity:               BlockerSeverityBlocking,
			WhoMustAct:             "user",
			Summary:                strings.TrimSpace(interaction.PendingQuestion),
			UnblocksWith:           "Reply in the build control surface so the next step can continue.",
			PartialProgressAllowed: false,
		})
	}

	for _, request := range interaction.PermissionRequests {
		if request.Status != PermissionRequestPending {
			continue
		}
		appendBlocker(BuildBlocker{
			ID:                     "permission-" + request.ID,
			Title:                  fmt.Sprintf("Permission needed: %s %s", request.Scope, request.Target),
			Type:                   "permission_request",
			Category:               blockerCategoryForPermissionScope(request.Scope),
			Severity:               firstPermissionSeverity(request.Blocking),
			WhoMustAct:             "user",
			Summary:                strings.TrimSpace(request.Reason),
			UnblocksWith:           "Allow or deny the permission request.",
			PartialProgressAllowed: !request.Blocking,
		})
	}

	if orchestration := build.SnapshotState.Orchestration; orchestration != nil {
		if orchestration.PromotionDecision != nil {
			for index, unresolved := range orchestration.PromotionDecision.UnresolvedBlockers {
				unresolved = strings.TrimSpace(unresolved)
				if unresolved == "" {
					continue
				}
				appendBlocker(BuildBlocker{
					ID:                     fmt.Sprintf("promotion-%d", index),
					Title:                  "Promotion is blocked",
					Type:                   "promotion_blocker",
					Category:               BlockerCategoryRuntimeFailure,
					Severity:               BlockerSeverityBlocking,
					WhoMustAct:             "system",
					Summary:                unresolved,
					UnblocksWith:           "Resolve the verification blocker and rerun readiness checks.",
					PartialProgressAllowed: false,
				})
			}
		}
		for _, report := range orchestration.VerificationReports {
			for index, blocker := range report.Blockers {
				blocker = strings.TrimSpace(blocker)
				if blocker == "" {
					continue
				}
				appendBlocker(BuildBlocker{
					ID:                     fmt.Sprintf("verification-%s-%d", report.ID, index),
					Title:                  "Verification blocker",
					Type:                   "verification_blocker",
					Category:               BlockerCategoryRuntimeFailure,
					Severity:               BlockerSeverityBlocking,
					WhoMustAct:             "system",
					Summary:                blocker,
					UnblocksWith:           "Repair the failing surface and rerun verification.",
					PartialProgressAllowed: false,
				})
			}
		}
	}

	_ = now
	return blockers
}

func firstPermissionSeverity(blocking bool) BuildBlockerSeverity {
	if blocking {
		return BlockerSeverityBlocking
	}
	return BlockerSeverityWarning
}

func permissionScopeTitle(scope BuildPermissionScope) string {
	switch scope {
	case PermissionScopeFilesystem:
		return "Filesystem"
	case PermissionScopeNetwork:
		return "Network"
	case PermissionScopeService:
		return "Service"
	default:
		return "Program"
	}
}

func approvalStatusFromPermissionRequestStatus(status BuildPermissionRequestStatus) BuildApprovalStatus {
	switch status {
	case PermissionRequestAllowed:
		return ApprovalStatusSatisfied
	case PermissionRequestDenied:
		return ApprovalStatusDenied
	default:
		return ApprovalStatusPending
	}
}

func deriveBuildApprovals(build *Build, capabilityState *BuildCapabilityState, policy *BuildPolicyState) []BuildApproval {
	if build == nil || capabilityState == nil {
		return nil
	}

	now := build.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	approvals := make([]BuildApproval, 0)
	appendApproval := func(kind, title string, required bool, blocked bool, reason string) {
		status := ApprovalStatusNotRequired
		requiredPlan := "builder"
		if policy != nil && strings.TrimSpace(policy.RequiredPlan) != "" {
			requiredPlan = strings.TrimSpace(policy.RequiredPlan)
		}
		var resolvedAt *time.Time
		if required {
			if blocked {
				status = ApprovalStatusPending
			} else {
				status = ApprovalStatusSatisfied
				resolved := now
				resolvedAt = &resolved
			}
		}
		approvals = append(approvals, BuildApproval{
			ID:                     kind,
			Kind:                   kind,
			Title:                  title,
			Status:                 status,
			Required:               required,
			Summary:                reason,
			Reason:                 firstNonEmptyString(reason, fmt.Sprintf("Requires %s approval.", title)),
			SourceType:             "policy",
			SourceID:               kind,
			Actor:                  "system",
			PartialProgressAllowed: !blocked,
			PlanTierRelated:        blocked,
			RequestedAt:            now,
			ResolvedAt:             resolvedAt,
		})
		if blocked && required {
			approvals[len(approvals)-1].Summary = fmt.Sprintf("%s blocked until %s plan access is available.", title, requiredPlan)
		}
	}

	upgradeBlocked := policy != nil && policy.UpgradeRequired
	appendApproval("plan_upgrade_acknowledgement", "Plan upgrade acknowledgement", upgradeBlocked, upgradeBlocked, "This request exceeds the current plan and needs explicit user acknowledgement or an upgrade before paid-only work can continue.")
	if upgradeBlocked && len(approvals) > 0 {
		approvals[len(approvals)-1].Actor = "user"
		approvals[len(approvals)-1].AcknowledgementRequired = true
		approvals[len(approvals)-1].PlanTierRelated = true
	}
	appendApproval("full_stack_upgrade", "Full-stack upgrade", policy != nil && policy.Classification != BuildClassificationStaticReady, upgradeBlocked, policy.UpgradeReason)
	appendApproval("auth", "Authentication and session wiring", capabilityState.RequiresAuth, upgradeBlocked, "Authentication flows need runtime support, secret review, and truthful session semantics.")
	appendApproval("database", "Database access", capabilityState.RequiresDatabase, upgradeBlocked, "Persistent data needs a paid full-stack plan.")
	appendApproval("external_api", "External API access", capabilityState.RequiresExternalAPI, false, "External integrations need secrets and integration review.")
	appendApproval("file_storage", "File storage", capabilityState.RequiresStorage, upgradeBlocked, "Uploads and storage need backend runtime support.")
	appendApproval("realtime", "Realtime channels", capabilityState.RequiresRealtime, upgradeBlocked, "Realtime features need backend runtime, connection auth, and transport verification.")
	appendApproval("background_jobs", "Background jobs", capabilityState.RequiresJobs, upgradeBlocked, "Queues and background processing need server-side execution.")
	appendApproval("secrets_usage", "Secrets usage", capabilityState.RequiresExternalAPI || capabilityState.RequiresBilling || capabilityState.RequiresAuth, false, "Runtime secrets should be provided before live integrations are promoted.")
	appendApproval("billing", "Billing-related steps", capabilityState.RequiresBilling, upgradeBlocked, "Billing flows require backend runtime, callbacks, and production review.")
	appendApproval("public_deployment", "Public deployment", capabilityState.RequiresPublish, policy != nil && !policy.PublishEnabled, "Publishing is limited to paid plans.")
	appendApproval("paid_provider_usage", "Paid model/provider usage", build.ProviderMode != "byok", false, "Hosted providers consume managed credits under the active subscription.")
	appendApproval("byok", "BYOK usage", capabilityState.RequiresBYOK || build.ProviderMode == "byok", policy != nil && !policy.BYOKEnabled, "BYOK is only available on paid plans.")

	for _, request := range build.Interaction.PermissionRequests {
		approval := BuildApproval{
			ID:                      "permission_request_" + request.ID,
			Kind:                    fmt.Sprintf("permission_%s", request.Scope),
			Title:                   fmt.Sprintf("%s access for %s", permissionScopeTitle(request.Scope), request.Target),
			Status:                  approvalStatusFromPermissionRequestStatus(request.Status),
			Required:                true,
			Summary:                 request.Reason,
			Reason:                  request.Reason,
			SourceType:              "permission_request",
			SourceID:                request.ID,
			Actor:                   "user",
			PartialProgressAllowed:  !request.Blocking,
			AcknowledgementRequired: true,
			RequestedAt:             request.RequestedAt,
			ResolvedAt:              request.ResolvedAt,
		}
		if request.Status == PermissionRequestDenied {
			approval.MismatchDetected = request.Blocking
			if approval.MismatchDetected {
				approval.MismatchReason = "The requested local access was denied and the build remains blocked until the plan changes."
			}
		}
		if strings.TrimSpace(request.ResolutionNote) != "" {
			approval.Summary = request.ResolutionNote
		}
		approvals = append(approvals, approval)
	}

	return approvals
}

func refreshDerivedTruthLocked(state *BuildOrchestrationState, capabilityState *BuildCapabilityState, policy *BuildPolicyState, blockers []BuildBlocker, qualityGateStatus string) {
	if state == nil || state.BuildContract == nil {
		return
	}

	globalTags := append([]TruthTag(nil), state.BuildContract.TruthBySurface[string(SurfaceGlobal)]...)
	frontendTags := append([]TruthTag(nil), state.BuildContract.TruthBySurface[string(SurfaceFrontend)]...)
	backendTags := append([]TruthTag(nil), state.BuildContract.TruthBySurface[string(SurfaceBackend)]...)
	integrationTags := append([]TruthTag(nil), state.BuildContract.TruthBySurface[string(SurfaceIntegration)]...)

	globalTags = removeTruthTags(globalTags, TruthBlocked, TruthNeedsApproval, TruthUpgradeRequired, TruthPrototypeUIOnly, TruthTested)
	frontendTags = removeTruthTags(frontendTags, TruthPrototypeUIOnly)

	if policy != nil {
		switch policy.Classification {
		case BuildClassificationStaticReady:
			globalTags = append(globalTags, TruthPrototypeUIOnly)
			frontendTags = append(frontendTags, TruthPrototypeUIOnly)
		case BuildClassificationUpgradeRequired:
			globalTags = append(globalTags, TruthUpgradeRequired, TruthBlocked)
		}
	}

	if capabilityState != nil {
		if capabilityState.RequiresBackendRuntime {
			globalTags = append(globalTags, TruthNeedsBackendRuntime)
			backendTags = append(backendTags, TruthNeedsBackendRuntime)
		}
		if capabilityState.RequiresExternalAPI || capabilityState.RequiresBilling {
			globalTags = append(globalTags, TruthNeedsExternalAPI)
			integrationTags = append(integrationTags, TruthNeedsExternalAPI)
		}
	}

	if len(blockers) > 0 {
		globalTags = append(globalTags, TruthBlocked, TruthNeedsApproval)
	}
	if normalizeQualityGateStatus(qualityGateStatus) == "passed" {
		globalTags = append(globalTags, TruthTested)
	}

	state.BuildContract.TruthBySurface[string(SurfaceGlobal)] = normalizeTruthTags(globalTags)
	state.BuildContract.TruthBySurface[string(SurfaceFrontend)] = normalizeTruthTags(frontendTags)
	state.BuildContract.TruthBySurface[string(SurfaceBackend)] = normalizeTruthTags(backendTags)
	state.BuildContract.TruthBySurface[string(SurfaceIntegration)] = normalizeTruthTags(integrationTags)
	if state.PromotionDecision != nil {
		state.PromotionDecision.TruthBySurface = cloneTruthBySurface(state.BuildContract.TruthBySurface)
	}
}

func cloneTruthBySurface(input map[string][]TruthTag) map[string][]TruthTag {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string][]TruthTag, len(input))
	for key, tags := range input {
		out[key] = append([]TruthTag(nil), tags...)
	}
	return out
}

func refreshDerivedSnapshotStateLocked(build *Build, state *BuildSnapshotState) {
	if build == nil || state == nil {
		return
	}
	capabilityState := buildCapabilityState(build)
	policyState := buildPolicyState(build, capabilityState)
	blockers := deriveBuildBlockers(build, policyState)
	approvals := deriveBuildApprovals(build, capabilityState, policyState)

	state.CapabilityState = capabilityState
	state.PolicyState = policyState
	state.Blockers = blockers
	state.Approvals = approvals

	if orchestration := state.Orchestration; orchestration != nil {
		refreshDerivedTruthLocked(orchestration, capabilityState, policyState, blockers, state.QualityGateStatus)
	}
}

func enrichBuildMessageSnapshotStateLocked(build *Build, msg *WSMessage) {
	if build == nil || msg == nil {
		return
	}
	typeName := strings.TrimSpace(string(msg.Type))
	if !strings.HasPrefix(typeName, "build:") && !strings.HasPrefix(typeName, "budget:") && !strings.HasPrefix(typeName, "spend:") {
		return
	}

	data := buildActivityDataMap(msg.Data)
	for key, value := range buildSnapshotStateResponseFields(copyBuildSnapshotStateLocked(build), string(build.Status)) {
		data[key] = value
	}
	if _, ok := data["status"]; !ok {
		data["status"] = string(build.Status)
	}
	if _, ok := data["progress"]; !ok {
		data["progress"] = build.Progress
	}
	msg.Data = data
}
