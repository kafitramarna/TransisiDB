package replica

import (
	"database/sql"
	"sync"
	"time"
)

// HealthChecker monitors database health
type HealthChecker struct {
	primary       *sql.DB
	replicas      []*sql.DB
	healthStatus  []bool
	mu            sync.RWMutex
	checkInterval time.Duration
	stopCh        chan struct{}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(primary *sql.DB, replicas []*sql.DB) *HealthChecker {
	hc := &HealthChecker{
		primary:       primary,
		replicas:      replicas,
		healthStatus:  make([]bool, len(replicas)),
		checkInterval: 10 * time.Second, // Default check interval
		stopCh:        make(chan struct{}),
	}

	// Initialize all as healthy
	for i := range hc.healthStatus {
		hc.healthStatus[i] = true
	}

	// Start health check goroutine
	go hc.runHealthChecks()

	return hc
}

// runHealthChecks periodically checks replica health
func (hc *HealthChecker) runHealthChecks() {
	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.checkAllReplicas()
		case <-hc.stopCh:
			return
		}
	}
}

// checkAllReplicas checks health of all replicas
func (hc *HealthChecker) checkAllReplicas() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	for i, replica := range hc.replicas {
		hc.healthStatus[i] = hc.checkReplica(replica)
	}
}

// checkReplica checks if a replica is healthy
func (hc *HealthChecker) checkReplica(db *sql.DB) bool {
	// Simple ping check
	err := db.Ping()
	return err == nil
}

// GetHealthyReplicas returns list of healthy replica connections
func (hc *HealthChecker) GetHealthyReplicas() []*sql.DB {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	healthy := make([]*sql.DB, 0)
	for i, isHealthy := range hc.healthStatus {
		if isHealthy {
			healthy = append(healthy, hc.replicas[i])
		}
	}
	return healthy
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}

// SetCheckInterval sets the health check interval
func (hc *HealthChecker) SetCheckInterval(interval time.Duration) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checkInterval = interval
}
