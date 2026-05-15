package clicmd

import (
	"context"
	"flag"
)

func cmdGetMemories(ctx context.Context, deps Deps, args []string) error {
	fs := flag.NewFlagSet("get-memories", flag.ContinueOnError)
	fs.SetOutput(deps.Stderr)
	user := fs.String("user", "", "user id")
	app := fs.String("app", "", "app id")
	limit := fs.Int("limit", 0, "max rows (0 = server default)")
	jsonOut := fs.Bool("json", false, "emit JSON response")
	if err := fs.Parse(args); err != nil {
		return err
	}
	out, err := deps.Client.GetAll(ctx,
		chooseString(*user, deps.Config.UserID),
		chooseString(*app, deps.Config.AppID),
		*limit,
	)
	if err != nil {
		return err
	}
	return emit(deps.Stdout, *jsonOut, out)
}