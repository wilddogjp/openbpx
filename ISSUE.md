# Issue Guide

Thanks for opening an Issue for `bpx`.

## Before Creating an Issue

- Search existing Issues to avoid duplicates.
- Check [README.md](README.md), [docs/commands.md](docs/commands.md), and [SECURITY.md](SECURITY.md) first.
- For security vulnerabilities, do not open a public Issue. Use private reporting in [SECURITY.md](SECURITY.md).

## Issue Types

### Bug Report

Please include:

1. BPX version (`bpx version`) or commit SHA
2. Unreal Engine file version (if known)
3. Command and arguments used
4. Expected behavior
5. Actual behavior
6. Minimal reproduction asset/setup (if shareable)
7. Logs or error output

### Feature Request

Please include:

1. Problem statement
2. Proposed behavior
3. Why existing commands are insufficient
4. Compatibility and safety impact (if any)

### Documentation Request

Please include:

1. Target document path
2. Missing or unclear part
3. Suggested wording or structure

## Scope Reminder

`bpx` prioritizes binary safety:

- Unknown bytes must be preserved
- No-op round-trip must remain byte-identical
- High-risk structural rewrites may be explicitly unsupported

When proposing new behavior, include how safety guarantees are preserved.
