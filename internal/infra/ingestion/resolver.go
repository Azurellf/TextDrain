// Package ingestion normalizes user-provided media inputs.
package ingestion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"textdrain/internal/domain"
)

const (
	localSiteName = "local"
	unknownTitle  = "untitled"
)

var (
	ErrEmptyInput      = errors.New("input cannot be empty")
	ErrInvalidURL      = errors.New("input is not a valid URL")
	ErrLocalNotFile    = errors.New("local input is not a file")
	ErrLocalUnreadable = errors.New("local input is not readable")
)

// Resolver converts local file paths and URLs into a single MediaAsset shape.
type Resolver struct {
	jobsDir      string
	languageHint string
}

// NewResolver builds a source resolver with the shared jobs directory root.
func NewResolver(jobsDir string, languageHint string) *Resolver {
	return &Resolver{
		jobsDir:      jobsDir,
		languageHint: languageHint,
	}
}

// Resolve normalizes an input string into a MediaAsset.
func (r *Resolver) Resolve(ctx context.Context, input string) (domain.MediaAsset, error) {
	if err := ctx.Err(); err != nil {
		return domain.MediaAsset{}, err
	}

	rawInput := strings.TrimSpace(input)
	if rawInput == "" {
		return domain.MediaAsset{}, ErrEmptyInput
	}

	if info, err := os.Stat(rawInput); err == nil {
		return r.resolveLocal(rawInput, info)
	} else if !errors.Is(err, os.ErrNotExist) {
		return domain.MediaAsset{}, fmt.Errorf("inspect local input %s: %w", rawInput, err)
	}

	return r.resolveURL(rawInput)
}

func (r *Resolver) resolveLocal(input string, info os.FileInfo) (domain.MediaAsset, error) {
	if !info.Mode().IsRegular() {
		return domain.MediaAsset{}, fmt.Errorf("%w: %s", ErrLocalNotFile, input)
	}
	if !hasReadPermission(info.Mode()) {
		return domain.MediaAsset{}, fmt.Errorf("%w: %s", ErrLocalUnreadable, input)
	}

	file, err := os.Open(input)
	if err != nil {
		return domain.MediaAsset{}, fmt.Errorf("%w: %s: %v", ErrLocalUnreadable, input, err)
	}
	if err := file.Close(); err != nil {
		return domain.MediaAsset{}, fmt.Errorf("close local input %s: %w", input, err)
	}

	absPath, err := filepath.Abs(input)
	if err != nil {
		return domain.MediaAsset{}, fmt.Errorf("resolve absolute path %s: %w", input, err)
	}

	title := titleFromPath(absPath)
	workDir := r.workDir(domain.SourceTypeLocalFile, title, absPath)

	return domain.MediaAsset{
		SourceType:   domain.SourceTypeLocalFile,
		RawInput:     input,
		Title:        title,
		Site:         localSiteName,
		WorkDir:      workDir,
		MediaPath:    absPath,
		LanguageHint: r.languageHint,
		Metadata: map[string]string{
			"filename": filepath.Base(absPath),
		},
	}, nil
}

func (r *Resolver) resolveURL(input string) (domain.MediaAsset, error) {
	parsed, err := url.ParseRequestURI(input)
	if err != nil {
		return domain.MediaAsset{}, fmt.Errorf("%w: %s", ErrInvalidURL, input)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return domain.MediaAsset{}, fmt.Errorf("%w: unsupported scheme %q", ErrInvalidURL, parsed.Scheme)
	}
	if parsed.Host == "" {
		return domain.MediaAsset{}, fmt.Errorf("%w: missing host", ErrInvalidURL)
	}

	title := titleFromURL(parsed)
	site := parsed.Hostname()
	workDir := r.workDir(domain.SourceTypeURL, title, input)

	return domain.MediaAsset{
		SourceType:   domain.SourceTypeURL,
		RawInput:     input,
		Title:        title,
		Site:         site,
		WorkDir:      workDir,
		LanguageHint: r.languageHint,
		Metadata: map[string]string{
			"url": input,
		},
	}, nil
}

func (r *Resolver) workDir(sourceType domain.SourceType, title string, identity string) string {
	root := r.jobsDir
	if root == "" {
		root = "."
	}

	sum := sha256.Sum256([]byte(identity))
	shortHash := hex.EncodeToString(sum[:])[:12]
	name := fmt.Sprintf("%s-%s-%s", sourceType, sanitizePathPart(title), shortHash)

	return filepath.Join(root, name)
}

func titleFromPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	title := strings.TrimSuffix(base, ext)
	title = strings.TrimSpace(title)
	if title == "" {
		return unknownTitle
	}
	return title
}

func titleFromURL(parsed *url.URL) string {
	base := filepath.Base(parsed.EscapedPath())
	if base == "." || base == "/" || base == "" {
		return parsed.Hostname()
	}
	if unescaped, err := url.PathUnescape(base); err == nil {
		base = unescaped
	}
	ext := filepath.Ext(base)
	title := strings.TrimSuffix(base, ext)
	title = strings.TrimSpace(title)
	if title == "" {
		return parsed.Hostname()
	}
	return title
}

func sanitizePathPart(input string) string {
	var builder strings.Builder
	lastDash := false

	for _, r := range strings.ToLower(input) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
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
	if len(result) > 60 {
		return strings.Trim(result[:60], "-")
	}
	return result
}

func hasReadPermission(mode os.FileMode) bool {
	if runtime.GOOS == "windows" {
		return true
	}
	return mode.Perm()&0o444 != 0
}
