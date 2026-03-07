package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestBorderlessPaneWidth verifies that borderless pane boxes render at exactly
// the requested width. lipgloss Width() includes padding but NOT border, so
// removing a border without adjusting Width shrinks the output by 2 chars.
func TestBorderlessPaneWidth(t *testing.T) {
	bordered := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Background(lipgloss.Color("236")).
		Width(38).
		Height(3).
		Padding(0, 1)
	borderless := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Width(40).
		Height(3).
		Padding(0, 1)

	bResult := bordered.Render("hello")
	blResult := borderless.Render("hello")

	bWidth := lipgloss.Width(bResult)
	blWidth := lipgloss.Width(blResult)

	// Bordered: Width(38) + 2 (border) = 40
	if bWidth != 40 {
		t.Errorf("bordered rendered width = %d, want 40", bWidth)
	}
	// Borderless: Width(40) = 40
	if blWidth != 40 {
		t.Errorf("borderless rendered width = %d, want 40", blWidth)
	}
	if bWidth != blWidth {
		t.Errorf("bordered (%d) and borderless (%d) widths should match", bWidth, blWidth)
	}
}

// TestPaneBoxStyleWidth verifies that paneBoxStyle renders at the requested width.
func TestPaneBoxStyleWidth(t *testing.T) {
	// paneStyle is a package-level var initialized borderless
	for _, width := range []int{30, 40, 60, 80} {
		result := paneBoxStyle(width, 5, false).Render("test")
		got := lipgloss.Width(result)
		if got != width {
			t.Errorf("paneBoxStyle(%d, 5, false): rendered width = %d, want %d", width, got, width)
		}

		resultFocused := paneBoxStyle(width, 5, true).Render("test")
		gotFocused := lipgloss.Width(resultFocused)
		if gotFocused != width {
			t.Errorf("paneBoxStyle(%d, 5, true): rendered width = %d, want %d", width, gotFocused, width)
		}
	}
}

// TestPaneContentWidth verifies that paneContentWidth returns the usable text
// width inside a borderless pane (total width minus padding).
func TestPaneContentWidth(t *testing.T) {
	for _, width := range []int{30, 40, 60, 80} {
		got := paneContentWidth(width)
		want := width - 2 // padding only
		if got != want {
			t.Errorf("paneContentWidth(%d) = %d, want %d", width, got, want)
		}
	}
}

// TestModalBoxStyleWidth verifies that modalBoxStyle (bordered) renders at the
// requested width.
func TestModalBoxStyleWidth(t *testing.T) {
	for _, width := range []int{44, 60, 80, 96} {
		result := modalBoxStyle(width, 10).Render("modal content")
		got := lipgloss.Width(result)
		if got != width {
			t.Errorf("modalBoxStyle(%d, 10): rendered width = %d, want %d", width, got, width)
		}
	}
}

// TestModalContentWidth verifies that modalContentWidth returns the usable text
// width inside a bordered modal (total width minus border and padding).
func TestModalContentWidth(t *testing.T) {
	for _, width := range []int{44, 60, 80, 96} {
		got := modalContentWidth(width)
		want := width - modalPaneStyle.GetHorizontalFrameSize() - 2
		if got != want {
			t.Errorf("modalContentWidth(%d) = %d, want %d", width, got, want)
		}
	}
}

// TestTitleAndPaneWidthMatch verifies that renderPaneWithTitle produces output
// where the title line and pane body have the same width.
func TestTitleAndPaneWidthMatch(t *testing.T) {
	title := panelTitle("s", "Sessions", true, defaultThemes()[0])
	result := renderPaneWithTitle(40, 10, title, "line1\nline2", true)

	lines := splitLines(result)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	titleWidth := lipgloss.Width(lines[0])
	bodyWidth := lipgloss.Width(lines[1])
	if titleWidth != bodyWidth {
		t.Errorf("title width (%d) != body width (%d)", titleWidth, bodyWidth)
	}
	if titleWidth != 40 {
		t.Errorf("title width = %d, want 40", titleWidth)
	}
}

func TestViewportSlice(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}

	tests := []struct {
		name     string
		selected int
		maxVis   int
		want     []string
	}{
		{"all fit", 0, 10, []string{"a", "b", "c", "d", "e"}},
		{"top selected", 0, 3, []string{"a", "b", "c"}},
		{"middle selected", 2, 3, []string{"a", "b", "c"}},
		{"scroll down", 3, 3, []string{"b", "c", "d"}},
		{"bottom selected", 4, 3, []string{"c", "d", "e"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := viewportSlice(items, tt.selected, tt.maxVis)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
