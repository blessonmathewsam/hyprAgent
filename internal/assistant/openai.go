package assistant

import (
	"context"
	"fmt"
	"net/http"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements LLMProvider using the OpenAI API
type OpenAIProvider struct {
	client *openai.Client
	model  string
}

// NewOpenAIProvider creates a new OpenAI provider instance
func NewOpenAIProvider(apiKey string, model string) *OpenAIProvider {
	if model == "" {
		model = openai.GPT5Mini
	}

	// Create HTTP client with proper timeouts
	httpClient := &http.Client{
		Timeout: 120 * time.Second, // 2 minute timeout for API calls
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	config := openai.DefaultConfig(apiKey)
	config.HTTPClient = httpClient

	return &OpenAIProvider{
		client: openai.NewClientWithConfig(config),
		model:  model,
	}
}

// Chat sends messages to the LLM and returns the response
func (p *OpenAIProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (*Message, error) {
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Simple exponential backoff: 1s, 2s, 4s
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}

		apiMessages := make([]openai.ChatCompletionMessage, len(messages))
		for i, msg := range messages {
			role := openai.ChatMessageRoleUser
			switch msg.Role {
			case RoleSystem:
				role = openai.ChatMessageRoleSystem
			case RoleAssistant:
				role = openai.ChatMessageRoleAssistant
			case RoleTool:
				role = openai.ChatMessageRoleTool
			}

			var toolCalls []openai.ToolCall
			if len(msg.ToolCalls) > 0 {
				toolCalls = make([]openai.ToolCall, len(msg.ToolCalls))
				for j, tc := range msg.ToolCalls {
					toolCalls[j] = openai.ToolCall{
						ID:   tc.ID,
						Type: openai.ToolType(tc.Type),
						Function: openai.FunctionCall{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				}
			}

			// Fix: OpenAI requires Content to be non-null for Assistant messages,
			// unless there are tool calls. However, some messages might just be empty tool results?
			// No, actually, if Role is Assistant and it has ToolCalls, Content can be null.
			// BUT, if Role is Tool, Content CANNOT be null.
			content := msg.Content
			if role == openai.ChatMessageRoleTool && content == "" {
				content = "{}" // Return empty JSON object if content is empty for tool
			}
			// Also, for Assistant role, if ToolCalls is present, Content is optional in API but
			// the Go library might treat empty string as "" which is fine.
			// The error "Invalid value for 'content': expected a string, got null" often comes
			// from sending nil where a string is expected, or vice versa.
			// The go-openai library handles string fields, so empty string is "".
			// However, if the previous assistant message had tool calls and NO content, we must ensure
			// we send it back exactly like that.

			apiMessages[i] = openai.ChatCompletionMessage{
				Role:       role,
				Content:    content,
				Name:       msg.Name,
				ToolCalls:  toolCalls,
				ToolCallID: msg.ToolCallID,
			}
		}

		var apiTools []openai.Tool
		if len(tools) > 0 {
			apiTools = make([]openai.Tool, len(tools))
			for i, t := range tools {
				apiTools[i] = openai.Tool{
					Type: openai.ToolTypeFunction,
					Function: &openai.FunctionDefinition{
						Name:        t.Name,
						Description: t.Description,
						Parameters:  t.Parameters,
					},
				}
			}
		}

		req := openai.ChatCompletionRequest{
			Model:    p.model,
			Messages: apiMessages,
			Tools:    apiTools,
		}

		resp, err := p.client.CreateChatCompletion(ctx, req)
		if err != nil {
			lastErr = err
			// If context canceled or deadline exceeded, stop retrying immediately
			if ctx.Err() != nil {
				return nil, fmt.Errorf("openai completion error (context): %w", ctx.Err())
			}
			continue // Retry on other errors
		}

		choice := resp.Choices[0]
		msg := choice.Message

		result := &Message{
			Role:    RoleAssistant, // OpenAI responses are always assistant
			Content: msg.Content,
		}

		if len(msg.ToolCalls) > 0 {
			result.ToolCalls = make([]ToolCall, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				result.ToolCalls[i] = ToolCall{
					ID:   tc.ID,
					Type: string(tc.Type),
					Function: FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}

		return result, nil
	}

	return nil, fmt.Errorf("openai completion failed after %d attempts: %w", maxRetries, lastErr)
}
