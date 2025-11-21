package dualwrite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kafitramarna/TransisiDB/internal/config"
	"github.com/kafitramarna/TransisiDB/internal/parser"
)

// Orchestrator manages dual-write operations with bidirectional conversion (v2.0)
type Orchestrator struct {
	db        *sql.DB
	parser    *parser.Parser
	converter *Converter // v2.0: Bidirectional converter
	config    *config.Config
}

// NewOrchestrator creates a new dual-write orchestrator
func NewOrchestrator(db *sql.DB, cfg *config.Config) *Orchestrator {
	return &Orchestrator{
		db:        db,
		parser:    parser.NewParser(cfg.Tables),
		converter: NewConverter(cfg), // v2.0: Use new converter
		config:    cfg,
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

	// v2.0: Detect conversion direction
	direction, err := o.converter.DetectDirection(pq)
	if err != nil {
		return "", fmt.Errorf("failed to detect conversion direction: %w", err)
	}

	// v2.0: Convert based on detected direction
	convertedValues, err := o.converter.ConvertValues(pq, direction)
	if err != nil {
		return "", fmt.Errorf("failed to convert values: %w", err)
	}

	// Skip rewrite if no conversion needed
	if direction == DirectionNone || convertedValues == nil {
		return query, nil
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
