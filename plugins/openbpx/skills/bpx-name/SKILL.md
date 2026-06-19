---
name: bpx-name
description: BPX `name` command skill. Inspect and edit NameMap entries with UE5-compatible hashes.
---

# name

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx name list <file.uasset>
bpx name add <file.uasset> --value <name> [--non-case-hash <u16>] [--case-preserving-hash <u16>] [--dry-run] [--backup]
bpx name set <file.uasset> --index <n> --value <name> [--non-case-hash <u16>] [--case-preserving-hash <u16>] [--dry-run] [--backup]
bpx name remove <file.uasset> --index <n> [--dry-run] [--backup]
```

## Behavior

- `list`: lists NameMap entries and hashes.
- `add`: appends a new NameMap entry.
- `set`: rewrites one NameMap entry by index.
- `remove`: removes tail NameMap entry only when safety checks pass.
- `add`/`set` auto-compute UE5 hashes when hash flags are omitted.

> [!CAUTION]
> This command includes write-capable operations. Confirm intent and run `--dry-run` first.

## Command Matrix

| Command | Use when | Notable defaults |
|------|------|------|
| `list` | lists NameMap entries and hashes. | Read-only path; safe for discovery. |
| `add` | appends a new NameMap entry. | Run `--dry-run` first and use `--backup` for real writes. |
| `set` | rewrites one NameMap entry by index. | Run `--dry-run` first and use `--backup` for real writes. |
| `remove` | removes tail NameMap entry only when safety checks pass. | Run `--dry-run` first and use `--backup` for real writes. |

## High-Signal Examples

```bash
bpx name list ./Sample.uasset
bpx name add ./Sample.uasset --value SampleName [--non-case-hash <u16>] [--case-preserving-hash <u16>] [--dry-run] [--backup]
bpx name set ./Sample.uasset --index 1 --value SampleName [--non-case-hash <u16>] [--case-preserving-hash <u16>] [--dry-run] [--backup]
bpx name remove ./Sample.uasset --index 1 [--dry-run] [--backup]
```
