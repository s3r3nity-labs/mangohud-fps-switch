// Package cmd holds the Cobra commands.
package cmd

import (
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/s3r3nity-labs/mangohud-fps-switch/internal/notify"
)

// Name is the program's name, also used in --version.
const Name = "mangohud-fps-switch"

// version is set at build time (-ldflags "-X .../cmd.version=v0.1.0"); otherwise it comes
// from the module version (go install ...@v0.1.0), and "dev" for a local build.
var version = ""

// Version returns the program version.
func Version() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// Notifier shows a desktop notification.
type Notifier interface {
	Notify(summary, body string) error
}

// deps is what the commands use from outside, replaced in tests.
type deps struct {
	getenv   func(string) string
	notifier Notifier
}

// options are the global flags.
type options struct {
	configPath string
	noNotify   bool
}

// NewRootCmd builds the command tree.
func NewRootCmd() *cobra.Command {
	return newRootCmd(deps{
		getenv:   os.Getenv,
		notifier: notify.Desktop{IDFile: notify.RuntimeIDFile(os.Getenv)},
	})
}

func newRootCmd(d deps) *cobra.Command {
	opts := &options{}
	root := &cobra.Command{
		Use:   Name + " <FPS>",
		Short: "Switch MangoHud's FPS limit, also while a game is running",
		Long: `Sets fps_limit in MangoHud's config. A running game picks up the change by itself
within a moment, no restart needed.

  ` + Name + ` 60          set 60 FPS (0 = unlimited, 60,90,0 = a list for Shift_L+F1)
  ` + Name + ` profile 2   set profile 2 from config.toml (or by name)
  ` + Name + ` cycle       the next profile
  ` + Name + ` status      what is set and which files are used

Bind these to keyboard shortcuts (e.g. KDE: System Settings → Shortcuts → Add New →
Command). Each change shows a short desktop notification.`,
		Example: "  " + Name + " 45\n  " + Name + " profile 0",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return setFPS(cmd, d, opts, args[0])
		},
		Version:       Version(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate(Name + " {{.Version}}\n")
	root.PersistentFlags().StringVar(&opts.configPath, "config", "",
		"profiles file (default ~/.config/"+Name+"/config.toml)")
	root.PersistentFlags().BoolVarP(&opts.noNotify, "no-notify", "n", false, "don't show a desktop notification")
	root.AddCommand(newProfileCmd(d, opts), newCycleCmd(d, opts), newStatusCmd(d, opts))
	return root
}
