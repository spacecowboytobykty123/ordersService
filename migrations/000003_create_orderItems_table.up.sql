CREATE TABLE IF NOT EXISTS order_items (
    id SERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    toy_id BIGINT NOT NULL,
    toy_name varchar(200) NOT NULL,
    quantity INT NOT NULL DEFAULT 1
)