package uasset

import "fmt"

const (
	packageFileTag        = uint32(0x9E2A83C1)
	packageFileTagSwapped = uint32(0xC1832A9E)

	pkgFlagNewlyCreated       = uint32(0x00000001)
	pkgFlagIsSaving           = uint32(0x00008000)
	pkgFlagReloadingForCooker = uint32(0x40000000)
	pkgFlagTransient          = pkgFlagNewlyCreated | pkgFlagIsSaving | pkgFlagReloadingForCooker
	pkgFlagFilterEditorOnly   = uint32(0x80000000)
	pkgFlagUnversionedProps   = uint32(0x00002000)

	ue5OptionalResources              = int32(1003)
	ue5RemoveObjectExportPkgGUID      = int32(1005)
	ue5TrackObjectExportInherited     = int32(1006)
	ue5SoftObjectPathTopLevelAsset    = int32(1007)
	ue5AddSoftObjectPathList          = int32(1008)
	ue5DataResources                  = int32(1009)
	ue5ScriptSerializationOffset      = int32(1010)
	ue5PropertyTagExtension           = int32(1011)
	ue5PropertyTagCompleteType        = int32(1012)
	ue5MetadataSerializationOff       = int32(1014)
	ue5VerseCells                     = int32(1015)
	ue5PackageSavedHash               = int32(1016)
	ue5OSSubObjectShadowSerialization = int32(1017)
	ue5NamesFromExportData            = int32(1001)
	ue5PayloadTOC                     = int32(1002)

	ue5MinimumKnown = int32(1000)
	ue5MaximumKnown = int32(1017)
	// UE5.6 uses UE4 file version 522 when writing package summaries.
	ue4VersionUE56 = int32(522)

	maxTableEntries      = int32(1_000_000)
	maxFStringBytes      = int64(16 * 1024 * 1024)
	maxFStringUTF16Units = int64(8 * 1024 * 1024)
)

// Asset is the parsed representation required for read commands.
type Asset struct {
	Raw     RawAsset
	Summary PackageSummary
	Names   []NameEntry
	Imports []ImportEntry
	Exports []ExportEntry

	softObjectPathList       []softObjectPathEntry
	softObjectPathListParsed bool
}

// Version returns Major.Minor string.
func (v EngineVersion) Version() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}
