---
name: bpx-shared
description: Shared BPX safety and execution guidance. Use before command-specific BPX skills.
---

# bpx shared

## Installation

- Prefer `bpx` from `PATH` when available.
- Plugin installs include a bundled fallback wrapper at `../../bin/bpx` (or `../../bin/bpx.cmd` on Windows) relative to this skill.
- Confirm with `<bpx-command> version` and `<bpx-command> help`; use the same resolved command in later examples.

## Safety Rules

- Treat assets as untrusted binary input.
- Prefer read commands before write commands.
- Run `--dry-run` first for write-capable commands.
- Use `--backup` when writing files in place.

## Standard Workflow

1. Inspect command shape with `bpx help <command>`.
2. Identify exact targets using read commands.
3. Run write command with `--dry-run`.
4. Execute real write with `--backup` when approved.
5. Validate with `bpx validate <file> --binary-equality` as needed.

## Global Rules

- Prefer read commands before write commands.
- Use `--dry-run` first for write-capable commands.
- Use `--backup` for in-place updates unless explicitly declined.
- For automation, prefer `--format toml` where available.

## Command Selection Heuristics

- Package shape/version checks: `info`, `dump`, `validate`.
- Export/import and reference analysis: `export`, `import`, `ref`, `raw`.
- Gameplay/content edits: `prop`, `var`, `datatable`, `localization`, `stringtable`, `level`.
- Blueprint analysis workflow: `blueprint info` -> `blueprint disasm` -> `blueprint trace/search`.

## Output Reading Tips

- Treat warnings as actionable signals; they often indicate partial decode paths.
- For write responses, inspect changed-byte and size-delta fields before applying.
- On errors, re-run `bpx help <command>` to confirm required flags and accepted forms.
