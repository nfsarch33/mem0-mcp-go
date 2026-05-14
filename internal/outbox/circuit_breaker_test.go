package outbox

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_StartsInClosedState(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(CBConfig{FailureThreshold: 3, ResetTimeout: 30 * time.Second})
	if cb.State() != StateClosed {
		t.Fatalf("initial state = %v, want Closed", cb.State())
	}
}

func TestCircuitBreaker_StaysClosedBelowThreshold(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(CBConfig{FailureThreshold: 3, ResetTimeout: 30 * time.Second})
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != StateClosed {
		t.Fatalf("state = %v after 2 failures (threshold 3), want Closed", cb.State())
	}
}

func TestCircuitBreaker_OpensAtThreshold(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(CBConfig{FailureThreshold: 3, ResetTimeout: 30 * time.Second})
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatalf("state = %v after 3 failures, want Open", cb.State())
	}
}

func TestCircuitBreaker_AllowRequest_DeniedWhenOpen(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(CBConfig{FailureThreshold: 1, ResetTimeout: time.Hour})
	cb.RecordFailure()
	if cb.AllowRequest() {
		t.Fatal("AllowRequest() = true when circuit is open, want false")
	}
}

func TestCircuitBreaker_TransitionsToHalfOpenAfterTimeout(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(CBConfig{FailureThreshold: 1, ResetTimeout: 10 * time.Millisecond})
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatalf("state = %v, want Open", cb.State())
	}
	time.Sleep(20 * time.Millisecond)
	if cb.State() != StateHalfOpen {
		t.Fatalf("state = %v after reset timeout, want HalfOpen", cb.State())
	}
}

func TestCircuitBreaker_HalfOpen_AllowsSingleProbe(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(CBConfig{FailureThreshold: 1, ResetTimeout: 10 * time.Millisecond})
	cb.RecordFailure()
	time.Sleep(20 * time.Millisecond)
	if !cb.AllowRequest() {
		t.Fatal("AllowRequest() = false in HalfOpen, want true for probe")
	}
}

func TestCircuitBreaker_HalfOpen_ClosesOnSuccess(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(CBConfig{FailureThreshold: 1, ResetTimeout: 10 * time.Millisecond})
	cb.RecordFailure()
	time.Sleep(20 * time.Millisecond)
	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Fatalf("state = %v after success in HalfOpen, want Closed", cb.State())
	}
}

func TestCircuitBreaker_HalfOpen_ReopensOnFailure(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(CBConfig{FailureThreshold: 1, ResetTimeout: 10 * time.Millisecond})
	cb.RecordFailure()
	time.Sleep(20 * time.Millisecond)
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatalf("state = %v after failure in HalfOpen, want Open", cb.State())
	}
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(CBConfig{FailureThreshold: 3, ResetTimeout: 30 * time.Second})
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != StateClosed {
		t.Fatalf("state = %v, want Closed (success should have reset counter)", cb.State())
	}
}

func TestCircuitBreaker_Do_SuccessPath(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(CBConfig{FailureThreshold: 3, ResetTimeout: 30 * time.Second})
	err := cb.Do(func() error { return nil })
	if err != nil {
		t.Fatalf("Do() err = %v, want nil", err)
	}
	if cb.State() != StateClosed {
		t.Fatalf("state = %v, want Closed", cb.State())
	}
}

func TestCircuitBreaker_Do_FailurePath(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(CBConfig{FailureThreshold: 2, ResetTimeout: 30 * time.Second})
	sentinel := errors.New("upstream down")
	for i := 0; i < 2; i++ {
		err := cb.Do(func() error { return sentinel })
		if !errors.Is(err, sentinel) {
			t.Fatalf("Do() err = %v, want sentinel", err)
		}
	}
	if cb.State() != StateOpen {
		t.Fatalf("state = %v, want Open", cb.State())
	}
	err := cb.Do(func() error { return nil })
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("Do() when open err = %v, want ErrCircuitOpen", err)
	}
}

func TestCircuitBreaker_DefaultConfig(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(CBConfig{})
	if cb.config.FailureThreshold != 3 {
		t.Fatalf("default FailureThreshold = %d, want 3", cb.config.FailureThreshold)
	}
	if cb.config.ResetTimeout != 30*time.Second {
		t.Fatalf("default ResetTimeout = %v, want 30s", cb.config.ResetTimeout)
	}
}
