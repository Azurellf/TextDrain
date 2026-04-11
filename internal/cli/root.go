package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"textdrain/internal/config"
)

type RootOptions struct {
	Paths  config.Paths
	Config config.Config
	UI     *UI
}

func NewRootCommand(_ context.Context, opts RootOptions) *cobra.Command {
	cfg := opts.Config
	if cfg.ModelDir == "" && cfg.JobsDir == "" {
		cfg = config.Default(opts.Paths)
	}

	cmd := &cobra.Command{
		Use:   "textdrain",
		Short: "Offline media transcription CLI",
		Long:  "TextDrain is a local-first CLI for downloading media, preparing audio, and exporting transcripts.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.Version = "0.1.0"
	cmd.SetVersionTemplate("{{printf \"%s\\n\" .Version}}")
	if opts.UI != nil {
		cmd.SetOut(opts.UI.Stdout)
		cmd.SetErr(opts.UI.Stderr)
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return NewParameterError("%s", err)
	})

	cmd.AddCommand(newPathsCommand(opts.Paths, cfg))
	cmd.AddCommand(newTranscribeCommand(cfg))
	cmd.AddCommand(newDoctorCommand())
	cmd.AddCommand(newModelsCommand(cfg))

	return cmd
}

func newPathsCommand(paths config.Paths, cfg config.Config) *cobra.Command {
	return &cobra.Command{
		Use:    "paths",
		Short:  "Show configured directories",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"config=%s\nconfig_file=%s\ncache=%s\njobs=%s\nmodels=%s\n",
				paths.ConfigDir,
				paths.ConfigFile,
				paths.CacheDir,
				cfg.JobsDir,
				cfg.ModelDir,
			)
			return err
		},
	}
}
