// APEX.BUILD Package Management Handlers
// REST API endpoints for package management

package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"apex-build/internal/middleware"
	"apex-build/internal/packages"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
)

// PackageHandler handles package management endpoints
type PackageHandler struct {
	*Handler
	PackageService *packages.PackageManagerService
}

// NewPackageHandler creates a new package handler
func NewPackageHandler(h *Handler) *PackageHandler {
	return &PackageHandler{
		Handler:        h,
		PackageService: packages.NewPackageManagerService(h.DB),
	}
}

// RegisterPackageRoutes registers all package management routes
func (h *PackageHandler) RegisterPackageRoutes(rg *gin.RouterGroup) {
	pkg := rg.Group("/packages")
	{
		// Search packages across registries
		pkg.GET("/search", h.SearchPackages)

		// Get package details
		pkg.GET("/info/:name", h.GetPackageInfo)

		// Install a package to a project
		pkg.POST("/install", h.InstallPackage)

		// Uninstall a package from a project
		pkg.DELETE("/:projectId/:name", h.UninstallPackage)

		// List installed packages for a project
		pkg.GET("/project/:projectId", h.ListInstalledPackages)

		// Update all packages for a project
		pkg.POST("/project/:projectId/update", h.UpdateAllPackages)

		// Get package suggestions based on project type
		pkg.GET("/suggestions/:projectId", h.GetPackageSuggestions)
	}
}

// SearchPackagesRequest represents a package search request
type SearchPackagesRequest struct {
	Query string `form:"q" binding:"required,min=1"`
	Type  string `form:"type" binding:"required,oneof=npm pip go"`
	Limit int    `form:"limit,default=20"`
}

// SearchPackages searches for packages in the specified registry
// GET /api/v1/packages/search?q=query&type=npm|pip|go&limit=20
func (h *PackageHandler) SearchPackages(c *gin.Context) {
	var req SearchPackagesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request. Required: q (query), type (npm|pip|go)",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Validate limit
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	// Convert type string to PackageType
	pkgType := packages.PackageType(req.Type)

	// Search packages
	results, err := h.PackageService.Search(req.Query, pkgType, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to search packages: " + err.Error(),
			Code:    "SEARCH_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"packages": results,
			"query":    req.Query,
			"type":     req.Type,
			"count":    len(results),
		},
	})
}

// GetPackageInfo retrieves detailed information about a package
// GET /api/v1/packages/info/:name?type=npm|pip|go
func (h *PackageHandler) GetPackageInfo(c *gin.Context) {
	name := c.Param("name")
	pkgTypeStr := c.DefaultQuery("type", "npm")

	if name == "" {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Package name is required",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Handle scoped npm packages (e.g., @types/node)
	// They come in as @types%2Fnode due to URL encoding
	name = strings.ReplaceAll(name, "%2F", "/")

	// Validate type
	if pkgTypeStr != "npm" && pkgTypeStr != "pip" && pkgTypeStr != "go" {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid package type. Must be: npm, pip, or go",
			Code:    "INVALID_PACKAGE_TYPE",
		})
		return
	}

	pkgType := packages.PackageType(pkgTypeStr)

	// Get package info
	info, err := h.PackageService.GetPackageInfo(name, pkgType)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, StandardResponse{
			Success: false,
			Error:   "Failed to get package info: " + err.Error(),
			Code:    "PACKAGE_INFO_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data:    info,
	})
}

// InstallPackageRequest represents a package installation request
type InstallPackageRequest struct {
	ProjectID   uint   `json:"project_id" binding:"required"`
	PackageName string `json:"package_name" binding:"required"`
	Version     string `json:"version"`
	Type        string `json:"type" binding:"required,oneof=npm pip go"`
	IsDev       bool   `json:"is_dev"`
}

// InstallPackage installs a package to a project
// POST /api/v1/packages/install
func (h *PackageHandler) InstallPackage(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	var req InstallPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Verify user owns the project
	var project models.Project
	if err := h.DB.Where("id = ? AND owner_id = ?", req.ProjectID, userID).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Project not found or access denied",
			Code:    "PROJECT_NOT_FOUND",
		})
		return
	}

	pkgType := packages.PackageType(req.Type)

	// Install the package
	if err := h.PackageService.Install(req.ProjectID, req.PackageName, req.Version, pkgType, req.IsDev); err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to install package: " + err.Error(),
			Code:    "INSTALL_FAILED",
		})
		return
	}

	// Get updated package info
	info, _ := h.PackageService.GetPackageInfo(req.PackageName, pkgType)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Package installed successfully",
		Data: map[string]interface{}{
			"package": req.PackageName,
			"version": req.Version,
			"is_dev":  req.IsDev,
			"info":    info,
		},
	})
}

// UninstallPackage removes a package from a project
// DELETE /api/v1/packages/:projectId/:name?type=npm|pip|go
func (h *PackageHandler) UninstallPackage(c *gin.Context) {
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
	packageName := c.Param("name")
	pkgTypeStr := c.DefaultQuery("type", "npm")

	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	// Handle scoped npm packages
	packageName = strings.ReplaceAll(packageName, "%2F", "/")

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

	// Validate type
	if pkgTypeStr != "npm" && pkgTypeStr != "pip" && pkgTypeStr != "go" {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid package type. Must be: npm, pip, or go",
			Code:    "INVALID_PACKAGE_TYPE",
		})
		return
	}

	pkgType := packages.PackageType(pkgTypeStr)

	// Uninstall the package
	if err := h.PackageService.Uninstall(uint(projectID), packageName, pkgType); err != nil {
		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not installed") {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, StandardResponse{
			Success: false,
			Error:   "Failed to uninstall package: " + err.Error(),
			Code:    "UNINSTALL_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Package uninstalled successfully",
		Data: map[string]interface{}{
			"package": packageName,
		},
	})
}

// ListInstalledPackages lists all installed packages for a project
// GET /api/v1/packages/project/:projectId?type=npm|pip|go|all
func (h *PackageHandler) ListInstalledPackages(c *gin.Context) {
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
	pkgTypeStr := c.DefaultQuery("type", "all")

	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	// Verify user has access to the project
	var project models.Project
	if err := h.DB.Where("id = ? AND (owner_id = ? OR is_public = ?)", uint(projectID), userID, true).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Project not found or access denied",
			Code:    "PROJECT_NOT_FOUND",
		})
		return
	}

	var response interface{}

	if pkgTypeStr == "all" {
		// List all installed packages across all package managers
		allPackages, err := h.PackageService.ListAllInstalled(uint(projectID))
		if err != nil {
			c.JSON(http.StatusInternalServerError, StandardResponse{
				Success: false,
				Error:   "Failed to list packages: " + err.Error(),
				Code:    "LIST_FAILED",
			})
			return
		}
		response = allPackages
	} else {
		// Validate type
		if pkgTypeStr != "npm" && pkgTypeStr != "pip" && pkgTypeStr != "go" {
			c.JSON(http.StatusBadRequest, StandardResponse{
				Success: false,
				Error:   "Invalid package type. Must be: npm, pip, go, or all",
				Code:    "INVALID_PACKAGE_TYPE",
			})
			return
		}

		pkgType := packages.PackageType(pkgTypeStr)
		pkgList, err := h.PackageService.ListInstalled(uint(projectID), pkgType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, StandardResponse{
				Success: false,
				Error:   "Failed to list packages: " + err.Error(),
				Code:    "LIST_FAILED",
			})
			return
		}
		response = map[string]interface{}{
			pkgTypeStr: pkgList,
		}
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"project_id": projectID,
			"packages":   response,
		},
	})
}

// UpdateAllPackages updates all packages for a project to their latest versions
// POST /api/v1/packages/project/:projectId/update?type=npm|pip|go|all
func (h *PackageHandler) UpdateAllPackages(c *gin.Context) {
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
	pkgTypeStr := c.DefaultQuery("type", "")

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

	// If no type specified, detect from project language
	if pkgTypeStr == "" || pkgTypeStr == "all" {
		detectedType, err := h.PackageService.DetectPackageType(uint(projectID))
		if err != nil {
			c.JSON(http.StatusBadRequest, StandardResponse{
				Success: false,
				Error:   "Could not detect package type for project",
				Code:    "DETECTION_FAILED",
			})
			return
		}
		pkgTypeStr = string(detectedType)
	}

	// Validate type
	if pkgTypeStr != "npm" && pkgTypeStr != "pip" && pkgTypeStr != "go" {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid package type. Must be: npm, pip, or go",
			Code:    "INVALID_PACKAGE_TYPE",
		})
		return
	}

	pkgType := packages.PackageType(pkgTypeStr)

	// Update all packages
	if err := h.PackageService.UpdateDependencyFile(uint(projectID), pkgType); err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to update packages: " + err.Error(),
			Code:    "UPDATE_FAILED",
		})
		return
	}

	// Get updated package list
	updatedPackages, _ := h.PackageService.ListInstalled(uint(projectID), pkgType)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "All packages updated successfully",
		Data: map[string]interface{}{
			"project_id": projectID,
			"type":       pkgTypeStr,
			"packages":   updatedPackages,
		},
	})
}

// GetPackageSuggestions returns package suggestions based on project type
// GET /api/v1/packages/suggestions/:projectId
func (h *PackageHandler) GetPackageSuggestions(c *gin.Context) {
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
	if err := h.DB.First(&project, uint(projectID)).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Project not found",
			Code:    "PROJECT_NOT_FOUND",
		})
		return
	}

	// Generate suggestions based on language/framework
	suggestions := getPackageSuggestions(project.Language, project.Framework)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"project_id":  projectID,
			"language":    project.Language,
			"framework":   project.Framework,
			"suggestions": suggestions,
		},
	})
}

// PackageSuggestion represents a suggested package
type PackageSuggestion struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	IsDev       bool   `json:"is_dev"`
}

// getPackageSuggestions returns package suggestions based on language/framework
func getPackageSuggestions(language, framework string) []PackageSuggestion {
	suggestions := []PackageSuggestion{}

	switch language {
	case "javascript", "typescript":
		suggestions = append(suggestions, []PackageSuggestion{
			{Name: "axios", Description: "Promise based HTTP client", Category: "http"},
			{Name: "lodash", Description: "Utility library", Category: "utility"},
			{Name: "date-fns", Description: "Date utility library", Category: "utility"},
			{Name: "uuid", Description: "UUID generation", Category: "utility"},
			{Name: "zod", Description: "TypeScript schema validation", Category: "validation"},
			{Name: "eslint", Description: "JavaScript linter", Category: "development", IsDev: true},
			{Name: "prettier", Description: "Code formatter", Category: "development", IsDev: true},
			{Name: "jest", Description: "Testing framework", Category: "testing", IsDev: true},
			{Name: "typescript", Description: "TypeScript compiler", Category: "development", IsDev: true},
		}...)

		switch framework {
		case "react":
			suggestions = append(suggestions, []PackageSuggestion{
				{Name: "react-router-dom", Description: "React routing", Category: "routing"},
				{Name: "react-query", Description: "Data fetching library", Category: "state"},
				{Name: "@tanstack/react-query", Description: "TanStack Query for React", Category: "state"},
				{Name: "zustand", Description: "State management", Category: "state"},
				{Name: "react-hook-form", Description: "Form handling", Category: "forms"},
				{Name: "@testing-library/react", Description: "React testing utilities", Category: "testing", IsDev: true},
			}...)
		case "next":
			suggestions = append(suggestions, []PackageSuggestion{
				{Name: "next-auth", Description: "Authentication for Next.js", Category: "auth"},
				{Name: "@vercel/analytics", Description: "Vercel analytics", Category: "analytics"},
				{Name: "next-seo", Description: "SEO for Next.js", Category: "seo"},
			}...)
		case "node":
			suggestions = append(suggestions, []PackageSuggestion{
				{Name: "express", Description: "Web framework", Category: "web"},
				{Name: "cors", Description: "CORS middleware", Category: "middleware"},
				{Name: "helmet", Description: "Security headers", Category: "security"},
				{Name: "dotenv", Description: "Environment variables", Category: "config"},
				{Name: "morgan", Description: "HTTP logger", Category: "logging"},
				{Name: "bcrypt", Description: "Password hashing", Category: "security"},
				{Name: "jsonwebtoken", Description: "JWT handling", Category: "auth"},
			}...)
		}

	case "python":
		suggestions = append(suggestions, []PackageSuggestion{
			{Name: "requests", Description: "HTTP library", Category: "http"},
			{Name: "python-dotenv", Description: "Environment variables", Category: "config"},
			{Name: "pydantic", Description: "Data validation", Category: "validation"},
			{Name: "pytest", Description: "Testing framework", Category: "testing", IsDev: true},
			{Name: "black", Description: "Code formatter", Category: "development", IsDev: true},
			{Name: "ruff", Description: "Fast Python linter", Category: "development", IsDev: true},
		}...)

		switch framework {
		case "flask":
			suggestions = append(suggestions, []PackageSuggestion{
				{Name: "flask-cors", Description: "CORS for Flask", Category: "middleware"},
				{Name: "flask-sqlalchemy", Description: "SQLAlchemy integration", Category: "database"},
				{Name: "flask-login", Description: "User session management", Category: "auth"},
				{Name: "flask-migrate", Description: "Database migrations", Category: "database"},
			}...)
		case "django":
			suggestions = append(suggestions, []PackageSuggestion{
				{Name: "djangorestframework", Description: "REST API framework", Category: "api"},
				{Name: "django-cors-headers", Description: "CORS for Django", Category: "middleware"},
				{Name: "celery", Description: "Task queue", Category: "async"},
				{Name: "django-filter", Description: "Filtering for DRF", Category: "api"},
			}...)
		case "fastapi":
			suggestions = append(suggestions, []PackageSuggestion{
				{Name: "uvicorn", Description: "ASGI server", Category: "server"},
				{Name: "sqlalchemy", Description: "SQL toolkit", Category: "database"},
				{Name: "alembic", Description: "Database migrations", Category: "database"},
				{Name: "python-jose", Description: "JWT handling", Category: "auth"},
			}...)
		}

	case "go":
		suggestions = append(suggestions, []PackageSuggestion{
			{Name: "github.com/gin-gonic/gin", Description: "Web framework", Category: "web"},
			{Name: "gorm.io/gorm", Description: "ORM library", Category: "database"},
			{Name: "github.com/spf13/viper", Description: "Configuration", Category: "config"},
			{Name: "go.uber.org/zap", Description: "Logging", Category: "logging"},
			{Name: "github.com/stretchr/testify", Description: "Testing toolkit", Category: "testing", IsDev: true},
			{Name: "github.com/golang-jwt/jwt/v5", Description: "JWT handling", Category: "auth"},
			{Name: "github.com/google/uuid", Description: "UUID generation", Category: "utility"},
			{Name: "github.com/go-playground/validator/v10", Description: "Validation", Category: "validation"},
			{Name: "github.com/gorilla/websocket", Description: "WebSocket support", Category: "websocket"},
			{Name: "github.com/redis/go-redis/v9", Description: "Redis client", Category: "database"},
		}...)
	}

	return suggestions
}
