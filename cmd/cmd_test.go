package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type notification struct{ summary, body string }

type fakeNotifier struct {
	sent []notification
	err  error
}

func (f *fakeNotifier) Notify(summary, body string) error {
	f.sent = append(f.sent, notification{summary, body})
	return f.err
}

type setup struct {
	home     string
	env      map[string]string
	notifier *fakeNotifier
}

// newSetup makes a home with a MangoHud config (content) and no profiles yet.
func newSetup(t *testing.T, content string) *setup {
	t.Helper()
	home := t.TempDir()
	s := &setup{home: home, env: map[string]string{"HOME": home}, notifier: &fakeNotifier{}}
	if content != "" {
		s.write(t, ".config/MangoHud/MangoHud.conf", content)
	}
	return s
}

func (s *setup) write(t *testing.T, rel, content string) {
	t.Helper()
	path := filepath.Join(s.home, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (s *setup) mangohud(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(s.home, ".config/MangoHud/MangoHud.conf"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func (s *setup) run(args ...string) (stdout, stderr string, err error) {
	root := newRootCmd(deps{getenv: func(k string) string { return s.env[k] }, notifier: s.notifier})
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errOut.String(), err
}

func TestSetFPSAndNotify(t *testing.T) {
	s := newSetup(t, "# fps_limit=0\nfps_limit=60\ngpu_stats\n")

	out, errOut, err := s.run("90")

	if err != nil || out != "MangoHud: 90 FPS\n" {
		t.Fatalf("out %q, err %v", out, err)
	}
	if !strings.Contains(errOut, "Created") {
		t.Errorf("first run doesn't say it created config.toml: %q", errOut)
	}
	if got := s.mangohud(t); got != "# fps_limit=0\nfps_limit=90\ngpu_stats\n" {
		t.Errorf("MangoHud.conf %q", got)
	}
	if len(s.notifier.sent) != 1 || s.notifier.sent[0] != (notification{"90 FPS", ""}) {
		t.Errorf("notifications %v", s.notifier.sent)
	}
}

func TestBadFPSChangesNothing(t *testing.T) {
	s := newSetup(t, "fps_limit=60\n")
	if _, _, err := s.run("fast"); err == nil {
		t.Error("accepted")
	}
	if s.mangohud(t) != "fps_limit=60\n" || len(s.notifier.sent) != 0 {
		t.Error("changed something")
	}
}

func TestProfileByNumberAndName(t *testing.T) {
	s := newSetup(t, "fps_limit=60\n")
	s.write(t, ".config/mangohud-fps-switch/config.toml", `
[[profile]]
name = "Unlimited"
fps_limit = 0

[[profile]]
name = "Battery"
fps_limit = 40
settings = { fps_limit_method = "early" }
`)

	out, _, err := s.run("profile", "1")
	if err != nil || out != "MangoHud: 40 FPS (profile \"Battery\")\n" {
		t.Fatalf("out %q, err %v", out, err)
	}
	if got := s.mangohud(t); got != "fps_limit=40\nfps_limit_method=early\n" {
		t.Errorf("MangoHud.conf %q", got)
	}
	if s.notifier.sent[0] != (notification{"40 FPS", "Profile: Battery"}) {
		t.Errorf("notification %v", s.notifier.sent[0])
	}

	if _, _, err := s.run("profile", "unlimited"); err != nil {
		t.Fatal(err)
	}
	// The label already says it: no "Profile: Unlimited" body
	if got := s.notifier.sent[1]; got != (notification{"Unlimited", ""}) {
		t.Errorf("notification %v", got)
	}
}

func TestProfileListMarksTheActiveOne(t *testing.T) {
	s := newSetup(t, "fps_limit=60\n")

	out, _, err := s.run("profile")

	if err != nil {
		t.Fatal(err)
	}
	want := "  0  Unlimited  fps_limit=0\n" +
		"  1  45 FPS     fps_limit=45\n" +
		"* 2  60 FPS     fps_limit=60\n" +
		"  3  90 FPS     fps_limit=90\n"
	if out != want {
		t.Errorf("got\n%s", out)
	}
	if len(s.notifier.sent) != 0 {
		t.Error("listing notified")
	}
}

func TestCycleWrapsAround(t *testing.T) {
	s := newSetup(t, "fps_limit=60\n")
	var got []string
	for range 3 {
		out, _, err := s.run("cycle")
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, strings.TrimSpace(out))
	}
	want := []string{
		`MangoHud: 90 FPS (profile "90 FPS")`,
		`MangoHud: Unlimited (profile "Unlimited")`,
		`MangoHud: 45 FPS (profile "45 FPS")`,
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q", got)
	}
}

func TestCycleFromNoProfile(t *testing.T) {
	s := newSetup(t, "fps_limit=30\n")
	if out, _, _ := s.run("cycle"); !strings.Contains(out, `"Unlimited"`) {
		t.Errorf("forward: %q", out)
	}
	s = newSetup(t, "fps_limit=30\n")
	if out, _, _ := s.run("cycle", "--back"); !strings.Contains(out, `"90 FPS"`) {
		t.Errorf("back: %q", out)
	}
}

func TestNoNotify(t *testing.T) {
	s := newSetup(t, "fps_limit=60\n")
	if _, _, err := s.run("45", "--no-notify"); err != nil {
		t.Fatal(err)
	}
	s.write(t, ".config/mangohud-fps-switch/config.toml", "notify = false\n[[profile]]\nname = \"a\"\nfps_limit = 1\n")
	if _, _, err := s.run("profile", "0"); err != nil {
		t.Fatal(err)
	}
	if len(s.notifier.sent) != 0 {
		t.Errorf("notified %v", s.notifier.sent)
	}
}

// Without a notification server the change still counts; the reason goes to stderr.
func TestNotificationFailureIsNotAnError(t *testing.T) {
	s := newSetup(t, "fps_limit=60\n")
	s.notifier.err = errors.New("no session bus")

	_, errOut, err := s.run("45")

	if err != nil || !strings.Contains(errOut, "no session bus") || s.mangohud(t) != "fps_limit=45\n" {
		t.Errorf("err %v, stderr %q", err, errOut)
	}
}

func TestMissingMangoHudConfig(t *testing.T) {
	s := newSetup(t, "")

	_, _, err := s.run("60")

	if err == nil || !strings.Contains(err.Error(), "MangoHud config not found") {
		t.Errorf("err %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(s.home, ".config/MangoHud/MangoHud.conf")); statErr == nil {
		t.Error("created MangoHud.conf")
	}
}

func TestMangoHudConfigFromSettingAndEnv(t *testing.T) {
	s := newSetup(t, "fps_limit=60\n")
	s.write(t, "games/mango.conf", "fps_limit=60\n")
	s.write(t, ".config/mangohud-fps-switch/config.toml",
		"mangohud_config = \"~/games/mango.conf\"\n[[profile]]\nname = \"a\"\nfps_limit = 1\n")
	if _, _, err := s.run("30"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(s.home, "games/mango.conf")); string(data) != "fps_limit=30\n" {
		t.Errorf("mango.conf %q", data)
	}

	s = newSetup(t, "fps_limit=60\n")
	s.write(t, "other.conf", "fps_limit=60\n")
	s.env["MANGOHUD_CONFIGFILE"] = filepath.Join(s.home, "other.conf")
	if _, _, err := s.run("30"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(s.env["MANGOHUD_CONFIGFILE"]); string(data) != "fps_limit=30\n" {
		t.Errorf("other.conf %q", data)
	}
	if s.mangohud(t) != "fps_limit=60\n" {
		t.Error("changed the global config")
	}
}

func TestStatus(t *testing.T) {
	s := newSetup(t, "fps_limit=45\n")
	s.write(t, ".config/MangoHud/wine-GenshinImpact.conf", "")
	s.env["MANGOHUD_CONFIG"] = "fps_limit=30,gpu_stats"

	out, _, err := s.run("status")

	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"FPS limit:        45 FPS", `Profile:          1 "45 FPS"`,
		"MangoHud.conf", "wine-GenshinImpact.conf", "MANGOHUD_CONFIG sets fps_limit"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestHelpWithoutArguments(t *testing.T) {
	s := newSetup(t, "fps_limit=60\n")
	out, _, err := s.run()
	if err != nil || !strings.Contains(out, "cycle") {
		t.Errorf("out %q, err %v", out, err)
	}
}
