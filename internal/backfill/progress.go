package backfill

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Progress tracks backfill progress
type Progress struct {
	mu sync.RWMutex

	tableName     string
	totalRows     int64
	completedRows int64
	errors        int64
	startTime     time.Time
	endTime       *time.Time
	status        Status
}

// Status represents backfill status
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// NewProgress creates a new progress tracker
func NewProgress() *Progress {
	return &Progress{
		status: StatusPending,
	}
}

// Start marks the backfill as started
func (p *Progress) Start(tableName string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.tableName = tableName
	p.startTime = time.Now()
	p.status = StatusRunning
}

// SetTotal sets the total number of rows
func (p *Progress) SetTotal(total int64) {
	atomic.StoreInt64(&p.totalRows, total)
}

// IncrementCompleted increments completed rows count
func (p *Progress) IncrementCompleted(count int64) {
	atomic.AddInt64(&p.completedRows, count)
}

// IncrementErrors increments error count
func (p *Progress) IncrementErrors() {
	atomic.AddInt64(&p.errors, 1)
}

// Complete marks the backfill as completed
func (p *Progress) Complete() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	p.endTime = &now
	p.status = StatusCompleted
}

// Fail marks the backfill as failed
func (p *Progress) Fail() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	p.endTime = &now
	p.status = StatusFailed
}

// Pause marks the backfill as paused
func (p *Progress) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = StatusPaused
}

// Resume marks the backfill as running
func (p *Progress) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = StatusRunning
}

// GetSnapshot returns a snapshot of current progress
func (p *Progress) GetSnapshot() *Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total := atomic.LoadInt64(&p.totalRows)
	completed := atomic.LoadInt64(&p.completedRows)
	errors := atomic.LoadInt64(&p.errors)

	var percentage float64
	if total > 0 {
		percentage = float64(completed) / float64(total) * 100
	}

	var rowsPerSecond float64
	var eta *time.Time
	if p.status == StatusRunning && completed > 0 {
		elapsed := time.Since(p.startTime).Seconds()
		rowsPerSecond = float64(completed) / elapsed

		if rowsPerSecond > 0 {
			remaining := total - completed
			etaSeconds := float64(remaining) / rowsPerSecond
			etaTime := time.Now().Add(time.Duration(etaSeconds) * time.Second)
			eta = &etaTime
		}
	}

	return &Snapshot{
		TableName:           p.tableName,
		Status:              p.status,
		TotalRows:           total,
		CompletedRows:       completed,
		Errors:              errors,
		ProgressPercentage:  percentage,
		RowsPerSecond:       rowsPerSecond,
		StartTime:           p.startTime,
		EndTime:             p.endTime,
		EstimatedCompletion: eta,
	}
}

// Snapshot represents a point-in-time snapshot of progress
type Snapshot struct {
	TableName           string     `json:"table_name"`
	Status              Status     `json:"status"`
	TotalRows           int64      `json:"total_rows"`
	CompletedRows       int64      `json:"completed_rows"`
	Errors              int64      `json:"errors"`
	ProgressPercentage  float64    `json:"progress_percentage"`
	RowsPerSecond       float64    `json:"rows_per_second"`
	StartTime           time.Time  `json:"start_time"`
	EndTime             *time.Time `json:"end_time,omitempty"`
	EstimatedCompletion *time.Time `json:"estimated_completion,omitempty"`
}

// String returns a human-readable representation
func (s *Snapshot) String() string {
	if s.Status == StatusCompleted {
		duration := s.EndTime.Sub(s.StartTime)
		return fmt.Sprintf("Table: %s | Status: %s | Completed: %d/%d (100%%) | Duration: %s",
			s.TableName, s.Status, s.CompletedRows, s.TotalRows, duration.Round(time.Second))
	}

	eta := "calculating..."
	if s.EstimatedCompletion != nil {
		eta = s.EstimatedCompletion.Format("15:04:05")
	}

	return fmt.Sprintf("Table: %s | Status: %s | Progress: %d/%d (%.1f%%) | Speed: %.0f rows/sec | ETA: %s | Errors: %d",
		s.TableName, s.Status, s.CompletedRows, s.TotalRows, s.ProgressPercentage,
		s.RowsPerSecond, eta, s.Errors)
}
