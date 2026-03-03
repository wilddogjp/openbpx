---
description: Operate and troubleshoot BPX (`bpx`) safely for Unreal Engine package assets. Use this skill to run read/write commands with explicit safety checks and predictable output.
---

# BPX Claude Plugin Skill

## Audience

- This skill is for users who want to use `bpx` from Claude Code.
- Assume `bpx` is available on `PATH`.
- If `bpx` is not installed, use one of these commands:
  - macOS: `brew install --formula https://raw.githubusercontent.com/wilddogjp/openbpx/main/packaging/homebrew/openbpx.rb`
  - Debian/Ubuntu: `VER=0.1.4; ARCH="$(dpkg --print-architecture)"; curl -fsSLO "https://github.com/wilddogjp/openbpx/releases/download/v${VER}/openbpx_${VER}_${ARCH}.deb"; sudo dpkg -i "openbpx_${VER}_${ARCH}.deb"`
  - Windows: `pwsh -File ./skills/bpx/scripts/install-bpx-from-release.ps1` (or `winget install --id WilddogJP.OpenBPX --exact`)

## Purpose

Use implemented BPX commands safely for UE 5.x assets, with round-trip safety and predictable output.

## Safety Guardrails

- Preserve unknown bytes exactly.
- Prefer read/inspect commands before write commands.
- For writes, run `--dry-run` first.
- For real writes, use `--backup` unless the user explicitly declines.
- Return clear errors for invalid input.

## Standard Workflow

1. Confirm target file and command shape from built-in help (`bpx help` / `bpx help <command>`).
2. Run read commands to identify exact export/path/row/key targets.
3. Run the write command with `--dry-run`.
4. If output is correct, run the real write with `--backup`.
5. Re-run a read or `validate` command to confirm the result.
