package models

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverFindsSupportedModelFiles(t *testing.T) {
	modelDir := t.TempDir()
	writeFile(t, filepath.Join(modelDir, "small.gguf"), "small-model")
	writeFile(t, filepath.Join(modelDir, "ggml-base.bin"), "base")
	writeFile(t, filepath.Join(modelDir, "readme.txt"), "ignore")
	if err := os.Mkdir(filepath.Join(modelDir, "ct2-model"), 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	discovered, err := Discover(modelDir, "small")
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(discovered) != 2 {
		t.Fatalf("Discover() len = %d, want 2: %#v", len(discovered), discovered)
	}
	if discovered[0].Name != "ggml-base.bin" || discovered[0].Default {
		t.Fatalf("discovered[0] = %#v, want non-default base model", discovered[0])
	}
	if discovered[1].Name != "small.gguf" || !discovered[1].Default {
		t.Fatalf("discovered[1] = %#v, want default GGUF model", discovered[1])
	}
	if discovered[1].Path != filepath.Join(modelDir, "small.gguf") {
		t.Fatalf("Path = %q, want cleaned model path", discovered[1].Path)
	}
	if discovered[1].SizeBytes != int64(len("small-model")) {
		t.Fatalf("SizeBytes = %d, want %d", discovered[1].SizeBytes, len("small-model"))
	}
}

func TestDiscoverMissingDirectoryReturnsEmptyList(t *testing.T) {
	discovered, err := Discover(filepath.Join(t.TempDir(), "missing"), "small")
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(discovered) != 0 {
		t.Fatalf("Discover() len = %d, want 0", len(discovered))
	}
}

func TestCandidatePathsIncludesGGUFAndLegacyBinNames(t *testing.T) {
	modelDir := "/models"
	got := CandidatePaths(modelDir, "small")
	want := []string{
		filepath.Join(modelDir, "small"),
		filepath.Join(modelDir, "small.gguf"),
		filepath.Join(modelDir, "small.bin"),
		filepath.Join(modelDir, "ggml-small.gguf"),
		filepath.Join(modelDir, "ggml-small.bin"),
		filepath.Join(modelDir, "ggml-small.q5_0.gguf"),
		filepath.Join(modelDir, "ggml-small.q5_0.bin"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CandidatePaths() = %#v, want %#v", got, want)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
