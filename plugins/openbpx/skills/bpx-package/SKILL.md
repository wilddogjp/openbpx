---
name: bpx-package
description: BPX `package` command skill. Inspect package metadata/sections or update package flags.
---

# package

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx package meta <file.uasset>
bpx package custom-versions <file.uasset>
bpx package depends <file.uasset> [--reverse]
bpx package resolve-index <file.uasset> --index <i>
bpx package section <file.uasset> --name <section>
bpx package set-flags <file.uasset> --flags <enum-or-raw> [--dry-run] [--backup]
```

## Behavior

- `meta`: shows package GUID/flags/version/offset summary.
- `custom-versions`: lists custom version GUID/version pairs.
- `depends`: decodes DependsMap entries.
- `depends --reverse`: adds reverse dependency view (who references each export).
- `resolve-index`: classifies and resolves signed FPackageIndex.
- `section`: reads one raw package section by --name.
- `set-flags`: rewrites package flags within supported safe scope.
- `set-flags` blocks `PKG_FilterEditorOnly` and `PKG_UnversionedProperties` toggles.

> [!CAUTION]
> This command includes write-capable operations. Confirm intent and run `--dry-run` first.

## Command Matrix

| Command | Use when | Notable defaults |
|------|------|------|
| `meta` | shows package GUID/flags/version/offset summary. | Check `bpx help` for exact required flags. |
| `custom-versions` | lists custom version GUID/version pairs. | Check `bpx help` for exact required flags. |
| `depends` | decodes DependsMap entries. | Check `bpx help` for exact required flags. |
| `resolve-index` | classifies and resolves signed FPackageIndex. | Check `bpx help` for exact required flags. |
| `section` | reads one raw package section by --name. | Check `bpx help` for exact required flags. |
| `set-flags` | rewrites package flags within supported safe scope. | Run `--dry-run` first and use `--backup` for real writes. |

## Code-Aligned Caveats

- `set-flags` blocks unsupported/safety-critical toggles by design.
- `resolve-index` is the safest way to interpret signed `FPackageIndex` in automation flows.

## High-Signal Examples

```bash
bpx package meta ./Sample.uasset
bpx package custom-versions ./Sample.uasset
bpx package depends ./Sample.uasset [--reverse]
bpx package resolve-index ./Sample.uasset --index 1
```
