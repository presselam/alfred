package main

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// codeFenceRe matches fenced code blocks, capturing the language (the
// leading token of the info string - any trailing text on that line, such
// as a filename hint, is ignored) and the code body.
var codeFenceRe = regexp.MustCompile("(?s)```([a-zA-Z0-9_+-]*)[^\\n]*\\n(.*?)```")

const (
	chromaFormatter = "terminal256"
	chromaStyle     = "monokai"
)

// codeBlock is one fenced code block extracted from a message.
type codeBlock struct {
	lang string
	code string
}

// extractCodeBlocks returns every fenced code block in text, in order.
func extractCodeBlocks(text string) []codeBlock {
	matches := codeFenceRe.FindAllStringSubmatch(text, -1)
	blocks := make([]codeBlock, len(matches))
	for i, match := range matches {
		blocks[i] = codeBlock{lang: match[1], code: strings.TrimSuffix(match[2], "\n")}
	}
	return blocks
}

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

// overlay centers fg on top of bg within a width x height canvas, splicing
// each fg line into its corresponding bg line without disturbing existing
// ANSI styling on either side (bg's lines are assumed to already span
// width, which holds for this app's header/chat/input layout).
func overlay(bg, fg string, width, height int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	fgWidth := 0
	for _, l := range fgLines {
		if w := ansi.StringWidth(l); w > fgWidth {
			fgWidth = w
		}
	}
	fgHeight := len(fgLines)

	x := (width - fgWidth) / 2
	if x < 0 {
		x = 0
	}
	y := (height - fgHeight) / 2
	if y < 0 {
		y = 0
	}

	out := make([]string, len(bgLines))
	for i, bgLine := range bgLines {
		if i < y || i >= y+fgHeight {
			out[i] = bgLine
			continue
		}
		fgLine := fgLines[i-y]
		if w := ansi.StringWidth(fgLine); w < fgWidth {
			fgLine += strings.Repeat(" ", fgWidth-w)
		}
		left := ansi.Cut(bgLine, 0, x)
		right := ansi.Cut(bgLine, x+fgWidth, width)
		out[i] = left + fgLine + right
	}
	return strings.Join(out, "\n")
}

// styleForRole picks the display style for a message's role.
func styleForRole(role string) lipgloss.Style {
	switch role {
	case "ai":
		return aiStyle
	case "system", roleImageRequest:
		return dimStyle
	default:
		return youStyle
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
		style := styleForRole(msg.Role)
		b.WriteString(wrap.Render(renderMessageBody(msg.Text, style, m.viewport.Width)))
	}
	if len(m.messages) == 0 {
		b.WriteString(dimStyle.Render("No messages yet."))
	}
	if m.generatingImage {
		if len(m.messages) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(dimStyle.Render(m.spinner.View() + "generating image..."))
	}
	m.viewport.SetContent(b.String())
	m.viewport.GotoBottom()
}
