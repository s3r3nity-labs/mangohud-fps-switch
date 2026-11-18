package main

import (
	"errors"
	"strings"
	"testing"
)

// lines renders a tray menu as text: "(x)" checked profile, "( )" profile, "-" separator,
// "[text]" not clickable.
func lines(entries []trayEntry) string {
	var out []string
	for _, e := range entries {
		switch e.kind {
		case entryInfo:
			out = append(out, "["+e.label+"]")
		case entrySeparator:
			out = append(out, "-")
		case entryProfile:
			mark := "( )"
			if e.checked {
				mark = "(x)"
			}
			out = append(out, mark+" "+e.label)
		default:
			out = append(out, e.label)
		}
	}
	return strings.Join(out, "\n")
}

func trayState() State {
	return State{
		FPSLabel: "60 FPS",
		Active:   1,
		Profiles: []Profile{
			{Name: "Unlimited", Label: "Unlimited"},
			{Name: "60 FPS", Label: "60 FPS"},
			{Name: "Battery", Label: "40 FPS"},
		},
	}
}

func TestTrayMenu(t *testing.T) {
	got := lines(trayMenu(trayState(), nil))

	want := `[Now: 60 FPS]
-
( ) 0  Unlimited
(x) 1  60 FPS
( ) 2  Battery (40 FPS)
-
Open window
Quit`
	if got != want {
		t.Errorf("got\n%s", got)
	}
}

func TestTrayMenuProfileIndexes(t *testing.T) {
	for _, e := range trayMenu(trayState(), nil) {
		if e.kind == entryProfile && !strings.HasPrefix(e.label, string(rune('0'+e.profile))) {
			t.Errorf("%q uses profile %d", e.label, e.profile)
		}
	}
}

// Without a usable MangoHud config the profiles are shown but can't be clicked.
func TestTrayMenuWithoutMangoHudConfig(t *testing.T) {
	st := trayState()
	st.MangoHudMissing, st.Active, st.FPSLabel = true, -1, ""

	got := lines(trayMenu(st, nil))

	if !strings.HasPrefix(got, "[MangoHud has no config file: open the window]") ||
		strings.Contains(got, "( )") || !strings.Contains(got, "[2  Battery (40 FPS)]") {
		t.Errorf("got\n%s", got)
	}
}

func TestTrayMenuOnError(t *testing.T) {
	got := lines(trayMenu(State{}, errors.New("broken config.toml")))
	if got != "[Can't read the profiles: open the window]\n-\nOpen window\nQuit" {
		t.Errorf("got\n%s", got)
	}
}

func TestTrayTooltip(t *testing.T) {
	st := trayState()
	if got := trayTooltip(st, nil); got != "MangoHud FPS Switch: 60 FPS (60 FPS)" {
		t.Errorf("active: %q", got)
	}
	st.Active, st.FPSLabel = -1, "72 FPS"
	if got := trayTooltip(st, nil); got != "MangoHud FPS Switch: 72 FPS" {
		t.Errorf("no profile: %q", got)
	}
	if got := trayTooltip(State{}, errors.New("x")); !strings.Contains(got, "attention") {
		t.Errorf("error: %q", got)
	}
}
