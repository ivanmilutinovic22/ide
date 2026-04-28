package agentstatus

import "testing"

func TestIsAITool(t *testing.T) {
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
			got := IsAITool(tc.in)
			if got != tc.want {
				t.Errorf("IsAITool(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestKey pins the "session:window" format. Other code parses
// this format apart, so it's load-bearing despite being a one-liner.
func TestKey(t *testing.T) {
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
		got := Key(tc.session, tc.window)
		if got != tc.want {
			t.Errorf("Key(%q, %q) = %q, want %q", tc.session, tc.window, got, tc.want)
		}
	}
}
