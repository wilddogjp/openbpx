package edit

import (
	"encoding/binary"
	"fmt"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

// ReplaceExportDependencies rewrites one export's DependsMap entry while
// preserving all other export dependency lists.
func ReplaceExportDependencies(asset *uasset.Asset, exportIndex int, deps []uasset.PackageIndex) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if exportIndex < 0 || exportIndex >= len(asset.Exports) {
		return nil, fmt.Errorf("export index out of range: %d", exportIndex)
	}
	entries, start, end, err := parseDependsMapEntries(asset)
	if err != nil {
		return nil, err
	}
	entries[exportIndex] = append([]uasset.PackageIndex(nil), deps...)

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	out, err := RewriteRawRange(asset, start, end, encodeDependsMapEntries(entries, order))
	if err != nil {
		return nil, fmt.Errorf("rewrite depends map: %w", err)
	}
	return out, nil
}

// AppendExportDependencies appends one or more package indices to one export's
// DependsMap entry, preserving existing order and skipping duplicates.
func AppendExportDependencies(asset *uasset.Asset, exportIndex int, deps []uasset.PackageIndex) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if exportIndex < 0 || exportIndex >= len(asset.Exports) {
		return nil, fmt.Errorf("export index out of range: %d", exportIndex)
	}
	if len(deps) == 0 {
		return append([]byte(nil), asset.Raw.Bytes...), nil
	}

	entries, start, end, err := parseDependsMapEntries(asset)
	if err != nil {
		return nil, err
	}

	existing := append([]uasset.PackageIndex(nil), entries[exportIndex]...)
	for _, dep := range deps {
		if dependsSliceContains(existing, dep) {
			continue
		}
		existing = append(existing, dep)
	}
	entries[exportIndex] = existing

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	out, err := RewriteRawRange(asset, start, end, encodeDependsMapEntries(entries, order))
	if err != nil {
		return nil, fmt.Errorf("rewrite depends map: %w", err)
	}
	return out, nil
}

func dependsSliceContains(items []uasset.PackageIndex, target uasset.PackageIndex) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func parseDependsMapEntries(asset *uasset.Asset) ([][]uasset.PackageIndex, int64, int64, error) {
	if asset == nil {
		return nil, 0, 0, fmt.Errorf("asset is nil")
	}
	raw := asset.Raw.Bytes
	start := int64(asset.Summary.DependsOffset)
	if start == 0 {
		return nil, 0, 0, fmt.Errorf("depends map section not present")
	}
	if start < 0 || start > int64(len(raw)) {
		return nil, 0, 0, fmt.Errorf("depends offset out of range: %d", asset.Summary.DependsOffset)
	}
	end, err := findDependsMapEndOffset(asset, raw)
	if err != nil {
		return nil, 0, 0, err
	}
	if end < start || end > int64(len(raw)) {
		return nil, 0, 0, fmt.Errorf("invalid depends map range: %d..%d", start, end)
	}

	r := uasset.NewByteReaderWithByteSwapping(raw[start:end], asset.Summary.UsesByteSwappedSerialization())
	items := make([][]uasset.PackageIndex, len(asset.Exports))
	for i := range asset.Exports {
		count, err := r.ReadInt32()
		if err != nil {
			return nil, 0, 0, fmt.Errorf("export[%d] read dependency count: %w", i+1, err)
		}
		if count < 0 || count > 1_000_000 {
			return nil, 0, 0, fmt.Errorf("export[%d] invalid dependency count: %d", i+1, count)
		}
		deps := make([]uasset.PackageIndex, 0, count)
		for j := int32(0); j < count; j++ {
			rawIdx, err := r.ReadInt32()
			if err != nil {
				return nil, 0, 0, fmt.Errorf("export[%d] read dependency %d: %w", i+1, j, err)
			}
			deps = append(deps, uasset.PackageIndex(rawIdx))
		}
		items[i] = deps
	}
	return items, start, end, nil
}

func encodeDependsMapEntries(entries [][]uasset.PackageIndex, order binary.ByteOrder) []byte {
	total := 0
	for _, deps := range entries {
		total += 4 + len(deps)*4
	}
	w := newByteWriter(order, total)
	for _, deps := range entries {
		w.writeInt32(int32(len(deps)))
		for _, dep := range deps {
			w.writeInt32(int32(dep))
		}
	}
	return w.bytes()
}

func findDependsMapEndOffset(asset *uasset.Asset, raw []byte) (int64, error) {
	if asset == nil {
		return 0, fmt.Errorf("asset is nil")
	}
	start := int64(asset.Summary.DependsOffset)
	if start < 0 {
		return 0, fmt.Errorf("depends offset is negative: %d", asset.Summary.DependsOffset)
	}

	candidates := []int64{
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
		if off <= start || off > int64(len(raw)) {
			continue
		}
		if off < end {
			end = off
		}
	}
	if end < start {
		return 0, fmt.Errorf("could not determine DependsMap end offset")
	}
	return end, nil
}

func patchShiftedDependsPackageIndices(out []byte, asset, updatedAsset *uasset.Asset, remap map[int]int, order binary.ByteOrder) error {
	if asset == nil || asset.Summary.DependsOffset == 0 {
		return nil
	}
	oldEntries, _, _, err := parseDependsMapEntries(asset)
	if err != nil {
		return fmt.Errorf("parse original depends map: %w", err)
	}
	positions, err := scanDependsPackageIndexPositions(out, updatedAsset)
	if err != nil {
		return fmt.Errorf("scan depends package indices: %w", err)
	}
	if len(oldEntries) != len(positions) {
		return fmt.Errorf("depends map position mismatch: got %d want %d", len(positions), len(oldEntries))
	}
	for i := range oldEntries {
		if len(oldEntries[i]) != len(positions[i]) {
			return fmt.Errorf("export[%d] depends entry count mismatch: got %d want %d", i+1, len(positions[i]), len(oldEntries[i]))
		}
		for j, dep := range oldEntries[i] {
			newDep := remapImportPackageIndex(dep, remap)
			if newDep == dep {
				continue
			}
			if err := writeInt32At(out, positions[i][j], int32(newDep), order); err != nil {
				return fmt.Errorf("patch export[%d] dependency[%d]: %w", i+1, j, err)
			}
		}
	}
	return nil
}

func patchRemovedDependsPackageIndices(out []byte, asset, updatedAsset *uasset.Asset, remap map[int]int, removeSet map[int]bool, order binary.ByteOrder) error {
	if asset == nil || asset.Summary.DependsOffset == 0 {
		return nil
	}
	oldEntries, _, _, err := parseDependsMapEntries(asset)
	if err != nil {
		return fmt.Errorf("parse original depends map: %w", err)
	}
	positions, err := scanDependsPackageIndexPositions(out, updatedAsset)
	if err != nil {
		return fmt.Errorf("scan depends package indices: %w", err)
	}
	if len(oldEntries) != len(positions) {
		return fmt.Errorf("depends map position mismatch: got %d want %d", len(positions), len(oldEntries))
	}
	for i := range oldEntries {
		if len(oldEntries[i]) != len(positions[i]) {
			return fmt.Errorf("export[%d] depends entry count mismatch: got %d want %d", i+1, len(positions[i]), len(oldEntries[i]))
		}
		for j, dep := range oldEntries[i] {
			newDep, err := remapImportPackageIndexForDeletion(dep, remap, removeSet)
			if err != nil {
				return fmt.Errorf("export[%d] dependency[%d]: %w", i+1, j, err)
			}
			if newDep == dep {
				continue
			}
			if err := writeInt32At(out, positions[i][j], int32(newDep), order); err != nil {
				return fmt.Errorf("patch export[%d] dependency[%d]: %w", i+1, j, err)
			}
		}
	}
	return nil
}

func scanDependsPackageIndexPositions(data []byte, asset *uasset.Asset) ([][]int, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	_, start, end, err := parseDependsMapEntries(asset)
	if err != nil {
		return nil, err
	}
	r := uasset.NewByteReaderWithByteSwapping(data[start:end], asset.Summary.UsesByteSwappedSerialization())
	positions := make([][]int, 0, len(asset.Exports))
	for i := range asset.Exports {
		count, err := r.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("export[%d] read dependency count: %w", i+1, err)
		}
		if count < 0 || count > 1_000_000 {
			return nil, fmt.Errorf("export[%d] invalid dependency count: %d", i+1, count)
		}
		deps := make([]int, 0, count)
		for j := int32(0); j < count; j++ {
			deps = append(deps, int(start+int64(r.Offset())))
			if _, err := r.ReadInt32(); err != nil {
				return nil, fmt.Errorf("export[%d] read dependency %d: %w", i+1, j, err)
			}
		}
		positions = append(positions, deps)
	}
	return positions, nil
}
