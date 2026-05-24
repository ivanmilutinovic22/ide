// Package layout holds the pure geometry calculations used by the TUI:
// splitting the screen into panes, sizing modal popups, picking the visible
// slice of a scrollable list. These functions don't touch lipgloss styles
// or Model state, which makes them straightforward to unit test.
package layout

// SplitPaneWidths splits a horizontal width into a (left, right) pair.
// Left pane is roughly 1/4 of total but clamped to [28, 36]; right pane
// gets the remainder, with a 28-col minimum that wins over the left clamp
// when the screen is wide enough (>= 56). At very narrow widths each pane
// is guaranteed at least 1 column.
//
// The left max is kept tight — sessions/agents/templates lists fit
// comfortably in ~33 cols — so wide terminals give the preview/terminal
// pane on the right as much room as possible.
func SplitPaneWidths(total int) (int, int) {
	leftWidth := total / 4
	if leftWidth < 28 {
		leftWidth = 28
	}
	if leftWidth > 36 {
		leftWidth = 36
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

// ModalPopupDimensions clamps a desired (width, height) into a sensible
// modal popup size. Caps width at 96 and height at 20; floors at 44×10
// when the body has room, otherwise shrinks down to fit.
func ModalPopupDimensions(bodyWidth, bodyHeight, desiredWidth, desiredHeight int) (int, int) {
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

// PaneContentWidth returns the usable text width inside a borderless pane
// of the given total width. The borderless pane uses Padding(0,1), so two
// columns are reserved for left/right padding.
func PaneContentWidth(width int) int {
	contentWidth := width - 2
	if contentWidth < 0 {
		return 0
	}
	return contentWidth
}

// SplitLeftPaneHeights divides the left column into three stacked sections:
// Sessions (top), Agents (middle), Templates (bottom). Agents and Templates
// each reserve room for ~5 rows so a few entries don't reflow the layout,
// but neither claims more than 1/3 of the column. Sessions takes the rest.
func SplitLeftPaneHeights(total, agentCount, templateCount int) (sessions, agents, templates int) {
	if total <= 3 {
		return 1, 1, 1
	}

	const minVisible = 5
	desire := func(count int) int {
		rows := count
		if rows < minVisible {
			rows = minVisible
		}
		return 1 + rows + 1 // title + rows + slack
	}

	sectionCap := total / 3
	if sectionCap < 3 {
		sectionCap = 3
	}

	agents = desire(agentCount)
	if agents > sectionCap {
		agents = sectionCap
	}
	templates = desire(templateCount)
	if templates > sectionCap {
		templates = sectionCap
	}

	sessions = total - agents - templates
	if sessions < 1 {
		// Squeeze the bottom two so Sessions keeps at least one row.
		excess := 1 - sessions
		shrinkAgents := excess / 2
		shrinkTemplates := excess - shrinkAgents
		agents -= shrinkAgents
		templates -= shrinkTemplates
		if agents < 1 {
			agents = 1
		}
		if templates < 1 {
			templates = 1
		}
		sessions = total - agents - templates
		if sessions < 1 {
			sessions = 1
		}
	}
	return sessions, agents, templates
}

// ViewportSlice returns the visible slice of rows for a scrollable list
// where `selected` is the cursor index and `maxVisible` is the row capacity.
// The slice always includes the selected row when one fits.
func ViewportSlice(rows []string, selected, maxVisible int) []string {
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
