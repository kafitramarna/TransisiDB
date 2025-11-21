# TransisiDB Testing Guide

Complete guide for testing all components of TransisiDB.

---

## Testing Strategy

TransisiDB has multiple test suites:

1. **Unit Tests** - Go test suite (`go test ./...`)
2. **Integration Tests** - Automated proxy testing
3. **Manual Tests** - SQL commands through proxy
4. **Circuit Breaker Tests** - Fault injection testing
5. **Metrics Tests** - Monitoring validation
6. **Load Tests** - Performance testing (optional)

---

## Prerequisites

- Docker Desktop running
- Go 1.21+ installed
- MySQL client (or use docker exec)
- Terminal/PowerShell

---

## Quick Start

```bash
# 1. Start infrastructure
docker-compose up -d mysql redis

# 2. Start proxy
go run cmd/proxy/main.go

#3. Run all integration tests
go run cmd/test_proxy/main.go
```

---

## Test 1: Automated Integration Tests

### Run All Tests

```bash
cd TransisiDB
go run cmd/test_proxy/main.go
```

### Expected Output

```
=== TransisiDB Proxy Integration Tests ===

✓ Test 1: Basic Connectivity (41ms)
✓ Test 2: Dual-Write INSERT (125ms)
✓ Test 3: Dual-Write UPDATE (98ms)
✓ Test 4: Transaction Handling (156ms)
✓ Test 5: Banker's Rounding (112ms)
✓ Test 6: COM_PING (35ms)
✓ Test 7: Database Switching (67ms)

=== All Tests Passed! (7/7) ===
```

### Test Details

#### Test 1: Basic Connectivity
- Connects to proxy on port 3308
- Verifies MySQL protocol handshake
- Checks authentication

#### Test 2: Dual-Write INSERT
- Inserts record with currency fields
- Verifies shadow columns populated
- Validates conversion accuracy (15M → 15K)

#### Test 3: Dual-Write UPDATE
- Updates currency field
- Verifies shadow column updated
- Validates conversion accuracy (25M → 25K)

#### Test 4: Transaction Handling
- Tests COMMIT behavior
- Tests ROLLBACK behavior
- Verifies ACID compliance

#### Test 5: Banker's Rounding
- Tests 15500 → 15.5000
- Tests 16500 → 16.5000
- Validates IEEE 754 rounding

#### Test 6: COM_PING
- Tests connection health check
- Measures latency (~41ms)

#### Test 7: Database Switching
- Tests `USE database` command
- Verifies context switching

---

## Test 2: Database Viewer

### CLI Viewer

```bash
go run cmd/view_rows/main.go
```

**Output:**
```
=== TransisiDB Data Viewer ===

📦 ORDERS TABLE:
ID     Customer  Total (IDR)     Total (IDN)      Ship (IDR)   Ship (IDN)      Status
1      1001      500000          NULL             25000        NULL            completed
1001   5001      25000000        25000.0000       50000        50.0000         pending

🧾 INVOICES TABLE:
ID     Order ID  Grand Total (IDR)  Grand Total (IDN)   Tax (IDR)    Tax (IDN)
1      1         525000             NULL                52500        NULL

📊 DUAL-WRITE STATUS SUMMARY:
Orders:   12 total, 7 with IDN values (58.3% converted)
Invoices: 3 total, 0 with IDN values (0.0% converted)
```

### Web Dashboard

```bash
# Open in browser
start cmd/view_rows/dashboard.html
```

Features:
- Visual statistics with charts
- Color-coded IDR (red) vs IDN (green)
- Highlighted converted rows
- Refresh button

---

## Test 3: Manual SQL Testing

### Run Automated Manual Tests

```bash
go run cmd/test_manual/main.go
```

**Tests:**
1. Dual-Write INSERT
2. Dual-Write UPDATE
3. Transaction COMMIT
4. Transaction ROLLBACK
5. Banker's Rounding

### Manual MySQL CLI Testing

**Connect through proxy:**
```bash
mysql -h 127.0.0.1 -P 3308 -u root -psecret ecommerce_db
```

**Test INSERT:**
```sql
INSERT INTO orders (id, customer_id, total_amount, shipping_fee) 
VALUES (9999, 8888, 50000000, 15000);

SELECT id, total_amount, total_amount_idn, shipping_fee, shipping_fee_idn 
FROM orders WHERE id = 9999;
```

**Expected:**
```
+------+--------------+-------------------+--------------+------------------+
| id   | total_amount | total_amount_idn  | shipping_fee | shipping_fee_idn |
+------+--------------+-------------------+--------------+------------------+
| 9999 |     50000000 |        50000.0000 |        15000 |          15.0000 |
+------+--------------+-------------------+--------------+------------------+
```

**Test UPDATE:**
```sql
UPDATE orders SET total_amount = 75000000 WHERE id = 9999;

SELECT id, total_amount, total_amount_idn FROM orders WHERE id = 9999;
```

**Expected:**
```
+------+--------------+-------------------+
| id   | total_amount | total_amount_idn  |
+------+--------------+-------------------+
| 9999 |     75000000 |        75000.0000 |
+------+--------------+-------------------+
```

**Cleanup:**
```sql
DELETE FROM orders WHERE id >= 9000;
```

---

## Test 4: Circuit Breaker Testing

### Automated Test

```bash
go run cmd/test_circuit_breaker/main.go
```

**Interactive steps:**
1. Verifies normal operation
2. Asks you to stop MySQL (`docker-compose stop mysql`)
3. Tests circuit breaker opening
4. Asks you to start MySQL (`docker-compose start mysql`)
5. Tests auto-recovery

### Manual Test

**Phase 1: Trigger Circuit Breaker**
```bash
# Stop MySQL
docker-compose stop mysql

# Try connecting through proxy (will fail)
mysql -h 127.0.0.1 -P 3308 -u root -psecret ecommerce_db
# Repeat 5-6 times
```

**Check proxy logs:**
```json
{"level":"WARN","message":"Circuit breaker opened","failures":5}
{"level":"WARN","message":"Circuit breaker is OPEN, rejecting connection"}
```

**Phase 2: Test Recovery**
```bash
# Start MySQL
docker-compose start mysql

# Wait 35 seconds for circuit breaker timeout

# Try connecting again (should succeed)
mysql -h 127.0.0.1 -P 3308 -u root -psecret ecommerce_db -e "SELECT 1"
```

**Check proxy logs:**
```json
{"level":"INFO","message":"Circuit breaker state changed","from":"OPEN","to":"HALF_OPEN"}
{"level":"INFO","message":"Circuit breaker state changed","from":"HALF_OPEN","to":"CLOSED"}
```

---

## Test 5: Metrics & Monitoring

### Test Metrics Endpoint

```bash
go run cmd/test_metrics/main.go
```

**Output:**
```
=== Test 5: Connection Pool & Metrics ===

Circuit Breaker Metrics:
  (No metrics yet)

Connection Pool Metrics:
transisidb_connection_pool_active 0

Query Metrics:
transisidb_query_duration_seconds_count{operation="api_request"} 10
transisidb_query_duration_seconds_sum{operation="api_request"} 0.0087

API Request Metrics:
transisidb_api_requests_total{endpoint="/api/v1/config",method="GET",status="200"} 3

Total metrics available: 26
✓ Metrics endpoint is accessible
✓ Prometheus integration working
```

### View Metrics in Browser

```bash
# Open metrics endpoint
start http://localhost:8080/metrics
```

### Prometheus Queries

```bash
# Open Prometheus
start http://localhost:9090

# Try these queries:
rate(transisidb_query_total[1m])
histogram_quantile(0.99, transisidb_query_duration_seconds_bucket)
transisidb_connection_pool_active
```

---

## Test 6: Unit Tests

### Run Go Tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/proxy

# With coverage
go test -cover ./...

# Verbose
go test -v ./internal/proxy

# Run specific test
go test -run TestCircuitBreaker ./internal/proxy
```

### Expected Output

```
ok      github.com/kafitramarna/TransisiDB/internal/proxy       0.234s
ok      github.com/kafitramarna/TransisiDB/internal/parser      0.156s
ok      github.com/kafitramarna/TransisiDB/internal/transform   0.189s
ok      github.com/kafitramarna/TransisiDB/pkg/protocol         0.123s
```

---

## Test 7: Load Testing (Optional)

### Simple Load Test with mysqlslap

```bash
mysqlslap \
  --host=127.0.0.1 \
  --port=3308 \
  --user=root \
  --password=secret \
  --database=ecommerce_db \
  --concurrency=10 \
  --iterations=100 \
  --create-schema \
  --query="SELECT * FROM orders LIMIT 10" \
  --verbose
```

### Advanced Load Test with k6

**load_test.js:**
```javascript
import sql from 'k6/x/sql';

const db = sql.open("mysql", "root:secret@tcp(127.0.0.1:3308)/ecommerce_db");

export default function () {
  sql.exec(db, "SELECT * FROM orders LIMIT 10");
}
```

**Run:**
```bash
k6 run --vus 100 --duration 30s load_test.js
```

---

## Troubleshooting Tests

### Test Failures

**Issue: Connection refused**
```
Error: dial tcp 127.0.0.1:3308: connect: connection refused
```

**Solution:**
```bash
# Start proxy
go run cmd/proxy/main.go

# Or check if port is in use
netstat -ano | findstr :3308
```

**Issue: Authentication failed**
```
Error: Access denied for user 'root'@'localhost'
```

**Solution:**
```bash
# Check MySQL is using mysql_native_password
docker exec transisidb-mysql mysql -u root -psecret \
  -e "SELECT user, host, plugin FROM mysql.user WHERE user='root'"

# Should show: mysql_native_password
```

**Issue: Shadow columns not populated**
```
total_amount_idn: NULL
```

**Solution:**
```bash
# Check if using interpolateParams=true in DSN
dsn := "root:secret@tcp(127.0.0.1:3308)/ecommerce_db?interpolateParams=true"

# Or use text protocol queries instead of prepared statements
```

**Issue: Wrong conversion values**
```
Expected: 50000.0000
Got: 50001.0000
```

**Solution:**
```bash
# Check conversion ratio in config.yaml
Conversion:
  Ratio: 1000  # Should be 1000 for IDR→IDN

# Check rounding strategy
  RoundingStrategy: BANKERS_ROUND
```

---

## Validation Checklist

Before deployment, verify all tests pass:

- [ ] Unit tests: `go test ./...`
- [ ] Integration Test 1: Basic Connectivity
- [ ] Integration Test 2: Dual-Write INSERT
- [ ] Integration Test 3: Dual-Write UPDATE
- [ ] Integration Test 4: Transactions
- [ ] Integration Test 5: Banker's Rounding
- [ ] Integration Test 6: COM_PING
- [ ] Integration Test 7: Database Switching
- [ ] Manual Test 1: INSERT through proxy
- [ ] Manual Test 2: UPDATE through proxy
- [ ] Manual Test 3: Transaction COMMIT
- [ ] Manual Test 4: Transaction ROLLBACK
- [ ] Manual Test 5: Banker's Rounding
- [ ] Circuit Breaker: Opens on failures
- [ ] Circuit Breaker: Recovers automatically
- [ ] Metrics: Endpoint accessible
- [ ] Metrics: Values updating
- [ ] Database Viewer: CLI works
- [ ] Database Viewer: Web dashboard works
- [ ] Load Test: Handles 100 concurrent connections
- [ ] Load Test: p99 latency < 10ms

---

## Continuous Integration

### GitHub Actions Example

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    
    services:
      mysql:
        image: mysql:8.0
        env:
          MYSQL_ROOT_PASSWORD: secret
          MYSQL_DATABASE: ecommerce_db
        ports:
          - 3307:3306
      
      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379
    
    steps:
      - uses: actions/checkout@v3
      
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run unit tests
        run: go test -v -cover ./...
      
      - name: Start proxy
        run: go run cmd/proxy/main.go &
        
      - name: Run integration tests
        run: go run cmd/test_proxy/main.go
```

---

Last Updated: 2025-11-21  
Version: 1.0
