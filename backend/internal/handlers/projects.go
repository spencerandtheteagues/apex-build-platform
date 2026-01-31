// APEX.BUILD Project Handlers
// Project and file management endpoints

package handlers

import (
	"net/http"
	"strconv"

	"apex-build/internal/database"
	"apex-build/internal/middleware"
	"apex-build/internal/secrets"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Global references for auto-provisioning (set during app initialization)
var (
	globalDBManager      *database.DatabaseManager
	globalSecretsManager *secrets.SecretsManager
)

// InitAutoProvisioningDeps initializes the global dependencies for auto-provisioning
// This should be called during application startup after DatabaseManager and SecretsManager are created
func InitAutoProvisioningDeps(dbManager *database.DatabaseManager, secretsMgr *secrets.SecretsManager) {
	globalDBManager = dbManager
	globalSecretsManager = secretsMgr
}

// GetProjects returns all projects for the authenticated user
func (h *Handler) GetProjects(c *gin.Context) {
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

	var projects []models.Project
	var total int64

	// Get total count
	h.DB.Model(&models.Project{}).Where("owner_id = ?", userID).Count(&total)

	// Get paginated projects (without files - use GetProject for full details)
	result := h.DB.Where("owner_id = ?", userID).
		Order("updated_at DESC").
		Scopes(paginate(page, limit)).
		Find(&projects)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to fetch projects",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Return projects directly in the format frontend expects
	c.JSON(http.StatusOK, gin.H{
		"projects":   projects,
		"pagination": getPaginationInfo(page, limit, total),
	})
}

// CreateProject creates a new project with auto-provisioned PostgreSQL database
func (h *Handler) CreateProject(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	var req struct {
		Name        string                 `json:"name" binding:"required,min=1,max=100"`
		Description string                 `json:"description" binding:"max=500"`
		Language    string                 `json:"language" binding:"required"`
		Framework   string                 `json:"framework"`
		IsPublic    *bool                  `json:"is_public"`
		Environment map[string]interface{} `json:"environment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Validate language
	validLanguages := []string{"javascript", "typescript", "python", "go", "rust", "java", "cpp", "html", "css"}
	languageValid := false
	for _, lang := range validLanguages {
		if req.Language == lang {
			languageValid = true
			break
		}
	}

	if !languageValid {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid language. Supported languages: " + "javascript, typescript, python, go, rust, java, cpp, html, css",
			Code:    "INVALID_LANGUAGE",
		})
		return
	}

	// Create project
	project := models.Project{
		Name:        req.Name,
		Description: req.Description,
		Language:    req.Language,
		Framework:   req.Framework,
		OwnerID:     userID,
		IsPublic:    req.IsPublic != nil && *req.IsPublic,
		Environment: req.Environment,
		BuildConfig: make(map[string]interface{}),
		Dependencies: make(map[string]interface{}),
	}

	if err := h.DB.Create(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to create project",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Create default files based on language
	h.createDefaultFiles(project.ID, req.Language, req.Framework)

	// Auto-provision PostgreSQL database for the project (Replit parity feature)
	// This runs in background and doesn't block project creation
	go h.autoProvisionProjectDatabase(project.ID, userID, req.Name)

	// Reload project with files
	h.DB.Preload("Files").First(&project, project.ID)

	// Return project directly in format frontend expects
	c.JSON(http.StatusCreated, gin.H{
		"message": "Project created successfully",
		"project": project,
	})
}

// autoProvisionProjectDatabase provisions a PostgreSQL database for a new project
// This is called asynchronously after project creation
func (h *Handler) autoProvisionProjectDatabase(projectID, userID uint, projectName string) {
	// Get database manager from context (injected via dependency)
	// Since we can't inject dependencies easily here, we'll use a package-level variable
	// that gets set during initialization
	if globalDBManager == nil {
		return // Database provisioning not available
	}

	if globalSecretsManager == nil {
		return // Secrets manager not available
	}

	// Provision PostgreSQL database
	managedDB, err := globalDBManager.AutoProvisionPostgreSQLForProject(projectID, userID, projectName)
	if err != nil {
		// Log error but don't fail - project creation should succeed even if DB provisioning fails
		return
	}

	// Encrypt the password before storing
	encryptedPassword, salt, _, err := globalSecretsManager.Encrypt(userID, managedDB.Password)
	if err != nil {
		return
	}

	// Store the raw password temporarily for the connection string before encrypting
	rawPassword := managedDB.Password

	managedDB.Password = encryptedPassword
	managedDB.Salt = salt

	// Save managed database to database
	if err := h.DB.Create(managedDB).Error; err != nil {
		return
	}

	// Update project with provisioned database ID
	h.DB.Model(&models.Project{}).Where("id = ?", projectID).Update("provisioned_database_id", managedDB.ID)

	// Create DATABASE_URL secret for the project
	connectionURL := globalDBManager.GetConnectionString(managedDB, rawPassword)

	// Create the secret using the secrets package
	encryptedURL, urlSalt, keyFingerprint, err := globalSecretsManager.Encrypt(userID, connectionURL)
	if err != nil {
		return
	}

	// Import secrets package types
	dbURLSecret := struct {
		UserID         uint
		ProjectID      *uint
		Name           string
		Description    string
		Type           string
		EncryptedValue string
		Salt           string
		KeyFingerprint string
	}{
		UserID:         userID,
		ProjectID:      &projectID,
		Name:           "DATABASE_URL",
		Description:    "Auto-provisioned PostgreSQL connection string",
		Type:           "database",
		EncryptedValue: encryptedURL,
		Salt:           urlSalt,
		KeyFingerprint: keyFingerprint,
	}

	// Insert into secrets table
	h.DB.Table("secrets").Create(&dbURLSecret)
}

// GetProject returns a specific project
func (h *Handler) GetProject(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectIDStr := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	var project models.Project
	result := h.DB.Where("id = ? AND (owner_id = ? OR is_public = ?)", uint(projectID), userID, true).
		Preload("Files").
		Preload("Owner").
		First(&project)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Project not found",
				Code:    "PROJECT_NOT_FOUND",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to fetch project",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Return project directly in format frontend expects
	c.JSON(http.StatusOK, gin.H{
		"project": project,
	})
}

// UpdateProject updates a project
func (h *Handler) UpdateProject(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectIDStr := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	var req struct {
		Name         *string                `json:"name" binding:"omitempty,min=1,max=100"`
		Description  *string                `json:"description" binding:"omitempty,max=500"`
		Framework    *string                `json:"framework"`
		IsPublic     *bool                  `json:"is_public"`
		IsArchived   *bool                  `json:"is_archived"`
		Environment  map[string]interface{} `json:"environment"`
		BuildConfig  map[string]interface{} `json:"build_config"`
		Dependencies map[string]interface{} `json:"dependencies"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Check if user owns the project
	var project models.Project
	if err := h.DB.Where("id = ? AND owner_id = ?", uint(projectID), userID).First(&project).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Project not found or access denied",
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

	// Prepare updates
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Framework != nil {
		updates["framework"] = *req.Framework
	}
	if req.IsPublic != nil {
		updates["is_public"] = *req.IsPublic
	}
	if req.IsArchived != nil {
		updates["is_archived"] = *req.IsArchived
	}
	if req.Environment != nil {
		updates["environment"] = req.Environment
	}
	if req.BuildConfig != nil {
		updates["build_config"] = req.BuildConfig
	}
	if req.Dependencies != nil {
		updates["dependencies"] = req.Dependencies
	}

	// Update project
	if err := h.DB.Model(&project).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to update project",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Project updated successfully",
	})
}

// DeleteProject deletes a project
func (h *Handler) DeleteProject(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectIDStr := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	// Check if user owns the project
	var project models.Project
	if err := h.DB.Where("id = ? AND owner_id = ?", uint(projectID), userID).First(&project).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Project not found or access denied",
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

	// Delete project (soft delete)
	if err := h.DB.Delete(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to delete project",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Project deleted successfully",
	})
}

// createDefaultFiles creates default files for a new project based on language/framework
func (h *Handler) createDefaultFiles(projectID uint, language, framework string) {
	var defaultFiles []models.File

	switch language {
	case "javascript":
		if framework == "react" {
			defaultFiles = h.createReactFiles(projectID)
		} else if framework == "node" {
			defaultFiles = h.createNodeFiles(projectID)
		} else {
			defaultFiles = h.createVanillaJSFiles(projectID)
		}
	case "typescript":
		if framework == "react" {
			defaultFiles = h.createReactTSFiles(projectID)
		} else {
			defaultFiles = h.createNodeTSFiles(projectID)
		}
	case "python":
		if framework == "flask" {
			defaultFiles = h.createFlaskFiles(projectID)
		} else if framework == "django" {
			defaultFiles = h.createDjangoFiles(projectID)
		} else {
			defaultFiles = h.createPythonFiles(projectID)
		}
	case "go":
		defaultFiles = h.createGoFiles(projectID)
	default:
		defaultFiles = h.createGenericFiles(projectID, language)
	}

	// Create all default files
	for _, file := range defaultFiles {
		h.DB.Create(&file)
	}
}

// Helper functions to create default files for different languages/frameworks
func (h *Handler) createReactFiles(projectID uint) []models.File {
	return []models.File{
		{
			ProjectID: projectID,
			Name:      "package.json",
			Path:      "/package.json",
			Type:      "file",
			MimeType:  "application/json",
			Content: `{
  "name": "react-app",
  "version": "0.1.0",
  "dependencies": {
    "react": "^18.0.0",
    "react-dom": "^18.0.0"
  },
  "scripts": {
    "start": "react-scripts start",
    "build": "react-scripts build",
    "test": "react-scripts test"
  }
}`,
		},
		{
			ProjectID: projectID,
			Name:      "App.js",
			Path:      "/src/App.js",
			Type:      "file",
			MimeType:  "text/javascript",
			Content: `import React from 'react';

function App() {
  return (
    <div className="App">
      <header>
        <h1>Welcome to APEX.BUILD</h1>
        <p>Start building amazing apps!</p>
      </header>
    </div>
  );
}

export default App;`,
		},
		{
			ProjectID: projectID,
			Name:      "index.js",
			Path:      "/src/index.js",
			Type:      "file",
			MimeType:  "text/javascript",
			Content: `import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';

const root = ReactDOM.createRoot(document.getElementById('root'));
root.render(<App />);`,
		},
	}
}

func (h *Handler) createGoFiles(projectID uint) []models.File {
	return []models.File{
		{
			ProjectID: projectID,
			Name:      "main.go",
			Path:      "/main.go",
			Type:      "file",
			MimeType:  "text/x-go",
			Content: `package main

import (
	"fmt"
	"net/http"
	"log"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from APEX.BUILD!")
	})

	fmt.Println("Server starting on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}`,
		},
		{
			ProjectID: projectID,
			Name:      "go.mod",
			Path:      "/go.mod",
			Type:      "file",
			MimeType:  "text/plain",
			Content:   `module main

go 1.21`,
		},
	}
}

func (h *Handler) createPythonFiles(projectID uint) []models.File {
	return []models.File{
		{
			ProjectID: projectID,
			Name:      "main.py",
			Path:      "/main.py",
			Type:      "file",
			MimeType:  "text/x-python",
			Content: `#!/usr/bin/env python3
"""
APEX.BUILD Python Application
"""

def main():
    print("Hello from APEX.BUILD!")
    print("Start building amazing Python applications!")

if __name__ == "__main__":
    main()`,
		},
		{
			ProjectID: projectID,
			Name:      "requirements.txt",
			Path:      "/requirements.txt",
			Type:      "file",
			MimeType:  "text/plain",
			Content:   "# Add your Python dependencies here\n",
		},
	}
}

func (h *Handler) createGenericFiles(projectID uint, language string) []models.File {
	var extension, mimeType, content string

	switch language {
	case "html":
		extension = ".html"
		mimeType = "text/html"
		content = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>APEX.BUILD Project</title>
</head>
<body>
    <h1>Welcome to APEX.BUILD</h1>
    <p>Start building amazing web applications!</p>
</body>
</html>`
	case "css":
		extension = ".css"
		mimeType = "text/css"
		content = `/* APEX.BUILD Styles */
body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    margin: 0;
    padding: 20px;
    background: #1a1a1a;
    color: #ffffff;
}

h1 {
    color: #00f5ff;
    text-align: center;
}`
	default:
		extension = ".txt"
		mimeType = "text/plain"
		content = "# APEX.BUILD Project\n\nStart building amazing applications!\n"
	}

	return []models.File{
		{
			ProjectID: projectID,
			Name:      "index" + extension,
			Path:      "/index" + extension,
			Type:      "file",
			MimeType:  mimeType,
			Content:   content,
		},
	}
}

// Additional helper functions for other frameworks would go here...
func (h *Handler) createReactTSFiles(projectID uint) []models.File  { return []models.File{} }
func (h *Handler) createNodeFiles(projectID uint) []models.File    { return []models.File{} }
func (h *Handler) createNodeTSFiles(projectID uint) []models.File  { return []models.File{} }
func (h *Handler) createVanillaJSFiles(projectID uint) []models.File { return []models.File{} }
func (h *Handler) createFlaskFiles(projectID uint) []models.File    { return []models.File{} }
func (h *Handler) createDjangoFiles(projectID uint) []models.File   { return []models.File{} }