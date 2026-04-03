#!/bin/bash
set -e

REPO="https://github.com/HashShin/go-wasm-android"
DEST="$HOME/go-wasm-android"

# Install git if missing
command -v git &>/dev/null || pkg install -y git

# Clone or update
if [ -d "$DEST/.git" ]; then
    echo "▶ Updating existing repo..."
    git -C "$DEST" pull
else
    echo "▶ Cloning $REPO..."
    git clone "$REPO" "$DEST"
fi

cd "$DEST"
bash scripts/setup.sh
