package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
)

func TestSanitizeBillingPortalReturnURLNormalizesRelativePath(t *testing.T) {
	t.Setenv("APP_URL", "https://apex-build.dev")

	got, err := sanitizeBillingPortalReturnURL("/billing?tab=manage#plans")
	if err != nil {
		t.Fatalf("expected relative return url to be accepted: %v", err)
	}
	if got != "https://apex-build.dev/billing?tab=manage#plans" {
		t.Fatalf("got %q, want normalized apex return url", got)
	}
}

func TestSanitizeBillingPortalReturnURLRejectsExternalOriginInProduction(t *testing.T) {
	t.Setenv("APP_URL", "https://apex-build.dev")
	t.Setenv("GO_ENV", "production")

	if _, err := sanitizeBillingPortalReturnURL("https://evil.example/phish"); err == nil {
		t.Fatal("expected external origin to be rejected")
	}
}

func TestCreateBillingPortalSessionRejectsInvalidReturnURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/billing/portal", strings.NewReader(`{"return_url":"https://evil.example/phish"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", uint(42))

	h := &PaymentHandlers{}
	h.CreateBillingPortalSession(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if !strings.Contains(w.Body.String(), `"code":"INVALID_RETURN_URL"`) {
		t.Fatalf("expected INVALID_RETURN_URL response, got %s", w.Body.String())
	}
}

func TestCreateCheckoutSessionRejectsPlaceholderPlanPriceID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", strings.NewReader(`{"price_id":"price_builder_monthly"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", uint(42))

	h := &PaymentHandlers{}
	h.CreateCheckoutSession(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(w.Body.String(), `"code":"PLAN_NOT_CONFIGURED"`) {
		t.Fatalf("expected PLAN_NOT_CONFIGURED response, got %s", w.Body.String())
	}
}

func TestCreateCheckoutSessionRejectsUnknownPlanPriceID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", strings.NewReader(`{"price_id":"price_not_real"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", uint(42))

	h := &PaymentHandlers{}
	h.CreateCheckoutSession(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if !strings.Contains(w.Body.String(), `"code":"INVALID_PRICE_ID"`) {
		t.Fatalf("expected INVALID_PRICE_ID response, got %s", w.Body.String())
	}
}

func TestCreateCheckoutSessionRejectsInvalidSuccessURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("STRIPE_PRICE_PRO_MONTHLY", "price_pro_monthly_live")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", strings.NewReader(`{"price_id":"price_pro_monthly_live","success_url":"https://evil.example/phish"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", uint(42))

	h := &PaymentHandlers{}
	h.CreateCheckoutSession(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if !strings.Contains(w.Body.String(), `"code":"INVALID_SUCCESS_URL"`) {
		t.Fatalf("expected INVALID_SUCCESS_URL response, got %s", w.Body.String())
	}
}

func TestCreateCheckoutSessionRejectsInvalidCancelURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("STRIPE_PRICE_PRO_MONTHLY", "price_pro_monthly_live")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/billing/checkout", strings.NewReader(`{"price_id":"price_pro_monthly_live","cancel_url":"https://evil.example/phish"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", uint(42))

	h := &PaymentHandlers{}
	h.CreateCheckoutSession(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if !strings.Contains(w.Body.String(), `"code":"INVALID_CANCEL_URL"`) {
		t.Fatalf("expected INVALID_CANCEL_URL response, got %s", w.Body.String())
	}
}

func TestGetSubscriptionSupportsOwnerPlan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openTestDB(t)
	user := models.User{
		Username:            "owner-user",
		Email:               "owner@example.com",
		PasswordHash:        "hash",
		IsActive:            true,
		SubscriptionType:    "owner",
		SubscriptionStatus:  "active",
		HasUnlimitedCredits: true,
		BypassBilling:       true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to seed owner user: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/billing/subscription", nil)
	c.Set("user_id", user.ID)

	h := NewPaymentHandlers(db, "")
	h.GetSubscription(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			PlanType string `json:"plan_type"`
			PlanName string `json:"plan_name"`
			Status   string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !body.Success {
		t.Fatalf("expected success response, got %s", w.Body.String())
	}
	if body.Data.PlanType != "owner" {
		t.Fatalf("plan_type = %q, want %q", body.Data.PlanType, "owner")
	}
	if body.Data.PlanName != "Owner" {
		t.Fatalf("plan_name = %q, want %q", body.Data.PlanName, "Owner")
	}
	if body.Data.Status != "active" {
		t.Fatalf("status = %q, want %q", body.Data.Status, "active")
	}
}
