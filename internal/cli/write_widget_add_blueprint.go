package cli

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/wilddogjp/openbpx/pkg/edit"
	"github.com/wilddogjp/openbpx/pkg/uasset"
)

type widgetAddName struct {
	Display string
	Base    string
	Number  int32
}

type widgetAddContext struct {
	Target                widgetWriteTarget
	DesignerParentExport  int
	GeneratedParentExport int
	DesignerTreeExport    int
	GeneratedTreeExport   int
	GeneratedClassExport  int
	CDOExport             int
	BlueprintObjectName   string
	PanelClass            string
}

type widgetAddMode int

const (
	widgetAddModeGeneral widgetAddMode = iota
	widgetAddModeOverlayRootChain
	widgetAddModeGeneratedRootlessNestedOverlay
)

type widgetAddDependsUpdate struct {
	exportIndex int
	deps        []uasset.PackageIndex
	appendOnly  bool
}

type widgetAddChildClassSpec struct {
	ResolvedClassName    string
	BlueprintPackagePath string
	BlueprintObjectName  string
}

func runBlueprintWidgetAdd(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("blueprint widget-add", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "optional 1-based WidgetBlueprint export index")
	parentSelector := fs.String("parent", "", "parent widget path, object name, or root")
	widgetType := fs.String("type", "", "supported: image, textblock, richtextblock, progressbar, slider, spacer, scrollbar, editabletext, editabletextbox, multilineeditabletextbox, spinbox, comboboxstring, checkbox, userwidget, button, border, retainerbox, invalidationbox, menuanchor, namedslot, sizebox, scalebox, backgroundblur, safezone, windowtitlebararea, canvaspanel, overlay, verticalbox, horizontalbox, stackbox, scrollbox, wrapbox, gridpanel, uniformgridpanel, widgetswitcher, listview, tileview, treeview")
	classPath := fs.String("class", "", "required for --type userwidget: Blueprint asset path like /Game/UI/WBP_Status")
	nameRaw := fs.String("name", "", "new widget object name (e.g. Image_23)")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || *exportIndex < 0 || strings.TrimSpace(*parentSelector) == "" || strings.TrimSpace(*widgetType) == "" || strings.TrimSpace(*nameRaw) == "" {
		fmt.Fprintln(stderr, "usage: bpx blueprint widget-add <file.uasset> --parent <path|name|root> --type <image|textblock|richtextblock|progressbar|slider|spacer|scrollbar|editabletext|editabletextbox|multilineeditabletextbox|spinbox|comboboxstring|checkbox|userwidget|button|border|retainerbox|invalidationbox|menuanchor|namedslot|sizebox|scalebox|backgroundblur|safezone|windowtitlebararea|canvaspanel|overlay|verticalbox|horizontalbox|stackbox|scrollbox|wrapbox|gridpanel|uniformgridpanel|widgetswitcher|listview|tileview|treeview> --name <Widget_N> [--class </Game/...> when --type userwidget] [--export <n>] [--dry-run] [--backup]")
		return 1
	}

	normalizedType := strings.ToLower(strings.TrimSpace(*widgetType))
	if normalizedType != "image" && normalizedType != "textblock" && normalizedType != "richtextblock" &&
		normalizedType != "progressbar" && normalizedType != "slider" &&
		normalizedType != "spacer" &&
		normalizedType != "scrollbar" && normalizedType != "editabletext" &&
		normalizedType != "editabletextbox" &&
		normalizedType != "multilineeditabletextbox" &&
		normalizedType != "spinbox" && normalizedType != "comboboxstring" &&
		normalizedType != "checkbox" &&
		normalizedType != "userwidget" && normalizedType != "button" &&
		normalizedType != "border" && normalizedType != "retainerbox" &&
		normalizedType != "invalidationbox" && normalizedType != "menuanchor" &&
		normalizedType != "namedslot" &&
		normalizedType != "sizebox" &&
		normalizedType != "scalebox" && normalizedType != "backgroundblur" &&
		normalizedType != "safezone" && normalizedType != "windowtitlebararea" &&
		normalizedType != "canvaspanel" &&
		normalizedType != "overlay" && normalizedType != "verticalbox" &&
		normalizedType != "horizontalbox" && normalizedType != "stackbox" &&
		normalizedType != "scrollbox" && normalizedType != "wrapbox" &&
		normalizedType != "gridpanel" && normalizedType != "uniformgridpanel" &&
		normalizedType != "widgetswitcher" && normalizedType != "listview" &&
		normalizedType != "tileview" && normalizedType != "treeview" {
		fmt.Fprintf(stderr, "error: unsupported widget-add type %q (supported: image, textblock, richtextblock, progressbar, slider, spacer, scrollbar, editabletext, editabletextbox, multilineeditabletextbox, spinbox, comboboxstring, checkbox, userwidget, button, border, retainerbox, invalidationbox, menuanchor, namedslot, sizebox, scalebox, backgroundblur, safezone, windowtitlebararea, canvaspanel, overlay, verticalbox, horizontalbox, stackbox, scrollbox, wrapbox, gridpanel, uniformgridpanel, widgetswitcher, listview, tileview, treeview)\n", *widgetType)
		return 1
	}

	childName, err := parseWidgetAddName(*nameRaw)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if isWidgetAddRootSelector(*parentSelector) {
		return runBlueprintWidgetAddRoot(asset, *opts, file, *exportIndex, normalizedType, childName, *dryRun, *backup, stdout, stderr)
	}

	targets, err := collectWidgetWriteTargets(asset, *exportIndex)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	parent, err := selectWidgetWriteTarget(targets, *parentSelector)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := validateWidgetWriteTarget(*parent); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := validateWidgetAddTarget(asset, targets, *parent, childName.Display); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	childClassSpec, err := resolveWidgetAddChildClassSpec(normalizedType, *classPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	childWidgetClass := childClassSpec.ResolvedClassName

	ctx, err := resolveWidgetAddContext(asset, *parent)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	parentSlots, err := readWidgetAddObjectRefArrayValue(asset, ctx.DesignerParentExport, "Slots")
	if err != nil {
		fmt.Fprintf(stderr, "error: read parent Slots: %v\n", err)
		return 1
	}
	parentHadChildren := len(parentSlots) > 0
	fixtureEmptyCanvasPanelButtonMode := !parentHadChildren &&
		strings.EqualFold(ctx.PanelClass, "CanvasPanel") &&
		strings.EqualFold(childWidgetClass, "Button")
	fixtureEmptyCanvasPanelRichTextMode := !parentHadChildren &&
		strings.EqualFold(ctx.PanelClass, "CanvasPanel") &&
		strings.EqualFold(childWidgetClass, "RichTextBlock")
	fixtureEmptyCanvasPanelBareLeafMode := !parentHadChildren &&
		strings.EqualFold(ctx.PanelClass, "CanvasPanel") &&
		widgetAddUsesBareCanvasPanelLeafConventions(childWidgetClass)

	_, workingAsset, addedNames, addedImports, err := ensureWidgetAddPrerequisites(
		asset,
		*opts,
		ctx.PanelClass,
		childClassSpec,
		childName,
		ctx.BlueprintObjectName,
		!parentHadChildren && !fixtureEmptyCanvasPanelButtonMode && !fixtureEmptyCanvasPanelBareLeafMode,
		!fixtureEmptyCanvasPanelButtonMode && !fixtureEmptyCanvasPanelRichTextMode && !fixtureEmptyCanvasPanelBareLeafMode && !strings.EqualFold(ctx.PanelClass, "NamedSlot"),
		!fixtureEmptyCanvasPanelButtonMode && !fixtureEmptyCanvasPanelRichTextMode && !fixtureEmptyCanvasPanelBareLeafMode,
	)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	var workingBytes []byte
	ctx, err = refreshWidgetAddContext(workingAsset, *exportIndex, *parentSelector)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	missingGeneratedParent := ctx.GeneratedParentExport < 0
	mode := resolveWidgetAddMode(workingAsset, ctx)
	if mode == widgetAddModeGeneratedRootlessNestedOverlay && ctx.GeneratedParentExport < 0 {
		_, workingAsset, err = rewriteWidgetAddGeneratedTreeRootlessTopLevelParent(workingAsset, *opts, ctx)
		if err != nil {
			fmt.Fprintf(stderr, "error: rewrite generated WidgetTree rootless companions: %v\n", err)
			return 1
		}
		ctx, err = refreshWidgetAddContext(workingAsset, *exportIndex, *parentSelector)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}
	if ctx.GeneratedParentExport < 0 {
		_, workingAsset, err = ensureWidgetAddGeneratedParentCompanion(workingAsset, *opts, ctx, mode != widgetAddModeGeneratedRootlessNestedOverlay)
		if err != nil {
			fmt.Fprintf(stderr, "error: ensure generated widget companion: %v\n", err)
			return 1
		}
		ctx, err = refreshWidgetAddContext(workingAsset, *exportIndex, *parentSelector)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}
	if ctx.GeneratedParentExport < 0 {
		fmt.Fprintf(stderr, "error: generated WidgetTree parent companion is missing for %s\n", ctx.Target.Path)
		return 1
	}

	overlayChainMode := mode == widgetAddModeOverlayRootChain
	addGeneratedChild := widgetAddShouldInsertGeneratedChild(ctx.Target.Path, ctx.PanelClass, childWidgetClass, childName, overlayChainMode, parentHadChildren)
	richTextLeafMode := strings.EqualFold(childWidgetClass, "RichTextBlock")
	bareCanvasPanelLeafMode := !parentHadChildren &&
		!overlayChainMode &&
		strings.EqualFold(ctx.PanelClass, "CanvasPanel") &&
		widgetAddUsesBareCanvasPanelLeafConventions(childWidgetClass)
	emptyCanvasPanelButtonMode := !parentHadChildren &&
		strings.EqualFold(ctx.PanelClass, "CanvasPanel") &&
		strings.EqualFold(childWidgetClass, "Button") &&
		!overlayChainMode
	if !addGeneratedChild && (mode == widgetAddModeGeneratedRootlessNestedOverlay || missingGeneratedParent) {
		addGeneratedChild = true
	}
	existingGeneratedClassLayout, err := captureGeneratedClassWidgetVariableFieldLayout(workingAsset, ctx.GeneratedClassExport)
	if err != nil {
		fmt.Fprintf(stderr, "error: capture generated class field layout: %v\n", err)
		return 1
	}
	existingGeneratedClassSerialSize := int(workingAsset.Exports[ctx.GeneratedClassExport].SerialSize)

	reusableGeneratedSlotExport, reusableGeneratedSlotName, err := findWidgetAddReusableGeneratedSlotExport(workingAsset, ctx)
	if err != nil {
		fmt.Fprintf(stderr, "error: resolve reusable generated slot: %v\n", err)
		return 1
	}
	slotName := reusableGeneratedSlotName
	if strings.TrimSpace(slotName) == "" {
		if emptyCanvasPanelButtonMode {
			slotName = widgetAddSlotName("CanvasPanelSlot", 0)
		} else {
			defaultSlotSuffix := widgetAddSlotDefaultSuffix(ctx.PanelClass, overlayChainMode)
			if richTextLeafMode || bareCanvasPanelLeafMode {
				defaultSlotSuffix = 0
			}
			slotName, err = nextWidgetAddSlotName(workingAsset, ctx.PanelClass, defaultSlotSuffix)
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
		}
	}
	designerSlotEntry, err := buildWidgetAddSlotEntry(workingAsset, ctx.PanelClass, ctx.DesignerParentExport, slotName)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	slotInsertAt, err := findWidgetAddChildInsertPos(workingAsset, ctx.DesignerParentExport, maxIntLocal(ctx.DesignerParentExport, ctx.GeneratedParentExport)+1)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	workingBytes, err = edit.InsertExportEntries(workingAsset, slotInsertAt, []edit.ExportInsertEntry{designerSlotEntry})
	if err != nil {
		fmt.Fprintf(stderr, "error: insert designer slot export: %v\n", err)
		return 1
	}
	workingAsset, err = uasset.ParseBytes(workingBytes, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: reparse after designer slot export insert: %v\n", err)
		return 1
	}
	designerSlotExport := slotInsertAt

	ctx, err = refreshWidgetAddContext(workingAsset, *exportIndex, *parentSelector)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	generatedSlotExport := reusableGeneratedSlotExport
	if generatedSlotExport < 0 {
		generatedSlotEntry, buildErr := buildWidgetAddSlotEntry(workingAsset, ctx.PanelClass, ctx.GeneratedParentExport, slotName)
		if buildErr != nil {
			fmt.Fprintf(stderr, "error: %v\n", buildErr)
			return 1
		}
		slotInsertAt, err = findWidgetAddChildInsertPos(workingAsset, ctx.GeneratedParentExport, designerSlotExport+1)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		workingBytes, err = edit.InsertExportEntries(workingAsset, slotInsertAt, []edit.ExportInsertEntry{generatedSlotEntry})
		if err != nil {
			fmt.Fprintf(stderr, "error: insert generated slot export: %v\n", err)
			return 1
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, *opts)
		if err != nil {
			fmt.Fprintf(stderr, "error: reparse after generated slot export insert: %v\n", err)
			return 1
		}
		generatedSlotExport = slotInsertAt
	}

	ctx, err = refreshWidgetAddContext(workingAsset, *exportIndex, *parentSelector)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	imageInsertAt := 0
	if widgetAddIsBlueprintGeneratedWidgetClass(childWidgetClass) {
		imageInsertAt = ctx.Target.BlueprintExport
	} else if !parentHadChildren &&
		strings.EqualFold(ctx.PanelClass, "CanvasPanel") &&
		strings.EqualFold(childWidgetClass, "ComboBoxString") {
		imageInsertAt, err = findWidgetAddImageInsertPos(workingAsset)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		if imageInsertAt > 0 {
			imageInsertAt--
		}
	} else {
		imageInsertAt, err = findWidgetAddChildWidgetInsertPos(workingAsset, ctx, designerSlotExport, generatedSlotExport, widgetAddUsesPostEventGraphLeafInsertion(childWidgetClass))
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}
	imageEntries, err := buildWidgetAddChildEntries(workingAsset, ctx, childClassSpec, childName, addGeneratedChild)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	workingBytes, err = edit.InsertExportEntries(workingAsset, imageInsertAt, imageEntries)
	if err != nil {
		fmt.Fprintf(stderr, "error: insert image exports: %v\n", err)
		return 1
	}
	workingAsset, err = uasset.ParseBytes(workingBytes, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: reparse after image export insert: %v\n", err)
		return 1
	}
	designerImageExport := imageInsertAt
	generatedImageExport := -1
	if addGeneratedChild {
		generatedImageExport = imageInsertAt + 1
	}
	designerSlotExport = remapWidgetAddInsertedExportIndex(designerSlotExport, imageInsertAt, len(imageEntries))
	generatedSlotExport = remapWidgetAddInsertedExportIndex(generatedSlotExport, imageInsertAt, len(imageEntries))

	ctx, err = refreshWidgetAddContext(workingAsset, *exportIndex, *parentSelector)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	designerSlotValues, err := appendWidgetAddObjectRefArrayValue(workingAsset, ctx.DesignerParentExport, "Slots", objectRefValue(designerSlotExport))
	if err != nil {
		fmt.Fprintf(stderr, "error: read designer parent Slots: %v\n", err)
		return 1
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, ctx.DesignerParentExport, "Slots", "ArrayProperty(ObjectProperty)", designerSlotValues, widgetAddParentSlotsBeforeProperty(workingAsset, ctx.DesignerParentExport))
	if err != nil {
		fmt.Fprintf(stderr, "error: write designer parent Slots: %v\n", err)
		return 1
	}
	generatedSlotValues, err := appendWidgetAddObjectRefArrayValue(workingAsset, ctx.GeneratedParentExport, "Slots", objectRefValue(generatedSlotExport))
	if err != nil {
		fmt.Fprintf(stderr, "error: read generated parent Slots: %v\n", err)
		return 1
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, ctx.GeneratedParentExport, "Slots", "ArrayProperty(ObjectProperty)", generatedSlotValues, widgetAddParentSlotsBeforeProperty(workingAsset, ctx.GeneratedParentExport))
	if err != nil {
		fmt.Fprintf(stderr, "error: write generated parent Slots: %v\n", err)
		return 1
	}
	if widgetAddShouldEnsureParentExpandedInDesigner(ctx.PanelClass, overlayChainMode, emptyCanvasPanelButtonMode, richTextLeafMode, bareCanvasPanelLeafMode) {
		_, workingAsset, err = ensureWidgetAddExpandedInDesigner(workingAsset, ctx.DesignerParentExport)
		if err != nil {
			fmt.Fprintf(stderr, "error: ensure designer parent bExpandedInDesigner: %v\n", err)
			return 1
		}
		_, workingAsset, err = ensureWidgetAddExpandedInDesigner(workingAsset, ctx.GeneratedParentExport)
		if err != nil {
			fmt.Fprintf(stderr, "error: ensure generated parent bExpandedInDesigner: %v\n", err)
			return 1
		}
	}

	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, designerImageExport, "Slot", "ObjectProperty", objectRefValue(designerSlotExport), "")
	if err != nil {
		fmt.Fprintf(stderr, "error: write designer image Slot: %v\n", err)
		return 1
	}
	if widgetAddShouldWriteDesignerChildDisplayLabel(ctx.PanelClass, mode, parentHadChildren, emptyCanvasPanelButtonMode, richTextLeafMode, bareCanvasPanelLeafMode) ||
		widgetAddShouldForceChildDisplayLabel(ctx.PanelClass, childWidgetClass) {
		_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, designerImageExport, "DisplayLabel", "StrProperty", childName.Display, "")
		if err != nil {
			fmt.Fprintf(stderr, "error: write designer image DisplayLabel: %v\n", err)
			return 1
		}
	}
	if generatedImageExport >= 0 {
		_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, generatedImageExport, "Slot", "ObjectProperty", objectRefValue(generatedSlotExport), "")
		if err != nil {
			fmt.Fprintf(stderr, "error: write generated image Slot: %v\n", err)
			return 1
		}
		if widgetAddShouldForceChildDisplayLabel(ctx.PanelClass, childWidgetClass) ||
			(!emptyCanvasPanelButtonMode && (mode == widgetAddModeGeneratedRootlessNestedOverlay || !addGeneratedChild)) {
			_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, generatedImageExport, "DisplayLabel", "StrProperty", childName.Display, "")
			if err != nil {
				fmt.Fprintf(stderr, "error: write generated image DisplayLabel: %v\n", err)
				return 1
			}
		}
	}
	slotGuidString := ""
	if strings.EqualFold(childWidgetClass, "NamedSlot") {
		slotGuidString = generateWidgetAddGUIDString()
		slotGUID := map[string]any{
			"structType": "Guid",
			"value":      slotGuidString,
		}
		_, workingAsset, err = applyWidgetAddPropertyWriteWithFlags(workingAsset, designerImageExport, "SlotGuid", "StructProperty(Guid(/Script/CoreUObject))", slotGUID, 8, "Slot")
		if err != nil {
			fmt.Fprintf(stderr, "error: write designer NamedSlot SlotGuid: %v\n", err)
			return 1
		}
		if generatedImageExport >= 0 {
			_, workingAsset, err = applyWidgetAddPropertyWriteWithFlags(workingAsset, generatedImageExport, "SlotGuid", "StructProperty(Guid(/Script/CoreUObject))", slotGUID, 8, "Slot")
			if err != nil {
				fmt.Fprintf(stderr, "error: write generated NamedSlot SlotGuid: %v\n", err)
				return 1
			}
		}
	}

	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, designerSlotExport, "Parent", "ObjectProperty", objectRefValue(ctx.DesignerParentExport), "")
	if err != nil {
		fmt.Fprintf(stderr, "error: write designer slot Parent: %v\n", err)
		return 1
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, designerSlotExport, "Content", "ObjectProperty", objectRefValue(designerImageExport), "")
	if err != nil {
		fmt.Fprintf(stderr, "error: write designer slot Content: %v\n", err)
		return 1
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, generatedSlotExport, "Parent", "ObjectProperty", objectRefValue(ctx.GeneratedParentExport), "")
	if err != nil {
		fmt.Fprintf(stderr, "error: write generated slot Parent: %v\n", err)
		return 1
	}
	generatedSlotContent := map[string]any{"index": int32(0)}
	if generatedImageExport >= 0 {
		generatedSlotContent = objectRefValue(generatedImageExport)
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, generatedSlotExport, "Content", "ObjectProperty", generatedSlotContent, "")
	if err != nil {
		fmt.Fprintf(stderr, "error: write generated slot Content: %v\n", err)
		return 1
	}

	designerAllWidgets, err := appendWidgetAddObjectRefArrayValue(workingAsset, ctx.DesignerTreeExport, "AllWidgets", objectRefValue(designerImageExport))
	if err != nil {
		fmt.Fprintf(stderr, "error: read designer WidgetTree AllWidgets: %v\n", err)
		return 1
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, ctx.DesignerTreeExport, "AllWidgets", "ArrayProperty(ObjectProperty)", designerAllWidgets, "")
	if err != nil {
		fmt.Fprintf(stderr, "error: write designer WidgetTree AllWidgets: %v\n", err)
		return 1
	}
	generatedAllWidgetsValue := map[string]any{"index": int32(0)}
	if generatedImageExport >= 0 {
		generatedAllWidgetsValue = objectRefValue(generatedImageExport)
	}
	generatedAllWidgets, err := appendWidgetAddObjectRefArrayValue(workingAsset, ctx.GeneratedTreeExport, "AllWidgets", generatedAllWidgetsValue)
	if err != nil {
		fmt.Fprintf(stderr, "error: read generated WidgetTree AllWidgets: %v\n", err)
		return 1
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, ctx.GeneratedTreeExport, "AllWidgets", "ArrayProperty(ObjectProperty)", generatedAllWidgets, "")
	if err != nil {
		fmt.Fprintf(stderr, "error: write generated WidgetTree AllWidgets: %v\n", err)
		return 1
	}

	_, workingAsset, widgetGuid, err := appendWidgetVariableGuidEntry(workingAsset, ctx.Target.BlueprintExport, childName)
	if err != nil {
		fmt.Fprintf(stderr, "error: append WidgetVariableNameToGuidMap entry: %v\n", err)
		return 1
	}
	appendGeneratedClassVariable := widgetAddShouldAppendGeneratedClassVariable(childWidgetClass) && !strings.EqualFold(ctx.PanelClass, "NamedSlot")
	childClassImportIndex := 0
	if appendGeneratedClassVariable {
		resolvedImportIndex, importErr := resolveWidgetAddChildClassImportIndex(workingAsset, childClassSpec)
		if importErr != nil {
			fmt.Fprintf(stderr, "error: %v\n", importErr)
			return 1
		}
		childClassImportIndex = resolvedImportIndex
		_, workingAsset, err = appendWidgetAddGeneratedVariable(workingAsset, ctx.Target.BlueprintExport, workingAsset.Exports[ctx.Target.BlueprintExport].ObjectName.Display(workingAsset.Names), ctx.PanelClass, childWidgetClass, childClassImportIndex, childName, widgetGuid, parentHadChildren, overlayChainMode, mode == widgetAddModeGeneratedRootlessNestedOverlay)
		if err != nil {
			fmt.Fprintf(stderr, "error: append GeneratedVariables entry: %v\n", err)
			return 1
		}
	}
	if !addGeneratedChild {
		_, workingAsset, err = appendWidgetAddCategorySortingEntry(workingAsset, ctx.Target.BlueprintExport, ctx.BlueprintObjectName)
		if err != nil {
			fmt.Fprintf(stderr, "error: append CategorySorting entry: %v\n", err)
			return 1
		}
	}
	if appendGeneratedClassVariable {
		_, workingAsset, err = appendGeneratedClassPropertyGuidEntry(workingAsset, ctx.GeneratedClassExport, ctx.BlueprintObjectName, childWidgetClass, childClassImportIndex, childName, widgetGuid, overlayChainMode, mode == widgetAddModeGeneratedRootlessNestedOverlay, false, existingGeneratedClassLayout)
		if err != nil {
			fmt.Fprintf(stderr, "error: append PropertyGuids entry: %v\n", err)
			return 1
		}
	}
	if strings.EqualFold(childWidgetClass, "NamedSlot") {
		nameRef, resolveErr := resolveDisplayNameRef(workingAsset.Names, childName.Display)
		if resolveErr != nil {
			fmt.Fprintf(stderr, "error: resolve NamedSlot metadata name ref: %v\n", resolveErr)
			return 1
		}
		slotGuidValue, parseErr := parseWidgetAddGUIDString(slotGuidString)
		if parseErr != nil {
			fmt.Fprintf(stderr, "error: parse NamedSlot SlotGuid: %v\n", parseErr)
			return 1
		}
		_, workingAsset, err = appendGeneratedClassNamedSlotMetadata(workingAsset, ctx.GeneratedClassExport, nameRef, slotGuidValue)
		if err != nil {
			fmt.Fprintf(stderr, "error: append NamedSlot generated class metadata: %v\n", err)
			return 1
		}
		_, workingAsset, err = rewriteWidgetAddNamedSlotAssetRegistryTags(workingAsset, ctx.BlueprintObjectName, childName.Display)
		if err != nil {
			fmt.Fprintf(stderr, "error: rewrite NamedSlot asset registry tags: %v\n", err)
			return 1
		}
	}
	blueprintClassDependencyIndex := 0
	if widgetAddIsBlueprintGeneratedWidgetClass(childWidgetClass) {
		blueprintClassDependencyIndex = childClassImportIndex
	}
	_, workingAsset, err = applyWidgetAddDependsMap(workingAsset, ctx, designerSlotExport, generatedSlotExport, designerImageExport, generatedImageExport, blueprintClassDependencyIndex)
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite DependsMap: %v\n", err)
		return 1
	}
	if parentHadChildren || strings.Contains(ctx.Target.Path, "/") || strings.EqualFold(ctx.PanelClass, "Overlay") || strings.EqualFold(childWidgetClass, "Button") || overlayChainMode || mode == widgetAddModeGeneratedRootlessNestedOverlay || generatedImageExport >= 0 {
		_, workingAsset, err = rewriteWidgetAddBlueprintSoftObjectPaths(workingAsset, *opts, ctx)
		if err != nil {
			fmt.Fprintf(stderr, "error: rewrite soft object paths: %v\n", err)
			return 1
		}
	}
	_, workingAsset, err = normalizeWidgetAddTextNamespaces(workingAsset, ctx)
	if err != nil {
		fmt.Fprintf(stderr, "error: normalize widget text namespaces: %v\n", err)
		return 1
	}
	if overlayChainMode {
		_, workingAsset, err = rewriteWidgetAddOverlayRootChainCategorySorting(workingAsset, ctx.Target.BlueprintExport, ctx.BlueprintObjectName)
		if err != nil {
			fmt.Fprintf(stderr, "error: rewrite overlay root-chain CategorySorting: %v\n", err)
			return 1
		}
	}
	if overlayChainMode {
		_, workingAsset, err = ensureWidgetAddDisplayLabel(workingAsset, ctx.DesignerParentExport, ctx.Target.ObjectName)
		if err != nil {
			fmt.Fprintf(stderr, "error: ensure designer parent DisplayLabel: %v\n", err)
			return 1
		}
		_, workingAsset, err = ensureWidgetAddDisplayLabel(workingAsset, ctx.GeneratedParentExport, ctx.Target.ObjectName)
		if err != nil {
			fmt.Fprintf(stderr, "error: ensure generated parent DisplayLabel: %v\n", err)
			return 1
		}
		_, workingAsset, err = reorderWidgetAddOverlayRootChainExports(workingAsset, ctx)
		if err != nil {
			fmt.Fprintf(stderr, "error: reorder overlay root-chain exports: %v\n", err)
			return 1
		}
		ctx, err = refreshWidgetAddContext(workingAsset, *exportIndex, *parentSelector)
		if err != nil {
			fmt.Fprintf(stderr, "error: refresh overlay root-chain context: %v\n", err)
			return 1
		}
		designerImageExport, generatedImageExport = findWidgetAddChildExportsByPath(workingAsset, ctx.Target.BlueprintExport, ctx.Target.Path+"/"+childName.Display)
		designerSlotExport, generatedSlotExport = findWidgetAddSlotExportsByPath(workingAsset, ctx.Target.BlueprintExport, ctx.Target.Path+"/"+childName.Display)
		_, workingAsset, err = removeWidgetAddOverlayRootChainLegacyNames(workingAsset, *opts, ctx)
		if err != nil {
			fmt.Fprintf(stderr, "error: cleanup overlay root-chain names: %v\n", err)
			return 1
		}
		scriptEndHint := existingGeneratedClassLayout.ScriptEnd + int(workingAsset.Exports[ctx.GeneratedClassExport].SerialSize) - existingGeneratedClassSerialSize
		_, workingAsset, err = normalizeWidgetAddGeneratedClassFStringLengths(workingAsset, ctx.GeneratedClassExport, ctx.BlueprintObjectName, scriptEndHint)
		if err != nil {
			fmt.Fprintf(stderr, "error: normalize overlay root-chain generated class strings: %v\n", err)
			return 1
		}
	}
	if mode == widgetAddModeGeneratedRootlessNestedOverlay {
		_, workingAsset, err = reorderWidgetAddGeneratedRootlessTopLevelNestedExports(workingAsset, ctx)
		if err != nil {
			fmt.Fprintf(stderr, "error: reorder generated rootless nested exports: %v\n", err)
			return 1
		}
		ctx, err = refreshWidgetAddContext(workingAsset, *exportIndex, *parentSelector)
		if err != nil {
			fmt.Fprintf(stderr, "error: refresh generated rootless nested context: %v\n", err)
			return 1
		}
		_, workingAsset, err = rewriteWidgetAddGeneratedRootlessTopLevelNestedReferences(workingAsset, ctx)
		if err != nil {
			fmt.Fprintf(stderr, "error: rewrite generated rootless nested references: %v\n", err)
			return 1
		}
		ctx, err = refreshWidgetAddContext(workingAsset, *exportIndex, *parentSelector)
		if err != nil {
			fmt.Fprintf(stderr, "error: refresh generated rootless nested references context: %v\n", err)
			return 1
		}
		_, workingAsset, err = rewriteWidgetAddGeneratedRootlessTopLevelDependsMap(workingAsset, ctx)
		if err != nil {
			fmt.Fprintf(stderr, "error: rewrite generated rootless nested DependsMap: %v\n", err)
			return 1
		}
		_, workingAsset, err = rewriteWidgetAddGeneratedRootlessTopLevelImportedClassesTrailer(workingAsset, ctx)
		if err != nil {
			fmt.Fprintf(stderr, "error: rewrite generated rootless nested asset registry imported classes: %v\n", err)
			return 1
		}
	}
	if strings.Contains(ctx.Target.Path, "/") &&
		strings.EqualFold(ctx.PanelClass, "CanvasPanel") &&
		!overlayChainMode &&
		mode != widgetAddModeGeneratedRootlessNestedOverlay {
		_, workingAsset, err = rewriteWidgetAddPreorderMetadata(workingAsset, ctx)
		if err != nil {
			fmt.Fprintf(stderr, "error: rewrite WidgetTree preorder metadata: %v\n", err)
			return 1
		}
	}
	if emptyCanvasPanelButtonMode {
		_, workingAsset, err = reorderWidgetAddEmptyCanvasPanelGeneratedChildExports(workingAsset, ctx, childName)
		if err != nil {
			fmt.Fprintf(stderr, "error: reorder empty CanvasPanel generated-child exports: %v\n", err)
			return 1
		}
		ctx, err = refreshWidgetAddContext(workingAsset, *exportIndex, *parentSelector)
		if err != nil {
			fmt.Fprintf(stderr, "error: refresh empty CanvasPanel generated-child context: %v\n", err)
			return 1
		}
	}
	if fixtureEmptyCanvasPanelRichTextMode {
		_, workingAsset, err = reorderWidgetAddEmptyCanvasPanelRichTextExports(workingAsset, ctx, childName)
		if err != nil {
			fmt.Fprintf(stderr, "error: reorder empty CanvasPanel RichText exports: %v\n", err)
			return 1
		}
		ctx, err = refreshWidgetAddContext(workingAsset, *exportIndex, *parentSelector)
		if err != nil {
			fmt.Fprintf(stderr, "error: refresh empty CanvasPanel RichText context: %v\n", err)
			return 1
		}
	}
	_, workingAsset, err = normalizeRichTextWidgetCompileArtifacts(workingAsset, *opts, childWidgetClass, ctx.BlueprintObjectName)
	if err != nil {
		fmt.Fprintf(stderr, "error: normalize richtext widget-add compile artifacts: %v\n", err)
		return 1
	}
	finalBytes, workingAsset, err := finalizeWidgetBlueprintMutation(asset, workingAsset, *opts, ctx.BlueprintObjectName)
	if err != nil {
		fmt.Fprintf(stderr, "error: finalize widget-add: %v\n", err)
		return 1
	}

	if err := validateWidgetAddResult(workingAsset, ctx.Target.BlueprintExport, ctx.Target.Path, childName.Display, generatedImageExport >= 0); err != nil {
		fmt.Fprintf(stderr, "error: widget-add validation: %v\n", err)
		return 1
	}

	resp := map[string]any{
		"file":               file,
		"parent":             *parentSelector,
		"resolvedParentPath": ctx.Target.Path,
		"parentClassName":    ctx.PanelClass,
		"type":               normalizedType,
		"name":               childName.Display,
		"slotName":           slotName,
		"addedNames":         addedNames,
		"addedImports":       addedImports,
		"parentExports":      widgetWriteExportIndicesOneBased(ctx.Target.Exports),
		"slotExports":        []int{designerSlotExport + 1, generatedSlotExport + 1},
		"widgetExports":      widgetAddResponseWidgetExports(designerImageExport, generatedImageExport),
		"dryRun":             *dryRun,
		"changed":            !bytes.Equal(asset.Raw.Bytes, finalBytes),
		"outputBytes":        len(finalBytes),
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
	if err := writeFileAtomically(file, finalBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write file: %v\n", err)
		return 1
	}
	return printJSON(stdout, resp)
}

func runBlueprintWidgetAddRoot(asset *uasset.Asset, opts uasset.ParseOptions, file string, exportFilter int, normalizedType string, childName widgetAddName, dryRun bool, backup bool, stdout, stderr io.Writer) int {
	panelClass, err := widgetAddRootPanelClass(normalizedType)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	ctx, err := resolveWidgetAddRootContext(asset, exportFilter, panelClass)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	_, workingAsset, addedNames, addedImports, err := ensureWidgetAddRootPrerequisites(asset, opts, panelClass, childName)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	var workingBytes []byte
	ctx, err = resolveWidgetAddRootContext(workingAsset, exportFilter, panelClass)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	insertAt, err := findWidgetAddRootInsertPos(workingAsset, panelClass)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	rootEntries, err := buildWidgetAddRootEntries(workingAsset, ctx, childName)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	workingBytes, err = edit.InsertExportEntries(workingAsset, insertAt, rootEntries)
	if err != nil {
		fmt.Fprintf(stderr, "error: insert root widget exports: %v\n", err)
		return 1
	}
	workingAsset, err = uasset.ParseBytes(workingBytes, opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: reparse after root widget insert: %v\n", err)
		return 1
	}
	designerRootExport := insertAt
	generatedRootExport := insertAt + 1
	ctx.Target.BlueprintExport = remapWidgetAddInsertedExportIndex(ctx.Target.BlueprintExport, insertAt, len(rootEntries))
	ctx.GeneratedClassExport = remapWidgetAddInsertedExportIndex(ctx.GeneratedClassExport, insertAt, len(rootEntries))
	ctx.DesignerTreeExport = remapWidgetAddInsertedExportIndex(ctx.DesignerTreeExport, insertAt, len(rootEntries))
	ctx.GeneratedTreeExport = remapWidgetAddInsertedExportIndex(ctx.GeneratedTreeExport, insertAt, len(rootEntries))

	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, ctx.DesignerTreeExport, "RootWidget", "ObjectProperty", objectRefValue(designerRootExport), "")
	if err != nil {
		fmt.Fprintf(stderr, "error: write designer WidgetTree RootWidget: %v\n", err)
		return 1
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, ctx.GeneratedTreeExport, "RootWidget", "ObjectProperty", objectRefValue(generatedRootExport), "")
	if err != nil {
		fmt.Fprintf(stderr, "error: write generated WidgetTree RootWidget: %v\n", err)
		return 1
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, ctx.DesignerTreeExport, "AllWidgets", "ArrayProperty(ObjectProperty)", []any{objectRefValue(designerRootExport)}, "")
	if err != nil {
		fmt.Fprintf(stderr, "error: write designer WidgetTree AllWidgets: %v\n", err)
		return 1
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, ctx.GeneratedTreeExport, "AllWidgets", "ArrayProperty(ObjectProperty)", []any{objectRefValue(generatedRootExport)}, "")
	if err != nil {
		fmt.Fprintf(stderr, "error: write generated WidgetTree AllWidgets: %v\n", err)
		return 1
	}
	if strings.EqualFold(panelClass, "Overlay") {
		_, workingAsset, err = ensureWidgetAddDisplayLabel(workingAsset, designerRootExport, childName.Display)
		if err != nil {
			fmt.Fprintf(stderr, "error: write designer root DisplayLabel: %v\n", err)
			return 1
		}
		_, workingAsset, err = ensureWidgetAddDisplayLabel(workingAsset, generatedRootExport, childName.Display)
		if err != nil {
			fmt.Fprintf(stderr, "error: write generated root DisplayLabel: %v\n", err)
			return 1
		}
	}
	_, workingAsset, _, err = appendWidgetVariableGuidEntry(workingAsset, ctx.Target.BlueprintExport, childName)
	if err != nil {
		fmt.Fprintf(stderr, "error: append WidgetVariableNameToGuidMap entry: %v\n", err)
		return 1
	}
	_, workingAsset, err = applyWidgetAddRootDependsMap(workingAsset, ctx, designerRootExport)
	if err != nil {
		fmt.Fprintf(stderr, "error: update root DependsMap: %v\n", err)
		return 1
	}
	if !strings.EqualFold(panelClass, "CanvasPanel") {
		_, workingAsset, err = restoreWidgetBlueprintCompileArtifacts(workingAsset, opts, ctx.BlueprintObjectName, widgetCompileArtifactsSnapshot{
			softObjectPathOrder: widgetCompileArtifactsReversedOrder(),
			functionDocOrder:    widgetCompileArtifactsReversedOrder(),
		})
		if err != nil {
			fmt.Fprintf(stderr, "error: normalize root widget-add compile artifacts: %v\n", err)
			return 1
		}
	}
	finalBytes, workingAsset, err := finalizeWidgetBlueprintMutation(asset, workingAsset, opts, ctx.BlueprintObjectName)
	if err != nil {
		fmt.Fprintf(stderr, "error: finalize root widget-add: %v\n", err)
		return 1
	}
	if err := validateWidgetAddRootResult(workingAsset, ctx.Target.BlueprintExport, childName.Display); err != nil {
		fmt.Fprintf(stderr, "error: widget-add validation: %v\n", err)
		return 1
	}

	resp := map[string]any{
		"file":               file,
		"parent":             "root",
		"resolvedParentPath": "root",
		"type":               normalizedType,
		"name":               childName.Display,
		"addedNames":         addedNames,
		"addedImports":       addedImports,
		"widgetExports":      []int{designerRootExport + 1, generatedRootExport + 1},
		"dryRun":             dryRun,
		"changed":            !bytes.Equal(asset.Raw.Bytes, finalBytes),
		"outputBytes":        len(finalBytes),
	}
	if dryRun {
		return printJSON(stdout, resp)
	}
	if backup {
		if err := createBackupFile(file); err != nil {
			fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
			return 1
		}
	}
	if err := writeFileAtomically(file, finalBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write file: %v\n", err)
		return 1
	}
	return printJSON(stdout, resp)
}

func validateWidgetAddTarget(asset *uasset.Asset, targets []widgetWriteTarget, target widgetWriteTarget, childName string) error {
	if asset == nil {
		return fmt.Errorf("asset is nil")
	}
	panelClass := strings.TrimSpace(target.ClassName)
	policy, known := widgetChildPolicyForClass(panelClass)
	if !widgetAddSupportsParentClass(panelClass) {
		switch {
		case known && policy == widgetChildPolicyNone:
			return fmt.Errorf("parent class %q does not accept child widgets", target.ClassName)
		case known && policy == widgetChildPolicySingle:
			return fmt.Errorf("parent class %q is a single-child ContentWidget, but widget-add currently implements only CanvasPanel, Overlay, VerticalBox, HorizontalBox, StackBox, ScrollBox, WrapBox, GridPanel, UniformGridPanel, WidgetSwitcher, Button, Border, RetainerBox, InvalidationBox, MenuAnchor, SizeBox, ScaleBox, BackgroundBlur, SafeZone, and WindowTitleBarArea parents", target.ClassName)
		case known && policy == widgetChildPolicyMulti:
			return fmt.Errorf("parent class %q is a multi-child PanelWidget, but widget-add currently implements only CanvasPanel, Overlay, VerticalBox, HorizontalBox, StackBox, ScrollBox, WrapBox, GridPanel, UniformGridPanel, WidgetSwitcher, Button, Border, RetainerBox, InvalidationBox, MenuAnchor, SizeBox, ScaleBox, BackgroundBlur, SafeZone, and WindowTitleBarArea parents", target.ClassName)
		default:
			return fmt.Errorf("widget-add currently supports only CanvasPanel, Overlay, VerticalBox, HorizontalBox, StackBox, ScrollBox, WrapBox, GridPanel, UniformGridPanel, WidgetSwitcher, Button, Border, RetainerBox, InvalidationBox, MenuAnchor, SizeBox, ScaleBox, BackgroundBlur, SafeZone, or WindowTitleBarArea parents, got %q", target.ClassName)
		}
	}
	for _, other := range targets {
		if other.BlueprintExport != target.BlueprintExport {
			continue
		}
		if policy == widgetChildPolicySingle && widgetAddIsDirectChildPath(target.Path, other.Path) {
			return fmt.Errorf("single-child parent %q already has a child; %s accepts only one direct child", target.Path, target.ClassName)
		}
		if strings.EqualFold(other.ObjectName, childName) {
			return fmt.Errorf("widget %q already exists in this WidgetBlueprint", childName)
		}
		if strings.EqualFold(other.Path, target.Path+"/"+childName) {
			return fmt.Errorf("widget path %q already exists", target.Path+"/"+childName)
		}
	}
	return nil
}

type widgetChildPolicy int

const (
	widgetChildPolicyUnknown widgetChildPolicy = iota
	widgetChildPolicyNone
	widgetChildPolicySingle
	widgetChildPolicyMulti
)

func widgetChildPolicyForClass(className string) (widgetChildPolicy, bool) {
	switch strings.ToLower(strings.TrimSpace(className)) {
	case "image", "textblock", "richtextblock", "progressbar", "slider", "spacer", "scrollbar", "editabletext", "editabletextbox", "multilineeditabletextbox", "spinbox", "comboboxstring", "listview", "tileview", "treeview":
		return widgetChildPolicyNone, true
	case "backgroundblur", "border", "button", "checkbox", "invalidationbox", "menuanchor", "namedslot", "retainerbox", "safezone", "scalebox", "sizebox", "windowtitlebararea":
		return widgetChildPolicySingle, true
	case "canvaspanel", "gridpanel", "horizontalbox", "overlay", "scrollbox", "stackbox", "uniformgridpanel", "verticalbox", "widgetswitcher", "wrapbox":
		return widgetChildPolicyMulti, true
	default:
		return widgetChildPolicyUnknown, false
	}
}

func widgetAddSupportsParentClass(className string) bool {
	switch strings.ToLower(strings.TrimSpace(className)) {
	case "backgroundblur", "border", "button", "canvaspanel", "checkbox", "gridpanel", "horizontalbox", "invalidationbox", "menuanchor", "namedslot", "overlay", "retainerbox", "safezone", "scalebox", "scrollbox", "sizebox", "stackbox", "uniformgridpanel", "verticalbox", "widgetswitcher", "windowtitlebararea", "wrapbox":
		return true
	default:
		return false
	}
}

func widgetAddIsDirectChildPath(parentPath, candidatePath string) bool {
	parent := strings.TrimSpace(parentPath)
	candidate := strings.TrimSpace(candidatePath)
	if parent == "" || candidate == "" || strings.EqualFold(parent, candidate) {
		return false
	}
	prefix := parent + "/"
	if !strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(prefix)) {
		return false
	}
	rest := candidate[len(prefix):]
	return rest != "" && !strings.Contains(rest, "/")
}

func widgetAddLeafWidgetClass(normalizedType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(normalizedType)) {
	case "image":
		return "Image", nil
	case "textblock":
		return "TextBlock", nil
	case "richtextblock":
		return "RichTextBlock", nil
	case "progressbar":
		return "ProgressBar", nil
	case "slider":
		return "Slider", nil
	case "spacer":
		return "Spacer", nil
	case "scrollbar":
		return "ScrollBar", nil
	case "editabletext":
		return "EditableText", nil
	case "editabletextbox":
		return "EditableTextBox", nil
	case "multilineeditabletextbox":
		return "MultiLineEditableTextBox", nil
	case "spinbox":
		return "SpinBox", nil
	case "comboboxstring":
		return "ComboBoxString", nil
	case "checkbox":
		return "CheckBox", nil
	case "button":
		return "Button", nil
	case "border":
		return "Border", nil
	case "retainerbox":
		return "RetainerBox", nil
	case "invalidationbox":
		return "InvalidationBox", nil
	case "menuanchor":
		return "MenuAnchor", nil
	case "namedslot":
		return "NamedSlot", nil
	case "sizebox":
		return "SizeBox", nil
	case "scalebox":
		return "ScaleBox", nil
	case "backgroundblur":
		return "BackgroundBlur", nil
	case "safezone":
		return "SafeZone", nil
	case "windowtitlebararea":
		return "WindowTitleBarArea", nil
	case "canvaspanel":
		return "CanvasPanel", nil
	case "overlay":
		return "Overlay", nil
	case "verticalbox":
		return "VerticalBox", nil
	case "horizontalbox":
		return "HorizontalBox", nil
	case "stackbox":
		return "StackBox", nil
	case "scrollbox":
		return "ScrollBox", nil
	case "wrapbox":
		return "WrapBox", nil
	case "gridpanel":
		return "GridPanel", nil
	case "uniformgridpanel":
		return "UniformGridPanel", nil
	case "widgetswitcher":
		return "WidgetSwitcher", nil
	case "listview":
		return "ListView", nil
	case "tileview":
		return "TileView", nil
	case "treeview":
		return "TreeView", nil
	default:
		return "", fmt.Errorf("non-root widget-add currently supports only image, textblock, richtextblock, progressbar, slider, spacer, scrollbar, editabletext, editabletextbox, multilineeditabletextbox, spinbox, comboboxstring, checkbox, button, border, retainerbox, invalidationbox, menuanchor, namedslot, sizebox, scalebox, backgroundblur, safezone, windowtitlebararea, canvaspanel, overlay, verticalbox, horizontalbox, stackbox, scrollbox, wrapbox, gridpanel, uniformgridpanel, widgetswitcher, listview, tileview, or treeview, got %q", normalizedType)
	}
}

func resolveWidgetAddChildClassSpec(normalizedType, rawClassPath string) (widgetAddChildClassSpec, error) {
	if strings.EqualFold(strings.TrimSpace(normalizedType), "userwidget") {
		packagePath, objectName, ok := parseBlueprintGeneratedClassPath(rawClassPath)
		if !ok {
			return widgetAddChildClassSpec{}, fmt.Errorf("widget-add --type userwidget requires --class with a Blueprint asset path like /Game/UI/WBP_Status")
		}
		return widgetAddChildClassSpec{
			ResolvedClassName:    objectName,
			BlueprintPackagePath: packagePath,
			BlueprintObjectName:  objectName,
		}, nil
	}
	if strings.TrimSpace(rawClassPath) != "" {
		return widgetAddChildClassSpec{}, fmt.Errorf("widget-add --class is supported only with --type userwidget")
	}
	className, err := widgetAddLeafWidgetClass(normalizedType)
	if err != nil {
		return widgetAddChildClassSpec{}, err
	}
	return widgetAddChildClassSpec{ResolvedClassName: className}, nil
}

func widgetAddRootPanelClass(normalizedType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(normalizedType)) {
	case "canvaspanel":
		return "CanvasPanel", nil
	case "overlay":
		return "Overlay", nil
	case "border":
		return "Border", nil
	case "retainerbox":
		return "RetainerBox", nil
	case "invalidationbox":
		return "InvalidationBox", nil
	case "menuanchor":
		return "MenuAnchor", nil
	case "namedslot":
		return "NamedSlot", nil
	case "sizebox":
		return "SizeBox", nil
	case "scalebox":
		return "ScaleBox", nil
	case "backgroundblur":
		return "BackgroundBlur", nil
	case "safezone":
		return "SafeZone", nil
	case "windowtitlebararea":
		return "WindowTitleBarArea", nil
	case "verticalbox":
		return "VerticalBox", nil
	case "horizontalbox":
		return "HorizontalBox", nil
	case "button":
		return "Button", nil
	case "checkbox":
		return "CheckBox", nil
	case "stackbox":
		return "StackBox", nil
	case "scrollbox":
		return "ScrollBox", nil
	case "wrapbox":
		return "WrapBox", nil
	case "gridpanel":
		return "GridPanel", nil
	case "uniformgridpanel":
		return "UniformGridPanel", nil
	case "widgetswitcher":
		return "WidgetSwitcher", nil
	case "listview":
		return "ListView", nil
	case "tileview":
		return "TileView", nil
	case "treeview":
		return "TreeView", nil
	default:
		return "", fmt.Errorf("root widget-add supports only canvaspanel, overlay, border, retainerbox, invalidationbox, menuanchor, namedslot, sizebox, scalebox, backgroundblur, safezone, windowtitlebararea, verticalbox, horizontalbox, stackbox, scrollbox, wrapbox, gridpanel, uniformgridpanel, widgetswitcher, listview, tileview, treeview, checkbox, or button, got %q", normalizedType)
	}
}

func resolveWidgetAddRootContext(asset *uasset.Asset, exportFilter int, panelClass string) (widgetAddContext, error) {
	ctx := widgetAddContext{
		DesignerTreeExport:   -1,
		GeneratedTreeExport:  -1,
		GeneratedClassExport: -1,
		PanelClass:           panelClass,
	}
	targetBlueprints := findWidgetBlueprintExports(asset)
	if exportFilter > 0 {
		idx, err := asset.ResolveExportIndex(exportFilter)
		if err != nil {
			return ctx, err
		}
		targetBlueprints = []int{idx}
	}
	if len(targetBlueprints) == 0 {
		return ctx, fmt.Errorf("no WidgetBlueprint exports found")
	}
	if len(targetBlueprints) > 1 {
		return ctx, fmt.Errorf("multiple WidgetBlueprint exports found; use --export for root widget-add")
	}
	bpIdx := targetBlueprints[0]
	ctx.Target = widgetWriteTarget{
		BlueprintExport: bpIdx,
		ClassName:       panelClass,
		ObjectName:      asset.Exports[bpIdx].ObjectName.Display(asset.Names),
		Path:            "root",
	}
	ctx.BlueprintObjectName = ctx.Target.ObjectName

	bpProps := asset.ParseExportProperties(bpIdx)
	decoded := decodeAllProperties(asset, bpProps.Properties)
	if v, ok := decoded["GeneratedClass"]; ok {
		ctx.GeneratedClassExport = widgetExportIndexFromDecoded(v) - 1
	}
	if ctx.GeneratedClassExport < 0 || ctx.GeneratedClassExport >= len(asset.Exports) {
		return ctx, fmt.Errorf("GeneratedClass export not found for WidgetBlueprint %s", ctx.Target.ObjectName)
	}

	for _, treeIdx := range findWidgetTreeExports(asset, bpIdx, ctx.GeneratedClassExport) {
		treeExp := asset.Exports[treeIdx]
		outerIdx := resolveOuterExportIndex(asset, treeExp)
		outerClassName := ""
		if outerIdx >= 0 && outerIdx < len(asset.Exports) {
			outerClassName = asset.ResolveClassName(asset.Exports[outerIdx])
		}
		treeProps := asset.ParseExportProperties(treeIdx)
		treeDecoded := decodeAllProperties(asset, treeProps.Properties)
		rootWidgetExport := widgetExportIndexFromDecoded(treeDecoded["RootWidget"])
		if rootWidgetExport != 0 {
			return ctx, fmt.Errorf("WidgetBlueprint %s already has a root widget", ctx.Target.ObjectName)
		}
		switch widgetTreeRole(bpIdx, ctx.GeneratedClassExport, outerIdx, outerClassName) {
		case "designer":
			ctx.DesignerTreeExport = treeIdx
		case "generated":
			ctx.GeneratedTreeExport = treeIdx
		}
	}
	if ctx.DesignerTreeExport < 0 || ctx.GeneratedTreeExport < 0 {
		return ctx, fmt.Errorf("could not resolve designer/generated WidgetTree pair for %s", ctx.Target.ObjectName)
	}
	return ctx, nil
}

func ensureWidgetAddRootPrerequisites(asset *uasset.Asset, opts uasset.ParseOptions, panelClass string, childName widgetAddName) ([]byte, *uasset.Asset, []string, []map[string]any, error) {
	prefixNames := []string{
		"None",
		"/Script/CoreUObject",
		"Package",
		"Class",
		"/Script/UMG",
		"ArrayProperty",
		"ObjectProperty",
		"RootWidget",
		"AllWidgets",
		"MapProperty",
		"WidgetVariableNameToGuidMap",
		"NameProperty",
		"StructProperty",
		"Guid",
		childName.Base,
	}
	if strings.EqualFold(panelClass, "Overlay") {
		prefixNames = append(prefixNames, "DisplayLabel")
	}
	suffixNames := []string{panelClass}
	_, workingAsset, addedNames, err := insertBrushImageNames(asset, opts, prefixNames, suffixNames)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("ensure root widget-add names: %w", err)
	}
	_, workingAsset, err = normalizeWidgetAddTickEventTailStrings(workingAsset)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("normalize root widget-add tick event tails: %w", err)
	}
	workingBytes, workingAsset, addedImports, err := ensureWidgetAddUMGClassImports(workingAsset, opts, []string{panelClass})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("ensure root widget-add imports: %w", err)
	}
	return workingBytes, workingAsset, addedNames, addedImports, nil
}

func resolveWidgetAddContext(asset *uasset.Asset, target widgetWriteTarget) (widgetAddContext, error) {
	ctx := widgetAddContext{
		Target:                target,
		DesignerParentExport:  -1,
		GeneratedParentExport: -1,
		DesignerTreeExport:    -1,
		GeneratedTreeExport:   -1,
		GeneratedClassExport:  -1,
		CDOExport:             -1,
		BlueprintObjectName:   asset.Exports[target.BlueprintExport].ObjectName.Display(asset.Names),
		PanelClass:            target.ClassName,
	}
	if target.BlueprintExport < 0 || target.BlueprintExport >= len(asset.Exports) {
		return ctx, fmt.Errorf("widget blueprint export out of range: %d", target.BlueprintExport+1)
	}

	bpProps := asset.ParseExportProperties(target.BlueprintExport)
	decoded := decodeAllProperties(asset, bpProps.Properties)
	if v, ok := decoded["GeneratedClass"]; ok {
		ctx.GeneratedClassExport = widgetExportIndexFromDecoded(v) - 1
	}
	if ctx.GeneratedClassExport < 0 || ctx.GeneratedClassExport >= len(asset.Exports) {
		return ctx, fmt.Errorf("GeneratedClass export not found for WidgetBlueprint %s", target.ObjectName)
	}
	ctx.CDOExport = findWidgetBlueprintCDOExport(asset, ctx.GeneratedClassExport)
	if ctx.CDOExport < 0 {
		return ctx, fmt.Errorf("CDO export not found for GeneratedClass %d", ctx.GeneratedClassExport+1)
	}

	for _, treeIdx := range findWidgetTreeExports(asset, target.BlueprintExport, ctx.GeneratedClassExport) {
		treeExp := asset.Exports[treeIdx]
		outerIdx := resolveOuterExportIndex(asset, treeExp)
		outerClassName := ""
		if outerIdx >= 0 && outerIdx < len(asset.Exports) {
			outerClassName = asset.ResolveClassName(asset.Exports[outerIdx])
		}
		switch widgetTreeRole(target.BlueprintExport, ctx.GeneratedClassExport, outerIdx, outerClassName) {
		case "designer":
			ctx.DesignerTreeExport = treeIdx
		case "generated":
			ctx.GeneratedTreeExport = treeIdx
		}
	}

	for _, exportIdx := range target.Exports {
		if exportIdx < 0 || exportIdx >= len(asset.Exports) {
			return ctx, fmt.Errorf("target export out of range: %d", exportIdx+1)
		}
		treeIdx := resolveOuterExportIndex(asset, asset.Exports[exportIdx])
		if treeIdx < 0 || treeIdx >= len(asset.Exports) {
			return ctx, fmt.Errorf("target export %d does not live under a WidgetTree", exportIdx+1)
		}
		treeExp := asset.Exports[treeIdx]
		outerIdx := resolveOuterExportIndex(asset, treeExp)
		outerClassName := ""
		if outerIdx >= 0 && outerIdx < len(asset.Exports) {
			outerClassName = asset.ResolveClassName(asset.Exports[outerIdx])
		}
		switch widgetTreeRole(target.BlueprintExport, ctx.GeneratedClassExport, outerIdx, outerClassName) {
		case "designer":
			ctx.DesignerParentExport = exportIdx
		case "generated":
			ctx.GeneratedParentExport = exportIdx
		}
	}
	if ctx.DesignerParentExport < 0 || ctx.DesignerTreeExport < 0 || ctx.GeneratedTreeExport < 0 {
		return ctx, fmt.Errorf("could not resolve designer/generated WidgetTree pair for %s", target.Path)
	}
	return ctx, nil
}

func findWidgetBlueprintCDOExport(asset *uasset.Asset, generatedClassExport int) int {
	classIndex := uasset.PackageIndex(generatedClassExport + 1)
	for i, exp := range asset.Exports {
		if exp.ClassIndex != classIndex {
			continue
		}
		if exp.OuterIndex != 0 {
			continue
		}
		return i
	}
	return -1
}

func refreshWidgetAddContext(asset *uasset.Asset, exportFilter int, selector string) (widgetAddContext, error) {
	targets, err := collectWidgetWriteTargets(asset, exportFilter)
	if err != nil {
		return widgetAddContext{}, err
	}
	target, err := selectWidgetWriteTarget(targets, selector)
	if err != nil {
		return widgetAddContext{}, err
	}
	return resolveWidgetAddContext(asset, *target)
}

func parseWidgetAddName(raw string) (widgetAddName, error) {
	display := strings.TrimSpace(raw)
	if display == "" {
		return widgetAddName{}, fmt.Errorf("widget name must not be empty")
	}
	name := widgetAddName{Display: display, Base: display}
	sep := strings.LastIndex(display, "_")
	if sep <= 0 || sep >= len(display)-1 {
		return name, nil
	}
	base := display[:sep]
	suffix := display[sep+1:]
	n, err := strconv.Atoi(suffix)
	if err != nil || n < 0 {
		return name, nil
	}
	name.Base = base
	name.Number = int32(n + 1)
	return name, nil
}

func isWidgetAddRootSelector(selector string) bool {
	trimmed := strings.TrimSpace(selector)
	return strings.EqualFold(trimmed, "root") || trimmed == "/"
}

func widgetAddSlotName(slotClass string, slotNumber int32) string {
	return fmt.Sprintf("%s_%d", slotClass, slotNumber)
}

func nextWidgetAddSlotName(asset *uasset.Asset, panelClass string, defaultNextSuffix int32) (string, error) {
	if defaultNextSuffix < 0 {
		defaultNextSuffix = 0
	}
	slotClass, err := widgetAddSlotClassName(panelClass)
	if err != nil {
		return "", err
	}
	if asset == nil {
		return widgetAddSlotName(slotClass, defaultNextSuffix), nil
	}
	base := slotClass
	nextSuffix := defaultNextSuffix
	for _, exp := range asset.Exports {
		name, parsed := parseWidgetAddName(exp.ObjectName.Display(asset.Names))
		if parsed != nil || !strings.EqualFold(name.Base, base) {
			continue
		}
		suffix := name.Number - 1
		if suffix >= nextSuffix {
			nextSuffix = suffix + 1
		}
	}
	return widgetAddSlotName(slotClass, nextSuffix), nil
}

func widgetAddSlotDefaultSuffix(panelClass string, overlayChainMode bool) int32 {
	if strings.EqualFold(panelClass, "Overlay") && overlayChainMode {
		return 0
	}
	if strings.EqualFold(panelClass, "CanvasPanel") || strings.EqualFold(panelClass, "Overlay") {
		return 1
	}
	return 0
}

func widgetAddShouldEnsureParentExpandedInDesigner(panelClass string, overlayChainMode bool, emptyCanvasPanelButtonMode bool, richTextLeafMode bool, bareCanvasPanelLeafMode bool) bool {
	if emptyCanvasPanelButtonMode || richTextLeafMode || bareCanvasPanelLeafMode {
		return false
	}
	if overlayChainMode {
		return true
	}
	return strings.EqualFold(panelClass, "CanvasPanel") || strings.EqualFold(panelClass, "Overlay")
}

func widgetAddShouldWriteDesignerChildDisplayLabel(panelClass string, mode widgetAddMode, parentHadChildren bool, emptyCanvasPanelButtonMode bool, richTextLeafMode bool, bareCanvasPanelLeafMode bool) bool {
	if emptyCanvasPanelButtonMode || richTextLeafMode || bareCanvasPanelLeafMode {
		return false
	}
	if mode == widgetAddModeGeneratedRootlessNestedOverlay {
		return true
	}
	if parentHadChildren {
		return false
	}
	return strings.EqualFold(panelClass, "CanvasPanel") || strings.EqualFold(panelClass, "Overlay")
}

func widgetAddShouldForceChildDisplayLabel(parentPanelClass string, childWidgetClass string) bool {
	return strings.EqualFold(parentPanelClass, "CanvasPanel") && strings.EqualFold(childWidgetClass, "TextBlock")
}

func widgetAddUsesBareCanvasPanelLeafConventions(childWidgetClass string) bool {
	if widgetAddIsBlueprintGeneratedWidgetClass(childWidgetClass) {
		return true
	}
	switch {
	case strings.EqualFold(childWidgetClass, "EditableText"):
		return true
	case strings.EqualFold(childWidgetClass, "EditableTextBox"):
		return true
	case strings.EqualFold(childWidgetClass, "MultiLineEditableTextBox"):
		return true
	case strings.EqualFold(childWidgetClass, "NamedSlot"):
		return true
	case strings.EqualFold(childWidgetClass, "ProgressBar"):
		return true
	case strings.EqualFold(childWidgetClass, "SpinBox"):
		return true
	case strings.EqualFold(childWidgetClass, "ScrollBar"):
		return true
	case strings.EqualFold(childWidgetClass, "Slider"):
		return true
	case strings.EqualFold(childWidgetClass, "Spacer"):
		return true
	case strings.EqualFold(childWidgetClass, "ComboBoxString"):
		return true
	case strings.EqualFold(childWidgetClass, "ListView"):
		return true
	case strings.EqualFold(childWidgetClass, "TileView"):
		return true
	case strings.EqualFold(childWidgetClass, "TreeView"):
		return true
	default:
		return false
	}
}

func widgetAddUsesPostEventGraphLeafInsertion(childWidgetClass string) bool {
	switch {
	case strings.EqualFold(childWidgetClass, "NamedSlot"):
		return true
	case strings.EqualFold(childWidgetClass, "ListView"):
		return true
	case strings.EqualFold(childWidgetClass, "TileView"):
		return true
	case strings.EqualFold(childWidgetClass, "TreeView"):
		return true
	case strings.EqualFold(childWidgetClass, "MultiLineEditableTextBox"):
		return true
	case strings.EqualFold(childWidgetClass, "ProgressBar"):
		return true
	case strings.EqualFold(childWidgetClass, "RichTextBlock"):
		return true
	case strings.EqualFold(childWidgetClass, "ScrollBar"):
		return true
	case strings.EqualFold(childWidgetClass, "Slider"):
		return true
	case strings.EqualFold(childWidgetClass, "SpinBox"):
		return true
	case strings.EqualFold(childWidgetClass, "Spacer"):
		return true
	default:
		return false
	}
}

func widgetAddShouldInsertGeneratedChild(parentPath string, parentPanelClass string, childWidgetClass string, childName widgetAddName, overlayChainMode bool, parentHadChildren bool) bool {
	if widgetAddParentRequiresGeneratedChildPair(parentPanelClass) {
		return true
	}
	if widgetAddRequiresGeneratedChildPair(childWidgetClass) {
		return true
	}
	if childName.Number == 0 {
		if strings.EqualFold(parentPanelClass, "CanvasPanel") &&
			strings.EqualFold(childWidgetClass, "TextBlock") &&
			!strings.Contains(parentPath, "/") {
			return false
		}
		return true
	}
	return overlayChainMode && !parentHadChildren
}

func widgetAddRequiresGeneratedChildPair(childWidgetClass string) bool {
	if widgetAddIsBlueprintGeneratedWidgetClass(childWidgetClass) {
		return true
	}
	switch {
	case strings.EqualFold(childWidgetClass, "BackgroundBlur"):
		return true
	case strings.EqualFold(childWidgetClass, "Border"):
		return true
	case strings.EqualFold(childWidgetClass, "Button"):
		return true
	case strings.EqualFold(childWidgetClass, "CheckBox"):
		return true
	case strings.EqualFold(childWidgetClass, "ComboBoxString"):
		return true
	case strings.EqualFold(childWidgetClass, "EditableText"):
		return true
	case strings.EqualFold(childWidgetClass, "EditableTextBox"):
		return true
	case strings.EqualFold(childWidgetClass, "MultiLineEditableTextBox"):
		return true
	case strings.EqualFold(childWidgetClass, "ProgressBar"):
		return true
	case strings.EqualFold(childWidgetClass, "RichTextBlock"):
		return true
	case strings.EqualFold(childWidgetClass, "ScrollBar"):
		return true
	case strings.EqualFold(childWidgetClass, "Slider"):
		return true
	case strings.EqualFold(childWidgetClass, "SpinBox"):
		return true
	case strings.EqualFold(childWidgetClass, "Spacer"):
		return true
	case strings.EqualFold(childWidgetClass, "GridPanel"):
		return true
	case strings.EqualFold(childWidgetClass, "InvalidationBox"):
		return true
	case strings.EqualFold(childWidgetClass, "MenuAnchor"):
		return true
	case strings.EqualFold(childWidgetClass, "NamedSlot"):
		return true
	case strings.EqualFold(childWidgetClass, "RetainerBox"):
		return true
	case strings.EqualFold(childWidgetClass, "SafeZone"):
		return true
	case strings.EqualFold(childWidgetClass, "ScaleBox"):
		return true
	case strings.EqualFold(childWidgetClass, "ScrollBox"):
		return true
	case strings.EqualFold(childWidgetClass, "SizeBox"):
		return true
	case strings.EqualFold(childWidgetClass, "StackBox"):
		return true
	case strings.EqualFold(childWidgetClass, "ListView"):
		return true
	case strings.EqualFold(childWidgetClass, "TileView"):
		return true
	case strings.EqualFold(childWidgetClass, "TreeView"):
		return true
	case strings.EqualFold(childWidgetClass, "UniformGridPanel"):
		return true
	case strings.EqualFold(childWidgetClass, "VerticalBox"):
		return true
	case strings.EqualFold(childWidgetClass, "WidgetSwitcher"):
		return true
	case strings.EqualFold(childWidgetClass, "WindowTitleBarArea"):
		return true
	case strings.EqualFold(childWidgetClass, "WrapBox"):
		return true
	case strings.EqualFold(childWidgetClass, "HorizontalBox"):
		return true
	default:
		return false
	}
}

// UMG multi-child panels can be bindable designer widgets without becoming
// GeneratedVariables / PropertyGuids entries. The nested Overlay fixture proves
// that WidgetVariableNameToGuidMap and generated-widget companions are enough
// for these panel nodes; adding generated-class variable records regresses UE
// loadability on fresh widget-init assets.
func widgetAddShouldAppendGeneratedClassVariable(childWidgetClass string) bool {
	switch {
	case strings.EqualFold(childWidgetClass, "ScrollBar"):
		return false
	case strings.EqualFold(childWidgetClass, "Spacer"):
		return false
	}
	policy, ok := widgetChildPolicyForClass(childWidgetClass)
	if ok && policy == widgetChildPolicyMulti {
		return false
	}
	return true
}

func widgetAddParentRequiresGeneratedChildPair(parentPanelClass string) bool {
	switch {
	case strings.EqualFold(parentPanelClass, "BackgroundBlur"):
		return true
	case strings.EqualFold(parentPanelClass, "Border"):
		return true
	case strings.EqualFold(parentPanelClass, "Button"):
		return true
	case strings.EqualFold(parentPanelClass, "CheckBox"):
		return true
	case strings.EqualFold(parentPanelClass, "GridPanel"):
		return true
	case strings.EqualFold(parentPanelClass, "InvalidationBox"):
		return true
	case strings.EqualFold(parentPanelClass, "MenuAnchor"):
		return true
	case strings.EqualFold(parentPanelClass, "NamedSlot"):
		return true
	case strings.EqualFold(parentPanelClass, "RetainerBox"):
		return true
	case strings.EqualFold(parentPanelClass, "SafeZone"):
		return true
	case strings.EqualFold(parentPanelClass, "ScaleBox"):
		return true
	case strings.EqualFold(parentPanelClass, "ScrollBox"):
		return true
	case strings.EqualFold(parentPanelClass, "SizeBox"):
		return true
	case strings.EqualFold(parentPanelClass, "StackBox"):
		return true
	case strings.EqualFold(parentPanelClass, "UniformGridPanel"):
		return true
	case strings.EqualFold(parentPanelClass, "VerticalBox"):
		return true
	case strings.EqualFold(parentPanelClass, "WidgetSwitcher"):
		return true
	case strings.EqualFold(parentPanelClass, "WindowTitleBarArea"):
		return true
	case strings.EqualFold(parentPanelClass, "WrapBox"):
		return true
	case strings.EqualFold(parentPanelClass, "HorizontalBox"):
		return true
	default:
		return false
	}
}

func appendWidgetAddObjectRefArrayValue(asset *uasset.Asset, exportIndex int, propertyName string, appendedValue map[string]any) ([]any, error) {
	values, err := readWidgetAddObjectRefArrayValue(asset, exportIndex, propertyName)
	if err != nil {
		return nil, err
	}
	values = append(values, cloneAnyMapLocal(appendedValue))
	return values, nil
}

func readWidgetAddObjectRefArrayValue(asset *uasset.Asset, exportIndex int, propertyName string) ([]any, error) {
	current, err := decodeExportRootPropertyValue(asset, exportIndex, propertyName)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "property not found") {
			return []any{}, nil
		}
		return nil, err
	}
	m, ok := current.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a decoded array property", propertyName)
	}
	items, ok := m["value"].([]any)
	if !ok {
		return nil, fmt.Errorf("%s value is not an array", propertyName)
	}
	out := make([]any, 0, len(items))
	for _, item := range items {
		wrapped, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s entry is not an object", propertyName)
		}
		value, ok := wrapped["value"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s entry value is not an object", propertyName)
		}
		out = append(out, cloneAnyMapLocal(value))
	}
	return out, nil
}

func ensureWidgetAddPrerequisites(asset *uasset.Asset, opts uasset.ParseOptions, panelClass string, childClassSpec widgetAddChildClassSpec, childName widgetAddName, blueprintObjectName string, includeDisplayLabel bool, includeBlueprintCategory bool, includeExpandedInDesigner bool) ([]byte, *uasset.Asset, []string, []map[string]any, error) {
	requiredSuffixNames, err := widgetAddRequiredSuffixNames(panelClass, childName)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	requiredClasses, err := widgetAddRequiredClasses(panelClass, childClassSpec)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	_, workingAsset, addedNames, err := insertBrushImageNames(asset, opts, widgetAddRequiredPrefixNames(panelClass, childClassSpec.ResolvedClassName, childName, blueprintObjectName, includeDisplayLabel, includeBlueprintCategory, includeExpandedInDesigner), requiredSuffixNames)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("ensure widget-add names: %w", err)
	}
	_, workingAsset, err = normalizeWidgetAddTickEventTailStrings(workingAsset)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("normalize widget-add tick event tails: %w", err)
	}
	workingBytes, workingAsset, addedImports, err := ensureWidgetAddUMGClassImports(workingAsset, opts, requiredClasses)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("ensure widget-add imports: %w", err)
	}
	if strings.TrimSpace(childClassSpec.BlueprintPackagePath) != "" {
		beforeIdx, foundBefore := findObjectImportByPath(workingAsset, "/Script/UMG", "WidgetBlueprintGeneratedClass", childClassSpec.BlueprintPackagePath)
		workingBytes, workingAsset, _, err = appendWidgetBlueprintGeneratedClassImportIfMissing(workingAsset, opts, childClassSpec.BlueprintPackagePath, childClassSpec.BlueprintObjectName)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("ensure widget-add WidgetBlueprintGeneratedClass import: %w", err)
		}
		afterIdx, foundAfter := findObjectImportByPath(workingAsset, "/Script/UMG", "WidgetBlueprintGeneratedClass", childClassSpec.BlueprintPackagePath)
		if !foundAfter || afterIdx <= 0 {
			return nil, nil, nil, nil, fmt.Errorf("WidgetBlueprintGeneratedClass import %q not found after insertion", childClassSpec.BlueprintPackagePath)
		}
		if !foundBefore {
			addedImports = append(addedImports, importAddResult(workingAsset, afterIdx-1, true))
		} else if beforeIdx > 0 {
			addedImports = append(addedImports, importAddResult(workingAsset, beforeIdx-1, false))
		}
	}
	return workingBytes, workingAsset, addedNames, addedImports, nil
}

func normalizeWidgetAddTickEventTailStrings(asset *uasset.Asset) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}

	target := []byte{0x04, 0x00, 0x00, 0x00, '0', '.', '2', 0x00}
	replacement := []byte{0x04, 0x00, 0x00, 0x00, '0', '.', '0', 0x00}
	workingAsset := asset
	for exportIndex, exp := range workingAsset.Exports {
		if !strings.EqualFold(workingAsset.ResolveClassName(exp), "K2Node_Event") {
			continue
		}
		start := int(exp.SerialOffset)
		end := start + int(exp.SerialSize)
		if start < 0 || end > len(workingAsset.Raw.Bytes) || start >= end {
			continue
		}
		raw := workingAsset.Raw.Bytes[start:end]
		if !bytes.Contains(raw, []byte("In Delta Time")) || !bytes.Contains(raw, target) {
			continue
		}
		rewritten := bytes.Replace(raw, target, replacement, -1)
		if bytes.Equal(rewritten, raw) {
			continue
		}
		outBytes, err := edit.RewriteAsset(workingAsset, []edit.ExportMutation{{
			ExportIndex: exportIndex,
			Payload:     rewritten,
		}})
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite export[%d] tick tail strings: %w", exportIndex+1, err)
		}
		workingAsset, err = uasset.ParseBytes(outBytes, uasset.ParseOptions{KeepUnknown: true})
		if err != nil {
			return nil, nil, fmt.Errorf("reparse tick tail normalization: %w", err)
		}
	}
	return append([]byte(nil), workingAsset.Raw.Bytes...), workingAsset, nil
}

func widgetAddRequiredPrefixNames(panelClass string, childWidgetClass string, childName widgetAddName, blueprintObjectName string, includeDisplayLabel bool, includeBlueprintCategory bool, includeExpandedInDesigner bool) []string {
	out := []string{
		"None",
		"/Script/CoreUObject",
		"/Script/Engine",
		"Class",
		"/Script/UMG",
		"ArrayProperty",
		"BoolProperty",
		"ObjectProperty",
		"StructProperty",
		"NameProperty",
		"ByteProperty",
		"Slots",
		"Slot",
		"Parent",
		"Content",
		"Guid",
		"Category",
	}
	if !widgetAddIsBlueprintGeneratedWidgetClass(childWidgetClass) {
		out = append(out, childWidgetClass)
	}
	if strings.EqualFold(childWidgetClass, "NamedSlot") {
		out = append(out,
			"SlotGuid",
			"AvailableNamedSlots",
			"InstanceNamedSlots",
			"NamedSlots",
			"NamedSlotsWithID",
		)
	}
	if widgetAddShouldAppendGeneratedClassVariable(childWidgetClass) {
		out = append(out,
			"StrProperty",
			"TextProperty",
			"UInt64Property",
			"DisplayName",
			"EditInline",
			"GeneratedVariables",
			"PropertyGuids",
			"BPVariableDescription",
			"BPVariableMetaDataEntry",
			"EdGraphPinType",
			"VarName",
			"VarGuid",
			"VarType",
			"FriendlyName",
			"PropertyFlags",
			"RepNotifyFunc",
			"ReplicationCondition",
			"MetaDataArray",
			"DataKey",
			"DataValue",
			"DefaultValue",
			"ELifetimeCondition",
			"COND_None",
			"object",
		)
	}
	if includeExpandedInDesigner {
		out = append(out, "bExpandedInDesigner")
	}
	if includeBlueprintCategory {
		if categoryName := widgetAddBlueprintCategoryName(blueprintObjectName); categoryName != "" {
			out = append(out, categoryName)
		}
	}
	if includeDisplayLabel {
		out = append(out, "DisplayLabel")
	}
	if childName.Number > 0 && !strings.EqualFold(childName.Base, childWidgetClass) {
		out = append(out, childName.Base)
	}
	return out
}

func widgetAddBlueprintCategoryName(blueprintObjectName string) string {
	trimmed := strings.TrimSpace(blueprintObjectName)
	if trimmed == "" {
		return ""
	}
	replaced := strings.ReplaceAll(trimmed, "_", " ")
	var out strings.Builder
	var prev rune
	for i, r := range replaced {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prevIsLower := prev >= 'a' && prev <= 'z'
			prevIsDigit := prev >= '0' && prev <= '9'
			if prevIsLower || prevIsDigit {
				out.WriteByte(' ')
			}
		}
		out.WriteRune(r)
		prev = r
	}
	return out.String()
}

func widgetAddRequiredSuffixNames(panelClass string, childName widgetAddName) ([]string, error) {
	slotClass, err := widgetAddSlotClassName(panelClass)
	if err != nil {
		return nil, err
	}
	out := []string{slotClass}
	if childName.Number == 0 && strings.TrimSpace(childName.Display) != "" {
		out = append(out, childName.Display)
	}
	return out, nil
}

func widgetAddRequiredClasses(panelClass string, childClassSpec widgetAddChildClassSpec) ([]string, error) {
	slotClass, err := widgetAddSlotClassName(panelClass)
	if err != nil {
		return nil, err
	}
	out := []string{slotClass}
	if strings.TrimSpace(childClassSpec.BlueprintPackagePath) == "" {
		out = append(out, childClassSpec.ResolvedClassName)
	}
	return out, nil
}

func widgetAddSlotClassName(panelClass string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(panelClass)) {
	case "backgroundblur":
		return "BackgroundBlurSlot", nil
	case "border":
		return "BorderSlot", nil
	case "button":
		return "ButtonSlot", nil
	case "checkbox":
		return "PanelSlot", nil
	case "canvaspanel":
		return "CanvasPanelSlot", nil
	case "gridpanel":
		return "GridSlot", nil
	case "horizontalbox":
		return "HorizontalBoxSlot", nil
	case "invalidationbox":
		return "PanelSlot", nil
	case "menuanchor":
		return "PanelSlot", nil
	case "namedslot":
		return "PanelSlot", nil
	case "overlay":
		return "OverlaySlot", nil
	case "retainerbox":
		return "PanelSlot", nil
	case "safezone":
		return "SafeZoneSlot", nil
	case "scalebox":
		return "ScaleBoxSlot", nil
	case "scrollbox":
		return "ScrollBoxSlot", nil
	case "sizebox":
		return "SizeBoxSlot", nil
	case "stackbox":
		return "StackBoxSlot", nil
	case "uniformgridpanel":
		return "UniformGridSlot", nil
	case "verticalbox":
		return "VerticalBoxSlot", nil
	case "widgetswitcher":
		return "WidgetSwitcherSlot", nil
	case "windowtitlebararea":
		return "WindowTitleBarAreaSlot", nil
	case "wrapbox":
		return "WrapBoxSlot", nil
	default:
		return "", fmt.Errorf("widget-add slot class is not mapped for parent class %q", panelClass)
	}
}

func ensureWidgetAddUMGClassImports(asset *uasset.Asset, opts uasset.ParseOptions, classNames []string) ([]byte, *uasset.Asset, []map[string]any, error) {
	workingAsset := asset
	workingBytes := append([]byte(nil), asset.Raw.Bytes...)
	results := make([]map[string]any, 0, len(classNames))

	_, umgExists := findPackageImportByObjectPath(workingAsset, "/Script/UMG")
	if !umgExists {
		entry, err := buildImportAddEntry(workingAsset, "/Script/CoreUObject", "Package", 0, "/Script/UMG")
		if err != nil {
			return nil, nil, nil, err
		}
		insertAt := preferredPackageImportInsertPos(workingAsset)
		workingBytes, err = edit.InsertImportEntries(workingAsset, insertAt, []uasset.ImportEntry{entry})
		if err != nil {
			return nil, nil, nil, fmt.Errorf("insert /Script/UMG package import: %w", err)
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("reparse after /Script/UMG package import: %w", err)
		}
		results = append(results, importAddResult(workingAsset, insertAt, true))
	}

	missing := make([]string, 0, len(classNames))
	for _, className := range classNames {
		if _, found := findUMGClassImport(workingAsset, className); found {
			continue
		}
		missing = append(missing, className)
	}
	if len(missing) == 0 {
		return workingBytes, workingAsset, results, nil
	}

	sortWidgetAddClassNames(missing)
	for _, className := range missing {
		currentUMGPackageIndex, found := findPackageImportByObjectPath(workingAsset, "/Script/UMG")
		if !found {
			return nil, nil, nil, fmt.Errorf("/Script/UMG package import disappeared after prior insert")
		}
		insertAt := preferredUMGClassImportInsertPos(workingAsset, className)
		entry, err := buildImportAddEntry(workingAsset, "/Script/CoreUObject", "Class", uasset.PackageIndex(-currentUMGPackageIndex), className)
		if err != nil {
			return nil, nil, nil, err
		}
		workingBytes, err = edit.InsertImportEntries(workingAsset, insertAt, []uasset.ImportEntry{entry})
		if err != nil {
			return nil, nil, nil, fmt.Errorf("insert UMG class import %s: %w", className, err)
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("reparse after UMG class import %s: %w", className, err)
		}
		results = append(results, importAddResult(workingAsset, insertAt, true))
	}
	return workingBytes, workingAsset, results, nil
}

func findUMGClassImport(asset *uasset.Asset, className string) (int, bool) {
	for i, imp := range asset.Imports {
		if !strings.EqualFold(imp.ClassPackage.Display(asset.Names), "/Script/CoreUObject") {
			continue
		}
		if !strings.EqualFold(imp.ClassName.Display(asset.Names), "Class") {
			continue
		}
		if !strings.EqualFold(imp.ObjectName.Display(asset.Names), className) {
			continue
		}
		if imp.OuterIndex >= 0 {
			continue
		}
		outerIdx := imp.OuterIndex.ResolveIndex()
		if outerIdx < 0 || outerIdx >= len(asset.Imports) {
			continue
		}
		if !strings.EqualFold(asset.Imports[outerIdx].ObjectName.Display(asset.Names), "/Script/UMG") {
			continue
		}
		return i + 1, true
	}
	return 0, false
}

func resolveWidgetAddChildClassImportIndex(asset *uasset.Asset, childClassSpec widgetAddChildClassSpec) (int, error) {
	if strings.TrimSpace(childClassSpec.BlueprintPackagePath) != "" {
		idx, found := findObjectImportByPath(asset, "/Script/UMG", "WidgetBlueprintGeneratedClass", childClassSpec.BlueprintPackagePath)
		if !found {
			return 0, fmt.Errorf("WidgetBlueprintGeneratedClass import %q not found after prerequisite insert", childClassSpec.BlueprintPackagePath)
		}
		return idx, nil
	}
	idx, found := findUMGClassImport(asset, childClassSpec.ResolvedClassName)
	if !found {
		return 0, fmt.Errorf("%s class import not found after prerequisite insert", childClassSpec.ResolvedClassName)
	}
	return idx, nil
}

func preferredUMGClassImportInsertPos(asset *uasset.Asset, className string) int {
	target := strings.ToLower(strings.TrimSpace(className))
	lastUMGClass := -1
	for i, imp := range asset.Imports {
		if !strings.EqualFold(imp.ClassPackage.Display(asset.Names), "/Script/CoreUObject") {
			continue
		}
		if !strings.EqualFold(imp.ClassName.Display(asset.Names), "Class") {
			continue
		}
		if imp.OuterIndex >= 0 {
			continue
		}
		outerIdx := imp.OuterIndex.ResolveIndex()
		if outerIdx < 0 || outerIdx >= len(asset.Imports) {
			continue
		}
		if !strings.EqualFold(asset.Imports[outerIdx].ObjectName.Display(asset.Names), "/Script/UMG") {
			continue
		}
		current := strings.ToLower(strings.TrimSpace(imp.ObjectName.Display(asset.Names)))
		if current > target {
			return i
		}
		lastUMGClass = i
	}
	if lastUMGClass >= 0 {
		return lastUMGClass + 1
	}
	return len(asset.Imports)
}

func sortWidgetAddClassNames(items []string) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			left := strings.ToLower(strings.TrimSpace(items[i]))
			right := strings.ToLower(strings.TrimSpace(items[j]))
			if right < left {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func buildWidgetAddSlotEntry(asset *uasset.Asset, panelClass string, parentExport int, slotName string) (edit.ExportInsertEntry, error) {
	slotRef, err := resolveDisplayNameRef(asset.Names, slotName)
	if err != nil {
		return edit.ExportInsertEntry{}, fmt.Errorf("resolve slot object name %q: %w", slotName, err)
	}
	slotClass, err := widgetAddSlotClassName(panelClass)
	if err != nil {
		return edit.ExportInsertEntry{}, err
	}
	slotImportIdx, found := findUMGClassImport(asset, slotClass)
	if !found {
		return edit.ExportInsertEntry{}, fmt.Errorf("%s class import not found after prerequisite insert", slotClass)
	}
	payload, scriptLen, err := buildWidgetAddEmptyExportPayload(asset)
	if err != nil {
		return edit.ExportInsertEntry{}, err
	}
	return edit.ExportInsertEntry{
		Header:  widgetAddInsertedHeader(-slotImportIdx, parentExport+1, slotRef, len(payload), scriptLen),
		Payload: append([]byte(nil), payload...),
	}, nil
}

func buildWidgetAddChildEntries(asset *uasset.Asset, ctx widgetAddContext, childClassSpec widgetAddChildClassSpec, childName widgetAddName, includeGenerated bool) ([]edit.ExportInsertEntry, error) {
	childRef, err := resolveDisplayNameRef(asset.Names, childName.Display)
	if err != nil {
		return nil, fmt.Errorf("resolve child object name %q: %w", childName.Display, err)
	}
	classImportIdx, err := resolveWidgetAddChildClassImportIndex(asset, childClassSpec)
	if err != nil {
		return nil, err
	}
	payload, scriptLen, err := buildWidgetAddEmptyExportPayload(asset)
	if err != nil {
		return nil, err
	}
	entries := []edit.ExportInsertEntry{
		{
			Header:  widgetAddInsertedHeader(-classImportIdx, ctx.DesignerTreeExport+1, childRef, len(payload), scriptLen),
			Payload: payload,
		},
	}
	if includeGenerated {
		entries = append(entries, edit.ExportInsertEntry{
			Header:  widgetAddInsertedHeader(-classImportIdx, ctx.GeneratedTreeExport+1, childRef, len(payload), scriptLen),
			Payload: append([]byte(nil), payload...),
		})
	}
	return entries, nil
}

func buildWidgetAddGeneratedCompanionEntry(asset *uasset.Asset, className, objectName string, generatedTreeExport int) (edit.ExportInsertEntry, error) {
	childRef, err := resolveDisplayNameRef(asset.Names, objectName)
	if err != nil {
		return edit.ExportInsertEntry{}, fmt.Errorf("resolve generated companion object name %q: %w", objectName, err)
	}
	classImportIdx, found := findUMGClassImport(asset, className)
	if !found {
		return edit.ExportInsertEntry{}, fmt.Errorf("%s class import not found after prerequisite insert", className)
	}
	payload, scriptLen, err := buildWidgetAddEmptyExportPayload(asset)
	if err != nil {
		return edit.ExportInsertEntry{}, err
	}
	return edit.ExportInsertEntry{
		Header:  widgetAddInsertedHeader(-classImportIdx, generatedTreeExport+1, childRef, len(payload), scriptLen),
		Payload: append([]byte(nil), payload...),
	}, nil
}

func buildWidgetAddRootEntries(asset *uasset.Asset, ctx widgetAddContext, childName widgetAddName) ([]edit.ExportInsertEntry, error) {
	childRef, err := resolveDisplayNameRef(asset.Names, childName.Display)
	if err != nil {
		return nil, fmt.Errorf("resolve root widget object name %q: %w", childName.Display, err)
	}
	classImportIdx, found := findUMGClassImport(asset, ctx.PanelClass)
	if !found {
		return nil, fmt.Errorf("%s class import not found after prerequisite insert", ctx.PanelClass)
	}
	payload, scriptLen, err := buildWidgetAddEmptyExportPayload(asset)
	if err != nil {
		return nil, err
	}
	return []edit.ExportInsertEntry{
		{
			Header:  widgetAddInsertedHeader(-classImportIdx, ctx.DesignerTreeExport+1, childRef, len(payload), scriptLen),
			Payload: append([]byte(nil), payload...),
		},
		{
			Header:  widgetAddInsertedHeader(-classImportIdx, ctx.GeneratedTreeExport+1, childRef, len(payload), scriptLen),
			Payload: append([]byte(nil), payload...),
		},
	}, nil
}

func buildWidgetAddEmptyExportPayload(asset *uasset.Asset) ([]byte, int, error) {
	payload, err := buildTaggedNonePayload(asset)
	if err != nil {
		return nil, 0, err
	}
	if !asset.Summary.UsesUnversionedPropertySerialization() && asset.Summary.FileVersionUE5 >= 1011 {
		out := make([]byte, 0, len(payload)+1+4)
		out = append(out, 0)
		out = append(out, payload...)
		scriptLen := len(out)
		out = append(out, 0, 0, 0, 0)
		return out, scriptLen, nil
	}
	out := make([]byte, 0, len(payload)+4)
	out = append(out, payload...)
	scriptLen := len(out)
	out = append(out, 0, 0, 0, 0)
	return out, scriptLen, nil
}

func findWidgetAddImageInsertPos(asset *uasset.Asset) (int, error) {
	for i, exp := range asset.Exports {
		if strings.EqualFold(exp.ObjectName.Display(asset.Names), "EventGraph") {
			return i + 1, nil
		}
	}
	return 0, fmt.Errorf("EventGraph export not found; widget-add v1 requires the validated WidgetBlueprint fixture layout")
}

func findWidgetAddRootInsertPos(asset *uasset.Asset, panelClass string) (int, error) {
	for i, exp := range asset.Exports {
		if strings.EqualFold(exp.ObjectName.Display(asset.Names), "EventGraph") {
			if strings.EqualFold(panelClass, "CanvasPanel") {
				return i, nil
			}
			if strings.EqualFold(panelClass, "HorizontalBox") {
				return i + 1, nil
			}
			last := i
			for j := i + 1; j < len(asset.Exports); j++ {
				if int(asset.Exports[j].OuterIndex) != i+1 {
					break
				}
				last = j
			}
			return last + 1, nil
		}
	}
	return 0, fmt.Errorf("EventGraph export not found; root widget-add requires the validated WidgetBlueprint fixture layout")
}

func findWidgetAddChildWidgetInsertPos(asset *uasset.Asset, ctx widgetAddContext, designerSlotExport int, generatedSlotExport int, usePostEventGraphGap bool) (int, error) {
	defaultInsertAt, err := findWidgetAddImageInsertPos(asset)
	if err != nil {
		return 0, err
	}
	if strings.EqualFold(ctx.PanelClass, "NamedSlot") {
		return maxIntLocal(designerSlotExport, generatedSlotExport) + 1, nil
	}
	eventGraphExport, firstEventGraphChildExport, lastEventGraphChildExport, err := findWidgetAddEventGraphGap(asset)
	if err != nil {
		return 0, err
	}
	if usePostEventGraphGap {
		if lastEventGraphChildExport >= 0 {
			return lastEventGraphChildExport + 1, nil
		}
		return eventGraphExport + 1, nil
	}
	parentStart := minIntLocal(ctx.DesignerParentExport, ctx.GeneratedParentExport)
	if parentStart > eventGraphExport && (firstEventGraphChildExport < 0 || parentStart < firstEventGraphChildExport) {
		return maxIntLocal(designerSlotExport, generatedSlotExport) + 1, nil
	}
	return defaultInsertAt, nil
}

func findWidgetAddEventGraphGap(asset *uasset.Asset) (int, int, int, error) {
	if asset == nil {
		return -1, -1, -1, fmt.Errorf("asset is nil")
	}
	for i, exp := range asset.Exports {
		if !strings.EqualFold(exp.ObjectName.Display(asset.Names), "EventGraph") {
			continue
		}
		firstChild := -1
		lastChild := -1
		for j := i + 1; j < len(asset.Exports); j++ {
			if int(asset.Exports[j].OuterIndex) == i+1 {
				if firstChild < 0 {
					firstChild = j
				}
				lastChild = j
			}
		}
		return i, firstChild, lastChild, nil
	}
	return -1, -1, -1, fmt.Errorf("EventGraph export not found; widget-add v1 requires the validated WidgetBlueprint fixture layout")
}

func findWidgetAddChildInsertPos(asset *uasset.Asset, outerExport int, defaultInsertAt int) (int, error) {
	if asset == nil {
		return 0, fmt.Errorf("asset is nil")
	}
	if defaultInsertAt < 0 {
		defaultInsertAt = 0
	}
	if defaultInsertAt > len(asset.Exports) {
		defaultInsertAt = len(asset.Exports)
	}
	last := -1
	for i, exp := range asset.Exports {
		if int(exp.OuterIndex) == outerExport+1 {
			last = i
		}
	}
	if last >= 0 {
		return last + 1, nil
	}
	return defaultInsertAt, nil
}

func remapWidgetAddInsertedExportIndex(exportIndex, insertAt, insertedCount int) int {
	if exportIndex >= insertAt {
		return exportIndex + insertedCount
	}
	return exportIndex
}

func maxIntLocal(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minIntLocal(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func widgetAddInsertedHeader(classImportIndex int, outerExportIndex int, objectName uasset.NameRef, payloadLen int, scriptLen int) uasset.ExportEntry {
	out := uasset.ExportEntry{}
	out.ClassIndex = uasset.PackageIndex(classImportIndex)
	out.SuperIndex = 0
	out.TemplateIndex = 0
	out.OuterIndex = uasset.PackageIndex(outerExportIndex)
	out.ObjectName = objectName
	out.ObjectFlags = 8
	out.ForcedExport = false
	out.NotForClient = false
	out.NotForServer = false
	out.NotAlwaysLoadedForEditor = false
	out.IsAsset = false
	out.IsInheritedInstance = false
	out.GeneratePublicHash = false
	out.PackageFlags = 0
	out.FirstExportDependency = -1
	out.SerializationBeforeSerializationDeps = 0
	out.CreateBeforeSerializationDeps = 0
	out.SerializationBeforeCreateDependencies = 0
	out.CreateBeforeCreateDependencies = 0
	out.ScriptSerializationStartOffset = 0
	out.ScriptSerializationEndOffset = int64(scriptLen)
	return out
}

func applyWidgetAddPropertyWrite(asset *uasset.Asset, exportIndex int, propertyName, propertyType string, value any, beforeProperty string) ([]byte, *uasset.Asset, error) {
	return applyWidgetAddPropertyWriteWithFlags(asset, exportIndex, propertyName, propertyType, value, 0, beforeProperty)
}

func applyWidgetAddPropertyRemove(asset *uasset.Asset, exportIndex int, propertyName string) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	editResult, err := edit.BuildPropertyRemoveMutation(asset, exportIndex, propertyName)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "property not found") {
			return append([]byte(nil), asset.Raw.Bytes...), asset, nil
		}
		return nil, nil, err
	}
	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{editResult.Mutation})
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite asset: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(outBytes, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, nil, fmt.Errorf("reparse rewritten asset: %w", err)
	}
	return outBytes, updatedAsset, nil
}

func rewriteWidgetAddExportHeader(asset *uasset.Asset, exportIndex int, classIndex uasset.PackageIndex, outerIndex uasset.PackageIndex, objectName uasset.NameRef) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if exportIndex < 0 || exportIndex >= len(asset.Exports) {
		return nil, nil, fmt.Errorf("export index out of range: %d", exportIndex+1)
	}
	positions, err := scanExportHeaderPositions(asset)
	if err != nil {
		return nil, nil, err
	}
	if len(positions) != len(asset.Exports) {
		return nil, nil, fmt.Errorf("export header position mismatch")
	}
	pos := positions[exportIndex]
	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}
	out := append([]byte(nil), asset.Raw.Bytes...)
	order.PutUint32(out[pos.classIndex:pos.classIndex+4], uint32(int32(classIndex)))
	order.PutUint32(out[pos.outerIndex:pos.outerIndex+4], uint32(int32(outerIndex)))
	order.PutUint32(out[pos.objectName:pos.objectName+4], uint32(objectName.Index))
	order.PutUint32(out[pos.objectName+4:pos.objectName+8], uint32(objectName.Number))
	if err := edit.FinalizePackageBytes(out, asset.Summary.FileVersionUE5); err != nil {
		return nil, nil, fmt.Errorf("finalize package bytes: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(out, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, nil, fmt.Errorf("reparse rewritten asset: %w", err)
	}
	return out, updatedAsset, nil
}

func rewriteWidgetAddExportOuterIndex(asset *uasset.Asset, exportIndex int, outerIndex uasset.PackageIndex) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if exportIndex < 0 || exportIndex >= len(asset.Exports) {
		return nil, nil, fmt.Errorf("export index out of range: %d", exportIndex+1)
	}
	exp := asset.Exports[exportIndex]
	return rewriteWidgetAddExportHeader(asset, exportIndex, exp.ClassIndex, outerIndex, exp.ObjectName)
}

func ensureWidgetAddExpandedInDesigner(asset *uasset.Asset, exportIndex int) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	_, err := decodeExportRootPropertyValue(asset, exportIndex, "bExpandedInDesigner")
	if err == nil {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	if !strings.Contains(strings.ToLower(err.Error()), "property not found") {
		return nil, nil, err
	}
	return applyWidgetAddPropertyWriteWithFlags(asset, exportIndex, "bExpandedInDesigner", "BoolProperty", true, 16, "")
}

func ensureWidgetAddDisplayLabel(asset *uasset.Asset, exportIndex int, label string) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	current, err := decodeExportRootPropertyValue(asset, exportIndex, "DisplayLabel")
	if err == nil {
		if currentText, ok := current.(string); ok && currentText == label {
			return append([]byte(nil), asset.Raw.Bytes...), asset, nil
		}
	} else if !strings.Contains(strings.ToLower(err.Error()), "property not found") {
		return nil, nil, err
	}
	return applyWidgetAddPropertyWrite(asset, exportIndex, "DisplayLabel", "StrProperty", label, "")
}

func isWidgetAddOverlayRootChainMode(asset *uasset.Asset, ctx widgetAddContext) bool {
	if asset == nil {
		return false
	}
	if !strings.EqualFold(ctx.PanelClass, "Overlay") {
		return false
	}
	if strings.Contains(ctx.Target.Path, "/") {
		return false
	}
	if ctx.DesignerParentExport >= 0 && ctx.GeneratedParentExport >= 0 {
		eventGraphExport := -1
		for i, exp := range asset.Exports {
			if strings.EqualFold(exp.ObjectName.Display(asset.Names), "EventGraph") {
				eventGraphExport = i
				break
			}
		}
		if eventGraphExport >= 0 && ctx.DesignerParentExport < eventGraphExport && ctx.GeneratedParentExport < eventGraphExport {
			return true
		}
	}
	if ctx.DesignerParentExport < 0 || ctx.GeneratedParentExport < 0 || ctx.DesignerTreeExport < 0 || ctx.GeneratedTreeExport < 0 {
		return false
	}
	if !widgetAddIsRootWidget(asset, ctx.DesignerTreeExport, ctx.DesignerParentExport) || !widgetAddIsRootWidget(asset, ctx.GeneratedTreeExport, ctx.GeneratedParentExport) {
		return false
	}
	return widgetAddHasDisplayLabel(asset, ctx.DesignerParentExport, ctx.Target.ObjectName) &&
		widgetAddHasDisplayLabel(asset, ctx.GeneratedParentExport, ctx.Target.ObjectName)
}

func resolveWidgetAddMode(asset *uasset.Asset, ctx widgetAddContext) widgetAddMode {
	if isWidgetAddGeneratedRootlessNestedOverlayMode(asset, ctx) {
		return widgetAddModeGeneratedRootlessNestedOverlay
	}
	if isWidgetAddOverlayRootChainMode(asset, ctx) {
		return widgetAddModeOverlayRootChain
	}
	return widgetAddModeGeneral
}

func isWidgetAddGeneratedRootlessNestedOverlayMode(asset *uasset.Asset, ctx widgetAddContext) bool {
	if asset == nil {
		return false
	}
	if asset.Summary.ThumbnailTableOffset <= 0 {
		return false
	}
	if ctx.GeneratedParentExport >= 0 {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(ctx.PanelClass), "Overlay") {
		return false
	}
	if len(splitWidgetPathSegments(ctx.Target.Path)) != 2 {
		return false
	}
	if ctx.DesignerParentExport < 0 || ctx.DesignerTreeExport < 0 || ctx.GeneratedTreeExport < 0 {
		return false
	}
	designerRootExport := widgetExportIndexFromDecodedMust(asset, ctx.DesignerTreeExport, "RootWidget") - 1
	generatedRootExport := widgetExportIndexFromDecodedMust(asset, ctx.GeneratedTreeExport, "RootWidget") - 1
	if designerRootExport < 0 || generatedRootExport < 0 {
		return false
	}
	designerChildren, err := widgetAddTopLevelChildrenFromSlots(asset, designerRootExport)
	if err != nil || len(designerChildren) == 0 {
		return false
	}
	generatedRootSlots, err := widgetAddSlotExportsFromSlots(asset, generatedRootExport)
	if err != nil || len(designerChildren) != len(generatedRootSlots) {
		return false
	}
	for _, designerChildExport := range designerChildren {
		if designerChildExport == ctx.DesignerParentExport {
			return true
		}
	}
	return false
}

func widgetAddIsRootWidget(asset *uasset.Asset, treeExport, widgetExport int) bool {
	if asset == nil || treeExport < 0 || treeExport >= len(asset.Exports) || widgetExport < 0 || widgetExport >= len(asset.Exports) {
		return false
	}
	decoded, err := decodeExportRootPropertyValue(asset, treeExport, "RootWidget")
	if err != nil {
		return false
	}
	ref, ok := decoded.(map[string]any)
	if !ok {
		return false
	}
	raw, ok := widgetAddInt64(ref["index"])
	return ok && int(raw) == widgetExport+1
}

func widgetAddHasDisplayLabel(asset *uasset.Asset, exportIndex int, expected string) bool {
	decoded, err := decodeExportRootPropertyValue(asset, exportIndex, "DisplayLabel")
	if err != nil {
		return false
	}
	value, ok := decoded.(string)
	return ok && strings.EqualFold(value, expected)
}

func widgetAddResponseWidgetExports(designerImageExport, generatedImageExport int) []int {
	out := []int{designerImageExport + 1}
	if generatedImageExport >= 0 {
		out = append(out, generatedImageExport+1)
	}
	return out
}

func widgetAddParentSlotsBeforeProperty(asset *uasset.Asset, exportIndex int) string {
	if asset == nil || exportIndex < 0 || exportIndex >= len(asset.Exports) {
		return "bExpandedInDesigner"
	}
	if _, err := decodeExportRootPropertyValue(asset, exportIndex, "SlotGuid"); err == nil {
		return "Slot"
	}
	if _, err := decodeExportRootPropertyValue(asset, exportIndex, "DisplayLabel"); err == nil {
		return "DisplayLabel"
	}
	if _, err := decodeExportRootPropertyValue(asset, exportIndex, "Slot"); err == nil {
		return "Slot"
	}
	return "bExpandedInDesigner"
}

func ensureWidgetAddGeneratedParentCompanion(asset *uasset.Asset, opts uasset.ParseOptions, ctx widgetAddContext, attachExistingGeneratedSlot bool) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if ctx.GeneratedParentExport >= 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	if ctx.GeneratedTreeExport < 0 {
		return nil, nil, fmt.Errorf("generated WidgetTree export not resolved for %s", ctx.Target.Path)
	}
	if strings.TrimSpace(ctx.Target.ObjectName) == "" {
		return nil, nil, fmt.Errorf("generated widget companion requires target object name")
	}
	if strings.TrimSpace(ctx.PanelClass) == "" {
		return nil, nil, fmt.Errorf("generated widget companion requires target class name")
	}
	generatedParentSlotExport := -1
	if attachExistingGeneratedSlot {
		_, generatedParentSlotExport = findWidgetAddSlotExportsByPath(asset, ctx.Target.BlueprintExport, ctx.Target.Path)
		if generatedParentSlotExport < 0 {
			return nil, nil, fmt.Errorf("generated parent slot companion is missing for %s", ctx.Target.Path)
		}
	}

	entry, err := buildWidgetAddGeneratedCompanionEntry(asset, ctx.PanelClass, ctx.Target.ObjectName, ctx.GeneratedTreeExport)
	if err != nil {
		return nil, nil, err
	}
	insertDefault, err := findWidgetAddImageInsertPos(asset)
	if err != nil {
		return nil, nil, err
	}
	insertAt, err := findWidgetAddChildInsertPos(asset, ctx.GeneratedTreeExport, insertDefault)
	if err != nil {
		return nil, nil, err
	}
	workingBytes, err := edit.InsertExportEntries(asset, insertAt, []edit.ExportInsertEntry{entry})
	if err != nil {
		return nil, nil, fmt.Errorf("insert generated widget companion export: %w", err)
	}
	workingAsset, err := uasset.ParseBytes(workingBytes, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("reparse after generated widget companion insert: %w", err)
	}

	generatedParentExport := insertAt
	generatedTreeExport := remapWidgetAddInsertedExportIndex(ctx.GeneratedTreeExport, insertAt, 1)
	if generatedParentSlotExport >= 0 {
		generatedParentSlotExport = remapWidgetAddInsertedExportIndex(generatedParentSlotExport, insertAt, 1)
	}
	allWidgets, err := readWidgetAddObjectRefArrayValue(workingAsset, generatedTreeExport, "AllWidgets")
	if err != nil {
		return nil, nil, fmt.Errorf("read generated WidgetTree AllWidgets for companion: %w", err)
	}
	// Fresh widget-init assets already serialize generated root Slots plus
	// placeholder AllWidgets entries for top-level children. Replace the matching
	// placeholder instead of appending a new entry, or UE will see an extra null
	// hole after the nested panel is materialized.
	replacedPlaceholder := false
	if generatedParentSlotExport >= 0 {
		generatedRootExport := widgetExportIndexFromDecodedMust(workingAsset, generatedTreeExport, "RootWidget") - 1
		if generatedRootExport >= 0 {
			generatedRootSlots, slotErr := widgetAddSlotExportsFromSlots(workingAsset, generatedRootExport)
			if slotErr != nil {
				return nil, nil, fmt.Errorf("read generated root slots for companion: %w", slotErr)
			}
			for ordinal, slotExport := range generatedRootSlots {
				if slotExport != generatedParentSlotExport {
					continue
				}
				allWidgetsIndex := ordinal + 1
				if allWidgetsIndex >= 0 && allWidgetsIndex < len(allWidgets) && widgetAddDecodedObjectRefIsNull(allWidgets[allWidgetsIndex]) {
					allWidgets[allWidgetsIndex] = objectRefValue(generatedParentExport)
					replacedPlaceholder = true
				}
				break
			}
		}
	}
	if !replacedPlaceholder {
		allWidgets = append(allWidgets, objectRefValue(generatedParentExport))
	}
	workingBytes, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, generatedTreeExport, "AllWidgets", "ArrayProperty(ObjectProperty)", allWidgets, "")
	if err != nil {
		return nil, nil, fmt.Errorf("write generated WidgetTree AllWidgets for companion: %w", err)
	}
	if generatedParentSlotExport >= 0 {
		_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, generatedParentExport, "Slot", "ObjectProperty", objectRefValue(generatedParentSlotExport), "")
		if err != nil {
			return nil, nil, fmt.Errorf("write generated widget companion Slot: %w", err)
		}
		workingBytes, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, generatedParentSlotExport, "Content", "ObjectProperty", objectRefValue(generatedParentExport), "")
		if err != nil {
			return nil, nil, fmt.Errorf("write generated parent slot Content: %w", err)
		}
	}
	return workingBytes, workingAsset, nil
}

func rewriteWidgetAddGeneratedTreeRootlessTopLevelParent(asset *uasset.Asset, opts uasset.ParseOptions, ctx widgetAddContext) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if ctx.GeneratedParentExport >= 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	pathSegments := splitWidgetPathSegments(ctx.Target.Path)
	if len(pathSegments) != 2 || ctx.DesignerParentExport < 0 || ctx.DesignerTreeExport < 0 || ctx.GeneratedTreeExport < 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	designerRootExport := widgetExportIndexFromDecodedMust(asset, ctx.DesignerTreeExport, "RootWidget") - 1
	generatedRootExport := widgetExportIndexFromDecodedMust(asset, ctx.GeneratedTreeExport, "RootWidget") - 1
	if designerRootExport < 0 || generatedRootExport < 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	designerChildren, err := widgetAddTopLevelChildrenFromSlots(asset, designerRootExport)
	if err != nil {
		return nil, nil, fmt.Errorf("designer top-level children: %w", err)
	}
	generatedRootSlots, err := widgetAddSlotExportsFromSlots(asset, generatedRootExport)
	if err != nil {
		return nil, nil, fmt.Errorf("generated root slots: %w", err)
	}
	if len(designerChildren) == 0 || len(designerChildren) != len(generatedRootSlots) {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	targetOrdinal := -1
	for i, exportIdx := range designerChildren {
		if exportIdx == ctx.DesignerParentExport {
			targetOrdinal = i
			break
		}
	}
	if targetOrdinal < 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	slotClass, err := widgetAddSlotClassName(ctx.PanelClass)
	if err != nil {
		return nil, nil, err
	}
	placeholderSlotName := widgetAddSlotName(slotClass, 0)
	var workingBytes []byte
	workingAsset := asset

	companionPool := make([]int, 0, len(designerChildren))
	companionPool = append(companionPool, generatedRootExport)
	for i, slotExport := range generatedRootSlots {
		if i == targetOrdinal {
			continue
		}
		companionPool = append(companionPool, slotExport)
	}
	if len(companionPool) != len(designerChildren) {
		return nil, nil, fmt.Errorf("generated companion pool mismatch: got %d want %d", len(companionPool), len(designerChildren))
	}

	targetGeneratedParentExport := -1
	for i, designerChildExport := range designerChildren {
		_, workingAsset, err = rewriteWidgetAddGeneratedRootlessCompanionExport(workingAsset, opts, companionPool[i], ctx.GeneratedTreeExport, designerChildExport)
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite generated companion for export %d: %w", designerChildExport+1, err)
		}
		if designerChildExport == ctx.DesignerParentExport {
			targetGeneratedParentExport = companionPool[i]
		}
	}
	if targetGeneratedParentExport < 0 {
		return nil, nil, fmt.Errorf("target generated companion export not resolved for %s", ctx.Target.Path)
	}

	reusableGeneratedSlotExport := generatedRootSlots[targetOrdinal]
	_, workingAsset, err = rewriteWidgetAddGeneratedSlotPlaceholder(workingAsset, opts, reusableGeneratedSlotExport, targetGeneratedParentExport, placeholderSlotName, slotClass)
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite generated slot placeholder: %w", err)
	}

	allWidgets := make([]any, 0, len(companionPool)+1)
	allWidgets = append(allWidgets, map[string]any{"index": int32(0)})
	for _, exportIdx := range companionPool {
		allWidgets = append(allWidgets, objectRefValue(exportIdx))
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, ctx.GeneratedTreeExport, "RootWidget", "ObjectProperty", map[string]any{"index": int32(0)}, "")
	if err != nil {
		return nil, nil, fmt.Errorf("clear generated WidgetTree RootWidget: %w", err)
	}
	workingBytes, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, ctx.GeneratedTreeExport, "AllWidgets", "ArrayProperty(ObjectProperty)", allWidgets, "")
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite generated WidgetTree AllWidgets: %w", err)
	}
	return workingBytes, workingAsset, nil
}

func rewriteWidgetAddGeneratedRootlessCompanionExport(asset *uasset.Asset, opts uasset.ParseOptions, exportIndex int, generatedTreeExport int, designerExport int) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if exportIndex < 0 || exportIndex >= len(asset.Exports) || designerExport < 0 || designerExport >= len(asset.Exports) {
		return nil, nil, fmt.Errorf("export index out of range")
	}
	className := asset.ResolveClassName(asset.Exports[designerExport])
	objectName := asset.Exports[designerExport].ObjectName.Display(asset.Names)
	classImportIdx, found := findUMGClassImport(asset, className)
	if !found {
		return nil, nil, fmt.Errorf("%s class import not found", className)
	}
	objectRef, err := resolveDisplayNameRef(asset.Names, objectName)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve generated companion object name %q: %w", objectName, err)
	}
	_, workingAsset, err := rewriteWidgetAddExportHeader(asset, exportIndex, uasset.PackageIndex(-classImportIdx), uasset.PackageIndex(generatedTreeExport+1), objectRef)
	if err != nil {
		return nil, nil, err
	}
	for _, propertyName := range []string{"Parent", "Content", "Slots", "Slot", "DisplayLabel", "bExpandedInDesigner"} {
		_, workingAsset, err = applyWidgetAddPropertyRemove(workingAsset, exportIndex, propertyName)
		if err != nil {
			return nil, nil, fmt.Errorf("remove %s from generated companion: %w", propertyName, err)
		}
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, exportIndex, "Slot", "ObjectProperty", map[string]any{"index": int32(0)}, "")
	if err != nil {
		return nil, nil, fmt.Errorf("write generated companion Slot: %w", err)
	}
	if label, ok := decodeExportRootPropertyValue(asset, designerExport, "DisplayLabel"); ok == nil {
		if text, ok := label.(string); ok && strings.TrimSpace(text) != "" {
			_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, exportIndex, "DisplayLabel", "StrProperty", text, "")
			if err != nil {
				return nil, nil, fmt.Errorf("write generated companion DisplayLabel: %w", err)
			}
		}
	}
	return append([]byte(nil), workingAsset.Raw.Bytes...), workingAsset, nil
}

func rewriteWidgetAddGeneratedSlotPlaceholder(asset *uasset.Asset, opts uasset.ParseOptions, exportIndex int, generatedParentExport int, objectName string, slotClass string) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	slotImportIdx, found := findUMGClassImport(asset, slotClass)
	if !found {
		return nil, nil, fmt.Errorf("%s class import not found", slotClass)
	}
	objectRef, err := resolveDisplayNameRef(asset.Names, objectName)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve generated slot placeholder object name %q: %w", objectName, err)
	}
	_, workingAsset, err := rewriteWidgetAddExportHeader(asset, exportIndex, uasset.PackageIndex(-slotImportIdx), uasset.PackageIndex(generatedParentExport+1), objectRef)
	if err != nil {
		return nil, nil, err
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, exportIndex, "Parent", "ObjectProperty", map[string]any{"index": int32(0)}, "")
	if err != nil {
		return nil, nil, fmt.Errorf("clear generated slot placeholder Parent: %w", err)
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, exportIndex, "Content", "ObjectProperty", map[string]any{"index": int32(0)}, "")
	if err != nil {
		return nil, nil, fmt.Errorf("clear generated slot placeholder Content: %w", err)
	}
	return append([]byte(nil), workingAsset.Raw.Bytes...), workingAsset, nil
}

func widgetExportIndexFromDecodedMust(asset *uasset.Asset, exportIndex int, propertyName string) int {
	decoded, err := decodeExportRootPropertyValue(asset, exportIndex, propertyName)
	if err != nil {
		return 0
	}
	return widgetExportIndexFromDecoded(decoded)
}

func widgetAddTopLevelChildrenFromSlots(asset *uasset.Asset, parentExport int) ([]int, error) {
	slotExports, err := widgetAddSlotExportsFromSlots(asset, parentExport)
	if err != nil {
		return nil, err
	}
	children := make([]int, 0, len(slotExports))
	for _, slotExport := range slotExports {
		contentExport := widgetExportIndexFromDecodedMust(asset, slotExport, "Content") - 1
		if contentExport < 0 {
			return nil, fmt.Errorf("slot export %d missing Content export", slotExport+1)
		}
		children = append(children, contentExport)
	}
	return children, nil
}

func rewriteWidgetAddAllWidgetsPreorder(asset *uasset.Asset, ctx widgetAddContext) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if ctx.Target.BlueprintExport < 0 || ctx.DesignerTreeExport < 0 || ctx.GeneratedTreeExport < 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	targets, err := widgetAddPreorderTargets(asset, ctx)
	if err != nil {
		return nil, nil, err
	}
	designerAllWidgets, generatedAllWidgets, err := widgetAddBuildPreorderAllWidgets(asset, targets)
	if err != nil {
		return nil, nil, err
	}

	_, workingAsset, err := applyWidgetAddPropertyWrite(asset, ctx.DesignerTreeExport, "AllWidgets", "ArrayProperty(ObjectProperty)", designerAllWidgets, "")
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite designer tree AllWidgets preorder: %w", err)
	}
	workingBytes, workingAsset, err := applyWidgetAddPropertyWrite(workingAsset, ctx.GeneratedTreeExport, "AllWidgets", "ArrayProperty(ObjectProperty)", generatedAllWidgets, "")
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite generated tree AllWidgets preorder: %w", err)
	}
	return workingBytes, workingAsset, nil
}

func rewriteWidgetAddPreorderMetadata(asset *uasset.Asset, ctx widgetAddContext) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	targets, err := widgetAddPreorderTargets(asset, ctx)
	if err != nil {
		return nil, nil, err
	}
	preorderNames := make([]string, 0, len(targets))
	for _, target := range targets {
		preorderNames = append(preorderNames, target.ObjectName)
	}

	_, workingAsset, err := rewriteWidgetAddAllWidgetsPreorder(asset, ctx)
	if err != nil {
		return nil, nil, err
	}
	_, workingAsset, err = rewriteWidgetAddWidgetVariableGuidMapPreorder(workingAsset, ctx.Target.BlueprintExport, preorderNames)
	if err != nil {
		return nil, nil, err
	}
	_, workingAsset, err = rewriteWidgetAddGeneratedVariablesPreorder(workingAsset, ctx.Target.BlueprintExport, preorderNames)
	if err != nil {
		return nil, nil, err
	}
	_, workingAsset, err = rewriteGeneratedClassPropertyGuidsPreorder(workingAsset, ctx.GeneratedClassExport, preorderNames)
	if err != nil {
		return nil, nil, err
	}
	workingBytes, workingAsset, err := rewriteGeneratedClassWidgetVariableFieldRecordsPreorder(workingAsset, ctx.GeneratedClassExport, preorderNames)
	if err != nil {
		return nil, nil, err
	}
	return workingBytes, workingAsset, nil
}

func widgetAddParentPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	sep := strings.LastIndex(path, "/")
	if sep <= 0 {
		return ""
	}
	return path[:sep]
}

func widgetAddPreorderTargets(asset *uasset.Asset, ctx widgetAddContext) ([]widgetWriteTarget, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	targets, err := collectWidgetWriteTargets(asset, ctx.Target.BlueprintExport+1)
	if err != nil {
		return nil, err
	}
	rootDesignerExport := widgetExportIndexFromDecodedMust(asset, ctx.DesignerTreeExport, "RootWidget") - 1
	if rootDesignerExport < 0 {
		return nil, fmt.Errorf("designer root export not found")
	}
	rootTargetIndex := -1
	for i, target := range targets {
		designerExport, _, resolveErr := widgetAddSortedWidgetAndSlotExports(target)
		if resolveErr != nil {
			continue
		}
		if designerExport == rootDesignerExport {
			rootTargetIndex = i
			break
		}
	}
	if rootTargetIndex < 0 {
		return nil, fmt.Errorf("root widget target not found")
	}
	rootTarget := targets[rootTargetIndex]
	pathToTarget := make(map[string]widgetWriteTarget, len(targets))
	childrenByParent := make(map[string][]widgetWriteTarget, len(targets))
	for _, target := range targets {
		pathToTarget[target.Path] = target
		parentPath := widgetAddParentPath(target.Path)
		if parentPath == "" {
			continue
		}
		childrenByParent[parentPath] = append(childrenByParent[parentPath], target)
	}
	for parentPath, children := range childrenByParent {
		parentTarget, ok := pathToTarget[parentPath]
		if !ok {
			continue
		}
		designerParentExport, _, resolveErr := widgetAddSortedWidgetAndSlotExports(parentTarget)
		if resolveErr != nil {
			continue
		}
		sortWidgetTargetsByDesignerSlotOrder(asset, designerParentExport, children)
		childrenByParent[parentPath] = children
	}
	preorder := make([]widgetWriteTarget, 0, len(targets))
	var visit func(widgetWriteTarget) error
	visit = func(target widgetWriteTarget) error {
		preorder = append(preorder, target)
		for _, child := range childrenByParent[target.Path] {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(rootTarget); err != nil {
		return nil, err
	}
	return preorder, nil
}

func widgetAddBuildPreorderAllWidgets(asset *uasset.Asset, targets []widgetWriteTarget) ([]any, []any, error) {
	designerAllWidgets := make([]any, 0, len(targets))
	generatedAllWidgets := make([]any, 0, len(targets))
	for _, target := range targets {
		designerExport, generatedExport, err := widgetAddSortedWidgetAndSlotExports(target)
		if err != nil {
			return nil, nil, err
		}
		designerAllWidgets = append(designerAllWidgets, objectRefValue(designerExport))
		if generatedExport >= 0 {
			generatedAllWidgets = append(generatedAllWidgets, objectRefValue(generatedExport))
		} else {
			generatedAllWidgets = append(generatedAllWidgets, map[string]any{"index": int32(0)})
		}
	}
	return designerAllWidgets, generatedAllWidgets, nil
}

func reorderNamedEntriesPreorder(items []any, desiredNames []string, nameFn func(any) (string, bool)) []any {
	if len(items) == 0 {
		return items
	}
	byName := make(map[string][]any, len(items))
	leftovers := make([]any, 0, len(items))
	for _, item := range items {
		name, ok := nameFn(item)
		if !ok || strings.TrimSpace(name) == "" {
			leftovers = append(leftovers, item)
			continue
		}
		byName[name] = append(byName[name], item)
	}
	out := make([]any, 0, len(items))
	for _, name := range desiredNames {
		queue := byName[name]
		if len(queue) == 0 {
			continue
		}
		out = append(out, queue[0])
		if len(queue) == 1 {
			delete(byName, name)
		} else {
			byName[name] = queue[1:]
		}
	}
	for _, item := range items {
		name, ok := nameFn(item)
		if !ok || strings.TrimSpace(name) == "" {
			continue
		}
		queue := byName[name]
		if len(queue) == 0 {
			continue
		}
		out = append(out, queue[0])
		if len(queue) == 1 {
			delete(byName, name)
		} else {
			byName[name] = queue[1:]
		}
	}
	return append(out, leftovers...)
}

func rewriteWidgetAddWidgetVariableGuidMapPreorder(asset *uasset.Asset, blueprintExport int, desiredNames []string) ([]byte, *uasset.Asset, error) {
	current, err := decodeExportRootPropertyValue(asset, blueprintExport, "WidgetVariableNameToGuidMap")
	if err != nil {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	m, ok := current.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("WidgetVariableNameToGuidMap is not decodable as a map")
	}
	items, err := anySliceLocal(m["value"])
	if err != nil {
		return nil, nil, fmt.Errorf("WidgetVariableNameToGuidMap value is not an entry list")
	}
	reordered := reorderNamedEntriesPreorder(items, desiredNames, func(item any) (string, bool) {
		entry, ok := item.(map[string]any)
		if !ok {
			return "", false
		}
		key, ok := entry["key"].(map[string]any)
		if !ok {
			return "", false
		}
		value, ok := key["value"].(map[string]any)
		if !ok {
			return "", false
		}
		name, _ := value["name"].(string)
		return name, name != ""
	})
	filtered := make([]any, 0, len(reordered))
	for _, item := range reordered {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("WidgetVariableNameToGuidMap entry is not an object")
		}
		entryCopy := cloneAnyMapLocal(entry)
		valueWrapper, ok := entryCopy["value"].(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("WidgetVariableNameToGuidMap entry value is not an object")
		}
		valueCopy := cloneAnyMapLocal(valueWrapper)
		if guidRaw, ok := valueCopy["value"].(string); ok && strings.TrimSpace(guidRaw) != "" {
			valueCopy["value"] = map[string]any{
				"structType": "Guid",
				"value":      guidRaw,
			}
		}
		entryCopy["value"] = valueCopy
		filtered = append(filtered, entryCopy)
	}
	out := cloneAnyMapLocal(m)
	out["value"] = filtered
	return applyWidgetAddPropertyWrite(asset, blueprintExport, "WidgetVariableNameToGuidMap", "MapProperty(NameProperty,StructProperty(Guid(/Script/CoreUObject)))", out, "WidgetTree")
}

func rewriteWidgetAddGeneratedVariablesPreorder(asset *uasset.Asset, blueprintExport int, desiredNames []string) ([]byte, *uasset.Asset, error) {
	current, err := decodeExportRootPropertyValue(asset, blueprintExport, "GeneratedVariables")
	if err != nil {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	m, ok := current.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("GeneratedVariables is not a decoded array property")
	}
	items, err := anySliceLocal(m["value"])
	if err != nil {
		return nil, nil, fmt.Errorf("GeneratedVariables value is not an entry list")
	}
	reordered := reorderNamedEntriesPreorder(items, desiredNames, func(item any) (string, bool) {
		entry, ok := item.(map[string]any)
		if !ok {
			return "", false
		}
		value, ok := entry["value"].(map[string]any)
		if !ok {
			return "", false
		}
		fields, ok := value["value"].(map[string]any)
		if !ok {
			return "", false
		}
		varName, ok := fields["VarName"].(map[string]any)
		if !ok {
			return "", false
		}
		nameValue, ok := varName["value"].(map[string]any)
		if !ok {
			return "", false
		}
		name, _ := nameValue["name"].(string)
		return name, name != ""
	})
	filtered := make([]any, 0, len(reordered))
	for _, item := range reordered {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("GeneratedVariables entry is not an object")
		}
		if inner, ok := entry["value"].(map[string]any); ok {
			filtered = append(filtered, cloneAnyMapLocal(inner))
			continue
		}
		filtered = append(filtered, cloneAnyMapLocal(entry))
	}
	return applyWidgetAddPropertyWrite(asset, blueprintExport, "GeneratedVariables", "ArrayProperty(StructProperty(BPVariableDescription(/Script/Engine)))", filtered, "CategorySorting")
}

func rewriteGeneratedClassPropertyGuidsPreorder(asset *uasset.Asset, generatedClassExport int, desiredNames []string) ([]byte, *uasset.Asset, error) {
	current, err := decodeExportRootPropertyValue(asset, generatedClassExport, "PropertyGuids")
	if err != nil {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	m, ok := current.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("PropertyGuids is not a decoded map property")
	}
	items, err := anySliceLocal(m["value"])
	if err != nil {
		return nil, nil, fmt.Errorf("PropertyGuids value is not an entry list")
	}
	reordered := reorderNamedEntriesPreorder(items, desiredNames, func(item any) (string, bool) {
		entry, ok := item.(map[string]any)
		if !ok {
			return "", false
		}
		key, ok := entry["key"].(map[string]any)
		if !ok {
			return "", false
		}
		value, ok := key["value"].(map[string]any)
		if !ok {
			return "", false
		}
		name, _ := value["name"].(string)
		return name, name != ""
	})
	filtered := make([]any, 0, len(reordered))
	for _, item := range reordered {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("PropertyGuids entry is not an object")
		}
		entryCopy := cloneAnyMapLocal(entry)
		valueWrapper, ok := entryCopy["value"].(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("PropertyGuids entry value is not an object")
		}
		valueCopy := cloneAnyMapLocal(valueWrapper)
		if guidRaw, ok := valueCopy["value"].(string); ok && strings.TrimSpace(guidRaw) != "" {
			valueCopy["value"] = map[string]any{
				"structType": "Guid",
				"value":      guidRaw,
			}
		}
		entryCopy["value"] = valueCopy
		filtered = append(filtered, entryCopy)
	}
	out := cloneAnyMapLocal(m)
	out["value"] = filtered
	return applyWidgetAddPropertyWrite(asset, generatedClassExport, "PropertyGuids", "MapProperty(NameProperty,StructProperty(Guid(/Script/CoreUObject)))", out, "")
}

type generatedClassFieldRecord struct {
	Name string
	Raw  []byte
}

func rewriteGeneratedClassWidgetVariableFieldRecordsPreorder(asset *uasset.Asset, generatedClassExport int, desiredNames []string) ([]byte, *uasset.Asset, error) {
	layout, records, suffix, err := captureGeneratedClassWidgetVariableFieldRecords(asset, generatedClassExport)
	if err != nil {
		return nil, nil, err
	}
	if len(records) == 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	byName := make(map[string][]generatedClassFieldRecord, len(records))
	for _, record := range records {
		byName[record.Name] = append(byName[record.Name], record)
	}
	ordered := make([]generatedClassFieldRecord, 0, len(records))
	for _, name := range desiredNames {
		queue := byName[name]
		if len(queue) == 0 {
			continue
		}
		ordered = append(ordered, queue[0])
		if len(queue) == 1 {
			delete(byName, name)
		} else {
			byName[name] = queue[1:]
		}
	}
	for _, record := range records {
		queue := byName[record.Name]
		if len(queue) == 0 {
			continue
		}
		ordered = append(ordered, queue[0])
		if len(queue) == 1 {
			delete(byName, record.Name)
		} else {
			byName[record.Name] = queue[1:]
		}
	}

	exp := asset.Exports[generatedClassExport]
	serialStart := int(exp.SerialOffset)
	serialEnd := int(exp.SerialOffset + exp.SerialSize)
	payload := append([]byte(nil), asset.Raw.Bytes[serialStart:serialEnd]...)
	order := packageByteOrder(asset)
	tail := make([]byte, 0, 16+len(payload))
	tail = append(tail, payload[layout.ScriptEnd:layout.ScriptEnd+12]...)
	tail = appendInt32Ordered(tail, int32(len(ordered)), order)
	for _, record := range ordered {
		tail = append(tail, record.Raw...)
	}
	tail = append(tail, suffix...)
	newPayload := append(append([]byte(nil), payload[:layout.ScriptEnd]...), tail...)
	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{{
		ExportIndex: generatedClassExport,
		Payload:     newPayload,
	}})
	if err != nil {
		return nil, nil, fmt.Errorf("replace generated class field record order: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(outBytes, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, nil, fmt.Errorf("reparse generated class field record reorder: %w", err)
	}
	return outBytes, updatedAsset, nil
}

func captureGeneratedClassWidgetVariableFieldRecords(asset *uasset.Asset, generatedClassExport int) (generatedClassWidgetVariableFieldLayout, []generatedClassFieldRecord, []byte, error) {
	layout, err := captureGeneratedClassWidgetVariableFieldLayout(asset, generatedClassExport)
	if err != nil {
		return layout, nil, nil, err
	}
	exp := asset.Exports[generatedClassExport]
	serialStart := int(exp.SerialOffset)
	serialEnd := int(exp.SerialOffset + exp.SerialSize)
	payload := asset.Raw.Bytes[serialStart:serialEnd]
	tail := append([]byte(nil), payload[layout.ScriptEnd:]...)
	order := packageByteOrder(asset)
	records := make([]generatedClassFieldRecord, 0, layout.FieldCount)
	offset := 16
	for i := 0; i < layout.FieldCount; i++ {
		recordLen, parseErr := generatedClassWidgetVariableFieldRecordLength(tail[offset:], order)
		if parseErr != nil {
			return layout, nil, nil, fmt.Errorf("parse generated class field record %d: %w", i, parseErr)
		}
		nameRef := uasset.NameRef{
			Index:  int32(order.Uint32(tail[offset+8 : offset+12])),
			Number: int32(order.Uint32(tail[offset+12 : offset+16])),
		}
		records = append(records, generatedClassFieldRecord{
			Name: nameRef.Display(asset.Names),
			Raw:  append([]byte(nil), tail[offset:offset+recordLen]...),
		})
		offset += recordLen
	}
	return layout, records, append([]byte(nil), tail[layout.RecordsEnd:]...), nil
}

func widgetAddSlotExportsFromSlots(asset *uasset.Asset, parentExport int) ([]int, error) {
	items, err := readWidgetAddObjectRefArrayValue(asset, parentExport, "Slots")
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(items))
	for _, item := range items {
		exportIndex, ok := widgetAddDecodedObjectRefExportIndex(item)
		if !ok || exportIndex < 0 {
			return nil, fmt.Errorf("parent export %d contains invalid slot reference", parentExport+1)
		}
		out = append(out, exportIndex)
	}
	return out, nil
}

func findWidgetAddReusableGeneratedSlotExport(asset *uasset.Asset, ctx widgetAddContext) (int, string, error) {
	if asset == nil || ctx.GeneratedParentExport < 0 {
		return -1, "", nil
	}
	slotClass, err := widgetAddSlotClassName(ctx.PanelClass)
	if err != nil {
		return -1, "", err
	}
	for i, exp := range asset.Exports {
		if !strings.EqualFold(asset.ResolveClassName(exp), slotClass) {
			continue
		}
		if int(exp.OuterIndex) != ctx.GeneratedParentExport+1 {
			continue
		}
		if containsIntLocal(ctx.Target.SlotExports, i) {
			continue
		}
		if !widgetAddGeneratedSlotIsReusablePlaceholder(asset, i) {
			continue
		}
		return i, exp.ObjectName.Display(asset.Names), nil
	}
	return -1, "", nil
}

func widgetAddGeneratedSlotIsReusablePlaceholder(asset *uasset.Asset, exportIndex int) bool {
	if asset == nil || exportIndex < 0 || exportIndex >= len(asset.Exports) {
		return false
	}
	parent, err := decodeExportRootPropertyValue(asset, exportIndex, "Parent")
	if err != nil || !widgetAddDecodedObjectRefIsNull(parent) {
		return false
	}
	content, err := decodeExportRootPropertyValue(asset, exportIndex, "Content")
	if err != nil || !widgetAddDecodedObjectRefIsNull(content) {
		return false
	}
	return true
}

func containsIntLocal(items []int, want int) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func rewriteWidgetAddOverlayRootChainCategorySorting(asset *uasset.Asset, blueprintExport int, blueprintObjectName string) ([]byte, *uasset.Asset, error) {
	categoryName := widgetAddBlueprintCategoryName(blueprintObjectName)
	if categoryName == "" {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	eventGraphRef, err := resolveDisplayNameRef(asset.Names, "Event Graph")
	if err != nil {
		return nil, nil, err
	}
	categoryRef, err := resolveDisplayNameRef(asset.Names, categoryName)
	if err != nil {
		return nil, nil, err
	}
	values := []any{
		map[string]any{"index": eventGraphRef.Index, "name": "Event Graph", "number": eventGraphRef.Number},
		map[string]any{"index": categoryRef.Index, "name": categoryName, "number": categoryRef.Number},
	}
	return applyWidgetAddPropertyWrite(asset, blueprintExport, "CategorySorting", "ArrayProperty(NameProperty)", values, "")
}

func reorderWidgetAddOverlayRootChainExports(asset *uasset.Asset, ctx widgetAddContext) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	targets, err := collectWidgetWriteTargets(asset, ctx.Target.BlueprintExport+1)
	if err != nil {
		return nil, nil, err
	}
	rootTarget, err := selectWidgetWriteTarget(targets, ctx.Target.Path)
	if err != nil {
		return nil, nil, err
	}
	children := make([]widgetWriteTarget, 0, 4)
	for _, target := range targets {
		if !strings.HasPrefix(target.Path, ctx.Target.Path+"/") {
			continue
		}
		children = append(children, target)
	}
	sortWidgetTargetsByDesignerSlotOrder(asset, rootTarget.Exports[0], children)

	eventGraphExport := -1
	k2Exports := make([]int, 0, 4)
	for i, exp := range asset.Exports {
		className := asset.ResolveClassName(exp)
		switch {
		case strings.EqualFold(exp.ObjectName.Display(asset.Names), "EventGraph"):
			eventGraphExport = i
		case strings.EqualFold(className, "K2Node_Event"):
			k2Exports = append(k2Exports, i)
		}
	}
	if eventGraphExport < 0 {
		return nil, nil, fmt.Errorf("EventGraph export not found")
	}
	if len(rootTarget.Exports) < 2 {
		return nil, nil, fmt.Errorf("overlay root target missing designer/generated exports")
	}

	order := make([]int, 0, len(asset.Exports))
	seen := make([]bool, len(asset.Exports))
	appendExport := func(idx int) error {
		if idx < 0 || idx >= len(asset.Exports) {
			return fmt.Errorf("export index out of range: %d", idx+1)
		}
		if seen[idx] {
			return nil
		}
		seen[idx] = true
		order = append(order, idx)
		return nil
	}

	if err := appendExport(eventGraphExport); err != nil {
		return nil, nil, err
	}
	for _, child := range children {
		if len(child.Exports) > 0 {
			if err := appendExport(child.Exports[0]); err != nil {
				return nil, nil, err
			}
		}
	}
	for _, child := range children {
		if len(child.Exports) > 1 {
			if err := appendExport(child.Exports[1]); err != nil {
				return nil, nil, err
			}
		}
	}
	for _, idx := range k2Exports {
		if err := appendExport(idx); err != nil {
			return nil, nil, err
		}
	}
	for _, idx := range []int{rootTarget.Exports[0], rootTarget.Exports[1]} {
		if err := appendExport(idx); err != nil {
			return nil, nil, err
		}
	}
	for _, child := range children {
		if len(child.SlotExports) > 0 {
			if err := appendExport(child.SlotExports[0]); err != nil {
				return nil, nil, err
			}
		}
	}
	for _, child := range children {
		if len(child.SlotExports) > 1 {
			if err := appendExport(child.SlotExports[1]); err != nil {
				return nil, nil, err
			}
		}
	}
	for _, idx := range []int{ctx.CDOExport, ctx.Target.BlueprintExport, ctx.GeneratedClassExport, ctx.DesignerTreeExport, ctx.GeneratedTreeExport} {
		if err := appendExport(idx); err != nil {
			return nil, nil, err
		}
	}
	if len(order) != len(asset.Exports) {
		leftovers := make([]int, 0, len(asset.Exports)-len(order))
		for i := range asset.Exports {
			if !seen[i] {
				leftovers = append(leftovers, i+1)
			}
		}
		return nil, nil, fmt.Errorf("overlay root-chain export reorder left unresolved exports: %v", leftovers)
	}
	outBytes, err := edit.ReorderExports(asset, order)
	if err != nil {
		return nil, nil, fmt.Errorf("order=%v: %w", order, err)
	}
	outAsset, err := uasset.ParseBytes(outBytes, uasset.DefaultParseOptions())
	if err != nil {
		return nil, nil, err
	}
	return outBytes, outAsset, nil
}

func reorderWidgetAddGeneratedRootlessTopLevelNestedExports(asset *uasset.Asset, ctx widgetAddContext) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if ctx.Target.BlueprintExport < 0 {
		return nil, nil, fmt.Errorf("target blueprint export is not resolved")
	}
	pathSegments := splitWidgetPathSegments(ctx.Target.Path)
	if len(pathSegments) != 2 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	targets, err := collectWidgetWriteTargets(asset, ctx.Target.BlueprintExport+1)
	if err != nil {
		return nil, nil, err
	}
	rootTarget, err := selectWidgetWriteTarget(targets, pathSegments[0])
	if err != nil {
		return nil, nil, err
	}
	parentTarget, err := selectWidgetWriteTarget(targets, ctx.Target.Path)
	if err != nil {
		return nil, nil, err
	}
	if len(rootTarget.Exports) == 0 || len(parentTarget.Exports) < 2 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	topLevelChildren := make([]widgetWriteTarget, 0, 8)
	descendants := make([]widgetWriteTarget, 0, 8)
	rootDepth := len(pathSegments)
	for _, target := range targets {
		if target.Path == rootTarget.Path {
			continue
		}
		segments := splitWidgetPathSegments(target.Path)
		switch {
		case len(segments) == rootDepth && strings.HasPrefix(target.Path, rootTarget.Path+"/"):
			topLevelChildren = append(topLevelChildren, target)
		case strings.HasPrefix(target.Path, parentTarget.Path+"/"):
			descendants = append(descendants, target)
		}
	}
	sortWidgetTargetsByDesignerSlotOrder(asset, rootTarget.Exports[0], topLevelChildren)
	sortWidgetTargetsByDesignerSlotOrder(asset, parentTarget.Exports[0], descendants)

	eventGraphExport := -1
	k2Exports := make([]int, 0, 4)
	for i, exp := range asset.Exports {
		className := asset.ResolveClassName(exp)
		switch {
		case strings.EqualFold(exp.ObjectName.Display(asset.Names), "EventGraph"):
			eventGraphExport = i
		case strings.EqualFold(className, "K2Node_Event"):
			k2Exports = append(k2Exports, i)
		}
	}
	if eventGraphExport < 0 {
		return nil, nil, fmt.Errorf("EventGraph export not found")
	}

	order := make([]int, 0, len(asset.Exports))
	seen := make([]bool, len(asset.Exports))
	appendExport := func(idx int) error {
		if idx < 0 || idx >= len(asset.Exports) {
			return fmt.Errorf("export index out of range: %d", idx+1)
		}
		if seen[idx] {
			return nil
		}
		seen[idx] = true
		order = append(order, idx)
		return nil
	}

	if err := appendExport(rootTarget.Exports[0]); err != nil {
		return nil, nil, err
	}
	for _, target := range topLevelChildren {
		if len(target.SlotExports) > 0 {
			if err := appendExport(target.SlotExports[0]); err != nil {
				return nil, nil, err
			}
		}
	}
	if err := appendExport(eventGraphExport); err != nil {
		return nil, nil, err
	}
	for _, target := range topLevelChildren {
		if target.Path == parentTarget.Path {
			continue
		}
		if len(target.Exports) > 0 {
			if err := appendExport(target.Exports[0]); err != nil {
				return nil, nil, err
			}
		}
	}
	for _, target := range descendants {
		if len(target.Exports) > 0 {
			if err := appendExport(target.Exports[0]); err != nil {
				return nil, nil, err
			}
		}
	}
	for _, target := range topLevelChildren {
		if target.Path == parentTarget.Path {
			continue
		}
		if len(target.Exports) > 1 {
			if err := appendExport(target.Exports[1]); err != nil {
				return nil, nil, err
			}
		}
	}
	for _, target := range descendants {
		if len(target.Exports) > 1 {
			if err := appendExport(target.Exports[1]); err != nil {
				return nil, nil, err
			}
		}
	}
	for _, idx := range k2Exports {
		if err := appendExport(idx); err != nil {
			return nil, nil, err
		}
	}
	for _, idx := range parentTarget.Exports {
		if err := appendExport(idx); err != nil {
			return nil, nil, err
		}
	}
	for _, target := range descendants {
		for _, slotExport := range target.SlotExports {
			if err := appendExport(slotExport); err != nil {
				return nil, nil, err
			}
		}
	}
	for _, idx := range []int{ctx.CDOExport, ctx.Target.BlueprintExport, ctx.GeneratedClassExport, ctx.DesignerTreeExport, ctx.GeneratedTreeExport} {
		if err := appendExport(idx); err != nil {
			return nil, nil, err
		}
	}
	if len(order) != len(asset.Exports) {
		leftovers := make([]int, 0, len(asset.Exports)-len(order))
		for i := range asset.Exports {
			if !seen[i] {
				leftovers = append(leftovers, i+1)
			}
		}
		return nil, nil, fmt.Errorf("generated rootless nested export reorder left unresolved exports: %v", leftovers)
	}
	outBytes, err := edit.ReorderExports(asset, order)
	if err != nil {
		return nil, nil, fmt.Errorf("order=%v: %w", order, err)
	}
	outAsset, err := uasset.ParseBytes(outBytes, uasset.DefaultParseOptions())
	if err != nil {
		return nil, nil, err
	}
	return outBytes, outAsset, nil
}

func rewriteWidgetAddGeneratedRootlessTopLevelDependsMap(asset *uasset.Asset, ctx widgetAddContext) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if ctx.Target.BlueprintExport < 0 {
		return nil, nil, fmt.Errorf("target blueprint export is not resolved")
	}
	pathSegments := splitWidgetPathSegments(ctx.Target.Path)
	if len(pathSegments) != 2 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	targets, err := collectWidgetWriteTargets(asset, ctx.Target.BlueprintExport+1)
	if err != nil {
		return nil, nil, err
	}
	rootTarget, err := selectWidgetWriteTarget(targets, pathSegments[0])
	if err != nil {
		return nil, nil, err
	}
	parentTarget, err := selectWidgetWriteTarget(targets, ctx.Target.Path)
	if err != nil {
		return nil, nil, err
	}
	if len(rootTarget.Exports) == 0 {
		return nil, nil, fmt.Errorf("root target %s missing exports", rootTarget.Path)
	}

	topLevelChildren := make([]widgetWriteTarget, 0, 8)
	descendants := make([]widgetWriteTarget, 0, 8)
	rootDepth := len(pathSegments)
	for _, target := range targets {
		if target.Path == rootTarget.Path {
			continue
		}
		segments := splitWidgetPathSegments(target.Path)
		switch {
		case len(segments) == rootDepth && strings.HasPrefix(target.Path, rootTarget.Path+"/"):
			topLevelChildren = append(topLevelChildren, target)
		case strings.HasPrefix(target.Path, parentTarget.Path+"/"):
			descendants = append(descendants, target)
		}
	}
	sortWidgetTargetsByDesignerSlotOrder(asset, rootTarget.Exports[0], topLevelChildren)
	sortWidgetTargetsByDesignerSlotOrder(asset, parentTarget.Exports[0], descendants)

	updates := map[int][]uasset.PackageIndex{}
	rootSlotDeps := make([]uasset.PackageIndex, 0, len(topLevelChildren))
	designerTreeDeps := []uasset.PackageIndex{uasset.PackageIndex(-23), uasset.PackageIndex(rootTarget.Exports[0] + 1)}
	for _, target := range topLevelChildren {
		if len(target.SlotExports) > 0 {
			rootSlotDeps = append(rootSlotDeps, uasset.PackageIndex(target.SlotExports[0]+1))
			updates[target.SlotExports[0]] = []uasset.PackageIndex{
				uasset.PackageIndex(rootTarget.Exports[0] + 1),
				uasset.PackageIndex(target.Exports[0] + 1),
			}
		}
		if target.Path == parentTarget.Path {
			designerTreeDeps = append(designerTreeDeps, uasset.PackageIndex(target.Exports[0]+1))
			continue
		}
		updates[target.Exports[0]] = []uasset.PackageIndex{uasset.PackageIndex(target.SlotExports[0] + 1)}
		if len(target.Exports) > 1 {
			updates[target.Exports[1]] = nil
		}
		designerTreeDeps = append(designerTreeDeps, uasset.PackageIndex(target.Exports[0]+1))
	}
	updates[rootTarget.Exports[0]] = rootSlotDeps

	designerNestedSlotDeps := make([]uasset.PackageIndex, 0, len(descendants))
	generatedNestedSlotDeps := make([]uasset.PackageIndex, 0, len(descendants))
	for _, target := range descendants {
		if len(target.SlotExports) > 0 {
			designerNestedSlotDeps = append(designerNestedSlotDeps, uasset.PackageIndex(target.SlotExports[0]+1))
			updates[target.Exports[0]] = []uasset.PackageIndex{uasset.PackageIndex(target.SlotExports[0] + 1)}
			updates[target.SlotExports[0]] = []uasset.PackageIndex{
				uasset.PackageIndex(parentTarget.Exports[0] + 1),
				uasset.PackageIndex(target.Exports[0] + 1),
			}
			designerTreeDeps = append(designerTreeDeps, uasset.PackageIndex(target.Exports[0]+1))
		}
		if len(target.Exports) > 1 {
			if len(target.SlotExports) > 1 {
				updates[target.Exports[1]] = []uasset.PackageIndex{uasset.PackageIndex(target.SlotExports[1] + 1)}
				updates[target.SlotExports[1]] = []uasset.PackageIndex{
					uasset.PackageIndex(parentTarget.Exports[1] + 1),
					uasset.PackageIndex(target.Exports[1] + 1),
				}
				generatedNestedSlotDeps = append(generatedNestedSlotDeps, uasset.PackageIndex(target.SlotExports[1]+1))
			} else {
				updates[target.Exports[1]] = nil
			}
		}
	}
	if len(parentTarget.Exports) > 0 && len(parentTarget.SlotExports) > 0 {
		updates[parentTarget.Exports[0]] = append(append([]uasset.PackageIndex(nil), designerNestedSlotDeps...), uasset.PackageIndex(parentTarget.SlotExports[0]+1))
	}
	if len(parentTarget.Exports) > 1 {
		updates[parentTarget.Exports[1]] = append([]uasset.PackageIndex(nil), generatedNestedSlotDeps...)
	}
	if ctx.DesignerTreeExport >= 0 {
		updates[ctx.DesignerTreeExport] = designerTreeDeps
	}
	if ctx.GeneratedTreeExport >= 0 {
		updates[ctx.GeneratedTreeExport] = nil
	}

	var workingBytes []byte
	workingAsset := asset
	for exportIndex, deps := range updates {
		workingBytes, err = edit.ReplaceExportDependencies(workingAsset, exportIndex, deps)
		if err != nil {
			return nil, nil, fmt.Errorf("replace dependencies for export %d: %w", exportIndex+1, err)
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, uasset.ParseOptions{KeepUnknown: true})
		if err != nil {
			return nil, nil, fmt.Errorf("reparse generated rootless nested depends rewrite: %w", err)
		}
	}
	return workingBytes, workingAsset, nil
}

func rewriteWidgetAddGeneratedRootlessTopLevelImportedClassesTrailer(asset *uasset.Asset, ctx widgetAddContext) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if ctx.Target.BlueprintExport < 0 {
		return nil, nil, fmt.Errorf("target blueprint export is not resolved")
	}
	pathSegments := splitWidgetPathSegments(ctx.Target.Path)
	if len(pathSegments) != 2 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	targets, err := collectWidgetWriteTargets(asset, ctx.Target.BlueprintExport+1)
	if err != nil {
		return nil, nil, err
	}
	rootTarget, err := selectWidgetWriteTarget(targets, pathSegments[0])
	if err != nil {
		return nil, nil, err
	}
	rootSlotClass, err := widgetAddSlotClassName(rootTarget.ClassName)
	if err != nil {
		return nil, nil, err
	}
	targetSlotClass, err := widgetAddSlotClassName(ctx.PanelClass)
	if err != nil {
		return nil, nil, err
	}

	clearImports := make([]int, 0, 3)
	for _, className := range []string{rootTarget.ClassName, rootSlotClass, targetSlotClass} {
		if importIdx, found := findUMGClassImport(asset, className); found {
			clearImports = append(clearImports, importIdx)
		}
	}
	if len(clearImports) == 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	return rewriteWidgetAddAssetRegistryImportedClassBits(asset, nil, clearImports)
}

func rewriteWidgetAddGeneratedRootlessTopLevelNestedReferences(asset *uasset.Asset, ctx widgetAddContext) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if ctx.Target.BlueprintExport < 0 {
		return nil, nil, fmt.Errorf("target blueprint export is not resolved")
	}
	pathSegments := splitWidgetPathSegments(ctx.Target.Path)
	if len(pathSegments) != 2 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	rootPath := pathSegments[0]
	targets, err := collectWidgetWriteTargets(asset, ctx.Target.BlueprintExport+1)
	if err != nil {
		return nil, nil, err
	}
	rootTarget, err := selectWidgetWriteTarget(targets, rootPath)
	if err != nil {
		return nil, nil, err
	}
	parentTarget, err := selectWidgetWriteTarget(targets, ctx.Target.Path)
	if err != nil {
		return nil, nil, err
	}
	rootDesignerExport, _, err := widgetAddSortedWidgetAndSlotExports(*rootTarget)
	if err != nil {
		return nil, nil, err
	}
	parentDesignerExport, parentGeneratedExport, err := widgetAddSortedWidgetAndSlotExports(*parentTarget)
	if err != nil {
		return nil, nil, err
	}

	rootDepth := len(pathSegments)
	topLevelTargets := make([]widgetWriteTarget, 0, 8)
	descendantTargets := make([]widgetWriteTarget, 0, 8)
	for _, target := range targets {
		if target.Path == rootPath {
			continue
		}
		segments := splitWidgetPathSegments(target.Path)
		switch {
		case len(segments) == rootDepth && strings.HasPrefix(target.Path, rootPath+"/"):
			topLevelTargets = append(topLevelTargets, target)
		case strings.HasPrefix(target.Path, ctx.Target.Path+"/"):
			descendantTargets = append(descendantTargets, target)
		}
	}
	sortWidgetTargetsByDesignerSlotOrder(asset, rootDesignerExport, topLevelTargets)
	sortWidgetTargetsByDesignerSlotOrder(asset, parentDesignerExport, descendantTargets)

	var workingBytes []byte
	workingAsset := asset

	designerNestedSlots := make([]any, 0, len(descendantTargets))
	generatedNestedSlots := make([]any, 0, len(descendantTargets))
	designerAllWidgets := make([]any, 0, len(topLevelTargets)+len(descendantTargets)+1)
	designerAllWidgets = append(designerAllWidgets, objectRefValue(rootDesignerExport))
	generatedAllWidgets := make([]any, 0, len(topLevelTargets)+len(descendantTargets)+1)
	generatedAllWidgets = append(generatedAllWidgets, map[string]any{"index": int32(0)})

	rootSlotRefs := make([]any, 0, len(topLevelTargets))
	for _, target := range topLevelTargets {
		designerExport, generatedExport, resolveErr := widgetAddSortedWidgetAndSlotExports(target)
		if resolveErr != nil {
			return nil, nil, resolveErr
		}
		designerSlotExport, generatedSlotExport, resolveErr := widgetAddSortedSlotExports(target)
		if resolveErr != nil {
			return nil, nil, resolveErr
		}
		rootSlotRefs = append(rootSlotRefs, objectRefValue(designerSlotExport))
		designerAllWidgets = append(designerAllWidgets, objectRefValue(designerExport))
		if generatedExport >= 0 {
			generatedAllWidgets = append(generatedAllWidgets, objectRefValue(generatedExport))
		}

		_, workingAsset, err = rewriteWidgetAddExportOuterIndex(workingAsset, designerExport, uasset.PackageIndex(ctx.DesignerTreeExport+1))
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite designer root child outer for %s: %w", target.Path, err)
		}
		if generatedExport >= 0 {
			_, workingAsset, err = rewriteWidgetAddExportOuterIndex(workingAsset, generatedExport, uasset.PackageIndex(ctx.GeneratedTreeExport+1))
			if err != nil {
				return nil, nil, fmt.Errorf("rewrite generated root child outer for %s: %w", target.Path, err)
			}
		}

		_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, designerSlotExport, "Parent", "ObjectProperty", objectRefValue(rootDesignerExport), "")
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite root child slot Parent for %s: %w", target.Path, err)
		}
		_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, designerSlotExport, "Content", "ObjectProperty", objectRefValue(designerExport), "")
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite root child slot Content for %s: %w", target.Path, err)
		}
		_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, designerExport, "Slot", "ObjectProperty", objectRefValue(designerSlotExport), "")
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite designer root child Slot for %s: %w", target.Path, err)
		}
		if target.Path == ctx.Target.Path {
			_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, generatedExport, "Slot", "ObjectProperty", map[string]any{"index": int32(0)}, "")
			if err != nil {
				return nil, nil, fmt.Errorf("rewrite generated parent Slot for %s: %w", target.Path, err)
			}
			continue
		}
		if generatedExport >= 0 {
			_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, generatedExport, "Slot", "ObjectProperty", map[string]any{"index": int32(0)}, "")
			if err != nil {
				return nil, nil, fmt.Errorf("rewrite generated root child Slot for %s: %w", target.Path, err)
			}
		}
		if generatedSlotExport >= 0 {
			_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, generatedSlotExport, "Parent", "ObjectProperty", map[string]any{"index": int32(0)}, "")
			if err != nil {
				return nil, nil, fmt.Errorf("clear generated root child slot Parent for %s: %w", target.Path, err)
			}
			_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, generatedSlotExport, "Content", "ObjectProperty", map[string]any{"index": int32(0)}, "")
			if err != nil {
				return nil, nil, fmt.Errorf("clear generated root child slot Content for %s: %w", target.Path, err)
			}
		}
	}
	for _, target := range descendantTargets {
		designerExport, generatedExport, resolveErr := widgetAddSortedWidgetAndSlotExports(target)
		if resolveErr != nil {
			return nil, nil, resolveErr
		}
		designerSlotExport, generatedSlotExport, resolveErr := widgetAddSortedSlotExports(target)
		if resolveErr != nil {
			return nil, nil, resolveErr
		}
		designerNestedSlots = append(designerNestedSlots, objectRefValue(designerSlotExport))
		generatedNestedSlots = append(generatedNestedSlots, objectRefValue(generatedSlotExport))
		designerAllWidgets = append(designerAllWidgets, objectRefValue(designerExport))
		generatedAllWidgets = append(generatedAllWidgets, objectRefValue(generatedExport))

		_, workingAsset, err = rewriteWidgetAddExportOuterIndex(workingAsset, designerExport, uasset.PackageIndex(ctx.DesignerTreeExport+1))
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite designer child outer for %s: %w", target.Path, err)
		}
		_, workingAsset, err = rewriteWidgetAddExportOuterIndex(workingAsset, generatedExport, uasset.PackageIndex(ctx.GeneratedTreeExport+1))
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite generated child outer for %s: %w", target.Path, err)
		}

		_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, designerExport, "Slot", "ObjectProperty", objectRefValue(designerSlotExport), "")
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite designer child Slot for %s: %w", target.Path, err)
		}
		_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, generatedExport, "Slot", "ObjectProperty", objectRefValue(generatedSlotExport), "")
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite generated child Slot for %s: %w", target.Path, err)
		}
		_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, designerSlotExport, "Parent", "ObjectProperty", objectRefValue(parentDesignerExport), "")
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite designer child slot Parent for %s: %w", target.Path, err)
		}
		_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, designerSlotExport, "Content", "ObjectProperty", objectRefValue(designerExport), "")
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite designer child slot Content for %s: %w", target.Path, err)
		}
		_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, generatedSlotExport, "Parent", "ObjectProperty", objectRefValue(parentGeneratedExport), "")
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite generated child slot Parent for %s: %w", target.Path, err)
		}
		_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, generatedSlotExport, "Content", "ObjectProperty", objectRefValue(generatedExport), "")
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite generated child slot Content for %s: %w", target.Path, err)
		}
	}

	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, rootDesignerExport, "Slots", "ArrayProperty(ObjectProperty)", rootSlotRefs, widgetAddParentSlotsBeforeProperty(workingAsset, rootDesignerExport))
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite root Slots: %w", err)
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, parentDesignerExport, "Slots", "ArrayProperty(ObjectProperty)", designerNestedSlots, widgetAddParentSlotsBeforeProperty(workingAsset, parentDesignerExport))
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite designer parent Slots: %w", err)
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, parentGeneratedExport, "Slots", "ArrayProperty(ObjectProperty)", generatedNestedSlots, widgetAddParentSlotsBeforeProperty(workingAsset, parentGeneratedExport))
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite generated parent Slots: %w", err)
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, ctx.DesignerTreeExport, "RootWidget", "ObjectProperty", objectRefValue(rootDesignerExport), "")
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite designer tree RootWidget: %w", err)
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, ctx.DesignerTreeExport, "AllWidgets", "ArrayProperty(ObjectProperty)", designerAllWidgets, "")
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite designer tree AllWidgets: %w", err)
	}
	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, ctx.GeneratedTreeExport, "RootWidget", "ObjectProperty", map[string]any{"index": int32(0)}, "")
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite generated tree RootWidget: %w", err)
	}
	workingBytes, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, ctx.GeneratedTreeExport, "AllWidgets", "ArrayProperty(ObjectProperty)", generatedAllWidgets, "")
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite generated tree AllWidgets: %w", err)
	}
	return workingBytes, workingAsset, nil
}

func rewriteWidgetAddAssetRegistryImportedClassBits(asset *uasset.Asset, setImports []int, clearImports []int) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	sectionBytes, sectionStart, _, present := sectionByOffset(asset, int64(asset.Summary.AssetRegistryDataOffset))
	if !present {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	trailerStart, importCount, wordCount, err := locateWidgetAddAssetRegistryImportedClassesTrailer(sectionBytes, packageByteOrder(asset), len(asset.Imports))
	if err != nil {
		return nil, nil, err
	}
	if trailerStart < 0 || wordCount <= 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	out := append([]byte(nil), asset.Raw.Bytes...)
	order := packageByteOrder(asset)
	changed := false
	applyImport := func(importIdx int, set bool) {
		if importIdx <= 0 {
			return
		}
		zeroIdx := importIdx - 1
		if zeroIdx < 0 || zeroIdx >= importCount {
			return
		}
		word := zeroIdx / 32
		if word < 0 || word >= wordCount {
			return
		}
		bit := uint(zeroIdx % 32)
		wordPos := int(sectionStart) + trailerStart + 5 + word*4
		if wordPos < 0 || wordPos+4 > len(out) {
			return
		}
		current := order.Uint32(out[wordPos : wordPos+4])
		next := current
		if set {
			next |= uint32(1) << bit
		} else {
			next &^= uint32(1) << bit
		}
		if next != current {
			order.PutUint32(out[wordPos:wordPos+4], next)
			changed = true
		}
	}
	for _, importIdx := range setImports {
		applyImport(importIdx, true)
	}
	for _, importIdx := range clearImports {
		applyImport(importIdx, false)
	}
	if !changed {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	if err := edit.FinalizePackageBytes(out, asset.Summary.FileVersionUE5); err != nil {
		return nil, nil, fmt.Errorf("finalize package bytes: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(out, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, nil, fmt.Errorf("reparse rewritten asset: %w", err)
	}
	return out, updatedAsset, nil
}

func locateWidgetAddAssetRegistryImportedClassesTrailer(sectionBytes []byte, order binary.ByteOrder, expectedImportCount int) (start int, importCount int, wordCount int, err error) {
	if len(sectionBytes) == 0 {
		return -1, 0, 0, nil
	}
	matches := 0
	foundStart := -1
	foundImportCount := 0
	foundWordCount := 0
	for candidateWords := 0; ; candidateWords++ {
		trailerLen := 17 + candidateWords*4
		if trailerLen > len(sectionBytes) {
			break
		}
		candidateStart := len(sectionBytes) - trailerLen
		if sectionBytes[candidateStart] != 0 {
			continue
		}
		count := int(order.Uint32(sectionBytes[candidateStart+1 : candidateStart+5]))
		if expectedImportCount > 0 && count != expectedImportCount {
			continue
		}
		expectedWords := 0
		if count > 0 {
			expectedWords = (count + 31) / 32
		}
		if expectedWords != candidateWords {
			continue
		}
		tailPos := candidateStart + 5 + candidateWords*4
		if tailPos+12 != len(sectionBytes) {
			continue
		}
		tailA := order.Uint32(sectionBytes[tailPos : tailPos+4])
		tailB := order.Uint32(sectionBytes[tailPos+4 : tailPos+8])
		tailC := order.Uint32(sectionBytes[tailPos+8 : tailPos+12])
		if tailC != 0 || tailA == 0 || tailB == 0 {
			continue
		}
		matches++
		foundStart = candidateStart
		foundImportCount = count
		foundWordCount = candidateWords
	}
	switch matches {
	case 0:
		return -1, 0, 0, fmt.Errorf("asset registry imported classes trailer layout is unsupported")
	case 1:
		return foundStart, foundImportCount, foundWordCount, nil
	default:
		return -1, 0, 0, fmt.Errorf("asset registry imported classes trailer layout is ambiguous")
	}
}

func widgetAddSortedWidgetAndSlotExports(target widgetWriteTarget) (int, int, error) {
	exports := append([]int(nil), target.Exports...)
	slices.Sort(exports)
	switch len(exports) {
	case 0:
		return -1, -1, fmt.Errorf("widget target %s has no exports", target.Path)
	case 1:
		return exports[0], -1, nil
	default:
		return exports[0], exports[1], nil
	}
}

func widgetAddSortedSlotExports(target widgetWriteTarget) (int, int, error) {
	slots := append([]int(nil), target.SlotExports...)
	slices.Sort(slots)
	switch len(slots) {
	case 0:
		return -1, -1, fmt.Errorf("widget target %s has no slot exports", target.Path)
	case 1:
		return slots[0], -1, nil
	default:
		return slots[0], slots[1], nil
	}
}

func sortWidgetTargetsByDesignerSlotOrder(asset *uasset.Asset, parentExport int, targets []widgetWriteTarget) {
	slotOrder := map[int]int{}
	if asset != nil && parentExport >= 0 {
		if refs, err := readWidgetAddObjectRefArrayValue(asset, parentExport, "Slots"); err == nil {
			for pos, ref := range refs {
				if idx, ok := widgetAddDecodedObjectRefExportIndex(ref); ok {
					slotOrder[idx] = pos
				}
			}
		}
	}
	for i := 0; i < len(targets); i++ {
		best := i
		for j := i + 1; j < len(targets); j++ {
			if compareWidgetTargetDesignerSlotOrder(targets[j], targets[best], slotOrder) < 0 {
				best = j
			}
		}
		if best != i {
			targets[i], targets[best] = targets[best], targets[i]
		}
	}
}

func compareWidgetTargetDesignerSlotOrder(a, b widgetWriteTarget, slotOrder map[int]int) int {
	aOrder := widgetTargetDesignerSlotOrder(a, slotOrder)
	bOrder := widgetTargetDesignerSlotOrder(b, slotOrder)
	if aOrder != bOrder {
		if aOrder < bOrder {
			return -1
		}
		return 1
	}
	if cmp, ok := compareWidgetAddNameNaturalOrder(a.ObjectName, b.ObjectName); ok && cmp != 0 {
		return cmp
	}
	return strings.Compare(a.Path, b.Path)
}

func compareWidgetAddNameNaturalOrder(a, b string) (int, bool) {
	aName, errA := parseWidgetAddName(a)
	bName, errB := parseWidgetAddName(b)
	if errA != nil || errB != nil {
		return 0, false
	}
	if !strings.EqualFold(aName.Base, bName.Base) {
		return strings.Compare(aName.Base, bName.Base), true
	}
	if aName.Number < bName.Number {
		return -1, true
	}
	if aName.Number > bName.Number {
		return 1, true
	}
	return 0, true
}

func widgetTargetDesignerSlotOrder(target widgetWriteTarget, slotOrder map[int]int) int {
	if len(target.SlotExports) > 0 {
		if pos, ok := slotOrder[target.SlotExports[0]]; ok {
			return pos
		}
		return target.SlotExports[0]
	}
	return maxIntLocal(1<<30, len(target.Path))
}

func widgetAddDecodedObjectRefExportIndex(value any) (int, bool) {
	ref, ok := value.(map[string]any)
	if !ok {
		return 0, false
	}
	raw, ok := widgetAddInt64(ref["index"])
	if !ok || raw <= 0 {
		return 0, false
	}
	return int(raw) - 1, true
}

func widgetAddDecodedObjectRefIsNull(value any) bool {
	ref, ok := value.(map[string]any)
	if !ok {
		return false
	}
	raw, ok := widgetAddInt64(ref["index"])
	return ok && raw == 0
}

func widgetAddInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		return int64(v), true
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		if v > 1<<63-1 {
			return 0, false
		}
		return int64(v), true
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}

func removeWidgetAddOverlayRootChainLegacyNames(asset *uasset.Asset, opts uasset.ParseOptions, ctx widgetAddContext) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	targets, err := collectWidgetWriteTargets(asset, ctx.Target.BlueprintExport+1)
	if err != nil {
		return nil, nil, err
	}
	candidates := make([]string, 0, 4)
	currentCategory := widgetAddBlueprintCategoryName(ctx.BlueprintObjectName)
	for _, target := range targets {
		if !strings.HasPrefix(target.Path, ctx.Target.Path+"/") {
			continue
		}
		legacyName := widgetAddLegacyGeneratedVariableCategoryName(target.ObjectName)
		if legacyName == "" || strings.EqualFold(legacyName, currentCategory) {
			continue
		}
		candidates = append(candidates, legacyName)
	}
	return removeWidgetAddNameValues(asset, opts, candidates)
}

func widgetAddLegacyGeneratedVariableCategoryName(display string) string {
	category := widgetAddBlueprintCategoryName(display)
	if category == "" {
		return ""
	}
	return "WBP " + category
}

func removeWidgetAddNameValues(asset *uasset.Asset, opts uasset.ParseOptions, values []string) ([]byte, *uasset.Asset, error) {
	workingBytes := append([]byte(nil), asset.Raw.Bytes...)
	workingAsset := asset
	seen := map[string]bool{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		nameIdx := findNameIndexByValue(workingAsset.Names, trimmed)
		if nameIdx < 0 {
			continue
		}
		nextBytes, nextAsset, err := removeWidgetAddNameValue(workingAsset, opts, int32(nameIdx))
		if err != nil {
			return nil, nil, fmt.Errorf("remove name %q: %w", trimmed, err)
		}
		workingBytes = nextBytes
		workingAsset = nextAsset
	}
	return workingBytes, workingAsset, nil
}

func removeWidgetAddNameValue(asset *uasset.Asset, opts uasset.ParseOptions, removeIdx int32) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if removeIdx < 0 || int(removeIdx) >= len(asset.Names) {
		return nil, nil, fmt.Errorf("name index out of range: %d", removeIdx)
	}
	newNames := make([]uasset.NameEntry, 0, len(asset.Names)-1)
	indexRemap := make(map[int32]int32, len(asset.Names)-1)
	nextIdx := int32(0)
	for i, entry := range asset.Names {
		if int32(i) == removeIdx {
			continue
		}
		newNames = append(newNames, entry)
		indexRemap[int32(i)] = nextIdx
		nextIdx++
	}
	if len(newNames) == len(asset.Names) {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	workingBytes, err := edit.RewriteImportExportNameRefs(asset, indexRemap)
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite import/export name refs: %w", err)
	}
	remapped := *asset
	remapped.Raw.Bytes = workingBytes
	workingBytes, err = edit.RewriteNameMap(&remapped, newNames)
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite name map: %w", err)
	}
	workingAsset, err := uasset.ParseBytes(workingBytes, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("reparse after name removal: %w", err)
	}

	exportMutations, err := edit.BuildExportNameRemapMutations(asset, workingAsset, indexRemap, "", "")
	if err != nil {
		return nil, nil, fmt.Errorf("build export name remap mutations: %w", err)
	}
	if len(exportMutations) > 0 {
		workingBytes, err = edit.RewriteAsset(workingAsset, exportMutations)
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite export payloads: %w", err)
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("reparse after export payload rewrite: %w", err)
		}
	}

	workingBytes, workingAsset, err = edit.PatchInstancedTailNameRefs(asset, workingAsset, indexRemap, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("patch instanced tail name refs: %w", err)
	}
	return workingBytes, workingAsset, nil
}

func findWidgetAddChildExportsByPath(asset *uasset.Asset, blueprintExport int, path string) (int, int) {
	targets, err := collectWidgetWriteTargets(asset, blueprintExport+1)
	if err != nil {
		return -1, -1
	}
	target, err := selectWidgetWriteTarget(targets, path)
	if err != nil || len(target.Exports) == 0 {
		return -1, -1
	}
	designer := target.Exports[0]
	generated := -1
	if len(target.Exports) > 1 {
		generated = target.Exports[1]
	}
	return designer, generated
}

func findWidgetAddSlotExportsByPath(asset *uasset.Asset, blueprintExport int, path string) (int, int) {
	targets, err := collectWidgetWriteTargets(asset, blueprintExport+1)
	if err != nil {
		return -1, -1
	}
	target, err := selectWidgetWriteTarget(targets, path)
	if err != nil || len(target.SlotExports) == 0 {
		return -1, -1
	}
	designer := target.SlotExports[0]
	generated := -1
	if len(target.SlotExports) > 1 {
		generated = target.SlotExports[1]
	}
	return designer, generated
}

func applyWidgetAddPropertyWriteWithFlags(asset *uasset.Asset, exportIndex int, propertyName, propertyType string, value any, propertyFlags uint8, beforeProperty string) ([]byte, *uasset.Asset, error) {
	valueJSON, err := marshalJSONValue(value)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal %s value: %w", propertyName, err)
	}
	editResult, err := edit.BuildPropertySetMutation(asset, exportIndex, propertyName, valueJSON)
	if err != nil {
		fallbackResult, fallbackErr := buildPropertySetStructLeafFallbackMutation(asset, exportIndex, propertyName, valueJSON)
		if fallbackErr == nil {
			editResult = fallbackResult
		} else if strings.Contains(err.Error(), "property not found") {
			specJSON, specErr := marshalJSONValue(map[string]any{
				"name":  propertyName,
				"type":  propertyType,
				"flags": propertyFlags,
				"value": value,
			})
			if specErr != nil {
				return nil, nil, fmt.Errorf("marshal property add spec for %s: %w", propertyName, specErr)
			}
			addResult, addErr := edit.BuildPropertyAddMutationBefore(asset, exportIndex, specJSON, beforeProperty)
			if addErr != nil {
				return nil, nil, addErr
			}
			outBytes, rewriteErr := edit.RewriteAsset(asset, []edit.ExportMutation{addResult.Mutation})
			if rewriteErr != nil {
				return nil, nil, fmt.Errorf("rewrite asset: %w", rewriteErr)
			}
			updatedAsset, parseErr := uasset.ParseBytes(outBytes, uasset.ParseOptions{KeepUnknown: true})
			if parseErr != nil {
				return nil, nil, fmt.Errorf("reparse rewritten asset: %w", parseErr)
			}
			return outBytes, updatedAsset, nil
		} else {
			return nil, nil, err
		}
	}
	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{editResult.Mutation})
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite asset: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(outBytes, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, nil, fmt.Errorf("reparse rewritten asset: %w", err)
	}
	return outBytes, updatedAsset, nil
}

func applyWidgetAddDependsMap(asset *uasset.Asset, ctx widgetAddContext, designerSlotExport int, generatedSlotExport int, designerImageExport int, generatedImageExport int, childClassImportIndex int) ([]byte, *uasset.Asset, error) {
	parentTarget, directChildren, err := collectWidgetAddDirectChildTargets(asset, ctx)
	if err != nil {
		return nil, nil, err
	}
	_, parentGeneratedSlot, _ := widgetAddSortedSlotExports(parentTarget)

	designerParentDeps, generatedParentDeps, err := buildWidgetAddParentDepends(parentTarget, directChildren)
	if err != nil {
		return nil, nil, err
	}

	designerChildDeps := []uasset.PackageIndex{
		uasset.PackageIndex(designerSlotExport + 1),
	}
	generatedChildDeps := []uasset.PackageIndex{}
	if generatedSlotExport >= 0 {
		generatedChildDeps = append(generatedChildDeps, uasset.PackageIndex(generatedSlotExport+1))
	}
	if childClassImportIndex > 0 {
		classImportDep := uasset.PackageIndex(-childClassImportIndex)
		designerChildDeps = append([]uasset.PackageIndex{classImportDep}, designerChildDeps...)
		if len(generatedChildDeps) > 0 {
			generatedChildDeps = append([]uasset.PackageIndex{classImportDep}, generatedChildDeps...)
		}
	}

	replaceUpdates := map[int][]uasset.PackageIndex{
		ctx.DesignerParentExport: designerParentDeps,
		designerSlotExport: {
			uasset.PackageIndex(ctx.DesignerParentExport + 1),
			uasset.PackageIndex(designerImageExport + 1),
		},
		designerImageExport: designerChildDeps,
	}
	if ctx.GeneratedParentExport >= 0 {
		replaceUpdates[ctx.GeneratedParentExport] = generatedParentDeps
	}
	if generatedSlotExport >= 0 && generatedImageExport < 0 && ctx.GeneratedParentExport >= 0 {
		replaceUpdates[generatedSlotExport] = []uasset.PackageIndex{}
	}
	if generatedSlotExport >= 0 && generatedImageExport >= 0 && ctx.GeneratedParentExport >= 0 && parentGeneratedSlot >= 0 {
		replaceUpdates[generatedSlotExport] = []uasset.PackageIndex{
			uasset.PackageIndex(ctx.GeneratedParentExport + 1),
			uasset.PackageIndex(generatedImageExport + 1),
		}
	}
	if generatedSlotExport >= 0 && generatedImageExport >= 0 {
		replaceUpdates[generatedImageExport] = generatedChildDeps
	}

	updates := make([]widgetAddDependsUpdate, 0, len(replaceUpdates)+1)
	for exportIndex, deps := range replaceUpdates {
		updates = append(updates, widgetAddDependsUpdate{
			exportIndex: exportIndex,
			deps:        deps,
		})
	}
	if ctx.DesignerTreeExport >= 0 {
		updates = append(updates, widgetAddDependsUpdate{
			exportIndex: ctx.DesignerTreeExport,
			deps: []uasset.PackageIndex{
				uasset.PackageIndex(ctx.DesignerParentExport + 1),
				uasset.PackageIndex(designerImageExport + 1),
			},
			appendOnly: true,
		})
	}
	if ctx.GeneratedTreeExport >= 0 &&
		ctx.GeneratedParentExport >= 0 &&
		strings.Contains(ctx.Target.Path, "/") &&
		strings.EqualFold(ctx.PanelClass, "CanvasPanel") {
		generatedTreeDeps := []uasset.PackageIndex{
			uasset.PackageIndex(ctx.GeneratedParentExport + 1),
		}
		if generatedImageExport >= 0 {
			generatedTreeDeps = append(generatedTreeDeps, uasset.PackageIndex(generatedImageExport+1))
		}
		updates = append(updates, widgetAddDependsUpdate{
			exportIndex: ctx.GeneratedTreeExport,
			deps:        generatedTreeDeps,
			appendOnly:  true,
		})
	}
	sortDepUpdatesByExportIndex(updates)

	workingBytes := append([]byte(nil), asset.Raw.Bytes...)
	workingAsset := asset
	for _, update := range updates {
		if update.exportIndex < 0 {
			continue
		}
		if update.appendOnly {
			workingBytes, err = edit.AppendExportDependencies(workingAsset, update.exportIndex, update.deps)
		} else {
			workingBytes, err = edit.ReplaceExportDependencies(workingAsset, update.exportIndex, update.deps)
		}
		if err != nil {
			return nil, nil, err
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, uasset.ParseOptions{KeepUnknown: true})
		if err != nil {
			return nil, nil, fmt.Errorf("reparse depends rewrite: %w", err)
		}
	}
	return workingBytes, workingAsset, nil
}

func collectWidgetAddDirectChildTargets(asset *uasset.Asset, ctx widgetAddContext) (widgetWriteTarget, []widgetWriteTarget, error) {
	if asset == nil {
		return widgetWriteTarget{}, nil, fmt.Errorf("asset is nil")
	}
	if ctx.Target.BlueprintExport < 0 {
		return widgetWriteTarget{}, nil, fmt.Errorf("target blueprint export is not resolved")
	}

	targets, err := collectWidgetWriteTargets(asset, ctx.Target.BlueprintExport+1)
	if err != nil {
		return widgetWriteTarget{}, nil, err
	}
	parentTarget, err := selectWidgetWriteTarget(targets, ctx.Target.Path)
	if err != nil {
		return widgetWriteTarget{}, nil, err
	}

	parentDepth := len(splitWidgetPathSegments(parentTarget.Path))
	children := make([]widgetWriteTarget, 0, 8)
	for _, target := range targets {
		if !strings.HasPrefix(target.Path, parentTarget.Path+"/") {
			continue
		}
		if len(splitWidgetPathSegments(target.Path)) != parentDepth+1 {
			continue
		}
		children = append(children, target)
	}
	if len(parentTarget.Exports) > 0 {
		sortWidgetTargetsByDesignerSlotOrder(asset, parentTarget.Exports[0], children)
	}
	return *parentTarget, children, nil
}

func buildWidgetAddParentDepends(parentTarget widgetWriteTarget, children []widgetWriteTarget) ([]uasset.PackageIndex, []uasset.PackageIndex, error) {
	parentDesignerSlot, parentGeneratedSlot, err := widgetAddSortedSlotExports(parentTarget)
	if err != nil {
		parentDesignerSlot = -1
		parentGeneratedSlot = -1
	}

	designerDeps := make([]uasset.PackageIndex, 0, len(children)+1)
	generatedDeps := make([]uasset.PackageIndex, 0, len(children)+1)
	for _, child := range children {
		designerSlot, generatedSlot, childErr := widgetAddSortedSlotExports(child)
		if childErr != nil {
			return nil, nil, childErr
		}
		if designerSlot >= 0 {
			designerDeps = append(designerDeps, uasset.PackageIndex(designerSlot+1))
		}
		if parentGeneratedSlot >= 0 && generatedSlot >= 0 {
			generatedDeps = append(generatedDeps, uasset.PackageIndex(generatedSlot+1))
		}
	}
	if parentDesignerSlot >= 0 {
		designerDeps = append(designerDeps, uasset.PackageIndex(parentDesignerSlot+1))
	}
	if parentGeneratedSlot >= 0 {
		generatedDeps = append(generatedDeps, uasset.PackageIndex(parentGeneratedSlot+1))
	}
	return designerDeps, generatedDeps, nil
}

func sortDepUpdatesByExportIndex(updates []widgetAddDependsUpdate) {
	slices.SortFunc(updates, func(a, b widgetAddDependsUpdate) int {
		switch {
		case a.exportIndex < b.exportIndex:
			return -1
		case a.exportIndex > b.exportIndex:
			return 1
		default:
			return 0
		}
	})
}

func applyWidgetAddRootDependsMap(asset *uasset.Asset, ctx widgetAddContext, designerRootExport int) ([]byte, *uasset.Asset, error) {
	workingBytes, err := edit.AppendExportDependencies(asset, ctx.DesignerTreeExport, []uasset.PackageIndex{
		uasset.PackageIndex(designerRootExport + 1),
	})
	if err != nil {
		return nil, nil, err
	}
	workingAsset, err := uasset.ParseBytes(workingBytes, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, nil, fmt.Errorf("reparse root depends rewrite: %w", err)
	}
	return workingBytes, workingAsset, nil
}

func objectRefValue(exportIndex int) map[string]any {
	return map[string]any{"index": int32(exportIndex + 1)}
}

func normalizeWidgetAddTextNamespaces(asset *uasset.Asset, ctx widgetAddContext) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}

	packageNamespace := widgetPackageLocalizationNamespace(asset)
	if packageNamespace == "" {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	targets, err := collectWidgetWriteTargets(asset, ctx.Target.BlueprintExport+1)
	if err != nil {
		return nil, nil, err
	}

	exports := make([]int, 0, len(targets)*2)
	seen := make(map[int]struct{}, len(targets)*2)
	for _, target := range targets {
		for _, exportIdx := range target.Exports {
			if _, ok := seen[exportIdx]; ok {
				continue
			}
			seen[exportIdx] = struct{}{}
			exports = append(exports, exportIdx)
		}
	}
	sortIntsLocal(exports)

	workingBytes := append([]byte(nil), asset.Raw.Bytes...)
	workingAsset := asset
	for _, exportIdx := range exports {
		current, err := decodeExportRootPropertyValue(workingAsset, exportIdx, "Text")
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "property not found") {
				continue
			}
			return nil, nil, fmt.Errorf("decode export %d Text: %w", exportIdx+1, err)
		}
		textPayload, ok := current.(map[string]any)
		if !ok || !shouldNormalizeWidgetAddTextNamespace(textPayload, packageNamespace) {
			continue
		}
		updated := cloneAnyMapLocal(textPayload)
		updated["namespace"] = packageNamespace
		workingBytes, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, exportIdx, "Text", "TextProperty", updated, "Slot")
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite export %d Text namespace: %w", exportIdx+1, err)
		}
	}
	return workingBytes, workingAsset, nil
}

func shouldNormalizeWidgetAddTextNamespace(payload map[string]any, packageNamespace string) bool {
	if payload == nil || packageNamespace == "" {
		return false
	}
	historyType, _ := payload["historyType"].(string)
	if !strings.EqualFold(historyType, "Base") {
		return false
	}
	namespace, _ := payload["namespace"].(string)
	if namespace == "" || namespace == packageNamespace || !isWidgetPackageNamespaceMarker(namespace) {
		return false
	}
	sourceString, _ := payload["sourceString"].(string)
	return strings.TrimSpace(sourceString) != ""
}

func widgetPackageLocalizationNamespace(asset *uasset.Asset) string {
	if asset == nil {
		return ""
	}
	locID := strings.TrimSpace(asset.Summary.LocalizationID)
	if locID == "" {
		return ""
	}
	return "[" + locID + "]"
}

func isWidgetPackageNamespaceMarker(namespace string) bool {
	if len(namespace) != 34 || namespace[0] != '[' || namespace[len(namespace)-1] != ']' {
		return false
	}
	for _, r := range namespace[1 : len(namespace)-1] {
		if (r < '0' || r > '9') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

func rewriteWidgetAddBlueprintSoftObjectPaths(asset *uasset.Asset, opts uasset.ParseOptions, ctx widgetAddContext) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}

	entries, err := edit.ReadSoftObjectPathEntries(asset)
	if err != nil {
		return nil, nil, err
	}

	targetPackage := strings.TrimSpace(asset.Summary.PackageName)
	targetAsset := "SKEL_" + strings.TrimSpace(ctx.BlueprintObjectName) + "_C"
	positions := map[string]int{
		"Tick":         -1,
		"Construct":    -1,
		"PreConstruct": -1,
	}
	for i, entry := range entries {
		if !strings.EqualFold(entry.PackageName.Display(asset.Names), targetPackage) {
			continue
		}
		if !strings.EqualFold(entry.AssetName.Display(asset.Names), targetAsset) {
			continue
		}
		if _, ok := positions[entry.SubPath]; !ok {
			continue
		}
		positions[entry.SubPath] = i
	}
	if positions["Tick"] < 0 || positions["Construct"] < 0 || positions["PreConstruct"] < 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	reordered := make([]edit.SoftObjectPathEntry, len(entries))
	copy(reordered, entries)
	targetPositions := []int{positions["Tick"], positions["Construct"], positions["PreConstruct"]}
	sortIntsLocal(targetPositions)
	original := []edit.SoftObjectPathEntry{
		entries[targetPositions[0]],
		entries[targetPositions[1]],
		entries[targetPositions[2]],
	}
	reordered[targetPositions[0]] = original[2]
	reordered[targetPositions[1]] = original[1]
	reordered[targetPositions[2]] = original[0]
	desiredDocOrder := []string{
		reordered[targetPositions[0]].SubPath,
		reordered[targetPositions[1]].SubPath,
		reordered[targetPositions[2]].SubPath,
	}
	updatedAsset := asset
	changed := false
	if reordered[targetPositions[0]] != entries[targetPositions[0]] ||
		reordered[targetPositions[1]] != entries[targetPositions[1]] ||
		reordered[targetPositions[2]] != entries[targetPositions[2]] {
		var outBytes []byte
		outBytes, err = edit.RewriteSoftObjectPathEntries(asset, reordered)
		if err != nil {
			return nil, nil, err
		}
		updatedAsset, err = uasset.ParseBytes(outBytes, uasset.ParseOptions{KeepUnknown: true})
		if err != nil {
			return nil, nil, fmt.Errorf("reparse soft object path rewrite: %w", err)
		}
		changed = true
	}
	if !changed {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	return rewriteWidgetFunctionDocBlocks(updatedAsset, opts, desiredDocOrder)
}

func sortIntsLocal(items []int) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j] < items[i] {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func appendWidgetVariableGuidEntry(asset *uasset.Asset, blueprintExport int, childName widgetAddName) ([]byte, *uasset.Asset, string, error) {
	hadCurrent := false
	m := map[string]any{
		"keyType":    "NameProperty",
		"valueType":  "StructProperty",
		"replaceMap": false,
		"value":      []any{},
	}
	current, err := decodeExportRootPropertyValue(asset, blueprintExport, "WidgetVariableNameToGuidMap")
	if err == nil {
		decodedMap, ok := current.(map[string]any)
		if !ok {
			return nil, nil, "", fmt.Errorf("WidgetVariableNameToGuidMap is not decodable as a map")
		}
		m = cloneAnyMapLocal(decodedMap)
		hadCurrent = true
	} else if !strings.Contains(strings.ToLower(err.Error()), "property not found") {
		return nil, nil, "", err
	}
	items, err := anySliceLocal(m["value"])
	if err != nil {
		return nil, nil, "", fmt.Errorf("WidgetVariableNameToGuidMap value is not an entry list")
	}
	normalizedItems := make([]any, 0, len(items)+1)
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, nil, "", fmt.Errorf("WidgetVariableNameToGuidMap entry is not an object")
		}
		entryCopy := map[string]any{}
		for k, v := range entry {
			entryCopy[k] = v
		}
		valueWrapper, ok := entryCopy["value"].(map[string]any)
		if !ok {
			return nil, nil, "", fmt.Errorf("WidgetVariableNameToGuidMap entry value is not an object")
		}
		valueCopy := map[string]any{}
		for k, v := range valueWrapper {
			valueCopy[k] = v
		}
		if guidRaw, ok := valueCopy["value"].(string); ok && strings.TrimSpace(guidRaw) != "" {
			valueCopy["value"] = map[string]any{
				"structType": "Guid",
				"value":      guidRaw,
			}
		}
		entryCopy["value"] = valueCopy
		normalizedItems = append(normalizedItems, entryCopy)
	}
	nameRef, err := resolveDisplayNameRef(asset.Names, childName.Display)
	if err != nil {
		return nil, nil, "", err
	}
	newGUID := generateWidgetAddGUIDString()
	normalizedItems = append(normalizedItems, map[string]any{
		"key": map[string]any{
			"type": "NameProperty",
			"value": map[string]any{
				"index":  nameRef.Index,
				"name":   childName.Display,
				"number": nameRef.Number,
			},
		},
		"value": map[string]any{
			"type":  "StructProperty",
			"value": map[string]any{"structType": "Guid", "value": newGUID},
		},
	})
	m["value"] = normalizedItems
	propertyFlags := uint8(0)
	beforeProperty := ""
	if !hadCurrent {
		propertyFlags = 8
		beforeProperty = "WidgetTree"
	}
	outBytes, updatedAsset, err := applyWidgetAddPropertyWriteWithFlags(asset, blueprintExport, "WidgetVariableNameToGuidMap", "MapProperty(NameProperty,StructProperty(Guid(/Script/CoreUObject)))", m, propertyFlags, beforeProperty)
	if err != nil {
		return nil, nil, "", err
	}
	return outBytes, updatedAsset, newGUID, nil
}

func generateWidgetAddGUIDString() string {
	var guid uasset.GUID
	if _, err := rand.Read(guid[:]); err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	return guid.String()
}

func appendGeneratedClassPropertyGuidEntry(asset *uasset.Asset, generatedClassExport int, blueprintObjectName string, childWidgetClass string, childClassImportIndex int, childName widgetAddName, guid string, overlayChainMode bool, prependFieldRecord bool, prependEntry bool, layout generatedClassWidgetVariableFieldLayout) ([]byte, *uasset.Asset, error) {
	_ = childWidgetClass
	updatedAsset := asset
	nameRef, err := resolveDisplayNameRef(updatedAsset.Names, childName.Display)
	if err != nil {
		return nil, nil, err
	}
	guidValue, err := parseWidgetAddGUIDString(guid)
	if err != nil {
		return nil, nil, err
	}
	_, updatedAsset, err = appendGeneratedClassPropertyGuidEntryRaw(updatedAsset, generatedClassExport, nameRef, guidValue, prependEntry)
	if err != nil {
		return nil, nil, err
	}
	nextLayout, err := captureGeneratedClassWidgetVariableFieldLayout(updatedAsset, generatedClassExport)
	if err != nil {
		if layout.ScriptEnd > 0 {
			nextLayout = layout
		} else {
			return nil, nil, err
		}
	}
	outBytes, updatedAsset, err := appendGeneratedClassWidgetVariableFieldWithLayout(updatedAsset, generatedClassExport, blueprintObjectName, childWidgetClass, childClassImportIndex, childName, overlayChainMode, prependFieldRecord, nextLayout)
	if err != nil {
		return nil, nil, err
	}
	return outBytes, updatedAsset, nil
}

func extractWrappedValueLocal(v any) any {
	if m, ok := v.(map[string]any); ok {
		if inner, ok := m["value"]; ok {
			return inner
		}
	}
	return v
}

type widgetAddGUIDMapEntry struct {
	Key  uasset.NameRef
	GUID uasset.GUID
}

type widgetAddGUIDMapValue struct {
	ReplaceMap bool
	Removed    []uasset.NameRef
	Entries    []widgetAddGUIDMapEntry
	Trailing   []byte
}

func appendGeneratedClassPropertyGuidEntryRaw(asset *uasset.Asset, exportIndex int, nameRef uasset.NameRef, guid uasset.GUID, prependEntry bool) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if exportIndex < 0 || exportIndex >= len(asset.Exports) {
		return nil, nil, fmt.Errorf("generated class export index out of range: %d", exportIndex+1)
	}

	parsed := asset.ParseExportProperties(exportIndex)
	existingProp, existingPropIndex := findWidgetAddRootPropertyTag(parsed.Properties, asset.Names, "PropertyGuids")
	mapValue := widgetAddGUIDMapValue{ReplaceMap: false}
	if existingProp != nil {
		var err error
		mapValue, err = parseWidgetAddGUIDMapPropertyValue(asset, *existingProp)
		if err != nil {
			return nil, nil, err
		}
	}

	filteredEntries := make([]widgetAddGUIDMapEntry, 0, len(mapValue.Entries)+1)
	for _, entry := range mapValue.Entries {
		if entry.Key.Index == nameRef.Index && entry.Key.Number == nameRef.Number {
			continue
		}
		filteredEntries = append(filteredEntries, entry)
	}
	newEntry := widgetAddGUIDMapEntry{Key: nameRef, GUID: guid}
	if prependEntry {
		mapValue.Entries = append([]widgetAddGUIDMapEntry{newEntry}, filteredEntries...)
	} else {
		mapValue.Entries = append(filteredEntries, newEntry)
	}

	valueBytes, err := encodeWidgetAddGUIDMapPropertyValue(asset, mapValue)
	if err != nil {
		return nil, nil, err
	}

	exp := asset.Exports[exportIndex]
	serialStart := int(exp.SerialOffset)
	serialEnd := int(exp.SerialOffset + exp.SerialSize)
	if serialStart < 0 || serialEnd < serialStart || serialEnd > len(asset.Raw.Bytes) {
		return nil, nil, fmt.Errorf("generated class export serial range out of bounds")
	}
	payload := append([]byte(nil), asset.Raw.Bytes[serialStart:serialEnd]...)

	var newPayload []byte
	if existingProp != nil {
		propEnd := widgetAddRootPropertyEndOffset(parsed, existingPropIndex)
		newTagBytes, err := rewriteWidgetAddRootPropertyTagValue(asset, *existingProp, valueBytes)
		if err != nil {
			return nil, nil, err
		}
		startRel := existingProp.Offset - serialStart
		endRel := propEnd - serialStart
		if startRel < 0 || endRel < startRel || endRel > len(payload) {
			return nil, nil, fmt.Errorf("PropertyGuids replacement range out of bounds")
		}
		newPayload = make([]byte, 0, len(payload)-((endRel-startRel)-len(newTagBytes)))
		newPayload = append(newPayload, payload[:startRel]...)
		newPayload = append(newPayload, newTagBytes...)
		newPayload = append(newPayload, payload[endRel:]...)
	} else {
		insertPos := parsed.EndOffset - 8 // preserve the tagged-property NAME_None terminator
		if insertPos < serialStart || insertPos > serialEnd {
			return nil, nil, fmt.Errorf("PropertyGuids insert position out of bounds")
		}
		newTagBytes, err := buildWidgetAddPropertyGuidsTag(asset, valueBytes)
		if err != nil {
			return nil, nil, err
		}
		insertRel := insertPos - serialStart
		newPayload = make([]byte, 0, len(payload)+len(newTagBytes))
		newPayload = append(newPayload, payload[:insertRel]...)
		newPayload = append(newPayload, newTagBytes...)
		newPayload = append(newPayload, payload[insertRel:]...)
	}

	payloadDelta := int64(len(newPayload) - len(payload))
	newScriptEnd := exp.ScriptSerializationEndOffset + payloadDelta
	if newScriptEnd < exp.ScriptSerializationStartOffset {
		return nil, nil, fmt.Errorf("generated class PropertyGuids rewrite produced invalid script range")
	}
	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{{
		ExportIndex:    exportIndex,
		Payload:        newPayload,
		UpdateScript:   true,
		ScriptStartRel: exp.ScriptSerializationStartOffset,
		ScriptEndRel:   newScriptEnd,
	}})
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite generated class PropertyGuids: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(outBytes, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, nil, fmt.Errorf("reparse generated class PropertyGuids rewrite: %w", err)
	}
	return outBytes, updatedAsset, nil
}

func appendGeneratedClassNamedSlotMetadata(asset *uasset.Asset, exportIndex int, nameRef uasset.NameRef, slotGuid uasset.GUID) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	workingAsset := asset
	var err error
	_, workingAsset, err = appendGeneratedClassNameArrayPropertyEntry(workingAsset, exportIndex, "NamedSlots", nameRef, "PropertyGuids")
	if err != nil {
		return nil, nil, err
	}
	_, workingAsset, err = appendGeneratedClassNameGuidMapPropertyEntry(workingAsset, exportIndex, "NamedSlotsWithID", nameRef, slotGuid, "PropertyGuids")
	if err != nil {
		return nil, nil, err
	}
	_, workingAsset, err = appendGeneratedClassNameArrayPropertyEntry(workingAsset, exportIndex, "AvailableNamedSlots", nameRef, "PropertyGuids")
	if err != nil {
		return nil, nil, err
	}
	_, workingAsset, err = appendGeneratedClassNameArrayPropertyEntry(workingAsset, exportIndex, "InstanceNamedSlots", nameRef, "PropertyGuids")
	if err != nil {
		return nil, nil, err
	}
	return append([]byte(nil), workingAsset.Raw.Bytes...), workingAsset, nil
}

func rewriteWidgetAddNamedSlotAssetRegistryTags(asset *uasset.Asset, blueprintObjectName string, slotName string) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	blueprintObjectName = strings.TrimSpace(blueprintObjectName)
	slotName = strings.TrimSpace(slotName)
	if blueprintObjectName == "" || slotName == "" {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	section, sectionStart, sectionEnd, err := parseAssetRegistrySection(asset)
	if err != nil {
		return nil, nil, err
	}
	if section == nil {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	changed := false
	for objIdx := range section.Objects {
		obj := &section.Objects[objIdx]
		targetValue := ""
		switch {
		case strings.EqualFold(obj.ObjectPath, blueprintObjectName):
			targetValue = slotName
		case strings.EqualFold(obj.ObjectPath, blueprintObjectName+"_C"):
			targetValue = fmt.Sprintf("(\"%s\")", slotName)
		default:
			continue
		}
		for tagIdx := range obj.Tags {
			tag := &obj.Tags[tagIdx]
			if !strings.EqualFold(tag.Key, "AvailableNamedSlots") {
				continue
			}
			if tag.Value == targetValue {
				continue
			}
			tag.Value = targetValue
			changed = true
		}
	}
	if !changed {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	outBytes, err := encodeAssetRegistrySection(asset, section, sectionStart, 0, false)
	if err != nil {
		return nil, nil, err
	}
	rewritten, err := edit.RewriteRawRange(asset, sectionStart, sectionEnd, outBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite NamedSlot asset registry section: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(rewritten, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, nil, fmt.Errorf("reparse NamedSlot asset registry rewrite: %w", err)
	}
	return rewritten, updatedAsset, nil
}

func appendGeneratedClassNameArrayPropertyEntry(asset *uasset.Asset, exportIndex int, propertyName string, nameRef uasset.NameRef, beforeProperty string) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	values := make([]any, 0, 1)
	current, err := decodeExportRootPropertyValue(asset, exportIndex, propertyName)
	if err == nil {
		decodedArray, ok := current.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("%s is not a decoded array property", propertyName)
		}
		items, err := anySliceLocal(decodedArray["value"])
		if err != nil {
			return nil, nil, fmt.Errorf("%s value is not an entry list", propertyName)
		}
		values = make([]any, 0, len(items)+1)
		for _, item := range items {
			unwrapped := extractWrappedValueLocal(item)
			valueMap, ok := unwrapped.(map[string]any)
			if !ok {
				return nil, nil, fmt.Errorf("%s entry value is not an object", propertyName)
			}
			if currentIndex, ok := intFromAny(valueMap["index"]); ok && int32(currentIndex) == nameRef.Index {
				if currentNumber, ok := intFromAny(valueMap["number"]); ok && int32(currentNumber) == nameRef.Number {
					return append([]byte(nil), asset.Raw.Bytes...), asset, nil
				}
			}
			values = append(values, cloneAnyMapLocal(valueMap))
		}
	} else if !strings.Contains(strings.ToLower(err.Error()), "property not found") {
		return nil, nil, err
	}
	values = append(values, map[string]any{
		"index":  nameRef.Index,
		"name":   nameRef.Display(asset.Names),
		"number": nameRef.Number,
	})
	return applyWidgetAddPropertyWrite(asset, exportIndex, propertyName, "ArrayProperty(NameProperty)", values, beforeProperty)
}

func appendGeneratedClassNameGuidMapPropertyEntry(asset *uasset.Asset, exportIndex int, propertyName string, nameRef uasset.NameRef, guid uasset.GUID, beforeProperty string) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	m := map[string]any{
		"keyType":    "NameProperty",
		"valueType":  "StructProperty",
		"replaceMap": false,
		"value":      []any{},
	}
	current, err := decodeExportRootPropertyValue(asset, exportIndex, propertyName)
	if err == nil {
		decodedMap, ok := current.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("%s is not a decoded map property", propertyName)
		}
		m = cloneAnyMapLocal(decodedMap)
	} else if !strings.Contains(strings.ToLower(err.Error()), "property not found") {
		return nil, nil, err
	}
	items, err := anySliceLocal(m["value"])
	if err != nil {
		return nil, nil, fmt.Errorf("%s value is not an entry list", propertyName)
	}
	normalizedItems := make([]any, 0, len(items)+1)
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("%s entry is not an object", propertyName)
		}
		entryCopy := cloneAnyMapLocal(entry)
		keyWrapper, ok := entryCopy["key"].(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("%s entry key is not an object", propertyName)
		}
		keyValue, ok := keyWrapper["value"].(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("%s entry key value is not an object", propertyName)
		}
		if currentIndex, ok := intFromAny(keyValue["index"]); ok && int32(currentIndex) == nameRef.Index {
			if currentNumber, ok := intFromAny(keyValue["number"]); ok && int32(currentNumber) == nameRef.Number {
				continue
			}
		}
		normalizedItems = append(normalizedItems, entryCopy)
	}
	normalizedItems = append(normalizedItems, map[string]any{
		"key": map[string]any{
			"type": "NameProperty",
			"value": map[string]any{
				"index":  nameRef.Index,
				"name":   nameRef.Display(asset.Names),
				"number": nameRef.Number,
			},
		},
		"value": map[string]any{
			"type":  "StructProperty",
			"value": map[string]any{"structType": "Guid", "value": guid.String()},
		},
	})
	m["value"] = normalizedItems
	return applyWidgetAddPropertyWriteWithFlags(asset, exportIndex, propertyName, "MapProperty(NameProperty,StructProperty(Guid(/Script/CoreUObject)))", m, 8, beforeProperty)
}

func findWidgetAddRootPropertyTag(props []uasset.PropertyTag, names []uasset.NameEntry, propertyName string) (*uasset.PropertyTag, int) {
	for i := range props {
		if strings.EqualFold(props[i].Name.Display(names), propertyName) {
			return &props[i], i
		}
	}
	return nil, -1
}

func widgetAddRootPropertyEndOffset(parsed uasset.PropertyListResult, propIndex int) int {
	if propIndex >= 0 && propIndex+1 < len(parsed.Properties) {
		return parsed.Properties[propIndex+1].Offset
	}
	if parsed.EndOffset >= 8 {
		return parsed.EndOffset - 8
	}
	return parsed.EndOffset
}

func parseWidgetAddGUIDMapPropertyValue(asset *uasset.Asset, prop uasset.PropertyTag) (widgetAddGUIDMapValue, error) {
	state := widgetAddGUIDMapValue{}
	if asset == nil {
		return state, fmt.Errorf("asset is nil")
	}
	start := prop.ValueOffset
	end := prop.ValueOffset + int(prop.Size)
	if start < 0 || end < start || end > len(asset.Raw.Bytes) {
		return state, fmt.Errorf("PropertyGuids value range out of bounds")
	}
	reader := uasset.NewByteReaderWithByteSwapping(asset.Raw.Bytes[start:end], asset.Summary.UsesByteSwappedSerialization())
	removeCount, err := reader.ReadInt32()
	if err != nil {
		return state, fmt.Errorf("read PropertyGuids remove count: %w", err)
	}
	switch {
	case removeCount == -1:
		state.ReplaceMap = true
	case removeCount >= 0:
		state.ReplaceMap = false
		state.Removed = make([]uasset.NameRef, 0, removeCount)
		for i := int32(0); i < removeCount; i++ {
			keyRef, err := reader.ReadNameRef(len(asset.Names))
			if err != nil {
				return state, fmt.Errorf("read PropertyGuids removed key %d: %w", i, err)
			}
			state.Removed = append(state.Removed, keyRef)
		}
	default:
		return state, fmt.Errorf("invalid PropertyGuids remove count: %d", removeCount)
	}

	addCount, err := reader.ReadInt32()
	if err != nil {
		return state, fmt.Errorf("read PropertyGuids add count: %w", err)
	}
	if addCount < 0 {
		return state, fmt.Errorf("invalid PropertyGuids add count: %d", addCount)
	}
	state.Entries = make([]widgetAddGUIDMapEntry, 0, addCount)
	for i := int32(0); i < addCount; i++ {
		keyRef, err := reader.ReadNameRef(len(asset.Names))
		if err != nil {
			return state, fmt.Errorf("read PropertyGuids key %d: %w", i, err)
		}
		guid, err := reader.ReadGUID()
		if err != nil {
			return state, fmt.Errorf("read PropertyGuids guid %d: %w", i, err)
		}
		state.Entries = append(state.Entries, widgetAddGUIDMapEntry{
			Key:  keyRef,
			GUID: guid,
		})
	}
	if reader.Offset() < end-start {
		state.Trailing = append([]byte(nil), asset.Raw.Bytes[start+reader.Offset():end]...)
	}
	return state, nil
}

func encodeWidgetAddGUIDMapPropertyValue(asset *uasset.Asset, state widgetAddGUIDMapValue) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	order := packageByteOrder(asset)
	out := make([]byte, 0, 8+len(state.Removed)*8+len(state.Entries)*24+len(state.Trailing))
	if state.ReplaceMap {
		out = appendInt32Ordered(out, -1, order)
	} else {
		out = appendInt32Ordered(out, int32(len(state.Removed)), order)
		for _, removed := range state.Removed {
			out = append(out, encodeNameRef(removed, order)...)
		}
	}
	out = appendInt32Ordered(out, int32(len(state.Entries)), order)
	for _, entry := range state.Entries {
		out = append(out, encodeNameRef(entry.Key, order)...)
		out = appendWidgetAddGUIDOrdered(out, entry.GUID, order)
	}
	out = append(out, state.Trailing...)
	return out, nil
}

func rewriteWidgetAddRootPropertyTagValue(asset *uasset.Asset, prop uasset.PropertyTag, valueBytes []byte) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if prop.Offset < 0 || prop.ValueOffset < prop.Offset || prop.ValueOffset > len(asset.Raw.Bytes) {
		return nil, fmt.Errorf("PropertyGuids tag header range out of bounds")
	}
	header := append([]byte(nil), asset.Raw.Bytes[prop.Offset:prop.ValueOffset]...)
	sizePos, err := widgetAddPropertyTagSizeOffset(asset, prop)
	if err != nil {
		return nil, err
	}
	if sizePos < 0 || sizePos+4 > len(header) {
		return nil, fmt.Errorf("PropertyGuids tag size field out of bounds")
	}
	order := packageByteOrder(asset)
	order.PutUint32(header[sizePos:sizePos+4], uint32(len(valueBytes)))
	header = append(header, valueBytes...)
	return header, nil
}

func widgetAddPropertyTagSizeOffset(asset *uasset.Asset, prop uasset.PropertyTag) (int, error) {
	if asset == nil {
		return 0, fmt.Errorf("asset is nil")
	}
	if asset.Summary.FileVersionUE5 >= 1012 {
		return 8 + len(prop.TypeNodes)*(8+4), nil
	}
	return 16, nil
}

func buildWidgetAddPropertyGuidsTag(asset *uasset.Asset, valueBytes []byte) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	propertyNameRef, err := widgetAddExactNameRef(asset.Names, "PropertyGuids")
	if err != nil {
		return nil, err
	}
	if asset.Summary.FileVersionUE5 >= 1012 {
		typeNodes := []struct {
			name       string
			innerCount int32
		}{
			{name: "MapProperty", innerCount: 2},
			{name: "NameProperty", innerCount: 0},
			{name: "StructProperty", innerCount: 1},
			{name: "Guid", innerCount: 1},
			{name: "/Script/CoreUObject", innerCount: 0},
		}
		order := packageByteOrder(asset)
		out := make([]byte, 0, 8+len(typeNodes)*12+len(valueBytes)+8)
		out = append(out, encodeNameRef(propertyNameRef, order)...)
		for _, node := range typeNodes {
			ref, err := widgetAddExactNameRef(asset.Names, node.name)
			if err != nil {
				return nil, err
			}
			out = append(out, encodeNameRef(ref, order)...)
			out = appendInt32Ordered(out, node.innerCount, order)
		}
		out = appendInt32Ordered(out, int32(len(valueBytes)), order)
		out = append(out, 8) // fixture-backed map-property flags on WidgetBlueprintGeneratedClass
		out = append(out, valueBytes...)
		return out, nil
	}

	order := packageByteOrder(asset)
	mapPropertyRef, err := widgetAddExactNameRef(asset.Names, "MapProperty")
	if err != nil {
		return nil, err
	}
	namePropertyRef, err := widgetAddExactNameRef(asset.Names, "NameProperty")
	if err != nil {
		return nil, err
	}
	structPropertyRef, err := widgetAddExactNameRef(asset.Names, "StructProperty")
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, 48+len(valueBytes))
	out = append(out, encodeNameRef(propertyNameRef, order)...)
	out = append(out, encodeNameRef(mapPropertyRef, order)...)
	out = appendInt32Ordered(out, int32(len(valueBytes)), order)
	out = appendInt32Ordered(out, 0, order)
	out = append(out, encodeNameRef(namePropertyRef, order)...)
	out = append(out, encodeNameRef(structPropertyRef, order)...)
	out = append(out, 0) // has property GUID
	if asset.Summary.FileVersionUE5 >= 1011 {
		out = append(out, 0) // property extensions
	}
	out = append(out, valueBytes...)
	return out, nil
}

func appendWidgetAddGUIDOrdered(dst []byte, guid uasset.GUID, order binary.ByteOrder) []byte {
	buf := make([]byte, 16)
	order.PutUint32(buf[0:4], binary.LittleEndian.Uint32(guid[0:4]))
	order.PutUint32(buf[4:8], binary.LittleEndian.Uint32(guid[4:8]))
	order.PutUint32(buf[8:12], binary.LittleEndian.Uint32(guid[8:12]))
	order.PutUint32(buf[12:16], binary.LittleEndian.Uint32(guid[12:16]))
	return append(dst, buf...)
}

func parseWidgetAddGUIDString(raw string) (uasset.GUID, error) {
	var out uasset.GUID
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, "-", "")
	if len(raw) != 32 {
		return out, fmt.Errorf("guid length mismatch")
	}
	decoded, err := hex.DecodeString(raw)
	if err != nil {
		return out, err
	}
	if len(decoded) != len(out) {
		return out, fmt.Errorf("guid length mismatch")
	}
	out[0] = decoded[3]
	out[1] = decoded[2]
	out[2] = decoded[1]
	out[3] = decoded[0]
	out[4] = decoded[5]
	out[5] = decoded[4]
	out[6] = decoded[7]
	out[7] = decoded[6]
	copy(out[8:], decoded[8:])
	return out, nil
}

type generatedClassWidgetVariableFieldLayout struct {
	ScriptEnd  int
	RecordsEnd int
	FieldCount int
}

func captureGeneratedClassWidgetVariableFieldLayout(asset *uasset.Asset, generatedClassExport int) (generatedClassWidgetVariableFieldLayout, error) {
	layout := generatedClassWidgetVariableFieldLayout{}
	if asset == nil {
		return layout, fmt.Errorf("asset is nil")
	}
	if generatedClassExport < 0 || generatedClassExport >= len(asset.Exports) {
		return layout, fmt.Errorf("generated class export index out of range: %d", generatedClassExport+1)
	}
	exp := asset.Exports[generatedClassExport]
	serialStart := int(exp.SerialOffset)
	serialEnd := int(exp.SerialOffset + exp.SerialSize)
	if serialStart < 0 || serialEnd < serialStart || serialEnd > len(asset.Raw.Bytes) {
		return layout, fmt.Errorf("generated class export serial range out of bounds")
	}
	scriptEnd := int(exp.ScriptSerializationEndOffset)
	if scriptEnd < 0 || scriptEnd > serialEnd-serialStart {
		return layout, fmt.Errorf("generated class script serialization range is invalid")
	}
	payload := asset.Raw.Bytes[serialStart:serialEnd]
	tail := append([]byte(nil), payload[scriptEnd:]...)
	if len(tail) < 16 {
		return layout, fmt.Errorf("generated class trailer is shorter than expected")
	}
	order := packageByteOrder(asset)
	fieldCount := int(order.Uint32(tail[12:16]))
	recordsEnd := 16
	for i := 0; i < fieldCount; i++ {
		recordLen, parseErr := generatedClassWidgetVariableFieldRecordLength(tail[recordsEnd:], order)
		if parseErr != nil {
			return layout, fmt.Errorf("parse generated class field record %d: %w", i, parseErr)
		}
		recordsEnd += recordLen
	}
	layout.ScriptEnd = scriptEnd
	layout.RecordsEnd = recordsEnd
	layout.FieldCount = fieldCount
	return layout, nil
}

func appendGeneratedClassWidgetVariableFieldWithLayout(asset *uasset.Asset, generatedClassExport int, blueprintObjectName string, childWidgetClass string, childClassImportIndex int, childName widgetAddName, overlayChainMode bool, prependEntry bool, layout generatedClassWidgetVariableFieldLayout) ([]byte, *uasset.Asset, error) {
	_ = childWidgetClass
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if generatedClassExport < 0 || generatedClassExport >= len(asset.Exports) {
		return nil, nil, fmt.Errorf("generated class export index out of range: %d", generatedClassExport+1)
	}
	exp := asset.Exports[generatedClassExport]
	serialStart := int(exp.SerialOffset)
	serialEnd := int(exp.SerialOffset + exp.SerialSize)
	if serialStart < 0 || serialEnd < serialStart || serialEnd > len(asset.Raw.Bytes) {
		return nil, nil, fmt.Errorf("generated class export serial range out of bounds")
	}
	scriptEnd := layout.ScriptEnd
	if scriptEnd < 0 || scriptEnd > serialEnd-serialStart {
		return nil, nil, fmt.Errorf("generated class script serialization range is invalid")
	}
	payload := append([]byte(nil), asset.Raw.Bytes[serialStart:serialEnd]...)
	tail := append([]byte(nil), payload[scriptEnd:]...)
	if len(tail) < layout.RecordsEnd {
		return nil, nil, fmt.Errorf("generated class trailer is shorter than expected")
	}
	fieldRecord, err := buildGeneratedClassWidgetVariableFieldRecord(asset, blueprintObjectName, childClassImportIndex, childName)
	if err != nil {
		return nil, nil, err
	}
	newTail := append([]byte{}, tail[:12]...)
	order := packageByteOrder(asset)
	newTail = appendInt32Ordered(newTail, int32(layout.FieldCount+1), order)
	if overlayChainMode || prependEntry {
		newTail = append(newTail, fieldRecord...)
		newTail = append(newTail, tail[16:layout.RecordsEnd]...)
	} else {
		newTail = append(newTail, tail[16:layout.RecordsEnd]...)
		newTail = append(newTail, fieldRecord...)
	}
	newTail = append(newTail, tail[layout.RecordsEnd:]...)
	newPayload := append([]byte{}, payload[:scriptEnd]...)
	newPayload = append(newPayload, newTail...)
	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{{
		ExportIndex: generatedClassExport,
		Payload:     newPayload,
	}})
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite generated class export: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(outBytes, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, nil, fmt.Errorf("reparse generated class export rewrite: %w", err)
	}
	return outBytes, updatedAsset, nil
}

func generatedClassWidgetVariableFieldRecordLength(raw []byte, order binary.ByteOrder) (int, error) {
	const fixedPrefix = 8 + 8 + 8 + 4
	if len(raw) < fixedPrefix {
		return 0, fmt.Errorf("record too short")
	}
	offset := 8 + 8 + 8
	metaCount := int(order.Uint32(raw[offset : offset+4]))
	offset += 4
	for i := 0; i < metaCount; i++ {
		if len(raw[offset:]) < 8 {
			return 0, fmt.Errorf("metadata %d key out of bounds", i)
		}
		offset += 8
		strLen, consumed, err := generatedClassFieldFStringLength(raw[offset:], order)
		if err != nil {
			return 0, fmt.Errorf("metadata %d string: %w", i, err)
		}
		offset += consumed
		_ = strLen
	}
	const fixedSuffix = 4 + 4 + 8 + 2 + 8 + 1 + 4
	if len(raw[offset:]) < fixedSuffix {
		return 0, fmt.Errorf("record suffix out of bounds")
	}
	offset += fixedSuffix
	return offset, nil
}

func generatedClassFieldFStringLength(raw []byte, order binary.ByteOrder) (int32, int, error) {
	if len(raw) < 4 {
		return 0, 0, fmt.Errorf("missing length prefix")
	}
	n := int32(order.Uint32(raw[:4]))
	if n == 0 {
		return 0, 4, nil
	}
	if n > 0 {
		byteLen := int(n)
		if len(raw) < 4+byteLen {
			return 0, 0, fmt.Errorf("ascii payload out of bounds")
		}
		return n, 4 + byteLen, nil
	}
	byteLen := int(-n) * 2
	if len(raw) < 4+byteLen {
		return 0, 0, fmt.Errorf("wide payload out of bounds")
	}
	return n, 4 + byteLen, nil
}

func findExportIndexByObjectName(asset *uasset.Asset, objectName string) (int, bool) {
	if asset == nil {
		return 0, false
	}
	for i, exp := range asset.Exports {
		if strings.EqualFold(exp.ObjectName.Display(asset.Names), objectName) {
			return i, true
		}
	}
	return 0, false
}

func buildGeneratedClassWidgetVariableFieldRecord(asset *uasset.Asset, blueprintObjectName string, childClassImportIndex int, childName widgetAddName) ([]byte, error) {
	order := packageByteOrder(asset)
	fieldTypeRef, err := widgetAddExactNameRef(asset.Names, "ObjectProperty")
	if err != nil {
		return nil, err
	}
	fieldNameRef, err := resolveDisplayNameRef(asset.Names, childName.Display)
	if err != nil {
		return nil, err
	}
	editInlineRef, err := widgetAddExactNameRef(asset.Names, "EditInline")
	if err != nil {
		return nil, err
	}
	displayNameRef, err := widgetAddExactNameRef(asset.Names, "DisplayName")
	if err != nil {
		return nil, err
	}
	categoryRef, err := widgetAddExactNameRef(asset.Names, "Category")
	if err != nil {
		return nil, err
	}
	noneRef, err := widgetAddExactNameRef(asset.Names, "None")
	if err != nil {
		return nil, err
	}
	if childClassImportIndex <= 0 {
		return nil, fmt.Errorf("child class import index is invalid: %d", childClassImportIndex)
	}

	// These bytes are fixture-derived from the UE-generated widget-add output for
	// one generated ObjectProperty field on WidgetBlueprintGeneratedClass.
	const (
		fieldObjectFlags   = int64(0x0000000100200001)
		fieldPropertyFlags = int64(0x000200008009001c)
	)

	out := make([]byte, 0, 128)
	out = append(out, encodeNameRef(fieldTypeRef, order)...)
	out = append(out, encodeNameRef(fieldNameRef, order)...)
	out = appendInt64Ordered(out, fieldObjectFlags, order)
	out = appendInt32Ordered(out, 3, order)
	out = append(out, encodeNameRef(editInlineRef, order)...)
	out = appendFStringOrdered(out, "true", order)
	out = append(out, encodeNameRef(displayNameRef, order)...)
	out = appendFStringOrdered(out, childName.Display, order)
	out = append(out, encodeNameRef(categoryRef, order)...)
	out = appendFStringOrdered(out, blueprintObjectName, order)
	out = appendInt32Ordered(out, 1, order)
	out = appendInt32Ordered(out, 8, order)
	out = appendInt64Ordered(out, fieldPropertyFlags, order)
	out = append(out, 0, 0) // RepIndex uint16
	out = append(out, encodeNameRef(noneRef, order)...)
	out = append(out, 0) // ELifetimeCondition::COND_None
	out = appendInt32Ordered(out, int32(-childClassImportIndex), order)
	return out, nil
}

func normalizeWidgetAddGeneratedClassFStringLengths(asset *uasset.Asset, generatedClassExport int, blueprintObjectName string, scriptEndHint int) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if generatedClassExport < 0 || generatedClassExport >= len(asset.Exports) {
		return nil, nil, fmt.Errorf("generated class export index out of range: %d", generatedClassExport+1)
	}
	exp := asset.Exports[generatedClassExport]
	serialStart := int(exp.SerialOffset)
	serialEnd := int(exp.SerialOffset + exp.SerialSize)
	if serialStart < 0 || serialEnd < serialStart || serialEnd > len(asset.Raw.Bytes) {
		return nil, nil, fmt.Errorf("generated class export serial range out of bounds")
	}
	scriptEnd := scriptEndHint
	if scriptEnd <= 0 {
		scriptEnd = int(exp.ScriptSerializationEndOffset)
	}
	if scriptEnd < 0 || scriptEnd > serialEnd-serialStart {
		return nil, nil, fmt.Errorf("generated class script serialization range is invalid")
	}
	payload := append([]byte(nil), asset.Raw.Bytes[serialStart:serialEnd]...)
	tail := payload[scriptEnd:]
	if len(tail) < 16 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	candidates, err := generatedClassWidgetVariableFieldStringCandidates(asset, generatedClassExport, blueprintObjectName)
	if err != nil {
		return nil, nil, err
	}
	if len(candidates) == 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	order := packageByteOrder(asset)
	changed := false
	for _, candidate := range candidates {
		if normalizeWidgetAddASCIIFStringLengthInPlace(payload, scriptEnd, candidate, order) {
			changed = true
		}
	}
	if !changed {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{{
		ExportIndex: generatedClassExport,
		Payload:     payload,
	}})
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite generated class FString normalization: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(outBytes, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, nil, fmt.Errorf("reparse generated class FString normalization: %w", err)
	}
	return outBytes, updatedAsset, nil
}

func generatedClassWidgetVariableFieldStringCandidates(asset *uasset.Asset, generatedClassExport int, blueprintObjectName string) ([]string, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	candidates := make([]string, 0, 8)
	seen := map[string]bool{}
	appendCandidate := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			return
		}
		seen[key] = true
		candidates = append(candidates, trimmed)
	}
	appendCandidate("true")
	appendCandidate(blueprintObjectName)

	blueprintExport, ok := findExportIndexByObjectName(asset, blueprintObjectName)
	if !ok {
		return candidates, nil
	}
	current, err := decodeExportRootPropertyValue(asset, blueprintExport, "GeneratedVariables")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "property not found") {
			return candidates, nil
		}
		return nil, err
	}
	decodedArray, ok := current.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("GeneratedVariables is not a decoded array property")
	}
	items, err := anySliceLocal(decodedArray["value"])
	if err != nil {
		return nil, fmt.Errorf("GeneratedVariables value is not an entry list")
	}
	for _, item := range items {
		wrapped, ok := item.(map[string]any)
		if !ok {
			continue
		}
		value, ok := wrapped["value"].(map[string]any)
		if !ok {
			continue
		}
		fields, ok := value["value"].(map[string]any)
		if !ok {
			continue
		}
		varName, ok := fields["VarName"].(map[string]any)
		if !ok {
			continue
		}
		nameValue, ok := varName["value"].(map[string]any)
		if !ok {
			continue
		}
		if name, ok := nameValue["name"].(string); ok {
			appendCandidate(name)
		}
	}
	return candidates, nil
}

func normalizeWidgetAddASCIIFStringLengthInPlace(payload []byte, searchStart int, value string, order binary.ByteOrder) bool {
	if len(payload) == 0 || searchStart < 0 || searchStart >= len(payload) {
		return false
	}
	if strings.TrimSpace(value) == "" {
		return false
	}
	needle := append([]byte(value), 0)
	expectedLen := uint32(len(value) + 1)
	changed := false
	region := payload[searchStart:]
	searchFrom := 0
	for {
		rel := bytes.Index(region[searchFrom:], needle)
		if rel < 0 {
			break
		}
		abs := searchStart + searchFrom + rel
		if abs >= 4 {
			current := order.Uint32(payload[abs-4 : abs])
			if current != expectedLen && current > 0 && current <= 256 {
				order.PutUint32(payload[abs-4:abs], expectedLen)
				changed = true
			}
		}
		searchFrom += rel + len(needle)
	}
	return changed
}

func widgetAddExactNameRef(names []uasset.NameEntry, value string) (uasset.NameRef, error) {
	idx := findNameIndex(names, value)
	if idx < 0 {
		return uasset.NameRef{}, fmt.Errorf("name %q is not present in NameMap", value)
	}
	return uasset.NameRef{Index: idx, Number: 0}, nil
}

func appendWidgetAddGeneratedVariable(asset *uasset.Asset, blueprintExport int, blueprintObjectName string, panelClass string, childWidgetClass string, childClassImportIndex int, childName widgetAddName, guid string, parentHadChildren bool, overlayChainMode bool, prependEntry bool) ([]byte, *uasset.Asset, error) {
	value := []any{
		map[string]any{
			"structType": "BPVariableDescription",
			"value":      widgetAddGeneratedVariableFields(asset, blueprintExport, blueprintObjectName, panelClass, childWidgetClass, childClassImportIndex, childName, guid, parentHadChildren, overlayChainMode),
		},
	}
	current, err := decodeExportRootPropertyValue(asset, blueprintExport, "GeneratedVariables")
	if err == nil {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("GeneratedVariables is not a decoded array property")
		}
		existing, listErr := anySliceLocal(m["value"])
		if listErr != nil {
			return nil, nil, fmt.Errorf("GeneratedVariables value is not an entry list")
		}
		unwrapped := make([]any, 0, len(existing)+len(value))
		for _, item := range existing {
			if wrapped, ok := item.(map[string]any); ok {
				if inner, exists := wrapped["value"]; exists {
					if innerMap, ok := inner.(map[string]any); ok {
						unwrapped = append(unwrapped, cloneAnyMapLocal(innerMap))
					} else {
						unwrapped = append(unwrapped, inner)
					}
					continue
				}
			}
			if itemMap, ok := item.(map[string]any); ok {
				unwrapped = append(unwrapped, cloneAnyMapLocal(itemMap))
			} else {
				unwrapped = append(unwrapped, item)
			}
		}
		if overlayChainMode || prependEntry {
			value = append(value, unwrapped...)
		} else {
			value = append(unwrapped, value...)
		}
	} else if !strings.Contains(strings.ToLower(err.Error()), "property not found") {
		return nil, nil, err
	}
	return applyWidgetAddPropertyWrite(asset, blueprintExport, "GeneratedVariables", "ArrayProperty(StructProperty(BPVariableDescription(/Script/Engine)))", value, "CategorySorting")
}

func appendWidgetAddCategorySortingEntry(asset *uasset.Asset, blueprintExport int, blueprintObjectName string) ([]byte, *uasset.Asset, error) {
	categoryName := widgetAddBlueprintCategoryName(blueprintObjectName)
	if categoryName == "" {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	current, err := decodeExportRootPropertyValue(asset, blueprintExport, "CategorySorting")
	if err != nil {
		return nil, nil, err
	}
	m, ok := current.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("CategorySorting is not a decoded array property")
	}
	items, ok := m["value"].([]any)
	if !ok {
		return nil, nil, fmt.Errorf("CategorySorting value is not an array")
	}
	values := make([]any, 0, len(items)+1)
	for _, item := range items {
		wrapped, ok := item.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("CategorySorting entry is not an object")
		}
		value, ok := wrapped["value"].(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("CategorySorting entry value is not an object")
		}
		if strings.EqualFold(strings.TrimSpace(fmt.Sprint(value["name"])), categoryName) {
			return append([]byte(nil), asset.Raw.Bytes...), asset, nil
		}
		values = append(values, cloneAnyMapLocal(value))
	}
	nameRef, err := resolveDisplayNameRef(asset.Names, categoryName)
	if err != nil {
		return nil, nil, err
	}
	values = append(values, map[string]any{
		"index":  nameRef.Index,
		"name":   categoryName,
		"number": nameRef.Number,
	})
	return applyWidgetAddPropertyWrite(asset, blueprintExport, "CategorySorting", "ArrayProperty(NameProperty)", values, "")
}

func widgetAddGeneratedVariableFields(asset *uasset.Asset, blueprintExport int, blueprintObjectName string, panelClass string, childWidgetClass string, childClassImportIndex int, childName widgetAddName, guid string, parentHadChildren bool, overlayChainMode bool) map[string]any {
	nameRef, _ := resolveDisplayNameRef(asset.Names, childName.Display)
	defaultValueOffset := 562
	if strings.EqualFold(panelClass, "Overlay") {
		defaultValueOffset = 558
	}
	varTypeRawBase64 := widgetAddGeneratedVariableVarTypeRawBase64(asset, blueprintExport, panelClass, childWidgetClass, childClassImportIndex, parentHadChildren, overlayChainMode)
	return map[string]any{
		"VarName": map[string]any{
			"offset": 0,
			"type":   "NameProperty",
			"value": map[string]any{
				"index":  nameRef.Index,
				"name":   childName.Display,
				"number": nameRef.Number,
			},
		},
		"VarGuid": map[string]any{
			"offset": 33,
			"flags":  8,
			"type":   "StructProperty(Guid(/Script/CoreUObject))",
			"value": map[string]any{
				"structType": "Guid",
				"value":      guid,
			},
		},
		"VarType": map[string]any{
			"offset": 98,
			"flags":  8,
			"type":   "StructProperty(EdGraphPinType(/Script/Engine))",
			"value": map[string]any{
				"structType": "EdGraphPinType",
				"value": map[string]any{
					"structType": "EdGraphPinType",
					"rawBase64":  varTypeRawBase64,
				},
			},
		},
		"FriendlyName": map[string]any{
			"offset": 216,
			"type":   "StrProperty",
			"value":  childName.Display,
		},
		"Category": map[string]any{
			"offset": 254,
			"type":   "TextProperty",
			"value": map[string]any{
				"flags":                     0,
				"historyType":               "None",
				"historyTypeCode":           255,
				"hasCultureInvariantString": false,
			},
		},
		"PropertyFlags": map[string]any{
			"offset": 288,
			"type":   "UInt64Property",
			"value":  uint64(562952101494812),
		},
		"RepNotifyFunc": map[string]any{
			"offset": 321,
			"type":   "NameProperty",
			"value":  "None",
		},
		"ReplicationCondition": map[string]any{
			"offset": 354,
			"type":   "ByteProperty(ELifetimeCondition(/Script/CoreUObject))",
			"value": map[string]any{
				"enumType": "ELifetimeCondition",
				"value":    "ELifetimeCondition::COND_None",
			},
		},
		"MetaDataArray": map[string]any{
			"offset": 411,
			"type":   "ArrayProperty(StructProperty(BPVariableMetaDataEntry(/Script/Engine)))",
			"value": map[string]any{
				"value": []any{
					map[string]any{
						"type": "StructProperty",
						"value": map[string]any{
							"structType": "BPVariableMetaDataEntry",
							"value": map[string]any{
								"DataKey": map[string]any{
									"offset": 0,
									"type":   "NameProperty",
									"value":  "Category",
								},
								"DataValue": map[string]any{
									"offset": 33,
									"type":   "StrProperty",
									"value":  blueprintObjectName,
								},
							},
						},
					},
				},
			},
		},
		"DefaultValue": map[string]any{
			"offset": defaultValueOffset,
			"type":   "StrProperty",
			"value":  "",
		},
	}
}

func widgetAddGeneratedVariableVarTypeRawBase64(asset *uasset.Asset, blueprintExport int, panelClass string, childWidgetClass string, childClassImportIndex int, parentHadChildren bool, overlayChainMode bool) string {
	if rawBase64 := widgetAddExistingGeneratedVariableVarTypeRawBase64(asset, blueprintExport, childWidgetClass); rawBase64 != "" {
		return widgetAddPatchGeneratedVariableVarTypeImportIndex(rawBase64, childWidgetClass, childClassImportIndex)
	}
	return widgetAddPatchGeneratedVariableVarTypeImportIndex(widgetAddGeneratedVariableDefaultVarTypeRawBase64(panelClass, childWidgetClass, parentHadChildren, overlayChainMode), childWidgetClass, childClassImportIndex)
}

func widgetAddExistingGeneratedVariableVarTypeRawBase64(asset *uasset.Asset, blueprintExport int, childWidgetClass string) string {
	targets, err := collectWidgetWriteTargets(asset, blueprintExport+1)
	if err != nil {
		return ""
	}
	for _, target := range targets {
		if !strings.EqualFold(target.ClassName, childWidgetClass) {
			continue
		}
		rawBase64 := widgetAddGeneratedVariableVarTypeRawBase64ByObjectName(asset, blueprintExport, target.ObjectName)
		if rawBase64 != "" {
			return rawBase64
		}
	}
	return ""
}

func widgetAddGeneratedVariableVarTypeRawBase64ByObjectName(asset *uasset.Asset, blueprintExport int, objectName string) string {
	current, err := decodeExportRootPropertyValue(asset, blueprintExport, "GeneratedVariables")
	if err != nil {
		return ""
	}
	decodedArray, ok := current.(map[string]any)
	if !ok {
		return ""
	}
	items, err := anySliceLocal(decodedArray["value"])
	if err != nil {
		return ""
	}
	for _, item := range items {
		wrapped, ok := item.(map[string]any)
		if !ok {
			continue
		}
		entryValue := wrapped
		if inner, exists := wrapped["value"]; exists {
			innerMap, ok := inner.(map[string]any)
			if !ok {
				continue
			}
			entryValue = innerMap
		}
		fields, ok := entryValue["value"].(map[string]any)
		if !ok {
			continue
		}
		varNameField, ok := fields["VarName"].(map[string]any)
		if !ok {
			continue
		}
		varNameValue, ok := varNameField["value"].(map[string]any)
		if !ok || !strings.EqualFold(strings.TrimSpace(fmt.Sprint(varNameValue["name"])), objectName) {
			continue
		}
		varTypeField, ok := fields["VarType"].(map[string]any)
		if !ok {
			continue
		}
		varTypeStruct, ok := varTypeField["value"].(map[string]any)
		if !ok {
			continue
		}
		innerValue, ok := varTypeStruct["value"].(map[string]any)
		if !ok {
			continue
		}
		rawBase64, _ := innerValue["rawBase64"].(string)
		return strings.TrimSpace(rawBase64)
	}
	return ""
}

func widgetAddGeneratedVariableDefaultVarTypeRawBase64(panelClass string, childWidgetClass string, parentHadChildren bool, overlayChainMode bool) string {
	if widgetAddIsBlueprintGeneratedWidgetClass(childWidgetClass) {
		return "QgAAAAAAAABBAAAAAAAAAPn///8AAAAAAAAAAAAAAAAAQQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	if strings.EqualFold(childWidgetClass, "EditableText") {
		return "QwAAAAAAAABCAAAAAAAAAPn///8AAAAAAAAAAAAAAAAAQgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	if strings.EqualFold(childWidgetClass, "EditableTextBox") {
		return "QwAAAAAAAABCAAAAAAAAAPn///8AAAAAAAAAAAAAAAAAQgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	if strings.EqualFold(childWidgetClass, "MultiLineEditableTextBox") {
		return "QwAAAAAAAABCAAAAAAAAAPn///8AAAAAAAAAAAAAAAAAQgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	if strings.EqualFold(childWidgetClass, "NamedSlot") {
		return "RwAAAAAAAABGAAAAAAAAAPn///8AAAAAAAAAAAAAAAAARgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	if strings.EqualFold(childWidgetClass, "ProgressBar") || strings.EqualFold(childWidgetClass, "Slider") {
		return "QgAAAAAAAABBAAAAAAAAAPn///8AAAAAAAAAAAAAAAAAQQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	if strings.EqualFold(childWidgetClass, "SpinBox") {
		return "QgAAAAAAAABBAAAAAAAAAPn///8AAAAAAAAAAAAAAAAAQQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	if strings.EqualFold(childWidgetClass, "ComboBoxString") {
		return "QwAAAAAAAABCAAAAAAAAAPn///8AAAAAAAAAAAAAAAAAQgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	if strings.EqualFold(childWidgetClass, "ListView") {
		return "QwAAAAAAAABCAAAAAAAAAPn///8AAAAAAAAAAAAAAAAAQgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	if strings.EqualFold(childWidgetClass, "TileView") {
		return "QgAAAAAAAABBAAAAAAAAAPn///8AAAAAAAAAAAAAAAAAQQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	if strings.EqualFold(childWidgetClass, "TreeView") {
		return "QgAAAAAAAABBAAAAAAAAAPn///8AAAAAAAAAAAAAAAAAQQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	if strings.EqualFold(childWidgetClass, "RichTextBlock") {
		if strings.EqualFold(panelClass, "Overlay") {
			return "QgAAAAAAAABBAAAAAAAAAPn///8AAAAAAAAAAAAAAAAAQQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		}
		if parentHadChildren {
			return "RAAAAAAAAABDAAAAAAAAAPn///8AAAAAAAAAAAAAAAAAQwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		}
		return "QwAAAAAAAABCAAAAAAAAAPn///8AAAAAAAAAAAAAAAAAQgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	if widgetAddUsesTextBlockGeneratedVariableTemplate(childWidgetClass) {
		if strings.EqualFold(panelClass, "Overlay") {
			return "QgAAAAAAAABBAAAAAAAAAPv///8AAAAAAAAAAAAAAAAAQQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		}
		return "QwAAAAAAAABCAAAAAAAAAPr///8AAAAAAAAAAAAAAAAAQgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	if strings.EqualFold(panelClass, "Overlay") && overlayChainMode {
		return "RAAAAAAAAABDAAAAAAAAAPv///8AAAAAAAAAAAAAAAAAQwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	if widgetAddRequiresGeneratedChildPair(childWidgetClass) {
		return "RAAAAAAAAABDAAAAAAAAAPv///8AAAAAAAAAAAAAAAAAQwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	if strings.EqualFold(panelClass, "Overlay") {
		if parentHadChildren {
			return "QwAAAAAAAABCAAAAAAAAAPv///8AAAAAAAAAAAAAAAAAQgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		}
		return "RQAAAAAAAABEAAAAAAAAAPv///8AAAAAAAAAAAAAAAAARAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	if parentHadChildren {
		return "RAAAAAAAAABDAAAAAAAAAPn///8AAAAAAAAAAAAAAAAAQwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	return "RgAAAAAAAABFAAAAAAAAAPn///8AAAAAAAAAAAAAAAAARQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
}

func widgetAddPatchGeneratedVariableVarTypeImportIndex(rawBase64 string, childWidgetClass string, childClassImportIndex int) string {
	if !widgetAddIsBlueprintGeneratedWidgetClass(childWidgetClass) || childClassImportIndex <= 0 {
		return rawBase64
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(rawBase64))
	if err != nil || len(decoded) < 20 {
		return rawBase64
	}
	binary.LittleEndian.PutUint32(decoded[16:20], uint32(int32(-childClassImportIndex)))
	return base64.StdEncoding.EncodeToString(decoded)
}

func widgetAddIsBlueprintGeneratedWidgetClass(className string) bool {
	trimmed := strings.TrimSpace(className)
	return strings.HasSuffix(trimmed, "_C")
}

func widgetAddUsesTextBlockGeneratedVariableTemplate(childWidgetClass string) bool {
	return strings.EqualFold(childWidgetClass, "TextBlock") || strings.EqualFold(childWidgetClass, "RichTextBlock")
}

func reorderWidgetAddEmptyCanvasPanelGeneratedChildExports(asset *uasset.Asset, ctx widgetAddContext, childName widgetAddName) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}

	childPath := ctx.Target.Path + "/" + childName.Display
	designerChildExport, generatedChildExport := findWidgetAddChildExportsByPath(asset, ctx.Target.BlueprintExport, childPath)
	designerSlotExport, generatedSlotExport := findWidgetAddSlotExportsByPath(asset, ctx.Target.BlueprintExport, childPath)
	if designerChildExport < 0 || generatedChildExport < 0 || designerSlotExport < 0 || generatedSlotExport < 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	prioritized := []int{
		designerChildExport,
		generatedChildExport,
		ctx.DesignerParentExport,
		ctx.GeneratedParentExport,
		designerSlotExport,
		generatedSlotExport,
	}
	order := make([]int, 0, len(asset.Exports))
	seen := make(map[int]bool, len(prioritized))
	for _, idx := range prioritized {
		if idx < 0 || idx >= len(asset.Exports) || seen[idx] {
			continue
		}
		seen[idx] = true
		order = append(order, idx)
	}
	for i := range asset.Exports {
		if seen[i] {
			continue
		}
		order = append(order, i)
	}
	if len(order) != len(asset.Exports) {
		return nil, nil, fmt.Errorf("empty CanvasPanel generated-child export reorder produced %d entries for %d exports", len(order), len(asset.Exports))
	}

	outBytes, err := edit.ReorderExports(asset, order)
	if err != nil {
		return nil, nil, err
	}
	updatedAsset, err := uasset.ParseBytes(outBytes, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, nil, fmt.Errorf("reparse empty CanvasPanel generated-child export reorder: %w", err)
	}
	return outBytes, updatedAsset, nil
}

func reorderWidgetAddEmptyCanvasPanelRichTextExports(asset *uasset.Asset, ctx widgetAddContext, childName widgetAddName) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}

	childPath := ctx.Target.Path + "/" + childName.Display
	designerChildExport, generatedChildExport := findWidgetAddChildExportsByPath(asset, ctx.Target.BlueprintExport, childPath)
	if designerChildExport < 0 || generatedChildExport < 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	insertAfter := -1
	for i, exp := range asset.Exports {
		if strings.EqualFold(asset.ResolveClassName(exp), "K2Node_Event") {
			insertAfter = i
		}
	}
	if insertAfter < 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	baseOrder := make([]int, 0, len(asset.Exports)-2)
	for i := range asset.Exports {
		if i == designerChildExport || i == generatedChildExport {
			continue
		}
		baseOrder = append(baseOrder, i)
	}

	baseInsertPos := len(baseOrder)
	for i, exportIndex := range baseOrder {
		if exportIndex == insertAfter {
			baseInsertPos = i + 1
			break
		}
	}

	order := make([]int, 0, len(asset.Exports))
	order = append(order, baseOrder[:baseInsertPos]...)
	order = append(order, designerChildExport, generatedChildExport)
	order = append(order, baseOrder[baseInsertPos:]...)
	if len(order) != len(asset.Exports) {
		return nil, nil, fmt.Errorf("empty CanvasPanel RichText export reorder produced %d entries for %d exports", len(order), len(asset.Exports))
	}

	outBytes, err := edit.ReorderExports(asset, order)
	if err != nil {
		return nil, nil, err
	}
	updatedAsset, err := uasset.ParseBytes(outBytes, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, nil, fmt.Errorf("reparse empty CanvasPanel RichText export reorder: %w", err)
	}
	return outBytes, updatedAsset, nil
}

func validateWidgetAddResult(asset *uasset.Asset, blueprintExport int, parentPath, childName string, expectGeneratedChild bool) error {
	shape, err := captureWidgetBlueprintShape(asset, blueprintExport)
	if err != nil {
		return err
	}
	wantPath := parentPath + "/" + childName
	designerPaths, ok := shape["designer"]
	if !ok {
		return fmt.Errorf("tree role %q missing after widget-add", "designer")
	}
	if !containsStringFold(designerPaths, wantPath) {
		return fmt.Errorf("tree role %q missing new child path %q", "designer", wantPath)
	}
	generatedPaths, ok := shape["generated"]
	if !ok {
		return fmt.Errorf("tree role %q missing after widget-add", "generated")
	}
	hasGeneratedChild := widgetBlueprintShapeContainsPathOrSuffix(generatedPaths, wantPath)
	if expectGeneratedChild && !hasGeneratedChild {
		return fmt.Errorf("tree role %q missing generated child path %q", "generated", wantPath)
	}
	if !expectGeneratedChild && hasGeneratedChild {
		return fmt.Errorf("tree role %q unexpectedly contains child path %q", "generated", wantPath)
	}
	return nil
}

func widgetBlueprintShapeContainsPathOrSuffix(paths []string, wantPath string) bool {
	if containsStringFold(paths, wantPath) {
		return true
	}
	for _, path := range paths {
		if widgetPathHasSegmentSuffix(wantPath, path) {
			return true
		}
	}
	return false
}

func validateWidgetAddRootResult(asset *uasset.Asset, blueprintExport int, rootName string) error {
	shape, err := captureWidgetBlueprintShape(asset, blueprintExport)
	if err != nil {
		return err
	}
	for _, role := range []string{"designer", "generated"} {
		paths, ok := shape[role]
		if !ok {
			return fmt.Errorf("tree role %q missing after widget-add", role)
		}
		if len(paths) != 1 || !strings.EqualFold(paths[0], rootName) {
			return fmt.Errorf("tree role %q missing new root path %q", role, rootName)
		}
	}
	return nil
}

func containsStringFold(items []string, needle string) bool {
	for _, item := range items {
		if strings.EqualFold(item, needle) {
			return true
		}
	}
	return false
}

func anySliceLocal(v any) ([]any, error) {
	if items, ok := v.([]any); ok {
		return items, nil
	}
	if items, ok := v.([]map[string]any); ok {
		out := make([]any, 0, len(items))
		for _, item := range items {
			out = append(out, item)
		}
		return out, nil
	}
	return nil, fmt.Errorf("value is %T, want []any", v)
}
