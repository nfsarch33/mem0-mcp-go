package outbox

import (
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is open and no
// requests are allowed through.
var ErrCircuitOpen = errors.New("circuit breaker is open")

type CBState int

const (
	StateClosed CBState = iota
	StateOpen
	StateHalfOpen
)

func (s CBState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

type CBConfig struct {
	FailureThreshold int
	ResetTimeout     time.Duration
}

func (c *CBConfig) applyDefaults() {
	if c.FailureThreshold <= 0 {
		c.FailureThreshold = 3
	}
	if c.ResetTimeout <= 0 {
		c.ResetTimeout = 30 * time.Second
	}
}

type CircuitBreaker struct {
	mu           sync.Mutex
	config       CBConfig
	state        CBState
	failureCount int
	lastFailure  time.Time
}

func NewCircuitBreaker(config CBConfig) *CircuitBreaker {
	config.applyDefaults()
	return &CircuitBreaker{config: config, state: StateClosed}
}

func (cb *CircuitBreaker) State() CBState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == StateOpen && time.Since(cb.lastFailure) >= cb.config.ResetTimeout {
		cb.state = StateHalfOpen
	}
	return cb.state
}

func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastFailure) >= cb.config.ResetTimeout {
			cb.state = StateHalfOpen
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount = 0
	cb.state = StateClosed
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount++
	cb.lastFailure = time.Now()
	if cb.failureCount >= cb.config.FailureThreshold {
		cb.state = StateOpen
	}
}

// Do executes fn if the circuit allows it, recording success or failure.
func (cb *CircuitBreaker) Do(fn func() error) error {
	if !cb.AllowRequest() {
		return ErrCircuitOpen
	}
	err := fn()
	if err != nil {
		cb.RecordFailure()
		return err
	}
	cb.RecordSuccess()
	return nil
}
