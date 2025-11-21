# TransisiDB Deployment Guide

## Production Deployment Checklist

### Pre-Deployment

- [ ] Database schema updated with shadow columns
- [ ] Configuration file prepared
- [ ] Redis instance available
- [ ] Monitoring stack ready (Prometheus/Grafana)
- [ ] Backup strategy in place
- [ ] Rollback plan documented

---

## Step 1: Prepare Database

### Add Shadow Columns

```sql
-- For orders table
ALTER TABLE orders 
ADD COLUMN total_amount_idn DECIMAL(19,4) DEFAULT NULL AFTER total_amount,
ADD COLUMN shipping_fee_idn DECIMAL(12,4) DEFAULT NULL AFTER shipping_fee;

-- For invoices table
ALTER TABLE invoices
ADD COLUMN grand_total_idn DECIMAL(19,4) DEFAULT NULL AFTER grand_total,
ADD COLUMN tax_amount_idn DECIMAL(19,4) DEFAULT NULL AFTER tax_amount;

-- Add indexes for shadow columns (optional, for reporting)
ALTER TABLE orders ADD INDEX idx_total_amount_idn (total_amount_idn);
ALTER TABLE invoices ADD INDEX idx_grand_total_idn (grand_total_idn);
```

### Verify Schema

```sql
-- Check orders table
DESC orders;

-- Expected output should include:
--  total_amount      | bigint       | NO
--  total_amount_idn  | decimal(19,4)| YES

SHOW CREATE TABLE orders\G
```

---

## Step 2: Deploy Infrastructure

### Option A: Docker Deployment

**docker-compose.production.yml:**
```yaml
version: '3.8'

services:
  transisidb-proxy:
    image: transisidb:latest
    container_name: transisidb-proxy
    ports:
      - "3308:3308"      # Proxy port
      - "8080:8080"      # Management API
    volumes:
      - ./config.yaml:/etc/transisidb/config.yaml:ro
      - ./logs:/var/log/transisidb
    environment:
      - CONFIG_PATH=/etc/transisidb/config.yaml
      - LOG_LEVEL=INFO
    restart: unless-stopped
    depends_on:
      - redis
    networks:
      - transisidb-net

  redis:
    image: redis:7-alpine
    container_name: transisidb-redis
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    command: redis-server --appendonly yes
    restart: unless-stopped
    networks:
      - transisidb-net

  prometheus:
    image: prom/prometheus:latest
    container_name: transisidb-prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus-data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
    restart: unless-stopped
    networks:
      - transisidb-net

  grafana:
    image: grafana/grafana:latest
    container_name: transisidb-grafana
    ports:
      - "3000:3000"
    volumes:
      - grafana-data:/var/lib/grafana
      - ./grafana/dashboards:/etc/grafana/provisioning/dashboards:ro
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin123
    restart: unless-stopped
    networks:
      - transisidb-net

volumes:
  redis-data:
  prometheus-data:
  grafana-data:

networks:
  transisidb-net:
    driver: bridge
```

**Deploy:**
```bash
# Build image
docker build -t transisidb:latest -f Dockerfile .

# Start services
docker-compose -f docker-compose.production.yml up -d

# Check status
docker-compose -f docker-compose.production.yml ps

# View logs
docker-compose -f docker-compose.production.yml logs -f transisidb-proxy
```

### Option B: Systemd Service

**Build Binary:**
```bash
# Build for production
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-w -s -X main.version=$(git describe --tags)" \
  -o dist/transisidb \
  cmd/proxy/main.go
```

**Create systemd unit:**  
`/etc/systemd/system/transisidb.service`
```ini
[Unit]
Description=TransisiDB Currency Proxy
After=network.target mysql.service redis.service
Wants=redis.service

[Service]
Type=simple
User=transisidb
Group=transisidb
WorkingDirectory=/opt/transisidb
ExecStart=/opt/transisidb/bin/transisidb -config /etc/transisidb/config.yaml
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/transisidb

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
```

**Install and start:**
```bash
# Create user
sudo useradd -r -s /bin/false transisidb

# Create directories
sudo mkdir -p /opt/transisidb/{bin,logs}
sudo mkdir -p /etc/transisidb

# Copy files
sudo cp dist/transisidb /opt/transisidb/bin/
sudo cp config.yaml /etc/transisidb/
sudo chown -R transisidb:transisidb /opt/transisidb

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable transisidb
sudo systemctl start transisidb

# Check status
sudo systemctl status transisidb
sudo journalctl -u transisidb -f
```

---

## Step 3: Configuration

### Production config.yaml

```yaml
# Database connection
Database:
  Type: mysql
  Host: mysql-primary.prod.internal
  Port: 3306
  User: transisidb_proxy
  Password: "${DB_PASSWORD}"  # Use env variable
  Database: ecommerce_db
  MaxConnections: 100
  IdleConnections: 10
  ConnectionTimeout: 30s

# Proxy settings
Proxy:
  Host: 0.0.0.0
  Port: 3308
  PoolSize: 100
  MaxConnectionsPerHost: 50
  ReadTimeout: 30s
  WriteTimeout: 30s
  CircuitBreaker:
    MaxFailures: 5
    Timeout: 30s
    MaxConcurrent: 10

# Redis for configuration
Redis:
  Host: redis.prod.internal
  Port: 6379
  Password: "${REDIS_PASSWORD}"
  Database: 0
  PoolSize: 10

# Currency conversion
Conversion:
  Ratio: 1000
  Precision: 4
  RoundingStrategy: BANKERS_ROUND

# Table configurations
Tables:
  orders:
    Enabled: true
    Columns:
      total_amount:
        SourceColumn: total_amount
        SourceType: BIGINT
        TargetColumn: total_amount_idn
        TargetType: DECIMAL(19,4)
        Precision: 4
        RoundingStrategy: BANKERS_ROUND
      shipping_fee:
        SourceColumn: shipping_fee
        SourceType: INT
        TargetColumn: shipping_fee_idn
        TargetType: DECIMAL(12,4)
        Precision: 4
        RoundingStrategy: BANKERS_ROUND

  invoices:
    Enabled: true
    Columns:
      grand_total:
        SourceColumn: grand_total
        SourceType: BIGINT
        TargetColumn: grand_total_idn
        TargetType: DECIMAL(19,4)
        Precision: 4
        RoundingStrategy: BANKERS_ROUND

# Management API
API:
  Host: 0.0.0.0
  Port: 8080
  APIKey: "${API_KEY}"  # Use strong random key

# Monitoring
Monitoring:
  PrometheusEnabled: true
  PrometheusPort: 9090
  MetricsPath: /metrics

# Logging
Logging:
  Level: INFO  # DEBUG in staging, INFO in production
  Format: json
  Output: stdout  # Or file path

# Backfill (optional)
Backfill:
  Enabled: false  # Enable when ready to backfill
  BatchSize: 1000
  SleepIntervalMs: 100
  MaxCPUPercent: 20
  RetryAttempts: 3
  RetryBackoffMs: 500

# Simulation mode (disable in production)
Simulation:
  Enabled: false
```

### Environment Variables

Create `.env` file:
```bash
# Database
DB_PASSWORD=your_secure_database_password

# Redis
REDIS_PASSWORD=your_secure_redis_password

# API
API_KEY=sk_prod_your_random_api_key_here

# Optional: Override config values
# PROXY_PORT=3308
# LOG_LEVEL=INFO
```

**Load environment:**
```bash
# For systemd
sudo systemctl edit transisidb
# Add: EnvironmentFile=/etc/transisidb/.env

# For docker-compose
# Already supported via env_file: .env
```

---

## Step 4: Update Application Configuration

### Before (Direct MySQL)
```python
# Django settings.py
DATABASES = {
    'default': {
        'ENGINE': 'django.db.backends.mysql',
        'NAME': 'ecommerce_db',
        'USER': 'app_user',
        'PASSWORD': 'password',
        'HOST': 'mysql.prod.internal',
        'PORT': '3306',
    }
}
```

### After (Through Proxy)
```python
# Django settings.py
DATABASES = {
    'default': {
        'ENGINE': 'django.db.backends.mysql',
        'NAME': 'ecommerce_db',
        'USER': 'app_user',
        'PASSWORD': 'password',
        'HOST': 'transisidb-proxy.prod.internal',  # ← Changed
        'PORT': '3308',                             # ← Changed
        'OPTIONS': {
            'init_command': "SET sql_mode='STRICT_TRANS_TABLES'",
            'charset': 'utf8mb4',
        },
    }
}
```

### Go Application
```go
// Before
dsn := "user:password@tcp(mysql.prod.internal:3306)/ecommerce_db?parseTime=true"

// After
dsn := "user:password@tcp(transisidb-proxy.prod.internal:3308)/ecommerce_db?parseTime=true&interpolateParams=true"
```

### Java (JDBC)
```java
// Before
String url = "jdbc:mysql://mysql.prod.internal:3306/ecommerce_db";

// After
String url = "jdbc:mysql://transisidb-proxy.prod.internal:3308/ecommerce_db?useServerPrepStmts=false";
```

### Node.js
```javascript
// Before
const pool = mysql.createPool({
  host: 'mysql.prod.internal',
  port: 3306,
  // ...
});

// After
const pool = mysql.createPool({
  host: 'transisidb-proxy.prod.internal',
  port: 3308,
  // ...
});
```

---

## Step 5: Gradual Rollout Strategy

### Phase 1: Shadow Mode (Week 1)
- Deploy proxy alongside existing MySQL
- Route 0% traffic through proxy
- Monitor proxy logs for transformation accuracy
- Compare shadow column values with expected values

**Validation:**
```sql
-- Check transformation accuracy
SELECT 
  id,
  total_amount,
  total_amount_idn,
  ABS(total_amount_idn - (total_amount / 1000)) as diff
FROM orders
WHERE total_amount_idn IS NOT NULL
  AND ABS(total_amount_idn - (total_amount / 1000)) > 0.0001;

-- Should return 0 rows
```

### Phase 2: Canary (Week 2)
- Route 5% traffic through proxy
- Monitor error rates, latency
- Gradually increase to 25%

**Canary Configuration (HAProxy):**
```
backend mysql
    balance roundrobin
    server direct mysql.prod.internal:3306 weight 95
    server proxy transisidb-proxy.prod.internal:3308 weight 5
```

### Phase 3: Ramp Up (Week 3-4)
- Increase to 50%, then 100%
- Monitor continuously
- Keep direct MySQL route as fallback

### Phase 4: Full Cutover (Week 5)
- Route 100% through proxy
- Remove direct MySQL routes
- Celebrate! 🎉

---

## Step 6: Monitoring Setup

### Prometheus Configuration

**prometheus.yml:**
```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'transisidb'
    static_configs:
      - targets: ['transisidb-proxy:8080']
    metrics_path: /metrics

alerting:
  alertmanagers:
    - static_configs:
        - targets: ['alertmanager:9093']

rule_files:
  - '/etc/prometheus/alerts/*.yml'
```

**alerts/transisidb.yml:**
```yaml
groups:
  - name: transisidb
    interval: 30s
    rules:
      - alert: TransisiDBHighErrorRate
        expr: rate(transisidb_errors_total[5m]) > 0.01
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value }} errors/sec"

      - alert: TransisiDBCircuitBreakerOpen
        expr: transisidb_circuit_breaker_state == 1
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Circuit breaker is OPEN"
          description: "Backend MySQL may be down"

      - alert: TransisiDBHighLatency
        expr: histogram_quantile(0.99, rate(transisidb_query_duration_seconds_bucket[5m])) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High query latency"
          description: "P99 latency is {{ $value }}s"

      - alert: TransisiDBConnectionPoolExhausted
        expr: transisidb_connection_pool_active / transisidb_connection_pool_max > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Connection pool almost full"
          description: "Pool usage is {{ $value | humanizePercentage }}"
```

### Grafana Dashboard

**Key Panels:**
1. QPS (Queries Per Second)
2. P50/P95/P99 Latency
3. Error Rate
4. Circuit Breaker State
5. Connection Pool Usage
6. Backend Health

Import dashboard JSON from `monitoring/grafana-dashboard.json`

---

## Step 7: Health Checks

### Kubernetes Liveness Probe
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3
```

### Application Health Check
```bash
#!/bin/bash
# healthcheck.sh

# Check proxy is listening
nc -z localhost 3308 || exit 1

# Check API responds
curl -f http://localhost:8080/health || exit 1

# Check can connect to MySQL through proxy
timeout 5 mysql -h 127.0.0.1 -P 3308 -u healthcheck -e "SELECT 1" || exit 1

exit 0
```

---

## Step 8: Backup and Rollback

### Backup Strategy
```bash
# Before deployment
mysqldump -h mysql.prod.internal \
  --single-transaction \
  --routines \
  --triggers \
  ecommerce_db > backup_$(date +%Y%m%d_%H%M%S).sql

# Store in S3 or backup server
aws s3 cp backup_*.sql s3://backups/transisidb/
```

### Rollback Plan

**If proxy has issues:**
1. Stop routing traffic to proxy
```bash
# HAProxy: Set proxy weight to 0
echo "set server mysql/proxy weight 0" | socat stdio /var/run/haproxy.sock
```

2. Route 100% back to direct MySQL
```bash
echo "set server mysql/direct weight 100" | socat stdio /var/run/haproxy.sock
```

3. Investigate logs
```bash
docker logs transisidb-proxy
# or
sudo journalctl -u transisidb -n 1000
```

4. Fix and redeploy

**If data corruption detected:**
1. Stop ALL writes immediately
2. Restore from backup
3. Analyze root cause
4. Fix and re-test in staging

---

## Step 9: Performance Tuning

### OS Tuning (Linux)

```bash
# Increase file descriptor limits
sudo sysctl -w fs.file-max=100000
echo "fs.file-max = 100000" | sudo tee -a /etc/sysctl.conf

# TCP tuning
sudo sysctl -w net.core.somaxconn=4096
sudo sysctl -w net.ipv4.tcp_max_syn_backlog=4096
sudo sysctl -w net.core.netdev_max_backlog=5000

# Apply
sudo sysctl -p
```

### Transisi DB Tuning

**High-throughput scenario:**
```yaml
Proxy:
  PoolSize: 500           # Increase pool
  MaxConnectionsPerHost: 100
  ReadTimeout: 60s        # Longer timeout
  WriteTimeout: 60s
```

**Low-latency scenario:**
```yaml
Proxy:
  PoolSize: 50            # Smaller pool
  MaxConnectionsPerHost: 10
  ReadTimeout: 5s         # Shorter timeout
  WriteTimeout: 5s
```

---

## Step 10: Disaster Recovery

### Scenarios and Responses

| Scenario | Detection | Response | RTO |
|----------|-----------|----------|-----|
| Proxy crash | Health check fails | Restart service | < 1 min |
| MySQL down | Circuit breaker opens | Failover to replica | < 5 min |
| Redis down | Config errors | Use static config | < 1 min |
| Network partition | Connection timeouts | Investigate network | Varies |
| Data corruption | Data validation alerts | Stop writes, restore | < 30 min |

### Runbook: Proxy Crash
```bash
# 1. Check if process is running
ps aux | grep transisidb

# 2. Check recent logs
sudo journalctl -u transisidb -n 100

# 3. Restart service
sudo systemctl restart transisidb

# 4. Verify health
curl http://localhost:8080/health

# 5. Check metrics for anomalies
curl http://localhost:8080/metrics | grep error
```

---

## Post-Deployment Checklist

- [ ] Proxy is running and healthy
- [ ] Application connects successfully through proxy
- [ ] Shadow columns are being populated
- [ ] Metrics are being collected
- [ ] Alerts are configured and firing tests successfully
- [ ] Dashboards show expected values
- [ ] Logs are being centralized (ELK/Loki)
- [ ] Backup job is running
- [ ] Runbooks are documented and accessible
- [ ] Team is trained on operations

---

## Security Hardening

### Network Security
```bash
# Firewall rules (iptables)
# Allow only application servers to connect to proxy port
sudo iptables -A INPUT -p tcp --dport 3308 -s 10.0.1.0/24 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 3308 -j DROP

# Allow only ops team to access API
sudo iptables -A INPUT -p tcp --dport 8080 -s 10.0.0.0/16 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 8080 -j DROP
```

### Secrets Management
```bash
# Use Vault or AWS Secrets Manager
# Example with Vault:
export VAULT_ADDR=https://vault.prod.internal
vault kv get -field=password secret/transisidb/db > /tmp/db_password
export DB_PASSWORD=$(cat /tmp/db_password)
rm /tmp/db_password
```

### Regular Security Updates
```bash
# Weekly security scan
trivy image transisidb:latest

# Update base image monthly
docker pull alpine:latest
docker build --no-cache -t transisidb:latest .
```

---

Last Updated: 2025-11-21  
Version: 1.0
