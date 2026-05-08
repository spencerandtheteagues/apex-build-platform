// APEX.BUILD Payment Handlers
// Production-ready HTTP handlers for Stripe payment integration

package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"apex-build/internal/email"
	"apex-build/internal/origins"
	"apex-build/internal/payments"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PaymentHandlers contains all payment-related HTTP handlers
type PaymentHandlers struct {
	db            *gorm.DB
	stripeService *payments.StripeService
	emailService  *email.Service
}

// NewPaymentHandlers creates a new payment handlers instance
func NewPaymentHandlers(db *gorm.DB, stripeSecretKey string, emailSvc ...*email.Service) *PaymentHandlers {
	ph := &PaymentHandlers{
		db:            db,
		stripeService: payments.NewStripeService(stripeSecretKey),
	}
	if len(emailSvc) > 0 {
		ph.emailService = emailSvc[0]
	}
	return ph
}

func isDuplicateInsertError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate") || strings.Contains(message, "unique")
}

func configuredAppURL() string {
	appURL := strings.TrimRight(os.Getenv("APP_URL"), "/")
	if appURL == "" {
		appURL = strings.TrimRight(os.Getenv("FRONTEND_URL"), "/")
	}
	if appURL == "" {
		appURL = "https://apex-build.dev"
	}
	return appURL
}

func sanitizeBillingPortalReturnURL(raw string) (string, error) {
	base, err := url.Parse(configuredAppURL())
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", errors.New("configured app url is invalid")
	}

	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", errors.New("invalid return url")
	}
	if parsed.User != nil {
		return "", errors.New("invalid return url")
	}

	if parsed.IsAbs() {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", errors.New("invalid return url")
		}

		baseOrigin := base.Scheme + "://" + base.Host
		candidateOrigin := parsed.Scheme + "://" + parsed.Host
		if candidateOrigin != baseOrigin {
			if origins.IsProductionEnvironment() || !origins.IsAllowedOrigin(candidateOrigin) {
				return "", errors.New("return url must stay on an approved app origin")
			}
		}

		return parsed.String(), nil
	}

	if parsed.Host != "" || parsed.Scheme != "" {
		return "", errors.New("invalid return url")
	}

	if parsed.Path == "" {
		parsed.Path = "/"
	}

	return base.ResolveReference(parsed).String(), nil
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
		SuccessURL string `json:"success_url"`
		CancelURL  string `json:"cancel_url"`
		ApplyPromo bool   `json:"apply_promo"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: price_id is required",
			"code":    "INVALID_REQUEST",
		})
		return
	}

	if payments.IsPlaceholderPriceID(req.PriceID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Billing is not configured for the selected plan yet",
			"code":    "PLAN_NOT_CONFIGURED",
		})
		return
	}

	if payments.GetPlanByPriceID(req.PriceID) == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid subscription plan",
			"code":    "INVALID_PRICE_ID",
		})
		return
	}

	// Generate redirect URLs server-side from configured base URL
	appURL := configuredAppURL()
	successURL := appURL + "/billing?success=true"
	cancelURL := appURL + "/billing?canceled=true"
	if strings.TrimSpace(req.SuccessURL) != "" {
		sanitizedURL, err := sanitizeBillingPortalReturnURL(req.SuccessURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   err.Error(),
				"code":    "INVALID_SUCCESS_URL",
			})
			return
		}
		successURL = sanitizedURL
	}
	if strings.TrimSpace(req.CancelURL) != "" {
		sanitizedURL, err := sanitizeBillingPortalReturnURL(req.CancelURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   err.Error(),
				"code":    "INVALID_CANCEL_URL",
			})
			return
		}
		cancelURL = sanitizedURL
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

	var couponID string
	if req.ApplyPromo {
		couponID = os.Getenv("STRIPE_COUPON_PRO_LAUNCH")
	}
	result, err := h.stripeService.CreateCheckoutSession(ctx, customerID, req.PriceID, successURL, cancelURL, metadata, couponID)
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
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Failed to read webhook body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Failed to read request body"})
		return
	}

	signature := c.GetHeader("Stripe-Signature")
	event, err := h.stripeService.HandleWebhook(payload, signature)
	if err != nil {
		log.Printf("Webhook processing failed: %v", err)
		if err == payments.ErrInvalidWebhook {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid webhook signature"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to process webhook"})
		return
	}

	log.Printf("Processing webhook event: %s (id=%s)", event.Type, event.EventID)

	// Events that mutate credit_balance go through the idempotent transactional path.
	// All others (subscription metadata updates) are idempotent by nature — a repeated
	// overwrite of the same status value is harmless — so they go through the normal path.
	// We propagate handler errors so Stripe retries on transient DB failures.
	var handlerErr error
	switch event.Type {
	case "checkout.session.completed":
		handlerErr = h.handleCheckoutCompleted(event)
	case "customer.subscription.created", "customer.subscription.updated":
		handlerErr = h.handleSubscriptionUpdate(event)
	case "customer.subscription.deleted":
		handlerErr = h.handleSubscriptionDeleted(event)
	case "invoice.paid":
		handlerErr = h.handleInvoicePaid(event)
	case "invoice.payment_failed":
		handlerErr = h.handleInvoicePaymentFailed(event)
	}

	if handlerErr != nil {
		log.Printf("Webhook handler error for event %s (id=%s): %v", event.Type, event.EventID, handlerErr)
		// Return 500 so Stripe retries the webhook delivery.
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "handler failed, will retry"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "received": true})
}

// applyCredit adds credits to a user inside an already-open transaction.
// It records a CreditLedgerEntry and updates credit_balance atomically.
// Returns errAlreadyProcessed (nil) if the stripe_event_id was already recorded.
func (h *PaymentHandlers) applyCredit(
	tx *gorm.DB,
	userID uint,
	amount float64,
	entryType, description, stripeEventID, stripeInvoiceID, planType string,
) error {
	if err := payments.ApplyCreditGrant(tx, userID, amount, entryType, description, stripeEventID, stripeInvoiceID, planType); err != nil {
		if stripeEventID != "" && isDuplicateInsertError(err) {
			log.Printf("Stripe event %s already processed — skipping duplicate", stripeEventID)
			return nil
		}
		return err
	}

	log.Printf("Credit applied: user=%d type=%s amount=+$%.4f event=%s",
		userID, entryType, amount, stripeEventID)
	return nil
}

// handleCheckoutCompleted processes checkout.session.completed events
func (h *PaymentHandlers) handleCheckoutCompleted(event *payments.WebhookEvent) error {
	log.Printf("Checkout completed for customer: %s, type: %s", event.CustomerID, event.Metadata["type"])

	// Route to the right handler based on purchase type
	if event.Metadata["type"] == "credit_purchase" {
		return h.handleCreditPurchaseCompleted(event)
	}

	// Default: subscription checkout
	log.Printf("Checkout subscription completed for customer: %s, subscription: %s", event.CustomerID, event.SubscriptionID)

	// Find user by Stripe customer ID
	var user models.User
	if err := h.db.Where("stripe_customer_id = ?", event.CustomerID).First(&user).Error; err != nil {
		// Try to find by user_id in metadata
		if userIDStr, ok := event.Metadata["user_id"]; ok {
			if err := h.db.First(&user, userIDStr).Error; err != nil {
				log.Printf("User not found for checkout: %v", err)
				return fmt.Errorf("user not found for checkout: %w", err)
			}
			// Update Stripe customer ID
			user.StripeCustomerID = event.CustomerID
		} else {
			log.Printf("User not found for checkout completion")
			return fmt.Errorf("user not found for checkout completion")
		}
	}

	// Update user subscription info
	updates := map[string]interface{}{
		"stripe_customer_id":  event.CustomerID,
		"subscription_id":     event.SubscriptionID,
		"subscription_status": string(event.Status),
		"billing_cycle_start": event.PeriodStart,
	}

	if event.PlanType != "" {
		updates["subscription_type"] = string(event.PlanType)
	}

	if err := h.db.Model(&user).Updates(updates).Error; err != nil {
		log.Printf("Failed to update user subscription: %v", err)
		return fmt.Errorf("failed to update user subscription: %w", err)
	}

	// Grant initial credits for the new subscription — invoice.paid will also fire,
	// but only for the *first* subscription invoiced via checkout; subsequent renewals
	// are handled exclusively by invoice.paid, so we skip the initial grant here to
	// avoid double-crediting. (Stripe always fires invoice.paid for subscription payments.)
	log.Printf("User %s subscription updated to %s", user.Email, event.PlanType)
	return nil
}

// handleCreditPurchaseCompleted credits the user's account after a successful one-time payment.
// Uses applyCredit inside a transaction so duplicate webhook deliveries are no-ops.
func (h *PaymentHandlers) handleCreditPurchaseCompleted(event *payments.WebhookEvent) error {
	creditUSDStr, ok := event.Metadata["credit_usd"]
	if !ok || creditUSDStr == "" {
		log.Printf("Credit purchase webhook missing credit_usd metadata (event=%s)", event.EventID)
		return nil // bad payload, don't retry
	}
	creditAmt, err := strconv.ParseFloat(creditUSDStr, 64)
	if err != nil || creditAmt <= 0 {
		log.Printf("Invalid credit_usd in webhook metadata: %s (event=%s)", creditUSDStr, event.EventID)
		return nil // bad payload, don't retry
	}

	// Locate user — prefer stripe_customer_id, fall back to metadata user_id.
	var user models.User
	if err := h.db.Where("stripe_customer_id = ?", event.CustomerID).First(&user).Error; err != nil {
		userIDStr, ok := event.Metadata["user_id"]
		if !ok {
			log.Printf("Cannot locate user for credit purchase: no customer or user_id (event=%s)", event.EventID)
			return fmt.Errorf("cannot locate user for credit purchase (event=%s)", event.EventID)
		}
		if err2 := h.db.First(&user, userIDStr).Error; err2 != nil {
			log.Printf("User not found for credit purchase webhook: %v (event=%s)", err2, event.EventID)
			return fmt.Errorf("user not found for credit purchase: %w", err2)
		}
		// Persist the Stripe customer ID for future lookups.
		if err3 := h.db.Model(&user).Update("stripe_customer_id", event.CustomerID).Error; err3 != nil {
			log.Printf("Warning: failed to persist stripe_customer_id for user %d (event=%s): %v", user.ID, event.EventID, err3)
		}
	}

	txErr := h.db.Transaction(func(tx *gorm.DB) error {
		return h.applyCredit(
			tx,
			user.ID,
			creditAmt,
			"credit_purchase",
			fmt.Sprintf("One-time credit purchase ($%.2f)", creditAmt),
			event.EventID,
			"",
			"",
		)
	})
	if txErr != nil {
		log.Printf("Credit purchase transaction failed for user %d: %v", user.ID, txErr)
		return txErr
	}
	return nil
}

// handleSubscriptionUpdate processes subscription update events
func (h *PaymentHandlers) handleSubscriptionUpdate(event *payments.WebhookEvent) error {
	log.Printf("Subscription update for customer: %s, status: %s, plan: %s",
		event.CustomerID, event.Status, event.PlanType)

	// Find user by Stripe customer ID
	var user models.User
	if err := h.db.Where("stripe_customer_id = ?", event.CustomerID).First(&user).Error; err != nil {
		log.Printf("User not found for subscription update: %v", err)
		return fmt.Errorf("user not found for subscription update: %w", err)
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
		return fmt.Errorf("failed to update subscription: %w", err)
	}

	log.Printf("User %s subscription updated: status=%s, plan=%s", user.Email, event.Status, event.PlanType)
	return nil
}

// handleSubscriptionDeleted processes subscription deletion events
func (h *PaymentHandlers) handleSubscriptionDeleted(event *payments.WebhookEvent) error {
	log.Printf("Subscription deleted for customer: %s", event.CustomerID)

	// Find user by Stripe customer ID
	var user models.User
	if err := h.db.Where("stripe_customer_id = ?", event.CustomerID).First(&user).Error; err != nil {
		log.Printf("User not found for subscription deletion: %v", err)
		return fmt.Errorf("user not found for subscription deletion: %w", err)
	}

	// Downgrade to free plan
	updates := map[string]interface{}{
		"subscription_id":     "",
		"subscription_status": string(payments.StatusCanceled),
		"subscription_type":   string(payments.PlanFree),
	}

	if err := h.db.Model(&user).Updates(updates).Error; err != nil {
		log.Printf("Failed to update user after subscription deletion: %v", err)
		return fmt.Errorf("failed to downgrade user: %w", err)
	}

	log.Printf("User %s downgraded to free plan", user.Email)
	return nil
}

// handleInvoicePaid processes invoice.paid events — marks subscription active and allocates monthly credits.
// Credit allocation is guarded by applyCredit's idempotency so retried webhooks are safe.
func (h *PaymentHandlers) handleInvoicePaid(event *payments.WebhookEvent) error {
	log.Printf("Invoice paid for customer: %s, amount: %d %s (event=%s)",
		event.CustomerID, event.Amount, event.Currency, event.EventID)

	var user models.User
	if err := h.db.Where("stripe_customer_id = ?", event.CustomerID).First(&user).Error; err != nil {
		log.Printf("User not found for invoice payment: %v (event=%s)", err, event.EventID)
		return fmt.Errorf("user not found for invoice payment: %w", err)
	}

	// Mark subscription active outside the credit transaction — this update is idempotent.
	if err := h.db.Model(&user).Update("subscription_status", string(payments.StatusActive)).Error; err != nil {
		log.Printf("Failed to update subscription status: %v", err)
	}

	planType := payments.PlanType(user.SubscriptionType)
	if planType == "" {
		planType = payments.PlanFree
	}
	plan := payments.GetPlanByType(planType)
	if plan == nil || plan.MonthlyCreditsUSD <= 0 {
		log.Printf("No credits to allocate for plan %s (user=%s event=%s)", planType, user.Email, event.EventID)
		return nil
	}

	txErr := h.db.Transaction(func(tx *gorm.DB) error {
		return h.applyCredit(
			tx,
			user.ID,
			plan.MonthlyCreditsUSD,
			"monthly_allocation",
			fmt.Sprintf("Monthly credit allocation — %s plan", planType),
			event.EventID,
			event.InvoiceID,
			string(planType),
		)
	})
	if txErr != nil {
		log.Printf("Monthly allocation transaction failed for user %d: %v", user.ID, txErr)
		return txErr
	}

	log.Printf("Invoice payment processed for user %s (plan=%s credits=$%.2f)", user.Email, planType, plan.MonthlyCreditsUSD)
	return nil
}

// handleInvoicePaymentFailed processes invoice.payment_failed events
func (h *PaymentHandlers) handleInvoicePaymentFailed(event *payments.WebhookEvent) error {
	log.Printf("Invoice payment failed for customer: %s, amount: %d %s",
		event.CustomerID, event.Amount, event.Currency)

	// Find user by Stripe customer ID
	var user models.User
	if err := h.db.Where("stripe_customer_id = ?", event.CustomerID).First(&user).Error; err != nil {
		log.Printf("User not found for failed payment: %v", err)
		return fmt.Errorf("user not found for failed payment: %w", err)
	}

	// Update subscription status to past_due
	if err := h.db.Model(&user).Update("subscription_status", string(payments.StatusPastDue)).Error; err != nil {
		log.Printf("Failed to update subscription status: %v", err)
		return fmt.Errorf("failed to update subscription status: %w", err)
	}

	// Send payment failure email notification
	if h.emailService != nil {
		invoiceID := ""
		if event.InvoiceID != "" {
			invoiceID = event.InvoiceID
		}
		if err := h.emailService.SendPaymentFailed(user.Email, user.Username, invoiceID); err != nil {
			log.Printf("Failed to send payment failure email to %s: %v", user.Email, err)
			// Non-fatal: don't fail the webhook because of email issues
		} else {
			log.Printf("Payment failure email sent to %s", user.Email)
		}
	}

	log.Printf("Payment failed for user %s - status set to past_due", user.Email)
	return nil
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
	if plan == nil {
		planType = payments.PlanFree
		plan = payments.GetPlanByType(planType)
	}
	if plan == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "billing plans are unavailable",
			"code":    "BILLING_PLAN_UNAVAILABLE",
		})
		return
	}

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
	if user.SubscriptionID != "" && h.stripeService != nil && h.stripeService.IsConfigured() {
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

	returnURL, err := sanitizeBillingPortalReturnURL(req.ReturnURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "return_url must stay on an approved app origin",
			"code":    "INVALID_RETURN_URL",
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
	result, err := h.stripeService.CreateBillingPortalSession(ctx, user.StripeCustomerID, returnURL)
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
	startOfMonth := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -time.Now().Day()+1)
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
			"used":   aiRequestsCount,
			"limit":  limits.AIRequestsPerMonth,
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
			"cancel_at":          subInfo.CancelAt,
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

// ChangePlan switches an existing paid subscription to a different plan or billing cycle.
// POST /api/v1/billing/change-plan
func (h *PaymentHandlers) ChangePlan(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "User not authenticated", "code": "UNAUTHORIZED"})
		return
	}

	var req struct {
		PlanType     string `json:"plan_type" binding:"required"`
		BillingCycle string `json:"billing_cycle" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "plan_type and billing_cycle are required", "code": "INVALID_REQUEST"})
		return
	}
	if req.BillingCycle != "monthly" && req.BillingCycle != "annual" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "billing_cycle must be 'monthly' or 'annual'", "code": "INVALID_BILLING_CYCLE"})
		return
	}

	newPlanType := payments.PlanType(req.PlanType)
	if newPlanType == payments.PlanFree || newPlanType == payments.PlanOwner || !payments.IsValidPlanType(req.PlanType) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid plan type for upgrade/downgrade", "code": "INVALID_PLAN_TYPE"})
		return
	}

	plan := payments.GetPlanByType(newPlanType)
	if plan == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Plan not found", "code": "PLAN_NOT_FOUND"})
		return
	}

	var newPriceID string
	if req.BillingCycle == "annual" {
		newPriceID = plan.AnnualPriceID
	} else {
		newPriceID = plan.MonthlyPriceID
	}
	if payments.IsPlaceholderPriceID(newPriceID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Billing is not fully configured for this plan", "code": "PRICE_NOT_CONFIGURED"})
		return
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "User not found", "code": "USER_NOT_FOUND"})
		return
	}
	if user.SubscriptionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "No active subscription to change. Start a new subscription via checkout.", "code": "NO_SUBSCRIPTION"})
		return
	}
	if user.SubscriptionStatus != string(payments.StatusActive) && user.SubscriptionStatus != string(payments.StatusTrialing) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Subscription is not active", "code": "SUBSCRIPTION_NOT_ACTIVE"})
		return
	}

	if !h.stripeService.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Payment system is not configured", "code": "STRIPE_NOT_CONFIGURED"})
		return
	}

	ctx := context.Background()
	subInfo, err := h.stripeService.UpdateSubscription(ctx, user.SubscriptionID, newPriceID)
	if err != nil {
		log.Printf("Failed to change plan for user %d: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to change plan", "code": "PLAN_CHANGE_FAILED"})
		return
	}

	h.db.Model(&user).Updates(map[string]interface{}{
		"subscription_type":   string(newPlanType),
		"subscription_status": string(subInfo.Status),
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Plan changed to %s (%s)", plan.Name, req.BillingCycle),
		"data": gin.H{
			"plan_type":          string(newPlanType),
			"billing_cycle":      req.BillingCycle,
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
	startOfMonth := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -time.Now().Day()+1)
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
	readiness := h.stripeService.LaunchConfigStatus()

	// Don't expose the actual key, just whether it's configured
	testMode := false
	if secretKey := os.Getenv("STRIPE_SECRET_KEY"); secretKey != "" {
		testMode = len(secretKey) > 7 && secretKey[:7] == "sk_test"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"configured":                    configured,
			"test_mode":                     testMode,
			"ready":                         readiness.Ready,
			"webhook_configured":            readiness.WebhookConfigured,
			"required_price_ids_configured": readiness.RequiredPriceIDsConfigured,
			"missing_env":                   readiness.MissingEnv,
			"placeholder_env":               readiness.PlaceholderEnv,
			"issues":                        readiness.Issues,
		},
	})
}

// PurchaseCredits creates a Stripe one-time checkout session for an AI credit top-up.
// POST /api/v1/billing/credits/purchase
func (h *PaymentHandlers) PurchaseCredits(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "User not authenticated", "code": "UNAUTHORIZED"})
		return
	}

	var req struct {
		AmountUSD int64 `json:"amount_usd" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "amount_usd is required", "code": "INVALID_REQUEST"})
		return
	}

	// Generate redirect URLs server-side
	creditAppURL := configuredAppURL()
	creditSuccessURL := creditAppURL + "/billing?credits=success"
	creditCancelURL := creditAppURL + "/billing?credits=canceled"

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "User not found", "code": "USER_NOT_FOUND"})
		return
	}

	if !h.stripeService.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Payment system is not configured", "code": "STRIPE_NOT_CONFIGURED"})
		return
	}

	ctx := c.Request.Context()

	// Ensure Stripe customer exists
	customerID := user.StripeCustomerID
	if customerID == "" {
		cust, err := h.stripeService.CreateCustomer(ctx, user.Email, user.FullName, map[string]string{
			"user_id":  strconv.FormatUint(uint64(user.ID), 10),
			"username": user.Username,
		})
		if err != nil {
			log.Printf("Failed to create Stripe customer: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create billing profile", "code": "CUSTOMER_CREATION_FAILED"})
			return
		}
		h.db.Model(&user).Update("stripe_customer_id", cust.ID)
		customerID = cust.ID
	}

	meta := map[string]string{
		"user_id":  strconv.FormatUint(uint64(user.ID), 10),
		"username": user.Username,
		"email":    user.Email,
	}

	result, err := h.stripeService.CreateCreditPurchaseSession(ctx, customerID, req.AmountUSD, creditSuccessURL, creditCancelURL, meta)
	if err != nil {
		log.Printf("Failed to create credit purchase session: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error(), "code": "CHECKOUT_CREATION_FAILED"})
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

// GetCreditBalance returns the authenticated user's current credit balance.
// GET /api/v1/billing/credits/balance
func (h *PaymentHandlers) GetCreditBalance(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "User not authenticated", "code": "UNAUTHORIZED"})
		return
	}

	var user models.User
	if err := h.db.Select("id, credit_balance, has_unlimited_credits, bypass_billing").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "User not found", "code": "USER_NOT_FOUND"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"balance":         user.CreditBalance,
			"has_unlimited":   user.HasUnlimitedCredits,
			"bypass_billing":  user.BypassBilling,
			"available_packs": payments.CreditPacks(),
		},
	})
}

// GetCreditLedger returns the paginated credit ledger for the authenticated user.
// Each entry is an immutable record of a credit or debit event.
// GET /api/v1/billing/credits/ledger
func (h *PaymentHandlers) GetCreditLedger(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "User not authenticated", "code": "UNAUTHORIZED"})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 200 {
		limit = 50
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	var entries []models.CreditLedgerEntry
	if err := h.db.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch ledger", "code": "DATABASE_ERROR"})
		return
	}

	var total int64
	h.db.Model(&models.CreditLedgerEntry{}).Where("user_id = ?", userID).Count(&total)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"entries": entries,
			"total":   total,
			"limit":   limit,
			"offset":  offset,
		},
	})
}
