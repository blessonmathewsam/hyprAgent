package assistant

import (
	"context"
)

// Role represents the role of a message sender
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message represents a single message in the conversation
type Message struct {
	Role       Role
	Content    string
	Name       string // Optional, used for tool responses
	ToolCalls  []ToolCall
	ToolCallID string // Used when Role is Tool to link back to the call
}

// ToolCall represents a request from the LLM to execute a tool
type ToolCall struct {
	ID       string
	Type     string
	Function FunctionCall
}

// FunctionCall represents the details of a function execution request
type FunctionCall struct {
	Name      string
	Arguments string // JSON string of arguments
}

// ToolDefinition defines a tool that can be used by the LLM
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  interface{} // JSON Schema describing the parameters
}

// LLMProvider defines the interface for interacting with LLM backends
type LLMProvider interface {
	// Chat sends messages to the LLM and returns the response, potentially including tool calls
	Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (*Message, error)
}

