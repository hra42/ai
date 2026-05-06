<p align="left">
  <img src="./media/images/icon.png" alt="ai logo" width="96" />
</p>

# ai

A minimal AI shell assistant CLI in Go. Type a request in natural language, get a shell command back — review it in a TUI, run it on enter.

```
$ ai list all files with .go extension

  Suggested command
  ╭─────────────────────────────╮
  │ find . -name '*.go' -type f │
  ╰─────────────────────────────╯
  Lists all .go files in the current directory and below.
  [enter/y] run · [esc/n] cancel
```

## Install

### Homebrew (macOS, Linux)

```sh
brew install hra42/tap/ai
```

### Install script (macOS, Linux)

```sh
curl -fsSL https://ai.hra42.lol/install | sh
```

Picks the right binary for your OS/arch, verifies the SHA-256 from `checksums.txt`, and drops it into `/usr/local/bin` (override with `INSTALL_DIR=…`). Pin a version with `VERSION=v0.1.4`.

### `go install`

```sh
go install github.com/hra42/ai@latest
```

### Pre-built binaries

Grab a tarball for your OS/arch from the [Releases page](https://github.com/hra42/ai/releases) and drop the `ai` binary into your `$PATH`.

### From source

```sh
git clone https://github.com/hra42/ai
cd ai
go build -o ai .
```

Check the installed version with `ai --version`.

## First run

On first launch (no config) `ai` walks you through a TUI setup wizard:

1. **API key source** — pick one:
   - Save plaintext in `~/.config/ai/config.yaml`
   - Read from 1Password via secret reference (`op://Vault/Item/field`)
   - Use `$OPENROUTER_API_KEY` env var only (nothing saved)
2. **Model** — fuzzy-search the live OpenRouter model catalog. Hit ↑/↓, enter to pick.

Re-run setup any time by deleting `~/.config/ai/config.yaml`.

## Usage

### Command mode (default)

```sh
ai <request>
```

Generates a shell command, shows it in a review TUI with a one-line explanation, runs it on `enter`. Cancel with `esc`/`n`. Executed commands land in your zsh history.

![Basics demo](./media/ai_demos/gifs/01_basic.gif)

### Chat mode

```sh
ai -c <question>
```

Free-form Q&A. If the answer naturally implies a command (and it's actually runnable — no placeholders), the TUI offers to run it.

```sh
ai -c "what does chmod 755 do"            # explanation only
ai -c "how do I list .go files"           # explanation + suggested command
```

![Chat demo](./media/ai_demos/gifs/03_chat.gif)

### Pipe support

```sh
cat error.log | ai explain the error
ps aux | ai -c "which process is using the most memory"
```

Piped stdin is appended to the prompt as additional context. The review TUI still works because input is read from `/dev/tty`.

![Pipe demo](./media/ai_demos/gifs/02_pipe.gif)

### Flags

| Flag | Purpose |
|---|---|
| `-c, --chat` | Chat mode — answer the question, optionally suggest a command |
| `-y, --yes` | Skip confirmation, run the command directly (for scripts) |
| `-p, --print` | Print the command and exit, don't execute |
| `--model <id>` | Override the configured model for this call |

`--yes` and `--print` are required for non-TTY use (CI, pipes); without one of them the CLI errors out instead of running silently.

## Context

Each request includes lightweight environment context so the model can tailor commands to your system:

- **Static**: OS (`darwin`/`linux`), shell (`zsh`), hostname, current working directory
- **Working directory listing**: top-level entries, with `.gitignore` respected when inside a Git repo
- **Git status**: branch + counts (`M=3 ?=1`), no file paths

**Never sent**: file contents, env vars, recursive listings, command history, or git diffs.

### Sensitive filenames

Filenames that look like credentials are redacted to `[redacted]` before being sent. The list covers dotenv files, PEM/key/keystore files, SSH keys, cloud credential dirs (`.aws`, `.gcp`, `.azure`, `.kube`), shell history, Terraform state, WireGuard configs, OpenVPN profiles, and similar (see [`internal/runner/context.go`](internal/runner/context.go) for the full list).

To disable redaction, edit `~/.config/ai/config.yaml`:

```yaml
redact_secrets: false
```

## Configuration

`~/.config/ai/config.yaml`:

```yaml
api_key: sk-or-...                    # plaintext OpenRouter key, OR…
op_ref: op://Vault/Item/field         # 1Password secret reference (resolved via `op read`)
model: anthropic/claude-haiku-4.5
redact_secrets: true                  # default; set false to send raw filenames
```

`$OPENROUTER_API_KEY` always takes precedence over the file.

## Releasing

Tagged pushes (`v*`) trigger `.github/workflows/release.yml`, which runs GoReleaser to build binaries for darwin/linux × amd64/arm64, publish a GitHub Release with checksums, and update the `hra42/homebrew-tap` formula.

```sh
git tag v0.1.4
git push origin v0.1.4
```

A `HOMEBREW_TAP_GITHUB_TOKEN` secret (PAT with `repo` scope on `hra42/homebrew-tap`) must be configured in the repo's Actions secrets.

## License

[Unlicense](LICENSE) — public domain.
