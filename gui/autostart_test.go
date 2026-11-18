package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

func envOf(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestHostEnvInsideFlatpak(t *testing.T) {
	outside := envOf(map[string]string{"HOME": "/home/k", "XDG_CONFIG_HOME": "/home/k/.cfg"})
	if got := hostEnv(outside)("XDG_CONFIG_HOME"); got != "/home/k/.cfg" {
		t.Errorf("outside: %q", got)
	}
	// In the sandbox XDG_CONFIG_HOME is the app's own folder: use the host's (or ~/.config)
	inside := map[string]string{"HOME": "/home/k", "FLATPAK_ID": appID,
		"XDG_CONFIG_HOME": "/home/k/.var/app/" + appID + "/config"}
	if got := configHome(hostEnv(envOf(inside))); got != "/home/k/.config" {
		t.Errorf("inside: %q", got)
	}
	inside["HOST_XDG_CONFIG_HOME"] = "/home/k/.cfg"
	if got := configHome(hostEnv(envOf(inside))); got != "/home/k/.cfg" {
		t.Errorf("inside, host has its own: %q", got)
	}
}

// The window and the tray read the real configs inside the Flatpak.
func TestServiceUsesTheHostConfigInFlatpak(t *testing.T) {
	home := t.TempDir()
	mango := filepath.Join(home, ".config", "MangoHud", "MangoHud.conf")
	if err := os.MkdirAll(filepath.Dir(mango), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mango, []byte("fps_limit=45\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := envOf(map[string]string{"HOME": home, "FLATPAK_ID": appID,
		"XDG_CONFIG_HOME": filepath.Join(home, ".var", "app", appID, "config")})
	s := &ProfileService{getenv: hostEnv(env)}

	st, err := s.State()

	if err != nil || st.MangoHudPath != mango || st.FPSLabel != "45 FPS" ||
		st.ConfigPath != filepath.Join(home, ".config", "mangohud-fps-switch", "config.toml") {
		t.Errorf("state %+v, %v", st, err)
	}
}

func TestAutostartWithoutFlatpak(t *testing.T) {
	home := t.TempDir()
	a := newAutostart(envOf(map[string]string{"HOME": home}))
	a.executable = func() (string, error) { return "/home/k/my apps/mangohud-fps-switch-gui", nil }
	a.requestBackground = func(bool) (bool, error) { t.Fatal("portal used outside Flatpak"); return false, nil }

	if a.Enabled() {
		t.Fatal("enabled at start")
	}
	if on, err := a.Set(true); err != nil || !on || !a.Enabled() {
		t.Fatalf("on: %v %v", on, err)
	}
	data, _ := os.ReadFile(filepath.Join(home, ".config", "autostart", appID+".desktop"))
	if !strings.Contains(string(data), `Exec="/home/k/my apps/mangohud-fps-switch-gui" --hidden`) {
		t.Errorf("entry:\n%s", data)
	}
	if on, err := a.Set(false); err != nil || on || a.Enabled() {
		t.Errorf("off: %v %v", on, err)
	}
	if _, err := a.Set(false); err != nil {
		t.Errorf("off twice: %v", err)
	}
}

// Desktops' settings may switch an entry off with Hidden=true instead of deleting it.
func TestAutostartHiddenEntryIsOff(t *testing.T) {
	home := t.TempDir()
	a := newAutostart(envOf(map[string]string{"HOME": home}))
	path := a.entry()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[Desktop Entry]\nExec=x\nHidden=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if a.Enabled() {
		t.Error("hidden entry counts as on")
	}
}

func TestAutostartInFlatpakAsksThePortal(t *testing.T) {
	home := t.TempDir()
	a := newAutostart(envOf(map[string]string{"HOME": home, "FLATPAK_ID": appID}))
	var asked []bool
	a.requestBackground = func(enable bool) (bool, error) {
		asked = append(asked, enable)
		return enable, nil
	}

	if on, err := a.Set(true); err != nil || !on || len(asked) != 1 || !asked[0] {
		t.Errorf("on: %v %v %v", on, err, asked)
	}
	if _, err := os.Stat(a.entry()); err == nil {
		t.Error("wrote the entry itself inside the Flatpak")
	}
}

func TestBackgroundResponse(t *testing.T) {
	granted := map[string]dbus.Variant{"autostart": dbus.MakeVariant(true), "background": dbus.MakeVariant(true)}
	if on, err := backgroundResponse(uint32(0), granted, true); err != nil || !on {
		t.Errorf("granted: %v %v", on, err)
	}
	if on, err := backgroundResponse(uint32(0), map[string]dbus.Variant{}, false); err != nil || on {
		t.Errorf("no autostart in results: %v %v", on, err)
	}
	if _, err := backgroundResponse(uint32(1), nil, true); err == nil {
		t.Error("refused without error")
	}
	if _, err := backgroundResponse(uint32(2), nil, true); err == nil {
		t.Error("failure without error")
	}
}

func TestServiceAutostartNeedsTray(t *testing.T) {
	s := &ProfileService{autostart: newAutostart(envOf(map[string]string{"HOME": t.TempDir()}))}
	if st := s.AutostartState(); st.Available {
		t.Error("available without tray")
	}
	if _, err := s.SetAutostart(true); err == nil {
		t.Error("set without tray")
	}
	s.hasTray = true
	s.autostart.executable = func() (string, error) { return "/usr/bin/x", nil }
	if st, err := s.SetAutostart(true); err != nil || !st.Enabled || !st.Available {
		t.Errorf("with tray: %+v %v", st, err)
	}
}

func TestExecQuote(t *testing.T) {
	for in, want := range map[string]string{
		"/app/bin/x":          "/app/bin/x",
		"/home/k/my apps/x":   `"/home/k/my apps/x"`,
		`/home/k/$weird"\dir`: `"/home/k/\\$weird\\"\\\\dir"`,
	} {
		if got := execQuote(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}
