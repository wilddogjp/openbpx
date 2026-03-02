package uasset

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// ParseOptions controls parser behavior.
type ParseOptions struct {
	KeepUnknown bool
}

// DefaultParseOptions returns CLI defaults.
func DefaultParseOptions() ParseOptions {
	return ParseOptions{
		KeepUnknown: true,
	}
}

// ParseFile parses one .uasset file from disk.
func ParseFile(path string, opts ParseOptions) (*Asset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	asset, err := ParseBytes(data, opts)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return asset, nil
}

// ParseBytes parses one .uasset from in-memory bytes.
func ParseBytes(data []byte, opts ParseOptions) (*Asset, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("file too small")
	}

	r := newByteReader(data)
	summary, err := parseSummary(r)
	if err != nil {
		return nil, fmt.Errorf("parse summary: %w", err)
	}
	summary.SummarySize = r.offset()

	if err := validateSupportedVersionWindow(summary); err != nil {
		return nil, err
	}

	names, err := parseNameMap(data, summary)
	if err != nil {
		return nil, fmt.Errorf("parse name map: %w", err)
	}
	imports, err := parseImportMap(data, summary, len(names))
	if err != nil {
		return nil, fmt.Errorf("parse import map: %w", err)
	}
	exports, err := parseExportMap(data, summary, len(names))
	if err != nil {
		return nil, fmt.Errorf("parse export map: %w", err)
	}

	rawBytes := data
	if opts.KeepUnknown {
		rawBytes = append([]byte(nil), data...)
	}

	asset := &Asset{
		Raw:     RawAsset{Bytes: rawBytes},
		Summary: summary,
		Names:   names,
		Imports: imports,
		Exports: exports,
	}
	return asset, nil
}

func validateSupportedVersionWindow(summary PackageSummary) error {
	if summary.FileVersionUE5 < ue5MinimumKnown || summary.FileVersionUE5 > ue5MaximumKnown {
		return fmt.Errorf(
			"unsupported fileVersionUE5=%d (supported range: %d..%d)",
			summary.FileVersionUE5,
			ue5MinimumKnown,
			ue5MaximumKnown,
		)
	}
	return nil
}

func parseSummary(r *byteReader) (PackageSummary, error) {
	var s PackageSummary

	tag, err := r.readInt32()
	if err != nil {
		return s, err
	}
	s.Tag = tag
	switch uint32(tag) {
	case packageFileTag:
		// ok
	case packageFileTagSwapped:
		s.ByteSwapped = true
		s.Tag = int32(int64(packageFileTag) - (1 << 32))
		r.SetByteSwapping(true)
	default:
		return s, fmt.Errorf("invalid package tag: 0x%x", uint32(tag))
	}

	legacy, err := r.readInt32()
	if err != nil {
		return s, err
	}
	s.LegacyFileVersion = legacy
	if legacy >= 0 {
		return s, fmt.Errorf("legacy package format is not supported")
	}
	if legacy < -9 {
		return s, fmt.Errorf("unsupported legacy file version: %d", legacy)
	}

	if legacy != -4 {
		ue3, err := r.readInt32()
		if err != nil {
			return s, err
		}
		s.LegacyUE3Version = ue3
	}

	if s.FileVersionUE4, err = r.readInt32(); err != nil {
		return s, err
	}
	if legacy <= -8 {
		if s.FileVersionUE5, err = r.readInt32(); err != nil {
			return s, err
		}
	} else {
		return s, fmt.Errorf("missing UE5 file version in summary")
	}
	if s.FileVersionLicenseeUE4, err = r.readInt32(); err != nil {
		return s, err
	}
	s.Unversioned = s.FileVersionUE4 == 0 && s.FileVersionUE5 == 0 && s.FileVersionLicenseeUE4 == 0
	if s.Unversioned {
		// Match UE loader behavior: unversioned package summaries are promoted to
		// current supported versions before reading the rest of the summary.
		s.FileVersionUE4 = ue4VersionUE56
		s.FileVersionUE5 = ue5MaximumKnown
		s.FileVersionLicenseeUE4 = 0
	}

	if s.FileVersionUE5 >= ue5PackageSavedHash {
		if s.SavedHash, err = r.readHash20(); err != nil {
			return s, err
		}
		if s.TotalHeaderSize, err = r.readInt32(); err != nil {
			return s, err
		}
	}

	if legacy <= -2 {
		if s.CustomVersions, err = readCustomVersions(r, legacy); err != nil {
			return s, err
		}
	}

	if s.FileVersionUE5 < ue5PackageSavedHash {
		if s.TotalHeaderSize, err = r.readInt32(); err != nil {
			return s, err
		}
	}

	if s.PackageName, err = r.readFString(); err != nil {
		return s, err
	}
	if s.PackageFlags, err = r.readUint32(); err != nil {
		return s, err
	}
	// Match UE loader behavior: transient runtime-only flags are cleared after deserialization.
	s.PackageFlags &^= pkgFlagTransient
	if s.NameCount, err = r.readInt32(); err != nil {
		return s, err
	}
	if err := validateCount("name count", s.NameCount); err != nil {
		return s, err
	}
	if s.NameOffset, err = r.readInt32(); err != nil {
		return s, err
	}

	if s.FileVersionUE5 >= ue5AddSoftObjectPathList {
		if s.SoftObjectPathsCount, err = r.readInt32(); err != nil {
			return s, err
		}
		if err := validateCount("soft object path count", s.SoftObjectPathsCount); err != nil {
			return s, err
		}
		if s.SoftObjectPathsOffset, err = r.readInt32(); err != nil {
			return s, err
		}
	}

	if !s.IsEditorOnlyFiltered() {
		if s.LocalizationID, err = r.readFString(); err != nil {
			return s, err
		}
	}

	if s.GatherableTextDataCount, err = r.readInt32(); err != nil {
		return s, err
	}
	if err := validateCount("gatherable text data count", s.GatherableTextDataCount); err != nil {
		return s, err
	}
	if s.GatherableTextDataOffset, err = r.readInt32(); err != nil {
		return s, err
	}
	if s.ExportCount, err = r.readInt32(); err != nil {
		return s, err
	}
	if err := validateCount("export count", s.ExportCount); err != nil {
		return s, err
	}
	if s.ExportOffset, err = r.readInt32(); err != nil {
		return s, err
	}
	if s.ImportCount, err = r.readInt32(); err != nil {
		return s, err
	}
	if err := validateCount("import count", s.ImportCount); err != nil {
		return s, err
	}
	if s.ImportOffset, err = r.readInt32(); err != nil {
		return s, err
	}

	if s.FileVersionUE5 >= ue5VerseCells {
		if s.CellExportCount, err = r.readInt32(); err != nil {
			return s, err
		}
		if err := validateCount("cell export count", s.CellExportCount); err != nil {
			return s, err
		}
		if s.CellExportOffset, err = r.readInt32(); err != nil {
			return s, err
		}
		if s.CellImportCount, err = r.readInt32(); err != nil {
			return s, err
		}
		if err := validateCount("cell import count", s.CellImportCount); err != nil {
			return s, err
		}
		if s.CellImportOffset, err = r.readInt32(); err != nil {
			return s, err
		}
	}

	if s.FileVersionUE5 >= ue5MetadataSerializationOff {
		if s.MetaDataOffset, err = r.readInt32(); err != nil {
			return s, err
		}
	}

	if s.DependsOffset, err = r.readInt32(); err != nil {
		return s, err
	}
	if s.SoftPackageReferencesCount, err = r.readInt32(); err != nil {
		return s, err
	}
	if err := validateCount("soft package reference count", s.SoftPackageReferencesCount); err != nil {
		return s, err
	}
	if s.SoftPackageReferencesOffset, err = r.readInt32(); err != nil {
		return s, err
	}
	if s.SearchableNamesOffset, err = r.readInt32(); err != nil {
		return s, err
	}
	if s.ThumbnailTableOffset, err = r.readInt32(); err != nil {
		return s, err
	}

	if s.FileVersionUE5 < ue5PackageSavedHash {
		if _, err := r.readGUID(); err != nil {
			return s, err
		}
	}

	if !s.IsEditorOnlyFiltered() {
		if s.PersistentGUID, err = r.readGUID(); err != nil {
			return s, err
		}
	}

	generationCount, err := r.readInt32()
	if err != nil {
		return s, err
	}
	if err := validateCountWithRemaining("generation count", generationCount, r.remaining(), 8); err != nil {
		return s, err
	}
	s.Generations = make([]GenerationInfo, generationCount)
	for i := 0; i < int(generationCount); i++ {
		exports, err := r.readInt32()
		if err != nil {
			return s, err
		}
		names, err := r.readInt32()
		if err != nil {
			return s, err
		}
		s.Generations[i] = GenerationInfo{ExportCount: exports, NameCount: names}
	}

	if s.SavedByEngineVersion, err = r.readEngineVersion(); err != nil {
		return s, err
	}
	if s.CompatibleEngineVersion, err = r.readEngineVersion(); err != nil {
		return s, err
	}

	if s.CompressionFlags, err = r.readUint32(); err != nil {
		return s, err
	}
	chunkCount, err := r.readInt32()
	if err != nil {
		return s, err
	}
	if err := validateCountWithRemaining("compressed chunk count", chunkCount, r.remaining(), 16); err != nil {
		return s, err
	}
	s.CompressedChunks = make([]CompressedChunk, 0, chunkCount)
	for i := 0; i < int(chunkCount); i++ {
		chunk := CompressedChunk{}
		if chunk.UncompressedOffset, err = r.readInt32(); err != nil {
			return s, err
		}
		if chunk.UncompressedSize, err = r.readInt32(); err != nil {
			return s, err
		}
		if chunk.CompressedOffset, err = r.readInt32(); err != nil {
			return s, err
		}
		if chunk.CompressedSize, err = r.readInt32(); err != nil {
			return s, err
		}
		s.CompressedChunks = append(s.CompressedChunks, chunk)
	}

	if s.PackageSource, err = r.readUint32(); err != nil {
		return s, err
	}
	if s.AdditionalPackagesToCook, err = readStringArray(r); err != nil {
		return s, err
	}

	if legacy > -7 {
		if _, err := r.readInt32(); err != nil {
			return s, err
		}
	}

	if s.AssetRegistryDataOffset, err = r.readInt32(); err != nil {
		return s, err
	}
	if s.BulkDataStartOffset, err = r.readInt64(); err != nil {
		return s, err
	}
	if s.WorldTileInfoDataOffset, err = r.readInt32(); err != nil {
		return s, err
	}
	if s.ChunkIDs, err = readInt32Array(r); err != nil {
		return s, err
	}
	if s.PreloadDependencyCount, err = r.readInt32(); err != nil {
		return s, err
	}
	if s.PreloadDependencyCount < -1 || s.PreloadDependencyCount > maxTableEntries {
		return s, fmt.Errorf("invalid preload dependency count: %d", s.PreloadDependencyCount)
	}
	if s.PreloadDependencyOffset, err = r.readInt32(); err != nil {
		return s, err
	}

	if s.FileVersionUE5 >= ue5NamesFromExportData {
		if s.NamesReferencedFromExportDataCount, err = r.readInt32(); err != nil {
			return s, err
		}
		if err := validateCount("names referenced from export data count", s.NamesReferencedFromExportDataCount); err != nil {
			return s, err
		}
	} else {
		s.NamesReferencedFromExportDataCount = s.NameCount
	}

	if s.FileVersionUE5 >= ue5PayloadTOC {
		if s.PayloadTOCOffset, err = r.readInt64(); err != nil {
			return s, err
		}
	} else {
		s.PayloadTOCOffset = -1
	}

	if s.FileVersionUE5 >= ue5DataResources {
		if s.DataResourceOffset, err = r.readInt32(); err != nil {
			return s, err
		}
	} else {
		s.DataResourceOffset = -1
	}

	return s, nil
}

func parseNameMap(data []byte, summary PackageSummary) ([]NameEntry, error) {
	if err := validateCount("name count", summary.NameCount); err != nil {
		return nil, err
	}
	if err := requireOffsetInFile(data, summary.NameOffset, "name offset"); err != nil {
		return nil, err
	}
	remaining := remainingFromOffset(data, summary.NameOffset)
	if err := validateCountWithRemaining("name count", summary.NameCount, remaining, 8); err != nil {
		return nil, err
	}
	r := NewByteReaderWithByteSwapping(data, summary.UsesByteSwappedSerialization())
	if err := r.seek(int(summary.NameOffset)); err != nil {
		return nil, err
	}

	out := make([]NameEntry, 0, summary.NameCount)
	for i := int32(0); i < summary.NameCount; i++ {
		value, err := r.readFString()
		if err != nil {
			return nil, fmt.Errorf("name[%d] string: %w", i, err)
		}
		h1, err := r.readUint16()
		if err != nil {
			return nil, fmt.Errorf("name[%d] non-case hash: %w", i, err)
		}
		h2, err := r.readUint16()
		if err != nil {
			return nil, fmt.Errorf("name[%d] case hash: %w", i, err)
		}
		out = append(out, NameEntry{
			Value:              value,
			NonCaseHash:        h1,
			CasePreservingHash: h2,
		})
	}
	return out, nil
}

func parseImportMap(data []byte, summary PackageSummary, nameCount int) ([]ImportEntry, error) {
	if err := validateCount("import count", summary.ImportCount); err != nil {
		return nil, err
	}
	if err := requireOffsetInFile(data, summary.ImportOffset, "import offset"); err != nil {
		return nil, err
	}
	remaining := remainingFromOffset(data, summary.ImportOffset)
	if err := validateCountWithRemaining("import count", summary.ImportCount, remaining, estimateMinImportEntryBytes(summary)); err != nil {
		return nil, err
	}
	r := NewByteReaderWithByteSwapping(data, summary.UsesByteSwappedSerialization())
	if err := r.seek(int(summary.ImportOffset)); err != nil {
		return nil, err
	}

	out := make([]ImportEntry, 0, summary.ImportCount)
	for i := int32(0); i < summary.ImportCount; i++ {
		entry := ImportEntry{}
		var err error
		if entry.ClassPackage, err = r.readNameRef(nameCount); err != nil {
			return nil, fmt.Errorf("import[%d] class package: %w", i, err)
		}
		if entry.ClassName, err = r.readNameRef(nameCount); err != nil {
			return nil, fmt.Errorf("import[%d] class name: %w", i, err)
		}
		outer, err := r.readInt32()
		if err != nil {
			return nil, fmt.Errorf("import[%d] outer index: %w", i, err)
		}
		entry.OuterIndex = PackageIndex(outer)
		if entry.ObjectName, err = r.readNameRef(nameCount); err != nil {
			return nil, fmt.Errorf("import[%d] object name: %w", i, err)
		}
		if !summary.IsEditorOnlyFiltered() {
			if entry.PackageName, err = r.readNameRef(nameCount); err != nil {
				return nil, fmt.Errorf("import[%d] package name: %w", i, err)
			}
		}
		if summary.FileVersionUE5 >= ue5OptionalResources {
			opt, err := r.readUBool()
			if err != nil {
				return nil, fmt.Errorf("import[%d] optional flag: %w", i, err)
			}
			entry.ImportOptional = opt
		}
		out = append(out, entry)
	}
	return out, nil
}

func parseExportMap(data []byte, summary PackageSummary, nameCount int) ([]ExportEntry, error) {
	if err := validateCount("export count", summary.ExportCount); err != nil {
		return nil, err
	}
	if err := requireOffsetInFile(data, summary.ExportOffset, "export offset"); err != nil {
		return nil, err
	}
	remaining := remainingFromOffset(data, summary.ExportOffset)
	if err := validateCountWithRemaining("export count", summary.ExportCount, remaining, estimateMinExportEntryBytes(summary)); err != nil {
		return nil, err
	}
	r := NewByteReaderWithByteSwapping(data, summary.UsesByteSwappedSerialization())
	if err := r.seek(int(summary.ExportOffset)); err != nil {
		return nil, err
	}

	out := make([]ExportEntry, 0, summary.ExportCount)
	for i := int32(0); i < summary.ExportCount; i++ {
		entry := ExportEntry{}
		classIndex, err := r.readInt32()
		if err != nil {
			return nil, fmt.Errorf("export[%d] class index: %w", i, err)
		}
		entry.ClassIndex = PackageIndex(classIndex)

		superIndex, err := r.readInt32()
		if err != nil {
			return nil, fmt.Errorf("export[%d] super index: %w", i, err)
		}
		entry.SuperIndex = PackageIndex(superIndex)

		templateIndex, err := r.readInt32()
		if err != nil {
			return nil, fmt.Errorf("export[%d] template index: %w", i, err)
		}
		entry.TemplateIndex = PackageIndex(templateIndex)

		outerIndex, err := r.readInt32()
		if err != nil {
			return nil, fmt.Errorf("export[%d] outer index: %w", i, err)
		}
		entry.OuterIndex = PackageIndex(outerIndex)

		if entry.ObjectName, err = r.readNameRef(nameCount); err != nil {
			return nil, fmt.Errorf("export[%d] object name: %w", i, err)
		}

		if entry.ObjectFlags, err = r.readUint32(); err != nil {
			return nil, fmt.Errorf("export[%d] object flags: %w", i, err)
		}
		if entry.SerialSize, err = r.readInt64(); err != nil {
			return nil, fmt.Errorf("export[%d] serial size: %w", i, err)
		}
		if entry.SerialOffset, err = r.readInt64(); err != nil {
			return nil, fmt.Errorf("export[%d] serial offset: %w", i, err)
		}

		if entry.ForcedExport, err = r.readUBool(); err != nil {
			return nil, fmt.Errorf("export[%d] forced export: %w", i, err)
		}
		if entry.NotForClient, err = r.readUBool(); err != nil {
			return nil, fmt.Errorf("export[%d] not for client: %w", i, err)
		}
		if entry.NotForServer, err = r.readUBool(); err != nil {
			return nil, fmt.Errorf("export[%d] not for server: %w", i, err)
		}

		if summary.FileVersionUE5 < ue5RemoveObjectExportPkgGUID {
			if _, err := r.readGUID(); err != nil {
				return nil, fmt.Errorf("export[%d] package guid: %w", i, err)
			}
		}

		if summary.FileVersionUE5 >= ue5TrackObjectExportInherited {
			if entry.IsInheritedInstance, err = r.readUBool(); err != nil {
				return nil, fmt.Errorf("export[%d] inherited flag: %w", i, err)
			}
		}

		if entry.PackageFlags, err = r.readUint32(); err != nil {
			return nil, fmt.Errorf("export[%d] package flags: %w", i, err)
		}

		if entry.NotAlwaysLoadedForEditor, err = r.readUBool(); err != nil {
			return nil, fmt.Errorf("export[%d] not always loaded for editor: %w", i, err)
		}
		if entry.IsAsset, err = r.readUBool(); err != nil {
			return nil, fmt.Errorf("export[%d] is asset: %w", i, err)
		}

		if summary.FileVersionUE5 >= ue5OptionalResources {
			if entry.GeneratePublicHash, err = r.readUBool(); err != nil {
				return nil, fmt.Errorf("export[%d] generate public hash: %w", i, err)
			}
		}

		if entry.FirstExportDependency, err = r.readInt32(); err != nil {
			return nil, fmt.Errorf("export[%d] first dependency: %w", i, err)
		}
		if entry.SerializationBeforeSerializationDeps, err = r.readInt32(); err != nil {
			return nil, fmt.Errorf("export[%d] serialization->serialization deps: %w", i, err)
		}
		if entry.CreateBeforeSerializationDeps, err = r.readInt32(); err != nil {
			return nil, fmt.Errorf("export[%d] create->serialization deps: %w", i, err)
		}
		if entry.SerializationBeforeCreateDependencies, err = r.readInt32(); err != nil {
			return nil, fmt.Errorf("export[%d] serialization->create deps: %w", i, err)
		}
		if entry.CreateBeforeCreateDependencies, err = r.readInt32(); err != nil {
			return nil, fmt.Errorf("export[%d] create->create deps: %w", i, err)
		}

		if summary.SupportsScriptSerializationOffsets() {
			if entry.ScriptSerializationStartOffset, err = r.readInt64(); err != nil {
				return nil, fmt.Errorf("export[%d] script start offset: %w", i, err)
			}
			if entry.ScriptSerializationEndOffset, err = r.readInt64(); err != nil {
				return nil, fmt.Errorf("export[%d] script end offset: %w", i, err)
			}
		}

		out = append(out, entry)
	}

	for i, exp := range out {
		if exp.SerialOffset < 0 || exp.SerialSize < 0 {
			return nil, fmt.Errorf("export[%d] has negative serial range", i)
		}
		end := exp.SerialOffset + exp.SerialSize
		if end < exp.SerialOffset || end > int64(len(data)) {
			return nil, fmt.Errorf("export[%d] serial range out of bounds (%d..%d, file=%d)", i, exp.SerialOffset, end, len(data))
		}
		if summary.SupportsScriptSerializationOffsets() ||
			exp.ScriptSerializationStartOffset != 0 ||
			exp.ScriptSerializationEndOffset != 0 {
			if exp.ScriptSerializationStartOffset < 0 ||
				exp.ScriptSerializationEndOffset < exp.ScriptSerializationStartOffset ||
				exp.ScriptSerializationEndOffset > exp.SerialSize {
				return nil, fmt.Errorf(
					"export[%d] invalid script serialization range (start=%d end=%d serialSize=%d)",
					i,
					exp.ScriptSerializationStartOffset,
					exp.ScriptSerializationEndOffset,
					exp.SerialSize,
				)
			}
		}
	}

	return out, nil
}

func requireOffsetInFile(data []byte, offset int32, label string) error {
	if offset < 0 || int(offset) > len(data) {
		return fmt.Errorf("%s out of range: %d (file size %d)", label, offset, len(data))
	}
	return nil
}

func readCustomVersionsOptimized(r *byteReader) ([]CustomVersion, error) {
	count, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	if err := validateCountWithRemaining("custom version count", count, r.remaining(), 20); err != nil {
		return nil, err
	}
	out := make([]CustomVersion, 0, count)
	for i := int32(0); i < count; i++ {
		guid, err := r.readGUID()
		if err != nil {
			return nil, err
		}
		ver, err := r.readInt32()
		if err != nil {
			return nil, err
		}
		out = append(out, CustomVersion{Key: guid, Version: ver})
	}
	return out, nil
}

func readCustomVersions(r *byteReader, legacy int32) ([]CustomVersion, error) {
	if legacy > -2 {
		return nil, nil
	}
	switch {
	case legacy == -2:
		return readCustomVersionsEnums(r)
	case legacy >= -5:
		return readCustomVersionsGuids(r)
	default:
		return readCustomVersionsOptimized(r)
	}
}

func readCustomVersionsEnums(r *byteReader) ([]CustomVersion, error) {
	count, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	if err := validateCountWithRemaining("custom version count", count, r.remaining(), 8); err != nil {
		return nil, err
	}
	out := make([]CustomVersion, 0, count)
	for i := int32(0); i < count; i++ {
		tag, err := r.readUint32()
		if err != nil {
			return nil, err
		}
		version, err := r.readInt32()
		if err != nil {
			return nil, err
		}
		var key GUID
		binary.LittleEndian.PutUint32(key[12:16], tag)
		out = append(out, CustomVersion{Key: key, Version: version})
	}
	return out, nil
}

func readCustomVersionsGuids(r *byteReader) ([]CustomVersion, error) {
	count, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	if err := validateCountWithRemaining("custom version count", count, r.remaining(), 24); err != nil {
		return nil, err
	}
	out := make([]CustomVersion, 0, count)
	for i := int32(0); i < count; i++ {
		guid, err := r.readGUID()
		if err != nil {
			return nil, err
		}
		version, err := r.readInt32()
		if err != nil {
			return nil, err
		}
		if _, err := r.readFString(); err != nil {
			return nil, err
		}
		out = append(out, CustomVersion{Key: guid, Version: version})
	}
	return out, nil
}

func readStringArray(r *byteReader) ([]string, error) {
	count, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	// Minimum 4 bytes per entry (FString length field), so we can bound allocations.
	if err := validateCountWithRemaining("string array count", count, r.remaining(), 4); err != nil {
		return nil, err
	}
	out := make([]string, 0, count)
	for i := int32(0); i < count; i++ {
		s, err := r.readFString()
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func readInt32Array(r *byteReader) ([]int32, error) {
	count, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	if err := validateCountWithRemaining("int32 array count", count, r.remaining(), 4); err != nil {
		return nil, err
	}
	out := make([]int32, 0, count)
	for i := int32(0); i < count; i++ {
		v, err := r.readInt32()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// ParseIndex resolves a package index to a human readable target.
func (a *Asset) ParseIndex(idx PackageIndex) string {
	if idx == 0 {
		return "null"
	}
	resolved := idx.ResolveIndex()
	if idx > 0 {
		if resolved >= 0 && resolved < len(a.Exports) {
			return fmt.Sprintf("export:%d:%s", resolved+1, a.Exports[resolved].ObjectName.Display(a.Names))
		}
		return fmt.Sprintf("export:%d:<out-of-range>", resolved+1)
	}
	if resolved >= 0 && resolved < len(a.Imports) {
		return fmt.Sprintf("import:%d:%s", resolved+1, a.Imports[resolved].ObjectName.Display(a.Names))
	}
	return fmt.Sprintf("import:%d:<out-of-range>", resolved+1)
}

// GuessAssetKind returns a rough kind name derived from import/export class names.
func (a *Asset) GuessAssetKind() string {
	for _, exp := range a.Exports {
		className := strings.ToLower(a.ResolveClassName(exp))
		if strings.Contains(className, "datatable") {
			return "DataTable"
		}
		if strings.Contains(className, "blueprint") {
			return "Blueprint"
		}
	}
	return "Unknown"
}

func validateCount(label string, count int32) error {
	if count < 0 {
		return fmt.Errorf("invalid %s: %d", label, count)
	}
	if count > maxTableEntries {
		return fmt.Errorf("invalid %s: %d exceeds max %d", label, count, maxTableEntries)
	}
	return nil
}

func validateCountWithRemaining(label string, count int32, remaining int, minBytesPerItem int) error {
	if err := validateCount(label, count); err != nil {
		return err
	}
	if minBytesPerItem <= 0 {
		return nil
	}
	if remaining < 0 {
		return fmt.Errorf("invalid %s: negative remaining bytes %d", label, remaining)
	}
	maxByRemaining := int32(remaining / minBytesPerItem)
	if count > maxByRemaining {
		return fmt.Errorf(
			"invalid %s: count=%d exceeds remaining=%d with minimum entry size=%d",
			label,
			count,
			remaining,
			minBytesPerItem,
		)
	}
	return nil
}

func remainingFromOffset(data []byte, offset int32) int {
	if offset < 0 {
		return -1
	}
	off := int(offset)
	if off > len(data) {
		return -1
	}
	return len(data) - off
}

func estimateMinImportEntryBytes(summary PackageSummary) int {
	size := 28 // class package/name, outer, object name
	if !summary.IsEditorOnlyFiltered() {
		size += 8 // package name
	}
	if summary.FileVersionUE5 >= ue5OptionalResources {
		size += 4 // bImportOptional
	}
	return size
}

func estimateMinExportEntryBytes(summary PackageSummary) int {
	size := 88 // FObjectExport fields without optional pieces
	if summary.FileVersionUE5 < ue5RemoveObjectExportPkgGUID {
		size += 16 // package guid
	}
	if summary.FileVersionUE5 >= ue5TrackObjectExportInherited {
		size += 4 // bIsInheritedInstance
	}
	if summary.FileVersionUE5 >= ue5OptionalResources {
		size += 4 // bGeneratePublicHash
	}
	if summary.SupportsScriptSerializationOffsets() {
		size += 16 // script serialization offsets
	}
	return size
}
