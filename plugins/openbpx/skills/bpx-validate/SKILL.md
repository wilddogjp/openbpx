---
name: bpx-validate
description: BPX `validate` command skill. Run package integrity checks (exit code 2 when result is not OK).
---

# validate

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx validate <file.uasset> [--binary-equality]
```

## Behavior

- Runs parse and consistency checks for one package.
- `--binary-equality` also checks no-op rewrite byte equality.
- Returns exit code 2 when validation result is not OK.
- Validation details are emitted in `result` payload.

## Code-Aligned Caveats

- Exit code `2` indicates a non-OK validation result (not a transport/runtime failure).
- `--binary-equality` is the strongest no-op round-trip safety check.

## High-Signal Examples

```bash
bpx validate ./Sample.uasset [--binary-equality]
```
