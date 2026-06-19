---
name: bpx-find
description: BPX `find` command skill. Scan directories for assets and summarize parse outcomes.
---

# find

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx find assets <directory> [--pattern "*.uasset"] [--recursive]
bpx find summary <directory> [--pattern "*.uasset"] [--recursive] [--format json|toml] [--out <path>]
```

## Behavior

- `assets`: collects files matching --pattern under a directory (default `*.uasset`, recursive).
- `summary`: parses each match and reports parsed counts, asset kind counts, and parse failures.
- `summary` continues when per-file parse fails and reports `parseFailures`.
- For map-only scans, pass `--pattern "*.umap"`.

## Command Matrix

| Command | Use when | Notable defaults |
|------|------|------|
| `assets` | collects files matching --pattern under a directory (default `*.uasset`, recursive). | Check `bpx help` for exact required flags. |
| `summary` | parses each match and reports parsed counts, asset kind counts, and parse failures. | Read-only path; safe for discovery. |

## Code-Aligned Caveats

- `find summary` continues across parse failures; inspect `parseFailures` before deciding next steps.
- For map-only scans, use `--pattern "*.umap"`.

## High-Signal Examples

```bash
bpx find assets ./Content [--pattern "*.uasset"] [--recursive]
bpx find summary ./Content [--pattern "*.uasset"] [--recursive] [--format json|toml] [--out MyProperty]
```
