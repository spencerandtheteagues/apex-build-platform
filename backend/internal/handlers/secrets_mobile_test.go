package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"apex-build/internal/mobile"
	secretstore "apex-build/internal/secrets"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateProjectMobileCredentialStoresEncryptedMetadataOnly(t *testing.T) {
	handler, userID, project, gormDB := newMobileSecretHandlerTest(t)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/credentials", project.ID), strings.NewReader(`{
		"type":"eas_token",
		"values":{"token":"eas-handler-secret"}
	}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	context.Set("user_id", userID)

	handler.CreateProjectMobileCredential(context)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "eas-handler-secret")
	var response struct {
		Credentials mobile.MobileCredentialStatus `json:"credentials"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "partial", response.Credentials.Status)
	require.Len(t, response.Credentials.Metadata, 1)

	var stored secretstore.Secret
	require.NoError(t, gormDB.Where("user_id = ? AND project_id = ? AND name = ?", userID, project.ID, "mobile:eas_token").First(&stored).Error)
	require.NotContains(t, stored.EncryptedValue, "eas-handler-secret")
}

func TestListProjectMobileCredentialsReturnsRequiredMissingTypes(t *testing.T) {
	handler, userID, project, _ := newMobileSecretHandlerTest(t)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%d/mobile/credentials", project.ID), nil)
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	context.Set("user_id", userID)

	handler.ListProjectMobileCredentials(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Credentials mobile.MobileCredentialStatus `json:"credentials"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "missing", response.Credentials.Status)
	require.Contains(t, response.Credentials.Missing, mobile.MobileCredentialEASToken)
	require.Contains(t, response.Credentials.Missing, mobile.MobileCredentialAppleAppStoreConnect)
	require.Contains(t, response.Credentials.Missing, mobile.MobileCredentialGooglePlayService)
}

func TestCreateProjectMobileCredentialRejectsInvalidPayloadWithoutPersisting(t *testing.T) {
	handler, userID, project, gormDB := newMobileSecretHandlerTest(t)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%d/mobile/credentials", project.ID), strings.NewReader(`{
		"type":"google_play_service_account",
		"values":{"service_account_json":"{\"client_email\":\"missing-private@example.com\"}"}
	}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	context.Set("user_id", userID)

	handler.CreateProjectMobileCredential(context)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var count int64
	require.NoError(t, gormDB.Model(&secretstore.Secret{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestDeleteProjectMobileCredentialRemovesStoredSecret(t *testing.T) {
	handler, userID, project, gormDB := newMobileSecretHandlerTest(t)
	vault := mobile.NewMobileCredentialVault(gormDB, handler.manager)
	_, err := vault.Store(contextWithTestName(t), userID, project, mobile.MobileCredentialInput{
		Type:   mobile.MobileCredentialEASToken,
		Values: map[string]string{"token": "eas-delete-secret"},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/projects/%d/mobile/credentials/eas_token", project.ID), nil)
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}, {Key: "type", Value: "eas_token"}}
	context.Set("user_id", userID)

	handler.DeleteProjectMobileCredential(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var count int64
	require.NoError(t, gormDB.Model(&secretstore.Secret{}).Where("name = ?", "mobile:eas_token").Count(&count).Error)
	require.Zero(t, count)
}

func newMobileSecretHandlerTest(t *testing.T) (*SecretsHandler, uint, models.Project, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	gormDB, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gormDB.AutoMigrate(&models.User{}, &models.Project{}, &secretstore.Secret{}, &secretstore.SecretAuditLog{}))
	manager, err := secretstore.NewSecretsManager("mobile-handler-test-master-key")
	require.NoError(t, err)

	user := models.User{
		Username:     strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_"),
		Email:        strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_") + "@example.com",
		PasswordHash: "hashed",
	}
	require.NoError(t, gormDB.Create(&user).Error)
	project := models.Project{
		Name:            "Mobile Credential Handler",
		Language:        "typescript",
		OwnerID:         user.ID,
		TargetPlatform:  string(mobile.TargetPlatformMobileExpo),
		MobilePlatforms: []string{"android", "ios"},
		MobileFramework: string(mobile.MobileFrameworkExpoReactNative),
		MobileMetadata:  map[string]interface{}{},
	}
	require.NoError(t, gormDB.Create(&project).Error)
	return NewSecretsHandler(gormDB, manager), user.ID, project, gormDB
}

func contextWithTestName(t *testing.T) context.Context {
	t.Helper()
	return httptest.NewRequest(http.MethodPost, "/test/"+strings.ReplaceAll(t.Name(), "/", "_"), nil).Context()
}
