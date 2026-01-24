// Package preview provides live app preview functionality
// This allows users to see and interact with their generated applications
// similar to Replit's preview feature.
package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// PreviewManager manages live app previews
type PreviewManager struct {
	previews    map[string]*Preview
	baseDir     string
	basePort    int
	mu          sync.RWMutex
}

// Preview represents a running app preview
type Preview struct {
	ID          string            `json:"id"`
	BuildID     string            `json:"build_id"`
	Status      PreviewStatus     `json:"status"`
	URL         string            `json:"url"`
	Port        int               `json:"port"`
	Files       map[string]string `json:"files"`
	AppType     AppType           `json:"app_type"`
	WorkDir     string            `json:"work_dir"`
	Process     *exec.Cmd         `json:"-"`
	Logs        []string          `json:"logs"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	mu          sync.RWMutex
}

// PreviewStatus represents the state of a preview
type PreviewStatus string

const (
	PreviewPending   PreviewStatus = "pending"
	PreviewBuilding  PreviewStatus = "building"
	PreviewRunning   PreviewStatus = "running"
	PreviewStopped   PreviewStatus = "stopped"
	PreviewError     PreviewStatus = "error"
)

// AppType identifies the type of application
type AppType string

const (
	AppTypeReact     AppType = "react"
	AppTypeVue       AppType = "vue"
	AppTypeNext      AppType = "nextjs"
	AppTypeNode      AppType = "node"
	AppTypeStatic    AppType = "static"
	AppTypePython    AppType = "python"
	AppTypeGo        AppType = "go"
)

// NewPreviewManager creates a new preview manager
func NewPreviewManager(baseDir string, basePort int) *PreviewManager {
	if baseDir == "" {
		baseDir = "/tmp/apex-previews"
	}
	if basePort == 0 {
		basePort = 9000
	}

	os.MkdirAll(baseDir, 0755)

	return &PreviewManager{
		previews: make(map[string]*Preview),
		baseDir:  baseDir,
		basePort: basePort,
	}
}

// CreatePreview creates a new preview from generated code
func (pm *PreviewManager) CreatePreview(buildID string, files map[string]string) (*Preview, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	id := uuid.New().String()
	port := pm.basePort + len(pm.previews)
	workDir := filepath.Join(pm.baseDir, id)

	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}

	now := time.Now()
	preview := &Preview{
		ID:        id,
		BuildID:   buildID,
		Status:    PreviewPending,
		Port:      port,
		Files:     files,
		WorkDir:   workDir,
		Logs:      make([]string, 0),
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Detect app type from files
	preview.AppType = pm.detectAppType(files)

	// Write files to disk
	if err := pm.writeFiles(preview); err != nil {
		return nil, fmt.Errorf("failed to write files: %w", err)
	}

	pm.previews[id] = preview

	log.Printf("Created preview %s for build %s (type: %s)", id, buildID, preview.AppType)
	return preview, nil
}

// detectAppType determines the application type from files
func (pm *PreviewManager) detectAppType(files map[string]string) AppType {
	for path := range files {
		if strings.Contains(path, "package.json") {
			// Check package.json content for framework
			if content, ok := files[path]; ok {
				if strings.Contains(content, "\"react\"") {
					return AppTypeReact
				}
				if strings.Contains(content, "\"vue\"") {
					return AppTypeVue
				}
				if strings.Contains(content, "\"next\"") {
					return AppTypeNext
				}
			}
			return AppTypeNode
		}
		if strings.HasSuffix(path, ".py") || path == "main.py" {
			return AppTypePython
		}
		if strings.HasSuffix(path, ".go") || path == "main.go" {
			return AppTypeGo
		}
	}

	// Check for HTML files (static site)
	for path := range files {
		if strings.HasSuffix(path, ".html") {
			return AppTypeStatic
		}
	}

	return AppTypeStatic
}

// writeFiles writes all files to the preview directory
func (pm *PreviewManager) writeFiles(preview *Preview) error {
	for path, content := range preview.Files {
		fullPath := filepath.Join(preview.WorkDir, path)
		dir := filepath.Dir(fullPath)

		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", path, err)
		}
	}
	return nil
}

// StartPreview starts the preview server
func (pm *PreviewManager) StartPreview(previewID string) error {
	pm.mu.RLock()
	preview, exists := pm.previews[previewID]
	pm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("preview %s not found", previewID)
	}

	preview.mu.Lock()
	preview.Status = PreviewBuilding
	preview.UpdatedAt = time.Now()
	preview.mu.Unlock()

	// Start the appropriate server based on app type
	var err error
	switch preview.AppType {
	case AppTypeReact, AppTypeVue, AppTypeNext, AppTypeNode:
		err = pm.startNodeApp(preview)
	case AppTypePython:
		err = pm.startPythonApp(preview)
	case AppTypeGo:
		err = pm.startGoApp(preview)
	case AppTypeStatic:
		err = pm.startStaticServer(preview)
	default:
		err = pm.startStaticServer(preview)
	}

	if err != nil {
		preview.mu.Lock()
		preview.Status = PreviewError
		preview.Logs = append(preview.Logs, fmt.Sprintf("Error: %v", err))
		preview.mu.Unlock()
		return err
	}

	preview.mu.Lock()
	preview.Status = PreviewRunning
	preview.URL = fmt.Sprintf("http://localhost:%d", preview.Port)
	preview.UpdatedAt = time.Now()
	preview.mu.Unlock()

	log.Printf("Preview %s started at %s", previewID, preview.URL)
	return nil
}

// startNodeApp starts a Node.js application
func (pm *PreviewManager) startNodeApp(preview *Preview) error {
	// Install dependencies
	installCmd := exec.Command("npm", "install")
	installCmd.Dir = preview.WorkDir
	if output, err := installCmd.CombinedOutput(); err != nil {
		preview.addLog(fmt.Sprintf("npm install output: %s", string(output)))
		// Continue anyway, might work without install
	}

	// Start dev server
	var cmd *exec.Cmd
	switch preview.AppType {
	case AppTypeNext:
		cmd = exec.Command("npx", "next", "dev", "-p", fmt.Sprintf("%d", preview.Port))
	case AppTypeVue:
		cmd = exec.Command("npx", "vite", "--port", fmt.Sprintf("%d", preview.Port), "--host")
	default:
		// Try vite first (React), fallback to npm start
		cmd = exec.Command("npx", "vite", "--port", fmt.Sprintf("%d", preview.Port), "--host")
	}

	cmd.Dir = preview.WorkDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", preview.Port))

	// Capture output
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start node app: %w", err)
	}

	preview.Process = cmd

	// Log output in background
	go pm.captureOutput(preview, stdout)
	go pm.captureOutput(preview, stderr)

	return nil
}

// startPythonApp starts a Python application
func (pm *PreviewManager) startPythonApp(preview *Preview) error {
	// Check for requirements.txt
	reqPath := filepath.Join(preview.WorkDir, "requirements.txt")
	if _, err := os.Stat(reqPath); err == nil {
		installCmd := exec.Command("pip", "install", "-r", "requirements.txt")
		installCmd.Dir = preview.WorkDir
		installCmd.CombinedOutput()
	}

	// Start Python app
	cmd := exec.Command("python", "main.py")
	cmd.Dir = preview.WorkDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", preview.Port))

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start python app: %w", err)
	}

	preview.Process = cmd

	go pm.captureOutput(preview, stdout)
	go pm.captureOutput(preview, stderr)

	return nil
}

// startGoApp starts a Go application
func (pm *PreviewManager) startGoApp(preview *Preview) error {
	// Build the Go app
	buildCmd := exec.Command("go", "build", "-o", "app", ".")
	buildCmd.Dir = preview.WorkDir
	if output, err := buildCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to build go app: %s", string(output))
	}

	// Run the app
	cmd := exec.Command("./app")
	cmd.Dir = preview.WorkDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", preview.Port))

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start go app: %w", err)
	}

	preview.Process = cmd

	go pm.captureOutput(preview, stdout)
	go pm.captureOutput(preview, stderr)

	return nil
}

// startStaticServer starts a simple static file server
func (pm *PreviewManager) startStaticServer(preview *Preview) error {
	// Create a simple HTTP server for static files
	mux := http.NewServeMux()
	
	// Serve static files
	fs := http.FileServer(http.Dir(preview.WorkDir))
	mux.Handle("/", fs)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", preview.Port),
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			preview.addLog(fmt.Sprintf("Static server error: %v", err))
		}
	}()

	preview.addLog(fmt.Sprintf("Static server started on port %d", preview.Port))
	return nil
}

// captureOutput captures process output to logs
func (pm *PreviewManager) captureOutput(preview *Preview, reader io.Reader) {
	buf := make([]byte, 1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			preview.addLog(string(buf[:n]))
		}
		if err != nil {
			break
		}
	}
}

// addLog adds a log entry to the preview
func (p *Preview) addLog(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Logs = append(p.Logs, msg)
	if len(p.Logs) > 1000 {
		p.Logs = p.Logs[len(p.Logs)-1000:]
	}
}

// StopPreview stops a running preview
func (pm *PreviewManager) StopPreview(previewID string) error {
	pm.mu.RLock()
	preview, exists := pm.previews[previewID]
	pm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("preview %s not found", previewID)
	}

	preview.mu.Lock()
	defer preview.mu.Unlock()

	if preview.Process != nil {
		if err := preview.Process.Process.Kill(); err != nil {
			return fmt.Errorf("failed to stop preview: %w", err)
		}
	}

	preview.Status = PreviewStopped
	preview.UpdatedAt = time.Now()

	log.Printf("Preview %s stopped", previewID)
	return nil
}

// GetPreview returns a preview by ID
func (pm *PreviewManager) GetPreview(previewID string) (*Preview, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	preview, exists := pm.previews[previewID]
	if !exists {
		return nil, fmt.Errorf("preview %s not found", previewID)
	}
	return preview, nil
}

// GetPreviewByBuild returns a preview by build ID
func (pm *PreviewManager) GetPreviewByBuild(buildID string) (*Preview, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, preview := range pm.previews {
		if preview.BuildID == buildID {
			return preview, nil
		}
	}
	return nil, fmt.Errorf("no preview found for build %s", buildID)
}

// UpdateFiles updates files in a preview and hot-reloads
func (pm *PreviewManager) UpdateFiles(previewID string, files map[string]string) error {
	pm.mu.RLock()
	preview, exists := pm.previews[previewID]
	pm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("preview %s not found", previewID)
	}

	preview.mu.Lock()
	for path, content := range files {
		preview.Files[path] = content
	}
	preview.UpdatedAt = time.Now()
	preview.mu.Unlock()

	// Write updated files
	if err := pm.writeFiles(preview); err != nil {
		return err
	}

	preview.addLog("Files updated - hot reload triggered")
	return nil
}

// GetLogs returns the logs for a preview
func (pm *PreviewManager) GetLogs(previewID string) ([]string, error) {
	pm.mu.RLock()
	preview, exists := pm.previews[previewID]
	pm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("preview %s not found", previewID)
	}

	preview.mu.RLock()
	defer preview.mu.RUnlock()

	logs := make([]string, len(preview.Logs))
	copy(logs, preview.Logs)
	return logs, nil
}

// CleanupOldPreviews removes previews older than the specified duration
func (pm *PreviewManager) CleanupOldPreviews(maxAge time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, preview := range pm.previews {
		if preview.CreatedAt.Before(cutoff) {
			if preview.Process != nil {
				preview.Process.Process.Kill()
			}
			os.RemoveAll(preview.WorkDir)
			delete(pm.previews, id)
			log.Printf("Cleaned up old preview %s", id)
		}
	}
}

// PreviewHandler returns an HTTP handler for serving preview content
type PreviewHandler struct {
	manager *PreviewManager
}

// NewPreviewHandler creates a new preview HTTP handler
func NewPreviewHandler(manager *PreviewManager) *PreviewHandler {
	return &PreviewHandler{manager: manager}
}

// ServeHTTP handles preview requests
func (h *PreviewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract preview ID from path: /preview/{id}/...
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/preview/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Preview ID required", http.StatusBadRequest)
		return
	}

	previewID := parts[0]
	preview, err := h.manager.GetPreview(previewID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// If preview is running, proxy to the preview server
	if preview.Status == PreviewRunning && preview.URL != "" {
		h.proxyToPreview(w, r, preview)
		return
	}

	// Otherwise, serve the preview status page
	h.serveStatusPage(w, preview)
}

// proxyToPreview proxies requests to the running preview
func (h *PreviewHandler) proxyToPreview(w http.ResponseWriter, r *http.Request, preview *Preview) {
	// Simple proxy implementation
	targetURL := preview.URL + r.URL.Path
	
	resp, err := http.Get(targetURL)
	if err != nil {
		http.Error(w, "Preview not responding", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// serveStatusPage shows the preview status
func (h *PreviewHandler) serveStatusPage(w http.ResponseWriter, preview *Preview) {
	w.Header().Set("Content-Type", "text/html")
	
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>APEX.BUILD Preview</title>
    <style>
        body {
            font-family: 'Monaco', 'Menlo', monospace;
            background: linear-gradient(135deg, #0a0a1a 0%, #1a1a3a 100%);
            color: #00ffff;
            min-height: 100vh;
            margin: 0;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container {
            background: rgba(0, 255, 255, 0.1);
            border: 1px solid #00ffff;
            border-radius: 8px;
            padding: 40px;
            text-align: center;
            max-width: 500px;
        }
        h1 { color: #00ffff; margin-bottom: 20px; }
        .status { 
            font-size: 24px; 
            padding: 10px 20px;
            border-radius: 4px;
            margin: 20px 0;
        }
        .pending { background: rgba(255, 165, 0, 0.2); color: #ffa500; }
        .building { background: rgba(0, 255, 255, 0.2); color: #00ffff; }
        .running { background: rgba(0, 255, 0, 0.2); color: #00ff00; }
        .error { background: rgba(255, 0, 0, 0.2); color: #ff0000; }
        .logs {
            background: #000;
            border: 1px solid #333;
            border-radius: 4px;
            padding: 15px;
            text-align: left;
            max-height: 200px;
            overflow-y: auto;
            font-size: 12px;
            color: #0f0;
        }
        .spinner {
            border: 3px solid #333;
            border-top: 3px solid #00ffff;
            border-radius: 50%;
            width: 40px;
            height: 40px;
            animation: spin 1s linear infinite;
            margin: 20px auto;
        }
        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
    </style>
    <script>
        setTimeout(() => location.reload(), 3000);
    </script>
</head>
<body>
    <div class="container">
        <h1>APEX.BUILD Preview</h1>
        <div class="status {{.StatusClass}}">{{.Status}}</div>
        {{if eq .Status "building"}}
        <div class="spinner"></div>
        <p>Building your application...</p>
        {{end}}
        {{if eq .Status "pending"}}
        <div class="spinner"></div>
        <p>Preparing preview environment...</p>
        {{end}}
        {{if .Logs}}
        <div class="logs">
            {{range .Logs}}
            <div>{{.}}</div>
            {{end}}
        </div>
        {{end}}
    </div>
</body>
</html>`

	preview.mu.RLock()
	data := struct {
		Status      string
		StatusClass string
		Logs        []string
	}{
		Status:      string(preview.Status),
		StatusClass: string(preview.Status),
		Logs:        preview.Logs,
	}
	preview.mu.RUnlock()

	t, _ := template.New("status").Parse(tmpl)
	t.Execute(w, data)
}

// PreviewAPIHandler handles API requests for previews
type PreviewAPIHandler struct {
	manager *PreviewManager
}

// NewPreviewAPIHandler creates a new preview API handler
func NewPreviewAPIHandler(manager *PreviewManager) *PreviewAPIHandler {
	return &PreviewAPIHandler{manager: manager}
}

// HandleCreate handles POST /api/v1/preview
func (h *PreviewAPIHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BuildID string            `json:"build_id"`
		Files   map[string]string `json:"files"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	preview, err := h.manager.CreatePreview(req.BuildID, req.Files)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Start the preview
	go func() {
		if err := h.manager.StartPreview(preview.ID); err != nil {
			log.Printf("Failed to start preview %s: %v", preview.ID, err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"preview_id":  preview.ID,
		"preview_url": fmt.Sprintf("/preview/%s", preview.ID),
		"status":      string(preview.Status),
	})
}

// HandleGet handles GET /api/v1/preview/:id
func (h *PreviewAPIHandler) HandleGet(w http.ResponseWriter, r *http.Request, previewID string) {
	preview, err := h.manager.GetPreview(previewID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	preview.mu.RLock()
	defer preview.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":         preview.ID,
		"build_id":   preview.BuildID,
		"status":     string(preview.Status),
		"url":        preview.URL,
		"app_type":   string(preview.AppType),
		"created_at": preview.CreatedAt,
		"updated_at": preview.UpdatedAt,
	})
}

// HandleLogs handles GET /api/v1/preview/:id/logs
func (h *PreviewAPIHandler) HandleLogs(w http.ResponseWriter, r *http.Request, previewID string) {
	logs, err := h.manager.GetLogs(previewID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"logs": logs,
	})
}

// HandleStop handles POST /api/v1/preview/:id/stop
func (h *PreviewAPIHandler) HandleStop(w http.ResponseWriter, r *http.Request, previewID string) {
	if err := h.manager.StopPreview(previewID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "stopped",
	})
}

// StartCleanupRoutine starts a background routine to clean up old previews
func (pm *PreviewManager) StartCleanupRoutine(ctx context.Context, interval, maxAge time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pm.CleanupOldPreviews(maxAge)
		}
	}
}
