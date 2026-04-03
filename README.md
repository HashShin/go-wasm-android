# go-wasm-android

> Go + WebAssembly → Android APK. Write your app in Go and HTML/CSS, build and deploy directly on your Android device.

---

## How it works

Your Go logic compiles to a `.wasm` binary. A plain HTML/CSS frontend loads and runs it via the WebAssembly runtime. The whole thing is packaged as an Android APK and rendered inside a native WebView — no Kotlin or Java logic required.

```
app/go/main.go      →  GOOS=js GOARCH=wasm  →  app.wasm
app/ui/index.html   ─┐
app/ui/style.css    ─┤  copied into Android assets  →  APK
app.wasm            ─┘
```

---

## Project structure

```
app/
  go/main.go          # Your Go logic (compiled to WASM)
  ui/index.html       # Frontend UI
  ui/style.css        # Styles
  Icon.png            # App icon (all densities generated from this)

platform/
  android/            # Android Gradle project
  genicons/           # Icon generator (stdlib only)
  web/                # Hot-reload dev server

scripts/
  setup.sh            # One-command toolchain installer
  build.sh            # Build APK
  gen-icons.sh        # Regenerate icons from app/Icon.png
  web.sh              # Start dev server
```

---

## Requirements

| Tool | Version |
|------|---------|
| Go | 1.22+ |
| Java | 17+ |
| Android SDK | API 34 |

---

## Setup

### Android device (native Termux)

Install [Termux](https://f-droid.org/packages/com.termux/) from F-Droid, then run this one-liner:

```bash
curl -fsSL https://raw.githubusercontent.com/HashShin/go-wasm-android/main/install.sh | bash
```

Installs Go, Java, aapt2, zipalign, and the Android SDK automatically via `pkg`. No PRoot or Linux emulation needed.

### Linux (Ubuntu / Debian)

```bash
curl -fsSL https://raw.githubusercontent.com/HashShin/go-wasm-android/main/install.sh | bash
```

---

## Build

```bash
make apk
```

Output: `builds/app-debug.apk`

---

## Development

Start the hot-reload dev server and open `http://127.0.0.1:7000` in a browser. Changes to Go, HTML, or CSS are picked up automatically.

```bash
make web
```

---

## Customise

| File | What to change |
|------|---------------|
| `app/go/main.go` | Go logic — functions exposed to the frontend |
| `app/ui/index.html` | UI markup |
| `app/ui/style.css` | Styles |
| `app/Icon.png` | App icon — run `make icons` to regenerate all sizes |
| `platform/android/app/build.gradle` | App ID, SDK versions |

---

## License

MIT
