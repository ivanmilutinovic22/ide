package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ide/internal/config"
)

var (
	paneStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			ColorWhitespace(true)
	focusedPaneStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("236")).
				Foreground(lipgloss.Color("252")).
				ColorWhitespace(true)
	modalPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("39")).
			BorderBackground(lipgloss.Color("236")).
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			ColorWhitespace(true)
	selectedLineStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("24")).
				Bold(true)
	activeSessionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42")).
				Background(lipgloss.Color("236")).
				ColorWhitespace(true).
				Bold(true)
	inactiveSessionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Background(lipgloss.Color("236")).
				ColorWhitespace(true)
	selectedTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")).
				Background(lipgloss.Color("236")).
				ColorWhitespace(true).
				Bold(true)
	tabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Background(lipgloss.Color("236")).
			ColorWhitespace(true)
	backdropStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Background(lipgloss.Color("236")).
			ColorWhitespace(true)
	paneTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("236")).
			ColorWhitespace(true)
	statusStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	windowBoxStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("244"))
	selectedWindowBoxStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("236")).
				Foreground(lipgloss.Color("220")).
				Bold(true)
)

const (
	createFieldName = iota
	createFieldRoot
	createFieldTemplate
	createFieldCustomWindows
)

const (
	focusPaneEnvironments = iota
	focusPaneWindows
	focusPaneTemplates
)

const (
	templateFieldName = iota
	templateFieldWindows
)

// AgentStatus represents the detected status of an AI agent
type AgentStatus string

const (
	AgentStatusIdle          AgentStatus = "idle"
	AgentStatusCooking       AgentStatus = "cooking"
	AgentStatusAwaitingInput AgentStatus = "awaiting_input"
)

// ProcessInfo tracks process metrics for agent status detection
type ProcessInfo struct {
	PID       int
	CPU       float64
	State     string // R, S, D, T, etc.
	Timestamp time.Time
}

// WindowProcessInfo holds process info for a specific window
type WindowProcessInfo struct {
	Current          ProcessInfo
	Previous         ProcessInfo
	Status           AgentStatus
	LowActivityCount int     // Consecutive samples with low activity
	BaselineCPU      float64 // Average CPU when idle (awaiting_input)
	SampleCount      int     // Number of samples taken for baseline
}

// fuzzySearchItem represents a single item in the fuzzy search results.
// IsHeader=true marks a session group header (not selectable).
type fuzzySearchItem struct {
	EnvIndex    int
	WindowIndex int // -1 for headers
	EnvName     string
	WindowName  string
	Status      AgentStatus // for windows: window status; for headers: session-level status
	Tags        []string
	Running     bool
	IsHeader    bool
}

// fuzzyWinCacheEntry is a precomputed window entry for fuzzy search.
type fuzzyWinCacheEntry struct {
	item     fuzzySearchItem
	haystack string
}

// fuzzyEnvCacheEntry groups precomputed search data per environment.
type fuzzyEnvCacheEntry struct {
	header  fuzzySearchItem
	windows []fuzzyWinCacheEntry
}

type uiTheme struct {
	Name       string
	AppBG      string
	AppFG      string
	PaneBG     string
	Border     string
	SelectedFG string
	SelectedBG string
	Active     string
	Inactive   string
	Accent     string
	Muted      string
	Status     string
}

const (
	defaultThemeAppBG = "#1a1a1a"
	defaultThemeAppFG = "#dddddd"
)

type Model struct {
	width                 int
	height                int
	environments          []config.Environment
	templates             []config.Template
	focusPane             int
	selectedEnv           int
	selectedWindow        int
	selectedTemplate      int
	sessions              map[string]struct{}
	sessionWindows        map[string][]string
	createMode            bool
	createField           int
	createName            textinput.Model
	createRoot            textinput.Model
	createTemplate        int
	createCustom          textinput.Model
	templateMode          bool
	templateField         int
	templateName          textinput.Model
	templateSpec          textinput.Model
	templateEditing       bool
	templateOrigin        string
	showShortcuts         bool
	shortcutCursor        int
	showThemePicker       bool
	themeQuery            textinput.Model
	themePickerCursor     int
	killConfirm           string
	deleteConfirm         string
	templateDeleteConfirm string
	pendingSelect         string
	pendingTemplateSelect string
	themes                []uiTheme
	themeIndex            int
	status                string
	previewContent        string
	previewSession        string
	previewWindow         string
	previewProcess        string
	previewBG             string
	terminalBG            string
	windowProcessInfo     map[string]WindowProcessInfo // key: session:window
	showFuzzySearch       bool
	fuzzySearchQuery      textinput.Model
	fuzzySearchCursor     int
	fuzzySearchResults    []fuzzySearchItem
	fuzzySearchCache      []fuzzyEnvCacheEntry
	terminalMode          bool              // true = keys forwarded to embedded PTY
	embeddedTerm          *EmbeddedTerminal // live PTY + VT emulator
}

func newTextInput(prompt, placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = prompt
	ti.Placeholder = placeholder
	ti.CharLimit = 0
	return ti
}

func NewModel(terminalBG string) Model {
	m := Model{
		sessions:          map[string]struct{}{},
		sessionWindows:    map[string][]string{},
		windowProcessInfo: map[string]WindowProcessInfo{},
		focusPane:         focusPaneEnvironments,
		themes:            defaultThemes(),
		status:            "Loading environments...",
		terminalBG:        terminalBG,
	}
	m.createName = newTextInput("Name: ", "")
	m.createRoot = newTextInput("Root: ", "")
	m.createRoot.ShowSuggestions = true
	m.createCustom = newTextInput("Windows: ", "")
	m.templateName = newTextInput("Name: ", "")
	m.templateSpec = newTextInput("Windows: ", "")
	m.themeQuery = newTextInput("", "")
	m.fuzzySearchQuery = newTextInput("/ ", "Search sessions and windows...")
	m.applyCurrentTheme()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadConfigCmd(), loadSessionsCmd(), tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return previewTickMsg{}
	}))
}
