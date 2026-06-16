package edit

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

const assetRegistryImportedClassesTrailerPrefix = byte(0)

type assetRegistryImportedClassesTrailer struct {
	Prefix      byte
	ImportCount uint32
	ImportFlags []uint32
	TailA       uint32
	TailB       uint32
	TailC       uint32
}

func rewriteAssetRegistryImportedClassesTrailer(asset *uasset.Asset, importRemap map[int]int, insertAt int, insertedEntries []uasset.ImportEntry) ([]byte, error) {
	return rewriteAssetRegistryImportedClassesTrailerWithRemap(asset, len(asset.Imports), importRemap, insertAt, insertedEntries)
}

func rewriteAssetRegistryImportedClassesTrailerWithRemap(asset *uasset.Asset, newImportCount int, importRemap map[int]int, insertAt int, insertedEntries []uasset.ImportEntry) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if len(asset.Raw.Bytes) == 0 {
		return nil, fmt.Errorf("asset has no raw bytes")
	}

	sectionBytes, sectionStart, sectionEnd, present := assetRegistrySectionByOffset(asset, int64(asset.Summary.AssetRegistryDataOffset))
	if !present {
		return append([]byte(nil), asset.Raw.Bytes...), nil
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	trailer, trailerStart, err := parseAssetRegistryImportedClassesTrailer(sectionBytes, order)
	if err != nil {
		return nil, err
	}
	if trailer == nil {
		return append([]byte(nil), asset.Raw.Bytes...), nil
	}

	rewritten, err := trailer.remap(newImportCount, importRemap, insertAt, insertedEntries)
	if err != nil {
		return nil, err
	}
	replacement := encodeAssetRegistryImportedClassesTrailer(rewritten, order)
	if bytes.Equal(sectionBytes[trailerStart:], replacement) {
		return append([]byte(nil), asset.Raw.Bytes...), nil
	}

	out, err := RewriteRawRange(asset, sectionStart+int64(trailerStart), sectionEnd, replacement)
	if err != nil {
		return nil, fmt.Errorf("rewrite asset registry imported classes trailer: %w", err)
	}
	return out, nil
}

func parseAssetRegistryImportedClassesTrailer(sectionBytes []byte, order binary.ByteOrder) (*assetRegistryImportedClassesTrailer, int, error) {
	if len(sectionBytes) == 0 {
		return nil, 0, nil
	}

	type candidate struct {
		trailer *assetRegistryImportedClassesTrailer
		start   int
	}
	matches := make([]candidate, 0, 1)

	for wordCount := 0; ; wordCount++ {
		trailerLen := 17 + wordCount*4
		if trailerLen > len(sectionBytes) {
			break
		}
		start := len(sectionBytes) - trailerLen
		if sectionBytes[start] != assetRegistryImportedClassesTrailerPrefix {
			continue
		}

		count := order.Uint32(sectionBytes[start+1 : start+5])
		expectedWords := 0
		if count > 0 {
			expectedWords = int((count + 31) / 32)
		}
		if expectedWords != wordCount {
			continue
		}

		tailPos := start + 5 + wordCount*4
		if tailPos+12 != len(sectionBytes) {
			continue
		}

		tailA := order.Uint32(sectionBytes[tailPos : tailPos+4])
		tailB := order.Uint32(sectionBytes[tailPos+4 : tailPos+8])
		tailC := order.Uint32(sectionBytes[tailPos+8 : tailPos+12])
		if tailC != 0 || tailA == 0 || tailB == 0 {
			continue
		}

		flags := make([]uint32, wordCount)
		for i := 0; i < wordCount; i++ {
			flagPos := start + 5 + i*4
			flags[i] = order.Uint32(sectionBytes[flagPos : flagPos+4])
		}
		if hasImportedClassBitsOutsideCount(flags, int(count)) {
			continue
		}

		matches = append(matches, candidate{
			trailer: &assetRegistryImportedClassesTrailer{
				Prefix:      sectionBytes[start],
				ImportCount: count,
				ImportFlags: flags,
				TailA:       tailA,
				TailB:       tailB,
				TailC:       tailC,
			},
			start: start,
		})
	}

	switch len(matches) {
	case 0:
		return nil, 0, fmt.Errorf("asset registry imported classes trailer layout is unsupported")
	case 1:
		return matches[0].trailer, matches[0].start, nil
	default:
		return nil, 0, fmt.Errorf("asset registry imported classes trailer layout is ambiguous")
	}
}

func encodeAssetRegistryImportedClassesTrailer(trailer *assetRegistryImportedClassesTrailer, order binary.ByteOrder) []byte {
	if trailer == nil {
		return nil
	}
	w := newByteWriter(order, 17+len(trailer.ImportFlags)*4)
	w.writeUint8(trailer.Prefix)
	w.writeUint32(trailer.ImportCount)
	for _, flags := range trailer.ImportFlags {
		w.writeUint32(flags)
	}
	w.writeUint32(trailer.TailA)
	w.writeUint32(trailer.TailB)
	w.writeUint32(trailer.TailC)
	return w.bytes()
}

func (t *assetRegistryImportedClassesTrailer) remap(newImportCount int, importRemap map[int]int, insertAt int, insertedEntries []uasset.ImportEntry) (*assetRegistryImportedClassesTrailer, error) {
	if t == nil {
		return nil, fmt.Errorf("asset registry imported classes trailer is nil")
	}
	if newImportCount < 0 {
		return nil, fmt.Errorf("new import count is negative: %d", newImportCount)
	}
	if insertAt < 0 || insertAt > newImportCount {
		return nil, fmt.Errorf("insert position out of range for imported classes trailer: %d", insertAt)
	}

	wordCount := 0
	if newImportCount > 0 {
		wordCount = (newImportCount + 31) / 32
	}
	remappedFlags := make([]uint32, wordCount)
	deletingImports := len(insertedEntries) == 0 && newImportCount < int(t.ImportCount)

	for oldZero := 0; oldZero < int(t.ImportCount); oldZero++ {
		if !isImportedClassBitSet(t.ImportFlags, oldZero) {
			continue
		}
		newZero, ok := importRemap[oldZero]
		if !ok {
			if deletingImports {
				continue
			}
			return nil, fmt.Errorf("asset registry imported classes trailer remap missing import %d", oldZero+1)
		}
		if newZero < 0 || newZero >= newImportCount {
			return nil, fmt.Errorf("asset registry imported classes trailer remap out of range: import %d -> %d", oldZero+1, newZero+1)
		}
		setImportedClassBit(remappedFlags, newZero)
	}

	for i, entry := range insertedEntries {
		if entry.ImportOptional {
			continue
		}
		newZero := insertAt + i
		if newZero < 0 || newZero >= newImportCount {
			return nil, fmt.Errorf("inserted import bit out of range: %d", newZero+1)
		}
		setImportedClassBit(remappedFlags, newZero)
	}

	return &assetRegistryImportedClassesTrailer{
		Prefix:      t.Prefix,
		ImportCount: uint32(newImportCount),
		ImportFlags: remappedFlags,
		TailA:       t.TailA,
		TailB:       t.TailB,
		TailC:       t.TailC,
	}, nil
}

func isImportedClassBitSet(words []uint32, zeroIndex int) bool {
	if zeroIndex < 0 {
		return false
	}
	word := zeroIndex / 32
	if word >= len(words) {
		return false
	}
	bit := uint(zeroIndex % 32)
	return words[word]&(uint32(1)<<bit) != 0
}

func setImportedClassBit(words []uint32, zeroIndex int) {
	if zeroIndex < 0 {
		return
	}
	word := zeroIndex / 32
	if word >= len(words) {
		return
	}
	bit := uint(zeroIndex % 32)
	words[word] |= uint32(1) << bit
}

func hasImportedClassBitsOutsideCount(words []uint32, count int) bool {
	if count < 0 {
		return true
	}
	for zeroIndex := count; zeroIndex < len(words)*32; zeroIndex++ {
		if isImportedClassBitSet(words, zeroIndex) {
			return true
		}
	}
	return false
}

func assetRegistrySectionByOffset(asset *uasset.Asset, start int64) ([]byte, int64, int64, bool) {
	fileSize := int64(len(asset.Raw.Bytes))
	if start <= 0 || start >= fileSize {
		return nil, 0, 0, false
	}
	end := nextKnownPackageOffset(asset, start)
	if end <= start || end > fileSize {
		end = fileSize
	}
	return asset.Raw.Bytes[start:end], start, end, true
}

func nextKnownPackageOffset(asset *uasset.Asset, start int64) int64 {
	candidates := make([]int64, 0, 64)
	add := func(v int64) {
		if v > start && v <= int64(len(asset.Raw.Bytes)) {
			candidates = append(candidates, v)
		}
	}

	s := asset.Summary
	add(int64(s.TotalHeaderSize))
	add(int64(s.NameOffset))
	add(int64(s.SoftObjectPathsOffset))
	add(int64(s.GatherableTextDataOffset))
	add(int64(s.ExportOffset))
	add(int64(s.ImportOffset))
	add(int64(s.CellExportOffset))
	add(int64(s.CellImportOffset))
	add(int64(s.MetaDataOffset))
	add(int64(s.DependsOffset))
	add(int64(s.SoftPackageReferencesOffset))
	add(int64(s.SearchableNamesOffset))
	add(int64(s.ThumbnailTableOffset))
	add(int64(s.ImportTypeHierarchiesOffset))
	add(int64(s.AssetRegistryDataOffset))
	add(s.BulkDataStartOffset)
	add(int64(s.WorldTileInfoDataOffset))
	add(int64(s.PreloadDependencyOffset))
	add(s.PayloadTOCOffset)
	add(int64(s.DataResourceOffset))
	for _, exp := range asset.Exports {
		add(exp.SerialOffset)
		add(exp.SerialOffset + exp.SerialSize)
	}
	add(int64(len(asset.Raw.Bytes)))

	if len(candidates) == 0 {
		return int64(len(asset.Raw.Bytes))
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i] < candidates[j] })
	return candidates[0]
}
