# Add `clear-cache` Command

## Summary

Add a top-level CLI command `clear-cache` to clear TextDrain's job cache directory.
Before deleting anything, the command must prompt the user in English and include
the absolute cache path in the confirmation message. This version only clears the
configured `jobs` directory and leaves `models` untouched.

## Key Changes

- Add and register a new top-level `clear-cache` command in `internal/cli`.
- Define command behavior:
  - Accept no positional arguments.
  - Resolve the target directory from `cfg.JobsDir`.
  - Print an English confirmation prompt before deletion.
  - Include the absolute target path in the prompt.
  - Read user input from `cmd.InOrStdin()`.
  - Only proceed when the user explicitly confirms.
  - Treat any non-confirmation input as a safe no-op.
- Implement deletion as "clear the job cache contents" instead of removing the
  `jobs` directory permanently, so the directory still exists after the command
  completes.
- Keep the implementation in the CLI layer or a thin internal helper. Do not
  route this through the transcription use case.
- Show the command in root help output. Keep the hidden `paths` command unchanged.
- If the `jobs` directory does not exist, still show the confirmation prompt and
  treat the confirmed operation as a successful no-op.
- Reuse the existing CLI error model:
  - Parameter validation uses `NewParameterError`.
  - Filesystem failures use a runtime error classification.

## Public Interface Changes

- Add a new public CLI command:
  - `textdrain clear-cache`
- No new config keys.
- No changes to default path resolution; continue to use the configured `jobs_dir`.

## Test Plan

- Root help output includes `clear-cache`.
- `textdrain clear-cache extra` returns a parameter error.
- Confirmed execution (`y` / `Y`) removes files and nested directories under
  `cfg.JobsDir`.
- After confirmed execution, the `jobs` directory still exists.
- Non-confirmation input or an empty response does not delete anything.
- The confirmation prompt is in English and includes the absolute `cfg.JobsDir`.
- A missing `jobs` directory does not cause an error.
- Deletion or recreation failures surface as runtime errors.
- Extend CLI test helpers so tests can inject stdin for interactive command coverage.

## Assumptions

- The command name is `clear-cache`.
- "Cache" in this feature means the job cache directory only, not model files.
- This version does not add flags such as `--all`, `--yes`, or `--dry-run`.
- User-facing confirmation text is in English. Code comments and documentation stay in English.
