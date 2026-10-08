CREATE TABLE IF NOT EXISTS stock (
    sku      text PRIMARY KEY,
    quantity int NOT NULL CHECK (quantity >= 0)
);
