package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
)

func TestUsageLimitsPricingUsesLaunchPlanTruth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openTestDB(t)
	user := models.User{
		Username:           "pro-user",
		Email:              "pro@example.com",
		PasswordHash:       "hash",
		IsActive:           true,
		SubscriptionType:   "pro",
		SubscriptionStatus: "active",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/usage/limits", nil)
	c.Set("user_id", user.ID)

	NewUsageHandlers(db, nil).GetLimits(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			CurrentPlan string `json:"current_plan"`
			AllPlans    map[string]struct {
				StorageBytes int64 `json:"storage_bytes"`
			} `json:"all_plans"`
			Pricing map[string]struct {
				PriceMonthly int64 `json:"price_monthly"`
				PriceYearly  int64 `json:"price_yearly"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !body.Success {
		t.Fatalf("expected success response, got %s", w.Body.String())
	}
	if body.Data.CurrentPlan != "pro" {
		t.Fatalf("current_plan = %q, want pro", body.Data.CurrentPlan)
	}
	if _, ok := body.Data.AllPlans["builder"]; !ok {
		t.Fatalf("expected builder limits in all_plans, got %+v", body.Data.AllPlans)
	}
	if _, ok := body.Data.AllPlans["owner"]; !ok {
		t.Fatalf("expected owner limits in all_plans, got %+v", body.Data.AllPlans)
	}
	if body.Data.Pricing["pro"].PriceMonthly != 5900 {
		t.Fatalf("pro monthly price = %d, want 5900", body.Data.Pricing["pro"].PriceMonthly)
	}
	if body.Data.Pricing["pro"].PriceYearly != 56640 {
		t.Fatalf("pro yearly price = %d, want 56640", body.Data.Pricing["pro"].PriceYearly)
	}
}
