package notify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIDFileRoundTrip(t *testing.T) {
	d := Desktop{IDFile: filepath.Join(t.TempDir(), "mangohud-fps-switch", "notification-id")}
	if d.lastID() != 0 {
		t.Error("id before any notification")
	}
	d.saveID(42)
	if d.lastID() != 42 {
		t.Errorf("got %d", d.lastID())
	}
	if info, _ := os.Stat(filepath.Dir(d.IDFile)); info.Mode().Perm() != 0o700 {
		t.Errorf("folder mode %v", info.Mode())
	}
}

func TestBrokenIDFileStartsFresh(t *testing.T) {
	d := Desktop{IDFile: filepath.Join(t.TempDir(), "id")}
	if err := os.WriteFile(d.IDFile, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if d.lastID() != 0 {
		t.Error("used a broken id")
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
