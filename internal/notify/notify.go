// Package notify shows desktop notifications over D-Bus (org.freedesktop.Notifications),
// without needing notify-send.
package notify

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

// AppName is shown as the notification's sender.
const AppName = "MangoHud FPS Switch"

// Desktop sends notifications to the session's notification server. Each new notification
// replaces the previous one (its id is kept in IDFile), so pressing a shortcut several
// times shows one bubble, not a stack.
type Desktop struct {
	// IDFile keeps the last notification's id between runs; empty: don't replace.
	IDFile string
}

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
			AppName, d.lastID(), "input-gaming", summary, body, []string{}, hints, int32(2500)).
		Store(&id)
	if err != nil {
		return err
	}
	d.saveID(id)
	return nil
}

func (d Desktop) lastID() uint32 {
	if d.IDFile == "" {
		return 0
	}
	data, err := os.ReadFile(d.IDFile)
	if err != nil {
		return 0
	}
	id, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 32)
	if err != nil {
		return 0
	}
	return uint32(id)
}

func (d Desktop) saveID(id uint32) {
	if d.IDFile == "" {
		return
	}
	if os.MkdirAll(filepath.Dir(d.IDFile), 0o700) == nil {
		_ = os.WriteFile(d.IDFile, []byte(strconv.FormatUint(uint64(id), 10)), 0o600)
	}
}
