package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/openai/openai-go"
)

const imageModel = "gpt-image-1"

// imageResultMsg reports the outcome of an :image command's async
// generation step. A non-empty Err shows the error popup; otherwise Note is
// appended to the chat as a "system" status message.
type imageResultMsg struct {
	Note string
	Err  string
}

// handleImageCommand validates a ":image <prompt>" command synchronously -
// only OpenAI can generate images, so a Claude-provider session is a
// synchronous error, same as a missing prompt. On success it echoes text
// (the raw ":image <prompt>" input) into the chat, starts the progress
// spinner, and returns a tea.Cmd that performs the actual (network)
// generation asynchronously.
func (m *model) handleImageCommand(text, prompt string) tea.Cmd {
	if prompt == "" {
		m.errorMsg = "usage: :image <prompt>"
		return nil
	}
	if m.provider != providerOpenAI {
		m.errorMsg = "image generation is not supported by the claude engine"
		return nil
	}
	m.messages = append(m.messages, message{roleImageRequest, text})
	m.generatingImage = true
	viewer := m.imageViewer
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		return generateImage(prompt, viewer)
	})
}

// generateImage calls OpenAI's image API, saves the result to a timestamped
// PNG in the current directory, and - if viewer is set - launches it.
func generateImage(prompt, viewer string) imageResultMsg {
	resp, err := openaiClient.Images.Generate(context.Background(), openai.ImageGenerateParams{
		Model:  imageModel,
		Prompt: prompt,
	})
	if err != nil {
		return imageResultMsg{Err: "image generation failed: " + err.Error()}
	}
	if len(resp.Data) == 0 || resp.Data[0].B64JSON == "" {
		return imageResultMsg{Err: "image generation returned no image"}
	}
	raw, err := base64.StdEncoding.DecodeString(resp.Data[0].B64JSON)
	if err != nil {
		return imageResultMsg{Err: "could not decode generated image: " + err.Error()}
	}

	path := fmt.Sprintf("image-%s.png", time.Now().Format("20060102-150405"))
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return imageResultMsg{Err: "could not save image: " + err.Error()}
	}

	note := "saved " + path
	if viewer != "" {
		if err := launchViewer(viewer, path); err != nil {
			note += fmt.Sprintf(" (could not launch viewer: %v)", err)
		}
	}
	return imageResultMsg{Note: note}
}

// launchViewer starts the configured image viewer command with path
// appended as its final argument, without waiting for it to exit.
func launchViewer(viewer, path string) error {
	parts := strings.Fields(viewer)
	if len(parts) == 0 {
		return fmt.Errorf("empty image_viewer command")
	}
	args := append(append([]string{}, parts[1:]...), path)
	return exec.Command(parts[0], args...).Start()
}
