# OpenBPX: Blueprint Toolkit for Unreal Engine (`bpx`)

[![Test](https://github.com/wilddogjp/openbpx/actions/workflows/test.yml/badge.svg)](https://github.com/wilddogjp/openbpx/actions/workflows/test.yml)
[![Release](https://img.shields.io/github/v/release/wilddogjp/openbpx)](https://github.com/wilddogjp/openbpx/releases)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](https://go.dev/)

`BPX` is a Go CLI for reading and safely editing Unreal Engine package assets (`.uasset`, `.umap`) without launching Unreal Editor.

It is designed for automation and scripting use-cases where binary safety matters: unknown bytes are preserved, no-op round-trip stays byte-identical, and unsupported/high-risk operations fail explicitly.

BPX runs on **Windows**, **macOS**, and **Linux**.

## Project Status

- Phase: **Alpha**
- Current CLI version: **0.1.4**
- Supported UE window: **UE 5.0 to UE 5.6** (`FileVersionUE5=1000..1017`)
- Supported platforms: **Windows / macOS / Linux** (`amd64`, `arm64`)
- Core principles: **unknown-byte preservation**, **round-trip fidelity**, **safety-first editing**, **UE behavior-grounded implementation**

## Install

### CLI (`bpx`)

#### From package managers

```bash
# macOS (Homebrew formula hosted in this repository)
brew install --formula https://raw.githubusercontent.com/wilddogjp/openbpx/main/packaging/homebrew/openbpx.rb

# Debian / Ubuntu (dpkg from GitHub Releases)
VER=0.1.4
ARCH="$(dpkg --print-architecture)" # amd64 / arm64
curl -fsSLO "https://github.com/wilddogjp/openbpx/releases/download/v${VER}/openbpx_${VER}_${ARCH}.deb"
sudo dpkg -i "openbpx_${VER}_${ARCH}.deb"

# Windows
winget install --id WilddogJP.OpenBPX --exact
```

#### From source with `go install`

```bash
go install github.com/wilddogjp/openbpx/cmd/bpx@latest
```

#### Build locally

```bash
git clone https://github.com/wilddogjp/openbpx.git
cd openbpx
go build ./cmd/bpx
```

Official release artifacts are published on [GitHub Releases](https://github.com/wilddogjp/openbpx/releases).

### Install BPX skill for Codex

```bash
python3 ~/.codex/skills/.system/skill-installer/scripts/install-skill-from-github.py \
  --repo wilddogjp/openbpx \
  --path .agents/skills/bpx \
  --method git
```

After installation, restart Codex. Ensure `bpx` is available on `PATH` before using the skill.

### Install BPX plugin for Claude Code

```bash
git clone https://github.com/wilddogjp/openbpx.git
cd openbpx

claude --plugin-dir .
```

Ensure `bpx` is available on `PATH` before using the plugin skill. On Windows you can use the helper installer: `pwsh -File ./skills/bpx/scripts/install-bpx-from-release.ps1`.

Use the skill in Claude prompts as `/openbpx:bpx`.

## Quick Start

```bash
# Show help and version
bpx --help
bpx version

# Inspect an asset
bpx info ./Sample.uasset
bpx validate ./Sample.uasset --binary-equality

# Read properties
bpx prop list ./Sample.uasset --export 1 --format json

# Safe write flow example
bpx prop set ./Sample.uasset --export 1 --path "MyValue" --value '123' --dry-run
bpx prop set ./Sample.uasset --export 1 --path "MyValue" --value '123' --backup
```

## Safety and Security Model

- Preserve bytes that BPX does not interpret.
- Keep `read -> no edit -> write` byte-identical.
- Reject unsupported UE versions outside `1000..1017`.
- Fail explicitly for unsupported/high-risk structural rewrites.
- Treat all input assets as untrusted binary input.

See [SECURITY.md](SECURITY.md) for vulnerability reporting and response policy.

## Supported Scope

| Item | Current Support |
|---|---|
| UE version window | `FileVersionUE5=1000..1017` (UE 5.0 to 5.6) |
| Asset files | `.uasset`, `.umap` |
| Read/inspect commands | Implemented (see [docs/commands.md](docs/commands.md)) |
| Scoped update commands | Implemented (safety-constrained) |
| High-risk structural rewrites | Partially blocked by design |

## How It Works

1. Parse package data into a binary-safe model.
2. Decode known structures while retaining unknown/raw regions.
3. Apply deterministic, scoped edits only to the intended region.
4. Re-serialize with offset/size updates and validation checks.

## Documentation

- [Command Specification](docs/commands.md)
- [Test Plan and Fixture Specification](docs/test-fixtures.md)
- [Distribution Build Guide](docs/build-distribution.md)
- [Disassembly Script Extraction Notes](docs/disasm-script-extraction.md)
- [Package Manager Publish Guide](docs/dev/package-manager-publish.md)

## Contributing

Contributions are welcome.

- Read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a PR.
- Use [ISSUE.md](ISSUE.md) when filing bug reports or feature requests.
- For large features, open an Issue first to align design and scope.
- For CLI contract changes, update [docs/commands.md](docs/commands.md) in the same PR.

## Changelog

Release history is tracked in [CHANGELOG.md](CHANGELOG.md).

## License

Apache License 2.0. See [LICENSE](LICENSE).

## About Wild Dog

<img src="docs/assets/wilddog-logo-full-white.webp" alt="Wild Dog logo" width="220" />

BPX is created and maintained by [Wild Dog, Inc. (株式会社ワイルドドッグ)](https://wilddog.jp), an indie game development studio based in Japan, operated by a team of 20 members. Our mission is to deliver player-first games where fun always comes first.
