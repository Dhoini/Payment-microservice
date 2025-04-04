BEGIN;

DROP TABLE IF EXISTS customers;
DROP INDEX IF EXISTS idx_customers_stripe_customer_id;
DROP INDEX IF EXISTS idx_customers_email;

COMMIT;