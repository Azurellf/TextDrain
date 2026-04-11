package ingestion

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"textdrain/internal/domain"
)

func TestResolverResolvesLocalFile(t *testing.T) {
	root := t.TempDir()
	mediaPath := filepath.Join(root, "Meeting Recording.mp4")
	if err := os.WriteFile(mediaPath, []byte("media"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	resolver := NewResolver(filepath.Join(root, "jobs"), "zh")
	asset, err := resolver.Resolve(context.Background(), mediaPath)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if asset.SourceType != domain.SourceTypeLocalFile {
		t.Fatalf("SourceType = %q, want %q", asset.SourceType, domain.SourceTypeLocalFile)
	}
	if asset.RawInput != mediaPath {
		t.Fatalf("RawInput = %q, want %q", asset.RawInput, mediaPath)
	}
	if asset.Title != "Meeting Recording" {
		t.Fatalf("Title = %q, want Meeting Recording", asset.Title)
	}
	if asset.Site != localSiteName {
		t.Fatalf("Site = %q, want %q", asset.Site, localSiteName)
	}
	if asset.MediaPath == "" || !filepath.IsAbs(asset.MediaPath) {
		t.Fatalf("MediaPath = %q, want absolute path", asset.MediaPath)
	}
	if !strings.Contains(asset.WorkDir, filepath.Join("jobs", "local_file-meeting-recording-")) {
		t.Fatalf("WorkDir = %q, want sanitized local job directory", asset.WorkDir)
	}
	if asset.LanguageHint != "zh" {
		t.Fatalf("LanguageHint = %q, want zh", asset.LanguageHint)
	}
}

func TestResolverResolvesURL(t *testing.T) {
	root := t.TempDir()
	input := "https://www.youtube.com/watch?v=abc123"

	resolver := NewResolver(filepath.Join(root, "jobs"), "auto")
	asset, err := resolver.Resolve(context.Background(), input)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if asset.SourceType != domain.SourceTypeURL {
		t.Fatalf("SourceType = %q, want %q", asset.SourceType, domain.SourceTypeURL)
	}
	if asset.RawInput != input {
		t.Fatalf("RawInput = %q, want %q", asset.RawInput, input)
	}
	if asset.Site != "www.youtube.com" {
		t.Fatalf("Site = %q, want www.youtube.com", asset.Site)
	}
	if asset.MediaPath != "" {
		t.Fatalf("MediaPath = %q, want empty path before download", asset.MediaPath)
	}
	if !strings.Contains(asset.WorkDir, filepath.Join("jobs", "url-watch-")) {
		t.Fatalf("WorkDir = %q, want sanitized URL job directory", asset.WorkDir)
	}
}

func TestResolverRejectsInvalidURLWhenLocalPathDoesNotExist(t *testing.T) {
	resolver := NewResolver(t.TempDir(), "auto")

	_, err := resolver.Resolve(context.Background(), "missing-file.mp4")
	if err == nil {
		t.Fatal("Resolve() error = nil, want error")
	}
	if !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("Resolve() error = %v, want ErrInvalidURL", err)
	}
}

func TestResolverRejectsDirectoryInput(t *testing.T) {
	dir := t.TempDir()
	resolver := NewResolver(t.TempDir(), "auto")

	_, err := resolver.Resolve(context.Background(), dir)
	if err == nil {
		t.Fatal("Resolve() error = nil, want error")
	}
	if !errors.Is(err, ErrLocalNotFile) {
		t.Fatalf("Resolve() error = %v, want ErrLocalNotFile", err)
	}
}
