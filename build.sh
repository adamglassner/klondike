#!/usr/bin/env bash
# Builds Klondike.exe for Windows (64-bit). Output: dist/Klondike/
# Needs Go 1.24+. Works offline (everything is vendored).
# The version comes from the first entry in CHANGELOG.md.
set -euo pipefail
cd "$(dirname "$0")"
ROOT=$PWD
VERSION=$(grep -m1 -oE '^## \[[0-9.]+' CHANGELOG.md | tr -d '#[ ')
OUT=$ROOT/dist/Klondike; B=$ROOT/build
rm -rf "$ROOT/dist" "$B" && mkdir -p "$OUT" "$B"

( cd tools/mkres && GOFLAGS=-mod=vendor go build -o "$B/mkres" . )
cp -r app/. "$B/" && cp CHANGELOG.md "$B/assets/CHANGELOG.md"
"$B/mkres" -version "$VERSION" -icon assets/icon-1024.png -out "$B/rsrc_windows_amd64.syso" \
  -name "Klondike" -exe Klondike.exe -internal Klondike -identity Ironite.Klondike -desc "Klondike - Gold Rush Solitaire"
( cd "$B" && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -mod=vendor -trimpath \
    -ldflags "-H windowsgui -s -w -X main.version=$VERSION" -o "$OUT/Klondike.exe" . )

crlf() { sed 's/\r$//; s/$/\r/' "$1" > "$2"; }
crlf CHANGELOG.md "$OUT/CHANGELOG.txt"
for f in README CREDITS; do sed "s/@VERSION@/$VERSION/g" "$f.txt" > "$B/$f.txt" && crlf "$B/$f.txt" "$OUT/$f.txt"; done
printf '%s\r\n' "$VERSION" > "$OUT/VERSION"
rm -rf "$B"
echo "Klondike $VERSION -> $OUT/Klondike.exe"
