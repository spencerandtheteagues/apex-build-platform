package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"apex-build/internal/mobile"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDownloadProjectPreparesMobileExpoFilesForOwnerZip(t *testing.T) {
	handler, userID, db := newProjectHandlerTestFixture(t, "pro")

	project := models.Project{
		Name:           "Legacy Mobile ZIP",
		Language:       "typescript",
		OwnerID:        userID,
		TargetPlatform: string(mobile.TargetPlatformMobileExpo),
	}
	require.NoError(t, db.Create(&project).Error)

	spec := mobile.FieldServiceContractorQuoteSpec()
	spec.App.Slug = "legacy-mobile-zip-spec"
	specJSON, err := json.Marshal(spec)
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.CompletedBuild{
		BuildID:        "legacy-mobile-zip-build",
		UserID:         userID,
		ProjectID:      &project.ID,
		Status:         "completed",
		TargetPlatform: string(mobile.TargetPlatformMobileExpo),
		MobileSpecJSON: string(specJSON),
	}).Error)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/projects/%d/download", project.ID), nil)
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	context.Set("user_id", userID)

	handler.DownloadProject(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, zipBodyHasPath(t, recorder.Body.Bytes(), "mobile/package.json"))
	require.True(t, zipBodyHasPath(t, recorder.Body.Bytes(), "mobile/store/store-readiness.json"))
	require.Contains(t, zipBodyReadPath(t, recorder.Body.Bytes(), "mobile/store/store-readiness.json"), "draft_ready_needs_manual_store_assets")
}

func zipBodyHasPath(t *testing.T, body []byte, path string) bool {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)
	for _, file := range reader.File {
		if strings.TrimPrefix(file.Name, "/") == path {
			return true
		}
	}
	return false
}

func zipBodyReadPath(t *testing.T, body []byte, path string) string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)
	for _, file := range reader.File {
		if strings.TrimPrefix(file.Name, "/") != path {
			continue
		}
		rc, err := file.Open()
		require.NoError(t, err)
		defer rc.Close()
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(rc)
		require.NoError(t, err)
		return buf.String()
	}
	t.Fatalf("zip path %q not found", path)
	return ""
}
