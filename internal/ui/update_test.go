package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ide/internal/config"
)

func TestClampSelection(t *testing.T) {
	tests := []struct {
		name    string
		current int
		delta   int
		count   int
		want    int
	}{
		{"empty count", 0, 0, 0, 0},
		{"negative count returns zero", 3, 1, -1, 0},
		{"underflow clamps to 0", 0, -1, 5, 0},
		{"overflow clamps to count-1", 0, 10, 5, 4},
		{"normal forward step", 2, 1, 5, 3},
		{"normal backward step", 2, -1, 5, 1},
		{"no movement at last index", 4, 0, 5, 4},
		{"zero delta from middle", 2, 0, 5, 2},
		{"negative current still works (delta corrects)", -1, 1, 5, 0},
		{"large negative delta clamps to 0", 2, -100, 5, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := clampSelection(tc.current, tc.delta, tc.count)
			if got != tc.want {
				t.Errorf("clampSelection(%d, %d, %d) = %d, want %d",
					tc.current, tc.delta, tc.count, got, tc.want)
			}
		})
	}
}

func TestNormalizeRootPath(t *testing.T) {
	home, homeErr := os.UserHomeDir()

	t.Run("empty input returns empty", func(t *testing.T) {
		if got := normalizeRootPath(""); got != "" {
			t.Errorf("normalizeRootPath(\"\") = %q, want \"\"", got)
		}
	})

	t.Run("whitespace trimmed", func(t *testing.T) {
		// "   " trims to "", returns ""
		if got := normalizeRootPath("   "); got != "" {
			t.Errorf("normalizeRootPath(\"   \") = %q, want \"\"", got)
		}
	})

	t.Run("absolute path is cleaned", func(t *testing.T) {
		got := normalizeRootPath("/foo/bar/../baz")
		want := filepath.Clean("/foo/bar/../baz")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("absolute path unchanged when already clean", func(t *testing.T) {
		got := normalizeRootPath("/usr/local/bin")
		if got != "/usr/local/bin" {
			t.Errorf("got %q, want \"/usr/local/bin\"", got)
		}
	})

	if homeErr == nil {
		t.Run("tilde expansion", func(t *testing.T) {
			got := normalizeRootPath("~/foo")
			want := filepath.Clean(filepath.Join(home, "foo"))
			if got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})

		t.Run("$HOME expansion", func(t *testing.T) {
			got := normalizeRootPath("$HOME/x")
			want := filepath.Clean(filepath.Join(home, "x"))
			if got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}

	t.Run("surrounding whitespace trimmed", func(t *testing.T) {
		got := normalizeRootPath("  /tmp/x  ")
		if got != "/tmp/x" {
			t.Errorf("got %q, want \"/tmp/x\"", got)
		}
	})

	t.Run("custom env var expanded", func(t *testing.T) {
		t.Setenv("UI_TEST_VAR", "/expanded")
		got := normalizeRootPath("$UI_TEST_VAR/sub")
		want := filepath.Clean("/expanded/sub")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// TestParseWindowEntry exercises the single-entry parser. parseWindowSpec
// just splits and delegates, so the entry-level tests cover most behavior.
func TestParseWindowEntry(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    config.WindowTemplate
		wantErr bool
	}{
		{
			name:  "name only",
			input: "shell",
			want:  config.WindowTemplate{Name: "shell"},
		},
		{
			name:  "name and command via equals",
			input: "build=npm run build",
			want:  config.WindowTemplate{Name: "build", Cmd: "npm run build"},
		},
		{
			name:  "name, command, and cwd via equals + pipe",
			input: "build=npm run build|./web",
			want:  config.WindowTemplate{Name: "build", Cmd: "npm run build", Cwd: "./web"},
		},
		{
			name:  "name and command via pipe",
			input: "shell|zsh",
			want:  config.WindowTemplate{Name: "shell", Cmd: "zsh"},
		},
		{
			name:  "name, command, cwd via pipes",
			input: "shell|zsh|./api",
			want:  config.WindowTemplate{Name: "shell", Cmd: "zsh", Cwd: "./api"},
		},
		{
			name:  "tags extracted",
			input: "agent [ai] [db]=claude",
			want:  config.WindowTemplate{Name: "agent", Cmd: "claude", Tags: []string{"ai", "db"}},
		},
		{
			name:    "empty entry errors",
			input:   "",
			wantErr: true,
		},
		{
			name:    "empty name in equals form errors",
			input:   "=cmd",
			wantErr: true,
		},
		{
			name:    "too many pipe parts errors",
			input:   "a|b|c|d",
			wantErr: true,
		},
		{
			// Regression: the equals form silently swallowed extra pipes by
			// stuffing everything after the first pipe into Cwd, despite
			// the pipe-only form rejecting >3 parts. That asymmetry meant a
			// typo like "name=cmd|sub|rest" produced Cwd="sub|rest" with no
			// warning.
			name:    "too many pipe parts after equals errors",
			input:   "name=cmd|cwd|extra",
			wantErr: true,
		},
		{
			name:  "whitespace trimmed around segments",
			input: "  shell  =  zsh  |  ./x  ",
			want:  config.WindowTemplate{Name: "shell", Cmd: "zsh", Cwd: "./x"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseWindowEntry(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result=%+v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Name != tc.want.Name || got.Cmd != tc.want.Cmd || got.Cwd != tc.want.Cwd {
				t.Errorf("name/cmd/cwd mismatch: got %+v want %+v", got, tc.want)
			}
			if !reflect.DeepEqual(got.Tags, tc.want.Tags) {
				t.Errorf("tags mismatch: got %#v want %#v", got.Tags, tc.want.Tags)
			}
		})
	}
}

func TestParseWindowSpec(t *testing.T) {
	t.Run("comma-separated entries", func(t *testing.T) {
		got, err := parseWindowSpec("a, b=cmd, c|sh")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(got))
		}
		if got[0].Name != "a" || got[1].Name != "b" || got[1].Cmd != "cmd" || got[2].Name != "c" || got[2].Cmd != "sh" {
			t.Errorf("unexpected parse: %+v", got)
		}
	})
	t.Run("semicolon takes precedence when present", func(t *testing.T) {
		// splitWindowEntries switches to ';' when the spec contains one.
		got, err := parseWindowSpec("a, b; c")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 entries (split on ';'), got %d: %+v", len(got), got)
		}
	})
	t.Run("empty spec errors", func(t *testing.T) {
		if _, err := parseWindowSpec(""); err == nil {
			t.Errorf("expected error for empty spec")
		}
	})
	t.Run("whitespace-only spec errors", func(t *testing.T) {
		if _, err := parseWindowSpec("   "); err == nil {
			t.Errorf("expected error for whitespace-only spec")
		}
	})
}

// TestShortcutsListAmbiguousKeyResolvesByCursor sanity-checks that the
// ambiguity targeted by updateShortcutsMode's cursor-first dispatch is real:
// the help overlay has two rows keyed "a" — one for create-environment in the
// Sessions section and one for create-template in the Templates section.
// Without cursor-aware dispatch, pressing "a" while focused on the templates
// row would still fire create-environment because the list scan returns the
// first match.
func TestShortcutsListAmbiguousKeyResolvesByCursor(t *testing.T) {
	items := shortcutsList()
	envIndex, tmplIndex := -1, -1
	for i, it := range items {
		if it.isHeader {
			continue
		}
		if it.key == "a" && it.action == "create" {
			envIndex = i
		}
		if it.key == "a" && it.action == "create-template" {
			tmplIndex = i
		}
	}
	if envIndex < 0 || tmplIndex < 0 {
		t.Fatalf("expected both 'a' rows in shortcutsList; got envIndex=%d tmplIndex=%d", envIndex, tmplIndex)
	}
	if tmplIndex <= envIndex {
		t.Fatalf("expected templates 'a' row to come after env 'a' row; got envIndex=%d tmplIndex=%d", envIndex, tmplIndex)
	}
}

// TestShortcutsListTemplatesEnter ensures the Templates section advertises
// "enter" as an alias for edit, matching the actual updateTemplatesPanelKey
// behavior (case "e", "enter": startEditTemplateMode).
func TestShortcutsListTemplatesEnter(t *testing.T) {
	items := shortcutsList()
	inTemplates := false
	for _, it := range items {
		if it.isHeader {
			inTemplates = it.desc == "Templates"
			continue
		}
		if !inTemplates {
			continue
		}
		if it.desc == "edit template" {
			for _, k := range strings.Split(it.key, "/") {
				if k == "enter" {
					return
				}
			}
			t.Fatalf("templates 'edit template' row should advertise 'enter'; got key %q", it.key)
		}
	}
	t.Fatalf("did not find templates 'edit template' row in shortcutsList")
}

// TestUpdatePathSuggestionsOnlySuggestsDirectories guards the rule that the
// env "Root" field's autocomplete must surface directories only — files are
// invalid roots and offering them is misleading.
func TestUpdatePathSuggestionsOnlySuggestsDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plain.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("writefile: %v", err)
	}

	m := &Model{}
	ti := newTextInput("> ", "")
	ti.SetValue(dir + "/")
	m.updatePathSuggestions(&ti)

	got := ti.AvailableSuggestions()
	for _, s := range got {
		if !strings.HasSuffix(s, "/") {
			t.Errorf("suggestion %q should end with '/' (dirs only), but it does not", s)
		}
		base := filepath.Base(strings.TrimSuffix(s, "/"))
		if base == "plain.txt" {
			t.Errorf("plain.txt should not appear in suggestions: %v", got)
		}
	}
	// Sanity: subdir is among the suggestions.
	found := false
	for _, s := range got {
		if filepath.Base(strings.TrimSuffix(s, "/")) == "subdir" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'subdir' to be suggested, got %v", got)
	}
}
