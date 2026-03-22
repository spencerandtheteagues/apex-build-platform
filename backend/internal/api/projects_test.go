package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"apex-build/internal/db"
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
	require.NoError(t, gormDB.AutoMigrate(&models.User{}, &models.Project{}))

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
