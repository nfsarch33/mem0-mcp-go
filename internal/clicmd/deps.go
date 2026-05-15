// Package clicmd implements the runx-friendly CLI subcommand surface for
// mem0-mcp-go. The same Go binary is both an MCP server (default) and a
// thin HTTP client driven by ~/.config/mem0-mcp-go/config.yaml.
//
// All commands share a single Mem0Client port (defined in internal/tools)
// so the MCP and CLI surfaces never diverge. No subcommand reads or writes
// secrets to argv: API key + base URL come from the YAML config or env.
package clicmd

import (
	"io"
	"log/slog"

	"github.com/nfsarch33/mem0-mcp-go/internal/cliconfig"
	"github.com/nfsarch33/mem0-mcp-go/internal/tools"
)

// Deps is the bag of collaborators each subcommand needs. It is built once
// per invocation by the dispatcher and threaded through all subcommands.
type Deps struct {
	Client tools.Mem0Client
	Config *cliconfig.Resolved
	Logger *slog.Logger
	Stdout io.Writer
	Stderr io.Writer
}