package cli

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/wilddogjp/openbpx/pkg/edit"
	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func runBlueprintWidgetRemove(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("blueprint widget-remove", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "optional 1-based WidgetBlueprint export index")
	widgetSelector := fs.String("widget", "", "widget path or object name")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || *exportIndex < 0 || strings.TrimSpace(*widgetSelector) == "" {
		fmt.Fprintln(stderr, "usage: bpx blueprint widget-remove <file.uasset> --widget <path|name> [--export <n>] [--dry-run] [--backup]")
		return 1
	}

	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	targets, err := collectWidgetWriteTargets(asset, *exportIndex)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	target, err := selectWidgetWriteTarget(targets, *widgetSelector)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := validateWidgetWriteTarget(*target); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := validateWidgetRemoveTarget(targets, *target); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	parentPath := widgetRemoveParentPath(target.Path)
	parent, err := selectWidgetWriteTarget(targets, parentPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: resolve parent widget %q: %v\n", parentPath, err)
		return 1
	}
	ctx, err := resolveWidgetAddContext(asset, *parent)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if ctx.GeneratedParentExport < 0 {
		fmt.Fprintf(stderr, "error: widget-remove currently requires a generated-tree parent companion for %s\n", parent.Path)
		return 1
	}

	beforeShape, err := captureWidgetBlueprintShape(asset, target.BlueprintExport)
	if err != nil {
		fmt.Fprintf(stderr, "error: capture widget-remove shape: %v\n", err)
		return 1
	}
	compileArtifactsSnapshot := captureWidgetCompileArtifactsSnapshot(asset, ctx.BlueprintObjectName)

	workingAsset := asset
	var workingBytes []byte
	slotRemoveSet := widgetRemoveIndexSet(target.SlotExports)
	widgetRemoveSet := widgetRemoveIndexSet(target.Exports)

	_, workingAsset, err = widgetRemoveRewriteParentSlots(workingAsset, ctx, slotRemoveSet)
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite parent Slots: %v\n", err)
		return 1
	}
	_, workingAsset, err = widgetRemoveRewriteAllWidgets(workingAsset, ctx, widgetRemoveSet)
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite WidgetTree AllWidgets: %v\n", err)
		return 1
	}
	_, workingAsset, err = widgetRemoveRewriteGuidMap(workingAsset, target.BlueprintExport, target.ObjectName)
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite WidgetVariableNameToGuidMap: %v\n", err)
		return 1
	}
	remainingGeneratedVariables, _, nextAsset, err := widgetRemoveRewriteGeneratedVariables(workingAsset, target.BlueprintExport, target.ObjectName)
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite GeneratedVariables: %v\n", err)
		return 1
	}
	workingAsset = nextAsset
	_, workingAsset, err = widgetRemoveRewriteCategorySorting(workingAsset, target.BlueprintExport, ctx.BlueprintObjectName, remainingGeneratedVariables)
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite CategorySorting: %v\n", err)
		return 1
	}
	if ctx.GeneratedClassExport >= 0 {
		_, workingAsset, err = widgetRemoveRewriteGeneratedClassPropertyGuids(workingAsset, ctx.GeneratedClassExport, target.ObjectName)
		if err != nil {
			fmt.Fprintf(stderr, "error: rewrite generated class PropertyGuids: %v\n", err)
			return 1
		}
		_, workingAsset, err = widgetRemoveRewriteGeneratedClassFieldRecords(workingAsset, ctx.GeneratedClassExport, target.ObjectName)
		if err != nil {
			fmt.Fprintf(stderr, "error: rewrite generated class field records: %v\n", err)
			return 1
		}
	}

	_, workingAsset, err = restoreWidgetBlueprintCompileArtifacts(workingAsset, *opts, ctx.BlueprintObjectName, compileArtifactsSnapshot)
	if err != nil {
		fmt.Fprintf(stderr, "error: restore widget-remove compile artifacts: %v\n", err)
		return 1
	}
	_, workingAsset, err = finalizeWidgetBlueprintMutation(asset, workingAsset, *opts, ctx.BlueprintObjectName)
	if err != nil {
		fmt.Fprintf(stderr, "error: finalize widget-remove: %v\n", err)
		return 1
	}
	_, workingAsset, err = widgetRemoveEnsureTargetDetached(
		workingAsset,
		*opts,
		target.Path,
		target.ObjectName,
		ctx.BlueprintObjectName,
		compileArtifactsSnapshot,
	)
	if err != nil {
		fmt.Fprintf(stderr, "error: ensure widget-remove detach: %v\n", err)
		return 1
	}
	compaction, _, workingAsset, err := widgetRemoveCompactOrphans(
		workingAsset,
		*opts,
		append(append([]int(nil), target.SlotExports...), target.Exports...),
		target.ObjectName,
		target.ClassName,
	)
	if err != nil {
		fmt.Fprintf(stderr, "error: compact widget-remove orphans: %v\n", err)
		return 1
	}
	workingBytes, workingAsset, extraRemovedNames, err := widgetRemoveNormalizeTextResidue(workingAsset, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: normalize widget-remove text residue: %v\n", err)
		return 1
	}
	compaction.removedNameCount += extraRemovedNames
	updatedTargets, err := collectWidgetWriteTargets(workingAsset, 0)
	if err != nil {
		fmt.Fprintf(stderr, "error: collect compacted widget targets: %v\n", err)
		return 1
	}
	if lingering, err := selectWidgetWriteTarget(updatedTargets, target.Path); err == nil && lingering != nil {
		fmt.Fprintf(stderr, "error: widget-remove target still present after rewrite: %s\n", target.Path)
		return 1
	}
	updatedParent, err := selectWidgetWriteTarget(updatedTargets, parent.Path)
	if err != nil {
		fmt.Fprintf(stderr, "error: resolve compacted parent widget %q: %v\n", parent.Path, err)
		return 1
	}

	afterShape, err := captureWidgetBlueprintShape(workingAsset, updatedParent.BlueprintExport)
	if err != nil {
		fmt.Fprintf(stderr, "error: capture rewritten widget-remove shape: %v\n", err)
		return 1
	}
	if err := validateWidgetRemoveResult(beforeShape, afterShape, target.Path); err != nil {
		fmt.Fprintf(stderr, "error: widget-remove structural validation: %v\n", err)
		return 1
	}

	resp := map[string]any{
		"file":                    file,
		"widget":                  *widgetSelector,
		"resolvedPath":            target.Path,
		"objectName":              target.ObjectName,
		"className":               target.ClassName,
		"parentPath":              parent.Path,
		"targetExports":           widgetWriteExportIndicesOneBased(target.Exports),
		"slotExports":             widgetWriteExportIndicesOneBased(target.SlotExports),
		"removedExportCount":      compaction.removedExportCount,
		"removedImportCount":      compaction.removedImportCount,
		"removedNameCount":        compaction.removedNameCount,
		"remainingGeneratedVars":  remainingGeneratedVariables,
		"dryRun":                  *dryRun,
		"changed":                 !bytes.Equal(asset.Raw.Bytes, workingBytes),
		"compactedExportEntries":  compaction.removedExportCount > 0,
		"logicalTreeChildRemoved": true,
		"outputBytes":             len(workingBytes),
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

func validateWidgetRemoveTarget(targets []widgetWriteTarget, target widgetWriteTarget) error {
	if strings.TrimSpace(target.Path) == "" {
		return fmt.Errorf("widget path is empty")
	}
	if !strings.Contains(target.Path, "/") {
		return fmt.Errorf("widget-remove currently supports non-root child widgets only")
	}
	if len(target.SlotExports) == 0 {
		return fmt.Errorf("widget-remove currently supports slot-backed child widgets only")
	}
	for _, candidate := range targets {
		if strings.EqualFold(candidate.Path, target.Path) {
			continue
		}
		if strings.HasPrefix(strings.ToLower(candidate.Path), strings.ToLower(target.Path)+"/") {
			return fmt.Errorf("widget %q is not a leaf widget; nested descendants are not supported yet", target.Path)
		}
	}
	return nil
}

func widgetRemoveParentPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if idx := strings.LastIndex(trimmed, "/"); idx > 0 {
		return trimmed[:idx]
	}
	return ""
}

func widgetRemoveIndexSet(indices []int) map[int]bool {
	out := make(map[int]bool, len(indices))
	for _, idx := range indices {
		if idx < 0 {
			continue
		}
		out[idx] = true
	}
	return out
}

func widgetRemoveRewriteParentSlots(asset *uasset.Asset, ctx widgetAddContext, removeSet map[int]bool) ([]byte, *uasset.Asset, error) {
	_, workingAsset, err := widgetRemoveRewriteObjectRefArrayProperty(asset, ctx.DesignerParentExport, "Slots", removeSet, widgetAddParentSlotsBeforeProperty(asset, ctx.DesignerParentExport))
	if err != nil {
		return nil, nil, err
	}
	return widgetRemoveRewriteObjectRefArrayProperty(workingAsset, ctx.GeneratedParentExport, "Slots", removeSet, widgetAddParentSlotsBeforeProperty(workingAsset, ctx.GeneratedParentExport))
}

func widgetRemoveRewriteAllWidgets(asset *uasset.Asset, ctx widgetAddContext, removeSet map[int]bool) ([]byte, *uasset.Asset, error) {
	_, workingAsset, err := widgetRemoveRewriteObjectRefArrayProperty(asset, ctx.DesignerTreeExport, "AllWidgets", removeSet, "")
	if err != nil {
		return nil, nil, err
	}
	return widgetRemoveRewriteObjectRefArrayProperty(workingAsset, ctx.GeneratedTreeExport, "AllWidgets", removeSet, "")
}

func widgetRemoveRewriteObjectRefArrayProperty(asset *uasset.Asset, exportIndex int, propertyName string, removeSet map[int]bool, beforeProperty string) ([]byte, *uasset.Asset, error) {
	items, err := readWidgetAddObjectRefArrayValue(asset, exportIndex, propertyName)
	if err != nil {
		if widgetRemovePropertyNotFound(err) {
			return append([]byte(nil), asset.Raw.Bytes...), asset, nil
		}
		return nil, nil, err
	}
	filtered := make([]any, 0, len(items))
	changed := false
	for _, item := range items {
		valueMap, ok := item.(map[string]any)
		if !ok {
			filtered = append(filtered, item)
			continue
		}
		rawIndex, ok := valueMap["index"]
		if !ok {
			filtered = append(filtered, cloneAnyMapLocal(valueMap))
			continue
		}
		indexValue, err := widgetRemoveAsInt64(rawIndex)
		if err != nil {
			filtered = append(filtered, cloneAnyMapLocal(valueMap))
			continue
		}
		if indexValue > 0 && removeSet[int(indexValue)-1] {
			changed = true
			continue
		}
		filtered = append(filtered, cloneAnyMapLocal(valueMap))
	}
	if !changed {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	if len(filtered) == 0 {
		return widgetRemoveRemovePropertyIfPresent(asset, exportIndex, propertyName)
	}
	return applyWidgetAddPropertyWrite(asset, exportIndex, propertyName, "ArrayProperty(ObjectProperty)", filtered, beforeProperty)
}

func widgetRemoveRewriteGuidMap(asset *uasset.Asset, blueprintExport int, objectName string) ([]byte, *uasset.Asset, error) {
	current, err := decodeExportRootPropertyValue(asset, blueprintExport, "WidgetVariableNameToGuidMap")
	if err != nil {
		if widgetRemovePropertyNotFound(err) {
			return append([]byte(nil), asset.Raw.Bytes...), asset, nil
		}
		return nil, nil, err
	}
	m, ok := current.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("WidgetVariableNameToGuidMap is not decodable as a map")
	}
	items, err := anySliceLocal(m["value"])
	if err != nil {
		return nil, nil, fmt.Errorf("WidgetVariableNameToGuidMap value is not an entry list")
	}
	filtered := make([]any, 0, len(items))
	changed := false
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("WidgetVariableNameToGuidMap entry is not an object")
		}
		keyValue, ok := entry["key"].(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("WidgetVariableNameToGuidMap entry key is not an object")
		}
		nameValue := extractWrappedValueLocal(keyValue["value"])
		if nameMap, ok := nameValue.(map[string]any); ok {
			if name, ok := nameMap["name"].(string); ok && strings.EqualFold(name, objectName) {
				changed = true
				continue
			}
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
	if !changed {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	out := cloneAnyMapLocal(m)
	out["value"] = filtered
	return applyWidgetAddPropertyWrite(asset, blueprintExport, "WidgetVariableNameToGuidMap", "MapProperty(NameProperty,StructProperty(Guid(/Script/CoreUObject)))", out, "")
}

func widgetRemoveRewriteGeneratedVariables(asset *uasset.Asset, blueprintExport int, objectName string) (int, []byte, *uasset.Asset, error) {
	current, err := decodeExportRootPropertyValue(asset, blueprintExport, "GeneratedVariables")
	if err != nil {
		if widgetRemovePropertyNotFound(err) {
			return 0, append([]byte(nil), asset.Raw.Bytes...), asset, nil
		}
		return 0, nil, nil, err
	}
	decodedArray, ok := current.(map[string]any)
	if !ok {
		return 0, nil, nil, fmt.Errorf("GeneratedVariables is not a decoded array property")
	}
	items, err := anySliceLocal(decodedArray["value"])
	if err != nil {
		return 0, nil, nil, fmt.Errorf("GeneratedVariables value is not an entry list")
	}
	filtered := make([]any, 0, len(items))
	changed := false
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return 0, nil, nil, fmt.Errorf("GeneratedVariables entry is not an object")
		}
		if strings.EqualFold(widgetRemoveGeneratedVariableEntryName(entry), objectName) {
			changed = true
			continue
		}
		if inner, ok := entry["value"].(map[string]any); ok {
			filtered = append(filtered, cloneAnyMapLocal(inner))
			continue
		}
		filtered = append(filtered, cloneAnyMapLocal(entry))
	}
	if !changed {
		return len(filtered), append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	if len(filtered) == 0 {
		outBytes, updatedAsset, err := widgetRemoveRemovePropertyIfPresent(asset, blueprintExport, "GeneratedVariables")
		return 0, outBytes, updatedAsset, err
	}
	outBytes, updatedAsset, err := applyWidgetAddPropertyWrite(asset, blueprintExport, "GeneratedVariables", "ArrayProperty(StructProperty(BPVariableDescription(/Script/Engine)))", filtered, "CategorySorting")
	if err != nil {
		return 0, nil, nil, err
	}
	return len(filtered), outBytes, updatedAsset, nil
}

func widgetRemoveGeneratedVariableEntryName(entry map[string]any) string {
	entryValue := entry
	if inner, ok := entry["value"].(map[string]any); ok {
		entryValue = inner
	}
	fields, ok := entryValue["value"].(map[string]any)
	if !ok {
		return ""
	}
	varNameField, ok := fields["VarName"].(map[string]any)
	if !ok {
		return ""
	}
	varNameValue := extractWrappedValueLocal(varNameField["value"])
	if nameMap, ok := varNameValue.(map[string]any); ok {
		if name, ok := nameMap["name"].(string); ok {
			return name
		}
	}
	if name, ok := varNameValue.(string); ok {
		return name
	}
	return ""
}

func widgetRemoveRewriteCategorySorting(asset *uasset.Asset, blueprintExport int, blueprintObjectName string, remainingGeneratedVariables int) ([]byte, *uasset.Asset, error) {
	_ = blueprintExport
	_ = blueprintObjectName
	_ = remainingGeneratedVariables
	// UE-authored remove fixtures keep CategorySorting entries even when the last
	// generated variable under that category is removed, so preserve this property.
	return append([]byte(nil), asset.Raw.Bytes...), asset, nil
}

func widgetRemoveRewriteGeneratedClassPropertyGuids(asset *uasset.Asset, exportIndex int, objectName string) ([]byte, *uasset.Asset, error) {
	parsed := asset.ParseExportProperties(exportIndex)
	existingProp, existingPropIndex := findWidgetAddRootPropertyTag(parsed.Properties, asset.Names, "PropertyGuids")
	if existingProp == nil {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	mapValue, err := parseWidgetAddGUIDMapPropertyValue(asset, *existingProp)
	if err != nil {
		return nil, nil, err
	}
	filtered := make([]widgetAddGUIDMapEntry, 0, len(mapValue.Entries))
	changed := false
	for _, entry := range mapValue.Entries {
		if strings.EqualFold(entry.Key.Display(asset.Names), objectName) {
			changed = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if !changed {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	if len(filtered) == 0 {
		return widgetRemoveRemovePropertyIfPresent(asset, exportIndex, "PropertyGuids")
	}
	mapValue.Entries = filtered
	valueBytes, err := encodeWidgetAddGUIDMapPropertyValue(asset, mapValue)
	if err != nil {
		return nil, nil, err
	}
	newTagBytes, err := rewriteWidgetAddRootPropertyTagValue(asset, *existingProp, valueBytes)
	if err != nil {
		return nil, nil, err
	}
	exp := asset.Exports[exportIndex]
	serialStart := int(exp.SerialOffset)
	serialEnd := int(exp.SerialOffset + exp.SerialSize)
	payload := append([]byte(nil), asset.Raw.Bytes[serialStart:serialEnd]...)
	propEnd := widgetAddRootPropertyEndOffset(parsed, existingPropIndex)
	startRel := existingProp.Offset - serialStart
	endRel := propEnd - serialStart
	if startRel < 0 || endRel < startRel || endRel > len(payload) {
		return nil, nil, fmt.Errorf("PropertyGuids replacement range out of bounds")
	}
	newPayload := make([]byte, 0, len(payload)-((endRel-startRel)-len(newTagBytes)))
	newPayload = append(newPayload, payload[:startRel]...)
	newPayload = append(newPayload, newTagBytes...)
	newPayload = append(newPayload, payload[endRel:]...)
	payloadDelta := int64(len(newPayload) - len(payload))
	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{{
		ExportIndex:    exportIndex,
		Payload:        newPayload,
		UpdateScript:   true,
		ScriptStartRel: exp.ScriptSerializationStartOffset,
		ScriptEndRel:   exp.ScriptSerializationEndOffset + payloadDelta,
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

func widgetRemoveRewriteGeneratedClassFieldRecords(asset *uasset.Asset, generatedClassExport int, objectName string) ([]byte, *uasset.Asset, error) {
	layout, err := captureGeneratedClassWidgetVariableFieldLayout(asset, generatedClassExport)
	if err != nil {
		return nil, nil, err
	}
	exp := asset.Exports[generatedClassExport]
	serialStart := int(exp.SerialOffset)
	serialEnd := int(exp.SerialOffset + exp.SerialSize)
	if serialStart < 0 || serialEnd < serialStart || serialEnd > len(asset.Raw.Bytes) {
		return nil, nil, fmt.Errorf("generated class export serial range out of bounds")
	}
	payload := append([]byte(nil), asset.Raw.Bytes[serialStart:serialEnd]...)
	tail := append([]byte(nil), payload[layout.ScriptEnd:]...)
	if len(tail) < layout.RecordsEnd || len(tail) < 16 {
		return nil, nil, fmt.Errorf("generated class trailer is shorter than expected")
	}
	order := packageByteOrder(asset)
	records := make([][]byte, 0, layout.FieldCount)
	offset := 16
	removed := 0
	for i := 0; i < layout.FieldCount; i++ {
		recordLen, parseErr := generatedClassWidgetVariableFieldRecordLength(tail[offset:], order)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("parse generated class field record %d: %w", i, parseErr)
		}
		record := append([]byte(nil), tail[offset:offset+recordLen]...)
		match, err := widgetRemoveGeneratedClassFieldRecordMatches(asset, record, objectName)
		if err != nil {
			return nil, nil, err
		}
		if match {
			removed++
		} else {
			records = append(records, record)
		}
		offset += recordLen
	}
	if removed == 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	newTail := append([]byte{}, tail[:12]...)
	newTail = appendInt32Ordered(newTail, int32(len(records)), order)
	for _, record := range records {
		newTail = append(newTail, record...)
	}
	newTail = append(newTail, tail[layout.RecordsEnd:]...)
	newPayload := append([]byte{}, payload[:layout.ScriptEnd]...)
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

func widgetRemoveGeneratedClassFieldRecordMatches(asset *uasset.Asset, raw []byte, objectName string) (bool, error) {
	if asset == nil {
		return false, fmt.Errorf("asset is nil")
	}
	reader := uasset.NewByteReaderWithByteSwapping(raw, asset.Summary.UsesByteSwappedSerialization())
	if _, err := reader.ReadNameRef(len(asset.Names)); err != nil {
		return false, fmt.Errorf("read generated class field type: %w", err)
	}
	fieldName, err := reader.ReadNameRef(len(asset.Names))
	if err != nil {
		return false, fmt.Errorf("read generated class field name: %w", err)
	}
	return strings.EqualFold(fieldName.Display(asset.Names), objectName), nil
}

func widgetRemoveRemovePropertyIfPresent(asset *uasset.Asset, exportIndex int, propertyName string) ([]byte, *uasset.Asset, error) {
	result, err := edit.BuildPropertyRemoveMutation(asset, exportIndex, propertyName)
	if err != nil {
		if widgetRemovePropertyNotFound(err) {
			return append([]byte(nil), asset.Raw.Bytes...), asset, nil
		}
		return nil, nil, err
	}
	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{result.Mutation})
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite export removing %s: %w", propertyName, err)
	}
	updatedAsset, err := uasset.ParseBytes(outBytes, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, nil, fmt.Errorf("reparse property removal %s: %w", propertyName, err)
	}
	return outBytes, updatedAsset, nil
}

func widgetRemovePropertyNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "property not found")
}

func widgetRemoveAsInt64(v any) (int64, error) {
	switch x := v.(type) {
	case int:
		return int64(x), nil
	case int32:
		return int64(x), nil
	case int64:
		return x, nil
	case uint32:
		return int64(x), nil
	case uint64:
		if x > uint64(^uint64(0)>>1) {
			return 0, fmt.Errorf("uint64 overflows int64: %d", x)
		}
		return int64(x), nil
	case float64:
		return int64(x), nil
	default:
		return 0, fmt.Errorf("unsupported numeric type %T", v)
	}
}

type widgetRemoveCompactionResult struct {
	removedExportCount int
	removedImportCount int
	removedNameCount   int
}

func widgetRemoveCompactOrphans(asset *uasset.Asset, opts uasset.ParseOptions, removeExports []int, removedObjectName, removedClassName string) (widgetRemoveCompactionResult, []byte, *uasset.Asset, error) {
	if asset == nil {
		return widgetRemoveCompactionResult{}, nil, nil, fmt.Errorf("asset is nil")
	}

	candidateExports := uniqueSortedInts(removeExports)
	candidateImportSet := map[int]bool{}
	nameCandidates := make([]string, 0, len(candidateExports)*2)
	for _, exportIdx := range candidateExports {
		if exportIdx < 0 || exportIdx >= len(asset.Exports) {
			return widgetRemoveCompactionResult{}, nil, nil, fmt.Errorf("remove export index out of range: %d", exportIdx)
		}
		exp := asset.Exports[exportIdx]
		if name := strings.TrimSpace(exp.ObjectName.Display(asset.Names)); name != "" {
			nameCandidates = append(nameCandidates, name)
		}
		if exp.ClassIndex < 0 {
			importIdx := exp.ClassIndex.ResolveIndex()
			candidateImportSet[importIdx] = true
			if importIdx >= 0 && importIdx < len(asset.Imports) {
				if name := strings.TrimSpace(asset.Imports[importIdx].ObjectName.Display(asset.Names)); name != "" {
					nameCandidates = append(nameCandidates, name)
				}
			}
		}
	}

	workingBytes, err := edit.RemoveExportEntries(asset, candidateExports)
	if err != nil {
		return widgetRemoveCompactionResult{}, nil, nil, err
	}
	workingAsset, err := uasset.ParseBytes(workingBytes, opts)
	if err != nil {
		return widgetRemoveCompactionResult{}, nil, nil, fmt.Errorf("reparse export-compacted asset: %w", err)
	}
	result := widgetRemoveCompactionResult{
		removedExportCount: len(asset.Exports) - len(workingAsset.Exports),
	}

	candidateImports := make([]int, 0, len(candidateImportSet))
	for idx := range candidateImportSet {
		candidateImports = append(candidateImports, idx)
	}
	candidateImports = uniqueSortedInts(candidateImports)
	if len(candidateImports) > 0 {
		workingBytes, err = widgetRemoveCompactCandidateImports(workingAsset, candidateImports)
		if err != nil {
			return widgetRemoveCompactionResult{}, nil, nil, err
		}
		nextAsset, err := uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return widgetRemoveCompactionResult{}, nil, nil, fmt.Errorf("reparse import-compacted asset: %w", err)
		}
		result.removedImportCount = len(workingAsset.Imports) - len(nextAsset.Imports)
		workingAsset = nextAsset
	}

	nameCandidates = append(nameCandidates, widgetRemoveSupplementalNameCandidates(removedObjectName, removedClassName)...)
	nameCandidates = uniqueNonEmptyStrings(nameCandidates)
	if len(nameCandidates) > 0 {
		beforeNames := len(workingAsset.Names)
		workingBytes, _, err = compactUnusedNames(workingAsset, opts, nameCandidates)
		if err != nil {
			// Name compaction is opportunistic. Some real-world WidgetBlueprint
			// payloads still contain names that the generic property update scope
			// cannot safely re-encode; keep the rewritten asset without dropping
			// names rather than failing the widget removal itself.
		} else {
			nextAsset, err := uasset.ParseBytes(workingBytes, opts)
			if err != nil {
				return widgetRemoveCompactionResult{}, nil, nil, fmt.Errorf("reparse name-compacted asset: %w", err)
			}
			result.removedNameCount = beforeNames - len(nextAsset.Names)
			workingAsset = nextAsset
		}
	}

	return result, append([]byte(nil), workingAsset.Raw.Bytes...), workingAsset, nil
}

func widgetRemoveNormalizeTextResidue(asset *uasset.Asset, opts uasset.ParseOptions) ([]byte, *uasset.Asset, int, error) {
	if asset == nil {
		return nil, nil, 0, fmt.Errorf("asset is nil")
	}

	hasTextProperty := false
	for exportIdx := range asset.Exports {
		_, err := decodeExportRootPropertyValue(asset, exportIdx, "Text")
		if err == nil {
			hasTextProperty = true
			break
		}
		if widgetRemovePropertyNotFound(err) {
			continue
		}
		return nil, nil, 0, err
	}
	if hasTextProperty {
		return append([]byte(nil), asset.Raw.Bytes...), asset, 0, nil
	}

	workingAsset := asset
	workingBytes := append([]byte(nil), asset.Raw.Bytes...)
	if workingAsset.Summary.GatherableTextDataCount == 0 && (workingAsset.Summary.PackageFlags&packageFlagRequiresLoc) != 0 {
		outBytes, _, err := rewritePackageFlags(workingAsset, workingAsset.Summary.PackageFlags&^packageFlagRequiresLoc)
		if err != nil {
			return nil, nil, 0, err
		}
		workingAsset, err = uasset.ParseBytes(outBytes, opts)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("reparse package flag rewrite: %w", err)
		}
		workingBytes = outBytes
	}

	beforeNames := len(workingAsset.Names)
	compactedBytes, _, err := compactUnusedNames(workingAsset, opts, []string{"Text"})
	if err != nil {
		return workingBytes, workingAsset, 0, nil
	}
	nextAsset, err := uasset.ParseBytes(compactedBytes, opts)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("reparse Text name compaction: %w", err)
	}
	return compactedBytes, nextAsset, beforeNames - len(nextAsset.Names), nil
}

func widgetRemoveCompactCandidateImports(asset *uasset.Asset, candidateImports []int) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	current := uniqueSortedInts(candidateImports)
	if len(current) == 0 {
		return append([]byte(nil), asset.Raw.Bytes...), nil
	}
	for len(current) > 0 {
		out, err := edit.RemoveImportEntries(asset, current)
		if err == nil {
			return out, nil
		}
		blockedOneBased, ok := widgetRemoveBlockedImportOneBased(err)
		if !ok {
			return nil, err
		}
		blockedZero := blockedOneBased - 1
		next := current[:0]
		removedBlocked := false
		for _, idx := range current {
			if idx == blockedZero {
				removedBlocked = true
				continue
			}
			next = append(next, idx)
		}
		if !removedBlocked {
			return nil, err
		}
		current = append([]int(nil), next...)
	}
	return append([]byte(nil), asset.Raw.Bytes...), nil
}

func widgetRemoveBlockedImportOneBased(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	msg := strings.ToLower(err.Error())
	marker := "removed import "
	idx := strings.Index(msg, marker)
	if idx < 0 {
		return 0, false
	}
	start := idx + len(marker)
	end := start
	for end < len(msg) && msg[end] >= '0' && msg[end] <= '9' {
		end++
	}
	if end == start {
		return 0, false
	}
	n, parseErr := strconv.Atoi(msg[start:end])
	if parseErr != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func widgetRemoveSupplementalNameCandidates(removedObjectName, removedClassName string) []string {
	candidates := []string{
		removedObjectName,
		removedClassName,
		"DisplayLabel",
		"GeneratedVariables",
		"BPVariableDescription",
		"BPVariableMetaDataEntry",
		"VarGuid",
		"VarName",
		"VarType",
		"FriendlyName",
		"DefaultValue",
		"MetaDataArray",
		"DataKey",
		"DataValue",
		"PropertyFlags",
		"RepNotifyFunc",
		"ReplicationCondition",
		"ELifetimeCondition",
		"COND_None",
		"TextProperty",
		"UInt64Property",
		"DisplayName",
		"EditInline",
		"EdGraphPinType",
		"PropertyGuids",
		"Slots",
		"Slot",
		"Parent",
		"Content",
		"object",
	}
	return uniqueNonEmptyStrings(candidates)
}

func uniqueSortedInts(items []int) []int {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[int]bool, len(items))
	out := make([]int, 0, len(items))
	for _, item := range items {
		if seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	sortIntsLocal(out)
	return out
}

func uniqueNonEmptyStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, trimmed)
	}
	return out
}

func widgetRemoveEnsureTargetDetached(asset *uasset.Asset, opts uasset.ParseOptions, targetPath, objectName, blueprintObjectName string, snapshot widgetCompileArtifactsSnapshot) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		return nil, nil, err
	}
	target, err := selectWidgetWriteTarget(targets, targetPath)
	if err != nil {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	parentPath := widgetRemoveParentPath(target.Path)
	parent, err := selectWidgetWriteTarget(targets, parentPath)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve persistent target parent %q: %w", parentPath, err)
	}
	ctx, err := resolveWidgetAddContext(asset, *parent)
	if err != nil {
		return nil, nil, err
	}
	slotRemoveSet := widgetRemoveIndexSet(target.SlotExports)
	widgetRemoveSet := widgetRemoveIndexSet(target.Exports)

	_, workingAsset, err := widgetRemoveRewriteParentSlots(asset, ctx, slotRemoveSet)
	if err != nil {
		return nil, nil, err
	}
	_, workingAsset, err = widgetRemoveRewriteAllWidgets(workingAsset, ctx, widgetRemoveSet)
	if err != nil {
		return nil, nil, err
	}
	_, workingAsset, err = widgetRemoveRewriteGuidMap(workingAsset, target.BlueprintExport, objectName)
	if err != nil {
		return nil, nil, err
	}
	remainingGeneratedVariables, nextBytes, nextAsset, err := widgetRemoveRewriteGeneratedVariables(workingAsset, target.BlueprintExport, objectName)
	if err != nil {
		return nil, nil, err
	}
	_ = nextBytes
	workingAsset = nextAsset
	_, workingAsset, err = widgetRemoveRewriteCategorySorting(workingAsset, target.BlueprintExport, blueprintObjectName, remainingGeneratedVariables)
	if err != nil {
		return nil, nil, err
	}
	if ctx.GeneratedClassExport >= 0 {
		_, workingAsset, err = widgetRemoveRewriteGeneratedClassPropertyGuids(workingAsset, ctx.GeneratedClassExport, objectName)
		if err != nil {
			return nil, nil, err
		}
		_, workingAsset, err = widgetRemoveRewriteGeneratedClassFieldRecords(workingAsset, ctx.GeneratedClassExport, objectName)
		if err != nil {
			return nil, nil, err
		}
	}
	_, workingAsset, err = restoreWidgetBlueprintCompileArtifacts(workingAsset, opts, blueprintObjectName, snapshot)
	if err != nil {
		return nil, nil, err
	}
	return finalizeWidgetBlueprintMutation(asset, workingAsset, opts, blueprintObjectName)
}

func validateWidgetRemoveResult(beforeShape, afterShape widgetBlueprintShape, targetPath string) error {
	for role, beforePaths := range beforeShape {
		afterPaths := afterShape[role]
		beforeHadTarget := false
		for _, path := range beforePaths {
			if strings.EqualFold(path, targetPath) {
				beforeHadTarget = true
				break
			}
		}
		if !beforeHadTarget {
			continue
		}
		if len(afterPaths) != len(beforePaths)-1 {
			return fmt.Errorf("tree role %q path count: got %d want %d", role, len(afterPaths), len(beforePaths)-1)
		}
		for _, path := range afterPaths {
			if strings.EqualFold(path, targetPath) || strings.HasPrefix(strings.ToLower(path), strings.ToLower(targetPath)+"/") {
				return fmt.Errorf("tree role %q still contains removed path %q", role, targetPath)
			}
		}
	}
	return nil
}
