package domain

// JobStatus is the current stage of a transcription job.
type JobStatus string

const (
	JobStatusPending         JobStatus = "PENDING"
	JobStatusResolving       JobStatus = "RESOLVING"
	JobStatusDownloading     JobStatus = "DOWNLOADING"
	JobStatusExtractingAudio JobStatus = "EXTRACTING_AUDIO"
	JobStatusTranscribing    JobStatus = "TRANSCRIBING"
	JobStatusExporting       JobStatus = "EXPORTING"
	JobStatusCompleted       JobStatus = "COMPLETED"
	JobStatusFailed          JobStatus = "FAILED"
)
