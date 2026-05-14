package outbox

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"
)

// UpstreamWriter is the interface the outbox delegates successful writes to.
type UpstreamWriter interface {
	Write(ctx context.Context, e Entry) error
}

// Entry represents a single buffered mem0 operation.
type Entry struct {
	Operation string         `json:"operation"`
	Payload   map[string]any `json:"payload"`
	Timestamp time.Time      `json:"timestamp"`
}

// Config controls the outbox and its embedded circuit breaker.
type Config struct {
	FilePath         string
	Upstream         UpstreamWriter
	FailureThreshold int
	ResetTimeout     time.Duration
	DrainInterval    time.Duration
}

// Outbox provides an append-only NDJSON buffer that captures writes when the
// upstream mem0 API is unreachable (circuit open). A background goroutine
// drains the file when the circuit transitions back to closed/half-open.
type Outbox struct {
	mu       sync.Mutex
	filePath string
	upstream UpstreamWriter
	cb       *CircuitBreaker
	cancel   context.CancelFunc
	done     chan struct{}
}

// New creates an Outbox and starts the background drain goroutine.
func New(cfg Config) *Outbox {
	if cfg.DrainInterval <= 0 {
		cfg.DrainInterval = 5 * time.Second
	}
	cb := NewCircuitBreaker(CBConfig{
		FailureThreshold: cfg.FailureThreshold,
		ResetTimeout:     cfg.ResetTimeout,
	})
	ctx, cancel := context.WithCancel(context.Background())
	ob := &Outbox{
		filePath: cfg.FilePath,
		upstream: cfg.Upstream,
		cb:       cb,
		cancel:   cancel,
		done:     make(chan struct{}),
	}
	go ob.drainLoop(ctx, cfg.DrainInterval)
	return ob
}

// Submit tries to write to upstream. If the circuit is open, the entry is
// appended to the local NDJSON file instead.
func (o *Outbox) Submit(ctx context.Context, e Entry) error {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	if o.cb.AllowRequest() {
		err := o.upstream.Write(ctx, e)
		if err == nil {
			o.cb.RecordSuccess()
			return nil
		}
		o.cb.RecordFailure()
		if o.cb.State() == StateOpen {
			return o.appendToFile(e)
		}
		return err
	}
	return o.appendToFile(e)
}

// PendingCount returns the number of NDJSON lines in the outbox file.
func (o *Outbox) PendingCount() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	f, err := os.Open(o.filePath)
	if err != nil {
		return 0
	}
	defer f.Close()
	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if len(scanner.Bytes()) > 0 {
			count++
		}
	}
	return count
}

// CircuitState returns the current circuit breaker state.
func (o *Outbox) CircuitState() CBState {
	return o.cb.State()
}

// Close stops the drain goroutine and waits for it to finish.
func (o *Outbox) Close() {
	o.cancel()
	<-o.done
}

func (o *Outbox) appendToFile(e Entry) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	f, err := os.OpenFile(o.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	_, err = f.Write(raw)
	return err
}

func (o *Outbox) drainLoop(ctx context.Context, interval time.Duration) {
	defer close(o.done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.tryDrain(ctx)
		}
	}
}

func (o *Outbox) tryDrain(ctx context.Context) {
	if o.cb.State() == StateOpen {
		return
	}
	o.mu.Lock()
	entries, err := o.readAllLocked()
	if err != nil || len(entries) == 0 {
		o.mu.Unlock()
		return
	}
	// Truncate the file before releasing the lock so new writes don't
	// interleave with our drain.
	_ = os.Truncate(o.filePath, 0)
	o.mu.Unlock()

	var failed []Entry
	for _, e := range entries {
		if ctx.Err() != nil {
			failed = append(failed, e)
			continue
		}
		if err := o.upstream.Write(ctx, e); err != nil {
			o.cb.RecordFailure()
			failed = append(failed, e)
			// Re-queue remaining entries without attempting them.
			continue
		}
		o.cb.RecordSuccess()
	}
	if len(failed) > 0 {
		o.mu.Lock()
		for _, e := range failed {
			_ = o.appendToFileLocked(e)
		}
		o.mu.Unlock()
	}
}

func (o *Outbox) readAllLocked() ([]Entry, error) {
	f, err := os.Open(o.filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}

func (o *Outbox) appendToFileLocked(e Entry) error {
	f, err := os.OpenFile(o.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	_, err = f.Write(raw)
	return err
}
