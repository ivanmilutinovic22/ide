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
