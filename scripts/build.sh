#!/bin/bash
# build.sh — Compile Go → WASM, copy assets, assemble APK.
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GOLIB="$ROOT/app/go"
ASSETS_SRC="$ROOT/app/ui"
ASSETS_DST="$ROOT/platform/android/app/src/main/assets"
ANDROID="$ROOT/platform/android"

export ANDROID_HOME="${ANDROID_HOME:-$HOME/android-sdk}"
# Ensure $(go env GOPATH)/bin is in PATH (for any Go tools)
export PATH="$PATH:$(go env GOPATH)/bin"

log() { echo "[build] $*"; }

# ── 0. Generate icons ─────────────────────────────────────────────────────────
bash "$ROOT/scripts/gen-icons.sh"

# ── 1. Compile Go → WASM ──────────────────────────────────────────────────────
log "Compiling Go → WebAssembly..."
mkdir -p "$ASSETS_DST"

GOOS=js GOARCH=wasm go build \
    -o "$ASSETS_DST/app.wasm" \
    "$GOLIB/main.go"

WASM_SIZE=$(du -sh "$ASSETS_DST/app.wasm" | cut -f1)
log "app.wasm built (${WASM_SIZE})"

# ── 2. Copy wasm_exec.js (Go runtime bridge for WASM) ─────────────────────────
WASM_EXEC="$(go env GOROOT)/lib/wasm/wasm_exec.js"
[ ! -f "$WASM_EXEC" ] && WASM_EXEC="$(go env GOROOT)/misc/wasm/wasm_exec.js"
cp "$WASM_EXEC" "$ASSETS_DST/wasm_exec.js"
log "wasm_exec.js copied"

# ── 3. Copy HTML/CSS assets ───────────────────────────────────────────────────
cp "$ASSETS_SRC/index.html" "$ASSETS_DST/"
cp "$ASSETS_SRC/style.css"  "$ASSETS_DST/"
log "HTML/CSS assets copied"

# ── 4. ARM64: tell AGP to use native aapt2 instead of its Maven x86_64 copy ───
# AGP 8.x downloads its own x86_64 aapt2 from Maven, ignoring the SDK's copy.
# android.aapt2FromMavenOverride points AGP at the native ARM64 binary instead.
HOST_ARCH=$(uname -m)
GRADLE_PROPS="$ANDROID/gradle.properties"
# Remove any stale override line first, then re-add for ARM64
sed -i '/android\.aapt2FromMavenOverride/d' "$GRADLE_PROPS" 2>/dev/null || true
if [ "$HOST_ARCH" = "aarch64" ] || [ "$HOST_ARCH" = "arm64" ]; then
    # Native Termux: $PREFIX/bin/aapt2 — PRoot/other: fixed Termux path
    if [ -n "$PREFIX" ] && [ -f "$PREFIX/bin/aapt2" ]; then
        NATIVE_AAPT2="$PREFIX/bin/aapt2"
    else
        NATIVE_AAPT2="/data/data/com.termux/files/usr/bin/aapt2"
    fi
    if [ -f "$NATIVE_AAPT2" ]; then
        echo "android.aapt2FromMavenOverride=$NATIVE_AAPT2" >> "$GRADLE_PROPS"
        log "ARM64: aapt2 override → $NATIVE_AAPT2"
    fi
fi

# ── 5. Assemble APK ───────────────────────────────────────────────────────────
log "Assembling APK with Gradle..."
cd "$ANDROID"
chmod +x gradlew
./gradlew assembleDebug

APK="$ANDROID/app/build/outputs/apk/debug/app-debug.apk"
if [ -f "$APK" ]; then
    mkdir -p "$ROOT/builds"
    cp "$APK" "$ROOT/builds/app-debug.apk"
    log ""
    log "BUILD SUCCESSFUL"
    log "APK : builds/app-debug.apk"
    log "Size: $(du -sh "$ROOT/builds/app-debug.apk" | cut -f1)"
else
    echo "[build] ERROR: APK not found" && exit 1
fi
