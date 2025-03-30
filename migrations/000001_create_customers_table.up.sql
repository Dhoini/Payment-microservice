CREATE TABLE IF NOT EXISTS customers (
                                         id UUID PRIMARY KEY,
                                         email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255),
    phone VARCHAR(50),
    external_id VARCHAR(255),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL
                             );

CREATE INDEX idx_customers_email ON customers(email);
CREATE INDEX idx_customers_external_id ON customers(external_id);