package clicmd

import (
	"context"
	"fmt"
)

func cmdHealth(ctx context.Context, deps Deps, args []string) error {
	if err := deps.Client.Doctor(ctx); err != nil {
		return err
	}
	_, err := fmt.Fprintln(deps.Stdout, "ok")
	return err
}
