# TransisiDB Architecture Guide

## Overview

TransisiDB is a stateless MySQL proxy that sits between your application and MySQL database, transparently transforming queries to support dual-write currency migration.

---

## System Architecture

### High-Level Design

```
┌─────────────────────────────────────────────────────────────┐
│                     Application Layer                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   API Server │  │  Background  │  │   Cron Jobs  │      │
│  │              │  │    Workers   │  │              │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         │                 │                  │              │
│         └─────────────────┼──────────────────┘              │
│                           │                                 │
└───────────────────────────┼─────────────────────────────────┘
                            │ MySQL Protocol (Port 3308)
┌───────────────────────────▼─────────────────────────────────┐
│                  TransisiDB Proxy Layer                      │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              Session Handler                          │   │
│  │  ┌──────────────────┐  ┌──────────────────┐         │   │
│  │  │  Authentication  │  │  Connection Pool │         │   │
│  │  └──────────────────┘  └──────────────────┘         │   │
│  └──────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────┐   │
│  │           Query Processing Pipeline                   │   │
│  │  ┌────────┐  ┌────────┐  ┌────────┐  ┌──────────┐  │   │
│  │  │ Parser │→ │Analyzer│→ │Transform│→ │ Executor │  │   │
│  │  └────────┘  └────────┘  └────────┘  └──────────┘  │   │
│  └──────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────┐   │
│  │          Resilience & Monitoring                      │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────┐  │   │
│  │  │Circuit Breaker│ │    Metrics   │  │  Logger  │  │   │
│  │  └──────────────┘  └──────────────┘  └──────────┘  │   │
│  └──────────────────────────────────────────────────────┘   │
└───────────────────────────┬─────────────────────────────────┘
                            │
         ┌──────────────────┴──────────────────┐
         │                                     │
┌────────▼─────────┐                  ┌────────▼─────────┐
│  MySQL Backend   │                  │  Redis (Config)  │
│   Port 3307      │                  │   Port 6379      │
│                  │                  │                  │
│  ┌────────────┐  │                  │  ┌────────────┐  │
│  │   orders   │  │                  │  │Table Config│  │
│  │  invoices  │  │                  │  │Conversion  │  │
│  └────────────┘  │                  │  │  Rules     │  │
└──────────────────┘                  └──────────────────┘
```

---

## Component Details

### 1. Proxy Server (`internal/proxy/server.go`)

**Responsibilities:**
- TCP listener on port 3308
- Accept incoming MySQL connections
- Delegate to Session Handler
- Graceful shutdown handling

**Key Functions:**
```go
func NewServer(cfg *config.Config) *Server
func (s *Server) Start() error
func (s *Server) Stop()
```

**Lifecycle:**
1. Load configuration
2. Initialize connection pool
3. Start TCP listener
4. Accept connections in goroutines
5. Handle SIGTERM for graceful shutdown

---

### 2. Session Handler (`internal/proxy/session.go`)

**Responsibilities:**
- MySQL protocol handshake
- Authentication forwarding
- Command routing
- Query transformation
- Response proxying

**State Machine:**
```
┌──────────┐
│   NEW    │
└────┬─────┘
     │
     ▼
┌──────────────┐
│  HANDSHAKE   │ ← Forward handshake from backend
└────┬─────────┘
     │
     ▼
┌──────────────┐
│    AUTH      │ ← Forward auth response from backend
└────┬─────────┘
     │
     ▼
┌──────────────┐
│   READY      │ ← Process commands
└────┬─────────┘
     │
     ▼
┌──────────────┐
│   CLOSED     │
└──────────────┘
```

**Command Flow:**
```go
switch command {
case COM_QUERY:
    handleQuery()    // Transform if needed
case COM_STMT_PREPARE:
    handlePrepare()  // Forward multi-packet response
case COM_PING:
    forwardCommand() // Simple forward
case COM_INIT_DB:
    forwardCommand() // Database switch
case COM_QUIT:
    closeSession()   // Cleanup
}
```

---

### 3. Query Parser (`internal/parser/parser.go`)

**Responsibilities:**
- Parse SQL queries
- Identify query type (INSERT, UPDATE, SELECT, etc.)
- Extract table names
- Analyze column references

**Algorithm:**
```
Input: SQL string
↓
1. Tokenize
   - Split by keywords, operators
   - Identify literals, identifiers
↓
2. Parse Structure
   - Detect query type
   - Extract table name
   - Parse column list
↓
3. Build AST (Abstract Syntax Tree)
   - Table node
   - Column nodes
   - Value nodes
↓
Output: QueryInfo struct
```

**Example:**
```sql
-- Input
INSERT INTO orders (customer_id, total_amount) VALUES (1001, 50000000)

-- Parsed Output
QueryType: INSERT
Table: "orders"
Columns: ["customer_id", "total_amount"]
Values: [1001, 50000000]
```

---

### 4. Query Transformer (`internal/transformer/transformer.go`)

**Responsibilities:**
- Match columns against config
- Apply conversion formula
- Inject shadow column writes
- Preserve query semantics

**Transformation Rules:**

#### INSERT Transformation
```sql
-- Original
INSERT INTO orders (customer_id, total_amount, shipping_fee)
VALUES (1001, 50000000, 15000)

-- Check config for "orders.total_amount"
-- Config says: SourceColumn=total_amount, TargetColumn=total_amount_idn

-- Transform
INSERT INTO orders (customer_id, total_amount, total_amount_idn, shipping_fee, shipping_fee_idn)
VALUES (1001, 50000000, 50000000/1000, 15000, 15000/1000)

-- MySQL executes
INSERT INTO orders (customer_id, total_amount, total_amount_idn, shipping_fee, shipping_fee_idn)
VALUES (1001, 50000000, 50000.0000, 15000, 15.0000)
```

#### UPDATE Transformation
```sql
-- Original
UPDATE orders SET total_amount = 75000000 WHERE id = 1001

-- Transform
UPDATE orders 
SET total_amount = 75000000, 
    total_amount_idn = 75000000/1000 
WHERE id = 1001
```

**Banker's Rounding:**
```go
func bankerRound(value float64) float64 {
    // Round half to even
    // Examples:
    // 15.5 → 15 (nearest even)
    // 16.5 → 16 (nearest even)
    // 15.6 → 16 (standard round)
}
```

---

### 5. Circuit Breaker (`internal/proxy/circuit_breaker.go`)

**Pattern:** Sony Gobreaker

**States:**
```
      ┌─────────┐
 ┌────┤ CLOSED  ├────┐
 │    └─────────┘    │
 │                   │
 │ Success      5 Failures
 │                   │
 │    ┌─────────┐    │
 └────┤  OPEN   ├────┘
 │    └─────────┘    │
 │                   │
 │ Success      30s Timeout
 │                   │
 │    ┌─────────┐    │
 └────┤HALF-OPEN├────┘
      └─────────┘
```

**Configuration:**
```yaml
Proxy:
  CircuitBreaker:
    MaxFailures: 5        # Failures before opening
    Timeout: 30s          # Wait before trying HALF-OPEN
    MaxConcurrent: 10     # Max requests in HALF-OPEN
```

**Behavior:**

| State | On Request | On Success | On Failure |
|-------|------------|------------|------------|
| CLOSED | Forward | Continue | Count++ |
| OPEN | Reject immediately | N/A | N/A |
| HALF-OPEN | Forward (limited) | → CLOSED | → OPEN |

---

### 6. Connection Pool (`internal/pool/pool.go`)

**Design:**
- Pre-create connections to MySQL
- Reuse connections across client sessions
- TTL-based eviction
- Health checking

**Pool Operations:**
```go
type Pool struct {
    idle     chan *Connection
    active   map[*Connection]bool
    maxConns int
}

func (p *Pool) Acquire() (*Connection, error)
func (p *Pool) Release(conn *Connection)
func (p *Pool) Close()
```

**Connection Lifecycle:**
```
┌──────────┐
│   NEW    │
└────┬─────┘
     │ Connect to MySQL
     ▼
┌──────────┐
│   IDLE   │ ← Waiting in pool
└────┬─────┘
     │ Acquired by session
     ▼
┌──────────┐
│  ACTIVE  │ ← Processing queries
└────┬─────┘
     │ Released back
     ▼
┌──────────┐
│   IDLE   │
└────┬─────┘
     │ TTL expired or Close()
     ▼
┌──────────┐
│  CLOSED  │
└──────────┘
```

---

### 7. Configuration Manager (`internal/config/config.go`)

**Sources:**
1. `config.yaml` (primary)
2. Redis (runtime updates)
3. Environment variables (overrides)

**Configuration Schema:**
```go
type Config struct {
    Database   DatabaseConfig
    Proxy      ProxyConfig
    Redis      RedisConfig
    Conversion ConversionConfig
    Tables     map[string]TableConfig
    API        APIConfig
    Logging    LoggingConfig
}
```

**Hot Reload:**
```
User → POST /api/v1/config
        ↓
   Save to Redis
        ↓
   Broadcast update
        ↓
   Proxy reloads
```

---

### 8. Metrics Collector (`internal/metrics/metrics.go`)

**Prometheus Metrics:**

| Metric | Type | Description |
|--------|------|-------------|
| `transisidb_query_total` | Counter | Total queries processed |
| `transisidb_query_duration_seconds` | Histogram | Query latency distribution |
| `transisidb_connection_pool_active` | Gauge | Active connections |
| `transisidb_circuit_breaker_state` | Gauge | CB state (0=CLOSED, 1=OPEN, 2=HALF-OPEN) |
| `transisidb_errors_total` | Counter | Total errors by type |

**Instrumentation Points:**
```go
// On query start
metrics.QueryCounter.Inc()
timer := prometheus.NewTimer(metrics.QueryDuration)

// On query end
timer.ObserveDuration()

// On error
metrics.ErrorCounter.WithLabelValues("type").Inc()
```

---

### 9. Logger (`internal/logger/logger.go`)

**Format:** Structured JSON

**Levels:** DEBUG, INFO, WARN, ERROR, FATAL

**Example Output:**
```json
{
  "time": "2025-11-21T09:00:00+07:00",
  "level": "INFO",
  "message": "Query transformed",
  "query_type": "INSERT",
  "table": "orders",
  "columns_added": ["total_amount_idn", "shipping_fee_idn"],
  "conn_id": 42
}
```

---

## Data Flow

### INSERT Query Flow

```
1. Client → Proxy: INSERT INTO orders (customer_id, total_amount) VALUES (1001, 50000000)
              ↓
2. Session: Receive packet
              ↓
3. Parser: Extract table="orders", columns=["customer_id","total_amount"]
              ↓
4. Config Lookup: Check if "orders.total_amount" needs transformation
              ↓
5. Transformer: Add ", total_amount_idn" to columns
                Add ", 50000000/1000" to values
              ↓
6. Circuit Breaker: Check if backend is available
              ↓
7. Pool: Acquire connection
              ↓
8. Backend ← Proxy: Transformed query
              ↓
9. Backend → Proxy: OK packet (rows affected)
              ↓
10. Pool: Release connection
              ↓
11. Client ← Proxy: OK packet (unchanged)
```

---

## Failure Scenarios

### Scenario 1: Backend MySQL Down

```
1. Client connects to proxy
2. Proxy tries to connect to backend → FAILS
3. Circuit Breaker: Count failure (1/5)
4. Return error to client
5. After 5 failures → Circuit OPENS
6. Subsequent clients get immediate rejection
7. After 30s → Circuit goes HALF-OPEN
8. Next client tries → Success → Circuit CLOSES
```

### Scenario 2: Redis Configuration Unavailable

```
1. Proxy starts, tries to load config from Redis → FAILS
2. Fallback to config.yaml
3. Log warning: "Redis unavailable, using static config"
4. Proxy continues with file-based config
5. Hot-reload disabled until Redis recovers
```

### Scenario 3: Invalid Query Transformation

```
1. Client sends: INSERT INTO unknown_table ...
2. Parser: Identifies table="unknown_table"
3. Config Lookup: No rules for "unknown_table"
4. Transformer: Pass through unchanged
5. Backend processes normally
```

---

## Performance Considerations

### Latency Breakdown

| Operation | Typical Latency |
|-----------|-----------------|
| Query parsing | 50-100μs |
| Config lookup | 10-20μs |
| Transformation | 30-50μs |
| Network (proxy↔backend) | 0.5-1ms |
| **Total overhead** | **~1ms** |

### Optimization Strategies

1. **Connection Pooling**
   - Reuse connections
   - Avoid handshake overhead

2. **Query Parser**
   - Regex-based for simple cases
   - Full parser only when needed

3. **Configuration Cache**
   - Cache table configs in memory
   - Invalidate on updates

4. **Circuit Breaker**
   - Fail fast when backend down
   - Prevent cascading failures

---

## Security Model

### Authentication
- **Pass-through**: Proxy forwards credentials to MySQL
- **No storage**: Credentials never stored by proxy
- **API Key**: Management API uses separate authentication

### Authorization
- **MySQL ACL**: Database-level permissions enforced by MySQL
- **IP Whitelist**: Simulation mode restricted to trusted IPs

### Data Privacy
- **No query logging**: Queries not logged by default (configurable)
- **Encrypted transit**: TLS support planned for v2.0

---

## Scalability

### Horizontal Scaling
```
    ┌─────────┐
    │ HAProxy │ (Load Balancer)
    └────┬────┘
         │
    ┌────┴────┐
    │         │
┌───▼───┐ ┌──▼────┐
│Proxy 1│ │Proxy 2│
└───┬───┘ └──┬────┘
    │        │
    └───┬────┘
        │
    ┌───▼───┐
    │ MySQL │
    └───────┘
```

### Resource Limits

| Component | Limit | Configurable |
|-----------|-------|--------------|
| Max Connections | 100 | Yes (`Proxy.PoolSize`) |
| Max Query Size | 16MB | MySQL default |
| Memory per connection | ~50KB | N/A |
| **Total memory** | **~50MB + 5MB per conn** | - |

---

## Deployment Topologies

### Topology 1: Sidecar Pattern
```
┌────────────────┐
│  App Server    │
│  ┌──────────┐  │
│  │   App    │  │
│  └────┬─────┘  │
│       │        │
│  ┌────▼─────┐  │
│  │  Proxy   │  │ ← Same host
│  └────┬─────┘  │
└───────┼────────┘
        │
    ┌───▼───┐
    │ MySQL │
    └───────┘
```

### Topology 2: Dedicated Proxy Layer
```
 ┌─────┐  ┌─────┐  ┌─────┐
 │App 1│  │App 2│  │App 3│
 └──┬──┘  └──┬──┘  └──┬──┘
    │        │        │
    └────────┼────────┘
             │
      ┌──────▼──────┐
      │Proxy Cluster│ ← Dedicated fleet
      └──────┬──────┘
             │
         ┌───▼───┐
         │ MySQL │
         └───────┘
```

---

## Monitoring Strategy

### Key Metrics to Watch

1. **Throughput**
   - `rate(transisidb_query_total[1m])`
   - Expected: Matches application QPS

2. **Latency**
   - `histogram_quantile(0.99, transisidb_query_duration_seconds)`
   - SLO: p99 < 5ms

3. **Error Rate**
   - `rate(transisidb_errors_total[5m])`
   - SLO: < 0.1%

4. **Circuit Breaker**
   - `transisidb_circuit_breaker_state`
   - Alert if OPEN for > 1 minute

5. **Connection Pool**
   - `transisidb_connection_pool_active / transisidb_connection_pool_max`
   - Alert if > 90%

---

## Future Enhancements

### Planned Features

1. **Read Replica Support**
   - Route SELECT to replicas
   - Write to primary only

2. **Query Caching**
   - Cache SELECT results
   - TTL-based invalidation

3. **Sharding Support**
   - Partition by customer_id
   - Auto-routing

4. **TLS/SSL**
   - Encrypted client→proxy
   - Encrypted proxy→backend

---

## References

- MySQL Protocol: https://dev.mysql.com/doc/dev/mysql-server/latest/PAGE_PROTOCOL.html
- Circuit Breaker Pattern: https://martinfowler.com/bliki/CircuitBreaker.html
- Banker's Rounding: IEEE 754 Standard

---

Last Updated: 2025-11-21  
Version: 1.0
