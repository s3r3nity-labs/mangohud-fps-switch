package profiles

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSaveRoundTrip(t *testing.T) {
	off := false
	cfg := Config{
		Notify:         &off,
		MangoHudConfig: "~/games/mango.conf",
		Profiles: []Profile{
			{Name: `Quiet "night" \ mode`, FPSLimit: "40",
				Settings: map[string]any{"fps_limit_method": "early", "show_fps_limit": true,
					"font_size": int64(20), "font_scale": float64(1), "gpu_stats": false}},
			{Name: "Toggle ⚡", FPSLimit: "60,90,0"},
			{Name: "NTSC", FPSLimit: "59.94"},
		},
	}
	path := filepath.Join(t.TempDir(), "config.toml")

	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, created, err := Load(path)

	if err != nil || created {
		t.Fatalf("created %v, err %v", created, err)
	}
	if !reflect.DeepEqual(got, cfg) {
		t.Errorf("got  %+v\nwant %+v", got, cfg)
	}
	data, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(data), "# mangohud-fps-switch profiles") {
		t.Error("header missing")
	}
}

func TestSaveRejectsInvalidAndKeepsTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := Save(path, DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	bad := DefaultConfig()
	bad.Profiles[1].Name = "unlimited" // same as profile 0, case doesn't matter

	if err := Save(path, bad); err == nil {
		t.Error("saved two profiles with the same name")
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Error("file changed")
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".config-*")); len(left) != 0 {
		t.Errorf("temp files left: %v", left)
	}
}

func TestSaveKeepsALinkedConfig(t *testing.T) {
	dotfiles := filepath.Join(t.TempDir(), "config.toml")
	if err := Save(dotfiles, DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "config.toml")
	if err := os.Symlink(dotfiles, link); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.Profiles = cfg.Profiles[:1]

	if err := Save(link, cfg); err != nil {
		t.Fatal(err)
	}

	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Error("link replaced")
	}
	if got, _, _ := Load(dotfiles); len(got.Profiles) != 1 {
		t.Errorf("target not written: %d profiles", len(got.Profiles))
	}
}

func TestDefaultFileLoadsAsDefaultConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	got, created, err := Load(path)
	if err != nil || !created || !reflect.DeepEqual(got, DefaultConfig()) {
		t.Errorf("got %+v, created %v, err %v", got, created, err)
	}
}
