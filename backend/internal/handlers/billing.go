package handlers

import (
	"net/http"
	"strconv"

	"apex-build/internal/billing"
	"github.com/gin-gonic/gin"
)

// BillingHandlers contains all billing-related HTTP handlers
type BillingHandlers struct {
	billingService *billing.SimpleBillingService
}

// NewBillingHandlers creates a new billing handlers instance
func NewBillingHandlers(billingService *billing.SimpleBillingService) *BillingHandlers {
	return &BillingHandlers{
		billingService: billingService,
	}
}

// GetPricing returns pricing information for all plans
func (b *BillingHandlers) GetPricing(c *gin.Context) {
	pricing := b.billingService.GetPricing()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    pricing,
	})
}

// CreateCheckoutSession creates a simplified checkout process
func (b *BillingHandlers) CreateCheckoutSession(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	var request struct {
		PriceID    string `json:"price_id" binding:"required"`
		SuccessURL string `json:"success_url" binding:"required"`
		CancelURL  string `json:"cancel_url" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request data",
		})
		return
	}

	// In the simple version, return upgrade instructions
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "To upgrade your plan, please contact support at support@apex.build",
		"contact_url": "mailto:support@apex.build?subject=Plan Upgrade Request",
	})
}

// GetSubscription returns user's current subscription details
func (b *BillingHandlers) GetSubscription(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	// In a real implementation, fetch from database
	subscription := map[string]interface{}{
		"plan":        "pro",
		"status":      "active",
		"current_period_end": "2025-02-23T10:00:00Z",
		"cancel_at_period_end": false,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    subscription,
	})
}

// GetUsage returns user's current usage statistics
func (b *BillingHandlers) GetUsage(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	usage, err := b.billingService.GetUsage(c.Request.Context(), strconv.FormatUint(uint64(userID), 10))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get usage data",
		})
		return
	}

	// Get user's plan
	userPlan, err2 := b.billingService.GetUserPlan(c.Request.Context(), strconv.FormatUint(uint64(userID), 10))
	if err2 != nil {
		userPlan = billing.PlanFree // Default to free
	}

	// Get plan limits
	plans := billing.GetPlans()
	var limits map[billing.UsageType]int
	for _, plan := range plans {
		if plan.Type == userPlan {
			limits = plan.Limits
			break
		}
	}

	response := map[string]interface{}{
		"usage":  usage,
		"limits": limits,
		"plan":   string(userPlan),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// CancelSubscription handles subscription cancellation
func (b *BillingHandlers) CancelSubscription(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "To cancel your subscription, please contact support at support@apex.build",
		"contact_url": "mailto:support@apex.build?subject=Subscription Cancellation Request",
	})
}

// UpdateSubscription handles subscription plan changes
func (b *BillingHandlers) UpdateSubscription(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "To change your subscription plan, please contact support at support@apex.build",
		"contact_url": "mailto:support@apex.build?subject=Plan Change Request",
	})
}

// HandleStripeWebhook processes webhooks (simplified version)
func (b *BillingHandlers) HandleStripeWebhook(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Webhook received - simplified billing implementation",
	})
}

// GetInvoices returns user's billing history
func (b *BillingHandlers) GetInvoices(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	// Parse pagination parameters
	page := 1
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	// In real implementation, fetch invoices from Stripe
	invoices := []map[string]interface{}{
		{
			"id":         "in_test_123",
			"date":       "2025-01-01T10:00:00Z",
			"amount":     1900,
			"status":     "paid",
			"description": "APEX.BUILD Pro Plan - Monthly",
			"pdf_url":    "https://pay.stripe.com/invoice/test_123/pdf",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"invoices":    invoices,
			"page":        page,
			"limit":       limit,
			"total_count": len(invoices),
		},
	})
}

// GetPaymentMethods returns user's saved payment methods
func (b *BillingHandlers) GetPaymentMethods(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	// In real implementation, fetch from Stripe
	paymentMethods := []map[string]interface{}{
		{
			"id":          "pm_test_123",
			"type":        "card",
			"card_brand":  "visa",
			"card_last4":  "4242",
			"card_exp_month": 12,
			"card_exp_year":  2025,
			"is_default":  true,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    paymentMethods,
	})
}

// CheckUsageLimit checks if user has exceeded usage limits
func (b *BillingHandlers) CheckUsageLimit(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	usageType := c.Param("usage_type")
	if usageType == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Usage type required",
		})
		return
	}

	// Convert string to UsageType
	var ut billing.UsageType
	switch usageType {
	case "ai_requests":
		ut = billing.UsageAIRequests
	case "code_generation":
		ut = billing.UsageCodeGen
	case "collaborators":
		ut = billing.UsageCollaborators
	case "projects":
		ut = billing.UsageProjects
	case "storage":
		ut = billing.UsageStorage
	case "executions":
		ut = billing.UsageExecutions
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid usage type",
		})
		return
	}

	// In real implementation, get user's plan from database
	userPlan := billing.PlanFree

	exceeded, err := b.billingService.CheckUsageLimit(c.Request.Context(), strconv.FormatUint(uint64(userID), 10), userPlan, ut)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to check usage limit",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"limit_exceeded": exceeded,
		"usage_type":   usageType,
		"plan":         string(userPlan),
	})
}