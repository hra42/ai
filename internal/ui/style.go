package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

// Catppuccin Mocha palette (subset).
const (
	mocha_mauve    = "#cba6f7"
	mocha_red      = "#f38ba8"
	mocha_green    = "#a6e3a1"
	mocha_subtext1 = "#bac2de"
)

var (
	Command = lipgloss.NewStyle().Foreground(lipgloss.Color(mocha_mauve)).Bold(true)
	Error   = lipgloss.NewStyle().Foreground(lipgloss.Color(mocha_red)).Bold(true)
	Prompt  = lipgloss.NewStyle().Foreground(lipgloss.Color(mocha_green))
	Hint    = lipgloss.NewStyle().Foreground(lipgloss.Color(mocha_subtext1))
)

func init() {
	if os.Getenv("NO_COLOR") != "" || !isatty.IsTerminal(os.Stdout.Fd()) {
		plain := lipgloss.NewStyle()
		Command = plain
		Error = plain
		Prompt = plain
		Hint = plain
	}
}
