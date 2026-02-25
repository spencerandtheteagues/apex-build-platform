package handlers

import (
	"net/http"
	"strconv"

	"apex-build/internal/budget"

	"github.com/gin-gonic/gin"
)

// BudgetHandler serves the budget cap management API.
type BudgetHandler struct {
	enforcer *budget.BudgetEnforcer
}

// NewBudgetHandler creates a new BudgetHandler.
func NewBudgetHandler(enforcer *budget.BudgetEnforcer) *BudgetHandler {
	return &BudgetHandler{enforcer: enforcer}
}

// GetCaps returns all active budget caps for the authenticated user.
// GET /budget/caps
func (h *BudgetHandler) GetCaps(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	caps, err := h.enforcer.GetCaps(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"caps": caps})
}

// setCapRequest is the JSON body for POST /budget/caps
type setCapRequest struct {
	CapType   string  `json:"cap_type" binding:"required"`
	LimitUSD  float64 `json:"limit_usd" binding:"required,gt=0"`
	Action    string  `json:"action"`
	ProjectID *uint   `json:"project_id,omitempty"`
}

// SetCap creates or updates a budget cap.
// POST /budget/caps
func (h *BudgetHandler) SetCap(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req setCapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cap, err := h.enforcer.SetCap(userID, req.CapType, req.ProjectID, req.LimitUSD, req.Action)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"cap": cap})
}

// DeleteCap soft-deletes a budget cap by ID.
// DELETE /budget/caps/:id
func (h *BudgetHandler) DeleteCap(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	idStr := c.Param("id")
	capID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cap id"})
		return
	}

	if err := h.enforcer.DeleteCap(uint(capID), userID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// PreAuthorize checks whether a build can proceed given the user's budget.
// GET /budget/preauthorize?build_id=xxx&estimated_cost=0.15
func (h *BudgetHandler) PreAuthorize(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	buildID := c.Query("build_id")
	estimatedCostStr := c.DefaultQuery("estimated_cost", "0")
	estimatedCost, err := strconv.ParseFloat(estimatedCostStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid estimated_cost"})
		return
	}

	result, err := h.enforcer.PreAuthorize(userID, buildID, estimatedCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	status := http.StatusOK
	if !result.Allowed {
		status = http.StatusPaymentRequired
	}
	c.JSON(status, result)
}

// KillAll is a placeholder for the panic button that stops all running builds.
// The actual kill logic will be wired in manager.go.
// POST /budget/kill-all
func (h *BudgetHandler) KillAll(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Placeholder: actual kill logic will be wired through the build manager
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "kill-all signal sent",
		"user_id": userID,
	})
}

// RegisterRoutes registers all budget endpoints under the given router group.
func (h *BudgetHandler) RegisterRoutes(rg *gin.RouterGroup) {
	bg := rg.Group("/budget")
	{
		bg.GET("/caps", h.GetCaps)
		bg.POST("/caps", h.SetCap)
		bg.DELETE("/caps/:id", h.DeleteCap)
		bg.GET("/preauthorize", h.PreAuthorize)
		bg.POST("/kill-all", h.KillAll)
	}
}
