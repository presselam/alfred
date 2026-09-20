package main

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/charmbracelet/lipgloss"
)

// codeFenceRe matches fenced code blocks, capturing the language (the
// leading token of the info string - any trailing text on that line, such
// as a filename hint, is ignored) and the code body.
var codeFenceRe = regexp.MustCompile("(?s)```([a-zA-Z0-9_+-]*)[^\\n]*\\n(.*?)```")

const (
	chromaFormatter = "terminal256"
	chromaStyle     = "monokai"
)

// highlightCode syntax-highlights a code block for terminal display. If lang
// is empty or unrecognized, chroma falls back to auto-detection and then
// plain text.
func highlightCode(code, lang string) string {
	code = strings.TrimSuffix(code, "\n")
	var buf bytes.Buffer
	if err := quick.Highlight(&buf, code, lang, chromaFormatter, chromaStyle); err != nil {
		return code
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// renderCodeBlock syntax-highlights a code block and frames it in a bordered
// box sized to fit within width (the border itself costs 2 columns).
func renderCodeBlock(code, lang string, width int) string {
	contentWidth := width - 2
	if contentWidth < 1 {
		contentWidth = 1
	}
	return codeBoxStyle.Width(contentWidth).Render(highlightCode(code, lang))
}

// renderMessageBody applies style to a message's plain-text portions while
// syntax-highlighting and bordering any fenced code blocks it contains. width
// is the space available for the message (used to size code block borders).
func renderMessageBody(text string, style lipgloss.Style, width int) string {
	matches := codeFenceRe.FindAllStringSubmatchIndex(text, -1)
	if matches == nil {
		return style.Render(text)
	}

	var b strings.Builder
	last := 0
	for _, loc := range matches {
		if loc[0] > last {
			b.WriteString(style.Render(text[last:loc[0]]))
		}
		lang, code := text[loc[2]:loc[3]], text[loc[4]:loc[5]]
		b.WriteString(renderCodeBlock(code, lang, width))
		last = loc[1]
	}
	if last < len(text) {
		b.WriteString(style.Render(text[last:]))
	}
	return b.String()
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
		b.WriteString(wrap.Render(renderMessageBody(msg.Text, style, m.viewport.Width)))
	}
	if len(m.messages) == 0 {
		b.WriteString(dimStyle.Render("No messages yet."))
	}
	m.viewport.SetContent(b.String())
	m.viewport.GotoBottom()
}
