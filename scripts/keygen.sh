#!/bin/bash
# keygen.sh — Generate a signing keystore for release builds.
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
KEYSTORE="$ROOT/keystore.jks"
PROPS="$ROOT/platform/android/keystore.properties"

if [ -f "$KEYSTORE" ]; then
    echo "keystore.jks already exists. Delete it first to regenerate."
    exit 0
fi

echo ""
echo "▶ Generating signing keystore for release builds..."
echo ""

read -p "Key alias (default: mykey): " KEY_ALIAS
KEY_ALIAS="${KEY_ALIAS:-mykey}"

read -s -p "Keystore password (min 6 chars): " STORE_PASS
echo ""
read -s -p "Confirm password: " STORE_PASS2
echo ""

if [ "$STORE_PASS" != "$STORE_PASS2" ]; then
    echo "ERROR: Passwords do not match" && exit 1
fi

keytool -genkeypair \
    -keystore "$KEYSTORE" \
    -alias "$KEY_ALIAS" \
    -keyalg RSA \
    -keysize 2048 \
    -validity 10000 \
    -storepass "$STORE_PASS" \
    -keypass "$STORE_PASS" \
    -dname "CN=Android,O=Android,C=US" \
    -noprompt

cat > "$PROPS" << EOF
storeFile=$KEYSTORE
storePassword=$STORE_PASS
keyAlias=$KEY_ALIAS
keyPassword=$STORE_PASS
EOF

echo ""
echo "  ✔ keystore.jks created"
echo "  ✔ keystore.properties created"
echo ""
echo "  Run 'make release' to build a signed release APK."
echo "  Keep keystore.jks safe — you need it to update your app."
echo ""
