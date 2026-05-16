package payments

import "testing"

func TestEvaluateBillingLaunchConfigReadyWhenAllSelfServePriceIDsConfigured(t *testing.T) {
	t.Setenv("STRIPE_PRICE_BUILDER_MONTHLY", "price_builder_monthly_live")
	t.Setenv("STRIPE_PRICE_BUILDER_ANNUAL", "price_builder_annual_live")
	t.Setenv("STRIPE_PRICE_PRO_MONTHLY", "price_pro_monthly_live")
	t.Setenv("STRIPE_PRICE_PRO_ANNUAL", "price_pro_annual_live")
	t.Setenv("STRIPE_PRICE_TEAM_MONTHLY", "price_team_monthly_live")
	t.Setenv("STRIPE_PRICE_TEAM_ANNUAL", "price_team_annual_live")

	status := EvaluateBillingLaunchConfig("sk_test_valid_but_not_placeholder_123", "whsec_valid_123")
	if !status.Ready {
		t.Fatalf("expected billing launch config to be ready, got %+v", status)
	}
	if !status.StripeConfigured || !status.WebhookConfigured || !status.RequiredPriceIDsConfigured {
		t.Fatalf("expected all billing readiness flags true, got %+v", status)
	}
	if len(status.Issues) != 0 {
		t.Fatalf("expected no readiness issues, got %+v", status.Issues)
	}
}

func TestPricingInfoExposesSelfServeReadiness(t *testing.T) {
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_valid_but_not_placeholder_123")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_valid_123")
	t.Setenv("STRIPE_PRICE_BUILDER_MONTHLY", "price_builder_monthly_live")
	t.Setenv("STRIPE_PRICE_BUILDER_ANNUAL", "price_builder_annual_live")
	t.Setenv("STRIPE_PRICE_PRO_MONTHLY", "price_pro_monthly_live")
	t.Setenv("STRIPE_PRICE_PRO_ANNUAL", "price_pro_annual_live")
	t.Setenv("STRIPE_PRICE_TEAM_MONTHLY", "price_team_monthly_live")
	t.Setenv("STRIPE_PRICE_TEAM_ANNUAL", "price_team_annual_live")

	pricing := GetPricingInfo()
	if !pricing.SelfServeReady || !pricing.StripeConfigured || !pricing.WebhookConfigured || !pricing.RequiredPriceIDsConfigured {
		t.Fatalf("expected pricing readiness to be fully true, got %+v", pricing)
	}
}

func TestEvaluateBillingLaunchConfigReportsMissingWebhookAndPriceIDs(t *testing.T) {
	t.Setenv("STRIPE_PRICE_BUILDER_MONTHLY", "price_builder_monthly_live")
	t.Setenv("STRIPE_PRICE_BUILDER_ANNUAL", "price_builder_annual_live")
	t.Setenv("STRIPE_PRICE_PRO_MONTHLY", "price_pro_monthly_live")
	t.Setenv("STRIPE_PRICE_PRO_ANNUAL", "price_pro_annual_live")
	t.Setenv("STRIPE_PRICE_TEAM_MONTHLY", "price_team_monthly_live")

	status := EvaluateBillingLaunchConfig("sk_test_valid_but_not_placeholder_123", "")
	if status.Ready {
		t.Fatalf("expected billing launch config to be blocked, got %+v", status)
	}
	if status.WebhookConfigured {
		t.Fatalf("expected webhook to be reported missing, got %+v", status)
	}
	if !containsString(status.MissingEnv, "STRIPE_WEBHOOK_SECRET") {
		t.Fatalf("expected STRIPE_WEBHOOK_SECRET in missing env, got %+v", status.MissingEnv)
	}
	if !containsString(status.MissingEnv, "STRIPE_PRICE_TEAM_ANNUAL") {
		t.Fatalf("expected STRIPE_PRICE_TEAM_ANNUAL in missing env, got %+v", status.MissingEnv)
	}
}

func TestEvaluateBillingLaunchConfigSeparatesSecretAndPriceReadiness(t *testing.T) {
	t.Setenv("STRIPE_PRICE_BUILDER_MONTHLY", "price_builder_monthly_live")
	t.Setenv("STRIPE_PRICE_BUILDER_ANNUAL", "price_builder_annual_live")
	t.Setenv("STRIPE_PRICE_PRO_MONTHLY", "price_pro_monthly_live")
	t.Setenv("STRIPE_PRICE_PRO_ANNUAL", "price_pro_annual_live")
	t.Setenv("STRIPE_PRICE_TEAM_MONTHLY", "price_team_monthly_live")
	t.Setenv("STRIPE_PRICE_TEAM_ANNUAL", "price_team_annual_live")

	status := EvaluateBillingLaunchConfig("", "whsec_valid_123")
	if status.Ready {
		t.Fatalf("expected billing launch config to be blocked without Stripe secret, got %+v", status)
	}
	if !status.RequiredPriceIDsConfigured {
		t.Fatalf("expected price IDs to be ready even when Stripe secret is missing, got %+v", status)
	}
	if status.StripeConfigured {
		t.Fatalf("expected Stripe secret to be reported missing, got %+v", status)
	}
}

func TestEvaluateBillingLaunchConfigReportsPlaceholderPriceIDs(t *testing.T) {
	t.Setenv("STRIPE_PRICE_BUILDER_MONTHLY", "price_builder_monthly")
	t.Setenv("STRIPE_PRICE_BUILDER_ANNUAL", "price_builder_annual_live")
	t.Setenv("STRIPE_PRICE_PRO_MONTHLY", "price_pro_monthly_live")
	t.Setenv("STRIPE_PRICE_PRO_ANNUAL", "price_pro_annual_live")
	t.Setenv("STRIPE_PRICE_TEAM_MONTHLY", "price_team_monthly_live")
	t.Setenv("STRIPE_PRICE_TEAM_ANNUAL", "price_team_annual_live")

	status := EvaluateBillingLaunchConfig("sk_test_valid_but_not_placeholder_123", "whsec_valid_123")
	if status.Ready {
		t.Fatalf("expected billing launch config to be blocked, got %+v", status)
	}
	if !containsString(status.PlaceholderEnv, "STRIPE_PRICE_BUILDER_MONTHLY") {
		t.Fatalf("expected placeholder builder monthly price, got %+v", status.PlaceholderEnv)
	}
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
