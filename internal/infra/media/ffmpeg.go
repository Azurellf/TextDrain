// Package media contains local media processing integrations.
package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"textdrain/internal/domain"
)

const (
	defaultFFmpegBinary = "ffmpeg"
	defaultSampleRateHz = 16000
	defaultChannels     = 1
	defaultCodec        = "pcm_s16le"
	defaultAudioExt     = ".wav"
	unknownTitle        = "audio"
)

var (
	ErrEmptyMediaPath = errors.New("media path cannot be empty")
	ErrEmptyWorkDir   = errors.New("media workdir cannot be empty")
	ErrMediaNotFile   = errors.New("media path is not a file")
)

// FFmpeg shells out to ffmpeg to extract and normalize local audio.
type FFmpeg struct {
	binary string
}

// NewFFmpeg creates an audio processor using ffmpeg from PATH.
func NewFFmpeg() *FFmpeg {
	return &FFmpeg{binary: defaultFFmpegBinary}
}

// NewFFmpegWithBinary creates an audio processor using an explicit ffmpeg-compatible executable.
func NewFFmpegWithBinary(binary string) *FFmpeg {
	if strings.TrimSpace(binary) == "" {
		binary = defaultFFmpegBinary
	}
	return &FFmpeg{binary: binary}
}

// Prepare converts any ffmpeg-readable media file into mono 16kHz s16le WAV by default.
func (p *FFmpeg) Prepare(ctx context.Context, mediaPath string, workdir string, opts domain.AudioOptions) (domain.PreparedAudio, error) {
	mediaPath = strings.TrimSpace(mediaPath)
	if mediaPath == "" {
		return domain.PreparedAudio{}, ErrEmptyMediaPath
	}
	workdir = strings.TrimSpace(workdir)
	if workdir == "" {
		return domain.PreparedAudio{}, ErrEmptyWorkDir
	}
	if err := ctx.Err(); err != nil {
		return domain.PreparedAudio{}, err
	}

	sourcePath, err := validateMediaPath(mediaPath)
	if err != nil {
		return domain.PreparedAudio{}, err
	}
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return domain.PreparedAudio{}, fmt.Errorf("create media workdir %s: %w", workdir, err)
	}

	normalized := normalizeOptions(opts)
	outputPath, err := nextAvailablePath(filepath.Join(workdir, sanitizeFilename(sourceTitle(sourcePath))+defaultAudioExt))
	if err != nil {
		return domain.PreparedAudio{}, err
	}
	tempPath := outputPath + ".tmp"
	defer os.Remove(tempPath)

	if err := p.run(ctx, sourcePath, tempPath, normalized); err != nil {
		return domain.PreparedAudio{}, err
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		return domain.PreparedAudio{}, fmt.Errorf("move prepared audio to final path %s: %w", outputPath, err)
	}

	absOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return domain.PreparedAudio{}, fmt.Errorf("resolve prepared audio path %s: %w", outputPath, err)
	}

	return domain.PreparedAudio{
		SourcePath:   sourcePath,
		Path:         absOutputPath,
		SampleRateHz: normalized.SampleRateHz,
		Channels:     normalized.Channels,
		Codec:        normalized.Codec,
	}, nil
}

func (p *FFmpeg) run(ctx context.Context, mediaPath string, outputPath string, opts domain.AudioOptions) error {
	args := []string{
		"-hide_banner",
		"-nostdin",
		"-y",
		"-i", mediaPath,
		"-vn",
		"-ac", fmt.Sprintf("%d", opts.Channels),
		"-ar", fmt.Sprintf("%d", opts.SampleRateHz),
		"-acodec", opts.Codec,
	}
	if filter := audioFilter(opts); filter != "" {
		args = append(args, "-af", filter)
	}
	args = append(args, outputPath)

	cmd := exec.CommandContext(ctx, p.binary, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return commandError(err, stderr.Bytes())
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func normalizeOptions(opts domain.AudioOptions) domain.AudioOptions {
	if opts.SampleRateHz <= 0 {
		opts.SampleRateHz = defaultSampleRateHz
	}
	if opts.Channels <= 0 {
		opts.Channels = defaultChannels
	}
	if strings.TrimSpace(opts.Codec) == "" {
		opts.Codec = defaultCodec
	} else {
		opts.Codec = strings.TrimSpace(opts.Codec)
	}
	return opts
}

func audioFilter(opts domain.AudioOptions) string {
	filters := make([]string, 0, 2)
	if opts.LoudnessNormalize {
		filters = append(filters, "loudnorm")
	}
	if opts.TrimSilence {
		filters = append(filters, "silenceremove=start_periods=1:start_duration=0.1:start_threshold=-50dB")
	}
	return strings.Join(filters, ",")
}

func validateMediaPath(mediaPath string) (string, error) {
	info, err := os.Stat(mediaPath)
	if err != nil {
		return "", fmt.Errorf("inspect media path %s: %w", mediaPath, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: %s", ErrMediaNotFile, mediaPath)
	}

	absPath, err := filepath.Abs(mediaPath)
	if err != nil {
		return "", fmt.Errorf("resolve media path %s: %w", mediaPath, err)
	}
	return absPath, nil
}

func sourceTitle(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	title := strings.TrimSpace(strings.TrimSuffix(base, ext))
	if title == "" {
		return unknownTitle
	}
	return title
}

func commandError(err error, stderr []byte) error {
	message := strings.TrimSpace(string(stderr))
	if message == "" {
		return fmt.Errorf("ffmpeg audio preparation failed: %w", err)
	}
	return fmt.Errorf("ffmpeg audio preparation failed: %s: %w", firstLine(message), err)
}

func nextAvailablePath(path string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return path, nil
		}
		return "", fmt.Errorf("inspect prepared audio path %s: %w", path, err)
	}

	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(path, ext)
	for index := 1; ; index++ {
		candidate := fmt.Sprintf("%s-%d%s", stem, index, ext)
		if _, err := os.Stat(candidate); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return candidate, nil
			}
			return "", fmt.Errorf("inspect prepared audio path %s: %w", candidate, err)
		}
	}
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

func firstLine(input string) string {
	line, _, _ := strings.Cut(input, "\n")
	return strings.TrimSpace(line)
}
