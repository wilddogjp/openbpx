---
name: bpx-prop
description: BPX `prop` command skill. Read decoded properties or mutate properties in one export.
---

# prop

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx prop list <file.uasset> --export <n>
bpx prop set <file.uasset> --export <n> --path <dot.path> --value '<json>' [--dry-run] [--backup]
bpx prop add <file.uasset> --export <n> --spec '<json>' [--dry-run] [--backup]
bpx prop remove <file.uasset> --export <n> --path <dot.path> [--dry-run] [--backup]
```

## Behavior

- `list`: decodes properties for one export and includes warnings.
- `set`: updates an existing property value at --path.
- `add`: appends a new top-level property from --spec JSON.
- `remove`: removes a property at --path.
- Write subcommands report old/new values, size deltas, and changed-byte status.

> [!CAUTION]
> This command includes write-capable operations. Confirm intent and run `--dry-run` first.

## Command Matrix

| Command | Use when | Notable defaults |
|------|------|------|
| `list` | decodes properties for one export and includes warnings. | Read-only path; safe for discovery. |
| `set` | updates an existing property value at --path. | Run `--dry-run` first and use `--backup` for real writes. |
| `add` | appends a new top-level property from --spec JSON. | Run `--dry-run` first and use `--backup` for real writes. |
| `remove` | removes a property at --path. | Run `--dry-run` first and use `--backup` for real writes. |

## Code-Aligned Caveats

- Write paths mutate one export only; invalid dot paths fail explicitly.
- Prefer `prop list` immediately before `prop set/add/remove` to avoid stale assumptions.

## High-Signal Examples

```bash
bpx prop list ./Sample.uasset --export 1
bpx prop set ./Sample.uasset --export 1 --path MyProperty --value '{"value":1}' [--dry-run] [--backup]
bpx prop add ./Sample.uasset --export 1 --spec '{"value":1}' [--dry-run] [--backup]
bpx prop remove ./Sample.uasset --export 1 --path MyProperty [--dry-run] [--backup]
```
