# ai

A minimal AI shell assistant CLI in Go. Type a request in natural language, get a shell command back.

```
$ ai find all files larger than 1GB
find / -type f -size +1G
```

## Status

Phase 1 — scaffolding. The CLI takes args, calls the OpenRouter API, and prints the raw model response. No flags, config file, confirmation prompt, or command execution yet — those come in later phases.

## Install

```sh
go install github.com/hra42/ai@latest
```

Or build from source:

```sh
git clone https://github.com/hra42/ai
cd ai
go build -o ai .
```

## Usage

Set your OpenRouter API key, then invoke `ai` with your request:

```sh
export OPENROUTER_API_KEY=sk-or-...
ai list all running docker containers
```

The model (`anthropic/claude-sonnet-4-5`) is hardcoded for now and returns the command on stdout.

## License

[Unlicense](LICENSE) — public domain.
