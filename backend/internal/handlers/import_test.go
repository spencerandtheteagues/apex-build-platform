package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newImportHandlerTestFixture(t *testing.T, subscriptionType string) (*ImportHandler, uint, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Project{}, &models.File{}))

	user := models.User{
		Username:         strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_"),
		Email:            strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_") + "@example.com",
		PasswordHash:     "hashed-password",
		SubscriptionType: subscriptionType,
	}
	require.NoError(t, db.Create(&user).Error)

	return NewImportHandler(db, nil, nil), user.ID, db
}

func TestImportGitHubRejectsPublicImportOnFreePlan(t *testing.T) {
	handler, userID, db := newImportHandlerTestFixture(t, "free")

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/projects/import/github", strings.NewReader(`{"url":"https://github.com/apex-build/example","is_public":true}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("user_id", userID)
	context.Set("subscription_type", "free")

	handler.ImportGitHub(context)

	require.Equal(t, http.StatusPaymentRequired, recorder.Code)
	require.Contains(t, recorder.Body.String(), backendSubscriptionRequiredCode)

	var projectCount int64
	require.NoError(t, db.Model(&models.Project{}).Count(&projectCount).Error)
	require.Zero(t, projectCount)
}
