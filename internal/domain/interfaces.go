package domain

import "context"

// SourceResolver normalizes user input into a MediaAsset.
type SourceResolver interface {
	Resolve(ctx context.Context, input string) (MediaAsset, error)
}

// URLInspector reads URL metadata before a final job directory is created.
type URLInspector interface {
	Inspect(ctx context.Context, asset MediaAsset) (MediaAsset, error)
}

// Downloader fetches URL-backed media and returns a local media file.
type Downloader interface {
	Fetch(ctx context.Context, asset MediaAsset, workdir string) (DownloadResult, error)
}

// AudioProcessor converts media into normalized audio for ASR.
type AudioProcessor interface {
	Prepare(ctx context.Context, mediaPath string, workdir string, opts AudioOptions) (PreparedAudio, error)
}

// ASREngine transcribes normalized audio into a structured transcript.
type ASREngine interface {
	Transcribe(ctx context.Context, audioPath string, opts TranscribeOptions) (Transcript, error)
}

// Exporter writes transcript artifacts and returns their paths.
type Exporter interface {
	Export(ctx context.Context, transcript Transcript, outputDir string, formats []OutputFormat) ([]string, error)
}
