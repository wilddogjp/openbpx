---
name: bpx
description: Operate and troubleshoot BPX (`bpx`) and guide safe installation from official releases when needed. Use this skill to run or guide Unreal Engine 5.6 `.uasset` and `.umap` read/write commands safely and explain command output.
---

# BPX Installed CLI Skill

## Audience

- This skill is for users who need to run `bpx` in their local environment.
- Assume `bpx` is available on `PATH`.
- If `bpx` is not installed, use one of these commands:
  - macOS: `brew install --formula https://raw.githubusercontent.com/wilddogjp/openbpx/main/packaging/homebrew/openbpx.rb`
  - Debian/Ubuntu: `VER=0.1.4; ARCH="$(dpkg --print-architecture)"; curl -fsSLO "https://github.com/wilddogjp/openbpx/releases/download/v${VER}/openbpx_${VER}_${ARCH}.deb"; sudo dpkg -i "openbpx_${VER}_${ARCH}.deb"`
  - Windows: `pwsh -File ./scripts/install-bpx-from-release.ps1` (or `winget install --id WilddogJP.OpenBPX --exact`)

## Purpose

Use implemented BPX commands safely for UE 5.6 assets, with round-trip safety and predictable output.

## Output Format Recommendation

- Structured output defaults to `json`.
- For human review and LLM-driven post-processing, prefer `--format toml` unless CSV/TSV/text is explicitly needed.

## Command Coverage

- `find`: `assets`, `summary`
- `info`, `dump`, `validate`
- `export`: `list`, `info`, `set-header`
- `import`: `list`, `search`, `graph`
- `prop`: `list`, `set`, `add`, `remove`
- `write`
- `var`: `list`, `set-default`, `rename`
- `ref`: `rewrite`
- `package`: `meta`, `custom-versions`, `depends`, `resolve-index`, `section`, `set-flags`
- `localization`: `read`, `query`, `resolve`, `set-source`, `set-id`, `set-stringtable-ref`, `rewrite-namespace`, `rekey`
- `datatable`: `read`, `update-row`, `add-row`, `remove-row`
- `blueprint`: `info`, `bytecode`, `disasm`, `infer-pack`, `scan-functions`
- `enum`: `list`, `write-value`
- `name`: `list`, `add`, `set`, `remove`
- `struct`: `definition`, `details`
- `stringtable`: `read`, `write-entry`, `remove-entry`, `set-namespace`
- `class`
- `level`: `info`, `var-list`, `var-set`
- `raw`
- `metadata`: `inspect` (`bpx metadata <file.uasset> --export <n>`), `set-root`, `set-object`

## Safety Guardrails

- Preserve unknown bytes exactly.
- Prefer read/inspect commands before write commands.
- For writes, run `--dry-run` first.
- For real writes, use `--backup` unless the user explicitly declines.
- Keep UE 5.6 strict behavior unless compatibility work is explicitly requested.
- Return clear errors for invalid input.

## Standard Workflow (Installed CLI)

1. Confirm target file and command shape from built-in help (`bpx help` / `bpx help <command>`).
2. Run read commands to identify exact export/path/row/key targets.
3. Run the write command with `--dry-run`.
4. If output is correct, run the real write with `--backup`.
5. Re-run a read or `validate` command to confirm the result.

## Known Pitfalls and Fast Paths

- Metadata read has no `read` subcommand. Use `bpx metadata <file.uasset> --export <n>`.
- `find assets`, `find summary`, and `import graph` default to `--pattern "*.uasset"`. For map files, pass `--pattern "*.umap"`.
- `import graph` summarizes ImportMap dependencies only. For map reference tracing, combine `bpx blueprint disasm` and `rg -a` as needed.
- Large Widget Blueprint `blueprint disasm` output can be noisy or truncated. Start with `bpx export list <file.uasset> --class Function` and target specific exports.

## Completion Criteria

- Command usage matches implemented behavior.
- Safety options (`--dry-run`, `--backup`) are handled correctly.
- Output/report includes command, target file, and result.
- For source changes, local tests are executed and reported.
