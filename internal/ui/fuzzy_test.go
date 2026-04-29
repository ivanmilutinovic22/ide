package ui

import (
	"reflect"
	"testing"

	"ide/internal/config"
	"ide/internal/tmux"
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
			name: "down from last window into a final header (no following window)",
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

// TestRebuildFuzzyIndexAndCompute is an end-to-end test for the fuzzy
// pipeline: rebuildFuzzyIndex populates m.fuzzySearchCache from the model's
// environments + sessions + windowProcessInfo state, and
// computeFuzzySearchResults filters that cache by the current query.
//
// This guards three behaviors that other handlers rely on:
//  1. Empty query emits header + every window for every env.
//  2. Query filters by env name, window name, and tag text.
//  3. Cache reflects the m.sessions Running flag — including the in-place
//     mutation done by terminalSessionReadyMsg, which adds a session to
//     m.sessions and then calls rebuildFuzzyIndex without a round-trip
//     through loadSessionsCmd.
func TestRebuildFuzzyIndexAndCompute(t *testing.T) {
	envA := config.Environment{
		Name: "alpha",
		Windows: []config.WindowTemplate{
			{Name: "shell"},
			{Name: "agent", Tags: []string{"ai"}},
		},
	}
	envB := config.Environment{
		Name: "beta",
		Windows: []config.WindowTemplate{
			{Name: "shell"},
		},
	}

	m := &Model{
		environments:      []config.Environment{envA, envB},
		sessions:          map[string]struct{}{},
		sessionWindows:    map[string][]string{},
		windowProcessInfo: map[string]WindowProcessInfo{},
		fuzzySearchQuery:  newTextInput("/ ", ""),
	}
	m.rebuildFuzzyIndex()

	t.Run("empty query yields header plus every window per env", func(t *testing.T) {
		m.fuzzySearchQuery.SetValue("")
		results := m.computeFuzzySearchResults()
		// envA: 1 header + 2 windows; envB: 1 header + 1 window = 5.
		if len(results) != 5 {
			t.Fatalf("expected 5 result rows, got %d: %+v", len(results), results)
		}
		if !results[0].IsHeader || results[0].EnvName != "alpha" {
			t.Errorf("expected first row to be alpha header, got %+v", results[0])
		}
		if !results[3].IsHeader || results[3].EnvName != "beta" {
			t.Errorf("expected fourth row to be beta header, got %+v", results[3])
		}
	})

	t.Run("query filters by tag text", func(t *testing.T) {
		m.fuzzySearchQuery.SetValue("ai")
		results := m.computeFuzzySearchResults()
		// Only envA's "agent" window has the [ai] tag, plus envA's header.
		var windows []fuzzySearchItem
		for _, r := range results {
			if !r.IsHeader {
				windows = append(windows, r)
			}
		}
		if len(windows) != 1 || windows[0].WindowName != "agent" {
			t.Errorf("expected only the [ai]-tagged window to match, got %+v", windows)
		}
	})

	t.Run("query filters by env name", func(t *testing.T) {
		m.fuzzySearchQuery.SetValue("beta")
		results := m.computeFuzzySearchResults()
		for _, r := range results {
			if r.IsHeader && r.EnvName != "beta" {
				t.Errorf("unexpected header in beta-filtered results: %+v", r)
			}
			if !r.IsHeader && r.EnvName != "beta" {
				t.Errorf("unexpected window in beta-filtered results: %+v", r)
			}
		}
	})

	t.Run("Running flag follows m.sessions in-place mutation", func(t *testing.T) {
		// Initially no sessions are running.
		m.fuzzySearchQuery.SetValue("")
		m.rebuildFuzzyIndex()
		results := m.computeFuzzySearchResults()
		for _, r := range results {
			if r.Running {
				t.Errorf("expected no row to be Running before session start, got %+v", r)
			}
		}

		// Simulate terminalSessionReadyMsg: directly mark alpha's session as live
		// and rebuild the index (matching what update.go now does).
		m.sessions[tmux.SessionName(envA.Name)] = struct{}{}
		m.rebuildFuzzyIndex()
		results = m.computeFuzzySearchResults()
		var alphaRunning, betaRunning bool
		for _, r := range results {
			if r.EnvName == "alpha" && r.Running {
				alphaRunning = true
			}
			if r.EnvName == "beta" && r.Running {
				betaRunning = true
			}
		}
		if !alphaRunning {
			t.Errorf("expected alpha rows to be Running after sessions mutation")
		}
		if betaRunning {
			t.Errorf("expected beta rows NOT to be Running")
		}
	})
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
