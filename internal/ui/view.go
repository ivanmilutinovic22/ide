package ui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"ide/internal/config"
	"ide/internal/tmux"
)

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	leftWidth, rightWidth := splitPaneWidths(m.width - 1) // -1 for horizontal gap

	bodyHeight := m.height - 1 // -1 status bar (no gap, body sits flush against it)
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

	topHeight, bottomHeight := splitLeftPaneHeights(leftContentTotal, len(m.templates))
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
	rendered := lipgloss.JoinVertical(lipgloss.Left, body, status)

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

func (m Model) hintSeparator() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.currentTheme().Muted)).
		Render(" · ")
}

func (m Model) contextShortcutHints() string {
	sep := m.hintSeparator()

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
	if m.terminalMode {
		return strings.Join([]string{
			m.shortcutHint("ctrl+]", "exit terminal"),
			m.shortcutHint("", "keys → tmux"),
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
			m.shortcutHint("enter", "terminal"),
			m.shortcutHint("H/L", "reorder"),
		)
	case focusPaneTemplates:
		hints = append(hints,
			m.shortcutHint("j/k", "select"),
			m.shortcutHint("a", "create"),
			m.shortcutHint("e", "edit"),
		)
	}
	// Always available globals — kept short so the bar reads at a glance.
	// Less common shortcuts (n next-ai, ctrl+t themes, q quit, r refresh)
	// live in the `?` overlay rather than the always-visible bar.
	hints = append(hints,
		m.shortcutHint("tab", "panels"),
		m.shortcutHint("ctrl+p", "search"),
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

// splitLeftPaneHeights divides the left column between Sessions (top) and
// Templates (bottom). Templates always reserves room for ~5 rows so adding
// a few templates doesn't reflow the layout each time, but it never claims
// more than half the column. Sessions takes the rest.
func splitLeftPaneHeights(total, templateCount int) (int, int) {
	if total <= 2 {
		return 1, 1
	}

	// At least templatesMinVisible rows of body so the user always sees a
	// few entries (or has room to scroll a longer list). Grow with content,
	// shrink to half the column max.
	const templatesMinVisible = 5
	rows := templateCount
	if rows < templatesMinVisible {
		rows = templatesMinVisible
	}
	desired := 1 + rows + 1 // title + rows + slack

	cap := total / 2
	if cap < 3 {
		cap = 3
	}
	if desired > cap {
		desired = cap
	}

	bottom := desired
	top := total - bottom
	if top < 1 {
		top = 1
		bottom = total - 1
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
			indicator = " ◆ Awaiting"
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
				line = renderStyledPaneLine(selStyle, "▸ "+plainLine, contentWidth)
			} else {
				line = renderStyledPaneLine(selectedLineStyle, "▸ "+plainLine, contentWidth)
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
			line = renderStyledPaneLine(selectedLineStyle, "▸ "+line, contentWidth)
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
	env, ok := m.currentEnv()

	// Title changes based on terminal mode
	if m.terminalMode {
		title := panelTitle("w", "Terminal — Ctrl+] exit", true, theme)
		if !ok {
			body := strings.Join([]string{"", "No environment selected."}, "\n")
			return renderPaneWithTitle(width, height, title, body, true)
		}
		return m.renderTerminalPane(width, height, title, env)
	}

	title := panelTitle("w", "Windows", focused, theme)
	if !ok {
		body := strings.Join([]string{"", "No environment selected."}, "\n")
		return renderPaneWithTitle(width, height, title, body, focused)
	}

	contentWidth := paneContentWidth(width)
	windows := m.currentWindowNames()
	session := tmux.SessionName(env.Name)

	tabsLine := m.renderWindowTabs(windows, session, contentWidth, theme)

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

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Background(lipgloss.Color(theme.PaneBG))
	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.AppFG)).
		Background(lipgloss.Color(theme.PaneBG))
	infoLine := func(label, value string) string {
		return renderStyledPaneLine(
			lipgloss.NewStyle().Background(lipgloss.Color(theme.PaneBG)),
			labelStyle.Render(label+" ")+valueStyle.Render(value),
			contentWidth,
		)
	}

	topRows := []string{tabsLine, ""}
	if strings.TrimSpace(selectedWindowCwd) != "" {
		topRows = append(topRows, infoLine("Cwd:", selectedWindowCwd))
	}
	if strings.TrimSpace(selectedWindowCmd) != "" {
		topRows = append(topRows, infoLine("Cmd:", selectedWindowCmd))
	}
	if usingLiveWindows && m.previewSession == session && m.previewWindow == selectedWindowName && strings.TrimSpace(m.previewProcess) != "" {
		topRows = append(topRows, infoLine("Running:", m.previewProcess))
	}
	topRows = append(topRows, "") // blank separator before preview

	topVisualHeight := len(topRows)
	contentHeight := height - 1
	previewHeight := contentHeight - topVisualHeight - 1
	if previewHeight < 0 {
		previewHeight = 0
	}

	previewRows := m.renderPreviewRows(session, selectedWindowName, usingLiveWindows, contentWidth, previewHeight, theme)

	allRows := append(topRows, previewRows...)
	return renderPaneWithTitle(width, height, title, strings.Join(allRows, "\n"), focused)
}

// renderTerminalPane renders the details pane in interactive terminal mode.
// Minimizes chrome to maximize the terminal display area.
func (m Model) renderTerminalPane(width, height int, title string, env config.Environment) string {
	contentWidth := paneContentWidth(width)
	windows := m.currentWindowNames()
	session := tmux.SessionName(env.Name)
	theme := m.currentTheme()

	tabsLine := m.renderWindowTabs(windows, session, contentWidth, theme)

	// In terminal mode: just tabs + blank + terminal output (no info section)
	topRows := []string{tabsLine, ""}
	topVisualHeight := len(topRows)

	contentHeight := height - 1
	previewHeight := contentHeight - topVisualHeight - 1
	if previewHeight < 0 {
		previewHeight = 0
	}

	// Render from the embedded VT emulator
	var terminalRows []string
	if m.embeddedTerm != nil && previewHeight > 0 {
		rendered := m.embeddedTerm.Render(contentWidth, previewHeight)
		if rendered != "" {
			terminalRows = strings.Split(rendered, "\n")
		}
	}

	// Pad to exact height
	previewBGColor := m.terminalBG
	if previewBGColor == "" {
		previewBGColor = theme.PaneBG
	}
	previewBGStyle := lipgloss.NewStyle().Background(lipgloss.Color(previewBGColor))
	for len(terminalRows) < previewHeight {
		terminalRows = append(terminalRows, previewBGStyle.Render(strings.Repeat(" ", contentWidth)))
	}

	allRows := append(topRows, terminalRows...)
	return renderPaneWithTitle(width, height, title, strings.Join(allRows, "\n"), true)
}

// renderWindowTabs builds the tab bar string for window tabs.
func (m Model) renderWindowTabs(windows []string, session string, contentWidth int, theme uiTheme) string {
	tabs := make([]string, 0, len(windows))
	for i, w := range windows {
		status := m.getWindowAgentStatus(session, w)
		label := m.formatWindowLabel(w, status)
		if i < 9 {
			label = fmt.Sprintf("[%d] %s", i+1, label)
		}

		if i == m.selectedWindow {
			if status != AgentStatusIdle {
				statusColor := m.getWindowStatusColor(status)
				sStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color(statusColor)).
					Background(lipgloss.Color(theme.PaneBG)).
					Bold(true)
				tabs = append(tabs, sStyle.Render(label))
			} else {
				tabs = append(tabs, selectedWindowBoxStyle.Render(label))
			}
		} else {
			if status != AgentStatusIdle {
				statusColor := m.getWindowStatusColor(status)
				sStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color(statusColor)).
					Background(lipgloss.Color(theme.PaneBG))
				tabs = append(tabs, sStyle.Render(label))
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
	return tabsLine
}

// renderPreviewRows renders the tmux pane capture content as display rows.
func (m Model) renderPreviewRows(session, windowName string, usingLiveWindows bool, contentWidth, previewHeight int, theme uiTheme) []string {
	previewRows := make([]string, 0, previewHeight)
	hasPreview := usingLiveWindows &&
		m.previewSession == session &&
		m.previewWindow == windowName &&
		strings.TrimSpace(m.previewContent) != ""

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
				line = injectBGIntoLine(line, previewBGSeq)
				if padding > 0 {
					line = line + previewBGSeq + strings.Repeat(" ", padding)
				}
			} else {
				line = previewBGStyle.Render(padLineToWidth(line, contentWidth))
			}
			previewRows = append(previewRows, line)
		}
	} else if !usingLiveWindows && previewHeight > 0 {
		placeholder := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Inactive)).Render("No active session — Enter to start")
		previewRows = append(previewRows, placeholder)
	}

	for len(previewRows) < previewHeight {
		previewRows = append(previewRows, previewBGStyle.Render(strings.Repeat(" ", contentWidth)))
	}
	return previewRows
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
				line = renderStyledPaneLine(selectedLineStyle, "▸ "+name, contentWidth)
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
