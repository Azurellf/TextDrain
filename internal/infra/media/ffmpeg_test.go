package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"textdrain/internal/domain"
)

func TestFFmpegPrepareNormalizesAudioWithDefaults(t *testing.T) {
	binary := writeFakeFFmpeg(t, fakeFFmpegConfig{})
	mediaPath := writeMediaFile(t, "Bad : Title? Episode 1.mp4")
	workdir := t.TempDir()

	audio, err := NewFFmpegWithBinary(binary).Prepare(context.Background(), mediaPath, workdir, domain.AudioOptions{})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	if audio.SourcePath == "" || !filepath.IsAbs(audio.SourcePath) {
		t.Fatalf("SourcePath = %q, want absolute path", audio.SourcePath)
	}
	if audio.Path == "" || !filepath.IsAbs(audio.Path) {
		t.Fatalf("Path = %q, want absolute path", audio.Path)
	}
	if filepath.Base(audio.Path) != "Bad-Title-Episode-1.wav" {
		t.Fatalf("Path basename = %q, want sanitized wav name", filepath.Base(audio.Path))
	}
	if audio.SampleRateHz != 16000 {
		t.Fatalf("SampleRateHz = %d, want 16000", audio.SampleRateHz)
	}
	if audio.Channels != 1 {
		t.Fatalf("Channels = %d, want 1", audio.Channels)
	}
	if audio.Codec != "pcm_s16le" {
		t.Fatalf("Codec = %q, want pcm_s16le", audio.Codec)
	}
	if data, err := os.ReadFile(audio.Path); err != nil || string(data) != "wav" {
		t.Fatalf("prepared audio content = %q, %v; want wav", data, err)
	}

	args := readArgs(t, workdir)
	assertContainsArgSequence(t, args, "-i", audio.SourcePath)
	assertContainsArgSequence(t, args, "-vn")
	assertContainsArgSequence(t, args, "-ac", "1")
	assertContainsArgSequence(t, args, "-ar", "16000")
	assertContainsArgSequence(t, args, "-acodec", "pcm_s16le")
	assertNotContainsArg(t, args, "-af")
}

func TestFFmpegPrepareAppliesCustomOptions(t *testing.T) {
	binary := writeFakeFFmpeg(t, fakeFFmpegConfig{})
	mediaPath := writeMediaFile(t, "clip.mov")
	workdir := t.TempDir()

	audio, err := NewFFmpegWithBinary(binary).Prepare(context.Background(), mediaPath, workdir, domain.AudioOptions{
		SampleRateHz:      8000,
		Channels:          2,
		Codec:             "pcm_s24le",
		LoudnessNormalize: true,
		TrimSilence:       true,
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	if audio.SampleRateHz != 8000 {
		t.Fatalf("SampleRateHz = %d, want 8000", audio.SampleRateHz)
	}
	if audio.Channels != 2 {
		t.Fatalf("Channels = %d, want 2", audio.Channels)
	}
	if audio.Codec != "pcm_s24le" {
		t.Fatalf("Codec = %q, want pcm_s24le", audio.Codec)
	}

	args := readArgs(t, workdir)
	assertContainsArgSequence(t, args, "-ac", "2")
	assertContainsArgSequence(t, args, "-ar", "8000")
	assertContainsArgSequence(t, args, "-acodec", "pcm_s24le")
	assertContainsArgSequence(t, args, "-af", "loudnorm,silenceremove=start_periods=1:start_duration=0.1:start_threshold=-50dB")
}

func TestFFmpegPrepareUsesNextAvailableOutputPath(t *testing.T) {
	binary := writeFakeFFmpeg(t, fakeFFmpegConfig{})
	mediaPath := writeMediaFile(t, "clip.mp4")
	workdir := t.TempDir()
	existingPath := filepath.Join(workdir, "clip.wav")
	if err := os.WriteFile(existingPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	audio, err := NewFFmpegWithBinary(binary).Prepare(context.Background(), mediaPath, workdir, domain.AudioOptions{})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	if filepath.Base(audio.Path) != "clip-1.wav" {
		t.Fatalf("Path basename = %q, want clip-1.wav", filepath.Base(audio.Path))
	}
}

func TestFFmpegPrepareRejectsInvalidInput(t *testing.T) {
	_, err := NewFFmpegWithBinary("unused").Prepare(context.Background(), "", t.TempDir(), domain.AudioOptions{})
	if !errors.Is(err, ErrEmptyMediaPath) {
		t.Fatalf("Prepare() error = %v, want ErrEmptyMediaPath", err)
	}

	_, err = NewFFmpegWithBinary("unused").Prepare(context.Background(), t.TempDir(), t.TempDir(), domain.AudioOptions{})
	if !errors.Is(err, ErrMediaNotFile) {
		t.Fatalf("Prepare() error = %v, want ErrMediaNotFile", err)
	}
}

func TestFFmpegPrepareReportsCommandError(t *testing.T) {
	binary := writeFakeFFmpeg(t, fakeFFmpegConfig{fail: true})
	mediaPath := writeMediaFile(t, "broken.mp4")

	_, err := NewFFmpegWithBinary(binary).Prepare(context.Background(), mediaPath, t.TempDir(), domain.AudioOptions{})
	if err == nil {
		t.Fatal("Prepare() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "ffmpeg audio preparation failed: invalid media") {
		t.Fatalf("Prepare() error = %v, want ffmpeg stderr context", err)
	}
}

type fakeFFmpegConfig struct {
	fail bool
}

func writeFakeFFmpeg(t *testing.T, cfg fakeFFmpegConfig) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script test helper is not supported on windows")
	}

	dir := t.TempDir()
	binary := filepath.Join(dir, "ffmpeg")
	script := `#!/bin/sh
set -eu
output=""
previous=""
input=""
for arg in "$@"; do
  case "$previous" in
    -i)
      input="$arg"
      ;;
  esac
  previous="$arg"
  output="$arg"
done

if [ "` + boolString(cfg.fail) + `" = "true" ]; then
  echo "invalid media" >&2
  exit 2
fi

if [ ! -f "$input" ]; then
  echo "missing input" >&2
  exit 3
fi

mkdir -p "$(dirname "$output")"
printf '%s\n' "$@" > "$(dirname "$output")/ffmpeg.args"
printf 'wav' > "$output"
`
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return binary
}

func writeMediaFile(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("media"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func readArgs(t *testing.T, workdir string) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(workdir, "ffmpeg.args"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

func assertContainsArgSequence(t *testing.T, args []string, want ...string) {
	t.Helper()

	for i := 0; i <= len(args)-len(want); i++ {
		matched := true
		for j := range want {
			if args[i+j] != want[j] {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	t.Fatalf("args = %#v, want sequence %#v", args, want)
}

func assertNotContainsArg(t *testing.T, args []string, want string) {
	t.Helper()

	for _, arg := range args {
		if arg == want {
			t.Fatalf("args = %#v, did not expect %q", args, want)
		}
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
