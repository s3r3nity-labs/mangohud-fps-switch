package profiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func load(t *testing.T, content string) (Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(path)
	return cfg, err
}

func TestFirstRunWritesTheDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mangohud-fps-switch", "config.toml")

	cfg, created, err := Load(path)

	if err != nil || !created {
		t.Fatalf("created %v, err %v", created, err)
	}
	var names []string
	for _, p := range cfg.Profiles {
		names = append(names, p.Name+"="+string(p.FPSLimit))
	}
	if got := strings.Join(names, " "); got != "Unlimited=0 45 FPS=45 60 FPS=60 90 FPS=90" {
		t.Errorf("profiles %s", got)
	}
	if !cfg.ShouldNotify() {
		t.Error("notify off")
	}
	if _, created, _ := Load(path); created {
		t.Error("written again")
	}
}

func TestProfilesWithSettings(t *testing.T) {
	cfg, err := load(t, `
notify = false
mangohud_config = "~/games.conf"

[[profile]]
name = "Battery"
fps_limit = 40
settings = { fps_limit_method = "early", show_fps_limit = true, gpu_stats = false, font_size = 20 }

[[profile]]
name = "Toggle"
fps_limit = "60, 90, 0"

[[profile]]
name = "NTSC"
fps_limit = 59.94
`)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ShouldNotify() || cfg.MangoHudConfig != "~/games.conf" {
		t.Errorf("cfg %+v", cfg)
	}
	got := cfg.Profiles[0].MangoHudSettings()
	want := map[string]string{"fps_limit": "40", "fps_limit_method": "early", "show_fps_limit": "1",
		"gpu_stats": "0", "font_size": "20"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if cfg.Profiles[1].FPSLimit != "60,90,0" || cfg.Profiles[2].FPSLimit != "59.94" {
		t.Errorf("fps %q %q", cfg.Profiles[1].FPSLimit, cfg.Profiles[2].FPSLimit)
	}
}

func TestInvalidConfigs(t *testing.T) {
	for name, content := range map[string]string{
		"no profiles":    `notify = true`,
		"no name":        "[[profile]]\nfps_limit = 60",
		"no fps":         "[[profile]]\nname = \"a\"",
		"bad fps":        "[[profile]]\nname = \"a\"\nfps_limit = \"fast\"",
		"negative fps":   "[[profile]]\nname = \"a\"\nfps_limit = -5",
		"same name":      "[[profile]]\nname = \"A\"\nfps_limit = 1\n[[profile]]\nname = \"a\"\nfps_limit = 2",
		"fps in setting": "[[profile]]\nname = \"a\"\nfps_limit = 1\nsettings = { fps_limit = 2 }",
		"bad key":        "[[profile]]\nname = \"a\"\nfps_limit = 1\nsettings = { \"Font Size\" = 2 }",
		"comment value":  "[[profile]]\nname = \"a\"\nfps_limit = 1\nsettings = { position = \"x # y\" }",
		"list value":     "[[profile]]\nname = \"a\"\nfps_limit = 1\nsettings = { position = [1, 2] }",
		"not toml":       "[[profile",
	} {
		if _, err := load(t, content); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestParseFPS(t *testing.T) {
	for in, want := range map[string]string{"60": "60", "060": "60", "0": "0", "59.940": "59.94",
		"60, 90,0": "60,90,0", "60.0": "60"} {
		if got, err := ParseFPS(in); err != nil || string(got) != want {
			t.Errorf("%q: got %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "-1", "abc", "60,", "60/90", "1e3"} {
		if _, err := ParseFPS(in); err == nil {
			t.Errorf("%q accepted", in)
		}
	}
}

func TestLabel(t *testing.T) {
	for in, want := range map[FPS]string{"0": "Unlimited", "60": "60 FPS", "60,90,0": "60, 90, unlimited FPS"} {
		if got := in.Label(); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
}

func TestFindByNumberOrName(t *testing.T) {
	cfg := Config{Profiles: []Profile{{Name: "Unlimited"}, {Name: "Battery"}, {Name: "60"}}}
	for arg, want := range map[string]int{"0": 0, "1": 1, "battery": 1, "BATTERY": 1, "60": 2} {
		if got, err := cfg.Find(arg); err != nil || got != want {
			t.Errorf("%q: %d %v", arg, got, err)
		}
	}
	if _, err := cfg.Find("3"); err == nil {
		t.Error("3 found")
	}
}

func TestActive(t *testing.T) {
	cfg := Config{Profiles: []Profile{
		{Name: "Unlimited", FPSLimit: "0"},
		{Name: "Battery", FPSLimit: "45", Settings: map[string]any{"fps_limit_method": "early"}},
		{Name: "45", FPSLimit: "45"},
	}}
	for _, c := range []struct {
		current map[string]string
		want    int
	}{
		{map[string]string{}, 0}, // no fps_limit: unlimited
		{map[string]string{"fps_limit": "45.0", "fps_limit_method": "early"}, 1},
		{map[string]string{"fps_limit": "45"}, 2},
		{map[string]string{"fps_limit": "30"}, -1},
	} {
		if got := cfg.Active(c.current); got != c.want {
			t.Errorf("%v: %d, want %d", c.current, got, c.want)
		}
	}
}
