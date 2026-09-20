package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

func main() {
	var idFlag string
	flag.StringVar(&idFlag, "id", "", "resume a chat session by id")
	flag.StringVar(&idFlag, "i", "", "resume a chat session by id (shorthand)")
	flag.Parse()

	chatID := idFlag
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

	initialMessage := strings.Join(flag.Args(), " ")
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
