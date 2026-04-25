# TextDrain

TextDrain is a local-first CLI for downloading media, preparing audio, running offline speech recognition, and exporting transcripts.

The MVP uses a Go CLI as the orchestrator and delegates heavy media work to mature local tools:

- `yt-dlp` for URL metadata and downloads
- `deno` for `yt-dlp` JavaScript extraction on sites such as YouTube
- `ffmpeg` for audio extraction and normalization
- `whisper-cli` from `whisper.cpp` for offline transcription

TextDrain supports local media files and `yt-dlp` compatible URLs. Transcription runs locally after the download step.

## Requirements

- Go `1.24+`
- Python `3.12+` for the uv-managed tool environment
- `uv`
- `yt-dlp`
- `deno`
- `ffmpeg`
- `whisper-cli` from `whisper.cpp`
- A local whisper.cpp model file in `.gguf` or legacy `.bin` format

## Install External Tools

Install or synchronize the Python environment from the project root:

```bash
uv sync
```

This installs Python package tools declared by the project, including `yt-dlp`.

Install `deno` so current `yt-dlp` can run site JavaScript needed by extractors such as YouTube. On macOS with Homebrew:

```bash
brew install deno
```

Run TextDrain through `uv run` when you want the Go process to discover `yt-dlp` from the uv environment:

```bash
uv run go run ./cmd/textdrain doctor
```

Alternatively, activate the virtual environment first:

```bash
source .venv/bin/activate
go run ./cmd/textdrain doctor
```

Install `ffmpeg` with your system package manager. On macOS with Homebrew:

```bash
brew install ffmpeg
```

Install `whisper.cpp` and make `whisper-cli` available on `PATH`. On macOS with Homebrew:

```bash
brew install whisper-cpp
```

Verify the external commands:

```bash
uv run yt-dlp --version
deno --version
ffmpeg -version
whisper-cli --version
```

## Prepare A Model

TextDrain does not download models in the MVP. Place a whisper.cpp model file in the configured model directory.

The default model directory is:

```text
~/.cache/textdrain/models
```

Create the directory:

```bash
mkdir -p ~/.cache/textdrain/models
```

Then copy or move a GGUF model into it. The default model name is `small`, so TextDrain looks for candidate files such as:

- `small`
- `small.gguf`
- `small.bin`
- `ggml-small.gguf`
- `ggml-small.bin`
- `ggml-small.q5_0.gguf`
- `ggml-small.q5_0.bin`

You can also point `model` or `--model` at an absolute model file path.

Check discovered models:

```bash
go run ./cmd/textdrain models --list
```

## First Run

From the project root, run the environment check:

```bash
uv run go run ./cmd/textdrain doctor
```

`doctor` checks tool availability, versions, default paths, and the selected model file. Fix any reported dependency or model issue before running transcription.

Build the CLI:

```bash
go build -o bin/textdrain ./cmd/textdrain
```

Show help:

```bash
./bin/textdrain --help
```

## Common Commands

Transcribe a local media file:

```bash
uv run go run ./cmd/textdrain transcribe ./samples/zh_prompt.wav
```

Transcribe a URL supported by `yt-dlp`:

```bash
uv run go run ./cmd/textdrain transcribe "<yt-dlp-compatible-url>"
```

Use browser cookies for sites that require a signed-in session:

```bash
uv run go run ./cmd/textdrain transcribe "https://www.youtube.com/watch?v=CIUtEnnjA2U" --cookies-from-browser chrome
uv run go run ./cmd/textdrain transcribe "<yt-dlp-compatible-url>" --cookies ./cookies.txt
```

Pass additional `yt-dlp` arguments through when a site needs browser impersonation or custom headers:

```bash
uv run go run ./cmd/textdrain transcribe "https://www.bilibili.com/video/BV19LoeBbEv8" \
  --yt-dlp-arg=--impersonate \
  --yt-dlp-arg=chrome
```

For Bilibili, impersonation alone may still return `HTTP 412`. In that case, pass browser cookies as well:

```bash
uv run go run ./cmd/textdrain transcribe "https://www.bilibili.com/video/BV19LoeBbEv8" \
  --cookies-from-browser chrome \
  --yt-dlp-arg=--impersonate \
  --yt-dlp-arg=chrome
```

Specify the transcription language:

```bash
uv run go run ./cmd/textdrain transcribe ./samples/zh_prompt.wav --lang zh
uv run go run ./cmd/textdrain transcribe /path/to/english-media.mp3 --lang en
uv run go run ./cmd/textdrain transcribe /path/to/media.mp4 --lang auto
```

Specify the model:

```bash
uv run go run ./cmd/textdrain transcribe ./samples/zh_prompt.wav --model small
uv run go run ./cmd/textdrain transcribe ./samples/zh_prompt.wav --model ~/.cache/textdrain/models/ggml-small.gguf
```

Specify the output directory:

```bash
uv run go run ./cmd/textdrain transcribe ./samples/zh_prompt.wav --output ./outputs/demo
```

Keep intermediate files for debugging:

```bash
uv run go run ./cmd/textdrain transcribe ./samples/zh_prompt.wav --keep-intermediate
```

Check the environment:

```bash
uv run go run ./cmd/textdrain doctor
```

List local models:

```bash
uv run go run ./cmd/textdrain models --list
```

## Outputs

TextDrain exports four transcript formats by default:

- `txt`
- `srt`
- `vtt`
- `json`

If `--output` is omitted, files are written under:

```text
outputs/<job-id>/
```

Each job uses a separate working directory under the configured jobs directory. Intermediate media and normalized audio files are removed by default unless `--keep-intermediate` or `keep_intermediate_files = true` is set.

## Configuration

TextDrain reads an optional TOML config file from:

```text
~/.config/textdrain/config.toml
```

Supported MVP keys:

```toml
model = "small"
language = "auto"
output_formats = ["txt", "srt", "vtt", "json"]
keep_intermediate_files = false
model_dir = "/Users/<your-name>/.cache/textdrain/models"
jobs_dir = "/Users/<your-name>/.cache/textdrain/jobs"
```

Configuration precedence:

1. CLI flags
2. `config.toml`
3. Built-in defaults

The `transcribe` command exposes CLI overrides for language, model, output directory, and intermediate file retention.

## Troubleshooting

### `yt-dlp=missing`

Run TextDrain through `uv run` or activate the virtual environment so the Go process can find `yt-dlp`:

```bash
uv run go run ./cmd/textdrain doctor
```

If needed, synchronize the environment again:

```bash
uv sync
```

### `deno=missing`

Install `deno` so `yt-dlp` can run the JavaScript needed by sites such as YouTube:

```bash
brew install deno
deno --version
```

### `ffmpeg=missing`

Install `ffmpeg` and ensure it is available on `PATH`:

```bash
ffmpeg -version
```

### `whisper-cli=missing`

Install `whisper.cpp` and ensure the executable is named `whisper-cli` and available on `PATH`:

```bash
whisper-cli --version
```

### Model File Was Not Found

Run:

```bash
uv run go run ./cmd/textdrain models --list
```

Place a `.gguf` or `.bin` model in `~/.cache/textdrain/models`, or set `model_dir` and `model` in `config.toml`. You can also pass an absolute file path:

```bash
uv run go run ./cmd/textdrain transcribe ./samples/zh_prompt.wav --model /path/to/model.gguf
```

### Invalid Language

Only these language values are supported in the MVP:

- `auto`
- `zh`
- `en`

Use:

```bash
uv run go run ./cmd/textdrain transcribe ./samples/zh_prompt.wav --lang auto
```

### URL Download Failed

Check that the URL is supported by `yt-dlp` and that the network is reachable:

```bash
uv run yt-dlp --dump-single-json "<yt-dlp-compatible-url>"
```

If `yt-dlp` reports `No supported JavaScript runtime could be found`, install `deno` or configure `yt-dlp` with `--js-runtimes` for another supported runtime.

If YouTube reports `Sign in to confirm you're not a bot`, pass browser cookies from a signed-in browser profile:

```bash
uv run go run ./cmd/textdrain transcribe "https://www.youtube.com/watch?v=CIUtEnnjA2U" --cookies-from-browser chrome
```

You can also export a cookies.txt file and pass it with `--cookies ./cookies.txt`.

TextDrain does not handle DRM-protected content.

### Audio Extraction Failed

Check that `ffmpeg` can read the input media:

```bash
ffmpeg -i ./path/to/media.mp4
```

If the input is corrupt or uses an unsupported codec, convert it to a common audio or video format before retrying.

### Transcription Failed

Check that the model path is valid, the audio file can be prepared, and `whisper-cli` can run independently:

```bash
whisper-cli -m ~/.cache/textdrain/models/ggml-small.gguf -f ./samples/zh_prompt.wav -l auto -oj -of /tmp/textdrain-check -np
```

Use `--keep-intermediate` to preserve job files for inspection.

## Exit Codes

- `0`: success
- `2`: parameter or configuration error
- `3`: dependency error
- `4`: runtime error

## MVP Scope

The MVP focuses on single-job CLI transcription for local files and `yt-dlp` compatible URLs. It does not implement model downloads, speaker diarization, translation, summary generation, DRM handling, or GUI workflows.
