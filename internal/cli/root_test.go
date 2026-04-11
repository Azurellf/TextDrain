package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"textdrain/internal/config"
)

func TestRootHelpIncludesCommandSurface(t *testing.T) {
	output, err := executeTestCommand(t, []string{"--help"}, testConfig(t))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	for _, want := range []string{"transcribe", "doctor", "models"} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output does not contain %q:\n%s", want, output)
		}
	}
}

func TestTranscribeParsesFlags(t *testing.T) {
	cfg := testConfig(t)
	output, err := executeTestCommand(t, []string{
		"transcribe",
		"https://example.com/video",
		"--lang",
		"zh",
		"--model",
		"base",
		"--output",
		filepath.Join(t.TempDir(), "out"),
		"--keep-intermediate",
	}, cfg)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	for _, want := range []string{
		"input=https://example.com/video",
		"language=zh",
		"model=base",
		"keep_intermediate=true",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("transcribe output does not contain %q:\n%s", want, output)
		}
	}
}

func TestTranscribeResolvesLocalMediaAsset(t *testing.T) {
	cfg := testConfig(t)
	mediaPath := filepath.Join(t.TempDir(), "Local Clip.mp4")
	if err := os.WriteFile(mediaPath, []byte("media"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	output, err := executeTestCommand(t, []string{
		"transcribe",
		mediaPath,
		"--lang",
		"en",
		"--output",
		filepath.Join(t.TempDir(), "out"),
	}, cfg)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	for _, want := range []string{
		"source_type=local_file",
		"title=Local Clip",
		"site=local",
		"media_path=" + mediaPath,
		"language=en",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("transcribe output does not contain %q:\n%s", want, output)
		}
	}
}

func TestTranscribeRejectsUnsupportedLanguage(t *testing.T) {
	_, err := executeTestCommand(t, []string{"transcribe", "media.mp4", "--lang", "fr"}, testConfig(t))
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if ExitCode(err) != ExitParameter {
		t.Fatalf("ExitCode() = %d, want %d", ExitCode(err), ExitParameter)
	}
}

func TestDoctorRejectsArguments(t *testing.T) {
	_, err := executeTestCommand(t, []string{"doctor", "extra"}, testConfig(t))
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if ExitCode(err) != ExitParameter {
		t.Fatalf("ExitCode() = %d, want %d", ExitCode(err), ExitParameter)
	}
}

func TestDoctorReportsMissingDependencies(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	output, err := executeTestCommand(t, []string{"doctor"}, testConfig(t))
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if ExitCode(err) != ExitDependency {
		t.Fatalf("ExitCode() = %d, want %d", ExitCode(err), ExitDependency)
	}

	for _, want := range []string{
		"yt-dlp=missing",
		"ffmpeg=missing",
		"whisper-cli=missing",
		"model_file=missing",
		"Status: failed",
		"advice=",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor output does not contain %q:\n%s", want, output)
		}
	}
}

func TestDoctorReportsHealthyEnvironment(t *testing.T) {
	binDir := t.TempDir()
	writeFakeExecutable(t, filepath.Join(binDir, "yt-dlp"), "yt-dlp 2026.01.01")
	writeFakeExecutable(t, filepath.Join(binDir, "ffmpeg"), "ffmpeg version 7.0")
	writeFakeExecutable(t, filepath.Join(binDir, "whisper-cli"), "whisper-cli 1.7.0")
	t.Setenv("PATH", binDir)

	cfg := testConfig(t)
	modelPath := filepath.Join(cfg.ModelDir, "ggml-small.bin")
	if err := os.WriteFile(modelPath, []byte("model"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	output, err := executeTestCommand(t, []string{"doctor"}, cfg)
	if err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, output)
	}

	for _, want := range []string{
		"yt-dlp=ok",
		"version=yt-dlp 2026.01.01",
		"ffmpeg=ok",
		"whisper-cli=ok",
		"model_file=ok",
		"Status: ok",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor output does not contain %q:\n%s", want, output)
		}
	}
}

func TestModelsListShowsFilesInModelDir(t *testing.T) {
	cfg := testConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.ModelDir, "ggml-base.bin"), []byte("model"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ModelDir, "ggml-small.bin"), []byte("model"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	output, err := executeTestCommand(t, []string{"models", "--list"}, cfg)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	for _, want := range []string{"models=2", "ggml-base.bin", "ggml-small.bin"} {
		if !strings.Contains(output, want) {
			t.Fatalf("models output does not contain %q:\n%s", want, output)
		}
	}
}

func TestExitCodeDefaultsToRuntimeForUnclassifiedErrors(t *testing.T) {
	if got := ExitCode(errors.New("boom")); got != ExitRuntime {
		t.Fatalf("ExitCode() = %d, want %d", got, ExitRuntime)
	}
}

func executeTestCommand(t *testing.T, args []string, cfg config.Config) (string, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewRootCommand(context.Background(), RootOptions{
		Paths:  testPaths(t),
		Config: cfg,
		UI:     NewUI(&stdout, &stderr),
	})
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(context.Background())
	return stdout.String() + stderr.String(), err
}

func testConfig(t *testing.T) config.Config {
	t.Helper()

	paths := testPaths(t)
	if err := os.MkdirAll(paths.ModelsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	return config.Default(paths)
}

func testPaths(t *testing.T) config.Paths {
	t.Helper()

	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	cacheDir := filepath.Join(root, "cache")

	return config.Paths{
		ConfigDir:  configDir,
		ConfigFile: filepath.Join(configDir, "config.toml"),
		CacheDir:   cacheDir,
		JobsDir:    filepath.Join(cacheDir, "jobs"),
		ModelsDir:  filepath.Join(cacheDir, "models"),
	}
}

func writeFakeExecutable(t *testing.T, path string, version string) {
	t.Helper()

	content := "#!/bin/sh\nprintf '%s\\n' '" + version + "'\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
