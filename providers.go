package main

import (
	"context"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
	openaioption "github.com/openai/openai-go/option"
)

const (
	claudeModel = "claude-opus-5"
	openaiModel = "gpt-5"
)

// claudeClient and openaiClient are initialized in main, once the config
// file (which may override their host URLs) has been loaded.
var (
	claudeClient anthropic.Client
	openaiClient openai.Client
)

// apiKeyEnvVar is the single environment variable Alfred reads credentials
// from, regardless of which chat engine (or host override) is active.
const apiKeyEnvVar = "ALFRED_API_KEY"

// initClients builds the API clients, pointing them at custom hosts from
// cfg when configured. All auth comes from ALFRED_API_KEY.
func initClients(cfg config) {
	apiKey := os.Getenv(apiKeyEnvVar)

	claudeOpts := []anthropicoption.RequestOption{anthropicoption.WithAPIKey(apiKey)}
	if cfg.ClaudeHost != "" {
		claudeOpts = append(claudeOpts, anthropicoption.WithBaseURL(cfg.ClaudeHost))
	}
	claudeClient = anthropic.NewClient(claudeOpts...)

	var openaiOpts []openaioption.RequestOption
	switch {
	case cfg.OpenAIAzureEndpoint != "":
		openaiOpts = append(openaiOpts,
			azure.WithEndpoint(cfg.OpenAIAzureEndpoint, cfg.OpenAIAzureAPIVersion),
			azure.WithAPIKey(apiKey),
		)
	default:
		openaiOpts = append(openaiOpts, openaioption.WithAPIKey(apiKey))
		if cfg.OpenAIHost != "" {
			openaiOpts = append(openaiOpts, openaioption.WithBaseURL(cfg.OpenAIHost))
		}
	}
	openaiClient = openai.NewClient(openaiOpts...)
}

// provider selects which chat engine a session talks to.
type provider string

const (
	providerClaude provider = "claude"
	providerOpenAI provider = "openai"
)

// codeFenceInstruction asks the model to format code as fenced Markdown
// blocks with a language tag, which is what codeFenceRe looks for when
// syntax-highlighting a reply.
const codeFenceInstruction = "You are a helpful assistant.. When your response includes code, always format it as a fenced Markdown code block with a language tag, e.g. ```go."

// fetchReply sends the conversation so far to the session's chat engine and
// returns its reply as an aiReplyMsg.
func fetchReply(history []message, prov provider) tea.Cmd {
	if prov == providerOpenAI {
		return fetchOpenAIReply(history)
	}
	return fetchClaudeReply(history)
}

// fetchClaudeReply sends the conversation so far to Claude. Local-only
// "system" notices (e.g. from the :w save command) are never sent.
func fetchClaudeReply(history []message) tea.Cmd {
	var msgs []anthropic.MessageParam
	for _, msg := range history {
		switch msg.Role {
		case "ai":
			msgs = append(msgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Text)))
		case "you":
			msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Text)))
		}
	}

	return func() tea.Msg {
		resp, err := claudeClient.Messages.New(context.Background(), anthropic.MessageNewParams{
			Model:     claudeModel,
			MaxTokens: 1024,
			System:    []anthropic.TextBlockParam{{Text: codeFenceInstruction}},
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

// fetchOpenAIReply sends the conversation so far to OpenAI. Local-only
// "system" notices (e.g. from the :w save command) are never sent.
func fetchOpenAIReply(history []message) tea.Cmd {
	msgs := []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(codeFenceInstruction)}
	for _, msg := range history {
		switch msg.Role {
		case "ai":
			msgs = append(msgs, openai.AssistantMessage(msg.Text))
		case "you":
			msgs = append(msgs, openai.UserMessage(msg.Text))
		}
	}

	return func() tea.Msg {
		resp, err := openaiClient.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
			Model:    openaiModel,
			Messages: msgs,
		})
		if err != nil {
			return aiReplyMsg("(error) " + err.Error())
		}
		if len(resp.Choices) == 0 {
			return aiReplyMsg("(error) empty response from OpenAI")
		}
		return aiReplyMsg(resp.Choices[0].Message.Content)
	}
}
