# alfred
terminal ai assistant

Bubble Tea TUI: UUID header, scrollable chat history with bordered, syntax-highlighted code blocks, auto-growing (1-5 line) input.

    go mod tidy
    go run .

Keys: Enter = send, Alt+Enter / Ctrl+J = newline, PgUp/PgDn or mouse wheel = scroll, Ctrl+Y = copy chat id to clipboard, Esc / Ctrl+C / `:q` = quit.

Type `:w <filename>` and press Enter to save the code block(s) from the last AI response to disk, in the current directory. A single code block is written to `<filename>` exactly; multiple blocks are numbered against it, e.g. `:w quick.pl` with two blocks writes `quick-0.pl` and `quick-1.pl`. A short status line (e.g. `saved quick.pl`) is appended to the chat - it's local only and never sent to the model.

Type `:image <prompt>` to generate an image (via OpenAI's `gpt-image-1` - Claude has no image-generation API, so this requires the session's engine to be `openai`; trying it under Claude pops up an error) and save it to `image-<timestamp>.png` in the current directory. The request is echoed into the chat immediately, with an animated "generating image..." indicator underneath while it's in flight, replaced by a status line (e.g. `saved image-20260101-120000.png`) once it's done. Set `image_viewer` in `~/.alfred/config.yaml` to also open it automatically, e.g. `image_viewer: open` (macOS) or `image_viewer: xdg-open` (Linux) - it's run as `<image_viewer> <path>` without waiting for it to exit.

Any input starting with `:` is treated as a command, not a chat message. Forgetting an argument (`:w`, `:image`), having no code blocks in the last response, or typing an unrecognized command (e.g. `:x`) pops up a dismissible error dialog - the rest of the chat stays visible behind it; press any key (other than Ctrl+C, which still quits) to close it. Anything that doesn't start with `:` is always sent as a normal chat message.

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

A resumed session keeps using whichever engine it was started with, regardless of the config file. Omit `--id` to start a new session with a fresh id. Passing an `--id` that doesn't match any saved session is an error (exit status 1) rather than silently starting a new, empty chat under that id.

By default session files are kept forever. Set `session_retention_days` in `~/.alfred/config.yaml` to automatically delete session files that haven't been touched in that many days, checked once on every startup:

    session_retention_days: 30

The session you're currently starting or resuming is never deleted by this, no matter how old it is.

Any other command-line arguments are joined with spaces and sent as the first message, so you can jump straight into a chat from the shell. `--id`/`-i` can go anywhere on the line - before, after, or in the middle of the message:

    go run . what is the capital of France?
    go run . -i <chat-id> can you rewrite that in Python?
    go run . can you rewrite that in Python? --id <chat-id>
