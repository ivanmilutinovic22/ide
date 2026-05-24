package layout

import "testing"

func TestPaneContentWidth(t *testing.T) {
	for _, width := range []int{30, 40, 60, 80} {
		got := PaneContentWidth(width)
		want := width - 2 // padding only
		if got != want {
			t.Errorf("PaneContentWidth(%d) = %d, want %d", width, got, want)
		}
	}
}

func TestPaneContentWidthClampsBelowZero(t *testing.T) {
	for _, width := range []int{-5, 0, 1, 2} {
		got := PaneContentWidth(width)
		if got < 0 {
			t.Errorf("PaneContentWidth(%d) = %d, want >= 0", width, got)
		}
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
			got := ViewportSlice(items, tt.selected, tt.maxVis)
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

// TestSplitLeftPaneHeights verifies the three-way split between Sessions
// (top), Agents (middle), and Templates (bottom). Agents and Templates each
// reserve room for ~5 rows so adding a few entries doesn't reflow, but
// neither claims more than 1/3 of the column.
func TestSplitLeftPaneHeights(t *testing.T) {
	tests := []struct {
		name        string
		total       int
		agentCount  int
		tmplCount   int
		wantSession int
		wantAgents  int
		wantTmpl    int
	}{
		{"tiny total", 3, 5, 5, 1, 1, 1},
		{"empty agents+templates reserve min visible (5)", 60, 0, 0, 46, 7, 7},
		{"few agents, few templates use min", 60, 1, 3, 46, 7, 7},
		{"more than min grows agents pane", 60, 8, 3, 43, 10, 7},
		{"more than min grows templates pane", 60, 3, 8, 43, 7, 10},
		{"agents capped at 1/3", 60, 30, 0, 33, 20, 7},
		{"templates capped at 1/3", 60, 0, 30, 33, 7, 20},
		{"both capped at 1/3 each", 60, 30, 30, 20, 20, 20},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, a, b := SplitLeftPaneHeights(tc.total, tc.agentCount, tc.tmplCount)
			if s != tc.wantSession || a != tc.wantAgents || b != tc.wantTmpl {
				t.Errorf("SplitLeftPaneHeights(%d, %d, %d) = (%d, %d, %d), want (%d, %d, %d)",
					tc.total, tc.agentCount, tc.tmplCount, s, a, b, tc.wantSession, tc.wantAgents, tc.wantTmpl)
			}
			if s+a+b != tc.total {
				t.Errorf("sessions+agents+templates = %d, want %d", s+a+b, tc.total)
			}
		})
	}
}

func TestTerminalPreviewHeight(t *testing.T) {
	tests := []struct {
		height, want int
	}{
		{0, 1},   // floored
		{5, 1},   // floored after reservation
		{6, 1},
		{7, 1},
		{8, 2},
		{20, 14},
		{50, 44},
	}
	for _, tc := range tests {
		if got := TerminalPreviewHeight(tc.height); got != tc.want {
			t.Errorf("TerminalPreviewHeight(%d) = %d, want %d", tc.height, got, tc.want)
		}
	}
}

// TestSplitPaneWidthsSums verifies left+right == total once total is wide
// enough that both panes can satisfy their 1-col floor (>=2). At very narrow
// widths each pane is clamped to 1, which can exceed the requested total.
func TestSplitPaneWidthsSums(t *testing.T) {
	for _, total := range []int{2, 10, 28, 56, 80, 120, 200} {
		l, r := SplitPaneWidths(total)
		if l+r != total {
			t.Errorf("SplitPaneWidths(%d) = (%d, %d); sum %d != %d", total, l, r, l+r, total)
		}
		if l < 1 || r < 1 {
			t.Errorf("SplitPaneWidths(%d) = (%d, %d); both panes must be >= 1", total, l, r)
		}
	}
}
