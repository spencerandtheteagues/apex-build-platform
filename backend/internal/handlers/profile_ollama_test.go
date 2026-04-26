package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
)

func TestUpdateProfileAllowsFreeUserToSelectOllama(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openTestDB(t)
	user := models.User{
		Username:         "ollama-free-user",
		Email:            "ollama-free@example.com",
		PasswordHash:     "hash",
		IsActive:         true,
		SubscriptionType: "free",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	handler := &Handler{DB: db}
	body, _ := json.Marshal(map[string]any{"preferred_ai": "ollama"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", user.ID)
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/profile", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateProfile(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated models.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if updated.PreferredAI != "ollama" {
		t.Fatalf("preferred_ai = %q, want ollama", updated.PreferredAI)
	}
}
