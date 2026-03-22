package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"apex-build/internal/ai"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newBYOKHandlerTestFixture(t *testing.T, subscriptionType string) (*BYOKHandlers, uint) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.UserAPIKey{}, &models.AIUsageLog{}))

	user := models.User{
		Username:         strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_"),
		Email:            strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_") + "@example.com",
		PasswordHash:     "hashed-password",
		SubscriptionType: subscriptionType,
	}
	require.NoError(t, db.Create(&user).Error)

	return NewBYOKHandlers(ai.NewBYOKManager(db, nil, nil)), user.ID
}

func TestBYOKHandlersRequirePaidPlan(t *testing.T) {
	handler, userID := newBYOKHandlerTestFixture(t, "free")

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/byok/models", nil)
	context.Set("user_id", userID)

	handler.GetModels(context)

	require.Equal(t, http.StatusPaymentRequired, recorder.Code)
	require.Contains(t, recorder.Body.String(), byokSubscriptionRequiredCode)
}

func TestBYOKHandlersAllowPaidPlan(t *testing.T) {
	handler, userID := newBYOKHandlerTestFixture(t, "builder")

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/byok/models", nil)
	context.Set("user_id", userID)

	handler.GetModels(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
}
