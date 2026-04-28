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

// TestSearchComputeResultsStatusIdleWhenSessionNotRunning verifies that even
// if the statuses map contains a key, computeResults marks the row Idle when
// the parent session is not running (defensive: stale status shouldn't bleed
// into a closed session row).
func TestSearchComputeResultsStatusIdleWhenSessionNotRunning(t *testing.T) {
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
	for _, r := range results {
		if r.header {
			continue
		}
		if r.status != AgentStatusIdle {
			t.Errorf("window %q in non-running session: status = %q, want idle",
				r.window, r.status)
		}
	}
}
