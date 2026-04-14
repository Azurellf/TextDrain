package exporter

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"textdrain/internal/domain"
)

func TestFileExporterExportsRequestedFormats(t *testing.T) {
	outputDir := t.TempDir()
	transcript := sampleTranscript()

	paths, err := New().Export(context.Background(), transcript, outputDir, []domain.OutputFormat{
		domain.OutputFormatTXT,
		domain.OutputFormatSRT,
		domain.OutputFormatVTT,
		domain.OutputFormatJSON,
	})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if len(paths) != 4 {
		t.Fatalf("Export() paths len = %d, want 4", len(paths))
	}
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			t.Fatalf("Export() path = %q, want absolute path", path)
		}
	}

	assertFile(t, filepath.Join(outputDir, "Bad-Title-Episode-1.txt"), "hello\nworld\n")
	assertFile(t, filepath.Join(outputDir, "Bad-Title-Episode-1.srt"), "1\n00:00:00,000 --> 00:00:01,240\nhello\n\n2\n00:00:01,240 --> 00:01:01,005\nworld\n")
	assertFile(t, filepath.Join(outputDir, "Bad-Title-Episode-1.vtt"), "WEBVTT\n\n00:00:00.000 --> 00:00:01.240\nhello\n\n00:00:01.240 --> 00:01:01.005\nworld\n\n")

	data := readFile(t, filepath.Join(outputDir, "Bad-Title-Episode-1.json"))
	var payload struct {
		Language string `json:"language"`
		Text     string `json:"text"`
		Segments []struct {
			Index   int    `json:"index"`
			StartMs int64  `json:"start_ms"`
			EndMs   int64  `json:"end_ms"`
			Text    string `json:"text"`
		} `json:"segments"`
		Engine struct {
			Name         string `json:"name"`
			ModelName    string `json:"model_name"`
			LanguageMode string `json:"language_mode"`
		} `json:"engine"`
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if payload.Language != "zh" || payload.Text != "hello\nworld" {
		t.Fatalf("payload language/text = %q/%q, want transcript values", payload.Language, payload.Text)
	}
	if len(payload.Segments) != 2 || payload.Segments[1].EndMs != 61005 {
		t.Fatalf("payload segments = %#v, want stable segment fields", payload.Segments)
	}
	if payload.Engine.Name != "whisper.cpp" || payload.Engine.ModelName != "small" || payload.Engine.LanguageMode != "auto" {
		t.Fatalf("payload engine = %#v, want engine metadata", payload.Engine)
	}
	if payload.Metadata["title"] != "Bad / Title: Episode 1" {
		t.Fatalf("payload metadata title = %q, want source title", payload.Metadata["title"])
	}
}

func TestFileExporterDefaultsToAllFormats(t *testing.T) {
	outputDir := t.TempDir()

	paths, err := New().Export(context.Background(), sampleTranscript(), outputDir, nil)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if len(paths) != 4 {
		t.Fatalf("Export() paths len = %d, want 4", len(paths))
	}
	for _, ext := range []string{".txt", ".srt", ".vtt", ".json"} {
		if _, err := os.Stat(filepath.Join(outputDir, "Bad-Title-Episode-1"+ext)); err != nil {
			t.Fatalf("Stat(%s) error = %v", ext, err)
		}
	}
}

func TestFileExporterDefaultsOutputDirFromJobID(t *testing.T) {
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDir); err != nil {
			t.Fatalf("Chdir(previous) error = %v", err)
		}
	})

	transcript := sampleTranscript()
	transcript.Metadata["job_id"] = "job/001"
	paths, err := New().Export(context.Background(), transcript, "", []domain.OutputFormat{domain.OutputFormatTXT})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if len(paths) != 1 || filepath.Base(filepath.Dir(paths[0])) != "job-001" || filepath.Base(filepath.Dir(filepath.Dir(paths[0]))) != "outputs" {
		t.Fatalf("Export() paths = %#v, want default outputs/job-001 directory", paths)
	}
	assertFile(t, paths[0], "hello\nworld\n")
}

func TestFileExporterFallsBackToFilenameAndTranscriptName(t *testing.T) {
	outputDir := t.TempDir()
	transcript := domain.Transcript{
		Text: "text",
		Metadata: map[string]string{
			"filename": "/tmp/Local Clip.mp4",
		},
	}

	_, err := New().Export(context.Background(), transcript, outputDir, []domain.OutputFormat{domain.OutputFormatTXT})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	assertFile(t, filepath.Join(outputDir, "Local-Clip.txt"), "text\n")

	outputDir = t.TempDir()
	_, err = New().Export(context.Background(), domain.Transcript{Text: "text"}, outputDir, []domain.OutputFormat{domain.OutputFormatTXT})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	assertFile(t, filepath.Join(outputDir, "transcript.txt"), "text\n")
}

func TestFileExporterRejectsInvalidInput(t *testing.T) {
	_, err := New().Export(context.Background(), sampleTranscript(), "", []domain.OutputFormat{domain.OutputFormatTXT})
	if !errors.Is(err, ErrEmptyOutputDir) {
		t.Fatalf("Export() error = %v, want ErrEmptyOutputDir", err)
	}

	_, err = New().Export(context.Background(), sampleTranscript(), t.TempDir(), []domain.OutputFormat{"docx"})
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("Export() error = %v, want ErrUnsupportedFormat", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = New().Export(ctx, sampleTranscript(), t.TempDir(), []domain.OutputFormat{domain.OutputFormatTXT})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Export() error = %v, want context.Canceled", err)
	}
}

func TestFileExporterTXTJoinsSegmentsWhenTextMissing(t *testing.T) {
	outputDir := t.TempDir()
	transcript := sampleTranscript()
	transcript.Text = ""
	transcript.Segments = append(transcript.Segments, domain.TranscriptSegment{
		Index:   3,
		StartMs: 62000,
		EndMs:   63000,
		Text:    " ",
	})

	_, err := New().Export(context.Background(), transcript, outputDir, []domain.OutputFormat{domain.OutputFormatTXT})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	assertFile(t, filepath.Join(outputDir, "Bad-Title-Episode-1.txt"), "hello\nworld\n")
}

func sampleTranscript() domain.Transcript {
	return domain.Transcript{
		Language: "zh",
		Text:     "hello\nworld",
		Segments: []domain.TranscriptSegment{
			{Index: 0, StartMs: 0, EndMs: 1240, Text: " hello "},
			{Index: 1, StartMs: 1240, EndMs: 61005, Text: "world"},
		},
		Engine: domain.TranscriptEngine{
			Name:         "whisper.cpp",
			ModelName:    "small",
			LanguageMode: "auto",
		},
		Metadata: map[string]string{
			"title": "Bad / Title: Episode 1",
		},
	}
}

func assertFile(t *testing.T, path string, want string) {
	t.Helper()
	got := readFile(t, path)
	if got != want {
		t.Fatalf("%s = %q, want %q", filepath.Base(path), got, want)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}
