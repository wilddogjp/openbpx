# Package Manager Publish Guide

This document tracks how OpenBPX is published to package managers and how Skill installers resolve `bpx` quickly.

## Goal

- `bpx` should be installable from:
  - `brew` (macOS)
  - `dpkg` with `.deb` assets (Debian/Ubuntu)
  - `winget` (Windows)
- Skill installers should try package manager first, then fallback to GitHub Releases with checksum verification.

## Current Installer Behavior

- Users install manually via package manager instructions in `README.md`.

## Publish Targets

### Homebrew (Formula in openbpx)

1. Update `packaging/homebrew/openbpx.rb` in this repository.
2. Set release URL and SHA-256 for both macOS architectures.
3. Verify:
   - `brew install --formula https://raw.githubusercontent.com/wilddogjp/openbpx/main/packaging/homebrew/openbpx.rb`
   - `bpx version`

### Debian/Ubuntu (`dpkg`)

1. Build `.deb` assets on each release (`openbpx_<version>_<arch>.deb`).
2. Attach `.deb` artifacts to the GitHub Release.
3. Verify:
   - `curl -fsSLO https://github.com/wilddogjp/openbpx/releases/download/vX.Y.Z/openbpx_X.Y.Z_amd64.deb`
   - `sudo dpkg -i openbpx_X.Y.Z_amd64.deb`
   - `bpx version`

### WinGet

1. Create/maintain WinGet package ID (for example `WilddogJP.OpenBPX`).
2. Submit/update manifests to `microsoft/winget-pkgs` for each release.
3. Wait for merge and propagation.
4. Verify:
   - `winget install --id WilddogJP.OpenBPX --exact`
   - `bpx version`

## Release Checklist (Package Managers)

1. Cut GitHub Release (`vX.Y.Z`) with all platform assets and `checksums.txt`.
2. Update Homebrew formula in `openbpx` and verify raw-URL install.
3. Verify `.deb` assets are present in the GitHub Release.
4. Submit WinGet manifest update.
5. Smoke test package-manager installation commands on each target platform.
