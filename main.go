package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

const claudeModel = "claude-opus-5"

var claudeClient = anthropic.NewClient()

const (
	maxInputLines = 5
	headerHeight  = 1
	borderSize    = 2 // top + bottom (or left + right) of a simple border
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Reverse(true).Padding(0, 1)
	boxStyle    = lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	youStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	aiStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	dimStyle    = lipgloss.NewStyle().Faint(true)
)

type message struct {
	Role string `json:"role"` // "you" or "ai"
	Text string `json:"text"`
}

type aiReplyMsg string

type model struct {
	chatID   string
	messages []message
	viewport viewport.Model
	input    textarea.Model
	width    int
	height   int
	ready    bool
}

func newModel(chatID string, history []message) model {
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

	case tea.MouseMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
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
			return m, fetchReply(m.messages)
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

// refreshChat re-renders the transcript into the viewport and scrolls to end.
func (m *model) refreshChat() {
	wrap := lipgloss.NewStyle().Width(m.viewport.Width)
	var b strings.Builder
	for i, msg := range m.messages {
		if i > 0 {
			b.WriteString("\n")
		}
		style := youStyle
		if msg.Role == "ai" {
			style = aiStyle
		}
		b.WriteString(wrap.Render(style.Render(msg.Text)))
	}
	if len(m.messages) == 0 {
		b.WriteString(dimStyle.Render("No messages yet."))
	}
	m.viewport.SetContent(b.String())
	m.viewport.GotoBottom()
}

func (m model) View() string {
	if !m.ready {
		return "Loading..."
	}
	header := headerStyle.Width(m.width).MaxHeight(1).Render("Alfred -- " + m.chatID)
	chat := boxStyle.Render(m.viewport.View())
	input := boxStyle.Render(m.input.View())
	return lipgloss.JoinVertical(lipgloss.Left, header, chat, input)
}

// fetchReply sends the conversation so far to Claude and returns its reply
// as an aiReplyMsg.
func fetchReply(history []message) tea.Cmd {
	msgs := make([]anthropic.MessageParam, len(history))
	for i, msg := range history {
		block := anthropic.NewTextBlock(msg.Text)
		if msg.Role == "ai" {
			msgs[i] = anthropic.NewAssistantMessage(block)
		} else {
			msgs[i] = anthropic.NewUserMessage(block)
		}
	}

	return func() tea.Msg {
		resp, err := claudeClient.Messages.New(context.Background(), anthropic.MessageNewParams{
			Model:     claudeModel,
			MaxTokens: 1024,
			Messages:  msgs,
		})
		if err != nil {
			return aiReplyMsg("(error) " + err.Error())
		}
		var b strings.Builder
		for _, block := range resp.Content {
			if text, ok := block.AsAny().(anthropic.TextBlock); ok {
				b.WriteString(text.Text)
			}
		}
		return aiReplyMsg(b.String())
	}
}

// sessionsDir returns the directory where chat session files are stored,
// creating it if necessary.
func sessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".alfred", "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// sessionPath returns the file path for a given chat id.
func sessionPath(chatID string) (string, error) {
	dir, err := sessionsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filepath.Base(chatID)+".json"), nil
}

// loadSession reads a previously saved chat history for chatID. A missing
// file is not an error - it just means there's no history yet.
func loadSession(chatID string) ([]message, error) {
	path, err := sessionPath(chatID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var history []message
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}
	return history, nil
}

// saveSession persists the chat history to disk, best-effort.
func (m *model) saveSession() {
	path, err := sessionPath(m.chatID)
	if err != nil {
		return
	}
	data, err := json.MarshalIndent(m.messages, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

func main() {
	var idFlag string
	flag.StringVar(&idFlag, "id", "", "resume a chat session by id")
	flag.StringVar(&idFlag, "i", "", "resume a chat session by id (shorthand)")
	flag.Parse()

	chatID := idFlag
	if chatID == "" {
		chatID = uuid.NewString()
	}

	history, err := loadSession(chatID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not load session:", err)
	}

	p := tea.NewProgram(newModel(chatID, history), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
