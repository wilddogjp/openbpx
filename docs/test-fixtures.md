# Test Plan and Fixture Specification

> **Design principle**: Following SQLite-style testing discipline, generate golden `before -> after` fixture pairs in UE 5.6 Editor and verify that `bpx` output is byte-identical to UE Editor output for equivalent operations.
> Parser bugs can directly corrupt assets, so test depth is treated as non-negotiable.

---

## UE Fixture Generation

The source-of-truth BPX fixture plugin lives at `testdata/BPXFixtureGenerator/` in this repository.
Sync it into the Lyra sample project's plugin directory before running the commandlet.

```bash
# Sync plugin from WSL (Lyra path)
./scripts/sync-bpx-plugin.sh --lyra-root <LYRA_ROOT>

# Generate fixtures (sync runs first internally)
./scripts/gen-fixtures.sh \
  --lyra-root <LYRA_ROOT> \
  --bpx-repo-root <REPO_ROOT> \
  --scope 1,2
```

`gen-fixtures` runs a `Build.bat` prebuild for the editor by default and launches `UnrealEditor-Cmd` with `-Unattended` to avoid blocking dialogs.
Use `--skip-editor-build` only if the editor build is already up to date and you need a faster loop.
`--scope` accepts `1 (read/validate)` or `2 (write/update)`.

## 0. Testing Philosophy

### Principles Inspired by SQLite

| SQLite practice | BPX equivalent |
|---|---|
| 100% branch coverage | Cover all `FPropertyTag` type branches and `FPackageIndex` sign branches |
| OOM-style failure injection | Safely fail on truncated input/EOF |
| Fuzz testing | go-fuzz / AFL++ style random mutation for `.uasset` |
| Golden testing | UE Editor output as oracle for `before -> after` equivalence |
| Regression retention | Add fixture for each fixed bug under `testdata/regression/` |
| Boundary testing | zero-length arrays, int32 min/max, empty strings, max-length strings |

### Top Invariants (must hold across all test layers)

1. **Round-trip invariant**: `Read(file) -> Write() == file` (byte-identical for no-op)
2. **Operation-equivalence invariant**: BPX edit output == UE Editor output for the same operation
3. **Error-safety invariant**: no panic on any input; invalid data returns explicit errors

---

## 1. Test Layer Model

```
Layer 5: Fuzz tests              - random mutated `.uasset` panic/hang detection
Layer 4: Operation equivalence   - BPX edit == UE Editor edit (golden pairs)
Layer 3: Round-trip tests        - Read->Write byte equality for all fixtures
Layer 2: Field-accuracy tests    - parsed fields checked against UE-defined expectations
Layer 1: Struct unit tests       - Summary, NameMap, Import/Export, etc.
Layer 0: Error/boundary tests    - invalid input, bounds, EOF must return errors
```

---

## 2. `testdata` Layout

```
testdata/
├── BPXFixtureGenerator/            # source-of-truth UE plugin (tracked)
├── manifest.json                   # fixture metadata and SHA-256
├── golden/
│   ├── parse/                      # parse-validation fixtures
│   ├── roundtrip/                  # helper marker for round-trip scope
│   ├── operations/                 # operation-equivalence before/after pairs
│   └── expected_output/            # expected CLI output fixtures
├── regression/                     # bug-fix regression fixtures
├── fuzz/
│   ├── corpus/                     # initial fuzz corpus
│   └── crashers/                   # crash-inducing samples (kept after fixes)
└── synthetic/                      # generated malformed/boundary fixtures
```

---

## 3. Golden Fixtures for Parse Validation (`golden/parse/`)

### 3.1 Blueprint Fixtures

| File | UE creation steps | Verification focus |
|---|---|---|
| `BP_Empty.uasset` | New Actor Blueprint, no vars/functions/nodes, save | Minimal Summary/Name/Import/Export shape |
| `BP_SimpleVars.uasset` | Add `MyBool`, `MyInt`, `MyFloat`, `MyString`, `MyName`, `MyVector`, `MyRotator` with defaults | `FBPVariableDescription` fields, type decoding |
| `BP_AllScalarTypes.uasset` | Add scalar vars (`Bool`, `Byte`, `Int`, `Int64`, `Float`, `Double`, `String`, `Name`, `Text`) with non-zero defaults | Full scalar `FPropertyTag` branches |
| `BP_MathTypes.uasset` | Add math vars (`Vector`, `Vector2D`, `Vector4`, `Rotator`, `Quat`, `Transform`, `LinearColor`, `Color`, `IntPoint`, `IntVector`, `Plane`) | Math struct decoding |
| `BP_RefTypes.uasset` | Add object/class/soft refs + enum variables | Reference and enum property handling |
| `BP_Containers.uasset` | Add `TArray`, `TSet`, `TMap` defaults | Container decode |
| `BP_Nested.uasset` | Add nested struct and nested container patterns | Nested parse correctness |
| `BP_GameplayTags.uasset` | Enable GameplayAbilities and add `GameplayTag` vars | GAS-related types |
| `BP_WithFunctions.uasset` | Add custom functions with nodes | Function Export + bytecode read |
| `BP_Inheritance.uasset` | 3-level inheritance (`BP_Base -> BP_Mid -> BP_Child`) | Parent import chain resolution |
| `BP_SoftRefs.uasset` | Set `SoftObjectPath` defaults | `FSoftObjectPath`, `package section --name soft-object-paths` |
| `BP_ManyImports.uasset` | Add references across many classes/packages | Large ImportMap parsing |
| `BP_WithMetadata.uasset` | Add Category/ToolTip/DisplayName metadata | Metadata export parsing |
| `BP_Unicode.uasset` | Add unicode variable names and text values (for example Japanese) | Wide-string NameMap handling |
| `BP_LargeArray.uasset` | Add 1000-element integer array | Performance and memory behavior |

### 3.2 Non-Blueprint Assets

| File | UE creation steps | Verification focus |
|---|---|---|
| `DT_Simple.uasset` | DataTable with basic row fields and multiple rows | DataTable read |
| `DT_Complex.uasset` | DataTable with nested/array/color fields | Complex DataTable decode |
| `E_Direction.uasset` | UserDefinedEnum with named values | Enum read |
| `S_PlayerData.uasset` | UserDefinedStruct with mixed fields | Struct definition read |
| `ST_UI.uasset` | StringTable namespace + entries | StringTable read |
| `L_Minimal.umap` | Minimal level with one actor | Level export structure |
| `MI_Chrome.uasset` | MaterialInstance with scalar/vector params | MaterialInstance parse |

### 3.3 Package-Structure-Oriented Fixtures

| File | UE creation steps | Verification focus |
|---|---|---|
| `BP_CustomVersions.uasset` | Save with multiple plugin systems enabled | `package custom-versions` |
| `BP_WithThumbnail.uasset` | Save with generated asset thumbnail | `package section --name thumbnails` |
| `BP_DependsMap.uasset` | Include multiple inter-asset dependencies | `package depends` |

---

## 4. Golden Operation Pairs (`golden/operations/`)

Each directory includes `before.uasset` and `after.uasset` (UE Editor output after performing the equivalent operation).
The BPX output is valid only when it is byte-identical to `after.uasset`.

### 4.1 `prop set` Operations

| Directory | UE operation | BPX command | Key concern |
|---|---|---|---|
| `prop_set_bool/` | `false -> true` | `bpx prop set ... --path MyBool --value 'true'` | bit-level update |
| `prop_set_int/` | `0 -> 42` | `bpx prop set ... --path MyInt --value '42'` | fixed-width rewrite |
| `prop_set_int_negative/` | `0 -> -1` | `bpx prop set ... --value '-1'` | signed handling |
| `prop_set_int_max/` | `0 -> 2147483647` | `bpx prop set ... --value '2147483647'` | int32 boundary |
| `prop_set_int_min/` | `0 -> -2147483648` | `bpx prop set ... --value '-2147483648'` | int32 boundary |
| `prop_set_int64/` | `0 -> 9223372036854775807` | `bpx prop set ...` | int64 boundary |
| `prop_set_float/` | `0.0 -> 3.14` | `bpx prop set ... --value '3.14'` | IEEE754 |
| `prop_set_float_special/` | `0.0 -> 1e-38` | `bpx prop set ... --value '1e-38'` | near-subnormal |
| `prop_set_double/` | `0.0 -> 2.718281828` | `bpx prop set ...` | 8-byte float |
| `prop_set_string_same_len/` | same-length replacement | `bpx prop set ... --value '"World"'` | no size delta |
| `prop_set_string_diff_len/` | different-length replacement | `bpx prop set ... --value '"Hello World"'` | offset recomputation |
| `prop_set_string_empty/` | non-empty to empty | `bpx prop set ... --value '""'` | FString empty form |
| `prop_set_string_unicode/` | ASCII to unicode string | `bpx prop set ...` | UTF path |
| `prop_set_name/` | Name value replacement | `bpx prop set ... --value '"NewName"'` | NameMap relation |
| `prop_set_text/` | Text value replacement | `bpx prop set ...` | FText serialization |
| `prop_set_enum/` | Enum value replacement | `bpx prop set ... --value '"NewValue"'` | enum-name mapping |
| `prop_set_vector/` | vector replacement | `bpx prop set ... --value '{"X":1.5,"Y":-2.3,"Z":100.0}'` | struct rewrite |
| `prop_set_rotator/` | rotator replacement | `bpx prop set ... --value '{"Pitch":45,"Yaw":90,"Roll":180}'` | struct rewrite |
| `prop_set_color/` | color replacement | `bpx prop set ... --value '{"R":1,"G":0,"B":0,"A":1}'` | struct rewrite |
| `prop_set_transform/` | transform field update | `bpx prop set ...` | compound struct |
| `prop_set_soft_object/` | soft path update | `bpx prop set ... --value '{"packageName":"/Game/New","assetName":"Asset"}'` | `FSoftObjectPath` |
| `prop_set_array_element/` | array element update | `bpx prop set ... --path 'MyArray[1]' --value '99'` | index targeting |
| `prop_set_map_value/` | map value update by key | `bpx prop set ... --path 'MyMap["key"]' --value '99'` | keyed update |
| `prop_set_nested_struct/` | inner struct field update | `bpx prop set ... --path 'Inner.IntVal' --value '42'` | dot-path traversal |
| `prop_set_nested_array_struct/` | nested array struct field update | `bpx prop set ... --path 'InnerArray[0].StrVal' --value '"new"'` | deep nesting |

### 4.2 DataTable Operations

| Directory | Operation | BPX command |
|---|---|---|
| `dt_update_int/` | update one integer field | `bpx datatable update-row ... --row Row_A --values '{"Score":999}'` |
| `dt_update_float/` | update one float field | `bpx datatable update-row ... --row Row_B --values '{"Rate":0.5}'` |
| `dt_update_string/` | update one string field | `bpx datatable update-row ... --row Row_A --values '{"Name":"NewName"}'` |
| `dt_update_multi_field/` | update multiple row fields | `bpx datatable update-row ... --values '{"Score":50,"Rate":0.1}'` |
| `dt_update_complex/` | update struct/array row fields | `bpx datatable update-row ...` |

### 4.3 Metadata / Package Operations

| Directory | Operation | BPX command |
|---|---|---|
| `metadata_set_tooltip/` | update ToolTip metadata | `bpx metadata set-root ...` |
| `metadata_set_category/` | update Category metadata | `bpx metadata set-root ...` |
| `export_set_header/` | update Export header field | `bpx export set-header ...` |
| `package_set_flags/` | update PackageFlags | `bpx package set-flags ...` |

### 4.4 Low-Risk Abstraction Operations

| Directory | UE operation | BPX command |
|---|---|---|
| `var_set_default_int/` | change default int | `bpx var set-default ... --name Score --value '100'` |
| `var_set_default_string/` | change default string | `bpx var set-default ... --name Title --value '"NewTitle"'` |
| `var_set_default_vector/` | change default vector | `bpx var set-default ... --name Position --value '{"X":1,"Y":2,"Z":3}'` |

### 4.5 High-Difficulty Operations

| Directory | UE operation | BPX command |
|---|---|---|
| `var_rename_simple/` | rename variable | `bpx var rename ... --from OldVar --to NewVar` |
| `var_rename_with_refs/` | rename variable used by graph refs | `bpx var rename ... --from UsedVar --to RenamedVar` |
| `var_rename_unicode/` | rename unicode variable | `bpx var rename ... --from <unicode_old> --to <unicode_new>` |
| `ref_rewrite_single/` | rewrite one soft path | `bpx ref rewrite ... --from "/Game/Old/Mesh" --to "/Game/New/Mesh"` |
| `ref_rewrite_multi/` | rewrite many soft paths | `bpx ref rewrite ... --from "/Game/OldDir" --to "/Game/NewDir"` |

### 4.6 `operation.json` Format

Each operation pair directory includes `operation.json`, consumed by the test runner.

```json
{
  "command": "prop set",
  "args": {
    "export": 1,
    "path": "MyInt",
    "value": "42"
  },
  "ue_procedure": "Open BP_SimpleVars, change MyInt default to 42, compile, save",
  "expect": "byte_equal",
  "notes": "Fixed-size int32 update; no file-size change"
}
```

---

## 5. Round-Trip Validation

### 5.1 Target Scope

All files under `golden/parse/` are round-trip targets.
Collection should stay directory-driven so new fixtures are auto-included.

### 5.2 Validation Logic

```text
for each .uasset in golden/parse/:
    asset = Parse(file)
    output = Serialize(asset)
    assert output == file  // byte-identical
```

### 5.3 Failure Diagnostics

- print first differing offset and section (Summary, NameMap, Import, Export, Property)
- show hex windows (for example +/- 32 bytes) for expected vs actual

---

## 6. Field-Accuracy Tests

Compare parsed results against UE-verified expected outputs in `golden/expected_output/*.json`.

### 6.1 Expected File Format

```json
{
  "oracle": "ue-fixture",
  "name": "export_list_BP_SimpleVars",
  "argv": ["export", "list", "testdata/golden/parse/BP_SimpleVars.uasset"],
  "expected": [
    {"index": 1, "objectName": "BP_SimpleVars", "className": "BlueprintGeneratedClass", "superIndex": -3},
    {"index": 2, "objectName": "Default__BP_SimpleVars", "className": "BP_SimpleVars_C"}
  ]
}
```

- `oracle` is required and must be `ue-fixture`.
- `TestFieldAccuracy` fails fixtures whose oracle is not `ue-fixture`.
- `scripts/gen_expected_output` emits helper snapshots as `oracle: "bpx-generated"`, so do not commit those directly as golden oracle files.

### 6.2 Helper Generation for `expected_output`

- default output of `go run ./scripts/gen_expected_output` is `testdata/reports/generated_expected_output/`
- only use `--allow-golden-overwrite` for intentional writes into `golden/expected_output/`
- after direct overwrite, keep `oracle` as `ue-fixture` and commit only UE-reviewed diffs

### 6.3 Coverage Targets

| Command | Primary fixtures |
|---|---|
| `info` | `BP_Empty`, `BP_SimpleVars`, `DT_Simple`, `E_Direction`, `S_PlayerData` |
| `export list` | `BP_Empty`, `BP_SimpleVars`, `BP_ManyImports`, `DT_Simple` |
| `export info` | `BP_SimpleVars` (all exports), `BP_WithFunctions` |
| `prop list` | `BP_AllScalarTypes`, `BP_MathTypes`, `BP_RefTypes`, `BP_Containers`, `BP_Nested` |
| `import list` | `BP_ManyImports`, `BP_Inheritance` |
| `package meta` | `BP_Empty`, `BP_CustomVersions` |
| `package custom-versions` | `BP_CustomVersions` |
| `package section --name soft-object-paths` | `BP_SoftRefs` |
| `var list` | `BP_SimpleVars`, `BP_Unicode` |
| `datatable read` | `DT_Simple`, `DT_Complex` |
| `enum list` | `E_Direction` |
| `struct definition` | `S_PlayerData` |
| `stringtable read` | `ST_UI` |
| `blueprint info` | `BP_WithFunctions` |
| `metadata` | `BP_WithMetadata` |

---

## 7. Error and Boundary Fixtures (`synthetic/`)

### 7.1 Generated synthetic fixtures

| File | Generation method | Validation target |
|---|---|---|
| `Empty.uasset` | zero-byte file | immediate EOF error |
| `NotAnAsset.bin` | random bytes | magic-tag mismatch |
| `BadMagic.uasset` | corrupted first 4 bytes | `PACKAGE_FILE_TAG` validation |
| `SwappedMagic.uasset` | swapped-endian magic | byte-order detection |
| `BP_Truncated_Summary.uasset` | cut inside summary | summary parse failure |
| `BP_Truncated_NameMap.uasset` | cut inside name map | name map parse failure |
| `BP_Truncated_ImportMap.uasset` | cut inside import map | import map parse failure |
| `BP_Truncated_ExportMap.uasset` | cut inside export map | export map parse failure |
| `BP_Truncated_ExportData.uasset` | cut inside export payload | property parse failure |
| `BP_BadNameIndex.uasset` | out-of-range NameIndex | name index bounds check |
| `BP_BadImportIndex.uasset` | out-of-range ImportIndex | import resolution failure |
| `BP_BadExportSize.uasset` | oversized SerialSize | export payload bounds check |
| `BP_NegativeCount.uasset` | negative NameCount | signed count validation |
| `BP_HugeCount.uasset` | huge NameCount | allocation limit checks |
| `BP_ZeroExports.uasset` | ExportCount=0 | empty export table behavior |
| `BP_CircularImport.uasset` | self-referential OuterIndex | cycle detection |

### 7.2 Version-window fixtures

| File | Content | Validation target |
|---|---|---|
| `BP_UE55.uasset` | header-mutated `FileVersionUE5=1014` | parser path smoke for in-window legacy version (synthetic) |
| `BP_UE54.uasset` | header-mutated `FileVersionUE5=1009` | parser path smoke for in-window legacy version (synthetic) |
| `BP_FutureVersion.uasset` | tampered `FileVersionUE5=9999` | unknown-version rejection |

### 7.3 `gen_synthetic.go`

`testdata/synthetic/gen_synthetic.go` centralizes synthetic fixture generation.
It mutates specific offsets on top of valid fixtures to produce error cases.

```go
//go:generate go run gen_synthetic.go
```

---

## 8. Fuzz Testing

### 8.1 Fuzz targets

| Target function | Input | Goal |
|---|---|---|
| `FuzzParseSummary` | `[]byte` | detect parser panic/hang in summary path |
| `FuzzParseNameMap` | `[]byte` | name map parser hardening |
| `FuzzParseImportMap` | `[]byte` | import map parser hardening |
| `FuzzParseExportMap` | `[]byte` | export map parser hardening |
| `FuzzParsePropertyTag` | `[]byte` | full property-tag branch stress |
| `FuzzParseAsset` | full `.uasset` bytes | integrated parser stress |
| `FuzzRoundTrip` | full `.uasset` bytes | parse-serialize invariant stress |

### 8.2 Fuzz corpus

Initialize corpus primarily from all `.uasset` files in `golden/parse/`.

### 8.3 CI integration

- run short fuzz windows (30s) on every PR
- run long fuzz windows (1h) on weekly schedule
- preserve discovered crashers under `fuzz/crashers/` after fixes

---

## 9. How to Build UE Operation Pairs

### 9.0 Plugin source vs deploy target

- plugin source-of-truth: `<REPO_ROOT>/testdata/BPXFixtureGenerator/`
- Lyra deploy target: `<LYRA_ROOT>/Plugins/BPXFixtureGenerator`
- deploy via `scripts/sync-bpx-plugin.ps1` / `scripts/sync-bpx-plugin.sh` only
- always run sync before fixture generation

### 9.1 Standard workflow

1. Open the Lyra sample project in UE 5.6 editor.
2. Create assets under `Content/TestFixtures/Operations/<case>/`.
3. Save initial state and copy as `before.uasset`.
4. Perform intended editor operation (value update, rename, etc.).
5. Save result and copy as `after.uasset`.
6. Record procedure in `README.md` and command args in `operation.json`.
7. Verify byte diff changes only intended regions.

### 9.2 Important rules

- Save once per state whenever possible to reduce incidental metadata churn.
- Create `before` and `after` in the same editor session when possible.
- Compile blueprint only as needed for operation consistency.
- Disable autosave during fixture generation.
- Record unavoidable unrelated byte changes in `operation.json` `ignore_offsets`.

### 9.3 `ignore_offsets` handling

UE may recompute bytes like `SavedHash` on save even when operation intent is unrelated.
These must be explicitly documented and ignored by the comparison runner.

```json
{
  "ignore_offsets": [
    {"offset": 145, "length": 20, "reason": "FPackageFileSummary.SavedHash (FSHA1)"}
  ]
}
```

Add `ignore_offsets` minimally and always with UE-source-backed rationale.

---

## 10. Test Runner Design

### 10.1 Go test function map

```go
// Layer 1: struct unit tests
func TestParseSummary(t *testing.T)
func TestParseNameMap(t *testing.T)
func TestParseImportMap(t *testing.T)
func TestParseExportMap(t *testing.T)
func TestParsePropertyTag(t *testing.T)

// Layer 2: field accuracy
func TestFieldAccuracy(t *testing.T)

// Layer 3: round-trip
func TestRoundTrip(t *testing.T)

// Layer 4: operation equivalence
func TestOperationEquivalence(t *testing.T)

// Layer 5: fuzz
func FuzzParseAsset(f *testing.F)
func FuzzRoundTrip(f *testing.F)

// Layer 0: synthetic errors
func TestSyntheticErrors(t *testing.T)
func TestVersionReject(t *testing.T)
```

### 10.2 Test commands

```bash
# all tests
go test ./... -v

# layer-specific
go test ./pkg/uasset/ -run TestParse -v
go test ./pkg/validate/ -run TestFieldAccuracy -v
go test ./pkg/validate/ -run TestRoundTrip -v
go test ./pkg/validate/ -run TestOperation -v
go test ./pkg/uasset/ -fuzz FuzzParseAsset -fuzztime 30s
go test ./pkg/validate/ -run TestSynthetic -v

# CI-style
go test ./... -v && go test ./pkg/uasset/ -fuzz FuzzParseAsset -fuzztime 30s
```

### 10.3 Generation scripts (operational example)

```bash
./scripts/sync-bpx-plugin.sh --lyra-root <LYRA_ROOT>

./scripts/gen-fixtures.sh \
  --lyra-root <LYRA_ROOT> \
  --bpx-repo-root <REPO_ROOT> \
  --scope 1,2
```

---

## 11. Fixture Inventory Summary

### Golden parse fixtures: 27 files

| Category | Count | Purpose |
|---|---|---|
| Blueprint core/types | 16 | parse, round-trip, field accuracy |
| Non-Blueprint assets | 7 | DataTable, Enum, Struct, StringTable, Level, Material |
| Package-structure focused | 3 | CustomVersions, Thumbnail, DependsMap |
| Version variants (UE 5.5/5.4) | 2 | in-window parser-path smoke (synthetic) |

### Operation pairs: 42 directories

| Category | Pair count | Purpose |
|---|---|---|
| scalar `prop set` | 16 | scalar value rewrites |
| struct/reference `prop set` | 5 | Vector/Rotator/Color/Transform/SoftObjectPath |
| container/nested `prop set` | 4 | array/map/nested rewrites |
| DataTable ops | 5 | row updates |
| metadata/package ops | 4 | ToolTip/Category/Header/Flags |
| `var set-default` | 3 | existing default value rewrites |
| high-level BP edits | 5 | `var rename`, `ref rewrite` |

### Synthetic fixtures: 19 files

| Category | Count | Purpose |
|---|---|---|
| empty/non-asset | 2 | basic invalid input |
| bad magic | 2 | header validation |
| truncated sections | 5 | EOF resilience |
| index/size tampering | 7 | bounds checks |
| version tampering | 1 | unknown version reject |
| circular reference | 1 | import reference validation |

Total: **132 test artifacts** (fixtures + operation assets)

---

## 12. Milestone-to-Test Rollout

| Milestone | Test layers | Required fixtures | Goal |
|---|---|---|---|
| **1** (read) | Layers 0,1,2,3,5 | all parse + all synthetic + expected outputs | parser completeness, round-trip, invalid-input safety |
| **2** (low-risk write) | Layer 4 add | `prop_set`, `dt_update`, metadata/package, `var_set_default` pairs | UE-equivalent existing-value updates |
| **3** (high-risk edits) | Layer 4 add | `var_rename`, `ref_rewrite` pairs | reference-consistent high-level edits |
| **4** (safety hardening) | all layers harden | expanded regressions + longer fuzz windows | edge-case coverage |

---

## 13. CI Integration Example

```yaml
# .github/workflows/test.yml (outline)
jobs:
  test:
    steps:
      - run: go test ./... -v
      - run: go test ./pkg/uasset/ -fuzz FuzzParseAsset -fuzztime 30s

  fuzz-weekly:
    schedule: "0 3 * * 0"
    steps:
      - run: go test ./pkg/uasset/ -fuzz FuzzParseAsset -fuzztime 1h
      - run: go test ./pkg/uasset/ -fuzz FuzzRoundTrip -fuzztime 1h
```

---

## 14. `manifest.json`

`manifest.json` tracks fixture metadata and hashes so CI can detect unintended fixture drift.

```json
{
  "version": 1,
  "generated": "2026-XX-XX",
  "fixtures": [
    {
      "path": "golden/parse/BP_Empty.uasset",
      "description": "Empty Actor Blueprint fixture",
      "ue_version": "5.6",
      "sha256": "<hash>",
      "size_bytes": 0,
      "layers": ["roundtrip", "parse", "field_accuracy"],
      "milestone": "1"
    },
    {
      "path": "golden/operations/prop_set_int/before.uasset",
      "description": "prop set int: before state",
      "ue_version": "5.6",
      "sha256": "<hash>",
      "size_bytes": 0,
      "layers": ["operation_equivalence"],
      "milestone": "2",
      "pair": "golden/operations/prop_set_int/after.uasset"
    }
  ]
}
```
