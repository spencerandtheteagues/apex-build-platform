// APEX.BUILD Environment Configuration Handlers
// Nix-like reproducible environment system for project dependencies
// Provides Replit parity for development environment configuration

package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"apex-build/internal/middleware"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
)

// EnvironmentHandler handles environment configuration endpoints
type EnvironmentHandler struct {
	*Handler
}

// NewEnvironmentHandler creates a new environment handler
func NewEnvironmentHandler(h *Handler) *EnvironmentHandler {
	return &EnvironmentHandler{
		Handler: h,
	}
}

// RegisterEnvironmentRoutes registers all environment management routes
func (h *EnvironmentHandler) RegisterEnvironmentRoutes(rg *gin.RouterGroup) {
	env := rg.Group("/environment")
	{
		// Get environment config for a project
		env.GET("/project/:projectId", h.GetEnvironmentConfig)

		// Update environment config for a project
		env.PUT("/project/:projectId", h.UpdateEnvironmentConfig)

		// Auto-detect environment from project files
		env.POST("/project/:projectId/detect", h.DetectEnvironment)

		// Get available language runtimes
		env.GET("/runtimes", h.GetAvailableRuntimes)

		// Get packages for a runtime
		env.GET("/packages/:runtime", h.GetAvailablePackages)

		// Validate environment config
		env.POST("/validate", h.ValidateEnvironmentConfig)

		// Get environment presets (templates)
		env.GET("/presets", h.GetEnvironmentPresets)

		// Apply a preset to a project
		env.POST("/project/:projectId/preset/:presetId", h.ApplyEnvironmentPreset)
	}
}

// EnvironmentConfig represents the project environment configuration
// Stored as JSON in the Project.EnvironmentConfig field
type EnvironmentConfig struct {
	// Language runtime configuration
	Language string `json:"language"`          // node, python, go, rust, java, ruby, php
	Version  string `json:"version"`           // Runtime version (e.g., "20", "3.11", "1.21")

	// Package dependencies
	Packages    []PackageDependency `json:"packages"`     // Language-specific packages
	DevPackages []PackageDependency `json:"dev_packages"` // Development-only packages

	// System dependencies (like Nix packages)
	System []string `json:"system"` // System packages (git, curl, ffmpeg, etc.)

	// Environment variables (non-secret)
	EnvVars map[string]string `json:"env_vars"`

	// Build configuration
	BuildCommand   string `json:"build_command,omitempty"`   // Custom build command
	StartCommand   string `json:"start_command,omitempty"`   // Custom start command
	InstallCommand string `json:"install_command,omitempty"` // Custom install command

	// Runtime options
	Options map[string]interface{} `json:"options,omitempty"` // Runtime-specific options
}

// PackageDependency represents a package dependency
type PackageDependency struct {
	Name    string `json:"name"`              // Package name
	Version string `json:"version,omitempty"` // Version constraint (semver)
	Source  string `json:"source,omitempty"`  // Package source (npm, pypi, etc.)
}

// GetEnvironmentConfig retrieves the environment configuration for a project
// GET /api/v1/environment/project/:projectId
func (h *EnvironmentHandler) GetEnvironmentConfig(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	// Get project with access check
	var project models.Project
	if err := h.DB.Where("id = ? AND (owner_id = ? OR is_public = ?)", uint(projectID), userID, true).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Project not found or access denied",
			Code:    "PROJECT_NOT_FOUND",
		})
		return
	}

	// Parse environment config from project
	envConfig := parseEnvironmentConfig(&project)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"project_id":  projectID,
			"environment": envConfig,
		},
	})
}

// UpdateEnvironmentConfig updates the environment configuration for a project
// PUT /api/v1/environment/project/:projectId
func (h *EnvironmentHandler) UpdateEnvironmentConfig(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	// Verify user owns the project
	var project models.Project
	if err := h.DB.Where("id = ? AND owner_id = ?", uint(projectID), userID).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Project not found or access denied",
			Code:    "PROJECT_NOT_FOUND",
		})
		return
	}

	var req EnvironmentConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid environment configuration format",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Validate environment config
	if validationErr := validateEnvironmentConfig(&req); validationErr != "" {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   validationErr,
			Code:    "VALIDATION_ERROR",
		})
		return
	}

	// Serialize config to JSON and store in project
	envConfigJSON, err := json.Marshal(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to serialize environment configuration",
			Code:    "SERIALIZATION_ERROR",
		})
		return
	}

	// Update project with new environment config
	updates := map[string]interface{}{
		"environment_config": string(envConfigJSON),
	}

	// Also update the project language if changed
	if req.Language != "" {
		updates["language"] = req.Language
	}

	if err := h.DB.Model(&project).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to update environment configuration",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Environment configuration updated successfully",
		Data: map[string]interface{}{
			"project_id":  projectID,
			"environment": req,
		},
	})
}

// DetectEnvironment auto-detects environment from project files
// POST /api/v1/environment/project/:projectId/detect
func (h *EnvironmentHandler) DetectEnvironment(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	// Get project
	var project models.Project
	if err := h.DB.Where("id = ? AND owner_id = ?", uint(projectID), userID).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Project not found or access denied",
			Code:    "PROJECT_NOT_FOUND",
		})
		return
	}

	// Get project files for detection
	var files []models.File
	h.DB.Where("project_id = ? AND type = ?", projectID, "file").Find(&files)

	// Detect environment from files
	detectedEnv := detectEnvironmentFromFiles(files, &project)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Environment detected successfully",
		Data: map[string]interface{}{
			"project_id":  projectID,
			"detected":    detectedEnv,
			"confidence":  calculateDetectionConfidence(files, detectedEnv),
			"suggestions": generateEnvironmentSuggestions(detectedEnv),
		},
	})
}

// GetAvailableRuntimes returns all available language runtimes
// GET /api/v1/environment/runtimes
func (h *EnvironmentHandler) GetAvailableRuntimes(c *gin.Context) {
	runtimes := []RuntimeInfo{
		{
			ID:          "node",
			Name:        "Node.js",
			Description: "JavaScript runtime built on Chrome's V8 engine",
			Versions:    []string{"22", "20", "18", "16"},
			Default:     "20",
			PackageManager: "npm",
			Icon:        "nodejs",
		},
		{
			ID:          "python",
			Name:        "Python",
			Description: "Versatile programming language for web, data science, and more",
			Versions:    []string{"3.12", "3.11", "3.10", "3.9"},
			Default:     "3.11",
			PackageManager: "pip",
			Icon:        "python",
		},
		{
			ID:          "go",
			Name:        "Go",
			Description: "Fast, efficient compiled language by Google",
			Versions:    []string{"1.22", "1.21", "1.20"},
			Default:     "1.21",
			PackageManager: "go",
			Icon:        "go",
		},
		{
			ID:          "rust",
			Name:        "Rust",
			Description: "Memory-safe systems programming language",
			Versions:    []string{"1.75", "1.74", "1.73"},
			Default:     "1.75",
			PackageManager: "cargo",
			Icon:        "rust",
		},
		{
			ID:          "java",
			Name:        "Java",
			Description: "Enterprise-grade, platform-independent language",
			Versions:    []string{"21", "17", "11"},
			Default:     "17",
			PackageManager: "maven",
			Icon:        "java",
		},
		{
			ID:          "ruby",
			Name:        "Ruby",
			Description: "Dynamic, elegant language optimized for developer happiness",
			Versions:    []string{"3.3", "3.2", "3.1"},
			Default:     "3.2",
			PackageManager: "gem",
			Icon:        "ruby",
		},
		{
			ID:          "php",
			Name:        "PHP",
			Description: "Popular server-side scripting language",
			Versions:    []string{"8.3", "8.2", "8.1"},
			Default:     "8.2",
			PackageManager: "composer",
			Icon:        "php",
		},
		{
			ID:          "deno",
			Name:        "Deno",
			Description: "Secure runtime for JavaScript and TypeScript",
			Versions:    []string{"1.40", "1.39", "1.38"},
			Default:     "1.40",
			PackageManager: "deno",
			Icon:        "deno",
		},
		{
			ID:          "bun",
			Name:        "Bun",
			Description: "Fast all-in-one JavaScript runtime",
			Versions:    []string{"1.0", "0.8"},
			Default:     "1.0",
			PackageManager: "bun",
			Icon:        "bun",
		},
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"runtimes": runtimes,
		},
	})
}

// RuntimeInfo represents information about a language runtime
type RuntimeInfo struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Versions       []string `json:"versions"`
	Default        string   `json:"default"`
	PackageManager string   `json:"package_manager"`
	Icon           string   `json:"icon"`
}

// GetAvailablePackages returns common packages for a runtime
// GET /api/v1/environment/packages/:runtime
func (h *EnvironmentHandler) GetAvailablePackages(c *gin.Context) {
	runtime := c.Param("runtime")

	packages := getCommonPackagesForRuntime(runtime)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"runtime":  runtime,
			"packages": packages,
		},
	})
}

// ValidateEnvironmentConfig validates an environment configuration
// POST /api/v1/environment/validate
func (h *EnvironmentHandler) ValidateEnvironmentConfig(c *gin.Context) {
	var req EnvironmentConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	validationErr := validateEnvironmentConfig(&req)
	if validationErr != "" {
		c.JSON(http.StatusOK, StandardResponse{
			Success: true,
			Data: map[string]interface{}{
				"valid":  false,
				"errors": []string{validationErr},
			},
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"valid":  true,
			"errors": []string{},
		},
	})
}

// GetEnvironmentPresets returns available environment presets
// GET /api/v1/environment/presets
func (h *EnvironmentHandler) GetEnvironmentPresets(c *gin.Context) {
	presets := []EnvironmentPreset{
		{
			ID:          "nextjs",
			Name:        "Next.js App",
			Description: "Full-stack React framework with TypeScript",
			Language:    "node",
			Version:     "20",
			Packages: []PackageDependency{
				{Name: "next", Version: "^14.0.0"},
				{Name: "react", Version: "^18.0.0"},
				{Name: "react-dom", Version: "^18.0.0"},
			},
			DevPackages: []PackageDependency{
				{Name: "typescript", Version: "^5.0.0"},
				{Name: "@types/react", Version: "^18.0.0"},
				{Name: "@types/node", Version: "^20.0.0"},
				{Name: "eslint", Version: "^8.0.0"},
			},
			System: []string{"git"},
		},
		{
			ID:          "fastapi",
			Name:        "FastAPI Backend",
			Description: "Modern Python API framework",
			Language:    "python",
			Version:     "3.11",
			Packages: []PackageDependency{
				{Name: "fastapi", Version: ">=0.100.0"},
				{Name: "uvicorn", Version: ">=0.23.0"},
				{Name: "pydantic", Version: ">=2.0.0"},
			},
			DevPackages: []PackageDependency{
				{Name: "pytest", Version: ">=7.0.0"},
				{Name: "black", Version: ">=23.0.0"},
				{Name: "ruff", Version: ">=0.1.0"},
			},
			System: []string{"git", "curl"},
		},
		{
			ID:          "gin-api",
			Name:        "Gin REST API",
			Description: "High-performance Go web framework",
			Language:    "go",
			Version:     "1.21",
			Packages: []PackageDependency{
				{Name: "github.com/gin-gonic/gin", Version: "v1.9.1"},
				{Name: "gorm.io/gorm", Version: "v1.25.0"},
				{Name: "gorm.io/driver/postgres", Version: "v1.5.0"},
			},
			DevPackages: []PackageDependency{
				{Name: "github.com/stretchr/testify", Version: "v1.8.0"},
			},
			System: []string{"git", "curl"},
		},
		{
			ID:          "react-vite",
			Name:        "React + Vite",
			Description: "Fast React development with Vite",
			Language:    "node",
			Version:     "20",
			Packages: []PackageDependency{
				{Name: "react", Version: "^18.0.0"},
				{Name: "react-dom", Version: "^18.0.0"},
				{Name: "react-router-dom", Version: "^6.0.0"},
			},
			DevPackages: []PackageDependency{
				{Name: "vite", Version: "^5.0.0"},
				{Name: "@vitejs/plugin-react", Version: "^4.0.0"},
				{Name: "typescript", Version: "^5.0.0"},
			},
			System: []string{"git"},
		},
		{
			ID:          "django",
			Name:        "Django Web App",
			Description: "Full-featured Python web framework",
			Language:    "python",
			Version:     "3.11",
			Packages: []PackageDependency{
				{Name: "django", Version: ">=4.2.0"},
				{Name: "django-cors-headers", Version: ">=4.0.0"},
				{Name: "psycopg2-binary", Version: ">=2.9.0"},
			},
			DevPackages: []PackageDependency{
				{Name: "pytest-django", Version: ">=4.5.0"},
				{Name: "black", Version: ">=23.0.0"},
			},
			System: []string{"git", "curl"},
		},
		{
			ID:          "express",
			Name:        "Express.js API",
			Description: "Minimal Node.js web framework",
			Language:    "node",
			Version:     "20",
			Packages: []PackageDependency{
				{Name: "express", Version: "^4.18.0"},
				{Name: "cors", Version: "^2.8.0"},
				{Name: "helmet", Version: "^7.0.0"},
				{Name: "dotenv", Version: "^16.0.0"},
			},
			DevPackages: []PackageDependency{
				{Name: "typescript", Version: "^5.0.0"},
				{Name: "ts-node", Version: "^10.0.0"},
				{Name: "nodemon", Version: "^3.0.0"},
				{Name: "@types/express", Version: "^4.17.0"},
			},
			System: []string{"git"},
		},
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"presets": presets,
		},
	})
}

// EnvironmentPreset represents a pre-configured environment template
type EnvironmentPreset struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Language    string              `json:"language"`
	Version     string              `json:"version"`
	Packages    []PackageDependency `json:"packages"`
	DevPackages []PackageDependency `json:"dev_packages"`
	System      []string            `json:"system"`
}

// ApplyEnvironmentPreset applies a preset to a project
// POST /api/v1/environment/project/:projectId/preset/:presetId
func (h *EnvironmentHandler) ApplyEnvironmentPreset(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectIDStr := c.Param("projectId")
	presetID := c.Param("presetId")

	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	// Verify user owns the project
	var project models.Project
	if err := h.DB.Where("id = ? AND owner_id = ?", uint(projectID), userID).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Project not found or access denied",
			Code:    "PROJECT_NOT_FOUND",
		})
		return
	}

	// Get preset
	preset := getPresetByID(presetID)
	if preset == nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Preset not found",
			Code:    "PRESET_NOT_FOUND",
		})
		return
	}

	// Create environment config from preset
	envConfig := EnvironmentConfig{
		Language:    preset.Language,
		Version:     preset.Version,
		Packages:    preset.Packages,
		DevPackages: preset.DevPackages,
		System:      preset.System,
		EnvVars:     make(map[string]string),
	}

	// Serialize and save
	envConfigJSON, err := json.Marshal(envConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to serialize environment configuration",
			Code:    "SERIALIZATION_ERROR",
		})
		return
	}

	updates := map[string]interface{}{
		"environment_config": string(envConfigJSON),
		"language":           preset.Language,
	}

	if err := h.DB.Model(&project).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to apply preset",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Environment preset applied successfully",
		Data: map[string]interface{}{
			"project_id":  projectID,
			"preset":      preset.Name,
			"environment": envConfig,
		},
	})
}

// Helper functions

func parseEnvironmentConfig(project *models.Project) *EnvironmentConfig {
	// Check if environment_config exists in project
	if project.Environment == nil {
		// Return default based on language
		return getDefaultEnvironmentConfig(project.Language)
	}

	// Try to parse from environment_config field
	if configStr, ok := project.Environment["config"].(string); ok {
		var config EnvironmentConfig
		if err := json.Unmarshal([]byte(configStr), &config); err == nil {
			return &config
		}
	}

	// Return default
	return getDefaultEnvironmentConfig(project.Language)
}

func getDefaultEnvironmentConfig(language string) *EnvironmentConfig {
	defaults := map[string]*EnvironmentConfig{
		"javascript": {
			Language: "node",
			Version:  "20",
			Packages: []PackageDependency{},
			System:   []string{"git"},
		},
		"typescript": {
			Language: "node",
			Version:  "20",
			Packages: []PackageDependency{},
			DevPackages: []PackageDependency{
				{Name: "typescript", Version: "^5.0.0"},
			},
			System: []string{"git"},
		},
		"python": {
			Language: "python",
			Version:  "3.11",
			Packages: []PackageDependency{},
			System:   []string{"git"},
		},
		"go": {
			Language: "go",
			Version:  "1.21",
			Packages: []PackageDependency{},
			System:   []string{"git"},
		},
		"rust": {
			Language: "rust",
			Version:  "1.75",
			Packages: []PackageDependency{},
			System:   []string{"git"},
		},
	}

	if config, ok := defaults[strings.ToLower(language)]; ok {
		return config
	}

	// Default fallback
	return &EnvironmentConfig{
		Language: "node",
		Version:  "20",
		Packages: []PackageDependency{},
		System:   []string{"git"},
	}
}

func validateEnvironmentConfig(config *EnvironmentConfig) string {
	validLanguages := []string{"node", "python", "go", "rust", "java", "ruby", "php", "deno", "bun"}

	// Validate language
	validLang := false
	for _, lang := range validLanguages {
		if config.Language == lang {
			validLang = true
			break
		}
	}
	if !validLang {
		return "Invalid language. Must be one of: " + strings.Join(validLanguages, ", ")
	}

	// Validate version is not empty
	if config.Version == "" {
		return "Version is required"
	}

	// Validate packages have names
	for _, pkg := range config.Packages {
		if pkg.Name == "" {
			return "Package name cannot be empty"
		}
	}

	for _, pkg := range config.DevPackages {
		if pkg.Name == "" {
			return "Dev package name cannot be empty"
		}
	}

	return ""
}

func detectEnvironmentFromFiles(files []models.File, project *models.Project) *EnvironmentConfig {
	config := &EnvironmentConfig{
		Packages:    []PackageDependency{},
		DevPackages: []PackageDependency{},
		System:      []string{"git"},
		EnvVars:     make(map[string]string),
	}

	hasPackageJSON := false
	hasRequirementsTxt := false
	hasGoMod := false
	hasCargoToml := false
	hasPomXML := false
	hasGemfile := false
	hasComposerJSON := false

	for _, file := range files {
		switch file.Name {
		case "package.json":
			hasPackageJSON = true
			// Try to parse packages from content
			parsePackageJSON(file.Content, config)
		case "requirements.txt":
			hasRequirementsTxt = true
			parseRequirementsTxt(file.Content, config)
		case "go.mod":
			hasGoMod = true
			config.Language = "go"
			config.Version = "1.21"
		case "Cargo.toml":
			hasCargoToml = true
			config.Language = "rust"
			config.Version = "1.75"
		case "pom.xml":
			hasPomXML = true
			config.Language = "java"
			config.Version = "17"
		case "Gemfile":
			hasGemfile = true
			config.Language = "ruby"
			config.Version = "3.2"
		case "composer.json":
			hasComposerJSON = true
			config.Language = "php"
			config.Version = "8.2"
		}
	}

	// Set language based on detected files
	if hasPackageJSON {
		config.Language = "node"
		config.Version = "20"
	} else if hasRequirementsTxt {
		config.Language = "python"
		config.Version = "3.11"
	} else if hasGoMod {
		// Already set above
	} else if hasCargoToml {
		// Already set above
	} else if hasPomXML {
		// Already set above
	} else if hasGemfile {
		// Already set above
	} else if hasComposerJSON {
		// Already set above
	} else {
		// Fallback to project language
		config.Language = strings.ToLower(project.Language)
		config.Version = getDefaultVersionForLanguage(config.Language)
	}

	return config
}

func parsePackageJSON(content string, config *EnvironmentConfig) {
	var pkgJSON map[string]interface{}
	if err := json.Unmarshal([]byte(content), &pkgJSON); err != nil {
		return
	}

	// Parse dependencies
	if deps, ok := pkgJSON["dependencies"].(map[string]interface{}); ok {
		for name, version := range deps {
			if vStr, ok := version.(string); ok {
				config.Packages = append(config.Packages, PackageDependency{
					Name:    name,
					Version: vStr,
					Source:  "npm",
				})
			}
		}
	}

	// Parse devDependencies
	if devDeps, ok := pkgJSON["devDependencies"].(map[string]interface{}); ok {
		for name, version := range devDeps {
			if vStr, ok := version.(string); ok {
				config.DevPackages = append(config.DevPackages, PackageDependency{
					Name:    name,
					Version: vStr,
					Source:  "npm",
				})
			}
		}
	}
}

func parseRequirementsTxt(content string, config *EnvironmentConfig) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse package==version or package>=version format
		var name, version string
		if strings.Contains(line, "==") {
			parts := strings.SplitN(line, "==", 2)
			name = parts[0]
			if len(parts) > 1 {
				version = parts[1]
			}
		} else if strings.Contains(line, ">=") {
			parts := strings.SplitN(line, ">=", 2)
			name = parts[0]
			if len(parts) > 1 {
				version = ">=" + parts[1]
			}
		} else {
			name = line
		}

		config.Packages = append(config.Packages, PackageDependency{
			Name:    name,
			Version: version,
			Source:  "pypi",
		})
	}
}

func getDefaultVersionForLanguage(language string) string {
	defaults := map[string]string{
		"node":       "20",
		"javascript": "20",
		"typescript": "20",
		"python":     "3.11",
		"go":         "1.21",
		"rust":       "1.75",
		"java":       "17",
		"ruby":       "3.2",
		"php":        "8.2",
	}

	if version, ok := defaults[language]; ok {
		return version
	}
	return "latest"
}

func calculateDetectionConfidence(files []models.File, config *EnvironmentConfig) float64 {
	// Base confidence
	confidence := 0.5

	// Increase confidence based on detected files
	for _, file := range files {
		switch file.Name {
		case "package.json", "requirements.txt", "go.mod", "Cargo.toml":
			confidence += 0.3
		case "tsconfig.json", "pyproject.toml", "Makefile":
			confidence += 0.1
		}
	}

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

func generateEnvironmentSuggestions(config *EnvironmentConfig) []string {
	suggestions := []string{}

	switch config.Language {
	case "node":
		if !hasPackage(config.Packages, "typescript") && !hasPackage(config.DevPackages, "typescript") {
			suggestions = append(suggestions, "Consider adding TypeScript for better type safety")
		}
		if !hasPackage(config.DevPackages, "eslint") {
			suggestions = append(suggestions, "Add ESLint for code linting")
		}
	case "python":
		if !hasPackage(config.DevPackages, "pytest") {
			suggestions = append(suggestions, "Consider adding pytest for testing")
		}
		if !hasPackage(config.DevPackages, "black") && !hasPackage(config.DevPackages, "ruff") {
			suggestions = append(suggestions, "Add black or ruff for code formatting")
		}
	case "go":
		suggestions = append(suggestions, "Consider adding golangci-lint for linting")
	}

	return suggestions
}

func hasPackage(packages []PackageDependency, name string) bool {
	for _, pkg := range packages {
		if pkg.Name == name {
			return true
		}
	}
	return false
}

func getCommonPackagesForRuntime(runtime string) []PackageInfo {
	packages := map[string][]PackageInfo{
		"node": {
			{Name: "typescript", Description: "TypeScript language support", Category: "language"},
			{Name: "express", Description: "Web framework", Category: "web"},
			{Name: "react", Description: "UI library", Category: "ui"},
			{Name: "next", Description: "React framework", Category: "framework"},
			{Name: "axios", Description: "HTTP client", Category: "http"},
			{Name: "lodash", Description: "Utility library", Category: "utility"},
			{Name: "zod", Description: "Schema validation", Category: "validation"},
			{Name: "prisma", Description: "Database ORM", Category: "database"},
			{Name: "eslint", Description: "Code linter", Category: "dev"},
			{Name: "prettier", Description: "Code formatter", Category: "dev"},
			{Name: "jest", Description: "Testing framework", Category: "testing"},
			{Name: "vitest", Description: "Fast testing framework", Category: "testing"},
		},
		"python": {
			{Name: "fastapi", Description: "Modern web framework", Category: "web"},
			{Name: "django", Description: "Full-stack framework", Category: "framework"},
			{Name: "flask", Description: "Micro web framework", Category: "web"},
			{Name: "requests", Description: "HTTP library", Category: "http"},
			{Name: "pydantic", Description: "Data validation", Category: "validation"},
			{Name: "sqlalchemy", Description: "Database ORM", Category: "database"},
			{Name: "pytest", Description: "Testing framework", Category: "testing"},
			{Name: "black", Description: "Code formatter", Category: "dev"},
			{Name: "ruff", Description: "Fast linter", Category: "dev"},
			{Name: "numpy", Description: "Numerical computing", Category: "data"},
			{Name: "pandas", Description: "Data analysis", Category: "data"},
		},
		"go": {
			{Name: "github.com/gin-gonic/gin", Description: "Web framework", Category: "web"},
			{Name: "gorm.io/gorm", Description: "ORM library", Category: "database"},
			{Name: "github.com/spf13/viper", Description: "Configuration", Category: "config"},
			{Name: "go.uber.org/zap", Description: "Logging", Category: "logging"},
			{Name: "github.com/stretchr/testify", Description: "Testing toolkit", Category: "testing"},
			{Name: "github.com/gorilla/mux", Description: "HTTP router", Category: "web"},
			{Name: "github.com/go-redis/redis", Description: "Redis client", Category: "database"},
		},
		"rust": {
			{Name: "tokio", Description: "Async runtime", Category: "async"},
			{Name: "serde", Description: "Serialization", Category: "utility"},
			{Name: "actix-web", Description: "Web framework", Category: "web"},
			{Name: "reqwest", Description: "HTTP client", Category: "http"},
			{Name: "sqlx", Description: "SQL toolkit", Category: "database"},
			{Name: "tracing", Description: "Logging", Category: "logging"},
		},
	}

	if pkgs, ok := packages[runtime]; ok {
		return pkgs
	}
	return []PackageInfo{}
}

// PackageInfo represents package metadata
type PackageInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

func getPresetByID(presetID string) *EnvironmentPreset {
	presets := map[string]*EnvironmentPreset{
		"nextjs": {
			ID:          "nextjs",
			Name:        "Next.js App",
			Description: "Full-stack React framework with TypeScript",
			Language:    "node",
			Version:     "20",
			Packages: []PackageDependency{
				{Name: "next", Version: "^14.0.0"},
				{Name: "react", Version: "^18.0.0"},
				{Name: "react-dom", Version: "^18.0.0"},
			},
			DevPackages: []PackageDependency{
				{Name: "typescript", Version: "^5.0.0"},
				{Name: "@types/react", Version: "^18.0.0"},
				{Name: "@types/node", Version: "^20.0.0"},
			},
			System: []string{"git"},
		},
		"fastapi": {
			ID:          "fastapi",
			Name:        "FastAPI Backend",
			Language:    "python",
			Version:     "3.11",
			Packages: []PackageDependency{
				{Name: "fastapi", Version: ">=0.100.0"},
				{Name: "uvicorn", Version: ">=0.23.0"},
				{Name: "pydantic", Version: ">=2.0.0"},
			},
			DevPackages: []PackageDependency{
				{Name: "pytest", Version: ">=7.0.0"},
				{Name: "black", Version: ">=23.0.0"},
			},
			System: []string{"git", "curl"},
		},
		"gin-api": {
			ID:          "gin-api",
			Name:        "Gin REST API",
			Language:    "go",
			Version:     "1.21",
			Packages: []PackageDependency{
				{Name: "github.com/gin-gonic/gin", Version: "v1.9.1"},
				{Name: "gorm.io/gorm", Version: "v1.25.0"},
			},
			DevPackages: []PackageDependency{
				{Name: "github.com/stretchr/testify", Version: "v1.8.0"},
			},
			System: []string{"git"},
		},
		"react-vite": {
			ID:          "react-vite",
			Name:        "React + Vite",
			Language:    "node",
			Version:     "20",
			Packages: []PackageDependency{
				{Name: "react", Version: "^18.0.0"},
				{Name: "react-dom", Version: "^18.0.0"},
			},
			DevPackages: []PackageDependency{
				{Name: "vite", Version: "^5.0.0"},
				{Name: "@vitejs/plugin-react", Version: "^4.0.0"},
				{Name: "typescript", Version: "^5.0.0"},
			},
			System: []string{"git"},
		},
		"django": {
			ID:          "django",
			Name:        "Django Web App",
			Language:    "python",
			Version:     "3.11",
			Packages: []PackageDependency{
				{Name: "django", Version: ">=4.2.0"},
				{Name: "django-cors-headers", Version: ">=4.0.0"},
			},
			DevPackages: []PackageDependency{
				{Name: "pytest-django", Version: ">=4.5.0"},
				{Name: "black", Version: ">=23.0.0"},
			},
			System: []string{"git", "curl"},
		},
		"express": {
			ID:          "express",
			Name:        "Express.js API",
			Language:    "node",
			Version:     "20",
			Packages: []PackageDependency{
				{Name: "express", Version: "^4.18.0"},
				{Name: "cors", Version: "^2.8.0"},
				{Name: "helmet", Version: "^7.0.0"},
			},
			DevPackages: []PackageDependency{
				{Name: "typescript", Version: "^5.0.0"},
				{Name: "nodemon", Version: "^3.0.0"},
			},
			System: []string{"git"},
		},
	}

	return presets[presetID]
}
