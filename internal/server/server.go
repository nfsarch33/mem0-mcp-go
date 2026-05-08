package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/nfsarch33/mem0-mcp-go/internal/tools"
)

// DualWriteStatus is reported on the health endpoint when dual-write is active.
type DualWriteStatus struct {
	Enabled    bool   `json:"enabled"`
	ReadSource string `json:"read_source,omitempty"`
	HasCloud   bool   `json:"has_cloud"`
	HasBackup  bool   `json:"has_backup"`
}

type Server struct {
	logger          *slog.Logger
	version         string
	mcp             *mcpserver.MCPServer
	toolCount       int
	dualWriteStatus DualWriteStatus
}

func New(registry *tools.Registry, logger *slog.Logger, version string) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{logger: logger, version: version}
	s.mcp = s.buildMCPServer(registry)
	return s
}

// SetDualWriteStatus records dual-write config so the health endpoint can report it.
func (s *Server) SetDualWriteStatus(status DualWriteStatus) {
	s.dualWriteStatus = status
}

func (s *Server) buildMCPServer(registry *tools.Registry) *mcpserver.MCPServer {
	srv := mcpserver.NewMCPServer("mem0-mcp-go", s.version, mcpserver.WithToolCapabilities(true))
	add := func(t mcp.Tool, h mcpserver.ToolHandlerFunc) {
		srv.AddTool(t, h)
		s.toolCount++
	}
	for _, tool := range registry.Tools() {
		add(tool.Tool, tool.Handler)
	}
	return srv
}

func (s *Server) MCPServer() *mcpserver.MCPServer { return s.mcp }

func (s *Server) ToolCount() int { return s.toolCount }

func (s *Server) Run(ctx context.Context, transport, sseAddr string) error {
	s.logger.Info("mem0-mcp-go ready", "transport", transport, "tools", s.toolCount)
	switch transport {
	case "stdio":
		stdio := mcpserver.NewStdioServer(s.mcp)
		return stdio.Listen(ctx, os.Stdin, os.Stdout)
	case "sse":
		return s.runSSE(ctx, sseAddr)
	default:
		return fmt.Errorf("unknown transport %q", transport)
	}
}

func (s *Server) runSSE(ctx context.Context, addr string) error {
	sse := mcpserver.NewSSEServer(s.mcp,
		mcpserver.WithBaseURL(fmt.Sprintf("http://localhost%s", addr)),
		mcpserver.WithKeepAlive(true),
	)
	mux := http.NewServeMux()
	mux.Handle("/", sse)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/healthz", s.handleHealth)
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		_ = sse.Shutdown(context.Background())
		_ = srv.Close()
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("sse server: %w", err)
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	payload := map[string]any{
		"status":     "ok",
		"transport":  "sse",
		"tools":      s.toolCount,
		"dual_write": s.dualWriteStatus,
	}
	_ = json.NewEncoder(w).Encode(payload)
}
