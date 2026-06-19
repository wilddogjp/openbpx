---
name: bpx-metadata
description: BPX `metadata` command skill. Read metadata exports or update root/object metadata key-values.
---

# metadata

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx metadata <file.uasset> --export <n>
bpx metadata set-root <file.uasset> --export <n> --key <k> --value <v> [--dry-run] [--backup]
bpx metadata set-object <file.uasset> --export <n> --import <i> --key <k> --value <v> [--dry-run] [--backup]
```

## Behavior

- Default form reads one metadata export by --export index.
- `set-root`: updates root metadata key/value.
- `set-object`: updates metadata for one import key/value.
- Set commands report the resolved property path that was mutated.

> [!CAUTION]
> This command includes write-capable operations. Confirm intent and run `--dry-run` first.

## Command Matrix

| Command | Use when | Notable defaults |
|------|------|------|
| `metadata` | Default form reads one metadata export by --export index. | Check `bpx help` for exact required flags. |
| `set-root` | updates root metadata key/value. | Check `bpx help` for exact required flags. |
| `set-object` | updates metadata for one import key/value. | Check `bpx help` for exact required flags. |

## Code-Aligned Caveats

- There is no `metadata read` subcommand; read form is `bpx metadata <file> --export <n>`.
- Object metadata updates require a valid `--import` target.

## High-Signal Examples

```bash
bpx metadata ./Sample.uasset --export 1
bpx metadata set-root ./Sample.uasset --export 1 --key SampleKey --value SampleValue [--dry-run] [--backup]
bpx metadata set-object ./Sample.uasset --export 1 --import 1 --key SampleKey --value SampleValue [--dry-run] [--backup]
```
