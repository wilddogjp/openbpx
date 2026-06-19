---
name: bpx-datatable
description: BPX `datatable` command skill. Read DataTable-family rows and update DataTable rows.
---

# datatable

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx datatable read <file.uasset> [--export <n>] [--row <name>] [--format json|toml|csv|tsv] [--out path]
bpx datatable update-row <file.uasset> --row <name> --values '<json>' [--export <n>] [--dry-run] [--backup]
bpx datatable add-row <file.uasset> --row <name> [--values '<json>'] [--export <n>] [--dry-run] [--backup]
bpx datatable remove-row <file.uasset> --row <name> [--export <n>] [--dry-run] [--backup]
```

## Behavior

- `read`: decodes DataTable/CurveTable/CompositeDataTable rows.
- `update-row`: patches fields in an existing row (DataTable exports only).
- `add-row`: appends a new row (DataTable only; row name must resolve in NameMap).
- `remove-row`: removes a row by name (DataTable only).
- `read --format csv|tsv` flattens rows for spreadsheet-style output.

> [!CAUTION]
> This command includes write-capable operations. Confirm intent and run `--dry-run` first.

## Command Matrix

| Command | Use when | Notable defaults |
|------|------|------|
| `read` | decodes DataTable/CurveTable/CompositeDataTable rows. | Read-only path; safe for discovery. |
| `update-row` | patches fields in an existing row (DataTable exports only). | Run `--dry-run` first and use `--backup` for real writes. |
| `add-row` | appends a new row (DataTable only; row name must resolve in NameMap). | Run `--dry-run` first and use `--backup` for real writes. |
| `remove-row` | removes a row by name (DataTable only). | Run `--dry-run` first and use `--backup` for real writes. |

## Code-Aligned Caveats

- Row mutations target DataTable exports only; non-DataTable types are rejected.
- When using CSV/TSV output, confirm flattened values before patching rows.

## High-Signal Examples

```bash
bpx datatable read ./Sample.uasset [--export 1] [--row SampleName] [--format json|toml|csv|tsv] [--out path]
bpx datatable update-row ./Sample.uasset --row SampleName --values '{"value":1}' [--export 1] [--dry-run] [--backup]
bpx datatable add-row ./Sample.uasset --row SampleName [--values '{"value":1}'] [--export 1] [--dry-run] [--backup]
bpx datatable remove-row ./Sample.uasset --row SampleName [--export 1] [--dry-run] [--backup]
```
