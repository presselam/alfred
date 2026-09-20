package main

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// config is the on-disk settings file at ~/.alfred/config.yaml.
type config struct {
	Provider   provider `yaml:"provider"`
	ClaudeHost string   `yaml:"claude_host"`
	OpenAIHost string   `yaml:"openai_host"`

	// OpenAIAzureEndpoint switches the OpenAI client to Azure OpenAI when
	// set (taking precedence over OpenAIHost). Auth still comes from the
	// AZURE_OPENAI_API_KEY environment variable, never from this file.
	OpenAIAzureEndpoint   string `yaml:"openai_azure_endpoint"`
	OpenAIAzureAPIVersion string `yaml:"openai_azure_api_version"`
}

// loadConfig reads ~/.alfred/config.yaml. A missing file is not an error -
// it just means the defaults apply.
func loadConfig() (config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return config{}, err
	}
	data, err := os.ReadFile(filepath.Join(home, ".alfred", "config.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return config{}, nil
		}
		return config{}, err
	}
	var cfg config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return config{}, err
	}
	return cfg, nil
}
