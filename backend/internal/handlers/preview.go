// Package handlers - Live Preview HTTP Handlers for APEX.BUILD
package handlers

import (
	"net/http"
	"strconv"

	"apex-build/internal/bundler"
	"apex-build/internal/preview"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PreviewHandler handles preview-related endpoints
type PreviewHandler struct {
	db             *gorm.DB
	server         *preview.PreviewServer
	factory        *preview.PreviewServerFactory
	serverRunner   *preview.ServerRunner
	bundlerService *bundler.Service
}

// NewPreviewHandler creates a new preview handler
func NewPreviewHandler(db *gorm.DB, server *preview.PreviewServer) *PreviewHandler {
	return &PreviewHandler{
		db:             db,
		server:         server,
		serverRunner:   preview.NewServerRunner(db),
		bundlerService: bundler.NewService(db),
	}
}

// NewPreviewHandlerWithFactory creates a preview handler with Docker sandbox support
func NewPreviewHandlerWithFactory(db *gorm.DB, factory *preview.PreviewServerFactory) *PreviewHandler {
	return &PreviewHandler{
		db:             db,
		server:         factory.GetProcessServer(),
		factory:        factory,
		serverRunner:   preview.NewServerRunner(db),
		bundlerService: bundler.NewService(db),
	}
}

// StartPreview starts a live preview session for a project
// POST /api/v1/preview/start
func (h *PreviewHandler) StartPreview(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		ProjectID    uint              `json:"project_id" binding:"required"`
		EntryPoint   string            `json:"entry_point"`
		Framework    string            `json:"framework"`
		EnvVars      map[string]string `json:"env_vars"`
		Sandbox      bool              `json:"sandbox"` // Enable Docker container sandbox
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

	var status *preview.PreviewStatus
	var err error

	// Use factory if available for sandbox support
	if h.factory != nil {
		status, err = h.factory.StartPreview(c.Request.Context(), config, req.Sandbox)
	} else {
		status, err = h.server.StartPreview(c.Request.Context(), config)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := gin.H{
		"success": true,
		"preview": status,
		"message": "Preview started successfully",
		"sandbox": req.Sandbox,
	}

	// Include Docker availability info
	if h.factory != nil {
		response["docker_available"] = h.factory.IsDockerAvailable()
	}

	c.JSON(http.StatusOK, response)
}

// StopPreview stops a live preview session
// POST /api/v1/preview/stop
func (h *PreviewHandler) StopPreview(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		ProjectID uint `json:"project_id" binding:"required"`
		Sandbox   bool `json:"sandbox"` // Whether this was a sandbox preview
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

	var err error
	if h.factory != nil {
		err = h.factory.StopPreview(c.Request.Context(), req.ProjectID, req.Sandbox)
	} else {
		err = h.server.StopPreview(c.Request.Context(), req.ProjectID)
	}

	if err != nil {
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

	status := h.server.GetPreviewStatus(uint(projectID))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"preview": status,
	})
}

// RefreshPreview triggers a reload of the preview
// POST /api/v1/preview/refresh
func (h *PreviewHandler) RefreshPreview(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		ProjectID    uint     `json:"project_id" binding:"required"`
		ChangedFiles []string `json:"changed_files"`
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
	userID := c.GetUint("user_id")

	var req struct {
		ProjectID uint   `json:"project_id" binding:"required"`
		FilePath  string `json:"file_path" binding:"required"`
		Content   string `json:"content" binding:"required"`
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
	userID := c.GetUint("user_id")

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

// GetDockerStatus returns Docker availability and container statistics
// GET /api/v1/preview/docker/status
func (h *PreviewHandler) GetDockerStatus(c *gin.Context) {
	if h.factory == nil {
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"available": false,
			"message":   "Docker container previews not configured",
		})
		return
	}

	status := h.factory.GetDockerStatus()

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"available":        status.Available,
		"container_count":  status.ContainerCount,
		"max_containers":   status.MaxContainers,
		"total_created":    status.TotalCreated,
		"failed_count":     status.FailedCount,
		"total_build_ms":   status.TotalBuildTime,
		"total_runtime_ms": status.TotalRuntime,
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

// ========== BUNDLER ENDPOINTS ==========

// BuildProject triggers a bundle build for a project
// POST /api/v1/preview/build
func (h *PreviewHandler) BuildProject(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		ProjectID  uint   `json:"project_id" binding:"required"`
		EntryPoint string `json:"entry_point"`
		Format     string `json:"format"`
		Minify     bool   `json:"minify"`
		SourceMap  bool   `json:"source_map"`
		Framework  string `json:"framework"`
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

	// Check if bundler is available
	if !h.bundlerService.IsAvailable() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Bundler not available. Please install esbuild: npm install -g esbuild",
		})
		return
	}

	// Create bundle config
	config := bundler.BundleConfig{
		EntryPoint: req.EntryPoint,
		Format:     req.Format,
		Minify:     req.Minify,
		SourceMap:  req.SourceMap,
		Framework:  req.Framework,
	}

	// Auto-detect if not specified
	if config.EntryPoint == "" {
		config.EntryPoint = h.detectBundleEntryPoint(req.ProjectID)
	}
	if config.Framework == "" {
		config.Framework = h.detectFramework(req.ProjectID)
	}
	if config.Format == "" {
		config.Format = "esm"
	}

	// Perform the build
	result, err := h.bundlerService.BundleProject(c.Request.Context(), req.ProjectID, config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Convert errors to response format
	var errors []gin.H
	for _, e := range result.Errors {
		errors = append(errors, gin.H{
			"message": e.Message,
			"file":    e.File,
			"line":    e.Line,
			"column":  e.Column,
			"text":    e.Text,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     result.Success,
		"duration_ms": result.Duration.Milliseconds(),
		"warnings":    result.Warnings,
		"errors":      errors,
		"hash":        result.Hash,
		"js_size":     len(result.OutputJS),
		"css_size":    len(result.OutputCSS),
	})
}

// GetBundlerStatus returns the status of the bundler service
// GET /api/v1/preview/bundler/status
func (h *PreviewHandler) GetBundlerStatus(c *gin.Context) {
	status := h.bundlerService.Status()

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"available":   status.Available,
		"version":     status.Version,
		"cache_stats": status.CacheStats,
	})
}

// InvalidateBundleCache invalidates the bundle cache for a project
// POST /api/v1/preview/bundler/invalidate
func (h *PreviewHandler) InvalidateBundleCache(c *gin.Context) {
	userID := c.GetUint("user_id")

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

	h.bundlerService.InvalidateCache(req.ProjectID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Bundle cache invalidated",
	})
}

// detectBundleEntryPoint finds the entry point for bundling
func (h *PreviewHandler) detectBundleEntryPoint(projectID uint) string {
	// Priority order for entry points
	entryPoints := []string{
		"src/index.tsx",
		"src/index.ts",
		"src/index.jsx",
		"src/index.js",
		"src/main.tsx",
		"src/main.ts",
		"src/main.jsx",
		"src/main.js",
		"src/App.tsx",
		"src/App.jsx",
		"index.tsx",
		"index.ts",
		"index.jsx",
		"index.js",
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

	return "src/index.js"
}

// ========== BACKEND SERVER CONTROL ENDPOINTS ==========

// StartServer starts a backend server for a project
// POST /api/v1/preview/server/start
func (h *PreviewHandler) StartServer(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		ProjectID uint              `json:"project_id" binding:"required"`
		EntryFile string            `json:"entry_file"`
		Command   string            `json:"command"`
		EnvVars   map[string]string `json:"env_vars"`
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

	// Start the server
	config := &preview.ServerConfig{
		ProjectID: req.ProjectID,
		EntryFile: req.EntryFile,
		Command:   req.Command,
		EnvVars:   req.EnvVars,
	}

	proc, err := h.serverRunner.Start(c.Request.Context(), config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Link to preview session if one exists
	h.linkServerToPreview(req.ProjectID, proc)

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"port":       proc.Port,
		"pid":        proc.Pid,
		"command":    proc.Command,
		"entry_file": proc.EntryFile,
		"url":        proc.URL,
		"message":    "Backend server started successfully",
	})
}

// StopServer stops a backend server for a project
// POST /api/v1/preview/server/stop
func (h *PreviewHandler) StopServer(c *gin.Context) {
	userID := c.GetUint("user_id")

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

	// Stop the server
	if err := h.serverRunner.Stop(c.Request.Context(), req.ProjectID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Unlink from preview session
	h.unlinkServerFromPreview(req.ProjectID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Backend server stopped",
	})
}

// GetServerStatus returns the status of a backend server
// GET /api/v1/preview/server/status/:projectId
func (h *PreviewHandler) GetServerStatus(c *gin.Context) {
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

	status := h.serverRunner.GetStatus(uint(projectID))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"server":  status,
	})
}

// GetServerLogs returns the logs of a backend server
// GET /api/v1/preview/server/logs/:projectId
func (h *PreviewHandler) GetServerLogs(c *gin.Context) {
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

	logs := h.serverRunner.GetLogs(uint(projectID))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"stdout":  logs.Stdout,
		"stderr":  logs.Stderr,
	})
}

// DetectServer detects backend server configuration from project files
// GET /api/v1/preview/server/detect/:projectId
func (h *PreviewHandler) DetectServer(c *gin.Context) {
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

	detection, err := h.serverRunner.DetectServer(c.Request.Context(), uint(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"has_backend": detection.HasBackend,
		"server_type": detection.ServerType,
		"entry_file":  detection.EntryFile,
		"command":     detection.Command,
		"framework":   detection.Framework,
	})
}

// GetServerRunner returns the server runner instance
func (h *PreviewHandler) GetServerRunner() *preview.ServerRunner {
	return h.serverRunner
}

// Helper to link backend server to preview session
func (h *PreviewHandler) linkServerToPreview(projectID uint, proc *preview.ServerProcess) {
	h.server.SetBackendServer(projectID, proc)
}

// Helper to unlink backend server from preview session
func (h *PreviewHandler) unlinkServerFromPreview(projectID uint) {
	h.server.ClearBackendServer(projectID)
}
