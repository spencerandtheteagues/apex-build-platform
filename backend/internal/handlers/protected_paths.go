package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ProtectedPathsHandler serves protected path configuration endpoints.
type ProtectedPathsHandler struct {
	db *gorm.DB
}

// NewProtectedPathsHandler creates a new handler.
func NewProtectedPathsHandler(db *gorm.DB) *ProtectedPathsHandler {
	return &ProtectedPathsHandler{db: db}
}

func (h *ProtectedPathsHandler) loadOwnedProject(c *gin.Context) (*models.Project, bool) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return nil, false
	}

	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project ID"})
		return nil, false
	}

	var project models.Project
	if err := h.db.Select("id", "owner_id").First(&project, projectID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return nil, false
	}
	if project.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return nil, false
	}

	return &project, true
}

// GetProtectedPaths returns the protected paths for a project.
// GET /projects/:id/protected-paths
func (h *ProtectedPathsHandler) GetProtectedPaths(c *gin.Context) {
	project, ok := h.loadOwnedProject(c)
	if !ok {
		return
	}

	var raw string
	if err := h.db.Table("projects").Where("id = ?", project.ID).Pluck("protected_paths", &raw).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	var paths []string
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &paths)
	}

	c.JSON(http.StatusOK, gin.H{"paths": paths})
}

// UpdateProtectedPaths sets the protected paths for a project.
// PUT /projects/:id/protected-paths
func (h *ProtectedPathsHandler) UpdateProtectedPaths(c *gin.Context) {
	project, ok := h.loadOwnedProject(c)
	if !ok {
		return
	}

	var req struct {
		Paths []string `json:"paths"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	data, _ := json.Marshal(req.Paths)
	if err := h.db.Table("projects").Where("id = ?", project.ID).Update("protected_paths", string(data)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update protected paths"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"paths": req.Paths})
}

// RegisterProtectedPathsRoutes registers protected path endpoints on a projects group.
func (h *ProtectedPathsHandler) RegisterProtectedPathsRoutes(projects *gin.RouterGroup) {
	projects.GET("/:id/protected-paths", h.GetProtectedPaths)
	projects.PUT("/:id/protected-paths", h.UpdateProtectedPaths)
}
