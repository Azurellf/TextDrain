package transcription

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"textdrain/internal/domain"
)

// StatusReporter receives job status updates as the transcription pipeline advances.
type StatusReporter interface {
	Update(ctx context.Context, status domain.JobStatus) error
}

// Request contains the user-facing options for a transcription job.
type Request struct {
	Input            string
	Language         string
	Model            string
	OutputDir        string
	Formats          []domain.OutputFormat
	KeepIntermediate bool
}

// Result describes the artifacts produced by a transcription job.
type Result struct {
	JobID       string
	WorkDir     string
	OutputPaths []string
	Asset       domain.MediaAsset
	Audio       domain.PreparedAudio
	Transcript  domain.Transcript
}

// StageError wraps a failure with the pipeline stage where it occurred.
type StageError struct {
	Stage domain.JobStatus
	Err   error
}

func (e *StageError) Error() string {
	return fmt.Sprintf("stage %s: %v", e.Stage, e.Err)
}

func (e *StageError) Unwrap() error {
	return e.Err
}

// UseCase orchestrates source resolution, download, audio preparation, ASR, export, and cleanup.
type UseCase struct {
	resolver       domain.SourceResolver
	downloader     domain.Downloader
	audioProcessor domain.AudioProcessor
	asrEngine      domain.ASREngine
	exporter       domain.Exporter
	reporter       StatusReporter
}

// Dependencies are the infrastructure adapters required by the use case.
type Dependencies struct {
	Resolver       domain.SourceResolver
	Downloader     domain.Downloader
	AudioProcessor domain.AudioProcessor
	ASREngine      domain.ASREngine
	Exporter       domain.Exporter
	Reporter       StatusReporter
}

func NewUseCase(deps Dependencies) *UseCase {
	return &UseCase{
		resolver:       deps.Resolver,
		downloader:     deps.Downloader,
		audioProcessor: deps.AudioProcessor,
		asrEngine:      deps.ASREngine,
		exporter:       deps.Exporter,
		reporter:       deps.Reporter,
	}
}

func (uc *UseCase) Run(ctx context.Context, req Request) (Result, error) {
	var result Result

	if err := uc.validate(); err != nil {
		return result, err
	}
	if err := uc.report(ctx, domain.JobStatusPending); err != nil {
		return result, err
	}

	asset, err := uc.resolve(ctx, req.Input)
	if err != nil {
		return result, err
	}
	result.Asset = asset
	result.WorkDir = asset.WorkDir
	result.JobID = jobIDFromWorkDir(asset.WorkDir)

	if err := os.MkdirAll(asset.WorkDir, 0o755); err != nil {
		return result, uc.fail(ctx, domain.JobStatusPending, fmt.Errorf("create job workdir %s: %w", asset.WorkDir, err))
	}

	mediaPath := asset.MediaPath
	if asset.SourceType == domain.SourceTypeURL {
		download, err := uc.download(ctx, asset)
		if err != nil {
			return result, err
		}
		asset = download.Asset
		mediaPath = download.MediaPath
		result.Asset = asset
	}

	audio, err := uc.prepareAudio(ctx, mediaPath, asset.WorkDir)
	if err != nil {
		return result, err
	}
	result.Audio = audio

	transcript, err := uc.transcribe(ctx, audio.Path, asset.WorkDir, req)
	if err != nil {
		return result, err
	}
	transcript.Metadata = transcriptMetadata(transcript.Metadata, asset, audio, result.JobID)
	result.Transcript = transcript

	outputPaths, err := uc.export(ctx, transcript, req)
	if err != nil {
		return result, err
	}
	result.OutputPaths = outputPaths

	if !req.KeepIntermediate {
		if err := cleanupIntermediate(asset, audio); err != nil {
			return result, uc.fail(ctx, domain.JobStatusExporting, err)
		}
	}

	if err := uc.report(ctx, domain.JobStatusCompleted); err != nil {
		return result, err
	}

	return result, nil
}

func (uc *UseCase) validate() error {
	switch {
	case uc.resolver == nil:
		return fmt.Errorf("transcribe use case resolver is nil")
	case uc.downloader == nil:
		return fmt.Errorf("transcribe use case downloader is nil")
	case uc.audioProcessor == nil:
		return fmt.Errorf("transcribe use case audio processor is nil")
	case uc.asrEngine == nil:
		return fmt.Errorf("transcribe use case asr engine is nil")
	case uc.exporter == nil:
		return fmt.Errorf("transcribe use case exporter is nil")
	default:
		return nil
	}
}

func (uc *UseCase) resolve(ctx context.Context, input string) (domain.MediaAsset, error) {
	if err := uc.report(ctx, domain.JobStatusResolving); err != nil {
		return domain.MediaAsset{}, err
	}
	asset, err := uc.resolver.Resolve(ctx, input)
	if err != nil {
		return domain.MediaAsset{}, uc.fail(ctx, domain.JobStatusResolving, err)
	}
	return asset, nil
}

func (uc *UseCase) download(ctx context.Context, asset domain.MediaAsset) (domain.DownloadResult, error) {
	if err := uc.report(ctx, domain.JobStatusDownloading); err != nil {
		return domain.DownloadResult{}, err
	}
	download, err := uc.downloader.Fetch(ctx, asset, asset.WorkDir)
	if err != nil {
		return domain.DownloadResult{}, uc.fail(ctx, domain.JobStatusDownloading, err)
	}
	return download, nil
}

func (uc *UseCase) prepareAudio(ctx context.Context, mediaPath string, workdir string) (domain.PreparedAudio, error) {
	if err := uc.report(ctx, domain.JobStatusExtractingAudio); err != nil {
		return domain.PreparedAudio{}, err
	}
	audio, err := uc.audioProcessor.Prepare(ctx, mediaPath, workdir, domain.AudioOptions{})
	if err != nil {
		return domain.PreparedAudio{}, uc.fail(ctx, domain.JobStatusExtractingAudio, err)
	}
	return audio, nil
}

func (uc *UseCase) transcribe(ctx context.Context, audioPath string, workdir string, req Request) (domain.Transcript, error) {
	if err := uc.report(ctx, domain.JobStatusTranscribing); err != nil {
		return domain.Transcript{}, err
	}
	transcript, err := uc.asrEngine.Transcribe(ctx, audioPath, domain.TranscribeOptions{
		ModelName: req.Model,
		Language:  req.Language,
		WorkDir:   workdir,
	})
	if err != nil {
		return domain.Transcript{}, uc.fail(ctx, domain.JobStatusTranscribing, err)
	}
	return transcript, nil
}

func (uc *UseCase) export(ctx context.Context, transcript domain.Transcript, req Request) ([]string, error) {
	if err := uc.report(ctx, domain.JobStatusExporting); err != nil {
		return nil, err
	}
	outputPaths, err := uc.exporter.Export(ctx, transcript, req.OutputDir, req.Formats)
	if err != nil {
		return nil, uc.fail(ctx, domain.JobStatusExporting, err)
	}
	return outputPaths, nil
}

func (uc *UseCase) report(ctx context.Context, status domain.JobStatus) error {
	if uc.reporter == nil {
		return nil
	}
	if err := uc.reporter.Update(ctx, status); err != nil {
		return fmt.Errorf("report status %s: %w", status, err)
	}
	return nil
}

func (uc *UseCase) fail(ctx context.Context, stage domain.JobStatus, err error) error {
	_ = uc.report(ctx, domain.JobStatusFailed)
	return &StageError{Stage: stage, Err: err}
}

func transcriptMetadata(existing map[string]string, asset domain.MediaAsset, audio domain.PreparedAudio, jobID string) map[string]string {
	metadata := make(map[string]string, len(existing)+len(asset.Metadata)+10)
	for key, value := range existing {
		metadata[key] = value
	}
	for key, value := range asset.Metadata {
		metadata[key] = value
	}
	metadata["job_id"] = jobID
	metadata["source_type"] = string(asset.SourceType)
	metadata["input"] = asset.RawInput
	metadata["title"] = asset.Title
	metadata["site"] = asset.Site
	metadata["media_path"] = asset.MediaPath
	metadata["audio_path"] = audio.Path
	metadata["work_dir"] = asset.WorkDir
	if audio.SampleRateHz > 0 {
		metadata["audio_sample_rate_hz"] = fmt.Sprintf("%d", audio.SampleRateHz)
	}
	if audio.Channels > 0 {
		metadata["audio_channels"] = fmt.Sprintf("%d", audio.Channels)
	}
	if audio.Codec != "" {
		metadata["audio_codec"] = audio.Codec
	}
	return metadata
}

func cleanupIntermediate(asset domain.MediaAsset, audio domain.PreparedAudio) error {
	var errs []error
	if audio.Path != "" {
		if err := removeFileIfExists(audio.Path); err != nil {
			errs = append(errs, err)
		}
	}
	if asset.SourceType == domain.SourceTypeURL && asset.MediaPath != "" {
		if err := removeFileIfExists(asset.MediaPath); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("cleanup intermediate files: %w", errs[0])
	}
	return nil
}

func removeFileIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func jobIDFromWorkDir(workdir string) string {
	jobID := strings.TrimSpace(filepath.Base(workdir))
	if jobID == "." || jobID == string(filepath.Separator) || jobID == "" {
		return "job"
	}
	return jobID
}
