// internal/ui/ui.go
package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport" // For scrollable content
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jmfirth/hf-lms-sync/internal/fsutils"
)

// model defines the Bubble Tea model for our UI.
type model struct {
	models        []fsutils.ModelInfo
	stale         []fsutils.ModelInfo
	combined      []fsutils.ModelInfo
	selectedIndex int
	status        string
	targetDir     string

	viewport viewport.Model // viewport for the scrolling list
	width    int
	height   int

	quitting bool
}

// New creates and returns a new UI model.
func New(targetDir string) tea.Model {
	models, _ := fsutils.LoadModels(targetDir)
	stale, _ := fsutils.FindStaleLinks(targetDir)
	combined := append(models, stale...)
	sort.Slice(combined, func(i, j int) bool {
		return combined[i].CacheDirName < combined[j].CacheDirName
	})
	return model{
		models:        models,
		stale:         stale,
		combined:      combined,
		selectedIndex: 0,
		status:        fmt.Sprintf("Found %d model(s) and %d stale reference(s).", len(models), len(stale)),
		targetDir:     targetDir,
	}
}

// Init sets up the initial UI state.
func (m model) Init() tea.Cmd {
	// Request the current terminal dimensions.
	return tea.EnterAltScreen
}

// opResultMsg is used to update the UI state with fresh model data.
type opResultMsg struct {
	status string
	models []fsutils.ModelInfo
	stale  []fsutils.ModelInfo
}

// errorMsg is used to pass error information to the UI.
type errorMsg string

func updateState(targetDir, status string) tea.Msg {
	models, _ := fsutils.LoadModels(targetDir)
	stale, _ := fsutils.FindStaleLinks(targetDir)
	return opResultMsg{
		status: status,
		models: models,
		stale:  stale,
	}
}

// linkModelCmd creates a command to link a model.
func linkModelCmd(m fsutils.ModelInfo, targetDir string) tea.Cmd {
	return func() tea.Msg {
		if err := fsutils.LinkModel(m); err != nil {
			return errorMsg(fmt.Sprintf("Error linking model %s: %v", m.ModelName, err))
		}
		return updateState(targetDir, fmt.Sprintf("Linked model: %s", m.ModelName))
	}
}

// unlinkModelCmd creates a command to unlink a model.
func unlinkModelCmd(m fsutils.ModelInfo, targetDir string) tea.Cmd {
	return func() tea.Msg {
		if err := fsutils.UnlinkModel(m); err != nil {
			return errorMsg(fmt.Sprintf("Error unlinking model %s: %v", m.ModelName, err))
		}
		return updateState(targetDir, fmt.Sprintf("Unlinked model: %s", m.ModelName))
	}
}

// purgeModelCmd creates a command to purge a stale model.
func purgeModelCmd(m fsutils.ModelInfo, targetDir string) tea.Cmd {
	return func() tea.Msg {
		if err := fsutils.UnlinkModel(m); err != nil {
			return errorMsg(fmt.Sprintf("Error purging model %s: %v", m.ModelName, err))
		}
		return updateState(targetDir, fmt.Sprintf("Purged stale model: %s", m.ModelName))
	}
}

// linkAllCmd creates a command to link all unlinked models.
func linkAllCmd(models []fsutils.ModelInfo, targetDir string) tea.Cmd {
	return func() tea.Msg {
		for _, m := range models {
			if !m.IsLinked {
				_ = fsutils.LinkModel(m)
			}
		}
		return updateState(targetDir, "All models linked successfully")
	}
}

// unlinkAllCmd creates a command to unlink all linked models.
func unlinkAllCmd(models []fsutils.ModelInfo, targetDir string) tea.Cmd {
	return func() tea.Msg {
		for _, m := range models {
			if m.IsLinked {
				_ = fsutils.UnlinkModel(m)
			}
		}
		return updateState(targetDir, "All models unlinked successfully")
	}
}

// purgeAllCmd creates a command to purge all stale links.
func purgeAllCmd(stale []fsutils.ModelInfo, targetDir string) tea.Cmd {
	return func() tea.Msg {
		for _, m := range stale {
			_ = fsutils.UnlinkModel(m)
		}
		return updateState(targetDir, fmt.Sprintf("Purged %d stale links", len(stale)))
	}
}

// Update processes incoming messages.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Reserve 2 lines for status and command bar; title is rendered separately.
		viewportHeight := m.height - 3
		m.viewport = viewport.New(m.width, viewportHeight)
		m.viewport.SetContent(m.renderList())
		return m, nil

	case tea.KeyMsg:
		var cmds []tea.Cmd

		// Process custom commands first.
		switch msg.String() {
		case "ctrl+c", "q", "Q":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			} else if len(m.combined) > 0 {
				m.selectedIndex = len(m.combined) - 1
			}
		case "down", "j":
			if len(m.combined) > 0 {
				m.selectedIndex = (m.selectedIndex + 1) % len(m.combined)
			}
		case "l":
			if len(m.combined) > 0 {
				selected := m.combined[m.selectedIndex]
				if !selected.IsStale && !selected.IsLinked {
					m.status = "Linking model: " + selected.ModelName
					cmds = append(cmds, linkModelCmd(selected, m.targetDir))
				}
			}
		case "u":
			if len(m.combined) > 0 {
				selected := m.combined[m.selectedIndex]
				if !selected.IsStale && selected.IsLinked {
					m.status = "Unlinking model: " + selected.ModelName
					cmds = append(cmds, unlinkModelCmd(selected, m.targetDir))
				}
			}
		case "c":
			if len(m.combined) > 0 {
				selected := m.combined[m.selectedIndex]
				if selected.IsStale {
					m.status = "Purging stale model: " + selected.ModelName
					cmds = append(cmds, purgeModelCmd(selected, m.targetDir))
				}
			}
		case "L":
			m.status = "Linking all models..."
			cmds = append(cmds, linkAllCmd(m.models, m.targetDir))
		case "U":
			m.status = "Unlinking all models..."
			cmds = append(cmds, unlinkAllCmd(m.models, m.targetDir))
		case "C":
			m.status = "Purging all stale links..."
			cmds = append(cmds, purgeAllCmd(m.stale, m.targetDir))
		}

		// Now allow the viewport to process the key.
		var vpCmd tea.Cmd
		m.viewport, vpCmd = m.viewport.Update(msg)
		cmds = append(cmds, vpCmd)

		// Update the viewport content.
		m.viewport.SetContent(m.renderList())
		return m, tea.Batch(cmds...)

	// Process other messages (like opResultMsg, errorMsg, etc.)
	case opResultMsg:
		m.status = msg.status
		m.models = msg.models
		m.stale = msg.stale
		m.combined = append(msg.models, msg.stale...)
		sort.Slice(m.combined, func(i, j int) bool {
			return m.combined[i].CacheDirName < m.combined[j].CacheDirName
		})
		if m.selectedIndex >= len(m.combined) {
			m.selectedIndex = 0
		}
		// Update viewport content so UI re-renders with the new state.
		m.viewport.SetContent(m.renderList())
		return m, nil

	case errorMsg:
		m.status = string(msg)
		return m, nil
	}

	return m, nil
}

// renderList builds the scrollable list content (without the title).
func (m model) renderList() string {
	var b strings.Builder
	// Show the target directory info, but not the main title.
	b.WriteString(fmt.Sprintf("LM Studio Models: %s\n\n", m.targetDir))
	for i, item := range m.combined {
		pointer := " "
		if i == m.selectedIndex {
			pointer = "‣"
		}
		var statusIcon string
		if item.IsStale {
			statusIcon = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("⦿")
		} else if item.IsLinked {
			statusIcon = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("⦿")
		} else {
			statusIcon = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("⦿")
		}

		var statusText string
		if item.IsStale {
			statusText = fmt.Sprintf(" Stale Link (%s)", item.StaleReason)
		} else if item.IsLinked {
			statusText = " Linked"
		} else {
			statusText = " Not Linked"
		}

		line := fmt.Sprintf("%s %s %s - %s%s", pointer, statusIcon, item.ModelName, item.OrganizationName, statusText)
		b.WriteString(line + "\n")
	}
	return b.String()
}

// View renders the full UI.
func (m model) View() string {
	if m.quitting {
		return ""
	}

	// Create a static title bar.
	titleBar := lipgloss.NewStyle().
		Bold(true).
		Align(lipgloss.Center).
		Width(m.width).
		Render("Hugging Face to LM Studio Sync")

	// Render the status and command bars as before.
	statusBar := fmt.Sprintf("Status: %s", m.status)
	commandBarStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#333")).
		Foreground(lipgloss.Color("#FFF")).
		Width(m.width)
	commandBar := commandBarStyle.Render("↑/k: Up | ↓/j: Down | l: Link | u: Unlink | c: Purge | L: Link All | U: Unlink All | C: Purge All | q: Quit")

	// Combine the title, viewport, status, and command bar.
	return titleBar + "\n" + m.viewport.View() + "\n" + commandBar + "\n" + statusBar
}
