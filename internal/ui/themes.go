package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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
