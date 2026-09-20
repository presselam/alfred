package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

// extractChatID pulls a -i/--id flag (as "-i VALUE" or "-i=VALUE", either
// dash style) out of args, wherever it appears, and returns its value plus
// the remaining arguments - which become the initial chat message. The
// standard library's flag package only recognizes flags before the first
// positional argument, which would silently swallow a trailing --id into
// the message instead of resuming that session.
func extractChatID(args []string) (chatID string, rest []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-i" || a == "--id":
			if i+1 < len(args) {
				chatID = args[i+1]
				i++
				continue
			}
		case strings.HasPrefix(a, "-i="):
			chatID = strings.TrimPrefix(a, "-i=")
			continue
		case strings.HasPrefix(a, "--id="):
			chatID = strings.TrimPrefix(a, "--id=")
			continue
		}
		rest = append(rest, a)
	}
	return chatID, rest
}

func main() {
	chatID, args := extractChatID(os.Args[1:])
	if chatID == "" {
		chatID = uuid.NewString()
	}

	session, err := loadSession(chatID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not load session:", err)
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not load config:", err)
	}
	initClients(cfg)

	prov := providerClaude
	switch {
	case session.Provider != "":
		prov = session.Provider
	case cfg.Provider != "":
		prov = cfg.Provider
	}
	if prov != providerClaude && prov != providerOpenAI {
		fmt.Fprintf(os.Stderr, "unknown provider %q in ~/.alfred/config.yaml (expected %q or %q)\n", prov, providerClaude, providerOpenAI)
		os.Exit(1)
	}

	initialMessage := strings.Join(args, " ")
	if initialMessage != "" {
		session.Messages = append(session.Messages, message{"you", initialMessage})
	}

	m := newModel(chatID, prov, session.Messages)
	if initialMessage != "" {
		m.saveSession()
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
