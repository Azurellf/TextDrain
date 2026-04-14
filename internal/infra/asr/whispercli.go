// Package asr contains local speech recognition integrations.
package asr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"textdrain/internal/domain"
)

const (
	defaultWhisperCLIBinary = "whisper-cli"
	defaultLanguageMode     = "auto"
	whisperEngineName       = "whisper.cpp"
)

var (
	ErrEmptyAudioPath    = errors.New("audio path cannot be empty")
	ErrAudioNotFile      = errors.New("audio path is not a file")
	ErrMissingModel      = errors.New("whisper model path cannot be empty")
	ErrUnsupportedLang   = errors.New("unsupported transcription language")
	ErrMissingTranscript = errors.New("whisper-cli did not produce a transcript")
	ErrInvalidTranscript = errors.New("whisper-cli produced an invalid transcript")
)

// WhisperCLI shells out to whisper.cpp's whisper-cli binary.
type WhisperCLI struct {
	binary   string
	modelDir string
}

var _ domain.ASREngine = (*WhisperCLI)(nil)

// NewWhisperCLI creates an ASR engine using whisper-cli from PATH.
func NewWhisperCLI(modelDir string) *WhisperCLI {
	return &WhisperCLI{
		binary:   defaultWhisperCLIBinary,
		modelDir: strings.TrimSpace(modelDir),
	}
}

// NewWhisperCLIWithBinary creates an ASR engine using an explicit whisper-cli-compatible executable.
func NewWhisperCLIWithBinary(binary string, modelDir string) *WhisperCLI {
	if strings.TrimSpace(binary) == "" {
		binary = defaultWhisperCLIBinary
	}
	return &WhisperCLI{
		binary:   binary,
		modelDir: strings.TrimSpace(modelDir),
	}
}

// Transcribe runs whisper-cli on normalized audio and returns a structured transcript.
func (e *WhisperCLI) Transcribe(ctx context.Context, audioPath string, opts domain.TranscribeOptions) (domain.Transcript, error) {
	audioPath = strings.TrimSpace(audioPath)
	if audioPath == "" {
		return domain.Transcript{}, ErrEmptyAudioPath
	}
	if err := ctx.Err(); err != nil {
		return domain.Transcript{}, err
	}

	sourcePath, err := validateAudioPath(audioPath)
	if err != nil {
		return domain.Transcript{}, err
	}

	normalized := normalizeOptions(opts)
	if err := validateLanguage(normalized.Language); err != nil {
		return domain.Transcript{}, err
	}
	modelPath, err := e.resolveModelPath(normalized)
	if err != nil {
		return domain.Transcript{}, err
	}

	workdir, cleanup, err := prepareWorkDir(normalized.WorkDir)
	if err != nil {
		return domain.Transcript{}, err
	}
	defer cleanup()

	outputPrefix := filepath.Join(workdir, "transcript")
	if err := e.run(ctx, sourcePath, modelPath, outputPrefix, normalized); err != nil {
		return domain.Transcript{}, err
	}

	transcript, err := parseTranscriptFile(outputPrefix + ".json")
	if err != nil {
		return domain.Transcript{}, err
	}
	transcript.Engine = domain.TranscriptEngine{
		Name:         whisperEngineName,
		ModelName:    normalized.ModelName,
		ModelPath:    modelPath,
		LanguageMode: normalized.Language,
	}
	if transcript.Metadata == nil {
		transcript.Metadata = map[string]string{}
	}
	transcript.Metadata["execution"] = "cli"
	transcript.Metadata["binary"] = e.binary
	if normalized.Threads > 0 {
		transcript.Metadata["threads"] = strconv.Itoa(normalized.Threads)
	}

	return transcript, nil
}

func (e *WhisperCLI) run(ctx context.Context, audioPath string, modelPath string, outputPrefix string, opts domain.TranscribeOptions) error {
	args := []string{
		"-m", modelPath,
		"-f", audioPath,
		"-l", opts.Language,
		"-oj",
		"-of", outputPrefix,
		"-np",
	}
	if opts.Threads > 0 {
		args = append([]string{"-t", strconv.Itoa(opts.Threads)}, args...)
	}

	cmd := exec.CommandContext(ctx, e.binary, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return commandError(err, stderr.Bytes())
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (e *WhisperCLI) resolveModelPath(opts domain.TranscribeOptions) (string, error) {
	if strings.TrimSpace(opts.ModelPath) != "" {
		return validateModelPath(opts.ModelPath)
	}
	if strings.TrimSpace(opts.ModelName) == "" {
		return "", ErrMissingModel
	}

	if filepath.IsAbs(opts.ModelName) || strings.ContainsRune(opts.ModelName, filepath.Separator) {
		return validateModelPath(opts.ModelName)
	}
	if strings.TrimSpace(e.modelDir) == "" {
		return "", fmt.Errorf("%w: model directory is not configured", ErrMissingModel)
	}

	var candidates []string
	for _, name := range modelCandidateNames(opts.ModelName) {
		candidates = append(candidates, filepath.Join(e.modelDir, name))
	}
	for _, candidate := range candidates {
		path, err := validateModelPath(candidate)
		if err == nil {
			return path, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}

	return "", fmt.Errorf("%w: tried %s", ErrMissingModel, strings.Join(candidates, ", "))
}

func normalizeOptions(opts domain.TranscribeOptions) domain.TranscribeOptions {
	opts.ModelName = strings.TrimSpace(opts.ModelName)
	opts.ModelPath = strings.TrimSpace(opts.ModelPath)
	opts.Language = strings.TrimSpace(opts.Language)
	if opts.Language == "" {
		opts.Language = defaultLanguageMode
	}
	return opts
}

func validateLanguage(language string) error {
	switch language {
	case "auto", "zh", "en":
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedLang, language)
	}
}

func validateAudioPath(audioPath string) (string, error) {
	info, err := os.Stat(audioPath)
	if err != nil {
		return "", fmt.Errorf("inspect audio path %s: %w", audioPath, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: %s", ErrAudioNotFile, audioPath)
	}

	absPath, err := filepath.Abs(audioPath)
	if err != nil {
		return "", fmt.Errorf("resolve audio path %s: %w", audioPath, err)
	}
	return absPath, nil
}

func validateModelPath(modelPath string) (string, error) {
	modelPath = strings.TrimSpace(modelPath)
	if modelPath == "" {
		return "", ErrMissingModel
	}

	info, err := os.Stat(modelPath)
	if err != nil {
		return "", fmt.Errorf("inspect model path %s: %w", modelPath, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: %s", ErrMissingModel, modelPath)
	}

	absPath, err := filepath.Abs(modelPath)
	if err != nil {
		return "", fmt.Errorf("resolve model path %s: %w", modelPath, err)
	}
	return absPath, nil
}

func prepareWorkDir(workdir string) (string, func(), error) {
	workdir = strings.TrimSpace(workdir)
	if workdir == "" {
		tempDir, err := os.MkdirTemp("", "textdrain-asr-*")
		if err != nil {
			return "", func() {}, fmt.Errorf("create temporary asr workdir: %w", err)
		}
		return tempDir, func() { _ = os.RemoveAll(tempDir) }, nil
	}

	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return "", func() {}, fmt.Errorf("create asr workdir %s: %w", workdir, err)
	}
	tempDir, err := os.MkdirTemp(workdir, "asr-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("create temporary asr workdir in %s: %w", workdir, err)
	}
	return tempDir, func() { _ = os.RemoveAll(tempDir) }, nil
}

func modelCandidateNames(modelName string) []string {
	seen := map[string]struct{}{}
	candidates := make([]string, 0, 4)
	add := func(name string) {
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		candidates = append(candidates, name)
	}

	add(modelName)
	add(modelName + ".bin")
	add("ggml-" + modelName + ".bin")
	add("ggml-" + modelName + ".q5_0.bin")

	return candidates
}

func commandError(err error, stderr []byte) error {
	message := strings.TrimSpace(string(stderr))
	if message == "" {
		return fmt.Errorf("whisper-cli transcription failed: %w", err)
	}
	return fmt.Errorf("whisper-cli transcription failed: %s: %w", firstLine(message), err)
}

func firstLine(input string) string {
	line, _, _ := strings.Cut(input, "\n")
	return strings.TrimSpace(line)
}
