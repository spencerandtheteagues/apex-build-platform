// Package preview - Live Preview Server for APEX.BUILD
// Provides real-time preview of web applications with hot reload support
package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"apex-build/pkg/models"

	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

// PreviewServer manages live preview sessions for projects
type PreviewServer struct {
	db        *gorm.DB
	sessions  map[uint]*PreviewSession
	mu        sync.RWMutex
	upgrader  websocket.Upgrader
	basePort  int
	portMap   map[uint]int // projectID -> assigned port
	portMu    sync.Mutex
}

// PreviewSession represents an active preview session
type PreviewSession struct {
	ProjectID    uint
	Port         int
	Clients      map[*websocket.Conn]bool
	FileCache    map[string]*CachedFile
	StartedAt    time.Time
	LastAccess   time.Time
	mu           sync.RWMutex
	server       *http.Server
	stopChan     chan struct{}
}

// CachedFile stores processed file content for preview
type CachedFile struct {
	Content     string
	ContentType string
	ProcessedAt time.Time
	Size        int64
}

// PreviewConfig contains configuration for a preview session
type PreviewConfig struct {
	ProjectID    uint   `json:"project_id"`
	EntryPoint   string `json:"entry_point"`   // e.g., "index.html" or "src/index.tsx"
	Framework    string `json:"framework"`     // react, vue, vanilla, etc.
	BuildCommand string `json:"build_command"` // Optional build step
	EnvVars      map[string]string `json:"env_vars"`
}

// PreviewStatus represents the current state of a preview
type PreviewStatus struct {
	ProjectID  uint      `json:"project_id"`
	Active     bool      `json:"active"`
	Port       int       `json:"port"`
	URL        string    `json:"url"`
	StartedAt  time.Time `json:"started_at"`
	LastAccess time.Time `json:"last_access"`
	Clients    int       `json:"connected_clients"`
}

// NewPreviewServer creates a new preview server manager
func NewPreviewServer(db *gorm.DB) *PreviewServer {
	return &PreviewServer{
		db:       db,
		sessions: make(map[uint]*PreviewSession),
		portMap:  make(map[uint]int),
		basePort: 9000, // Preview ports start at 9000
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for preview
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

// StartPreview starts a preview session for a project
func (ps *PreviewServer) StartPreview(ctx context.Context, config *PreviewConfig) (*PreviewStatus, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Check if session already exists
	if session, exists := ps.sessions[config.ProjectID]; exists {
		session.mu.Lock()
		session.LastAccess = time.Now()
		session.mu.Unlock()
		return ps.getStatus(session), nil
	}

	// Assign a port
	port := ps.assignPort(config.ProjectID)

	// Create new session
	session := &PreviewSession{
		ProjectID:  config.ProjectID,
		Port:       port,
		Clients:    make(map[*websocket.Conn]bool),
		FileCache:  make(map[string]*CachedFile),
		StartedAt:  time.Now(),
		LastAccess: time.Now(),
		stopChan:   make(chan struct{}),
	}

	// Load project files into cache
	if err := ps.loadProjectFiles(ctx, session, config); err != nil {
		return nil, fmt.Errorf("failed to load project files: %w", err)
	}

	// Start HTTP server for preview
	mux := http.NewServeMux()
	mux.HandleFunc("/", ps.createFileHandler(session, config))
	mux.HandleFunc("/__apex_reload", ps.createReloadHandler(session))
	mux.HandleFunc("/__apex_ws", ps.createWebSocketHandler(session))

	session.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Start server in background
	go func() {
		if err := session.server.ListenAndServe(); err != http.ErrServerClosed {
			// Log error but don't crash
			fmt.Printf("Preview server for project %d stopped: %v\n", config.ProjectID, err)
		}
	}()

	ps.sessions[config.ProjectID] = session

	return ps.getStatus(session), nil
}

// StopPreview stops a preview session
func (ps *PreviewServer) StopPreview(ctx context.Context, projectID uint) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	session, exists := ps.sessions[projectID]
	if !exists {
		return nil // Already stopped
	}

	// Close all websocket connections
	session.mu.Lock()
	for client := range session.Clients {
		client.Close()
	}
	session.mu.Unlock()

	// Stop HTTP server
	close(session.stopChan)
	if session.server != nil {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		session.server.Shutdown(ctx)
	}

	// Release port
	ps.releasePort(projectID)

	delete(ps.sessions, projectID)
	return nil
}

// GetPreviewStatus returns the status of a preview session
func (ps *PreviewServer) GetPreviewStatus(projectID uint) *PreviewStatus {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	session, exists := ps.sessions[projectID]
	if !exists {
		return &PreviewStatus{
			ProjectID: projectID,
			Active:    false,
		}
	}

	return ps.getStatus(session)
}

// RefreshPreview notifies all clients to reload
func (ps *PreviewServer) RefreshPreview(projectID uint, changedFiles []string) error {
	ps.mu.RLock()
	session, exists := ps.sessions[projectID]
	ps.mu.RUnlock()

	if !exists {
		return nil // No active preview
	}

	// Update file cache
	ctx := context.Background()
	for _, path := range changedFiles {
		ps.updateFileCache(ctx, session, path)
	}

	// Notify all connected clients
	session.mu.RLock()
	defer session.mu.RUnlock()

	message := map[string]interface{}{
		"type":  "reload",
		"files": changedFiles,
	}
	msgBytes, _ := json.Marshal(message)

	for client := range session.Clients {
		client.WriteMessage(websocket.TextMessage, msgBytes)
	}

	return nil
}

// HotReload sends a hot reload message for specific file changes
func (ps *PreviewServer) HotReload(projectID uint, filePath string, content string) error {
	ps.mu.RLock()
	session, exists := ps.sessions[projectID]
	ps.mu.RUnlock()

	if !exists {
		return nil
	}

	// Update cache
	session.mu.Lock()
	session.FileCache[filePath] = &CachedFile{
		Content:     content,
		ContentType: ps.getContentType(filePath),
		ProcessedAt: time.Now(),
		Size:        int64(len(content)),
	}
	session.mu.Unlock()

	// Determine if hot reload is possible (CSS changes) or full reload needed
	ext := filepath.Ext(filePath)
	reloadType := "full"
	if ext == ".css" {
		reloadType = "css"
	} else if ext == ".json" && strings.Contains(filePath, "i18n") {
		reloadType = "json"
	}

	// Notify clients
	session.mu.RLock()
	defer session.mu.RUnlock()

	message := map[string]interface{}{
		"type":    "hot-reload",
		"reload":  reloadType,
		"file":    filePath,
		"content": content,
	}
	msgBytes, _ := json.Marshal(message)

	for client := range session.Clients {
		client.WriteMessage(websocket.TextMessage, msgBytes)
	}

	return nil
}

// Helper methods

func (ps *PreviewServer) assignPort(projectID uint) int {
	ps.portMu.Lock()
	defer ps.portMu.Unlock()

	// Check if already assigned
	if port, exists := ps.portMap[projectID]; exists {
		return port
	}

	// Find next available port
	port := ps.basePort
	usedPorts := make(map[int]bool)
	for _, p := range ps.portMap {
		usedPorts[p] = true
	}

	for usedPorts[port] {
		port++
	}

	ps.portMap[projectID] = port
	return port
}

func (ps *PreviewServer) releasePort(projectID uint) {
	ps.portMu.Lock()
	defer ps.portMu.Unlock()
	delete(ps.portMap, projectID)
}

func (ps *PreviewServer) getStatus(session *PreviewSession) *PreviewStatus {
	session.mu.RLock()
	defer session.mu.RUnlock()

	return &PreviewStatus{
		ProjectID:  session.ProjectID,
		Active:     true,
		Port:       session.Port,
		URL:        fmt.Sprintf("http://localhost:%d", session.Port),
		StartedAt:  session.StartedAt,
		LastAccess: session.LastAccess,
		Clients:    len(session.Clients),
	}
}

func (ps *PreviewServer) loadProjectFiles(ctx context.Context, session *PreviewSession, config *PreviewConfig) error {
	var files []models.File
	if err := ps.db.WithContext(ctx).Where("project_id = ?", config.ProjectID).Find(&files).Error; err != nil {
		return err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	for _, file := range files {
		processed := ps.processFile(&file, config)
		session.FileCache[file.Path] = &CachedFile{
			Content:     processed,
			ContentType: ps.getContentType(file.Path),
			ProcessedAt: time.Now(),
			Size:        int64(len(processed)),
		}
	}

	return nil
}

func (ps *PreviewServer) updateFileCache(ctx context.Context, session *PreviewSession, path string) error {
	var file models.File
	if err := ps.db.WithContext(ctx).
		Where("project_id = ? AND path = ?", session.ProjectID, path).
		First(&file).Error; err != nil {
		return err
	}

	session.mu.Lock()
	session.FileCache[path] = &CachedFile{
		Content:     file.Content,
		ContentType: ps.getContentType(path),
		ProcessedAt: time.Now(),
		Size:        file.Size,
	}
	session.mu.Unlock()

	return nil
}

func (ps *PreviewServer) processFile(file *models.File, config *PreviewConfig) string {
	content := file.Content

	// Inject hot reload script into HTML files
	if strings.HasSuffix(file.Path, ".html") {
		content = ps.injectHotReloadScript(content, config)
	}

	// Process environment variables
	if config.EnvVars != nil {
		for key, value := range config.EnvVars {
			content = strings.ReplaceAll(content, fmt.Sprintf("${%s}", key), value)
			content = strings.ReplaceAll(content, fmt.Sprintf("process.env.%s", key), fmt.Sprintf("'%s'", value))
		}
	}

	return content
}

func (ps *PreviewServer) injectHotReloadScript(html string, config *PreviewConfig) string {
	script := `
<script>
(function() {
  const ws = new WebSocket('ws://' + window.location.host + '/__apex_ws');

  ws.onmessage = function(event) {
    const data = JSON.parse(event.data);

    if (data.type === 'reload') {
      console.log('[APEX] Reloading...');
      window.location.reload();
    } else if (data.type === 'hot-reload') {
      if (data.reload === 'css') {
        // Hot reload CSS
        const links = document.querySelectorAll('link[rel="stylesheet"]');
        links.forEach(link => {
          const href = link.getAttribute('href');
          if (href && href.includes(data.file)) {
            link.href = href.split('?')[0] + '?t=' + Date.now();
          }
        });
        console.log('[APEX] CSS hot reloaded:', data.file);
      } else {
        console.log('[APEX] Full reload required for:', data.file);
        window.location.reload();
      }
    }
  };

  ws.onclose = function() {
    console.log('[APEX] Connection closed, attempting reconnect...');
    setTimeout(() => window.location.reload(), 1000);
  };

  ws.onerror = function(error) {
    console.error('[APEX] WebSocket error:', error);
  };

  console.log('[APEX] Hot reload connected');
})();
</script>
`
	// Inject before </body> or at end
	if strings.Contains(html, "</body>") {
		return strings.Replace(html, "</body>", script+"</body>", 1)
	}
	return html + script
}

func (ps *PreviewServer) createFileHandler(session *PreviewSession, config *PreviewConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session.mu.Lock()
		session.LastAccess = time.Now()
		session.mu.Unlock()

		path := r.URL.Path
		if path == "/" {
			path = "/" + config.EntryPoint
			if config.EntryPoint == "" {
				path = "/index.html"
			}
		}
		path = strings.TrimPrefix(path, "/")

		session.mu.RLock()
		cached, exists := session.FileCache[path]
		session.mu.RUnlock()

		if !exists {
			// Try common variations
			variations := []string{
				path,
				path + ".html",
				path + "/index.html",
				"public/" + path,
				"src/" + path,
			}
			for _, v := range variations {
				session.mu.RLock()
				cached, exists = session.FileCache[v]
				session.mu.RUnlock()
				if exists {
					break
				}
			}
		}

		if !exists {
			// Return 404 page
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(ps.generate404Page(path)))
			return
		}

		w.Header().Set("Content-Type", cached.ContentType)
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("X-APEX-Preview", "true")
		w.Write([]byte(cached.Content))
	}
}

func (ps *PreviewServer) createReloadHandler(session *PreviewSession) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session.mu.RLock()
		defer session.mu.RUnlock()

		message := map[string]interface{}{
			"type": "reload",
		}
		msgBytes, _ := json.Marshal(message)

		for client := range session.Clients {
			client.WriteMessage(websocket.TextMessage, msgBytes)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "reloaded"}`))
	}
}

func (ps *PreviewServer) createWebSocketHandler(session *PreviewSession) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := ps.upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		session.mu.Lock()
		session.Clients[conn] = true
		session.mu.Unlock()

		defer func() {
			session.mu.Lock()
			delete(session.Clients, conn)
			session.mu.Unlock()
			conn.Close()
		}()

		// Keep connection alive
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}
}

func (ps *PreviewServer) getContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	types := map[string]string{
		".html": "text/html; charset=utf-8",
		".htm":  "text/html; charset=utf-8",
		".css":  "text/css; charset=utf-8",
		".js":   "application/javascript; charset=utf-8",
		".mjs":  "application/javascript; charset=utf-8",
		".json": "application/json; charset=utf-8",
		".xml":  "application/xml; charset=utf-8",
		".svg":  "image/svg+xml",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".ico":  "image/x-icon",
		".woff": "font/woff",
		".woff2": "font/woff2",
		".ttf":  "font/ttf",
		".eot":  "application/vnd.ms-fontobject",
		".txt":  "text/plain; charset=utf-8",
		".md":   "text/markdown; charset=utf-8",
		".ts":   "application/typescript; charset=utf-8",
		".tsx":  "application/typescript; charset=utf-8",
		".jsx":  "application/javascript; charset=utf-8",
		".vue":  "application/javascript; charset=utf-8",
		".svelte": "application/javascript; charset=utf-8",
	}

	if contentType, ok := types[ext]; ok {
		return contentType
	}
	return "application/octet-stream"
}

func (ps *PreviewServer) generate404Page(path string) string {
	tmpl := `<!DOCTYPE html>
<html>
<head>
  <title>404 - Not Found</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
      background: #0a0a0f;
      color: #fff;
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      margin: 0;
    }
    .container {
      text-align: center;
      padding: 40px;
    }
    h1 {
      font-size: 120px;
      margin: 0;
      background: linear-gradient(135deg, #06b6d4, #8b5cf6);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
    }
    p {
      color: #64748b;
      font-size: 18px;
    }
    code {
      background: #1e1e2e;
      padding: 4px 8px;
      border-radius: 4px;
      color: #06b6d4;
    }
    .hint {
      margin-top: 30px;
      padding: 20px;
      background: #1e1e2e;
      border-radius: 8px;
      text-align: left;
    }
    .hint h3 {
      color: #06b6d4;
      margin-top: 0;
    }
    .hint ul {
      color: #94a3b8;
      line-height: 1.8;
    }
  </style>
</head>
<body>
  <div class="container">
    <h1>404</h1>
    <p>File not found: <code>{{.Path}}</code></p>
    <div class="hint">
      <h3>Tips:</h3>
      <ul>
        <li>Make sure the file exists in your project</li>
        <li>Check the file path is correct</li>
        <li>For React/Vue apps, ensure you have an <code>index.html</code> entry point</li>
        <li>Static assets should be in a <code>public</code> folder</li>
      </ul>
    </div>
  </div>
</body>
</html>`

	t, _ := template.New("404").Parse(tmpl)
	var buf strings.Builder
	t.Execute(&buf, map[string]string{"Path": path})
	return buf.String()
}

// CleanupIdleSessions removes preview sessions that have been idle
func (ps *PreviewServer) CleanupIdleSessions(maxIdleTime time.Duration) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	now := time.Now()
	for projectID, session := range ps.sessions {
		session.mu.RLock()
		idle := now.Sub(session.LastAccess) > maxIdleTime && len(session.Clients) == 0
		session.mu.RUnlock()

		if idle {
			// Stop session
			close(session.stopChan)
			if session.server != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				session.server.Shutdown(ctx)
				cancel()
			}
			ps.releasePort(projectID)
			delete(ps.sessions, projectID)
		}
	}
}

// GetAllPreviews returns all active preview sessions
func (ps *PreviewServer) GetAllPreviews() []*PreviewStatus {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	previews := make([]*PreviewStatus, 0, len(ps.sessions))
	for _, session := range ps.sessions {
		previews = append(previews, ps.getStatus(session))
	}
	return previews
}

// PreviewHandler HTTP handler for preview management
type PreviewHandler struct {
	server *PreviewServer
}

// NewPreviewHandler creates a new preview handler
func NewPreviewHandler(server *PreviewServer) *PreviewHandler {
	return &PreviewHandler{server: server}
}

// ServePreviewFrame serves an iframe wrapper for the preview
func (h *PreviewHandler) ServePreviewFrame(w http.ResponseWriter, r *http.Request) {
	projectIDStr := r.URL.Query().Get("project_id")
	projectID, _ := strconv.ParseUint(projectIDStr, 10, 32)

	status := h.server.GetPreviewStatus(uint(projectID))

	tmpl := `<!DOCTYPE html>
<html>
<head>
  <title>APEX.BUILD Preview</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body {
      background: #0a0a0f;
      height: 100vh;
      display: flex;
      flex-direction: column;
    }
    .toolbar {
      background: #1e1e2e;
      padding: 8px 16px;
      display: flex;
      align-items: center;
      gap: 12px;
      border-bottom: 1px solid #2d2d3d;
    }
    .toolbar-title {
      color: #06b6d4;
      font-weight: 600;
      font-size: 14px;
    }
    .toolbar-url {
      flex: 1;
      background: #0a0a0f;
      border: 1px solid #2d2d3d;
      border-radius: 6px;
      padding: 6px 12px;
      color: #94a3b8;
      font-size: 13px;
    }
    .toolbar-btn {
      background: #2d2d3d;
      border: none;
      border-radius: 6px;
      padding: 6px 12px;
      color: #fff;
      cursor: pointer;
      font-size: 13px;
    }
    .toolbar-btn:hover {
      background: #3d3d4d;
    }
    iframe {
      flex: 1;
      border: none;
      width: 100%;
    }
    .not-running {
      flex: 1;
      display: flex;
      align-items: center;
      justify-content: center;
      color: #64748b;
    }
  </style>
</head>
<body>
  <div class="toolbar">
    <span class="toolbar-title">Preview</span>
    <input type="text" class="toolbar-url" value="{{.URL}}" readonly />
    <button class="toolbar-btn" onclick="document.getElementById('preview').src = document.getElementById('preview').src">Refresh</button>
    <button class="toolbar-btn" onclick="window.open('{{.URL}}', '_blank')">Open in Tab</button>
  </div>
  {{if .Active}}
  <iframe id="preview" src="{{.URL}}"></iframe>
  {{else}}
  <div class="not-running">
    <p>Preview not running. Start the preview to see your app.</p>
  </div>
  {{end}}
</body>
</html>`

	t, _ := template.New("frame").Parse(tmpl)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t.Execute(w, status)
}
