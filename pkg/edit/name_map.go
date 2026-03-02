package edit

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"unicode"
	"unicode/utf16"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

// RewriteNameMap replaces the serialized NameMap with the provided entries and
// updates dependent offsets/count fields.
func RewriteNameMap(asset *uasset.Asset, names []uasset.NameEntry) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if len(asset.Raw.Bytes) == 0 {
		return nil, fmt.Errorf("asset has no raw bytes")
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("name map must not be empty")
	}

	raw := asset.Raw.Bytes
	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	nameStart := int64(asset.Summary.NameOffset)
	if nameStart < 0 || nameStart > int64(len(raw)) {
		return nil, fmt.Errorf("name offset out of range: %d", asset.Summary.NameOffset)
	}
	nameEnd, err := findNameMapEndOffset(asset, raw)
	if err != nil {
		return nil, err
	}
	if nameEnd < nameStart || nameEnd > int64(len(raw)) {
		return nil, fmt.Errorf("invalid name map range: %d..%d", nameStart, nameEnd)
	}

	encodedNameMap := encodeNameMap(names, order)
	delta := int64(len(encodedNameMap)) - (nameEnd - nameStart)

	out := make([]byte, 0, int64(len(raw))+delta)
	out = append(out, raw[:nameStart]...)
	newStart := int64(len(out))
	out = append(out, encodedNameMap...)
	out = append(out, raw[nameEnd:]...)

	patches := []nameMapRangePatch{{
		oldStart: nameStart,
		oldEnd:   nameEnd,
		oldLen:   nameEnd - nameStart,
		newLen:   int64(len(encodedNameMap)),
		newStart: newStart,
	}}

	summaryOffsets, err := scanSummaryOffsetFields(raw)
	if err != nil {
		return nil, fmt.Errorf("scan summary offsets: %w", err)
	}
	for _, field := range summaryOffsets {
		switch field.size {
		case 4:
			oldV, err := readInt32At(raw, field.pos, order)
			if err != nil {
				return nil, fmt.Errorf("read summary %s: %w", field.name, err)
			}
			if oldV < 0 {
				continue
			}
			mapped := translateNameMapOffset(int64(oldV), patches)
			if mapped > int64(^uint32(0)>>1) {
				return nil, fmt.Errorf("summary %s overflow after translation: %d", field.name, mapped)
			}
			if err := writeInt32At(out, field.pos, int32(mapped), order); err != nil {
				return nil, fmt.Errorf("patch summary %s: %w", field.name, err)
			}
		case 8:
			oldV, err := readInt64At(raw, field.pos, order)
			if err != nil {
				return nil, fmt.Errorf("read summary %s: %w", field.name, err)
			}
			if oldV < 0 {
				continue
			}
			mapped := translateNameMapOffset(oldV, patches)
			if err := writeInt64At(out, field.pos, mapped, order); err != nil {
				return nil, fmt.Errorf("patch summary %s: %w", field.name, err)
			}
		default:
			return nil, fmt.Errorf("unsupported summary field size for %s: %d", field.name, field.size)
		}
	}

	countFields, err := scanSummaryNameCountFields(raw)
	if err != nil {
		return nil, fmt.Errorf("scan summary name counts: %w", err)
	}
	newCount := int32(len(names))
	if err := writeInt32At(out, countFields.NameCountPos, newCount, order); err != nil {
		return nil, fmt.Errorf("patch summary NameCount: %w", err)
	}
	if len(countFields.GenerationNameCountPos) > 0 {
		lastPos := countFields.GenerationNameCountPos[len(countFields.GenerationNameCountPos)-1]
		if err := writeInt32At(out, lastPos, newCount, order); err != nil {
			return nil, fmt.Errorf("patch summary Generations.Last().NameCount: %w", err)
		}
	}
	if countFields.NamesReferencedFromExportDataCountPos >= 0 {
		current, err := readInt32At(raw, countFields.NamesReferencedFromExportDataCountPos, order)
		if err != nil {
			return nil, fmt.Errorf("read summary NamesReferencedFromExportDataCount: %w", err)
		}
		if current > newCount {
			if err := writeInt32At(out, countFields.NamesReferencedFromExportDataCountPos, newCount, order); err != nil {
				return nil, fmt.Errorf("patch summary NamesReferencedFromExportDataCount: %w", err)
			}
		}
	}

	exportFields, err := scanExportFieldPositions(raw, asset)
	if err != nil {
		return nil, fmt.Errorf("scan export field positions: %w", err)
	}
	if len(exportFields) != len(asset.Exports) {
		return nil, fmt.Errorf("export field scan mismatch: got %d want %d", len(exportFields), len(asset.Exports))
	}
	for i, expField := range exportFields {
		oldSerialOffset := asset.Exports[i].SerialOffset
		if oldSerialOffset < 0 {
			continue
		}
		mappedSerialOffset := translateNameMapOffset(oldSerialOffset, patches)
		mappedFieldPos := translateNameMapOffset(int64(expField.serialOffsetPos), patches)
		if mappedFieldPos < 0 || mappedFieldPos > int64(len(out)-8) {
			return nil, fmt.Errorf("mapped export[%d] serial offset field out of range: %d", i+1, mappedFieldPos)
		}
		if err := writeInt64At(out, int(mappedFieldPos), mappedSerialOffset, order); err != nil {
			return nil, fmt.Errorf("patch export[%d] serial offset: %w", i+1, err)
		}
	}

	return out, nil
}

// ComputeNameEntryHashesUE56 computes FNameEntrySerialized hash fields following
// UE's FCrc::StrCrc32 / FCrc::Strihash_DEPRECATED behavior.
func ComputeNameEntryHashesUE56(value string) (nonCaseHash uint16, casePreservingHash uint16) {
	if isASCIIName(value) {
		bytes := []byte(value)
		caseHash := ueStrCrc32ASCII(bytes)
		nonCase := ueStriHashDeprecatedASCII(bytes)
		return uint16(nonCase & 0xFFFF), uint16(caseHash & 0xFFFF)
	}

	units := utf16.Encode([]rune(value))
	caseHash := ueStrCrc32UTF16(units)
	nonCase := ueStriHashDeprecatedUTF16(units)
	return uint16(nonCase & 0xFFFF), uint16(caseHash & 0xFFFF)
}

func encodeNameMap(names []uasset.NameEntry, order binary.ByteOrder) []byte {
	w := newByteWriter(order, len(names)*16)
	for _, entry := range names {
		w.writeFString(entry.Value)
		w.writeUint16(entry.NonCaseHash)
		w.writeUint16(entry.CasePreservingHash)
	}
	return w.bytes()
}

type nameMapRangePatch struct {
	oldStart int64
	oldEnd   int64
	oldLen   int64
	newLen   int64
	newStart int64
}

func translateNameMapOffset(oldPos int64, patches []nameMapRangePatch) int64 {
	delta := int64(0)
	for _, patch := range patches {
		if oldPos < patch.oldStart {
			break
		}
		if oldPos >= patch.oldEnd {
			delta += patch.newLen - patch.oldLen
			continue
		}
		rel := oldPos - patch.oldStart
		if rel < 0 {
			rel = 0
		}
		if rel > patch.newLen {
			rel = patch.newLen
		}
		return patch.newStart + rel
	}
	return oldPos + delta
}

func findNameMapEndOffset(asset *uasset.Asset, raw []byte) (int64, error) {
	if asset == nil {
		return 0, fmt.Errorf("asset is nil")
	}
	start := int64(asset.Summary.NameOffset)
	if start < 0 {
		return 0, fmt.Errorf("name offset is negative: %d", asset.Summary.NameOffset)
	}

	candidates := []int64{
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
		return 0, fmt.Errorf("could not determine NameMap end offset")
	}
	return end, nil
}

type summaryNameCountFields struct {
	NameCountPos                          int
	GenerationNameCountPos                []int
	NamesReferencedFromExportDataCountPos int
}

func scanSummaryNameCountFields(data []byte) (summaryNameCountFields, error) {
	fields := summaryNameCountFields{NamesReferencedFromExportDataCountPos: -1}
	if len(data) < 4 {
		return fields, fmt.Errorf("file too small")
	}
	tagLE := binary.LittleEndian.Uint32(data[:4])
	var order binary.ByteOrder = binary.LittleEndian
	switch tagLE {
	case packageFileTag:
		order = binary.LittleEndian
	case packageFileTagSwapped:
		order = binary.BigEndian
	default:
		return fields, fmt.Errorf("invalid package tag: 0x%x", tagLE)
	}

	r := newByteCodec(data, order)

	if _, err := r.readInt32(); err != nil { // tag
		return fields, err
	}
	legacy, err := r.readInt32()
	if err != nil {
		return fields, err
	}
	if legacy != -4 {
		if _, err := r.readInt32(); err != nil {
			return fields, err
		}
	}
	fileUE4, err := r.readInt32()
	if err != nil {
		return fields, err
	}
	fileUE5, err := r.readInt32()
	if err != nil {
		return fields, err
	}
	fileLicensee, err := r.readInt32()
	if err != nil {
		return fields, err
	}
	if fileUE4 == 0 && fileUE5 == 0 && fileLicensee == 0 {
		fileUE4 = ue4VersionUE56
		fileUE5 = ue5OSSubObjectShadowSerialization
	}
	if fileUE5 >= ue5PackageSavedHash {
		if err := r.skip(20); err != nil {
			return fields, err
		}
		if _, err := r.readInt32(); err != nil { // TotalHeaderSize
			return fields, err
		}
	}
	if legacy <= -2 {
		if err := skipSummaryCustomVersions(r, legacy); err != nil {
			return fields, err
		}
	}
	if fileUE5 < ue5PackageSavedHash {
		if _, err := r.readInt32(); err != nil { // TotalHeaderSize
			return fields, err
		}
	}
	if _, err := r.readFString(); err != nil { // PackageName
		return fields, err
	}
	packageFlags, err := r.readUint32()
	if err != nil {
		return fields, err
	}

	fields.NameCountPos = r.off
	if _, err := r.readInt32(); err != nil { // NameCount
		return fields, err
	}
	if _, err := r.readInt32(); err != nil { // NameOffset
		return fields, err
	}

	if fileUE5 >= ue5AddSoftObjectPathList {
		if _, err := r.readInt32(); err != nil { // SoftObjectPathsCount
			return fields, err
		}
		if _, err := r.readInt32(); err != nil { // SoftObjectPathsOffset
			return fields, err
		}
	}
	if packageFlags&pkgFlagFilterEditorOnly == 0 {
		if _, err := r.readFString(); err != nil { // LocalizationID
			return fields, err
		}
	}
	if _, err := r.readInt32(); err != nil { // GatherableTextDataCount
		return fields, err
	}
	if _, err := r.readInt32(); err != nil { // GatherableTextDataOffset
		return fields, err
	}
	if _, err := r.readInt32(); err != nil { // ExportCount
		return fields, err
	}
	if _, err := r.readInt32(); err != nil { // ExportOffset
		return fields, err
	}
	if _, err := r.readInt32(); err != nil { // ImportCount
		return fields, err
	}
	if _, err := r.readInt32(); err != nil { // ImportOffset
		return fields, err
	}
	if fileUE5 >= ue5VerseCells {
		if _, err := r.readInt32(); err != nil { // CellExportCount
			return fields, err
		}
		if _, err := r.readInt32(); err != nil { // CellExportOffset
			return fields, err
		}
		if _, err := r.readInt32(); err != nil { // CellImportCount
			return fields, err
		}
		if _, err := r.readInt32(); err != nil { // CellImportOffset
			return fields, err
		}
	}
	if fileUE5 >= ue5MetadataSerializationOff {
		if _, err := r.readInt32(); err != nil { // MetaDataOffset
			return fields, err
		}
	}
	if _, err := r.readInt32(); err != nil { // DependsOffset
		return fields, err
	}
	if _, err := r.readInt32(); err != nil { // SoftPackageReferencesCount
		return fields, err
	}
	if _, err := r.readInt32(); err != nil { // SoftPackageReferencesOffset
		return fields, err
	}
	if _, err := r.readInt32(); err != nil { // SearchableNamesOffset
		return fields, err
	}
	if _, err := r.readInt32(); err != nil { // ThumbnailTableOffset
		return fields, err
	}
	if fileUE5 < ue5PackageSavedHash {
		if err := r.skip(16); err != nil { // Guid
			return fields, err
		}
	}
	if packageFlags&pkgFlagFilterEditorOnly == 0 {
		if err := r.skip(16); err != nil { // PersistentGUID
			return fields, err
		}
	}
	generationCount, err := r.readInt32()
	if err != nil {
		return fields, err
	}
	if generationCount < 0 {
		return fields, fmt.Errorf("invalid generation count: %d", generationCount)
	}
	for i := int32(0); i < generationCount; i++ {
		if _, err := r.readInt32(); err != nil { // ExportCount
			return fields, err
		}
		namePos := r.off
		if _, err := r.readInt32(); err != nil { // NameCount
			return fields, err
		}
		fields.GenerationNameCountPos = append(fields.GenerationNameCountPos, namePos)
	}
	if err := skipEngineVersion(r); err != nil {
		return fields, err
	}
	if err := skipEngineVersion(r); err != nil {
		return fields, err
	}
	if _, err := r.readUint32(); err != nil { // CompressionFlags
		return fields, err
	}
	chunkCount, err := r.readInt32()
	if err != nil {
		return fields, err
	}
	if chunkCount < 0 {
		return fields, fmt.Errorf("invalid compressed chunk count: %d", chunkCount)
	}
	if err := r.skip(int(chunkCount) * 16); err != nil {
		return fields, err
	}
	if _, err := r.readUint32(); err != nil { // PackageSource
		return fields, err
	}
	additionalCount, err := r.readInt32()
	if err != nil {
		return fields, err
	}
	if additionalCount < 0 {
		return fields, fmt.Errorf("invalid additional packages count: %d", additionalCount)
	}
	for i := int32(0); i < additionalCount; i++ {
		if _, err := r.readFString(); err != nil {
			return fields, err
		}
	}
	if legacy > -7 {
		if _, err := r.readInt32(); err != nil { // TextureAllocations
			return fields, err
		}
	}
	if _, err := r.readInt32(); err != nil { // AssetRegistryDataOffset
		return fields, err
	}
	if _, err := r.readInt64(); err != nil { // BulkDataStartOffset
		return fields, err
	}
	if _, err := r.readInt32(); err != nil { // WorldTileInfoDataOffset
		return fields, err
	}
	chunkIDCount, err := r.readInt32()
	if err != nil {
		return fields, err
	}
	if chunkIDCount < 0 {
		return fields, fmt.Errorf("invalid chunk ID count: %d", chunkIDCount)
	}
	if err := r.skip(int(chunkIDCount) * 4); err != nil {
		return fields, err
	}
	if _, err := r.readInt32(); err != nil { // PreloadDependencyCount
		return fields, err
	}
	if _, err := r.readInt32(); err != nil { // PreloadDependencyOffset
		return fields, err
	}
	if fileUE5 >= ue5NamesFromExportData {
		fields.NamesReferencedFromExportDataCountPos = r.off
		if _, err := r.readInt32(); err != nil {
			return fields, err
		}
	}
	return fields, nil
}

func isASCIIName(v string) bool {
	for _, r := range v {
		if r > 0x7F {
			return false
		}
	}
	return true
}

func ueStrCrc32ASCII(v []byte) uint32 {
	crc := ^uint32(0)
	for _, ch := range v {
		u := uint32(ch)
		for i := 0; i < 4; i++ {
			crc = (crc >> 8) ^ crc32.IEEETable[(crc^u)&0xFF]
			u >>= 8
		}
	}
	return ^crc
}

func ueStrCrc32UTF16(v []uint16) uint32 {
	crc := ^uint32(0)
	for _, ch := range v {
		u := uint32(ch)
		for i := 0; i < 4; i++ {
			crc = (crc >> 8) ^ crc32.IEEETable[(crc^u)&0xFF]
			u >>= 8
		}
	}
	return ^crc
}

func ueStriHashDeprecatedASCII(v []byte) uint32 {
	table := deprecatedCRCTable()
	hash := uint32(0)
	for _, ch := range v {
		upper := byte(unicode.ToUpper(rune(ch)))
		hash = ((hash >> 8) & 0x00FFFFFF) ^ table[(hash^uint32(upper))&0x000000FF]
	}
	return hash
}

func ueStriHashDeprecatedUTF16(v []uint16) uint32 {
	table := deprecatedCRCTable()
	hash := uint32(0)
	for _, ch := range v {
		upper := uint16(unicode.ToUpper(rune(ch)))
		low := uint32(upper & 0x00FF)
		hash = ((hash >> 8) & 0x00FFFFFF) ^ table[(hash^low)&0x000000FF]
		high := uint32((upper >> 8) & 0x00FF)
		hash = ((hash >> 8) & 0x00FFFFFF) ^ table[(hash^high)&0x000000FF]
	}
	return hash
}

var deprecatedCRCTableCache *[256]uint32

func deprecatedCRCTable() *[256]uint32 {
	if deprecatedCRCTableCache != nil {
		return deprecatedCRCTableCache
	}
	const poly = uint32(0x04C11DB7)
	table := &[256]uint32{}
	for i := 0; i < 256; i++ {
		crc := uint32(i) << 24
		for j := 0; j < 8; j++ {
			if crc&0x80000000 != 0 {
				crc = (crc << 1) ^ poly
			} else {
				crc <<= 1
			}
		}
		table[i] = crc
	}
	deprecatedCRCTableCache = table
	return deprecatedCRCTableCache
}
