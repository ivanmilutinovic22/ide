package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestParseUnknownFlagSurfacesName(t *testing.T) {
	fs := newFlagSet("test")
	fs.string("root", "filesystem root")
	err := fs.parse([]string{"--rooot", "/x"})
	if err == nil {
		t.Fatal("expected parse error for unknown flag")
	}
	if !strings.Contains(err.Error(), "rooot") {
		t.Errorf("error %q does not mention the offending flag name", err)
	}
	var pe *parseError
	if !errors.As(err, &pe) {
		t.Errorf("expected *parseError, got %T", err)
	}
}

func TestParseMissingValueSurfacesError(t *testing.T) {
	fs := newFlagSet("test")
	fs.string("root", "filesystem root")
	err := fs.parse([]string{"--root"})
	if err == nil {
		t.Fatal("expected parse error for flag missing value")
	}
	if !strings.Contains(err.Error(), "root") {
		t.Errorf("error %q does not mention the offending flag name", err)
	}
}

func TestParseSuccess(t *testing.T) {
	fs := newFlagSet("test")
	root := fs.string("root", "filesystem root")
	if err := fs.parse([]string{"--root", "/x", "envname"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *root != "/x" {
		t.Errorf("root = %q, want /x", *root)
	}
	if got := fs.positional(); len(got) != 1 || got[0] != "envname" {
		t.Errorf("positionals = %v, want [envname]", got)
	}
}
