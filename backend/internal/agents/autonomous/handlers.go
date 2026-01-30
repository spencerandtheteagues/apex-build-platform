// Package autonomous - HTTP API Handlers
// RESTful endpoints for autonomous agent management
package autonomous

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Handler handles autonomous agent HTTP requests
type Handler struct {
	agent *AutonomousAgent
}

// NewHandler creates a new autonomous agent handler
func NewHandler(agent *AutonomousAgent) *Handler {
	return &Handler{agent: agent}
}

// StartTaskRequest is the request body for starting a new task
type StartTaskRequest struct {
	Description string `json:"description" binding:"required"`
	ProjectID   *uint  `json:"project_id,omitempty"`
}

// StartTask starts a new autonomous task
// POST /api/v1/agent/start
func (h *Handler) StartTask(c *gin.Context) {
	log.Println("Handler: StartTask called")

	// Get user ID from auth context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid := userID.(uint)

	// Parse request
	var req StartTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"details": err.Error(),
		})
		return
	}

	// Validate description length
	if len(req.Description) < 10 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "description too short",
			"details": "Please provide a more detailed description of what you want to build",
		})
		return
	}

	// Start the task
	task, err := h.agent.StartTask(uid, req.ProjectID, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to start task",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"task_id":       task.ID,
		"status":        task.State,
		"websocket_url": "/ws/agent/" + task.ID,
		"message":       "Autonomous task started. Connect to WebSocket for real-time updates.",
	})
}

// GetTaskStatus returns the current status of a task
// GET /api/v1/agent/:id/status
func (h *Handler) GetTaskStatus(c *gin.Context) {
	taskID := c.Param("id")

	// Get user ID for authorization
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	task, err := h.agent.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "task not found",
			"details": err.Error(),
		})
		return
	}

	// Verify ownership
	if task.UserID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	status, err := h.agent.GetTaskStatus(taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to get status",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, status)
}

// GetTaskDetails returns full details of a task
// GET /api/v1/agent/:id
func (h *Handler) GetTaskDetails(c *gin.Context) {
	taskID := c.Param("id")

	// Get user ID for authorization
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	task, err := h.agent.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "task not found",
			"details": err.Error(),
		})
		return
	}

	// Verify ownership
	if task.UserID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	task.mu.RLock()
	defer task.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"id":           task.ID,
		"user_id":      task.UserID,
		"project_id":   task.ProjectID,
		"description":  task.Description,
		"state":        task.State,
		"progress":     task.Progress,
		"plan":         task.Plan,
		"actions":      task.Actions,
		"artifacts":    task.Artifacts,
		"error":        task.Error,
		"retry_count":  task.RetryCount,
		"max_retries":  task.MaxRetries,
		"created_at":   task.CreatedAt,
		"started_at":   task.StartedAt,
		"completed_at": task.CompletedAt,
	})
}

// StopTask stops a running task
// POST /api/v1/agent/:id/stop
func (h *Handler) StopTask(c *gin.Context) {
	taskID := c.Param("id")

	// Get user ID for authorization
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	task, err := h.agent.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "task not found",
			"details": err.Error(),
		})
		return
	}

	// Verify ownership
	if task.UserID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	if err := h.agent.StopTask(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "failed to stop task",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "stopped",
		"message": "Task has been stopped",
	})
}

// PauseTask pauses a running task
// POST /api/v1/agent/:id/pause
func (h *Handler) PauseTask(c *gin.Context) {
	taskID := c.Param("id")

	// Get user ID for authorization
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	task, err := h.agent.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "task not found",
			"details": err.Error(),
		})
		return
	}

	// Verify ownership
	if task.UserID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	if err := h.agent.PauseTask(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "failed to pause task",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "paused",
		"message": "Task has been paused. Use /resume to continue.",
	})
}

// ResumeTask resumes a paused task
// POST /api/v1/agent/:id/resume
func (h *Handler) ResumeTask(c *gin.Context) {
	taskID := c.Param("id")

	// Get user ID for authorization
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	task, err := h.agent.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "task not found",
			"details": err.Error(),
		})
		return
	}

	// Verify ownership
	if task.UserID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	if err := h.agent.ResumeTask(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "failed to resume task",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "resumed",
		"message": "Task has been resumed",
	})
}

// GetTaskLogs returns the execution logs for a task
// GET /api/v1/agent/:id/logs
func (h *Handler) GetTaskLogs(c *gin.Context) {
	taskID := c.Param("id")

	// Get user ID for authorization
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	task, err := h.agent.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "task not found",
			"details": err.Error(),
		})
		return
	}

	// Verify ownership
	if task.UserID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Parse limit parameter
	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	logs, err := h.agent.GetTaskLogs(taskID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to get logs",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id": taskID,
		"logs":    logs,
		"count":   len(logs),
	})
}

// GetTaskPlan returns the execution plan for a task
// GET /api/v1/agent/:id/plan
func (h *Handler) GetTaskPlan(c *gin.Context) {
	taskID := c.Param("id")

	// Get user ID for authorization
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	task, err := h.agent.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "task not found",
			"details": err.Error(),
		})
		return
	}

	// Verify ownership
	if task.UserID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	task.mu.RLock()
	plan := task.Plan
	task.mu.RUnlock()

	if plan == nil {
		c.JSON(http.StatusOK, gin.H{
			"task_id": taskID,
			"plan":    nil,
			"message": "Plan not yet created",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id": taskID,
		"plan":    plan,
	})
}

// GetTaskArtifacts returns the artifacts generated by a task
// GET /api/v1/agent/:id/artifacts
func (h *Handler) GetTaskArtifacts(c *gin.Context) {
	taskID := c.Param("id")

	// Get user ID for authorization
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	task, err := h.agent.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "task not found",
			"details": err.Error(),
		})
		return
	}

	// Verify ownership
	if task.UserID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	task.mu.RLock()
	artifacts := task.Artifacts
	task.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"task_id":   taskID,
		"artifacts": artifacts,
		"count":     len(artifacts),
	})
}

// WebSocket upgrader for real-time updates
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

// HandleWebSocket handles WebSocket connections for real-time task updates
// GET /ws/agent/:id
func (h *Handler) HandleWebSocket(c *gin.Context) {
	taskID := c.Param("id")

	log.Printf("WebSocket connection request for task %s", taskID)

	// Verify task exists
	task, err := h.agent.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	// Get user ID from query param (WebSocket can't use headers easily)
	// In production, implement proper JWT validation
	var uid uint
	if userID, exists := c.Get("user_id"); exists {
		uid = userID.(uint)
	} else if tokenStr := c.Query("token"); tokenStr != "" {
		// Validate token and extract user ID
		// This would use the same JWT validation as auth middleware
		// For now, we'll skip validation in development
		log.Printf("WebSocket: Token provided but validation skipped in development")
	}

	// Verify ownership (if we have a user ID)
	if uid != 0 && task.UserID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Upgrade to WebSocket
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("WebSocket connection established for task %s", taskID)

	// Create update channel
	updateChan := make(chan *WSUpdate, 100)
	h.agent.Subscribe(taskID, updateChan)
	defer h.agent.Unsubscribe(taskID, updateChan)

	// Send current task state first
	task.mu.RLock()
	initialState := &WSUpdate{
		Type:      "task_state",
		TaskID:    taskID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"state":       task.State,
			"progress":    task.Progress,
			"description": task.Description,
			"plan":        task.Plan,
			"logs_count":  len(task.Logs),
		},
	}
	task.mu.RUnlock()

	if data, err := json.Marshal(initialState); err == nil {
		conn.WriteMessage(websocket.TextMessage, data)
	}

	// Send connection confirmation
	confirmMsg := map[string]interface{}{
		"type":      "connection:established",
		"task_id":   taskID,
		"timestamp": time.Now(),
		"message":   "Connected to autonomous agent stream",
	}
	if data, err := json.Marshal(confirmMsg); err == nil {
		conn.WriteMessage(websocket.TextMessage, data)
	}

	// Handle incoming messages and outgoing updates
	done := make(chan struct{})

	// Read messages from client (for commands like pause/resume)
	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket error: %v", err)
				}
				return
			}

			// Handle client commands
			var cmd struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(message, &cmd); err == nil {
				switch cmd.Type {
				case "pause":
					h.agent.PauseTask(taskID)
				case "resume":
					h.agent.ResumeTask(taskID)
				case "stop":
					h.agent.StopTask(taskID)
				case "ping":
					// Respond with pong
					pong := map[string]interface{}{
						"type":      "pong",
						"timestamp": time.Now(),
					}
					if data, err := json.Marshal(pong); err == nil {
						conn.WriteMessage(websocket.TextMessage, data)
					}
				}
			}
		}
	}()

	// Send updates to client
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return

		case update := <-updateChan:
			data, err := json.Marshal(update)
			if err != nil {
				log.Printf("Failed to marshal update: %v", err)
				continue
			}

			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("WebSocket write failed: %v", err)
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// RegisterRoutes registers all autonomous agent routes
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	agent := rg.Group("/agent")
	{
		agent.POST("/start", h.StartTask)
		agent.GET("/:id", h.GetTaskDetails)
		agent.GET("/:id/status", h.GetTaskStatus)
		agent.POST("/:id/stop", h.StopTask)
		agent.POST("/:id/pause", h.PauseTask)
		agent.POST("/:id/resume", h.ResumeTask)
		agent.GET("/:id/logs", h.GetTaskLogs)
		agent.GET("/:id/plan", h.GetTaskPlan)
		agent.GET("/:id/artifacts", h.GetTaskArtifacts)
	}
}

// RegisterWebSocketRoute registers the WebSocket route separately
// This is needed because WebSocket routes often need to bypass auth middleware
func (h *Handler) RegisterWebSocketRoute(router *gin.Engine) {
	router.GET("/ws/agent/:id", h.HandleWebSocket)
}
