// APEX.BUILD Code Execution Handlers
// HTTP handlers for code execution API

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"apex-build/internal/execution"
	"apex-build/internal/middleware"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ExecutionHandler handles code execution requests
type ExecutionHandler struct {
	DB              *gorm.DB
	Sandbox         *execution.Sandbox
	TerminalManager *execution.TerminalManager
	ProjectsDir     string
}

// NewExecutionHandler creates a new execution handler
func NewExecutionHandler(db *gorm.DB, projectsDir string) (*ExecutionHandler, error) {
	// Create sandbox with default config
	sandbox, err := execution.NewSandbox(execution.DefaultSandboxConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox: %w", err)
	}

	// Create terminal manager
	termManager := execution.NewTerminalManager()
	termManager.StartCleanupRoutine()

	// Ensure projects directory exists
	if projectsDir == "" {
		projectsDir = filepath.Join(os.TempDir(), "apex-build-projects")
	}
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create projects directory: %w", err)
	}

	return &ExecutionHandler{
		DB:              db,
		Sandbox:         sandbox,
		TerminalManager: termManager,
		ProjectsDir:     projectsDir,
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

	// Configure sandbox
	config := execution.DefaultSandboxConfig()
	if req.Timeout > 0 && req.Timeout <= 120 {
		config.Timeout = time.Duration(req.Timeout) * time.Second
	}
	if req.Env != nil {
		config.Environment = req.Env
	}

	// Create sandbox with custom config
	sandbox, err := execution.NewSandbox(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to create execution sandbox",
			Code:    "SANDBOX_ERROR",
		})
		return
	}

	// Execute code
	ctx := context.Background()
	result, err := sandbox.Execute(ctx, req.Language, req.Code, req.Stdin)
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
		fmt.Printf("Failed to update execution record: %v\n", err)
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
		fmt.Printf("Failed to create execution record: %v\n", err)
	}

	// Configure and execute
	config := execution.DefaultSandboxConfig()
	if req.Timeout > 0 && req.Timeout <= 120 {
		config.Timeout = time.Duration(req.Timeout) * time.Second
	}

	sandbox, err := execution.NewSandbox(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to create sandbox",
			Code:    "SANDBOX_ERROR",
		})
		return
	}

	ctx := context.Background()
	result, err := sandbox.ExecuteFile(ctx, filePath, req.Args, req.Stdin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Execution failed: " + err.Error(),
			Code:    "EXECUTION_ERROR",
		})
		return
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

	// Configure sandbox
	config := execution.DefaultSandboxConfig()
	config.WorkDir = projectDir
	if req.Timeout > 0 && req.Timeout <= 300 {
		config.Timeout = time.Duration(req.Timeout) * time.Second
	} else {
		config.Timeout = 60 * time.Second // Default 60s for projects
	}
	if req.Env != nil {
		config.Environment = req.Env
	}

	sandbox, err := execution.NewSandbox(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to create sandbox",
			Code:    "SANDBOX_ERROR",
		})
		return
	}

	// Execute the run command as shell script
	ctx := context.Background()
	result, err := sandbox.Execute(ctx, "javascript", fmt.Sprintf(`
const { execSync } = require('child_process');
try {
  const output = execSync(%q, { encoding: 'utf8', cwd: %q, timeout: %d });
  console.log(output);
} catch (e) {
  console.error(e.message);
  process.exit(e.status || 1);
}
`, runCmd, projectDir, int(config.Timeout.Milliseconds())), "")

	if err != nil {
		// Fallback: try to run the entry point directly
		entryPoint := project.EntryPoint
		if entryPoint == "" {
			entryPoint = findEntryPoint(project.Language, project.Files)
		}

		if entryPoint != "" {
			entryPath := filepath.Join(projectDir, entryPoint)
			result, err = sandbox.ExecuteFile(ctx, entryPath, nil, "")
		}
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

	// Try to kill the execution
	if err := h.Sandbox.Kill(execID); err != nil {
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

	// Get active executions
	activeExecs := h.Sandbox.GetActiveExecutions()

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"user_stats":        stats,
			"terminal_stats":    termStats,
			"active_executions": activeExecs,
			"supported_languages": len(execution.GetSupportedLanguages()),
		},
	})
}

// Helper functions

func getDefaultRunCommand(language, framework, entryPoint string) string {
	switch language {
	case "javascript", "typescript":
		if framework == "next" || framework == "nextjs" {
			return "npm run dev"
		}
		if framework == "react" {
			return "npm start"
		}
		if entryPoint != "" {
			if language == "typescript" {
				return "npx ts-node " + entryPoint
			}
			return "node " + entryPoint
		}
		return "npm start"

	case "python":
		if framework == "django" {
			return "python manage.py runserver"
		}
		if framework == "flask" {
			return "flask run"
		}
		if entryPoint != "" {
			return "python3 " + entryPoint
		}
		return "python3 main.py"

	case "go":
		if entryPoint != "" {
			return "go run " + entryPoint
		}
		return "go run ."

	case "rust":
		return "cargo run"

	case "java":
		if entryPoint != "" {
			return "javac " + entryPoint + " && java " + entryPoint[:len(entryPoint)-5]
		}
		return "mvn exec:java"

	case "ruby":
		if framework == "rails" {
			return "rails server"
		}
		if entryPoint != "" {
			return "ruby " + entryPoint
		}
		return "ruby main.rb"

	case "php":
		if framework == "laravel" {
			return "php artisan serve"
		}
		if entryPoint != "" {
			return "php " + entryPoint
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
	if h.Sandbox != nil {
		return h.Sandbox.Cleanup()
	}
	return nil
}
