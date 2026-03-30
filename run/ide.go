package run

import (
	"fmt"
	"ide/pkg/config"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type configLoadedMsg struct {
	cfg config.Config
	err error
}

type model struct {
	width  int
	height int
	cfg    config.Config
	err    error
}

func loadConfigCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		return configLoadedMsg{cfg: cfg, err: err}
	}
}

func (m model) Init() tea.Cmd {
	return loadConfigCmd()
}

const frameMarginLR = 1
const frameMarginTB = 1
const panePadLR = 1
const panePadTB = 1
const sessionPaneBottomMarg = 1
const windowsPaneLeftMarg = 1

const sidebarWidthPct = 0.25

const windowWidthPct = 1 - sidebarWidthPct
const sessionHeightPct = 0.75
const templatesHeightPct = 1 - sessionHeightPct

func sessionPaneWidth(totalWidth int) int {
	return int(float32(totalWidth) * sidebarWidthPct)
}

func sessionPaneHeight(totalHeight int) int {
	return int(float32(totalHeight)*sessionHeightPct) - sessionPaneBottomMarg
}

func templatesPaneWidth(totalWidth int) int {
	return int(float32(totalWidth) * sidebarWidthPct)
}

// remainder after sessions so float truncation doesn't leave a gap
func templatesPaneHeight(totalHeight int, sessHeight int) int {
	return totalHeight - sessHeight - sessionPaneBottomMarg
}

func windowsPaneWidth(totalWidth int) int {
	return totalWidth - int(float32(totalWidth)*sidebarWidthPct) - windowsPaneLeftMarg
}

func windowsPaneHeight(totalHeight int) int {
	return totalHeight
}

func (m model) workableWidth() int {
	return m.width - frameMarginLR*2
}

func (m model) workableHeight() int {
	return m.height - frameMarginTB*2
}

var paneStyle = lipgloss.NewStyle().
	Padding(panePadTB, panePadLR).
	Background(lipgloss.Color("236")).
	Foreground(lipgloss.Color("252"))

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case configLoadedMsg:
		m.cfg = msg.cfg
		m.err = msg.err
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() tea.View {
	w := m.workableWidth()
	h := m.workableHeight()

	sessW := sessionPaneWidth(w)
	sessH := sessionPaneHeight(h)
	tmplW := templatesPaneWidth(w)
	tmplH := templatesPaneHeight(h, sessH)
	winW := windowsPaneWidth(w)
	winH := windowsPaneHeight(h)

	sessionsPane := paneStyle.MarginBottom(1).Width(sessW).Height(sessH).Render(fmt.Sprintf("w=%d h=%d / %dx%d", w, h, sessW, sessH))
	templatesPane := paneStyle.Width(tmplW).Height(tmplH).Render(fmt.Sprintf("w=%d h=%d / %dx%d", w, h, tmplW, tmplH))
	windowsPane := paneStyle.MarginLeft(windowsPaneLeftMarg).Width(winW).Height(winH).Render(fmt.Sprintf("w=%d h=%d / %dx%d", w, h, winW, winH))

	layout := lipgloss.JoinHorizontal(lipgloss.Left, lipgloss.JoinVertical(lipgloss.Top, sessionsPane, templatesPane), windowsPane)

	frame := lipgloss.NewStyle().
		MarginTop(frameMarginTB).MarginLeft(frameMarginLR).
		Render(layout)

	bg := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		// Background(lipgloss.Color("#2d2d2d")).
		Render(frame)

	return tea.NewView(bg)
}

func Ide() model {
	return model{}
}
