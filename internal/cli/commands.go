package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"textdrain/internal/app/transcription"
	"textdrain/internal/config"
	"textdrain/internal/domain"
	"textdrain/internal/infra/asr"
	"textdrain/internal/infra/downloader"
	"textdrain/internal/infra/environment"
	"textdrain/internal/infra/exporter"
	"textdrain/internal/infra/ingestion"
	"textdrain/internal/infra/media"
	"textdrain/internal/infra/models"
)

type transcribeOptions struct {
	input              string
	language           string
	model              string
	outputDir          string
	keepIntermediate   bool
	cookiesFromBrowser string
	cookiesFile        string
}

func newTranscribeCommand(cfg config.Config, transcriber Transcriber) *cobra.Command {
	opts := transcribeOptions{
		language:         cfg.Language,
		model:            cfg.Model,
		outputDir:        "",
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
			if opts.cookiesFromBrowser != "" && opts.cookiesFile != "" {
				return NewParameterError("--cookies-from-browser and --cookies cannot be used together")
			}
			if transcriber == nil {
				transcriber = newDefaultTranscriber(cfg, opts, cmd.OutOrStdout())
			}

			result, err := transcriber.Run(cmd.Context(), transcription.Request{
				Input:            opts.input,
				Language:         opts.language,
				Model:            opts.model,
				OutputDir:        opts.outputDir,
				Formats:          cfg.OutputFormats,
				KeepIntermediate: opts.keepIntermediate,
			})
			if err != nil {
				return NewPipelineError(err)
			}

			return printTranscribeResult(cmd, result)
		},
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return NewParameterError("%s", err)
	})

	cmd.Flags().StringVar(&opts.language, "lang", opts.language, "Transcription language: auto, zh, or en")
	cmd.Flags().StringVar(&opts.model, "model", opts.model, "Model name to use for transcription")
	cmd.Flags().StringVar(&opts.outputDir, "output", opts.outputDir, "Directory for transcript output")
	cmd.Flags().BoolVar(&opts.keepIntermediate, "keep-intermediate", opts.keepIntermediate, "Keep intermediate media and audio files")
	cmd.Flags().StringVar(&opts.cookiesFromBrowser, "cookies-from-browser", opts.cookiesFromBrowser, "Browser profile to read yt-dlp cookies from, such as chrome or safari")
	cmd.Flags().StringVar(&opts.cookiesFile, "cookies", opts.cookiesFile, "Path to a cookies.txt file for yt-dlp")

	return cmd
}

type statusWriter struct {
	out io.Writer
}

func (w statusWriter) Update(_ context.Context, status domain.JobStatus) error {
	_, err := fmt.Fprintf(w.out, "stage=%s\n", status)
	return err
}

func newDefaultTranscriber(cfg config.Config, opts transcribeOptions, out io.Writer) Transcriber {
	ytdlp := downloader.NewYTDLPWithOptions("", downloader.YTDLPOptions{
		CookiesFromBrowser: opts.cookiesFromBrowser,
		CookiesFile:        opts.cookiesFile,
	})
	return transcription.NewUseCase(transcription.Dependencies{
		Resolver:       ingestion.NewResolver(cfg.JobsDir, opts.language),
		URLInspector:   ytdlp,
		Downloader:     ytdlp,
		AudioProcessor: media.NewFFmpeg(),
		ASREngine:      asr.NewWhisperCLI(cfg.ModelDir),
		Exporter:       exporter.New(),
		Reporter:       statusWriter{out: out},
	})
}

func printTranscribeResult(cmd *cobra.Command, result transcription.Result) error {
	out := cmd.OutOrStdout()
	if _, err := fmt.Fprintf(out, "job_id=%s\nwork_dir=%s\noutputs=%d\n", result.JobID, result.WorkDir, len(result.OutputPaths)); err != nil {
		return err
	}
	if result.Asset.MediaPath != "" {
		if _, err := fmt.Fprintf(out, "media_path=%s\n", result.Asset.MediaPath); err != nil {
			return err
		}
	}
	if result.Audio.Path != "" {
		if _, err := fmt.Fprintf(out, "audio_path=%s\n", result.Audio.Path); err != nil {
			return err
		}
	}
	for _, path := range result.OutputPaths {
		if _, err := fmt.Fprintf(out, "output=%s\n", path); err != nil {
			return err
		}
	}
	if len(result.OutputPaths) > 0 {
		if _, err := fmt.Fprintf(out, "final_output=%s\n", result.OutputPaths[len(result.OutputPaths)-1]); err != nil {
			return err
		}
	}
	return nil
}

func newDoctorCommand(paths config.Paths, cfg config.Config) *cobra.Command {
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
			report := environment.Check(cmd.Context(), paths, cfg)
			if err := printDoctorReport(cmd, report); err != nil {
				return err
			}
			if !report.Healthy() {
				return NewDependencyError("environment checks failed")
			}
			return nil
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
			return listModels(cmd, cfg)
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

func listModels(cmd *cobra.Command, cfg config.Config) error {
	discovered, err := models.Discover(cfg.ModelDir, cfg.Model)
	if err != nil {
		return NewRuntimeError("read model directory %s: %w", cfg.ModelDir, err)
	}

	out := cmd.OutOrStdout()
	if _, err := fmt.Fprintf(out, "model_dir=%s\ndefault_model=%s\nmodels=%d\n", filepath.Clean(cfg.ModelDir), cfg.Model, len(discovered)); err != nil {
		return err
	}
	for _, model := range discovered {
		if _, err := fmt.Fprintf(out, "model name=%s path=%s size_bytes=%d default=%t\n", model.Name, model.Path, model.SizeBytes, model.Default); err != nil {
			return err
		}
	}

	return nil
}

func printDoctorReport(cmd *cobra.Command, report environment.Report) error {
	out := cmd.OutOrStdout()

	if _, err := fmt.Fprintln(out, "TextDrain environment check"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, ""); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "Tools:"); err != nil {
		return err
	}
	for _, tool := range []environment.ToolCheck{report.Tools.YTDLP, report.Tools.FFmpeg, report.Tools.WhisperCLI} {
		if err := printToolCheck(out, tool); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(out, ""); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "Models:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  default_model=%s\n", report.Model.Name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  model_dir=%s\n", report.Model.Dir); err != nil {
		return err
	}
	if report.Model.Found {
		if _, err := fmt.Fprintf(out, "  model_file=ok path=%s\n", report.Model.Path); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(out, "  model_file=missing reason=%s\n", report.Model.Error); err != nil {
			return err
		}
		for _, candidate := range report.Model.Candidates {
			if _, err := fmt.Fprintf(out, "  candidate=%s\n", candidate); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(out, "  advice=%s\n", report.Model.Advice); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(out, ""); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "Paths:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  config_file=%s\n", report.Paths.ConfigFile); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  cache=%s\n", report.Paths.CacheDir); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  jobs=%s\n", report.Paths.JobsDir); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  models=%s\n", report.Paths.ModelDir); err != nil {
		return err
	}

	if report.Healthy() {
		_, err := fmt.Fprintln(out, "\nStatus: ok")
		return err
	}
	_, err := fmt.Fprintln(out, "\nStatus: failed")
	return err
}

func printToolCheck(out io.Writer, tool environment.ToolCheck) error {
	status := "missing"
	if tool.Found && tool.Executable {
		status = "ok"
	} else if tool.Found {
		status = "not_executable"
	}

	if _, err := fmt.Fprintf(out, "  %s=%s\n", tool.Name, status); err != nil {
		return err
	}
	if tool.Path != "" {
		if _, err := fmt.Fprintf(out, "    path=%s\n", tool.Path); err != nil {
			return err
		}
	}
	if tool.Version != "" {
		if _, err := fmt.Fprintf(out, "    version=%s\n", tool.Version); err != nil {
			return err
		}
	}
	if tool.Error != "" {
		if _, err := fmt.Fprintf(out, "    reason=%s\n", tool.Error); err != nil {
			return err
		}
	}
	if status != "ok" {
		if _, err := fmt.Fprintf(out, "    advice=%s\n", tool.Advice); err != nil {
			return err
		}
	}
	return nil
}
