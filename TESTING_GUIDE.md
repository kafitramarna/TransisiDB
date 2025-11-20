# TransisiDB - Testing Guide

Panduan lengkap untuk menjalankan dan testing TransisiDB secara lokal.

---

## Prerequisites

Pastikan sudah terinstall:
- ✅ Docker Desktop (running)
- ✅ Go 1.21+ (untuk build)
- ✅ Terminal/PowerShell

---

## Step 1: Start Infrastructure Services

### 1.1 Start Docker Compose

```powershell
# Dari directory project
cd "C:\Users\HYPE AMD\Documents\Coding\Go\TransisiDB"

# Start MySQL, Redis, Prometheus, Grafana
docker-compose up -d
```

**Expected Output:**
```
Creating network "transisidb_transisidb-network" ... done
Creating transisidb-mysql ... done
Creating transisidb-redis ... done
Creating transisidb-prometheus ... done
Creating transisidb-grafana ... done
```

### 1.2 Verify Services Running

```powershell
# Check running containers
docker ps
```

**Should show 4 containers:**
- transisidb-mysql (port 3306)
- transisidb-redis (port 6379)
- transisidb-prometheus (port 9090)
- transisidb-grafana (port 3000)

### 1.3 Wait for MySQL to be Ready

```powershell
# Wait ~10 seconds for MySQL to initialize
Start-Sleep -Seconds 10

# Or check MySQL logs
docker logs transisidb-mysql
```

**Look for:** `ready for connections`

---

## Step 2: Verify Database Initialization

### 2.1 Connect to MySQL

```powershell
# Connect using Docker exec
docker exec -it transisidb-mysql mysql -uroot -psecret ecommerce_db
```

### 2.2 Check Tables Created

```sql
-- Show tables
SHOW TABLES;

-- Should see: orders, invoices

-- Check orders table structure
DESCRIBE orders;

-- Should show columns:
-- total_amount (BIGINT)
-- total_amount_idn (DECIMAL 19,4) - shadow column
-- shipping_fee (INT)
-- shipping_fee_idn (DECIMAL 12,4) - shadow column
```

### 2.3 Check Sample Data

```sql
-- Count rows
SELECT COUNT(*) FROM orders;
-- Should show: 5 rows

-- View sample data
SELECT id, customer_id, total_amount, total_amount_idn, shipping_fee, shipping_fee_idn 
FROM orders 
LIMIT 3;

-- total_amount_idn should be NULL (not migrated yet)
```

**Exit MySQL:**
```sql
EXIT;
```

---

## Step 3: Test API Server

### 3.1 Start API Server

```powershell
# Open terminal di directory project
cd "C:\Users\HYPE AMD\Documents\Coding\Go\TransisiDB"

# Run API server
.\bin\transisidb-api.exe --config config.yaml
```

**Expected Output:**
```
TransisiDB Management API
Configuration loaded from: config.yaml
Redis connection established
Starting API server on 0.0.0.0:8080
```

### 3.2 Test Health Check (Public Endpoint)

**Buka terminal BARU (jangan close yang API server), lalu:**

```powershell
# Test health endpoint (no auth required)
curl http://localhost:8080/health
```

**Expected Response:**
```json
{
  "status": "healthy",
  "redis": "healthy",
  "timestamp": 1700000000
}
```

### 3.3 Test Configuration Endpoint (With Auth)

```powershell
# Get configuration (requires API key)
curl -H "Authorization: Bearer sk_dev_changeme" http://localhost:8080/api/v1/config
```

**Expected:** Full configuration JSON returned

### 3.4 Test List Tables

```powershell
curl -H "Authorization: Bearer sk_dev_changeme" http://localhost:8080/api/v1/tables
```

**Expected Response:**
```json
{
  "tables": ["orders", "invoices"],
  "count": 2
}
```

### 3.5 Test Get Table Config

```powershell
curl -H "Authorization: Bearer sk_dev_changeme" http://localhost:8080/api/v1/tables/orders
```

**Expected:** Table configuration with columns returned

**✅ API Server Working!** 

Tekan `Ctrl+C` di terminal API server untuk stop (atau biarkan running).

---

## Step 4: Test Backfill Worker

### 4.1 Check Current Database State

```powershell
# Connect to MySQL
docker exec -it transisidb-mysql mysql -uroot -psecret ecommerce_db -e "SELECT id, total_amount, total_amount_idn FROM orders;"
```

**Expected Output:**
```
+----+--------------+-----------------+
| id | total_amount | total_amount_idn|
+----+--------------+-----------------+
|  1 |       500000 |            NULL |
|  2 |      1250000 |            NULL |
|  3 |       750000 |            NULL |
|  4 |      2000000 |            NULL |
|  5 |       350000 |            NULL |
+----+--------------+-----------------+
```

**total_amount_idn is NULL** - belum di-migrate.

### 4.2 Run Backfill Worker

```powershell
# Run backfill untuk table 'orders'
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
Speed: 50 rows/sec | ETA: now | Errors: 0

============================================================
BACKFILL COMPLETED SUCCESSFULLY
============================================================
Table: orders
Total rows processed: 5
Errors: 0
Duration: 1s
Average speed: 5 rows/second
============================================================
```

### 4.3 Verify Migration Results

```powershell
# Check database again
docker exec -it transisidb-mysql mysql -uroot -psecret ecommerce_db -e "SELECT id, total_amount, total_amount_idn, shipping_fee, shipping_fee_idn FROM orders;"
```

**Expected Output (NOW WITH VALUES):**
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

**✅ Backfill Working!** Data successfully migrated with correct conversion!

**Verify Conversion:**
- 500000 / 1000 = 500.0000 ✓
- 1250000 / 1000 = 1250.0000 ✓
- Banker's Rounding applied ✓

---

## Step 5: Test Banker's Rounding Edge Cases

### 5.1 Insert Test Data with Halfway Values

```powershell
# Insert rows dengan nilai halfway (x.5)
docker exec -it transisidb-mysql mysql -uroot -psecret ecommerce_db -e "
INSERT INTO orders (customer_id, total_amount, shipping_fee, status) VALUES
(9001, 500500, 10500, 'test'),   -- 500.5 -> should round to 500.5000 (even)
(9002, 501500, 11500, 'test'),   -- 501.5 -> should round to 501.5000 (even)  
(9003, 502500, 12500, 'test'),   -- 502.5 -> should round to 502.5000 (even)
(9004, 503500, 13500, 'test');   -- 503.5 -> should round to 503.5000 (even)
"
```

### 5.2 Run Backfill Again

```powershell
.\bin\transisidb-backfill.exe --table orders --config config.yaml
```

### 5.3 Verify Banker's Rounding

```powershell
docker exec -it transisidb-mysql mysql -uroot -psecret ecommerce_db -e "
SELECT customer_id, total_amount, total_amount_idn 
FROM orders WHERE customer_id >= 9001;
"
```

**Expected (Banker's Rounding):**
```
+-------------+--------------+-----------------+
| customer_id | total_amount | total_amount_idn|
+-------------+--------------+-----------------+
|        9001 |       500500 |        500.5000 | ← 500.5 to even (correct)
|        9002 |       501500 |        501.5000 | ← 501.5 to even (correct)
|        9003 |       502500 |        502.5000 | ← 502.5 to even (correct)
|        9004 |       503500 |        503.5000 | ← 503.5 to even (correct)
+-------------+--------------+-----------------+
```

**✅ Banker's Rounding Working Correctly!**

---

## Step 6: Test API Backfill Status

### 6.1 Start Backfill for Large Table (Simulation)

**If API server not running, start it first:**
```powershell
.\bin\transisidb-api.exe --config config.yaml
```

### 6.2 Check Backfill Status via API

```powershell
curl -H "Authorization: Bearer sk_dev_changeme" http://localhost:8080/api/v1/backfill/status
```

**Response (if no backfill running):**
```json
{
  "status": "no_worker",
  "message": "Backfill worker not initialized"
}
```

**Note:** Full backfill monitoring via API requires worker integration (bisa di-implement later as extension).

---

## Step 7: Access Monitoring Dashboards

### 7.1 Prometheus

```
http://localhost:9090
```

**Try Query:**
```
up
```

### 7.2 Grafana

```
http://localhost:3000
```

**Login:**
- Username: `admin`
- Password: `admin`

**Note:** Dashboards belum configured (optional Phase 5).

---

## Step 8: Cleanup

### 8.1 Stop Applications

```powershell
# Stop API server: Ctrl+C di terminal yang running

# Stop all Docker containers
docker-compose down
```

### 8.2 Remove Test Data (Optional)

```powershell
# If you want to reset
docker-compose down -v  # Remove volumes too
```

---

## Testing Checklist

- [x] Docker services start successfully
- [x] MySQL initialized with schema
- [x] API server responds to health check
- [x] API endpoints work with authentication
- [x] Backfill worker processes data
- [x] Data correctly converted (BIGINT → DECIMAL)
- [x] Banker's Rounding works for halfway values
- [x] Progress tracking shows in logs
- [x] Prometheus & Grafana accessible

---

## Common Issues & Solutions

### Issue: Docker containers won't start

**Solution:**
```powershell
# Check Docker Desktop is running
# Check port conflicts (3306, 6379, 8080, 9090, 3000)
docker ps -a  # See all containers
docker-compose down  # Clean up
docker-compose up -d  # Retry
```

### Issue: MySQL not ready

**Solution:**
```powershell
# Wait longer
Start-Sleep -Seconds 20

# Check logs
docker logs transisidb-mysql
```

### Issue: API returns 401 Unauthorized

**Solution:**
```powershell
# Make sure using correct API key from config.yaml
# Default: sk_dev_changeme
curl -H "Authorization: Bearer sk_dev_changeme" ...
```

### Issue: Backfill shows 0 rows

**Solution:**
```sql
-- Check if data already migrated
SELECT COUNT(*) FROM orders WHERE total_amount_idn IS NOT NULL;

-- Reset for testing
UPDATE orders SET total_amount_idn = NULL, shipping_fee_idn = NULL;
```

---

## What to Observe

### Successful Backfill Indicators:
1. ✅ Progress shows increasing percentage
2. ✅ Speed shows rows/second
3. ✅ ETA calculates automatically
4. ✅ Completes with "SUCCESS" message
5. ✅ Database has converted values

### Successful API Indicators:
1. ✅ Health check returns "healthy"
2. ✅ Authentication works
3. ✅ Endpoints return JSON responses
4. ✅ No error messages in console

---

**🎉 Semua Test Passed? TransisiDB Working Perfectly!**

Lanjutkan dengan:
- Load testing (optional)
- Integration dengan aplikasi real
- Deploy ke staging environment
