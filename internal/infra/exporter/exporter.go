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

	"textdrain/internal/domain"
)

const (
	defaultBaseName   = "transcript"
	defaultOutputsDir = "outputs"
)

var (
	ErrEmptyOutputDir    = errors.New("export output directory cannot be empty")
	ErrMissingJobID      = errors.New("export job_id metadata is required for default output directory")
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
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		defaultDir, err := defaultOutputDir(transcript)
		if err != nil {
			return nil, err
		}
		outputDir = defaultDir
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create export output directory %s: %w", outputDir, err)
	}

	normalizedFormats := normalizeFormats(formats)
	baseName := transcriptBaseName(transcript)
	paths := make([]string, 0, len(normalizedFormats))
	for _, format := range normalizedFormats {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		content, err := render(transcript, format)
		if err != nil {
			return nil, err
		}
		path := filepath.Join(outputDir, baseName+"."+string(format))
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

func defaultOutputDir(transcript domain.Transcript) (string, error) {
	jobID := strings.TrimSpace(transcript.Metadata["job_id"])
	if jobID == "" {
		jobID = strings.TrimSpace(transcript.Metadata["job-id"])
	}
	if jobID == "" {
		return "", ErrEmptyOutputDir
	}
	jobID = sanitizeFilename(jobID)
	if jobID == defaultBaseName {
		return "", ErrMissingJobID
	}
	return filepath.Join(defaultOutputsDir, jobID), nil
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

func transcriptBaseName(transcript domain.Transcript) string {
	if value := strings.TrimSpace(transcript.Metadata["title"]); value != "" {
		value = strings.TrimSuffix(value, filepath.Ext(value))
		if baseName := sanitizeFilename(value); baseName != "" {
			return baseName
		}
	}
	for _, key := range []string{"filename", "input_filename", "source_filename", "media_path", "source_path", "input"} {
		value := strings.TrimSpace(transcript.Metadata[key])
		if value == "" {
			continue
		}
		if strings.ContainsAny(value, `/\`) {
			value = filepath.Base(value)
		}
		value = strings.TrimSuffix(value, filepath.Ext(value))
		if baseName := sanitizeFilename(value); baseName != "" {
			return baseName
		}
	}
	return defaultBaseName
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
		return defaultBaseName
	}
	if len(result) > 80 {
		return strings.Trim(result[:80], "-")
	}
	return result
}
