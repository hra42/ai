package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type promptMode int

const (
	modeCommand promptMode = iota
	modeChat
)

type sysInfo struct {
	OS       string
	Shell    string
	Hostname string
	Cwd      string
}

func gatherSysInfo() sysInfo {
	info := sysInfo{OS: runtime.GOOS}
	if sh := os.Getenv("SHELL"); sh != "" {
		info.Shell = filepath.Base(sh)
	}
	if h, err := os.Hostname(); err == nil {
		info.Hostname = h
	}
	if d, err := os.Getwd(); err == nil {
		info.Cwd = d
	}
	return info
}

func (s sysInfo) block() string {
	var b strings.Builder
	b.WriteString("Environment:\n")
	if s.OS != "" {
		fmt.Fprintf(&b, "- os: %s\n", s.OS)
	}
	if s.Shell != "" {
		fmt.Fprintf(&b, "- shell: %s\n", s.Shell)
	}
	if s.Hostname != "" {
		fmt.Fprintf(&b, "- hostname: %s\n", s.Hostname)
	}
	if s.Cwd != "" {
		fmt.Fprintf(&b, "- cwd: %s\n", s.Cwd)
	}
	return b.String()
}

func buildSystemPrompt(mode promptMode) string {
	env := gatherSysInfo().block()
	switch mode {
	case modeChat:
		return env + `
You are a concise shell and systems assistant. Answer in plain text — no markdown headings.

suggested_command rules — be strict:
- Set it ONLY when the user is asking how to do something AND the command is fully runnable as-is on this system, with no placeholders the user has to fill in.
- "What does X do?" / "Explain Y" / "Why does Z happen?" → suggested_command MUST be empty.
- Any command containing placeholders like <file>, FILENAME, dateiname, /path/to/x, $VAR-without-real-value → suggested_command MUST be empty.
- Destructive commands (rm -rf, dd, mkfs, anything that mutates outside cwd) → suggested_command MUST be empty unless the user explicitly named the target.
- When unsure, leave it empty. Mention the example command inside answer instead.`
	default:
		return env + "\nYou generate shell commands for the user's environment above (mind BSD vs GNU tooling and the user's shell). Put the raw single-line command in command. Put one short sentence in explanation describing what it does. No markdown anywhere."
	}
}
