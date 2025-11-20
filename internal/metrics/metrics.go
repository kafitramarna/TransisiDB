package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics for TransisiDB
var (
	// DualWriteTotal counts total dual-write operations
	DualWriteTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "transisidb_dual_write_total",
			Help: "Total number of dual-write operations",
		},
		[]string{"status"}, // labels: success, error
	)

	// QueryDuration tracks query execution time
	QueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "transisidb_query_duration_seconds",
			Help:    "Query execution duration in seconds",
			Buckets: prometheus.DefBuckets, // [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]
		},
		[]string{"operation"}, // labels: insert, update, select, delete
	)

	// BackfillProgress tracks backfill completion percentage
	BackfillProgress = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "transisidb_backfill_progress",
			Help: "Backfill progress percentage (0-100)",
		},
		[]string{"table"},
	)

	// ConnectionPoolActive tracks active database connections
	ConnectionPoolActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "transisidb_connection_pool_active",
			Help: "Number of active database connections",
		},
	)

	// ErrorsTotal counts errors by type
	ErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "transisidb_errors_total",
			Help: "Total number of errors by type",
		},
		[]string{"type"}, // labels: database, parsing, conversion, etc.
	)

	// APIRequestsTotal counts API requests
	APIRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "transisidb_api_requests_total",
			Help: "Total number of API requests",
		},
		[]string{"endpoint", "method", "status"},
	)

	// BackfillRowsProcessed counts total rows processed during backfill
	BackfillRowsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "transisidb_backfill_rows_processed_total",
			Help: "Total number of rows processed during backfill",
		},
		[]string{"table"},
	)

	// BackfillErrors counts backfill errors
	BackfillErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "transisidb_backfill_errors_total",
			Help: "Total number of backfill errors",
		},
		[]string{"table"},
	)
)

// Helper functions for common operations

// RecordDualWrite records a dual-write operation
func RecordDualWrite(success bool) {
	if success {
		DualWriteTotal.WithLabelValues("success").Inc()
	} else {
		DualWriteTotal.WithLabelValues("error").Inc()
	}
}

// RecordQueryDuration records query execution time
func RecordQueryDuration(operation string, durationSeconds float64) {
	QueryDuration.WithLabelValues(operation).Observe(durationSeconds)
}

// SetBackfillProgress sets backfill progress percentage
func SetBackfillProgress(table string, percentage float64) {
	BackfillProgress.WithLabelValues(table).Set(percentage)
}

// RecordBackfillRow increments backfill row counter
func RecordBackfillRow(table string) {
	BackfillRowsProcessed.WithLabelValues(table).Inc()
}

// RecordBackfillError increments backfill error counter
func RecordBackfillError(table string) {
	BackfillErrors.WithLabelValues(table).Inc()
}

// SetConnectionPoolActive sets active connection count
func SetConnectionPoolActive(count int) {
	ConnectionPoolActive.Set(float64(count))
}

// RecordError records an error by type
func RecordError(errorType string) {
	ErrorsTotal.WithLabelValues(errorType).Inc()
}

// RecordAPIRequest records an API request
func RecordAPIRequest(endpoint, method, status string) {
	APIRequestsTotal.WithLabelValues(endpoint, method, status).Inc()
}
