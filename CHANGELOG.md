# Changelog

All notable changes to TransisiDB will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial project release
- SQL query parser with currency column detection
- Dual-write engine with atomic transactions
- Banker's Rounding implementation (IEEE 754)
- Background backfill worker with progress tracking
- Redis configuration store with hot-reload
- REST API with 15+ endpoints
- Time Travel simulation mode
- Comprehensive documentation (SRS, API docs, testing guides)
- Docker Compose development environment
- Unit tests with 100% passing rate

### Technical Specifications
- Go 1.21+ support
- MySQL 5.7+ and PostgreSQL 11+ compatibility
- Redis 7+ for configuration management
- Production-ready error handling
- Graceful shutdown mechanisms

## [0.1.0] - 2025-11-20

### Added
- Phase 1: Project foundation and Banker's Rounding engine
- Phase 2: SQL parser and dual-write orchestrator
- Phase 3: Redis configuration store and backfill worker
- Phase 4: Management REST API and Time Travel mode

### Features
- Zero-downtime migration capability
- Financial compliance (IEEE 754 rounding)
- Configuration hot-reload via Redis Pub/Sub
- Real-time progress tracking with ETA
- Comprehensive testing suite (40+ tests)

---

## Project Milestones

- **2025-11-20**: Initial development complete (Phases 1-4)
  - Core features implemented
  - Documentation finalized
  - Testing infrastructure established
  - Ready for GitHub publish

---

[Unreleased]: https://github.com/yourusername/transisidb/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/yourusername/transisidb/releases/tag/v0.1.0
