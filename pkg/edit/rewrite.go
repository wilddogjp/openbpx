package edit

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

// ExportMutation describes one replacement for an export payload.
type ExportMutation struct {
	ExportIndex    int
	Payload        []byte
	UpdateScript   bool
	ScriptStartRel int64
	ScriptEndRel   int64
}

type exportRangePatch struct {
	exportIndex int
	oldStart    int64
	oldEnd      int64
	oldLen      int64
	newLen      int64
	newStart    int64
	payload     []byte
	mutation    *ExportMutation
}

type summaryOffsetField struct {
	name string
	pos  int
	size int
}

type exportFieldPatch struct {
	serialSizePos   int
	serialOffsetPos int
	scriptStartPos  int
	scriptEndPos    int
}

// RewriteAsset rewrites the .uasset bytes with optional export payload mutations.
// It recalculates export serial offsets/sizes and applies summary offset translation.
func RewriteAsset(asset *uasset.Asset, mutations []ExportMutation) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	raw := asset.Raw.Bytes
	if len(raw) == 0 {
		return nil, fmt.Errorf("asset has no raw bytes")
	}

	mutMap := map[int]ExportMutation{}
	for _, m := range mutations {
		if m.ExportIndex < 0 || m.ExportIndex >= len(asset.Exports) {
			return nil, fmt.Errorf("mutation export index out of range: %d", m.ExportIndex)
		}
		if m.Payload == nil {
			return nil, fmt.Errorf("mutation payload is nil for export %d", m.ExportIndex+1)
		}
		if _, exists := mutMap[m.ExportIndex]; exists {
			return nil, fmt.Errorf("duplicate mutation for export %d", m.ExportIndex+1)
		}
		payload := make([]byte, len(m.Payload))
		copy(payload, m.Payload)
		m.Payload = payload
		mutMap[m.ExportIndex] = m
	}

	patches := make([]*exportRangePatch, 0, len(asset.Exports))
	totalDelta := int64(0)
	for i, exp := range asset.Exports {
		oldStart := exp.SerialOffset
		oldEnd := exp.SerialOffset + exp.SerialSize
		if oldStart < 0 || oldEnd < oldStart || oldEnd > int64(len(raw)) {
			return nil, fmt.Errorf("export[%d] serial range out of bounds (%d..%d)", i+1, oldStart, oldEnd)
		}
		payload := raw[oldStart:oldEnd]
		var mutation *ExportMutation
		if m, ok := mutMap[i]; ok {
			payload = m.Payload
			mutation = &m
		}
		newLen := int64(len(payload))
		totalDelta += newLen - (oldEnd - oldStart)
		patches = append(patches, &exportRangePatch{
			exportIndex: i,
			oldStart:    oldStart,
			oldEnd:      oldEnd,
			oldLen:      oldEnd - oldStart,
			newLen:      newLen,
			payload:     payload,
			mutation:    mutation,
		})
	}

	sort.Slice(patches, func(i, j int) bool {
		if patches[i].oldStart == patches[j].oldStart {
			return patches[i].exportIndex < patches[j].exportIndex
		}
		return patches[i].oldStart < patches[j].oldStart
	})

	cursor := int64(0)
	for _, p := range patches {
		if p.oldStart < cursor {
			return nil, fmt.Errorf("overlapping export serial ranges around export %d", p.exportIndex+1)
		}
		cursor = p.oldEnd
	}

	estCap := int64(len(raw)) + totalDelta
	if estCap < 0 {
		return nil, fmt.Errorf("invalid rewritten file size")
	}
	out := make([]byte, 0, estCap)
	cursor = 0
	for _, p := range patches {
		out = append(out, raw[cursor:p.oldStart]...)
		p.newStart = int64(len(out))
		out = append(out, p.payload...)
		cursor = p.oldEnd
	}
	out = append(out, raw[cursor:]...)

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	summaryFields, err := scanSummaryOffsetFields(raw)
	if err != nil {
		return nil, fmt.Errorf("scan summary offsets: %w", err)
	}
	exportFields, err := scanExportFieldPositions(raw, asset)
	if err != nil {
		return nil, fmt.Errorf("scan export fields: %w", err)
	}
	if len(exportFields) != len(asset.Exports) {
		return nil, fmt.Errorf("export field scan mismatch: got %d want %d", len(exportFields), len(asset.Exports))
	}

	patchByExport := map[int]*exportRangePatch{}
	for _, p := range patches {
		patchByExport[p.exportIndex] = p
	}

	for i, expPatch := range exportFields {
		rangePatch := patchByExport[i]
		if rangePatch == nil {
			return nil, fmt.Errorf("internal error: missing range patch for export %d", i+1)
		}
		if err := writeInt64At(out, expPatch.serialOffsetPos, rangePatch.newStart, order); err != nil {
			return nil, fmt.Errorf("patch export[%d] serial offset: %w", i+1, err)
		}
		if err := writeInt64At(out, expPatch.serialSizePos, rangePatch.newLen, order); err != nil {
			return nil, fmt.Errorf("patch export[%d] serial size: %w", i+1, err)
		}
		if expPatch.scriptStartPos >= 0 && expPatch.scriptEndPos >= 0 {
			scriptStart := asset.Exports[i].ScriptSerializationStartOffset
			scriptEnd := asset.Exports[i].ScriptSerializationEndOffset
			if rangePatch.mutation != nil && rangePatch.mutation.UpdateScript {
				scriptStart = rangePatch.mutation.ScriptStartRel
				scriptEnd = rangePatch.mutation.ScriptEndRel
			}
			if err := writeInt64At(out, expPatch.scriptStartPos, scriptStart, order); err != nil {
				return nil, fmt.Errorf("patch export[%d] script start: %w", i+1, err)
			}
			if err := writeInt64At(out, expPatch.scriptEndPos, scriptEnd, order); err != nil {
				return nil, fmt.Errorf("patch export[%d] script end: %w", i+1, err)
			}
		}
	}

	for _, field := range summaryFields {
		switch field.size {
		case 4:
			oldV, err := readInt32At(raw, field.pos, order)
			if err != nil {
				return nil, fmt.Errorf("read summary %s: %w", field.name, err)
			}
			if oldV < 0 {
				continue
			}
			mapped := translateOffset(int64(oldV), patches)
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
			mapped := translateOffset(oldV, patches)
			if err := writeInt64At(out, field.pos, mapped, order); err != nil {
				return nil, fmt.Errorf("patch summary %s: %w", field.name, err)
			}
		default:
			return nil, fmt.Errorf("unsupported summary field size for %s: %d", field.name, field.size)
		}
	}

	return out, nil
}

func translateOffset(oldPos int64, patches []*exportRangePatch) int64 {
	delta := int64(0)
	for _, p := range patches {
		if oldPos < p.oldStart {
			break
		}
		if oldPos >= p.oldEnd {
			delta += p.newLen - p.oldLen
			continue
		}
		rel := oldPos - p.oldStart
		if rel < 0 {
			rel = 0
		}
		if rel > p.newLen {
			rel = p.newLen
		}
		return p.newStart + rel
	}
	return oldPos + delta
}

func scanSummaryOffsetFields(data []byte) ([]summaryOffsetField, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("file too small")
	}
	tagLE := binary.LittleEndian.Uint32(data[:4])
	var order binary.ByteOrder = binary.LittleEndian
	switch tagLE {
	case packageFileTag:
		order = binary.LittleEndian
	case packageFileTagSwapped:
		order = binary.BigEndian
	default:
		return nil, fmt.Errorf("invalid package tag: 0x%x", tagLE)
	}

	r := newByteCodec(data, order)
	fields := make([]summaryOffsetField, 0, 20)
	record32 := func(name string) (int32, error) {
		pos := r.off
		v, err := r.readInt32()
		if err != nil {
			return 0, err
		}
		fields = append(fields, summaryOffsetField{name: name, pos: pos, size: 4})
		return v, nil
	}
	record64 := func(name string) (int64, error) {
		pos := r.off
		v, err := r.readInt64()
		if err != nil {
			return 0, err
		}
		fields = append(fields, summaryOffsetField{name: name, pos: pos, size: 8})
		return v, nil
	}

	if _, err := r.readInt32(); err != nil { // tag
		return nil, err
	}
	legacy, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	if legacy != -4 {
		if _, err := r.readInt32(); err != nil {
			return nil, err
		}
	}
	fileUE4, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	fileUE5, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	fileLicensee, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	if fileUE4 == 0 && fileUE5 == 0 && fileLicensee == 0 {
		fileUE4 = ue4VersionUE56
		fileUE5 = ue5OSSubObjectShadowSerialization
	}
	if fileUE5 >= ue5PackageSavedHash {
		if err := r.skip(20); err != nil {
			return nil, err
		}
		if _, err := record32("TotalHeaderSize"); err != nil {
			return nil, err
		}
	}
	if legacy <= -2 {
		if err := skipSummaryCustomVersions(r, legacy); err != nil {
			return nil, err
		}
	}
	if fileUE5 < ue5PackageSavedHash {
		if _, err := record32("TotalHeaderSize"); err != nil {
			return nil, err
		}
	}
	if _, err := r.readFString(); err != nil {
		return nil, err
	}
	packageFlags, err := r.readUint32()
	if err != nil {
		return nil, err
	}
	if _, err := r.readInt32(); err != nil { // name count
		return nil, err
	}
	if _, err := record32("NameOffset"); err != nil {
		return nil, err
	}
	if fileUE5 >= ue5AddSoftObjectPathList {
		if _, err := r.readInt32(); err != nil {
			return nil, err
		}
		if _, err := record32("SoftObjectPathsOffset"); err != nil {
			return nil, err
		}
	}
	if packageFlags&pkgFlagFilterEditorOnly == 0 {
		if _, err := r.readFString(); err != nil {
			return nil, err
		}
	}
	if _, err := r.readInt32(); err != nil { // gatherable count
		return nil, err
	}
	if _, err := record32("GatherableTextDataOffset"); err != nil {
		return nil, err
	}
	if _, err := r.readInt32(); err != nil { // export count
		return nil, err
	}
	if _, err := record32("ExportOffset"); err != nil {
		return nil, err
	}
	if _, err := r.readInt32(); err != nil { // import count
		return nil, err
	}
	if _, err := record32("ImportOffset"); err != nil {
		return nil, err
	}
	if fileUE5 >= ue5VerseCells {
		if _, err := r.readInt32(); err != nil {
			return nil, err
		}
		if _, err := record32("CellExportOffset"); err != nil {
			return nil, err
		}
		if _, err := r.readInt32(); err != nil {
			return nil, err
		}
		if _, err := record32("CellImportOffset"); err != nil {
			return nil, err
		}
	}
	if fileUE5 >= ue5MetadataSerializationOff {
		if _, err := record32("MetaDataOffset"); err != nil {
			return nil, err
		}
	}
	if _, err := record32("DependsOffset"); err != nil {
		return nil, err
	}
	if _, err := r.readInt32(); err != nil { // soft package refs count
		return nil, err
	}
	if _, err := record32("SoftPackageReferencesOffset"); err != nil {
		return nil, err
	}
	if _, err := record32("SearchableNamesOffset"); err != nil {
		return nil, err
	}
	if _, err := record32("ThumbnailTableOffset"); err != nil {
		return nil, err
	}
	if fileUE5 < ue5PackageSavedHash {
		if err := r.skip(16); err != nil {
			return nil, err
		}
	}
	if packageFlags&pkgFlagFilterEditorOnly == 0 {
		if err := r.skip(16); err != nil {
			return nil, err
		}
	}
	generationCount, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	if generationCount < 0 {
		return nil, fmt.Errorf("invalid generation count: %d", generationCount)
	}
	for i := int32(0); i < generationCount; i++ {
		if err := r.skip(8); err != nil {
			return nil, err
		}
	}
	if err := skipEngineVersion(r); err != nil {
		return nil, err
	}
	if err := skipEngineVersion(r); err != nil {
		return nil, err
	}
	if _, err := r.readUint32(); err != nil {
		return nil, err
	}
	chunkCount, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	if chunkCount < 0 {
		return nil, fmt.Errorf("invalid compressed chunk count: %d", chunkCount)
	}
	if err := r.skip(int(chunkCount) * 16); err != nil {
		return nil, err
	}
	if _, err := r.readUint32(); err != nil {
		return nil, err
	}
	additionalCount, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	if additionalCount < 0 {
		return nil, fmt.Errorf("invalid additional packages count: %d", additionalCount)
	}
	for i := int32(0); i < additionalCount; i++ {
		if _, err := r.readFString(); err != nil {
			return nil, err
		}
	}
	if legacy > -7 {
		if _, err := r.readInt32(); err != nil {
			return nil, err
		}
	}
	if _, err := record32("AssetRegistryDataOffset"); err != nil {
		return nil, err
	}
	if _, err := record64("BulkDataStartOffset"); err != nil {
		return nil, err
	}
	if _, err := record32("WorldTileInfoDataOffset"); err != nil {
		return nil, err
	}
	chunkIDCount, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	if chunkIDCount < 0 {
		return nil, fmt.Errorf("invalid chunk ID count: %d", chunkIDCount)
	}
	if err := r.skip(int(chunkIDCount) * 4); err != nil {
		return nil, err
	}
	if _, err := r.readInt32(); err != nil { // preload count
		return nil, err
	}
	if _, err := record32("PreloadDependencyOffset"); err != nil {
		return nil, err
	}
	if fileUE5 >= ue5NamesFromExportData {
		if _, err := r.readInt32(); err != nil {
			return nil, err
		}
	}
	if fileUE5 >= ue5PayloadTOC {
		if _, err := record64("PayloadTOCOffset"); err != nil {
			return nil, err
		}
	}
	if fileUE5 >= ue5DataResources {
		if _, err := record32("DataResourceOffset"); err != nil {
			return nil, err
		}
	}

	return fields, nil
}

func skipEngineVersion(r *byteCodec) error {
	if err := r.skip(2); err != nil {
		return err
	}
	if err := r.skip(2); err != nil {
		return err
	}
	if err := r.skip(2); err != nil {
		return err
	}
	if err := r.skip(4); err != nil {
		return err
	}
	_, err := r.readFString()
	return err
}

func skipSummaryCustomVersions(r *byteCodec, legacy int32) error {
	count, err := r.readInt32()
	if err != nil {
		return err
	}
	if count < 0 {
		return fmt.Errorf("invalid custom version count: %d", count)
	}
	switch {
	case legacy == -2:
		for i := int32(0); i < count; i++ {
			if err := r.skip(8); err != nil {
				return err
			}
		}
	case legacy >= -5:
		for i := int32(0); i < count; i++ {
			if err := r.skip(16); err != nil {
				return err
			}
			if _, err := r.readInt32(); err != nil {
				return err
			}
			if _, err := r.readFString(); err != nil {
				return err
			}
		}
	default:
		for i := int32(0); i < count; i++ {
			if err := r.skip(16); err != nil {
				return err
			}
			if _, err := r.readInt32(); err != nil {
				return err
			}
		}
	}
	return nil
}

func scanExportFieldPositions(data []byte, asset *uasset.Asset) ([]exportFieldPatch, error) {
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

	fields := make([]exportFieldPatch, 0, len(asset.Exports))
	for i := 0; i < len(asset.Exports); i++ {
		if err := r.skip(4 * 4); err != nil { // class/super/template/outer indices
			return nil, fmt.Errorf("export[%d] read class/super/template/outer: %w", i+1, err)
		}
		if err := r.skip(8); err != nil { // object name
			return nil, fmt.Errorf("export[%d] read object name: %w", i+1, err)
		}
		if err := r.skip(4); err != nil { // object flags
			return nil, fmt.Errorf("export[%d] read object flags: %w", i+1, err)
		}
		serialSizePos := r.off
		if _, err := r.readInt64(); err != nil {
			return nil, fmt.Errorf("export[%d] read serial size: %w", i+1, err)
		}
		serialOffsetPos := r.off
		if _, err := r.readInt64(); err != nil {
			return nil, fmt.Errorf("export[%d] read serial offset: %w", i+1, err)
		}
		if err := r.skip(4 * 3); err != nil { // forced/client/server
			return nil, fmt.Errorf("export[%d] read bool flags: %w", i+1, err)
		}
		if asset.Summary.FileVersionUE5 < ue5RemoveObjectExportPkgGUID {
			if err := r.skip(16); err != nil {
				return nil, fmt.Errorf("export[%d] read package guid: %w", i+1, err)
			}
		}
		if asset.Summary.FileVersionUE5 >= ue5TrackObjectExportInherited {
			if err := r.skip(4); err != nil {
				return nil, fmt.Errorf("export[%d] read inherited flag: %w", i+1, err)
			}
		}
		if err := r.skip(4); err != nil { // package flags
			return nil, fmt.Errorf("export[%d] read package flags: %w", i+1, err)
		}
		if err := r.skip(4 * 2); err != nil { // not always loaded, is asset
			return nil, fmt.Errorf("export[%d] read load flags: %w", i+1, err)
		}
		if asset.Summary.FileVersionUE5 >= ue5OptionalResources {
			if err := r.skip(4); err != nil {
				return nil, fmt.Errorf("export[%d] read public hash flag: %w", i+1, err)
			}
		}
		if err := r.skip(4 * 5); err != nil {
			return nil, fmt.Errorf("export[%d] read dependency header: %w", i+1, err)
		}
		patch := exportFieldPatch{
			serialSizePos:   serialSizePos,
			serialOffsetPos: serialOffsetPos,
			scriptStartPos:  -1,
			scriptEndPos:    -1,
		}
		if !asset.Summary.UsesUnversionedPropertySerialization() && asset.Summary.FileVersionUE5 >= ue5ScriptSerializationOffset {
			patch.scriptStartPos = r.off
			if _, err := r.readInt64(); err != nil {
				return nil, fmt.Errorf("export[%d] read script start: %w", i+1, err)
			}
			patch.scriptEndPos = r.off
			if _, err := r.readInt64(); err != nil {
				return nil, fmt.Errorf("export[%d] read script end: %w", i+1, err)
			}
		}
		fields = append(fields, patch)
	}
	return fields, nil
}
