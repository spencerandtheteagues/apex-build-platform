// APEX.BUILD Debugging API Handlers
// REST API and WebSocket endpoints for debugging

package handlers

import (
	"encoding/json"
	"net/http"

	"apex-build/internal/debugging"
	"apex-build/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

// DebuggingHandler handles debugging API requests
type DebuggingHandler struct {
	db           *gorm.DB
	debugService *debugging.DebugService
	upgrader     websocket.Upgrader
}

// NewDebuggingHandler creates a new debugging handler
func NewDebuggingHandler(db *gorm.DB, debugService *debugging.DebugService) *DebuggingHandler {
	return &DebuggingHandler{
		db:           db,
		debugService: debugService,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// StartSessionRequest represents the request to start a debug session
type StartSessionRequest struct {
	ProjectID  uint   `json:"project_id" binding:"required"`
	FileID     uint   `json:"file_id"`
	EntryPoint string `json:"entry_point" binding:"required"`
	Language   string `json:"language" binding:"required"`
}

// StartSession starts a new debugging session
// POST /api/v1/debug/sessions
func (h *DebuggingHandler) StartSession(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req StartSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Verify user owns the project
	var count int64
	h.db.Table("projects").Where("id = ? AND owner_id = ?", req.ProjectID, userID).Count(&count)
	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this project"})
		return
	}

	session, err := h.debugService.StartSession(
		c.Request.Context(),
		userID,
		req.ProjectID,
		req.FileID,
		req.Language,
		req.EntryPoint,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":      "Debug session started",
		"session":      session,
		"websocket_url": "/ws/debug/" + session.ID,
	})
}

// GetSession returns a debug session by ID
// GET /api/v1/debug/sessions/:id
func (h *DebuggingHandler) GetSession(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID := c.Param("id")

	session, err := h.debugService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get breakpoints
	breakpoints, _ := h.debugService.GetSessionBreakpoints(sessionID)

	c.JSON(http.StatusOK, gin.H{
		"session":     session,
		"breakpoints": breakpoints,
	})
}

// StopSession stops a debugging session
// POST /api/v1/debug/sessions/:id/stop
func (h *DebuggingHandler) StopSession(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID := c.Param("id")

	session, err := h.debugService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.debugService.StopSession(sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Debug session stopped",
	})
}

// SetBreakpointRequest represents the request to set a breakpoint
type SetBreakpointRequest struct {
	FileID    uint                    `json:"file_id" binding:"required"`
	FilePath  string                  `json:"file_path" binding:"required"`
	Line      int                     `json:"line" binding:"required"`
	Column    int                     `json:"column"`
	Type      debugging.BreakpointType `json:"type"`
	Condition string                  `json:"condition"`
}

// SetBreakpoint sets a breakpoint in the session
// POST /api/v1/debug/sessions/:id/breakpoints
func (h *DebuggingHandler) SetBreakpoint(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID := c.Param("id")

	session, err := h.debugService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	var req SetBreakpointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	bpType := req.Type
	if bpType == "" {
		bpType = debugging.BreakpointLine
	}

	breakpoint, err := h.debugService.SetBreakpoint(
		sessionID,
		req.FileID,
		req.FilePath,
		req.Line,
		bpType,
		req.Condition,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Breakpoint set",
		"breakpoint": breakpoint,
	})
}

// RemoveBreakpoint removes a breakpoint
// DELETE /api/v1/debug/sessions/:id/breakpoints/:bpId
func (h *DebuggingHandler) RemoveBreakpoint(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID := c.Param("id")
	breakpointID := c.Param("bpId")

	session, err := h.debugService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.debugService.RemoveBreakpoint(sessionID, breakpointID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Breakpoint removed",
	})
}

// ToggleBreakpoint enables/disables a breakpoint
// PATCH /api/v1/debug/sessions/:id/breakpoints/:bpId
func (h *DebuggingHandler) ToggleBreakpoint(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID := c.Param("id")
	breakpointID := c.Param("bpId")

	session, err := h.debugService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := h.debugService.ToggleBreakpoint(sessionID, breakpointID, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Breakpoint toggled",
		"enabled": req.Enabled,
	})
}

// Continue resumes execution
// POST /api/v1/debug/sessions/:id/continue
func (h *DebuggingHandler) Continue(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID := c.Param("id")

	session, err := h.debugService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.debugService.Continue(sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Execution resumed",
	})
}

// StepOver executes the next line
// POST /api/v1/debug/sessions/:id/step-over
func (h *DebuggingHandler) StepOver(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID := c.Param("id")

	session, err := h.debugService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.debugService.StepOver(sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Stepped over",
	})
}

// StepInto steps into a function call
// POST /api/v1/debug/sessions/:id/step-into
func (h *DebuggingHandler) StepInto(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID := c.Param("id")

	session, err := h.debugService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.debugService.StepInto(sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Stepped into",
	})
}

// StepOut steps out of the current function
// POST /api/v1/debug/sessions/:id/step-out
func (h *DebuggingHandler) StepOut(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID := c.Param("id")

	session, err := h.debugService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.debugService.StepOut(sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Stepped out",
	})
}

// Pause pauses execution
// POST /api/v1/debug/sessions/:id/pause
func (h *DebuggingHandler) Pause(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID := c.Param("id")

	session, err := h.debugService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.debugService.Pause(sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Execution paused",
	})
}

// GetCallStack returns the current call stack
// GET /api/v1/debug/sessions/:id/stack
func (h *DebuggingHandler) GetCallStack(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID := c.Param("id")

	session, err := h.debugService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	stack, err := h.debugService.GetCallStack(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"call_stack": stack,
	})
}

// GetVariables returns variables for a scope
// GET /api/v1/debug/sessions/:id/variables/:objectId
func (h *DebuggingHandler) GetVariables(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID := c.Param("id")
	objectID := c.Param("objectId")

	session, err := h.debugService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	variables, err := h.debugService.GetVariables(sessionID, objectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"variables": variables,
	})
}

// EvaluateExpression evaluates an expression
// POST /api/v1/debug/sessions/:id/evaluate
func (h *DebuggingHandler) EvaluateExpression(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID := c.Param("id")

	session, err := h.debugService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	var req struct {
		Expression string `json:"expression" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	result, err := h.debugService.EvaluateExpression(sessionID, req.Expression)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": result,
	})
}

// AddWatch adds a watch expression
// POST /api/v1/debug/sessions/:id/watches
func (h *DebuggingHandler) AddWatch(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID := c.Param("id")

	session, err := h.debugService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	var req struct {
		Expression string `json:"expression" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	watch, err := h.debugService.AddWatch(sessionID, req.Expression)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"watch": watch,
	})
}

// GetWatches returns all watch expressions
// GET /api/v1/debug/sessions/:id/watches
func (h *DebuggingHandler) GetWatches(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID := c.Param("id")

	session, err := h.debugService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	watches, err := h.debugService.GetWatches(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"watches": watches,
	})
}

// RemoveWatch removes a watch expression
// DELETE /api/v1/debug/sessions/:id/watches/:watchId
func (h *DebuggingHandler) RemoveWatch(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sessionID := c.Param("id")
	watchID := c.Param("watchId")

	session, err := h.debugService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.debugService.RemoveWatch(sessionID, watchID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Watch removed",
	})
}

// HandleDebugWebSocket handles WebSocket connections for debug events
// GET /ws/debug/:sessionId
func (h *DebuggingHandler) HandleDebugWebSocket(c *gin.Context) {
	sessionID := c.Param("sessionId")

	// Get event channel
	eventCh, err := h.debugService.GetEventChannel(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Upgrade to WebSocket
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Send events to client
	for event := range eventCh {
		data, err := json.Marshal(event)
		if err != nil {
			continue
		}

		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			break
		}
	}
}

// RegisterRoutes registers all debugging routes
func (h *DebuggingHandler) RegisterRoutes(rg *gin.RouterGroup) {
	debug := rg.Group("/debug")
	{
		// Session management
		debug.POST("/sessions", h.StartSession)
		debug.GET("/sessions/:id", h.GetSession)
		debug.POST("/sessions/:id/stop", h.StopSession)

		// Breakpoints
		debug.POST("/sessions/:id/breakpoints", h.SetBreakpoint)
		debug.DELETE("/sessions/:id/breakpoints/:bpId", h.RemoveBreakpoint)
		debug.PATCH("/sessions/:id/breakpoints/:bpId", h.ToggleBreakpoint)

		// Execution control
		debug.POST("/sessions/:id/continue", h.Continue)
		debug.POST("/sessions/:id/pause", h.Pause)
		debug.POST("/sessions/:id/step-over", h.StepOver)
		debug.POST("/sessions/:id/step-into", h.StepInto)
		debug.POST("/sessions/:id/step-out", h.StepOut)

		// Inspection
		debug.GET("/sessions/:id/stack", h.GetCallStack)
		debug.GET("/sessions/:id/variables/:objectId", h.GetVariables)
		debug.POST("/sessions/:id/evaluate", h.EvaluateExpression)

		// Watch expressions
		debug.GET("/sessions/:id/watches", h.GetWatches)
		debug.POST("/sessions/:id/watches", h.AddWatch)
		debug.DELETE("/sessions/:id/watches/:watchId", h.RemoveWatch)
	}
}
