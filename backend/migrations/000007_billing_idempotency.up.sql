-- Stripe webhook idempotency: one row per Stripe event ID, unique constraint
-- prevents duplicate processing even under concurrent retries.
CREATE TABLE IF NOT EXISTS processed_stripe_events (
    id             BIGSERIAL PRIMARY KEY,
    stripe_event_id VARCHAR(64)  NOT NULL,
    event_type      VARCHAR(64)  NOT NULL,
    user_id         BIGINT,
    customer_id     VARCHAR(64),
    processed_at    TIMESTAMPTZ  NOT NULL,
    CONSTRAINT uq_processed_stripe_events_id UNIQUE (stripe_event_id)
);

CREATE INDEX IF NOT EXISTS idx_processed_stripe_events_user_id     ON processed_stripe_events (user_id);
CREATE INDEX IF NOT EXISTS idx_processed_stripe_events_customer_id ON processed_stripe_events (customer_id);
CREATE INDEX IF NOT EXISTS idx_processed_stripe_events_event_type  ON processed_stripe_events (event_type);
CREATE INDEX IF NOT EXISTS idx_processed_stripe_events_processed_at ON processed_stripe_events (processed_at);

-- Immutable credit ledger: one row per credit change, never updated or deleted.
-- Positive amount_usd = credit added; negative = debit.
-- balance_after_usd snapshots the running balance at write time for fast point-in-time queries.
CREATE TABLE IF NOT EXISTS credit_ledger_entries (
    id                BIGSERIAL    PRIMARY KEY,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    user_id           BIGINT       NOT NULL,
    amount_usd        NUMERIC(12,6) NOT NULL,
    balance_after_usd NUMERIC(12,6) NOT NULL,
    entry_type        VARCHAR(32)  NOT NULL,  -- monthly_allocation | credit_purchase | admin_grant | spend_deduction | refund
    description       VARCHAR(255),
    stripe_event_id   VARCHAR(64),
    stripe_invoice_id VARCHAR(64),
    plan_type         VARCHAR(20)
);

CREATE INDEX IF NOT EXISTS idx_credit_ledger_user_id          ON credit_ledger_entries (user_id);
CREATE INDEX IF NOT EXISTS idx_credit_ledger_created_at       ON credit_ledger_entries (created_at);
CREATE INDEX IF NOT EXISTS idx_credit_ledger_entry_type       ON credit_ledger_entries (entry_type);
CREATE INDEX IF NOT EXISTS idx_credit_ledger_stripe_event_id  ON credit_ledger_entries (stripe_event_id);
CREATE INDEX IF NOT EXISTS idx_credit_ledger_stripe_invoice   ON credit_ledger_entries (stripe_invoice_id);
