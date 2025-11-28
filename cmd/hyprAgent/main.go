package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/reinhart/hyprAgent/internal/assistant"
	"github.com/reinhart/hyprAgent/internal/configuration"
	"github.com/reinhart/hyprAgent/internal/logger"
	"github.com/reinhart/hyprAgent/internal/safety"
	"github.com/reinhart/hyprAgent/internal/ui"
)

func buildSystemPrompt(cfg *configuration.Config, backendType configuration.ConfigSourceType) string {
	var sec configuration.BackendSecurity
	switch backendType {
	case configuration.SourceNative:
		sec = cfg.Security.Native
	case configuration.SourceHyDE:
		sec = cfg.Security.Hyde
	case configuration.SourceOmarchy:
		sec = cfg.Security.Omarchy
	default:
		sec = cfg.Security.Native
	}

	allowedDirsStr := strings.Join(sec.AllowedDirs, ", ")
	allowedFilesStr := strings.Join(sec.AllowedFiles, ", ")

	return fmt.Sprintf(`You are HyprAgent, an expert assistant for configuring the Hyprland window manager.
Your goal is to help the user modify their Hyprland configuration safely and correctly.

ENVIRONMENT:
- Installation Type: %s
- Allowed Directories: %s
- Allowed Files: %s

SECURITY CONSTRAINTS:
- You can ONLY read/write files within the allowed directories and files listed above.
- Any attempt to access files outside these paths will be rejected.
- The configuration root is ~/.config/hypr/

GUIDELINES:
1. DETECTION: Start by using 'detect_installation_root' to understand the environment (Native, HyDE, Omarchy).
2. EXPLORATION: Use 'list_dir' and 'read_file' to locate relevant config files within allowed paths.
3. ANALYSIS: Read the config files to understand the current state.
4. PLANNING: Formulate a plan.
5. PATCHING PROTOCOL (IMPORTANT):
   - FIRST, use 'make_patch' to generate the diff.
   - STOP and show this diff to the user in your response.
   - ASK the user for confirmation (e.g., "Shall I apply this change?").
   - WAIT for the user to reply "Yes" or "Apply".
   - ONLY THEN use 'apply_patch' to execute the change.
   - DO NOT call 'apply_patch' in the same turn as 'make_patch'.
6. SAFETY:
   - The system automatically snapshots files before 'apply_patch'.
   - Verify that your generated config is valid Hyprland syntax.
7. ROLLBACK:
   - If the user says "undo", "revert", or "it broke", use the 'rollback' tool.
`, backendType, allowedDirsStr, allowedFilesStr)
}

func main() {
	// Load Configuration
	cfg, err := configuration.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Initialize Logger
	logger.Init()
	if cfg.Agent.Debug {
		logger.DebugMode = true
	}

	// If DEBUG is set, redirect logs to file immediately so we catch early init issues
	if logger.DebugMode {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			fmt.Println("fatal: could not open debug.log:", err)
			os.Exit(1)
		}
		defer f.Close()
		logger.SetOutput(f) // Redirect standard log to the file
		logger.Debug("Logger initialized")
	}

	// Provider Selection Logic (config takes precedence over env)
	providerType := cfg.LLM.Provider
	if envProvider := os.Getenv("LLM_PROVIDER"); envProvider != "" {
		providerType = envProvider
	}
	logger.Debug("Selected Provider: %s", providerType)

	var llm assistant.LLMProvider

	// Validate API key is available
	var apiKey string
	switch strings.ToLower(providerType) {
	case "anthropic":
		apiKey = cfg.LLM.AnthropicKey
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if apiKey == "" {
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println("❌ Error: ANTHROPIC_API_KEY not set")
			fmt.Println("")
			fmt.Println("Set it via environment variable:")
			fmt.Println("  export ANTHROPIC_API_KEY='sk-ant-...'")
			fmt.Println("")
			fmt.Println("Or add it to ~/.config/hypragent/config.toml:")
			fmt.Println("  [llm]")
			fmt.Println("  anthropic_api_key = \"sk-ant-...\"")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			os.Exit(1)
		}
		model := cfg.LLM.AnthropicModel
		if model == "" {
			model = os.Getenv("ANTHROPIC_MODEL")
		}
		llm = assistant.NewAnthropicProvider(apiKey, model)

	case "gemini":
		apiKey = cfg.LLM.GeminiKey
		if apiKey == "" {
			apiKey = os.Getenv("GEMINI_API_KEY")
		}
		if apiKey == "" {
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println("❌ Error: GEMINI_API_KEY not set")
			fmt.Println("")
			fmt.Println("Set it via environment variable:")
			fmt.Println("  export GEMINI_API_KEY='...'")
			fmt.Println("")
			fmt.Println("Or add it to ~/.config/hypragent/config.toml:")
			fmt.Println("  [llm]")
			fmt.Println("  gemini_api_key = \"...\"")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			os.Exit(1)
		}
		model := cfg.LLM.GeminiModel
		if model == "" {
			model = os.Getenv("GEMINI_MODEL")
		}
		llm, err = assistant.NewGeminiProvider(context.Background(), apiKey, model)
		if err != nil {
			fmt.Printf("Error initializing Gemini: %v\n", err)
			os.Exit(1)
		}

	case "ollama":
		host := cfg.LLM.OllamaHost
		if host == "" {
			host = os.Getenv("OLLAMA_HOST")
		}
		model := cfg.LLM.OllamaModel
		if model == "" {
			model = os.Getenv("OLLAMA_MODEL")
		}
		llm = assistant.NewOllamaProvider(host, model)

	case "openai":
		apiKey = cfg.LLM.OpenAIKey
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println("❌ Error: OPENAI_API_KEY not set")
			fmt.Println("")
			fmt.Println("Set it via environment variable:")
			fmt.Println("  export OPENAI_API_KEY='sk-...'")
			fmt.Println("")
			fmt.Println("Or add it to ~/.config/hypragent/config.toml:")
			fmt.Println("  [llm]")
			fmt.Println("  openai_api_key = \"sk-...\"")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			os.Exit(1)
		}
		model := cfg.LLM.OpenAIModel
		if model == "" {
			model = os.Getenv("OPENAI_MODEL")
		}
		llm = assistant.NewOpenAIProvider(apiKey, model)

	default:
		fmt.Printf("Error: Unknown LLM_PROVIDER '%s'. Supported: openai, anthropic, gemini, ollama\n", providerType)
		os.Exit(1)
	}

	// Initialize Safety Service
	snapshotService, err := safety.NewSnapshotService("")
	if err != nil {
		fmt.Printf("Warning: Failed to initialize snapshot service: %v\n", err)
	}

	// Initialize Backends
	nativeBackend := configuration.NewNativeBackend()
	hydeBackend := &configuration.HyDEBackend{}
	omarchyBackend := &configuration.OmarchyBackend{}

	backends := []configuration.ConfigBackend{hydeBackend, nativeBackend, omarchyBackend}

	// Detect active backend for system prompt
	var activeBackend configuration.ConfigBackend = nativeBackend // Default
	var detectedType configuration.ConfigSourceType = configuration.SourceNative
	for _, b := range backends {
		if found, _ := b.Detect(""); found {
			activeBackend = b
			detectedType = b.Type()
			break
		}
	}

	// Build system prompt with security context
	systemPrompt := buildSystemPrompt(cfg, detectedType)

	// Initialize Tools with config
	registry := assistant.NewToolRegistry()
	registry.Register(&assistant.DetectRootTool{Backends: backends})
	registry.Register(&assistant.ListDirTool{Config: cfg, Backend: activeBackend})
	registry.Register(&assistant.ReadFileTool{Config: cfg, Backend: activeBackend})
	registry.Register(&assistant.ParseConfigTool{Backend: activeBackend})
	registry.Register(&assistant.MakePatchTool{})
	registry.Register(&assistant.ApplyPatchTool{
		Backend:  activeBackend,
		Snapshot: snapshotService,
		Config:   cfg,
	})
	registry.Register(&assistant.RollbackTool{Snapshot: snapshotService})

	// Initialize Assistant with dynamic max turns
	agent := assistant.NewAgent(llm, registry, systemPrompt)

	// Initialize UI
	model := ui.NewModel(agent)

	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running HyprAgent: %v\n", err)
		os.Exit(1)
	}
}
