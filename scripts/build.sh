#!/bin/bash
# build.sh — Compile Go → WASM, copy assets, assemble APK.
# Usage: bash scripts/build.sh [release]
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GOLIB="$ROOT/app/go"
ASSETS_SRC="$ROOT/app/ui"
ASSETS_DST="$ROOT/platform/android/app/src/main/assets"
ANDROID="$ROOT/platform/android"
BUILD_TYPE="${1:-debug}"

export ANDROID_HOME="${ANDROID_HOME:-$HOME/android-sdk}"
export PATH="$PATH:$(go env GOPATH)/bin"

log() { echo "[build] $*"; }

# ── 0. Load app config ────────────────────────────────────────────────────────
source "$ROOT/app/app.conf"
log "App: $APP_NAME ($APP_ID) v$VERSION_NAME ($VERSION_CODE)"

# Apply config to Android files
sed -i "s|<string name=\"app_name\">.*</string>|<string name=\"app_name\">$APP_NAME</string>|" \
    "$ANDROID/app/src/main/res/values/strings.xml"
sed -i "s|applicationId \".*\"|applicationId \"$APP_ID\"|" \
    "$ANDROID/app/build.gradle"
sed -i "s|namespace '.*'|namespace '$APP_ID'|" \
    "$ANDROID/app/build.gradle"
sed -i "s|versionCode [0-9]*|versionCode $VERSION_CODE|" \
    "$ANDROID/app/build.gradle"
sed -i "s|versionName \".*\"|versionName \"$VERSION_NAME\"|" \
    "$ANDROID/app/build.gradle"

# ── 1. Generate splash.xml resource ──────────────────────────────────────────
mkdir -p "$ANDROID/app/src/main/res/values"
cat > "$ANDROID/app/src/main/res/values/splash.xml" << EOF
<?xml version="1.0" encoding="utf-8"?>
<resources>
    <bool name="splash_enabled">$SPLASH_ENABLED</bool>
    <color name="splash_background">$SPLASH_BG_COLOR</color>
    <dimen name="splash_image_size">${SPLASH_IMAGE_SIZE}dp</dimen>
    <integer name="splash_duration">$SPLASH_DURATION</integer>
    <bool name="splash_animation">$SPLASH_ANIMATION</bool>
</resources>
EOF
log "Splash: enabled=$SPLASH_ENABLED bg=$SPLASH_BG_COLOR size=${SPLASH_IMAGE_SIZE}dp duration=${SPLASH_DURATION}ms animation=$SPLASH_ANIMATION"

# ── 2. Generate icons ─────────────────────────────────────────────────────────
bash "$ROOT/scripts/gen-icons.sh"

# ── 3. Compile Go → WASM ──────────────────────────────────────────────────────
log "Compiling Go → WebAssembly..."
mkdir -p "$ASSETS_DST"

GOOS=js GOARCH=wasm go build \
    -o "$ASSETS_DST/app.wasm" \
    "$GOLIB/main.go"

WASM_SIZE=$(du -sh "$ASSETS_DST/app.wasm" | cut -f1)
log "app.wasm built (${WASM_SIZE})"

# ── 3. Copy wasm_exec.js ──────────────────────────────────────────────────────
WASM_EXEC="$(go env GOROOT)/lib/wasm/wasm_exec.js"
[ ! -f "$WASM_EXEC" ] && WASM_EXEC="$(go env GOROOT)/misc/wasm/wasm_exec.js"
cp "$WASM_EXEC" "$ASSETS_DST/wasm_exec.js"
log "wasm_exec.js copied"

# ── 4. Copy HTML/CSS assets ───────────────────────────────────────────────────
cp "$ASSETS_SRC/index.html" "$ASSETS_DST/"
cp "$ASSETS_SRC/style.css"  "$ASSETS_DST/"
# Splash image: use app/splash.png if present, otherwise fall back to app/Icon.png
DRAWABLE_DST="$ANDROID/app/src/main/res/drawable"
mkdir -p "$DRAWABLE_DST"
if [ -f "$ROOT/app/splash.png" ]; then
    cp "$ROOT/app/splash.png" "$ASSETS_DST/splash_image.png"
    cp "$ROOT/app/splash.png" "$DRAWABLE_DST/splash_image.png"
    log "Splash image: app/splash.png"
elif [ -f "$ROOT/app/Icon.png" ]; then
    cp "$ROOT/app/Icon.png" "$ASSETS_DST/splash_image.png"
    cp "$ROOT/app/Icon.png" "$DRAWABLE_DST/splash_image.png"
    log "Splash image: app/Icon.png (fallback)"
fi
[ -f "$ROOT/app/splash.html" ] && cp "$ROOT/app/splash.html" "$ASSETS_DST/" && log "splash.html copied"
log "HTML/CSS assets copied"

# ── 5. ARM64: native aapt2 override ──────────────────────────────────────────
HOST_ARCH=$(uname -m)
GRADLE_PROPS="$ANDROID/gradle.properties"
sed -i '/android\.aapt2FromMavenOverride/d' "$GRADLE_PROPS" 2>/dev/null || true
if [ "$HOST_ARCH" = "aarch64" ] || [ "$HOST_ARCH" = "arm64" ]; then
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

# ── 6. Assemble APK ───────────────────────────────────────────────────────────
log "Assembling $BUILD_TYPE APK with Gradle..."
cd "$ANDROID"
chmod +x gradlew

if [ "$BUILD_TYPE" = "release" ]; then
    if [ ! -f "$ANDROID/keystore.properties" ]; then
        echo "[build] ERROR: keystore.properties not found — run 'make keygen' first"
        exit 1
    fi
    ./gradlew assembleRelease
    APK="$ANDROID/app/build/outputs/apk/release/app-release.apk"
    OUT="$ROOT/builds/app-release.apk"
else
    ./gradlew assembleDebug
    APK="$ANDROID/app/build/outputs/apk/debug/app-debug.apk"
    OUT="$ROOT/builds/app-debug.apk"
fi

if [ -f "$APK" ]; then
    mkdir -p "$ROOT/builds"
    cp "$APK" "$OUT"
    log ""
    log "BUILD SUCCESSFUL"
    log "APK : $OUT"
    log "Size: $(du -sh "$OUT" | cut -f1)"
else
    echo "[build] ERROR: APK not found" && exit 1
fi
