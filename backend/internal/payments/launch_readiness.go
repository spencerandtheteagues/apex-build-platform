package payments

import (
	"os"
	"strings"
)

type BillingLaunchConfigIssue struct {
	Code     string `json:"code"`
	EnvVar   string `json:"env_var,omitempty"`
	PlanType string `json:"plan_type,omitempty"`
	Field    string `json:"field,omitempty"`
	Summary  string `json:"summary"`
}

type BillingLaunchConfigStatus struct {
	Ready                      bool                       `json:"ready"`
	StripeConfigured           bool                       `json:"stripe_configured"`
	WebhookConfigured          bool                       `json:"webhook_configured"`
	RequiredPriceIDsConfigured bool                       `json:"required_price_ids_configured"`
	MissingEnv                 []string                   `json:"missing_env,omitempty"`
	PlaceholderEnv             []string                   `json:"placeholder_env,omitempty"`
	Issues                     []BillingLaunchConfigIssue `json:"issues,omitempty"`
}

type requiredBillingPriceEnv struct {
	EnvVar   string
	PlanType PlanType
	Field    string
}

var launchRequiredBillingPriceEnvs = []requiredBillingPriceEnv{
	{EnvVar: "STRIPE_PRICE_BUILDER_MONTHLY", PlanType: PlanBuilder, Field: "monthly_price_id"},
	{EnvVar: "STRIPE_PRICE_BUILDER_ANNUAL", PlanType: PlanBuilder, Field: "annual_price_id"},
	{EnvVar: "STRIPE_PRICE_PRO_MONTHLY", PlanType: PlanPro, Field: "monthly_price_id"},
	{EnvVar: "STRIPE_PRICE_PRO_ANNUAL", PlanType: PlanPro, Field: "annual_price_id"},
	{EnvVar: "STRIPE_PRICE_TEAM_MONTHLY", PlanType: PlanTeam, Field: "monthly_price_id"},
	{EnvVar: "STRIPE_PRICE_TEAM_ANNUAL", PlanType: PlanTeam, Field: "annual_price_id"},
}

// EvaluateBillingLaunchConfig checks whether self-serve paid billing is ready for launch.
// It intentionally ignores Enterprise/contact-sales and dynamic credit top-up price IDs.
func EvaluateBillingLaunchConfig(stripeSecretKey, webhookSecret string) BillingLaunchConfigStatus {
	status := BillingLaunchConfigStatus{
		StripeConfigured:  isConfiguredStripeSecret(stripeSecretKey),
		WebhookConfigured: isConfiguredStripeWebhookSecret(webhookSecret),
	}

	if !status.StripeConfigured {
		status.MissingEnv = append(status.MissingEnv, "STRIPE_SECRET_KEY")
		status.Issues = append(status.Issues, BillingLaunchConfigIssue{
			Code:    "STRIPE_SECRET_KEY_MISSING",
			EnvVar:  "STRIPE_SECRET_KEY",
			Summary: "Stripe secret key is not configured; checkout and billing portal are disabled.",
		})
	}

	if !status.WebhookConfigured {
		status.MissingEnv = append(status.MissingEnv, "STRIPE_WEBHOOK_SECRET")
		status.Issues = append(status.Issues, BillingLaunchConfigIssue{
			Code:    "STRIPE_WEBHOOK_SECRET_MISSING",
			EnvVar:  "STRIPE_WEBHOOK_SECRET",
			Summary: "Stripe webhook secret is not configured; subscription and credit updates cannot be trusted.",
		})
	}

	requiredPriceIDsConfigured := true
	for _, required := range launchRequiredBillingPriceEnvs {
		value, exists := os.LookupEnv(required.EnvVar)
		trimmed := strings.TrimSpace(value)
		switch {
		case !exists || trimmed == "":
			requiredPriceIDsConfigured = false
			status.MissingEnv = append(status.MissingEnv, required.EnvVar)
			status.Issues = append(status.Issues, BillingLaunchConfigIssue{
				Code:     "STRIPE_PRICE_ID_MISSING",
				EnvVar:   required.EnvVar,
				PlanType: string(required.PlanType),
				Field:    required.Field,
				Summary:  "Stripe price ID is missing for a self-serve launch plan.",
			})
		case IsPlaceholderPriceID(trimmed):
			requiredPriceIDsConfigured = false
			status.PlaceholderEnv = append(status.PlaceholderEnv, required.EnvVar)
			status.Issues = append(status.Issues, BillingLaunchConfigIssue{
				Code:     "STRIPE_PRICE_ID_PLACEHOLDER",
				EnvVar:   required.EnvVar,
				PlanType: string(required.PlanType),
				Field:    required.Field,
				Summary:  "Stripe price ID is still using a repository placeholder value.",
			})
		}
	}

	status.RequiredPriceIDsConfigured = requiredPriceIDsConfigured
	status.Ready = status.StripeConfigured && status.WebhookConfigured && status.RequiredPriceIDsConfigured
	return status
}

func (s *StripeService) LaunchConfigStatus() BillingLaunchConfigStatus {
	if s == nil {
		return EvaluateBillingLaunchConfig("", os.Getenv("STRIPE_WEBHOOK_SECRET"))
	}
	return EvaluateBillingLaunchConfig(s.secretKey, s.webhookSecret)
}

func isConfiguredStripeSecret(secret string) bool {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" || strings.EqualFold(trimmed, "sk_test_xxx") {
		return false
	}
	return true
}

func isConfiguredStripeWebhookSecret(secret string) bool {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" || strings.EqualFold(trimmed, "whsec_xxx") {
		return false
	}
	return true
}
