package main

import (
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// roleImageRequest tags an echoed ":image <prompt>" request. It's styled
// like "system" (dim) in refreshChat, grouping it visually with the
// progress spinner and save confirmation that follow it, and - like
// "system" - is excluded from what gets sent to the chat API, since no
// reply was ever generated for it.
const roleImageRequest = "image-request"

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
	errorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("196")).Padding(1, 3)
)

// message is one turn of chat history.
type message struct {
	Role string // "you", "ai", "system" (local-only status), or roleImageRequest
	Text string
}

type aiReplyMsg string

type copiedFlashExpiredMsg struct{}

type model struct {
	chatID          string
	provider        provider
	imageViewer     string // shell command to open a generated image, from config.yaml
	messages        []message
	viewport        viewport.Model
	input           textarea.Model
	width           int
	height          int
	ready           bool
	copied          bool
	errorMsg        string // non-empty shows a dismissible popup instead of the normal view
	spinner         spinner.Model
	generatingImage bool // true while an :image request is in flight
}

func newModel(chatID string, prov provider, imageViewer string, history []message) model {
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
		chatID:      chatID,
		provider:    prov,
		imageViewer: imageViewer,
		messages:    history,
		input:       ta,
		viewport:    viewport.New(0, 0),
		spinner:     spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
}

// hasPendingReply reports whether the last message is an unanswered "you"
// turn - e.g. an initial message passed on the command line, or a session
// that was interrupted before the reply came back.
func hasPendingReply(messages []message) bool {
	return len(messages) > 0 && messages[len(messages)-1].Role == "you"
}

// Init starts the cursor blinking and, if the session was loaded (or
// started via command-line arguments) with a trailing unanswered "you"
// message, immediately sends it off for a reply.
func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{textarea.Blink}
	if hasPendingReply(m.messages) {
		cmds = append(cmds, fetchReply(m.messages, m.provider))
	}
	return tea.Batch(cmds...)
}

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

	case imageResultMsg:
		m.generatingImage = false
		if msg.Err != "" {
			m.errorMsg = msg.Err
			m.refreshChat()
			return m, nil
		}
		m.messages = append(m.messages, message{"system", msg.Note})
		m.refreshChat()
		m.saveSession()
		return m, nil

	case spinner.TickMsg:
		if !m.generatingImage {
			return m, nil // generation finished (or never started) - stop ticking
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		m.refreshChat()
		return m, cmd

	case tea.MouseMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if m.errorMsg != "" {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			m.errorMsg = "" // any other key dismisses the popup
			return m, nil
		}

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
			if strings.HasPrefix(text, ":") {
				m.input.Reset()
				var cmd tea.Cmd
				switch {
				case text == ":q":
					return m, tea.Quit
				case text == ":w" || strings.HasPrefix(text, ":w "):
					m.handleSaveCommand(strings.TrimSpace(strings.TrimPrefix(text, ":w")))
				case text == ":image" || strings.HasPrefix(text, ":image "):
					cmd = m.handleImageCommand(text, strings.TrimSpace(strings.TrimPrefix(text, ":image")))
				default:
					m.errorMsg = "unknown command: " + text
				}
				m.relayout()
				m.refreshChat()
				m.saveSession()
				return m, cmd
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
	background := lipgloss.JoinVertical(lipgloss.Left, header, chat, input)

	if m.errorMsg == "" {
		return background
	}
	box := errorStyle.Render(m.errorMsg)
	hint := dimStyle.Render("press any key to continue")
	popup := lipgloss.JoinVertical(lipgloss.Center, box, hint)
	return overlay(background, popup, m.width, m.height)
}
