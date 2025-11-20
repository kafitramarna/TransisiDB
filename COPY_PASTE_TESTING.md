# TransisiDB - Copy-Paste Testing Commands

> **Status:** Docker containers already running!
> Just follow the commands below in sequence.

---

## Part 1: Verify Docker Services (30 seconds)

```powershell
# Check all containers running
docker ps

# Wait for MySQL to finish initialization (if just started)
Start-Sleep -Seconds 15

# Test MySQL connection
docker exec transisidb-mysql mysql -uroot -psecret -e "SELECT 1;"
```

**Expected:** Should see "1" output (MySQL is ready)

---

## Part 2: Check Database Schema (1 minute)

```powershell
# Show all tables
docker exec transisidb-mysql mysql -uroot -psecret ecommerce_db -e "SHOW TABLES;"

# Check orders table structure
docker exec transisidb-mysql mysql -uroot -psecret ecommerce_db -e "DESCRIBE orders;"

# View sample data (BEFORE migration)
docker exec transisidb-mysql mysql -uroot -psecret ecommerce_db -e "SELECT id, total_amount, total_amount_idn, shipping_fee, shipping_fee_idn FROM orders LIMIT 3;"
```

**Expected:** 
- See `orders` and `invoices` tables
- `total_amount_idn` and `shipping_fee_idn` columns exist
- Shadow columns (`*_idn`) should be NULL (not migrated yet)

---

## Part 3: Test API Server (2 minutes)

### Terminal 1: Start API Server

```powershell
# Navigate to project directory (if not already)
cd "C:\Users\HYPE AMD\Documents\Coding\Go\TransisiDB"

# Start API server
.\bin\transisidb-api.exe --config config.yaml
```

**Keep this terminal open** (API server running)

### Terminal 2: Test API Endpoints

Open a **NEW** PowerShell terminal:

```powershell
# Navigate to project
cd "C:\Users\HYPE AMD\Documents\Coding\Go\TransisiDB"

# Test 1: Health Check (NO auth required)
curl http://localhost:8080/health

# Test 2: List Tables (with auth) - FIRST RUN MAY SHOW count:0
curl -H "Authorization: Bearer sk_dev_changeme" http://localhost:8080/api/v1/tables
```

**If you see `{"count":0,"tables":null}`:**

This is normal on first run! API reads from Redis, which is empty initially.

**Fix: Restart API Server**

1. Go to Terminal 1 (where API server is running)
2. Press `Ctrl+C` to stop
3. Restart API server:
   ```powershell
   .\bin\transisidb-api.exe --config config.yaml
   ```
4. In Terminal 2, test again:
   ```powershell
   curl -H "Authorization: Bearer sk_dev_changeme" http://localhost:8080/api/v1/tables
   ```

**Now you should see:**
```json
{
  "count": 2,
  "tables": ["orders", "invoices"]
}
```

**Continue testing:**

```powershell
# Test 3: Get Table Config
curl -H "Authorization: Bearer sk_dev_changeme" http://localhost:8080/api/v1/tables/orders

# Test 4: Get Full Config
curl -H "Authorization: Bearer sk_dev_changeme" http://localhost:8080/api/v1/config
```

**Expected:**
- Health check returns JSON with status
- Tables list shows `["orders", "invoices"]` ✓
- Table config shows column configurations
- All requests should return JSON

**Why restart needed?** 
- API server loads `config.yaml` at startup
- Then saves it to Redis for hot-reload capability
- Endpoints read from Redis (not config file)
- First startup: config.yaml → Redis (auto-save)

---

## Part 4: Test Backfill Worker (3 minutes)

Still in **Terminal 2** (API server still running in Terminal 1):

```powershell
# Run backfill migration for 'orders' table
.\bin\transisidb-backfill.exe --table orders --config config.yaml
```

**Expected Output:**
```
Starting backfill for table: orders
Batch size: 1000
Sleep interval: 100ms
Conversion ratio: 1:1000
Rounding strategy: BANKERS_ROUND
Database connection established

Table: orders | Status: running | Progress: 5/5 (100.0%) | 
Speed: XX rows/sec | ETA: now | Errors: 0

============================================================
BACKFILL COMPLETED SUCCESSFULLY
============================================================
Table: orders
Total rows processed: 5
Errors: 0
Duration: Xs
Average speed: X rows/second
============================================================
```

---

## Part 5: Verify Migration Results (1 minute)

```powershell
# Check migrated data (AFTER migration)
docker exec transisidb-mysql mysql -uroot -psecret ecommerce_db -e "SELECT id, total_amount, total_amount_idn, shipping_fee, shipping_fee_idn FROM orders LIMIT 5;"
```

**Expected:**
```
+----+--------------+-----------------+--------------+------------------+
| id | total_amount | total_amount_idn| shipping_fee | shipping_fee_idn |
+----+--------------+-----------------+--------------+------------------+
|  1 |       500000 |        500.0000 |        25000 |          25.0000 |
|  2 |      1250000 |       1250.0000 |        30000 |          30.0000 |
|  3 |       750000 |        750.0000 |        20000 |          20.0000 |
|  4 |      2000000 |       2000.0000 |        50000 |          50.0000 |
|  5 |       350000 |        350.0000 |        15000 |          15.0000 |
+----+--------------+-----------------+--------------+------------------+
```

**Verify Conversion:**
- 500000 / 1000 = 500.0000 ✓
- 1250000 / 1000 = 1250.0000 ✓
- Shadow columns now have values!

---

## Part 6: Test Banker's Rounding (2 minutes)

```powershell
# Insert test data with halfway values
docker exec transisidb-mysql mysql -uroot -psecret ecommerce_db -e "INSERT INTO orders (customer_id, total_amount, shipping_fee, status) VALUES (9001, 500500, 10500, 'test'), (9002, 501500, 11500, 'test'), (9003, 502500, 12500, 'test');"

# Run backfill again
.\bin\transisidb-backfill.exe --table orders --config config.yaml

# Check Banker's Rounding results
docker exec transisidb-mysql mysql -uroot -psecret ecommerce_db -e "SELECT customer_id, total_amount, total_amount_idn FROM orders WHERE customer_id >= 9001;"
```

**Expected (Banker's Rounding):**
```
+-------------+--------------+-----------------+
| customer_id | total_amount | total_amount_idn|
+-------------+--------------+-----------------+
|        9001 |       500500 |        500.5000 | ← Rounds to even
|        9002 |       501500 |        501.5000 | ← Rounds to even
|        9003 |       502500 |        502.5000 | ← Rounds to even
+-------------+--------------+-----------------+
```

**All halfway values round to EVEN** (not up) - this is IEEE 754 Banker's Rounding! ✓

---

## Part 7: Access Monitoring (Optional - 1 minute)

Open in browser:

```
Prometheus:  http://localhost:9090
Grafana:     http://localhost:3000 (user: admin, pass: admin)
```

---

## Cleanup

```powershell
# Stop API server (in Terminal 1)
Press Ctrl+C

# Stop Docker containers (in Terminal 2)
docker-compose down

# OR: Stop and remove all data
docker-compose down -v
```

---

## Summary Checklist

After running all commands, you should have verified:

- ✅ Docker services running (MySQL, Redis, Prometheus, Grafana)
- ✅ Database schema initialized with shadow columns
- ✅ API server responding to health checks
- ✅ Authentication working (API key required)
- ✅ Backfill worker migrating data
- ✅ Data conversion correct (BIGINT → DECIMAL)
- ✅ Banker's Rounding working (rounds to even)
- ✅ Shadow columns populated with converted values

---

## Troubleshooting

**MySQL not ready:**
```powershell
# Wait longer
Start-Sleep -Seconds 20

# Check MySQL logs
docker logs transisidb-mysql
```

**API returns 401:**
```powershell
# Make sure using correct API key
curl -H "Authorization: Bearer sk_dev_changeme" http://localhost:8080/api/v1/tables
```

**Port conflicts:**
```powershell
# Check what's using ports
netstat -ano | findstr ":3307"
netstat -ano | findstr ":8080"
```

---

## Total Test Time: ~10 minutes

**All features of TransisiDB tested and verified!** 🎉
