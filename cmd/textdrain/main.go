package main

import (
	"context"
	"os"

	"textdrain/internal/app"
)

func main() {
	ctx := context.Background()
	application := app.New()

	if err := application.Run(ctx, os.Args[1:]); err != nil {
		os.Exit(1)
	}
}
