// Package handlers - Git Integration HTTP Handlers for APEX.BUILD
package handlers

import (
	"net/http"
	"strconv"

	"apex-build/internal/git"
	"apex-build/internal/secrets"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GitHandler handles git-related endpoints
type GitHandler struct {
	db             *gorm.DB
	gitService     *git.GitService
	secretsManager *secrets.SecretsManager
}

// NewGitHandler creates a new git handler
func NewGitHandler(db *gorm.DB, gitService *git.GitService, secretsManager *secrets.SecretsManager) *GitHandler {
	return &GitHandler{
		db:             db,
		gitService:     gitService,
		secretsManager: secretsManager,
	}
}

// ConnectRepository connects a project to a remote repository
// POST /api/v1/git/connect
func (h *GitHandler) ConnectRepository(c *gin.Context) {
	userID := c.GetUint("userID")

	var req struct {
		ProjectID uint   `json:"project_id" binding:"required"`
		RemoteURL string `json:"remote_url" binding:"required"`
		Token     string `json:"token"` // GitHub/GitLab personal access token
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify project ownership
	var project models.Project
	if err := h.db.First(&project, req.ProjectID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// If token provided, store it securely as an encrypted secret in the database
	token := req.Token
	if token != "" && h.secretsManager != nil {
		// Encrypt the token
		encryptedValue, salt, _, err := h.secretsManager.Encrypt(userID, token)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt token"})
			return
		}

		// Store as a secret in the database
		secret := &secrets.Secret{
			UserID:         userID,
			ProjectID:      &req.ProjectID,
			Name:           "GITHUB_TOKEN",
			Type:           secrets.SecretTypeOAuth,
			EncryptedValue: encryptedValue,
			Salt:           salt,
			KeyFingerprint: "", // Derived from encryption
			Description:    "GitHub Personal Access Token for " + req.RemoteURL,
		}
		if err := h.db.Create(secret).Error; err != nil {
			// Update if exists
			h.db.Model(&secrets.Secret{}).
				Where("user_id = ? AND project_id = ? AND name = ?", userID, req.ProjectID, "GITHUB_TOKEN").
				Updates(map[string]interface{}{
					"encrypted_value": encryptedValue,
					"salt":            salt,
				})
		}
	}

	repo, err := h.gitService.ConnectRepository(c.Request.Context(), req.ProjectID, req.RemoteURL, token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"repository": repo,
		"message":    "Repository connected successfully",
	})
}

// GetRepository returns the repository configuration for a project
// GET /api/v1/git/repo/:projectId
func (h *GitHandler) GetRepository(c *gin.Context) {
	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	repo, err := h.gitService.GetRepository(c.Request.Context(), uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Repository not connected"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"repository": repo,
	})
}

// GetBranches returns all branches for a repository
// GET /api/v1/git/branches/:projectId
func (h *GitHandler) GetBranches(c *gin.Context) {
	userID := c.GetUint("userID")
	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	token := h.getGitToken(userID, uint(projectID))

	branches, err := h.gitService.GetBranches(c.Request.Context(), uint(projectID), token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"branches": branches,
		"count":    len(branches),
	})
}

// GetCommits returns commit history
// GET /api/v1/git/commits/:projectId
func (h *GitHandler) GetCommits(c *gin.Context) {
	userID := c.GetUint("userID")
	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	branch := c.DefaultQuery("branch", "")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	token := h.getGitToken(userID, uint(projectID))

	commits, err := h.gitService.GetCommits(c.Request.Context(), uint(projectID), branch, limit, token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"commits": commits,
		"count":   len(commits),
	})
}

// GetStatus returns the working tree status
// GET /api/v1/git/status/:projectId
func (h *GitHandler) GetStatus(c *gin.Context) {
	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	changes, err := h.gitService.GetWorkingTreeStatus(c.Request.Context(), uint(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Separate staged and unstaged
	var staged, unstaged []*git.FileChange
	for _, change := range changes {
		if change.Staged {
			staged = append(staged, change)
		} else {
			unstaged = append(unstaged, change)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"staged":   staged,
		"unstaged": unstaged,
		"total":    len(changes),
	})
}

// Commit creates a new commit
// POST /api/v1/git/commit
func (h *GitHandler) Commit(c *gin.Context) {
	userID := c.GetUint("userID")

	var req struct {
		ProjectID uint     `json:"project_id" binding:"required"`
		Message   string   `json:"message" binding:"required"`
		Files     []string `json:"files" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify project ownership
	var project models.Project
	if err := h.db.First(&project, req.ProjectID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	token := h.getGitToken(userID, req.ProjectID)

	commit, err := h.gitService.CreateCommit(c.Request.Context(), req.ProjectID, req.Message, req.Files, token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"commit":  commit,
		"message": "Commit created and pushed successfully",
	})
}

// Push pushes commits to remote
// POST /api/v1/git/push
func (h *GitHandler) Push(c *gin.Context) {
	userID := c.GetUint("userID")

	var req struct {
		ProjectID uint `json:"project_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token := h.getGitToken(userID, req.ProjectID)

	if err := h.gitService.Push(c.Request.Context(), req.ProjectID, token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Pushed successfully",
	})
}

// Pull pulls changes from remote
// POST /api/v1/git/pull
func (h *GitHandler) Pull(c *gin.Context) {
	userID := c.GetUint("userID")

	var req struct {
		ProjectID uint `json:"project_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify project ownership
	var project models.Project
	if err := h.db.First(&project, req.ProjectID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	token := h.getGitToken(userID, req.ProjectID)

	if err := h.gitService.Pull(c.Request.Context(), req.ProjectID, token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Pulled successfully",
	})
}

// CreateBranch creates a new branch
// POST /api/v1/git/branch
func (h *GitHandler) CreateBranch(c *gin.Context) {
	userID := c.GetUint("userID")

	var req struct {
		ProjectID  uint   `json:"project_id" binding:"required"`
		BranchName string `json:"branch_name" binding:"required"`
		BaseBranch string `json:"base_branch"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.BaseBranch == "" {
		req.BaseBranch = "main"
	}

	token := h.getGitToken(userID, req.ProjectID)

	branch, err := h.gitService.CreateBranch(c.Request.Context(), req.ProjectID, req.BranchName, req.BaseBranch, token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"branch":  branch,
		"message": "Branch created successfully",
	})
}

// SwitchBranch switches to a different branch
// POST /api/v1/git/checkout
func (h *GitHandler) SwitchBranch(c *gin.Context) {
	userID := c.GetUint("userID")

	var req struct {
		ProjectID  uint   `json:"project_id" binding:"required"`
		BranchName string `json:"branch_name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token := h.getGitToken(userID, req.ProjectID)

	if err := h.gitService.SwitchBranch(c.Request.Context(), req.ProjectID, req.BranchName, token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"branch":  req.BranchName,
		"message": "Switched to branch " + req.BranchName,
	})
}

// GetPullRequests lists pull requests
// GET /api/v1/git/pulls/:projectId
func (h *GitHandler) GetPullRequests(c *gin.Context) {
	userID := c.GetUint("userID")
	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	state := c.DefaultQuery("state", "open")
	token := h.getGitToken(userID, uint(projectID))

	prs, err := h.gitService.GetPullRequests(c.Request.Context(), uint(projectID), state, token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"pull_requests": prs,
		"count":         len(prs),
	})
}

// CreatePullRequest creates a new pull request
// POST /api/v1/git/pulls
func (h *GitHandler) CreatePullRequest(c *gin.Context) {
	userID := c.GetUint("userID")

	var req struct {
		ProjectID uint   `json:"project_id" binding:"required"`
		Title     string `json:"title" binding:"required"`
		Body      string `json:"body"`
		Head      string `json:"head" binding:"required"` // Source branch
		Base      string `json:"base"`                    // Target branch
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Base == "" {
		req.Base = "main"
	}

	token := h.getGitToken(userID, req.ProjectID)

	pr, err := h.gitService.CreatePullRequest(c.Request.Context(), req.ProjectID, req.Title, req.Body, req.Head, req.Base, token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"pull_request": pr,
		"message":      "Pull request created successfully",
	})
}

// DisconnectRepository disconnects a repository from a project
// DELETE /api/v1/git/repo/:projectId
func (h *GitHandler) DisconnectRepository(c *gin.Context) {
	userID := c.GetUint("userID")
	projectIDStr := c.Param("projectId")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	// Verify project ownership
	var project models.Project
	if err := h.db.First(&project, uint(projectID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Delete repository record
	if err := h.db.Where("project_id = ?", projectID).Delete(&git.Repository{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disconnect repository"})
		return
	}

	// Delete stored token
	h.db.Where("user_id = ? AND project_id = ? AND name = ?", userID, projectID, "GITHUB_TOKEN").
		Delete(&secrets.Secret{})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Repository disconnected",
	})
}

// Helper to get stored git token
func (h *GitHandler) getGitToken(userID, projectID uint) string {
	if h.secretsManager == nil {
		return ""
	}

	var secret secrets.Secret
	if err := h.db.Where("user_id = ? AND project_id = ? AND name = ?", userID, projectID, "GITHUB_TOKEN").
		First(&secret).Error; err != nil {
		return ""
	}

	// Decrypt the token
	token, err := h.secretsManager.Decrypt(userID, secret.EncryptedValue, secret.Salt)
	if err != nil {
		return ""
	}
	return token
}
