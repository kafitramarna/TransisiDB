package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/kafitramarna/TransisiDB/internal/config"
)

// Pool manages database connections
type Pool struct {
	db     *sql.DB
	config *config.DatabaseConfig
}

// NewPool creates a new database connection pool
func NewPool(cfg *config.DatabaseConfig) (*Pool, error) {
	dsn := getDSN(cfg)

	db, err := sql.Open(cfg.Type, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxConnections)
	db.SetMaxIdleConns(cfg.IdleConnections)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectionTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Pool{
		db:     db,
		config: cfg,
	}, nil
}

// Query executes a SELECT query
func (p *Pool) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return p.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row
func (p *Pool) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return p.db.QueryRowContext(ctx, query, args...)
}

// Exec executes a query without returning rows
func (p *Pool) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return p.db.ExecContext(ctx, query, args...)
}

// Begin starts a new transaction
func (p *Pool) Begin(ctx context.Context) (*sql.Tx, error) {
	return p.db.BeginTx(ctx, nil)
}

// BeginTx starts a new transaction with options
func (p *Pool) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return p.db.BeginTx(ctx, opts)
}

// Close closes all connections in the pool
func (p *Pool) Close() error {
	return p.db.Close()
}

// Stats returns database statistics
func (p *Pool) Stats() sql.DBStats {
	return p.db.Stats()
}

// Ping checks if the database is reachable
func (p *Pool) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

// Health checks database health
func (p *Pool) Health(ctx context.Context) error {
	// Ping the database
	if err := p.Ping(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Check stats
	stats := p.Stats()
	if stats.OpenConnections == 0 {
		return fmt.Errorf("no open connections")
	}

	return nil
}

// GetDB returns the underlying *sql.DB (use with caution)
func (p *Pool) GetDB() *sql.DB {
	return p.db
}

// getDSN generates database connection string
func getDSN(cfg *config.DatabaseConfig) string {
	switch cfg.Type {
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Asia%%2FJakarta",
			cfg.User,
			cfg.Password,
			cfg.Host,
			cfg.Port,
			cfg.Database,
		)
	case "postgresql":
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Jakarta",
			cfg.Host,
			cfg.Port,
			cfg.User,
			cfg.Password,
			cfg.Database,
		)
	default:
		return ""
	}
}
