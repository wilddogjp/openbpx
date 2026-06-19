---
name: bpx-import
description: BPX `import` command skill. Inspect ImportMap entries and aggregate import dependency graphs.
---

# import

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx import list <file.uasset>
bpx import search <file.uasset> [--object <name>] [--class-package <pkg>] [--class-name <cls>]
bpx import graph <directory> [--pattern "*.uasset"] [--recursive] [--group-by root|object] [--filter <token>]
```

## Behavior

- `list`: lists ImportMap entries for one package.
- `search`: filters imports by object/class tokens (requires at least one filter).
- `graph`: aggregates import dependency edges across a directory.
- `graph` reports per-file parse failures without aborting the whole scan.

## Command Matrix

| Command | Use when | Notable defaults |
|------|------|------|
| `list` | lists ImportMap entries for one package. | Read-only path; safe for discovery. |
| `search` | filters imports by object/class tokens (requires at least one filter). | Check `bpx help` for exact required flags. |
| `graph` | aggregates import dependency edges across a directory. | Check `bpx help` for exact required flags. |

## Code-Aligned Caveats

- `graph` is ImportMap-based and may not reflect K2 graph-level references.
- Large directory scans should use `--filter` and narrower patterns for speed.

## High-Signal Examples

```bash
bpx import list ./Sample.uasset
bpx import search ./Sample.uasset [--object SampleName] [--class-package <pkg>] [--class-name <cls>]
bpx import graph ./Content [--pattern "*.uasset"] [--recursive] [--group-by root|object] [--filter SampleToken]
```
