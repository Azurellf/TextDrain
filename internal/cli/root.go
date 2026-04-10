package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"textdrain/internal/config"
)

type RootOptions struct {
	Paths config.Paths
	UI    *UI
}

func NewRootCommand(_ context.Context, opts RootOptions) *cobra.Command {
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

	cmd.AddCommand(newPathsCommand(opts.Paths))

	return cmd
}

func newPathsCommand(paths config.Paths) *cobra.Command {
	return &cobra.Command{
		Use:    "paths",
		Short:  "Show default directories",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"config=%s\ncache=%s\njobs=%s\nmodels=%s\n",
				paths.ConfigDir,
				paths.CacheDir,
				paths.JobsDir,
				paths.ModelsDir,
			)
			return err
		},
	}
}
