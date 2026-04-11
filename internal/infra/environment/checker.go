package environment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"textdrain/internal/config"
)

const versionTimeout = 3 * time.Second

type Report struct {
	Tools ToolChecks
	Model ModelCheck
	Paths PathChecks
}

type ToolChecks struct {
	YTDLP      ToolCheck
	FFmpeg     ToolCheck
	WhisperCLI ToolCheck
}

type ToolCheck struct {
	Name       string
	Found      bool
	Executable bool
	Path       string
	Version    string
	Error      string
	Advice     string
}

type ModelCheck struct {
	Name       string
	Dir        string
	DirExists  bool
	Path       string
	Found      bool
	Candidates []string
	Advice     string
	Error      string
}

type PathChecks struct {
	ConfigFile string
	CacheDir   string
	JobsDir    string
	ModelDir   string
}

func Check(ctx context.Context, paths config.Paths, cfg config.Config) Report {
	return Report{
		Tools: ToolChecks{
			YTDLP:      checkTool(ctx, "yt-dlp", []string{"--version"}, "Install yt-dlp: https://github.com/yt-dlp/yt-dlp#installation"),
			FFmpeg:     checkTool(ctx, "ffmpeg", []string{"-version"}, "Install ffmpeg: https://ffmpeg.org/download.html"),
			WhisperCLI: checkTool(ctx, "whisper-cli", []string{"--version"}, "Install whisper.cpp and make whisper-cli available on PATH."),
		},
		Model: checkModel(cfg.ModelDir, cfg.Model),
		Paths: PathChecks{
			ConfigFile: paths.ConfigFile,
			CacheDir:   paths.CacheDir,
			JobsDir:    cfg.JobsDir,
			ModelDir:   cfg.ModelDir,
		},
	}
}

func (r Report) Healthy() bool {
	return r.Tools.YTDLP.Found &&
		r.Tools.YTDLP.Executable &&
		r.Tools.FFmpeg.Found &&
		r.Tools.FFmpeg.Executable &&
		r.Tools.WhisperCLI.Found &&
		r.Tools.WhisperCLI.Executable &&
		r.Model.Found
}

func checkTool(ctx context.Context, name string, versionArgs []string, advice string) ToolCheck {
	check := ToolCheck{
		Name:   name,
		Advice: advice,
	}

	path, err := exec.LookPath(name)
	if err != nil {
		check.Error = "not found on PATH"
		return check
	}
	check.Found = true
	check.Path = path

	info, err := os.Stat(path)
	if err != nil {
		check.Error = err.Error()
		return check
	}
	if info.IsDir() {
		check.Error = "resolved path is a directory"
		return check
	}
	check.Executable = info.Mode().Perm()&0o111 != 0
	if !check.Executable {
		check.Error = "resolved path is not executable"
		return check
	}

	version, err := commandVersion(ctx, path, versionArgs)
	if err != nil {
		check.Version = version
		check.Error = err.Error()
		return check
	}
	check.Version = version

	return check
}

func commandVersion(ctx context.Context, path string, args []string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, versionTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, path, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		output = strings.TrimSpace(stderr.String())
	}
	if runCtx.Err() != nil {
		return "", fmt.Errorf("version command timed out")
	}
	if err != nil {
		if output != "" {
			return firstLine(output), fmt.Errorf("version command failed: %s", firstLine(output))
		}
		return "", fmt.Errorf("version command failed: %w", err)
	}
	if output == "" {
		return "unknown", nil
	}

	return firstLine(output), nil
}

func checkModel(modelDir string, modelName string) ModelCheck {
	check := ModelCheck{
		Name:   modelName,
		Dir:    filepath.Clean(modelDir),
		Advice: "Download a whisper.cpp ggml model into the model directory, or set model_dir/model to an existing model.",
	}

	if modelName == "" {
		check.Error = "model name is empty"
		return check
	}
	if modelDir == "" {
		check.Error = "model directory is not configured"
		return check
	}

	info, err := os.Stat(modelDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			check.Error = "model directory does not exist"
			check.Candidates = modelCandidates(modelDir, modelName)
			return check
		}
		check.Error = err.Error()
		return check
	}
	if !info.IsDir() {
		check.Error = "model directory path is not a directory"
		return check
	}
	check.DirExists = true

	for _, candidate := range modelCandidates(modelDir, modelName) {
		check.Candidates = append(check.Candidates, candidate)
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			check.Path = candidate
			check.Found = true
			return check
		}
	}

	check.Error = "model file was not found"
	return check
}

func modelCandidates(modelDir string, modelName string) []string {
	if filepath.IsAbs(modelName) || strings.ContainsRune(modelName, filepath.Separator) {
		return []string{filepath.Clean(modelName)}
	}

	seen := map[string]struct{}{}
	candidates := make([]string, 0, 4)
	add := func(name string) {
		path := filepath.Join(modelDir, name)
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		candidates = append(candidates, path)
	}

	add(modelName)
	add(modelName + ".bin")
	add("ggml-" + modelName + ".bin")
	add("ggml-" + modelName + ".q5_0.bin")

	return candidates
}

func firstLine(output string) string {
	line, _, _ := strings.Cut(output, "\n")
	return strings.TrimSpace(line)
}
