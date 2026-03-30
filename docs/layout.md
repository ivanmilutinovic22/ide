# Three-Pane Full-Screen Layout with Bubble Tea + Lipgloss

## 1. Capture Terminal Size

Bubble Tea sends `tea.WindowSizeMsg` whenever the terminal resizes. Store it:

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
```

## 2. Compute Pane Dimensions

Split the available space programmatically:

```go
leftWidth, rightWidth := splitPaneWidths(m.width - 1)  // -1 for gap
bodyHeight := m.height - 2                               // -1 status bar, -1 gap
topHeight, bottomHeight := splitLeftPaneHeights(bodyHeight - 1) // -1 for gap
```

`splitPaneWidths` gives left 1/3 (clamped 28–44 chars), right gets the rest.
`splitLeftPaneHeights` gives top 2/3, bottom 1/3.

## 3. Render Each Pane Independently

Each pane is a function that takes `(width, height)` and returns a styled string:

```go
leftTopPane  := m.renderEnvironmentPane(leftWidth, topHeight)
leftBotPane  := m.renderTemplatesPane(leftWidth, bottomHeight)
rightPane    := m.renderDetailsPane(rightWidth, bodyHeight)
```

Inside each, a helper like `renderPaneWithTitle` applies a lipgloss style with exact dimensions:

```go
func paneBoxStyle(width, height int, focused bool) lipgloss.Style {
    return paneStyle.Width(width).Height(height).MaxHeight(height).Padding(0, 1)
}
```

`.Width()` and `.Height()` are the key — they force the lipgloss block to fill exactly that space, regardless of content length.

## 4. Compose with JoinVertical / JoinHorizontal

No grid system, just joining styled blocks:

```go
// Stack top-left and bottom-left vertically
verticalGap := lipgloss.NewStyle().Width(leftWidth).Background(gapBG).Render("")
leftPane := lipgloss.JoinVertical(lipgloss.Left, leftTopPane, verticalGap, leftBotPane)

// Place left column and right pane side by side
horizontalGap := lipgloss.NewStyle().Width(1).Height(bodyHeight).Background(gapBG).Render("")
body := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, horizontalGap, rightPane)
```

Visual structure:

```
┌──────────────┐ ┌──────────────────────────┐
│  Env pane    │ │                           │
│  (top-left)  │ │   Details pane (right)    │
├──────────────┤ │                           │
│  Templates   │ │                           │
│  (bot-left)  │ │                           │
└──────────────┘ └──────────────────────────┘
[status bar                                  ]
```

## 5. Focus Styling

Focus is tracked as an int (`focusPane`). The focused pane gets a bright accent-colored title, unfocused panes get muted titles:

```go
func panelTitle(shortcut, name string, focused bool, theme uiTheme) string {
    color := theme.Muted
    if focused { color = theme.Accent }
    return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(focused).Render(...)
}
```

## 6. Final Placement

Fill the entire terminal:

```go
rendered := lipgloss.JoinVertical(lipgloss.Left, body, bottomGap, status)
rendered = lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, rendered,
    lipgloss.WithWhitespaceBackground(lipgloss.Color(theme.AppBG)))
```

`lipgloss.Place` aligns the content within the full terminal dimensions and fills any remaining space with the background color.

## The Minimal Recipe

To replicate this pattern from scratch:

1. **Store `width`/`height`** from `tea.WindowSizeMsg`
2. **Compute splits** — simple arithmetic, no layout engine needed
3. **Render each pane** as a lipgloss-styled string with fixed `.Width()` and `.Height()`
4. **Compose** with `lipgloss.JoinVertical` and `lipgloss.JoinHorizontal`
5. **Fill the screen** with `lipgloss.Place(width, height, ...)`

No CSS, no flexbox — just explicit dimension math and string joining.
