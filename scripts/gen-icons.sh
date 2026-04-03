#!/bin/bash
# gen-icons.sh — Generate all platform icons from app/Icon.png.
# Called automatically by build.sh and build-exe.sh.
# If app/Icon.png is missing, placeholder files are written so builds still work.
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="$PATH:$(go env GOPATH)/bin"

cd "$ROOT/platform/genicons" && go run . "$ROOT"
