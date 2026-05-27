# Test Plan and Fixture Specification

> **Design principle**: Following SQLite-style testing discipline, generate golden `before -> after` fixture pairs per UE engine version and verify that `bpx` output is byte-identical to UE Editor output for equivalent operations.
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

You can keep local UE/Lyra paths in a machine-local config file instead of passing them every time:

```bash
# One-time setup (local only; gitignored)
cp scripts/local-fixtures.config.example.json scripts/local-fixtures.config.json

# Then run without path flags
./scripts/gen-fixtures.sh --scope 1,2
```

`gen-fixtures.sh` and `sync-bpx-plugin.sh` automatically read `scripts/local-fixtures.config.json` when present.
Use `--config <path>` to load a different config file explicitly.
When `engines` is defined, both scripts process all configured profiles in one run.
If multiple profiles are configured, they are executed in parallel.

`local-fixtures.config.json` keys:

| Key | Required | Description |
|---|---|---|
| `engines` | ✅ (for multi-engine) | Engine profile map. Each key is an arbitrary version label (for example `5.6.1`, `5.7.3`) |
| `engines.<key>.lyraRoot` | ✅ | Lyra project root (Windows path) for that engine profile |
| `engines.<key>.ueEngineRoot` | recommended | UE engine root (Windows path); used to resolve editor binaries when `editorCmdPath` is omitted |
| `engines.<key>.editorCmdPath` | optional | Explicit Unreal editor executable path (`UnrealEditor-Cmd.exe` or `UnrealEditor.exe`) |
| `engines.<key>.goldenRoot` | optional | Per-engine fixture output root |
| `bpxRepoRoot` | optional | BPX repo root path; defaults to this repository root |
| `scope` / `include` / `skipEditorBuild` | optional | Default values for corresponding CLI options |

Legacy flat keys (`lyraRoot`, `ueEngineRoot`, `editorCmdPath`, `goldenRoot`) are still supported for single-engine setups.
For parallel multi-engine runs, each profile should target a distinct `lyraRoot`.

`gen-fixtures` runs a `Build.bat` prebuild for the editor by default and launches `UnrealEditor-Cmd` with `-Unattended` to avoid blocking dialogs.
Use `--skip-editor-build` only if the editor build is already up to date and you need a faster loop.
`--scope` accepts `1 (read/validate)` or `2 (write/update)`.
By default, generated fixtures are written under `testdata/golden/ue<major>.<minor>/`.
Use `--golden-root` to override the destination root explicitly.

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
│   ├── ue5.6/
│   │   ├── parse/                  # parse-validation fixtures
│   │   ├── operations/             # operation-equivalence before/after pairs
│   │   └── expected_output/        # expected CLI output fixtures
│   ├── ue5.7/
│   │   ├── parse/
│   │   ├── operations/
│   │   └── expected_output/
├── regression/                     # bug-fix regression fixtures
├── fuzz/
│   ├── corpus/                     # initial fuzz corpus
│   └── crashers/                   # crash-inducing samples (kept after fixes)
└── synthetic/                      # generated malformed/boundary fixtures
```

---

## 3. Golden Fixtures for Parse Validation (`golden/<engine>/parse/`)

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


### 3.4 WidgetBlueprint Fixtures

WidgetBlueprint coverage is tracked separately because it is expected to grow into a major write surface for BPX.

Current fixture set already includes:

| File | UE creation steps | Verification focus |
|---|---|---|
| `WBP_Minimum.uasset` | Empty WidgetBlueprint with default root setup | minimal WidgetBlueprint / WidgetTree shape |
| `WBP_TextBlock.uasset` | Root `TextBlock` only | root widget discovery, text property handling |
| `WBP_CanvasPanel.uasset` | Root `CanvasPanel` only | panel/root hierarchy |
| `WBP_CanvasPanel_TextBlock.uasset` | Root `CanvasPanel` with child `TextBlock` | parent/slot/child ordering |
| `WBP_Overlay.uasset` | Root `Overlay` only | alternative panel hierarchy |
| `WBP_Overlay_TextBlock.uasset` | Root `Overlay` with child `TextBlock` | nested WidgetTree traversal |

Planned near-term additions for image/brush work:

| File | UE creation steps | Verification focus |
|---|---|---|
| `WBP_Image.uasset` | Root `Image` widget with an assigned brush/image asset | root `Image` widget parse, `Brush` payload visibility, image reference decode |
| `WBP_CanvasPanel_Image.uasset` | Root `CanvasPanel` with child `Image` | child image widget discovery, slot + brush decode |
| `WBP_Button_Image.uasset` | `Button` using image/brush styling | button-state brush structure and reference shape |
| `T_UI_Icon.uasset` or equivalent texture fixture | Imported texture asset referenced by WidgetBlueprint fixtures | texture-side parse/reference target for widget brush wiring |

Notes:

- `WBP_Image*` fixtures are currently missing and should be added before implementing WidgetImage-specific write commands.
- For image-oriented WidgetBlueprint work, expected-output fixtures should make the `Brush` / asset reference shape observable via CLI output.
- If direct texture import is pursued, the texture fixture must be UE-generated and documented like other golden assets.

Planned near-term additions for basic widget write work:

| File | UE creation steps | Verification focus |
|---|---|---|
| `WBP_CanvasPanel_ProgressBar.uasset` | Root `CanvasPanel` with child `ProgressBar` | child `ProgressBar` discovery, `Percent` and `FillColorAndOpacity` writes |
| `WBP_CanvasPanel_Slider.uasset` | Root `CanvasPanel` with child `Slider` | child `Slider` discovery, `Value` and `Orientation` writes |
| `WBP_CanvasPanel_Spacer.uasset` | Root `CanvasPanel` with child `Spacer` | child `Spacer` discovery, `Size` write |
| `WBP_CanvasPanel_ScrollBar.uasset` | Root `CanvasPanel` with child `ScrollBar` | child `ScrollBar` discovery, `Thickness` and `Orientation` writes |

Notes:

- These fixtures should be UE-generated before removing the corresponding `widget_write_*` deferrals in `BPXGenerateFixtures`.
- The initial operation-equivalence scope only needs representative properties for each widget family; it does not need every supported `widget-write` flag on day one.

---

## 4. Golden Operation Pairs (`golden/<engine>/operations/`)

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

### 4.4 WidgetBlueprint Operations

Current WidgetBlueprint coverage includes rootless parent-class rewrites, root/container insertion, child widget insertion, conservative leaf removal, and a growing set of widget-write operations across text, image, layout, button, RichTextBlock, and basic scalar widget families.

| Directory | UE operation | BPX command | Key concern |
|---|---|---|---|
| `widget_parent_class_commonactivatablewidget_rootless/` | rewrite rootless WidgetBlueprint parent class to `CommonActivatableWidget` | `bpx blueprint widget-parent-class ... --class /Script/CommonUI.CommonActivatableWidget` | rootless parent-class rewrite and generated-class super/import synchronization |
| `widget_add_userwidget_canvaspanel/` | add one child WidgetBlueprint instance under `CanvasPanel` | `bpx blueprint widget-add ... --type userwidget --class /Game/...` | WidgetBlueprintGeneratedClass import wiring |
| `widget_write_text_root_textblock/` | update root `TextBlock` text | `bpx blueprint widget-write ... --property text` | cross-tree text rewrite |
| `widget_write_text_canvaspanel_child/` | update child `TextBlock` text | `bpx blueprint widget-write ... --property text` | child widget targeting |
| `widget_write_text_overlay_child/` | update child `TextBlock` text under `Overlay` | `bpx blueprint widget-write ... --property text` | alternate panel hierarchy |
| `widget_write_opacity_root_textblock/` | update root `RenderOpacity` | `bpx blueprint widget-write ... --property render-opacity` | scalar widget property write |
| `widget_write_opacity_canvaspanel_child/` | update child `RenderOpacity` | `bpx blueprint widget-write ... --property render-opacity` | child scalar property write |
| `widget_write_visibility_root_textblock/` | update root `Visibility` | `bpx blueprint widget-write ... --property visibility` | enum property write |
| `widget_write_visibility_overlay_child/` | update child `Visibility` | `bpx blueprint widget-write ... --property visibility` | enum property write |
| `widget_write_progressbar_percent/` | update `ProgressBar.Percent` | `bpx blueprint widget-write ... --property progressbar-percent` | basic scalar widget write |
| `widget_write_progressbar_fill_color/` | update `ProgressBar.FillColorAndOpacity` | `bpx blueprint widget-write ... --property progressbar-fill-color` | widget color write |
| `widget_write_slider_value/` | update `Slider.Value` | `bpx blueprint widget-write ... --property slider-value` | basic scalar widget write |
| `widget_write_slider_orientation/` | update `Slider.Orientation` | `bpx blueprint widget-write ... --property slider-orientation` | enum-backed widget write |
| `widget_write_spacer_size/` | update `Spacer.Size` | `bpx blueprint widget-write ... --property spacer-size` | vector-style widget write |
| `widget_write_scrollbar_thickness/` | update `ScrollBar.Thickness` | `bpx blueprint widget-write ... --property scrollbar-thickness` | widget struct leaf write |
| `widget_write_scrollbar_orientation/` | update `ScrollBar.Orientation` | `bpx blueprint widget-write ... --property scrollbar-orientation` | enum-backed widget write |
| `widget_write_text_color_root_textblock/` | update `TextBlock.ColorAndOpacity` | `bpx blueprint widget-write ... --property text-color` | TextBlock style write |
| `widget_write_text_font_size_root_textblock/` | update `TextBlock.Font.Size` | `bpx blueprint widget-write ... --property text-font-size` | nested style write |
| `widget_write_text_justification_root_textblock/` | update `TextBlock.Justification` | `bpx blueprint widget-write ... --property text-justification` | enum-backed TextBlock style write |
| `widget_write_sizebox_width_canvaspanel/` | update `SizeBox.WidthOverride` | `bpx blueprint widget-write ... --property sizebox-width` | SizeBox override flag + scalar write |
| `widget_write_sizebox_height_canvaspanel/` | update `SizeBox.HeightOverride` | `bpx blueprint widget-write ... --property sizebox-height` | SizeBox override flag + scalar write |
| `widget_write_scrollbox_orientation_canvaspanel/` | update `ScrollBox.Orientation` | `bpx blueprint widget-write ... --property scrollbox-orientation` | enum-backed ScrollBox write |
| `widget_write_scrollbox_scrollbar_visibility_canvaspanel/` | update `ScrollBox.ScrollBarVisibility` | `bpx blueprint widget-write ... --property scrollbox-scrollbar-visibility` | enum-backed ScrollBox write |

Additional image/style/layout-oriented WidgetBlueprint operations:

| Directory | UE operation | BPX command | Key concern |
|---|---|---|---|
| `widget_write_image_root_image/` | assign existing imported texture/brush to root `Image` widget | `bpx blueprint widget-write ... --property brush-image` | `Brush`/asset reference rewrite |
| `widget_write_image_canvaspanel_child/` | assign image to child `Image` widget | `bpx blueprint widget-write ... --property brush-image` | child widget image targeting |
| `widget_write_image_size/` | update brush image size metadata | `bpx blueprint widget-write ... --property brush-image-size` | brush struct leaf update |
| `widget_write_button_brush_normal/` | assign texture to button normal state brush | `bpx blueprint widget-write ... --property button-normal-image` | nested style/brush write |
| `widget_write_button_brush_tint/` | update button state brush tint | `bpx blueprint widget-write ... --property button-normal-tint` | nested style/brush color write |
| `widget_write_button_brush_draw_as/` | update button state brush draw type | `bpx blueprint widget-write ... --property button-normal-draw-as` | nested style/brush enum write |
| `widget_write_button_brush_image_size/` | update button state brush image size | `bpx blueprint widget-write ... --property button-normal-image-size` | nested style/brush vector write |

Notes:

- WidgetBlueprint fixture coverage is intentionally incremental: each new widget family should land with a representative UE-generated operation fixture, not every possible property on day one.
- Direct texture import should be validated as a separate workstream from WidgetBlueprint brush assignment.
- `widget-remove` coverage remains intentionally conservative and should continue to favor leaf-removal fixtures with clean post-removal validation over broader structural rewrite scenarios.

### 4.5 Low-Risk Abstraction Operations

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

All files under each `golden/<engine>/parse/` are round-trip targets.
Collection should stay directory-driven so new fixtures are auto-included.

### 5.2 Validation Logic

```text
for each .uasset in golden/<engine>/parse/:
    asset = Parse(file)
    output = Serialize(asset)
    assert output == file  // byte-identical
```

### 5.3 Failure Diagnostics

- print first differing offset and section (Summary, NameMap, Import, Export, Property)
- show hex windows (for example +/- 32 bytes) for expected vs actual

---

## 6. Field-Accuracy Tests

Compare parsed results against UE-verified expected outputs in `golden/<engine>/expected_output/*.json`.
Fixtures keep canonical `testdata/golden/...` argv/expected paths; the test harness rebases them to each engine root at runtime.

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

- default output of `go run ./scripts/gen_expected_output` is `testdata/reports/generated_expected_output/<engine>/` when `--golden-root` points at `testdata/golden/`
- only use `--allow-golden-overwrite` for intentional writes into `golden/<engine>/expected_output/`
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
| `BP_UE55.uasset` | synthetic minimal package with UE5.5-era summary layout (`FileVersionUE5=1014`) | parser path smoke for in-window legacy version |
| `BP_UE54.uasset` | synthetic minimal package with UE5.4-era summary layout (`FileVersionUE5=1009`) | parser path smoke for in-window legacy version |
| `BP_FutureVersion.uasset` | synthetic minimal package with unsupported `FileVersionUE5=9999` but otherwise aligned summary layout | unknown-version rejection |

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

Initialize corpus primarily from all `.uasset` files in `golden/<engine>/parse/`.

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

1. Open the Lyra sample project in target UE editor (for example UE 5.6 or UE 5.7).
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
      "path": "golden/<engine>/parse/BP_Empty.uasset",
      "description": "Empty Actor Blueprint fixture",
      "ue_version": "5.6",
      "sha256": "<hash>",
      "size_bytes": 0,
      "layers": ["roundtrip", "parse", "field_accuracy"],
      "milestone": "1"
    },
    {
      "path": "golden/<engine>/operations/prop_set_int/before.uasset",
      "description": "prop set int: before state",
      "ue_version": "5.6",
      "sha256": "<hash>",
      "size_bytes": 0,
      "layers": ["operation_equivalence"],
      "milestone": "2",
      "pair": "golden/<engine>/operations/prop_set_int/after.uasset"
    }
  ]
}
```
