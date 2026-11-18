// MangoHud FPS Switch: the desktop app of mangohud-fps-switch, a window to set and edit
// the FPS profiles. Same logic as the CLI (internal/app in the parent module).
package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/s3r3nity-labs/mangohud-fps-switch/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

// The app icon, also for the window's title bar (without it, Linux shows Wails' logo).
//
//go:embed build/appicon.png
var appIcon []byte

// stateChanged tells the window to load the state again: a config file changed outside.
const stateChanged = "state-changed"

func init() {
	application.RegisterEvent[string](stateChanged)
}

func main() {
	// For test setups: MANGOHUD_FPS_SWITCH_CONFIG points to another config.toml (whose
	// mangohud_config can point to another MangoHud config)
	service := &ProfileService{configPath: os.Getenv("MANGOHUD_FPS_SWITCH_CONFIG"), getenv: os.Getenv}

	a := application.New(application.Options{
		Name:        "MangoHud FPS Switch",
		Description: "Switch MangoHud's FPS limit with profiles, also while a game is running",
		Icon:        appIcon,
		Services:    []application.Service{application.NewService(service)},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		// The Flatpak ID: desktops match the window to its menu entry and icon by it
		Linux: application.LinuxOptions{
			ApplicationID: "org.s3r3nity.MangoHudFPSSwitch",
		},
	})

	a.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "MangoHud FPS Switch",
		Width:            720,
		Height:           640,
		MinWidth:         480,
		MinHeight:        420,
		BackgroundColour: application.NewRGB(37, 37, 37), // the dark background, until the page has loaded
		URL:              "/",
	})

	stop := make(chan struct{})
	defer close(stop)
	err := watch(func() []string {
		l, err := app.Load(os.Getenv, service.configPath)
		if err != nil || l == nil {
			return nil
		}
		return []string{l.ConfigPath, l.MangoHud}
	}, func() { a.Event.Emit(stateChanged, "") }, stop)
	if err != nil {
		log.Printf("not watching the config files: %v", err)
	}

	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}
