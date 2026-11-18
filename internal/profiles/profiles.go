// Package profiles loads the FPS profiles from config.toml.
package profiles

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/s3r3nity-labs/mangohud-fps-switch/internal/mangohud"
)

// Config is config.toml.
type Config struct {
	// Notify shows a desktop notification on every change (default true).
	Notify *bool `toml:"notify"`
	// MangoHudConfig is the MangoHud config to change; empty means the global one
	// (or MANGOHUD_CONFIGFILE). A leading ~/ is the home folder.
	MangoHudConfig string    `toml:"mangohud_config"`
	Profiles       []Profile `toml:"profile"`
}

// Profile is one [[profile]]: a name, an FPS limit and optionally more MangoHud settings.
type Profile struct {
	Name     string         `toml:"name"`
	FPSLimit FPS            `toml:"fps_limit"`
	Settings map[string]any `toml:"settings"`
}

// FPS is a MangoHud fps_limit: one number or a comma-separated list (MangoHud's
// toggle_fps_limit key cycles through it). 0 is unlimited. In config.toml it may be a
// number (60, 59.94) or a string ("60,90,0").
type FPS string

var fpsPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?(,[0-9]+(\.[0-9]+)?)*$`)

// ParseFPS checks an FPS limit as typed on the command line and normalises it ("060" → "60").
func ParseFPS(s string) (FPS, error) {
	s = strings.ReplaceAll(s, " ", "")
	if !fpsPattern.MatchString(s) {
		return "", fmt.Errorf("%q is not an FPS limit: use a number like 60 (0 = unlimited), or a list like 60,90,0", s)
	}
	parts := strings.Split(s, ",")
	for i, p := range parts {
		f, _ := strconv.ParseFloat(p, 64)
		parts[i] = strconv.FormatFloat(f, 'f', -1, 64)
	}
	return FPS(strings.Join(parts, ",")), nil
}

// UnmarshalTOML accepts a number or a string.
func (f *FPS) UnmarshalTOML(v any) error {
	var s string
	switch v := v.(type) {
	case int64:
		s = strconv.FormatInt(v, 10)
	case float64:
		s = strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		s = v
	default:
		return fmt.Errorf("fps_limit must be a number or a string like \"60,90\", not %T", v)
	}
	parsed, err := ParseFPS(s)
	if err != nil {
		return err
	}
	*f = parsed
	return nil
}

// Label is how an FPS limit is shown: "60 FPS", "Unlimited", "60, 90, unlimited FPS".
func (f FPS) Label() string {
	if f == "0" {
		return "Unlimited"
	}
	parts := strings.Split(string(f), ",")
	for i, p := range parts {
		if p == "0" {
			parts[i] = "unlimited"
		}
	}
	return strings.Join(parts, ", ") + " FPS"
}

// Equal compares two fps_limit values by their numbers ("60.0" equals "60").
func Equal(a, b string) bool {
	pa, pb := strings.Split(a, ","), strings.Split(b, ",")
	if len(pa) != len(pb) {
		return false
	}
	for i := range pa {
		fa, errA := strconv.ParseFloat(strings.TrimSpace(pa[i]), 64)
		fb, errB := strconv.ParseFloat(strings.TrimSpace(pb[i]), 64)
		if errA != nil || errB != nil || math.Abs(fa-fb) > 1e-9 {
			return false
		}
	}
	return true
}

// MangoHudSettings is everything the profile writes to MangoHud's config, as text.
func (p Profile) MangoHudSettings() map[string]string {
	out := map[string]string{"fps_limit": string(p.FPSLimit)}
	for k, v := range p.Settings {
		out[k] = settingText(v)
	}
	return out
}

func settingText(v any) string {
	switch v := v.(type) {
	case bool: // MangoHud: key=0|1
		if v {
			return "1"
		}
		return "0"
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}

// Matches reports whether MangoHud's current values are this profile's.
func (p Profile) Matches(current map[string]string) bool {
	for k, want := range p.MangoHudSettings() {
		got, ok := current[k]
		if k == "fps_limit" {
			if !ok {
				got = "0" // not set: MangoHud doesn't limit
			}
			if !Equal(got, want) {
				return false
			}
			continue
		}
		if !ok || got != want {
			return false
		}
	}
	return true
}

// Path is where config.toml lives: $XDG_CONFIG_HOME/mangohud-fps-switch/config.toml.
func Path(getenv func(string) string) (string, error) {
	dir := getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home := getenv("HOME")
		if home == "" {
			return "", errors.New("neither XDG_CONFIG_HOME nor HOME is set")
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "mangohud-fps-switch", "config.toml"), nil
}

// Load reads config.toml. If it doesn't exist yet, it writes DefaultConfig first and says
// so with created.
func Load(path string) (cfg Config, created bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		text := render(DefaultConfig())
		if err := writeFile(path, text); err != nil {
			return Config{}, false, err
		}
		data, created = []byte(text), true
	} else if err != nil {
		return Config{}, false, err
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, created, fmt.Errorf("%s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, created, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, created, nil
}

// Validate checks what Load and Save need: at least one profile, unique names, valid
// values.
func (c Config) Validate() error {
	if len(c.Profiles) == 0 {
		return errors.New("no [[profile]] defined")
	}
	names := map[string]bool{}
	for i, p := range c.Profiles {
		where := fmt.Sprintf("profile %d", i)
		if p.Name != "" {
			where += fmt.Sprintf(" (%q)", p.Name)
		}
		if strings.TrimSpace(p.Name) == "" {
			return fmt.Errorf("%s: name is missing", where)
		}
		if names[strings.ToLower(p.Name)] {
			return fmt.Errorf("%s: the name is used twice", where)
		}
		names[strings.ToLower(p.Name)] = true
		if p.FPSLimit == "" {
			return fmt.Errorf("%s: fps_limit is missing", where)
		}
		for k, v := range p.Settings {
			if k == "fps_limit" {
				return fmt.Errorf("%s: set fps_limit directly, not in settings", where)
			}
			if !mangohud.ValidKey(k) {
				return fmt.Errorf("%s: %q is not a MangoHud setting name", where, k)
			}
			switch v := v.(type) {
			case bool, int64, float64:
			case string:
				if strings.ContainsAny(v, "#\n\r") {
					return fmt.Errorf("%s: settings.%s can't contain # or line breaks", where, k)
				}
			default:
				return fmt.Errorf("%s: settings.%s must be a string, number or true/false", where, k)
			}
		}
	}
	return nil
}

// ShouldNotify is the notify setting (default true).
func (c Config) ShouldNotify() bool { return c.Notify == nil || *c.Notify }

// Find returns a profile by number (counting from 0) or by name (case doesn't matter).
func (c Config) Find(arg string) (int, error) {
	if n, err := strconv.Atoi(arg); err == nil && n >= 0 && n < len(c.Profiles) {
		return n, nil
	}
	for i, p := range c.Profiles {
		if strings.EqualFold(p.Name, arg) {
			return i, nil
		}
	}
	return -1, fmt.Errorf("no profile %q: use a number from 0 to %d or a name (list them with \"profile\")",
		arg, len(c.Profiles)-1)
}

// Active returns the first profile matching MangoHud's current values, or -1.
func (c Config) Active(current map[string]string) int {
	for i, p := range c.Profiles {
		if p.Matches(current) {
			return i
		}
	}
	return -1
}
