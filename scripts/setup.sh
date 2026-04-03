#!/bin/bash
# setup.sh — One-command setup for building Android APK.
# Supports: Native Termux (arm64 Android), Ubuntu/Debian (x86_64 or arm64).
#
# Installs everything needed to build the APK:
#   - Go (compiler + WASM support)
#   - Java 17 (required by Gradle / Android SDK)
#   - Android SDK + build-tools + platform-tools
#   - aapt2 / zipalign (native binaries via pkg on Termux)
#
# Usage:
#   bash scripts/setup.sh
set -e

# ── Config ────────────────────────────────────────────────────────────────────
GO_VERSION="1.22.5"
ANDROID_HOME="${ANDROID_HOME:-$HOME/android-sdk}"
CMDLINE_TOOLS_VERSION="11076708"
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOST_ARCH=$(uname -m)

log()  { echo ""; echo "▶ $*"; }
ok()   { echo "  ✔ $*"; }
warn() { echo "  ⚠ $*"; }

# ── Detect environment ────────────────────────────────────────────────────────
IS_TERMUX=false
if [ -n "$PREFIX" ] && echo "$PREFIX" | grep -q "com.termux"; then
    IS_TERMUX=true
fi

log "Setup — $(uname -s) / $HOST_ARCH$([ "$IS_TERMUX" = true ] && echo " / native Termux" || true)"
echo "  Project : $PROJECT_DIR"
echo "  Android : $ANDROID_HOME"

# ── 0. System packages ────────────────────────────────────────────────────────
if [ "$IS_TERMUX" = true ]; then

    log "Installing packages via pkg (Termux)..."
    pkg update -y

    command -v curl     &>/dev/null || pkg install -y curl
    command -v unzip    &>/dev/null || pkg install -y unzip
    command -v wget     &>/dev/null || pkg install -y wget
    command -v git      &>/dev/null || pkg install -y git

    # Go — use Termux package (already native arm64, no manual download needed)
    command -v go       &>/dev/null || pkg install -y golang

    # Java — required by Gradle and Android SDK manager
    command -v java     &>/dev/null || pkg install -y openjdk-17

    # Native ARM64 build tools for Android SDK
    command -v aapt2    &>/dev/null || pkg install -y aapt2
    command -v zipalign &>/dev/null || pkg install -y zipalign

    ok "Termux packages ready"

elif command -v apt-get &>/dev/null; then

    APT() {
        if [ "$(id -u)" -eq 0 ]; then apt-get "$@"; else sudo apt-get "$@"; fi
    }

    log "Checking system packages (apt)..."
    MISSING=()

    command -v curl   &>/dev/null || MISSING+=(curl)
    command -v unzip  &>/dev/null || MISSING+=(unzip)
    command -v wget   &>/dev/null || MISSING+=(wget)
    command -v git    &>/dev/null || MISSING+=(git)
    command -v file   &>/dev/null || MISSING+=(file)
    command -v tar    &>/dev/null || MISSING+=(tar)

    command -v java &>/dev/null || MISSING+=(openjdk-17-jdk-headless)

    if [ ${#MISSING[@]} -gt 0 ]; then
        log "Installing: ${MISSING[*]}"
        APT update -q
        APT install -y "${MISSING[@]}"
        ok "System packages installed"
    else
        ok "All system packages already installed"
    fi

elif command -v brew &>/dev/null; then
    log "Checking system packages (brew)..."
    command -v java  &>/dev/null || brew install openjdk@17
    command -v wget  &>/dev/null || brew install wget
    ok "Homebrew packages ready"

else
    warn "Unknown package manager — install manually: Java 17, Go, curl, unzip"
fi

ok "Java: $(java -version 2>&1 | head -1)"

# ── 1. Go ─────────────────────────────────────────────────────────────────────
log "Checking Go..."
if command -v go &>/dev/null; then
    ok "Go already installed: $(go version)"
elif [ "$IS_TERMUX" = false ]; then
    # Non-Termux: download Go from golang.dev
    log "Installing Go $GO_VERSION..."
    [ "$HOST_ARCH" = "x86_64" ] && GOARCH_DL="amd64" || GOARCH_DL="arm64"
    GO_TAR="go${GO_VERSION}.linux-${GOARCH_DL}.tar.gz"
    curl -fsSL "https://go.dev/dl/${GO_TAR}" -o "/tmp/${GO_TAR}"
    tar -C /usr/local -xzf "/tmp/${GO_TAR}"
    rm "/tmp/${GO_TAR}"

    GO_PATH_LINE='export PATH="$PATH:/usr/local/go/bin"'
    grep -qxF "$GO_PATH_LINE" ~/.bashrc  2>/dev/null || echo "$GO_PATH_LINE" >> ~/.bashrc
    grep -qxF "$GO_PATH_LINE" ~/.profile 2>/dev/null || echo "$GO_PATH_LINE" >> ~/.profile
    export PATH="$PATH:/usr/local/go/bin"
    ok "Go $GO_VERSION installed"
else
    echo "ERROR: Go not found — run: pkg install golang" && exit 1
fi

# Verify wasm_exec.js is present (needed for APK build)
WASM_EXEC="$(go env GOROOT)/lib/wasm/wasm_exec.js"
[ ! -f "$WASM_EXEC" ] && WASM_EXEC="$(go env GOROOT)/misc/wasm/wasm_exec.js"
[ -f "$WASM_EXEC" ] && ok "wasm_exec.js found" \
                    || { echo "ERROR: wasm_exec.js missing — Go install may be incomplete"; exit 1; }

# ── 2. JAVA_HOME (Termux needs explicit path) ─────────────────────────────────
if [ "$IS_TERMUX" = true ] && [ -z "$JAVA_HOME" ]; then
    # Find the JVM installed by pkg
    JVM_DIR="$PREFIX/lib/jvm"
    if [ -d "$JVM_DIR" ]; then
        JAVA_HOME_CANDIDATE=$(ls -d "$JVM_DIR"/java-* 2>/dev/null | head -1)
        if [ -n "$JAVA_HOME_CANDIDATE" ]; then
            export JAVA_HOME="$JAVA_HOME_CANDIDATE"
            JAVA_HOME_LINE="export JAVA_HOME=\"$JAVA_HOME\""
            grep -qxF "$JAVA_HOME_LINE" ~/.bashrc  2>/dev/null || echo "$JAVA_HOME_LINE" >> ~/.bashrc
            grep -qxF "$JAVA_HOME_LINE" ~/.profile 2>/dev/null || echo "$JAVA_HOME_LINE" >> ~/.profile
            ok "JAVA_HOME set to $JAVA_HOME"
        fi
    fi
fi

# ── 3. Android SDK command-line tools ─────────────────────────────────────────
log "Checking Android SDK..."
if [ ! -d "$ANDROID_HOME/cmdline-tools/latest" ]; then
    log "Downloading Android command-line tools..."
    mkdir -p "$ANDROID_HOME/cmdline-tools"
    TMP=$(mktemp -d)
    curl -fsSL \
        "https://dl.google.com/android/repository/commandlinetools-linux-${CMDLINE_TOOLS_VERSION}_latest.zip" \
        -o "$TMP/cmdtools.zip"
    unzip -q "$TMP/cmdtools.zip" -d "$TMP"
    mv "$TMP/cmdline-tools" "$ANDROID_HOME/cmdline-tools/latest"
    rm -rf "$TMP"
    ok "Android command-line tools downloaded"
else
    ok "Android command-line tools already present"
fi

SDKMANAGER="$ANDROID_HOME/cmdline-tools/latest/bin/sdkmanager"

log "Accepting Android licenses..."
yes | "$SDKMANAGER" --licenses >/dev/null 2>&1 || true

log "Installing Android SDK components..."
"$SDKMANAGER" "platform-tools" "platforms;android-34" "build-tools;34.0.0"
ok "Android SDK components ready"

# Persist Android SDK PATH
SDK_PATH_LINE="export ANDROID_HOME=\"$ANDROID_HOME\""
PT_PATH_LINE='export PATH="$PATH:$ANDROID_HOME/platform-tools"'
grep -qxF "$SDK_PATH_LINE" ~/.bashrc  2>/dev/null || echo "$SDK_PATH_LINE" >> ~/.bashrc
grep -qxF "$PT_PATH_LINE"  ~/.bashrc  2>/dev/null || echo "$PT_PATH_LINE"  >> ~/.bashrc
grep -qxF "$SDK_PATH_LINE" ~/.profile 2>/dev/null || echo "$SDK_PATH_LINE" >> ~/.profile
grep -qxF "$PT_PATH_LINE"  ~/.profile 2>/dev/null || echo "$PT_PATH_LINE"  >> ~/.profile

# Write local.properties so Gradle finds the SDK
cat > "$PROJECT_DIR/platform/android/local.properties" << EOF
sdk.dir=$ANDROID_HOME
EOF
ok "android/local.properties written"

# ── 4. ARM64: replace x86_64 SDK binaries with native ARM64 ones ──────────────
if [ "$HOST_ARCH" = "aarch64" ] || [ "$HOST_ARCH" = "arm64" ]; then
    SDK_BT="$ANDROID_HOME/build-tools/34.0.0"
    if [ "$IS_TERMUX" = true ]; then
        # In native Termux, aapt2/zipalign are already installed as native ARM64 binaries
        TERMUX_BIN="$PREFIX/bin"
    else
        TERMUX_BIN="/data/data/com.termux/files/usr/bin"
    fi
    if [ -d "$TERMUX_BIN" ]; then
        log "ARM64 host — replacing SDK x86_64 binaries with native ARM64 builds..."
        for bin in aapt aapt2 zipalign; do
            if [ -f "$TERMUX_BIN/$bin" ]; then
                cp "$TERMUX_BIN/$bin" "$SDK_BT/$bin"
                ok "Replaced $bin"
            fi
        done
    else
        warn "ARM64 bin dir not found — if APK build fails, install: pkg install aapt2 zipalign"
    fi
fi

# ── Done ──────────────────────────────────────────────────────────────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Setup complete!"
echo ""
echo "  Build commands:"
echo "    make apk              → builds/app-debug.apk"
echo "    make install          → install APK via adb"
echo "    make run              → install + launch + logcat"
echo "    make web              → http://127.0.0.1:7000 (hot reload)"
echo ""
echo "  Edit your app:"
echo "    app/go/main.go        → Go logic"
echo "    app/ui/index.html     → UI"
echo "    app/ui/style.css      → styles"
echo "    app/Icon.png          → icon"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
warn "Restart your shell (or run: source ~/.bashrc) to update PATH"
echo ""
