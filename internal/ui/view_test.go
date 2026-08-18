package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// bgANSISeq parses lipgloss's rendered output to pull out the background
// escape sequence. In test runs lipgloss may detect no color profile and
// render plain text, so the seq can legitimately be empty. The point of
// this test is to pin shape invariants: when lipgloss DOES emit a sequence,
// it must precede the marker rune and not leak the marker.
func TestBgANSISeqShape(t *testing.T) {
	seq := bgANSISeq(lipgloss.Color("#ff0000"))
	if seq == "" {
		t.Skip("lipgloss returned no ANSI output in this test profile; nothing to verify")
	}
	if !strings.HasPrefix(seq, "\x1b[") {
		t.Errorf("bgANSISeq result %q does not look like an ANSI escape", seq)
	}
	if strings.Contains(seq, "X") {
		t.Errorf("bgANSISeq leaked marker character: %q", seq)
	}
}

func TestBgANSISeqHandlesNoColorGracefully(t *testing.T) {
	// Passing NoColor should not panic and should return empty (or at
	// worst a no-op sequence).
	_ = bgANSISeq(lipgloss.NoColor{})
}

func TestClampPopupWidth(t *testing.T) {
	// Cap at max
	if got := clampPopupWidth(200, 220, popupMaxWidthSmall); got != popupMaxWidthSmall {
		t.Errorf("expected cap at %d, got %d", popupMaxWidthSmall, got)
	}
	// Below preferred min: take all body width
	if got := clampPopupWidth(30, 50, popupMaxWidthSmall); got != 48 {
		t.Errorf("expected fallback to bodyWidth-2 (48), got %d", got)
	}
	// Below absolute floor
	if got := clampPopupWidth(5, 10, popupMaxWidthSmall); got != popupMinWidth {
		t.Errorf("expected floor at %d, got %d", popupMinWidth, got)
	}
}

// TestModalInputWidthPositive verifies the shared modal-input width helper
// returns a usable (>0) width once the terminal size is known, so the
// persisted textinput can scroll horizontally.
func TestModalInputWidthPositive(t *testing.T) {
	m := NewModel()
	m.width = 120
	m.height = 40
	if w := m.modalInputWidth(m.envEditSpec.Prompt); w <= 0 {
		t.Fatalf("expected positive input width, got %d", w)
	}
	// Unknown size yields 0 (nothing to render yet).
	m.width, m.height = 0, 0
	if w := m.modalInputWidth(m.envEditSpec.Prompt); w != 0 {
		t.Errorf("expected 0 width before first resize, got %d", w)
	}
}

// TestEnvEditSpecScrollsToEnd verifies that a spec longer than the input
// width is scrolled so the tail is visible (cursor at end), rather than
// clipped at the start. This guards the horizontal-panning fix: the width
// must be set on the persisted textinput so bubbles' handleOverflow advances
// the offset when SetValue/CursorEnd move the cursor past the visible width.
func TestEnvEditSpecScrollsToEnd(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24
	m.syncModalInputWidths()

	long := "alpha=cmd;beta=cmd;gamma=cmd;delta=cmd;epsilon=cmd;zeta=cmd;THE_TAIL=cmd"
	m.envEditSpec.SetValue(long)
	m.envEditSpec.CursorEnd()

	view := m.envEditSpec.View()
	if !strings.Contains(view, "THE_TAIL") {
		t.Errorf("expected the end of a long spec to be visible after CursorEnd, got:\n%s", view)
	}
}
