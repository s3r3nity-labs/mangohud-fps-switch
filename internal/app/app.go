// Package app is what the CLI and the GUI share: which files are used, the current state,
// setting FPS limits and profiles, saving profiles.
package app

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/s3r3nity-labs/mangohud-fps-switch/internal/mangohud"
	"github.com/s3r3nity-labs/mangohud-fps-switch/internal/profiles"
)

// App is config.toml and the MangoHud config it points to.
type App struct {
	Config     profiles.Config
	ConfigPath string
	// Created: config.toml didn't exist and was just written with the default profiles.
	Created bool
	// MangoHud is the MangoHud config that gets changed.
	MangoHud string
	// mangoIsStd: the global config, not chosen by mangohud_config or MANGOHUD_CONFIGFILE.
	mangoIsStd bool
	getenv     func(string) string
}

// Load reads config.toml from configPath, or the default place if it's empty. It writes
// the default profiles on the first run.
func Load(getenv func(string) string, configPath string) (*App, error) {
	a := &App{ConfigPath: configPath, getenv: getenv}
	if a.ConfigPath == "" {
		p, err := profiles.Path(getenv)
		if err != nil {
			return nil, err
		}
		a.ConfigPath = p
	}
	cfg, created, err := profiles.Load(a.ConfigPath)
	a.Created = created
	if err != nil {
		return a, err
	}
	a.Config = cfg
	return a, a.resolveMangoHud()
}

func (a *App) resolveMangoHud() error {
	if a.Config.MangoHudConfig != "" {
		a.MangoHud = ExpandHome(a.Config.MangoHudConfig, a.getenv("HOME"))
		a.mangoIsStd = false
		return nil
	}
	p, err := mangohud.DefaultPath(a.getenv)
	if err != nil {
		return err
	}
	a.MangoHud = p
	a.mangoIsStd = a.getenv("MANGOHUD_CONFIGFILE") == ""
	return nil
}

// ExpandHome turns a leading ~/ into the home folder.
func ExpandHome(p, home string) string {
	if p == "~" {
		return home
	}
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return filepath.Join(home, rest)
	}
	return p
}

// ErrNoMangoHudConfig: the MangoHud config doesn't exist. The tool doesn't create one,
// since without a file MangoHud uses its defaults, which a new file would change.
var ErrNoMangoHudConfig = errors.New("MangoHud config not found")

// Current reads MangoHud's current settings.
func (a *App) Current() (map[string]string, error) {
	values, err := mangohud.Read(a.MangoHud)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s\nCreate it (e.g. with GOverlay, or from MangoHud's example config), "+
			"or set mangohud_config in %s", ErrNoMangoHudConfig, a.MangoHud, a.ConfigPath)
	}
	return values, err
}

// Apply writes settings to MangoHud's config; a running game picks them up by itself.
func (a *App) Apply(settings map[string]string) error {
	if _, err := a.Current(); err != nil {
		return err
	}
	return mangohud.Set(a.MangoHud, settings)
}

// SetFPS sets only the FPS limit.
func (a *App) SetFPS(fps profiles.FPS) error {
	return a.Apply(map[string]string{"fps_limit": string(fps)})
}

// SetProfile sets profile i (its FPS limit and settings).
func (a *App) SetProfile(i int) error {
	if i < 0 || i >= len(a.Config.Profiles) {
		return fmt.Errorf("no profile %d", i)
	}
	return a.Apply(a.Config.Profiles[i].MangoHudSettings())
}

// Status is what MangoHud uses now.
type Status struct {
	FPSLimit profiles.FPS
	// Active is the first profile matching MangoHud's settings, -1 if none does.
	Active int
	// PerGame are configs of single games that win over the global one.
	PerGame []string
	// EnvOverride: MANGOHUD_CONFIG (of this session) sets fps_limit too, which wins.
	EnvOverride bool
}

// Status reads MangoHud's config.
func (a *App) Status() (Status, error) {
	current, err := a.Current()
	if err != nil {
		return Status{Active: -1}, err
	}
	fps := current["fps_limit"]
	if fps == "" {
		fps = "0" // not set: unlimited
	}
	if parsed, err := profiles.ParseFPS(fps); err == nil {
		fps = string(parsed)
	}
	s := Status{
		FPSLimit:    profiles.FPS(fps),
		Active:      a.Config.Active(current),
		EnvOverride: strings.Contains(a.getenv("MANGOHUD_CONFIG"), "fps_limit"),
	}
	if a.mangoIsStd {
		s.PerGame = mangohud.PerGameConfigs(a.MangoHud)
	}
	return s, nil
}

// Next is the profile cycle switches to: after the active one (back: before it), wrapping
// around; without an active one the first (back: the last).
func (a *App) Next(back bool) (int, error) {
	s, err := a.Status()
	if err != nil {
		return -1, err
	}
	n := len(a.Config.Profiles)
	switch {
	case s.Active < 0 && back:
		return n - 1, nil
	case s.Active < 0:
		return 0, nil
	case back:
		return (s.Active - 1 + n) % n, nil
	default:
		return (s.Active + 1) % n, nil
	}
}

// Save writes cfg to config.toml and uses it from now on.
func (a *App) Save(cfg profiles.Config) error {
	if err := profiles.Save(a.ConfigPath, cfg); err != nil {
		return err
	}
	a.Config = cfg
	return a.resolveMangoHud()
}
