package ui

import (
	"testing"
)

func TestClampIndex(t *testing.T) {
	tests := []struct {
		idx, count, want int
	}{
		{0, 0, 0},
		{5, 0, 0},
		{-1, 5, 0},
		{0, 5, 0},
		{4, 5, 4},
		{5, 5, 4},
		{100, 5, 4},
		{-100, 5, 0},
		{3, -1, 0},
	}
	for _, tc := range tests {
		if got := clampIndex(tc.idx, tc.count); got != tc.want {
			t.Errorf("clampIndex(%d, %d) = %d, want %d", tc.idx, tc.count, got, tc.want)
		}
	}
}

func TestNormalizeFuzzySearchCursorHeaderOnly(t *testing.T) {
	header := fuzzySearchItem{IsHeader: true}
	m := &Model{
		fuzzySearchResults: []fuzzySearchItem{header, header, header},
		fuzzySearchCursor:  1,
	}
	// All headers — cursor must stay in-bounds.
	m.normalizeFuzzySearchCursor()
	if m.fuzzySearchCursor < 0 || m.fuzzySearchCursor >= 3 {
		t.Errorf("cursor out of bounds with all-header list: %d", m.fuzzySearchCursor)
	}
}

func TestNormalizeFuzzySearchCursorWalksBackPastTrailingHeader(t *testing.T) {
	header := fuzzySearchItem{IsHeader: true}
	window := fuzzySearchItem{}
	// Layout: [header, window, header] — cursor on last header should land on the window.
	m := &Model{
		fuzzySearchResults: []fuzzySearchItem{header, window, header},
		fuzzySearchCursor:  2,
	}
	m.normalizeFuzzySearchCursor()
	if m.fuzzySearchCursor != 1 {
		t.Errorf("expected cursor=1 (the only window), got %d", m.fuzzySearchCursor)
	}
}

func TestNormalizeFuzzySearchCursorPicksFirstWindowAfterHeader(t *testing.T) {
	header := fuzzySearchItem{IsHeader: true}
	window := fuzzySearchItem{}
	// [header, window, window] — cursor on header should land on first window.
	m := &Model{
		fuzzySearchResults: []fuzzySearchItem{header, window, window},
		fuzzySearchCursor:  0,
	}
	m.normalizeFuzzySearchCursor()
	if m.fuzzySearchCursor != 1 {
		t.Errorf("expected cursor=1, got %d", m.fuzzySearchCursor)
	}
}

func TestKeyToBytesUnknownKeyDropped(t *testing.T) {
	// Multi-rune key names we don't understand should not be smuggled into
	// the PTY as their literal name. Drop instead.
	cases := []string{
		"shift+f13",
		"ctrl+alt+delete",
		"super+space",
		"hyper+x",
	}
	for _, key := range cases {
		got := keyToBytes(key)
		if got != nil {
			t.Errorf("keyToBytes(%q) = %q, want nil", key, got)
		}
	}
}

func TestKeyToBytesAltRecursesIntoInnerKey(t *testing.T) {
	// alt+<key> should send ESC followed by the inner key's bytes — not
	// the literal string "ctrl+x".
	got := keyToBytes("alt+ctrl+x")
	want := []byte{0x1b, 0x18}
	if string(got) != string(want) {
		t.Errorf("keyToBytes(alt+ctrl+x) = % x, want % x", got, want)
	}

	got = keyToBytes("alt+a")
	want = []byte{0x1b, 'a'}
	if string(got) != string(want) {
		t.Errorf("keyToBytes(alt+a) = % x, want % x", got, want)
	}
}

func TestKeyToBytesSingleRunePassesThrough(t *testing.T) {
	cases := map[string]byte{
		"a": 'a',
		"Z": 'Z',
		"1": '1',
		"!": '!',
	}
	for key, want := range cases {
		got := keyToBytes(key)
		if len(got) != 1 || got[0] != want {
			t.Errorf("keyToBytes(%q) = %v, want [%c]", key, got, want)
		}
	}
}

func TestKeyToBytesCtrlLetter(t *testing.T) {
	if got := keyToBytes("ctrl+a"); len(got) != 1 || got[0] != 0x01 {
		t.Errorf("keyToBytes(ctrl+a) = %v, want 0x01", got)
	}
	if got := keyToBytes("ctrl+z"); len(got) != 1 || got[0] != 0x1a {
		t.Errorf("keyToBytes(ctrl+z) = %v, want 0x1a", got)
	}
}

func TestColorRGBHexValid(t *testing.T) {
	r, g, b := colorRGB("#ff8000")
	if r != 0xff || g != 0x80 || b != 0x00 {
		t.Errorf("colorRGB(#ff8000) = (%d,%d,%d), want (255,128,0)", r, g, b)
	}
}

func TestColorRGBHexMalformed(t *testing.T) {
	// Garbage hex chars should fall back to neutral grey, not (0,0,0).
	r, g, b := colorRGB("#zzzzzz")
	if r != 128 || g != 128 || b != 128 {
		t.Errorf("colorRGB(#zzzzzz) = (%d,%d,%d), want (128,128,128) fallback", r, g, b)
	}
}

func TestColorRGBBadInputFallsBackToGrey(t *testing.T) {
	r, g, b := colorRGB("not-a-color")
	if r != 128 || g != 128 || b != 128 {
		t.Errorf("colorRGB(\"not-a-color\") = (%d,%d,%d), want (128,128,128)", r, g, b)
	}
}
