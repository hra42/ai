package history

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Append writes command to the user's zsh history file in extended-history
// format (`: <ts>:0;<cmd>`). It's a no-op when $SHELL isn't zsh, since other
// shells use different history formats. File errors are returned to the
// caller, which decides whether to surface them.
func Append(command string) error {
	if !isZsh() {
		return nil
	}
	path, err := histPath()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	// zsh extended history: `: <unix-ts>:<elapsed>;<command>`. Multi-line
	// commands escape newlines with a trailing backslash.
	escaped := strings.ReplaceAll(command, "\n", "\\\n")
	_, err = fmt.Fprintf(f, ": %d:0;%s\n", time.Now().Unix(), escaped)
	return err
}

func isZsh() bool {
	sh := os.Getenv("SHELL")
	return filepath.Base(sh) == "zsh"
}

func histPath() (string, error) {
	if p := os.Getenv("HISTFILE"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".zsh_history"), nil
}
