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

func TestLoginWithSeededAdminCredentials(t *testing.T) {
	t.Setenv("GO_ENV", "production")
	t.Setenv("ADMIN_SEED_PASSWORD", "TheStarsh1pKEY!")
	t.Setenv("SPENCER_SEED_PASSWORD", "")
	t.Setenv("ALLOW_DEFAULT_SEED_PASSWORDS", "false")

	gin.SetMode(gin.TestMode)

	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gormDB.AutoMigrate(&models.User{}, &models.RefreshToken{}))

	database := &db.Database{DB: gormDB}
	require.NoError(t, database.RunSeeds())

	authService := auth.NewAuthService("test-jwt-secret-with-sufficient-length-1234567890")
	authService.SetDB(gormDB)

	server := NewServer(database, authService, nil, nil)

	body := bytes.NewBufferString(`{"username":"admin","password":"TheStarsh1pKEY!"}`)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	context.Request.Header.Set("Content-Type", "application/json")

	server.Login(context)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		User struct {
			Username     string `json:"username"`
			Email        string `json:"email"`
			IsAdmin      bool   `json:"is_admin"`
			IsSuperAdmin bool   `json:"is_super_admin"`
		} `json:"user"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "admin", response.User.Username)
	require.Equal(t, "admin@apex.build", response.User.Email)
	require.True(t, response.User.IsAdmin)
	require.True(t, response.User.IsSuperAdmin)
	requireCookiePresent(t, recorder, auth.AccessTokenCookieName)
	requireCookiePresent(t, recorder, auth.RefreshTokenCookieName)
}

func TestLoginWithDefaultDevelopmentAdminCredentials(t *testing.T) {
	t.Setenv("GO_ENV", "development")
	t.Setenv("ADMIN_SEED_PASSWORD", "")
	t.Setenv("SPENCER_SEED_PASSWORD", "")
	t.Setenv("ALLOW_DEFAULT_SEED_PASSWORDS", "false")

	gin.SetMode(gin.TestMode)

	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gormDB.AutoMigrate(&models.User{}, &models.RefreshToken{}))

	database := &db.Database{DB: gormDB}
	require.NoError(t, database.RunSeeds())

	authService := auth.NewAuthService("test-jwt-secret-with-sufficient-length-1234567890")
	authService.SetDB(gormDB)

	server := NewServer(database, authService, nil, nil)

	body := bytes.NewBufferString(`{"username":"admin","password":"TheStarsh1pKEY!"}`)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	context.Request.Header.Set("Content-Type", "application/json")

	server.Login(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	requireCookiePresent(t, recorder, auth.AccessTokenCookieName)
	requireCookiePresent(t, recorder, auth.RefreshTokenCookieName)
}

func requireCookiePresent(t *testing.T, recorder *httptest.ResponseRecorder, name string) {
	t.Helper()

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			require.NotEmpty(t, cookie.Value)
			return
		}
	}

	t.Fatalf("expected cookie %s", name)
}
