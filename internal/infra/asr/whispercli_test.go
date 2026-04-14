package asr

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

func TestWhisperCLITranscribeBuildsTranscriptWithDefaults(t *testing.T) {
	binary := writeFakeWhisperCLI(t, fakeWhisperConfig{})
	modelDir := t.TempDir()
	modelPath := filepath.Join(modelDir, "ggml-small.bin")
	if err := os.WriteFile(modelPath, []byte("model"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	audioPath := writeAudioFile(t, "clip.wav")
	workdir := t.TempDir()

	transcript, err := NewWhisperCLIWithBinary(binary, modelDir).Transcribe(context.Background(), audioPath, domain.TranscribeOptions{
		ModelName: "small",
		WorkDir:   workdir,
	})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}

	if transcript.Language != "zh" {
		t.Fatalf("Language = %q, want zh", transcript.Language)
	}
	if transcript.Text != "hello\nworld" {
		t.Fatalf("Text = %q, want joined segment text", transcript.Text)
	}
	if len(transcript.Segments) != 2 {
		t.Fatalf("Segments len = %d, want 2", len(transcript.Segments))
	}
	if transcript.Segments[0].StartMs != 0 || transcript.Segments[0].EndMs != 1240 || transcript.Segments[0].Text != "hello" {
		t.Fatalf("Segments[0] = %#v, want parsed offsets and text", transcript.Segments[0])
	}
	if transcript.Engine.Name != "whisper.cpp" {
		t.Fatalf("Engine.Name = %q, want whisper.cpp", transcript.Engine.Name)
	}
	if transcript.Engine.ModelName != "small" {
		t.Fatalf("Engine.ModelName = %q, want small", transcript.Engine.ModelName)
	}
	if transcript.Engine.ModelPath != modelPath {
		t.Fatalf("Engine.ModelPath = %q, want %q", transcript.Engine.ModelPath, modelPath)
	}
	if transcript.Engine.LanguageMode != "auto" {
		t.Fatalf("Engine.LanguageMode = %q, want auto", transcript.Engine.LanguageMode)
	}
	if transcript.Metadata["execution"] != "cli" {
		t.Fatalf("Metadata[execution] = %q, want cli", transcript.Metadata["execution"])
	}

	args := readArgs(t, workdir)
	assertContainsArgSequence(t, args, "-m", modelPath)
	assertContainsArgSequence(t, args, "-f", audioPath)
	assertContainsArgSequence(t, args, "-l", "auto")
	assertContainsArgSequence(t, args, "-oj")
	assertContainsArgSequence(t, args, "-np")
}

func TestWhisperCLITranscribeAppliesLanguageModelPathAndThreads(t *testing.T) {
	binary := writeFakeWhisperCLI(t, fakeWhisperConfig{})
	modelPath := writeModelFile(t, "custom.bin")
	audioPath := writeAudioFile(t, "clip.wav")
	workdir := t.TempDir()

	transcript, err := NewWhisperCLIWithBinary(binary, "").Transcribe(context.Background(), audioPath, domain.TranscribeOptions{
		ModelName: "custom",
		ModelPath: modelPath,
		Language:  "en",
		WorkDir:   workdir,
		Threads:   6,
	})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}

	if transcript.Engine.ModelPath != modelPath {
		t.Fatalf("Engine.ModelPath = %q, want %q", transcript.Engine.ModelPath, modelPath)
	}
	if transcript.Engine.LanguageMode != "en" {
		t.Fatalf("Engine.LanguageMode = %q, want en", transcript.Engine.LanguageMode)
	}
	if transcript.Metadata["threads"] != "6" {
		t.Fatalf("Metadata[threads] = %q, want 6", transcript.Metadata["threads"])
	}

	args := readArgs(t, workdir)
	assertContainsArgSequence(t, args, "-t", "6")
	assertContainsArgSequence(t, args, "-m", modelPath)
	assertContainsArgSequence(t, args, "-l", "en")
}

func TestWhisperCLITranscribeRejectsInvalidInput(t *testing.T) {
	_, err := NewWhisperCLIWithBinary("unused", "").Transcribe(context.Background(), "", domain.TranscribeOptions{})
	if !errors.Is(err, ErrEmptyAudioPath) {
		t.Fatalf("Transcribe() error = %v, want ErrEmptyAudioPath", err)
	}

	_, err = NewWhisperCLIWithBinary("unused", "").Transcribe(context.Background(), t.TempDir(), domain.TranscribeOptions{})
	if !errors.Is(err, ErrAudioNotFile) {
		t.Fatalf("Transcribe() error = %v, want ErrAudioNotFile", err)
	}

	_, err = NewWhisperCLIWithBinary("unused", "").Transcribe(context.Background(), writeAudioFile(t, "clip.wav"), domain.TranscribeOptions{
		ModelPath: writeModelFile(t, "model.bin"),
		Language:  "fr",
	})
	if !errors.Is(err, ErrUnsupportedLang) {
		t.Fatalf("Transcribe() error = %v, want ErrUnsupportedLang", err)
	}
}

func TestWhisperCLITranscribeReportsCommandError(t *testing.T) {
	binary := writeFakeWhisperCLI(t, fakeWhisperConfig{fail: true})
	modelPath := writeModelFile(t, "model.bin")
	audioPath := writeAudioFile(t, "clip.wav")

	_, err := NewWhisperCLIWithBinary(binary, "").Transcribe(context.Background(), audioPath, domain.TranscribeOptions{
		ModelPath: modelPath,
	})
	if err == nil {
		t.Fatal("Transcribe() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "whisper-cli transcription failed: bad audio") {
		t.Fatalf("Transcribe() error = %v, want whisper stderr context", err)
	}
}

func TestParseTranscriptJSONFallsBackToParamsLanguage(t *testing.T) {
	transcript, err := parseTranscriptJSON([]byte(`{
		"params": {"language": "en"},
		"transcription": [
			{"offsets": {"from": 10, "to": 20}, "text": " text "}
		]
	}`))
	if err != nil {
		t.Fatalf("parseTranscriptJSON() error = %v", err)
	}

	if transcript.Language != "en" {
		t.Fatalf("Language = %q, want en", transcript.Language)
	}
	if transcript.Text != "text" {
		t.Fatalf("Text = %q, want trimmed text", transcript.Text)
	}
	if transcript.Segments[0].StartMs != 10 || transcript.Segments[0].EndMs != 20 {
		t.Fatalf("Segments[0] = %#v, want offsets", transcript.Segments[0])
	}
}

type fakeWhisperConfig struct {
	fail bool
}

func writeFakeWhisperCLI(t *testing.T, cfg fakeWhisperConfig) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script test helper is not supported on windows")
	}

	dir := t.TempDir()
	binary := filepath.Join(dir, "whisper-cli")
	script := `#!/bin/sh
set -eu
audio=""
model=""
output=""
previous=""
for arg in "$@"; do
  case "$previous" in
    -f)
      audio="$arg"
      ;;
    -m)
      model="$arg"
      ;;
    -of)
      output="$arg"
      ;;
  esac
  previous="$arg"
done

if [ "` + boolString(cfg.fail) + `" = "true" ]; then
  echo "bad audio" >&2
  exit 2
fi

if [ ! -f "$audio" ]; then
  echo "missing audio" >&2
  exit 3
fi
if [ ! -f "$model" ]; then
  echo "missing model" >&2
  exit 4
fi

mkdir -p "$(dirname "$output")"
printf '%s\n' "$@" > "$(dirname "$(dirname "$output")")/whisper.args"
cat > "$output.json" <<'JSON'
{
  "params": {"model": "ggml-small.bin", "language": "auto"},
  "result": {"language": "zh"},
  "transcription": [
    {"timestamps": {"from": "00:00:00,000", "to": "00:00:01,240"}, "offsets": {"from": 0, "to": 1240}, "text": " hello "},
    {"timestamps": {"from": "00:00:01,240", "to": "00:00:02,000"}, "offsets": {"from": 1240, "to": 2000}, "text": "world"}
  ]
}
JSON
`
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return binary
}

func writeAudioFile(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("audio"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func writeModelFile(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("model"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func readArgs(t *testing.T, workdir string) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(workdir, "whisper.args"))
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

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
