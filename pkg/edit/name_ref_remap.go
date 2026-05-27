package edit

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

type nameEntryKey struct {
	Value              string
	NonCaseHash        uint16
	CasePreservingHash uint16
}

type importNameRefPatch struct {
	classPackagePos int
	classNamePos    int
	objectNamePos   int
	packageNamePos  int
}

type exportObjectNamePatch struct {
	objectNamePos int
}

// BuildNameIndexRemap matches old NameMap indices to the corresponding indices
// in a rewritten NameMap. Exact entry matches are paired first; remaining
// unmatched entries are paired in order, which covers one-entry rename/reorder
// flows such as Blueprint variable rename.
func BuildNameIndexRemap(oldNames, newNames []uasset.NameEntry) (map[int32]int32, error) {
	if len(oldNames) == 0 {
		return map[int32]int32{}, nil
	}

	newQueues := make(map[nameEntryKey][]int32, len(newNames))
	for i, entry := range newNames {
		key := nameEntryKey{
			Value:              entry.Value,
			NonCaseHash:        entry.NonCaseHash,
			CasePreservingHash: entry.CasePreservingHash,
		}
		newQueues[key] = append(newQueues[key], int32(i))
	}

	remap := make(map[int32]int32, len(oldNames))
	matchedNew := make([]bool, len(newNames))
	unmatchedOld := make([]int32, 0, 4)
	for i, entry := range oldNames {
		key := nameEntryKey{
			Value:              entry.Value,
			NonCaseHash:        entry.NonCaseHash,
			CasePreservingHash: entry.CasePreservingHash,
		}
		queue := newQueues[key]
		if len(queue) == 0 {
			unmatchedOld = append(unmatchedOld, int32(i))
			continue
		}
		newIdx := queue[0]
		newQueues[key] = queue[1:]
		remap[int32(i)] = newIdx
		if newIdx >= 0 && int(newIdx) < len(matchedNew) {
			matchedNew[newIdx] = true
		}
	}

	unmatchedNew := make([]int32, 0, 4)
	for i := range newNames {
		if !matchedNew[i] {
			unmatchedNew = append(unmatchedNew, int32(i))
		}
	}
	if len(unmatchedOld) != len(unmatchedNew) {
		return nil, fmt.Errorf("cannot build name remap: unmatched old=%d new=%d", len(unmatchedOld), len(unmatchedNew))
	}
	for i := range unmatchedOld {
		remap[unmatchedOld[i]] = unmatchedNew[i]
	}
	return remap, nil
}

// BuildNameIndexRemapAllowInsertedNewEntries matches each old NameMap entry to
// an exact entry in newNames while permitting additional unmatched entries in
// newNames. This is suitable for flows that insert new names without deleting
// or renaming existing ones.
func BuildNameIndexRemapAllowInsertedNewEntries(oldNames, newNames []uasset.NameEntry) (map[int32]int32, error) {
	if len(oldNames) == 0 {
		return map[int32]int32{}, nil
	}

	newQueues := make(map[nameEntryKey][]int32, len(newNames))
	for i, entry := range newNames {
		key := nameEntryKey{
			Value:              entry.Value,
			NonCaseHash:        entry.NonCaseHash,
			CasePreservingHash: entry.CasePreservingHash,
		}
		newQueues[key] = append(newQueues[key], int32(i))
	}

	remap := make(map[int32]int32, len(oldNames))
	for i, entry := range oldNames {
		key := nameEntryKey{
			Value:              entry.Value,
			NonCaseHash:        entry.NonCaseHash,
			CasePreservingHash: entry.CasePreservingHash,
		}
		queue := newQueues[key]
		if len(queue) == 0 {
			return nil, fmt.Errorf("cannot build insertion name remap: old index %d (%q) missing in rewritten NameMap", i, entry.Value)
		}
		newIdx := queue[0]
		newQueues[key] = queue[1:]
		remap[int32(i)] = newIdx
	}
	return remap, nil
}

// RewriteImportExportNameRefs patches ImportMap / ExportMap NameRef indices
// after the NameMap ordering changes.
func RewriteImportExportNameRefs(asset *uasset.Asset, indexRemap map[int32]int32) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if len(asset.Raw.Bytes) == 0 {
		return nil, fmt.Errorf("asset has no raw bytes")
	}
	if len(indexRemap) == 0 {
		return append([]byte(nil), asset.Raw.Bytes...), nil
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	imports, err := scanImportNameRefPositions(asset.Raw.Bytes, asset)
	if err != nil {
		return nil, fmt.Errorf("scan import name refs: %w", err)
	}
	if len(imports) != len(asset.Imports) {
		return nil, fmt.Errorf("import field scan mismatch: got %d want %d", len(imports), len(asset.Imports))
	}
	exports, err := scanExportObjectNamePositions(asset.Raw.Bytes, asset)
	if err != nil {
		return nil, fmt.Errorf("scan export object names: %w", err)
	}
	if len(exports) != len(asset.Exports) {
		return nil, fmt.Errorf("export field scan mismatch: got %d want %d", len(exports), len(asset.Exports))
	}
	softObjectPathRefs, err := scanSoftObjectPathNameRefPositions(asset.Raw.Bytes, asset)
	if err != nil {
		return nil, fmt.Errorf("scan soft object path name refs: %w", err)
	}

	out := append([]byte(nil), asset.Raw.Bytes...)
	changed := false
	targetNameCount := maxRemappedNameCount(len(asset.Names), indexRemap)
	for i := range imports {
		for _, pos := range []int{
			imports[i].classPackagePos,
			imports[i].classNamePos,
			imports[i].objectNamePos,
			imports[i].packageNamePos,
		} {
			if pos < 0 {
				continue
			}
			patched, err := patchNameRefIndexAt(out, pos, indexRemap, order, targetNameCount)
			if err != nil {
				return nil, fmt.Errorf("patch import[%d] name ref: %w", i+1, err)
			}
			changed = changed || patched
		}
	}
	for i := range exports {
		patched, err := patchNameRefIndexAt(out, exports[i].objectNamePos, indexRemap, order, targetNameCount)
		if err != nil {
			return nil, fmt.Errorf("patch export[%d] object name: %w", i+1, err)
		}
		changed = changed || patched
	}
	for i, pos := range softObjectPathRefs {
		patched, err := patchNameRefIndexAt(out, pos, indexRemap, order, targetNameCount)
		if err != nil {
			return nil, fmt.Errorf("patch soft object path ref[%d]: %w", i+1, err)
		}
		changed = changed || patched
	}
	if asset.Summary.MetaDataOffset > 0 {
		metaStart := int64(asset.Summary.MetaDataOffset)
		metaEnd := nextKnownOffsetWithinFile(asset, metaStart)
		patched, err := rewriteOpaqueNameRefsInRange(out, int(metaStart), int(metaEnd), indexRemap)
		if err != nil {
			return nil, fmt.Errorf("patch metadata name refs: %w", err)
		}
		changed = changed || patched
	}
	if !changed {
		return out, nil
	}
	if err := FinalizePackageBytes(out, asset.Summary.FileVersionUE5); err != nil {
		return nil, fmt.Errorf("finalize package bytes: %w", err)
	}
	return out, nil
}

func maxRemappedNameCount(baseCount int, indexRemap map[int32]int32) int {
	count := baseCount
	for _, idx := range indexRemap {
		if next := int(idx) + 1; next > count {
			count = next
		}
	}
	return count
}

// BuildExportNameRemapMutations reserializes tagged-property exports against the
// rewritten NameMap and updates Blueprint-facing display strings that UE renames
// alongside variable declarations.
// BuildExportNameRemapMutationsPropertyOnly remaps NameRef indices only within
// the tagged-property region of each export. Bytecode, script headers, and
// other opaque areas are left untouched. This avoids false-positive remapping
// of int32 values that coincidentally match old name indices.
func BuildExportNameRemapMutationsPropertyOnly(oldAsset, newAsset *uasset.Asset, indexRemap map[int32]int32) ([]ExportMutation, error) {
	return buildExportNameRemapMutationsInner(oldAsset, newAsset, indexRemap, "", "", true)
}

func BuildExportNameRemapMutations(oldAsset, newAsset *uasset.Asset, indexRemap map[int32]int32, fromDisplay, toDisplay string) ([]ExportMutation, error) {
	return buildExportNameRemapMutationsInner(oldAsset, newAsset, indexRemap, fromDisplay, toDisplay, false)
}

func buildExportNameRemapMutationsInner(oldAsset, newAsset *uasset.Asset, indexRemap map[int32]int32, fromDisplay, toDisplay string, propertyOnly bool) ([]ExportMutation, error) {
	if oldAsset == nil {
		return nil, fmt.Errorf("old asset is nil")
	}
	if newAsset == nil {
		return nil, fmt.Errorf("new asset is nil")
	}
	if len(oldAsset.Exports) != len(newAsset.Exports) {
		return nil, fmt.Errorf("export count mismatch: old=%d new=%d", len(oldAsset.Exports), len(newAsset.Exports))
	}

	var order binary.ByteOrder = binary.LittleEndian
	if newAsset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}
	fromDisplay = strings.TrimSpace(fromDisplay)
	toDisplay = strings.TrimSpace(toDisplay)
	oldNoneIndex := int32(findNameIndex(oldAsset.Names, "None"))
	newNoneIndex := int32(findNameIndex(newAsset.Names, "None"))
	hasNoneRemap := oldNoneIndex >= 0 && newNoneIndex >= 0 && oldNoneIndex != newNoneIndex

	mutations := make([]ExportMutation, 0, len(oldAsset.Exports))
	for i, oldExp := range oldAsset.Exports {
		oldStart := int(oldExp.SerialOffset)
		oldEnd := int(oldExp.SerialOffset + oldExp.SerialSize)
		if oldStart < 0 || oldEnd < oldStart || oldEnd > len(oldAsset.Raw.Bytes) {
			return nil, fmt.Errorf("export[%d] serial range out of bounds", i+1)
		}
		oldPayload := append([]byte(nil), oldAsset.Raw.Bytes[oldStart:oldEnd]...)
		newPayload := append([]byte(nil), oldPayload...)
		propertyDelta := 0

		propertyStart, propertyEnd, withClassControl := exportPropertyBounds(oldAsset, oldExp)
		if propertyStart < oldStart || propertyEnd < propertyStart || propertyEnd > oldEnd {
			return nil, fmt.Errorf("export[%d] property range out of bounds", i+1)
		}

		className := oldAsset.ResolveClassName(oldExp)
		useSafePartialRemap := !propertyOnly &&
			fromDisplay == "" &&
			toDisplay == "" &&
			packageIndexRemapCanSkipTaggedPropertyWarnings(className) &&
			!strings.EqualFold(className, "WidgetBlueprint")
		parsed := oldAsset.ParseTaggedPropertiesRange(propertyStart, propertyEnd, withClassControl)
		if len(parsed.Warnings) > 0 {
			if packageIndexRemapCanSkipTaggedPropertyWarnings(className) {
				mutation, changed, err := buildPartialNameMapRemapMutation(oldAsset, newAsset, i, oldExp, parsed, order, indexRemap, fromDisplay, toDisplay)
				if err != nil {
					return nil, err
				}
				if changed {
					mutations = append(mutations, *mutation)
				}
				continue
			}
			return nil, fmt.Errorf("cannot safely remap export[%d] tagged properties: %s", i+1, strings.Join(parsed.Warnings, "; "))
		}
		if parsed.EndOffset < propertyStart+8 {
			return nil, fmt.Errorf("export[%d] property terminator not found", i+1)
		}
		if useSafePartialRemap {
			mutation, changed, err := buildPartialNameMapRemapMutation(oldAsset, newAsset, i, oldExp, parsed, order, indexRemap, fromDisplay, toDisplay)
			if err != nil {
				return nil, err
			}
			if changed {
				mutations = append(mutations, *mutation)
			}
			continue
		}
		noneStart := parsed.EndOffset - 8
		propsChanged := false

		if propertyOnly {
			// Property-only mode: remap NameRefs in the property region using
			// direct byte scanning, avoiding the decode→encode roundtrip.
			propRelStart := propertyStart - oldStart
			propRelEnd := parsed.EndOffset - oldStart
			if propRelStart >= 0 && propRelEnd <= len(newPayload) {
				rewritten, changed := remapOpaqueNameRefPairsLE(newPayload[propRelStart:propRelEnd], indexRemap)
				if changed {
					patchedPayload := append([]byte(nil), newPayload...)
					copy(patchedPayload[propRelStart:], rewritten)
					newPayload = patchedPayload
					propsChanged = true
				}
			}
		} else {
			prefixEnd := noneStart
			if len(parsed.Properties) > 0 {
				prefixEnd = parsed.Properties[0].Offset
			}
			tagBlob := append([]byte(nil), oldAsset.Raw.Bytes[propertyStart:prefixEnd]...)
			for j, tag := range parsed.Properties {
				decoded, ok := oldAsset.DecodePropertyValue(tag)
				if !ok {
					tagStart := tag.Offset
					tagEnd := noneStart
					if j+1 < len(parsed.Properties) {
						tagEnd = parsed.Properties[j+1].Offset
					}
					tagBlob = append(tagBlob, oldAsset.Raw.Bytes[tagStart:tagEnd]...)
					continue
				}
				remappedValue, valueChanged, err := remapDecodedValueForNameMap(decoded, indexRemap, newAsset.Names, fromDisplay, toDisplay)
				if err != nil {
					return nil, fmt.Errorf("remap export[%d] property %s value: %w", i+1, tag.Name.Display(oldAsset.Names), err)
				}
				remappedTag, tagChanged, err := remapPropertyTagNameRefs(tag, indexRemap, newAsset.Names)
				if err != nil {
					return nil, fmt.Errorf("remap export[%d] property %s tag: %w", i+1, tag.Name.Display(oldAsset.Names), err)
				}
				typeTree, err := buildTypeTree(remappedTag.TypeNodes, newAsset.Names)
				if err != nil {
					return nil, fmt.Errorf("build export[%d] property %s type tree: %w", i+1, tag.Name.Display(oldAsset.Names), err)
				}
				valueBytes, boolValue, err := encodePropertyValue(newAsset, typeTree, remappedValue, order)
				if err != nil {
					return nil, fmt.Errorf("encode export[%d] property %s: %w", i+1, tag.Name.Display(oldAsset.Names), err)
				}
				tagBytes, _, err := serializePropertyTag(newAsset, remappedTag, valueBytes, boolValue, order)
				if err != nil {
					return nil, fmt.Errorf("serialize export[%d] property %s: %w", i+1, tag.Name.Display(oldAsset.Names), err)
				}
				tagStart := tag.Offset
				tagEnd := noneStart
				if j+1 < len(parsed.Properties) {
					tagEnd = parsed.Properties[j+1].Offset
				}
				if !bytes.Equal(tagBytes, oldAsset.Raw.Bytes[tagStart:tagEnd]) || tagChanged || valueChanged {
					propsChanged = true
				}
				tagBlob = append(tagBlob, tagBytes...)
			}
			if propsChanged {
				noneBytes := oldAsset.Raw.Bytes[noneStart:parsed.EndOffset]
				trailing := oldAsset.Raw.Bytes[parsed.EndOffset:propertyEnd]
				newPropertyRegion := make([]byte, 0, len(tagBlob)+len(noneBytes)+len(trailing))
				newPropertyRegion = append(newPropertyRegion, tagBlob...)
				newPropertyRegion = append(newPropertyRegion, noneBytes...)
				newPropertyRegion = append(newPropertyRegion, trailing...)

				relStart := propertyStart - oldStart
				relEnd := propertyEnd - oldStart
				nextPayload := make([]byte, 0, len(oldPayload)+(len(newPropertyRegion)-(propertyEnd-propertyStart)))
				nextPayload = append(nextPayload, oldPayload[:relStart]...)
				nextPayload = append(nextPayload, newPropertyRegion...)
				nextPayload = append(nextPayload, oldPayload[relEnd:]...)
				newPayload = nextPayload
				propertyDelta = len(newPropertyRegion) - (propertyEnd - propertyStart)
			}
		}

		rawChanged := false
		tailStart := parsed.EndOffset - oldStart
		if propertyOnly {
			// Property-only mode: remap NameRefs only in the property region
			// using opaque scanning (avoids decode→encode roundtrip issues).
			propRelStart := propertyStart - oldStart
			propRelEnd := parsed.EndOffset - oldStart
			if propRelStart >= 0 && propRelEnd <= len(newPayload) {
				rewritten, propChanged := remapOpaqueNameRefPairsLE(newPayload[propRelStart:propRelEnd], indexRemap)
				if propChanged {
					patchedPayload := append([]byte(nil), newPayload...)
					copy(patchedPayload[propRelStart:], rewritten)
					newPayload = patchedPayload
					rawChanged = true
					propsChanged = false // clear to avoid double-write below
				}
			}
		} else if fromDisplay == "" {
			blockedTailOffsets := map[int]struct{}{}
			className := oldAsset.ResolveClassName(oldExp)
			blockedOpaqueNoneOffsets := map[int]struct{}{}
			if strings.EqualFold(className, "WidgetBlueprintGeneratedClass") && tailStart >= 0 && tailStart < len(newPayload) {
				for off := range collectOpaqueExportIndexLikeZeroNumberOffsets(newPayload[tailStart:], int32(len(newAsset.Exports)), className, order) {
					blockedOpaqueNoneOffsets[tailStart+off] = struct{}{}
				}
			}
			if hasNoneRemap {
				// Determine the opaque-scan region for None remapping.
				// When propsChanged is true, the property region was already
				// rebuilt with correct NameRefs by the decode→encode cycle.
				// Scanning it again would double-remap values that coincidentally
				// equal oldNoneIndex.  Restrict the scan to non-property areas.
				_ = propertyStart // used only in propsChanged branch below
				_ = propertyEnd
				if strings.EqualFold(className, "EdGraph") {
					specificTailStart := len(newPayload) - 32
					if specificTailStart < 0 {
						specificTailStart = 0
					}
					if specificTailStart >= 0 && specificTailStart < len(newPayload) {
						rewrittenTail, changedOffsets, changed := remapOpaqueSpecificNameRefPairLEPositionsSkipBlocked(newPayload[specificTailStart:], oldNoneIndex, newNoneIndex, nil)
						if changed {
							nextPayload := append([]byte(nil), newPayload[:specificTailStart]...)
							nextPayload = append(nextPayload, rewrittenTail...)
							newPayload = nextPayload
							rawChanged = true
							for off := range changedOffsets {
								absOff := specificTailStart + off
								if absOff >= tailStart {
									blockedTailOffsets[absOff-tailStart] = struct{}{}
								}
							}
						}
					}
				} else if propsChanged {
					// Property region was rebuilt by decode→encode. Some NameRefs
					// now equal oldNoneIndex (e.g. NameProperty remapped to 71)
					// while embedded opaque data still has the old None (71).
					// Only remap positions where the ORIGINAL byte was also
					// oldNoneIndex to avoid double-remapping.
					for off := 0; off+8 <= len(newPayload); off++ {
						cur := int32(order.Uint32(newPayload[off : off+4]))
						num := int32(order.Uint32(newPayload[off+4 : off+8]))
						if _, blocked := blockedOpaqueNoneOffsets[off]; blocked {
							continue
						}
						if cur != oldNoneIndex || num != 0 {
							continue
						}
						// Check original payload at this position.
						if off < len(oldPayload) && off+4 <= len(oldPayload) {
							orig := int32(order.Uint32(oldPayload[off : off+4]))
							if orig != oldNoneIndex {
								continue // Was remapped TO oldNoneIndex by encode; skip.
							}
						}
						order.PutUint32(newPayload[off:off+4], uint32(newNoneIndex))
						rawChanged = true
						if off >= tailStart {
							blockedTailOffsets[off-tailStart] = struct{}{}
						}
						off += 7
					}
				} else {
					rewritten, changedOffsets, changed := remapOpaqueSpecificNameRefPairLEPositionsSkipBlocked(newPayload, oldNoneIndex, newNoneIndex, blockedOpaqueNoneOffsets)
					if changed {
						newPayload = rewritten
						rawChanged = true
						for off := range changedOffsets {
							if off >= tailStart {
								blockedTailOffsets[off-tailStart] = struct{}{}
							}
						}
					}
				}
				_ = propertyDelta
			}
			if tailStart >= 0 && tailStart < len(newPayload) {
				for off := range collectOpaqueExportIndexLikeZeroNumberOffsets(newPayload[tailStart:], int32(len(newAsset.Exports)), className, order) {
					blockedTailOffsets[off] = struct{}{}
				}
			}
			if strings.EqualFold(className, "WidgetBlueprintGeneratedClass") && tailStart >= 0 && tailStart < len(newPayload) {
				nameRefOffsets, err := collectWidgetBlueprintGeneratedClassFieldNameRefOffsets(newPayload[tailStart:], order)
				if err != nil {
					return nil, fmt.Errorf("collect export[%d] WidgetBlueprintGeneratedClass field refs: %w", i+1, err)
				}
				for _, off := range nameRefOffsets {
					blockedTailOffsets[off] = struct{}{}
				}
			}
			if tailStart >= 0 && tailStart < len(newPayload) {
				for off := range collectASCIIFStringDataOffsets(newPayload[tailStart:]) {
					blockedTailOffsets[off] = struct{}{}
				}
			}
			if tailStart >= 0 && tailStart < len(newPayload) {
				rewrittenTail, tailChanged := remapOpaqueNameRefPairsLESkipBlocked(newPayload[tailStart:], indexRemap, -1, blockedTailOffsets)
				if tailChanged {
					nextPayload := append([]byte(nil), newPayload[:tailStart]...)
					nextPayload = append(nextPayload, rewrittenTail...)
					newPayload = nextPayload
					rawChanged = true
				}
			}
			if strings.EqualFold(className, "WidgetBlueprintGeneratedClass") && tailStart >= 0 && tailStart < len(newPayload) {
				tailChanged, err := remapWidgetBlueprintGeneratedClassFieldNameRefs(newPayload[tailStart:], indexRemap, order)
				if err != nil {
					return nil, fmt.Errorf("remap export[%d] WidgetBlueprintGeneratedClass field refs: %w", i+1, err)
				}
				rawChanged = rawChanged || tailChanged
			}
		} else {
			if tailStart >= 0 && tailStart < len(newPayload) {
				rewrittenTail, tailChanged := remapOpaqueNameRefPairsLEAnyNumber(newPayload[tailStart:], indexRemap)
				if tailChanged {
					nextPayload := append([]byte(nil), newPayload[:tailStart]...)
					nextPayload = append(nextPayload, rewrittenTail...)
					newPayload = nextPayload
					rawChanged = true
				}
			}
			if isBlueprintLikeExport(oldAsset, oldExp) && fromDisplay != toDisplay {
				rewritten, changed := replaceEncodedFStringLiterals(newPayload, order, fromDisplay, toDisplay)
				if changed {
					newPayload = rewritten
					rawChanged = true
				}
			}
			if hasNoneRemap {
				className := oldAsset.ResolveClassName(oldExp)
				blockedOpaqueNoneOffsets := map[int]struct{}{}
				if strings.EqualFold(className, "WidgetBlueprintGeneratedClass") {
					propStart, propEnd, withCC := exportPropertyBounds(newAsset, newAsset.Exports[i])
					parsed := newAsset.ParseTaggedPropertiesRange(propStart, propEnd, withCC)
					tailStart := parsed.EndOffset - int(newAsset.Exports[i].SerialOffset)
					if tailStart >= 0 && tailStart < len(newPayload) {
						for off := range collectOpaqueExportIndexLikeZeroNumberOffsets(newPayload[tailStart:], int32(len(newAsset.Exports)), className, order) {
							blockedOpaqueNoneOffsets[tailStart+off] = struct{}{}
						}
					}
				}
				if strings.EqualFold(className, "EdGraph") {
					specificTailStart := len(newPayload) - 32
					if specificTailStart < 0 {
						specificTailStart = 0
					}
					if specificTailStart >= 0 && specificTailStart < len(newPayload) {
						rewrittenTail, changed := remapOpaqueSpecificNameRefPairLE(newPayload[specificTailStart:], oldNoneIndex, newNoneIndex)
						if changed {
							nextPayload := append([]byte(nil), newPayload[:specificTailStart]...)
							nextPayload = append(nextPayload, rewrittenTail...)
							newPayload = nextPayload
							rawChanged = true
						}
					}
				} else {
					rewritten, _, changed := remapOpaqueSpecificNameRefPairLEPositionsSkipBlocked(newPayload, oldNoneIndex, newNoneIndex, blockedOpaqueNoneOffsets)
					if changed {
						newPayload = rewritten
						rawChanged = true
					}
				}
			}
		}

		if !propsChanged && !rawChanged && bytes.Equal(newPayload, oldPayload) {
			continue
		}

		mutation := ExportMutation{
			ExportIndex: i,
			Payload:     newPayload,
		}
		if propertyDelta != 0 && !newAsset.Summary.UsesUnversionedPropertySerialization() && newAsset.Summary.FileVersionUE5 >= ue5ScriptSerializationOffset {
			oldStartRel := oldExp.ScriptSerializationStartOffset
			oldEndRel := oldExp.ScriptSerializationEndOffset
			if oldEndRel >= oldStartRel {
				rangeStartRel := int64(propertyStart - oldStart)
				rangeEndRel := int64(propertyEnd - oldStart)
				if oldStartRel == rangeStartRel && oldEndRel == rangeEndRel {
					mutation.UpdateScript = true
					mutation.ScriptStartRel = oldStartRel
					mutation.ScriptEndRel = oldEndRel + int64(propertyDelta)
				}
			}
		}
		mutations = append(mutations, mutation)
	}
	return mutations, nil
}

func buildPartialNameMapRemapMutation(oldAsset, newAsset *uasset.Asset, exportIndex int, oldExp uasset.ExportEntry, parsed uasset.PropertyListResult, order binary.ByteOrder, indexRemap map[int32]int32, fromDisplay, toDisplay string) (*ExportMutation, bool, error) {
	oldStart := int(oldExp.SerialOffset)
	oldEnd := int(oldExp.SerialOffset + oldExp.SerialSize)
	if oldStart < 0 || oldEnd < oldStart || oldEnd > len(oldAsset.Raw.Bytes) {
		return nil, false, fmt.Errorf("export[%d] serial range out of bounds", exportIndex+1)
	}
	className := oldAsset.ResolveClassName(oldExp)
	newPayload := append([]byte(nil), oldAsset.Raw.Bytes[oldStart:oldEnd]...)
	changed := false
	if strings.EqualFold(className, "WidgetBlueprintGeneratedClass") && fromDisplay == "" && toDisplay == "" {
		rawChanged, err := remapWidgetBlueprintGeneratedClassPropertiesFromOld(oldAsset, oldStart, oldEnd, newPayload, parsed, indexRemap, order)
		if err != nil {
			return nil, false, fmt.Errorf("export[%d] raw WidgetBlueprintGeneratedClass property remap: %w", exportIndex+1, err)
		}
		changed = changed || rawChanged
	} else {
		for i, tag := range parsed.Properties {
			decoded, ok := oldAsset.DecodePropertyValue(tag)
			if !ok {
				continue
			}
			remappedValue, valueChanged, err := remapDecodedValueForNameMap(decoded, indexRemap, newAsset.Names, fromDisplay, toDisplay)
			if err != nil {
				return nil, false, fmt.Errorf("remap export[%d] property %s value: %w", exportIndex+1, tag.Name.Display(oldAsset.Names), err)
			}
			remappedTag, tagChanged, err := remapPropertyTagNameRefs(tag, indexRemap, newAsset.Names)
			if err != nil {
				return nil, false, fmt.Errorf("remap export[%d] property %s tag: %w", exportIndex+1, tag.Name.Display(oldAsset.Names), err)
			}
			if !valueChanged && !tagChanged {
				continue
			}
			typeTree, err := buildTypeTree(remappedTag.TypeNodes, newAsset.Names)
			if err != nil {
				return nil, false, fmt.Errorf("build export[%d] property %s type tree: %w", exportIndex+1, tag.Name.Display(oldAsset.Names), err)
			}
			valueBytes, boolValue, err := encodePropertyValue(newAsset, typeTree, remappedValue, order)
			if err != nil {
				return nil, false, fmt.Errorf("encode export[%d] property %s: %w", exportIndex+1, tag.Name.Display(oldAsset.Names), err)
			}
			tagBytes, _, err := serializePropertyTag(newAsset, remappedTag, valueBytes, boolValue, order)
			if err != nil {
				return nil, false, fmt.Errorf("serialize export[%d] property %s: %w", exportIndex+1, tag.Name.Display(oldAsset.Names), err)
			}
			tagStartRel := tag.Offset - oldStart
			tagEndAbs := tag.ValueOffset + int(tag.Size)
			if i+1 < len(parsed.Properties) {
				tagEndAbs = parsed.Properties[i+1].Offset
			}
			tagEndRel := tagEndAbs - oldStart
			if tagStartRel < 0 || tagEndRel < tagStartRel || tagEndRel > len(newPayload) {
				return nil, false, fmt.Errorf("export[%d] property %s partial range out of bounds", exportIndex+1, tag.Name.Display(oldAsset.Names))
			}
			if len(tagBytes) != tagEndRel-tagStartRel {
				continue
			}
			copy(newPayload[tagStartRel:tagEndRel], tagBytes)
			changed = true
		}
	}
	if parsed.EndOffset >= oldStart+8 {
		noneStartRel := parsed.EndOffset - 8 - oldStart
		if noneStartRel >= 0 && noneStartRel+8 <= len(newPayload) {
			if patchNameRefIndexInPlace(newPayload[noneStartRel:noneStartRel+8], indexRemap, order) {
				changed = true
			}
		}
	}
	tailStartRel := parsed.EndOffset - oldStart
	if tailStartRel >= 0 && tailStartRel < len(newPayload) {
		blockedTailOffsets := map[int]struct{}{}
		for off := range collectOpaqueExportIndexLikeZeroNumberOffsets(oldAsset.Raw.Bytes[oldStart+tailStartRel:oldEnd], int32(len(newAsset.Exports)), className, order) {
			blockedTailOffsets[off] = struct{}{}
		}
		for off := range collectASCIIFStringDataOffsets(oldAsset.Raw.Bytes[oldStart+tailStartRel : oldEnd]) {
			blockedTailOffsets[off] = struct{}{}
		}
		if remapOpaqueNameRefPairsFromOldSkipBlocked(
			oldAsset.Raw.Bytes[oldStart+tailStartRel:oldEnd],
			newPayload[tailStartRel:],
			indexRemap,
			blockedTailOffsets,
			order,
		) {
			changed = true
		}
	}
	if strings.EqualFold(className, "WidgetBlueprintGeneratedClass") {
		fieldRaw := oldAsset.Raw.Bytes[oldStart+tailStartRel : oldEnd]
		fieldOffsets, err := collectWidgetBlueprintGeneratedClassFieldNameRefOffsets(fieldRaw, order)
		if err == nil {
			for _, off := range fieldOffsets {
				if patchNameRefIndexFromOldAtOffset(
					oldAsset.Raw.Bytes[oldStart:oldEnd],
					newPayload,
					tailStartRel+off,
					indexRemap,
					order,
				) {
					changed = true
				}
			}
		}
	}
	if !changed {
		return nil, false, nil
	}
	return &ExportMutation{ExportIndex: exportIndex, Payload: newPayload}, true, nil
}

func remapWidgetBlueprintGeneratedClassPropertiesFromOld(oldAsset *uasset.Asset, oldStart, oldEnd int, newPayload []byte, parsed uasset.PropertyListResult, indexRemap map[int32]int32, order binary.ByteOrder) (bool, error) {
	if oldAsset == nil || len(indexRemap) == 0 {
		return false, nil
	}
	oldPayload := oldAsset.Raw.Bytes[oldStart:oldEnd]
	changed := false
	for _, tag := range parsed.Properties {
		if err := remapPropertyTagNameRefsFromOld(oldPayload, newPayload, tag, oldStart, indexRemap, order); err != nil {
			return false, err
		}
		name := tag.Name.Display(oldAsset.Names)
		if !strings.EqualFold(name, "PropertyGuids") {
			continue
		}
		entryOffsets, err := widgetBlueprintGeneratedClassPropertyGuidsKeyOffsets(oldPayload, tag, oldStart, order)
		if err != nil {
			return false, err
		}
		for _, off := range entryOffsets {
			if patchNameRefIndexFromOldAtOffset(oldPayload, newPayload, off, indexRemap, order) {
				changed = true
			}
		}
	}
	for _, tag := range parsed.Properties {
		if remapPropertyTagNameRefsChanged(oldPayload, newPayload, tag, oldStart, indexRemap, order) {
			changed = true
		}
	}
	return changed, nil
}

func remapPropertyTagNameRefsFromOld(oldPayload, newPayload []byte, tag uasset.PropertyTag, oldStart int, indexRemap map[int32]int32, order binary.ByteOrder) error {
	tagStartRel := tag.Offset - oldStart
	if tagStartRel < 0 || tagStartRel+8 > len(oldPayload) || tagStartRel+8 > len(newPayload) {
		return fmt.Errorf("property tag start out of bounds: %d", tagStartRel)
	}
	valueStartRel := tag.ValueOffset - oldStart
	if valueStartRel < tagStartRel {
		return fmt.Errorf("property value start out of bounds: %d", valueStartRel)
	}
	cursor := tagStartRel
	_ = patchNameRefIndexFromOldAtOffset(oldPayload, newPayload, cursor, indexRemap, order)
	cursor += 8
	for range tag.TypeNodes {
		if cursor+12 > valueStartRel {
			break
		}
		if cursor+12 > len(oldPayload) || cursor+12 > len(newPayload) {
			return fmt.Errorf("property type node out of bounds: %d", cursor)
		}
		_ = patchNameRefIndexFromOldAtOffset(oldPayload, newPayload, cursor, indexRemap, order)
		cursor += 12
	}
	return nil
}

func remapPropertyTagNameRefsChanged(oldPayload, newPayload []byte, tag uasset.PropertyTag, oldStart int, indexRemap map[int32]int32, order binary.ByteOrder) bool {
	tagStartRel := tag.Offset - oldStart
	valueStartRel := tag.ValueOffset - oldStart
	changed := patchNameRefIndexFromOldAtOffset(oldPayload, newPayload, tagStartRel, indexRemap, order)
	cursor := tagStartRel + 8
	for range tag.TypeNodes {
		if cursor+12 > valueStartRel {
			break
		}
		if patchNameRefIndexFromOldAtOffset(oldPayload, newPayload, cursor, indexRemap, order) {
			changed = true
		}
		cursor += 12
	}
	return changed
}

func widgetBlueprintGeneratedClassPropertyGuidsKeyOffsets(oldPayload []byte, tag uasset.PropertyTag, oldStart int, order binary.ByteOrder) ([]int, error) {
	if tag.ValueOffset < oldStart {
		return nil, fmt.Errorf("PropertyGuids value offset out of bounds")
	}
	valueRel := tag.ValueOffset - oldStart
	if valueRel < 0 || valueRel+8 > len(oldPayload) {
		return nil, fmt.Errorf("PropertyGuids value range out of bounds")
	}
	entryCount := int(order.Uint32(oldPayload[valueRel+4 : valueRel+8]))
	if entryCount < 0 || entryCount > 1024 {
		return nil, fmt.Errorf("PropertyGuids entry count out of bounds: %d", entryCount)
	}
	off := valueRel + 8
	offsets := make([]int, 0, entryCount)
	for i := 0; i < entryCount; i++ {
		if off+24 > len(oldPayload) {
			return nil, fmt.Errorf("PropertyGuids entry %d out of bounds", i)
		}
		offsets = append(offsets, off)
		off += 24
	}
	return offsets, nil
}

func packageIndexRemapCanSkipTaggedPropertyWarnings(className string) bool {
	switch strings.ToLower(strings.TrimSpace(className)) {
	case "widgetblueprint", "widgetblueprintgeneratedclass", "edgraph", "k2node_event":
		return true
	default:
		return false
	}
}

// BuildExportPackageIndexRemapMutations reserializes tagged-property exports
// after ImportMap insertion shifts import package indices referenced from
// export payloads.
func BuildExportPackageIndexRemapMutations(oldAsset, newAsset *uasset.Asset, importRemap map[int]int) ([]ExportMutation, error) {
	if oldAsset == nil {
		return nil, fmt.Errorf("old asset is nil")
	}
	if newAsset == nil {
		return nil, fmt.Errorf("new asset is nil")
	}
	if len(oldAsset.Exports) != len(newAsset.Exports) {
		return nil, fmt.Errorf("export count mismatch: old=%d new=%d", len(oldAsset.Exports), len(newAsset.Exports))
	}
	if len(importRemap) == 0 {
		return nil, nil
	}

	var order binary.ByteOrder = binary.LittleEndian
	if newAsset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	mutations := make([]ExportMutation, 0, len(oldAsset.Exports))
	for i, oldExp := range oldAsset.Exports {
		oldStart := int(oldExp.SerialOffset)
		oldEnd := int(oldExp.SerialOffset + oldExp.SerialSize)
		if oldStart < 0 || oldEnd < oldStart || oldEnd > len(oldAsset.Raw.Bytes) {
			return nil, fmt.Errorf("export[%d] serial range out of bounds", i+1)
		}
		propertyStart, propertyEnd, withClassControl := exportPropertyBounds(oldAsset, oldExp)
		if propertyStart < oldStart || propertyEnd < propertyStart || propertyEnd > oldEnd {
			return nil, fmt.Errorf("export[%d] property range out of bounds", i+1)
		}

		parsed := oldAsset.ParseTaggedPropertiesRange(propertyStart, propertyEnd, withClassControl)
		if len(parsed.Warnings) > 0 {
			className := oldAsset.ResolveClassName(oldExp)
			if packageIndexRemapCanSkipTaggedPropertyWarnings(className) {
				continue
			}
			return nil, fmt.Errorf("cannot safely remap export[%d] tagged properties: %s", i+1, strings.Join(parsed.Warnings, "; "))
		}
		if parsed.EndOffset < propertyStart+8 {
			return nil, fmt.Errorf("export[%d] property terminator not found", i+1)
		}
		noneStart := parsed.EndOffset - 8
		prefixEnd := noneStart
		if len(parsed.Properties) > 0 {
			prefixEnd = parsed.Properties[0].Offset
		}
		tagBlob := append([]byte(nil), oldAsset.Raw.Bytes[propertyStart:prefixEnd]...)
		propsChanged := false
		for j, tag := range parsed.Properties {
			decoded, ok := oldAsset.DecodePropertyValue(tag)
			if !ok {
				tagStart := tag.Offset
				tagEnd := noneStart
				if j+1 < len(parsed.Properties) {
					tagEnd = parsed.Properties[j+1].Offset
				}
				tagBlob = append(tagBlob, oldAsset.Raw.Bytes[tagStart:tagEnd]...)
				continue
			}
			remappedValue, valueChanged, err := remapDecodedValueForPackageIndex(decoded, newAsset, importRemap)
			if err != nil {
				return nil, fmt.Errorf("remap export[%d] property %s package indices: %w", i+1, tag.Name.Display(oldAsset.Names), err)
			}
			tagStart := tag.Offset
			tagEnd := noneStart
			if j+1 < len(parsed.Properties) {
				tagEnd = parsed.Properties[j+1].Offset
			}
			if !valueChanged {
				tagBlob = append(tagBlob, oldAsset.Raw.Bytes[tagStart:tagEnd]...)
				continue
			}
			typeTree, err := buildTypeTree(tag.TypeNodes, newAsset.Names)
			if err != nil {
				return nil, fmt.Errorf("build export[%d] property %s type tree: %w", i+1, tag.Name.Display(oldAsset.Names), err)
			}
			valueBytes, boolValue, err := encodePropertyValue(newAsset, typeTree, remappedValue, order)
			if err != nil {
				return nil, fmt.Errorf("encode export[%d] property %s: %w", i+1, tag.Name.Display(oldAsset.Names), err)
			}
			tagBytes, _, err := serializePropertyTag(newAsset, tag, valueBytes, boolValue, order)
			if err != nil {
				return nil, fmt.Errorf("serialize export[%d] property %s: %w", i+1, tag.Name.Display(oldAsset.Names), err)
			}
			if !bytes.Equal(tagBytes, oldAsset.Raw.Bytes[tagStart:tagEnd]) || valueChanged {
				propsChanged = true
			}
			tagBlob = append(tagBlob, tagBytes...)
		}
		oldPayload := oldAsset.Raw.Bytes[oldStart:oldEnd]

		// Scan the tail region for PackageIndex values that need remapping
		// (import indices shifted after import insertion).
		tailStart := parsed.EndOffset - oldStart
		tailChanged := false
		if tailStart >= 0 && tailStart < len(oldPayload) {
			serializedPkgRemap := make(map[int32]int32, len(importRemap))
			for oldIdx, newIdx := range importRemap {
				oldPkg := int32(-(oldIdx + 1))
				newPkg := int32(-(newIdx + 1))
				if oldPkg != newPkg {
					serializedPkgRemap[oldPkg] = newPkg
				}
			}
			if len(serializedPkgRemap) > 0 {
				tail := append([]byte(nil), oldPayload[tailStart:]...)
				// Scan at every byte offset since bytecode int32 values
				// may not be aligned to the tail start boundary.
				for pos := 0; pos+4 <= len(tail); pos++ {
					cur := int32(order.Uint32(tail[pos : pos+4]))
					if replacement, ok := serializedPkgRemap[cur]; ok {
						order.PutUint32(tail[pos:pos+4], uint32(replacement))
						tailChanged = true
						pos += 3 // skip past this int32
					}
				}
				if tailChanged {
					copy(oldPayload[tailStart:], tail)
				}
			}
		}

		if !propsChanged && !tailChanged {
			continue
		}

		var newPayload []byte
		if propsChanged {
			noneBytes := oldAsset.Raw.Bytes[noneStart:parsed.EndOffset]
			trailing := oldAsset.Raw.Bytes[parsed.EndOffset:propertyEnd]
			newPropertyRegion := make([]byte, 0, len(tagBlob)+len(noneBytes)+len(trailing))
			newPropertyRegion = append(newPropertyRegion, tagBlob...)
			newPropertyRegion = append(newPropertyRegion, noneBytes...)
			newPropertyRegion = append(newPropertyRegion, trailing...)

			relStart := propertyStart - oldStart
			relEnd := propertyEnd - oldStart
			newPayload = make([]byte, 0, len(oldPayload)+(len(newPropertyRegion)-(propertyEnd-propertyStart)))
			newPayload = append(newPayload, oldPayload[:relStart]...)
			newPayload = append(newPayload, newPropertyRegion...)
			newPayload = append(newPayload, oldPayload[relEnd:]...)
		} else {
			newPayload = append([]byte(nil), oldPayload...)
		}

		mutation := ExportMutation{ExportIndex: i, Payload: newPayload}
		propertyDelta := len(newPayload) - len(oldPayload)
		if propertyDelta != 0 && !newAsset.Summary.UsesUnversionedPropertySerialization() && newAsset.Summary.FileVersionUE5 >= ue5ScriptSerializationOffset {
			oldStartRel := oldExp.ScriptSerializationStartOffset
			oldEndRel := oldExp.ScriptSerializationEndOffset
			if oldEndRel >= oldStartRel {
				rangeStartRel := int64(propertyStart - oldStart)
				rangeEndRel := int64(propertyEnd - oldStart)
				if oldStartRel == rangeStartRel && oldEndRel == rangeEndRel {
					mutation.UpdateScript = true
					mutation.ScriptStartRel = oldStartRel
					mutation.ScriptEndRel = oldEndRel + int64(propertyDelta)
				}
			}
		}
		mutations = append(mutations, mutation)
	}
	return mutations, nil
}

// BuildExportPackageIndexDeleteMutations reserializes tagged-property exports
// after ImportMap deletion shifts or removes import package indices referenced
// from export payloads.
func BuildExportPackageIndexDeleteMutations(oldAsset, newAsset *uasset.Asset, importRemap map[int]int, removeSet map[int]bool) ([]ExportMutation, error) {
	if oldAsset == nil {
		return nil, fmt.Errorf("old asset is nil")
	}
	if newAsset == nil {
		return nil, fmt.Errorf("new asset is nil")
	}
	if len(oldAsset.Exports) != len(newAsset.Exports) {
		return nil, fmt.Errorf("export count mismatch: old=%d new=%d", len(oldAsset.Exports), len(newAsset.Exports))
	}
	if len(importRemap) == 0 && len(removeSet) == 0 {
		return nil, nil
	}

	var order binary.ByteOrder = binary.LittleEndian
	if newAsset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	mutations := make([]ExportMutation, 0, len(oldAsset.Exports))
	for i, oldExp := range oldAsset.Exports {
		oldStart := int(oldExp.SerialOffset)
		oldEnd := int(oldExp.SerialOffset + oldExp.SerialSize)
		if oldStart < 0 || oldEnd < oldStart || oldEnd > len(oldAsset.Raw.Bytes) {
			return nil, fmt.Errorf("export[%d] serial range out of bounds", i+1)
		}
		propertyStart, propertyEnd, withClassControl := exportPropertyBounds(oldAsset, oldExp)
		if propertyStart < oldStart || propertyEnd < propertyStart || propertyEnd > oldEnd {
			return nil, fmt.Errorf("export[%d] property range out of bounds", i+1)
		}

		parsed := oldAsset.ParseTaggedPropertiesRange(propertyStart, propertyEnd, withClassControl)
		if len(parsed.Warnings) > 0 {
			className := oldAsset.ResolveClassName(oldExp)
			if packageIndexRemapCanSkipTaggedPropertyWarnings(className) {
				continue
			}
			return nil, fmt.Errorf("cannot safely remap export[%d] tagged properties: %s", i+1, strings.Join(parsed.Warnings, "; "))
		}
		if parsed.EndOffset < propertyStart+8 {
			return nil, fmt.Errorf("export[%d] property terminator not found", i+1)
		}
		noneStart := parsed.EndOffset - 8
		prefixEnd := noneStart
		if len(parsed.Properties) > 0 {
			prefixEnd = parsed.Properties[0].Offset
		}
		tagBlob := append([]byte(nil), oldAsset.Raw.Bytes[propertyStart:prefixEnd]...)
		propsChanged := false
		for j, tag := range parsed.Properties {
			decoded, ok := oldAsset.DecodePropertyValue(tag)
			if !ok {
				tagStart := tag.Offset
				tagEnd := noneStart
				if j+1 < len(parsed.Properties) {
					tagEnd = parsed.Properties[j+1].Offset
				}
				tagBlob = append(tagBlob, oldAsset.Raw.Bytes[tagStart:tagEnd]...)
				continue
			}
			remappedValue, valueChanged, err := remapDecodedValueForDeletedImportPackageIndex(decoded, newAsset, importRemap, removeSet)
			if err != nil {
				return nil, fmt.Errorf("remap export[%d] property %s package indices: %w", i+1, tag.Name.Display(oldAsset.Names), err)
			}
			tagStart := tag.Offset
			tagEnd := noneStart
			if j+1 < len(parsed.Properties) {
				tagEnd = parsed.Properties[j+1].Offset
			}
			if !valueChanged {
				tagBlob = append(tagBlob, oldAsset.Raw.Bytes[tagStart:tagEnd]...)
				continue
			}
			typeTree, err := buildTypeTree(tag.TypeNodes, newAsset.Names)
			if err != nil {
				return nil, fmt.Errorf("build export[%d] property %s type tree: %w", i+1, tag.Name.Display(oldAsset.Names), err)
			}
			valueBytes, boolValue, err := encodePropertyValue(newAsset, typeTree, remappedValue, order)
			if err != nil {
				return nil, fmt.Errorf("encode export[%d] property %s: %w", i+1, tag.Name.Display(oldAsset.Names), err)
			}
			tagBytes, _, err := serializePropertyTag(newAsset, tag, valueBytes, boolValue, order)
			if err != nil {
				return nil, fmt.Errorf("serialize export[%d] property %s: %w", i+1, tag.Name.Display(oldAsset.Names), err)
			}
			if !bytes.Equal(tagBytes, oldAsset.Raw.Bytes[tagStart:tagEnd]) || valueChanged {
				propsChanged = true
			}
			tagBlob = append(tagBlob, tagBytes...)
		}
		oldPayload := oldAsset.Raw.Bytes[oldStart:oldEnd]

		tailStart := parsed.EndOffset - oldStart
		tailChanged := false
		if tailStart >= 0 && tailStart < len(oldPayload) {
			serializedPkgRemap := make(map[int32]int32, len(importRemap))
			serializedRemoved := make(map[int32]bool, len(removeSet))
			for oldIdx := range removeSet {
				serializedRemoved[int32(-(oldIdx + 1))] = true
			}
			for oldIdx, newIdx := range importRemap {
				oldPkg := int32(-(oldIdx + 1))
				newPkg := int32(-(newIdx + 1))
				if oldPkg != newPkg {
					serializedPkgRemap[oldPkg] = newPkg
				}
			}
			if len(serializedPkgRemap) > 0 || len(serializedRemoved) > 0 {
				tail := append([]byte(nil), oldPayload[tailStart:]...)
				for pos := 0; pos+4 <= len(tail); pos++ {
					cur := int32(order.Uint32(tail[pos : pos+4]))
					if serializedRemoved[cur] {
						return nil, fmt.Errorf("export[%d] tail still references removed import %d", i+1, -cur)
					}
					replacement, ok := serializedPkgRemap[cur]
					if !ok || replacement == cur {
						continue
					}
					order.PutUint32(tail[pos:pos+4], uint32(replacement))
					tailChanged = true
					pos += 3
				}
				if tailChanged {
					copy(oldPayload[tailStart:], tail)
				}
			}
		}

		if !propsChanged && !tailChanged {
			continue
		}

		var newPayload []byte
		if propsChanged {
			noneBytes := oldAsset.Raw.Bytes[noneStart:parsed.EndOffset]
			trailing := oldAsset.Raw.Bytes[parsed.EndOffset:propertyEnd]
			newPropertyRegion := make([]byte, 0, len(tagBlob)+len(noneBytes)+len(trailing))
			newPropertyRegion = append(newPropertyRegion, tagBlob...)
			newPropertyRegion = append(newPropertyRegion, noneBytes...)
			newPropertyRegion = append(newPropertyRegion, trailing...)

			relStart := propertyStart - oldStart
			relEnd := propertyEnd - oldStart
			newPayload = make([]byte, 0, len(oldPayload)+(len(newPropertyRegion)-(propertyEnd-propertyStart)))
			newPayload = append(newPayload, oldPayload[:relStart]...)
			newPayload = append(newPayload, newPropertyRegion...)
			newPayload = append(newPayload, oldPayload[relEnd:]...)
		} else {
			newPayload = append([]byte(nil), oldPayload...)
		}

		mutation := ExportMutation{ExportIndex: i, Payload: newPayload}
		propertyDelta := len(newPayload) - len(oldPayload)
		if propertyDelta != 0 && !oldAsset.Summary.UsesUnversionedPropertySerialization() && oldAsset.Summary.FileVersionUE5 >= ue5ScriptSerializationOffset {
			oldStartRel := oldExp.ScriptSerializationStartOffset
			oldEndRel := oldExp.ScriptSerializationEndOffset
			if oldEndRel >= oldStartRel {
				rangeStartRel := int64(propertyStart - oldStart)
				rangeEndRel := int64(propertyEnd - oldStart)
				if oldStartRel == rangeStartRel && oldEndRel == rangeEndRel {
					mutation.UpdateScript = true
					mutation.ScriptStartRel = oldStartRel
					mutation.ScriptEndRel = oldEndRel + int64(propertyDelta)
				}
			}
		}
		mutations = append(mutations, mutation)
	}
	return mutations, nil
}

func remapPropertyTagNameRefs(tag uasset.PropertyTag, indexRemap map[int32]int32, newNames []uasset.NameEntry) (uasset.PropertyTag, bool, error) {
	out := tag
	changed := false

	ref, refChanged, err := remapNameRefIndex(tag.Name, indexRemap, newNames)
	if err != nil {
		return out, false, err
	}
	out.Name = ref
	changed = changed || refChanged

	if len(tag.TypeNodes) > 0 {
		out.TypeNodes = append(make([]uasset.PropertyTypeNode, 0, len(tag.TypeNodes)), tag.TypeNodes...)
		for i, node := range out.TypeNodes {
			nextRef, nextChanged, err := remapNameRefIndex(node.Name, indexRemap, newNames)
			if err != nil {
				return out, false, err
			}
			out.TypeNodes[i].Name = nextRef
			changed = changed || nextChanged
		}
	}
	return out, changed, nil
}

func remapNameRefIndex(ref uasset.NameRef, indexRemap map[int32]int32, newNames []uasset.NameEntry) (uasset.NameRef, bool, error) {
	if ref.Index < 0 {
		return ref, false, nil
	}
	newIdx, ok := indexRemap[ref.Index]
	if !ok || newIdx == ref.Index {
		return ref, false, nil
	}
	if newIdx < 0 || int(newIdx) >= len(newNames) {
		return ref, false, fmt.Errorf("remapped name index out of range: %d", newIdx)
	}
	ref.Index = newIdx
	return ref, true, nil
}

func remapDecodedValueForNameMap(value any, indexRemap map[int32]int32, newNames []uasset.NameEntry, fromDisplay, toDisplay string) (any, bool, error) {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		changed := false
		for key, item := range v {
			next, itemChanged, err := remapDecodedValueForNameMap(item, indexRemap, newNames, fromDisplay, toDisplay)
			if err != nil {
				return nil, false, err
			}
			out[key] = next
			changed = changed || itemChanged
		}

		if rawB64, ok := v["rawBase64"].(string); ok && rawB64 != "" {
			structType, _ := v["structType"].(string)
			rewritten, rawChanged, err := remapOpaqueStructRawBase64(rawB64, structType, indexRemap)
			if err != nil {
				return nil, false, err
			}
			if rawChanged {
				out["rawBase64"] = rewritten
				changed = true
			}
		}

		nameRaw, hasName := v["name"]
		if hasName {
			name, _ := nameRaw.(string)
			if idx, err := asInt64(v["index"]); err == nil {
				if newIdx, ok := indexRemap[int32(idx)]; ok {
					if newIdx < 0 || int(newIdx) >= len(newNames) {
						return nil, false, fmt.Errorf("remapped decoded name index out of range: %d", newIdx)
					}
					newName := newNames[newIdx].Value
					if int32(idx) != newIdx || name != newName {
						out["index"] = newIdx
						out["name"] = newName
						changed = true
					}
				} else if name != "" {
					if resolved := findNameIndex(newNames, name); resolved >= 0 && int32(resolved) != int32(idx) {
						out["index"] = int32(resolved)
						changed = true
					}
				}
			} else if name != "" {
				if resolved := findNameIndex(newNames, name); resolved >= 0 {
					out["index"] = int32(resolved)
					changed = true
				}
			}
		}
		return out, changed, nil
	case []any:
		out := make([]any, len(v))
		changed := false
		for i, item := range v {
			next, itemChanged, err := remapDecodedValueForNameMap(item, indexRemap, newNames, fromDisplay, toDisplay)
			if err != nil {
				return nil, false, err
			}
			out[i] = next
			changed = changed || itemChanged
		}
		return out, changed, nil
	case []map[string]any:
		out := make([]map[string]any, len(v))
		changed := false
		for i, item := range v {
			next, itemChanged, err := remapDecodedValueForNameMap(item, indexRemap, newNames, fromDisplay, toDisplay)
			if err != nil {
				return nil, false, err
			}
			nextMap, ok := next.(map[string]any)
			if !ok {
				return nil, false, fmt.Errorf("remapped array item has invalid type %T", next)
			}
			out[i] = nextMap
			changed = changed || itemChanged
		}
		return out, changed, nil
	case string:
		if fromDisplay != "" && fromDisplay != toDisplay && v == fromDisplay {
			return toDisplay, true, nil
		}
		return v, false, nil
	default:
		return value, false, nil
	}
}

func remapDecodedValueForPackageIndex(value any, asset *uasset.Asset, importRemap map[int]int) (any, bool, error) {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		changed := false
		for key, item := range v {
			next, itemChanged, err := remapDecodedValueForPackageIndex(item, asset, importRemap)
			if err != nil {
				return nil, false, err
			}
			out[key] = next
			changed = changed || itemChanged
		}
		if resolved, ok := v["resolved"].(string); ok {
			if idx, err := asInt64(v["index"]); err == nil {
				newIdx := remapImportPackageIndex(uasset.PackageIndex(int32(idx)), importRemap)
				if int32(newIdx) != int32(idx) {
					out["index"] = int32(newIdx)
					out["resolved"] = asset.ParseIndex(newIdx)
					changed = true
				} else {
					out["resolved"] = resolved
				}
			}
		}
		return out, changed, nil
	case []any:
		out := make([]any, len(v))
		changed := false
		for i, item := range v {
			next, itemChanged, err := remapDecodedValueForPackageIndex(item, asset, importRemap)
			if err != nil {
				return nil, false, err
			}
			out[i] = next
			changed = changed || itemChanged
		}
		return out, changed, nil
	case []map[string]any:
		out := make([]map[string]any, len(v))
		changed := false
		for i, item := range v {
			next, itemChanged, err := remapDecodedValueForPackageIndex(item, asset, importRemap)
			if err != nil {
				return nil, false, err
			}
			nextMap, ok := next.(map[string]any)
			if !ok {
				return nil, false, fmt.Errorf("remapped array item has invalid type %T", next)
			}
			out[i] = nextMap
			changed = changed || itemChanged
		}
		return out, changed, nil
	default:
		return value, false, nil
	}
}

func remapDecodedValueForDeletedImportPackageIndex(value any, asset *uasset.Asset, importRemap map[int]int, removeSet map[int]bool) (any, bool, error) {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		changed := false
		for key, item := range v {
			next, itemChanged, err := remapDecodedValueForDeletedImportPackageIndex(item, asset, importRemap, removeSet)
			if err != nil {
				return nil, false, err
			}
			out[key] = next
			changed = changed || itemChanged
		}
		if resolved, ok := v["resolved"].(string); ok {
			if idx, err := asInt64(v["index"]); err == nil {
				newIdx, err := remapImportPackageIndexForDeletion(uasset.PackageIndex(int32(idx)), importRemap, removeSet)
				if err != nil {
					return nil, false, err
				}
				if int32(newIdx) != int32(idx) {
					out["index"] = int32(newIdx)
					out["resolved"] = asset.ParseIndex(newIdx)
					changed = true
				} else {
					out["resolved"] = resolved
				}
			}
		}
		return out, changed, nil
	case []any:
		out := make([]any, len(v))
		changed := false
		for i, item := range v {
			next, itemChanged, err := remapDecodedValueForDeletedImportPackageIndex(item, asset, importRemap, removeSet)
			if err != nil {
				return nil, false, err
			}
			out[i] = next
			changed = changed || itemChanged
		}
		return out, changed, nil
	case []map[string]any:
		out := make([]map[string]any, len(v))
		changed := false
		for i, item := range v {
			next, itemChanged, err := remapDecodedValueForDeletedImportPackageIndex(item, asset, importRemap, removeSet)
			if err != nil {
				return nil, false, err
			}
			nextMap, ok := next.(map[string]any)
			if !ok {
				return nil, false, fmt.Errorf("remapped array item has invalid type %T", next)
			}
			out[i] = nextMap
			changed = changed || itemChanged
		}
		return out, changed, nil
	default:
		return value, false, nil
	}
}

func remapOpaqueStructRawBase64(rawB64, structType string, indexRemap map[int32]int32) (string, bool, error) {
	if rawB64 == "" {
		return "", false, nil
	}
	switch strings.ToLower(strings.TrimSpace(structType)) {
	case "edgraphpintype":
		raw, err := decodeBase64String(rawB64)
		if err != nil {
			return "", false, fmt.Errorf("decode %s rawBase64: %w", structType, err)
		}
		rewritten, changed := remapOpaqueNameRefPairsLE(raw, indexRemap)
		if !changed {
			return rawB64, false, nil
		}
		return encodeBase64Bytes(rewritten), true, nil
	default:
		return rawB64, false, nil
	}
}

// RemapOpaqueExportNameRefs performs an opaque scan of an export payload,
// remapping all NameRef-like int32 pairs according to the index remap.
func RemapOpaqueExportNameRefs(payload []byte, indexRemap map[int32]int32) ([]byte, bool) {
	return remapOpaqueNameRefPairsLESkip(payload, indexRemap, -1)
}

func remapOpaqueNameRefPairsLE(raw []byte, indexRemap map[int32]int32) ([]byte, bool) {
	return remapOpaqueNameRefPairsLESkip(raw, indexRemap, -1)
}

// remapOpaqueNameRefPairsLEAnyNumber is like remapOpaqueNameRefPairsLE but
// also matches NameRef pairs where Number != 0 (e.g., instanced names like
// "Image_22" → Name="Image" Number=23). This is needed for tail/bytecode
// regions that contain instanced name references.
func remapOpaqueNameRefPairsLEAnyNumber(raw []byte, indexRemap map[int32]int32) ([]byte, bool) {
	return remapOpaqueNameRefPairsLEAnyNumberSkipBlocked(raw, indexRemap, nil)
}

func remapOpaqueNameRefPairsLEAnyNumberSkipBlocked(raw []byte, indexRemap map[int32]int32, blocked map[int]struct{}) ([]byte, bool) {
	if len(raw) < 8 || len(indexRemap) == 0 {
		return append([]byte(nil), raw...), false
	}
	out := append([]byte(nil), raw...)
	changed := false
	for off := 0; off+8 <= len(out); off++ {
		if _, ok := blocked[off]; ok {
			continue
		}
		idx := int32(binary.LittleEndian.Uint32(out[off : off+4]))
		if idx < 0 {
			continue
		}
		num := int32(binary.LittleEndian.Uint32(out[off+4 : off+8]))
		if num < 0 || num > 10000 {
			continue
		}
		newIdx, ok := indexRemap[idx]
		if !ok || newIdx == idx {
			continue
		}
		binary.LittleEndian.PutUint32(out[off:off+4], uint32(newIdx))
		changed = true
		off += 7
	}
	return out, changed
}

func remapOpaqueNameRefPairsLESkip(raw []byte, indexRemap map[int32]int32, skipIndex int32) ([]byte, bool) {
	return remapOpaqueNameRefPairsLESkipBlocked(raw, indexRemap, skipIndex, nil)
}

func remapOpaqueNameRefPairsLESkipBlocked(raw []byte, indexRemap map[int32]int32, skipIndex int32, blocked map[int]struct{}) ([]byte, bool) {
	if len(raw) < 8 || len(indexRemap) == 0 {
		return append([]byte(nil), raw...), false
	}
	out := append([]byte(nil), raw...)
	changed := false
	for off := 0; off+8 <= len(out); off++ {
		if _, ok := blocked[off]; ok {
			continue
		}
		idx := int32(binary.LittleEndian.Uint32(out[off : off+4]))
		if idx < 0 || idx == skipIndex {
			continue
		}
		num := int32(binary.LittleEndian.Uint32(out[off+4 : off+8]))
		if num != 0 {
			continue
		}
		newIdx, ok := indexRemap[idx]
		if !ok || newIdx == idx {
			continue
		}
		binary.LittleEndian.PutUint32(out[off:off+4], uint32(newIdx))
		changed = true
		off += 7
	}
	return out, changed
}

func remapOpaqueSpecificNameRefPairLE(raw []byte, oldIndex, newIndex int32) ([]byte, bool) {
	rewritten, _, changed := remapOpaqueSpecificNameRefPairLEPositions(raw, oldIndex, newIndex)
	return rewritten, changed
}

func remapOpaqueSpecificNameRefPairLEPositionsSkipBlocked(raw []byte, oldIndex, newIndex int32, blocked map[int]struct{}) ([]byte, map[int]struct{}, bool) {
	if len(blocked) == 0 {
		return remapOpaqueSpecificNameRefPairLEPositions(raw, oldIndex, newIndex)
	}
	if len(raw) < 8 || oldIndex < 0 || newIndex < 0 || oldIndex == newIndex {
		return append([]byte(nil), raw...), nil, false
	}
	out := append([]byte(nil), raw...)
	changed := false
	offsets := map[int]struct{}{}
	for off := 0; off+8 <= len(out); off++ {
		if _, skip := blocked[off]; skip {
			continue
		}
		if int32(binary.LittleEndian.Uint32(out[off:off+4])) != oldIndex {
			continue
		}
		if binary.LittleEndian.Uint32(out[off+4:off+8]) != 0 {
			continue
		}
		binary.LittleEndian.PutUint32(out[off:off+4], uint32(newIndex))
		offsets[off] = struct{}{}
		changed = true
		off += 7
	}
	return out, offsets, changed
}

func remapOpaqueSpecificNameRefPairLEPositions(raw []byte, oldIndex, newIndex int32) ([]byte, map[int]struct{}, bool) {
	if len(raw) < 8 || oldIndex < 0 || newIndex < 0 || oldIndex == newIndex {
		return append([]byte(nil), raw...), nil, false
	}
	out := append([]byte(nil), raw...)
	changed := false
	offsets := map[int]struct{}{}
	for off := 0; off+8 <= len(out); off++ {
		if int32(binary.LittleEndian.Uint32(out[off:off+4])) != oldIndex {
			continue
		}
		if binary.LittleEndian.Uint32(out[off+4:off+8]) != 0 {
			continue
		}
		binary.LittleEndian.PutUint32(out[off:off+4], uint32(newIndex))
		offsets[off] = struct{}{}
		changed = true
		off += 7
	}
	return out, offsets, changed
}

func collectOpaqueExportIndexLikeZeroNumberOffsets(raw []byte, maxSerializedExport int32, className string, order binary.ByteOrder) map[int]struct{} {
	blocked := map[int]struct{}{}
	if len(raw) < 8 || maxSerializedExport <= 0 {
		return blocked
	}
	switch {
	case strings.EqualFold(className, "K2Node_Event"), strings.EqualFold(className, "WidgetBlueprintGeneratedClass"):
	default:
		return blocked
	}
	for off := 0; off+8 <= len(raw); off++ {
		idx := int32(order.Uint32(raw[off : off+4]))
		if idx <= 0 || idx > maxSerializedExport {
			continue
		}
		num := int32(order.Uint32(raw[off+4 : off+8]))
		if num != 0 {
			continue
		}
		blocked[off] = struct{}{}
	}
	return blocked
}

func collectASCIIFStringDataOffsets(raw []byte) map[int]struct{} {
	blocked := map[int]struct{}{}
	for off := 0; off+5 <= len(raw); off++ {
		n := int(int32(binary.LittleEndian.Uint32(raw[off : off+4])))
		if n <= 1 || n > 256 || off+4+n > len(raw) {
			continue
		}
		body := raw[off+4 : off+4+n-1]
		if raw[off+4+n-1] != 0 || len(body) == 0 {
			continue
		}
		printable := true
		for _, b := range body {
			if b < 0x20 || b > 0x7e {
				printable = false
				break
			}
		}
		if !printable {
			continue
		}
		for i := off + 4; i < off+4+n; i++ {
			blocked[i] = struct{}{}
		}
		off += 3 + n
	}
	return blocked
}

func decodeBase64String(raw string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func encodeBase64Bytes(raw []byte) string {
	return base64.StdEncoding.EncodeToString(raw)
}

func isBlueprintLikeExport(asset *uasset.Asset, exp uasset.ExportEntry) bool {
	if asset == nil {
		return false
	}
	return strings.Contains(strings.ToLower(asset.ResolveClassName(exp)), "blueprint")
}

func replaceEncodedFStringLiterals(payload []byte, order binary.ByteOrder, from, to string) ([]byte, bool) {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" || from == to {
		return append([]byte(nil), payload...), false
	}
	oldBytes := encodeFStringLiteral(from, order)
	newBytes := encodeFStringLiteral(to, order)
	if len(oldBytes) == 0 {
		return append([]byte(nil), payload...), false
	}
	out := bytes.ReplaceAll(payload, oldBytes, newBytes)
	if bytes.Equal(out, payload) {
		return append([]byte(nil), payload...), false
	}
	return out, true
}

func encodeFStringLiteral(value string, order binary.ByteOrder) []byte {
	w := newByteWriter(order, len(value)+8)
	w.writeFString(value)
	return w.bytes()
}

func patchNameRefIndexAt(data []byte, pos int, indexRemap map[int32]int32, order binary.ByteOrder, nameCount int) (bool, error) {
	idx, err := readInt32At(data, pos, order)
	if err != nil {
		return false, err
	}
	if idx < 0 {
		return false, nil
	}
	newIdx, ok := indexRemap[idx]
	if !ok || newIdx == idx {
		return false, nil
	}
	if newIdx < 0 || int(newIdx) >= nameCount {
		return false, fmt.Errorf("remapped name index out of range: %d", newIdx)
	}
	if err := writeInt32At(data, pos, newIdx, order); err != nil {
		return false, err
	}
	return true, nil
}

func rewriteOpaqueNameRefsInRange(data []byte, start, end int, indexRemap map[int32]int32) (bool, error) {
	if start <= 0 || end <= start {
		return false, nil
	}
	if end > len(data) {
		return false, fmt.Errorf("opaque range out of bounds: %d..%d (size=%d)", start, end, len(data))
	}
	rewritten, changed := remapOpaqueNameRefPairsLE(data[start:end], indexRemap)
	if !changed {
		return false, nil
	}
	copy(data[start:end], rewritten)
	return true, nil
}

func scanSoftObjectPathNameRefPositions(data []byte, asset *uasset.Asset) ([]int, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if asset.Summary.SoftObjectPathsCount <= 0 || asset.Summary.SoftObjectPathsOffset <= 0 || !asset.Summary.SupportsSoftObjectPathListInSummary() {
		return nil, nil
	}
	start := int(asset.Summary.SoftObjectPathsOffset)
	end := int(nextKnownOffsetWithinFile(asset, int64(asset.Summary.SoftObjectPathsOffset)))
	if start < 0 || start > len(data) || end < start || end > len(data) {
		return nil, fmt.Errorf("soft object path range out of bounds: %d..%d (size=%d)", start, end, len(data))
	}

	r := uasset.NewByteReaderWithByteSwapping(data[start:end], asset.Summary.UsesByteSwappedSerialization())
	positions := make([]int, 0, int(asset.Summary.SoftObjectPathsCount)*2)
	for i := int32(0); i < asset.Summary.SoftObjectPathsCount; i++ {
		pkgPos := start + r.Offset()
		if _, err := r.ReadNameRef(len(asset.Names)); err != nil {
			return nil, fmt.Errorf("soft object path[%d] package name: %w", i, err)
		}
		positions = append(positions, pkgPos)

		assetPos := start + r.Offset()
		if _, err := r.ReadNameRef(len(asset.Names)); err != nil {
			return nil, fmt.Errorf("soft object path[%d] asset name: %w", i, err)
		}
		positions = append(positions, assetPos)

		if _, err := r.ReadSoftObjectSubPath(); err != nil {
			return nil, fmt.Errorf("soft object path[%d] sub path: %w", i, err)
		}
	}
	return positions, nil
}

func nextKnownOffsetWithinFile(asset *uasset.Asset, start int64) int64 {
	if asset == nil {
		return 0
	}
	fileSize := int64(len(asset.Raw.Bytes))
	end := fileSize
	for _, off := range []int64{
		int64(asset.Summary.NameOffset),
		int64(asset.Summary.SoftObjectPathsOffset),
		int64(asset.Summary.GatherableTextDataOffset),
		int64(asset.Summary.MetaDataOffset),
		int64(asset.Summary.ImportOffset),
		int64(asset.Summary.ExportOffset),
		int64(asset.Summary.CellImportOffset),
		int64(asset.Summary.CellExportOffset),
		int64(asset.Summary.DependsOffset),
		int64(asset.Summary.SoftPackageReferencesOffset),
		int64(asset.Summary.SearchableNamesOffset),
		int64(asset.Summary.ThumbnailTableOffset),
		int64(asset.Summary.ImportTypeHierarchiesOffset),
		int64(asset.Summary.AssetRegistryDataOffset),
		int64(asset.Summary.PreloadDependencyOffset),
		int64(asset.Summary.DataResourceOffset),
		asset.Summary.BulkDataStartOffset,
		asset.Summary.PayloadTOCOffset,
		int64(asset.Summary.TotalHeaderSize),
		fileSize,
	} {
		if off > start && off <= fileSize && off < end {
			end = off
		}
	}
	return end
}

func scanImportNameRefPositions(data []byte, asset *uasset.Asset) ([]importNameRefPatch, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if asset.Summary.ImportOffset < 0 || int(asset.Summary.ImportOffset) > len(data) {
		return nil, fmt.Errorf("import offset out of range: %d", asset.Summary.ImportOffset)
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}
	r := newByteCodec(data, order)
	if err := r.seek(int(asset.Summary.ImportOffset)); err != nil {
		return nil, err
	}

	fields := make([]importNameRefPatch, 0, len(asset.Imports))
	for i := 0; i < len(asset.Imports); i++ {
		patch := importNameRefPatch{packageNamePos: -1}
		patch.classPackagePos = r.off
		if err := r.skip(8); err != nil {
			return nil, fmt.Errorf("import[%d] class package: %w", i+1, err)
		}
		patch.classNamePos = r.off
		if err := r.skip(8); err != nil {
			return nil, fmt.Errorf("import[%d] class name: %w", i+1, err)
		}
		if err := r.skip(4); err != nil {
			return nil, fmt.Errorf("import[%d] outer index: %w", i+1, err)
		}
		patch.objectNamePos = r.off
		if err := r.skip(8); err != nil {
			return nil, fmt.Errorf("import[%d] object name: %w", i+1, err)
		}
		if !asset.Summary.IsEditorOnlyFiltered() {
			patch.packageNamePos = r.off
			if err := r.skip(8); err != nil {
				return nil, fmt.Errorf("import[%d] package name: %w", i+1, err)
			}
		}
		if asset.Summary.FileVersionUE5 >= ue5OptionalResources {
			if err := r.skip(4); err != nil {
				return nil, fmt.Errorf("import[%d] optional flag: %w", i+1, err)
			}
		}
		fields = append(fields, patch)
	}
	return fields, nil
}

func scanExportObjectNamePositions(data []byte, asset *uasset.Asset) ([]exportObjectNamePatch, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if asset.Summary.ExportOffset < 0 || int(asset.Summary.ExportOffset) > len(data) {
		return nil, fmt.Errorf("export offset out of range: %d", asset.Summary.ExportOffset)
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}
	r := newByteCodec(data, order)
	if err := r.seek(int(asset.Summary.ExportOffset)); err != nil {
		return nil, err
	}

	fields := make([]exportObjectNamePatch, 0, len(asset.Exports))
	for i := 0; i < len(asset.Exports); i++ {
		if err := r.skip(4 * 4); err != nil {
			return nil, fmt.Errorf("export[%d] class/super/template/outer: %w", i+1, err)
		}
		patch := exportObjectNamePatch{objectNamePos: r.off}
		if err := r.skip(8); err != nil {
			return nil, fmt.Errorf("export[%d] object name: %w", i+1, err)
		}
		if err := r.skip(4); err != nil {
			return nil, fmt.Errorf("export[%d] object flags: %w", i+1, err)
		}
		if err := r.skip(8 * 2); err != nil {
			return nil, fmt.Errorf("export[%d] serial fields: %w", i+1, err)
		}
		if err := r.skip(4 * 3); err != nil {
			return nil, fmt.Errorf("export[%d] bool flags: %w", i+1, err)
		}
		if asset.Summary.FileVersionUE5 < ue5RemoveObjectExportPkgGUID {
			if err := r.skip(16); err != nil {
				return nil, fmt.Errorf("export[%d] package guid: %w", i+1, err)
			}
		}
		if asset.Summary.FileVersionUE5 >= ue5TrackObjectExportInherited {
			if err := r.skip(4); err != nil {
				return nil, fmt.Errorf("export[%d] inherited flag: %w", i+1, err)
			}
		}
		if err := r.skip(4); err != nil {
			return nil, fmt.Errorf("export[%d] package flags: %w", i+1, err)
		}
		if err := r.skip(4 * 2); err != nil {
			return nil, fmt.Errorf("export[%d] load flags: %w", i+1, err)
		}
		if asset.Summary.FileVersionUE5 >= ue5OptionalResources {
			if err := r.skip(4); err != nil {
				return nil, fmt.Errorf("export[%d] public hash flag: %w", i+1, err)
			}
		}
		if err := r.skip(4 * 5); err != nil {
			return nil, fmt.Errorf("export[%d] dependency header: %w", i+1, err)
		}
		if !asset.Summary.UsesUnversionedPropertySerialization() && asset.Summary.FileVersionUE5 >= ue5ScriptSerializationOffset {
			if err := r.skip(8 * 2); err != nil {
				return nil, fmt.Errorf("export[%d] script offsets: %w", i+1, err)
			}
		}
		fields = append(fields, patch)
	}
	return fields, nil
}
