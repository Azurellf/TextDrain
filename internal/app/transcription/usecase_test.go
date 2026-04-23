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
			JobID:      "job-001",
			SourceType: domain.SourceTypeLocalFile,
			RawInput:   mediaPath,
			Title:      "clip",
			Site:       "local",
			WorkDir:    filepath.Join(tempDir, "jobs", "local-file-clip"),
			MediaPath:  mediaPath,
			Metadata:   map[string]string{"filename": "clip.mp4"},
		}},
		URLInspector:   &fakeURLInspector{},
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
	if result.JobID != "job-001" {
		t.Fatalf("JobID = %q, want job-001", result.JobID)
	}
	if exporter.transcript.Metadata["job_id"] != "job-001" {
		t.Fatalf("export job_id = %q, want job-001", exporter.transcript.Metadata["job_id"])
	}
	if exporter.transcript.Metadata["title"] != "clip" || exporter.transcript.Metadata["audio_path"] != audioPath {
		t.Fatalf("export metadata = %#v, want source and audio metadata", exporter.transcript.Metadata)
	}
	if _, err := os.Stat(audioPath); err != nil {
		t.Fatalf("audio file should be kept: %v", err)
	}
}

func TestUseCaseRunsEndToEndForLocalAudioInput(t *testing.T) {
	tempDir := t.TempDir()
	mediaPath := filepath.Join(tempDir, "voice.wav")
	audioPath := filepath.Join(tempDir, "jobs", "local-audio", "voice.wav")
	if err := os.WriteFile(mediaPath, []byte("audio"), 0o644); err != nil {
		t.Fatalf("WriteFile(media) error = %v", err)
	}

	audioProcessor := &fakeAudioProcessor{audioPath: audioPath}
	uc := NewUseCase(Dependencies{
		Resolver: &fakeResolver{asset: domain.MediaAsset{
			JobID:      "local-audio",
			SourceType: domain.SourceTypeLocalFile,
			RawInput:   mediaPath,
			Title:      "voice",
			Site:       "local",
			WorkDir:    filepath.Dir(audioPath),
			MediaPath:  mediaPath,
			Metadata:   map[string]string{"filename": "voice.wav"},
		}},
		URLInspector:   &fakeURLInspector{},
		Downloader:     &fakeDownloader{},
		AudioProcessor: audioProcessor,
		ASREngine:      &fakeASR{},
		Exporter:       &fakeExporter{paths: []string{filepath.Join(tempDir, "out", "voice.txt")}},
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

	if result.Asset.SourceType != domain.SourceTypeLocalFile || result.Audio.SourcePath != mediaPath {
		t.Fatalf("result asset/audio = %#v/%#v, want local audio path preserved", result.Asset, result.Audio)
	}
	if audioProcessor.mediaPath != mediaPath {
		t.Fatalf("audio processor mediaPath = %q, want %q", audioProcessor.mediaPath, mediaPath)
	}
}

func TestUseCaseRunsEndToEndForLocalVideoInput(t *testing.T) {
	tempDir := t.TempDir()
	mediaPath := filepath.Join(tempDir, "meeting.mp4")
	audioPath := filepath.Join(tempDir, "jobs", "local-video", "meeting.wav")
	if err := os.WriteFile(mediaPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("WriteFile(media) error = %v", err)
	}

	audioProcessor := &fakeAudioProcessor{audioPath: audioPath}
	uc := NewUseCase(Dependencies{
		Resolver: &fakeResolver{asset: domain.MediaAsset{
			JobID:      "local-video",
			SourceType: domain.SourceTypeLocalFile,
			RawInput:   mediaPath,
			Title:      "meeting",
			Site:       "local",
			WorkDir:    filepath.Dir(audioPath),
			MediaPath:  mediaPath,
			Metadata:   map[string]string{"filename": "meeting.mp4"},
		}},
		URLInspector:   &fakeURLInspector{},
		Downloader:     &fakeDownloader{},
		AudioProcessor: audioProcessor,
		ASREngine:      &fakeASR{},
		Exporter:       &fakeExporter{paths: []string{filepath.Join(tempDir, "out", "meeting.txt")}},
	})

	result, err := uc.Run(context.Background(), Request{
		Input:            mediaPath,
		Language:         "zh",
		Model:            "small",
		OutputDir:        filepath.Join(tempDir, "out"),
		Formats:          []domain.OutputFormat{domain.OutputFormatSRT},
		KeepIntermediate: true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Asset.Title != "meeting" || result.Audio.Codec != "pcm_s16le" {
		t.Fatalf("result = %#v, want local video converted to prepared audio", result)
	}
	if audioProcessor.mediaPath != mediaPath {
		t.Fatalf("audio processor mediaPath = %q, want %q", audioProcessor.mediaPath, mediaPath)
	}
}

func TestUseCaseDownloadsURLAndCleansIntermediateFiles(t *testing.T) {
	tempDir := t.TempDir()
	workDir := filepath.Join(tempDir, "jobs", "example-com-abc123")
	downloadedPath := filepath.Join(workDir, "video.m4a")
	audioPath := filepath.Join(workDir, "video.wav")

	uc := NewUseCase(Dependencies{
		Resolver: &fakeResolver{asset: domain.MediaAsset{
			JobID:      "job-url-001",
			SourceType: domain.SourceTypeURL,
			RawInput:   "https://example.com/video",
			Title:      "video",
			Site:       "example.com",
			WorkDir:    filepath.Join(tempDir, "jobs", "url-video"),
			Metadata:   map[string]string{"url": "https://example.com/video"},
		}},
		URLInspector:   &fakeURLInspector{metadata: map[string]string{"id": "abc123", "title": "Video"}},
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
	if _, err := os.Stat(workDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("empty workdir exists after cleanup, stat err = %v", err)
	}
}

func TestUseCaseUsesInspectedURLSiteAndIDForJobPath(t *testing.T) {
	tempDir := t.TempDir()
	resolvedWorkDir := filepath.Join(tempDir, "jobs", "url-watch-random")
	downloadedPath := filepath.Join(tempDir, "jobs", "youtube-QsZFBqtgI8A", "video.m4a")
	audioPath := filepath.Join(tempDir, "jobs", "youtube-QsZFBqtgI8A", "video.wav")

	exporter := &fakeExporter{paths: []string{filepath.Join(tempDir, "outputs", "video.txt")}}
	uc := NewUseCase(Dependencies{
		Resolver: &fakeResolver{asset: domain.MediaAsset{
			JobID:      "url-watch-random",
			SourceType: domain.SourceTypeURL,
			RawInput:   "https://www.youtube.com/watch?v=QsZFBqtgI8A",
			Title:      "watch",
			Site:       "www.youtube.com",
			WorkDir:    resolvedWorkDir,
			Metadata:   map[string]string{"url": "https://www.youtube.com/watch?v=QsZFBqtgI8A"},
		}},
		URLInspector: &fakeURLInspector{metadata: map[string]string{
			"id":            "QsZFBqtgI8A",
			"title":         "Bad / Title: Episode 1",
			"extractor_key": "Youtube",
		}},
		Downloader:     &fakeDownloader{mediaPath: downloadedPath},
		AudioProcessor: &fakeAudioProcessor{audioPath: audioPath},
		ASREngine:      &fakeASR{},
		Exporter:       exporter,
	})

	result, err := uc.Run(context.Background(), Request{
		Input:            "https://www.youtube.com/watch?v=QsZFBqtgI8A",
		Language:         "auto",
		Model:            "small",
		Formats:          []domain.OutputFormat{domain.OutputFormatTXT},
		KeepIntermediate: true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantJobID := "youtube-QsZFBqtgI8A"
	if result.JobID != wantJobID {
		t.Fatalf("JobID = %q, want %q", result.JobID, wantJobID)
	}
	if result.WorkDir != filepath.Join(tempDir, "jobs", wantJobID) {
		t.Fatalf("WorkDir = %q, want stable site/id directory", result.WorkDir)
	}
	if exporter.transcript.Metadata["work_dir"] != result.WorkDir {
		t.Fatalf("export work_dir = %q, want %q", exporter.transcript.Metadata["work_dir"], result.WorkDir)
	}
}

func TestUseCasePreservesBilibiliIDCaseInURLJobPath(t *testing.T) {
	result := runURLNamingCase(t, urlNamingCase{
		title:  "Bilibili Clip",
		id:     "BV1GUoNB5E1X",
		rawURL: "https://www.bilibili.com/video/BV1GUoNB5E1X/",
		site:   "www.bilibili.com",
	})

	if result.JobID != "bilibili-BV1GUoNB5E1X" {
		t.Fatalf("JobID = %q, want Bilibili ID casing preserved", result.JobID)
	}
}

func TestUseCaseOmitsTitleFromURLJobPath(t *testing.T) {
	result := runURLNamingCase(t, urlNamingCase{
		title:        "中文 标题：第一集",
		id:           "BV1GUoNB5E1X",
		rawURL:       "https://www.bilibili.com/video/BV1GUoNB5E1X/",
		site:         "www.bilibili.com",
		extractorKey: "BiliBili",
	})

	if result.JobID != "bilibili-BV1GUoNB5E1X" {
		t.Fatalf("JobID = %q, want URL title omitted", result.JobID)
	}
}

func TestUseCaseFallsBackToURLHashWhenMetadataIDIsMissing(t *testing.T) {
	rawURL := "https://example.com/watch?v=missing-id"
	result := runURLNamingCase(t, urlNamingCase{
		title:  "Fallback Clip",
		rawURL: rawURL,
		site:   "example.com",
	})

	wantJobID := "example-com-url-" + shortHash(rawURL)
	if result.JobID != wantJobID {
		t.Fatalf("JobID = %q, want %q", result.JobID, wantJobID)
	}
}

type urlNamingCase struct {
	title        string
	id           string
	rawURL       string
	site         string
	extractor    string
	extractorKey string
}

func runURLNamingCase(t *testing.T, tt urlNamingCase) Result {
	t.Helper()

	tempDir := t.TempDir()
	workDir := filepath.Join(tempDir, "jobs", "url-preliminary")
	metadata := map[string]string{"title": tt.title}
	if tt.id != "" {
		metadata["id"] = tt.id
	}
	if tt.extractor != "" {
		metadata["extractor"] = tt.extractor
	}
	if tt.extractorKey != "" {
		metadata["extractor_key"] = tt.extractorKey
	}

	uc := NewUseCase(Dependencies{
		Resolver: &fakeResolver{asset: domain.MediaAsset{
			JobID:      "url-preliminary",
			SourceType: domain.SourceTypeURL,
			RawInput:   tt.rawURL,
			Title:      "preliminary",
			Site:       tt.site,
			WorkDir:    workDir,
			Metadata:   map[string]string{"url": tt.rawURL},
		}},
		URLInspector:   &fakeURLInspector{metadata: metadata},
		Downloader:     &fakeDownloader{mediaPath: filepath.Join(tempDir, "downloaded.m4a")},
		AudioProcessor: &fakeAudioProcessor{audioPath: filepath.Join(tempDir, "audio.wav")},
		ASREngine:      &fakeASR{},
		Exporter:       &fakeExporter{paths: []string{filepath.Join(tempDir, "out.txt")}},
	})

	result, err := uc.Run(context.Background(), Request{
		Input:            tt.rawURL,
		Language:         "auto",
		Model:            "small",
		Formats:          []domain.OutputFormat{domain.OutputFormatTXT},
		KeepIntermediate: true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	return result
}

func TestUseCaseReportsMissingDependency(t *testing.T) {
	wantErr := errors.New("yt-dlp executable not found")
	reporter := &recordingReporter{}
	uc := NewUseCase(Dependencies{
		Resolver: &fakeResolver{asset: domain.MediaAsset{
			JobID:      "url-video",
			SourceType: domain.SourceTypeURL,
			RawInput:   "https://example.com/video",
			Title:      "video",
			Site:       "example.com",
			WorkDir:    t.TempDir(),
			Metadata:   map[string]string{"url": "https://example.com/video"},
		}},
		URLInspector:   &fakeURLInspector{},
		Downloader:     &fakeDownloader{err: wantErr},
		AudioProcessor: &fakeAudioProcessor{},
		ASREngine:      &fakeASR{},
		Exporter:       &fakeExporter{},
		Reporter:       reporter,
	})

	_, err := uc.Run(context.Background(), Request{Input: "https://example.com/video", Language: "auto", Model: "small"})
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
	var stageErr *StageError
	if !errors.As(err, &stageErr) {
		t.Fatalf("Run() error = %T, want StageError", err)
	}
	if stageErr.Stage != domain.JobStatusDownloading {
		t.Fatalf("Stage = %s, want %s", stageErr.Stage, domain.JobStatusDownloading)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want wrapped dependency error", err)
	}
	if got := reporter.statuses[len(reporter.statuses)-1]; got != domain.JobStatusFailed {
		t.Fatalf("last status = %s, want FAILED", got)
	}
}

func TestUseCaseReportsMissingModel(t *testing.T) {
	wantErr := errors.New("model small is missing")
	reporter := &recordingReporter{}
	uc := NewUseCase(Dependencies{
		Resolver: &fakeResolver{asset: domain.MediaAsset{
			JobID:      "local-audio",
			SourceType: domain.SourceTypeLocalFile,
			RawInput:   "clip.wav",
			Title:      "clip",
			Site:       "local",
			WorkDir:    t.TempDir(),
			MediaPath:  "clip.wav",
		}},
		URLInspector:   &fakeURLInspector{},
		Downloader:     &fakeDownloader{},
		AudioProcessor: &fakeAudioProcessor{audioPath: filepath.Join(t.TempDir(), "clip.wav")},
		ASREngine:      &fakeASR{err: wantErr},
		Exporter:       &fakeExporter{},
		Reporter:       reporter,
	})

	_, err := uc.Run(context.Background(), Request{Input: "clip.wav", Language: "en", Model: "small"})
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
	var stageErr *StageError
	if !errors.As(err, &stageErr) {
		t.Fatalf("Run() error = %T, want StageError", err)
	}
	if stageErr.Stage != domain.JobStatusTranscribing {
		t.Fatalf("Stage = %s, want %s", stageErr.Stage, domain.JobStatusTranscribing)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want wrapped model error", err)
	}
	if got := reporter.statuses[len(reporter.statuses)-1]; got != domain.JobStatusFailed {
		t.Fatalf("last status = %s, want FAILED", got)
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
		URLInspector:   &fakeURLInspector{},
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

type fakeURLInspector struct {
	metadata map[string]string
	err      error
}

func (i *fakeURLInspector) Inspect(_ context.Context, asset domain.MediaAsset) (domain.MediaAsset, error) {
	if i.err != nil {
		return domain.MediaAsset{}, i.err
	}
	if len(i.metadata) == 0 {
		return asset, nil
	}
	merged := make(map[string]string, len(asset.Metadata)+len(i.metadata))
	for key, value := range asset.Metadata {
		merged[key] = value
	}
	for key, value := range i.metadata {
		merged[key] = value
	}
	asset.Metadata = merged
	if title := i.metadata["title"]; title != "" {
		asset.Title = title
	}
	return asset, nil
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
	mediaPath string
	err       error
}

func (p *fakeAudioProcessor) Prepare(_ context.Context, mediaPath string, _ string, _ domain.AudioOptions) (domain.PreparedAudio, error) {
	if p.err != nil {
		return domain.PreparedAudio{}, p.err
	}
	p.mediaPath = mediaPath
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
