// Package handlers - GitHub Export Handler for APEX.BUILD
// Enables exporting projects to new GitHub repositories
package handlers

import (
	"net/http"
	"strconv"

	"apex-build/internal/git"
	"apex-build/internal/handlers/export_templates"
	"apex-build/internal/secrets"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ExportHandler handles project export endpoints
type ExportHandler struct {
	db             *gorm.DB
	gitService     *git.GitService
	secretsManager *secrets.SecretsManager
}

// NewExportHandler creates a new export handler
func NewExportHandler(db *gorm.DB, gitService *git.GitService, secretsManager *secrets.SecretsManager) *ExportHandler {
	return &ExportHandler{
		db:             db,
		gitService:     gitService,
		secretsManager: secretsManager,
	}
}

// ExportToGitHub exports a project to a new GitHub repository
// POST /api/v1/git/export
func (h *ExportHandler) ExportToGitHub(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		ProjectID        uint   `json:"project_id" binding:"required"`
		RepoName         string `json:"repo_name" binding:"required"`
		Description      string `json:"description"`
		IsPrivate        bool   `json:"is_private"`
		Token            string `json:"token"` // GitHub PAT — required for export
		IncludeGitignore bool   `json:"include_gitignore"`
		IncludeDockerfile bool  `json:"include_dockerfile"`
		IncludeReadme    bool   `json:"include_readme"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Verify project ownership
	var project models.Project
	if err := h.db.First(&project, req.ProjectID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Project not found",
		})
		return
	}

	if project.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Access denied",
		})
		return
	}

	// Resolve GitHub token: use provided token, fall back to stored token
	token := req.Token
	if token == "" {
		token = h.getGitToken(userID, req.ProjectID)
	}
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "GitHub token required — provide a personal access token with 'repo' scope",
		})
		return
	}

	// Build description
	description := req.Description
	if description == "" {
		description = "Exported from APEX.BUILD"
		if project.Description != "" {
			description = project.Description
		}
	}

	// Determine the tech stack from the project's language/framework
	stack := project.Language
	if project.Framework != "" {
		stack = project.Framework
	}

	// Collect supplementary export files
	type ExportFile struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	var extraFiles []ExportFile

	if req.IncludeGitignore {
		extraFiles = append(extraFiles, ExportFile{
			Path:    ".gitignore",
			Content: export_templates.GitignoreForStack(stack),
		})
	}
	if req.IncludeDockerfile {
		extraFiles = append(extraFiles, ExportFile{
			Path:    "Dockerfile",
			Content: export_templates.DockerfileForStack(stack),
		})
	}
	if req.IncludeReadme {
		extraFiles = append(extraFiles, ExportFile{
			Path:    "README.md",
			Content: export_templates.ReadmeForProject(project.Name, description, stack),
		})
	}

	// Export the project
	result, err := h.gitService.ExportToGitHub(
		c.Request.Context(),
		req.ProjectID,
		req.RepoName,
		description,
		token,
		req.IsPrivate,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Export failed: " + err.Error(),
		})
		return
	}

	// Track how many extra files were included
	extraFileCount := len(extraFiles)
	_ = extraFileCount // Will be used when git service supports additional files

	// Optionally store the token for future use
	if req.Token != "" && h.secretsManager != nil {
		encryptedValue, salt, _, err := h.secretsManager.Encrypt(userID, req.Token)
		if err == nil {
			secret := &secrets.Secret{
				UserID:         userID,
				ProjectID:      &req.ProjectID,
				Name:           "GITHUB_TOKEN",
				Type:           secrets.SecretTypeOAuth,
				EncryptedValue: encryptedValue,
				Salt:           salt,
				Description:    "GitHub Token for " + result.RepoURL,
			}
			if createErr := h.db.Create(secret).Error; createErr != nil {
				// Update existing
				h.db.Model(&secrets.Secret{}).
					Where("user_id = ? AND project_id = ? AND name = ?", userID, req.ProjectID, "GITHUB_TOKEN").
					Updates(map[string]interface{}{
						"encrypted_value": encryptedValue,
						"salt":            salt,
					})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
		"message": "Project exported to GitHub successfully",
	})
}

// GetExportStatus checks if a project is already connected to a GitHub repo
// GET /api/v1/git/export/status/:projectId
func (h *ExportHandler) GetExportStatus(c *gin.Context) {
	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid project ID",
		})
		return
	}

	repo, err := h.gitService.GetRepository(c.Request.Context(), uint(projectID))
	if err != nil {
		// No repo connected — that's fine
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"exported":   false,
			"repository": nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"exported":   true,
		"repository": repo,
	})
}

// Helper to get stored git token
func (h *ExportHandler) getGitToken(userID, projectID uint) string {
	if h.secretsManager == nil {
		return ""
	}

	var secret secrets.Secret
	if err := h.db.Where("user_id = ? AND project_id = ? AND name = ?", userID, projectID, "GITHUB_TOKEN").
		First(&secret).Error; err != nil {
		return ""
	}

	token, err := h.secretsManager.Decrypt(userID, secret.EncryptedValue, secret.Salt)
	if err != nil {
		return ""
	}
	return token
}
