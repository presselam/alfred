# alfred
terminal ai assistant

Bubble Tea TUI: UUID header, scrollable chat history with bordered, syntax-highlighted code blocks, auto-growing (1-5 line) input.

    go mod tidy
    go run .

Keys: Enter = send, Alt+Enter / Ctrl+J = newline, PgUp/PgDn or mouse wheel = scroll, Ctrl+Y = copy chat id to clipboard, Esc / Ctrl+C / `:q` = quit.

Replies come from either Claude (`claude-opus-5`) or OpenAI (`gpt-5`). Both engines (and Azure OpenAI) authenticate with a single environment variable: `ALFRED_API_KEY`.

Pick the default engine for new sessions in `~/.alfred/config.yaml` (defaults to `claude` if the file or field is absent). You can also point either engine at a custom host, e.g. a proxy or self-hosted gateway:

    provider: openai
    claude_host: https://my-claude-proxy.example.com
    openai_host: https://my-openai-proxy.example.com/v1

Both `claude_host` and `openai_host` are optional and default to each provider's normal public API.

To use Azure OpenAI instead of the public OpenAI API, set an endpoint (this takes precedence over `openai_host`):

    openai_azure_endpoint: https://my-resource.openai.azure.com
    openai_azure_api_version: 2024-06-01

Azure auth uses the same `ALFRED_API_KEY` environment variable as everything else. Note that Azure OpenAI routes requests by *deployment name*, so your Azure deployment should be named `gpt-5` (or you'll need a way to override the model - not currently supported) to line up with the model this app requests.

Chat history (including the chosen engine) is saved in Go's binary `gob` encoding to `~/.alfred/sessions/<chat-id>.gob` after every message. Resume a session with:

    go run . --id=<chat-id>
    go run . -i <chat-id>

A resumed session keeps using whichever engine it was started with, regardless of the config file. Omit `--id` to start a new session with a fresh id.

Any other command-line arguments are joined with spaces and sent as the first message, so you can jump straight into a chat from the shell. `--id`/`-i` can go anywhere on the line - before, after, or in the middle of the message:

    go run . what is the capital of France?
    go run . -i <chat-id> can you rewrite that in Python?
    go run . can you rewrite that in Python? --id <chat-id>
