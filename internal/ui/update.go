package ui

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"ide/internal/config"
	"ide/internal/tmux"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.terminalMode && m.embeddedTerm != nil {
			_, rightWidth := splitPaneWidths(m.width - 1)
			contentWidth := paneContentWidth(rightWidth)
			bodyHeight := m.height - 2
			if bodyHeight < 1 {
				bodyHeight = 1
			}
			previewHeight := bodyHeight - 4
			if previewHeight < 1 {
				previewHeight = 1
			}
			m.embeddedTerm.Resize(contentWidth, previewHeight)
		}
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		// Terminal mode: forward all keys to tmux except the exit key.
		// Must be checked before global shortcuts so keys like ? and
		// ctrl+t reach the embedded terminal.
		if m.terminalMode && !m.showFuzzySearch && !m.showThemePicker && !m.showShortcuts {
			return m.updateTerminalMode(key)
		}

		if key == "ctrl+t" {
			if m.showThemePicker {
				m.showThemePicker = false
				m.themeQuery.Blur()
				m.status = "Theme picker closed."
				return m, nil
			}
			m.terminalMode = false
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
			m.terminalMode = false
			cmd := m.openFuzzySearch()
			m.status = "Search open. Type to filter, Enter to attach."
			return m, cmd
		}
		if key == "?" {
			m.showThemePicker = false
			m.showShortcuts = !m.showShortcuts
			if m.showShortcuts {
				m.terminalMode = false
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
			m.terminalMode = false
			if m.embeddedTerm != nil {
				m.embeddedTerm.Close()
				m.embeddedTerm = nil
			}
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
			if m.embeddedTerm != nil {
				m.embeddedTerm.Close()
			}
			return m, tea.Quit
		case "tab":
			m.terminalMode = false
			if m.embeddedTerm != nil {
				m.embeddedTerm.Close()
				m.embeddedTerm = nil
			}
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
		m.rebuildFuzzyIndex()
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
		liveKeys := make(map[string]struct{}, len(msg.windows))
		for session, windows := range msg.windows {
			for _, w := range windows {
				liveKeys[windowKey(session, w)] = struct{}{}
			}
		}
		for k := range m.windowProcessInfo {
			if _, ok := liveKeys[k]; !ok {
				delete(m.windowProcessInfo, k)
			}
		}
		m.rebuildFuzzyIndex()
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
		m.updateWindowProcessInfoFromMsg(msg.session, msg.window, msg.procInfo, msg.command)
		m.rebuildFuzzyIndex()
		// Refresh search results so status changes appear live
		if m.showFuzzySearch {
			m.fuzzySearchResults = m.computeFuzzySearchResults()
		}
		return m, nil

	case ptyReadMsg:
		// New PTY output was processed into the VT emulator; keep reading
		if m.embeddedTerm != nil && !m.embeddedTerm.IsClosed() {
			return m, readPTYCmd(m.embeddedTerm)
		}
		return m, nil

	case ptyEOFMsg:
		// PTY closed (tmux detached or session killed)
		m.terminalMode = false
		if m.embeddedTerm != nil {
			m.embeddedTerm.Close()
			m.embeddedTerm = nil
		}
		m.status = "Terminal closed."
		return m, loadSessionsCmd()

	case terminalSessionReadyMsg:
		if msg.err != nil {
			m.status = "Session start failed: " + msg.err.Error()
			return m, nil
		}
		// Session was just created. Directly enter terminal mode since we know
		// the session exists even though our sessions map hasn't refreshed yet.
		env, ok := m.currentEnv()
		if !ok {
			m.status = "No environment selected."
			return m, nil
		}
		session := tmux.SessionName(env.Name)
		// Add to sessions map immediately so enterTerminalMode finds it
		m.sessions[session] = struct{}{}
		// Also populate sessionWindows
		if wl, err := tmux.ListWindows(session); err == nil {
			m.sessionWindows[session] = wl
		}
		// The fuzzy cache snapshots `Running` per env; without this rebuild
		// it would still report the freshly-started session as not-running
		// until the next 500ms loadSessionsCmd tick.
		m.rebuildFuzzyIndex()
		return m.enterTerminalMode()

	case previewTickMsg:
		return m, tea.Batch(m.captureCurrentWindowCmd(), tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
			return previewTickMsg{}
		}))

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

func clampSelection(current, delta, count int) int {
	if count <= 0 {
		return 0
	}
	next := current + delta
	if next < 0 {
		return 0
	}
	if next >= count {
		return count - 1
	}
	return next
}

func (m *Model) moveEnv(delta int) {
	m.selectedEnv = clampSelection(m.selectedEnv, delta, len(m.environments))
	m.selectedWindow = 0
}

func (m *Model) moveWindow(delta int) {
	m.selectedWindow = clampSelection(m.selectedWindow, delta, len(m.currentWindowNames()))
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
		return m.enterTerminalMode()
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
		// Root paths are directories. Skip plain files so the user isn't
		// offered something they can't actually use as an env root.
		if !isDirEntry(searchDir, e) {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}
		if prefix != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			continue
		}
		full := filepath.Join(searchDir, name) + "/"
		// Convert back to ~/... for display (must match the input prefix)
		if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(val, "~") && strings.HasPrefix(full, home) {
			full = "~" + full[len(home):]
		}
		suggestions = append(suggestions, full)
	}
	sort.Strings(suggestions)
	ti.SetSuggestions(suggestions)
}

// isDirEntry reports whether a directory entry resolves to a directory,
// following symlinks so symlinked dirs (common in monorepos) still suggest.
func isDirEntry(parent string, e os.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	if e.Type()&os.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(filepath.Join(parent, e.Name()))
	if err != nil {
		return false
	}
	return info.IsDir()
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
		// Try to match the pressed key to a shortcut and execute it.
		// Prefer the cursor's row first so duplicate keys (e.g. "a" appears
		// for both create-environment and create-template) resolve to the
		// section the user has navigated to.
		pressed := msg.String()
		items := shortcutsList()
		if m.shortcutCursor >= 0 && m.shortcutCursor < len(items) {
			if cur := items[m.shortcutCursor]; !cur.isHeader && cur.action != "" {
				for _, k := range strings.Split(cur.key, "/") {
					if k == pressed {
						m.showShortcuts = false
						return m.executeShortcutAction(cur.action)
					}
				}
			}
		}
		for _, item := range items {
			if item.isHeader || item.action == "" {
				continue
			}
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
		{"enter", "enter terminal mode", false, ""},
		{"H/L", "reorder window", false, ""},
		{"ctrl+]", "exit terminal mode", false, ""},

		{desc: "Templates", isHeader: true},
		{"j/k", "select prev/next", false, ""},
		{"a", "create template", false, "create-template"},
		{"e/enter", "edit template", false, ""},
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
			cmdCwd := strings.Split(cmdPart, "|")
			if len(cmdCwd) > 2 {
				return config.WindowTemplate{}, fmt.Errorf("invalid window entry %q", entry)
			}
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
