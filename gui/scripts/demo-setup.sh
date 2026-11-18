#!/usr/bin/env bash
# A config folder to try the window without touching your real configs: a MangoHud config
# with comments, a per-game config (shows the warning) and no config.toml yet (the app
# writes the default profiles). Then:
#   env XDG_CONFIG_HOME=/tmp/mangohud-fps-switch-demo wails3 dev
# and watch the file change: watch -n 0.5 cat /tmp/mangohud-fps-switch-demo/MangoHud/MangoHud.conf
set -euo pipefail

demo=/tmp/mangohud-fps-switch-demo
rm -rf "$demo"
mkdir -p "$demo/MangoHud"
cat >"$demo/MangoHud/MangoHud.conf" <<'EOF'
# Demo MangoHud config
legacy_layout=false
# fps_limit=0
fps_limit=60 # changed by mangohud-fps-switch
gpu_stats
cpu_stats
EOF
: >"$demo/MangoHud/wine-GenshinImpact.conf"
echo "Demo config in $demo. Run: env XDG_CONFIG_HOME=$demo wails3 dev"
