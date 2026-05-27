# UE 5.6/5.7 Implementation Notes (Verified Against Engine Source)

Verified against UE source under `<UE_SOURCE_ROOT>`.

## Verified Baseline (moved from README)

- `PACKAGE_FILE_TAG = 0x9E2A83C1` / `PACKAGE_FILE_TAG_SWAPPED = 0xC1832A9E`
  - `Runtime/Core/Public/UObject/ObjectVersion.h`
- `EUnrealEngineObjectUE5Version` range is `1000..1018`
  - `Runtime/Core/Public/UObject/ObjectVersion.h`
- Core serialization order of `FPackageFileSummary` (SavedHash, Name/Import/Export, MetaData, DataResource, etc.)
  - `Runtime/CoreUObject/Private/UObject/PackageFileSummary.cpp`
- Serialization order and condition branches for `FObjectImport` / `FObjectExport`
  - `Runtime/CoreUObject/Private/UObject/ObjectResource.cpp`
- `FObjectExport.ScriptSerializationStart/EndOffset` is serialized only when
  `!UseUnversionedPropertySerialization() && UEVer >= SCRIPT_SERIALIZATION_OFFSET`
  - `Runtime/CoreUObject/Private/UObject/ObjectResource.cpp` (`operator<<(FStructuredArchive::FSlot, FObjectExport&)`)
  - `Runtime/CoreUObject/Private/UObject/SavePackage2.cpp` normalizes by subtracting `SerialOffset`
    (`ScriptSerialization*Offset -= SerialOffset`)
- `UseUnversionedPropertySerialization` is decided from `PKG_UnversionedProperties` at load time
  - `Runtime/CoreUObject/Private/UObject/LinkerLoad.cpp` (`SetUseUnversionedPropertySerialization`)
- Main fields of `FBPVariableDescription`
  - `Runtime/Engine/Classes/Engine/Blueprint.h`
- Main fields of `FEdGraphPinType`
  - `Runtime/Engine/Classes/EdGraph/EdGraphPin.h`

## Corrections vs Older Notes (drift prevention)

- One NameMap entry is not `bIsWide` + bytes; it is serialized as `FString` (sign indicates wide) + two hashes
  - `Runtime/Core/Private/UObject/UnrealNames.cpp` (`operator<<(FNameEntrySerialized&)`)
- In UE 5.6 (`PROPERTY_TAG_COMPLETE_TYPE_NAME` onward), `FPropertyTag` is primarily driven by `TypeName + Flags`, not legacy type-specific field lists
  - `Runtime/CoreUObject/Private/UObject/PropertyTag.cpp`
- Modern `FSoftObjectPath` is `FTopLevelAssetPath + SubPath`, but includes old-version/custom-version compatibility branches
  - `Runtime/CoreUObject/Private/UObject/SoftObjectPath.cpp`
- `LegacyUE3Version=864` is a common saved value, not a universal invariant
- `FString` compatibility logic treats `SaveNum == MIN_int32` as corrupted input
  - `Runtime/Core/Private/Internationalization/TextKey.cpp` (`LoadKeyString`)
- `UDataTable::LoadStructData` / `SaveStructData` uses `FStructuredArchiveArray` with leading `NumRows`
  - `Runtime/Engine/Private/DataTable.cpp`

## Additional Findings from 2026-02-28 Audit

- `FPropertyTag` uses legacy format (`LoadPropertyTagNoFullType`) when `UEVer < PROPERTY_TAG_COMPLETE_TYPE_NAME(1012)`
  - `Runtime/CoreUObject/Private/UObject/PropertyTag.cpp` (`operator<<(FStructuredArchive::FSlot, FPropertyTag&)`)
  - BPX policy: parser targets UE 5.6 `TypeName` node format; non-strict legacy reads should emit explicit misread-prevention warnings.

- `FPropertyTag` flags (`EPropertyTagFlags`) and extension flags (`EPropertyTagExtension`)
  - `HasArrayIndex=0x01`, `HasPropertyGuid=0x02`, `HasPropertyExtensions=0x04`, `HasBinaryOrNativeSerialize=0x08`, `BoolTrue=0x10`, `SkippedSerialize=0x20`
  - `OverridableInformation=0x02`
  - Reference: `Runtime/CoreUObject/Private/UObject/PropertyTag.cpp`

- Class-level vs property-level overridable extensions are asymmetric:
  - class-level (`EClassSerializationControlExtension::OverridableSerializationInformation`) stores only `OverridableOperation (uint8)`
    - `Runtime/CoreUObject/Private/UObject/Class.cpp` (`UStruct::SerializeVersionedTaggedProperties`)
  - property-level (`EPropertyTagExtension::OverridableInformation`) stores
    `OverriddenPropertyOperation (uint8)` + `ExperimentalOverridableLogic (bool)`
    - `Runtime/CoreUObject/Private/UObject/PropertyTag.cpp` (`SerializePropertyExtensions`)

- Conditional fields in `FPackageFileSummary`:
  - `LocalizationID` omitted when `PKG_FilterEditorOnly`
  - `PersistentGUID` omitted when `PKG_FilterEditorOnly`
  - `MetaDataOffset` added at `>= 1014 (PROPERTY_TAG_EXTENSION_AND_OVERRIDABLE_SERIALIZATION)`
  - `CellExport/CellImport` added at `>= 1015 (VERSE_CELLS)`
  - Reference: `Runtime/CoreUObject/Private/UObject/PackageFileSummary.cpp`

- `FSoftObjectPath` still carries legacy compatibility branches in addition to modern `FTopLevelAssetPath + SubPath`
  - `Runtime/CoreUObject/Private/UObject/SoftObjectPath.cpp`
  - BPX policy: parser accepts `FileVersionUE5=1000..1018`; legacy branches outside that window are rejected.

## 2026-03-05 UE 5.7.3 delta notes (vs 5.6.1)

- `EUnrealEngineObjectUE5Version` adds `IMPORT_TYPE_HIERARCHIES` after `OS_SUB_OBJECT_SHADOW_SERIALIZATION`.
  - `Runtime/Core/Public/UObject/ObjectVersion.h` (`enum class EUnrealEngineObjectUE5Version`)
- `FPackageFileSummary` adds:
  - `ImportTypeHierarchiesCount`
  - `ImportTypeHierarchiesOffset`
  - `Runtime/CoreUObject/Public/UObject/PackageFileSummary.h` (`struct FPackageFileSummary`)
- Summary serialization gates these two fields at `FileVersionUE >= IMPORT_TYPE_HIERARCHIES`.
  - `Runtime/CoreUObject/Private/UObject/PackageFileSummary.cpp` (`operator<<(FStructuredArchive::FSlot, FPackageFileSummary&)`)

## 2026-05-22 WidgetBlueprint list view behavior notes

- `UListView` exposes widget-editable properties:
  - `bIsFocusable`
  - `bClearScrollVelocityOnSelection`
  - `bReturnFocusToSelection`
  - `ScrollIntoViewAlignment`
  - Reference: `Runtime/UMG/Public/Components/ListView.h`
- `ScrollIntoViewAlignment` values are defined in Slate as:
  - `IntoView`
  - `TopOrLeft`
  - `CenterAligned`
  - `BottomOrRight`
  - Reference: `Runtime/Slate/Public/Widgets/Views/STableViewBase.h`
- `UListViewBase` exposes scrolling-behavior properties and setters:
  - `AllowOverscroll`
  - `bEnableRightClickScrolling`
  - `bEnableTouchScrolling`
  - `bIsPointerScrollingEnabled`
  - `bIsGamepadScrollingEnabled`
  - `SetAllowOverScroll`
  - `SetIsPointerScrollingEnabled`
  - `SetIsGamepadScrollingEnabled`
  - Reference: `Runtime/UMG/Public/Components/ListViewBase.h`
- `UTileView` adds:
  - `ScrollbarDisabledVisibility`
  - `bEntrySizeIncludesEntrySpacing`
  - Reference: `Runtime/UMG/Public/Components/TileView.h`

## 2026-03-06 unversioned summary fallback alignment

- UE loader marks package summary as unversioned when all version fields are zero and then applies `GPackageFileUEVersion` / `GPackageFileLicenseeUEVersion`.
  - `Runtime/CoreUObject/Private/UObject/PackageFileSummary.cpp` (`operator<<(FStructuredArchive::FSlot, FPackageFileSummary&)`)
- BPX now mirrors this behavior by attempting unversioned parse with latest supported UE5 version first (`1018`), then retrying `1017` only when needed for legacy layout compatibility.
- Retry trigger uses summary alignment (`NameOffset == SummarySize`) to avoid silently selecting a mismatched layout branch.

## 2026-03-01 FText Equivalence Notes

### UE 5.6 facts (source-backed)

- `FText` persistence is handled by `operator<<(FStructuredArchive::FSlot, FText&)`
  - `Runtime/Core/Private/Internationalization/Text.cpp`
  - `HistoryType=None` adds `bHasCultureInvariantString` + `CultureInvariantString`
  - concrete history class is selected by `switch (ETextHistoryType)`
- `ETextHistoryType` spans `Base(0)`..`TextGenerator(12)` plus `None(-1)`; `AsCultureInvariant` is not a history type
  - `Runtime/Core/Private/Internationalization/TextHistory.h`
- history payload formats are defined in each `FTextHistory_*::Serialize`
  - `Runtime/Core/Private/Internationalization/TextHistory.cpp`
  - `Named/Ordered/ArgumentFormat`: `FormatText(FText)` + args array/map
  - `AsNumber/AsPercent/AsCurrency`: `FFormatArgumentValue` + `bHasFormatOptions` + `FNumberFormattingOptions` + `CultureName`
  - `AsDate/AsTime/AsDateTime`: `FDateTime` + style/timezone/culture (`AsDateTime` has `CustomPattern` branch)
  - `Transform`: `SourceText(FText)` + `TransformType`
  - `StringTableEntry`: `TableId(FName)` + `Key(FTextKey.SerializeAsString)`
  - `TextGenerator`: `GeneratorTypeID(FName)` + `GeneratorContents(TArray<uint8>)`
- argument binary formats are defined by `FFormatArgumentValue` / `FFormatArgumentData` stream operators
  - `Runtime/Core/Private/Internationalization/Text.cpp`

### Remaining work for full equivalence

1. Full reconstruction of history payload
- Status: implemented (`pkg/uasset/value_decode.go`)

2. Localized display resolution for `Base` (`Namespace/Key/SourceHash` lookup in `.locres`)
- Status: not implemented
- Basis: `FTextLocalizationManager::GetDisplayString`, `FindDisplayString_Internal`
- Reference: `Runtime/Core/Private/Internationalization/TextLocalizationManager.cpp`

3. `.locres` loading behavior (version differences, `SourceStringHash`, precedence conflicts)
- Status: not implemented
- Basis: `FTextLocalizationResource::LoadFromArchive`, `ShouldReplaceEntry`
- Reference: `Runtime/Core/Private/Internationalization/TextLocalizationResource.cpp`

4. StringTable redirect / load-policy parity
- Status: not implemented
- Basis: `FTextHistory_StringTableEntry::FStringTableReferenceData::*`
- Reference: `Runtime/Core/Private/Internationalization/TextHistory.cpp`

5. Re-evaluation for generated histories (number/date/transform/format)
- Status: not implemented (payload reconstruction only)
- Basis: `BuildLocalizedDisplayString` / `BuildInvariantDisplayString`
- Reference: `Runtime/Core/Private/Internationalization/TextHistory.cpp`

### Notes

- Exact display-string parity with UE Editor requires `.locres` and full culture-resolution chain; raw `.uasset` decode alone is insufficient.
- `TextGenerator` depends on runtime registration (`FText::RegisterTextGenerator`), so generic CLI tooling is limited to payload-level visibility.

### 2026-03-01 localization audit fixes

- `localization resolve` now verifies `SourceStringHash` during `.locres` resolution.
  - `Runtime/Core/Private/Internationalization/TextLocalizationManager.cpp` (`FindDisplayString_Internal`)
  - `Runtime/Core/Public/Internationalization/TextLocalizationResource.h` (`HashString`)
- Added structured decode for `GatherableTextData`, integrated into `localization read/query/resolve`.
  - `Runtime/Core/Public/Internationalization/GatherableTextData.h`
  - `Runtime/Core/Private/Internationalization/GatherableTextData.cpp`
  - `Runtime/Core/Private/Internationalization/InternationalizationMetadata.cpp`

## 2026-03-01 write/prop/var implementation notes

### UE 5.6 alignment points

- Recompute `FObjectExport::SerialOffset/SerialSize` and update ExportMap when export payload is rewritten.
  - `Runtime/CoreUObject/Private/UObject/ObjectResource.cpp`
- Preserve export-relative `ScriptSerializationStart/EndOffset`; adjust `End` by payload size delta.
  - `Runtime/CoreUObject/Private/UObject/SavePackage2.cpp`
- Shift summary section offsets according to export payload deltas (header patcher approach).
  - `Developer/AssetTools/Private/AssetHeaderPatcher.cpp`
- Recompute `FPropertyTag` size on updates and write header back; for `BoolProperty`, update tag flag (`BoolTrue`) rather than value bytes.
  - `Runtime/CoreUObject/Private/UObject/Class.cpp`, `Runtime/CoreUObject/Private/UObject/PropertyTag.cpp`

### Known current limits

- `var list` now extracts `VarName` from raw tagged properties even when `NewVariables` appears as `rawBase64`.
  - `Runtime/CoreUObject/Private/UObject/Class.cpp` (tagged-property terminator `NAME_None`)
- Full decode of `FBPVariableDescription.VarType` (`FEdGraphPinType`) is still pending; fallback path currently returns empty declaration type to avoid misreads.

## 2026-03-07 audit follow-up fixes

- Blueprint variable rename now derives `FriendlyName` using UE-equivalent `FName::NameToDisplayString` rules instead of a BPX-only identifier splitter.
  - `Editor/UnrealEd/Private/Kismet2/BlueprintEditorUtils.cpp` (`FBlueprintEditorUtils::RenameMemberVariable`)
  - `Runtime/Core/Private/UObject/UnrealNames.cpp` (`FName::NameToDisplayString`)
- `BoolProperty` omission on CDO rewrites is now gated by actual `RF_ClassDefaultObject` and Blueprint declaration presence, rather than object/class-name string heuristics.
  - `Runtime/CoreUObject/Public/UObject/ObjectMacros.h` (`RF_ClassDefaultObject`)
  - `Runtime/CoreUObject/Private/UObject/Class.cpp` (`UStruct::SerializeVersionedTaggedProperties`)
- Plain-string `TextProperty` updates now follow editor `FText::FromString` semantics; persistent save strips transient conversion bits, so saved flags end up zero.
  - `Runtime/Core/Private/Internationalization/Text.cpp` (`FText::FromString`, `operator<<(FStructuredArchive::FSlot, FText&)`)

## 2026-04-04 UMG GridPanel fill notes

- `UGridPanel` exposes row/column fill controls and applies them during widget synchronization.
  - `Runtime/UMG/Public/Components/GridPanel.h` (`SetRowFill`, `SetColumnFill`)
  - `Runtime/UMG/Private/Components/GridPanel.cpp` (`SynchronizeProperties`)
- BPX `widget-write` models these as widget-level `grid-row-fill` / `grid-column-fill` full-array replacements on the `GridPanel` export.

## 2026-04-04 UMG Button / Border appearance notes

- `UButton` exposes `BackgroundColor` and `ColorAndOpacity` widget properties and forwards them during synchronization.
  - `Runtime/UMG/Public/Components/Button.h` (`SetBackgroundColor`, `SetColorAndOpacity`)
  - `Runtime/UMG/Private/Components/Button.cpp` (`SynchronizeProperties`)
- `UButton` also stores per-state slate brushes on `WidgetStyle` (`Normal`, `Hovered`, `Pressed`, `Disabled`), which are synchronized through the same widget style path.
  - `Runtime/UMG/Public/Components/Button.h` (`WidgetStyle`)
  - `Runtime/UMG/Private/Components/Button.cpp` (`SynchronizeProperties`)
- `UBorder` exposes `Padding`, `BrushColor`, `ContentColorAndOpacity`, and content alignment widget properties.
  - `Runtime/UMG/Public/Components/Border.h` (`SetPadding`, `SetBrushColor`, `SetContentColorAndOpacity`, `SetHorizontalAlignment`, `SetVerticalAlignment`)
  - `Runtime/UMG/Private/Components/Border.cpp` (`SynchronizeProperties`)
- BPX `widget-write` models these as widget-level `button-background-color`, `button-color-and-opacity`, button state-brush helpers such as `button-normal-image`, `button-normal-tint`, `button-normal-image-size`, and `button-normal-draw-as`, plus `border-padding`, `border-brush-color`, `border-content-color-and-opacity`, and border alignment helpers.

## 2026-04-05 WidgetBlueprint editor search tail compact-slot note

- Fixture-backed behavior: the `PreloadDependencyOffset..BulkDataStartOffset` editor-search tail is not only verbose records; UE-authored WidgetBlueprint fixtures also contain compact mixed-lane slot sequences where successive 4-byte cells can carry the active byte in different positions.
  - Reproduction authority in this repo:
    - `testdata/golden/ue5.6/operations/widget_write_button_brush_image_size/after.uasset`
    - `testdata/golden/ue5.6/operations/widget_write_button_brush_tint/after.uasset`
- In the button brush fixtures, those compact sequences carry names such as `WidgetTree`, `ObjectProperty`, `AllWidgets`, `PropertyGuids`, `MapProperty`, `NameProperty`, `StructProperty`, and in the image-size path an extra `/Script/Engine` slot.
- Practical BPX rule: after NameMap-changing widget rewrites, blueprint search-tail sync must preserve those mixed compact slot layouts rather than assuming every compact reference uses the same byte lane or that verbose-record patching is sufficient.

## 2026-04-05 WidgetBlueprint instanced-tail remap openability note

- Editor-openability-backed behavior: fresh `widget-init` assets with a direct `CanvasPanel` child that later receives `layout-data` can still crash UE even when BPX `validate` passes, if NameMap-changing rewrites patch instanced tail `FName` refs using pre-rewrite absolute export offsets.
  - Real-editor verification on 2026-04-05:
    - `NestedOverlay` and `VerticalBox` opened in Lyra before this fix.
    - `CanvasImage` and `ButtonBrush` crashed UE before this fix, and both opened successfully after the instanced-tail offset fix landed.
- Root cause isolated during repro:
  - `PatchInstancedTailNameRefs` was correctly using the old payload as the source of truth for `{NameIndex, Number>0}` pairs, but it was writing those remapped indices back at the old export `SerialOffset`.
  - After `ensureNameEntriesPresentSorted` grows the NameMap, export payloads move later in the file; writing at the stale absolute offset corrupts unrelated bytes in `WidgetBlueprintGeneratedClass`.
  - The first visible corruption in the crashing assets was `WidgetBlueprintGeneratedClass` tagged-property damage around `WidgetTree` / `PropertyGuids`, which then surfaced in UE as linker/name-range assertions or `UStruct::SerializeVersionedTaggedProperties` seeking past EOF.
- Practical BPX rule:
  - For instanced tail remap after any NameMap insertion/reorder, compute candidate offsets relative to the original export payload, but apply the patch at the rewritten export's current `SerialOffset + relativeOffset`.
  - Do not reuse stale file-absolute positions from the pre-rewrite asset once NameMap growth has shifted export data.

## 2026-04-05 UMG RichTextBlock safe-v1 notes

- `URichTextBlock` exposes text mutation through `SetText` and synchronizes that widget text through the RichTextBlock component path.
  - `Runtime/UMG/Public/Components/RichTextBlock.h` (`SetText`)
  - `Runtime/UMG/Private/Components/RichTextBlock.cpp` (`SetText`, `SynchronizeProperties`)
- BPX safe-v1 treats `RichTextBlock` as a bare text widget for `widget-add` and generic widget-level `text` / `visibility` / `render-opacity` writes, plus a minimal default-style override surface (`bOverrideDefaultStyle`, `DefaultTextStyleOverride` font size / color / shadow / outline, and `Justification`).
- `URichTextBlock` exposes default-style / layout hooks that align with the minimal BPX surface above.
  - `Runtime/UMG/Public/Components/RichTextBlock.h` (`SetDefaultColorAndOpacity`, `SetDefaultFont`, `SetJustification`)
  - `Runtime/UMG/Private/Components/RichTextBlock.cpp` (`SetDefaultColorAndOpacity`, `SetDefaultFont`, `SynchronizeProperties`)
- The remaining default-style fields BPX now writes live inside Slate style structs rather than dedicated `URichTextBlock` setters.
  - `Runtime/SlateCore/Public/Styling/SlateTypes.h` (`FTextBlockStyle`)
  - `Runtime/SlateCore/Public/Fonts/SlateFontInfo.h` (`FSlateFontInfo`, `FFontOutlineSettings`)
- In the checked Lyra authoring output, `DefaultTextStyleOverride.Font.Size` serialized as `FloatProperty`, and `FontObject` resolved through a standard `/Script/Engine:Font` import whose outer package import pointed at `/Game/UI/Foundation/Fonts/NotoSans`.
- In BPX-authored regression assets, `DefaultTextStyleOverride.ShadowOffset` serialized as `StructProperty(Vector2D(/Script/CoreUObject))`, `ShadowColorAndOpacity` as `StructProperty(LinearColor(/Script/CoreUObject))`, and `Font.OutlineSettings` as `StructProperty(FontOutlineSettings(/Script/SlateCore))` with `OutlineSize` / `OutlineColor` child fields.
- In the checked Lyra authoring output on 2026-04-06, assigning `TextStyleSet=/Game/UI/Settings/SettingsDescriptionStyles` added a `/Script/Engine:DataTable` import, but the `TextStyleSet` `ObjectProperty` itself appeared only on the designer-tree `RichTextBlock` export, not on the generated-tree twin export. BPX mirrors that observed save shape for `richtext-style-set`.
- In the checked Lyra authoring output on 2026-04-06, assigning `DecoratorClasses=[/Game/UI/Settings/NewRichTextBlockDecorator]` added a `/Script/Engine:BlueprintGeneratedClass` import whose object name was `NewRichTextBlockDecorator_C`, and the `DecoratorClasses` `ArrayProperty(ObjectProperty)` also appeared only on the designer-tree `RichTextBlock` export.
- UE-generated fixture coverage now exists in this repo for root and child text writes, root visibility / render-opacity writes, `widget-add --type richtextblock` under `CanvasPanel`, and default-style shadow / outline writes.
  - `testdata/golden/ue5.6/parse/WBP_RichTextBlock.uasset`
  - `testdata/golden/ue5.6/parse/WBP_CanvasPanel_RichTextBlock.uasset`
  - `testdata/golden/ue5.6/operations/widget_add_richtextblock_canvaspanel`
  - `testdata/golden/ue5.6/operations/widget_write_text_root_richtextblock`
  - `testdata/golden/ue5.6/operations/widget_write_text_canvaspanel_child_richtextblock`
  - `testdata/golden/ue5.6/operations/widget_write_opacity_root_richtextblock`
  - `testdata/golden/ue5.6/operations/widget_write_visibility_root_richtextblock`
  - `testdata/golden/ue5.6/operations/widget_write_richtext_default_shadow_offset`
  - `testdata/golden/ue5.6/operations/widget_write_richtext_default_shadow_color`
  - `testdata/golden/ue5.6/operations/widget_write_richtext_default_outline_size`
  - `testdata/golden/ue5.6/operations/widget_write_richtext_default_outline_color`
- Rich text transform policy, strike brush writes, and material-backed font overrides remain out of scope.
- The current fixture-backed implementation still treats untouched `RichTextBlock.Text` localization-key bytes and generated-variable raw payload differences as observed binary data to preserve rather than fully re-derived semantics.
- Regression coverage in this repo:
  - `internal/cli/write_widget_name_remap_regression_test.go`
  - The regression reproduces the exact dangerous path: `widget-init -> CanvasPanel -> Image -> ensureNameEntriesPresentSorted(layout-data names)` and asserts that `WidgetBlueprintGeneratedClass` stays parseable.

## 2026-04-05 UMG nested multi-child panel variable persistence note

- Fixture-backed behavior: in the UE-authored `widget_add_image_nested_overlay` operation, nested `Overlay_1` is present in `WidgetVariableNameToGuidMap` and has generated companion exports/slots, but it is intentionally absent from both `WidgetBlueprint.GeneratedVariables` and `WidgetBlueprintGeneratedClass.PropertyGuids`.
  - Reproduction authority in this repo:
    - `testdata/golden/ue5.6/operations/widget_add_image_nested_overlay/after.uasset`
    - `testdata/BPXFixtureGenerator/Source/BPXFixtureGenerator/Private/BPXGenerateFixturesCommandlet.cpp` (`BuildOperationSpecs`, nested overlay mutation case)
- Practical BPX rule: for multi-child panel widgets such as `Overlay`, `CanvasPanel`, `VerticalBox`, `HorizontalBox`, `StackBox`, `ScrollBox`, `WrapBox`, `GridPanel`, `UniformGridPanel`, and `WidgetSwitcher`, do not infer a generated-class variable record just because the widget is bindable or has a generated companion.
- Regression that exposed this:
  - Fresh `widget-init -> CanvasPanel -> Image1 -> Image2 -> Overlay -> Image3` assets could pass BPX structural validation but still crash UE if BPX appended `Overlay_1` into `GeneratedVariables` / `PropertyGuids`.
  - Control check: the UE-authored fixture opens successfully in editor, while the BPX-generated asset only became loadable after matching that omission.
- Implementation guardrails added in BPX:
  - `widgetAddShouldAppendGeneratedClassVariable` now skips generated-class variable persistence for multi-child panels.
  - `ensureWidgetAddGeneratedParentCompanion` replaces the matching generated `AllWidgets` placeholder instead of appending a second entry, preventing extra null holes in fresh nested assets.
- Source-audit status:
  - The exact UMG editor/compiler callsite responsible for omitting these generated-class records has not yet been narrowed to a single engine function path.
  - Treat the current rule as UE-fixture-backed and editor-openability-backed until a direct source citation is added.

## 2026-03-02 level var-list / var-set notes (placed-object variables)

- `.umap` placed-object resolution uses `FObjectExport::OuterIndex`, targeting exports under `PersistentLevel`.
  - `Runtime/CoreUObject/Private/UObject/SavePackage2.cpp`
- `ULevel` actor references are serialized in `ULevel::Serialize`; objects under `PersistentLevel` carry level context.
  - `Runtime/Engine/Private/Level.cpp` (`ULevel::Serialize`)
- Variable read/write reuses tagged-property paths aligned with `UStruct::SerializeVersionedTaggedProperties`.
  - `Runtime/CoreUObject/Private/UObject/Class.cpp`

## 2026-03-02 LevelViewportInfo support notes

- `FLevelViewportInfo` is a UE-defined struct with custom serialization via `operator<<`.
  - `Runtime/Engine/Classes/Engine/World.h` (`struct FLevelViewportInfo`, `operator<<`)
- `UWorld::Serialize` reads/writes it through `EditorViews` (`TArray<FLevelViewportInfo>`).
  - `Runtime/Engine/Private/World.cpp` (`UWorld::Serialize`)
- BPX now decodes `StructProperty(LevelViewportInfo)` via explicit UE-defined custom-serializer handling.

## 2026-03-02 UE BlueprintType custom serializer expansion

### Extraction criteria

- UE-defined `USTRUCT(BlueprintType)` with `TStructOpsTypeTraits` custom serializer flags (`WithSerializer=true` or `WithStructuredSerializer=true`).
- Source basis:
  - `Runtime/CoreUObject/Private/UObject/Property.cpp`
  - `Runtime/Engine/Public/PerQualityLevelProperties.h`
  - `Runtime/Engine/Classes/GameFramework/OnlineReplStructs.h`
  - `Runtime/Engine/Public/Animation/AnimTypes.h`
  - `Runtime/Engine/Public/Animation/AnimCurveTypes.h`
  - `Runtime/Engine/Classes/Animation/AttributeCurve.h`

### Added read/write coverage

- `FPerQualityLevelInt`, `FPerQualityLevelFloat`
  - `Runtime/Engine/Private/PerQualityLevelProperties.cpp` (`FPerQualityLevelProperty::StreamArchive`)
- `FUniqueNetIdRepl`
  - `Runtime/Engine/Private/OnlineReplStructs.cpp` (`operator<<(FArchive&, FUniqueNetIdRepl&)`)
- `FFrameNumber`
  - `Runtime/CoreUObject/Private/UObject/Property.cpp` (`TStructOpsTypeTraits<FFrameNumber>`)
- Generic tagged re-encode path for custom structs
  - Keeps `typeNodes/flags` on struct field wrappers and reconstructs `FPropertyTag` on write
  - Enables leaf edits in tagged custom structs such as `FAnimNotifyEvent`, `FAnimSyncMarker`, `FAnimCurveBase`, `FFloatCurve`, `FTransformCurve`

## 2026-04-02 WidgetBlueprint add-child notes

- `UWidgetTree::ConstructWidget` is the editor/runtime path used to instantiate a widget object under the owning `WidgetTree`.
  - `Runtime/UMG/Public/Blueprint/WidgetTree.h` (`UWidgetTree::ConstructWidget`)
- `UPanelWidget::AddChild` is the common parent-child attach path for panel widgets.
  - `Runtime/UMG/Private/Components/PanelWidget.cpp` (`UPanelWidget::AddChild`)
- `UCanvasPanel::AddChildToCanvas` creates and returns a `UCanvasPanelSlot` for the inserted child.
  - `Runtime/UMG/Public/Components/CanvasPanel.h` (`UCanvasPanel::AddChildToCanvas`)
  - `Runtime/UMG/Private/Components/CanvasPanel.cpp` (`UCanvasPanel::AddChildToCanvas`)
- `UOverlay::AddChildToOverlay` creates and returns a `UOverlaySlot` for the inserted child.
  - `Runtime/UMG/Public/Components/Overlay.h` (`UOverlay::AddChildToOverlay`)
  - `Runtime/UMG/Private/Components/Overlay.cpp` (`UOverlay::AddChildToOverlay`)

### Explicit boundaries

- User-defined custom-serialized structs are out of scope.
- Updates requiring NameMap insertion/rewrite for missing enum names are out of current write scope.

## 2026-03-02 NameMap read/write notes (`add` / `set` / `remove`)

- `FNameEntrySerialized` persistence format is `FString` + `NonCasePreservingHash(uint16)` + `CasePreservingHash(uint16)`.
  - `Runtime/Core/Private/UObject/UnrealNames.cpp` (`operator<<(FArchive&, FNameEntrySerialized&)`)
- UE load path may treat stored hashes as dummy-read values and rely on runtime hash computation.
  - Same source as above
- Hash computations use `FCrc::StrCrc32` (case-preserving) and `FCrc::Strihash_DEPRECATED` (non-case)
  - `Runtime/Core/Public/Misc/Crc.h`, `Runtime/Core/Private/Misc/Crc.cpp`
- `NamesReferencedFromExportDataCount` reserve rules constrain removable NameMap indices.
  - `Runtime/CoreUObject/Private/UObject/SavePackage2.cpp`
  - `Runtime/CoreUObject/Private/UObject/LinkerSave.cpp` (`operator<<(FName&)`)
- Summary fields typically affected by NameMap operations:
  - `NameCount`, `NameOffset`, `Generations.Last().NameCount`, `NamesReferencedFromExportDataCount` (clamp if needed)
  - `Runtime/CoreUObject/Private/UObject/PackageFileSummary.cpp`

## 2026-03-02 Niagara `TMap<FNiagaraVariableBase, FNiagaraVariant>` decode notes

- `FNiagaraVariableBase` is not fully tagged-struct serialized; it custom-serializes `Name(FName)` followed by `FNiagaraTypeDefinitionHandle`.
  - `Plugins/FX/Niagara/Source/Niagara/Private/NiagaraModule.cpp`
    - `FNiagaraVariableBase::Serialize`
    - `operator<<(FArchive&, FNiagaraTypeDefinitionHandle&)`
- `FNiagaraTypeDefinition` itself serializes tagged properties for `ClassStructOrEnum / UnderlyingType / Flags`.
  - `Plugins/FX/Niagara/Source/Niagara/Private/NiagaraModule.cpp` (`FNiagaraTypeDefinition::Serialize`)
- `FNiagaraVariant` can be reconstructed from tagged fields `Object / DataInterface / Bytes / CurrentMode`.
  - `Plugins/FX/Niagara/Source/Niagara/Public/NiagaraVariant.h`

## 2026-03-04 Material CLI notes (`material read`)

- Material instance parent and override arrays are serialized on `UMaterialInstance`.
  - `Runtime/Engine/Public/Materials/MaterialInstance.h`
    - `Parent`
    - `ScalarParameterValues`, `VectorParameterValues`, `DoubleVectorParameterValues`
    - `TextureParameterValues`, `TextureCollectionParameterValues`
    - `RuntimeVirtualTextureParameterValues`, `SparseVolumeTextureParameterValues`
    - `FontParameterValues`

- Parameter identity semantics come from `FMaterialParameterInfo` (`Name`, `Association`, `Index`).
  - `Runtime/Engine/Public/MaterialTypes.h`
    - `struct FMaterialParameterInfo`
    - `enum EMaterialParameterAssociation`

- Custom-node source snippets are serialized as `UMaterialExpressionCustom::Code` plus related fields.
  - `Runtime/Engine/Public/Materials/MaterialExpressionCustom.h`
    - `Code`
    - `Inputs`, `AdditionalOutputs`, `AdditionalDefines`, `IncludeFilePaths`

- Full translated material HLSL is generated by UE material compilation, not stored as one complete source blob in `.uasset`.
  - `Runtime/Engine/Private/Materials/HLSLMaterialTranslator.cpp`
    - `FHLSLMaterialTranslator::Translate`
    - `FHLSLMaterialTranslator::CustomExpression`
  - `Runtime/Engine/Private/Materials/Material.cpp`
    - `UMaterial::CompilePropertyEx`

## 2026-05-22 WidgetBlueprint ListView entry-class notes

- `UListViewBase` requires `EntryWidgetClass`; missing classes are treated as compile errors for `ListView` / `TileView` / `TreeView`.
  - `Runtime/UMG/Private/Components/ListViewBase.cpp`
    - `UListViewBase::ValidateCompiledDefaults`
    - `UListViewBase::ValidateCompiledWidgetTree`
- The editor detail customization filters the picker against the `EntryWidgetClass` property on `UListViewBase`.
  - `Editor/UMGEditor/Private/Customizations/ListViewBaseDetails.cpp`
    - `FListViewBaseDetails::CustomizeDetails`
    - `AddEntryClassPicker`
- Runtime storage lives on `UListViewBase::EntryWidgetClass` as `TSubclassOf<UUserWidget>`.
  - `Runtime/UMG/Public/Components/ListViewBase.h`
    - `EntryWidgetClass`
    - `GetEntryWidgetClass`

## 2026-05-22 WidgetBlueprint focusable-property notes

- `Button`, `CheckBox`, `Slider`, `ComboBoxString`, and `ScrollBox` expose widget-level focusability booleans in their UMG component headers.
  - `Runtime/UMG/Public/Components/Button.h`
    - `IsFocusable`
    - `GetIsFocusable`
  - `Runtime/UMG/Public/Components/CheckBox.h`
    - `IsFocusable`
    - `GetIsFocusable`
  - `Runtime/UMG/Public/Components/Slider.h`
    - `IsFocusable`
  - `Runtime/UMG/Public/Components/ComboBoxString.h`
    - `bIsFocusable`
    - `IsFocusable`
    - `InitIsFocusable`
  - `Runtime/UMG/Public/Components/ScrollBox.h`
    - `bIsFocusable`
    - `GetIsFocusable`
    - `SetIsFocusable`

- `ListView` / `TileView` / `TreeView` persist focusability through `UListView::bIsFocusable`, which feeds `UListViewBase::FArguments::bAllowFocus`.
  - `Runtime/UMG/Public/Components/ListView.h`
    - `bIsFocusable`
    - `RebuildListWidget`
  - `Runtime/UMG/Public/Components/TileView.h`
    - `RebuildListWidget`
  - `Runtime/UMG/Public/Components/ListViewBase.h`
    - `FArguments::bAllowFocus`

- `EditableText`, `EditableTextBox`, `MultiLineEditableTextBox`, and `SpinBox` do not expose a comparable widget-level `IsFocusable` UPROPERTY in the current UMG public component headers, so BPX should not advertise a write for them.
  - `Runtime/UMG/Public/Components/EditableText.h`
  - `Runtime/UMG/Public/Components/EditableTextBox.h`
  - `Runtime/UMG/Public/Components/MultiLineEditableTextBox.h`
  - `Runtime/UMG/Public/Components/SpinBox.h`
