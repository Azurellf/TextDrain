package main

import (
	"context"
	"os"

	"textdrain/internal/app"
	"textdrain/internal/cli"
)

func main() {
	ctx := context.Background()
	application := app.New()

	if err := application.Run(ctx, os.Args[1:]); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}
