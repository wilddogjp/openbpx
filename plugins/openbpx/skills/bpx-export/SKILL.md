---
name: bpx-export
description: BPX `export` command skill. Inspect export headers or update selected header fields.
---

# export

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx export list <file.uasset> [--class <token>]
bpx export info <file.uasset> --export <n>
bpx export set-header <file.uasset> --index <n> --fields '<json>' [--dry-run] [--backup]
```

## Behavior

- `list`: lists export headers with class/object/serial info.
- `info`: inspects one export header by --export index.
- `set-header`: updates selected export header fields (write command).
- `set-header` requires non-empty `--fields` JSON and reports old/new field values.

> [!CAUTION]
> This command includes write-capable operations. Confirm intent and run `--dry-run` first.

## Command Matrix

| Command | Use when | Notable defaults |
|------|------|------|
| `list` | lists export headers with class/object/serial info. | Read-only path; safe for discovery. |
| `info` | inspects one export header by --export index. | Read-only path; safe for discovery. |
| `set-header` | updates selected export header fields (write command). | Run `--dry-run` first and use `--backup` for real writes. |

## High-Signal Examples

```bash
bpx export list ./Sample.uasset [--class SampleToken]
bpx export info ./Sample.uasset --export 1
bpx export set-header ./Sample.uasset --index 1 --fields '{"value":1}' [--dry-run] [--backup]
```
