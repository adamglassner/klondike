# Klondike: Gold Rush Solitaire

Classic Klondike solitaire set in the 1898 Yukon gold rush.

An Ironite game for Windows 10 and 11.

## Play

The easiest way is **[Ironite Launcher](https://github.com/adamglassner/launcher)**: it downloads
Klondike from this repository's releases, installs it, and updates it automatically when a
new version is released.

Or grab `Klondike-<version>-windows.zip` from the [latest release](../../releases/latest), unzip it
anywhere and run `Klondike.exe`.

## Build

`bash build.sh` (needs Go 1.22+) builds `dist/` with the Windows executable.

## Releasing

Bump the version, add a section at the top of the changelog, and push to `main`. The
[Release workflow](.github/workflows/release.yml) builds the game and publishes `v<version>` with
the zip that Ironite Launcher installs.

(c) 2026 Ironite.
