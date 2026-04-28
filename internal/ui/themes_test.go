package ui

import (
	"strings"
	"testing"
)

func TestNormalizeTheme(t *testing.T) {
	t.Run("all-empty falls back to defaults and chains", func(t *testing.T) {
		got := normalizeTheme(uiTheme{})
		if got.AppBG != defaultThemeAppBG {
			t.Errorf("AppBG = %q, want default %q", got.AppBG, defaultThemeAppBG)
		}
		if got.AppFG != defaultThemeAppFG {
			t.Errorf("AppFG = %q, want default %q", got.AppFG, defaultThemeAppFG)
		}
		// Chained defaults: PaneBG <- AppBG; Border <- PaneBG; SelectedFG <- AppFG;
		// Accent <- SelectedFG; SelectedBG <- Border; Muted <- AppFG; Active <- Accent;
		// Inactive <- Muted; Status <- AppFG.
		if got.PaneBG != defaultThemeAppBG {
			t.Errorf("PaneBG should chain from AppBG, got %q", got.PaneBG)
		}
		if got.Border != defaultThemeAppBG {
			t.Errorf("Border should chain from PaneBG, got %q", got.Border)
		}
		if got.SelectedFG != defaultThemeAppFG {
			t.Errorf("SelectedFG should chain from AppFG, got %q", got.SelectedFG)
		}
		if got.Accent != defaultThemeAppFG {
			t.Errorf("Accent should chain from SelectedFG, got %q", got.Accent)
		}
		if got.SelectedBG != defaultThemeAppBG {
			t.Errorf("SelectedBG should chain from Border, got %q", got.SelectedBG)
		}
		if got.Muted != defaultThemeAppFG {
			t.Errorf("Muted should chain from AppFG, got %q", got.Muted)
		}
		if got.Active != defaultThemeAppFG {
			t.Errorf("Active should chain from Accent, got %q", got.Active)
		}
		if got.Inactive != defaultThemeAppFG {
			t.Errorf("Inactive should chain from Muted, got %q", got.Inactive)
		}
		if got.Status != defaultThemeAppFG {
			t.Errorf("Status should chain from AppFG, got %q", got.Status)
		}
	})

	t.Run("filled theme is left alone", func(t *testing.T) {
		in := uiTheme{
			Name:       "Custom",
			AppBG:      "#000001",
			AppFG:      "#fffffe",
			PaneBG:     "#000002",
			Border:     "#000003",
			SelectedFG: "#000004",
			SelectedBG: "#000005",
			Active:     "#000006",
			Inactive:   "#000007",
			Accent:     "#000008",
			Muted:      "#000009",
			Status:     "#00000a",
		}
		got := normalizeTheme(in)
		if got != in {
			t.Errorf("filled theme was modified: got %+v want %+v", got, in)
		}
	})

	t.Run("whitespace trimmed before defaulting", func(t *testing.T) {
		in := uiTheme{
			Name:  "  spacey  ",
			AppBG: "  #abcdef  ",
		}
		got := normalizeTheme(in)
		if got.Name != "spacey" {
			t.Errorf("Name = %q, want \"spacey\"", got.Name)
		}
		if got.AppBG != "#abcdef" {
			t.Errorf("AppBG = %q, want \"#abcdef\"", got.AppBG)
		}
		// AppFG was empty, should default
		if got.AppFG != defaultThemeAppFG {
			t.Errorf("AppFG = %q, want default", got.AppFG)
		}
	})

	t.Run("partially filled values are preserved while empties chain", func(t *testing.T) {
		in := uiTheme{AppBG: "#111111", Accent: "#aaaaaa"}
		got := normalizeTheme(in)
		if got.AppBG != "#111111" {
			t.Errorf("AppBG should be preserved, got %q", got.AppBG)
		}
		if got.Accent != "#aaaaaa" {
			t.Errorf("Accent should be preserved, got %q", got.Accent)
		}
		// SelectedFG empty -> AppFG (which itself defaulted)
		if got.SelectedFG != defaultThemeAppFG {
			t.Errorf("SelectedFG should chain, got %q", got.SelectedFG)
		}
		// PaneBG should chain from filled AppBG
		if got.PaneBG != "#111111" {
			t.Errorf("PaneBG should chain from AppBG, got %q", got.PaneBG)
		}
	})
}

func TestThemeIndexByName(t *testing.T) {
	m := Model{themes: defaultThemes()}
	if len(m.themes) == 0 {
		t.Fatal("defaultThemes() returned empty list")
	}

	first := m.themes[0].Name
	last := m.themes[len(m.themes)-1].Name

	t.Run("known name", func(t *testing.T) {
		idx, ok := m.themeIndexByName(first)
		if !ok || idx != 0 {
			t.Errorf("lookup of %q: idx=%d ok=%v, want idx=0 ok=true", first, idx, ok)
		}
	})

	t.Run("known last name", func(t *testing.T) {
		idx, ok := m.themeIndexByName(last)
		if !ok || idx != len(m.themes)-1 {
			t.Errorf("lookup of %q: idx=%d ok=%v, want idx=%d ok=true",
				last, idx, ok, len(m.themes)-1)
		}
	})

	t.Run("unknown name returns false", func(t *testing.T) {
		_, ok := m.themeIndexByName("definitely-not-a-real-theme-name-zzz")
		if ok {
			t.Errorf("expected ok=false for unknown name")
		}
	})

	t.Run("case-insensitive lookup", func(t *testing.T) {
		// EqualFold under the hood. Try both upper and lower variants.
		mixedUpper := strings.ToUpper(first)
		idx, ok := m.themeIndexByName(mixedUpper)
		if !ok || idx != 0 {
			t.Errorf("uppercase lookup of %q: idx=%d ok=%v, want idx=0 ok=true", mixedUpper, idx, ok)
		}
		mixedLower := strings.ToLower(first)
		idx, ok = m.themeIndexByName(mixedLower)
		if !ok || idx != 0 {
			t.Errorf("lowercase lookup of %q: idx=%d ok=%v, want idx=0 ok=true", mixedLower, idx, ok)
		}
	})

	t.Run("whitespace-padded lookup", func(t *testing.T) {
		idx, ok := m.themeIndexByName("  " + first + "  ")
		if !ok || idx != 0 {
			t.Errorf("padded lookup: idx=%d ok=%v, want idx=0 ok=true", idx, ok)
		}
	})

	t.Run("empty name returns false", func(t *testing.T) {
		_, ok := m.themeIndexByName("")
		if ok {
			t.Errorf("expected ok=false for empty name")
		}
	})
}
