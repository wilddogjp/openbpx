---
name: bpx-blueprint
description: BPX `blueprint` command skill. Inspect and analyze blueprint exports, bytecode, and graph data.
---

# blueprint

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx blueprint info <file.uasset> [--export <n>]
bpx blueprint bytecode <file.uasset> --export <n> [--range-source auto|export-map|ustruct-script|serial-full] [--strict-range] [--diagnostics]
bpx blueprint disasm <file.uasset> --export <n> [--format json|toml|text] [--analysis] [--entrypoint <vm>] [--max-steps <n>] [--range-source auto|export-map|ustruct-script|serial-full] [--strict-range] [--diagnostics]
bpx blueprint trace <file.uasset> --from <Node|Node.Pin> [--to-node <token>] [--to-function <token>] [--max-depth <n>]
bpx blueprint call-args <file.uasset> --member <token> [--class <token>] [--all-pins] [--include-exec]
bpx blueprint refs <file.uasset> --soft-path <path> [--class <token>] [--include-routes] [--max-routes <n>] [--max-depth <n>]
bpx blueprint search <file.uasset> [--class <token>] [--member <token>] [--name <token>] [--show <fields>] [--limit <n>]
bpx blueprint infer-pack <file.uasset> --export <n> [--entrypoint <vm>] [--max-steps <n>] [--out <dir>] [--range-source auto|export-map|ustruct-script|serial-full] [--strict-range] [--diagnostics]
bpx blueprint scan-functions <directory> --recursive [--name-like <regex>] [--aggregate]
```

## Behavior

- `info`: summarizes blueprint/function exports.
- `bytecode`: extracts selected bytecode range as base64.
- `disasm`: disassembles bytecode (json|toml|text, optional analysis).
- `trace`: traces an execution path between nodes.
- `call-args`: inspects call-node argument pins/defaults.
- `refs`: reverse-searches soft-path usage on node pins.
- `search`: token-searches nodes/pins in one blueprint package.
- `scan-functions`: aggregates function names across a directory.
- `infer-pack`: emits CFG/callsite/def-use inference artifacts.
- `bytecode`/`disasm` support range selection (`auto|export-map|ustruct-script|serial-full`).

## Command Matrix

| Command | Use when | Notable defaults |
|------|------|------|
| `info` | summarizes blueprint/function exports. | Read-only path; safe for discovery. |
| `bytecode` | extracts selected bytecode range as base64. | Check `bpx help` for exact required flags. |
| `disasm` | disassembles bytecode (json\|toml\|text, optional analysis). | Check `bpx help` for exact required flags. |
| `trace` | traces an execution path between nodes. | Check `bpx help` for exact required flags. |
| `call-args` | inspects call-node argument pins/defaults. | Check `bpx help` for exact required flags. |
| `refs` | reverse-searches soft-path usage on node pins. | Check `bpx help` for exact required flags. |
| `search` | token-searches nodes/pins in one blueprint package. | Check `bpx help` for exact required flags. |
| `infer-pack` | emits CFG/callsite/def-use inference artifacts. | Check `bpx help` for exact required flags. |
| `scan-functions` | aggregates function names across a directory. | Check `bpx help` for exact required flags. |

## Recommended Workflow

- Start with `bpx blueprint info` to identify candidate exports and confirm whether you are dealing with a Blueprint asset, a WidgetBlueprint, or a generated class payload.
- Use `search` to narrow by member, node, or symbol name before moving to heavier analysis commands.
- Reach for `trace` and `call-args` when you already know the node or function you care about and want a focused dependency path.
- Use `disasm` or `bytecode` only after you have identified the target export and execution slice; they are best for instruction-level inspection, not broad discovery.
- Prefer `refs` when you need reverse lookup from a soft object path back to the Blueprint nodes/pins that mention it.
- For WidgetBlueprint construction and edit workflows, switch to [bpx-widget](../bpx-widget/SKILL.md). Keep this skill focused on Blueprint analysis rather than widget tree authoring.

## Worked Recipe

```bash
bpx blueprint info ./Content/BP_Player.uasset
bpx blueprint search ./Content/BP_Player.uasset --member ApplyDamage --show name,class,member
bpx blueprint trace ./Content/BP_Player.uasset --from K2Node_CallFunction_12 --max-depth 6
bpx blueprint call-args ./Content/BP_Player.uasset --member ApplyDamage --all-pins
bpx blueprint disasm ./Content/BP_Player.uasset --export 21 --analysis --format text
```

- This is the preferred shape for "find a Blueprint function, inspect the call path, then disassemble the final target export".
- Use `search` and `trace` first so `disasm` stays focused on a specific export instead of becoming a broad dump.
- If your goal is WidgetBlueprint construction or widget property mutation, use [bpx-widget](../bpx-widget/SKILL.md) instead of treating this analysis recipe as an authoring workflow.

## Code-Aligned Caveats

- Large blueprints can produce very large payloads; constrain via `--limit`/`--max-steps`.
- `refs --include-routes` can be expensive; disable routes when doing broad scans.
- `disasm --entrypoint` implies analysis-oriented output.
- `trace` and `call-args` are most useful after `search` or `info` has already narrowed the target export/node set.
- For WidgetBlueprint edit flows and UE-openability caveats, prefer [bpx-widget](../bpx-widget/SKILL.md) over embedding widget-authoring guidance here.

## High-Signal Examples

```bash
bpx blueprint info ./Content/BP_Player.uasset
bpx blueprint search ./Content/BP_Player.uasset --member ApplyDamage --show name,class,member
bpx blueprint trace ./Content/BP_Player.uasset --from K2Node_CallFunction_12 --max-depth 6
bpx blueprint disasm ./Content/BP_Player.uasset --export 21 --analysis --format text
```
