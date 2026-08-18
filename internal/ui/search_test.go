package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"

	"ide/internal/config"
	"ide/internal/tmux"
)

// TestSearchComputeResultsTagsByName verifies that SearchModel.computeResults
// associates tags with windows by NAME, not by positional index. The previous
// implementation used `env.Windows[winIdx].Tags`, which silently mismatches
// when sessionWindows has a different order from env.Windows (e.g. after a
// live tmux reorder, or when window count differs).
//
// In this scenario:
//   - env.Windows declares: alpha (no tags), beta (z9q9 tag)
//   - live sessionWindows reports: beta, alpha  (live order swapped)
//
// Each result row's tags should match the WINDOW NAME, not the slot index.
// With index-based lookup, the "beta" row gets tags from env.Windows[0]
// (alpha's nil tags) and the "alpha" row inherits beta's z9q9 tag — bug.
func TestSearchComputeResultsTagsByName(t *testing.T) {
	env := config.Environment{
		Name: "proj",
		Windows: []config.WindowTemplate{
			{Name: "alpha"},
			{Name: "beta", Tags: []string{"z9q9"}},
		},
	}
	session := tmux.SessionName(env.Name)

	// Empty query so every window appears, regardless of haystack contents.
	ti := textinput.New()

	m := SearchModel{
		query: ti,
		envs:  []config.Environment{env},
		sessions: map[string]struct{}{
			session: {},
		},
		sessionWindows: map[string][]string{
			// Live order has beta first, alpha second — opposite of env.Windows.
			session: {"beta", "alpha"},
		},
	}

	results := m.computeResults()

	gotTags := map[string][]string{}
	for _, r := range results {
		if r.header {
			continue
		}
		gotTags[r.window] = r.tags
	}

	// alpha has no tags; beta carries z9q9.
	if len(gotTags["alpha"]) != 0 {
		t.Errorf("alpha should have no tags, got %v", gotTags["alpha"])
	}
	if len(gotTags["beta"]) != 1 || gotTags["beta"][0] != "z9q9" {
		t.Errorf("beta should carry tag [z9q9], got %v", gotTags["beta"])
	}
}

// TestSearchComputeResultsScopedToCurrentSession verifies that when
// scopeSession is set, computeResults lists only that session's windows, but
// keeps the session-header row (same style as the cross-session popup).
func TestSearchComputeResultsScopedToCurrentSession(t *testing.T) {
	envA := config.Environment{
		Name:    "alpha",
		Windows: []config.WindowTemplate{{Name: "editor"}, {Name: "logs"}},
	}
	envB := config.Environment{
		Name:    "beta",
		Windows: []config.WindowTemplate{{Name: "shell"}},
	}
	sessionA := tmux.SessionName(envA.Name)
	sessionB := tmux.SessionName(envB.Name)

	ti := textinput.New()
	m := SearchModel{
		query: ti,
		envs:  []config.Environment{envA, envB},
		sessions: map[string]struct{}{
			sessionA: {},
			sessionB: {},
		},
		sessionWindows: map[string][]string{},
		scopeSession:   sessionA,
	}

	results := m.computeResults()

	if len(results) == 0 || !results[0].header || results[0].env != envA.Name {
		t.Fatalf("expected results to start with alpha's session header, got %+v", results)
	}
	for _, r := range results {
		if r.env != envA.Name {
			t.Errorf("scoped results must only include %q, got row from %q", envA.Name, r.env)
		}
	}

	var windows []string
	for _, r := range results {
		if r.header {
			continue
		}
		windows = append(windows, r.window)
	}
	if len(windows) != 2 || windows[0] != "editor" || windows[1] != "logs" {
		t.Errorf("expected only alpha's windows [editor logs], got %v", windows)
	}
}

// TestSearchComputeResultsAliasFirst verifies that windows whose alias (tag)
// matches the query are ranked above windows that match only on name.
func TestSearchComputeResultsAliasFirst(t *testing.T) {
	env := config.Environment{
		Name: "proj",
		Windows: []config.WindowTemplate{
			// "server" matches query "sv" on the NAME (s..v..), no tag.
			{Name: "server"},
			// "editor" matches query "sv" only via its [sv] alias.
			{Name: "editor", Tags: []string{"sv"}},
		},
	}
	session := tmux.SessionName(env.Name)

	ti := textinput.New()
	ti.SetValue("sv")
	m := SearchModel{
		query:          ti,
		envs:           []config.Environment{env},
		sessions:       map[string]struct{}{session: {}},
		sessionWindows: map[string][]string{},
	}

	results := m.computeResults()

	var windows []string
	for _, r := range results {
		if !r.header {
			windows = append(windows, r.window)
		}
	}
	if len(windows) != 2 {
		t.Fatalf("expected both windows to match %q, got %v", "sv", windows)
	}
	if windows[0] != "editor" {
		t.Errorf("alias match should rank first: got %v, want editor before server", windows)
	}
}

// TestSearchComputeResultsNoCrossBoundaryMatch reproduces the reported bug
// where query "lg" matched every window in session "te-liveevents": the old
// combined haystack ("<env> <window> [tags] running up") let the query pull
// "l" from the env name and "g" from the "running" status text. Now env-name
// and window matching are separate and "running up" is dropped, so "lg" must
// match only the window whose name/alias contains it.
func TestSearchComputeResultsNoCrossBoundaryMatch(t *testing.T) {
	env := config.Environment{
		Name: "te-liveevents",
		Windows: []config.WindowTemplate{
			{Name: "lazygit", Tags: []string{"lg"}},
			{Name: "editor", Tags: []string{"ed"}},
			{Name: "terminal", Tags: []string{"tm"}},
			{Name: "henv", Tags: []string{"he"}},
			{Name: "database", Tags: []string{"db"}},
			{Name: "agent", Tags: []string{"ai"}},
		},
	}
	session := tmux.SessionName(env.Name)

	ti := textinput.New()
	ti.SetValue("lg")
	m := SearchModel{
		query:          ti,
		envs:           []config.Environment{env},
		sessions:       map[string]struct{}{session: {}},
		sessionWindows: map[string][]string{},
		scopeSession:   session,
	}

	var windows []string
	for _, r := range m.computeResults() {
		if !r.header {
			windows = append(windows, r.window)
		}
	}
	if len(windows) != 1 || windows[0] != "lazygit" {
		t.Errorf("query %q should match only lazygit, got %v", "lg", windows)
	}
}

// TestSearchMoveCursorNeverLandsOnHeader verifies that moving the cursor up
// from the first window does not park it on the session header (which would
// drop the highlight and make Enter a no-op). It should stay on the first
// window instead.
func TestSearchMoveCursorNeverLandsOnHeader(t *testing.T) {
	results := []searchItem{
		{header: true, env: "proj"},
		{window: "a"},
		{window: "b"},
	}
	m := &SearchModel{results: results, cursor: 1} // on first window

	m.moveCursor(-1) // press up
	if m.results[m.cursor].header {
		t.Errorf("cursor landed on header after up from first window (cursor=%d)", m.cursor)
	}
	if m.cursor != 1 {
		t.Errorf("cursor should stay on first window, got %d", m.cursor)
	}

	m.cursor = 2 // last window
	m.moveCursor(1)
	if m.cursor != 2 {
		t.Errorf("cursor should stay on last window when moving past end, got %d", m.cursor)
	}
}

// TestSearchComputeResultsAppliesStatuses verifies that computeResults wires
// per-window agent statuses (cooking / awaiting) onto matching searchItem rows
// when the parent session is running and the window key is present in
// m.statuses. Unrelated rows must remain Idle.
func TestSearchComputeResultsAppliesStatuses(t *testing.T) {
	env := config.Environment{
		Name: "proj",
		Windows: []config.WindowTemplate{
			{Name: "agent-a", Tags: []string{"ai"}},
			{Name: "agent-b", Tags: []string{"ai"}},
			{Name: "shell", Tags: []string{"shell"}},
		},
	}
	session := tmux.SessionName(env.Name)

	ti := textinput.New()
	m := SearchModel{
		query: ti,
		envs:  []config.Environment{env},
		sessions: map[string]struct{}{
			session: {},
		},
		sessionWindows: map[string][]string{
			session: {"agent-a", "agent-b", "shell"},
		},
		statuses: map[string]AgentStatus{
			windowKey(session, "agent-a"): AgentStatusCooking,
			windowKey(session, "agent-b"): AgentStatusAwaitingInput,
		},
	}

	results := m.computeResults()

	got := map[string]AgentStatus{}
	for _, r := range results {
		if r.header {
			continue
		}
		got[r.window] = r.status
	}

	if got["agent-a"] != AgentStatusCooking {
		t.Errorf("agent-a status = %q, want %q", got["agent-a"], AgentStatusCooking)
	}
	if got["agent-b"] != AgentStatusAwaitingInput {
		t.Errorf("agent-b status = %q, want %q", got["agent-b"], AgentStatusAwaitingInput)
	}
	if got["shell"] != AgentStatusIdle {
		t.Errorf("shell status = %q, want %q (no entry in statuses → idle)",
			got["shell"], AgentStatusIdle)
	}
}

// TestSearchComputeResultsExcludesNonRunningSessions verifies that
// environments without a live tmux session are omitted from the popup
// entirely — only running sessions are listed.
func TestSearchComputeResultsExcludesNonRunningSessions(t *testing.T) {
	env := config.Environment{
		Name: "proj",
		Windows: []config.WindowTemplate{
			{Name: "agent-a", Tags: []string{"ai"}},
		},
	}
	session := tmux.SessionName(env.Name)

	ti := textinput.New()
	m := SearchModel{
		query:          ti,
		envs:           []config.Environment{env},
		sessions:       map[string]struct{}{}, // session NOT running
		sessionWindows: map[string][]string{},
		statuses: map[string]AgentStatus{
			windowKey(session, "agent-a"): AgentStatusCooking,
		},
	}

	results := m.computeResults()
	if len(results) != 0 {
		t.Errorf("non-running session should yield no rows, got %+v", results)
	}
}
