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
	calls               int
	refreshCalls        int
	lastReq             mobile.MobileBuildRequest
	lastRefreshJob      mobile.MobileBuildJob
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
