package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/nfsarch33/mem0-mcp-go/internal/tools"
)

type Server struct {
	logger    *slog.Logger
	version   string
	mcp       *mcpserver.MCPServer
	toolCount int
}

func New(registry *tools.Registry, logger *slog.Logger, version string) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{logger: logger, version: version}
	s.mcp = s.buildMCPServer(registry)
	return s
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
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","transport":"sse","tools":%d}`, s.toolCount)
	})
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
