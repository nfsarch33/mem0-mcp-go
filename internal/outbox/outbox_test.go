package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockWriter struct {
	mu       sync.Mutex
	calls    []Entry
	failNext bool
}

func (m *mockWriter) Write(ctx context.Context, e Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failNext {
		return errors.New("upstream down")
	}
	m.calls = append(m.calls, e)
	return nil
}

func (m *mockWriter) setFail(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failNext = fail
}

func (m *mockWriter) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func TestOutbox_WritesGoToUpstreamWhenHealthy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	upstream := &mockWriter{}
	ob := New(Config{
		FilePath:         filepath.Join(dir, "outbox.ndjson"),
		Upstream:         upstream,
		FailureThreshold: 3,
		ResetTimeout:     30 * time.Second,
		DrainInterval:    100 * time.Millisecond,
	})
	defer ob.Close()

	err := ob.Submit(context.Background(), Entry{
		Operation: "add",
		Payload:   map[string]any{"text": "hello"},
	})
	if err != nil {
		t.Fatalf("Submit() err = %v", err)
	}
	if upstream.callCount() != 1 {
		t.Fatalf("upstream calls = %d, want 1", upstream.callCount())
	}
	assertFileLineCount(t, filepath.Join(dir, "outbox.ndjson"), 0)
}

func TestOutbox_WritesGoToFileWhenCircuitOpen(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	upstream := &mockWriter{failNext: true}
	ob := New(Config{
		FilePath:         filepath.Join(dir, "outbox.ndjson"),
		Upstream:         upstream,
		FailureThreshold: 1,
		ResetTimeout:     time.Hour,
		DrainInterval:    100 * time.Millisecond,
	})
	defer ob.Close()

	// First submit fails and trips the circuit; the failed entry is
	// buffered to the outbox file.
	_ = ob.Submit(context.Background(), Entry{Operation: "add", Payload: map[string]any{"text": "first-fail"}})

	// Subsequent submit goes straight to file because circuit is open.
	err := ob.Submit(context.Background(), Entry{
		Operation: "add",
		Payload:   map[string]any{"text": "queued"},
	})
	if err != nil {
		t.Fatalf("Submit() err = %v, want nil (buffered to file)", err)
	}
	// Both entries end up in the file: the one that tripped the circuit
	// and the one submitted while open.
	assertFileLineCount(t, filepath.Join(dir, "outbox.ndjson"), 2)
}

func TestOutbox_DrainFlushesToUpstreamWhenCircuitCloses(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	upstream := &mockWriter{failNext: true}
	ob := New(Config{
		FilePath:         filepath.Join(dir, "outbox.ndjson"),
		Upstream:         upstream,
		FailureThreshold: 1,
		ResetTimeout:     20 * time.Millisecond,
		DrainInterval:    10 * time.Millisecond,
	})
	defer ob.Close()

	// Trip the circuit; the failed entry gets buffered.
	_ = ob.Submit(context.Background(), Entry{Operation: "add", Payload: map[string]any{"text": "fail1"}})

	// This one goes straight to file (circuit open).
	err := ob.Submit(context.Background(), Entry{
		Operation: "add",
		Payload:   map[string]any{"text": "buffered"},
	})
	if err != nil {
		t.Fatalf("Submit() = %v", err)
	}
	assertFileLineCount(t, filepath.Join(dir, "outbox.ndjson"), 2)

	upstream.setFail(false)
	time.Sleep(150 * time.Millisecond)

	assertFileLineCount(t, filepath.Join(dir, "outbox.ndjson"), 0)
	if upstream.callCount() != 2 {
		t.Fatalf("upstream calls = %d, want 2 (drained entries)", upstream.callCount())
	}
}

func TestOutbox_ConcurrentWritesAreSafe(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var count atomic.Int64
	upstream := &mockWriter{}
	ob := New(Config{
		FilePath:         filepath.Join(dir, "outbox.ndjson"),
		Upstream:         upstream,
		FailureThreshold: 3,
		ResetTimeout:     30 * time.Second,
		DrainInterval:    100 * time.Millisecond,
	})
	defer ob.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := ob.Submit(context.Background(), Entry{
				Operation: "add",
				Payload:   map[string]any{"text": "concurrent"},
			})
			if err == nil {
				count.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := count.Load(); got != 50 {
		t.Fatalf("successful submits = %d, want 50", got)
	}
}

func TestOutbox_PendingCount(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	upstream := &mockWriter{failNext: true}
	ob := New(Config{
		FilePath:         filepath.Join(dir, "outbox.ndjson"),
		Upstream:         upstream,
		FailureThreshold: 1,
		ResetTimeout:     time.Hour,
		DrainInterval:    time.Hour,
	})
	defer ob.Close()

	// Trip the circuit (this entry also gets buffered).
	_ = ob.Submit(context.Background(), Entry{Operation: "add", Payload: map[string]any{"text": "trip"}})

	// 3 more entries go to file while circuit is open.
	for i := 0; i < 3; i++ {
		_ = ob.Submit(context.Background(), Entry{
			Operation: "add",
			Payload:   map[string]any{"text": "queued"},
		})
	}
	// 1 (trip) + 3 (queued) = 4 pending
	if got := ob.PendingCount(); got != 4 {
		t.Fatalf("PendingCount() = %d, want 4", got)
	}
}

func TestOutbox_CircuitState(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	upstream := &mockWriter{}
	ob := New(Config{
		FilePath:         filepath.Join(dir, "outbox.ndjson"),
		Upstream:         upstream,
		FailureThreshold: 3,
		ResetTimeout:     30 * time.Second,
		DrainInterval:    time.Hour,
	})
	defer ob.Close()

	if ob.CircuitState() != StateClosed {
		t.Fatalf("CircuitState() = %v, want Closed", ob.CircuitState())
	}
}

func assertFileLineCount(t *testing.T, path string, want int) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		if want == 0 {
			return
		}
		t.Fatalf("read file: %v", err)
	}
	content := strings.TrimSpace(string(raw))
	if content == "" {
		if want != 0 {
			t.Fatalf("file is empty, want %d lines", want)
		}
		return
	}
	lines := strings.Split(content, "\n")
	got := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(trimmed), &entry); err != nil {
			t.Fatalf("invalid NDJSON line: %q: %v", trimmed, err)
		}
		got++
	}
	if got != want {
		t.Fatalf("file lines = %d, want %d", got, want)
	}
}
