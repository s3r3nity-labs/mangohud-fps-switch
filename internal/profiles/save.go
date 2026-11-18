package profiles

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// header starts every config.toml.
const header = `# mangohud-fps-switch profiles. Switch with:
#   mangohud-fps-switch profile <number or name>   (numbers count from 0, in this order)
#   mangohud-fps-switch cycle                      (the next one)
#
# fps_limit: a number (0 = unlimited) or a list like "60,90,0" that MangoHud's
# toggle_fps_limit key (Shift_L+F1) cycles through.
# settings: optional, any other MangoHud setting, e.g.
#   settings = { fps_limit_method = "early", show_fps_limit = true }
`

// Save writes cfg to path (after Validate). Comments of your own in the file are not kept.
func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	return writeFile(path, render(cfg))
}

// render writes config.toml's text itself rather than through a TOML encoder, to keep the
// explaining comments and a readable order.
func render(cfg Config) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n# Show a desktop notification on every change.\n")
	fmt.Fprintf(&b, "notify = %t\n", cfg.ShouldNotify())
	b.WriteString("\n# The MangoHud config to change. Default: ~/.config/MangoHud/MangoHud.conf\n")
	b.WriteString("# (or MANGOHUD_CONFIGFILE, if set).\n")
	if cfg.MangoHudConfig != "" {
		fmt.Fprintf(&b, "mangohud_config = %s\n", quote(cfg.MangoHudConfig))
	} else {
		b.WriteString("# mangohud_config = \"~/.config/MangoHud/MangoHud.conf\"\n")
	}
	for _, p := range cfg.Profiles {
		b.WriteString("\n[[profile]]\n")
		fmt.Fprintf(&b, "name = %s\n", quote(p.Name))
		if strings.Contains(string(p.FPSLimit), ",") {
			fmt.Fprintf(&b, "fps_limit = %s\n", quote(string(p.FPSLimit)))
		} else {
			fmt.Fprintf(&b, "fps_limit = %s\n", p.FPSLimit)
		}
		if len(p.Settings) > 0 {
			keys := make([]string, 0, len(p.Settings))
			for k := range p.Settings {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			parts := make([]string, len(keys))
			for i, k := range keys {
				parts[i] = k + " = " + value(p.Settings[k])
			}
			fmt.Fprintf(&b, "settings = { %s }\n", strings.Join(parts, ", "))
		}
	}
	return b.String()
}

func writeFile(path, text string) error {
	// A linked config.toml (dotfiles) stays a link: replace the file it points to
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Temp file + rename: a crash mid-write never leaves half a config
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.toml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func value(v any) string {
	switch v := v.(type) {
	case string:
		return quote(v)
	case bool:
		return strconv.FormatBool(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		s := strconv.FormatFloat(v, 'f', -1, 64)
		if !strings.Contains(s, ".") {
			s += ".0" // stays a float in TOML
		}
		return s
	default:
		return quote(fmt.Sprint(v))
	}
}

// quote writes a TOML basic string: backslash, quote and control characters escaped.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// DefaultConfig is written on the first run.
func DefaultConfig() Config {
	on := true
	return Config{Notify: &on, Profiles: []Profile{
		{Name: "Unlimited", FPSLimit: "0"},
		{Name: "45 FPS", FPSLimit: "45"},
		{Name: "60 FPS", FPSLimit: "60"},
		{Name: "90 FPS", FPSLimit: "90"},
	}}
}
