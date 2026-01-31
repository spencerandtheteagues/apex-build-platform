// APEX.BUILD Code Execution Handlers
// HTTP handlers for code execution API
//
// SECURITY: All code execution goes through Docker containers by default.
// Container sandboxing provides: seccomp profiles, network isolation,
// resource limits (memory, CPU, PIDs), read-only root filesystem,
// and dropped capabilities.

package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"apex-build/internal/execution"
	"apex-build/internal/middleware"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// validEntryPoint matches only safe characters for entry point file paths
// Allows: alphanumeric, dots, underscores, hyphens, and forward slashes
var validEntryPoint = regexp.MustCompile(`^[a-zA-Z0-9_./-]+$`)

// sanitizeEntryPoint validates that an entry point path contains only safe characters
// and does not include shell metacharacters that could enable command injection
func sanitizeEntryPoint(entryPoint string) string {
	if entryPoint == "" {
		return ""
	}
	// Reject paths with shell metacharacters, spaces, or control characters
	if !validEntryPoint.MatchString(entryPoint) {
		return ""
	}
	// Prevent directory traversal
	if filepath.Clean(entryPoint) != entryPoint || filepath.IsAbs(entryPoint) {
		return ""
	}
	// Additional check for .. sequences
	if len(entryPoint) >= 2 && (entryPoint[:2] == ".." || entryPoint[len(entryPoint)-2:] == "..") {
		return ""
	}
	for i := 0; i < len(entryPoint)-2; i++ {
		if entryPoint[i:i+3] == "/.." || entryPoint[i:i+3] == "../" {
			return ""
		}
	}
	return entryPoint
}

// ExecutionHandler handles code execution requests
type ExecutionHandler struct {
	DB              *gorm.DB
	SandboxFactory  *execution.SandboxFactory
	TerminalManager *execution.TerminalManager
	ProjectsDir     string
	// ContainerRequired indicates whether container sandbox is required
	// When true, execution will fail if Docker is unavailable
	ContainerRequired bool
}

// ExecutionHandlerConfig configures the execution handler
type ExecutionHandlerConfig struct {
	// ProjectsDir is the directory for project files
	ProjectsDir string
	// ForceContainer requires Docker for all code execution (recommended for production)
	ForceContainer bool
	// DisableExecution disables all code execution (useful if Docker is required but unavailable)
	DisableExecution bool
}

// DefaultExecutionHandlerConfig returns secure defaults for production
func DefaultExecutionHandlerConfig() *ExecutionHandlerConfig {
	// Check environment for configuration
	forceContainer := os.Getenv("EXECUTION_FORCE_CONTAINER") == "true" ||
		os.Getenv("ENVIRONMENT") == "production"

	return &ExecutionHandlerConfig{
		ProjectsDir:    os.Getenv("PROJECTS_DIR"),
		ForceContainer: forceContainer,
	}
}

// NewExecutionHandler creates a new execution handler with secure container sandboxing
func NewExecutionHandler(db *gorm.DB, projectsDir string) (*ExecutionHandler, error) {
	config := DefaultExecutionHandlerConfig()
	if projectsDir != "" {
		config.ProjectsDir = projectsDir
	}
	return NewExecutionHandlerWithConfig(db, config)
}

// NewExecutionHandlerWithConfig creates a new execution handler with custom configuration
func NewExecutionHandlerWithConfig(db *gorm.DB, config *ExecutionHandlerConfig) (*ExecutionHandler, error) {
	if config == nil {
		config = DefaultExecutionHandlerConfig()
	}

	// Check Docker availability first
	dockerStatus := execution.CheckDockerStatus()
	if dockerStatus.Available {
		log.Printf("SECURITY: Docker sandbox available (version: %s)", dockerStatus.Version)
		log.Println("SECURITY: Code execution will use container isolation with:")
		log.Println("          - Seccomp syscall filtering")
		log.Println("          - Network isolation (disabled by default)")
		log.Println("          - Memory limits (256MB default)")
		log.Println("          - CPU limits (0.5 cores default)")
		log.Println("          - Read-only root filesystem")
		log.Println("          - All capabilities dropped")
	} else {
		log.Printf("WARNING: Docker not available: %s", dockerStatus.Error)
		if config.ForceContainer {
			log.Println("SECURITY: Container execution is required but Docker is unavailable")
			log.Println("SECURITY: Code execution features will be DISABLED")
			config.DisableExecution = true
		} else {
			log.Println("WARNING: Falling back to process-based sandbox (less secure)")
			log.Println("WARNING: Set EXECUTION_FORCE_CONTAINER=true to require Docker")
		}
	}

	// Configure sandbox factory
	factoryConfig := &execution.SandboxFactoryConfig{
		PreferContainer: true, // Always prefer container when available
		ForceContainer:  config.ForceContainer,
		ContainerConfig: execution.DefaultContainerSandboxConfig(),
		ProcessConfig:   execution.DefaultSandboxConfig(),
	}

	// Create sandbox factory
	sandboxFactory, err := execution.NewSandboxFactory(factoryConfig)
	if err != nil {
		if config.ForceContainer {
			// If container is required and unavailable, create handler with disabled execution
			log.Printf("SECURITY: Sandbox factory failed: %v", err)
			log.Println("SECURITY: Code execution is DISABLED due to security requirements")

			// Still create the handler but with execution disabled
			termManager := execution.NewTerminalManager()
			termManager.StartCleanupRoutine()

			projectsPath := config.ProjectsDir
			if projectsPath == "" {
				projectsPath = filepath.Join(os.TempDir(), "apex-build-projects")
			}
			os.MkdirAll(projectsPath, 0755)

			return &ExecutionHandler{
				DB:                db,
				SandboxFactory:    nil, // No sandbox available
				TerminalManager:   termManager,
				ProjectsDir:       projectsPath,
				ContainerRequired: true,
			}, nil
		}
		return nil, fmt.Errorf("failed to create sandbox factory: %w", err)
	}

	// Log sandbox capabilities
	caps := sandboxFactory.GetCapabilities()
	log.Printf("SECURITY: Sandbox capabilities:")
	log.Printf("          - Container isolation: %v", caps.ContainerIsolation)
	log.Printf("          - Network isolation: %v", caps.NetworkIsolation)
	log.Printf("          - Seccomp enabled: %v", caps.SeccompEnabled)
	log.Printf("          - Read-only root: %v", caps.ReadOnlyRoot)
	log.Printf("          - Resource limits: %v", caps.ResourceLimits)

	// Create terminal manager
	termManager := execution.NewTerminalManager()
	termManager.StartCleanupRoutine()

	// Ensure projects directory exists
	projectsPath := config.ProjectsDir
	if projectsPath == "" {
		projectsPath = filepath.Join(os.TempDir(), "apex-build-projects")
	}
	if err := os.MkdirAll(projectsPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create projects directory: %w", err)
	}

	return &ExecutionHandler{
		DB:                db,
		SandboxFactory:    sandboxFactory,
		TerminalManager:   termManager,
		ProjectsDir:       projectsPath,
		ContainerRequired: config.ForceContainer,
	}, nil
}

// ExecuteCodeRequest represents a code execution request
type ExecuteCodeRequest struct {
	Code      string            `json:"code" binding:"required"`
	Language  string            `json:"language" binding:"required"`
	Stdin     string            `json:"stdin"`
	Timeout   int               `json:"timeout"`   // Timeout in seconds
	Env       map[string]string `json:"env"`       // Environment variables
	ProjectID uint              `json:"project_id"`
}

// ExecuteCode handles POST /api/v1/execute
// SECURITY: All code execution uses Docker container sandboxing by default
func (h *ExecutionHandler) ExecuteCode(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	// SECURITY: Check if execution is available
	if h.SandboxFactory == nil {
		c.JSON(http.StatusServiceUnavailable, StandardResponse{
			Success: false,
			Error:   "Code execution is currently disabled for security reasons. Docker container sandbox is required but unavailable.",
			Code:    "EXECUTION_DISABLED",
		})
		return
	}

	var req ExecuteCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format: " + err.Error(),
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Validate language
	if _, err := execution.GetRunner(req.Language); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   err.Error(),
			Code:    "UNSUPPORTED_LANGUAGE",
		})
		return
	}

	// Create execution record
	execRecord := &models.Execution{
		ExecutionID: uuid.New().String(),
		UserID:      userID,
		Language:    req.Language,
		Command:     fmt.Sprintf("execute %s code", req.Language),
		Status:      "running",
		StartedAt:   time.Now(),
	}

	if req.ProjectID > 0 {
		execRecord.ProjectID = req.ProjectID
	}

	if err := h.DB.Create(execRecord).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to create execution record",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Get the appropriate executor from factory (prefers container sandbox)
	// SECURITY: Container sandbox is always preferred when available
	var executor execution.CodeExecutor
	var err error

	if h.ContainerRequired {
		// SECURITY: In production, require container sandbox
		executor, err = h.SandboxFactory.GetExecutor(execution.SandboxTypeContainer)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, StandardResponse{
				Success: false,
				Error:   "Secure container execution is required but unavailable",
				Code:    "CONTAINER_REQUIRED",
			})
			return
		}
	} else {
		// Use auto mode which prefers container but falls back to process
		executor, err = h.SandboxFactory.GetExecutor(execution.SandboxTypeAuto)
		if err != nil {
			c.JSON(http.StatusInternalServerError, StandardResponse{
				Success: false,
				Error:   "Failed to get execution sandbox: " + err.Error(),
				Code:    "SANDBOX_ERROR",
			})
			return
		}
	}

	// Create execution context with timeout
	timeout := 30 * time.Second
	if req.Timeout > 0 && req.Timeout <= 120 {
		timeout = time.Duration(req.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Execute code using the secure sandbox
	result, err := executor.Execute(ctx, req.Language, req.Code, req.Stdin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Execution failed: " + err.Error(),
			Code:    "EXECUTION_ERROR",
		})
		return
	}

	// Update execution record
	execRecord.Output = result.Output
	execRecord.ErrorOut = result.ErrorOutput
	execRecord.ExitCode = result.ExitCode
	execRecord.Status = result.Status
	execRecord.Duration = result.DurationMs
	execRecord.MemoryUsed = result.MemoryUsed
	execRecord.CPUTime = result.CPUTime
	if result.CompletedAt != nil {
		execRecord.CompletedAt = result.CompletedAt
	}

	if err := h.DB.Save(execRecord).Error; err != nil {
		// Log error but don't fail the request
		log.Printf("Failed to update execution record: %v", err)
	}

	// Include sandbox info in response for transparency
	sandboxInfo := "container"
	if !h.SandboxFactory.IsContainerAvailable() {
		sandboxInfo = "process"
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"id":             execRecord.ExecutionID,
			"status":         result.Status,
			"output":         result.Output,
			"error_output":   result.ErrorOutput,
			"exit_code":      result.ExitCode,
			"duration_ms":    result.DurationMs,
			"memory_used":    result.MemoryUsed,
			"cpu_time_ms":    result.CPUTime,
			"timed_out":      result.TimedOut,
			"killed":         result.Killed,
			"compile_error":  result.CompileError,
			"language":       result.Language,
			"started_at":     result.StartedAt,
			"completed_at":   result.CompletedAt,
			"sandbox_type":   sandboxInfo,
		},
	})
}

// ExecuteFileRequest represents a file execution request
type ExecuteFileRequest struct {
	FileID    uint     `json:"file_id" binding:"required"`
	Args      []string `json:"args"`
	Stdin     string   `json:"stdin"`
	Timeout   int      `json:"timeout"`
}

// ExecuteFile handles POST /api/v1/execute/file
// SECURITY: File execution uses Docker container sandboxing by default
func (h *ExecutionHandler) ExecuteFile(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	// SECURITY: Check if execution is available
	if h.SandboxFactory == nil {
		c.JSON(http.StatusServiceUnavailable, StandardResponse{
			Success: false,
			Error:   "Code execution is currently disabled for security reasons. Docker container sandbox is required but unavailable.",
			Code:    "EXECUTION_DISABLED",
		})
		return
	}

	var req ExecuteFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Get file from database
	var file models.File
	if err := h.DB.Preload("Project").First(&file, req.FileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "File not found",
				Code:    "FILE_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Check ownership
	if file.Project.OwnerID != userID {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Access denied",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	// Create temp file with content
	tempDir := filepath.Join(h.ProjectsDir, fmt.Sprintf("exec-%d", req.FileID))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to create temp directory",
			Code:    "SYSTEM_ERROR",
		})
		return
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, file.Name)
	if err := os.WriteFile(filePath, []byte(file.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to write file",
			Code:    "SYSTEM_ERROR",
		})
		return
	}

	// Create execution record
	execRecord := &models.Execution{
		ExecutionID: uuid.New().String(),
		ProjectID:   file.ProjectID,
		UserID:      userID,
		Language:    file.Project.Language,
		Command:     fmt.Sprintf("execute %s", file.Name),
		Status:      "running",
		StartedAt:   time.Now(),
	}

	if err := h.DB.Create(execRecord).Error; err != nil {
		// Continue even if record creation fails
		log.Printf("Failed to create execution record: %v", err)
	}

	// Get executor from factory
	// SECURITY: For file execution, we need to read file content and use container sandbox
	var executor execution.CodeExecutor
	var err error

	if h.ContainerRequired {
		executor, err = h.SandboxFactory.GetExecutor(execution.SandboxTypeContainer)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, StandardResponse{
				Success: false,
				Error:   "Secure container execution is required but unavailable",
				Code:    "CONTAINER_REQUIRED",
			})
			return
		}
	} else {
		executor, err = h.SandboxFactory.GetExecutor(execution.SandboxTypeAuto)
		if err != nil {
			c.JSON(http.StatusInternalServerError, StandardResponse{
				Success: false,
				Error:   "Failed to get execution sandbox",
				Code:    "SANDBOX_ERROR",
			})
			return
		}
	}

	// Create execution context with timeout
	timeout := 30 * time.Second
	if req.Timeout > 0 && req.Timeout <= 120 {
		timeout = time.Duration(req.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Execute file - for container sandbox, we read the file content and execute as code
	// since container sandbox doesn't support direct file execution
	result, err := executor.ExecuteFile(ctx, filePath, req.Args, req.Stdin)
	if err != nil {
		// If file execution not supported (container sandbox), fall back to Execute with code
		if err.Error() == "file execution not supported in container sandbox - use Execute with code content" {
			// Read file content and execute as code
			result, err = executor.Execute(ctx, file.Project.Language, file.Content, req.Stdin)
			if err != nil {
				c.JSON(http.StatusInternalServerError, StandardResponse{
					Success: false,
					Error:   "Execution failed: " + err.Error(),
					Code:    "EXECUTION_ERROR",
				})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, StandardResponse{
				Success: false,
				Error:   "Execution failed: " + err.Error(),
				Code:    "EXECUTION_ERROR",
			})
			return
		}
	}

	// Update record
	if execRecord.ID > 0 {
		execRecord.Output = result.Output
		execRecord.ErrorOut = result.ErrorOutput
		execRecord.ExitCode = result.ExitCode
		execRecord.Status = result.Status
		execRecord.Duration = result.DurationMs
		execRecord.MemoryUsed = result.MemoryUsed
		h.DB.Save(execRecord)
	}

	// Include sandbox info
	sandboxInfo := "container"
	if !h.SandboxFactory.IsContainerAvailable() {
		sandboxInfo = "process"
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"id":            result.ID,
			"status":        result.Status,
			"output":        result.Output,
			"error_output":  result.ErrorOutput,
			"exit_code":     result.ExitCode,
			"duration_ms":   result.DurationMs,
			"memory_used":   result.MemoryUsed,
			"timed_out":     result.TimedOut,
			"compile_error": result.CompileError,
			"sandbox_type":  sandboxInfo,
		},
	})
}

// ExecuteProjectRequest represents a project execution request
type ExecuteProjectRequest struct {
	ProjectID uint              `json:"project_id" binding:"required"`
	Command   string            `json:"command"` // Optional: custom run command
	Env       map[string]string `json:"env"`
	Timeout   int               `json:"timeout"`
}

// ExecuteProject handles POST /api/v1/execute/project
// SECURITY: Project execution uses Docker container sandboxing by default
func (h *ExecutionHandler) ExecuteProject(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	// SECURITY: Check if execution is available
	if h.SandboxFactory == nil {
		c.JSON(http.StatusServiceUnavailable, StandardResponse{
			Success: false,
			Error:   "Code execution is currently disabled for security reasons. Docker container sandbox is required but unavailable.",
			Code:    "EXECUTION_DISABLED",
		})
		return
	}

	var req ExecuteProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Get project
	var project models.Project
	if err := h.DB.Preload("Files").First(&project, req.ProjectID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Project not found",
				Code:    "PROJECT_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Check ownership
	if project.OwnerID != userID {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Access denied",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	// Create project directory
	projectDir := filepath.Join(h.ProjectsDir, fmt.Sprintf("project-%d-%s", project.ID, uuid.New().String()[:8]))
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to create project directory",
			Code:    "SYSTEM_ERROR",
		})
		return
	}
	defer os.RemoveAll(projectDir)

	// Write all project files
	for _, file := range project.Files {
		if file.Type == "directory" {
			os.MkdirAll(filepath.Join(projectDir, file.Path), 0755)
			continue
		}

		filePath := filepath.Join(projectDir, file.Path)
		fileDir := filepath.Dir(filePath)
		if err := os.MkdirAll(fileDir, 0755); err != nil {
			continue
		}
		os.WriteFile(filePath, []byte(file.Content), 0644)
	}

	// Determine run command
	runCmd := req.Command
	if runCmd == "" {
		runCmd = getDefaultRunCommand(project.Language, project.Framework, project.EntryPoint)
	}

	// Create execution record
	execRecord := &models.Execution{
		ExecutionID: uuid.New().String(),
		ProjectID:   project.ID,
		UserID:      userID,
		Language:    project.Language,
		Command:     runCmd,
		Status:      "running",
		StartedAt:   time.Now(),
	}
	if req.Env != nil {
		execRecord.Environment = make(map[string]interface{})
		for k, v := range req.Env {
			execRecord.Environment[k] = v
		}
	}
	h.DB.Create(execRecord)

	// Get executor from factory
	var executor execution.CodeExecutor
	var err error

	if h.ContainerRequired {
		executor, err = h.SandboxFactory.GetExecutor(execution.SandboxTypeContainer)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, StandardResponse{
				Success: false,
				Error:   "Secure container execution is required but unavailable",
				Code:    "CONTAINER_REQUIRED",
			})
			return
		}
	} else {
		executor, err = h.SandboxFactory.GetExecutor(execution.SandboxTypeAuto)
		if err != nil {
			c.JSON(http.StatusInternalServerError, StandardResponse{
				Success: false,
				Error:   "Failed to get execution sandbox",
				Code:    "SANDBOX_ERROR",
			})
			return
		}
	}

	// Create execution context with timeout
	timeout := 60 * time.Second // Default 60s for projects
	if req.Timeout > 0 && req.Timeout <= 300 {
		timeout = time.Duration(req.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Find entry point
	entryPoint := project.EntryPoint
	if entryPoint == "" {
		entryPoint = findEntryPoint(project.Language, project.Files)
	}

	// Execute the entry point file
	var result *execution.ExecutionResult
	if entryPoint != "" {
		// Find the entry point file content
		var entryContent string
		for _, file := range project.Files {
			if file.Path == entryPoint || file.Name == entryPoint {
				entryContent = file.Content
				break
			}
		}

		if entryContent != "" {
			// Execute the entry point content in the container
			result, err = executor.Execute(ctx, project.Language, entryContent, "")
		} else {
			err = fmt.Errorf("entry point file not found: %s", entryPoint)
		}
	} else {
		// No entry point found, try to find main file
		err = fmt.Errorf("no entry point found for project")
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Project execution failed: " + err.Error(),
			Code:    "EXECUTION_ERROR",
		})
		return
	}

	// Update record
	execRecord.Output = result.Output
	execRecord.ErrorOut = result.ErrorOutput
	execRecord.ExitCode = result.ExitCode
	execRecord.Status = result.Status
	execRecord.Duration = result.DurationMs
	execRecord.MemoryUsed = result.MemoryUsed
	h.DB.Save(execRecord)

	// Include sandbox info
	sandboxInfo := "container"
	if !h.SandboxFactory.IsContainerAvailable() {
		sandboxInfo = "process"
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"id":           result.ID,
			"status":       result.Status,
			"output":       result.Output,
			"error_output": result.ErrorOutput,
			"exit_code":    result.ExitCode,
			"duration_ms":  result.DurationMs,
			"memory_used":  result.MemoryUsed,
			"timed_out":    result.TimedOut,
			"command":      runCmd,
			"sandbox_type": sandboxInfo,
		},
	})
}

// GetLanguages handles GET /api/v1/execute/languages
func (h *ExecutionHandler) GetLanguages(c *gin.Context) {
	languages := execution.GetSupportedLanguages()

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"languages": languages,
			"total":     len(languages),
		},
	})
}

// GetExecution handles GET /api/v1/execute/:id
func (h *ExecutionHandler) GetExecution(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	execID := c.Param("id")

	var exec models.Execution
	if err := h.DB.Where("execution_id = ? AND user_id = ?", execID, userID).First(&exec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Execution not found",
				Code:    "NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data:    exec,
	})
}

// GetExecutionHistory handles GET /api/v1/execute/history
func (h *ExecutionHandler) GetExecutionHistory(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	page, limit := parsePaginationParams(c)

	// Optional project filter
	projectIDStr := c.Query("project_id")
	var projectID uint
	if projectIDStr != "" {
		if id, err := strconv.ParseUint(projectIDStr, 10, 32); err == nil {
			projectID = uint(id)
		}
	}

	// Build query
	query := h.DB.Model(&models.Execution{}).Where("user_id = ?", userID)
	if projectID > 0 {
		query = query.Where("project_id = ?", projectID)
	}

	// Get total count
	var total int64
	query.Count(&total)

	// Get executions
	var executions []models.Execution
	if err := query.Order("created_at DESC").Scopes(paginate(page, limit)).Find(&executions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, PaginatedResponse{
		StandardResponse: StandardResponse{
			Success: true,
			Data:    executions,
		},
		Pagination: getPaginationInfo(page, limit, total),
	})
}

// StopExecution handles POST /api/v1/execute/:id/stop
func (h *ExecutionHandler) StopExecution(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	execID := c.Param("id")

	// Verify ownership
	var exec models.Execution
	if err := h.DB.Where("execution_id = ? AND user_id = ?", execID, userID).First(&exec).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Execution not found",
			Code:    "NOT_FOUND",
		})
		return
	}

	// SECURITY: Check if execution is available
	if h.SandboxFactory == nil {
		c.JSON(http.StatusServiceUnavailable, StandardResponse{
			Success: false,
			Error:   "Code execution is currently disabled",
			Code:    "EXECUTION_DISABLED",
		})
		return
	}

	// Try to kill the execution using the factory
	executor, err := h.SandboxFactory.GetExecutor(execution.SandboxTypeAuto)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to get executor",
			Code:    "SANDBOX_ERROR",
		})
		return
	}

	if err := executor.Kill(execID); err != nil {
		// Execution might already be completed
		c.JSON(http.StatusOK, StandardResponse{
			Success: true,
			Message: "Execution already completed or not found",
		})
		return
	}

	// Update status
	exec.Status = "killed"
	now := time.Now()
	exec.CompletedAt = &now
	h.DB.Save(&exec)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Execution stopped",
	})
}

// Terminal handlers

// CreateTerminalSession handles POST /api/v1/terminal/sessions
func (h *ExecutionHandler) CreateTerminalSession(c *gin.Context) {
	h.TerminalManager.CreateSession(0, 0, "")
	handler := execution.NewTerminalHandler(h.TerminalManager)
	handler.CreateSessionHandler(c)
}

// GetTerminalSession handles GET /api/v1/terminal/sessions/:id
func (h *ExecutionHandler) GetTerminalSession(c *gin.Context) {
	handler := execution.NewTerminalHandler(h.TerminalManager)
	handler.GetSessionHandler(c)
}

// ListTerminalSessions handles GET /api/v1/terminal/sessions
func (h *ExecutionHandler) ListTerminalSessions(c *gin.Context) {
	handler := execution.NewTerminalHandler(h.TerminalManager)
	handler.ListSessionsHandler(c)
}

// DeleteTerminalSession handles DELETE /api/v1/terminal/sessions/:id
func (h *ExecutionHandler) DeleteTerminalSession(c *gin.Context) {
	handler := execution.NewTerminalHandler(h.TerminalManager)
	handler.DeleteSessionHandler(c)
}

// ResizeTerminalSession handles POST /api/v1/terminal/sessions/:id/resize
func (h *ExecutionHandler) ResizeTerminalSession(c *gin.Context) {
	handler := execution.NewTerminalHandler(h.TerminalManager)
	handler.ResizeSessionHandler(c)
}

// GetTerminalHistory handles GET /api/v1/terminal/sessions/:id/history
func (h *ExecutionHandler) GetTerminalHistory(c *gin.Context) {
	handler := execution.NewTerminalHandler(h.TerminalManager)
	handler.GetHistoryHandler(c)
}

// GetAvailableShells handles GET /api/v1/terminal/shells
func (h *ExecutionHandler) GetAvailableShells(c *gin.Context) {
	handler := execution.NewTerminalHandler(h.TerminalManager)
	handler.GetAvailableShellsHandler(c)
}

// HandleTerminalWebSocket handles WebSocket /ws/terminal/:sessionId
func (h *ExecutionHandler) HandleTerminalWebSocket(c *gin.Context) {
	handler := execution.NewTerminalHandler(h.TerminalManager)
	handler.WebSocketHandler(c)
}

// GetExecutionStats handles GET /api/v1/execute/stats
func (h *ExecutionHandler) GetExecutionStats(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	// Get user stats
	var stats struct {
		TotalExecutions  int64 `json:"total_executions"`
		SuccessfulExecs  int64 `json:"successful_executions"`
		FailedExecs      int64 `json:"failed_executions"`
		TimeoutExecs     int64 `json:"timeout_executions"`
		TotalCPUTime     int64 `json:"total_cpu_time_ms"`
		TotalMemoryUsed  int64 `json:"total_memory_used_bytes"`
		AvgExecutionTime float64 `json:"avg_execution_time_ms"`
	}

	h.DB.Model(&models.Execution{}).Where("user_id = ?", userID).Count(&stats.TotalExecutions)
	h.DB.Model(&models.Execution{}).Where("user_id = ? AND status = ?", userID, "completed").Count(&stats.SuccessfulExecs)
	h.DB.Model(&models.Execution{}).Where("user_id = ? AND status = ?", userID, "failed").Count(&stats.FailedExecs)
	h.DB.Model(&models.Execution{}).Where("user_id = ? AND status = ?", userID, "timeout").Count(&stats.TimeoutExecs)

	h.DB.Model(&models.Execution{}).Where("user_id = ?", userID).Select("COALESCE(SUM(cpu_time), 0)").Scan(&stats.TotalCPUTime)
	h.DB.Model(&models.Execution{}).Where("user_id = ?", userID).Select("COALESCE(SUM(memory_used), 0)").Scan(&stats.TotalMemoryUsed)
	h.DB.Model(&models.Execution{}).Where("user_id = ?", userID).Select("COALESCE(AVG(duration), 0)").Scan(&stats.AvgExecutionTime)

	// Get terminal stats
	termStats := h.TerminalManager.GetStats()

	// Get sandbox stats and capabilities
	var sandboxStats map[string]interface{}
	var capabilities execution.SandboxCapabilities
	var activeExecs int

	if h.SandboxFactory != nil {
		sandboxStats = h.SandboxFactory.GetStats()
		capabilities = h.SandboxFactory.GetCapabilities()

		// Get active executions from the appropriate executor
		if executor, err := h.SandboxFactory.GetExecutor(execution.SandboxTypeAuto); err == nil {
			activeExecs = executor.GetActiveExecutions()
		}
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"user_stats":          stats,
			"terminal_stats":      termStats,
			"active_executions":   activeExecs,
			"supported_languages": len(execution.GetSupportedLanguages()),
			"sandbox_stats":       sandboxStats,
			"sandbox_capabilities": map[string]interface{}{
				"container_isolation": capabilities.ContainerIsolation,
				"network_isolation":   capabilities.NetworkIsolation,
				"seccomp_enabled":     capabilities.SeccompEnabled,
				"read_only_root":      capabilities.ReadOnlyRoot,
				"resource_limits":     capabilities.ResourceLimits,
			},
			"execution_enabled": h.SandboxFactory != nil,
		},
	})
}

// GetSandboxStatusHandler handles GET /api/v1/execute/sandbox/status
// Returns detailed information about sandbox security status
func (h *ExecutionHandler) GetSandboxStatusHandler(c *gin.Context) {
	status := h.GetSandboxStatus()

	// Add security recommendations if container is not available
	if h.SandboxFactory == nil || !h.SandboxFactory.IsContainerAvailable() {
		status["security_warning"] = "Container sandbox is not available. Code execution is using process-based isolation which provides less security."
		status["recommendations"] = []string{
			"Install Docker to enable container sandboxing",
			"Set EXECUTION_FORCE_CONTAINER=true to require Docker",
			"In production, always use container isolation",
		}
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data:    status,
	})
}

// Helper functions

func getDefaultRunCommand(language, framework, entryPoint string) string {
	// Sanitize entryPoint to prevent command injection
	safeEntryPoint := sanitizeEntryPoint(entryPoint)

	switch language {
	case "javascript", "typescript":
		if framework == "next" || framework == "nextjs" {
			return "npm run dev"
		}
		if framework == "react" {
			return "npm start"
		}
		if safeEntryPoint != "" {
			if language == "typescript" {
				return "npx ts-node " + safeEntryPoint
			}
			return "node " + safeEntryPoint
		}
		return "npm start"

	case "python":
		if framework == "django" {
			return "python manage.py runserver"
		}
		if framework == "flask" {
			return "flask run"
		}
		if safeEntryPoint != "" {
			return "python3 " + safeEntryPoint
		}
		return "python3 main.py"

	case "go":
		if safeEntryPoint != "" {
			return "go run " + safeEntryPoint
		}
		return "go run ."

	case "rust":
		return "cargo run"

	case "java":
		if safeEntryPoint != "" && len(safeEntryPoint) > 5 {
			return "javac " + safeEntryPoint + " && java " + safeEntryPoint[:len(safeEntryPoint)-5]
		}
		return "mvn exec:java"

	case "ruby":
		if framework == "rails" {
			return "rails server"
		}
		if safeEntryPoint != "" {
			return "ruby " + safeEntryPoint
		}
		return "ruby main.rb"

	case "php":
		if framework == "laravel" {
			return "php artisan serve"
		}
		if safeEntryPoint != "" {
			return "php " + safeEntryPoint
		}
		return "php -S localhost:8000"

	default:
		return "echo 'No run command configured'"
	}
}

func findEntryPoint(language string, files []models.File) string {
	// Common entry point patterns by language
	entryPoints := map[string][]string{
		"javascript": {"index.js", "main.js", "app.js", "server.js"},
		"typescript": {"index.ts", "main.ts", "app.ts", "server.ts"},
		"python":     {"main.py", "app.py", "__main__.py", "run.py"},
		"go":         {"main.go"},
		"rust":       {"main.rs", "src/main.rs"},
		"java":       {"Main.java", "App.java"},
		"ruby":       {"main.rb", "app.rb"},
		"php":        {"index.php", "main.php", "app.php"},
		"c":          {"main.c"},
		"cpp":        {"main.cpp", "main.cc"},
	}

	patterns, ok := entryPoints[language]
	if !ok {
		return ""
	}

	for _, pattern := range patterns {
		for _, file := range files {
			if file.Path == pattern || file.Name == pattern {
				return file.Path
			}
		}
	}

	return ""
}

// Cleanup cleans up handler resources
func (h *ExecutionHandler) Cleanup() error {
	if h.SandboxFactory != nil {
		return h.SandboxFactory.Cleanup()
	}
	return nil
}

// GetSandboxStatus returns the current sandbox status for health checks
func (h *ExecutionHandler) GetSandboxStatus() map[string]interface{} {
	status := map[string]interface{}{
		"execution_enabled": h.SandboxFactory != nil,
		"container_required": h.ContainerRequired,
	}

	if h.SandboxFactory != nil {
		caps := h.SandboxFactory.GetCapabilities()
		status["container_available"] = h.SandboxFactory.IsContainerAvailable()
		status["container_isolation"] = caps.ContainerIsolation
		status["network_isolation"] = caps.NetworkIsolation
		status["seccomp_enabled"] = caps.SeccompEnabled
		status["supported_languages"] = caps.SupportedLanguages
	}

	// Check Docker status directly
	dockerStatus := execution.CheckDockerStatus()
	status["docker"] = map[string]interface{}{
		"available":   dockerStatus.Available,
		"version":     dockerStatus.Version,
		"api_version": dockerStatus.APIVersion,
		"error":       dockerStatus.Error,
	}

	return status
}
