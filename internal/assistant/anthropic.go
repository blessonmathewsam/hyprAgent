package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/liushuangls/go-anthropic/v2"
)

// AnthropicProvider implements LLMProvider using the Anthropic API
type AnthropicProvider struct {
	client *anthropic.Client
	model  string
}

// NewAnthropicProvider creates a new Anthropic provider instance
func NewAnthropicProvider(apiKey string, model string) *AnthropicProvider {
	if model == "" {
		model = string(anthropic.ModelClaude3Dot5Sonnet20240620)
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
	
	return &AnthropicProvider{
		client: anthropic.NewClient(apiKey, anthropic.WithHTTPClient(httpClient)),
		model:  model,
	}
}

func (p *AnthropicProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (*Message, error) {
	var anthropicMessages []anthropic.Message
	var systemPrompt string

	// Extract system prompt if present (Anthropic sends it separately)
	for _, msg := range messages {
		if msg.Role == RoleSystem {
			systemPrompt += msg.Content + "\n"
			continue
		}

		role := anthropic.RoleUser
		if msg.Role == RoleAssistant {
			role = anthropic.RoleAssistant
		} else if msg.Role == RoleTool {
			// Anthropic handles tool results as User messages with specific content blocks
			role = anthropic.RoleUser
		}

		// Content construction
		// For simple text:
		content := []anthropic.MessageContent{
			anthropic.NewTextMessageContent(msg.Content),
		}

		// If this message has tool calls (Assistant output)
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				var input map[string]interface{}
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
				inputBytes, _ := json.Marshal(input)

				content = append(content, anthropic.NewToolUseMessageContent(tc.ID, tc.Function.Name, json.RawMessage(inputBytes)))
			}
		}

		// If this is a tool result (RoleTool)
		if msg.Role == RoleTool {
			content = []anthropic.MessageContent{
				anthropic.NewToolResultMessageContent(msg.ToolCallID, msg.Content, false),
			}
		}

		anthropicMessages = append(anthropicMessages, anthropic.Message{
			Role:    role,
			Content: content,
		})
	}

	// Define tools
	var anthropicTools []anthropic.ToolDefinition
	for _, t := range tools {
		// Ensure parameters are map[string]interface{}
		var params map[string]interface{}

		// Handle json.RawMessage
		if raw, ok := t.Parameters.(json.RawMessage); ok {
			_ = json.Unmarshal(raw, &params)
		} else if pMap, ok := t.Parameters.(map[string]interface{}); ok {
			params = pMap
		}

		anthropicTools = append(anthropicTools, anthropic.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: params,
		})
	}

	req := anthropic.MessagesRequest{
		Model:     anthropic.Model(p.model),
		Messages:  anthropicMessages,
		Tools:     anthropicTools,
		MaxTokens: 4096,
		System:    systemPrompt,
	}

	resp, err := p.client.CreateMessages(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("anthropic completion error: %w", err)
	}

	result := &Message{
		Role: RoleAssistant,
	}

	// Parse response
	for _, content := range resp.Content {
		if content.Type == anthropic.MessagesContentTypeText {
			result.Content += *content.Text
		} else if content.Type == anthropic.MessagesContentTypeToolUse {
			argsBytes, _ := json.Marshal(content.Input)
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:   content.ID,
				Type: "function",
				Function: FunctionCall{
					Name:      content.Name,
					Arguments: string(argsBytes),
				},
			})
		}
	}

	return result, nil
}
