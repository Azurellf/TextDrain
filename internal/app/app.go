package app

import (
	"context"
	"fmt"
	"os"

	"textdrain/internal/cli"
	"textdrain/internal/config"
	"textdrain/internal/infra/logging"
)

type App struct {
	paths  config.Paths
	logger *logging.Logger
	ui     *cli.UI
}

func New() *App {
	return &App{
		paths:  config.DefaultPaths(),
		logger: logging.New(),
		ui:     cli.NewUI(os.Stdout, os.Stderr),
	}
}

func (a *App) Run(ctx context.Context, args []string) error {
	rootCmd := cli.NewRootCommand(ctx, cli.RootOptions{
		Paths: a.paths,
		UI:    a.ui,
	})
	rootCmd.SetArgs(args)

	rootCmd.SetErrPrefix("Error: ")

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		a.logger.Error("command execution failed", "error", err)
		return fmt.Errorf("execute command: %w", err)
	}

	return nil
}
