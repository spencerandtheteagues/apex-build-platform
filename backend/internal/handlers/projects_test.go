package handlers

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newProjectHandlerTestFixture(t *testing.T, subscriptionType string) (*Handler, uint, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Project{}))

	user := models.User{
		Username:         strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_"),
		Email:            strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_") + "@example.com",
		PasswordHash:     "hashed-password",
		SubscriptionType: subscriptionType,
	}
	require.NoError(t, db.Create(&user).Error)

	return NewHandler(db, nil, nil, nil), user.ID, db
}

func TestCreateProjectRejectsPublicProjectOnFreePlan(t *testing.T) {
	handler, userID, _ := newProjectHandlerTestFixture(t, "free")

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/projects", strings.NewReader(`{"name":"Public Demo","language":"typescript","is_public":true}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("user_id", userID)
	context.Set("subscription_type", "free")

	handler.CreateProject(context)

	require.Equal(t, http.StatusPaymentRequired, recorder.Code)
	require.Contains(t, recorder.Body.String(), backendSubscriptionRequiredCode)
}

func TestUpdateProjectRejectsPublishingOnFreePlan(t *testing.T) {
	handler, userID, db := newProjectHandlerTestFixture(t, "free")

	project := models.Project{
		Name:      "Private Demo",
		Language:  "typescript",
		OwnerID:   userID,
		IsPublic:  false,
		Framework: "React",
	}
	require.NoError(t, db.Create(&project).Error)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	projectID := strconv.FormatUint(uint64(project.ID), 10)
	context.Request = httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, strings.NewReader(`{"is_public":true}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: projectID}}
	context.Set("user_id", userID)
	context.Set("subscription_type", "free")

	handler.UpdateProject(context)

	require.Equal(t, http.StatusPaymentRequired, recorder.Code)
	require.Contains(t, recorder.Body.String(), backendSubscriptionRequiredCode)
}
