package api

import (
	"net/http"
	"strings"

	"apex-build/internal/mobile"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
)

type mobileSubmitRequestPayload struct {
	Track  string `json:"track,omitempty"`
	DryRun bool   `json:"dry_run,omitempty"`
}

func (s *Server) SubmitProjectMobileBuild(c *gin.Context) {
	build, project, ok := s.requireProjectMobileBuildWithProject(c)
	if !ok {
		return
	}
	if s.mobileSubmit == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Mobile submission service is not configured", "code": "MOBILE_SUBMISSION_SERVICE_UNAVAILABLE"})
		return
	}
	if build.Status != mobile.MobileBuildSucceeded {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only succeeded native builds can be submitted to store pipelines", "code": "INVALID_MOBILE_SUBMISSION_REQUEST", "build": build})
		return
	}
	if !requireMobileSubmissionEntitlement(c, s.db.DB, build.UserID, build.Platform) {
		return
	}

	var payload mobileSubmitRequestPayload
	if c.Request.Body != nil {
		_ = c.ShouldBindJSON(&payload)
	}

	if existing, found, err := s.existingActiveMobileSubmissionForBuild(c, project.ID, build.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to inspect existing mobile submissions", "code": "MOBILE_SUBMISSION_LIST_FAILED"})
		return
	} else if found {
		c.JSON(http.StatusConflict, gin.H{"error": "This native build already has a store-upload submission job", "code": "MOBILE_SUBMISSION_ALREADY_EXISTS", "submission": existing})
		return
	}

	credentials, err := mobile.NewMobileCredentialVault(s.db.DB, nil).Status(c.Request.Context(), build.UserID, project)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify mobile submission credentials", "code": "MOBILE_SUBMISSION_CREDENTIAL_STATUS_FAILED"})
		return
	}
	if !credentials.Complete {
		c.JSON(http.StatusConflict, gin.H{"error": "Required mobile submission credentials are missing", "code": "MOBILE_SUBMISSION_CREDENTIALS_REQUIRED", "credentials": credentials})
		return
	}

	storeReadiness, err := s.mobileStoreReadinessForProject(c, project)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to evaluate store readiness", "code": "MOBILE_STORE_READINESS_FAILED"})
		return
	}
	if storeReadiness.Package == nil || len(storeReadiness.Errors) > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Store-readiness package must be valid before submission upload", "code": "MOBILE_STORE_READINESS_REQUIRED", "store_readiness": storeReadiness})
		return
	}

	req := mobile.MobileSubmissionRequest{
		ProjectID:       build.ProjectID,
		UserID:          build.UserID,
		BuildID:         build.ID,
		Platform:        build.Platform,
		ArtifactURL:     build.ArtifactURL,
		ProviderBuildID: build.ProviderBuildID,
		Track:           payload.Track,
		DryRun:          payload.DryRun,
	}
	submission, err := s.mobileSubmit.CreateSubmission(c.Request.Context(), req)
	if submission.ID != "" {
		if persistErr := s.persistMobileSubmissionProjectSummary(c, project, submission); persistErr != nil && err == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to persist mobile submission status", "code": "MOBILE_SUBMISSION_STATUS_PERSIST_FAILED"})
			return
		}
	}
	if err != nil {
		s.writeMobileSubmissionError(c, err, &submission)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"submission": submission, "credentials": credentials, "store_readiness": storeReadiness})
}

func (s *Server) ListProjectMobileSubmissions(c *gin.Context) {
	_, project, ok := s.requireOwnedMobileExpoProject(c)
	if !ok {
		return
	}
	if s.mobileSubmit == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Mobile submission service is not configured", "code": "MOBILE_SUBMISSION_SERVICE_UNAVAILABLE"})
		return
	}
	submissions, err := s.mobileSubmit.ListProjectSubmissions(c.Request.Context(), project.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch mobile submissions", "code": "MOBILE_SUBMISSION_LIST_FAILED"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"submissions": submissions})
}

func (s *Server) GetProjectMobileSubmission(c *gin.Context) {
	_, project, ok := s.requireOwnedMobileExpoProject(c)
	if !ok {
		return
	}
	if s.mobileSubmit == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Mobile submission service is not configured", "code": "MOBILE_SUBMISSION_SERVICE_UNAVAILABLE"})
		return
	}
	submission, exists, err := s.mobileSubmit.GetSubmission(c.Request.Context(), c.Param("submissionId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch mobile submission", "code": "MOBILE_SUBMISSION_FETCH_FAILED"})
		return
	}
	if !exists || submission.ProjectID != project.ID || submission.UserID != project.OwnerID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Mobile submission not found", "code": "MOBILE_SUBMISSION_NOT_FOUND"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"submission": submission})
}

func (s *Server) mobileStoreReadinessForProject(c *gin.Context, project models.Project) (mobile.MobileStoreReadinessReport, error) {
	if err := mobile.PrepareExpoProjectFiles(c.Request.Context(), s.db.DB, project); err != nil {
		return mobile.MobileStoreReadinessReport{}, err
	}
	files, err := s.fetchMobileReadinessFiles(c, project.ID)
	if err != nil {
		return mobile.MobileStoreReadinessReport{}, err
	}
	validation := mobile.ValidateProjectSourcePackage(project, files)
	scorecard := mobile.BuildMobileReadinessScorecard(project, files, validation)
	return mobile.BuildMobileStoreReadinessReport(project, files, validation, scorecard), nil
}

func (s *Server) persistMobileSubmissionProjectSummary(c *gin.Context, project models.Project, submission mobile.MobileSubmissionJob) error {
	metadata := map[string]interface{}{}
	for key, value := range project.MobileMetadata {
		metadata[key] = value
	}
	metadata["last_mobile_submission_id"] = submission.ID
	metadata["last_mobile_submission_build_id"] = submission.BuildID
	metadata["last_mobile_submission_platform"] = string(submission.Platform)
	metadata["last_mobile_submission_status"] = string(submission.Status)
	if submission.ProviderSubmissionID != "" {
		metadata["last_mobile_provider_submission_id"] = submission.ProviderSubmissionID
	}
	if submission.FailureMessage != "" {
		metadata["last_mobile_submission_failure_message"] = mobile.RedactMobileBuildSecrets(submission.FailureMessage)
	}
	project.MobileMetadata = metadata
	switch submission.Status {
	case mobile.MobileSubmissionCompletedUpload,
		mobile.MobileSubmissionSubmittedToStorePipeline,
		mobile.MobileSubmissionReadyForGoogleInternalTesting,
		mobile.MobileSubmissionReadyForTestFlight,
		mobile.MobileSubmissionRequiresManualReviewSubmission:
		project.MobileStoreReadinessStatus = "submitted_to_store_pipeline"
	case mobile.MobileSubmissionFailed:
		project.MobileStoreReadinessStatus = "submission_failed"
	default:
		project.MobileStoreReadinessStatus = strings.TrimSpace(project.MobileStoreReadinessStatus)
		if project.MobileStoreReadinessStatus == "" {
			project.MobileStoreReadinessStatus = "draft_ready_needs_manual_store_assets"
		}
	}
	return s.db.DB.WithContext(c.Request.Context()).Select("MobileStoreReadinessStatus", "MobileMetadata").Save(&project).Error
}

func (s *Server) existingActiveMobileSubmissionForBuild(c *gin.Context, projectID uint, buildID string) (mobile.MobileSubmissionJob, bool, error) {
	if s.mobileSubmit == nil {
		return mobile.MobileSubmissionJob{}, false, nil
	}
	submissions, err := s.mobileSubmit.ListProjectSubmissions(c.Request.Context(), projectID)
	if err != nil {
		return mobile.MobileSubmissionJob{}, false, err
	}
	buildID = strings.TrimSpace(buildID)
	for _, submission := range submissions {
		if strings.TrimSpace(submission.BuildID) != buildID {
			continue
		}
		if submission.Status != mobile.MobileSubmissionFailed {
			return submission, true, nil
		}
	}
	return mobile.MobileSubmissionJob{}, false, nil
}

func (s *Server) writeMobileSubmissionError(c *gin.Context, err error, submission *mobile.MobileSubmissionJob) {
	body := gin.H{"error": err.Error()}
	status := http.StatusInternalServerError
	switch {
	case strings.Contains(err.Error(), mobile.ErrMobileBuildsDisabled.Error()), strings.Contains(err.Error(), mobile.ErrMobileBuildPlatformDisabled.Error()):
		status = http.StatusServiceUnavailable
		body["code"] = "MOBILE_SUBMISSION_DISABLED"
	case strings.Contains(err.Error(), mobile.ErrMobileBuildInvalidRequest.Error()):
		status = http.StatusBadRequest
		body["code"] = "INVALID_MOBILE_SUBMISSION_REQUEST"
	case strings.Contains(err.Error(), mobile.ErrMobileBuildProviderMissing.Error()):
		status = http.StatusServiceUnavailable
		body["code"] = "MOBILE_SUBMISSION_PROVIDER_MISSING"
	case strings.Contains(err.Error(), mobile.ErrMobileBuildProviderFailed.Error()):
		status = http.StatusBadGateway
		body["code"] = "MOBILE_SUBMISSION_PROVIDER_FAILED"
	default:
		body["code"] = "MOBILE_SUBMISSION_FAILED"
	}
	if submission != nil && strings.TrimSpace(submission.ID) != "" {
		body["submission"] = *submission
	}
	c.JSON(status, body)
}
