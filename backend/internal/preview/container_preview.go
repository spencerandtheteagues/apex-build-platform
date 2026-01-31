// Package preview - Container-Based Preview Server for APEX.BUILD
// Provides Docker-based isolated preview environments with security sandboxing.
package preview

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	osexec "os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"apex-build/pkg/models"

	"gorm.io/gorm"
)

// ContainerPreviewServer provides Docker-based preview isolation
type ContainerPreviewServer struct {
	*PreviewServer
	containerSessions map[uint]*ContainerSession
	containerMu       sync.RWMutex
	config            *ContainerPreviewConfig
	dockerAvailable   bool
	seccompProfile    string
	baseTempDir       string
	stats             *ContainerPreviewStats
	cleanupTicker     *time.Ticker
	stopCleanup       chan struct{}
}

// ContainerSession represents an active container-based preview
type ContainerSession struct {
	ProjectID     uint
	ContainerID   string
	ContainerName string
	ImageName     string
	Port          int
	InternalPort  int
	StartedAt     time.Time
	LastAccess    time.Time
	Framework     string
	TempDir       string
	Config        *ContainerConfig
	stopChan      chan struct{}
	mu            sync.RWMutex
}

// ContainerConfig holds container-specific configuration
type ContainerConfig struct {
	Image           string        // Base image (auto-detected if empty)
	MemoryMB        int64         // Memory limit in MB (default 256)
	CPUPercent      float64       // CPU limit as fraction (default 0.5)
	NetworkMode     string        // "none", "bridge" (default: "bridge" for previews)
	Timeout         time.Duration // Max runtime (default: 30 minutes)
	ReadOnly        bool          // Read-only root filesystem (default: true for security)
	PidsLimit       int64         // Max processes (default: 100)
	EnableSeccomp   bool          // Use seccomp profile (default: true)
	DropCapabilities bool         // Drop all Linux capabilities (default: true)
}

// ContainerPreviewConfig holds global container preview configuration
type ContainerPreviewConfig struct {
	// Docker settings
	DockerSocket      string
	ImagePrefix       string
	BasePort          int
	MaxContainers     int32
	MaxContainerAge   time.Duration
	CleanupInterval   time.Duration

	// Default resource limits
	DefaultMemoryMB   int64
	DefaultCPUPercent float64
	DefaultTimeout    time.Duration
	DefaultPidsLimit  int64

	// Security settings
	EnableSeccomp        bool
	EnableReadOnlyRoot   bool
	DropAllCapabilities  bool
	NoNewPrivileges      bool

	// Temp directory for build contexts
	TempDir string

	// Logging
	EnableAuditLog bool
	AuditLogPath   string
}

// ContainerPreviewStats tracks container preview statistics
type ContainerPreviewStats struct {
	TotalContainersCreated int64
	ActiveContainers       int32
	MaxConcurrentContainers int32
	FailedContainers       int64
	TimeoutContainers      int64
	TotalBuildTime         int64 // milliseconds
	TotalRuntime           int64 // milliseconds
}

// SeccompProfile represents the seccomp security profile for containers
type SeccompProfile struct {
	DefaultAction string           `json:"defaultAction"`
	Architectures []string         `json:"architectures"`
	Syscalls      []SeccompSyscall `json:"syscalls"`
}

// SeccompSyscall defines allowed/blocked syscalls
type SeccompSyscall struct {
	Names  []string     `json:"names"`
	Action string       `json:"action"`
	Args   []SeccompArg `json:"args,omitempty"`
}

// SeccompArg for conditional syscall filtering
type SeccompArg struct {
	Index uint   `json:"index"`
	Value uint64 `json:"value"`
	Op    string `json:"op"`
}

// DefaultContainerPreviewConfig returns production-ready defaults
func DefaultContainerPreviewConfig() *ContainerPreviewConfig {
	return &ContainerPreviewConfig{
		DockerSocket:         "/var/run/docker.sock",
		ImagePrefix:          "apex-preview",
		BasePort:             10000,
		MaxContainers:        50,
		MaxContainerAge:      30 * time.Minute,
		CleanupInterval:      5 * time.Minute,
		DefaultMemoryMB:      256,
		DefaultCPUPercent:    0.5,
		DefaultTimeout:       30 * time.Minute,
		DefaultPidsLimit:     100,
		EnableSeccomp:        true,
		EnableReadOnlyRoot:   true,
		DropAllCapabilities:  true,
		NoNewPrivileges:      true,
		TempDir:              filepath.Join(os.TempDir(), "apex-preview-containers"),
		EnableAuditLog:       true,
		AuditLogPath:         "/var/log/apex-preview/audit.log",
	}
}

// NewContainerPreviewServer creates a new container-based preview server
func NewContainerPreviewServer(db *gorm.DB, config *ContainerPreviewConfig) (*ContainerPreviewServer, error) {
	if config == nil {
		config = DefaultContainerPreviewConfig()
	}

	// Create base preview server
	baseServer := NewPreviewServer(db)

	// Create temp directory
	if err := os.MkdirAll(config.TempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	server := &ContainerPreviewServer{
		PreviewServer:     baseServer,
		containerSessions: make(map[uint]*ContainerSession),
		config:            config,
		baseTempDir:       config.TempDir,
		stats:             &ContainerPreviewStats{},
		stopCleanup:       make(chan struct{}),
	}

	// Check Docker availability
	server.dockerAvailable = server.checkDockerAvailable()
	if !server.dockerAvailable {
		// Log warning but don't fail - we can fall back to process-based previews
		fmt.Println("Warning: Docker not available, container sandbox mode disabled")
	} else {
		// Generate seccomp profile
		seccompPath := filepath.Join(config.TempDir, "seccomp-preview.json")
		if err := server.writeSeccompProfile(seccompPath); err != nil {
			fmt.Printf("Warning: could not write seccomp profile: %v\n", err)
		} else {
			server.seccompProfile = seccompPath
		}

		// Start cleanup goroutine
		server.startCleanupLoop()
	}

	return server, nil
}

// IsDockerAvailable returns whether Docker is available for container previews
func (s *ContainerPreviewServer) IsDockerAvailable() bool {
	return s.dockerAvailable
}

// checkDockerAvailable verifies Docker daemon is accessible
func (s *ContainerPreviewServer) checkDockerAvailable() bool {
	cmd := osexec.Command("docker", "info")
	cmd.Env = append(os.Environ(), "DOCKER_HOST=unix://"+s.config.DockerSocket)
	err := cmd.Run()
	return err == nil
}

// StartContainerPreview starts a container-based preview session
func (s *ContainerPreviewServer) StartContainerPreview(ctx context.Context, config *PreviewConfig) (*PreviewStatus, error) {
	if !s.dockerAvailable {
		return nil, fmt.Errorf("Docker is not available - container preview requires Docker daemon")
	}

	// Check concurrent container limit
	current := atomic.LoadInt32(&s.stats.ActiveContainers)
	if current >= s.config.MaxContainers {
		return nil, fmt.Errorf("maximum container limit reached (%d)", s.config.MaxContainers)
	}

	s.containerMu.Lock()
	defer s.containerMu.Unlock()

	// Check if container session already exists
	if session, exists := s.containerSessions[config.ProjectID]; exists {
		session.mu.Lock()
		session.LastAccess = time.Now()
		session.mu.Unlock()
		return s.getContainerStatus(session), nil
	}

	// Create container config with defaults
	containerConfig := &ContainerConfig{
		MemoryMB:         s.config.DefaultMemoryMB,
		CPUPercent:       s.config.DefaultCPUPercent,
		NetworkMode:      "bridge", // Need network for preview access
		Timeout:          s.config.DefaultTimeout,
		ReadOnly:         s.config.EnableReadOnlyRoot,
		PidsLimit:        s.config.DefaultPidsLimit,
		EnableSeccomp:    s.config.EnableSeccomp,
		DropCapabilities: s.config.DropAllCapabilities,
	}

	// Create temp directory for build context
	tempDir, err := os.MkdirTemp(s.baseTempDir, fmt.Sprintf("preview-%d-", config.ProjectID))
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Get project files from database
	var files []models.File
	if err := s.db.WithContext(ctx).Where("project_id = ?", config.ProjectID).Find(&files).Error; err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to load project files: %w", err)
	}

	// Write files to temp directory
	for _, file := range files {
		filePath := filepath.Join(tempDir, file.Path)
		// Validate path to prevent path traversal attacks
		cleanPath := filepath.Clean(filePath)
		cleanTempDir := filepath.Clean(tempDir)
		if !strings.HasPrefix(cleanPath, cleanTempDir+string(filepath.Separator)) && cleanPath != cleanTempDir {
			os.RemoveAll(tempDir)
			return nil, fmt.Errorf("path traversal detected: %s", file.Path)
		}
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			os.RemoveAll(tempDir)
			return nil, fmt.Errorf("failed to create directory for %s: %w", file.Path, err)
		}
		if err := os.WriteFile(filePath, []byte(file.Content), 0644); err != nil {
			os.RemoveAll(tempDir)
			return nil, fmt.Errorf("failed to write file %s: %w", file.Path, err)
		}
	}

	// Auto-detect framework if not specified
	framework := config.Framework
	if framework == "" {
		framework = s.detectFrameworkFromFiles(files)
	}

	// Generate Dockerfile
	dockerfile := s.generateDockerfile(framework)
	if err := os.WriteFile(filepath.Join(tempDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	// Assign port
	port := s.assignContainerPort(config.ProjectID)

	// Build container name and image name
	containerName := fmt.Sprintf("apex-preview-%d", config.ProjectID)
	imageName := fmt.Sprintf("%s-%d:latest", s.config.ImagePrefix, config.ProjectID)

	// Build Docker image
	startBuild := time.Now()
	if err := s.buildDockerImage(ctx, tempDir, imageName); err != nil {
		os.RemoveAll(tempDir)
		s.releaseContainerPort(config.ProjectID)
		atomic.AddInt64(&s.stats.FailedContainers, 1)
		return nil, fmt.Errorf("failed to build Docker image: %w", err)
	}
	atomic.AddInt64(&s.stats.TotalBuildTime, time.Since(startBuild).Milliseconds())

	// Run container
	internalPort := s.getInternalPort(framework)
	containerID, err := s.runContainer(ctx, imageName, containerName, port, internalPort, containerConfig)
	if err != nil {
		os.RemoveAll(tempDir)
		s.releaseContainerPort(config.ProjectID)
		atomic.AddInt64(&s.stats.FailedContainers, 1)
		// Clean up image
		osexec.Command("docker", "rmi", "-f", imageName).Run()
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Create session
	session := &ContainerSession{
		ProjectID:     config.ProjectID,
		ContainerID:   containerID,
		ContainerName: containerName,
		ImageName:     imageName,
		Port:          port,
		InternalPort:  internalPort,
		StartedAt:     time.Now(),
		LastAccess:    time.Now(),
		Framework:     framework,
		TempDir:       tempDir,
		Config:        containerConfig,
		stopChan:      make(chan struct{}),
	}

	s.containerSessions[config.ProjectID] = session
	atomic.AddInt32(&s.stats.ActiveContainers, 1)
	atomic.AddInt64(&s.stats.TotalContainersCreated, 1)

	// Update max concurrent
	for {
		max := atomic.LoadInt32(&s.stats.MaxConcurrentContainers)
		current := atomic.LoadInt32(&s.stats.ActiveContainers)
		if current <= max || atomic.CompareAndSwapInt32(&s.stats.MaxConcurrentContainers, max, current) {
			break
		}
	}

	// Wait for container to be ready
	if err := s.waitForContainerReady(ctx, port, 30*time.Second); err != nil {
		// Container started but not responding - still return status
		fmt.Printf("Warning: container may not be fully ready: %v\n", err)
	}

	return s.getContainerStatus(session), nil
}

// StopContainerPreview stops a container-based preview session
func (s *ContainerPreviewServer) StopContainerPreview(ctx context.Context, projectID uint) error {
	s.containerMu.Lock()
	session, exists := s.containerSessions[projectID]
	if !exists {
		s.containerMu.Unlock()
		return nil // Already stopped
	}
	delete(s.containerSessions, projectID)
	s.containerMu.Unlock()

	close(session.stopChan)
	atomic.AddInt32(&s.stats.ActiveContainers, -1)

	// Record runtime
	runtime := time.Since(session.StartedAt)
	atomic.AddInt64(&s.stats.TotalRuntime, runtime.Milliseconds())

	// Stop container gracefully (10s timeout)
	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := osexec.CommandContext(stopCtx, "docker", "stop", "-t", "5", session.ContainerID)
	cmd.Run() // Ignore errors - container might already be stopped

	// Remove container
	osexec.Command("docker", "rm", "-f", session.ContainerID).Run()

	// Remove image
	osexec.Command("docker", "rmi", "-f", session.ImageName).Run()

	// Clean up temp directory
	if session.TempDir != "" && strings.HasPrefix(session.TempDir, s.baseTempDir) {
		os.RemoveAll(session.TempDir)
	}

	// Release port
	s.releaseContainerPort(projectID)

	return nil
}

// GetContainerPreviewStatus returns the status of a container preview
func (s *ContainerPreviewServer) GetContainerPreviewStatus(projectID uint) *PreviewStatus {
	s.containerMu.RLock()
	session, exists := s.containerSessions[projectID]
	s.containerMu.RUnlock()

	if !exists {
		return &PreviewStatus{
			ProjectID: projectID,
			Active:    false,
		}
	}

	return s.getContainerStatus(session)
}

// buildDockerImage builds a Docker image from the project files
func (s *ContainerPreviewServer) buildDockerImage(ctx context.Context, contextDir, imageName string) error {
	cmd := osexec.CommandContext(ctx, "docker", "build",
		"-t", imageName,
		"-f", filepath.Join(contextDir, "Dockerfile"),
		"--no-cache",
		contextDir,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker build failed: %s\nOutput: %s", err, string(output))
	}

	return nil
}

// runContainer starts a Docker container with security constraints
func (s *ContainerPreviewServer) runContainer(ctx context.Context, imageName, containerName string, hostPort, containerPort int, config *ContainerConfig) (string, error) {
	args := []string{
		"run",
		"-d", // Detached
		"--name", containerName,
		// Port mapping
		"-p", fmt.Sprintf("%d:%d", hostPort, containerPort),
		// Resource limits
		"--memory", fmt.Sprintf("%dm", config.MemoryMB),
		"--memory-swap", fmt.Sprintf("%dm", config.MemoryMB), // Disable swap
		"--cpus", fmt.Sprintf("%.2f", config.CPUPercent),
		"--pids-limit", fmt.Sprintf("%d", config.PidsLimit),
	}

	// Security settings
	if config.DropCapabilities {
		args = append(args, "--cap-drop=ALL")
		// Add back minimal capabilities needed for web servers
		args = append(args, "--cap-add=NET_BIND_SERVICE")
	}

	// No new privileges
	if s.config.NoNewPrivileges {
		args = append(args, "--security-opt=no-new-privileges:true")
	}

	// Seccomp profile
	if config.EnableSeccomp && s.seccompProfile != "" {
		args = append(args, "--security-opt", fmt.Sprintf("seccomp=%s", s.seccompProfile))
	}

	// Read-only root filesystem with tmpfs for needed directories
	if config.ReadOnly {
		args = append(args,
			"--read-only",
			"--tmpfs", "/tmp:rw,noexec,nosuid,size=64m",
			"--tmpfs", "/var/run:rw,noexec,nosuid,size=8m",
			"--tmpfs", "/var/cache/nginx:rw,noexec,nosuid,size=32m",
		)
	}

	// Network mode
	if config.NetworkMode != "" {
		args = append(args, "--network="+config.NetworkMode)
	}

	// Restart policy - no automatic restarts for previews
	args = append(args, "--restart=no")

	// Labels for identification
	args = append(args,
		"--label", "apex.preview=true",
		"--label", fmt.Sprintf("apex.project=%s", containerName),
	)

	// Add image name
	args = append(args, imageName)

	cmd := osexec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker run failed: %s\nOutput: %s", err, string(output))
	}

	containerID := strings.TrimSpace(string(output))
	return containerID, nil
}

// waitForContainerReady waits for the container to be ready to accept connections
func (s *ContainerPreviewServer) waitForContainerReady(ctx context.Context, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	address := fmt.Sprintf("localhost:%d", port)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("container not ready after %v", timeout)
}

// generateDockerfile creates a Dockerfile based on the framework
func (s *ContainerPreviewServer) generateDockerfile(framework string) string {
	switch framework {
	case "react", "vue", "svelte", "next", "nuxt":
		return s.nodeDockerfile()
	case "flask", "django", "fastapi":
		return s.pythonDockerfile()
	case "static", "vanilla":
		return s.staticDockerfile()
	default:
		return s.staticDockerfile()
	}
}

// nodeDockerfile returns a Dockerfile for Node.js projects
func (s *ContainerPreviewServer) nodeDockerfile() string {
	return `# APEX.BUILD Preview Container - Node.js
FROM node:20-slim

# Create non-root user
RUN groupadd -r sandbox && useradd -r -g sandbox sandbox

# Install serve for static file serving
RUN npm install -g serve@14 && npm cache clean --force

# Set working directory
WORKDIR /app

# Copy project files
COPY --chown=sandbox:sandbox . .

# Install dependencies if package.json exists
RUN if [ -f package.json ]; then \
      npm install --production 2>/dev/null || true; \
    fi

# Build if build script exists
RUN if [ -f package.json ] && grep -q '"build"' package.json; then \
      npm run build 2>/dev/null || true; \
    fi

# Switch to non-root user
USER sandbox

# Expose port
EXPOSE 3000

# Start server - try common build output directories
CMD if [ -d "dist" ]; then \
      serve -s dist -l 3000; \
    elif [ -d "build" ]; then \
      serve -s build -l 3000; \
    elif [ -d "public" ]; then \
      serve -s public -l 3000; \
    else \
      serve -s . -l 3000; \
    fi
`
}

// pythonDockerfile returns a Dockerfile for Python projects
func (s *ContainerPreviewServer) pythonDockerfile() string {
	return `# APEX.BUILD Preview Container - Python
FROM python:3.12-slim

# Create non-root user
RUN groupadd -r sandbox && useradd -r -g sandbox sandbox

# Set working directory
WORKDIR /app

# Copy project files
COPY --chown=sandbox:sandbox . .

# Install dependencies if requirements.txt exists
RUN if [ -f requirements.txt ]; then \
      pip install --no-cache-dir -r requirements.txt 2>/dev/null || true; \
    fi

# Switch to non-root user
USER sandbox

# Expose port
EXPOSE 5000

# Start server - detect framework
CMD if [ -f "app.py" ]; then \
      python app.py; \
    elif [ -f "main.py" ]; then \
      python main.py; \
    elif [ -f "manage.py" ]; then \
      python manage.py runserver 0.0.0.0:5000; \
    else \
      python -m http.server 5000; \
    fi
`
}

// staticDockerfile returns a Dockerfile for static HTML/CSS/JS projects
func (s *ContainerPreviewServer) staticDockerfile() string {
	return `# APEX.BUILD Preview Container - Static
FROM nginx:alpine

# Create custom nginx config for better SPA support
RUN echo 'server { \
    listen 80; \
    server_name localhost; \
    root /usr/share/nginx/html; \
    index index.html; \
    \
    location / { \
        try_files $uri $uri/ /index.html; \
    } \
    \
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2)$ { \
        expires 1y; \
        add_header Cache-Control "public, immutable"; \
    } \
    \
    gzip on; \
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml; \
}' > /etc/nginx/conf.d/default.conf

# Copy project files
COPY . /usr/share/nginx/html/

# Expose port
EXPOSE 80
`
}

// detectFrameworkFromFiles detects the framework from project files
func (s *ContainerPreviewServer) detectFrameworkFromFiles(files []models.File) string {
	for _, file := range files {
		if file.Path == "package.json" {
			content := file.Content
			if strings.Contains(content, `"react"`) {
				return "react"
			}
			if strings.Contains(content, `"vue"`) {
				return "vue"
			}
			if strings.Contains(content, `"svelte"`) {
				return "svelte"
			}
			if strings.Contains(content, `"next"`) {
				return "next"
			}
			if strings.Contains(content, `"nuxt"`) {
				return "nuxt"
			}
		}
		if file.Path == "requirements.txt" {
			content := file.Content
			if strings.Contains(content, "flask") {
				return "flask"
			}
			if strings.Contains(content, "django") {
				return "django"
			}
			if strings.Contains(content, "fastapi") {
				return "fastapi"
			}
		}
	}
	return "static"
}

// getInternalPort returns the internal port based on framework
func (s *ContainerPreviewServer) getInternalPort(framework string) int {
	switch framework {
	case "react", "vue", "svelte", "next", "nuxt":
		return 3000
	case "flask", "django", "fastapi":
		return 5000
	case "static", "vanilla":
		return 80
	default:
		return 80
	}
}

// assignContainerPort assigns a unique port for a container
func (s *ContainerPreviewServer) assignContainerPort(projectID uint) int {
	s.portMu.Lock()
	defer s.portMu.Unlock()

	// Check if already assigned
	if port, exists := s.portMap[projectID]; exists {
		return port
	}

	// Find next available port starting from container base port
	port := s.config.BasePort
	usedPorts := make(map[int]bool)
	for _, p := range s.portMap {
		usedPorts[p] = true
	}

	for usedPorts[port] {
		port++
	}

	s.portMap[projectID] = port
	return port
}

// releaseContainerPort releases the port assigned to a container
func (s *ContainerPreviewServer) releaseContainerPort(projectID uint) {
	s.portMu.Lock()
	defer s.portMu.Unlock()
	delete(s.portMap, projectID)
}

// getContainerStatus returns the status of a container session
func (s *ContainerPreviewServer) getContainerStatus(session *ContainerSession) *PreviewStatus {
	session.mu.RLock()
	defer session.mu.RUnlock()

	return &PreviewStatus{
		ProjectID:  session.ProjectID,
		Active:     true,
		Port:       session.Port,
		URL:        fmt.Sprintf("http://localhost:%d", session.Port),
		StartedAt:  session.StartedAt,
		LastAccess: session.LastAccess,
		Clients:    0, // Container previews don't track WebSocket clients
	}
}

// writeSeccompProfile creates a restrictive seccomp profile for preview containers
func (s *ContainerPreviewServer) writeSeccompProfile(path string) error {
	profile := SeccompProfile{
		DefaultAction: "SCMP_ACT_ERRNO",
		Architectures: []string{
			"SCMP_ARCH_X86_64",
			"SCMP_ARCH_X86",
			"SCMP_ARCH_AARCH64",
			"SCMP_ARCH_ARM",
		},
		Syscalls: []SeccompSyscall{
			// Essential syscalls for web servers
			{Names: []string{"read", "write", "open", "close", "stat", "fstat", "lstat"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"poll", "lseek", "mmap", "mprotect", "munmap", "brk"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"rt_sigaction", "rt_sigprocmask", "rt_sigreturn", "ioctl"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"access", "pipe", "select", "sched_yield", "mremap"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"dup", "dup2", "pause", "nanosleep", "getitimer", "alarm"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"setitimer", "getpid", "socket", "connect", "sendto"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"recvfrom", "sendmsg", "recvmsg", "shutdown", "bind"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"listen", "getsockname", "getpeername", "socketpair"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"setsockopt", "getsockopt", "clone", "fork", "vfork"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"execve", "exit", "wait4", "kill", "uname", "fcntl"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"flock", "fsync", "fdatasync", "truncate", "ftruncate"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"getdents", "getcwd", "chdir", "fchdir", "rename"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"mkdir", "rmdir", "creat", "link", "unlink", "symlink"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"readlink", "chmod", "fchmod", "chown", "fchown"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"lchown", "umask", "gettimeofday", "getrlimit", "getrusage"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"sysinfo", "times", "getuid", "getgid", "setuid"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"setgid", "geteuid", "getegid", "setpgid", "getppid"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"getpgrp", "setsid", "setreuid", "setregid", "getgroups"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"setgroups", "setresuid", "getresuid", "setresgid"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"getresgid", "getpgid", "setfsuid", "setfsgid", "getsid"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"capget", "capset", "rt_sigpending", "rt_sigtimedwait"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"rt_sigqueueinfo", "sigaltstack", "utime", "mknod"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"personality", "statfs", "fstatfs", "getpriority"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"setpriority", "sched_setparam", "sched_getparam"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"sched_setscheduler", "sched_getscheduler"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"sched_get_priority_max", "sched_get_priority_min"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"sched_rr_get_interval", "mlock", "munlock", "mlockall"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"munlockall", "prctl", "arch_prctl"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"setrlimit", "sync"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"gettid", "readahead", "setxattr", "lsetxattr"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"fsetxattr", "getxattr", "lgetxattr", "fgetxattr"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"listxattr", "llistxattr", "flistxattr", "removexattr"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"lremovexattr", "fremovexattr", "tkill", "time"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"futex", "sched_setaffinity", "sched_getaffinity"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"set_thread_area", "io_setup", "io_destroy", "io_getevents"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"io_submit", "io_cancel", "get_thread_area", "epoll_create"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"remap_file_pages", "getdents64", "set_tid_address"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"restart_syscall", "semtimedop", "fadvise64", "timer_create"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"timer_settime", "timer_gettime", "timer_getoverrun"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"timer_delete", "clock_settime", "clock_gettime"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"clock_getres", "clock_nanosleep", "exit_group", "epoll_wait"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"epoll_ctl", "tgkill", "utimes", "mbind"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"set_mempolicy", "get_mempolicy"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"waitid"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"ioprio_set", "ioprio_get", "inotify_init"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"inotify_add_watch", "inotify_rm_watch"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"openat", "mkdirat", "mknodat", "fchownat", "futimesat"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"newfstatat", "unlinkat", "renameat", "linkat", "symlinkat"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"readlinkat", "fchmodat", "faccessat", "pselect6", "ppoll"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"unshare", "set_robust_list", "get_robust_list", "splice"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"tee", "sync_file_range", "vmsplice"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"utimensat", "epoll_pwait", "signalfd", "timerfd_create"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"eventfd", "fallocate", "timerfd_settime", "timerfd_gettime"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"accept4", "signalfd4", "eventfd2", "epoll_create1"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"dup3", "pipe2", "inotify_init1", "preadv", "pwritev"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"rt_tgsigqueueinfo", "recvmmsg"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"prlimit64"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"name_to_handle_at", "open_by_handle_at", "clock_adjtime"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"syncfs", "sendmmsg", "setns", "getcpu"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"process_vm_readv", "process_vm_writev", "kcmp"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"finit_module", "sched_setattr", "sched_getattr"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"renameat2", "seccomp", "getrandom", "memfd_create"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"execveat", "membarrier"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"mlock2", "copy_file_range", "preadv2", "pwritev2"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"statx", "io_pgetevents", "rseq"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"accept"}, Action: "SCMP_ACT_ALLOW"}, // For web servers
			// Block dangerous syscalls
			{Names: []string{"ptrace"}, Action: "SCMP_ACT_ERRNO"},
			{Names: []string{"mount", "umount2"}, Action: "SCMP_ACT_ERRNO"},
			{Names: []string{"reboot", "swapon", "swapoff"}, Action: "SCMP_ACT_ERRNO"},
			{Names: []string{"kexec_load", "kexec_file_load"}, Action: "SCMP_ACT_ERRNO"},
			{Names: []string{"acct"}, Action: "SCMP_ACT_ERRNO"},
			{Names: []string{"init_module", "delete_module"}, Action: "SCMP_ACT_ERRNO"},
			{Names: []string{"bpf"}, Action: "SCMP_ACT_ERRNO"},
			{Names: []string{"userfaultfd"}, Action: "SCMP_ACT_ERRNO"},
		},
	}

	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// startCleanupLoop starts the periodic cleanup goroutine
func (s *ContainerPreviewServer) startCleanupLoop() {
	s.cleanupTicker = time.NewTicker(s.config.CleanupInterval)

	go func() {
		for {
			select {
			case <-s.cleanupTicker.C:
				s.cleanupOrphanedContainers()
				s.cleanupOldContainers()
			case <-s.stopCleanup:
				s.cleanupTicker.Stop()
				return
			}
		}
	}()
}

// cleanupOrphanedContainers removes containers that are no longer tracked
func (s *ContainerPreviewServer) cleanupOrphanedContainers() {
	cmd := osexec.Command("docker", "ps", "-a", "--filter", "label=apex.preview=true", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	s.containerMu.RLock()
	trackedContainers := make(map[string]bool)
	for _, session := range s.containerSessions {
		trackedContainers[session.ContainerName] = true
	}
	s.containerMu.RUnlock()

	for _, containerName := range lines {
		if containerName == "" {
			continue
		}
		if !trackedContainers[containerName] {
			// Orphaned container - remove it
			osexec.Command("docker", "rm", "-f", containerName).Run()
		}
	}
}

// cleanupOldContainers removes containers that have exceeded max age
func (s *ContainerPreviewServer) cleanupOldContainers() {
	s.containerMu.Lock()
	defer s.containerMu.Unlock()

	now := time.Now()
	toRemove := make([]uint, 0)

	for projectID, session := range s.containerSessions {
		session.mu.RLock()
		age := now.Sub(session.StartedAt)
		session.mu.RUnlock()

		if age > s.config.MaxContainerAge {
			toRemove = append(toRemove, projectID)
			atomic.AddInt64(&s.stats.TimeoutContainers, 1)
		}
	}

	// Remove old containers outside the loop
	for _, projectID := range toRemove {
		session := s.containerSessions[projectID]
		delete(s.containerSessions, projectID)
		atomic.AddInt32(&s.stats.ActiveContainers, -1)

		go func(s *ContainerSession) {
			// Stop and remove container
			osexec.Command("docker", "stop", "-t", "5", s.ContainerID).Run()
			osexec.Command("docker", "rm", "-f", s.ContainerID).Run()
			osexec.Command("docker", "rmi", "-f", s.ImageName).Run()
			os.RemoveAll(s.TempDir)
		}(session)

		// Release port
		delete(s.portMap, projectID)
	}
}

// GetStats returns container preview statistics
func (s *ContainerPreviewServer) GetStats() *ContainerPreviewStats {
	return &ContainerPreviewStats{
		TotalContainersCreated:  atomic.LoadInt64(&s.stats.TotalContainersCreated),
		ActiveContainers:        atomic.LoadInt32(&s.stats.ActiveContainers),
		MaxConcurrentContainers: atomic.LoadInt32(&s.stats.MaxConcurrentContainers),
		FailedContainers:        atomic.LoadInt64(&s.stats.FailedContainers),
		TimeoutContainers:       atomic.LoadInt64(&s.stats.TimeoutContainers),
		TotalBuildTime:          atomic.LoadInt64(&s.stats.TotalBuildTime),
		TotalRuntime:            atomic.LoadInt64(&s.stats.TotalRuntime),
	}
}

// Cleanup cleans up all container resources
func (s *ContainerPreviewServer) Cleanup() error {
	// Stop cleanup loop
	close(s.stopCleanup)

	// Stop all containers
	s.containerMu.Lock()
	for _, session := range s.containerSessions {
		osexec.Command("docker", "stop", "-t", "2", session.ContainerID).Run()
		osexec.Command("docker", "rm", "-f", session.ContainerID).Run()
		osexec.Command("docker", "rmi", "-f", session.ImageName).Run()
		os.RemoveAll(session.TempDir)
	}
	s.containerSessions = make(map[uint]*ContainerSession)
	s.containerMu.Unlock()

	// Cleanup orphaned containers
	s.cleanupOrphanedContainers()

	// Remove temp directory
	if s.baseTempDir != "" {
		os.RemoveAll(s.baseTempDir)
	}

	return nil
}

// RefreshContainerPreview updates files in a running container preview
func (s *ContainerPreviewServer) RefreshContainerPreview(ctx context.Context, projectID uint, changedFiles []string) error {
	s.containerMu.RLock()
	session, exists := s.containerSessions[projectID]
	s.containerMu.RUnlock()

	if !exists {
		return nil // No active preview
	}

	// For container previews, we need to rebuild and restart
	// First, update files in temp directory
	var files []models.File
	if err := s.db.WithContext(ctx).Where("project_id = ? AND path IN ?", projectID, changedFiles).Find(&files).Error; err != nil {
		return err
	}

	session.mu.Lock()
	tempDir := session.TempDir
	session.mu.Unlock()

	for _, file := range files {
		filePath := filepath.Join(tempDir, file.Path)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			continue
		}
		os.WriteFile(filePath, []byte(file.Content), 0644)
	}

	// For now, we'll need to restart the container to pick up changes
	// In a production system, we could use docker cp to update files
	// and send a signal to the server to reload

	return nil
}

// Additional helper to create tar archive for docker build context
func (s *ContainerPreviewServer) createTarArchive(srcDir string) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(tw, file)
			return err
		}

		return nil
	})

	return buf, err
}

// Unexported helper to avoid unused import warnings
var _ = runtime.GOOS
