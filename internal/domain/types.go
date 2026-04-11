package domain

import "time"

// SourceType identifies how the input media is obtained.
type SourceType string

const (
	SourceTypeLocalFile SourceType = "local_file"
	SourceTypeURL       SourceType = "url"
)

// OutputFormat is a supported transcript export format.
type OutputFormat string

const (
	OutputFormatTXT  OutputFormat = "txt"
	OutputFormatSRT  OutputFormat = "srt"
	OutputFormatVTT  OutputFormat = "vtt"
	OutputFormatJSON OutputFormat = "json"
)

// MediaAsset is the normalized representation of either a local file or a URL source.
type MediaAsset struct {
	SourceType   SourceType
	RawInput     string
	Title        string
	Site         string
	WorkDir      string
	MediaPath    string
	Duration     time.Duration
	LanguageHint string
	Metadata     map[string]string
}

// DownloadResult describes the concrete local media file produced by a downloader.
type DownloadResult struct {
	Asset       MediaAsset
	MediaPath   string
	OriginalURL string
	Title       string
	Site        string
	Duration    time.Duration
	Metadata    map[string]string
}

// PreparedAudio describes the normalized audio file ready for ASR.
type PreparedAudio struct {
	SourcePath   string
	Path         string
	SampleRateHz int
	Channels     int
	Codec        string
	Duration     time.Duration
}

// Transcript is the ASR output consumed by exporters.
type Transcript struct {
	Language string
	Text     string
	Segments []TranscriptSegment
	Engine   TranscriptEngine
	Metadata map[string]string
}

// TranscriptEngine records ASR engine metadata useful for diagnostics and JSON export.
type TranscriptEngine struct {
	Name         string
	ModelName    string
	ModelPath    string
	LanguageMode string
}

// TranscriptSegment is a time-bounded transcript fragment.
type TranscriptSegment struct {
	Index      int
	StartMs    int64
	EndMs      int64
	Text       string
	Confidence *float64
}

// AudioOptions configures local audio normalization.
type AudioOptions struct {
	SampleRateHz      int
	Channels          int
	Codec             string
	LoudnessNormalize bool
	TrimSilence       bool
}

// TranscribeOptions configures ASR execution.
type TranscribeOptions struct {
	ModelName string
	ModelPath string
	Language  string
	WorkDir   string
	Threads   int
}
