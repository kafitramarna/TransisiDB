# TransisiDB Configuration Reference

Complete reference for all configuration options in `config.yaml`.

---

## Configuration File Structure

```yaml
Database:        # MySQL connection settings
Proxy:           # Proxy server settings
Redis:           # Redis connection for config storage
Conversion:      # Currency conversion rules
Tables:          # Per-table transformation rules
API:             # Management API settings
Backfill:        # Backfill job settings
Simulation:      # Time-travel simulation mode
Monitoring:      # Prometheus/metrics settings
Logging:         # Log configuration
```

---

## Database Configuration

MySQL backend connection settings.

```yaml
Database:
  Type: mysql                    # Database type (currently only mysql)
  Host: localhost                # MySQL host
  Port: 3307                     # MySQL port
  User: root                     # MySQL username
  Password: secret               # MySQL password (use env var in production)
  Database: ecommerce_db         # Database name
  MaxConnections: 100            # Maximum connections to MySQL
  IdleConnections: 10            # Idle connections to keep
  ConnectionTimeout: 30s         # Connection timeout duration
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Type` | string | `mysql` | Database type |
| `Host` | string | `localhost` | MySQL server hostname |
| `Port` | int | `3306` | MySQL server port |
| `User` | string | -  | MySQL username |
| `Password` | string | - | MySQL password |
| `Database` | string | - | Database name |
| `MaxConnections` | int | `100` | Max concurrent connections |
| `IdleConnections` | int | `10` | Min idle connections in pool |
| `ConnectionTimeout` | duration | `30s` | Timeout for new connections |

### Environment Variables

```bash
# Override via env vars
export DB_HOST=mysql.prod.internal
export DB_PORT=3306
export DB_USER=proxy_user
export DB_PASSWORD=secure_password
export DB_NAME=ecommerce_db
```

---

## Proxy Configuration

TransisiDB proxy server settings.

```yaml
Proxy:
  Host: 0.0.0.0                  # Bind address (0.0.0.0 = all interfaces)
  Port: 3308                     # Proxy listen port
  PoolSize: 100                  # Connection pool size
  MaxConnectionsPerHost: 50      # Max connections per client host
  ReadTimeout: 30s               # Read timeout
  WriteTimeout: 30s              # Write timeout
  CircuitBreaker:
    MaxFailures: 5               # Failures before opening circuit
    Timeout: 30s                 # Time to wait before trying HALF-OPEN
    MaxConcurrent: 10            # Max concurrent requests in HALF-OPEN
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Host` | string | `0.0.0.0` | Bind address |
| `Port` | int | `3308` | Listen port |
| `PoolSize` | int | `100` | Backend connection pool size |
| `MaxConnectionsPerHost` | int | `50` | Per-client connection limit |
| `ReadTimeout` | duration | `30s` | Socket read timeout |
| `WriteTimeout` | duration | `30s` | Socket write timeout |

### Circuit Breaker Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `MaxFailures` | int | `5` | Consecutive failures to trigger OPEN |
| `Timeout` | duration | `30s` | Wait time before HALF-OPEN |
| `MaxConcurrent` | int | `10` | Max requests in HALF-OPEN state |

---

## Redis Configuration

Redis connection for storing runtime configuration.

```yaml
Redis:
  Host: localhost                # Redis host
  Port: 6379                     # Redis port
  Password: ""                   # Redis password (empty for no auth)
  Database: 0                    # Redis database number (0-15)
  PoolSize: 10                   # Connection pool size
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Host` | string | `localhost` | Redis server hostname |
| `Port` | int | `6379` | Redis server port |
| `Password` | string | `""` | Redis password (empty = no auth) |
| `Database` | int | `0` | Redis database number |
| `PoolSize` | int | `10` | Connection pool size |

---

## Conversion Configuration

Global currency conversion settings.

```yaml
Conversion:
  Ratio: 1000                    # Conversion ratio (IDR to IDN)
  Precision: 4                   # Decimal precision
  RoundingStrategy: BANKERS_ROUND  # Rounding algorithm
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Ratio` | int | `1000` | Division ratio for conversion |
| `Precision` | int | `4` | Decimal places in target column |
| `RoundingStrategy` | string | `BANKERS_ROUND` | Rounding algorithm |

### Rounding Strategies

| Strategy | Description | Example |
|----------|-------------|---------|
| `BANKERS_ROUND` | Round half to even (IEEE 754) | 15.5 → 15, 16.5 → 16 |
| `ROUND_UP` | Always round up | 15.5 → 16, 16.5 → 17 |
| `ROUND_DOWN` | Always round down | 15.5 → 15, 16.5 → 16 |
| `ROUND_HALF_UP` | Standard rounding | 15.5 → 16, 16.5 → 17 |

---

## Tables Configuration

Per-table transformation rules.

```yaml
Tables:
  table_name:
    Enabled: true                # Enable transformation for this table
    Columns:
      column_name:
        SourceColumn: column_name      # Source column name
        SourceType: BIGINT             # Source data type
        TargetColumn: column_name_idn  # Target (shadow) column name
        TargetType: DECIMAL(19,4)      # Target data type
        Precision: 4                   # Decimal precision (overrides global)
        RoundingStrategy: BANKERS_ROUND # Rounding (overrides global)
```

### Example: Orders Table

```yaml
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
        Target Type: DECIMAL(12,4)
        Precision: 4
        RoundingStrategy: BANKERS_ROUND
```

### Column Options

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `SourceColumn` | string | Yes | Original column name |
| `SourceType` | string | Yes | MySQL data type (BIGINT, INT, etc.) |
| `TargetColumn` | string | Yes | Shadow column name |
| `TargetType` | string | Yes | Shadow column MySQL type |
| `Precision` | int | No | Decimal places (default: global) |
| `RoundingStrategy` | string | No | Rounding method (default: global) |

---

## API Configuration

Management REST API settings.

```yaml
API:
  Host: 0.0.0.0                  # API bind address
  Port: 8080                     # API listen port
  APIKey: sk_dev_changeme        # API authentication key
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Host` | string | `0.0.0.0` | Bind address for API server |
| `Port` | int | `8080` | API listen port |
| `APIKey` | string | - | Secret key for API authentication |

### Generating Secure API Key

```bash
# Generate random API key
openssl rand -hex 32
# Output: sk_prod_a1b2c3d4e5f6...

# Or use uuidgen
echo "sk_prod_$(uuidgen)"
```

---

## Backfill Configuration

Settings for backfill jobs to populate shadow columns.

```yaml
Backfill:
  Enabled: false                 # Enable backfill feature
  BatchSize: 1000                # Rows per batch
  SleepIntervalMs: 100           # Sleep between batches (ms)
  MaxCPUPercent: 20              # Max CPU usage %
  RetryAttempts: 3               # Retries on failure
  RetryBackoffMs: 500            # Backoff between retries (ms)
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Enabled` | bool | `false` | Enable backfill API endpoints |
| `BatchSize` | int | `1000` | Number of rows to process per batch |
| `SleepIntervalMs` | int | `100` | Milliseconds to sleep between batches |
| `MaxCPUPercent` | int | `20` | Target max CPU usage (throttling) |
| `RetryAttempts` | int | `3` | Number of retries on error |
| `RetryBackoffMs` | int | `500` | Milliseconds between retries |

---

## Simulation Configuration

Time-travel / simulation mode for testing.

```yaml
Simulation:
  Enabled: false                 # Enable simulation mode
  AllowedIPs:                    # IP whitelist for simulation
    - 127.0.0.1
    - 10.0.0.0/8
    - 192.168.0.0/16
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Enabled` | bool | `false` | Enable simulation mode |
| `AllowedIPs` | []string | `[]` | IP addresses/ranges allowed to use simulation |

### Usage

In simulation mode, clients can send custom headers to override behavior:

```sql
-- Use connection with headers:
-- X-Simulate-Time: 2024-01-01T00:00:00Z
-- X-Simulate-Currency: IDN

-- Proxy will return IDN values for all queries
SELECT total_amount_idn FROM orders;
```

---

## Monitoring Configuration

Prometheus and metrics settings.

```yaml
Monitoring:
  PrometheusEnabled: true        # Enable Prometheus metrics
  PrometheusPort: 9090           # Prometheus scrape port
  MetricsPath: /metrics          # Metrics endpoint path
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `PrometheusEnabled` | bool | `true` | Enable Prometheus metrics export |
| `PrometheusPort` | int | `9090` | Port for Prometheus (deprecated, uses API port) |
| `MetricsPath` | string | `/metrics` | HTTP path for metrics endpoint |

---

## Logging Configuration

Structured logging settings.

```yaml
Logging:
  Level: INFO                    # Log level
  Format: json                   # Log format
  Output: stdout                 # Log output destination
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Level` | string | `INFO` | Minimum log level to output |
| `Format` | string | `json` | Log format (json or text) |
| `Output` | string | `stdout` | Output destination (stdout, stderr, or file path) |

### Log Levels

| Level | Description | Use Case |
|-------|-------------|----------|
| `DEBUG` | Detailed debug information | Development, troubleshooting |
| `INFO` | Informational messages | Production (default) |
| `WARN` | Warning messages | Production |
| `ERROR` | Error messages | Production |
| `FATAL` | Fatal errors (exits) | Production |

---

## Complete Example

```yaml
# Production configuration

Database:
  Type: mysql
  Host: mysql-primary.prod.internal
  Port: 3306
  User: transisidb_user
  Password: "${DB_PASSWORD}"      # From environment
  Database: ecommerce_db
  MaxConnections: 200
  IdleConnections: 20
  ConnectionTimeout: 30s

Proxy:
  Host: 0.0.0.0
  Port: 3308
  PoolSize: 200
  MaxConnectionsPerHost: 100
  ReadTimeout: 60s
  WriteTimeout: 60s
  CircuitBreaker:
    MaxFailures: 5
    Timeout: 30s
    MaxConcurrent: 20

Redis:
  Host: redis.prod.internal
  Port: 6379
  Password: "${REDIS_PASSWORD}"
  Database: 0
  PoolSize: 20

Conversion:
  Ratio: 1000
  Precision: 4
  RoundingStrategy: BANKERS_ROUND

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

API:
  Host: 0.0.0.0
  Port: 8080
  APIKey: "${API_KEY}"            # From environment

Backfill:
  Enabled: true
  BatchSize: 5000
  SleepIntervalMs: 50
  MaxCPUPercent: 30
  RetryAttempts: 3
  RetryBackoffMs: 1000

Simulation:
  Enabled: false
  AllowedIPs: []

Monitoring:
  PrometheusEnabled: true
  PrometheusPort: 9090
  MetricsPath: /metrics

Logging:
  Level: INFO
  Format: json
  Output: /var/log/transisidb/proxy.log
```

---

## Environment Variable Substitution

Config values can reference environment variables using `${VAR_NAME}` syntax.

**Example:**
```yaml
Database:
  Password: "${DB_PASSWORD}"
  Host: "${DB_HOST:-localhost}"  # With default value
```

**Set environment:**
```bash
export DB_PASSWORD=my_secure_password
export DB_HOST=mysql.prod.internal
```

---

## Validation

Validate configuration before starting:

```bash
# Dry-run validation
./transisidb -config config.yaml -validate

# Output:
# ✓ Configuration is valid
# ✓ Can connect to MySQL
# ✓ Can connect to Redis
# ✓ All tables exist in database
# ✓ All shadow columns exist
```

---

## Hot Reload

Change configuration without restarting:

```bash
# Update via API
curl -X PUT \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d @new_config.json \
  http://localhost:8080/api/v1/config

# Or send SIGHUP signal
kill -HUP $(pidof transisidb)
```

---

Last Updated: 2025-11-21  
Version: 1.0
