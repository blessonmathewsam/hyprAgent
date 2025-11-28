package assistant

import (
	"context"
	"fmt"
	"sync"

	"github.com/reinhart/hyprAgent/internal/logger"
)

// StatusUpdate represents a real-time update from the agent
type StatusUpdate struct {
	Message string
}

// Agent manages the conversation flow between the user, the LLM, and the tools
type Agent struct {
	provider LLMProvider
	registry *ToolRegistry
	history  []Message
	system   string
	updates  chan StatusUpdate // Channel for sending updates to UI
}

// NewAgent creates a new agent instance
func NewAgent(provider LLMProvider, registry *ToolRegistry, systemPrompt string) *Agent {
	agent := &Agent{
		provider: provider,
		registry: registry,
		history:  make([]Message, 0),
		system:   systemPrompt,
		updates:  make(chan StatusUpdate, 10), // Buffered channel
	}
	return agent
}

// Updates returns the channel for status updates
func (a *Agent) Updates() <-chan StatusUpdate {
	return a.updates
}

// sendUpdate sends a status update non-blocking
func (a *Agent) sendUpdate(msg string) {
	select {
	case a.updates <- StatusUpdate{Message: msg}:
	default:
		// Drop if channel full or no listener
	}
}

// ProcessMessage handles a user message and runs the agent loop
func (a *Agent) ProcessMessage(ctx context.Context, input string) (string, error) {
	logger.Info("Processing user input: %s", input)
	a.sendUpdate("Analysing request...")

	// If history is empty and we have a system prompt, add it first
	if len(a.history) == 0 && a.system != "" {
		a.history = append(a.history, Message{Role: RoleSystem, Content: a.system})
	}

	// Add user message to history
	a.history = append(a.history, Message{Role: RoleUser, Content: input})

	// Max turns loop to prevent infinite loops
	const maxTurns = 25
	for i := 0; i < maxTurns; i++ {
		logger.Debug("Agent Loop Turn: %d", i+1)

		// Call LLM
		a.sendUpdate(fmt.Sprintf("Thinking (Turn %d)...", i+1))
		logger.Debug("Sending request to LLM Provider...")
		resp, err := a.provider.Chat(ctx, a.history, a.registry.Definitions())
		if err != nil {
			logger.Info("LLM Error: %v", err)

			// Check if error is due to context timeout/cancellation
			if ctx.Err() == context.DeadlineExceeded {
				a.sendUpdate("Request timed out")
				return "", fmt.Errorf("LLM request timed out after waiting too long. The API may be slow or unavailable")
			} else if ctx.Err() == context.Canceled {
				a.sendUpdate("Request cancelled")
				return "", fmt.Errorf("request was cancelled")
			}

			a.sendUpdate("Error communicating with LLM")
			return "", err
		}
		logger.Debug("Received response from LLM (Content len: %d, ToolCalls: %d)", len(resp.Content), len(resp.ToolCalls))

		a.history = append(a.history, *resp)

		// If no tool calls, we are done
		if len(resp.ToolCalls) == 0 {
			logger.Info("Final response received")
			a.sendUpdate("Done")
			return resp.Content, nil
		}

		// Handle tool calls
		results := make([]Message, len(resp.ToolCalls))
		var wg sync.WaitGroup

		for i, tc := range resp.ToolCalls {
			wg.Add(1)
			go func(i int, tc ToolCall) {
				defer wg.Done()
				logger.Info("Tool Call Request: %s(%s)", tc.Function.Name, tc.Function.Arguments)

				// Update UI with specific action
				switch tc.Function.Name {
				case "detect_installation_root":
					a.sendUpdate("Detecting Hyprland installation...")
				case "list_dir":
					a.sendUpdate("Listing directory contents...")
				case "read_file":
					a.sendUpdate("Reading configuration file...")
				case "parse_config":
					a.sendUpdate("Parsing configuration structure...")
				case "make_patch":
					a.sendUpdate("Generating configuration patch...")
				case "apply_patch":
					a.sendUpdate("Requesting to apply patch...")
				}

				tool, ok := a.registry.Get(tc.Function.Name)
				if !ok {
					logger.Info("Error: Tool not found: %s", tc.Function.Name)
					results[i] = Message{
						Role:       RoleTool,
						ToolCallID: tc.ID,
						Name:       tc.Function.Name,
						Content:    fmt.Sprintf("Error: Tool %s not found", tc.Function.Name),
					}
					return
				}

				// Execute
				output, err := tool.Execute(tc.Function.Arguments)
				if err != nil {
					logger.Info("Tool Execution Error (%s): %v", tc.Function.Name, err)
					a.sendUpdate(fmt.Sprintf("Error in %s: %v", tc.Function.Name, err))
					// Include error in content so LLM knows
					output = fmt.Sprintf("Error: %v", err)
				} else {
					logger.Debug("Tool Output (%s): %s", tc.Function.Name, output)
					a.sendUpdate(fmt.Sprintf("Finished %s", tc.Function.Name))
				}

				results[i] = Message{
					Role:       RoleTool,
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
					Content:    output,
				}
			}(i, tc)
		}
		wg.Wait()

		// Append all results to history
		a.history = append(a.history, results...)

		// Loop continues to send tool results back to LLM
	}

	logger.Info("Agent loop limit reached")
	a.sendUpdate("Error: Loop limit reached")
	return "Error: Agent loop limit reached without final response. I got stuck trying to solve this.", nil
}

// Reset clears the conversation history
func (a *Agent) Reset() {
	a.history = make([]Message, 0)
}
