# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog and the project follows Semantic Versioning.

## [Unreleased]

## [0.1.5]

### Changed

- BPX CLI and plugin version strings updated to `0.1.5`.
- Removed helper installer scripts from skill/plugin paths (`install-bpx-from-release.ps1`); package-manager install flow is now standard (`brew` / `dpkg` / `winget`).
- Consolidated skill definition to a single file at `skills/bpx/SKILL.md` and updated Codex install path accordingly.
- Updated BPX skill description to improve trigger coverage for asset inspection/edit requests.

## [0.1.4]

### Added

- Homebrew formula source in repository (`packaging/homebrew/openbpx.rb`) and package-manager publish guide (`docs/dev/package-manager-publish.md`).
- Debian package output support in release config via GoReleaser `nfpms` (`openbpx_<version>_<arch>.deb`).

### Changed

- BPX CLI and plugin version strings updated to `0.1.4`.
- Skill installation guidance is now self-contained in each `SKILL.md` without README indirection.
- Linux installation guidance standardized on `.deb` + `dpkg` from GitHub Releases.
- Removed helper installer scripts (`install-bpx-from-release.sh` / `.ps1`) from skill/plugin paths; installation now assumes package-manager availability (`brew` / `dpkg` / `winget`).
- Public export allowlist updated to include package-manager docs and Homebrew formula path.

## [0.1.3]

### Added

- Release-based BPX installer scripts for Codex/Claude skill flows (`install-bpx-from-release.sh` / `.ps1`) with SHA-256 verification against release `checksums.txt`.
- Package manager publish/runbook documentation (`docs/dev/package-manager-publish.md`) for `brew` / `dpkg` / `winget`.

### Changed

- Removed bundled prebuilt binaries from `.agents/skills/bpx/`.
- Updated skill installers to try package managers first (`brew` / `dpkg` / `winget`) and fallback to release download + checksum verification.
- Homebrew install flow now uses formula hosted in `openbpx` (`packaging/homebrew/openbpx.rb`) instead of requiring a separate tap repository.
- Updated README and skill docs to match package-manager-first install behavior.

## [0.1.2]

### Added

- Installation guides for Codex skill and Claude Code plugin in `README`.
- Claude plugin manifest (`.claude-plugin/plugin.json`) and plugin skill entry (`skills/bpx/SKILL.md`).

### Changed

- Public export now includes `.agents`, `.claude-plugin`, and `skills` so bundled skill/plugin artifacts can be promoted to the public repository.
- CLI and plugin version strings updated to `0.1.2`.

## [0.1.1]

### Added

- OSS publishing baseline documents (`README`, `CONTRIBUTING`, `ISSUE`, `SECURITY`, `CHANGELOG`) and public export alignment.

### Changed

- Public repository naming aligned to `openbpx` while keeping CLI/binary name as `bpx`.
- Public module path changed to `github.com/wilddogjp/openbpx`.

## [0.1.0]

### Added

- Initial alpha release of BPX CLI.
- Core read/validate commands for Unreal `.uasset` / `.umap` assets.
- Safety-constrained write commands with binary-preservation focus.
