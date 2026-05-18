package clicmd

import (
	"context"
	"flag"
)

func cmdHealth(ctx context.Context, deps Deps, args []string) error {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	fs.SetOutput(deps.Stderr)
	jsonOut := fs.Bool("json", false, "emit JSON response")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := deps.Client.Doctor(ctx); err != nil {
		return err
	}
	return emit(deps.Stdout, *jsonOut, map[string]any{
		"status":   "ok",
		"base_url": deps.Config.BaseURL,
	})
}
