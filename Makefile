APK      := builds/app-debug.apk
APK_REL  := builds/app-release.apk

.PHONY: all apk build release keygen clean web setup icons

# ── Build ─────────────────────────────────────────────────────────────────────
all: apk

apk: build

build:
	bash scripts/build.sh

release:
	bash scripts/build.sh release

keygen:
	bash scripts/keygen.sh

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
