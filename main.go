// mangohud-fps-switch switches MangoHud's FPS limit, also while a game is running.
package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"

	"github.com/s3r3nity-labs/mangohud-fps-switch/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, color.New(color.Bold, color.FgRed).Sprint("❌ ")+err.Error())
		os.Exit(1)
	}
}
