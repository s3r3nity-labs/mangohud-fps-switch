# mangohud-fps-switch

Switches [MangoHud](https://github.com/flightlessmango/MangoHud)'s FPS limit on Linux,
also **while a game is running**: MangoHud notices the changed config file and applies
it within a moment. Profiles for keyboard shortcuts, a desktop notification on every
change.

```fish
mangohud-fps-switch 60          # 60 FPS (0 = unlimited)
mangohud-fps-switch profile 1   # profile 1 from config.toml (or its name)
mangohud-fps-switch cycle       # the next profile (--back: the previous one)
mangohud-fps-switch profile     # list the profiles, * = active
mangohud-fps-switch status      # current limit, profile and the files used
```

An independent tool for MangoHud, not part of the MangoHud project.

## Installation

With [Homebrew](https://brew.sh) (e.g. on Bazzite), updated by `brew upgrade`:

```fish
brew install s3r3nity-labs/tap/mangohud-fps-switch
```

Or without Homebrew, from the latest release (one static binary):

```fish
curl -sL https://github.com/s3r3nity-labs/mangohud-fps-switch/releases/latest/download/mangohud-fps-switch_linux_amd64.tar.gz \
  | tar -xz -C ~/.local/bin mangohud-fps-switch
```

## Desktop app

**MangoHud FPS Switch** puts the profiles in the tray (click one to use it) and has a
window to add, edit and order them. It can start with the desktop, in the tray. It
uses the same `config.toml` as the command, so shortcuts and app stay in step. Install it
from the [s3r3nity-labs Flatpak repository](https://s3r3nity-labs.github.io/flatpak/):

```fish
flatpak install --user https://s3r3nity-labs.github.io/flatpak/org.s3r3nity.MangoHudFPSSwitch.flatpakref
```

The Flatpak contains the command too:
`flatpak run --command=mangohud-fps-switch org.s3r3nity.MangoHudFPSSwitch profile 1`.
It can only change the MangoHud config in `~/.config/MangoHud`, so a `mangohud_config`
somewhere else needs the Homebrew version. Source and development: [gui/](gui/).

## Keyboard shortcuts

Bind the profiles to keys, e.g. in KDE Plasma: *System Settings → Keyboard → Shortcuts →
Add New → Command or Script*:

| Shortcut | Command |
| --- | --- |
| Meta+Alt+Num0 | `mangohud-fps-switch profile 0` |
| Meta+Alt+Num1 | `mangohud-fps-switch profile 1` |
| Meta+Alt+Num2 | `mangohud-fps-switch profile 2` |
| Meta+Alt+Num3 | `mangohud-fps-switch profile 3` |

Installed with Homebrew, the shortcut may not know brew's `PATH`: then use the full path,
`/home/linuxbrew/.linuxbrew/bin/mangohud-fps-switch`.

## Profiles

The first run writes `~/.config/mangohud-fps-switch/config.toml` with four profiles
(numbers count from 0, in the file's order):

```toml
notify = true   # desktop notification on every change

[[profile]]
name = "Unlimited"
fps_limit = 0

[[profile]]
name = "45 FPS"
fps_limit = 45

[[profile]]
name = "60 FPS"
fps_limit = 60

[[profile]]
name = "90 FPS"
fps_limit = 90
```

- `fps_limit`: a number (`0` = unlimited, `59.94` works too), or a list like `"60,90,0"`
  that MangoHud's own `toggle_fps_limit` key (Shift_L+F1) cycles through.
- `settings` (optional): any other MangoHud setting the profile sets as well, e.g.
  `settings = { fps_limit_method = "early", show_fps_limit = true }`. `true`/`false`
  become MangoHud's `1`/`0`.
- `mangohud_config` (optional, top level): which MangoHud config to change, e.g.
  `"~/.config/MangoHud/MangoHud.conf"`.

## How it changes MangoHud's config

- The config is `~/.config/MangoHud/MangoHud.conf`, or the file in `MANGOHUD_CONFIGFILE`,
  or `mangohud_config` from config.toml. It must exist; the tool doesn't create it.
- Only the lines of the changed settings are rewritten; comments and everything else stay.
  A missing setting is added at the end.
- The file is written in place, so a linked config (dotfiles) stays a link.

Things that win over the global config, which `status` points out:

- **Per-game configs** like `~/.config/MangoHud/wine-GenshinImpact.conf`: MangoHud uses
  only one config file per game, the most specific one.
- **`MANGOHUD_CONFIG`** with an `fps_limit` in a game's launch options.

In Bazzite's / SteamOS' Gaming Mode, the frame limit comes from gamescope, not MangoHud:
this tool is for the desktop.

## License

MIT
