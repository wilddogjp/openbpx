---
name: bpx-enum
description: BPX `enum` command skill. List enum exports or update existing enum values.
---

# enum

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx enum list <file.uasset>
bpx enum write-value <file.uasset> --export <n> --name <k> --value <v> [--dry-run] [--backup]
```

## Behavior

- `list`: enumerates enum exports.
- `write-value`: updates an existing enum entry value.
- `write-value` edits existing data only (no enum entry insertion/removal).

> [!CAUTION]
> This command includes write-capable operations. Confirm intent and run `--dry-run` first.

## Command Matrix

| Command | Use when | Notable defaults |
|------|------|------|
| `list` | enumerates enum exports. | Read-only path; safe for discovery. |
| `write-value` | updates an existing enum entry value. | Run `--dry-run` first and use `--backup` for real writes. |

## High-Signal Examples

```bash
bpx enum list ./Sample.uasset
bpx enum write-value ./Sample.uasset --export 1 --name SampleKey --value SampleValue [--dry-run] [--backup]
```
