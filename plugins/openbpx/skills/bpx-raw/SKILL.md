---
name: bpx-raw
description: BPX `raw` command skill. Read one export serial payload as base64.
---

# raw

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx raw <file.uasset> --export <n>
```

## Behavior

- Reads raw serial payload bytes for one export.
- `--full` includes the complete base64 payload.
- Default output keeps payload compact via abbreviated base64 fields.

## High-Signal Examples

```bash
bpx raw ./Sample.uasset --export 1
```
