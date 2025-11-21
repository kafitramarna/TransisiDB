package replica

import (
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// QueryType represents the type of SQL query
type QueryType int

const (
	QueryTypeRead  QueryType = iota // SELECT queries
	QueryTypeWrite                  // INSERT, UPDATE, DELETE queries
)

// Config holds replica routing configuration
type Config struct {
	Primary  DatabaseConfig   `yaml:"primary"`
	Replicas []DatabaseConfig `yaml:"replicas"`
	Strategy string           `yaml:"strategy"` // ROUND_ROBIN, LEAST_CONNECTIONS, RANDOM
}

// DatabaseConfig represents a database connection configuration
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

// Router manages connections to primary and replica databases
type Router struct {
	primary       *sql.DB
	replicas      []*sql.DB
	replicaIndex  int
	strategy      string
	healthChecker *HealthChecker
	mu            sync.RWMutex
}

// NewRouter creates a new replica router
func NewRouter(cfg *Config) (*Router, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	router := &Router{
		replicas: make([]*sql.DB, 0),
		strategy: cfg.Strategy,
	}

	// Connect to primary
	primaryDSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		cfg.Primary.User,
		cfg.Primary.Password,
		cfg.Primary.Host,
		cfg.Primary.Port,
		cfg.Primary.Database,
	)

	primary, err := sql.Open("mysql", primaryDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to primary: %w", err)
	}
	router.primary = primary

	// Connect to replicas
	for i, replicaCfg := range cfg.Replicas {
		replicaDSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
			replicaCfg.User,
			replicaCfg.Password,
			replicaCfg.Host,
			replicaCfg.Port,
			replicaCfg.Database,
		)

		replica, err := sql.Open("mysql", replicaDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to replica %d: %w", i, err)
		}
		router.replicas = append(router.replicas, replica)
	}

	// Initialize health checker
	router.healthChecker = NewHealthChecker(router.primary, router.replicas)

	// Set default strategy if not specified
	if router.strategy == "" {
		router.strategy = "ROUND_ROBIN"
	}

	return router, nil
}

// GetConnection returns appropriate database connection based on query type
func (r *Router) GetConnection(queryType QueryType) (*sql.DB, error) {
	switch queryType {
	case QueryTypeWrite:
		return r.primary, nil
	case QueryTypeRead:
		return r.getReadConnection()
	default:
		return nil, fmt.Errorf("unknown query type: %d", queryType)
	}
}

// getReadConnection returns a read replica connection using the configured strategy
func (r *Router) getReadConnection() (*sql.DB, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// If no replicas, fallback to primary
	if len(r.replicas) == 0 {
		return r.primary, nil
	}

	// Filter healthy replicas
	healthyReplicas := r.healthChecker.GetHealthyReplicas()
	if len(healthyReplicas) == 0 {
		// No healthy replicas, fallback to primary
		return r.primary, nil
	}

	// Select replica based on strategy
	switch r.strategy {
	case "ROUND_ROBIN":
		replica := healthyReplicas[r.replicaIndex%len(healthyReplicas)]
		r.replicaIndex++
		return replica, nil

	case "RANDOM":
		idx := time.Now().UnixNano() % int64(len(healthyReplicas))
		return healthyReplicas[idx], nil

	case "LEAST_CONNECTIONS":
		// For simplicity, using round-robin
		// In production, track active connections per replica
		replica := healthyReplicas[r.replicaIndex%len(healthyReplicas)]
		r.replicaIndex++
		return replica, nil

	default:
		// Default to round-robin
		replica := healthyReplicas[r.replicaIndex%len(healthyReplicas)]
		r.replicaIndex++
		return replica, nil
	}
}

// Close closes all database connections
func (r *Router) Close() error {
	var errors []error

	// Close primary
	if err := r.primary.Close(); err != nil {
		errors = append(errors, fmt.Errorf("failed to close primary: %w", err))
	}

	// Close replicas
	for i, replica := range r.replicas {
		if err := replica.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close replica %d: %w", i, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing connections: %v", errors)
	}

	return nil
}

// GetPrimaryDB returns the primary database connection
func (r *Router) GetPrimaryDB() *sql.DB {
	return r.primary
}

// GetReplicaCount returns the number of configured replicas
func (r *Router) GetReplicaCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.replicas)
}

// GetHealthyReplicaCount returns the number of healthy replicas
func (r *Router) GetHealthyReplicaCount() int {
	return len(r.healthChecker.GetHealthyReplicas())
}
