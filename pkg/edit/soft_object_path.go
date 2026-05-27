package edit

import (
	"encoding/binary"
	"fmt"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

// SoftObjectPathEntry is one decoded soft object path list entry.
type SoftObjectPathEntry struct {
	PackageName uasset.NameRef
	AssetName   uasset.NameRef
	SubPath     string
}

// ReadSoftObjectPathEntries decodes the package soft object path list.
func ReadSoftObjectPathEntries(asset *uasset.Asset) ([]SoftObjectPathEntry, error) {
	entries, _, _, err := readSoftObjectPathEntriesWithRange(asset)
	return entries, err
}

// RewriteSoftObjectPathEntries replaces the package soft object path list with
// the provided entries. The entry count must stay unchanged so the summary
// count remains valid.
func RewriteSoftObjectPathEntries(asset *uasset.Asset, entries []SoftObjectPathEntry) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	current, start, end, err := readSoftObjectPathEntriesWithRange(asset)
	if err != nil {
		return nil, err
	}
	if len(current) != len(entries) {
		return nil, fmt.Errorf("soft object path entry count mismatch: got %d want %d", len(entries), len(current))
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}
	w := newByteWriter(order, int(end-start))
	for i, entry := range entries {
		if entry.PackageName.Index < 0 || int(entry.PackageName.Index) >= len(asset.Names) {
			return nil, fmt.Errorf("soft object path[%d] package name index out of range: %d", i, entry.PackageName.Index)
		}
		if entry.AssetName.Index < 0 || int(entry.AssetName.Index) >= len(asset.Names) {
			return nil, fmt.Errorf("soft object path[%d] asset name index out of range: %d", i, entry.AssetName.Index)
		}
		w.writeNameRef(entry.PackageName.Index, entry.PackageName.Number)
		w.writeNameRef(entry.AssetName.Index, entry.AssetName.Number)
		w.writeUTF8String(entry.SubPath)
	}
	replacement := w.bytes()
	if len(replacement) != int(end-start) {
		return nil, fmt.Errorf("rewritten soft object path section size mismatch: got %d want %d", len(replacement), int(end-start))
	}
	return RewriteRawRange(asset, start, end, replacement)
}

func readSoftObjectPathEntriesWithRange(asset *uasset.Asset) ([]SoftObjectPathEntry, int64, int64, error) {
	if asset == nil {
		return nil, 0, 0, fmt.Errorf("asset is nil")
	}
	if asset.Summary.SoftObjectPathsCount <= 0 || asset.Summary.SoftObjectPathsOffset <= 0 || !asset.Summary.SupportsSoftObjectPathListInSummary() {
		return nil, 0, 0, fmt.Errorf("asset has no soft object path list")
	}
	start := int64(asset.Summary.SoftObjectPathsOffset)
	end := nextKnownOffsetWithinFile(asset, start)
	if start < 0 || end < start || end > int64(len(asset.Raw.Bytes)) {
		return nil, 0, 0, fmt.Errorf("soft object path range out of bounds: %d..%d (size=%d)", start, end, len(asset.Raw.Bytes))
	}

	r := uasset.NewByteReaderWithByteSwapping(asset.Raw.Bytes[start:end], asset.Summary.UsesByteSwappedSerialization())
	entries := make([]SoftObjectPathEntry, 0, int(asset.Summary.SoftObjectPathsCount))
	for i := int32(0); i < asset.Summary.SoftObjectPathsCount; i++ {
		pkg, err := r.ReadNameRef(len(asset.Names))
		if err != nil {
			return nil, 0, 0, fmt.Errorf("soft object path[%d] package name: %w", i, err)
		}
		assetName, err := r.ReadNameRef(len(asset.Names))
		if err != nil {
			return nil, 0, 0, fmt.Errorf("soft object path[%d] asset name: %w", i, err)
		}
		subPath, err := r.ReadSoftObjectSubPath()
		if err != nil {
			return nil, 0, 0, fmt.Errorf("soft object path[%d] sub path: %w", i, err)
		}
		entries = append(entries, SoftObjectPathEntry{
			PackageName: pkg,
			AssetName:   assetName,
			SubPath:     subPath,
		})
	}
	if r.Offset() != int(end-start) {
		return nil, 0, 0, fmt.Errorf("soft object path section has trailing bytes: consumed=%d size=%d", r.Offset(), int(end-start))
	}
	return entries, start, end, nil
}
