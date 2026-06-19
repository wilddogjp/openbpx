---
name: bpx-widget
description: BPX widget construction and inspection skill. Use for widget-init/read/parent-class/add/remove/write workflows.
---

# widget

> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).

## Usage

```bash
bpx blueprint widget-read <file.uasset> [--export <n>]
bpx blueprint widget-init <out.uasset> --template <minimum> [--engine <auto|ue5.6>] [--asset-name <Name>] [--package-path </Game/...>] [--parent-class </Script/Module.ClassName>] [--force] [--dry-run] [--backup]
bpx blueprint widget-parent-class <file.uasset> --class </Script/Module.ClassName> [--export <n>] [--dry-run] [--backup]
bpx blueprint widget-add <file.uasset> --parent <path|name|root> --type <image|textblock|richtextblock|progressbar|slider|spacer|scrollbar|editabletext|editabletextbox|multilineeditabletextbox|spinbox|comboboxstring|checkbox|userwidget|button|border|retainerbox|invalidationbox|menuanchor|namedslot|sizebox|scalebox|backgroundblur|safezone|windowtitlebararea|canvaspanel|overlay|verticalbox|horizontalbox|stackbox|scrollbox|wrapbox|gridpanel|uniformgridpanel|widgetswitcher|listview|tileview|treeview> --name <Widget_N> [--class </Game/...> when --type userwidget] [--export <n>] [--dry-run] [--backup]
bpx blueprint widget-remove <file.uasset> --widget <path|name> [--export <n>] [--dry-run] [--backup]
bpx blueprint widget-write <file.uasset> --widget <path|name> --property <text|visibility|render-opacity|brush-image|progressbar-percent|progressbar-fill-color|slider-value|slider-min-value|slider-max-value|slider-step-size|slider-orientation|spacer-size|scrollbar-thickness|scrollbar-orientation|checkbox-checked-state|checkbox-is-checked|editabletext-hint-text|editabletext-is-read-only|editabletext-is-password|editabletext-minimum-desired-width|editabletext-justification|editabletextbox-hint-text|editabletextbox-is-read-only|editabletextbox-is-password|editabletextbox-minimum-desired-width|editabletextbox-justification|multilineeditabletextbox-hint-text|multilineeditabletextbox-is-read-only|multilineeditabletextbox-justification|spinbox-value|spinbox-min-value|spinbox-max-value|spinbox-delta|comboboxstring-selected-option|comboboxstring-options|is-focusable|button-is-focusable|checkbox-is-focusable|slider-is-focusable|scrollbox-is-focusable|comboboxstring-is-focusable|listview-entry-widget-class|listview-orientation|listview-selection-mode|listview-consume-mouse-wheel|listview-is-focusable|listview-return-focus-to-selection|listview-clear-scroll-velocity-on-selection|listview-scroll-into-view-alignment|listview-wheel-scroll-multiplier|listview-enable-scroll-animation|listview-allow-overscroll|listview-enable-right-click-scrolling|listview-enable-touch-scrolling|listview-is-pointer-scrolling-enabled|listview-is-gamepad-scrolling-enabled|listview-horizontal-entry-spacing|listview-vertical-entry-spacing|listview-scrollbar-padding|tileview-entry-width|tileview-entry-height|tileview-scrollbar-disabled-visibility|tileview-entry-size-includes-entry-spacing|scrollbox-orientation|scrollbox-scrollbar-visibility|scrollbox-consume-mouse-wheel|sizebox-width-override|sizebox-width|sizebox-height-override|sizebox-height|sizebox-min-desired-width|sizebox-min-desired-height|sizebox-max-desired-width|sizebox-max-desired-height|sizebox-min-aspect-ratio|sizebox-max-aspect-ratio|scalebox-stretch|scalebox-stretch-direction|scalebox-user-specified-scale|scalebox-ignore-inherited-scale|wrapbox-wrap-size|wrapbox-explicit-wrap-size|wrapbox-inner-slot-padding|wrapbox-orientation|widgetswitcher-active-widget-index|retainerbox-retain-render|retainerbox-render-on-invalidation|retainerbox-render-on-phase|retainerbox-phase|retainerbox-phase-count|backgroundblur-strength|backgroundblur-apply-alpha-to-blur|safezone-pad-left|safezone-pad-right|safezone-pad-top|safezone-pad-bottom|invalidationbox-can-cache|uniformgridpanel-min-desired-slot-width|uniformgridpanel-min-desired-slot-height|uniformgridpanel-slot-padding|text-color-and-opacity|text-color|text-font|text-font-family|text-typeface|text-font-size|text-justification|text-auto-wrap-text|text-wrap-text-at|text-line-height-percentage|text-shadow-offset|text-shadow-color-and-opacity|text-outline-size|text-outline-color|button-normal-image|button-hovered-image|button-pressed-image|button-disabled-image|button-normal-tint|button-hovered-tint|button-pressed-tint|button-disabled-tint|button-normal-image-size|button-hovered-image-size|button-pressed-image-size|button-disabled-image-size|button-normal-draw-as|button-hovered-draw-as|button-pressed-draw-as|button-disabled-draw-as|menu-anchor-placement|button-background-color|button-color-and-opacity|border-padding|border-brush-color|border-content-color-and-opacity|border-horizontal-alignment|border-vertical-alignment|grid-row-fill|grid-column-fill|richtext-style-set|richtext-decorator-classes|richtext-override-default-style|richtext-default-font|richtext-default-font-family|richtext-default-typeface|richtext-default-font-size|richtext-default-color-and-opacity|richtext-default-shadow-offset|richtext-default-shadow-color-and-opacity|richtext-default-outline-size|richtext-default-outline-color|richtext-auto-wrap-text|richtext-wrap-text-at|richtext-line-height-percentage|richtext-justification|slot-padding|slot-size|slot-horizontal-alignment|slot-vertical-alignment|slot-row|slot-column|slot-row-span|slot-column-span|slot-layer|slot-nudge|layout-position|layout-size|layout-anchors|layout-alignment|layout-data> --value <value> [--export <n>] [--dry-run] [--backup]
```

## Behavior

- `widget-read`: reads WidgetBlueprint / WidgetTree hierarchy as normalized JSON, plus logical widget aggregation and high-level widget/slot summaries.
- `widget-init`: clones a validated empty WidgetBlueprint template into a new output asset and rewrites package/object identity; this is the default starting point for unspecified new-widget requests.
- `widget-parent-class`: rewrites the compiled `/Script/...` parent class on a rootless WidgetBlueprint before widgets are added.
- `widget-add`: creates a root container/content widget or inserts a bare child widget under supported panel/content parents.
- `widget-remove`: removes one non-root leaf widget from the logical WidgetTree and related WidgetBlueprint metadata.
- `widget-write`: updates one logical widget across designer/generated trees.
- Do not repurpose or mutate an existing WidgetBlueprint unless the user explicitly identifies that asset as the edit target.
- Pair multi-step widget edits with `bpx validate <file.uasset>` when you want a structural safety pass after a write chain.

> [!CAUTION]
> This skill is write-heavy. Confirm intent and run `--dry-run` first.

## Supported Widgets

- Rootless start: `widget-init --template minimum`, optionally followed by `widget-parent-class` or `widget-init --parent-class` while the asset still has no root widget.
- Root / parent containers: `CanvasPanel`, `Overlay`, `VerticalBox`, `HorizontalBox`, `StackBox`, `ScrollBox`, `WrapBox`, `GridPanel`, `UniformGridPanel`, `WidgetSwitcher`, `ListView`, `TileView`, `TreeView`, `Button`, `CheckBox`, `Border`, `RetainerBox`, `InvalidationBox`, `MenuAnchor`, `NamedSlot`, `SizeBox`, `ScaleBox`, `BackgroundBlur`, `SafeZone`, and `WindowTitleBarArea`.
- Child widgets: `Image`, `TextBlock`, `RichTextBlock`, `ProgressBar`, `Slider`, `Spacer`, `ScrollBar`, `EditableText`, `EditableTextBox`, `MultiLineEditableTextBox`, `SpinBox`, `ComboBoxString`, `ListView`, `TileView`, `TreeView`, `CheckBox`, and `userwidget`.
- `widget-add --type userwidget` expects a WidgetBlueprint asset path like `/Game/UI/WBP_Status` and instantiates the referenced `WidgetBlueprintGeneratedClass` as a child widget.
- Parent-class rewrites currently accept compiled `/Script/...` classes, including project/plugin module classes such as `/Script/LyraGame.LyraActivatableWidget`.

## Supported Writes

- Universal reads/writes: `text`, `visibility`, `render-opacity`, `brush-image`.
- Basic widgets: `progressbar-percent`, `progressbar-fill-color`, `slider-value`, `slider-min-value`, `slider-max-value`, `slider-step-size`, `slider-orientation`, `spacer-size`, `scrollbar-thickness`, `scrollbar-orientation`, `checkbox-checked-state`, `checkbox-is-checked`, `editabletext-hint-text`, `editabletext-is-read-only`, `editabletext-is-password`, `editabletext-minimum-desired-width`, `editabletext-justification`, `editabletextbox-hint-text`, `editabletextbox-is-read-only`, `editabletextbox-is-password`, `editabletextbox-minimum-desired-width`, `editabletextbox-justification`, `multilineeditabletextbox-hint-text`, `multilineeditabletextbox-is-read-only`, `multilineeditabletextbox-justification`, `spinbox-value`, `spinbox-min-value`, `spinbox-max-value`, `spinbox-delta`, `comboboxstring-selected-option`, `comboboxstring-options`, `is-focusable`, `button-is-focusable`, `checkbox-is-focusable`, `slider-is-focusable`, `scrollbox-is-focusable`, `comboboxstring-is-focusable`, `listview-entry-widget-class`, `listview-orientation`, `listview-selection-mode`, `listview-consume-mouse-wheel`, `listview-is-focusable`, `listview-return-focus-to-selection`, `listview-clear-scroll-velocity-on-selection`, `listview-scroll-into-view-alignment`, `listview-wheel-scroll-multiplier`, `listview-enable-scroll-animation`, `listview-allow-overscroll`, `listview-enable-right-click-scrolling`, `listview-enable-touch-scrolling`, `listview-is-pointer-scrolling-enabled`, `listview-is-gamepad-scrolling-enabled`, `listview-horizontal-entry-spacing`, `listview-vertical-entry-spacing`, `listview-scrollbar-padding`, `tileview-entry-width`, `tileview-entry-height`, `tileview-scrollbar-disabled-visibility`, `tileview-entry-size-includes-entry-spacing`, `scrollbox-orientation`, `scrollbox-scrollbar-visibility`, `scrollbox-consume-mouse-wheel`, `sizebox-width-override`, `sizebox-width`, `sizebox-height-override`, `sizebox-height`, `sizebox-min-desired-width`, `sizebox-min-desired-height`, `sizebox-max-desired-width`, `sizebox-max-desired-height`, `sizebox-min-aspect-ratio`, `sizebox-max-aspect-ratio`, `scalebox-stretch`, `scalebox-stretch-direction`, `scalebox-user-specified-scale`, `scalebox-ignore-inherited-scale`, `wrapbox-wrap-size`, `wrapbox-explicit-wrap-size`, `wrapbox-inner-slot-padding`, `wrapbox-orientation`, `widgetswitcher-active-widget-index`, `retainerbox-retain-render`, `retainerbox-render-on-invalidation`, `retainerbox-render-on-phase`, `retainerbox-phase`, `retainerbox-phase-count`, `backgroundblur-strength`, `backgroundblur-apply-alpha-to-blur`, `safezone-pad-left`, `safezone-pad-right`, `safezone-pad-top`, `safezone-pad-bottom`, `invalidationbox-can-cache`, `uniformgridpanel-min-desired-slot-width`, `uniformgridpanel-min-desired-slot-height`, `uniformgridpanel-slot-padding`.
- `EditableText`: generic `text` plus `editabletext-hint-text`, `editabletext-is-read-only`, `editabletext-is-password`, `editabletext-minimum-desired-width`, `editabletext-justification`.
- `EditableTextBox`: generic `text` plus `editabletextbox-hint-text`, `editabletextbox-is-read-only`, `editabletextbox-is-password`, `editabletextbox-minimum-desired-width`, `editabletextbox-justification`.
- `MultiLineEditableTextBox`: generic `text` plus `multilineeditabletextbox-hint-text`, `multilineeditabletextbox-is-read-only`, `multilineeditabletextbox-justification`.
- `SpinBox`: `spinbox-value`, `spinbox-min-value`, `spinbox-max-value`, `spinbox-delta`.
- `ComboBoxString`: `comboboxstring-selected-option`, `comboboxstring-options`, `comboboxstring-is-focusable`.
- `TextBlock`: `text-color(-and-opacity)`, `text-font`, `text-font-family`, `text-typeface`, `text-font-size`, `text-justification`, `text-auto-wrap-text`, `text-wrap-text-at`, `text-line-height-percentage`, `text-shadow-offset`, `text-shadow-color-and-opacity`, `text-outline-size`, `text-outline-color`.
- `Button`: per-state brush image/tint/size/draw-as writes plus `button-background-color`, `button-color-and-opacity`, and `button-is-focusable`.
- `Border`: `border-padding`, `border-brush-color`, `border-content-color-and-opacity`, `border-horizontal-alignment`, `border-vertical-alignment`.
- `GridPanel`: `grid-row-fill`, `grid-column-fill`.
- `RichTextBlock`: `richtext-style-set`, `richtext-decorator-classes`, `richtext-override-default-style`, `richtext-default-font`, `richtext-default-font-family`, `richtext-default-typeface`, `richtext-default-font-size`, `richtext-default-color-and-opacity`, `richtext-default-shadow-offset`, `richtext-default-shadow-color-and-opacity`, `richtext-default-outline-size`, `richtext-default-outline-color`, `richtext-auto-wrap-text`, `richtext-wrap-text-at`, `richtext-line-height-percentage`, `richtext-justification`.
- Slot/layout writes: `slot-padding`, `slot-size`, `slot-horizontal-alignment`, `slot-vertical-alignment`, `slot-row`, `slot-column`, `slot-row-span`, `slot-column-span`, `slot-layer`, `slot-nudge`, `layout-position`, `layout-size`, `layout-anchors`, `layout-alignment`, `layout-data`.

## Known Gaps

- `widget-move` and `widget-clone` are not implemented.
- `widget-remove` is intentionally narrow: non-root leaf widgets only.
- Broader `RichTextBlock` styling such as strike brushes, transform policy, and material-backed font overrides is not implemented yet.
- CommonUI-specific widget/property writers are not implemented yet.
- `widget-parent-class` is intentionally narrow: rootless WidgetBlueprints only, and compiled `/Script/...` parent classes only.

## Recommended Workflow

- If the user asks to create a widget without explicitly naming an existing `.uasset` to edit, treat it as a new-widget request and start from `bpx blueprint widget-init ... --template minimum`.
- Do not choose an existing WidgetBlueprint as a base asset unless the user explicitly asks to modify that specific widget.
- Start a new widget with `bpx blueprint widget-init ... --template minimum` so BPX clones a validated empty WidgetBlueprint. If the widget should inherit a compiled project/plugin class, set it before adding the root widget with `--parent-class /Script/...` or `widget-parent-class --class /Script/...`.
- Run widget-building commands sequentially on the same asset. Do not parallelize `widget-init`, `widget-parent-class`, repeated `widget-add` / `widget-remove`, or `widget-write` steps.
- Add exactly one root container with `widget-add --parent root`. The common choices are `canvaspanel`, `overlay`, `verticalbox`, `horizontalbox`, `scrollbox`, `sizebox`, `border`, or `button` depending on layout intent.
- Add child widgets under that parent with `widget-add --parent <panel>` and keep names unique. Use `--type userwidget --class /Game/.../WBP_Name` when the user wants to compose with another WidgetBlueprint.
- For nested multi-child layouts, build the tree in exact order and switch to full widget paths as soon as siblings exist. A proven sequence is `root -> CanvasPanel -> Image1 -> Image2 -> Overlay -> Image3`.
- After each nested `widget-add`, re-run `widget-read` and confirm the new logical path exists before continuing. This catches regressions like a missing second child under `CanvasPanel` or `Overlay` early.
- Fill in appearance after structure. Common writes are `layout-data`, `slot-*`, `brush-image`, `text`, `text-color`, `text-font-family`, `text-font-size`, `text-auto-wrap-text`, `progressbar-percent`, `slider-value`, `checkbox-is-checked`, `sizebox-width`, and `scrollbox-orientation`.
- Use `widget-remove` only for non-root leaf cleanup after confirming the exact path with `widget-read`.
- Keep the order stable: later widget commands depend on the export/name/import state produced by earlier ones.
- Re-check the tree with `widget-read`; use `validate` when you want an extra safety pass after a multi-step edit chain.
- Treat `testdata/golden/ue5.6/operations/widget_add_image_nested_overlay` as the ground-truth fixture for nested `CanvasPanel -> Overlay -> Image` behavior. If a fresh asset differs or opens badly in UE, compare against that operation first.
- `--package-path` is a directory like `/Game/UI`, not a full asset path like `/Game/UI/WBP_Login`. When the destination is already under a UE `Content` directory, keep `--package-path` / `--asset-name` aligned with that filesystem location.
- `brush-image` expects a UE package path like `/Game/UI/T_Icon`, not a filesystem path like `C:\Textures\Icon.png`.
- If your shell rewrites `/Game/...` arguments (for example Git Bash/MSYS), disable path conversion for those arguments first.

## Worked Recipe

```bash
export MSYS_NO_PATHCONV=1
bpx blueprint widget-init ./Content/WBP_Status.uasset --template minimum --asset-name WBP_Status --package-path /Game
bpx blueprint widget-add ./Content/WBP_Status.uasset --parent root --type canvaspanel --name Canvas_Root
bpx blueprint widget-add ./Content/WBP_Status.uasset --parent Canvas_Root --type image --name Image_Circle
bpx blueprint widget-add ./Content/WBP_Status.uasset --parent Canvas_Root --type image --name Image_Square
bpx blueprint widget-add ./Content/WBP_Status.uasset --parent Canvas_Root --type textblock --name Text_Label
bpx blueprint widget-write ./Content/WBP_Status.uasset --widget Canvas_Root/Image_Circle --property brush-image --value /Game/Textures/Circle
bpx blueprint widget-write ./Content/WBP_Status.uasset --widget Canvas_Root/Image_Square --property brush-image --value /Game/Textures/Square
bpx blueprint widget-write ./Content/WBP_Status.uasset --widget Canvas_Root/Text_Label --property text --value "Hello World"
bpx blueprint widget-read ./Content/WBP_Status.uasset
bpx validate ./Content/WBP_Status.uasset
```

```bash
export MSYS_NO_PATHCONV=1
bpx blueprint widget-init ./Content/WBP_MenuRoot.uasset --template minimum --asset-name WBP_MenuRoot --package-path /Game/UI --parent-class /Script/LyraGame.LyraActivatableWidget
bpx blueprint widget-add ./Content/WBP_MenuRoot.uasset --parent root --type overlay --name Overlay_1
bpx blueprint widget-add ./Content/WBP_MenuRoot.uasset --parent Overlay_1 --type userwidget --class /Game/UI/WBP_Status --name WBP_Status_1
bpx blueprint widget-write ./Content/WBP_MenuRoot.uasset --widget Overlay_1/WBP_Status_1 --property slot-horizontal-alignment --value fill
bpx blueprint widget-read ./Content/WBP_MenuRoot.uasset
bpx validate ./Content/WBP_MenuRoot.uasset
```

```bash
export MSYS_NO_PATHCONV=1
bpx blueprint widget-init ./Content/WBP_NestedStatus.uasset --template minimum --asset-name WBP_NestedStatus --package-path /Game
bpx blueprint widget-add ./Content/WBP_NestedStatus.uasset --parent root --type canvaspanel --name CanvasPanel_1
bpx blueprint widget-add ./Content/WBP_NestedStatus.uasset --parent CanvasPanel_1 --type image --name Image_1
bpx blueprint widget-add ./Content/WBP_NestedStatus.uasset --parent CanvasPanel_1 --type image --name Image_2
bpx blueprint widget-add ./Content/WBP_NestedStatus.uasset --parent CanvasPanel_1 --type overlay --name Overlay_1
bpx blueprint widget-add ./Content/WBP_NestedStatus.uasset --parent CanvasPanel_1/Overlay_1 --type image --name Image_3
bpx blueprint widget-read ./Content/WBP_NestedStatus.uasset
bpx validate ./Content/WBP_NestedStatus.uasset
```

- The first recipe is the preferred happy-path for "new widget + CanvasPanel + Image x2 + TextBlock x1".
- The second recipe shows the safe parent-class-first flow for project/plugin widget bases plus child `userwidget` composition.
- When the request is simply "make a widget" and no existing target asset is named, use this `widget-init`-first flow instead of editing a pre-existing WidgetBlueprint.
- The third recipe is the regression-sensitive nested path that should yield `CanvasPanel_1`, `CanvasPanel_1/Image_1`, `CanvasPanel_1/Image_2`, `CanvasPanel_1/Overlay_1`, and `CanvasPanel_1/Overlay_1/Image_3` in `widget-read`.
- Do not insert manual `name add` steps in the normal widget workflow. `widget-add` and `widget-write --property brush-image` are expected to manage required names/imports themselves.
- If a freshly built `widget-init` asset still reports missing NameMap/import-add names, treat that as a stale binary/build mismatch and update `bpx` instead of patching NameMap by hand.
- Use full widget paths like `Canvas_Root/Image_Circle` once the tree has more than one child; that avoids ambiguous selectors and keeps later writes aligned with the intended widget.
- In the nested recipe, use the full path `CanvasPanel_1/Overlay_1` for the final add. Do not rely on bare `Overlay_1` once siblings already exist under `CanvasPanel_1`.
- If a nested asset passes BPX `validate` but still behaves differently in UE, diff it against `widget_add_image_nested_overlay` before hand-editing NameMap, imports, or GeneratedClass data.

## Code-Aligned Caveats

- On a normal `widget-init` -> `widget-add` flow, you should not need manual `name add` before inserting widgets.
- If `widget-add` on a fresh widget reports missing NameMap/import-add names, suspect a stale `bpx` binary rather than treating manual NameMap surgery as standard workflow.
- Nested multi-child panels can appear in `WidgetVariableNameToGuidMap` without appearing in `GeneratedVariables` or `PropertyGuids`. `widget_add_image_nested_overlay` is the reference case: `Overlay_1` is real and valid there even though it does not become a generated class field.
- `widget-parent-class` is only safe before a root widget exists. Once the WidgetBlueprint has a root/widget tree, change the structure instead of trying to reparent the class in place.
- `widget-add --type userwidget` expects a WidgetBlueprint asset path like `/Game/UI/WBP_Status`, not a generated-class path like `/Game/UI/WBP_Status_C`.
- `widget-remove` is conservative by design: it supports only non-root leaf widgets and only compacts orphan export/import/name entries when the remaining references validate cleanly.
- BPX structural `validate` is necessary but not sufficient for UE-openability on newly generated nested widgets. If the editor still crashes or refuses to open, compare both `widget-read` output and the binary delta against the UE-authored nested fixture before widening BPX special-cases.

## High-Signal Examples

```bash
bpx blueprint widget-init ./Content/WBP_Login.uasset --template minimum --asset-name WBP_Login --package-path /Game/UI
bpx blueprint widget-add ./Content/WBP_Login.uasset --parent root --type canvaspanel --name Canvas_Root
bpx blueprint widget-add ./Content/WBP_Login.uasset --parent Canvas_Root --type image --name Image_Logo
bpx blueprint widget-write ./Content/WBP_Login.uasset --widget Canvas_Root/Image_Logo --property brush-image --value /Game/UI/T_Logo
```
