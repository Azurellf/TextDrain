package app

import (
	"context"
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
		configErr := cli.NewConfigError("load config: %w", err)
		_ = cli.FprintError(a.ui.Stderr, configErr)
		return configErr
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
		_ = cli.FprintError(a.ui.Stderr, err)
		return err
	}

	return nil
}
