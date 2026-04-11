package downloader

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"textdrain/internal/domain"
)

func TestYTDLPFetchDownloadsBestAudioAndReturnsMetadata(t *testing.T) {
	binary := writeFakeYTDLP(t, fakeYTDLPConfig{
		metadataJSON: `{"id":"abc123","title":"Bad / Title: Episode 1","extractor_key":"YouTube","webpage_url":"https://example.com/watch?v=abc123","original_url":"https://example.com/watch?v=abc123","duration":12.5}`,
	})
	asset := domain.MediaAsset{
		SourceType: domain.SourceTypeURL,
		RawInput:   "https://example.com/watch?v=abc123",
		Title:      "watch",
		Site:       "example.com",
		WorkDir:    filepath.Join(t.TempDir(), "job"),
		Metadata: map[string]string{
			"url": "https://example.com/watch?v=abc123",
		},
	}

	result, err := NewYTDLPWithBinary(binary).Fetch(context.Background(), asset, asset.WorkDir)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if result.Title != "Bad / Title: Episode 1" {
		t.Fatalf("Title = %q, want metadata title", result.Title)
	}
	if result.Site != "YouTube" {
		t.Fatalf("Site = %q, want YouTube", result.Site)
	}
	if result.OriginalURL != asset.RawInput {
		t.Fatalf("OriginalURL = %q, want %q", result.OriginalURL, asset.RawInput)
	}
	if result.Duration != 12500*time.Millisecond {
		t.Fatalf("Duration = %s, want 12.5s", result.Duration)
	}
	if result.MediaPath == "" || !filepath.IsAbs(result.MediaPath) {
		t.Fatalf("MediaPath = %q, want absolute path", result.MediaPath)
	}
	if filepath.Base(result.MediaPath) != "Bad-Title-Episode-1.m4a" {
		t.Fatalf("MediaPath basename = %q, want sanitized m4a name", filepath.Base(result.MediaPath))
	}
	if result.Asset.MediaPath != result.MediaPath {
		t.Fatalf("Asset.MediaPath = %q, want result media path", result.Asset.MediaPath)
	}
	if result.Metadata["duration"] != "12.5" {
		t.Fatalf("metadata duration = %q, want 12.5", result.Metadata["duration"])
	}
}

func TestYTDLPFetchFallsBackToBestMedia(t *testing.T) {
	binary := writeFakeYTDLP(t, fakeYTDLPConfig{
		metadataJSON:      `{"title":"Fallback Clip","extractor":"Generic","duration":3}`,
		failBestAudio:     true,
		fallbackExtension: "mp4",
	})
	asset := domain.MediaAsset{
		SourceType: domain.SourceTypeURL,
		RawInput:   "https://example.com/fallback",
		WorkDir:    t.TempDir(),
	}

	result, err := NewYTDLPWithBinary(binary).Fetch(context.Background(), asset, asset.WorkDir)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if filepath.Ext(result.MediaPath) != ".mp4" {
		t.Fatalf("MediaPath = %q, want mp4 fallback", result.MediaPath)
	}
}

func TestYTDLPFetchRejectsLocalAsset(t *testing.T) {
	_, err := NewYTDLPWithBinary("unused").Fetch(context.Background(), domain.MediaAsset{
		SourceType: domain.SourceTypeLocalFile,
		RawInput:   "local.mp4",
	}, t.TempDir())
	if err == nil {
		t.Fatal("Fetch() error = nil, want error")
	}
	if !errors.Is(err, ErrNonURLAsset) {
		t.Fatalf("Fetch() error = %v, want ErrNonURLAsset", err)
	}
}

func TestYTDLPMetadataReportsParseError(t *testing.T) {
	binary := writeFakeYTDLP(t, fakeYTDLPConfig{metadataJSON: `{bad json`})

	_, err := NewYTDLPWithBinary(binary).Metadata(context.Background(), "https://example.com/video")
	if err == nil {
		t.Fatal("Metadata() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "parse yt-dlp metadata") {
		t.Fatalf("Metadata() error = %v, want parse error", err)
	}
}

type fakeYTDLPConfig struct {
	metadataJSON      string
	failBestAudio     bool
	fallbackExtension string
}

func writeFakeYTDLP(t *testing.T, cfg fakeYTDLPConfig) string {
	t.Helper()

	dir := t.TempDir()
	binary := filepath.Join(dir, "yt-dlp")
	extension := cfg.fallbackExtension
	if extension == "" {
		extension = "m4a"
	}

	script := `#!/bin/sh
set -eu
if [ "$1" = "--dump-single-json" ]; then
  printf '%s\n' '` + shellSingleQuote(cfg.metadataJSON) + `'
  exit 0
fi

format=""
output=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -f)
      shift
      format="$1"
      ;;
    -o)
      shift
      output="$1"
      ;;
  esac
  shift || true
done

if [ "$format" = "bestaudio/best" ] && [ "` + boolString(cfg.failBestAudio) + `" = "true" ]; then
  echo "no audio format" >&2
  exit 2
fi

case "$format" in
  best)
    path=$(printf '%s' "$output" | sed 's/%(ext)s/` + extension + `/g')
    ;;
  *)
    path=$(printf '%s' "$output" | sed 's/%(ext)s/m4a/g')
    ;;
esac

mkdir -p "$(dirname "$path")"
printf 'media' > "$path"
`
	if runtime.GOOS == "windows" {
		t.Skip("shell script test helper is not supported on windows")
	}
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return binary
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func shellSingleQuote(input string) string {
	return strings.ReplaceAll(input, `'`, `'\''`)
}
