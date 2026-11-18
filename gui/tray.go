package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// entryKind is what a tray menu line is.
type entryKind int

const (
	entryInfo      entryKind = iota // a line of text, not clickable
	entryProfile                    // a profile: click to use it, checked when active
	entrySeparator                  //
	entryOpen                       // open the window
	entryQuit                       // quit the app
)

// trayEntry is one line of the tray menu.
type trayEntry struct {
	kind    entryKind
	label   string
	checked bool
	profile int
}

// trayMenu is what the tray menu shows for a state: the current limit, the profiles (the
// active one checked), Open and Quit. err is a failure to read the profiles.
func trayMenu(st State, err error) []trayEntry {
	var out []trayEntry
	switch {
	case err != nil:
		out = append(out, trayEntry{kind: entryInfo, label: "Can't read the profiles: open the window"})
	case st.MangoHudMissing:
		out = append(out, trayEntry{kind: entryInfo, label: "MangoHud has no config file: open the window"})
	case st.Problem != "":
		out = append(out, trayEntry{kind: entryInfo, label: "Can't read MangoHud's config: open the window"})
	default:
		out = append(out, trayEntry{kind: entryInfo, label: "Now: " + st.FPSLabel})
	}
	if err == nil && len(st.Profiles) > 0 {
		out = append(out, trayEntry{kind: entrySeparator})
		usable := !st.MangoHudMissing && st.Problem == ""
		for i, p := range st.Profiles {
			label := fmt.Sprintf("%d  %s", i, p.Name)
			if p.Label != p.Name {
				label += " (" + p.Label + ")"
			}
			e := trayEntry{kind: entryProfile, label: label, checked: i == st.Active, profile: i}
			if !usable {
				e.kind = entryInfo
			}
			out = append(out, e)
		}
	}
	return append(out,
		trayEntry{kind: entrySeparator},
		trayEntry{kind: entryOpen, label: "Open window"},
		trayEntry{kind: entryQuit, label: "Quit"},
	)
}

// trayTooltip is the text when hovering the tray icon.
func trayTooltip(st State, err error) string {
	switch {
	case err != nil || st.MangoHudMissing || st.Problem != "":
		return "MangoHud FPS Switch: needs attention"
	case st.Active >= 0:
		return fmt.Sprintf("MangoHud FPS Switch: %s (%s)", st.FPSLabel, st.Profiles[st.Active].Name)
	default:
		return "MangoHud FPS Switch: " + st.FPSLabel
	}
}

// tray is the tray icon and its menu.
type tray struct {
	app     *application.App
	icon    *application.SystemTray
	window  application.Window
	service *ProfileService
}

func newTray(app *application.App, window application.Window, service *ProfileService, icon []byte) *tray {
	t := &tray{app: app, window: window, service: service, icon: app.SystemTray.New()}
	t.icon.SetIcon(icon)
	// Left click: show the window, or hide it when it's open. Right click: the menu
	// (the desktop shows it).
	t.icon.OnClick(func() {
		if t.window.IsVisible() {
			t.window.Hide()
		} else {
			t.show()
		}
	})
	t.refresh()
	return t
}

func (t *tray) show() {
	t.window.Show().Focus()
}

// refresh rebuilds the menu from the current state; called after every change.
func (t *tray) refresh() {
	st, err := t.service.State()
	menu := t.app.NewMenu()
	for _, e := range trayMenu(st, err) {
		switch e.kind {
		case entryInfo:
			menu.Add(e.label).SetEnabled(false)
		case entrySeparator:
			menu.AddSeparator()
		case entryProfile:
			i := e.profile
			menu.AddRadio(e.label, e.checked).OnClick(func(*application.Context) {
				if _, err := t.service.UseProfile(i); err != nil {
					log.Printf("profile %d: %v", i, err)
				}
				// The watcher reloads the window; the menu right away
				go t.refresh()
			})
		case entryOpen:
			menu.Add(e.label).OnClick(func(*application.Context) { t.show() })
		case entryQuit:
			menu.Add(e.label).OnClick(func(*application.Context) { t.app.Quit() })
		}
	}
	t.icon.SetMenu(menu)
	t.icon.SetTooltip(trayTooltip(st, err))
	announceTooltip()
}

// announceTooltip tells the desktop that the tooltip changed. Wails 3.1 only updates the
// ToolTip property, but tray hosts like KDE Plasma re-read it on the NewToolTip signal (as
// they re-read the icon on NewIcon, which Wails does send), so the hover text would stay
// the first one. It has to come from the tray's own D-Bus connection: Wails uses godbus'
// shared session connection, which dbus.SessionBus returns here too.
func announceTooltip() {
	conn, err := dbus.SessionBus()
	if err != nil {
		return
	}
	if err := conn.Emit("/StatusNotifierItem", "org.kde.StatusNotifierItem.NewToolTip"); err != nil {
		log.Printf("tray tooltip: %v", err)
	}
}

// trayAvailable reports whether the desktop shows tray icons (StatusNotifierItem: KDE
// Plasma, most others; GNOME only with the AppIndicator extension). Without it, closing
// the window must quit, or the app would keep running invisibly.
func trayAvailable() bool {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return false
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var has bool
	err = conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0,
		"org.kde.StatusNotifierWatcher").Store(&has)
	return err == nil && has
}
