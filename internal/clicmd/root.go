// runx-public-repo-gate: allow-file network_topology — documents the canonical Mem0 OSS loopback endpoint (127.0.0.1:18888) used as the local CLI default; not a personal-stack tunnel.
package clicmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nfsarch33/mem0-mcp-go/internal/cliconfig"
	"github.com/nfsarch33/mem0-mcp-go/internal/mem0"
	"github.com/nfsarch33/mem0-mcp-go/internal/tools"
)

// Run dispatches CLI subcommands. Returns a process-style exit code.
//
// args is the argv after the leading "cli" token (i.e., os.Args[2:]).
// stdout and stderr are injectable for testing.
//
// Subcommands:
//
//	add            POST /memories
//	search         POST /search
//	get-memories   GET  /memories
//	get            GET  /memories/{id}
//	delete         DELETE /memories/{id}
//	list-entities  GET  /entities
//	health         GET  /openapi.json (or /healthz, /docs)
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		printHelp(stdout)
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	sub := args[0]
	rest := args[1:]

	cfgPath := cliconfig.DefaultConfigPath
	for i := 0; i < len(rest); i++ {
		if rest[i] == "--config" && i+1 < len(rest) {
			cfgPath = rest[i+1]
			rest = append(rest[:i], rest[i+2:]...)
			break
		}
	}

	resolved, err := cliconfig.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(stderr, "config: %v\n", err)
		return 2
	}
	if sub != "config" { // future: a config-show subcommand could skip Validate
		if err := resolved.Validate(); err != nil {
			fmt.Fprintf(stderr, "config: %v\n", err)
			return 2
		}
	}

	logger := slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: parseLevel(resolved.LogLevel)}))
	client := mem0.NewClient(mem0.Options{
		BaseURL: resolved.BaseURL,
		APIKey:  resolved.APIKey,
		Timeout: resolved.Timeout,
	})

	deps := Deps{
		Client: tools.Mem0Client(client),
		Config: resolved,
		Logger: logger,
		Stdout: stdout,
		Stderr: stderr,
	}

	cancelCtx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var rerr error
	switch sub {
	case "add":
		rerr = cmdAdd(cancelCtx, deps, rest)
	case "search":
		rerr = cmdSearch(cancelCtx, deps, rest)
	case "get":
		rerr = cmdGet(cancelCtx, deps, rest)
	case "get-memories":
		rerr = cmdGetMemories(cancelCtx, deps, rest)
	case "delete":
		rerr = cmdDelete(cancelCtx, deps, rest)
	case "list-entities":
		rerr = cmdListEntities(cancelCtx, deps, rest)
	case "health":
		rerr = cmdHealth(cancelCtx, deps, rest)
	default:
		fmt.Fprintf(stderr, "unknown subcommand: %s\n", sub)
		printHelp(stderr)
		return 2
	}
	if rerr != nil {
		fmt.Fprintf(stderr, "%s: %v\n", sub, rerr)
		return 1
	}
	return 0
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, `mem0-mcp-go cli — runx-friendly Mem0 OSS HTTP client

Usage:
  mem0-mcp-go cli <subcommand> [flags] [--config PATH] [--json]

Subcommands:
  add            Add a memory (--text, --user, --app)
  search         Search memories (--query, --user, --top-k)
  get            Get one memory by id (--memory-id)
  get-memories   List memories (--user, --app)
  delete         Delete a memory (--memory-id)
  list-entities  List user/agent/run entities
  health         Probe server reachability

Config (~/.config/mem0-mcp-go/config.yaml, mode 0600):
  endpoints:
    default:
      base_url: "http://127.0.0.1:18888"
      api_key: "..."
  defaults:
    user_id: "default-user"
    app_id: "default-app"

Precedence: file < env (MEM0_*) < flag.
No credentials may appear on argv.
`)
}
