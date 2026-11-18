package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newService gives a service on a temporary home with a MangoHud config (if mangohud isn't
// empty) and no config.toml yet.
func newService(t *testing.T, mangohud string) (*ProfileService, string) {
	t.Helper()
	home := t.TempDir()
	if mangohud != "" {
		path := filepath.Join(home, ".config", "MangoHud", "MangoHud.conf")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(mangohud), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env := map[string]string{"HOME": home}
	return &ProfileService{getenv: func(k string) string { return env[k] }}, home
}

func mangoConf(t *testing.T, home string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".config", "MangoHud", "MangoHud.conf"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestStateOnFirstStart(t *testing.T) {
	s, _ := newService(t, "fps_limit=60\n")

	st, err := s.State()

	if err != nil {
		t.Fatal(err)
	}
	if len(st.Profiles) != 4 || st.Active != 2 || st.FPSLabel != "60 FPS" || !st.Notify {
		t.Errorf("state %+v", st)
	}
	if st.Profiles[0].Label != "Unlimited" || st.Profiles[0].Settings == nil {
		t.Errorf("profile 0 %+v (settings must be [] for the window, not null)", st.Profiles[0])
	}
}

func TestUseProfileAndFPS(t *testing.T) {
	s, home := newService(t, "fps_limit=60\n")

	st, err := s.UseProfile(3)
	if err != nil || st.Active != 3 || mangoConf(t, home) != "fps_limit=90\n" {
		t.Errorf("profile: %+v, %v", st, err)
	}
	st, err = s.UseFPS("72")
	if err != nil || st.Active != -1 || st.FPSLabel != "72 FPS" {
		t.Errorf("fps: %+v, %v", st, err)
	}
	if _, err := s.UseFPS("fast"); err == nil {
		t.Error("accepted fast")
	}
}

func TestMissingMangoHudConfigIsAState(t *testing.T) {
	s, _ := newService(t, "")

	st, err := s.State()

	if err != nil || !st.MangoHudMissing || st.Active != -1 || len(st.Profiles) != 4 {
		t.Errorf("state %+v, %v", st, err)
	}
	if _, err := s.UseProfile(0); err == nil {
		t.Error("used a profile without MangoHud config")
	}
}

func TestSaveFromTheWindow(t *testing.T) {
	s, home := newService(t, "fps_limit=40\nfps_limit_method=early\n")
	st, _ := s.State()

	edit := Edit{Notify: false, Profiles: []Profile{
		{Name: "  Battery ", FPSLimit: "40", Settings: []Setting{
			{Key: "fps_limit_method", Value: "early"},
			{Key: "show_fps_limit", Value: "true"},
			{Key: "font_size", Value: "20"},
			{Key: "", Value: ""}, // an empty row from the form
		}},
		st.Profiles[0],
	}}
	st, err := s.Save(edit)

	if err != nil {
		t.Fatal(err)
	}
	if st.Notify || len(st.Profiles) != 2 || st.Profiles[0].Name != "Battery" {
		t.Errorf("state %+v", st)
	}
	// Matching compares MangoHud's text: show_fps_limit isn't in MangoHud.conf yet
	if st.Active != -1 {
		t.Errorf("active %d", st.Active)
	}
	data, _ := os.ReadFile(st.ConfigPath)
	for _, want := range []string{`name = "Battery"`,
		`settings = { font_size = 20, fps_limit_method = "early", show_fps_limit = true }`, "notify = false"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("config.toml lacks %q:\n%s", want, data)
		}
	}
	if _, err := s.UseProfile(0); err != nil {
		t.Fatal(err)
	}
	if got := mangoConf(t, home); got != "fps_limit=40\nfps_limit_method=early\nfont_size=20\nshow_fps_limit=1\n" {
		t.Errorf("MangoHud.conf %q", got)
	}
	if st, _ := s.State(); st.Active != 0 {
		t.Errorf("after using it: active %d", st.Active)
	}
}

func TestSaveRejectsAndKeepsTheFile(t *testing.T) {
	s, _ := newService(t, "fps_limit=60\n")
	st, _ := s.State()
	before, _ := os.ReadFile(st.ConfigPath)

	for name, e := range map[string]Edit{
		"no profiles": {},
		"bad fps":     {Profiles: []Profile{{Name: "a", FPSLimit: "fast"}}},
		"same names":  {Profiles: []Profile{{Name: "a", FPSLimit: "1"}, {Name: "A", FPSLimit: "2"}}},
		"bad key":     {Profiles: []Profile{{Name: "a", FPSLimit: "1", Settings: []Setting{{Key: "Font Size", Value: "1"}}}}},
	} {
		if _, err := s.Save(e); err == nil {
			t.Errorf("%s: saved", name)
		}
	}
	if after, _ := os.ReadFile(st.ConfigPath); string(after) != string(before) {
		t.Error("config.toml changed")
	}
}

func TestTyped(t *testing.T) {
	for in, want := range map[string]any{"true": true, "false": false, "20": int64(20), "1.5": 1.5,
		"early": "early", "top-left": "top-left", "1e3": "1e3", "NaN": "NaN", "": ""} {
		if got := typed(in); got != want {
			t.Errorf("%q: %#v", in, got)
		}
	}
}

func TestCheckFPS(t *testing.T) {
	s, _ := newService(t, "")
	if s.CheckFPS("60,90,0") != "" || s.CheckFPS("sixty") == "" {
		t.Error("check")
	}
}

// The watcher sees the CLI's in-place write, a replace-by-rename (editors, config save),
// and a MangoHud folder that appears later.
func TestWatchSeesChanges(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "config.toml")
	mango := filepath.Join(dir, "MangoHud", "MangoHud.conf")
	if err := os.WriteFile(conf, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed := make(chan struct{}, 10)
	stop := make(chan struct{})
	defer close(stop)
	if err := watch(func() []string { return []string{conf, mango} },
		func() { changed <- struct{}{} }, stop); err != nil {
		t.Fatal(err)
	}
	wait := func(what string) {
		t.Helper()
		select {
		case <-changed:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: no change seen", what)
		}
	}
	time.Sleep(100 * time.Millisecond) // let the watcher start

	if err := os.WriteFile(conf, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	wait("write")

	tmp := filepath.Join(dir, ".config-1.toml")
	if err := os.WriteFile(tmp, []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, conf); err != nil {
		t.Fatal(err)
	}
	wait("rename")

	if err := os.MkdirAll(filepath.Dir(mango), 0o755); err != nil {
		t.Fatal(err)
	}
	time.Sleep(3500 * time.Millisecond) // the retry adds the new folder
	if err := os.WriteFile(mango, []byte("fps_limit=60\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wait("new folder")
}
