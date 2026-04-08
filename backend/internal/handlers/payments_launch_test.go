package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
