package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/s3r3nity-labs/mangohud-fps-switch/internal/profiles"
)

func setup(t *testing.T, mangohud string) (*App, string, map[string]string) {
	t.Helper()
	home := t.TempDir()
	env := map[string]string{"HOME": home}
	if mangohud != "" {
		path := filepath.Join(home, ".config", "MangoHud", "MangoHud.conf")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(mangohud), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a, err := Load(func(k string) string { return env[k] }, "")
	if err != nil {
		t.Fatal(err)
	}
	return a, home, env
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestFirstLoadCreatesDefaults(t *testing.T) {
	a, home, _ := setup(t, "fps_limit=60\n")
	if !a.Created || a.ConfigPath != filepath.Join(home, ".config", "mangohud-fps-switch", "config.toml") {
		t.Errorf("created %v at %s", a.Created, a.ConfigPath)
	}
	if s, err := a.Status(); err != nil || s.Active != 2 || s.FPSLimit != "60" {
		t.Errorf("status %+v, %v", s, err)
	}
}

func TestSetProfileAndNext(t *testing.T) {
	a, _, _ := setup(t, "fps_limit=60\n")
	next, _ := a.Next(false)
	if next != 3 {
		t.Fatalf("next %d", next)
	}
	if err := a.SetProfile(next); err != nil {
		t.Fatal(err)
	}
	if got := read(t, a.MangoHud); got != "fps_limit=90\n" {
		t.Errorf("MangoHud.conf %q", got)
	}
	if next, _ := a.Next(false); next != 0 {
		t.Errorf("after the last: %d", next)
	}
	if back, _ := a.Next(true); back != 2 {
		t.Errorf("back: %d", back)
	}
	if err := a.SetProfile(9); err == nil {
		t.Error("profile 9 set")
	}
}

func TestMissingMangoHudConfig(t *testing.T) {
	a, _, _ := setup(t, "")
	if err := a.SetFPS("60"); !errors.Is(err, ErrNoMangoHudConfig) {
		t.Errorf("err %v", err)
	}
	if _, err := os.Stat(a.MangoHud); err == nil {
		t.Error("created MangoHud.conf")
	}
}

func TestStatusWarnings(t *testing.T) {
	a, home, env := setup(t, "fps_limit=30\n")
	if err := os.WriteFile(filepath.Join(home, ".config", "MangoHud", "wine-Game.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	env["MANGOHUD_CONFIG"] = "fps_limit=30"

	s, err := a.Status()

	if err != nil || s.Active != -1 || len(s.PerGame) != 1 || !s.EnvOverride {
		t.Errorf("status %+v, %v", s, err)
	}
}

func TestSaveSwitchesTheMangoHudConfig(t *testing.T) {
	a, home, _ := setup(t, "fps_limit=60\n")
	other := filepath.Join(home, "games.conf")
	if err := os.WriteFile(other, []byte("fps_limit=60\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := a.Config
	cfg.MangoHudConfig = "~/games.conf"
	cfg.Profiles = append(cfg.Profiles, profiles.Profile{Name: "30 FPS", FPSLimit: "30"})

	if err := a.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if a.MangoHud != other {
		t.Errorf("MangoHud %s", a.MangoHud)
	}
	if err := a.SetProfile(4); err != nil || read(t, other) != "fps_limit=30\n" {
		t.Errorf("err %v, %q", err, read(t, other))
	}
	if s, _ := a.Status(); s.PerGame != nil {
		t.Error("per-game configs for a chosen file")
	}
	reloaded, _ := Load(func(k string) string { return map[string]string{"HOME": home}[k] }, "")
	if len(reloaded.Config.Profiles) != 5 {
		t.Errorf("not saved: %d profiles", len(reloaded.Config.Profiles))
	}
}
