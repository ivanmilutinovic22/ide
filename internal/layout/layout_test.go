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

// TestSplitLeftPaneHeights verifies that the templates pane is sized to its
// content (with a small slack) when there are few templates, while still
// reserving room for the placeholder when the list is empty, and capping at
// half the column when there are many templates.
func TestSplitLeftPaneHeights(t *testing.T) {
	tests := []struct {
		name          string
		total         int
		templateCount int
		wantTop       int
		wantBottom    int
	}{
		{"tiny total", 2, 5, 1, 1},
		{"empty templates still reserves min visible (5)", 50, 0, 43, 7},
		{"single template still reserves min visible (5)", 50, 1, 43, 7},
		{"three templates still reserves min visible (5)", 50, 3, 43, 7},
		{"five templates uses min row count", 50, 5, 43, 7},
		{"more than min grows pane", 50, 8, 40, 10},
		{"many templates capped at half", 50, 30, 25, 25},
		{"narrow column falls back to half", 8, 0, 4, 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			top, bottom := SplitLeftPaneHeights(tc.total, tc.templateCount)
			if top != tc.wantTop || bottom != tc.wantBottom {
				t.Errorf("SplitLeftPaneHeights(%d, %d) = (%d, %d), want (%d, %d)",
					tc.total, tc.templateCount, top, bottom, tc.wantTop, tc.wantBottom)
			}
			if top+bottom != tc.total {
				t.Errorf("top+bottom = %d, want %d", top+bottom, tc.total)
			}
		})
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
