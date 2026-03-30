package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"ide/internal/config"
	"ide/internal/tmux"
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


}

type configLoadedMsg struct {
	envs      []config.Environment
	templates []config.Template
	theme     string
	err       error
}

type sessionsLoadedMsg struct {
	names   []string
	windows map[string][]string
	err     error
}

type attachReadyMsg struct {
	target string
	err    error
}

type attachDoneMsg struct {
	err error
}

type environmentCreatedMsg struct {
	env        config.Environment
	sessionErr error
	err        error
}

type windowMovedMsg struct {
	envName    string
	direction  int
	sessionErr error
	err        error
}

type templateSavedMsg struct {
	name   string
	edited bool
	err    error
}

type templateDeletedMsg struct {
	name string
	err  error
}

type sessionKilledMsg struct {
	session string
	err     error
}

type environmentDeletedMsg struct {
	environment string
	session     string
	killed      bool
	err         error
}

type themePersistedMsg struct {
	name string
	err  error
}

type panePreviewMsg struct {
	session string
	window  string
	content string
	process string
}

type previewTickMsg struct{}

// agentStatusUpdateMsg carries process info updates for agent status detection
type agentStatusUpdateMsg struct {
	session  string
	window   string
	procInfo ProcessInfo
}

func defaultThemes() []uiTheme {
	return []uiTheme{
		{
			Name:       "Midnight",
			AppBG:      "#0f1020",
			AppFG:      "#d7dce2",
			PaneBG:     "#17182b",
			Border:     "#3b3f63",
			SelectedFG: "#f8f8f2",
			SelectedBG: "#3a4a8a",
			Active:     "#8bd5ca",
			Inactive:   "#7f849c",
			Accent:     "#f9e2af",
			Muted:      "#a6adc8",
			Status:     "#cdd6f4",
		},
		{
			Name:       "Catppuccin Mocha",
			AppBG:      "#1e1e2e",
			AppFG:      "#cdd6f4",
			PaneBG:     "#181825",
			Border:     "#6c7086",
			SelectedFG: "#f5e0dc",
			SelectedBG: "#45475a",
			Active:     "#a6e3a1",
			Inactive:   "#7f849c",
			Accent:     "#89b4fa",
			Muted:      "#a6adc8",
			Status:     "#cdd6f4",
		},
		{
			Name:       "Catppuccin Latte",
			AppBG:      "#eff1f5",
			AppFG:      "#4c4f69",
			PaneBG:     "#e6e9ef",
			Border:     "#9ca0b0",
			SelectedFG: "#1e66f5",
			SelectedBG: "#ccd0da",
			Active:     "#40a02b",
			Inactive:   "#8c8fa1",
			Accent:     "#8839ef",
			Muted:      "#7c7f93",
			Status:     "#4c4f69",
		},
		{
			Name:       "Gruvbox Dark",
			AppBG:      "#282828",
			AppFG:      "#ebdbb2",
			PaneBG:     "#32302f",
			Border:     "#665c54",
			SelectedFG: "#fbf1c7",
			SelectedBG: "#504945",
			Active:     "#b8bb26",
			Inactive:   "#a89984",
			Accent:     "#fabd2f",
			Muted:      "#d5c4a1",
			Status:     "#ebdbb2",
		},
		{
			Name:       "Gruvbox Light",
			AppBG:      "#fbf1c7",
			AppFG:      "#3c3836",
			PaneBG:     "#f2e5bc",
			Border:     "#bdae93",
			SelectedFG: "#282828",
			SelectedBG: "#d5c4a1",
			Active:     "#79740e",
			Inactive:   "#928374",
			Accent:     "#af3a03",
			Muted:      "#665c54",
			Status:     "#3c3836",
		},
		{
			Name:       "Tokyo Night",
			AppBG:      "#1a1b26",
			AppFG:      "#c0caf5",
			PaneBG:     "#24283b",
			Border:     "#414868",
			SelectedFG: "#c0caf5",
			SelectedBG: "#33467c",
			Active:     "#9ece6a",
			Inactive:   "#565f89",
			Accent:     "#7aa2f7",
			Muted:      "#a9b1d6",
			Status:     "#c0caf5",
		},
		{
			Name:       "Nord",
			AppBG:      "#2e3440",
			AppFG:      "#d8dee9",
			PaneBG:     "#3b4252",
			Border:     "#4c566a",
			SelectedFG: "#eceff4",
			SelectedBG: "#434c5e",
			Active:     "#a3be8c",
			Inactive:   "#81a1c1",
			Accent:     "#88c0d0",
			Muted:      "#d8dee9",
			Status:     "#e5e9f0",
		},
		{
			Name:       "Dracula",
			AppBG:      "#282a36",
			AppFG:      "#f8f8f2",
			PaneBG:     "#303444",
			Border:     "#6272a4",
			SelectedFG: "#f8f8f2",
			SelectedBG: "#44475a",
			Active:     "#50fa7b",
			Inactive:   "#8be9fd",
			Accent:     "#bd93f9",
			Muted:      "#a4a7b5",
			Status:     "#f1fa8c",
		},
		{
			Name:       "Solarized Dark",
			AppBG:      "#002b36",
			AppFG:      "#93a1a1",
			PaneBG:     "#073642",
			Border:     "#586e75",
			SelectedFG: "#fdf6e3",
			SelectedBG: "#005f73",
			Active:     "#859900",
			Inactive:   "#657b83",
			Accent:     "#b58900",
			Muted:      "#839496",
			Status:     "#93a1a1",
		},
		{
			Name:       "Solarized Light",
			AppBG:      "#fdf6e3",
			AppFG:      "#586e75",
			PaneBG:     "#eee8d5",
			Border:     "#93a1a1",
			SelectedFG: "#073642",
			SelectedBG: "#d3cbb6",
			Active:     "#859900",
			Inactive:   "#93a1a1",
			Accent:     "#b58900",
			Muted:      "#657b83",
			Status:     "#586e75",
		},
		{
			Name:       "One Dark",
			AppBG:      "#282c34",
			AppFG:      "#abb2bf",
			PaneBG:     "#21252b",
			Border:     "#5c6370",
			SelectedFG: "#e6e6e6",
			SelectedBG: "#3e4451",
			Active:     "#98c379",
			Inactive:   "#7f848e",
			Accent:     "#61afef",
			Muted:      "#c5c8cf",
			Status:     "#abb2bf",
		},
		{
			Name:       "Monokai Pro",
			AppBG:      "#2d2a2e",
			AppFG:      "#fcfcfa",
			PaneBG:     "#36333a",
			Border:     "#727072",
			SelectedFG: "#fffef9",
			SelectedBG: "#5b595c",
			Active:     "#a9dc76",
			Inactive:   "#939293",
			Accent:     "#ffd866",
			Muted:      "#c1c0c0",
			Status:     "#fcfcfa",
		},
		{
			Name:       "Night Owl",
			AppBG:      "#011627",
			AppFG:      "#d6deeb",
			PaneBG:     "#0b1f33",
			Border:     "#24496a",
			SelectedFG: "#ffffff",
			SelectedBG: "#1d3b53",
			Active:     "#addb67",
			Inactive:   "#7fdbca",
			Accent:     "#82aaff",
			Muted:      "#7e97b0",
			Status:     "#d6deeb",
		},
		{
			Name:       "Rose Pine",
			AppBG:      "#191724",
			AppFG:      "#e0def4",
			PaneBG:     "#1f1d2e",
			Border:     "#6e6a86",
			SelectedFG: "#faf4ed",
			SelectedBG: "#403d52",
			Active:     "#9ccfd8",
			Inactive:   "#908caa",
			Accent:     "#ebbcba",
			Muted:      "#c4a7e7",
			Status:     "#e0def4",
		},
		{
			Name:       "Everforest Dark",
			AppBG:      "#2b3339",
			AppFG:      "#d3c6aa",
			PaneBG:     "#323d43",
			Border:     "#4f585e",
			SelectedFG: "#fdf6e3",
			SelectedBG: "#475258",
			Active:     "#a7c080",
			Inactive:   "#7fbbb3",
			Accent:     "#e69875",
			Muted:      "#9da9a0",
			Status:     "#d3c6aa",
		},
		{
			Name:       "Kanagawa",
			AppBG:      "#1f1f28",
			AppFG:      "#dcd7ba",
			PaneBG:     "#2a2a37",
			Border:     "#54546d",
			SelectedFG: "#f2ecbc",
			SelectedBG: "#363646",
			Active:     "#98bb6c",
			Inactive:   "#7e9cd8",
			Accent:     "#ffa066",
			Muted:      "#c8c093",
			Status:     "#dcd7ba",
		},
		{
			Name:       "Ayu Mirage",
			AppBG:      "#1f2430",
			AppFG:      "#cccac2",
			PaneBG:     "#242b38",
			Border:     "#5c6773",
			SelectedFG: "#ffffff",
			SelectedBG: "#34455e",
			Active:     "#87d96c",
			Inactive:   "#80bfff",
			Accent:     "#ffcc66",
			Muted:      "#b8c2cc",
			Status:     "#cccac2",
		},
		{
			Name:       "Ice",
			AppBG:      "#0f172a",
			AppFG:      "#e2e8f0",
			PaneBG:     "#1e293b",
			Border:     "#475569",
			SelectedFG: "#f8fafc",
			SelectedBG: "#334155",
			Active:     "#22d3ee",
			Inactive:   "#94a3b8",
			Accent:     "#38bdf8",
			Muted:      "#cbd5e1",
			Status:     "#e2e8f0",
		},
		{
			Name:       "Amber",
			AppBG:      "#1f1300",
			AppFG:      "#f8e7c2",
			PaneBG:     "#2b1a00",
			Border:     "#7a4b00",
			SelectedFG: "#1f1300",
			SelectedBG: "#f59e0b",
			Active:     "#fbbf24",
			Inactive:   "#d6a54f",
			Accent:     "#f59e0b",
			Muted:      "#fcd34d",
			Status:     "#fde68a",
		},
	}
}

func normalizeTheme(theme uiTheme) uiTheme {
	theme.Name = strings.TrimSpace(theme.Name)
	theme.AppBG = strings.TrimSpace(theme.AppBG)
	theme.AppFG = strings.TrimSpace(theme.AppFG)
	theme.PaneBG = strings.TrimSpace(theme.PaneBG)
	theme.Border = strings.TrimSpace(theme.Border)
	theme.SelectedFG = strings.TrimSpace(theme.SelectedFG)
	theme.SelectedBG = strings.TrimSpace(theme.SelectedBG)
	theme.Active = strings.TrimSpace(theme.Active)
	theme.Inactive = strings.TrimSpace(theme.Inactive)
	theme.Accent = strings.TrimSpace(theme.Accent)
	theme.Muted = strings.TrimSpace(theme.Muted)
	theme.Status = strings.TrimSpace(theme.Status)

	if theme.AppBG == "" {
		theme.AppBG = defaultThemeAppBG
	}
	if theme.AppFG == "" {
		theme.AppFG = defaultThemeAppFG
	}
	if theme.PaneBG == "" {
		theme.PaneBG = theme.AppBG
	}
	if theme.Border == "" {
		theme.Border = theme.PaneBG
	}
	if theme.SelectedFG == "" {
		theme.SelectedFG = theme.AppFG
	}
	if theme.Accent == "" {
		theme.Accent = theme.SelectedFG
	}
	if theme.SelectedBG == "" {
		theme.SelectedBG = theme.Border
	}
	if theme.Muted == "" {
		theme.Muted = theme.AppFG
	}
	if theme.Active == "" {
		theme.Active = theme.Accent
	}
	if theme.Inactive == "" {
		theme.Inactive = theme.Muted
	}
	if theme.Status == "" {
		theme.Status = theme.AppFG
	}

	return theme
}

func applyThemeStyles(theme uiTheme) {
	theme = normalizeTheme(theme)
	paneStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(theme.PaneBG)).
		Foreground(lipgloss.Color(theme.AppFG)).
		ColorWhitespace(true)
	focusedPaneStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(theme.PaneBG)).
		Foreground(lipgloss.Color(theme.AppFG)).
		ColorWhitespace(true)
	modalPaneStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Accent)).
		BorderBackground(lipgloss.Color(theme.PaneBG)).
		Background(lipgloss.Color(theme.PaneBG)).
		Foreground(lipgloss.Color(theme.AppFG)).
		ColorWhitespace(true)
	selectedLineStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.SelectedFG)).
		Background(lipgloss.Color(theme.SelectedBG)).
		ColorWhitespace(true).
		Bold(true)
	activeSessionStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Active)).
		Background(lipgloss.Color(theme.PaneBG)).
		ColorWhitespace(true).
		Bold(true)
	inactiveSessionStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Inactive)).
		Background(lipgloss.Color(theme.PaneBG)).
		ColorWhitespace(true)
	selectedTabStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent)).
		Background(lipgloss.Color(theme.PaneBG)).
		ColorWhitespace(true).
		Bold(true)
	tabStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Background(lipgloss.Color(theme.PaneBG)).
		ColorWhitespace(true)
	backdropStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Background(lipgloss.Color(theme.AppBG)).
		ColorWhitespace(true)
	paneTextStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.AppFG)).
		Background(lipgloss.Color(theme.PaneBG)).
		ColorWhitespace(true)
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Status)).Background(lipgloss.Color(theme.PaneBG))
	windowBoxStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(theme.PaneBG)).
		Foreground(lipgloss.Color(theme.Muted))
	selectedWindowBoxStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(theme.PaneBG)).
		Foreground(lipgloss.Color(theme.Accent)).
		Bold(true)
}

func applyTextInputTheme(ti *textinput.Model, theme uiTheme) {
	bg := lipgloss.Color(theme.PaneBG)
	fg := lipgloss.Color(theme.AppFG)
	muted := lipgloss.Color(theme.Muted)
	ti.TextStyle = lipgloss.NewStyle().Foreground(fg).Background(bg)
	ti.PromptStyle = lipgloss.NewStyle().Foreground(fg).Background(bg)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(muted).Background(bg)
	ti.CompletionStyle = lipgloss.NewStyle().Foreground(muted).Background(bg)
	ti.Cursor.TextStyle = lipgloss.NewStyle().Foreground(fg).Background(bg)
}

func (m *Model) applyCurrentTheme() {
	if len(m.themes) == 0 {
		m.themes = defaultThemes()
	}
	if len(m.themes) == 0 {
		return
	}
	if m.themeIndex < 0 || m.themeIndex >= len(m.themes) {
		m.themeIndex = 0
	}
	theme := m.currentTheme()
	applyThemeStyles(theme)
	applyTextInputTheme(&m.createName, theme)
	applyTextInputTheme(&m.createRoot, theme)
	applyTextInputTheme(&m.createCustom, theme)
	applyTextInputTheme(&m.templateName, theme)
	applyTextInputTheme(&m.templateSpec, theme)
	applyTextInputTheme(&m.themeQuery, theme)
	applyTextInputTheme(&m.fuzzySearchQuery, theme)
}

func (m Model) currentTheme() uiTheme {
	if len(m.themes) == 0 {
		return normalizeTheme(uiTheme{})
	}
	idx := m.themeIndex
	if idx < 0 || idx >= len(m.themes) {
		idx = 0
	}
	return normalizeTheme(m.themes[idx])
}

func (m Model) currentThemeName() string {
	if len(m.themes) == 0 {
		return ""
	}
	idx := m.themeIndex
	if idx < 0 || idx >= len(m.themes) {
		idx = 0
	}
	return m.themes[idx].Name
}

func (m Model) themeIndexByName(name string) (int, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, false
	}
	for idx := range m.themes {
		if strings.EqualFold(strings.TrimSpace(m.themes[idx].Name), name) {
			return idx, true
		}
	}
	return 0, false
}

func (m Model) filteredThemeIndices() []int {
	query := strings.ToLower(strings.TrimSpace(m.themeQuery.Value()))
	indices := make([]int, 0, len(m.themes))
	for idx := range m.themes {
		if query == "" || strings.Contains(strings.ToLower(m.themes[idx].Name), query) {
			indices = append(indices, idx)
		}
	}
	return indices
}

func (m *Model) normalizeThemePickerCursor() {
	indices := m.filteredThemeIndices()
	if len(indices) == 0 {
		m.themePickerCursor = 0
		return
	}
	if m.themePickerCursor < 0 {
		m.themePickerCursor = 0
	}
	if m.themePickerCursor >= len(indices) {
		m.themePickerCursor = len(indices) - 1
	}
}

func (m *Model) openThemePicker() tea.Cmd {
	m.showThemePicker = true
	m.showShortcuts = false
	m.themeQuery.SetValue("")
	m.themePickerCursor = 0
	m.themeQuery.Focus()
	indices := m.filteredThemeIndices()
	for i, themeIdx := range indices {
		if themeIdx == m.themeIndex {
			m.themePickerCursor = i
			break
		}
	}
	return textinput.Blink
}

func (m *Model) focusCreateField() {
	m.createName.Blur()
	m.createRoot.Blur()
	m.createCustom.Blur()
	switch m.createField {
	case createFieldName:
		m.createName.Focus()
	case createFieldRoot:
		m.createRoot.Focus()
	case createFieldCustomWindows:
		m.createCustom.Focus()
	}
}

func (m *Model) focusTemplateField() {
	m.templateName.Blur()
	m.templateSpec.Blur()
	switch m.templateField {
	case templateFieldName:
		m.templateName.Focus()
	case templateFieldWindows:
		m.templateSpec.Focus()
	}
}

func (m Model) updateThemePickerMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showThemePicker = false
		m.themeQuery.Blur()
		m.status = "Theme picker closed."
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		m.themePickerCursor--
		m.normalizeThemePickerCursor()
		return m, nil
	case "down", "j":
		m.themePickerCursor++
		m.normalizeThemePickerCursor()
		return m, nil
	case "enter":
		indices := m.filteredThemeIndices()
		if len(indices) == 0 {
			m.status = "No theme matches the search query."
			return m, nil
		}
		selectedThemeIndex := indices[m.themePickerCursor]
		m.themeIndex = selectedThemeIndex
		m.applyCurrentTheme()
		m.showThemePicker = false
		m.themeQuery.Blur()
		themeName := m.currentThemeName()
		m.status = "Theme: " + themeName
		return m, saveThemePreferenceCmd(themeName)
	default:
		var cmd tea.Cmd
		m.themeQuery, cmd = m.themeQuery.Update(msg)
		m.normalizeThemePickerCursor()
		return m, cmd
	}
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
	return tea.Batch(loadConfigCmd(), loadSessionsCmd(), previewTickCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+t" {
			if m.showThemePicker {
				m.showThemePicker = false
				m.themeQuery.Blur()
				m.status = "Theme picker closed."
				return m, nil
			}
			cmd := m.openThemePicker()
			m.status = "Theme picker open. Type to filter, Enter to apply."
			return m, cmd
		}
		if key == "ctrl+p" {
			if m.showFuzzySearch {
				m.showFuzzySearch = false
				m.fuzzySearchQuery.Blur()
				m.status = "Search closed."
				return m, nil
			}
			cmd := m.openFuzzySearch()
			m.status = "Search open. Type to filter, Enter to attach."
			return m, cmd
		}
		if key == "?" {
			m.showThemePicker = false
			m.showShortcuts = !m.showShortcuts
			if m.showShortcuts {
				// Start cursor on first non-header item
				m.shortcutCursor = 0
				items := shortcutsList()
				for i, item := range items {
					if !item.isHeader {
						m.shortcutCursor = i
						break
					}
				}
				m.status = "Shortcuts open. Press ? or Esc to close."
			} else {
				m.status = "Shortcuts closed."
			}
			return m, nil
		}

		if m.showFuzzySearch {
			return m.updateFuzzySearchMode(msg)
		}

		if m.showThemePicker {
			return m.updateThemePickerMode(msg)
		}

		if m.showShortcuts {
			return m.updateShortcutsMode(msg)
		}

		if m.createMode {
			return m.updateCreateMode(msg)
		}
		if m.templateMode {
			return m.updateTemplateMode(msg)
		}

		if key != "x" {
			m.killConfirm = ""
		}
		if key != "d" {
			m.deleteConfirm = ""
			m.templateDeleteConfirm = ""
		}
		if pane, ok := parsePaneShortcut(key); ok {
			m.focusPane = pane
			m.status = focusedPaneStatus(pane)
			return m, m.captureCurrentWindowCmd()
		}
		if idx, ok := parseIndexShortcut(key); ok {
			switch m.focusPane {
			case focusPaneEnvironments:
				if idx < len(m.environments) {
					m.selectedEnv = idx
					return m, m.captureCurrentWindowCmd()
				}
			case focusPaneTemplates:
				if idx < len(m.templates) {
					m.selectedTemplate = idx
				}
			case focusPaneWindows:
				if windows := m.currentWindowNames(); idx < len(windows) {
					m.selectedWindow = idx
					return m, m.captureCurrentWindowCmd()
				}
			}
			return m, nil
		}
		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.toggleFocusPane()
			m.status = focusedPaneStatus(m.focusPane)
			return m, m.captureCurrentWindowCmd()
		case "r":
			m.status = "Refreshing..."
			return m, tea.Batch(loadConfigCmd(), loadSessionsCmd())
		case "n":
			m.jumpToNextCookingSession(1)
			return m, m.captureCurrentWindowCmd()
		case "N":
			m.jumpToNextCookingSession(-1)
			return m, m.captureCurrentWindowCmd()
		}

		if m.focusPane == focusPaneEnvironments {
			return m.updateEnvironmentPanelKey(key)
		}
		if m.focusPane == focusPaneWindows {
			return m.updateWindowPanelKey(key)
		}
		return m.updateTemplatesPanelKey(key)

	case configLoadedMsg:
		if msg.err != nil {
			m.status = "Config error: " + msg.err.Error()
			return m, nil
		}
		m.environments = msg.envs
		m.templates = msg.templates
		if idx, ok := m.themeIndexByName(msg.theme); ok {
			if idx != m.themeIndex {
				m.themeIndex = idx
				m.applyCurrentTheme()
			}
		}
		m.normalizeCreateTemplate()
		if strings.TrimSpace(m.pendingSelect) != "" {
			for i := range m.environments {
				if strings.EqualFold(m.environments[i].Name, m.pendingSelect) {
					m.selectedEnv = i
					m.selectedWindow = 0
					break
				}
			}
			m.pendingSelect = ""
		}
		if strings.TrimSpace(m.pendingTemplateSelect) != "" {
			for i := range m.templates {
				if strings.EqualFold(m.templates[i].Name, m.pendingTemplateSelect) {
					m.selectedTemplate = i
					break
				}
			}
			m.pendingTemplateSelect = ""
		}
		m.normalizeSelection()
		if len(m.environments) == 0 {
			path, err := config.ConfigFilePath()
			if err != nil {
				m.status = "No environments configured."
			} else {
				m.status = "No environments configured in " + path
			}
		} else if strings.HasPrefix(m.status, "Loading") || strings.HasPrefix(m.status, "Refreshing") || strings.HasPrefix(m.status, "No environments configured") {
			m.status = "Ready. Enter attaches, Ctrl-b d detaches back here."
		}
		return m, nil

	case sessionsLoadedMsg:
		if msg.err != nil {
			m.status = "Tmux error: " + msg.err.Error()
			return m, nil
		}
		m.sessions = map[string]struct{}{}
		m.sessionWindows = map[string][]string{}
		for _, name := range msg.names {
			m.sessions[name] = struct{}{}
		}
		for session, windows := range msg.windows {
			m.sessionWindows[session] = windows
		}
		// Ensure search keybinding is set for tmux popup
		go func() {
			if exe, err := os.Executable(); err == nil {
				tmux.BindSearchKey(exe)
			}
		}()
		m.normalizeSelection()
		return m, m.captureCurrentWindowCmd()

	case panePreviewMsg:
		if msg.session != m.previewSession || msg.window != m.previewWindow {
			m.previewBG = detectPreviewBG(msg.content)
		}
		m.previewContent = msg.content
		m.previewSession = msg.session
		m.previewWindow = msg.window
		m.previewProcess = msg.process
		return m, nil

	case agentStatusUpdateMsg:
		log.Printf("[Update] Received agentStatusUpdateMsg for session=%s window=%s", msg.session, msg.window)
		// Update the window process info based on the message
		m.updateWindowProcessInfoFromMsg(msg.session, msg.window, msg.procInfo)
		// Refresh search results so status changes appear live
		if m.showFuzzySearch {
			m.fuzzySearchResults = m.computeFuzzySearchResults()
		}
		return m, nil

	case previewTickMsg:
		return m, tea.Batch(m.captureCurrentWindowCmd(), previewTickCmd())

	case themePersistedMsg:
		if msg.err != nil {
			m.status = "Theme applied but not saved: " + msg.err.Error()
			return m, nil
		}
		m.status = "Theme: " + msg.name
		return m, nil

	case attachReadyMsg:
		if msg.err != nil {
			m.status = "Attach failed: " + msg.err.Error()
			return m, nil
		}
		m.status = "Attached. Detach with Ctrl-b d to return."
		return m, execAttachCmd(msg.target)

	case attachDoneMsg:
		if msg.err != nil {
			m.status = "Returned from tmux with error: " + msg.err.Error()
		} else {
			m.status = "Returned from tmux."
		}
		return m, loadSessionsCmd()

	case environmentCreatedMsg:
		if msg.err != nil {
			m.status = "Create failed: " + msg.err.Error()
			return m, nil
		}
		m.createMode = false
		m.createName.SetValue("")
		m.createRoot.SetValue("")
		m.createName.Blur()
		m.createRoot.Blur()
		m.createCustom.Blur()
		m.createField = createFieldName
		m.createTemplate = m.defaultTemplateIndex()
		m.createCustom.SetValue("")
		m.pendingSelect = msg.env.Name
		if msg.sessionErr != nil {
			m.status = "Environment saved, but tmux session was not created: " + msg.sessionErr.Error()
		} else {
			m.status = "Environment and tmux session created. Press Enter to attach."
		}
		return m, tea.Batch(loadConfigCmd(), loadSessionsCmd())

	case windowMovedMsg:
		if msg.err != nil {
			m.status = "Reorder failed: " + msg.err.Error()
			return m, nil
		}
		if msg.direction < 0 {
			m.moveWindow(-1)
		} else {
			m.moveWindow(1)
		}
		if msg.sessionErr != nil {
			m.status = "Template order saved, but live tmux reorder failed: " + msg.sessionErr.Error()
		} else if msg.direction < 0 {
			m.status = "Window moved left."
		} else {
			m.status = "Window moved right."
		}
		return m, tea.Batch(loadConfigCmd(), loadSessionsCmd())

	case templateSavedMsg:
		if msg.err != nil {
			m.status = "Template save failed: " + msg.err.Error()
			return m, nil
		}
		m.templateMode = false
		m.templateEditing = false
		m.templateOrigin = ""
		m.templateField = templateFieldName
		m.templateName.SetValue("")
		m.templateSpec.SetValue("")
		m.templateName.Blur()
		m.templateSpec.Blur()
		m.pendingTemplateSelect = msg.name
		if msg.edited {
			m.status = "Template updated: " + msg.name
		} else {
			m.status = "Template created: " + msg.name
		}
		return m, loadConfigCmd()

	case templateDeletedMsg:
		if msg.err != nil {
			m.status = "Template delete failed: " + msg.err.Error()
			return m, nil
		}
		m.templateDeleteConfirm = ""
		m.status = "Template deleted: " + msg.name
		return m, loadConfigCmd()

	case sessionKilledMsg:
		if msg.err != nil {
			m.status = "Kill failed: " + msg.err.Error()
			return m, nil
		}
		m.status = "Killed session: " + msg.session
		m.killConfirm = ""
		return m, loadSessionsCmd()

	case environmentDeletedMsg:
		if msg.err != nil {
			m.status = "Delete failed: " + msg.err.Error()
			return m, nil
		}
		m.deleteConfirm = ""
		if msg.killed {
			m.status = "Deleted environment " + msg.environment + " and killed session " + msg.session
		} else {
			m.status = "Deleted environment " + msg.environment
		}
		return m, tea.Batch(loadConfigCmd(), loadSessionsCmd())
	}

	// Route unhandled messages (e.g. cursor blink ticks) to the focused textinput.
	if m.createMode || m.templateMode || m.showThemePicker || m.showFuzzySearch {
		var cmd tea.Cmd
		if m.showFuzzySearch {
			m.fuzzySearchQuery, cmd = m.fuzzySearchQuery.Update(msg)
			return m, cmd
		}
		if m.createMode {
			switch m.createField {
			case createFieldName:
				m.createName, cmd = m.createName.Update(msg)
			case createFieldRoot:
				m.createRoot, cmd = m.createRoot.Update(msg)
			case createFieldCustomWindows:
				m.createCustom, cmd = m.createCustom.Update(msg)
			}
		} else if m.templateMode {
			switch m.templateField {
			case templateFieldName:
				m.templateName, cmd = m.templateName.Update(msg)
			case templateFieldWindows:
				m.templateSpec, cmd = m.templateSpec.Update(msg)
			}
		} else if m.showThemePicker {
			m.themeQuery, cmd = m.themeQuery.Update(msg)
		}
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	leftWidth, rightWidth := splitPaneWidths(m.width - 1) // -1 for horizontal gap

	bodyHeight := m.height - 2 // -1 status bar, -1 bottom gap
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	rightPaneHeight := bodyHeight
	if rightPaneHeight < 1 {
		rightPaneHeight = 1
	}
	leftContentTotal := bodyHeight - 1 // -1 for vertical gap
	if leftContentTotal < 2 {
		leftContentTotal = 2
	}

	theme := m.currentTheme()
	gapBG := lipgloss.Color(theme.AppBG)

	topHeight, bottomHeight := splitLeftPaneHeights(leftContentTotal)
	leftTopPane := m.renderEnvironmentPane(leftWidth, topHeight)
	leftBottomPane := m.renderTemplatesPane(leftWidth, bottomHeight)
	verticalGap := lipgloss.NewStyle().Width(leftWidth).Background(gapBG).Render("")
	leftPane := lipgloss.JoinVertical(lipgloss.Left, leftTopPane, verticalGap, leftBottomPane)
	rightPane := m.renderDetailsPane(rightWidth, rightPaneHeight)
	horizontalGap := lipgloss.NewStyle().Width(1).Height(bodyHeight).Background(gapBG).Render("")
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, horizontalGap, rightPane)
	if m.createMode || m.templateMode {
		bodyWidth := lipgloss.Width(body)
		bodyHeight := lipgloss.Height(body)
		popupWidth, popupHeight := modalPopupDimensions(bodyWidth, bodyHeight, rightWidth, rightPaneHeight)
		popup := m.renderCreatePane(popupWidth, popupHeight)
		if m.templateMode {
			popup = m.renderTemplatePane(popupWidth, popupHeight)
		}
		body = overlayCentered(body, popup)
	}
	if m.showShortcuts || m.showThemePicker {
		bodyWidth := lipgloss.Width(body)
		bodyHeight := lipgloss.Height(body)
		popupWidth := bodyWidth - 8
		if popupWidth > 92 {
			popupWidth = 92
		}
		if popupWidth < 44 {
			popupWidth = bodyWidth - 2
		}
		if popupWidth < 20 {
			popupWidth = 20
		}

		popupHeight := bodyHeight - 4
		if popupHeight > 30 {
			popupHeight = 30
		}
		if popupHeight < 10 {
			popupHeight = bodyHeight
		}
		if popupHeight < 6 {
			popupHeight = 6
		}

		popup := m.renderShortcutsPane(popupWidth, popupHeight)
		if m.showThemePicker {
			popup = m.renderThemePickerPane(popupWidth, popupHeight)
		}
		body = overlayCentered(body, popup)
	}
	if m.showFuzzySearch {
		bw := lipgloss.Width(body)
		bh := lipgloss.Height(body)
		popupWidth := bw - 6
		if popupWidth > 100 {
			popupWidth = 100
		}
		if popupWidth < 44 {
			popupWidth = bw - 2
		}
		if popupWidth < 20 {
			popupWidth = 20
		}
		popupHeight := bh - 2
		if popupHeight > 42 {
			popupHeight = 42
		}
		if popupHeight < 10 {
			popupHeight = bh
		}
		popup := m.renderFuzzySearchPane(popupWidth, popupHeight)
		body = overlayCentered(body, popup)
	}
	statusText := fitLineToWidth(m.statusLineText(), m.width)
	statusBgSeq := bgANSISeq(statusStyle.GetBackground())
	statusText = statusBgSeq + strings.ReplaceAll(statusText, "\x1b[0m", "\x1b[0m"+statusBgSeq) + "\x1b[0m"
	status := statusStyle.Width(m.width).Render(statusText)
	bottomGap := lipgloss.NewStyle().Width(m.width).Background(gapBG).Render("")
	rendered := lipgloss.JoinVertical(lipgloss.Left, body, bottomGap, status)

	if m.width > 0 && m.height > 0 {
		rendered = lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Left,
			lipgloss.Top,
			rendered,
			lipgloss.WithWhitespaceBackground(lipgloss.Color(theme.AppBG)),
			lipgloss.WithWhitespaceForeground(lipgloss.Color(theme.AppFG)),
		)
	}

	// Safety: ensure output never exceeds terminal height to prevent scrolling.
	if m.height > 0 {
		rendered = truncateLines(rendered, m.height)
	}

	return rendered
}

func (m *Model) moveEnv(delta int) {
	if len(m.environments) == 0 {
		m.selectedEnv = 0
		m.selectedWindow = 0
		return
	}
	m.selectedEnv += delta
	if m.selectedEnv < 0 {
		m.selectedEnv = 0
	}
	if m.selectedEnv >= len(m.environments) {
		m.selectedEnv = len(m.environments) - 1
	}
	m.selectedWindow = 0
}

func (m *Model) moveWindow(delta int) {
	windows := m.currentWindowNames()
	if len(windows) == 0 {
		m.selectedWindow = 0
		return
	}
	m.selectedWindow += delta
	if m.selectedWindow < 0 {
		m.selectedWindow = 0
	}
	if m.selectedWindow >= len(windows) {
		m.selectedWindow = len(windows) - 1
	}
}

func (m *Model) toggleFocusPane() {
	if m.focusPane == focusPaneEnvironments {
		m.focusPane = focusPaneWindows
		return
	}
	if m.focusPane == focusPaneWindows {
		m.focusPane = focusPaneTemplates
		return
	}
	m.focusPane = focusPaneEnvironments
}

func parsePaneShortcut(key string) (int, bool) {
	switch key {
	case "s":
		return focusPaneEnvironments, true
	case "w":
		return focusPaneWindows, true
	case "t":
		return focusPaneTemplates, true
	default:
		return 0, false
	}
}

func parseIndexShortcut(key string) (int, bool) {
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		return int(key[0] - '1'), true
	}
	return 0, false
}

func focusedPaneStatus(pane int) string {
	switch pane {
	case focusPaneEnvironments:
		return "Focused [s] Sessions panel"
	case focusPaneWindows:
		return "Focused [w] Windows panel"
	case focusPaneTemplates:
		return "Focused [t] Templates panel"
	default:
		return "Focused panel"
	}
}

func (m Model) updateEnvironmentPanelKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		m.moveEnv(-1)
		return m, m.captureCurrentWindowCmd()
	case "down", "j":
		m.moveEnv(1)
		return m, m.captureCurrentWindowCmd()
	case "a":
		m.createMode = true
		m.templateMode = false
		m.createField = createFieldName
		m.createName.SetValue("")
		m.createRoot.SetValue("")
		m.createTemplate = m.defaultTemplateIndex()
		m.createCustom.SetValue("")
		m.focusCreateField()
		m.status = "Create mode: enter environment name and root path."
		return m, textinput.Blink
	case "t":
		m.status = "Switch to [3] Templates panel for template actions"
		return m, nil
	case "x":
		return m.startKillSession()
	case "d":
		return m.startDeleteEnvironment()
	case "left", "right", "h", "l":
		m.status = "[1] Sessions panel focused"
		return m, nil
	case "H", "L":
		m.status = "Window reorder is available in [2] Windows panel"
		return m, nil
	case "enter":
		return m.startAttachSelected()
	default:
		return m, nil
	}
}

func (m Model) updateWindowPanelKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "left", "h", "up", "k":
		m.moveWindow(-1)
		return m, m.captureCurrentWindowCmd()
	case "right", "l", "down", "j":
		m.moveWindow(1)
		return m, m.captureCurrentWindowCmd()
	case "x", "d":
		m.status = "Switch to [1] Sessions panel for this action"
		return m, nil
	case "a":
		m.status = "Switch to [1] Sessions panel to create environments"
		return m, nil
	case "t", "e", "c":
		m.status = "Switch to [3] Templates panel for template actions"
		return m, nil
	case "H":
		return m.startMoveWindow(-1)
	case "L":
		return m.startMoveWindow(1)
	case "enter":
		return m.startAttachSelected()
	default:
		return m, nil
	}
}

func (m Model) updateTemplatesPanelKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		m.moveTemplate(-1)
		return m, nil
	case "down", "j":
		m.moveTemplate(1)
		return m, nil
	case "a":
		return m.openCreateTemplateMode()
	case "e", "enter":
		return m.startEditTemplateMode()
	case "d":
		return m.startDeleteTemplate()
	case "left", "right", "h", "l":
		m.status = "[3] Templates panel focused"
		return m, nil
	case "x":
		m.status = "Session kill is available in [1] Sessions panel"
		return m, nil
	default:
		return m, nil
	}
}

func (m *Model) moveTemplate(delta int) {
	if len(m.templates) == 0 {
		m.selectedTemplate = 0
		return
	}
	m.selectedTemplate += delta
	if m.selectedTemplate < 0 {
		m.selectedTemplate = 0
	}
	if m.selectedTemplate >= len(m.templates) {
		m.selectedTemplate = len(m.templates) - 1
	}
}

func (m Model) currentTemplate() (config.Template, bool) {
	if len(m.templates) == 0 {
		return config.Template{}, false
	}
	if m.selectedTemplate < 0 || m.selectedTemplate >= len(m.templates) {
		return config.Template{}, false
	}
	return m.templates[m.selectedTemplate], true
}

func (m Model) openCreateTemplateMode() (Model, tea.Cmd) {
	m.templateMode = true
	m.createMode = false
	m.templateField = templateFieldName
	m.templateName.SetValue("")
	m.templateSpec.SetValue("")
	m.templateEditing = false
	m.templateOrigin = ""
	m.focusTemplateField()
	m.status = "Template mode: name and window spec."
	return m, textinput.Blink
}

func (m Model) startEditTemplateMode() (tea.Model, tea.Cmd) {
	tpl, ok := m.currentTemplate()
	if !ok {
		m.status = "No template selected."
		return m, nil
	}
	m.templateMode = true
	m.createMode = false
	m.templateField = templateFieldName
	m.templateName.SetValue(tpl.Name)
	m.templateSpec.SetValue(formatWindowSpec(tpl.Windows))
	m.templateEditing = true
	m.templateOrigin = tpl.Name
	m.focusTemplateField()
	m.status = "Edit template mode."
	return m, textinput.Blink
}

func (m Model) startDeleteTemplate() (tea.Model, tea.Cmd) {
	tpl, ok := m.currentTemplate()
	if !ok {
		m.status = "No template selected."
		return m, nil
	}
	name := strings.TrimSpace(tpl.Name)
	if name == "" {
		m.status = "Selected template has empty name."
		return m, nil
	}
	if m.templateDeleteConfirm != name {
		m.templateDeleteConfirm = name
		m.status = "Press d again to delete template: " + name
		return m, nil
	}
	m.templateDeleteConfirm = ""
	m.status = "Deleting template..."
	return m, deleteTemplateCmd(name)
}

func (m Model) startAttachSelected() (tea.Model, tea.Cmd) {
	env, ok := m.currentEnv()
	if !ok {
		m.status = "No environment selected."
		return m, nil
	}
	windows := m.currentWindowNames()
	wName := ""
	if len(windows) > 0 && m.selectedWindow < len(windows) {
		wName = windows[m.selectedWindow]
	}
	m.status = "Preparing tmux session..."
	return m, prepareAttachCmd(env, wName)
}

func (m Model) startMoveWindow(direction int) (tea.Model, tea.Cmd) {
	env, ok := m.currentEnv()
	if !ok {
		m.status = "No environment selected."
		return m, nil
	}
	windows := m.currentWindowNames()
	if len(windows) < 2 {
		m.status = "Need at least two windows to reorder."
		return m, nil
	}
	if m.selectedWindow < 0 || m.selectedWindow >= len(windows) {
		m.status = "No window selected."
		return m, nil
	}
	if direction < 0 && m.selectedWindow == 0 {
		m.status = "Window is already first."
		return m, nil
	}
	if direction > 0 && m.selectedWindow == len(windows)-1 {
		m.status = "Window is already last."
		return m, nil
	}

	windowName := windows[m.selectedWindow]
	m.status = "Reordering window..."
	return m, moveWindowOrderCmd(env.Name, windowName, direction)
}

func (m Model) startKillSession() (tea.Model, tea.Cmd) {
	env, ok := m.currentEnv()
	if !ok {
		m.status = "No environment selected."
		return m, nil
	}
	session := tmux.SessionName(env.Name)
	if _, running := m.sessions[session]; !running {
		m.status = "Session is not running: " + session
		m.killConfirm = ""
		return m, nil
	}
	if m.killConfirm != session {
		m.killConfirm = session
		m.status = "Press x again to kill session: " + session
		return m, nil
	}
	m.killConfirm = ""
	m.status = "Killing session..."
	return m, killSessionCmd(session)
}

func (m Model) startDeleteEnvironment() (tea.Model, tea.Cmd) {
	env, ok := m.currentEnv()
	if !ok {
		m.status = "No environment selected."
		return m, nil
	}
	name := strings.TrimSpace(env.Name)
	if name == "" {
		m.status = "Selected environment has empty name."
		return m, nil
	}
	if m.deleteConfirm != name {
		m.deleteConfirm = name
		m.status = "Press d again to delete environment: " + name
		return m, nil
	}
	m.deleteConfirm = ""
	m.status = "Deleting environment..."
	return m, deleteEnvironmentCmd(name)
}

func (m *Model) normalizeCreateTemplate() {
	if len(m.templates) == 0 {
		m.createTemplate = 0
		return
	}
	total := len(m.templates) + 1
	if m.createTemplate < 0 {
		m.createTemplate = 0
	}
	if m.createTemplate >= total {
		m.createTemplate = 0
	}
}

func (m Model) defaultTemplateIndex() int {
	for idx := range m.templates {
		if strings.EqualFold(m.templates[idx].Name, "default") {
			return idx
		}
	}
	return 0
}

func (m Model) isCustomTemplateSelected() bool {
	if len(m.templates) == 0 {
		return true
	}
	return m.createTemplate >= len(m.templates)
}

func (m Model) createTemplateOptions() []string {
	options := make([]string, 0, len(m.templates)+1)
	for _, t := range m.templates {
		options = append(options, t.Name)
	}
	options = append(options, "custom")
	return options
}

func (m Model) selectedCreateTemplateName() string {
	options := m.createTemplateOptions()
	if len(options) == 0 {
		return "custom"
	}
	idx := m.createTemplate
	if idx < 0 {
		idx = 0
	}
	if idx >= len(options) {
		idx = len(options) - 1
	}
	return options[idx]
}

func (m *Model) moveCreateTemplate(delta int) {
	options := m.createTemplateOptions()
	if len(options) == 0 {
		m.createTemplate = 0
		return
	}
	m.createTemplate += delta
	if m.createTemplate < 0 {
		m.createTemplate = len(options) - 1
	}
	if m.createTemplate >= len(options) {
		m.createTemplate = 0
	}
	if !m.isCustomTemplateSelected() && m.createField == createFieldCustomWindows {
		m.createField = createFieldTemplate
	}
}

func (m Model) createFieldOrder() []int {
	order := []int{createFieldName, createFieldRoot, createFieldTemplate}
	if m.isCustomTemplateSelected() {
		order = append(order, createFieldCustomWindows)
	}
	return order
}

func (m *Model) shiftCreateField(direction int) {
	order := m.createFieldOrder()
	if len(order) == 0 {
		m.createField = createFieldName
		return
	}
	current := 0
	for i := range order {
		if order[i] == m.createField {
			current = i
			break
		}
	}
	current += direction
	if current < 0 {
		current = len(order) - 1
	}
	if current >= len(order) {
		current = 0
	}
	m.createField = order[current]
}

func (m Model) isCreateLastField() bool {
	order := m.createFieldOrder()
	if len(order) == 0 {
		return true
	}
	return m.createField == order[len(order)-1]
}

func (m *Model) normalizeSelection() {
	if len(m.environments) == 0 {
		m.selectedEnv = 0
		m.selectedWindow = 0
	} else {
		if m.selectedEnv >= len(m.environments) {
			m.selectedEnv = len(m.environments) - 1
		}
		if m.selectedEnv < 0 {
			m.selectedEnv = 0
		}
		wins := m.currentWindowNames()
		if len(wins) == 0 {
			m.selectedWindow = 0
		} else {
			if m.selectedWindow >= len(wins) {
				m.selectedWindow = len(wins) - 1
			}
			if m.selectedWindow < 0 {
				m.selectedWindow = 0
			}
		}
	}

	if len(m.templates) == 0 {
		m.selectedTemplate = 0
		return
	}
	if m.selectedTemplate >= len(m.templates) {
		m.selectedTemplate = len(m.templates) - 1
	}
	if m.selectedTemplate < 0 {
		m.selectedTemplate = 0
	}
}

func (m Model) currentEnv() (config.Environment, bool) {
	if len(m.environments) == 0 {
		return config.Environment{}, false
	}
	if m.selectedEnv < 0 || m.selectedEnv >= len(m.environments) {
		return config.Environment{}, false
	}
	return m.environments[m.selectedEnv], true
}

func (m Model) currentWindowNames() []string {
	env, ok := m.currentEnv()
	if !ok {
		return []string{}
	}
	session := tmux.SessionName(env.Name)
	if windows, ok := m.sessionWindows[session]; ok && len(windows) > 0 {
		return windows
	}
	return tmux.WindowNames(env)
}

func paneBoxStyle(width, height int, focused bool) lipgloss.Style {
	baseStyle := paneStyle
	if focused {
		baseStyle = focusedPaneStyle
	}
	return baseStyle.Width(width).Height(height).MaxHeight(height).Padding(0, 1)
}

func modalBoxStyle(width, height int) lipgloss.Style {
	return modalPaneStyle.Width(width-modalPaneStyle.GetHorizontalFrameSize()).Height(height).Padding(0, 1)
}

func splitPaneWidths(total int) (int, int) {
	leftWidth := total / 3
	if leftWidth < 28 {
		leftWidth = 28
	}
	if leftWidth > 44 {
		leftWidth = 44
	}

	rightWidth := total - leftWidth
	if rightWidth < 28 && total >= 56 {
		rightWidth = 28
		leftWidth = total - rightWidth
	}
	if rightWidth < 1 {
		rightWidth = 1
		leftWidth = total - rightWidth
	}
	if leftWidth < 1 {
		leftWidth = 1
	}

	return leftWidth, rightWidth
}

func modalPopupDimensions(bodyWidth, bodyHeight, desiredWidth, desiredHeight int) (int, int) {
	popupWidth := desiredWidth
	if popupWidth > 96 {
		popupWidth = 96
	}
	if popupWidth > bodyWidth-2 {
		popupWidth = bodyWidth - 2
	}
	if popupWidth < 44 {
		popupWidth = 44
	}
	if popupWidth > bodyWidth {
		popupWidth = bodyWidth
	}
	if popupWidth < 1 {
		popupWidth = 1
	}

	popupHeight := desiredHeight
	if popupHeight > 20 {
		popupHeight = 20
	}
	if popupHeight > bodyHeight-2 {
		popupHeight = bodyHeight - 2
	}
	if popupHeight < 10 {
		popupHeight = 10
	}
	if popupHeight > bodyHeight {
		popupHeight = bodyHeight
	}
	if popupHeight < 1 {
		popupHeight = 1
	}

	return popupWidth, popupHeight
}

func paneContentWidth(width int) int {
	contentWidth := width - 2 // -2 for Padding(0,1) in paneBoxStyle; no border frame
	if contentWidth < 0 {
		return 0
	}
	return contentWidth
}

func modalContentWidth(width int) int {
	contentWidth := width - modalPaneStyle.GetHorizontalFrameSize() - 2 // border frame + padding
	if contentWidth < 0 {
		return 0
	}
	return contentWidth
}

func padLineToWidth(line string, width int) string {
	if width <= 0 {
		return line
	}
	lineWidth := ansi.StringWidth(line)
	if lineWidth >= width {
		return line
	}
	return line + strings.Repeat(" ", width-lineWidth)
}

func fitLineToWidth(line string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(line) > width {
		line = ansi.Cut(line, 0, width)
	}
	return padLineToWidth(line, width)
}

func (m Model) statusLineText() string {
	hints := m.contextShortcutHints()
	msg := strings.TrimSpace(m.status)
	if msg == "" || suppressStatusMessage(msg) {
		return hints
	}
	return hints + " | " + msg
}

func suppressStatusMessage(msg string) bool {
	if strings.HasPrefix(msg, "Ready.") {
		return true
	}
	if strings.HasPrefix(msg, "Focused ") {
		return true
	}
	if strings.HasSuffix(msg, "panel focused") {
		return true
	}
	if strings.HasPrefix(msg, "Shortcuts ") {
		return true
	}
	if strings.HasPrefix(msg, "Theme picker ") {
		return true
	}
	return false
}

// shortcutHint renders a single "key description" pair with the key bold and description muted.
func (m Model) shortcutHint(key, desc string) string {
	theme := m.currentTheme()
	k := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent)).
		Bold(true).
		Render(key)
	d := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Render(" " + desc)
	return k + d
}

func (m Model) contextShortcutHints() string {
	sep := "   "

	if m.showFuzzySearch {
		return strings.Join([]string{
			m.shortcutHint("↑↓", "navigate"),
			m.shortcutHint("enter", "attach"),
			m.shortcutHint("esc", "close"),
		}, sep)
	}
	if m.showThemePicker {
		return strings.Join([]string{
			m.shortcutHint("↑↓", "navigate"),
			m.shortcutHint("enter", "apply"),
			m.shortcutHint("esc", "close"),
		}, sep)
	}
	if m.showShortcuts {
		return strings.Join([]string{
			m.shortcutHint("?", "close"),
			m.shortcutHint("esc", "close"),
		}, sep)
	}
	if m.createMode || m.templateMode {
		return strings.Join([]string{
			m.shortcutHint("tab", "next field"),
			m.shortcutHint("enter", "confirm"),
			m.shortcutHint("esc", "cancel"),
		}, sep)
	}

	// Context-specific hints first, then global
	var hints []string
	switch m.focusPane {
	case focusPaneEnvironments:
		hints = append(hints,
			m.shortcutHint("j/k", "select"),
			m.shortcutHint("enter", "attach"),
			m.shortcutHint("a", "create"),
		)
	case focusPaneWindows:
		hints = append(hints,
			m.shortcutHint("h/l", "select"),
			m.shortcutHint("enter", "attach"),
			m.shortcutHint("H/L", "reorder"),
		)
	case focusPaneTemplates:
		hints = append(hints,
			m.shortcutHint("j/k", "select"),
			m.shortcutHint("a", "create"),
			m.shortcutHint("e", "edit"),
		)
	}
	hints = append(hints,
		m.shortcutHint("tab", "panels"),
		m.shortcutHint("ctrl+p", "search"),
		m.shortcutHint("n", "next ai"),
		m.shortcutHint("ctrl+t", "themes"),
		m.shortcutHint("?", "help"),
	)
	return strings.Join(hints, sep)
}

func renderStyledPaneLine(style lipgloss.Style, line string, width int) string {
	return style.Render(fitLineToWidth(line, width))
}

func panelTitle(shortcut string, name string, focused bool, theme uiTheme) string {
	color := theme.Muted
	if focused {
		color = theme.Accent
	}
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		Bold(focused)
	return style.Render(fmt.Sprintf("[%s] %s", shortcut, name))
}

func viewportSlice(rows []string, selected, maxVisible int) []string {
	if len(rows) <= maxVisible {
		return rows
	}
	start := 0
	if selected > maxVisible-1 {
		start = selected - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(rows) {
		end = len(rows)
		start = end - maxVisible
	}
	if start < 0 {
		start = 0
	}
	return rows[start:end]
}

func truncateLines(s string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n")
}

func renderPaneWithTitle(width, height int, title string, body string, focused bool) string {
	contentHeight := height - 1 // title takes 1 row
	if contentHeight < 0 {
		contentHeight = 0
	}
	body = applyPaneTextBackground(body, paneContentWidth(width))
	body = truncateLines(body, contentHeight)
	pane := paneBoxStyle(width, contentHeight, focused).Render(body)
	titleStyle := lipgloss.NewStyle().
		Background(paneStyle.GetBackground()).
		Width(width).
		Padding(0, 1)
	titleLine := titleStyle.Render(title)
	return lipgloss.JoinVertical(lipgloss.Left, titleLine, pane)
}

func renderModalWithBorderTitle(width, height int, borderTitle string, body string) string {
	body = applyPaneTextBackground(body, modalContentWidth(width))
	style := modalBoxStyle(width, height)
	pane := style.Render(body)
	return injectBorderTitle(pane, borderTitle, style)
}

// bgANSISeq extracts the raw ANSI escape sequence that sets a background color.
func bgANSISeq(c lipgloss.TerminalColor) string {
	rendered := lipgloss.NewStyle().Background(c).Render("X")
	if i := strings.Index(rendered, "X"); i > 0 {
		return rendered[:i]
	}
	return ""
}

func applyPaneTextBackground(body string, width int) string {
	if body == "" {
		return body
	}
	lines := strings.Split(body, "\n")
	bg := paneTextStyle.GetBackground()
	bgSeq := bgANSISeq(bg)
	for i := range lines {
		if ansi.StringWidth(lines[i]) > width {
			lines[i] = ansi.Cut(lines[i], 0, width)
		}
		if strings.Contains(lines[i], "\x1b[") {
			// Line has ANSI styling — pad with spaces, then replace inner
			// resets with reset+re-apply-bg so the background persists
			pad := width - ansi.StringWidth(lines[i])
			if pad > 0 {
				lines[i] = lines[i] + strings.Repeat(" ", pad)
			}
			lines[i] = bgSeq + strings.ReplaceAll(lines[i], "\x1b[0m", "\x1b[0m"+bgSeq) + "\x1b[0m"
		} else {
			lines[i] = paneTextStyle.Render(padLineToWidth(lines[i], width))
		}
	}
	return strings.Join(lines, "\n")
}

func injectBorderTitle(pane, title string, style lipgloss.Style) string {
	if strings.TrimSpace(title) == "" {
		return pane
	}
	lines := strings.Split(pane, "\n")
	if len(lines) == 0 {
		return pane
	}
	plainTop := ansi.Strip(lines[0])
	top := []rune(plainTop)
	if len(top) < 3 {
		return pane
	}

	interiorLen := len(top) - 2
	fill := top[1]
	if fill == ' ' {
		fill = '─'
	}

	titleRunes := []rune(title)
	if len(titleRunes) > interiorLen {
		titleRunes = titleRunes[:interiorLen]
	}

	interior := make([]rune, interiorLen)
	for i := range interior {
		interior[i] = fill
	}
	copy(interior, titleRunes)

	renderedTop := string(top[0]) + string(interior) + string(top[len(top)-1])
	topStyle := lipgloss.NewStyle()
	if c := style.GetBorderTopForeground(); c != nil {
		topStyle = topStyle.Foreground(c)
	} else if c := style.GetForeground(); c != nil {
		topStyle = topStyle.Foreground(c)
	}
	if c := style.GetBorderTopBackground(); c != nil {
		topStyle = topStyle.Background(c)
	} else if c := style.GetBackground(); c != nil {
		topStyle = topStyle.Background(c)
	}
	lines[0] = topStyle.Render(renderedTop)
	return strings.Join(lines, "\n")
}

func overlayCentered(base, overlay string) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")
	if len(baseLines) == 0 || len(overlayLines) == 0 {
		return base
	}

	baseW := lipgloss.Width(base)
	baseH := lipgloss.Height(base)
	overlayW := lipgloss.Width(overlay)
	overlayH := lipgloss.Height(overlay)

	startX := (baseW - overlayW) / 2
	startY := (baseH - overlayH) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}
	endY := startY + overlayH

	for y := range baseLines {
		if y < startY || y >= endY {
			baseLines[y] = backdropSegment(baseLines[y])
		}
	}

	for y := 0; y < overlayH; y++ {
		baseY := startY + y
		if baseY < 0 || baseY >= len(baseLines) || y >= len(overlayLines) {
			continue
		}
		baseLine := baseLines[baseY]
		overlayLine := overlayLines[y]
		lineWidth := ansi.StringWidth(baseLine)
		prefix := ansi.Cut(baseLine, 0, startX)
		suffix := ""
		if startX+overlayW < lineWidth {
			suffix = ansi.Cut(baseLine, startX+overlayW, lineWidth)
		}
		baseLines[baseY] = backdropSegment(prefix) + overlayLine + backdropSegment(suffix)
	}

	return strings.Join(baseLines, "\n")
}

func backdropSegment(segment string) string {
	if segment == "" {
		return ""
	}
	return backdropStyle.Render(ansi.Strip(segment))
}

func splitLeftPaneHeights(total int) (int, int) {
	if total <= 2 {
		return 1, 1
	}

	top := (total * 2) / 3
	if top < 1 {
		top = 1
	}
	if top > total-1 {
		top = total - 1
	}

	bottom := total - top
	if bottom < 1 {
		bottom = 1
		top = total - 1
	}

	return top, bottom
}

func (m Model) renderEnvironmentPane(width, height int) string {
	rows := make([]string, 0, len(m.environments)+1)
	focused := !m.createMode && !m.templateMode && m.focusPane == focusPaneEnvironments
	theme := m.currentTheme()
	title := panelTitle("s", "Sessions", focused, theme)
	contentWidth := paneContentWidth(width)

	if len(m.environments) == 0 {
		rows = append(rows, "")
		rows = append(rows, "No environments configured.")
		rows = append(rows, "Press a to create one or edit ~/.config/ide/environments.json")
		return renderPaneWithTitle(width, height, title, strings.Join(rows, "\n"), focused)
	}

	for idx, env := range m.environments {
		sessionName := tmux.SessionName(env.Name)
		_, running := m.sessions[sessionName]
		state := "down"
		if running {
			state = "up"
		}
		num := "   "
		if idx < 9 {
			num = fmt.Sprintf("[%d]", idx+1)
		}

		// Check agent status across all windows
		sessionStatus := AgentStatusIdle
		if running {
			sessionStatus = m.getSessionAgentStatus(env)
		}
		indicator := ""
		if sessionStatus == AgentStatusCooking {
			indicator = " ● Cooking"
		} else if sessionStatus == AgentStatusAwaitingInput {
			indicator = " ◆ Awaiting Input"
		}

		plainLine := fmt.Sprintf("%s %-20s [%s]%s", num, env.Name, state, indicator)
		line := "  " + plainLine
		if idx == m.selectedEnv {
			if sessionStatus != AgentStatusIdle {
				statusColor := m.getWindowStatusColor(sessionStatus)
				selStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color(statusColor)).
					Background(lipgloss.Color(theme.SelectedBG)).
					Bold(true)
				line = renderStyledPaneLine(selStyle, "> "+plainLine, contentWidth)
			} else {
				line = renderStyledPaneLine(selectedLineStyle, "> "+plainLine, contentWidth)
			}
		} else if running && sessionStatus != AgentStatusIdle {
			statusColor := m.getWindowStatusColor(sessionStatus)
			stStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(statusColor)).
				Background(lipgloss.Color(theme.PaneBG)).
				Bold(true)
			line = renderStyledPaneLine(stStyle, line, contentWidth)
		} else {
			if running {
				line = renderStyledPaneLine(activeSessionStyle, line, contentWidth)
			} else {
				line = renderStyledPaneLine(inactiveSessionStyle, line, contentWidth)
			}
		}
		rows = append(rows, line)
	}

	visibleHeight := height - 1 // title takes 1 row
	rows = viewportSlice(rows, m.selectedEnv, visibleHeight)
	return renderPaneWithTitle(width, height, title, strings.Join(rows, "\n"), focused)
}

func (m Model) renderTemplatesPane(width, height int) string {
	rows := make([]string, 0, len(m.templates)+2)
	focused := !m.createMode && !m.templateMode && m.focusPane == focusPaneTemplates
	theme := m.currentTheme()
	title := panelTitle("t", "Templates", focused, theme)
	contentWidth := paneContentWidth(width)

	if len(m.templates) == 0 {
		rows = append(rows, "")
		rows = append(rows, "No templates saved.")
		rows = append(rows, "Press a in this panel to add one.")
		return renderPaneWithTitle(width, height, title, strings.Join(rows, "\n"), focused)
	}

	for idx, tpl := range m.templates {
		num := "   "
		if idx < 9 {
			num = fmt.Sprintf("[%d]", idx+1)
		}
		line := fmt.Sprintf("%s %-15s (%d windows)", num, tpl.Name, len(tpl.Windows))
		if idx == m.selectedTemplate {
			line = renderStyledPaneLine(selectedLineStyle, "> "+line, contentWidth)
		} else {
			line = padLineToWidth("  "+line, contentWidth)
		}
		rows = append(rows, line)
	}

	visibleHeight := height - 1 // title takes 1 row
	rows = viewportSlice(rows, m.selectedTemplate, visibleHeight)
	return renderPaneWithTitle(width, height, title, strings.Join(rows, "\n"), focused)
}

func (m Model) renderDetailsPane(width, height int) string {
	focused := !m.createMode && !m.templateMode && m.focusPane == focusPaneWindows
	theme := m.currentTheme()
	title := panelTitle("w", "Windows", focused, theme)
	env, ok := m.currentEnv()
	if !ok {
		body := strings.Join([]string{"", "No environment selected."}, "\n")
		return renderPaneWithTitle(width, height, title, body, focused)
	}

	contentWidth := paneContentWidth(width)

	windows := m.currentWindowNames()
	session := tmux.SessionName(env.Name)
	tabs := make([]string, 0, len(windows))
	for i, w := range windows {
		// Get agent status for this window
		status := m.getWindowAgentStatus(session, w)

		// Format label with status suffix
		label := m.formatWindowLabel(w, status)
		if i < 9 {
			label = fmt.Sprintf("[%d] %s", i+1, label)
		}

		// Choose style based on status and selection
		if i == m.selectedWindow {
			// Selected window - use status color if active, otherwise accent
			if status != AgentStatusIdle {
				statusColor := m.getWindowStatusColor(status)
				statusStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color(statusColor)).
					Background(lipgloss.Color(theme.PaneBG)).
					Bold(true)
				tabs = append(tabs, statusStyle.Render(label))
			} else {
				tabs = append(tabs, selectedWindowBoxStyle.Render(label))
			}
		} else {
			// Unselected window - use status color if active
			if status != AgentStatusIdle {
				statusColor := m.getWindowStatusColor(status)
				statusStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color(statusColor)).
					Background(lipgloss.Color(theme.PaneBG))
				tabs = append(tabs, statusStyle.Render(label))
			} else {
				tabs = append(tabs, windowBoxStyle.Render(label))
			}
		}
	}
	sepStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(theme.PaneBG)).
		Foreground(lipgloss.Color(theme.Muted))
	tabsLine := strings.Join(tabs, sepStyle.Render(" - "))
	if ansi.StringWidth(tabsLine) > contentWidth {
		tabsLine = ansi.Cut(tabsLine, 0, contentWidth)
	}

	selectedWindowName := ""
	selectedWindowCmd := ""
	selectedWindowCwd := env.Root
	usingLiveWindows := false
	if len(windows) > 0 && m.selectedWindow < len(windows) {
		selectedWindowName = windows[m.selectedWindow]
	}
	if sw, ok := m.sessionWindows[session]; ok && len(sw) > 0 {
		usingLiveWindows = true
	}
	if m.selectedWindow < len(env.Windows) {
		selectedWindowCmd = env.Windows[m.selectedWindow].Cmd
		if strings.TrimSpace(env.Windows[m.selectedWindow].Cwd) != "" {
			selectedWindowCwd = env.Windows[m.selectedWindow].Cwd
		}
	}

	infoLineStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Background(lipgloss.Color(theme.SelectedBG)).
		ColorWhitespace(true)

	// Top section: tabs + shaded info lines.
	// tabsLine is 3 visual lines (bordered boxes); each other entry is 1 line.
	topRows := []string{tabsLine, ""}
	if strings.TrimSpace(selectedWindowCwd) != "" {
		topRows = append(topRows, renderStyledPaneLine(infoLineStyle, fmt.Sprintf("Cwd: %s", selectedWindowCwd), contentWidth))
	}
	if strings.TrimSpace(selectedWindowCmd) != "" {
		topRows = append(topRows, renderStyledPaneLine(infoLineStyle, fmt.Sprintf("Cmd: %s", selectedWindowCmd), contentWidth))
	}
	if usingLiveWindows && m.previewSession == session && m.previewWindow == selectedWindowName && strings.TrimSpace(m.previewProcess) != "" {
		topRows = append(topRows, renderStyledPaneLine(infoLineStyle, fmt.Sprintf("Running: %s", m.previewProcess), contentWidth))
	}
	topRows = append(topRows, "") // blank separator before preview

	// tabsLine contributes 1 visual line; every other entry contributes 1.
	topVisualHeight := len(topRows)

	// Preview fills the remaining space below the top section.
	contentHeight := height - 1                          // subtract title line
	previewHeight := contentHeight - topVisualHeight - 1 // -1 for bottom pane margin
	if previewHeight < 0 {
		previewHeight = 0
	}

	previewRows := make([]string, 0, previewHeight)
	hasPreview := usingLiveWindows &&
		m.previewSession == session &&
		m.previewWindow == selectedWindowName &&
		strings.TrimSpace(m.previewContent) != ""

	// Build styles/sequences for the terminal's own background.
	// Fallback chain: explicit BG detected in captured content → real terminal
	// background queried via OSC 11 at startup → theme default.
	previewBGColor := m.previewBG
	if previewBGColor == "" {
		previewBGColor = m.terminalBG
	}
	if previewBGColor == "" {
		previewBGColor = theme.PaneBG
	}
	previewBGStyle := lipgloss.NewStyle().Background(lipgloss.Color(previewBGColor))
	previewBGSeq := colorToANSIBG(previewBGColor)

	if hasPreview && previewHeight > 0 {
		captureLines := strings.Split(strings.TrimRight(m.previewContent, "\n"), "\n")
		start := len(captureLines) - previewHeight
		if start < 0 {
			start = 0
		}
		for _, line := range captureLines[start:] {
			line = strings.TrimRight(line, " \t")
			lineWidth := ansi.StringWidth(line)
			if lineWidth > contentWidth {
				line = ansi.Cut(line, 0, contentWidth)
				lineWidth = contentWidth
			}
			padding := max(0, contentWidth-lineWidth)
			if strings.Contains(line, "\x1b[") {
				// ANSI content: inject bg at start and after every reset so
				// the pane background never bleeds through.
				line = injectBGIntoLine(line, previewBGSeq)
				if padding > 0 {
					line = line + previewBGSeq + strings.Repeat(" ", padding)
				}
			} else {
				// Plain text: render the full line width with terminal bg.
				line = previewBGStyle.Render(padLineToWidth(line, contentWidth))
			}
			previewRows = append(previewRows, line)
		}
	} else if !usingLiveWindows && previewHeight > 0 {
		placeholder := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Inactive)).Render("No active session")
		previewRows = append(previewRows, placeholder)
	}

	// Pad preview to exact height with terminal background.
	for len(previewRows) < previewHeight {
		previewRows = append(previewRows, previewBGStyle.Render(strings.Repeat(" ", contentWidth)))
	}

	allRows := append(topRows, previewRows...)
	return renderPaneWithTitle(width, height, title, strings.Join(allRows, "\n"), focused)
}

func (m Model) renderCreatePane(width, height int) string {
	rows := make([]string, 0, 20)
	contentWidth := modalContentWidth(width)

	inputW := func(prompt string) int {
		w := contentWidth - lipgloss.Width(prompt) - 1 // -1 for cursor space in View()
		if w < 1 {
			w = 1
		}
		return w
	}
	m.createName.Width = inputW(m.createName.Prompt)
	m.createRoot.Width = inputW(m.createRoot.Prompt)
	m.createCustom.Width = inputW(m.createCustom.Prompt)

	templateName := m.selectedCreateTemplateName()
	templateLine := "Template: " + templateName
	if m.createField == createFieldTemplate {
		templateLine = renderStyledPaneLine(selectedLineStyle, templateLine, contentWidth)
	}

	rows = append(rows, m.createName.View())
	rows = append(rows, m.createRoot.View())
	rows = append(rows, templateLine)
	if m.isCustomTemplateSelected() {
		rows = append(rows, m.createCustom.View())
	}
	rows = append(rows, "")
	rows = append(rows, "Enter moves field; Enter on last field creates env + tmux")
	rows = append(rows, "Template field uses left/right to pick a template")
	rows = append(rows, "Window spec format: name=cmd;name2;name3=cmd|cwd")
	rows = append(rows, "Esc cancels")

	borderTitle := "Create Environment"
	return renderModalWithBorderTitle(width, height, borderTitle, strings.Join(rows, "\n"))
}

func (m Model) updateCreateMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.createMode = false
		m.createField = createFieldName
		m.createName.SetValue("")
		m.createRoot.SetValue("")
		m.createTemplate = m.defaultTemplateIndex()
		m.createCustom.SetValue("")
		m.createName.Blur()
		m.createRoot.Blur()
		m.createCustom.Blur()
		m.status = "Create canceled."
		return m, nil
	case "tab":
		// On root field, let textinput handle tab for path autocomplete
		if m.createField == createFieldRoot {
			break // fall through to textinput delegate
		}
		m.shiftCreateField(1)
		m.focusCreateField()
		return m, nil
	case "down":
		// On root field with suggestions, cycle to next suggestion
		if m.createField == createFieldRoot {
			break // fall through to textinput delegate (NextSuggestion)
		}
		m.shiftCreateField(1)
		m.focusCreateField()
		return m, nil
	case "up":
		// On root field with suggestions, cycle to prev suggestion
		if m.createField == createFieldRoot {
			break // fall through to textinput delegate (PrevSuggestion)
		}
		m.shiftCreateField(-1)
		m.focusCreateField()
		return m, nil
	case "shift+tab":
		m.shiftCreateField(-1)
		m.focusCreateField()
		return m, nil
	case "left":
		if m.createField == createFieldTemplate {
			m.moveCreateTemplate(-1)
			return m, nil
		}
	case "right":
		if m.createField == createFieldTemplate {
			m.moveCreateTemplate(1)
			return m, nil
		}
	case "enter":
		if !m.isCreateLastField() {
			m.shiftCreateField(1)
			m.focusCreateField()
			return m, nil
		}
		name := strings.TrimSpace(m.createName.Value())
		root := strings.TrimSpace(m.createRoot.Value())
		if name == "" || root == "" {
			if name == "" {
				m.createField = createFieldName
				m.focusCreateField()
				m.status = "Name is required."
			} else {
				m.createField = createFieldRoot
				m.focusCreateField()
				m.status = "Root path is required."
			}
			return m, nil
		}
		windows, err := m.resolveCreateWindows()
		if err != nil {
			m.status = "Template error: " + err.Error()
			if m.isCustomTemplateSelected() {
				m.createField = createFieldCustomWindows
				m.focusCreateField()
			}
			return m, nil
		}
		m.status = "Creating environment and tmux session..."
		return m, createEnvironmentCmd(name, root, windows)
	}

	// Delegate to the focused textinput
	var cmd tea.Cmd
	switch m.createField {
	case createFieldName:
		m.createName, cmd = m.createName.Update(msg)
	case createFieldRoot:
		m.createRoot, cmd = m.createRoot.Update(msg)
		m.updatePathSuggestions(&m.createRoot)
	case createFieldCustomWindows:
		m.createCustom, cmd = m.createCustom.Update(msg)
	}
	return m, cmd
}

// updatePathSuggestions feeds filesystem path completions to a textinput's
// built-in suggestion system, which renders them as grey ghost text.
func (m *Model) updatePathSuggestions(ti *textinput.Model) {
	val := ti.Value()
	if val == "" {
		ti.SetSuggestions(nil)
		return
	}

	// Expand ~ to home dir for filesystem lookup
	expanded := val
	if strings.HasPrefix(expanded, "~/") || expanded == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			expanded = home + expanded[1:]
		}
	}

	// Determine the directory to list and the prefix to match
	searchDir := expanded
	prefix := ""
	if !strings.HasSuffix(expanded, "/") {
		searchDir = filepath.Dir(expanded)
		prefix = filepath.Base(expanded)
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		ti.SetSuggestions(nil)
		return
	}

	var suggestions []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}
		if prefix != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			continue
		}
		full := filepath.Join(searchDir, name)
		if e.IsDir() {
			full += "/"
		}
		// Convert back to ~/... for display (must match the input prefix)
		if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(val, "~") && strings.HasPrefix(full, home) {
			full = "~" + full[len(home):]
		}
		suggestions = append(suggestions, full)
	}
	sort.Strings(suggestions)
	ti.SetSuggestions(suggestions)
}

func (m Model) resolveCreateWindows() ([]config.WindowTemplate, error) {
	if m.isCustomTemplateSelected() {
		return parseWindowSpec(m.createCustom.Value())
	}
	if m.createTemplate < 0 || m.createTemplate >= len(m.templates) {
		return nil, fmt.Errorf("selected template is invalid")
	}
	return cloneWindowTemplates(m.templates[m.createTemplate].Windows), nil
}

func (m Model) renderTemplatePane(width, height int) string {
	rows := make([]string, 0, 14)
	contentWidth := modalContentWidth(width)

	inputW := func(prompt string) int {
		w := contentWidth - lipgloss.Width(prompt) - 1 // -1 for cursor space in View()
		if w < 1 {
			w = 1
		}
		return w
	}
	m.templateName.Width = inputW(m.templateName.Prompt)
	m.templateSpec.Width = inputW(m.templateSpec.Prompt)

	rows = append(rows, m.templateName.View())
	rows = append(rows, m.templateSpec.View())
	rows = append(rows, "")
	rows = append(rows, "Window spec format: name=cmd;name2;name3=cmd|cwd")
	rows = append(rows, "Enter on last field saves template")
	rows = append(rows, "Esc cancels")

	modeName := "Create Template"
	if m.templateEditing {
		modeName = "Edit Template"
	}
	return renderModalWithBorderTitle(width, height, modeName, strings.Join(rows, "\n"))
}

func (m Model) renderThemePickerPane(width, height int) string {
	indices := m.filteredThemeIndices()
	contentWidth := modalContentWidth(width)

	promptW := lipgloss.Width(m.themeQuery.Prompt)
	inputW := contentWidth - promptW - 1 // -1 for cursor space in View()
	if inputW < 1 {
		inputW = 1
	}
	m.themeQuery.Width = inputW

	rows := []string{
		fmt.Sprintf("Current: %s", m.currentThemeName()),
		m.themeQuery.View(),
		"",
	}

	if len(indices) == 0 {
		rows = append(rows, "No themes match your search.")
	} else {
		for listIdx, themeIdx := range indices {
			name := m.themes[themeIdx].Name
			if themeIdx == m.themeIndex {
				name += " (active)"
			}
			line := "  " + name
			if listIdx == m.themePickerCursor {
				line = renderStyledPaneLine(selectedLineStyle, "> "+name, contentWidth)
			} else {
				line = padLineToWidth(line, contentWidth)
			}
			rows = append(rows, line)
		}
	}

	rows = append(rows, "")
	rows = append(rows, "Type to filter, Enter to apply, Esc to close")

	borderTitle := "[ctrl+t]-Themes"
	return renderModalWithBorderTitle(width, height, borderTitle, strings.Join(rows, "\n"))
}

// --- Fuzzy Search (Command Palette) ---

func (m *Model) openFuzzySearch() tea.Cmd {
	m.showFuzzySearch = true
	m.showThemePicker = false
	m.showShortcuts = false
	m.fuzzySearchQuery.SetValue("")
	m.fuzzySearchCursor = 0
	m.fuzzySearchQuery.Focus()
	m.fuzzySearchResults = m.computeFuzzySearchResults()
	return textinput.Blink
}

func fuzzyMatch(query, target string) bool {
	qi := 0
	for i := 0; i < len(target) && qi < len(query); i++ {
		if target[i] == query[qi] {
			qi++
		}
	}
	return qi == len(query)
}

func (m Model) computeFuzzySearchResults() []fuzzySearchItem {
	query := strings.ToLower(strings.TrimSpace(m.fuzzySearchQuery.Value()))
	var results []fuzzySearchItem

	for envIdx, env := range m.environments {
		session := tmux.SessionName(env.Name)
		_, running := m.sessions[session]
		sessionStatus := m.getSessionAgentStatus(env)

		windows := m.windowNamesForEnv(env)

		// Collect matching windows for this session
		var matchedWindows []fuzzySearchItem
		for winIdx, wName := range windows {
			var status AgentStatus
			var tags []string
			if tmpl, ok := findWindowTemplate(env, wName); ok {
				tags = tmpl.Tags
				if HasTag(tmpl, "ai") {
					key := windowKey(session, wName)
					if info, ok := m.windowProcessInfo[key]; ok {
						status = info.Status
					}
				}
			}

			// Build searchable string: env name, window name, tags (with brackets), status
			tagStr := ""
			for _, t := range tags {
				tagStr += " [" + t + "]"
			}
			searchStr := strings.ToLower(env.Name + " " + wName + tagStr)
			if running {
				searchStr += " running up"
			}
			switch status {
			case AgentStatusCooking:
				searchStr += " cooking"
			case AgentStatusAwaitingInput:
				searchStr += " awaiting input"
			}

			if query == "" || fuzzyMatch(query, searchStr) {
				matchedWindows = append(matchedWindows, fuzzySearchItem{
					EnvIndex:    envIdx,
					WindowIndex: winIdx,
					EnvName:     env.Name,
					WindowName:  wName,
					Status:      status,
					Tags:        tags,
					Running:     running,
				})
			}
		}

		// If any windows matched, add a session header + the windows
		if len(matchedWindows) > 0 {
			results = append(results, fuzzySearchItem{
				EnvIndex:    envIdx,
				WindowIndex: -1,
				EnvName:     env.Name,
				Status:      sessionStatus,
				Running:     running,
				IsHeader:    true,
			})
			results = append(results, matchedWindows...)
		}
	}
	return results
}

func (m *Model) normalizeFuzzySearchCursor() {
	if len(m.fuzzySearchResults) == 0 {
		m.fuzzySearchCursor = 0
		return
	}
	if m.fuzzySearchCursor < 0 {
		m.fuzzySearchCursor = 0
	}
	if m.fuzzySearchCursor >= len(m.fuzzySearchResults) {
		m.fuzzySearchCursor = len(m.fuzzySearchResults) - 1
	}
	// Skip header rows
	if m.fuzzySearchResults[m.fuzzySearchCursor].IsHeader {
		m.fuzzySearchCursor++
		if m.fuzzySearchCursor >= len(m.fuzzySearchResults) {
			// Try going backwards instead
			m.fuzzySearchCursor -= 2
			for m.fuzzySearchCursor >= 0 && m.fuzzySearchResults[m.fuzzySearchCursor].IsHeader {
				m.fuzzySearchCursor--
			}
			if m.fuzzySearchCursor < 0 {
				m.fuzzySearchCursor = 0
			}
		}
	}
}

func (m *Model) moveFuzzySearchCursor(direction int) {
	n := len(m.fuzzySearchResults)
	if n == 0 {
		return
	}
	m.fuzzySearchCursor += direction
	// Clamp
	if m.fuzzySearchCursor < 0 {
		m.fuzzySearchCursor = 0
	}
	if m.fuzzySearchCursor >= n {
		m.fuzzySearchCursor = n - 1
	}
	// Skip headers in the direction of movement
	for m.fuzzySearchCursor >= 0 && m.fuzzySearchCursor < n && m.fuzzySearchResults[m.fuzzySearchCursor].IsHeader {
		m.fuzzySearchCursor += direction
	}
	// Final clamp
	if m.fuzzySearchCursor < 0 {
		m.fuzzySearchCursor = 0
	}
	if m.fuzzySearchCursor >= n {
		m.fuzzySearchCursor = n - 1
	}
}

func (m Model) updateFuzzySearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showFuzzySearch = false
		m.fuzzySearchQuery.Blur()
		m.status = "Search closed."
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "up", "ctrl+k":
		m.moveFuzzySearchCursor(-1)
		return m, nil
	case "down", "ctrl+j":
		m.moveFuzzySearchCursor(1)
		return m, nil
	case "enter":
		if m.fuzzySearchCursor < len(m.fuzzySearchResults) {
			item := m.fuzzySearchResults[m.fuzzySearchCursor]
			if item.IsHeader {
				return m, nil
			}
			m.selectedEnv = item.EnvIndex
			m.selectedWindow = item.WindowIndex
			m.showFuzzySearch = false
			m.fuzzySearchQuery.Blur()
			m.focusPane = focusPaneWindows
			return m.startAttachSelected()
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.fuzzySearchQuery, cmd = m.fuzzySearchQuery.Update(msg)
		m.fuzzySearchResults = m.computeFuzzySearchResults()
		m.fuzzySearchCursor = 0
		m.normalizeFuzzySearchCursor()
		return m, cmd
	}
}

func (m Model) renderFuzzySearchPane(width, height int) string {
	theme := m.currentTheme()
	contentWidth := modalContentWidth(width)

	promptW := lipgloss.Width(m.fuzzySearchQuery.Prompt)
	inputW := contentWidth - promptW - 1
	if inputW < 1 {
		inputW = 1
	}
	m.fuzzySearchQuery.Width = inputW

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent)).
		Background(lipgloss.Color(theme.PaneBG)).
		Bold(true)
	mutedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Background(lipgloss.Color(theme.PaneBG))
	tagStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent)).
		Background(lipgloss.Color(theme.PaneBG))

	rows := []string{
		m.fuzzySearchQuery.View(),
		"",
	}

	// Count selectable items for footer
	selectableCount := 0
	for _, item := range m.fuzzySearchResults {
		if !item.IsHeader {
			selectableCount++
		}
	}

	if selectableCount == 0 {
		rows = append(rows, "  No matches found.")
	} else {
		visibleMax := height - 6
		if visibleMax < 1 {
			visibleMax = 1
		}
		start := 0
		if m.fuzzySearchCursor >= start+visibleMax {
			start = m.fuzzySearchCursor - visibleMax + 1
		}
		// Try to keep headers visible by scrolling up a bit
		if start > 0 && start < len(m.fuzzySearchResults) && !m.fuzzySearchResults[start].IsHeader {
			// Check if previous item is a header — if so, include it
			if start-1 >= 0 && m.fuzzySearchResults[start-1].IsHeader {
				start--
			}
		}
		end := start + visibleMax
		if end > len(m.fuzzySearchResults) {
			end = len(m.fuzzySearchResults)
		}

		for listIdx := start; listIdx < end; listIdx++ {
			item := m.fuzzySearchResults[listIdx]

			if item.IsHeader {
				// Session header row
				runIndicator := "○"
				if item.Running {
					runIndicator = "●"
				}
				statusStr := ""
				switch item.Status {
				case AgentStatusCooking:
					statusStr = "  ● Cooking"
				case AgentStatusAwaitingInput:
					statusStr = "  ◆ Awaiting Input"
				}

				headerText := fmt.Sprintf("  %s %s%s", runIndicator, item.EnvName, statusStr)
				if item.Status != AgentStatusIdle {
					statusColor := m.getWindowStatusColor(item.Status)
					stStyle := lipgloss.NewStyle().
						Foreground(lipgloss.Color(statusColor)).
						Background(lipgloss.Color(theme.PaneBG)).
						Bold(true)
					rows = append(rows, stStyle.Render(fitLineToWidth(headerText, contentWidth)))
				} else {
					rows = append(rows, headerStyle.Render(fitLineToWidth(headerText, contentWidth)))
				}
				continue
			}

			// Window row (indented under session)
			statusStr := ""
			switch item.Status {
			case AgentStatusCooking:
				statusStr = "  ● Cooking"
			case AgentStatusAwaitingInput:
				statusStr = "  ◆ Awaiting Input"
			}

			// Tags rendered inline
			tagStr := ""
			for _, t := range item.Tags {
				tagStr += " " + tagStyle.Render("["+t+"]")
			}

			windowText := item.WindowName + tagStr + statusStr

			if listIdx == m.fuzzySearchCursor {
				// Selected window
				plainText := item.WindowName
				for _, t := range item.Tags {
					plainText += " [" + t + "]"
				}
				plainText += statusStr
				if item.Status != AgentStatusIdle {
					statusColor := m.getWindowStatusColor(item.Status)
					selStyle := lipgloss.NewStyle().
						Foreground(lipgloss.Color(statusColor)).
						Background(lipgloss.Color(theme.SelectedBG)).
						Bold(true)
					rows = append(rows, renderStyledPaneLine(selStyle, "    > "+plainText, contentWidth))
				} else {
					rows = append(rows, renderStyledPaneLine(selectedLineStyle, "    > "+plainText, contentWidth))
				}
			} else if item.Status != AgentStatusIdle {
				statusColor := m.getWindowStatusColor(item.Status)
				stStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color(statusColor)).
					Background(lipgloss.Color(theme.PaneBG))
				// Use plain text for status-colored lines
				plainText := item.WindowName
				for _, t := range item.Tags {
					plainText += " [" + t + "]"
				}
				plainText += statusStr
				rows = append(rows, stStyle.Render(fitLineToWidth("      "+plainText, contentWidth)))
			} else {
				rows = append(rows, mutedStyle.Render("      "+windowText))
			}
		}
	}

	rows = append(rows, "")
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Background(lipgloss.Color(theme.PaneBG))
	footer := fmt.Sprintf("  %d windows | enter attach | esc close", selectableCount)
	rows = append(rows, footerStyle.Render(footer))

	return renderModalWithBorderTitle(width, height, "[/]-Search", strings.Join(rows, "\n"))
}

// shortcutItem represents a single shortcut entry in the shortcuts pane
type shortcutItem struct {
	key      string // key combo to display
	desc     string // description
	isHeader bool   // true = section header, not selectable
	action   string // action identifier for execution (empty = not executable)
}

func (m *Model) shortcutMoveDown() {
	items := shortcutsList()
	for i := m.shortcutCursor + 1; i < len(items); i++ {
		if !items[i].isHeader {
			m.shortcutCursor = i
			return
		}
	}
}

func (m *Model) shortcutMoveUp() {
	items := shortcutsList()
	for i := m.shortcutCursor - 1; i >= 0; i-- {
		if !items[i].isHeader {
			m.shortcutCursor = i
			return
		}
	}
}

func (m Model) updateShortcutsMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "?":
		m.showShortcuts = false
		m.status = "Shortcuts closed."
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		m.shortcutMoveDown()
		return m, nil
	case "k", "up":
		m.shortcutMoveUp()
		return m, nil
	case "enter":
		items := shortcutsList()
		if m.shortcutCursor < len(items) {
			item := items[m.shortcutCursor]
			if item.action != "" {
				m.showShortcuts = false
				return m.executeShortcutAction(item.action)
			}
		}
		return m, nil
	default:
		// Try to match the pressed key to a shortcut and execute it
		pressed := msg.String()
		for _, item := range shortcutsList() {
			if item.isHeader || item.action == "" {
				continue
			}
			// Match against each key in the shortcut (e.g. "n/N" matches "n" or "N")
			for _, k := range strings.Split(item.key, "/") {
				if k == pressed {
					m.showShortcuts = false
					return m.executeShortcutAction(item.action)
				}
			}
		}
		return m, nil
	}
}

func (m Model) executeShortcutAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "focus-sessions":
		m.focusPane = focusPaneEnvironments
		m.status = "Sessions panel focused"
	case "focus-windows":
		m.focusPane = focusPaneWindows
		m.status = "Windows panel focused"
	case "focus-templates":
		m.focusPane = focusPaneTemplates
		m.status = "Templates panel focused"
	case "cycle-panels":
		m.focusPane = (m.focusPane + 1) % 3
	case "search":
		m.showFuzzySearch = true
		cmd := m.openFuzzySearch()
		return m, cmd
	case "next-ai":
		m.jumpToNextCookingSession(1)
		return m, m.captureCurrentWindowCmd()
	case "themes":
		m.showThemePicker = true
		m.themePickerCursor = 0
		m.status = "Theme picker open."
	case "refresh":
		return m, loadSessionsCmd()
	case "quit":
		return m, tea.Quit
	case "create":
		m.createMode = true
		m.createField = createFieldName
		m.createName.Focus()
		m.status = "Create environment."
	case "create-template":
		m.templateMode = true
		m.templateEditing = false
		m.templateField = templateFieldName
		m.templateName.Focus()
		m.status = "Create template."
	}
	return m, nil
}

// shortcutsList returns the flat list of shortcut items (headers + bindings)
func shortcutsList() []shortcutItem {
	return []shortcutItem{
		{desc: "Global", isHeader: true},
		{"1", "focus sessions panel", false, "focus-sessions"},
		{"2", "focus windows panel", false, "focus-windows"},
		{"3", "focus templates panel", false, "focus-templates"},
		{"tab", "cycle panels", false, "cycle-panels"},
		{"ctrl+p", "search", false, "search"},
		{"n/N", "next/prev ai window", false, "next-ai"},
		{"ctrl+t", "theme picker", false, "themes"},
		{"r", "refresh sessions", false, "refresh"},
		{"q", "quit", false, "quit"},

		{desc: "Sessions", isHeader: true},
		{"j/k", "select prev/next", false, ""},
		{"enter", "attach to session", false, ""},
		{"a", "create environment", false, "create"},
		{"d d", "delete environment", false, ""},
		{"x x", "kill session", false, ""},

		{desc: "Windows", isHeader: true},
		{"h/l", "select prev/next", false, ""},
		{"enter", "attach to window", false, ""},
		{"H/L", "reorder window", false, ""},

		{desc: "Templates", isHeader: true},
		{"j/k", "select prev/next", false, ""},
		{"a", "create template", false, "create-template"},
		{"e", "edit template", false, ""},
		{"d d", "delete template", false, ""},

		{desc: "Search", isHeader: true},
		{"ctrl+p", "open search", false, "search"},
		{"↑/↓", "navigate results", false, ""},
		{"enter", "attach to match", false, ""},
		{"esc", "close search", false, ""},

		{desc: "Create / Edit", isHeader: true},
		{"tab", "next field", false, ""},
		{"←/→", "cycle template", false, ""},
		{"enter", "confirm", false, ""},
		{"esc", "cancel", false, ""},

		{desc: "Theme Picker", isHeader: true},
		{"j/k", "navigate themes", false, ""},
		{"enter", "apply theme", false, ""},
		{"esc", "close picker", false, ""},
	}
}

func (m Model) renderShortcutsPane(width, height int) string {
	theme := m.currentTheme()
	contentWidth := modalContentWidth(width)

	headingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent)).
		Bold(true)
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.SelectedFG)).
		Bold(true)
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted))

	items := shortcutsList()

	// Viewport scrolling
	visibleMax := height - 3
	if visibleMax < 1 {
		visibleMax = 1
	}
	start := 0
	if m.shortcutCursor >= start+visibleMax {
		start = m.shortcutCursor - visibleMax + 1
	}
	if start < 0 {
		start = 0
	}
	end := start + visibleMax
	if end > len(items) {
		end = len(items)
	}

	keyColW := 8

	var rows []string
	for i := start; i < end; i++ {
		item := items[i]
		if item.isHeader {
			rows = append(rows, headingStyle.Render("  "+item.desc))
			continue
		}

		pad := keyColW - len(item.key)
		if pad < 1 {
			pad = 1
		}

		k := keyStyle.Render(item.key)
		d := descStyle.Render(item.desc)
		line := "    " + k + strings.Repeat(" ", pad) + d

		if i == m.shortcutCursor {
			// Render as selected
			plainLine := "    " + item.key + strings.Repeat(" ", pad) + item.desc
			line = selectedLineStyle.Render(fitLineToWidth(plainLine, contentWidth))
		}

		rows = append(rows, line)
	}

	rows = append(rows, "")
	rows = append(rows, descStyle.Render("  enter execute  ?/esc close"))

	borderTitle := "[?]-Shortcuts"
	return renderModalWithBorderTitle(width, height, borderTitle, strings.Join(rows, "\n"))
}

func (m Model) updateTemplateMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.templateMode = false
		m.templateEditing = false
		m.templateOrigin = ""
		m.templateField = templateFieldName
		m.templateName.SetValue("")
		m.templateSpec.SetValue("")
		m.templateName.Blur()
		m.templateSpec.Blur()
		m.status = "Template mode canceled."
		return m, nil
	case "tab", "down", "up", "shift+tab":
		if m.templateField == templateFieldName {
			m.templateField = templateFieldWindows
		} else {
			m.templateField = templateFieldName
		}
		m.focusTemplateField()
		return m, nil
	case "enter":
		if m.templateField == templateFieldName {
			m.templateField = templateFieldWindows
			m.focusTemplateField()
			return m, nil
		}
		name := strings.TrimSpace(m.templateName.Value())
		if name == "" {
			m.templateField = templateFieldName
			m.focusTemplateField()
			m.status = "Template name is required."
			return m, nil
		}
		windows, err := parseWindowSpec(m.templateSpec.Value())
		if err != nil {
			m.templateField = templateFieldWindows
			m.focusTemplateField()
			m.status = "Template windows error: " + err.Error()
			return m, nil
		}
		m.status = "Saving template..."
		origin := ""
		if m.templateEditing {
			origin = m.templateOrigin
		}
		return m, saveTemplateCmd(origin, name, windows)
	}

	// Delegate to the focused textinput
	var cmd tea.Cmd
	switch m.templateField {
	case templateFieldName:
		m.templateName, cmd = m.templateName.Update(msg)
	case templateFieldWindows:
		m.templateSpec, cmd = m.templateSpec.Update(msg)
	}
	return m, cmd
}

func loadConfigCmd() tea.Cmd {
	return func() tea.Msg {
		log.Printf("loadConfig: reading config")
		data, err := config.LoadAll()
		if err != nil {
			log.Printf("loadConfig: ERROR %v", err)
			return configLoadedMsg{err: err}
		}
		envs := data.Environments
		templates := data.Templates
		theme := strings.TrimSpace(data.Theme)
		log.Printf("loadConfig: loaded %d environments, %d templates, theme=%q", len(envs), len(templates), theme)
		sort.Slice(envs, func(i, j int) bool {
			return strings.ToLower(envs[i].Name) < strings.ToLower(envs[j].Name)
		})
		sort.SliceStable(templates, func(i, j int) bool {
			leftDefault := strings.EqualFold(templates[i].Name, "default")
			rightDefault := strings.EqualFold(templates[j].Name, "default")
			if leftDefault && !rightDefault {
				return true
			}
			if !leftDefault && rightDefault {
				return false
			}
			return strings.ToLower(templates[i].Name) < strings.ToLower(templates[j].Name)
		})
		return configLoadedMsg{envs: envs, templates: templates, theme: theme}
	}
}

func loadSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		log.Printf("loadSessions: listing tmux sessions")
		names, err := tmux.ListSessions()
		if err != nil {
			log.Printf("loadSessions: ERROR listing sessions: %v", err)
			return sessionsLoadedMsg{err: err}
		}
		log.Printf("loadSessions: found %d sessions: %v", len(names), names)
		windows := map[string][]string{}
		for _, name := range names {
			ws, winErr := tmux.ListWindows(name)
			if winErr != nil {
				log.Printf("loadSessions: ERROR listing windows for %q: %v", name, winErr)
				return sessionsLoadedMsg{err: winErr}
			}
			log.Printf("loadSessions: session %q has windows: %v", name, ws)
			windows[name] = ws
		}
		return sessionsLoadedMsg{names: names, windows: windows}
	}
}

func saveThemePreferenceCmd(theme string) tea.Cmd {
	return func() tea.Msg {
		theme = strings.TrimSpace(theme)
		if theme == "" {
			return themePersistedMsg{err: fmt.Errorf("theme name is required")}
		}
		err := config.SaveTheme(theme)
		return themePersistedMsg{name: theme, err: err}
	}
}

func createEnvironmentCmd(name, root string, windows []config.WindowTemplate) tea.Cmd {
	return func() tea.Msg {
		name = strings.TrimSpace(name)
		root = normalizeRootPath(root)
		log.Printf("createEnvironment: name=%q root=%q windows=%d", name, root, len(windows))
		if name == "" {
			return environmentCreatedMsg{err: fmt.Errorf("name is required")}
		}
		if root == "" {
			return environmentCreatedMsg{err: fmt.Errorf("root path is required")}
		}

		data, err := config.LoadAll()
		if err != nil {
			log.Printf("createEnvironment: ERROR loading config: %v", err)
			return environmentCreatedMsg{err: err}
		}
		for _, env := range data.Environments {
			if strings.EqualFold(strings.TrimSpace(env.Name), name) {
				log.Printf("createEnvironment: environment %q already exists", name)
				return environmentCreatedMsg{err: fmt.Errorf("environment %q already exists", name)}
			}
		}
		if len(windows) == 0 {
			windows = config.DefaultWindows()
			log.Printf("createEnvironment: no windows provided, using defaults (%d windows)", len(windows))
		}
		for i, w := range windows {
			log.Printf("createEnvironment: window[%d] name=%q cmd=%q cwd=%q", i, w.Name, w.Cmd, w.Cwd)
		}

		newEnv := config.Environment{
			Name:    name,
			Root:    root,
			Windows: cloneWindowTemplates(windows),
		}
		data.Environments = append(data.Environments, newEnv)
		if err := config.SaveAll(data); err != nil {
			log.Printf("createEnvironment: ERROR saving config: %v", err)
			return environmentCreatedMsg{err: err}
		}

		sessionErr := tmux.CheckTmuxExists()
		if sessionErr == nil {
			sessionErr = tmux.EnsureSession(newEnv)
		}
		if sessionErr != nil {
			log.Printf("createEnvironment: ERROR ensuring session: %v", sessionErr)
		} else {
			log.Printf("createEnvironment: session ready for %q", name)
		}

		return environmentCreatedMsg{env: newEnv, sessionErr: sessionErr}
	}
}

func saveTemplateCmd(originalName, name string, windows []config.WindowTemplate) tea.Cmd {
	return func() tea.Msg {
		originalName = strings.TrimSpace(originalName)
		name = strings.TrimSpace(name)
		log.Printf("saveTemplate: originalName=%q name=%q windows=%d", originalName, name, len(windows))
		for i, w := range windows {
			log.Printf("saveTemplate: window[%d] name=%q cmd=%q cwd=%q", i, w.Name, w.Cmd, w.Cwd)
		}
		if name == "" {
			return templateSavedMsg{err: fmt.Errorf("template name is required")}
		}
		if len(windows) == 0 {
			return templateSavedMsg{err: fmt.Errorf("template must contain at least one window")}
		}

		data, err := config.LoadAll()
		if err != nil {
			return templateSavedMsg{err: err}
		}

		targetIdx := -1
		if originalName != "" {
			for i := range data.Templates {
				if strings.EqualFold(strings.TrimSpace(data.Templates[i].Name), originalName) {
					targetIdx = i
					break
				}
			}
			if targetIdx < 0 {
				return templateSavedMsg{err: fmt.Errorf("template %q not found", originalName)}
			}
		}

		for i, existing := range data.Templates {
			if strings.EqualFold(strings.TrimSpace(existing.Name), name) {
				if targetIdx >= 0 && i == targetIdx {
					continue
				}
				return templateSavedMsg{err: fmt.Errorf("template %q already exists", name)}
			}
		}

		template := config.Template{Name: name, Windows: cloneWindowTemplates(windows)}
		edited := targetIdx >= 0
		if edited {
			data.Templates[targetIdx] = template
		} else {
			data.Templates = append(data.Templates, template)
		}

		if err := config.SaveAll(data); err != nil {
			return templateSavedMsg{err: err}
		}

		return templateSavedMsg{name: name, edited: edited}
	}
}

func killSessionCmd(session string) tea.Cmd {
	return func() tea.Msg {
		log.Printf("killSession: session=%q", session)
		if err := tmux.CheckTmuxExists(); err != nil {
			log.Printf("killSession: tmux not found: %v", err)
			return sessionKilledMsg{session: session, err: err}
		}
		err := tmux.KillSession(session)
		if err != nil {
			log.Printf("killSession: ERROR killing %q: %v", session, err)
		} else {
			log.Printf("killSession: killed %q", session)
		}
		return sessionKilledMsg{session: session, err: err}
	}
}

func deleteTemplateCmd(name string) tea.Cmd {
	return func() tea.Msg {
		name = strings.TrimSpace(name)
		if name == "" {
			return templateDeletedMsg{err: fmt.Errorf("template name is required")}
		}

		data, err := config.LoadAll()
		if err != nil {
			return templateDeletedMsg{err: err}
		}

		idx := -1
		for i := range data.Templates {
			if strings.EqualFold(strings.TrimSpace(data.Templates[i].Name), name) {
				idx = i
				break
			}
		}
		if idx < 0 {
			return templateDeletedMsg{err: fmt.Errorf("template %q not found", name)}
		}

		deletedName := data.Templates[idx].Name
		data.Templates = append(data.Templates[:idx], data.Templates[idx+1:]...)
		if err := config.SaveAll(data); err != nil {
			return templateDeletedMsg{err: err}
		}

		return templateDeletedMsg{name: deletedName}
	}
}

func deleteEnvironmentCmd(name string) tea.Cmd {
	return func() tea.Msg {
		data, err := config.LoadAll()
		if err != nil {
			return environmentDeletedMsg{err: err}
		}

		idx := -1
		removed := config.Environment{}
		for i := range data.Environments {
			if strings.EqualFold(strings.TrimSpace(data.Environments[i].Name), strings.TrimSpace(name)) {
				idx = i
				removed = data.Environments[i]
				break
			}
		}
		if idx < 0 {
			return environmentDeletedMsg{err: fmt.Errorf("environment %q not found", name)}
		}

		data.Environments = append(data.Environments[:idx], data.Environments[idx+1:]...)
		if err := config.SaveAll(data); err != nil {
			return environmentDeletedMsg{err: err}
		}

		session := tmux.SessionName(removed.Name)
		killed := false
		if tmux.CheckTmuxExists() == nil && tmux.HasSession(session) {
			if err := tmux.KillSession(session); err != nil {
				return environmentDeletedMsg{err: err}
			}
			killed = true
		}

		return environmentDeletedMsg{environment: removed.Name, session: session, killed: killed}
	}
}

func moveWindowOrderCmd(envName, windowName string, direction int) tea.Cmd {
	return func() tea.Msg {
		if direction == 0 {
			return windowMovedMsg{err: fmt.Errorf("direction must be non-zero")}
		}

		envs, err := config.Load()
		if err != nil {
			return windowMovedMsg{err: err}
		}

		envIdx := -1
		for i := range envs {
			if strings.EqualFold(strings.TrimSpace(envs[i].Name), strings.TrimSpace(envName)) {
				envIdx = i
				break
			}
		}
		if envIdx < 0 {
			return windowMovedMsg{err: fmt.Errorf("environment %q not found", envName)}
		}

		env := &envs[envIdx]
		if len(env.Windows) < 2 {
			return windowMovedMsg{err: fmt.Errorf("environment %q has fewer than 2 windows", env.Name)}
		}

		targetWindow := normalizeWindowName(windowName)
		sourceIdx := -1
		for i := range env.Windows {
			if normalizeWindowName(env.Windows[i].Name) == targetWindow {
				sourceIdx = i
				break
			}
		}
		if sourceIdx < 0 {
			return windowMovedMsg{err: fmt.Errorf("window %q not found in template", windowName)}
		}

		destinationIdx := sourceIdx + direction
		if destinationIdx < 0 || destinationIdx >= len(env.Windows) {
			if direction < 0 {
				return windowMovedMsg{err: fmt.Errorf("window is already first")}
			}
			return windowMovedMsg{err: fmt.Errorf("window is already last")}
		}

		sourceWindow := normalizeWindowName(env.Windows[sourceIdx].Name)
		destinationWindow := normalizeWindowName(env.Windows[destinationIdx].Name)
		env.Windows[sourceIdx], env.Windows[destinationIdx] = env.Windows[destinationIdx], env.Windows[sourceIdx]

		if err := config.Save(envs); err != nil {
			return windowMovedMsg{err: err}
		}

		var sessionErr error
		session := tmux.SessionName(env.Name)
		if tmux.HasSession(session) {
			proc := exec.Command("tmux", "swap-window", "-s", session+":"+sourceWindow, "-t", session+":"+destinationWindow)
			if err := proc.Run(); err != nil {
				sessionErr = fmt.Errorf("swap live tmux windows: %w", err)
			}
		}

		return windowMovedMsg{envName: env.Name, direction: direction, sessionErr: sessionErr}
	}
}

func formatWindowSpec(windows []config.WindowTemplate) string {
	if len(windows) == 0 {
		return ""
	}
	parts := make([]string, 0, len(windows))
	for _, w := range windows {
		name := strings.TrimSpace(w.Name)
		if name == "" {
			continue
		}
		cmd := strings.TrimSpace(w.Cmd)
		cwd := strings.TrimSpace(w.Cwd)
		entry := name
		if cmd != "" || cwd != "" {
			entry = name + "=" + cmd
			if cwd != "" {
				entry += "|" + cwd
			}
		}
		parts = append(parts, entry)
	}
	return strings.Join(parts, ";")
}

func parseWindowSpec(spec string) ([]config.WindowTemplate, error) {
	entries := splitWindowEntries(spec)
	if len(entries) == 0 {
		return nil, fmt.Errorf("at least one window is required")
	}
	windows := make([]config.WindowTemplate, 0, len(entries))
	for _, entry := range entries {
		window, err := parseWindowEntry(entry)
		if err != nil {
			return nil, err
		}
		windows = append(windows, window)
	}
	return windows, nil
}

func splitWindowEntries(spec string) []string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return []string{}
	}
	separator := ","
	if strings.Contains(spec, ";") {
		separator = ";"
	}
	raw := strings.Split(spec, separator)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func parseWindowEntry(entry string) (config.WindowTemplate, error) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return config.WindowTemplate{}, fmt.Errorf("empty window entry")
	}

	// Extract tags first (format: [tag1][tag2])
	tags := extractTags(&entry)

	if strings.Contains(entry, "=") {
		parts := strings.SplitN(entry, "=", 2)
		name := strings.TrimSpace(parts[0])
		if name == "" {
			return config.WindowTemplate{}, fmt.Errorf("window name cannot be empty in %q", entry)
		}
		cmdPart := strings.TrimSpace(parts[1])
		cmd := cmdPart
		cwd := ""
		if strings.Contains(cmdPart, "|") {
			cmdCwd := strings.SplitN(cmdPart, "|", 2)
			cmd = strings.TrimSpace(cmdCwd[0])
			cwd = strings.TrimSpace(cmdCwd[1])
		}
		return config.WindowTemplate{Name: name, Cmd: cmd, Cwd: cwd, Tags: tags}, nil
	}

	if strings.Contains(entry, "|") {
		parts := strings.Split(entry, "|")
		if len(parts) > 3 {
			return config.WindowTemplate{}, fmt.Errorf("invalid window entry %q", entry)
		}
		name := strings.TrimSpace(parts[0])
		if name == "" {
			return config.WindowTemplate{}, fmt.Errorf("window name cannot be empty in %q", entry)
		}
		cmd := ""
		cwd := ""
		if len(parts) >= 2 {
			cmd = strings.TrimSpace(parts[1])
		}
		if len(parts) == 3 {
			cwd = strings.TrimSpace(parts[2])
		}
		return config.WindowTemplate{Name: name, Cmd: cmd, Cwd: cwd, Tags: tags}, nil
	}

	name := strings.TrimSpace(entry)
	if name == "" {
		return config.WindowTemplate{}, fmt.Errorf("window name cannot be empty")
	}
	return config.WindowTemplate{Name: name, Tags: tags}, nil
}

// extractTags extracts tags in format [tag1][tag2] from the entry
// Modifies entry to remove the tags and returns the list of tags
func extractTags(entry *string) []string {
	var tags []string
	re := regexp.MustCompile(`\[(\w+)\]`)
	matches := re.FindAllStringSubmatch(*entry, -1)
	for _, match := range matches {
		if len(match) > 1 {
			tags = append(tags, match[1])
		}
	}
	// Remove tags from entry
	*entry = re.ReplaceAllString(*entry, "")
	*entry = strings.TrimSpace(*entry)
	return tags
}

// HasTag checks if a window template has a specific tag
// findWindowTemplate finds a config window template by name within an environment.
func findWindowTemplate(env config.Environment, windowName string) (config.WindowTemplate, bool) {
	for _, w := range env.Windows {
		if w.Name == windowName {
			return w, true
		}
	}
	return config.WindowTemplate{}, false
}

func HasTag(w config.WindowTemplate, tag string) bool {
	for _, t := range w.Tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

func cloneWindowTemplates(windows []config.WindowTemplate) []config.WindowTemplate {
	out := make([]config.WindowTemplate, len(windows))
	copy(out, windows)
	return out
}

func normalizeWindowName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "shell"
	}
	return strings.ReplaceAll(name, " ", "-")
}

func normalizeRootPath(value string) string {
	value = os.ExpandEnv(strings.TrimSpace(value))
	if strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
		}
	}
	if value == "" {
		return value
	}
	return filepath.Clean(value)
}

func prepareAttachCmd(env config.Environment, windowName string) tea.Cmd {
	return func() tea.Msg {
		log.Printf("prepareAttach: env=%q window=%q", env.Name, windowName)
		if err := tmux.CheckTmuxExists(); err != nil {
			log.Printf("prepareAttach: tmux not found: %v", err)
			return attachReadyMsg{err: err}
		}
		if err := tmux.EnsureSession(env); err != nil {
			log.Printf("prepareAttach: ERROR ensuring session for %q: %v", env.Name, err)
			return attachReadyMsg{err: err}
		}
		session := tmux.SessionName(env.Name)
		target := tmux.AttachTarget(env, windowName)
		log.Printf("prepareAttach: session=%q target=%q", session, target)
		if strings.TrimSpace(windowName) != "" {
			hasWindow, err := tmux.HasWindow(session, windowName)
			if err != nil {
				log.Printf("prepareAttach: ERROR checking window %q: %v", windowName, err)
				return attachReadyMsg{err: err}
			}
			log.Printf("prepareAttach: hasWindow(%q)=%v", windowName, hasWindow)
			if hasWindow {
				_ = exec.Command("tmux", "select-window", "-t", target).Run()
			} else {
				log.Printf("prepareAttach: window %q not found, falling back to session root", windowName)
				target = session
			}
		}
		log.Printf("prepareAttach: ready, attaching to %q", target)
		return attachReadyMsg{target: target}
	}
}

func execAttachCmd(target string) tea.Cmd {
	proc := exec.Command("tmux", "attach-session", "-t", target)
	return tea.ExecProcess(proc, func(err error) tea.Msg {
		return attachDoneMsg{err: err}
	})
}

func capturePaneCmd(session, window string) tea.Cmd {
	return func() tea.Msg {
		content, _ := tmux.CapturePane(session, window)
		process := tmux.CurrentProcess(session, window)
		return panePreviewMsg{session: session, window: window, content: content, process: process}
	}
}

var sgrRe = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// colorToANSIBG converts a lipgloss color string to a raw ANSI background
// escape sequence: "#rrggbb" → truecolor, "N" → 256-color palette.
func colorToANSIBG(color string) string {
	if len(color) == 7 && color[0] == '#' {
		r, _ := strconv.ParseInt(color[1:3], 16, 32)
		g, _ := strconv.ParseInt(color[3:5], 16, 32)
		b, _ := strconv.ParseInt(color[5:7], 16, 32)
		return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
	}
	if color != "" {
		return fmt.Sprintf("\x1b[48;5;%sm", color)
	}
	return ""
}

// injectBGIntoLine prepends bgSeq to the line and re-injects it after every
// SGR reset (\e[0m or \e[m) so the terminal bg doesn't bleed through resets.
func injectBGIntoLine(line, bgSeq string) string {
	if bgSeq == "" {
		return line
	}
	line = strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+bgSeq)
	line = strings.ReplaceAll(line, "\x1b[m", "\x1b[m"+bgSeq)
	return bgSeq + line
}

// colorRGB converts a lipgloss color string to R,G,B components (0-255).
func colorRGB(color string) (int, int, int) {
	if len(color) == 7 && color[0] == '#' {
		r, _ := strconv.ParseInt(color[1:3], 16, 32)
		g, _ := strconv.ParseInt(color[3:5], 16, 32)
		b, _ := strconv.ParseInt(color[5:7], 16, 32)
		return int(r), int(g), int(b)
	}
	n, err := strconv.Atoi(color)
	if err != nil || n < 0 || n > 255 {
		return 128, 128, 128
	}
	if n >= 232 { // grayscale ramp
		v := 8 + 10*(n-232)
		return v, v, v
	}
	if n >= 16 { // 6x6x6 color cube
		idx := n - 16
		bi := idx % 6
		idx /= 6
		gi := idx % 6
		idx /= 6
		ri := idx
		toC := func(i int) int {
			if i == 0 {
				return 0
			}
			return 55 + 40*i
		}
		return toC(ri), toC(gi), toC(bi)
	}
	// Standard 16 ANSI colors (approximate)
	ansi16 := [][3]int{
		{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0},
		{0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192},
		{128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
		{0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
	}
	c := ansi16[n]
	return c[0], c[1], c[2]
}

// colorLuminance returns perceptual luminance (0–255) of a lipgloss color string.
func colorLuminance(color string) float64 {
	r, g, b := colorRGB(color)
	return 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
}

// detectPreviewBG scans ANSI escape sequences in captured terminal content and
// returns the darkest explicit background color found, as a lipgloss-compatible
// color string (e.g. "#1e1e2e" or "240"). Terminal base backgrounds are always
// the darkest color; accent/highlight colors are brighter.
// Returns "" if no explicit background colors are found.
func detectPreviewBG(content string) string {
	darkest, darkestLum := "", float64(256)
	for _, match := range sgrRe.FindAllStringSubmatch(content, -1) {
		params := strings.Split(match[1], ";")
		for i := 0; i < len(params); i++ {
			if params[i] != "48" {
				continue
			}
			var color string
			if i+2 < len(params) && params[i+1] == "5" {
				color = params[i+2]
				i += 2
			} else if i+4 < len(params) && params[i+1] == "2" {
				r, _ := strconv.Atoi(params[i+2])
				g, _ := strconv.Atoi(params[i+3])
				b, _ := strconv.Atoi(params[i+4])
				color = fmt.Sprintf("#%02x%02x%02x", r, g, b)
				i += 4
			}
			if color != "" {
				if lum := colorLuminance(color); lum < darkestLum {
					darkestLum = lum
					darkest = color
				}
			}
		}
	}
	return darkest
}

// Agent status detection functions

// detectAgentStatus determines the agent status with hysteresis and adaptive baseline:
// - Uses baseline CPU (average when idle) to detect significant increases
// - Cooking: State == "R" OR CPU > baseline + 15% OR CPU > baseline * 1.5
// - Awaiting: State != "R" AND CPU <= baseline + 10%
// - Baseline is continuously updated when in awaiting_input state
func detectAgentStatus(current ProcessInfo, currentStatus AgentStatus, lowActivityCount int, baselineCPU float64, sampleCount int) (AgentStatus, int, float64, int) {
	// Calculate thresholds using ORIGINAL baseline (before any updates)
	// This prevents high CPU readings from corrupting the baseline before cooking detection
	effectiveBaseline := baselineCPU
	if effectiveBaseline < 1.0 {
		effectiveBaseline = 1.0
	}

	// Cooking threshold: CPU must be significantly above baseline
	// Use +5% absolute or 1.3x multiplier, whichever gives lower threshold
	cookingThreshold := effectiveBaseline + 5.0
	if effectiveBaseline*1.3 > cookingThreshold {
		cookingThreshold = effectiveBaseline * 1.3
	}

	// High activity detection:
	// Primary: CPU exceeds threshold (indicates real work)
	// Agents like opencode use 10-18% CPU when actually working vs 4-5% when idle
	// Threshold of baseline + 5% reliably catches this (9% when baseline is 4%)
	cpuElevated := current.CPU > cookingThreshold
	isHighActivity := cpuElevated
	// To exit cooking state, we need CPU to drop below threshold
	// This creates natural hysteresis without a dead zone
	isLowActivity := current.CPU <= cookingThreshold

	log.Printf("[AGENT-DEBUG] detectAgentStatus: CPU=%.1f, State=%s, baseline=%.1f, threshold=%.1f, currentStatus=%s, sampleCount=%d",
		current.CPU, current.State, baselineCPU, cookingThreshold, currentStatus, sampleCount)

	switch currentStatus {
	case AgentStatusCooking:
		if isHighActivity {
			log.Printf("[AGENT-DEBUG] Still cooking (high activity)")
			return AgentStatusCooking, 0, baselineCPU, sampleCount
		}
		if isLowActivity {
			newCount := lowActivityCount + 1
			log.Printf("[AGENT-DEBUG] Low activity detected, count=%d/3", newCount)
			if newCount >= 3 {
				log.Printf("[AGENT-DEBUG] Switching to awaiting_input")
				return AgentStatusAwaitingInput, 0, baselineCPU, sampleCount
			}
			return AgentStatusCooking, newCount, baselineCPU, sampleCount
		}
		return AgentStatusCooking, lowActivityCount, baselineCPU, sampleCount

	case AgentStatusAwaitingInput, AgentStatusIdle:
		if isHighActivity {
			// Switch to cooking immediately, do NOT update baseline with high CPU reading
			return AgentStatusCooking, 0, baselineCPU, sampleCount
		}
		// Stay awaiting_input and update baseline with this (low) CPU reading
		newBaseline := baselineCPU
		newSampleCount := sampleCount
		if sampleCount == 0 {
			newBaseline = current.CPU
			newSampleCount = 1
		} else if sampleCount < 20 {
			// Build up initial baseline (first 20 samples)
			newBaseline = (baselineCPU*float64(sampleCount) + current.CPU) / float64(sampleCount+1)
			newSampleCount = sampleCount + 1
		} else {
			// Rolling average with more weight on recent samples
			newBaseline = baselineCPU*0.9 + current.CPU*0.1
			newSampleCount = sampleCount
		}
		return AgentStatusAwaitingInput, 0, newBaseline, newSampleCount

	default:
		// For new windows, start with awaiting_input and build baseline
		// Don't immediately assume cooking on first high reading
		// Collect samples first to establish proper baseline
		log.Printf("[AGENT-DEBUG] New window, sampleCount=%d", sampleCount)
		if sampleCount < 10 {
			// First 10 samples: just collect baseline, stay in awaiting
			newBaseline := current.CPU
			newSampleCount := sampleCount + 1
			if sampleCount > 0 {
				newBaseline = (baselineCPU*float64(sampleCount) + current.CPU) / float64(sampleCount+1)
			}
			log.Printf("[AGENT-DEBUG] Initializing sample %d, baseline=%.1f", newSampleCount, newBaseline)
			return AgentStatusAwaitingInput, 0, newBaseline, newSampleCount
		}

		// After 10 samples, use normal logic
		if isHighActivity {
			log.Printf("[AGENT-DEBUG] High activity detected after init, switching to cooking")
			return AgentStatusCooking, 0, baselineCPU, sampleCount
		}
		return AgentStatusAwaitingInput, 0, current.CPU, 1
	}
}

// windowKey creates a unique key for a window
func windowKey(session, window string) string {
	return session + ":" + window
}

// getWindowAgentStatus returns the current agent status for a window
// Only returns non-idle status if window has [ai] tag
func (m Model) getWindowAgentStatus(session, window string) AgentStatus {
	log.Printf("[getWindowAgentStatus] session=%s window=%s", session, window)

	env, ok := m.currentEnv()
	if !ok {
		log.Printf("[getWindowAgentStatus] No current environment, returning idle")
		return AgentStatusIdle
	}

	// Find the window template to check for [ai] tag
	windowIdx := -1
	windows := m.currentWindowNames()
	log.Printf("[getWindowAgentStatus] env=%s windows=%v", env.Name, windows)
	for i, w := range windows {
		if w == window {
			windowIdx = i
			break
		}
	}

	if windowIdx < 0 || windowIdx >= len(env.Windows) {
		log.Printf("[getWindowAgentStatus] Window %s not found in config (idx=%d, len=%d), returning idle", window, windowIdx, len(env.Windows))
		return AgentStatusIdle
	}

	// Check if window has [ai] tag
	winTemplate := env.Windows[windowIdx]
	hasAITag := HasTag(winTemplate, "ai")
	log.Printf("[getWindowAgentStatus] Found window %s, tags=%v, has_ai=%v", window, winTemplate.Tags, hasAITag)
	if !hasAITag {
		return AgentStatusIdle
	}

	// Return tracked status
	key := windowKey(session, window)
	if info, ok := m.windowProcessInfo[key]; ok {
		log.Printf("[getWindowAgentStatus] Returning tracked status: %s for key=%s", info.Status, key)
		return info.Status
	}
	log.Printf("[getWindowAgentStatus] No tracking info for key=%s, returning idle", key)
	return AgentStatusIdle
}

// getWindowStatusColor returns the color for a given agent status
func (m Model) getWindowStatusColor(status AgentStatus) string {
	theme := m.currentTheme()
	switch status {
	case AgentStatusCooking:
		return "#fbbf24" // Amber/yellow for cooking
	case AgentStatusAwaitingInput:
		return "#22d3ee" // Cyan for awaiting input
	default:
		return theme.Muted
	}
}

// jumpToNextCookingSession cycles to the next/previous session that has an active AI agent.
func (m *Model) jumpToNextCookingSession(direction int) {
	if len(m.environments) == 0 {
		return
	}
	start := m.selectedEnv
	for i := 1; i <= len(m.environments); i++ {
		idx := (start + i*direction + len(m.environments)*2) % len(m.environments)
		env := m.environments[idx]
		session := tmux.SessionName(env.Name)
		if _, running := m.sessions[session]; !running {
			continue
		}
		status := m.getSessionAgentStatus(env)
		if status == AgentStatusCooking || status == AgentStatusAwaitingInput {
			m.selectedEnv = idx
			m.selectedWindow = 0
			m.focusPane = focusPaneEnvironments
			statusLabel := "Cooking"
			if status == AgentStatusAwaitingInput {
				statusLabel = "Awaiting Input"
			}
			m.status = fmt.Sprintf("Jumped to %s (%s)", env.Name, statusLabel)
			return
		}
	}
	m.status = "No sessions with active AI agents found."
}

// windowNamesForEnv returns window names for a given environment, preferring live session windows.
func (m Model) windowNamesForEnv(env config.Environment) []string {
	session := tmux.SessionName(env.Name)
	if windows, ok := m.sessionWindows[session]; ok && len(windows) > 0 {
		return windows
	}
	return tmux.WindowNames(env)
}

// getSessionAgentStatus returns the highest-priority agent status across all windows of a session.
// Priority: Cooking > AwaitingInput > Idle
func (m Model) getSessionAgentStatus(env config.Environment) AgentStatus {
	session := tmux.SessionName(env.Name)
	windows := m.windowNamesForEnv(env)
	highest := AgentStatusIdle
	for _, wName := range windows {
		tmpl, ok := findWindowTemplate(env, wName)
		if !ok || !HasTag(tmpl, "ai") {
			continue
		}
		key := windowKey(session, wName)
		if info, ok := m.windowProcessInfo[key]; ok {
			if info.Status == AgentStatusCooking {
				return AgentStatusCooking
			}
			if info.Status == AgentStatusAwaitingInput {
				highest = AgentStatusAwaitingInput
			}
		}
	}
	return highest
}

// updateWindowProcessInfo updates the process info and status for a window
func (m *Model) updateWindowProcessInfo(session, window string) {
	log.Printf("[updateWindowProcessInfo] Starting check for session=%s window=%s", session, window)
	env, ok := m.currentEnv()
	if !ok {
		return
	}

	// Find the window template to check for [ai] tag
	windowIdx := -1
	windows := m.currentWindowNames()
	for i, w := range windows {
		if w == window {
			windowIdx = i
			break
		}
	}

	if windowIdx < 0 || windowIdx >= len(env.Windows) {
		return
	}

	// Only track if window has [ai] tag
	winTemplate := env.Windows[windowIdx]
	if !HasTag(winTemplate, "ai") {
		return
	}

	// Get current process info from tmux
	procInfo, err := tmux.GetPaneProcessInfo(session, window)
	if err != nil {
		return
	}

	log.Printf("[updateWindowProcessInfo] Got procInfo: PID=%d CPU=%.2f State=%s", procInfo.PID, procInfo.CPU, procInfo.State)

	key := windowKey(session, window)
	current := ProcessInfo{
		PID:       procInfo.PID,
		CPU:       procInfo.CPU,
		State:     procInfo.State,
		Timestamp: time.Now(),
	}

	// Get previous info and current tracking state
	var previous ProcessInfo
	var currentStatus AgentStatus
	var lowActivityCount int
	var baselineCPU float64
	var sampleCount int
	if existing, ok := m.windowProcessInfo[key]; ok {
		previous = existing.Current
		currentStatus = existing.Status
		lowActivityCount = existing.LowActivityCount
		baselineCPU = existing.BaselineCPU
		sampleCount = existing.SampleCount
	}

	log.Printf("[updateWindowProcessInfo] Current tracking state: status=%s baselineCPU=%.2f sampleCount=%d", currentStatus, baselineCPU, sampleCount)

	// Detect status with hysteresis and adaptive baseline
	status, newLowActivityCount, newBaselineCPU, newSampleCount := detectAgentStatus(
		current, currentStatus, lowActivityCount, baselineCPU, sampleCount,
	)

	log.Printf("[updateWindowProcessInfo] Detected new status: %s", status)

	// Update tracking
	m.windowProcessInfo[key] = WindowProcessInfo{
		Current:          current,
		Previous:         previous,
		Status:           status,
		LowActivityCount: newLowActivityCount,
		BaselineCPU:      newBaselineCPU,
		SampleCount:      newSampleCount,
	}
}

// updateWindowProcessInfoFromMsg updates the process info from an agentStatusUpdateMsg
// This should be called from the Update method when receiving agentStatusUpdateMsg
func (m *Model) updateWindowProcessInfoFromMsg(session, window string, procInfo ProcessInfo) {
	log.Printf("[updateWindowProcessInfoFromMsg] Processing msg for session=%s window=%s", session, window)

	key := windowKey(session, window)

	// Get previous info and current tracking state
	var previous ProcessInfo
	var currentStatus AgentStatus
	var lowActivityCount int
	var baselineCPU float64
	var sampleCount int
	if existing, ok := m.windowProcessInfo[key]; ok {
		previous = existing.Current
		currentStatus = existing.Status
		lowActivityCount = existing.LowActivityCount
		baselineCPU = existing.BaselineCPU
		sampleCount = existing.SampleCount
	}

	log.Printf("[updateWindowProcessInfoFromMsg] Current tracking: status=%s baseline=%.2f samples=%d",
		currentStatus, baselineCPU, sampleCount)

	// Detect status with hysteresis and adaptive baseline
	status, newLowActivityCount, newBaselineCPU, newSampleCount := detectAgentStatus(
		procInfo, currentStatus, lowActivityCount, baselineCPU, sampleCount,
	)

	log.Printf("[updateWindowProcessInfoFromMsg] New status: %s baseline=%.2f", status, newBaselineCPU)

	// Update tracking
	m.windowProcessInfo[key] = WindowProcessInfo{
		Current:          procInfo,
		Previous:         previous,
		Status:           status,
		LowActivityCount: newLowActivityCount,
		BaselineCPU:      newBaselineCPU,
		SampleCount:      newSampleCount,
	}
}

// formatWindowLabel formats a window name with its status suffix
func (m Model) formatWindowLabel(name string, status AgentStatus) string {
	switch status {
	case AgentStatusCooking:
		return name + " ● Cooking"
	case AgentStatusAwaitingInput:
		return name + " ◆ Awaiting Input"
	default:
		return name
	}
}

func previewTickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return previewTickMsg{}
	})
}

func (m Model) captureCurrentWindowCmd() tea.Cmd {
	var cmds []tea.Cmd

	// Check agent status for all AI windows across ALL running environments
	for _, e := range m.environments {
		s := tmux.SessionName(e.Name)
		if _, live := m.sessions[s]; !live {
			continue
		}
		ws := m.windowNamesForEnv(e)
		for _, w := range ws {
			if tmpl, ok := findWindowTemplate(e, w); ok && HasTag(tmpl, "ai") {
				cmds = append(cmds, checkAgentStatusCmd(s, w))
			}
		}
	}

	// Also capture the pane preview for the currently selected window
	if env, ok := m.currentEnv(); ok {
		session := tmux.SessionName(env.Name)
		if _, live := m.sessions[session]; live {
			windows := m.currentWindowNames()
			if len(windows) > 0 && m.selectedWindow < len(windows) {
				cmds = append(cmds, capturePaneCmd(session, windows[m.selectedWindow]))
			}
		}
	}

	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// checkAgentStatusCmd creates a command to check agent status for a window
func checkAgentStatusCmd(session, window string) tea.Cmd {
	return func() tea.Msg {
		procInfo, err := tmux.GetPaneProcessInfo(session, window)
		if err != nil {
			return nil
		}
		return agentStatusUpdateMsg{
			session: session,
			window:  window,
			procInfo: ProcessInfo{
				PID:       procInfo.PID,
				CPU:       procInfo.CPU,
				State:     procInfo.State,
				Timestamp: time.Now(),
			},
		}
	}
}
