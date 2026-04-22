# Cache Directory Naming Optimization

## Summary

Change URL transcription job cache directories from the current random job ID shape, such as `url-watch-<timestamp>-<hash>-<random>`, to a stable and readable format:

```text
<video-title>-<video-id>
```

Examples:

```text
~/.cache/textdrain/jobs/Some-Video-Title-BV1GUoNB5E1X/
~/.cache/textdrain/jobs/Bad-Title-Episode-1-QsZFBqtgI8A/
```

## Key Changes

- Inspect URL metadata with `yt-dlp --dump-single-json` before creating the final job work directory.
- Use `title` and `id` from yt-dlp metadata to build URL `JobID` and `WorkDir`.
- Preserve useful letters, numbers, Unicode text, and video ID casing while normalizing path separators, whitespace, and punctuation to `-`.
- Fall back to `<safe-title>-url-<8-char-url-hash>` if yt-dlp does not return an `id`.
- Keep the existing unique directory strategy for local file inputs.

## API And Interface Changes

- Add a `domain.URLInspector` interface:

```go
Inspect(ctx context.Context, asset domain.MediaAsset) (domain.MediaAsset, error)
```

- Add `URLInspector domain.URLInspector` to `transcription.Dependencies`.
- Make `downloader.YTDLP` implement `URLInspector` by reusing the existing metadata parsing path.
- Wire the default CLI so one `YTDLP` instance is used as both URL inspector and downloader.
- Do not add new CLI flags. Existing `job_id`, `work_dir`, and default `outputs/<job_id>` output naturally use the new naming.

## Implementation Details

- Update `transcription.UseCase` URL flow:
  - `Resolve` continues to classify local files and URLs.
  - For URL inputs, call `Inspect` before creating the job work directory.
  - Recompute `asset.JobID` and `asset.WorkDir` from inspected metadata.
  - Create the final work directory, then continue download, audio preparation, transcription, and export.
- Avoid duplicate metadata reads by letting `YTDLP.Fetch` use metadata already present on the inspected asset.
- Keep the current status order unchanged: `PENDING -> RESOLVING -> DOWNLOADING -> EXTRACTING_AUDIO -> TRANSCRIBING -> EXPORTING -> COMPLETED`.
- When the same video is transcribed repeatedly, reuse the same jobs directory. Existing `nextAvailablePath` behavior should continue adding `-1`, `-2`, and so on for colliding media and audio files.

## Test Plan

- Update resolver tests so URL resolution validates the preliminary URL asset rather than final video-title workdir naming.
- Add use case coverage for:
  - YouTube metadata producing a stable `title + id` `JobID` and `WorkDir`.
  - Bilibili-style IDs such as `BV1GUoNB5E1X` preserving casing.
  - Chinese titles appearing in directory names instead of degrading to `untitled`.
  - Missing metadata `id` falling back to `url-<hash>`.
- Add downloader coverage for:
  - `YTDLP.Inspect` returning an asset with `id`, `title`, `site`, `duration`, and metadata.
  - `YTDLP.Fetch` using an already inspected asset without breaking existing fallback download behavior.
- Run:

```bash
go test ./...
```

## Assumptions

- The video ID is the `id` field from yt-dlp JSON metadata.
- Only URL video job cache directories are changed in this work.
- The chosen naming policy is exactly `title + video ID`, without an additional random suffix.
