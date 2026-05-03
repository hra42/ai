# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`ai` is a Go CLI that turns a natural-language request into a shell command via OpenRouter, then shows it in a Bubble Tea TUI for review before executing it through `$SHELL -c`. It also has a chat mode (`-c`) that returns a free-form answer plus an optional suggested command.

## Common commands

```sh
go build -o ai .          # build the single binary
go run . <request>        # run from source
go test ./...             # run tests (no test files yet — runs cleanly)
go vet ./...              # vet
gofmt -l .                # list unformatted files
```

There is no Makefile, lint config, or CI yet.

## Architecture

`main.go` is the Cobra entry point. It loads config, resolves the API key (env → 1Password `op_ref` → plaintext `api_key`), runs the first-time TUI wizard if either key or model is missing, then dispatches to `handleCommand` (default) or `handleChat` (`-c`). Both handlers have two paths: a non-TTY path (used with `--print`, `--yes`, or piped output) that bypasses the TUI, and a TTY path that drives `ui.RunReview`.

Internal packages — each owns one concern, no cross-imports between siblings except via `runner` → none and `exec` → `history`/`ui`:

- **`internal/config`** — YAML at `~/.config/ai/config.yaml` (`Config` struct, `Load`/`Save`/`Path`), the first-run Bubble Tea wizard (`wizard.go`, `picker_io.go`) that picks API-key source and fuzzy-searches the live OpenRouter model catalog, and `ResolveOpRef` which shells out to `op read` for 1Password references. `RedactSecretsEnabled()` defaults true when the YAML field is absent.
- **`internal/runner`** — the OpenRouter call. Two modes (`modeCommand`, `modeChat`) with strict JSON schemas (`commandSchema`, `chatSchema`) so the model returns structured `{command, explanation}` or `{answer, suggested_command}`. `Options.RedactSecrets` flows through to `context.go`, which builds the system-prompt context: OS/shell/hostname/cwd from `sysinfo.go`, plus a top-level directory listing (`.gitignore`-aware inside a git repo) and `git status` summary. Sensitive filenames (dotenv, PEM, SSH keys, cloud creds, etc.) are replaced with `[redacted]` — the redaction list lives in `context.go` and the README documents it.
- **`internal/ui`** — Bubble Tea + Lipgloss. `RunReview` is the unified review TUI used by both command and chat modes; it takes a generator callback so the spinner and the API call run concurrently. Catppuccin Mocha palette in `style.go`.
- **`internal/exec`** — runs the chosen command via `$SHELL -c` (falls back to `/bin/sh`), inheriting stdio. Calls `history.Append` before exec.
- **`internal/history`** — best-effort append to `$HISTFILE` (or `~/.zsh_history`) in zsh extended-history format. No-op for non-zsh shells. Errors are surfaced as a hint, never fatal.

## Conventions worth knowing

- `--print` and `--yes` are mutually exclusive and are required when stdout/stderr is not a TTY (CI, pipes); without one of them the CLI errors instead of running silently.
- Piped stdin is read in `readPipedStdin` and passed as `Options.StdinContext` so the model sees it as additional context. The TUI still works because Bubble Tea reads from `/dev/tty`, not stdin.
- Default model constant is `defaultModel` in `main.go` (currently `anthropic/claude-sonnet-4-5`); the README example uses `anthropic/claude-haiku-4.5`. Treat the constant as the source of truth.
- The OpenRouter client is `github.com/hra42/openrouter-go` (a separate project by the same author). Structured outputs use its `JSONSchema` with `Strict: true`.
- Go module targets `go 1.26.2` (very recent — older toolchains will reject the build).
