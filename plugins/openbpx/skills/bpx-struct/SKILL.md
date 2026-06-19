---
name: bpx-struct
description: BPX `struct` command skill. List struct exports or inspect one struct export.
---

# struct

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx struct definition <file.uasset>
bpx struct details <file.uasset> --export <n>
```

## Behavior

- `definition`: lists struct-like exports.
- `details`: inspects one struct export by --export index.

## Command Matrix

| Command | Use when | Notable defaults |
|------|------|------|
| `definition` | lists struct-like exports. | Check `bpx help` for exact required flags. |
| `details` | inspects one struct export by --export index. | Check `bpx help` for exact required flags. |

## High-Signal Examples

```bash
bpx struct definition ./Sample.uasset
bpx struct details ./Sample.uasset --export 1
```
