package domain

import (
	"testing"
	"time"
)

func TestDomainConstantsUseStableWireValues(t *testing.T) {
	tests := map[string]string{
		"SourceTypeLocalFile":      string(SourceTypeLocalFile),
		"SourceTypeURL":            string(SourceTypeURL),
		"OutputFormatTXT":          string(OutputFormatTXT),
		"OutputFormatSRT":          string(OutputFormatSRT),
		"OutputFormatVTT":          string(OutputFormatVTT),
		"OutputFormatJSON":         string(OutputFormatJSON),
		"JobStatusPending":         string(JobStatusPending),
		"JobStatusResolving":       string(JobStatusResolving),
		"JobStatusDownloading":     string(JobStatusDownloading),
		"JobStatusExtractingAudio": string(JobStatusExtractingAudio),
		"JobStatusTranscribing":    string(JobStatusTranscribing),
		"JobStatusExporting":       string(JobStatusExporting),
		"JobStatusCompleted":       string(JobStatusCompleted),
		"JobStatusFailed":          string(JobStatusFailed),
	}

	want := map[string]string{
		"SourceTypeLocalFile":      "local_file",
		"SourceTypeURL":            "url",
		"OutputFormatTXT":          "txt",
		"OutputFormatSRT":          "srt",
		"OutputFormatVTT":          "vtt",
		"OutputFormatJSON":         "json",
		"JobStatusPending":         "PENDING",
		"JobStatusResolving":       "RESOLVING",
		"JobStatusDownloading":     "DOWNLOADING",
		"JobStatusExtractingAudio": "EXTRACTING_AUDIO",
		"JobStatusTranscribing":    "TRANSCRIBING",
		"JobStatusExporting":       "EXPORTING",
		"JobStatusCompleted":       "COMPLETED",
		"JobStatusFailed":          "FAILED",
	}

	for name, got := range tests {
		if got != want[name] {
			t.Fatalf("%s = %q, want %q", name, got, want[name])
		}
	}
}

func TestTranscriptKeepsSegmentAndEngineMetadata(t *testing.T) {
	confidence := 0.93
	transcript := Transcript{
		Language: "en",
		Text:     "hello",
		Segments: []TranscriptSegment{
			{Index: 0, StartMs: 120, EndMs: 980, Text: "hello", Confidence: &confidence},
		},
		Engine: TranscriptEngine{
			Name:         "whisper.cpp",
			ModelName:    "small",
			ModelPath:    "/models/ggml-small.bin",
			LanguageMode: "auto",
		},
		Metadata: map[string]string{"job_id": "job-001"},
	}

	if transcript.Segments[0].Confidence == nil || *transcript.Segments[0].Confidence != confidence {
		t.Fatalf("Confidence = %#v, want %.2f", transcript.Segments[0].Confidence, confidence)
	}
	if transcript.Engine.ModelName != "small" || transcript.Metadata["job_id"] != "job-001" {
		t.Fatalf("transcript = %#v, want engine and metadata values preserved", transcript)
	}
}

func TestMediaAndAudioTypesPreserveNormalizedPathsAndDuration(t *testing.T) {
	asset := MediaAsset{
		JobID:      "job-001",
		SourceType: SourceTypeURL,
		RawInput:   "https://example.com/watch",
		Title:      "watch",
		Site:       "example.com",
		WorkDir:    "/tmp/textdrain/job-001",
		MediaPath:  "/tmp/textdrain/job-001/watch.m4a",
		Duration:   12500 * time.Millisecond,
		Metadata:   map[string]string{"url": "https://example.com/watch"},
	}
	audio := PreparedAudio{
		SourcePath:   asset.MediaPath,
		Path:         "/tmp/textdrain/job-001/watch.wav",
		SampleRateHz: 16000,
		Channels:     1,
		Codec:        "pcm_s16le",
		Duration:     asset.Duration,
	}

	if audio.SourcePath != asset.MediaPath {
		t.Fatalf("SourcePath = %q, want asset media path", audio.SourcePath)
	}
	if audio.Duration != 12500*time.Millisecond {
		t.Fatalf("Duration = %s, want 12.5s", audio.Duration)
	}
}
