// Package preview - Backend Server Runner for APEX.BUILD Preview System
// Manages Node.js, Python, Go, and Rust backend server processes alongside frontend preview
package preview

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"apex-build/pkg/models"

	"gorm.io/gorm"
)

// ServerRunner manages backend server processes for preview sessions
type ServerRunner struct {
	db        *gorm.DB
	processes map[uint]*ServerProcess
	mu        sync.RWMutex
	portStart int
	portMap   map[uint]int // projectID -> assigned backend port
	portMu    sync.Mutex
}

// ServerProcess represents a running backend server
type ServerProcess struct {
	ProjectID   uint
	Command     string   // "node", "python", "go run", "cargo run"
	Args        []string // Command arguments
	EntryFile   string   // "server.js", "app.py", "main.go"
	Port        int
	Cmd         *exec.Cmd
	Stdout      *bytes.Buffer
	Stderr      *bytes.Buffer
	bufMu       sync.Mutex // Protects Stdout and Stderr buffer access
	StartedAt   time.Time
	Ready       bool
	URL         string // Full URL to the server
	Pid         int
	WorkDir     string
	EnvVars     map[string]string
	stopChan    chan struct{}
	stoppedChan chan struct{}
}

// ServerConfig contains configuration for starting a backend server
type ServerConfig struct {
	ProjectID uint              `json:"project_id"`
	EntryFile string            `json:"entry_file"` // Required if not auto-detected
	Command   string            `json:"command"`    // Optional, auto-detect if empty
	EnvVars   map[string]string `json:"env_vars"`
	WorkDir   string            `json:"work_dir"`
}

// ServerStatus represents the current state of a backend server
type ServerStatus struct {
	Running       bool      `json:"running"`
	Port          int       `json:"port,omitempty"`
	Pid           int       `json:"pid,omitempty"`
	UptimeSeconds int64     `json:"uptime_seconds,omitempty"`
	Command       string    `json:"command,omitempty"`
	EntryFile     string    `json:"entry_file,omitempty"`
	URL           string    `json:"url,omitempty"`
	StartedAt     time.Time `json:"started_at,omitempty"`
	Ready         bool      `json:"ready"`
}

// ServerLogs contains captured server output
type ServerLogs struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

// ServerDetection contains auto-detected server configuration
type ServerDetection struct {
	HasBackend bool   `json:"has_backend"`
	ServerType string `json:"server_type"` // node, python, go, rust
	EntryFile  string `json:"entry_file"`
	Command    string `json:"command"`
	Framework  string `json:"framework"` // express, fastapi, gin, etc.
}

// NewServerRunner creates a new server runner manager
func NewServerRunner(db *gorm.DB) *ServerRunner {
	return &ServerRunner{
		db:        db,
		processes: make(map[uint]*ServerProcess),
		portStart: 9100, // Backend ports start at 9100 (preview uses 9000+)
		portMap:   make(map[uint]int),
	}
}

// DetectServer auto-detects backend server configuration from project files
func (sr *ServerRunner) DetectServer(ctx context.Context, projectID uint) (*ServerDetection, error) {
	detection := &ServerDetection{
		HasBackend: false,
	}

	// Load all project files
	var files []models.File
	if err := sr.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&files).Error; err != nil {
		return nil, fmt.Errorf("failed to load project files: %w", err)
	}

	// Create a map for quick lookup
	fileMap := make(map[string]string)
	for _, f := range files {
		fileMap[f.Path] = f.Content
	}

	// Check for Node.js
	if content, ok := fileMap["package.json"]; ok {
		if strings.Contains(content, `"start"`) || strings.Contains(content, `"serve"`) {
			detection.HasBackend = true
			detection.ServerType = "node"
			detection.Command = "node"

			// Detect framework
			if strings.Contains(content, `"express"`) {
				detection.Framework = "express"
			} else if strings.Contains(content, `"fastify"`) {
				detection.Framework = "fastify"
			} else if strings.Contains(content, `"koa"`) {
				detection.Framework = "koa"
			} else if strings.Contains(content, `"hapi"`) {
				detection.Framework = "hapi"
			} else if strings.Contains(content, `"nest"`) || strings.Contains(content, `"@nestjs"`) {
				detection.Framework = "nestjs"
			}

			// Find entry file
			nodeEntries := []string{
				"server.js", "index.js", "app.js", "main.js",
				"src/server.js", "src/index.js", "src/app.js", "src/main.js",
				"server/index.js", "server/app.js",
				"dist/index.js", "dist/server.js",
			}
			for _, entry := range nodeEntries {
				if _, exists := fileMap[entry]; exists {
					detection.EntryFile = entry
					break
				}
			}

			return detection, nil
		}
	}

	// Check for Python
	if content, ok := fileMap["requirements.txt"]; ok {
		if strings.Contains(strings.ToLower(content), "flask") ||
			strings.Contains(strings.ToLower(content), "django") ||
			strings.Contains(strings.ToLower(content), "fastapi") ||
			strings.Contains(strings.ToLower(content), "uvicorn") {

			detection.HasBackend = true
			detection.ServerType = "python"
			detection.Command = "python"

			// Detect framework
			if strings.Contains(strings.ToLower(content), "flask") {
				detection.Framework = "flask"
			} else if strings.Contains(strings.ToLower(content), "django") {
				detection.Framework = "django"
			} else if strings.Contains(strings.ToLower(content), "fastapi") {
				detection.Framework = "fastapi"
				detection.Command = "uvicorn"
			}

			// Find entry file
			pythonEntries := []string{
				"app.py", "main.py", "server.py", "wsgi.py", "run.py",
				"src/app.py", "src/main.py", "src/server.py",
				"application.py", "api.py",
			}
			for _, entry := range pythonEntries {
				if _, exists := fileMap[entry]; exists {
					detection.EntryFile = entry
					break
				}
			}

			return detection, nil
		}
	}

	// Check for pyproject.toml (modern Python)
	if content, ok := fileMap["pyproject.toml"]; ok {
		if strings.Contains(strings.ToLower(content), "flask") ||
			strings.Contains(strings.ToLower(content), "django") ||
			strings.Contains(strings.ToLower(content), "fastapi") {

			detection.HasBackend = true
			detection.ServerType = "python"
			detection.Command = "python"

			pythonEntries := []string{
				"app.py", "main.py", "server.py",
				"src/app.py", "src/main.py",
			}
			for _, entry := range pythonEntries {
				if _, exists := fileMap[entry]; exists {
					detection.EntryFile = entry
					break
				}
			}

			return detection, nil
		}
	}

	// Check for Go
	if _, ok := fileMap["go.mod"]; ok {
		detection.HasBackend = true
		detection.ServerType = "go"
		detection.Command = "go run"

		// Detect framework
		if content, ok := fileMap["go.mod"]; ok {
			if strings.Contains(content, "gin-gonic/gin") {
				detection.Framework = "gin"
			} else if strings.Contains(content, "gorilla/mux") {
				detection.Framework = "gorilla"
			} else if strings.Contains(content, "labstack/echo") {
				detection.Framework = "echo"
			} else if strings.Contains(content, "go-chi/chi") {
				detection.Framework = "chi"
			} else if strings.Contains(content, "gofiber/fiber") {
				detection.Framework = "fiber"
			}
		}

		// Find entry file
		goEntries := []string{
			"main.go", "cmd/main.go", "cmd/server/main.go",
			"server/main.go", "cmd/api/main.go",
		}
		for _, entry := range goEntries {
			if _, exists := fileMap[entry]; exists {
				detection.EntryFile = entry
				break
			}
		}

		return detection, nil
	}

	// Check for Rust
	if _, ok := fileMap["Cargo.toml"]; ok {
		detection.HasBackend = true
		detection.ServerType = "rust"
		detection.Command = "cargo run"

		// Detect framework from Cargo.toml
		if content, ok := fileMap["Cargo.toml"]; ok {
			if strings.Contains(content, "actix-web") {
				detection.Framework = "actix"
			} else if strings.Contains(content, "rocket") {
				detection.Framework = "rocket"
			} else if strings.Contains(content, "axum") {
				detection.Framework = "axum"
			} else if strings.Contains(content, "warp") {
				detection.Framework = "warp"
			}
		}

		detection.EntryFile = "src/main.rs"
		return detection, nil
	}

	return detection, nil
}

// Start starts a backend server process
func (sr *ServerRunner) Start(ctx context.Context, config *ServerConfig) (*ServerProcess, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	// Check if already running
	if proc, exists := sr.processes[config.ProjectID]; exists {
		if proc.Ready {
			return proc, nil
		}
		// Stop existing process if not ready
		sr.stopProcessLocked(config.ProjectID)
	}

	// Auto-detect if needed
	if config.Command == "" || config.EntryFile == "" {
		detection, err := sr.DetectServer(ctx, config.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("failed to detect server: %w", err)
		}
		if !detection.HasBackend {
			return nil, fmt.Errorf("no backend server detected in project")
		}
		if config.Command == "" {
			config.Command = detection.Command
		}
		if config.EntryFile == "" {
			config.EntryFile = detection.EntryFile
		}
	}

	// Allocate port
	port := sr.allocatePort(config.ProjectID)

	// Build work directory (use a temp dir for now)
	workDir := config.WorkDir
	if workDir == "" {
		workDir = filepath.Join(os.TempDir(), fmt.Sprintf("apex-preview-%d", config.ProjectID))
		if err := os.MkdirAll(workDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create work directory: %w", err)
		}
		// Write project files to work directory
		if err := sr.writeProjectFiles(ctx, config.ProjectID, workDir); err != nil {
			return nil, fmt.Errorf("failed to write project files: %w", err)
		}
	}

	// Build command and args based on server type
	var cmd *exec.Cmd
	var args []string

	switch {
	case config.Command == "node":
		args = []string{config.EntryFile}
		cmd = exec.CommandContext(ctx, "node", args...)

	case config.Command == "python":
		args = []string{config.EntryFile}
		cmd = exec.CommandContext(ctx, "python", args...)

	case config.Command == "uvicorn":
		// FastAPI: uvicorn main:app --host 0.0.0.0 --port 9100
		module := strings.TrimSuffix(config.EntryFile, ".py")
		module = strings.ReplaceAll(module, "/", ".")
		args = []string{module + ":app", "--host", "0.0.0.0", "--port", fmt.Sprintf("%d", port)}
		cmd = exec.CommandContext(ctx, "uvicorn", args...)

	case config.Command == "go run":
		if config.EntryFile != "" {
			args = []string{"run", config.EntryFile}
		} else {
			args = []string{"run", "."}
		}
		cmd = exec.CommandContext(ctx, "go", args...)

	case config.Command == "cargo run":
		args = []string{"run"}
		cmd = exec.CommandContext(ctx, "cargo", args...)

	default:
		// Custom command
		parts := strings.Fields(config.Command)
		if len(parts) == 0 {
			return nil, fmt.Errorf("invalid command: %s", config.Command)
		}
		cmd = exec.CommandContext(ctx, parts[0], append(parts[1:], config.EntryFile)...)
	}

	// Set working directory
	cmd.Dir = workDir

	// Set up process group for proper cleanup
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Set environment variables
	env := os.Environ()
	env = append(env, fmt.Sprintf("PORT=%d", port))
	env = append(env, fmt.Sprintf("HOST=0.0.0.0"))
	env = append(env, fmt.Sprintf("NODE_ENV=development"))
	env = append(env, fmt.Sprintf("FLASK_ENV=development"))
	env = append(env, fmt.Sprintf("DEBUG=true"))

	if config.EnvVars != nil {
		for k, v := range config.EnvVars {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	cmd.Env = env

	// Capture output
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Create multi-writers to capture and limit output
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	// Create process struct
	proc := &ServerProcess{
		ProjectID:   config.ProjectID,
		Command:     config.Command,
		Args:        args,
		EntryFile:   config.EntryFile,
		Port:        port,
		Cmd:         cmd,
		Stdout:      stdout,
		Stderr:      stderr,
		StartedAt:   time.Now(),
		Ready:       false,
		URL:         fmt.Sprintf("http://localhost:%d", port),
		WorkDir:     workDir,
		EnvVars:     config.EnvVars,
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		sr.releasePort(config.ProjectID)
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	proc.Pid = cmd.Process.Pid

	// Start output capture goroutines
	go sr.captureOutput(stdoutPipe, stdout, proc)
	go sr.captureOutput(stderrPipe, stderr, proc)

	// Wait for process completion in background
	go func() {
		defer close(proc.stoppedChan)
		cmd.Wait()
	}()

	// Wait for port to be ready
	ready := sr.waitForPort(port, 30*time.Second, proc.stopChan)
	proc.Ready = ready

	if !ready {
		// Check if process died
		select {
		case <-proc.stoppedChan:
			sr.releasePort(config.ProjectID)
			return nil, fmt.Errorf("server process exited before becoming ready: %s", stderr.String())
		default:
			// Process still running but port not ready
			sr.releasePort(config.ProjectID)
			sr.killProcess(proc)
			return nil, fmt.Errorf("server did not start listening on port %d within 30 seconds", port)
		}
	}

	sr.processes[config.ProjectID] = proc

	return proc, nil
}

// Stop stops a backend server process
func (sr *ServerRunner) Stop(ctx context.Context, projectID uint) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	return sr.stopProcessLocked(projectID)
}

func (sr *ServerRunner) stopProcessLocked(projectID uint) error {
	proc, exists := sr.processes[projectID]
	if !exists {
		return nil // Already stopped
	}

	// Signal stop
	close(proc.stopChan)

	// Kill the process
	sr.killProcess(proc)

	// Release port
	sr.releasePort(projectID)

	// Clean up work directory
	if proc.WorkDir != "" && strings.HasPrefix(proc.WorkDir, os.TempDir()) {
		os.RemoveAll(proc.WorkDir)
	}

	delete(sr.processes, projectID)
	return nil
}

func (sr *ServerRunner) killProcess(proc *ServerProcess) {
	if proc.Cmd == nil || proc.Cmd.Process == nil {
		return
	}

	// Try graceful shutdown first (SIGTERM)
	syscall.Kill(-proc.Cmd.Process.Pid, syscall.SIGTERM)

	// Wait up to 5 seconds for graceful shutdown
	done := make(chan struct{})
	go func() {
		select {
		case <-proc.stoppedChan:
			close(done)
		case <-time.After(5 * time.Second):
			// Force kill (SIGKILL)
			syscall.Kill(-proc.Cmd.Process.Pid, syscall.SIGKILL)
			close(done)
		}
	}()

	<-done
}

// GetStatus returns the current status of a backend server
func (sr *ServerRunner) GetStatus(projectID uint) *ServerStatus {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	proc, exists := sr.processes[projectID]
	if !exists {
		return &ServerStatus{Running: false}
	}

	uptime := int64(0)
	if !proc.StartedAt.IsZero() {
		uptime = int64(time.Since(proc.StartedAt).Seconds())
	}

	return &ServerStatus{
		Running:       true,
		Port:          proc.Port,
		Pid:           proc.Pid,
		UptimeSeconds: uptime,
		Command:       proc.Command,
		EntryFile:     proc.EntryFile,
		URL:           proc.URL,
		StartedAt:     proc.StartedAt,
		Ready:         proc.Ready,
	}
}

// GetLogs returns captured server logs
func (sr *ServerRunner) GetLogs(projectID uint) *ServerLogs {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	proc, exists := sr.processes[projectID]
	if !exists {
		return &ServerLogs{}
	}

	// Lock buffer access to prevent data race
	proc.bufMu.Lock()
	stdout := proc.Stdout.String()
	stderr := proc.Stderr.String()
	proc.bufMu.Unlock()

	return &ServerLogs{
		Stdout: sr.getLastLines(stdout, 1000),
		Stderr: sr.getLastLines(stderr, 1000),
	}
}

// GetProcess returns the server process for a project (for reverse proxy integration)
func (sr *ServerRunner) GetProcess(projectID uint) *ServerProcess {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.processes[projectID]
}

// Helper methods

func (sr *ServerRunner) allocatePort(projectID uint) int {
	sr.portMu.Lock()
	defer sr.portMu.Unlock()

	// Check if already assigned
	if port, exists := sr.portMap[projectID]; exists {
		return port
	}

	// Find next available port
	port := sr.portStart
	usedPorts := make(map[int]bool)
	for _, p := range sr.portMap {
		usedPorts[p] = true
	}

	for usedPorts[port] || !sr.isPortAvailable(port) {
		port++
	}

	sr.portMap[projectID] = port
	return port
}

func (sr *ServerRunner) releasePort(projectID uint) {
	sr.portMu.Lock()
	defer sr.portMu.Unlock()
	delete(sr.portMap, projectID)
}

func (sr *ServerRunner) isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

func (sr *ServerRunner) waitForPort(port int, timeout time.Duration, stop <-chan struct{}) bool {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		select {
		case <-stop:
			return false
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 100*time.Millisecond)
			if err == nil {
				conn.Close()
				return true
			}
		}
	}
	return false
}

func (sr *ServerRunner) captureOutput(pipe io.ReadCloser, buf *bytes.Buffer, proc *ServerProcess) {
	defer pipe.Close()

	// Read in chunks
	buffer := make([]byte, 4096)
	for {
		select {
		case <-proc.stopChan:
			return
		default:
			n, err := pipe.Read(buffer)
			if n > 0 {
				proc.bufMu.Lock()
				buf.Write(buffer[:n])
				// Limit buffer size to 10MB
				if buf.Len() > 10*1024*1024 {
					// Trim to last 5MB
					data := buf.Bytes()
					buf.Reset()
					buf.Write(data[len(data)-5*1024*1024:])
				}
				proc.bufMu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}
}

func (sr *ServerRunner) getLastLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

func (sr *ServerRunner) writeProjectFiles(ctx context.Context, projectID uint, workDir string) error {
	var files []models.File
	if err := sr.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&files).Error; err != nil {
		return err
	}

	cleanWorkDir := filepath.Clean(workDir)

	for _, file := range files {
		if file.Type == "directory" {
			continue
		}

		filePath := filepath.Join(workDir, file.Path)
		// Validate path to prevent path traversal attacks
		cleanPath := filepath.Clean(filePath)
		if !strings.HasPrefix(cleanPath, cleanWorkDir+string(filepath.Separator)) && cleanPath != cleanWorkDir {
			return fmt.Errorf("path traversal detected: %s", file.Path)
		}

		dirPath := filepath.Dir(filePath)

		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
		}

		if err := os.WriteFile(filePath, []byte(file.Content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", filePath, err)
		}
	}

	return nil
}

// StopAll stops all running server processes (for cleanup)
func (sr *ServerRunner) StopAll(ctx context.Context) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	for projectID := range sr.processes {
		sr.stopProcessLocked(projectID)
	}
}
