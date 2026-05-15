package clicmd

import (
	"context"
	"flag"
	"fmt"
)

func cmdDelete(ctx context.Context, deps Deps, args []string) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(deps.Stderr)
	id := fs.String("memory-id", "", "memory id (required)")
	jsonOut := fs.Bool("json", false, "emit JSON response")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return fmt.Errorf("--memory-id is required")
	}
	out, err := deps.Client.Delete(ctx, *id)
	if err != nil {
		return err
	}
	return emit(deps.Stdout, *jsonOut, out)
}