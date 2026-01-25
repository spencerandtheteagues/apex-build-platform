// APEX.BUILD Stripe Payment Integration
// Production-ready Stripe service with full webhook handling

package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/invoice"
	"github.com/stripe/stripe-go/v76/paymentmethod"
	"github.com/stripe/stripe-go/v76/subscription"
	"github.com/stripe/stripe-go/v76/webhook"
)

// Common errors
var (
	ErrCustomerNotFound     = errors.New("customer not found")
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrInvalidWebhook       = errors.New("invalid webhook signature")
	ErrInvalidPriceID       = errors.New("invalid price ID")
	ErrSubscriptionInactive = errors.New("subscription is not active")
)

// StripeService handles all Stripe payment operations
type StripeService struct {
	secretKey      string
	webhookSecret  string
	planConfig     *PlanConfig
}

// WebhookEvent represents a processed webhook event
type WebhookEvent struct {
	Type            string                 `json:"type"`
	CustomerID      string                 `json:"customer_id,omitempty"`
	SubscriptionID  string                 `json:"subscription_id,omitempty"`
	PriceID         string                 `json:"price_id,omitempty"`
	Status          SubscriptionStatus     `json:"status,omitempty"`
	PlanType        PlanType               `json:"plan_type,omitempty"`
	InvoiceID       string                 `json:"invoice_id,omitempty"`
	PaymentIntentID string                 `json:"payment_intent_id,omitempty"`
	Amount          int64                  `json:"amount,omitempty"`
	Currency        string                 `json:"currency,omitempty"`
	PeriodStart     time.Time              `json:"period_start,omitempty"`
	PeriodEnd       time.Time              `json:"period_end,omitempty"`
	CancelAt        *time.Time             `json:"cancel_at,omitempty"`
	Metadata        map[string]string      `json:"metadata,omitempty"`
	RawData         map[string]interface{} `json:"raw_data,omitempty"`
}

// CheckoutSessionResult represents the result of creating a checkout session
type CheckoutSessionResult struct {
	SessionID  string `json:"session_id"`
	URL        string `json:"url"`
	CustomerID string `json:"customer_id"`
}

// PortalSessionResult represents the result of creating a billing portal session
type PortalSessionResult struct {
	URL string `json:"url"`
}

// SubscriptionInfo represents detailed subscription information
type SubscriptionInfo struct {
	ID                   string             `json:"id"`
	CustomerID           string             `json:"customer_id"`
	Status               SubscriptionStatus `json:"status"`
	PlanType             PlanType           `json:"plan_type"`
	PriceID              string             `json:"price_id"`
	CurrentPeriodStart   time.Time          `json:"current_period_start"`
	CurrentPeriodEnd     time.Time          `json:"current_period_end"`
	CancelAtPeriodEnd    bool               `json:"cancel_at_period_end"`
	CancelAt             *time.Time         `json:"cancel_at,omitempty"`
	CanceledAt           *time.Time         `json:"canceled_at,omitempty"`
	TrialStart           *time.Time         `json:"trial_start,omitempty"`
	TrialEnd             *time.Time         `json:"trial_end,omitempty"`
	Metadata             map[string]string  `json:"metadata,omitempty"`
	LatestInvoiceID      string             `json:"latest_invoice_id,omitempty"`
	DefaultPaymentMethod string             `json:"default_payment_method,omitempty"`
}

// CustomerInfo represents Stripe customer information
type CustomerInfo struct {
	ID           string            `json:"id"`
	Email        string            `json:"email"`
	Name         string            `json:"name"`
	Phone        string            `json:"phone,omitempty"`
	Created      time.Time         `json:"created"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	DefaultSource string           `json:"default_source,omitempty"`
}

// InvoiceInfo represents invoice information
type InvoiceInfo struct {
	ID             string    `json:"id"`
	CustomerID     string    `json:"customer_id"`
	SubscriptionID string    `json:"subscription_id,omitempty"`
	Status         string    `json:"status"`
	AmountDue      int64     `json:"amount_due"`
	AmountPaid     int64     `json:"amount_paid"`
	Currency       string    `json:"currency"`
	DueDate        *time.Time `json:"due_date,omitempty"`
	Created        time.Time `json:"created"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	HostedInvoiceURL string  `json:"hosted_invoice_url,omitempty"`
	InvoicePDF     string    `json:"invoice_pdf,omitempty"`
}

// PaymentMethodInfo represents payment method information
type PaymentMethodInfo struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	CardBrand string `json:"card_brand,omitempty"`
	CardLast4 string `json:"card_last4,omitempty"`
	CardExpMonth int64 `json:"card_exp_month,omitempty"`
	CardExpYear int64 `json:"card_exp_year,omitempty"`
	IsDefault bool   `json:"is_default"`
}

// NewStripeService creates a new Stripe service instance
func NewStripeService(secretKey string) *StripeService {
	if secretKey == "" {
		secretKey = os.Getenv("STRIPE_SECRET_KEY")
	}

	// Set the Stripe API key globally
	stripe.Key = secretKey

	webhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")

	return &StripeService{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
		planConfig:    LoadPlanConfig(),
	}
}

// IsConfigured returns true if Stripe is properly configured
func (s *StripeService) IsConfigured() bool {
	return s.secretKey != "" && s.secretKey != "sk_test_xxx"
}

// CreateCustomer creates a new Stripe customer
func (s *StripeService) CreateCustomer(ctx context.Context, email, name string, metadata map[string]string) (*CustomerInfo, error) {
	if !s.IsConfigured() {
		return nil, errors.New("stripe is not configured")
	}

	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
	}

	if metadata != nil {
		params.Metadata = metadata
	}

	c, err := customer.New(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create customer: %w", err)
	}

	return &CustomerInfo{
		ID:       c.ID,
		Email:    c.Email,
		Name:     c.Name,
		Phone:    c.Phone,
		Created:  time.Unix(c.Created, 0),
		Metadata: c.Metadata,
	}, nil
}

// GetCustomer retrieves a Stripe customer by ID
func (s *StripeService) GetCustomer(ctx context.Context, customerID string) (*CustomerInfo, error) {
	if !s.IsConfigured() {
		return nil, errors.New("stripe is not configured")
	}

	c, err := customer.Get(customerID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get customer: %w", err)
	}

	return &CustomerInfo{
		ID:       c.ID,
		Email:    c.Email,
		Name:     c.Name,
		Phone:    c.Phone,
		Created:  time.Unix(c.Created, 0),
		Metadata: c.Metadata,
	}, nil
}

// UpdateCustomer updates a Stripe customer
func (s *StripeService) UpdateCustomer(ctx context.Context, customerID, email, name string, metadata map[string]string) (*CustomerInfo, error) {
	if !s.IsConfigured() {
		return nil, errors.New("stripe is not configured")
	}

	params := &stripe.CustomerParams{}
	if email != "" {
		params.Email = stripe.String(email)
	}
	if name != "" {
		params.Name = stripe.String(name)
	}
	if metadata != nil {
		params.Metadata = metadata
	}

	c, err := customer.Update(customerID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to update customer: %w", err)
	}

	return &CustomerInfo{
		ID:       c.ID,
		Email:    c.Email,
		Name:     c.Name,
		Phone:    c.Phone,
		Created:  time.Unix(c.Created, 0),
		Metadata: c.Metadata,
	}, nil
}

// CreateCheckoutSession creates a Stripe checkout session for subscription
func (s *StripeService) CreateCheckoutSession(ctx context.Context, customerID, priceID, successURL, cancelURL string, metadata map[string]string) (*CheckoutSessionResult, error) {
	if !s.IsConfigured() {
		return nil, errors.New("stripe is not configured")
	}

	if priceID == "" {
		return nil, ErrInvalidPriceID
	}

	// Validate price ID corresponds to a valid plan
	plan := GetPlanByPriceID(priceID)
	if plan == nil {
		return nil, ErrInvalidPriceID
	}

	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: metadata,
		},
	}

	// If customer exists, use their ID
	if customerID != "" {
		params.Customer = stripe.String(customerID)
	} else {
		// Allow Stripe to create customer from email
		params.CustomerCreation = stripe.String("always")
	}

	// Add trial period if plan has one
	if plan.TrialDays > 0 {
		params.SubscriptionData.TrialPeriodDays = stripe.Int64(int64(plan.TrialDays))
	}

	// Add metadata
	if metadata != nil {
		params.Metadata = metadata
	}

	// Allow promotion codes
	params.AllowPromotionCodes = stripe.Bool(true)

	// Billing address collection
	params.BillingAddressCollection = stripe.String("auto")

	sess, err := checkoutsession.New(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create checkout session: %w", err)
	}

	return &CheckoutSessionResult{
		SessionID:  sess.ID,
		URL:        sess.URL,
		CustomerID: string(sess.Customer.ID),
	}, nil
}

// CreateBillingPortalSession creates a Stripe billing portal session
func (s *StripeService) CreateBillingPortalSession(ctx context.Context, customerID, returnURL string) (*PortalSessionResult, error) {
	if !s.IsConfigured() {
		return nil, errors.New("stripe is not configured")
	}

	if customerID == "" {
		return nil, ErrCustomerNotFound
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(returnURL),
	}

	sess, err := session.New(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create billing portal session: %w", err)
	}

	return &PortalSessionResult{
		URL: sess.URL,
	}, nil
}

// GetSubscription retrieves a subscription by ID
func (s *StripeService) GetSubscription(ctx context.Context, subscriptionID string) (*SubscriptionInfo, error) {
	if !s.IsConfigured() {
		return nil, errors.New("stripe is not configured")
	}

	if subscriptionID == "" {
		return nil, ErrSubscriptionNotFound
	}

	sub, err := subscription.Get(subscriptionID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	return s.subscriptionToInfo(sub), nil
}

// GetSubscriptionByCustomer retrieves the active subscription for a customer
func (s *StripeService) GetSubscriptionByCustomer(ctx context.Context, customerID string) (*SubscriptionInfo, error) {
	if !s.IsConfigured() {
		return nil, errors.New("stripe is not configured")
	}

	if customerID == "" {
		return nil, ErrCustomerNotFound
	}

	params := &stripe.SubscriptionListParams{
		Customer: stripe.String(customerID),
		Status:   stripe.String("all"),
	}
	params.Limit = stripe.Int64(1)

	iter := subscription.List(params)
	for iter.Next() {
		sub := iter.Subscription()
		return s.subscriptionToInfo(sub), nil
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to list subscriptions: %w", err)
	}

	return nil, ErrSubscriptionNotFound
}

// CancelSubscription cancels a subscription at period end
func (s *StripeService) CancelSubscription(ctx context.Context, subscriptionID string) (*SubscriptionInfo, error) {
	if !s.IsConfigured() {
		return nil, errors.New("stripe is not configured")
	}

	if subscriptionID == "" {
		return nil, ErrSubscriptionNotFound
	}

	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(true),
	}

	sub, err := subscription.Update(subscriptionID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel subscription: %w", err)
	}

	return s.subscriptionToInfo(sub), nil
}

// CancelSubscriptionImmediately cancels a subscription immediately
func (s *StripeService) CancelSubscriptionImmediately(ctx context.Context, subscriptionID string) (*SubscriptionInfo, error) {
	if !s.IsConfigured() {
		return nil, errors.New("stripe is not configured")
	}

	if subscriptionID == "" {
		return nil, ErrSubscriptionNotFound
	}

	sub, err := subscription.Cancel(subscriptionID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel subscription: %w", err)
	}

	return s.subscriptionToInfo(sub), nil
}

// ReactivateSubscription removes the cancellation of a subscription
func (s *StripeService) ReactivateSubscription(ctx context.Context, subscriptionID string) (*SubscriptionInfo, error) {
	if !s.IsConfigured() {
		return nil, errors.New("stripe is not configured")
	}

	if subscriptionID == "" {
		return nil, ErrSubscriptionNotFound
	}

	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(false),
	}

	sub, err := subscription.Update(subscriptionID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to reactivate subscription: %w", err)
	}

	return s.subscriptionToInfo(sub), nil
}

// UpdateSubscription updates a subscription to a new price/plan
func (s *StripeService) UpdateSubscription(ctx context.Context, subscriptionID, newPriceID string) (*SubscriptionInfo, error) {
	if !s.IsConfigured() {
		return nil, errors.New("stripe is not configured")
	}

	if subscriptionID == "" {
		return nil, ErrSubscriptionNotFound
	}

	if newPriceID == "" {
		return nil, ErrInvalidPriceID
	}

	// Get current subscription to find the item ID
	sub, err := subscription.Get(subscriptionID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	if len(sub.Items.Data) == 0 {
		return nil, errors.New("subscription has no items")
	}

	itemID := sub.Items.Data[0].ID

	params := &stripe.SubscriptionParams{
		Items: []*stripe.SubscriptionItemsParams{
			{
				ID:    stripe.String(itemID),
				Price: stripe.String(newPriceID),
			},
		},
		ProrationBehavior: stripe.String("create_prorations"),
	}

	updatedSub, err := subscription.Update(subscriptionID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to update subscription: %w", err)
	}

	return s.subscriptionToInfo(updatedSub), nil
}

// HandleWebhook processes Stripe webhook events
func (s *StripeService) HandleWebhook(payload []byte, signature string) (*WebhookEvent, error) {
	// Verify webhook signature if secret is configured
	var event stripe.Event
	var err error

	if s.webhookSecret != "" {
		event, err = webhook.ConstructEvent(payload, signature, s.webhookSecret)
		if err != nil {
			log.Printf("Webhook signature verification failed: %v", err)
			return nil, ErrInvalidWebhook
		}
	} else {
		// For development without webhook secret
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, fmt.Errorf("failed to parse webhook: %w", err)
		}
	}

	log.Printf("Processing Stripe webhook: %s", event.Type)

	webhookEvent := &WebhookEvent{
		Type: string(event.Type),
	}

	// Process different event types
	switch event.Type {
	case "checkout.session.completed":
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			return nil, fmt.Errorf("failed to parse checkout session: %w", err)
		}
		webhookEvent.CustomerID = string(session.Customer.ID)
		webhookEvent.SubscriptionID = string(session.Subscription.ID)
		webhookEvent.Metadata = session.Metadata

	case "customer.subscription.created", "customer.subscription.updated":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return nil, fmt.Errorf("failed to parse subscription: %w", err)
		}
		webhookEvent.CustomerID = sub.Customer.ID
		webhookEvent.SubscriptionID = sub.ID
		webhookEvent.Status = mapStripeStatus(sub.Status)
		webhookEvent.PeriodStart = time.Unix(sub.CurrentPeriodStart, 0)
		webhookEvent.PeriodEnd = time.Unix(sub.CurrentPeriodEnd, 0)
		webhookEvent.Metadata = sub.Metadata

		if len(sub.Items.Data) > 0 {
			webhookEvent.PriceID = sub.Items.Data[0].Price.ID
			webhookEvent.PlanType = GetPlanTypeByPriceID(sub.Items.Data[0].Price.ID)
		}

		if sub.CancelAt > 0 {
			cancelAt := time.Unix(sub.CancelAt, 0)
			webhookEvent.CancelAt = &cancelAt
		}

	case "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return nil, fmt.Errorf("failed to parse subscription: %w", err)
		}
		webhookEvent.CustomerID = sub.Customer.ID
		webhookEvent.SubscriptionID = sub.ID
		webhookEvent.Status = StatusCanceled
		webhookEvent.Metadata = sub.Metadata

	case "invoice.paid":
		var inv stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
			return nil, fmt.Errorf("failed to parse invoice: %w", err)
		}
		webhookEvent.CustomerID = inv.Customer.ID
		webhookEvent.InvoiceID = inv.ID
		webhookEvent.SubscriptionID = string(inv.Subscription.ID)
		webhookEvent.Amount = inv.AmountPaid
		webhookEvent.Currency = string(inv.Currency)

	case "invoice.payment_failed":
		var inv stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
			return nil, fmt.Errorf("failed to parse invoice: %w", err)
		}
		webhookEvent.CustomerID = inv.Customer.ID
		webhookEvent.InvoiceID = inv.ID
		webhookEvent.SubscriptionID = string(inv.Subscription.ID)
		webhookEvent.Amount = inv.AmountDue
		webhookEvent.Currency = string(inv.Currency)
		webhookEvent.Status = StatusPastDue

	case "customer.created", "customer.updated":
		var cust stripe.Customer
		if err := json.Unmarshal(event.Data.Raw, &cust); err != nil {
			return nil, fmt.Errorf("failed to parse customer: %w", err)
		}
		webhookEvent.CustomerID = cust.ID
		webhookEvent.Metadata = cust.Metadata

	case "payment_intent.succeeded":
		var pi stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
			return nil, fmt.Errorf("failed to parse payment intent: %w", err)
		}
		webhookEvent.CustomerID = string(pi.Customer.ID)
		webhookEvent.PaymentIntentID = pi.ID
		webhookEvent.Amount = pi.Amount
		webhookEvent.Currency = string(pi.Currency)

	case "payment_intent.payment_failed":
		var pi stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
			return nil, fmt.Errorf("failed to parse payment intent: %w", err)
		}
		webhookEvent.CustomerID = string(pi.Customer.ID)
		webhookEvent.PaymentIntentID = pi.ID
		webhookEvent.Amount = pi.Amount
		webhookEvent.Currency = string(pi.Currency)
	}

	// Store raw data for debugging
	var rawData map[string]interface{}
	if err := json.Unmarshal(event.Data.Raw, &rawData); err == nil {
		webhookEvent.RawData = rawData
	}

	return webhookEvent, nil
}

// GetInvoices retrieves invoices for a customer
func (s *StripeService) GetInvoices(ctx context.Context, customerID string, limit int64) ([]*InvoiceInfo, error) {
	if !s.IsConfigured() {
		return nil, errors.New("stripe is not configured")
	}

	if customerID == "" {
		return nil, ErrCustomerNotFound
	}

	if limit <= 0 || limit > 100 {
		limit = 10
	}

	params := &stripe.InvoiceListParams{
		Customer: stripe.String(customerID),
	}
	params.Limit = stripe.Int64(limit)

	var invoices []*InvoiceInfo
	iter := invoice.List(params)
	for iter.Next() {
		inv := iter.Invoice()
		info := &InvoiceInfo{
			ID:               inv.ID,
			CustomerID:       inv.Customer.ID,
			Status:           string(inv.Status),
			AmountDue:        inv.AmountDue,
			AmountPaid:       inv.AmountPaid,
			Currency:         string(inv.Currency),
			Created:          time.Unix(inv.Created, 0),
			HostedInvoiceURL: inv.HostedInvoiceURL,
			InvoicePDF:       inv.InvoicePDF,
		}

		if inv.Subscription != nil {
			info.SubscriptionID = inv.Subscription.ID
		}

		if inv.DueDate > 0 {
			dueDate := time.Unix(inv.DueDate, 0)
			info.DueDate = &dueDate
		}

		if inv.StatusTransitions != nil && inv.StatusTransitions.PaidAt > 0 {
			paidAt := time.Unix(inv.StatusTransitions.PaidAt, 0)
			info.PaidAt = &paidAt
		}

		invoices = append(invoices, info)
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to list invoices: %w", err)
	}

	return invoices, nil
}

// GetPaymentMethods retrieves payment methods for a customer
func (s *StripeService) GetPaymentMethods(ctx context.Context, customerID string) ([]*PaymentMethodInfo, error) {
	if !s.IsConfigured() {
		return nil, errors.New("stripe is not configured")
	}

	if customerID == "" {
		return nil, ErrCustomerNotFound
	}

	// Get the customer to find the default payment method
	cust, err := customer.Get(customerID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get customer: %w", err)
	}

	defaultPM := ""
	if cust.InvoiceSettings != nil && cust.InvoiceSettings.DefaultPaymentMethod != nil {
		defaultPM = cust.InvoiceSettings.DefaultPaymentMethod.ID
	}

	params := &stripe.PaymentMethodListParams{
		Customer: stripe.String(customerID),
		Type:     stripe.String("card"),
	}

	var methods []*PaymentMethodInfo
	iter := paymentmethod.List(params)
	for iter.Next() {
		pm := iter.PaymentMethod()
		info := &PaymentMethodInfo{
			ID:        pm.ID,
			Type:      string(pm.Type),
			IsDefault: pm.ID == defaultPM,
		}

		if pm.Card != nil {
			info.CardBrand = string(pm.Card.Brand)
			info.CardLast4 = pm.Card.Last4
			info.CardExpMonth = pm.Card.ExpMonth
			info.CardExpYear = pm.Card.ExpYear
		}

		methods = append(methods, info)
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to list payment methods: %w", err)
	}

	return methods, nil
}

// Helper function to convert Stripe subscription to our info struct
func (s *StripeService) subscriptionToInfo(sub *stripe.Subscription) *SubscriptionInfo {
	info := &SubscriptionInfo{
		ID:                 sub.ID,
		CustomerID:         sub.Customer.ID,
		Status:             mapStripeStatus(sub.Status),
		CurrentPeriodStart: time.Unix(sub.CurrentPeriodStart, 0),
		CurrentPeriodEnd:   time.Unix(sub.CurrentPeriodEnd, 0),
		CancelAtPeriodEnd:  sub.CancelAtPeriodEnd,
		Metadata:           sub.Metadata,
	}

	if len(sub.Items.Data) > 0 {
		info.PriceID = sub.Items.Data[0].Price.ID
		info.PlanType = GetPlanTypeByPriceID(info.PriceID)
	}

	if sub.CancelAt > 0 {
		cancelAt := time.Unix(sub.CancelAt, 0)
		info.CancelAt = &cancelAt
	}

	if sub.CanceledAt > 0 {
		canceledAt := time.Unix(sub.CanceledAt, 0)
		info.CanceledAt = &canceledAt
	}

	if sub.TrialStart > 0 {
		trialStart := time.Unix(sub.TrialStart, 0)
		info.TrialStart = &trialStart
	}

	if sub.TrialEnd > 0 {
		trialEnd := time.Unix(sub.TrialEnd, 0)
		info.TrialEnd = &trialEnd
	}

	if sub.LatestInvoice != nil {
		info.LatestInvoiceID = sub.LatestInvoice.ID
	}

	if sub.DefaultPaymentMethod != nil {
		info.DefaultPaymentMethod = sub.DefaultPaymentMethod.ID
	}

	return info
}

// mapStripeStatus maps Stripe subscription status to our status type
func mapStripeStatus(status stripe.SubscriptionStatus) SubscriptionStatus {
	switch status {
	case stripe.SubscriptionStatusActive:
		return StatusActive
	case stripe.SubscriptionStatusCanceled:
		return StatusCanceled
	case stripe.SubscriptionStatusPastDue:
		return StatusPastDue
	case stripe.SubscriptionStatusTrialing:
		return StatusTrialing
	case stripe.SubscriptionStatusIncomplete, stripe.SubscriptionStatusIncompleteExpired, stripe.SubscriptionStatusUnpaid, stripe.SubscriptionStatusPaused:
		return StatusInactive
	default:
		return StatusInactive
	}
}
