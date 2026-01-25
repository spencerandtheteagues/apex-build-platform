// APEX.BUILD Payment Handlers
// Production-ready HTTP handlers for Stripe payment integration

package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"apex-build/internal/payments"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PaymentHandlers contains all payment-related HTTP handlers
type PaymentHandlers struct {
	db            *gorm.DB
	stripeService *payments.StripeService
}

// NewPaymentHandlers creates a new payment handlers instance
func NewPaymentHandlers(db *gorm.DB, stripeSecretKey string) *PaymentHandlers {
	return &PaymentHandlers{
		db:            db,
		stripeService: payments.NewStripeService(stripeSecretKey),
	}
}

// CreateCheckoutSession creates a Stripe checkout session for subscription
// POST /api/v1/billing/checkout
func (h *PaymentHandlers) CreateCheckoutSession(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
			"code":    "UNAUTHORIZED",
		})
		return
	}

	var req struct {
		PriceID    string `json:"price_id" binding:"required"`
		SuccessURL string `json:"success_url" binding:"required"`
		CancelURL  string `json:"cancel_url" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: price_id, success_url, and cancel_url are required",
			"code":    "INVALID_REQUEST",
		})
		return
	}

	// Get user from database
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
			"code":    "USER_NOT_FOUND",
		})
		return
	}

	// Check if Stripe is configured
	if !h.stripeService.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Payment system is not configured. Please contact support.",
			"code":    "STRIPE_NOT_CONFIGURED",
		})
		return
	}

	ctx := context.Background()

	// Create or retrieve Stripe customer
	customerID := user.StripeCustomerID
	if customerID == "" {
		// Create new Stripe customer
		customer, err := h.stripeService.CreateCustomer(ctx, user.Email, user.FullName, map[string]string{
			"user_id":  strconv.FormatUint(uint64(user.ID), 10),
			"username": user.Username,
		})
		if err != nil {
			log.Printf("Failed to create Stripe customer: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to create customer profile",
				"code":    "CUSTOMER_CREATION_FAILED",
			})
			return
		}

		// Save Stripe customer ID to user
		user.StripeCustomerID = customer.ID
		if err := h.db.Model(&user).Update("stripe_customer_id", customer.ID).Error; err != nil {
			log.Printf("Failed to save Stripe customer ID: %v", err)
		}
		customerID = customer.ID
	}

	// Create checkout session
	metadata := map[string]string{
		"user_id":  strconv.FormatUint(uint64(user.ID), 10),
		"username": user.Username,
		"email":    user.Email,
	}

	result, err := h.stripeService.CreateCheckoutSession(ctx, customerID, req.PriceID, req.SuccessURL, req.CancelURL, metadata)
	if err != nil {
		log.Printf("Failed to create checkout session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create checkout session",
			"code":    "CHECKOUT_CREATION_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"session_id":   result.SessionID,
			"checkout_url": result.URL,
		},
	})
}

// HandleWebhook processes Stripe webhook events
// POST /api/v1/billing/webhook
func (h *PaymentHandlers) HandleWebhook(c *gin.Context) {
	// Read the raw request body
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Failed to read webhook body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Failed to read request body",
		})
		return
	}

	// Get the Stripe signature header
	signature := c.GetHeader("Stripe-Signature")

	// Process the webhook
	event, err := h.stripeService.HandleWebhook(payload, signature)
	if err != nil {
		log.Printf("Webhook processing failed: %v", err)
		if err == payments.ErrInvalidWebhook {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid webhook signature",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to process webhook",
		})
		return
	}

	log.Printf("Processing webhook event: %s", event.Type)

	// Handle different event types
	switch event.Type {
	case "checkout.session.completed":
		h.handleCheckoutCompleted(event)

	case "customer.subscription.created", "customer.subscription.updated":
		h.handleSubscriptionUpdate(event)

	case "customer.subscription.deleted":
		h.handleSubscriptionDeleted(event)

	case "invoice.paid":
		h.handleInvoicePaid(event)

	case "invoice.payment_failed":
		h.handleInvoicePaymentFailed(event)
	}

	// Always return 200 to acknowledge receipt
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"received": true,
	})
}

// handleCheckoutCompleted processes checkout.session.completed events
func (h *PaymentHandlers) handleCheckoutCompleted(event *payments.WebhookEvent) {
	log.Printf("Checkout completed for customer: %s, subscription: %s", event.CustomerID, event.SubscriptionID)

	// Find user by Stripe customer ID
	var user models.User
	if err := h.db.Where("stripe_customer_id = ?", event.CustomerID).First(&user).Error; err != nil {
		// Try to find by user_id in metadata
		if userIDStr, ok := event.Metadata["user_id"]; ok {
			if err := h.db.First(&user, userIDStr).Error; err != nil {
				log.Printf("User not found for checkout: %v", err)
				return
			}
			// Update Stripe customer ID
			user.StripeCustomerID = event.CustomerID
		} else {
			log.Printf("User not found for checkout completion")
			return
		}
	}

	// Update user subscription info
	updates := map[string]interface{}{
		"stripe_customer_id":   event.CustomerID,
		"subscription_id":      event.SubscriptionID,
		"subscription_status":  string(event.Status),
		"billing_cycle_start":  event.PeriodStart,
	}

	if event.PlanType != "" {
		updates["subscription_type"] = string(event.PlanType)
	}

	if err := h.db.Model(&user).Updates(updates).Error; err != nil {
		log.Printf("Failed to update user subscription: %v", err)
	}

	log.Printf("User %s subscription updated to %s", user.Email, event.PlanType)
}

// handleSubscriptionUpdate processes subscription update events
func (h *PaymentHandlers) handleSubscriptionUpdate(event *payments.WebhookEvent) {
	log.Printf("Subscription update for customer: %s, status: %s, plan: %s",
		event.CustomerID, event.Status, event.PlanType)

	// Find user by Stripe customer ID
	var user models.User
	if err := h.db.Where("stripe_customer_id = ?", event.CustomerID).First(&user).Error; err != nil {
		log.Printf("User not found for subscription update: %v", err)
		return
	}

	// Update subscription details
	updates := map[string]interface{}{
		"subscription_id":     event.SubscriptionID,
		"subscription_status": string(event.Status),
		"billing_cycle_start": event.PeriodStart,
		"subscription_end":    event.PeriodEnd,
	}

	if event.PlanType != "" {
		updates["subscription_type"] = string(event.PlanType)
	}

	if err := h.db.Model(&user).Updates(updates).Error; err != nil {
		log.Printf("Failed to update user subscription: %v", err)
	}

	log.Printf("User %s subscription updated: status=%s, plan=%s", user.Email, event.Status, event.PlanType)
}

// handleSubscriptionDeleted processes subscription deletion events
func (h *PaymentHandlers) handleSubscriptionDeleted(event *payments.WebhookEvent) {
	log.Printf("Subscription deleted for customer: %s", event.CustomerID)

	// Find user by Stripe customer ID
	var user models.User
	if err := h.db.Where("stripe_customer_id = ?", event.CustomerID).First(&user).Error; err != nil {
		log.Printf("User not found for subscription deletion: %v", err)
		return
	}

	// Downgrade to free plan
	updates := map[string]interface{}{
		"subscription_id":     "",
		"subscription_status": string(payments.StatusCanceled),
		"subscription_type":   string(payments.PlanFree),
	}

	if err := h.db.Model(&user).Updates(updates).Error; err != nil {
		log.Printf("Failed to update user after subscription deletion: %v", err)
	}

	log.Printf("User %s downgraded to free plan", user.Email)
}

// handleInvoicePaid processes invoice.paid events
func (h *PaymentHandlers) handleInvoicePaid(event *payments.WebhookEvent) {
	log.Printf("Invoice paid for customer: %s, amount: %d %s",
		event.CustomerID, event.Amount, event.Currency)

	// Find user by Stripe customer ID
	var user models.User
	if err := h.db.Where("stripe_customer_id = ?", event.CustomerID).First(&user).Error; err != nil {
		log.Printf("User not found for invoice payment: %v", err)
		return
	}

	// Ensure subscription is active
	if err := h.db.Model(&user).Update("subscription_status", string(payments.StatusActive)).Error; err != nil {
		log.Printf("Failed to update subscription status: %v", err)
	}

	log.Printf("Invoice payment recorded for user %s", user.Email)
}

// handleInvoicePaymentFailed processes invoice.payment_failed events
func (h *PaymentHandlers) handleInvoicePaymentFailed(event *payments.WebhookEvent) {
	log.Printf("Invoice payment failed for customer: %s, amount: %d %s",
		event.CustomerID, event.Amount, event.Currency)

	// Find user by Stripe customer ID
	var user models.User
	if err := h.db.Where("stripe_customer_id = ?", event.CustomerID).First(&user).Error; err != nil {
		log.Printf("User not found for failed payment: %v", err)
		return
	}

	// Update subscription status to past_due
	if err := h.db.Model(&user).Update("subscription_status", string(payments.StatusPastDue)).Error; err != nil {
		log.Printf("Failed to update subscription status: %v", err)
	}

	// TODO: Send notification email to user about failed payment
	log.Printf("Payment failed for user %s - status set to past_due", user.Email)
}

// GetSubscription returns the user's current subscription details
// GET /api/v1/billing/subscription
func (h *PaymentHandlers) GetSubscription(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
			"code":    "UNAUTHORIZED",
		})
		return
	}

	// Get user from database
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
			"code":    "USER_NOT_FOUND",
		})
		return
	}

	// Get plan details
	planType := payments.PlanType(user.SubscriptionType)
	if planType == "" {
		planType = payments.PlanFree
	}
	plan := payments.GetPlanByType(planType)

	// Build subscription response
	subscription := gin.H{
		"plan_type":           string(planType),
		"plan_name":           plan.Name,
		"status":              user.SubscriptionStatus,
		"current_period_end":  user.SubscriptionEnd,
		"billing_cycle_start": user.BillingCycleStart,
		"limits":              plan.Limits,
		"features":            plan.Features,
	}

	// If user has an active Stripe subscription, get more details
	if user.SubscriptionID != "" && h.stripeService.IsConfigured() {
		ctx := context.Background()
		subInfo, err := h.stripeService.GetSubscription(ctx, user.SubscriptionID)
		if err == nil {
			subscription["cancel_at_period_end"] = subInfo.CancelAtPeriodEnd
			subscription["cancel_at"] = subInfo.CancelAt
			subscription["trial_end"] = subInfo.TrialEnd
			subscription["current_period_start"] = subInfo.CurrentPeriodStart
			subscription["current_period_end"] = subInfo.CurrentPeriodEnd
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    subscription,
	})
}

// CreateBillingPortalSession creates a Stripe billing portal session
// POST /api/v1/billing/portal
func (h *PaymentHandlers) CreateBillingPortalSession(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
			"code":    "UNAUTHORIZED",
		})
		return
	}

	var req struct {
		ReturnURL string `json:"return_url" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "return_url is required",
			"code":    "INVALID_REQUEST",
		})
		return
	}

	// Get user from database
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
			"code":    "USER_NOT_FOUND",
		})
		return
	}

	// Check if user has a Stripe customer ID
	if user.StripeCustomerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "No billing account found. Please subscribe to a plan first.",
			"code":    "NO_BILLING_ACCOUNT",
		})
		return
	}

	if !h.stripeService.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Payment system is not configured",
			"code":    "STRIPE_NOT_CONFIGURED",
		})
		return
	}

	ctx := context.Background()
	result, err := h.stripeService.CreateBillingPortalSession(ctx, user.StripeCustomerID, req.ReturnURL)
	if err != nil {
		log.Printf("Failed to create billing portal session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create billing portal session",
			"code":    "PORTAL_CREATION_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"portal_url": result.URL,
		},
	})
}

// GetPlans returns all available subscription plans
// GET /api/v1/billing/plans
func (h *PaymentHandlers) GetPlans(c *gin.Context) {
	pricing := payments.GetPricingInfo()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    pricing,
	})
}

// GetUsage returns the user's current usage statistics
// GET /api/v1/billing/usage
func (h *PaymentHandlers) GetUsage(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
			"code":    "UNAUTHORIZED",
		})
		return
	}

	// Get user from database
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
			"code":    "USER_NOT_FOUND",
		})
		return
	}

	// Get plan limits
	planType := payments.PlanType(user.SubscriptionType)
	if planType == "" {
		planType = payments.PlanFree
	}
	limits := payments.GetPlanLimits(planType)

	// Calculate current usage
	ctx := context.Background()

	// AI Requests this month
	var aiRequestsCount int64
	startOfMonth := time.Now().UTC().Truncate(24 * time.Hour).AddDate(0, 0, -time.Now().Day()+1)
	h.db.WithContext(ctx).Model(&models.AIRequest{}).
		Where("user_id = ? AND created_at >= ?", userID, startOfMonth).
		Count(&aiRequestsCount)

	// Projects count
	var projectsCount int64
	h.db.WithContext(ctx).Model(&models.Project{}).
		Where("owner_id = ?", userID).
		Count(&projectsCount)

	// Storage used (in bytes)
	var storageUsed int64
	h.db.WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(f.size), 0)
		FROM files f
		JOIN projects p ON f.project_id = p.id
		WHERE p.owner_id = ?
	`, userID).Scan(&storageUsed)

	// Code executions today
	var executionsToday int64
	startOfDay := time.Now().UTC().Truncate(24 * time.Hour)
	h.db.WithContext(ctx).Model(&models.Execution{}).
		Where("user_id = ? AND created_at >= ?", userID, startOfDay).
		Count(&executionsToday)

	usage := gin.H{
		"ai_requests": gin.H{
			"used":  aiRequestsCount,
			"limit": limits.AIRequestsPerMonth,
			"period": "month",
		},
		"projects": gin.H{
			"used":  projectsCount,
			"limit": limits.ProjectsLimit,
		},
		"storage": gin.H{
			"used_bytes": storageUsed,
			"used_gb":    float64(storageUsed) / (1024 * 1024 * 1024),
			"limit_gb":   limits.StorageGB,
		},
		"executions": gin.H{
			"used":   executionsToday,
			"limit":  limits.CodeExecutionsPerDay,
			"period": "day",
		},
		"plan_type": string(planType),
		"features": gin.H{
			"priority_ai":         limits.PriorityAI,
			"team_features":       limits.TeamFeatures,
			"dedicated_support":   limits.DedicatedSupport,
			"sla":                 limits.SLA,
			"custom_integrations": limits.CustomIntegrations,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    usage,
	})
}

// CancelSubscription cancels the user's subscription at period end
// POST /api/v1/billing/cancel
func (h *PaymentHandlers) CancelSubscription(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
			"code":    "UNAUTHORIZED",
		})
		return
	}

	// Get user from database
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
			"code":    "USER_NOT_FOUND",
		})
		return
	}

	if user.SubscriptionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "No active subscription to cancel",
			"code":    "NO_SUBSCRIPTION",
		})
		return
	}

	if !h.stripeService.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Payment system is not configured",
			"code":    "STRIPE_NOT_CONFIGURED",
		})
		return
	}

	ctx := context.Background()
	subInfo, err := h.stripeService.CancelSubscription(ctx, user.SubscriptionID)
	if err != nil {
		log.Printf("Failed to cancel subscription: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to cancel subscription",
			"code":    "CANCELLATION_FAILED",
		})
		return
	}

	// Update user in database
	h.db.Model(&user).Updates(map[string]interface{}{
		"subscription_status": string(subInfo.Status),
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Subscription will be canceled at the end of the current billing period",
		"data": gin.H{
			"cancel_at":        subInfo.CancelAt,
			"current_period_end": subInfo.CurrentPeriodEnd,
		},
	})
}

// ReactivateSubscription reactivates a canceled subscription
// POST /api/v1/billing/reactivate
func (h *PaymentHandlers) ReactivateSubscription(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
			"code":    "UNAUTHORIZED",
		})
		return
	}

	// Get user from database
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
			"code":    "USER_NOT_FOUND",
		})
		return
	}

	if user.SubscriptionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "No subscription to reactivate",
			"code":    "NO_SUBSCRIPTION",
		})
		return
	}

	if !h.stripeService.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Payment system is not configured",
			"code":    "STRIPE_NOT_CONFIGURED",
		})
		return
	}

	ctx := context.Background()
	subInfo, err := h.stripeService.ReactivateSubscription(ctx, user.SubscriptionID)
	if err != nil {
		log.Printf("Failed to reactivate subscription: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to reactivate subscription",
			"code":    "REACTIVATION_FAILED",
		})
		return
	}

	// Update user in database
	h.db.Model(&user).Updates(map[string]interface{}{
		"subscription_status": string(subInfo.Status),
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Subscription reactivated successfully",
		"data": gin.H{
			"status":             string(subInfo.Status),
			"current_period_end": subInfo.CurrentPeriodEnd,
		},
	})
}

// GetInvoices returns the user's billing history
// GET /api/v1/billing/invoices
func (h *PaymentHandlers) GetInvoices(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
			"code":    "UNAUTHORIZED",
		})
		return
	}

	// Get user from database
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
			"code":    "USER_NOT_FOUND",
		})
		return
	}

	if user.StripeCustomerID == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"invoices": []interface{}{},
				"message":  "No billing history available",
			},
		})
		return
	}

	if !h.stripeService.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Payment system is not configured",
			"code":    "STRIPE_NOT_CONFIGURED",
		})
		return
	}

	// Parse limit from query
	limit := int64(10)
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.ParseInt(l, 10, 64); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	ctx := context.Background()
	invoices, err := h.stripeService.GetInvoices(ctx, user.StripeCustomerID, limit)
	if err != nil {
		log.Printf("Failed to get invoices: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve invoices",
			"code":    "INVOICES_RETRIEVAL_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"invoices": invoices,
		},
	})
}

// GetPaymentMethods returns the user's saved payment methods
// GET /api/v1/billing/payment-methods
func (h *PaymentHandlers) GetPaymentMethods(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
			"code":    "UNAUTHORIZED",
		})
		return
	}

	// Get user from database
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
			"code":    "USER_NOT_FOUND",
		})
		return
	}

	if user.StripeCustomerID == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"payment_methods": []interface{}{},
				"message":         "No payment methods saved",
			},
		})
		return
	}

	if !h.stripeService.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Payment system is not configured",
			"code":    "STRIPE_NOT_CONFIGURED",
		})
		return
	}

	ctx := context.Background()
	methods, err := h.stripeService.GetPaymentMethods(ctx, user.StripeCustomerID)
	if err != nil {
		log.Printf("Failed to get payment methods: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve payment methods",
			"code":    "PAYMENT_METHODS_RETRIEVAL_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"payment_methods": methods,
		},
	})
}

// CheckUsageLimit checks if user has exceeded a specific usage limit
// GET /api/v1/billing/check-limit/:type
func (h *PaymentHandlers) CheckUsageLimit(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
			"code":    "UNAUTHORIZED",
		})
		return
	}

	limitType := c.Param("type")
	if limitType == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Limit type is required",
			"code":    "INVALID_REQUEST",
		})
		return
	}

	// Get user from database
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
			"code":    "USER_NOT_FOUND",
		})
		return
	}

	// Check for bypass flags
	if user.BypassBilling || user.HasUnlimitedCredits || user.IsSuperAdmin {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"within_limit": true,
				"bypassed":     true,
				"message":      "User has unlimited access",
			},
		})
		return
	}

	// Get plan limits
	planType := payments.PlanType(user.SubscriptionType)
	if planType == "" {
		planType = payments.PlanFree
	}
	limits := payments.GetPlanLimits(planType)

	var currentUsage int64
	var limit int
	var withinLimit bool

	ctx := context.Background()
	startOfMonth := time.Now().UTC().Truncate(24 * time.Hour).AddDate(0, 0, -time.Now().Day()+1)
	startOfDay := time.Now().UTC().Truncate(24 * time.Hour)

	switch limitType {
	case "ai_requests":
		h.db.WithContext(ctx).Model(&models.AIRequest{}).
			Where("user_id = ? AND created_at >= ?", userID, startOfMonth).
			Count(&currentUsage)
		limit = limits.AIRequestsPerMonth
		withinLimit = payments.IsWithinLimit(limit, int(currentUsage))

	case "projects":
		h.db.WithContext(ctx).Model(&models.Project{}).
			Where("owner_id = ?", userID).
			Count(&currentUsage)
		limit = limits.ProjectsLimit
		withinLimit = payments.IsWithinLimit(limit, int(currentUsage))

	case "storage":
		h.db.WithContext(ctx).Raw(`
			SELECT COALESCE(SUM(f.size), 0)
			FROM files f
			JOIN projects p ON f.project_id = p.id
			WHERE p.owner_id = ?
		`, userID).Scan(&currentUsage)
		limitBytes := int64(limits.StorageGB) * 1024 * 1024 * 1024
		limit = limits.StorageGB
		withinLimit = limits.StorageGB == -1 || currentUsage < limitBytes

	case "executions":
		h.db.WithContext(ctx).Model(&models.Execution{}).
			Where("user_id = ? AND created_at >= ?", userID, startOfDay).
			Count(&currentUsage)
		limit = limits.CodeExecutionsPerDay
		withinLimit = payments.IsWithinLimit(limit, int(currentUsage))

	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Unknown limit type: %s", limitType),
			"code":    "INVALID_LIMIT_TYPE",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"type":         limitType,
			"current":      currentUsage,
			"limit":        limit,
			"within_limit": withinLimit,
			"plan_type":    string(planType),
		},
	})
}

// StripeConfigStatus returns whether Stripe is configured (for frontend)
// GET /api/v1/billing/config-status
func (h *PaymentHandlers) StripeConfigStatus(c *gin.Context) {
	configured := h.stripeService.IsConfigured()

	// Don't expose the actual key, just whether it's configured
	testMode := false
	if secretKey := os.Getenv("STRIPE_SECRET_KEY"); secretKey != "" {
		testMode = len(secretKey) > 7 && secretKey[:7] == "sk_test"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"configured": configured,
			"test_mode":  testMode,
		},
	})
}
