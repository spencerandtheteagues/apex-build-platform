// Package agents - HTTP API Handlers
// RESTful endpoints for build management
package agents

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"apex-build/internal/ai"
	"apex-build/internal/applog"
	appmiddleware "apex-build/internal/middleware"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	errBuildActionNotFound = errors.New("build not found")
	errBuildAccessDenied   = errors.New("access denied")
	errBuildLookupTimeout  = errors.New("live build lookup timed out")
	errBuildReadTimeout    = errors.New("live build read timed out")
)

var readableBuildLookupTimeout = 2 * time.Second
var readableBuildStateTimeout = 2 * time.Second

// BuildHandler handles build-related HTTP requests
type BuildHandler struct {
	manager *AgentManager
	hub     *WSHub
	db      *gorm.DB
}

type buildPlatformIssue struct {
	Service           string
	IssueType         string
	Summary           string
	Retryable         bool
	MaintenanceWindow bool
}

func classifyBuildMessageError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if _, ok := asBuildSubscriptionRequiredError(err); ok {
		return http.StatusPaymentRequired
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "message cannot be empty"),
		strings.Contains(message, "no agents matched the selected target"),
		strings.Contains(message, "unknown command"):
		return http.StatusBadRequest
	case strings.Contains(message, "direct agent messages require an active build"),
		strings.Contains(message, "restart is only available for failed builds"),
		strings.Contains(message, "restart is not available for completed or cancelled builds"):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func buildMessageErrorResponse(err error) gin.H {
	if ent, ok := asBuildSubscriptionRequiredError(err); ok {
		return gin.H{
			"error":          ent.BlockedReason + " requires a paid subscription",
			"details":        err.Error(),
			"error_code":     backendSubscriptionRequiredCode,
			"current_plan":   firstNonEmptyString(ent.CurrentPlan, "free"),
			"required_plan":  firstNonEmptyString(ent.RequiredPlan, "builder"),
			"blocked_reason": ent.BlockedReason,
			"suggestion":     ent.Suggestion,
		}
	}

	return gin.H{
		"error":   "failed to send message",
		"details": err.Error(),
	}
}

// NewBuildHandler creates a new build handler
func NewBuildHandler(manager *AgentManager, hub *WSHub) *BuildHandler {
	return &BuildHandler{
		manager: manager,
		hub:     hub,
		db:      manager.db,
	}
}

func buildPlatformIssueFromError(err error) *buildPlatformIssue {
	if err == nil {
		return nil
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if message == "" {
		return nil
	}

	if strings.Contains(message, "redis") {
		if strings.Contains(message, "allowlist") {
			return &buildPlatformIssue{
				Service:           "redis_cache",
				IssueType:         "platform_configuration",
				Summary:           "Redis is configured with an external allowlisted endpoint. Point REDIS_URL at the internal Render Key Value connection string so live build coordination can recover.",
				Retryable:         false,
				MaintenanceWindow: false,
			}
		}
		maintenance := looksLikeMaintenanceWindow(message)
		issueType := "platform_service_interruption"
		if maintenance {
			issueType = "platform_maintenance"
		}
		return &buildPlatformIssue{
			Service:           "redis_cache",
			IssueType:         issueType,
			Summary:           "Redis connectivity is temporarily degraded. Live coordination may lag while the platform reconnects.",
			Retryable:         true,
			MaintenanceWindow: maintenance,
		}
	}

	if isDatabaseIssueError(err) || isDatabaseIssueMessage(message) {
		maintenance := looksLikeMaintenanceWindow(message)
		issueType := "platform_service_interruption"
		if maintenance {
			issueType = "platform_maintenance"
		}
		return &buildPlatformIssue{
			Service:           "primary_database",
			IssueType:         issueType,
			Summary:           "Primary database connectivity is temporarily unavailable. Build history, restore, and status sync can pause until the platform reconnects.",
			Retryable:         true,
			MaintenanceWindow: maintenance,
		}
	}

	return nil
}

func isDatabaseIssueError(err error) bool {
	return errors.Is(err, sql.ErrConnDone) || errors.Is(err, gorm.ErrInvalidDB)
}

func isDatabaseIssueMessage(message string) bool {
	indicators := []string{
		"build history not available",
		"database unavailable",
		"database is closed",
		"driver: bad connection",
		"bad connection",
		"sqlstate",
		"sql:",
		"dial tcp",
		"connection refused",
		"connection reset",
		"broken pipe",
		"server closed the connection unexpectedly",
		"terminating connection",
		"the database system is starting up",
		"could not connect to server",
		"connection exception",
		"connection timed out",
		"i/o timeout",
		"no such host",
		"timeout expired",
	}
	for _, indicator := range indicators {
		if strings.Contains(message, indicator) {
			return true
		}
	}
	return false
}

func looksLikeMaintenanceWindow(message string) bool {
	indicators := []string{
		"maintenance",
		"server closed the connection unexpectedly",
		"terminating connection",
		"the database system is starting up",
		"connection reset",
		"broken pipe",
		"driver: bad connection",
		"database is closed",
	}
	for _, indicator := range indicators {
		if strings.Contains(message, indicator) {
			return true
		}
	}
	return false
}

func buildPlatformIssueResponse(err error, fallbackError string, fallbackDetails string) gin.H {
	response := gin.H{
		"error": fallbackError,
	}
	details := strings.TrimSpace(fallbackDetails)
	if details == "" && err != nil {
		details = strings.TrimSpace(err.Error())
	}
	if details != "" {
		response["details"] = details
	}

	if issue := buildPlatformIssueFromError(err); issue != nil {
		response["platform_issue"] = true
		response["platform_service"] = issue.Service
		response["platform_issue_type"] = issue.IssueType
		response["platform_issue_summary"] = issue.Summary
		response["retryable"] = issue.Retryable
		response["maintenance_window"] = issue.MaintenanceWindow
	}

	return response
}

func retryBuildHistoryRead(operation string, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		lastErr = fn()
		if lastErr == nil || errors.Is(lastErr, gorm.ErrRecordNotFound) {
			return lastErr
		}
		if buildPlatformIssueFromError(lastErr) == nil {
			return lastErr
		}
		if attempt == 3 {
			return lastErr
		}
		time.Sleep(time.Duration(attempt+1) * 150 * time.Millisecond)
	}

	if lastErr != nil {
		log.Printf("build history read %s exhausted retries: %v", operation, lastErr)
	}
	return lastErr
}

func promptPackActivationRequestsEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(promptPackActivationFeatureFlag)), "true")
}

func requestContextIsAdmin(c *gin.Context) bool {
	for _, key := range []string{"is_admin", "is_super_admin"} {
		value, exists := c.Get(key)
		if !exists {
			continue
		}
		if isAdmin, ok := value.(bool); ok && isAdmin {
			return true
		}
	}
	return false
}

func writeBuildLookupError(c *gin.Context, err error, fallbackErr error) {
	lookupErr := err
	if lookupErr == nil {
		lookupErr = fallbackErr
	}
	if errors.Is(lookupErr, errBuildLookupTimeout) || errors.Is(lookupErr, errBuildReadTimeout) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "build session unavailable",
			"details": "The live build session did not respond in time. A saved snapshot may still be available.",
		})
		return
	}
	if buildPlatformIssueFromError(lookupErr) != nil {
		c.JSON(http.StatusServiceUnavailable, buildPlatformIssueResponse(lookupErr, "build session unavailable", "The build state could not be loaded because a platform service is temporarily unavailable."))
		return
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error":   "build not found",
		"details": fallbackErr.Error(),
	})
}

func requestedBuildProviderMode(providerMode string) string {
	if strings.EqualFold(strings.TrimSpace(providerMode), "byok") {
		return "byok"
	}
	return "platform"
}

func providersForBuildProviderMode(router AIRouter, userID uint, providerMode string) []ai.AIProvider {
	if router == nil {
		return nil
	}
	if requestedBuildProviderMode(providerMode) == "byok" {
		return router.GetAvailableProvidersForUser(userID)
	}
	return hostedPlatformProviders(router.GetAvailableProviders())
}

// PreflightCheck validates provider credentials and billing status before a build.
// POST /api/v1/build/preflight
func (h *BuildHandler) PreflightCheck(c *gin.Context) {
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	type preflightResult struct {
		Ready            bool                     `json:"ready"`
		Providers        int                      `json:"providers_available"`
		Names            []string                 `json:"provider_names"`
		ProviderStatuses map[string]string        `json:"provider_statuses,omitempty"`
		HasBYOK          bool                     `json:"has_byok"`
		CapabilityState  *BuildCapabilityState    `json:"capability_detector,omitempty"`
		PolicyState      *BuildPolicyState        `json:"policy,omitempty"`
		Classification   BuildClassificationState `json:"classification,omitempty"`
		UpgradeRequired  bool                     `json:"upgrade_required"`
		ErrorCode        string                   `json:"error_code,omitempty"`
		Error            string                   `json:"error,omitempty"`
		Suggestion       string                   `json:"suggestion,omitempty"`
	}

	var req BuildRequest
	if c.Request.Body != nil {
		if err := c.ShouldBindJSON(&req); err != nil && !strings.Contains(strings.ToLower(err.Error()), "eof") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid request",
				"details": err.Error(),
			})
			return
		}
	}
	if req.Description == "" && req.Prompt != "" {
		req.Description = req.Prompt
	}
	if req.Prompt == "" && req.Description != "" {
		req.Prompt = req.Description
	}
	req.Description = strings.TrimSpace(req.Description)
	req.Prompt = strings.TrimSpace(req.Prompt)

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

	providerMode := requestedBuildProviderMode(req.ProviderMode)
	providers := providersForBuildProviderMode(router, uid, providerMode)
	if len(providers) == 0 {
		allProviders := hostedPlatformProviders(router.GetAvailableProviders())
		if len(allProviders) == 0 {
			c.JSON(http.StatusServiceUnavailable, preflightResult{
				ErrorCode:  "ALL_PROVIDERS_DOWN",
				Error:      "All AI providers are currently unavailable",
				Suggestion: "Check your API keys in Settings or try again shortly",
			})
			return
		}
		if providerMode == "byok" {
			c.JSON(http.StatusPaymentRequired, preflightResult{
				ErrorCode:  "BYOK_PROVIDER_UNAVAILABLE",
				Error:      "No BYOK providers available for your account",
				Suggestion: "Switch to platform routing or check your personal API keys in Settings",
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

	// Build provider status map: always include core platform providers,
	// plus hosted or BYOK Ollama when it is actually available.
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
	planType := h.currentSubscriptionType(c, uid)
	var capabilityState *BuildCapabilityState
	var policyState *BuildPolicyState
	if req.Description != "" || req.Prompt != "" || req.TechStack != nil {
		capabilityState, policyState = buildPreflightSemanticState(&req, planType)
	}
	classification := BuildClassificationState("")
	upgradeRequired := false
	if policyState != nil {
		classification = policyState.Classification
		upgradeRequired = policyState.UpgradeRequired
	}

	c.JSON(http.StatusOK, preflightResult{
		Ready:            true,
		Providers:        len(providers),
		Names:            names,
		ProviderStatuses: statuses,
		HasBYOK:          hasBYOK,
		CapabilityState:  capabilityState,
		PolicyState:      policyState,
		Classification:   classification,
		UpgradeRequired:  upgradeRequired,
	})
}

// StartBuild creates and starts a new build
// POST /api/v1/build/start
func (h *BuildHandler) StartBuild(c *gin.Context) {
	log.Printf("StartBuild handler called")

	// Get user ID from auth context
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		log.Printf("StartBuild: invalid or missing user context")
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
	if !req.RequirePreviewReady && inferIntentAppType(req.Description, req.TechStack) != "api" {
		req.RequirePreviewReady = true
	}

	planType := h.currentSubscriptionType(c, uid)
	if requiresUpgrade, reason := buildSubscriptionRequirement(&req); requiresUpgrade && !isPaidBuildPlan(planType) {
		log.Printf("StartBuild: free-tier request includes paid runtime scope (%s); enforcing frontend-only mode", reason)
		// Strip backend/database from the request so the build runs as frontend-only.
		// The preview pane backend proxy is still available for rendering the UI.
		if req.TechStack != nil {
			req.TechStack.Backend = ""
			req.TechStack.Database = ""
		}
		// Surface a clear upgrade CTA to the client.
		c.Header("X-Plan-Limit", "frontend-only")
		c.Header("X-Upgrade-Reason", reason)
		c.Header("X-Upgrade-URL", "/settings/billing")
	}

	// Validate power mode against plan tier.
	// Free → fast only; Builder → balanced max; Pro/Team/Enterprise → all modes.
	if req.PowerMode != "" {
		powerModeRank := map[PowerMode]int{PowerFast: 0, PowerBalanced: 1, PowerMax: 2}
		maxAllowed := PowerFast
		switch planType {
		case "builder":
			maxAllowed = PowerBalanced
		case "pro", "team", "enterprise", "owner":
			maxAllowed = PowerMax
		}
		reqRank, reqKnown := powerModeRank[req.PowerMode]
		maxRank := powerModeRank[maxAllowed]
		if reqKnown && reqRank > maxRank {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error":          fmt.Sprintf("Power mode %q requires a higher subscription tier", req.PowerMode),
				"error_code":     "POWER_MODE_UPGRADE_REQUIRED",
				"current_plan":   planType,
				"max_power_mode": string(maxAllowed),
				"suggestion":     "Upgrade your plan to use higher power modes, or select a lower power mode.",
			})
			return
		}
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
		}
	}

	if req.ProviderModelOverrides != nil {
		validProvs := map[string]bool{"claude": true, "gpt4": true, "gemini": true, "grok": true, "ollama": true}
		for provider, model := range req.ProviderModelOverrides {
			if !validProvs[provider] {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "invalid provider",
					"details": fmt.Sprintf("Unknown provider: %s. Valid: claude, gpt4, gemini, grok, ollama", provider),
				})
				return
			}
			normalizedModel := strings.TrimSpace(model)
			if normalizedModel != "" && !strings.EqualFold(normalizedModel, "auto") && !modelBelongsToProvider(ai.AIProvider(provider), normalizedModel) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "invalid provider model override",
					"details": fmt.Sprintf("Model %s does not belong to provider %s", model, provider),
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
		providerMode := requestedBuildProviderMode(req.ProviderMode)
		if providers := providersForBuildProviderMode(router, uid, providerMode); len(providers) == 0 {
			if providerMode == "byok" {
				c.JSON(http.StatusPaymentRequired, gin.H{
					"error":      "No BYOK providers available for your account",
					"error_code": "BYOK_PROVIDER_UNAVAILABLE",
					"suggestion": "Switch to platform routing or check your personal API keys in Settings",
				})
				return
			}
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
			// Defensively broadcast the failure to any already-connected WS clients.
			// StartBuild marks the build failed internally on most paths but a few
			// early-exit paths (e.g. persist failure before agents spawn) set the status
			// without sending a WS event, leaving the frontend in an infinite loading
			// state.  This broadcast guarantees at least one error event is emitted.
			h.manager.broadcast(build.ID, &WSMessage{
				Type:      WSBuildError,
				BuildID:   build.ID,
				Timestamp: time.Now(),
				Data: map[string]any{
					"error":   "Build failed to initialize",
					"details": err.Error(),
					"status":  string(BuildFailed),
				},
			})
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
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	userPlan := h.currentSubscriptionType(c, uid)
	build, snapshot, restored, err := h.loadReadableBuild(buildID, uid)
	if err == nil && build != nil {
		payload, readErr := h.readLiveBuildStatusPayload(build, userPlan, restored)
		if readErr == nil {
			if uid != payload.ownerID {
				c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
				return
			}
			c.JSON(http.StatusOK, payload.response)
			return
		}
		if snapshot == nil {
			snapshot, _ = h.getBuildSnapshot(uid, buildID)
		}
		if snapshot == nil {
			writeBuildLookupError(c, readErr, readErr)
			return
		}
	}

	if err != nil {
		writeBuildLookupError(c, err, err)
		return
	}
	if snapshot == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}

	snapshotStatus := string(presentedSnapshotStatus(snapshot))
	snapshotProgress := presentedSnapshotProgress(snapshot, BuildStatus(snapshotStatus))
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
	for key, value := range buildSnapshotStateResponseFields(parseBuildSnapshotState(snapshot.StateJSON), snapshotStatus, userPlan) {
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

func (h *BuildHandler) getBuildActionSession(userID uint, buildID string, resumeExecutionOnRestore bool) (*Build, bool, error) {
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

	build, restored, restoreErr := h.manager.restoreBuildSessionFromSnapshotWithOptions(snapshot, restoreBuildSessionOptions{
		resumeExecution: resumeExecutionOnRestore,
	})
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
		c.JSON(http.StatusServiceUnavailable, buildPlatformIssueResponse(err, "build session unavailable", err.Error()))
	}
}

func completedBuildSnapshotRepresentsSuccess(snapshot *models.CompletedBuild) bool {
	if snapshot == nil || snapshot.CompletedAt == nil {
		return false
	}
	if strings.TrimSpace(snapshot.Error) != "" {
		return false
	}
	return BuildStatus(strings.TrimSpace(snapshot.Status)) != BuildCancelled
}

func presentedSnapshotStatus(snapshot *models.CompletedBuild) BuildStatus {
	if snapshot == nil {
		return BuildCompleted
	}
	if completedBuildSnapshotRepresentsSuccess(snapshot) {
		return BuildCompleted
	}
	if snapshot.CompletedAt != nil && strings.TrimSpace(snapshot.Error) == "" &&
		BuildStatus(strings.TrimSpace(snapshot.Status)) == BuildCancelled {
		return BuildCancelled
	}
	return normalizeRestoredBuildStatus(snapshot)
}

func presentedSnapshotProgress(snapshot *models.CompletedBuild, status BuildStatus) int {
	if snapshot == nil {
		if status == BuildCompleted {
			return 100
		}
		return 0
	}
	progress := snapshot.Progress
	if status == BuildCompleted && progress < 100 {
		progress = 100
	}
	return progress
}

func presentedLiveBuildProgress(progress int, state BuildSnapshotState, status BuildStatus) int {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	if status == BuildCompleted && progress < 100 {
		return 100
	}
	if !isActiveBuildStatus(string(status)) {
		return progress
	}
	_, phaseMax, ok := buildPhaseProgressWindow(state.CurrentPhase, status)
	if ok && progress > phaseMax {
		return phaseMax
	}
	return progress
}

func normalizeBuildMessageProgress(msg *WSMessage, state BuildSnapshotState, status BuildStatus) {
	if msg == nil || msg.Data == nil {
		return
	}
	data, ok := msg.Data.(map[string]any)
	if !ok {
		return
	}
	progress, ok := progressValue(data["progress"])
	if !ok {
		return
	}
	data["progress"] = presentedLiveBuildProgress(progress, state, status)
}

func progressValue(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case uint:
		return int(v), true
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		return int(v), true
	case uint64:
		return int(v), true
	case float32:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return int(parsed), true
		}
	}
	return 0, false
}

// buildSnapshotStateResponseFields serialises BuildSnapshotState into a gin.H
// ready for JSON response. When currentPlan is a paid tier (non-empty, non-"free")
// any stale upgrade_required flag stored in the snapshot is cleared so that
// previously-free builds don't keep showing an upgrade gate to paying users.
func buildSnapshotStateResponseFields(state BuildSnapshotState, fallbackStatus string, currentPlan ...string) gin.H {
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
		fields["capability_detector"] = state.CapabilityState
	}
	if state.PolicyState != nil {
		policy := *state.PolicyState
		// If the caller supplies a paid plan, clear any stale upgrade gate that
		// was recorded when the build was first created under a free account.
		if len(currentPlan) > 0 && isPaidBuildPlan(currentPlan[0]) && policy.UpgradeRequired {
			policy.UpgradeRequired = false
			policy.UpgradeReason = ""
			policy.RequiredPlan = ""
			policy.Classification = "full_stack_candidate"
			policy.PlanType = currentPlan[0]
			policy.FullStackEligible = true
			policy.PublishEnabled = true
			policy.BYOKEnabled = true
			policy.StaticFrontendOnly = false
		}
		fields["policy_state"] = policy
		fields["policy"] = policy
		fields["build_classification"] = policy.Classification
		fields["classification"] = policy.Classification
		fields["upgrade_required"] = policy.UpgradeRequired
		if reason := strings.TrimSpace(policy.UpgradeReason); reason != "" {
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
		if orchestration.HistoricalLearning != nil {
			fields["historical_learning"] = orchestration.HistoricalLearning
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
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	userPlan := h.currentSubscriptionType(c, uid)
	build, snapshot, restored, err := h.loadReadableBuild(buildID, uid)
	if err == nil && build != nil {
		payload, readErr := h.readLiveBuildDetailsPayload(build, userPlan, restored)
		if readErr == nil {
			if uid != payload.ownerID {
				c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
				return
			}
			c.JSON(http.StatusOK, payload.response)
			return
		}
		if snapshot == nil {
			snapshot, _ = h.getBuildSnapshot(uid, buildID)
		}
		if snapshot == nil {
			writeBuildLookupError(c, readErr, readErr)
			return
		}
	}

	if err != nil {
		writeBuildLookupError(c, err, err)
		return
	}
	if snapshot == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}

	files, _ := parseBuildFiles(snapshot.FilesJSON)
	agents := orderedBuildAgents(parseBuildAgents(snapshot.AgentsJSON))
	tasks := parseBuildTasks(snapshot.TasksJSON)
	checkpoints := parseBuildCheckpoints(snapshot.CheckpointsJSON)
	interaction := parseBuildInteraction(snapshot.InteractionJSON)
	activityTimeline := parseBuildActivityTimeline(snapshot.ActivityJSON)
	snapshotState := parseBuildSnapshotState(snapshot.StateJSON)
	snapshotStatus := string(presentedSnapshotStatus(snapshot))
	snapshotProgress := presentedSnapshotProgress(snapshot, BuildStatus(snapshotStatus))

	response := gin.H{
		"id":                     snapshot.BuildID,
		"user_id":                snapshot.UserID,
		"project_id":             snapshot.ProjectID,
		"status":                 snapshotStatus,
		"mode":                   snapshot.Mode,
		"power_mode":             snapshot.PowerMode,
		"description":            snapshot.Description,
		"plan":                   nil,
		"agents":                 agents,
		"tasks":                  tasks,
		"checkpoints":            checkpoints,
		"progress":               snapshotProgress,
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
	for key, value := range buildSnapshotStateResponseFields(snapshotState, snapshotStatus, userPlan) {
		response[key] = value
	}
	c.JSON(http.StatusOK, response)
}

func (h *BuildHandler) loadReadableBuild(buildID string, userID uint) (*Build, *models.CompletedBuild, bool, error) {
	build, err := h.getLiveBuildForRead(buildID)
	if err == nil {
		if build.UserID != userID {
			return nil, nil, false, errBuildAccessDenied
		}
		return build, nil, false, nil
	}
	liveLookupTimedOut := errors.Is(err, errBuildLookupTimeout)
	if liveLookupTimedOut {
		log.Printf("Build %s: live readable lookup timed out after %s; falling back to snapshot", buildID, readableBuildLookupTimeout)
	}

	snapshot, snapErr := h.getBuildSnapshot(userID, buildID)
	if snapErr != nil {
		if liveLookupTimedOut {
			return nil, nil, false, errBuildLookupTimeout
		}
		return nil, nil, false, snapErr
	}

	// Claimed stale snapshots represent actively-running builds whose owner instance
	// died (server restart). We must resume execution so orphaned tasks are re-queued.
	// claimActiveSnapshotTakeover only succeeds on active-status builds with a stale
	// owner heartbeat, so completed/failed builds are never accidentally restarted here.
	if claimedSnapshot, claimed, claimErr := h.manager.claimActiveSnapshotTakeover(snapshot); claimErr == nil && claimed {
		build, restored, restoreErr := h.manager.restoreBuildSessionFromSnapshotWithOptions(claimedSnapshot, restoreBuildSessionOptions{
			resumeExecution: true,
		})
		if restoreErr == nil {
			if build.UserID != userID {
				return nil, nil, false, errBuildAccessDenied
			}
			return build, nil, restored, nil
		}
	} else if claimErr != nil {
		log.Printf("Build %s: failed to claim stale active snapshot for readable restore: %v", buildID, claimErr)
	} else if claimedSnapshot != nil {
		snapshot = claimedSnapshot
	}

	return nil, snapshot, false, nil
}

type liveBuildReadPayload struct {
	ownerID  uint
	response gin.H
}

func captureTimedLiveBuildRead[T any](timeout time.Duration, fn func() T) (T, error) {
	var zero T
	if timeout <= 0 {
		return fn(), nil
	}

	resultCh := make(chan T, 1)
	go func() {
		resultCh <- fn()
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case result := <-resultCh:
		return result, nil
	case <-timer.C:
		return zero, errBuildReadTimeout
	}
}

func (h *BuildHandler) readLiveBuildStatusPayload(build *Build, userPlan string, restored bool) (liveBuildReadPayload, error) {
	return captureTimedLiveBuildRead(readableBuildStateTimeout, func() liveBuildReadPayload {
		h.manager.selfHealReadableActiveBuild(build)

		build.mu.RLock()
		defer build.mu.RUnlock()

		errorMessage := build.Error
		if strings.TrimSpace(errorMessage) == "" && build.Status == BuildFailed {
			errorMessage = latestFailedTaskErrorLocked(build)
		}
		interaction := copyBuildInteractionStateLocked(build)
		snapshotState := copyBuildSnapshotStateLocked(build)
		displayProgress := presentedLiveBuildProgress(build.Progress, snapshotState, build.Status)

		response := gin.H{
			"id":                       build.ID,
			"status":                   string(build.Status),
			"mode":                     string(build.Mode),
			"power_mode":               string(build.PowerMode),
			"provider_mode":            build.ProviderMode,
			"require_preview_ready":    build.RequirePreviewReady,
			"description":              build.Description,
			"provider_model_overrides": cloneStringMap(build.ProviderModelOverrides),
			"progress":                 displayProgress,
			"agents_count":             len(build.Agents),
			"tasks_count":              len(build.Tasks),
			"checkpoints":              len(build.Checkpoints),
			"created_at":               build.CreatedAt,
			"updated_at":               build.UpdatedAt,
			"completed_at":             build.CompletedAt,
			"error":                    errorMessage,
			"interaction":              interaction,
			"live":                     true,
			"restored_from_snapshot":   restored,
		}
		for key, value := range buildSnapshotStateResponseFields(snapshotState, string(build.Status), userPlan) {
			response[key] = value
		}
		return liveBuildReadPayload{
			ownerID:  build.UserID,
			response: response,
		}
	})
}

func (h *BuildHandler) readLiveBuildDetailsPayload(build *Build, userPlan string, restored bool) (liveBuildReadPayload, error) {
	return captureTimedLiveBuildRead(readableBuildStateTimeout, func() liveBuildReadPayload {
		h.manager.selfHealReadableActiveBuild(build)

		build.mu.RLock()
		defer build.mu.RUnlock()

		agents := orderedBuildAgents(build.Agents)
		interaction := copyBuildInteractionStateLocked(build)
		snapshotState := copyBuildSnapshotStateLocked(build)
		displayProgress := presentedLiveBuildProgress(build.Progress, snapshotState, build.Status)

		response := gin.H{
			"id":                       build.ID,
			"user_id":                  build.UserID,
			"project_id":               build.ProjectID,
			"status":                   string(build.Status),
			"mode":                     string(build.Mode),
			"power_mode":               string(build.PowerMode),
			"provider_mode":            build.ProviderMode,
			"require_preview_ready":    build.RequirePreviewReady,
			"description":              build.Description,
			"provider_model_overrides": cloneStringMap(build.ProviderModelOverrides),
			"plan":                     build.Plan,
			"agents":                   agents,
			"tasks":                    build.Tasks,
			"checkpoints":              build.Checkpoints,
			"progress":                 displayProgress,
			"created_at":               build.CreatedAt,
			"updated_at":               build.UpdatedAt,
			"completed_at":             build.CompletedAt,
			"error":                    build.Error,
			"files":                    h.manager.collectGeneratedFiles(build),
			"messages":                 interaction.Messages,
			"interaction":              interaction,
			"activity_timeline":        copyBuildActivityTimelineLocked(build),
			"live":                     isActiveBuildStatus(string(build.Status)),
			"restored_from_snapshot":   restored,
		}
		for key, value := range buildSnapshotStateResponseFields(snapshotState, string(build.Status), userPlan) {
			response[key] = value
		}
		return liveBuildReadPayload{
			ownerID:  build.UserID,
			response: response,
		}
	})
}

func (h *BuildHandler) getLiveBuildForRead(buildID string) (*Build, error) {
	if readableBuildLookupTimeout <= 0 {
		return h.manager.GetBuild(buildID)
	}

	type buildLookupResult struct {
		build *Build
		err   error
	}

	resultCh := make(chan buildLookupResult, 1)
	go func() {
		build, err := h.manager.GetBuild(buildID)
		resultCh <- buildLookupResult{build: build, err: err}
	}()

	timer := time.NewTimer(readableBuildLookupTimeout)
	defer timer.Stop()

	select {
	case result := <-resultCh:
		return result.build, result.err
	case <-timer.C:
		return nil, errBuildLookupTimeout
	}
}

// SendMessage sends a message to the build's lead agent
// POST /api/v1/build/:id/message
func (h *BuildHandler) SendMessage(c *gin.Context) {
	buildID := c.Param("id")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	// Verify build exists and ownership
	restoredSession := false
	build, err := h.manager.GetBuild(buildID)
	if err != nil {
		snapshot, snapErr := h.getBuildSnapshot(uid, buildID)
		if snapErr != nil {
			writeBuildLookupError(c, snapErr, err)
			return
		}

		build, restoredSession, err = h.manager.restoreBuildSessionFromSnapshot(snapshot)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, buildPlatformIssueResponse(err, "build session unavailable", err.Error()))
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
		c.JSON(classifyBuildMessageError(sendErr), buildMessageErrorResponse(sendErr))
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
	uid, ok := appmiddleware.RequireUserID(c)
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
		writeBuildLookupError(c, err, err)
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
	uid, ok := appmiddleware.RequireUserID(c)
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
		writeBuildLookupError(c, err, err)
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
	uid, ok := appmiddleware.RequireUserID(c)
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
	uid, ok := appmiddleware.RequireUserID(c)
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
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	_, restoredSession, err := h.getBuildActionSession(uid, buildID, false)
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
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	_, restoredSession, err := h.getBuildActionSession(uid, buildID, false)
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

// SetProviderModelOverride updates the build's per-provider model lock.
// POST /api/v1/build/:id/provider-model
func (h *BuildHandler) SetProviderModelOverride(c *gin.Context) {
	buildID := c.Param("id")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID, false)
	if err != nil {
		writeBuildActionSessionError(c, err)
		return
	}

	var req struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"details": err.Error(),
		})
		return
	}

	provider := ai.AIProvider(strings.TrimSpace(strings.ToLower(req.Provider)))
	switch provider {
	case ai.ProviderClaude, ai.ProviderGPT4, ai.ProviderGemini, ai.ProviderGrok, ai.ProviderOllama:
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid provider",
			"details": fmt.Sprintf("Unknown provider: %s", req.Provider),
		})
		return
	}

	model := strings.TrimSpace(req.Model)
	if model != "" && !strings.EqualFold(model, "auto") && !modelBelongsToProvider(provider, model) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid provider model override",
			"details": fmt.Sprintf("Model %s does not belong to provider %s", model, provider),
		})
		return
	}

	if err := h.manager.SetProviderModelOverride(buildID, provider, model); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	build.mu.RLock()
	overrides := cloneStringMap(build.ProviderModelOverrides)
	agents := orderedBuildAgents(build.Agents)
	build.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"status":                   "updated",
		"provider_model_overrides": overrides,
		"agents":                   agents,
		"live":                     true,
		"restored_session":         restoredSession,
	})
}

// GetCheckpoints returns all checkpoints for a build
// GET /api/v1/build/:id/checkpoints
func (h *BuildHandler) GetCheckpoints(c *gin.Context) {
	buildID := c.Param("id")
	uid, ok := appmiddleware.RequireUserID(c)
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
		writeBuildLookupError(c, snapErr, err)
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
	uid, ok := appmiddleware.RequireUserID(c)
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
		} else if buildPlatformIssueFromError(snapErr) != nil {
			writeBuildLookupError(c, snapErr, err)
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
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	build, snapshot, restored, err := h.loadReadableBuild(buildID, uid)
	if err != nil {
		writeBuildLookupError(c, err, nil)
		return
	}
	if build != nil {
		payload, readErr := captureTimedLiveBuildRead(readableBuildStateTimeout, func() liveBuildReadPayload {
			h.manager.selfHealReadableActiveBuild(build)

			build.mu.RLock()
			agents := orderedBuildAgents(build.Agents)
			ownerID := build.UserID
			build.mu.RUnlock()

			return liveBuildReadPayload{
				ownerID: ownerID,
				response: gin.H{
					"build_id":               buildID,
					"agents":                 agents,
					"count":                  len(agents),
					"live":                   true,
					"restored_from_snapshot": restored,
				},
			}
		})
		if readErr == nil {
			if uid != payload.ownerID {
				c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
				return
			}
			c.JSON(http.StatusOK, payload.response)
			return
		}
		log.Printf("Build %s: live agents read timed out after %s; falling back to snapshot", buildID, readableBuildStateTimeout)
		snapshot, err = h.getBuildSnapshot(uid, buildID)
		if err != nil {
			writeBuildLookupError(c, err, readErr)
			return
		}
	}
	if snapshot == nil {
		writeBuildLookupError(c, errBuildActionNotFound, errBuildActionNotFound)
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
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	build, snapshot, restored, err := h.loadReadableBuild(buildID, uid)
	if err != nil {
		writeBuildLookupError(c, err, nil)
		return
	}
	if build != nil {
		payload, readErr := captureTimedLiveBuildRead(readableBuildStateTimeout, func() liveBuildReadPayload {
			h.manager.selfHealReadableActiveBuild(build)

			build.mu.RLock()
			tasks := append([]*Task(nil), build.Tasks...)
			tasksByStatus := make(map[string][]*Task)
			for _, task := range tasks {
				status := string(task.Status)
				tasksByStatus[status] = append(tasksByStatus[status], task)
			}
			ownerID := build.UserID
			build.mu.RUnlock()

			return liveBuildReadPayload{
				ownerID: ownerID,
				response: gin.H{
					"build_id":               buildID,
					"tasks":                  tasks,
					"tasks_by_status":        tasksByStatus,
					"total":                  len(tasks),
					"live":                   true,
					"restored_from_snapshot": restored,
				},
			}
		})
		if readErr == nil {
			if uid != payload.ownerID {
				c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
				return
			}
			c.JSON(http.StatusOK, payload.response)
			return
		}
		log.Printf("Build %s: live tasks read timed out after %s; falling back to snapshot", buildID, readableBuildStateTimeout)
		snapshot, err = h.getBuildSnapshot(uid, buildID)
		if err != nil {
			writeBuildLookupError(c, err, readErr)
			return
		}
	}
	if snapshot == nil {
		writeBuildLookupError(c, errBuildActionNotFound, errBuildActionNotFound)
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
	uid, ok := appmiddleware.RequireUserID(c)
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
		writeBuildLookupError(c, snapErr, err)
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
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	manifest, live, err := h.loadArtifactManifestForUser(uid, buildID)
	if err != nil {
		if buildPlatformIssueFromError(err) != nil {
			c.JSON(http.StatusServiceUnavailable, buildPlatformIssueResponse(err, "build artifacts unavailable", "The build artifacts could not be loaded because a platform service is temporarily unavailable."))
			return
		}
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
	uid, ok := appmiddleware.RequireUserID(c)
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
		if buildPlatformIssueFromError(err) != nil {
			c.JSON(http.StatusServiceUnavailable, buildPlatformIssueResponse(err, "build artifacts unavailable", "The build artifacts could not be loaded because a platform service is temporarily unavailable."))
			return
		}
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
		c.JSON(http.StatusServiceUnavailable, buildPlatformIssueResponse(fmt.Errorf("database unavailable"), "database unavailable", "Project persistence is temporarily unavailable because the primary database is offline."))
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
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	_, restoredSession, err := h.getBuildActionSession(uid, buildID, false)
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
		c.JSON(http.StatusServiceUnavailable, buildPlatformIssueResponse(fmt.Errorf("build history not available"), "build history not available", "Build history is temporarily unavailable because the primary database is offline."))
		return
	}

	uid, ok := appmiddleware.RequireUserID(c)
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

	if err := retryBuildHistoryRead("list_builds_count", func() error {
		return h.db.Model(&models.CompletedBuild{}).Where("user_id = ?", uid).Count(&total).Error
	}); err != nil {
		c.JSON(http.StatusServiceUnavailable, buildPlatformIssueResponse(err, "build history not available", "Build history is temporarily unavailable because the primary database is offline."))
		return
	}
	if err := retryBuildHistoryRead("list_builds_page", func() error {
		return h.db.Where("user_id = ?", uid).Order("updated_at DESC").Offset(offset).Limit(limit).Find(&builds).Error
	}); err != nil {
		c.JSON(http.StatusServiceUnavailable, buildPlatformIssueResponse(err, "build history not available", "Build history is temporarily unavailable because the primary database is offline."))
		return
	}

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
		displayStatus := presentedSnapshotStatus(&b)
		s := BuildSummary{
			ID:          b.ID,
			BuildID:     b.BuildID,
			ProjectID:   b.ProjectID,
			ProjectName: b.ProjectName,
			Description: b.Description,
			Status:      string(displayStatus),
			Mode:        b.Mode,
			PowerMode:   b.PowerMode,
			TechStack:   techStack,
			FilesCount:  b.FilesCount,
			TotalCost:   b.TotalCost,
			Progress:    presentedSnapshotProgress(&b, displayStatus),
			DurationMs:  b.DurationMs,
			CreatedAt:   b.CreatedAt.Format("2006-01-02T15:04:05Z"),
			Live:        false,
			Resumable:   isActiveBuildStatus(string(displayStatus)),
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
		c.JSON(http.StatusServiceUnavailable, buildPlatformIssueResponse(fmt.Errorf("build history not available"), "build history not available", "Completed build details are temporarily unavailable because the primary database is offline."))
		return
	}

	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}
	userPlan := h.currentSubscriptionType(c, uid)
	buildID := c.Param("buildId")

	var build models.CompletedBuild
	if err := retryBuildHistoryRead("get_completed_build", func() error {
		return h.db.Where("build_id = ? AND user_id = ?", buildID, uid).
			Order("updated_at DESC").
			Order("id DESC").
			First(&build).Error
	}); err != nil {
		if buildPlatformIssueFromError(err) != nil {
			c.JSON(http.StatusServiceUnavailable, buildPlatformIssueResponse(err, "build history not available", "Completed build details are temporarily unavailable because the primary database is offline."))
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}

	if refreshedSnapshot, liveBuild := h.refreshCompletedBuildFromLive(buildID, uid, &build); refreshedSnapshot != nil {
		build = *refreshedSnapshot
		if liveBuild != nil && h.shouldPreferLiveCompletedBuildResponse(&build, liveBuild) {
			payload, readErr := h.readLiveCompletedBuildPayload(liveBuild, &build, userPlan)
			if readErr == nil {
				c.JSON(http.StatusOK, payload.response)
				return
			}
		}
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
	displayStatus := presentedSnapshotStatus(&build)
	displayProgress := presentedSnapshotProgress(&build, displayStatus)

	response := gin.H{
		"id":                build.ID,
		"build_id":          build.BuildID,
		"project_id":        build.ProjectID,
		"project_name":      build.ProjectName,
		"description":       build.Description,
		"status":            string(displayStatus),
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
		"progress":          displayProgress,
		"duration_ms":       build.DurationMs,
		"error":             build.Error,
		"created_at":        build.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"completed_at":      build.CompletedAt,
		"live":              live,
		"resumable":         isActiveBuildStatus(string(displayStatus)),
	}
	for key, value := range buildSnapshotStateResponseFields(snapshotState, string(displayStatus), userPlan) {
		response[key] = value
	}
	c.JSON(http.StatusOK, response)
}

func (h *BuildHandler) refreshCompletedBuildFromLive(buildID string, userID uint, snapshot *models.CompletedBuild) (*models.CompletedBuild, *Build) {
	if h == nil || h.manager == nil || snapshot == nil {
		return snapshot, nil
	}

	liveBuild, err := h.manager.GetBuild(buildID)
	if err != nil || liveBuild == nil || liveBuild.UserID != userID {
		return snapshot, nil
	}
	if !h.shouldPreferLiveCompletedBuildResponse(snapshot, liveBuild) {
		return snapshot, liveBuild
	}

	if syncErr := h.manager.persistBuildSnapshotCritical(liveBuild, nil); syncErr != nil {
		log.Printf("GetCompletedBuild: failed to refresh stale snapshot for build %s: %v", buildID, syncErr)
		return snapshot, liveBuild
	}

	refreshed, snapErr := h.getBuildSnapshot(userID, buildID)
	if snapErr != nil || refreshed == nil {
		if snapErr != nil {
			log.Printf("GetCompletedBuild: failed to reload refreshed snapshot for build %s: %v", buildID, snapErr)
		}
		return snapshot, liveBuild
	}
	return refreshed, liveBuild
}

func (h *BuildHandler) shouldPreferLiveCompletedBuildResponse(snapshot *models.CompletedBuild, liveBuild *Build) bool {
	if snapshot == nil || liveBuild == nil {
		return false
	}

	liveBuild.mu.RLock()
	liveStatus := liveBuild.Status
	liveUpdatedAt := liveBuild.UpdatedAt
	liveError := strings.TrimSpace(liveBuild.Error)
	liveBuild.mu.RUnlock()

	if isActiveBuildStatus(string(liveStatus)) {
		return false
	}

	snapshotStatus := presentedSnapshotStatus(snapshot)
	if isActiveBuildStatus(string(snapshotStatus)) {
		return true
	}

	liveRepresentsSuccess := liveStatus == BuildCompleted && liveError == ""
	snapshotRepresentsSuccess := completedBuildSnapshotRepresentsSuccess(snapshot)
	if liveRepresentsSuccess && !snapshotRepresentsSuccess {
		return true
	}
	if snapshotRepresentsSuccess && !liveRepresentsSuccess {
		return false
	}

	return snapshot.UpdatedAt.Before(liveUpdatedAt)
}

func (h *BuildHandler) readLiveCompletedBuildPayload(build *Build, snapshot *models.CompletedBuild, userPlan string) (liveBuildReadPayload, error) {
	payload, err := h.readLiveBuildDetailsPayload(build, userPlan, false)
	if err != nil {
		return liveBuildReadPayload{}, err
	}

	response := payload.response
	files, _ := response["files"].([]GeneratedFile)

	var techStack any
	if snapshot != nil && strings.TrimSpace(snapshot.TechStack) != "" {
		_ = json.Unmarshal([]byte(snapshot.TechStack), &techStack)
	}
	if techStack == nil {
		build.mu.RLock()
		if build.TechStack != nil {
			techStack = build.TechStack
		}
		build.mu.RUnlock()
	}

	projectName := ""
	if snapshot != nil {
		projectName = strings.TrimSpace(snapshot.ProjectName)
	}

	var (
		projectID any
		duration  int64
	)
	build.mu.RLock()
	if build.ProjectID != nil {
		projectID = *build.ProjectID
	}
	if projectName == "" && build.Plan != nil {
		projectName = build.Plan.AppType
	}
	if build.CompletedAt != nil {
		duration = build.CompletedAt.Sub(build.CreatedAt).Milliseconds()
	}
	build.mu.RUnlock()

	if snapshot != nil && snapshot.DurationMs > 0 {
		duration = snapshot.DurationMs
	}

	response["id"] = build.ID
	if snapshot != nil && snapshot.ID != 0 {
		response["id"] = snapshot.ID
	}
	response["build_id"] = build.ID
	response["project_id"] = projectID
	response["project_name"] = projectName
	response["tech_stack"] = techStack
	response["files_count"] = len(files)
	response["total_cost"] = 0.0
	if snapshot != nil {
		response["total_cost"] = snapshot.TotalCost
	}
	response["duration_ms"] = duration
	response["live"] = true
	response["resumable"] = false

	return payload, nil
}

// DownloadCompletedBuild streams a completed build as a ZIP archive
// GET /api/v1/builds/:buildId/download
func (h *BuildHandler) DownloadCompletedBuild(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, buildPlatformIssueResponse(fmt.Errorf("build history not available"), "build history not available", "Build download is temporarily unavailable because the primary database is offline."))
		return
	}

	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}
	buildID := c.Param("buildId")

	var build models.CompletedBuild
	if err := retryBuildHistoryRead("download_completed_build", func() error {
		return h.db.Where("build_id = ? AND user_id = ?", buildID, uid).
			Order("updated_at DESC").
			Order("id DESC").
			First(&build).Error
	}); err != nil {
		if buildPlatformIssueFromError(err) != nil {
			c.JSON(http.StatusServiceUnavailable, buildPlatformIssueResponse(err, "build history not available", "Build download is temporarily unavailable because the primary database is offline."))
			return
		}
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

	buildStatus := presentedSnapshotStatus(&build)
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

// DeleteBuild removes a saved terminal build from build history.
// DELETE /api/v1/builds/:buildId
func (h *BuildHandler) DeleteBuild(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, buildPlatformIssueResponse(fmt.Errorf("build history not available"), "build history not available", "Build history is temporarily unavailable because the primary database is offline."))
		return
	}

	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}
	buildID := strings.TrimSpace(c.Param("buildId"))
	if buildID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "build id is required"})
		return
	}

	snapshot, err := h.getBuildSnapshot(uid, buildID)
	if err != nil {
		if buildPlatformIssueFromError(err) != nil {
			c.JSON(http.StatusServiceUnavailable, buildPlatformIssueResponse(err, "build history not available", "Build history is temporarily unavailable because the primary database is offline."))
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}

	// If the build is live and active, cancel it automatically so the user
	// doesn't need a separate cancel step before deleting.
	if liveBuild, liveErr := h.manager.GetBuild(buildID); liveErr == nil && liveBuild != nil &&
		liveBuild.Status != BuildCompleted && liveBuild.Status != BuildFailed && liveBuild.Status != BuildCancelled {
		_ = h.manager.CancelBuild(buildID)
	}

	// If the snapshot still shows an active status, guard against deleting a running build.
	displayStatus := presentedSnapshotStatus(snapshot)
	if isActiveBuildStatus(string(displayStatus)) {
		liveBuild, liveErr := h.manager.GetBuild(buildID)
		if liveErr != nil {
			// No live build in this instance but snapshot shows active — reject.
			// The build may be running on another instance or the cancel hasn't
			// propagated yet. Caller should cancel first and then retry.
			log.Printf("DeleteBuild: build %s snapshot shows active status %q but no live build found; rejecting deletion", buildID, displayStatus)
			c.JSON(http.StatusConflict, gin.H{"error": "build is still active; cancel the build before deleting"})
			return
		}
		// Live build is in memory. If it's still active (cancel hasn't finished), reject.
		liveBuild.mu.RLock()
		liveStatus := liveBuild.Status
		liveBuild.mu.RUnlock()
		if liveStatus != BuildCompleted && liveStatus != BuildFailed && liveStatus != BuildCancelled {
			c.JSON(http.StatusConflict, gin.H{"error": "build is still active; cancel the build before deleting"})
			return
		}
		// Live build is now terminal (was cancelled above) — proceed with deletion.
	}

	if err := h.db.Unscoped().Where("build_id = ? AND user_id = ?", buildID, uid).Delete(&models.CompletedBuild{}).Error; err != nil {
		if buildPlatformIssueFromError(err) != nil {
			c.JSON(http.StatusServiceUnavailable, buildPlatformIssueResponse(err, "build history not available", "Build history is temporarily unavailable because the primary database is offline."))
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove build"})
		return
	}

	if h.manager != nil && h.manager.editStore != nil {
		h.manager.editStore.Clear(buildID)
	}
	if h.hub != nil {
		h.hub.CloseAllConnections(buildID)
	}
	if h.manager != nil {
		h.manager.ForgetBuild(buildID)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "deleted",
		"build_id": buildID,
	})
}

func (h *BuildHandler) getBuildSnapshot(userID uint, buildID string) (*models.CompletedBuild, error) {
	if h.db == nil {
		return nil, fmt.Errorf("build history not available")
	}

	var snapshot models.CompletedBuild
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		lastErr = h.db.Where("build_id = ? AND user_id = ?", buildID, userID).
			Order("updated_at DESC").
			Order("id DESC").
			First(&snapshot).Error
		if lastErr == nil {
			return &snapshot, nil
		}
		if !errors.Is(lastErr, gorm.ErrRecordNotFound) {
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
				continue
			}
			return nil, lastErr
		}
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
		}
	}

	// Fallback to a build-id lookup in case the row was persisted but the user-scoped read
	// briefly missed during a process boundary. Ownership is still enforced below.
	if err := h.db.Where("build_id = ?", buildID).
		Order("updated_at DESC").
		Order("id DESC").
		First(&snapshot).Error; err != nil {
		var unscopedSnapshot models.CompletedBuild
		if unscopedErr := h.db.Unscoped().
			Where("build_id = ? AND user_id = ?", buildID, userID).
			Order("updated_at DESC").
			Order("id DESC").
			First(&unscopedSnapshot).Error; unscopedErr == nil {
			return &unscopedSnapshot, nil
		}
		return nil, err
	}
	if snapshot.UserID != userID {
		var unscopedSnapshot models.CompletedBuild
		if err := h.db.Unscoped().
			Where("build_id = ? AND user_id = ?", buildID, userID).
			Order("updated_at DESC").
			Order("id DESC").
			First(&unscopedSnapshot).Error; err == nil {
			return &unscopedSnapshot, nil
		}
		return nil, gorm.ErrRecordNotFound
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
	uid, ok := appmiddleware.RequireUserID(c)
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
	uid, ok := appmiddleware.RequireUserID(c)
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
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID, true)
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
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID, true)
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
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID, true)
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
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID, true)
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

func patchBundleReviewReason(c *gin.Context) string {
	if c == nil || c.Request == nil || c.Request.Body == nil || c.Request.ContentLength == 0 {
		return ""
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		return ""
	}
	return strings.TrimSpace(req.Reason)
}

// ApprovePatchBundle approves a review-required patch bundle and applies it if needed.
// POST /api/v1/build/:id/patch-bundles/:bundleId/approve
func (h *BuildHandler) ApprovePatchBundle(c *gin.Context) {
	buildID := c.Param("id")
	bundleID := c.Param("bundleId")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID, true)
	if err != nil {
		writeBuildActionSessionError(c, err)
		return
	}

	result, err := h.manager.approvePatchBundle(build, bundleID, patchBundleReviewReason(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"build_id":         buildID,
		"bundle_id":        bundleID,
		"status":           string(result.Status),
		"review_status":    string(PatchBundleReviewApproved),
		"applied":          result.Applied,
		"patch_bundle":     result.Bundle,
		"message":          "Patch bundle approved",
		"live":             true,
		"restored_session": restoredSession,
	})
}

// RejectPatchBundle rejects a review-required patch bundle.
// POST /api/v1/build/:id/patch-bundles/:bundleId/reject
func (h *BuildHandler) RejectPatchBundle(c *gin.Context) {
	buildID := c.Param("id")
	bundleID := c.Param("bundleId")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID, true)
	if err != nil {
		writeBuildActionSessionError(c, err)
		return
	}

	result, err := h.manager.rejectPatchBundle(build, bundleID, patchBundleReviewReason(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"build_id":         buildID,
		"bundle_id":        bundleID,
		"status":           string(result.Status),
		"review_status":    string(PatchBundleReviewRejected),
		"applied":          false,
		"patch_bundle":     result.Bundle,
		"message":          "Patch bundle rejected",
		"live":             true,
		"restored_session": restoredSession,
	})
}

// ApprovePromptImprovementProposal records human approval for a prompt proposal.
// POST /api/v1/build/:id/prompt-proposals/:proposalId/approve
func (h *BuildHandler) ApprovePromptImprovementProposal(c *gin.Context) {
	buildID := c.Param("id")
	proposalID := c.Param("proposalId")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID, false)
	if err != nil {
		writeBuildActionSessionError(c, err)
		return
	}

	result, err := h.manager.approvePromptImprovementProposal(build, proposalID, patchBundleReviewReason(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"build_id":              buildID,
		"proposal_id":           proposalID,
		"status":                string(result.Status),
		"review_status":         string(PromptProposalReviewApproved),
		"prompt_proposal":       result.Proposal,
		"historical_learning":   result.HistoricalLearning,
		"message":               "Prompt proposal approved for benchmark-gated adoption",
		"prompt_mutated":        false,
		"restored_session":      restoredSession,
		"benchmark_gate_status": "pending",
	})
}

// RejectPromptImprovementProposal records human rejection for a prompt proposal.
// POST /api/v1/build/:id/prompt-proposals/:proposalId/reject
func (h *BuildHandler) RejectPromptImprovementProposal(c *gin.Context) {
	buildID := c.Param("id")
	proposalID := c.Param("proposalId")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID, false)
	if err != nil {
		writeBuildActionSessionError(c, err)
		return
	}

	result, err := h.manager.rejectPromptImprovementProposal(build, proposalID, patchBundleReviewReason(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"build_id":              buildID,
		"proposal_id":           proposalID,
		"status":                string(result.Status),
		"review_status":         string(PromptProposalReviewRejected),
		"prompt_proposal":       result.Proposal,
		"historical_learning":   result.HistoricalLearning,
		"message":               "Prompt proposal rejected",
		"prompt_mutated":        false,
		"restored_session":      restoredSession,
		"benchmark_gate_status": "not_applicable",
	})
}

// BenchmarkPromptImprovementProposal records benchmark-gate results for an approved prompt proposal.
// POST /api/v1/build/:id/prompt-proposals/:proposalId/benchmark
func (h *BuildHandler) BenchmarkPromptImprovementProposal(c *gin.Context) {
	buildID := c.Param("id")
	proposalID := c.Param("proposalId")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID, false)
	if err != nil {
		writeBuildActionSessionError(c, err)
		return
	}

	result, err := h.manager.benchmarkPromptImprovementProposal(build, proposalID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"build_id":              buildID,
		"proposal_id":           proposalID,
		"status":                string(result.Status),
		"benchmark_status":      string(result.Proposal.BenchmarkStatus),
		"prompt_proposal":       result.Proposal,
		"historical_learning":   result.HistoricalLearning,
		"message":               "Prompt proposal benchmark gate recorded",
		"prompt_mutated":        false,
		"restored_session":      restoredSession,
		"benchmark_gate_status": string(result.Proposal.BenchmarkStatus),
	})
}

// CreatePromptPackDraft creates an inactive prompt-pack draft from ready adoption candidates.
// POST /api/v1/build/:id/prompt-pack-drafts
func (h *BuildHandler) CreatePromptPackDraft(c *gin.Context) {
	buildID := c.Param("id")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID, false)
	if err != nil {
		writeBuildActionSessionError(c, err)
		return
	}

	result, err := h.manager.createPromptPackDraft(build)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"build_id":            buildID,
		"status":              string(result.Status),
		"prompt_pack_draft":   result.Draft,
		"historical_learning": result.HistoricalLearning,
		"message":             "Prompt pack draft created",
		"prompt_mutated":      false,
		"activation_ready":    false,
		"restored_session":    restoredSession,
	})
}

// RequestPromptPackDraftActivation records an admin-only activation request for a draft.
// POST /api/v1/build/:id/prompt-pack-drafts/:draftId/request-activation
func (h *BuildHandler) RequestPromptPackDraftActivation(c *gin.Context) {
	if !promptPackActivationRequestsEnabled() {
		c.JSON(http.StatusForbidden, gin.H{
			"error":          "prompt pack activation requests are disabled",
			"feature_flag":   promptPackActivationFeatureFlag,
			"prompt_mutated": false,
		})
		return
	}
	if !requestContextIsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":          "admin access required",
			"prompt_mutated": false,
		})
		return
	}

	buildID := c.Param("id")
	draftID := c.Param("draftId")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID, false)
	if err != nil {
		writeBuildActionSessionError(c, err)
		return
	}

	result, err := h.manager.requestPromptPackDraftActivation(build, draftID, uid, patchBundleReviewReason(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"build_id":                         buildID,
		"draft_id":                         draftID,
		"status":                           string(result.Status),
		"activation_status":                string(result.Request.Status),
		"prompt_pack_draft":                result.Draft,
		"prompt_pack_activation_request":   result.Request,
		"message":                          "Prompt pack activation request recorded for admin review",
		"prompt_mutated":                   false,
		"restored_session":                 restoredSession,
		"feature_flag":                     promptPackActivationFeatureFlag,
		"live_prompt_generation_changed":   false,
		"historical_learning_mutated":      false,
		"requires_separate_activation_job": true,
	})
}

// ActivatePromptPackRequest materializes a pending activation request into the global registry.
// POST /api/v1/build/:id/prompt-pack-activation-requests/:requestId/activate
func (h *BuildHandler) ActivatePromptPackRequest(c *gin.Context) {
	if !promptPackActivationRequestsEnabled() {
		c.JSON(http.StatusForbidden, gin.H{
			"error":          "prompt pack activation is disabled",
			"feature_flag":   promptPackActivationFeatureFlag,
			"prompt_mutated": false,
		})
		return
	}
	if !requestContextIsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":          "admin access required",
			"prompt_mutated": false,
		})
		return
	}

	buildID := c.Param("id")
	requestID := c.Param("requestId")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID, false)
	if err != nil {
		writeBuildActionSessionError(c, err)
		return
	}

	result, err := h.manager.activatePromptPackRequest(build, requestID, uid, patchBundleReviewReason(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"build_id":                         buildID,
		"request_id":                       requestID,
		"status":                           string(result.Status),
		"activation_status":                string(result.Request.Status),
		"prompt_pack_activation_request":   result.Request,
		"prompt_pack_version":              result.Version,
		"prompt_pack_activation_event":     result.Event,
		"message":                          "Prompt pack version activated in registry",
		"prompt_mutated":                   false,
		"restored_session":                 restoredSession,
		"feature_flag":                     promptPackActivationFeatureFlag,
		"live_prompt_generation_changed":   false,
		"live_prompt_read_enabled":         false,
		"requires_separate_live_rollout":   true,
		"requires_separate_activation_job": false,
	})
}

// RollbackPromptPackVersion creates a new registry entry that rolls back an active version.
// POST /api/v1/build/:id/prompt-pack-versions/:versionId/rollback
func (h *BuildHandler) RollbackPromptPackVersion(c *gin.Context) {
	if !promptPackActivationRequestsEnabled() {
		c.JSON(http.StatusForbidden, gin.H{
			"error":          "prompt pack activation requests are disabled",
			"feature_flag":   promptPackActivationFeatureFlag,
			"prompt_mutated": false,
		})
		return
	}
	if !requestContextIsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":          "admin access required",
			"prompt_mutated": false,
		})
		return
	}

	buildID := c.Param("id")
	versionID := c.Param("versionId")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	build, restoredSession, err := h.getBuildActionSession(uid, buildID, false)
	if err != nil {
		writeBuildActionSessionError(c, err)
		return
	}

	result, err := h.manager.rollbackPromptPackVersion(build, versionID, uid, patchBundleReviewReason(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"build_id":                         buildID,
		"version_id":                       versionID,
		"status":                           string(result.Status),
		"prompt_pack_version":              result.RollbackVersion,
		"rolled_back_version":              result.RolledBackVersion,
		"prompt_pack_activation_event":     result.Event,
		"message":                          "Prompt pack version rolled back in registry",
		"prompt_mutated":                   false,
		"restored_session":                 restoredSession,
		"feature_flag":                     promptPackActivationFeatureFlag,
		"live_prompt_generation_changed":   false,
		"live_prompt_read_enabled":         false,
		"requires_separate_live_rollout":   true,
		"requires_separate_activation_job": false,
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
		build.POST("/:id/provider-model", h.SetProviderModelOverride)
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
		build.POST("/:id/patch-bundles/:bundleId/approve", h.ApprovePatchBundle)
		build.POST("/:id/patch-bundles/:bundleId/reject", h.RejectPatchBundle)
		build.POST("/:id/prompt-proposals/:proposalId/approve", h.ApprovePromptImprovementProposal)
		build.POST("/:id/prompt-proposals/:proposalId/reject", h.RejectPromptImprovementProposal)
		build.POST("/:id/prompt-proposals/:proposalId/benchmark", h.BenchmarkPromptImprovementProposal)
		build.POST("/:id/prompt-pack-drafts", h.CreatePromptPackDraft)
		build.POST("/:id/prompt-pack-drafts/:draftId/request-activation", h.RequestPromptPackDraftActivation)
		build.POST("/:id/prompt-pack-activation-requests/:requestId/activate", h.ActivatePromptPackRequest)
		build.POST("/:id/prompt-pack-versions/:versionId/rollback", h.RollbackPromptPackVersion)
	}

	// Build history endpoints
	rg.GET("/builds", h.ListBuilds)
	rg.GET("/builds/:buildId", h.GetCompletedBuild)
	rg.GET("/builds/:buildId/download", h.DownloadCompletedBuild)
	rg.DELETE("/builds/:buildId", h.DeleteBuild)
}
