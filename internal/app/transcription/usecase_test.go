package transcription

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"textdrain/internal/domain"
)

func TestUseCaseRunsEndToEndForLocalFile(t *testing.T) {
	tempDir := t.TempDir()
	mediaPath := filepath.Join(tempDir, "clip.mp4")
	audioPath := filepath.Join(tempDir, "jobs", "local-file-clip", "clip.wav")
	if err := os.WriteFile(mediaPath, []byte("media"), 0o644); err != nil {
		t.Fatalf("WriteFile(media) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(audioPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(audio dir) error = %v", err)
	}

	reporter := &recordingReporter{}
	exporter := &fakeExporter{paths: []string{filepath.Join(tempDir, "out", "clip.txt")}}
	uc := NewUseCase(Dependencies{
		Resolver: &fakeResolver{asset: domain.MediaAsset{
			SourceType: domain.SourceTypeLocalFile,
			RawInput:   mediaPath,
			Title:      "clip",
			Site:       "local",
			WorkDir:    filepath.Join(tempDir, "jobs", "local-file-clip"),
			MediaPath:  mediaPath,
			Metadata:   map[string]string{"filename": "clip.mp4"},
		}},
		Downloader:     &fakeDownloader{},
		AudioProcessor: &fakeAudioProcessor{audioPath: audioPath},
		ASREngine:      &fakeASR{},
		Exporter:       exporter,
		Reporter:       reporter,
	})

	result, err := uc.Run(context.Background(), Request{
		Input:            mediaPath,
		Language:         "en",
		Model:            "base",
		OutputDir:        filepath.Join(tempDir, "out"),
		Formats:          []domain.OutputFormat{domain.OutputFormatTXT},
		KeepIntermediate: true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantStatuses := []domain.JobStatus{
		domain.JobStatusPending,
		domain.JobStatusResolving,
		domain.JobStatusExtractingAudio,
		domain.JobStatusTranscribing,
		domain.JobStatusExporting,
		domain.JobStatusCompleted,
	}
	if !reflect.DeepEqual(reporter.statuses, wantStatuses) {
		t.Fatalf("statuses = %#v, want %#v", reporter.statuses, wantStatuses)
	}
	if result.JobID != "local-file-clip" {
		t.Fatalf("JobID = %q, want local-file-clip", result.JobID)
	}
	if exporter.transcript.Metadata["job_id"] != "local-file-clip" {
		t.Fatalf("export job_id = %q, want local-file-clip", exporter.transcript.Metadata["job_id"])
	}
	if exporter.transcript.Metadata["title"] != "clip" || exporter.transcript.Metadata["audio_path"] != audioPath {
		t.Fatalf("export metadata = %#v, want source and audio metadata", exporter.transcript.Metadata)
	}
	if _, err := os.Stat(audioPath); err != nil {
		t.Fatalf("audio file should be kept: %v", err)
	}
}

func TestUseCaseDownloadsURLAndCleansIntermediateFiles(t *testing.T) {
	tempDir := t.TempDir()
	workDir := filepath.Join(tempDir, "jobs", "url-video")
	downloadedPath := filepath.Join(workDir, "video.m4a")
	audioPath := filepath.Join(workDir, "video.wav")

	uc := NewUseCase(Dependencies{
		Resolver: &fakeResolver{asset: domain.MediaAsset{
			SourceType: domain.SourceTypeURL,
			RawInput:   "https://example.com/video",
			Title:      "video",
			Site:       "example.com",
			WorkDir:    workDir,
			Metadata:   map[string]string{"url": "https://example.com/video"},
		}},
		Downloader:     &fakeDownloader{mediaPath: downloadedPath},
		AudioProcessor: &fakeAudioProcessor{audioPath: audioPath},
		ASREngine:      &fakeASR{},
		Exporter:       &fakeExporter{paths: []string{filepath.Join(tempDir, "outputs", "video.txt")}},
	})

	_, err := uc.Run(context.Background(), Request{
		Input:            "https://example.com/video",
		Language:         "auto",
		Model:            "small",
		Formats:          []domain.OutputFormat{domain.OutputFormatTXT},
		KeepIntermediate: false,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for _, path := range []string{downloadedPath, audioPath} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s exists after cleanup, stat err = %v", path, err)
		}
	}
}

func TestUseCaseWrapsFailureWithStage(t *testing.T) {
	wantErr := errors.New("ffmpeg missing")
	reporter := &recordingReporter{}
	uc := NewUseCase(Dependencies{
		Resolver: &fakeResolver{asset: domain.MediaAsset{
			SourceType: domain.SourceTypeLocalFile,
			RawInput:   "clip.mp4",
			Title:      "clip",
			Site:       "local",
			WorkDir:    t.TempDir(),
			MediaPath:  "clip.mp4",
		}},
		Downloader:     &fakeDownloader{},
		AudioProcessor: &fakeAudioProcessor{err: wantErr},
		ASREngine:      &fakeASR{},
		Exporter:       &fakeExporter{},
		Reporter:       reporter,
	})

	_, err := uc.Run(context.Background(), Request{Input: "clip.mp4", Language: "en", Model: "base"})
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
	var stageErr *StageError
	if !errors.As(err, &stageErr) {
		t.Fatalf("Run() error = %T, want StageError", err)
	}
	if stageErr.Stage != domain.JobStatusExtractingAudio {
		t.Fatalf("Stage = %s, want %s", stageErr.Stage, domain.JobStatusExtractingAudio)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want wrapped ffmpeg error", err)
	}
	if got := reporter.statuses[len(reporter.statuses)-1]; got != domain.JobStatusFailed {
		t.Fatalf("last status = %s, want FAILED", got)
	}
}

type recordingReporter struct {
	statuses []domain.JobStatus
}

func (r *recordingReporter) Update(_ context.Context, status domain.JobStatus) error {
	r.statuses = append(r.statuses, status)
	return nil
}

type fakeResolver struct {
	asset domain.MediaAsset
	err   error
}

func (r *fakeResolver) Resolve(_ context.Context, _ string) (domain.MediaAsset, error) {
	if r.err != nil {
		return domain.MediaAsset{}, r.err
	}
	return r.asset, nil
}

type fakeDownloader struct {
	mediaPath string
	err       error
}

func (d *fakeDownloader) Fetch(_ context.Context, asset domain.MediaAsset, _ string) (domain.DownloadResult, error) {
	if d.err != nil {
		return domain.DownloadResult{}, d.err
	}
	if err := os.MkdirAll(filepath.Dir(d.mediaPath), 0o755); err != nil {
		return domain.DownloadResult{}, err
	}
	if d.mediaPath != "" {
		if err := os.WriteFile(d.mediaPath, []byte("download"), 0o644); err != nil {
			return domain.DownloadResult{}, err
		}
	}
	asset.MediaPath = d.mediaPath
	return domain.DownloadResult{Asset: asset, MediaPath: d.mediaPath}, nil
}

type fakeAudioProcessor struct {
	audioPath string
	err       error
}

func (p *fakeAudioProcessor) Prepare(_ context.Context, mediaPath string, _ string, _ domain.AudioOptions) (domain.PreparedAudio, error) {
	if p.err != nil {
		return domain.PreparedAudio{}, p.err
	}
	if err := os.MkdirAll(filepath.Dir(p.audioPath), 0o755); err != nil {
		return domain.PreparedAudio{}, err
	}
	if err := os.WriteFile(p.audioPath, []byte("audio"), 0o644); err != nil {
		return domain.PreparedAudio{}, err
	}
	return domain.PreparedAudio{
		SourcePath:   mediaPath,
		Path:         p.audioPath,
		SampleRateHz: 16000,
		Channels:     1,
		Codec:        "pcm_s16le",
	}, nil
}

type fakeASR struct {
	err error
}

func (e *fakeASR) Transcribe(_ context.Context, _ string, opts domain.TranscribeOptions) (domain.Transcript, error) {
	if e.err != nil {
		return domain.Transcript{}, e.err
	}
	return domain.Transcript{
		Language: opts.Language,
		Text:     "hello",
		Segments: []domain.TranscriptSegment{
			{Index: 0, StartMs: 0, EndMs: 1000, Text: "hello"},
		},
		Engine: domain.TranscriptEngine{
			Name:         "fake",
			ModelName:    opts.ModelName,
			LanguageMode: opts.Language,
		},
		Metadata: map[string]string{"asr": "fake"},
	}, nil
}

type fakeExporter struct {
	paths      []string
	transcript domain.Transcript
	err        error
}

func (e *fakeExporter) Export(_ context.Context, transcript domain.Transcript, _ string, _ []domain.OutputFormat) ([]string, error) {
	e.transcript = transcript
	if e.err != nil {
		return nil, e.err
	}
	return e.paths, nil
}
