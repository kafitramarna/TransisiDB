package proxy

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/kafitramarna/TransisiDB/internal/config"
	"github.com/kafitramarna/TransisiDB/internal/logger"
)

// BackendConn wraps a backend MySQL connection with metadata
type BackendConn struct {
	conn          net.Conn
	connectionID  uint32
	createdAt     time.Time
	lastUsedAt    time.Time
	inTransaction bool
	database      string
	mu            sync.Mutex
}

// NewBackendConn creates a new backend connection wrapper
func NewBackendConn(conn net.Conn, connID uint32) *BackendConn {
	now := time.Now()
	return &BackendConn{
		conn:         conn,
		connectionID: connID,
		createdAt:    now,
		lastUsedAt:   now,
	}
}

// Conn returns the underlying net.Conn
func (bc *BackendConn) Conn() net.Conn {
	return bc.conn
}

// Close closes the backend connection
func (bc *BackendConn) Close() error {
	return bc.conn.Close()
}

// IsInTransaction returns whether the connection is in a transaction
func (bc *BackendConn) IsInTransaction() bool {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.inTransaction
}

// SetInTransaction sets the transaction state
func (bc *BackendConn) SetInTransaction(inTx bool) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.inTransaction = inTx
}

// GetDatabase returns the current database context
func (bc *BackendConn) GetDatabase() string {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.database
}

// SetDatabase sets the current database context
func (bc *BackendConn) SetDatabase(db string) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.database = db
}

// UpdateLastUsed updates the last used timestamp
func (bc *BackendConn) UpdateLastUsed() {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.lastUsedAt = time.Now()
}

// Age returns the age of the connection
func (bc *BackendConn) Age() time.Duration {
	return time.Since(bc.createdAt)
}

// IdleTime returns how long the connection has been idle
func (bc *BackendConn) IdleTime() time.Duration {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return time.Since(bc.lastUsedAt)
}

// IsHealthy performs a basic health check
func (bc *BackendConn) IsHealthy() bool {
	// Check if connection is still alive
	if bc.conn == nil {
		return false
	}

	// Set a short deadline for health check
	bc.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	defer bc.conn.SetReadDeadline(time.Time{})

	// Try to peek at the connection
	// A closed connection will return an error immediately
	one := make([]byte, 1)
	_, err := bc.conn.Read(one)
	if err == nil {
		// This shouldn't happen in normal operation (no data should be waiting)
		logger.Warn("Unexpected data on idle backend connection", "conn_id", bc.connectionID)
		return false
	}

	// Check if it's a timeout (expected for healthy idle connection)
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	// Any other error means connection is not healthy
	return false
}

// Reset resets the connection state for reuse
func (bc *BackendConn) Reset() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Reset transaction state
	bc.inTransaction = false
	bc.database = ""
	bc.lastUsedAt = time.Now()

	return nil
}

// BackendPool manages a pool of backend MySQL connections
type BackendPool struct {
	config         *config.Config
	connections    chan *BackendConn
	connCounter    uint32
	mu             sync.Mutex
	closed         bool
	wg             sync.WaitGroup
	circuitBreaker *CircuitBreaker

	// Metrics
	totalCreated  uint64
	totalAcquired uint64
	totalReleased uint64
	totalEvicted  uint64
	currentActive int32
	currentIdle   int32
}

// NewBackendPool creates a new backend connection pool
func NewBackendPool(cfg *config.Config, poolSize int) (*BackendPool, error) {
	pool := &BackendPool{
		config:         cfg,
		connections:    make(chan *BackendConn, poolSize),
		circuitBreaker: NewCircuitBreaker(DefaultCircuitBreakerConfig()),
	}

	logger.Info("Backend connection pool created",
		"pool_size", poolSize,
		"circuit_breaker_max_failures", pool.circuitBreaker.config.MaxFailures,
		"circuit_breaker_timeout", pool.circuitBreaker.config.Timeout)

	// Start background worker to clean up idle connections
	pool.wg.Add(1)
	go pool.cleanupWorker()

	return pool, nil
}

// Acquire gets a connection from the pool or creates a new one
func (bp *BackendPool) Acquire() (*BackendConn, error) {
	bp.mu.Lock()
	if bp.closed {
		bp.mu.Unlock()
		return nil, fmt.Errorf("pool is closed")
	}
	bp.mu.Unlock()

	// Try to get an existing connection from the pool
	select {
	case conn := <-bp.connections:
		// Check if connection is still healthy
		if conn.IsHealthy() {
			conn.UpdateLastUsed()
			bp.totalAcquired++
			logger.Debug("Reused backend connection from pool", "conn_id", conn.connectionID)
			return conn, nil
		}

		// Connection is not healthy, close it and create a new one
		logger.Warn("Evicting unhealthy connection from pool", "conn_id", conn.connectionID)
		conn.Close()
		bp.totalEvicted++
		// Fall through to create new connection
	default:
		// No idle connections available, create new one
	}

	// Create a new connection
	return bp.createConnection()
}

// Release returns a connection to the pool
func (bp *BackendPool) Release(conn *BackendConn) {
	if conn == nil {
		return
	}

	bp.mu.Lock()
	if bp.closed {
		bp.mu.Unlock()
		conn.Close()
		return
	}
	bp.mu.Unlock()

	// Don't reuse connections that are in a transaction
	if conn.IsInTransaction() {
		logger.Warn("Not returning connection to pool (in transaction)", "conn_id", conn.connectionID)
		conn.Close()
		return
	}

	// Reset connection state
	if err := conn.Reset(); err != nil {
		logger.Error("Failed to reset connection", "conn_id", conn.connectionID, "error", err)
		conn.Close()
		return
	}

	// Try to return to pool (non-blocking)
	select {
	case bp.connections <- conn:
		bp.totalReleased++
		logger.Debug("Returned backend connection to pool", "conn_id", conn.connectionID)
	default:
		// Pool is full, close the connection
		logger.Debug("Pool full, closing backend connection", "conn_id", conn.connectionID)
		conn.Close()
	}
}

// createConnection creates a new backend connection
func (bp *BackendPool) createConnection() (*BackendConn, error) {
	var conn net.Conn
	var connErr error

	// Use circuit breaker to protect against cascading failures
	err := bp.circuitBreaker.Call(func() error {
		addr := net.JoinHostPort(bp.config.Database.Host, fmt.Sprintf("%d", bp.config.Database.Port))

		// Dial backend with timeout
		dialer := &net.Dialer{
			Timeout: bp.config.Database.ConnectionTimeout,
		}

		var dialErr error
		conn, dialErr = dialer.Dial("tcp", addr)
		if dialErr != nil {
			connErr = fmt.Errorf("failed to connect to backend %s: %w", addr, dialErr)
			return dialErr
		}

		return nil
	})

	// Check circuit breaker result
	if err != nil {
		if err == ErrCircuitBreakerOpen {
			logger.Warn("Circuit breaker is OPEN, rejecting connection attempt")
			return nil, fmt.Errorf("backend unavailable (circuit breaker open): %w", err)
		}
		// Return the actual connection error
		return nil, connErr
	}

	// Generate connection ID
	bp.mu.Lock()
	bp.connCounter++
	connID := bp.connCounter
	bp.totalCreated++
	bp.mu.Unlock()

	backendConn := NewBackendConn(conn, connID)

	logger.Info("Created new backend connection", "conn_id", connID)

	return backendConn, nil
}

// Close closes the pool and all connections
func (bp *BackendPool) Close() error {
	bp.mu.Lock()
	if bp.closed {
		bp.mu.Unlock()
		return nil
	}
	bp.closed = true
	bp.mu.Unlock()

	// Close all idle connections
	close(bp.connections)
	for conn := range bp.connections {
		conn.Close()
	}

	// Wait for cleanup worker to finish
	bp.wg.Wait()

	logger.Info("Backend connection pool closed")
	return nil
}

// cleanupWorker periodically cleans up stale idle connections
func (bp *BackendPool) cleanupWorker() {
	defer bp.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	maxIdleTime := 5 * time.Minute
	maxAge := 30 * time.Minute

	for {
		select {
		case <-ticker.C:
			bp.cleanupStaleConnections(maxIdleTime, maxAge)
		case <-time.After(1 * time.Minute):
			bp.mu.Lock()
			if bp.closed {
				bp.mu.Unlock()
				return
			}
			bp.mu.Unlock()
		}
	}
}

// cleanupStaleConnections removes connections that are too old or idle too long
func (bp *BackendPool) cleanupStaleConnections(maxIdleTime, maxAge time.Duration) {
	var healthyConns []*BackendConn

	// Drain all connections from pool
	for {
		select {
		case conn := <-bp.connections:
			// Skip nil connections
			if conn == nil {
				continue
			}

			// Check if connection should be evicted
			if conn.IdleTime() > maxIdleTime || conn.Age() > maxAge || !conn.IsHealthy() {
				logger.Debug("Evicting stale connection",
					"conn_id", conn.connectionID,
					"idle_time", conn.IdleTime(),
					"age", conn.Age())
				conn.Close()
				bp.totalEvicted++
			} else {
				healthyConns = append(healthyConns, conn)
			}
		default:
			// No more connections in pool
			goto done
		}
	}

done:
	// Return healthy connections to pool
	for _, conn := range healthyConns {
		select {
		case bp.connections <- conn:
		default:
			// Pool is somehow full, close excess connections
			conn.Close()
		}
	}

	if len(healthyConns) > 0 {
		logger.Debug("Cleanup completed", "healthy_conns", len(healthyConns), "evicted", bp.totalEvicted)
	}
}

// Stats returns pool statistics
func (bp *BackendPool) Stats() map[string]interface{} {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	stats := map[string]interface{}{
		"total_created":   bp.totalCreated,
		"total_acquired":  bp.totalAcquired,
		"total_released":  bp.totalReleased,
		"total_evicted":   bp.totalEvicted,
		"current_idle":    len(bp.connections),
		"pool_capacity":   cap(bp.connections),
		"circuit_breaker": bp.circuitBreaker.GetStats(),
	}

	return stats
}
