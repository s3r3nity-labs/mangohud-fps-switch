# MangoHud FPS Switch (desktop app)

The window of mangohud-fps-switch: click a profile to use it, add, edit, reorder and
delete profiles, set a one-off FPS limit, switch notifications and choose the MangoHud
config. Same logic as the CLI (`internal/app` in the parent module). Changes made by
keyboard shortcuts (the CLI) or an editor show up by themselves: the app watches both
config files.

Built with [Wails 3](https://v3.wails.io) (Go backend, system WebView), Svelte 5,
Tailwind CSS 4 and [shadcn-svelte](https://shadcn-svelte.com), like Home Sync.

**Tray icon:** the profiles in its menu (click one to use it, the active one is marked),
Open window and Quit; left click shows or hides the window; hovering shows the current
limit. Closing the window keeps the app in the tray. `--hidden` starts with just the tray
icon (for autostart). Starting it again shows the running one's window. On desktops
without tray icons (GNOME without the AppIndicator extension) closing the window quits, as
it would otherwise run on invisibly.

**Start with the desktop:** a switch in the window. As a Flatpak the app asks the desktop
through the Background portal (which writes the autostart entry); otherwise it writes
`~/.config/autostart/org.s3r3nity.MangoHudFPSSwitch.desktop` itself, starting `--hidden`.

**Flatpak:** `build/flatpak/` (manifest, desktop file, AppStream metadata), built by
`.github/workflows/flatpak.yml`: on pull requests unsigned as a downloadable artifact, on
releases signed into the s3r3nity-labs Flatpak repository. Inside the sandbox
`XDG_CONFIG_HOME` is the app's own folder, so the app uses the real `~/.config` (see
`hostEnv`); it may only touch `~/.config/MangoHud` and `~/.config/mangohud-fps-switch`.

## Development

Needs Go, Node 22+ (26 tested), and GTK4 and WebKitGTK 6.0 with headers
(Ubuntu: `libgtk-4-dev libwebkitgtk-6.0-dev`, Arch: `gtk4 webkitgtk-6.0`). Then:

```fish
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
wails3 dev         # runs the app with hot reload
wails3 build       # bin/mangohud-fps-switch-gui
```

To try it without touching your real configs:

```fish
scripts/demo-setup.sh
env XDG_CONFIG_HOME=/tmp/mangohud-fps-switch-demo wails3 dev
```

`MANGOHUD_FPS_SWITCH_CONFIG` points the app at another config.toml.

- `service.go` – `ProfileService`, the Go methods the window calls; `wails3 build`/`dev`
  turn them into `frontend/bindings`
- `watch.go` – watches config.toml and the MangoHud config, tells the window and the
  tray to reload
- `tray.go` – the tray icon: `trayMenu` decides the menu's lines (tested), `tray` builds
  it with Wails; `build/tray.png` is the icon at 64 px
- `frontend/src/App.svelte` – the window; `frontend/src/lib/ProfileDialog.svelte` – add/edit
  a profile; `frontend/src/lib/components/ui` – shadcn-svelte components
- `build/appicon.svg` – the icon; `wails3 build` makes the PNG-based ones from
  `build/appicon.png` (render it and the tray icon after changing the SVG:
  `inkscape build/appicon.svg --export-type=png --export-filename=build/appicon.png -w 1024 -h 1024`,
  the same with `build/tray.png` and `-w 64 -h 64`)
- `go.mod` uses the parent module through `replace … => ../`, so the CLI's `go.mod` stays
  free of Wails and cgo

Saving from the window rewrites config.toml with its explaining comments; comments of your
own in that file are not kept.
