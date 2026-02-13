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
	uid := userID.(uint)
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
	log.Printf("StartBuild: description=%s, mode=%s", truncate(req.Description, 50), req.Mode)

	// Validate description
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

		c.JSON(http.StatusOK, gin.H{
			"id":           build.ID,
			"status":       string(build.Status),
			"mode":         string(build.Mode),
			"description":  build.Description,
			"progress":     build.Progress,
			"agents_count": len(build.Agents),
			"tasks_count":  len(build.Tasks),
			"checkpoints":  len(build.Checkpoints),
			"created_at":   build.CreatedAt,
			"updated_at":   build.UpdatedAt,
			"completed_at": build.CompletedAt,
			"error":        build.Error,
			"live":         true,
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

	c.JSON(http.StatusOK, gin.H{
		"id":                     snapshot.BuildID,
		"status":                 snapshot.Status,
		"mode":                   snapshot.Mode,
		"description":            snapshot.Description,
		"progress":               snapshot.Progress,
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
			"id":           build.ID,
			"user_id":      build.UserID,
			"project_id":   build.ProjectID,
			"status":       string(build.Status),
			"mode":         string(build.Mode),
			"description":  build.Description,
			"plan":         build.Plan,
			"agents":       agents,
			"tasks":        build.Tasks,
			"checkpoints":  build.Checkpoints,
			"progress":     build.Progress,
			"created_at":   build.CreatedAt,
			"updated_at":   build.UpdatedAt,
			"completed_at": build.CompletedAt,
			"error":        build.Error,
			"files":        h.manager.collectGeneratedFiles(build),
			"live":         true,
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

		// Collect all generated files from tasks
		files := make([]GeneratedFile, 0)
		for _, task := range build.Tasks {
			if task.Output != nil {
				files = append(files, task.Output.Files...)
			}
		}

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
	if err := h.db.Where("build_id = ? AND user_id = ?", buildID, userID).First(&snapshot).Error; err != nil {
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

func isActiveBuildStatus(status string) bool {
	switch status {
	case string(BuildPending), string(BuildPlanning), string(BuildInProgress), string(BuildTesting), string(BuildReviewing):
		return true
	default:
		return false
	}
}

// RegisterRoutes registers all build routes on the router
func (h *BuildHandler) RegisterRoutes(rg *gin.RouterGroup) {
	build := rg.Group("/build")
	{
		build.POST("/start", h.StartBuild)
		build.GET("/:id", h.GetBuildDetails)
		build.GET("/:id/status", h.GetBuildStatus)
		build.POST("/:id/message", h.SendMessage)
		build.GET("/:id/checkpoints", h.GetCheckpoints)
		build.POST("/:id/rollback/:checkpointId", h.RollbackCheckpoint)
		build.GET("/:id/agents", h.GetAgents)
		build.GET("/:id/tasks", h.GetTasks)
		build.GET("/:id/files", h.GetGeneratedFiles)
		build.POST("/:id/cancel", h.CancelBuild)
	}

	// Build history endpoints
	rg.GET("/builds", h.ListBuilds)
	rg.GET("/builds/:buildId", h.GetCompletedBuild)
	rg.GET("/builds/:buildId/download", h.DownloadCompletedBuild)
}
