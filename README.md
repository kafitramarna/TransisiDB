# TransisiDB

**Intelligent Database Proxy for Currency Redenomination**

TransisiDB adalah middleware database proxy yang dirancang untuk menangani migrasi tipe data mata uang dari representasi integer (BIGINT/INTEGER) ke desimal (DECIMAL) secara zero-downtime dalam konteks redenominasi mata uang Rupiah Indonesia 2027.

## Features

- ✅ **Zero-Downtime Migration**: Migrasi tipe data tanpa table locking atau aplikasi downtime
- ✅ **Dual-Write Atomik**: Menulis ke kolom lama dan baru dalam satu transaksi
- ✅ **Banker's Rounding**: Pembulatan sesuai standar IEEE 754 untuk compliance
- ✅ **Intelligent Backfill**: Worker background untuk migrasi data historis
- ✅ **Time Travel Mode**: Simulasi nilai redenominasi untuk testing
- ✅ **High Performance**: Latency overhead < 10ms, throughput 10K+ TPS

## Quick Start

### Prerequisites

- Go 1.21+
- Redis 7.0+
- MySQL 5.7+ atau PostgreSQL 11+
- Docker (optional, untuk development)

### Installation

```bash
# Clone repository
git clone https://github.com/kafitramarna/TransisiDB.git
cd TransisiDB

# Install dependencies
go mod download

# Build
go build -o transisidb cmd/proxy/main.go

# Run
./transisidb --config config.yaml
```

### Configuration

```yaml
# config.yaml
database:
  host: localhost
  port: 3306
  user: root
  password: secret
  database: ecommerce_db

proxy:
  port: 3307
  pool_size: 100

redis:
  host: localhost
  port: 6379

conversion:
  ratio: 1000
  precision: 4
  rounding_strategy: BANKERS_ROUND
```

## Architecture

```
[Application] → [TransisiDB Proxy] → [MySQL/PostgreSQL]
                        ↓
                  [Redis Config]
```

Lihat [SRS_TransisiDB.md](./SRS_TransisiDB.md) untuk dokumentasi lengkap.

## Development

### Project Structure

```
transisidb/
├── cmd/
│   ├── proxy/          # Main proxy service
│   ├── backfill/       # Backfill worker
│   └── api/            # Management API
├── internal/
│   ├── api/            # API server implementation
│   ├── backfill/       # Backfill worker logic
│   ├── config/         # Configuration management
│   ├── database/       # Database connection pool
│   ├── dualwrite/      # Dual-write orchestrator
│   ├── metrics/        # Prometheus metrics collectors
│   ├── parser/         # SQL parser
│   ├── rounding/       # Banker's rounding engine
│   └── simulation/     # Time travel simulation
├── scripts/            # Utility scripts (init, cleanup)
├── prometheus/         # Prometheus configuration
├── configs/            # Configuration files
└── docs/               # Documentation
```

### Running Tests

```bash
go test ./...
```

### Running with Docker

```bash
docker-compose up -d
```

## Roadmap

- [x] Project Setup
- [ ] Core Proxy Engine (Week 3-6)
- [x] Backfill Worker (Week 7-8)
- [x] Time Travel Feature (Week 9-10)
- [x] Monitoring & Testing (Week 11-12)

Lihat [Roadmap lengkap](./SRS_TransisiDB.md#8-roadmap-pengembangan) untuk detail.

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](./CONTRIBUTING.md) first.

## License

MIT License - see [LICENSE](./LICENSE) for details.

## Dokumentasi

- [System Requirements Specification](./SRS_TransisiDB.md)
- [API Documentation](./docs/API.md)
- [Deployment Guide](./docs/DEPLOYMENT.md) (Coming soon)

## Contact

- GitHub: [@kafitramarna](https://github.com/kafitramarna)
- Email: kafitra.marna@gmail.com

---

**⚠️ Status**: Currently in active development (MVP Phase)
