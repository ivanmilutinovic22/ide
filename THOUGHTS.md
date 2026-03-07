# UI Rework: OpenCode-Inspired Borderless Style

## What Changed
Replaced bordered panes with background-shaded borderless blocks separated by gaps.
Modals (create/template/theme/shortcuts) retain rounded borders.

## Visual Design
- **Color layers**: AppBG (darkest) → PaneBG (pane fill) → SelectedBG (hover/select)
- **Focus indicator**: Bold accent-colored title line at top of focused pane (vs muted for unfocused)
- **Gaps**: 1-char horizontal gap + 1-line vertical gap between panes, filled with AppBG

## Key Changes in model.go

### Global style vars (lines ~24-80)
- `paneStyle` / `focusedPaneStyle` — borderless, just background + fg
- `modalPaneStyle` — NEW, retains `RoundedBorder()` for popup modals

### `applyThemeStyles()` (~line 554)
- Sets borderless pane styles from theme colors
- Sets `modalPaneStyle` with accent border color

### Helper functions
| Function | Purpose |
|---|---|
| `paneBoxStyle(w, h, focused)` | Borderless box: bg + dimensions + Padding(0,1) |
| `modalBoxStyle(w, h)` | Bordered box for modals |
| `paneContentWidth(w)` | `w - 2` (just padding, no border frame) |
| `modalContentWidth(w)` | `w - modalPaneStyle.GetHorizontalFrameSize() - 2` |
| `panelTitle(name, focused, theme)` | Bold title line, accent if focused, muted if not |
| `renderPaneWithTitle(w, h, title, body, focused, theme)` | Main pane renderer: title line + body, no border |
| `renderModalWithBorderTitle(w, h, title, body)` | Modal renderer: body in bordered box with injected title |

### Removed
- `panelBorderTitle()` — replaced by `panelTitle()`
- `renderPaneWithBorderTitle()` — replaced by `renderPaneWithTitle()` + `renderModalWithBorderTitle()`

### `View()` layout
- `splitPaneWidths(m.width - 1)` — reserves 1 char for horizontal gap
- Vertical gap: 1-line `lipgloss.Style{AppBG}.Width(leftWidth)` between top/bottom left panes
- Horizontal gap: `Width(1).Height(bodyHeight)` column between left and right
- `rightPaneHeight = bodyHeight` (no border subtraction needed)

### Height accounting in renderDetailsPane
- `contentHeight = height - 1` (title line takes 1 row, no border rows)

### Pane render functions
- `renderEnvironmentPane()` → `renderPaneWithTitle(..., "Sessions", ...)`
- `renderTemplatesPane()` → `renderPaneWithTitle(..., "Templates", ...)`
- `renderDetailsPane()` → `renderPaneWithTitle(..., "Windows", ...)`

### Modal render functions (kept bordered)
- `renderCreatePane()`, `renderTemplatePane()`, `renderThemePickerPane()`, `renderShortcutsPane()`
- Use `modalContentWidth()` + `renderModalWithBorderTitle()`

## Unchanged
- `injectBorderTitle()` — still used by `renderModalWithBorderTitle()`
- `overlayCentered()`, `backdropSegment()` — modals still work the same
- All keyboard handling, async commands, tmux logic
- Preview content rendering (ANSI bg injection)
