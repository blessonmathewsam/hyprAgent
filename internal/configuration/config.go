package configuration

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config represents the application configuration
type Config struct {
	LLM      LLMConfig      `toml:"llm"`
	Agent    AgentConfig    `toml:"agent"`
	Security SecurityConfig `toml:"security"`
}

type LLMConfig struct {
	Provider       string `toml:"provider"`
	OpenAIKey      string `toml:"openai_api_key"`
	AnthropicKey   string `toml:"anthropic_api_key"`
	GeminiKey      string `toml:"gemini_api_key"`
	OpenAIModel    string `toml:"openai_model"`
	AnthropicModel string `toml:"anthropic_model"`
	GeminiModel    string `toml:"gemini_model"`
	OllamaHost     string `toml:"ollama_host"`
	OllamaModel    string `toml:"ollama_model"`
}

type AgentConfig struct {
	MaxTurns int  `toml:"max_turns"`
	Debug    bool `toml:"debug"`
}

type SecurityConfig struct {
	Native  BackendSecurity `toml:"native"`
	Hyde    BackendSecurity `toml:"hyde"`
	Omarchy BackendSecurity `toml:"omarchy"`
}

type BackendSecurity struct {
	AllowedDirs  []string `toml:"allowed_dirs"`
	AllowedFiles []string `toml:"allowed_files"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Provider: "openai",
		},
		Agent: AgentConfig{
			MaxTurns: 25,
			Debug:    false,
		},
		Security: SecurityConfig{
			Native: BackendSecurity{
				AllowedDirs: []string{".", "./scripts", "./themes"},
				AllowedFiles: []string{
					"hyprland.conf", "hyprpaper.conf", "hypridle.conf", "hyprlock.conf",
					"keybindings.conf", "windowrules.conf", "monitors.conf",
					"workspaces.conf", "animations.conf", "userprefs.conf",
				},
			},
			Hyde: BackendSecurity{
				AllowedDirs: []string{
					".", "./Configs", "./scripts", "./themes", "./animations",
					"./shaders", "./hyprlock", "./workflows",
				},
				AllowedFiles: []string{
					"hyprland.conf", "hyde.conf", "hypridle.conf", "hyprlock.conf",
					"keybindings.conf", "windowrules.conf", "monitors.conf",
					"workspaces.conf", "workflows.conf", "animations.conf",
					"shaders.conf", "userprefs.conf", "pyprland.toml",
				},
			},
			Omarchy: BackendSecurity{
				AllowedDirs: []string{".", "./omarchy", "./scripts", "./themes"},
				AllowedFiles: []string{
					"hyprland.conf", "keybindings.conf", "windowrules.conf",
					"monitors.conf", "workspaces.conf",
				},
			},
		},
	}
}

// LoadConfig loads configuration from file with fallback to defaults
func LoadConfig() (*Config, error) {
	config := DefaultConfig()

	// Try multiple config locations in order (following XDG and Arch conventions)
	configPaths := []string{
		"./config.toml", // Current directory (for development)
		filepath.Join(os.Getenv("HOME"), ".config", "hypragent", "config.toml"), // User config (XDG)
		"/etc/hypragent/config.toml", // System-wide config (Arch standard)
	}

	var loaded bool
	var loadedPath string
	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			if _, err := toml.DecodeFile(path, config); err != nil {
				return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
			}
			loaded = true
			loadedPath = path
			break
		}
	}

	// Override with environment variables if set
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		config.LLM.OpenAIKey = key
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		config.LLM.AnthropicKey = key
	}
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		config.LLM.GeminiKey = key
	}
	if provider := os.Getenv("LLM_PROVIDER"); provider != "" {
		config.LLM.Provider = provider
	}
	if debug := os.Getenv("DEBUG"); debug == "true" {
		config.Agent.Debug = true
	}

	if !loaded {
		// No config file found, using defaults
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("⚠️  No config file found. Using default settings.")
		fmt.Println("")
		fmt.Println("To configure HyprAgent:")
		fmt.Println("  1. Copy the example config:")
		fmt.Println("     mkdir -p ~/.config/hypragent")
		fmt.Println("     cp /etc/hypragent/config.toml.example ~/.config/hypragent/config.toml")
		fmt.Println("")
		fmt.Println("  2. Edit the config to add your API key:")
		fmt.Println("     nano ~/.config/hypragent/config.toml")
		fmt.Println("")
		fmt.Println("  OR use environment variables (temporary):")
		fmt.Println("     export OPENAI_API_KEY='sk-...'")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("")
	} else {
		fmt.Printf("✓ Loaded config from: %s\n", loadedPath)
	}

	return config, nil
}

// IsPathAllowed checks if a path is within the allowed directories/files for a backend
func (c *Config) IsPathAllowed(backendType ConfigSourceType, targetPath string) (bool, error) {
	// Get the appropriate security config
	var sec BackendSecurity
	switch backendType {
	case SourceNative:
		sec = c.Security.Native
	case SourceHyDE:
		sec = c.Security.Hyde
	case SourceOmarchy:
		sec = c.Security.Omarchy
	default:
		return false, fmt.Errorf("unknown backend type: %s", backendType)
	}

	// Get Hyprland config root
	home, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}
	configRoot := filepath.Join(home, ".config", "hypr")

	// Resolve target path to absolute
	var absTarget string
	if filepath.IsAbs(targetPath) {
		absTarget = filepath.Clean(targetPath)
	} else {
		absTarget = filepath.Join(configRoot, targetPath)
		absTarget = filepath.Clean(absTarget)
	}

	// Check if target is within config root
	if !filepath.HasPrefix(absTarget, configRoot) {
		return false, fmt.Errorf("path %s is outside Hyprland config directory", targetPath)
	}

	// Get relative path from config root
	relPath, err := filepath.Rel(configRoot, absTarget)
	if err != nil {
		return false, err
	}

	// Check if it's an allowed file directly
	for _, allowedFile := range sec.AllowedFiles {
		if relPath == allowedFile || filepath.Base(absTarget) == allowedFile {
			return true, nil
		}
	}

	// Check if it's within an allowed directory
	for _, allowedDir := range sec.AllowedDirs {
		allowedDirAbs := filepath.Join(configRoot, allowedDir)
		allowedDirAbs = filepath.Clean(allowedDirAbs)

		if absTarget == allowedDirAbs || filepath.HasPrefix(absTarget, allowedDirAbs+string(filepath.Separator)) {
			return true, nil
		}
	}

	return false, fmt.Errorf("path %s is not in the allowed list for %s backend", relPath, backendType)
}
