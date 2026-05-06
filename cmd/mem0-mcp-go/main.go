package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/nfsarch33/mem0-mcp-go/internal/config"
	"github.com/nfsarch33/mem0-mcp-go/internal/mem0"
	"github.com/nfsarch33/mem0-mcp-go/internal/server"
	"github.com/nfsarch33/mem0-mcp-go/internal/tools"
)

const version = "0.1.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "mem0-mcp-go: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("mem0-mcp-go", flag.ContinueOnError)
	wantVersion := fs.Bool("version", false, "Print version and exit.")
	wantHelp := fs.Bool("help", false, "Print usage and exit.")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *wantHelp {
		printUsage()
		return nil
	}
	if *wantVersion {
		fmt.Printf("mem0-mcp-go %s\n", version)
		return nil
	}
	cfg := config.Load()
	logger := buildLogger(cfg.LogLevel)
	client := mem0.NewClient(mem0.Options{BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Timeout: cfg.Timeout})
	registry := tools.NewRegistry(client, tools.Defaults{UserID: cfg.UserID, AppID: cfg.AppID})
	srv := server.New(registry, logger, version)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return srv.Run(ctx, cfg.Transport, cfg.SSEAddr)
}

func buildLogger(level string) *slog.Logger {
	var slevel slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		slevel = slog.LevelDebug
	case "warn", "warning":
		slevel = slog.LevelWarn
	case "error":
		slevel = slog.LevelError
	default:
		slevel = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slevel}))
}

func printUsage() {
	fmt.Printf(`mem0-mcp-go %s

Go-native MCP server for self-hosted Mem0 OSS.

Usage:
  mem0-mcp-go [--version|--help]

Env:
  MEM0_BASE_URL          Self-hosted Mem0 base URL (default http://127.0.0.1:18888)
  MEM0_API_KEY           Mem0 OSS API key (X-API-Key)
  MEM0_USER_ID           Default Mem0 user id (default nfsarch33)
  MEM0_DEFAULT_USER_ID   Compatibility fallback for old MCP configs
  MEM0_APP_ID            Default app id (default cursor-global-kb)
  MCP_TRANSPORT          stdio | sse (default stdio)
  MCP_SSE_ADDR           Bind addr for sse (default :9092)
  LOG_LEVEL              debug | info | warn | error

No credentials may appear on argv.
`, version)
}
