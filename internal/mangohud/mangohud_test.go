package mangohud

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "MangoHud.conf")
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
	return path
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestReadLikeMangoHud(t *testing.T) {
	path := write(t, "# fps_limit=30\nfps_limit = 60 # comment\ngpu_stats\nposition=top-left\nfps_limit=90\n\n")

	got, err := Read(path)

	if err != nil {
		t.Fatal(err)
	}
	// commented-out line ignored, last line wins, bare key = 1, spaces trimmed
	want := map[string]string{"fps_limit": "90", "gpu_stats": "1", "position": "top-left"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestSetKeepsEverythingElse(t *testing.T) {
	path := write(t, "# my config\n# fps_limit=0\n  fps_limit=60 # battery\ngpu_stats\nfps_limit=30\n")

	if err := Set(path, map[string]string{"fps_limit": "90"}); err != nil {
		t.Fatal(err)
	}

	want := "# my config\n# fps_limit=0\n  fps_limit=90 # battery\ngpu_stats\nfps_limit=90\n"
	if got := read(t, path); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestSetAddsMissingKeysSorted(t *testing.T) {
	path := write(t, "# fps_limit=0\ngpu_stats\n")

	if err := Set(path, map[string]string{"fps_limit": "45", "fps_limit_method": "early", "gpu_stats": "0"}); err != nil {
		t.Fatal(err)
	}

	want := "# fps_limit=0\ngpu_stats=0\nfps_limit=45\nfps_limit_method=early\n"
	if got := read(t, path); got != want {
		t.Errorf("got %q", got)
	}
}

func TestSetFileWithoutTrailingNewlineAndCRLF(t *testing.T) {
	path := write(t, "gpu_stats")
	if err := Set(path, map[string]string{"fps_limit": "60"}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != "gpu_stats\nfps_limit=60\n" {
		t.Errorf("got %q", got)
	}

	path = write(t, "fps_limit=30\r\ngpu_stats\r\n")
	if err := Set(path, map[string]string{"fps_limit": "60"}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != "fps_limit=60\r\ngpu_stats\r\n" {
		t.Errorf("got %q", got)
	}
}

// Written in place: the file keeps its mode, and a linked config (dotfiles) stays a link.
func TestSetWritesThroughLinkAndKeepsMode(t *testing.T) {
	target := write(t, "fps_limit=60\n")
	link := filepath.Join(t.TempDir(), "MangoHud.conf")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := Set(link, map[string]string{"fps_limit": "90"}); err != nil {
		t.Fatal(err)
	}

	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("link replaced: %v %v", info, err)
	}
	if got := read(t, target); got != "fps_limit=90\n" {
		t.Errorf("target %q", got)
	}
	if info, _ := os.Stat(target); info.Mode().Perm() != 0o640 {
		t.Errorf("mode %v", info.Mode())
	}
}

func TestSetRejectsBadInput(t *testing.T) {
	path := write(t, "fps_limit=60\n")
	for _, s := range []map[string]string{
		{"fps_limit": "60\ngpu_stats"},
		{"fps_limit": "60 # x"},
		{"Bad Key": "1"},
	} {
		if err := Set(path, s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
	if got := read(t, path); got != "fps_limit=60\n" {
		t.Errorf("changed: %q", got)
	}
}

func TestSetNeedsExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "MangoHud.conf")
	if err := Set(path, map[string]string{"fps_limit": "60"}); err == nil {
		t.Error("created a config")
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("file exists")
	}
}

func TestDefaultPath(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	for _, c := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"HOME": "/h"}, "/h/.config/MangoHud/MangoHud.conf"},
		{map[string]string{"HOME": "/h", "XDG_CONFIG_HOME": "/x"}, "/x/MangoHud/MangoHud.conf"},
		{map[string]string{"HOME": "/h", "MANGOHUD_CONFIGFILE": "/g/my.conf"}, "/g/my.conf"},
	} {
		if got, _ := DefaultPath(env(c.env)); got != c.want {
			t.Errorf("%v: got %s", c.env, got)
		}
	}
}

func TestPerGameConfigs(t *testing.T) {
	global := write(t, "")
	dir := filepath.Dir(global)
	for _, name := range []string{"wine-GenshinImpact.conf", "Client-Win64-Shipping.conf", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := PerGameConfigs(global)

	if len(got) != 2 {
		t.Errorf("got %v", got)
	}
}
