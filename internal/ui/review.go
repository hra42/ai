package ui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Outcome reports what the user picked in the review TUI.
type Outcome int

const (
	OutcomeRun Outcome = iota
	OutcomeCancel
)

var (
	rvTitle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#cba6f7")).Bold(true)
	rvCommand = lipgloss.NewStyle().Foreground(lipgloss.Color("#cba6f7")).Bold(true)
	rvAnswer  = lipgloss.NewStyle().Foreground(lipgloss.Color("#bac2de"))
	rvHint    = lipgloss.NewStyle().Foreground(lipgloss.Color("#bac2de"))
	rvError   = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")).Bold(true)
	rvFrame   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#cba6f7")).
			Padding(0, 1)
)

// LLMCall is the async work the review runs while showing a spinner. It must
// honor ctx cancellation so Ctrl+C in the TUI aborts the request.
type LLMCall func(ctx context.Context) (Result, error)

// Result is what the LLMCall returns to the TUI: either a runnable command
// (with explanation) or a chat-style answer plus an optional suggestion.
type Result struct {
	Answer           string
	Command          string
	Explanation      string
	SuggestedCommand string
}

type reviewModel struct {
	ctx    context.Context
	cancel context.CancelFunc
	call   LLMCall

	spinner spinner.Model
	loading bool
	err     error
	res     Result

	outcome Outcome
}

type reviewDoneMsg struct {
	res Result
	err error
}

func newReviewModel(ctx context.Context, call LLMCall) reviewModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#cba6f7"))

	cctx, cancel := context.WithCancel(ctx)
	return reviewModel{
		ctx:     cctx,
		cancel:  cancel,
		call:    call,
		spinner: sp,
		loading: true,
	}
}

func (m reviewModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.runCall())
}

func (m reviewModel) runCall() tea.Cmd {
	return func() tea.Msg {
		res, err := m.call(m.ctx)
		return reviewDoneMsg{res: res, err: err}
	}
}

func (m reviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case reviewDoneMsg:
		m.loading = false
		m.err = msg.err
		m.res = msg.res
		if msg.err != nil {
			return m, tea.Quit
		}
		// If there's nothing actionable (chat answer with no suggestion), just
		// quit so main can print the answer plainly.
		if m.res.Command == "" && m.res.SuggestedCommand == "" {
			m.outcome = OutcomeCancel
			return m, tea.Quit
		}
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc {
				m.cancel()
				m.outcome = OutcomeCancel
				return m, tea.Quit
			}
			return m, nil
		}
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.outcome = OutcomeCancel
			return m, tea.Quit
		case tea.KeyEnter:
			m.outcome = OutcomeRun
			return m, tea.Quit
		case tea.KeyRunes:
			if len(msg.Runes) == 1 {
				switch msg.Runes[0] {
				case 'y', 'Y':
					m.outcome = OutcomeRun
					return m, tea.Quit
				case 'n', 'N', 'q':
					m.outcome = OutcomeCancel
					return m, tea.Quit
				}
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m reviewModel) View() string {
	if m.loading {
		return fmt.Sprintf("%s %s", m.spinner.View(), rvHint.Render("thinking…"))
	}
	if m.err != nil {
		return rvError.Render("error: " + m.err.Error())
	}

	var b strings.Builder

	// Chat answer (if present)
	if m.res.Answer != "" {
		b.WriteString(rvAnswer.Render(m.res.Answer))
		b.WriteString("\n\n")
	}

	cmd := m.res.Command
	if cmd == "" {
		cmd = m.res.SuggestedCommand
	}
	if cmd == "" {
		// Pure chat answer — no card, just the answer.
		return strings.TrimRight(b.String(), "\n")
	}

	card := rvCommand.Render(cmd)
	if m.res.Explanation != "" {
		card += "\n" + rvHint.Render(m.res.Explanation)
	}
	b.WriteString(rvTitle.Render("Suggested command"))
	b.WriteString("\n")
	b.WriteString(rvFrame.Render(card))
	b.WriteString("\n")
	b.WriteString(rvHint.Render("[enter/y] run · [esc/n] cancel"))
	return b.String()
}

// RunReview shows the review TUI: spinner while call runs, then a confirm
// card. Returns the outcome plus the LLM result so callers can act on it.
// Renders on stderr and reads input from /dev/tty so stdout/stdin pipes
// work seamlessly. Caller must guarantee a TTY is present.
func RunReview(ctx context.Context, call LLMCall) (Outcome, Result, error) {
	m := newReviewModel(ctx, call)
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr), tea.WithInput(reviewTTYInput()))
	final, err := p.Run()
	if err != nil {
		return OutcomeCancel, Result{}, err
	}
	fm := final.(reviewModel)
	if fm.err != nil {
		return OutcomeCancel, Result{}, fm.err
	}
	return fm.outcome, fm.res, nil
}

func reviewTTYInput() *os.File {
	if tty, err := os.Open("/dev/tty"); err == nil {
		return tty
	}
	return os.Stdin
}
