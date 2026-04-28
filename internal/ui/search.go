package ui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"ide/internal/config"
	"ide/internal/tmux"
)

// searchItem represents a row in the search popup.
type searchItem struct {
	envIdx  int
	winIdx  int // -1 for session headers
	env     string
	window  string
	tags    []string
	running bool
	header  bool
	status  AgentStatus
}

// SearchModel is a standalone Bubble Tea model for the tmux popup search.
type SearchModel struct {
	width          int
	height         int
	query          textinput.Model
	cursor         int
	results        []searchItem
	envs           []config.Environment
	sessions       map[string]struct{}
	sessionWindows map[string][]string
	statuses       map[string]AgentStatus
	theme          uiTheme
}

type searchSessionsLoadedMsg struct {
	names   []string
	windows map[string][]string
}

type searchConfigLoadedMsg struct {
	envs  []config.Environment
	theme string
}

type searchStatusLoadedMsg struct {
	statuses map[string]AgentStatus
}

func NewSearchModel() SearchModel {
	ti := textinput.New()
	ti.Prompt = "/ "
	ti.Placeholder = "Search sessions and windows..."
	ti.Focus()

	themes := defaultThemes()
	m := SearchModel{
		query:          ti,
		sessions:       map[string]struct{}{},
		sessionWindows: map[string][]string{},
		statuses:       map[string]AgentStatus{},
		theme:          themes[0],
	}
	return m
}

func (m SearchModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.loadConfig(), m.loadSessions())
}

func (m SearchModel) loadConfig() tea.Cmd {
	return func() tea.Msg {
		data, err := config.LoadAll()
		if err != nil {
			return searchConfigLoadedMsg{}
		}
		return searchConfigLoadedMsg{envs: data.Environments, theme: data.Theme}
	}
}

func (m SearchModel) loadSessions() tea.Cmd {
	return func() tea.Msg {
		names, _ := tmux.ListSessions()
		windows := map[string][]string{}
		for _, s := range names {
			w, err := tmux.ListWindows(s)
			if err == nil {
				windows[s] = w
			}
		}
		return searchSessionsLoadedMsg{names: names, windows: windows}
	}
}

// loadStatuses fetches a one-shot agent status for every [ai]-tagged window
// in a running session. The popup is short-lived, so we don't run hysteresis;
// we derive a single sample heuristic per window:
//   - state == "R"      → Cooking
//   - CPU > 5.0         → Cooking
//   - otherwise         → AwaitingInput (the agent CLI is alive in an [ai] pane)
//
// If GetPaneProcessInfo errors (no live session, etc.) the window is omitted
// from the result map and renders as Idle (no suffix).
func (m SearchModel) loadStatuses() tea.Cmd {
	envs := m.envs
	sessions := m.sessions
	sessionWindows := m.sessionWindows
	return func() tea.Msg {
		out := map[string]AgentStatus{}
		for _, env := range envs {
			session := tmux.SessionName(env.Name)
			if _, running := sessions[session]; !running {
				continue
			}
			windows := tmux.WindowNames(env)
			if sw, ok := sessionWindows[session]; ok && len(sw) > 0 {
				windows = sw
			}
			for _, wName := range windows {
				tmpl, hasTmpl := findWindowTemplate(env, wName)
				hasAI := hasTmpl && HasTag(tmpl, "ai")
				if !hasAI {
					if !isAIToolProcess(tmux.CurrentProcess(session, wName)) {
						continue
					}
				}
				info, err := tmux.GetPaneProcessInfo(session, wName)
				if err != nil {
					continue
				}
				status := AgentStatusAwaitingInput
				if info.State == "R" || info.CPU > 5.0 {
					status = AgentStatusCooking
				}
				out[windowKey(session, wName)] = status
			}
		}
		return searchStatusLoadedMsg{statuses: out}
	}
}

func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case searchConfigLoadedMsg:
		m.envs = msg.envs
		// Apply theme
		for _, t := range defaultThemes() {
			if strings.EqualFold(t.Name, msg.theme) {
				m.theme = t
				break
			}
		}
		m.results = m.computeResults()
		m.normalizeCursor()
		return m, nil

	case searchSessionsLoadedMsg:
		m.sessions = map[string]struct{}{}
		m.sessionWindows = map[string][]string{}
		for _, name := range msg.names {
			m.sessions[name] = struct{}{}
		}
		for s, w := range msg.windows {
			m.sessionWindows[s] = w
		}
		m.results = m.computeResults()
		m.normalizeCursor()
		return m, m.loadStatuses()

	case searchStatusLoadedMsg:
		m.statuses = msg.statuses
		m.results = m.computeResults()
		m.normalizeCursor()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			return m, tea.Quit
		case "up", "ctrl+k":
			m.moveCursor(-1)
			return m, nil
		case "down", "ctrl+j":
			m.moveCursor(1)
			return m, nil
		case "enter":
			if m.cursor >= 0 && m.cursor < len(m.results) {
				item := m.results[m.cursor]
				if !item.header {
					m.switchTo(item)
					return m, tea.Quit
				}
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.query, cmd = m.query.Update(msg)
			m.results = m.computeResults()
			m.cursor = 0
			m.normalizeCursor()
			return m, cmd
		}
	}

	// Forward other messages to textinput (blink etc.)
	var cmd tea.Cmd
	m.query, cmd = m.query.Update(msg)
	return m, cmd
}

func (m SearchModel) switchTo(item searchItem) {
	session := tmux.SessionName(item.env)
	target := session + ":" + item.window

	// Check if we're already in the target session
	current := currentTmuxSession()
	if current == session {
		// Same session — just select the window
		_ = exec.Command("tmux", "select-window", "-t", target).Run()
	} else {
		// Different session — switch client and select window
		_ = exec.Command("tmux", "switch-client", "-t", target).Run()
	}
}

func currentTmuxSession() string {
	out, err := exec.Command("tmux", "display-message", "-p", "#{session_name}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (m SearchModel) computeResults() []searchItem {
	query := strings.ToLower(strings.TrimSpace(m.query.Value()))
	var results []searchItem

	for envIdx, env := range m.envs {
		session := tmux.SessionName(env.Name)
		_, running := m.sessions[session]

		windows := tmux.WindowNames(env)
		if sw, ok := m.sessionWindows[session]; ok && len(sw) > 0 {
			windows = sw
		}

		var matched []searchItem
		for winIdx, wName := range windows {
			var tags []string
			if tmpl, ok := findWindowTemplate(env, wName); ok {
				tags = tmpl.Tags
			}

			tagStr := ""
			for _, t := range tags {
				tagStr += " [" + t + "]"
			}
			searchStr := strings.ToLower(env.Name + " " + wName + tagStr)
			if running {
				searchStr += " running up"
			}

			if query == "" || fuzzyMatch(query, searchStr) {
				status := AgentStatusIdle
				if running && m.statuses != nil {
					if s, ok := m.statuses[windowKey(session, wName)]; ok {
						status = s
					}
				}
				matched = append(matched, searchItem{
					envIdx:  envIdx,
					winIdx:  winIdx,
					env:     env.Name,
					window:  wName,
					tags:    tags,
					running: running,
					status:  status,
				})
			}
		}

		if len(matched) > 0 {
			results = append(results, searchItem{
				envIdx:  envIdx,
				winIdx:  -1,
				env:     env.Name,
				running: running,
				header:  true,
			})
			results = append(results, matched...)
		}
	}
	return results
}

func (m *SearchModel) normalizeCursor() {
	if len(m.results) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.results) {
		m.cursor = len(m.results) - 1
	}
	// Skip headers
	if m.results[m.cursor].header {
		m.cursor++
		if m.cursor >= len(m.results) {
			m.cursor--
			for m.cursor >= 0 && m.results[m.cursor].header {
				m.cursor--
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
		}
	}
}

func (m *SearchModel) moveCursor(dir int) {
	n := len(m.results)
	if n == 0 {
		return
	}
	m.cursor += dir
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
	for m.cursor >= 0 && m.cursor < n && m.results[m.cursor].header {
		m.cursor += dir
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
}

func (m SearchModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	theme := m.theme
	contentWidth := m.width - 4
	if contentWidth < 10 {
		contentWidth = m.width
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent)).
		Bold(true)
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.SelectedFG)).
		Background(lipgloss.Color(theme.SelectedBG)).
		Bold(true)
	mutedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted))
	tagStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent))
	activeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Active)).
		Bold(true)

	m.query.Width = contentWidth - 4
	rows := []string{
		"  " + m.query.View(),
		"",
	}

	selectableCount := 0
	for _, item := range m.results {
		if !item.header {
			selectableCount++
		}
	}

	if selectableCount == 0 {
		rows = append(rows, mutedStyle.Render("  No matches found."))
	} else {
		visibleMax := m.height - 5
		if visibleMax < 1 {
			visibleMax = 1
		}
		start := 0
		if m.cursor >= start+visibleMax {
			start = m.cursor - visibleMax + 1
		}
		if start > 0 && start < len(m.results) && !m.results[start].header {
			if start-1 >= 0 && m.results[start-1].header {
				start--
			}
		}
		end := start + visibleMax
		if end > len(m.results) {
			end = len(m.results)
		}

		for i := start; i < end; i++ {
			item := m.results[i]

			if item.header {
				indicator := "○"
				if item.running {
					indicator = "●"
				}
				text := fmt.Sprintf("  %s %s", indicator, item.env)
				if item.running {
					rows = append(rows, activeStyle.Render(fitToWidth(text, contentWidth)))
				} else {
					rows = append(rows, headerStyle.Render(fitToWidth(text, contentWidth)))
				}
				continue
			}

			// Status indicator (rendered between window name and tags).
			statusPlain := ""
			statusStyled := ""
			switch item.status {
			case AgentStatusCooking:
				statusPlain = " ●"
				statusStyled = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#fbbf24")).
					Bold(true).
					Render(statusPlain)
			case AgentStatusAwaitingInput:
				statusPlain = " ◆"
				statusStyled = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#22d3ee")).
					Bold(true).
					Render(statusPlain)
			}

			// Tags
			tagStr := ""
			for _, t := range item.tags {
				tagStr += " " + tagStyle.Render("["+t+"]")
			}

			name := item.window + statusStyled + tagStr

			if i == m.cursor {
				plain := item.window + statusPlain
				for _, t := range item.tags {
					plain += " [" + t + "]"
				}
				line := fitToWidth("    > "+plain, contentWidth)
				rows = append(rows, selectedStyle.Render(line))
			} else {
				rows = append(rows, mutedStyle.Render("      "+name))
			}
		}
	}

	rows = append(rows, "")
	rows = append(rows, mutedStyle.Render(fmt.Sprintf("  %d windows | enter switch | esc close", selectableCount)))

	body := strings.Join(rows, "\n")

	// Trim to fit height
	lines := strings.Split(body, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}

	return strings.Join(lines, "\n")
}

func fitToWidth(s string, width int) string {
	w := ansi.StringWidth(s)
	if w > width {
		return ansi.Cut(s, 0, width)
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}
