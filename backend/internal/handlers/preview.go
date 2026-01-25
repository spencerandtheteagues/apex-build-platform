// Package handlers - Live Preview HTTP Handlers for APEX.BUILD
package handlers

import (
	"net/http"
	"strconv"

	"apex-build/internal/preview"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PreviewHandler handles preview-related endpoints
type PreviewHandler struct {
	db     *gorm.DB
	server *preview.PreviewServer
}

// NewPreviewHandler creates a new preview handler
func NewPreviewHandler(db *gorm.DB, server *preview.PreviewServer) *PreviewHandler {
	return &PreviewHandler{
		db:     db,
		server: server,
	}
}

// StartPreview starts a live preview session for a project
// POST /api/v1/preview/start
func (h *PreviewHandler) StartPreview(c *gin.Context) {
	userID := c.GetUint("userID")

	var req struct {
		ProjectID    uint              `json:"project_id" binding:"required"`
		EntryPoint   string            `json:"entry_point"`
		Framework    string            `json:"framework"`
		EnvVars      map[string]string `json:"env_vars"`
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

	// Auto-detect entry point if not specified
	if req.EntryPoint == "" {
		req.EntryPoint = h.detectEntryPoint(req.ProjectID)
	}

	// Auto-detect framework if not specified
	if req.Framework == "" {
		req.Framework = h.detectFramework(req.ProjectID)
	}

	config := &preview.PreviewConfig{
		ProjectID:  req.ProjectID,
		EntryPoint: req.EntryPoint,
		Framework:  req.Framework,
		EnvVars:    req.EnvVars,
	}

	status, err := h.server.StartPreview(c.Request.Context(), config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"preview": status,
		"message": "Preview started successfully",
	})
}

// StopPreview stops a live preview session
// POST /api/v1/preview/stop
func (h *PreviewHandler) StopPreview(c *gin.Context) {
	userID := c.GetUint("userID")

	var req struct {
		ProjectID uint `json:"project_id" binding:"required"`
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

	if err := h.server.StopPreview(c.Request.Context(), req.ProjectID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Preview stopped",
	})
}

// GetPreviewStatus returns the status of a preview session
// GET /api/v1/preview/status/:projectId
func (h *PreviewHandler) GetPreviewStatus(c *gin.Context) {
	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	status := h.server.GetPreviewStatus(uint(projectID))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"preview": status,
	})
}

// RefreshPreview triggers a reload of the preview
// POST /api/v1/preview/refresh
func (h *PreviewHandler) RefreshPreview(c *gin.Context) {
	var req struct {
		ProjectID    uint     `json:"project_id" binding:"required"`
		ChangedFiles []string `json:"changed_files"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.server.RefreshPreview(req.ProjectID, req.ChangedFiles); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Preview refreshed",
	})
}

// HotReload sends a hot reload update for a specific file
// POST /api/v1/preview/hot-reload
func (h *PreviewHandler) HotReload(c *gin.Context) {
	var req struct {
		ProjectID uint   `json:"project_id" binding:"required"`
		FilePath  string `json:"file_path" binding:"required"`
		Content   string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.server.HotReload(req.ProjectID, req.FilePath, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Hot reload sent",
	})
}

// ListPreviews returns all active preview sessions for a user
// GET /api/v1/preview/list
func (h *PreviewHandler) ListPreviews(c *gin.Context) {
	userID := c.GetUint("userID")

	// Get user's projects
	var projects []models.Project
	if err := h.db.Where("owner_id = ?", userID).Find(&projects).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch projects"})
		return
	}

	projectIDs := make(map[uint]bool)
	for _, p := range projects {
		projectIDs[p.ID] = true
	}

	// Filter previews to only user's projects
	allPreviews := h.server.GetAllPreviews()
	userPreviews := make([]*preview.PreviewStatus, 0)
	for _, p := range allPreviews {
		if projectIDs[p.ProjectID] {
			userPreviews = append(userPreviews, p)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"previews": userPreviews,
		"count":    len(userPreviews),
	})
}

// GetPreviewURL returns the preview URL for embedding
// GET /api/v1/preview/url/:projectId
func (h *PreviewHandler) GetPreviewURL(c *gin.Context) {
	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	status := h.server.GetPreviewStatus(uint(projectID))

	if !status.Active {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Preview not running",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"url":        status.URL,
		"port":       status.Port,
		"started_at": status.StartedAt,
	})
}

// Helper methods

func (h *PreviewHandler) detectEntryPoint(projectID uint) string {
	// Check for common entry points
	entryPoints := []string{
		"index.html",
		"public/index.html",
		"src/index.html",
		"dist/index.html",
		"build/index.html",
	}

	for _, ep := range entryPoints {
		var count int64
		h.db.Model(&models.File{}).
			Where("project_id = ? AND path = ?", projectID, ep).
			Count(&count)
		if count > 0 {
			return ep
		}
	}

	return "index.html"
}

func (h *PreviewHandler) detectFramework(projectID uint) string {
	// Check package.json for framework detection
	var file models.File
	if err := h.db.Where("project_id = ? AND path = ?", projectID, "package.json").First(&file).Error; err == nil {
		content := file.Content

		// Check for common frameworks
		frameworks := map[string][]string{
			"react":  {"react", "react-dom"},
			"vue":    {"vue"},
			"svelte": {"svelte"},
			"next":   {"next"},
			"nuxt":   {"nuxt"},
			"angular": {"@angular/core"},
		}

		for framework, deps := range frameworks {
			for _, dep := range deps {
				if contains(content, `"`+dep+`"`) {
					return framework
				}
			}
		}
	}

	// Check for Python frameworks
	var reqFile models.File
	if err := h.db.Where("project_id = ? AND path = ?", projectID, "requirements.txt").First(&reqFile).Error; err == nil {
		if contains(reqFile.Content, "flask") {
			return "flask"
		}
		if contains(reqFile.Content, "django") {
			return "django"
		}
		if contains(reqFile.Content, "fastapi") {
			return "fastapi"
		}
	}

	return "vanilla"
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
