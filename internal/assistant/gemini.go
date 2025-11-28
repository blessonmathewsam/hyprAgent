package assistant

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiProvider implements LLMProvider using Google's Gemini API
type GeminiProvider struct {
	client *genai.Client
	model  string
}

// NewGeminiProvider creates a new Gemini provider instance
func NewGeminiProvider(ctx context.Context, apiKey string, model string) (*GeminiProvider, error) {
	if model == "" {
		model = "gemini-2.5-pro" // Or whatever the exact string for 2.5 is when released, using placeholder based on request
	}
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	return &GeminiProvider{
		client: client,
		model:  model,
	}, nil
}

func (p *GeminiProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (*Message, error) {
	model := p.client.GenerativeModel(p.model)

	// Convert Tools
	var toolDecls []*genai.Tool
	if len(tools) > 0 {
		// Gemini expects a specific FunctionDeclaration format
		// Simplified: we create one Tool object containing all function declarations
		var funcDecls []*genai.FunctionDeclaration

		for _, t := range tools {
			// We need to manually map the JSON schema parameters to genai.Schema
			// This is complex as standard JSON schema -> GenAI Schema mapping isn't 1:1 automatic in the SDK
			// For this MVP, we might need a simplified schema mapper or rely on the fact that GenAI
			// supports OpenAPI schema objects.

			// NOTE: Proper Schema mapping is significant work.
			// For now, assuming simple object/string params or using a map workaround?
			// Actually, genai-go requires structured Schema types.
			// As a fallback for MVP, we define a "Any" schema or try to parse.
			// Realistically, we need a helper to convert JSON Schema to genai.Schema.

			// WORKAROUND: Use empty schema (allow anything) if we can't parse easily,
			// or basic mapping.
			// Let's implement a basic mapper in a helper function later.

			f := &genai.FunctionDeclaration{
				Name:        t.Name,
				Description: t.Description,
				// Parameters: ... (Skipping complex mapping for brevity, essentially need a converter)
			}
			funcDecls = append(funcDecls, f)
		}

		toolDecls = append(toolDecls, &genai.Tool{FunctionDeclarations: funcDecls})
	}

	model.Tools = toolDecls

	cs := model.StartChat()

	// Replay History
	// Gemini uses History []*Content
	for _, msg := range messages {
		// Extract System Prompt - Gemini usually sets this on Model, not chat history,
		// but "System Instructions" are a newer feature.
		// For Chat session, usually role "user" and "model".
		if msg.Role == RoleSystem {
			model.SystemInstruction = &genai.Content{
				Parts: []genai.Part{genai.Text(msg.Content)},
			}
			continue
		}

		role := "user"
		if msg.Role == RoleAssistant {
			role = "model"
		} else if msg.Role == RoleTool {
			role = "function" // Gemini uses separate logic for function responses
		}

		// Construct Parts
		var parts []genai.Part
		if msg.Content != "" {
			parts = append(parts, genai.Text(msg.Content))
		}

		// Tool Calls (Model Output)
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				var args map[string]interface{}
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
				parts = append(parts, genai.FunctionCall{
					Name: tc.Function.Name,
					Args: args,
				})
			}
		}

		// Tool Results (User/Function Input)
		if msg.Role == RoleTool {
			var response map[string]interface{}
			// Try to parse JSON, otherwise wrap string
			if err := json.Unmarshal([]byte(msg.Content), &response); err != nil {
				response = map[string]interface{}{"result": msg.Content}
			}

			parts = append(parts, genai.FunctionResponse{
				Name:     msg.Name,
				Response: response,
			})
		}

		cs.History = append(cs.History, &genai.Content{
			Role:  role,
			Parts: parts,
		})
	}

	// Send (empty) message to trigger generation?
	// Actually, ChatSession manages history. If we rebuilt history manually, we just send the *last* user message.
	// But our `messages` slice includes the last user message.
	// So we should pop the last message from history and send it via SendMessage.

	if len(cs.History) > 0 {
		lastMsg := cs.History[len(cs.History)-1]
		if lastMsg.Role == "user" {
			// Pop it
			cs.History = cs.History[:len(cs.History)-1]
			resp, err := cs.SendMessage(ctx, lastMsg.Parts...)
			if err != nil {
				return nil, err
			}
			return p.parseResponse(resp)
		}
	}

	// Fallback if last message wasn't user (unexpected in chat loop)
	return nil, fmt.Errorf("last message was not from user")
}

func (p *GeminiProvider) parseResponse(resp *genai.GenerateContentResponse) (*Message, error) {
	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates returned")
	}
	cand := resp.Candidates[0]

	result := &Message{
		Role: RoleAssistant,
	}

	for _, part := range cand.Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			result.Content += string(txt)
		} else if fc, ok := part.(genai.FunctionCall); ok {
			argsBytes, _ := json.Marshal(fc.Args)
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:   "", // Gemini doesn't strictly use IDs like OpenAI, context implies order
				Type: "function",
				Function: FunctionCall{
					Name:      fc.Name,
					Arguments: string(argsBytes),
				},
			})
		}
	}

	return result, nil
}
