package api

import (
	"errors"
	"net/http"
	"strings"

	appmiddleware "apex-build/internal/middleware"
	"apex-build/internal/mobile"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
)

type mobileBuildRequestPayload struct {
	Platform     mobile.MobilePlatform     `json:"platform"`
	Profile      mobile.MobileBuildProfile `json:"profile"`
	ReleaseLevel mobile.MobileReleaseLevel `json:"release_level"`
	AppVersion   string                    `json:"app_version,omitempty"`
	BuildNumber  string                    `json:"build_number,omitempty"`
	VersionCode  int                       `json:"version_code,omitempty"`
	CommitRef    string                    `json:"commit_ref,omitempty"`
	SourcePath   string                    `json:"source_path,omitempty"`
	DryRun       bool                      `json:"dry_run,omitempty"`
}

func (s *Server) CreateProjectMobileBuild(c *gin.Context) {
	uid, project, ok := s.requireOwnedMobileExpoProject(c)
	if !ok {
		return
	}
	if s.mobile == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Mobile build service is not configured",
			"code":  "MOBILE_BUILD_SERVICE_UNAVAILABLE",
		})
		return
	}

	var payload mobileBuildRequestPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid mobile build request", "code": "INVALID_REQUEST"})
		return
	}

	req := mobile.MobileBuildRequest{
		ProjectID:    project.ID,
		UserID:       uid,
		Platform:     normalizeRequestedMobileBuildPlatform(project, payload.Platform),
		Profile:      payload.Profile,
		ReleaseLevel: payload.ReleaseLevel,
		AppVersion:   payload.AppVersion,
		BuildNumber:  payload.BuildNumber,
		VersionCode:  payload.VersionCode,
		CommitRef:    payload.CommitRef,
		SourcePath:   payload.SourcePath,
		DryRun:       payload.DryRun,
	}
	if err := s.mobile.ValidateRequest(req); err != nil {
		s.writeMobileBuildError(c, err, nil)
		return
	}

	credentials, err := mobile.NewMobileCredentialVault(s.db.DB, nil).Status(c.Request.Context(), uid, project)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify mobile credentials", "code": "MOBILE_CREDENTIAL_STATUS_FAILED"})
		return
	}
	if !credentials.Complete {
		c.JSON(http.StatusConflict, gin.H{
			"error":       "Required mobile build credentials are missing",
			"code":        "MOBILE_CREDENTIALS_REQUIRED",
			"credentials": credentials,
		})
		return
	}

	job, err := s.mobile.CreateBuild(c.Request.Context(), req)
	if job.ID != "" {
		if persistErr := s.persistMobileBuildProjectSummary(c, project, job); persistErr != nil && err == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to persist mobile build status", "code": "MOBILE_BUILD_STATUS_PERSIST_FAILED"})
			return
		}
	}
	if err != nil {
		s.writeMobileBuildError(c, err, &job)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"build": job, "credentials": credentials})
}

func (s *Server) ListProjectMobileBuilds(c *gin.Context) {
	_, project, ok := s.requireOwnedMobileExpoProject(c)
	if !ok {
		return
	}
	if s.mobile == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Mobile build service is not configured", "code": "MOBILE_BUILD_SERVICE_UNAVAILABLE"})
		return
	}
	jobs, err := s.mobile.ListProjectBuilds(c.Request.Context(), project.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch mobile builds", "code": "MOBILE_BUILD_LIST_FAILED"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"builds": jobs})
}

func (s *Server) GetProjectMobileBuild(c *gin.Context) {
	job, ok := s.requireProjectMobileBuild(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"build": job})
}

func (s *Server) GetProjectMobileBuildLogs(c *gin.Context) {
	job, ok := s.requireProjectMobileBuild(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"build_id": job.ID, "logs": job.Logs})
}

func (s *Server) GetProjectMobileBuildArtifacts(c *gin.Context) {
	job, ok := s.requireProjectMobileBuild(c)
	if !ok {
		return
	}
	if strings.TrimSpace(job.ArtifactURL) == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "No mobile build artifact is available yet", "code": "MOBILE_BUILD_ARTIFACT_NOT_READY"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"build_id":      job.ID,
		"artifact_url":  job.ArtifactURL,
		"platform":      job.Platform,
		"profile":       job.Profile,
		"release_level": job.ReleaseLevel,
	})
}

func (s *Server) requireProjectMobileBuild(c *gin.Context) (mobile.MobileBuildJob, bool) {
	_, project, ok := s.requireOwnedMobileExpoProject(c)
	if !ok {
		return mobile.MobileBuildJob{}, false
	}
	if s.mobile == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Mobile build service is not configured", "code": "MOBILE_BUILD_SERVICE_UNAVAILABLE"})
		return mobile.MobileBuildJob{}, false
	}
	job, exists, err := s.mobile.GetBuild(c.Request.Context(), c.Param("buildId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch mobile build", "code": "MOBILE_BUILD_FETCH_FAILED"})
		return mobile.MobileBuildJob{}, false
	}
	if !exists || job.ProjectID != project.ID || job.UserID != project.OwnerID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Mobile build not found", "code": "MOBILE_BUILD_NOT_FOUND"})
		return mobile.MobileBuildJob{}, false
	}
	return job, true
}

func (s *Server) requireOwnedMobileExpoProject(c *gin.Context) (uint, models.Project, bool) {
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return 0, models.Project{}, false
	}
	var project models.Project
	if err := s.db.DB.WithContext(c.Request.Context()).
		Where("id = ? AND owner_id = ?", c.Param("id"), uid).
		First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found", "code": "PROJECT_NOT_FOUND"})
		return 0, models.Project{}, false
	}
	if !strings.EqualFold(project.TargetPlatform, string(mobile.TargetPlatformMobileExpo)) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Native mobile builds require a mobile_expo project",
			"code":  "NOT_MOBILE_EXPO_PROJECT",
		})
		return 0, models.Project{}, false
	}
	return uid, project, true
}

func (s *Server) persistMobileBuildProjectSummary(c *gin.Context, project models.Project, job mobile.MobileBuildJob) error {
	mobile.ApplyMobileBuildJobToProject(&project, job)
	return s.db.DB.WithContext(c.Request.Context()).
		Select("MobileBuildStatus", "MobileMetadata").
		Save(&project).
		Error
}

func normalizeRequestedMobileBuildPlatform(project models.Project, requested mobile.MobilePlatform) mobile.MobilePlatform {
	if requested != "" {
		return requested
	}
	if len(project.MobilePlatforms) == 1 {
		return mobile.MobilePlatform(project.MobilePlatforms[0])
	}
	return requested
}

func (s *Server) writeMobileBuildError(c *gin.Context, err error, job *mobile.MobileBuildJob) {
	body := gin.H{"error": err.Error()}
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, mobile.ErrMobileBuildsDisabled), errors.Is(err, mobile.ErrMobileBuildPlatformDisabled):
		status = http.StatusServiceUnavailable
		body["code"] = "MOBILE_BUILD_DISABLED"
	case errors.Is(err, mobile.ErrMobileBuildInvalidRequest):
		status = http.StatusBadRequest
		body["code"] = "INVALID_MOBILE_BUILD_REQUEST"
	case errors.Is(err, mobile.ErrMobileBuildProviderMissing):
		status = http.StatusServiceUnavailable
		body["code"] = "MOBILE_BUILD_PROVIDER_MISSING"
	case errors.Is(err, mobile.ErrMobileBuildProviderFailed):
		status = http.StatusBadGateway
		body["code"] = "MOBILE_BUILD_PROVIDER_FAILED"
	default:
		body["code"] = "MOBILE_BUILD_FAILED"
	}
	if job != nil && job.ID != "" {
		body["build"] = *job
	}
	c.JSON(status, body)
}
