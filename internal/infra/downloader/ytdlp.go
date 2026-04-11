// Package downloader contains URL-backed media download integrations.
package downloader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"textdrain/internal/domain"
)

const (
	defaultYTDLPBinary = "yt-dlp"
	unknownTitle       = "untitled"
)

var (
	ErrNonURLAsset      = errors.New("downloader requires a URL media asset")
	ErrNoDownloadedFile = errors.New("yt-dlp did not produce a media file")
)

// YTDLP shells out to yt-dlp to inspect and download URL media.
type YTDLP struct {
	binary string
}

// NewYTDLP creates a downloader using yt-dlp from PATH.
func NewYTDLP() *YTDLP {
	return &YTDLP{binary: defaultYTDLPBinary}
}

// NewYTDLPWithBinary creates a downloader using an explicit yt-dlp-compatible executable.
func NewYTDLPWithBinary(binary string) *YTDLP {
	if strings.TrimSpace(binary) == "" {
		binary = defaultYTDLPBinary
	}
	return &YTDLP{binary: binary}
}

// Fetch downloads URL-backed media into workdir and returns structured metadata.
func (d *YTDLP) Fetch(ctx context.Context, asset domain.MediaAsset, workdir string) (domain.DownloadResult, error) {
	if asset.SourceType != domain.SourceTypeURL {
		return domain.DownloadResult{}, fmt.Errorf("%w: %s", ErrNonURLAsset, asset.SourceType)
	}
	if strings.TrimSpace(asset.RawInput) == "" {
		return domain.DownloadResult{}, fmt.Errorf("%w: missing URL", ErrNonURLAsset)
	}
	if strings.TrimSpace(workdir) == "" {
		workdir = asset.WorkDir
	}
	if strings.TrimSpace(workdir) == "" {
		return domain.DownloadResult{}, errors.New("download workdir cannot be empty")
	}
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return domain.DownloadResult{}, fmt.Errorf("create download workdir %s: %w", workdir, err)
	}

	metadata, err := d.Metadata(ctx, asset.RawInput)
	if err != nil {
		return domain.DownloadResult{}, err
	}

	title := firstNonEmpty(metadata.Title, asset.Title, unknownTitle)
	site := firstNonEmpty(metadata.ExtractorKey, metadata.Extractor, asset.Site)
	originalURL := firstNonEmpty(metadata.OriginalURL, metadata.WebpageURL, asset.RawInput)
	safeTitle := sanitizeFilename(title)

	tempDir, err := os.MkdirTemp(workdir, "download-*")
	if err != nil {
		return domain.DownloadResult{}, fmt.Errorf("create temporary download directory in %s: %w", workdir, err)
	}
	defer os.RemoveAll(tempDir)

	outputTemplate := filepath.Join(tempDir, safeTitle+".%(ext)s")
	tempMediaPath, err := d.download(ctx, asset.RawInput, outputTemplate)
	if err != nil {
		return domain.DownloadResult{}, err
	}
	mediaPath, err := moveToFinalPath(tempMediaPath, filepath.Join(workdir, safeTitle+filepath.Ext(tempMediaPath)))
	if err != nil {
		return domain.DownloadResult{}, err
	}

	updatedAsset := asset
	updatedAsset.Title = title
	updatedAsset.Site = site
	updatedAsset.Duration = metadata.duration()
	updatedAsset.MediaPath = mediaPath
	updatedAsset.Metadata = mergeMetadata(asset.Metadata, metadata.toMap())

	return domain.DownloadResult{
		Asset:       updatedAsset,
		MediaPath:   mediaPath,
		OriginalURL: originalURL,
		Title:       title,
		Site:        site,
		Duration:    metadata.duration(),
		Metadata:    updatedAsset.Metadata,
	}, nil
}

// Metadata returns structured yt-dlp metadata without downloading media.
func (d *YTDLP) Metadata(ctx context.Context, rawURL string) (Metadata, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return Metadata{}, errors.New("metadata URL cannot be empty")
	}

	stdout, stderr, err := d.run(ctx, "--dump-single-json", "--no-playlist", rawURL)
	if err != nil {
		return Metadata{}, commandError("read metadata", err, stderr)
	}

	var metadata Metadata
	decoder := json.NewDecoder(bytes.NewReader(stdout))
	decoder.UseNumber()
	if err := decoder.Decode(&metadata); err != nil {
		return Metadata{}, fmt.Errorf("parse yt-dlp metadata: %w", err)
	}
	if metadata.Title == "" {
		metadata.Title = unknownTitle
	}

	return metadata, nil
}

func (d *YTDLP) download(ctx context.Context, rawURL string, outputTemplate string) (string, error) {
	args := []string{"--no-playlist", "-f", "bestaudio/best", "-o", outputTemplate, rawURL}
	_, stderr, err := d.run(ctx, args...)
	if err != nil {
		fallbackArgs := []string{"--no-playlist", "-f", "best", "-o", outputTemplate, rawURL}
		if _, fallbackStderr, fallbackErr := d.run(ctx, fallbackArgs...); fallbackErr != nil {
			return "", commandError("download media", fallbackErr, append(stderr, fallbackStderr...))
		}
	}

	mediaPath, err := findDownloadedFile(outputTemplate)
	if err != nil {
		return "", err
	}
	return mediaPath, nil
}

func (d *YTDLP) run(ctx context.Context, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, d.binary, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return stdout.Bytes(), stderr.Bytes(), ctxErr
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

// Metadata is the subset of yt-dlp JSON used by TextDrain.
type Metadata struct {
	ID           string      `json:"id"`
	Title        string      `json:"title"`
	Extractor    string      `json:"extractor"`
	ExtractorKey string      `json:"extractor_key"`
	WebpageURL   string      `json:"webpage_url"`
	OriginalURL  string      `json:"original_url"`
	Duration     json.Number `json:"duration"`
}

func (m Metadata) duration() time.Duration {
	if m.Duration == "" {
		return 0
	}
	seconds, err := strconv.ParseFloat(m.Duration.String(), 64)
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func (m Metadata) toMap() map[string]string {
	values := map[string]string{
		"id":            m.ID,
		"title":         m.Title,
		"extractor":     m.Extractor,
		"extractor_key": m.ExtractorKey,
		"webpage_url":   m.WebpageURL,
		"original_url":  m.OriginalURL,
		"duration":      m.Duration.String(),
	}

	for key, value := range values {
		if value == "" {
			delete(values, key)
		}
	}
	return values
}

func findDownloadedFile(outputTemplate string) (string, error) {
	pattern := strings.ReplaceAll(outputTemplate, "%(ext)s", "*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("inspect downloaded files %s: %w", pattern, err)
	}

	for _, match := range matches {
		if isTemporaryDownloadPath(match) {
			continue
		}
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		if info.Mode().IsRegular() {
			absPath, err := filepath.Abs(match)
			if err != nil {
				return "", fmt.Errorf("resolve downloaded media path %s: %w", match, err)
			}
			return absPath, nil
		}
	}

	return "", fmt.Errorf("%w: %s", ErrNoDownloadedFile, pattern)
}

func moveToFinalPath(sourcePath string, targetPath string) (string, error) {
	targetPath, err := nextAvailablePath(targetPath)
	if err != nil {
		return "", err
	}
	if err := os.Rename(sourcePath, targetPath); err != nil {
		return "", fmt.Errorf("move downloaded media to final path %s: %w", targetPath, err)
	}
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("resolve downloaded media path %s: %w", targetPath, err)
	}
	return absPath, nil
}

func nextAvailablePath(path string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return path, nil
		}
		return "", fmt.Errorf("inspect final media path %s: %w", path, err)
	}

	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(path, ext)
	for index := 1; ; index++ {
		candidate := fmt.Sprintf("%s-%d%s", stem, index, ext)
		if _, err := os.Stat(candidate); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return candidate, nil
			}
			return "", fmt.Errorf("inspect final media path %s: %w", candidate, err)
		}
	}
}

func isTemporaryDownloadPath(path string) bool {
	base := filepath.Base(path)
	return strings.HasSuffix(base, ".part") ||
		strings.HasSuffix(base, ".ytdl")
}

func commandError(stage string, err error, stderr []byte) error {
	message := strings.TrimSpace(string(stderr))
	if message == "" {
		return fmt.Errorf("yt-dlp %s failed: %w", stage, err)
	}
	return fmt.Errorf("yt-dlp %s failed: %s: %w", stage, firstLine(message), err)
}

func mergeMetadata(existing map[string]string, extra map[string]string) map[string]string {
	merged := make(map[string]string, len(existing)+len(extra))
	for key, value := range existing {
		merged[key] = value
	}
	for key, value := range extra {
		merged[key] = value
	}
	return merged
}

func sanitizeFilename(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return unknownTitle
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}

	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return unknownTitle
	}
	if len(result) > 80 {
		return strings.Trim(result[:80], "-")
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func firstLine(input string) string {
	line, _, _ := strings.Cut(input, "\n")
	return strings.TrimSpace(line)
}
