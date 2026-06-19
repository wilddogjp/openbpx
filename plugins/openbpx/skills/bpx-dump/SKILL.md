---
name: bpx-dump
description: BPX `dump` command skill. Dump package summary/name/import/export tables in structured formats.
---

# dump

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx dump <file.uasset> [--format json|toml|yaml] [--out path]
```

## Behavior

- Emits Summary/NameMap/ImportMap/ExportMap payload for one package.
- Supports --format json|toml|yaml and optional --out file write.
- When `--out` is used, stdout returns an acknowledgement object (`file`, `format`, `out`).

## High-Signal Examples

```bash
bpx dump ./Sample.uasset [--format json|toml|yaml] [--out path]
```
