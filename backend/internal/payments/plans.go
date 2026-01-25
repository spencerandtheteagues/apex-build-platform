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
	Limits             PlanLimits        `json:"limits"`
	Features           []string          `json:"features"`
	IsPopular          bool              `json:"is_popular"`
	IsEnterprise       bool              `json:"is_enterprise"`
	TrialDays          int               `json:"trial_days"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

// PlanLimits defines the resource limits for each plan
type PlanLimits struct {
	AIRequestsPerMonth     int   `json:"ai_requests_per_month"`
	ProjectsLimit          int   `json:"projects_limit"`          // -1 for unlimited
	StorageGB              int   `json:"storage_gb"`
	CollaboratorsPerProject int  `json:"collaborators_per_project"` // -1 for unlimited
	CodeExecutionsPerDay   int   `json:"code_executions_per_day"`
	PriorityAI             bool  `json:"priority_ai"`
	TeamFeatures           bool  `json:"team_features"`
	DedicatedSupport       bool  `json:"dedicated_support"`
	SLA                    bool  `json:"sla"`
	CustomIntegrations     bool  `json:"custom_integrations"`
}

// PlanConfig holds the environment-based configuration for plans
type PlanConfig struct {
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
		// Free Plan - $0/month
		{
			Type:              PlanFree,
			Name:              "Free",
			Description:       "Perfect for getting started with cloud development",
			MonthlyPriceCents: 0,
			MonthlyPriceID:    "", // No Stripe price for free plan
			AnnualPriceCents:  0,
			AnnualPriceID:     "",
			Limits: PlanLimits{
				AIRequestsPerMonth:     1000,
				ProjectsLimit:          3,
				StorageGB:              1,
				CollaboratorsPerProject: 1,
				CodeExecutionsPerDay:   50,
				PriorityAI:             false,
				TeamFeatures:           false,
				DedicatedSupport:       false,
				SLA:                    false,
				CustomIntegrations:     false,
			},
			Features: []string{
				"1,000 AI requests/month",
				"3 projects",
				"1GB storage",
				"Basic code editor",
				"Community support",
				"Public projects only",
			},
			IsPopular:    false,
			IsEnterprise: false,
			TrialDays:    0,
		},

		// Pro Plan - $12/month
		{
			Type:              PlanPro,
			Name:              "Pro",
			Description:       "For individual developers who need more power",
			MonthlyPriceCents: 1200, // $12.00
			MonthlyPriceID:    config.StripePriceIDProMonthly,
			AnnualPriceCents:  11520, // $115.20/year ($9.60/month - 20% off)
			AnnualPriceID:     config.StripePriceIDProAnnual,
			Limits: PlanLimits{
				AIRequestsPerMonth:     10000,
				ProjectsLimit:          -1, // Unlimited
				StorageGB:              10,
				CollaboratorsPerProject: 3,
				CodeExecutionsPerDay:   500,
				PriorityAI:             true,
				TeamFeatures:           false,
				DedicatedSupport:       false,
				SLA:                    false,
				CustomIntegrations:     false,
			},
			Features: []string{
				"10,000 AI requests/month",
				"Unlimited projects",
				"10GB storage",
				"Priority AI responses",
				"Private projects",
				"Advanced code editor",
				"All AI models (Claude, GPT-4, Gemini)",
				"Email support",
				"Custom themes",
				"Git integration",
			},
			IsPopular:    true,
			IsEnterprise: false,
			TrialDays:    14,
		},

		// Team Plan - $29/month
		{
			Type:              PlanTeam,
			Name:              "Team",
			Description:       "For teams that build together",
			MonthlyPriceCents: 2900, // $29.00
			MonthlyPriceID:    config.StripePriceIDTeamMonthly,
			AnnualPriceCents:  27840, // $278.40/year ($23.20/month - 20% off)
			AnnualPriceID:     config.StripePriceIDTeamAnnual,
			Limits: PlanLimits{
				AIRequestsPerMonth:     50000,
				ProjectsLimit:          -1, // Unlimited
				StorageGB:              50,
				CollaboratorsPerProject: -1, // Unlimited
				CodeExecutionsPerDay:   2000,
				PriorityAI:             true,
				TeamFeatures:           true,
				DedicatedSupport:       false,
				SLA:                    false,
				CustomIntegrations:     true,
			},
			Features: []string{
				"50,000 AI requests/month",
				"Unlimited projects",
				"50GB storage",
				"Real-time collaboration",
				"Team management",
				"Priority AI responses",
				"All AI models",
				"Priority support",
				"Advanced analytics",
				"SSO integration",
				"Custom integrations",
				"API access",
			},
			IsPopular:    false,
			IsEnterprise: false,
			TrialDays:    14,
		},

		// Enterprise Plan - $99/month
		{
			Type:              PlanEnterprise,
			Name:              "Enterprise",
			Description:       "For organizations that need everything",
			MonthlyPriceCents: 9900, // $99.00
			MonthlyPriceID:    config.StripePriceIDEnterpriseMonthly,
			AnnualPriceCents:  95040, // $950.40/year ($79.20/month - 20% off)
			AnnualPriceID:     config.StripePriceIDEnterpriseAnnual,
			Limits: PlanLimits{
				AIRequestsPerMonth:     -1, // Unlimited
				ProjectsLimit:          -1, // Unlimited
				StorageGB:              -1, // Unlimited
				CollaboratorsPerProject: -1, // Unlimited
				CodeExecutionsPerDay:   -1, // Unlimited
				PriorityAI:             true,
				TeamFeatures:           true,
				DedicatedSupport:       true,
				SLA:                    true,
				CustomIntegrations:     true,
			},
			Features: []string{
				"Unlimited AI requests",
				"Unlimited projects",
				"Unlimited storage",
				"Unlimited collaborators",
				"Real-time collaboration",
				"Team management",
				"Priority AI responses",
				"All AI models",
				"24/7 dedicated support",
				"99.9% SLA guarantee",
				"Advanced security",
				"On-premise deployment option",
				"Custom integrations",
				"Dedicated account manager",
				"Custom contracts",
				"SAML/SCIM SSO",
				"Audit logs",
				"SOC 2 compliance",
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
	case PlanFree, PlanPro, PlanTeam, PlanEnterprise:
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
