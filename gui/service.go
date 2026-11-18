package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/s3r3nity-labs/mangohud-fps-switch/internal/app"
	"github.com/s3r3nity-labs/mangohud-fps-switch/internal/profiles"
)

// ProfileService is what the window calls (Wails generates frontend/bindings from its
// exported methods). Every call reads config.toml again, so changes made with the CLI or
// an editor show up.
type ProfileService struct {
	// configPath is config.toml; empty: the default place.
	configPath string
	getenv     func(string) string
	mu         sync.Mutex
}

// Setting is one extra MangoHud setting of a profile, as text.
type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Profile is a profile as the window shows and edits it.
type Profile struct {
	Name string `json:"name"`
	// FPSLimit as typed: "60", "0", "60,90,0".
	FPSLimit string `json:"fpsLimit"`
	// Label: "60 FPS", "Unlimited".
	Label    string    `json:"label"`
	Settings []Setting `json:"settings"`
}

// State is everything the window shows.
type State struct {
	Profiles []Profile `json:"profiles"`
	// Active is the profile MangoHud uses now, -1 if none matches.
	Active int `json:"active"`
	// FPSLabel is MangoHud's current limit; empty if its config can't be read.
	FPSLabel string `json:"fpsLabel"`
	Notify   bool   `json:"notify"`
	// MangoHudConfig is config.toml's mangohud_config (empty: the default file).
	MangoHudConfig string `json:"mangoHudConfig"`
	// MangoHudPath is the file actually changed.
	MangoHudPath string `json:"mangoHudPath"`
	ConfigPath   string `json:"configPath"`
	// MangoHudMissing: the MangoHud config doesn't exist (the window explains it).
	MangoHudMissing bool `json:"mangoHudMissing"`
	// Problem is any other error reading MangoHud's config.
	Problem     string   `json:"problem"`
	PerGame     []string `json:"perGame"`
	EnvOverride bool     `json:"envOverride"`
}

// Edit is what the window saves: all profiles in their order, and the settings.
type Edit struct {
	Profiles       []Profile `json:"profiles"`
	Notify         bool      `json:"notify"`
	MangoHudConfig string    `json:"mangoHudConfig"`
}

func (s *ProfileService) load() (*app.App, error) {
	getenv := s.getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	return app.Load(getenv, s.configPath)
}

func (s *ProfileService) state(a *app.App) State {
	st := State{
		Active:         -1,
		Notify:         a.Config.ShouldNotify(),
		MangoHudConfig: a.Config.MangoHudConfig,
		MangoHudPath:   a.MangoHud,
		ConfigPath:     a.ConfigPath,
		Profiles:       make([]Profile, 0, len(a.Config.Profiles)),
	}
	for _, p := range a.Config.Profiles {
		st.Profiles = append(st.Profiles, toView(p))
	}
	status, err := a.Status()
	switch {
	case errors.Is(err, app.ErrNoMangoHudConfig):
		st.MangoHudMissing = true
	case err != nil:
		st.Problem = err.Error()
	default:
		st.Active = status.Active
		st.FPSLabel = status.FPSLimit.Label()
		st.PerGame = status.PerGame
		st.EnvOverride = status.EnvOverride
	}
	return st
}

func toView(p profiles.Profile) Profile {
	v := Profile{Name: p.Name, FPSLimit: string(p.FPSLimit), Label: p.FPSLimit.Label(), Settings: []Setting{}}
	keys := make([]string, 0, len(p.Settings))
	for k := range p.Settings {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		v.Settings = append(v.Settings, Setting{Key: k, Value: settingText(p.Settings[k])})
	}
	return v
}

// settingText shows a value the way it's typed back: true/false stay words.
func settingText(v any) string {
	switch v := v.(type) {
	case bool:
		return strconv.FormatBool(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

// fromView turns typed text back into a profile: "true"/"false" become true/false, whole
// numbers and decimals numbers, everything else text.
func fromView(v Profile) (profiles.Profile, error) {
	fps, err := profiles.ParseFPS(v.FPSLimit)
	if err != nil {
		return profiles.Profile{}, err
	}
	p := profiles.Profile{Name: strings.TrimSpace(v.Name), FPSLimit: fps}
	for _, st := range v.Settings {
		key := strings.TrimSpace(st.Key)
		if key == "" {
			continue // an empty row
		}
		if p.Settings == nil {
			p.Settings = map[string]any{}
		}
		p.Settings[key] = typed(strings.TrimSpace(st.Value))
	}
	return p, nil
}

func typed(s string) any {
	switch s {
	case "true":
		return true
	case "false":
		return false
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && !strings.ContainsAny(s, "eEnN") {
		return f
	}
	return s
}

// State returns everything the window shows.
func (s *ProfileService) State() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, err := s.load()
	if err != nil {
		return State{}, err
	}
	return s.state(a), nil
}

// UseProfile sets profile i in MangoHud's config.
func (s *ProfileService) UseProfile(i int) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, err := s.load()
	if err != nil {
		return State{}, err
	}
	if err := a.SetProfile(i); err != nil {
		return State{}, err
	}
	return s.state(a), nil
}

// UseFPS sets just an FPS limit, e.g. "72".
func (s *ProfileService) UseFPS(fps string) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	parsed, err := profiles.ParseFPS(fps)
	if err != nil {
		return State{}, err
	}
	a, err := s.load()
	if err != nil {
		return State{}, err
	}
	if err := a.SetFPS(parsed); err != nil {
		return State{}, err
	}
	return s.state(a), nil
}

// Save writes the profiles and settings to config.toml.
func (s *ProfileService) Save(e Edit) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, err := s.load()
	if err != nil {
		return State{}, err
	}
	notify := e.Notify
	cfg := profiles.Config{Notify: &notify, MangoHudConfig: strings.TrimSpace(e.MangoHudConfig)}
	for i, v := range e.Profiles {
		p, err := fromView(v)
		if err != nil {
			return State{}, errors.New("profile " + strconv.Itoa(i) + " (" + v.Name + "): " + err.Error())
		}
		cfg.Profiles = append(cfg.Profiles, p)
	}
	if err := a.Save(cfg); err != nil {
		return State{}, err
	}
	return s.state(a), nil
}

// CheckFPS says what's wrong with a typed FPS limit, or "" if it's fine (for the form).
func (s *ProfileService) CheckFPS(fps string) string {
	if _, err := profiles.ParseFPS(fps); err != nil {
		return err.Error()
	}
	return ""
}
