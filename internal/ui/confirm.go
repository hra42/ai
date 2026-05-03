package ui

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Confirm renders a styled y/N prompt to out, reads one line from in, and
// returns true only on an affirmative ("y" or "yes", case-insensitive).
// Default is no.
func Confirm(in io.Reader, out io.Writer, prompt string) (bool, error) {
	fmt.Fprint(out, Prompt.Render(prompt), Hint.Render(" [y/N] "))
	r := bufio.NewReader(in)
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
