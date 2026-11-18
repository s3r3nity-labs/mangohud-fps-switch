package cmd

import (
	"fmt"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/s3r3nity-labs/mangohud-fps-switch/internal/app"
	"github.com/s3r3nity-labs/mangohud-fps-switch/internal/profiles"
)

// load reads config.toml (writing the defaults on the first run, and saying so).
func load(cmd *cobra.Command, d deps, opts *options) (*app.App, error) {
	a, err := app.Load(d.getenv, opts.configPath)
	if a != nil && a.Created {
		fmt.Fprintf(cmd.ErrOrStderr(), "Created %s with %d profiles, edit it to change them.\n",
			a.ConfigPath, len(a.Config.Profiles))
	}
	return a, err
}

// done prints what was set and shows a notification.
func done(cmd *cobra.Command, d deps, opts *options, a *app.App, fps profiles.FPS, profile string) {
	label := fps.Label()
	msg := "MangoHud: " + label
	if profile != "" {
		msg += fmt.Sprintf(" (profile %q)", profile)
	}
	fmt.Fprintln(cmd.OutOrStdout(), msg)

	if a.Config.ShouldNotify() && !opts.noNotify {
		body := ""
		if profile != "" && !strings.EqualFold(profile, label) {
			body = "Profile: " + profile
		}
		if err := d.notifier.Notify(label, body); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "No desktop notification: %v\n", err)
		}
	}
}

func setFPS(cmd *cobra.Command, d deps, opts *options, arg string) error {
	fps, err := profiles.ParseFPS(arg)
	if err != nil {
		return err
	}
	a, err := load(cmd, d, opts)
	if err != nil {
		return err
	}
	if err := a.SetFPS(fps); err != nil {
		return err
	}
	done(cmd, d, opts, a, fps, "")
	return nil
}

func setProfile(cmd *cobra.Command, d deps, opts *options, a *app.App, i int) error {
	if err := a.SetProfile(i); err != nil {
		return err
	}
	p := a.Config.Profiles[i]
	done(cmd, d, opts, a, p.FPSLimit, p.Name)
	return nil
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
			a, err := load(cmd, d, opts)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				i, err := a.Config.Find(args[0])
				if err != nil {
					return err
				}
				return setProfile(cmd, d, opts, a, i)
			}
			s, err := a.Status()
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for i, p := range a.Config.Profiles {
				mark := " "
				if i == s.Active {
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
			a, err := load(cmd, d, opts)
			if err != nil {
				return err
			}
			next, err := a.Next(back)
			if err != nil {
				return err
			}
			return setProfile(cmd, d, opts, a, next)
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
			a, err := load(cmd, d, opts)
			if err != nil {
				return err
			}
			s, err := a.Status()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "FPS limit:\t%s\n", s.FPSLimit.Label())
			if s.Active >= 0 {
				fmt.Fprintf(w, "Profile:\t%d %q\n", s.Active, a.Config.Profiles[s.Active].Name)
			} else {
				fmt.Fprintf(w, "Profile:\tnone matches\n")
			}
			fmt.Fprintf(w, "MangoHud config:\t%s\n", a.MangoHud)
			fmt.Fprintf(w, "Profiles:\t%s\n", a.ConfigPath)
			if err := w.Flush(); err != nil {
				return err
			}
			if len(s.PerGame) > 0 {
				fmt.Fprintln(out, "\nThese games have their own MangoHud config, which wins over the one above:")
				for _, g := range s.PerGame {
					fmt.Fprintln(out, "  "+g)
				}
			}
			if s.EnvOverride {
				fmt.Fprintln(out, "\nMANGOHUD_CONFIG sets fps_limit too; for games started with it, it wins over the file.")
			}
			return nil
		},
	}
}
