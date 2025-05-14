CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL CHECK ( user_id > 0 ),
    address VARCHAR(255) NOT NULL,
    status order_status NOT NULL DEFAULT 'STATUS_PROCESSING',
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);