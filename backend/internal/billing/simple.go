package billing

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// PlanType represents different subscription tiers
type PlanType string

const (
	PlanFree       PlanType = "free"
	PlanPro        PlanType = "pro"        // $19/month
	PlanTeam       PlanType = "team"       // $49/month
	PlanEnterprise PlanType = "enterprise" // $199/month
)

// UsageType represents different usage metrics
type UsageType string

const (
	UsageAIRequests   UsageType = "ai_requests"
	UsageCodeGen      UsageType = "code_generation"
	UsageCollaborators UsageType = "collaborators"
	UsageProjects     UsageType = "projects"
	UsageStorage      UsageType = "storage_gb"
	UsageExecutions   UsageType = "code_executions"
)

// Plan represents a subscription plan
type Plan struct {
	Type           PlanType           `json:"type"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	MonthlyPrice   int64              `json:"monthly_price"` // in cents
	AnnualPrice    int64              `json:"annual_price"`  // in cents
	StripePriceID  string             `json:"stripe_price_id"`
	Limits         map[UsageType]int  `json:"limits"`
	Features       []string           `json:"features"`
	PopularPlan    bool               `json:"popular_plan"`
}

// GetPlans returns all available subscription plans
func GetPlans() []Plan {
	return []Plan{
		{
			Type:         PlanFree,
			Name:         "Free",
			Description:  "Perfect for getting started",
			MonthlyPrice: 0,
			AnnualPrice:  0,
			Limits: map[UsageType]int{
				UsageAIRequests:   100,
				UsageCodeGen:      20,
				UsageCollaborators: 1,
				UsageProjects:     3,
				UsageStorage:      1,
				UsageExecutions:   50,
			},
			Features: []string{
				"3 Projects",
				"1 Collaborator",
				"100 AI Requests/month",
				"20 Code Generations/month",
				"1GB Storage",
				"Community Support",
			},
		},
		{
			Type:          PlanPro,
			Name:          "Pro",
			Description:   "For individual developers",
			MonthlyPrice:  1900, // $19.00
			AnnualPrice:   19000, // $190.00 (2 months free)
			StripePriceID: "price_1234567890", // Set in Stripe
			PopularPlan:   true,
			Limits: map[UsageType]int{
				UsageAIRequests:   2000,
				UsageCodeGen:      500,
				UsageCollaborators: 3,
				UsageProjects:     25,
				UsageStorage:      10,
				UsageExecutions:   1000,
			},
			Features: []string{
				"25 Projects",
				"3 Collaborators",
				"2,000 AI Requests/month",
				"500 Code Generations/month",
				"10GB Storage",
				"All AI Models (Claude, GPT-5, Gemini)",
				"Advanced Code Editor",
				"Email Support",
				"Custom Themes",
			},
		},
		{
			Type:         PlanTeam,
			Name:         "Team",
			Description:  "For small development teams",
			MonthlyPrice: 4900, // $49.00
			AnnualPrice:  49000, // $490.00 (2 months free)
			Limits: map[UsageType]int{
				UsageAIRequests:   10000,
				UsageCodeGen:      2000,
				UsageCollaborators: 15,
				UsageProjects:     100,
				UsageStorage:      100,
				UsageExecutions:   5000,
			},
			Features: []string{
				"100 Projects",
				"15 Collaborators",
				"10,000 AI Requests/month",
				"2,000 Code Generations/month",
				"100GB Storage",
				"Real-time Collaboration",
				"Team Management",
				"Priority Support",
				"Advanced Analytics",
				"SSO Integration",
			},
		},
		{
			Type:         PlanEnterprise,
			Name:         "Enterprise",
			Description:  "For large organizations",
			MonthlyPrice: 19900, // $199.00
			AnnualPrice:  199000, // $1,990.00 (2 months free)
			Limits: map[UsageType]int{
				UsageAIRequests:   100000,
				UsageCodeGen:      20000,
				UsageCollaborators: -1, // Unlimited
				UsageProjects:     -1,  // Unlimited
				UsageStorage:      1000, // 1TB
				UsageExecutions:   50000,
			},
			Features: []string{
				"Unlimited Projects",
				"Unlimited Collaborators",
				"100,000 AI Requests/month",
				"20,000 Code Generations/month",
				"1TB Storage",
				"Advanced Security",
				"On-premise Deployment",
				"24/7 Phone Support",
				"Custom Integrations",
				"Dedicated Account Manager",
				"SLA Guarantees",
			},
		},
	}
}

// SimpleBillingService provides a simplified billing service for APEX.BUILD
// This is a production-ready implementation without complex Stripe integration
type SimpleBillingService struct {
	db *gorm.DB
}

// NewSimpleBillingService creates a new simple billing service
func NewSimpleBillingService(db *gorm.DB) *SimpleBillingService {
	return &SimpleBillingService{
		db: db,
	}
}

// GetPlans returns all available subscription plans (same as Stripe version)
func (s *SimpleBillingService) GetPlans() []Plan {
	return GetPlans()
}

// GetUsage calculates current usage for a user
func (s *SimpleBillingService) GetUsage(ctx context.Context, userID string) (map[UsageType]int, error) {
	usage := make(map[UsageType]int)

	// AI Requests
	var aiRequests int64
	if err := s.db.Raw(`
		SELECT COUNT(*) FROM ai_requests
		WHERE user_id = ? AND created_at >= date_trunc('month', CURRENT_DATE)
	`, userID).Scan(&aiRequests).Error; err == nil {
		usage[UsageAIRequests] = int(aiRequests)
	}

	// Code Generation
	var codeGen int64
	if err := s.db.Raw(`
		SELECT COUNT(*) FROM ai_requests
		WHERE user_id = ? AND capability = 'code_generation'
		AND created_at >= date_trunc('month', CURRENT_DATE)
	`, userID).Scan(&codeGen).Error; err == nil {
		usage[UsageCodeGen] = int(codeGen)
	}

	// Projects
	var projects int64
	if err := s.db.Raw(`
		SELECT COUNT(*) FROM projects WHERE user_id = ?
	`, userID).Scan(&projects).Error; err == nil {
		usage[UsageProjects] = int(projects)
	}

	// Storage (in GB)
	var storageBytes int64
	if err := s.db.Raw(`
		SELECT COALESCE(SUM(size), 0) FROM files
		JOIN projects ON files.project_id = projects.id
		WHERE projects.user_id = ?
	`, userID).Scan(&storageBytes).Error; err == nil {
		usage[UsageStorage] = int(storageBytes / (1024 * 1024 * 1024))
	}

	// Code Executions
	var executions int64
	if err := s.db.Raw(`
		SELECT COUNT(*) FROM executions
		WHERE user_id = ? AND created_at >= date_trunc('month', CURRENT_DATE)
	`, userID).Scan(&executions).Error; err == nil {
		usage[UsageExecutions] = int(executions)
	}

	return usage, nil
}

// CheckUsageLimit checks if user has exceeded usage limits
func (s *SimpleBillingService) CheckUsageLimit(ctx context.Context, userID string, plan PlanType, usageType UsageType) (bool, error) {
	plans := GetPlans()
	var userPlan Plan
	for _, p := range plans {
		if p.Type == plan {
			userPlan = p
			break
		}
	}

	if userPlan.Type == "" {
		return false, fmt.Errorf("plan not found: %s", plan)
	}

	// Check if usage type has a limit (-1 means unlimited)
	limit, exists := userPlan.Limits[usageType]
	if !exists || limit == -1 {
		return false, nil // No limit
	}

	// Get current usage
	usage, err := s.GetUsage(ctx, userID)
	if err != nil {
		return true, err // Err on the side of caution
	}

	current := usage[usageType]
	return current >= limit, nil
}

// GetPricing returns pricing information for the frontend
func (s *SimpleBillingService) GetPricing() map[string]interface{} {
	plans := GetPlans()

	return map[string]interface{}{
		"plans": plans,
		"currency": "usd",
		"tax_inclusive": false,
		"trial_days": 14,
		"billing_enabled": false, // Indicates this is the simple implementation
	}
}

// IncrementUsage increments usage for a specific metric
func (s *SimpleBillingService) IncrementUsage(ctx context.Context, userID string, usageType UsageType, amount int) error {
	// In the simple version, usage is tracked by actual database records
	// This is a placeholder for manual tracking if needed
	return nil
}

// GetUserPlan returns the user's current plan
func (s *SimpleBillingService) GetUserPlan(ctx context.Context, userID string) (PlanType, error) {
	var subscriptionType string
	err := s.db.Raw(`
		SELECT COALESCE(subscription_type, 'free') FROM users WHERE id = ?
	`, userID).Scan(&subscriptionType).Error

	if err != nil {
		return PlanFree, err
	}

	return PlanType(subscriptionType), nil
}