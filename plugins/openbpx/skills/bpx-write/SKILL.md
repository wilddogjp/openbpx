---
name: bpx-write
description: BPX `write` command skill. Rewrite a parsed package to a separate output path.
---

# write

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx write <file.uasset> --out <new.uasset> [--dry-run] [--backup]
```

## Behavior

- Rewrites parsed package bytes to --out using current in-memory structure.
- Never modifies the source file; output target is required.
- `--dry-run` reports changed/bytes without writing files.
- `--backup` creates `<out>.backup` when destination already exists.

> [!CAUTION]
> This command includes write-capable operations. Confirm intent and run `--dry-run` first.

## High-Signal Examples

```bash
bpx write ./Sample.uasset --out ./Sample.out.uasset [--dry-run] [--backup]
```
