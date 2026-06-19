---
name: bpx-var
description: BPX `var` command skill. Inspect variable defaults/declarations and update defaults or names.
---

# var

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx var list <file.uasset>
bpx var set-default <file.uasset> --name <var> --value '<json>' [--dry-run] [--backup]
bpx var rename <file.uasset> --from <old> --to <new> [--dry-run] [--backup]
```

## Behavior

- `list`: merges CDO defaults with declaration metadata.
- `set-default`: writes a variable default on CDO properties.
- `rename`: rewrites matching NameMap entries from --from to --to.
- `rename` fails when destination variable is already declared; may return declaration warnings.

> [!CAUTION]
> This command includes write-capable operations. Confirm intent and run `--dry-run` first.

## Command Matrix

| Command | Use when | Notable defaults |
|------|------|------|
| `list` | merges CDO defaults with declaration metadata. | Read-only path; safe for discovery. |
| `set-default` | writes a variable default on CDO properties. | Check `bpx help` for exact required flags. |
| `rename` | rewrites matching NameMap entries from --from to --to. | Run `--dry-run` first and use `--backup` for real writes. |

## High-Signal Examples

```bash
bpx var list ./Sample.uasset
bpx var set-default ./Sample.uasset --name <var> --value '{"value":1}' [--dry-run] [--backup]
bpx var rename ./Sample.uasset --from OldValue --to NewValue [--dry-run] [--backup]
```
