# Repository Guidelines

## Project Structure & Module Organization

TextDrain is a Go CLI for local media ingestion, audio preparation, offline transcription, and transcript export. The executable entry point is `cmd/textdrain/main.go`. Core code lives under `internal/`: `cli` defines Cobra commands, `app/transcription` coordinates use cases, `domain` holds shared types and ports, `config` manages defaults and TOML settings, and `infra` wraps tools such as `yt-dlp`, `ffmpeg`, and `whisper-cli`. Test fixtures are in `testdata/`, sample media is in `samples/`, generated output defaults to `outputs/`, and project docs live in `docs/`.

## Documentation Index for Agents

Read `ARCHITECTURE.md` before changing package boundaries, data flow, external-tool adapters, or extension points. Use "Core Components" and "Layers and Dependency Rules" to decide where code belongs. Check "Data Architecture" for filesystem paths, domain data shapes, and transformations. Review "Implementation Patterns" before adding pipeline stages, adapters, metadata handling, or path sanitization. Use "Testing Architecture" when choosing test scope or fixtures. For feature additions, start with "Extension and Evolution Patterns" and "Blueprint for New Development".

## Build, Test, and Development Commands

- `uv sync`: install the Python tool environment, including `yt-dlp`.
- `uv run go run ./cmd/textdrain doctor`: check local dependencies with uv-managed tools on `PATH`.
- `uv run go run ./cmd/textdrain transcribe ./samples/zh_prompt.wav --lang zh`: run a local transcription smoke test.
- `go test ./...`: run all Go unit tests.
- `go build -o bin/textdrain ./cmd/textdrain`: build the CLI binary.

## Coding Style & Naming Conventions

Use standard Go formatting and run `gofmt` on changed Go files. Keep package names short, lowercase, and aligned with directory purpose, such as `cli`, `config`, or `media`. Prefer explicit interfaces at domain boundaries and small structs for command options or configuration. Write code comments and documentation in English; keep comments focused on non-obvious behavior.

## Testing Guidelines

Tests use Go's standard `testing` package and live beside implementation files as `*_test.go`. Name tests after observable behavior, for example `TestResolverRejectsMissingInput`. Cover external command behavior with fixtures under `testdata/commands` or small media files under `testdata/media`; avoid network-dependent tests. Run `go test ./...` before opening a pull request.

## Commit & Pull Request Guidelines

Recent history uses concise imperative or conventional-style subjects, such as `Add MVP test coverage` and `docs: archive MVP planning docs`. Keep commits focused on one logical change. Pull requests should include the problem, implementation summary, test results, and any dependency or configuration notes. Include screenshots or terminal output only when they clarify user-facing CLI behavior.

## Security & Configuration Tips

Do not commit local models, generated transcripts, secrets, or large temporary media. Keep machine-specific settings in `~/.config/textdrain/config.toml`; repository defaults and examples should remain portable.
