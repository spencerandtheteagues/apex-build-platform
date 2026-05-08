package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"apex-build/internal/mobile"
	secretstore "apex-build/internal/secrets"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCreateProjectMobileBuildRejectsWhenFeatureFlagsDisabled(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	provider := &mockAPIMobileBuildProvider{}
	server.SetMobileBuildService(mobile.NewMobileBuildService(mobile.FeatureFlags{}, provider, mobile.NewGormMobileBuildStore(gormDB)))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds", project.ID), strings.NewReader(`{
		"platform":"android",
		"profile":"preview",
		"release_level":"internal_android_apk"
	}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	context.Set("user_id", userID)

	server.CreateProjectMobileBuild(context)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "MOBILE_BUILD_DISABLED")
	require.Zero(t, provider.calls)
}

func TestCreateProjectMobileBuildRequiresValidatedCredentials(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	provider := &mockAPIMobileBuildProvider{}
	server.SetMobileBuildService(mobile.NewMobileBuildService(mobileBuildAPITestFlags(), provider, mobile.NewGormMobileBuildStore(gormDB)))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds", project.ID), strings.NewReader(`{
		"platform":"android",
		"profile":"preview",
		"release_level":"internal_android_apk"
	}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	context.Set("user_id", userID)

	server.CreateProjectMobileBuild(context)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), "MOBILE_CREDENTIALS_REQUIRED")
	require.Contains(t, recorder.Body.String(), "eas_token")
	require.NotContains(t, recorder.Body.String(), "google_play_service_account")
	require.Zero(t, provider.calls)
}

func TestCreateProjectMobileBuildRequiresPaidMobilePlan(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "free")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	provider := &mockAPIMobileBuildProvider{}
	server.SetMobileBuildService(mobile.NewMobileBuildService(mobileBuildAPITestFlags(), provider, mobile.NewGormMobileBuildStore(gormDB)))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds", project.ID), strings.NewReader(`{
		"platform":"android",
		"profile":"preview",
		"release_level":"internal_android_apk"
	}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	context.Set("user_id", userID)

	server.CreateProjectMobileBuild(context)

	require.Equal(t, http.StatusPaymentRequired, recorder.Code)
	require.Contains(t, recorder.Body.String(), mobileBuildPlanRequiredCode)
	require.Contains(t, recorder.Body.String(), `"required_plan":"builder"`)
	require.Zero(t, provider.calls)
}

func TestCreateProjectMobileBuildRequiresProForIOS(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "builder")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"ios"})
	provider := &mockAPIMobileBuildProvider{}
	server.SetMobileBuildService(mobile.NewMobileBuildService(mobileBuildAPITestFlags(), provider, mobile.NewGormMobileBuildStore(gormDB)))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds", project.ID), strings.NewReader(`{
		"platform":"ios",
		"profile":"internal",
		"release_level":"ios_internal"
	}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	context.Set("user_id", userID)

	server.CreateProjectMobileBuild(context)

	require.Equal(t, http.StatusPaymentRequired, recorder.Code)
	require.Contains(t, recorder.Body.String(), mobileBuildPlanRequiredCode)
	require.Contains(t, recorder.Body.String(), `"required_plan":"pro"`)
	require.Zero(t, provider.calls)
}

func TestCreateProjectMobileBuildEnforcesMonthlyQuota(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "builder")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	buildStore := mobile.NewGormMobileBuildStore(gormDB)
	for i := 0; i < 5; i++ {
		require.NoError(t, buildStore.Save(context.Background(), mobile.MobileBuildJob{
			ID:           fmt.Sprintf("mbld_quota_%d", i),
			ProjectID:    project.ID,
			UserID:       userID,
			Platform:     mobile.MobilePlatformAndroid,
			Profile:      mobile.MobileBuildProfilePreview,
			ReleaseLevel: mobile.ReleaseInternalAndroidAPK,
			Status:       mobile.MobileBuildFailed,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}))
	}
	provider := &mockAPIMobileBuildProvider{}
	server.SetMobileBuildService(mobile.NewMobileBuildService(mobileBuildAPITestFlags(), provider, buildStore))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds", project.ID), strings.NewReader(`{
		"platform":"android",
		"profile":"preview",
		"release_level":"internal_android_apk"
	}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	context.Set("user_id", userID)

	server.CreateProjectMobileBuild(context)

	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	require.Contains(t, recorder.Body.String(), mobileBuildQuotaExceededCode)
	require.Contains(t, recorder.Body.String(), `"monthly_limit":5`)
	require.Zero(t, provider.calls)
}

func TestCreateProjectMobileBuildReportsMissingProviderAfterCredentials(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	storeRequiredAndroidMobileCredentials(t, gormDB, userID, project)
	server.SetMobileBuildService(mobile.NewMobileBuildService(mobileBuildAPITestFlags(), nil, mobile.NewGormMobileBuildStore(gormDB)))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds", project.ID), strings.NewReader(`{
		"platform":"android",
		"profile":"preview",
		"release_level":"internal_android_apk"
	}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	context.Set("user_id", userID)

	server.CreateProjectMobileBuild(context)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "MOBILE_BUILD_PROVIDER_MISSING")
}

func TestCreateProjectMobileBuildRejectsPlatformNotEnabledForProject(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	provider := &mockAPIMobileBuildProvider{}
	server.SetMobileBuildService(mobile.NewMobileBuildService(mobileBuildAPITestFlags(), provider, mobile.NewGormMobileBuildStore(gormDB)))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds", project.ID), strings.NewReader(`{
		"platform":"ios",
		"profile":"preview",
		"release_level":"ios_internal"
	}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	context.Set("user_id", userID)

	server.CreateProjectMobileBuild(context)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "INVALID_MOBILE_BUILD_REQUEST")
	require.Contains(t, recorder.Body.String(), "not enabled")
	require.Zero(t, provider.calls)
}

func TestCreateProjectMobileBuildPersistsJobAndArtifactEndpoints(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	storeEASMobileCredential(t, gormDB, userID, project)
	provider := &mockAPIMobileBuildProvider{
		result: mobile.MobileBuildProviderResult{
			ProviderBuildID: "eas-build-android-1",
			Status:          mobile.MobileBuildSucceeded,
			ArtifactURL:     "https://artifacts.example.com/app.apk",
			Logs: []mobile.MobileBuildLogLine{{
				Level:   "info",
				Message: "uploaded with EAS_TOKEN=secret-token",
			}},
		},
	}
	server.SetMobileBuildService(mobile.NewMobileBuildService(
		mobileBuildAPITestFlags(),
		provider,
		mobile.NewGormMobileBuildStore(gormDB),
		mobile.WithMobileBuildIDGenerator(func() string { return "mbld_api_success" }),
	))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds", project.ID), strings.NewReader(`{
		"platform":"android",
		"profile":"preview",
		"release_level":"internal_android_apk",
		"app_version":"1.2.3",
		"version_code":12,
		"source_path":"/etc"
	}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	context.Set("user_id", userID)

	server.CreateProjectMobileBuild(context)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "secret-token")
	var createResponse struct {
		Build mobile.MobileBuildJob `json:"build"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &createResponse))
	require.Equal(t, "mbld_api_success", createResponse.Build.ID)
	require.Equal(t, mobile.MobileBuildSucceeded, createResponse.Build.Status)
	require.Equal(t, "https://artifacts.example.com/app.apk", createResponse.Build.ArtifactURL)
	require.Equal(t, 1, provider.calls)
	require.NotEqual(t, "/etc", provider.lastReq.SourcePath)
	require.True(t, filepath.IsAbs(provider.lastReq.SourcePath), "provider source path should be an absolute materialized path, got %q", provider.lastReq.SourcePath)
	require.True(t, provider.sourcePackageExists, "provider should receive a materialized Expo source directory containing package.json")

	var updated models.Project
	require.NoError(t, gormDB.First(&updated, project.ID).Error)
	require.Equal(t, string(mobile.MobileBuildSucceeded), updated.MobileBuildStatus)
	require.Equal(t, "https://artifacts.example.com/app.apk", mobileMetadataStringForAPITest(updated.MobileMetadata, "android_apk_url"))

	listRecorder := httptest.NewRecorder()
	listContext, _ := gin.CreateTestContext(listRecorder)
	listContext.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%d/mobile/builds", project.ID), nil)
	listContext.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	listContext.Set("user_id", userID)
	server.ListProjectMobileBuilds(listContext)
	require.Equal(t, http.StatusOK, listRecorder.Code)
	require.Contains(t, listRecorder.Body.String(), "mbld_api_success")

	logRecorder := httptest.NewRecorder()
	logContext, _ := gin.CreateTestContext(logRecorder)
	logContext.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%d/mobile/builds/mbld_api_success/logs", project.ID), nil)
	logContext.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}, {Key: "buildId", Value: "mbld_api_success"}}
	logContext.Set("user_id", userID)
	server.GetProjectMobileBuildLogs(logContext)
	require.Equal(t, http.StatusOK, logRecorder.Code)
	require.NotContains(t, logRecorder.Body.String(), "secret-token")
	require.Contains(t, logRecorder.Body.String(), "[REDACTED]")

	artifactRecorder := httptest.NewRecorder()
	artifactContext, _ := gin.CreateTestContext(artifactRecorder)
	artifactContext.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%d/mobile/builds/mbld_api_success/artifacts", project.ID), nil)
	artifactContext.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}, {Key: "buildId", Value: "mbld_api_success"}}
	artifactContext.Set("user_id", userID)
	server.GetProjectMobileBuildArtifacts(artifactContext)
	require.Equal(t, http.StatusOK, artifactRecorder.Code)
	require.Contains(t, artifactRecorder.Body.String(), "https://artifacts.example.com/app.apk")
}

func TestCreateProjectMobileBuildReturnsRepairPlanForProviderFailure(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	storeEASMobileCredential(t, gormDB, userID, project)
	provider := &mockAPIMobileBuildProvider{
		err: fmt.Errorf("Metro bundle failed with EAS_TOKEN=secret-token"),
	}
	server.SetMobileBuildService(mobile.NewMobileBuildService(
		mobileBuildAPITestFlags(),
		provider,
		mobile.NewGormMobileBuildStore(gormDB),
		mobile.WithMobileBuildIDGenerator(func() string { return "mbld_api_failure_repair" }),
	))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds", project.ID), strings.NewReader(`{
		"platform":"android",
		"profile":"preview",
		"release_level":"internal_android_apk"
	}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	context.Set("user_id", userID)

	server.CreateProjectMobileBuild(context)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "secret-token")
	require.Contains(t, recorder.Body.String(), `"repair_plan"`)
	require.Contains(t, recorder.Body.String(), `"failure_type":"metro_bundle_failed"`)
	require.Contains(t, recorder.Body.String(), `"requires_source_change":true`)

	var response struct {
		Build mobile.MobileBuildJob `json:"build"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, mobile.MobileBuildFailed, response.Build.Status)
	require.NotNil(t, response.Build.RepairPlan)
	require.Equal(t, mobile.MobileBuildFailureMetroBundleFailed, response.Build.RepairPlan.FailureType)

	var updated models.Project
	require.NoError(t, gormDB.First(&updated, project.ID).Error)
	require.Equal(t, string(mobile.MobileBuildFailed), updated.MobileBuildStatus)
	require.Equal(t, "metro_bundle_failed", mobileMetadataStringForAPITest(updated.MobileMetadata, "last_mobile_build_failure_type"))
	require.Equal(t, "Repair Metro bundling", mobileMetadataStringForAPITest(updated.MobileMetadata, "last_mobile_build_repair_title"))
}

func TestRefreshProjectMobileBuildUpdatesProviderStatusAndProjectSummary(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	storeEASMobileCredential(t, gormDB, userID, project)
	provider := &mockAPIMobileBuildProvider{
		result: mobile.MobileBuildProviderResult{
			ProviderBuildID: "eas-build-android-refresh",
			Status:          mobile.MobileBuildBuilding,
			Logs: []mobile.MobileBuildLogLine{{
				Level:   "info",
				Message: "queued",
			}},
		},
		refreshResult: mobile.MobileBuildProviderResult{
			Status:      mobile.MobileBuildSucceeded,
			ArtifactURL: "https://artifacts.example.com/refreshed.apk",
			Logs: []mobile.MobileBuildLogLine{{
				Level:   "info",
				Message: "finished with EAS_TOKEN=refresh-secret",
			}},
		},
	}
	server.SetMobileBuildService(mobile.NewMobileBuildService(
		mobileBuildAPITestFlags(),
		provider,
		mobile.NewGormMobileBuildStore(gormDB),
		mobile.WithMobileBuildIDGenerator(func() string { return "mbld_api_refresh" }),
	))

	createRecorder := httptest.NewRecorder()
	createContext, _ := gin.CreateTestContext(createRecorder)
	createContext.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds", project.ID), strings.NewReader(`{
		"platform":"android",
		"profile":"preview",
		"release_level":"internal_android_apk"
	}`))
	createContext.Request.Header.Set("Content-Type", "application/json")
	createContext.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	createContext.Set("user_id", userID)
	server.CreateProjectMobileBuild(createContext)
	require.Equal(t, http.StatusCreated, createRecorder.Code)

	refreshRecorder := httptest.NewRecorder()
	refreshContext, _ := gin.CreateTestContext(refreshRecorder)
	refreshContext.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds/mbld_api_refresh/refresh", project.ID), nil)
	refreshContext.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}, {Key: "buildId", Value: "mbld_api_refresh"}}
	refreshContext.Set("user_id", userID)
	server.RefreshProjectMobileBuild(refreshContext)

	require.Equal(t, http.StatusOK, refreshRecorder.Code)
	require.NotContains(t, refreshRecorder.Body.String(), "refresh-secret")
	require.Contains(t, refreshRecorder.Body.String(), "https://artifacts.example.com/refreshed.apk")
	require.Equal(t, 1, provider.refreshCalls)

	var updated models.Project
	require.NoError(t, gormDB.First(&updated, project.ID).Error)
	require.Equal(t, string(mobile.MobileBuildSucceeded), updated.MobileBuildStatus)
	require.Equal(t, "https://artifacts.example.com/refreshed.apk", mobileMetadataStringForAPITest(updated.MobileMetadata, "android_apk_url"))
}

func TestCancelProjectMobileBuildUpdatesProviderStatus(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	storeEASMobileCredential(t, gormDB, userID, project)
	provider := &mockAPIMobileBuildProvider{
		result: mobile.MobileBuildProviderResult{
			ProviderBuildID: "eas-build-android-cancel",
			Status:          mobile.MobileBuildBuilding,
		},
		cancelResult: mobile.MobileBuildProviderResult{
			Status: mobile.MobileBuildCanceled,
			Logs: []mobile.MobileBuildLogLine{{
				Level:   "info",
				Message: "cancelled with EAS_TOKEN=cancel-secret",
			}},
		},
	}
	server.SetMobileBuildService(mobile.NewMobileBuildService(
		mobileBuildAPITestFlags(),
		provider,
		mobile.NewGormMobileBuildStore(gormDB),
		mobile.WithMobileBuildIDGenerator(func() string { return "mbld_api_cancel" }),
	))

	createRecorder := httptest.NewRecorder()
	createContext, _ := gin.CreateTestContext(createRecorder)
	createContext.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds", project.ID), strings.NewReader(`{
		"platform":"android",
		"profile":"preview",
		"release_level":"internal_android_apk"
	}`))
	createContext.Request.Header.Set("Content-Type", "application/json")
	createContext.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	createContext.Set("user_id", userID)
	server.CreateProjectMobileBuild(createContext)
	require.Equal(t, http.StatusCreated, createRecorder.Code)

	cancelRecorder := httptest.NewRecorder()
	cancelContext, _ := gin.CreateTestContext(cancelRecorder)
	cancelContext.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds/mbld_api_cancel/cancel", project.ID), nil)
	cancelContext.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}, {Key: "buildId", Value: "mbld_api_cancel"}}
	cancelContext.Set("user_id", userID)
	server.CancelProjectMobileBuild(cancelContext)

	require.Equal(t, http.StatusOK, cancelRecorder.Code)
	require.NotContains(t, cancelRecorder.Body.String(), "cancel-secret")
	require.Contains(t, cancelRecorder.Body.String(), string(mobile.MobileBuildCanceled))
	require.Equal(t, 1, provider.cancelCalls)
}

func TestRetryProjectMobileBuildQueuesNewProviderJob(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	storeEASMobileCredential(t, gormDB, userID, project)
	store := mobile.NewGormMobileBuildStore(gormDB)
	require.NoError(t, store.Save(context.Background(), mobile.MobileBuildJob{
		ID:           "mbld_api_failed",
		ProjectID:    project.ID,
		UserID:       userID,
		Platform:     mobile.MobilePlatformAndroid,
		Profile:      mobile.MobileBuildProfilePreview,
		ReleaseLevel: mobile.ReleaseInternalAndroidAPK,
		Status:       mobile.MobileBuildFailed,
		AppVersion:   "1.2.3",
		VersionCode:  12,
	}))
	provider := &mockAPIMobileBuildProvider{
		result: mobile.MobileBuildProviderResult{
			ProviderBuildID: "eas-build-android-retry",
			Status:          mobile.MobileBuildQueued,
			Logs: []mobile.MobileBuildLogLine{{
				Level:   "info",
				Message: "retry queued with EAS_TOKEN=retry-secret",
			}},
		},
	}
	server.SetMobileBuildService(mobile.NewMobileBuildService(
		mobileBuildAPITestFlags(),
		provider,
		store,
		mobile.WithMobileBuildIDGenerator(func() string { return "mbld_api_retry" }),
	))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds/mbld_api_failed/retry", project.ID), nil)
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}, {Key: "buildId", Value: "mbld_api_failed"}}
	context.Set("user_id", userID)

	server.RetryProjectMobileBuild(context)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "retry-secret")
	require.Contains(t, recorder.Body.String(), "mbld_api_retry")
	require.Equal(t, 1, provider.calls)
	require.Equal(t, mobile.MobilePlatformAndroid, provider.lastReq.Platform)
	require.Equal(t, mobile.MobileBuildProfilePreview, provider.lastReq.Profile)
	require.Equal(t, mobile.ReleaseInternalAndroidAPK, provider.lastReq.ReleaseLevel)
	require.NotEmpty(t, provider.lastReq.SourcePath)
	require.True(t, provider.sourcePackageExists)

	var updated models.Project
	require.NoError(t, gormDB.First(&updated, project.ID).Error)
	require.Equal(t, string(mobile.MobileBuildQueued), updated.MobileBuildStatus)
	require.Equal(t, "mbld_api_retry", mobileMetadataStringForAPITest(updated.MobileMetadata, "last_mobile_build_id"))
}

func TestRetryProjectMobileBuildBlocksSourceFailureUntilValidationPasses(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	storeEASMobileCredential(t, gormDB, userID, project)
	store := mobile.NewGormMobileBuildStore(gormDB)
	require.NoError(t, store.Save(context.Background(), mobile.MobileBuildJob{
		ID:             "mbld_api_source_failed",
		ProjectID:      project.ID,
		UserID:         userID,
		Platform:       mobile.MobilePlatformAndroid,
		Profile:        mobile.MobileBuildProfilePreview,
		ReleaseLevel:   mobile.ReleaseInternalAndroidAPK,
		Status:         mobile.MobileBuildFailed,
		FailureType:    mobile.MobileBuildFailureMetroBundleFailed,
		FailureMessage: "Metro bundle failed.",
	}))
	spec := mobile.FieldServiceContractorQuoteSpec()
	files, errs := mobile.GenerateExpoProject(spec, mobile.ExpoGeneratorOptions{})
	require.Empty(t, errs)
	for _, file := range files {
		if file.Path == "mobile/package.json" {
			createMobileBuildAPIFile(t, gormDB, project, file.Path, `{"scripts":`)
			continue
		}
		createMobileBuildAPIFile(t, gormDB, project, file.Path, file.Content)
	}
	provider := &mockAPIMobileBuildProvider{}
	server.SetMobileBuildService(mobile.NewMobileBuildService(mobileBuildAPITestFlags(), provider, store))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds/mbld_api_source_failed/retry", project.ID), nil)
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}, {Key: "buildId", Value: "mbld_api_source_failed"}}
	context.Set("user_id", userID)

	server.RetryProjectMobileBuild(context)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), "MOBILE_BUILD_PRE_RETRY_VALIDATION_FAILED")
	require.Contains(t, recorder.Body.String(), "package.json is not valid JSON")
	require.Contains(t, recorder.Body.String(), `"repair_plan"`)
	require.Equal(t, 0, provider.calls)
}

func TestRetryProjectMobileBuildAllowsCredentialFailureWithoutSourceValidation(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	storeEASMobileCredential(t, gormDB, userID, project)
	store := mobile.NewGormMobileBuildStore(gormDB)
	require.NoError(t, store.Save(context.Background(), mobile.MobileBuildJob{
		ID:             "mbld_api_credential_failed",
		ProjectID:      project.ID,
		UserID:         userID,
		Platform:       mobile.MobilePlatformAndroid,
		Profile:        mobile.MobileBuildProfilePreview,
		ReleaseLevel:   mobile.ReleaseInternalAndroidAPK,
		Status:         mobile.MobileBuildFailed,
		FailureType:    mobile.MobileBuildFailureAndroidSigningFailed,
		FailureMessage: "Android signing failed.",
	}))
	spec := mobile.FieldServiceContractorQuoteSpec()
	files, errs := mobile.GenerateExpoProject(spec, mobile.ExpoGeneratorOptions{})
	require.Empty(t, errs)
	for _, file := range files {
		if file.Path == "mobile/eas.json" {
			continue
		}
		createMobileBuildAPIFile(t, gormDB, project, file.Path, file.Content)
	}
	provider := &mockAPIMobileBuildProvider{
		result: mobile.MobileBuildProviderResult{
			ProviderBuildID: "eas-build-credential-retry",
			Status:          mobile.MobileBuildQueued,
		},
	}
	server.SetMobileBuildService(mobile.NewMobileBuildService(
		mobileBuildAPITestFlags(),
		provider,
		store,
		mobile.WithMobileBuildIDGenerator(func() string { return "mbld_api_credential_retry" }),
	))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds/mbld_api_credential_failed/retry", project.ID), nil)
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}, {Key: "buildId", Value: "mbld_api_credential_failed"}}
	context.Set("user_id", userID)

	server.RetryProjectMobileBuild(context)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Contains(t, recorder.Body.String(), "mbld_api_credential_retry")
	require.Equal(t, 1, provider.calls)
}

func TestRepairProjectMobileBuildMarksSourceFailureReadyForRetryAfterValidation(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	storeEASMobileCredential(t, gormDB, userID, project)
	store := mobile.NewGormMobileBuildStore(gormDB)
	require.NoError(t, store.Save(context.Background(), mobile.MobileBuildJob{
		ID:             "mbld_api_repair_source",
		ProjectID:      project.ID,
		UserID:         userID,
		Platform:       mobile.MobilePlatformAndroid,
		Profile:        mobile.MobileBuildProfilePreview,
		ReleaseLevel:   mobile.ReleaseInternalAndroidAPK,
		Status:         mobile.MobileBuildFailed,
		FailureType:    mobile.MobileBuildFailureMetroBundleFailed,
		FailureMessage: "Metro bundle failed.",
	}))
	spec := mobile.FieldServiceContractorQuoteSpec()
	files, errs := mobile.GenerateExpoProject(spec, mobile.ExpoGeneratorOptions{})
	require.Empty(t, errs)
	for _, file := range files {
		createMobileBuildAPIFile(t, gormDB, project, file.Path, file.Content)
	}
	server.SetMobileBuildService(mobile.NewMobileBuildService(mobileBuildAPITestFlags(), &mockAPIMobileBuildProvider{}, store))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds/mbld_api_repair_source/repair", project.ID), nil)
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}, {Key: "buildId", Value: "mbld_api_repair_source"}}
	context.Set("user_id", userID)

	server.RepairProjectMobileBuild(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"repaired":true`)
	require.Contains(t, recorder.Body.String(), string(mobile.MobileBuildRepairedRetryPending))

	var response struct {
		Build      mobile.MobileBuildJob         `json:"build"`
		Validation mobile.MobileValidationReport `json:"validation"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, mobile.MobileBuildRepairedRetryPending, response.Build.Status)
	require.Equal(t, mobile.MobileValidationPassed, response.Validation.Status)

	var updated models.Project
	require.NoError(t, gormDB.First(&updated, project.ID).Error)
	require.Equal(t, string(mobile.MobileBuildRepairedRetryPending), updated.MobileBuildStatus)
	require.Equal(t, "mbld_api_repair_source", mobileMetadataStringForAPITest(updated.MobileMetadata, "last_mobile_build_id"))
}

func TestRepairProjectMobileBuildRequiresCredentialRepairForSigningFailure(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	storeEASMobileCredential(t, gormDB, userID, project)
	store := mobile.NewGormMobileBuildStore(gormDB)
	require.NoError(t, store.Save(context.Background(), mobile.MobileBuildJob{
		ID:             "mbld_api_repair_signing",
		ProjectID:      project.ID,
		UserID:         userID,
		Platform:       mobile.MobilePlatformAndroid,
		Profile:        mobile.MobileBuildProfilePreview,
		ReleaseLevel:   mobile.ReleaseInternalAndroidAPK,
		Status:         mobile.MobileBuildFailed,
		FailureType:    mobile.MobileBuildFailureAndroidSigningFailed,
		FailureMessage: "Android signing failed.",
	}))
	server.SetMobileBuildService(mobile.NewMobileBuildService(mobileBuildAPITestFlags(), &mockAPIMobileBuildProvider{}, store))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds/mbld_api_repair_signing/repair", project.ID), nil)
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}, {Key: "buildId", Value: "mbld_api_repair_signing"}}
	context.Set("user_id", userID)

	server.RepairProjectMobileBuild(context)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), "MOBILE_REPAIR_CREDENTIALS_REQUIRED")
	require.Contains(t, recorder.Body.String(), string(mobile.MobileCredentialAndroidSigning))
	require.Contains(t, recorder.Body.String(), `"repair_plan"`)
}

func TestSubmitProjectMobileBuildCreatesStoreUploadJob(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	project.MobileStoreReadinessStatus = "succeeded"
	require.NoError(t, gormDB.Save(&project).Error)
	storeRequiredAndroidMobileCredentials(t, gormDB, userID, project)
	buildStore := mobile.NewGormMobileBuildStore(gormDB)
	require.NoError(t, buildStore.Save(context.Background(), mobile.MobileBuildJob{
		ID:              "mbld_api_submit_ready",
		ProjectID:       project.ID,
		UserID:          userID,
		Platform:        mobile.MobilePlatformAndroid,
		Profile:         mobile.MobileBuildProfileProduction,
		ReleaseLevel:    mobile.ReleaseAndroidAAB,
		Status:          mobile.MobileBuildSucceeded,
		Provider:        "eas",
		ProviderBuildID: "eas-build-submit-ready",
		ArtifactURL:     "https://artifacts.example.com/app.aab",
	}))
	spec := mobile.FieldServiceContractorQuoteSpec()
	files, errs := mobile.GenerateExpoProject(spec, mobile.ExpoGeneratorOptions{})
	require.Empty(t, errs)
	for _, file := range files {
		createMobileBuildAPIFile(t, gormDB, project, file.Path, file.Content)
	}
	provider := &mockAPIMobileSubmissionProvider{
		result: mobile.MobileSubmissionProviderResult{
			ProviderSubmissionID: "eas-submit-ready",
			Status:               mobile.MobileSubmissionReadyForGoogleInternalTesting,
		},
	}
	flags := mobileBuildAPITestFlags()
	flags.MobileEASSubmitEnabled = true
	server.SetMobileBuildService(mobile.NewMobileBuildService(flags, &mockAPIMobileBuildProvider{}, buildStore))
	server.SetMobileSubmissionService(mobile.NewMobileSubmissionService(
		flags,
		provider,
		mobile.NewGormMobileSubmissionStore(gormDB),
		mobile.WithMobileSubmissionIDGenerator(func() string { return "msub_api_submit" }),
	))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds/mbld_api_submit_ready/submit", project.ID), strings.NewReader(`{}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}, {Key: "buildId", Value: "mbld_api_submit_ready"}}
	context.Set("user_id", userID)

	server.SubmitProjectMobileBuild(context)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Contains(t, recorder.Body.String(), "msub_api_submit")
	require.Equal(t, 1, provider.calls)
	require.Equal(t, "mbld_api_submit_ready", provider.last.BuildID)
	require.Equal(t, "eas-build-submit-ready", provider.last.ProviderBuildID)

	var updated models.Project
	require.NoError(t, gormDB.First(&updated, project.ID).Error)
	require.Equal(t, "submitted_to_store_pipeline", updated.MobileStoreReadinessStatus)
	require.Equal(t, "msub_api_submit", mobileMetadataStringForAPITest(updated.MobileMetadata, "last_mobile_submission_id"))
}

func TestSubmitProjectMobileBuildRequiresCompleteStoreCredentials(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	storeEASMobileCredential(t, gormDB, userID, project)
	buildStore := mobile.NewGormMobileBuildStore(gormDB)
	require.NoError(t, buildStore.Save(context.Background(), mobile.MobileBuildJob{
		ID:              "mbld_api_submit_missing_creds",
		ProjectID:       project.ID,
		UserID:          userID,
		Platform:        mobile.MobilePlatformAndroid,
		Profile:         mobile.MobileBuildProfileProduction,
		ReleaseLevel:    mobile.ReleaseAndroidAAB,
		Status:          mobile.MobileBuildSucceeded,
		ProviderBuildID: "eas-build-submit-missing-creds",
	}))
	flags := mobileBuildAPITestFlags()
	flags.MobileEASSubmitEnabled = true
	provider := &mockAPIMobileSubmissionProvider{}
	server.SetMobileBuildService(mobile.NewMobileBuildService(flags, &mockAPIMobileBuildProvider{}, buildStore))
	server.SetMobileSubmissionService(mobile.NewMobileSubmissionService(flags, provider, mobile.NewGormMobileSubmissionStore(gormDB)))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds/mbld_api_submit_missing_creds/submit", project.ID), strings.NewReader(`{}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}, {Key: "buildId", Value: "mbld_api_submit_missing_creds"}}
	context.Set("user_id", userID)

	server.SubmitProjectMobileBuild(context)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), "MOBILE_SUBMISSION_CREDENTIALS_REQUIRED")
	require.Contains(t, recorder.Body.String(), string(mobile.MobileCredentialGooglePlayService))
	require.Zero(t, provider.calls)
}

func TestSubmitProjectMobileBuildRequiresProPlan(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "builder")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	buildStore := mobile.NewGormMobileBuildStore(gormDB)
	require.NoError(t, buildStore.Save(context.Background(), mobile.MobileBuildJob{
		ID:              "mbld_api_submit_builder_blocked",
		ProjectID:       project.ID,
		UserID:          userID,
		Platform:        mobile.MobilePlatformAndroid,
		Profile:         mobile.MobileBuildProfileProduction,
		ReleaseLevel:    mobile.ReleaseAndroidAAB,
		Status:          mobile.MobileBuildSucceeded,
		ProviderBuildID: "eas-build-submit-builder-blocked",
		ArtifactURL:     "https://artifacts.example.com/app.aab",
	}))
	flags := mobileBuildAPITestFlags()
	flags.MobileEASSubmitEnabled = true
	provider := &mockAPIMobileSubmissionProvider{}
	server.SetMobileBuildService(mobile.NewMobileBuildService(flags, &mockAPIMobileBuildProvider{}, buildStore))
	server.SetMobileSubmissionService(mobile.NewMobileSubmissionService(flags, provider, mobile.NewGormMobileSubmissionStore(gormDB)))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds/mbld_api_submit_builder_blocked/submit", project.ID), strings.NewReader(`{}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}, {Key: "buildId", Value: "mbld_api_submit_builder_blocked"}}
	context.Set("user_id", userID)

	server.SubmitProjectMobileBuild(context)

	require.Equal(t, http.StatusPaymentRequired, recorder.Code)
	require.Contains(t, recorder.Body.String(), mobileSubmissionPlanRequiredCode)
	require.Contains(t, recorder.Body.String(), `"required_plan":"pro"`)
	require.Zero(t, provider.calls)
}

func TestSubmitProjectMobileBuildRejectsDuplicateActiveSubmission(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	project.MobileStoreReadinessStatus = "succeeded"
	require.NoError(t, gormDB.Save(&project).Error)
	storeRequiredAndroidMobileCredentials(t, gormDB, userID, project)
	buildStore := mobile.NewGormMobileBuildStore(gormDB)
	require.NoError(t, buildStore.Save(context.Background(), mobile.MobileBuildJob{
		ID:              "mbld_api_submit_duplicate",
		ProjectID:       project.ID,
		UserID:          userID,
		Platform:        mobile.MobilePlatformAndroid,
		Profile:         mobile.MobileBuildProfileProduction,
		ReleaseLevel:    mobile.ReleaseAndroidAAB,
		Status:          mobile.MobileBuildSucceeded,
		ProviderBuildID: "eas-build-submit-duplicate",
		ArtifactURL:     "https://artifacts.example.com/app.aab",
	}))
	submissionStore := mobile.NewGormMobileSubmissionStore(gormDB)
	require.NoError(t, submissionStore.Save(context.Background(), mobile.MobileSubmissionJob{
		ID:                   "msub_api_existing",
		ProjectID:            project.ID,
		UserID:               userID,
		BuildID:              "mbld_api_submit_duplicate",
		Platform:             mobile.MobilePlatformAndroid,
		Status:               mobile.MobileSubmissionReadyForGoogleInternalTesting,
		Provider:             "eas",
		ProviderSubmissionID: "eas-submit-existing",
		Track:                "internal",
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}))
	flags := mobileBuildAPITestFlags()
	flags.MobileEASSubmitEnabled = true
	provider := &mockAPIMobileSubmissionProvider{}
	server.SetMobileBuildService(mobile.NewMobileBuildService(flags, &mockAPIMobileBuildProvider{}, buildStore))
	server.SetMobileSubmissionService(mobile.NewMobileSubmissionService(flags, provider, submissionStore))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds/mbld_api_submit_duplicate/submit", project.ID), strings.NewReader(`{}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}, {Key: "buildId", Value: "mbld_api_submit_duplicate"}}
	context.Set("user_id", userID)

	server.SubmitProjectMobileBuild(context)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), "MOBILE_SUBMISSION_ALREADY_EXISTS")
	require.Contains(t, recorder.Body.String(), "msub_api_existing")
	require.Zero(t, provider.calls)
}

func TestRetryProjectMobileBuildRejectsActiveJob(t *testing.T) {
	server, userID, gormDB := newProjectAPITestServer(t, "pro")
	project := createMobileBuildAPIProject(t, gormDB, userID, []string{"android"})
	storeEASMobileCredential(t, gormDB, userID, project)
	store := mobile.NewGormMobileBuildStore(gormDB)
	require.NoError(t, store.Save(context.Background(), mobile.MobileBuildJob{
		ID:              "mbld_api_active_retry",
		ProjectID:       project.ID,
		UserID:          userID,
		Platform:        mobile.MobilePlatformAndroid,
		Profile:         mobile.MobileBuildProfilePreview,
		ReleaseLevel:    mobile.ReleaseInternalAndroidAPK,
		Status:          mobile.MobileBuildBuilding,
		ProviderBuildID: "eas-build-active",
	}))
	provider := &mockAPIMobileBuildProvider{}
	server.SetMobileBuildService(mobile.NewMobileBuildService(mobileBuildAPITestFlags(), provider, store))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/builds/mbld_api_active_retry/retry", project.ID), nil)
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}, {Key: "buildId", Value: "mbld_api_active_retry"}}
	context.Set("user_id", userID)

	server.RetryProjectMobileBuild(context)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "cannot be retried")
	require.Equal(t, 0, provider.calls)
}

func createMobileBuildAPIProject(t *testing.T, gormDB *gorm.DB, userID uint, platforms []string) models.Project {
	t.Helper()
	project := models.Project{
		Name:                      "Mobile Build API",
		Language:                  "typescript",
		OwnerID:                   userID,
		TargetPlatform:            string(mobile.TargetPlatformMobileExpo),
		MobilePlatforms:           platforms,
		MobileFramework:           string(mobile.MobileFrameworkExpoReactNative),
		MobileReleaseLevel:        string(mobile.ReleaseSourceOnly),
		GeneratedMobileClientPath: "mobile/",
		MobileBuildStatus:         "not_requested",
		MobileMetadata:            map[string]interface{}{},
	}
	require.NoError(t, gormDB.Create(&project).Error)
	return project
}

func createMobileBuildAPIFile(t *testing.T, gormDB *gorm.DB, project models.Project, path string, content string) {
	t.Helper()
	require.NoError(t, gormDB.Create(&models.File{
		ProjectID: project.ID,
		Path:      path,
		Name:      filepath.Base(path),
		Type:      "file",
		Content:   content,
		Size:      int64(len(content)),
	}).Error)
}

func storeEASMobileCredential(t *testing.T, gormDB *gorm.DB, userID uint, project models.Project) {
	t.Helper()
	manager, err := secretstore.NewSecretsManager("mobile-build-api-test-master-key")
	require.NoError(t, err)
	vault := mobile.NewMobileCredentialVault(gormDB, manager)
	_, err = vault.Store(context.Background(), userID, project, mobile.MobileCredentialInput{
		Type:   mobile.MobileCredentialEASToken,
		Values: map[string]string{"token": "eas-test-token"},
	})
	require.NoError(t, err)
}

func storeRequiredAndroidMobileCredentials(t *testing.T, gormDB *gorm.DB, userID uint, project models.Project) {
	t.Helper()
	manager, err := secretstore.NewSecretsManager("mobile-build-api-test-master-key")
	require.NoError(t, err)
	vault := mobile.NewMobileCredentialVault(gormDB, manager)
	_, err = vault.Store(context.Background(), userID, project, mobile.MobileCredentialInput{
		Type:   mobile.MobileCredentialEASToken,
		Values: map[string]string{"token": "eas-test-token"},
	})
	require.NoError(t, err)
	_, err = vault.Store(context.Background(), userID, project, mobile.MobileCredentialInput{
		Type: mobile.MobileCredentialGooglePlayService,
		Values: map[string]string{
			"service_account_json": `{"client_email":"play@example.iam.gserviceaccount.com","private_key":"-----BEGIN PRIVATE KEY-----\nplay\n-----END PRIVATE KEY-----"}`,
		},
	})
	require.NoError(t, err)
}

func mobileBuildAPITestFlags() mobile.FeatureFlags {
	return mobile.FeatureFlags{
		MobileBuilderEnabled:       true,
		MobileExpoEnabled:          true,
		MobileEASBuildEnabled:      true,
		MobileAndroidBuildsEnabled: true,
		MobileIOSBuildsEnabled:     true,
	}
}

func mobileMetadataStringForAPITest(metadata map[string]interface{}, key string) string {
	if value, ok := metadata[key].(string); ok {
		return value
	}
	return ""
}

type mockAPIMobileBuildProvider struct {
	result              mobile.MobileBuildProviderResult
	err                 error
	refreshResult       mobile.MobileBuildProviderResult
	refreshErr          error
	cancelResult        mobile.MobileBuildProviderResult
	cancelErr           error
	calls               int
	refreshCalls        int
	cancelCalls         int
	lastReq             mobile.MobileBuildRequest
	lastRefreshJob      mobile.MobileBuildJob
	lastCancelJob       mobile.MobileBuildJob
	sourcePackageExists bool
}

func (p *mockAPIMobileBuildProvider) Name() string {
	return "mock-eas"
}

func (p *mockAPIMobileBuildProvider) CreateBuild(_ context.Context, req mobile.MobileBuildRequest) (mobile.MobileBuildProviderResult, error) {
	p.calls++
	p.lastReq = req
	if _, err := os.Stat(filepath.Join(req.SourcePath, "package.json")); err == nil {
		p.sourcePackageExists = true
	}
	return p.result, p.err
}

func (p *mockAPIMobileBuildProvider) RefreshBuild(_ context.Context, job mobile.MobileBuildJob) (mobile.MobileBuildProviderResult, error) {
	p.refreshCalls++
	p.lastRefreshJob = job
	if p.refreshErr != nil {
		return mobile.MobileBuildProviderResult{}, p.refreshErr
	}
	return p.refreshResult, nil
}

func (p *mockAPIMobileBuildProvider) CancelBuild(_ context.Context, job mobile.MobileBuildJob) (mobile.MobileBuildProviderResult, error) {
	p.cancelCalls++
	p.lastCancelJob = job
	if p.cancelErr != nil {
		return mobile.MobileBuildProviderResult{}, p.cancelErr
	}
	return p.cancelResult, nil
}

type mockAPIMobileSubmissionProvider struct {
	result mobile.MobileSubmissionProviderResult
	err    error
	calls  int
	last   mobile.MobileSubmissionRequest
}

func (p *mockAPIMobileSubmissionProvider) Name() string {
	return "mock-eas-submit"
}

func (p *mockAPIMobileSubmissionProvider) SubmitBuild(_ context.Context, req mobile.MobileSubmissionRequest) (mobile.MobileSubmissionProviderResult, error) {
	p.calls++
	p.last = req
	if p.err != nil {
		return mobile.MobileSubmissionProviderResult{}, p.err
	}
	return p.result, nil
}
