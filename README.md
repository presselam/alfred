# alfred
terminal ai assistant

Bubble Tea TUI: UUID header, scrollable chat history, auto-growing (1-5 line) input.

    go mod tidy
    go run .

Keys: Enter = send, Alt+Enter / Ctrl+J = newline, PgUp/PgDn or mouse wheel = scroll, Esc / Ctrl+C / `:q` = quit.

Replies come from the Claude API (`claude-opus-5`). Set `ANTHROPIC_API_KEY` in your environment before running.

Chat history is saved to `~/.alfred/sessions/<chat-id>.json` after every message. Resume a session with:

    go run . --id=<chat-id>
    go run . -i <chat-id>

Omit the flag to start a new session with a fresh id.
