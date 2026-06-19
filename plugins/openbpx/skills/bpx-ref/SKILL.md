---
name: bpx-ref
description: BPX `ref` command skill. Bulk-rewrite reference strings in NameMap and decoded properties.
---

# ref

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx ref rewrite <file.uasset> --from <old> --to <new> [--dry-run] [--backup]
```

## Behavior

- `rewrite`: replaces reference tokens across NameMap and decodable properties.
- Requires different `--from` and `--to` values.
- Response includes NameMap/property rewrite counts and warnings.

> [!CAUTION]
> This command includes write-capable operations. Confirm intent and run `--dry-run` first.

## High-Signal Examples

```bash
bpx ref rewrite ./Sample.uasset --from OldValue --to NewValue [--dry-run] [--backup]
```
