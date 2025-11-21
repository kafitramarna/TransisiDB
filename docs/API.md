# TransisiDB Management API Documentation

Base URL: `http://localhost:8080`

## Authentication

All `/api/v1/*` endpoints require API Key authentication.

**Header:**
```
Authorization: Bearer YOUR_API_KEY
```

**Example:**
```bash
curl -H "Authorization: Bearer sk_dev_changeme" \
  http://localhost:8080/api/v1/config
```

---

## Endpoints

### Health & Metrics

#### GET /health
Check proxy health status.

**No authentication required**

**Response:**
```json
{
  "status": "healthy",
  "timestamp": 1700000000,
  "redis": "healthy"
}
```

#### GET /metrics
Prometheus metrics endpoint.

**No authentication required**

**Response:** Prometheus text format
```
# HELP transisidb_query_total Total number of queries
# TYPE transisidb_query_total counter
transisidb_query_total{type="INSERT"} 42
transisidb_query_total{type="UPDATE"} 15
```

---

### Configuration Management

#### GET /api/v1/config
Get current configuration.

**Request:**
```bash
curl -H "Authorization: Bearer sk_dev_changeme" \
  http://localhost:8080/api/v1/config
```

**Response:**
```json
{
  "Database": {
    "Host": "localhost",
    "Port": 3307,
    "User": "root",
    "Password": "",
    "Database": "ecommerce_db",
    "Type": "mysql",
    "MaxConnections": 100,
    "IdleConnections": 10,
    "ConnectionTimeout": 30000000000
  },
  "Proxy": {
    "Host": "0.0.0.0",
    "Port": 3308,
    "PoolSize": 100,
    "MaxConnectionsPerHost": 50,
    "ReadTimeout": 30000000000,
    "WriteTimeout": 30000000000
  },
  "Conversion": {
    "Ratio": 1000,
    "Precision": 4,
    "RoundingStrategy": "BANKERS_ROUND"
  },
  "Tables": {
    "orders": {
      "Enabled": true,
      "Columns": {
        "total_amount": {
          "SourceColumn": "total_amount",
          "SourceType": "BIGINT",
          "TargetColumn": "total_amount_idn",
          "TargetType": "DECIMAL(19,4)",
          "Precision": 4,
          "RoundingStrategy": "BANKERS_ROUND"
        }
      }
    }
  }
}
```

#### PUT /api/v1/config
Update configuration (hot reload).

**Request:**
```bash
curl -X PUT \
  -H "Authorization: Bearer sk_dev_changeme" \
  -H "Content-Type: application/json" \
  -d @new_config.json \
  http://localhost:8080/api/v1/config
```

**Request Body:**
```json
{
  "Conversion": {
    "Ratio": 1000,
    "Precision": 4
  },
  "Tables": {
    "orders": {
      "Enabled": true,
      "Columns": {
        "total_amount": {
          "SourceColumn": "total_amount",
          "TargetColumn": "total_amount_idn",
          "Precision": 4
        }
      }
    }
  }
}
```

**Response:**
```json
{
  "message": "Configuration updated successfully",
  "reloaded": true
}
```

---

### Table Management

#### GET /api/v1/tables
List all configured tables.

**Request:**
```bash
curl -H "Authorization: Bearer sk_dev_changeme" \
  http://localhost:8080/api/v1/tables
```

**Response:**
```json
{
  "tables": ["orders", "invoices"],
  "count": 2
}
```

#### GET /api/v1/tables/:name
Get configuration for specific table.

**Request:**
```bash
curl -H "Authorization: Bearer sk_dev_changeme" \
  http://localhost:8080/api/v1/tables/orders
```

**Response:**
```json
{
  "Enabled": true,
  "Columns": {
    "total_amount": {
      "SourceColumn": "total_amount",
      "SourceType": "BIGINT",
      "TargetColumn": "total_amount_idn",
      "TargetType": "DECIMAL(19,4)",
      "Precision": 4,
      "RoundingStrategy": "BANKERS_ROUND"
    },
    "shipping_fee": {
      "SourceColumn": "shipping_fee",
      "SourceType": "INT",
      "TargetColumn": "shipping_fee_idn",
      "TargetType": "DECIMAL(12,4)",
      "Precision": 4,
      "RoundingStrategy": "BANKERS_ROUND"
    }
  }
}
```

#### PUT /api/v1/tables/:name
Update table configuration.

**Request:**
```bash
curl -X PUT \
  -H "Authorization: Bearer sk_dev_changeme" \
  -H "Content-Type: application/json" \
  -d '{
    "Enabled": true,
    "Columns": {
      "total_amount": {
        "SourceColumn": "total_amount",
        "TargetColumn": "total_amount_idn",
        "Precision": 4,
        "RoundingStrategy": "BANKERS_ROUND"
      }
    }
  }' \
  http://localhost:8080/api/v1/tables/orders
```

**Response:**
```json
{
  "message": "Table configuration updated",
  "table": "orders"
}
```

#### DELETE /api/v1/tables/:name
Disable table transformation.

**Request:**
```bash
curl -X DELETE \
  -H "Authorization: Bearer sk_dev_changeme" \
  http://localhost:8080/api/v1/tables/orders
```

**Response:**
```json
{
  "message": "Table transformation disabled",
  "table": "orders"
}
```

---

### Backfill Management

#### POST /api/v1/backfill/start
Start backfill process to populate shadow columns.

**Request:**
```bash
curl -X POST \
  -H "Authorization: Bearer sk_dev_changeme" \
  -H "Content-Type: application/json" \
  -d '{
    "table": "orders",
    "batch_size": 1000,
    "start_id": 0,
    "end_id": 100000
  }' \
  http://localhost:8080/api/v1/backfill/start
```

**Response:**
```json
{
  "job_id": "bf_20251121_123456",
  "status": "running",
  "table": "orders",
  "progress": {
    "current_id": 0,
    "total_estimated": 100000,
    "percentage": 0
  }
}
```

#### GET /api/v1/backfill/status/:job_id
Get backfill job status.

**Request:**
```bash
curl -H "Authorization: Bearer sk_dev_changeme" \
  http://localhost:8080/api/v1/backfill/status/bf_20251121_123456
```

**Response:**
```json
{
  "job_id": "bf_20251121_123456",
  "status": "running",
  "table": "orders",
  "progress": {
    "current_id": 45000,
    "total_estimated": 100000,
    "percentage": 45,
    "rows_processed": 45000,
    "rows_updated": 45000,
    "errors": 0
  },
  "started_at": "2025-11-21T10:00:00Z",
  "estimated_completion": "2025-11-21T10:15:00Z"
}
```

#### POST /api/v1/backfill/stop/:job_id
Stop running backfill job.

**Request:**
```bash
curl -X POST \
  -H "Authorization: Bearer sk_dev_changeme" \
  http://localhost:8080/api/v1/backfill/stop/bf_20251121_123456
```

**Response:**
```json
{
  "job_id": "bf_20251121_123456",
  "status": "stopped",
  "message": "Backfill job stopped gracefully"
}
```

---

## Error Responses

All endpoints return errors in consistent format:

```json
{
  "error": "Error message",
  "code": "ERROR_CODE",
  "details": {}
}
```

### HTTP Status Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 201 | Created |
| 400 | Bad Request - Invalid input |
| 401 | Unauthorized - Missing or invalid API key |
| 404 | Not Found - Resource doesn't exist |
| 500 | Internal Server Error |
| 503 | Service Unavailable - Backend down |

### Error Codes

| Code | Description |
|------|-------------|
| `INVALID_API_KEY` | API key is missing or invalid |
| `INVALID_REQUEST` | Request body is malformed |
| `TABLE_NOT_FOUND` | Table configuration not found |
| `BACKEND_UNAVAILABLE` | MySQL backend is unavailable |
| `CONFIG_ERROR` | Configuration update failed |
| `BACKFILL_RUNNING` | Another backfill job is already running |

**Example Error Response:**
```json
{
  "error": "Table configuration not found",
  "code": "TABLE_NOT_FOUND",
  "details": {
    "table": "unknown_table"
  }
}
```

---

## Rate Limiting

API endpoints are rate-limited:
- **100 requests/minute** per API key
- **Burst**: 10 requests

**Headers:**
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1700000060
```

**429 Response:**
```json
{
  "error": "Rate limit exceeded",
  "code": "RATE_LIMIT_EXCEEDED",
  "retry_after": 60
}
```

---

## SDK Examples

### Go Client

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

const (
    baseURL = "http://localhost:8080"
    apiKey  = "sk_dev_changeme"
)

type Client struct {
    baseURL string
    apiKey  string
    http    *http.Client
}

func New Client() *Client {
    return &Client{
        base URL: baseURL,
        apiKey:  apiKey,
        http:    &http.Client{},
    }
}

func (c *Client) doRequest(method, path string, body interface{}) (*http.Response, error) {
    var req *http.Request
    var err error
    
    if body != nil {
        jsonBody, _ := json.Marshal(body)
        req, err = http.NewRequest(method, c.baseURL+path, bytes.NewBuffer(jsonBody))
        req.Header.Set("Content-Type", "application/json")
    } else {
        req, err = http.NewRequest(method, c.baseURL+path, nil)
    }
    
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("Authorization", "Bearer "+c.apiKey)
    return c.http.Do(req)
}

func (c *Client) GetConfig() (map[string]interface{}, error) {
    resp, err := c.doRequest("GET", "/api/v1/config", nil)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var config map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&config)
    return config, nil
}

func (c *Client) GetTable(name string) (map[string]interface{}, error) {
    resp, err := c.doRequest("GET", "/api/v1/tables/"+name, nil)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var table map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&table)
    return table, nil
}

// Usage
func main() {
    client := NewClient()
    
    config, err := client.GetConfig()
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Config: %+v\n", config)
}
```

### Python Client

```python
import requests

class TransisiDBClient:
    def __init__(self, base_url="http://localhost:8080", api_key="sk_dev_changeme"):
        self.base_url = base_url
        self.api_key = api_key
        self.headers = {"Authorization": f"Bearer {api_key}"}
    
    def get_config(self):
        response = requests.get(
            f"{self.base_url}/api/v1/config",
            headers=self.headers
        )
        response.raise_for_status()
        return response.json()
    
    def get_table(self, name):
        response = requests.get(
            f"{self.base_url}/api/v1/tables/{name}",
            headers=self.headers
        )
        response.raise_for_status()
        return response.json()
    
    def start_backfill(self, table, batch_size=1000, start_id=0, end_id=None):
        payload = {
            "table": table,
            "batch_size": batch_size,
            "start_id": start_id,
            "end_id": end_id
        }
        response = requests.post(
            f"{self.base_url}/api/v1/backfill/start",
            headers=self.headers,
            json=payload
        )
        response.raise_for_status()
        return response.json()
    
    def get_backfill_status(self, job_id):
        response = requests.get(
            f"{self.base_url}/api/v1/backfill/status/{job_id}",
            headers=self.headers
        )
        response.raise_for_status()
        return response.json()

# Usage
client = TransisiDBClient()

# Get configuration
config = client.get_config()
print(config)

# Start backfill
job = client.start_backfill("orders", batch_size=1000)
print(f"Job started: {job['job_id']}")

# Check status
status = client.get_backfill_status(job['job_id'])
print(f"Progress: {status['progress']['percentage']}%")
```

### JavaScript/Node.js Client

```javascript
const axios = require('axios');

class TransisiDBClient {
  constructor(baseURL = 'http://localhost:8080', apiKey = 'sk_dev_changeme') {
    this.client = axios.create({
      baseURL,
      headers: {
        'Authorization': `Bearer ${apiKey}`,
        'Content-Type': 'application/json'
      }
    });
  }

  async getConfig() {
    const response = await this.client.get('/api/v1/config');
    return response.data;
  }

  async getTable(name) {
    const response = await this.client.get(`/api/v1/tables/${name}`);
    return response.data;
  }

  async startBackfill(table, options = {}) {
    const payload = {
      table,
      batch_size: options.batchSize || 1000,
      start_id: options.startId || 0,
      end_id: options.endId || null
    };
    const response = await this.client.post('/api/v1/backfill/start', payload);
    return response.data;
  }

  async getBackfillStatus(jobId) {
    const response = await this.client.get(`/api/v1/backfill/status/${jobId}`);
    return response.data;
  }
}

// Usage
(async () => {
  const client = new TransisiDBClient();
  
  const config = await client.getConfig();
  console.log('Config:', config);
  
  const job = await client.startBackfill('orders', { batchSize: 1000 });
  console.log('Job started:', job.job_id);
  
  const status = await client.getBackfillStatus(job.job_id);
  console.log(`Progress: ${status.progress.percentage}%`);
})();
```

---

## Webhooks (Coming in v2.0)

Subscribe to events:
- `config.updated` - Configuration changed
- `backfill.completed` - Backfill job finished
- `circuit_breaker.opened` - Backend failure detected
- `circuit_breaker.closed` - Backend recovered

---

Last Updated: 2025-11-21  
Version: 1.0
