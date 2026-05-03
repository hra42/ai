package exec

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/hra42/ai/internal/history"
	"github.com/hra42/ai/internal/ui"
)

// Run executes command via $SHELL -c, with stdio inherited from the parent.
// Falls back to /bin/sh when $SHELL is unset. The command is appended to the
// user's zsh history (best effort) so it shows up under the up-arrow.
func Run(ctx context.Context, command string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	if err := history.Append(command); err != nil {
		fmt.Fprintln(os.Stderr, ui.Hint.Render("history: "+err.Error()))
	}
	c := exec.CommandContext(ctx, shell, "-c", command)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
