// Package models discovers local transcription model files.
package models

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Model describes a local model file.
type Model struct {
	Name      string
	Path      string
	SizeBytes int64
	Default   bool
}

// Discover scans modelDir for supported local model files.
func Discover(modelDir string, defaultModel string) ([]Model, error) {
	if strings.TrimSpace(modelDir) == "" {
		return nil, errors.New("model directory is not configured")
	}

	entries, err := os.ReadDir(modelDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Model{}, nil
		}
		return nil, err
	}

	defaultPaths := CandidatePathSet(modelDir, defaultModel)
	models := make([]Model, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isSupportedModelFile(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		path := filepath.Clean(filepath.Join(modelDir, entry.Name()))
		models = append(models, Model{
			Name:      entry.Name(),
			Path:      path,
			SizeBytes: info.Size(),
			Default:   defaultPaths[path],
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].Name < models[j].Name
	})

	return models, nil
}

// CandidatePaths returns the local file paths that can satisfy modelName.
func CandidatePaths(modelDir string, modelName string) []string {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil
	}
	if filepath.IsAbs(modelName) || strings.ContainsRune(modelName, filepath.Separator) {
		return []string{filepath.Clean(modelName)}
	}

	candidates := make([]string, 0, 8)
	for _, name := range CandidateNames(modelName) {
		candidates = append(candidates, filepath.Clean(filepath.Join(modelDir, name)))
	}
	return candidates
}

// CandidatePathSet returns a set form of CandidatePaths.
func CandidatePathSet(modelDir string, modelName string) map[string]bool {
	set := map[string]bool{}
	for _, path := range CandidatePaths(modelDir, modelName) {
		set[path] = true
	}
	return set
}

// CandidateNames returns file names that can satisfy a short model name.
func CandidateNames(modelName string) []string {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil
	}

	seen := map[string]struct{}{}
	candidates := make([]string, 0, 8)
	add := func(name string) {
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		candidates = append(candidates, name)
	}

	add(modelName)
	add(modelName + ".gguf")
	add(modelName + ".bin")
	add("ggml-" + modelName + ".gguf")
	add("ggml-" + modelName + ".bin")
	add("ggml-" + modelName + ".q5_0.gguf")
	add("ggml-" + modelName + ".q5_0.bin")

	return candidates
}

func isSupportedModelFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".gguf" || ext == ".bin"
}
