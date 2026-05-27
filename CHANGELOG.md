# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog and the project follows Semantic Versioning.

## [Unreleased]

## [0.2.0]

### Added

- WidgetBlueprint workflows for template initialization, rootless parent-class rewrites, widget tree reads, safe add/remove operations, and supported widget/slot/property writes.
- Expanded WidgetBlueprint coverage for UMG containers, editable text widgets, list/tile/tree views, focusable controls, RichTextBlock helpers, brush-image imports, and UE 5.6/5.7 operation-equivalence fixtures.
- Codex app Marketplace, Codex plugin, and Claude Code plugin manifest support that reuses the existing BPX command skills while keeping the legacy Agent Skill layout intact.
- OpenBPX logo assets and README branding for the public repository.

### Changed

- BPX CLI, plugin manifests, issue-template version examples, and release workflow examples now target `0.2.0`.
- Command help, `docs/commands.md`, widget skills, and test-fixture documentation now reflect the current WidgetBlueprint command surface.
- Public export scope now includes agent plugin metadata, package-manager packaging files, and the package-manager publish guide.

### Fixed

- WidgetBlueprint rewrites now preserve generated-class tail references, widget layout anchors/alignment, NameMap remap finalization, and leaf-removal metadata under fixture-backed safety checks.
- UE 5.6 and UE 5.7 golden operation fixtures were realigned with plugin-generated expectations while retaining the supported UE window `FileVersionUE5=1000..1018` (UE 5.0 to 5.7).

## [0.1.8]

### Added

- `bpx level actor-search` command for filtering `PersistentLevel` exports by actor attributes.
- `bpx material read` command with material inspection output and aggregated HLSL extraction support.
- BPX command-skill coverage for asset inspection and edit workflows, including new material- and version-focused skill generation.
- Public-release auto-tagging and stronger release guards so tags and changelog entries stay aligned with `bpx version`.

### Changed

- BPX CLI, Claude plugin manifest, and issue-template version references now target `0.1.8`.
- README and installation guidance were reorganized across platforms, including clarified Debian/Ubuntu flows and package-manager-first setup.
- Golden-fixture generation now runs across configured engines in parallel, and operation-fixture coverage tracking is stricter.
- UE 5.7.3 is now covered by versioned golden tests within the supported UE window `FileVersionUE5=1000..1018` (UE 5.0 to 5.7).

### Fixed

- UE 5.6 and UE 5.7 rewrite behavior, goldens, and operation-equivalence fixtures were realigned with expected UE outputs.
- Synthetic version-window fixtures were repaired to match supported parser behavior.
- Unused CLI compaction helpers that were breaking `staticcheck` were removed.

## [0.1.7]

### Added

- CLI subcommand dispatching for clearer command handling and more structured help output.

### Changed

- Command help and dispatch coverage were reorganized across read and write command families.
- Installation guidance stopped pinning a specific `bpx` version in example steps.
- BPX CLI, Claude plugin manifest, issue-template placeholder, and changelog references were updated to `0.1.7`.

## [0.1.6]

### Changed

- Running `bpx` with no arguments now prints root help and exits with status code `0` (instead of non-zero).
- BPX CLI and plugin version strings updated to `0.1.6`.
- Skill guidance now explicitly recommends `--format toml` for AI-driven post-processing.

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
