INSERT INTO stock (sku, quantity) VALUES
    ('SKU-001', 1000000),
    ('SKU-002', 1000000),
    ('SKU-003', 1000000),
    ('SKU-LOW', 3)
ON CONFLICT (sku) DO NOTHING;
