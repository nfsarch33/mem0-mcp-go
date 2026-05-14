package metrics

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"
)

type OpStats struct {
	Count  int64         `json:"count"`
	Errors int64         `json:"errors"`
	P50    time.Duration `json:"p50_ms"`
	P95    time.Duration `json:"p95_ms"`
	P99    time.Duration `json:"p99_ms"`
}

type MetricsSnapshot struct {
	Ops       map[string]OpStats `json:"ops"`
	Timestamp time.Time          `json:"timestamp"`
}

type opBucket struct {
	count    int64
	errors   int64
	latencies []time.Duration
}

type Collector struct {
	mu      sync.Mutex
	buckets map[string]*opBucket
}

func NewCollector() *Collector {
	return &Collector{
		buckets: make(map[string]*opBucket),
	}
}

func (c *Collector) Record(op string, latency time.Duration, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	b, ok := c.buckets[op]
	if !ok {
		b = &opBucket{}
		c.buckets[op] = b
	}
	b.count++
	b.latencies = append(b.latencies, latency)
	if err != nil {
		b.errors++
	}
}

func (c *Collector) Snapshot() MetricsSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	snap := MetricsSnapshot{
		Ops:       make(map[string]OpStats, len(c.buckets)),
		Timestamp: time.Now(),
	}
	for op, b := range c.buckets {
		sorted := make([]time.Duration, len(b.latencies))
		copy(sorted, b.latencies)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

		snap.Ops[op] = OpStats{
			Count:  b.count,
			Errors: b.errors,
			P50:    percentile(sorted, 0.50),
			P95:    percentile(sorted, 0.95),
			P99:    percentile(sorted, 0.99),
		}
	}
	return snap
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p * float64(len(sorted)))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

type ndjsonLine struct {
	Op        string  `json:"op"`
	Count     int64   `json:"count"`
	Errors    int64   `json:"errors"`
	P50Ms     float64 `json:"p50_ms"`
	P95Ms     float64 `json:"p95_ms"`
	P99Ms     float64 `json:"p99_ms"`
	Timestamp string  `json:"timestamp"`
}

func (c *Collector) WriteTo(w io.Writer) (int64, error) {
	snap := c.Snapshot()
	var total int64
	ts := snap.Timestamp.UTC().Format(time.RFC3339)

	ops := make([]string, 0, len(snap.Ops))
	for op := range snap.Ops {
		ops = append(ops, op)
	}
	sort.Strings(ops)

	for _, op := range ops {
		s := snap.Ops[op]
		line := ndjsonLine{
			Op:        op,
			Count:     s.Count,
			Errors:    s.Errors,
			P50Ms:     float64(s.P50) / float64(time.Millisecond),
			P95Ms:     float64(s.P95) / float64(time.Millisecond),
			P99Ms:     float64(s.P99) / float64(time.Millisecond),
			Timestamp: ts,
		}
		raw, err := json.Marshal(line)
		if err != nil {
			return total, fmt.Errorf("marshal metrics line: %w", err)
		}
		raw = append(raw, '\n')
		n, err := w.Write(raw)
		total += int64(n)
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
