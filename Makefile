APK := builds/app-debug.apk

.PHONY: all apk build clean web setup icons

# ── Build ─────────────────────────────────────────────────────────────────────
all: apk

apk: build

build:
	bash scripts/build.sh

clean:
	rm -rf platform/android/app/src/main/assets platform/android/app/build builds

# ── Dev web server ────────────────────────────────────────────────────────────
web:
	bash scripts/web.sh

# ── Icons ─────────────────────────────────────────────────────────────────────
icons:
	bash scripts/gen-icons.sh

# ── Setup ─────────────────────────────────────────────────────────────────────
setup:
	bash scripts/setup.sh
