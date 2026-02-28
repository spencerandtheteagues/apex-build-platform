// APEX.BUILD Subscription Plans Configuration
// Production-ready plan definitions with Stripe integration

package payments

import (
	"os"
)

// PlanType represents different subscription tiers
type PlanType string

const (
	PlanFree       PlanType = "free"
	PlanBuilder    PlanType = "builder"
	PlanPro        PlanType = "pro"
	PlanTeam       PlanType = "team"
	PlanEnterprise PlanType = "enterprise"
)

// SubscriptionStatus represents the current state of a subscription
type SubscriptionStatus string

const (
	StatusActive   SubscriptionStatus = "active"
	StatusCanceled SubscriptionStatus = "canceled"
	StatusPastDue  SubscriptionStatus = "past_due"
	StatusTrialing SubscriptionStatus = "trialing"
	StatusInactive SubscriptionStatus = "inactive"
)

// Plan represents a subscription plan with full details
type Plan struct {
	Type               PlanType          `json:"type"`
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	MonthlyPriceCents  int64             `json:"monthly_price_cents"`
	MonthlyPriceID     string            `json:"monthly_price_id"`
	AnnualPriceCents   int64             `json:"annual_price_cents"`
	AnnualPriceID      string            `json:"annual_price_id"`
	MonthlyCreditsUSD  float64           `json:"monthly_credits_usd"` // Credits added to balance each billing cycle
	Limits             PlanLimits        `json:"limits"`
	Features           []string          `json:"features"`
	IsPopular          bool              `json:"is_popular"`
	IsEnterprise       bool              `json:"is_enterprise"`
	TrialDays          int               `json:"trial_days"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

// PlanLimits defines the resource limits for each plan
type PlanLimits struct {
	AIRequestsPerMonth     int   `json:"ai_requests_per_month"`     // Platform-key requests; -1 for unlimited
	BYOKEnabled            bool  `json:"byok_enabled"`              // Can use Bring Your Own Key
	BYOKUnlimited          bool  `json:"byok_unlimited"`            // Unlimited requests via BYOK
	ProjectsLimit          int   `json:"projects_limit"`            // -1 for unlimited
	StorageGB              int   `json:"storage_gb"`
	CollaboratorsPerProject int  `json:"collaborators_per_project"` // -1 for unlimited
	CodeExecutionsPerDay   int   `json:"code_executions_per_day"`
	GitHubExport           bool  `json:"github_export"`
	PriorityAI             bool  `json:"priority_ai"`
	TeamFeatures           bool  `json:"team_features"`
	DedicatedSupport       bool  `json:"dedicated_support"`
	SLA                    bool  `json:"sla"`
	CustomIntegrations     bool  `json:"custom_integrations"`
}

// PlanConfig holds the environment-based configuration for plans
type PlanConfig struct {
	StripePriceIDBuilderMonthly    string
	StripePriceIDBuilderAnnual     string
	StripePriceIDProMonthly        string
	StripePriceIDProAnnual         string
	StripePriceIDTeamMonthly       string
	StripePriceIDTeamAnnual        string
	StripePriceIDEnterpriseMonthly string
	StripePriceIDEnterpriseAnnual  string
}

// LoadPlanConfig loads plan configuration from environment variables
func LoadPlanConfig() *PlanConfig {
	return &PlanConfig{
		StripePriceIDBuilderMonthly:    getEnvOrDefault("STRIPE_PRICE_BUILDER_MONTHLY", "price_builder_monthly"),
		StripePriceIDBuilderAnnual:     getEnvOrDefault("STRIPE_PRICE_BUILDER_ANNUAL", "price_builder_annual"),
		StripePriceIDProMonthly:        getEnvOrDefault("STRIPE_PRICE_PRO_MONTHLY", "price_pro_monthly"),
		StripePriceIDProAnnual:         getEnvOrDefault("STRIPE_PRICE_PRO_ANNUAL", "price_pro_annual"),
		StripePriceIDTeamMonthly:       getEnvOrDefault("STRIPE_PRICE_TEAM_MONTHLY", "price_team_monthly"),
		StripePriceIDTeamAnnual:        getEnvOrDefault("STRIPE_PRICE_TEAM_ANNUAL", "price_team_annual"),
		StripePriceIDEnterpriseMonthly: getEnvOrDefault("STRIPE_PRICE_ENTERPRISE_MONTHLY", "price_enterprise_monthly"),
		StripePriceIDEnterpriseAnnual:  getEnvOrDefault("STRIPE_PRICE_ENTERPRISE_ANNUAL", "price_enterprise_annual"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetAllPlans returns all available subscription plans
func GetAllPlans() []Plan {
	config := LoadPlanConfig()

	return []Plan{
		// Free Plan - $0/month — BYOK-only, transparent cost
		{
			Type:              PlanFree,
			Name:              "Free",
			Description:       "Evaluate the platform with your own API keys",
			MonthlyPriceCents: 0,
			MonthlyPriceID:    "",
			AnnualPriceCents:  0,
			AnnualPriceID:     "",
			MonthlyCreditsUSD: 0,
			Limits: PlanLimits{
				AIRequestsPerMonth:      0, // BYOK only
				BYOKEnabled:             true,
				BYOKUnlimited:           true,
				ProjectsLimit:           3,
				StorageGB:               1,
				CollaboratorsPerProject: 1,
				CodeExecutionsPerDay:    50,
				GitHubExport:            false,
				PriorityAI:              false,
				TeamFeatures:            false,
				DedicatedSupport:        false,
				SLA:                     false,
				CustomIntegrations:      false,
			},
			Features: []string{
				"Bring your own API keys",
				"All 5 AI agents",
				"All tech stacks",
				"Full cost transparency dashboard",
			},
			IsPopular:    false,
			IsEnterprise: false,
			TrialDays:    0,
		},

		// Builder Plan - $19/month — entry managed AI tier
		{
			Type:              PlanBuilder,
			Name:              "Builder",
			Description:       "For solo developers who want managed AI credits",
			MonthlyPriceCents: 1900, // $19.00
			MonthlyPriceID:    config.StripePriceIDBuilderMonthly,
			AnnualPriceCents:  18240, // $182.40/year ($15.20/month — 20% off)
			AnnualPriceID:     config.StripePriceIDBuilderAnnual,
			MonthlyCreditsUSD: 10.00, // $10 in AI credits per billing cycle
			Limits: PlanLimits{
				AIRequestsPerMonth:      -1, // Unlimited via credits
				BYOKEnabled:             true,
				BYOKUnlimited:           true,
				ProjectsLimit:           -1,
				StorageGB:               5,
				CollaboratorsPerProject: 1,
				CodeExecutionsPerDay:    200,
				GitHubExport:            true,
				PriorityAI:              false,
				TeamFeatures:            false,
				DedicatedSupport:        false,
				SLA:                     false,
				CustomIntegrations:      false,
			},
			Features: []string{
				"$10 in managed AI credits / mo",
				"All 5 AI agents",
				"All 6 AI providers",
				"All tech stacks",
				"Full cost transparency dashboard",
				"Live per-token cost tracking",
			},
			IsPopular:    false,
			IsEnterprise: false,
			TrialDays:    0,
		},

		// Pro Plan - $49/month — best value
		{
			Type:              PlanPro,
			Name:              "Pro",
			Description:       "Best value — for developers who ship constantly",
			MonthlyPriceCents: 4900, // $49.00
			MonthlyPriceID:    config.StripePriceIDProMonthly,
			AnnualPriceCents:  47040, // $470.40/year ($39.20/month — 20% off)
			AnnualPriceID:     config.StripePriceIDProAnnual,
			MonthlyCreditsUSD: 35.00, // $35 in AI credits per billing cycle
			Limits: PlanLimits{
				AIRequestsPerMonth:      -1, // Unlimited via credits
				BYOKEnabled:             true,
				BYOKUnlimited:           true,
				ProjectsLimit:           -1,
				StorageGB:               20,
				CollaboratorsPerProject: 3,
				CodeExecutionsPerDay:    1000,
				GitHubExport:            true,
				PriorityAI:              true,
				TeamFeatures:            false,
				DedicatedSupport:        false,
				SLA:                     false,
				CustomIntegrations:      false,
			},
			Features: []string{
				"$35 in managed AI credits / mo",
				"All 5 AI agents",
				"All 6 AI providers",
				"All tech stacks",
				"Full cost transparency dashboard",
				"Live per-token cost tracking",
				"Priority build queue",
			},
			IsPopular:    true,
			IsEnterprise: false,
			TrialDays:    0,
		},

		// Team Plan - $99/month — for small teams
		{
			Type:              PlanTeam,
			Name:              "Team",
			Description:       "For teams — shared workspace, up to 5 seats",
			MonthlyPriceCents: 9900, // $99.00
			MonthlyPriceID:    config.StripePriceIDTeamMonthly,
			AnnualPriceCents:  95040, // $950.40/year ($79.20/month — 20% off)
			AnnualPriceID:     config.StripePriceIDTeamAnnual,
			MonthlyCreditsUSD: 80.00, // $80 in AI credits per billing cycle
			Limits: PlanLimits{
				AIRequestsPerMonth:      -1, // Unlimited via credits
				BYOKEnabled:             true,
				BYOKUnlimited:           true,
				ProjectsLimit:           -1,
				StorageGB:               50,
				CollaboratorsPerProject: -1,
				CodeExecutionsPerDay:    5000,
				GitHubExport:            true,
				PriorityAI:              true,
				TeamFeatures:            true,
				DedicatedSupport:        false,
				SLA:                     false,
				CustomIntegrations:      true,
			},
			Features: []string{
				"$80 in managed AI credits / mo",
				"All 5 AI agents",
				"All 6 AI providers",
				"All tech stacks",
				"Full cost transparency dashboard",
				"Live per-token cost tracking",
				"Priority build queue",
				"Shared team workspace",
				"Up to 5 seats",
			},
			IsPopular:    false,
			IsEnterprise: false,
			TrialDays:    0,
		},

		// Enterprise Plan - custom pricing (contact sales)
		{
			Type:              PlanEnterprise,
			Name:              "Enterprise",
			Description:       "Full platform with SAML/SCIM, audit logs, SLA, and dedicated support",
			MonthlyPriceCents: 0, // Contact sales
			MonthlyPriceID:    config.StripePriceIDEnterpriseMonthly,
			AnnualPriceCents:  0,
			AnnualPriceID:     config.StripePriceIDEnterpriseAnnual,
			MonthlyCreditsUSD: 0, // Custom amount agreed with sales
			Limits: PlanLimits{
				AIRequestsPerMonth:      -1,
				BYOKEnabled:             true,
				BYOKUnlimited:           true,
				ProjectsLimit:           -1,
				StorageGB:               -1,
				CollaboratorsPerProject: -1,
				CodeExecutionsPerDay:    -1,
				GitHubExport:            true,
				PriorityAI:              true,
				TeamFeatures:            true,
				DedicatedSupport:        true,
				SLA:                     true,
				CustomIntegrations:      true,
			},
			Features: []string{
				"Unlimited AI credits (negotiated)",
				"Unlimited seats",
				"SAML/SCIM SSO",
				"Audit logs",
				"99.9% SLA",
				"24/7 dedicated support",
				"Custom contracts",
			},
			IsPopular:    false,
			IsEnterprise: true,
			TrialDays:    30,
		},
	}
}

// GetPlanByType returns a specific plan by its type
func GetPlanByType(planType PlanType) *Plan {
	plans := GetAllPlans()
	for _, plan := range plans {
		if plan.Type == planType {
			return &plan
		}
	}
	return nil
}

// GetPlanByPriceID returns a plan by its Stripe price ID
func GetPlanByPriceID(priceID string) *Plan {
	if priceID == "" {
		return nil
	}

	plans := GetAllPlans()
	for _, plan := range plans {
		if plan.MonthlyPriceID == priceID || plan.AnnualPriceID == priceID {
			return &plan
		}
	}
	return nil
}

// GetPlanTypeByPriceID returns the plan type for a given Stripe price ID
func GetPlanTypeByPriceID(priceID string) PlanType {
	plan := GetPlanByPriceID(priceID)
	if plan != nil {
		return plan.Type
	}
	return PlanFree
}

// IsValidPlanType checks if a plan type is valid
func IsValidPlanType(planType string) bool {
	switch PlanType(planType) {
	case PlanFree, PlanBuilder, PlanPro, PlanTeam, PlanEnterprise:
		return true
	}
	return false
}

// GetPlanLimits returns the limits for a specific plan type
func GetPlanLimits(planType PlanType) *PlanLimits {
	plan := GetPlanByType(planType)
	if plan != nil {
		return &plan.Limits
	}
	// Return free plan limits as default
	freePlan := GetPlanByType(PlanFree)
	if freePlan != nil {
		return &freePlan.Limits
	}
	return nil
}

// CanAccessFeature checks if a plan has access to a specific feature
func CanAccessFeature(planType PlanType, feature string) bool {
	limits := GetPlanLimits(planType)
	if limits == nil {
		return false
	}

	switch feature {
	case "priority_ai":
		return limits.PriorityAI
	case "team_features":
		return limits.TeamFeatures
	case "dedicated_support":
		return limits.DedicatedSupport
	case "sla":
		return limits.SLA
	case "custom_integrations":
		return limits.CustomIntegrations
	default:
		return true
	}
}

// IsWithinLimit checks if a value is within the plan limit (-1 means unlimited)
func IsWithinLimit(limit, current int) bool {
	if limit == -1 {
		return true // Unlimited
	}
	return current < limit
}

// PricingInfo returns a formatted pricing structure for the frontend
type PricingInfo struct {
	Plans         []Plan `json:"plans"`
	Currency      string `json:"currency"`
	CurrencySymbol string `json:"currency_symbol"`
	BillingCycles []string `json:"billing_cycles"`
	TrialAvailable bool   `json:"trial_available"`
}

// GetPricingInfo returns complete pricing information for display
func GetPricingInfo() *PricingInfo {
	return &PricingInfo{
		Plans:          GetAllPlans(),
		Currency:       "usd",
		CurrencySymbol: "$",
		BillingCycles:  []string{"monthly", "annual"},
		TrialAvailable: true,
	}
}
