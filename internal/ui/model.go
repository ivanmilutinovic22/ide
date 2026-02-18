package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"ide/internal/config"
	"ide/internal/tmux"
)

var (
	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("242")).
			BorderBackground(lipgloss.Color("236")).
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			ColorWhitespace(true)
	focusedPaneStyle = lipgloss.NewStyle().
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
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	windowBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("242")).
			BorderBackground(lipgloss.Color("236")).
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("244")).
			Padding(0, 1)
	selectedWindowBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("220")).
			BorderBackground(lipgloss.Color("236")).
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("220")).
			Bold(true).
			Padding(0, 1)
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
	createName            string
	createRoot            string
	createTemplate        int
	createCustom          string
	templateMode          bool
	templateField         int
	templateName          string
	templateSpec          string
	templateEditing       bool
	templateOrigin        string
	showShortcuts         bool
	showThemePicker       bool
	themeQuery            string
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
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Border)).
		BorderBackground(lipgloss.Color(theme.PaneBG)).
		Background(lipgloss.Color(theme.PaneBG)).
		Foreground(lipgloss.Color(theme.AppFG)).
		ColorWhitespace(true)
	focusedPaneStyle = lipgloss.NewStyle().
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
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Status)).Background(lipgloss.Color(theme.AppBG)).ColorWhitespace(true)
	windowBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Muted)).
		BorderBackground(lipgloss.Color(theme.PaneBG)).
		Background(lipgloss.Color(theme.PaneBG)).
		Foreground(lipgloss.Color(theme.Muted)).
		Padding(0, 1)
	selectedWindowBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Accent)).
		BorderBackground(lipgloss.Color(theme.PaneBG)).
		Background(lipgloss.Color(theme.PaneBG)).
		Foreground(lipgloss.Color(theme.Accent)).
		Bold(true).
		Padding(0, 1)
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
	applyThemeStyles(m.currentTheme())
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
	query := strings.ToLower(strings.TrimSpace(m.themeQuery))
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

func (m *Model) openThemePicker() {
	m.showThemePicker = true
	m.showShortcuts = false
	m.themeQuery = ""
	m.themePickerCursor = 0
	indices := m.filteredThemeIndices()
	for i, themeIdx := range indices {
		if themeIdx == m.themeIndex {
			m.themePickerCursor = i
			break
		}
	}
}

func (m Model) updateThemePickerMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.showThemePicker = false
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
	case "backspace", "ctrl+h":
		m.themeQuery = trimLastRune(m.themeQuery)
		m.normalizeThemePickerCursor()
		return m, nil
	case "ctrl+u":
		m.themeQuery = ""
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
		themeName := m.currentThemeName()
		m.status = "Theme: " + themeName
		return m, saveThemePreferenceCmd(themeName)
	}

	if len(msg.Runes) > 0 && !msg.Alt {
		m.themeQuery += string(msg.Runes)
		m.normalizeThemePickerCursor()
	}
	return m, nil
}

func NewModel() Model {
	m := Model{
		sessions:       map[string]struct{}{},
		sessionWindows: map[string][]string{},
		focusPane:      focusPaneEnvironments,
		themes:         defaultThemes(),
		status:         "Loading environments...",
	}
	m.applyCurrentTheme()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadConfigCmd(), loadSessionsCmd())
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
				m.status = "Theme picker closed."
			} else {
				m.openThemePicker()
				m.status = "Theme picker open. Type to filter, Enter to apply."
			}
			return m, nil
		}
		if key == "?" {
			m.showThemePicker = false
			m.showShortcuts = !m.showShortcuts
			if m.showShortcuts {
				m.status = "Shortcuts open. Press ? or Esc to close."
			} else {
				m.status = "Shortcuts closed."
			}
			return m, nil
		}

		if m.showThemePicker {
			return m.updateThemePickerMode(msg)
		}

		if m.showShortcuts {
			switch key {
			case "esc":
				m.showShortcuts = false
				m.status = "Shortcuts closed."
				return m, nil
			case "q", "ctrl+c":
				return m, tea.Quit
			default:
				return m, nil
			}
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
		m.normalizeSelection()
		return m, m.captureCurrentWindowCmd()

	case panePreviewMsg:
		m.previewContent = msg.content
		m.previewSession = msg.session
		m.previewWindow = msg.window
		m.previewProcess = msg.process
		return m, nil

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
		m.createName = ""
		m.createRoot = ""
		m.createField = createFieldName
		m.createTemplate = m.defaultTemplateIndex()
		m.createCustom = ""
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
		m.pendingSelect = msg.envName
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
		m.templateName = ""
		m.templateSpec = ""
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

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	leftWidth, rightWidth := splitPaneWidths(m.width)

	bodyHeight := m.height - 1
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	rightPaneHeight := bodyHeight - 2
	if rightPaneHeight < 1 {
		rightPaneHeight = 1
	}
	leftContentTotal := bodyHeight - 4
	if leftContentTotal < 2 {
		leftContentTotal = 2
	}

	topHeight, bottomHeight := splitLeftPaneHeights(leftContentTotal)
	leftTopPane := m.renderEnvironmentPane(leftWidth, topHeight)
	leftBottomPane := m.renderTemplatesPane(leftWidth, bottomHeight)
	leftPane := lipgloss.JoinVertical(lipgloss.Left, leftTopPane, leftBottomPane)
	rightPane := m.renderDetailsPane(rightWidth, rightPaneHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
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
	status := statusStyle.Render(fitLineToWidth(m.statusLineText(), m.width))
	rendered := lipgloss.JoinVertical(lipgloss.Left, body, status)

	if m.width > 0 && m.height > 0 {
		theme := m.currentTheme()
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
	case "1":
		return focusPaneEnvironments, true
	case "2":
		return focusPaneWindows, true
	case "3":
		return focusPaneTemplates, true
	default:
		return 0, false
	}
}

func focusedPaneStatus(pane int) string {
	switch pane {
	case focusPaneEnvironments:
		return "Focused [1] Sessions panel"
	case focusPaneWindows:
		return "Focused [2] Windows panel"
	case focusPaneTemplates:
		return "Focused [3] Templates panel"
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
		m.createName = ""
		m.createRoot = ""
		m.createTemplate = m.defaultTemplateIndex()
		m.createCustom = ""
		m.status = "Create mode: enter environment name and root path."
		return m, nil
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
		return m.openCreateTemplateMode(), nil
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

func (m Model) openCreateTemplateMode() Model {
	m.templateMode = true
	m.createMode = false
	m.templateField = templateFieldName
	m.templateName = ""
	m.templateSpec = ""
	m.templateEditing = false
	m.templateOrigin = ""
	m.status = "Template mode: name and window spec."
	return m
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
	m.templateName = tpl.Name
	m.templateSpec = formatWindowSpec(tpl.Windows)
	m.templateEditing = true
	m.templateOrigin = tpl.Name
	m.status = "Edit template mode."
	return m, nil
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
	return baseStyle.Width(width).Height(height).Padding(0, 1)
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
	contentWidth := width - paneStyle.GetHorizontalFrameSize()
	if contentWidth < 0 {
		return 0
	}
	return contentWidth
}

// blendColors blends fg toward bg by alpha (1.0 = full fg, 0.0 = full bg).
// Both colors must be 6-digit hex strings with or without leading "#".
func blendColors(fg, bg string, alpha float64) string {
	fg = strings.TrimPrefix(strings.TrimSpace(fg), "#")
	bg = strings.TrimPrefix(strings.TrimSpace(bg), "#")
	if len(fg) != 6 || len(bg) != 6 {
		return "#" + fg
	}
	parse := func(s string) (int64, int64, int64) {
		r, _ := strconv.ParseInt(s[0:2], 16, 64)
		g, _ := strconv.ParseInt(s[2:4], 16, 64)
		b, _ := strconv.ParseInt(s[4:6], 16, 64)
		return r, g, b
	}
	fr, fg2, fb := parse(fg)
	br, bg2, bb := parse(bg)
	blend := func(a, b int64) int { return int(float64(a)*alpha + float64(b)*(1-alpha)) }
	return fmt.Sprintf("#%02x%02x%02x", blend(fr, br), blend(fg2, bg2), blend(fb, bb))
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

func (m Model) contextShortcutHints() string {
	if m.showThemePicker {
		return "Themes: type filter | j/k move | enter apply | esc close"
	}
	if m.showShortcuts {
		return "Shortcuts: ?/esc close | q quit"
	}
	if m.createMode {
		return "Create env: tab/up/down field | left/right template | enter next/create | esc cancel"
	}
	if m.templateMode {
		return "Template mode: tab/up/down field | enter next/save | esc cancel"
	}

	global := "tab or 1/2/3 focus | ctrl+t themes | ? shortcuts | r refresh | q quit"
	switch m.focusPane {
	case focusPaneEnvironments:
		return "[1] Sessions: j/k select | enter attach | a create | d delete | x kill | " + global
	case focusPaneWindows:
		return "[2] Windows: h/l or j/k select | enter attach | H/L reorder | " + global
	case focusPaneTemplates:
		return "[3] Templates: j/k select | a create | e/enter edit | d delete | " + global
	default:
		return global
	}
}

func renderStyledPaneLine(style lipgloss.Style, line string, width int) string {
	return style.Render(padLineToWidth(line, width))
}

func panelBorderTitle(number int, name string, focused bool) string {
	title := fmt.Sprintf("[%d]-%s", number, name)
	if focused {
		title += "*"
	}
	return title
}

func renderPaneWithBorderTitle(width, height int, borderTitle string, body string, focused bool) string {
	body = applyPaneTextBackground(body, paneContentWidth(width))
	paneStyleForPane := paneBoxStyle(width, height, focused)
	pane := paneStyleForPane.Render(body)
	return injectBorderTitle(pane, borderTitle, paneStyleForPane)
}

func applyPaneTextBackground(body string, width int) string {
	if body == "" {
		return body
	}
	lines := strings.Split(body, "\n")
	for i := range lines {
		if strings.Contains(lines[i], "\x1b[") {
			lines[i] = lines[i] + paneTextStyle.Render(strings.Repeat(" ", max(0, width-ansi.StringWidth(lines[i]))))
			continue
		}
		lines[i] = paneTextStyle.Render(padLineToWidth(lines[i], width))
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
	borderTitle := panelBorderTitle(1, "Sessions", focused)
	contentWidth := paneContentWidth(width)

	if len(m.environments) == 0 {
		rows = append(rows, "")
		rows = append(rows, "No environments configured.")
		rows = append(rows, "Press a to create one or edit ~/.config/ide/environments.json")
		return renderPaneWithBorderTitle(width, height, borderTitle, strings.Join(rows, "\n"), focused)
	}

	for idx, env := range m.environments {
		sessionName := tmux.SessionName(env.Name)
		_, running := m.sessions[sessionName]
		state := "down"
		if running {
			state = "up"
		}
		plainLine := fmt.Sprintf("%-23s [%s]", env.Name, state)
		line := "  " + plainLine
		if idx == m.selectedEnv {
			line = renderStyledPaneLine(selectedLineStyle, "> "+plainLine, contentWidth)
		} else {
			if running {
				line = renderStyledPaneLine(activeSessionStyle, line, contentWidth)
			} else {
				line = renderStyledPaneLine(inactiveSessionStyle, line, contentWidth)
			}
		}
		rows = append(rows, line)
	}

	return renderPaneWithBorderTitle(width, height, borderTitle, strings.Join(rows, "\n"), focused)
}

func (m Model) renderTemplatesPane(width, height int) string {
	rows := make([]string, 0, len(m.templates)+2)
	focused := !m.createMode && !m.templateMode && m.focusPane == focusPaneTemplates
	borderTitle := panelBorderTitle(3, "Templates", focused)
	contentWidth := paneContentWidth(width)

	if len(m.templates) == 0 {
		rows = append(rows, "")
		rows = append(rows, "No templates saved.")
		rows = append(rows, "Press a in this panel to add one.")
		return renderPaneWithBorderTitle(width, height, borderTitle, strings.Join(rows, "\n"), focused)
	}

	for idx, tpl := range m.templates {
		line := fmt.Sprintf("%-18s (%d windows)", tpl.Name, len(tpl.Windows))
		if idx == m.selectedTemplate {
			line = renderStyledPaneLine(selectedLineStyle, "> "+line, contentWidth)
		} else {
			line = padLineToWidth("  "+line, contentWidth)
		}
		rows = append(rows, line)
	}

	return renderPaneWithBorderTitle(width, height, borderTitle, strings.Join(rows, "\n"), focused)
}

func (m Model) renderDetailsPane(width, height int) string {
	focused := !m.createMode && !m.templateMode && m.focusPane == focusPaneWindows
	borderTitle := panelBorderTitle(2, "Windows", focused)
	env, ok := m.currentEnv()
	if !ok {
		body := strings.Join([]string{"", "No environment selected."}, "\n")
		return renderPaneWithBorderTitle(width, height, borderTitle, body, focused)
	}

	contentWidth := paneContentWidth(width)

	windows := m.currentWindowNames()
	tabs := make([]string, 0, len(windows))
	for i, w := range windows {
		if i == m.selectedWindow {
			tabs = append(tabs, selectedWindowBoxStyle.Render(w))
		} else {
			tabs = append(tabs, windowBoxStyle.Render(w))
		}
	}
	tabsLine := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	selectedWindowName := ""
	selectedWindowCmd := ""
	selectedWindowCwd := env.Root
	usingLiveWindows := false
	if len(windows) > 0 && m.selectedWindow < len(windows) {
		selectedWindowName = windows[m.selectedWindow]
	}
	session := tmux.SessionName(env.Name)
	if sw, ok := m.sessionWindows[session]; ok && len(sw) > 0 {
		usingLiveWindows = true
	}
	if m.selectedWindow < len(env.Windows) {
		selectedWindowCmd = env.Windows[m.selectedWindow].Cmd
		if strings.TrimSpace(env.Windows[m.selectedWindow].Cwd) != "" {
			selectedWindowCwd = env.Windows[m.selectedWindow].Cwd
		}
	}

	theme := m.currentTheme()
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

	// tabsLine contributes 3 visual lines; every other entry contributes 1.
	tabsVisualHeight := strings.Count(tabsLine, "\n") + 1
	topVisualHeight := tabsVisualHeight + (len(topRows) - 1)

	// Preview fills the remaining space below the top section.
	contentHeight := height - 2 // subtract top + bottom borders
	previewHeight := contentHeight - topVisualHeight
	if previewHeight < 0 {
		previewHeight = 0
	}

	previewRows := make([]string, 0, previewHeight)
	hasPreview := usingLiveWindows &&
		m.previewSession == session &&
		m.previewWindow == selectedWindowName &&
		strings.TrimSpace(m.previewContent) != ""

	if hasPreview && previewHeight > 0 {
		captureLines := strings.Split(strings.TrimRight(m.previewContent, "\n"), "\n")
		start := len(captureLines) - previewHeight
		if start < 0 {
			start = 0
		}
		for _, line := range captureLines[start:] {
			line = strings.TrimRight(line, " \t")
			runes := []rune(line)
			if len(runes) > contentWidth {
				runes = runes[:contentWidth]
			}
			previewRows = append(previewRows, string(runes))
		}
	} else if !usingLiveWindows && previewHeight > 0 {
		placeholder := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Inactive)).Render("No active session")
		previewRows = append(previewRows, placeholder)
	}

	// Pad preview to exact height.
	for len(previewRows) < previewHeight {
		previewRows = append(previewRows, "")
	}

	// Fade the last few preview lines into the background.
	fadeAlphas := []float64{0.55, 0.3, 0.12, 0.04}
	for i, alpha := range fadeAlphas {
		idx := len(previewRows) - len(fadeAlphas) + i
		if idx >= 0 && idx < len(previewRows) {
			col := blendColors(theme.AppFG, theme.PaneBG, alpha)
			previewRows[idx] = lipgloss.NewStyle().Foreground(lipgloss.Color(col)).Render(previewRows[idx])
		}
	}

	allRows := append(topRows, previewRows...)
	return renderPaneWithBorderTitle(width, height, borderTitle, strings.Join(allRows, "\n"), focused)
}

func (m Model) renderCreatePane(width, height int) string {
	rows := make([]string, 0, 20)
	contentWidth := paneContentWidth(width)

	name := m.createName
	if name == "" {
		name = "<environment-name>"
	}
	root := m.createRoot
	if root == "" {
		root = "<path/to/project>"
	}
	templateName := m.selectedCreateTemplateName()
	custom := m.createCustom
	if custom == "" {
		custom = "<editor=nvim .;terminal;lazygit=lazygit>"
	}

	nameLine := "Name: " + name
	rootLine := "Root: " + root
	templateLine := "Template: " + templateName
	customLine := "Custom windows: " + custom
	if m.createField == createFieldName {
		nameLine = renderStyledPaneLine(selectedLineStyle, nameLine, contentWidth)
	}
	if m.createField == createFieldRoot {
		rootLine = renderStyledPaneLine(selectedLineStyle, rootLine, contentWidth)
	}
	if m.createField == createFieldTemplate {
		templateLine = renderStyledPaneLine(selectedLineStyle, templateLine, contentWidth)
	}
	if m.createField == createFieldCustomWindows {
		customLine = renderStyledPaneLine(selectedLineStyle, customLine, contentWidth)
	}

	rows = append(rows, nameLine)
	rows = append(rows, rootLine)
	rows = append(rows, templateLine)
	if m.isCustomTemplateSelected() {
		rows = append(rows, customLine)
	}
	rows = append(rows, "")
	rows = append(rows, "Enter moves field; Enter on last field creates env + tmux")
	rows = append(rows, "Template field uses left/right to pick a template")
	rows = append(rows, "Window spec format: name=cmd;name2;name3=cmd|cwd")
	rows = append(rows, "Esc cancels")

	borderTitle := "Create Environment"
	return renderPaneWithBorderTitle(width, height, borderTitle, strings.Join(rows, "\n"), true)
}

func (m Model) updateCreateMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.createMode = false
		m.createField = createFieldName
		m.createName = ""
		m.createRoot = ""
		m.createTemplate = m.defaultTemplateIndex()
		m.createCustom = ""
		m.status = "Create canceled."
		return m, nil
	case "tab", "down":
		m.shiftCreateField(1)
		return m, nil
	case "shift+tab", "up":
		m.shiftCreateField(-1)
		return m, nil
	case "left":
		if m.createField == createFieldTemplate {
			m.moveCreateTemplate(-1)
		}
		return m, nil
	case "right":
		if m.createField == createFieldTemplate {
			m.moveCreateTemplate(1)
		}
		return m, nil
	case "backspace", "ctrl+h":
		m.backspaceCreateField()
		return m, nil
	case "ctrl+u":
		m.clearCreateField()
		return m, nil
	case "enter":
		if !m.isCreateLastField() {
			m.shiftCreateField(1)
			return m, nil
		}

		name := strings.TrimSpace(m.createName)
		root := strings.TrimSpace(m.createRoot)
		if name == "" || root == "" {
			if name == "" {
				m.createField = createFieldName
				m.status = "Name is required."
			} else {
				m.createField = createFieldRoot
				m.status = "Root path is required."
			}
			return m, nil
		}
		windows, err := m.resolveCreateWindows()
		if err != nil {
			m.status = "Template error: " + err.Error()
			if m.isCustomTemplateSelected() {
				m.createField = createFieldCustomWindows
			}
			return m, nil
		}

		m.status = "Creating environment and tmux session..."
		return m, createEnvironmentCmd(name, root, windows)
	}

	if len(msg.Runes) > 0 && !msg.Alt {
		m.appendCreateField(string(msg.Runes))
	}
	return m, nil
}

func (m *Model) appendCreateField(s string) {
	if m.createField == createFieldName {
		m.createName += s
		return
	}
	if m.createField == createFieldRoot {
		m.createRoot += s
		return
	}
	if m.createField == createFieldCustomWindows {
		m.createCustom += s
	}
}

func (m *Model) backspaceCreateField() {
	if m.createField == createFieldName {
		m.createName = trimLastRune(m.createName)
		return
	}
	if m.createField == createFieldRoot {
		m.createRoot = trimLastRune(m.createRoot)
		return
	}
	if m.createField == createFieldCustomWindows {
		m.createCustom = trimLastRune(m.createCustom)
	}
}

func (m *Model) clearCreateField() {
	if m.createField == createFieldName {
		m.createName = ""
		return
	}
	if m.createField == createFieldRoot {
		m.createRoot = ""
		return
	}
	if m.createField == createFieldCustomWindows {
		m.createCustom = ""
	}
}

func (m Model) resolveCreateWindows() ([]config.WindowTemplate, error) {
	if m.isCustomTemplateSelected() {
		return parseWindowSpec(m.createCustom)
	}
	if m.createTemplate < 0 || m.createTemplate >= len(m.templates) {
		return nil, fmt.Errorf("selected template is invalid")
	}
	return cloneWindowTemplates(m.templates[m.createTemplate].Windows), nil
}

func (m Model) renderTemplatePane(width, height int) string {
	rows := make([]string, 0, 14)
	contentWidth := paneContentWidth(width)

	name := m.templateName
	if name == "" {
		name = "<template-name>"
	}
	spec := m.templateSpec
	if spec == "" {
		spec = "<editor=nvim .;terminal;lazygit=lazygit>"
	}

	nameLine := "Name: " + name
	specLine := "Windows: " + spec
	if m.templateField == templateFieldName {
		nameLine = renderStyledPaneLine(selectedLineStyle, nameLine, contentWidth)
	}
	if m.templateField == templateFieldWindows {
		specLine = renderStyledPaneLine(selectedLineStyle, specLine, contentWidth)
	}

	rows = append(rows, nameLine)
	rows = append(rows, specLine)
	rows = append(rows, "")
	rows = append(rows, "Window spec format: name=cmd;name2;name3=cmd|cwd")
	rows = append(rows, "Enter on last field saves template")
	rows = append(rows, "Esc cancels")

	modeName := "Create Template"
	if m.templateEditing {
		modeName = "Edit Template"
	}
	borderTitle := modeName
	return renderPaneWithBorderTitle(width, height, borderTitle, strings.Join(rows, "\n"), true)
}

func (m Model) renderThemePickerPane(width, height int) string {
	indices := m.filteredThemeIndices()
	query := m.themeQuery
	contentWidth := paneContentWidth(width)
	if query == "" {
		query = "<type to filter themes>"
	}

	rows := []string{
		fmt.Sprintf("Current: %s", m.currentThemeName()),
		"Search: " + query,
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
	return renderPaneWithBorderTitle(width, height, borderTitle, strings.Join(rows, "\n"), true)
}

func (m Model) renderShortcutsPane(width, height int) string {
	rows := []string{
		"Global",
		"1: focus [1] Sessions panel",
		"2: focus [2] Windows panel",
		"3: focus [3] Templates panel",
		"tab: switch panel focus",
		"ctrl+t: open theme picker",
		"r: refresh sessions and config",
		"q or ctrl+c: quit",
		"",
		"[1] Sessions panel",
		"j/k or up/down: select environment",
		"a: create environment",
		"enter: attach to selected target",
		"d: delete selected environment (press twice)",
		"x: kill selected session (press twice)",
		"h/l or left/right: keep focus on [1] Sessions panel",
		"",
		"[2] Windows panel",
		"h/l or left/right: select window",
		"j/k or up/down: select window",
		"enter: attach to selected target",
		"H/L: reorder window template",
		"",
		"[3] Templates panel",
		"j/k or up/down: select template",
		"a: create template",
		"e or enter: edit selected template",
		"d: delete selected template (press twice)",
		"",
		"Create Environment mode",
		"tab or up/down: switch field",
		"left/right on Template: choose template",
		"enter: next field, then create",
		"esc: cancel",
		"",
		"Create Template mode",
		"tab or up/down: switch field",
		"enter: next field, then save",
		"esc: cancel",
		"",
		"Theme picker",
		"type text: filter themes",
		"j/k or up/down: move in result list",
		"enter: apply selected theme",
		"esc: close picker",
		"",
		"Press ? or Esc to close",
	}

	borderTitle := "[?]-Shortcuts"
	return renderPaneWithBorderTitle(width, height, borderTitle, strings.Join(rows, "\n"), true)
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
		m.templateName = ""
		m.templateSpec = ""
		m.status = "Template mode canceled."
		return m, nil
	case "tab", "down", "up", "shift+tab":
		if m.templateField == templateFieldName {
			m.templateField = templateFieldWindows
		} else {
			m.templateField = templateFieldName
		}
		return m, nil
	case "backspace", "ctrl+h":
		m.backspaceTemplateField()
		return m, nil
	case "ctrl+u":
		m.clearTemplateField()
		return m, nil
	case "enter":
		if m.templateField == templateFieldName {
			m.templateField = templateFieldWindows
			return m, nil
		}

		name := strings.TrimSpace(m.templateName)
		if name == "" {
			m.templateField = templateFieldName
			m.status = "Template name is required."
			return m, nil
		}
		windows, err := parseWindowSpec(m.templateSpec)
		if err != nil {
			m.templateField = templateFieldWindows
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

	if len(msg.Runes) > 0 && !msg.Alt {
		m.appendTemplateField(string(msg.Runes))
	}
	return m, nil
}

func (m *Model) appendTemplateField(s string) {
	if m.templateField == templateFieldName {
		m.templateName += s
		return
	}
	m.templateSpec += s
}

func (m *Model) backspaceTemplateField() {
	if m.templateField == templateFieldName {
		m.templateName = trimLastRune(m.templateName)
		return
	}
	m.templateSpec = trimLastRune(m.templateSpec)
}

func (m *Model) clearTemplateField() {
	if m.templateField == templateFieldName {
		m.templateName = ""
		return
	}
	m.templateSpec = ""
}

func trimLastRune(input string) string {
	runes := []rune(input)
	if len(runes) == 0 {
		return input
	}
	return string(runes[:len(runes)-1])
}

func loadConfigCmd() tea.Cmd {
	return func() tea.Msg {
		data, err := config.LoadAll()
		if err != nil {
			return configLoadedMsg{err: err}
		}
		envs := data.Environments
		templates := data.Templates
		theme := strings.TrimSpace(data.Theme)
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
		names, err := tmux.ListSessions()
		if err != nil {
			return sessionsLoadedMsg{err: err}
		}
		windows := map[string][]string{}
		for _, name := range names {
			ws, winErr := tmux.ListWindows(name)
			if winErr != nil {
				return sessionsLoadedMsg{err: winErr}
			}
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
		if name == "" {
			return environmentCreatedMsg{err: fmt.Errorf("name is required")}
		}
		if root == "" {
			return environmentCreatedMsg{err: fmt.Errorf("root path is required")}
		}

		data, err := config.LoadAll()
		if err != nil {
			return environmentCreatedMsg{err: err}
		}
		for _, env := range data.Environments {
			if strings.EqualFold(strings.TrimSpace(env.Name), name) {
				return environmentCreatedMsg{err: fmt.Errorf("environment %q already exists", name)}
			}
		}
		if len(windows) == 0 {
			windows = config.DefaultWindows()
		}

		newEnv := config.Environment{
			Name:    name,
			Root:    root,
			Windows: cloneWindowTemplates(windows),
		}
		data.Environments = append(data.Environments, newEnv)
		if err := config.SaveAll(data); err != nil {
			return environmentCreatedMsg{err: err}
		}

		sessionErr := tmux.CheckTmuxExists()
		if sessionErr == nil {
			sessionErr = tmux.EnsureSession(newEnv)
		}

		return environmentCreatedMsg{env: newEnv, sessionErr: sessionErr}
	}
}

func saveTemplateCmd(originalName, name string, windows []config.WindowTemplate) tea.Cmd {
	return func() tea.Msg {
		originalName = strings.TrimSpace(originalName)
		name = strings.TrimSpace(name)
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
		if err := tmux.CheckTmuxExists(); err != nil {
			return sessionKilledMsg{session: session, err: err}
		}
		err := tmux.KillSession(session)
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
		return config.WindowTemplate{Name: name, Cmd: cmd, Cwd: cwd}, nil
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
		return config.WindowTemplate{Name: name, Cmd: cmd, Cwd: cwd}, nil
	}

	name := strings.TrimSpace(entry)
	if name == "" {
		return config.WindowTemplate{}, fmt.Errorf("window name cannot be empty")
	}
	return config.WindowTemplate{Name: name}, nil
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
		if err := tmux.CheckTmuxExists(); err != nil {
			return attachReadyMsg{err: err}
		}
		if err := tmux.EnsureSession(env); err != nil {
			return attachReadyMsg{err: err}
		}
		session := tmux.SessionName(env.Name)
		target := tmux.AttachTarget(env, windowName)
		if strings.TrimSpace(windowName) != "" {
			hasWindow, err := tmux.HasWindow(session, windowName)
			if err != nil {
				return attachReadyMsg{err: err}
			}
			if hasWindow {
				_ = exec.Command("tmux", "select-window", "-t", target).Run()
			} else {
				target = session
			}
		}
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

func (m Model) captureCurrentWindowCmd() tea.Cmd {
	env, ok := m.currentEnv()
	if !ok {
		return nil
	}
	session := tmux.SessionName(env.Name)
	if _, live := m.sessions[session]; !live {
		return nil
	}
	windows := m.currentWindowNames()
	if len(windows) == 0 || m.selectedWindow >= len(windows) {
		return nil
	}
	return capturePaneCmd(session, windows[m.selectedWindow])
}
