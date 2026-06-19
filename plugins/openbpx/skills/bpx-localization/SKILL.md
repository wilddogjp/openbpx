---
name: bpx-localization
description: BPX `localization` command skill. Read/query/resolve localization data and edit existing text identities.
---

# localization

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx localization read <file.uasset> [--export <n>] [--include-history] [--format json|toml|csv]
bpx localization query <file.uasset> [--export <n>] [--namespace <ns>] [--key <key>] [--text <token>] [--history-type <type>] [--limit <n>]
bpx localization resolve <file.uasset> [--export <n>] --culture <culture> [--locres <path>] [--missing-only]
bpx localization set-source <file.uasset> --export <n> --path <dot.path> --value <text> [--dry-run] [--backup]
bpx localization set-id <file.uasset> --export <n> --path <dot.path> --namespace <ns> --key <key> [--dry-run] [--backup]
bpx localization set-stringtable-ref <file.uasset> --export <n> --path <dot.path> --table <table-id> --key <key> [--dry-run] [--backup]
bpx localization rewrite-namespace <file.uasset> --from <ns-old> --to <ns-new> [--dry-run] [--backup]
bpx localization rekey <file.uasset> --namespace <ns> --from-key <k-old> --to-key <k-new> [--dry-run] [--backup]
```

## Behavior

- `read`: enumerates TextProperty + GatherableTextData entries.
- `query`: filters entries by namespace/key/text/history type.
- `resolve`: previews localized strings for --culture (optional .locres).
- `set-source`/`set-id`/`set-stringtable-ref`: updates existing text data.
- `rewrite-namespace`/`rekey`: bulk-rewrites namespace or key values.
- `resolve --missing-only` returns unresolved entries only.

> [!CAUTION]
> This command includes write-capable operations. Confirm intent and run `--dry-run` first.

## Command Matrix

| Command | Use when | Notable defaults |
|------|------|------|
| `read` | enumerates TextProperty + GatherableTextData entries. | Read-only path; safe for discovery. |
| `query` | filters entries by namespace/key/text/history type. | Check `bpx help` for exact required flags. |
| `resolve` | previews localized strings for --culture (optional .locres). | Check `bpx help` for exact required flags. |
| `set-source` | `read`: enumerates TextProperty + GatherableTextData entries. | Run `--dry-run` first and use `--backup` for real writes. |
| `set-id` | `read`: enumerates TextProperty + GatherableTextData entries. | Run `--dry-run` first and use `--backup` for real writes. |
| `set-stringtable-ref` | `read`: enumerates TextProperty + GatherableTextData entries. | Run `--dry-run` first and use `--backup` for real writes. |
| `rewrite-namespace` | `read`: enumerates TextProperty + GatherableTextData entries. | Run `--dry-run` first and use `--backup` for real writes. |
| `rekey` | `read`: enumerates TextProperty + GatherableTextData entries. | Run `--dry-run` first and use `--backup` for real writes. |

## Code-Aligned Caveats

- `resolve` is read-preview oriented; it does not mutate assets.
- Bulk namespace/key rewrites should always be previewed first in a narrowed scope.

## High-Signal Examples

```bash
bpx localization read ./Sample.uasset [--export 1] [--include-history] [--format json|toml|csv]
bpx localization query ./Sample.uasset [--export 1] [--namespace Game] [--key <key>] [--text SampleToken] [--history-type <type>] [--limit 1]
bpx localization resolve ./Sample.uasset [--export 1] --culture en [--locres MyProperty] [--missing-only]
bpx localization set-source ./Sample.uasset --export 1 --path MyProperty --value <text> [--dry-run] [--backup]
```
