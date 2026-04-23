package downloader

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"textdrain/internal/domain"
)

func TestYTDLPFetchDownloadsBestAudioAndReturnsMetadata(t *testing.T) {
	binary := writeFakeYTDLP(t, fakeYTDLPConfig{
		metadataJSON: readCommandFixture(t, "ytdlp_metadata_youtube.json"),
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
		metadataJSON:      readCommandFixture(t, "ytdlp_metadata_fallback.json"),
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

func TestYTDLPInspectReturnsAssetWithMetadata(t *testing.T) {
	binary := writeFakeYTDLP(t, fakeYTDLPConfig{
		metadataJSON: readCommandFixture(t, "ytdlp_metadata_youtube.json"),
	})
	asset := domain.MediaAsset{
		SourceType: domain.SourceTypeURL,
		RawInput:   "https://example.com/watch?v=abc123",
		Title:      "watch",
		Site:       "example.com",
		WorkDir:    filepath.Join(t.TempDir(), "job"),
		Metadata:   map[string]string{"url": "https://example.com/watch?v=abc123"},
	}

	inspected, err := NewYTDLPWithBinary(binary).Inspect(context.Background(), asset)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}

	if inspected.Title != "Bad / Title: Episode 1" {
		t.Fatalf("Title = %q, want metadata title", inspected.Title)
	}
	if inspected.Site != "YouTube" {
		t.Fatalf("Site = %q, want YouTube", inspected.Site)
	}
	if inspected.Duration != 12500*time.Millisecond {
		t.Fatalf("Duration = %s, want 12.5s", inspected.Duration)
	}
	if inspected.Metadata["id"] != "abc123" {
		t.Fatalf("metadata id = %q, want abc123", inspected.Metadata["id"])
	}
	if inspected.Metadata["title"] != "Bad / Title: Episode 1" {
		t.Fatalf("metadata title = %q, want metadata title", inspected.Metadata["title"])
	}
	if inspected.Metadata["url"] != asset.RawInput {
		t.Fatalf("metadata url = %q, want original resolver metadata preserved", inspected.Metadata["url"])
	}
}

func TestYTDLPFetchUsesInspectedAssetMetadata(t *testing.T) {
	binary := writeFakeYTDLP(t, fakeYTDLPConfig{
		metadataJSON:         readCommandFixture(t, "ytdlp_metadata_youtube.json"),
		failRepeatedMetadata: true,
	})
	downloader := NewYTDLPWithBinary(binary)
	asset := domain.MediaAsset{
		SourceType: domain.SourceTypeURL,
		RawInput:   "https://example.com/watch?v=abc123",
		Title:      "watch",
		Site:       "example.com",
		WorkDir:    filepath.Join(t.TempDir(), "job"),
		Metadata:   map[string]string{"url": "https://example.com/watch?v=abc123"},
	}

	inspected, err := downloader.Inspect(context.Background(), asset)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	result, err := downloader.Fetch(context.Background(), inspected, inspected.WorkDir)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if result.Title != "Bad / Title: Episode 1" {
		t.Fatalf("Title = %q, want inspected metadata title", result.Title)
	}
	if filepath.Base(result.MediaPath) != "Bad-Title-Episode-1.m4a" {
		t.Fatalf("MediaPath basename = %q, want inspected title filename", filepath.Base(result.MediaPath))
	}
}

func TestYTDLPPassesCookieOptionsToMetadataAndDownload(t *testing.T) {
	argLogPath := filepath.Join(t.TempDir(), "yt-dlp-args.log")
	binary := writeFakeYTDLP(t, fakeYTDLPConfig{
		metadataJSON: readCommandFixture(t, "ytdlp_metadata_youtube.json"),
		argLogPath:   argLogPath,
	})
	downloader := NewYTDLPWithOptions(binary, YTDLPOptions{
		CookiesFromBrowser: "safari",
	})
	asset := domain.MediaAsset{
		SourceType: domain.SourceTypeURL,
		RawInput:   "https://example.com/watch?v=abc123",
		WorkDir:    filepath.Join(t.TempDir(), "job"),
	}

	inspected, err := downloader.Inspect(context.Background(), asset)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if _, err := downloader.Fetch(context.Background(), inspected, inspected.WorkDir); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	logData, err := os.ReadFile(argLogPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	log := string(logData)
	for _, want := range []string{
		"--dump-single-json --no-playlist --cookies-from-browser safari https://example.com/watch?v=abc123",
		"--no-playlist --cookies-from-browser safari -f bestaudio/best -o ",
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("yt-dlp arg log does not contain %q:\n%s", want, log)
		}
	}
}

func TestYTDLPPassesExtraArgsToMetadataAndDownload(t *testing.T) {
	argLogPath := filepath.Join(t.TempDir(), "yt-dlp-args.log")
	binary := writeFakeYTDLP(t, fakeYTDLPConfig{
		metadataJSON: readCommandFixture(t, "ytdlp_metadata_youtube.json"),
		argLogPath:   argLogPath,
	})
	downloader := NewYTDLPWithOptions(binary, YTDLPOptions{
		ExtraArgs: []string{"--impersonate", "chrome", "--add-header", "Referer: https://www.bilibili.com"},
	})
	asset := domain.MediaAsset{
		SourceType: domain.SourceTypeURL,
		RawInput:   "https://example.com/watch?v=abc123",
		WorkDir:    filepath.Join(t.TempDir(), "job"),
	}

	inspected, err := downloader.Inspect(context.Background(), asset)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if _, err := downloader.Fetch(context.Background(), inspected, inspected.WorkDir); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	logData, err := os.ReadFile(argLogPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	log := string(logData)
	for _, want := range []string{
		"--dump-single-json --no-playlist --impersonate chrome --add-header Referer: https://www.bilibili.com https://example.com/watch?v=abc123",
		"--no-playlist --impersonate chrome --add-header Referer: https://www.bilibili.com -f bestaudio/best -o ",
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("yt-dlp arg log does not contain %q:\n%s", want, log)
		}
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

func TestYTDLPMetadataReportsRelevantTracebackLine(t *testing.T) {
	binary := writeFakeYTDLP(t, fakeYTDLPConfig{
		metadataStderr: "Traceback (most recent call last):\nyt_dlp.utils.YoutubeDLError: Impersonate target \"chrome\" is not available.\n",
		metadataExit:   1,
	})

	_, err := NewYTDLPWithBinary(binary).Metadata(context.Background(), "https://example.com/video")
	if err == nil {
		t.Fatal("Metadata() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "YoutubeDLError: Impersonate target") {
		t.Fatalf("Metadata() error = %v, want relevant traceback line", err)
	}
}

type fakeYTDLPConfig struct {
	metadataJSON         string
	metadataStderr       string
	metadataExit         int
	failBestAudio        bool
	failRepeatedMetadata bool
	fallbackExtension    string
	argLogPath           string
}

func writeFakeYTDLP(t *testing.T, cfg fakeYTDLPConfig) string {
	t.Helper()

	dir := t.TempDir()
	binary := filepath.Join(dir, "yt-dlp")
	extension := cfg.fallbackExtension
	if extension == "" {
		extension = "m4a"
	}
	metadataGuard := filepath.Join(dir, "metadata-called")

	script := `#!/bin/sh
set -eu
if [ "` + shellSingleQuote(cfg.argLogPath) + `" != "" ]; then
  printf '%s\n' "$*" >> '` + shellSingleQuote(cfg.argLogPath) + `'
fi
if [ "$1" = "--dump-single-json" ]; then
  if [ "` + shellSingleQuote(cfg.metadataStderr) + `" != "" ]; then
    printf '%s\n' '` + shellSingleQuote(cfg.metadataStderr) + `' >&2
  fi
  if [ "` + intString(cfg.metadataExit) + `" != "0" ]; then
    exit ` + intString(cfg.metadataExit) + `
  fi
  if [ "` + boolString(cfg.failRepeatedMetadata) + `" = "true" ]; then
    if [ -f '` + shellSingleQuote(metadataGuard) + `' ]; then
      echo "metadata already read" >&2
      exit 9
    fi
    touch '` + shellSingleQuote(metadataGuard) + `'
  fi
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

func intString(value int) string {
	return strconv.Itoa(value)
}

func shellSingleQuote(input string) string {
	return strings.ReplaceAll(input, `'`, `'\''`)
}

func readCommandFixture(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "commands", name))
	if err != nil {
		t.Fatalf("ReadFile(command fixture) error = %v", err)
	}
	return strings.TrimSpace(string(data))
}
