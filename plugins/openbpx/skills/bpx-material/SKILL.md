---
name: bpx-material
description: BPX `material` command skill. Inspect materials, scan child instances, and extract custom HLSL.
---

# material

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx material read <file.uasset> [--export <n>] [--include-hlsl] [--children-root <directory>] [--parent <token>] [--pattern "*.uasset"] [--recursive] [--limit <n>]
```

## Behavior

- `read`: unified read entry for material inputs/references/parent and optional child scan/HLSL summary.
- `inspect`: summarizes material inputs, asset references, and direct parent material.
- `children`: scans a directory for material instances matching --parent token.
- `hlsl`: shows custom-node HLSL snippets (`UMaterialExpressionCustom::Code`) and explains full-translation limits.

## Code-Aligned Caveats

- `material read` is the canonical entry; use flags to opt into HLSL and child scans.
- Directory scans should always narrow with `--parent`, `--pattern`, and `--limit`.

## High-Signal Examples

```bash
bpx material read ./Sample.uasset [--export 1] [--include-hlsl] [--children-root ./Content] [--parent SampleToken] [--pattern "*.uasset"] [--recursive] [--limit 1]
```
