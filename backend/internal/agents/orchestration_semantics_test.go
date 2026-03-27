package agents

import (
	"testing"
	"time"

	"apex-build/internal/ai"
)

func TestRefreshDerivedSnapshotStateLockedStaticReadyBuild(t *testing.T) {
	build := &Build{
		ID:               "static-build",
		UserID:           1,
		Status:           BuildPending,
		Mode:             ModeFull,
		PowerMode:        PowerFast,
		SubscriptionPlan: "free",
		ProviderMode:     "platform",
		Description:      "Build a static marketing landing page for a coffee shop with pricing and testimonials",
		Agents:           map[string]*Agent{},
		Tasks:            []*Task{},
		Checkpoints:      []*Checkpoint{},
		CreatedAt:        time.Now().Add(-time.Minute).UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	if orchestration := ensureBuildOrchestrationStateLocked(build); orchestration != nil {
		orchestration.IntentBrief = compileIntentBriefFromRequest(&BuildRequest{
			Description: build.Description,
			Mode:        build.Mode,
			PowerMode:   build.PowerMode,
		}, build.ProviderMode)
	}

	refreshDerivedSnapshotStateLocked(build, &build.SnapshotState)

	if build.SnapshotState.PolicyState == nil {
		t.Fatalf("expected policy state")
	}
	if build.SnapshotState.PolicyState.Classification != BuildClassificationStaticReady {
		t.Fatalf("expected static_ready classification, got %s", build.SnapshotState.PolicyState.Classification)
	}
	if build.SnapshotState.PolicyState.UpgradeRequired {
		t.Fatalf("expected no upgrade requirement")
	}
	if build.SnapshotState.PolicyState.RequiredPlan != "" {
		t.Fatalf("expected no required plan for static build, got %q", build.SnapshotState.PolicyState.RequiredPlan)
	}
	if build.SnapshotState.CapabilityState == nil {
		t.Fatalf("expected capability state")
	}
	if build.SnapshotState.CapabilityState.RequiresBackendRuntime {
		t.Fatalf("expected static build to avoid backend runtime inference")
	}
	if len(build.SnapshotState.Blockers) != 0 {
		t.Fatalf("expected no blockers, got %+v", build.SnapshotState.Blockers)
	}
}

func TestRefreshDerivedSnapshotStateLockedFrontendCanaryPromptAvoidsUpgradeRequired(t *testing.T) {
	build := &Build{
		ID:               "pulseboard-free-canary",
		UserID:           101,
		Status:           BuildPending,
		Mode:             ModeFull,
		PowerMode:        PowerFast,
		SubscriptionPlan: "free",
		ProviderMode:     "platform",
		Description:      "Build a polished frontend-only client dashboard called PulseBoard using React 18, Vite, and Tailwind CSS with a responsive dark modern UI that works well in the preview pane, a dashboard home with KPI cards, trend widgets, an activity feed, and a highlighted primary action, a clients page with searchable cards, filters, empty states, and detail panels, a projects page with kanban-style status columns and clear progress visuals, a settings page with profile, notifications, and theme sections, realistic seed content in the UI so the preview feels complete immediately, strong loading, empty, and error states, reusable components and a clean file structure, and no backend, no database, and no fake API requirements in this free-tier preview pass.",
		Agents:           map[string]*Agent{},
		Tasks:            []*Task{},
		Checkpoints:      []*Checkpoint{},
		CreatedAt:        time.Now().Add(-time.Minute).UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	if orchestration := ensureBuildOrchestrationStateLocked(build); orchestration != nil {
		orchestration.IntentBrief = compileIntentBriefFromRequest(&BuildRequest{
			Description: build.Description,
			Mode:        build.Mode,
			PowerMode:   build.PowerMode,
		}, build.ProviderMode)
	}

	refreshDerivedSnapshotStateLocked(build, &build.SnapshotState)

	if build.SnapshotState.PolicyState == nil {
		t.Fatalf("expected policy state")
	}
	if build.SnapshotState.PolicyState.Classification != BuildClassificationStaticReady {
		t.Fatalf("expected static_ready classification, got %+v", build.SnapshotState.PolicyState)
	}
	if build.SnapshotState.PolicyState.UpgradeRequired {
		t.Fatalf("did not expect upgrade_required for free frontend canary prompt, got %+v", build.SnapshotState.PolicyState)
	}
	if build.SnapshotState.CapabilityState == nil {
		t.Fatalf("expected capability state")
	}
	if build.SnapshotState.CapabilityState.RequiresStorage || build.SnapshotState.CapabilityState.RequiresBackendRuntime {
		t.Fatalf("did not expect storage/backend runtime flags, got %+v", build.SnapshotState.CapabilityState)
	}
	if len(build.SnapshotState.Blockers) != 0 {
		t.Fatalf("expected no blockers for frontend canary prompt, got %+v", build.SnapshotState.Blockers)
	}
	for _, approval := range build.SnapshotState.Approvals {
		if approval.Kind == "file_storage" || approval.Kind == "full_stack_upgrade" || approval.Kind == "plan_upgrade_acknowledgement" {
			if approval.Status == ApprovalStatusPending || approval.Required {
				t.Fatalf("did not expect active paid upgrade approvals for frontend canary prompt, got %+v", build.SnapshotState.Approvals)
			}
		}
	}
}

func TestRefreshDerivedSnapshotStateLockedFullStackPaidBuild(t *testing.T) {
	build := &Build{
		ID:               "fullstack-build",
		UserID:           2,
		Status:           BuildInProgress,
		Mode:             ModeFull,
		PowerMode:        PowerBalanced,
		SubscriptionPlan: "builder",
		ProviderMode:     "platform",
		Description:      "Build a full stack SaaS with auth, postgres database, stripe billing, websocket collaboration, and deploy it to Render",
		Agents:           map[string]*Agent{},
		Tasks:            []*Task{},
		Checkpoints:      []*Checkpoint{},
		CreatedAt:        time.Now().Add(-time.Minute).UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	if orchestration := ensureBuildOrchestrationStateLocked(build); orchestration != nil {
		orchestration.IntentBrief = compileIntentBriefFromRequest(&BuildRequest{
			Description: build.Description,
			Mode:        build.Mode,
			PowerMode:   build.PowerMode,
		}, build.ProviderMode)
	}

	refreshDerivedSnapshotStateLocked(build, &build.SnapshotState)

	if build.SnapshotState.PolicyState == nil {
		t.Fatalf("expected policy state")
	}
	if build.SnapshotState.PolicyState.Classification != BuildClassificationFullStackCandidate {
		t.Fatalf("expected full_stack_candidate, got %s", build.SnapshotState.PolicyState.Classification)
	}
	if build.SnapshotState.PolicyState.RequiredPlan != "" {
		t.Fatalf("expected no required plan once paid plan is active, got %q", build.SnapshotState.PolicyState.RequiredPlan)
	}
	if build.SnapshotState.CapabilityState == nil {
		t.Fatalf("expected capability state")
	}
	if !build.SnapshotState.CapabilityState.RequiresBackendRuntime || !build.SnapshotState.CapabilityState.RequiresDatabase || !build.SnapshotState.CapabilityState.RequiresAuth {
		t.Fatalf("expected backend, database, and auth capability flags, got %+v", build.SnapshotState.CapabilityState)
	}
	if !build.SnapshotState.CapabilityState.RequiresPublish {
		t.Fatalf("expected publish requirement")
	}
	if len(build.SnapshotState.Approvals) == 0 {
		t.Fatalf("expected approvals to be derived")
	}

	requiredKinds := map[string]bool{
		"full_stack_upgrade": false,
		"auth":               false,
		"database":           false,
		"billing":            false,
		"realtime":           false,
		"public_deployment":  false,
	}
	for _, approval := range build.SnapshotState.Approvals {
		if _, ok := requiredKinds[approval.Kind]; ok {
			requiredKinds[approval.Kind] = true
			if approval.Status != ApprovalStatusSatisfied {
				t.Fatalf("expected paid-plan approval %s to be satisfied, got %s", approval.Kind, approval.Status)
			}
		}
	}
	for kind, seen := range requiredKinds {
		if !seen {
			t.Fatalf("expected approval kind %s in %+v", kind, build.SnapshotState.Approvals)
		}
	}
}

func TestRefreshDerivedSnapshotStateLockedUpgradeRequiredBuildIncludesPlanAcknowledgement(t *testing.T) {
	build := &Build{
		ID:               "upgrade-build",
		UserID:           9,
		Status:           BuildPlanning,
		Mode:             ModeFull,
		PowerMode:        PowerBalanced,
		SubscriptionPlan: "free",
		ProviderMode:     "byok",
		Description:      "Build a full stack SaaS with auth, postgres database, stripe billing, websocket collaboration, BYOK support, and publish it publicly",
		Agents:           map[string]*Agent{},
		Tasks:            []*Task{},
		Checkpoints:      []*Checkpoint{},
		CreatedAt:        time.Now().Add(-time.Minute).UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	if orchestration := ensureBuildOrchestrationStateLocked(build); orchestration != nil {
		orchestration.IntentBrief = compileIntentBriefFromRequest(&BuildRequest{
			Description: build.Description,
			Mode:        build.Mode,
			PowerMode:   build.PowerMode,
		}, build.ProviderMode)
	}

	refreshDerivedSnapshotStateLocked(build, &build.SnapshotState)

	if build.SnapshotState.PolicyState == nil || build.SnapshotState.PolicyState.Classification != BuildClassificationUpgradeRequired {
		t.Fatalf("expected upgrade_required policy state, got %+v", build.SnapshotState.PolicyState)
	}

	approvalsByKind := map[string]BuildApproval{}
	for _, approval := range build.SnapshotState.Approvals {
		approvalsByKind[approval.Kind] = approval
	}

	planAck, ok := approvalsByKind["plan_upgrade_acknowledgement"]
	if !ok {
		t.Fatalf("expected plan_upgrade_acknowledgement approval")
	}
	if planAck.Status != ApprovalStatusPending {
		t.Fatalf("expected pending plan acknowledgement, got %s", planAck.Status)
	}
	if !planAck.AcknowledgementRequired || planAck.Actor != "user" || !planAck.PlanTierRelated {
		t.Fatalf("expected user acknowledgement metadata on %+v", planAck)
	}

	for _, kind := range []string{"full_stack_upgrade", "auth", "database", "billing", "realtime", "public_deployment", "byok"} {
		approval, ok := approvalsByKind[kind]
		if !ok {
			t.Fatalf("expected approval %s", kind)
		}
		if approval.Status != ApprovalStatusPending {
			t.Fatalf("expected pending status for %s, got %s", kind, approval.Status)
		}
	}

	if !build.SnapshotState.PolicyState.StaticFrontendOnly {
		t.Fatalf("expected free upgrade-required build to retain static frontend fallback")
	}

	blockersByID := map[string]BuildBlocker{}
	for _, blocker := range build.SnapshotState.Blockers {
		blockersByID[blocker.ID] = blocker
	}
	upgradeBlocker, ok := blockersByID["plan-upgrade-required"]
	if !ok {
		t.Fatalf("expected plan-upgrade-required blocker")
	}
	if upgradeBlocker.Severity != BlockerSeverityWarning {
		t.Fatalf("expected upgrade blocker to be downgraded to warning during static fallback, got %s", upgradeBlocker.Severity)
	}
	if !upgradeBlocker.PartialProgressAllowed {
		t.Fatalf("expected static fallback to allow partial progress, got %+v", upgradeBlocker)
	}
}

func TestRefreshDerivedSnapshotStateLockedFrontendPreviewOnlyClearsPaidRuntimeApprovals(t *testing.T) {
	build := &Build{
		ID:               "preview-only-build",
		UserID:           11,
		Status:           BuildReviewing,
		Mode:             ModeFull,
		PowerMode:        PowerFast,
		SubscriptionPlan: "free",
		ProviderMode:     "platform",
		Description:      "Build a full stack CRM with auth, postgres database, file uploads, background jobs, and global search.",
		Plan: &BuildPlan{
			ID:           "plan-preview-only",
			BuildID:      "preview-only-build",
			AppType:      "web",
			DeliveryMode: "frontend_preview_only",
			TechStack:    TechStack{Frontend: "React", Styling: "Tailwind"},
		},
		Agents:      map[string]*Agent{},
		Tasks:       []*Task{},
		Checkpoints: []*Checkpoint{},
		CreatedAt:   time.Now().Add(-time.Minute).UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if orchestration := ensureBuildOrchestrationStateLocked(build); orchestration != nil {
		orchestration.IntentBrief = compileIntentBriefFromRequest(&BuildRequest{
			Description: build.Description,
			Mode:        build.Mode,
			PowerMode:   build.PowerMode,
		}, build.ProviderMode)
		orchestration.BuildContract = &BuildContract{
			BuildID:      build.ID,
			AppType:      "web",
			DeliveryMode: "frontend_preview_only",
			TruthBySurface: map[string][]TruthTag{
				string(SurfaceGlobal):   {},
				string(SurfaceFrontend): {},
				string(SurfaceBackend):  {},
			},
		}
	}

	refreshDerivedSnapshotStateLocked(build, &build.SnapshotState)

	if build.SnapshotState.CapabilityState == nil {
		t.Fatalf("expected capability state")
	}
	if build.SnapshotState.CapabilityState.RequiresAuth ||
		build.SnapshotState.CapabilityState.RequiresDatabase ||
		build.SnapshotState.CapabilityState.RequiresStorage ||
		build.SnapshotState.CapabilityState.RequiresJobs ||
		build.SnapshotState.CapabilityState.RequiresBackendRuntime {
		t.Fatalf("expected paid runtime capabilities to be cleared in preview-only mode, got %+v", build.SnapshotState.CapabilityState)
	}

	approvalsByKind := map[string]BuildApproval{}
	for _, approval := range build.SnapshotState.Approvals {
		approvalsByKind[approval.Kind] = approval
	}
	for _, kind := range []string{"plan_upgrade_acknowledgement", "full_stack_upgrade", "auth", "database", "file_storage", "background_jobs"} {
		approval, ok := approvalsByKind[kind]
		if !ok {
			t.Fatalf("expected approval %s", kind)
		}
		if approval.Status != ApprovalStatusNotRequired {
			t.Fatalf("expected %s to be not_required in frontend preview mode, got %s", kind, approval.Status)
		}
	}

	for _, blocker := range build.SnapshotState.Blockers {
		if blocker.ID == "plan-upgrade-required" {
			t.Fatalf("did not expect plan upgrade blocker once delivery is explicitly frontend_preview_only: %+v", blocker)
		}
	}
}

func TestBroadcastBuildProgressIncludesDerivedSemanticState(t *testing.T) {
	manager := newTestIterationManager(&stubPreflight{
		configured:    true,
		allProviders:  []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4},
		userProviders: []ai.AIProvider{ai.ProviderClaude, ai.ProviderGPT4},
	})

	build := &Build{
		ID:               "semantic-build",
		UserID:           3,
		Status:           BuildInProgress,
		Mode:             ModeFull,
		PowerMode:        PowerBalanced,
		SubscriptionPlan: "builder",
		ProviderMode:     "platform",
		Description:      "Build a full stack dashboard with auth and postgres",
		Agents: map[string]*Agent{
			"agent-1": {
				ID:       "agent-1",
				Role:     RoleArchitect,
				Provider: ai.ProviderClaude,
			},
		},
		Tasks:       []*Task{},
		Checkpoints: []*Checkpoint{},
		CreatedAt:   time.Now().Add(-time.Minute).UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if orchestration := ensureBuildOrchestrationStateLocked(build); orchestration != nil {
		orchestration.IntentBrief = compileIntentBriefFromRequest(&BuildRequest{
			Description: build.Description,
			Mode:        build.Mode,
			PowerMode:   build.PowerMode,
		}, build.ProviderMode)
	}
	refreshDerivedSnapshotStateLocked(build, &build.SnapshotState)

	manager.builds[build.ID] = build
	ch := make(chan *WSMessage, 1)
	manager.subscribers[build.ID] = []chan *WSMessage{ch}

	manager.broadcast(build.ID, &WSMessage{
		Type:      "build:phase",
		BuildID:   build.ID,
		Timestamp: time.Now().UTC(),
		Data: map[string]any{
			"phase": "contract_compile",
		},
	})

	select {
	case msg := <-ch:
		data, ok := msg.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected data map, got %T", msg.Data)
		}
		if data["build_classification"] != string(BuildClassificationFullStackCandidate) && data["build_classification"] != BuildClassificationFullStackCandidate {
			t.Fatalf("expected build classification in broadcast, got %v", data["build_classification"])
		}
		if _, ok := data["policy_state"]; !ok {
			t.Fatalf("expected policy_state in broadcast payload")
		}
		if _, ok := data["capability_state"]; !ok {
			t.Fatalf("expected capability_state in broadcast payload")
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for broadcast")
	}
}

func TestRefreshDerivedSnapshotStateLockedIncludesPermissionApprovals(t *testing.T) {
	now := time.Now().UTC()
	resolved := now.Add(2 * time.Minute)

	build := &Build{
		ID:               "permission-approval-build",
		UserID:           4,
		Status:           BuildInProgress,
		Mode:             ModeFull,
		PowerMode:        PowerBalanced,
		SubscriptionPlan: "builder",
		ProviderMode:     "platform",
		Description:      "Build a hosted app and use local docker during verification",
		Agents:           map[string]*Agent{},
		Tasks:            []*Task{},
		Checkpoints:      []*Checkpoint{},
		CreatedAt:        now,
		UpdatedAt:        now,
		Interaction: BuildInteractionState{
			PermissionRequests: []BuildPermissionRequest{
				{
					ID:             "perm-1",
					Scope:          PermissionScopeProgram,
					Target:         "docker",
					Reason:         "Docker is needed to run the local preview image.",
					Blocking:       true,
					Status:         PermissionRequestAllowed,
					Mode:           PermissionModeBuild,
					ResolutionNote: "Approved for this build.",
					RequestedAt:    now,
					ResolvedAt:     &resolved,
				},
			},
		},
	}
	if orchestration := ensureBuildOrchestrationStateLocked(build); orchestration != nil {
		orchestration.IntentBrief = compileIntentBriefFromRequest(&BuildRequest{
			Description: build.Description,
			Mode:        build.Mode,
			PowerMode:   build.PowerMode,
		}, build.ProviderMode)
	}

	refreshDerivedSnapshotStateLocked(build, &build.SnapshotState)

	var permissionApproval *BuildApproval
	for idx := range build.SnapshotState.Approvals {
		approval := &build.SnapshotState.Approvals[idx]
		if approval.SourceType == "permission_request" {
			permissionApproval = approval
			break
		}
	}
	if permissionApproval == nil {
		t.Fatalf("expected a permission-request approval in %+v", build.SnapshotState.Approvals)
	}
	if permissionApproval.Status != ApprovalStatusSatisfied {
		t.Fatalf("expected resolved permission approval, got %s", permissionApproval.Status)
	}
	if permissionApproval.Actor != "user" {
		t.Fatalf("expected permission approval actor=user, got %s", permissionApproval.Actor)
	}
	if !permissionApproval.AcknowledgementRequired {
		t.Fatalf("expected permission approval to require acknowledgement metadata")
	}
	if permissionApproval.ResolvedAt == nil {
		t.Fatalf("expected resolved_at on permission approval")
	}
	if permissionApproval.Summary != "Approved for this build." {
		t.Fatalf("expected resolution note to flow into summary, got %q", permissionApproval.Summary)
	}
}
