package metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
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

func TestCollector_100Ops_CountAccuracy(t *testing.T) {
	t.Parallel()
	c := NewCollector()

	wantCounts := map[string]int64{"add": 30, "search": 25, "update": 25, "delete": 20}
	wantErrors := map[string]int64{"add": 0, "search": 0, "update": 0, "delete": 5}

	for i := int64(0); i < wantCounts["add"]; i++ {
		c.Record("add", time.Duration(i+1)*time.Millisecond, nil)
	}
	for i := int64(0); i < wantCounts["search"]; i++ {
		c.Record("search", time.Duration(i+1)*time.Millisecond, nil)
	}
	for i := int64(0); i < wantCounts["update"]; i++ {
		c.Record("update", time.Duration(i+1)*time.Millisecond, nil)
	}
	for i := int64(0); i < wantCounts["delete"]; i++ {
		var err error
		if i < wantErrors["delete"] {
			err = fmt.Errorf("delete failed")
		}
		c.Record("delete", time.Duration(i+1)*time.Millisecond, err)
	}

	snap := c.Snapshot()

	totalRecorded := int64(0)
	for op, want := range wantCounts {
		got := snap.Ops[op].Count
		if got != want {
			t.Fatalf("%s count = %d, want %d", op, got, want)
		}
		totalRecorded += got
	}
	if totalRecorded != 100 {
		t.Fatalf("total ops = %d, want 100", totalRecorded)
	}

	for op, want := range wantErrors {
		got := snap.Ops[op].Errors
		if got != want {
			t.Fatalf("%s errors = %d, want %d", op, got, want)
		}
	}
}

func TestCollector_LatencyPercentiles_KnownDistribution(t *testing.T) {
	t.Parallel()
	c := NewCollector()

	for i := 1; i <= 100; i++ {
		c.Record("op", time.Duration(i)*time.Millisecond, nil)
	}

	snap := c.Snapshot()
	s := snap.Ops["op"]

	const tolerancePct = 0.05
	checks := []struct {
		name   string
		got    time.Duration
		target float64
	}{
		{"p50", s.P50, 50},
		{"p95", s.P95, 95},
		{"p99", s.P99, 99},
	}
	for _, c := range checks {
		gotMs := float64(c.got) / float64(time.Millisecond)
		lo := c.target * (1 - tolerancePct)
		hi := c.target * (1 + tolerancePct)
		if gotMs < lo || gotMs > hi {
			t.Fatalf("%s = %.2fms, want %.2f–%.2fms (target %.0fms ±5%%)",
				c.name, gotMs, lo, hi, c.target)
		}
	}
}

func TestCollector_WriteTo_ValidNDJSON(t *testing.T) {
	t.Parallel()
	c := NewCollector()

	ops := []string{"add", "delete", "search", "update"}
	for _, op := range ops {
		c.Record(op, 10*time.Millisecond, nil)
		c.Record(op, 20*time.Millisecond, fmt.Errorf("err"))
	}

	var buf bytes.Buffer
	n, err := c.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if n == 0 {
		t.Fatal("WriteTo wrote 0 bytes")
	}

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != len(ops) {
		t.Fatalf("got %d lines, want %d", len(lines), len(ops))
	}

	requiredFields := []string{"op", "count", "errors", "p50_ms", "p95_ms", "p99_ms", "timestamp"}
	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			t.Fatalf("line %d not valid JSON: %v", i, err)
		}
		for _, field := range requiredFields {
			v, ok := obj[field]
			if !ok {
				t.Fatalf("line %d missing field %q", i, field)
			}
			if field == "count" || field == "errors" {
				num, ok := v.(float64)
				if !ok || math.IsNaN(num) {
					t.Fatalf("line %d field %q not a valid number: %v", i, field, v)
				}
			}
		}
	}
}

func TestCollector_ConcurrentRecordAndSnapshot(t *testing.T) {
	t.Parallel()
	c := NewCollector()

	const writers = 10
	const opsPerWriter = 500

	var wg sync.WaitGroup
	wg.Add(writers)
	for g := 0; g < writers; g++ {
		go func(id int) {
			defer wg.Done()
			op := fmt.Sprintf("op_%d", id%4)
			for i := 0; i < opsPerWriter; i++ {
				c.Record(op, time.Duration(i)*time.Microsecond, nil)
			}
		}(g)
	}

	snapshots := make([]MetricsSnapshot, 0, 20)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	for {
		select {
		case <-done:
			final := c.Snapshot()
			var total int64
			for _, s := range final.Ops {
				total += s.Count
			}
			if total != writers*opsPerWriter {
				t.Errorf("final total = %d, want %d", total, writers*opsPerWriter)
			}
			if len(snapshots) == 0 {
				t.Error("main goroutine took zero snapshots during concurrent writes")
			}
			return
		default:
			snapshots = append(snapshots, c.Snapshot())
			time.Sleep(50 * time.Microsecond)
		}
	}
}
