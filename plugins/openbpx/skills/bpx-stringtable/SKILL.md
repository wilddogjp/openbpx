---
name: bpx-stringtable
description: BPX `stringtable` command skill. Read and edit StringTable entries and namespace.
---

# stringtable

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx stringtable read <file.uasset>
bpx stringtable write-entry <file.uasset> --export <n> --key <k> --value <v> [--dry-run] [--backup]
bpx stringtable remove-entry <file.uasset> --export <n> --key <k> [--dry-run] [--backup]
bpx stringtable set-namespace <file.uasset> --export <n> --namespace <ns> [--dry-run] [--backup]
```

## Behavior

- `read`: lists string table exports.
- `write-entry`: updates an existing key value.
- `remove-entry`: removes an existing key.
- `set-namespace`: rewrites string table namespace.
- Write commands operate on existing string table exports and report changed-byte status.

> [!CAUTION]
> This command includes write-capable operations. Confirm intent and run `--dry-run` first.

## Command Matrix

| Command | Use when | Notable defaults |
|------|------|------|
| `read` | lists string table exports. | Read-only path; safe for discovery. |
| `write-entry` | updates an existing key value. | Run `--dry-run` first and use `--backup` for real writes. |
| `remove-entry` | removes an existing key. | Check `bpx help` for exact required flags. |
| `set-namespace` | rewrites string table namespace. | Check `bpx help` for exact required flags. |

## High-Signal Examples

```bash
bpx stringtable read ./Sample.uasset
bpx stringtable write-entry ./Sample.uasset --export 1 --key SampleKey --value SampleValue [--dry-run] [--backup]
bpx stringtable remove-entry ./Sample.uasset --export 1 --key SampleKey [--dry-run] [--backup]
bpx stringtable set-namespace ./Sample.uasset --export 1 --namespace Game [--dry-run] [--backup]
```
