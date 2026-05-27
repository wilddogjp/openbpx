package edit

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

// ExportInsertEntry is one inserted export header plus its serialized payload.
type ExportInsertEntry struct {
	Header  uasset.ExportEntry
	Payload []byte
}

// InsertExportEntries inserts export-map entries at zero-based position,
// expands DependsMap with empty entries, remaps existing positive export
// references in headers and tagged properties, and rebuilds the export-serial
// region in final export order.
func InsertExportEntries(asset *uasset.Asset, insertAt int, entries []ExportInsertEntry) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if len(asset.Raw.Bytes) == 0 {
		return nil, fmt.Errorf("asset has no raw bytes")
	}
	if len(entries) == 0 {
		return append([]byte(nil), asset.Raw.Bytes...), nil
	}
	if insertAt < 0 || insertAt > len(asset.Exports) {
		return nil, fmt.Errorf("insert position out of range: %d", insertAt)
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	exportRemap := buildInsertedExportIndexRemap(len(asset.Exports), insertAt, len(entries))

	workingBytes, err := rewriteExistingExportPayloadPackageIndices(asset, exportRemap)
	if err != nil {
		return nil, fmt.Errorf("rewrite existing export payload refs: %w", err)
	}
	workingAsset, err := uasset.ParseBytes(workingBytes, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse payload-remapped asset: %w", err)
	}

	serializedEntries, err := buildInsertedExportMapEntries(workingAsset, insertAt, entries, exportRemap)
	if err != nil {
		return nil, err
	}
	exportStart := int64(workingAsset.Summary.ExportOffset)
	exportEnd, err := findExportMapEndOffset(workingAsset, workingAsset.Raw.Bytes)
	if err != nil {
		return nil, err
	}
	tempExportMapBytes := encodeExportMapEntries(serializedEntries, order, workingAsset.Summary)
	exportMapDelta := len(tempExportMapBytes) - int(exportEnd-exportStart)
	if exportMapDelta != 0 {
		for i := range serializedEntries {
			if serializedEntries[i].SerialOffset <= 0 {
				continue
			}
			serializedEntries[i].SerialOffset += int64(exportMapDelta)
		}
	}
	finalExportMapBytes := encodeExportMapEntries(serializedEntries, order, workingAsset.Summary)
	exportRewriteAsset := *workingAsset
	exportRewriteAsset.Exports = nil
	workingBytes, err = RewriteRawRange(&exportRewriteAsset, exportStart, exportEnd, finalExportMapBytes)
	if err != nil {
		return nil, fmt.Errorf("rewrite export map: %w", err)
	}
	if err := patchSummaryExportCounts(workingBytes, workingAsset, int32(len(serializedEntries)), order); err != nil {
		return nil, err
	}
	workingAsset, err = uasset.ParseBytes(workingBytes, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse export-inserted asset: %w", err)
	}

	if workingAsset.Summary.DependsOffset > 0 {
		oldDepends, _, _, err := parseDependsMapEntries(asset)
		if err != nil {
			return nil, fmt.Errorf("parse original depends map: %w", err)
		}
		newDepends := make([][]uasset.PackageIndex, 0, len(oldDepends)+len(entries))
		for i := 0; i < insertAt; i++ {
			newDepends = append(newDepends, remapPackageIndexSlice(oldDepends[i], exportRemap))
		}
		for range entries {
			newDepends = append(newDepends, nil)
		}
		for i := insertAt; i < len(oldDepends); i++ {
			newDepends = append(newDepends, remapPackageIndexSlice(oldDepends[i], exportRemap))
		}
		depStart := int64(workingAsset.Summary.DependsOffset)
		depEnd, err := findDependsMapEndOffset(workingAsset, workingAsset.Raw.Bytes)
		if err != nil {
			return nil, err
		}
		workingBytes, err = RewriteRawRange(workingAsset, depStart, depEnd, encodeDependsMapEntries(newDepends, order))
		if err != nil {
			return nil, fmt.Errorf("rewrite depends map: %w", err)
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, uasset.ParseOptions{KeepUnknown: true})
		if err != nil {
			return nil, fmt.Errorf("reparse depends-expanded asset: %w", err)
		}
	}

	fieldPositions, err := scanExportFieldPositions(workingAsset.Raw.Bytes, workingAsset)
	if err != nil {
		return nil, fmt.Errorf("scan export fields: %w", err)
	}
	if len(fieldPositions) != len(serializedEntries) {
		return nil, fmt.Errorf("export field scan mismatch: got %d want %d", len(fieldPositions), len(serializedEntries))
	}

	serialLayout, err := buildInsertedExportSerialLayout(workingAsset, insertAt, entries)
	if err != nil {
		return nil, err
	}
	workingBytes, err = rewriteRawRangeAllowExportOverlap(workingAsset, serialLayout.start, serialLayout.end, serialLayout.blob)
	if err != nil {
		return nil, fmt.Errorf("rewrite export serial data: %w", err)
	}

	for exportIdx := range serializedEntries {
		field := fieldPositions[exportIdx]
		if err := writeInt64At(workingBytes, field.serialSizePos, serialLayout.sizes[exportIdx], order); err != nil {
			return nil, fmt.Errorf("patch export[%d] serial size: %w", exportIdx+1, err)
		}
		if err := writeInt64At(workingBytes, field.serialOffsetPos, serialLayout.offsets[exportIdx], order); err != nil {
			return nil, fmt.Errorf("patch export[%d] serial offset: %w", exportIdx+1, err)
		}
		if exportIdx < insertAt || exportIdx >= insertAt+len(entries) {
			continue
		}
		entry := entries[exportIdx-insertAt]
		if field.scriptStartPos >= 0 {
			if err := writeInt64At(workingBytes, field.scriptStartPos, entry.Header.ScriptSerializationStartOffset, order); err != nil {
				return nil, fmt.Errorf("patch inserted export[%d] script start: %w", exportIdx+1, err)
			}
		}
		if field.scriptEndPos >= 0 {
			if err := writeInt64At(workingBytes, field.scriptEndPos, entry.Header.ScriptSerializationEndOffset, order); err != nil {
				return nil, fmt.Errorf("patch inserted export[%d] script end: %w", exportIdx+1, err)
			}
		}
	}

	if err := FinalizePackageBytes(workingBytes, asset.Summary.FileVersionUE5); err != nil {
		return nil, fmt.Errorf("finalize package bytes: %w", err)
	}
	return workingBytes, nil
}

// ReorderExports rewrites the export map and serial region to the provided
// zero-based export order, remapping positive export references in headers,
// tagged properties, DependsMap, and known opaque export tails.
func ReorderExports(asset *uasset.Asset, newOrder []int) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if len(newOrder) != len(asset.Exports) {
		return nil, fmt.Errorf("reorder length mismatch: got %d want %d", len(newOrder), len(asset.Exports))
	}
	seen := make([]bool, len(asset.Exports))
	for _, oldIdx := range newOrder {
		if oldIdx < 0 || oldIdx >= len(asset.Exports) {
			return nil, fmt.Errorf("reorder export index out of range: %d", oldIdx)
		}
		if seen[oldIdx] {
			return nil, fmt.Errorf("reorder contains duplicate export index: %d", oldIdx)
		}
		seen[oldIdx] = true
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	exportRemap := buildExportReorderMap(newOrder)
	workingBytes, err := rewriteExistingExportPayloadPackageIndices(asset, exportRemap)
	if err != nil {
		return nil, fmt.Errorf("rewrite existing export payload refs: %w", err)
	}
	workingAsset, err := uasset.ParseBytes(workingBytes, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse payload-remapped asset: %w", err)
	}

	finalEntries := make([]uasset.ExportEntry, len(newOrder))
	tempEntries := make([]uasset.ExportEntry, len(newOrder))
	for newIdx, oldIdx := range newOrder {
		finalEntries[newIdx] = remapExportHeaderPackageIndices(workingAsset.Exports[oldIdx], exportRemap)
		tempEntries[newIdx] = finalEntries[newIdx]
		tempEntries[newIdx].ScriptSerializationStartOffset = 0
		tempEntries[newIdx].ScriptSerializationEndOffset = 0
	}
	exportStart := int64(workingAsset.Summary.ExportOffset)
	exportEnd, err := findExportMapEndOffset(workingAsset, workingAsset.Raw.Bytes)
	if err != nil {
		return nil, err
	}
	tempExportMapBytes := encodeExportMapEntries(tempEntries, order, workingAsset.Summary)
	exportMapDelta := len(tempExportMapBytes) - int(exportEnd-exportStart)
	if exportMapDelta != 0 {
		for i := range finalEntries {
			if finalEntries[i].SerialOffset <= 0 {
				continue
			}
			finalEntries[i].SerialOffset += int64(exportMapDelta)
			tempEntries[i].SerialOffset = finalEntries[i].SerialOffset
		}
	}
	reorderedExportMapBytes := encodeExportMapEntries(tempEntries, order, workingAsset.Summary)
	exportRewriteAsset := *workingAsset
	exportRewriteAsset.Exports = nil
	workingBytes, err = RewriteRawRange(&exportRewriteAsset, exportStart, exportEnd, reorderedExportMapBytes)
	if err != nil {
		return nil, fmt.Errorf("rewrite export map: %w", err)
	}
	workingAsset, err = uasset.ParseBytes(workingBytes, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse export-reordered asset: %w", err)
	}

	if workingAsset.Summary.DependsOffset > 0 {
		oldDepends, _, _, err := parseDependsMapEntries(asset)
		if err != nil {
			return nil, fmt.Errorf("parse original depends map: %w", err)
		}
		newDepends := make([][]uasset.PackageIndex, len(newOrder))
		for newIdx, oldIdx := range newOrder {
			newDepends[newIdx] = remapPackageIndexSlice(oldDepends[oldIdx], exportRemap)
		}
		depStart := int64(workingAsset.Summary.DependsOffset)
		depEnd, err := findDependsMapEndOffset(workingAsset, workingAsset.Raw.Bytes)
		if err != nil {
			return nil, err
		}
		workingBytes, err = RewriteRawRange(workingAsset, depStart, depEnd, encodeDependsMapEntries(newDepends, order))
		if err != nil {
			return nil, fmt.Errorf("rewrite depends map: %w", err)
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, uasset.ParseOptions{KeepUnknown: true})
		if err != nil {
			return nil, fmt.Errorf("reparse depends-reordered asset: %w", err)
		}
	}

	fieldPositions, err := scanExportFieldPositions(workingAsset.Raw.Bytes, workingAsset)
	if err != nil {
		return nil, fmt.Errorf("scan export fields: %w", err)
	}
	if len(fieldPositions) != len(finalEntries) {
		return nil, fmt.Errorf("export field scan mismatch: got %d want %d", len(fieldPositions), len(finalEntries))
	}

	serialLayout, err := buildReorderedExportSerialLayout(workingAsset)
	if err != nil {
		return nil, err
	}
	workingBytes, err = rewriteRawRangeAllowExportOverlap(workingAsset, serialLayout.start, serialLayout.end, serialLayout.blob)
	if err != nil {
		return nil, fmt.Errorf("rewrite export serial data: %w", err)
	}
	for exportIdx := range finalEntries {
		field := fieldPositions[exportIdx]
		if err := writeInt64At(workingBytes, field.serialSizePos, serialLayout.sizes[exportIdx], order); err != nil {
			return nil, fmt.Errorf("patch export[%d] serial size: %w", exportIdx+1, err)
		}
		if err := writeInt64At(workingBytes, field.serialOffsetPos, serialLayout.offsets[exportIdx], order); err != nil {
			return nil, fmt.Errorf("patch export[%d] serial offset: %w", exportIdx+1, err)
		}
		if field.scriptStartPos >= 0 {
			if err := writeInt64At(workingBytes, field.scriptStartPos, finalEntries[exportIdx].ScriptSerializationStartOffset, order); err != nil {
				return nil, fmt.Errorf("patch export[%d] script start: %w", exportIdx+1, err)
			}
		}
		if field.scriptEndPos >= 0 {
			if err := writeInt64At(workingBytes, field.scriptEndPos, finalEntries[exportIdx].ScriptSerializationEndOffset, order); err != nil {
				return nil, fmt.Errorf("patch export[%d] script end: %w", exportIdx+1, err)
			}
		}
	}
	if err := FinalizePackageBytes(workingBytes, asset.Summary.FileVersionUE5); err != nil {
		return nil, fmt.Errorf("finalize package bytes: %w", err)
	}
	return workingBytes, nil
}

func buildExportReorderMap(newOrder []int) map[int]int {
	remap := make(map[int]int, len(newOrder))
	for newIdx, oldIdx := range newOrder {
		remap[oldIdx] = newIdx
	}
	return remap
}

type exportSerialLayout struct {
	start   int64
	end     int64
	blob    []byte
	offsets []int64
	sizes   []int64
}

func buildInsertedExportSerialLayout(asset *uasset.Asset, insertAt int, entries []ExportInsertEntry) (*exportSerialLayout, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("entries is empty")
	}

	start, end, err := findContiguousExportSerialRegion(asset)
	if err != nil {
		return nil, err
	}

	finalCount := len(asset.Exports)
	offsets := make([]int64, finalCount)
	sizes := make([]int64, finalCount)
	blob := make([]byte, 0, int(end-start)+totalInsertPayloadSize(entries))
	cursor := start

	for finalIdx := 0; finalIdx < finalCount; finalIdx++ {
		var payload []byte
		if finalIdx >= insertAt && finalIdx < insertAt+len(entries) {
			payload = entries[finalIdx-insertAt].Payload
		} else {
			exp := asset.Exports[finalIdx]
			oldStart := int(exp.SerialOffset)
			oldEnd := int(exp.SerialOffset + exp.SerialSize)
			if oldStart < 0 || oldEnd < oldStart || oldEnd > len(asset.Raw.Bytes) {
				return nil, fmt.Errorf("export[%d] serial range out of bounds", finalIdx+1)
			}
			if exp.SerialSize == 0 {
				return nil, fmt.Errorf("export[%d] has no serial payload in the rebuilt layout", finalIdx+1)
			}
			payload = asset.Raw.Bytes[oldStart:oldEnd]
		}

		offsets[finalIdx] = cursor
		sizes[finalIdx] = int64(len(payload))
		blob = append(blob, payload...)
		cursor += int64(len(payload))
	}

	return &exportSerialLayout{
		start:   start,
		end:     end,
		blob:    blob,
		offsets: offsets,
		sizes:   sizes,
	}, nil
}

func buildReorderedExportSerialLayout(asset *uasset.Asset) (*exportSerialLayout, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}

	start, end, err := findContiguousExportSerialRegion(asset)
	if err != nil {
		return nil, err
	}

	offsets := make([]int64, len(asset.Exports))
	sizes := make([]int64, len(asset.Exports))
	blob := make([]byte, 0, int(end-start))
	cursor := start
	for exportIdx, exp := range asset.Exports {
		oldStart := int(exp.SerialOffset)
		oldEnd := int(exp.SerialOffset + exp.SerialSize)
		if oldStart < 0 || oldEnd < oldStart || oldEnd > len(asset.Raw.Bytes) {
			return nil, fmt.Errorf("export[%d] serial range out of bounds (%d..%d size=%d)", exportIdx+1, oldStart, oldEnd, len(asset.Raw.Bytes))
		}
		if exp.SerialSize == 0 {
			return nil, fmt.Errorf("export[%d] has no serial payload in the reordered layout", exportIdx+1)
		}
		payload := asset.Raw.Bytes[oldStart:oldEnd]
		offsets[exportIdx] = cursor
		sizes[exportIdx] = int64(len(payload))
		blob = append(blob, payload...)
		cursor += int64(len(payload))
	}

	return &exportSerialLayout{
		start:   start,
		end:     end,
		blob:    blob,
		offsets: offsets,
		sizes:   sizes,
	}, nil
}

func buildRemovedExportSerialLayout(asset *uasset.Asset, keepOrder []int) (*exportSerialLayout, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if len(keepOrder) == 0 {
		return nil, fmt.Errorf("keep order is empty")
	}

	start, end, err := findContiguousExportSerialRegion(asset)
	if err != nil {
		return nil, err
	}

	offsets := make([]int64, len(keepOrder))
	sizes := make([]int64, len(keepOrder))
	blob := make([]byte, 0, int(end-start))
	cursor := start
	for newIdx, oldIdx := range keepOrder {
		if oldIdx < 0 || oldIdx >= len(asset.Exports) {
			return nil, fmt.Errorf("keep export index out of range: %d", oldIdx)
		}
		exp := asset.Exports[oldIdx]
		oldStart := int(exp.SerialOffset)
		oldEnd := int(exp.SerialOffset + exp.SerialSize)
		if oldStart < 0 || oldEnd < oldStart || oldEnd > len(asset.Raw.Bytes) {
			return nil, fmt.Errorf("export[%d] serial range out of bounds (%d..%d size=%d)", oldIdx+1, oldStart, oldEnd, len(asset.Raw.Bytes))
		}
		if exp.SerialSize == 0 {
			return nil, fmt.Errorf("export[%d] has no serial payload in the removed-export layout", oldIdx+1)
		}
		payload := asset.Raw.Bytes[oldStart:oldEnd]
		offsets[newIdx] = cursor
		sizes[newIdx] = int64(len(payload))
		blob = append(blob, payload...)
		cursor += int64(len(payload))
	}

	return &exportSerialLayout{
		start:   start,
		end:     end,
		blob:    blob,
		offsets: offsets,
		sizes:   sizes,
	}, nil
}

func findContiguousExportSerialRegion(asset *uasset.Asset) (int64, int64, error) {
	if asset == nil {
		return 0, 0, fmt.Errorf("asset is nil")
	}
	type serialRange struct {
		start int64
		end   int64
	}
	ranges := make([]serialRange, 0, len(asset.Exports))
	for i, exp := range asset.Exports {
		if exp.SerialSize < 0 || exp.SerialOffset < 0 {
			return 0, 0, fmt.Errorf("export[%d] has negative serial range", i+1)
		}
		if exp.SerialSize == 0 {
			continue
		}
		start := exp.SerialOffset
		end := exp.SerialOffset + exp.SerialSize
		if end < start || end > int64(len(asset.Raw.Bytes)) {
			return 0, 0, fmt.Errorf("export[%d] serial range out of bounds (%d..%d size=%d)", i+1, start, end, len(asset.Raw.Bytes))
		}
		ranges = append(ranges, serialRange{start: start, end: end})
	}
	if len(ranges) == 0 {
		return 0, 0, fmt.Errorf("asset has no non-empty export serial data")
	}

	for i := 0; i < len(ranges); i++ {
		for j := i + 1; j < len(ranges); j++ {
			if ranges[j].start < ranges[i].start {
				ranges[i], ranges[j] = ranges[j], ranges[i]
			}
		}
	}

	start := ranges[0].start
	end := ranges[0].end
	for i := 1; i < len(ranges); i++ {
		if ranges[i].start != end {
			return 0, 0, fmt.Errorf("non-contiguous export serial layout is unsupported for export insertion")
		}
		end = ranges[i].end
	}
	return start, end, nil
}

func totalInsertPayloadSize(entries []ExportInsertEntry) int {
	total := 0
	for _, entry := range entries {
		total += len(entry.Payload)
	}
	return total
}

func buildInsertedExportIndexRemap(oldCount, insertAt, inserted int) map[int]int {
	remap := make(map[int]int, oldCount)
	for i := 0; i < oldCount; i++ {
		if i >= insertAt {
			remap[i] = i + inserted
			continue
		}
		remap[i] = i
	}
	return remap
}

func remapPackageIndexSlice(items []uasset.PackageIndex, exportRemap map[int]int) []uasset.PackageIndex {
	out := make([]uasset.PackageIndex, 0, len(items))
	for _, item := range items {
		out = append(out, remapPackageIndexForInsertedExports(item, exportRemap))
	}
	return out
}

func remapPackageIndexForInsertedExports(idx uasset.PackageIndex, exportRemap map[int]int) uasset.PackageIndex {
	if idx <= 0 {
		return idx
	}
	oldZero := idx.ResolveIndex()
	newZero, ok := exportRemap[oldZero]
	if !ok || newZero == oldZero {
		return idx
	}
	return uasset.PackageIndex(newZero + 1)
}

func rewriteExistingExportPayloadPackageIndices(asset *uasset.Asset, exportRemap map[int]int) ([]byte, error) {
	if len(exportRemap) == 0 {
		return append([]byte(nil), asset.Raw.Bytes...), nil
	}

	mutations, err := buildSameAssetExportPackageIndexRemapMutations(asset, exportRemap)
	if err != nil {
		return nil, err
	}
	if len(mutations) == 0 {
		return append([]byte(nil), asset.Raw.Bytes...), nil
	}
	out, err := RewriteAsset(asset, mutations)
	if err != nil {
		return nil, fmt.Errorf("rewrite export payload package indices: %w", err)
	}
	return out, nil
}

func buildSameAssetExportPackageIndexRemapMutations(asset *uasset.Asset, exportRemap map[int]int) ([]ExportMutation, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	mutations := make([]ExportMutation, 0, len(asset.Exports))
	for i, exp := range asset.Exports {
		oldStart := int(exp.SerialOffset)
		oldEnd := int(exp.SerialOffset + exp.SerialSize)
		if oldStart < 0 || oldEnd < oldStart || oldEnd > len(asset.Raw.Bytes) {
			return nil, fmt.Errorf("export[%d] serial range out of bounds", i+1)
		}
		propertyStart, propertyEnd, withClassControl := exportPropertyBounds(asset, exp)
		if propertyStart < oldStart || propertyEnd < propertyStart || propertyEnd > oldEnd {
			return nil, fmt.Errorf("export[%d] property range out of bounds", i+1)
		}

		parsed := asset.ParseTaggedPropertiesRange(propertyStart, propertyEnd, withClassControl)
		if len(parsed.Warnings) > 0 {
			className := asset.ResolveClassName(exp)
			if packageIndexRemapCanSkipTaggedPropertyWarnings(className) {
				mutation, changed, err := buildPartialSameAssetExportPackageIndexRemapMutation(asset, i, exp, parsed, order, exportRemap)
				if err != nil {
					return nil, err
				}
				if changed {
					mutations = append(mutations, *mutation)
				}
				continue
			}
			return nil, fmt.Errorf("cannot safely remap export[%d] tagged properties: %s", i+1, stringsJoin(parsed.Warnings, "; "))
		}
		if parsed.EndOffset < propertyStart+8 {
			return nil, fmt.Errorf("export[%d] property terminator not found", i+1)
		}

		noneStart := parsed.EndOffset - 8
		prefixEnd := noneStart
		if len(parsed.Properties) > 0 {
			prefixEnd = parsed.Properties[0].Offset
		}
		tagBlob := append([]byte(nil), asset.Raw.Bytes[propertyStart:prefixEnd]...)
		propsChanged := false
		for j, tag := range parsed.Properties {
			decoded, ok := asset.DecodePropertyValue(tag)
			tagStart := tag.Offset
			tagEnd := noneStart
			if j+1 < len(parsed.Properties) {
				tagEnd = parsed.Properties[j+1].Offset
			}
			if !ok {
				tagBlob = append(tagBlob, asset.Raw.Bytes[tagStart:tagEnd]...)
				continue
			}

			remappedValue, valueChanged, err := remapDecodedValueForInsertedExports(decoded, exportRemap)
			if err != nil {
				return nil, fmt.Errorf("remap export[%d] property %s package indices: %w", i+1, tag.Name.Display(asset.Names), err)
			}
			if !valueChanged {
				tagBlob = append(tagBlob, asset.Raw.Bytes[tagStart:tagEnd]...)
				continue
			}

			typeTree, err := buildTypeTree(tag.TypeNodes, asset.Names)
			if err != nil {
				return nil, fmt.Errorf("build export[%d] property %s type tree: %w", i+1, tag.Name.Display(asset.Names), err)
			}
			valueBytes, boolValue, err := encodePropertyValue(asset, typeTree, remappedValue, order)
			if err != nil {
				return nil, fmt.Errorf("encode export[%d] property %s: %w", i+1, tag.Name.Display(asset.Names), err)
			}
			tagBytes, _, err := serializePropertyTag(asset, tag, valueBytes, boolValue, order)
			if err != nil {
				return nil, fmt.Errorf("serialize export[%d] property %s: %w", i+1, tag.Name.Display(asset.Names), err)
			}
			if !bytes.Equal(tagBytes, asset.Raw.Bytes[tagStart:tagEnd]) {
				propsChanged = true
			}
			tagBlob = append(tagBlob, tagBytes...)
		}

		oldPayload := append([]byte(nil), asset.Raw.Bytes[oldStart:oldEnd]...)
		tailStart := parsed.EndOffset - oldStart
		tailChanged := false
		if tailStart >= 0 && tailStart < len(oldPayload) {
			tail := append([]byte(nil), oldPayload[tailStart:]...)
			className := asset.ResolveClassName(exp)
			switch {
			case strings.EqualFold(className, "K2Node_Event"):
				tailChanged = remapK2NodeEventTailSelfRefs(tail, i, exportRemap, order)
			case strings.EqualFold(className, "WidgetBlueprintGeneratedClass"):
				tailChanged = remapWidgetBlueprintGeneratedClassTailRefs(tail, tailStart, asset, exportRemap, order)
			}
			if tailChanged {
				copy(oldPayload[tailStart:], tail)
			}
		}
		if !propsChanged && !tailChanged {
			continue
		}

		var newPayload []byte
		if propsChanged {
			noneBytes := asset.Raw.Bytes[noneStart:parsed.EndOffset]
			trailing := asset.Raw.Bytes[parsed.EndOffset:propertyEnd]
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
		if propertyDelta != 0 && !asset.Summary.UsesUnversionedPropertySerialization() && asset.Summary.FileVersionUE5 >= ue5ScriptSerializationOffset {
			oldStartRel := exp.ScriptSerializationStartOffset
			oldEndRel := exp.ScriptSerializationEndOffset
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

func buildPartialSameAssetExportPackageIndexRemapMutation(asset *uasset.Asset, exportIndex int, exp uasset.ExportEntry, parsed uasset.PropertyListResult, order binary.ByteOrder, exportRemap map[int]int) (*ExportMutation, bool, error) {
	oldStart := int(exp.SerialOffset)
	oldEnd := int(exp.SerialOffset + exp.SerialSize)
	if oldStart < 0 || oldEnd < oldStart || oldEnd > len(asset.Raw.Bytes) {
		return nil, false, fmt.Errorf("export[%d] serial range out of bounds", exportIndex+1)
	}
	newPayload := append([]byte(nil), asset.Raw.Bytes[oldStart:oldEnd]...)
	changed := false
	for i, tag := range parsed.Properties {
		decoded, ok := asset.DecodePropertyValue(tag)
		if !ok {
			continue
		}
		remappedValue, valueChanged, err := remapDecodedValueForInsertedExports(decoded, exportRemap)
		if err != nil {
			return nil, false, fmt.Errorf("remap export[%d] property %s package indices: %w", exportIndex+1, tag.Name.Display(asset.Names), err)
		}
		if !valueChanged {
			continue
		}
		typeTree, err := buildTypeTree(tag.TypeNodes, asset.Names)
		if err != nil {
			return nil, false, fmt.Errorf("build export[%d] property %s type tree: %w", exportIndex+1, tag.Name.Display(asset.Names), err)
		}
		valueBytes, boolValue, err := encodePropertyValue(asset, typeTree, remappedValue, order)
		if err != nil {
			return nil, false, fmt.Errorf("encode export[%d] property %s: %w", exportIndex+1, tag.Name.Display(asset.Names), err)
		}
		tagBytes, _, err := serializePropertyTag(asset, tag, valueBytes, boolValue, order)
		if err != nil {
			return nil, false, fmt.Errorf("serialize export[%d] property %s: %w", exportIndex+1, tag.Name.Display(asset.Names), err)
		}
		tagStartRel := tag.Offset - oldStart
		tagEndAbs := tag.ValueOffset + int(tag.Size)
		if i+1 < len(parsed.Properties) {
			tagEndAbs = parsed.Properties[i+1].Offset
		}
		tagEndRel := tagEndAbs - oldStart
		if tagStartRel < 0 || tagEndRel < tagStartRel || tagEndRel > len(newPayload) {
			return nil, false, fmt.Errorf("export[%d] property %s partial range out of bounds", exportIndex+1, tag.Name.Display(asset.Names))
		}
		if len(tagBytes) != tagEndRel-tagStartRel {
			continue
		}
		copy(newPayload[tagStartRel:tagEndRel], tagBytes)
		changed = true
	}
	if !changed {
		return nil, false, nil
	}
	return &ExportMutation{ExportIndex: exportIndex, Payload: newPayload}, true, nil
}

func remapK2NodeEventTailSelfRefs(tail []byte, exportIdx int, exportRemap map[int]int, order binary.ByteOrder) bool {
	newZero, ok := exportRemap[exportIdx]
	if !ok || newZero == exportIdx {
		return false
	}
	oldSerialized := int32(exportIdx + 1)
	newSerialized := int32(newZero + 1)
	changed := false
	for pos := 0; pos+40 <= len(tail); pos++ {
		if int32(order.Uint32(tail[pos:pos+4])) != oldSerialized {
			continue
		}
		guid := tail[pos+4 : pos+20]
		if isAllZeroBytes(guid) {
			continue
		}
		if int32(order.Uint32(tail[pos+20:pos+24])) != oldSerialized {
			continue
		}
		if !bytes.Equal(guid, tail[pos+24:pos+40]) {
			continue
		}
		order.PutUint32(tail[pos:pos+4], uint32(newSerialized))
		order.PutUint32(tail[pos+20:pos+24], uint32(newSerialized))
		changed = true
		pos += 39
	}
	return changed
}

func remapWidgetBlueprintGeneratedClassTailRefs(tail []byte, tailStart int, asset *uasset.Asset, exportRemap map[int]int, order binary.ByteOrder) bool {
	scanStart, err := widgetBlueprintGeneratedClassFieldRecordsEnd(tail, order)
	if err != nil {
		return false
	}
	changed, err := remapWidgetBlueprintGeneratedClassTailSuffixRefs(tail, scanStart, asset, exportRemap, nil, order)
	if err != nil {
		return false
	}
	return changed
}

func widgetBlueprintGeneratedClassFieldRecordsEnd(raw []byte, order binary.ByteOrder) (int, error) {
	if len(raw) < 16 {
		return 0, nil
	}
	fieldCount := int(order.Uint32(raw[12:16]))
	offset := 16
	for i := 0; i < fieldCount; i++ {
		recordLen, err := widgetBlueprintGeneratedClassFieldRecordLength(raw[offset:], order)
		if err != nil {
			return 0, err
		}
		offset += recordLen
	}
	return offset, nil
}

func widgetBlueprintGeneratedClassFieldRecordLength(raw []byte, order binary.ByteOrder) (int, error) {
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
		_, consumed, err := generatedClassTailFStringLength(raw[offset:], order)
		if err != nil {
			return 0, fmt.Errorf("metadata %d string: %w", i, err)
		}
		offset += consumed
	}
	const fixedSuffix = 4 + 4 + 8 + 2 + 8 + 1 + 4
	if len(raw[offset:]) < fixedSuffix {
		return 0, fmt.Errorf("record suffix out of bounds")
	}
	offset += fixedSuffix
	return offset, nil
}

func isAllZeroBytes(raw []byte) bool {
	for _, b := range raw {
		if b != 0 {
			return false
		}
	}
	return true
}

func remapDecodedValueForInsertedExports(value any, exportRemap map[int]int) (any, bool, error) {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		changed := false
		for key, item := range v {
			next, itemChanged, err := remapDecodedValueForInsertedExports(item, exportRemap)
			if err != nil {
				return nil, false, err
			}
			out[key] = next
			changed = changed || itemChanged
		}
		if _, ok := v["resolved"].(string); ok {
			if idx, err := asInt64(v["index"]); err == nil {
				newIdx := remapPackageIndexForInsertedExports(uasset.PackageIndex(int32(idx)), exportRemap)
				if int32(newIdx) != int32(idx) {
					out["index"] = int32(newIdx)
					changed = true
				}
			}
		}
		return out, changed, nil
	case []any:
		out := make([]any, len(v))
		changed := false
		for i, item := range v {
			next, itemChanged, err := remapDecodedValueForInsertedExports(item, exportRemap)
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
			next, itemChanged, err := remapDecodedValueForInsertedExports(item, exportRemap)
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

func buildInsertedExportMapEntries(asset *uasset.Asset, insertAt int, entries []ExportInsertEntry, exportRemap map[int]int) ([]uasset.ExportEntry, error) {
	out := make([]uasset.ExportEntry, 0, len(asset.Exports)+len(entries))
	for i := 0; i < insertAt; i++ {
		out = append(out, remapExportHeaderPackageIndices(asset.Exports[i], exportRemap))
	}
	for _, entry := range entries {
		header := remapExportHeaderPackageIndices(entry.Header, exportRemap)
		header.SerialSize = 0
		header.SerialOffset = 0
		header.ScriptSerializationStartOffset = 0
		header.ScriptSerializationEndOffset = 0
		out = append(out, header)
	}
	for i := insertAt; i < len(asset.Exports); i++ {
		out = append(out, remapExportHeaderPackageIndices(asset.Exports[i], exportRemap))
	}
	return out, nil
}

func remapExportHeaderPackageIndices(exp uasset.ExportEntry, exportRemap map[int]int) uasset.ExportEntry {
	out := exp
	out.ClassIndex = remapPackageIndexForInsertedExports(out.ClassIndex, exportRemap)
	out.SuperIndex = remapPackageIndexForInsertedExports(out.SuperIndex, exportRemap)
	out.TemplateIndex = remapPackageIndexForInsertedExports(out.TemplateIndex, exportRemap)
	out.OuterIndex = remapPackageIndexForInsertedExports(out.OuterIndex, exportRemap)
	return out
}

func encodeExportMapEntries(entries []uasset.ExportEntry, order binary.ByteOrder, summary uasset.PackageSummary) []byte {
	w := newByteWriter(order, len(entries)*96)
	for _, entry := range entries {
		w.writeInt32(int32(entry.ClassIndex))
		w.writeInt32(int32(entry.SuperIndex))
		w.writeInt32(int32(entry.TemplateIndex))
		w.writeInt32(int32(entry.OuterIndex))
		w.writeNameRef(entry.ObjectName.Index, entry.ObjectName.Number)
		w.writeUint32(entry.ObjectFlags)
		w.writeInt64(entry.SerialSize)
		w.writeInt64(entry.SerialOffset)
		w.writeUBool(entry.ForcedExport)
		w.writeUBool(entry.NotForClient)
		w.writeUBool(entry.NotForServer)
		if summary.FileVersionUE5 < ue5RemoveObjectExportPkgGUID {
			w.writeBytes(make([]byte, 16))
		}
		if summary.FileVersionUE5 >= ue5TrackObjectExportInherited {
			w.writeUBool(entry.IsInheritedInstance)
		}
		w.writeUint32(entry.PackageFlags)
		w.writeUBool(entry.NotAlwaysLoadedForEditor)
		w.writeUBool(entry.IsAsset)
		if summary.FileVersionUE5 >= ue5OptionalResources {
			w.writeUBool(entry.GeneratePublicHash)
		}
		w.writeInt32(entry.FirstExportDependency)
		w.writeInt32(entry.SerializationBeforeSerializationDeps)
		w.writeInt32(entry.CreateBeforeSerializationDeps)
		w.writeInt32(entry.SerializationBeforeCreateDependencies)
		w.writeInt32(entry.CreateBeforeCreateDependencies)
		if summary.SupportsScriptSerializationOffsets() {
			w.writeInt64(entry.ScriptSerializationStartOffset)
			w.writeInt64(entry.ScriptSerializationEndOffset)
		}
	}
	return w.bytes()
}

func findExportMapEndOffset(asset *uasset.Asset, raw []byte) (int64, error) {
	if asset == nil {
		return 0, fmt.Errorf("asset is nil")
	}
	start := int64(asset.Summary.ExportOffset)
	if start < 0 {
		return 0, fmt.Errorf("export offset is negative: %d", asset.Summary.ExportOffset)
	}

	candidates := []int64{
		int64(asset.Summary.ImportOffset),
		int64(asset.Summary.CellImportOffset),
		int64(asset.Summary.CellExportOffset),
		int64(asset.Summary.MetaDataOffset),
		int64(asset.Summary.DependsOffset),
		int64(asset.Summary.SoftPackageReferencesOffset),
		int64(asset.Summary.SearchableNamesOffset),
		int64(asset.Summary.ThumbnailTableOffset),
		int64(asset.Summary.ImportTypeHierarchiesOffset),
		int64(asset.Summary.AssetRegistryDataOffset),
		int64(asset.Summary.PreloadDependencyOffset),
		int64(asset.Summary.DataResourceOffset),
		asset.Summary.PayloadTOCOffset,
		asset.Summary.BulkDataStartOffset,
	}

	end := int64(len(raw))
	for _, exp := range asset.Exports {
		if exp.SerialOffset > start && exp.SerialOffset < end {
			end = exp.SerialOffset
		}
	}
	for _, off := range candidates {
		if off <= start || off > int64(len(raw)) {
			continue
		}
		if off < end {
			end = off
		}
	}
	if end < start {
		return 0, fmt.Errorf("could not determine ExportMap end offset")
	}
	return end, nil
}

// RemoveExportEntries removes the selected zero-based ExportMap entries,
// remaps remaining positive export references, rewrites DependsMap, and
// rebuilds the export serial region in final kept-export order.
func RemoveExportEntries(asset *uasset.Asset, removeIndices []int) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if len(asset.Raw.Bytes) == 0 {
		return nil, fmt.Errorf("asset has no raw bytes")
	}
	if len(removeIndices) == 0 {
		return append([]byte(nil), asset.Raw.Bytes...), nil
	}

	removeSet := make(map[int]bool, len(removeIndices))
	for _, idx := range removeIndices {
		if idx < 0 || idx >= len(asset.Exports) {
			return nil, fmt.Errorf("remove export index out of range: %d", idx)
		}
		if removeSet[idx] {
			return nil, fmt.Errorf("duplicate remove export index: %d", idx)
		}
		removeSet[idx] = true
	}
	if len(removeSet) == len(asset.Exports) {
		return nil, fmt.Errorf("removing every export is unsupported")
	}

	keepOrder := make([]int, 0, len(asset.Exports)-len(removeSet))
	exportRemap := make(map[int]int, len(asset.Exports)-len(removeSet))
	for i := range asset.Exports {
		if removeSet[i] {
			continue
		}
		exportRemap[i] = len(keepOrder)
		keepOrder = append(keepOrder, i)
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	workingBytes, err := rewriteExistingExportPayloadPackageIndicesForDeletion(asset, exportRemap, removeSet)
	if err != nil {
		return nil, fmt.Errorf("rewrite existing export payload refs: %w", err)
	}
	workingAsset, err := uasset.ParseBytes(workingBytes, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse payload-remapped asset: %w", err)
	}

	finalEntries := make([]uasset.ExportEntry, len(keepOrder))
	tempEntries := make([]uasset.ExportEntry, len(keepOrder))
	for newIdx, oldIdx := range keepOrder {
		remapped, err := remapExportHeaderPackageIndicesForDeletion(workingAsset.Exports[oldIdx], exportRemap, removeSet)
		if err != nil {
			return nil, fmt.Errorf("remap export[%d] header: %w", oldIdx+1, err)
		}
		finalEntries[newIdx] = remapped
		tempEntries[newIdx] = remapped
		tempEntries[newIdx].ScriptSerializationStartOffset = 0
		tempEntries[newIdx].ScriptSerializationEndOffset = 0
	}

	if workingAsset.Summary.DependsOffset > 0 {
		oldDepends, _, _, err := parseDependsMapEntries(asset)
		if err != nil {
			return nil, fmt.Errorf("parse original depends map: %w", err)
		}
		newDepends := make([][]uasset.PackageIndex, len(keepOrder))
		for newIdx, oldIdx := range keepOrder {
			remappedDeps, err := remapDependsPackageIndexSliceForDeletedExports(oldDepends[oldIdx], exportRemap, removeSet)
			if err != nil {
				return nil, fmt.Errorf("remap export[%d] depends entry: %w", oldIdx+1, err)
			}
			newDepends[newIdx] = remappedDeps
		}
		depStart := int64(workingAsset.Summary.DependsOffset)
		depEnd, err := findDependsMapEndOffset(workingAsset, workingAsset.Raw.Bytes)
		if err != nil {
			return nil, err
		}
		workingBytes, err = RewriteRawRange(workingAsset, depStart, depEnd, encodeDependsMapEntries(newDepends, order))
		if err != nil {
			return nil, fmt.Errorf("rewrite depends map: %w", err)
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, uasset.ParseOptions{KeepUnknown: true})
		if err != nil {
			return nil, fmt.Errorf("reparse depends-pruned asset: %w", err)
		}
	}
	serialLayout, err := buildRemovedExportSerialLayout(workingAsset, keepOrder)
	if err != nil {
		return nil, err
	}

	exportStart := int64(workingAsset.Summary.ExportOffset)
	exportEnd, err := findExportMapEndOffset(workingAsset, workingAsset.Raw.Bytes)
	if err != nil {
		return nil, err
	}
	tempExportMapBytes := encodeExportMapEntries(tempEntries, order, workingAsset.Summary)
	exportMapDelta := len(tempExportMapBytes) - int(exportEnd-exportStart)
	if exportMapDelta != 0 {
		serialLayout.start += int64(exportMapDelta)
		serialLayout.end += int64(exportMapDelta)
		for i := range serialLayout.offsets {
			serialLayout.offsets[i] += int64(exportMapDelta)
		}
	}
	for i := range finalEntries {
		finalEntries[i].SerialOffset = serialLayout.offsets[i]
		finalEntries[i].SerialSize = serialLayout.sizes[i]
		tempEntries[i].SerialOffset = serialLayout.offsets[i]
		tempEntries[i].SerialSize = serialLayout.sizes[i]
	}
	reorderedExportMapBytes := encodeExportMapEntries(tempEntries, order, workingAsset.Summary)
	exportRewriteAsset := *workingAsset
	exportRewriteAsset.Exports = nil
	workingBytes, err = RewriteRawRange(&exportRewriteAsset, exportStart, exportEnd, reorderedExportMapBytes)
	if err != nil {
		return nil, fmt.Errorf("rewrite export map: %w", err)
	}
	if err := patchSummaryExportCounts(workingBytes, workingAsset, int32(len(finalEntries)), order); err != nil {
		return nil, err
	}
	workingAsset, err = uasset.ParseBytes(workingBytes, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse export-removed asset: %w", err)
	}

	fieldPositions, err := scanExportFieldPositions(workingAsset.Raw.Bytes, workingAsset)
	if err != nil {
		return nil, fmt.Errorf("scan export fields: %w", err)
	}
	if len(fieldPositions) != len(finalEntries) {
		return nil, fmt.Errorf("export field scan mismatch: got %d want %d", len(fieldPositions), len(finalEntries))
	}

	workingBytes, err = rewriteRawRangeAllowExportOverlap(workingAsset, serialLayout.start, serialLayout.end, serialLayout.blob)
	if err != nil {
		return nil, fmt.Errorf("rewrite export serial data: %w", err)
	}
	for exportIdx := range finalEntries {
		field := fieldPositions[exportIdx]
		if err := writeInt64At(workingBytes, field.serialSizePos, serialLayout.sizes[exportIdx], order); err != nil {
			return nil, fmt.Errorf("patch export[%d] serial size: %w", exportIdx+1, err)
		}
		if err := writeInt64At(workingBytes, field.serialOffsetPos, serialLayout.offsets[exportIdx], order); err != nil {
			return nil, fmt.Errorf("patch export[%d] serial offset: %w", exportIdx+1, err)
		}
		if field.scriptStartPos >= 0 {
			if err := writeInt64At(workingBytes, field.scriptStartPos, finalEntries[exportIdx].ScriptSerializationStartOffset, order); err != nil {
				return nil, fmt.Errorf("patch export[%d] script start: %w", exportIdx+1, err)
			}
		}
		if field.scriptEndPos >= 0 {
			if err := writeInt64At(workingBytes, field.scriptEndPos, finalEntries[exportIdx].ScriptSerializationEndOffset, order); err != nil {
				return nil, fmt.Errorf("patch export[%d] script end: %w", exportIdx+1, err)
			}
		}
	}

	if err := FinalizePackageBytes(workingBytes, asset.Summary.FileVersionUE5); err != nil {
		return nil, fmt.Errorf("finalize package bytes: %w", err)
	}
	return workingBytes, nil
}

func remapPackageIndexForDeletedExports(idx uasset.PackageIndex, exportRemap map[int]int, removeSet map[int]bool) (uasset.PackageIndex, error) {
	if idx <= 0 {
		return idx, nil
	}
	oldZero := idx.ResolveIndex()
	if removeSet[oldZero] {
		return 0, fmt.Errorf("reference to removed export %d", oldZero+1)
	}
	newZero, ok := exportRemap[oldZero]
	if !ok {
		return 0, fmt.Errorf("export remap missing kept export %d", oldZero+1)
	}
	return uasset.PackageIndex(newZero + 1), nil
}

func remapDependsPackageIndexSliceForDeletedExports(items []uasset.PackageIndex, exportRemap map[int]int, removeSet map[int]bool) ([]uasset.PackageIndex, error) {
	out := make([]uasset.PackageIndex, 0, len(items))
	for _, item := range items {
		if item > 0 && removeSet[item.ResolveIndex()] {
			continue
		}
		next, err := remapPackageIndexForDeletedExports(item, exportRemap, removeSet)
		if err != nil {
			return nil, err
		}
		out = append(out, next)
	}
	return out, nil
}

func remapExportHeaderPackageIndicesForDeletion(exp uasset.ExportEntry, exportRemap map[int]int, removeSet map[int]bool) (uasset.ExportEntry, error) {
	out := exp
	var err error
	out.ClassIndex, err = remapPackageIndexForDeletedExports(out.ClassIndex, exportRemap, removeSet)
	if err != nil {
		return out, fmt.Errorf("ClassIndex: %w", err)
	}
	out.SuperIndex, err = remapPackageIndexForDeletedExports(out.SuperIndex, exportRemap, removeSet)
	if err != nil {
		return out, fmt.Errorf("SuperIndex: %w", err)
	}
	out.TemplateIndex, err = remapPackageIndexForDeletedExports(out.TemplateIndex, exportRemap, removeSet)
	if err != nil {
		return out, fmt.Errorf("TemplateIndex: %w", err)
	}
	out.OuterIndex, err = remapPackageIndexForDeletedExports(out.OuterIndex, exportRemap, removeSet)
	if err != nil {
		return out, fmt.Errorf("OuterIndex: %w", err)
	}
	return out, nil
}

func rewriteExistingExportPayloadPackageIndicesForDeletion(asset *uasset.Asset, exportRemap map[int]int, removeSet map[int]bool) ([]byte, error) {
	mutations, err := buildSameAssetExportPackageIndexDeleteMutations(asset, exportRemap, removeSet)
	if err != nil {
		return nil, err
	}
	if len(mutations) == 0 {
		return append([]byte(nil), asset.Raw.Bytes...), nil
	}
	out, err := RewriteAsset(asset, mutations)
	if err != nil {
		return nil, fmt.Errorf("rewrite export payload package indices: %w", err)
	}
	return out, nil
}

func buildSameAssetExportPackageIndexDeleteMutations(asset *uasset.Asset, exportRemap map[int]int, removeSet map[int]bool) ([]ExportMutation, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	mutations := make([]ExportMutation, 0, len(asset.Exports))
	for i, exp := range asset.Exports {
		if removeSet[i] {
			continue
		}
		oldStart := int(exp.SerialOffset)
		oldEnd := int(exp.SerialOffset + exp.SerialSize)
		if oldStart < 0 || oldEnd < oldStart || oldEnd > len(asset.Raw.Bytes) {
			return nil, fmt.Errorf("export[%d] serial range out of bounds", i+1)
		}
		propertyStart, propertyEnd, withClassControl := exportPropertyBounds(asset, exp)
		if propertyStart < oldStart || propertyEnd < propertyStart || propertyEnd > oldEnd {
			return nil, fmt.Errorf("export[%d] property range out of bounds", i+1)
		}

		parsed := asset.ParseTaggedPropertiesRange(propertyStart, propertyEnd, withClassControl)
		if len(parsed.Warnings) > 0 {
			className := asset.ResolveClassName(exp)
			if packageIndexRemapCanSkipTaggedPropertyWarnings(className) {
				mutation, changed, err := buildPartialSameAssetExportPackageIndexDeleteMutation(asset, i, exp, parsed, order, exportRemap, removeSet)
				if err != nil {
					return nil, err
				}
				if changed {
					mutations = append(mutations, *mutation)
				}
				continue
			}
			return nil, fmt.Errorf("cannot safely remap export[%d] tagged properties: %s", i+1, stringsJoin(parsed.Warnings, "; "))
		}
		if parsed.EndOffset < propertyStart+8 {
			return nil, fmt.Errorf("export[%d] property terminator not found", i+1)
		}

		noneStart := parsed.EndOffset - 8
		prefixEnd := noneStart
		if len(parsed.Properties) > 0 {
			prefixEnd = parsed.Properties[0].Offset
		}
		tagBlob := append([]byte(nil), asset.Raw.Bytes[propertyStart:prefixEnd]...)
		propsChanged := false
		for j, tag := range parsed.Properties {
			decoded, ok := asset.DecodePropertyValue(tag)
			tagStart := tag.Offset
			tagEnd := noneStart
			if j+1 < len(parsed.Properties) {
				tagEnd = parsed.Properties[j+1].Offset
			}
			if !ok {
				tagBlob = append(tagBlob, asset.Raw.Bytes[tagStart:tagEnd]...)
				continue
			}

			remappedValue, valueChanged, err := remapDecodedValueForDeletedExports(decoded, exportRemap, removeSet)
			if err != nil {
				return nil, fmt.Errorf("remap export[%d] property %s package indices: %w", i+1, tag.Name.Display(asset.Names), err)
			}
			if !valueChanged {
				tagBlob = append(tagBlob, asset.Raw.Bytes[tagStart:tagEnd]...)
				continue
			}

			typeTree, err := buildTypeTree(tag.TypeNodes, asset.Names)
			if err != nil {
				return nil, fmt.Errorf("build export[%d] property %s type tree: %w", i+1, tag.Name.Display(asset.Names), err)
			}
			valueBytes, boolValue, err := encodePropertyValue(asset, typeTree, remappedValue, order)
			if err != nil {
				return nil, fmt.Errorf("encode export[%d] property %s: %w", i+1, tag.Name.Display(asset.Names), err)
			}
			tagBytes, _, err := serializePropertyTag(asset, tag, valueBytes, boolValue, order)
			if err != nil {
				return nil, fmt.Errorf("serialize export[%d] property %s: %w", i+1, tag.Name.Display(asset.Names), err)
			}
			if !bytes.Equal(tagBytes, asset.Raw.Bytes[tagStart:tagEnd]) {
				propsChanged = true
			}
			tagBlob = append(tagBlob, tagBytes...)
		}

		oldPayload := append([]byte(nil), asset.Raw.Bytes[oldStart:oldEnd]...)
		tailStart := parsed.EndOffset - oldStart
		tailChanged := false
		if tailStart >= 0 && tailStart < len(oldPayload) {
			tail := append([]byte(nil), oldPayload[tailStart:]...)
			className := asset.ResolveClassName(exp)
			switch {
			case strings.EqualFold(className, "K2Node_Event"):
				tailChanged = remapK2NodeEventTailSelfRefs(tail, i, exportRemap, order)
			case strings.EqualFold(className, "WidgetBlueprintGeneratedClass"):
				var tailErr error
				tailChanged, tailErr = remapWidgetBlueprintGeneratedClassTailRefsForDeletion(tail, asset, exportRemap, removeSet, order)
				if tailErr != nil {
					return nil, fmt.Errorf("remap WidgetBlueprintGeneratedClass tail refs: %w", tailErr)
				}
			}
			if tailChanged {
				copy(oldPayload[tailStart:], tail)
			}
		}
		if !propsChanged && !tailChanged {
			continue
		}

		var newPayload []byte
		if propsChanged {
			noneBytes := asset.Raw.Bytes[noneStart:parsed.EndOffset]
			trailing := asset.Raw.Bytes[parsed.EndOffset:propertyEnd]
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
		if propertyDelta != 0 && !asset.Summary.UsesUnversionedPropertySerialization() && asset.Summary.FileVersionUE5 >= ue5ScriptSerializationOffset {
			oldStartRel := exp.ScriptSerializationStartOffset
			oldEndRel := exp.ScriptSerializationEndOffset
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

func buildPartialSameAssetExportPackageIndexDeleteMutation(asset *uasset.Asset, exportIndex int, exp uasset.ExportEntry, parsed uasset.PropertyListResult, order binary.ByteOrder, exportRemap map[int]int, removeSet map[int]bool) (*ExportMutation, bool, error) {
	oldStart := int(exp.SerialOffset)
	oldEnd := int(exp.SerialOffset + exp.SerialSize)
	if oldStart < 0 || oldEnd < oldStart || oldEnd > len(asset.Raw.Bytes) {
		return nil, false, fmt.Errorf("export[%d] serial range out of bounds", exportIndex+1)
	}
	newPayload := append([]byte(nil), asset.Raw.Bytes[oldStart:oldEnd]...)
	changed := false
	for i, tag := range parsed.Properties {
		decoded, ok := asset.DecodePropertyValue(tag)
		if !ok {
			continue
		}
		remappedValue, valueChanged, err := remapDecodedValueForDeletedExports(decoded, exportRemap, removeSet)
		if err != nil {
			return nil, false, fmt.Errorf("remap export[%d] property %s package indices: %w", exportIndex+1, tag.Name.Display(asset.Names), err)
		}
		if !valueChanged {
			continue
		}
		typeTree, err := buildTypeTree(tag.TypeNodes, asset.Names)
		if err != nil {
			return nil, false, fmt.Errorf("build export[%d] property %s type tree: %w", exportIndex+1, tag.Name.Display(asset.Names), err)
		}
		valueBytes, boolValue, err := encodePropertyValue(asset, typeTree, remappedValue, order)
		if err != nil {
			return nil, false, fmt.Errorf("encode export[%d] property %s: %w", exportIndex+1, tag.Name.Display(asset.Names), err)
		}
		tagBytes, _, err := serializePropertyTag(asset, tag, valueBytes, boolValue, order)
		if err != nil {
			return nil, false, fmt.Errorf("serialize export[%d] property %s: %w", exportIndex+1, tag.Name.Display(asset.Names), err)
		}
		tagStartRel := tag.Offset - oldStart
		tagEndAbs := tag.ValueOffset + int(tag.Size)
		if i+1 < len(parsed.Properties) {
			tagEndAbs = parsed.Properties[i+1].Offset
		}
		tagEndRel := tagEndAbs - oldStart
		if tagStartRel < 0 || tagEndRel < tagStartRel || tagEndRel > len(newPayload) {
			return nil, false, fmt.Errorf("export[%d] property %s partial range out of bounds", exportIndex+1, tag.Name.Display(asset.Names))
		}
		if len(tagBytes) != tagEndRel-tagStartRel {
			continue
		}
		copy(newPayload[tagStartRel:tagEndRel], tagBytes)
		changed = true
	}
	if !changed {
		return nil, false, nil
	}
	return &ExportMutation{ExportIndex: exportIndex, Payload: newPayload}, true, nil
}

func remapWidgetBlueprintGeneratedClassTailRefsForDeletion(tail []byte, asset *uasset.Asset, exportRemap map[int]int, removeSet map[int]bool, order binary.ByteOrder) (bool, error) {
	scanStart, err := widgetBlueprintGeneratedClassFieldRecordsEnd(tail, order)
	if err != nil {
		return false, err
	}
	return remapWidgetBlueprintGeneratedClassTailSuffixRefs(tail, scanStart, asset, exportRemap, removeSet, order)
}

func remapWidgetBlueprintGeneratedClassTailSuffixRefs(tail []byte, scanStart int, asset *uasset.Asset, exportRemap map[int]int, removeSet map[int]bool, order binary.ByteOrder) (bool, error) {
	if asset == nil || len(tail) == 0 {
		return false, nil
	}
	if scanStart < 0 || scanStart > len(tail) {
		scanStart = 0
	}

	const (
		blueprintRefOffsetFromEnd = 28 // int64-like {exportIndex,0}
		cdoRefOffsetFromEnd       = 4  // trailing int32 export index
	)
	if len(tail)-blueprintRefOffsetFromEnd < scanStart || len(tail)-cdoRefOffsetFromEnd < scanStart {
		return false, nil
	}

	var (
		oldBlueprintSerialized int32
		newBlueprintSerialized int32
		oldCDOSerialized       int32
		newCDOSerialized       int32
	)
	for oldZero, newZero := range exportRemap {
		if oldZero < 0 || oldZero >= len(asset.Exports) {
			continue
		}
		className := asset.ResolveClassName(asset.Exports[oldZero])
		objectName := asset.Exports[oldZero].ObjectName.Display(asset.Names)
		switch {
		case strings.EqualFold(className, "WidgetBlueprint"):
			oldBlueprintSerialized = int32(oldZero + 1)
			newBlueprintSerialized = int32(newZero + 1)
		case strings.HasPrefix(objectName, "Default__"):
			oldCDOSerialized = int32(oldZero + 1)
			newCDOSerialized = int32(newZero + 1)
		}
	}

	changed := false
	if oldBlueprintSerialized > 0 {
		refPos := len(tail) - blueprintRefOffsetFromEnd
		cur := int32(order.Uint32(tail[refPos : refPos+4]))
		if cur == oldBlueprintSerialized && order.Uint32(tail[refPos+4:refPos+8]) == 0 && newBlueprintSerialized > 0 && newBlueprintSerialized != cur {
			order.PutUint32(tail[refPos:refPos+4], uint32(newBlueprintSerialized))
			changed = true
		} else if removeSet != nil && cur > 0 && removeSet[int(cur-1)] {
			return false, fmt.Errorf("tail still references removed export %d", cur)
		}
	}
	if oldCDOSerialized > 0 {
		refPos := len(tail) - cdoRefOffsetFromEnd
		cur := int32(order.Uint32(tail[refPos : refPos+4]))
		if cur == oldCDOSerialized && newCDOSerialized > 0 && newCDOSerialized != cur {
			order.PutUint32(tail[refPos:refPos+4], uint32(newCDOSerialized))
			changed = true
		} else if removeSet != nil && cur > 0 && removeSet[int(cur-1)] {
			return false, fmt.Errorf("tail still references removed export %d", cur)
		}
	}
	return changed, nil
}

func remapDecodedValueForDeletedExports(value any, exportRemap map[int]int, removeSet map[int]bool) (any, bool, error) {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		changed := false
		for key, item := range v {
			next, itemChanged, err := remapDecodedValueForDeletedExports(item, exportRemap, removeSet)
			if err != nil {
				return nil, false, err
			}
			out[key] = next
			changed = changed || itemChanged
		}
		if _, ok := v["resolved"].(string); ok {
			if idx, err := asInt64(v["index"]); err == nil {
				newIdx, err := remapPackageIndexForDeletedExports(uasset.PackageIndex(int32(idx)), exportRemap, removeSet)
				if err != nil {
					return nil, false, err
				}
				if int32(newIdx) != int32(idx) {
					out["index"] = int32(newIdx)
					changed = true
				}
			}
		}
		return out, changed, nil
	case []any:
		out := make([]any, len(v))
		changed := false
		for i, item := range v {
			next, itemChanged, err := remapDecodedValueForDeletedExports(item, exportRemap, removeSet)
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
			next, itemChanged, err := remapDecodedValueForDeletedExports(item, exportRemap, removeSet)
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

type summaryExportCountFields struct {
	ExportCountPos           int
	GenerationExportCountPos []int
}

func patchSummaryExportCounts(out []byte, asset *uasset.Asset, newCount int32, order binary.ByteOrder) error {
	fields, err := scanSummaryExportCountFields(asset.Raw.Bytes, asset.Summary.FileVersionUE5)
	if err != nil {
		return fmt.Errorf("scan summary export count fields: %w", err)
	}
	if err := writeInt32At(out, fields.ExportCountPos, newCount, order); err != nil {
		return fmt.Errorf("patch summary ExportCount: %w", err)
	}
	if len(fields.GenerationExportCountPos) > 0 {
		lastPos := fields.GenerationExportCountPos[len(fields.GenerationExportCountPos)-1]
		if err := writeInt32At(out, lastPos, newCount, order); err != nil {
			return fmt.Errorf("patch summary Generations.Last().ExportCount: %w", err)
		}
	}
	return nil
}

func scanSummaryExportCountFields(data []byte, unversionedFileUE5 int32) (summaryExportCountFields, error) {
	nameFields, err := scanSummaryNameCountFields(data, unversionedFileUE5)
	if err != nil {
		return summaryExportCountFields{}, err
	}
	return summaryExportCountFields{
		ExportCountPos:           nameFields.ExportCountPos,
		GenerationExportCountPos: append([]int(nil), nameFields.GenerationExportCountPos...),
	}, nil
}

func stringsJoin(items []string, sep string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	n := 0
	for _, item := range items {
		n += len(item)
	}
	n += len(sep) * (len(items) - 1)
	buf := make([]byte, 0, n)
	for i, item := range items {
		if i > 0 {
			buf = append(buf, sep...)
		}
		buf = append(buf, item...)
	}
	return string(buf)
}
