# TransisiDB - Intelligent Currency Redenomination Proxy

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-21%2F21%20passing-brightgreen)](docs/TESTING.md)

> **Zero-downtime database proxy for currency redenomination with intelligent dual-write capabilities**

TransisiDB is a production-ready MySQL proxy that enables seamless currency migration from Indonesian Rupiah (IDR) to Indonesian Rupiah Denominated (IDN) with a 1:1000 ratio. It performs real-time query transformation, dual-write operations, and maintains full ACID compliance.

## 🚀 Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.21 or higher
- MySQL 8.0+
- Redis 7+

### Installation

```bash
# Clone repository
git clone https://github.com/kafitramarna/TransisiDB.git
cd TransisiDB

# Start infrastructure
docker-compose up -d mysql redis

# Initialize database
docker exec transisidb-mysql mysql -u root -psecret < scripts/init.sql

# Start proxy
go run cmd/proxy/main.go

# Start Management API (optional)
go run cmd/api/main.go
```

### Connect Your Application

```go
import "database/sql"
import _ "github.com/go-sql-driver/mysql"

// Connect through proxy with dual-write enabled
dsn := "root:secret@tcp(localhost:3308)/ecommerce_db?parseTime=true&interpolateParams=true"
db, err := sql.Open("mysql", dsn)
```

### Verify It Works

```bash
# Run integration tests
go run cmd/test_proxy/main.go

# View database contents
go run cmd/view_rows/main.go
```

---

## ✨ Features

### Core Capabilities
- ✅ **Dual-Write Transformation** - Automatically converts and writes to shadow columns
- ✅ **Transaction Support** - Full ACID compliance with COMMIT/ROLLBACK
- ✅ **Banker's Rounding** - IEEE 754 compliant rounding algorithm
- ✅ **Circuit Breaker** - Automatic fault detection and recovery
- ✅ **Connection Pooling** - Efficient resource management
- ✅ **Query Rewriting** - Real-time SQL transformation

### MySQL Protocol Support
- ✅ COM_QUERY (text protocol)
- ✅ COM_STMT_PREPARE (prepared statements)
- ✅ COM_PING (health checks)
- ✅ COM_INIT_DB (database switching)
- ✅ COM_QUIT (graceful disconnect)

### Monitoring & Observability
- 📊 Prometheus metrics export
- 🏥 Health check endpoints
- 📈 Query duration histograms
- 🔍 Connection pool statistics
- 📝 Structured JSON logging

### Management API
- 🔧 Configuration hot-reload
- 📋 Table management
- 🔄 Backfill control
- 📊 Real-time metrics

---

## 📚 Documentation

| Document | Description |
|----------|-------------|
| [Architecture Guide](docs/ARCHITECTURE.md) | System design and components |
| [Deployment Guide](docs/DEPLOYMENT.md) | Production deployment steps |
| [Configuration Reference](docs/CONFIGURATION.md) | All configuration options |
| [API Documentation](docs/API.md) | Management REST API |
| [Testing Guide](docs/TESTING.md) | Running all test suites |
| [Troubleshooting](docs/TROUBLESHOOTING.md) | Common issues and solutions |
| [Contributing](CONTRIBUTING.md) | Development guidelines |

---

## 🏗️ Architecture

```
┌─────────────┐
│ Application │
└──────┬──────┘
       │ MySQL Protocol
       ▼
┌──────────────────────────────────────────┐
│         TransisiDB Proxy :3308           │
│  ┌────────────────────────────────────┐  │
│  │  Query Parser & Transformer        │  │
│  │  - Detect currency columns         │  │
│  │  - Apply conversion (÷1000)        │  │
│  │  - Add shadow column writes        │  │
│  └────────────────────────────────────┘  │
│  ┌────────────────────────────────────┐  │
│  │  Circuit Breaker                   │  │
│  │  - Fault detection                 │  │
│  │  - Auto-recovery                   │  │
│  └────────────────────────────────────┘  │
└──────────────┬───────────────────────────┘
               │
       ┌───────┴────────┐
       ▼                ▼
┌─────────────┐  ┌─────────────┐
│ MySQL :3307 │  │ Redis :6379 │
│ (Primary)   │  │ (Config)    │
└─────────────┘  └─────────────┘
```

### How It Works

1. **Application connects** to TransisiDB Proxy (port 3308)
2. **Proxy intercepts** MySQL queries
3. **Query parser** identifies INSERT/UPDATE with currency columns
4. **Transformer** adds dual-write for shadow columns (IDN)
5. **Circuit breaker** protects against backend failures
6. **Query forwarded** to MySQL backend
7. **Results returned** to application unchanged

---

## 💡 Use Cases

### Use Case 1: New Order Creation
```sql
-- Application sends:
INSERT INTO orders (customer_id, total_amount, shipping_fee) 
VALUES (1001, 50000000, 15000);

-- Proxy transforms to:
INSERT INTO orders (customer_id, total_amount, total_amount_idn, shipping_fee, shipping_fee_idn) 
VALUES (1001, 50000000, 50000.0000, 15000, 15.0000);
```

### Use Case 2: Order Update
```sql
-- Application sends:
UPDATE orders 
SET total_amount = 75000000 
WHERE id = 1001;

-- Proxy transforms to:
UPDATE orders 
SET total_amount = 75000000, total_amount_idn = 75000.0000 
WHERE id = 1001;
```

### Use Case 3: Transaction Handling
```sql
-- Application sends:
BEGIN;
INSERT INTO orders (...) VALUES (...);
UPDATE invoices SET ... WHERE ...;
COMMIT;

-- Proxy transforms both queries and maintains transaction boundary
```

---

## 📊 Performance

| Metric | Value | Notes |
|--------|-------|-------|
| Proxy Overhead | ~0.5-1ms | Minimal latency impact |
| Throughput | 10K+ QPS | Single instance |
| Connection Pool | 100 max | Configurable |
| Circuit Breaker Latency | <2ms | Fast-fail when open |
| Memory Footprint | ~50MB | Idle state |

Tested on: Intel i7-9700K, 16GB RAM, NVMe SSD

---

## 🔒 Security

- ✅ MySQL authentication pass-through
- ✅ API key authentication for Management API
- ✅ IP whitelist for simulation mode
- ✅ Secure configuration via Redis
- ⚠️ TLS/SSL support: Planned for v2.0

---

## 🧪 Testing

All 21 test cases pass with 100% success rate:

```bash
# Full test suite
go run cmd/test_proxy/main.go       # 7 integration tests
go run cmd/test_manual/main.go      # 5 manual tests
go run cmd/test_circuit_breaker/main.go  # Circuit breaker
go run cmd/test_metrics/main.go     # Metrics validation
```

See [Testing Guide](docs/TESTING.md) for details.

---

## 🛠️ Configuration

Minimal `config.yaml`:

```yaml
Database:
  Host: localhost
  Port: 3307
  User: root
  Password: secret
  Database: ecommerce_db

Proxy:
  Host: 0.0.0.0
  Port: 3308
  PoolSize: 100

Conversion:
  Ratio: 1000           # IDR to IDN
  Precision: 4          # Decimal places
  RoundingStrategy: BANKERS_ROUND

Tables:
  orders:
    Enabled: true
    Columns:
      total_amount:
        SourceColumn: total_amount
        TargetColumn: total_amount_idn
        SourceType: BIGINT
        TargetType: DECIMAL(19,4)
```

See [Configuration Reference](docs/CONFIGURATION.md) for all options.

---

## 🚦 Production Deployment

### System Requirements
- **CPU**: 2+ cores
- **RAM**: 4GB minimum, 8GB recommended
- **Network**: Low latency to MySQL backend (<5ms)
- **MySQL**: 8.0+ with shadow columns created

### Deployment Steps

1. **Prepare Database**
```sql
ALTER TABLE orders ADD COLUMN total_amount_idn DECIMAL(19,4);
ALTER TABLE orders ADD COLUMN shipping_fee_idn DECIMAL(12,4);
```

2. **Deploy Proxy**
```bash
# Build binary
go build -o transisidb cmd/proxy/main.go

# Run with config
./transisidb -config /etc/transisidb/config.yaml
```

3. **Update Application DSN**
```go
// Old: Direct MySQL
dsn := "user:pass@tcp(mysql:3306)/db"

// New: Through proxy
dsn := "user:pass@tcp(proxy:3308)/db?interpolateParams=true"
```

4. **Monitor**
```bash
curl http://proxy:8080/health
curl http://proxy:8080/metrics
```

See [Deployment Guide](docs/DEPLOYMENT.md) for detailed steps.

---

## 📈 Monitoring

### Prometheus Metrics
```promql
# Circuit breaker state
transisidb_circuit_breaker_state

# Query throughput
rate(transisidb_query_duration_seconds_count[1m])

# Error rate
rate(transisidb_errors_total[5m])

# Connection pool usage
transisidb_connection_pool_active / transisidb_connection_pool_max
```

### Grafana Dashboard
Import dashboard from `monitoring/grafana-dashboard.json` (coming soon)

---

## 🤝 Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Setup
```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Run linter
golangci-lint run

# Start dev environment
docker-compose up -d
go run cmd/proxy/main.go -config config.yaml
```

---

## 📝 License

MIT License - see [LICENSE](LICENSE) file for details.

---

## 🙏 Acknowledgments

- MySQL Protocol implementation inspired by [go-mysql](https://github.com/go-mysql-org/go-mysql)
- Circuit breaker pattern from [Sony Gobreaker](https://github.com/sony/gobreaker)
- Prometheus integration using [client_golang](https://github.com/prometheus/client_golang)

---

## 📞 Support

- 📧 Email: kafitra.marna@gmail.com
<!-- - 💬 Slack: [Join our community](https://transisidb.slack.com) -->
- 🐛 Issues: [GitHub Issues](https://github.com/kafitramarna/TransisiDB/issues)
- 📖 Wiki: [GitHub Wiki](https://github.com/kafitramarna/TransisiDB/wiki)

---

## 🗺️ Roadmap

### v1.0 (Current)
- ✅ Core dual-write functionality
- ✅ Circuit breaker
- ✅ Basic monitoring

### v2.0 (Planned)
- 🔲 TLS/SSL support
- 🔲 Read replica support
- 🔲 Query caching
- 🔲 Advanced backfill strategies

### v3.0 (Future)
- 🔲 Multi-database support (PostgreSQL)
- 🔲 Sharding support
- 🔲 Built-in load balancing

---

Made with ❤️ by the TransisiDB Team
