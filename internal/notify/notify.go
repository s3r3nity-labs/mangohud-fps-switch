// Package notify shows desktop notifications over D-Bus (org.freedesktop.Notifications),
// without needing notify-send.
package notify

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

// AppName is shown as the notification's sender.
const AppName = "MangoHud FPS Switch"

// Timeout is how long a notification stays on screen.
const Timeout = 2500 * time.Millisecond

// reuseWithin is how soon after the last notification the next one replaces it instead of
// opening a new one: while the old bubble is surely still on screen (each replacement
// restarts its timeout). Replacing a notification that has already closed doesn't show
// anything on KDE Plasma: the replacement inherits the "expired" state.
const reuseWithin = 2 * time.Second

// Desktop sends notifications to the session's notification server. Pressing a shortcut
// several times quickly shows one bubble, not a stack: the id and time of the last
// notification are kept in IDFile between runs.
type Desktop struct {
	// IDFile keeps the last notification's id between runs; empty: never replace.
	IDFile string
	// Now is the clock (tests); nil is time.Now.
	Now func() time.Time
}

// excessError is KDE Plasma's answer to the same notification again within a second: the
// first one is still on screen, so that's fine.
const excessError = "org.freedesktop.Notifications.Error.ExcessNotificationGeneration"

// RuntimeIDFile is where the id lives: $XDG_RUNTIME_DIR, gone after logout like the
// notifications themselves.
func RuntimeIDFile(getenv func(string) string) string {
	dir := getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "mangohud-fps-switch", "notification-id")
}

// Notify shows summary and body. It gives up after two seconds, e.g. without a session bus.
func (d Desktop) Notify(summary, body string) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	hints := map[string]dbus.Variant{
		"transient": dbus.MakeVariant(true), // don't keep it in the notification history
		"urgency":   dbus.MakeVariant(byte(0)),
	}
	var id uint32
	err = conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications").
		CallWithContext(ctx, "org.freedesktop.Notifications.Notify", 0,
			AppName, d.lastID(), "input-gaming", summary, body, []string{}, hints, int32(Timeout.Milliseconds())).
		Store(&id)
	if isExcess(err) {
		return nil
	}
	if err != nil {
		return err
	}
	d.saveID(id)
	return nil
}

// isExcess reports Plasma's "same notification again" refusal (godbus returns D-Bus errors
// as a value or a pointer).
func isExcess(err error) bool {
	var value dbus.Error
	var pointer *dbus.Error
	return errors.As(err, &value) && value.Name == excessError ||
		errors.As(err, &pointer) && pointer.Name == excessError
}

func (d Desktop) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// lastID is the previous notification's id if it's recent enough to replace, else 0 (new).
// The file holds "<id> <unix milliseconds>".
func (d Desktop) lastID() uint32 {
	if d.IDFile == "" {
		return 0
	}
	data, err := os.ReadFile(d.IDFile)
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) != 2 {
		return 0
	}
	id, errID := strconv.ParseUint(fields[0], 10, 32)
	ms, errMS := strconv.ParseInt(fields[1], 10, 64)
	if errID != nil || errMS != nil {
		return 0
	}
	age := d.now().Sub(time.UnixMilli(ms))
	if age < 0 || age >= reuseWithin {
		return 0
	}
	return uint32(id)
}

func (d Desktop) saveID(id uint32) {
	if d.IDFile == "" || id == 0 {
		return
	}
	if os.MkdirAll(filepath.Dir(d.IDFile), 0o700) == nil {
		line := strconv.FormatUint(uint64(id), 10) + " " + strconv.FormatInt(d.now().UnixMilli(), 10)
		_ = os.WriteFile(d.IDFile, []byte(line), 0o600)
	}
}
