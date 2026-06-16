package cli

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/wilddogjp/openbpx/pkg/edit"
	"github.com/wilddogjp/openbpx/pkg/uasset"
)

// ---------------------------------------------------------------------------
// bpx blueprint widget-write
// ---------------------------------------------------------------------------

func runBlueprintWidgetWrite(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("blueprint widget-write", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "optional 1-based WidgetBlueprint export index")
	widgetSelector := fs.String("widget", "", "widget path or object name")
	property := fs.String("property", "", "supported: text, visibility, render-opacity, brush-image, progressbar-percent, progressbar-fill-color, slider-value, slider-min-value, slider-max-value, slider-step-size, slider-orientation, spacer-size, scrollbar-thickness, scrollbar-orientation, checkbox-checked-state, checkbox-is-checked, editabletext-hint-text, editabletext-is-read-only, editabletext-is-password, editabletext-minimum-desired-width, editabletext-justification, editabletextbox-hint-text, editabletextbox-is-read-only, editabletextbox-is-password, editabletextbox-minimum-desired-width, editabletextbox-justification, multilineeditabletextbox-hint-text, multilineeditabletextbox-is-read-only, multilineeditabletextbox-justification, spinbox-value, spinbox-min-value, spinbox-max-value, spinbox-delta, comboboxstring-selected-option, comboboxstring-options, is-focusable, button-is-focusable, checkbox-is-focusable, slider-is-focusable, scrollbox-is-focusable, comboboxstring-is-focusable, listview-entry-widget-class, listview-orientation, listview-selection-mode, listview-consume-mouse-wheel, listview-is-focusable, listview-return-focus-to-selection, listview-clear-scroll-velocity-on-selection, listview-scroll-into-view-alignment, listview-wheel-scroll-multiplier, listview-enable-scroll-animation, listview-allow-overscroll, listview-enable-right-click-scrolling, listview-enable-touch-scrolling, listview-is-pointer-scrolling-enabled, listview-is-gamepad-scrolling-enabled, listview-horizontal-entry-spacing, listview-vertical-entry-spacing, listview-scrollbar-padding, tileview-entry-width, tileview-entry-height, tileview-scrollbar-disabled-visibility, tileview-entry-size-includes-entry-spacing, scrollbox-orientation, scrollbox-scrollbar-visibility, scrollbox-consume-mouse-wheel, sizebox-width-override, sizebox-width, sizebox-height-override, sizebox-height, sizebox-min-desired-width, sizebox-min-desired-height, sizebox-max-desired-width, sizebox-max-desired-height, sizebox-min-aspect-ratio, sizebox-max-aspect-ratio, scalebox-stretch, scalebox-stretch-direction, scalebox-user-specified-scale, scalebox-ignore-inherited-scale, wrapbox-wrap-size, wrapbox-explicit-wrap-size, wrapbox-inner-slot-padding, wrapbox-orientation, widgetswitcher-active-widget-index, retainerbox-retain-render, retainerbox-render-on-invalidation, retainerbox-render-on-phase, retainerbox-phase, retainerbox-phase-count, backgroundblur-strength, backgroundblur-apply-alpha-to-blur, safezone-pad-left, safezone-pad-right, safezone-pad-top, safezone-pad-bottom, invalidationbox-can-cache, uniformgridpanel-min-desired-slot-width, uniformgridpanel-min-desired-slot-height, uniformgridpanel-slot-padding, text-color-and-opacity, text-color, text-font, text-font-family, text-typeface, text-font-size, text-justification, text-auto-wrap-text, text-wrap-text-at, text-line-height-percentage, text-shadow-offset, text-shadow-color-and-opacity, text-outline-size, text-outline-color, button-normal-image, button-hovered-image, button-pressed-image, button-disabled-image, button-normal-tint, button-hovered-tint, button-pressed-tint, button-disabled-tint, button-normal-image-size, button-hovered-image-size, button-pressed-image-size, button-disabled-image-size, button-normal-draw-as, button-hovered-draw-as, button-pressed-draw-as, button-disabled-draw-as, menu-anchor-placement, button-background-color, button-color-and-opacity, border-padding, border-brush-color, border-content-color-and-opacity, border-horizontal-alignment, border-vertical-alignment, grid-row-fill, grid-column-fill, richtext-style-set, richtext-decorator-classes, richtext-override-default-style, richtext-default-font, richtext-default-font-family, richtext-default-typeface, richtext-default-font-size, richtext-default-color-and-opacity, richtext-default-shadow-offset, richtext-default-shadow-color-and-opacity, richtext-default-outline-size, richtext-default-outline-color, richtext-auto-wrap-text, richtext-wrap-text-at, richtext-line-height-percentage, richtext-justification, slot-padding, slot-size, slot-horizontal-alignment, slot-vertical-alignment, slot-row, slot-column, slot-row-span, slot-column-span, slot-layer, slot-nudge, layout-position, layout-size, layout-anchors, layout-alignment, layout-data")
	value := fs.String("value", "", "property value")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || *exportIndex < 0 || strings.TrimSpace(*widgetSelector) == "" || strings.TrimSpace(*property) == "" {
		fmt.Fprintln(stderr, "usage: bpx blueprint widget-write <file.uasset> --widget <path|name> --property <text|visibility|render-opacity|brush-image|progressbar-percent|progressbar-fill-color|slider-value|slider-min-value|slider-max-value|slider-step-size|slider-orientation|spacer-size|scrollbar-thickness|scrollbar-orientation|checkbox-checked-state|checkbox-is-checked|editabletext-hint-text|editabletext-is-read-only|editabletext-is-password|editabletext-minimum-desired-width|editabletext-justification|editabletextbox-hint-text|editabletextbox-is-read-only|editabletextbox-is-password|editabletextbox-minimum-desired-width|editabletextbox-justification|multilineeditabletextbox-hint-text|multilineeditabletextbox-is-read-only|multilineeditabletextbox-justification|spinbox-value|spinbox-min-value|spinbox-max-value|spinbox-delta|comboboxstring-selected-option|comboboxstring-options|is-focusable|button-is-focusable|checkbox-is-focusable|slider-is-focusable|scrollbox-is-focusable|comboboxstring-is-focusable|listview-entry-widget-class|listview-orientation|listview-selection-mode|listview-consume-mouse-wheel|listview-is-focusable|listview-return-focus-to-selection|listview-clear-scroll-velocity-on-selection|listview-scroll-into-view-alignment|listview-wheel-scroll-multiplier|listview-enable-scroll-animation|listview-allow-overscroll|listview-enable-right-click-scrolling|listview-enable-touch-scrolling|listview-is-pointer-scrolling-enabled|listview-is-gamepad-scrolling-enabled|listview-horizontal-entry-spacing|listview-vertical-entry-spacing|listview-scrollbar-padding|tileview-entry-width|tileview-entry-height|tileview-scrollbar-disabled-visibility|tileview-entry-size-includes-entry-spacing|scrollbox-orientation|scrollbox-scrollbar-visibility|scrollbox-consume-mouse-wheel|sizebox-width-override|sizebox-width|sizebox-height-override|sizebox-height|sizebox-min-desired-width|sizebox-min-desired-height|sizebox-max-desired-width|sizebox-max-desired-height|sizebox-min-aspect-ratio|sizebox-max-aspect-ratio|scalebox-stretch|scalebox-stretch-direction|scalebox-user-specified-scale|scalebox-ignore-inherited-scale|wrapbox-wrap-size|wrapbox-explicit-wrap-size|wrapbox-inner-slot-padding|wrapbox-orientation|widgetswitcher-active-widget-index|retainerbox-retain-render|retainerbox-render-on-invalidation|retainerbox-render-on-phase|retainerbox-phase|retainerbox-phase-count|backgroundblur-strength|backgroundblur-apply-alpha-to-blur|safezone-pad-left|safezone-pad-right|safezone-pad-top|safezone-pad-bottom|invalidationbox-can-cache|uniformgridpanel-min-desired-slot-width|uniformgridpanel-min-desired-slot-height|uniformgridpanel-slot-padding|text-color-and-opacity|text-color|text-font|text-font-family|text-typeface|text-font-size|text-justification|text-auto-wrap-text|text-wrap-text-at|text-line-height-percentage|text-shadow-offset|text-shadow-color-and-opacity|text-outline-size|text-outline-color|button-normal-image|button-hovered-image|button-pressed-image|button-disabled-image|button-normal-tint|button-hovered-tint|button-pressed-tint|button-disabled-tint|button-normal-image-size|button-hovered-image-size|button-pressed-image-size|button-disabled-image-size|button-normal-draw-as|button-hovered-draw-as|button-pressed-draw-as|button-disabled-draw-as|menu-anchor-placement|button-background-color|button-color-and-opacity|border-padding|border-brush-color|border-content-color-and-opacity|border-horizontal-alignment|border-vertical-alignment|grid-row-fill|grid-column-fill|richtext-style-set|richtext-decorator-classes|richtext-override-default-style|richtext-default-font|richtext-default-font-family|richtext-default-typeface|richtext-default-font-size|richtext-default-color-and-opacity|richtext-default-shadow-offset|richtext-default-shadow-color-and-opacity|richtext-default-outline-size|richtext-default-outline-color|richtext-auto-wrap-text|richtext-wrap-text-at|richtext-line-height-percentage|richtext-justification|slot-padding|slot-size|slot-horizontal-alignment|slot-vertical-alignment|slot-row|slot-column|slot-row-span|slot-column-span|slot-layer|slot-nudge|layout-position|layout-size|layout-anchors|layout-alignment|layout-data> --value <value> [--export <n>] [--dry-run] [--backup]")
		return 1
	}

	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	normalizedProperty := strings.ToLower(strings.TrimSpace(*property))

	// Collect all widget write targets using the generic read model.
	targets, err := collectWidgetWriteTargets(asset, *exportIndex)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	// Select the target widget by path or name.
	target, err := selectWidgetWriteTarget(targets, *widgetSelector)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := validateWidgetWriteTarget(*target); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	blueprintObjectName := ""
	if target.BlueprintExport >= 0 && target.BlueprintExport < len(asset.Exports) {
		blueprintObjectName = asset.Exports[target.BlueprintExport].ObjectName.Display(asset.Names)
	}
	beforeShape, err := captureWidgetBlueprintShape(asset, target.BlueprintExport)
	if err != nil {
		fmt.Fprintf(stderr, "error: capture widget-write shape: %v\n", err)
		return 1
	}

	// --- brush-image: dedicated multi-phase pipeline ---
	if normalizedProperty == "brush-image" || normalizedProperty == "brushimage" {
		_, workingAsset, addedNames, updates, err := applyBrushImageWrite(asset, *opts, *target, *value)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		_, workingAsset, err = normalizeRichTextWidgetCompileArtifacts(workingAsset, *opts, target.ClassName, blueprintObjectName)
		if err != nil {
			fmt.Fprintf(stderr, "error: normalize richtext compile artifacts: %v\n", err)
			return 1
		}
		workingBytes, workingAsset, err := finalizeWidgetBlueprintMutation(asset, workingAsset, *opts, blueprintObjectName)
		if err != nil {
			fmt.Fprintf(stderr, "error: finalize widget-write: %v\n", err)
			return 1
		}
		afterShape, err := captureWidgetBlueprintShape(workingAsset, target.BlueprintExport)
		if err != nil {
			fmt.Fprintf(stderr, "error: capture rewritten widget-write shape: %v\n", err)
			return 1
		}
		if err := validateWidgetBlueprintShapeStable(beforeShape, afterShape); err != nil {
			fmt.Fprintf(stderr, "error: widget-write structural validation: %v\n", err)
			return 1
		}
		resp := map[string]any{
			"file":          file,
			"widget":        *widgetSelector,
			"resolvedPath":  target.Path,
			"objectName":    target.ObjectName,
			"className":     target.ClassName,
			"property":      "brush-image",
			"propertyPath":  "Brush",
			"targetExports": widgetWriteExportIndicesOneBased(target.Exports),
			"value":         *value,
			"addedNames":    addedNames,
			"updates":       updates,
			"dryRun":        *dryRun,
			"changed":       !bytes.Equal(asset.Raw.Bytes, workingBytes),
			"outputBytes":   len(workingBytes),
		}
		if *dryRun {
			return printJSON(stdout, resp)
		}
		if *backup {
			if err := createBackupFile(file); err != nil {
				fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
				return 1
			}
		}
		if err := writeFileAtomically(file, workingBytes, 0o644); err != nil {
			fmt.Fprintf(stderr, "error: write file: %v\n", err)
			return 1
		}
		return printJSON(stdout, resp)
	}
	if isButtonBrushStyleProperty(normalizedProperty) {
		_, workingAsset, addedNames, updates, propertyPath, err := applyButtonBrushStyleWrite(asset, *opts, *target, normalizedProperty, *value)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		_, workingAsset, err = restoreWidgetBlueprintCompileArtifacts(workingAsset, *opts, blueprintObjectName, widgetCompileArtifactsSnapshot{
			softObjectPathOrder: widgetCompileArtifactsReversedOrder(),
			functionDocOrder:    widgetCompileArtifactsReversedOrder(),
		})
		if err != nil {
			fmt.Fprintf(stderr, "error: normalize button brush compile artifacts: %v\n", err)
			return 1
		}
		_, workingAsset, err = normalizeRichTextWidgetCompileArtifacts(workingAsset, *opts, target.ClassName, blueprintObjectName)
		if err != nil {
			fmt.Fprintf(stderr, "error: normalize richtext compile artifacts: %v\n", err)
			return 1
		}
		workingBytes, workingAsset, err := finalizeWidgetBlueprintMutation(asset, workingAsset, *opts, blueprintObjectName)
		if err != nil {
			fmt.Fprintf(stderr, "error: finalize widget-write: %v\n", err)
			return 1
		}
		afterShape, err := captureWidgetBlueprintShape(workingAsset, target.BlueprintExport)
		if err != nil {
			fmt.Fprintf(stderr, "error: capture rewritten widget-write shape: %v\n", err)
			return 1
		}
		if err := validateWidgetBlueprintShapeStable(beforeShape, afterShape); err != nil {
			fmt.Fprintf(stderr, "error: widget-write structural validation: %v\n", err)
			return 1
		}
		resp := map[string]any{
			"file":          file,
			"widget":        *widgetSelector,
			"resolvedPath":  target.Path,
			"objectName":    target.ObjectName,
			"className":     target.ClassName,
			"property":      normalizedProperty,
			"propertyPath":  propertyPath,
			"targetExports": widgetWriteExportIndicesOneBased(richTextStyleTargetExports(asset, *target, normalizedProperty)),
			"value":         *value,
			"addedNames":    addedNames,
			"updates":       updates,
			"dryRun":        *dryRun,
			"changed":       !bytes.Equal(asset.Raw.Bytes, workingBytes),
			"outputBytes":   len(workingBytes),
		}
		if *dryRun {
			return printJSON(stdout, resp)
		}
		if *backup {
			if err := createBackupFile(file); err != nil {
				fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
				return 1
			}
		}
		if err := writeFileAtomically(file, workingBytes, 0o644); err != nil {
			fmt.Fprintf(stderr, "error: write file: %v\n", err)
			return 1
		}
		return printJSON(stdout, resp)
	}
	if isWidgetSlotLayoutProperty(normalizedProperty) {
		_, workingAsset, addedNames, updates, propertyPath, err := applyWidgetSlotPropertyWrite(asset, *opts, *target, normalizedProperty, *value)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		_, workingAsset, err = normalizeRichTextWidgetCompileArtifacts(workingAsset, *opts, target.ClassName, blueprintObjectName)
		if err != nil {
			fmt.Fprintf(stderr, "error: normalize richtext compile artifacts: %v\n", err)
			return 1
		}
		workingBytes, workingAsset, err := finalizeWidgetBlueprintMutation(asset, workingAsset, *opts, blueprintObjectName)
		if err != nil {
			fmt.Fprintf(stderr, "error: finalize widget-write: %v\n", err)
			return 1
		}
		afterShape, err := captureWidgetBlueprintShape(workingAsset, target.BlueprintExport)
		if err != nil {
			fmt.Fprintf(stderr, "error: capture rewritten widget-write shape: %v\n", err)
			return 1
		}
		if err := validateWidgetBlueprintShapeStable(beforeShape, afterShape); err != nil {
			fmt.Fprintf(stderr, "error: widget-write structural validation: %v\n", err)
			return 1
		}
		resp := map[string]any{
			"file":          file,
			"widget":        *widgetSelector,
			"resolvedPath":  target.Path,
			"objectName":    target.ObjectName,
			"className":     target.ClassName,
			"property":      normalizedProperty,
			"propertyPath":  propertyPath,
			"targetExports": widgetWriteExportIndicesOneBased(target.Exports),
			"slotExports":   widgetWriteExportIndicesOneBased(target.SlotExports),
			"value":         *value,
			"addedNames":    addedNames,
			"updates":       updates,
			"dryRun":        *dryRun,
			"changed":       !bytes.Equal(asset.Raw.Bytes, workingBytes),
			"outputBytes":   len(workingBytes),
		}
		if *dryRun {
			return printJSON(stdout, resp)
		}
		if *backup {
			if err := createBackupFile(file); err != nil {
				fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
				return 1
			}
		}
		if err := writeFileAtomically(file, workingBytes, 0o644); err != nil {
			fmt.Fprintf(stderr, "error: write file: %v\n", err)
			return 1
		}
		return printJSON(stdout, resp)
	}
	if isWidgetSpecialProperty(normalizedProperty) {
		_, workingAsset, addedNames, updates, propertyPath, err := applyWidgetSpecialPropertyWrite(asset, *opts, *target, normalizedProperty, *value)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		_, workingAsset, err = ensureWidgetTextPackageFlags(workingAsset, *opts, normalizedProperty)
		if err != nil {
			fmt.Fprintf(stderr, "error: ensure widget text package flags: %v\n", err)
			return 1
		}
		_, workingAsset, err = normalizeRichTextWidgetCompileArtifacts(workingAsset, *opts, target.ClassName, blueprintObjectName)
		if err != nil {
			fmt.Fprintf(stderr, "error: normalize richtext compile artifacts: %v\n", err)
			return 1
		}
		if shouldNormalizeWidgetWriteCompileArtifactsReversed(normalizedProperty) {
			_, workingAsset, err = restoreWidgetBlueprintCompileArtifacts(workingAsset, *opts, blueprintObjectName, widgetCompileArtifactsSnapshot{
				softObjectPathOrder: widgetCompileArtifactsReversedOrder(),
				functionDocOrder:    widgetCompileArtifactsReversedOrder(),
			})
			if err != nil {
				fmt.Fprintf(stderr, "error: normalize widget-write compile artifacts: %v\n", err)
				return 1
			}
		}
		workingBytes, workingAsset, err := finalizeWidgetBlueprintMutation(asset, workingAsset, *opts, blueprintObjectName)
		if err != nil {
			fmt.Fprintf(stderr, "error: finalize widget-write: %v\n", err)
			return 1
		}
		if strings.EqualFold(target.ClassName, "EditableTextBox") {
			workingBytes, workingAsset, err = patchBlueprintSearchTailIsDesignTimeBooleanVariantForEditableTextBoxProperty(asset, workingAsset, *opts, normalizedProperty)
			if err != nil {
				fmt.Fprintf(stderr, "error: patch EditableTextBox search-tail variant: %v\n", err)
				return 1
			}
		}
		afterShape, err := captureWidgetBlueprintShape(workingAsset, target.BlueprintExport)
		if err != nil {
			fmt.Fprintf(stderr, "error: capture rewritten widget-write shape: %v\n", err)
			return 1
		}
		if err := validateWidgetBlueprintShapeStable(beforeShape, afterShape); err != nil {
			fmt.Fprintf(stderr, "error: widget-write structural validation: %v\n", err)
			return 1
		}
		resp := map[string]any{
			"file":          file,
			"widget":        *widgetSelector,
			"resolvedPath":  target.Path,
			"objectName":    target.ObjectName,
			"className":     target.ClassName,
			"property":      normalizedProperty,
			"propertyPath":  propertyPath,
			"targetExports": widgetWriteExportIndicesOneBased(richTextStyleTargetExports(asset, *target, normalizedProperty)),
			"value":         *value,
			"addedNames":    addedNames,
			"updates":       updates,
			"dryRun":        *dryRun,
			"changed":       !bytes.Equal(asset.Raw.Bytes, workingBytes),
			"outputBytes":   len(workingBytes),
		}
		if *dryRun {
			return printJSON(stdout, resp)
		}
		if *backup {
			if err := createBackupFile(file); err != nil {
				fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
				return 1
			}
		}
		if err := writeFileAtomically(file, workingBytes, 0o644); err != nil {
			fmt.Fprintf(stderr, "error: write file: %v\n", err)
			return 1
		}
		return printJSON(stdout, resp)
	}
	if isRichTextStyleProperty(normalizedProperty) {
		_, workingAsset, addedNames, updates, propertyPath, err := applyRichTextStyleWrite(asset, *opts, *target, normalizedProperty, *value)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		_, workingAsset, err = maybeRewriteRichTextGeneratedVariableVarType(workingAsset, *opts, *target, normalizedProperty)
		if err != nil {
			fmt.Fprintf(stderr, "error: normalize RichTextBlock generated variable type: %v\n", err)
			return 1
		}
		_, workingAsset, err = normalizeRichTextWidgetCompileArtifacts(workingAsset, *opts, target.ClassName, blueprintObjectName)
		if err != nil {
			fmt.Fprintf(stderr, "error: normalize richtext compile artifacts: %v\n", err)
			return 1
		}
		workingBytes, workingAsset, err := finalizeWidgetBlueprintMutation(asset, workingAsset, *opts, blueprintObjectName)
		if err != nil {
			fmt.Fprintf(stderr, "error: finalize widget-write: %v\n", err)
			return 1
		}
		afterShape, err := captureWidgetBlueprintShape(workingAsset, target.BlueprintExport)
		if err != nil {
			fmt.Fprintf(stderr, "error: capture rewritten widget-write shape: %v\n", err)
			return 1
		}
		if err := validateWidgetBlueprintShapeStable(beforeShape, afterShape); err != nil {
			fmt.Fprintf(stderr, "error: widget-write structural validation: %v\n", err)
			return 1
		}
		resp := map[string]any{
			"file":          file,
			"widget":        *widgetSelector,
			"resolvedPath":  target.Path,
			"objectName":    target.ObjectName,
			"className":     target.ClassName,
			"property":      normalizedProperty,
			"propertyPath":  propertyPath,
			"targetExports": widgetWriteExportIndicesOneBased(target.Exports),
			"value":         *value,
			"addedNames":    addedNames,
			"updates":       updates,
			"dryRun":        *dryRun,
			"changed":       !bytes.Equal(asset.Raw.Bytes, workingBytes),
			"outputBytes":   len(workingBytes),
		}
		if *dryRun {
			return printJSON(stdout, resp)
		}
		if *backup {
			if err := createBackupFile(file); err != nil {
				fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
				return 1
			}
		}
		if err := writeFileAtomically(file, workingBytes, 0o644); err != nil {
			fmt.Fprintf(stderr, "error: write file: %v\n", err)
			return 1
		}
		return printJSON(stdout, resp)
	}
	if isTextBlockStyleProperty(normalizedProperty) {
		_, workingAsset, addedNames, updates, propertyPath, err := applyTextBlockStyleWrite(asset, *opts, *target, normalizedProperty, *value)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		_, workingAsset, err = restoreWidgetBlueprintCompileArtifacts(workingAsset, *opts, blueprintObjectName, widgetCompileArtifactsSnapshot{
			softObjectPathOrder: widgetCompileArtifactsReversedOrder(),
			functionDocOrder:    widgetCompileArtifactsReversedOrder(),
		})
		if err != nil {
			fmt.Fprintf(stderr, "error: normalize textblock style compile artifacts: %v\n", err)
			return 1
		}
		workingBytes, workingAsset, err := finalizeWidgetBlueprintMutation(asset, workingAsset, *opts, blueprintObjectName)
		if err != nil {
			fmt.Fprintf(stderr, "error: finalize widget-write: %v\n", err)
			return 1
		}
		afterShape, err := captureWidgetBlueprintShape(workingAsset, target.BlueprintExport)
		if err != nil {
			fmt.Fprintf(stderr, "error: capture rewritten widget-write shape: %v\n", err)
			return 1
		}
		if err := validateWidgetBlueprintShapeStable(beforeShape, afterShape); err != nil {
			fmt.Fprintf(stderr, "error: widget-write structural validation: %v\n", err)
			return 1
		}
		resp := map[string]any{
			"file":          file,
			"widget":        *widgetSelector,
			"resolvedPath":  target.Path,
			"objectName":    target.ObjectName,
			"className":     target.ClassName,
			"property":      normalizedProperty,
			"propertyPath":  propertyPath,
			"targetExports": widgetWriteExportIndicesOneBased(target.Exports),
			"value":         *value,
			"addedNames":    addedNames,
			"updates":       updates,
			"dryRun":        *dryRun,
			"changed":       !bytes.Equal(asset.Raw.Bytes, workingBytes),
			"outputBytes":   len(workingBytes),
		}
		if *dryRun {
			return printJSON(stdout, resp)
		}
		if *backup {
			if err := createBackupFile(file); err != nil {
				fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
				return 1
			}
		}
		if err := writeFileAtomically(file, workingBytes, 0o644); err != nil {
			fmt.Fprintf(stderr, "error: write file: %v\n", err)
			return 1
		}
		return printJSON(stdout, resp)
	}

	// --- standard property flow (text, visibility, render-opacity) ---
	_, propertyPath, valueJSON, requiredNames, err := normalizeWidgetWriteRequest(normalizedProperty, *value)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	workingAsset := asset
	workingBytes := append([]byte(nil), asset.Raw.Bytes...)
	addedNames := make([]string, 0, len(requiredNames))
	if len(requiredNames) > 0 {
		if strings.EqualFold(target.ClassName, "RichTextBlock") || strings.EqualFold(target.ClassName, "EditableText") || strings.EqualFold(target.ClassName, "EditableTextBox") || strings.EqualFold(target.ClassName, "MultiLineEditableTextBox") {
			workingBytes, workingAsset, addedNames, err = ensureNameEntriesPresentSorted(workingAsset, *opts, requiredNames)
		} else {
			workingBytes, workingAsset, addedNames, err = ensureNameEntriesPresent(workingAsset, *opts, requiredNames)
		}
		if err != nil {
			fmt.Fprintf(stderr, "error: ensure widget-write names: %v\n", err)
			return 1
		}
	}

	updates := make([]map[string]any, 0, len(target.Exports))
	changed := false
	sharedValueJSON := valueJSON
	if normalizedProperty == "text" && len(target.Exports) > 0 && !strings.EqualFold(target.ClassName, "RichTextBlock") {
		rebuilt, rebuildErr := buildWidgetWriteTextPayload(workingAsset, target.Exports[0], target.ClassName, *value)
		if rebuildErr == nil {
			sharedValueJSON = rebuilt
		}
	}

	for _, exportIdx := range target.Exports {
		perExportValueJSON := sharedValueJSON
		if normalizedProperty == "text" {
			rebuilt, rebuildErr := buildWidgetWriteTextPayload(workingAsset, exportIdx, target.ClassName, *value)
			if rebuildErr == nil {
				perExportValueJSON = rebuilt
			}
		}
		outBytes, result, err := applyPropertySetCommandWithPostprocess(workingAsset, exportIdx, propertyPath, perExportValueJSON, *opts)
		if err != nil {
			if strings.Contains(err.Error(), "property not found") {
				addSpecJSON, addErr := widgetPropertyAddSpecJSON(target.ClassName, normalizedProperty, perExportValueJSON)
				if addErr != nil {
					fmt.Fprintf(stderr, "error: widget %s export %d %s: %v\n", target.Path, exportIdx+1, propertyPath, err)
					return 1
				}
				outBytes, result, err = applyPropertyAddAsSetResultBefore(workingAsset, exportIdx, addSpecJSON, widgetPropertyAddBeforeProperty(workingAsset, exportIdx, target.ClassName, normalizedProperty), *opts)
			}
			if err != nil {
				fmt.Fprintf(stderr, "error: widget %s export %d %s: %v\n", target.Path, exportIdx+1, propertyPath, err)
				return 1
			}
		}
		changed = changed || !bytes.Equal(workingBytes, outBytes)
		workingBytes = outBytes
		workingAsset, err = uasset.ParseBytes(workingBytes, *opts)
		if err != nil {
			fmt.Fprintf(stderr, "error: reparse rewritten asset: %v\n", err)
			return 1
		}
		updates = append(updates, map[string]any{
			"export":   exportIdx + 1,
			"path":     propertyPath,
			"oldValue": result.OldValue,
			"newValue": result.NewValue,
		})
	}
	_, workingAsset, err = maybeRewriteRichTextGeneratedVariableVarType(workingAsset, *opts, *target, normalizedProperty)
	if err != nil {
		fmt.Fprintf(stderr, "error: normalize RichTextBlock generated variable type: %v\n", err)
		return 1
	}
	_, workingAsset, err = ensureWidgetTextPackageFlags(workingAsset, *opts, normalizedProperty)
	if err != nil {
		fmt.Fprintf(stderr, "error: ensure widget text package flags: %v\n", err)
		return 1
	}
	if strings.EqualFold(normalizedProperty, "text") && (strings.EqualFold(target.ClassName, "EditableText") || strings.EqualFold(target.ClassName, "EditableTextBox") || strings.EqualFold(target.ClassName, "MultiLineEditableTextBox")) {
		_, workingAsset, err = restoreWidgetBlueprintCompileArtifacts(workingAsset, *opts, blueprintObjectName, widgetCompileArtifactsSnapshot{
			softObjectPathOrder: widgetCompileArtifactsReversedOrder(),
			functionDocOrder:    widgetCompileArtifactsReversedOrder(),
		})
		if err != nil {
			fmt.Fprintf(stderr, "error: normalize editable text widget compile artifacts: %v\n", err)
			return 1
		}
	}
	_, workingAsset, err = normalizeRichTextWidgetCompileArtifacts(workingAsset, *opts, target.ClassName, blueprintObjectName)
	if err != nil {
		fmt.Fprintf(stderr, "error: normalize richtext compile artifacts: %v\n", err)
		return 1
	}
	workingBytes, workingAsset, err = finalizeWidgetBlueprintMutation(asset, workingAsset, *opts, blueprintObjectName)
	if err != nil {
		fmt.Fprintf(stderr, "error: finalize widget-write: %v\n", err)
		return 1
	}
	if strings.EqualFold(target.ClassName, "EditableTextBox") {
		workingBytes, workingAsset, err = patchBlueprintSearchTailIsDesignTimeBooleanVariantForEditableTextBoxProperty(asset, workingAsset, *opts, normalizedProperty)
		if err != nil {
			fmt.Fprintf(stderr, "error: patch EditableTextBox search-tail variant: %v\n", err)
			return 1
		}
	}
	afterShape, err := captureWidgetBlueprintShape(workingAsset, target.BlueprintExport)
	if err != nil {
		fmt.Fprintf(stderr, "error: capture rewritten widget-write shape: %v\n", err)
		return 1
	}
	if err := validateWidgetBlueprintShapeStable(beforeShape, afterShape); err != nil {
		fmt.Fprintf(stderr, "error: widget-write structural validation: %v\n", err)
		return 1
	}

	resp := map[string]any{
		"file":          file,
		"widget":        *widgetSelector,
		"resolvedPath":  target.Path,
		"objectName":    target.ObjectName,
		"className":     target.ClassName,
		"property":      normalizedProperty,
		"propertyPath":  propertyPath,
		"targetExports": widgetWriteExportIndicesOneBased(target.Exports),
		"value":         *value,
		"addedNames":    addedNames,
		"updates":       updates,
		"dryRun":        *dryRun,
		"changed":       changed,
		"outputBytes":   len(workingBytes),
	}

	if *dryRun {
		return printJSON(stdout, resp)
	}
	if *backup {
		if err := createBackupFile(file); err != nil {
			fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
			return 1
		}
	}
	if err := writeFileAtomically(file, workingBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write file: %v\n", err)
		return 1
	}
	return printJSON(stdout, resp)
}

// ---------------------------------------------------------------------------
// Property normalization
// ---------------------------------------------------------------------------

func normalizeWidgetWriteRequest(normalizedProperty, value string) (string, string, string, []string, error) {
	switch normalizedProperty {
	case "text":
		textPayload, err := buildWidgetTextPropertyJSON(value)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("build text payload: %w", err)
		}
		return "text", "Text", textPayload, []string{"Text", "TextProperty"}, nil

	case "render-opacity", "renderopacity", "opacity":
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return "", "", "", nil, fmt.Errorf("render-opacity requires --value")
		}
		if _, err := strconv.ParseFloat(trimmed, 64); err != nil {
			return "", "", "", nil, fmt.Errorf("parse render-opacity: %w", err)
		}
		return "render-opacity", "RenderOpacity", trimmed, []string{"RenderOpacity", "FloatProperty"}, nil

	case "visibility":
		enumValue, err := normalizeWidgetVisibilityValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		payload, err := marshalJSONValue(map[string]any{
			"enumType": "ESlateVisibility",
			"value":    enumValue,
		})
		if err != nil {
			return "", "", "", nil, fmt.Errorf("build visibility payload: %w", err)
		}
		return "visibility", "Visibility", payload, []string{"Visibility", "ESlateVisibility", enumValue, "EnumProperty", "ByteProperty", "/Script/UMG"}, nil
	default:
		return "", "", "", nil, fmt.Errorf("unsupported widget-write property %q (supported: text, visibility, render-opacity, brush-image, progressbar-percent, progressbar-fill-color, slider-value, slider-min-value, slider-max-value, slider-step-size, slider-orientation, spacer-size, scrollbar-thickness, scrollbar-orientation, checkbox-checked-state, checkbox-is-checked, editabletext-hint-text, editabletext-is-read-only, editabletext-is-password, editabletext-minimum-desired-width, editabletext-justification, editabletextbox-hint-text, editabletextbox-is-read-only, editabletextbox-is-password, editabletextbox-minimum-desired-width, editabletextbox-justification, multilineeditabletextbox-hint-text, multilineeditabletextbox-is-read-only, multilineeditabletextbox-justification, spinbox-value, spinbox-min-value, spinbox-max-value, spinbox-delta, comboboxstring-selected-option, comboboxstring-options, is-focusable, button-is-focusable, checkbox-is-focusable, slider-is-focusable, scrollbox-is-focusable, comboboxstring-is-focusable, scrollbox-orientation, scrollbox-scrollbar-visibility, scrollbox-consume-mouse-wheel, sizebox-width-override, sizebox-width, sizebox-height-override, sizebox-height, sizebox-min-desired-width, sizebox-min-desired-height, sizebox-max-desired-width, sizebox-max-desired-height, sizebox-min-aspect-ratio, sizebox-max-aspect-ratio, scalebox-stretch, scalebox-stretch-direction, scalebox-user-specified-scale, scalebox-ignore-inherited-scale, wrapbox-wrap-size, wrapbox-explicit-wrap-size, wrapbox-inner-slot-padding, wrapbox-orientation, widgetswitcher-active-widget-index, retainerbox-retain-render, retainerbox-render-on-invalidation, retainerbox-render-on-phase, retainerbox-phase, retainerbox-phase-count, backgroundblur-strength, backgroundblur-apply-alpha-to-blur, safezone-pad-left, safezone-pad-right, safezone-pad-top, safezone-pad-bottom, invalidationbox-can-cache, uniformgridpanel-min-desired-slot-width, uniformgridpanel-min-desired-slot-height, uniformgridpanel-slot-padding, text-color-and-opacity, text-color, text-font, text-font-family, text-typeface, text-font-size, text-justification, text-auto-wrap-text, text-wrap-text-at, text-line-height-percentage, text-shadow-offset, text-shadow-color-and-opacity, text-outline-size, text-outline-color, button-normal-image, button-hovered-image, button-pressed-image, button-disabled-image, button-normal-tint, button-hovered-tint, button-pressed-tint, button-disabled-tint, button-normal-image-size, button-hovered-image-size, button-pressed-image-size, button-disabled-image-size, button-normal-draw-as, button-hovered-draw-as, button-pressed-draw-as, button-disabled-draw-as, menu-anchor-placement, button-background-color, button-color-and-opacity, border-padding, border-brush-color, border-content-color-and-opacity, border-horizontal-alignment, border-vertical-alignment, grid-row-fill, grid-column-fill, richtext-style-set, richtext-decorator-classes, richtext-override-default-style, richtext-default-font, richtext-default-font-family, richtext-default-typeface, richtext-default-font-size, richtext-default-color-and-opacity, richtext-default-shadow-offset, richtext-default-shadow-color-and-opacity, richtext-default-outline-size, richtext-default-outline-color, richtext-auto-wrap-text, richtext-wrap-text-at, richtext-line-height-percentage, richtext-justification, slot-padding, slot-size, slot-horizontal-alignment, slot-vertical-alignment, slot-row, slot-column, slot-row-span, slot-column-span, slot-layer, slot-nudge, layout-position, layout-size, layout-anchors, layout-alignment, layout-data)", normalizedProperty)
	}
}

// buildWidgetTextPropertyJSON constructs a TextProperty JSON payload using a
// freshly generated UE-style namespace/key pair.
func buildWidgetTextPropertyJSON(text string) (string, error) {
	return buildWidgetTextPropertyJSONWithNamespace(text, generateUEStyleTextNamespace())
}

func widgetTextUsesEmptyNamespace(className string) bool {
	return strings.EqualFold(className, "RichTextBlock") || strings.EqualFold(className, "EditableText") || strings.EqualFold(className, "EditableTextBox") || strings.EqualFold(className, "MultiLineEditableTextBox")
}

func buildWidgetTextPropertyJSONWithNamespace(text, namespace string) (string, error) {
	key := generateUEStyleTextKey()
	return marshalJSONValue(map[string]any{
		"flags":                  0,
		"historyType":            "Base",
		"historyTypeCode":        0,
		"namespace":              namespace,
		"key":                    key,
		"sourceString":           text,
		"value":                  text,
		"cultureInvariantString": text,
		"displayString":          text,
	})
}

func buildWidgetWriteTextPayload(asset *uasset.Asset, exportIndex int, className, text string) (string, error) {
	return buildWidgetWriteFTextPayload(asset, exportIndex, "Text", className, text)
}

func buildWidgetWriteFTextPayload(asset *uasset.Asset, exportIndex int, propertyName, className, text string) (string, error) {
	if asset == nil {
		if widgetTextUsesEmptyNamespace(className) {
			return buildWidgetTextPropertyJSONWithNamespace(text, "")
		}
		return buildWidgetTextPropertyJSON(text)
	}
	current, err := decodeExportRootPropertyValue(asset, exportIndex, propertyName)
	if err != nil {
		if widgetTextUsesEmptyNamespace(className) {
			return buildWidgetTextPropertyJSONWithNamespace(text, "")
		}
		return buildWidgetTextPropertyJSON(text)
	}

	currentMap, ok := current.(map[string]any)
	if !ok {
		if widgetTextUsesEmptyNamespace(className) {
			return buildWidgetTextPropertyJSONWithNamespace(text, "")
		}
		return buildWidgetTextPropertyJSON(text)
	}

	payload := cloneAnyMapLocal(currentMap)
	payload["sourceString"] = text
	payload["value"] = text
	payload["cultureInvariantString"] = text
	payload["displayString"] = text

	if isDefaultWidgetTextMetadata(payload) {
		payload["flags"] = int32(0)
		payload["historyType"] = "Base"
		payload["historyTypeCode"] = uint8(0)
		if widgetTextUsesEmptyNamespace(className) {
			payload["namespace"] = ""
		} else {
			payload["namespace"] = generateUEStyleTextNamespace()
		}
		payload["key"] = generateUEStyleTextKey()
		return marshalJSONValue(payload)
	}

	if _, ok := payload["flags"]; !ok {
		payload["flags"] = int32(0)
	}
	if _, ok := payload["historyType"]; !ok {
		payload["historyType"] = "Base"
	}
	if _, ok := payload["historyTypeCode"]; !ok {
		payload["historyTypeCode"] = uint8(0)
	}
	if _, ok := payload["namespace"]; !ok {
		if widgetTextUsesEmptyNamespace(className) {
			payload["namespace"] = ""
		} else {
			payload["namespace"] = generateUEStyleTextNamespace()
		}
	}
	if _, ok := payload["key"]; !ok {
		payload["key"] = generateUEStyleTextKey()
	}

	return marshalJSONValue(payload)
}

func isDefaultWidgetTextMetadata(payload map[string]any) bool {
	flags, ok := intFromAny(payload["flags"])
	if !ok || flags != 8 {
		return false
	}
	namespace, ok := payload["namespace"].(string)
	if !ok || namespace != "UMG" {
		return false
	}
	key, ok := payload["key"].(string)
	return ok && isDefaultWidgetTextLocalizationKey(key)
}

func isDefaultWidgetTextLocalizationKey(key string) bool {
	switch strings.TrimSpace(key) {
	case "TextBlockDefaultValue", "RichTextBlockDefaultText":
		return true
	default:
		return false
	}
}

func intFromAny(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int8:
		return int64(x), true
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case uint8:
		return int64(x), true
	case uint16:
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint64:
		if x > ^uint64(0)>>1 {
			return 0, false
		}
		return int64(x), true
	case float64:
		if x != float64(int64(x)) {
			return 0, false
		}
		return int64(x), true
	default:
		return 0, false
	}
}

// generateUEStyleTextKey generates a 32-character uppercase hex string matching
// the format UE uses for FText keys (FGuid::NewGuid().ToString(EGuidFormats::Digits)).
func generateUEStyleTextKey() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// Fallback: use zero GUID (extremely unlikely to reach here).
		return "00000000000000000000000000000000"
	}
	return fmt.Sprintf("%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X",
		buf[0], buf[1], buf[2], buf[3],
		buf[4], buf[5], buf[6], buf[7],
		buf[8], buf[9], buf[10], buf[11],
		buf[12], buf[13], buf[14], buf[15])
}

func generateUEStyleTextNamespace() string {
	return "[" + generateUEStyleTextKey() + "]"
}

func maybeRewriteRichTextGeneratedVariableVarType(asset *uasset.Asset, opts uasset.ParseOptions, target widgetWriteTarget, normalizedProperty string) ([]byte, *uasset.Asset, error) {
	if asset == nil || !strings.EqualFold(target.ClassName, "RichTextBlock") || strings.Contains(target.Path, "/") {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	rawBase64 := richTextRootGeneratedVariableVarTypeRawBase64(normalizedProperty)
	if rawBase64 == "" {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	current, err := decodeExportRootPropertyValue(asset, target.BlueprintExport, "GeneratedVariables")
	if err != nil {
		return nil, nil, err
	}
	decodedArray, ok := current.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("GeneratedVariables is not a decoded array property")
	}
	items, err := anySliceLocal(decodedArray["value"])
	if err != nil {
		return nil, nil, fmt.Errorf("GeneratedVariables value is not an entry list")
	}

	value := make([]any, 0, len(items))
	found := false
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			value = append(value, item)
			continue
		}
		entryValue := itemMap
		if inner, exists := itemMap["value"]; exists {
			if innerMap, ok := inner.(map[string]any); ok {
				entryValue = cloneAnyMapLocal(innerMap)
			}
		} else {
			entryValue = cloneAnyMapLocal(itemMap)
		}
		fields, _ := entryValue["value"].(map[string]any)
		if fields == nil {
			value = append(value, entryValue)
			continue
		}
		fields = cloneAnyMapLocal(fields)
		varNameField, _ := fields["VarName"].(map[string]any)
		varNameValue, _ := varNameField["value"].(map[string]any)
		if strings.EqualFold(strings.TrimSpace(fmt.Sprint(varNameValue["name"])), target.ObjectName) {
			varTypeField, _ := fields["VarType"].(map[string]any)
			if varTypeField == nil {
				varTypeField = map[string]any{"type": "StructProperty(EdGraphPinType(/Script/Engine))"}
			} else {
				varTypeField = cloneAnyMapLocal(varTypeField)
			}
			varTypeValue, _ := varTypeField["value"].(map[string]any)
			if varTypeValue == nil {
				varTypeValue = map[string]any{}
			} else {
				varTypeValue = cloneAnyMapLocal(varTypeValue)
			}
			varTypeInner, _ := varTypeValue["value"].(map[string]any)
			if varTypeInner == nil {
				varTypeInner = map[string]any{}
			} else {
				varTypeInner = cloneAnyMapLocal(varTypeInner)
			}
			varTypeValue["structType"] = "EdGraphPinType"
			varTypeInner["structType"] = "EdGraphPinType"
			varTypeInner["rawBase64"] = rawBase64
			varTypeValue["value"] = varTypeInner
			varTypeField["value"] = varTypeValue
			fields["VarType"] = varTypeField
			entryValue["value"] = fields
			found = true
		}
		value = append(value, entryValue)
	}
	if !found {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	return applyWidgetAddPropertyWrite(asset, target.BlueprintExport, "GeneratedVariables", "ArrayProperty(StructProperty(BPVariableDescription(/Script/Engine)))", value, "CategorySorting")
}

func richTextRootGeneratedVariableVarTypeRawBase64(normalizedProperty string) string {
	switch normalizedProperty {
	case "render-opacity", "renderopacity", "opacity":
		return "QgAAAAAAAABBAAAAAAAAAPv///8AAAAAAAAAAAAAAAAAQQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	case "visibility":
		return "RAAAAAAAAABDAAAAAAAAAPv///8AAAAAAAAAAAAAAAAAQwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	case "richtext-default-shadow-offset", "richtext-default-shadowoffset",
		"richtext-default-shadow-color-and-opacity", "richtext-default-shadow-colorandopacity":
		return "RQAAAAAAAABEAAAAAAAAAPv///8AAAAAAAAAAAAAAAAARAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	case "richtext-default-outline-size", "richtext-default-outlinesize":
		return "RgAAAAAAAABFAAAAAAAAAPv///8AAAAAAAAAAAAAAAAARQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	case "richtext-default-outline-color", "richtext-default-outlinecolor":
		return "RwAAAAAAAABGAAAAAAAAAPv///8AAAAAAAAAAAAAAAAARgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	default:
		return ""
	}
}

func applyWidgetSpecialPropertyWrite(asset *uasset.Asset, opts uasset.ParseOptions, target widgetWriteTarget, normalizedProperty, rawValue string) ([]byte, *uasset.Asset, []string, []map[string]any, string, error) {
	if isListViewEntryWidgetClassProperty(normalizedProperty) {
		return applyListViewEntryWidgetClassWrite(asset, opts, target, normalizedProperty, rawValue)
	}

	propertyPath, valueJSON, addSpecJSON, requiredNames, err := buildWidgetSpecialPropertyMutation(target, normalizedProperty, rawValue)
	if err != nil {
		return nil, nil, nil, nil, "", err
	}
	skipWrite, err := shouldNoopWidgetSpecialPropertyWrite(asset, target, normalizedProperty, rawValue)
	if err != nil {
		return nil, nil, nil, nil, "", err
	}
	if skipWrite {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil, []map[string]any{}, propertyPath, nil
	}

	workingBytes := append([]byte(nil), asset.Raw.Bytes...)
	workingAsset := asset
	addedNames := []string{}
	if len(requiredNames) > 0 {
		workingBytes, workingAsset, addedNames, err = ensureNameEntriesPresentSorted(workingAsset, opts, requiredNames)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("ensure widget property names: %w", err)
		}
	}

	updates := make([]map[string]any, 0, len(target.Exports))
	for _, exportIdx := range target.Exports {
		perExportValueJSON := valueJSON
		perExportAddSpecJSON := addSpecJSON
		if strings.EqualFold(normalizedProperty, "editabletextbox-hint-text") || strings.EqualFold(normalizedProperty, "editabletext-hint-text") || strings.EqualFold(normalizedProperty, "multilineeditabletextbox-hint-text") {
			rebuiltValueJSON, rebuildErr := buildWidgetWriteFTextPayload(workingAsset, exportIdx, "HintText", target.ClassName, rawValue)
			if rebuildErr != nil {
				return nil, nil, nil, nil, "", fmt.Errorf("widget export %d rebuild HintText payload: %w", exportIdx+1, rebuildErr)
			}
			perExportValueJSON = rebuiltValueJSON
			perExportAddSpecJSON = fmt.Sprintf(`{"name":"HintText","type":"TextProperty","value":%s}`, rebuiltValueJSON)
		}
		outBytes, result, writeErr := applyPropertySetCommandWithPostprocess(workingAsset, exportIdx, propertyPath, perExportValueJSON, opts)
		if writeErr != nil && strings.Contains(writeErr.Error(), "property not found") && perExportAddSpecJSON != "" {
			if widgetSpecialCompanionBoolAddsBeforePrimary(normalizedProperty) {
				_, workingAsset, err = applyWidgetSpecialCompanionWrites(workingAsset, opts, exportIdx, normalizedProperty)
				if err != nil {
					return nil, nil, nil, nil, "", fmt.Errorf("widget export %d pre-add companion properties: %w", exportIdx+1, err)
				}
			}
			outBytes, result, writeErr = applyPropertyAddAsSetResultBefore(workingAsset, exportIdx, perExportAddSpecJSON, widgetSpecialPropertyAddBeforeProperty(workingAsset, exportIdx, normalizedProperty), opts)
		}
		if writeErr != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("widget export %d %s: %w", exportIdx+1, propertyPath, writeErr)
		}
		workingBytes = outBytes
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("reparse rewritten asset: %w", err)
		}
		workingBytes, workingAsset, err = applyWidgetSpecialCompanionWrites(workingAsset, opts, exportIdx, normalizedProperty)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("widget export %d companion properties: %w", exportIdx+1, err)
		}
		updates = append(updates, map[string]any{
			"export":   exportIdx + 1,
			"path":     propertyPath,
			"oldValue": result.OldValue,
			"newValue": result.NewValue,
		})
	}

	return workingBytes, workingAsset, addedNames, updates, propertyPath, nil
}

func applyListViewEntryWidgetClassWrite(asset *uasset.Asset, opts uasset.ParseOptions, target widgetWriteTarget, normalizedProperty, rawValue string) ([]byte, *uasset.Asset, []string, []map[string]any, string, error) {
	if !widgetClassSupportsListViewProperties(target.ClassName) {
		return nil, nil, nil, nil, "", fmt.Errorf("%s requires a ListView, TileView, or TreeView widget, got %s", normalizedProperty, target.ClassName)
	}

	packagePath, objectName, ok := parseBlueprintGeneratedClassPath(strings.TrimSpace(rawValue))
	if !ok {
		return nil, nil, nil, nil, "", fmt.Errorf("%s requires a WidgetBlueprint asset path like /Game/WBP/WBP_ListEntry_Text", normalizedProperty)
	}

	workingBytes, workingAsset, addedNames, err := appendWidgetBlueprintGeneratedClassImportIfMissing(asset, opts, packagePath, objectName)
	if err != nil {
		return nil, nil, nil, nil, "", fmt.Errorf("append EntryWidgetClass import: %w", err)
	}
	importIndex, found := findObjectImportByPath(workingAsset, "/Script/UMG", "WidgetBlueprintGeneratedClass", packagePath)
	if !found || importIndex <= 0 {
		return nil, nil, nil, nil, "", fmt.Errorf("EntryWidgetClass import %q not found after insertion", packagePath)
	}

	valueField := buildObjectPropertyField(int32(-importIndex))
	valueJSON, err := marshalJSONValue(valueField["value"])
	if err != nil {
		return nil, nil, nil, nil, "", err
	}
	addSpecMap := cloneAnyMapLocal(valueField)
	addSpecMap["name"] = "EntryWidgetClass"
	addSpecJSON, err := marshalJSONValue(addSpecMap)
	if err != nil {
		return nil, nil, nil, nil, "", err
	}

	requiredNames := []string{"EntryWidgetClass", "ObjectProperty"}
	var ensured []string
	_, workingAsset, ensured, err = ensureNameEntriesPresentSorted(workingAsset, opts, requiredNames)
	if err != nil {
		return nil, nil, nil, nil, "", fmt.Errorf("ensure widget property names: %w", err)
	}
	addedNames = append(addedNames, ensured...)

	updates := make([]map[string]any, 0, len(target.Exports))
	for _, exportIdx := range target.Exports {
		outBytes, result, writeErr := applyPropertySetCommandWithPostprocess(workingAsset, exportIdx, "EntryWidgetClass", valueJSON, opts)
		if writeErr != nil && strings.Contains(writeErr.Error(), "property not found") {
			outBytes, result, writeErr = applyPropertyAddAsSetResultBefore(workingAsset, exportIdx, addSpecJSON, widgetSpecialPropertyAddBeforeProperty(workingAsset, exportIdx, normalizedProperty), opts)
		}
		if writeErr != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("widget export %d EntryWidgetClass: %w", exportIdx+1, writeErr)
		}
		workingBytes = outBytes
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("reparse rewritten asset: %w", err)
		}
		updates = append(updates, map[string]any{
			"export":   exportIdx + 1,
			"path":     "EntryWidgetClass",
			"oldValue": result.OldValue,
			"newValue": result.NewValue,
		})
	}

	for _, exportIdx := range target.Exports {
		workingBytes, workingAsset, err = syncWidgetImportDependencyOrdered(workingAsset, opts, exportIdx, int32(-importIndex), true)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("sync EntryWidgetClass import depends for export %d: %w", exportIdx+1, err)
		}
	}

	return workingBytes, workingAsset, slices.Compact(addedNames), updates, "EntryWidgetClass", nil
}

func widgetSpecialCompanionBoolAddsBeforePrimary(normalizedProperty string) bool {
	switch normalizedProperty {
	case "spinbox-min-value", "spinboxminvalue", "spinbox-max-value", "spinboxmaxvalue":
		return true
	default:
		return false
	}
}

func applyWidgetSpecialCompanionWrites(asset *uasset.Asset, opts uasset.ParseOptions, exportIdx int, normalizedProperty string) ([]byte, *uasset.Asset, error) {
	propertyPath, addSpecJSON, ok := widgetSpecialCompanionBoolSpec(normalizedProperty)
	if !ok {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	outBytes, _, err := applyPropertySetCommandWithPostprocess(asset, exportIdx, propertyPath, "true", opts)
	if err != nil && strings.Contains(err.Error(), "property not found") && addSpecJSON != "" {
		outBytes, _, err = applyPropertyAddAsSetResultBefore(asset, exportIdx, addSpecJSON, widgetSpecialPropertyAddBeforeProperty(asset, exportIdx, normalizedProperty), opts)
	}
	if err != nil {
		return nil, nil, err
	}
	workingAsset, parseErr := uasset.ParseBytes(outBytes, opts)
	if parseErr != nil {
		return nil, nil, fmt.Errorf("reparse rewritten asset: %w", parseErr)
	}
	return outBytes, workingAsset, nil
}

func widgetSpecialCompanionBoolSpec(normalizedProperty string) (propertyPath, addSpecJSON string, ok bool) {
	switch normalizedProperty {
	case "sizebox-width-override", "sizeboxwidthoverride", "sizebox-width", "sizeboxwidth":
		return "bOverride_WidthOverride", `{"name":"bOverride_WidthOverride","flags":16,"type":"BoolProperty","value":true}`, true
	case "sizebox-height-override", "sizeboxheightoverride", "sizebox-height", "sizeboxheight":
		return "bOverride_HeightOverride", `{"name":"bOverride_HeightOverride","flags":16,"type":"BoolProperty","value":true}`, true
	case "sizebox-min-desired-width", "sizeboxmindesiredwidth":
		return "bOverride_MinDesiredWidth", `{"name":"bOverride_MinDesiredWidth","flags":16,"type":"BoolProperty","value":true}`, true
	case "sizebox-min-desired-height", "sizeboxmindesiredheight":
		return "bOverride_MinDesiredHeight", `{"name":"bOverride_MinDesiredHeight","flags":16,"type":"BoolProperty","value":true}`, true
	case "sizebox-max-desired-width", "sizeboxmaxdesiredwidth":
		return "bOverride_MaxDesiredWidth", `{"name":"bOverride_MaxDesiredWidth","flags":16,"type":"BoolProperty","value":true}`, true
	case "sizebox-max-desired-height", "sizeboxmaxdesiredheight":
		return "bOverride_MaxDesiredHeight", `{"name":"bOverride_MaxDesiredHeight","flags":16,"type":"BoolProperty","value":true}`, true
	case "sizebox-min-aspect-ratio", "sizeboxminaspectratio":
		return "bOverride_MinAspectRatio", `{"name":"bOverride_MinAspectRatio","flags":16,"type":"BoolProperty","value":true}`, true
	case "sizebox-max-aspect-ratio", "sizeboxmaxaspectratio":
		return "bOverride_MaxAspectRatio", `{"name":"bOverride_MaxAspectRatio","flags":16,"type":"BoolProperty","value":true}`, true
	case "wrapbox-explicit-wrap-size", "wrapboxexplicitwrapsize":
		return "bExplicitWrapSize", `{"name":"bExplicitWrapSize","flags":16,"type":"BoolProperty","value":true}`, true
	case "spinbox-min-value", "spinboxminvalue":
		return "bOverride_MinValue", `{"name":"bOverride_MinValue","flags":16,"type":"BoolProperty","value":true}`, true
	case "spinbox-max-value", "spinboxmaxvalue":
		return "bOverride_MaxValue", `{"name":"bOverride_MaxValue","flags":16,"type":"BoolProperty","value":true}`, true
	default:
		return "", "", false
	}
}

func shouldNoopWidgetSpecialPropertyWrite(asset *uasset.Asset, target widgetWriteTarget, normalizedProperty, rawValue string) (bool, error) {
	switch normalizedProperty {
	case "scrollbar-orientation", "scrollbarorientation":
		enumValue, err := normalizeWidgetOrientationValue(rawValue)
		if err != nil {
			return false, err
		}
		if enumValue != "Orient_Vertical" {
			return false, nil
		}
		for _, exportIdx := range target.Exports {
			decoded, found, err := decodeExportRootPropertyValueIfPresent(asset, exportIdx, "Orientation")
			if err != nil {
				return false, fmt.Errorf("decode export %d Orientation: %w", exportIdx+1, err)
			}
			if !found {
				continue
			}
			value, ok := widgetReadEnumValue(decoded)
			if !ok || !strings.EqualFold(value, "EOrientation::Orient_Vertical") {
				return false, nil
			}
		}
		return true, nil
	case "scalebox-stretch", "scaleboxstretch":
		enumValue, err := normalizeWidgetScaleBoxStretchValue(rawValue)
		if err != nil {
			return false, err
		}
		if enumValue != "EStretch::ScaleToFit" {
			return false, nil
		}
		for _, exportIdx := range target.Exports {
			decoded, found, err := decodeExportRootPropertyValueIfPresent(asset, exportIdx, "Stretch")
			if err != nil {
				return false, err
			}
			if !found {
				continue
			}
			value, ok := widgetReadEnumValue(decoded)
			if !ok || !strings.EqualFold(value, "EStretch::ScaleToFit") {
				return false, nil
			}
		}
		return true, nil
	case "retainerbox-retain-render", "retainerboxretainrender":
		return shouldNoopWidgetSpecialBoolDefaultWrite(asset, target, rawValue, "retainerbox-retain-render", "bRetainRender", true)
	case "retainerbox-render-on-invalidation", "retainerboxrenderoninvalidation":
		return shouldNoopWidgetSpecialBoolDefaultWrite(asset, target, rawValue, "retainerbox-render-on-invalidation", "RenderOnInvalidation", false)
	case "retainerbox-render-on-phase", "retainerboxrenderonphase":
		return shouldNoopWidgetSpecialBoolDefaultWrite(asset, target, rawValue, "retainerbox-render-on-phase", "RenderOnPhase", true)
	case "backgroundblur-apply-alpha-to-blur", "backgroundblurapplyalphatoblur":
		return shouldNoopWidgetSpecialBoolDefaultWrite(asset, target, rawValue, "backgroundblur-apply-alpha-to-blur", "bApplyAlphaToBlur", true)
	case "safezone-pad-left", "safezonepadleft":
		return shouldNoopWidgetSpecialBoolDefaultWrite(asset, target, rawValue, "safezone-pad-left", "PadLeft", true)
	case "safezone-pad-right", "safezonepadright":
		return shouldNoopWidgetSpecialBoolDefaultWrite(asset, target, rawValue, "safezone-pad-right", "PadRight", true)
	case "safezone-pad-top", "safezonepadtop":
		return shouldNoopWidgetSpecialBoolDefaultWrite(asset, target, rawValue, "safezone-pad-top", "PadTop", true)
	case "safezone-pad-bottom", "safezonepadbottom":
		return shouldNoopWidgetSpecialBoolDefaultWrite(asset, target, rawValue, "safezone-pad-bottom", "PadBottom", true)
	case "invalidationbox-can-cache", "invalidationboxcancache":
		return shouldNoopWidgetSpecialBoolDefaultWrite(asset, target, rawValue, "invalidationbox-can-cache", "bCanCache", true)
	case "editabletextbox-is-read-only", "editabletextboxisreadonly":
		return shouldNoopWidgetSpecialBoolDefaultWrite(asset, target, rawValue, "editabletextbox-is-read-only", "IsReadOnly", false)
	case "editabletextbox-is-password", "editabletextboxispassword":
		return shouldNoopWidgetSpecialBoolDefaultWrite(asset, target, rawValue, "editabletextbox-is-password", "IsPassword", false)
	case "editabletext-is-read-only", "editabletextisreadonly":
		return shouldNoopWidgetSpecialBoolDefaultWrite(asset, target, rawValue, "editabletext-is-read-only", "IsReadOnly", false)
	case "editabletext-is-password", "editabletextispassword":
		return shouldNoopWidgetSpecialBoolDefaultWrite(asset, target, rawValue, "editabletext-is-password", "IsPassword", false)
	case "multilineeditabletextbox-is-read-only", "multilineeditabletextboxisreadonly":
		return shouldNoopWidgetSpecialBoolDefaultWrite(asset, target, rawValue, "multilineeditabletextbox-is-read-only", "bIsReadOnly", false)
	default:
		return false, nil
	}
}

func shouldNoopWidgetSpecialBoolDefaultWrite(asset *uasset.Asset, target widgetWriteTarget, rawValue, propertyLabel, propertyName string, defaultValue bool) (bool, error) {
	requested, err := parseWidgetBoolValue(rawValue, propertyLabel)
	if err != nil {
		return false, err
	}
	if requested != defaultValue {
		return false, nil
	}
	for _, exportIdx := range target.Exports {
		decoded, found, err := decodeExportRootPropertyValueIfPresent(asset, exportIdx, propertyName)
		if err != nil {
			return false, fmt.Errorf("decode export %d %s: %w", exportIdx+1, propertyName, err)
		}
		if !found {
			continue
		}
		value, ok := widgetDecodedScalarBool(decoded)
		if !ok || value != defaultValue {
			return false, nil
		}
	}
	return true, nil
}

func shouldNormalizeWidgetWriteCompileArtifactsReversed(normalizedProperty string) bool {
	switch normalizedProperty {
	case "progressbar-percent", "progressbarpercent",
		"progressbar-fill-color", "progressbar-fillcolor",
		"progressbar-fill-color-and-opacity", "progressbar-fillcolorandopacity",
		"slider-value", "slidervalue",
		"slider-min-value", "sliderminvalue",
		"slider-max-value", "slidermaxvalue",
		"slider-step-size", "sliderstepsize",
		"slider-orientation", "sliderorientation",
		"spacer-size", "spacersize",
		"scrollbar-thickness", "scrollbarthickness",
		"scrollbar-orientation", "scrollbarorientation",
		"checkbox-checked-state", "checkboxcheckedstate",
		"checkbox-is-checked", "checkboxischecked",
		"editabletext-hint-text", "editabletexthinttext",
		"editabletext-is-read-only", "editabletextisreadonly",
		"editabletext-is-password", "editabletextispassword",
		"editabletext-minimum-desired-width", "editabletextminimumdesiredwidth",
		"editabletext-justification", "editabletextjustification",
		"editabletextbox-hint-text", "editabletextboxhinttext",
		"editabletextbox-is-read-only", "editabletextboxisreadonly",
		"editabletextbox-is-password", "editabletextboxispassword",
		"editabletextbox-minimum-desired-width", "editabletextboxminimumdesiredwidth",
		"editabletextbox-justification", "editabletextboxjustification",
		"multilineeditabletextbox-hint-text", "multilineeditabletextboxhinttext",
		"multilineeditabletextbox-is-read-only", "multilineeditabletextboxisreadonly",
		"multilineeditabletextbox-justification", "multilineeditabletextboxjustification",
		"spinbox-value", "spinboxvalue",
		"spinbox-min-value", "spinboxminvalue",
		"spinbox-max-value", "spinboxmaxvalue",
		"spinbox-delta", "spinboxdelta",
		"comboboxstring-selected-option", "comboboxstringselectedoption",
		"comboboxstring-options", "comboboxstringoptions",
		"is-focusable", "isfocusable",
		"button-is-focusable", "buttonisfocusable",
		"checkbox-is-focusable", "checkboxisfocusable",
		"slider-is-focusable", "sliderisfocusable",
		"scrollbox-is-focusable", "scrollboxisfocusable",
		"comboboxstring-is-focusable", "comboboxstringisfocusable",
		"listview-entry-widget-class", "listviewentrywidgetclass",
		"listview-orientation", "listvieworientation",
		"listview-selection-mode", "listviewselectionmode",
		"listview-consume-mouse-wheel", "listviewconsumemousewheel",
		"listview-is-focusable", "listviewisfocusable",
		"listview-return-focus-to-selection", "listviewreturnfocustoselection",
		"listview-clear-scroll-velocity-on-selection", "listviewclearscrollvelocityonselection",
		"listview-scroll-into-view-alignment", "listviewscrollintoviewalignment",
		"listview-wheel-scroll-multiplier", "listviewwheelscrollmultiplier",
		"listview-enable-scroll-animation", "listviewenablescrollanimation",
		"listview-allow-overscroll", "listviewallowoverscroll",
		"listview-enable-right-click-scrolling", "listviewenablerightclickscrolling",
		"listview-enable-touch-scrolling", "listviewenabletouchscrolling",
		"listview-is-pointer-scrolling-enabled", "listviewispointerscrollingenabled",
		"listview-is-gamepad-scrolling-enabled", "listviewisgamepadscrollingenabled",
		"listview-horizontal-entry-spacing", "listviewhorizontalentryspacing",
		"listview-vertical-entry-spacing", "listviewverticalentryspacing",
		"listview-scrollbar-padding", "listviewscrollbarpadding",
		"tileview-entry-width", "tileviewentrywidth",
		"tileview-entry-height", "tileviewentryheight",
		"tileview-scrollbar-disabled-visibility", "tileviewscrollbardisabledvisibility",
		"tileview-entry-size-includes-entry-spacing", "tileviewentrysizeincludesentryspacing",
		"scrollbox-orientation", "scrollboxorientation",
		"scrollbox-scrollbar-visibility", "scrollboxscrollbarvisibility", "scrollbox-scroll-bar-visibility",
		"scrollbox-consume-mouse-wheel", "scrollboxconsumemousewheel",
		"sizebox-width-override", "sizeboxwidthoverride", "sizebox-width", "sizeboxwidth",
		"sizebox-height-override", "sizeboxheightoverride", "sizebox-height", "sizeboxheight",
		"sizebox-min-desired-width", "sizeboxmindesiredwidth",
		"sizebox-min-desired-height", "sizeboxmindesiredheight",
		"sizebox-max-desired-width", "sizeboxmaxdesiredwidth",
		"sizebox-max-desired-height", "sizeboxmaxdesiredheight",
		"sizebox-min-aspect-ratio", "sizeboxminaspectratio",
		"sizebox-max-aspect-ratio", "sizeboxmaxaspectratio",
		"scalebox-stretch", "scaleboxstretch",
		"scalebox-stretch-direction", "scaleboxstretchdirection",
		"scalebox-user-specified-scale", "scaleboxuserspecifiedscale",
		"scalebox-ignore-inherited-scale", "scaleboxignoreinheritedscale",
		"wrapbox-wrap-size", "wrapboxwrapsize",
		"wrapbox-explicit-wrap-size", "wrapboxexplicitwrapsize",
		"wrapbox-inner-slot-padding", "wrapboxinnerslotpadding",
		"wrapbox-orientation", "wrapboxorientation",
		"widgetswitcher-active-widget-index", "widgetswitcheractivewidgetindex",
		"retainerbox-retain-render", "retainerboxretainrender",
		"retainerbox-render-on-invalidation", "retainerboxrenderoninvalidation",
		"retainerbox-render-on-phase", "retainerboxrenderonphase",
		"retainerbox-phase", "retainerboxphase",
		"retainerbox-phase-count", "retainerboxphasecount",
		"backgroundblur-strength", "backgroundblurstrength",
		"backgroundblur-apply-alpha-to-blur", "backgroundblurapplyalphatoblur",
		"safezone-pad-left", "safezonepadleft",
		"safezone-pad-right", "safezonepadright",
		"safezone-pad-top", "safezonepadtop",
		"safezone-pad-bottom", "safezonepadbottom",
		"invalidationbox-can-cache", "invalidationboxcancache",
		"uniformgridpanel-min-desired-slot-width", "uniformgridpanelmindesiredslotwidth",
		"uniformgridpanel-min-desired-slot-height", "uniformgridpanelmindesiredslotheight",
		"uniformgridpanel-slot-padding", "uniformgridpanelslotpadding":
		return true
	default:
		return false
	}
}

type textBlockStyleMutation struct {
	PropertyPath  string
	ValueJSON     string
	AddSpecJSON   string
	RequiredNames []string
	NoOp          bool
}

func buildWidgetSpecialPropertyMutation(target widgetWriteTarget, normalizedProperty, rawValue string) (string, string, string, []string, error) {
	switch normalizedProperty {
	case "progressbar-percent", "progressbarpercent":
		if !strings.EqualFold(target.ClassName, "ProgressBar") {
			return "", "", "", nil, fmt.Errorf("progressbar-percent requires a ProgressBar widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("progressbar-percent: %w", err)
		}
		return "Percent", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"Percent","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"Percent", "FloatProperty"}, nil
	case "progressbar-fill-color", "progressbar-fillcolor", "progressbar-fill-color-and-opacity", "progressbar-fillcolorandopacity":
		if !strings.EqualFold(target.ClassName, "ProgressBar") {
			return "", "", "", nil, fmt.Errorf("progressbar-fill-color requires a ProgressBar widget, got %s", target.ClassName)
		}
		color, err := parseWidgetLinearColorValue(rawValue, normalizedProperty)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := buildWidgetLinearColorValueJSON(color)
		if err != nil {
			return "", "", "", nil, err
		}
		return "FillColorAndOpacity", valueJSON, fmt.Sprintf(`{"name":"FillColorAndOpacity","type":"StructProperty(LinearColor(/Script/CoreUObject))","value":%s}`, valueJSON), []string{"FillColorAndOpacity", "StructProperty", "LinearColor", "/Script/CoreUObject"}, nil
	case "slider-value", "slidervalue":
		if !strings.EqualFold(target.ClassName, "Slider") {
			return "", "", "", nil, fmt.Errorf("slider-value requires a Slider widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("slider-value: %w", err)
		}
		return "Value", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"Value","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"Value", "FloatProperty"}, nil
	case "slider-min-value", "sliderminvalue":
		if !strings.EqualFold(target.ClassName, "Slider") {
			return "", "", "", nil, fmt.Errorf("slider-min-value requires a Slider widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("slider-min-value: %w", err)
		}
		return "MinValue", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"MinValue","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"MinValue", "FloatProperty"}, nil
	case "slider-max-value", "slidermaxvalue":
		if !strings.EqualFold(target.ClassName, "Slider") {
			return "", "", "", nil, fmt.Errorf("slider-max-value requires a Slider widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("slider-max-value: %w", err)
		}
		return "MaxValue", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"MaxValue","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"MaxValue", "FloatProperty"}, nil
	case "slider-step-size", "sliderstepsize":
		if !strings.EqualFold(target.ClassName, "Slider") {
			return "", "", "", nil, fmt.Errorf("slider-step-size requires a Slider widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("slider-step-size: %w", err)
		}
		return "StepSize", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"StepSize","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"StepSize", "FloatProperty"}, nil
	case "slider-orientation", "sliderorientation":
		if !strings.EqualFold(target.ClassName, "Slider") {
			return "", "", "", nil, fmt.Errorf("slider-orientation requires a Slider widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetOrientationValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "EOrientation", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "Orientation", valueJSON, fmt.Sprintf(`{"name":"Orientation","type":"ByteProperty(EOrientation(/Script/SlateCore))","value":%s}`, valueJSON), []string{"Orientation", "ByteProperty", "EOrientation", "/Script/SlateCore", enumValue}, nil
	case "spacer-size", "spacersize":
		if !strings.EqualFold(target.ClassName, "Spacer") {
			return "", "", "", nil, fmt.Errorf("spacer-size requires a Spacer widget, got %s", target.ClassName)
		}
		x, y, err := parseWidgetFloatPair(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("spacer-size: %w", err)
		}
		valueJSON, err := buildWidgetVector2DValueJSON(x, y)
		if err != nil {
			return "", "", "", nil, err
		}
		return "Size", valueJSON, fmt.Sprintf(`{"name":"Size","flags":8,"type":"StructProperty(Vector2D(/Script/CoreUObject))","value":%s}`, valueJSON), []string{"Size", "StructProperty", "Vector2D", "/Script/CoreUObject"}, nil
	case "scrollbar-thickness", "scrollbarthickness":
		if !strings.EqualFold(target.ClassName, "ScrollBar") {
			return "", "", "", nil, fmt.Errorf("scrollbar-thickness requires a ScrollBar widget, got %s", target.ClassName)
		}
		x, y, err := parseWidgetFloatPair(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("scrollbar-thickness: %w", err)
		}
		valueJSON, err := buildWidgetVector2DValueJSON(x, y)
		if err != nil {
			return "", "", "", nil, err
		}
		return "Thickness", valueJSON, fmt.Sprintf(`{"name":"Thickness","flags":8,"type":"StructProperty(Vector2D(/Script/CoreUObject))","value":%s}`, valueJSON), []string{"Thickness", "StructProperty", "Vector2D", "/Script/CoreUObject"}, nil
	case "scrollbar-orientation", "scrollbarorientation":
		if !strings.EqualFold(target.ClassName, "ScrollBar") {
			return "", "", "", nil, fmt.Errorf("scrollbar-orientation requires a ScrollBar widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetOrientationValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "EOrientation", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "Orientation", valueJSON, fmt.Sprintf(`{"name":"Orientation","type":"ByteProperty(EOrientation(/Script/SlateCore))","value":%s}`, valueJSON), []string{"Orientation", "ByteProperty", "EOrientation", "/Script/SlateCore", enumValue}, nil
	case "is-focusable", "isfocusable",
		"button-is-focusable", "buttonisfocusable",
		"checkbox-is-focusable", "checkboxisfocusable",
		"slider-is-focusable", "sliderisfocusable",
		"scrollbox-is-focusable", "scrollboxisfocusable",
		"comboboxstring-is-focusable", "comboboxstringisfocusable":
		return buildWidgetIsFocusableMutation(target, normalizedProperty, rawValue)
	case "editabletextbox-hint-text", "editabletextboxhinttext":
		if !strings.EqualFold(target.ClassName, "EditableTextBox") {
			return "", "", "", nil, fmt.Errorf("editabletextbox-hint-text requires an EditableTextBox widget, got %s", target.ClassName)
		}
		valueJSON, err := buildWidgetWriteFTextPayload(nil, 0, "HintText", target.ClassName, rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("editabletextbox-hint-text: %w", err)
		}
		return "HintText", valueJSON, fmt.Sprintf(`{"name":"HintText","type":"TextProperty","value":%s}`, valueJSON), []string{"HintText", "TextProperty"}, nil
	case "editabletextbox-is-read-only", "editabletextboxisreadonly":
		if !strings.EqualFold(target.ClassName, "EditableTextBox") {
			return "", "", "", nil, fmt.Errorf("editabletextbox-is-read-only requires an EditableTextBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "editabletextbox-is-read-only")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "IsReadOnly", valueJSON, fmt.Sprintf(`{"name":"IsReadOnly","type":"BoolProperty","value":%s}`, valueJSON), []string{"IsReadOnly", "BoolProperty"}, nil
	case "editabletextbox-is-password", "editabletextboxispassword":
		if !strings.EqualFold(target.ClassName, "EditableTextBox") {
			return "", "", "", nil, fmt.Errorf("editabletextbox-is-password requires an EditableTextBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "editabletextbox-is-password")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "IsPassword", valueJSON, fmt.Sprintf(`{"name":"IsPassword","type":"BoolProperty","value":%s}`, valueJSON), []string{"IsPassword", "BoolProperty"}, nil
	case "editabletextbox-minimum-desired-width", "editabletextboxminimumdesiredwidth":
		if !strings.EqualFold(target.ClassName, "EditableTextBox") {
			return "", "", "", nil, fmt.Errorf("editabletextbox-minimum-desired-width requires an EditableTextBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("editabletextbox-minimum-desired-width: %w", err)
		}
		return "MinimumDesiredWidth", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"MinimumDesiredWidth","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"MinimumDesiredWidth", "FloatProperty"}, nil
	case "editabletextbox-justification", "editabletextboxjustification":
		if !strings.EqualFold(target.ClassName, "EditableTextBox") {
			return "", "", "", nil, fmt.Errorf("editabletextbox-justification requires an EditableTextBox widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetTextJustificationValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "ETextJustify", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "Justification", valueJSON, fmt.Sprintf(`{"name":"Justification","type":"ByteProperty(ETextJustify(/Script/Slate))","value":%s}`, valueJSON), []string{"Justification", "ByteProperty", "ETextJustify", "/Script/Slate", enumValue}, nil
	case "editabletext-hint-text", "editabletexthinttext":
		if !strings.EqualFold(target.ClassName, "EditableText") {
			return "", "", "", nil, fmt.Errorf("editabletext-hint-text requires an EditableText widget, got %s", target.ClassName)
		}
		valueJSON, err := buildWidgetWriteFTextPayload(nil, 0, "HintText", target.ClassName, rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("editabletext-hint-text: %w", err)
		}
		return "HintText", valueJSON, fmt.Sprintf(`{"name":"HintText","type":"TextProperty","value":%s}`, valueJSON), []string{"HintText", "TextProperty"}, nil
	case "editabletext-is-read-only", "editabletextisreadonly":
		if !strings.EqualFold(target.ClassName, "EditableText") {
			return "", "", "", nil, fmt.Errorf("editabletext-is-read-only requires an EditableText widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "editabletext-is-read-only")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "IsReadOnly", valueJSON, fmt.Sprintf(`{"name":"IsReadOnly","type":"BoolProperty","value":%s}`, valueJSON), []string{"IsReadOnly", "BoolProperty"}, nil
	case "editabletext-is-password", "editabletextispassword":
		if !strings.EqualFold(target.ClassName, "EditableText") {
			return "", "", "", nil, fmt.Errorf("editabletext-is-password requires an EditableText widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "editabletext-is-password")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "IsPassword", valueJSON, fmt.Sprintf(`{"name":"IsPassword","type":"BoolProperty","value":%s}`, valueJSON), []string{"IsPassword", "BoolProperty"}, nil
	case "editabletext-minimum-desired-width", "editabletextminimumdesiredwidth":
		if !strings.EqualFold(target.ClassName, "EditableText") {
			return "", "", "", nil, fmt.Errorf("editabletext-minimum-desired-width requires an EditableText widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("editabletext-minimum-desired-width: %w", err)
		}
		return "MinimumDesiredWidth", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"MinimumDesiredWidth","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"MinimumDesiredWidth", "FloatProperty"}, nil
	case "editabletext-justification", "editabletextjustification":
		if !strings.EqualFold(target.ClassName, "EditableText") {
			return "", "", "", nil, fmt.Errorf("editabletext-justification requires an EditableText widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetTextJustificationValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "ETextJustify", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "Justification", valueJSON, fmt.Sprintf(`{"name":"Justification","type":"ByteProperty(ETextJustify(/Script/Slate))","value":%s}`, valueJSON), []string{"Justification", "ByteProperty", "ETextJustify", "/Script/Slate", enumValue}, nil
	case "multilineeditabletextbox-hint-text", "multilineeditabletextboxhinttext":
		if !strings.EqualFold(target.ClassName, "MultiLineEditableTextBox") {
			return "", "", "", nil, fmt.Errorf("multilineeditabletextbox-hint-text requires a MultiLineEditableTextBox widget, got %s", target.ClassName)
		}
		valueJSON, err := buildWidgetWriteFTextPayload(nil, 0, "HintText", target.ClassName, rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("multilineeditabletextbox-hint-text: %w", err)
		}
		return "HintText", valueJSON, fmt.Sprintf(`{"name":"HintText","type":"TextProperty","value":%s}`, valueJSON), []string{"HintText", "TextProperty"}, nil
	case "multilineeditabletextbox-is-read-only", "multilineeditabletextboxisreadonly":
		if !strings.EqualFold(target.ClassName, "MultiLineEditableTextBox") {
			return "", "", "", nil, fmt.Errorf("multilineeditabletextbox-is-read-only requires a MultiLineEditableTextBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "multilineeditabletextbox-is-read-only")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "bIsReadOnly", valueJSON, fmt.Sprintf(`{"name":"bIsReadOnly","type":"BoolProperty","value":%s}`, valueJSON), []string{"bIsReadOnly", "BoolProperty"}, nil
	case "multilineeditabletextbox-justification", "multilineeditabletextboxjustification":
		if !strings.EqualFold(target.ClassName, "MultiLineEditableTextBox") {
			return "", "", "", nil, fmt.Errorf("multilineeditabletextbox-justification requires a MultiLineEditableTextBox widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetTextJustificationValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "ETextJustify", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "Justification", valueJSON, fmt.Sprintf(`{"name":"Justification","type":"ByteProperty(ETextJustify(/Script/Slate))","value":%s}`, valueJSON), []string{"Justification", "ByteProperty", "ETextJustify", "/Script/Slate", enumValue}, nil
	case "spinbox-value", "spinboxvalue":
		if !strings.EqualFold(target.ClassName, "SpinBox") {
			return "", "", "", nil, fmt.Errorf("spinbox-value requires a SpinBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("spinbox-value: %w", err)
		}
		return "Value", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"Value","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"Value", "FloatProperty"}, nil
	case "spinbox-min-value", "spinboxminvalue":
		if !strings.EqualFold(target.ClassName, "SpinBox") {
			return "", "", "", nil, fmt.Errorf("spinbox-min-value requires a SpinBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("spinbox-min-value: %w", err)
		}
		return "MinValue", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"MinValue","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"MinValue", "FloatProperty", "bOverride_MinValue", "BoolProperty"}, nil
	case "spinbox-max-value", "spinboxmaxvalue":
		if !strings.EqualFold(target.ClassName, "SpinBox") {
			return "", "", "", nil, fmt.Errorf("spinbox-max-value requires a SpinBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("spinbox-max-value: %w", err)
		}
		return "MaxValue", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"MaxValue","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"MaxValue", "FloatProperty", "bOverride_MaxValue", "BoolProperty"}, nil
	case "spinbox-delta", "spinboxdelta":
		if !strings.EqualFold(target.ClassName, "SpinBox") {
			return "", "", "", nil, fmt.Errorf("spinbox-delta requires a SpinBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("spinbox-delta: %w", err)
		}
		return "Delta", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"Delta","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"Delta", "FloatProperty"}, nil
	case "comboboxstring-selected-option", "comboboxstringselectedoption":
		if !strings.EqualFold(target.ClassName, "ComboBoxString") {
			return "", "", "", nil, fmt.Errorf("comboboxstring-selected-option requires a ComboBoxString widget, got %s", target.ClassName)
		}
		valueJSON, err := marshalJSONValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		return "SelectedOption", valueJSON, fmt.Sprintf(`{"name":"SelectedOption","type":"StrProperty","value":%s}`, valueJSON), []string{"SelectedOption", "StrProperty"}, nil
	case "comboboxstring-options", "comboboxstringoptions":
		if !strings.EqualFold(target.ClassName, "ComboBoxString") {
			return "", "", "", nil, fmt.Errorf("comboboxstring-options requires a ComboBoxString widget, got %s", target.ClassName)
		}
		values, err := parseWidgetStringListValue(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("comboboxstring-options: %w", err)
		}
		valueJSON, err := marshalJSONValue(values)
		if err != nil {
			return "", "", "", nil, err
		}
		return "DefaultOptions", valueJSON, fmt.Sprintf(`{"name":"DefaultOptions","type":"ArrayProperty(StrProperty)","value":%s}`, valueJSON), []string{"DefaultOptions", "ArrayProperty", "StrProperty"}, nil
	case "listview-entry-widget-class", "listviewentrywidgetclass":
		if !widgetClassSupportsListViewProperties(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-entry-widget-class requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		return "EntryWidgetClass", "", "", nil, nil
	case "listview-orientation", "listvieworientation":
		if !widgetClassSupportsListViewProperties(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-orientation requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetOrientationValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "EOrientation", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "Orientation", valueJSON, fmt.Sprintf(`{"name":"Orientation","type":"ByteProperty(EOrientation(/Script/SlateCore))","value":%s}`, valueJSON), []string{"Orientation", "ByteProperty", "EOrientation", "/Script/SlateCore", enumValue}, nil
	case "listview-selection-mode", "listviewselectionmode":
		if !widgetClassSupportsListViewProperties(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-selection-mode requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetSelectionModeValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "ESelectionMode", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "SelectionMode", valueJSON, fmt.Sprintf(`{"name":"SelectionMode","type":"ByteProperty(ESelectionMode(/Script/Slate))","value":%s}`, valueJSON), []string{"SelectionMode", "ByteProperty", "ESelectionMode", "/Script/Slate", enumValue}, nil
	case "listview-consume-mouse-wheel", "listviewconsumemousewheel":
		if !widgetClassSupportsListViewProperties(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-consume-mouse-wheel requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetConsumeMouseWheelValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "EConsumeMouseWheel", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "ConsumeMouseWheel", valueJSON, fmt.Sprintf(`{"name":"ConsumeMouseWheel","type":"EnumProperty(EConsumeMouseWheel(/Script/SlateCore),ByteProperty)","value":%s}`, valueJSON), []string{"ConsumeMouseWheel", "EnumProperty", "ByteProperty", "EConsumeMouseWheel", "/Script/SlateCore", enumValue}, nil
	case "listview-is-focusable", "listviewisfocusable":
		if !widgetClassSupportsListViewProperties(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-is-focusable requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "listview-is-focusable")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "bIsFocusable", valueJSON, fmt.Sprintf(`{"name":"bIsFocusable","type":"BoolProperty","value":%s}`, valueJSON), []string{"bIsFocusable", "BoolProperty"}, nil
	case "listview-return-focus-to-selection", "listviewreturnfocustoselection":
		if !widgetClassSupportsListViewProperties(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-return-focus-to-selection requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "listview-return-focus-to-selection")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "bReturnFocusToSelection", valueJSON, fmt.Sprintf(`{"name":"bReturnFocusToSelection","type":"BoolProperty","value":%s}`, valueJSON), []string{"bReturnFocusToSelection", "BoolProperty"}, nil
	case "listview-clear-scroll-velocity-on-selection", "listviewclearscrollvelocityonselection":
		if !widgetClassSupportsListViewProperties(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-clear-scroll-velocity-on-selection requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "listview-clear-scroll-velocity-on-selection")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "bClearScrollVelocityOnSelection", valueJSON, fmt.Sprintf(`{"name":"bClearScrollVelocityOnSelection","type":"BoolProperty","value":%s}`, valueJSON), []string{"bClearScrollVelocityOnSelection", "BoolProperty"}, nil
	case "listview-scroll-into-view-alignment", "listviewscrollintoviewalignment":
		if !widgetClassSupportsListViewProperties(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-scroll-into-view-alignment requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetScrollIntoViewAlignmentValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "EScrollIntoViewAlignment", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "ScrollIntoViewAlignment", valueJSON, fmt.Sprintf(`{"name":"ScrollIntoViewAlignment","type":"EnumProperty(EScrollIntoViewAlignment(/Script/Slate),ByteProperty)","value":%s}`, valueJSON), []string{"ScrollIntoViewAlignment", "EnumProperty", "ByteProperty", "EScrollIntoViewAlignment", "/Script/Slate", enumValue}, nil
	case "listview-wheel-scroll-multiplier", "listviewwheelscrollmultiplier":
		if !widgetClassSupportsListViewProperties(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-wheel-scroll-multiplier requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("listview-wheel-scroll-multiplier: %w", err)
		}
		return "WheelScrollMultiplier", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"WheelScrollMultiplier","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"WheelScrollMultiplier", "FloatProperty"}, nil
	case "listview-enable-scroll-animation", "listviewenablescrollanimation":
		if !widgetClassSupportsListViewProperties(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-enable-scroll-animation requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "listview-enable-scroll-animation")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "bEnableScrollAnimation", valueJSON, fmt.Sprintf(`{"name":"bEnableScrollAnimation","type":"BoolProperty","value":%s}`, valueJSON), []string{"bEnableScrollAnimation", "BoolProperty"}, nil
	case "listview-allow-overscroll", "listviewallowoverscroll":
		if !widgetClassSupportsListViewProperties(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-allow-overscroll requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "listview-allow-overscroll")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "AllowOverscroll", valueJSON, fmt.Sprintf(`{"name":"AllowOverscroll","type":"BoolProperty","value":%s}`, valueJSON), []string{"AllowOverscroll", "BoolProperty"}, nil
	case "listview-enable-right-click-scrolling", "listviewenablerightclickscrolling":
		if !widgetClassSupportsListViewProperties(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-enable-right-click-scrolling requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "listview-enable-right-click-scrolling")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "bEnableRightClickScrolling", valueJSON, fmt.Sprintf(`{"name":"bEnableRightClickScrolling","type":"BoolProperty","value":%s}`, valueJSON), []string{"bEnableRightClickScrolling", "BoolProperty"}, nil
	case "listview-enable-touch-scrolling", "listviewenabletouchscrolling":
		if !widgetClassSupportsListViewProperties(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-enable-touch-scrolling requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "listview-enable-touch-scrolling")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "bEnableTouchScrolling", valueJSON, fmt.Sprintf(`{"name":"bEnableTouchScrolling","type":"BoolProperty","value":%s}`, valueJSON), []string{"bEnableTouchScrolling", "BoolProperty"}, nil
	case "listview-is-pointer-scrolling-enabled", "listviewispointerscrollingenabled":
		if !widgetClassSupportsListViewProperties(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-is-pointer-scrolling-enabled requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "listview-is-pointer-scrolling-enabled")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "bIsPointerScrollingEnabled", valueJSON, fmt.Sprintf(`{"name":"bIsPointerScrollingEnabled","type":"BoolProperty","value":%s}`, valueJSON), []string{"bIsPointerScrollingEnabled", "BoolProperty"}, nil
	case "listview-is-gamepad-scrolling-enabled", "listviewisgamepadscrollingenabled":
		if !widgetClassSupportsListViewProperties(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-is-gamepad-scrolling-enabled requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "listview-is-gamepad-scrolling-enabled")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "bIsGamepadScrollingEnabled", valueJSON, fmt.Sprintf(`{"name":"bIsGamepadScrollingEnabled","type":"BoolProperty","value":%s}`, valueJSON), []string{"bIsGamepadScrollingEnabled", "BoolProperty"}, nil
	case "listview-horizontal-entry-spacing", "listviewhorizontalentryspacing":
		if !widgetClassSupportsListViewSpacing(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-horizontal-entry-spacing requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("listview-horizontal-entry-spacing: %w", err)
		}
		return "HorizontalEntrySpacing", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"HorizontalEntrySpacing","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"HorizontalEntrySpacing", "FloatProperty"}, nil
	case "listview-vertical-entry-spacing", "listviewverticalentryspacing":
		if !widgetClassSupportsListViewSpacing(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-vertical-entry-spacing requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("listview-vertical-entry-spacing: %w", err)
		}
		return "VerticalEntrySpacing", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"VerticalEntrySpacing","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"VerticalEntrySpacing", "FloatProperty"}, nil
	case "listview-scrollbar-padding", "listviewscrollbarpadding":
		if !widgetClassSupportsListViewSpacing(target.ClassName) {
			return "", "", "", nil, fmt.Errorf("listview-scrollbar-padding requires a ListView, TileView, or TreeView widget, got %s", target.ClassName)
		}
		padding, err := parseWidgetMarginValue(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("listview-scrollbar-padding: %w", err)
		}
		valueJSON, err := buildWidgetMarginValueJSON(padding)
		if err != nil {
			return "", "", "", nil, err
		}
		return "ScrollBarPadding", valueJSON, fmt.Sprintf(`{"name":"ScrollBarPadding","type":"StructProperty(Margin(/Script/SlateCore))","value":%s}`, valueJSON), []string{"ScrollBarPadding", "StructProperty", "Margin", "/Script/SlateCore", "Left", "Top", "Right", "Bottom", "FloatProperty"}, nil
	case "tileview-entry-width", "tileviewentrywidth":
		if !strings.EqualFold(target.ClassName, "TileView") {
			return "", "", "", nil, fmt.Errorf("tileview-entry-width requires a TileView widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("tileview-entry-width: %w", err)
		}
		return "EntryWidth", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"EntryWidth","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"EntryWidth", "FloatProperty"}, nil
	case "tileview-entry-height", "tileviewentryheight":
		if !strings.EqualFold(target.ClassName, "TileView") {
			return "", "", "", nil, fmt.Errorf("tileview-entry-height requires a TileView widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("tileview-entry-height: %w", err)
		}
		return "EntryHeight", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"EntryHeight","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"EntryHeight", "FloatProperty"}, nil
	case "tileview-scrollbar-disabled-visibility", "tileviewscrollbardisabledvisibility":
		if !strings.EqualFold(target.ClassName, "TileView") {
			return "", "", "", nil, fmt.Errorf("tileview-scrollbar-disabled-visibility requires a TileView widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetVisibilityValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "ESlateVisibility", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "ScrollbarDisabledVisibility", valueJSON, fmt.Sprintf(`{"name":"ScrollbarDisabledVisibility","type":"EnumProperty(ESlateVisibility(/Script/UMG),ByteProperty)","value":%s}`, valueJSON), []string{"ScrollbarDisabledVisibility", "EnumProperty", "ByteProperty", "ESlateVisibility", "/Script/UMG", enumValue}, nil
	case "tileview-entry-size-includes-entry-spacing", "tileviewentrysizeincludesentryspacing":
		if !strings.EqualFold(target.ClassName, "TileView") {
			return "", "", "", nil, fmt.Errorf("tileview-entry-size-includes-entry-spacing requires a TileView widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "tileview-entry-size-includes-entry-spacing")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "bEntrySizeIncludesEntrySpacing", valueJSON, fmt.Sprintf(`{"name":"bEntrySizeIncludesEntrySpacing","type":"BoolProperty","value":%s}`, valueJSON), []string{"bEntrySizeIncludesEntrySpacing", "BoolProperty"}, nil
	case "checkbox-checked-state", "checkboxcheckedstate":
		if !strings.EqualFold(target.ClassName, "CheckBox") {
			return "", "", "", nil, fmt.Errorf("checkbox-checked-state requires a CheckBox widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetCheckBoxStateValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "ECheckBoxState", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "CheckedState", valueJSON, fmt.Sprintf(`{"name":"CheckedState","type":"EnumProperty(ECheckBoxState(/Script/SlateCore),ByteProperty)","value":%s}`, valueJSON), []string{"CheckedState", "EnumProperty", "ByteProperty", "ECheckBoxState", "/Script/SlateCore", enumValue}, nil
	case "checkbox-is-checked", "checkboxischecked":
		if !strings.EqualFold(target.ClassName, "CheckBox") {
			return "", "", "", nil, fmt.Errorf("checkbox-is-checked requires a CheckBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "checkbox-is-checked")
		if err != nil {
			return "", "", "", nil, err
		}
		enumValue := "ECheckBoxState::Unchecked"
		if value {
			enumValue = "ECheckBoxState::Checked"
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "ECheckBoxState", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "CheckedState", valueJSON, fmt.Sprintf(`{"name":"CheckedState","type":"EnumProperty(ECheckBoxState(/Script/SlateCore),ByteProperty)","value":%s}`, valueJSON), []string{"CheckedState", "EnumProperty", "ByteProperty", "ECheckBoxState", "/Script/SlateCore", enumValue}, nil
	case "scrollbox-orientation", "scrollboxorientation":
		if !strings.EqualFold(target.ClassName, "ScrollBox") {
			return "", "", "", nil, fmt.Errorf("scrollbox-orientation requires a ScrollBox widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetOrientationValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "EOrientation", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "Orientation", valueJSON, fmt.Sprintf(`{"name":"Orientation","type":"ByteProperty(EOrientation(/Script/SlateCore))","value":%s}`, valueJSON), []string{"Orientation", "ByteProperty", "EOrientation", "/Script/SlateCore", enumValue}, nil
	case "scrollbox-scrollbar-visibility", "scrollboxscrollbarvisibility", "scrollbox-scroll-bar-visibility":
		if !strings.EqualFold(target.ClassName, "ScrollBox") {
			return "", "", "", nil, fmt.Errorf("scrollbox-scrollbar-visibility requires a ScrollBox widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetVisibilityValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "ESlateVisibility", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "ScrollBarVisibility", valueJSON, fmt.Sprintf(`{"name":"ScrollBarVisibility","type":"EnumProperty(ESlateVisibility(/Script/UMG),ByteProperty)","value":%s}`, valueJSON), []string{"ScrollBarVisibility", "EnumProperty", "ByteProperty", "ESlateVisibility", "/Script/UMG", enumValue}, nil
	case "scrollbox-consume-mouse-wheel", "scrollboxconsumemousewheel":
		if !strings.EqualFold(target.ClassName, "ScrollBox") {
			return "", "", "", nil, fmt.Errorf("scrollbox-consume-mouse-wheel requires a ScrollBox widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetConsumeMouseWheelValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "EConsumeMouseWheel", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "ConsumeMouseWheel", valueJSON, fmt.Sprintf(`{"name":"ConsumeMouseWheel","type":"EnumProperty(EConsumeMouseWheel(/Script/SlateCore),ByteProperty)","value":%s}`, valueJSON), []string{"ConsumeMouseWheel", "EnumProperty", "ByteProperty", "EConsumeMouseWheel", "/Script/SlateCore", enumValue}, nil
	case "sizebox-width-override", "sizeboxwidthoverride", "sizebox-width", "sizeboxwidth":
		if !strings.EqualFold(target.ClassName, "SizeBox") {
			return "", "", "", nil, fmt.Errorf("sizebox-width-override requires a SizeBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("sizebox-width-override: %w", err)
		}
		return "WidthOverride", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"WidthOverride","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"WidthOverride", "FloatProperty", "bOverride_WidthOverride", "BoolProperty"}, nil
	case "sizebox-height-override", "sizeboxheightoverride", "sizebox-height", "sizeboxheight":
		if !strings.EqualFold(target.ClassName, "SizeBox") {
			return "", "", "", nil, fmt.Errorf("sizebox-height-override requires a SizeBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("sizebox-height-override: %w", err)
		}
		return "HeightOverride", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"HeightOverride","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"HeightOverride", "FloatProperty", "bOverride_HeightOverride", "BoolProperty"}, nil
	case "sizebox-min-desired-width", "sizeboxmindesiredwidth":
		if !strings.EqualFold(target.ClassName, "SizeBox") {
			return "", "", "", nil, fmt.Errorf("sizebox-min-desired-width requires a SizeBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("sizebox-min-desired-width: %w", err)
		}
		return "MinDesiredWidth", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"MinDesiredWidth","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"MinDesiredWidth", "FloatProperty", "bOverride_MinDesiredWidth", "BoolProperty"}, nil
	case "sizebox-min-desired-height", "sizeboxmindesiredheight":
		if !strings.EqualFold(target.ClassName, "SizeBox") {
			return "", "", "", nil, fmt.Errorf("sizebox-min-desired-height requires a SizeBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("sizebox-min-desired-height: %w", err)
		}
		return "MinDesiredHeight", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"MinDesiredHeight","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"MinDesiredHeight", "FloatProperty", "bOverride_MinDesiredHeight", "BoolProperty"}, nil
	case "sizebox-max-desired-width", "sizeboxmaxdesiredwidth":
		if !strings.EqualFold(target.ClassName, "SizeBox") {
			return "", "", "", nil, fmt.Errorf("sizebox-max-desired-width requires a SizeBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("sizebox-max-desired-width: %w", err)
		}
		return "MaxDesiredWidth", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"MaxDesiredWidth","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"MaxDesiredWidth", "FloatProperty", "bOverride_MaxDesiredWidth", "BoolProperty"}, nil
	case "sizebox-max-desired-height", "sizeboxmaxdesiredheight":
		if !strings.EqualFold(target.ClassName, "SizeBox") {
			return "", "", "", nil, fmt.Errorf("sizebox-max-desired-height requires a SizeBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("sizebox-max-desired-height: %w", err)
		}
		return "MaxDesiredHeight", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"MaxDesiredHeight","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"MaxDesiredHeight", "FloatProperty", "bOverride_MaxDesiredHeight", "BoolProperty"}, nil
	case "sizebox-min-aspect-ratio", "sizeboxminaspectratio":
		if !strings.EqualFold(target.ClassName, "SizeBox") {
			return "", "", "", nil, fmt.Errorf("sizebox-min-aspect-ratio requires a SizeBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("sizebox-min-aspect-ratio: %w", err)
		}
		return "MinAspectRatio", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"MinAspectRatio","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"MinAspectRatio", "FloatProperty", "bOverride_MinAspectRatio", "BoolProperty"}, nil
	case "sizebox-max-aspect-ratio", "sizeboxmaxaspectratio":
		if !strings.EqualFold(target.ClassName, "SizeBox") {
			return "", "", "", nil, fmt.Errorf("sizebox-max-aspect-ratio requires a SizeBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("sizebox-max-aspect-ratio: %w", err)
		}
		return "MaxAspectRatio", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"MaxAspectRatio","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"MaxAspectRatio", "FloatProperty", "bOverride_MaxAspectRatio", "BoolProperty"}, nil
	case "scalebox-stretch", "scaleboxstretch":
		if !strings.EqualFold(target.ClassName, "ScaleBox") {
			return "", "", "", nil, fmt.Errorf("scalebox-stretch requires a ScaleBox widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetScaleBoxStretchValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "EStretch", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "Stretch", valueJSON, fmt.Sprintf(`{"name":"Stretch","type":"ByteProperty(EStretch(/Script/Slate))","value":%s}`, valueJSON), []string{"Stretch", "ByteProperty", "EStretch", "/Script/Slate", enumValue}, nil
	case "scalebox-stretch-direction", "scaleboxstretchdirection":
		if !strings.EqualFold(target.ClassName, "ScaleBox") {
			return "", "", "", nil, fmt.Errorf("scalebox-stretch-direction requires a ScaleBox widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetScaleBoxStretchDirectionValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "EStretchDirection", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "StretchDirection", valueJSON, fmt.Sprintf(`{"name":"StretchDirection","type":"ByteProperty(EStretchDirection(/Script/Slate))","value":%s}`, valueJSON), []string{"StretchDirection", "ByteProperty", "EStretchDirection", "/Script/Slate", enumValue}, nil
	case "scalebox-user-specified-scale", "scaleboxuserspecifiedscale":
		if !strings.EqualFold(target.ClassName, "ScaleBox") {
			return "", "", "", nil, fmt.Errorf("scalebox-user-specified-scale requires a ScaleBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("scalebox-user-specified-scale: %w", err)
		}
		return "UserSpecifiedScale", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"UserSpecifiedScale","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"UserSpecifiedScale", "FloatProperty"}, nil
	case "scalebox-ignore-inherited-scale", "scaleboxignoreinheritedscale":
		if !strings.EqualFold(target.ClassName, "ScaleBox") {
			return "", "", "", nil, fmt.Errorf("scalebox-ignore-inherited-scale requires a ScaleBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "scalebox-ignore-inherited-scale")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "IgnoreInheritedScale", valueJSON, fmt.Sprintf(`{"name":"IgnoreInheritedScale","type":"BoolProperty","value":%s}`, valueJSON), []string{"IgnoreInheritedScale", "BoolProperty"}, nil
	case "wrapbox-wrap-size", "wrapboxwrapsize":
		if !strings.EqualFold(target.ClassName, "WrapBox") {
			return "", "", "", nil, fmt.Errorf("wrapbox-wrap-size requires a WrapBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("wrapbox-wrap-size: %w", err)
		}
		return "WrapSize", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"WrapSize","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"WrapSize", "FloatProperty"}, nil
	case "wrapbox-explicit-wrap-size", "wrapboxexplicitwrapsize":
		if !strings.EqualFold(target.ClassName, "WrapBox") {
			return "", "", "", nil, fmt.Errorf("wrapbox-explicit-wrap-size requires a WrapBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "wrapbox-explicit-wrap-size")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "bExplicitWrapSize", valueJSON, fmt.Sprintf(`{"name":"bExplicitWrapSize","flags":16,"type":"BoolProperty","value":%s}`, valueJSON), []string{"bExplicitWrapSize", "BoolProperty"}, nil
	case "wrapbox-inner-slot-padding", "wrapboxinnerslotpadding":
		if !strings.EqualFold(target.ClassName, "WrapBox") {
			return "", "", "", nil, fmt.Errorf("wrapbox-inner-slot-padding requires a WrapBox widget, got %s", target.ClassName)
		}
		x, y, err := parseWidgetFloatPair(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("wrapbox-inner-slot-padding: %w", err)
		}
		valueJSON, err := buildWidgetVector2DValueJSON(x, y)
		if err != nil {
			return "", "", "", nil, err
		}
		return "InnerSlotPadding", valueJSON, fmt.Sprintf(`{"name":"InnerSlotPadding","flags":8,"type":"StructProperty(Vector2D(/Script/CoreUObject))","value":%s}`, valueJSON), []string{"InnerSlotPadding", "StructProperty", "Vector2D", "/Script/CoreUObject"}, nil
	case "wrapbox-orientation", "wrapboxorientation":
		if !strings.EqualFold(target.ClassName, "WrapBox") {
			return "", "", "", nil, fmt.Errorf("wrapbox-orientation requires a WrapBox widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetOrientationValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "EOrientation", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "Orientation", valueJSON, fmt.Sprintf(`{"name":"Orientation","type":"ByteProperty(EOrientation(/Script/SlateCore))","value":%s}`, valueJSON), []string{"Orientation", "ByteProperty", "EOrientation", "/Script/SlateCore", enumValue}, nil
	case "widgetswitcher-active-widget-index", "widgetswitcheractivewidgetindex":
		if !strings.EqualFold(target.ClassName, "WidgetSwitcher") {
			return "", "", "", nil, fmt.Errorf("widgetswitcher-active-widget-index requires a WidgetSwitcher widget, got %s", target.ClassName)
		}
		value, err := parseWidgetIntValue(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("widgetswitcher-active-widget-index: %w", err)
		}
		return "ActiveWidgetIndex", strconv.Itoa(value), fmt.Sprintf(`{"name":"ActiveWidgetIndex","type":"IntProperty","value":%d}`, value), []string{"ActiveWidgetIndex", "IntProperty"}, nil
	case "retainerbox-retain-render", "retainerboxretainrender":
		if !strings.EqualFold(target.ClassName, "RetainerBox") {
			return "", "", "", nil, fmt.Errorf("retainerbox-retain-render requires a RetainerBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "retainerbox-retain-render")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "bRetainRender", valueJSON, fmt.Sprintf(`{"name":"bRetainRender","type":"BoolProperty","value":%s}`, valueJSON), []string{"bRetainRender", "BoolProperty"}, nil
	case "retainerbox-render-on-invalidation", "retainerboxrenderoninvalidation":
		if !strings.EqualFold(target.ClassName, "RetainerBox") {
			return "", "", "", nil, fmt.Errorf("retainerbox-render-on-invalidation requires a RetainerBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "retainerbox-render-on-invalidation")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "RenderOnInvalidation", valueJSON, fmt.Sprintf(`{"name":"RenderOnInvalidation","type":"BoolProperty","value":%s}`, valueJSON), []string{"RenderOnInvalidation", "BoolProperty"}, nil
	case "retainerbox-render-on-phase", "retainerboxrenderonphase":
		if !strings.EqualFold(target.ClassName, "RetainerBox") {
			return "", "", "", nil, fmt.Errorf("retainerbox-render-on-phase requires a RetainerBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "retainerbox-render-on-phase")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "RenderOnPhase", valueJSON, fmt.Sprintf(`{"name":"RenderOnPhase","type":"BoolProperty","value":%s}`, valueJSON), []string{"RenderOnPhase", "BoolProperty"}, nil
	case "retainerbox-phase", "retainerboxphase":
		if !strings.EqualFold(target.ClassName, "RetainerBox") {
			return "", "", "", nil, fmt.Errorf("retainerbox-phase requires a RetainerBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetIntValue(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("retainerbox-phase: %w", err)
		}
		return "Phase", strconv.Itoa(value), fmt.Sprintf(`{"name":"Phase","type":"IntProperty","value":%d}`, value), []string{"Phase", "IntProperty"}, nil
	case "retainerbox-phase-count", "retainerboxphasecount":
		if !strings.EqualFold(target.ClassName, "RetainerBox") {
			return "", "", "", nil, fmt.Errorf("retainerbox-phase-count requires a RetainerBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetIntValue(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("retainerbox-phase-count: %w", err)
		}
		return "PhaseCount", strconv.Itoa(value), fmt.Sprintf(`{"name":"PhaseCount","type":"IntProperty","value":%d}`, value), []string{"PhaseCount", "IntProperty"}, nil
	case "backgroundblur-strength", "backgroundblurstrength":
		if !strings.EqualFold(target.ClassName, "BackgroundBlur") {
			return "", "", "", nil, fmt.Errorf("backgroundblur-strength requires a BackgroundBlur widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("backgroundblur-strength: %w", err)
		}
		return "BlurStrength", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"BlurStrength","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"BlurStrength", "FloatProperty"}, nil
	case "backgroundblur-apply-alpha-to-blur", "backgroundblurapplyalphatoblur":
		if !strings.EqualFold(target.ClassName, "BackgroundBlur") {
			return "", "", "", nil, fmt.Errorf("backgroundblur-apply-alpha-to-blur requires a BackgroundBlur widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "backgroundblur-apply-alpha-to-blur")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "bApplyAlphaToBlur", valueJSON, fmt.Sprintf(`{"name":"bApplyAlphaToBlur","type":"BoolProperty","value":%s}`, valueJSON), []string{"bApplyAlphaToBlur", "BoolProperty"}, nil
	case "safezone-pad-left", "safezonepadleft":
		if !strings.EqualFold(target.ClassName, "SafeZone") {
			return "", "", "", nil, fmt.Errorf("safezone-pad-left requires a SafeZone widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "safezone-pad-left")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "PadLeft", valueJSON, fmt.Sprintf(`{"name":"PadLeft","type":"BoolProperty","value":%s}`, valueJSON), []string{"PadLeft", "BoolProperty"}, nil
	case "safezone-pad-right", "safezonepadright":
		if !strings.EqualFold(target.ClassName, "SafeZone") {
			return "", "", "", nil, fmt.Errorf("safezone-pad-right requires a SafeZone widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "safezone-pad-right")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "PadRight", valueJSON, fmt.Sprintf(`{"name":"PadRight","type":"BoolProperty","value":%s}`, valueJSON), []string{"PadRight", "BoolProperty"}, nil
	case "safezone-pad-top", "safezonepadtop":
		if !strings.EqualFold(target.ClassName, "SafeZone") {
			return "", "", "", nil, fmt.Errorf("safezone-pad-top requires a SafeZone widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "safezone-pad-top")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "PadTop", valueJSON, fmt.Sprintf(`{"name":"PadTop","type":"BoolProperty","value":%s}`, valueJSON), []string{"PadTop", "BoolProperty"}, nil
	case "safezone-pad-bottom", "safezonepadbottom":
		if !strings.EqualFold(target.ClassName, "SafeZone") {
			return "", "", "", nil, fmt.Errorf("safezone-pad-bottom requires a SafeZone widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "safezone-pad-bottom")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "PadBottom", valueJSON, fmt.Sprintf(`{"name":"PadBottom","type":"BoolProperty","value":%s}`, valueJSON), []string{"PadBottom", "BoolProperty"}, nil
	case "invalidationbox-can-cache", "invalidationboxcancache":
		if !strings.EqualFold(target.ClassName, "InvalidationBox") {
			return "", "", "", nil, fmt.Errorf("invalidationbox-can-cache requires an InvalidationBox widget, got %s", target.ClassName)
		}
		value, err := parseWidgetBoolValue(rawValue, "invalidationbox-can-cache")
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(value)
		if err != nil {
			return "", "", "", nil, err
		}
		return "bCanCache", valueJSON, fmt.Sprintf(`{"name":"bCanCache","type":"BoolProperty","value":%s}`, valueJSON), []string{"bCanCache", "BoolProperty"}, nil
	case "uniformgridpanel-min-desired-slot-width", "uniformgridpanelmindesiredslotwidth":
		if !strings.EqualFold(target.ClassName, "UniformGridPanel") {
			return "", "", "", nil, fmt.Errorf("uniformgridpanel-min-desired-slot-width requires a UniformGridPanel widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("uniformgridpanel-min-desired-slot-width: %w", err)
		}
		return "MinDesiredSlotWidth", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"MinDesiredSlotWidth","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"MinDesiredSlotWidth", "FloatProperty"}, nil
	case "uniformgridpanel-min-desired-slot-height", "uniformgridpanelmindesiredslotheight":
		if !strings.EqualFold(target.ClassName, "UniformGridPanel") {
			return "", "", "", nil, fmt.Errorf("uniformgridpanel-min-desired-slot-height requires a UniformGridPanel widget, got %s", target.ClassName)
		}
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("uniformgridpanel-min-desired-slot-height: %w", err)
		}
		return "MinDesiredSlotHeight", strconv.FormatFloat(float64(value), 'f', -1, 32), `{"name":"MinDesiredSlotHeight","type":"FloatProperty","value":` + strconv.FormatFloat(float64(value), 'f', -1, 32) + `}`, []string{"MinDesiredSlotHeight", "FloatProperty"}, nil
	case "uniformgridpanel-slot-padding", "uniformgridpanelslotpadding":
		if !strings.EqualFold(target.ClassName, "UniformGridPanel") {
			return "", "", "", nil, fmt.Errorf("uniformgridpanel-slot-padding requires a UniformGridPanel widget, got %s", target.ClassName)
		}
		padding, err := parseWidgetMarginValue(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("uniformgridpanel-slot-padding: %w", err)
		}
		valueJSON, err := buildWidgetMarginValueJSON(padding)
		if err != nil {
			return "", "", "", nil, err
		}
		return "SlotPadding", valueJSON, fmt.Sprintf(`{"name":"SlotPadding","type":"StructProperty(Margin(/Script/SlateCore))","value":%s}`, valueJSON), []string{"SlotPadding", "StructProperty", "Margin", "/Script/SlateCore", "Left", "Top", "Right", "Bottom", "FloatProperty"}, nil
	case "menu-anchor-placement", "menuanchor-placement", "menu-anchor-place", "menuanchor-place":
		if !strings.EqualFold(target.ClassName, "MenuAnchor") {
			return "", "", "", nil, fmt.Errorf("menu-anchor-placement requires a MenuAnchor widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetMenuPlacementValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "EMenuPlacement", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "Placement", valueJSON, fmt.Sprintf(`{"name":"Placement","type":"EnumProperty(EMenuPlacement)","value":%s}`, valueJSON), []string{"Placement", "EnumProperty", "EMenuPlacement", enumValue}, nil
	case "grid-row-fill", "grid-rowfill":
		if !strings.EqualFold(target.ClassName, "GridPanel") {
			return "", "", "", nil, fmt.Errorf("grid-row-fill requires a GridPanel widget, got %s", target.ClassName)
		}
		values, err := parseWidgetFloatListValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(values)
		if err != nil {
			return "", "", "", nil, err
		}
		return "RowFill", valueJSON, fmt.Sprintf(`{"name":"RowFill","type":"ArrayProperty(FloatProperty)","value":%s}`, valueJSON), []string{"RowFill", "ArrayProperty", "FloatProperty"}, nil
	case "grid-column-fill", "grid-columnfill":
		if !strings.EqualFold(target.ClassName, "GridPanel") {
			return "", "", "", nil, fmt.Errorf("grid-column-fill requires a GridPanel widget, got %s", target.ClassName)
		}
		values, err := parseWidgetFloatListValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(values)
		if err != nil {
			return "", "", "", nil, err
		}
		return "ColumnFill", valueJSON, fmt.Sprintf(`{"name":"ColumnFill","type":"ArrayProperty(FloatProperty)","value":%s}`, valueJSON), []string{"ColumnFill", "ArrayProperty", "FloatProperty"}, nil
	case "button-background-color", "button-backgroundcolor":
		if !strings.EqualFold(target.ClassName, "Button") {
			return "", "", "", nil, fmt.Errorf("button-background-color requires a Button widget, got %s", target.ClassName)
		}
		color, err := parseWidgetLinearColorValue(rawValue, normalizedProperty)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := buildWidgetLinearColorValueJSON(color)
		if err != nil {
			return "", "", "", nil, err
		}
		return "BackgroundColor", valueJSON, fmt.Sprintf(`{"name":"BackgroundColor","type":"StructProperty(LinearColor(/Script/CoreUObject))","value":%s}`, valueJSON), []string{"BackgroundColor", "StructProperty", "LinearColor", "/Script/CoreUObject"}, nil
	case "button-color-and-opacity", "button-colorandopacity":
		if !strings.EqualFold(target.ClassName, "Button") {
			return "", "", "", nil, fmt.Errorf("button-color-and-opacity requires a Button widget, got %s", target.ClassName)
		}
		color, err := parseWidgetLinearColorValue(rawValue, normalizedProperty)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := buildWidgetLinearColorValueJSON(color)
		if err != nil {
			return "", "", "", nil, err
		}
		return "ColorAndOpacity", valueJSON, fmt.Sprintf(`{"name":"ColorAndOpacity","type":"StructProperty(LinearColor(/Script/CoreUObject))","value":%s}`, valueJSON), []string{"ColorAndOpacity", "StructProperty", "LinearColor", "/Script/CoreUObject"}, nil
	case "border-padding", "borderpadding":
		if !strings.EqualFold(target.ClassName, "Border") {
			return "", "", "", nil, fmt.Errorf("border-padding requires a Border widget, got %s", target.ClassName)
		}
		padding, err := parseWidgetMarginValue(rawValue)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("border-padding: %w", err)
		}
		valueJSON, err := buildWidgetMarginValueJSON(padding)
		if err != nil {
			return "", "", "", nil, err
		}
		return "Padding", valueJSON, fmt.Sprintf(`{"name":"Padding","type":"StructProperty(Margin(/Script/SlateCore))","value":%s}`, valueJSON), []string{"Padding", "StructProperty", "Margin", "/Script/SlateCore", "Left", "Top", "Right", "Bottom", "FloatProperty"}, nil
	case "border-brush-color", "border-brushcolor":
		if !strings.EqualFold(target.ClassName, "Border") {
			return "", "", "", nil, fmt.Errorf("border-brush-color requires a Border widget, got %s", target.ClassName)
		}
		color, err := parseWidgetLinearColorValue(rawValue, normalizedProperty)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := buildWidgetLinearColorValueJSON(color)
		if err != nil {
			return "", "", "", nil, err
		}
		return "BrushColor", valueJSON, fmt.Sprintf(`{"name":"BrushColor","type":"StructProperty(LinearColor(/Script/CoreUObject))","value":%s}`, valueJSON), []string{"BrushColor", "StructProperty", "LinearColor", "/Script/CoreUObject"}, nil
	case "border-content-color-and-opacity", "border-contentcolorandopacity":
		if !strings.EqualFold(target.ClassName, "Border") {
			return "", "", "", nil, fmt.Errorf("border-content-color-and-opacity requires a Border widget, got %s", target.ClassName)
		}
		color, err := parseWidgetLinearColorValue(rawValue, normalizedProperty)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := buildWidgetLinearColorValueJSON(color)
		if err != nil {
			return "", "", "", nil, err
		}
		return "ContentColorAndOpacity", valueJSON, fmt.Sprintf(`{"name":"ContentColorAndOpacity","type":"StructProperty(LinearColor(/Script/CoreUObject))","value":%s}`, valueJSON), []string{"ContentColorAndOpacity", "StructProperty", "LinearColor", "/Script/CoreUObject"}, nil
	case "border-horizontal-alignment", "border-halign":
		if !strings.EqualFold(target.ClassName, "Border") {
			return "", "", "", nil, fmt.Errorf("border-horizontal-alignment requires a Border widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetHorizontalAlignmentValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "EHorizontalAlignment", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "HorizontalAlignment", valueJSON, fmt.Sprintf(`{"name":"HorizontalAlignment","type":"EnumProperty(EHorizontalAlignment)","value":%s}`, valueJSON), []string{"HorizontalAlignment", "EnumProperty", "EHorizontalAlignment", enumValue}, nil
	case "border-vertical-alignment", "border-valign":
		if !strings.EqualFold(target.ClassName, "Border") {
			return "", "", "", nil, fmt.Errorf("border-vertical-alignment requires a Border widget, got %s", target.ClassName)
		}
		enumValue, err := normalizeWidgetVerticalAlignmentValue(rawValue)
		if err != nil {
			return "", "", "", nil, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "EVerticalAlignment", "value": enumValue})
		if err != nil {
			return "", "", "", nil, err
		}
		return "VerticalAlignment", valueJSON, fmt.Sprintf(`{"name":"VerticalAlignment","type":"EnumProperty(EVerticalAlignment)","value":%s}`, valueJSON), []string{"VerticalAlignment", "EnumProperty", "EVerticalAlignment", enumValue}, nil
	default:
		return "", "", "", nil, fmt.Errorf("unsupported widget-specific property %q", normalizedProperty)
	}
}

func widgetSpecialPropertyAddBeforeProperty(asset *uasset.Asset, exportIndex int, normalizedProperty string) string {
	switch normalizedProperty {
	case "grid-row-fill", "grid-rowfill", "grid-column-fill", "grid-columnfill":
		return "Slots"
	case "uniformgridpanel-min-desired-slot-width", "uniformgridpanelmindesiredslotwidth",
		"uniformgridpanel-min-desired-slot-height", "uniformgridpanelmindesiredslotheight",
		"uniformgridpanel-slot-padding", "uniformgridpanelslotpadding":
		return widgetPropertyAddBeforePropertyForExport(asset, exportIndex, "Slot", "DisplayLabel")
	case "progressbar-percent", "progressbarpercent":
		return widgetPropertyAddBeforePropertyForExport(asset, exportIndex, "FillColorAndOpacity", "Slot", "DisplayLabel")
	case "listview-entry-widget-class", "listviewentrywidgetclass":
		return widgetPropertyAddBeforePropertyForExport(asset, exportIndex, "Slot", "DisplayLabel")
	case "listview-orientation", "listvieworientation",
		"listview-selection-mode", "listviewselectionmode",
		"listview-consume-mouse-wheel", "listviewconsumemousewheel",
		"listview-is-focusable", "listviewisfocusable",
		"listview-return-focus-to-selection", "listviewreturnfocustoselection",
		"listview-clear-scroll-velocity-on-selection", "listviewclearscrollvelocityonselection",
		"listview-scroll-into-view-alignment", "listviewscrollintoviewalignment",
		"listview-horizontal-entry-spacing", "listviewhorizontalentryspacing",
		"listview-vertical-entry-spacing", "listviewverticalentryspacing",
		"listview-scrollbar-padding", "listviewscrollbarpadding",
		"tileview-entry-width", "tileviewentrywidth",
		"tileview-entry-height", "tileviewentryheight",
		"tileview-scrollbar-disabled-visibility", "tileviewscrollbardisabledvisibility",
		"tileview-entry-size-includes-entry-spacing", "tileviewentrysizeincludesentryspacing":
		return widgetPropertyAddBeforePropertyForExport(asset, exportIndex, "EntryWidgetClass", "Slot", "DisplayLabel")
	case "listview-wheel-scroll-multiplier", "listviewwheelscrollmultiplier",
		"listview-enable-scroll-animation", "listviewenablescrollanimation",
		"listview-allow-overscroll", "listviewallowoverscroll",
		"listview-enable-right-click-scrolling", "listviewenablerightclickscrolling",
		"listview-enable-touch-scrolling", "listviewenabletouchscrolling",
		"listview-is-pointer-scrolling-enabled", "listviewispointerscrollingenabled",
		"listview-is-gamepad-scrolling-enabled", "listviewisgamepadscrollingenabled":
		return widgetPropertyAddBeforePropertyForExport(asset, exportIndex, "Slot", "DisplayLabel")
	case "button-background-color", "button-backgroundcolor",
		"button-color-and-opacity", "button-colorandopacity",
		"is-focusable", "isfocusable",
		"button-is-focusable", "buttonisfocusable",
		"checkbox-is-focusable", "checkboxisfocusable",
		"slider-is-focusable", "sliderisfocusable",
		"scrollbox-is-focusable", "scrollboxisfocusable",
		"comboboxstring-is-focusable", "comboboxstringisfocusable",
		"progressbar-fill-color", "progressbar-fillcolor",
		"progressbar-fill-color-and-opacity", "progressbar-fillcolorandopacity",
		"slider-value", "slidervalue",
		"slider-min-value", "sliderminvalue",
		"slider-max-value", "slidermaxvalue",
		"slider-step-size", "sliderstepsize",
		"slider-orientation", "sliderorientation",
		"spacer-size", "spacersize",
		"scrollbar-thickness", "scrollbarthickness",
		"scrollbar-orientation", "scrollbarorientation",
		"checkbox-checked-state", "checkboxcheckedstate",
		"checkbox-is-checked", "checkboxischecked",
		"editabletext-hint-text", "editabletexthinttext",
		"editabletext-is-read-only", "editabletextisreadonly",
		"editabletext-is-password", "editabletextispassword",
		"editabletext-minimum-desired-width", "editabletextminimumdesiredwidth",
		"editabletext-justification", "editabletextjustification",
		"editabletextbox-hint-text", "editabletextboxhinttext",
		"editabletextbox-is-read-only", "editabletextboxisreadonly",
		"editabletextbox-is-password", "editabletextboxispassword",
		"editabletextbox-minimum-desired-width", "editabletextboxminimumdesiredwidth",
		"editabletextbox-justification", "editabletextboxjustification",
		"multilineeditabletextbox-hint-text", "multilineeditabletextboxhinttext",
		"multilineeditabletextbox-is-read-only", "multilineeditabletextboxisreadonly",
		"multilineeditabletextbox-justification", "multilineeditabletextboxjustification",
		"spinbox-value", "spinboxvalue",
		"spinbox-min-value", "spinboxminvalue",
		"spinbox-max-value", "spinboxmaxvalue",
		"spinbox-delta", "spinboxdelta",
		"comboboxstring-selected-option", "comboboxstringselectedoption",
		"comboboxstring-options", "comboboxstringoptions",
		"scrollbox-orientation", "scrollboxorientation",
		"scrollbox-scrollbar-visibility", "scrollboxscrollbarvisibility",
		"scrollbox-scroll-bar-visibility",
		"scrollbox-consume-mouse-wheel", "scrollboxconsumemousewheel",
		"sizebox-width-override", "sizeboxwidthoverride",
		"sizebox-width", "sizeboxwidth",
		"sizebox-height-override", "sizeboxheightoverride",
		"sizebox-height", "sizeboxheight",
		"sizebox-min-desired-width", "sizeboxmindesiredwidth",
		"sizebox-min-desired-height", "sizeboxmindesiredheight",
		"sizebox-max-desired-width", "sizeboxmaxdesiredwidth",
		"sizebox-max-desired-height", "sizeboxmaxdesiredheight",
		"sizebox-min-aspect-ratio", "sizeboxminaspectratio",
		"sizebox-max-aspect-ratio", "sizeboxmaxaspectratio",
		"scalebox-stretch", "scaleboxstretch",
		"scalebox-stretch-direction", "scaleboxstretchdirection",
		"scalebox-user-specified-scale", "scaleboxuserspecifiedscale",
		"scalebox-ignore-inherited-scale", "scaleboxignoreinheritedscale",
		"wrapbox-wrap-size", "wrapboxwrapsize",
		"wrapbox-explicit-wrap-size", "wrapboxexplicitwrapsize",
		"wrapbox-inner-slot-padding", "wrapboxinnerslotpadding",
		"wrapbox-orientation", "wrapboxorientation",
		"widgetswitcher-active-widget-index", "widgetswitcheractivewidgetindex",
		"retainerbox-retain-render", "retainerboxretainrender",
		"retainerbox-render-on-invalidation", "retainerboxrenderoninvalidation",
		"retainerbox-render-on-phase", "retainerboxrenderonphase",
		"retainerbox-phase", "retainerboxphase",
		"retainerbox-phase-count", "retainerboxphasecount",
		"backgroundblur-strength", "backgroundblurstrength",
		"backgroundblur-apply-alpha-to-blur", "backgroundblurapplyalphatoblur",
		"safezone-pad-left", "safezonepadleft",
		"safezone-pad-right", "safezonepadright",
		"safezone-pad-top", "safezonepadtop",
		"safezone-pad-bottom", "safezonepadbottom",
		"invalidationbox-can-cache", "invalidationboxcancache",
		"border-padding", "borderpadding",
		"border-brush-color", "border-brushcolor",
		"border-content-color-and-opacity", "border-contentcolorandopacity",
		"border-horizontal-alignment", "border-halign",
		"border-vertical-alignment", "border-valign":
		return widgetPropertyAddBeforePropertyForExport(asset, exportIndex, "Slot", "DisplayLabel")
	default:
		return ""
	}
}

func normalizeWidgetVisibilityValue(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("visibility requires --value")
	}
	trimmed = strings.TrimPrefix(trimmed, "ESlateVisibility::")
	switch strings.ToLower(trimmed) {
	case "visible":
		return "ESlateVisibility::Visible", nil
	case "collapsed":
		return "ESlateVisibility::Collapsed", nil
	case "hidden":
		return "ESlateVisibility::Hidden", nil
	case "hittestinvisible":
		return "ESlateVisibility::HitTestInvisible", nil
	case "selfhittestinvisible":
		return "ESlateVisibility::SelfHitTestInvisible", nil
	default:
		return "", fmt.Errorf("unsupported visibility %q (supported: Visible, Collapsed, Hidden, HitTestInvisible, SelfHitTestInvisible)", value)
	}
}

func normalizeWidgetOrientationValue(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("orientation requires --value")
	}
	trimmed = strings.TrimPrefix(trimmed, "EOrientation::")
	switch strings.ToLower(trimmed) {
	case "horizontal", "orient_horizontal":
		return "Orient_Horizontal", nil
	case "vertical", "orient_vertical":
		return "Orient_Vertical", nil
	default:
		return "", fmt.Errorf("unsupported orientation %q (supported: Horizontal, Vertical)", value)
	}
}

func normalizeWidgetScrollIntoViewAlignmentValue(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("listview-scroll-into-view-alignment requires --value")
	}
	trimmed = strings.TrimPrefix(trimmed, "EScrollIntoViewAlignment::")
	switch strings.ToLower(trimmed) {
	case "intoview":
		return "EScrollIntoViewAlignment::IntoView", nil
	case "toporleft":
		return "EScrollIntoViewAlignment::TopOrLeft", nil
	case "centeraligned":
		return "EScrollIntoViewAlignment::CenterAligned", nil
	case "bottomorright":
		return "EScrollIntoViewAlignment::BottomOrRight", nil
	default:
		return "", fmt.Errorf("unsupported listview-scroll-into-view-alignment %q (supported: IntoView, TopOrLeft, CenterAligned, BottomOrRight)", value)
	}
}

func normalizeWidgetCheckBoxStateValue(value string) (string, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "ECheckBoxState::"))
	switch strings.ToLower(trimmed) {
	case "unchecked":
		return "ECheckBoxState::Unchecked", nil
	case "checked":
		return "ECheckBoxState::Checked", nil
	case "undetermined":
		return "ECheckBoxState::Undetermined", nil
	default:
		return "", fmt.Errorf("unsupported checkbox state %q (supported: Unchecked, Checked, Undetermined)", value)
	}
}

func normalizeWidgetConsumeMouseWheelValue(value string) (string, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "EConsumeMouseWheel::"))
	switch strings.ToLower(trimmed) {
	case "when_scrolling_possible", "whenscrollingpossible":
		return "EConsumeMouseWheel::WhenScrollingPossible", nil
	case "always":
		return "EConsumeMouseWheel::Always", nil
	case "never":
		return "EConsumeMouseWheel::Never", nil
	default:
		return "", fmt.Errorf("unsupported consume mouse wheel %q (supported: WhenScrollingPossible, Always, Never)", value)
	}
}

func normalizeWidgetSelectionModeValue(value string) (string, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "ESelectionMode::"))
	switch strings.ToLower(trimmed) {
	case "none":
		return "ESelectionMode::None", nil
	case "single":
		return "ESelectionMode::Single", nil
	case "singletoggle":
		return "ESelectionMode::SingleToggle", nil
	case "multi":
		return "ESelectionMode::Multi", nil
	default:
		return "", fmt.Errorf("unsupported selection mode %q (supported: None, Single, SingleToggle, Multi)", value)
	}
}

func normalizeWidgetIsFocusableProperty(normalizedProperty string) (string, string, bool) {
	switch normalizedProperty {
	case "is-focusable", "isfocusable":
		return "is-focusable", "", true
	case "button-is-focusable", "buttonisfocusable":
		return "button-is-focusable", "Button", true
	case "checkbox-is-focusable", "checkboxisfocusable":
		return "checkbox-is-focusable", "CheckBox", true
	case "slider-is-focusable", "sliderisfocusable":
		return "slider-is-focusable", "Slider", true
	case "scrollbox-is-focusable", "scrollboxisfocusable":
		return "scrollbox-is-focusable", "ScrollBox", true
	case "comboboxstring-is-focusable", "comboboxstringisfocusable":
		return "comboboxstring-is-focusable", "ComboBoxString", true
	default:
		return "", "", false
	}
}

func widgetFocusablePropertyPath(className string) (string, []string, bool) {
	switch {
	case strings.EqualFold(className, "Button"),
		strings.EqualFold(className, "CheckBox"),
		strings.EqualFold(className, "Slider"):
		return "IsFocusable", []string{"IsFocusable", "BoolProperty"}, true
	case strings.EqualFold(className, "ScrollBox"),
		strings.EqualFold(className, "ComboBoxString"):
		return "bIsFocusable", []string{"bIsFocusable", "BoolProperty"}, true
	default:
		return "", nil, false
	}
}

func buildWidgetIsFocusableMutation(target widgetWriteTarget, normalizedProperty, rawValue string) (string, string, string, []string, error) {
	canonicalProperty, requiredClass, ok := normalizeWidgetIsFocusableProperty(normalizedProperty)
	if !ok {
		return "", "", "", nil, fmt.Errorf("unsupported is-focusable property %q", normalizedProperty)
	}
	if requiredClass != "" && !strings.EqualFold(target.ClassName, requiredClass) {
		return "", "", "", nil, fmt.Errorf("%s requires a %s widget, got %s", canonicalProperty, requiredClass, target.ClassName)
	}

	propertyPath, requiredNames, supported := widgetFocusablePropertyPath(target.ClassName)
	if !supported {
		return "", "", "", nil, fmt.Errorf("%s requires a widget class with an exposed IsFocusable property, got %s", canonicalProperty, target.ClassName)
	}

	value, err := parseWidgetBoolValue(rawValue, canonicalProperty)
	if err != nil {
		return "", "", "", nil, err
	}
	valueJSON, err := marshalJSONValue(value)
	if err != nil {
		return "", "", "", nil, err
	}
	return propertyPath, valueJSON, fmt.Sprintf(`{"name":"%s","type":"BoolProperty","value":%s}`, propertyPath, valueJSON), requiredNames, nil
}

func widgetClassSupportsListViewProperties(className string) bool {
	switch strings.ToLower(strings.TrimSpace(className)) {
	case "listview", "tileview", "treeview":
		return true
	default:
		return false
	}
}

func widgetClassSupportsListViewSpacing(className string) bool {
	return widgetClassSupportsListViewProperties(className)
}

func isListViewEntryWidgetClassProperty(normalizedProperty string) bool {
	switch normalizedProperty {
	case "listview-entry-widget-class", "listviewentrywidgetclass":
		return true
	default:
		return false
	}
}

func normalizeWidgetScaleBoxStretchValue(value string) (string, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "EStretch::"))
	switch strings.ToLower(trimmed) {
	case "none":
		return "EStretch::None", nil
	case "fill":
		return "EStretch::Fill", nil
	case "scaletofit":
		return "EStretch::ScaleToFit", nil
	case "scaletofitx":
		return "EStretch::ScaleToFitX", nil
	case "scaletofity":
		return "EStretch::ScaleToFitY", nil
	case "scaletofill":
		return "EStretch::ScaleToFill", nil
	case "scalebysafezone":
		return "EStretch::ScaleBySafeZone", nil
	case "userspecified":
		return "EStretch::UserSpecified", nil
	default:
		return "", fmt.Errorf("unsupported scalebox-stretch %q (supported: None, Fill, ScaleToFit, ScaleToFitX, ScaleToFitY, ScaleToFill, ScaleBySafeZone, UserSpecified)", value)
	}
}

func normalizeWidgetScaleBoxStretchDirectionValue(value string) (string, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "EStretchDirection::"))
	switch strings.ToLower(trimmed) {
	case "both":
		return "EStretchDirection::Both", nil
	case "downonly":
		return "EStretchDirection::DownOnly", nil
	case "uponly":
		return "EStretchDirection::UpOnly", nil
	default:
		return "", fmt.Errorf("unsupported scalebox-stretch-direction %q (supported: Both, DownOnly, UpOnly)", value)
	}
}

func normalizeWidgetMenuPlacementValue(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("menu-anchor-placement requires --value")
	}
	trimmed = strings.TrimPrefix(trimmed, "EMenuPlacement::")
	if strings.HasPrefix(trimmed, "MenuPlacement_") {
		return "EMenuPlacement::" + trimmed, nil
	}
	switch strings.ToLower(trimmed) {
	case "belowanchor":
		return "EMenuPlacement::MenuPlacement_BelowAnchor", nil
	case "centeredbelowanchor":
		return "EMenuPlacement::MenuPlacement_CenteredBelowAnchor", nil
	case "belowrightanchor":
		return "EMenuPlacement::MenuPlacement_BelowRightAnchor", nil
	case "combobox":
		return "EMenuPlacement::MenuPlacement_ComboBox", nil
	case "comboboxright":
		return "EMenuPlacement::MenuPlacement_ComboBoxRight", nil
	case "menuright":
		return "EMenuPlacement::MenuPlacement_MenuRight", nil
	case "centeredaboveanchor":
		return "EMenuPlacement::MenuPlacement_CenteredAboveAnchor", nil
	case "aboveanchor":
		return "EMenuPlacement::MenuPlacement_AboveAnchor", nil
	case "aboverightanchor":
		return "EMenuPlacement::MenuPlacement_AboveRightAnchor", nil
	default:
		return "", fmt.Errorf("unsupported menu-anchor-placement %q (supported: BelowAnchor, CenteredBelowAnchor, BelowRightAnchor, ComboBox, ComboBoxRight, MenuRight, CenteredAboveAnchor, AboveAnchor, AboveRightAnchor)", value)
	}
}

type widgetAnchorLayoutData struct {
	MinX   float32
	MinY   float32
	MaxX   float32
	MaxY   float32
	Left   float32
	Top    float32
	Right  float32
	Bottom float32
	AlignX float32
	AlignY float32
}

type widgetMarginData struct {
	Left   float32
	Top    float32
	Right  float32
	Bottom float32
}

type widgetSlateChildSize struct {
	Rule  string
	Value float32
}

type widgetLinearColor struct {
	R float32
	G float32
	B float32
	A float32
}

type widgetButtonBrushProperty struct {
	CanonicalProperty string
	StateField        string
	BrushField        string
}

func isWidgetSlotLayoutProperty(normalizedProperty string) bool {
	switch normalizedProperty {
	case "slot-padding", "slot-size", "slot-horizontal-alignment", "slot-halign", "slot-vertical-alignment", "slot-valign",
		"slot-row", "slot-column", "slot-row-span", "slot-column-span", "slot-layer", "slot-nudge",
		"layout-position", "layout-size", "layout-anchors", "layout-alignment", "layout-data":
		return true
	default:
		return false
	}
}

func isWidgetSpecialProperty(normalizedProperty string) bool {
	switch normalizedProperty {
	case "menu-anchor-placement", "menuanchor-placement", "menu-anchor-place", "menuanchor-place",
		"grid-row-fill", "grid-rowfill", "grid-column-fill", "grid-columnfill",
		"progressbar-percent", "progressbarpercent",
		"progressbar-fill-color", "progressbar-fillcolor",
		"progressbar-fill-color-and-opacity", "progressbar-fillcolorandopacity",
		"slider-value", "slidervalue",
		"slider-min-value", "sliderminvalue",
		"slider-max-value", "slidermaxvalue",
		"slider-step-size", "sliderstepsize",
		"slider-orientation", "sliderorientation",
		"spacer-size", "spacersize",
		"scrollbar-thickness", "scrollbarthickness",
		"scrollbar-orientation", "scrollbarorientation",
		"checkbox-checked-state", "checkboxcheckedstate",
		"checkbox-is-checked", "checkboxischecked",
		"editabletext-hint-text", "editabletexthinttext",
		"editabletext-is-read-only", "editabletextisreadonly",
		"editabletext-is-password", "editabletextispassword",
		"editabletext-minimum-desired-width", "editabletextminimumdesiredwidth",
		"editabletext-justification", "editabletextjustification",
		"editabletextbox-hint-text", "editabletextboxhinttext",
		"editabletextbox-is-read-only", "editabletextboxisreadonly",
		"editabletextbox-is-password", "editabletextboxispassword",
		"editabletextbox-minimum-desired-width", "editabletextboxminimumdesiredwidth",
		"editabletextbox-justification", "editabletextboxjustification",
		"multilineeditabletextbox-hint-text", "multilineeditabletextboxhinttext",
		"multilineeditabletextbox-is-read-only", "multilineeditabletextboxisreadonly",
		"multilineeditabletextbox-justification", "multilineeditabletextboxjustification",
		"spinbox-value", "spinboxvalue",
		"spinbox-min-value", "spinboxminvalue",
		"spinbox-max-value", "spinboxmaxvalue",
		"spinbox-delta", "spinboxdelta",
		"comboboxstring-selected-option", "comboboxstringselectedoption",
		"comboboxstring-options", "comboboxstringoptions",
		"listview-entry-widget-class", "listviewentrywidgetclass",
		"listview-orientation", "listvieworientation",
		"listview-selection-mode", "listviewselectionmode",
		"listview-consume-mouse-wheel", "listviewconsumemousewheel",
		"listview-is-focusable", "listviewisfocusable",
		"listview-return-focus-to-selection", "listviewreturnfocustoselection",
		"listview-clear-scroll-velocity-on-selection", "listviewclearscrollvelocityonselection",
		"listview-scroll-into-view-alignment", "listviewscrollintoviewalignment",
		"listview-wheel-scroll-multiplier", "listviewwheelscrollmultiplier",
		"listview-enable-scroll-animation", "listviewenablescrollanimation",
		"listview-allow-overscroll", "listviewallowoverscroll",
		"listview-enable-right-click-scrolling", "listviewenablerightclickscrolling",
		"listview-enable-touch-scrolling", "listviewenabletouchscrolling",
		"listview-is-pointer-scrolling-enabled", "listviewispointerscrollingenabled",
		"listview-is-gamepad-scrolling-enabled", "listviewisgamepadscrollingenabled",
		"listview-horizontal-entry-spacing", "listviewhorizontalentryspacing",
		"listview-vertical-entry-spacing", "listviewverticalentryspacing",
		"listview-scrollbar-padding", "listviewscrollbarpadding",
		"tileview-entry-width", "tileviewentrywidth",
		"tileview-entry-height", "tileviewentryheight",
		"tileview-scrollbar-disabled-visibility", "tileviewscrollbardisabledvisibility",
		"tileview-entry-size-includes-entry-spacing", "tileviewentrysizeincludesentryspacing",
		"scrollbox-orientation", "scrollboxorientation",
		"scrollbox-scrollbar-visibility", "scrollboxscrollbarvisibility", "scrollbox-scroll-bar-visibility",
		"scrollbox-consume-mouse-wheel", "scrollboxconsumemousewheel",
		"sizebox-width-override", "sizeboxwidthoverride", "sizebox-width", "sizeboxwidth",
		"sizebox-height-override", "sizeboxheightoverride", "sizebox-height", "sizeboxheight",
		"sizebox-min-desired-width", "sizeboxmindesiredwidth",
		"sizebox-min-desired-height", "sizeboxmindesiredheight",
		"sizebox-max-desired-width", "sizeboxmaxdesiredwidth",
		"sizebox-max-desired-height", "sizeboxmaxdesiredheight",
		"sizebox-min-aspect-ratio", "sizeboxminaspectratio",
		"sizebox-max-aspect-ratio", "sizeboxmaxaspectratio",
		"scalebox-stretch", "scaleboxstretch",
		"scalebox-stretch-direction", "scaleboxstretchdirection",
		"scalebox-user-specified-scale", "scaleboxuserspecifiedscale",
		"scalebox-ignore-inherited-scale", "scaleboxignoreinheritedscale",
		"wrapbox-wrap-size", "wrapboxwrapsize",
		"wrapbox-explicit-wrap-size", "wrapboxexplicitwrapsize",
		"wrapbox-inner-slot-padding", "wrapboxinnerslotpadding",
		"wrapbox-orientation", "wrapboxorientation",
		"widgetswitcher-active-widget-index", "widgetswitcheractivewidgetindex",
		"retainerbox-retain-render", "retainerboxretainrender",
		"retainerbox-render-on-invalidation", "retainerboxrenderoninvalidation",
		"retainerbox-render-on-phase", "retainerboxrenderonphase",
		"retainerbox-phase", "retainerboxphase",
		"retainerbox-phase-count", "retainerboxphasecount",
		"backgroundblur-strength", "backgroundblurstrength",
		"backgroundblur-apply-alpha-to-blur", "backgroundblurapplyalphatoblur",
		"safezone-pad-left", "safezonepadleft",
		"safezone-pad-right", "safezonepadright",
		"safezone-pad-top", "safezonepadtop",
		"safezone-pad-bottom", "safezonepadbottom",
		"invalidationbox-can-cache", "invalidationboxcancache",
		"uniformgridpanel-min-desired-slot-width", "uniformgridpanelmindesiredslotwidth",
		"uniformgridpanel-min-desired-slot-height", "uniformgridpanelmindesiredslotheight",
		"uniformgridpanel-slot-padding", "uniformgridpanelslotpadding",
		"is-focusable", "isfocusable",
		"button-is-focusable", "buttonisfocusable",
		"checkbox-is-focusable", "checkboxisfocusable",
		"slider-is-focusable", "sliderisfocusable",
		"scrollbox-is-focusable", "scrollboxisfocusable",
		"comboboxstring-is-focusable", "comboboxstringisfocusable",
		"button-background-color", "button-backgroundcolor", "button-color-and-opacity", "button-colorandopacity",
		"border-padding", "borderpadding", "border-brush-color", "border-brushcolor",
		"border-content-color-and-opacity", "border-contentcolorandopacity",
		"border-horizontal-alignment", "border-halign", "border-vertical-alignment", "border-valign":
		return true
	default:
		return false
	}
}

func isRichTextStyleProperty(normalizedProperty string) bool {
	switch normalizedProperty {
	case "richtext-style-set", "richtext-styleset", "richtext-text-style-set", "richtext-textstyleset",
		"richtext-decorator-classes", "richtext-decoratorclasses", "richtext-decorators",
		"richtext-override-default-style", "richtext-overridedefaultstyle",
		"richtext-default-font", "richtext-default-fontfamily", "richtext-default-font-family", "richtext-default-fontobject",
		"richtext-default-typeface", "richtext-default-font-face",
		"richtext-default-font-size", "richtext-default-fontsize",
		"richtext-default-color-and-opacity", "richtext-default-colorandopacity",
		"richtext-default-shadow-offset", "richtext-default-shadowoffset",
		"richtext-default-shadow-color-and-opacity", "richtext-default-shadow-colorandopacity",
		"richtext-default-outline-size", "richtext-default-outlinesize",
		"richtext-default-outline-color", "richtext-default-outlinecolor",
		"richtext-auto-wrap-text", "richtextautowraptext",
		"richtext-wrap-text-at", "richtextwraptextat",
		"richtext-line-height-percentage", "richtextlineheightpercentage",
		"richtext-justification", "richtext-justify":
		return true
	default:
		return false
	}
}

func isTextBlockStyleProperty(normalizedProperty string) bool {
	switch normalizedProperty {
	case "text-color-and-opacity", "text-colorandopacity", "text-color", "textcolor",
		"text-font", "textfont", "text-font-family", "textfontfamily", "text-font-object", "textfontobject",
		"text-typeface", "texttypeface", "text-font-face", "textfontface",
		"text-font-size", "text-fontsize",
		"text-justification", "text-justify",
		"text-auto-wrap-text", "textautowraptext",
		"text-wrap-text-at", "textwraptextat",
		"text-line-height-percentage", "textlineheightpercentage",
		"text-shadow-offset", "textshadowoffset",
		"text-shadow-color-and-opacity", "textshadowcolorandopacity",
		"text-outline-size", "textoutlinesize",
		"text-outline-color", "textoutlinecolor":
		return true
	default:
		return false
	}
}

func applyTextBlockStyleWrite(asset *uasset.Asset, opts uasset.ParseOptions, target widgetWriteTarget, normalizedProperty, rawValue string) ([]byte, *uasset.Asset, []string, []map[string]any, string, error) {
	workingBytes := append([]byte(nil), asset.Raw.Bytes...)
	workingAsset := asset
	addedNames := []string{}
	updates := make([]map[string]any, 0, len(target.Exports))
	propertyPath := ""
	fontImportIndex := int32(0)
	needsDependsSync := false
	var err error

	if isTextBlockFontObjectProperty(normalizedProperty) {
		packagePath, objectName, ok := parseAssetPackagePath(strings.TrimSpace(rawValue))
		if !ok {
			return nil, nil, nil, nil, "", fmt.Errorf("%s requires a full asset package path like /Game/UI/Foundation/Fonts/NotoSans", normalizedProperty)
		}
		var importAdded []string
		var err error
		workingBytes, workingAsset, importAdded, err = appendFontImportIfMissing(workingAsset, opts, packagePath, objectName)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("append text font import: %w", err)
		}
		addedNames = append(addedNames, importAdded...)
		importIndex, found := findFontImportByPath(workingAsset, packagePath)
		if !found {
			importIndex, found = findObjectImportByObjectName(workingAsset, "/Script/Engine", "Font", objectName)
		}
		if !found {
			return nil, nil, nil, nil, "", fmt.Errorf("font import %q not found after insertion", packagePath)
		}
		fontImportIndex = int32(-importIndex)
		needsDependsSync = true
	}

	for _, exportIdx := range target.Exports {
		mutation, err := buildTextBlockStyleMutation(workingAsset, exportIdx, target.ClassName, normalizedProperty, rawValue, fontImportIndex)
		if err != nil {
			return nil, nil, nil, nil, "", err
		}
		propertyPath = mutation.PropertyPath
		if mutation.NoOp {
			continue
		}

		var added []string
		if len(mutation.RequiredNames) > 0 {
			_, workingAsset, added, err = ensureNameEntriesPresentSorted(workingAsset, opts, mutation.RequiredNames)
			if err != nil {
				return nil, nil, nil, nil, "", fmt.Errorf("ensure textblock style names: %w", err)
			}
			addedNames = append(addedNames, added...)
		}

		outBytes, result, writeErr := applyPropertySetCommandWithPostprocess(workingAsset, exportIdx, mutation.PropertyPath, mutation.ValueJSON, opts)
		if writeErr != nil && strings.Contains(writeErr.Error(), "property not found") && mutation.AddSpecJSON != "" {
			outBytes, result, writeErr = applyPropertyAddAsSetResultBefore(workingAsset, exportIdx, mutation.AddSpecJSON, widgetPropertyAddBeforePropertyForExport(workingAsset, exportIdx, "Slot", "DisplayLabel"), opts)
		}
		if writeErr != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("widget export %d %s: %w", exportIdx+1, mutation.PropertyPath, writeErr)
		}

		workingBytes = outBytes
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("reparse rewritten asset: %w", err)
		}
		updates = append(updates, map[string]any{
			"export":   exportIdx + 1,
			"path":     mutation.PropertyPath,
			"oldValue": result.OldValue,
			"newValue": result.NewValue,
		})
	}

	if needsDependsSync && fontImportIndex != 0 {
		designerIdx := selectBrushDependsExport(workingAsset, target)
		if designerIdx >= 0 {
			workingBytes, workingAsset, err = syncWidgetImportDependency(workingAsset, opts, designerIdx, fontImportIndex)
			if err != nil {
				return nil, nil, nil, nil, "", fmt.Errorf("sync text font import depends: %w", err)
			}
		}
	}

	return workingBytes, workingAsset, slices.Compact(addedNames), updates, propertyPath, nil
}

func buildTextBlockStyleMutation(asset *uasset.Asset, exportIndex int, className, normalizedProperty, rawValue string, fontImportIndex int32) (textBlockStyleMutation, error) {
	if !strings.EqualFold(className, "TextBlock") {
		return textBlockStyleMutation{}, fmt.Errorf("%s requires a TextBlock widget, got %s", normalizedProperty, className)
	}

	switch normalizedProperty {
	case "text-color-and-opacity", "text-colorandopacity", "text-color", "textcolor":
		color, err := parseWidgetLinearColorValue(rawValue, normalizedProperty)
		if err != nil {
			return textBlockStyleMutation{}, err
		}
		field := buildSpecifiedSlateColorFieldFromExisting(nil, color)
		return buildTextBlockStyleFieldMutation("ColorAndOpacity", field, []string{
			"ColorAndOpacity", "StructProperty", "SlateColor", "/Script/SlateCore", "SpecifiedColor", "LinearColor", "/Script/CoreUObject",
		})
	case "text-font", "textfont", "text-font-family", "textfontfamily", "text-font-object", "textfontobject":
		if fontImportIndex == 0 {
			return textBlockStyleMutation{}, fmt.Errorf("%s requires a resolved font import", normalizedProperty)
		}
		current, err := decodeExportRootPropertyValue(asset, exportIndex, "Font")
		if err != nil {
			current = nil
		}
		field := buildSlateFontInfoStructFieldFromExisting(current)
		value, _ := field["value"].(map[string]any)
		if value == nil {
			value = map[string]any{}
		}
		value = cloneAnyMapLocal(value)
		fields, _ := value["value"].(map[string]any)
		if fields == nil {
			fields = map[string]any{}
		}
		fields = cloneAnyMapLocal(fields)
		fields["FontObject"] = buildObjectPropertyField(fontImportIndex)
		value["value"] = fields
		field["value"] = value
		return buildTextBlockStyleFieldMutation("Font", field, []string{
			"Font", "StructProperty", "SlateFontInfo", "/Script/SlateCore", "FontObject", "ObjectProperty",
		})
	case "text-typeface", "texttypeface", "text-font-face", "textfontface":
		trimmed := strings.TrimSpace(rawValue)
		if trimmed == "" {
			return textBlockStyleMutation{}, fmt.Errorf("text-typeface requires --value")
		}
		current, err := decodeExportRootPropertyValue(asset, exportIndex, "Font")
		if err != nil {
			return textBlockStyleMutation{PropertyPath: "Font", NoOp: true}, nil
		}
		field := buildSlateFontInfoStructFieldFromExisting(current)
		value, _ := field["value"].(map[string]any)
		if value == nil {
			value = map[string]any{}
		}
		value = cloneAnyMapLocal(value)
		fields, _ := value["value"].(map[string]any)
		if fields == nil {
			fields = map[string]any{}
		}
		fields = cloneAnyMapLocal(fields)
		fields["TypefaceFontName"] = buildNamePropertyField(trimmed)
		value["value"] = fields
		field["value"] = value
		return buildTextBlockStyleFieldMutation("Font", field, []string{
			"Font", "StructProperty", "SlateFontInfo", "/Script/SlateCore", "TypefaceFontName", "NameProperty", trimmed,
		})
	case "text-font-size", "text-fontsize":
		size, err := parseWidgetIntValue(rawValue)
		if err != nil {
			return textBlockStyleMutation{}, fmt.Errorf("text-font-size: %w", err)
		}
		current, err := decodeExportRootPropertyValue(asset, exportIndex, "Font")
		if err != nil {
			current = nil
		}
		field := buildSlateFontInfoStructFieldFromExisting(current)
		value, _ := field["value"].(map[string]any)
		if value == nil {
			value = map[string]any{}
		}
		value = cloneAnyMapLocal(value)
		fields, _ := value["value"].(map[string]any)
		if fields == nil {
			fields = map[string]any{}
		}
		fields = cloneAnyMapLocal(fields)
		fields["Size"] = map[string]any{
			"type":  "IntProperty",
			"value": size,
		}
		value["value"] = fields
		field["value"] = value
		return buildTextBlockStyleFieldMutation("Font", field, []string{
			"Font", "StructProperty", "SlateFontInfo", "/Script/SlateCore", "Size", "IntProperty", "FloatProperty",
		})
	case "text-auto-wrap-text", "textautowraptext":
		value, err := parseWidgetBoolValue(rawValue, "text-auto-wrap-text")
		if err != nil {
			return textBlockStyleMutation{}, err
		}
		valueJSON := "false"
		if value {
			valueJSON = "true"
		}
		return textBlockStyleMutation{
			PropertyPath:  "AutoWrapText",
			ValueJSON:     valueJSON,
			AddSpecJSON:   fmt.Sprintf(`{"name":"AutoWrapText","type":"BoolProperty","value":%s}`, valueJSON),
			RequiredNames: []string{"AutoWrapText", "BoolProperty"},
		}, nil
	case "text-wrap-text-at", "textwraptextat":
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return textBlockStyleMutation{}, fmt.Errorf("text-wrap-text-at: %w", err)
		}
		scalar := strconv.FormatFloat(float64(value), 'f', -1, 32)
		return textBlockStyleMutation{
			PropertyPath:  "WrapTextAt",
			ValueJSON:     scalar,
			AddSpecJSON:   `{"name":"WrapTextAt","type":"FloatProperty","value":` + scalar + `}`,
			RequiredNames: []string{"WrapTextAt", "FloatProperty"},
		}, nil
	case "text-line-height-percentage", "textlineheightpercentage":
		value, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return textBlockStyleMutation{}, fmt.Errorf("text-line-height-percentage: %w", err)
		}
		scalar := strconv.FormatFloat(float64(value), 'f', -1, 32)
		return textBlockStyleMutation{
			PropertyPath:  "LineHeightPercentage",
			ValueJSON:     scalar,
			AddSpecJSON:   `{"name":"LineHeightPercentage","type":"FloatProperty","value":` + scalar + `}`,
			RequiredNames: []string{"LineHeightPercentage", "FloatProperty"},
		}, nil
	case "text-shadow-offset", "textshadowoffset":
		x, y, err := parseWidgetFloatPair(rawValue)
		if err != nil {
			return textBlockStyleMutation{}, fmt.Errorf("text-shadow-offset: %w", err)
		}
		field := buildCoreVector2DStructFieldFromExisting(nil, x, y, 8)
		return buildTextBlockStyleFieldMutation("ShadowOffset", field, []string{
			"ShadowOffset", "StructProperty", "Vector2D", "/Script/CoreUObject",
		})
	case "text-shadow-color-and-opacity", "textshadowcolorandopacity":
		color, err := parseWidgetLinearColorValue(rawValue, normalizedProperty)
		if err != nil {
			return textBlockStyleMutation{}, err
		}
		field := buildWidgetLinearColorStructFieldWithFlagsFromExisting(nil, color, 8)
		return buildTextBlockStyleFieldMutation("ShadowColorAndOpacity", field, []string{
			"ShadowColorAndOpacity", "StructProperty", "LinearColor", "/Script/CoreUObject",
		})
	case "text-outline-size", "textoutlinesize":
		size, err := parseWidgetIntValue(rawValue)
		if err != nil {
			return textBlockStyleMutation{}, fmt.Errorf("text-outline-size: %w", err)
		}
		current, err := decodeExportRootPropertyValue(asset, exportIndex, "Font")
		if err != nil {
			current = nil
		}
		field := buildSlateFontInfoStructFieldFromExisting(current)
		value, _ := field["value"].(map[string]any)
		if value == nil {
			value = map[string]any{}
		}
		value = cloneAnyMapLocal(value)
		fields, _ := value["value"].(map[string]any)
		if fields == nil {
			fields = map[string]any{}
		}
		fields = cloneAnyMapLocal(fields)
		outlineField := buildFontOutlineSettingsStructFieldFromExisting(fields["OutlineSettings"])
		outlineValue, _ := outlineField["value"].(map[string]any)
		if outlineValue == nil {
			outlineValue = map[string]any{}
		}
		outlineValue = cloneAnyMapLocal(outlineValue)
		outlineFields, _ := outlineValue["value"].(map[string]any)
		if outlineFields == nil {
			outlineFields = map[string]any{}
		}
		outlineFields = cloneAnyMapLocal(outlineFields)
		outlineFields["OutlineSize"] = map[string]any{"type": "IntProperty", "value": size}
		outlineValue["value"] = outlineFields
		outlineField["value"] = outlineValue
		outlineField["flags"] = int32(8)
		fields["OutlineSettings"] = outlineField
		value["value"] = fields
		field["value"] = value
		return buildTextBlockStyleFieldMutation("Font", field, []string{
			"Font", "StructProperty", "SlateFontInfo", "/Script/SlateCore", "OutlineSettings", "FontOutlineSettings", "OutlineSize", "IntProperty",
		})
	case "text-outline-color", "textoutlinecolor":
		color, err := parseWidgetLinearColorValue(rawValue, normalizedProperty)
		if err != nil {
			return textBlockStyleMutation{}, err
		}
		current, err := decodeExportRootPropertyValue(asset, exportIndex, "Font")
		if err != nil {
			current = nil
		}
		field := buildSlateFontInfoStructFieldFromExisting(current)
		value, _ := field["value"].(map[string]any)
		if value == nil {
			value = map[string]any{}
		}
		value = cloneAnyMapLocal(value)
		fields, _ := value["value"].(map[string]any)
		if fields == nil {
			fields = map[string]any{}
		}
		fields = cloneAnyMapLocal(fields)
		outlineField := buildFontOutlineSettingsStructFieldFromExisting(fields["OutlineSettings"])
		outlineValue, _ := outlineField["value"].(map[string]any)
		if outlineValue == nil {
			outlineValue = map[string]any{}
		}
		outlineValue = cloneAnyMapLocal(outlineValue)
		outlineFields, _ := outlineValue["value"].(map[string]any)
		if outlineFields == nil {
			outlineFields = map[string]any{}
		}
		outlineFields = cloneAnyMapLocal(outlineFields)
		outlineFields["OutlineColor"] = buildWidgetLinearColorStructFieldWithFlagsFromExisting(outlineFields["OutlineColor"], color, 8)
		outlineValue["value"] = outlineFields
		outlineField["value"] = outlineValue
		outlineField["flags"] = int32(8)
		fields["OutlineSettings"] = outlineField
		value["value"] = fields
		field["value"] = value
		return buildTextBlockStyleFieldMutation("Font", field, []string{
			"Font", "StructProperty", "SlateFontInfo", "/Script/SlateCore", "OutlineSettings", "FontOutlineSettings", "OutlineColor", "LinearColor", "/Script/CoreUObject",
		})
	case "text-justification", "text-justify":
		enumValue, err := normalizeWidgetTextJustificationValue(rawValue)
		if err != nil {
			return textBlockStyleMutation{}, err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "ETextJustify", "value": enumValue})
		if err != nil {
			return textBlockStyleMutation{}, err
		}
		return textBlockStyleMutation{
			PropertyPath:  "Justification",
			ValueJSON:     valueJSON,
			AddSpecJSON:   fmt.Sprintf(`{"name":"Justification","type":"ByteProperty(ETextJustify(/Script/Slate))","value":%s}`, valueJSON),
			RequiredNames: []string{"Justification", "ByteProperty", "ETextJustify", "/Script/Slate", enumValue},
		}, nil
	default:
		return textBlockStyleMutation{}, fmt.Errorf("unsupported TextBlock style property %q", normalizedProperty)
	}
}

func isTextBlockFontObjectProperty(normalizedProperty string) bool {
	switch normalizedProperty {
	case "text-font", "textfont", "text-font-family", "textfontfamily", "text-font-object", "textfontobject":
		return true
	default:
		return false
	}
}

func buildTextBlockStyleFieldMutation(propertyPath string, field map[string]any, requiredNames []string) (textBlockStyleMutation, error) {
	valueJSON, err := marshalJSONValue(field["value"])
	if err != nil {
		return textBlockStyleMutation{}, err
	}
	addSpecMap := cloneAnyMapLocal(field)
	addSpecMap["name"] = propertyPath
	addSpecJSON, err := marshalJSONValue(addSpecMap)
	if err != nil {
		return textBlockStyleMutation{}, err
	}
	return textBlockStyleMutation{
		PropertyPath:  propertyPath,
		ValueJSON:     valueJSON,
		AddSpecJSON:   addSpecJSON,
		RequiredNames: requiredNames,
	}, nil
}

func applyRichTextStyleWrite(asset *uasset.Asset, opts uasset.ParseOptions, target widgetWriteTarget, normalizedProperty, rawValue string) ([]byte, *uasset.Asset, []string, []map[string]any, string, error) {
	workingBytes := append([]byte(nil), asset.Raw.Bytes...)
	workingAsset := asset
	addedNames := []string{}
	updates := []map[string]any{}
	propertyPath := ""
	importIndices := []int32{}
	decoratorPackagePaths := []string{}
	var err error

	if isRichTextObjectImportProperty(normalizedProperty) {
		var importAdded []string
		switch {
		case isRichTextFontObjectProperty(normalizedProperty):
			packagePath, objectName, ok := parseAssetPackagePath(strings.TrimSpace(rawValue))
			if !ok {
				return nil, nil, nil, nil, "", fmt.Errorf("%s requires a full asset package path like /Game/UI/Foundation/Fonts/NotoSans", normalizedProperty)
			}
			workingBytes, workingAsset, importAdded, _ = appendFontImportIfMissing(workingAsset, opts, packagePath, objectName)
			importIndex, found := findFontImportByPath(workingAsset, packagePath)
			if !found {
				importIndex, found = findObjectImportByObjectName(workingAsset, "/Script/Engine", "Font", objectName)
			}
			if !found {
				return nil, nil, nil, nil, "", fmt.Errorf("font import %q not found after insertion", packagePath)
			}
			importIndices = append(importIndices, int32(-importIndex))
		case isRichTextStyleSetProperty(normalizedProperty):
			packagePath, objectName, ok := parseAssetPackagePath(strings.TrimSpace(rawValue))
			if !ok {
				return nil, nil, nil, nil, "", fmt.Errorf("%s requires a full asset package path like /Game/UI/Settings/SettingsDescriptionStyles", normalizedProperty)
			}
			workingBytes, workingAsset, importAdded, _ = appendDataTableImportIfMissing(workingAsset, opts, packagePath, objectName)
			importIndex, found := findDataTableImportByPath(workingAsset, packagePath)
			if !found {
				return nil, nil, nil, nil, "", fmt.Errorf("data table import %q not found after insertion", packagePath)
			}
			importIndices = append(importIndices, int32(-importIndex))
		case isRichTextDecoratorProperty(normalizedProperty):
			classPaths, parseErr := parseRichTextDecoratorClassList(rawValue)
			if parseErr != nil {
				return nil, nil, nil, nil, "", parseErr
			}
			for _, classPath := range classPaths {
				packagePath, objectName, ok := parseBlueprintGeneratedClassPath(classPath)
				if !ok {
					return nil, nil, nil, nil, "", fmt.Errorf("%s requires Unreal asset paths like /Game/UI/Settings/NewRichTextBlockDecorator", normalizedProperty)
				}
				decoratorPackagePaths = append(decoratorPackagePaths, packagePath)
				var currentAdded []string
				workingBytes, workingAsset, currentAdded, _ = appendBlueprintGeneratedClassImportIfMissing(workingAsset, opts, packagePath, objectName)
				addedNames = appendUniqueStringSlice(addedNames, currentAdded)
			}
		default:
			return nil, nil, nil, nil, "", fmt.Errorf("unsupported RichTextBlock import-backed property %q", normalizedProperty)
		}
		addedNames = appendUniqueStringSlice(addedNames, importAdded)
	}

	if isRichTextDecoratorProperty(normalizedProperty) {
		importIndices = importIndices[:0]
		for _, packagePath := range decoratorPackagePaths {
			importIndex, found := findBlueprintGeneratedClassImportByPath(workingAsset, packagePath)
			if !found {
				return nil, nil, nil, nil, "", fmt.Errorf("decorator import %q not found after insertion", packagePath)
			}
			importIndices = append(importIndices, int32(-importIndex))
		}
	}

	for _, exportIdx := range richTextStyleTargetExports(workingAsset, target, normalizedProperty) {
		mutations, err := buildRichTextStyleMutations(workingAsset, exportIdx, target.ClassName, normalizedProperty, rawValue, importIndices)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("widget export %d %s: %w", exportIdx+1, normalizedProperty, err)
		}
		for _, mutation := range mutations {
			if propertyPath == "" {
				propertyPath = mutation.PropertyPath
			}
			if len(mutation.RequiredNames) > 0 {
				var ensured []string
				_, workingAsset, ensured, err = ensureNameEntriesPresentSorted(workingAsset, opts, mutation.RequiredNames)
				if err != nil {
					return nil, nil, nil, nil, "", fmt.Errorf("ensure richtext names: %w", err)
				}
				addedNames = appendUniqueStringSlice(addedNames, ensured)
			}
			outBytes, result, writeErr := applyPropertySetCommandWithPostprocess(workingAsset, exportIdx, mutation.PropertyPath, mutation.ValueJSON, opts)
			if writeErr != nil && mutation.AddSpecJSON != "" && widgetSlotPropertyWriteShouldFallbackToAdd(writeErr) {
				outBytes, result, writeErr = applyPropertyAddAsSetResultBefore(workingAsset, exportIdx, mutation.AddSpecJSON, richTextStyleAddBeforeProperty(workingAsset, exportIdx, normalizedProperty), opts)
			}
			if writeErr != nil {
				return nil, nil, nil, nil, "", fmt.Errorf("widget export %d %s: %w", exportIdx+1, mutation.PropertyPath, writeErr)
			}
			workingBytes = outBytes
			workingAsset, err = uasset.ParseBytes(workingBytes, opts)
			if err != nil {
				return nil, nil, nil, nil, "", fmt.Errorf("reparse rewritten asset: %w", err)
			}
			updates = append(updates, map[string]any{
				"export":   exportIdx + 1,
				"path":     mutation.PropertyPath,
				"oldValue": result.OldValue,
				"newValue": result.NewValue,
			})
		}
	}

	if len(importIndices) > 0 {
		designerIdx := selectBrushDependsExport(workingAsset, target)
		if designerIdx >= 0 {
			for _, importIdx := range importIndices {
				if importIdx == 0 {
					continue
				}
				workingBytes, workingAsset, err = syncWidgetImportDependency(workingAsset, opts, designerIdx, importIdx)
				if err != nil {
					return nil, nil, nil, nil, "", fmt.Errorf("sync rich text import depends: %w", err)
				}
			}
		}
	}

	return workingBytes, workingAsset, addedNames, updates, propertyPath, nil
}

type richTextStyleMutation struct {
	PropertyPath  string
	ValueJSON     string
	AddSpecJSON   string
	RequiredNames []string
}

func buildRichTextStyleMutations(asset *uasset.Asset, exportIndex int, className, normalizedProperty, rawValue string, importIndices []int32) ([]richTextStyleMutation, error) {
	if !strings.EqualFold(className, "RichTextBlock") {
		return nil, fmt.Errorf("%s requires a RichTextBlock widget, got %s", normalizedProperty, className)
	}
	switch normalizedProperty {
	case "richtext-style-set", "richtext-styleset", "richtext-text-style-set", "richtext-textstyleset":
		mutation, err := buildRichTextStyleSetMutation(normalizedProperty, firstRichTextImportIndex(importIndices))
		if err != nil {
			return nil, err
		}
		return []richTextStyleMutation{mutation}, nil
	case "richtext-decorator-classes", "richtext-decoratorclasses", "richtext-decorators":
		mutation, err := buildRichTextDecoratorClassesMutation(importIndices)
		if err != nil {
			return nil, err
		}
		return []richTextStyleMutation{mutation}, nil
	case "richtext-override-default-style", "richtext-overridedefaultstyle":
		mutation, err := buildRichTextOverrideDefaultStyleMutation(rawValue)
		if err != nil {
			return nil, err
		}
		return []richTextStyleMutation{mutation}, nil
	case "richtext-auto-wrap-text", "richtextautowraptext":
		mutation, err := buildRichTextAutoWrapTextMutation(rawValue)
		if err != nil {
			return nil, err
		}
		return []richTextStyleMutation{mutation}, nil
	case "richtext-wrap-text-at", "richtextwraptextat":
		mutation, err := buildRichTextWrapTextAtMutation(rawValue)
		if err != nil {
			return nil, err
		}
		return []richTextStyleMutation{mutation}, nil
	case "richtext-line-height-percentage", "richtextlineheightpercentage":
		mutation, err := buildRichTextLineHeightPercentageMutation(rawValue)
		if err != nil {
			return nil, err
		}
		return []richTextStyleMutation{mutation}, nil
	case "richtext-justification", "richtext-justify":
		mutation, err := buildRichTextJustificationMutation(rawValue)
		if err != nil {
			return nil, err
		}
		return []richTextStyleMutation{mutation}, nil
	case "richtext-default-font-size", "richtext-default-fontsize",
		"richtext-default-font", "richtext-default-fontfamily", "richtext-default-font-family", "richtext-default-fontobject",
		"richtext-default-typeface", "richtext-default-font-face",
		"richtext-default-color-and-opacity", "richtext-default-colorandopacity",
		"richtext-default-shadow-offset", "richtext-default-shadowoffset",
		"richtext-default-shadow-color-and-opacity", "richtext-default-shadow-colorandopacity",
		"richtext-default-outline-size", "richtext-default-outlinesize",
		"richtext-default-outline-color", "richtext-default-outlinecolor":
		styleMutation, err := buildRichTextDefaultStyleMutation(asset, exportIndex, normalizedProperty, rawValue, firstRichTextImportIndex(importIndices))
		if err != nil {
			return nil, err
		}
		overrideMutation, err := buildRichTextOverrideDefaultStyleMutation("true")
		if err != nil {
			return nil, err
		}
		return []richTextStyleMutation{styleMutation, overrideMutation}, nil
	default:
		return nil, fmt.Errorf("unsupported RichTextBlock style property %q", normalizedProperty)
	}
}

func buildRichTextStyleSetMutation(normalizedProperty string, importIdx int32) (richTextStyleMutation, error) {
	if importIdx == 0 {
		return richTextStyleMutation{}, fmt.Errorf("%s requires a resolved data table import", normalizedProperty)
	}
	return richTextStyleMutation{
		PropertyPath:  "TextStyleSet",
		ValueJSON:     fmt.Sprintf(`{"index":%d}`, importIdx),
		AddSpecJSON:   fmt.Sprintf(`{"name":"TextStyleSet","type":"ObjectProperty","value":{"index":%d}}`, importIdx),
		RequiredNames: []string{"TextStyleSet", "ObjectProperty"},
	}, nil
}

func buildRichTextDecoratorClassesMutation(importIndices []int32) (richTextStyleMutation, error) {
	if len(importIndices) == 0 {
		return richTextStyleMutation{}, fmt.Errorf("richtext-decorator-classes requires at least one resolved decorator import")
	}
	values := make([]any, 0, len(importIndices))
	for _, importIdx := range importIndices {
		values = append(values, buildObjectPropertyValue(importIdx))
	}
	valueJSON, err := marshalJSONValue(values)
	if err != nil {
		return richTextStyleMutation{}, err
	}
	return richTextStyleMutation{
		PropertyPath:  "DecoratorClasses",
		ValueJSON:     valueJSON,
		AddSpecJSON:   fmt.Sprintf(`{"name":"DecoratorClasses","type":"ArrayProperty(ObjectProperty)","value":%s}`, valueJSON),
		RequiredNames: []string{"DecoratorClasses", "ArrayProperty", "ObjectProperty"},
	}, nil
}

func buildRichTextOverrideDefaultStyleMutation(rawValue string) (richTextStyleMutation, error) {
	value, err := parseWidgetBoolValue(rawValue, "richtext-override-default-style")
	if err != nil {
		return richTextStyleMutation{}, err
	}
	valueJSON := "false"
	if value {
		valueJSON = "true"
	}
	return richTextStyleMutation{
		PropertyPath:  "bOverrideDefaultStyle",
		ValueJSON:     valueJSON,
		AddSpecJSON:   fmt.Sprintf(`{"name":"bOverrideDefaultStyle","type":"BoolProperty","value":%s}`, valueJSON),
		RequiredNames: []string{"bOverrideDefaultStyle", "BoolProperty"},
	}, nil
}

func buildRichTextAutoWrapTextMutation(rawValue string) (richTextStyleMutation, error) {
	value, err := parseWidgetBoolValue(rawValue, "richtext-auto-wrap-text")
	if err != nil {
		return richTextStyleMutation{}, err
	}
	valueJSON := "false"
	if value {
		valueJSON = "true"
	}
	return richTextStyleMutation{
		PropertyPath:  "AutoWrapText",
		ValueJSON:     valueJSON,
		AddSpecJSON:   fmt.Sprintf(`{"name":"AutoWrapText","type":"BoolProperty","value":%s}`, valueJSON),
		RequiredNames: []string{"AutoWrapText", "BoolProperty"},
	}, nil
}

func buildRichTextWrapTextAtMutation(rawValue string) (richTextStyleMutation, error) {
	value, err := parseWidgetFloat32(rawValue)
	if err != nil {
		return richTextStyleMutation{}, fmt.Errorf("richtext-wrap-text-at: %w", err)
	}
	scalar := strconv.FormatFloat(float64(value), 'f', -1, 32)
	return richTextStyleMutation{
		PropertyPath:  "WrapTextAt",
		ValueJSON:     scalar,
		AddSpecJSON:   `{"name":"WrapTextAt","type":"FloatProperty","value":` + scalar + `}`,
		RequiredNames: []string{"WrapTextAt", "FloatProperty"},
	}, nil
}

func buildRichTextLineHeightPercentageMutation(rawValue string) (richTextStyleMutation, error) {
	value, err := parseWidgetFloat32(rawValue)
	if err != nil {
		return richTextStyleMutation{}, fmt.Errorf("richtext-line-height-percentage: %w", err)
	}
	scalar := strconv.FormatFloat(float64(value), 'f', -1, 32)
	return richTextStyleMutation{
		PropertyPath:  "LineHeightPercentage",
		ValueJSON:     scalar,
		AddSpecJSON:   `{"name":"LineHeightPercentage","type":"FloatProperty","value":` + scalar + `}`,
		RequiredNames: []string{"LineHeightPercentage", "FloatProperty"},
	}, nil
}

func buildRichTextJustificationMutation(rawValue string) (richTextStyleMutation, error) {
	enumValue, err := normalizeWidgetTextJustificationValue(rawValue)
	if err != nil {
		return richTextStyleMutation{}, err
	}
	valueJSON, err := marshalJSONValue(map[string]any{"enumType": "ETextJustify", "value": enumValue})
	if err != nil {
		return richTextStyleMutation{}, err
	}
	return richTextStyleMutation{
		PropertyPath:  "Justification",
		ValueJSON:     valueJSON,
		AddSpecJSON:   fmt.Sprintf(`{"name":"Justification","type":"ByteProperty(ETextJustify(/Script/SlateCore))","value":%s}`, valueJSON),
		RequiredNames: []string{"Justification", "ByteProperty", "ETextJustify", "/Script/SlateCore", enumValue},
	}, nil
}

func buildRichTextDefaultStyleMutation(asset *uasset.Asset, exportIndex int, normalizedProperty, rawValue string, fontImportIdx int32) (richTextStyleMutation, error) {
	current, err := decodeExportRootPropertyValue(asset, exportIndex, "DefaultTextStyleOverride")
	if err != nil {
		current = nil
	}
	valueJSON, requiredNames, err := buildRichTextDefaultStyleValueJSON(asset, current, normalizedProperty, rawValue, fontImportIdx)
	if err != nil {
		return richTextStyleMutation{}, err
	}
	return richTextStyleMutation{
		PropertyPath:  "DefaultTextStyleOverride",
		ValueJSON:     valueJSON,
		AddSpecJSON:   fmt.Sprintf(`{"name":"DefaultTextStyleOverride","type":"StructProperty(TextBlockStyle(/Script/SlateCore))","value":%s}`, valueJSON),
		RequiredNames: requiredNames,
	}, nil
}

func buildRichTextDefaultStyleValueJSON(asset *uasset.Asset, current any, normalizedProperty, rawValue string, fontImportIdx int32) (string, []string, error) {
	style := buildRichTextTextBlockStyleValue(asset, current)
	fields, _ := style["value"].(map[string]any)
	if fields == nil {
		fields = map[string]any{}
	}

	requiredNames := []string{"DefaultTextStyleOverride", "StructProperty", "TextBlockStyle", "/Script/SlateCore"}

	switch normalizedProperty {
	case "richtext-default-font", "richtext-default-fontfamily", "richtext-default-font-family", "richtext-default-fontobject":
		if fontImportIdx == 0 {
			return "", nil, fmt.Errorf("%s requires a resolved font import", normalizedProperty)
		}
		fontField := buildSlateFontInfoStructFieldFromExisting(fields["Font"])
		fontValue, _ := fontField["value"].(map[string]any)
		fontFields, _ := fontValue["value"].(map[string]any)
		if fontFields == nil {
			fontFields = map[string]any{}
		}
		fontFields["FontObject"] = buildObjectPropertyField(fontImportIdx)
		fontValue["value"] = fontFields
		fontField["value"] = fontValue
		fields["Font"] = fontField
		requiredNames = append(requiredNames, "Font", "SlateFontInfo", "FontObject", "ObjectProperty")
	case "richtext-default-typeface", "richtext-default-font-face":
		typeface := strings.TrimSpace(rawValue)
		if typeface == "" {
			return "", nil, fmt.Errorf("%s requires --value", normalizedProperty)
		}
		fontField := buildSlateFontInfoStructFieldFromExisting(fields["Font"])
		fontValue, _ := fontField["value"].(map[string]any)
		fontFields, _ := fontValue["value"].(map[string]any)
		if fontFields == nil {
			fontFields = map[string]any{}
		}
		fontFields["TypefaceFontName"] = buildNamePropertyField(typeface)
		fontValue["value"] = fontFields
		fontField["value"] = fontValue
		fields["Font"] = fontField
		requiredNames = append(requiredNames, "Font", "SlateFontInfo", "TypefaceFontName", "NameProperty", typeface)
	case "richtext-default-font-size", "richtext-default-fontsize":
		size, err := parseWidgetFloat32(rawValue)
		if err != nil {
			return "", nil, fmt.Errorf("richtext-default-font-size: %w", err)
		}
		fontField := buildSlateFontInfoStructFieldFromExisting(fields["Font"])
		fontValue, _ := fontField["value"].(map[string]any)
		fontFields, _ := fontValue["value"].(map[string]any)
		if fontFields == nil {
			fontFields = map[string]any{}
		}
		fontFields["Size"] = map[string]any{
			"type":  "FloatProperty",
			"value": size,
		}
		fontValue["value"] = fontFields
		fontField["value"] = fontValue
		fields["Font"] = fontField
		requiredNames = append(requiredNames, "Font", "SlateFontInfo", "Size", "FloatProperty")
	case "richtext-default-color-and-opacity", "richtext-default-colorandopacity":
		color, err := parseWidgetLinearColorValue(rawValue, normalizedProperty)
		if err != nil {
			return "", nil, err
		}
		fields["ColorAndOpacity"] = buildSpecifiedSlateColorFieldFromExisting(fields["ColorAndOpacity"], color)
		requiredNames = append(requiredNames,
			"ColorAndOpacity", "SlateColor", "SpecifiedColor", "LinearColor", "/Script/CoreUObject",
		)
	case "richtext-default-shadow-offset", "richtext-default-shadowoffset":
		x, y, err := parseWidgetFloatPair(rawValue)
		if err != nil {
			return "", nil, fmt.Errorf("%s: %w", normalizedProperty, err)
		}
		fields["ShadowOffset"] = buildRichTextShadowOffsetFieldFromExisting(fields["ShadowOffset"], x, y)
		requiredNames = append(requiredNames, "ShadowOffset", "DeprecateSlateVector2D", "/Script/SlateCore")
	case "richtext-default-shadow-color-and-opacity", "richtext-default-shadow-colorandopacity":
		color, err := parseWidgetLinearColorValue(rawValue, normalizedProperty)
		if err != nil {
			return "", nil, err
		}
		fields["ShadowColorAndOpacity"] = buildWidgetLinearColorStructFieldWithFlagsFromExisting(fields["ShadowColorAndOpacity"], color, 8)
		requiredNames = append(requiredNames, "ShadowColorAndOpacity", "LinearColor", "/Script/CoreUObject")
	case "richtext-default-outline-size", "richtext-default-outlinesize":
		value, err := parseWidgetIntValue(rawValue)
		if err != nil {
			return "", nil, fmt.Errorf("%s: %w", normalizedProperty, err)
		}
		fontField := buildSlateFontInfoStructFieldFromExisting(fields["Font"])
		fontValue, _ := fontField["value"].(map[string]any)
		fontFields, _ := fontValue["value"].(map[string]any)
		if fontFields == nil {
			fontFields = map[string]any{}
		}
		outlineField := buildRichTextFontOutlineSettingsStructFieldFromExisting(fontFields["OutlineSettings"])
		outlineValue, _ := outlineField["value"].(map[string]any)
		outlineFields, _ := outlineValue["value"].(map[string]any)
		if outlineFields == nil {
			outlineFields = map[string]any{}
		}
		outlineFields["OutlineSize"] = map[string]any{
			"type":  "IntProperty",
			"value": value,
		}
		outlineValue["value"] = outlineFields
		outlineField["value"] = outlineValue
		fontFields["OutlineSettings"] = outlineField
		fontValue["value"] = fontFields
		fontField["value"] = fontValue
		fields["Font"] = fontField
		requiredNames = append(requiredNames, "Font", "SlateFontInfo", "OutlineSettings", "FontOutlineSettings", "OutlineSize", "IntProperty")
	case "richtext-default-outline-color", "richtext-default-outlinecolor":
		color, err := parseWidgetLinearColorValue(rawValue, normalizedProperty)
		if err != nil {
			return "", nil, err
		}
		fontField := buildSlateFontInfoStructFieldFromExisting(fields["Font"])
		fontValue, _ := fontField["value"].(map[string]any)
		fontFields, _ := fontValue["value"].(map[string]any)
		if fontFields == nil {
			fontFields = map[string]any{}
		}
		outlineField := buildRichTextFontOutlineSettingsStructFieldFromExisting(fontFields["OutlineSettings"])
		outlineValue, _ := outlineField["value"].(map[string]any)
		outlineFields, _ := outlineValue["value"].(map[string]any)
		if outlineFields == nil {
			outlineFields = map[string]any{}
		}
		outlineFields["OutlineColor"] = buildWidgetLinearColorStructFieldWithFlagsFromExisting(outlineFields["OutlineColor"], color, 8)
		outlineValue["value"] = outlineFields
		outlineField["value"] = outlineValue
		fontFields["OutlineSettings"] = outlineField
		fontValue["value"] = fontFields
		fontField["value"] = fontValue
		fields["Font"] = fontField
		requiredNames = append(requiredNames, "Font", "SlateFontInfo", "OutlineSettings", "FontOutlineSettings", "OutlineColor", "LinearColor", "/Script/CoreUObject")
	default:
		return "", nil, fmt.Errorf("unsupported RichTextBlock default style property %q", normalizedProperty)
	}

	style["value"] = fields
	valueJSON, err := marshalJSONValue(style)
	if err != nil {
		return "", nil, err
	}
	return valueJSON, requiredNames, nil
}

func buildRichTextTextBlockStyleValue(asset *uasset.Asset, current any) map[string]any {
	currentMap, _ := current.(map[string]any)
	style := map[string]any{
		"structType": "TextBlockStyle",
		"value":      map[string]any{},
	}
	if currentMap == nil {
		return style
	}
	if structType, _ := currentMap["structType"].(string); strings.EqualFold(structType, "TextBlockStyle") {
		style = cloneAnyMapLocal(currentMap)
	} else {
		for key, value := range currentMap {
			style[key] = value
		}
		style["structType"] = "TextBlockStyle"
	}
	if fields, ok := style["value"].(map[string]any); ok {
		style["value"] = cloneAnyMapLocal(fields)
	} else {
		style["value"] = map[string]any{}
	}
	if refreshed, ok := refreshWidgetNamePropertyReferences(asset, style).(map[string]any); ok {
		style = refreshed
	}
	return style
}

func refreshWidgetNamePropertyReferences(asset *uasset.Asset, raw any) any {
	switch typed := raw.(type) {
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = refreshWidgetNamePropertyReferences(asset, item)
		}
		return out
	case map[string]any:
		out := cloneAnyMapLocal(typed)
		typeName, _ := out["type"].(string)
		if strings.EqualFold(strings.TrimSpace(typeName), "NameProperty") {
			valueMap, _ := out["value"].(map[string]any)
			if valueMap != nil {
				refreshed := map[string]any{}
				if name, _ := valueMap["name"].(string); strings.TrimSpace(name) != "" {
					refreshed["name"] = name
				} else if idx, ok := anyInt(valueMap["index"]); ok && idx >= 0 && asset != nil && idx < len(asset.Names) {
					refreshed["name"] = asset.Names[idx].Value
				}
				if number, ok := valueMap["number"]; ok {
					refreshed["number"] = number
				}
				if len(refreshed) > 0 {
					out["value"] = refreshed
					return out
				}
			}
		}
		for key, value := range out {
			out[key] = refreshWidgetNamePropertyReferences(asset, value)
		}
		return out
	default:
		return raw
	}
}

func buildSlateFontInfoStructFieldFromExisting(raw any) map[string]any {
	return buildSlateTaggedStructFieldFromExisting(raw, "StructProperty(SlateFontInfo(/Script/SlateCore))", "SlateFontInfo")
}

func buildFontOutlineSettingsStructFieldFromExisting(raw any) map[string]any {
	return buildSlateTaggedStructFieldFromExisting(raw, "StructProperty(FontOutlineSettings(/Script/SlateCore))", "FontOutlineSettings")
}

func buildRichTextFontOutlineSettingsStructFieldFromExisting(raw any) map[string]any {
	field := buildFontOutlineSettingsStructFieldFromExisting(raw)
	field["flags"] = int32(8)
	return field
}

func buildWidgetLinearColorStructFieldWithFlagsFromExisting(raw any, color widgetLinearColor, flags int32) map[string]any {
	field := buildSlateTaggedStructFieldFromExisting(raw, "StructProperty(LinearColor(/Script/CoreUObject))", "LinearColor")
	if flags != 0 {
		field["flags"] = flags
	}
	value, _ := field["value"].(map[string]any)
	if value == nil {
		value = map[string]any{}
	}
	value = cloneAnyMapLocal(value)
	value["structType"] = "LinearColor"
	value["value"] = map[string]any{
		"r": color.R,
		"g": color.G,
		"b": color.B,
		"a": color.A,
	}
	field["value"] = value
	return field
}

func buildRichTextShadowOffsetFieldFromExisting(raw any, x, y float32) map[string]any {
	field := buildSlateTaggedStructFieldFromExisting(raw, "StructProperty(DeprecateSlateVector2D(/Script/SlateCore))", "DeprecateSlateVector2D")
	field["flags"] = int32(8)
	field["value"] = map[string]any{
		"structType": "DeprecateSlateVector2D",
		"value": map[string]any{
			"rawBase64":  buildWidgetDeprecateSlateVector2DRawBase64(x, y),
			"structType": "DeprecateSlateVector2D",
		},
	}
	return field
}

func buildCoreVector2DStructFieldFromExisting(raw any, x, y float32, flags int32) map[string]any {
	field := buildSlateTaggedStructFieldFromExisting(raw, "StructProperty(Vector2D(/Script/CoreUObject))", "Vector2D")
	if flags != 0 {
		field["flags"] = flags
	}
	field["value"] = map[string]any{
		"structType": "Vector2D",
		"value": map[string]any{
			"x": x,
			"y": y,
		},
	}
	return field
}

func buildObjectPropertyField(importIdx int32) map[string]any {
	return map[string]any{
		"type":  "ObjectProperty",
		"value": buildObjectPropertyValue(importIdx),
	}
}

func buildObjectPropertyValue(importIdx int32) map[string]any {
	return map[string]any{
		"index": importIdx,
	}
}

func buildNamePropertyField(name string) map[string]any {
	return map[string]any{
		"type": "NameProperty",
		"value": map[string]any{
			"name": strings.TrimSpace(name),
		},
	}
}

func buildSlateTaggedStructFieldFromExisting(raw any, typeName, structType string) map[string]any {
	field := map[string]any{
		"type": typeName,
		"value": map[string]any{
			"structType": structType,
			"value":      map[string]any{},
		},
	}
	existing, ok := raw.(map[string]any)
	if !ok {
		return field
	}
	field = cloneAnyMapLocal(existing)
	value, _ := field["value"].(map[string]any)
	if value == nil {
		value = map[string]any{}
	}
	value = cloneAnyMapLocal(value)
	if existingStructType, _ := value["structType"].(string); strings.TrimSpace(existingStructType) == "" {
		value["structType"] = structType
	}
	if fields, ok := value["value"].(map[string]any); ok {
		value["value"] = cloneAnyMapLocal(fields)
	} else {
		value["value"] = map[string]any{}
	}
	field["value"] = value
	if _, ok := field["type"].(string); !ok {
		field["type"] = typeName
	}
	return field
}

func buildSpecifiedSlateColorFieldFromExisting(raw any, color widgetLinearColor) map[string]any {
	field := buildSlateTaggedStructFieldFromExisting(raw, "StructProperty(SlateColor(/Script/SlateCore))", "SlateColor")
	value, _ := field["value"].(map[string]any)
	if value == nil {
		value = map[string]any{}
	}
	value = cloneAnyMapLocal(value)
	fields, _ := value["value"].(map[string]any)
	if fields == nil {
		fields = map[string]any{}
	}
	fields = cloneAnyMapLocal(fields)
	fresh := buildSlateColorField(color)
	freshValue, _ := fresh["value"].(map[string]any)
	freshFields, _ := freshValue["value"].(map[string]any)
	if freshFields != nil {
		fields["SpecifiedColor"] = freshFields["SpecifiedColor"]
	}
	value["value"] = fields
	field["value"] = value
	return field
}

func richTextStyleAddBeforeProperty(asset *uasset.Asset, exportIndex int, normalizedProperty string) string {
	switch normalizedProperty {
	case "richtext-style-set", "richtext-styleset", "richtext-text-style-set", "richtext-textstyleset":
		return widgetPropertyAddBeforePropertyForExport(asset, exportIndex, "DefaultTextStyleOverride", "Slot", "DisplayLabel")
	case "richtext-decorator-classes", "richtext-decoratorclasses", "richtext-decorators":
		return widgetPropertyAddBeforePropertyForExport(asset, exportIndex, "DefaultTextStyleOverride", "Slot", "DisplayLabel")
	default:
		return widgetPropertyAddBeforePropertyForExport(asset, exportIndex, "Slot", "DisplayLabel")
	}
}

func isRichTextFontObjectProperty(normalizedProperty string) bool {
	switch normalizedProperty {
	case "richtext-default-font", "richtext-default-fontfamily", "richtext-default-font-family", "richtext-default-fontobject":
		return true
	default:
		return false
	}
}

func isRichTextStyleSetProperty(normalizedProperty string) bool {
	switch normalizedProperty {
	case "richtext-style-set", "richtext-styleset", "richtext-text-style-set", "richtext-textstyleset":
		return true
	default:
		return false
	}
}

func isRichTextDecoratorProperty(normalizedProperty string) bool {
	switch normalizedProperty {
	case "richtext-decorator-classes", "richtext-decoratorclasses", "richtext-decorators":
		return true
	default:
		return false
	}
}

func isRichTextObjectImportProperty(normalizedProperty string) bool {
	return isRichTextFontObjectProperty(normalizedProperty) || isRichTextStyleSetProperty(normalizedProperty) || isRichTextDecoratorProperty(normalizedProperty)
}

func richTextStyleTargetExports(asset *uasset.Asset, target widgetWriteTarget, normalizedProperty string) []int {
	if !isRichTextStyleSetProperty(normalizedProperty) && !isRichTextDecoratorProperty(normalizedProperty) {
		return append([]int(nil), target.Exports...)
	}
	designerExport := selectBrushDependsExport(asset, target)
	if designerExport >= 0 {
		return []int{designerExport}
	}
	return append([]int(nil), target.Exports...)
}

func firstRichTextImportIndex(importIndices []int32) int32 {
	if len(importIndices) == 0 {
		return 0
	}
	return importIndices[0]
}

func parseRichTextDecoratorClassList(rawValue string) ([]string, error) {
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return nil, fmt.Errorf("richtext-decorator-classes requires --value")
	}
	if strings.HasPrefix(trimmed, "[") {
		var items []string
		if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
			return nil, fmt.Errorf("richtext-decorator-classes: parse JSON array: %w", err)
		}
		out := make([]string, 0, len(items))
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("richtext-decorator-classes requires at least one decorator path")
		}
		return out, nil
	}
	return []string{trimmed}, nil
}

func parseBlueprintGeneratedClassPath(selector string) (string, string, bool) {
	trimmed := strings.TrimSpace(selector)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return "", "", false
	}
	packagePath := trimmed
	objectName := ""
	if dot := strings.LastIndexByte(trimmed, '.'); dot > strings.LastIndexByte(trimmed, '/') {
		packagePath = trimmed[:dot]
		objectName = trimmed[dot+1:]
	} else {
		base := pathBaseName(trimmed)
		if base == "" {
			return "", "", false
		}
		objectName = base + "_C"
	}
	if packagePath == "" || objectName == "" {
		return "", "", false
	}
	return packagePath, objectName, true
}

func isButtonBrushStyleProperty(normalizedProperty string) bool {
	_, ok := normalizeWidgetButtonBrushProperty(normalizedProperty)
	return ok
}

func normalizeWidgetButtonBrushProperty(normalizedProperty string) (widgetButtonBrushProperty, bool) {
	switch normalizedProperty {
	case "button-normal-image", "button-normal-brush-image", "brush-normal-image", "brush-normal":
		return widgetButtonBrushProperty{CanonicalProperty: "button-normal-image", StateField: "Normal", BrushField: "image"}, true
	case "button-hovered-image", "button-hovered-brush-image", "brush-hovered-image", "brush-hovered":
		return widgetButtonBrushProperty{CanonicalProperty: "button-hovered-image", StateField: "Hovered", BrushField: "image"}, true
	case "button-pressed-image", "button-pressed-brush-image", "brush-pressed-image", "brush-pressed":
		return widgetButtonBrushProperty{CanonicalProperty: "button-pressed-image", StateField: "Pressed", BrushField: "image"}, true
	case "button-disabled-image", "button-disabled-brush-image", "brush-disabled-image", "brush-disabled":
		return widgetButtonBrushProperty{CanonicalProperty: "button-disabled-image", StateField: "Disabled", BrushField: "image"}, true
	case "button-normal-tint", "brush-normal-tint":
		return widgetButtonBrushProperty{CanonicalProperty: "button-normal-tint", StateField: "Normal", BrushField: "tint"}, true
	case "button-hovered-tint", "brush-hovered-tint":
		return widgetButtonBrushProperty{CanonicalProperty: "button-hovered-tint", StateField: "Hovered", BrushField: "tint"}, true
	case "button-pressed-tint", "brush-pressed-tint":
		return widgetButtonBrushProperty{CanonicalProperty: "button-pressed-tint", StateField: "Pressed", BrushField: "tint"}, true
	case "button-disabled-tint", "brush-disabled-tint":
		return widgetButtonBrushProperty{CanonicalProperty: "button-disabled-tint", StateField: "Disabled", BrushField: "tint"}, true
	case "button-normal-image-size", "brush-normal-image-size":
		return widgetButtonBrushProperty{CanonicalProperty: "button-normal-image-size", StateField: "Normal", BrushField: "image-size"}, true
	case "button-hovered-image-size", "brush-hovered-image-size":
		return widgetButtonBrushProperty{CanonicalProperty: "button-hovered-image-size", StateField: "Hovered", BrushField: "image-size"}, true
	case "button-pressed-image-size", "brush-pressed-image-size":
		return widgetButtonBrushProperty{CanonicalProperty: "button-pressed-image-size", StateField: "Pressed", BrushField: "image-size"}, true
	case "button-disabled-image-size", "brush-disabled-image-size":
		return widgetButtonBrushProperty{CanonicalProperty: "button-disabled-image-size", StateField: "Disabled", BrushField: "image-size"}, true
	case "button-normal-draw-as", "brush-normal-draw-as":
		return widgetButtonBrushProperty{CanonicalProperty: "button-normal-draw-as", StateField: "Normal", BrushField: "draw-as"}, true
	case "button-hovered-draw-as", "brush-hovered-draw-as":
		return widgetButtonBrushProperty{CanonicalProperty: "button-hovered-draw-as", StateField: "Hovered", BrushField: "draw-as"}, true
	case "button-pressed-draw-as", "brush-pressed-draw-as":
		return widgetButtonBrushProperty{CanonicalProperty: "button-pressed-draw-as", StateField: "Pressed", BrushField: "draw-as"}, true
	case "button-disabled-draw-as", "brush-disabled-draw-as":
		return widgetButtonBrushProperty{CanonicalProperty: "button-disabled-draw-as", StateField: "Disabled", BrushField: "draw-as"}, true
	default:
		return widgetButtonBrushProperty{}, false
	}
}

func applyWidgetSlotPropertyWrite(asset *uasset.Asset, opts uasset.ParseOptions, target widgetWriteTarget, normalizedProperty, rawValue string) ([]byte, *uasset.Asset, []string, []map[string]any, string, error) {
	if len(target.SlotExports) == 0 {
		return nil, nil, nil, nil, "", fmt.Errorf("widget %q does not resolve to writable slot exports", target.Path)
	}
	slotClassName, err := widgetWriteSlotClassName(asset, target.SlotExports)
	if err != nil {
		return nil, nil, nil, nil, "", err
	}

	requiredNames := widgetSlotPropertyRequiredNames(slotClassName, normalizedProperty, rawValue)
	workingBytes := append([]byte(nil), asset.Raw.Bytes...)
	workingAsset := asset
	addedNames := []string{}
	if len(requiredNames) > 0 {
		workingBytes, workingAsset, addedNames, err = ensureNameEntriesPresentSorted(workingAsset, opts, requiredNames)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("ensure widget slot names: %w", err)
		}
	}

	updates := make([]map[string]any, 0, len(target.SlotExports))
	propertyPath := ""
	for _, exportIdx := range target.SlotExports {
		var valueJSON string
		var addSpecJSON string
		var buildErr error
		propertyPath, valueJSON, addSpecJSON, buildErr = buildWidgetSlotPropertyMutation(workingAsset, exportIdx, slotClassName, normalizedProperty, rawValue)
		if buildErr != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("slot export %d %s: %w", exportIdx+1, normalizedProperty, buildErr)
		}
		outBytes, result, writeErr := applyPropertySetCommandWithPostprocess(workingAsset, exportIdx, propertyPath, valueJSON, opts)
		if writeErr != nil && addSpecJSON != "" && widgetSlotPropertyWriteShouldFallbackToAdd(writeErr) {
			beforeProperty := ""
			if propertyPath == "LayoutData" {
				beforeProperty = "Parent"
			}
			outBytes, result, writeErr = applyPropertyAddAsSetResultBefore(workingAsset, exportIdx, addSpecJSON, beforeProperty, opts)
		}
		if writeErr != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("slot export %d %s: %w", exportIdx+1, propertyPath, writeErr)
		}
		workingBytes = outBytes
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("reparse rewritten asset: %w", err)
		}
		updates = append(updates, map[string]any{
			"export":   exportIdx + 1,
			"path":     propertyPath,
			"oldValue": result.OldValue,
			"newValue": result.NewValue,
		})
	}

	return workingBytes, workingAsset, addedNames, updates, propertyPath, nil
}

func widgetSlotPropertyWriteShouldFallbackToAdd(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "property not found") || strings.Contains(message, "invalid current struct shape")
}

func buildWidgetSlotPropertyMutation(asset *uasset.Asset, exportIndex int, slotClassName, normalizedProperty, rawValue string) (string, string, string, error) {
	switch normalizedProperty {
	case "slot-padding":
		padding, err := parseWidgetMarginValue(rawValue)
		if err != nil {
			return "", "", "", err
		}
		valueJSON, err := buildWidgetMarginValueJSON(padding)
		if err != nil {
			return "", "", "", err
		}
		return "Padding", valueJSON, fmt.Sprintf(`{"name":"Padding","type":"StructProperty(Margin(/Script/SlateCore))","value":%s}`, valueJSON), nil
	case "slot-size":
		if !widgetSlotSupportsChildSize(slotClassName) {
			return "", "", "", fmt.Errorf("slot-size requires HorizontalBoxSlot, VerticalBoxSlot, or StackBoxSlot, got %s", slotClassName)
		}
		childSize, err := parseWidgetSlateChildSizeValue(rawValue)
		if err != nil {
			return "", "", "", err
		}
		valueJSON, err := buildWidgetSlateChildSizeValueJSON(childSize)
		if err != nil {
			return "", "", "", err
		}
		return "Size", valueJSON, fmt.Sprintf(`{"name":"Size","type":"StructProperty(SlateChildSize(/Script/UMG))","value":%s}`, valueJSON), nil
	case "slot-horizontal-alignment", "slot-halign":
		enumValue, err := normalizeWidgetHorizontalAlignmentValue(rawValue)
		if err != nil {
			return "", "", "", err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "EHorizontalAlignment", "value": enumValue})
		if err != nil {
			return "", "", "", err
		}
		return "HorizontalAlignment", valueJSON, fmt.Sprintf(`{"name":"HorizontalAlignment","type":"EnumProperty(EHorizontalAlignment)","value":%s}`, valueJSON), nil
	case "slot-vertical-alignment", "slot-valign":
		enumValue, err := normalizeWidgetVerticalAlignmentValue(rawValue)
		if err != nil {
			return "", "", "", err
		}
		valueJSON, err := marshalJSONValue(map[string]any{"enumType": "EVerticalAlignment", "value": enumValue})
		if err != nil {
			return "", "", "", err
		}
		return "VerticalAlignment", valueJSON, fmt.Sprintf(`{"name":"VerticalAlignment","type":"EnumProperty(EVerticalAlignment)","value":%s}`, valueJSON), nil
	case "slot-row":
		if !widgetSlotSupportsGridPosition(slotClassName) {
			return "", "", "", fmt.Errorf("slot-row requires GridSlot or UniformGridSlot, got %s", slotClassName)
		}
		value, err := parseWidgetIntValue(rawValue)
		if err != nil {
			return "", "", "", err
		}
		return "Row", strconv.Itoa(value), fmt.Sprintf(`{"name":"Row","type":"IntProperty","value":%d}`, value), nil
	case "slot-column":
		if !widgetSlotSupportsGridPosition(slotClassName) {
			return "", "", "", fmt.Errorf("slot-column requires GridSlot or UniformGridSlot, got %s", slotClassName)
		}
		value, err := parseWidgetIntValue(rawValue)
		if err != nil {
			return "", "", "", err
		}
		return "Column", strconv.Itoa(value), fmt.Sprintf(`{"name":"Column","type":"IntProperty","value":%d}`, value), nil
	case "slot-row-span":
		if !widgetSlotSupportsGridPosition(slotClassName) {
			return "", "", "", fmt.Errorf("slot-row-span requires GridSlot or UniformGridSlot, got %s", slotClassName)
		}
		value, err := parseWidgetIntValue(rawValue)
		if err != nil {
			return "", "", "", err
		}
		return "RowSpan", strconv.Itoa(value), fmt.Sprintf(`{"name":"RowSpan","type":"IntProperty","value":%d}`, value), nil
	case "slot-column-span":
		if !widgetSlotSupportsGridPosition(slotClassName) {
			return "", "", "", fmt.Errorf("slot-column-span requires GridSlot or UniformGridSlot, got %s", slotClassName)
		}
		value, err := parseWidgetIntValue(rawValue)
		if err != nil {
			return "", "", "", err
		}
		return "ColumnSpan", strconv.Itoa(value), fmt.Sprintf(`{"name":"ColumnSpan","type":"IntProperty","value":%d}`, value), nil
	case "slot-layer":
		if !widgetSlotSupportsGridPosition(slotClassName) {
			return "", "", "", fmt.Errorf("slot-layer requires GridSlot or UniformGridSlot, got %s", slotClassName)
		}
		value, err := parseWidgetIntValue(rawValue)
		if err != nil {
			return "", "", "", err
		}
		return "Layer", strconv.Itoa(value), fmt.Sprintf(`{"name":"Layer","type":"IntProperty","value":%d}`, value), nil
	case "slot-nudge":
		if !widgetSlotSupportsGridPosition(slotClassName) {
			return "", "", "", fmt.Errorf("slot-nudge requires GridSlot or UniformGridSlot, got %s", slotClassName)
		}
		x, y, err := parseWidgetFloatPair(rawValue)
		if err != nil {
			return "", "", "", err
		}
		valueJSON, err := buildWidgetVector2DValueJSON(x, y)
		if err != nil {
			return "", "", "", err
		}
		return "Nudge", valueJSON, fmt.Sprintf(`{"name":"Nudge","type":"StructProperty(Vector2D(/Script/CoreUObject))","value":%s}`, valueJSON), nil
	case "layout-position":
		if !strings.EqualFold(slotClassName, "CanvasPanelSlot") {
			return "", "", "", fmt.Errorf("layout-position requires CanvasPanelSlot, got %s", slotClassName)
		}
		layout, err := readWidgetAnchorLayoutData(asset, exportIndex, "LayoutData")
		if err != nil {
			return "", "", "", err
		}
		x, y, err := parseWidgetFloatPair(rawValue)
		if err != nil {
			return "", "", "", err
		}
		layout.Left = x
		layout.Top = y
		valueJSON, err := buildWidgetAnchorLayoutValueJSON(layout)
		if err != nil {
			return "", "", "", err
		}
		return "LayoutData", valueJSON, fmt.Sprintf(`{"name":"LayoutData","type":"StructProperty(AnchorData(/Script/UMG))","value":%s}`, valueJSON), nil
	case "layout-size":
		if !strings.EqualFold(slotClassName, "CanvasPanelSlot") {
			return "", "", "", fmt.Errorf("layout-size requires CanvasPanelSlot, got %s", slotClassName)
		}
		layout, err := readWidgetAnchorLayoutData(asset, exportIndex, "LayoutData")
		if err != nil {
			return "", "", "", err
		}
		w, h, err := parseWidgetFloatPair(rawValue)
		if err != nil {
			return "", "", "", err
		}
		layout.Right = w
		layout.Bottom = h
		valueJSON, err := buildWidgetAnchorLayoutValueJSON(layout)
		if err != nil {
			return "", "", "", err
		}
		return "LayoutData", valueJSON, fmt.Sprintf(`{"name":"LayoutData","type":"StructProperty(AnchorData(/Script/UMG))","value":%s}`, valueJSON), nil
	case "layout-anchors":
		if !strings.EqualFold(slotClassName, "CanvasPanelSlot") {
			return "", "", "", fmt.Errorf("layout-anchors requires CanvasPanelSlot, got %s", slotClassName)
		}
		layout, err := readWidgetAnchorLayoutData(asset, exportIndex, "LayoutData")
		if err != nil {
			return "", "", "", err
		}
		minX, minY, maxX, maxY, err := parseWidgetFloatQuad(rawValue)
		if err != nil {
			return "", "", "", err
		}
		layout.MinX = minX
		layout.MinY = minY
		layout.MaxX = maxX
		layout.MaxY = maxY
		valueJSON, err := buildWidgetAnchorLayoutValueJSON(layout)
		if err != nil {
			return "", "", "", err
		}
		return "LayoutData", valueJSON, fmt.Sprintf(`{"name":"LayoutData","type":"StructProperty(AnchorData(/Script/UMG))","value":%s}`, valueJSON), nil
	case "layout-alignment":
		if !strings.EqualFold(slotClassName, "CanvasPanelSlot") {
			return "", "", "", fmt.Errorf("layout-alignment requires CanvasPanelSlot, got %s", slotClassName)
		}
		layout, err := readWidgetAnchorLayoutData(asset, exportIndex, "LayoutData")
		if err != nil {
			return "", "", "", err
		}
		x, y, err := parseWidgetFloatPair(rawValue)
		if err != nil {
			return "", "", "", err
		}
		layout.AlignX = x
		layout.AlignY = y
		valueJSON, err := buildWidgetAnchorLayoutValueJSON(layout)
		if err != nil {
			return "", "", "", err
		}
		return "LayoutData", valueJSON, fmt.Sprintf(`{"name":"LayoutData","type":"StructProperty(AnchorData(/Script/UMG))","value":%s}`, valueJSON), nil
	case "layout-data":
		if !strings.EqualFold(slotClassName, "CanvasPanelSlot") {
			return "", "", "", fmt.Errorf("layout-data requires CanvasPanelSlot, got %s", slotClassName)
		}
		layout, err := parseWidgetAnchorLayoutValue(rawValue)
		if err != nil {
			return "", "", "", err
		}
		valueJSON, err := buildWidgetAnchorLayoutValueJSON(layout)
		if err != nil {
			return "", "", "", err
		}
		return "LayoutData", valueJSON, fmt.Sprintf(`{"name":"LayoutData","type":"StructProperty(AnchorData(/Script/UMG))","value":%s}`, valueJSON), nil
	default:
		return "", "", "", fmt.Errorf("unsupported slot/layout property %q", normalizedProperty)
	}
}

func widgetSlotPropertyRequiredNames(slotClassName, normalizedProperty, rawValue string) []string {
	switch normalizedProperty {
	case "slot-padding":
		return []string{"Padding", "StructProperty", "Margin", "/Script/SlateCore", "Left", "Top", "Right", "Bottom", "FloatProperty"}
	case "slot-size":
		if !widgetSlotSupportsChildSize(slotClassName) {
			return nil
		}
		childSize, err := parseWidgetSlateChildSizeValue(rawValue)
		if err != nil {
			return nil
		}
		return []string{"Size", "StructProperty", "SlateChildSize", "/Script/UMG", "SizeRule", "Value", "ByteProperty", "FloatProperty", "ESlateSizeRule", childSize.Rule}
	case "slot-horizontal-alignment", "slot-halign":
		enumValue, err := normalizeWidgetHorizontalAlignmentValue(rawValue)
		if err != nil {
			return nil
		}
		return []string{"HorizontalAlignment", "EnumProperty", "EHorizontalAlignment", enumValue}
	case "slot-vertical-alignment", "slot-valign":
		enumValue, err := normalizeWidgetVerticalAlignmentValue(rawValue)
		if err != nil {
			return nil
		}
		return []string{"VerticalAlignment", "EnumProperty", "EVerticalAlignment", enumValue}
	case "slot-row":
		if !widgetSlotSupportsGridPosition(slotClassName) {
			return nil
		}
		if _, err := parseWidgetIntValue(rawValue); err != nil {
			return nil
		}
		return []string{"Row", "IntProperty"}
	case "slot-column":
		if !widgetSlotSupportsGridPosition(slotClassName) {
			return nil
		}
		if _, err := parseWidgetIntValue(rawValue); err != nil {
			return nil
		}
		return []string{"Column", "IntProperty"}
	case "slot-row-span":
		if !widgetSlotSupportsGridPosition(slotClassName) {
			return nil
		}
		if _, err := parseWidgetIntValue(rawValue); err != nil {
			return nil
		}
		return []string{"RowSpan", "IntProperty"}
	case "slot-column-span":
		if !widgetSlotSupportsGridPosition(slotClassName) {
			return nil
		}
		if _, err := parseWidgetIntValue(rawValue); err != nil {
			return nil
		}
		return []string{"ColumnSpan", "IntProperty"}
	case "slot-layer":
		if !widgetSlotSupportsGridPosition(slotClassName) {
			return nil
		}
		if _, err := parseWidgetIntValue(rawValue); err != nil {
			return nil
		}
		return []string{"Layer", "IntProperty"}
	case "slot-nudge":
		if !widgetSlotSupportsGridPosition(slotClassName) {
			return nil
		}
		if _, _, err := parseWidgetFloatPair(rawValue); err != nil {
			return nil
		}
		return []string{"Nudge", "StructProperty", "Vector2D", "/Script/CoreUObject", "X", "Y"}
	case "layout-position":
		if !strings.EqualFold(slotClassName, "CanvasPanelSlot") {
			return nil
		}
		x, y, err := parseWidgetFloatPair(rawValue)
		if err != nil {
			return nil
		}
		return widgetLayoutRequiredNamesForData(widgetAnchorLayoutData{Left: x, Top: y})
	case "layout-size":
		if !strings.EqualFold(slotClassName, "CanvasPanelSlot") {
			return nil
		}
		w, h, err := parseWidgetFloatPair(rawValue)
		if err != nil {
			return nil
		}
		return widgetLayoutRequiredNamesForData(widgetAnchorLayoutData{Right: w, Bottom: h})
	case "layout-anchors":
		if !strings.EqualFold(slotClassName, "CanvasPanelSlot") {
			return nil
		}
		minX, minY, maxX, maxY, err := parseWidgetFloatQuad(rawValue)
		if err != nil {
			return nil
		}
		return widgetLayoutRequiredNamesForData(widgetAnchorLayoutData{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY})
	case "layout-alignment":
		if !strings.EqualFold(slotClassName, "CanvasPanelSlot") {
			return nil
		}
		x, y, err := parseWidgetFloatPair(rawValue)
		if err != nil {
			return nil
		}
		return widgetLayoutRequiredNamesForData(widgetAnchorLayoutData{AlignX: x, AlignY: y})
	case "layout-data":
		if !strings.EqualFold(slotClassName, "CanvasPanelSlot") {
			return nil
		}
		layout, err := parseWidgetAnchorLayoutValue(rawValue)
		if err != nil {
			return nil
		}
		return widgetLayoutRequiredNamesForData(layout)
	default:
		return nil
	}
}

func widgetSlotSupportsGridPosition(slotClassName string) bool {
	return strings.EqualFold(slotClassName, "GridSlot") || strings.EqualFold(slotClassName, "UniformGridSlot")
}

func widgetSlotSupportsChildSize(slotClassName string) bool {
	return strings.EqualFold(slotClassName, "HorizontalBoxSlot") ||
		strings.EqualFold(slotClassName, "VerticalBoxSlot") ||
		strings.EqualFold(slotClassName, "StackBoxSlot")
}

func widgetWriteSlotClassName(asset *uasset.Asset, slotExports []int) (string, error) {
	if asset == nil {
		return "", fmt.Errorf("asset is nil")
	}
	if len(slotExports) == 0 {
		return "", fmt.Errorf("slot exports are empty")
	}
	slotClassName := ""
	for _, exportIdx := range slotExports {
		if exportIdx < 0 || exportIdx >= len(asset.Exports) {
			return "", fmt.Errorf("slot export out of range: %d", exportIdx+1)
		}
		current := strings.TrimSpace(asset.ResolveClassName(asset.Exports[exportIdx]))
		if current == "" {
			return "", fmt.Errorf("slot export %d class name is empty", exportIdx+1)
		}
		if slotClassName == "" {
			slotClassName = current
			continue
		}
		if !strings.EqualFold(slotClassName, current) {
			return "", fmt.Errorf("widget resolves to mixed slot classes (%s, %s)", slotClassName, current)
		}
	}
	return slotClassName, nil
}

func parseWidgetIntValue(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, fmt.Errorf("value requires an integer")
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("parse int %q: %w", raw, err)
	}
	return value, nil
}

func parseWidgetBoolValue(raw string, propertyName string) (bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false, fmt.Errorf("%s requires --value true|false", propertyName)
	}
	value, err := strconv.ParseBool(trimmed)
	if err != nil {
		return false, fmt.Errorf("%s requires --value true|false: %w", propertyName, err)
	}
	return value, nil
}

func widgetLayoutRequiredNamesForData(layout widgetAnchorLayoutData) []string {
	out := []string{"LayoutData", "StructProperty", "AnchorData", "/Script/UMG"}
	if layout.Left != 0 || layout.Top != 0 || layout.Right != 0 || layout.Bottom != 0 {
		out = append(out, "Offsets", "Margin", "FloatProperty", "/Script/SlateCore")
		if layout.Left != 0 {
			out = append(out, "Left")
		}
		if layout.Top != 0 {
			out = append(out, "Top")
		}
		if layout.Right != 0 {
			out = append(out, "Right")
		}
		if layout.Bottom != 0 {
			out = append(out, "Bottom")
		}
	}
	out = append(out, "Anchors", "Minimum", "Maximum", "Vector2D", "/Script/Slate", "/Script/CoreUObject", "Alignment")
	return out
}

func parseWidgetAnchorLayoutValue(raw string) (widgetAnchorLayoutData, error) {
	type payload struct {
		Position  []float64 `json:"position"`
		Size      []float64 `json:"size"`
		Anchors   []float64 `json:"anchors"`
		Alignment []float64 `json:"alignment"`
	}
	var in payload
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &in); err != nil {
		return widgetAnchorLayoutData{}, fmt.Errorf("layout-data expects JSON like {\"position\":[0,0],\"size\":[200,60],\"anchors\":[0.5,0.5,0.5,0.5],\"alignment\":[0.5,0.5]}: %w", err)
	}
	if len(in.Position) != 2 || len(in.Size) != 2 || len(in.Anchors) != 4 || len(in.Alignment) != 2 {
		return widgetAnchorLayoutData{}, fmt.Errorf("layout-data requires position[2], size[2], anchors[4], alignment[2]")
	}
	return widgetAnchorLayoutData{
		Left:   float32(in.Position[0]),
		Top:    float32(in.Position[1]),
		Right:  float32(in.Size[0]),
		Bottom: float32(in.Size[1]),
		MinX:   float32(in.Anchors[0]),
		MinY:   float32(in.Anchors[1]),
		MaxX:   float32(in.Anchors[2]),
		MaxY:   float32(in.Anchors[3]),
		AlignX: float32(in.Alignment[0]),
		AlignY: float32(in.Alignment[1]),
	}, nil
}

func parseWidgetMarginValue(raw string) (widgetMarginData, error) {
	parts := splitWidgetCSV(raw)
	switch len(parts) {
	case 1:
		v, err := parseWidgetFloat32(parts[0])
		if err != nil {
			return widgetMarginData{}, err
		}
		return widgetMarginData{Left: v, Top: v, Right: v, Bottom: v}, nil
	case 4:
		left, err := parseWidgetFloat32(parts[0])
		if err != nil {
			return widgetMarginData{}, err
		}
		top, err := parseWidgetFloat32(parts[1])
		if err != nil {
			return widgetMarginData{}, err
		}
		right, err := parseWidgetFloat32(parts[2])
		if err != nil {
			return widgetMarginData{}, err
		}
		bottom, err := parseWidgetFloat32(parts[3])
		if err != nil {
			return widgetMarginData{}, err
		}
		return widgetMarginData{Left: left, Top: top, Right: right, Bottom: bottom}, nil
	default:
		return widgetMarginData{}, fmt.Errorf("slot-padding requires 1 or 4 comma-separated numbers")
	}
}

func parseWidgetLinearColorValue(raw string, propertyName string) (widgetLinearColor, error) {
	parts := splitWidgetCSV(raw)
	switch len(parts) {
	case 3:
		r, err := parseWidgetFloat32(parts[0])
		if err != nil {
			return widgetLinearColor{}, err
		}
		g, err := parseWidgetFloat32(parts[1])
		if err != nil {
			return widgetLinearColor{}, err
		}
		b, err := parseWidgetFloat32(parts[2])
		if err != nil {
			return widgetLinearColor{}, err
		}
		return widgetLinearColor{R: r, G: g, B: b, A: 1}, nil
	case 4:
		r, err := parseWidgetFloat32(parts[0])
		if err != nil {
			return widgetLinearColor{}, err
		}
		g, err := parseWidgetFloat32(parts[1])
		if err != nil {
			return widgetLinearColor{}, err
		}
		b, err := parseWidgetFloat32(parts[2])
		if err != nil {
			return widgetLinearColor{}, err
		}
		a, err := parseWidgetFloat32(parts[3])
		if err != nil {
			return widgetLinearColor{}, err
		}
		return widgetLinearColor{R: r, G: g, B: b, A: a}, nil
	default:
		return widgetLinearColor{}, fmt.Errorf("%s requires 3 or 4 comma-separated numbers", propertyName)
	}
}

func parseWidgetSlateChildSizeValue(raw string) (widgetSlateChildSize, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return widgetSlateChildSize{}, fmt.Errorf("slot-size requires a value like auto or fill[:weight]")
	}

	parts := strings.SplitN(trimmed, ":", 2)
	mode := strings.ToLower(strings.TrimSpace(parts[0]))
	switch mode {
	case "auto", "automatic":
		return widgetSlateChildSize{
			Rule:  "ESlateSizeRule::Automatic",
			Value: 1,
		}, nil
	case "fill":
		value := float32(1)
		if len(parts) == 2 {
			parsed, err := parseWidgetFloat32(parts[1])
			if err != nil {
				return widgetSlateChildSize{}, fmt.Errorf("slot-size fill weight: %w", err)
			}
			value = parsed
		}
		return widgetSlateChildSize{
			Rule:  "ESlateSizeRule::Fill",
			Value: value,
		}, nil
	default:
		return widgetSlateChildSize{}, fmt.Errorf("unsupported slot-size %q (supported: auto, fill[:weight])", raw)
	}
}

func parseWidgetFloatPair(raw string) (float32, float32, error) {
	parts := splitWidgetCSV(raw)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("value requires 2 comma-separated numbers")
	}
	a, err := parseWidgetFloat32(parts[0])
	if err != nil {
		return 0, 0, err
	}
	b, err := parseWidgetFloat32(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return a, b, nil
}

func parseWidgetFloatListValue(raw string) ([]float32, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("value requires 1 or more numbers")
	}
	if strings.HasPrefix(trimmed, "[") {
		var items []float64
		if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
			return nil, fmt.Errorf("parse float list JSON: %w", err)
		}
		out := make([]float32, 0, len(items))
		for _, item := range items {
			out = append(out, float32(item))
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("value requires 1 or more numbers")
		}
		return out, nil
	}
	parts := splitWidgetCSV(trimmed)
	if len(parts) == 0 {
		return nil, fmt.Errorf("value requires 1 or more numbers")
	}
	out := make([]float32, 0, len(parts))
	for _, part := range parts {
		value, err := parseWidgetFloat32(part)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func parseWidgetStringListValue(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}, nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var values []string
		if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
			return nil, fmt.Errorf("expected JSON string array or comma-separated list: %w", err)
		}
		return values, nil
	}
	parts := strings.Split(trimmed, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		values = append(values, item)
	}
	return values, nil
}

func parseWidgetFloatQuad(raw string) (float32, float32, float32, float32, error) {
	parts := splitWidgetCSV(raw)
	if len(parts) != 4 {
		return 0, 0, 0, 0, fmt.Errorf("value requires 4 comma-separated numbers")
	}
	a, err := parseWidgetFloat32(parts[0])
	if err != nil {
		return 0, 0, 0, 0, err
	}
	b, err := parseWidgetFloat32(parts[1])
	if err != nil {
		return 0, 0, 0, 0, err
	}
	c, err := parseWidgetFloat32(parts[2])
	if err != nil {
		return 0, 0, 0, 0, err
	}
	d, err := parseWidgetFloat32(parts[3])
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return a, b, c, d, nil
}

func splitWidgetCSV(raw string) []string {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseWidgetFloat32(raw string) (float32, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 32)
	if err != nil {
		return 0, fmt.Errorf("parse float %q: %w", raw, err)
	}
	return float32(v), nil
}

func normalizeWidgetHorizontalAlignmentValue(value string) (string, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "EHorizontalAlignment::"))
	switch strings.ToLower(trimmed) {
	case "fill":
		return "EHorizontalAlignment::HAlign_Fill", nil
	case "left":
		return "EHorizontalAlignment::HAlign_Left", nil
	case "center":
		return "EHorizontalAlignment::HAlign_Center", nil
	case "right":
		return "EHorizontalAlignment::HAlign_Right", nil
	default:
		return "", fmt.Errorf("unsupported horizontal alignment %q (supported: Fill, Left, Center, Right)", value)
	}
}

func normalizeWidgetVerticalAlignmentValue(value string) (string, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "EVerticalAlignment::"))
	switch strings.ToLower(trimmed) {
	case "fill":
		return "EVerticalAlignment::VAlign_Fill", nil
	case "top":
		return "EVerticalAlignment::VAlign_Top", nil
	case "center":
		return "EVerticalAlignment::VAlign_Center", nil
	case "bottom":
		return "EVerticalAlignment::VAlign_Bottom", nil
	default:
		return "", fmt.Errorf("unsupported vertical alignment %q (supported: Fill, Top, Center, Bottom)", value)
	}
}

func normalizeWidgetTextJustificationValue(value string) (string, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "ETextJustify::"))
	switch strings.ToLower(trimmed) {
	case "left":
		return "ETextJustify::Left", nil
	case "center":
		return "ETextJustify::Center", nil
	case "right":
		return "ETextJustify::Right", nil
	default:
		return "", fmt.Errorf("unsupported text justification %q (supported: Left, Center, Right)", value)
	}
}

func normalizeWidgetBrushDrawAsValue(value string) (string, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "ESlateBrushDrawType::"))
	switch strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(trimmed, "_", ""), "-", "")) {
	case "nodrawtype":
		return "ESlateBrushDrawType::NoDrawType", nil
	case "box":
		return "ESlateBrushDrawType::Box", nil
	case "border":
		return "ESlateBrushDrawType::Border", nil
	case "image":
		return "ESlateBrushDrawType::Image", nil
	case "roundedbox":
		return "ESlateBrushDrawType::RoundedBox", nil
	default:
		return "", fmt.Errorf("unsupported brush draw-as %q (supported: NoDrawType, Box, Border, Image, RoundedBox)", value)
	}
}

func readWidgetAnchorLayoutData(asset *uasset.Asset, exportIndex int, propertyName string) (widgetAnchorLayoutData, error) {
	current, err := decodeExportRootPropertyValue(asset, exportIndex, propertyName)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "property not found") {
			return widgetAnchorLayoutData{}, nil
		}
		return widgetAnchorLayoutData{}, err
	}
	root, ok := current.(map[string]any)
	if !ok {
		return widgetAnchorLayoutData{}, nil
	}
	if raw, ok := root["rawBase64"].(string); ok && raw != "" {
		return decodeWidgetAnchorLayoutRawBase64(raw)
	}
	value, _ := root["value"].(map[string]any)
	if raw, ok := value["rawBase64"].(string); ok && raw != "" {
		return decodeWidgetAnchorLayoutRawBase64(raw)
	}
	if value == nil {
		value = root
	}
	anchors := widgetDecodedStructValue(value["Anchors"])
	offsets := widgetDecodedStructValue(value["Offsets"])
	alignment := widgetDecodedStructValue(value["Alignment"])
	minimum := widgetDecodedStructValue(anchors["Minimum"])
	maximum := widgetDecodedStructValue(anchors["Maximum"])
	return widgetAnchorLayoutData{
		MinX:   widgetDecodedScalarFloat(minimum["X"]),
		MinY:   widgetDecodedScalarFloat(minimum["Y"]),
		MaxX:   widgetDecodedScalarFloat(maximum["X"]),
		MaxY:   widgetDecodedScalarFloat(maximum["Y"]),
		Left:   widgetDecodedScalarFloat(offsets["Left"]),
		Top:    widgetDecodedScalarFloat(offsets["Top"]),
		Right:  widgetDecodedScalarFloat(offsets["Right"]),
		Bottom: widgetDecodedScalarFloat(offsets["Bottom"]),
		AlignX: widgetDecodedScalarFloat(alignment["X"]),
		AlignY: widgetDecodedScalarFloat(alignment["Y"]),
	}, nil
}

func decodeWidgetAnchorLayoutRawBase64(raw string) (widgetAnchorLayoutData, error) {
	body, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return widgetAnchorLayoutData{}, err
	}
	if len(body) < 40 {
		return widgetAnchorLayoutData{}, fmt.Errorf("layout rawBase64 is too short: %d", len(body))
	}
	return widgetAnchorLayoutData{
		MinX:   math.Float32frombits(binary.LittleEndian.Uint32(body[0:4])),
		MinY:   math.Float32frombits(binary.LittleEndian.Uint32(body[4:8])),
		MaxX:   math.Float32frombits(binary.LittleEndian.Uint32(body[8:12])),
		MaxY:   math.Float32frombits(binary.LittleEndian.Uint32(body[12:16])),
		Left:   math.Float32frombits(binary.LittleEndian.Uint32(body[16:20])),
		Top:    math.Float32frombits(binary.LittleEndian.Uint32(body[20:24])),
		Right:  math.Float32frombits(binary.LittleEndian.Uint32(body[24:28])),
		Bottom: math.Float32frombits(binary.LittleEndian.Uint32(body[28:32])),
		AlignX: math.Float32frombits(binary.LittleEndian.Uint32(body[32:36])),
		AlignY: math.Float32frombits(binary.LittleEndian.Uint32(body[36:40])),
	}, nil
}

func buildWidgetAnchorLayoutValueJSON(layout widgetAnchorLayoutData) (string, error) {
	return marshalJSONValue(map[string]any{
		"structType": "AnchorData",
		"value":      buildWidgetAnchorLayoutStructFields(layout),
	})
}

func buildWidgetMarginValueJSON(margin widgetMarginData) (string, error) {
	return marshalJSONValue(map[string]any{
		"structType": "Margin",
		"value": map[string]any{
			"Left": map[string]any{
				"offset": 0,
				"type":   "FloatProperty",
				"value":  margin.Left,
			},
			"Top": map[string]any{
				"offset": 29,
				"type":   "FloatProperty",
				"value":  margin.Top,
			},
			"Right": map[string]any{
				"offset": 58,
				"type":   "FloatProperty",
				"value":  margin.Right,
			},
			"Bottom": map[string]any{
				"offset": 87,
				"type":   "FloatProperty",
				"value":  margin.Bottom,
			},
		},
	})
}

func buildWidgetLinearColorValueJSON(color widgetLinearColor) (string, error) {
	return marshalJSONValue(map[string]any{
		"structType": "LinearColor",
		"value": map[string]any{
			"r": color.R,
			"g": color.G,
			"b": color.B,
			"a": color.A,
		},
	})
}

func buildWidgetAnchorLayoutStructFields(layout widgetAnchorLayoutData) map[string]any {
	return map[string]any{
		"Offsets": map[string]any{
			"offset": 0,
			"type":   "StructProperty(Margin(/Script/SlateCore))",
			"value": map[string]any{
				"structType": "Margin",
				"value":      buildWidgetAnchorOffsetFields(layout),
			},
		},
		"Anchors": map[string]any{
			"offset": 1,
			"type":   "StructProperty(Anchors(/Script/Slate))",
			"value": map[string]any{
				"structType": "Anchors",
				"value": map[string]any{
					"Minimum": buildWidgetVector2DField(layout.MinX, layout.MinY, 0),
					"Maximum": buildWidgetVector2DField(layout.MaxX, layout.MaxY, 1),
				},
			},
		},
		"Alignment": buildWidgetVector2DField(layout.AlignX, layout.AlignY, 2),
	}
}

func buildWidgetAnchorOffsetFields(layout widgetAnchorLayoutData) map[string]any {
	fields := map[string]any{}
	order := 0
	appendFloat := func(name string, value float32) {
		if value == 0 {
			return
		}
		fields[name] = map[string]any{
			"offset": order,
			"type":   "FloatProperty",
			"value":  value,
		}
		order++
	}
	appendFloat("Left", layout.Left)
	appendFloat("Top", layout.Top)
	appendFloat("Right", layout.Right)
	appendFloat("Bottom", layout.Bottom)
	return fields
}

func buildWidgetVector2DField(x, y float32, offset int) map[string]any {
	return map[string]any{
		"flags":  8,
		"offset": offset,
		"type":   "StructProperty(Vector2D(/Script/CoreUObject))",
		"value": map[string]any{
			"structType": "Vector2D",
			"value": map[string]any{
				"x": x,
				"y": y,
			},
		},
	}
}

func buildWidgetVector2DValueJSON(x, y float32) (string, error) {
	return marshalJSONValue(map[string]any{
		"structType": "Vector2D",
		"value": map[string]any{
			"x": x,
			"y": y,
		},
	})
}

func buildWidgetSlateChildSizeValueJSON(size widgetSlateChildSize) (string, error) {
	return marshalJSONValue(map[string]any{
		"structType": "SlateChildSize",
		"value": map[string]any{
			"SizeRule": map[string]any{
				"type": "ByteProperty(ESlateSizeRule(/Script/UMG))",
				"value": map[string]any{
					"enumType": "ESlateSizeRule",
					"value":    size.Rule,
				},
			},
			"Value": map[string]any{
				"type":  "FloatProperty",
				"value": size.Value,
			},
		},
	})
}

func widgetDecodedStructValue(raw any) map[string]any {
	current := raw
	for {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		if value, ok := widgetDecodedStructField(m, "value"); ok {
			if inner, ok := value.(map[string]any); ok {
				current = inner
				continue
			}
		}
		return m
	}
}

func widgetDecodedStructField(m map[string]any, key string) (any, bool) {
	if m == nil {
		return nil, false
	}
	if value, ok := m[key]; ok {
		return value, true
	}
	for existingKey, value := range m {
		if strings.EqualFold(existingKey, key) {
			return value, true
		}
	}
	return nil, false
}

func widgetDecodedScalarFloat(raw any) float32 {
	switch v := raw.(type) {
	case float64:
		return float32(v)
	case float32:
		return v
	case int:
		return float32(v)
	case map[string]any:
		if value, ok := widgetDecodedStructField(v, "value"); ok {
			return widgetDecodedScalarFloat(value)
		}
	}
	return 0
}

func widgetDecodedScalarBool(raw any) (bool, bool) {
	switch v := raw.(type) {
	case bool:
		return v, true
	case map[string]any:
		if value, ok := widgetDecodedStructField(v, "value"); ok {
			return widgetDecodedScalarBool(value)
		}
	}
	return false, false
}

// ---------------------------------------------------------------------------
// brush-image multi-phase pipeline
// ---------------------------------------------------------------------------

func applyBrushImageWrite(asset *uasset.Asset, opts uasset.ParseOptions, target widgetWriteTarget, value string) ([]byte, *uasset.Asset, []string, []map[string]any, error) {
	packagePath, objectName, ok := parseTexturePackagePath(strings.TrimSpace(value))
	if !ok {
		return nil, nil, nil, nil, fmt.Errorf("brush-image requires a full texture package path like /Game/Path/TextureName, got %q", value)
	}

	// Phase 1: Insert texture import entries FIRST (before NameMap reorder,
	// so the decode→encode cycle in InsertImportEntries operates on the
	// original NameMap and produces byte-identical property re-encoding).
	_, workingAsset, importAddedNames, err := appendTextureImportIfMissing(asset, opts, packagePath, objectName)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("insert texture import: %w", err)
	}
	addedNames := append([]string(nil), importAddedNames...)

	// Phase 2: Insert NameMap entries in alphabetical order.
	_, workingAsset, nameAddedNames, err := insertBrushImageNames(
		workingAsset,
		opts,
		brushImageRequiredPrefixNames(),
		brushImageRequiredSuffixNames(packagePath, objectName),
	)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("insert brush-image names: %w", err)
	}
	addedNames = append(addedNames, nameAddedNames...)

	// Resolve the Texture2D import index for ResourceObject.
	textureImportIdx := int32(0)
	for i, imp := range workingAsset.Imports {
		if textureImportIsSupported(workingAsset, imp) &&
			strings.EqualFold(imp.ObjectName.Display(workingAsset.Names), objectName) {
			textureImportIdx = int32(-(i + 1))
			break
		}
	}
	if textureImportIdx == 0 {
		return nil, nil, nil, nil, fmt.Errorf("texture import %q not found after insertion", objectName)
	}

	// Phase 3: Add Brush property to each target widget export.
	updates := make([]map[string]any, 0, len(target.Exports))
	var workingBytes []byte
	for _, exportIdx := range target.Exports {
		valueJSON := buildBrushPropertySetValueJSON(textureImportIdx)
		outBytes, result, err := applyPropertySetCommandWithPostprocess(workingAsset, exportIdx, "Brush", valueJSON, opts)
		if err != nil && strings.Contains(err.Error(), "property not found") {
			specJSON := buildBrushPropertyAddSpec(textureImportIdx, workingAsset)
			outBytes, result, err = applyPropertyAddAsSetResult(workingAsset, exportIdx, specJSON, opts)
		}
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("write Brush on export %d: %w", exportIdx+1, err)
		}
		workingBytes = outBytes
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("reparse after Brush write: %w", err)
		}
		updates = append(updates, map[string]any{
			"export":   exportIdx + 1,
			"path":     "Brush",
			"oldValue": result.OldValue,
			"newValue": result.NewValue,
		})
	}

	// Phase 4: Update DependsMap for the designer-tree export.
	designerIdx := selectBrushDependsExport(workingAsset, target)
	if designerIdx >= 0 {
		_, workingAsset, err = syncBrushDependency(workingAsset, opts, designerIdx, textureImportIdx)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("sync brush depends: %w", err)
		}
	}

	_, workingAsset, err = normalizeBrushImageDesignerWidgetTreeFlags(workingAsset, opts, target.BlueprintExport)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("normalize brush-image designer WidgetTree flags: %w", err)
	}
	return workingAsset.Raw.Bytes, workingAsset, addedNames, updates, nil
}

func applyButtonBrushStyleWrite(asset *uasset.Asset, opts uasset.ParseOptions, target widgetWriteTarget, normalizedProperty, value string) ([]byte, *uasset.Asset, []string, []map[string]any, string, error) {
	propertySpec, ok := normalizeWidgetButtonBrushProperty(normalizedProperty)
	if !ok {
		return nil, nil, nil, nil, "", fmt.Errorf("unsupported button brush property %q", normalizedProperty)
	}
	if !strings.EqualFold(target.ClassName, "Button") {
		return nil, nil, nil, nil, "", fmt.Errorf("%s requires a Button widget, got %s", propertySpec.CanonicalProperty, target.ClassName)
	}

	workingAsset := asset
	addedNames := []string{}
	textureImportIdx := int32(0)
	var err error
	var importAddedNames []string
	var nameAddedNames []string
	var ensuredNames []string
	switch propertySpec.BrushField {
	case "image":
		packagePath, objectName, ok := parseTexturePackagePath(strings.TrimSpace(value))
		if !ok {
			return nil, nil, nil, nil, "", fmt.Errorf("%s requires a full texture package path like /Game/Path/TextureName, got %q", propertySpec.CanonicalProperty, value)
		}

		_, workingAsset, importAddedNames, err = appendTextureImportIfMissing(asset, opts, packagePath, objectName)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("insert texture import: %w", err)
		}
		addedNames = append(addedNames, importAddedNames...)

		_, workingAsset, nameAddedNames, err = insertBrushImageNames(
			workingAsset,
			opts,
			buttonBrushPropertyRequiredPrefixNames(propertySpec),
			brushImageRequiredSuffixNames(packagePath, objectName),
		)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("insert button brush-image names: %w", err)
		}
		addedNames = append(addedNames, nameAddedNames...)
		_, workingAsset, ensuredNames, err = ensureNameEntriesPresentSorted(workingAsset, opts, buttonBrushPropertyRequiredPrefixNames(propertySpec))
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("ensure button brush-image names: %w", err)
		}
		addedNames = append(addedNames, ensuredNames...)

		if idx, found := findTextureImportByPath(workingAsset, packagePath); found {
			textureImportIdx = int32(-idx)
		}
		if textureImportIdx == 0 {
			return nil, nil, nil, nil, "", fmt.Errorf("texture import %q not found after insertion", packagePath)
		}
	default:
		requiredNames := buttonBrushPropertyRequiredPrefixNames(propertySpec)
		_, workingAsset, addedNames, err = ensureNameEntriesPresentSorted(workingAsset, opts, requiredNames)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("ensure button brush names: %w", err)
		}
	}
	var guaranteedNames []string
	_, workingAsset, guaranteedNames, err = ensureNameEntriesPresent(workingAsset, opts, buttonBrushPropertyRequiredPrefixNames(propertySpec))
	if err != nil {
		return nil, nil, nil, nil, "", fmt.Errorf("guarantee button brush names: %w", err)
	}
	addedNames = append(addedNames, guaranteedNames...)

	updates := make([]map[string]any, 0, len(target.Exports))
	propertyPath := "WidgetStyle." + propertySpec.StateField
	switch propertySpec.BrushField {
	case "image":
		propertyPath += ".ResourceObject"
	case "tint":
		propertyPath += ".TintColor"
	case "image-size":
		propertyPath += ".ImageSize"
	case "draw-as":
		propertyPath += ".DrawAs"
	}
	if buttonBrushWriteShouldNoOp(asset, target, propertySpec, value) {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil, nil, propertyPath, nil
	}
	for _, exportIdx := range target.Exports {
		valueJSON, err := buildButtonStyleValueJSON(workingAsset, exportIdx, propertySpec, value, textureImportIdx)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("build WidgetStyle on export %d: %w", exportIdx+1, err)
		}
		if strings.TrimSpace(valueJSON) == "" {
			continue
		}
		addSpecJSON := buildButtonStyleAddSpec(valueJSON)
		outBytes, result, err := applyPropertySetCommandWithPostprocess(workingAsset, exportIdx, "WidgetStyle", valueJSON, opts)
		if err != nil && strings.Contains(err.Error(), "property not found") {
			outBytes, result, err = applyPropertyAddAsSetResultBefore(workingAsset, exportIdx, addSpecJSON, widgetButtonStyleAddBeforeProperty(workingAsset, exportIdx), opts)
		}
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("write WidgetStyle on export %d: %w", exportIdx+1, err)
		}
		workingAsset, err = uasset.ParseBytes(outBytes, opts)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("reparse after WidgetStyle write: %w", err)
		}
		updates = append(updates, map[string]any{
			"export":   exportIdx + 1,
			"path":     propertyPath,
			"oldValue": result.OldValue,
			"newValue": result.NewValue,
		})
	}

	if propertySpec.BrushField == "image" {
		designerIdx := selectBrushDependsExport(workingAsset, target)
		if designerIdx >= 0 {
			_, workingAsset, err = syncBrushDependency(workingAsset, opts, designerIdx, textureImportIdx)
			if err != nil {
				return nil, nil, nil, nil, "", fmt.Errorf("sync button brush depends: %w", err)
			}
		}
	}

	_, workingAsset, err = normalizeBrushImageDesignerWidgetTreeFlags(workingAsset, opts, target.BlueprintExport)
	if err != nil {
		return nil, nil, nil, nil, "", fmt.Errorf("normalize button brush designer WidgetTree flags: %w", err)
	}
	return workingAsset.Raw.Bytes, workingAsset, addedNames, updates, propertyPath, nil
}

func buttonBrushWriteShouldNoOp(asset *uasset.Asset, target widgetWriteTarget, spec widgetButtonBrushProperty, rawValue string) bool {
	if asset == nil || spec.BrushField != "draw-as" || spec.StateField != "Normal" {
		return false
	}
	drawAs, err := normalizeWidgetBrushDrawAsValue(rawValue)
	if err != nil || drawAs != "ESlateBrushDrawType::RoundedBox" {
		return false
	}
	if len(target.Exports) == 0 {
		return false
	}
	for _, exportIdx := range target.Exports {
		_, err := decodeExportRootPropertyValue(asset, exportIdx, "WidgetStyle")
		if err == nil {
			return false
		}
		if !strings.Contains(strings.ToLower(err.Error()), "property not found") {
			return false
		}
	}
	return true
}

// insertBrushImageNames inserts new names into the NameMap in alphabetical
// order, respecting the prefix/suffix boundary (NamesReferencedFromExportDataCount).
func insertBrushImageNames(asset *uasset.Asset, opts uasset.ParseOptions, prefixNames, suffixNames []string) ([]byte, *uasset.Asset, []string, error) {
	boundary := int(asset.Summary.NamesReferencedFromExportDataCount)
	if boundary < 0 {
		boundary = 0
	}
	if boundary > len(asset.Names) {
		boundary = len(asset.Names)
	}

	oldNames := append([]uasset.NameEntry(nil), asset.Names...)
	prefix := append([]uasset.NameEntry(nil), oldNames[:boundary]...)
	suffix := append([]uasset.NameEntry(nil), oldNames[boundary:]...)
	addedNames := make([]string, 0, len(prefixNames)+len(suffixNames))
	prefixAdded := 0

	for _, name := range prefixNames {
		if findNameIndexByValue(oldNames, name) >= 0 {
			continue
		}
		if findNameIndexByValue(prefix, name) >= 0 {
			continue
		}
		nonCase, caseHash := edit.ComputeNameEntryHashesUE56(name)
		entry := uasset.NameEntry{Value: name, NonCaseHash: nonCase, CasePreservingHash: caseHash}
		pos := sort.Search(len(prefix), func(i int) bool {
			return strings.ToLower(prefix[i].Value) >= strings.ToLower(name)
		})
		prefix = slices.Insert(prefix, pos, entry)
		addedNames = append(addedNames, name)
		prefixAdded++
	}
	for _, name := range suffixNames {
		if findNameIndexByValue(oldNames, name) >= 0 {
			continue
		}
		if findNameIndexByValue(prefix, name) >= 0 {
			continue
		}
		nonCase, caseHash := edit.ComputeNameEntryHashesUE56(name)
		entry := uasset.NameEntry{Value: name, NonCaseHash: nonCase, CasePreservingHash: caseHash}
		pos := sort.Search(len(suffix), func(i int) bool {
			return strings.ToLower(suffix[i].Value) >= strings.ToLower(name)
		})
		suffix = slices.Insert(suffix, pos, entry)
		addedNames = append(addedNames, name)
	}

	// Re-sort the suffix group alphabetically. Names added by earlier phases
	// (e.g. ensureNameEntriesPresent in appendTextureImportIfMissing) may have
	// been appended at the end rather than inserted in order.
	sort.SliceStable(suffix, func(i, j int) bool {
		return strings.ToLower(suffix[i].Value) < strings.ToLower(suffix[j].Value)
	})

	newNames := make([]uasset.NameEntry, 0, len(prefix)+len(suffix))
	newNames = append(newNames, prefix...)
	newNames = append(newNames, suffix...)

	// Check whether the NameMap actually changed (new names or reorder).
	nameMapChanged := len(newNames) != len(oldNames)
	if !nameMapChanged {
		for i := range oldNames {
			if oldNames[i].Value != newNames[i].Value {
				nameMapChanged = true
				break
			}
		}
	}
	if !nameMapChanged {
		return append([]byte(nil), asset.Raw.Bytes...), asset, addedNames, nil
	}

	// Build remap and rewrite all NameRef references.
	indexRemap, err := edit.BuildNameIndexRemapAllowInsertedNewEntries(oldNames, newNames)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build name remap: %w", err)
	}
	workingBytes, err := edit.RewriteImportExportNameRefs(asset, indexRemap)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("rewrite import/export name refs: %w", err)
	}

	remapped := *asset
	remapped.Raw.Bytes = workingBytes
	workingBytes, err = edit.RewriteNameMap(&remapped, newNames)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("rewrite name map: %w", err)
	}
	workingAsset, err := uasset.ParseBytes(workingBytes, opts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("reparse after name map rewrite: %w", err)
	}

	// Remap NameRef indices inside export payloads.
	exportMutations, err := edit.BuildExportNameRemapMutations(asset, workingAsset, indexRemap, "", "")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build export payload remap mutations: %w", err)
	}
	if len(exportMutations) > 0 {
		workingBytes, err = edit.RewriteAsset(workingAsset, exportMutations)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("rewrite export payloads: %w", err)
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("reparse after export payload rewrite: %w", err)
		}
	}

	// Patch instanced NameRefs (Number != 0) in export tails.
	workingBytes, workingAsset, err = edit.PatchInstancedTailNameRefs(asset, workingAsset, indexRemap, opts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("patch instanced tail name refs: %w", err)
	}

	// Expand NamesReferencedFromExportDataCount.
	newBoundary := int32(boundary + prefixAdded)
	if workingAsset.Summary.NamesReferencedFromExportDataCount < newBoundary {
		workingBytes, err = edit.ExpandNamesReferencedFromExportDataCount(workingAsset, newBoundary)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("expand names referenced count: %w", err)
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("reparse after ref count expand: %w", err)
		}
	}

	return workingBytes, workingAsset, addedNames, nil
}

func brushImageRequiredPrefixNames() []string {
	return []string{
		"/Script/SlateCore",
		"Brush",
		"ByteProperty",
		"DeprecateSlateVector2D",
		"ESlateBrushImageType",
		"ESlateBrushImageType::FullColor",
		"ImageSize",
		"ImageType",
		"ObjectProperty",
		"ResourceObject",
		"SlateBrush",
		"StructProperty",
	}
}

func buttonBrushPropertyRequiredPrefixNames(spec widgetButtonBrushProperty) []string {
	out := []string{"WidgetStyle", "ButtonStyle", "bool", "BoolProperty", spec.StateField}
	switch spec.BrushField {
	case "image":
		out = append(out,
			"/Script/SlateCore",
			"ByteProperty",
			"DeprecateSlateVector2D",
			"ESlateBrushImageType",
			"ESlateBrushImageType::FullColor",
			"ImageSize",
			"ImageType",
			"ObjectProperty",
			"ResourceObject",
			"SlateBrush",
			"StructProperty",
		)
	case "tint":
		out = append(out,
			"/Script/SlateCore",
			"ButtonStyle",
			"LinearColor",
			"Normal",
			"SlateBrush",
			"SlateColor",
			"SpecifiedColor",
			"TintColor",
			"WidgetStyle",
		)
	case "image-size":
		out = append(out,
			"/Script/SlateCore",
			"ButtonStyle",
			"ImageSize",
			"DeprecateSlateVector2D",
			"Normal",
			"SlateBrush",
			"WidgetStyle",
		)
	case "draw-as":
		out = append(out,
			"ButtonStyle",
			"DrawAs",
			"ESlateBrushDrawType",
			"ESlateBrushDrawType::NoDrawType",
			"ESlateBrushDrawType::Box",
			"ESlateBrushDrawType::Border",
			"ESlateBrushDrawType::Image",
			"ESlateBrushDrawType::RoundedBox",
			"Normal",
			"SlateBrush",
			"WidgetStyle",
		)
	}
	return out
}

func brushImageRequiredSuffixNames(packagePath, objectName string) []string {
	return []string{packagePath, objectName, "Texture2D"}
}

// appendTextureImportIfMissing adds Package + Texture2D imports if not present.
func appendTextureImportIfMissing(asset *uasset.Asset, opts uasset.ParseOptions, packagePath, objectName string) ([]byte, *uasset.Asset, []string, error) {
	return appendObjectImportIfMissing(asset, opts, packagePath, objectName, "/Script/Engine", "Texture2D")
}

func appendFontImportIfMissing(asset *uasset.Asset, opts uasset.ParseOptions, packagePath, objectName string) ([]byte, *uasset.Asset, []string, error) {
	if idx, found := findObjectImportByPath(asset, "/Script/Engine", "Font", packagePath); found && idx > 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil, nil
	}

	requiredNames := []string{
		"None",
		"/Script/CoreUObject",
		"Package",
		"/Script/Engine",
		"Font",
		packagePath,
		objectName,
	}
	workingBytes, workingAsset, addedNames, err := ensureNameEntriesPresentSortedSuffix(asset, opts, requiredNames)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("ensure font import names: %w", err)
	}

	if idx, found := findObjectImportByPath(workingAsset, "/Script/Engine", "Font", packagePath); found && idx > 0 {
		return workingBytes, workingAsset, addedNames, nil
	}

	packageImportIndex, packageExists := findPackageImportByObjectPath(workingAsset, packagePath)
	if !packageExists {
		packageEntry, err := buildImportAddEntry(workingAsset, "/Script/CoreUObject", "Package", 0, packagePath)
		if err != nil {
			return nil, nil, nil, err
		}
		packageInsertAt := preferredPackageImportInsertPos(workingAsset)
		workingBytes, err = edit.InsertImportEntries(workingAsset, packageInsertAt, []uasset.ImportEntry{packageEntry})
		if err != nil {
			return nil, nil, nil, fmt.Errorf("insert font package import: %w", err)
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("reparse rewritten asset: %w", err)
		}
		packageImportIndex = packageInsertAt + 1
	}

	objectInsertAt := packageImportIndex - 1
	objectEntry, err := buildImportAddEntry(workingAsset, "/Script/Engine", "Font", uasset.PackageIndex(-packageImportIndex), objectName)
	if err != nil {
		return nil, nil, nil, err
	}
	workingBytes, err = edit.InsertImportEntries(workingAsset, objectInsertAt, []uasset.ImportEntry{objectEntry})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("insert font import: %w", err)
	}
	workingAsset, err = uasset.ParseBytes(workingBytes, opts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("reparse rewritten asset: %w", err)
	}
	return workingBytes, workingAsset, addedNames, nil
}

func appendDataTableImportIfMissing(asset *uasset.Asset, opts uasset.ParseOptions, packagePath, objectName string) ([]byte, *uasset.Asset, []string, error) {
	return appendObjectImportIfMissing(asset, opts, packagePath, objectName, "/Script/Engine", "DataTable")
}

func appendBlueprintGeneratedClassImportIfMissing(asset *uasset.Asset, opts uasset.ParseOptions, packagePath, objectName string) ([]byte, *uasset.Asset, []string, error) {
	return appendObjectImportIfMissing(asset, opts, packagePath, objectName, "/Script/Engine", "BlueprintGeneratedClass")
}

func appendWidgetBlueprintGeneratedClassImportIfMissing(asset *uasset.Asset, opts uasset.ParseOptions, packagePath, objectName string) ([]byte, *uasset.Asset, []string, error) {
	if idx, found := findObjectImportByPath(asset, "/Script/UMG", "WidgetBlueprintGeneratedClass", packagePath); found && idx > 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil, nil
	}

	requiredNames := []string{
		"None",
		"/Script/CoreUObject",
		"Package",
		"/Script/UMG",
		"WidgetBlueprintGeneratedClass",
		packagePath,
		objectName,
	}
	_, workingAsset, addedNames, err := ensureNameEntriesPresentSortedSuffix(asset, opts, requiredNames)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("ensure widget blueprint generated class import names: %w", err)
	}
	var workingBytes []byte

	packageImportIndex, packageExists := findPackageImportByObjectPath(workingAsset, packagePath)
	if !packageExists {
		entry, err := buildImportAddEntry(workingAsset, "/Script/CoreUObject", "Package", 0, packagePath)
		if err != nil {
			return nil, nil, nil, err
		}
		insertAt := preferredPackageImportInsertPos(workingAsset)
		workingBytes, err = edit.InsertImportEntries(workingAsset, insertAt, []uasset.ImportEntry{entry})
		if err != nil {
			return nil, nil, nil, fmt.Errorf("insert widget blueprint package import: %w", err)
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("reparse rewritten asset: %w", err)
		}
		packageImportIndex = insertAt + 1
	}

	objectEntry, err := buildImportAddEntry(workingAsset, "/Script/UMG", "WidgetBlueprintGeneratedClass", uasset.PackageIndex(-packageImportIndex), objectName)
	if err != nil {
		return nil, nil, nil, err
	}
	insertAt := preferredWidgetBlueprintGeneratedClassImportInsertPos(workingAsset)
	workingBytes, err = edit.InsertImportEntries(workingAsset, insertAt, []uasset.ImportEntry{objectEntry})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("insert WidgetBlueprintGeneratedClass import: %w", err)
	}
	workingAsset, err = uasset.ParseBytes(workingBytes, opts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("reparse rewritten asset: %w", err)
	}
	return workingBytes, workingAsset, addedNames, nil
}

func preferredWidgetBlueprintGeneratedClassImportInsertPos(asset *uasset.Asset) int {
	for i, imp := range asset.Imports {
		if objectImportMatchesClass(asset, imp, "/Script/UMG", "WidgetTree") {
			return i
		}
	}
	return preferredObjectImportInsertPos(asset)
}

func appendObjectImportIfMissing(asset *uasset.Asset, opts uasset.ParseOptions, packagePath, objectName, classPackage, className string) ([]byte, *uasset.Asset, []string, error) {
	if idx, found := findObjectImportByPath(asset, classPackage, className, packagePath); found && idx > 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil, nil
	}
	workingBytes, workingAsset, addedNames, _, _, err := appendObjectImport(asset, opts, packagePath, objectName, classPackage, className)
	if err != nil {
		return nil, nil, nil, err
	}
	return workingBytes, workingAsset, addedNames, nil
}

func buildBrushPropertyAddSpec(textureImportIdx int32, asset *uasset.Asset) string {
	return fmt.Sprintf(`{"name":"Brush","type":"StructProperty(SlateBrush(/Script/SlateCore))","value":{"structType":"SlateBrush","value":{"ImageType":{"offset":0,"type":"ByteProperty(ESlateBrushImageType(/Script/SlateCore))","value":{"enumType":"ESlateBrushImageType","value":"ESlateBrushImageType::FullColor"}},"ImageSize":{"flags":8,"offset":57,"type":"StructProperty(DeprecateSlateVector2D(/Script/SlateCore))","value":{"structType":"DeprecateSlateVector2D","value":{"rawBase64":"AAAARAAAAEQ=","structType":"DeprecateSlateVector2D"}}},"ResourceObject":{"offset":114,"type":"ObjectProperty","value":{"index":%d,"resolved":"%s"}}}}}`,
		textureImportIdx, asset.ParseIndex(uasset.PackageIndex(textureImportIdx)))
}

func buildBrushPropertySetValueJSON(textureImportIdx int32) string {
	return fmt.Sprintf(`{"structType":"SlateBrush","value":{"ImageType":{"value":{"value":"ESlateBrushImageType::FullColor"}},"ImageSize":{"value":{"rawBase64":"AAAARAAAAEQ="}},"ResourceObject":{"value":{"index":%d}}}}`, textureImportIdx)
}

func buildButtonStyleImageValueJSON(textureImportIdx int32) string {
	return fmt.Sprintf(`{"structType":"ButtonStyle","value":{"Normal":{"type":"StructProperty(SlateBrush(/Script/SlateCore))","value":{"structType":"SlateBrush","value":{"ImageType":{"offset":0,"type":"ByteProperty(ESlateBrushImageType(/Script/SlateCore))","value":{"enumType":"ESlateBrushImageType","value":"ESlateBrushImageType::FullColor"}},"ImageSize":{"flags":8,"offset":57,"type":"StructProperty(DeprecateSlateVector2D(/Script/SlateCore))","value":{"structType":"DeprecateSlateVector2D","value":{"rawBase64":"AAAARAAAAEQ=","structType":"DeprecateSlateVector2D"}}},"ResourceObject":{"offset":114,"type":"ObjectProperty","value":{"index":%d}}}}}}}`, textureImportIdx)
}

func widgetButtonStyleAddBeforeProperty(asset *uasset.Asset, exportIndex int) string {
	if asset == nil || exportIndex < 0 || exportIndex >= len(asset.Exports) {
		return "Slot"
	}
	props := asset.ParseExportProperties(exportIndex)
	for _, prop := range props.Properties {
		name := prop.Name.Display(asset.Names)
		if strings.EqualFold(name, "DisplayLabel") {
			return "DisplayLabel"
		}
	}
	return "Slot"
}

func buildButtonStyleValueJSON(asset *uasset.Asset, exportIndex int, spec widgetButtonBrushProperty, rawValue string, textureImportIdx int32) (string, error) {
	payload := map[string]any{
		"structType": "ButtonStyle",
		"value":      map[string]any{},
	}

	current, err := decodeExportRootPropertyValue(asset, exportIndex, "WidgetStyle")
	if err == nil {
		if currentMap, ok := current.(map[string]any); ok {
			payload = cloneAnyMapLocal(currentMap)
			payload["structType"] = "ButtonStyle"
		}
	}

	fields, ok := payload["value"].(map[string]any)
	if ok {
		fields = cloneAnyMapLocal(fields)
	} else {
		fields = map[string]any{}
	}

	brushField := buildSlateBrushStructFieldFromExisting(fields[spec.StateField])
	brushValue, ok := brushField["value"].(map[string]any)
	if ok {
		brushValue = cloneAnyMapLocal(brushValue)
	} else {
		brushValue = map[string]any{}
	}
	brushValue["structType"] = "SlateBrush"

	brushFields, ok := brushValue["value"].(map[string]any)
	if ok {
		brushFields = cloneAnyMapLocal(brushFields)
	} else {
		brushFields = map[string]any{}
	}

	switch spec.BrushField {
	case "image":
		if len(fields) == 0 {
			return buildButtonStyleImageValueJSON(textureImportIdx), nil
		}
		brushFields["ResourceObject"] = buildSlateBrushObjectField(textureImportIdx)
		if _, exists := brushFields["ImageType"]; !exists {
			brushFields["ImageType"] = buildSlateBrushImageTypeField()
		}
		if _, exists := brushFields["ImageSize"]; !exists {
			brushFields["ImageSize"] = buildSlateBrushImageSizeField(512, 512)
		}
	case "tint":
		color, err := parseWidgetLinearColorValue(rawValue, spec.CanonicalProperty)
		if err != nil {
			return "", err
		}
		brushFields["TintColor"] = buildSlateColorField(color)
	case "image-size":
		x, y, err := parseWidgetFloatPair(rawValue)
		if err != nil {
			return "", fmt.Errorf("%s: %w", spec.CanonicalProperty, err)
		}
		brushFields["ImageSize"] = buildSlateBrushImageSizeField(x, y)
	case "draw-as":
		drawAs, err := normalizeWidgetBrushDrawAsValue(rawValue)
		if err != nil {
			return "", err
		}
		if len(fields) == 0 && spec.StateField == "Normal" && drawAs == "ESlateBrushDrawType::RoundedBox" {
			return "", nil
		}
		brushFields["DrawAs"] = buildSlateBrushDrawAsField(drawAs)
	default:
		return "", fmt.Errorf("unsupported button brush field %q", spec.BrushField)
	}

	brushValue["value"] = brushFields
	brushField["value"] = brushValue
	fields[spec.StateField] = brushField
	payload["value"] = fields
	payload["structType"] = "ButtonStyle"
	return marshalJSONValue(payload)
}

func buildButtonStyleAddSpec(valueJSON string) string {
	return fmt.Sprintf(`{"name":"WidgetStyle","type":"StructProperty(ButtonStyle(/Script/SlateCore))","value":%s}`, valueJSON)
}

func buildSlateBrushStructFieldFromExisting(raw any) map[string]any {
	out := map[string]any{
		"type": "StructProperty(SlateBrush(/Script/SlateCore))",
		"value": map[string]any{
			"structType": "SlateBrush",
			"value":      map[string]any{},
		},
	}
	existing := widgetDecodedStructValue(raw)
	if len(existing) == 0 {
		return out
	}
	if value, ok := out["value"].(map[string]any); ok {
		value["value"] = cloneAnyMapLocal(existing)
		out["value"] = value
	}
	return out
}

func buildSlateBrushObjectField(textureImportIdx int32) map[string]any {
	return map[string]any{
		"type": "ObjectProperty",
		"value": map[string]any{
			"index": textureImportIdx,
		},
	}
}

func buildSlateBrushImageTypeField() map[string]any {
	return map[string]any{
		"type": "ByteProperty(ESlateBrushImageType(/Script/SlateCore))",
		"value": map[string]any{
			"enumType": "ESlateBrushImageType",
			"value":    "ESlateBrushImageType::FullColor",
		},
	}
}

func buildSlateBrushImageSizeField(x, y float32) map[string]any {
	return map[string]any{
		"flags": 8,
		"type":  "StructProperty(DeprecateSlateVector2D(/Script/SlateCore))",
		"value": map[string]any{
			"structType": "DeprecateSlateVector2D",
			"value": map[string]any{
				"rawBase64":  buildWidgetDeprecateSlateVector2DRawBase64(x, y),
				"structType": "DeprecateSlateVector2D",
			},
		},
	}
}

func buildSlateBrushDrawAsField(drawAs string) map[string]any {
	return map[string]any{
		"type": "ByteProperty(ESlateBrushDrawType(/Script/SlateCore))",
		"value": map[string]any{
			"enumType": "ESlateBrushDrawType",
			"value":    drawAs,
		},
	}
}

func buildSlateColorField(color widgetLinearColor) map[string]any {
	return map[string]any{
		"type": "StructProperty(SlateColor(/Script/SlateCore))",
		"value": map[string]any{
			"structType": "SlateColor",
			"value": map[string]any{
				"SpecifiedColor": map[string]any{
					"flags": 8,
					"type":  "StructProperty(LinearColor(/Script/CoreUObject))",
					"value": map[string]any{
						"structType": "LinearColor",
						"value": map[string]any{
							"r": color.R,
							"g": color.G,
							"b": color.B,
							"a": color.A,
						},
					},
				},
			},
		},
	}
}

func buildWidgetDeprecateSlateVector2DRawBase64(x, y float32) string {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint32(buf[0:4], math.Float32bits(x))
	binary.LittleEndian.PutUint32(buf[4:8], math.Float32bits(y))
	return base64.StdEncoding.EncodeToString(buf)
}

// selectBrushDependsExport picks the designer-tree export for DependsMap update.
// The designer tree widget's outer chain leads to the WidgetBlueprint export.
// The generated tree widget's outer chain leads to the generated class export.
func selectBrushDependsExport(asset *uasset.Asset, target widgetWriteTarget) int {
	if len(target.Exports) == 0 {
		return -1
	}
	for _, exportIdx := range target.Exports {
		if exportIdx < 0 || exportIdx >= len(asset.Exports) {
			continue
		}
		// Walk outer chain: widget -> WidgetTree -> (Blueprint or GeneratedClass)
		current := exportIdx
		seen := make(map[int]bool, 4)
		for current >= 0 && current < len(asset.Exports) && !seen[current] {
			seen[current] = true
			outerIdx := resolveOuterExportIndex(asset, asset.Exports[current])
			if outerIdx == target.BlueprintExport {
				return exportIdx
			}
			current = outerIdx
		}
	}
	return target.Exports[0]
}

func syncBrushDependency(asset *uasset.Asset, opts uasset.ParseOptions, exportIndex int, textureImportIdx int32) ([]byte, *uasset.Asset, error) {
	return syncWidgetImportDependency(asset, opts, exportIndex, textureImportIdx)
}

func syncWidgetImportDependency(asset *uasset.Asset, opts uasset.ParseOptions, exportIndex int, importIdx int32) ([]byte, *uasset.Asset, error) {
	return syncWidgetImportDependencyOrdered(asset, opts, exportIndex, importIdx, false)
}

func syncWidgetImportDependencyOrdered(asset *uasset.Asset, opts uasset.ParseOptions, exportIndex int, importIdx int32, prepend bool) ([]byte, *uasset.Asset, error) {
	items, warnings := parseDependsMap(asset)
	if len(warnings) > 0 {
		return nil, nil, fmt.Errorf("parse depends map: %s", strings.Join(warnings, "; "))
	}
	if exportIndex >= len(items) {
		return nil, nil, fmt.Errorf("depends map entry missing for export %d", exportIndex+1)
	}

	dependencies := anyMapSlice(items[exportIndex]["dependencies"])
	existing := make([]uasset.PackageIndex, 0, len(dependencies)+1)
	for _, dep := range dependencies {
		rawIdx, ok := anyInt(dep["index"])
		if !ok {
			continue
		}
		existing = append(existing, uasset.PackageIndex(rawIdx))
	}

	desired := uasset.PackageIndex(importIdx)
	if slices.Contains(existing, desired) {
		if prepend && len(existing) > 1 && existing[0] != desired {
			reordered := []uasset.PackageIndex{desired}
			for _, dep := range existing {
				if dep == desired {
					continue
				}
				reordered = append(reordered, dep)
			}
			existing = reordered
		}
	} else if prepend {
		existing = append([]uasset.PackageIndex{desired}, existing...)
	} else {
		existing = append(existing, desired)
	}

	outBytes, err := edit.ReplaceExportDependencies(asset, exportIndex, existing)
	if err != nil {
		return nil, nil, err
	}
	updatedAsset, err := uasset.ParseBytes(outBytes, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("reparse after depends rewrite: %w", err)
	}
	return outBytes, updatedAsset, nil
}

func normalizeBrushImageDesignerWidgetTreeFlags(asset *uasset.Asset, opts uasset.ParseOptions, blueprintExport int) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if blueprintExport < 0 || blueprintExport >= len(asset.Exports) {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	designerTreeIdx := -1
	for i, exp := range asset.Exports {
		if !strings.EqualFold(asset.ResolveClassName(exp), "WidgetTree") {
			continue
		}
		if resolveOuterExportIndex(asset, exp) != blueprintExport {
			continue
		}
		designerTreeIdx = i
		break
	}
	if designerTreeIdx < 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	const rfArchetypeObject = uint32(0x20)
	oldFlags := asset.Exports[designerTreeIdx].ObjectFlags
	newFlags := oldFlags &^ rfArchetypeObject
	if newFlags == oldFlags {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	positions, err := scanExportHeaderPositions(asset)
	if err != nil {
		return nil, nil, fmt.Errorf("scan export header positions: %w", err)
	}
	if len(positions) != len(asset.Exports) {
		return nil, nil, fmt.Errorf("export header position mismatch")
	}

	out := append([]byte(nil), asset.Raw.Bytes...)
	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}
	pos := positions[designerTreeIdx].objectFlags
	if pos < 0 || pos+4 > len(out) {
		return nil, nil, fmt.Errorf("designer WidgetTree objectFlags field out of bounds")
	}
	order.PutUint32(out[pos:pos+4], newFlags)
	if err := edit.FinalizePackageBytes(out, asset.Summary.FileVersionUE5); err != nil {
		return nil, nil, fmt.Errorf("finalize package bytes: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(out, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("reparse normalized asset: %w", err)
	}
	return out, updatedAsset, nil
}

// ---------------------------------------------------------------------------
// Widget write target collection (reuses widget-read model)
// ---------------------------------------------------------------------------

// widgetWriteTarget represents a logical widget with all matching per-tree exports.
type widgetWriteTarget struct {
	BlueprintExport int
	ClassName       string
	ObjectName      string
	Path            string
	Exports         []int // 0-based export indices across matching widget trees
	SlotExports     []int // 0-based slot export indices across matching widget trees
}

// collectWidgetWriteTargets builds the list of logical widgets that can be
// written to, by reusing the generic widget-read model.
func collectWidgetWriteTargets(asset *uasset.Asset, exportFilter int) ([]widgetWriteTarget, error) {
	bpExports := findWidgetBlueprintExports(asset)
	if exportFilter > 0 {
		idx, err := asset.ResolveExportIndex(exportFilter)
		if err != nil {
			return nil, err
		}
		found := false
		for _, bpIdx := range bpExports {
			if bpIdx == idx {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("export %d is not a WidgetBlueprint", exportFilter)
		}
		bpExports = []int{idx}
	}

	out := make([]widgetWriteTarget, 0, 16)
	for _, bpIdx := range bpExports {
		bpProps := asset.ParseExportProperties(bpIdx)
		decoded := decodeAllProperties(asset, bpProps.Properties)

		genClassIdx := -1
		if v, ok := decoded["GeneratedClass"]; ok {
			genClassIdx = widgetExportIndexFromDecoded(v) - 1
		}

		treeIndices := findWidgetTreeExports(asset, bpIdx, genClassIdx)
		logicalWidgets, err := buildLogicalWidgetEntries(asset, bpIdx, genClassIdx, treeIndices)
		if err != nil {
			return nil, err
		}
		for _, item := range logicalWidgets {
			path, _ := item["path"].(string)
			objectName, _ := item["objectName"].(string)
			className, _ := item["className"].(string)
			out = append(out, widgetWriteTarget{
				BlueprintExport: bpIdx,
				ClassName:       className,
				ObjectName:      objectName,
				Path:            path,
				Exports:         widgetWriteZeroBasedIndices(item["widgetExports"]),
				SlotExports:     widgetWriteZeroBasedIndices(item["slotExports"]),
			})
		}
	}
	slices.SortFunc(out, func(a, b widgetWriteTarget) int {
		return strings.Compare(strings.ToLower(a.Path), strings.ToLower(b.Path))
	})
	return out, nil
}

// ---------------------------------------------------------------------------
// Target selection
// ---------------------------------------------------------------------------

func selectWidgetWriteTarget(targets []widgetWriteTarget, selector string) (*widgetWriteTarget, error) {
	trimmed := strings.TrimSpace(selector)
	if trimmed == "" {
		return nil, fmt.Errorf("widget selector is required")
	}

	// Exact path match first.
	for i := range targets {
		if strings.EqualFold(targets[i].Path, trimmed) {
			return &targets[i], nil
		}
	}

	// Object name match (must be unambiguous).
	nameMatches := make([]*widgetWriteTarget, 0, 4)
	for i := range targets {
		if strings.EqualFold(targets[i].ObjectName, trimmed) {
			nameMatches = append(nameMatches, &targets[i])
		}
	}
	if len(nameMatches) == 1 {
		return nameMatches[0], nil
	}
	if len(nameMatches) > 1 {
		paths := make([]string, 0, len(nameMatches))
		for _, item := range nameMatches {
			paths = append(paths, item.Path)
		}
		slices.Sort(paths)
		return nil, fmt.Errorf("widget %q is ambiguous; use a full widget path (%s)", selector, strings.Join(paths, ", "))
	}

	// Fuzzy candidates for error message.
	normalized := strings.ToLower(trimmed)
	candidates := make([]string, 0, len(targets))
	for _, item := range targets {
		if strings.Contains(strings.ToLower(item.Path), normalized) || strings.Contains(strings.ToLower(item.ObjectName), normalized) {
			candidates = append(candidates, item.Path)
		}
	}
	slices.Sort(candidates)
	if len(candidates) > 0 {
		return nil, fmt.Errorf("widget %q not found; candidates: %s", selector, strings.Join(candidates, ", "))
	}
	return nil, fmt.Errorf("widget %q not found", selector)
}

func validateWidgetWriteTarget(target widgetWriteTarget) error {
	if len(target.Exports) == 0 && len(target.SlotExports) == 0 {
		return fmt.Errorf(
			"widget %q did not resolve to a writable export (widget or slot)",
			target.Path,
		)
	}
	if len(target.Exports) > 2 {
		return fmt.Errorf(
			"widget %q resolved to %d exports; expected at most designer/generated pair",
			target.Path,
			len(target.Exports),
		)
	}
	return nil
}

type widgetBlueprintShape map[string][]string

func captureWidgetBlueprintShape(asset *uasset.Asset, bpIdx int) (widgetBlueprintShape, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if bpIdx < 0 || bpIdx >= len(asset.Exports) {
		return nil, fmt.Errorf("widget blueprint export out of range: %d", bpIdx+1)
	}
	bpProps := asset.ParseExportProperties(bpIdx)
	decoded := decodeAllProperties(asset, bpProps.Properties)

	genClassIdx := -1
	if v, ok := decoded["GeneratedClass"]; ok {
		genClassIdx = widgetExportIndexFromDecoded(v) - 1
	}

	shape := make(widgetBlueprintShape)
	for _, treeIdx := range findWidgetTreeExports(asset, bpIdx, genClassIdx) {
		treeExp := asset.Exports[treeIdx]
		outerIdx := resolveOuterExportIndex(asset, treeExp)
		outerClassName := ""
		if outerIdx >= 0 && outerIdx < len(asset.Exports) {
			outerClassName = asset.ResolveClassName(asset.Exports[outerIdx])
		}
		role := widgetTreeRole(bpIdx, genClassIdx, outerIdx, outerClassName)

		treeProps := asset.ParseExportProperties(treeIdx)
		treeDecoded := decodeAllProperties(asset, treeProps.Properties)
		rootWidgetExport := 0
		if v, ok := treeDecoded["RootWidget"]; ok {
			rootWidgetExport = widgetExportIndexFromDecoded(v)
		}
		allWidgetExports := normalizeWidgetExportList(rootWidgetExport, widgetExportIndicesFromDecodedArray(treeDecoded["AllWidgets"]))
		widgets, err := buildWidgetHierarchy(asset, rootWidgetExport, allWidgetExports)
		if err != nil {
			return nil, fmt.Errorf("tree export %d: %w", treeIdx+1, err)
		}
		paths := make([]string, 0, len(widgets))
		for _, w := range widgets {
			path, _ := w["path"].(string)
			if path != "" {
				paths = append(paths, path)
			}
		}
		slices.Sort(paths)
		shape[role] = paths
	}
	return shape, nil
}

func validateWidgetBlueprintShapeStable(before, after widgetBlueprintShape) error {
	if len(before) != len(after) {
		return fmt.Errorf("tree role count changed: before=%d after=%d", len(before), len(after))
	}
	for role, beforePaths := range before {
		afterPaths, ok := after[role]
		if !ok {
			return fmt.Errorf("tree role %q disappeared after rewrite", role)
		}
		if !slices.Equal(beforePaths, afterPaths) {
			return fmt.Errorf("tree role %q widget paths changed: before=%v after=%v", role, beforePaths, afterPaths)
		}
	}
	for role := range after {
		if _, ok := before[role]; !ok {
			return fmt.Errorf("tree role %q appeared after rewrite", role)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func widgetWriteExportIndicesOneBased(zeroBasedIndices []int) []int {
	out := make([]int, len(zeroBasedIndices))
	for i, idx := range zeroBasedIndices {
		out[i] = idx + 1
	}
	return out
}

func widgetWriteZeroBasedIndices(raw any) []int {
	items, ok := raw.([]int)
	if ok {
		out := make([]int, 0, len(items))
		for _, item := range items {
			if item > 0 {
				out = append(out, item-1)
			}
		}
		return out
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]int, 0, len(list))
	for _, item := range list {
		v, ok := extractIntLike(item)
		if !ok || v <= 0 {
			continue
		}
		out = append(out, v-1)
	}
	return out
}

// ---------------------------------------------------------------------------
// Property add fallback for default-valued properties
// ---------------------------------------------------------------------------

// widgetPropertyAddSpecJSON builds a JSON spec for BuildPropertyAddMutation
// when a widget property does not yet exist on the export (default value).
func widgetPropertyAddSpecJSON(className, normalizedProperty, valueJSON string) (string, error) {
	switch normalizedProperty {
	case "text":
		return fmt.Sprintf(`{"name":"Text","type":"TextProperty","value":%s}`, valueJSON), nil
	case "visibility":
		if strings.EqualFold(className, "RichTextBlock") {
			return fmt.Sprintf(`{"name":"Visibility","type":"EnumProperty(ESlateVisibility(/Script/UMG),ByteProperty)","value":%s}`, valueJSON), nil
		}
		return fmt.Sprintf(`{"name":"Visibility","type":"EnumProperty(ESlateVisibility)","value":%s}`, valueJSON), nil
	case "render-opacity":
		return fmt.Sprintf(`{"name":"RenderOpacity","type":"FloatProperty","value":%s}`, valueJSON), nil
	default:
		return "", fmt.Errorf("no add spec for property %q", normalizedProperty)
	}
}

func widgetPropertyAddBeforeProperty(asset *uasset.Asset, exportIndex int, className, normalizedProperty string) string {
	switch normalizedProperty {
	case "text":
		return widgetPropertyAddBeforePropertyForExport(asset, exportIndex, "Slot", "DisplayLabel")
	case "visibility", "render-opacity":
		if strings.EqualFold(className, "RichTextBlock") {
			return widgetPropertyAddBeforePropertyForExport(asset, exportIndex, "Slot", "DisplayLabel")
		}
		return ""
	default:
		return ""
	}
}

func widgetPropertyAddBeforePropertyForExport(asset *uasset.Asset, exportIndex int, candidates ...string) string {
	if asset == nil || exportIndex < 0 || exportIndex >= len(asset.Exports) {
		return ""
	}
	parsed := asset.ParseExportProperties(exportIndex)
	for _, candidate := range candidates {
		for _, prop := range parsed.Properties {
			if strings.EqualFold(prop.Name.Display(asset.Names), candidate) {
				return candidate
			}
		}
	}
	return ""
}

// applyPropertyAddAsSetResult wraps BuildPropertyAddMutation + RewriteAsset
// and returns a PropertySetResult so callers can use a uniform interface.
func applyPropertyAddAsSetResult(asset *uasset.Asset, exportIndex int, specJSON string, opts uasset.ParseOptions) ([]byte, *edit.PropertySetResult, error) {
	return applyPropertyAddAsSetResultBefore(asset, exportIndex, specJSON, "", opts)
}

func applyPropertyAddAsSetResultBefore(asset *uasset.Asset, exportIndex int, specJSON string, beforeProperty string, opts uasset.ParseOptions) ([]byte, *edit.PropertySetResult, error) {
	addResult, err := edit.BuildPropertyAddMutation(asset, exportIndex, specJSON)
	if beforeProperty != "" {
		addResult, err = edit.BuildPropertyAddMutationBefore(asset, exportIndex, specJSON, beforeProperty)
	}
	if err != nil {
		return nil, nil, err
	}
	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{addResult.Mutation})
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite asset: %w", err)
	}
	outBytes, _, err = postprocessWidgetPropertyWrite(
		outBytes,
		exportIndex,
		addResult.PropertyName,
		addResult.PropertyName,
		nil,
		addResult.NewValue,
		opts,
	)
	if err != nil {
		return nil, nil, err
	}
	return outBytes, &edit.PropertySetResult{
		Mutation:     addResult.Mutation,
		ExportIndex:  addResult.ExportIndex,
		PropertyName: addResult.PropertyName,
		Path:         addResult.PropertyName,
		OldValue:     nil,
		NewValue:     addResult.NewValue,
		OldSize:      0,
		NewSize:      addResult.NewSize,
		ByteDelta:    addResult.ByteDelta,
	}, nil
}

// ---------------------------------------------------------------------------
// Property set with post-processing (asset registry + name compaction)
// ---------------------------------------------------------------------------

func postprocessWidgetPropertyWrite(outBytes []byte, exportIndex int, path string, propertyName string, oldValue, newValue any, opts uasset.ParseOptions) ([]byte, *uasset.Asset, error) {
	updatedAsset, err := uasset.ParseBytes(outBytes, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("reparse rewritten asset: %w", err)
	}

	rootValue := newValue
	if strings.TrimSpace(path) != strings.TrimSpace(propertyName) {
		rootValue, err = decodeExportRootPropertyValue(updatedAsset, exportIndex, propertyName)
		if err != nil {
			return nil, nil, fmt.Errorf("decode rewritten root property: %w", err)
		}
	}

	outBytes, _, err = rewriteAssetRegistryValueChange(updatedAsset, propertyName, rootValue, oldValue, newValue)
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite asset registry search data: %w", err)
	}
	updatedAsset, err = uasset.ParseBytes(outBytes, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("reparse asset after asset registry rewrite: %w", err)
	}
	return outBytes, updatedAsset, nil
}

func applyPropertySetCommandWithPostprocess(asset *uasset.Asset, exportIndex int, path string, valueJSON string, opts uasset.ParseOptions) ([]byte, *edit.PropertySetResult, error) {
	editResult, err := edit.BuildPropertySetMutation(asset, exportIndex, path, valueJSON)
	if err != nil {
		fallbackResult, fallbackErr := buildPropertySetStructLeafFallbackMutation(asset, exportIndex, path, valueJSON)
		if fallbackErr != nil {
			return nil, nil, err
		}
		editResult = fallbackResult
	}

	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{editResult.Mutation})
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite asset: %w", err)
	}
	outBytes, _, err = postprocessWidgetPropertyWrite(
		outBytes,
		exportIndex,
		editResult.Path,
		editResult.PropertyName,
		editResult.OldValue,
		editResult.NewValue,
		opts,
	)
	if err != nil {
		return nil, nil, err
	}

	// Note: name compaction is intentionally skipped for widget-write.
	// UE does not remove unused NameMap entries when updating widget text,
	// so compacting would introduce NameMap hash differences.
	return outBytes, editResult, nil
}

// ---------------------------------------------------------------------------
// NameMap entry management
// ---------------------------------------------------------------------------

func ensureNameEntriesPresent(asset *uasset.Asset, opts uasset.ParseOptions, names []string) ([]byte, *uasset.Asset, []string, error) {
	if asset == nil {
		return nil, nil, nil, fmt.Errorf("asset is nil")
	}
	updatedNames := append([]uasset.NameEntry(nil), asset.Names...)
	added := make([]string, 0, len(names))
	for _, rawName := range names {
		nameValue := strings.TrimSpace(rawName)
		if nameValue == "" {
			continue
		}
		if findNameIndexByValue(updatedNames, nameValue) >= 0 {
			continue
		}
		nonCase, casePreserving := edit.ComputeNameEntryHashesUE56(nameValue)
		updatedNames = append(updatedNames, uasset.NameEntry{
			Value:              nameValue,
			NonCaseHash:        nonCase,
			CasePreservingHash: casePreserving,
		})
		added = append(added, nameValue)
	}
	if len(added) == 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil, nil
	}
	outBytes, err := edit.RewriteNameMap(asset, updatedNames)
	if err != nil {
		return nil, nil, nil, err
	}
	updatedAsset, err := uasset.ParseBytes(outBytes, opts)
	if err != nil {
		return nil, nil, nil, err
	}
	return outBytes, updatedAsset, added, nil
}

func ensureNameEntriesPresentSortedSuffix(asset *uasset.Asset, opts uasset.ParseOptions, names []string) ([]byte, *uasset.Asset, []string, error) {
	return insertBrushImageNames(asset, opts, nil, names)
}

func ensureNameEntriesPresentSorted(asset *uasset.Asset, opts uasset.ParseOptions, names []string) ([]byte, *uasset.Asset, []string, error) {
	if asset == nil {
		return nil, nil, nil, fmt.Errorf("asset is nil")
	}
	boundary := int(asset.Summary.NamesReferencedFromExportDataCount)
	if boundary < 0 {
		boundary = 0
	}
	if boundary > len(asset.Names) {
		boundary = len(asset.Names)
	}

	oldNames := append([]uasset.NameEntry(nil), asset.Names...)
	prefix := append([]uasset.NameEntry(nil), asset.Names[:boundary]...)
	suffix := append([]uasset.NameEntry(nil), asset.Names[boundary:]...)
	added := make([]string, 0, len(names))
	prefixAdded := 0

	for _, rawName := range names {
		nameValue := strings.TrimSpace(rawName)
		if nameValue == "" {
			continue
		}
		if findNameIndexByValue(prefix, nameValue) >= 0 {
			continue
		}
		if idx := findNameIndexByValue(suffix, nameValue); idx >= 0 {
			entry := suffix[idx]
			suffix = append(suffix[:idx], suffix[idx+1:]...)
			pos := sort.Search(len(prefix), func(i int) bool {
				return strings.ToLower(prefix[i].Value) >= strings.ToLower(nameValue)
			})
			prefix = slices.Insert(prefix, pos, entry)
			prefixAdded++
			continue
		}
		nonCase, casePreserving := edit.ComputeNameEntryHashesUE56(nameValue)
		entry := uasset.NameEntry{
			Value:              nameValue,
			NonCaseHash:        nonCase,
			CasePreservingHash: casePreserving,
		}
		pos := sort.Search(len(prefix), func(i int) bool {
			return strings.ToLower(prefix[i].Value) >= strings.ToLower(nameValue)
		})
		prefix = slices.Insert(prefix, pos, entry)
		added = append(added, nameValue)
		prefixAdded++
	}

	newNames := append(prefix, suffix...)
	nameMapChanged := len(newNames) != len(oldNames)
	if !nameMapChanged {
		for i := range oldNames {
			if oldNames[i].Value != newNames[i].Value {
				nameMapChanged = true
				break
			}
		}
	}
	if !nameMapChanged {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil, nil
	}

	indexRemap, err := edit.BuildNameIndexRemapAllowInsertedNewEntries(oldNames, newNames)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build name remap: %w", err)
	}
	workingBytes, err := edit.RewriteImportExportNameRefs(asset, indexRemap)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("rewrite import/export name refs: %w", err)
	}

	remapped := *asset
	remapped.Raw.Bytes = workingBytes
	workingBytes, err = edit.RewriteNameMap(&remapped, newNames)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("rewrite name map: %w", err)
	}
	workingAsset, err := uasset.ParseBytes(workingBytes, opts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("reparse after name map rewrite: %w", err)
	}

	exportMutations, err := edit.BuildExportNameRemapMutations(asset, workingAsset, indexRemap, "", "")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build export payload remap mutations: %w", err)
	}
	if len(exportMutations) > 0 {
		workingBytes, err = edit.RewriteAsset(workingAsset, exportMutations)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("rewrite export payloads: %w", err)
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("reparse after export payload rewrite: %w", err)
		}
	}

	workingBytes, workingAsset, err = edit.PatchInstancedTailNameRefs(asset, workingAsset, indexRemap, opts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("patch instanced tail name refs: %w", err)
	}

	newBoundary := int32(boundary + prefixAdded)
	if workingAsset.Summary.NamesReferencedFromExportDataCount < newBoundary {
		workingBytes, err = edit.ExpandNamesReferencedFromExportDataCount(workingAsset, newBoundary)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("expand NamesReferencedFromExportDataCount: %w", err)
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("reparse after NamesReferencedFromExportDataCount expansion: %w", err)
		}
	}
	return workingBytes, workingAsset, added, nil
}
