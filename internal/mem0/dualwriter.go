package mem0

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// DualWriterConfig controls fan-out behaviour.
type DualWriterConfig struct {
	// ReadSource selects the backend for read operations: "oss" or "cloud".
	ReadSource string

	// ShadowTimeout bounds async shadow/backup writes so goroutines
	// do not leak indefinitely. Defaults to 30s if zero.
	ShadowTimeout time.Duration
}

// DualWriter wraps a primary client and fans writes to shadow (cloud)
// and optionally backup targets. Reads are routed to either primary or
// shadow based on ReadSource.
type DualWriter struct {
	primary *Client
	shadow  *Client
	backup  *Client
	cfg     DualWriterConfig
	logger  *slog.Logger
	mu      sync.Mutex
	stats   DualWriteStats
}

// DualWriteStats tracks shadow/backup write outcomes.
type DualWriteStats struct {
	ShadowWrites int64
	ShadowErrors int64
	BackupWrites int64
	BackupErrors int64
}

// NewDualWriter creates a fan-out writer. shadow and backup may be nil.
func NewDualWriter(primary, shadow, backup *Client, cfg DualWriterConfig, logger *slog.Logger) *DualWriter {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.ShadowTimeout == 0 {
		cfg.ShadowTimeout = 30 * time.Second
	}
	return &DualWriter{
		primary: primary,
		shadow:  shadow,
		backup:  backup,
		cfg:     cfg,
		logger:  logger,
	}
}

// Stats returns a snapshot of dual-write counters.
func (dw *DualWriter) Stats() DualWriteStats {
	dw.mu.Lock()
	defer dw.mu.Unlock()
	return dw.stats
}

func (dw *DualWriter) readClient() *Client {
	if dw.cfg.ReadSource == "cloud" && dw.shadow != nil {
		return dw.shadow
	}
	return dw.primary
}

func (dw *DualWriter) Add(ctx context.Context, req MemoryRequest) (map[string]any, error) {
	out, err := dw.primary.Add(ctx, req)
	if err != nil {
		return nil, err
	}
	dw.fanOutAsync("add", func(c *Client, fctx context.Context) error {
		_, e := c.Add(fctx, req)
		return e
	})
	return out, nil
}

func (dw *DualWriter) Search(ctx context.Context, req SearchRequest) (map[string]any, error) {
	return dw.readClient().Search(ctx, req)
}

func (dw *DualWriter) Get(ctx context.Context, id string) (map[string]any, error) {
	return dw.readClient().Get(ctx, id)
}

func (dw *DualWriter) GetAll(ctx context.Context, userID, appID string, limit int) (map[string]any, error) {
	return dw.readClient().GetAll(ctx, userID, appID, limit)
}

func (dw *DualWriter) Update(ctx context.Context, id, memory string, metadata map[string]any) (map[string]any, error) {
	out, err := dw.primary.Update(ctx, id, memory, metadata)
	if err != nil {
		return nil, err
	}
	dw.fanOutAsync("update", func(c *Client, fctx context.Context) error {
		_, e := c.Update(fctx, id, memory, metadata)
		return e
	})
	return out, nil
}

func (dw *DualWriter) Delete(ctx context.Context, id string) (map[string]any, error) {
	out, err := dw.primary.Delete(ctx, id)
	if err != nil {
		return nil, err
	}
	dw.fanOutAsync("delete", func(c *Client, fctx context.Context) error {
		_, e := c.Delete(fctx, id)
		return e
	})
	return out, nil
}

func (dw *DualWriter) History(ctx context.Context, id string) (map[string]any, error) {
	return dw.readClient().History(ctx, id)
}

func (dw *DualWriter) Doctor(ctx context.Context) error {
	return dw.primary.Doctor(ctx)
}

// fanOutAsync fires shadow and backup writes in background goroutines.
// Errors are logged but never propagated to the caller.
func (dw *DualWriter) fanOutAsync(op string, fn func(c *Client, ctx context.Context) error) {
	if dw.shadow != nil {
		go dw.asyncWrite("shadow", op, dw.shadow, fn)
	}
	if dw.backup != nil {
		go dw.asyncWrite("backup", op, dw.backup, fn)
	}
}

func (dw *DualWriter) asyncWrite(target, op string, c *Client, fn func(*Client, context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), dw.cfg.ShadowTimeout)
	defer cancel()

	err := fn(c, ctx)

	dw.mu.Lock()
	defer dw.mu.Unlock()

	switch target {
	case "shadow":
		if err != nil {
			dw.stats.ShadowErrors++
			dw.logger.Warn("shadow write failed", "op", op, "target", target, "error", err)
		} else {
			dw.stats.ShadowWrites++
			dw.logger.Debug("shadow write ok", "op", op, "target", target)
		}
	case "backup":
		if err != nil {
			dw.stats.BackupErrors++
			dw.logger.Warn("backup write failed", "op", op, "target", target, "error", err)
		} else {
			dw.stats.BackupWrites++
			dw.logger.Debug("backup write ok", "op", op, "target", target)
		}
	}
}
