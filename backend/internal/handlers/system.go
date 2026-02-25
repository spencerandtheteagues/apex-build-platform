// APEX.BUILD System Handlers
// System information, health checks, and admin endpoints

package handlers

import (
	"net/http"
	"runtime"
	"time"

	"apex-build/internal/middleware"

	"github.com/gin-gonic/gin"
)

// GetSystemInfo returns system information and statistics
func (h *Handler) GetSystemInfo(c *gin.Context) {
	_, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	// Get system stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Get AI provider usage
	providerUsage := h.AIRouter.GetProviderUsage()

	// Get WebSocket statistics
	wsStats := map[string]interface{}{
		"active_connections": h.WSHub.GetClientCount(),
		"active_rooms":       len(h.WSHub.GetActiveRooms()),
	}

	// Get database statistics
	dbStats := h.getDatabaseStats()

	systemInfo := map[string]interface{}{
		"service": map[string]interface{}{
			"name":    "APEX.BUILD",
			"version": "1.0.0",
			"uptime":  time.Since(startTime).String(),
		},
		"runtime": map[string]interface{}{
			"go_version":      runtime.Version(),
			"goroutines":      runtime.NumGoroutine(),
			"cpu_count":       runtime.NumCPU(),
			"memory_alloc":    memStats.Alloc,
			"memory_total":    memStats.TotalAlloc,
			"memory_sys":      memStats.Sys,
			"gc_runs":         memStats.NumGC,
		},
		"ai": map[string]interface{}{
			"provider_usage":     providerUsage,
			"health_status":      h.AIRouter.GetHealthStatus(),
		},
		"websocket": wsStats,
		"database":  dbStats,
		"timestamp": time.Now().UTC(),
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data:    systemInfo,
	})
}

// ExecuteCode handles code execution requests
func (h *Handler) ExecuteCode(c *gin.Context) {
	_, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	var req struct {
		ProjectID uint   `json:"project_id" binding:"required"`
		Language  string `json:"language" binding:"required"`
		Code      string `json:"code" binding:"required"`
		Input     string `json:"input"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	c.JSON(http.StatusNotImplemented, StandardResponse{
		Success: false,
		Error:   "Code execution is not yet available",
		Code:    "NOT_IMPLEMENTED",
	})
}

// GetExecution returns execution details
func (h *Handler) GetExecution(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, StandardResponse{
		Success: false,
		Error:   "Execution tracking not yet implemented",
		Code:    "NOT_IMPLEMENTED",
	})
}

// GetExecutionHistory returns user's execution history
func (h *Handler) GetExecutionHistory(c *gin.Context) {
	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data:    []interface{}{},
		Message: "Execution history feature coming soon",
	})
}

// StopExecution stops a running execution
func (h *Handler) StopExecution(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, StandardResponse{
		Success: false,
		Error:   "Execution control not yet implemented",
		Code:    "NOT_IMPLEMENTED",
	})
}

// JoinCollabRoom joins a collaboration room
func (h *Handler) JoinCollabRoom(c *gin.Context) {
	_, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectIDStr := c.Param("project_id")

	// Get active users in the room
	roomID := "project_" + projectIDStr
	users := h.WSHub.GetRoomUsers(roomID)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"room_id": roomID,
			"project_id": projectIDStr,
			"users": users,
			"user_count": len(users),
		},
		Message: "Ready to join collaboration room",
	})
}

// LeaveCollabRoom leaves a collaboration room
func (h *Handler) LeaveCollabRoom(c *gin.Context) {
	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Left collaboration room",
	})
}

// GetCollabUsers returns users in a collaboration room
func (h *Handler) GetCollabUsers(c *gin.Context) {
	roomID := c.Param("room_id")
	users := h.WSHub.GetRoomUsers(roomID)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"room_id": roomID,
			"users": users,
			"user_count": len(users),
		},
	})
}

// HandleWebSocket upgrades HTTP connections to WebSocket
func (h *Handler) HandleWebSocket(c *gin.Context) {
	h.WSHub.HandleWebSocket(c)
}

// getDatabaseStats returns database statistics
func (h *Handler) getDatabaseStats() map[string]interface{} {
	sqlDB, err := h.DB.DB()
	if err != nil {
		return map[string]interface{}{
			"error": "Failed to get database stats",
		}
	}

	dbStats := sqlDB.Stats()

	return map[string]interface{}{
		"open_connections":     dbStats.OpenConnections,
		"in_use_connections":   dbStats.InUse,
		"idle_connections":     dbStats.Idle,
		"max_open_connections": dbStats.MaxOpenConnections,
		"wait_count":          dbStats.WaitCount,
		"wait_duration":       dbStats.WaitDuration.String(),
		"max_idle_closed":     dbStats.MaxIdleClosed,
		"max_lifetime_closed": dbStats.MaxLifetimeClosed,
	}
}

// Server start time for uptime calculation
var startTime = time.Now()