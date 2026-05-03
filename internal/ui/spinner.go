package ui

import (
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

type spinnerModel struct {
	sp      spinner.Model
	message string
}

func (m spinnerModel) Init() tea.Cmd { return m.sp.Tick }

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.QuitMsg:
		return m, tea.Quit
	case tea.KeyMsg:
		_ = msg
		return m, nil
	}
	var cmd tea.Cmd
	m.sp, cmd = m.sp.Update(msg)
	return m, cmd
}

func (m spinnerModel) View() string {
	return m.sp.View() + " " + m.message
}

// StartSpinner starts a spinner on stderr. Returns a stop function that
// tears the spinner down and clears the line. If stderr is not a TTY, the
// returned stop is a no-op.
func StartSpinner(message string) func() {
	if !isatty.IsTerminal(os.Stderr.Fd()) {
		return func() {}
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(mocha_mauve))

	prog := tea.NewProgram(
		spinnerModel{sp: sp, message: message},
		tea.WithOutput(os.Stderr),
		tea.WithoutSignalHandler(),
	)

	done := make(chan struct{})
	go func() {
		_, _ = prog.Run()
		close(done)
	}()

	return func() {
		prog.Quit()
		<-done
	}
}
