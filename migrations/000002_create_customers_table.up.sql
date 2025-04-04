BEGIN;
CREATE TABLE IF NOT EXISTS customers (
user_id VARCHAR(255) PRIMARY KEY,
stripe_customer_id VARCHAR(255) UNIQUE NOT NULL,
email VARCHAR(255) NOT NULL,
created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_customers_stripe_customer_id ON customers(stripe_customer_id);
CREATE INDEX idx_customers_email ON customers(email);

COMMIT;