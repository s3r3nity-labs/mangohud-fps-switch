package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

// appID is the Flatpak ID; the autostart entry is named after it either way, so the
// window can tell whether it's on.
const appID = "org.s3r3nity.MangoHudFPSSwitch"

// inFlatpak reports whether the app runs as a Flatpak.
func inFlatpak(getenv func(string) string) bool {
	return getenv("FLATPAK_ID") != ""
}

// hostEnv corrects the environment inside the Flatpak: there XDG_CONFIG_HOME is the app's
// own ~/.var/app/<id>/config, but MangoHud's config and the CLI's config.toml are in the
// real one. Flatpak passes that as HOST_XDG_CONFIG_HOME when it's set; otherwise it's
// ~/.config (HOME is the real home in the sandbox).
func hostEnv(getenv func(string) string) func(string) string {
	if !inFlatpak(getenv) {
		return getenv
	}
	return func(key string) string {
		if key == "XDG_CONFIG_HOME" {
			return getenv("HOST_XDG_CONFIG_HOME")
		}
		return getenv(key)
	}
}

// configHome is the (host's) config folder.
func configHome(getenv func(string) string) string {
	if dir := getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(getenv("HOME"), ".config")
}

// autostart turns starting with the desktop (in the tray, --hidden) on and off.
type autostart struct {
	getenv func(string) string
	// executable is this program, for the entry written outside the Flatpak.
	executable func() (string, error)
	// requestBackground asks the Background portal (Flatpak); replaced in tests.
	requestBackground func(enable bool) (bool, error)
}

func newAutostart(getenv func(string) string) *autostart {
	return &autostart{getenv: getenv, executable: os.Executable, requestBackground: portalRequestBackground}
}

// entry is the autostart file: written by us, or by the desktop on the portal's behalf
// (named after the app ID too).
func (a *autostart) entry() string {
	return filepath.Join(configHome(a.getenv), "autostart", appID+".desktop")
}

// Enabled reports whether the autostart entry exists (and isn't switched off with Hidden=true,
// which some desktops' settings do instead of deleting it).
func (a *autostart) Enabled() bool {
	data, err := os.ReadFile(a.entry())
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "Hidden=true" {
			return false
		}
	}
	return true
}

// Set turns autostart on or off and returns the new state.
func (a *autostart) Set(enable bool) (bool, error) {
	if inFlatpak(a.getenv) {
		// The sandbox can't write the host's autostart folder: the desktop does it
		return a.requestBackground(enable)
	}
	if !enable {
		if err := os.Remove(a.entry()); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return a.Enabled(), err
		}
		return false, nil
	}
	exe, err := a.executable()
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(a.entry()), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(a.entry(), []byte(desktopEntry(exe)), 0o644)
}

// desktopEntry is the autostart file for the program at exe.
func desktopEntry(exe string) string {
	return `[Desktop Entry]
Type=Application
Name=MangoHud FPS Switch
Comment=Switch MangoHud's FPS limit from the tray
Exec=` + execQuote(exe) + ` --hidden
Icon=` + appID + `
Terminal=false
X-GNOME-Autostart-enabled=true
`
}

// execQuote quotes a path for a desktop file's Exec line if it needs it.
func execQuote(path string) string {
	if !strings.ContainsAny(path, " \t\"'\\$`") {
		return path
	}
	r := strings.NewReplacer(`\`, `\\\\`, `"`, `\\"`, "`", "\\\\`", `$`, `\\$`)
	return `"` + r.Replace(path) + `"`
}

// portalRequestBackground asks xdg-desktop-portal to start the app with the desktop
// (org.freedesktop.portal.Background.RequestBackground). The answer comes as a Response
// signal on a request object; the desktop may ask the user first.
func portalRequestBackground(enable bool) (bool, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false, err
	}
	names := conn.Names()
	if len(names) == 0 {
		return false, errors.New("no D-Bus name")
	}
	tokenBytes := make([]byte, 8)
	if _, err := rand.Read(tokenBytes); err != nil {
		return false, err
	}
	token := "mfs" + hex.EncodeToString(tokenBytes)
	sender := strings.ReplaceAll(strings.TrimPrefix(names[0], ":"), ".", "_")
	request := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + sender + "/" + token)

	// Listen before asking, so the answer can't be missed
	match := []dbus.MatchOption{
		dbus.WithMatchObjectPath(request),
		dbus.WithMatchInterface("org.freedesktop.portal.Request"),
		dbus.WithMatchMember("Response"),
	}
	if err := conn.AddMatchSignal(match...); err != nil {
		return false, err
	}
	defer conn.RemoveMatchSignal(match...)
	signals := make(chan *dbus.Signal, 8)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)

	options := map[string]dbus.Variant{
		"handle_token":     dbus.MakeVariant(token),
		"reason":           dbus.MakeVariant("Start in the tray when you log in, to switch FPS profiles"),
		"autostart":        dbus.MakeVariant(enable),
		"commandline":      dbus.MakeVariant([]string{"mangohud-fps-switch-gui", "--hidden"}),
		"dbus-activatable": dbus.MakeVariant(false),
	}
	call := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop").
		Call("org.freedesktop.portal.Background.RequestBackground", 0, "", options)
	if call.Err != nil {
		return false, call.Err
	}

	timeout := time.After(2 * time.Minute) // the desktop may be asking the user
	for {
		select {
		case s := <-signals:
			if s.Path != request || len(s.Body) < 2 {
				continue
			}
			return backgroundResponse(s.Body[0], s.Body[1], enable)
		case <-timeout:
			return false, errors.New("no answer from the desktop about autostart")
		}
	}
}

// backgroundResponse reads the portal's answer: 0 granted, 1 refused by the user, 2 other.
func backgroundResponse(code, results any, asked bool) (bool, error) {
	c, _ := code.(uint32)
	switch c {
	case 0:
		if r, ok := results.(map[string]dbus.Variant); ok {
			if v, ok := r["autostart"].Value().(bool); ok {
				return v, nil
			}
		}
		return asked, nil
	case 1:
		return false, errors.New("the desktop didn't allow starting in the background")
	default:
		return false, fmt.Errorf("the desktop couldn't set up autostart (response %d)", c)
	}
}
