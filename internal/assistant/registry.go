package assistant

import (
	"encoding/json"
)

// Tool defines the interface for a tool
type Tool interface {
	Definition() ToolDefinition
	Execute(args string) (string, error)
}

// ToolRegistry manages the available tools
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry
func (r *ToolRegistry) Register(t Tool) {
	r.tools[t.Definition().Name] = t
}

// Get retrieves a tool by name
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Definitions returns the definitions of all registered tools
func (r *ToolRegistry) Definitions() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}
	return defs
}

// Helper to parse args
func ParseArgs(args string, v interface{}) error {
	return json.Unmarshal([]byte(args), v)
}
