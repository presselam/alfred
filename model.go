package main

import (
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const copiedFlashDuration = 1500 * time.Millisecond

const (
	maxInputLines = 5
	headerHeight  = 1
	borderSize    = 2 // top + bottom (or left + right) of a simple border
)

var (
	headerStyle  = lipgloss.NewStyle().Bold(true).Reverse(true).Padding(0, 1)
	boxStyle     = lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	youStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	aiStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	dimStyle     = lipgloss.NewStyle().Faint(true)
	codeBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
)

// message is one turn of chat history.
type message struct {
	Role string // "you" or "ai"
	Text string
}

type aiReplyMsg string

type copiedFlashExpiredMsg struct{}

type model struct {
	chatID   string
	provider provider
	messages []message
	viewport viewport.Model
	input    textarea.Model
	width    int
	height   int
	ready    bool
	copied   bool
}

func newModel(chatID string, prov provider, history []message) model {
	ta := textarea.New()
	ta.Placeholder = "Type a message... (Enter to send, Alt+Enter for newline)"
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetHeight(1)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle() // no highlighted line
	ta.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter", "ctrl+j"))
	ta.Focus()

	return model{
		chatID:   chatID,
		provider: prov,
		messages: history,
		input:    ta,
		viewport: viewport.New(0, 0),
	}
}

func (m model) Init() tea.Cmd { return textarea.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.relayout()
		return m, nil

	case aiReplyMsg:
		m.messages = append(m.messages, message{"ai", string(msg)})
		m.refreshChat()
		m.saveSession()
		return m, nil

	case copiedFlashExpiredMsg:
		m.copied = false
		return m, nil

	case tea.MouseMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "ctrl+y":
			if err := clipboard.WriteAll(m.chatID); err != nil {
				return m, nil
			}
			m.copied = true
			return m, tea.Tick(copiedFlashDuration, func(time.Time) tea.Msg {
				return copiedFlashExpiredMsg{}
			})
		case "pgup", "pgdown", "ctrl+u", "ctrl+d":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		case "enter":
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				return m, nil
			}
			if text == ":q" {
				return m, tea.Quit
			}
			m.messages = append(m.messages, message{"you", text})
			m.input.Reset()
			m.relayout()
			m.refreshChat()
			m.saveSession()
			return m, fetchReply(m.messages, m.provider)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.relayout() // input may have grown/shrunk
	return m, tea.Batch(cmds...)
}

// inputLines estimates how many visual rows the current input needs,
// clamped to [1, maxInputLines].
func (m model) inputLines() int {
	w := m.input.Width()
	if w <= 0 {
		return 1
	}
	n := 0
	for _, line := range strings.Split(m.input.Value(), "\n") {
		n += lipgloss.Width(line)/w + 1
	}
	if n < 1 {
		n = 1
	}
	if n > maxInputLines {
		n = maxInputLines
	}
	return n
}

// relayout sizes the viewport and input to the current terminal size.
func (m *model) relayout() {
	if !m.ready {
		return
	}
	inner := m.width - borderSize
	if inner < 1 {
		inner = 1
	}
	m.input.SetWidth(inner)
	lines := m.inputLines()
	m.input.SetHeight(lines)

	inputBox := lines + borderSize
	chatBox := m.height - headerHeight - inputBox
	vpH := chatBox - borderSize
	if vpH < 1 {
		vpH = 1
	}

	atBottom := m.viewport.AtBottom()
	widthChanged := m.viewport.Width != inner
	m.viewport.Width = inner
	m.viewport.Height = vpH
	if widthChanged {
		m.refreshChat() // re-wrap text for the new width
	} else if atBottom {
		m.viewport.GotoBottom()
	}
}

func (m model) View() string {
	if !m.ready {
		return "Loading..."
	}
	headerText := "Alfred [" + string(m.provider) + "] -- " + m.chatID
	if m.copied {
		headerText += " (copied!)"
	}
	header := headerStyle.Width(m.width).MaxHeight(1).Render(headerText)
	chat := boxStyle.Render(m.viewport.View())
	input := boxStyle.Render(m.input.View())
	return lipgloss.JoinVertical(lipgloss.Left, header, chat, input)
}
