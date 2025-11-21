# TransisiDB Troubleshooting Guide

Common issues and solutions for TransisiDB deployment and operation.

---

## Table of Contents

1. [Connection Issues](#connection-issues)
2. [Authentication Problems](#authentication-problems)
3. [Query Transformation Issues](#query-transformation-issues)
4. [Performance Problems](#performance-problems)
5. [Circuit Breaker Issues](#circuit-breaker-issues)
6. [Configuration Errors](#configuration-errors)
7. [Data Consistency Issues](#data-consistency-issues)
8. [Monitoring & Metrics](#monitoring--metrics)
9. [Docker Issues](#docker-issues)
10. [Production Issues](#production-issues)

---

## Connection Issues

### Issue: Connection Refused

**Symptom:**
```
Error: dial tcp 127.0.0.1:3308: connect: connection refused
```

**Diagnosis:**
```bash
# Check if proxy is running
ps aux | grep transisidb

# Check if port is listening
netstat -ano | findstr :3308  # Windows
lsof -i :3308                  # Linux/Mac
```

**Solutions:**

1. **Start the proxy:**
```bash
go run cmd/proxy/main.go
```

2. **Check port conflicts:**
```bash
# Find process using port 3308
netstat -ano | findstr :3308

# Kill process (Windows)
taskkill /PID <PID> /F

# Kill process (Linux/Mac)
kill -9 <PID>
```

3. **Check firewall:**
```bash
# Windows Firewall
netsh advfirewall firewall add rule name="TransisiDB" dir=in action=allow protocol=TCP localport=3308

# Linux iptables
sudo iptables -A INPUT -p tcp --dport 3308 -j ACCEPT
```

---

### Issue: Connection Timeout

**Symptom:**
```
Error: dial tcp 10.0.1.100:3308: i/o timeout
```

**Diagnosis:**
```bash
# Test connectivity
telnet 10.0.1.100 3308

# Check proxy logs
tail -f /var/log/transisidb/proxy.log
```

**Solutions:**

1. **Increase timeout in client:**
```go
dsn := "user:pass@tcp(host:3308)/db?timeout=30s"
```

2. **Check network:**
```bash
# Ping host
ping 10.0.1.100

# Traceroute
traceroute 10.0.1.100
```

3. ** Increase proxy timeout:**
```yaml
Proxy:
  ReadTimeout: 60s
  WriteTimeout: 60s
```

---

## Authentication Problems

### Issue: Access Denied

**Symptom:**
```
Error 1045: Access denied for user 'root'@'172.17.0.1' (using password: YES)
```

**Diagnosis:**
```bash
# Test direct MySQL connection
mysql -h mysql.host -P 3306 -u root -pSecret

# Check MySQL user
docker exec transisidb-mysql mysql -u root -psecret \
  -e "SELECT user, host, plugin FROM mysql.user WHERE user='root'"
```

**Solutions:**

1. **Check authentication plugin:**
```sql
-- MySQL must use mysql_native_password
ALTER USER 'root'@'%' IDENTIFIED WITH mysql_native_password BY 'secret';
FLUSH PRIVILEGES;
```

2. **Check config.yaml credentials:**
```yaml
Database:
  User: root
  Password: secret  # Must match MySQL password
```

3. **Check MySQL allows remote connections:**
```sql
-- Grant access from any host
GRANT ALL PRIVILEGES ON *.* TO 'root'@'%' WITH GRANT OPTION;
FLUSH PRIVILEGES;
```

---

### Issue: caching_sha2_password Not Supported

**Symptom:**
```
Error: caching_sha2_password authentication is not supported
```

**Solution:**

Change MySQL to use `mysql_native_password`:

**docker-compose.yml:**
```yaml
services:
  mysql:
    command: --default-authentication-plugin=mysql_native_password
```

**Or alter existing user:**
```sql
ALTER USER 'root'@'%' IDENTIFIED WITH mysql_native_password BY 'secret';
FLUSH PRIVILEGES;
```

---

## Query Transformation Issues

### Issue: Shadow Columns Not Populated

**Symptom:**
```sql
SELECT * FROM orders WHERE id = 1001;
-- total_amount_idn is NULL
```

**Diagnosis:**
```bash
# Check proxy logs
grep "Query transformed" /var/log/transisidb/proxy.log

# Check if using text protocol
# DSN must have interpolateParams=true
```

**Solutions:**

1. **Use text protocol in DSN:**
```go
// Force text protocol for transformation
dsn := "user:pass@tcp(proxy:3308)/db?interpolateParams=true"
```

2. **Check table configuration:**
```yaml
Tables:
  orders:
    Enabled: true  # Must be true
    Columns:
      total_amount:
        SourceColumn: total_amount
        TargetColumn: total_amount_idn
```

3. **Verify shadow column exists:**
```sql
DESC orders;
-- Should show: total_amount_idn DECIMAL(19,4)
```

4. **Check transformation logic:**
```bash
# Enable DEBUG logging
export LOG_LEVEL=DEBUG
go run cmd/proxy/main.go

# Look for transformation logs
grep "handleQuery" logs
```

---

### Issue: Wrong Conversion Values

**Symptom:**
```
Expected: 50000.0000
Got:      50001.0000  # Wrong!
```

**Diagnosis:**
```yaml
# Check conversion ratio
Conversion:
  Ratio: 1000  # Should be 1000 for IDR→IDN
  Precision: 4  # Should be 4
```

**Solutions:**

1. **Verify conversion formula:**
```
IDN = IDR / Ratio
50000000 / 1000 = 50000.0000 ✓
```

2. **Check rounding strategy:**
```yaml
Conversion:
  RoundingStrategy: BANKERS_ROUND  # Use Banker's Rounding
```

3. **Manual validation:**
```sql
-- Test conversion manually
SELECT 
  total_amount,
  total_amount_idn,
  total_amount / 1000 AS expected,
  ABS(total_amount_idn - (total_amount / 1000)) AS diff
FROM orders
WHERE total_amount_idn IS NOT NULL
  AND ABS(total_amount_idn - (total_amount / 1000)) > 0.0001;

-- Should return 0 rows
```

---

## Performance Problems

### Issue: High Latency

**Symptom:**
```
Query latency > 100ms (expected < 5ms)
```

**Diagnosis:**
```bash
# Check metrics
curl http://localhost:8080/metrics | grep duration

# Check connection pool
curl http://localhost:8080/metrics | grep pool

# MySQL slow query log
docker exec transisidb-mysql mysql -u root -psecret \
  -e "SELECT * FROM mysql.slow_log ORDER BY query_time DESC LIMIT 10"
```

**Solutions:**

1. **Increase connection pool:**
```yaml
Proxy:
  PoolSize: 200  # Increase from 100
  MaxConnectionsPerHost: 100
```

2. **Tune MySQL:**
```sql
-- Increase buffer pool
SET GLOBAL innodb_buffer_pool_size = 2147483648;  -- 2GB

-- Check indexes
SHOW INDEX FROM orders;
```

3. **Reduce proxy overhead:**
```yaml
Logging:
  Level: WARN  # Reduce logging from DEBUG
```

4. **Check network latency:**
```bash
# Ping backend MySQL
ping -c 10 mysql.prod.internal

# Should be < 1ms on same network
```

---

### Issue: Connection Pool Exhausted

**Symptom:**
```
Error: connection pool exhausted, waited 30s
```

**Diagnosis:**
```bash
# Check current pool usage
curl http://localhost:8080/metrics | grep connection_pool_active

# Check max connections
curl http://localhost:8080/api/v1/config | jq '.Proxy.PoolSize'
```

**Solutions:**

1. **Increase pool size:**
```yaml
Proxy:
  PoolSize: 500  # Increase capacity
```

2. **Check for connection leaks:**
```bash
# Monitor active connections over time
watch -n 1 'curl -s http://localhost:8080/metrics | grep connection_pool_active'

# Should not constantly increase
```

3. **Reduce connection idle time:**
```yaml
Database:
  ConnectionTimeout: 10s  # Reduce timeout
```

---

## Circuit Breaker Issues

### Issue: Circuit Breaker Stuck OPEN

**Symptom:**
```json
{"level":"WARN","message":"Circuit breaker is OPEN, rejecting connection"}
```

**Diagnosis:**
```bash
# Check circuit breaker state
curl http://localhost:8080/metrics | grep circuit_breaker_state

# Check backend MySQL health
mysql -h mysql.host -P 3306 -e "SELECT 1"
```

**Solutions:**

1. **Wait for timeout:**
```yaml
Proxy:
  CircuitBreaker:
    Timeout: 30s  # Wait 30s for HALF-OPEN
```

2. **Manually reset (restart proxy):**
```bash
# Restart proxy
sudo systemctl restart transisidb

# Or send SIGHUP
kill -HUP $(pidof transisidb)
```

3. **Fix backend MySQL:**
```bash
# Restart MySQL
docker-compose restart mysql

# Check MySQL logs
docker logs transisidb-mysql
```

---

### Issue: Circuit Breaker Opens Too Frequently

**Symptom:**
Circuit breaker opens every few minutes

**Diagnosis:**
```bash
# Check error rate
curl http://localhost:8080/metrics | grep errors_total

# Check MySQL connection stability
docker logs transisidb-mysql | grep -i error
```

**Solutions:**

1. **Increase failure threshold:**
```yaml
Proxy:
  CircuitBreaker:
    MaxFailures: 10  # Increase from 5
    Timeout: 60s      # Wait longer
```

2. **Investigate MySQL issues:**
```sql
-- Check MySQL processlist
SHOW FULL PROCESSLIST;

-- Check for locks
SHOW OPEN TABLES WHERE In_use > 0;
```

---

## Configuration Errors

### Issue: Config File Not Found

**Symptom:**
```
Fatal: Failed to load configuration: open config.yaml: no such file or directory
```

**Solutions:**

1. **Specify full path:**
```bash
./transisidb -config /etc/transisidb/config.yaml
```

2. **Check file exists:**
```bash
ls -l config.yaml
```

3. **Use default location:**
```bash
# Copy to default location
cp config.yaml /etc/transisidb/config.yaml
```

---

### Issue: Invalid YAML Syntax

**Symptom:**
```
Fatal: yaml: line 42: mapping values are not allowed in this context
```

**Solutions:**

1. **Validate YAML:**
```bash
# Use online validator or
python3 -c "import yaml; yaml.safe_load(open('config.yaml'))"
```

2. **Check indentation:**
```yaml
# WRONG (tabs)
Database:
	Host: localhost

# CORRECT (2 spaces)
Database:
  Host: localhost
```

3. **Check quotes:**
```yaml
# WRONG
Password: my:password:123

# CORRECT
Password: "my:password:123"
```

---

## Data Consistency Issues

### Issue: Data Mismatch Between IDR and IDN

**Symptom:**
```sql
-- IDR and IDN values don't match expected ratio
SELECT * FROM orders WHERE ABS(total_amount_idn - (total_amount / 1000)) > 0.01;
-- Returns rows (should be empty)
```

**Diagnosis:**
```sql
-- Check all mismatches
SELECT 
  id,
  total_amount,
  total_amount_idn,
  total_amount / 1000 AS expected,
  total_amount_idn - (total_amount / 1000) AS diff
FROM orders
WHERE total_amount_idn IS NOT NULL
  AND ABS(total_amount_idn - (total_amount / 1000)) > 0.0001;
```

**Solutions:**

1. **Re-run backfill:**
```bash
# Reset shadow columns
UPDATE orders SET total_amount_idn = NULL, shipping_fee_idn = NULL;

# Run backfill again
go run cmd/backfill/main.go -table orders
```

2. **Check for manual edits:**
```sql
-- Look for audit trail
SELECT * FROM orders_audit WHERE column_name = 'total_amount_idn';
```

3. **Fix specific rows:**
```sql
UPDATE orders 
SET total_amount_idn = total_amount / 1000
WHERE id IN (SELECT id FROM mismatches);
```

---

## Monitoring & Metrics

### Issue: Metrics Not Updating

**Symptom:**
Prometheus shows stale metrics

**Diagnosis:**
```bash
# Check metrics endpoint
curl http://localhost:8080/metrics

# Check Prometheus targets
curl http://localhost:9090/api/v1/targets
```

**Solutions:**

1. **Restart Prometheus:**
```bash
docker-compose restart prometheus
```

2. **Check scrape config:**
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'transisidb'
    static_configs:
      - targets: ['transisidb-proxy:8080']  # Correct host
```

3. **Check network:**
```bash
# From Prometheus container
docker exec transisidb-prometheus wget -O- http://transisidb-proxy:8080/metrics
```

---

## Docker Issues

### Issue: Container Won't Start

**Symptom:**
```
Error: Bind for 0.0.0.0:3308 failed: port is already allocated
```

**Solutions:**

1. **Find conflicting process:**
```bash
# Windows
netstat -ano | findstr :3308
taskkill /PID <PID> /F

# Linux
lsof -i :3308
kill -9 <PID>
```

2. **Change port:**
```yaml
# docker-compose.yml
ports:
  - "3309:3308"  # Use different host port
```

3. **Stop all containers:**
```bash
docker-compose down
docker ps -a  # Check if any stuck
docker rm -f $(docker ps -aq)  # Remove all
```

---

### Issue: MySQL Init Script Not Running

**Symptom:**
Tables don't exist in database

**Diagnosis:**
```bash
# Check if init.sql was executed
docker exec transisidb-mysql mysql -u root -psecret -e "SHOW TABLES" ecommerce_db
```

**Solutions:**

1. **Recreate with fresh volume:**
```bash
docker-compose down -v  # Remove volumes
docker-compose up -d
```

2. **Manually run init script:**
```bash
docker exec -i transisidb-mysql mysql -u root -psecret ecommerce_db < scripts/init.sql
```

---

## Production Issues

### Issue: Memory Leak

**Symptom:**
Memory usage constantly increases

**Diagnosis:**
```bash
# Monitor memory
top -p $(pidof transisidb)

# Generate heap profile
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

**Solutions:**

1. **Check for goroutine leaks:**
```bash
curl http://localhost:8080/debug/pprof/goroutine?debug=2
```

2. **Restart periodically:**
```bash
# Cron job to restart weekly
0 2 * * 0 systemctl restart transisidb
```

3. **Limit resources:**
```yaml
# docker-compose.yml
services:
  transisidb-proxy:
    mem_limit: 2g
    mem_reservation: 1g
```

---

### Issue: Proxy Crash / Panic

**Symptom:**
```
panic: runtime error: invalid memory address
```

**Diagnosis:**
```bash
# Check crash logs
journalctl -u transisidb -n 100

# Check core dump
coredumpctl list
coredumpctl dump transisidb
```

**Solutions:**

1. **Enable auto-restart:**
```ini
# /etc/systemd/system/transisidb.service
[Service]
Restart=on-failure
RestartSec=5s
```

2. **Update to latest version:**
```bash
git pull
go build -o transisidb cmd/proxy/main.go
```

3. **Report bug:**
```bash
# Collect information
- Go version: go version
- OS: uname -a
- Config: cat config.yaml
- Logs: last 100 lines
- Stack trace
```

---

## Getting Help

### Log Collection

```bash
# Collect all relevant logs
tar -czf transisidb-logs.tar.gz \
  /var/log/transisidb/ \
  config.yaml \
  docker-compose ps output

# Share in GitHub issue
```

### Debug Mode

```bash
# Enable debug logging
export LOG_LEVEL=DEBUG
./transisidb -config config.yaml

# Watch logs in real-time
tail -f /var/log/transisidb/proxy.log | jq .
```

### Support Channels

- 📧 Email: support@transisidb.com
- 🐛 GitHub Issues: https://github.com/kafitramarna/TransisiDB/issues
- 💬 Slack: https://transisidb.slack.com
- 📖 Documentation: https://docs.transisidb.com

---

Last Updated: 2025-11-21  
Version: 1.0
