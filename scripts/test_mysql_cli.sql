-- Test 3: MySQL CLI Manual Testing
-- Connect via: mysql -h 127.0.0.1 -P 3308 -u root -psecret ecommerce_db

-- ==========================================
-- Test 1: Dual-Write INSERT
-- ==========================================
SELECT '=== Test 1: Dual-Write INSERT ===' AS '';

INSERT INTO orders (id, customer_id, total_amount, shipping_fee) 
VALUES (9999, 8888, 50000000, 15000);

SELECT id, customer_id, total_amount, total_amount_idn, shipping_fee, shipping_fee_idn 
FROM orders WHERE id = 9999;

-- Expected: total_amount_idn = 50000.0000, shipping_fee_idn = 15.0000

-- ==========================================
-- Test 2: Dual-Write UPDATE
-- ==========================================
SELECT '=== Test 2: Dual-Write UPDATE ===' AS '';

UPDATE orders SET total_amount = 75000000 WHERE id = 9999;

SELECT id, total_amount, total_amount_idn, shipping_fee, shipping_fee_idn 
FROM orders WHERE id = 9999;

-- Expected: total_amount_idn = 75000.0000

-- ==========================================
-- Test 3: Transaction COMMIT
-- ==========================================
SELECT '=== Test 3: Transaction COMMIT ===' AS '';

BEGIN;
INSERT INTO orders (id, customer_id, total_amount, shipping_fee) 
VALUES (9998, 8887, 25000000, 10000);
SELECT id, total_amount, total_amount_idn FROM orders WHERE id = 9998;
COMMIT;

-- Verify data persisted after commit
SELECT id, total_amount, total_amount_idn FROM orders WHERE id = 9998;

-- Expected: Row should exist with total_amount_idn = 25000.0000

-- ==========================================
-- Test 4: Transaction ROLLBACK
-- ==========================================
SELECT '=== Test 4: Transaction ROLLBACK ===' AS '';

BEGIN;
INSERT INTO orders (id, customer_id, total_amount, shipping_fee) 
VALUES (9997, 8886, 30000000, 12000);
SELECT 'Row should be visible within transaction:' AS '';
SELECT COUNT(*) as count_in_tx FROM orders WHERE id = 9997;
ROLLBACK;

SELECT 'Row should NOT exist after rollback:' AS '';
SELECT COUNT(*) as count_after_rollback FROM orders WHERE id = 9997;

-- Expected: count_after_rollback = 0

-- ==========================================
-- Test 5: Banker's Rounding
-- ==========================================
SELECT '=== Test 5: Bankers Rounding ===' AS '';

-- Test edge case: 15500 / 1000 = 15.5 (should round to 15.5000)
INSERT INTO orders (id, customer_id, total_amount, shipping_fee) 
VALUES (9001, 8801, 15500, 10000);

-- Test edge case: 16500 / 1000 = 16.5 (should round to 16.5000)
INSERT INTO orders (id, customer_id, total_amount, shipping_fee) 
VALUES (9002, 8802, 16500, 10000);

SELECT id, total_amount, total_amount_idn, shipping_fee, shipping_fee_idn 
FROM orders 
WHERE id IN (9001, 9002)
ORDER BY id;

-- Expected:
-- 9001: 15500 -> 15.5000
-- 9002: 16500 -> 16.5000

-- ==========================================
-- Cleanup
-- ==========================================
SELECT '=== Cleanup Test Data ===' AS '';

DELETE FROM orders WHERE id >= 9000;

SELECT 'Cleanup complete!' AS '';
