package layout

import (
	"fmt"
	"ide/pkg/config"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const FrameMarginLR = 1
const FrameMarginTB = 0
const panePadLR = 1
const panePadTop = 0
const panePadBottom = 1
const sessionPaneBottomMarg = 1
const windowsPaneLeftMarg = 1

const sidebarWidthPct = 0.25
const sessionHeightPct = 0.75

var paneStyle = lipgloss.NewStyle().
	Padding(panePadTop, panePadLR, panePadBottom).
	Background(lipgloss.Color("236")).
	Foreground(lipgloss.Color("252"))

func usableWidth(termWidth int) int {
	return termWidth - FrameMarginLR*2
}

func usableHeight(termHeight int) int {
	return termHeight - FrameMarginTB*2
}

func sessionPaneWidth(totalWidth int) int {
	return int(float32(totalWidth) * sidebarWidthPct)
}

func sessionPaneHeight(totalHeight int) int {
	return int(float32(totalHeight)*sessionHeightPct) - sessionPaneBottomMarg
}

func templatesPaneWidth(totalWidth int) int {
	return int(float32(totalWidth) * sidebarWidthPct)
}

func templatesPaneHeight(totalHeight int, sessionsHeight int) int {
	return totalHeight - sessionsHeight - sessionPaneBottomMarg
}

func windowsPaneWidth(totalWidth int) int {
	return totalWidth - int(float32(totalWidth)*sidebarWidthPct) - windowsPaneLeftMarg
}

func windowsPaneHeight(totalHeight int) int {
	return totalHeight
}

type sessionListItem struct {
	Name string
}

func (s sessionListItem) FilterValue() string { return s.Name }

type sessionItemDelegate struct {
}

func (d sessionItemDelegate) Height() int                             { return 1 }
func (d sessionItemDelegate) Spacing() int                            { return 0 }
func (d sessionItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d sessionItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, i)

	fn := d.styles.item.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return d.styles.selectedItem.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}

func getSessionsList(sessions []config.Session, sessionPaneWidth, sessionPaneHeight int) list.Model {
	sessionListItems := make([]list.Item, len(sessions))

	for _, sItem := range sessions {
		sessionListItems = append(sessionListItems, sessionListItem{Name: sItem.Name})
	}

	l := list.New(sessionListItems, list.NewDefaultDelegate(), sessionPaneWidth-1, sessionPaneHeight)
	l.SetShowTitle(false)
	return l
}

// Render builds the full framed layout from terminal dimensions.
func Render(termWidth, termHeight int, sessions []config.Session) string {
	w := usableWidth(termWidth)
	h := usableHeight(termHeight)

	sessW := sessionPaneWidth(w)
	sessH := sessionPaneHeight(h)
	tmplW := templatesPaneWidth(w)
	tmplH := templatesPaneHeight(h, sessH)
	winW := windowsPaneWidth(w)
	winH := windowsPaneHeight(h)

	sessionsPane := paneStyle.MarginBottom(sessionPaneBottomMarg).Width(sessW).Height(sessH).
		Render(getSessionsList(sessions, sessW-1, sessH-1).View())
	templatesPane := paneStyle.Width(tmplW).Height(tmplH).
		Render("[T]emplates")
	windowsPane := paneStyle.MarginLeft(windowsPaneLeftMarg).Width(winW).Height(winH).
		Render("[W]indows")

	inner := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.JoinVertical(lipgloss.Top, sessionsPane, templatesPane),
		windowsPane,
	)

	frame := lipgloss.NewStyle().
		MarginTop(FrameMarginTB).MarginLeft(FrameMarginLR).
		Render(inner)

	bg := lipgloss.NewStyle().
		Width(termWidth).
		Height(termHeight).
		Render(frame)

	return bg
}
