// Package agents - HTTP API Handlers
// RESTful endpoints for build management
package agents

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BuildHandler handles build-related HTTP requests
type BuildHandler struct {
	manager *AgentManager
	hub     *WSHub
	db      *gorm.DB
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
		Ready      bool     `json:"ready"`
		Providers  int      `json:"providers_available"`
		Names      []string `json:"provider_names"`
		ErrorCode  string   `json:"error_code,omitempty"`
		Error      string   `json:"error,omitempty"`
		Suggestion string   `json:"suggestion,omitempty"`
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
	for i, p := range providers {
		names[i] = string(p)
	}

	c.JSON(http.StatusOK, preflightResult{
		Ready:     true,
		Providers: len(providers),
		Names:     names,
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
		}
	}

	// Validate description (trim whitespace before checking)
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

	// Create the build
	build, err := h.manager.CreateBuild(uid, &req)
	if err != nil {
		log.Printf("StartBuild: failed to create build: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to create build",
			"details": err.Error(),
		})
		return
	}
	log.Printf("StartBuild: build created with ID %s", build.ID)

	// Start the build process asynchronously
	go func() {
		log.Printf("StartBuild: starting async build process for %s", build.ID)
		if err := h.manager.StartBuild(build.ID); err != nil {
			log.Printf("Error starting build %s: %v", build.ID, err)
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
	uid := userID.(uint)

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

		c.JSON(http.StatusOK, gin.H{
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
			"live":                  true,
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

	c.JSON(http.StatusOK, gin.H{
		"id":                     snapshot.BuildID,
		"status":                 snapshotStatus,
		"mode":                   snapshot.Mode,
		"description":            snapshot.Description,
		"progress":               snapshotProgress,
		"agents_count":           0,
		"tasks_count":            0,
		"checkpoints":            0,
		"created_at":             snapshot.CreatedAt,
		"updated_at":             snapshot.UpdatedAt,
		"completed_at":           snapshot.CompletedAt,
		"error":                  snapshot.Error,
		"files_count":            snapshot.FilesCount,
		"live":                   false,
		"restored_from_snapshot": true,
	})
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

// GetBuildDetails returns full details of a build including agents and tasks
// GET /api/v1/build/:id
func (h *BuildHandler) GetBuildDetails(c *gin.Context) {
	buildID := c.Param("id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid := userID.(uint)

	build, err := h.manager.GetBuild(buildID)
	if err == nil {
		// Verify ownership
		if uid != build.UserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		build.mu.RLock()
		defer build.mu.RUnlock()

		// Convert agents map to slice for JSON
		agents := make([]*Agent, 0, len(build.Agents))
		for _, agent := range build.Agents {
			agents = append(agents, agent)
		}

		c.JSON(http.StatusOK, gin.H{
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
			"live":                  true,
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
		"id":                     snapshot.BuildID,
		"user_id":                snapshot.UserID,
		"project_id":             snapshot.ProjectID,
		"status":                 snapshot.Status,
		"mode":                   snapshot.Mode,
		"description":            snapshot.Description,
		"plan":                   nil,
		"agents":                 []any{},
		"tasks":                  []any{},
		"checkpoints":            []any{},
		"progress":               snapshot.Progress,
		"created_at":             snapshot.CreatedAt,
		"updated_at":             snapshot.UpdatedAt,
		"completed_at":           snapshot.CompletedAt,
		"error":                  snapshot.Error,
		"files":                  files,
		"live":                   false,
		"restored_from_snapshot": true,
	})
}

// SendMessage sends a message to the build's lead agent
// POST /api/v1/build/:id/message
func (h *BuildHandler) SendMessage(c *gin.Context) {
	buildID := c.Param("id")

	// Verify build exists and ownership
	build, err := h.manager.GetBuild(buildID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "build not found",
			"details": err.Error(),
		})
		return
	}

	userID, _ := c.Get("user_id")
	if userID.(uint) != build.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Parse message
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"details": err.Error(),
		})
		return
	}

	// Send message to lead agent
	if err := h.manager.SendMessage(buildID, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to send message",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "sent",
		"message": "Message sent to lead agent",
	})
}

// GetCheckpoints returns all checkpoints for a build
// GET /api/v1/build/:id/checkpoints
func (h *BuildHandler) GetCheckpoints(c *gin.Context) {
	buildID := c.Param("id")

	build, err := h.manager.GetBuild(buildID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "build not found",
			"details": err.Error(),
		})
		return
	}

	// Verify ownership
	userID, _ := c.Get("user_id")
	if userID.(uint) != build.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	build.mu.RLock()
	defer build.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"build_id":    buildID,
		"checkpoints": build.Checkpoints,
		"count":       len(build.Checkpoints),
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
	uid := userID.(uint)

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

	build, err := h.manager.GetBuild(buildID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "build not found",
			"details": err.Error(),
		})
		return
	}

	// Verify ownership
	userID, _ := c.Get("user_id")
	if userID.(uint) != build.UserID {
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
	})
}

// GetTasks returns all tasks for a build
// GET /api/v1/build/:id/tasks
func (h *BuildHandler) GetTasks(c *gin.Context) {
	buildID := c.Param("id")

	build, err := h.manager.GetBuild(buildID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "build not found",
			"details": err.Error(),
		})
		return
	}

	// Verify ownership
	userID, _ := c.Get("user_id")
	if userID.(uint) != build.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	build.mu.RLock()
	defer build.mu.RUnlock()

	// Group tasks by status
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
	uid := userID.(uint)

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
	uid := userID.(uint)

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
	uid := userID.(uint)

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

	build, err := h.manager.GetBuild(buildID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "build not found",
			"details": err.Error(),
		})
		return
	}

	// Verify ownership
	userID, _ := c.Get("user_id")
	if userID.(uint) != build.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Check if build can be cancelled
	build.mu.Lock()
	if build.Status == BuildCompleted || build.Status == BuildFailed || build.Status == BuildCancelled {
		build.mu.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "build cannot be cancelled",
			"details": "Build is already " + string(build.Status),
		})
		return
	}

	build.Status = BuildCancelled
	build.UpdatedAt = time.Now()
	build.mu.Unlock()
	h.manager.persistBuildSnapshot(build, nil)

	// Close all WebSocket connections
	h.hub.CloseAllConnections(buildID)

	c.JSON(http.StatusOK, gin.H{
		"status":  "cancelled",
		"message": "Build has been cancelled",
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
	uid := userID.(uint)

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
	uid := userID.(uint)
	buildID := c.Param("buildId")

	var build models.CompletedBuild
	if err := h.db.Where("build_id = ? AND user_id = ?", buildID, uid).First(&build).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}

	// Parse stored files JSON
	files, _ := parseBuildFiles(build.FilesJSON)

	var techStack any
	if build.TechStack != "" {
		json.Unmarshal([]byte(build.TechStack), &techStack)
	}
	live := false
	if _, liveErr := h.manager.GetBuild(build.BuildID); liveErr == nil {
		live = true
	}

	c.JSON(http.StatusOK, gin.H{
		"id":           build.ID,
		"build_id":     build.BuildID,
		"project_id":   build.ProjectID,
		"project_name": build.ProjectName,
		"description":  build.Description,
		"status":       build.Status,
		"mode":         build.Mode,
		"power_mode":   build.PowerMode,
		"tech_stack":   techStack,
		"files":        files,
		"files_count":  build.FilesCount,
		"total_cost":   build.TotalCost,
		"progress":     build.Progress,
		"duration_ms":  build.DurationMs,
		"error":        build.Error,
		"created_at":   build.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"completed_at": build.CompletedAt,
		"live":         live,
		"resumable":    isActiveBuildStatus(build.Status),
	})
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
	uid := userID.(uint)
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
		path := strings.TrimPrefix(file.Path, "/")
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
	case string(BuildPending), string(BuildPlanning), string(BuildInProgress), string(BuildTesting), string(BuildReviewing):
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
	uid := userID.(uint)

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
	uid := userID.(uint)

	build, err := h.manager.GetBuild(buildID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}
	if uid != build.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	edits := h.manager.editStore.GetAllEdits(buildID)
	c.JSON(http.StatusOK, gin.H{
		"build_id": buildID,
		"edits":    edits,
		"count":    len(edits),
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
	uid := userID.(uint)

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
		"approved": len(approved),
		"message":  "Edits approved and applied",
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
	uid := userID.(uint)

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

	c.JSON(http.StatusOK, gin.H{"rejected": len(req.EditIDs)})
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
	uid := userID.(uint)

	build, err := h.manager.GetBuild(buildID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}
	if uid != build.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	approved := h.manager.editStore.ApproveAll(buildID)
	h.manager.applyApprovedEdits(build, approved)

	c.JSON(http.StatusOK, gin.H{
		"approved": len(approved),
		"message":  "All edits approved and applied",
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
	uid := userID.(uint)

	build, err := h.manager.GetBuild(buildID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}
	if uid != build.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	_ = h.manager.editStore.RejectAll(buildID)
	h.manager.resumeBuildAfterReview(build)

	c.JSON(http.StatusOK, gin.H{
		"message": "All edits rejected, build resuming",
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
