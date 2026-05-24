package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// paneFocused reports whether the given pane is the active focus target.
// Any open modal (create/template/envEdit/extract) suppresses pane focus so
// pane titles render in their muted state.
func (m Model) paneFocused(pane int) bool {
	if m.createMode || m.templateMode || m.envEditMode || m.extractMode {
		return false
	}
	return m.focusPane == pane
}

// numPrefix returns "[N]" for the first 9 list entries (1-indexed) so the
// number can be typed as a shortcut, or three spaces otherwise to keep
// rows column-aligned.
func numPrefix(idx int) string {
	if idx < 9 {
		return fmt.Sprintf("[%d]", idx+1)
	}
	return "   "
}

// renderListRow paints a list row with the unified selection treatment:
// the selected row gets a 1-column accent ▌ bar on the left and a soft
// wash background across its full width. Unselected rows are flat —
// no background tint, no marker — keeping the focus on the selected row.
// `defaultStyle == nil` means "no styling, just pad to width".
func renderListRow(content string, selected bool, contentWidth int, theme uiTheme, selectedStyle lipgloss.Style, defaultStyle *lipgloss.Style) string {
	if selected {
		// Paint the bar on whatever background the caller's selectedStyle
		// uses, so status-color callers (cooking / awaiting-input) get
		// the same bar treatment without clashing.
		bg := selectedStyle.GetBackground()
		bar := lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.Accent)).
			Background(bg).
			Render("▌")
		return bar + renderStyledPaneLine(selectedStyle, " "+content, contentWidth-1)
	}
	if defaultStyle != nil {
		return renderStyledPaneLine(*defaultStyle, "  "+content, contentWidth)
	}
	return padLineToWidth("  "+content, contentWidth)
}

// renderListPane composes a titled, scrollable list pane: pre-rendered rows
// (one string per visible row) are sliced to fit the visible area, with an
// empty-state fallback when no rows exist.
func (m Model) renderListPane(width, height int, title string, focused bool, rows []string, selected int, emptyState []string) string {
	if len(rows) == 0 {
		return renderPaneWithTitle(width, height, title, strings.Join(emptyState, "\n"), focused)
	}
	visibleHeight := height - 1
	rows = viewportSlice(rows, selected, visibleHeight)
	return renderPaneWithTitle(width, height, title, strings.Join(rows, "\n"), focused)
}
