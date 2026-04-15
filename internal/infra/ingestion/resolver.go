// Package ingestion normalizes user-provided media inputs.
package ingestion

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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
	jobID := newJobID(domain.SourceTypeLocalFile, title, absPath)

	return domain.MediaAsset{
		JobID:        jobID,
		SourceType:   domain.SourceTypeLocalFile,
		RawInput:     input,
		Title:        title,
		Site:         localSiteName,
		WorkDir:      r.workDir(jobID),
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
	jobID := newJobID(domain.SourceTypeURL, title, input)

	return domain.MediaAsset{
		JobID:        jobID,
		SourceType:   domain.SourceTypeURL,
		RawInput:     input,
		Title:        title,
		Site:         site,
		WorkDir:      r.workDir(jobID),
		LanguageHint: r.languageHint,
		Metadata: map[string]string{
			"url": input,
		},
	}, nil
}

func (r *Resolver) workDir(jobID string) string {
	root := r.jobsDir
	if root == "" {
		root = "."
	}

	return filepath.Join(root, jobID)
}

func newJobID(sourceType domain.SourceType, title string, identity string) string {
	sum := sha256.Sum256([]byte(identity))
	identityHash := hex.EncodeToString(sum[:])[:8]
	now := time.Now().UTC()
	timestamp := fmt.Sprintf("%s%09dZ", now.Format("20060102T150405"), now.Nanosecond())
	randomSuffix := randomHex(4)

	return fmt.Sprintf("%s-%s-%s-%s-%s", sanitizePathPart(string(sourceType)), sanitizePathPart(title), timestamp, identityHash, randomSuffix)
}

func randomHex(size int) string {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	return hex.EncodeToString(data)
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
