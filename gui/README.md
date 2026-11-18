# MangoHud FPS Switch (desktop app)

The window of mangohud-fps-switch: click a profile to use it, add, edit, reorder and
delete profiles, set a one-off FPS limit, switch notifications and choose the MangoHud
config. Same logic as the CLI (`internal/app` in the parent module). Changes made by
keyboard shortcuts (the CLI) or an editor show up by themselves: the app watches both
config files.

Built with [Wails 3](https://v3.wails.io) (Go backend, system WebView), Svelte 5,
Tailwind CSS 4 and [shadcn-svelte](https://shadcn-svelte.com), like Home Sync.

**Status:** the window. Next: tray icon, autostart, Flatpak.

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
- `watch.go` – watches config.toml and the MangoHud config, tells the window to reload
- `frontend/src/App.svelte` – the window; `frontend/src/lib/ProfileDialog.svelte` – add/edit
  a profile; `frontend/src/lib/components/ui` – shadcn-svelte components
- `build/appicon.svg` – the icon; `wails3 build` makes the PNG-based ones from
  `build/appicon.png` (render it after changing the SVG:
  `inkscape build/appicon.svg --export-type=png --export-filename=build/appicon.png -w 1024 -h 1024`)
- `go.mod` uses the parent module through `replace … => ../`, so the CLI's `go.mod` stays
  free of Wails and cgo

Saving from the window rewrites config.toml with its explaining comments; comments of your
own in that file are not kept.
