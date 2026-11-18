// Package mangohud reads and changes settings in a MangoHud config file.
//
// It follows MangoHud's own parser (src/config.cpp): everything after '#' is a comment,
// "key=value" sets a value, a bare "key" means "key=1", and the last line for a key wins.
// Changes keep every other line, comment and the file itself (written in place, so links
// stay links). A running game reloads the file by itself when it changes.
package mangohud

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// DefaultPath is the global MangoHud config: $XDG_CONFIG_HOME/MangoHud/MangoHud.conf.
// MANGOHUD_CONFIGFILE replaces it, as it does for MangoHud.
func DefaultPath(getenv func(string) string) (string, error) {
	if p := getenv("MANGOHUD_CONFIGFILE"); p != "" {
		return p, nil
	}
	dir := getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home := getenv("HOME")
		if home == "" {
			return "", errors.New("neither XDG_CONFIG_HOME nor HOME is set")
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "MangoHud", "MangoHud.conf"), nil
}

// keyPattern is what a setting name may look like (MangoHud's names: fps_limit, gpu_stats …).
var keyPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// ValidKey reports whether key is a possible MangoHud setting name.
func ValidKey(key string) bool { return keyPattern.MatchString(key) }

// Read returns the value of every key set in the file (the last line wins, a bare key is "1").
func Read(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if key, value, ok := parse(line); ok {
			values[key] = value
		}
	}
	return values, nil
}

// parse splits a line the way MangoHud does.
func parse(line string) (key, value string, ok bool) {
	if i := strings.Index(line, "#"); i >= 0 {
		line = line[:i]
	}
	key, value, found := strings.Cut(line, "=")
	if !found {
		value = "1"
	}
	key, value = strings.TrimSpace(key), strings.TrimSpace(value)
	return key, value, key != ""
}

// Set changes settings in the file: every active line of a key gets the new value (a
// comment after it stays); keys without an active line are added at the end. The file must
// exist: without one MangoHud uses its defaults, which a new file would change.
func Set(path string, settings map[string]string) error {
	for key, value := range settings {
		if !ValidKey(key) {
			return fmt.Errorf("invalid MangoHud setting name %q", key)
		}
		if strings.ContainsAny(value, "#\n\r") {
			return fmt.Errorf("invalid value for %s: %q", key, value)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(data)
	newline := "\n"
	if strings.Contains(text, "\r\n") {
		newline = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	done := map[string]bool{}
	for i, line := range lines {
		key, _, ok := parse(line)
		value, change := settings[key]
		if !ok || !change {
			continue
		}
		comment := ""
		if j := strings.Index(line, "#"); j >= 0 {
			comment = " " + line[j:]
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		lines[i] = indent + key + "=" + value + comment
		done[key] = true
	}
	var added []string
	for key := range settings {
		if !done[key] {
			added = append(added, key)
		}
	}
	slices.Sort(added) // same order every time
	if len(added) > 0 {
		if lines[len(lines)-1] == "" { // the file ends with a newline: add before it
			lines = lines[:len(lines)-1]
		}
		for _, key := range added {
			lines = append(lines, key+"="+settings[key])
		}
		lines = append(lines, "")
	}
	out := strings.Join(lines, newline)
	if out == text {
		return nil
	}
	// In place (not temp file + rename): MangoHud watches the file for changes, and a
	// linked config (dotfiles) stays a link.
	return os.WriteFile(path, []byte(out), 0o644)
}

// PerGameConfigs lists the configs next to the global one that MangoHud prefers for some
// games: <program>.conf and wine-<program>.conf. Changing the global file doesn't affect
// those games.
func PerGameConfigs(globalPath string) []string {
	if filepath.Base(globalPath) != "MangoHud.conf" {
		return nil // MANGOHUD_CONFIGFILE: MangoHud reads only that file
	}
	found, _ := filepath.Glob(filepath.Join(filepath.Dir(globalPath), "*.conf"))
	var out []string
	for _, f := range found {
		if filepath.Base(f) != "MangoHud.conf" {
			out = append(out, f)
		}
	}
	return out
}
