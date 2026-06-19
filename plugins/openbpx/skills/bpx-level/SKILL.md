---
name: bpx-level
description: BPX `level` command skill. Inspect level exports and read/write actor properties in .umap.
---

# level

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx level info <file.umap> --export <n>
bpx level actor-search <file.umap> [--name <token>] [--actor-label <token>] [--actor-class <token>] [--limit <n>]
bpx level var-list <file.umap> --actor <name|PersistentLevel.Name|export-index>
bpx level var-set <file.umap> --actor <name|PersistentLevel.Name|export-index> --path <dot.path> --value '<json>' [--dry-run] [--backup]
```

## Behavior

- `info`: inspects one level export.
- `actor-search`: filters PersistentLevel child exports by name/ActorLabel/ActorClass tokens.
- `var-list`: decodes actor properties selected by --actor.
- `var-set`: updates one actor property at --path.
- `--actor` accepts object name, `PersistentLevel.<Name>`, or export index.

> [!CAUTION]
> This command includes write-capable operations. Confirm intent and run `--dry-run` first.

## Command Matrix

| Command | Use when | Notable defaults |
|------|------|------|
| `info` | inspects one level export. | Read-only path; safe for discovery. |
| `actor-search` | filters PersistentLevel child exports by name/ActorLabel/ActorClass tokens. | Check `bpx help` for exact required flags. |
| `var-list` | decodes actor properties selected by --actor. | Check `bpx help` for exact required flags. |
| `var-set` | updates one actor property at --path. | Run `--dry-run` first and use `--backup` for real writes. |

## Code-Aligned Caveats

- `--actor` resolution supports name, `PersistentLevel.<Name>`, or export index.
- `var-set` uses property path semantics; use `var-list` to validate target paths first.

## High-Signal Examples

```bash
bpx level info ./Sample.umap --export 1
bpx level actor-search ./Sample.umap [--name SampleToken] [--actor-label SampleToken] [--actor-class SampleToken] [--limit 1]
bpx level var-list ./Sample.umap --actor <name|PersistentLevel.Name|export-index>
bpx level var-set ./Sample.umap --actor <name|PersistentLevel.Name|export-index> --path MyProperty --value '{"value":1}' [--dry-run] [--backup]
```
