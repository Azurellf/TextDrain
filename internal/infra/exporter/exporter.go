// Package exporter contains transcript export adapters.
package exporter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"textdrain/internal/domain"
)

const (
	defaultBaseName   = "transcript"
	metadataFileName  = "metadata.json"
	defaultOutputsDir = "outputs"
)

var (
	ErrUnsupportedFormat = errors.New("unsupported export format")
)

// FileExporter writes transcript artifacts to local files.
type FileExporter struct{}

// New creates a local file transcript exporter.
func New() *FileExporter {
	return &FileExporter{}
}

// Export writes the requested transcript formats into outputDir.
func (e *FileExporter) Export(ctx context.Context, transcript domain.Transcript, outputDir string, formats []domain.OutputFormat) ([]string, error) {
	outputDir = resolvedOutputDir(transcript, outputDir)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create export output directory %s: %w", outputDir, err)
	}
	if err := writeMetadataFile(outputDir, transcript); err != nil {
		return nil, err
	}

	normalizedFormats := normalizeFormats(formats)
	paths := make([]string, 0, len(normalizedFormats))
	for _, format := range normalizedFormats {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		content, err := render(transcript, format)
		if err != nil {
			return nil, err
		}
		path := filepath.Join(outputDir, defaultBaseName+"."+string(format))
		if err := writeFile(path, content); err != nil {
			return nil, err
		}
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolve export path %s: %w", path, err)
		}
		paths = append(paths, absPath)
	}

	return paths, nil
}

func normalizeFormats(formats []domain.OutputFormat) []domain.OutputFormat {
	if len(formats) == 0 {
		return []domain.OutputFormat{
			domain.OutputFormatTXT,
			domain.OutputFormatSRT,
			domain.OutputFormatVTT,
			domain.OutputFormatJSON,
		}
	}
	return formats
}

func resolvedOutputDir(transcript domain.Transcript, outputDir string) string {
	baseDir := strings.TrimSpace(outputDir)
	if baseDir == "" {
		baseDir = defaultOutputsDir
	}
	return filepath.Join(baseDir, resolvedFolderName(transcript.Metadata))
}

func render(transcript domain.Transcript, format domain.OutputFormat) ([]byte, error) {
	switch format {
	case domain.OutputFormatTXT:
		return []byte(renderTXT(transcript)), nil
	case domain.OutputFormatSRT:
		return []byte(renderSRT(transcript)), nil
	case domain.OutputFormatVTT:
		return []byte(renderVTT(transcript)), nil
	case domain.OutputFormatJSON:
		return renderJSON(transcript)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, format)
	}
}

func renderTXT(transcript domain.Transcript) string {
	text := strings.TrimSpace(transcript.Text)
	if text == "" {
		text = joinedSegmentText(transcript.Segments)
	}
	if text == "" {
		return ""
	}
	return text + "\n"
}

func renderSRT(transcript domain.Transcript) string {
	var builder strings.Builder
	cueIndex := 1
	for _, segment := range transcript.Segments {
		text := strings.TrimSpace(segment.Text)
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(fmt.Sprintf("%d\n", cueIndex))
		builder.WriteString(formatSRTTimestamp(segment.StartMs))
		builder.WriteString(" --> ")
		builder.WriteString(formatSRTTimestamp(segment.EndMs))
		builder.WriteByte('\n')
		builder.WriteString(text)
		builder.WriteString("\n")
		cueIndex++
	}
	return builder.String()
}

func renderVTT(transcript domain.Transcript) string {
	var builder strings.Builder
	builder.WriteString("WEBVTT\n\n")
	for _, segment := range transcript.Segments {
		text := strings.TrimSpace(segment.Text)
		if text == "" {
			continue
		}
		builder.WriteString(formatVTTTimestamp(segment.StartMs))
		builder.WriteString(" --> ")
		builder.WriteString(formatVTTTimestamp(segment.EndMs))
		builder.WriteByte('\n')
		builder.WriteString(text)
		builder.WriteString("\n\n")
	}
	return builder.String()
}

type transcriptJSON struct {
	Language string                  `json:"language"`
	Text     string                  `json:"text"`
	Segments []transcriptSegmentJSON `json:"segments"`
	Engine   transcriptEngineJSON    `json:"engine"`
	Metadata map[string]string       `json:"metadata,omitempty"`
}

type transcriptSegmentJSON struct {
	Index      int      `json:"index"`
	StartMs    int64    `json:"start_ms"`
	EndMs      int64    `json:"end_ms"`
	Text       string   `json:"text"`
	Confidence *float64 `json:"confidence,omitempty"`
}

type transcriptEngineJSON struct {
	Name         string `json:"name,omitempty"`
	ModelName    string `json:"model_name,omitempty"`
	ModelPath    string `json:"model_path,omitempty"`
	LanguageMode string `json:"language_mode,omitempty"`
}

func renderJSON(transcript domain.Transcript) ([]byte, error) {
	segments := make([]transcriptSegmentJSON, 0, len(transcript.Segments))
	for _, segment := range transcript.Segments {
		segments = append(segments, transcriptSegmentJSON{
			Index:      segment.Index,
			StartMs:    segment.StartMs,
			EndMs:      segment.EndMs,
			Text:       segment.Text,
			Confidence: segment.Confidence,
		})
	}

	payload := transcriptJSON{
		Language: transcript.Language,
		Text:     transcript.Text,
		Segments: segments,
		Engine: transcriptEngineJSON{
			Name:         transcript.Engine.Name,
			ModelName:    transcript.Engine.ModelName,
			ModelPath:    transcript.Engine.ModelPath,
			LanguageMode: transcript.Engine.LanguageMode,
		},
		Metadata: transcript.Metadata,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode transcript json: %w", err)
	}
	return append(data, '\n'), nil
}

func joinedSegmentText(segments []domain.TranscriptSegment) string {
	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		text := strings.TrimSpace(segment.Text)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func formatSRTTimestamp(ms int64) string {
	hours, minutes, seconds, millis := splitMilliseconds(ms)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, millis)
}

func formatVTTTimestamp(ms int64) string {
	hours, minutes, seconds, millis := splitMilliseconds(ms)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, millis)
}

func splitMilliseconds(ms int64) (int64, int64, int64, int64) {
	if ms < 0 {
		ms = 0
	}
	hours := ms / 3_600_000
	ms %= 3_600_000
	minutes := ms / 60_000
	ms %= 60_000
	seconds := ms / 1_000
	millis := ms % 1_000
	return hours, minutes, seconds, millis
}

func writeFile(path string, content []byte) error {
	tempPath := path + ".tmp"
	defer os.Remove(tempPath)
	if err := os.WriteFile(tempPath, content, 0o644); err != nil {
		return fmt.Errorf("write export temp file %s: %w", tempPath, err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("move export file to final path %s: %w", path, err)
	}
	return nil
}

func writeMetadataFile(outputDir string, transcript domain.Transcript) error {
	data, err := renderMetadataJSON(transcript.Metadata)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(outputDir, metadataFileName), data)
}

func renderMetadataJSON(metadata map[string]string) ([]byte, error) {
	payload := map[string]string{}
	for _, field := range []string{"title", "url", "id", "source_type", "site", "job_id"} {
		var value string
		switch field {
		case "url":
			value = selectedURL(metadata)
		default:
			value = strings.TrimSpace(metadata[field])
		}
		if value != "" {
			payload[field] = value
		}
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode export metadata json: %w", err)
	}
	return append(data, '\n'), nil
}

func resolvedFolderName(metadata map[string]string) string {
	title := sanitizeTitle(metadata)
	if title == defaultBaseName {
		return defaultBaseName
	}

	if strings.TrimSpace(metadata["source_type"]) != string(domain.SourceTypeURL) {
		return title
	}

	id := sanitizeFilename(metadata["id"])
	if id == defaultBaseName {
		return title
	}
	return sanitizeFilename(title + "-" + id)
}

func sanitizeTitle(metadata map[string]string) string {
	title := strings.TrimSpace(metadata["title"])
	if title == "" {
		return defaultBaseName
	}
	title = strings.TrimSuffix(title, filepath.Ext(title))
	return sanitizeFilename(title)
}

func selectedURL(metadata map[string]string) string {
	for _, key := range []string{"webpage_url", "original_url", "input"} {
		if value := strings.TrimSpace(metadata[key]); value != "" {
			return value
		}
	}
	return ""
}

func sanitizeFilename(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultBaseName
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range input {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r):
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
		return defaultBaseName
	}
	if len(result) > 80 {
		return strings.Trim(result[:80], "-")
	}
	return result
}
