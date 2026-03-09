package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"apex-build/internal/preview"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newPreviewHandlerTestFixture(t *testing.T, requireSandbox bool) (*PreviewHandler, uint) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Project{}, &models.File{}))

	user := models.User{
		Username:     strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_"),
		Email:        strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_") + "@example.com",
		PasswordHash: "hashed-password",
	}
	require.NoError(t, db.Create(&user).Error)

	project := models.Project{
		Name:     "Preview Fixture",
		Language: "typescript",
		OwnerID:  user.ID,
	}
	require.NoError(t, db.Create(&project).Error)

	handler := &PreviewHandler{
		db:             db,
		server:         preview.NewPreviewServer(db),
		serverRunner:   preview.NewServerRunner(db),
		requireSandbox: requireSandbox,
	}

	return handler, project.ID
}

func TestPreviewHandlerRejectsExplicitSandboxWithoutDocker(t *testing.T) {
	useSandbox, err := (&PreviewHandler{}).resolveRequestedPreviewSandbox(true)
	require.Error(t, err)
	require.False(t, useSandbox)
}

func TestPreviewHandlerStartPreviewFailsClosedWhenSandboxRequired(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, true)

	body, err := json.Marshal(map[string]any{
		"project_id": projectID,
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/preview/start", bytes.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("user_id", uint(1))

	handler.StartPreview(context)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "secure preview mode requires Docker container previews")
}

func TestPreviewHandlerStartServerBlockedInSecureMode(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, true)

	body, err := json.Marshal(map[string]any{
		"project_id": projectID,
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/preview/server/start", bytes.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("user_id", uint(1))

	handler.StartServer(context)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), "secure sandbox preview is enforced")
}

func TestPreviewHandlerGetServerStatusReportsUnavailableInSecureMode(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, true)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "projectId", Value: "1"}}
	context.Request = httptest.NewRequest(http.MethodGet, "/preview/server/status/1", nil)
	context.Set("user_id", uint(1))

	context.Params = gin.Params{{Key: "projectId", Value: strconv.FormatUint(uint64(projectID), 10)}}
	context.Request = httptest.NewRequest(http.MethodGet, "/preview/server/status/"+strconv.FormatUint(uint64(projectID), 10), nil)

	handler.GetServerStatus(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"available":false`)
	require.Contains(t, recorder.Body.String(), "secure sandbox preview is enforced")
}

func TestPreviewHandlerGetDockerStatusIncludesSecureModeFlags(t *testing.T) {
	handler, _ := newPreviewHandlerTestFixture(t, true)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/preview/docker/status", nil)

	handler.GetDockerStatus(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"sandbox_required":true`)
	require.Contains(t, recorder.Body.String(), `"backend_preview_available":false`)
}
