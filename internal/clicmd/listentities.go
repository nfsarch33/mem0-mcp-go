package clicmd

import (
	"context"
	"flag"
)

func cmdListEntities(ctx context.Context, deps Deps, args []string) error {
	fs := flag.NewFlagSet("list-entities", flag.ContinueOnError)
	fs.SetOutput(deps.Stderr)
	jsonOut := fs.Bool("json", false, "emit JSON response")
	if err := fs.Parse(args); err != nil {
		return err
	}
	out, err := deps.Client.ListEntities(ctx)
	if err != nil {
		return err
	}
	return emit(deps.Stdout, *jsonOut, out)
}