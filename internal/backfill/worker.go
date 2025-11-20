package backfill

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/transisidb/transisidb/internal/config"
	"github.com/transisidb/transisidb/internal/rounding"
)

// Worker handles background data migration
type Worker struct {
	db             *sql.DB
	config         *config.BackfillConfig
	conversionCfg  *config.ConversionConfig
	roundingEngine *rounding.Engine

	// State
	running  atomic.Bool
	paused   atomic.Bool
	progress *Progress

	// Control channels
	pauseCh  chan struct{}
	resumeCh chan struct{}
	stopCh   chan struct{}
}

// NewWorker creates a new backfill worker
func NewWorker(db *sql.DB, cfg *config.Config) *Worker {
	return &Worker{
		db:            db,
		config:        &cfg.Backfill,
		conversionCfg: &cfg.Conversion,
		roundingEngine: rounding.NewEngine(
			rounding.Strategy(cfg.Conversion.RoundingStrategy),
			cfg.Conversion.Precision,
		),
		progress: NewProgress(),
		pauseCh:  make(chan struct{}),
		resumeCh: make(chan struct{}),
		stopCh:   make(chan struct{}),
	}
}

// Start begins the backfill process for a table
func (w *Worker) Start(ctx context.Context, tableName string, tableConfig config.TableConfig) error {
	if !w.running.CompareAndSwap(false, true) {
		return fmt.Errorf("worker already running")
	}
	defer w.running.Store(false)

	w.progress.Start(tableName)

	// Count total rows to migrate
	totalRows, err := w.countPendingRows(ctx, tableName, tableConfig)
	if err != nil {
		return fmt.Errorf("failed to count rows: %w", err)
	}
	w.progress.SetTotal(totalRows)

	if totalRows == 0 {
		w.progress.Complete()
		return nil
	}

	// Process in batches
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.stopCh:
			return nil
		case <-w.pauseCh:
			// Wait for resume
			<-w.resumeCh
		default:
			// Process next batch
			processed, err := w.processBatch(ctx, tableName, tableConfig)
			if err != nil {
				w.progress.IncrementErrors()

				// Retry logic
				if w.shouldRetry() {
					time.Sleep(time.Duration(w.config.RetryBackoffMs) * time.Millisecond)
					continue
				}
				return fmt.Errorf("batch processing failed: %w", err)
			}

			if processed == 0 {
				// No more rows to process
				w.progress.Complete()
				return nil
			}

			w.progress.IncrementCompleted(int64(processed))

			// Throttle to avoid overloading database
			time.Sleep(time.Duration(w.config.SleepIntervalMs) * time.Millisecond)
		}
	}
}

// processBatch processes a batch of rows
func (w *Worker) processBatch(ctx context.Context, tableName string, tableConfig config.TableConfig) (int, error) {
	// Build query to select batch of rows without converted values
	columns := make([]string, 0, len(tableConfig.Columns))
	for colName := range tableConfig.Columns {
		columns = append(columns, colName)
	}

	// For simplicity, we'll process the first currency column
	// In production, you'd handle all columns
	var firstColumn string
	var firstConfig config.ColumnConfig
	for col, cfg := range tableConfig.Columns {
		firstColumn = col
		firstConfig = cfg
		break
	}

	if firstColumn == "" {
		return 0, fmt.Errorf("no currency columns configured")
	}

	// Query for rows where shadow column is NULL
	query := fmt.Sprintf(
		`SELECT id, %s FROM %s WHERE %s IS NULL LIMIT %d`,
		firstColumn,
		tableName,
		firstConfig.TargetColumn,
		w.config.BatchSize,
	)

	rows, err := w.db.QueryContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to query batch: %w", err)
	}
	defer rows.Close()

	processed := 0
	for rows.Next() {
		var id int64
		var value int64

		if err := rows.Scan(&id, &value); err != nil {
			return processed, fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert value
		convertedValue := w.roundingEngine.ConvertIDRtoIDN(value, w.conversionCfg.Ratio)

		// Update row
		updateQuery := fmt.Sprintf(
			`UPDATE %s SET %s = ? WHERE id = ?`,
			tableName,
			firstConfig.TargetColumn,
		)

		_, err := w.db.ExecContext(ctx, updateQuery, convertedValue, id)
		if err != nil {
			return processed, fmt.Errorf("failed to update row %d: %w", id, err)
		}

		processed++
	}

	if err := rows.Err(); err != nil {
		return processed, fmt.Errorf("row iteration error: %w", err)
	}

	return processed, nil
}

// countPendingRows counts how many rows still need migration
func (w *Worker) countPendingRows(ctx context.Context, tableName string, tableConfig config.TableConfig) (int64, error) {
	// Get first currency column
	var firstConfig config.ColumnConfig
	for _, cfg := range tableConfig.Columns {
		firstConfig = cfg
		break
	}

	query := fmt.Sprintf(
		`SELECT COUNT(*) FROM %s WHERE %s IS NULL`,
		tableName,
		firstConfig.TargetColumn,
	)

	var count int64
	err := w.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// shouldRetry determines if we should retry after an error
func (w *Worker) shouldRetry() bool {
	return w.progress.errors < int64(w.config.RetryAttempts)
}

// Pause pauses the backfill worker
func (w *Worker) Pause() error {
	if !w.running.Load() {
		return fmt.Errorf("worker not running")
	}
	if w.paused.CompareAndSwap(false, true) {
		w.pauseCh <- struct{}{}
		return nil
	}
	return fmt.Errorf("worker already paused")
}

// Resume resumes the backfill worker
func (w *Worker) Resume() error {
	if !w.running.Load() {
		return fmt.Errorf("worker not running")
	}
	if w.paused.CompareAndSwap(true, false) {
		w.resumeCh <- struct{}{}
		return nil
	}
	return fmt.Errorf("worker not paused")
}

// Stop stops the backfill worker
func (w *Worker) Stop() {
	if w.running.Load() {
		w.stopCh <- struct{}{}
	}
}

// GetProgress returns current progress
func (w *Worker) GetProgress() *Progress {
	return w.progress
}

// IsRunning returns whether worker is running
func (w *Worker) IsRunning() bool {
	return w.running.Load()
}

// IsPaused returns whether worker is paused
func (w *Worker) IsPaused() bool {
	return w.paused.Load()
}
