#!/bin/bash
# web.sh — Dev server with hot reload.
# - Edit app/ui/index.html or app/ui/style.css → browser refreshes instantly
# - Edit app/go/main.go → WASM recompiles then browser refreshes
# - Open http://127.0.0.1:7000 in any browser
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"  # platform/web/main.go uses paths relative to project root

export PATH="$PATH:$(go env GOPATH)/bin"

# Export splash config so the web server can inject it
source "$ROOT/app/app.conf"
export SPLASH_ENABLED SPLASH_BG_COLOR SPLASH_IMAGE_SIZE SPLASH_DURATION SPLASH_ANIMATION

echo "[web] Starting dev server at http://127.0.0.1:7000"
# Pass project root as arg so the server can find app/ui/ and app/go/
cd "$ROOT/platform/web" && go run . "$ROOT"
