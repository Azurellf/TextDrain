package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"textdrain/internal/domain"
)

func TestLoadUsesDefaultsWhenConfigFileIsMissing(t *testing.T) {
	paths := testPaths(t)

	cfg, err := Load(paths, Overrides{})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Model != DefaultModel {
		t.Fatalf("Model = %q, want %q", cfg.Model, DefaultModel)
	}
	if cfg.Language != DefaultLanguage {
		t.Fatalf("Language = %q, want %q", cfg.Language, DefaultLanguage)
	}
	if !reflect.DeepEqual(cfg.OutputFormats, DefaultOutputFormats) {
		t.Fatalf("OutputFormats = %#v, want %#v", cfg.OutputFormats, DefaultOutputFormats)
	}
	if cfg.KeepIntermediateFiles {
		t.Fatal("KeepIntermediateFiles = true, want false")
	}
	if cfg.ModelDir != paths.ModelsDir {
		t.Fatalf("ModelDir = %q, want %q", cfg.ModelDir, paths.ModelsDir)
	}
	if cfg.JobsDir != paths.JobsDir {
		t.Fatalf("JobsDir = %q, want %q", cfg.JobsDir, paths.JobsDir)
	}
}

func TestLoadReadsConfigFile(t *testing.T) {
	paths := testPaths(t)
	writeConfig(t, paths.ConfigFile, `
model = "base"
language = "zh"
output_formats = ["txt", "srt"]
keep_intermediate_files = true
model_dir = "/models"
jobs_dir = "/jobs"
`)

	cfg, err := Load(paths, Overrides{})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	wantFormats := []domain.OutputFormat{domain.OutputFormatTXT, domain.OutputFormatSRT}
	if cfg.Model != "base" {
		t.Fatalf("Model = %q, want base", cfg.Model)
	}
	if cfg.Language != "zh" {
		t.Fatalf("Language = %q, want zh", cfg.Language)
	}
	if !reflect.DeepEqual(cfg.OutputFormats, wantFormats) {
		t.Fatalf("OutputFormats = %#v, want %#v", cfg.OutputFormats, wantFormats)
	}
	if !cfg.KeepIntermediateFiles {
		t.Fatal("KeepIntermediateFiles = false, want true")
	}
	if cfg.ModelDir != "/models" {
		t.Fatalf("ModelDir = %q, want /models", cfg.ModelDir)
	}
	if cfg.JobsDir != "/jobs" {
		t.Fatalf("JobsDir = %q, want /jobs", cfg.JobsDir)
	}
}

func TestLoadAppliesOverridesAfterConfigFile(t *testing.T) {
	paths := testPaths(t)
	writeConfig(t, paths.ConfigFile, `
model = "base"
language = "zh"
output_formats = ["txt"]
keep_intermediate_files = false
model_dir = "/models"
jobs_dir = "/jobs"
`)

	model := "small"
	language := "en"
	formats := []domain.OutputFormat{domain.OutputFormatJSON}
	keep := true

	cfg, err := Load(paths, Overrides{
		Model:                 &model,
		Language:              &language,
		OutputFormats:         &formats,
		KeepIntermediateFiles: &keep,
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Model != model {
		t.Fatalf("Model = %q, want %q", cfg.Model, model)
	}
	if cfg.Language != language {
		t.Fatalf("Language = %q, want %q", cfg.Language, language)
	}
	if !reflect.DeepEqual(cfg.OutputFormats, formats) {
		t.Fatalf("OutputFormats = %#v, want %#v", cfg.OutputFormats, formats)
	}
	if !cfg.KeepIntermediateFiles {
		t.Fatal("KeepIntermediateFiles = false, want true")
	}
}

func TestLoadRejectsUnsupportedOutputFormat(t *testing.T) {
	paths := testPaths(t)
	writeConfig(t, paths.ConfigFile, `output_formats = ["txt", "docx"]`)

	_, err := Load(paths, Overrides{})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadRejectsUnsupportedLanguage(t *testing.T) {
	paths := testPaths(t)
	writeConfig(t, paths.ConfigFile, `language = "fr"`)

	_, err := Load(paths, Overrides{})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func testPaths(t *testing.T) Paths {
	t.Helper()

	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	cacheDir := filepath.Join(root, "cache")

	return Paths{
		ConfigDir:  configDir,
		ConfigFile: filepath.Join(configDir, "config.toml"),
		CacheDir:   cacheDir,
		JobsDir:    filepath.Join(cacheDir, "jobs"),
		ModelsDir:  filepath.Join(cacheDir, "models"),
	}
}

func writeConfig(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
