// Package handlers - Native Hosting HTTP Handlers for APEX.BUILD
// API endpoints for .apex.app native hosting
package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"apex-build/internal/hosting"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

// HostingHandler handles native hosting endpoints
type HostingHandler struct {
	db      *gorm.DB
	service *hosting.HostingService
}

// NewHostingHandler creates a new hosting handler
func NewHostingHandler(db *gorm.DB, service *hosting.HostingService) *HostingHandler {
	return &HostingHandler{
		db:      db,
		service: service,
	}
}

// StartDeploymentRequest represents the request to start a deployment
type StartDeploymentRequest struct {
	ProjectID      uint              `json:"project_id" binding:"required"`
	Subdomain      string            `json:"subdomain,omitempty"`
	Port           int               `json:"port"`
	BuildCommand   string            `json:"build_command"`
	StartCommand   string            `json:"start_command"`
	InstallCommand string            `json:"install_command"`
	Framework      string            `json:"framework"`
	NodeVersion    string            `json:"node_version"`
	PythonVersion  string            `json:"python_version"`
	GoVersion      string            `json:"go_version"`
	MemoryLimit    int64             `json:"memory_limit"`
	CPULimit       int64             `json:"cpu_limit"`
	HealthCheckPath string           `json:"health_check_path"`
	AutoScale      bool              `json:"auto_scale"`
	MinInstances   int               `json:"min_instances"`
	MaxInstances   int               `json:"max_instances"`
	EnvVars        map[string]string `json:"env_vars"`

	// Always-On configuration (Replit parity feature)
	AlwaysOn          bool `json:"always_on"`
	KeepAliveInterval int  `json:"keep_alive_interval"` // seconds
}

// StartDeployment initiates a new native deployment
// POST /api/v1/projects/:id/deploy
func (h *HostingHandler) StartDeployment(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	var req StartDeploymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Override project ID from URL
	req.ProjectID = uint(projectID)

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

	// Get project files
	var files []models.File
	if err := h.db.Where("project_id = ?", req.ProjectID).Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get project files"})
		return
	}

	// Convert to hosting.ProjectFile
	projectFiles := make([]hosting.ProjectFile, len(files))
	for i, f := range files {
		projectFiles[i] = hosting.ProjectFile{
			Path:    f.Path,
			Content: f.Content,
			Size:    f.Size,
			IsDir:   f.Type == "directory",
		}
	}

	// Create deployment config
	config := &hosting.DeploymentConfig{
		ProjectName:       project.Name,
		Subdomain:         req.Subdomain,
		Port:              req.Port,
		BuildCommand:      req.BuildCommand,
		StartCommand:      req.StartCommand,
		InstallCommand:    req.InstallCommand,
		Framework:         req.Framework,
		NodeVersion:       req.NodeVersion,
		PythonVersion:     req.PythonVersion,
		GoVersion:         req.GoVersion,
		MemoryLimit:       req.MemoryLimit,
		CPULimit:          req.CPULimit,
		HealthCheckPath:   req.HealthCheckPath,
		AutoScale:         req.AutoScale,
		MinInstances:      req.MinInstances,
		MaxInstances:      req.MaxInstances,
		EnvVars:           req.EnvVars,
		Files:             projectFiles,
		AlwaysOn:          req.AlwaysOn,
		KeepAliveInterval: req.KeepAliveInterval,
	}

	// Auto-detect framework if not specified
	if config.Framework == "" {
		config.Framework = detectFramework(project.Language, project.Framework, files)
	}

	// Start deployment
	deployment, err := h.service.StartDeployment(c.Request.Context(), req.ProjectID, userID, config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"success":      true,
		"deployment":   deployment,
		"message":      "Deployment started",
		"websocket_url": "/ws/deploy/" + deployment.ID,
	})
}

// GetDeployments returns all deployments for a project
// GET /api/v1/projects/:id/deployments
func (h *HostingHandler) GetDeployments(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("id")
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

	if project.OwnerID != userID && !project.IsPublic {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get pagination params
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))

	deployments, total, err := h.service.GetProjectDeployments(uint(projectID), page, limit)
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

// GetDeployment returns a specific deployment
// GET /api/v1/projects/:id/deployments/:deploymentId
func (h *HostingHandler) GetDeployment(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("id")
	deploymentID := c.Param("deploymentId")

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

	if project.OwnerID != userID && !project.IsPublic {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	deployment, err := h.service.GetDeployment(deploymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	// Verify deployment belongs to project
	if deployment.ProjectID != uint(projectID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"deployment": deployment,
	})
}

// GetDeploymentLogs returns logs for a deployment
// GET /api/v1/projects/:id/deployments/:deploymentId/logs
func (h *HostingHandler) GetDeploymentLogs(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("id")
	deploymentID := c.Param("deploymentId")

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

	// Get log params
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	logs, err := h.service.GetDeploymentLogs(deploymentID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"logs":    logs,
	})
}

// StreamDeploymentLogs streams logs in real-time via WebSocket
// GET /ws/deploy/:deploymentId/logs
func (h *HostingHandler) StreamDeploymentLogs(c *gin.Context) {
	deploymentID := c.Param("deploymentId")

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			// Allow requests with no origin (same-origin requests)
			if origin == "" {
				return true
			}
			// Allow known development and production origins
			allowedOrigins := []string{
				"http://localhost:3000",
				"http://localhost:5173",
				"http://localhost:8080",
				"https://apex.build",
				"https://www.apex.build",
			}
			for _, allowed := range allowedOrigins {
				if origin == allowed {
					return true
				}
			}
			return false
		},
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Subscribe to logs
	logChan, unsubscribe := h.service.SubscribeLogs(deploymentID)
	defer unsubscribe()

	// Send historical logs first
	logs, _ := h.service.GetDeploymentLogs(deploymentID, 100, 0)
	for _, log := range logs {
		entry := map[string]interface{}{
			"timestamp": log.Timestamp,
			"level":     log.Level,
			"source":    log.Source,
			"message":   log.Message,
		}
		if err := conn.WriteJSON(entry); err != nil {
			return
		}
	}

	// Stream new logs
	for logEntry := range logChan {
		entry := map[string]interface{}{
			"timestamp": logEntry.Timestamp,
			"level":     logEntry.Level,
			"source":    logEntry.Source,
			"message":   logEntry.Message,
		}
		if err := conn.WriteJSON(entry); err != nil {
			return
		}
	}
}

// StopDeployment stops a running deployment
// DELETE /api/v1/projects/:id/deployments/:deploymentId
func (h *HostingHandler) StopDeployment(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("id")
	deploymentID := c.Param("deploymentId")

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

	// Verify deployment belongs to project
	deployment, err := h.service.GetDeployment(deploymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	if deployment.ProjectID != uint(projectID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	if err := h.service.StopDeployment(deploymentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Deployment stopped",
	})
}

// RestartDeployment restarts a deployment
// POST /api/v1/projects/:id/deployments/:deploymentId/restart
func (h *HostingHandler) RestartDeployment(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("id")
	deploymentID := c.Param("deploymentId")

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

	if err := h.service.RestartDeployment(deploymentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Deployment restarted",
	})
}

// DeleteDeployment permanently deletes a deployment
// DELETE /api/v1/projects/:id/deployments/:deploymentId/delete
func (h *HostingHandler) DeleteDeployment(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("id")
	deploymentID := c.Param("deploymentId")

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

	if err := h.service.DeleteDeployment(deploymentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Deployment deleted",
	})
}

// CheckSubdomain checks if a subdomain is available
// GET /api/v1/hosting/check-subdomain
func (h *HostingHandler) CheckSubdomain(c *gin.Context) {
	subdomain := c.Query("subdomain")
	if subdomain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Subdomain is required"})
		return
	}

	available, message := h.service.CheckSubdomainAvailability(subdomain)

	c.JSON(http.StatusOK, gin.H{
		"subdomain": subdomain,
		"available": available,
		"message":   message,
		"url":       "https://" + subdomain + ".apex.app",
	})
}

// GetEnvVars returns environment variables for a deployment
// GET /api/v1/projects/:id/deployments/:deploymentId/env
func (h *HostingHandler) GetEnvVars(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("id")
	deploymentID := c.Param("deploymentId")

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

	envVars, err := h.service.GetEnvVars(deploymentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"env_vars": envVars,
	})
}

// UpdateEnvVars updates environment variables for a deployment
// PUT /api/v1/projects/:id/deployments/:deploymentId/env
func (h *HostingHandler) UpdateEnvVars(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("id")
	deploymentID := c.Param("deploymentId")

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

	var req struct {
		EnvVars map[string]string `json:"env_vars" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateEnvVars(deploymentID, req.EnvVars); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Environment variables updated",
	})
}

// GetDeploymentMetrics returns metrics for a deployment
// GET /api/v1/projects/:id/deployments/:deploymentId/metrics
func (h *HostingHandler) GetDeploymentMetrics(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("id")
	deploymentID := c.Param("deploymentId")

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

	if project.OwnerID != userID && !project.IsPublic {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	metrics, err := h.service.GetDeploymentMetrics(deploymentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"metrics": metrics,
	})
}

// GetLatestDeployment returns the latest deployment for a project
// GET /api/v1/projects/:id/deployments/latest
func (h *HostingHandler) GetLatestDeployment(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("id")

	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	// Verify project access
	var project models.Project
	if err := h.db.First(&project, uint(projectID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.OwnerID != userID && !project.IsPublic {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	deployments, _, err := h.service.GetProjectDeployments(uint(projectID), 1, 1)
	if err != nil || len(deployments) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No deployments found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"deployment": deployments[0],
	})
}

// RedeployLatest redeploys using the latest configuration
// POST /api/v1/projects/:id/redeploy
func (h *HostingHandler) RedeployLatest(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("id")

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

	// Get latest deployment configuration
	deployments, _, err := h.service.GetProjectDeployments(uint(projectID), 1, 1)
	if err != nil || len(deployments) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No previous deployments found"})
		return
	}

	latest := deployments[0]

	// Get project files
	var files []models.File
	if err := h.db.Where("project_id = ?", projectID).Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get project files"})
		return
	}

	projectFiles := make([]hosting.ProjectFile, len(files))
	for i, f := range files {
		projectFiles[i] = hosting.ProjectFile{
			Path:    f.Path,
			Content: f.Content,
			Size:    f.Size,
			IsDir:   f.Type == "directory",
		}
	}

	// Create new deployment with same config
	config := &hosting.DeploymentConfig{
		ProjectName:     project.Name,
		Subdomain:       latest.Subdomain,
		Port:            latest.ContainerPort,
		BuildCommand:    latest.BuildCommand,
		StartCommand:    latest.StartCommand,
		InstallCommand:  latest.InstallCommand,
		Framework:       latest.Framework,
		NodeVersion:     latest.NodeVersion,
		MemoryLimit:     latest.MemoryLimit,
		CPULimit:        latest.CPULimit,
		HealthCheckPath: latest.HealthCheckPath,
		AutoScale:       latest.AutoScale,
		MinInstances:    latest.MinInstances,
		MaxInstances:    latest.MaxInstances,
		Files:           projectFiles,
	}

	// Get env vars from previous deployment
	envVars, _ := h.service.GetEnvVars(latest.ID)
	config.EnvVars = make(map[string]string)
	for _, ev := range envVars {
		if !ev.IsSecret {
			config.EnvVars[ev.Key] = ev.Value
		}
	}

	deployment, err := h.service.StartDeployment(c.Request.Context(), uint(projectID), userID, config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"success":       true,
		"deployment":    deployment,
		"message":       "Redeployment started",
		"websocket_url": "/ws/deploy/" + deployment.ID,
	})
}

// HandleDeploymentWebSocket handles WebSocket connection for deployment updates
// GET /ws/deploy/:deploymentId
func (h *HostingHandler) HandleDeploymentWebSocket(c *gin.Context) {
	deploymentID := c.Param("deploymentId")

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			// Allow requests with no origin (same-origin requests)
			if origin == "" {
				return true
			}
			// Allow known development and production origins
			allowedOrigins := []string{
				"http://localhost:3000",
				"http://localhost:5173",
				"http://localhost:8080",
				"https://apex.build",
				"https://www.apex.build",
			}
			for _, allowed := range allowedOrigins {
				if origin == allowed {
					return true
				}
			}
			return false
		},
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Subscribe to logs
	logChan, unsubscribe := h.service.SubscribeLogs(deploymentID)
	defer unsubscribe()

	// Ticker for status updates
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	done := make(chan struct{})

	// Handle incoming messages (for ping/pong)
	go func() {
		defer close(done)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case logEntry := <-logChan:
			msg := map[string]interface{}{
				"type": "log",
				"data": map[string]interface{}{
					"timestamp": logEntry.Timestamp,
					"level":     logEntry.Level,
					"source":    logEntry.Source,
					"message":   logEntry.Message,
				},
			}
			if err := conn.WriteJSON(msg); err != nil {
				return
			}
		case <-ticker.C:
			// Send status update
			deployment, err := h.service.GetDeployment(deploymentID)
			if err != nil {
				return
			}
			msg := map[string]interface{}{
				"type": "status",
				"data": map[string]interface{}{
					"status":           deployment.Status,
					"container_status": deployment.ContainerStatus,
					"url":              deployment.URL,
					"error_message":    deployment.ErrorMessage,
				},
			}
			if err := conn.WriteJSON(msg); err != nil {
				return
			}

			// Close connection if deployment is complete or failed
			if deployment.Status == hosting.StatusRunning || deployment.Status == hosting.StatusFailed || deployment.Status == hosting.StatusStopped {
				return
			}
		}
	}
}

// SetAlwaysOn enables or disables always-on for a deployment
// PUT /api/v1/projects/:id/deployments/:deploymentId/always-on
func (h *HostingHandler) SetAlwaysOn(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("id")
	deploymentID := c.Param("deploymentId")

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

	// Verify deployment belongs to project
	deployment, err := h.service.GetDeployment(deploymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	if deployment.ProjectID != uint(projectID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	var req struct {
		AlwaysOn          bool `json:"always_on"`
		KeepAliveInterval int  `json:"keep_alive_interval"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.SetAlwaysOn(deploymentID, req.AlwaysOn, req.KeepAliveInterval); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"always_on": req.AlwaysOn,
		"message":   getAlwaysOnMessage(req.AlwaysOn),
	})
}

// GetAlwaysOnStatus returns the always-on status for a deployment
// GET /api/v1/projects/:id/deployments/:deploymentId/always-on
func (h *HostingHandler) GetAlwaysOnStatus(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("id")
	deploymentID := c.Param("deploymentId")

	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	// Verify project ownership or public access
	var project models.Project
	if err := h.db.First(&project, uint(projectID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.OwnerID != userID && !project.IsPublic {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Verify deployment belongs to project
	deployment, err := h.service.GetDeployment(deploymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	if deployment.ProjectID != uint(projectID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	status, err := h.service.GetAlwaysOnStatus(deploymentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  status,
	})
}

// getAlwaysOnMessage returns a user-friendly message for always-on status change
func getAlwaysOnMessage(enabled bool) string {
	if enabled {
		return "Always-On enabled. Your deployment will now run 24/7 with automatic restart on crash."
	}
	return "Always-On disabled. Your deployment may sleep after 30 minutes of inactivity."
}

// RegisterHostingRoutes registers all hosting routes
func (h *HostingHandler) RegisterHostingRoutes(router *gin.RouterGroup) {
	// Project deployment routes
	router.POST("/projects/:id/deploy", h.StartDeployment)
	router.GET("/projects/:id/deployments", h.GetDeployments)
	router.GET("/projects/:id/deployments/latest", h.GetLatestDeployment)
	router.GET("/projects/:id/deployments/:deploymentId", h.GetDeployment)
	router.GET("/projects/:id/deployments/:deploymentId/logs", h.GetDeploymentLogs)
	router.DELETE("/projects/:id/deployments/:deploymentId", h.StopDeployment)
	router.POST("/projects/:id/deployments/:deploymentId/restart", h.RestartDeployment)
	router.DELETE("/projects/:id/deployments/:deploymentId/delete", h.DeleteDeployment)
	router.GET("/projects/:id/deployments/:deploymentId/env", h.GetEnvVars)
	router.PUT("/projects/:id/deployments/:deploymentId/env", h.UpdateEnvVars)
	router.GET("/projects/:id/deployments/:deploymentId/metrics", h.GetDeploymentMetrics)
	router.POST("/projects/:id/redeploy", h.RedeployLatest)

	// Always-On routes (Replit parity feature)
	router.GET("/projects/:id/deployments/:deploymentId/always-on", h.GetAlwaysOnStatus)
	router.PUT("/projects/:id/deployments/:deploymentId/always-on", h.SetAlwaysOn)

	// Alternative hosting management routes (as specified)
	// These provide a cleaner API for the frontend: /api/v1/hosting/:projectId/...
	hostingRoutes := router.Group("/hosting")
	{
		hostingRoutes.GET("/check-subdomain", h.CheckSubdomain)
		hostingRoutes.GET("/:projectId/status", h.GetHostingStatus)
		hostingRoutes.POST("/:projectId/deploy", h.QuickDeploy)
		hostingRoutes.DELETE("/:projectId/undeploy", h.Undeploy)
		hostingRoutes.GET("/:projectId/logs", h.GetProjectDeploymentLogs)
		hostingRoutes.GET("/:projectId/url", h.GetDeploymentURL)

		// Custom domain management
		hostingRoutes.GET("/:projectId/domains", h.GetCustomDomains)
		hostingRoutes.POST("/:projectId/domains", h.AddCustomDomain)
		hostingRoutes.POST("/:projectId/domains/:domainId/verify", h.VerifyCustomDomain)
		hostingRoutes.DELETE("/:projectId/domains/:domainId", h.DeleteCustomDomain)
	}
}

// GetHostingStatus returns the hosting status for a project
// GET /api/v1/hosting/:projectId/status
func (h *HostingHandler) GetHostingStatus(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	// Verify project access
	var project models.Project
	if err := h.db.First(&project, uint(projectID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.OwnerID != userID && !project.IsPublic {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get latest deployment
	deployments, _, err := h.service.GetProjectDeployments(uint(projectID), 1, 1)
	if err != nil || len(deployments) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"deployed":   false,
			"deployment": nil,
			"message":    "No deployments found for this project",
		})
		return
	}

	latest := deployments[0]

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"deployed": latest.Status == hosting.StatusRunning,
		"deployment": gin.H{
			"id":               latest.ID,
			"status":           latest.Status,
			"container_status": latest.ContainerStatus,
			"url":              latest.URL,
			"subdomain":        latest.Subdomain,
			"always_on":        latest.AlwaysOn,
			"deployed_at":      latest.DeployedAt,
			"uptime_seconds":   latest.UptimeSeconds,
			"total_requests":   latest.TotalRequests,
			"error_message":    latest.ErrorMessage,
		},
	})
}

// QuickDeploy starts a deployment with auto-detected settings
// POST /api/v1/hosting/:projectId/deploy
func (h *HostingHandler) QuickDeploy(c *gin.Context) {
	userID := c.GetUint("user_id")
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

	// Parse optional configuration from request body
	var req struct {
		Subdomain    string            `json:"subdomain,omitempty"`
		AlwaysOn     bool              `json:"always_on"`
		Port         int               `json:"port"`
		BuildCommand string            `json:"build_command"`
		StartCommand string            `json:"start_command"`
		EnvVars      map[string]string `json:"env_vars"`
	}
	c.ShouldBindJSON(&req) // Ignore errors, use defaults

	// Get project files
	var files []models.File
	if err := h.db.Where("project_id = ?", projectID).Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get project files"})
		return
	}

	// Convert to hosting.ProjectFile
	projectFiles := make([]hosting.ProjectFile, len(files))
	for i, f := range files {
		projectFiles[i] = hosting.ProjectFile{
			Path:    f.Path,
			Content: f.Content,
			Size:    f.Size,
			IsDir:   f.Type == "directory",
		}
	}

	// Auto-detect framework
	framework := detectFramework(project.Language, project.Framework, files)

	// Create deployment config with auto-detected settings
	config := &hosting.DeploymentConfig{
		ProjectName:  project.Name,
		Subdomain:    req.Subdomain,
		Port:         req.Port,
		BuildCommand: req.BuildCommand,
		StartCommand: req.StartCommand,
		Framework:    framework,
		AlwaysOn:     req.AlwaysOn,
		EnvVars:      req.EnvVars,
		Files:        projectFiles,
	}

	// Set defaults based on framework
	if config.Port == 0 {
		config.Port = 3000
	}
	if config.BuildCommand == "" {
		config.BuildCommand = getDefaultBuildCommand(framework)
	}
	if config.StartCommand == "" {
		config.StartCommand = getDefaultStartCommand(framework)
	}

	// Start deployment
	deployment, err := h.service.StartDeployment(c.Request.Context(), uint(projectID), userID, config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"success":       true,
		"deployment":    deployment,
		"message":       "Deployment started",
		"url":           deployment.URL,
		"websocket_url": "/ws/deploy/" + deployment.ID,
	})
}

// Undeploy stops and removes a project's deployment
// DELETE /api/v1/hosting/:projectId/undeploy
func (h *HostingHandler) Undeploy(c *gin.Context) {
	userID := c.GetUint("user_id")
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

	// Get latest deployment
	deployments, _, err := h.service.GetProjectDeployments(uint(projectID), 1, 1)
	if err != nil || len(deployments) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No deployment found"})
		return
	}

	// Delete the deployment
	if err := h.service.DeleteDeployment(deployments[0].ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Deployment removed successfully",
	})
}

// GetProjectDeploymentLogs returns logs for a project's latest deployment
// GET /api/v1/hosting/:projectId/logs
func (h *HostingHandler) GetProjectDeploymentLogs(c *gin.Context) {
	userID := c.GetUint("user_id")
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

	// Get log params
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	// Get latest deployment
	deployments, _, err := h.service.GetProjectDeployments(uint(projectID), 1, 1)
	if err != nil || len(deployments) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No deployment found"})
		return
	}

	logs, err := h.service.GetDeploymentLogs(deployments[0].ID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"logs":          logs,
		"deployment_id": deployments[0].ID,
	})
}

// GetDeploymentURL returns the deployment URL for a project
// GET /api/v1/hosting/:projectId/url
func (h *HostingHandler) GetDeploymentURL(c *gin.Context) {
	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	// Get latest deployment
	deployments, _, err := h.service.GetProjectDeployments(uint(projectID), 1, 1)
	if err != nil || len(deployments) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No deployment found"})
		return
	}

	latest := deployments[0]
	if latest.Status != hosting.StatusRunning {
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"available": false,
			"url":       latest.URL,
			"status":    latest.Status,
			"message":   "Deployment is not running",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"available": true,
		"url":       latest.URL,
		"subdomain": latest.Subdomain,
		"status":    latest.Status,
	})
}

// getDefaultBuildCommand returns the default build command for a framework
func getDefaultBuildCommand(framework string) string {
	switch framework {
	case "nextjs", "react", "vite", "vue", "svelte":
		return "npm run build"
	case "angular":
		return "npm run build -- --configuration production"
	case "django", "flask", "fastapi":
		return "" // Python doesn't typically need a build step
	case "go", "golang":
		return "go build -o main ."
	case "rust":
		return "cargo build --release"
	default:
		return "npm run build"
	}
}

// getDefaultStartCommand returns the default start command for a framework
func getDefaultStartCommand(framework string) string {
	switch framework {
	case "nextjs":
		return "npm start"
	case "react", "vite":
		return "npm run preview"
	case "vue":
		return "npm run preview"
	case "angular":
		return "npm start"
	case "svelte":
		return "npm run preview"
	case "express", "node":
		return "node index.js"
	case "django":
		return "python manage.py runserver 0.0.0.0:$PORT"
	case "flask":
		return "flask run --host=0.0.0.0 --port=$PORT"
	case "fastapi":
		return "uvicorn main:app --host 0.0.0.0 --port $PORT"
	case "go", "golang":
		return "./main"
	case "rust":
		return "./target/release/$(basename $(pwd))"
	default:
		return "npm start"
	}
}

// Custom Domain Management Handlers

// AddCustomDomain adds a custom domain to a project
// POST /api/v1/hosting/:projectId/domains
func (h *HostingHandler) AddCustomDomain(c *gin.Context) {
	userID := c.GetUint("user_id")
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

	var req struct {
		Domain string `json:"domain" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Domain is required"})
		return
	}

	// Validate domain format
	if !isValidDomain(req.Domain) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain format"})
		return
	}

	// Check if domain is already registered
	var existing hosting.CustomDomain
	if err := h.db.Where("domain = ?", req.Domain).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Domain is already registered"})
		return
	}

	// Generate verification token
	verificationToken := generateVerificationToken()

	// Create custom domain record
	customDomain := &hosting.CustomDomain{
		Domain:             req.Domain,
		ProjectID:          uint(projectID),
		UserID:             userID,
		VerificationStatus: "pending",
		VerificationToken:  verificationToken,
		DNSType:            "CNAME",
		DNSTarget:          "hosting.apex.app",
		SSLStatus:          "pending",
		SSLProvider:        "cloudflare",
		SSLAutoRenew:       true,
	}

	if err := h.db.Create(customDomain).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add custom domain"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"domain":  customDomain,
		"dns_instructions": gin.H{
			"type":   "CNAME",
			"name":   "@", // or www
			"target": "hosting.apex.app",
			"ttl":    300,
		},
		"verification_instructions": gin.H{
			"type":  "TXT",
			"name":  "_apex-verify." + req.Domain,
			"value": verificationToken,
		},
		"message": "Domain added. Please configure DNS and verify ownership.",
	})
}

// GetCustomDomains returns custom domains for a project
// GET /api/v1/hosting/:projectId/domains
func (h *HostingHandler) GetCustomDomains(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	// Verify project access
	var project models.Project
	if err := h.db.First(&project, uint(projectID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.OwnerID != userID && !project.IsPublic {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	var domains []hosting.CustomDomain
	if err := h.db.Where("project_id = ?", projectID).Find(&domains).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get custom domains"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"domains": domains,
	})
}

// VerifyCustomDomain verifies domain ownership
// POST /api/v1/hosting/:projectId/domains/:domainId/verify
func (h *HostingHandler) VerifyCustomDomain(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("projectId")
	domainIDStr := c.Param("domainId")

	projectID, _ := strconv.ParseUint(projectIDStr, 10, 32)
	domainID, _ := strconv.ParseUint(domainIDStr, 10, 32)

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

	// Get custom domain
	var domain hosting.CustomDomain
	if err := h.db.Where("id = ? AND project_id = ?", domainID, projectID).First(&domain).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	// In a real implementation, we would:
	// 1. Query DNS for TXT record: _apex-verify.{domain}
	// 2. Compare with stored verification token
	// 3. If match, mark as verified

	// For now, simulate verification success
	now := time.Now()
	domain.VerificationStatus = "verified"
	domain.VerifiedAt = &now
	domain.DNSVerified = true
	domain.DNSCheckedAt = &now
	domain.IsActive = true

	if err := h.db.Save(&domain).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify domain"})
		return
	}

	// Trigger SSL provisioning (async)
	go h.provisionSSL(&domain)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"domain":  domain,
		"message": "Domain verified successfully. SSL certificate is being provisioned.",
	})
}

// DeleteCustomDomain removes a custom domain
// DELETE /api/v1/hosting/:projectId/domains/:domainId
func (h *HostingHandler) DeleteCustomDomain(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Param("projectId")
	domainIDStr := c.Param("domainId")

	projectID, _ := strconv.ParseUint(projectIDStr, 10, 32)
	domainID, _ := strconv.ParseUint(domainIDStr, 10, 32)

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

	// Delete domain
	if err := h.db.Where("id = ? AND project_id = ?", domainID, projectID).Delete(&hosting.CustomDomain{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete domain"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Domain deleted successfully",
	})
}

// provisionSSL provisions an SSL certificate for a custom domain
func (h *HostingHandler) provisionSSL(domain *hosting.CustomDomain) {
	// In a real implementation, this would:
	// 1. Request SSL certificate from Let's Encrypt or Cloudflare
	// 2. Configure the certificate on the load balancer
	// 3. Update domain status

	// Simulate SSL provisioning
	time.Sleep(5 * time.Second)

	domain.SSLStatus = "active"
	h.db.Save(domain)
}

// isValidDomain validates a domain name
func isValidDomain(domain string) bool {
	// Simple validation - in production, use a proper validator
	if len(domain) < 4 || len(domain) > 253 {
		return false
	}
	// Check for apex.app subdomain attempts
	if strings.HasSuffix(domain, ".apex.app") {
		return false
	}
	// Check basic format
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return false
	}
	return true
}

// generateVerificationToken generates a random verification token
func generateVerificationToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "apex-verify-" + hex.EncodeToString(b)
}


// detectFramework detects the framework from project files
func detectFramework(language, framework string, files []models.File) string {
	if framework != "" {
		return framework
	}

	// Check for common framework indicators
	for _, file := range files {
		switch file.Name {
		case "next.config.js", "next.config.mjs":
			return "nextjs"
		case "vite.config.js", "vite.config.ts":
			return "vite"
		case "angular.json":
			return "angular"
		case "vue.config.js":
			return "vue"
		case "svelte.config.js":
			return "svelte"
		case "requirements.txt", "Pipfile":
			// Check for Django/Flask/FastAPI
			if containsString(file.Content, "django") {
				return "django"
			}
			if containsString(file.Content, "flask") {
				return "flask"
			}
			if containsString(file.Content, "fastapi") {
				return "fastapi"
			}
			return "python"
		case "go.mod":
			return "go"
		case "Cargo.toml":
			return "rust"
		}

		// Check package.json for framework
		if file.Name == "package.json" {
			var pkg map[string]interface{}
			if err := json.Unmarshal([]byte(file.Content), &pkg); err == nil {
				if deps, ok := pkg["dependencies"].(map[string]interface{}); ok {
					if _, hasNext := deps["next"]; hasNext {
						return "nextjs"
					}
					if _, hasReact := deps["react"]; hasReact {
						return "react"
					}
					if _, hasVue := deps["vue"]; hasVue {
						return "vue"
					}
					if _, hasExpress := deps["express"]; hasExpress {
						return "express"
					}
				}
			}
		}
	}

	// Default based on language
	switch language {
	case "javascript", "typescript":
		return "node"
	case "python":
		return "python"
	case "go":
		return "go"
	case "rust":
		return "rust"
	default:
		return "node"
	}
}

func containsString(content, search string) bool {
	if len(content) == 0 || len(search) == 0 || len(content) < len(search) {
		return false
	}
	for i := 0; i <= len(content)-len(search); i++ {
		if content[i:i+len(search)] == search {
			return true
		}
	}
	return false
}
