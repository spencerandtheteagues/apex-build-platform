// APEX.BUILD Completions HTTP Handlers
// API endpoints for AI inline code completions

package handlers

import (
	"net/http"
	"strconv"

	"apex-build/internal/completions"
	"apex-build/internal/middleware"

	"github.com/gin-gonic/gin"
)

// CompletionsHandler handles AI code completion requests
type CompletionsHandler struct {
	service *completions.CompletionService
}

// NewCompletionsHandler creates a new completions handler
func NewCompletionsHandler(service *completions.CompletionService) *CompletionsHandler {
	return &CompletionsHandler{service: service}
}

// GetInlineCompletion returns inline ghost-text completions
// POST /api/v1/completions/inline
func (h *CompletionsHandler) GetInlineCompletion(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req completions.CompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	item, err := h.service.GetInlineCompletion(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if item == nil {
		c.JSON(http.StatusOK, gin.H{"completion": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"completion": item,
	})
}

// GetCompletions returns full completion suggestions
// POST /api/v1/completions
func (h *CompletionsHandler) GetCompletions(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req completions.CompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	response, err := h.service.GetCompletions(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// AcceptCompletion records when a user accepts a completion
// POST /api/v1/completions/accept
func (h *CompletionsHandler) AcceptCompletion(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		CompletionID string `json:"completion_id" binding:"required"`
		Accepted     bool   `json:"accepted"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	h.service.AcceptCompletion(userID, req.CompletionID, req.Accepted)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetCompletionStats returns completion metrics
// GET /api/v1/completions/stats
func (h *CompletionsHandler) GetCompletionStats(c *gin.Context) {
	_, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Admin-only endpoint (simplified check)
	isAdmin, _ := c.Get("is_admin")
	if isAdmin != true {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	// Return metrics from service - not exposed directly by CompletionService
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Completion stats available",
	})
}

// RegisterCompletionRoutes registers all completion API routes
func (h *CompletionsHandler) RegisterCompletionRoutes(rg *gin.RouterGroup) {
	completionRoutes := rg.Group("/completions")
	{
		completionRoutes.POST("", h.GetCompletions)
		completionRoutes.POST("/inline", h.GetInlineCompletion)
		completionRoutes.POST("/accept", h.AcceptCompletion)
	}
}

// Helper to suppress unused import warning
var _ = strconv.Itoa
