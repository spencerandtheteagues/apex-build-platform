// Package handlers - Live Preview HTTP Handlers for APEX.BUILD
package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"apex-build/internal/auth"
	"apex-build/internal/bundler"
	"apex-build/internal/metrics"
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
	authService    *auth.AuthService
	requireSandbox bool
}

// NewPreviewHandler creates a new preview handler
func NewPreviewHandler(db *gorm.DB, server *preview.PreviewServer, authService *auth.AuthService) *PreviewHandler {
	return &PreviewHandler{
		db:             db,
		server:         server,
		serverRunner:   preview.NewServerRunnerFromEnv(db),
		bundlerService: bundler.NewService(db),
		authService:    authService,
		requireSandbox: previewSandboxRequired(),
	}
}

// NewPreviewHandlerWithFactory creates a preview handler with Docker sandbox support
func NewPreviewHandlerWithFactory(db *gorm.DB, factory *preview.PreviewServerFactory, authService *auth.AuthService) *PreviewHandler {
	return &PreviewHandler{
		db:             db,
		server:         factory.GetProcessServer(),
		factory:        factory,
		serverRunner:   preview.NewServerRunnerFromEnv(db),
		bundlerService: bundler.NewService(db),
		authService:    authService,
		requireSandbox: previewSandboxRequired(),
	}
}

// FeatureStatus summarizes preview subsystem readiness for health reporting.
func (h *PreviewHandler) FeatureStatus() map[string]interface{} {
	bundlerStatus := h.bundlerService.Status()
	serverRunnerStatus := map[string]interface{}{
		"available": h.backendPreviewAvailable(),
	}
	if h.serverRunner != nil {
		serverRunnerStatus["runtime"] = h.serverRunner.RuntimeName()
	}
	if !h.backendPreviewAvailable() {
		serverRunnerStatus["reason"] = h.backendPreviewDisabledReason()
	}
	status := map[string]interface{}{
		"bundler": map[string]interface{}{
			"available":   bundlerStatus.Available,
			"version":     bundlerStatus.Version,
			"cache_stats": bundlerStatus.CacheStats,
			"last_error":  bundlerStatus.LastError,
		},
		"server_runner": serverRunnerStatus,
	}

	if h.factory != nil {
		status["docker"] = h.factory.GetDockerStatus()
	}
	status["sandbox_required"] = h.requireSandbox
	status["sandbox_degraded"] = h.sandboxFallbackActive()
	status["sandbox_ready"] = !h.requireSandbox || (h.factory != nil && h.factory.IsDockerAvailable()) || h.sandboxFallbackActive()

	return status
}

// StartPreview starts a live preview session for a project
// POST /api/v1/preview/start
func (h *PreviewHandler) StartPreview(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		ProjectID  uint              `json:"project_id" binding:"required"`
		EntryPoint string            `json:"entry_point"`
		Framework  string            `json:"framework"`
		EnvVars    map[string]string `json:"env_vars"`
		Sandbox    bool              `json:"sandbox"` // Enable Docker container sandbox
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

	// Auto-detect framework if not specified
	if req.Framework == "" {
		req.Framework = h.detectFramework(req.ProjectID)
	}

	// Auto-detect entry point if not specified.
	// Prefer a JS/TS source entry whenever available (works with esbuild for React/SPA projects).
	// Fallback to HTML-style entry only when no source entry exists.
	if req.EntryPoint == "" {
		if bundleEntry := h.detectBundleEntryPoint(req.ProjectID); bundleEntry != "" {
			req.EntryPoint = bundleEntry
		} else {
			req.EntryPoint = h.detectEntryPoint(req.ProjectID)
		}
	}

	if isNextPreviewFramework(req.Framework) {
		previewStatus, serverStatus, startErr := h.startFrameworkRuntimePreview(c, req.ProjectID, req.EnvVars, previewRuntimeStartTimeout())
		if startErr != nil {
			metrics.RecordPreviewStart("frontend", "error", false)
			c.JSON(http.StatusInternalServerError, gin.H{"error": startErr.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success":          true,
			"preview":          previewStatus,
			"server":           serverStatus,
			"proxy_url":        previewStatus.URL,
			"message":          "Next.js preview started successfully",
			"sandbox":          false,
			"sandbox_degraded": h.sandboxFallbackActive(),
			"runtime_preview":  true,
		})
		metrics.RecordPreviewStart("frontend", "success", false)
		return
	}

	config := &preview.PreviewConfig{
		ProjectID:  req.ProjectID,
		EntryPoint: req.EntryPoint,
		Framework:  req.Framework,
		EnvVars:    req.EnvVars,
	}

	useSandbox, sandboxErr := h.resolveRequestedPreviewSandbox(req.Sandbox)
	if sandboxErr != nil {
		metrics.RecordPreviewStart("frontend", "sandbox_unavailable", true)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": sandboxErr.Error()})
		return
	}
	req.Sandbox = useSandbox

	status, actualSandbox, fallbackDegraded, fallbackReason, err := h.startFrontendPreviewWithFallback(c.Request.Context(), config, req.Sandbox)
	if err != nil {
		metrics.RecordPreviewStart("frontend", "error", req.Sandbox)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	req.Sandbox = actualSandbox

	// Override URL to use proxy so it's accessible from the browser
	status.URL = h.buildProxyURL(c, req.ProjectID)
	h.setPreviewAccessCookie(c, req.ProjectID)

	response := gin.H{
		"success":          true,
		"preview":          status,
		"message":          "Preview started successfully",
		"sandbox":          req.Sandbox,
		"sandbox_degraded": (h.sandboxFallbackActive() && !req.Sandbox) || fallbackDegraded,
	}
	if fallbackDegraded {
		response["degraded"] = true
		response["diagnostics"] = gin.H{
			"frontend_fallback": true,
			"sandbox_error":     fallbackReason,
		}
		response["message"] = "Sandbox preview fell back to process preview"
	}

	// Include Docker availability info
	if h.factory != nil {
		response["docker_available"] = h.factory.IsDockerAvailable()
	}

	previewStartResult := "success"
	if fallbackDegraded {
		previewStartResult = "degraded"
	}
	metrics.RecordPreviewStart("frontend", previewStartResult, req.Sandbox)
	c.JSON(http.StatusOK, response)
}

// StartFullStackPreview starts the frontend preview and backend runtime in one operation.
// POST /api/v1/preview/fullstack/start
func (h *PreviewHandler) StartFullStackPreview(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		ProjectID      uint              `json:"project_id" binding:"required"`
		EntryPoint     string            `json:"entry_point"`
		Framework      string            `json:"framework"`
		EnvVars        map[string]string `json:"env_vars"`
		Sandbox        bool              `json:"sandbox"`
		StartBackend   *bool             `json:"start_backend,omitempty"`
		RequireBackend bool              `json:"require_backend,omitempty"`
		BackendEntry   string            `json:"backend_entry_file,omitempty"`
		BackendCommand string            `json:"backend_command,omitempty"`
		BackendEnvVars map[string]string `json:"backend_env_vars,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startBackend := false
	if req.RequireBackend {
		startBackend = true
	} else if req.StartBackend != nil {
		startBackend = *req.StartBackend
	} else if strings.TrimSpace(req.BackendEntry) != "" || strings.TrimSpace(req.BackendCommand) != "" {
		startBackend = true
	}
	if (startBackend || req.RequireBackend) && !requirePaidBackendPlan(c, h.db, userID, "backend preview") {
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

	if req.Framework == "" {
		req.Framework = h.detectFramework(req.ProjectID)
	}
	if req.EntryPoint == "" {
		if bundleEntry := h.detectBundleEntryPoint(req.ProjectID); bundleEntry != "" {
			req.EntryPoint = bundleEntry
		} else {
			req.EntryPoint = h.detectEntryPoint(req.ProjectID)
		}
	}

	if isNextPreviewFramework(req.Framework) {
		nextEnvVars := mergePreviewEnvVars(req.EnvVars, req.BackendEnvVars)
		previewStatus, serverStatus, startErr := h.startFrameworkRuntimePreview(c, req.ProjectID, nextEnvVars, previewRuntimeStartTimeout())
		if startErr != nil {
			if req.RequireBackend {
				metrics.RecordPreviewStart("fullstack", "error", false)
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   startErr.Error(),
				})
				return
			}

			fallbackStatus, fallbackSandbox, fallbackErr := h.startFrontendPreviewFallback(c, req.ProjectID, req.EntryPoint, req.Framework, req.EnvVars, req.Sandbox)
			if fallbackErr != nil {
				metrics.RecordPreviewStart("fullstack", "error", fallbackSandbox)
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   fmt.Sprintf("%s; frontend preview fallback also failed: %v", startErr.Error(), fallbackErr),
				})
				return
			}
			metrics.RecordPreviewStart("fullstack", "degraded", fallbackSandbox)
			c.JSON(http.StatusOK, gin.H{
				"success":          true,
				"preview":          fallbackStatus,
				"server":           serverStatus,
				"proxy_url":        fallbackStatus.URL,
				"degraded":         true,
				"diagnostics":      gin.H{"preview_started": true, "runtime_preview": "next", "runtime_error": startErr.Error(), "frontend_fallback": true},
				"message":          "Next.js runtime preview fell back to frontend bundle preview",
				"sandbox":          fallbackSandbox,
				"sandbox_degraded": h.sandboxFallbackActive() && !fallbackSandbox,
				"runtime_preview":  false,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success":          true,
			"preview":          previewStatus,
			"server":           serverStatus,
			"proxy_url":        previewStatus.URL,
			"degraded":         false,
			"diagnostics":      gin.H{"preview_started": true, "runtime_preview": "next"},
			"message":          "Next.js full-stack preview started",
			"sandbox":          false,
			"sandbox_degraded": h.sandboxFallbackActive(),
			"runtime_preview":  true,
		})
		metrics.RecordPreviewStart("fullstack", "success", false)
		return
	}

	previewConfig := &preview.PreviewConfig{
		ProjectID:  req.ProjectID,
		EntryPoint: req.EntryPoint,
		Framework:  req.Framework,
		EnvVars:    req.EnvVars,
	}

	useSandbox, sandboxErr := h.resolveRequestedPreviewSandbox(req.Sandbox)
	if sandboxErr != nil {
		metrics.RecordPreviewStart("fullstack", "sandbox_unavailable", true)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": sandboxErr.Error()})
		return
	}
	req.Sandbox = useSandbox

	var previewStatus *preview.PreviewStatus
	var degraded bool
	var sandboxFallbackReason string
	var err error
	previewStatus, req.Sandbox, degraded, sandboxFallbackReason, err = h.startFrontendPreviewWithFallback(c.Request.Context(), previewConfig, req.Sandbox)
	if err != nil {
		metrics.RecordPreviewStart("fullstack", "error", req.Sandbox)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	previewStatus.URL = h.buildProxyURL(c, req.ProjectID)
	h.setPreviewAccessCookie(c, req.ProjectID)

	var serverStatus *preview.ServerStatus
	diagnostics := gin.H{
		"preview_started":   true,
		"backend_requested": startBackend,
	}
	if sandboxFallbackReason != "" {
		diagnostics["frontend_fallback"] = true
		diagnostics["sandbox_error"] = sandboxFallbackReason
	}

	if startBackend {
		if !h.backendPreviewAvailable() {
			degraded = true
			diagnostics["backend_started"] = false
			diagnostics["backend_error"] = h.backendPreviewDisabledReason()
			if req.RequireBackend {
				metrics.RecordPreviewStart("fullstack", "backend_required_failed", req.Sandbox)
				c.JSON(http.StatusBadGateway, gin.H{
					"success":     false,
					"error":       "backend preview failed to start",
					"details":     diagnostics["backend_error"],
					"preview":     previewStatus,
					"server":      serverStatus,
					"proxy_url":   previewStatus.URL,
					"diagnostics": diagnostics,
					"degraded":    true,
				})
				return
			}
			goto response
		}

		serverConfig := &preview.ServerConfig{
			ProjectID: req.ProjectID,
			EntryFile: req.BackendEntry,
			Command:   req.BackendCommand,
			EnvVars:   req.BackendEnvVars,
		}
		startCtx := c.Request.Context()
		var cancel context.CancelFunc
		if !req.RequireBackend {
			timeout := previewOptionalBackendStartTimeout()
			startCtx, cancel = context.WithTimeout(startCtx, timeout)
			serverConfig.ReadyTimeout = timeout
		}
		if cancel != nil {
			defer cancel()
		}
		proc, startErr := h.serverRunner.Start(startCtx, serverConfig)
		if startErr != nil {
			degraded = true
			diagnostics["backend_started"] = false
			diagnostics["backend_error"] = startErr.Error()
			if !req.RequireBackend && errors.Is(startCtx.Err(), context.DeadlineExceeded) {
				diagnostics["backend_pending"] = false
				diagnostics["backend_timeout_ms"] = previewOptionalBackendStartTimeout().Milliseconds()
			}
			serverStatus = h.serverRunner.GetStatus(req.ProjectID)
			if req.RequireBackend {
				metrics.RecordPreviewStart("fullstack", "backend_required_failed", req.Sandbox)
				c.JSON(http.StatusBadGateway, gin.H{
					"success":     false,
					"error":       "backend preview failed to start",
					"details":     startErr.Error(),
					"preview":     previewStatus,
					"server":      serverStatus,
					"proxy_url":   previewStatus.URL,
					"diagnostics": diagnostics,
					"degraded":    true,
				})
				return
			}
		} else {
			h.linkServerToPreview(req.ProjectID, proc)
			serverStatus = h.serverRunner.GetStatus(req.ProjectID)
			diagnostics["backend_started"] = true
			diagnostics["backend_port"] = proc.Port
			diagnostics["backend_url"] = proc.URL
			diagnostics["backend_detected_command"] = proc.Command
		}
	} else {
		diagnostics["backend_started"] = false
		diagnostics["backend_skipped"] = true
	}

response:
	resp := gin.H{
		"success":          true,
		"preview":          previewStatus,
		"server":           serverStatus,
		"proxy_url":        previewStatus.URL,
		"degraded":         degraded,
		"diagnostics":      diagnostics,
		"message":          "Full-stack preview started",
		"sandbox":          req.Sandbox,
		"sandbox_degraded": (h.sandboxFallbackActive() && !req.Sandbox) || sandboxFallbackReason != "",
	}
	if h.factory != nil {
		resp["docker_available"] = h.factory.IsDockerAvailable()
	}
	if degraded {
		metrics.RecordPreviewStart("fullstack", "degraded", req.Sandbox)
	} else {
		metrics.RecordPreviewStart("fullstack", "success", req.Sandbox)
	}
	c.JSON(http.StatusOK, resp)
}

// StopPreview stops a live preview session
// POST /api/v1/preview/stop
func (h *PreviewHandler) StopPreview(c *gin.Context) {
	userID := c.GetUint("user_id")
	if !requirePaidBackendPlan(c, h.db, userID, "backend preview") {
		return
	}

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
	useSandbox, sandboxErr := h.resolveRequestedPreviewSandbox(req.Sandbox)
	if sandboxErr != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": sandboxErr.Error()})
		return
	}
	req.Sandbox = useSandbox

	backendStopped := false
	if h.backendPreviewAvailable() {
		if serverStatus := h.serverRunner.GetStatus(req.ProjectID); serverStatus != nil && serverStatus.Running {
			if stopErr := h.serverRunner.Stop(c.Request.Context(), req.ProjectID); stopErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": stopErr.Error()})
				return
			}
			h.unlinkServerFromPreview(req.ProjectID)
			backendStopped = true
		}
	}
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
		"success":         true,
		"message":         "Preview stopped",
		"backend_stopped": backendStopped,
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

	useSandbox, sandboxErr := h.resolveRequestedPreviewSandbox(c.Query("sandbox") == "true" || c.Query("sandbox") == "1")
	if sandboxErr != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": sandboxErr.Error()})
		return
	}

	status, activeSandbox := h.getPreviewStatus(uint(projectID), useSandbox)
	sandboxDegraded := h.previewStatusSandboxDegraded(uint(projectID), status, activeSandbox)

	// Override URL to proxy
	if status != nil && status.Active {
		h.setPreviewAccessCookie(c, uint(projectID))
		status.URL = h.buildPublicPreviewURL(c, uint(projectID), h.isFrameworkRuntimePreviewActive(uint(projectID)))
	}

	var serverStatus *preview.ServerStatus
	if h.backendPreviewAvailable() {
		serverStatus = h.serverRunner.GetStatus(uint(projectID))
	}

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"preview":          status,
		"sandbox":          activeSandbox,
		"sandbox_degraded": sandboxDegraded,
		"server":           serverStatus,
	})
}

// RefreshPreview triggers a reload of the preview
// POST /api/v1/preview/refresh
func (h *PreviewHandler) RefreshPreview(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		ProjectID    uint     `json:"project_id" binding:"required"`
		ChangedFiles []string `json:"changed_files"`
		Sandbox      bool     `json:"sandbox"`
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

	useSandbox, sandboxErr := h.resolvePreviewOperationSandbox(req.ProjectID, req.Sandbox)
	if sandboxErr != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": sandboxErr.Error()})
		return
	}
	req.Sandbox = useSandbox
	if h.factory != nil {
		if err := h.factory.RefreshPreview(req.ProjectID, req.ChangedFiles, useSandbox); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else if err := h.server.RefreshPreview(req.ProjectID, req.ChangedFiles); err != nil {
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

	// Block hot reload only when sandbox is actively enforced AND Docker is available.
	// When Docker is unavailable (fallback mode), allow hot reload through the process preview path.
	if h.requireSandbox && !h.sandboxFallbackActive() {
		c.JSON(http.StatusConflict, gin.H{"error": "hot reload is unavailable while secure sandbox preview is enforced; use refresh instead"})
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
	if h.factory != nil {
		allPreviews = h.factory.GetAllPreviews()
	}
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

	useSandbox, sandboxErr := h.resolveRequestedPreviewSandbox(c.Query("sandbox") == "true" || c.Query("sandbox") == "1")
	if sandboxErr != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": sandboxErr.Error()})
		return
	}

	status, activeSandbox := h.getPreviewStatus(uint(projectID), useSandbox)

	if status == nil || !status.Active {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Preview not running",
		})
		return
	}

	h.setPreviewAccessCookie(c, uint(projectID))

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"url":        h.buildPublicPreviewURL(c, uint(projectID), h.isFrameworkRuntimePreviewActive(uint(projectID))),
		"port":       status.Port,
		"started_at": status.StartedAt,
		"sandbox":    activeSandbox,
	})
}

// ProxyPreview proxies preview traffic through the API host so it can be embedded securely
// GET/POST/etc /api/v1/preview/proxy/:projectId/*path
func (h *PreviewHandler) ProxyPreview(c *gin.Context) {
	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}
	if h.handlePreviewCORS(c) {
		return
	}

	userID, err := h.resolvePreviewUserID(c, uint(projectID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	previewToken := h.issuePreviewAccessToken(c, uint(projectID))
	h.setPreviewAccessCookie(c, uint(projectID))

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

	useSandbox, sandboxErr := h.resolveRequestedPreviewSandbox(c.Query("sandbox") == "true" || c.Query("sandbox") == "1")
	if sandboxErr != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": sandboxErr.Error()})
		return
	}
	status, _ := h.getPreviewStatus(uint(projectID), useSandbox)
	if status == nil || !status.Active {
		c.JSON(http.StatusNotFound, gin.H{"error": "Preview not running"})
		return
	}

	targetURL, err := previewProxyTargetURL(status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to build preview proxy"})
		return
	}

	// Check whether the backend server is also running for this project, so we can
	// inject the backend proxy URL into the HTML and make fetch() calls work.
	backendProxyURL := ""
	if h.backendPreviewAvailable() {
		if serverStatus := h.serverRunner.GetStatus(uint(projectID)); serverStatus != nil && serverStatus.Running {
			backendProxyURL = h.buildBackendProxyURL(c, uint(projectID))
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.FlushInterval = -1
	proxy.ModifyResponse = func(resp *http.Response) error {
		h.applyPreviewResponseHeaders(resp.Header, c.GetHeader("Origin"), strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html"))
		contentType := strings.ToLower(resp.Header.Get("Content-Type"))
		isHTML := strings.Contains(contentType, "text/html")
		responsePath := ""
		if resp.Request != nil && resp.Request.URL != nil {
			responsePath = resp.Request.URL.Path
		}
		isJavaScript := isPreviewJavaScriptResponse(contentType, responsePath)
		if !isHTML && !isJavaScript {
			return nil
		}

		originalBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return readErr
		}
		_ = resp.Body.Close()

		var rewritten string
		if isHTML {
			rewritten = h.rewritePreviewHTMLForProxyWithBackend(string(originalBody), uint(projectID), backendProxyURL, previewToken)
		} else {
			rewritten = h.rewritePreviewJavaScriptForProxyWithPrefix(string(originalBody), h.buildProxyBaseURL(c, uint(projectID)), previewToken)
		}
		resp.Body = io.NopCloser(bytes.NewBufferString(rewritten))
		resp.ContentLength = int64(len(rewritten))
		resp.Header.Set("Content-Length", strconv.Itoa(len(rewritten)))
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
		h.applyPreviewResponseHeaders(w.Header(), c.GetHeader("Origin"), false)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"Preview proxy unavailable"}`))
	}

	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.Host = targetURL.Host

		path := c.Param("path")
		if path == "" {
			path = "/"
		}
		req.URL.Path = path
		req.URL.RawPath = path

		// Strip auth token from proxied request
		query := req.URL.Query()
		query.Del("token")
		query.Del("preview_token")
		req.URL.RawQuery = query.Encode()
		req.Header.Del("Accept-Encoding")
	}

	h.applyPreviewResponseHeaders(c.Writer.Header(), c.GetHeader("Origin"), false)
	proxy.ServeHTTP(c.Writer, c.Request)
}

func (h *PreviewHandler) resolveUserID(c *gin.Context) (uint, error) {
	if userID := c.GetUint("user_id"); userID != 0 {
		return userID, nil
	}
	token := h.extractToken(c)
	if token == "" {
		return 0, fmt.Errorf("authentication required")
	}
	if h.authService == nil {
		return 0, fmt.Errorf("auth service unavailable")
	}
	claims, err := h.authService.ValidateToken(token)
	if err != nil {
		return 0, err
	}
	c.Set("user_id", claims.UserID)
	c.Set("username", claims.Username)
	c.Set("email", claims.Email)
	c.Set("role", claims.Role)
	return claims.UserID, nil
}

func (h *PreviewHandler) extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	if token := c.Query("token"); token != "" {
		return token
	}
	return ""
}

func (h *PreviewHandler) extractPreviewToken(c *gin.Context) string {
	if token := c.Query("preview_token"); token != "" {
		return token
	}
	if token, err := c.Cookie("apex_preview_token"); err == nil && token != "" {
		return token
	}
	if token := c.Query("token"); token != "" {
		return token
	}
	return ""
}

func (h *PreviewHandler) resolvePreviewUserID(c *gin.Context, projectID uint) (uint, error) {
	if userID := c.GetUint("user_id"); userID != 0 {
		return userID, nil
	}
	if userID, err := h.resolveUserID(c); err == nil {
		return userID, nil
	}
	if h.authService == nil {
		return 0, fmt.Errorf("auth service unavailable")
	}
	previewToken := h.extractPreviewToken(c)
	if previewToken == "" {
		return 0, fmt.Errorf("authentication required")
	}
	claims, err := h.authService.ValidatePreviewToken(previewToken, projectID)
	if err != nil {
		return 0, err
	}
	return claims.UserID, nil
}

func (h *PreviewHandler) issuePreviewAccessToken(c *gin.Context, projectID uint) string {
	if projectID == 0 || h.authService == nil {
		return ""
	}
	if existing, ok := c.Get("preview_access_token"); ok {
		if token, ok := existing.(string); ok && token != "" {
			return token
		}
	}
	if previewToken := h.extractPreviewToken(c); previewToken != "" {
		if _, err := h.authService.ValidatePreviewToken(previewToken, projectID); err == nil {
			c.Set("preview_access_token", previewToken)
			return previewToken
		}
	}
	userID := c.GetUint("user_id")
	if userID == 0 {
		fullToken := h.extractToken(c)
		if fullToken == "" {
			return ""
		}
		claims, err := h.authService.ValidateToken(fullToken)
		if err != nil {
			return ""
		}
		userID = claims.UserID
	}
	token, err := h.authService.GeneratePreviewToken(userID, projectID, time.Hour)
	if err != nil {
		return ""
	}
	c.Set("preview_access_token", token)
	return token
}

func previewCookieSecure(c *gin.Context) bool {
	secure := c.Request.TLS != nil
	if forwardedProto := strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Proto"), ",")[0]); forwardedProto != "" {
		secure = strings.EqualFold(forwardedProto, "https")
	}
	return secure
}

func (h *PreviewHandler) setPreviewAccessCookie(c *gin.Context, projectID uint) {
	token := h.issuePreviewAccessToken(c, projectID)
	if token == "" {
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "apex_preview_token",
		Value:    token,
		Path:     "/api/v1/preview",
		MaxAge:   3600,
		HttpOnly: true,
		Secure:   previewCookieSecure(c),
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *PreviewHandler) handlePreviewCORS(c *gin.Context) bool {
	h.applyPreviewResponseHeaders(c.Writer.Header(), c.GetHeader("Origin"), false)
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusNoContent)
		return true
	}
	return false
}

func (h *PreviewHandler) applyPreviewResponseHeaders(headers http.Header, origin string, htmlDocument bool) {
	if strings.EqualFold(strings.TrimSpace(origin), "null") {
		headers.Set("Access-Control-Allow-Origin", "null")
		headers.Set("Access-Control-Allow-Credentials", "true")
		headers.Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		headers.Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Requested-With")
		headers.Add("Vary", "Origin")
	}
	if htmlDocument {
		headers.Set("Content-Security-Policy", "sandbox allow-same-origin allow-scripts allow-forms allow-popups allow-modals")
	}
}

func previewPublicBase(c *gin.Context) (string, string) {
	host := c.Request.Host
	if forwardedHost := c.GetHeader("X-Forwarded-Host"); forwardedHost != "" {
		host = strings.TrimSpace(strings.Split(forwardedHost, ",")[0])
	}

	scheme := "http"
	if forwardedProto := c.GetHeader("X-Forwarded-Proto"); forwardedProto != "" {
		scheme = strings.TrimSpace(strings.Split(forwardedProto, ",")[0])
	} else if c.Request.TLS != nil {
		scheme = "https"
	}

	return scheme, host
}

func (h *PreviewHandler) buildProxyURL(c *gin.Context, projectID uint) string {
	base := previewRootURL(h.buildProxyBaseURL(c, projectID))
	token := h.issuePreviewAccessToken(c, projectID)
	if token == "" {
		return base
	}
	return base + "?preview_token=" + url.QueryEscape(token)
}

func (h *PreviewHandler) buildProxyBaseURL(c *gin.Context, projectID uint) string {
	scheme, host := previewPublicBase(c)
	return fmt.Sprintf("%s://%s/api/v1/preview/proxy/%d", scheme, host, projectID)
}

// ProxyBackend proxies API calls from the preview frontend to the running backend process.
// This makes full-stack preview work: the frontend's fetch('http://localhost:3001/api/...')
// is rewritten by a client-side script to hit this proxy URL instead.
// GET/POST/etc /api/v1/preview/backend-proxy/:projectId/*path
func (h *PreviewHandler) ProxyBackend(c *gin.Context) {
	if !h.ensureBackendPreviewAvailable(c) {
		return
	}
	if h.handlePreviewCORS(c) {
		return
	}

	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	userID, resolveErr := h.resolvePreviewUserID(c, uint(projectID))
	if resolveErr != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": resolveErr.Error()})
		return
	}
	h.setPreviewAccessCookie(c, uint(projectID))

	var project models.Project
	if dbErr := h.db.First(&project, uint(projectID)).Error; dbErr != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}
	if project.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Look up the running backend server port for this project
	serverStatus := h.serverRunner.GetStatus(uint(projectID))
	if serverStatus == nil || !serverStatus.Running {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Backend server not running"})
		return
	}

	targetURL, parseErr := backendProxyTargetURL(serverStatus)
	if parseErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to build backend proxy"})
		return
	}

	// Strip the /backend-proxy/:projectId prefix — forward just the path
	c.Request.URL.Path = c.Param("path")
	if c.Request.URL.Path == "" {
		c.Request.URL.Path = "/"
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.FlushInterval = -1
	baseDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		baseDirector(req)
		disablePreviewProxyCompression(req)
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
		h.applyPreviewResponseHeaders(w.Header(), c.GetHeader("Origin"), false)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"Backend proxy unavailable"}`))
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		contentType := strings.ToLower(resp.Header.Get("Content-Type"))
		responsePath := ""
		if resp.Request != nil && resp.Request.URL != nil {
			responsePath = resp.Request.URL.Path
		}
		isHTML := strings.Contains(contentType, "text/html")
		isJavaScript := isPreviewJavaScriptResponse(contentType, responsePath)
		h.applyPreviewResponseHeaders(resp.Header, c.GetHeader("Origin"), isHTML)
		if !isHTML && !isJavaScript {
			return nil
		}

		originalBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return readErr
		}
		_ = resp.Body.Close()

		previewToken := h.issuePreviewAccessToken(c, uint(projectID))
		var rewritten string
		if isHTML {
			rewritten = h.rewritePreviewHTMLForProxyWithPrefix(
				string(originalBody),
				fmt.Sprintf("/api/v1/preview/backend-proxy/%d", projectID),
				h.buildBackendProxyURL(c, uint(projectID)),
				previewToken,
			)
		} else {
			rewritten = h.rewritePreviewJavaScriptForProxyWithPrefix(
				string(originalBody),
				h.buildBackendProxyBaseURL(c, uint(projectID)),
				previewToken,
			)
		}
		setRewrittenPreviewResponseBody(resp, rewritten)
		return nil
	}
	h.applyPreviewResponseHeaders(c.Writer.Header(), c.GetHeader("Origin"), false)
	proxy.ServeHTTP(c.Writer, c.Request)
}

func disablePreviewProxyCompression(req *http.Request) {
	if req == nil {
		return
	}
	req.Header.Del("Accept-Encoding")
}

func setRewrittenPreviewResponseBody(resp *http.Response, body string) {
	resp.Body = io.NopCloser(bytes.NewBufferString(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Del("Content-Encoding")
	resp.Header.Del("ETag")
	resp.Header.Del("Last-Modified")
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
}

// buildBackendProxyURL returns the public URL for the backend proxy for a given project.
func (h *PreviewHandler) buildBackendProxyURL(c *gin.Context, projectID uint) string {
	return h.buildBackendProxyBaseURL(c, projectID)
}

func (h *PreviewHandler) buildBackendProxyBaseURL(c *gin.Context, projectID uint) string {
	scheme, host := previewPublicBase(c)
	return fmt.Sprintf("%s://%s/api/v1/preview/backend-proxy/%d", scheme, host, projectID)
}

func (h *PreviewHandler) buildBackendProxyURLForBrowser(c *gin.Context, projectID uint) string {
	base := previewRootURL(h.buildBackendProxyBaseURL(c, projectID))
	token := h.issuePreviewAccessToken(c, projectID)
	if token == "" {
		return base
	}
	return base + "?preview_token=" + url.QueryEscape(token)
}

func previewRootURL(base string) string {
	return strings.TrimRight(base, "/") + "/"
}

func backendProxyTargetURL(serverStatus *preview.ServerStatus) (*url.URL, error) {
	if serverStatus == nil {
		return nil, fmt.Errorf("backend server not running")
	}
	target := strings.TrimSpace(serverStatus.URL)
	if target == "" {
		target = fmt.Sprintf("http://127.0.0.1:%d", serverStatus.Port)
	}
	return url.Parse(target)
}

func previewProxyTargetURL(status *preview.PreviewStatus) (*url.URL, error) {
	if status == nil || !status.Active {
		return nil, fmt.Errorf("preview not running")
	}
	target := strings.TrimSpace(status.URL)
	if target == "" {
		target = fmt.Sprintf("http://127.0.0.1:%d", status.Port)
	}
	return url.Parse(target)
}

func (h *PreviewHandler) rewritePreviewHTMLForProxy(html string, projectID uint) string {
	return h.rewritePreviewHTMLForProxyWithBackend(html, projectID, "", "")
}

func (h *PreviewHandler) rewritePreviewHTMLForProxyWithBackend(html string, projectID uint, backendProxyURL string, previewToken string) string {
	return h.rewritePreviewHTMLForProxyWithPrefix(html, fmt.Sprintf("/api/v1/preview/proxy/%d", projectID), backendProxyURL, previewToken)
}

func (h *PreviewHandler) rewritePreviewHTMLForProxyWithPrefix(html string, prefix string, backendProxyURL string, previewToken string) string {
	replaced := strings.NewReplacer(
		`src="/`, `src="`+prefix+`/`,
		`src='/`, `src='`+prefix+`/`,
		`href="/`, `href="`+prefix+`/`,
		`href='/`, `href='`+prefix+`/`,
		`action="/`, `action="`+prefix+`/`,
		`action='/`, `action='`+prefix+`/`,
		`url("/`, `url("`+prefix+`/`,
		`url('/`, `url('`+prefix+`/`,
		`url(/`, `url(`+prefix+`/`,
	).Replace(html)

	// Preserve protocol-relative external URLs (e.g. //cdn.example.com).
	replaced = strings.ReplaceAll(replaced, `src="`+prefix+`//`, `src="//`)
	replaced = strings.ReplaceAll(replaced, `src='`+prefix+`//`, `src='//`)
	replaced = strings.ReplaceAll(replaced, `href="`+prefix+`//`, `href="//`)
	replaced = strings.ReplaceAll(replaced, `href='`+prefix+`//`, `href='//`)
	replaced = strings.ReplaceAll(replaced, `action="`+prefix+`//`, `action="//`)
	replaced = strings.ReplaceAll(replaced, `action='`+prefix+`//`, `action='//`)
	replaced = strings.ReplaceAll(replaced, `url("`+prefix+`//`, `url("//`)
	replaced = strings.ReplaceAll(replaced, `url('`+prefix+`//`, `url('//`)
	replaced = strings.ReplaceAll(replaced, `url(`+prefix+`//`, `url(//`)
	replaced = appendPreviewTokenToProxyAssets(replaced, prefix, previewToken)

	// Inject a script that patches fetch() / XHR to route localhost API calls through
	// the backend proxy. This makes full-stack preview work without requiring the
	// generated app to know the apex.build preview URL.
	if backendProxyURL != "" || previewToken != "" {
		backendScript := fmt.Sprintf(`<script>
(function(){
  var _bp=%q;
  var _pt=%q;
  var _re=/http:\/\/localhost:(3001|8000|8080|3000|5000|4000)/g;
  var _px=%q;
  var _apiPrefixes=['/api','/graphql','/trpc','/socket.io','/ws'];
  function _appendToken(u){
    if(!_pt)return u;
    try{
      var _url=new URL(u, window.location.href);
      if(_url.searchParams.get('preview_token')!==_pt){
        _url.searchParams.set('preview_token', _pt);
      }
      return _url.toString();
    }catch(_e){
      return u;
    }
  }
  function _rewrite(u){
    if(typeof u!=='string') return u;
    var _next=u.replace(_re,_bp);
    if(_next===u && u.charAt(0)==='/'){
      for(var i=0;i<_apiPrefixes.length;i++){
        if(u.indexOf(_apiPrefixes[i])===0){
          _next=_bp+u;
          break;
        }
      }
    }
    if(_next.indexOf(_px)===0 || _next.indexOf(_bp)===0){
      _next=_appendToken(_next);
    }
    return _next;
  }
  var _of=window.fetch;
  window.fetch=function(u,o){
    if(typeof u==='string')u=_rewrite(u);
    else if(u instanceof Request){var _u=_rewrite(u.url);if(_u!==u.url)u=new Request(_u,u);}
    return _of.call(this,u,o);
  };
  var _ox=XMLHttpRequest.prototype.open;
  XMLHttpRequest.prototype.open=function(m,u){
    if(typeof u==='string')u=_rewrite(u);
    return _ox.apply(this,arguments);
  };
  window.__APEX_BACKEND_URL__=_bp;
  window.__APEX_IMPORT_META_ENV__={
    VITE_API_URL:_bp,
    VITE_API_BASE_URL:_bp,
    REACT_APP_API_URL:_bp,
    MODE:'development',
    DEV:true,
    PROD:false,
    BASE_URL:'/',
    SSR:false
  };
  if(!window.import)window.import={meta:{env:{}}};
  window.import.meta={env:window.__APEX_IMPORT_META_ENV__};
})();
</script>`, backendProxyURL, previewToken, prefix)
		// Inject before </head> or at top of <body>
		if idx := strings.Index(replaced, "</head>"); idx >= 0 {
			replaced = replaced[:idx] + backendScript + replaced[idx:]
		} else if idx := strings.Index(replaced, "<body"); idx >= 0 {
			end := strings.Index(replaced[idx:], ">")
			if end >= 0 {
				insertAt := idx + end + 1
				replaced = replaced[:insertAt] + backendScript + replaced[insertAt:]
			}
		} else {
			replaced = backendScript + replaced
		}
	}

	return replaced
}

func isPreviewJavaScriptResponse(contentType string, responsePath string) bool {
	lowerContentType := strings.ToLower(contentType)
	if strings.Contains(lowerContentType, "javascript") || strings.Contains(lowerContentType, "ecmascript") {
		return true
	}

	lowerPath := strings.ToLower(responsePath)
	return strings.HasSuffix(lowerPath, ".js") || strings.HasSuffix(lowerPath, ".mjs")
}

func (h *PreviewHandler) rewritePreviewJavaScriptForProxy(js string, projectID uint, previewToken string) string {
	prefix := fmt.Sprintf("/api/v1/preview/proxy/%d", projectID)
	return h.rewritePreviewJavaScriptForProxyWithPrefix(js, prefix, previewToken)
}

func (h *PreviewHandler) rewritePreviewJavaScriptForProxyWithPrefix(js string, prefix string, previewToken string) string {
	assetLiteralPattern := regexp.MustCompile(`(["'])((?:/?assets/|\./|/?_next/)[^"'\s)]+?\.(?:js|mjs|css|svg|png|jpe?g|webp|gif|woff2?|ttf|eot)(?:\?[^"'\s)]*)?(?:#[^"'\s)]*)?)(["'])`)
	nextBasePattern := regexp.MustCompile(`(["'])(/_next/)(["'])`)

	rewritten := assetLiteralPattern.ReplaceAllStringFunc(js, func(match string) string {
		parts := assetLiteralPattern.FindStringSubmatch(match)
		if len(parts) != 4 || parts[1] != parts[3] {
			return match
		}
		rewritten := rewritePreviewAssetTargetForProxy(parts[2], prefix, previewToken)
		return parts[1] + rewritten + parts[3]
	})
	rewritten = nextBasePattern.ReplaceAllStringFunc(rewritten, func(match string) string {
		parts := nextBasePattern.FindStringSubmatch(match)
		if len(parts) != 4 || parts[1] != parts[3] {
			return match
		}
		return parts[1] + strings.TrimRight(prefix, "/") + "/_next/" + parts[3]
	})
	return normalizeVitePreloadDependencyMapForProxy(rewritten, prefix)
}

func rewritePreviewAssetTargetForProxy(target string, prefix string, previewToken string) string {
	if target == "" || prefix == "" {
		return target
	}
	if strings.Contains(target, "://") || strings.HasPrefix(target, "//") || strings.HasPrefix(target, "data:") || strings.HasPrefix(target, "blob:") {
		return target
	}

	rewritten := target
	if strings.HasPrefix(target, "/_next/") || strings.HasPrefix(target, "_next/") {
		rewritten = prefix + "/" + strings.TrimPrefix(target, "/")
	} else if strings.HasPrefix(target, "/assets/") || strings.HasPrefix(target, "assets/") {
		rewritten = prefix + "/" + strings.TrimPrefix(target, "/")
	} else if strings.HasPrefix(target, "./") {
		rewritten = strings.TrimRight(prefix, "/") + "/assets/" + strings.TrimPrefix(target, "./")
	}
	return appendPreviewTokenToProxyTarget(rewritten, prefix, previewToken)
}

func appendPreviewTokenToProxyTarget(target string, prefix string, previewToken string) string {
	if previewToken == "" || prefix == "" || target == "" || strings.Contains(target, "preview_token=") || !strings.HasPrefix(target, prefix) {
		return target
	}

	base := target
	fragment := ""
	if idx := strings.Index(base, "#"); idx >= 0 {
		fragment = base[idx:]
		base = base[:idx]
	}

	separator := "?"
	if strings.Contains(base, "?") {
		separator = "&"
	}
	return base + separator + "preview_token=" + url.QueryEscape(previewToken) + fragment
}

func normalizeVitePreloadDependencyMapForProxy(js string, prefix string) string {
	parsedPrefix, err := url.Parse(prefix)
	if err != nil || parsedPrefix.Scheme == "" || parsedPrefix.Host == "" {
		return js
	}

	fullAssetPrefix := strings.TrimRight(prefix, "/") + "/assets/"
	preloadAssetPrefix := strings.TrimLeft(strings.TrimRight(parsedPrefix.Path, "/")+"/assets/", "/")
	if preloadAssetPrefix == "" || fullAssetPrefix == preloadAssetPrefix {
		return js
	}

	mapDepsPattern := regexp.MustCompile(`m\.f\|\|\(m\.f=\[([^\]]*)\]\)`)
	return mapDepsPattern.ReplaceAllStringFunc(js, func(match string) string {
		parts := mapDepsPattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		normalizedDeps := strings.ReplaceAll(parts[1], fullAssetPrefix, preloadAssetPrefix)
		return strings.Replace(match, parts[1], normalizedDeps, 1)
	})
}

func appendPreviewTokenToProxyAssets(html string, prefix string, previewToken string) string {
	if previewToken == "" || prefix == "" || html == "" {
		return html
	}

	appendToken := func(target string) string {
		return appendPreviewTokenToProxyTarget(target, prefix, previewToken)
	}

	attributePattern := regexp.MustCompile(`(?i)(\b(?:src|href|action)=["'])([^"']+)(["'])`)
	html = attributePattern.ReplaceAllStringFunc(html, func(match string) string {
		parts := attributePattern.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}
		return parts[1] + appendToken(parts[2]) + parts[3]
	})

	cssURLPattern := regexp.MustCompile(`(?i)(\burl\()(["']?)([^)"']+)(["']?\))`)
	html = cssURLPattern.ReplaceAllStringFunc(html, func(match string) string {
		parts := cssURLPattern.FindStringSubmatch(match)
		if len(parts) != 5 {
			return match
		}
		return parts[1] + parts[2] + appendToken(parts[3]) + parts[4]
	})

	return html
}

// GetDockerStatus returns Docker availability and container statistics
// GET /api/v1/preview/docker/status
func (h *PreviewHandler) GetDockerStatus(c *gin.Context) {
	if h.factory == nil {
		configuredHost := strings.TrimSpace(os.Getenv("APEX_PREVIEW_DOCKER_HOST"))
		if configuredHost == "" {
			configuredHost = strings.TrimSpace(os.Getenv("APEX_PREVIEW_DOCKER_SOCKET"))
		}
		configuredContext := strings.TrimSpace(os.Getenv("APEX_PREVIEW_DOCKER_CONTEXT"))
		if configuredContext == "" {
			configuredContext = strings.TrimSpace(os.Getenv("DOCKER_CONTEXT"))
		}

		c.JSON(http.StatusOK, gin.H{
			"success":                   true,
			"available":                 false,
			"message":                   "Docker container previews not configured",
			"diagnostic":                "preview container factory is disabled",
			"docker_host":               configuredHost,
			"docker_context":            configuredContext,
			"sandbox_required":          h.requireSandbox,
			"sandbox_degraded":          h.sandboxFallbackActive(),
			"backend_preview_available": h.backendPreviewAvailable(),
			"backend_preview_reason":    h.backendPreviewDisabledReason(),
			"backend_preview_runtime":   h.backendPreviewRuntime(),
		})
		return
	}

	status := h.factory.GetDockerStatus()

	c.JSON(http.StatusOK, gin.H{
		"success":                   true,
		"available":                 status.Available,
		"docker_host":               status.DockerHost,
		"docker_context":            status.DockerContext,
		"diagnostic":                status.Diagnostic,
		"container_count":           status.ContainerCount,
		"max_containers":            status.MaxContainers,
		"total_created":             status.TotalCreated,
		"failed_count":              status.FailedCount,
		"total_build_ms":            status.TotalBuildTime,
		"total_runtime_ms":          status.TotalRuntime,
		"sandbox_required":          h.requireSandbox,
		"sandbox_degraded":          h.sandboxFallbackActive(),
		"backend_preview_available": h.backendPreviewAvailable(),
		"backend_preview_reason":    h.backendPreviewDisabledReason(),
		"backend_preview_runtime":   h.backendPreviewRuntime(),
	})
}

func previewSandboxRequired() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("ENVIRONMENT")), "production") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("PREVIEW_FORCE_CONTAINER")), "true")
}

func (h *PreviewHandler) sandboxFallbackActive() bool {
	return h.requireSandbox && (h.factory == nil || !h.factory.IsDockerAvailable())
}

func (h *PreviewHandler) resolveRequestedPreviewSandbox(requested bool) (bool, error) {
	if requested {
		if h.factory == nil || !h.factory.IsDockerAvailable() {
			if h.sandboxFallbackActive() {
				return false, nil
			}
			return false, fmt.Errorf("secure preview mode requires Docker container previews, but Docker preview is not available")
		}
		return true, nil
	}
	if h.requireSandbox {
		if h.factory != nil && h.factory.IsDockerAvailable() {
			return true, nil
		}
		return false, nil
	}
	return false, nil
}

func (h *PreviewHandler) resolvePreviewOperationSandbox(projectID uint, requested bool) (bool, error) {
	if !requested && h.requireSandbox && h.factory != nil && h.factory.IsDockerAvailable() {
		if status, activeSandbox := h.getPreviewStatus(projectID, false); status != nil && status.Active && !activeSandbox {
			return false, nil
		}
	}
	return h.resolveRequestedPreviewSandbox(requested)
}

func (h *PreviewHandler) previewStatusSandboxDegraded(projectID uint, status *preview.PreviewStatus, activeSandbox bool) bool {
	if status == nil || !status.Active || activeSandbox {
		return false
	}
	if h.isFrameworkRuntimePreviewActive(projectID) {
		return false
	}
	if h.sandboxFallbackActive() {
		return true
	}
	return h.requireSandbox && h.factory != nil && h.factory.IsDockerAvailable()
}

func (h *PreviewHandler) backendPreviewAvailable() bool {
	if h.serverRunner == nil {
		return false
	}
	if !h.requireSandbox {
		return true
	}
	if h.backendPreviewRuntimeSecure() {
		return true
	}
	return h.sandboxFallbackActive()
}

func (h *PreviewHandler) backendPreviewDisabledReason() string {
	if h.serverRunner == nil {
		return "Backend runtime preview is not configured"
	}
	if !h.requireSandbox {
		return ""
	}
	if h.backendPreviewRuntimeSecure() {
		return ""
	}
	if h.requireSandbox && !h.sandboxFallbackActive() {
		return "Backend runtime preview is disabled while secure sandbox preview is enforced"
	}
	if h.sandboxFallbackActive() {
		return "Server Docker is unavailable, so preview is using process fallback mode"
	}
	return ""
}

func (h *PreviewHandler) backendPreviewRuntime() string {
	if h.serverRunner == nil {
		return ""
	}
	return h.serverRunner.RuntimeName()
}

func (h *PreviewHandler) backendPreviewRuntimeSecure() bool {
	switch strings.ToLower(strings.TrimSpace(h.backendPreviewRuntime())) {
	case "container", "sandbox-v2", "e2b":
		return true
	default:
		return false
	}
}

func isNextPreviewFramework(framework string) bool {
	switch strings.ToLower(strings.TrimSpace(framework)) {
	case "next", "nextjs", "next.js":
		return true
	default:
		return false
	}
}

func mergePreviewEnvVars(primary map[string]string, overlays ...map[string]string) map[string]string {
	if len(primary) == 0 && len(overlays) == 0 {
		return nil
	}
	merged := make(map[string]string, len(primary))
	for key, value := range primary {
		merged[key] = value
	}
	for _, overlay := range overlays {
		for key, value := range overlay {
			merged[key] = value
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func previewRuntimeStartTimeout() time.Duration {
	return previewDurationFromEnv("APEX_PREVIEW_RUNTIME_START_TIMEOUT_MS", 25*time.Second)
}

func previewOptionalBackendStartTimeout() time.Duration {
	return previewDurationFromEnv("APEX_PREVIEW_OPTIONAL_BACKEND_START_TIMEOUT_MS", 8*time.Second)
}

func previewFrontendStartTimeout() time.Duration {
	return previewDurationFromEnv("APEX_PREVIEW_FRONTEND_START_TIMEOUT_MS", 180*time.Second)
}

func previewDurationFromEnv(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	ms, err := strconv.Atoi(raw)
	if err != nil || ms <= 0 {
		return fallback
	}
	return time.Duration(ms) * time.Millisecond
}

func (h *PreviewHandler) startFrameworkRuntimePreview(c *gin.Context, projectID uint, envVars map[string]string, timeout time.Duration) (*preview.PreviewStatus, *preview.ServerStatus, error) {
	if !h.backendPreviewAvailable() {
		reason := strings.TrimSpace(h.backendPreviewDisabledReason())
		if reason == "" {
			reason = "Backend runtime preview is not available"
		}
		return nil, nil, errors.New(reason)
	}

	startCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	proc, err := h.serverRunner.Start(startCtx, &preview.ServerConfig{
		ProjectID:    projectID,
		EnvVars:      envVars,
		ReadyTimeout: timeout,
	})
	if err != nil {
		return nil, h.serverRunner.GetStatus(projectID), err
	}

	serverStatus := h.serverRunner.GetStatus(projectID)
	status := &preview.PreviewStatus{
		ProjectID:  projectID,
		Active:     true,
		Port:       proc.Port,
		URL:        h.buildPublicPreviewURL(c, projectID, true),
		StartedAt:  proc.StartedAt,
		LastAccess: time.Now(),
	}
	return status, serverStatus, nil
}

func (h *PreviewHandler) startFrontendPreviewFallback(c *gin.Context, projectID uint, entryPoint string, framework string, envVars map[string]string, requestedSandbox bool) (*preview.PreviewStatus, bool, error) {
	useSandbox, sandboxErr := h.resolveRequestedPreviewSandbox(requestedSandbox)
	if sandboxErr != nil {
		return nil, useSandbox, sandboxErr
	}

	config := &preview.PreviewConfig{
		ProjectID:  projectID,
		EntryPoint: entryPoint,
		Framework:  framework,
		EnvVars:    envVars,
	}

	status, actualSandbox, _, _, err := h.startFrontendPreviewWithFallback(c.Request.Context(), config, useSandbox)
	if err != nil {
		return nil, actualSandbox, err
	}
	status.URL = h.buildProxyURL(c, projectID)
	h.setPreviewAccessCookie(c, projectID)
	return status, actualSandbox, nil
}

func (h *PreviewHandler) startFrontendPreviewWithFallback(ctx context.Context, config *preview.PreviewConfig, requestedSandbox bool) (*preview.PreviewStatus, bool, bool, string, error) {
	useSandbox, sandboxErr := h.resolveRequestedPreviewSandbox(requestedSandbox)
	if sandboxErr != nil {
		return nil, useSandbox, false, "", sandboxErr
	}

	if h.factory != nil {
		startCtx := ctx
		var cancel context.CancelFunc
		if useSandbox {
			startCtx, cancel = context.WithTimeout(ctx, previewFrontendStartTimeout())
		}
		if cancel != nil {
			defer cancel()
		}

		status, err := h.factory.StartPreview(startCtx, config, useSandbox)
		if err == nil {
			return status, useSandbox, false, "", nil
		}
		if useSandbox && h.server != nil && (!h.requireSandbox || h.sandboxFallbackActive()) {
			fallbackReason := err.Error()
			if errors.Is(startCtx.Err(), context.DeadlineExceeded) {
				fallbackReason = fmt.Sprintf("sandbox preview start exceeded %s", previewFrontendStartTimeout())
			}
			fallbackStatus, fallbackErr := h.server.StartPreview(ctx, config)
			if fallbackErr == nil {
				return fallbackStatus, false, true, fallbackReason, nil
			}
			return nil, useSandbox, false, fallbackReason, fmt.Errorf("sandbox preview failed: %v; process fallback also failed: %w", err, fallbackErr)
		}
		if useSandbox {
			fallbackReason := err.Error()
			if errors.Is(startCtx.Err(), context.DeadlineExceeded) {
				fallbackReason = fmt.Sprintf("sandbox preview start exceeded %s", previewFrontendStartTimeout())
			}
			return nil, useSandbox, false, fallbackReason, fmt.Errorf("sandbox preview failed: %w", err)
		}
		return nil, useSandbox, false, "", err
	}

	if useSandbox {
		return nil, useSandbox, false, "", fmt.Errorf("sandbox preview is required but not configured")
	}
	if h.server == nil {
		return nil, false, false, "", fmt.Errorf("process preview server is not configured")
	}
	status, err := h.server.StartPreview(ctx, config)
	if err != nil {
		return nil, false, false, "", err
	}
	return status, false, false, "", nil
}

func (h *PreviewHandler) runtimePreviewStatus(projectID uint) *preview.PreviewStatus {
	if h == nil || h.serverRunner == nil || !isNextPreviewFramework(h.detectFramework(projectID)) {
		return nil
	}
	serverStatus := h.serverRunner.GetStatus(projectID)
	if serverStatus == nil || !serverStatus.Running {
		return nil
	}
	startedAt := serverStatus.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	return &preview.PreviewStatus{
		ProjectID:  projectID,
		Active:     true,
		Port:       serverStatus.Port,
		URL:        serverStatus.URL,
		StartedAt:  startedAt,
		LastAccess: time.Now(),
	}
}

func (h *PreviewHandler) isFrameworkRuntimePreviewActive(projectID uint) bool {
	if h == nil || h.serverRunner == nil || !isNextPreviewFramework(h.detectFramework(projectID)) {
		return false
	}
	status := h.serverRunner.GetStatus(projectID)
	return status != nil && status.Running
}

func (h *PreviewHandler) buildPublicPreviewURL(c *gin.Context, projectID uint, runtimePreview bool) string {
	if runtimePreview {
		return h.buildBackendProxyURLForBrowser(c, projectID)
	}
	return h.buildProxyURL(c, projectID)
}

func (h *PreviewHandler) getPreviewStatus(projectID uint, preferredSandbox bool) (*preview.PreviewStatus, bool) {
	if h.factory != nil {
		primary := h.factory.GetPreviewStatus(projectID, preferredSandbox)
		if primary != nil && primary.Active {
			return primary, preferredSandbox
		}
		// A sandbox start can fall back to the process preview path when the
		// container build fails. Always check the alternate runtime before
		// reporting "not running" so the UI can recover the degraded preview.
		fallback := h.factory.GetPreviewStatus(projectID, !preferredSandbox)
		if fallback != nil && fallback.Active {
			return fallback, !preferredSandbox
		}
		if runtimeStatus := h.runtimePreviewStatus(projectID); runtimeStatus != nil {
			return runtimeStatus, false
		}
		if primary != nil {
			return primary, preferredSandbox
		}
		return fallback, !preferredSandbox
	}
	status := h.server.GetPreviewStatus(projectID)
	if status != nil && status.Active {
		return status, false
	}
	if runtimeStatus := h.runtimePreviewStatus(projectID); runtimeStatus != nil {
		return runtimeStatus, false
	}
	return status, false
}

func (h *PreviewHandler) ensureBackendPreviewAvailable(c *gin.Context) bool {
	if h.backendPreviewAvailable() {
		return true
	}
	c.JSON(http.StatusConflict, gin.H{
		"success": false,
		"error":   h.backendPreviewDisabledReason(),
	})
	return false
}

// Helper methods

func (h *PreviewHandler) detectEntryPoint(projectID uint) string {
	// Check for common entry points
	entryPoints := []string{
		"app/page.tsx",
		"app/page.ts",
		"app/page.jsx",
		"app/page.js",
		"src/app/page.tsx",
		"src/app/page.ts",
		"src/app/page.jsx",
		"src/app/page.js",
		"pages/index.tsx",
		"pages/index.ts",
		"pages/index.jsx",
		"pages/index.js",
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

		// Prefer meta-frameworks before their underlying UI libraries.
		frameworks := []struct {
			name string
			deps []string
		}{
			{name: "next", deps: []string{"next"}},
			{name: "nuxt", deps: []string{"nuxt"}},
			{name: "react", deps: []string{"react", "react-dom"}},
			{name: "vue", deps: []string{"vue"}},
			{name: "svelte", deps: []string{"svelte"}},
			{name: "angular", deps: []string{"@angular/core"}},
		}

		for _, framework := range frameworks {
			for _, dep := range framework.deps {
				if contains(content, `"`+dep+`"`) {
					return framework.name
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
		Title:      strings.TrimSpace(project.Name),
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
		"app/page.tsx",
		"app/page.ts",
		"app/page.jsx",
		"app/page.js",
		"src/app/page.tsx",
		"src/app/page.ts",
		"src/app/page.jsx",
		"src/app/page.js",
		"pages/index.tsx",
		"pages/index.ts",
		"pages/index.jsx",
		"pages/index.js",
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

	return ""
}

// ========== BACKEND SERVER CONTROL ENDPOINTS ==========

// StartServer starts a backend server for a project
// POST /api/v1/preview/server/start
func (h *PreviewHandler) StartServer(c *gin.Context) {
	if !h.ensureBackendPreviewAvailable(c) {
		return
	}

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

	// Backend preview processes are long-lived and must not be tied to request cancellation.
	proc, err := h.serverRunner.Start(context.Background(), config)
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
	if !h.ensureBackendPreviewAvailable(c) {
		return
	}

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

	if !h.backendPreviewAvailable() {
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"available": false,
			"reason":    h.backendPreviewDisabledReason(),
			"server":    nil,
		})
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

	if !h.backendPreviewAvailable() {
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"available": false,
			"reason":    h.backendPreviewDisabledReason(),
			"stdout":    "",
			"stderr":    "",
		})
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
