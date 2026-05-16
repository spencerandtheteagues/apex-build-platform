package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestExecutionRequiresVerifiedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}))

	user := models.User{
		Username:     "unverified-exec",
		Email:        "unverified-exec@example.com",
		PasswordHash: "hash",
		IsActive:     true,
	}
	require.NoError(t, db.Create(&user).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	handler := &ExecutionHandler{DB: db}

	require.False(t, handler.requireVerifiedExecutionUser(c, user.ID))
	require.Equal(t, 403, recorder.Code)
	require.Contains(t, recorder.Body.String(), "email_not_verified")
}
