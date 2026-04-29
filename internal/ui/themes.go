package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	pkgtheme "ide/internal/theme"
)

// defaultThemes / normalizeTheme are thin shims around internal/theme so the
// rest of this package keeps using the local names. Theme palette data and
// normalization rules live in internal/theme.
func defaultThemes() []uiTheme         { return pkgtheme.Defaults() }
func normalizeTheme(t uiTheme) uiTheme { return pkgtheme.Normalize(t) }

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
		// Defensive bounds clamp: the cursor is normalized after every
		// keypress in the default branch, but reclamping here protects
		// against any future code path that mutates the filter without
		// also normalizing the cursor.
		if m.themePickerCursor < 0 || m.themePickerCursor >= len(indices) {
			m.themePickerCursor = 0
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
