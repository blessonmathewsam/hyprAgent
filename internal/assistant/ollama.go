package assistant

import (
	openai "github.com/sashabaranov/go-openai"
)

// NewOllamaProvider creates a new OpenAI provider configured for local Ollama
func NewOllamaProvider(host string, model string) *OpenAIProvider {
	if host == "" {
		host = "http://localhost:11434/v1"
	}
	if model == "" {
		model = "llama3" // Default to a reasonable local model
	}

	config := openai.DefaultConfig("ollama") // API Key is ignored by Ollama usually
	config.BaseURL = host

	// Initialize the OpenAIProvider with a new client based on the config
	return &OpenAIProvider{
		client: openai.NewClientWithConfig(config),
		model:  model,
	}
}
