package config

import (
	"io"
	"os"

	"golang.org/x/term"
)

// stderrForPicker / stdinForPicker keep TUI rendering on stderr and accept
// input from /dev/tty so the wizard works even when stdin/stdout are pipes.
func stderrForPicker() *os.File { return os.Stderr }

func stdinForPicker() *os.File {
	if tty, err := os.Open("/dev/tty"); err == nil {
		return tty
	}
	return os.Stdin
}

// IsTTY reports whether the reader is an interactive terminal.
func IsTTY(in io.Reader) bool {
	f, ok := in.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
