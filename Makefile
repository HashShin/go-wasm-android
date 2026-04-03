APK := builds/app-debug.apk

.PHONY: all apk build clean install run web setup icons

# ── Build ─────────────────────────────────────────────────────────────────────
all: apk

apk: build

build:
	bash scripts/build.sh

clean:
	rm -rf platform/android/app/src/main/assets platform/android/app/build builds

install: apk
	adb install -r $(APK)

run: install
	adb shell am start -n com.gowebapp.app/.MainActivity
	adb logcat -s GoWebApp:V AndroidRuntime:E *:S

# ── Dev web server ────────────────────────────────────────────────────────────
web:
	bash scripts/web.sh

# ── Icons ─────────────────────────────────────────────────────────────────────
icons:
	bash scripts/gen-icons.sh

# ── Setup ─────────────────────────────────────────────────────────────────────
setup:
	bash scripts/setup.sh
