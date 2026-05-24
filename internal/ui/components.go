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

// renderListRow applies the standard selection marker and styling to a
// list-row content string. `defaultStyle == nil` means "no styling, just
// pad to width" (used when the row should look like plain text).
func renderListRow(content string, selected bool, contentWidth int, selectedStyle lipgloss.Style, defaultStyle *lipgloss.Style) string {
	if selected {
		return renderStyledPaneLine(selectedStyle, "▸ "+content, contentWidth)
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
