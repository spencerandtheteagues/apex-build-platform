// Package agents - HTTP API Handlers
// RESTful endpoints for build management
package agents

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"apex-build/internal/applog"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	errBuildActionNotFound = errors.New("build not found")
	errBuildAccessDenied   = errors.New("access denied")
)

// BuildHandler handles build-related HTTP requests
type BuildHandler struct {
	manager *AgentManager
	hub     *WSHub
	db      *gorm.DB
}

func classifyBuildMessageError(err error) int {
	if err == nil {
		return http.StatusOK
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "message cannot be empty"),
		strings.Contains(message, "no agents matched the selected target"),
		strings.Contains(message, "unknown command"):
		return http.StatusBadRequest
	case strings.Contains(message, "direct agent messages require an active build"),
		strings.Contains(message, "restart is only available for failed builds"):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func userIDFromValue(c *gin.Context, value any) (uint, bool) {
	uid, ok := value.(uint)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user context"})
		return 0, false
	}
	return uid, true
}

// NewBuildHandler creates a new build handler
func NewBuildHandler(manager *AgentManager, hub *WSHub) *BuildHandler {
	return &BuildHandler{
		manager: manager,
		hub:     hub,
		db:      manager.db,
	}
}

// PreflightCheck validates provider credentials and billing status before a build.
// POST /api/v1/build/preflight
func (h *BuildHandler) PreflightCheck(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userID.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}

	type preflightResult struct {
		Ready            bool              `json:"ready"`
		Providers        int               `json:"providers_available"`
		Names            []string          `json:"provider_names"`
		ProviderStatuses map[string]string `json:"provider_statuses,omitempty"`
		HasBYOK          bool              `json:"has_byok"`
		ErrorCode        string            `json:"error_code,omitempty"`
		Error            string            `json:"error,omitempty"`
		Suggestion       string            `json:"suggestion,omitempty"`
	}

	router := h.manager.aiRouter
	if router == nil {
		c.JSON(http.StatusServiceUnavailable, preflightResult{
			ErrorCode:  "NO_ROUTER",
			Error:      "AI routing service unavailable",
			Suggestion: "Server configuration error — contact support",
		})
		return
	}

	if !router.HasConfiguredProviders() {
		c.JSON(http.StatusServiceUnavailable, preflightResult{
			ErrorCode:  "NO_PROVIDER",
			Error:      "No AI providers configured",
			Suggestion: "Add an API key for at least one AI provider in Settings",
		})
		return
	}

	providers := router.GetAvailableProvidersForUser(uid)
	if len(providers) == 0 {
		allProviders := router.GetAvailableProviders()
		if len(allProviders) == 0 {
			c.JSON(http.StatusServiceUnavailable, preflightResult{
				ErrorCode:  "ALL_PROVIDERS_DOWN",
				Error:      "All AI providers are currently unavailable",
				Suggestion: "Check your API keys in Settings or try again shortly",
			})
			return
		}
		c.JSON(http.StatusPaymentRequired, preflightResult{
			ErrorCode:  "INSUFFICIENT_CREDITS",
			Error:      "No AI providers available for your account",
			Suggestion: "Add credits or configure a personal API key in Settings",
		})
		return
	}

	names := make([]string, len(providers))
	availSet := make(map[string]bool)
	for i, p := range providers {
		names[i] = string(p)
		availSet[string(p)] = true
	}

	// Build provider status map: always include 4 platform providers,
	// plus any BYOK-only providers (like ollama) that are actually available.
	statuses := make(map[string]string)
	for _, k := range []string{"claude", "gpt4", "gemini", "grok"} {
		if availSet[k] {
			statuses[k] = "available"
		} else {
			statuses[k] = "unavailable"
		}
	}
	// Include Ollama in the status map when the user has it configured (even if offline).
	// This ensures the frontend shows the Ollama card so users can assign roles to it.
	if availSet["ollama"] {
		statuses["ollama"] = "available"
	} else if h.db != nil {
		var ollamaKeyCount int64
		h.db.Model(&models.UserAPIKey{}).
			Where("user_id = ? AND LOWER(provider) = 'ollama' AND is_active = ? AND deleted_at IS NULL", uid, true).
			Count(&ollamaKeyCount)
		if ollamaKeyCount > 0 {
			statuses["ollama"] = "unavailable"
		}
	}

	hasBYOK := h.manager.userHasActiveBYOKKey(uid)

	c.JSON(http.StatusOK, preflightResult{
		Ready:            true,
		Providers:        len(providers),
		Names:            names,
		ProviderStatuses: statuses,
		HasBYOK:          hasBYOK,
	})
}

// StartBuild creates and starts a new build
// POST /api/v1/build/start
func (h *BuildHandler) StartBuild(c *gin.Context) {
	log.Printf("StartBuild handler called")

	// Get user ID from auth context
	userID, exists := c.Get("user_id")
	if !exists {
		log.Printf("StartBuild: unauthorized - no user_id in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userID.(uint)
	if !ok {
		log.Printf("StartBuild: invalid user_id type %T", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	log.Printf("StartBuild: user_id=%d", uid)

	// Parse request
	var req BuildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("StartBuild: invalid request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"details": err.Error(),
		})
		return
	}
	// Allow prompt as fallback for description and vice-versa
	if req.Description == "" && req.Prompt != "" {
		req.Description = req.Prompt
	}
	if req.Prompt == "" && req.Description != "" {
		req.Prompt = req.Description
	}

	log.Printf("StartBuild: description=%s, mode=%s", truncate(req.Description, 50), req.Mode)

	// Validate description before running provider or billing checks so clients
	// get actionable input errors instead of quota failures for malformed requests.
	req.Description = strings.TrimSpace(req.Description)
	if req.Prompt != "" {
		req.Prompt = strings.TrimSpace(req.Prompt)
	}
	if len(req.Description) < 10 {
		log.Printf("StartBuild: description too short (%d chars)", len(req.Description))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "description too short",
			"details": "Please provide a more detailed description of the app you want to build",
		})
		return
	}

	planType := h.currentSubscriptionType(c, uid)
	if requiresUpgrade, reason := buildSubscriptionRequirement(&req); requiresUpgrade && !isPaidBuildPlan(planType) {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error":          "Backend and full-stack builds require a paid subscription",
			"error_code":     backendSubscriptionRequiredCode,
			"current_plan":   planType,
			"required_plan":  "builder",
			"blocked_reason": reason,
			"suggestion":     "Free accounts can build static frontend websites. Upgrade to Builder or higher to unlock backend, database, auth, billing, and realtime app generation.",
		})
		return
	}

	// Validate role_assignments if provided
	if req.RoleAssignments != nil {
		validCats := map[string]bool{"architect": true, "coder": true, "tester": true, "devops": true}
		validProvs := map[string]bool{"claude": true, "gpt4": true, "gemini": true, "grok": true, "ollama": true}
		for cat, prov := range req.RoleAssignments {
			if !validCats[cat] {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "invalid role category",
					"details": fmt.Sprintf("Unknown role category: %s. Valid: architect, coder, tester, devops", cat),
				})
				return
			}
			if !validProvs[prov] {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "invalid provider",
					"details": fmt.Sprintf("Unknown provider: %s. Valid: claude, gpt4, gemini, grok, ollama", prov),
				})
				return
			}
			if prov == "ollama" && strings.ToLower(strings.TrimSpace(req.ProviderMode)) != "byok" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "invalid provider for hosted build",
					"details": "Ollama is local/BYOK-only. Hosted platform builds may only assign Claude, GPT, Gemini, or Grok.",
				})
				return
			}
		}
	}

	// Preflight: fail fast if no providers are available for this user
	if router := h.manager.aiRouter; router != nil {
		if !router.HasConfiguredProviders() {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":      "No AI providers configured",
				"error_code": "NO_PROVIDER",
				"suggestion": "Add an API key for at least one AI provider in Settings",
			})
			return
		}
		if providers := router.GetAvailableProvidersForUser(uid); len(providers) == 0 {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error":      "No AI providers available for your account",
				"error_code": "INSUFFICIENT_CREDITS",
				"suggestion": "Add credits or configure a personal API key in Settings",
			})
			return
		} else if req.RoleAssignments != nil {
			available := make(map[string]bool, len(providers))
			for _, provider := range providers {
				available[strings.TrimSpace(strings.ToLower(string(provider)))] = true
			}
			for category, provider := range req.RoleAssignments {
				if !available[strings.TrimSpace(strings.ToLower(provider))] {
					c.JSON(http.StatusConflict, gin.H{
						"error":   "requested provider unavailable",
						"details": fmt.Sprintf("Role %s requested provider %s, but it is not currently available for this account", category, provider),
					})
					return
				}
			}
		}
	}

	// Hard stop: check credit balance before creating the build.
	// BYOK users (own API keys) and admin bypass flags are exempt.
	if h.db != nil {
		bypassBilling, _ := c.Get("bypass_billing")
		hasUnlimited, _ := c.Get("has_unlimited_credits")
		bypass := false
		if b, ok := bypassBilling.(bool); ok && b {
			bypass = true
		}
		if b, ok := hasUnlimited.(bool); ok && b {
			bypass = true
		}
		if !bypass && !h.manager.userHasActiveBYOKKey(uid) {
			var creditBalance float64
			h.db.Raw("SELECT credit_balance FROM users WHERE id = ?", uid).Scan(&creditBalance)
			if creditBalance <= 0 {
				c.JSON(http.StatusPaymentRequired, gin.H{
					"error":          "Insufficient credits",
					"error_code":     "INSUFFICIENT_CREDITS",
					"suggestion":     "Purchase credits to continue building",
					"credit_balance": creditBalance,
				})
				return
			}
		}
	}

	// Create the build
	build, err := h.manager.CreateBuild(uid, planType, &req)
	if err != nil {
		log.Printf("StartBuild: failed to create build: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to create build",
			"details": err.Error(),
		})
		return
	}
	log.Printf("StartBuild: build created with ID %s", build.ID)
	applog.BuildStarted(build.ID, uid, string(req.Mode), string(req.PowerMode), req.Description)

	// Start the build process asynchronously
	go func() {
		log.Printf("StartBuild: starting async build process for %s", build.ID)
		if err := h.manager.StartBuild(build.ID); err != nil {
			log.Printf("Error starting build %s: %v", build.ID, err)
			applog.BuildFailed(build.ID, uid, err.Error(), 0)
		}
	}()

	// Return build info immediately with WebSocket URL
	response := BuildResponse{
		BuildID:      build.ID,
		WebSocketURL: "/ws/build/" + build.ID,
		Status:       string(build.Status),
	}
	log.Printf("StartBuild: returning response for build %s", build.ID)
	c.JSON(http.StatusCreated, response)
}

// GetBuildStatus returns the current status of a build
// GET /api/v1/build/:id/status
func (h *BuildHandler) GetBuildStatus(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	build, err := h.manager.GetBuild(buildID)
	if err == nil {
		// Verify ownership
		if uid != build.UserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		build.mu.RLock()
		defer build.mu.RUnlock()

		errorMessage := build.Error
		if strings.TrimSpace(errorMessage) == "" && build.Status == BuildFailed {
			errorMessage = latestFailedTaskErrorLocked(build)
		}
		interaction := copyBuildInteractionStateLocked(build)

		response := gin.H{
			"id":                    build.ID,
			"status":                string(build.Status),
			"mode":                  string(build.Mode),
			"power_mode":            string(build.PowerMode),
			"provider_mode":         build.ProviderMode,
			"require_preview_ready": build.RequirePreviewReady,
			"description":           build.Description,
			"progress":              build.Progress,
			"agents_count":          len(build.Agents),
			"tasks_count":           len(build.Tasks),
			"checkpoints":           len(build.Checkpoints),
			"created_at":            build.CreatedAt,
			"updated_at":            build.UpdatedAt,
			"completed_at":          build.CompletedAt,
			"error":                 errorMessage,
			"interaction":           interaction,
			"live":                  true,
		}
		for key, value := range buildSnapshotStateResponseFields(copyBuildSnapshotStateLocked(build), string(build.Status)) {
			response[key] = value
		}
		c.JSON(http.StatusOK, response)
		return
	}

	snapshot, snapErr := h.getBuildSnapshot(uid, buildID)
	if snapErr != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "build not found",
			"details": err.Error(),
		})
		return
	}

	// Normalize snapshot status: if completed_at is set and no error, it's completed
	snapshotStatus := snapshot.Status
	if !snapshot.CompletedAt.IsZero() && snapshot.Error == "" &&
		snapshotStatus != "failed" && snapshotStatus != "cancelled" {
		snapshotStatus = "completed"
	}
	snapshotProgress := snapshot.Progress
	if snapshotStatus == "completed" {
		snapshotProgress = 100
	}
	agents := parseBuildAgents(snapshot.AgentsJSON)
	tasks := parseBuildTasks(snapshot.TasksJSON)
	checkpoints := parseBuildCheckpoints(snapshot.CheckpointsJSON)

	response := gin.H{
		"id":                     snapshot.BuildID,
		"status":                 snapshotStatus,
		"mode":                   snapshot.Mode,
		"power_mode":             snapshot.PowerMode,
		"description":            snapshot.Description,
		"progress":               snapshotProgress,
		"agents_count":           len(agents),
		"tasks_count":            len(tasks),
		"checkpoints":            len(checkpoints),
		"created_at":             snapshot.CreatedAt,
		"updated_at":             snapshot.UpdatedAt,
		"completed_at":           snapshot.CompletedAt,
		"error":                  snapshot.Error,
		"files_count":            snapshot.FilesCount,
		"interaction":            parseBuildInteraction(snapshot.InteractionJSON),
		"live":                   false,
		"restored_from_snapshot": true,
	}
	for key, value := range buildSnapshotStateResponseFields(parseBuildSnapshotState(snapshot.StateJSON), snapshotStatus) {
		response[key] = value
	}
	c.JSON(http.StatusOK, response)
}

// latestFailedTaskErrorLocked extracts the latest actionable task failure from a live build.
// Callers must hold build.mu while invoking this helper.
func latestFailedTaskErrorLocked(build *Build) string {
	if build == nil {
		return ""
	}
	for i := len(build.Tasks) - 1; i >= 0; i-- {
		task := build.Tasks[i]
		if task == nil || task.Status != TaskFailed {
			continue
		}
		if msg := strings.TrimSpace(task.Error); msg != "" {
			return msg
		}
		for j := len(task.ErrorHistory) - 1; j >= 0; j-- {
			if msg := strings.TrimSpace(task.ErrorHistory[j].Error); msg != "" {
				return msg
			}
		}
	}
	return ""
}

func (h *BuildHandler) getBuildActionSession(userID uint, buildID string) (*Build, bool, error) {
	build, err := h.manager.GetBuild(buildID)
	if err == nil {
		if build.UserID != userID {
			return nil, false, errBuildAccessDenied
		}
		if !isActiveBuildStatus(string(build.Status)) {
			return nil, false, errBuildNotActive
		}
		return build, false, nil
	}

	snapshot, snapErr := h.getBuildSnapshot(userID, buildID)
	if snapErr != nil {
		if errors.Is(snapErr, gorm.ErrRecordNotFound) {
			return nil, false, errBuildActionNotFound
		}
		return nil, false, snapErr
	}
	if !isActiveBuildStatus(string(normalizeRestoredBuildStatus(snapshot))) {
		return nil, false, errBuildNotActive
	}

	build, restored, restoreErr := h.manager.restoreBuildSessionFromSnapshot(snapshot)
	if restoreErr != nil {
		return nil, false, restoreErr
	}
	if build.UserID != userID {
		return nil, false, errBuildAccessDenied
	}
	return build, restored, nil
}

func writeBuildActionSessionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errBuildAccessDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
	case errors.Is(err, errBuildActionNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
	case errors.Is(err, errBuildNotActive):
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "build not active",
			"details": "Only active builds can be controlled or reviewed",
		})
	default:
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "build session unavailable",
			"details": err.Error(),
		})
	}
}

func buildSnapshotStateResponseFields(state BuildSnapshotState, fallbackStatus string) gin.H {
	fields := gin.H{}

	currentPhase := strings.TrimSpace(state.CurrentPhase)
	if currentPhase == "" {
		currentPhase = strings.TrimSpace(strings.ToLower(fallbackStatus))
	}
	if currentPhase != "" {
		fields["phase"] = currentPhase
		fields["current_phase"] = currentPhase
	}

	if state.QualityGateRequired != nil {
		fields["quality_gate_required"] = *state.QualityGateRequired
	}
	if stage := strings.TrimSpace(state.QualityGateStage); stage != "" {
		fields["quality_gate_stage"] = stage
	}
	if len(state.AvailableProviders) > 0 {
		fields["available_providers"] = append([]string(nil), state.AvailableProviders...)
	}
	if state.CapabilityState != nil {
		fields["capability_state"] = state.CapabilityState
	}
	if state.PolicyState != nil {
		fields["policy_state"] = state.PolicyState
		fields["build_classification"] = state.PolicyState.Classification
		fields["upgrade_required"] = state.PolicyState.UpgradeRequired
		if reason := strings.TrimSpace(state.PolicyState.UpgradeReason); reason != "" {
			fields["upgrade_reason"] = reason
		}
	}
	if len(state.Blockers) > 0 {
		fields["blockers"] = append([]BuildBlocker(nil), state.Blockers...)
	}
	if len(state.Approvals) > 0 {
		fields["approvals"] = append([]BuildApproval(nil), state.Approvals...)
	}
	if orchestration := cloneBuildOrchestrationState(state.Orchestration); orchestration != nil {
		fields["orchestration"] = orchestration
		if orchestration.IntentBrief != nil {
			fields["intent_brief"] = orchestration.IntentBrief
		}
		if orchestration.BuildContract != nil {
			fields["build_contract"] = orchestration.BuildContract
		}
		if len(orchestration.WorkOrders) > 0 {
			fields["work_orders"] = orchestration.WorkOrders
		}
		if len(orchestration.PatchBundles) > 0 {
			fields["patch_bundles"] = orchestration.PatchBundles
		}
		if len(orchestration.VerificationReports) > 0 {
			fields["verification_reports"] = orchestration.VerificationReports
		}
		if orchestration.PromotionDecision != nil {
			fields["promotion_decision"] = orchestration.PromotionDecision
			fields["truth_by_surface"] = orchestration.PromotionDecision.TruthBySurface
		}
		if len(orchestration.ProviderScorecards) > 0 {
			fields["provider_scorecards"] = orchestration.ProviderScorecards
		}
		if len(orchestration.FailureFingerprints) > 0 {
			fields["failure_fingerprints"] = orchestration.FailureFingerprints
		}
	}

	switch normalizeQualityGateStatus(state.QualityGateStatus) {
	case "running":
		fields["quality_gate_status"] = "running"
		fields["quality_gate_active"] = true
	case "passed":
		fields["quality_gate_status"] = "passed"
		fields["quality_gate_passed"] = true
	case "failed":
		fields["quality_gate_status"] = "failed"
		fields["quality_gate_passed"] = false
	case "pending":
		fields["quality_gate_status"] = "pending"
	}

	return fields
}

// GetBuildDetails returns full details of a build including agents and tasks
// GET /api/v1/build/:id
func (h *BuildHandler) GetBuildDetails(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	build, err := h.manager.GetBuild(buildID)
	if err == nil {
		// Verify ownership
		if uid != build.UserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		build.mu.RLock()
		defer build.mu.RUnlock()

		agents := orderedBuildAgents(build.Agents)
		interaction := copyBuildInteractionStateLocked(build)

		response := gin.H{
			"id":                    build.ID,
			"user_id":               build.UserID,
			"project_id":            build.ProjectID,
			"status":                string(build.Status),
			"mode":                  string(build.Mode),
			"power_mode":            string(build.PowerMode),
			"provider_mode":         build.ProviderMode,
			"require_preview_ready": build.RequirePreviewReady,
			"description":           build.Description,
			"plan":                  build.Plan,
			"agents":                agents,
			"tasks":                 build.Tasks,
			"checkpoints":           build.Checkpoints,
			"progress":              build.Progress,
			"created_at":            build.CreatedAt,
			"updated_at":            build.UpdatedAt,
			"completed_at":          build.CompletedAt,
			"error":                 build.Error,
			"files":                 h.manager.collectGeneratedFiles(build),
			"messages":              interaction.Messages,
			"interaction":           interaction,
			"activity_timeline":     copyBuildActivityTimelineLocked(build),
			"live":                  true,
		}
		for key, value := range buildSnapshotStateResponseFields(copyBuildSnapshotStateLocked(build), string(build.Status)) {
			response[key] = value
		}
		c.JSON(http.StatusOK, response)
		return
	}

	snapshot, snapErr := h.getBuildSnapshot(uid, buildID)
	if snapErr != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "build not found",
			"details": err.Error(),
		})
		return
	}

	files, _ := parseBuildFiles(snapshot.FilesJSON)
	agents := orderedBuildAgents(parseBuildAgents(snapshot.AgentsJSON))
	tasks := parseBuildTasks(snapshot.TasksJSON)
	checkpoints := parseBuildCheckpoints(snapshot.CheckpointsJSON)
	interaction := parseBuildInteraction(snapshot.InteractionJSON)
	activityTimeline := parseBuildActivityTimeline(snapshot.ActivityJSON)
	snapshotState := parseBuildSnapshotState(snapshot.StateJSON)

	response := gin.H{
		"id":                     snapshot.BuildID,
		"user_id":                snapshot.UserID,
		"project_id":             snapshot.ProjectID,
		"status":                 snapshot.Status,
		"mode":                   snapshot.Mode,
		"power_mode":             snapshot.PowerMode,
		"description":            snapshot.Description,
		"plan":                   nil,
		"agents":                 agents,
		"tasks":                  tasks,
		"checkpoints":            checkpoints,
		"progress":               snapshot.Progress,
		"created_at":             snapshot.CreatedAt,
		"updated_at":             snapshot.UpdatedAt,
		"completed_at":           snapshot.CompletedAt,
		"error":                  snapshot.Error,
		"files":                  files,
		"messages":               interaction.Messages,
		"interaction":            interaction,
		"activity_timeline":      activityTimeline,
		"live":                   false,
		"restored_from_snapshot": true,
	}
	for key, value := range buildSnapshotStateResponseFields(snapshotState, snapshot.Status) {
		response[key] = value
	}
	c.JSON(http.StatusOK, response)
}

// SendMessage sends a message to the build's lead agent
// POST /api/v1/build/:id/message
func (h *BuildHandler) SendMessage(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	// Verify build exists and ownership
	restoredSession := false
	build, err := h.manager.GetBuild(buildID)
	if err != nil {
		snapshot, snapErr := h.getBuildSnapshot(uid, buildID)
		if snapErr != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "build not found",
				"details": err.Error(),
			})
			return
		}

		build, restoredSession, err = h.manager.restoreBuildSessionFromSnapshot(snapshot)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "build session unavailable",
				"details": err.Error(),
			})
			return
		}
	}

	if uid != build.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Parse message
	var req struct {
		Content         string `json:"content"`
		ClientToken     string `json:"client_token"`
		Command         string `json:"command"`
		TargetMode      string `json:"target_mode"`
		TargetAgentID   string `json:"target_agent_id"`
		TargetAgentRole string `json:"target_agent_role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"details": err.Error(),
		})
		return
	}

	command := strings.TrimSpace(strings.ToLower(req.Command))
	if command == "" && strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"details": "content is required unless a command is provided",
		})
		return
	}

	target := normalizeBuildMessageTarget(req.TargetMode, req.TargetAgentID, req.TargetAgentRole)
	responseMessage := "Message sent to lead agent"
	var sendErr error
	switch command {
	case "":
		sendErr = h.manager.sendTargetedMessageWithClientToken(buildID, req.Content, req.ClientToken, target)
		switch target.Mode {
		case BuildMessageTargetAgent:
			responseMessage = "Message sent to selected agent"
		case BuildMessageTargetRole:
			responseMessage = "Message sent to selected role"
		case BuildMessageTargetAllAgents:
			responseMessage = "Message broadcast to all agents"
		}
	case "restart_failed":
		sendErr = h.manager.RestartFailedBuildWithClientToken(buildID, req.Content, req.ClientToken)
		responseMessage = "Failed build restart requested"
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"details": fmt.Sprintf("unknown command %q", req.Command),
		})
		return
	}
	if sendErr != nil {
		c.JSON(classifyBuildMessageError(sendErr), gin.H{
			"error":   "failed to send message",
			"details": sendErr.Error(),
		})
		return
	}

	interaction, interactionErr := h.manager.GetBuildInteraction(buildID)
	if interactionErr != nil {
		interaction = BuildInteractionState{}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":           "sent",
		"message":          responseMessage,
		"interaction":      interaction,
		"live":             true,
		"restored_session": restoredSession,
	})
}

// GetMessages returns the persisted build conversation.
// GET /api/v1/build/:id/messages
func (h *BuildHandler) GetMessages(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	if build, err := h.manager.GetBuild(buildID); err == nil {
		if uid != build.UserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		build.mu.RLock()
		interaction := copyBuildInteractionStateLocked(build)
		build.mu.RUnlock()
		c.JSON(http.StatusOK, gin.H{
			"messages":    interaction.Messages,
			"interaction": interaction,
			"live":        true,
		})
		return
	}

	snapshot, err := h.getBuildSnapshot(uid, buildID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}

	interaction := parseBuildInteraction(snapshot.InteractionJSON)
	c.JSON(http.StatusOK, gin.H{
		"messages":    interaction.Messages,
		"interaction": interaction,
		"live":        false,
	})
}

// GetPermissions returns the local resource permission state for a build.
// GET /api/v1/build/:id/permissions
func (h *BuildHandler) GetPermissions(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	if build, err := h.manager.GetBuild(buildID); err == nil {
		if uid != build.UserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		interaction, interactionErr := h.manager.GetBuildInteraction(buildID)
		if interactionErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load permissions"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"interaction": interaction,
			"rules":       interaction.PermissionRules,
			"requests":    interaction.PermissionRequests,
			"live":        true,
		})
		return
	}

	snapshot, err := h.getBuildSnapshot(uid, buildID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}

	interaction := parseBuildInteraction(snapshot.InteractionJSON)
	c.JSON(http.StatusOK, gin.H{
		"interaction": interaction,
		"rules":       interaction.PermissionRules,
		"requests":    interaction.PermissionRequests,
		"live":        false,
	})
}

// SetPermissionRule stores a pre-approved or denied local permission for the build.
// POST /api/v1/build/:id/permissions/rules
func (h *BuildHandler) SetPermissionRule(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	build, err := h.manager.GetBuild(buildID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}
	if uid != build.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	var req struct {
		Scope    string `json:"scope" binding:"required"`
		Target   string `json:"target" binding:"required"`
		Decision string `json:"decision"`
		Mode     string `json:"mode"`
		Reason   string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	interaction, rule, err := h.manager.SetPermissionRule(
		buildID,
		normalizePermissionScope(req.Scope),
		req.Target,
		normalizePermissionDecision(req.Decision),
		normalizePermissionMode(req.Mode),
		req.Reason,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rule":        rule,
		"interaction": interaction,
	})
}

// ResolvePermissionRequest approves or denies a pending local resource request.
// POST /api/v1/build/:id/permissions/requests/:requestId/resolve
func (h *BuildHandler) ResolvePermissionRequest(c *gin.Context) {
	buildID := c.Param("id")
	requestID := c.Param("requestId")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	build, err := h.manager.GetBuild(buildID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}
	if uid != build.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	var req struct {
		Decision string `json:"decision" binding:"required"`
		Mode     string `json:"mode"`
		Note     string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	interaction, resolved, err := h.manager.ResolvePermissionRequest(
		buildID,
		requestID,
		normalizePermissionDecision(req.Decision),
		normalizePermissionMode(req.Mode),
		req.Note,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"request":     resolved,
		"interaction": interaction,
	})
}

// PauseBuild pauses an active build between phases or queued tasks.
// POST /api/v1/build/:id/pause
func (h *BuildHandler) PauseBuild(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	_, restoredSession, err := h.getBuildActionSession(uid, buildID)
	if err != nil {
		writeBuildActionSessionError(c, err)
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)

	interaction, err := h.manager.PauseBuild(buildID, req.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":           "paused",
		"interaction":      interaction,
		"live":             true,
		"restored_session": restoredSession,
	})
}

// ResumeBuild resumes a paused build.
// POST /api/v1/build/:id/resume
func (h *BuildHandler) ResumeBuild(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	_, restoredSession, err := h.getBuildActionSession(uid, buildID)
	if err != nil {
		writeBuildActionSessionError(c, err)
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)

	interaction, err := h.manager.ResumeBuild(buildID, req.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":           "resumed",
		"interaction":      interaction,
		"live":             true,
		"restored_session": restoredSession,
	})
}

// GetCheckpoints returns all checkpoints for a build
// GET /api/v1/build/:id/checkpoints
func (h *BuildHandler) GetCheckpoints(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	build, err := h.manager.GetBuild(buildID)
	if err == nil {
		if uid != build.UserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		build.mu.RLock()
		defer build.mu.RUnlock()

		c.JSON(http.StatusOK, gin.H{
			"build_id":    buildID,
			"checkpoints": build.Checkpoints,
			"count":       len(build.Checkpoints),
			"live":        true,
		})
		return
	}

	snapshot, snapErr := h.getBuildSnapshot(uid, buildID)
	if snapErr != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "build not found",
			"details": err.Error(),
		})
		return
	}

	checkpoints := parseBuildCheckpoints(snapshot.CheckpointsJSON)
	c.JSON(http.StatusOK, gin.H{
		"build_id":    buildID,
		"checkpoints": checkpoints,
		"count":       len(checkpoints),
		"live":        false,
	})
}

// RollbackCheckpoint rolls back to a specific checkpoint
// POST /api/v1/build/:id/rollback/:checkpointId
func (h *BuildHandler) RollbackCheckpoint(c *gin.Context) {
	buildID := c.Param("id")
	checkpointID := c.Param("checkpointId")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	// Verify build exists and ownership
	build, err := h.manager.GetBuild(buildID)
	if err != nil {
		if _, snapErr := h.getBuildSnapshot(uid, buildID); snapErr == nil {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "rollback unavailable",
				"details": "Rollback is only supported for active live builds",
			})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "build not found",
			"details": err.Error(),
		})
		return
	}

	if uid != build.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Perform rollback
	if err := h.manager.RollbackToCheckpoint(buildID, checkpointID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "rollback failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "rolled_back",
		"checkpoint_id": checkpointID,
		"message":       "Successfully rolled back to checkpoint",
	})
}

// GetAgents returns all agents for a build
// GET /api/v1/build/:id/agents
func (h *BuildHandler) GetAgents(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	build, err := h.manager.GetBuild(buildID)
	if err == nil {
		if uid != build.UserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		build.mu.RLock()
		defer build.mu.RUnlock()

		agents := make([]*Agent, 0, len(build.Agents))
		for _, agent := range build.Agents {
			agents = append(agents, agent)
		}

		c.JSON(http.StatusOK, gin.H{
			"build_id": buildID,
			"agents":   agents,
			"count":    len(agents),
			"live":     true,
		})
		return
	}

	snapshot, snapErr := h.getBuildSnapshot(uid, buildID)
	if snapErr != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "build not found",
			"details": err.Error(),
		})
		return
	}

	agents := orderedBuildAgents(parseBuildAgents(snapshot.AgentsJSON))
	c.JSON(http.StatusOK, gin.H{
		"build_id": buildID,
		"agents":   agents,
		"count":    len(agents),
		"live":     false,
	})
}

// GetTasks returns all tasks for a build
// GET /api/v1/build/:id/tasks
func (h *BuildHandler) GetTasks(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	build, err := h.manager.GetBuild(buildID)
	if err == nil {
		if uid != build.UserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		build.mu.RLock()
		defer build.mu.RUnlock()

		tasksByStatus := make(map[string][]*Task)
		for _, task := range build.Tasks {
			status := string(task.Status)
			tasksByStatus[status] = append(tasksByStatus[status], task)
		}

		c.JSON(http.StatusOK, gin.H{
			"build_id":        buildID,
			"tasks":           build.Tasks,
			"tasks_by_status": tasksByStatus,
			"total":           len(build.Tasks),
			"live":            true,
		})
		return
	}

	snapshot, snapErr := h.getBuildSnapshot(uid, buildID)
	if snapErr != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "build not found",
			"details": err.Error(),
		})
		return
	}

	tasks := parseBuildTasks(snapshot.TasksJSON)
	tasksByStatus := make(map[string][]*Task)
	for _, task := range tasks {
		status := string(task.Status)
		tasksByStatus[status] = append(tasksByStatus[status], task)
	}

	c.JSON(http.StatusOK, gin.H{
		"build_id":        buildID,
		"tasks":           tasks,
		"tasks_by_status": tasksByStatus,
		"total":           len(tasks),
		"live":            false,
	})
}

// GetGeneratedFiles returns all files generated during the build
// GET /api/v1/build/:id/files
func (h *BuildHandler) GetGeneratedFiles(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	build, err := h.manager.GetBuild(buildID)
	if err == nil {
		// Verify ownership
		if uid != build.UserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		build.mu.RLock()
		defer build.mu.RUnlock()

		// Return the canonical deduplicated artifact set used for finalization/checkpointing.
		files := h.manager.collectGeneratedFiles(build)

		c.JSON(http.StatusOK, gin.H{
			"build_id": buildID,
			"files":    files,
			"count":    len(files),
			"live":     true,
		})
		return
	}

	snapshot, snapErr := h.getBuildSnapshot(uid, buildID)
	if snapErr != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "build not found",
			"details": err.Error(),
		})
		return
	}

	files, _ := parseBuildFiles(snapshot.FilesJSON)
	c.JSON(http.StatusOK, gin.H{
		"build_id": buildID,
		"files":    files,
		"count":    len(files),
		"live":     false,
	})
}

// GetBuildArtifacts returns the canonical artifact manifest for a build.
// GET /api/v1/build/:id/artifacts
func (h *BuildHandler) GetBuildArtifacts(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	manifest, live, err := h.loadArtifactManifestForUser(uid, buildID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "build not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"build_id":   buildID,
		"manifest":   manifest,
		"live":       live,
		"revision":   manifest.Revision,
		"files":      len(manifest.Files),
		"source":     manifest.Source,
		"project_id": manifest.ProjectID,
	})
}

// ApplyBuildArtifacts transactionally applies a build's canonical artifact manifest to a project.
// POST /api/v1/build/:id/apply
func (h *BuildHandler) ApplyBuildArtifacts(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	var req struct {
		ProjectID      *uint  `json:"project_id"`
		ProjectName    string `json:"project_name,omitempty"`
		ReplaceMissing *bool  `json:"replace_missing,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && strings.TrimSpace(err.Error()) != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"details": err.Error(),
		})
		return
	}

	manifest, live, err := h.loadArtifactManifestForUser(uid, buildID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "build not found",
			"details": err.Error(),
		})
		return
	}
	if len(manifest.Files) == 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "no artifacts available",
			"details": "This build has no canonical artifacts to apply.",
		})
		return
	}

	replaceMissing := true
	if req.ReplaceMissing != nil {
		replaceMissing = *req.ReplaceMissing
	}

	var result ApplyArtifactsResult
	var targetProjectID uint
	var createdProject bool

	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}

	err = h.db.Transaction(func(tx *gorm.DB) error {
		var project models.Project
		projectResolved := false

		resolveExisting := func(projectID uint) error {
			if err := tx.Where("id = ? AND owner_id = ?", projectID, uid).First(&project).Error; err != nil {
				return err
			}
			projectResolved = true
			return nil
		}

		switch {
		case req.ProjectID != nil && *req.ProjectID != 0:
			if err := resolveExisting(*req.ProjectID); err != nil {
				return err
			}
		case manifest.ProjectID != nil && *manifest.ProjectID != 0:
			if err := resolveExisting(*manifest.ProjectID); err != nil {
				// Fall back to creating a new project if the old link no longer exists or is not accessible.
				projectResolved = false
			}
		}

		if !projectResolved {
			description := strings.TrimSpace(manifest.Description)
			if description == "" {
				description = "Generated App"
			}
			created, err := createProjectForArtifactManifestTx(tx, uid, description, manifest)
			if err != nil {
				return err
			}
			project = *created
			projectResolved = true
			createdProject = true
			if strings.TrimSpace(req.ProjectName) != "" {
				if err := tx.Model(&models.Project{}).Where("id = ?", project.ID).Update("name", strings.TrimSpace(req.ProjectName)).Error; err != nil {
					return err
				}
				project.Name = strings.TrimSpace(req.ProjectName)
			}
		}

		applied, err := applyArtifactManifestTx(tx, &project, uid, manifest, replaceMissing)
		if err != nil {
			return err
		}
		result = applied
		result.CreatedProject = createdProject
		targetProjectID = project.ID

		// Persist linkage for completed builds (idempotent even if the row does not exist yet).
		if err := tx.Model(&models.CompletedBuild{}).
			Where("build_id = ? AND user_id = ?", buildID, uid).
			Updates(map[string]any{
				"project_id":   project.ID,
				"project_name": project.Name,
				"updated_at":   time.Now(),
			}).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found or access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to apply build artifacts",
			"details": err.Error(),
		})
		return
	}

	if live {
		if build, getErr := h.manager.GetBuild(buildID); getErr == nil {
			build.mu.Lock()
			build.ProjectID = &targetProjectID
			build.UpdatedAt = time.Now()
			build.mu.Unlock()
		}
	}
	manifest.ProjectID = &targetProjectID

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"build_id":         buildID,
		"project_id":       targetProjectID,
		"created_project":  result.CreatedProject,
		"applied_revision": result.Manifest,
		"applied_files":    result.AppliedFiles,
		"deleted_files":    result.DeletedFiles,
		"replace_missing":  replaceMissing,
		"manifest":         manifest,
		"message":          "Build artifacts applied successfully",
	})
}

// CancelBuild cancels an in-progress build
// POST /api/v1/build/:id/cancel
func (h *BuildHandler) CancelBuild(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	_, restoredSession, err := h.getBuildActionSession(uid, buildID)
	if err != nil {
		writeBuildActionSessionError(c, err)
		return
	}

	if err := h.manager.CancelBuild(buildID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "build cannot be cancelled",
			"details": err.Error(),
		})
		return
	}

	if h.hub != nil {
		h.hub.CloseAllConnections(buildID)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":           "cancelled",
		"message":          "Build has been cancelled",
		"live":             true,
		"restored_session": restoredSession,
	})
}

// ListBuilds returns all completed builds for the authenticated user
// GET /api/v1/builds
func (h *BuildHandler) ListBuilds(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "build history not available"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var builds []models.CompletedBuild
	var total int64

	h.db.Model(&models.CompletedBuild{}).Where("user_id = ?", uid).Count(&total)
	h.db.Where("user_id = ?", uid).Order("updated_at DESC").Offset(offset).Limit(limit).Find(&builds)

	// Convert to response format (exclude raw files JSON, include file count)
	type BuildSummary struct {
		ID          uint    `json:"id"`
		BuildID     string  `json:"build_id"`
		ProjectID   *uint   `json:"project_id,omitempty"`
		ProjectName string  `json:"project_name"`
		Description string  `json:"description"`
		Status      string  `json:"status"`
		Mode        string  `json:"mode"`
		PowerMode   string  `json:"power_mode"`
		TechStack   any     `json:"tech_stack"`
		FilesCount  int     `json:"files_count"`
		TotalCost   float64 `json:"total_cost"`
		Progress    int     `json:"progress"`
		DurationMs  int64   `json:"duration_ms"`
		CreatedAt   string  `json:"created_at"`
		CompletedAt *string `json:"completed_at,omitempty"`
		Live        bool    `json:"live"`
		Resumable   bool    `json:"resumable"`
	}

	summaries := make([]BuildSummary, 0, len(builds))
	for _, b := range builds {
		var techStack any
		if b.TechStack != "" {
			json.Unmarshal([]byte(b.TechStack), &techStack)
		}
		s := BuildSummary{
			ID:          b.ID,
			BuildID:     b.BuildID,
			ProjectID:   b.ProjectID,
			ProjectName: b.ProjectName,
			Description: b.Description,
			Status:      b.Status,
			Mode:        b.Mode,
			PowerMode:   b.PowerMode,
			TechStack:   techStack,
			FilesCount:  b.FilesCount,
			TotalCost:   b.TotalCost,
			Progress:    b.Progress,
			DurationMs:  b.DurationMs,
			CreatedAt:   b.CreatedAt.Format("2006-01-02T15:04:05Z"),
			Live:        false,
			Resumable:   isActiveBuildStatus(b.Status),
		}
		if _, liveErr := h.manager.GetBuild(b.BuildID); liveErr == nil {
			s.Live = true
		}
		if b.CompletedAt != nil {
			t := b.CompletedAt.Format("2006-01-02T15:04:05Z")
			s.CompletedAt = &t
		}
		summaries = append(summaries, s)
	}

	c.JSON(http.StatusOK, gin.H{
		"builds": summaries,
		"total":  total,
		"page":   page,
		"limit":  limit,
	})
}

// GetCompletedBuild returns a specific completed build with all file data
// GET /api/v1/builds/:buildId
func (h *BuildHandler) GetCompletedBuild(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "build history not available"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}
	buildID := c.Param("buildId")

	var build models.CompletedBuild
	if err := h.db.Where("build_id = ? AND user_id = ?", buildID, uid).First(&build).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}

	// Parse stored files JSON
	files, _ := parseBuildFiles(build.FilesJSON)
	agents := orderedBuildAgents(parseBuildAgents(build.AgentsJSON))
	tasks := parseBuildTasks(build.TasksJSON)
	checkpoints := parseBuildCheckpoints(build.CheckpointsJSON)
	interaction := parseBuildInteraction(build.InteractionJSON)
	activityTimeline := parseBuildActivityTimeline(build.ActivityJSON)
	snapshotState := parseBuildSnapshotState(build.StateJSON)

	var techStack any
	if build.TechStack != "" {
		json.Unmarshal([]byte(build.TechStack), &techStack)
	}
	live := false
	if _, liveErr := h.manager.GetBuild(build.BuildID); liveErr == nil {
		live = true
	}

	response := gin.H{
		"id":                build.ID,
		"build_id":          build.BuildID,
		"project_id":        build.ProjectID,
		"project_name":      build.ProjectName,
		"description":       build.Description,
		"status":            build.Status,
		"mode":              build.Mode,
		"power_mode":        build.PowerMode,
		"tech_stack":        techStack,
		"agents":            agents,
		"tasks":             tasks,
		"checkpoints":       checkpoints,
		"files":             files,
		"messages":          interaction.Messages,
		"interaction":       interaction,
		"activity_timeline": activityTimeline,
		"files_count":       build.FilesCount,
		"total_cost":        build.TotalCost,
		"progress":          build.Progress,
		"duration_ms":       build.DurationMs,
		"error":             build.Error,
		"created_at":        build.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"completed_at":      build.CompletedAt,
		"live":              live,
		"resumable":         isActiveBuildStatus(build.Status),
	}
	for key, value := range buildSnapshotStateResponseFields(snapshotState, build.Status) {
		response[key] = value
	}
	c.JSON(http.StatusOK, response)
}

// DownloadCompletedBuild streams a completed build as a ZIP archive
// GET /api/v1/builds/:buildId/download
func (h *BuildHandler) DownloadCompletedBuild(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "build history not available"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}
	buildID := c.Param("buildId")

	var build models.CompletedBuild
	if err := h.db.Where("build_id = ? AND user_id = ?", buildID, uid).First(&build).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}

	// Parse stored files JSON
	files, _ := parseBuildFiles(build.FilesJSON)
	if len(files) == 0 {
		// Fallback: if build is currently live, export latest in-memory files.
		if liveBuild, liveErr := h.manager.GetBuild(buildID); liveErr == nil && liveBuild.UserID == uid {
			files = h.manager.collectGeneratedFiles(liveBuild)
		}
	}

	if len(files) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no files available for this build"})
		return
	}

	buildStatus := normalizeRestoredBuildStatus(&build)
	if buildStatus != BuildCompleted {
		c.JSON(http.StatusConflict, gin.H{
			"error":        "build is not exportable",
			"details":      "Only completed, validated builds can be downloaded as ZIP archives.",
			"build_status": build.Status,
		})
		return
	}

	if h.manager != nil {
		validationBuild := &Build{
			ID:                  build.BuildID,
			UserID:              build.UserID,
			Status:              buildStatus,
			Mode:                BuildMode(build.Mode),
			PowerMode:           PowerMode(build.PowerMode),
			RequirePreviewReady: true,
			Description:         build.Description,
		}
		if strings.TrimSpace(build.TechStack) != "" {
			var techStack TechStack
			if err := json.Unmarshal([]byte(build.TechStack), &techStack); err == nil {
				validationBuild.TechStack = &techStack
			}
		}
		if validationErrors := h.manager.validateFinalBuildReadiness(validationBuild, files); len(validationErrors) > 0 {
			c.JSON(http.StatusConflict, gin.H{
				"error":             "build artifacts failed final validation",
				"details":           "This snapshot is incomplete or broken and cannot be exported as a ZIP archive.",
				"build_status":      build.Status,
				"validation_errors": validationErrors,
			})
			return
		}
	}

	projectName := strings.TrimSpace(build.ProjectName)
	if projectName == "" {
		projectName = "apex-build"
	}

	c.Header("Content-Type", "application/zip")
	suffix := build.BuildID
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-%s.zip\"", projectName, suffix))

	zipWriter := zip.NewWriter(c.Writer)
	defer zipWriter.Close()

	for _, file := range files {
		if file.Path == "" || file.Content == "" {
			continue
		}
		cleanPath := filepath.Clean(strings.TrimSpace(file.Path))
		if cleanPath == "." || cleanPath == "" {
			continue
		}
		if filepath.IsAbs(cleanPath) || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
			log.Printf("Skipping suspicious build artifact path during download: %q", file.Path)
			continue
		}

		path := strings.TrimPrefix(filepath.ToSlash(cleanPath), "/")
		if path == "" || path == "." || strings.HasPrefix(path, "../") {
			log.Printf("Skipping suspicious normalized build artifact path during download: %q", file.Path)
			continue
		}
		w, err := zipWriter.Create(path)
		if err != nil {
			continue
		}
		if _, err := w.Write([]byte(file.Content)); err != nil {
			continue
		}
	}
}

func (h *BuildHandler) getBuildSnapshot(userID uint, buildID string) (*models.CompletedBuild, error) {
	if h.db == nil {
		return nil, fmt.Errorf("build history not available")
	}

	var snapshot models.CompletedBuild
	if err := h.db.Where("build_id = ? AND user_id = ?", buildID, userID).
		Order("updated_at DESC").
		Order("id DESC").
		First(&snapshot).Error; err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func parseBuildFiles(filesJSON string) ([]GeneratedFile, error) {
	if strings.TrimSpace(filesJSON) == "" {
		return []GeneratedFile{}, nil
	}
	var files []GeneratedFile
	if err := json.Unmarshal([]byte(filesJSON), &files); err != nil {
		return nil, err
	}
	return files, nil
}

func parseBuildInteraction(raw string) BuildInteractionState {
	if strings.TrimSpace(raw) == "" {
		return BuildInteractionState{}
	}
	var interaction BuildInteractionState
	if err := json.Unmarshal([]byte(raw), &interaction); err != nil {
		return BuildInteractionState{}
	}
	return interaction
}

func parseBuildActivityTimeline(raw string) []BuildActivityEntry {
	if strings.TrimSpace(raw) == "" {
		return []BuildActivityEntry{}
	}
	var timeline []BuildActivityEntry
	if err := json.Unmarshal([]byte(raw), &timeline); err != nil {
		return []BuildActivityEntry{}
	}
	return timeline
}

func (h *BuildHandler) loadArtifactManifestForUser(userID uint, buildID string) (BuildArtifactManifest, bool, error) {
	if build, err := h.manager.GetBuild(buildID); err == nil {
		if userID != build.UserID {
			return BuildArtifactManifest{}, false, fmt.Errorf("access denied")
		}

		build.mu.RLock()
		files := h.manager.collectGeneratedFiles(build)
		projectID := build.ProjectID
		buildError := strings.TrimSpace(build.Error)
		buildStatus := string(build.Status)
		build.mu.RUnlock()

		manifest := buildArtifactManifest(buildID, "live", build.Description, projectID, files)
		if buildError != "" {
			manifest.Errors = append(manifest.Errors, buildError)
		}
		manifest.Verification["build_status"] = buildStatus
		return manifest, true, nil
	}

	snapshot, err := h.getBuildSnapshot(userID, buildID)
	if err != nil {
		return BuildArtifactManifest{}, false, err
	}
	files, parseErr := parseBuildFiles(snapshot.FilesJSON)
	if parseErr != nil {
		return BuildArtifactManifest{}, false, parseErr
	}

	manifest := buildArtifactManifest(buildID, "snapshot", snapshot.Description, snapshot.ProjectID, files)
	if strings.TrimSpace(snapshot.Error) != "" {
		manifest.Errors = append(manifest.Errors, strings.TrimSpace(snapshot.Error))
	}
	manifest.Verification["build_status"] = snapshot.Status
	manifest.GeneratedAt = snapshot.UpdatedAt
	return manifest, false, nil
}

func isActiveBuildStatus(status string) bool {
	switch status {
	case string(BuildPending), string(BuildPlanning), string(BuildInProgress), string(BuildTesting), string(BuildReviewing), string(BuildAwaitingReview):
		return true
	default:
		return false
	}
}

// KillAllBuilds cancels all active builds for the authenticated user
// POST /api/v1/build/kill-all
func (h *BuildHandler) KillAllBuilds(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	killed := h.manager.KillAllBuilds(uid)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"killed":  killed,
		"message": fmt.Sprintf("Killed %d active builds", killed),
	})
}

// GetProposedEdits returns pending proposed edits for a build
// GET /api/v1/build/:id/proposed-edits
func (h *BuildHandler) GetProposedEdits(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	build, err := h.manager.GetBuild(buildID)
	live := err == nil
	if err == nil {
		if uid != build.UserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
	} else if _, snapErr := h.getBuildSnapshot(uid, buildID); snapErr != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}

	edits := h.manager.editStore.GetAllEdits(buildID)
	c.JSON(http.StatusOK, gin.H{
		"build_id": buildID,
		"edits":    edits,
		"count":    len(edits),
		"live":     live,
	})
}

// ApproveEdits approves specific proposed edits
// POST /api/v1/build/:id/approve-edits
func (h *BuildHandler) ApproveEdits(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID)
	if err != nil {
		writeBuildActionSessionError(c, err)
		return
	}

	var req struct {
		EditIDs []string `json:"edit_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	approved, err := h.manager.editStore.ApproveEdits(buildID, req.EditIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Write approved files and resume build
	h.manager.applyApprovedEdits(build, approved)

	c.JSON(http.StatusOK, gin.H{
		"approved":         len(approved),
		"message":          "Edits approved and applied",
		"live":             true,
		"restored_session": restoredSession,
	})
}

// RejectEdits rejects specific proposed edits
// POST /api/v1/build/:id/reject-edits
func (h *BuildHandler) RejectEdits(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID)
	if err != nil {
		writeBuildActionSessionError(c, err)
		return
	}

	var req struct {
		EditIDs []string `json:"edit_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.editStore.RejectEdits(buildID, req.EditIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if there are remaining pending edits; if none, resume build
	pending := h.manager.editStore.GetPendingEdits(buildID)
	if len(pending) == 0 {
		h.manager.resumeBuildAfterReview(build)
	}

	c.JSON(http.StatusOK, gin.H{
		"rejected":         len(req.EditIDs),
		"live":             true,
		"restored_session": restoredSession,
	})
}

// ApproveAllEdits approves all pending proposed edits
// POST /api/v1/build/:id/approve-all
func (h *BuildHandler) ApproveAllEdits(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID)
	if err != nil {
		writeBuildActionSessionError(c, err)
		return
	}

	approved := h.manager.editStore.ApproveAll(buildID)
	h.manager.applyApprovedEdits(build, approved)

	c.JSON(http.StatusOK, gin.H{
		"approved":         len(approved),
		"message":          "All edits approved and applied",
		"live":             true,
		"restored_session": restoredSession,
	})
}

// RejectAllEdits rejects all pending proposed edits
// POST /api/v1/build/:id/reject-all
func (h *BuildHandler) RejectAllEdits(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, ok := userIDFromValue(c, userID)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID)
	if err != nil {
		writeBuildActionSessionError(c, err)
		return
	}

	_ = h.manager.editStore.RejectAll(buildID)
	h.manager.resumeBuildAfterReview(build)

	c.JSON(http.StatusOK, gin.H{
		"message":          "All edits rejected, build resuming",
		"live":             true,
		"restored_session": restoredSession,
	})
}

// RegisterRoutes registers all build routes on the router
func (h *BuildHandler) RegisterRoutes(rg *gin.RouterGroup) {
	build := rg.Group("/build")
	{
		build.POST("/preflight", h.PreflightCheck)
		build.POST("/start", h.StartBuild)
		build.GET("/:id", h.GetBuildDetails)
		build.GET("/:id/status", h.GetBuildStatus)
		build.POST("/:id/message", h.SendMessage)
		build.GET("/:id/messages", h.GetMessages)
		build.GET("/:id/permissions", h.GetPermissions)
		build.POST("/:id/permissions/rules", h.SetPermissionRule)
		build.POST("/:id/permissions/requests/:requestId/resolve", h.ResolvePermissionRequest)
		build.POST("/:id/pause", h.PauseBuild)
		build.POST("/:id/resume", h.ResumeBuild)
		build.GET("/:id/checkpoints", h.GetCheckpoints)
		build.POST("/:id/rollback/:checkpointId", h.RollbackCheckpoint)
		build.GET("/:id/agents", h.GetAgents)
		build.GET("/:id/tasks", h.GetTasks)
		build.GET("/:id/files", h.GetGeneratedFiles)
		build.GET("/:id/artifacts", h.GetBuildArtifacts)
		build.POST("/:id/apply", h.ApplyBuildArtifacts)
		build.POST("/:id/cancel", h.CancelBuild)
		build.POST("/kill-all", h.KillAllBuilds)

		// Diff workflow routes
		build.GET("/:id/proposed-edits", h.GetProposedEdits)
		build.POST("/:id/approve-edits", h.ApproveEdits)
		build.POST("/:id/reject-edits", h.RejectEdits)
		build.POST("/:id/approve-all", h.ApproveAllEdits)
		build.POST("/:id/reject-all", h.RejectAllEdits)
	}

	// Build history endpoints
	rg.GET("/builds", h.ListBuilds)
	rg.GET("/builds/:buildId", h.GetCompletedBuild)
	rg.GET("/builds/:buildId/download", h.DownloadCompletedBuild)
}
