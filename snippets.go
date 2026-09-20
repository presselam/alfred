package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// lastAIMessage returns the text of the most recent "ai" message, if any.
func lastAIMessage(messages []message) (string, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "ai" {
			return messages[i].Text, true
		}
	}
	return "", false
}

// saveCodeBlocks writes the given code blocks to disk under filename. A
// single block is written to filename as-is. Multiple blocks are numbered:
// "quick.pl" becomes "quick-0.pl", "quick-1.pl", etc. It returns the paths
// written so far even when an error stops it partway through.
func saveCodeBlocks(blocks []codeBlock, filename string) ([]string, error) {
	if len(blocks) == 1 {
		if err := os.WriteFile(filename, []byte(blocks[0].code), 0o644); err != nil {
			return nil, err
		}
		return []string{filename}, nil
	}

	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	var written []string
	for i, b := range blocks {
		name := fmt.Sprintf("%s-%d%s", base, i, ext)
		if err := os.WriteFile(name, []byte(b.code), 0o644); err != nil {
			return written, err
		}
		written = append(written, name)
	}
	return written, nil
}

// handleSaveCommand implements the ":w <filename>" input command: it saves
// every code block in the last AI response to disk. Success (including
// partial success) appends a "system" message reporting what was written;
// failure - a missing filename, no code blocks, or a write error - shows
// an error popup instead.
func (m *model) handleSaveCommand(filename string) {
	if filename == "" {
		m.errorMsg = "usage: :w <filename>"
		return
	}

	text, ok := lastAIMessage(m.messages)
	blocks := extractCodeBlocks(text)
	if !ok || len(blocks) == 0 {
		m.errorMsg = "no code blocks found in the last response"
		return
	}

	written, err := saveCodeBlocks(blocks, filename)
	switch {
	case err != nil && len(written) == 0:
		m.errorMsg = "save failed: " + err.Error()
	case err != nil:
		m.messages = append(m.messages, message{"system", fmt.Sprintf("saved %s (stopped after error: %v)", strings.Join(written, ", "), err)})
	default:
		m.messages = append(m.messages, message{"system", "saved " + strings.Join(written, ", ")})
	}
}
