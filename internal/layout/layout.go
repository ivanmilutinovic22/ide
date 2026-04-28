// Package layout holds the pure geometry calculations used by the TUI:
// splitting the screen into panes, sizing modal popups, picking the visible
// slice of a scrollable list. These functions don't touch lipgloss styles
// or Model state, which makes them straightforward to unit test.
package layout

// SplitPaneWidths splits a horizontal width into a (left, right) pair.
// Left pane is roughly 1/3 of total but clamped to [28, 44]; right pane
// gets the remainder, with a 28-col minimum that wins over the left clamp
// when the screen is wide enough (>= 56). At very narrow widths each pane
// is guaranteed at least 1 column.
func SplitPaneWidths(total int) (int, int) {
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

// SplitLeftPaneHeights divides the left column between Sessions (top) and
// Templates (bottom). Templates always reserves room for ~5 rows so adding
// a few templates doesn't reflow the layout each time, but it never claims
// more than half the column. Sessions takes the rest.
func SplitLeftPaneHeights(total, templateCount int) (int, int) {
	if total <= 2 {
		return 1, 1
	}

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
