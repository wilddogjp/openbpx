# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog and the project follows Semantic Versioning.

## [Unreleased]

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
