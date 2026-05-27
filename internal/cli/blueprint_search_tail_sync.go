package cli

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/wilddogjp/openbpx/pkg/edit"
	"github.com/wilddogjp/openbpx/pkg/uasset"
)

var blueprintSearchTailIsDesignTimeBooleanNeedle = []byte("Is Design Time\nBoolean\x00")

type blueprintSearchTailVerboseRecord struct {
	length    int
	field1Pos int
	field2Pos int
	field1    int32
	field2    int32
	token1    string
	token2    string
	token3    string
}

func syncBlueprintEditorSearchTailOffsets(asset *uasset.Asset, opts uasset.ParseOptions) ([]byte, *uasset.Asset, error) {
	return syncBlueprintEditorSearchTailOffsetsWithNameRemap(asset, opts, nil)
}

func patchBlueprintSearchTailIsDesignTimeBooleanVariantFromOriginal(originalAsset, asset *uasset.Asset, opts uasset.ParseOptions) ([]byte, *uasset.Asset, error) {
	if originalAsset == nil || asset == nil {
		if asset == nil {
			return nil, nil, nil
		}
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	oldStart := int(originalAsset.Summary.PreloadDependencyOffset)
	oldEnd := int(originalAsset.Summary.BulkDataStartOffset)
	newStart := int(asset.Summary.PreloadDependencyOffset)
	newEnd := int(asset.Summary.BulkDataStartOffset)
	if oldStart <= 0 || oldEnd <= oldStart || oldEnd > len(originalAsset.Raw.Bytes) {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	if newStart <= 0 || newEnd <= newStart || newEnd > len(asset.Raw.Bytes) {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	oldTail := originalAsset.Raw.Bytes[oldStart:oldEnd]
	out := append([]byte(nil), asset.Raw.Bytes...)
	newTail := out[newStart:newEnd]
	changed := false
	searchFrom := 0
	for {
		oldPos := bytes.Index(oldTail[searchFrom:], blueprintSearchTailIsDesignTimeBooleanNeedle)
		if oldPos < 0 {
			break
		}
		oldPos += searchFrom
		newPos := bytes.Index(newTail[searchFrom:], blueprintSearchTailIsDesignTimeBooleanNeedle)
		if newPos < 0 {
			break
		}
		newPos += searchFrom

		oldVariant, oldMarker, ok := readBlueprintSearchTailIsDesignTimeBooleanVariant(oldTail, oldPos)
		if !ok {
			searchFrom = oldPos + len(blueprintSearchTailIsDesignTimeBooleanNeedle)
			continue
		}
		newVariant, newMarker, ok := readBlueprintSearchTailIsDesignTimeBooleanVariant(newTail, newPos)
		if !ok {
			searchFrom = oldPos + len(blueprintSearchTailIsDesignTimeBooleanNeedle)
			continue
		}
		if newVariant == oldVariant && newMarker == oldMarker+2 && oldVariant < 0xff {
			newTail[newPos+len(blueprintSearchTailIsDesignTimeBooleanNeedle)+1] = oldVariant + 1
			changed = true
		}
		searchFrom = oldPos + len(blueprintSearchTailIsDesignTimeBooleanNeedle)
	}

	if !changed {
		return out, asset, nil
	}
	if err := edit.FinalizePackageBytes(out, asset.Summary.FileVersionUE5); err != nil {
		return nil, nil, fmt.Errorf("finalize IsDesignTime boolean variant patch: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(out, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("reparse IsDesignTime boolean variant patch: %w", err)
	}
	return out, updatedAsset, nil
}

func patchBlueprintSearchTailIsDesignTimeBooleanVariantForEditableTextBoxProperty(originalAsset, asset *uasset.Asset, opts uasset.ParseOptions, normalizedProperty string) ([]byte, *uasset.Asset, error) {
	if originalAsset == nil || asset == nil {
		if asset == nil {
			return nil, nil, nil
		}
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	property := strings.ToLower(strings.TrimSpace(normalizedProperty))
	if property != "editabletextbox-justification" &&
		property != "editabletextboxjustification" &&
		property != "editabletextbox-minimum-desired-width" &&
		property != "editabletextboxminimumdesiredwidth" {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	oldStart := int(originalAsset.Summary.PreloadDependencyOffset)
	oldEnd := int(originalAsset.Summary.BulkDataStartOffset)
	newStart := int(asset.Summary.PreloadDependencyOffset)
	newEnd := int(asset.Summary.BulkDataStartOffset)
	if oldStart <= 0 || oldEnd <= oldStart || oldEnd > len(originalAsset.Raw.Bytes) {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	if newStart <= 0 || newEnd <= newStart || newEnd > len(asset.Raw.Bytes) {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	oldTail := originalAsset.Raw.Bytes[oldStart:oldEnd]
	out := append([]byte(nil), asset.Raw.Bytes...)
	newTail := out[newStart:newEnd]
	changed := false
	searchFrom := 0
	for {
		oldPos := bytes.Index(oldTail[searchFrom:], blueprintSearchTailIsDesignTimeBooleanNeedle)
		if oldPos < 0 {
			break
		}
		oldPos += searchFrom
		newPos := bytes.Index(newTail[searchFrom:], blueprintSearchTailIsDesignTimeBooleanNeedle)
		if newPos < 0 {
			break
		}
		newPos += searchFrom

		oldVariant, oldMarker, ok := readBlueprintSearchTailIsDesignTimeBooleanVariant(oldTail, oldPos)
		if !ok {
			searchFrom = oldPos + len(blueprintSearchTailIsDesignTimeBooleanNeedle)
			continue
		}
		newVariant, newMarker, ok := readBlueprintSearchTailIsDesignTimeBooleanVariant(newTail, newPos)
		if !ok {
			searchFrom = oldPos + len(blueprintSearchTailIsDesignTimeBooleanNeedle)
			continue
		}

		variantPos := newPos + len(blueprintSearchTailIsDesignTimeBooleanNeedle) + 1
		switch property {
		case "editabletextbox-justification", "editabletextboxjustification":
			if newMarker == oldMarker+4 && newVariant == oldVariant && oldVariant < 0xff {
				newTail[variantPos] = oldVariant + 1
				changed = true
			}
		case "editabletextbox-minimum-desired-width", "editabletextboxminimumdesiredwidth":
			if newMarker == oldMarker+2 && newVariant == oldVariant+1 {
				newTail[variantPos] = oldVariant
				changed = true
			}
		}
		searchFrom = oldPos + len(blueprintSearchTailIsDesignTimeBooleanNeedle)
	}

	if !changed {
		return out, asset, nil
	}
	if err := edit.FinalizePackageBytes(out, asset.Summary.FileVersionUE5); err != nil {
		return nil, nil, fmt.Errorf("finalize EditableTextBox IsDesignTime boolean variant patch: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(out, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("reparse EditableTextBox IsDesignTime boolean variant patch: %w", err)
	}
	return out, updatedAsset, nil
}

func readBlueprintSearchTailIsDesignTimeBooleanVariant(tail []byte, markerPos int) (variant byte, marker byte, ok bool) {
	base := markerPos + len(blueprintSearchTailIsDesignTimeBooleanNeedle)
	if base+10 > len(tail) {
		return 0, 0, false
	}
	if tail[base] != 0x01 {
		return 0, 0, false
	}
	for _, b := range tail[base+2 : base+9] {
		if b != 0x00 {
			return 0, 0, false
		}
	}
	if tail[base+10] != 0x00 {
		return 0, 0, false
	}
	return tail[base+1], tail[base+9], true
}

func rewriteBlueprintEditorSearchTailVerboseRecordFieldsFromOriginal(originalAsset, asset *uasset.Asset, opts uasset.ParseOptions, indexRemap map[int32]int32) ([]byte, *uasset.Asset, error) {
	if originalAsset == nil || asset == nil || len(indexRemap) == 0 {
		if asset == nil {
			return nil, nil, nil
		}
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	oldStart := int(originalAsset.Summary.PreloadDependencyOffset)
	oldEnd := int(originalAsset.Summary.BulkDataStartOffset)
	newStart := int(asset.Summary.PreloadDependencyOffset)
	newEnd := int(asset.Summary.BulkDataStartOffset)
	if oldStart <= 0 || oldEnd <= oldStart || oldEnd > len(originalAsset.Raw.Bytes) {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	if newStart <= 0 || newEnd <= newStart || newEnd > len(asset.Raw.Bytes) {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	oldTail := originalAsset.Raw.Bytes[oldStart:oldEnd]
	out := append([]byte(nil), asset.Raw.Bytes...)
	newTail := out[newStart:newEnd]
	limit := len(oldTail)
	if len(newTail) < limit {
		limit = len(newTail)
	}
	if limit <= 0 {
		return out, asset, nil
	}

	order := packageByteOrder(asset)
	oldKnownNames := blueprintSearchTailKnownObjectNames(originalAsset)
	newKnownNames := blueprintSearchTailKnownObjectNames(asset)
	changed := false
	for rel := 0; rel < limit; rel++ {
		oldRecord, ok := parseBlueprintSearchTailVerboseRecord(oldTail[rel:limit], originalAsset.Summary.UsesByteSwappedSerialization())
		if !ok {
			continue
		}
		newRecord, ok := parseBlueprintSearchTailVerboseRecord(newTail[rel:], asset.Summary.UsesByteSwappedSerialization())
		if !ok || newRecord.length != oldRecord.length || newRecord.field1Pos != oldRecord.field1Pos || newRecord.field2Pos != oldRecord.field2Pos {
			continue
		}
		if !isLikelyWidgetBlueprintSearchTailVerboseRecord(oldRecord, oldKnownNames) || !isLikelyWidgetBlueprintSearchTailVerboseRecord(newRecord, newKnownNames) {
			continue
		}
		if oldRecord.token1 != newRecord.token1 || oldRecord.token2 != newRecord.token2 || oldRecord.token3 != newRecord.token3 {
			continue
		}

		if remapped, ok := indexRemap[oldRecord.field1]; ok && remapped > 0 && newRecord.field1 != remapped {
			order.PutUint32(newTail[rel+newRecord.field1Pos:rel+newRecord.field1Pos+4], uint32(remapped))
			changed = true
		}
		if remapped, ok := indexRemap[oldRecord.field2]; ok && remapped > 0 && newRecord.field2 != remapped {
			order.PutUint32(newTail[rel+newRecord.field2Pos:rel+newRecord.field2Pos+4], uint32(remapped))
			changed = true
		}
		rel += oldRecord.length - 1
	}

	if !changed {
		return out, asset, nil
	}
	if err := edit.FinalizePackageBytes(out, asset.Summary.FileVersionUE5); err != nil {
		return nil, nil, fmt.Errorf("finalize blueprint search tail verbose field rewrite: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(out, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("reparse blueprint search tail verbose field rewrite: %w", err)
	}
	return out, updatedAsset, nil
}

func syncBlueprintEditorSearchTailOffsetsWithNameRemap(asset *uasset.Asset, opts uasset.ParseOptions, indexRemap map[int32]int32) ([]byte, *uasset.Asset, error) {
	if asset == nil || len(asset.Raw.Bytes) == 0 {
		return nil, asset, nil
	}

	start := int(asset.Summary.PreloadDependencyOffset)
	end := int(asset.Summary.BulkDataStartOffset)
	if start <= 0 || end <= start || end > len(asset.Raw.Bytes) {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	order := packageByteOrder(asset)
	knownNames := blueprintSearchTailKnownObjectNames(asset)
	out := append([]byte(nil), asset.Raw.Bytes...)
	changed := false
	for pos := start + 0x24; pos < end; pos++ {
		record, ok := parseBlueprintSearchTailVerboseRecord(out[pos:end], asset.Summary.UsesByteSwappedSerialization())
		if !ok {
			continue
		}
		if !isLikelyWidgetBlueprintSearchTailVerboseRecord(record, knownNames) {
			continue
		}

		source1Pos := pos - 0x24
		source2Pos := pos - 0x0c
		if source1Pos < start || source2Pos < start || source2Pos+4 > end {
			continue
		}
		source1 := int32(order.Uint32(out[source1Pos : source1Pos+4]))
		source2 := int32(order.Uint32(out[source2Pos : source2Pos+4]))
		if source1 <= 0 || source2 <= 0 || source1 > 1<<20 || source2 > 1<<20 {
			continue
		}
		if remapped, ok := indexRemap[source1]; ok && remapped > 0 {
			source1 = remapped
			order.PutUint32(out[source1Pos:source1Pos+4], uint32(source1))
			changed = true
		}
		if remapped, ok := indexRemap[source2]; ok && remapped > 0 {
			source2 = remapped
			order.PutUint32(out[source2Pos:source2Pos+4], uint32(source2))
			changed = true
		}
		if record.field1 != source1 {
			order.PutUint32(out[pos+record.field1Pos:pos+record.field1Pos+4], uint32(source1))
			changed = true
		}
		if record.field2 != source2 {
			order.PutUint32(out[pos+record.field2Pos:pos+record.field2Pos+4], uint32(source2))
			changed = true
		}
		pos += record.length - 1
	}
	if patchBlueprintSearchTailCompactNameVariants(asset, out[start:end]) {
		changed = true
	}

	if !changed {
		return out, asset, nil
	}
	if err := edit.FinalizePackageBytes(out, asset.Summary.FileVersionUE5); err != nil {
		return nil, nil, fmt.Errorf("finalize blueprint search tail sync: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(out, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("reparse blueprint search tail sync: %w", err)
	}
	return out, updatedAsset, nil
}

func parseBlueprintSearchTailVerboseRecord(data []byte, byteSwap bool) (blueprintSearchTailVerboseRecord, bool) {
	reader := uasset.NewByteReaderWithByteSwapping(data, byteSwap)

	neg, err := reader.ReadInt32()
	if err != nil || neg >= 0 || neg < -64 {
		return blueprintSearchTailVerboseRecord{}, false
	}
	if v, err := reader.ReadInt32(); err != nil || v != 0 {
		return blueprintSearchTailVerboseRecord{}, false
	}
	if v, err := reader.ReadInt32(); err != nil || v != 1 {
		return blueprintSearchTailVerboseRecord{}, false
	}
	if v, err := reader.ReadInt32(); err != nil || v <= 0 || v > 1024 {
		return blueprintSearchTailVerboseRecord{}, false
	}
	if v, err := reader.ReadInt32(); err != nil || v != 0 {
		return blueprintSearchTailVerboseRecord{}, false
	}

	field1Pos := reader.Offset()
	field1, err := reader.ReadInt32()
	if err != nil || field1 <= 0 || field1 > 1<<20 {
		return blueprintSearchTailVerboseRecord{}, false
	}
	if v, err := reader.ReadInt32(); err != nil || v <= 0 || v > 1024 {
		return blueprintSearchTailVerboseRecord{}, false
	}
	// Editor search tail records carry a packed metadata marker here. We only
	// need to preserve structure, so accept the observed packed form instead of
	// assuming the value is a small integer.
	if v, err := reader.ReadInt32(); err != nil || (v != 1 && v != 0x00200001) {
		return blueprintSearchTailVerboseRecord{}, false
	}
	if v, err := reader.ReadInt32(); err != nil || v <= 0 || v > 16 {
		return blueprintSearchTailVerboseRecord{}, false
	}
	if v, err := reader.ReadInt32(); err != nil || v <= 0 || v > 16 {
		return blueprintSearchTailVerboseRecord{}, false
	}
	if v, err := reader.ReadInt32(); err != nil || v <= 0 || v > 1024 {
		return blueprintSearchTailVerboseRecord{}, false
	}
	if v, err := reader.ReadInt32(); err != nil || v != 0 {
		return blueprintSearchTailVerboseRecord{}, false
	}

	s1, err := reader.ReadFString()
	if err != nil || !isBlueprintSearchTailStringToken(s1) {
		return blueprintSearchTailVerboseRecord{}, false
	}
	if v, err := reader.ReadInt32(); err != nil || v <= 0 || v > 1024 {
		return blueprintSearchTailVerboseRecord{}, false
	}
	if v, err := reader.ReadInt32(); err != nil || v != 0 {
		return blueprintSearchTailVerboseRecord{}, false
	}
	s2, err := reader.ReadFString()
	if err != nil || !isBlueprintSearchTailStringToken(s2) {
		return blueprintSearchTailVerboseRecord{}, false
	}
	if v, err := reader.ReadInt32(); err != nil || v <= 0 || v > 1024 {
		return blueprintSearchTailVerboseRecord{}, false
	}
	if v, err := reader.ReadInt32(); err != nil || v != 0 {
		return blueprintSearchTailVerboseRecord{}, false
	}
	s3, err := reader.ReadFString()
	if err != nil || !isBlueprintSearchTailStringToken(s3) {
		return blueprintSearchTailVerboseRecord{}, false
	}
	if v, err := reader.ReadInt32(); err != nil || v <= 0 || v > 16 {
		return blueprintSearchTailVerboseRecord{}, false
	}
	blobLen, err := reader.ReadInt32()
	if err != nil || blobLen <= 0 || blobLen > 64 {
		return blueprintSearchTailVerboseRecord{}, false
	}
	if _, err := reader.ReadBytes(int(blobLen)); err != nil {
		return blueprintSearchTailVerboseRecord{}, false
	}
	if v, err := reader.ReadUint16(); err != nil || v != 0 {
		return blueprintSearchTailVerboseRecord{}, false
	}

	field2Pos := reader.Offset()
	field2, err := reader.ReadInt32()
	if err != nil || field2 <= 0 || field2 > 1<<20 {
		return blueprintSearchTailVerboseRecord{}, false
	}

	return blueprintSearchTailVerboseRecord{
		length:    reader.Offset(),
		field1Pos: field1Pos,
		field2Pos: field2Pos,
		field1:    field1,
		field2:    field2,
		token1:    s1,
		token2:    s2,
		token3:    s3,
	}, true
}

func isBlueprintSearchTailStringToken(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" || len(v) > 256 {
		return false
	}
	for _, r := range v {
		if r < 0x20 || r > 0x7e {
			return false
		}
	}
	return true
}

func patchBlueprintSearchTailCompactNameVariants(asset *uasset.Asset, tail []byte) bool {
	if asset == nil || len(tail) == 0 {
		return false
	}
	changed := false
	for _, pattern := range []struct {
		name     string
		prefix   []byte
		suffix   []byte
		slotByte int
	}{
		{
			name: "object",
			prefix: []byte{
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x6f,
				0x00, 0x00, 0x00, 0x00,
			},
			suffix: []byte{
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x04,
				0x00, 0x00, 0x00, 0x00,
				0x0b, 0x00, 0x00, 0x00,
				0x54, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
			},
			slotByte: 3,
		},
		{
			name: "GraphGuid",
			prefix: []byte{
				0x00, 0x12, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x6d, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x01, 0x00, 0x00,
			},
			suffix: []byte{
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x01, 0x00, 0x00,
				0x00, 0x01, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x10, 0x00, 0x00,
				0x00, 0x08, 0x34, 0x99,
			},
			slotByte: 1,
		},
	} {
		idx := findNameIndex(asset.Names, pattern.name)
		if idx < 0 || idx >= 0xff {
			continue
		}
		for pos := 0; pos+len(pattern.prefix)+4+len(pattern.suffix) <= len(tail); pos++ {
			if !bytes.Equal(tail[pos:pos+len(pattern.prefix)], pattern.prefix) {
				continue
			}
			slotStart := pos + len(pattern.prefix)
			slotEnd := slotStart + 4
			if !bytes.Equal(tail[slotEnd:slotEnd+len(pattern.suffix)], pattern.suffix) {
				continue
			}
			slot := tail[slotStart:slotEnd]
			nonZero := 0
			for _, b := range slot {
				if b != 0 {
					nonZero++
				}
			}
			if nonZero > 1 {
				continue
			}
			want := [4]byte{}
			want[pattern.slotByte] = byte(idx + 1)
			if bytes.Equal(slot, want[:]) {
				continue
			}
			copy(slot, want[:])
			changed = true
		}
	}
	for _, pattern := range []struct {
		prefix []blueprintSearchTailCompactSlotSpec
		middle []blueprintSearchTailCompactSlotSpec
		suffix []blueprintSearchTailCompactSlotSpec
	}{
		{
			prefix: []blueprintSearchTailCompactSlotSpec{
				{name: "WidgetTree", slotByte: 0},
				{zero: true},
				{name: "ObjectProperty", slotByte: 0},
				{zero: true},
				{zero: true},
				{name: "AllWidgets", slotByte: 0},
				{name: "bLegacyNeedToPurgeSkelRefs", slotByte: 1},
				{name: "PropertyGuids", slotByte: 1},
				{zero: true},
				{name: "MapProperty", slotByte: 1},
			},
			middle: []blueprintSearchTailCompactSlotSpec{
				{zero: true},
				{name: "/Script/Engine", slotByte: 1},
			},
			suffix: []blueprintSearchTailCompactSlotSpec{
				{name: "NameProperty", slotByte: 1},
				{zero: true},
				{zero: true},
				{name: "StructProperty", slotByte: 1},
			},
		},
		{
			prefix: []blueprintSearchTailCompactSlotSpec{
				{zero: true},
				{name: "WidgetTree", slotByte: 3},
				{zero: true},
				{name: "ObjectProperty", slotByte: 3},
			},
			middle: []blueprintSearchTailCompactSlotSpec{
				{zero: true},
			},
			suffix: []blueprintSearchTailCompactSlotSpec{
				{zero: true},
				{name: "AllWidgets", slotByte: 3},
				{zero: true},
				{name: "bLegacyNeedToPurgeSkelRefs", slotByte: 0},
				{name: "PropertyGuids", slotByte: 0},
				{zero: true},
				{name: "MapProperty", slotByte: 0},
				{zero: true},
				{name: "/Script/Engine", slotByte: 0},
				{name: "NameProperty", slotByte: 0},
				{zero: true},
				{zero: true},
				{name: "StructProperty", slotByte: 0},
			},
		},
	} {
		if patchBlueprintSearchTailCompactSlotSequence(asset, tail, pattern.prefix, pattern.middle, pattern.suffix) {
			changed = true
		}
	}
	return changed
}

type blueprintSearchTailCompactSlotSpec struct {
	name     string
	slotByte int
	plusOne  bool
	zero     bool
}

func patchBlueprintSearchTailCompactSlotSequence(asset *uasset.Asset, tail []byte, prefixSpecs, middleSpecs, suffixSpecs []blueprintSearchTailCompactSlotSpec) bool {
	if asset == nil || len(tail) == 0 {
		return false
	}
	prefix, ok := buildBlueprintSearchTailCompactSlotSequence(asset, prefixSpecs)
	if !ok {
		return false
	}
	wantMiddle, ok := buildBlueprintSearchTailCompactSlotSequence(asset, middleSpecs)
	if !ok {
		return false
	}
	suffix, ok := buildBlueprintSearchTailCompactSlotSequence(asset, suffixSpecs)
	if !ok {
		return false
	}
	changed := false
	for pos := 0; pos+len(prefix)+len(wantMiddle)+len(suffix) <= len(tail); pos++ {
		if !bytes.Equal(tail[pos:pos+len(prefix)], prefix) {
			continue
		}
		middleStart := pos + len(prefix)
		middleEnd := middleStart + len(wantMiddle)
		if !bytes.Equal(tail[middleEnd:middleEnd+len(suffix)], suffix) {
			continue
		}
		middle := tail[middleStart:middleEnd]
		if !isBlueprintSearchTailCompactSequenceShapeCompatible(middle, len(wantMiddle)/4) {
			continue
		}
		if bytes.Equal(middle, wantMiddle) {
			continue
		}
		copy(middle, wantMiddle)
		changed = true
	}
	return changed
}

func isBlueprintSearchTailCompactSequenceShapeCompatible(raw []byte, slotCount int) bool {
	if slotCount <= 0 || len(raw) != slotCount*4 {
		return false
	}
	for i := 0; i < slotCount; i++ {
		slot := raw[i*4 : i*4+4]
		nonZero := 0
		for _, b := range slot {
			if b != 0 {
				nonZero++
			}
		}
		if nonZero > 1 {
			return false
		}
	}
	return true
}

func buildBlueprintSearchTailCompactSlotSequence(asset *uasset.Asset, specs []blueprintSearchTailCompactSlotSpec) ([]byte, bool) {
	out := make([]byte, 0, len(specs)*4)
	for _, spec := range specs {
		slot, ok := encodeBlueprintSearchTailCompactSlot(asset, spec)
		if !ok {
			return nil, false
		}
		out = append(out, slot...)
	}
	return out, true
}

func encodeBlueprintSearchTailCompactSlot(asset *uasset.Asset, spec blueprintSearchTailCompactSlotSpec) ([]byte, bool) {
	slot := make([]byte, 4)
	if spec.zero {
		return slot, true
	}
	if asset == nil || spec.slotByte < 0 || spec.slotByte >= len(slot) {
		return nil, false
	}
	idx := findNameIndex(asset.Names, spec.name)
	if idx < 0 || idx > 0xff {
		return nil, false
	}
	if spec.plusOne {
		idx++
		if idx > 0xff {
			return nil, false
		}
	}
	slot[spec.slotByte] = byte(idx)
	return slot, true
}

func blueprintSearchTailKnownObjectNames(asset *uasset.Asset) map[string]struct{} {
	if asset == nil {
		return nil
	}
	out := make(map[string]struct{}, len(asset.Exports))
	for _, exp := range asset.Exports {
		name := strings.TrimSpace(exp.ObjectName.Display(asset.Names))
		if name == "" {
			continue
		}
		out[name] = struct{}{}
	}
	return out
}

func isLikelyWidgetBlueprintSearchTailVerboseRecord(record blueprintSearchTailVerboseRecord, knownNames map[string]struct{}) bool {
	token1 := strings.TrimSpace(record.token1)
	if token1 != "true" && token1 != "false" {
		return false
	}
	token2 := strings.TrimSpace(record.token2)
	if token2 == "" || !strings.Contains(token2, "_") {
		return false
	}
	token3 := strings.TrimSpace(record.token3)
	if token3 == "" || !strings.ContainsAny(token3, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz") {
		return false
	}
	if len(knownNames) == 0 {
		return true
	}
	if _, ok := knownNames[token2]; !ok {
		return false
	}
	if _, ok := knownNames[token3]; !ok {
		return false
	}
	return true
}
