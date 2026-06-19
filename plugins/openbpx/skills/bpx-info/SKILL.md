---
name: bpx-info
description: BPX `info` command skill. Read one package summary (engine version, table counts, asset kind).
---

# info

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx info <file.uasset>
```

## Behavior

- Parses one package and prints engine version, table counts, and guessed asset kind.
- Read-only command; no files are written.

## High-Signal Examples

```bash
bpx info ./Sample.uasset
```
