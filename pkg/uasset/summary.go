package uasset

import (
	"encoding/hex"
	"fmt"
)

// GUID is a simple 128-bit identifier from UE serialization.
type GUID [16]byte

func (g GUID) String() string {
	return fmt.Sprintf(
		"%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		g[3], g[2], g[1], g[0],
		g[5], g[4],
		g[7], g[6],
		g[8], g[9],
		g[10], g[11], g[12], g[13], g[14], g[15],
	)
}

// EngineVersion mirrors FEngineVersion fields used in package summary.
type EngineVersion struct {
	Major      uint16 `json:"major"`
	Minor      uint16 `json:"minor"`
	Patch      uint16 `json:"patch"`
	Changelist uint32 `json:"changelist"`
	Branch     string `json:"branch"`
}

// CustomVersion is one entry in FCustomVersionContainer.
type CustomVersion struct {
	Key     GUID  `json:"key"`
	Version int32 `json:"version"`
}

// GenerationInfo is one FGenerationInfo entry.
type GenerationInfo struct {
	ExportCount int32 `json:"exportCount"`
	NameCount   int32 `json:"nameCount"`
}

// CompressedChunk mirrors FCompressedChunk.
type CompressedChunk struct {
	UncompressedOffset int32 `json:"uncompressedOffset"`
	UncompressedSize   int32 `json:"uncompressedSize"`
	CompressedOffset   int32 `json:"compressedOffset"`
	CompressedSize     int32 `json:"compressedSize"`
}

// PackageSummary is the parsed FPackageFileSummary (subset required by read commands + validation).
type PackageSummary struct {
	Tag                    int32 `json:"tag"`
	ByteSwapped            bool  `json:"byteSwapped,omitempty"`
	LegacyFileVersion      int32 `json:"legacyFileVersion"`
	LegacyUE3Version       int32 `json:"legacyUE3Version"`
	FileVersionUE4         int32 `json:"fileVersionUE4"`
	FileVersionUE5         int32 `json:"fileVersionUE5"`
	FileVersionLicenseeUE4 int32 `json:"fileVersionLicenseeUE4"`
	SavedHash              [20]byte
	TotalHeaderSize        int32           `json:"totalHeaderSize"`
	CustomVersions         []CustomVersion `json:"customVersions"`

	PackageName  string `json:"packageName"`
	PackageFlags uint32 `json:"packageFlags"`

	NameCount  int32 `json:"nameCount"`
	NameOffset int32 `json:"nameOffset"`

	SoftObjectPathsCount  int32 `json:"softObjectPathsCount"`
	SoftObjectPathsOffset int32 `json:"softObjectPathsOffset"`

	LocalizationID string `json:"localizationId"`

	GatherableTextDataCount  int32 `json:"gatherableTextDataCount"`
	GatherableTextDataOffset int32 `json:"gatherableTextDataOffset"`

	MetaDataOffset int32 `json:"metaDataOffset"`

	ExportCount  int32 `json:"exportCount"`
	ExportOffset int32 `json:"exportOffset"`

	ImportCount  int32 `json:"importCount"`
	ImportOffset int32 `json:"importOffset"`

	CellExportCount  int32 `json:"cellExportCount"`
	CellExportOffset int32 `json:"cellExportOffset"`
	CellImportCount  int32 `json:"cellImportCount"`
	CellImportOffset int32 `json:"cellImportOffset"`

	DependsOffset int32 `json:"dependsOffset"`

	SoftPackageReferencesCount  int32 `json:"softPackageReferencesCount"`
	SoftPackageReferencesOffset int32 `json:"softPackageReferencesOffset"`

	SearchableNamesOffset int32 `json:"searchableNamesOffset"`
	ThumbnailTableOffset  int32 `json:"thumbnailTableOffset"`
	PersistentGUID        GUID  `json:"persistentGuid"`

	Generations []GenerationInfo `json:"generations"`

	SavedByEngineVersion               EngineVersion `json:"savedByEngineVersion"`
	CompatibleEngineVersion            EngineVersion `json:"compatibleEngineVersion"`
	CompressionFlags                   uint32        `json:"compressionFlags"`
	CompressedChunks                   []CompressedChunk
	PackageSource                      uint32   `json:"packageSource"`
	AdditionalPackagesToCook           []string `json:"additionalPackagesToCook"`
	AssetRegistryDataOffset            int32    `json:"assetRegistryDataOffset"`
	BulkDataStartOffset                int64    `json:"bulkDataStartOffset"`
	WorldTileInfoDataOffset            int32    `json:"worldTileInfoDataOffset"`
	ChunkIDs                           []int32  `json:"chunkIds"`
	PreloadDependencyCount             int32    `json:"preloadDependencyCount"`
	PreloadDependencyOffset            int32    `json:"preloadDependencyOffset"`
	NamesReferencedFromExportDataCount int32    `json:"namesReferencedFromExportDataCount"`
	PayloadTOCOffset                   int64    `json:"payloadTocOffset"`
	DataResourceOffset                 int32    `json:"dataResourceOffset"`

	SummarySize int  `json:"summarySize"`
	Unversioned bool `json:"unversioned"`
}

// IsEditorOnlyFiltered reports whether PKG_FilterEditorOnly is set.
func (s PackageSummary) IsEditorOnlyFiltered() bool {
	return s.PackageFlags&pkgFlagFilterEditorOnly != 0
}

// UsesUnversionedPropertySerialization reports whether export map omits script offsets.
func (s PackageSummary) UsesUnversionedPropertySerialization() bool {
	return s.PackageFlags&pkgFlagUnversionedProps != 0
}

// UsesByteSwappedSerialization reports whether package numeric fields should be byte-swapped.
func (s PackageSummary) UsesByteSwappedSerialization() bool {
	return s.ByteSwapped
}

// SupportsScriptSerializationOffsets reports whether export map includes script offset fields.
func (s PackageSummary) SupportsScriptSerializationOffsets() bool {
	return !s.UsesUnversionedPropertySerialization() && s.FileVersionUE5 >= ue5ScriptSerializationOffset
}

// SupportsPropertyTagCompleteTypeName reports whether FPropertyTag uses complete type-name format.
func (s PackageSummary) SupportsPropertyTagCompleteTypeName() bool {
	return s.FileVersionUE5 >= ue5PropertyTagCompleteType
}

// SupportsTopLevelAssetPathSoftObjectPath reports whether FSoftObjectPath uses TopLevelAssetPath + SubPath.
func (s PackageSummary) SupportsTopLevelAssetPathSoftObjectPath() bool {
	return s.FileVersionUE5 >= ue5SoftObjectPathTopLevelAsset
}

// SupportsSoftObjectPathListInSummary reports whether package summary carries SoftObjectPathsCount/Offset.
func (s PackageSummary) SupportsSoftObjectPathListInSummary() bool {
	return s.FileVersionUE5 >= ue5AddSoftObjectPathList
}

// IsUE56 reports whether summary looks like UE 5.6 package metadata.
func (s PackageSummary) IsUE56() bool {
	// UE5.6 package file versions observed in UE source notes are 1015..1017.
	if s.FileVersionUE5 < ue5VerseCells || s.FileVersionUE5 > ue5MaximumKnown {
		return false
	}

	// UE cooker may serialize empty engine versions when no changelist is available.
	// Treat empty SavedBy as acceptable for UE5.6 as long as CompatibleWith is either
	// empty or explicitly 5.6.
	savedMajor := s.SavedByEngineVersion.Major
	savedMinor := s.SavedByEngineVersion.Minor
	if savedMajor == 0 && savedMinor == 0 {
		compatibleMajor := s.CompatibleEngineVersion.Major
		compatibleMinor := s.CompatibleEngineVersion.Minor
		if compatibleMajor == 0 && compatibleMinor == 0 {
			return true
		}
		return compatibleMajor == 5 && compatibleMinor == 6
	}
	return savedMajor == 5 && savedMinor == 6
}

// SavedHashHex returns the package saved hash in lowercase hex.
func (s PackageSummary) SavedHashHex() string {
	return hex.EncodeToString(s.SavedHash[:])
}
