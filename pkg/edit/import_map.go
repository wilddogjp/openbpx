package edit

import (
	"encoding/binary"
	"fmt"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

type importOuterIndexPatch struct {
	outerIndexPos int
}

type exportPackageIndexPatch struct {
	classIndexPos    int
	superIndexPos    int
	templateIndexPos int
	outerIndexPos    int
}

// AppendImportEntries appends serialized ImportMap entries without rewriting
// existing import indices. This is only safe for append-only scenarios where
// new references point at the newly appended imports.
func AppendImportEntries(asset *uasset.Asset, entries []uasset.ImportEntry) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if len(asset.Raw.Bytes) == 0 {
		return nil, fmt.Errorf("asset has no raw bytes")
	}
	if len(entries) == 0 {
		return append([]byte(nil), asset.Raw.Bytes...), nil
	}

	start := int64(asset.Summary.ImportOffset)
	if start < 0 || start > int64(len(asset.Raw.Bytes)) {
		return nil, fmt.Errorf("import offset out of range: %d", asset.Summary.ImportOffset)
	}
	end, err := findImportMapEndOffset(asset, asset.Raw.Bytes)
	if err != nil {
		return nil, err
	}
	if end < start || end > int64(len(asset.Raw.Bytes)) {
		return nil, fmt.Errorf("invalid import map range: %d..%d", start, end)
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	replacement := make([]byte, 0, int(end-start)+len(entries)*40)
	replacement = append(replacement, asset.Raw.Bytes[start:end]...)
	replacement = append(replacement, encodeImportMapEntries(entries, order, asset.Summary)...)

	out, err := RewriteRawRange(asset, start, end, replacement)
	if err != nil {
		return nil, fmt.Errorf("rewrite import map: %w", err)
	}
	importCountPos, err := scanSummaryImportCountPos(asset.Raw.Bytes, asset.Summary.FileVersionUE5)
	if err != nil {
		return nil, fmt.Errorf("scan summary import count: %w", err)
	}
	newImportCount := int32(len(asset.Imports) + len(entries))
	if err := writeInt32At(out, importCountPos, newImportCount, order); err != nil {
		return nil, fmt.Errorf("patch summary ImportCount: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(out, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse rewritten import map: %w", err)
	}
	importRemap := buildInsertedImportIndexRemap(len(asset.Imports), len(asset.Imports), len(entries))
	out, err = rewriteAssetRegistryImportedClassesTrailer(updatedAsset, importRemap, len(asset.Imports), entries)
	if err != nil {
		return nil, fmt.Errorf("rewrite asset registry imported classes: %w", err)
	}
	if err := FinalizePackageBytes(out, asset.Summary.FileVersionUE5); err != nil {
		return nil, fmt.Errorf("finalize package bytes: %w", err)
	}
	return out, nil
}

// InsertImportEntries inserts ImportMap entries at zero-based position and
// remaps ImportMap / ExportMap package indices that point to shifted imports.
func InsertImportEntries(asset *uasset.Asset, insertAt int, entries []uasset.ImportEntry) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if len(asset.Raw.Bytes) == 0 {
		return nil, fmt.Errorf("asset has no raw bytes")
	}
	if len(entries) == 0 {
		return append([]byte(nil), asset.Raw.Bytes...), nil
	}
	if insertAt < 0 || insertAt > len(asset.Imports) {
		return nil, fmt.Errorf("insert position out of range: %d", insertAt)
	}

	start := int64(asset.Summary.ImportOffset)
	if start < 0 || start > int64(len(asset.Raw.Bytes)) {
		return nil, fmt.Errorf("import offset out of range: %d", asset.Summary.ImportOffset)
	}
	end, err := findImportMapEndOffset(asset, asset.Raw.Bytes)
	if err != nil {
		return nil, err
	}
	if end < start || end > int64(len(asset.Raw.Bytes)) {
		return nil, fmt.Errorf("invalid import map range: %d..%d", start, end)
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	oldImportBytes := asset.Raw.Bytes[start:end]
	importRemap := buildInsertedImportIndexRemap(len(asset.Imports), insertAt, len(entries))
	adjustedEntries := make([]uasset.ImportEntry, len(entries))
	copy(adjustedEntries, entries)
	for i := range adjustedEntries {
		adjustedEntries[i].OuterIndex = remapImportPackageIndex(adjustedEntries[i].OuterIndex, importRemap)
	}
	entryStarts, err := scanImportEntryStartOffsets(asset.Raw.Bytes, asset)
	if err != nil {
		return nil, fmt.Errorf("scan import entry starts: %w", err)
	}
	if len(entryStarts) != len(asset.Imports) {
		return nil, fmt.Errorf("import start scan mismatch: got %d want %d", len(entryStarts), len(asset.Imports))
	}
	prefixLen := len(oldImportBytes)
	if insertAt < len(entryStarts) {
		prefixLen = entryStarts[insertAt] - int(start)
	}
	if prefixLen < 0 || prefixLen > len(oldImportBytes) {
		return nil, fmt.Errorf("import insertion offset out of range: %d", prefixLen)
	}

	encodedEntries := encodeImportMapEntries(adjustedEntries, order, asset.Summary)
	replacement := make([]byte, 0, len(oldImportBytes)+len(encodedEntries))
	replacement = append(replacement, oldImportBytes[:prefixLen]...)
	replacement = append(replacement, encodedEntries...)
	replacement = append(replacement, oldImportBytes[prefixLen:]...)

	out, err := RewriteRawRange(asset, start, end, replacement)
	if err != nil {
		return nil, fmt.Errorf("rewrite import map: %w", err)
	}

	importCountPos, err := scanSummaryImportCountPos(asset.Raw.Bytes, asset.Summary.FileVersionUE5)
	if err != nil {
		return nil, fmt.Errorf("scan summary import count: %w", err)
	}
	newImportCount := int32(len(asset.Imports) + len(entries))
	if err := writeInt32At(out, importCountPos, newImportCount, order); err != nil {
		return nil, fmt.Errorf("patch summary ImportCount: %w", err)
	}

	updatedAsset, err := uasset.ParseBytes(out, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse rewritten import map: %w", err)
	}
	if err := patchShiftedImportOuterIndices(out, asset, updatedAsset, importRemap, order); err != nil {
		return nil, err
	}
	updatedAsset, err = uasset.ParseBytes(out, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse import-patched asset: %w", err)
	}
	if err := patchShiftedExportPackageIndices(out, asset, updatedAsset, importRemap, order); err != nil {
		return nil, err
	}
	updatedAsset, err = uasset.ParseBytes(out, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse export-patched asset: %w", err)
	}
	if err := patchShiftedDependsPackageIndices(out, asset, updatedAsset, importRemap, order); err != nil {
		return nil, err
	}
	updatedAsset, err = uasset.ParseBytes(out, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse depends-patched asset: %w", err)
	}
	exportMutations, err := BuildExportPackageIndexRemapMutations(asset, updatedAsset, importRemap)
	if err != nil {
		return nil, err
	}
	if len(exportMutations) > 0 {
		out, err = RewriteAsset(updatedAsset, exportMutations)
		if err != nil {
			return nil, fmt.Errorf("rewrite export payload package indices: %w", err)
		}
	}
	updatedAsset, err = uasset.ParseBytes(out, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse export-payload-patched asset: %w", err)
	}
	if err := patchShiftedPreloadDependencyImportIndices(out, asset, updatedAsset, importRemap, order); err != nil {
		return nil, err
	}
	updatedAsset, err = uasset.ParseBytes(out, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse preload-patched asset: %w", err)
	}
	out, err = rewriteAssetRegistryImportedClassesTrailer(updatedAsset, importRemap, insertAt, entries)
	if err != nil {
		return nil, fmt.Errorf("rewrite asset registry imported classes: %w", err)
	}
	if err := FinalizePackageBytes(out, asset.Summary.FileVersionUE5); err != nil {
		return nil, fmt.Errorf("finalize package bytes: %w", err)
	}
	return out, nil
}

// RemoveImportEntries removes the selected zero-based ImportMap entries and
// remaps remaining negative import references in import/export headers,
// DependsMap, export payloads, preload dependencies, and the asset registry
// imported-classes trailer.
func RemoveImportEntries(asset *uasset.Asset, removeIndices []int) ([]byte, error) {
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
		if idx < 0 || idx >= len(asset.Imports) {
			return nil, fmt.Errorf("remove import index out of range: %d", idx)
		}
		if removeSet[idx] {
			return nil, fmt.Errorf("duplicate remove import index: %d", idx)
		}
		removeSet[idx] = true
	}
	if len(removeSet) == len(asset.Imports) {
		return nil, fmt.Errorf("removing every import is unsupported")
	}

	keepEntries := make([]uasset.ImportEntry, 0, len(asset.Imports)-len(removeSet))
	importRemap := make(map[int]int, len(asset.Imports)-len(removeSet))
	for i, entry := range asset.Imports {
		if removeSet[i] {
			continue
		}
		importRemap[i] = len(keepEntries)
		keepEntries = append(keepEntries, entry)
	}
	for i := range keepEntries {
		next, err := remapImportEntryForDeletion(keepEntries[i], importRemap, removeSet)
		if err != nil {
			return nil, fmt.Errorf("remap import[%d]: %w", i+1, err)
		}
		keepEntries[i] = next
	}

	start := int64(asset.Summary.ImportOffset)
	if start < 0 || start > int64(len(asset.Raw.Bytes)) {
		return nil, fmt.Errorf("import offset out of range: %d", asset.Summary.ImportOffset)
	}
	end, err := findImportMapEndOffset(asset, asset.Raw.Bytes)
	if err != nil {
		return nil, err
	}
	if end < start || end > int64(len(asset.Raw.Bytes)) {
		return nil, fmt.Errorf("invalid import map range: %d..%d", start, end)
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	out, err := RewriteRawRange(asset, start, end, encodeImportMapEntries(keepEntries, order, asset.Summary))
	if err != nil {
		return nil, fmt.Errorf("rewrite import map: %w", err)
	}
	importCountPos, err := scanSummaryImportCountPos(asset.Raw.Bytes, asset.Summary.FileVersionUE5)
	if err != nil {
		return nil, fmt.Errorf("scan summary import count: %w", err)
	}
	if err := writeInt32At(out, importCountPos, int32(len(keepEntries)), order); err != nil {
		return nil, fmt.Errorf("patch summary ImportCount: %w", err)
	}

	updatedAsset, err := uasset.ParseBytes(out, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse import-removed asset: %w", err)
	}
	if err := patchRemovedExportPackageIndices(out, asset, updatedAsset, importRemap, removeSet, order); err != nil {
		return nil, err
	}
	updatedAsset, err = uasset.ParseBytes(out, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse export-patched asset: %w", err)
	}
	if err := patchRemovedDependsPackageIndices(out, asset, updatedAsset, importRemap, removeSet, order); err != nil {
		return nil, err
	}
	updatedAsset, err = uasset.ParseBytes(out, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse depends-patched asset: %w", err)
	}
	exportMutations, err := BuildExportPackageIndexDeleteMutations(asset, updatedAsset, importRemap, removeSet)
	if err != nil {
		return nil, err
	}
	if len(exportMutations) > 0 {
		out, err = RewriteAsset(updatedAsset, exportMutations)
		if err != nil {
			return nil, fmt.Errorf("rewrite export payload package indices: %w", err)
		}
	}
	updatedAsset, err = uasset.ParseBytes(out, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse export-payload-patched asset: %w", err)
	}
	if err := patchRemovedPreloadDependencyImportIndices(out, asset, updatedAsset, importRemap, removeSet, order); err != nil {
		return nil, err
	}
	updatedAsset, err = uasset.ParseBytes(out, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("reparse preload-patched asset: %w", err)
	}
	out, err = rewriteAssetRegistryImportedClassesTrailerWithRemap(updatedAsset, len(keepEntries), importRemap, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("rewrite asset registry imported classes: %w", err)
	}
	if err := FinalizePackageBytes(out, asset.Summary.FileVersionUE5); err != nil {
		return nil, fmt.Errorf("finalize package bytes: %w", err)
	}
	return out, nil
}

func remapImportEntryForDeletion(entry uasset.ImportEntry, importRemap map[int]int, removeSet map[int]bool) (uasset.ImportEntry, error) {
	out := entry
	var err error
	out.OuterIndex, err = remapImportPackageIndexForDeletion(out.OuterIndex, importRemap, removeSet)
	if err != nil {
		return out, fmt.Errorf("OuterIndex: %w", err)
	}
	return out, nil
}

func encodeImportMapEntries(entries []uasset.ImportEntry, order binary.ByteOrder, summary uasset.PackageSummary) []byte {
	w := newByteWriter(order, len(entries)*40)
	for _, entry := range entries {
		w.writeNameRef(entry.ClassPackage.Index, entry.ClassPackage.Number)
		w.writeNameRef(entry.ClassName.Index, entry.ClassName.Number)
		w.writeInt32(int32(entry.OuterIndex))
		w.writeNameRef(entry.ObjectName.Index, entry.ObjectName.Number)
		if !summary.IsEditorOnlyFiltered() {
			w.writeNameRef(entry.PackageName.Index, entry.PackageName.Number)
		}
		if summary.FileVersionUE5 >= 1008 {
			w.writeUBool(entry.ImportOptional)
		}
	}
	return w.bytes()
}

func buildInsertedImportIndexRemap(oldCount, insertAt, inserted int) map[int]int {
	remap := make(map[int]int, oldCount)
	for i := 0; i < oldCount; i++ {
		next := i
		if i >= insertAt {
			next += inserted
		}
		remap[i] = next
	}
	return remap
}

func remapImportPackageIndex(idx uasset.PackageIndex, remap map[int]int) uasset.PackageIndex {
	if idx >= 0 {
		return idx
	}
	oldZero := idx.ResolveIndex()
	newZero, ok := remap[oldZero]
	if !ok || newZero == oldZero {
		return idx
	}
	return uasset.PackageIndex(-(newZero + 1))
}

func remapImportPackageIndexForDeletion(idx uasset.PackageIndex, remap map[int]int, removeSet map[int]bool) (uasset.PackageIndex, error) {
	if idx >= 0 {
		return idx, nil
	}
	oldZero := idx.ResolveIndex()
	if removeSet[oldZero] {
		return 0, fmt.Errorf("reference to removed import %d", oldZero+1)
	}
	newZero, ok := remap[oldZero]
	if !ok {
		return 0, fmt.Errorf("import remap missing kept import %d", oldZero+1)
	}
	return uasset.PackageIndex(-(newZero + 1)), nil
}

func patchShiftedPreloadDependencyImportIndices(out []byte, asset, updatedAsset *uasset.Asset, remap map[int]int, order binary.ByteOrder) error {
	if updatedAsset == nil || updatedAsset.Summary.PreloadDependencyOffset <= 0 {
		return nil
	}
	if updatedAsset.Summary.PreloadDependencyCount <= 0 {
		return nil
	}
	start := int(updatedAsset.Summary.PreloadDependencyOffset)
	// Limit the scan to the preload dependency section only.
	// The section contains PreloadDependencyCount int32 entries (4 bytes each).
	// Do NOT scan into export serial data that follows the header.
	end := start + int(updatedAsset.Summary.PreloadDependencyCount)*4
	headerEnd := int(updatedAsset.Summary.TotalHeaderSize)
	if end > headerEnd {
		end = headerEnd
	}
	if start < 0 || end < start || end > len(out) {
		return fmt.Errorf("preload dependency range out of bounds: %d..%d (size=%d)", start, end, len(out))
	}
	if end-start < 8 {
		return nil
	}

	serializedRemap := make(map[int32]int32, len(remap))
	for oldZero, newZero := range remap {
		oldSerialized := int32(oldZero + 1)
		newSerialized := int32(newZero + 1)
		if oldSerialized != newSerialized {
			serializedRemap[oldSerialized] = newSerialized
		}
	}
	if len(serializedRemap) == 0 {
		return nil
	}

	for pos := start; pos+8 <= end; pos += 4 {
		current, err := readInt32At(out, pos, order)
		if err != nil {
			return fmt.Errorf("read preload dependency value at %d: %w", pos, err)
		}
		next, err := readInt32At(out, pos+4, order)
		if err != nil {
			return fmt.Errorf("read preload dependency lookahead at %d: %w", pos+4, err)
		}
		replacement, ok := serializedRemap[current]
		if !ok || next != 0 {
			continue
		}
		if err := writeInt32At(out, pos, replacement, order); err != nil {
			return fmt.Errorf("patch preload dependency import index at %d: %w", pos, err)
		}
	}
	return nil
}

func patchRemovedPreloadDependencyImportIndices(out []byte, asset, updatedAsset *uasset.Asset, remap map[int]int, removeSet map[int]bool, order binary.ByteOrder) error {
	if updatedAsset == nil || updatedAsset.Summary.PreloadDependencyOffset <= 0 {
		return nil
	}
	if updatedAsset.Summary.PreloadDependencyCount <= 0 {
		return nil
	}
	start := int(updatedAsset.Summary.PreloadDependencyOffset)
	end := start + int(updatedAsset.Summary.PreloadDependencyCount)*4
	headerEnd := int(updatedAsset.Summary.TotalHeaderSize)
	if end > headerEnd {
		end = headerEnd
	}
	if start < 0 || end < start || end > len(out) {
		return fmt.Errorf("preload dependency range out of bounds: %d..%d (size=%d)", start, end, len(out))
	}
	if end-start < 8 {
		return nil
	}

	serializedRemap := make(map[int32]int32, len(remap))
	serializedRemoved := make(map[int32]bool, len(removeSet))
	for oldZero := range removeSet {
		serializedRemoved[int32(oldZero+1)] = true
	}
	for oldZero, newZero := range remap {
		oldSerialized := int32(oldZero + 1)
		newSerialized := int32(newZero + 1)
		if oldSerialized != newSerialized {
			serializedRemap[oldSerialized] = newSerialized
		}
	}

	for pos := start; pos+8 <= end; pos += 4 {
		current, err := readInt32At(out, pos, order)
		if err != nil {
			return fmt.Errorf("read preload dependency value at %d: %w", pos, err)
		}
		next, err := readInt32At(out, pos+4, order)
		if err != nil {
			return fmt.Errorf("read preload dependency lookahead at %d: %w", pos+4, err)
		}
		if next != 0 {
			continue
		}
		if serializedRemoved[current] {
			return fmt.Errorf("preload dependency still references removed import %d", current)
		}
		replacement, ok := serializedRemap[current]
		if !ok || replacement == current {
			continue
		}
		if err := writeInt32At(out, pos, replacement, order); err != nil {
			return fmt.Errorf("patch preload dependency import index at %d: %w", pos, err)
		}
	}
	return nil
}

func patchShiftedImportOuterIndices(out []byte, asset, updatedAsset *uasset.Asset, remap map[int]int, order binary.ByteOrder) error {
	fields, err := scanImportOuterIndexPositions(out, updatedAsset)
	if err != nil {
		return fmt.Errorf("scan import outer indices: %w", err)
	}
	if len(fields) != len(updatedAsset.Imports) {
		return fmt.Errorf("import outer field scan mismatch: got %d want %d", len(fields), len(updatedAsset.Imports))
	}
	for oldIndex, newIndex := range remap {
		if oldIndex < 0 || oldIndex >= len(asset.Imports) || newIndex < 0 || newIndex >= len(updatedAsset.Imports) {
			return fmt.Errorf("import remap out of range: old=%d new=%d", oldIndex, newIndex)
		}
		oldIdx := asset.Imports[oldIndex].OuterIndex
		newIdx := remapImportPackageIndex(oldIdx, remap)
		if newIdx == oldIdx {
			continue
		}
		if err := writeInt32At(out, fields[newIndex].outerIndexPos, int32(newIdx), order); err != nil {
			return fmt.Errorf("patch import[%d] outer index: %w", newIndex+1, err)
		}
	}
	return nil
}

func patchShiftedExportPackageIndices(out []byte, asset, updatedAsset *uasset.Asset, remap map[int]int, order binary.ByteOrder) error {
	fields, err := scanExportPackageIndexPositions(out, updatedAsset)
	if err != nil {
		return fmt.Errorf("scan export package indices: %w", err)
	}
	if len(fields) != len(updatedAsset.Exports) {
		return fmt.Errorf("export package index field scan mismatch: got %d want %d", len(fields), len(updatedAsset.Exports))
	}
	for i, field := range fields {
		type pair struct {
			pos int
			idx uasset.PackageIndex
		}
		for _, item := range []pair{
			{pos: field.classIndexPos, idx: asset.Exports[i].ClassIndex},
			{pos: field.superIndexPos, idx: asset.Exports[i].SuperIndex},
			{pos: field.templateIndexPos, idx: asset.Exports[i].TemplateIndex},
			{pos: field.outerIndexPos, idx: asset.Exports[i].OuterIndex},
		} {
			newIdx := remapImportPackageIndex(item.idx, remap)
			if newIdx == item.idx {
				continue
			}
			if err := writeInt32At(out, item.pos, int32(newIdx), order); err != nil {
				return fmt.Errorf("patch export[%d] package index: %w", i+1, err)
			}
		}
	}
	return nil
}

func patchRemovedExportPackageIndices(out []byte, asset, updatedAsset *uasset.Asset, remap map[int]int, removeSet map[int]bool, order binary.ByteOrder) error {
	fields, err := scanExportPackageIndexPositions(out, updatedAsset)
	if err != nil {
		return fmt.Errorf("scan export package indices: %w", err)
	}
	if len(fields) != len(updatedAsset.Exports) {
		return fmt.Errorf("export package index field scan mismatch: got %d want %d", len(fields), len(updatedAsset.Exports))
	}
	for i, field := range fields {
		type pair struct {
			pos  int
			idx  uasset.PackageIndex
			name string
		}
		for _, item := range []pair{
			{pos: field.classIndexPos, idx: asset.Exports[i].ClassIndex, name: "ClassIndex"},
			{pos: field.superIndexPos, idx: asset.Exports[i].SuperIndex, name: "SuperIndex"},
			{pos: field.templateIndexPos, idx: asset.Exports[i].TemplateIndex, name: "TemplateIndex"},
			{pos: field.outerIndexPos, idx: asset.Exports[i].OuterIndex, name: "OuterIndex"},
		} {
			newIdx, err := remapImportPackageIndexForDeletion(item.idx, remap, removeSet)
			if err != nil {
				return fmt.Errorf("export[%d] %s: %w", i+1, item.name, err)
			}
			if newIdx == item.idx {
				continue
			}
			if err := writeInt32At(out, item.pos, int32(newIdx), order); err != nil {
				return fmt.Errorf("patch export[%d] package index: %w", i+1, err)
			}
		}
	}
	return nil
}

func findImportMapEndOffset(asset *uasset.Asset, raw []byte) (int64, error) {
	if asset == nil {
		return 0, fmt.Errorf("asset is nil")
	}
	start := int64(asset.Summary.ImportOffset)
	if start < 0 {
		return 0, fmt.Errorf("import offset is negative: %d", asset.Summary.ImportOffset)
	}

	candidates := []int64{
		int64(asset.Summary.ExportOffset),
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
		int64(asset.Summary.TotalHeaderSize),
	}

	end := int64(len(raw))
	for _, off := range candidates {
		if off <= start {
			continue
		}
		if off > int64(len(raw)) {
			continue
		}
		if off < end {
			end = off
		}
	}
	if end < start {
		return 0, fmt.Errorf("could not determine ImportMap end offset")
	}
	return end, nil
}

func scanImportOuterIndexPositions(data []byte, asset *uasset.Asset) ([]importOuterIndexPatch, error) {
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

	fields := make([]importOuterIndexPatch, 0, len(asset.Imports))
	for i := 0; i < len(asset.Imports); i++ {
		if err := r.skip(8); err != nil {
			return nil, fmt.Errorf("import[%d] class package: %w", i+1, err)
		}
		if err := r.skip(8); err != nil {
			return nil, fmt.Errorf("import[%d] class name: %w", i+1, err)
		}
		patch := importOuterIndexPatch{outerIndexPos: r.off}
		if err := r.skip(4); err != nil {
			return nil, fmt.Errorf("import[%d] outer index: %w", i+1, err)
		}
		if err := r.skip(8); err != nil {
			return nil, fmt.Errorf("import[%d] object name: %w", i+1, err)
		}
		if !asset.Summary.IsEditorOnlyFiltered() {
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

func scanImportEntryStartOffsets(data []byte, asset *uasset.Asset) ([]int, error) {
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

	offsets := make([]int, 0, len(asset.Imports))
	for i := 0; i < len(asset.Imports); i++ {
		offsets = append(offsets, r.off)
		if err := r.skip(8); err != nil {
			return nil, fmt.Errorf("import[%d] class package: %w", i+1, err)
		}
		if err := r.skip(8); err != nil {
			return nil, fmt.Errorf("import[%d] class name: %w", i+1, err)
		}
		if err := r.skip(4); err != nil {
			return nil, fmt.Errorf("import[%d] outer index: %w", i+1, err)
		}
		if err := r.skip(8); err != nil {
			return nil, fmt.Errorf("import[%d] object name: %w", i+1, err)
		}
		if !asset.Summary.IsEditorOnlyFiltered() {
			if err := r.skip(8); err != nil {
				return nil, fmt.Errorf("import[%d] package name: %w", i+1, err)
			}
		}
		if asset.Summary.FileVersionUE5 >= ue5OptionalResources {
			if err := r.skip(4); err != nil {
				return nil, fmt.Errorf("import[%d] optional flag: %w", i+1, err)
			}
		}
	}
	return offsets, nil
}

func scanExportPackageIndexPositions(data []byte, asset *uasset.Asset) ([]exportPackageIndexPatch, error) {
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

	fields := make([]exportPackageIndexPatch, 0, len(asset.Exports))
	for i := 0; i < len(asset.Exports); i++ {
		patch := exportPackageIndexPatch{
			classIndexPos:    r.off,
			superIndexPos:    r.off + 4,
			templateIndexPos: r.off + 8,
			outerIndexPos:    r.off + 12,
		}
		if err := r.skip(4 * 4); err != nil {
			return nil, fmt.Errorf("export[%d] class/super/template/outer: %w", i+1, err)
		}
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

func scanSummaryImportCountPos(data []byte, unversionedFileUE5 int32) (int, error) {
	if len(data) < 4 {
		return 0, fmt.Errorf("file too small")
	}
	tagLE := binary.LittleEndian.Uint32(data[:4])
	var order binary.ByteOrder = binary.LittleEndian
	switch tagLE {
	case packageFileTag:
		order = binary.LittleEndian
	case packageFileTagSwapped:
		order = binary.BigEndian
	default:
		return 0, fmt.Errorf("invalid package tag: 0x%x", tagLE)
	}

	r := newByteCodec(data, order)
	if _, err := r.readInt32(); err != nil {
		return 0, err
	}
	legacy, err := r.readInt32()
	if err != nil {
		return 0, err
	}
	if legacy != -4 {
		if _, err := r.readInt32(); err != nil {
			return 0, err
		}
	}
	fileUE4, err := r.readInt32()
	if err != nil {
		return 0, err
	}
	fileUE5, err := r.readInt32()
	if err != nil {
		return 0, err
	}
	fileLicensee, err := r.readInt32()
	if err != nil {
		return 0, err
	}
	if fileUE4 == 0 && fileUE5 == 0 && fileLicensee == 0 {
		fileUE4 = ue4VersionUE56
		if unversionedFileUE5 >= ue5MinimumKnown {
			fileUE5 = unversionedFileUE5
		} else {
			fileUE5 = ue5ImportTypeHierarchies
		}
	}
	if fileUE5 >= ue5PackageSavedHash {
		if err := r.skip(20); err != nil {
			return 0, err
		}
		if _, err := r.readInt32(); err != nil {
			return 0, err
		}
	}
	if legacy <= -2 {
		if err := skipSummaryCustomVersions(r, legacy); err != nil {
			return 0, err
		}
	}
	if fileUE5 < ue5PackageSavedHash {
		if _, err := r.readInt32(); err != nil {
			return 0, err
		}
	}
	if _, err := r.readFString(); err != nil {
		return 0, err
	}
	packageFlags, err := r.readUint32()
	if err != nil {
		return 0, err
	}
	if _, err := r.readInt32(); err != nil {
		return 0, err
	}
	if _, err := r.readInt32(); err != nil {
		return 0, err
	}
	if fileUE5 >= ue5AddSoftObjectPathList {
		if _, err := r.readInt32(); err != nil {
			return 0, err
		}
		if _, err := r.readInt32(); err != nil {
			return 0, err
		}
	}
	if packageFlags&pkgFlagFilterEditorOnly == 0 {
		if _, err := r.readFString(); err != nil {
			return 0, err
		}
	}
	if _, err := r.readInt32(); err != nil {
		return 0, err
	}
	if _, err := r.readInt32(); err != nil {
		return 0, err
	}
	if _, err := r.readInt32(); err != nil {
		return 0, err
	}
	if _, err := r.readInt32(); err != nil {
		return 0, err
	}
	importCountPos := r.off
	if _, err := r.readInt32(); err != nil {
		return 0, err
	}
	return importCountPos, nil
}
