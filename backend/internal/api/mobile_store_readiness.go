package api

import (
	"net/http"

	appmiddleware "apex-build/internal/middleware"
	"apex-build/internal/mobile"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
)

// GetProjectMobileStoreReadiness returns a backend-owned store-readiness report.
func (s *Server) GetProjectMobileStoreReadiness(c *gin.Context) {
	projectID := c.Param("id")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	var project models.Project
	query := s.db.DB.WithContext(c.Request.Context()).Where("id = ?", projectID)
	query = query.Where("owner_id = ? OR is_public = ?", uid, true)
	if err := query.First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found", "code": "PROJECT_NOT_FOUND"})
		return
	}

	if project.OwnerID == uid {
		if err := mobile.PrepareExpoProjectFiles(c.Request.Context(), s.db.DB, project); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare mobile store-readiness files", "code": "MOBILE_STORE_READINESS_PREPARE_FAILED"})
			return
		}
	}

	files, err := s.fetchMobileReadinessFiles(c, project.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch mobile store-readiness files", "code": "MOBILE_STORE_READINESS_FILES_FAILED"})
		return
	}

	validation := mobile.ValidateProjectSourcePackage(project, files)
	scorecard := mobile.BuildMobileReadinessScorecard(project, files, validation)
	report := mobile.BuildMobileStoreReadinessReport(project, files, validation, scorecard)
	c.JSON(http.StatusOK, gin.H{"store_readiness": report})
}

func (s *Server) fetchMobileReadinessFiles(c *gin.Context, projectID uint) ([]models.File, error) {
	var files []models.File
	err := s.db.DB.WithContext(c.Request.Context()).
		Where(
			"project_id = ? AND (path LIKE ? OR path LIKE ? OR path LIKE ? OR path LIKE ? OR path = ? OR path = ?)",
			projectID,
			"mobile/%",
			"/mobile/%",
			"backend/%",
			"/backend/%",
			"docs/mobile-backend-routes.md",
			"/docs/mobile-backend-routes.md",
		).
		Find(&files).Error
	return files, err
}
