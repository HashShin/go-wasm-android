#!/bin/bash
# web.sh — Dev server with hot reload.
# - Edit app/ui/index.html or app/ui/style.css → browser refreshes instantly
# - Edit app/go/main.go → WASM recompiles then browser refreshes
# - Open http://127.0.0.1:7000 in any browser
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

command -v go &>/dev/null || { echo "ERROR: Go not found — run 'make setup' first"; exit 1; }
export PATH="$PATH:$(go env GOPATH)/bin"

# Export splash config so the web server can inject it
source "$ROOT/app/app.conf"
export SPLASH_ENABLED SPLASH_BG_COLOR SPLASH_IMAGE_SIZE SPLASH_DURATION SPLASH_ANIMATION

# Parse -p PORT and -H HOST flags
DEV_HOST="127.0.0.1"
DEV_PORT=7000
while getopts "p:H:" opt 2>/dev/null; do
    case $opt in
        p) DEV_PORT="$OPTARG" ;;
        H) DEV_HOST="$OPTARG" ;;
    esac
done
export DEV_HOST DEV_PORT

echo "[web] Starting dev server at http://$DEV_HOST:$DEV_PORT"
# Pass project root as arg so the server can find app/ui/ and app/go/
cd "$ROOT/platform/web" && go run . "$ROOT"
