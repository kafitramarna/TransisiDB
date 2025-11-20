package dualwrite

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/transisidb/transisidb/internal/config"
	"github.com/transisidb/transisidb/internal/parser"
	"github.com/transisidb/transisidb/internal/rounding"
)

// Orchestrator manages dual-write operations
type Orchestrator struct {
	db             *sql.DB
	parser         *parser.Parser
	roundingEngine *rounding.Engine
	config         *config.Config
}

// NewOrchestrator creates a new dual-write orchestrator
func NewOrchestrator(db *sql.DB, cfg *config.Config) *Orchestrator {
	return &Orchestrator{
		db:     db,
		parser: parser.NewParser(cfg.Tables),
		roundingEngine: rounding.NewEngine(
			rounding.Strategy(cfg.Conversion.RoundingStrategy),
			cfg.Conversion.Precision,
		),
		config: cfg,
	}
}

// InterceptAndRewrite intercepts a query and rewrites it for dual-write if needed
func (o *Orchestrator) InterceptAndRewrite(query string) (string, error) {
	// Parse the query
	pq, err := o.parser.Parse(query)
	if err != nil {
		return "", fmt.Errorf("failed to parse query: %w", err)
	}

	// If transformation is not needed, return original query
	if !pq.NeedsTransform {
		return query, nil
	}

	// Extract and convert values
	convertedValues, err := o.convertCurrencyValues(pq)
	if err != nil {
		return "", fmt.Errorf("failed to convert values: %w", err)
	}

	// Rewrite query to include shadow columns
	rewritten, err := o.parser.RewriteForDualWrite(pq, convertedValues)
	if err != nil {
		return "", fmt.Errorf("failed to rewrite query: %w", err)
	}

	return rewritten, nil
}

// ExecuteWithDualWrite executes a query with dual-write transformation
func (o *Orchestrator) ExecuteWithDualWrite(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	// Rewrite the query
	rewritten, err := o.InterceptAndRewrite(query)
	if err != nil {
		// If rewrite fails, we can either:
		// 1. Fail-safe: execute original query (lose dual-write)
		// 2. Fail-closed: return error (safer for data consistency)
		// We choose fail-closed approach
		return nil, fmt.Errorf("dual-write rewrite failed: %w", err)
	}

	// Execute the rewritten query within a transaction for atomicity
	tx, err := o.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Execute query
	result, err := tx.ExecContext(ctx, rewritten, args...)
	if err != nil {
		// Rollback on error
		if rbErr := tx.Rollback(); rbErr != nil {
			return nil, fmt.Errorf("query execution failed and rollback failed: %w (rollback: %v)", err, rbErr)
		}
		return nil, fmt.Errorf("query execution failed: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
}

// QueryWithDualWrite executes a SELECT query (no transformation needed, but kept for consistency)
func (o *Orchestrator) QueryWithDualWrite(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	// For SELECT queries, we typically don't transform
	// Transformation happens in response for simulation mode
	return o.db.QueryContext(ctx, query, args...)
}

// convertCurrencyValues converts IDR values to IDN for all currency columns
func (o *Orchestrator) convertCurrencyValues(pq *parser.ParsedQuery) (map[string]float64, error) {
	converted := make(map[string]float64)

	for _, colName := range pq.CurrencyColumns {
		// Get the value from parsed query
		value, exists := pq.Values[colName]
		if !exists {
			continue
		}

		// Convert to int64
		var intValue int64
		switch v := value.(type) {
		case int64:
			intValue = v
		case int:
			intValue = int64(v)
		case string:
			// Parse string to int
			parsed, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse value for column %s: %w", colName, err)
			}
			intValue = parsed
		default:
			return nil, fmt.Errorf("unsupported value type for column %s: %T", colName, v)
		}

		// Convert using rounding engine
		convertedValue := o.roundingEngine.ConvertIDRtoIDN(intValue, o.config.Conversion.Ratio)
		converted[colName] = convertedValue
	}

	return converted, nil
}

// Stats tracks dual-write statistics
type Stats struct {
	TotalQueries       int64
	TransformedQueries int64
	SuccessfulWrites   int64
	FailedWrites       int64
	TotalErrors        int64
}

// GetStats returns current statistics (placeholder for metrics)
func (o *Orchestrator) GetStats() Stats {
	// TODO: Implement actual metrics tracking
	return Stats{}
}
