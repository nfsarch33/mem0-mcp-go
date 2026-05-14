package metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRecord_IncrementsCounters(t *testing.T) {
	t.Parallel()
	c := NewCollector()

	c.Record("add", 10*time.Millisecond, nil)
	c.Record("add", 20*time.Millisecond, nil)
	c.Record("search", 5*time.Millisecond, nil)
	c.Record("delete", 15*time.Millisecond, fmt.Errorf("boom"))

	snap := c.Snapshot()

	if snap.Ops["add"].Count != 2 {
		t.Fatalf("add count = %d, want 2", snap.Ops["add"].Count)
	}
	if snap.Ops["search"].Count != 1 {
		t.Fatalf("search count = %d, want 1", snap.Ops["search"].Count)
	}
	if snap.Ops["delete"].Count != 1 {
		t.Fatalf("delete count = %d, want 1", snap.Ops["delete"].Count)
	}
	if snap.Ops["delete"].Errors != 1 {
		t.Fatalf("delete errors = %d, want 1", snap.Ops["delete"].Errors)
	}
	if snap.Ops["add"].Errors != 0 {
		t.Fatalf("add errors = %d, want 0", snap.Ops["add"].Errors)
	}
}

func TestSnapshot_PercentilesCorrect(t *testing.T) {
	t.Parallel()
	c := NewCollector()

	// Record 100 data points: 1ms, 2ms, ..., 100ms
	for i := 1; i <= 100; i++ {
		c.Record("search", time.Duration(i)*time.Millisecond, nil)
	}

	snap := c.Snapshot()
	s := snap.Ops["search"]

	// p50 should be ~50ms (index 49 in sorted 0-based)
	if s.P50 < 49*time.Millisecond || s.P50 > 51*time.Millisecond {
		t.Fatalf("p50 = %v, want ~50ms", s.P50)
	}
	// p95 should be ~95ms
	if s.P95 < 94*time.Millisecond || s.P95 > 96*time.Millisecond {
		t.Fatalf("p95 = %v, want ~95ms", s.P95)
	}
	// p99 should be ~99ms
	if s.P99 < 98*time.Millisecond || s.P99 > 100*time.Millisecond {
		t.Fatalf("p99 = %v, want ~99ms", s.P99)
	}
}

func TestWriteTo_ProducesValidNDJSON(t *testing.T) {
	t.Parallel()
	c := NewCollector()

	c.Record("add", 10*time.Millisecond, nil)
	c.Record("search", 20*time.Millisecond, nil)
	c.Record("search", 30*time.Millisecond, fmt.Errorf("timeout"))

	var buf bytes.Buffer
	n, err := c.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo error: %v", err)
	}
	if n == 0 {
		t.Fatal("WriteTo wrote 0 bytes")
	}

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) == 0 {
		t.Fatal("expected at least one NDJSON line")
	}

	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			t.Fatalf("line %d is not valid JSON: %v\nline: %s", i, err, line)
		}
		if _, ok := obj["op"]; !ok {
			t.Fatalf("line %d missing 'op' field", i)
		}
		if _, ok := obj["count"]; !ok {
			t.Fatalf("line %d missing 'count' field", i)
		}
		if _, ok := obj["errors"]; !ok {
			t.Fatalf("line %d missing 'errors' field", i)
		}
		if _, ok := obj["timestamp"]; !ok {
			t.Fatalf("line %d missing 'timestamp' field", i)
		}
	}
}

func TestRecord_ConcurrentSafe(t *testing.T) {
	t.Parallel()
	c := NewCollector()

	const goroutines = 50
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				c.Record("add", time.Duration(i)*time.Microsecond, nil)
			}
		}()
	}
	wg.Wait()

	snap := c.Snapshot()
	want := int64(goroutines * opsPerGoroutine)
	if snap.Ops["add"].Count != want {
		t.Fatalf("count = %d, want %d", snap.Ops["add"].Count, want)
	}
}
