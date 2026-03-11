# OpenBPX: Blueprint Toolkit for Unreal Engine (`bpx`)

<p align="center">
    <picture>
        <source media="(prefers-color-scheme: light)" srcset="docs/assets/openbpx-logo-dark.png">
        <img src="docs/assets/openbpx-logo-light.png" alt="OpenBPX logo" width="500">
    </picture>
</p>

[![Test](https://github.com/wilddogjp/openbpx/actions/workflows/test.yml/badge.svg)](https://github.com/wilddogjp/openbpx/actions/workflows/test.yml)
[![Release](https://img.shields.io/github/v/release/wilddogjp/openbpx)](https://github.com/wilddogjp/openbpx/releases)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](https://go.dev/)

**A safety-first CLI for reading and editing Unreal package assets (`.uasset`, `.umap`) without launching Unreal Editor.**

`BPX` is designed for automation and scripting workflows where binary safety matters: unknown bytes are preserved, no-op round-trip stays byte-identical, and unsupported/high-risk operations fail explicitly.

BPX runs on **Windows**, **macOS**, and **Linux**.

```bash
# Windows
winget install --id WilddogJP.OpenBPX --exact

# Debian / Ubuntu (WSL)
curl -fsSL https://raw.githubusercontent.com/wilddogjp/openbpx/main/scripts/install-bpx-from-release.sh | bash

# macOS
HOMEBREW_DEVELOPER=1 brew install --formula https://raw.githubusercontent.com/wilddogjp/openbpx/main/packaging/homebrew/openbpx.rb
```

> [!IMPORTANT]
> This project is under active development. Expect breaking changes as we march toward v1.0.

## Contents

- [Quick Start](#quick-start)
- [Why OpenBPX?](#why-openbpx)
- [Install](#install)
- [AI Agent Skills](#ai-agent-skills)
- [Safety and Security Model](#safety-and-security-model)
- [Supported Scope](#supported-scope)
- [How It Works](#how-it-works)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [Changelog](#changelog)
- [License](#license)
- [About Wild Dog](#about-wild-dog)

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

## Why OpenBPX?

**For humans**: inspect package metadata, exports/imports, properties, DataTables, and Blueprint data from a single CLI.

**For automation and AI agents**: use structured output (`json`/`toml`) and deterministic safety-first write flows (`--dry-run`, `--backup`, `validate`).

### Project Status

- Phase: **Alpha**
- Supported UE window: **UE 5.0 to UE 5.7** (`FileVersionUE5=1000..1018`)
- Supported platforms: **Windows / macOS / Linux** (`amd64`, `arm64`)
- Core principles: **unknown-byte preservation**, **round-trip fidelity**, **safety-first editing**, **UE behavior-grounded implementation**

## Install

### CLI (`bpx`)

#### From package managers

```bash
# Windows
winget install --id WilddogJP.OpenBPX --exact

# Debian / Ubuntu (one-command installer with checksum verification) / WSL
curl -fsSL https://raw.githubusercontent.com/wilddogjp/openbpx/main/scripts/install-bpx-from-release.sh | bash

# macOS (Homebrew formula hosted in this repository)
# Homebrew blocks URL/path formula install by default, so enable developer mode for this command.
HOMEBREW_DEVELOPER=1 brew install --formula https://raw.githubusercontent.com/wilddogjp/openbpx/main/packaging/homebrew/openbpx.rb
```

#### Build locally

```bash
git clone https://github.com/wilddogjp/openbpx.git
cd openbpx
go build ./cmd/bpx
```

Official release artifacts are published on [GitHub Releases](https://github.com/wilddogjp/openbpx/releases).

## AI Agent Skills

### Generate SKILL.md files from installed `bpx` (recommended)

```bash
# Generate all command Codex skills to .codex/skills
bpx generate-skills --output-dir .codex/skills

# Generate all command Claude skills to .claude/skills
bpx generate-skills --output-dir .claude/skills
```

Generated layout:

- `skills/bpx-shared/SKILL.md`
- `skills/bpx-<command>/SKILL.md`

`bpx generate-skills` uses built-in command help metadata and built-in command-profile supplements baked into the binary, so generation works from a single binary without reading repository files.

## Safety and Security Model

- Preserve bytes that BPX does not interpret.
- Keep `read -> no edit -> write` byte-identical.
- Reject unsupported UE versions outside `1000..1018`.
- Fail explicitly for unsupported/high-risk structural rewrites.
- Treat all input assets as untrusted binary input.

See [SECURITY.md](SECURITY.md) for vulnerability reporting and response policy.

## Supported Scope

| Item | Current Support |
|---|---|
| UE version window | `FileVersionUE5=1000..1018` (UE 5.0 to 5.7) |
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
