// Package handlers - Deployment HTTP Handlers for APEX.BUILD
package handlers

import (
	"net/http"
	"strconv"

	"apex-build/internal/deploy"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DeployHandler handles deployment-related endpoints
type DeployHandler struct {
	db      *gorm.DB
	service *deploy.DeploymentService
}

// NewDeployHandler creates a new deploy handler
func NewDeployHandler(db *gorm.DB, service *deploy.DeploymentService) *DeployHandler {
	return &DeployHandler{
		db:      db,
		service: service,
	}
}

// StartDeployment initiates a new deployment
// POST /api/v1/deploy
func (h *DeployHandler) StartDeployment(c *gin.Context) {
	userID := c.GetUint("userID")

	var req struct {
		ProjectID    uint              `json:"project_id" binding:"required"`
		Provider     string            `json:"provider" binding:"required"`
		Environment  string            `json:"environment"`
		Branch       string            `json:"branch"`
		EnvVars      map[string]string `json:"env_vars"`
		BuildCommand string            `json:"build_command"`
		OutputDir    string            `json:"output_dir"`
		InstallCmd   string            `json:"install_cmd"`
		Framework    string            `json:"framework"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify project ownership
	var project models.Project
	if err := h.db.First(&project, req.ProjectID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Set defaults
	if req.Environment == "" {
		req.Environment = "production"
	}
	if req.Branch == "" {
		req.Branch = "main"
	}

	// Create deployment config
	config := &deploy.DeploymentConfig{
		ProjectID:    req.ProjectID,
		Provider:     deploy.DeploymentProvider(req.Provider),
		Environment:  req.Environment,
		Branch:       req.Branch,
		EnvVars:      req.EnvVars,
		BuildCommand: req.BuildCommand,
		OutputDir:    req.OutputDir,
		InstallCmd:   req.InstallCmd,
		Framework:    req.Framework,
	}

	// Start deployment
	result, err := h.service.StartDeployment(c.Request.Context(), userID, config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"success":    true,
		"deployment": result,
		"message":    "Deployment started",
	})
}

// GetDeployment returns deployment details
// GET /api/v1/deploy/:id
func (h *DeployHandler) GetDeployment(c *gin.Context) {
	deploymentID := c.Param("id")

	deployment, err := h.service.GetDeploymentStatus(deploymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"deployment": deployment,
	})
}

// GetDeploymentStatus returns just the status of a deployment
// GET /api/v1/deploy/:id/status
func (h *DeployHandler) GetDeploymentStatus(c *gin.Context) {
	deploymentID := c.Param("id")

	deployment, err := h.service.GetDeploymentStatus(deploymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  deployment.Status,
		"url":     deployment.URL,
	})
}

// GetDeploymentLogs returns logs for a deployment
// GET /api/v1/deploy/:id/logs
func (h *DeployHandler) GetDeploymentLogs(c *gin.Context) {
	deploymentID := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	logs, err := h.service.GetDeploymentLogs(deploymentID, limit)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"logs":    logs,
	})
}

// CancelDeployment cancels an in-progress deployment
// DELETE /api/v1/deploy/:id
func (h *DeployHandler) CancelDeployment(c *gin.Context) {
	userID := c.GetUint("userID")
	deploymentID := c.Param("id")

	// Verify ownership
	deployment, err := h.service.GetDeploymentStatus(deploymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	if deployment.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.service.CancelDeployment(deploymentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Deployment cancelled",
	})
}

// Redeploy redeployes a previous deployment
// POST /api/v1/deploy/:id/redeploy
func (h *DeployHandler) Redeploy(c *gin.Context) {
	userID := c.GetUint("userID")
	deploymentID := c.Param("id")

	// Get original deployment
	original, err := h.service.GetDeploymentStatus(deploymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	if original.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Create new deployment with same config
	result, err := h.service.Redeploy(c.Request.Context(), deploymentID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"success":    true,
		"deployment": result,
		"message":    "Redeployment started",
	})
}

// GetProjectDeployments returns deployment history for a project
// GET /api/v1/deploy/projects/:projectId/history
func (h *DeployHandler) GetProjectDeployments(c *gin.Context) {
	userID := c.GetUint("userID")
	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	// Verify project ownership
	var project models.Project
	if err := h.db.First(&project, uint(projectID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get pagination params
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))

	deployments, total, err := h.service.GetDeploymentHistory(uint(projectID), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"deployments": deployments,
		"total":       total,
		"limit":       limit,
		"page":        page,
	})
}

// GetProviders returns available deployment providers
// GET /api/v1/deploy/providers
func (h *DeployHandler) GetProviders(c *gin.Context) {
	// Get configured providers from service
	providers := h.service.GetAvailableProviders()

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"providers": providers,
	})
}

// GetLatestDeployment returns the latest deployment for a project
// GET /api/v1/deploy/projects/:projectId/latest
func (h *DeployHandler) GetLatestDeployment(c *gin.Context) {
	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	// Get the most recent deployment
	deployments, _, err := h.service.GetDeploymentHistory(uint(projectID), 1, 1)
	if err != nil || len(deployments) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No deployments found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"deployment": deployments[0],
	})
}
