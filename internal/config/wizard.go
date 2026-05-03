package config

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	openrouter "github.com/hra42/openrouter-go"
	"github.com/sahilm/fuzzy"
)

// recommendedModels is the curated shortlist shown when the picker has no
// query. IDs are validated against the live OpenRouter listing — entries that
// don't resolve are silently dropped.
var recommendedModels = []string{
	"google/gemini-3-flash-preview",
	"google/gemini-3.1-flash-lite-preview",
	"anthropic/claude-haiku-4.5",
	"openai/gpt-5.4-nano",
	"openai/gpt-5.4-mini",
}

const pickerPageSize = 10

type pickerHit struct {
	id      string
	matches []int
}

type pickerModel struct {
	all         []string
	recommended []string
	input       textinput.Model
	results     []pickerHit
	cursor      int
}

func newPickerModel(all, recommended []string) pickerModel {
	ti := textinput.New()
	ti.Placeholder = "type to filter…"
	ti.Focus()
	ti.Prompt = "› "
	ti.CharLimit = 80

	m := pickerModel{
		all:         all,
		recommended: recommended,
		input:       ti,
	}
	m.refresh()
	return m
}

func (m pickerModel) Init() tea.Cmd { return textinput.Blink }

func (m *pickerModel) refresh() {
	q := strings.TrimSpace(m.input.Value())
	if q == "" {
		m.results = make([]pickerHit, 0, len(m.recommended))
		for _, id := range m.recommended {
			m.results = append(m.results, pickerHit{id: id})
		}
	} else {
		matches := fuzzy.Find(q, m.all)
		m.results = make([]pickerHit, 0, len(matches))
		for _, mt := range matches {
			m.results = append(m.results, pickerHit{id: mt.Str, matches: mt.MatchedIndexes})
		}
	}
	if m.cursor >= len(m.results) {
		m.cursor = len(m.results) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// Result is what the wizard hands back to main: the resolved API key, the
// updated config (with whichever fields the user filled in), and the chosen
// model id. Empty fields mean "user already had this configured, we didn't
// touch it."
type Result struct {
	APIKey string
	Model  string
	Cfg    Config
}

var (
	wzTitle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#cba6f7")).Bold(true)
	wzSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1")).Bold(true)
	wzHint     = lipgloss.NewStyle().Foreground(lipgloss.Color("#bac2de"))
	wzError    = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")).Bold(true)
	wzMatch    = lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af")).Bold(true)
)

type wizardStep int

const (
	stepKeySource wizardStep = iota
	stepKeyEntry             // plaintext
	stepOpRef                // 1Password reference
	stepModel
	stepDone
)

type keySource int

const (
	srcPlaintext keySource = iota
	srcOpRef
	srcEnv
)

var keySourceLabels = []string{
	"Save plaintext in ~/.config/ai/config.yaml",
	"Read from 1Password (secret reference)",
	"Use $OPENROUTER_API_KEY env var only",
}

type wizardModel struct {
	ctx context.Context

	step      wizardStep
	stepsTodo []wizardStep

	// Step 1: key source
	srcCursor int
	srcChosen keySource

	// Step 2/3: text input
	input textinput.Model

	// Step 4: model picker
	allModels         []string
	recommendedModels []string
	picker            *pickerModel

	// Output
	apiKey string
	cfg    Config
	model  string
	errMsg string

	cancelled bool
}

func newWizard(ctx context.Context, cfg Config, needKey, needModel bool) wizardModel {
	m := wizardModel{ctx: ctx, cfg: cfg}
	if needKey {
		m.stepsTodo = append(m.stepsTodo, stepKeySource)
	}
	if needModel {
		m.stepsTodo = append(m.stepsTodo, stepModel)
	}
	m.advance()
	return m
}

func (m *wizardModel) advance() {
	if len(m.stepsTodo) == 0 {
		m.step = stepDone
		return
	}
	m.step = m.stepsTodo[0]
	m.stepsTodo = m.stepsTodo[1:]
	m.errMsg = ""
}

func (m wizardModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		if k.Type == tea.KeyCtrlC {
			m.cancelled = true
			return m, tea.Quit
		}
	}

	switch m.step {
	case stepKeySource:
		return m.updateKeySource(msg)
	case stepKeyEntry:
		return m.updateTextInput(msg, true)
	case stepOpRef:
		return m.updateTextInput(msg, false)
	case stepModel:
		return m.updateModelPicker(msg)
	}
	return m, nil
}

func (m wizardModel) updateKeySource(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyEsc:
			m.cancelled = true
			return m, tea.Quit
		case tea.KeyUp, tea.KeyCtrlP:
			if m.srcCursor > 0 {
				m.srcCursor--
			}
		case tea.KeyDown, tea.KeyCtrlN:
			if m.srcCursor < len(keySourceLabels)-1 {
				m.srcCursor++
			}
		case tea.KeyEnter:
			m.srcChosen = keySource(m.srcCursor)
			switch m.srcChosen {
			case srcPlaintext:
				m.input = newSecretInput("API key")
				m.step = stepKeyEntry
			case srcOpRef:
				m.input = newPlainInput("op://Vault/Item/field")
				m.step = stepOpRef
			case srcEnv:
				m.errMsg = "set $OPENROUTER_API_KEY in your shell and re-run"
				m.cancelled = true
				return m, tea.Quit
			}
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m wizardModel) updateTextInput(msg tea.Msg, isPlaintextKey bool) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyEsc:
			// Back to source choice.
			m.step = stepKeySource
			m.errMsg = ""
			return m, nil
		case tea.KeyEnter:
			val := strings.Trim(strings.TrimSpace(m.input.Value()), `"'`)
			if val == "" {
				m.errMsg = "value cannot be empty"
				return m, nil
			}
			if isPlaintextKey {
				m.apiKey = val
				m.cfg.APIKey = val
				m.cfg.OpRef = ""
				m.advance()
				return m.maybeInitModel()
			}
			// 1Password reference path
			if !strings.HasPrefix(val, "op://") {
				m.errMsg = "reference must start with op://"
				return m, nil
			}
			key, err := ResolveOpRef(m.ctx, val)
			if err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			m.apiKey = key
			m.cfg.OpRef = val
			m.cfg.APIKey = ""
			m.advance()
			return m.maybeInitModel()
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// maybeInitModel transitions into the model picker if it's next, otherwise
// quits. Returns a Cmd to load models when entering the picker.
func (m wizardModel) maybeInitModel() (tea.Model, tea.Cmd) {
	if m.step != stepModel {
		return m, tea.Quit
	}
	return m, m.loadModelsCmd()
}

func (m wizardModel) loadModelsCmd() tea.Cmd {
	return func() tea.Msg {
		client := openrouter.NewClient(openrouter.WithAPIKey(m.apiKey))
		resp, err := client.ListModels(m.ctx, nil)
		if err != nil {
			return modelsLoadedMsg{err: err}
		}
		ids := make([]string, 0, len(resp.Data))
		set := make(map[string]struct{}, len(resp.Data))
		for _, mm := range resp.Data {
			ids = append(ids, mm.ID)
			set[mm.ID] = struct{}{}
		}
		var rec []string
		for _, id := range recommendedModels {
			if _, ok := set[id]; ok {
				rec = append(rec, id)
			}
		}
		return modelsLoadedMsg{all: ids, rec: rec}
	}
}

type modelsLoadedMsg struct {
	all []string
	rec []string
	err error
}

func (m wizardModel) updateModelPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	if loaded, ok := msg.(modelsLoadedMsg); ok {
		if loaded.err != nil {
			m.errMsg = "list models: " + loaded.err.Error()
			m.cancelled = true
			return m, tea.Quit
		}
		m.allModels = loaded.all
		m.recommendedModels = loaded.rec
		pm := newPickerModel(m.allModels, m.recommendedModels)
		m.picker = &pm
		return m, pm.Init()
	}

	if m.picker == nil {
		// Still loading.
		return m, nil
	}

	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyEsc:
			m.cancelled = true
			return m, tea.Quit
		case tea.KeyEnter:
			if len(m.picker.results) > 0 {
				m.model = m.picker.results[m.picker.cursor].id
				m.cfg.Model = m.model
				m.advance()
				return m, tea.Quit
			}
			return m, nil
		case tea.KeyUp, tea.KeyCtrlP:
			if m.picker.cursor > 0 {
				m.picker.cursor--
			}
			return m, nil
		case tea.KeyDown, tea.KeyCtrlN:
			if m.picker.cursor < len(m.picker.results)-1 {
				m.picker.cursor++
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.picker.input, cmd = m.picker.input.Update(msg)
	m.picker.refresh()
	return m, cmd
}

func (m wizardModel) View() string {
	switch m.step {
	case stepKeySource:
		return m.viewKeySource()
	case stepKeyEntry:
		return m.viewTextInput("Enter your OpenRouter API key (input hidden):")
	case stepOpRef:
		return m.viewTextInput("1Password secret reference:")
	case stepModel:
		return m.viewModel()
	}
	return ""
}

func (m wizardModel) viewKeySource() string {
	var b strings.Builder
	b.WriteString(wzTitle.Render("Setup — API key source"))
	b.WriteString("\n\n")
	for i, label := range keySourceLabels {
		if i == m.srcCursor {
			fmt.Fprintf(&b, "%s %s\n", wzSelected.Render("›"), wzSelected.Render(label))
		} else {
			fmt.Fprintf(&b, "  %s\n", label)
		}
	}
	b.WriteString("\n")
	if m.errMsg != "" {
		b.WriteString(wzError.Render(m.errMsg))
		b.WriteString("\n")
	}
	b.WriteString(wzHint.Render("↑/↓ move · enter pick · ctrl+c quit"))
	return b.String()
}

func (m wizardModel) viewTextInput(prompt string) string {
	var b strings.Builder
	b.WriteString(wzTitle.Render(prompt))
	b.WriteString("\n\n")
	b.WriteString(m.input.View())
	b.WriteString("\n\n")
	if m.errMsg != "" {
		b.WriteString(wzError.Render(m.errMsg))
		b.WriteString("\n")
	}
	b.WriteString(wzHint.Render("enter confirm · esc back · ctrl+c quit"))
	return b.String()
}

func (m wizardModel) viewModel() string {
	if m.picker == nil {
		return wzHint.Render("loading models…")
	}
	var b strings.Builder
	b.WriteString(wzTitle.Render("Setup — pick a default model"))
	b.WriteString("\n")
	b.WriteString(m.picker.input.View())
	b.WriteString("\n")
	if len(m.picker.results) == 0 {
		b.WriteString(wzHint.Render("  no matches"))
		b.WriteString("\n")
	} else {
		start := 0
		if m.picker.cursor >= pickerPageSize {
			start = m.picker.cursor - pickerPageSize + 1
		}
		end := start + pickerPageSize
		if end > len(m.picker.results) {
			end = len(m.picker.results)
		}
		for i := start; i < end; i++ {
			hit := m.picker.results[i]
			if i == m.picker.cursor {
				fmt.Fprintf(&b, "%s %s\n", wzSelected.Render("›"), wzSelected.Render(hit.id))
			} else {
				fmt.Fprintf(&b, "  %s\n", highlightWith(hit.id, hit.matches, wzMatch))
			}
		}
		if len(m.picker.results) > pickerPageSize {
			fmt.Fprintf(&b, "  %s\n", wzHint.Render(fmt.Sprintf("(%d/%d)", m.picker.cursor+1, len(m.picker.results))))
		}
	}
	b.WriteString(wzHint.Render("type to filter · ↑/↓ move · enter pick · esc cancel"))
	return b.String()
}

func newSecretInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.CharLimit = 200
	ti.Prompt = "› "
	ti.Focus()
	return ti
}

func newPlainInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 200
	ti.Prompt = "› "
	ti.Focus()
	return ti
}

func highlightWith(s string, idx []int, style lipgloss.Style) string {
	if len(idx) == 0 {
		return s
	}
	set := make(map[int]bool, len(idx))
	for _, i := range idx {
		set[i] = true
	}
	var b strings.Builder
	for i, r := range s {
		if set[i] {
			b.WriteString(style.Render(string(r)))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// RunSetup launches the unified TUI wizard for whatever steps are still
// missing. needKey/needModel let the caller skip already-completed steps.
// Returns ErrSetupCancelled when the user aborts.
func RunSetup(ctx context.Context, cfg Config, needKey, needModel bool) (Result, error) {
	if !needKey && !needModel {
		return Result{Cfg: cfg}, nil
	}
	m := newWizard(ctx, cfg, needKey, needModel)
	p := tea.NewProgram(m, tea.WithOutput(stderrForPicker()), tea.WithInput(stdinForPicker()))
	final, err := p.Run()
	if err != nil {
		return Result{}, err
	}
	fm := final.(wizardModel)
	if fm.cancelled {
		if fm.errMsg != "" {
			return Result{}, errors.New(fm.errMsg)
		}
		return Result{}, ErrSetupCancelled
	}
	return Result{APIKey: fm.apiKey, Model: fm.model, Cfg: fm.cfg}, nil
}

// ErrSetupCancelled is returned when the user aborts the wizard.
var ErrSetupCancelled = errors.New("setup cancelled")
