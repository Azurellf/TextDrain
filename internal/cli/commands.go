package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"textdrain/internal/config"
)

type transcribeOptions struct {
	input            string
	language         string
	model            string
	outputDir        string
	keepIntermediate bool
}

func newTranscribeCommand(cfg config.Config) *cobra.Command {
	opts := transcribeOptions{
		language:         cfg.Language,
		model:            cfg.Model,
		outputDir:        cfg.JobsDir,
		keepIntermediate: cfg.KeepIntermediateFiles,
	}

	cmd := &cobra.Command{
		Use:   "transcribe <url-or-path>",
		Short: "Transcribe a URL or local media file",
		Long:  "Transcribe accepts a yt-dlp compatible URL or a local media path and prepares a transcription job.",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return NewParameterError("transcribe expects exactly one <url-or-path> argument")
			}
			if args[0] == "" {
				return NewParameterError("transcribe input cannot be empty")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.input = args[0]
			if err := validateLanguage(opts.language); err != nil {
				return err
			}
			if opts.model == "" {
				return NewParameterError("--model cannot be empty")
			}
			if opts.outputDir == "" {
				return NewParameterError("--output cannot be empty")
			}

			_, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"input=%s\nlanguage=%s\nmodel=%s\noutput=%s\nkeep_intermediate=%t\n",
				opts.input,
				opts.language,
				opts.model,
				opts.outputDir,
				opts.keepIntermediate,
			)
			return err
		},
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return NewParameterError("%s", err)
	})

	cmd.Flags().StringVar(&opts.language, "lang", opts.language, "Transcription language: auto, zh, or en")
	cmd.Flags().StringVar(&opts.model, "model", opts.model, "Model name to use for transcription")
	cmd.Flags().StringVar(&opts.outputDir, "output", opts.outputDir, "Directory for transcript output")
	cmd.Flags().BoolVar(&opts.keepIntermediate, "keep-intermediate", opts.keepIntermediate, "Keep intermediate media and audio files")

	return cmd
}

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the local TextDrain environment",
		Long:  "Doctor checks the local TextDrain environment and reports dependency or model setup issues.",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return NewParameterError("doctor does not accept arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "doctor checks are not implemented yet")
			return err
		},
	}
}

func newModelsCommand(cfg config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Manage local transcription models",
		Long:  "Models lists local transcription models from the configured model directory.",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return NewParameterError("models does not accept positional arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			list, err := cmd.Flags().GetBool("list")
			if err != nil {
				return NewParameterError("%s", err)
			}
			if !list {
				return cmd.Help()
			}
			return listModels(cmd, cfg.ModelDir)
		},
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return NewParameterError("%s", err)
	})
	cmd.Flags().Bool("list", false, "List models in the configured model directory")

	return cmd
}

func validateLanguage(language string) error {
	switch language {
	case "auto", "zh", "en":
		return nil
	default:
		return NewParameterError("--lang must be one of auto, zh, or en")
	}
}

func listModels(cmd *cobra.Command, modelDir string) error {
	if modelDir == "" {
		return NewParameterError("model directory is not configured")
	}

	entries, err := os.ReadDir(modelDir)
	if err != nil {
		if os.IsNotExist(err) {
			_, writeErr := fmt.Fprintf(cmd.OutOrStdout(), "model_dir=%s\nmodels=0\n", modelDir)
			return writeErr
		}
		return NewRuntimeError("read model directory %s: %w", modelDir, err)
	}

	models := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		models = append(models, entry.Name())
	}
	sort.Strings(models)

	out := cmd.OutOrStdout()
	if _, err := fmt.Fprintf(out, "model_dir=%s\nmodels=%d\n", filepath.Clean(modelDir), len(models)); err != nil {
		return err
	}
	for _, model := range models {
		if _, err := fmt.Fprintf(out, "%s\n", model); err != nil {
			return err
		}
	}

	return nil
}
