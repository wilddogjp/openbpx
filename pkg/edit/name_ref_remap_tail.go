package edit

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

// PatchInstancedTailNameRefs scans export tails for instanced NameRef pairs
// (Name index + Number where Number > 0, e.g. "Image_22" stored as
// {nameIndex("Image"), 23}) and rewrites them using the ORIGINAL payload bytes
// as the source of truth. This avoids cascading remaps when a rewritten NameRef
// index is itself also a valid old NameMap index.
func PatchInstancedTailNameRefs(oldAsset, asset *uasset.Asset, indexRemap map[int32]int32, opts uasset.ParseOptions) ([]byte, *uasset.Asset, error) {
	if asset == nil || oldAsset == nil || len(indexRemap) == 0 {
		if asset == nil {
			return nil, nil, nil
		}
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	if len(oldAsset.Exports) != len(asset.Exports) {
		return nil, nil, fmt.Errorf("export count mismatch: old=%d new=%d", len(oldAsset.Exports), len(asset.Exports))
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	nameCount := int32(len(oldAsset.Names))
	out := append([]byte(nil), asset.Raw.Bytes...)
	changed := false

	for i, oldExp := range oldAsset.Exports {
		if i >= len(asset.Exports) {
			return nil, nil, fmt.Errorf("export[%d] missing from rewritten asset", i+1)
		}
		newExp := asset.Exports[i]
		className := oldAsset.ResolveClassName(oldExp)
		propStart, propEnd, withCC := exportPropertyBounds(oldAsset, oldExp)
		parsed := oldAsset.ParseTaggedPropertiesRange(propStart, propEnd, withCC)
		if len(parsed.Warnings) > 0 {
			continue
		}

		oldStart := int(oldExp.SerialOffset)
		tailStart := parsed.EndOffset - oldStart
		oldTailEnd := int(oldExp.SerialSize)
		newTailEnd := int(newExp.SerialSize)
		if tailStart < 0 || tailStart >= oldTailEnd || tailStart >= newTailEnd {
			continue
		}

		oldAbsStart := int(oldExp.SerialOffset) + tailStart
		newAbsStart := int(newExp.SerialOffset) + tailStart
		oldAbsEnd := int(oldExp.SerialOffset) + oldTailEnd
		newAbsEnd := int(newExp.SerialOffset) + newTailEnd
		if oldAbsStart < 0 || oldAbsEnd > len(oldAsset.Raw.Bytes) || oldAbsStart > oldAbsEnd {
			return nil, nil, fmt.Errorf("export[%d] old tail range out of bounds", i+1)
		}
		if newAbsStart < 0 || newAbsEnd > len(out) || newAbsStart > newAbsEnd {
			return nil, nil, fmt.Errorf("export[%d] new tail range out of bounds", i+1)
		}
		limit := oldAbsEnd - oldAbsStart
		if next := newAbsEnd - newAbsStart; next < limit {
			limit = next
		}

		for rel := 0; rel+8 <= limit; rel++ {
			oldPos := oldAbsStart + rel
			newPos := newAbsStart + rel
			idx := int32(order.Uint32(oldAsset.Raw.Bytes[oldPos : oldPos+4]))
			if idx < 0 || idx >= nameCount {
				continue
			}
			if (strings.EqualFold(className, "K2Node_Event") || strings.EqualFold(className, "WidgetBlueprintGeneratedClass")) &&
				idx > 0 && idx <= int32(len(oldAsset.Exports)) {
				continue
			}
			num := int32(order.Uint32(oldAsset.Raw.Bytes[oldPos+4 : oldPos+8]))
			if num <= 0 || num > 10000 {
				continue
			}
			newIdx, ok := indexRemap[idx]
			if !ok || newIdx == idx {
				continue
			}
			order.PutUint32(out[newPos:newPos+4], uint32(newIdx))
			changed = true
			rel += 7
		}
	}

	if !changed {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	newAsset, err := uasset.ParseBytes(out, opts)
	if err != nil {
		return nil, nil, err
	}
	return out, newAsset, nil
}

func remapWidgetBlueprintGeneratedClassFieldNameRefs(raw []byte, indexRemap map[int32]int32, order binary.ByteOrder) (bool, error) {
	offsets, err := collectWidgetBlueprintGeneratedClassFieldNameRefOffsets(raw, order)
	if err != nil {
		return false, err
	}
	changed := false
	for _, off := range offsets {
		if off < 0 || off+8 > len(raw) {
			return false, fmt.Errorf("generated class field name ref out of bounds: %d", off)
		}
		if patchNameRefIndexInPlace(raw[off:off+8], indexRemap, order) {
			changed = true
		}
	}
	return changed, nil
}

func patchNameRefIndexFromOldAtOffset(oldRaw, newRaw []byte, off int, indexRemap map[int32]int32, order binary.ByteOrder) bool {
	if off < 0 || off+8 > len(oldRaw) || off+8 > len(newRaw) {
		return false
	}
	idx := int32(order.Uint32(oldRaw[off : off+4]))
	if idx < 0 {
		return false
	}
	newIdx, ok := indexRemap[idx]
	if !ok || newIdx == idx {
		return false
	}
	order.PutUint32(newRaw[off:off+4], uint32(newIdx))
	return true
}

func remapOpaqueNameRefPairsFromOldSkipBlocked(oldRaw, newRaw []byte, indexRemap map[int32]int32, blocked map[int]struct{}, order binary.ByteOrder) bool {
	if len(oldRaw) < 8 || len(newRaw) < 8 || len(indexRemap) == 0 {
		return false
	}
	limit := len(oldRaw)
	if len(newRaw) < limit {
		limit = len(newRaw)
	}
	changed := false
	for off := 0; off+8 <= limit; off++ {
		if _, ok := blocked[off]; ok {
			continue
		}
		idx := int32(order.Uint32(oldRaw[off : off+4]))
		if idx < 0 {
			continue
		}
		num := int32(order.Uint32(oldRaw[off+4 : off+8]))
		if num != 0 {
			continue
		}
		newIdx, ok := indexRemap[idx]
		if !ok || newIdx == idx {
			continue
		}
		order.PutUint32(newRaw[off:off+4], uint32(newIdx))
		changed = true
		off += 7
	}
	return changed
}

func collectWidgetBlueprintGeneratedClassFieldNameRefOffsets(raw []byte, order binary.ByteOrder) ([]int, error) {
	if len(raw) < 16 {
		return nil, nil
	}

	fieldCount := int(order.Uint32(raw[12:16]))
	if fieldCount < 0 {
		return nil, fmt.Errorf("invalid generated class field count: %d", fieldCount)
	}
	if fieldCount > len(raw)/24 || fieldCount > 1024 {
		return nil, fmt.Errorf("generated class field count out of bounds: %d", fieldCount)
	}

	offsets := make([]int, 0, fieldCount*4)
	offset := 16
	for i := 0; i < fieldCount; i++ {
		if len(raw[offset:]) < 28 {
			return nil, fmt.Errorf("generated class field %d header out of bounds", i)
		}

		offsets = append(offsets, offset, offset+8)
		offset += 24

		metaCount := int(order.Uint32(raw[offset : offset+4]))
		if metaCount < 0 {
			return nil, fmt.Errorf("generated class field %d metadata count is invalid: %d", i, metaCount)
		}
		offset += 4
		for j := 0; j < metaCount; j++ {
			if len(raw[offset:]) < 8 {
				return nil, fmt.Errorf("generated class field %d metadata %d key out of bounds", i, j)
			}
			offsets = append(offsets, offset)
			offset += 8

			_, consumed, err := generatedClassTailFStringLength(raw[offset:], order)
			if err != nil {
				return nil, fmt.Errorf("generated class field %d metadata %d string: %w", i, j, err)
			}
			offset += consumed
		}

		if len(raw[offset:]) < 4+4+8+2+8+1+4 {
			return nil, fmt.Errorf("generated class field %d suffix out of bounds", i)
		}
		offset += 4 + 4 + 8 + 2
		offsets = append(offsets, offset)
		offset += 8 + 1 + 4
	}

	return offsets, nil
}

func patchNameRefIndexInPlace(raw []byte, indexRemap map[int32]int32, order binary.ByteOrder) bool {
	if len(raw) < 8 {
		return false
	}
	idx := int32(order.Uint32(raw[:4]))
	if idx < 0 {
		return false
	}
	newIdx, ok := indexRemap[idx]
	if !ok || newIdx == idx {
		return false
	}
	order.PutUint32(raw[:4], uint32(newIdx))
	return true
}

func generatedClassTailFStringLength(raw []byte, order binary.ByteOrder) (int32, int, error) {
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
