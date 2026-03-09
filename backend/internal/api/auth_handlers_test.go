package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"apex-build/internal/auth"
	"apex-build/internal/db"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newLogoutTestServer(t *testing.T) (*Server, *auth.AuthService, *models.User) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gormDB.AutoMigrate(&models.User{}, &models.RefreshToken{}))

	authService := auth.NewAuthService("test-jwt-secret-with-sufficient-length-1234567890")
	authService.SetDB(gormDB)

	passwordHash, err := authService.HashPassword("Passw0rd!Passw0rd!")
	require.NoError(t, err)

	user := &models.User{
		Username:         "logout-user",
		Email:            "logout@example.com",
		PasswordHash:     passwordHash,
		IsActive:         true,
		SubscriptionType: "free",
	}
	require.NoError(t, gormDB.Create(user).Error)

	server := NewServer(&db.Database{DB: gormDB}, authService, nil, nil)
	return server, authService, user
}

func TestLogoutRevokesRefreshToken(t *testing.T) {
	server, authService, user := newLogoutTestServer(t)

	tokens, err := authService.GenerateTokens(user)
	require.NoError(t, err)

	_, err = authService.ValidateRefreshToken(tokens.RefreshToken)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout",
		bytes.NewBufferString(`{"refresh_token":"`+tokens.RefreshToken+`"}`))
	context.Request.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	context.Request.Header.Set("Content-Type", "application/json")

	server.Logout(context)

	require.Equal(t, http.StatusOK, recorder.Code)

	_, err = authService.ValidateToken(tokens.AccessToken)
	require.ErrorIs(t, err, auth.ErrTokenBlacklisted)

	_, err = authService.ValidateRefreshToken(tokens.RefreshToken)
	require.ErrorIs(t, err, auth.ErrRefreshTokenRevoked)
}
