# HyprAgent ☕

> **Your Intelligent Hyprland Configuration Assistant**

HyprAgent is a terminal-based AI assistant designed to help you configure, manage, and troubleshoot your [Hyprland](https://hyprland.org/) environment. It combines the power of LLMs (OpenAI, Anthropic, Gemini, Ollama) with deep knowledge of Hyprland's configuration syntax.

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.23%2B-cyan)

## ✨ Features

- **Natural Language Configuration**: Ask "Change my border color to peach" or "Set up a keybind for Firefox".
- **Cafe Mocha UI**: A beautiful, cozy terminal interface built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).
- **Multi-Provider Support**: Use your preferred LLM:
  - OpenAI (GPT-4o)
  - Anthropic (Claude 3.5 Sonnet)
  - Google Gemini (Pro 1.5)
  - Ollama (Local models)
- **Safe Configuration**: HyprAgent validates changes and can backup your config before applying them.
- **Context Aware**: It understands your current file structure and existing configuration.

## 🚀 Getting Started

### Prerequisites

- Go 1.23 or higher
- A working Hyprland installation
- An API Key for your chosen LLM provider

### Installation

#### From Source
```bash
git clone https://github.com/blessonmathewsam/hyprAgent.git
cd hyprAgent
go mod download
go build -o hypragent cmd/hyprAgent/main.go
sudo install -Dm755 hypragent /usr/bin/hypragent
sudo install -Dm644 config.example.toml /etc/hypragent/config.toml.example
```

### Configuration

HyprAgent uses a TOML configuration file. The config is loaded from (in order):
1. `./config.toml` (current directory - for development)
2. `~/.config/hypragent/config.toml` (user config - **recommended**)
3. `/etc/hypragent/config.toml` (system-wide config)

**Setup Configuration:**
```bash
mkdir -p ~/.config/hypragent
cp config.example.toml ~/.config/hypragent/config.toml
nano ~/.config/hypragent/config.toml
```

Edit the config file to add your API keys:
```toml
[llm]
provider = "openai"
openai_api_key = "sk-..."
```

**Alternatively**, use environment variables (these override config file values):

```bash
# For OpenAI
export OPENAI_API_KEY="your-key-here"

# For Anthropic
export ANTHROPIC_API_KEY="your-key-here"

# For Gemini
export GEMINI_API_KEY="your-key-here"
```

### Security

HyprAgent implements a **whitelist-based security model**:
- File operations are restricted to specific Hyprland directories
- Different whitelists for Native, HyDE, and Omarchy installations
- Paths are validated before any read/write operation
- The agent is informed of allowed paths in its system prompt

You can customize the whitelist in `config.toml`:
```toml
[security.hyde]
allowed_dirs = [".", "./Configs", "./scripts"]
allowed_files = ["hyprland.conf", "keybindings.conf"]
```

### Usage

Run the agent:

```bash
./hypragent
```

## ☕ UI Navigation

- **Type** your request in the input box at the bottom.
- **Enter** to send your message.
- **Ctrl+C** or **Esc** to quit.

## 🛠️ Architecture

HyprAgent is built with a modular architecture:

- **`internal/assistant`**: Core logic for handling LLM interactions and tool execution.
- **`internal/ui`**: The TUI (Terminal User Interface) implementation.
- **`internal/configuration`**: Parsers and handlers for Hyprland config files.
- **`internal/safety`**: Backup and snapshot mechanisms.

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 📄 License

This project is licensed under the MIT License.

