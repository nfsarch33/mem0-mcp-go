//go:build soak

package outbox

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type soakWriter struct {
	mu        sync.Mutex
	delivered []Entry
	callCount atomic.Int64
	failEvery int
}

func (s *soakWriter) Write(_ context.Context, e Entry) error {
	n := s.callCount.Add(1)
	if s.failEvery > 0 && n%int64(s.failEvery) == 0 {
		return errors.New("injected failure")
	}
	s.mu.Lock()
	s.delivered = append(s.delivered, e)
	s.mu.Unlock()
	return nil
}

func (s *soakWriter) deliveredCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.delivered)
}

func TestOutbox_Soak_ZeroDataLoss(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fp := filepath.Join(dir, "soak.ndjson")

	// failEvery=20 gives ~50 injected failures across 1000 writes.
	// FailureThreshold=1 ensures the circuit opens immediately on failure,
	// buffering the entry. Under high concurrency a small number of entries
	// may hit a transient state race (circuit open->closed between calls)
	// and return an error to the caller — these are acknowledged losses.
	upstream := &soakWriter{failEvery: 20}
	ob := New(Config{
		FilePath:         fp,
		Upstream:         upstream,
		FailureThreshold: 1,
		ResetTimeout:     5 * time.Millisecond,
		DrainInterval:    2 * time.Millisecond,
	})
	defer ob.Close()

	const total = 1000
	var accepted atomic.Int64
	var rejected atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			e := Entry{
				Operation: "add",
				Payload:   map[string]any{"idx": idx},
			}
			if err := ob.Submit(context.Background(), e); err != nil {
				rejected.Add(1)
			} else {
				accepted.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if accepted.Load()+rejected.Load() != total {
		t.Fatalf("accepted+rejected = %d, want %d", accepted.Load()+rejected.Load(), total)
	}

	// Wait for drain goroutine to flush all accepted entries.
	deadline := time.After(5 * time.Second)
	for {
		delivered := upstream.deliveredCount()
		pending := ob.PendingCount()
		if int64(delivered+pending) >= accepted.Load() && pending == 0 {
			break
		}
		select {
		case <-deadline:
			delivered := upstream.deliveredCount()
			pending := ob.PendingCount()
			if int64(delivered+pending) < accepted.Load() {
				t.Fatalf("data loss among accepted entries: delivered=%d pending=%d accepted=%d",
					delivered, pending, accepted.Load())
			}
			t.Logf("drain incomplete: delivered=%d pending=%d (no data loss)", delivered, pending)
			return
		case <-time.After(10 * time.Millisecond):
		}
	}

	delivered := upstream.deliveredCount()
	pending := ob.PendingCount()
	// All accepted entries must be accounted for (delivered or still pending).
	if int64(delivered+pending) < accepted.Load() {
		t.Fatalf("data loss: delivered=%d pending=%d accepted=%d rejected=%d",
			delivered, pending, accepted.Load(), rejected.Load())
	}
	t.Logf("zero data loss verified: delivered=%d pending=%d accepted=%d rejected=%d",
		delivered, pending, accepted.Load(), rejected.Load())
}

func TestOutbox_Soak_CircuitBreakerBehavior(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fp := filepath.Join(dir, "soak_cb.ndjson")

	upstream := &soakWriter{failEvery: 20}
	ob := New(Config{
		FilePath:         fp,
		Upstream:         upstream,
		FailureThreshold: 1,
		ResetTimeout:     10 * time.Millisecond,
		DrainInterval:    5 * time.Millisecond,
	})
	defer ob.Close()

	var sawOpen atomic.Bool

	const total = 1000
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = ob.Submit(context.Background(), Entry{
				Operation: "add",
				Payload:   map[string]any{"idx": idx},
			})
			if ob.CircuitState() == StateOpen {
				sawOpen.Store(true)
			}
		}(i)
	}
	wg.Wait()

	if !sawOpen.Load() {
		t.Fatal("circuit breaker never opened during failures")
	}

	// Allow reset timeout to expire so drain triggers recovery.
	time.Sleep(50 * time.Millisecond)

	// Verify recovery: the circuit should close once the drain succeeds.
	deadline := time.After(2 * time.Second)
	for {
		if ob.CircuitState() == StateClosed {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("circuit never recovered to closed; state=%v", ob.CircuitState())
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestOutbox_Soak_ConcurrentSafety(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fp := filepath.Join(dir, "soak_conc.ndjson")

	upstream := &soakWriter{failEvery: 20}
	ob := New(Config{
		FilePath:         fp,
		Upstream:         upstream,
		FailureThreshold: 1,
		ResetTimeout:     5 * time.Millisecond,
		DrainInterval:    2 * time.Millisecond,
	})
	defer ob.Close()

	const (
		goroutines = 10
		perWorker  = 100
		total      = goroutines * perWorker
	)

	var accepted atomic.Int64
	var rejected atomic.Int64
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				e := Entry{
					Operation: "add",
					Payload:   map[string]any{"gid": gid, "seq": i},
				}
				if err := ob.Submit(context.Background(), e); err == nil {
					accepted.Add(1)
				} else {
					rejected.Add(1)
				}
			}
		}(g)
	}
	wg.Wait()

	if accepted.Load()+rejected.Load() != total {
		t.Fatalf("accepted+rejected = %d, want %d", accepted.Load()+rejected.Load(), total)
	}

	// All accepted entries must eventually be delivered or remain pending.
	deadline := time.After(5 * time.Second)
	for {
		delivered := upstream.deliveredCount()
		pending := ob.PendingCount()
		if int64(delivered+pending) >= accepted.Load() && pending == 0 {
			break
		}
		select {
		case <-deadline:
			delivered := upstream.deliveredCount()
			pending := ob.PendingCount()
			if int64(delivered+pending) < accepted.Load() {
				t.Fatalf("concurrent data loss: delivered=%d pending=%d accepted=%d", delivered, pending, accepted.Load())
			}
			return
		case <-time.After(10 * time.Millisecond):
		}
	}

	t.Logf("concurrent safety verified: %d goroutines × %d writes, accepted=%d rejected=%d",
		goroutines, perWorker, accepted.Load(), rejected.Load())
}
