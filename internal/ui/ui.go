// internal/ui/ui.go
package ui

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jmfirth/hf-lms-sync/internal/fsutils"
	"github.com/jmfirth/hf-lms-sync/internal/logger"
)

// Default size used for initialization before WindowSizeMsg is received
const (
	defaultWidth  = 120
	defaultHeight = 30
)

// Define the keymap for the application
type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	Home       key.Binding
	End        key.Binding
	Search     key.Binding
	Link       key.Binding
	Unlink     key.Binding
	Purge      key.Binding
	LinkAll    key.Binding
	UnlinkAll  key.Binding
	PurgeAll   key.Binding
	ToggleHelp key.Binding
	Quit       key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Search, k.Link, k.Unlink, k.Purge, k.ToggleHelp, k.Quit}
}

// FullHelp returns keybindings for the expanded help view
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Home, k.End},
		{k.Link, k.Unlink, k.Purge},
		{k.LinkAll, k.UnlinkAll, k.PurgeAll},
		{k.Search, k.ToggleHelp, k.Quit},
	}
}

// Default keymap
var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Home: key.NewBinding(
		key.WithKeys("home"),
		key.WithHelp("home", "first item"),
	),
	End: key.NewBinding(
		key.WithKeys("end"),
		key.WithHelp("end", "last item"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	Link: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "link"),
	),
	Unlink: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "unlink"),
	),
	Purge: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "purge"),
	),
	LinkAll: key.NewBinding(
		key.WithKeys("L"),
		key.WithHelp("L", "link all"),
	),
	UnlinkAll: key.NewBinding(
		key.WithKeys("U"),
		key.WithHelp("U", "unlink all"),
	),
	PurgeAll: key.NewBinding(
		key.WithKeys("C"),
		key.WithHelp("C", "purge all"),
	),
	ToggleHelp: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c", "esc"),
		key.WithHelp("q", "quit"),
	),
}

// Style definitions
var (
	appStyle = lipgloss.NewStyle().
		Padding(1, 2)

	// Initialize styles - widths will be updated when we get window size
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			PaddingLeft(2).
			PaddingRight(2)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#3B82F6")).
			PaddingLeft(1).
			PaddingRight(1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#333333")).
			PaddingLeft(1).
			PaddingRight(1)
)

// modelItem represents a list item for the BubbleTea list component
type modelItem struct {
	model         fsutils.ModelInfo
	titleWidth    int
	selectedWidth int
}

// FilterValue implements list.Item interface
func (i modelItem) FilterValue() string {
	return strings.ToLower(i.model.OrganizationName + "/" + i.model.ModelName)
}

// Title implements list.Item interface
func (i modelItem) Title() string {
	return fmt.Sprintf("%s/%s", i.model.OrganizationName, i.model.ModelName)
}

// Description implements list.Item interface
func (i modelItem) Description() string {
	if i.model.IsStale {
		return "Stale - " + i.model.StaleReason
	} else if i.model.IsLinked {
		return "Linked"
	}
	return "Not linked"
}

// itemDelegate implements list.ItemDelegate interface
type itemDelegate struct {
	styles               map[string]lipgloss.Style
	selectedPrefix       string
	unselectedPrefix     string
	shortHelpStyle       lipgloss.Style
	fullHelpStyle        lipgloss.Style
	statusMessageLiftime time.Duration
}

// Height implements list.ItemDelegate
func (d itemDelegate) Height() int {
	return 1
}

// Spacing implements list.ItemDelegate
func (d itemDelegate) Spacing() int {
	return 0
}

// Update implements list.ItemDelegate
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

// Render implements list.ItemDelegate
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(modelItem)
	if !ok {
		return
	}

	var statusIcon string
	var statusStyle lipgloss.Style

	if item.model.IsStale {
		statusStyle = d.styles["stale"]
		statusIcon = "⦿"
	} else if item.model.IsLinked {
		statusStyle = d.styles["linked"]
		statusIcon = "⦿"
	} else {
		statusStyle = d.styles["unlinked"]
		statusIcon = "⦿"
	}

	isSelected := index == m.Index()
	titleStr := item.Title()
	
	var (
		prefix, line string
		title, desc  string
	)

	if isSelected {
		prefix = d.selectedPrefix
		title = d.styles["selectedTitle"].Render(titleStr)
		desc = d.styles["selectedDesc"].Render(item.Description())
	} else {
		prefix = d.unselectedPrefix
		title = d.styles["title"].Render(titleStr)
		desc = d.styles["desc"].Render(item.Description())
	}

	line = fmt.Sprintf("%s %s %s %s", prefix, statusStyle.Render(statusIcon), title, desc)
	fmt.Fprint(w, line)
}

// newItemDelegate creates a new item delegate with custom styling
func newItemDelegate() itemDelegate {
	// Define styles
	d := itemDelegate{
		styles: map[string]lipgloss.Style{
			"title": lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")),
			
			"selectedTitle": lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true),
			
			"desc": lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")),
			
			"selectedDesc": lipgloss.NewStyle().
				Foreground(lipgloss.Color("#DDDDDD")),
			
			"linked": lipgloss.NewStyle().
				Foreground(lipgloss.Color("#48BB78")),  // Green
			
			"unlinked": lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F6AD55")), // Yellow
			
			"stale": lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F56565")), // Red
		},
		shortHelpStyle:       lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")),
		fullHelpStyle:        lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")),
		selectedPrefix:       "›",
		unselectedPrefix:     " ",
		statusMessageLiftime: time.Second * 5,
	}

	return d
}

// UI model for bubbles
type model struct {
	// Data
	models   []fsutils.ModelInfo
	stale    []fsutils.ModelInfo
	combined []fsutils.ModelInfo
	
	// Bubbles components
	list          list.Model
	help          help.Model
	keymap        keyMap
	spinner       spinner.Model
	searchInput   textinput.Model
	
	// UI state
	width         int
	height        int
	ready         bool
	showFullHelp  bool
	status        string
	targetDir     string
	searching     bool
	loading       bool
	
	// Logging
	logger        *logger.Logger
}

// New creates and returns a new UI model
func New(targetDir string, appLogger *logger.Logger) tea.Model {
	// Load models
	models, _ := fsutils.LoadModels(targetDir)
	stale, _ := fsutils.FindStaleLinks(targetDir)
	combined := append(models, stale...)
	sort.Slice(combined, func(i, j int) bool {
		return combined[i].CacheDirName < combined[j].CacheDirName
	})
	
	if appLogger != nil && appLogger.Verbose {
		appLogger.Info("UI", "Initializing UI with %d models and %d stale references", len(models), len(stale))
	}
	
	// Set up the list
	delegate := newItemDelegate()
	modelsList := list.New([]list.Item{}, delegate, defaultWidth, defaultHeight-7)
	modelsList.SetShowStatusBar(false)
	modelsList.SetFilteringEnabled(false)
	modelsList.SetShowTitle(false)
	modelsList.SetShowHelp(false)
	modelsList.SetStatusBarItemName("model", "models")
	
	// Convert models to list items
	var items []list.Item
	for _, m := range combined {
		items = append(items, modelItem{model: m})
	}
	modelsList.SetItems(items)
	
	// Set up spinner for loading state
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	
	// Set up help
	h := help.New()
	h.ShowAll = false
	
	// Set up search input
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 32
	ti.Width = 30
	
	return model{
		models:      models,
		stale:       stale,
		combined:    combined,
		list:        modelsList,
		help:        h,
		keymap:      keys,
		spinner:     s,
		searchInput: ti,
		status:      fmt.Sprintf("Found %d model(s) and %d stale reference(s).", len(models), len(stale)),
		targetDir:   targetDir,
		logger:      appLogger,
	}
}

// Init initializes the model
func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.spinner.Tick,
	)
}

// opResultMsg is used to update the UI state with fresh model data
type opResultMsg struct {
	status string
	models []fsutils.ModelInfo
	stale  []fsutils.ModelInfo
}

// errorMsg is used to pass error information to the UI
type errorMsg string

// ListItemsMsg is a custom message for setting list items
type ListItemsMsg []list.Item

// updateModelListCmd updates the model list after operations
func updateModelListCmd(m model, combined []fsutils.ModelInfo) tea.Cmd {
	return func() tea.Msg {
		var items []list.Item
		for _, mdl := range combined {
			items = append(items, modelItem{model: mdl})
		}
		return ListItemsMsg(items)
	}
}

// Update handles messages and updates the model
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)
	
	switch msg := msg.(type) {
	case ListItemsMsg:
		// Custom message to update list items
		items := []list.Item(msg)
		m.list.SetItems(items)
		return m, nil
		
	case tea.KeyMsg:
		// Handle key shortcuts based on current mode
		if m.searching {
			// In search mode, handle only specific control keys specially
			switch msg.Type {
			case tea.KeyEsc, tea.KeyCtrlC: // Exit search mode
				m.searching = false
				m.searchInput.Blur()
				m.searchInput.SetValue("")
				return m, updateModelListCmd(m, m.combined)
				
			case tea.KeyEnter: // Complete search
				m.searching = false
				m.searchInput.Blur()
				m.status = fmt.Sprintf("Found %d matches for: %s", len(m.list.Items()), m.searchInput.Value())
				return m, nil
			}
			
			// Process all other input for search box
			var searchCmd tea.Cmd
			m.searchInput, searchCmd = m.searchInput.Update(msg)
			
			// Filter list based on search input
			cmds = append(cmds, searchCmd)
			if m.searchInput.Value() != "" {
				filtered := filterModels(m.combined, m.searchInput.Value())
				return m, updateModelListCmd(m, filtered)
			} else {
				return m, updateModelListCmd(m, m.combined)
			}
		}
		
		// Normal mode keyboard shortcuts
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
			
		case key.Matches(msg, keys.ToggleHelp):
			m.showFullHelp = !m.showFullHelp
			
		case key.Matches(msg, keys.Search):
			m.searching = true
			m.searchInput.Focus()
			m.status = "Searching..."
			return m, nil
			
		case key.Matches(msg, keys.Link):
			if len(m.list.Items()) > 0 {
				selectedItem, ok := m.list.SelectedItem().(modelItem)
				if ok && !selectedItem.model.IsStale && !selectedItem.model.IsLinked {
					m.status = "Linking model: " + selectedItem.model.ModelName
					m.loading = true
					return m, tea.Batch(
						m.spinner.Tick,
						func() tea.Msg {
							return linkModelCmd(selectedItem.model, m.targetDir, m.logger)()
						},
					)
				}
			}
			
		case key.Matches(msg, keys.Unlink):
			if len(m.list.Items()) > 0 {
				selectedItem, ok := m.list.SelectedItem().(modelItem)
				if ok && !selectedItem.model.IsStale && selectedItem.model.IsLinked {
					m.status = "Unlinking model: " + selectedItem.model.ModelName
					m.loading = true
					return m, tea.Batch(
						m.spinner.Tick,
						func() tea.Msg {
							return unlinkModelCmd(selectedItem.model, m.targetDir, m.logger)()
						},
					)
				}
			}
			
		case key.Matches(msg, keys.Purge):
			if len(m.list.Items()) > 0 {
				selectedItem, ok := m.list.SelectedItem().(modelItem)
				if ok && selectedItem.model.IsStale {
					m.status = "Purging stale model: " + selectedItem.model.ModelName
					m.loading = true
					return m, tea.Batch(
						m.spinner.Tick,
						func() tea.Msg {
							return purgeModelCmd(selectedItem.model, m.targetDir, m.logger)()
						},
					)
				}
			}
			
		case key.Matches(msg, keys.LinkAll):
			m.status = "Linking all models..."
			m.loading = true
			return m, tea.Batch(
				m.spinner.Tick,
				func() tea.Msg {
					return linkAllCmd(m.models, m.targetDir, m.logger)()
				},
			)
			
		case key.Matches(msg, keys.UnlinkAll):
			m.status = "Unlinking all models..."
			m.loading = true
			return m, tea.Batch(
				m.spinner.Tick,
				func() tea.Msg {
					return unlinkAllCmd(m.models, m.targetDir, m.logger)()
				},
			)
			
		case key.Matches(msg, keys.PurgeAll):
			m.status = "Purging all stale links..."
			m.loading = true
			return m, tea.Batch(
				m.spinner.Tick,
				func() tea.Msg {
					return purgeAllCmd(m.stale, m.targetDir, m.logger)()
				},
			)
		}
		
	case tea.WindowSizeMsg:
		headerHeight := 3
		footerHeight := 4
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.width = msg.Width
			m.height = msg.Height
			m.list.SetWidth(msg.Width)
			m.list.SetHeight(msg.Height - verticalMarginHeight)
			m.ready = true
		} else {
			m.width = msg.Width
			m.height = msg.Height
			m.list.SetWidth(msg.Width)
			m.list.SetHeight(msg.Height - verticalMarginHeight)
		}
		
		m.help.Width = msg.Width
		
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
        
    // We don't need to handle list.SetItemsMsg anymore as we're using our custom ListItemsMsg
	
	case opResultMsg:
		m.status = msg.status
		m.models = msg.models
		m.stale = msg.stale
		m.combined = append(msg.models, msg.stale...)
		sort.Slice(m.combined, func(i, j int) bool {
			return m.combined[i].CacheDirName < m.combined[j].CacheDirName
		})
		
		// Update the list with new data
		var items []list.Item
		for _, mdl := range m.combined {
			items = append(items, modelItem{model: mdl})
		}
		
		m.loading = false
		cmds = append(cmds, m.list.SetItems(items))
		
	case errorMsg:
		m.status = string(msg)
		m.loading = false
	}
	
	// Update list with any pending commands
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	
	return m, tea.Batch(cmds...)
}

// Filter models based on search term
func filterModels(models []fsutils.ModelInfo, searchTerm string) []fsutils.ModelInfo {
    if searchTerm == "" {
        return models
    }
    
    lowerSearch := strings.ToLower(searchTerm)
    var filtered []fsutils.ModelInfo
    
    for _, m := range models {
        if strings.Contains(strings.ToLower(m.ModelName), lowerSearch) ||
           strings.Contains(strings.ToLower(m.OrganizationName), lowerSearch) {
            filtered = append(filtered, m)
        }
    }
    
    return filtered
}

// View renders the UI
func (m model) View() string {
	if !m.ready {
		return "\nInitializing..."
	}
	
	// Update styles to use current window width
	titleStyleWidth := titleStyle.Copy().Width(m.width - 4)
	statusStyleWidth := statusStyle.Copy().Width(m.width - 4)
	
	// Render header
	header := titleStyleWidth.Align(lipgloss.Center).Render("Hugging Face to LM Studio Sync")
	
	// Render info section
	hfCache, _ := fsutils.GetHfCacheDir()
	infoSection := lipgloss.JoinVertical(lipgloss.Left,
		fmt.Sprintf("Hugging Face Cache: %s", hfCache),
		fmt.Sprintf("LM Studio Models: %s", m.targetDir),
	)
	
	// Render status bar
	var statusBar string
	if m.loading {
		statusBar = lipgloss.JoinHorizontal(lipgloss.Left, 
			m.spinner.View(),
			" "+m.status,
		)
	} else {
		statusBar = m.status
	}
	
	// Render help
	var helpView string
	if m.showFullHelp {
		helpView = m.help.View(m.keymap)
	} else {
		helpView = m.help.ShortHelpView([]key.Binding{
			keys.Link,
			keys.Unlink,
			keys.Purge,
			keys.LinkAll,
			keys.UnlinkAll,
			keys.PurgeAll,
			keys.Search,
			keys.ToggleHelp,
			keys.Quit,
		})
	}
	
	// Render search box if searching
	var searchView string
	if m.searching {
		searchStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(0, 1)
		
		searchView = searchStyle.Render(m.searchInput.View())
	}
	
	// Compose the UI
	var view string
	if m.searching {
		view = lipgloss.JoinVertical(lipgloss.Left,
			header,
			infoSection,
			searchView,
			m.list.View(),
			statusStyleWidth.Render(statusBar),
			helpView,
		)
	} else {
		view = lipgloss.JoinVertical(lipgloss.Left,
			header,
			infoSection,
			m.list.View(),
			statusStyleWidth.Render(statusBar),
			helpView,
		)
	}
	
	return appStyle.Render(view)
}

// All the command helpers below are retained from the original implementation
// but updated to work with the new UI

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
		return updateState(targetDir, fmt.Sprintf("Successfully linked %d models", linkedCount))
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
		return updateState(targetDir, fmt.Sprintf("Successfully unlinked %d models", unlinkedCount))
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
		return updateState(targetDir, fmt.Sprintf("Successfully purged %d stale links", purgedCount))
	}
}
