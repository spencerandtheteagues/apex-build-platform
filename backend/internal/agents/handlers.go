// Package agents - HTTP API Handlers
// RESTful endpoints for build management
package agents

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BuildHandler handles build-related HTTP requests
type BuildHandler struct {
	manager *AgentManager
	hub     *WSHub
}

// NewBuildHandler creates a new build handler
func NewBuildHandler(manager *AgentManager, hub *WSHub) *BuildHandler {
	return &BuildHandler{
		manager: manager,
		hub:     hub,
	}
}

// StartBuild creates and starts a new build
// POST /api/v1/build/start
func (h *BuildHandler) StartBuild(c *gin.Context) {
	// Get user ID from auth context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid := userID.(uint)

	// Parse request
	var req BuildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"details": err.Error(),
		})
		return
	}

	// Validate description
	if len(req.Description) < 10 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "description too short",
			"details": "Please provide a more detailed description of the app you want to build",
		})
		return
	}

	// Create the build
	build, err := h.manager.CreateBuild(uid, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to create build",
			"details": err.Error(),
		})
		return
	}

	// Start the build process
	if err := h.manager.StartBuild(build.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to start build",
			"details": err.Error(),
		})
		return
	}

	// Return build info with WebSocket URL
	c.JSON(http.StatusCreated, BuildResponse{
		BuildID:      build.ID,
		WebSocketURL: "/ws/build/" + build.ID,
		Status:       string(build.Status),
	})
}

// GetBuildStatus returns the current status of a build
// GET /api/v1/build/:id/status
func (h *BuildHandler) GetBuildStatus(c *gin.Context) {
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
	})
}

// GetBuildDetails returns full details of a build including agents and tasks
// GET /api/v1/build/:id
func (h *BuildHandler) GetBuildDetails(c *gin.Context) {
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

	// Convert agents map to slice for JSON
	agents := make([]*Agent, 0, len(build.Agents))
	for _, agent := range build.Agents {
		agents = append(agents, agent)
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          build.ID,
		"user_id":     build.UserID,
		"project_id":  build.ProjectID,
		"status":      string(build.Status),
		"mode":        string(build.Mode),
		"description": build.Description,
		"plan":        build.Plan,
		"agents":      agents,
		"tasks":       build.Tasks,
		"checkpoints": build.Checkpoints,
		"progress":    build.Progress,
		"created_at":  build.CreatedAt,
		"updated_at":  build.UpdatedAt,
		"completed_at": build.CompletedAt,
		"error":       build.Error,
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
			"error": "build cannot be cancelled",
			"details": "Build is already " + string(build.Status),
		})
		return
	}

	build.Status = BuildCancelled
	build.mu.Unlock()

	// Close all WebSocket connections
	h.hub.CloseAllConnections(buildID)

	c.JSON(http.StatusOK, gin.H{
		"status":  "cancelled",
		"message": "Build has been cancelled",
	})
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
}
