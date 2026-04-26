package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"apex-build/internal/db"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUpdateUserProfileAllowsFreeUserToSelectOllama(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gormDB, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := gormDB.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	user := models.User{
		Username:         "api-ollama-free-user",
		Email:            "api-ollama-free@example.com",
		PasswordHash:     "hash",
		IsActive:         true,
		SubscriptionType: "free",
	}
	if err := gormDB.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	server := &Server{db: &db.Database{DB: gormDB}}
	body, _ := json.Marshal(map[string]any{"preferred_ai": "ollama"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", user.ID)
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/user/profile", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	server.UpdateUserProfile(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated models.User
	if err := gormDB.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if updated.PreferredAI != "ollama" {
		t.Fatalf("preferred_ai = %q, want ollama", updated.PreferredAI)
	}
}
