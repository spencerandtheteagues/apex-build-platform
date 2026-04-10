package api

import (
	"bytes"
	"encoding/json"
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

	gormDB, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
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
		IsVerified:       true,
		SubscriptionType: "free",
	}
	require.NoError(t, gormDB.Create(user).Error)

	server := NewServer(&db.Database{DB: gormDB}, authService, nil, nil)
	return server, authService, user
}

func TestLoginSetsHttpOnlyAuthCookies(t *testing.T) {
	server, _, _ := newLogoutTestServer(t)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		bytes.NewBufferString(`{"username":"logout-user","password":"Passw0rd!Passw0rd!"}`))
	context.Request.Header.Set("Content-Type", "application/json")

	server.Login(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	requireAuthCookiePresent(t, recorder, auth.AccessTokenCookieName)
	requireAuthCookiePresent(t, recorder, auth.RefreshTokenCookieName)

	var response map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	_, hasAccessToken := response["access_token"]
	_, hasRefreshToken := response["refresh_token"]
	require.False(t, hasAccessToken)
	require.False(t, hasRefreshToken)
	require.NotEmpty(t, response["access_token_expires_at"])
	require.NotEmpty(t, response["refresh_token_expires_at"])
	require.Equal(t, "cookie", response["session_strategy"])
	require.Equal(t, "Bearer", response["token_type"])
}

func TestRefreshTokenAcceptsRefreshCookie(t *testing.T) {
	server, authService, user := newLogoutTestServer(t)

	tokens, err := authService.GenerateTokens(user)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString(`{}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.AddCookie(&http.Cookie{Name: auth.RefreshTokenCookieName, Value: tokens.RefreshToken})

	server.RefreshToken(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	requireAuthCookiePresent(t, recorder, auth.AccessTokenCookieName)
	requireAuthCookiePresent(t, recorder, auth.RefreshTokenCookieName)

	var response map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	_, hasAccessToken := response["access_token"]
	_, hasRefreshToken := response["refresh_token"]
	require.False(t, hasAccessToken)
	require.False(t, hasRefreshToken)
	require.NotEmpty(t, response["access_token_expires_at"])
	require.NotEmpty(t, response["refresh_token_expires_at"])
	require.Equal(t, "cookie", response["session_strategy"])
	require.Equal(t, "Bearer", response["token_type"])
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
	requireCookieCleared(t, recorder, auth.AccessTokenCookieName)
	requireCookieCleared(t, recorder, auth.RefreshTokenCookieName)

	_, err = authService.ValidateToken(tokens.AccessToken)
	require.ErrorIs(t, err, auth.ErrTokenBlacklisted)

	_, err = authService.ValidateRefreshToken(tokens.RefreshToken)
	require.ErrorIs(t, err, auth.ErrRefreshTokenRevoked)
}

func requireCookieCleared(t *testing.T, recorder *httptest.ResponseRecorder, name string) {
	t.Helper()

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			require.Empty(t, cookie.Value)
			require.LessOrEqual(t, cookie.MaxAge, 0)
			return
		}
	}

	t.Fatalf("expected cleared cookie %s", name)
}

func requireAuthCookiePresent(t *testing.T, recorder *httptest.ResponseRecorder, name string) {
	t.Helper()

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			require.NotEmpty(t, cookie.Value)
			return
		}
	}

	t.Fatalf("expected cookie %s", name)
}
