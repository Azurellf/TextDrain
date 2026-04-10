# TextDrain Usage Guide

## Overview

TextDrain is a local-first CLI for downloading media, preparing audio, and exporting transcripts.

Current version: `0.1.0`

At this stage, the project mainly provides:

- CLI entry and help output
- Version display
- Hidden path inspection command for local directories

The end-to-end transcription workflow described in the design documents is not implemented yet. This guide focuses on what you can use today.

## Requirements

- Go `1.24+`

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
- No config file loading yet

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
