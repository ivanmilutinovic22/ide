package config

import (
	"reflect"
	"testing"
)

// TestNormalizeWindowsLiftsNameTags covers the legacy-config migration:
// older configs (and the old defaults) embedded tags in the window name,
// e.g. "ai-assistant [ai]". normalizeWindows must lift them into Tags so
// AI detection works without the user re-editing every window.
func TestNormalizeWindowsLiftsNameTags(t *testing.T) {
	in := []WindowTemplate{
		{Name: "ai-assistant [ai]", Cmd: "opencode"},
		{Name: "agent [ai] [db]", Cmd: "claude"},
		{Name: "tagged [ai]", Tags: []string{"AI"}}, // already present (case-insensitive) — no duplicate
		{Name: "shell"},
	}
	got := normalizeWindows(in)
	want := []WindowTemplate{
		{Name: "ai-assistant", Cmd: "opencode", Tags: []string{"ai"}},
		{Name: "agent", Cmd: "claude", Tags: []string{"ai", "db"}},
		{Name: "tagged", Tags: []string{"AI"}},
		{Name: "shell"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("normalizeWindows mismatch:\ngot:  %+v\nwant: %+v", got, want)
	}
}

func TestNormalizeWindowsNameOnlyTagsGetsFallbackName(t *testing.T) {
	got := normalizeWindows([]WindowTemplate{{Name: "[ai]"}})
	if len(got) != 1 || got[0].Name != "window-1" || !reflect.DeepEqual(got[0].Tags, []string{"ai"}) {
		t.Errorf("unexpected result: %+v", got)
	}
}
