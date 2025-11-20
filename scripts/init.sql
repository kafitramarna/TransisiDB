-- Initialize test database schema
CREATE TABLE IF NOT EXISTS orders (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    customer_id BIGINT NOT NULL,
    total_amount BIGINT NOT NULL COMMENT 'Amount in IDR (old format)',
    total_amount_idn DECIMAL(19,4) DEFAULT NULL COMMENT 'Amount in IDN (new format)',
    shipping_fee INT NOT NULL,
    shipping_fee_idn DECIMAL(12,4) DEFAULT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_customer_id (customer_id),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS invoices (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    order_id BIGINT NOT NULL,
    grand_total BIGINT NOT NULL,
    grand_total_idn DECIMAL(19,4) DEFAULT NULL,
    tax_amount BIGINT NOT NULL,
    tax_amount_idn DECIMAL(19,4) DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (order_id) REFERENCES orders(id),
    INDEX idx_order_id (order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Insert sample data
INSERT INTO orders (customer_id, total_amount, shipping_fee, status) VALUES
(1001, 500000, 25000, 'completed'),
(1002, 1250000, 30000, 'completed'),
(1003, 750000, 20000, 'pending'),
(1004, 2000000, 50000, 'shipped'),
(1005, 350000, 15000, 'completed');

INSERT INTO invoices (order_id, grand_total, tax_amount) VALUES
(1, 525000, 52500),
(2, 1280000, 128000),
(4, 2050000, 205000);
