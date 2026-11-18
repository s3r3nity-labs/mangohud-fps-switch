package notify

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newDesktop(t *testing.T) (Desktop, *clock) {
	t.Helper()
	c := &clock{time.Date(2026, 11, 18, 20, 0, 0, 0, time.UTC)}
	return Desktop{IDFile: filepath.Join(t.TempDir(), "mangohud-fps-switch", "notification-id"), Now: c.now}, c
}

func TestReplacesOnlyWhileTheBubbleIsShown(t *testing.T) {
	d, c := newDesktop(t)
	if d.lastID() != 0 {
		t.Error("id before any notification")
	}
	d.saveID(42)

	c.t = c.t.Add(1500 * time.Millisecond) // pressed again quickly: replace
	if got := d.lastID(); got != 42 {
		t.Errorf("after 1.5s: %d, want 42", got)
	}
	// Plasma shows nothing for a replaced notification that already closed: after a pause,
	// open a new one
	c.t = c.t.Add(time.Second)
	if got := d.lastID(); got != 0 {
		t.Errorf("after 2.5s: %d, want a new notification", got)
	}
	if info, _ := os.Stat(filepath.Dir(d.IDFile)); info.Mode().Perm() != 0o700 {
		t.Errorf("folder mode %v", info.Mode())
	}
}

func TestClockGoingBackStartsFresh(t *testing.T) {
	d, c := newDesktop(t)
	d.saveID(42)
	c.t = c.t.Add(-time.Minute)
	if d.lastID() != 0 {
		t.Error("reused an id from the future")
	}
}

func TestBrokenOrOldIDFileStartsFresh(t *testing.T) {
	d, _ := newDesktop(t)
	if err := os.MkdirAll(filepath.Dir(d.IDFile), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{"garbage", "42", "42 abc", "x 1"} {
		if err := os.WriteFile(d.IDFile, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if d.lastID() != 0 {
			t.Errorf("%q: used it", content)
		}
	}
}

func TestRuntimeIDFile(t *testing.T) {
	if got := RuntimeIDFile(func(string) string { return "" }); got != "" {
		t.Errorf("without XDG_RUNTIME_DIR: %q", got)
	}
	if got := RuntimeIDFile(func(string) string { return "/run/user/1000" }); got != "/run/user/1000/mangohud-fps-switch/notification-id" {
		t.Errorf("got %q", got)
	}
}

func TestPlasmasDuplicateRefusalIsNoError(t *testing.T) {
	for _, err := range []error{
		dbus.Error{Name: excessError},
		dbus.NewError(excessError, nil),
		fmt.Errorf("call: %w", dbus.Error{Name: excessError}),
	} {
		if !isExcess(err) {
			t.Errorf("%#v not recognised", err)
		}
	}
	for _, err := range []error{nil, errors.New("no bus"), dbus.Error{Name: "org.freedesktop.DBus.Error.ServiceUnknown"}} {
		if isExcess(err) {
			t.Errorf("%v taken as duplicate", err)
		}
	}
}
