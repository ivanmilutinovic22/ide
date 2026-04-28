package theme

import "testing"

func TestNormalize(t *testing.T) {
	t.Run("all-empty falls back to defaults and chains", func(t *testing.T) {
		got := Normalize(Theme{})
		if got.AppBG != DefaultAppBG {
			t.Errorf("AppBG = %q, want default %q", got.AppBG, DefaultAppBG)
		}
		if got.AppFG != DefaultAppFG {
			t.Errorf("AppFG = %q, want default %q", got.AppFG, DefaultAppFG)
		}
		if got.PaneBG != DefaultAppBG {
			t.Errorf("PaneBG should chain from AppBG, got %q", got.PaneBG)
		}
		if got.Border != DefaultAppBG {
			t.Errorf("Border should chain from PaneBG, got %q", got.Border)
		}
		if got.SelectedFG != DefaultAppFG {
			t.Errorf("SelectedFG should chain from AppFG, got %q", got.SelectedFG)
		}
		if got.Accent != DefaultAppFG {
			t.Errorf("Accent should chain from SelectedFG, got %q", got.Accent)
		}
		if got.SelectedBG != DefaultAppBG {
			t.Errorf("SelectedBG should chain from Border, got %q", got.SelectedBG)
		}
		if got.Muted != DefaultAppFG {
			t.Errorf("Muted should chain from AppFG, got %q", got.Muted)
		}
		if got.Active != DefaultAppFG {
			t.Errorf("Active should chain from Accent, got %q", got.Active)
		}
		if got.Inactive != DefaultAppFG {
			t.Errorf("Inactive should chain from Muted, got %q", got.Inactive)
		}
		if got.Status != DefaultAppFG {
			t.Errorf("Status should chain from AppFG, got %q", got.Status)
		}
	})

	t.Run("filled theme is left alone", func(t *testing.T) {
		in := Theme{
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
		got := Normalize(in)
		if got != in {
			t.Errorf("filled theme was modified: got %+v want %+v", got, in)
		}
	})

	t.Run("whitespace trimmed before defaulting", func(t *testing.T) {
		in := Theme{Name: "  spacey  ", AppBG: "  #abcdef  "}
		got := Normalize(in)
		if got.Name != "spacey" {
			t.Errorf("Name = %q, want \"spacey\"", got.Name)
		}
		if got.AppBG != "#abcdef" {
			t.Errorf("AppBG = %q, want \"#abcdef\"", got.AppBG)
		}
		if got.AppFG != DefaultAppFG {
			t.Errorf("AppFG = %q, want default", got.AppFG)
		}
	})

	t.Run("partially filled values are preserved while empties chain", func(t *testing.T) {
		in := Theme{AppBG: "#111111", Accent: "#aaaaaa"}
		got := Normalize(in)
		if got.AppBG != "#111111" {
			t.Errorf("AppBG should be preserved, got %q", got.AppBG)
		}
		if got.Accent != "#aaaaaa" {
			t.Errorf("Accent should be preserved, got %q", got.Accent)
		}
		if got.SelectedFG != DefaultAppFG {
			t.Errorf("SelectedFG should chain, got %q", got.SelectedFG)
		}
		if got.PaneBG != "#111111" {
			t.Errorf("PaneBG should chain from AppBG, got %q", got.PaneBG)
		}
	})
}

func TestDefaultsNotEmpty(t *testing.T) {
	if len(Defaults()) == 0 {
		t.Fatal("Defaults() returned empty list")
	}
}
