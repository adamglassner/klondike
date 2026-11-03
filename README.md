# Klondike: Gold Rush Solitaire

Classic Klondike solitaire set in the 1898 Yukon gold rush.

An Ironite game for Windows 10 and 11, written in Go: a native Win32 window, software
renderer and waveOut audio, with no browser and no cgo.

## Play

The easiest way is **[Ironite Launcher](https://github.com/adamglassner/launcher)**: it downloads
Klondike from this repository's releases, installs it, and updates it automatically when a
new version is released.

Or grab `Klondike-<version>-windows.zip` from the [latest release](../../releases/latest), unzip it
anywhere and run `Klondike.exe`.

## Build

`bash build.sh` (needs Go 1.24+, works offline - dependencies are vendored) builds
`dist/Klondike/` with the Windows executable. `cd app && go test ./...` runs the rules
and rendering tests on any OS (set `KL_SHOTS=<dir>` to save screenshots).

- `app/` - the game (Go). Art, music and sounds are embedded from `app/assets/`.
- `art-source/klondike-web.html` - the original HTML version, the source of the card art
  and the music synthesizer.
- `tools/export/` - re-exports `app/assets/` from it (Playwright + Pillow).

## Releasing

Bump the version, add a section at the top of the changelog, and push to `main`. The
[Release workflow](.github/workflows/release.yml) builds the game and publishes `v<version>` with
the zip that Ironite Launcher installs.

(c) 2026 Ironite.
