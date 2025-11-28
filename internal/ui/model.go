package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/reinhart/hyprAgent/internal/assistant"
)

// --- Mocha Palette & Styles ---

var (
	// Colors
	mochaBase    = lipgloss.Color("#1e1e2e") // Deep background
	mochaText    = lipgloss.Color("#cdd6f4") // Main text
	mochaSubtext = lipgloss.Color("#a6adc8") // Dimmed text

	colorCream  = lipgloss.Color("#f5e0dc")
	colorLatte  = lipgloss.Color("#ef9f76") // Orange-ish (User)
	colorMatcha = lipgloss.Color("#a6e3a1") // Green-ish (Agent)
	colorCoffee = lipgloss.Color("#fab387") // Peach/Brown
	colorMauve  = lipgloss.Color("#cba6f7") // Purple/Accent

	colorBorder = lipgloss.Color("#45475a") // Soft gray-blue border
	colorActive = lipgloss.Color("#f9e2af") // Yellow/Gold focus

	// Component Styles
	styleBase = lipgloss.NewStyle().Foreground(mochaText)

	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	styleFocusBorder = styleBorder.Copy().
				BorderForeground(colorActive)

	styleUserHeader = lipgloss.NewStyle().
			Foreground(colorLatte).
			Bold(true).
			MarginTop(1)

	styleAgentHeader = lipgloss.NewStyle().
				Foreground(colorMatcha).
				Bold(true).
				MarginTop(1)

	styleError = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f38ba8")). // Red
			Bold(true)

	styleStatus = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Italic(true)

	colorSubtext = lipgloss.Color("#9399b2")
)

type State int

const (
	StateReady State = iota
	StateThinking
)

type Model struct {
	agent         *assistant.Agent
	textarea      textarea.Model
	viewport      viewport.Model
	spinner       spinner.Model
	state         State
	statusHistory []string

	// Layout
	width  int
	height int
}

func NewModel(agent *assistant.Agent) Model {
	ta := textarea.New()
	ta.Placeholder = "Order a coffee or ask a question..."
	ta.Focus()
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.Prompt = ""     // Disable default prompt to avoid repetition on every line
	ta.CharLimit = 280 // Prevent massive inputs

	// Input Styles
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle() // No extra bg
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(colorSubtext)
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(colorCoffee)
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(colorCream)

	vp := viewport.New(80, 20)
	// Initial welcome message
	welcomeMsg := styleAgentHeader.Render("HyprAgent") + "\n" +
		styleBase.Render("Welcome! I'm ready to help you configure your system.")
	vp.SetContent(welcomeMsg)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorMauve)

	return Model{
		agent:         agent,
		textarea:      ta,
		viewport:      vp,
		spinner:       s,
		state:         StateReady,
		statusHistory: []string{},
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick)
}

type agentMsg struct {
	response string
	err      error
}

type statusMsg struct {
	msg  string
	diff string
}

func listenForUpdates(sub <-chan assistant.StatusUpdate) tea.Cmd {
	return func() tea.Msg {
		update, ok := <-sub
		if !ok {
			return nil
		}
		return statusMsg{msg: update.Message, diff: update.Diff}
	}
}

func (m Model) processInput(input string) tea.Cmd {
	return func() tea.Msg {
		// Create a context with timeout to prevent indefinite hangs
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second) // 3 minutes
		defer cancel()

		resp, err := m.agent.ProcessMessage(ctx, input)
		return agentMsg{response: resp, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Responsive Layout
		// Header (1) + Viewport (dynamic) + Status(1) + Input(5)
		// Input is usually height 3 + 2 border lines = 5 lines total

		verticalMargins := 7 // Borders + Status + Padding
		viewportHeight := msg.Height - verticalMargins
		if viewportHeight < 5 {
			viewportHeight = 5
		}

		m.viewport.Width = msg.Width - 4 // Minus borders/padding
		m.viewport.Height = viewportHeight

		m.textarea.SetWidth(msg.Width - 4)

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if !msg.Alt && m.state == StateReady {
				input := m.textarea.Value()
				if strings.TrimSpace(input) == "" {
					break
				}

				// Format User Message
				userHeader := styleUserHeader.Render("You")
				userBody := styleBase.Render(input)

				newContent := m.viewport.View() + "\n" + userHeader + "\n" + userBody + "\n"
				m.viewport.SetContent(newContent)
				m.viewport.GotoBottom()

				m.state = StateThinking
				m.statusHistory = []string{"Brewing response..."}

				// FORCE: Recreate the text area to nuke any internal state holding line position
				// This is a workaround for bubbletea/textarea sometimes retaining scroll
				newTa := textarea.New()
				newTa.Placeholder = m.textarea.Placeholder
				newTa.Focus()
				newTa.SetHeight(m.textarea.Height())
				newTa.ShowLineNumbers = false
				newTa.Prompt = ""
				newTa.CharLimit = m.textarea.CharLimit

				// Styles
				newTa.FocusedStyle.CursorLine = lipgloss.NewStyle()
				newTa.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(colorSubtext)
				newTa.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(colorCoffee)
				newTa.FocusedStyle.Text = lipgloss.NewStyle().Foreground(colorCream)

				// Set width
				newTa.SetWidth(m.width - 4)
				m.textarea = newTa

				cmds = append(cmds, listenForUpdates(m.agent.Updates()))
				cmds = append(cmds, m.processInput(input))

				// Don't update textarea with this Enter key event since we just replaced it
				return m, tea.Batch(cmds...)
			}
		}

	case statusMsg:
		m.statusHistory = append(m.statusHistory, msg.msg)
		if len(m.statusHistory) > 3 {
			m.statusHistory = m.statusHistory[len(m.statusHistory)-3:]
		}

		// If there's a diff, render it immediately to the viewport
		if msg.diff != "" {
			diffHeader := styleAgentHeader.Render(" Proposed Changes:")
			// Use a simple style for diff content, maybe syntax highlight later
			diffBody := lipgloss.NewStyle().Foreground(colorSubtext).Render(msg.diff)
			// Wrap in code block style or similar
			diffBlock := fmt.Sprintf("\n%s\n```diff\n%s\n```\n", diffHeader, diffBody)

			newContent := m.viewport.View() + diffBlock
			m.viewport.SetContent(newContent)
			m.viewport.GotoBottom()
		}

		if m.state == StateThinking {
			cmds = append(cmds, listenForUpdates(m.agent.Updates()))
		}

	case agentMsg:
		m.state = StateReady
		var output string
		agentHeader := styleAgentHeader.Render("HyprAgent")

		if msg.err != nil {
			output = agentHeader + "\n" + styleError.Render(fmt.Sprintf("Error: %v", msg.err))
		} else {
			output = agentHeader + "\n" + styleBase.Render(msg.response)
		}

		// Append Assistant Response
		// Add a subtle separator
		separator := lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", m.width/2))

		newContent := m.viewport.View() + output + "\n\n" + separator + "\n"
		m.viewport.SetContent(newContent)

		// Force scroll to bottom AFTER setting content
		m.viewport.GotoBottom()

		// Ensure viewport processes the scroll by updating it immediately
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)

		m.textarea.Focus()

		// Return early to skip the normal update flow which might interfere with scroll
		return m, tea.Batch(cmds...)

	case spinner.TickMsg:
		if m.state == StateThinking {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Always update components (unless we already returned early)
	// Update viewport first to process scroll commands
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	// Update textarea (but skip if we're in Thinking state to avoid processing stale events)
	if m.state == StateReady {
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	// 1. Header / Chat Viewport
	chatView := styleBorder.Width(m.width - 2).Height(m.viewport.Height + 2).Render(m.viewport.View())

	// 2. Status Area
	var statusStr string
	if m.state == StateThinking {
		// Show last 3 statuses joined
		fullStatus := strings.Join(m.statusHistory, "  ➜  ")
		statusStr = fmt.Sprintf(" %s %s", m.spinner.View(), styleStatus.Render(fullStatus))
	} else {
		statusStr = styleStatus.Render(" Ready to serve.")
	}
	// Pad status to width
	statusView := lipgloss.NewStyle().Width(m.width).PaddingLeft(1).Render(statusStr)

	// 3. Input Area
	// We add the prompt manually outside the textarea to ensure it only appears once
	// and doesn't clutter the multi-line input
	prompt := lipgloss.NewStyle().Foreground(colorCoffee).Render("☕ ")
	inputContent := lipgloss.JoinHorizontal(lipgloss.Top, prompt, m.textarea.View())

	inputView := styleFocusBorder.Width(m.width - 2).Render(inputContent)

	// Layout Composition
	return lipgloss.JoinVertical(lipgloss.Left,
		chatView,
		statusView,
		inputView,
	)
}
