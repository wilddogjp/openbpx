---
name: bpx-class
description: BPX `class` command skill. Inspect one class export payload/header by index.
---

# class

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx class <file.uasset> --export <n>
```

## Behavior

- Inspects one class export payload/header by --export index.
- Output follows generic export info shape (`file`, `export`).

## High-Signal Examples

```bash
bpx class ./Sample.uasset --export 1
```
