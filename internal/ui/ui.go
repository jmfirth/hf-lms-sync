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
	"github.com/jmfirth/hf-lms-sync/internal/logger"
)

// model defines the Bubble Tea model for our UI.
type model struct {
	models        []fsutils.ModelInfo
	stale         []fsutils.ModelInfo
	combined      []fsutils.ModelInfo
	filtered      []fsutils.ModelInfo // Filtered list based on search
	selectedIndex int
	status        string
	targetDir     string

	viewport viewport.Model // viewport for the scrolling list
	width    int
	height   int

	searching     bool   // Whether we're in search mode
	searchQuery   string // Current search query
	searchMatches int    // Number of matches found

	quitting bool
	logger   *logger.Logger
}

// New creates and returns a new UI model.
func New(targetDir string, appLogger *logger.Logger) tea.Model {
	models, _ := fsutils.LoadModels(targetDir)
	stale, _ := fsutils.FindStaleLinks(targetDir)
	combined := append(models, stale...)
	sort.Slice(combined, func(i, j int) bool {
		return combined[i].CacheDirName < combined[j].CacheDirName
	})
	if appLogger != nil && appLogger.Verbose {
		appLogger.Info("UI", "Initializing UI with %d models and %d stale references", len(models), len(stale))
	}
	
	return model{
		models:        models,
		stale:         stale,
		combined:      combined,
		filtered:      combined, // Initially show all models
		selectedIndex: 0,
		status:        fmt.Sprintf("Found %d model(s) and %d stale reference(s).", len(models), len(stale)),
		targetDir:     targetDir,
		searching:     false,
		searchQuery:   "",
		searchMatches: len(combined),
		logger:        appLogger,
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
func linkModelCmd(m fsutils.ModelInfo, targetDir string, logger *logger.Logger) tea.Cmd {
	return func() tea.Msg {
		if logger != nil && logger.Verbose {
			logger.Info("UI", "Linking model: %s/%s", m.OrganizationName, m.ModelName)
		}
		if err := fsutils.LinkModel(m); err != nil {
			if logger != nil && logger.Verbose {
				logger.Error("UI", "Error linking model %s/%s: %v", m.OrganizationName, m.ModelName, err)
			}
			return errorMsg(fmt.Sprintf("Error linking model %s: %v", m.ModelName, err))
		}
		if logger != nil && logger.Verbose {
			logger.Info("UI", "Successfully linked model: %s/%s", m.OrganizationName, m.ModelName)
		}
		return updateState(targetDir, fmt.Sprintf("Linked model: %s", m.ModelName))
	}
}

// unlinkModelCmd creates a command to unlink a model.
func unlinkModelCmd(m fsutils.ModelInfo, targetDir string, logger *logger.Logger) tea.Cmd {
	return func() tea.Msg {
		if logger != nil && logger.Verbose {
			logger.Info("UI", "Unlinking model: %s/%s", m.OrganizationName, m.ModelName)
		}
		if err := fsutils.UnlinkModel(m); err != nil {
			if logger != nil && logger.Verbose {
				logger.Error("UI", "Error unlinking model %s/%s: %v", m.OrganizationName, m.ModelName, err)
			}
			return errorMsg(fmt.Sprintf("Error unlinking model %s: %v", m.ModelName, err))
		}
		if logger != nil && logger.Verbose {
			logger.Info("UI", "Successfully unlinked model: %s/%s", m.OrganizationName, m.ModelName)
		}
		return updateState(targetDir, fmt.Sprintf("Unlinked model: %s", m.ModelName))
	}
}

// purgeModelCmd creates a command to purge a stale model.
func purgeModelCmd(m fsutils.ModelInfo, targetDir string, logger *logger.Logger) tea.Cmd {
	return func() tea.Msg {
		if logger != nil && logger.Verbose {
			logger.Info("UI", "Purging stale model: %s/%s (Reason: %s)", m.OrganizationName, m.ModelName, m.StaleReason)
		}
		if err := fsutils.UnlinkModel(m); err != nil {
			if logger != nil && logger.Verbose {
				logger.Error("UI", "Error purging stale model %s/%s: %v", m.OrganizationName, m.ModelName, err)
			}
			return errorMsg(fmt.Sprintf("Error purging model %s: %v", m.ModelName, err))
		}
		if logger != nil && logger.Verbose {
			logger.Info("UI", "Successfully purged stale model: %s/%s", m.OrganizationName, m.ModelName)
		}
		return updateState(targetDir, fmt.Sprintf("Purged stale model: %s", m.ModelName))
	}
}

// linkAllCmd creates a command to link all unlinked models.
func linkAllCmd(models []fsutils.ModelInfo, targetDir string, logger *logger.Logger) tea.Cmd {
	return func() tea.Msg {
		if logger != nil && logger.Verbose {
			logger.Info("UI", "Linking all unlinked models (%d total)", len(models))
		}
		linkedCount := 0
		for _, m := range models {
			if !m.IsLinked {
				if err := fsutils.LinkModel(m); err != nil {
					if logger != nil && logger.Verbose {
						logger.Error("UI", "Error linking model %s/%s: %v", m.OrganizationName, m.ModelName, err)
					}
				} else {
					linkedCount++
					if logger != nil && logger.Verbose {
						logger.Debug("UI", "Linked model: %s/%s", m.OrganizationName, m.ModelName)
					}
				}
			}
		}
		if logger != nil && logger.Verbose {
			logger.Info("UI", "Successfully linked %d models", linkedCount)
		}
		return updateState(targetDir, "All models linked successfully")
	}
}

// unlinkAllCmd creates a command to unlink all linked models.
func unlinkAllCmd(models []fsutils.ModelInfo, targetDir string, logger *logger.Logger) tea.Cmd {
	return func() tea.Msg {
		if logger != nil && logger.Verbose {
			logger.Info("UI", "Unlinking all linked models")
		}
		unlinkedCount := 0
		for _, m := range models {
			if m.IsLinked {
				if err := fsutils.UnlinkModel(m); err != nil {
					if logger != nil && logger.Verbose {
						logger.Error("UI", "Error unlinking model %s/%s: %v", m.OrganizationName, m.ModelName, err)
					}
				} else {
					unlinkedCount++
					if logger != nil && logger.Verbose {
						logger.Debug("UI", "Unlinked model: %s/%s", m.OrganizationName, m.ModelName)
					}
				}
			}
		}
		if logger != nil && logger.Verbose {
			logger.Info("UI", "Successfully unlinked %d models", unlinkedCount)
		}
		return updateState(targetDir, "All models unlinked successfully")
	}
}

// purgeAllCmd creates a command to purge all stale links.
func purgeAllCmd(stale []fsutils.ModelInfo, targetDir string, logger *logger.Logger) tea.Cmd {
	return func() tea.Msg {
		if logger != nil && logger.Verbose {
			logger.Info("UI", "Purging all stale links (%d total)", len(stale))
		}
		purgedCount := 0
		for _, m := range stale {
			if err := fsutils.UnlinkModel(m); err != nil {
				if logger != nil && logger.Verbose {
					logger.Error("UI", "Error purging stale model %s/%s: %v", m.OrganizationName, m.ModelName, err)
				}
			} else {
				purgedCount++
				if logger != nil && logger.Verbose {
					logger.Debug("UI", "Purged stale model: %s/%s", m.OrganizationName, m.ModelName)
				}
			}
		}
		if logger != nil && logger.Verbose {
			logger.Info("UI", "Successfully purged %d stale links", purgedCount)
		}
		return updateState(targetDir, fmt.Sprintf("Purged %d stale links", len(stale)))
	}
}

// Update processes incoming messages.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Reserve space for title (1), command bar (1), and status bar (1)
		viewportHeight := m.height - 3
		m.viewport = viewport.New(m.width, viewportHeight)
		m.viewport.YPosition = 1 // Position viewport below title
		m.viewport.SetContent(m.renderList())
		return m, nil

	case tea.KeyMsg:
		var cmds []tea.Cmd

		// Handle search mode separately
		if m.searching {
			switch msg.String() {
			case "ctrl+c", "esc":
				m.searching = false
				m.searchQuery = ""
				m.filtered = m.combined
				m.searchMatches = len(m.combined)
				m.status = fmt.Sprintf("Search cancelled. Found %d model(s) and %d stale reference(s).", len(m.models), len(m.stale))
				m.viewport.SetContent(m.renderList())
				return m, nil
			case "ctrl+u":
				m.searchQuery = ""
				m.updateSearch()
				return m, nil
			case "enter":
				m.searching = false
				m.status = fmt.Sprintf("Found %d matches", m.searchMatches)
				m.viewport.SetContent(m.renderList())
				return m, nil
			case "backspace":
				if len(m.searchQuery) > 0 {
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
					m.updateSearch()
				}
				return m, nil
			default:
				// In search mode, treat all other keys as potential search input
				if len(msg.String()) == 1 {
					m.searchQuery += msg.String()
					m.updateSearch()
				}
				return m, nil
			}
		}

		// Handle normal mode commands
		switch msg.String() {
		case "ctrl+c", "q", "Q":
			m.quitting = true
			return m, tea.Quit
		case "/":
			m.searching = true
			m.status = fmt.Sprintf("Search: %s", m.searchQuery)
			m.viewport.SetContent(m.renderList())
			return m, nil
		case "up", "k":
			if m.selectedIndex > 0 {
				m.selectedIndex--
				// Ensure selected item is visible
				if m.selectedIndex < m.viewport.YOffset {
					m.viewport.YOffset = m.selectedIndex
				}
			}
		case "down", "j":
			if m.selectedIndex < len(m.filtered)-1 {
				m.selectedIndex++
				// Ensure selected item is visible
				if m.selectedIndex >= m.viewport.YOffset+m.viewport.Height {
					m.viewport.YOffset = m.selectedIndex - m.viewport.Height + 1
				}
			}
		case "l":
			if len(m.filtered) > 0 {
				selected := m.filtered[m.selectedIndex]
				if !selected.IsStale && !selected.IsLinked {
					m.status = "Linking model: " + selected.ModelName
					cmds = append(cmds, linkModelCmd(selected, m.targetDir, m.logger))
				}
			}
		case "u":
			if len(m.filtered) > 0 {
				selected := m.filtered[m.selectedIndex]
				if !selected.IsStale && selected.IsLinked {
					m.status = "Unlinking model: " + selected.ModelName
					cmds = append(cmds, unlinkModelCmd(selected, m.targetDir, m.logger))
				}
			}
		case "c":
			if len(m.filtered) > 0 {
				selected := m.filtered[m.selectedIndex]
				if selected.IsStale {
					m.status = "Purging stale model: " + selected.ModelName
					cmds = append(cmds, purgeModelCmd(selected, m.targetDir, m.logger))
				}
			}
		case "L":
			m.status = "Linking all models..."
			cmds = append(cmds, linkAllCmd(m.models, m.targetDir, m.logger))
		case "U":
			m.status = "Unlinking all models..."
			cmds = append(cmds, unlinkAllCmd(m.models, m.targetDir, m.logger))
		case "C":
			m.status = "Purging all stale links..."
			cmds = append(cmds, purgeAllCmd(m.stale, m.targetDir, m.logger))
		}

		// Now allow the viewport to process the key.
		var vpCmd tea.Cmd
		m.viewport, vpCmd = m.viewport.Update(msg)
		cmds = append(cmds, vpCmd)

		// Update the viewport content.
		if len(cmds) > 0 {
			m.viewport.SetContent(m.renderList())
			return m, tea.Batch(cmds...)
		}
		return m, nil

	// Process other messages (like opResultMsg, errorMsg, etc.)
	case opResultMsg:
		m.status = msg.status
		m.models = msg.models
		m.stale = msg.stale
		m.combined = append(msg.models, msg.stale...)
		sort.Slice(m.combined, func(i, j int) bool {
			return m.combined[i].CacheDirName < m.combined[j].CacheDirName
		})
		// Update filtered list and maintain search if active
		if m.searching {
			m.updateSearch()
		} else {
			m.filtered = m.combined
		}
		if m.selectedIndex >= len(m.filtered) {
			m.selectedIndex = 0
		}
		// Update viewport content and ensure selected item remains visible
		m.viewport.SetContent(m.renderList())
		if m.selectedIndex < m.viewport.YOffset {
			m.viewport.YOffset = m.selectedIndex
		} else if m.selectedIndex >= m.viewport.YOffset+m.viewport.Height {
			m.viewport.YOffset = m.selectedIndex - m.viewport.Height + 1
		}
		return m, nil

	case errorMsg:
		m.status = string(msg)
		return m, nil
	}

	return m, nil
}

// updateSearch filters the combined list based on the current search query
func (m *model) updateSearch() {
	if m.searchQuery == "" {
		m.filtered = m.combined
		m.searchMatches = len(m.combined)
		m.status = fmt.Sprintf("Search: %s", m.searchQuery)
		m.viewport.SetContent(m.renderList())
		return
	}

	query := strings.ToLower(m.searchQuery)
	var filtered []fsutils.ModelInfo
	for _, item := range m.combined {
		if strings.Contains(strings.ToLower(item.ModelName), query) ||
			strings.Contains(strings.ToLower(item.OrganizationName), query) {
			filtered = append(filtered, item)
		}
	}
	m.filtered = filtered
	m.searchMatches = len(filtered)
	m.status = fmt.Sprintf("Search: %s (%d matches)", m.searchQuery, m.searchMatches)
	m.viewport.SetContent(m.renderList())
}

// highlightMatch adds highlighting to matching text
func (m model) highlightMatch(text string) string {
	if !m.searching || m.searchQuery == "" {
		return text
	}
	
	query := strings.ToLower(m.searchQuery)
	textLower := strings.ToLower(text)
	if !strings.Contains(textLower, query) {
		return text
	}

	highlighted := lipgloss.NewStyle().
		Background(lipgloss.Color("4")).
		Foreground(lipgloss.Color("15")).
		Render(text)
	return highlighted
}

// renderList builds the scrollable list content (without the title).
func (m model) renderList() string {
	var b strings.Builder
	// Show the target directory info, but not the main title.
	hfCache, _ := fsutils.GetHfCacheDir()
	hfCacheLine := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(m.width).
		Render(fmt.Sprintf("Hugging Face Cache: %s", hfCache))
	b.WriteString(hfCacheLine)
	b.WriteString("\n")
	lmsModelsCacheLine := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(m.width).
		Render(fmt.Sprintf("LM Studio Models: %s", m.targetDir))
	b.WriteString(lmsModelsCacheLine)
	b.WriteString("\n\n")
	for i, item := range m.filtered {
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

		modelName := m.highlightMatch(item.ModelName)
		orgName := m.highlightMatch(item.OrganizationName)

		line := fmt.Sprintf("%s %s %s/%s", pointer, statusIcon, orgName, modelName)
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
		Background(lipgloss.Color("#333")).
		Foreground(lipgloss.Color("#FFF")).
		Align(lipgloss.Center).
		Width(m.width).
		Render("Hugging Face to LM Studio Sync")

	// Render the status and command bars
	var statusBar string
	if m.searching {
		statusBar = m.status // During search, show raw status without prefix
	} else {
		statusBar = fmt.Sprintf("Status: %s", m.status)
	}
	commandBarStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#333")).
		Foreground(lipgloss.Color("#FFF")).
		Width(m.width)
	var commandBar string
	if m.searching {
		commandBar = commandBarStyle.Render("Type to search | Enter: Accept | Esc/Ctrl+C: Cancel | Ctrl+U: Clear")
	} else {
		commandBar = commandBarStyle.Render("↑/k: Up | ↓/j: Down | /: Search | l: Link | u: Unlink | c: Purge | L: Link All | U: Unlink All | C: Purge All | q: Quit")
	}

	// Combine the title, viewport, status, and command bar.
	return titleBar + "\n" + m.viewport.View() + "\n" + commandBar + "\n" + statusBar
}
