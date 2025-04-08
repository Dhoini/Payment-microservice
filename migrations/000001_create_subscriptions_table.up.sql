BEGIN;

CREATE TABLE IF NOT EXISTS subscriptions (
    subscription_id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    plan_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    stripe_customer_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NULL,
    canceled_at TIMESTAMPTZ NULL
    );

-- Индексы для ускорения поиска
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_stripe_customer_id ON subscriptions(stripe_customer_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_status ON subscriptions(status);

COMMENT ON TABLE subscriptions IS 'Stores subscription details synced from Stripe';
COMMENT ON COLUMN subscriptions.subscription_id IS 'Primary key, usually the Stripe Subscription ID (sub_...)';
COMMENT ON COLUMN subscriptions.user_id IS 'Foreign key relating to the user in the User Management Service';
COMMENT ON COLUMN subscriptions.plan_id IS 'Stripe Price ID (price_...) associated with the subscription item';
COMMENT ON COLUMN subscriptions.status IS 'Current status (e.g., active, incomplete, past_due, canceled)';
COMMENT ON COLUMN subscriptions.stripe_customer_id IS 'Stripe Customer ID (cus_...)';
COMMENT ON COLUMN subscriptions.expires_at IS 'Timestamp when the current billing period ends (or subscription expires)';
COMMENT ON COLUMN subscriptions.canceled_at IS 'Timestamp when the subscription was canceled in Stripe';


COMMIT;