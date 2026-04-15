# TextDrain Usage Guide

## Overview

TextDrain is a local-first CLI for downloading media, preparing audio, and exporting transcripts.

Current version: `0.1.0`

At this stage, the project mainly provides:

- CLI entry and help output
- Version display
- Config file loading
- `transcribe` end-to-end transcription workflow
- `doctor` dependency and model environment checks
- `models --list` command for local model directory inspection
- Hidden path inspection command for local directories

## Requirements

- Go `1.24+`
- uv

## Python Tool Environment

TextDrain uses external command-line tools for media downloads. The project includes a uv-managed Python environment for tools that are distributed as Python packages, including `yt-dlp`.

From the project root, create or synchronize the Python environment:

```bash
uv sync
```

Run Python tools through uv:

```bash
uv run yt-dlp --version
```

If you want commands such as `textdrain doctor` to discover `yt-dlp` from the virtual environment, either run TextDrain through `uv run`:

```bash
uv run go run ./cmd/textdrain doctor
```

Or activate the virtual environment before running TextDrain:

```bash
source .venv/bin/activate
go run ./cmd/textdrain doctor
```

## Build And Run

Run directly from source:

```bash
go run ./cmd/textdrain --help
```

Build a local binary:

```bash
go build -o bin/textdrain ./cmd/textdrain
```

Run the built binary:

```bash
./bin/textdrain --help
```

## Available Commands

### Show Help

```bash
textdrain --help
```

Or, if you are running from source:

```bash
go run ./cmd/textdrain --help
```

Expected output includes:

- CLI description
- Available commands
- Global flags such as `--help` and `--version`

### Transcribe Media

```bash
textdrain transcribe <url-or-path> --lang auto --model small --output ./out --keep-intermediate
```

Supported flags:

- `--lang auto|zh|en`
- `--model <name>`
- `--output <dir>`
- `--keep-intermediate`

The command resolves the input, downloads URL media when needed, prepares normalized audio with `ffmpeg`, transcribes with `whisper-cli`, exports the configured transcript formats, and removes intermediate media unless `--keep-intermediate` is set.

If `--output` is omitted, exports are written under `outputs/<job-id>/` from the current working directory.

### Check The Environment

```bash
textdrain doctor
```

The command checks:

- Whether `yt-dlp`, `ffmpeg`, and `whisper-cli` are available and executable
- Version output for each external tool
- The configured model directory and default model file
- Default config, cache, jobs, and model paths

If a dependency or model file is missing, the command prints a repair suggestion and exits with the dependency error code.

### List Local Models

```bash
textdrain models --list
```

The command lists files from the configured `model_dir`. If the directory does not exist, it reports zero models.

## Configuration

TextDrain reads configuration from:

```text
~/.config/textdrain/config.toml
```

The file is optional. When it does not exist, TextDrain runs with built-in defaults.

Supported MVP keys:

```toml
model = "small"
language = "auto"
output_formats = ["txt", "srt", "vtt", "json"]
keep_intermediate_files = false
model_dir = "/Users/<your-name>/.cache/textdrain/models"
jobs_dir = "/Users/<your-name>/.cache/textdrain/jobs"
```

Default values:

- `model`: `small`
- `language`: `auto`
- `output_formats`: `txt`, `srt`, `vtt`, `json`
- `keep_intermediate_files`: `false`
- `model_dir`: `~/.cache/textdrain/models`
- `jobs_dir`: `~/.cache/textdrain/jobs`

Config precedence is:

1. CLI overrides
2. `config.toml`
3. Built-in defaults

The `transcribe` command currently exposes CLI overrides for language, model, output directory, and intermediate file retention.

### Show Version

```bash
textdrain --version
```

Current output:

```text
0.1.0
```

### Inspect Default Paths

This is a hidden command intended for diagnostics.

```bash
textdrain paths
```

Example output:

```text
config=/Users/<your-name>/.config/textdrain
config_file=/Users/<your-name>/.config/textdrain/config.toml
cache=/Users/<your-name>/.cache/textdrain
jobs=/Users/<your-name>/.cache/textdrain/jobs
models=/Users/<your-name>/.cache/textdrain/models
```

## Default Directory Layout

TextDrain uses the following local directories by default:

- Config directory: `~/.config/textdrain`
- Cache directory: `~/.cache/textdrain`
- Job directory: `~/.cache/textdrain/jobs`
- Model directory: `~/.cache/textdrain/models`

These paths are computed from the current user's home directory.

## Shell Completion

Because the project uses Cobra, completion scripts are available through the generated command:

```bash
textdrain completion --help
```

Common examples:

```bash
textdrain completion bash
textdrain completion zsh
textdrain completion fish
```

## Current Limitations

- No `transcribe` command yet
- No media download pipeline yet
- No audio extraction pipeline yet
- No offline ASR integration yet
- No export workflow yet

Planned capabilities are documented in:

- `docs/design.md`
- `docs/mvp-plan.md`
- `docs/tech-selection.md`

## Troubleshooting

If the command fails to run:

1. Make sure Go `1.24+` is installed.
2. Make sure you are in the project root directory.
3. Run `go mod download` if dependencies are missing.
4. Re-run `go run ./cmd/textdrain --help` to confirm the CLI starts correctly.
