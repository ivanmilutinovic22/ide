package ui

import (
	"reflect"
	"testing"
)

// TestFuzzyMatch verifies the byte-level subsequence match used by the
// fuzzy search popup. The implementation in fuzzy.go is case-sensitive
// and operates on raw bytes (no rune handling, no lowercasing).
func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		target string
		want   bool
	}{
		{"exact match", "abc", "abc", true},
		{"subsequence", "abc", "axbycz", true},
		{"out-of-order is no match", "abc", "acb", false},
		{"empty query matches anything", "", "anything", true},
		{"empty query and empty target matches", "", "", true},
		{"empty target with non-empty query does not match", "a", "", false},
		{"prefix match", "ab", "abcdef", true},
		{"suffix match", "ef", "abcdef", true},
		// CHARACTERIZATION: fuzzyMatch is case-sensitive at the byte level.
		// Callers (e.g. computeFuzzySearchResults) lowercase both sides before
		// invoking it, so the case-sensitivity is hidden in practice.
		{"case sensitive: uppercase query, lowercase target", "ABC", "abc", false},
		{"case sensitive: lowercase query, uppercase target", "abc", "ABC", false},
		{"longer query than target", "abcd", "abc", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fuzzyMatch(tc.query, tc.target)
			if got != tc.want {
				t.Errorf("fuzzyMatch(%q, %q) = %v, want %v", tc.query, tc.target, got, tc.want)
			}
		})
	}
}

// TestMoveFuzzySearchCursorDoesNotLandOnHeader verifies that pressing
// up/down in the fuzzy search popup never leaves the cursor parked on a
// session-header row. Because pressing Enter on a header is a no-op, a
// cursor stuck on a header makes the popup feel broken: arrow-keys appear
// to move "to nothing", and Enter does nothing.
//
// The bug is in moveFuzzySearchCursor's "Skip headers in the direction of
// movement" loop. When the cursor moves past index 0 (the first header),
// the loop walks negative, the final clamp resets to 0, and the cursor
// ends up parked on the very header it was trying to skip.
func TestMoveFuzzySearchCursorDoesNotLandOnHeader(t *testing.T) {
	header := fuzzySearchItem{IsHeader: true}
	window := fuzzySearchItem{}

	tests := []struct {
		name      string
		results   []fuzzySearchItem
		startCur  int
		direction int
	}{
		{
			name:      "up from first window with header above",
			results:   []fuzzySearchItem{header, window},
			startCur:  1,
			direction: -1,
		},
		{
			name:      "up from second-window past header",
			results:   []fuzzySearchItem{header, window, window},
			startCur:  1,
			direction: -1,
		},
		{
			name:      "down from last window into a final header (no following window)",
			// Not currently emitted by computeFuzzySearchResults, but the
			// cursor mover should still cope.
			results:   []fuzzySearchItem{header, window, header},
			startCur:  1,
			direction: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &Model{
				fuzzySearchResults: tc.results,
				fuzzySearchCursor:  tc.startCur,
			}
			m.moveFuzzySearchCursor(tc.direction)
			if m.fuzzySearchCursor < 0 || m.fuzzySearchCursor >= len(tc.results) {
				t.Fatalf("cursor out of bounds after move: %d (len=%d)", m.fuzzySearchCursor, len(tc.results))
			}
			if tc.results[m.fuzzySearchCursor].IsHeader {
				t.Errorf("cursor landed on header at index %d (results=%d)", m.fuzzySearchCursor, len(tc.results))
			}
		})
	}
}

// TestExtractTags verifies the [tag] extraction helper. The function mutates
// its argument in place to remove the tags and trim surrounding whitespace,
// returning the captured tag list.
func TestExtractTags(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantEntry string
		wantTags  []string
	}{
		{
			name:      "two tags interspersed",
			input:     "hello [foo] world [bar]",
			wantEntry: "hello  world",
			wantTags:  []string{"foo", "bar"},
		},
		{
			name:      "no tags",
			input:     "plain entry",
			wantEntry: "plain entry",
			wantTags:  nil,
		},
		{
			name:      "only tags adjacent",
			input:     "[a][b]",
			wantEntry: "",
			wantTags:  []string{"a", "b"},
		},
		{
			name:      "single tag at end",
			input:     "name [tag]",
			wantEntry: "name",
			wantTags:  []string{"tag"},
		},
		{
			name:      "empty string",
			input:     "",
			wantEntry: "",
			wantTags:  nil,
		},
		{
			name:      "tag at start with trailing content",
			input:     "[ai] do thing",
			wantEntry: "do thing",
			wantTags:  []string{"ai"},
		},
		{
			// CHARACTERIZATION: tagRe = `\[(\w+)\]` — only ASCII word chars
			// match. Tags with hyphens, spaces, or special chars are left in
			// place rather than captured.
			name:      "tag with hyphen is not captured",
			input:     "x [foo-bar]",
			wantEntry: "x [foo-bar]",
			wantTags:  nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := tc.input
			tags := extractTags(&entry)
			if entry != tc.wantEntry {
				t.Errorf("entry: got %q, want %q", entry, tc.wantEntry)
			}
			if !reflect.DeepEqual(tags, tc.wantTags) {
				t.Errorf("tags: got %#v, want %#v", tags, tc.wantTags)
			}
		})
	}
}
