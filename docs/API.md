# TransisiDB Management API

REST API untuk mengontrol dan memonitor TransisiDB proxy.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

Semua endpoint (kecuali `/health`) memerlukan API key:

```bash
Authorization: Bearer sk_dev_changeme
```

## Endpoints

### Health Check

**Public endpoint** - tidak perlu authentication

```http
GET /health
```

**Response:**
```json
{
  "status": "healthy",
  "redis": "healthy",
  "timestamp": 1700000000
}
```

---

### Configuration Management

#### Get Configuration

```http
GET /api/v1/config
```

**Response:**
```json
{
  "conversion": {
    "ratio": 1000,
    "precision": 4,
    "rounding_strategy": "BANKERS_ROUND"
  },
  "tables": {
    "orders": {
      "enabled": true,
      "columns": {...}
    }
  }
}
```

#### Update Configuration

```http
PUT /api/v1/config
Content-Type: application/json

{
  "conversion": {
    "ratio": 1000,
    "precision": 4
  }
}
```

**Response:**
```json
{
  "message": "Configuration updated successfully",
  "timestamp": 1700000000
}
```

#### Reload Configuration

Trigger hot-reload via Redis Pub/Sub

```http
POST /api/v1/config/reload
```

---

### Backfill Control

#### Get Backfill Status

```http
GET /api/v1/backfill/status
```

**Response:**
```json
{
  "table_name": "orders",
  "status": "running",
  "total_rows": 1000000,
  "completed_rows": 250000,
  "progress_percentage": 25.0,
  "rows_per_second": 833.5,
  "errors": 0,
  "start_time": "2025-11-20T10:00:00Z",
  "estimated_completion": "2025-11-20T10:30:00Z"
}
```

#### Pause Backfill

```http
POST /api/v1/backfill/pause
```

#### Resume Backfill

```http
POST /api/v1/backfill/resume
```

#### Stop Backfill

```http
POST /api/v1/backfill/stop
```

---

### Table Configuration

#### List Tables

```http
GET /api/v1/tables
```

**Response:**
```json
{
  "tables": ["orders", "invoices", "payments"],
  "count": 3
}
```

#### Get Table Configuration

```http
GET /api/v1/tables/orders
```

**Response:**
```json
{
  "enabled": true,
  "columns": {
    "total_amount": {
      "source_column": "total_amount",
      "target_column": "total_amount_idn",
      "source_type": "BIGINT",
      "target_type": "DECIMAL(19,4)",
      "rounding_strategy": "BANKERS_ROUND",
      "precision": 4
    }
  }
}
```

#### Update Table Configuration

```http
PUT /api/v1/tables/orders
Content-Type: application/json

{
  "enabled": true,
  "columns": {
    "total_amount": {...}
  }
}
```

#### Delete Table Configuration

```http
DELETE /api/v1/tables/orders
```

---

## Error Responses

Semua error menggunakan format standar:

```json
{
  "error": "Error message description"
}
```

**HTTP Status Codes:**
- `200` - OK
- `400` - Bad Request
- `401` - Unauthorized
- `404` - Not Found
- `500` - Internal Server Error
- `501` - Not Implemented

---

## Examples

### Using cURL

```bash
# Health check
curl http://localhost:8080/health

# Get configuration
curl -H "Authorization: Bearer sk_dev_changeme" \
     http://localhost:8080/api/v1/config

# Update table configuration
curl -X PUT \
     -H "Authorization: Bearer sk_dev_changeme" \
     -H "Content-Type: application/json" \
     -d '{"enabled": true, "columns": {...}}' \
     http://localhost:8080/api/v1/tables/orders

# Get backfill status
curl -H "Authorization: Bearer sk_dev_changeme" \
     http://localhost:8080/api/v1/backfill/status
```

### Time Travel Simulation Mode

Simulation mode diaktifkan melalui header khusus pada query database (bukan REST API):

```http
X-TransisiDB-Mode: SIMULATE_IDN
```

Response akan menampilkan nilai dalam format IDN (sudah terkonversi):

```json
{
  "data": [
    {
      "id": 123,
      "total_amount": 500.0000  // Converted from 500000
    }
  ],
  "_metadata": {
    "simulated": true,
    "currency": "IDN",
    "conversion_ratio": 1000
  }
}
```

---

## Running the API Server

```bash
# Start API server
./transisidb-api.exe --config config.yaml

# Or with Go
go run cmd/api/main.go --config config.yaml
```

Server akan listen di port yang dikonfigurasi (default: 8080).
