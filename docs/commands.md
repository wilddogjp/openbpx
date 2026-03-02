# Command Specification

This document defines `bpx` capabilities by implementation phase and risk level.

## Read / Validate Commands

These are read/validate commands (implemented plus planned additions).

| Category | `bpx` Command | Status | Notes |
|---|---|---|---|
| Meta | `bpx version` | ✅ Implemented | Prints CLI semantic version |
| Asset Search | `bpx find assets <directory> [--pattern "*.uasset"] [--recursive]` | ✅ Implemented | `.umap` is also included |
| Asset Search | `bpx find summary <directory> [--pattern "*.uasset"] [--recursive] [--format json\|toml] [--out <path>]` | ✅ Implemented | Cross-directory summary aggregation (default: `*.uasset`) |
| Asset Info | `bpx info <file.uasset>` | ✅ Implemented | Basic metadata |
| Asset Dump | `bpx dump <file.uasset> [--format json\|toml\|yaml] [--out path]` | ✅ Implemented | JSON/TOML/YAML output |
| Validation | `bpx validate <file.uasset> [--binary-equality]` | ✅ Implemented | Includes additional integrity checks |
| Export | `bpx export list <file.uasset> [--class <token>]` | ✅ Implemented | Export listing |
| Export | `bpx export info <file.uasset> --export <n>` | ✅ Implemented | Header details |
| Property | `bpx prop list <file.uasset> --export <n>` | ✅ Implemented | Always includes decoded values |
| NameMap | `bpx name list <file.uasset>` | ✅ Implemented | NameMap listing |
| Import | `bpx import list <file.uasset>` | ✅ Implemented | Import listing |
| Import | `bpx import search <file.uasset> --object <name> [--class-package <pkg>] [--class-name <cls>]` | ✅ Implemented | Name-based search |
| Import | `bpx import graph <directory> [--pattern "*.uasset"] [--recursive] [--group-by root\|object] [--filter <token>]` | ✅ Implemented | ImportMap dependency graph aggregation |
| Package | `bpx package meta <file.uasset>` | ✅ Implemented | GUID/flags/version |
| Package | `bpx package custom-versions <file.uasset>` | ✅ Implemented | Custom version listing |
| Package | `bpx package depends <file.uasset>` | ✅ Implemented | DependsMap listing |
| Package | `bpx package resolve-index <file.uasset> --index <i>` | ✅ Implemented | `FPackageIndex` resolution |
| Package | `bpx package section <file.uasset> --name <section>` | ✅ Implemented | Includes `soft-object-paths` and other raw sections |
| Localization | `bpx localization read <file.uasset> [--export <n>] [--include-history] [--format json\|toml\|csv]` | ✅ Implemented | Lists TextProperty + GatherableTextData (`--export` excludes GatherableTextData) |
| Localization | `bpx localization query <file.uasset> [--export <n>] [--namespace <ns>] [--key <key>] [--text <token>] [--history-type <type>] [--limit <n>]` | ✅ Implemented | Filters by namespace/key/historyType/etc. |
| Localization | `bpx localization resolve <file.uasset> [--export <n>] --culture <culture> [--locres <path>] [--missing-only]` | ✅ Implemented | Preview resolved display strings via `.locres` |
| DataTable | `bpx datatable read <file.uasset> [--export <n>] [--row <name>] [--format json\|toml\|csv\|tsv] [--out path]` | ✅ Implemented | Reads all rows by default; `--row` filters; `CompositeDataTable` includes `compositeParents` |
| Blueprint | `bpx blueprint info <file.uasset> [--export <n>]` | ✅ Implemented | Function/blueprint summary |
| Blueprint | `bpx blueprint bytecode <file.uasset> --export <n> [--range-source auto\|export-map\|ustruct-script\|serial-full] [--strict-range] [--diagnostics]` | ✅ Implemented | Raw bytecode (base64) |
| Blueprint | `bpx blueprint disasm <file.uasset> --export <n> [--format json\|toml\|text] [--analysis] [--entrypoint <vm>] [--max-steps <n>] [--range-source auto\|export-map\|ustruct-script\|serial-full] [--strict-range] [--diagnostics]` | ✅ Implemented | `--analysis` adds inference metadata; for large blueprints, narrow with `export list --class Function` first |
| Blueprint | `bpx blueprint trace <file.uasset> --from <Node\|Node.Pin> [--to-node <token>] [--to-function <token>] [--max-depth <n>]` | ✅ Implemented | Returns shortest exec-link route across K2 nodes |
| Blueprint | `bpx blueprint call-args <file.uasset> --member <token> [--class <token>] [--all-pins] [--include-exec]` | ✅ Implemented | Lists input pin defaults/connections for call nodes |
| Blueprint | `bpx blueprint refs <file.uasset> --soft-path <path> [--class <token>] [--include-routes] [--max-routes <n>] [--max-depth <n>]` | ✅ Implemented | Reverse lookup for pin usage of soft paths, optional event routes |
| Blueprint | `bpx blueprint search <file.uasset> [--class <token>] [--member <token>] [--name <token>] [--show <fields>] [--limit <n>]` | ✅ Implemented | Single-shot cross-export search |
| Blueprint | `bpx blueprint infer-pack <file.uasset> --export <n> [--entrypoint <vm>] [--max-steps <n>] [--out <dir>] [--range-source auto\|export-map\|ustruct-script\|serial-full] [--strict-range] [--diagnostics]` | ✅ Implemented | Exports CFG/callsite/def-use/signature pack |
| Blueprint | `bpx blueprint scan-functions <directory> --recursive [--name-like <regex>] [--aggregate]` | ✅ Implemented | Cross-function analysis |
| Enum | `bpx enum list <file.uasset>` | ✅ Implemented | Enum read |
| Struct | `bpx struct definition <file.uasset>` | ✅ Implemented | UDS definition read |
| StringTable | `bpx stringtable read <file.uasset>` | ✅ Implemented | String table KV read |
| Class | `bpx class <file.uasset> --export <n>` | ✅ Implemented | ClassExport read |
| Level | `bpx level info <file.umap> --export <n>` | ✅ Implemented | LevelExport read |
| Level | `bpx level var-list <file.umap> --actor <name\|PersistentLevel.Name\|export-index>` | ✅ Implemented | Resolves placed objects via `OuterIndex -> PersistentLevel` and returns decoded variables |
| Raw Export | `bpx raw <file.uasset> --export <n>` | ✅ Implemented | Raw payload base64 |
| Metadata | `bpx metadata <file.uasset> --export <n>` | ✅ Implemented | MetaDataExport read |
| Struct Export | `bpx struct details <file.uasset> --export <n>` | ✅ Implemented | StructExport details |

## Update Commands (Existing-Element Updates)

Update commands are intentionally limited to existing value updates or abstraction output, avoiding high-risk structural rewiring.

Common write-command behavior:

- `--dry-run`: report planned changes without writing
- `--backup`: create `<target>.backup` before overwrite when target exists
- In-place `.uasset` updates always create `XXX.uasset.backup` first

| Category | `bpx` Command | Status | Risk | Notes |
|---|---|---|---|---|
| Property | `bpx prop set <file.uasset> --export <n> --path <dot.path> --value '<json>'` | ✅ Implemented | Low | Supports `A.B[0].C`, `Map["k"]`, `Map[42]`, `Map[true]`; recomputes `Tag.Size`; updates bool tag flags |
| DataTable | `bpx datatable update-row <file.uasset> --row <name> --values '<json>'` | ✅ Implemented | Low | Existing-row update only; only `DataTable` class supported (`CompositeDataTable` / `CurveTable` rejected) |
| Export | `bpx export set-header <file.uasset> --index <n> --fields '<json>'` | ✅ Implemented | Low | Existing header fields only |
| Package | `bpx package set-flags <file.uasset> --flags <enum-or-raw>` | ✅ Implemented | Low | `PKG_FilterEditorOnly` / `PKG_UnversionedProperties` changes are not supported |
| Metadata | `bpx metadata set-object <file.uasset> --export <n> --import <i> --key <k> --value <v>` | ✅ Implemented | Low | Import metadata |
| Metadata | `bpx metadata set-root <file.uasset> --export <n> --key <k> --value <v>` | ✅ Implemented | Low | Root metadata |
| Save | `bpx write <file.uasset> --out <new.uasset>` | ✅ Implemented | Low | Recomputes summary offsets and `Export.SerialOffset` after payload relocation |
| Variable | `bpx var list <file.uasset>` | ✅ Implemented | Low | Uses CDO defaults and returns `source`/`mismatch`; merges declaration names even when `NewVariables` appears as `rawBase64` |
| Variable | `bpx var set-default <file.uasset> --name <var> --value '<json>'` | ✅ Implemented | Low | Uses same write engine as `prop set`; type mismatch returns error |
| NameMap | `bpx name set <file.uasset> --index <n> --value <name> [--non-case-hash <u16>] [--case-preserving-hash <u16>]` | ✅ Implemented | Medium | Updates existing index value/hash; computes UE5.6-compatible hashes when omitted |
| Level | `bpx level var-set <file.umap> --actor <name\|PersistentLevel.Name\|export-index> --path <dot.path> --value '<json>'` | ✅ Implemented | Low | Uses same write engine as `prop set`; actor resolution via `OuterIndex -> PersistentLevel` |
| Enum | `bpx enum write-value <file.uasset> --export <n> --name <k> --value <v>` | ✅ Implemented | Medium | Existing enum values only |
| StringTable | `bpx stringtable write-entry <file.uasset> --export <n> --key <k> --value <v>` | ✅ Implemented | Medium | Existing key update only |
| Localization | `bpx localization set-source <file.uasset> --export <n> --path <dot.path> --value <text>` | ✅ Implemented | Low | Existing TextProperty source-string update |
| Localization | `bpx localization set-id <file.uasset> --export <n> --path <dot.path> --namespace <ns> --key <key>` | ✅ Implemented | Medium | Existing TextProperty namespace/key update |
| Localization | `bpx localization set-stringtable-ref <file.uasset> --export <n> --path <dot.path> --table <table-id> --key <key>` | ✅ Implemented | Medium | Converts existing TextProperty to StringTable reference |

## High-Level Edits / Structural Changes

This area is managed as two groups:

- `Safe`: implemented with explicit safety constraints on existing export-payload rewrite foundations
- `Unsupported`: intentionally blocked due to high-risk structural rewiring needs

### Safe (Implemented Scope)

| Category | `bpx` Command | Status | Risk | Notes |
|---|---|---|---|---|
| Variable | `bpx var rename <file.uasset> --from <old> --to <new>` | ✅ Implemented | High | Bulk rename via NameMap references; warns and still performs NameMap-level rename when declaration is missing |
| Reference | `bpx ref rewrite <file.uasset> --from "/Game/Old" --to "/Game/New"` | ✅ Implemented | High | Cross-rewrites NameMap + decodable SoftObjectPath/Text/Str references |
| NameMap | `bpx name add <file.uasset> --value <name> [--non-case-hash <u16>] [--case-preserving-hash <u16>]` | ✅ Implemented | High | Appends entry; rejects duplicates; auto-computes hashes when omitted |
| NameMap | `bpx name remove <file.uasset> --index <n>` | ✅ Implemented | High | Safety constraints: tail index only, cannot remove indices within `NamesReferencedFromExportDataCount` or active Import/Export refs |
| Property | `bpx prop add <file.uasset> --export <n> --spec '<json>'` | ✅ Implemented | High | Top-level property add; `spec.type` currently limited to non-nested types |
| Property | `bpx prop remove <file.uasset> --export <n> --path <dot.path>` | ✅ Implemented | High | Top-level property remove (optional `[arrayIndex]`) |
| DataTable | `bpx datatable add-row <file.uasset> --row <name> [--values '<json>']` | ✅ Implemented | High | Uses existing NameMap only; no new Name insertion; `DataTable` class only |
| DataTable | `bpx datatable remove-row <file.uasset> --row <name>` | ✅ Implemented | High | Structural row removal; `DataTable` class only |
| StringTable | `bpx stringtable remove-entry <file.uasset> --export <n> --key <k>` | ✅ Implemented | High | Removes existing key from serialized StringTable payload |
| StringTable | `bpx stringtable set-namespace <file.uasset> --export <n> --namespace <ns>` | ✅ Implemented | High | Updates serialized StringTable namespace |
| Localization | `bpx localization rewrite-namespace <file.uasset> --from <ns-old> --to <ns-new>` | ✅ Implemented | High | Cross-updates namespace for TextProperty + GatherableTextData |
| Localization | `bpx localization rekey <file.uasset> --namespace <ns> --from-key <k-old> --to-key <k-new>` | ✅ Implemented | High | Cross-updates keys for TextProperty + GatherableTextData |

### Unsupported (Intentionally Blocked)

| Category | `bpx` Command | Status | Handling | Reason |
|---|---|---|---|---|
| Import | `bpx import add <file.uasset> --spec '<json>'` | ❌ Not implemented | ⛔ Unsupported | Requires ImportMap insertion and `FPackageIndex` rewiring |
| Import | `bpx import remove <file.uasset> --index <n>` | ❌ Not implemented | ⛔ Unsupported | Requires global import-reference recalculation |
| Export | `bpx export add <file.uasset> --spec '<json>'` | ❌ Not implemented | ⛔ Unsupported | Requires ExportMap/header/dependency reconstruction |
| Export | `bpx export remove <file.uasset> --index <n>` | ❌ Not implemented | ⛔ Unsupported | Requires global export-reference recalculation |
| Export | `bpx export clone <file.uasset> --index <n>` | ❌ Not implemented | ⛔ Unsupported | Deep-copy reference adjustment is high risk |
| Export | `bpx export set-raw-data <file.uasset> --index <n> --data <base64>` | ❌ Not implemented | ⛔ Unsupported | Arbitrary payload rewrite has high corruption risk |
| Variable | `bpx var add <file.uasset> --name <name> --type <type> --default '<json>'` | ❌ Not implemented | ⛔ Unsupported | Declaration insertion + reference consistency is not established |
| Variable | `bpx var remove <file.uasset> --name <name>` | ❌ Not implemented | ⛔ Unsupported | High destructive risk if references remain |
| Enum | `bpx enum remove-value <file.uasset> --export <n> --name <k>` | ❌ Not implemented | ⛔ Unsupported | Reference-integrity guarantees are not established |
| Localization | `bpx localization sync-locres <file.uasset> --culture <culture> --locres <path> [--mode patch\|replace]` | ❌ Not implemented | ⛔ Unsupported | External locres sync conflict policy is undefined |
| Bytecode | `bpx blueprint set-bytecode-raw <file.uasset> --export <n> --data <base64>` | ⛔ Unsupported | ⛔ Unsupported | High-risk operation intentionally blocked |

## `bpx localization` Scope

Localization operations focus on `TextProperty` and `GatherableTextData`.

- `read`: list localization targets (`namespace`, `key`, `source`, `historyType`, etc.)
- `query`: filter by `namespace` / `key` / `text` / `historyType`
- `resolve`: preview resolved display strings via `--culture` and optional `.locres`
- `set-source`: update only source string of existing TextProperty
- `set-id`: update namespace/key of existing TextProperty
- `set-stringtable-ref`: switch existing TextProperty to StringTable-reference form (no new key insertion)
- `rewrite-namespace` / `rekey` / `sync-locres`: broader rewiring or external locres synchronization (higher-risk domain)

## Decode Coverage and JSON Output Format

`bpx prop list` and `bpx datatable read` always run in decode mode.
`--decode` / `--raw` are removed.

> Structured-output commands share `--format json|toml` (default: `json`).

### Common Output Rules

- Each property includes `name`, `type`, `size`, `arrayIndex`, `offset`, `valueOffset`, `flags`.
- `value` is included only when decode succeeds.
- If decode fails, `value` is omitted and command execution continues (safe fallback).

### JSON Output Coverage

| Item | Status | Notes |
|---|---|---|
| Common property keys (`name`, `type`, `size`, `arrayIndex`, `offset`, `valueOffset`, `flags`) | ✅ Implemented | emitted for all properties |
| `value` on decode success | ✅ Implemented | typed structured output |
| safe fallback when decode fails (`value` omitted) | ✅ Implemented | command does not fail globally |
| generic struct fallback (`rawBase64`) | ✅ Implemented | when recursive generic decode fails |
| detailed Text history reconstruction by type | ✅ Implemented | UE5.6 `ETextHistoryType` (`Base..TextGenerator` + `None`) |
| full unsigned preservation for `UInt64Property` | ✅ Implemented | kept as `uint64` in JSON |
| legacy `FName AssetPathName + FString SubPath` `FSoftObjectPath` (UE5 pre-1007) | ✅ Implemented | UE5 `1000..1006` branch supported |

### Decoded Types (Scalar / Reference)

| UE Type | `value` Output | Status | Notes |
|---|---|---|---|
| `BoolProperty` | `true` / `false` | ✅ Implemented | |
| `ByteProperty` | `123` | ✅ Implemented | |
| `Int8Property` / `Int16Property` / `IntProperty` / `Int64Property` | integer | ✅ Implemented | |
| `UInt16Property` / `UInt32Property` / `UInt64Property` | integer | ✅ Implemented | `UInt64Property` stays unsigned |
| `FloatProperty` / `DoubleProperty` | floating-point | ✅ Implemented | |
| `NameProperty` | `{"index":0,"number":0,"name":"None"}` | ✅ Implemented | |
| `StrProperty` | `"text"` | ✅ Implemented | |
| `EnumProperty` | `{"enumType":"EMyEnum","value":"EMyEnum::ValueA"}` | ✅ Implemented | falls back to numeric when name resolve fails |
| `ObjectProperty` / `ClassProperty` / `WeakObjectProperty` / `LazyObjectProperty` / `InterfaceProperty` | `{"index":1,"resolved":"..."}` | ✅ Implemented | |
| `SoftObjectProperty` / `SoftObjectPathProperty` / `SoftClassPathProperty` | `{"packageName":"...","assetName":"...","subPath":"..."}` | ✅ Implemented | UE5 legacy/new layouts + summary-list index decode |
| `TextProperty` | `{"flags":0,"historyType":"NamedFormat","historyTypeCode":1,...,"displayString":"..."}` | ⚠️ Partial | history payloads covered for UE5.6; `.locres` resolution is via `bpx localization resolve` |
| `DelegateProperty` | `{"object":0,"resolved":"...","delegate":"FunctionName"}` | ✅ Implemented | |
| `MulticastDelegateProperty` / `MulticastInlineDelegateProperty` / `MulticastSparseDelegateProperty` | array of DelegateProperty | ✅ Implemented | |
| `FieldPathProperty` | `{"path":["FieldA","FieldB"],"resolvedOwner":0}` | ✅ Implemented | |

### Decoded Types (Container / Struct)

| UE Type | `value` Output | Status | Notes |
|---|---|---|---|
| `ArrayProperty` | `{"arrayType":"IntProperty","value":[{"type":"IntProperty","value":1},...]}` | ✅ Implemented | |
| `SetProperty` | `{"setType":"NameProperty","value":[{"type":"NameProperty","value":...},...]}` | ✅ Implemented | |
| `MapProperty` | `{"keyType":"StrProperty","valueType":"IntProperty","value":[{"key":{...},"value":{...}},...]}` | ✅ Implemented | |
| `StructProperty` | `{"structType":"Vector","value":...}` | ✅ Implemented | fixed structs use dedicated decoders; others use recursive tagged decode |

#### StructProperty Fixed-Struct Shortcuts

- `Vector`, `Rotator`, `Quat`, `Vector2D`, `Vector4`, `Plane`
- `Vector2f`, `Vector3f`, `Vector4f`
- `LinearColor`, `Color`
- `IntPoint`, `IntVector`, `IntVector2`
- `Box`, `Matrix`, `TwoVectors`
- `Guid`, `DateTime`, `Timespan`
- `SoftObjectPath`, `SoftClassPath`
- `GameplayTag`, `GameplayTagContainer`
- `FloatRange`

#### UE-Defined Custom-Serializer Structs (Explicit Support)

- `LevelViewportInfo`
  - decode implemented (`CamPosition/CamRotation/CamOrthoZoom/CamUpdated` from `Array<StructProperty(LevelViewportInfo)>`)
  - source: `Runtime/Engine/Classes/Engine/World.h`, `Runtime/Engine/Private/World.cpp` (`UWorld::Serialize`)
- `PerQualityLevelInt`, `PerQualityLevelFloat`
  - decode/write implemented (`bCooked`, `Default`, `PerQuality(TMap<int32, ...>)`)
  - source: `Runtime/Engine/Public/PerQualityLevelProperties.h`, `Runtime/Engine/Private/PerQualityLevelProperties.cpp`
- `UniqueNetIdRepl`
  - decode/write implemented (persistent-archive layout: `Size`, `Type(FName)`, `Contents(FString)`)
  - source: `Runtime/Engine/Classes/GameFramework/OnlineReplStructs.h`, `Runtime/Engine/Private/OnlineReplStructs.cpp`
- `FrameNumber`
  - decode/write implemented (`int32`)
  - source: `Runtime/CoreUObject/Private/UObject/Property.cpp` (`TStructOpsTypeTraits<FFrameNumber>`)
- `NiagaraVariableBase`
  - decode implemented (custom-serialized head: `Name(FName)` + `TypeDefHandle(FNiagaraTypeDefinition)`)
  - source: Niagara module serialization implementation
- `NiagaraVariant`
  - decode implemented (`Object`, `DataInterface`, `Bytes`, `CurrentMode` tagged fields)
  - source: `Plugins/FX/Niagara/Source/Niagara/Public/NiagaraVariant.h`

#### Tagged Struct Re-Encode for UE Custom Serializers

- If struct field payload carries `typeNodes/flags` metadata, BPX performs generic tagged re-encode on write.
- This allows leaf edits for UE-defined tagged custom-serializer structs (for example `AnimNotifyEvent`, `AnimSyncMarker`, `AnimCurveBase`, `FloatCurve`, `TransformCurve`).

### Current Limitations

- Some struct shortcuts (for example `RichCurveKey`) are not yet implemented; they use generic decode or `rawBase64` fallback.
- User-defined custom-serialized structs are out of scope.
- Updates to enum names not present in NameMap (requiring NameMap insertion/rewrite) are out of current update scope.
- `name remove` is intentionally limited to safe tail-entry removal (arbitrary index removal is not supported).
- Pre-UE5 single-string `FSoftObjectPath` (UE4-era `VER_UE4_ADDED_SOFT_OBJECT_PATH` 이전) is out of current support window.
- `TextProperty` supports history reconstruction and `displayString` generation; `.locres` resolution verifies source-string hash to avoid mismatches.
- Full UE-Editor-equivalent display resolution (complete StringTable redirect/culture fallback parity) is not yet implemented.

## `bpx blueprint disasm`

See [`disasm-script-extraction.md`](./disasm-script-extraction.md) for full extraction-quality requirements.
`--analysis` appends entrypoint slice / CFG / callsite / persistent frame / signature / def-use / confidence metadata.

## `bpx blueprint infer-pack`

`bpx blueprint infer-pack` exports disassembly outputs as an inference bundle.

- Output path: `--out` directory (default: `testdata/reports/bpx_infer_pack_<timestamp>`)
- Main files:
  - `disasm_inference.json`
  - `cfg.json`
  - `callsites.tsv`
  - `persistent_frame.json`
  - `signature.json`
  - `def_use.json`
  - `branches.json`
  - `structured_flow.json`
  - `SUMMARY.md`
  - `RESIDUAL_TASKS.md`

## `bpx class` Output Extension Plan

Current `bpx class` mainly returns `exportReadInfo` (property list).
Planned additions:

```json
{
  "funcMap": [
    { "name": "ReceiveBeginPlay", "exportIndex": 3 },
    { "name": "MyCustomFunction", "exportIndex": 5 }
  ],
  "interfaces": [
    { "class": "/Script/Engine.Interface_PostProcessVolume", "importIndex": -12 }
  ],
  "classFlags": "0x20100080",
  "classFlagsDecoded": ["CLASS_Native", "CLASS_Intrinsic"],
  "classWithin": { "index": 0, "resolved": "Object" },
  "classDefaultObject": { "exportIndex": 2, "objectName": "Default__BP_Example_C" }
}
```

- `funcMap`: function-name to export-index mapping from UField chain or FuncMap
- `interfaces`: implemented interface references from ImportMap
- `classFlags`: raw and decoded class flags
- `classWithin`: outer-class constraint for this class
- `classDefaultObject`: CDO export index and object name

## `--value` JSON Format (v1)

- Scalars: `bool`, `int`, `int64`, `uint*`, `float`, `double`, `string`, `name`, `enum`
- Soft references: `{"packageName":"...", "assetName":"...", "subPath":"..."}`
- Math types: object forms for `Vector/Rotator/Quat/LinearColor/Color/Plane`
- Array/Set/Map: index-targeted updates are primary (full replacement may be added later)
- Struct: `{"FieldA":123, "FieldB":"x"}` updates existing fields only

## Explicitly Forbidden Operations (Current)

- Structural commands under `Unsupported` (Import / Export / `var add/remove` / `enum remove-value` / `localization sync-locres`)
- `bpx blueprint set-bytecode-raw` (blocked due to high corruption risk)

## Current Global Behavior / Flags

- `--dry-run` (write commands only)
- `--backup` (write commands only)
