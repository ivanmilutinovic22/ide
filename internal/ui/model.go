package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ide/internal/agentstatus"
	"ide/internal/config"
	"ide/internal/theme"
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

const (
	envEditFieldWindows = iota
)

const (
	extractFieldName = iota
)

// AgentStatus, ProcessInfo, and WindowProcessInfo are re-exports of the
// agentstatus package types so callers in this package can keep using the
// shorter local names. The actual definitions and pure detection logic
// live in internal/agentstatus.
type (
	AgentStatus       = agentstatus.Status
	ProcessInfo       = agentstatus.ProcessInfo
	WindowProcessInfo = agentstatus.WindowInfo
)

const (
	AgentStatusIdle          = agentstatus.StatusIdle
	AgentStatusCooking       = agentstatus.StatusCooking
	AgentStatusAwaitingInput = agentstatus.StatusAwaitingInput
)

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
//
// envHaystack covers env-level search terms (env name + "running"/"up" when
// the session is live). It's kept separate from each window's haystack so
// the fuzzy match cannot span the env/window boundary — that produced
// false positives like query "edit" matching env "update-windows-view"
// window "term" via e..d..i..t scattered across both strings.
type fuzzyEnvCacheEntry struct {
	header      fuzzySearchItem
	envHaystack string
	windows     []fuzzyWinCacheEntry
}

// uiTheme is an alias for theme.Theme so existing callers in this package
// don't need to be rewritten. New code should reference theme.Theme directly.
type uiTheme = theme.Theme

const (
	defaultThemeAppBG = theme.DefaultAppBG
	defaultThemeAppFG = theme.DefaultAppFG
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
	envEditMode           bool
	envEditTarget         string
	envEditSpec           textinput.Model
	extractMode           bool
	extractTarget         string
	extractName           textinput.Model
	restartConfirm        string
	showShortcuts         bool
	shortcutCursor        int
	showThemePicker       bool
	themeQuery            textinput.Model
	themePickerCursor     int
	confirmMode           bool
	confirmKind           string // "env_delete" | "session_kill" | "template_delete"
	confirmTarget         string
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
	m.envEditSpec = newTextInput("Windows: ", "")
	m.extractName = newTextInput("Name: ", "")
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
