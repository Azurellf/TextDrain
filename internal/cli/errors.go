package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"textdrain/internal/app/transcription"
	"textdrain/internal/domain"
)

const (
	ExitSuccess    = 0
	ExitParameter  = 2
	ExitConfig     = 2
	ExitDependency = 3
	ExitRuntime    = 4
)

type ErrorKind string

const (
	ErrorKindParameter  ErrorKind = "parameter"
	ErrorKindConfig     ErrorKind = "config"
	ErrorKindDependency ErrorKind = "dependency"
	ErrorKindDownload   ErrorKind = "download"
	ErrorKindMedia      ErrorKind = "media"
	ErrorKindASR        ErrorKind = "asr"
	ErrorKindExport     ErrorKind = "export"
	ErrorKindRuntime    ErrorKind = "runtime"
)

type ExitError struct {
	Code   int
	Kind   ErrorKind
	Stage  string
	Reason string
	Advice string
	Err    error
}

func (e *ExitError) Error() string {
	if e.Reason != "" {
		return e.Reason
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return string(e.Kind)
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

func NewParameterError(format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	return &ExitError{
		Code:   ExitParameter,
		Kind:   ErrorKindParameter,
		Stage:  "INPUT",
		Reason: err.Error(),
		Advice: "Run `textdrain <command> --help` and check the arguments.",
		Err:    err,
	}
}

func NewConfigError(format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	return &ExitError{
		Code:   ExitConfig,
		Kind:   ErrorKindConfig,
		Stage:  "CONFIG",
		Reason: err.Error(),
		Advice: "Check config.toml or run `textdrain paths` to inspect TextDrain paths.",
		Err:    err,
	}
}

func NewDependencyError(format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	return &ExitError{
		Code:   ExitDependency,
		Kind:   ErrorKindDependency,
		Stage:  "DEPENDENCY",
		Reason: err.Error(),
		Advice: "Install the missing tool, make it executable, and ensure it is available on PATH.",
		Err:    err,
	}
}

func NewRuntimeError(format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	return &ExitError{
		Code:   ExitRuntime,
		Kind:   ErrorKindRuntime,
		Stage:  "RUNTIME",
		Reason: err.Error(),
		Advice: "Re-run the command with the same input. If it fails again, check the underlying tool output.",
		Err:    err,
	}
}

func NewPipelineError(err error) error {
	if err == nil {
		return nil
	}

	var stageErr *transcription.StageError
	if !errors.As(err, &stageErr) {
		return NewRuntimeError("%w", err)
	}

	kind, code, advice := pipelineErrorDetails(stageErr.Stage, stageErr.Err)
	reason := stageErr.Err.Error()
	return &ExitError{
		Code:   code,
		Kind:   kind,
		Stage:  string(stageErr.Stage),
		Reason: reason,
		Advice: advice,
		Err:    err,
	}
}

func FprintError(out io.Writer, err error) error {
	_, writeErr := fmt.Fprint(out, FormatError(err))
	return writeErr
}

func FormatError(err error) string {
	if err == nil {
		return ""
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		exitErr = &ExitError{
			Code:   ExitRuntime,
			Kind:   ErrorKindRuntime,
			Stage:  "RUNTIME",
			Reason: err.Error(),
			Advice: "Re-run the command. If it fails again, inspect the command output above.",
			Err:    err,
		}
	}

	stage := firstNonEmpty(exitErr.Stage, "RUNTIME")
	kind := firstNonEmpty(string(exitErr.Kind), string(ErrorKindRuntime))
	reason := firstNonEmpty(exitErr.Reason, err.Error())
	advice := firstNonEmpty(exitErr.Advice, "Check the command input and local environment, then try again.")

	return fmt.Sprintf("Error:\n  stage: %s\n  type: %s\n  reason: %s\n  advice: %s\n", stage, kind, reason, advice)
}

func ExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}

	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}

	return ExitRuntime
}

func pipelineErrorDetails(stage domain.JobStatus, err error) (ErrorKind, int, string) {
	reason := ""
	if err != nil {
		reason = strings.ToLower(err.Error())
	}
	if strings.Contains(reason, "executable file not found") || strings.Contains(reason, "not found in $path") {
		return ErrorKindDependency, ExitDependency, "Install the required external tool and ensure it is available on PATH."
	}
	if strings.Contains(reason, "no supported javascript runtime") {
		return ErrorKindDependency, ExitDependency, "Install deno or configure yt-dlp with --js-runtimes for another supported JavaScript runtime."
	}
	if strings.Contains(reason, "sign in to confirm") || strings.Contains(reason, "not a bot") || strings.Contains(reason, "cookies-from-browser") || strings.Contains(reason, "cookies for the authentication") {
		return ErrorKindDownload, ExitRuntime, "Pass YouTube login cookies with --cookies-from-browser <browser> or an exported cookies.txt file with --cookies <path>."
	}
	if strings.Contains(reason, "impersonate target") || strings.Contains(reason, "list-impersonate-targets") {
		return ErrorKindDependency, ExitDependency, "The requested yt-dlp impersonation target is unavailable in this environment. Install the optional impersonation dependency and verify it with `uv run yt-dlp --list-impersonate-targets`."
	}
	if strings.Contains(reason, "http error 412") || strings.Contains(reason, "precondition failed") {
		return ErrorKindDownload, ExitRuntime, "The site rejected the metadata request. Retry with browser cookies first, and add yt-dlp flags such as --yt-dlp-arg=--impersonate --yt-dlp-arg=chrome when the site also requires a browser-like client."
	}

	switch stage {
	case domain.JobStatusResolving:
		return ErrorKindParameter, ExitParameter, "Check that the input path exists or the URL is valid."
	case domain.JobStatusDownloading:
		return ErrorKindDownload, ExitRuntime, "Check the URL, network access, and yt-dlp installation."
	case domain.JobStatusExtractingAudio:
		return ErrorKindMedia, ExitRuntime, "Check that the media file is readable by ffmpeg."
	case domain.JobStatusTranscribing:
		return ErrorKindASR, ExitRuntime, "Check that whisper-cli is installed and the selected model exists."
	case domain.JobStatusExporting:
		return ErrorKindExport, ExitRuntime, "Check the output directory permissions and requested export formats."
	default:
		return ErrorKindRuntime, ExitRuntime, "Check the command output above and try again."
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
