# Circuit Breaker Test Guide

## Automated Test
Run the automated test script:
```bash
go run cmd/test_circuit_breaker/main.go
```

The script will guide you through:
1. Phase 1: Verify normal operation
2. Phase 2: Simulate backend failure (you'll stop MySQL)
3. Phase 3: Test recovery (you'll restart MySQL)

## Manual Test (Alternative)

### Phase 1: Verify Normal Operation
```bash
# Test connection works
mysql -h 127.0.0.1 -P 3308 -u root -psecret ecommerce_db -e "SELECT 1"
```

### Phase 2: Backend Failure Simulation
```bash
# Terminal 1: Stop MySQL
docker-compose stop mysql

# Terminal 2: Try to connect (should fail quickly after circuit opens)
mysql -h 127.0.0.1 -P 3308 -u root -psecret ecommerce_db
# Try multiple times to trigger circuit breaker
```

Expected proxy logs:
```json
{"level":"ERROR","message":"Backend connection failed"}
{"level":"WARN","message":"Circuit breaker opened","failures":5}
{"level":"WARN","message":"Circuit breaker is OPEN, rejecting connection"}
```

### Phase 3: Recovery Test
```bash
# Terminal 1: Start MySQL
docker-compose start mysql

# Wait 35 seconds for circuit breaker timeout
# Circuit breaker config: timeout = 30s

# Terminal 2: Try to connect again
mysql -h 127.0.0.1 -P 3308 -u root -psecret ecommerce_db -e "SELECT 1"
```

Expected proxy logs:
```json
{"level":"INFO","message":"Circuit breaker transitioned to HALF-OPEN"}
{"level":"INFO","message":"Backend connection successful"}
{"level":"INFO","message":"Circuit breaker closed"}
```

## Circuit Breaker Configuration
From `config.yaml`:
```yaml
Proxy:
  CircuitBreaker:
    MaxFailures: 5          # Open after 5 failures
    Timeout: 30s            # Wait 30s before trying HALF-OPEN
    MaxConcurrent: 10       # Max concurrent requests in HALF-OPEN
```

## State Transitions
```
CLOSED ──(5 failures)──> OPEN ──(30s timeout)──> HALF-OPEN ──(success)──> CLOSED
   │                       │                         │
   │                       └──(reject)───────────────┘
   └──────────────────────────────────────────────────┘
```
