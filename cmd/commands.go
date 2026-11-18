package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/s3r3nity-labs/mangohud-fps-switch/internal/mangohud"
	"github.com/s3r3nity-labs/mangohud-fps-switch/internal/profiles"
)

// env is everything a command works with, loaded once per run.
type env struct {
	cfg        profiles.Config
	cfgPath    string
	mango      string // MangoHud config to change
	mangoIsStd bool   // the global one, not chosen by mangohud_config or MANGOHUD_CONFIGFILE
}

func load(cmd *cobra.Command, d deps, opts *options) (*env, error) {
	e := &env{cfgPath: opts.configPath}
	if e.cfgPath == "" {
		p, err := profiles.Path(d.getenv)
		if err != nil {
			return nil, err
		}
		e.cfgPath = p
	}
	cfg, created, err := profiles.Load(e.cfgPath)
	if created {
		fmt.Fprintf(cmd.ErrOrStderr(), "Created %s with %d profiles, edit it to change them.\n",
			e.cfgPath, len(cfg.Profiles))
	}
	if err != nil {
		return nil, err
	}
	e.cfg = cfg

	switch {
	case cfg.MangoHudConfig != "":
		e.mango = expandHome(cfg.MangoHudConfig, d.getenv("HOME"))
	default:
		p, err := mangohud.DefaultPath(d.getenv)
		if err != nil {
			return nil, err
		}
		e.mango = p
		e.mangoIsStd = d.getenv("MANGOHUD_CONFIGFILE") == ""
	}
	return e, nil
}

func expandHome(p, home string) string {
	if p == "~" {
		return home
	}
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return filepath.Join(home, rest)
	}
	return p
}

// current reads MangoHud's config, with a helpful error if it's missing.
func (e *env) current() (map[string]string, error) {
	values, err := mangohud.Read(e.mango)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("MangoHud config not found: %s\n"+
			"Create it (e.g. with GOverlay, or from MangoHud's example config), or set "+
			"mangohud_config in %s", e.mango, e.cfgPath)
	}
	return values, err
}

// apply writes settings to MangoHud's config, prints what happened and notifies.
func apply(cmd *cobra.Command, d deps, opts *options, e *env, settings map[string]string, profile string) error {
	if _, err := e.current(); err != nil {
		return err
	}
	if err := mangohud.Set(e.mango, settings); err != nil {
		return err
	}
	label := profiles.FPS(settings["fps_limit"]).Label()
	msg := "MangoHud: " + label
	if profile != "" {
		msg += fmt.Sprintf(" (profile %q)", profile)
	}
	fmt.Fprintln(cmd.OutOrStdout(), msg)

	if e.cfg.ShouldNotify() && !opts.noNotify {
		body := ""
		if profile != "" && !strings.EqualFold(profile, label) {
			body = "Profile: " + profile
		}
		if err := d.notifier.Notify(label, body); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "No desktop notification: %v\n", err)
		}
	}
	return nil
}

func setFPS(cmd *cobra.Command, d deps, opts *options, arg string) error {
	fps, err := profiles.ParseFPS(arg)
	if err != nil {
		return err
	}
	e, err := load(cmd, d, opts)
	if err != nil {
		return err
	}
	return apply(cmd, d, opts, e, map[string]string{"fps_limit": string(fps)}, "")
}

func setProfile(cmd *cobra.Command, d deps, opts *options, e *env, i int) error {
	p := e.cfg.Profiles[i]
	return apply(cmd, d, opts, e, p.MangoHudSettings(), p.Name)
}

func newProfileCmd(d deps, opts *options) *cobra.Command {
	return &cobra.Command{
		Use:     "profile [number|name]",
		Aliases: []string{"profiles", "p"},
		Short:   "Set a profile from config.toml, or list them",
		Long: `Sets a profile by its number (counting from 0, in config.toml's order) or name.
Without one, lists the profiles; * marks the one MangoHud uses now.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := load(cmd, d, opts)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				i, err := e.cfg.Find(args[0])
				if err != nil {
					return err
				}
				return setProfile(cmd, d, opts, e, i)
			}
			current, err := e.current()
			if err != nil {
				return err
			}
			active := e.cfg.Active(current)
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for i, p := range e.cfg.Profiles {
				mark := " "
				if i == active {
					mark = "*"
				}
				fmt.Fprintf(w, "%s %d\t%s\t%s\n", mark, i, p.Name, describe(p))
			}
			return w.Flush()
		},
	}
}

// describe lists a profile's MangoHud settings: "fps_limit=60 fps_limit_method=early".
func describe(p profiles.Profile) string {
	s := p.MangoHudSettings()
	parts := []string{"fps_limit=" + s["fps_limit"]}
	delete(s, "fps_limit")
	var rest []string
	for k, v := range s {
		rest = append(rest, k+"="+v)
	}
	slices.Sort(rest)
	return strings.Join(append(parts, rest...), " ")
}

func newCycleCmd(d deps, opts *options) *cobra.Command {
	var back bool
	cmd := &cobra.Command{
		Use:   "cycle",
		Short: "Set the next profile (after the last one: the first)",
		Long: `Sets the profile after the one MangoHud uses now. If the current settings match no
profile, starts with the first one (with --back: the last one).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, err := load(cmd, d, opts)
			if err != nil {
				return err
			}
			current, err := e.current()
			if err != nil {
				return err
			}
			n := len(e.cfg.Profiles)
			next := 0
			switch active := e.cfg.Active(current); {
			case active < 0 && back:
				next = n - 1
			case active < 0:
				next = 0
			case back:
				next = (active - 1 + n) % n
			default:
				next = (active + 1) % n
			}
			return setProfile(cmd, d, opts, e, next)
		},
	}
	cmd.Flags().BoolVarP(&back, "back", "b", false, "the previous profile instead")
	return cmd
}

func newStatusCmd(d deps, opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current FPS limit, the profile and the files used",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, err := load(cmd, d, opts)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			current, err := e.current()
			if err != nil {
				return err
			}
			fps, ok := current["fps_limit"]
			if !ok {
				fps = "0"
			}
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "FPS limit:\t%s\n", profiles.FPS(normalize(fps)).Label())
			if i := e.cfg.Active(current); i >= 0 {
				fmt.Fprintf(w, "Profile:\t%d %q\n", i, e.cfg.Profiles[i].Name)
			} else {
				fmt.Fprintf(w, "Profile:\tnone matches\n")
			}
			fmt.Fprintf(w, "MangoHud config:\t%s\n", e.mango)
			fmt.Fprintf(w, "Profiles:\t%s\n", e.cfgPath)
			if err := w.Flush(); err != nil {
				return err
			}
			if e.mangoIsStd {
				if games := mangohud.PerGameConfigs(e.mango); len(games) > 0 {
					fmt.Fprintln(out, "\nThese games have their own MangoHud config, which wins over the one above:")
					for _, g := range games {
						fmt.Fprintln(out, "  "+g)
					}
				}
			}
			if v := d.getenv("MANGOHUD_CONFIG"); strings.Contains(v, "fps_limit") {
				fmt.Fprintln(out, "\nMANGOHUD_CONFIG sets fps_limit too; for games started with it, it wins over the file.")
			}
			return nil
		},
	}
}

// normalize shows an fps_limit from MangoHud's file the way profiles do, if it can.
func normalize(s string) string {
	if f, err := profiles.ParseFPS(s); err == nil {
		return string(f)
	}
	return s
}
