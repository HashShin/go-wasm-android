#!/bin/bash
set -e

REPO="https://github.com/HashShin/go-wasm-android"
DEST="$HOME/go-wasm-android"

# Install git if missing
if ! command -v git &>/dev/null; then
    if command -v pkg &>/dev/null; then
        pkg install -y git
    elif command -v apt-get &>/dev/null; then
        if [ "$(id -u)" -eq 0 ]; then apt-get install -y git; else sudo apt-get install -y git; fi
    else
        echo "ERROR: install git manually then re-run" && exit 1
    fi
fi

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
