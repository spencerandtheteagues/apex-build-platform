// Package handlers - Live Preview HTTP Handlers for APEX.BUILD
package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

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
}

// NewPreviewHandler creates a new preview handler
func NewPreviewHandler(db *gorm.DB, server *preview.PreviewServer, authService *auth.AuthService) *PreviewHandler {
	return &PreviewHandler{
		db:             db,
		server:         server,
		serverRunner:   preview.NewServerRunner(db),
		bundlerService: bundler.NewService(db),
		authService:    authService,
	}
}

// NewPreviewHandlerWithFactory creates a preview handler with Docker sandbox support
func NewPreviewHandlerWithFactory(db *gorm.DB, factory *preview.PreviewServerFactory, authService *auth.AuthService) *PreviewHandler {
	return &PreviewHandler{
		db:             db,
		server:         factory.GetProcessServer(),
		factory:        factory,
		serverRunner:   preview.NewServerRunner(db),
		bundlerService: bundler.NewService(db),
		authService:    authService,
	}
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
		metrics.RecordPreviewStart("frontend", "error", req.Sandbox)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Override URL to use proxy so it's accessible from the browser
	status.URL = h.buildProxyURL(c, req.ProjectID)

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

	metrics.RecordPreviewStart("frontend", "success", req.Sandbox)
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

	previewConfig := &preview.PreviewConfig{
		ProjectID:  req.ProjectID,
		EntryPoint: req.EntryPoint,
		Framework:  req.Framework,
		EnvVars:    req.EnvVars,
	}

	var previewStatus *preview.PreviewStatus
	var err error
	if h.factory != nil {
		previewStatus, err = h.factory.StartPreview(c.Request.Context(), previewConfig, req.Sandbox)
	} else {
		previewStatus, err = h.server.StartPreview(c.Request.Context(), previewConfig)
	}
	if err != nil {
		metrics.RecordPreviewStart("fullstack", "error", req.Sandbox)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	previewStatus.URL = h.buildProxyURL(c, req.ProjectID)

	startBackend := true
	if req.StartBackend != nil {
		startBackend = *req.StartBackend
	}

	var serverStatus *preview.ServerStatus
	diagnostics := gin.H{
		"preview_started":   true,
		"backend_requested": startBackend,
	}
	degraded := false

	if startBackend {
		serverConfig := &preview.ServerConfig{
			ProjectID: req.ProjectID,
			EntryFile: req.BackendEntry,
			Command:   req.BackendCommand,
			EnvVars:   req.BackendEnvVars,
		}
		proc, startErr := h.serverRunner.Start(context.Background(), serverConfig)
		if startErr != nil {
			degraded = true
			diagnostics["backend_started"] = false
			diagnostics["backend_error"] = startErr.Error()
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

	resp := gin.H{
		"success":     true,
		"preview":     previewStatus,
		"server":      serverStatus,
		"proxy_url":   previewStatus.URL,
		"degraded":    degraded,
		"diagnostics": diagnostics,
		"message":     "Full-stack preview started",
		"sandbox":     req.Sandbox,
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
	if h.factory != nil {
		useSandbox := c.Query("sandbox") == "true" || c.Query("sandbox") == "1"
		status = h.getPreviewStatus(uint(projectID), useSandbox)
	}

	// Override URL to proxy
	if status != nil && status.Active {
		status.URL = h.buildProxyURL(c, uint(projectID))
	}

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

	useSandbox := req.Sandbox
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
	if h.factory != nil {
		useSandbox := c.Query("sandbox") == "true" || c.Query("sandbox") == "1"
		status = h.getPreviewStatus(uint(projectID), useSandbox)
	}

	if !status.Active {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Preview not running",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"url":        h.buildProxyURL(c, uint(projectID)),
		"port":       status.Port,
		"started_at": status.StartedAt,
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

	userID, err := h.resolveUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	token := h.extractToken(c)
	if token != "" {
		secure := c.Request.TLS != nil
		if forwardedProto := strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Proto"), ",")[0]); forwardedProto != "" {
			secure = strings.EqualFold(forwardedProto, "https")
		}
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     "apex_preview_token",
			Value:    token,
			Path:     fmt.Sprintf("/api/v1/preview/proxy/%d", uint(projectID)),
			MaxAge:   3600,
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
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

	useSandbox := c.Query("sandbox") == "true" || c.Query("sandbox") == "1"
	status := h.getPreviewStatus(uint(projectID), useSandbox)
	if status == nil || !status.Active {
		c.JSON(http.StatusNotFound, gin.H{"error": "Preview not running"})
		return
	}

	targetURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", status.Port))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to build preview proxy"})
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.FlushInterval = -1
	proxy.ModifyResponse = func(resp *http.Response) error {
		contentType := strings.ToLower(resp.Header.Get("Content-Type"))
		if !strings.Contains(contentType, "text/html") {
			return nil
		}

		originalBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return readErr
		}
		_ = resp.Body.Close()

		rewritten := h.rewritePreviewHTMLForProxy(string(originalBody), uint(projectID))
		resp.Body = io.NopCloser(bytes.NewBufferString(rewritten))
		resp.ContentLength = int64(len(rewritten))
		resp.Header.Set("Content-Length", strconv.Itoa(len(rewritten)))
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
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
		req.URL.RawQuery = query.Encode()
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}

func (h *PreviewHandler) getPreviewStatus(projectID uint, useSandbox bool) *preview.PreviewStatus {
	if h.factory != nil {
		status := h.factory.GetPreviewStatus(projectID, useSandbox)
		if status.Active {
			return status
		}
		if useSandbox {
			return h.factory.GetPreviewStatus(projectID, false)
		}
		return status
	}
	return h.server.GetPreviewStatus(projectID)
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
	if token, err := c.Cookie("apex_preview_token"); err == nil && token != "" {
		return token
	}
	return ""
}

func (h *PreviewHandler) buildProxyURL(c *gin.Context, projectID uint) string {
	host := c.Request.Host
	if forwardedHost := c.GetHeader("X-Forwarded-Host"); forwardedHost != "" {
		host = strings.Split(forwardedHost, ",")[0]
	}

	scheme := "http"
	if forwardedProto := c.GetHeader("X-Forwarded-Proto"); forwardedProto != "" {
		scheme = strings.Split(forwardedProto, ",")[0]
	} else if c.Request.TLS != nil {
		scheme = "https"
	}

	base := fmt.Sprintf("%s://%s/api/v1/preview/proxy/%d", scheme, host, projectID)
	token := h.extractToken(c)
	if token == "" {
		return base
	}
	return base + "?token=" + url.QueryEscape(token)
}

func (h *PreviewHandler) rewritePreviewHTMLForProxy(html string, projectID uint) string {
	prefix := fmt.Sprintf("/api/v1/preview/proxy/%d", projectID)
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

	return replaced
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
			"react":   {"react", "react-dom"},
			"vue":     {"vue"},
			"svelte":  {"svelte"},
			"next":    {"next"},
			"nuxt":    {"nuxt"},
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

	return ""
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
