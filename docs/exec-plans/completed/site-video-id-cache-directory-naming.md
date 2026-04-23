# Site Video ID Cache Directory Naming

## Summary

Change URL transcription job cache directories from the current `title-video-id` shape to `site-video-id`.

Examples:

- `bilibili-BV1GUoNB5E1X`
- `youtube-QsZFBqtgI8A`

Only URL-backed job/cache directory naming changes. Local file inputs, downloaded media filenames, transcript export formats, and the transcription pipeline remain unchanged.

## Key Changes

- Update URL job ID generation in `internal/app/transcription/usecase.go`.
- Use `asset.Metadata["id"]` as the video ID when it is available.
- Derive the site name from inspected `yt-dlp` metadata or the already resolved URL site.
- Normalize YouTube sites to `youtube`.
- Normalize Bilibili sites to `bilibili`.
- For other `yt-dlp` compatible sites, use a safe lowercase extractor or hostname-derived site name.
- Use these naming rules:
  - With video ID: `<site>-<video-id>`
  - Without video ID: `<site>-url-<8-char-url-hash>`
- Preserve video ID casing, such as `BV1GUoNB5E1X` and `QsZFBqtgI8A`.
- Keep existing path safety behavior by replacing path separators, whitespace, and unsafe punctuation with `-`.
- Do not add CLI flags or change domain interfaces; this is an internal naming policy update.

## Test Plan

- Update URL job path tests for YouTube metadata so `id=QsZFBqtgI8A` produces `youtube-QsZFBqtgI8A`.
- Add or update a Bilibili URL case for `https://www.bilibili.com/video/BV1GUoNB5E1X/` so it produces `bilibili-BV1GUoNB5E1X`.
- Verify Bilibili ID casing is preserved.
- Verify missing metadata ID falls back to `<site>-url-<hash>`.
- Replace old `title-video-id` expectations and remove any expectation that Chinese titles participate in URL cache directory names.
- Run `go test ./...`.

## Assumptions

- The video ID is the `id` field from `yt-dlp` JSON metadata.
- Cache directory site names use lowercase English short names for the requested platforms: `youtube` and `bilibili`.
- Non-YouTube and non-Bilibili `yt-dlp` compatible sites continue to work by using a normalized extractor or hostname as `<site>`.
