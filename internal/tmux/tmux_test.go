package tmux

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestSessionName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple lowercase", "prod", "ide-prod"},
		{"spaces become dashes after lowercase", "My App", "ide-my-app"},
		// CHARACTERIZATION: a whitespace-only env name produces "ide" (no
		// trailing dash). Every other input gets the "ide-" prefix. Callers
		// that build display strings from SessionName should be aware of
		// this irregular case.
		{"whitespace-only returns bare ide", "  ", "ide"},
		{"empty returns bare ide", "", "ide"},
		{"uppercase mixed gets lowercased and dashed", "MyAPP NAME", "ide-myapp-name"},
		{"already prefixed gets prefix again", "ide-prod", "ide-ide-prod"},
		{"surrounding whitespace trimmed", "  prod  ", "ide-prod"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SessionName(tc.in)
			if got != tc.want {
				t.Errorf("SessionName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSafeWindowName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty returns shell", "", "shell"},
		{"whitespace-only returns shell", "   ", "shell"},
		{"single space replaced", "my window", "my-window"},
		{"multiple spaces each replaced", "a b c d", "a-b-c-d"},
		{"already-fine name unchanged", "already-fine", "already-fine"},
		// CHARACTERIZATION: SafeWindowName does NOT lowercase, unlike
		// SessionName. Mixed-case names are preserved.
		{"mixed-case preserved", "MyWindow", "MyWindow"},
		{"surrounding whitespace trimmed", "  shell  ", "shell"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SafeWindowName(tc.in)
			if got != tc.want {
				t.Errorf("SafeWindowName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSplitNonEmptyLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty string", "", []string{}},
		{"whitespace-only string", "  \n  \n", []string{}},
		{"single line", "alpha", []string{"alpha"}},
		{"trailing newline", "alpha\n", []string{"alpha"}},
		{"two lines", "alpha\nbeta", []string{"alpha", "beta"}},
		{"embedded blank lines dropped", "alpha\n\nbeta\n\n", []string{"alpha", "beta"}},
		{"per-line whitespace trimmed", "  alpha  \n\tbeta\t", []string{"alpha", "beta"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitNonEmptyLines(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("splitNonEmptyLines(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveCwd(t *testing.T) {
	tests := []struct {
		name     string
		root     string
		override string
		want     string
	}{
		{"empty override returns root", "/repos/x", "", "/repos/x"},
		{"absolute override wins, root ignored", "/repos/x", "/other/path", "/other/path"},
		{"relative override joined with root", "/repos/x", "sub/dir", filepath.Join("/repos/x", "sub/dir")},
		{"both empty", "", "", ""},
		{"empty root, relative override returned as-is", "", "sub/dir", "sub/dir"},
		{"whitespace-only root treated as empty, relative override returned", "  ", "sub", "sub"},
		{"whitespace trimmed around override", "/repos/x", "  sub  ", filepath.Join("/repos/x", "sub")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveCwd(tc.root, tc.override)
			if got != tc.want {
				t.Errorf("resolveCwd(%q, %q) = %q, want %q", tc.root, tc.override, got, tc.want)
			}
		})
	}
}
