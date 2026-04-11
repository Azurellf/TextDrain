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
	config config.Config
	logger *logging.Logger
	ui     *cli.UI
}

func New() *App {
	paths := config.DefaultPaths()

	return &App{
		paths:  paths,
		config: config.Default(paths),
		logger: logging.New(),
		ui:     cli.NewUI(os.Stdout, os.Stderr),
	}
}

func (a *App) Run(ctx context.Context, args []string) error {
	cfg, err := config.Load(a.paths, config.Overrides{})
	if err != nil {
		a.logger.Error("config loading failed", "error", err)
		return fmt.Errorf("load config: %w", err)
	}
	a.config = cfg

	rootCmd := cli.NewRootCommand(ctx, cli.RootOptions{
		Paths:  a.paths,
		Config: a.config,
		UI:     a.ui,
	})
	rootCmd.SetArgs(args)

	rootCmd.SetErrPrefix("Error: ")

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintf(a.ui.Stderr, "Error: %v\n", err)
		return fmt.Errorf("execute command: %w", err)
	}

	return nil
}
