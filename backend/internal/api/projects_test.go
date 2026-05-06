package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"apex-build/internal/db"
	"apex-build/internal/mobile"
	secretstore "apex-build/internal/secrets"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newProjectAPITestServer(t *testing.T, subscriptionType string) (*Server, uint, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	gormDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gormDB.AutoMigrate(&models.User{}, &models.Project{}, &models.File{}, &models.CompletedBuild{}, &mobile.MobileBuildRecord{}, &secretstore.Secret{}, &secretstore.SecretAuditLog{}))

	user := models.User{
		Username:         strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_"),
		Email:            strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_") + "@example.com",
		PasswordHash:     "hashed-password",
		SubscriptionType: subscriptionType,
	}
	require.NoError(t, gormDB.Create(&user).Error)

	server := NewServer(&db.Database{DB: gormDB}, nil, nil, nil)
	return server, user.ID, gormDB
}

func TestDownloadProjectPreparesMobileExpoFilesForOwnerZip(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")

	project := models.Project{
		Name:           "Mobile ZIP",
		Language:       "typescript",
		OwnerID:        userID,
		TargetPlatform: string(mobile.TargetPlatformMobileExpo),
	}
	require.NoError(t, gormDB.Create(&project).Error)

	spec := mobile.FieldServiceContractorQuoteSpec()
	spec.App.Slug = "mobile-zip-spec"
	specJSON, err := json.Marshal(spec)
	require.NoError(t, err)
	require.NoError(t, gormDB.Create(&models.CompletedBuild{
		BuildID:        "mobile-zip-build",
		UserID:         userID,
		ProjectID:      &project.ID,
		Status:         "completed",
		TargetPlatform: string(mobile.TargetPlatformMobileExpo),
		MobileSpecJSON: string(specJSON),
	}).Error)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%d/download", project.ID), nil)
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	context.Set("user_id", userID)

	server.DownloadProject(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	zipReader, err := zip.NewReader(bytes.NewReader(recorder.Body.Bytes()), int64(recorder.Body.Len()))
	require.NoError(t, err)
	require.True(t, zipHasPath(zipReader, "mobile/package.json"))
	require.True(t, zipHasPath(zipReader, "mobile/app.config.ts"))
}

func TestCreateProjectPersistsMobileMetadata(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(`{
		"name":"Mobile Metadata",
		"language":"typescript",
		"target_platform":"mobile_expo",
		"mobile_platforms":["android","ios","android"],
		"mobile_framework":"expo-react-native",
		"mobile_release_level":"source_only",
		"mobile_capabilities":["offlineMode","fileUploads","offlineMode"],
		"mobile_dependency_policy":"expo-allowlist"
	}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("user_id", userID)
	context.Set("subscription_type", "pro")

	server.CreateProject(context)

	require.Equal(t, http.StatusCreated, recorder.Code)

	var project models.Project
	require.NoError(t, gormDB.Where("owner_id = ? AND name = ?", userID, "Mobile Metadata").First(&project).Error)
	require.Equal(t, string(mobile.TargetPlatformMobileExpo), project.TargetPlatform)
	require.Equal(t, []string{"android", "ios"}, project.MobilePlatforms)
	require.Equal(t, string(mobile.MobileFrameworkExpoReactNative), project.MobileFramework)
	require.Equal(t, string(mobile.ReleaseSourceOnly), project.MobileReleaseLevel)
	require.Equal(t, []string{"offlineMode", "fileUploads"}, project.MobileCapabilities)
	require.Equal(t, "expo-allowlist", project.MobileDependencyPolicy)
	require.Equal(t, "mobile/", project.GeneratedMobileClientPath)
	require.Equal(t, "source_only", project.MobilePreviewStatus)
	require.Equal(t, "not_requested", project.MobileBuildStatus)
	require.Equal(t, "not_requested", project.MobileStoreReadinessStatus)
}

func TestGetProjectMobileValidationPreparesAndReportsSourceStatus(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")

	project := models.Project{
		Name:           "Mobile Validation",
		Language:       "typescript",
		OwnerID:        userID,
		TargetPlatform: string(mobile.TargetPlatformMobileExpo),
	}
	require.NoError(t, gormDB.Create(&project).Error)

	spec := mobile.FieldServiceContractorQuoteSpec()
	spec.App.Slug = "mobile-validation-spec"
	specJSON, err := json.Marshal(spec)
	require.NoError(t, err)
	require.NoError(t, gormDB.Create(&models.CompletedBuild{
		BuildID:        "mobile-validation-build",
		UserID:         userID,
		ProjectID:      &project.ID,
		Status:         "completed",
		TargetPlatform: string(mobile.TargetPlatformMobileExpo),
		MobileSpecJSON: string(specJSON),
	}).Error)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%d/mobile/validation", project.ID), nil)
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	context.Set("user_id", userID)

	server.GetProjectMobileValidation(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Validation mobile.MobileValidationReport `json:"validation"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, mobile.MobileValidationPassed, response.Validation.Status)
	require.Equal(t, "draft_ready_needs_manual_store_assets", response.Validation.StoreReadinessState)
	require.True(t, hasValidationCheck(response.Validation, "release_truth", mobile.MobileValidationPassed))
}

func TestGetProjectMobileScorecardReportsBlockersUntilNativeProofExists(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")

	project := models.Project{
		Name:                      "Mobile Scorecard",
		Language:                  "typescript",
		OwnerID:                   userID,
		TargetPlatform:            string(mobile.TargetPlatformMobileExpo),
		MobilePlatforms:           []string{"android", "ios"},
		MobileFramework:           string(mobile.MobileFrameworkExpoReactNative),
		MobileReleaseLevel:        string(mobile.ReleaseSourceOnly),
		GeneratedMobileClientPath: "mobile/",
		MobileBuildStatus:         "not_requested",
	}
	require.NoError(t, gormDB.Create(&project).Error)

	spec := mobile.FieldServiceContractorQuoteSpec()
	spec.App.Slug = "mobile-scorecard-spec"
	specJSON, err := json.Marshal(spec)
	require.NoError(t, err)
	require.NoError(t, gormDB.Create(&models.CompletedBuild{
		BuildID:        "mobile-scorecard-build",
		UserID:         userID,
		ProjectID:      &project.ID,
		Status:         "completed",
		TargetPlatform: string(mobile.TargetPlatformMobileExpo),
		MobileSpecJSON: string(specJSON),
	}).Error)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%d/mobile/scorecard", project.ID), nil)
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	context.Set("user_id", userID)

	server.GetProjectMobileScorecard(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Scorecard mobile.MobileReadinessScorecard `json:"scorecard"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Scorecard.IsReady)
	require.Less(t, response.Scorecard.OverallScore, response.Scorecard.TargetScore)
	require.GreaterOrEqual(t, readinessCategoryScore(response.Scorecard, "source_generation"), 95)
	require.Equal(t, 0, readinessCategoryScore(response.Scorecard, "credentials_signing"))
	require.Contains(t, response.Scorecard.Blockers, "Add encrypted user-provided mobile credential vault and validate EAS/Apple/Google signing prerequisites.")
}

func TestCreateProjectRejectsPublicProjectOnFreePlan(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "free")

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(`{"name":"Public Demo","language":"typescript","is_public":true}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("user_id", userID)
	context.Set("subscription_type", "free")

	server.CreateProject(context)

	require.Equal(t, http.StatusPaymentRequired, recorder.Code)
	require.Contains(t, recorder.Body.String(), backendSubscriptionRequiredCode)

	var projectCount int64
	require.NoError(t, gormDB.Model(&models.Project{}).Count(&projectCount).Error)
	require.Zero(t, projectCount)
}

func TestCreateProjectRejectsInvalidUserContext(t *testing.T) {
	server, _, gormDB := newProjectAPITestServer(t, "pro")

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(`{"name":"Safe Project","language":"typescript"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("user_id", "not-a-uint")
	context.Set("subscription_type", "pro")

	server.CreateProject(context)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "AUTH_REQUIRED")

	var projectCount int64
	require.NoError(t, gormDB.Model(&models.Project{}).Count(&projectCount).Error)
	require.Zero(t, projectCount)
}

func zipHasPath(reader *zip.Reader, path string) bool {
	for _, file := range reader.File {
		if file.Name == path {
			return true
		}
	}
	return false
}

func hasValidationCheck(report mobile.MobileValidationReport, id string, status mobile.MobileValidationStatus) bool {
	for _, check := range report.Checks {
		if check.ID == id && check.Status == status {
			return true
		}
	}
	return false
}

func readinessCategoryScore(scorecard mobile.MobileReadinessScorecard, id string) int {
	for _, category := range scorecard.Categories {
		if category.ID == id {
			return category.Score
		}
	}
	return -1
}
