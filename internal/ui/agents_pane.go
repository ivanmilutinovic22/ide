package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"ide/internal/tmux"
)

// agentItem is one row in the Agents pane — a single AI window from a
// running session. envIdx and windowName are how we route an "attach"
// action back to the right session/window.
type agentItem struct {
	envIdx     int
	envName    string
	windowName string
	status     AgentStatus
}

// agentItems collects every AI window from every running session into a
// flat list, in declaration order (envs ordered by config, windows ordered
// by tmux). Idle agents are included so the pane lists every agent the
// user could attach to, not just the ones currently active.
func (m Model) agentItems() []agentItem {
	var items []agentItem
	for envIdx, env := range m.environments {
		session := tmux.SessionName(env.Name)
		if _, running := m.sessions[session]; !running {
			continue
		}
		for _, wName := range m.windowNamesForEnv(env) {
			key := windowKey(session, wName)
			info, hasInfo := m.windowProcessInfo[key]
			cmd := ""
			if hasInfo {
				cmd = info.Command
			}
			if !m.isAIWindow(env, wName, cmd) {
				continue
			}
			status := AgentStatusIdle
			if hasInfo {
				status = info.Status
			}
			items = append(items, agentItem{
				envIdx:     envIdx,
				envName:    env.Name,
				windowName: wName,
				status:     status,
			})
		}
	}
	return items
}

func (m Model) renderAgentsPane(width, height int) string {
	focused := m.paneFocused(focusPaneAgents)
	theme := m.currentTheme()
	title := panelTitle("a", "Agents", focused, theme)
	contentWidth := paneContentWidth(width)

	items := m.agentItems()
	rows := make([]string, 0, len(items))
	for idx, it := range items {
		indicator := ""
		switch it.status {
		case AgentStatusCooking:
			indicator = " ●"
		case AgentStatusAwaitingInput:
			indicator = " ◆"
		}
		content := fmt.Sprintf("%s %s / %s%s", numPrefix(idx), it.envName, it.windowName, indicator)

		selected := idx == m.selectedAgent
		selectedStyle := selectedLineStyle
		var defaultStyle *lipgloss.Style
		if it.status != AgentStatusIdle {
			statusColor := m.getWindowStatusColor(it.status)
			selectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(statusColor)).
				Background(lipgloss.Color(theme.SelectionBG)).
				Bold(true)
			ds := lipgloss.NewStyle().
				Foreground(lipgloss.Color(statusColor)).
				Background(lipgloss.Color(theme.PaneBG)).
				Bold(true)
			defaultStyle = &ds
		} else {
			ds := activeSessionStyle
			defaultStyle = &ds
		}
		rows = append(rows, renderListRow(content, selected, contentWidth, theme, selectedStyle, defaultStyle))
	}

	empty := []string{"", "No AI agents detected.", "Start a session with an [ai]-tagged window or run a known AI CLI."}
	return m.renderListPane(width, height, title, focused, rows, m.selectedAgent, empty)
}
