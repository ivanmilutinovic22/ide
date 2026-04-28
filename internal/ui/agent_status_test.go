package ui

import (
	"testing"

	"ide/internal/config"
)

func TestIsAIToolProcess(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"known claude", "claude", true},
		{"known opencode", "opencode", true},
		{"known codex", "codex", true},
		{"known aider", "aider", true},
		{"mixed case", "Claude", true},
		{"upper", "OPENCODE", true},
		{"with leading path", "/usr/local/bin/claude", true},
		{"with arg suffix", "claude --resume", true},
		{"with tab arg", "aider\t--model gpt-4", true},
		{"unknown name", "vim", false},
		{"unknown shell", "bash", false},
		{"empty string", "", false},
		{"whitespace only", "   ", false},
		{"close but wrong", "claudia", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isAIToolProcess(tc.in)
			if got != tc.want {
				t.Errorf("isAIToolProcess(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestModelIsAIWindow(t *testing.T) {
	env := config.Environment{
		Name: "demo",
		Windows: []config.WindowTemplate{
			{Name: "agent", Tags: []string{"ai"}},
			{Name: "shell"},
			{Name: "logs", Tags: []string{"db"}},
		},
	}
	m := Model{}
	tests := []struct {
		name    string
		window  string
		process string
		want    bool
	}{
		{"tag only", "agent", "", true},
		{"tag with non-AI process", "agent", "bash", true},
		{"non-tagged window with AI process", "shell", "claude", true},
		{"non-tagged window with non-AI process", "shell", "vim", false},
		{"non-tagged window with empty process", "shell", "", false},
		{"non-tagged with path-prefixed AI process", "logs", "/usr/local/bin/aider", true},
		{"unknown window with AI process", "ghost", "codex", true},
		{"unknown window with non-AI process", "ghost", "less", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.isAIWindow(env, tc.window, tc.process)
			if got != tc.want {
				t.Errorf("isAIWindow(%q, %q) = %v, want %v", tc.window, tc.process, got, tc.want)
			}
		})
	}
}

// TestWindowKey just pins the "session:window" format. Other code parses
// this format apart, so it's load-bearing despite being a one-liner.
func TestWindowKey(t *testing.T) {
	tests := []struct {
		session string
		window  string
		want    string
	}{
		{"ide-prod", "shell", "ide-prod:shell"},
		{"", "", ":"},
		{"ide-x", "", "ide-x:"},
		{"", "w", ":w"},
		{"a:b", "c", "a:b:c"}, // documents that nothing escapes colons
	}
	for _, tc := range tests {
		got := windowKey(tc.session, tc.window)
		if got != tc.want {
			t.Errorf("windowKey(%q, %q) = %q, want %q", tc.session, tc.window, got, tc.want)
		}
	}
}

// TestHasTag verifies the case-insensitive tag membership check.
func TestHasTag(t *testing.T) {
	mk := func(tags ...string) config.WindowTemplate {
		return config.WindowTemplate{Name: "w", Tags: tags}
	}
	tests := []struct {
		name string
		w    config.WindowTemplate
		tag  string
		want bool
	}{
		{"present exact case", mk("ai", "db"), "ai", true},
		{"present uppercase tag, lowercase query", mk("AI", "DB"), "ai", true},
		{"present lowercase tag, uppercase query", mk("ai"), "AI", true},
		{"absent tag", mk("ai", "db"), "ssh", false},
		{"empty tags slice", mk(), "ai", false},
		// CHARACTERIZATION: HasTag uses strings.EqualFold, so an empty
		// query matches an empty tag in the slice (since "" == "" under
		// EqualFold). It does NOT spuriously match non-empty tags.
		{"empty query, empty tag in list", config.WindowTemplate{Tags: []string{""}}, "", true},
		{"empty query, no empty tag in list", mk("ai"), "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := HasTag(tc.w, tc.tag)
			if got != tc.want {
				t.Errorf("HasTag(%v, %q) = %v, want %v", tc.w.Tags, tc.tag, got, tc.want)
			}
		})
	}
}
