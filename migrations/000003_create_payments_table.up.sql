CREATE TABLE IF NOT EXISTS payments (
                                        id UUID PRIMARY KEY,
                                        customer_id UUID NOT NULL REFERENCES customers(id),
    amount DECIMAL(12, 2) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    description TEXT,
    status VARCHAR(20) NOT NULL,
    method_id UUID,
    method_type VARCHAR(50),
    transaction_id VARCHAR(255),
    receipt_url TEXT,
    error_message TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL
                             );

CREATE INDEX idx_payments_customer_id ON payments(customer_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_created_at ON payments(created_at);