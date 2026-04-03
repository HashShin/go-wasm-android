APK      := builds/app-debug.apk
APK_REL  := builds/app-release.apk

.PHONY: all apk build debug release keygen clean web setup icons

# ── Build ─────────────────────────────────────────────────────────────────────
all: apk

apk: build

debug: build

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
	bash scripts/web.sh $(if $(PORT),-p $(PORT)) $(if $(HOST),-H $(HOST))

# ── Icons ─────────────────────────────────────────────────────────────────────
icons:
	bash scripts/gen-icons.sh

# ── Setup ─────────────────────────────────────────────────────────────────────
setup:
	bash scripts/setup.sh
