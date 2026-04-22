package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"textdrain/internal/app/transcription"
	"textdrain/internal/config"
	"textdrain/internal/domain"
)

func TestRootHelpIncludesCommandSurface(t *testing.T) {
	output, err := executeTestCommand(t, []string{"--help"}, testConfig(t), nil)
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
	tempDir := t.TempDir()
	transcriber := &fakeTranscriber{result: transcription.Result{
		JobID:       "job-1",
		WorkDir:     filepath.Join(tempDir, "job-1"),
		OutputPaths: []string{filepath.Join(tempDir, "out.txt")},
		Asset: domain.MediaAsset{
			MediaPath: filepath.Join(tempDir, "media.mp4"),
		},
		Audio: domain.PreparedAudio{
			Path: filepath.Join(tempDir, "audio.wav"),
		},
	}}
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
	}, cfg, transcriber)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if transcriber.request.Input != "https://example.com/video" {
		t.Fatalf("request input = %q, want URL", transcriber.request.Input)
	}
	if transcriber.request.Language != "zh" || transcriber.request.Model != "base" || !transcriber.request.KeepIntermediate {
		t.Fatalf("request = %#v, want parsed flags", transcriber.request)
	}
	if transcriber.request.OutputDir == "" {
		t.Fatal("request OutputDir is empty, want --output value")
	}
	for _, want := range []string{
		"job_id=job-1",
		"outputs=1",
		"media_path=",
		"audio_path=",
		"final_output=",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("transcribe output missing %q:\n%s", want, output)
		}
	}
}

func TestTranscribeFormatsStageError(t *testing.T) {
	cfg := testConfig(t)
	transcriber := &fakeTranscriber{
		err: &transcription.StageError{
			Stage: domain.JobStatusTranscribing,
			Err:   errors.New("whisper-cli transcription failed: model missing"),
		},
	}

	_, err := executeTestCommand(t, []string{"transcribe", "media.mp4"}, cfg, transcriber)
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if ExitCode(err) != ExitRuntime {
		t.Fatalf("ExitCode() = %d, want %d", ExitCode(err), ExitRuntime)
	}

	formatted := FormatError(err)
	for _, want := range []string{
		"stage: TRANSCRIBING",
		"type: asr",
		"reason: whisper-cli transcription failed: model missing",
		"advice: Check that whisper-cli is installed and the selected model exists.",
	} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("formatted error does not contain %q:\n%s", want, formatted)
		}
	}
}

func TestTranscribeFormatsDownloadDependencyError(t *testing.T) {
	cfg := testConfig(t)
	transcriber := &fakeTranscriber{
		err: &transcription.StageError{
			Stage: domain.JobStatusDownloading,
			Err:   errors.New("exec: \"yt-dlp\": executable file not found in $PATH"),
		},
	}

	_, err := executeTestCommand(t, []string{"transcribe", "https://example.com/video"}, cfg, transcriber)
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if ExitCode(err) != ExitDependency {
		t.Fatalf("ExitCode() = %d, want %d", ExitCode(err), ExitDependency)
	}

	formatted := FormatError(err)
	for _, want := range []string{
		"stage: DOWNLOADING",
		"type: dependency",
		"advice: Install the required external tool and ensure it is available on PATH.",
	} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("formatted error does not contain %q:\n%s", want, formatted)
		}
	}
}

func TestParameterErrorUsesUserFacingFormat(t *testing.T) {
	_, err := executeTestCommand(t, []string{"transcribe", "media.mp4", "--lang", "fr"}, testConfig(t), &fakeTranscriber{})
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}

	formatted := FormatError(err)
	for _, want := range []string{
		"stage: INPUT",
		"type: parameter",
		"reason: --lang must be one of auto, zh, or en",
		"advice: Run `textdrain <command> --help` and check the arguments.",
	} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("formatted error does not contain %q:\n%s", want, formatted)
		}
	}
}

func TestStatusWriterPrintsStage(t *testing.T) {
	var output bytes.Buffer
	writer := statusWriter{out: &output}
	if err := writer.Update(context.Background(), domain.JobStatusDownloading); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got := output.String(); got != "stage=DOWNLOADING\n" {
		t.Fatalf("status output = %q, want stage line", got)
	}
}

func TestFormatErrorDefaultsUnclassifiedError(t *testing.T) {
	formatted := FormatError(errors.New("boom"))
	for _, want := range []string{
		"stage: RUNTIME",
		"type: runtime",
		"reason: boom",
		"advice:",
	} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("formatted error does not contain %q:\n%s", want, formatted)
		}
	}
}

func TestConfigErrorUsesUserFacingFormat(t *testing.T) {
	err := NewConfigError("parse config: %s", "bad value")
	formatted := FormatError(err)
	for _, want := range []string{
		"stage: CONFIG",
		"type: config",
		"reason: parse config: bad value",
	} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("formatted error does not contain %q:\n%s", want, formatted)
		}
	}
	if ExitCode(err) != ExitConfig {
		t.Fatalf("ExitCode() = %d, want %d", ExitCode(err), ExitConfig)
	}
}

func TestPipelineErrorClassifiesStages(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "download",
			err:  &transcription.StageError{Stage: domain.JobStatusDownloading, Err: errors.New("download failed")},
			want: "type: download",
		},
		{
			name: "media",
			err:  &transcription.StageError{Stage: domain.JobStatusExtractingAudio, Err: errors.New("ffmpeg failed")},
			want: "type: media",
		},
		{
			name: "export",
			err:  &transcription.StageError{Stage: domain.JobStatusExporting, Err: errors.New("write failed")},
			want: "type: export",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			formatted := FormatError(NewPipelineError(tc.err))
			if !strings.Contains(formatted, tc.want) {
				t.Fatalf("formatted error does not contain %q:\n%s", tc.want, formatted)
			}
		})
	}
}

func TestTranscribeOutputContainsResultSummary(t *testing.T) {
	output, err := executeTestCommand(t, []string{"transcribe", "media.mp4"}, testConfig(t), &fakeTranscriber{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, want := range []string{"job_id=job", "work_dir=work", "outputs=1", "output=output.txt", "final_output=output.txt"} {
		if !strings.Contains(output, want) {
			t.Fatalf("transcribe output does not contain %q:\n%s", want, output)
		}
	}
}

func TestTranscribeParsesFlagsOutputSummary(t *testing.T) {
	output, err := executeTestCommand(t, []string{"transcribe", "media.mp4"}, testConfig(t), &fakeTranscriber{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(output, "job_id=job") || !strings.Contains(output, "outputs=1") {
		t.Fatalf("transcribe output missing result summary:\n%s", output)
	}
}

func TestTranscribeResolvesLocalMediaAsset(t *testing.T) {
	cfg := testConfig(t)
	mediaPath := filepath.Join(t.TempDir(), "Local Clip.mp4")
	if err := os.WriteFile(mediaPath, []byte("media"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	transcriber := &fakeTranscriber{result: transcription.Result{
		JobID:       "local-file-local-clip",
		WorkDir:     filepath.Join(t.TempDir(), "local-file-local-clip"),
		OutputPaths: []string{filepath.Join(t.TempDir(), "Local-Clip.txt")},
	}}

	output, err := executeTestCommand(t, []string{
		"transcribe",
		mediaPath,
		"--lang",
		"en",
		"--output",
		filepath.Join(t.TempDir(), "out"),
	}, cfg, transcriber)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if transcriber.request.Input != mediaPath || transcriber.request.Language != "en" {
		t.Fatalf("request = %#v, want local media request", transcriber.request)
	}
	if !reflect.DeepEqual(transcriber.request.Formats, cfg.OutputFormats) {
		t.Fatalf("request formats = %#v, want config formats", transcriber.request.Formats)
	}
	if !strings.Contains(output, "job_id=local-file-local-clip") {
		t.Fatalf("transcribe output missing job id:\n%s", output)
	}
}

func TestTranscribeRejectsUnsupportedLanguage(t *testing.T) {
	_, err := executeTestCommand(t, []string{"transcribe", "media.mp4", "--lang", "fr"}, testConfig(t), &fakeTranscriber{})
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if ExitCode(err) != ExitParameter {
		t.Fatalf("ExitCode() = %d, want %d", ExitCode(err), ExitParameter)
	}
}

func TestTranscribeRejectsConflictingCookieFlags(t *testing.T) {
	_, err := executeTestCommand(t, []string{
		"transcribe",
		"https://example.com/video",
		"--cookies-from-browser",
		"safari",
		"--cookies",
		filepath.Join(t.TempDir(), "cookies.txt"),
	}, testConfig(t), nil)
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if ExitCode(err) != ExitParameter {
		t.Fatalf("ExitCode() = %d, want %d", ExitCode(err), ExitParameter)
	}
	formatted := FormatError(err)
	if !strings.Contains(formatted, "--cookies-from-browser and --cookies cannot be used together") {
		t.Fatalf("formatted error missing cookie flag conflict:\n%s", formatted)
	}
}

func TestTranscribeFormatsYTDLPBotCookieAdvice(t *testing.T) {
	cfg := testConfig(t)
	transcriber := &fakeTranscriber{
		err: &transcription.StageError{
			Stage: domain.JobStatusResolving,
			Err:   errors.New("yt-dlp read metadata failed: ERROR: [youtube] CIUtEnnjA2U: Sign in to confirm you're not a bot. Use --cookies-from-browser or --cookies for the authentication"),
		},
	}

	_, err := executeTestCommand(t, []string{"transcribe", "https://www.youtube.com/watch?v=CIUtEnnjA2U"}, cfg, transcriber)
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}

	formatted := FormatError(err)
	for _, want := range []string{
		"stage: RESOLVING",
		"type: download",
		"advice: Pass YouTube login cookies with --cookies-from-browser <browser>",
	} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("formatted error does not contain %q:\n%s", want, formatted)
		}
	}
}

func TestDoctorRejectsArguments(t *testing.T) {
	_, err := executeTestCommand(t, []string{"doctor", "extra"}, testConfig(t), nil)
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if ExitCode(err) != ExitParameter {
		t.Fatalf("ExitCode() = %d, want %d", ExitCode(err), ExitParameter)
	}
}

func TestDoctorReportsMissingDependencies(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	output, err := executeTestCommand(t, []string{"doctor"}, testConfig(t), nil)
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

	output, err := executeTestCommand(t, []string{"doctor"}, cfg, nil)
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

func TestModelsListShowsLocalModelDetails(t *testing.T) {
	cfg := testConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.ModelDir, "ggml-base.bin"), []byte("base-model"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ModelDir, "small.gguf"), []byte("small-model"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ModelDir, "notes.txt"), []byte("not a model"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	output, err := executeTestCommand(t, []string{"models", "--list"}, cfg, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	for _, want := range []string{
		"default_model=small",
		"models=2",
		"model name=ggml-base.bin path=" + filepath.Join(cfg.ModelDir, "ggml-base.bin") + " size_bytes=10 default=false",
		"model name=small.gguf path=" + filepath.Join(cfg.ModelDir, "small.gguf") + " size_bytes=11 default=true",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("models output does not contain %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "notes.txt") {
		t.Fatalf("models output contains non-model file:\n%s", output)
	}
}

func TestModelsListReportsZeroWhenModelDirDoesNotExist(t *testing.T) {
	cfg := testConfig(t)
	cfg.ModelDir = filepath.Join(t.TempDir(), "missing")

	output, err := executeTestCommand(t, []string{"models", "--list"}, cfg, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	for _, want := range []string{"model_dir=" + cfg.ModelDir, "default_model=small", "models=0"} {
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

func executeTestCommand(t *testing.T, args []string, cfg config.Config, transcriber Transcriber) (string, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewRootCommand(context.Background(), RootOptions{
		Paths:       testPaths(t),
		Config:      cfg,
		UI:          NewUI(&stdout, &stderr),
		Transcriber: transcriber,
	})
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(context.Background())
	return stdout.String() + stderr.String(), err
}

type fakeTranscriber struct {
	request transcription.Request
	result  transcription.Result
	err     error
}

func (t *fakeTranscriber) Run(_ context.Context, req transcription.Request) (transcription.Result, error) {
	t.request = req
	if t.err != nil {
		return transcription.Result{}, t.err
	}
	if len(t.result.OutputPaths) == 0 {
		t.result = transcription.Result{
			JobID:       "job",
			WorkDir:     "work",
			OutputPaths: []string{"output.txt"},
			Transcript:  domain.Transcript{Text: "text"},
		}
	}
	return t.result, nil
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
