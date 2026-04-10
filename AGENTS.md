# Repository Guidelines

## Project Structure & Module Organization
`TextDrain` is a Go CLI project. The executable entrypoint lives in `cmd/textdrain/main.go`. Core application wiring is in `internal/app`, command definitions and terminal I/O are in `internal/cli`, runtime path defaults are in `internal/config`, and infrastructure helpers such as logging live under `internal/infra`. Use `internal/domain` for shared business types and interfaces as the transcription pipeline grows. Design and planning notes are kept in `docs/`.

## Build, Test, and Development Commands
- `go run ./cmd/textdrain --help`: run the CLI locally.
- `go build ./cmd/textdrain`: build the binary for the current platform.
- `go test ./...`: run all unit tests across the module.
- `go fmt ./...`: format Go source files before review.
- `go vet ./...`: catch suspicious constructs early.

Example: run `go run ./cmd/textdrain paths` to inspect the default config, cache, jobs, and models directories.

## Coding Style & Naming Conventions
Follow standard Go formatting and keep code `gofmt`-clean. Use tabs for indentation, exported identifiers in `PascalCase`, unexported identifiers in `camelCase`, and short package names such as `app`, `cli`, and `config`. Keep command setup in `internal/cli`, orchestration in `internal/app`, and avoid mixing user-facing output with structured logs. Write comments and docs in English, and prefer small functions that return explicit errors.

## Testing Guidelines
Place tests next to the code they cover using `*_test.go` files. Prefer table-driven tests for command parsing, path resolution, and future pipeline state transitions. Add coverage for both success paths and operator-facing failures, especially around filesystem paths and external tool detection. Run `go test ./...` before opening a PR.

## Commit & Pull Request Guidelines
Current history uses short, imperative subjects with optional prefixes, for example `chore: initialize textdrain cli skeleton`. Keep commits focused and descriptive. For PRs, include:
- a brief summary of behavior changes
- linked issue or planning doc when applicable
- test evidence such as `go test ./...`
- CLI output examples or screenshots when help text or terminal UX changes

## Configuration & Runtime Notes
The project assumes local-first execution. Default runtime paths resolve under `~/.config/textdrain` and `~/.cache/textdrain`. When adding features that shell out to tools like `yt-dlp`, `ffmpeg`, or `whisper-cli`, keep failures actionable and surface clear install guidance in CLI errors.
