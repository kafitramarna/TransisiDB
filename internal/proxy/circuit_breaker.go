package proxy

import (
	"errors"
	"sync"
	"time"

	"github.com/kafitramarna/TransisiDB/internal/logger"
)

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

var (
	// ErrCircuitBreakerOpen is returned when circuit breaker is open
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open")
)

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	// MaxFailures before opening the circuit
	MaxFailures int
	// Timeout duration to wait before attempting to close an open circuit
	Timeout time.Duration
	// MaxRequests allowed in half-open state
	MaxRequests int
}

// DefaultCircuitBreakerConfig returns default configuration
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		MaxFailures: 5,                // Open circuit after 5 consecutive failures
		Timeout:     30 * time.Second, // Try to recover after 30 seconds
		MaxRequests: 3,                // Allow 3 requests in half-open state
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config CircuitBreakerConfig
	mu     sync.RWMutex

	state            CircuitBreakerState
	failures         int
	lastFailureTime  time.Time
	lastStateChange  time.Time
	halfOpenRequests int

	// Metrics
	totalRequests   uint64
	totalSuccesses  uint64
	totalFailures   uint64
	totalRejections uint64
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config:          config,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

// Call executes the given function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() error) error {
	// Check if we can proceed
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	// Execute the function
	err := fn()

	// Record the result
	cb.afterRequest(err)

	return err
}

// beforeRequest checks if the circuit breaker allows the request
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests++

	switch cb.state {
	case StateClosed:
		// Allow request
		return nil

	case StateOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailureTime) > cb.config.Timeout {
			// Transition to half-open
			cb.setState(StateHalfOpen)
			cb.halfOpenRequests = 0
			logger.Info("Circuit breaker transitioning to HALF_OPEN", "previous_failures", cb.failures)
			return nil
		}

		// Reject request
		cb.totalRejections++
		return ErrCircuitBreakerOpen

	case StateHalfOpen:
		// Check if we've reached max requests in half-open state
		if cb.halfOpenRequests >= cb.config.MaxRequests {
			cb.totalRejections++
			return ErrCircuitBreakerOpen
		}

		cb.halfOpenRequests++
		return nil

	default:
		return nil
	}
}

// afterRequest records the result of the request
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

// onSuccess handles a successful request
func (cb *CircuitBreaker) onSuccess() {
	cb.totalSuccesses++

	switch cb.state {
	case StateClosed:
		// Reset failure count on success
		if cb.failures > 0 {
			cb.failures = 0
		}

	case StateHalfOpen:
		// In half-open state, we've already incremented halfOpenRequests in beforeRequest
		// If we've completed enough successful requests, close the circuit
		if cb.halfOpenRequests >= cb.config.MaxRequests {
			cb.setState(StateClosed)
			cb.failures = 0
			cb.halfOpenRequests = 0
			logger.Info("Circuit breaker closed after successful recovery")
		}
	}
}

// onFailure handles a failed request
func (cb *CircuitBreaker) onFailure() {
	cb.totalFailures++
	cb.failures++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		// Check if we've exceeded max failures
		if cb.failures >= cb.config.MaxFailures {
			cb.setState(StateOpen)
			logger.Warn("Circuit breaker opened due to failures",
				"failures", cb.failures,
				"threshold", cb.config.MaxFailures)
		}

	case StateHalfOpen:
		// Any failure in half-open immediately opens the circuit again
		cb.setState(StateOpen)
		logger.Warn("Circuit breaker re-opened due to failure in HALF_OPEN state")
	}
}

// setState transitions to a new state
func (cb *CircuitBreaker) setState(state CircuitBreakerState) {
	if cb.state != state {
		oldState := cb.state
		cb.state = state
		cb.lastStateChange = time.Now()
		logger.Info("Circuit breaker state changed",
			"from", oldState.String(),
			"to", state.String())
	}
}

// GetState returns the current state (thread-safe)
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"state":             cb.state.String(),
		"failures":          cb.failures,
		"total_requests":    cb.totalRequests,
		"total_successes":   cb.totalSuccesses,
		"total_failures":    cb.totalFailures,
		"total_rejections":  cb.totalRejections,
		"last_state_change": cb.lastStateChange.Format(time.RFC3339),
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.halfOpenRequests = 0
	cb.lastStateChange = time.Now()

	logger.Info("Circuit breaker manually reset")
}

// IsOpen returns true if the circuit is open
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == StateOpen
}
