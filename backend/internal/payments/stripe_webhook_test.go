package payments

import (
	"errors"
	"testing"
)

const customerCreatedWebhookPayload = `{
  "id": "evt_test_customer_created",
  "type": "customer.created",
  "data": {
    "object": {
      "id": "cus_test_123",
      "metadata": {
        "user_id": "42"
      }
    }
  }
}`

func TestHandleWebhookRejectsUnsignedProductionWebhookWithoutSecret(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("GO_ENV", "")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "")

	service := NewStripeService("sk_test_valid_for_webhook_unit")
	event, err := service.HandleWebhook([]byte(customerCreatedWebhookPayload), "")
	if !errors.Is(err, ErrInvalidWebhook) {
		t.Fatalf("HandleWebhook() err = %v, want ErrInvalidWebhook", err)
	}
	if event != nil {
		t.Fatalf("HandleWebhook() event = %+v, want nil", event)
	}
}

func TestHandleWebhookAllowsUnsignedDevelopmentWebhookWithoutSecret(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("GO_ENV", "")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "")

	service := NewStripeService("sk_test_valid_for_webhook_unit")
	event, err := service.HandleWebhook([]byte(customerCreatedWebhookPayload), "")
	if err != nil {
		t.Fatalf("HandleWebhook() err = %v, want nil", err)
	}
	if event == nil || event.Type != "customer.created" || event.CustomerID != "cus_test_123" {
		t.Fatalf("HandleWebhook() event = %+v, want parsed customer event", event)
	}
}

func TestHandleWebhookDerivesInvoicePlanFromLinePrice(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("GO_ENV", "")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "")
	t.Setenv("STRIPE_PRICE_PRO_MONTHLY", "price_pro_live_invoice")

	payload := `{
	  "id": "evt_test_invoice_paid",
	  "type": "invoice.paid",
	  "data": {
	    "object": {
	      "id": "in_test_123",
	      "customer": "cus_invoice_123",
	      "subscription": "sub_invoice_123",
	      "amount_paid": 5900,
	      "currency": "usd",
	      "lines": {
	        "data": [
	          {
	            "price": {
	              "id": "price_pro_live_invoice"
	            }
	          }
	        ]
	      }
	    }
	  }
	}`

	service := NewStripeService("sk_test_valid_for_webhook_unit")
	event, err := service.HandleWebhook([]byte(payload), "")
	if err != nil {
		t.Fatalf("HandleWebhook() err = %v, want nil", err)
	}
	if event == nil || event.PriceID != "price_pro_live_invoice" || event.PlanType != PlanPro {
		t.Fatalf("HandleWebhook() event = %+v, want invoice pro plan", event)
	}
}
